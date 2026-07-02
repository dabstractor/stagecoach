package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/dustin/stagehand/internal/config"
)

// These white-box tests cover the MOCKING contract for the config command tree
// (PRD FR38, PRD §15.3/§16.1/§16.2). They mirror the repo's testing
// conventions: white-box package main, stdlib only (+ go-toml/v2 and
// internal/config), no testify. The hermetic targets are the pure helpers
// writeExampleConfig / offerGitignore and the RunE closures, driven with
// bytes.Buffer / strings.Reader, with the env-derived path pinned via
// t.Setenv("XDG_CONFIG_HOME", t.TempDir()).

// stripComments removes a single leading '#' plus an optional following space
// from every non-blank line of s, leaving blank lines blank and inline
// trailing comments intact (go-toml/v2 ignores them). It mirrors the helper in
// internal/config/example_test.go so a WRITTEN file (commented) can be turned
// back into the parseable TOML body. Prose lines written as `## note` survive
// as `# note` comments.
func stripComments(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "" {
			b.WriteByte('\n')
			continue
		}
		rest := line
		if strings.HasPrefix(rest, "#") {
			rest = rest[1:]
			if strings.HasPrefix(rest, " ") {
				rest = rest[1:]
			}
		}
		b.WriteString(rest)
		b.WriteByte('\n')
	}
	return b.String()
}

// localDTO mirrors the in-package config.fileDTO TOML shape for the CLI seam:
// it asserts the FILE WRITTEN by the command (not just the in-memory string)
// parses back to the defaults. It lives here because config.fileDTO is
// UNEXPORTED; the white-box config-side coverage is in
// internal/config/example_test.go.
type localDTO struct {
	Defaults   *localDefaults   `toml:"defaults"`
	Generation *localGeneration `toml:"generation"`
}

type localDefaults struct {
	Provider     *string `toml:"provider"`
	Model        *string `toml:"model"`
	Timeout      *string `toml:"timeout"`
	AutoStageAll *bool   `toml:"auto_stage_all"`
	Verbose      *bool   `toml:"verbose"`
}

type localGeneration struct {
	MaxDiffBytes        *int    `toml:"max_diff_bytes"`
	MaxMdLines          *int    `toml:"max_md_lines"`
	MaxDuplicateRetries *int    `toml:"max_duplicate_retries"`
	Output              *string `toml:"output"`
	StripCodeFence      *bool   `toml:"strip_code_fence"`
	SubjectTargetChars  *int    `toml:"subject_target_chars"`
}

// TestConfigPath_PrintsXDG asserts `config path` prints the XDG-resolved path
// (ends in stagehand/config.toml) when XDG_CONFIG_HOME is set.
func TestConfigPath_PrintsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", "")

	cmd := newConfigPathCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if !strings.HasSuffix(out, "stagehand/config.toml") {
		t.Errorf("output %q does not end with stagehand/config.toml", out)
	}
}

// TestConfigPath_HomeFallback asserts the $HOME/.config fallback path is used
// when XDG_CONFIG_HOME is unset.
func TestConfigPath_HomeFallback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", t.TempDir())

	cmd := newConfigPathCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if !strings.HasSuffix(out, ".config/stagehand/config.toml") {
		t.Errorf("output %q does not end with .config/stagehand/config.toml", out)
	}
}

// TestConfigPath_ErrorsWhenUnset asserts `config path` errors (exit 1) when
// neither XDG_CONFIG_HOME nor HOME is set.
func TestConfigPath_ErrorsWhenUnset(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	cmd := newConfigPathCmd()
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.RunE(cmd, nil); err == nil {
		t.Fatal("expected error when both XDG_CONFIG_HOME and HOME unset, got nil")
	}
}

// TestConfigInit_WritesValidDefaults asserts the FILE WRITTEN by writeExampleConfig
// (with leading '# ' stripped) parses back to the config.Default() scalar
// values. This proves the CLI writes valid TOML that round-trips to defaults.
func TestConfigInit_WritesValidDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", "")

	p, err := config.GlobalConfigPath()
	if err != nil {
		t.Fatalf("GlobalConfigPath error: %v", err)
	}

	var buf bytes.Buffer
	if err := writeExampleConfig(&buf, p, false); err != nil {
		t.Fatalf("writeExampleConfig error: %v", err)
	}
	if !strings.Contains(buf.String(), "Wrote commented example config") {
		t.Errorf("missing confirmation line; got %q", buf.String())
	}

	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile(%s) error: %v", p, err)
	}
	var d localDTO
	if err := toml.Unmarshal([]byte(stripComments(string(data))), &d); err != nil {
		t.Fatalf("written file failed to parse as TOML: %v", err)
	}

	if d.Defaults == nil {
		t.Fatal("written file missing [defaults] table")
	}
	if d.Defaults.Provider == nil || *d.Defaults.Provider != "" {
		t.Errorf("defaults.provider = %v, want %q", d.Defaults.Provider, "")
	}
	if d.Defaults.Model == nil || *d.Defaults.Model != "" {
		t.Errorf("defaults.model = %v, want %q", d.Defaults.Model, "")
	}
	if d.Defaults.Timeout == nil {
		t.Fatal("defaults.timeout nil")
	}
	gotTimeout, err := time.ParseDuration(*d.Defaults.Timeout)
	if err != nil {
		t.Fatalf("ParseDuration(%q) error: %v", *d.Defaults.Timeout, err)
	}
	if gotTimeout != config.DefaultTimeout {
		t.Errorf("defaults.timeout = %v, want %v", gotTimeout, config.DefaultTimeout)
	}
	if d.Defaults.AutoStageAll == nil || *d.Defaults.AutoStageAll != config.DefaultAutoStageAll {
		t.Errorf("defaults.auto_stage_all = %v, want %v", d.Defaults.AutoStageAll, config.DefaultAutoStageAll)
	}

	if d.Generation == nil {
		t.Fatal("written file missing [generation] table")
	}
	if d.Generation.MaxDiffBytes == nil || *d.Generation.MaxDiffBytes != config.DefaultMaxDiffBytes {
		t.Errorf("generation.max_diff_bytes = %v, want %d", d.Generation.MaxDiffBytes, config.DefaultMaxDiffBytes)
	}
	if d.Generation.MaxMdLines == nil || *d.Generation.MaxMdLines != config.DefaultMaxMdLines {
		t.Errorf("generation.max_md_lines = %v, want %d", d.Generation.MaxMdLines, config.DefaultMaxMdLines)
	}
	if d.Generation.MaxDuplicateRetries == nil || *d.Generation.MaxDuplicateRetries != config.DefaultMaxDuplicateRetries {
		t.Errorf("generation.max_duplicate_retries = %v, want %d", d.Generation.MaxDuplicateRetries, config.DefaultMaxDuplicateRetries)
	}
	if d.Generation.Output == nil || *d.Generation.Output != config.DefaultOutput {
		t.Errorf("generation.output = %v, want %q", d.Generation.Output, config.DefaultOutput)
	}
	if d.Generation.StripCodeFence == nil || *d.Generation.StripCodeFence != config.DefaultStripCodeFence {
		t.Errorf("generation.strip_code_fence = %v, want %v", d.Generation.StripCodeFence, config.DefaultStripCodeFence)
	}
	if d.Generation.SubjectTargetChars == nil || *d.Generation.SubjectTargetChars != config.DefaultSubjectTargetChars {
		t.Errorf("generation.subject_target_chars = %v, want %d", d.Generation.SubjectTargetChars, config.DefaultSubjectTargetChars)
	}
}

// TestConfigInit_NoClobber asserts writeExampleConfig refuses to overwrite an
// existing file without --force (error mentioning "already exists", file left
// untouched).
func TestConfigInit_NoClobber(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", "")

	p, err := config.GlobalConfigPath()
	if err != nil {
		t.Fatalf("GlobalConfigPath error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("OLD\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err = writeExampleConfig(&buf, p, false)
	if err == nil {
		t.Fatal("expected error for existing file without --force, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error %q does not mention \"already exists\"", err.Error())
	}

	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile(%s) error: %v", p, err)
	}
	if string(data) != "OLD\n" {
		t.Errorf("file was modified; got %q, want %q", string(data), "OLD\n")
	}
}

// TestConfigInit_ForceOverwrites asserts writeExampleConfig overwrites an
// existing file when force is true.
func TestConfigInit_ForceOverwrites(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", "")

	p, err := config.GlobalConfigPath()
	if err != nil {
		t.Fatalf("GlobalConfigPath error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("OLD\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := writeExampleConfig(&buf, p, true); err != nil {
		t.Fatalf("writeExampleConfig(force) error: %v", err)
	}

	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile(%s) error: %v", p, err)
	}
	if !strings.Contains(string(data), "timeout =") {
		t.Errorf("file does not contain the template (missing \"timeout =\"); got:\n%s", string(data))
	}
}

// TestOfferGitignore_Yes asserts a 'y' answer (inside a repo) appends
// .stagehand.toml to .gitignore, creating it if absent.
func TestOfferGitignore_Yes(t *testing.T) {
	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := offerGitignore(&buf, strings.NewReader("y\n"), cwd); err != nil {
		t.Fatalf("offerGitignore error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cwd, ".gitignore"))
	if err != nil {
		t.Fatalf(".gitignore not created: %v", err)
	}
	if !strings.Contains(string(data), ".stagehand.toml") {
		t.Errorf(".gitignore does not contain .stagehand.toml; got:\n%s", string(data))
	}
	if !strings.Contains(buf.String(), "Added") {
		t.Errorf("missing confirmation output; got %q", buf.String())
	}
}

// TestOfferGitignore_No asserts a 'n' answer is a no-op (no .gitignore).
func TestOfferGitignore_No(t *testing.T) {
	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := offerGitignore(&buf, strings.NewReader("n\n"), cwd); err != nil {
		t.Fatalf("offerGitignore error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cwd, ".gitignore")); !os.IsNotExist(err) {
		t.Errorf("expected .gitignore to NOT exist; stat err=%v", err)
	}
}

// TestOfferGitignore_NoGitDir asserts offerGitignore is a no-op (no prompt, no
// write) when run outside a git repository.
func TestOfferGitignore_NoGitDir(t *testing.T) {
	cwd := t.TempDir() // intentionally no .git

	var buf bytes.Buffer
	if err := offerGitignore(&buf, strings.NewReader("y\n"), cwd); err != nil {
		t.Fatalf("offerGitignore error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output outside a repo; got %q", buf.String())
	}
	if _, err := os.Stat(filepath.Join(cwd, ".gitignore")); !os.IsNotExist(err) {
		t.Errorf("expected .gitignore to NOT exist outside a repo; stat err=%v", err)
	}
}

// TestConfigCmd_Registered asserts the config tree is wired onto rootCmd (via
// the package init) without main.go being edited, that init/path are its
// children, and that the init command carries a LOCAL --force flag.
func TestConfigCmd_Registered(t *testing.T) {
	cfgCmd, _, err := rootCmd.Find([]string{"config"})
	if err != nil {
		t.Fatalf("rootCmd.Find([config]) error: %v", err)
	}
	if cfgCmd == nil {
		t.Fatal("config command not registered on rootCmd")
	}
	if cfgCmd.Name() != "config" {
		t.Errorf("config command Name = %q, want %q", cfgCmd.Name(), "config")
	}
	for _, sub := range []string{"init", "path"} {
		c, _, err := cfgCmd.Find([]string{sub})
		if err != nil || c == nil {
			t.Errorf("config subcommand %q not found: err=%v", sub, err)
		}
	}
	initCmd, _, err := cfgCmd.Find([]string{"init"})
	if err != nil || initCmd == nil {
		t.Fatalf("init subcommand not found: err=%v", err)
	}
	if initCmd.Flags().Lookup("force") == nil {
		t.Error("init command is missing the local --force flag")
	}
}
