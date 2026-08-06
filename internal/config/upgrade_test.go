package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadUpgradeConfig exercises the global-only [upgrade] reader (PRD §9.29 FR-U10). It is a
// white-box test (package config) so it can call the unexported helpers loadEnvSetup/writeConfigFile
// (load_test.go) and inspect globalConfigPath/fileConfig directly. The FR-B3 "no bootstrap write"
// assertion (missing_file_writes_nothing) is the load-bearing case: config.Load would have written a
// file HERE; LoadUpgradeConfig must NOT.
func TestLoadUpgradeConfig(t *testing.T) {
	defaults := Defaults().Upgrade // {Channel:"stable", SourceRepo:"dabstractor/stagecoach"}

	tests := []struct {
		name      string
		body      string // global config.toml body; "" ⇒ do NOT write the file (missing-file case)
		want      UpgradeConfig
		wantErr   bool
		errSubstr string // substring the error must contain (the path, when wantErr)
	}{
		{
			name: "missing_file_returns_defaults",
			body: "",
			want: defaults,
		},
		{
			name: "no_upgrade_table_returns_defaults",
			body: "[defaults]\nprovider = \"pi\"\n",
			want: defaults,
		},
		{
			name: "full_upgrade_table_merged",
			body: "[upgrade]\nchannel = \"prerelease\"\nsource_repo = \"foo/bar\"\n",
			want: UpgradeConfig{Channel: "prerelease", SourceRepo: "foo/bar"},
		},
		{
			name: "channel_only_other_field_default",
			body: "[upgrade]\nchannel = \"prerelease\"\n",
			want: UpgradeConfig{Channel: "prerelease", SourceRepo: "dabstractor/stagecoach"},
		},
		{
			name: "source_repo_only_other_field_default",
			body: "[upgrade]\nsource_repo = \"fork/op\"\n",
			want: UpgradeConfig{Channel: "stable", SourceRepo: "fork/op"},
		},
		{
			name: "empty_value_does_not_clobber_default",
			body: "[upgrade]\nchannel = \"\"\nsource_repo = \"\"\n",
			want: defaults, // non-empty wins: "" must not clobber the seeded default
		},
		{
			name:      "malformed_toml_wraps_error_with_path",
			body:      "[upgrade\nchannel = \"broken\n", // unterminated table + string
			wantErr:   true,
			errSubstr: "config.toml", // the global path basename (full path is environment-specific)
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Defensive notice capture — LoadUpgradeConfig must write nothing to noticeOut.
			origNoticeOut := noticeOut
			noticeOut = &strings.Builder{}
			defer func() { noticeOut = origNoticeOut }()

			_, _, globalDir := loadEnvSetup(t) // sets HOME + XDG_CONFIG_HOME ⇒ globalDir=$home/stagecoach

			if tc.body != "" {
				writeConfigFile(t, globalDir, "config.toml", tc.body)
			}

			uc, err := LoadUpgradeConfig()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil (uc=%+v)", uc)
				}
				if !strings.Contains(err.Error(), tc.errSubstr) {
					t.Fatalf("err %q must contain %q", err.Error(), tc.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("err=%v, want nil", err)
			}
			if uc != tc.want {
				t.Fatalf("LoadUpgradeConfig = %+v, want %+v", uc, tc.want)
			}

			// noticeOut must be empty (no advisory — FR-B4).
			if got := noticeOut.(*strings.Builder).String(); got != "" {
				t.Fatalf("LoadUpgradeConfig wrote a notice (%q); contract forbids it", got)
			}
		})
	}

	// THE load-bearing assertion: config.Load would have bootstrapped a file HERE on a missing global
	// file. LoadUpgradeConfig must NOT (FR-B3 boundary — the core reason this seam exists).
	t.Run("missing_file_writes_nothing_FR_B3", func(t *testing.T) {
		_, _, globalDir := loadEnvSetup(t) // XDG set; NO global file written
		globalPath := filepath.Join(globalDir, "config.toml")

		uc, err := LoadUpgradeConfig()
		if err != nil {
			t.Fatalf("err=%v, want nil", err)
		}
		if uc.Channel != "stable" || uc.SourceRepo != "dabstractor/stagecoach" {
			t.Fatalf("missing file ⇒ defaults; got %+v", uc)
		}
		if _, statErr := os.Stat(globalPath); !os.IsNotExist(statErr) {
			t.Fatalf("LoadUpgradeConfig must NOT write a bootstrap file (FR-B3); statErr=%v", statErr)
		}
	})

	// Older-version (v2) global file WITH [upgrade] must load with NO advisory (additive discipline,
	// FR-B4). go-toml silently drops unknown keys, but [upgrade] is now decodable; config_version=2 is
	// harmless to THIS reader (the advisory is Load()'s concern, not LoadUpgradeConfig's).
	t.Run("v2_file_with_upgrade_no_advisory", func(t *testing.T) {
		origNoticeOut := noticeOut
		noticeOut = &strings.Builder{}
		defer func() { noticeOut = origNoticeOut }()

		_, _, globalDir := loadEnvSetup(t)
		writeConfigFile(t, globalDir, "config.toml",
			"config_version = 2\n[upgrade]\nchannel = \"prerelease\"\nsource_repo = \"other/repo\"\n")

		uc, err := LoadUpgradeConfig()
		if err != nil {
			t.Fatalf("err=%v, want nil", err)
		}
		if uc.Channel != "prerelease" || uc.SourceRepo != "other/repo" {
			t.Fatalf("v2 file [upgrade] ⇒ merged; got %+v", uc)
		}
		if got := noticeOut.(*strings.Builder).String(); got != "" {
			t.Fatalf("v2 file with [upgrade] emitted a notice (%q); FR-B4 forbids it", got)
		}
	})
}

// TestLoad_UpgradeTableIgnoredByResolver is the FR-U10 + FR-B4 regression guard. config.Load() must
// structurally ignore [upgrade] from BOTH the global file AND the repo .stagecoach.toml (the resolver
// must not surface a per-repo [upgrade]). materialize()/overlay() do NOT copy fc.Upgrade, so the
// resolved cfg.Upgrade always equals Defaults().Upgrade regardless of what either file sets. A v3
// global file WITH [upgrade] must also emit NO advisory (FR-B4 — [upgrade] is additive, never advisory).
func TestLoad_UpgradeTableIgnoredByResolver(t *testing.T) {
	t.Run("global_and_repo_upgrade_both_ignored", func(t *testing.T) {
		_, repo, globalDir := loadEnvSetup(t)

		// Global file sets channel="prerelease" (would leak if materialize copied fc.Upgrade).
		writeConfigFile(t, globalDir, "config.toml",
			"[upgrade]\nchannel = \"prerelease\"\nsource_repo = \"global/repo\"\n")
		// Repo file sets source_repo="evil/repo" (the FR-U10 per-repo-leak hazard).
		writeConfigFile(t, repo, ".stagecoach.toml",
			"[upgrade]\nsource_repo = \"evil/repo\"\n")

		chdir(t, repo)
		cfg, err := Load(context.Background(), LoadOpts{RepoDir: repo, DisableBootstrap: true})
		if err != nil {
			t.Fatalf("Load err=%v", err)
		}
		if cfg.Upgrade != Defaults().Upgrade {
			t.Fatalf("resolver ignored [upgrade]; got %+v, want %+v (Defaults)", cfg.Upgrade, Defaults().Upgrade)
		}
	})

	t.Run("v3_file_with_upgrade_no_advisory", func(t *testing.T) {
		origNoticeOut := noticeOut
		noticeOut = &strings.Builder{}
		defer func() { noticeOut = origNoticeOut }()

		_, repo, globalDir := loadEnvSetup(t)
		writeConfigFile(t, globalDir, "config.toml",
			"config_version = 3\n[upgrade]\nchannel = \"prerelease\"\n")

		chdir(t, repo)
		if _, err := Load(context.Background(), LoadOpts{RepoDir: repo, DisableBootstrap: true}); err != nil {
			t.Fatalf("Load err=%v", err)
		}
		if got := noticeOut.(*strings.Builder).String(); got != "" {
			t.Fatalf("v3 file with [upgrade] emitted a notice (%q); FR-B4 forbids it", got)
		}
	})
}

// TestCurrentConfigVersion_Unchanged is a cheap lock ensuring FR-B4's "stays at 3" discipline holds
// after this additive task (adding [upgrade] must NOT bump the schema version).
func TestCurrentConfigVersion_Unchanged(t *testing.T) {
	if CurrentConfigVersion != 3 {
		t.Fatalf("CurrentConfigVersion = %d, want 3 (FR-B4: [upgrade] is additive, no version bump)", CurrentConfigVersion)
	}
}
