package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/exitcode"
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
	globalDir = filepath.Join(home, "stagecoach")
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
	// Must end with stagecoach/config.toml
	if !strings.HasSuffix(got, filepath.Join("stagecoach", "config.toml")) {
		t.Errorf("config path output = %q, want to end with stagecoach/config.toml", got)
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
// config path — Issue-4 override tests (--config / STAGECOACH_CONFIG honored)
// ---------------------------------------------------------------------------

func TestConfigPath_ConfigFlag_PrintsOverride(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupNoRepo(t)
	t.Setenv("STAGECOACH_CONFIG", "")                  // isolate: this test exercises the FLAG, not the env
	override := filepath.Join(t.TempDir(), "foo.toml") // parent (TempDir) exists

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"--config", override, "config", "path"})

	if err := Execute(context.Background()); err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}
	if got := strings.TrimSpace(out.String()); got != override {
		t.Errorf("config path = %q, want override %q (--config must be honored)", got, override)
	}
}

func TestConfigPath_StagecoachConfigEnv_PrintsOverride(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupNoRepo(t)
	override := filepath.Join(t.TempDir(), "foo.toml")
	t.Setenv("STAGECOACH_CONFIG", override)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "path"})

	if err := Execute(context.Background()); err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}
	if got := strings.TrimSpace(out.String()); got != override {
		t.Errorf("config path = %q, want override %q (STAGECOACH_CONFIG must be honored)", got, override)
	}
}

// ---------------------------------------------------------------------------
// config upgrade — Issue-4 override test (--config honored, global NOT touched)
// ---------------------------------------------------------------------------

func TestConfigUpgrade_ConfigFlag_UpgradesOverride_NotGlobal(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	home, _, _ := setupNoRepo(t)
	t.Setenv("STAGECOACH_CONFIG", "") // isolate: this test exercises the FLAG
	override := filepath.Join(t.TempDir(), "foo.toml")
	if err := os.WriteFile(override, []byte("config_version = 1\n[defaults]\nprovider = \"pi\"\n"), 0o644); err != nil {
		t.Fatalf("write override config: %v", err)
	}

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"--config", override, "config", "upgrade"})

	if err := Execute(context.Background()); err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}
	if !strings.Contains(out.String(), "Upgraded") {
		t.Errorf("stdout = %q, want to contain 'Upgraded'", out.String())
	}

	// The override file must be upgraded
	data, _ := os.ReadFile(override)
	if !strings.Contains(string(data), "config_version = 3") {
		t.Errorf("override file not upgraded; got:\n%s", data)
	}
	if !strings.Contains(string(data), "provider = \"pi\"") {
		t.Errorf("provider = pi not preserved in override file")
	}

	// The global config must NOT have been created
	globalPath := filepath.Join(home, "stagecoach", "config.toml")
	if _, err := os.Stat(globalPath); !os.IsNotExist(err) {
		t.Errorf("global config %s must NOT exist (upgrade must target the --config file only)", globalPath)
	}
}

// ---------------------------------------------------------------------------
// config init — Issue-4 override test (--config honored, writes to override path)
// ---------------------------------------------------------------------------

func TestConfigInit_ConfigFlag_WritesOverride(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	setupNoRepo(t)
	t.Setenv("STAGECOACH_CONFIG", "") // isolate: this test exercises the FLAG
	override := filepath.Join(t.TempDir(), "foo.toml")

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"--config", override, "config", "init"})

	if err := Execute(context.Background()); err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}
	if !strings.Contains(out.String(), "Wrote config to") {
		t.Errorf("stdout = %q, want to contain 'Wrote config to'", out.String())
	}

	// The override file must exist and contain config_version = 3
	data, err := os.ReadFile(override)
	if err != nil {
		t.Fatalf("cannot read override config at %s: %v", override, err)
	}
	if !strings.Contains(string(data), "config_version = 3") {
		t.Errorf("override config missing 'config_version = 3'; got:\n%s", data)
	}
}

// ---------------------------------------------------------------------------
// config path — Issue-4 back-compat test (no override → global path)
// ---------------------------------------------------------------------------

func TestConfigPath_NoOverride_BackCompatGlobal(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupNoRepo(t)
	t.Setenv("STAGECOACH_CONFIG", "") // isolate: no override

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "path"})

	if err := Execute(context.Background()); err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}
	got := strings.TrimSpace(out.String())
	expected := config.GlobalConfigPath()
	if got != expected {
		t.Errorf("config path = %q, want global %q (back-compat)", got, expected)
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

	// Must have uncommented config_version = 3
	if !strings.Contains(content, "config_version = 3") {
		t.Error("populated config missing uncommented config_version = 3")
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
	if !strings.Contains(content, "config_version = 3") {
		t.Error("missing uncommented config_version = 3")
	}

	// [defaults] provider = "pi"
	if !strings.Contains(content, `provider = "pi"`) {
		t.Error("missing provider = \"pi\" in [defaults]")
	}

	// pi's role models (blanked — no sub-provider in bootstrap)
	assertContains(t, content, "[role.planner]", `model = ""`)
	assertContains(t, content, "[role.message]", `model = ""`)
	assertContains(t, content, "[role.stager]", `model = ""`)
	assertContains(t, content, "[role.arbiter]", `model = ""`)
	// NOTE: the negative gpt-5.4 check is intentionally omitted here because the real CLI
	// path may detect other installed providers whose commented blocks legitimately contain gpt-5.4.
	// The uncommented model="" assertions above are sufficient.

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
	rootCmd.SetArgs([]string{"config", "init", "--provider", "agy"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	data, err := os.ReadFile(config.GlobalConfigPath())
	if err != nil {
		t.Fatalf("cannot read config: %v", err)
	}
	content := string(data)

	// [defaults] provider = "agy"
	if !strings.Contains(content, `provider = "agy"`) {
		t.Error("missing provider = \"agy\" in [defaults]")
	}

	// agy is stager-capable (§12.5.1.1 item 4) — fast-by-default planner, NO pi fallback.
	assertContains(t, content, "[role.planner]", `model = "Gemini 3.5 Flash (Low)"`)
	// the ACTIVE stager block carries agy's own mid-tier model (proves agy is the stager, not pi).
	assertContains(t, content, "[role.stager]", `model = "Gemini 3.5 Flash (Medium)"`)
	if strings.Contains(content, "cannot serve as the stager") {
		t.Errorf("agy is stager-capable; config should not carry a stager-fallback annotation")
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

	// The template must be FUNCTIONALLY inert: zero active key=value settings (config.IsInert).
	// [generation] is an intentional exception — an ACTIVE empty table (no uncommented keys) that makes
	// uncommenting a single key land in the right table instead of silently attaching to the preceding
	// section (see config.GenerationSection's doc comment). It holds no settings, so the file is still
	// inert and `config upgrade` on it is a no-op.
	if !config.IsInert(content) {
		t.Errorf("template is not inert (has active key=value settings)")
	}
	// Every table header EXCEPT [generation] must stay commented (documentation style): the inert
	// reference documents options without activating anything. [generation] is the sole active header.
	uncommentedSection := regexp.MustCompile(`^[[a-z]`)
	for i, line := range strings.Split(content, "\n") {
		if line == "[generation]" {
			continue // the one intentional active empty table (foot-gun-safe key attachment)
		}
		if uncommentedSection.MatchString(line) {
			t.Errorf("line %d is an uncommented TOML header: %q (only [generation] may be active)", i+1, line)
		}
	}

	// But the commented headers MUST be present (as guidance)
	for _, section := range []string{"[defaults]", "[generation]", "[provider.pi]", "[provider.myagent]"} {
		if !strings.Contains(content, section) {
			t.Errorf("template missing commented section %q", section)
		}
	}

	// Env-var and git-key docs must be present
	if !strings.Contains(content, "STAGECOACH_PROVIDER") {
		t.Error("template missing STAGECOACH_PROVIDER env-var doc")
	}
	if !strings.Contains(content, "stagecoach.provider") {
		t.Error("template missing stagecoach.provider git-key doc")
	}

	// Regression for Issue 2 (P1.M2.T7.S1): the git-config hint must advertise the SETTABLE
	// camelCase key (stagecoach.autoStageAll). git rejects underscores in the final config-key
	// segment, so shipping `git config stagecoach.auto_stage_all` gives users `error: invalid key`.
	// NOTE: assert on the git-config hint only — the TOML field `auto_stage_all` (snake_case) is
	// correct and remains elsewhere in this file.
	if strings.Contains(content, "git config stagecoach.auto_stage_all") {
		t.Errorf("template advertises un-settable snake_case git key stagecoach.auto_stage_all; use camelCase autoStageAll")
	}
	if !strings.Contains(content, "stagecoach.autoStageAll") {
		t.Errorf("template missing camelCase git key stagecoach.autoStageAll")
	}
}

func TestConfigInit_Template_GenerationKeys(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	setupNoRepo(t)
	tmp := filepath.Join(t.TempDir(), "ref.toml")
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--template", "--config", tmp})

	if err := Execute(context.Background()); err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("cannot read template at %s: %v", tmp, err)
	}
	content := string(data)

	// All 5 v2.1 keys must render as commented documentation lines in the [generation] section.
	for _, key := range []string{"exclude", "format", "locale", "template", "push"} {
		needle := "# " + key
		if !strings.Contains(content, needle) {
			t.Errorf("template [generation] missing commented key %q (needle %q not found)", key, needle)
		}
	}

	// The push line's `git push` code span must render correctly (validates the backtick-split concatenation).
	if !strings.Contains(content, "`git push`") {
		t.Errorf("template push line did not render the `git push` code span (backtick split mis-wired)")
	}
}

// TestConfigInit_TemplateFlag_CollisionSafe verifies the §9.19 FR-F8 cobra collision analysis:
// `config init` keeps its LOCAL bool `--template` (v1 inert-reference-config behavior); the root
// PERSISTENT string `--template` (message template) is skipped for `config init` by pflag's
// AddFlagSet (local name already present) and is unaffected — no panic, no type clash.
func TestConfigInit_TemplateFlag_CollisionSafe(t *testing.T) {
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

	// The LOCAL bool won: it wrote the inert reference config (v1 behavior), not a value-taking flag.
	got, err := configInitCmd.Flags().GetBool("template")
	if err != nil {
		t.Fatalf("config init --template did not parse as a bool: %v", err)
	}
	if !got {
		t.Error("config init --template bool = false, want true")
	}

	// The root persistent string --template is untouched (still its zero value) — no collision.
	if flagTemplate != "" {
		t.Errorf("flagTemplate = %q, want empty (config init's local bool must not leak into the global string)", flagTemplate)
	}
}

// ---------------------------------------------------------------------------
// config init tests -- --force
// ---------------------------------------------------------------------------

func TestConfigInit_Force_PreservesActiveSettings(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, _, globalDir := setupNoRepo(t)
	// Pre-create a hand-tuned config with active settings in proper sections (FR-B8 Issue-3 repro).
	preExisting := `config_version = 3
[defaults]
provider = "claude"
model = "sonnet"
timeout = "60s"
[role.planner]
provider = "agy"
model = "gemini-3.1-pro"
`
	writeConfigFile(t, globalDir, "config.toml", preExisting)

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

	// FR-B8: every active user setting must be PRESERVED verbatim (not clobbered by the pi bootstrap).
	if !strings.Contains(content, `provider = "claude"`) {
		t.Errorf("[defaults] provider=claude was clobbered (FR-B8 violation)\n%s", content)
	}
	if !strings.Contains(content, `model = "sonnet"`) {
		t.Errorf("[defaults] model=sonnet was dropped (FR-B8 violation)\n%s", content)
	}
	if !strings.Contains(content, `timeout = "60s"`) {
		t.Errorf("[defaults] timeout=60s was dropped (FR-B8 violation)\n%s", content)
	}
	if !strings.Contains(content, `provider = "agy"`) {
		t.Errorf("[role.planner] provider=agy was dropped (FR-B8 violation)\n%s", content)
	}
	if !strings.Contains(content, `model = "gemini-3.1-pro"`) {
		t.Errorf("[role.planner] model=gemini-3.1-pro was clobbered (FR-B8 violation)\n%s", content)
	}

	// The fresh template structure is still present (the refresh happened — this is a merge, not a skip).
	if !strings.Contains(content, "config_version = 3") {
		t.Errorf("fresh template missing config_version = 3\n%s", content)
	}

	// FR-B8: a timestamped backup of the prior config must exist alongside the written file.
	matches, _ := filepath.Glob(filepath.Join(globalDir, "config.toml.bak.*"))
	if len(matches) == 0 {
		t.Errorf("no timestamped backup created at %s/*.bak.* (FR-B8 reversible-write guarantee)", globalDir)
	}
}

func TestConfigInit_Force_ReTargetsToPreservedProvider(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, _, globalDir := setupNoRepo(t)
	// Single-backend, stager-capable default — the BUG-001 repro (providerName="" + --force).
	preExisting := `config_version = 3
[defaults]
provider = "claude"
model = "sonnet"
`
	writeConfigFile(t, globalDir, "config.toml", preExisting)

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--force"}) // NO --provider ⇒ providerName==""

	if err := Execute(context.Background()); err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	data, err := os.ReadFile(config.GlobalConfigPath())
	if err != nil {
		t.Fatalf("cannot read config: %v", err)
	}
	content := string(data)

	// BUG-001 core: the stager must NOT be routed to pi (the auto-detected default). Pre-fix,
	// GenerateBootstrapConfig("") wrote an ACTIVE [role.stager] provider = "pi" and
	// MergeActiveSettings could not reconcile it → decompose failed under FR-R5b. The bootstrap
	// template DOES contain commented documentation lines listing pi among providers
	// ("# provider = \"pi\""), so we assert against ACTIVE (uncommented) lines only — the exact
	// BUG-001 symptom is an injected active stager=pi block.
	activePi := regexp.MustCompile(`(?m)^provider = "pi"`)
	if loc := activePi.FindStringIndex(content); loc != nil {
		t.Errorf("BUG-001 regression: an ACTIVE role routes to pi despite preserved default=claude;\nthe [role.stager] block must target claude (or inherit it), never pi.\n%s", content)
	}
	// FR-B8 still holds: the preserved default is carried verbatim.
	if !strings.Contains(content, `provider = "claude"`) {
		t.Errorf("[defaults] provider=claude was not preserved (FR-B8 violation)\n%s", content)
	}
}

func TestConfigInit_Force_Template_PreservesActiveSettings(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, _, globalDir := setupNoRepo(t)
	// Pre-create a hand-tuned config with active settings (the inert template has no active lines,
	// so FR-B8 carries these in as new [section] blocks appended to the refreshed template).
	preExisting := `config_version = 3
[defaults]
provider = "claude"
model = "sonnet"
`
	writeConfigFile(t, globalDir, "config.toml", preExisting)

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

	// FR-B8: the user's active settings survive even an inert-template refresh.
	if !strings.Contains(content, `provider = "claude"`) {
		t.Errorf("[defaults] provider=claude was dropped by --template refresh (FR-B8)\n%s", content)
	}
	if !strings.Contains(content, `model = "sonnet"`) {
		t.Errorf("[defaults] model=sonnet was dropped by --template refresh (FR-B8)\n%s", content)
	}

	// FR-B8: a timestamped backup of the prior config must exist.
	matches, _ := filepath.Glob(filepath.Join(globalDir, "config.toml.bak.*"))
	if len(matches) == 0 {
		t.Errorf("no timestamped backup created (FR-B8 reversible-write guarantee)")
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
	// Finding 1 regression guard: the "use a built-in" hint must list ALL built-ins. The hint
	// sources the registry's PreferredBuiltins() — single source of truth.
	for _, name := range []string{"pi", "opencode", "cursor", "agy", "codex", "claude"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error message %q should list built-in %q in the hint", err.Error(), name)
		}
	}
}

func TestConfigInit_MkdirAllParent(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, _, globalDir := setupNoRepo(t)
	// The parent dir (<home>/stagecoach) should NOT exist yet (loadEnvSetup doesn't create it)
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
	if !strings.Contains(got, "upgrade") {
		t.Error(`help output missing "upgrade" subcommand (registration — configCmd.AddCommand(configUpgradeCmd))`)
	}
	if strings.Contains(got, "Subcommands:") {
		t.Error(`help output must NOT contain a manual "Subcommands:" block (FR-B6: cobra "Available Commands" is the single source)`)
	}
	if !strings.Contains(got, "Available Commands:") {
		t.Error(`help output missing cobra "Available Commands:" (FR-B6 single source)`)
	}
}

// ---------------------------------------------------------------------------
// config lifecycle test — init → upgrade end-to-end
// ---------------------------------------------------------------------------

func TestConfigLifecycle_InitThenUpgrade(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupNoRepo(t)

	// (1) config init — populated bootstrap (no --template, no --provider → auto-detect/default "pi")
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init"})
	if err := Execute(context.Background()); err != nil {
		t.Fatalf("config init err=%v, want nil", err)
	}

	// (2) the written config is POPULATED and carries the current schema version
	path := config.GlobalConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "config_version = 3") {
		t.Errorf("populated config missing 'config_version = 3' (GenerateBootstrapConfig must write CurrentConfigVersion);\ngot:\n%s", content)
	}
	// populated (NOT inert): it has at least one uncommented [defaults] or [role. block
	if !strings.Contains(content, "[defaults]") && !strings.Contains(content, "[role.") {
		t.Errorf("populated config appears inert (no uncommented [defaults]/[role.*]);\ngot:\n%s", content)
	}

	// (3) config upgrade on the fresh-init'd config → already current, byte-identical
	preContent := content
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "upgrade"})
	if err := Execute(context.Background()); err != nil {
		t.Fatalf("config upgrade err=%v, want nil (already-current is success)", err)
	}
	if !strings.Contains(out.String(), "no changes") && !strings.Contains(out.String(), "already") {
		t.Errorf("upgrade stdout=%q, want 'already up to date'/'no changes'", out.String())
	}
	afterContent, _ := os.ReadFile(path)
	if string(afterContent) != preContent {
		t.Errorf("config file changed after an already-current upgrade (must be byte-identical)")
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

// ---------------------------------------------------------------------------
// upgradeConfigVersion — pure unit tests (no Execute, no filesystem)
// ---------------------------------------------------------------------------

func TestUpgradeConfigVersion_NoVersion_Inserts(t *testing.T) {
	input := "# comment\n[defaults]\nprovider = \"pi\"\n"
	result, changed := upgradeConfigVersion(input, config.CurrentConfigVersion)
	if !changed {
		t.Fatal("expected changed=true")
	}
	// config_version = 3 must be present before the table
	if !strings.Contains(result, "config_version = 3") {
		t.Error("missing config_version = 3")
	}
	// Other lines must be byte-identical
	if !strings.Contains(result, "[defaults]\nprovider = \"pi\"\n") {
		t.Error("original lines not preserved")
	}
	// Inserted BEFORE the first table
	lines := strings.Split(result, "\n")
	var versionIdx, tableIdx int
	for i, l := range lines {
		if l == "config_version = 3" {
			versionIdx = i
		}
		if l == "[defaults]" {
			tableIdx = i
		}
	}
	if versionIdx == 0 {
		t.Fatal("config_version not found")
	}
	if tableIdx == 0 {
		t.Fatal("[defaults] not found")
	}
	if versionIdx >= tableIdx {
		t.Errorf("config_version (line %d) should be before [defaults] (line %d)", versionIdx, tableIdx)
	}
}

func TestUpgradeConfigVersion_OlderVersion_Updates(t *testing.T) {
	input := "config_version = 1\n[defaults]\nprovider = \"pi\"\n"
	result, changed := upgradeConfigVersion(input, config.CurrentConfigVersion)
	if !changed {
		t.Fatal("expected changed=true")
	}
	if !strings.HasPrefix(result, "config_version = 3\n") {
		t.Errorf("first line = %q, want config_version = 3", strings.Split(result, "\n")[0])
	}
	// Other lines preserved
	if !strings.Contains(result, "[defaults]\nprovider = \"pi\"\n") {
		t.Error("original lines not preserved")
	}
}

func TestUpgradeConfigVersion_CurrentVersion_NoChange(t *testing.T) {
	input := "config_version = 3\n[defaults]\nprovider = \"pi\"\n"
	result, changed := upgradeConfigVersion(input, config.CurrentConfigVersion)
	if changed {
		t.Fatal("expected changed=false (already current)")
	}
	// Byte-identical
	if result != input {
		t.Errorf("content changed (length %d vs %d)", len(result), len(input))
	}
}

func TestUpgradeConfigVersion_CommentedVersionIgnored(t *testing.T) {
	input := "# config_version = 1\n[defaults]\nprovider = \"pi\"\n"
	result, changed := upgradeConfigVersion(input, config.CurrentConfigVersion)
	if !changed {
		t.Fatal("expected changed=true (commented version is not the schema key)")
	}
	if !strings.Contains(result, "config_version = 3") {
		t.Error("missing inserted config_version = 3")
	}
	// The original comment is preserved
	if !strings.Contains(result, "# config_version = 1") {
		t.Error("original comment not preserved")
	}
}

func TestUpgradeConfigVersion_VersionInTableNotMatched(t *testing.T) {
	input := "[defaults]\nconfig_version = 1\nprovider = \"pi\"\n"
	result, changed := upgradeConfigVersion(input, config.CurrentConfigVersion)
	if !changed {
		t.Fatal("expected changed=true (config_version after table is not top-level)")
	}
	// Should have inserted a top-level config_version
	if !strings.Contains(result, "config_version = 3") {
		t.Error("missing inserted config_version = 3")
	}
	// The old line inside [defaults] is preserved
	if !strings.Contains(result, "config_version = 1") {
		t.Error("config_version inside table should be preserved")
	}
	// The result must be valid TOML with root config_version = 3
	var m map[string]any
	if err := toml.Unmarshal([]byte(result), &m); err != nil {
		t.Fatalf("result is not valid TOML: %v", err)
	}
	if cv, ok := m["config_version"]; !ok || cv != int64(config.CurrentConfigVersion) {
		t.Errorf("root config_version = %v, want %d", cv, config.CurrentConfigVersion)
	}
}

func TestUpgradeConfigVersion_Idempotent(t *testing.T) {
	input := "[defaults]\nprovider = \"pi\"\n"
	result1, changed1 := upgradeConfigVersion(input, config.CurrentConfigVersion)
	if !changed1 {
		t.Fatal("first call should change")
	}
	result2, changed2 := upgradeConfigVersion(result1, config.CurrentConfigVersion)
	if changed2 {
		t.Fatal("second call should NOT change (idempotent)")
	}
	if result2 != result1 {
		t.Errorf("2nd call changed content (length %d vs %d)", len(result2), len(result1))
	}
}

// ---------------------------------------------------------------------------
// config upgrade — Execute-driven tests
// ---------------------------------------------------------------------------

func TestConfigUpgrade_NoFile_Errors(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupNoRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "upgrade"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (no file)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "config init") {
		t.Errorf("error %q should mention 'config init'", err.Error())
	}
}

func TestConfigUpgrade_AddsVersion(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	_, _, globalDir := setupNoRepo(t)
	writeConfigFile(t, globalDir, "config.toml", "[defaults]\nprovider = \"pi\"\n")

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "upgrade"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}
	if !strings.Contains(out.String(), "Upgraded") {
		t.Errorf("stdout = %q, want to contain 'Upgraded'", out.String())
	}

	data, _ := os.ReadFile(config.GlobalConfigPath())
	content := string(data)
	if !strings.Contains(content, "config_version = 3") {
		t.Error("missing config_version = 3")
	}
	if !strings.Contains(content, "provider = \"pi\"") {
		t.Error("user value 'provider = pi' not preserved")
	}
}

func TestConfigUpgrade_AlreadyCurrent(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	_, _, globalDir := setupNoRepo(t)
	writeConfigFile(t, globalDir, "config.toml", "config_version = 3\n[defaults]\nprovider = \"pi\"\n")
	preContent, _ := os.ReadFile(config.GlobalConfigPath())

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "upgrade"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}
	if !strings.Contains(out.String(), "no changes") {
		t.Errorf("stdout = %q, want to contain 'no changes'", out.String())
	}

	afterContent, _ := os.ReadFile(config.GlobalConfigPath())
	if string(afterContent) != string(preContent) {
		t.Error("file was rewritten (should be byte-identical — already current)")
	}
}

func TestConfigUpgrade_OlderUpdated(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	_, _, globalDir := setupNoRepo(t)
	writeConfigFile(t, globalDir, "config.toml", "config_version = 1\n[generation]\nmax_md_lines = 7\n")

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "upgrade"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}
	if !strings.Contains(out.String(), "Upgraded") {
		t.Errorf("stdout = %q, want to contain 'Upgraded'", out.String())
	}

	data, _ := os.ReadFile(config.GlobalConfigPath())
	content := string(data)
	if !strings.Contains(content, "config_version = 3") {
		t.Error("missing config_version = 3")
	}
	if !strings.Contains(content, "max_md_lines = 7") {
		t.Error("max_md_lines = 7 not preserved")
	}

	// FR-B8: a timestamped backup of the prior (v1) config must exist alongside the upgraded file.
	matches, _ := filepath.Glob(filepath.Join(globalDir, "config.toml.bak.*"))
	if len(matches) == 0 {
		t.Errorf("no timestamped backup created after config upgrade (FR-B8)")
	} else {
		bak, _ := os.ReadFile(matches[0])
		if !strings.Contains(string(bak), "config_version = 1") {
			t.Errorf("backup does not hold prior (v1) content; got:\n%s", bak)
		}
	}
}

func TestConfigUpgrade_Idempotent(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	_, _, globalDir := setupNoRepo(t)
	writeConfigFile(t, globalDir, "config.toml", "[defaults]\nprovider = \"pi\"\n")

	// First run
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "upgrade"})
	if err := Execute(context.Background()); err != nil {
		t.Fatalf("1st Execute err=%v", err)
	}
	firstContent, _ := os.ReadFile(config.GlobalConfigPath())

	// Second run (must reset state)
	rootCmd.SetArgs(nil)
	resetFlags(rootCmd.Flags())
	resetFlags(rootCmd.PersistentFlags())

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "upgrade"})
	if err := Execute(context.Background()); err != nil {
		t.Fatalf("2nd Execute err=%v", err)
	}
	if !strings.Contains(out.String(), "no changes") {
		t.Errorf("2nd run stdout = %q, want 'no changes'", out.String())
	}

	secondContent, _ := os.ReadFile(config.GlobalConfigPath())
	if string(secondContent) != string(firstContent) {
		t.Error("file changed on 2nd run (should be idempotent)")
	}
}

func TestConfigUpgrade_MalformedTOML(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	_, _, globalDir := setupNoRepo(t)
	writeConfigFile(t, globalDir, "config.toml", "bad {toml\n")

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "upgrade"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (malformed TOML)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "not valid TOML") {
		t.Errorf("error %q should contain 'not valid TOML'", err.Error())
	}

	// File must be UNCHANGED
	data, _ := os.ReadFile(config.GlobalConfigPath())
	if string(data) != "bad {toml\n" {
		t.Error("file was modified (should be untouched — malformed TOML)")
	}
}

func TestConfigUpgrade_InertFile_NoOp(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	_, _, globalDir := setupNoRepo(t)
	// All-commented inert file (mirrors `config init --template` output — zero active settings).
	inert := "# All-commented reference template (inert)\n" +
		"# config_version = 3\n" +
		"# [defaults]\n" +
		"# provider = \"pi\"\n"
	writeConfigFile(t, globalDir, "config.toml", inert)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "upgrade"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (inert file is a no-op success)", err)
	}
	// FR-B9 corollary: report "inert — nothing to upgrade", NOT "Upgraded".
	if !strings.Contains(out.String(), "inert") || !strings.Contains(out.String(), "nothing to upgrade") {
		t.Errorf("stdout = %q, want to contain 'inert' + 'nothing to upgrade'", out.String())
	}
	if strings.Contains(out.String(), "Upgraded") {
		t.Errorf("stdout = %q, must NOT say 'Upgraded' (inert file is a no-op)", out.String())
	}

	// The file must be byte-identical (no stray config_version appended).
	after, _ := os.ReadFile(config.GlobalConfigPath())
	if string(after) != inert {
		t.Errorf("inert file was modified (should be byte-identical): before=%q after=%q", inert, after)
	}
}

func TestConfigUpgrade_ExtraArgsExits1(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupNoRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "upgrade", "x"})

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
// upgradeConfigVersion — on-disk →v3 rewrite pure unit tests (ADDITIVE)
// ---------------------------------------------------------------------------

// TestUpgradeConfigVersion_V3Rewrite exercises the on-disk →v3 rewrite via the pure function (target=3).
// Sub-tests pin each FR-B7 guarantee: fold (provider default_model / global model / per-role model / role
// inheriting global), single-backend untouched, comment-out, agent rename, idempotency, no-invent, version bump.
func TestUpgradeConfigVersion_V3Rewrite(t *testing.T) {
	t.Run("folds provider default_model + comments out default_provider + bumps version", func(t *testing.T) {
		input := "config_version = 2\n" +
			"\n" +
			"[provider.pi]\n" +
			"default_provider = \"anthropic\"\n" +
			"default_model = \"claude-haiku\"\n" +
			"provider_flag = \"--provider\"\n"
		got, changed := upgradeConfigVersion(input, 3)
		if !changed {
			t.Fatal("changed=false, want true (v2 → v3 rewrite)")
		}
		if !strings.Contains(got, "default_model = \"anthropic/claude-haiku\"") {
			t.Errorf("default_model not folded:\n%s", got)
		}
		if strings.Contains(got, "\ndefault_provider = \"anthropic\"\n") {
			t.Errorf("default_provider still active (should be commented out):\n%s", got)
		}
		if !strings.Contains(got, "# default_provider = \"anthropic\"") {
			t.Errorf("default_provider not commented out with note:\n%s", got)
		}
		if !strings.HasPrefix(got, "config_version = 3\n") {
			t.Errorf("config_version not bumped to 3:\n%s", got)
		}
		// The result must be valid TOML.
		var m map[string]any
		if err := toml.Unmarshal([]byte(got), &m); err != nil {
			t.Fatalf("upgraded output is not valid TOML: %v\n%s", err, got)
		}
	})

	t.Run("folds global [defaults] model when global provider is multi-backend", func(t *testing.T) {
		input := "config_version = 2\n" +
			"[defaults]\n" +
			"provider = \"pi\"\n" +
			"model = \"claude-haiku\"\n" +
			"\n" +
			"[provider.pi]\n" +
			"default_provider = \"anthropic\"\n"
		got, _ := upgradeConfigVersion(input, 3)
		if !strings.Contains(got, "model = \"anthropic/claude-haiku\"") {
			t.Errorf("global model not folded:\n%s", got)
		}
	})

	t.Run("folds per-role model (explicit role provider) and role inheriting global", func(t *testing.T) {
		input := "config_version = 2\n" +
			"[defaults]\n" +
			"provider = \"pi\"\n" +
			"\n" +
			"[role.planner]\n" +
			"provider = \"pi\"\n" +
			"model = \"claude-haiku\"\n" +
			"\n" +
			"[role.message]\n" + // no provider → inherits global pi
			"model = \"claude-haiku\"\n" +
			"\n" +
			"[provider.pi]\n" +
			"default_provider = \"anthropic\"\n"
		got, _ := upgradeConfigVersion(input, 3)
		// Both role models must be prefixed (one explicit provider, one inherited).
		if c := strings.Count(got, "model = \"anthropic/claude-haiku\""); c != 2 {
			t.Errorf("expected 2 folded role models, got %d:\n%s", c, got)
		}
	})

	t.Run("single-backend provider: default_provider commented out, model NOT prefixed", func(t *testing.T) {
		input := "config_version = 2\n" +
			"[provider.claude]\n" +
			"default_provider = \"anthropic\"\n" +
			"default_model = \"opus\"\n"
		got, _ := upgradeConfigVersion(input, 3)
		if !strings.Contains(got, "# default_provider = \"anthropic\"") {
			t.Errorf("single-backend default_provider not commented out:\n%s", got)
		}
		if strings.Contains(got, "\"anthropic/opus\"") {
			t.Errorf("single-backend model must NOT be prefixed:\n%s", got)
		}
		if !strings.Contains(got, "default_model = \"opus\"") {
			t.Errorf("single-backend default_model must be unchanged:\n%s", got)
		}
	})

	t.Run("agent table header renamed to provider", func(t *testing.T) {
		input := "config_version = 2\n" +
			"[agent.pi]\n" +
			"default_provider = \"anthropic\"\n" +
			"default_model = \"claude-haiku\"\n"
		got, _ := upgradeConfigVersion(input, 3)
		if strings.Contains(got, "[agent.pi]") {
			t.Errorf("[agent.pi] not renamed:\n%s", got)
		}
		if !strings.Contains(got, "[provider.pi]") {
			t.Errorf("missing [provider.pi]:\n%s", got)
		}
		if !strings.Contains(got, "default_model = \"anthropic/claude-haiku\"") {
			t.Errorf("default_model not folded after rename:\n%s", got)
		}
	})

	t.Run("idempotent: a v3 file is a no-op", func(t *testing.T) {
		v3 := "config_version = 3\n[provider.pi]\ndefault_model = \"anthropic/claude-haiku\"\n"
		got, changed := upgradeConfigVersion(v3, 3)
		if changed {
			t.Errorf("a v3 file must be a no-op; got changed=true:\n%s", got)
		}
		if got != v3 {
			t.Errorf("a v3 file must be byte-unchanged; got:\n%s", got)
		}
	})

	t.Run("bare pi model with NO default_provider stays bare (no-invent)", func(t *testing.T) {
		input := "config_version = 2\n[provider.pi]\ndefault_model = \"claude-haiku\"\n"
		got, _ := upgradeConfigVersion(input, 3)
		if !strings.Contains(got, "default_model = \"claude-haiku\"") {
			t.Errorf("a bare model with no default_provider must stay bare:\n%s", got)
		}
		if strings.Contains(got, "/claude-haiku\"") {
			t.Errorf("a prefix was invented (no default_provider to fold):\n%s", got)
		}
	})
}

// TestConfigUpgrade_V2ToV3Rewrite is the COMMAND round-trip: write a v2 file, run `config upgrade`, assert the
// on-disk result (prefixed model + commented default_provider + config_version=3); re-run → no change.
// Mirrors the TestConfigUpgrade_AddsVersion harness (temp home + writeConfigFile + Execute).
func TestConfigUpgrade_V2ToV3Rewrite(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	_, _, globalDir := setupNoRepo(t)
	globalPath := filepath.Join(globalDir, "config.toml")
	v2 := "config_version = 2\n" +
		"\n" +
		"[defaults]\n" +
		"provider = \"pi\"\n" +
		"model = \"claude-haiku\"\n" +
		"\n" +
		"[provider.pi]\n" +
		"default_provider = \"anthropic\"\n" +
		"provider_flag = \"--provider\"\n"
	writeConfigFile(t, globalDir, "config.toml", v2)

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "upgrade"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("config upgrade failed: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "Upgraded config") {
		t.Errorf("expected upgrade confirmation; got:\n%s", out.String())
	}

	data, rerr := os.ReadFile(globalPath)
	if rerr != nil {
		t.Fatal(rerr)
	}
	upgraded := string(data)
	if !strings.Contains(upgraded, "model = \"anthropic/claude-haiku\"") {
		t.Errorf("on-disk global model not folded:\n%s", upgraded)
	}
	if !strings.Contains(upgraded, "# default_provider = \"anthropic\"") {
		t.Errorf("on-disk default_provider not commented out:\n%s", upgraded)
	}
	if !strings.Contains(upgraded, "config_version = 3") {
		t.Errorf("on-disk config_version not 3:\n%s", upgraded)
	}

	// FR-B8: a timestamped backup of the prior (v2) config must exist alongside the upgraded file.
	matches, _ := filepath.Glob(filepath.Join(globalDir, "config.toml.bak.*"))
	if len(matches) == 0 {
		t.Errorf("no timestamped backup created after config upgrade (FR-B8)")
	} else {
		bak, _ := os.ReadFile(matches[0])
		if string(bak) != v2 {
			t.Errorf("backup does not hold prior (v2) content; got:\n%s", bak)
		}
	}

	// Re-run → no change (idempotent).
	out.Reset()
	rootCmd.SetArgs(nil)
	resetFlags(rootCmd.Flags())
	resetFlags(rootCmd.PersistentFlags())

	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "upgrade"})
	if err2 := Execute(context.Background()); err2 != nil {
		t.Fatalf("second config upgrade failed: %v\n%s", err2, out.String())
	}
	if !strings.Contains(out.String(), "already at version 3") && !strings.Contains(out.String(), "no changes") {
		t.Errorf("second run should be a no-op; got:\n%s", out.String())
	}
	data2, _ := os.ReadFile(globalPath)
	if string(data2) != upgraded {
		t.Errorf("second run changed the file (not idempotent)")
	}

	// FR-B8: the idempotent re-run must NOT create a second backup (changed==false returns before the backup block).
	matches2, _ := filepath.Glob(filepath.Join(globalDir, "config.toml.bak.*"))
	if len(matches2) != 1 {
		t.Errorf("expected exactly 1 backup after idempotent re-run, got %d (FR-B8 no-op gate)", len(matches2))
	}
}

// TestPreservedDefaultProvider covers preservedDefaultProvider — the BUG-001 (FR-B2/FR-B8) helper that
// extracts the active [defaults] provider from an existing config and validates it against the built-in
// registry, returning "" (auto-detect) for absent/unreadable/inert/no-provider/wrong-section/custom
// files and the built-in name for a known built-in. S1 delivers the helper + this test; S2 wires it into
// runConfigInit. ActiveSettings stores values verbatim (with quotes), so the helper unquotes before
// validating — the "known built-in" rows prove that unquoting works (a quoted value validated directly
// would be reported as unknown).
func TestPreservedDefaultProvider(t *testing.T) {
	cases := []struct {
		name   string
		body   string // empty + absent=true ⇒ the file is not written (read-error path)
		absent bool
		want   string
	}{
		{name: "absent file", absent: true, want: ""},                              // read error → ""
		{name: "inert all-commented", body: "# provider = \"claude\"\n", want: ""}, // no active setting → ""
		{name: "known built-in claude", body: "[defaults]\nprovider = \"claude\"\n", want: "claude"},
		{name: "known built-in codex", body: "[defaults]\nprovider = \"codex\"\n", want: "codex"},
		{name: "known built-in pi", body: "[defaults]\nprovider = \"pi\"\n", want: "pi"},
		{name: "custom unknown provider", body: "[defaults]\nprovider = \"myagent\"\n", want: ""},       // don't break custom
		{name: "provider under wrong section", body: "[generation]\nprovider = \"claude\"\n", want: ""}, // only [defaults] counts
		{name: "no provider key", body: "[defaults]\nmodel = \"sonnet\"\n", want: ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "c.toml")
			if !tc.absent {
				if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := preservedDefaultProvider(path); got != tc.want {
				t.Errorf("preservedDefaultProvider(%q) = %q; want %q", tc.body, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// config init --local — repo-local config generation (./.stagecoach.toml)
// ---------------------------------------------------------------------------

// TestTemplates_SharedGenerationSection proves the [generation] documentation is UNIFIED: BOTH the
// populated bootstrap config and the inert --template reference embed the SAME config.GenerationSection
// block, so neither can drift to a different key subset (the original bug: the populated template
// lacked format/template/locale/push/exclude while the inert one lacked token_limit/diff_context/
// multi_turn_*/no_parent_watchdog). The ONE intentional difference: token_limit is UNCOMMENTED
// (active, = 50000) in the populated config and COMMENTED in the inert reference (which stays inert).
func TestTemplates_SharedGenerationSection(t *testing.T) {
	populated := config.GenerateBootstrapConfig("pi")

	// The inert reference embeds GenerationSection VERBATIM (every key commented ⇒ inert). The populated
	// config embeds the SAME block with exactly one line flipped: token_limit is uncommented.
	if !strings.Contains(exampleConfigTemplate, config.GenerationSection) {
		t.Errorf("inert --template reference does not embed config.GenerationSection (drift)")
	}
	populatedGen := strings.Replace(config.GenerationSection, "\n# token_limit", "\ntoken_limit", 1)
	if !strings.Contains(populated, populatedGen) {
		t.Errorf("populated config does not embed GenerationSection (with token_limit uncommented)")
	}

	// token_limit: ACTIVE (= 50000) in the populated config; COMMENTED in the inert reference.
	activeTL := regexp.MustCompile(`(?m)^token_limit\s*=\s*50000\b`)
	if !activeTL.MatchString(populated) {
		t.Errorf("populated config missing ACTIVE token_limit = 50000 (should be uncommented)")
	}
	commentedTL := regexp.MustCompile(`(?m)^#\s*token_limit\s*=\s*50000\b`)
	if !commentedTL.MatchString(exampleConfigTemplate) {
		t.Errorf("inert template missing commented token_limit = 50000")
	}
	if regexp.MustCompile(`(?m)^token_limit\s*=`).MatchString(exampleConfigTemplate) {
		t.Errorf("inert template must NOT have an active token_limit (would break inertness)")
	}

	// Every OTHER [generation] key is documented (commented) in BOTH (token_limit is handled above).
	for _, key := range []string{
		"max_diff_bytes", "max_md_lines", "diff_context",
		"max_duplicate_retries", "subject_target_chars", "output", "strip_code_fence",
		"max_commits", "binary_extensions", "exclude", "multi_turn_fallback",
		"multi_turn_chunk_tokens", "no_parent_watchdog", "format", "locale", "template", "push",
	} {
		needle := "# " + key
		if !strings.Contains(populated, needle) {
			t.Errorf("populated config [generation] missing documented key %q", key)
		}
		if !strings.Contains(exampleConfigTemplate, needle) {
			t.Errorf("inert template [generation] missing documented key %q", key)
		}
	}
}

// TestConfigInit_Local_WritesRepoLocalConfig: `config init --local` writes a POPULATED config to
// ./.stagecoach.toml in CWD (not the global path), with the header scope rewritten to REPO-LOCAL
// framing and the same body as the global populated config.
func TestConfigInit_Local_WritesRepoLocalConfig(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, plainDir, globalDir := setupNoRepo(t)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--local"})

	if err := Execute(context.Background()); err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	localPath := filepath.Join(plainDir, ".stagecoach.toml")
	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("repo-local config not written to %s: %v", localPath, err)
	}
	if !strings.Contains(out.String(), "Wrote repo-local config to .stagecoach.toml") {
		t.Errorf("stdout = %q, want to contain 'Wrote repo-local config to .stagecoach.toml'", out.String())
	}

	content := string(data)
	// Header scope rewritten to REPO-LOCAL framing.
	if !strings.Contains(content, "This is the REPO-LOCAL config") {
		t.Errorf("repo-local config header not rewritten (still GLOBAL framing)")
	}
	if strings.Contains(content, "This is the GLOBAL file") {
		t.Errorf("repo-local config header still says \"This is the GLOBAL file\"")
	}
	// Still a valid populated config.
	if !strings.Contains(content, "config_version = 3") {
		t.Errorf("repo-local config missing config_version = 3")
	}
	var m map[string]any
	if err := toml.Unmarshal([]byte(content), &m); err != nil {
		t.Errorf("repo-local config is not valid TOML: %v", err)
	}

	// The GLOBAL config must NOT have been created (--local targets the repo file only).
	globalPath := filepath.Join(globalDir, "config.toml")
	if _, err := os.Stat(globalPath); !os.IsNotExist(err) {
		t.Errorf("global config %s must NOT exist (--local targets the repo file only)", globalPath)
	}
}

// TestConfigInit_Local_Template_WritesRepoLocalExample: `config init --local --template` writes the
// inert reference to ./.stagecoach.toml (still inert, scope-rewritten, documents the unified keys).
func TestConfigInit_Local_Template_WritesRepoLocalExample(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, plainDir, _ := setupNoRepo(t)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--template", "--local"})

	if err := Execute(context.Background()); err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}
	localPath := filepath.Join(plainDir, ".stagecoach.toml")
	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("repo-local example config not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(out.String(), "Wrote repo-local example config to .stagecoach.toml") {
		t.Errorf("stdout = %q, want 'Wrote repo-local example config to .stagecoach.toml'", out.String())
	}
	if !strings.Contains(content, "This is the REPO-LOCAL config") {
		t.Errorf("repo-local example config header not rewritten")
	}
	if !config.IsInert(content) {
		t.Errorf("repo-local example config is not inert (config.IsInert)")
	}
	// Both v2.1 keys (format/template/locale/push/exclude) AND the v2 tuning keys are documented now.
	for _, key := range []string{"format", "template", "locale", "push", "exclude", "token_limit", "diff_context"} {
		if !strings.Contains(content, "# "+key) {
			t.Errorf("repo-local example config missing documented key %q", key)
		}
	}
}

// TestConfigInit_Local_ConfigFlag_MutuallyExclusive: --local and --config are mutually exclusive
// (--local fixes the target to ./.stagecoach.toml; --config sets an arbitrary path).
func TestConfigInit_Local_ConfigFlag_MutuallyExclusive(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	setupNoRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"--config", filepath.Join(t.TempDir(), "x.toml"), "config", "init", "--local"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (--local + --config mutually exclusive)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "one or the other") {
		t.Errorf("error %q should explain the mutual exclusion", err.Error())
	}
}

// TestConfigInit_Local_Force_PreservesAndBacksUp: --force refresh of a repo-local config preserves
// the user's active settings (FR-B8 merge), backs up the prior file, AND rewrites the header scope.
func TestConfigInit_Local_Force_PreservesAndBacksUp(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

	_, plainDir, _ := setupNoRepo(t)
	preExisting := "config_version = 3\n[defaults]\nprovider = \"claude\"\n"
	if err := os.WriteFile(filepath.Join(plainDir, ".stagecoach.toml"), []byte(preExisting), 0o644); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"config", "init", "--local", "--force", "--provider", "pi"})

	if err := Execute(context.Background()); err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}
	data, _ := os.ReadFile(filepath.Join(plainDir, ".stagecoach.toml"))
	content := string(data)
	// FR-B8: the user's active setting survives the --force refresh.
	if !strings.Contains(content, `provider = "claude"`) {
		t.Errorf("repo-local active setting provider=claude dropped by --force (FR-B8)\n%s", content)
	}
	// FR-B8: a timestamped backup of the prior repo-local config exists in CWD.
	matches, _ := filepath.Glob(filepath.Join(plainDir, ".stagecoach.toml.bak.*"))
	if len(matches) == 0 {
		t.Errorf("no timestamped backup of prior repo-local config (FR-B8 reversible-write guarantee)")
	}
	// Header scope rewritten even after the merge.
	if !strings.Contains(content, "This is the REPO-LOCAL config") {
		t.Errorf("repo-local --force header not rewritten to local scope")
	}
}
