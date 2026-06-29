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

// minGitEnv returns a minimal environment with PATH and HOME so git commands
// can find the binary and the user's home directory without leaking config.
func minGitEnv() []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}
}

// makeEmptyCommit creates an empty commit in the repo at dir with the given message.
// It sets author/committer identity via environment variables (gotcha G9).
func makeEmptyCommit(t *testing.T, dir, msg string) {
	t.Helper()
	env := append(minGitEnv(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	cmd := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", msg)
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("makeEmptyCommit failed: %v\n%s", err, out)
	}
}

// TestRevParseHEAD_UnbornRepo verifies that RevParseHEAD returns isUnborn=true
// on a zero-commit repo, detected via git's exit code 128 (NOT stdout emptiness).
func TestRevParseHEAD_UnbornRepo(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo) // zero commits — unborn repo (reuses S1's helper, gotcha G8)

	g := New(repo)
	sha, isUnborn, err := g.RevParseHEAD(context.Background())

	if err != nil {
		t.Fatalf("RevParseHEAD err = %v, want nil", err)
	}
	if !isUnborn {
		t.Fatalf("RevParseHEAD isUnborn = false, want true (unborn repo)")
	}
	if sha != "" {
		t.Fatalf("RevParseHEAD sha = %q, want empty string", sha)
	}
}

// TestRevParseHEAD_BornRepo verifies that RevParseHEAD returns a trimmed SHA on
// a repo with at least one commit.
func TestRevParseHEAD_BornRepo(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "initial")

	g := New(repo)
	sha, isUnborn, err := g.RevParseHEAD(context.Background())

	if err != nil {
		t.Fatalf("RevParseHEAD err = %v, want nil", err)
	}
	if isUnborn {
		t.Fatalf("RevParseHEAD isUnborn = true, want false (repo has commits)")
	}
	if !regexp.MustCompile(`^[0-9a-f]{40,64}$`).MatchString(sha) {
		t.Fatalf("RevParseHEAD sha = %q, want 40 or 64 hex chars", sha)
	}
}

// TestRevParseHEAD_GitBinaryMissing verifies that a missing git binary surfaces
// as a non-nil error (NOT isUnborn=true).
func TestRevParseHEAD_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := New(t.TempDir()) // dir need not be a repo; LookPath fails first
	sha, isUnborn, err := g.RevParseHEAD(context.Background())

	if err == nil {
		t.Fatal("RevParseHEAD err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("RevParseHEAD err = %v, want it to contain 'git binary not found'", err)
	}
	if isUnborn {
		t.Fatalf("RevParseHEAD isUnborn = true, want false (LookPath miss is NOT unborn)")
	}
	if sha != "" {
		t.Fatalf("RevParseHEAD sha = %q, want empty string", sha)
	}
}

// TestRevParseHEAD_ContextCancelled verifies that a pre-cancelled context
// surfaces as context.Canceled (not exit 128 / unborn).
func TestRevParseHEAD_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism (gotcha G5)

	g := New(t.TempDir())
	sha, isUnborn, err := g.RevParseHEAD(ctx)

	if err == nil {
		t.Fatal("RevParseHEAD err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RevParseHEAD err = %v, want context.Canceled", err)
	}
	if isUnborn {
		t.Fatalf("RevParseHEAD isUnborn = true, want false (context cancel is NOT unborn)")
	}
	if sha != "" {
		t.Fatalf("RevParseHEAD sha = %q, want empty string", sha)
	}
}
