package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestReadTree_LoadsTreeIntoIndex verifies that ReadTree loads a tree's contents
// into the index (verified via an independent git ls-files oracle).
func TestReadTree_LoadsTreeIntoIndex(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "hello\n")
	stageFile(t, repo, "a.txt")
	makeEmptyCommit(t, repo, "init")
	tree := writeTreeOf(t, repo) // the tree holding a.txt

	// Remove a.txt from the index so the load is observable.
	rmCmd := exec.Command("git", "-C", repo, "rm", "--cached", "-q", "a.txt")
	rmCmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := rmCmd.CombinedOutput(); err != nil {
		t.Fatalf("git rm --cached failed: %v\n%s", err, out)
	}

	// Independent oracle BEFORE: git ls-files shows nothing.
	beforeCmd := exec.Command("git", "-C", repo, "ls-files")
	beforeOut, _ := beforeCmd.Output()
	if strings.TrimSpace(string(beforeOut)) != "" {
		t.Fatalf("expected empty index before ReadTree, got: %s", string(beforeOut))
	}

	g := New(repo)
	if err := g.ReadTree(context.Background(), tree); err != nil {
		t.Fatalf("ReadTree err = %v, want nil", err)
	}

	// Independent oracle AFTER: git ls-files shows a.txt again.
	afterCmd := exec.Command("git", "-C", repo, "ls-files")
	afterOut, err := afterCmd.Output()
	if err != nil {
		t.Fatalf("git ls-files failed: %v", err)
	}
	got := strings.TrimSpace(string(afterOut))
	if got != "a.txt" {
		t.Fatalf("ReadTree did not load a.txt into index; git ls-files = %q, want \"a.txt\"", got)
	}
}

// TestReadTree_ReplacesIndex verifies that ReadTree REPLACES the index (not merges).
// Loading an older tree (holding only a.txt) into an index holding a.txt+b.txt
// should leave ONLY a.txt — the arbiter's mid-chain rebuild depends on this.
func TestReadTree_ReplacesIndex(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// Commit 1: a.txt only.
	writeFile(t, repo, "a.txt", "a\n")
	stageFile(t, repo, "a.txt")
	makeEmptyCommit(t, repo, "one")

	// Commit 2: a.txt + b.txt.
	writeFile(t, repo, "b.txt", "b\n")
	stageFile(t, repo, "b.txt")
	makeEmptyCommit(t, repo, "two")

	// Index now holds a.txt + b.txt (matches HEAD).
	// Get the tree of HEAD~1 (holds ONLY a.txt) via independent git rev-parse.
	olderTree := execGit(t, repo, "rev-parse", "HEAD~1^{tree}")

	g := New(repo)
	if err := g.ReadTree(context.Background(), olderTree); err != nil {
		t.Fatalf("ReadTree err = %v, want nil", err)
	}

	// Independent oracle: git ls-files should show ONLY a.txt (b.txt DROPPED — REPLACES not merge).
	out, err := exec.Command("git", "-C", repo, "ls-files").Output()
	if err != nil {
		t.Fatalf("git ls-files failed: %v", err)
	}
	got := strings.Fields(string(out))
	if len(got) != 1 || got[0] != "a.txt" {
		t.Fatalf("ReadTree did not REPLACE index; git ls-files = %v, want [a.txt]", got)
	}
}

// TestReadTree_BadTree verifies that ReadTree returns a non-nil error for an invalid tree SHA.
func TestReadTree_BadTree(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	g := New(repo)
	err := g.ReadTree(context.Background(), "0000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("ReadTree err = nil, want non-nil (bad tree SHA)")
	}
	if !strings.Contains(err.Error(), "git read-tree: failed") {
		t.Fatalf("ReadTree err = %v, want it to contain 'git read-tree: failed'", err)
	}
}

// TestReadTree_NotARepo verifies that ReadTree returns a non-nil error on a non-repo directory.
func TestReadTree_NotARepo(t *testing.T) {
	g := New(t.TempDir()) // plain dir, NOT a git repo
	err := g.ReadTree(context.Background(), "2e81171448eb9f2ee3821e3d447aa6b2fe3ddba1")
	if err == nil {
		t.Fatal("ReadTree err = nil, want non-nil (non-repo)")
	}
	if !strings.Contains(err.Error(), "git read-tree: failed") {
		t.Fatalf("ReadTree err = %v, want it to contain 'git read-tree: failed'", err)
	}
}

// TestReadTree_GitBinaryMissing verifies that a missing git binary surfaces
// as a non-nil error containing "git binary not found".
func TestReadTree_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := New(t.TempDir())
	err := g.ReadTree(context.Background(), "tree")
	if err == nil {
		t.Fatal("ReadTree err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("ReadTree err = %v, want it to contain 'git binary not found'", err)
	}
}

// TestReadTree_ContextCancelled verifies that a pre-cancelled context
// surfaces as context.Canceled.
func TestReadTree_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism

	g := New(t.TempDir())
	err := g.ReadTree(ctx, "tree")
	if err == nil {
		t.Fatal("ReadTree err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ReadTree err = %v, want errors.Is(err, context.Canceled)", err)
	}
}
