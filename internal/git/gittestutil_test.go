package git

// Shared white-box test harness for package git. It is package git (NOT
// git_test) so it can call the unexported (g *Git).run seam and read the
// unexported g.dir field, mirroring the shipped internal/git/git_test.go.
// Like every _test.go file it is compiled ONLY into this package's test
// binary and never into the shipped `go build` output, so no build tag is
// needed (git is always present on the host per system_context.md §2).
//
// The harness drives the REAL host git binary (PRD §20.1 layer 2: "Unit — git
// wrapper, with a real git binary. Each internal/git/* test creates a temp
// directory, git init, stages known content, and asserts on ...") and mocks
// ONLY the repository (a per-test temp dir + a deterministic commit
// identity). Standing it up as its own unit (plan_overview §8; decisions.md
// §8) keeps the git init/add/commit boilerplate in ONE place so a git-version
// or identity quirk is fixed once, not in every test. The downstream
// plumbing (M3.T2), diff (M3.T3), log/stage (M3.T4) tests and the generate
// integration suite (M6.T3) all compose these helpers instead of reinventing
// that boilerplate.

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// mustRun is a tiny fail-fast wrapper over the shipped (g *Git).run seam: it
// surfaces any git error via tb.Fatalf (with the args, error, and captured
// stdout) so a git regression aborts the test loudly instead of silently
// corrupting every downstream assertion. It is private convenience shared by
// the three exported-to-the-test-binary helpers below.
func mustRun(tb testing.TB, g *Git, args ...string) {
	tb.Helper()
	out, err := g.run(args...)
	if err != nil {
		tb.Fatalf("git %v: %v\nstdout:%s", args, err, out)
	}
}

// newTempRepo bootstraps an isolated repository for one test. It creates a
// unique temp dir via tb.TempDir (auto-removed by the testing package at test
// end — no manual os.MkdirTemp/t.Cleanup), binds a *Git to it, runs
// `git init -q`, and sets REPO-LOCAL deterministic config (user.email /
// user.name so commits are authorable, plus commit.gpgsign=false as a
// defensive cross-machine hardening against hosts with global gpgsign). It
// drives the REAL git binary (PRD §20.1 layer 2) and returns the repo UNBORN
// (no initial commit), so T2 root-commit tests observe RevParseHEAD returning
// ok=false; tests that need history call seedCommits themselves. The harness
// is the shared setup seam for every internal/git/*_test.go and the generate
// integration suite (decisions.md §8).
func newTempRepo(tb testing.TB) *Git {
	tb.Helper()
	dir := tb.TempDir()
	g, err := New(dir)
	if err != nil {
		tb.Fatalf("git.New: %v", err)
	}
	mustRun(tb, g, "init", "-q")
	mustRun(tb, g, "config", "user.email", "stagehand@example.com")
	mustRun(tb, g, "config", "user.name", "Stagehand Test")
	mustRun(tb, g, "config", "commit.gpgsign", "false") // repo-local, zero production impact
	return g
}

// writeFileStage writes a file under the repo (creating any parent
// directories so nested paths like "pkg/sub/f.go" work) and stages it via the
// REAL `git add` (PRD §20.1 layer 2). path is RELATIVE to the repo root; it is
// staged as-is because cmd.Dir is g.dir, so the index entry matches the
// on-disk path. It is the shared setup seam (decisions.md §8) used by the diff
// tests (M3.T3) and by seedCommits.
func writeFileStage(tb testing.TB, g *Git, path, content string) {
	tb.Helper()
	full := filepath.Join(g.dir, path) // white-box read of the unexported g.dir
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		tb.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		tb.Fatalf("write %s: %v", full, err)
	}
	mustRun(tb, g, "add", path) // stage the RELATIVE path (cmd.Dir is g.dir)
}

// seedCommits creates a deterministic LINEAR history of len(msgs) commits,
// one per message, by pairing writeFileStage (a UNIQUE file<i>.txt per commit
// so the tree changes and every commit is non-empty) with a single
// `git commit -q -m <msg>` driven through the REAL binary (PRD §20.1 layer 2).
// The ENTIRE message — including embedded newlines/paragraphs — is passed as
// ONE -m arg (verified: `git log --format=%B` reproduces multi-line bodies
// verbatim); multiple -m flags would split it into separate paragraphs and
// stdin (-F -) is unusable because g.run leaves cmd.Stdin nil. This is the
// shared setup seam (decisions.md §8) the log/history tests (M3.T4) and the
// generate integration suite (M6.T3) build on.
func seedCommits(tb testing.TB, g *Git, msgs []string) {
	tb.Helper()
	for i, msg := range msgs {
		writeFileStage(tb, g, fmt.Sprintf("file%d.txt", i), fmt.Sprintf("content %d\n", i))
		mustRun(tb, g, "commit", "-q", "-m", msg)
	}
}

// TestHarness_BootstrapsRepoWithCommits is the harness's own regression guard:
// it proves newTempRepo + seedCommits of >=2 messages yields a repo whose
// `rev-list --count HEAD` is >=2, whose `rev-parse HEAD` is a 40-char SHA,
// and whose deterministic user.email/user.name are read back exactly — all
// against the REAL git binary.
func TestHarness_BootstrapsRepoWithCommits(t *testing.T) {
	g := newTempRepo(t)
	seedCommits(t, g, []string{
		"feat: first commit\n\nFirst body line.\nSecond body line.",
		"fix: second commit",
	})

	out, err := g.run("rev-list", "--count", "HEAD")
	if err != nil {
		t.Fatalf("rev-list --count HEAD: %v", err)
	}
	n, e := strconv.Atoi(strings.TrimSpace(out))
	if e != nil {
		t.Fatalf("rev-list --count HEAD = %q; want an int: %v", out, e)
	}
	if n < 2 {
		t.Errorf("rev-list --count HEAD = %d; want >= 2", n)
	}

	head, err := g.run("rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	if got := len(strings.TrimSpace(head)); got != 40 {
		t.Errorf("rev-parse HEAD length = %d; want 40 (full SHA)", got)
	}

	email, _ := g.run("config", "user.email")
	if got := strings.TrimSpace(email); got != "stagehand@example.com" {
		t.Errorf("config user.email = %q; want %q", got, "stagehand@example.com")
	}
	name, _ := g.run("config", "user.name")
	if got := strings.TrimSpace(name); got != "Stagehand Test" {
		t.Errorf("config user.name = %q; want %q", got, "Stagehand Test")
	}
}
