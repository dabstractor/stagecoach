package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/exitcode"
	"github.com/dabstractor/stagecoach/internal/provider"
	"github.com/dabstractor/stagecoach/internal/stubtest"
)

// ---------------------------------------------------------------------------
// Golden renderer test — deterministic, no PATH juggling
// ---------------------------------------------------------------------------

func TestModels_CuratedGolden(t *testing.T) {
	reg := provider.NewRegistry(nil)
	m, ok := reg.Get("claude")
	if !ok {
		t.Fatal("claude not found in registry")
	}

	var buf bytes.Buffer
	printCuratedTable(&buf, modelTarget{name: "claude", manifest: m})

	got := buf.String()
	// Assert fixed role order and expected model values for claude
	substrings := []string{
		"claude:",
		"  planner  haiku",  // fast tier (FR-D3 fast-by-default)
		"  stager   sonnet", // mid tier — tool-use reliability
		"  message  haiku",  // fast tier (highest-volume role)
		"  arbiter  haiku",  // fast tier
		"verified 2026-07-09",
		"consult `claude --help`",
	}
	for _, sub := range substrings {
		if !strings.Contains(got, sub) {
			t.Errorf("curated table output missing %q\nGot:\n%s", sub, got)
		}
	}
}

func TestModels_CuratedGolden_UserDefined(t *testing.T) {
	// A user-defined provider has no FR-D4 column — prints informational message
	m := provider.Manifest{
		Name:    "myagent",
		Command: strPtrUnexported("/opt/myagent"),
	}
	var buf bytes.Buffer
	printCuratedTable(&buf, modelTarget{name: "myagent", manifest: m})

	got := buf.String()
	if !strings.Contains(got, "myagent:") {
		t.Error("output missing 'myagent:'")
	}
	if !strings.Contains(got, "no list_models_command and no curated per-role defaults") {
		t.Errorf("expected informational message for user-defined provider\nGot:\n%s", got)
	}
}

func TestModels_CuratedGolden_VerificationDate(t *testing.T) {
	// Verify the constant matches what's printed
	if config.DefaultModelsVerificationDate != "2026-07-09" {
		t.Errorf("DefaultModelsVerificationDate = %q, want %q", config.DefaultModelsVerificationDate, "2026-07-09")
	}
}

// ---------------------------------------------------------------------------
// Live list renderer test
// ---------------------------------------------------------------------------

func TestModels_PrintLiveList(t *testing.T) {
	var buf bytes.Buffer
	printLiveList(&buf, "pi", "gpt-5.4\ngpt-5.4-mini\n")

	got := buf.String()
	if !strings.Contains(got, "pi:") {
		t.Error("missing 'pi:' heading")
	}
	if !strings.Contains(got, "gpt-5.4") {
		t.Error("missing model line")
	}
}

func TestModels_PrintLiveList_Empty(t *testing.T) {
	var buf bytes.Buffer
	printLiveList(&buf, "test", "")

	got := buf.String()
	if !strings.Contains(got, "(no models reported)") {
		t.Errorf("expected '(no models reported)' for empty stdout\nGot:\n%s", got)
	}
}

func TestModels_PrintLiveList_NoTrailingNewline(t *testing.T) {
	var buf bytes.Buffer
	printLiveList(&buf, "test", "single-line")

	got := buf.String()
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("expected trailing newline for block separation\nGot:\n%q", got)
	}
}

// ---------------------------------------------------------------------------
// Stub-binary live-list test
// ---------------------------------------------------------------------------

// placeStubProvider builds the cross-platform stub CLI (cmd/stubcli), installs it
// into a fresh temp dir under the given provider name (opencode/pi/myagent-bin/...),
// and prepends that dir to PATH, applying o via STAGECOACH_STUBCLI_* env vars. It
// replaces the historical #!/bin/sh stub scripts that cannot run on Windows
// (CreateProcess ignores the shebang). Returns the temp dir (mostly for symmetry).
func placeStubProvider(t *testing.T, name string, o stubtest.CLIOptions) string {
	t.Helper()
	dir := t.TempDir()
	for _, kv := range stubtest.PlaceCLI(t, dir, name, o) {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				t.Setenv(kv[:i], kv[i+1:])
				break
			}
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func TestModels_LiveList_StubBinary(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)

	// Install a fake "opencode" provider that prints a model list (compiled stub;
	// the old #!/bin/sh script cannot run on Windows).
	placeStubProvider(t, "opencode", stubtest.CLIOptions{Out: "openai/gpt-5.4\nopenai/gpt-5.4-mini"})

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "opencode"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	if !strings.Contains(got, "opencode:") {
		t.Errorf("output missing 'opencode:' heading\nGot:\n%s", got)
	}
	if !strings.Contains(got, "openai/gpt-5.4") {
		t.Errorf("output missing live model 'openai/gpt-5.4'\nGot:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Command-failure fallback test
// ---------------------------------------------------------------------------

func TestModels_CommandFailure_Fallback(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)

	// Install a fake "opencode" provider that exits 1 (compiled stub).
	placeStubProvider(t, "opencode", stubtest.CLIOptions{Exit: 1})

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"models", "opencode"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (fallback succeeded)", err)
	}

	gotOut := outBuf.String()
	gotErr := errBuf.String()

	// Stdout should have the curated table
	if !strings.Contains(gotOut, "opencode:") {
		t.Errorf("stdout missing 'opencode:'\nGot:\n%s", gotOut)
	}
	if !strings.Contains(gotOut, "consult `opencode --help`") {
		t.Errorf("stdout missing curated footer\nGot:\n%s", gotOut)
	}
	// Stderr should have the failure notice
	if !strings.Contains(gotErr, "list command failed") {
		t.Errorf("stderr missing failure notice\nGot:\n%s", gotErr)
	}
}

// ---------------------------------------------------------------------------
// Timeout fallback test
// ---------------------------------------------------------------------------

func TestModels_Timeout_Fallback(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)

	// Install a fake "opencode" provider that sleeps 10s (compiled stub).
	placeStubProvider(t, "opencode", stubtest.CLIOptions{SleepMS: 10000})

	// Set a very short timeout
	t.Setenv("STAGECOACH_TIMEOUT", "1s")

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"models", "opencode"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (fallback succeeded)", err)
	}

	gotOut := outBuf.String()
	// Stdout should have the curated table (timeout triggered fallback)
	if !strings.Contains(gotOut, "opencode:") {
		t.Errorf("stdout missing 'opencode:' after timeout\nGot:\n%s", gotOut)
	}
	if !strings.Contains(gotOut, "consult `opencode --help`") {
		t.Errorf("stdout missing curated footer after timeout\nGot:\n%s", gotOut)
	}
}

// ---------------------------------------------------------------------------
// Error matrix
// ---------------------------------------------------------------------------

func TestModels_UnknownProvider(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "ghost"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (unknown provider)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error message %q should contain 'ghost'", err.Error())
	}
}

func TestModels_UndetectedNamedProvider(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	repo := setupRepo(t)
	// Register a KNOWN provider whose command is not on $PATH so `models <name>` errors with
	// "not detected" (not "unknown provider"). Override a built-in's command+detect to a nonexistent
	// absolute path — robust regardless of what is installed on this machine.
	writeConfigFile(t, repo, ".stagecoach.toml", `config_version = 3
[provider.codex]
command = "/nonexistent/codex"
detect = "/nonexistent/codex"
`)
	// Prepend empty tmpDir so we still have git from the real PATH.
	tmpDir := t.TempDir()
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "codex"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (codex not detected)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "not detected") {
		t.Errorf("error message %q should contain 'not detected'", err.Error())
	}
}

func TestModels_NoDefault_NothingDetected(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)
	flagModelsAll = false

	_, repo, globalDir := loadEnvSetup(t)
	chdir(t, repo)
	// Use a clean PATH with only git so no providers are detected.
	tmpDir := t.TempDir()
	t.Setenv("PATH", putGitOnPath(t, tmpDir))
	// Override all built-in commands to nonexistent paths AND pre-write a global config
	// with empty provider so the bootstrap doesn't set a default.
	writeConfigFile(t, repo, ".stagecoach.toml", `
config_version = 3
[defaults]
provider = ""

[provider.claude]
command = "/nonexistent/claude"
[provider.pi]
command = "/nonexistent/pi"
[provider.opencode]
command = "/nonexistent/opencode"
[provider.codex]
command = "/nonexistent/codex"
[provider.cursor]
command = "/nonexistent/cursor"
[provider.agy]
command = "/nonexistent/agy"
`)
	// Also write the global config to prevent bootstrap from creating one with provider="pi"
	writeConfigFile(t, globalDir, "config.toml", `config_version = 3
[defaults]
provider = ""
`)

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (nothing detected)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "no provider detected") {
		t.Errorf("error message %q should contain 'no provider detected'", err.Error())
	}
}

func TestModels_AllEmpty(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)
	flagModelsAll = false // reset from any prior --all test

	setupRepo(t)
	// CI has no agents on $PATH; place a stub `pi` so --all has a detected provider to list.
	stubBinOnPath(t, "pi")
	// We can't guarantee no providers are on PATH (real pi, claude, etc. may exist).
	// Instead, test the --all + arg error which is independent of detection.
	// The "no providers detected" case for --all is implicitly covered by the
	// error message check in TestModels_AllEmpty_Detection.

	// Test the --all flag works (it's separate from the empty-detection case).
	// With real providers on PATH, --all should succeed and print blocks.
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "--all"})

	err := Execute(context.Background())
	// Should succeed if any providers are detected
	if err != nil {
		t.Fatalf("Execute err=%v, want nil (--all with detected providers)", err)
	}
	got := out.String()
	if got == "" {
		t.Error("--all output is empty")
	}
}

// TestModels_AllEmpty_Detection tests the --all error when no providers are detected.
// This uses a user-defined provider override with a nonexistent command to simulate
// no detected providers while keeping git on PATH for config load.
func TestModels_AllEmpty_Detection(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)
	flagModelsAll = false

	repo := setupRepo(t)
	// Override all built-in commands to nonexistent paths so nothing is detected.
	// This doesn't remove the built-ins but makes their detect commands unfindable.
	// We do this by setting PATH to a tmpDir with only git (symlinked).
	tmpDir := t.TempDir()
	// Find real git and place it on the isolated PATH (git.exe on Windows).
	t.Setenv("PATH", putGitOnPath(t, tmpDir))

	// Write a config that overrides all built-in commands to nonexistent paths
	writeConfigFile(t, repo, ".stagecoach.toml", `
[provider.claude]
command = "/nonexistent/claude"
[provider.pi]
command = "/nonexistent/pi"
[provider.opencode]
command = "/nonexistent/opencode"
[provider.codex]
command = "/nonexistent/codex"
[provider.cursor]
command = "/nonexistent/cursor"
[provider.agy]
command = "/nonexistent/agy"
`)

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "--all"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (--all, nothing detected)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "no providers detected") {
		t.Errorf("error message %q should contain 'no providers detected'", err.Error())
	}
}

func TestModels_AllWithArg(t *testing.T) {
	flagModelsAll = false // reset from any prior --all test
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "--all", "opencode"})

	err := Execute(context.Background())
	if err == nil {
		t.Fatal("Execute err=nil, want error (--all + arg)")
	}
	code := exitcode.For(err)
	if code != exitcode.Error {
		t.Errorf("exitcode.For(err) = %d, want %d (Error)", code, exitcode.Error)
	}
	if !strings.Contains(err.Error(), "--all cannot be combined") {
		t.Errorf("error message %q should contain '--all cannot be combined'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Default resolution tests
// ---------------------------------------------------------------------------

func TestModels_DefaultResolved(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)

	// Put a fake "pi" (highest priority) on PATH (compiled stub).
	placeStubProvider(t, "pi", stubtest.CLIOptions{Out: "gpt-5.4"})

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	// pi is the default (highest priority detected), should show pi's block
	if !strings.Contains(got, "pi:") {
		t.Errorf("output missing 'pi:' (default resolution)\nGot:\n%s", got)
	}
}

func TestModels_DefaultResolved_ExplicitProvider(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)
	flagModelsAll = false // reset from any prior --all test

	setupRepo(t)

	// Create a user-defined provider "myagent" with a fake binary, and set it as default.
	// CI has no agents on $PATH; place a stub `claude` so the explicit `models claude` arg is detected.
	stubBinOnPath(t, "claude")
	// Install a fake "myagent-bin" provider that exits 0 (compiled stub).
	placeStubProvider(t, "myagent-bin", stubtest.CLIOptions{Exit: 0})
	t.Setenv("STAGECOACH_PROVIDER", "claude")

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "claude"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	// claude is the explicitly requested provider, should show claude's block
	if !strings.Contains(got, "claude:") {
		t.Errorf("output missing 'claude:' (explicit arg)\nGot:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// --all over detected providers
// ---------------------------------------------------------------------------

func TestModels_AllDetected(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)

	// Put fake claude and opencode on PATH
	// Install fake "claude" and "opencode" providers that exit 0 (compiled stubs).
	for _, name := range []string{"claude", "opencode"} {
		placeStubProvider(t, name, stubtest.CLIOptions{Exit: 0})
	}

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "--all"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := out.String()
	if !strings.Contains(got, "claude:") {
		t.Errorf("output missing 'claude:'\nGot:\n%s", got)
	}
	if !strings.Contains(got, "opencode:") {
		t.Errorf("output missing 'opencode:'\nGot:\n%s", got)
	}
}

// ---------------------------------------------------------------------------
// Help shows models-scoped --all text
// ---------------------------------------------------------------------------

func TestModels_HelpShowsAllScopedText(t *testing.T) {
	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	setupRepo(t)
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"models", "--help"})

	err := Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute err=%v, want nil", err)
	}

	got := buf.String()
	if !strings.Contains(got, "every detected provider") {
		t.Errorf("help output missing models-scoped --all text\nGot:\n%s", got)
	}
	if strings.Contains(got, "git add -A") {
		t.Error(`help output must NOT contain "git add -A" (root's --all text)`)
	}
}

// strPtrUnexported is a test helper to create *string values for Manifest fields.
func strPtrUnexported(s string) *string { return &s }
