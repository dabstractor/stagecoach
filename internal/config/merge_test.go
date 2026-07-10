package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// TestMergeActiveSettings verifies the FR-B8 merge: active settings from `existing` are carried
// verbatim into `fresh`, in-place where fresh has the key, appended under their section otherwise.
// This is the engine behind `config init --force`'s preserve-settings contract (Issue 3).
func TestMergeActiveSettings(t *testing.T) {
	t.Run("inert existing is a no-op (returns fresh unchanged)", func(t *testing.T) {
		fresh := "config_version = 3\n[defaults]\nprovider = \"pi\"\n"
		inert := "# all commented\n# provider = \"claude\"\n"
		got := MergeActiveSettings(fresh, inert)
		if got != fresh {
			t.Errorf("inert existing changed fresh:\ngot:  %q\nwant: %q", got, fresh)
		}
	})

	t.Run("in-place replace: fresh key overridden by existing value", func(t *testing.T) {
		fresh := "config_version = 3\n[defaults]\nprovider = \"pi\"\nmodel = \"\"\n"
		existing := "[defaults]\nprovider = \"claude\"\nmodel = \"sonnet\"\ntimeout = \"60s\"\n"
		got := MergeActiveSettings(fresh, existing)
		if !strings.Contains(got, `provider = "claude"`) {
			t.Errorf("provider not overridden to claude:\n%s", got)
		}
		if !strings.Contains(got, `model = "sonnet"`) {
			t.Errorf("model not overridden to sonnet:\n%s", got)
		}
		// timeout is in existing but NOT fresh → appended as an owed key under [defaults].
		if !strings.Contains(got, `timeout = "60s"`) {
			t.Errorf("owed timeout not appended:\n%s", got)
		}
	})

	t.Run("owed section appended when fresh lacks it", func(t *testing.T) {
		fresh := "config_version = 3\n[defaults]\nprovider = \"pi\"\n"
		existing := "[role.planner]\nprovider = \"agy\"\nmodel = \"gemini-3.1-pro\"\n"
		got := MergeActiveSettings(fresh, existing)
		// [role.planner] is absent from fresh → appended as its own block with both keys.
		if !strings.Contains(got, "[role.planner]") {
			t.Errorf("owed [role.planner] section not appended:\n%s", got)
		}
		if !strings.Contains(got, `provider = "agy"`) {
			t.Errorf("owed planner provider not appended:\n%s", got)
		}
		if !strings.Contains(got, `model = "gemini-3.1-pro"`) {
			t.Errorf("owed planner model not appended:\n%s", got)
		}
	})

	t.Run("config_version from existing is NOT carried (metadata)", func(t *testing.T) {
		// A stale v2 config_version must not override the fresh current version.
		fresh := "config_version = 3\n[defaults]\nprovider = \"pi\"\n"
		existing := "config_version = 2\n[defaults]\nprovider = \"claude\"\n"
		got := MergeActiveSettings(fresh, existing)
		// Must contain exactly one top-level config_version = 3, NOT a carried config_version = 2.
		if strings.Contains(got, "config_version = 2") {
			t.Errorf("stale config_version=2 was carried (should NOT be — metadata):\n%s", got)
		}
		if strings.Count(got, "config_version = 3") != 1 {
			t.Errorf("expected exactly one config_version = 3:\n%s", got)
		}
	})

	t.Run("result is valid TOML", func(t *testing.T) {
		fresh := GenerateBootstrapConfig("pi")
		existing := "config_version = 3\n[defaults]\nprovider = \"claude\"\nmodel = \"sonnet\"\ntimeout = \"60s\"\n" +
			"[role.planner]\nprovider = \"agy\"\nmodel = \"gemini-3.1-pro\"\n" +
			"[generation]\nmax_diff_bytes = 200000\n"
		got := MergeActiveSettings(fresh, existing)
		var m map[string]any
		if err := toml.Unmarshal([]byte(got), &m); err != nil {
			t.Fatalf("merged result is not valid TOML: %v\n%s", err, got)
		}
		// The preserved values must decode correctly.
		defs, _ := m["defaults"].(map[string]any)
		if defs == nil || defs["provider"] != "claude" {
			t.Errorf("preserved [defaults] provider not decoded as claude:\n%s", got)
		}
	})

	t.Run("full Issue-3 repro: hand-tuned config preserved through pi bootstrap", func(t *testing.T) {
		// The exact reproduction from the validation report Issue 3.
		existing := "config_version = 3\n" +
			"[defaults]\n" +
			"provider = \"claude\"\n" +
			"model = \"sonnet\"\n" +
			"timeout = \"60s\"\n" +
			"[role.planner]\n" +
			"provider = \"agy\"\n" +
			"model = \"gemini-3.1-pro\"\n"
		fresh := GenerateBootstrapConfig("pi")
		got := MergeActiveSettings(fresh, existing)

		// Every active user setting must survive (FR-B8).
		for _, want := range []string{
			`provider = "claude"`,
			`model = "sonnet"`,
			`timeout = "60s"`,
			`provider = "agy"`,
			`model = "gemini-3.1-pro"`,
		} {
			if !strings.Contains(got, want) {
				t.Errorf("FR-B8: active setting %q was NOT preserved:\n%s", want, got)
			}
		}
		// Valid TOML.
		var m map[string]any
		if err := toml.Unmarshal([]byte(got), &m); err != nil {
			t.Fatalf("merged result not valid TOML: %v\n%s", err, got)
		}
	})
}

// TestWriteTimestampedBackup verifies the FR-B8 backup helper: it creates a sibling .bak.<timestamp>
// file with identical contents and returns the path. A nonexistent source is a no-op (returns "").
func TestWriteTimestampedBackup(t *testing.T) {
	t.Run("creates timestamped sibling with identical contents", func(t *testing.T) {
		dir := t.TempDir()
		src := dir + "/config.toml"
		body := "config_version = 3\n[defaults]\nprovider = \"pi\"\n"
		if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		backup, err := WriteTimestampedBackup(src)
		if err != nil {
			t.Fatalf("WriteTimestampedBackup err=%v", err)
		}
		if backup == "" {
			t.Fatal("backup path empty, want a .bak.<timestamp> path")
		}
		if !strings.HasPrefix(filepath.Base(backup), "config.toml.bak.") {
			t.Errorf("backup name = %q, want prefix config.toml.bak.", filepath.Base(backup))
		}
		got, err := os.ReadFile(backup)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != body {
			t.Errorf("backup contents differ from source:\ngot:  %q\nwant: %q", string(got), body)
		}
	})

	t.Run("nonexistent source is a no-op", func(t *testing.T) {
		dir := t.TempDir()
		backup, err := WriteTimestampedBackup(dir + "/absent.toml")
		if err != nil {
			t.Fatalf("err=%v, want nil (absent source is not an error)", err)
		}
		if backup != "" {
			t.Errorf("backup = %q, want empty (nothing to back up)", backup)
		}
	})
}
