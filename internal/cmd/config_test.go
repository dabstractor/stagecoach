package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/exitcode"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// setupNoRepo creates isolated temp dirs for HOME/XDG and a plain (non-git) temp dir,
// then chdir's into it. Returns home, plainDir, globalDir. Use for tests proving config init/path
// work OUTSIDE a git repo (shouldSkipConfigLoad returns true for init/path).
func setupNoRepo(t *testing.T) (home, plainDir, globalDir string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	plainDir = t.TempDir()
	chdir(t, plainDir)
	globalDir = filepath.Join(home, "stagehand")
	return home, plainDir, globalDir
}

// ---------------------------------------------------------------------------
// config path tests
// ---------------------------------------------------------------------------

func TestConfigPath_PrintsGlobalPath(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupNoRepo(t)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "path"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := strings.TrimSpace(out.String())
	expected := config.GlobalConfigPath()
	if got != expected {
		t.Errorf("config path output = %q, want %q", got, expected)
	}
	// Must end with stagehand/config.toml
	if !strings.HasSuffix(got, filepath.Join("stagehand", "config.toml")) {
		t.Errorf("config path output = %q, want to end with stagehand/config.toml", got)
	}
}

func TestConfigPath_ExtraArgsExits1(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupNoRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "path", "x"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (extra args)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
}

func TestConfigPath_WorksOutsideGitRepo(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupNoRepo(t)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "path"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (works outside git repo)", err)
	}
	if out.Len() == 0 {
		t.Error("expected output on stdout")
	}
}

// ---------------------------------------------------------------------------
// config init tests — populated default (no flags)
// ---------------------------------------------------------------------------

func TestConfigInit_Populated_WritesWorkingConfig(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, _, globalDir := setupNoRepo(t)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	// stdout should contain the confirmation
	if !strings.Contains(out.String(), "Wrote config to") {
		t.Errorf("stdout = %q, want to contain 'Wrote config to'", out.String())
	}

	// The file should exist at the global config path
	path := config.GlobalConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read written config at %s: %v", path, err)
	}

	content := string(data)

	// Must have uncommented config_version = 2
	if !strings.Contains(content, "config_version = 2") {
		t.Error("populated config missing uncommented config_version = 2")
	}

	// Must have an uncommented [defaults] section with provider = "..."
	if !strings.Contains(content, "provider = \"") {
		t.Error("populated config missing uncommented provider line")
	}

	// Must have an uncommented [role.message] section (structural — model may vary)
	if !strings.Contains(content, "[role.message]") {
		t.Error("populated config missing [role.message] section")
	}

	// Must have all four role blocks
	for _, role := range []string{"planner", "stager", "message", "arbiter"} {
		if !strings.Contains(content, "[role."+role+"]") {
			t.Errorf("populated config missing [role.%s] section", role)
		}
	}

	// Parent dir should exist
	if _, err := os.Stat(globalDir); err != nil {
		t.Errorf("parent dir %s should exist: %v", globalDir, err)
	}
}

// ---------------------------------------------------------------------------
// config init tests — --provider pin
// ---------------------------------------------------------------------------

func TestConfigInit_ProviderPin_ExactOutput(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, _, _ = setupNoRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--provider", "pi"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	data, err := os.ReadFile(config.GlobalConfigPath())
	if err != nil {
		t.Fatalf("cannot read config: %v", err)
	}
	content := string(data)

	// config_version uncommented
	if !strings.Contains(content, "config_version = 2") {
		t.Error("missing uncommented config_version = 2")
	}

	// [defaults] provider = "pi"
	if !strings.Contains(content, `provider = "pi"`) {
		t.Error("missing provider = \"pi\" in [defaults]")
	}

	// pi's role models (exact)
	assertContains(t, content, "[role.planner]", `model = "gpt-5.4"`)
	assertContains(t, content, "[role.message]", `model = "gpt-5.4-nano"`)
	assertContains(t, content, "[role.stager]", `model = "gpt-5.4-mini"`)
	assertContains(t, content, "[role.arbiter]", `model = "gpt-5.4-mini"`)

	// pi IS stager-capable — no fallback annotation
	if strings.Contains(content, "cannot serve as the stager") {
		t.Error("pi config should NOT have stager fallback annotation")
	}
}

func TestConfigInit_ProviderStagerFallback(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, _, _ = setupNoRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--provider", "gemini"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	data, err := os.ReadFile(config.GlobalConfigPath())
	if err != nil {
		t.Fatalf("cannot read config: %v", err)
	}
	content := string(data)

	// [defaults] provider = "gemini"
	if !strings.Contains(content, `provider = "gemini"`) {
		t.Error("missing provider = \"gemini\" in [defaults]")
	}

	// planner uses gemini's model
	assertContains(t, content, "[role.planner]", `model = "gemini-3.5-pro"`)

	// stager is routed to pi (fallback)
	assertContains(t, content, "[role.stager]", `provider = "pi"`)
	assertContains(t, content, "[role.stager]", `model = "gpt-5.4-mini"`)

	// annotation about gemini not being stager-capable
	if !strings.Contains(content, "cannot serve as the stager") {
		t.Error("gemini config should have stager fallback annotation")
	}
	if !strings.Contains(content, "routed to pi") {
		t.Error("gemini config should mention routed to pi")
	}
}

// ---------------------------------------------------------------------------
// config init tests -- --template
// ---------------------------------------------------------------------------

func TestConfigInit_Template_WritesInert(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, _, globalDir := setupNoRepo(t)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--template"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	// stdout should contain the confirmation
	if !strings.Contains(out.String(), "Wrote example config") {
		t.Errorf("stdout = %q, want to contain 'Wrote example config'", out.String())
	}

	// The file should exist at the global config path
	path := config.GlobalConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read written config at %s: %v", path, err)
	}

	// Content should match the template exactly
	got := string(data)
	if got != exampleConfigTemplate {
		t.Errorf("written config does not match template (length %d vs %d)", len(got), len(exampleConfigTemplate))
	}

	// Parent dir should exist
	if _, err := os.Stat(globalDir); err != nil {
		t.Errorf("parent dir %s should exist: %v", globalDir, err)
	}
}

func TestConfigInit_TemplateIsInert(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	setupNoRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--template"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	path := config.GlobalConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read config at %s: %v", path, err)
	}

	content := string(data)

	// NO line should be an un-commented TOML table header: ^[[a-z]
	uncommentedSection := regexp.MustCompile(`^[[a-z]`)
	for i, line := range strings.Split(content, "\n") {
		if uncommentedSection.MatchString(line) {
			t.Errorf("line %d is an uncommented TOML header: %q (template must be inert)", i+1, line)
		}
	}

	// But the commented headers MUST be present (as guidance)
	for _, section := range []string{"[defaults]", "[generation]", "[provider.pi]", "[provider.myagent]"} {
		if !strings.Contains(content, section) {
			t.Errorf("template missing commented section %q", section)
		}
	}

	// Env-var and git-key docs must be present
	if !strings.Contains(content, "STAGEHAND_PROVIDER") {
		t.Error("template missing STAGEHAND_PROVIDER env-var doc")
	}
	if !strings.Contains(content, "stagehand.provider") {
		t.Error("template missing stagehand.provider git-key doc")
	}
}

// ---------------------------------------------------------------------------
// config init tests -- --force
// ---------------------------------------------------------------------------

func TestConfigInit_Force_OverwritesPopulated(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, _, globalDir := setupNoRepo(t)
	// Pre-create the config file with some content
	writeConfigFile(t, globalDir, "config.toml", `provider = "mine"
`)

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--force", "--provider", "pi"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	data, err := os.ReadFile(config.GlobalConfigPath())
	if err != nil {
		t.Fatalf("cannot read config: %v", err)
	}
	content := string(data)

	// Should be the populated pi config, NOT "mine"
	if !strings.Contains(content, `provider = "pi"`) {
		t.Error("after --force overwrite, expected provider = \"pi\", got different content")
	}
	if strings.Contains(content, "mine") {
		t.Error("after --force overwrite, old content \"mine\" should be gone")
	}
}

func TestConfigInit_Force_OverwritesTemplate(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, _, globalDir := setupNoRepo(t)
	// Pre-create the config file with some content
	writeConfigFile(t, globalDir, "config.toml", `provider = "mine"
`)

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--force", "--template"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	data, err := os.ReadFile(config.GlobalConfigPath())
	if err != nil {
		t.Fatalf("cannot read config: %v", err)
	}
	content := string(data)

	// Should be the exampleConfigTemplate
	if content != exampleConfigTemplate {
		t.Error("after --force --template overwrite, expected exampleConfigTemplate")
	}
}

// ---------------------------------------------------------------------------
// config init tests -- error cases
// ---------------------------------------------------------------------------

func TestConfigInit_RefusesOverwrite(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, _, globalDir := setupNoRepo(t)
	// Pre-create the config file with some content
	writeConfigFile(t, globalDir, "config.toml", `provider = "mine"
`)
	prePath := filepath.Join(globalDir, "config.toml")
	preContent, _ := os.ReadFile(prePath)

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (file already exists)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error message %q should contain 'already exists'", err.Error())
	}

	// File must be UNCHANGED
	afterContent, _ := os.ReadFile(prePath)
	if string(afterContent) != string(preContent) {
		t.Error("config file was modified (should be unchanged — non-destructive)")
	}
}

func TestConfigInit_UnknownProvider(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	setupNoRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--provider", "bogus"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (unknown provider)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("error message %q should contain 'unknown provider'", err.Error())
	}
}

func TestConfigInit_MkdirAllParent(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, _, globalDir := setupNoRepo(t)
	// The parent dir (<home>/stagehand) should NOT exist yet (loadEnvSetup doesn't create it)
	if _, err := os.Stat(globalDir); err == nil {
		t.Fatalf("parent dir %s already exists (test setup issue)", globalDir)
	}

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	// Parent dir should now exist
	if _, err := os.Stat(globalDir); err != nil {
		t.Errorf("parent dir %s should exist after init: %v", globalDir, err)
	}
	// File should exist
	path := config.GlobalConfigPath()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("config file %s should exist after init: %v", path, err)
	}
}

func TestConfigInit_WorksOutsideGitRepo(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	setupNoRepo(t)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)

	// config init should succeed outside a git repo
	rootCmd.SetArgs([]string{"config", "init"})
	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (works outside git repo)", err)
	}
	if !strings.Contains(out.String(), "Wrote config to") {
		t.Errorf("stdout = %q, want to contain 'Wrote config to'", out.String())
	}

	// A second init should fail (refuse overwrite)
	out.Reset()
	rootCmd.SetArgs([]string{"config", "init"})
	err = Execute(context.Background())
	if err == nil {
		t.Fatal("second Execute err=nil, want error (refuse overwrite)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
}

func TestConfigInit_ExtraArgsExits1(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	setupNoRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "x"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (extra args)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
}

// ---------------------------------------------------------------------------
// buildBootstrapConfig — pure unit tests (no Execute, no $PATH)
// ---------------------------------------------------------------------------

func TestBuildBootstrapConfig_Pi(t *testing.T) {
	content := buildBootstrapConfig("pi", []string{"pi"})

	// config_version = 2 uncommented
	if !strings.Contains(content, "config_version = 2") {
		t.Error("missing config_version = 2")
	}

	// provider = "pi" uncommented
	if !strings.Contains(content, `provider = "pi"`) {
		t.Error("missing provider = \"pi\"")
	}

	// pi's four role models uncommented
	assertContains(t, content, "[role.planner]", `model = "gpt-5.4"`)
	assertContains(t, content, "[role.stager]", `model = "gpt-5.4-mini"`)
	assertContains(t, content, "[role.message]", `model = "gpt-5.4-nano"`)
	assertContains(t, content, "[role.arbiter]", `model = "gpt-5.4-mini"`)

	// pi IS stager-capable — no fallback annotation
	if strings.Contains(content, "cannot serve as the stager") {
		t.Error("pi config should NOT have stager fallback annotation")
	}

	// No other-provider commented blocks (only pi in installed)
	if strings.Contains(content, "=== claude (installed)") {
		t.Error("pi-only config should not have claude commented block")
	}
}

func TestBuildBootstrapConfig_GeminiStagerFallback(t *testing.T) {
	content := buildBootstrapConfig("gemini", nil)

	// provider = "gemini"
	if !strings.Contains(content, `provider = "gemini"`) {
		t.Error("missing provider = \"gemini\"")
	}

	// gemini's planner model
	assertContains(t, content, "[role.planner]", `model = "gemini-3.5-pro"`)

	// stager routed to pi
	assertContains(t, content, "[role.stager]", `provider = "pi"`)
	assertContains(t, content, "[role.stager]", `model = "gpt-5.4-mini"`)

	// annotation
	if !strings.Contains(content, "cannot serve as the stager") {
		t.Error("gemini config should have stager fallback annotation")
	}
	if !strings.Contains(content, "routed to pi") {
		t.Error("gemini config should mention routed to pi")
	}

	// gemini's message and arbiter
	assertContains(t, content, "[role.message]", `model = "gemini-3.1-flash-lite"`)
	assertContains(t, content, "[role.arbiter]", `model = "gemini-3.5-flash"`)
}

func TestBuildBootstrapConfig_OtherInstalledCommented(t *testing.T) {
	content := buildBootstrapConfig("pi", []string{"pi", "claude"})

	// UNCOMMENTED role blocks are pi's
	assertContains(t, content, "[role.planner]", `model = "gpt-5.4"`)
	assertContains(t, content, "[role.message]", `model = "gpt-5.4-nano"`)

	// claude appears as commented block
	if !strings.Contains(content, "=== claude (installed)") {
		t.Error("missing claude commented block header")
	}
	if !strings.Contains(content, `# provider = "claude"`) {
		t.Error("missing commented claude provider line")
	}
	if !strings.Contains(content, `# model = "haiku"`) {
		t.Error("missing commented claude haiku model")
	}

	// claude's uncommented role blocks should NOT appear (only pi is the target)
	if !strings.Contains(content, `model = "haiku"`) {
		// The commented block should have haiku but NOT as an uncommented role
	}
	// Count uncommented [role.message] — should be exactly 1 (pi's)
	count := strings.Count(content, "\n[role.message]")
	if count != 1 {
		t.Errorf("expected exactly 1 uncommented [role.message], got %d", count)
	}
}

func TestBuildBootstrapConfig_NoInstallFallback(t *testing.T) {
	content := buildBootstrapConfig("pi", nil)

	// Should have the fallback annotation on the provider line
	if !strings.Contains(content, "no built-in agent detected on $PATH") {
		t.Error("missing no-install fallback annotation")
	}
}

func TestBuildBootstrapConfig_ValidTOML(t *testing.T) {
	cases := []struct {
		target    string
		installed []string
	}{
		{"pi", []string{"pi"}},
		{"pi", []string{"pi", "claude"}},
		{"claude", []string{"claude"}},
		{"gemini", nil},
		{"claude", []string{"claude", "pi"}},
		{"agy", []string{"agy", "pi", "claude"}},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("target=%s_installed=%v", tc.target, tc.installed), func(t *testing.T) {
			content := buildBootstrapConfig(tc.target, tc.installed)
			var m map[string]any
			if err := toml.Unmarshal([]byte(content), &m); err != nil {
				t.Errorf("buildBootstrapConfig(%q, %v) produced invalid TOML: %v", tc.target, tc.installed, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// config group (no subcommand → help)
// ---------------------------------------------------------------------------

func TestConfigGroup_NoSubcommandPrintsHelp(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupNoRepo(t)
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (prints help)", err)
	}

	got := buf.String()
	if !strings.Contains(got, "init") {
		t.Error(`help output missing "init" subcommand`)
	}
	if !strings.Contains(got, "path") {
		t.Error(`help output missing "path" subcommand`)
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// assertContains checks that content contains all the specified substrings.
func assertContains(t *testing.T, content string, substrs ...string) {
	t.Helper()
	for _, s := range substrs {
		if !strings.Contains(content, s) {
			t.Errorf("content missing %q", s)
		}
	}
}
