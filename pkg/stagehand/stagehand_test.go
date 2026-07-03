package stagehand

// Integration tests for the v1-stable public API (P1.M7.T1.S1). It is
// `package stagehand` (external test) so it exercises GenerateCommit exactly
// as an external integrator would — through the public surface only — driving
// a REAL temp git repo + the compiled stub agent binary end-to-end. It mirrors
// the internal/generate test harness (stubprovider_test.go's BuildStubBinary +
// gittestutil_test.go's newTempRepo) inline because a _test.go helper in
// internal/generate is NOT importable from another package.
//
// Two behaviors are proven:
//   - TestGenerateCommit_DryRun: opts.DryRun=true runs the full pipeline but
//     returns CommitSHA="" with the message, leaves HEAD UNCHANGED (no commit),
//     and resolves Provider+Model (PRD FR49).
//   - TestGenerateCommit_Default: the default path creates a real commit;
//     Result.CommitSHA == the new git rev-parse HEAD and the commit appears in
//     git log with the generated subject; Provider+Model are resolved.
//
// The stub agent (internal/generate/testdata/stubagent) reads its behavior
// script from STAGEHAND_STUB_SCRIPT + STAGEHAND_STUB_STATE env vars. The
// provider.Executor does `cmd.Env = append(os.Environ(), r.Env...)`, so a
// t.Setenv'd var REACHES the child without a TOML env subtable — but only when
// it is ALSO on the manifest's Env map OR os.Environ already carries it.
// Because os.Chdir(repo) changes the process cwd for the test, and because
// config.Load(".") discovers <repoDir>/.stagehand.toml, the test writes that
// file into the temp repo to register the [provider.stub] table pointing at
// the compiled stub binary.
//
// Dependencies: stdlib (os/os/exec/path/filepath/strings/testing) + the public
// stagehand package only. NO testify, NO real LLM.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// compileStubAgent builds internal/generate/testdata/stubagent into a temp
// binary and returns its path. It mirrors internal/generate.BuildStubBinary
// (which is NOT importable across packages — it lives in a _test.go file).
//
// The relative path is resolved against THIS FILE's location (via runtime
// Caller), NOT the process cwd, because the tests os.Chdir into a temp repo
// before/after building — a cwd-relative path would point at the repo, not
// the module root. The source dir of this _test.go is pkg/stagehand/, so
// "../internal/generate/testdata/stubagent" reaches the module-rooted package.
func compileStubAgent(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller: cannot locate test source file")
	}
	pkgDir := filepath.Dir(thisFile) // pkg/stagehand
	srcPkg := filepath.Join(pkgDir, "..", "..", "internal", "generate", "testdata", "stubagent")
	bin := filepath.Join(t.TempDir(), "stubagent")
	cmd := exec.Command("go", "build", "-o", bin, srcPkg)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build stubagent: %v\n%s", err, out)
	}
	return bin
}

// setupRepo bootstraps an isolated git repo with ONE initial commit (so HEAD
// exists and DryRun can assert it is unchanged), writes a .stagehand.toml
// registering the [provider.stub] provider pointing at the compiled stub
// binary, stages a NEW change (so there is something to generate a message
// for), and returns the repo dir + the initial HEAD SHA. It mirrors
// internal/git/gittestutil_test.go newTempRepo inline (that helper is in a
// _test.go file, not importable).
//
// The caller is responsible for os.Chdir(repo) + defer restore.
func setupRepo(t *testing.T) (repoDir, initialHEAD string) {
	t.Helper()
	repoDir = t.TempDir()
	mustGit(t, repoDir, "init", "-q")
	mustGit(t, repoDir, "config", "user.email", "stagehand@example.com")
	mustGit(t, repoDir, "config", "user.name", "Stagehand Test")
	mustGit(t, repoDir, "config", "commit.gpgsign", "false")

	// One initial commit so HEAD exists (the DryRun case asserts it is
	// unchanged; without a parent the assertion would be vacuous).
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# init\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	mustGit(t, repoDir, "add", "README.md")
	mustGit(t, repoDir, "commit", "-q", "-m", "chore: initial commit")
	initialHEAD = gitRevParseHEAD(t, repoDir)

	// Register the stub provider via the repo-local .stagehand.toml (the file
	// config.Load(repoDir) discovers). Use %q so an absolute binary path with
	// spaces would still quote correctly (no spaces expected, but defensive).
	bin := compileStubAgent(t)
	// Top-level provider/model make this hermetic: repo-local settings override
	// the host's GLOBAL config (FR34: repo file > global file), so a dev box
	// with ~/.config/stagehand pointing at a real provider/model never leaks in.
	toml := strings.Join([]string{
		"[defaults]",
		"provider = \"stub\"",
		"model = \"stub-model\"",
		"",
		"[provider.stub]",
		"name = \"stub\"",
		"command = " + quoteTOML(bin),
		"detect = " + quoteTOML(bin),
		"prompt_delivery = \"stdin\"",
		"print_flag = \"-p\"",
		"output = \"raw\"",
		"strip_code_fence = true",
		"default_model = \"stub-model\"",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(repoDir, ".stagehand.toml"), []byte(toml), 0o644); err != nil {
		t.Fatalf("write .stagehand.toml: %v", err)
	}

	// Stage a NEW change (a second file) so the diff is non-empty and the
	// pipeline reaches generation (an empty diff → ErrNothingToCommit).
	if err := os.WriteFile(filepath.Join(repoDir, "feature.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write feature.go: %v", err)
	}
	mustGit(t, repoDir, "add", "feature.go")
	return repoDir, initialHEAD
}

// quoteTOML wraps s in double quotes, escaping any embedded backslash/quote so
// the .stagehand.toml parses (an absolute Windows path would need this; a unix
// path is unchanged). Used only for the binary path in setupRepo.
func quoteTOML(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "\"", "\\\"")
	return "\"" + r.Replace(s) + "\""
}

// mustGit runs `git -C repoDir <args...>` and fatals on a non-zero exit. It
// drives the REAL git binary (PRD §20.1 layer 2) for test setup only — the
// production code under test goes through internal/git.
func mustGit(t *testing.T, repoDir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", repoDir}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// gitRevParseHEAD returns the full 40-hex HEAD SHA of repoDir via the REAL git
// binary. Used to snapshot HEAD before/after GenerateCommit (the DryRun case
// asserts it is unchanged; the default case asserts Result.CommitSHA matches).
func gitRevParseHEAD(t *testing.T, repoDir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repoDir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// gitLogSubject returns `git log --format=%s -n1 HEAD` of repoDir (the subject
// of the newest commit) so the default-path test can assert the generated
// subject landed on HEAD.
func gitLogSubject(t *testing.T, repoDir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repoDir, "log", "--format=%s", "-n1", "HEAD").Output()
	if err != nil {
		t.Fatalf("git log --format=%%s -n1 HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// withCwd runs fn with the process cwd set to dir, restoring the original cwd
// on return (so one test's os.Chdir never leaks into another). These tests do
// NOT run in parallel (they mutate process-global cwd + env).
func withCwd(t *testing.T, dir string, fn func()) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore cwd %s: %v", cwd, err)
		}
	}()
	fn()
}

// TestGenerateCommit_DryRun proves PRD FR49: with opts.DryRun=true the full
// pipeline runs (diff→snapshot→generate→parse→dedupe) but NO commit is made.
// Asserts: err==nil, Result.CommitSHA=="" (no commit/ref mutation), the
// generated message is in Subject+Message, Provider=="stub" and
// Model=="stub-model" (resolved), and HEAD is UNCHANGED (rev-parse before ==
// after). It NEVER stages (verified by the unchanged index: feature.go stays
// staged, nothing else moves).
func TestGenerateCommit_DryRun(t *testing.T) {
	repoDir, initialHEAD := setupRepo(t)

	// Feed the stub its single-call behavior script via env. The executor
	// appends the manifest Env + os.Environ to the child, so a t.Setenv'd
	// STAGEHAND_STUB_* var reaches the stub process.
	state := filepath.Join(t.TempDir(), "counter")
	t.Setenv("STAGEHAND_STUB_SCRIPT", `[{"emit":"feat: stubbed change"}]`)
	t.Setenv("STAGEHAND_STUB_STATE", state)

	withCwd(t, repoDir, func() {
		res, err := GenerateCommit(context.Background(), Options{Provider: "stub", DryRun: true})
		if err != nil {
			t.Fatalf("GenerateCommit error = %v; want nil (dry-run succeeds)", err)
		}
		// ★ CommitSHA empty — NO commit/ref mutation.
		if res.CommitSHA != "" {
			t.Errorf("Result.CommitSHA = %q; want \"\" (dry-run must NOT commit)", res.CommitSHA)
		}
		// The generated message is returned in Subject + Message.
		if res.Subject != "feat: stubbed change" {
			t.Errorf("Result.Subject = %q; want %q", res.Subject, "feat: stubbed change")
		}
		if res.Message != "feat: stubbed change" {
			t.Errorf("Result.Message = %q; want %q", res.Message, "feat: stubbed change")
		}
		// Provider + Model resolved to the stub manifest values.
		if res.Provider != "stub" {
			t.Errorf("Result.Provider = %q; want %q", res.Provider, "stub")
		}
		if res.Model != "stub-model" {
			t.Errorf("Result.Model = %q; want %q", res.Model, "stub-model")
		}
		// ★ HEAD must be UNCHANGED — no commit was created.
		if got := gitRevParseHEAD(t, repoDir); got != initialHEAD {
			t.Errorf("HEAD moved: was %s, now %s (dry-run must leave refs untouched)", initialHEAD, got)
		}
	})
}

// TestGenerateCommit_Default proves the default (commit) path: with
// opts.DryRun=false a real commit is created. Asserts: err==nil,
// Result.CommitSHA == the NEW git rev-parse HEAD (a 40-hex SHA, and different
// from the pre-call HEAD), the commit's subject in `git log` matches the
// generated subject, and Provider+Model are resolved.
func TestGenerateCommit_Default(t *testing.T) {
	repoDir, initialHEAD := setupRepo(t)

	state := filepath.Join(t.TempDir(), "counter")
	t.Setenv("STAGEHAND_STUB_SCRIPT", `[{"emit":"feat: stubbed change"}]`)
	t.Setenv("STAGEHAND_STUB_STATE", state)

	withCwd(t, repoDir, func() {
		res, err := GenerateCommit(context.Background(), Options{Provider: "stub"})
		if err != nil {
			t.Fatalf("GenerateCommit error = %v; want nil", err)
		}
		// ★ A real commit was created: CommitSHA is the new HEAD.
		newHEAD := gitRevParseHEAD(t, repoDir)
		if res.CommitSHA != newHEAD {
			t.Errorf("Result.CommitSHA = %q; want new HEAD %q", res.CommitSHA, newHEAD)
		}
		if newHEAD == initialHEAD {
			t.Errorf("HEAD did not advance: still %s (default path must create a commit)", initialHEAD)
		}
		if len(res.CommitSHA) != 40 {
			t.Errorf("Result.CommitSHA length = %d; want 40 (full SHA)", len(res.CommitSHA))
		}
		// The commit appears in git log with the generated subject.
		if got := gitLogSubject(t, repoDir); got != "feat: stubbed change" {
			t.Errorf("git log subject = %q; want %q", got, "feat: stubbed change")
		}
		// Provider + Model resolved.
		if res.Provider != "stub" {
			t.Errorf("Result.Provider = %q; want %q", res.Provider, "stub")
		}
		if res.Model != "stub-model" {
			t.Errorf("Result.Model = %q; want %q", res.Model, "stub-model")
		}
		if res.Subject != "feat: stubbed change" || res.Message != "feat: stubbed change" {
			t.Errorf("Result = {%q,%q}; want the generated message", res.Subject, res.Message)
		}
	})
}

// TestGenerateCommit_AutoResolveProvider proves provider/model resolution from
// the resolved config: with opts.Provider=="" AND opts.Model=="", GenerateCommit
// falls back to cfg.Provider/cfg.Model (here set by the repo-local .stagehand.toml
// [defaults] table, which overrides the host's global config per FR34). On a host
// with a CLEAN global config (no [defaults] provider), the same Options{} path
// would instead auto-resolve to the first DETECTED provider in reg.List() order;
// because the stub is registered + its binary is on PATH, that would also be
// "stub". Either way the resolved Provider lands on Result.Provider.
func TestGenerateCommit_AutoResolveProvider(t *testing.T) {
	repoDir, _ := setupRepo(t)

	state := filepath.Join(t.TempDir(), "counter")
	t.Setenv("STAGEHAND_STUB_SCRIPT", `[{"emit":"feat: auto-resolved"}]`)
	t.Setenv("STAGEHAND_STUB_STATE", state)

	withCwd(t, repoDir, func() {
		// Empty Options{}: no provider specified anywhere → auto-resolve.
		res, err := GenerateCommit(context.Background(), Options{})
		if err != nil {
			t.Fatalf("GenerateCommit error = %v; want nil (auto-resolve should pick the stub)", err)
		}
		if res.Provider != "stub" {
			t.Errorf("Result.Provider = %q; want \"stub\" (first detected provider)", res.Provider)
		}
		if res.Model != "stub-model" {
			t.Errorf("Result.Model = %q; want \"stub-model\"", res.Model)
		}
		if res.CommitSHA == "" {
			t.Errorf("Result.CommitSHA empty; want a real commit (auto-resolve + default path)")
		}
	})
}
