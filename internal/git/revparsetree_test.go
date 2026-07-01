package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// TestRevParseTree_BornRepoHEAD verifies that RevParseTree returns the tree SHA of HEAD
// on a born repo. The returned SHA MUST equal an independent git write-tree oracle
// (proves ^{tree} peeling == git's own tree resolution).
func TestRevParseTree_BornRepoHEAD(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "hello\n")
	stageFile(t, repo, "a.txt")
	makeEmptyCommit(t, repo, "init") // establishes HEAD

	g := New(repo)
	treeSHA, err := g.RevParseTree(context.Background(), "HEAD")
	if err != nil {
		t.Fatalf("RevParseTree err = %v, want nil", err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40,64}$`).MatchString(treeSHA) {
		t.Fatalf("RevParseTree sha = %q, want 40 or 64 hex chars", treeSHA)
	}
	// Independent oracle: git write-tree over the same staged+committed index yields the SAME tree.
	want := writeTreeOf(t, repo)
	if treeSHA != want {
		t.Fatalf("RevParseTree treeSHA = %q, want %q (independent write-tree oracle)", treeSHA, want)
	}
}

// TestRevParseTree_BornRepoCommitSHA verifies that RevParseTree returns the same tree SHA
// when given the commit SHA directly (proves commit-ish peeling works, not just the HEAD literal).
func TestRevParseTree_BornRepoCommitSHA(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "hello\n")
	stageFile(t, repo, "a.txt")
	makeEmptyCommit(t, repo, "init")

	commitSHA := headSHA(t, repo) // independent git rev-parse HEAD → commit SHA

	g := New(repo)
	treeSHA, err := g.RevParseTree(context.Background(), commitSHA)
	if err != nil {
		t.Fatalf("RevParseTree err = %v, want nil", err)
	}
	want := writeTreeOf(t, repo)
	if treeSHA != want {
		t.Fatalf("RevParseTree treeSHA = %q, want %q (independent write-tree oracle)", treeSHA, want)
	}
}

// TestRevParseTree_UnbornRepoHEAD verifies that RevParseTree returns ("", nil) on an unborn repo.
// git exits 128 AND prints the literal "HEAD^{tree}" to stdout — the impl MUST branch on code==128,
// NOT on stdout emptiness. Returning the literal string would be a bug.
func TestRevParseTree_UnbornRepoHEAD(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo) // zero commits — unborn repo

	g := New(repo)
	treeSHA, err := g.RevParseTree(context.Background(), "HEAD")
	if err != nil {
		t.Fatalf("RevParseTree err = %v, want nil (128 is NOT an error)", err)
	}
	if treeSHA != "" {
		t.Fatalf("RevParseTree treeSHA = %q, want empty string (exit-128, not-stdout rule)", treeSHA)
	}
}

// TestRevParseTree_GitBinaryMissing verifies that a missing git binary surfaces
// as a non-nil error containing "git binary not found".
func TestRevParseTree_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := New(t.TempDir())
	treeSHA, err := g.RevParseTree(context.Background(), "HEAD")
	if err == nil {
		t.Fatal("RevParseTree err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("RevParseTree err = %v, want it to contain 'git binary not found'", err)
	}
	if treeSHA != "" {
		t.Fatalf("RevParseTree treeSHA = %q, want empty string", treeSHA)
	}
}

// TestRevParseTree_ContextCancelled verifies that a pre-cancelled context
// surfaces as context.Canceled (not exit 128 / unborn).
func TestRevParseTree_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism

	g := New(t.TempDir())
	treeSHA, err := g.RevParseTree(ctx, "HEAD")
	if err == nil {
		t.Fatal("RevParseTree err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RevParseTree err = %v, want context.Canceled", err)
	}
	if treeSHA != "" {
		t.Fatalf("RevParseTree treeSHA = %q, want empty string", treeSHA)
	}
}

// execGit runs a git command in dir and returns trimmed stdout. Fatal on error.
func execGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
