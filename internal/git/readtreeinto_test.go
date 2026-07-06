package git

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// scopedLsFiles is an INDEPENDENT oracle that lists the paths in the throwaway index at indexFile
// (via GIT_INDEX_FILE) WITHOUT going through the runner under test. It is the readtree_test.go
// `git ls-files` oracle, scoped to a non-default index.
func scopedLsFiles(t *testing.T, repo, indexFile string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "ls-files")
	cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+indexFile)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("scoped ls-files oracle failed: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// TestReadTreeInto_LoadsTreeIntoScopedIndex verifies that ReadTreeInto primes a throwaway index
// from <tree> (verified via an INDEPENDENT GIT_INDEX_FILE-scoped git ls-files oracle).
func TestReadTreeInto_LoadsTreeIntoScopedIndex(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "hello\n")
	stageFile(t, repo, "a.txt")
	makeEmptyCommit(t, repo, "init")
	tree := writeTreeOf(t, repo) // the tree holding a.txt

	// A SEPARATE temp dir/file for the throwaway index (NOT under repo/.git).
	tmpIndex := filepath.Join(t.TempDir(), "scoped.index")

	g := New(repo)
	if err := g.ReadTreeInto(context.Background(), tree, tmpIndex); err != nil {
		t.Fatalf("ReadTreeInto err = %v, want nil", err)
	}

	// Independent oracle: the throwaway index holds the tree's path.
	if got := scopedLsFiles(t, repo, tmpIndex); got != "a.txt" {
		t.Fatalf("throwaway index = %q, want \"a.txt\"", got)
	}
}

// TestReadTreeInto_LiveIndexUntouched is THE KEYSTONE: the live .git/index is byte-identical
// before/after the scoped call. This is what distinguishes ReadTreeInto from ReadTree.
func TestReadTreeInto_LiveIndexUntouched(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "hello\n")
	stageFile(t, repo, "a.txt") // live index now holds a.txt
	makeEmptyCommit(t, repo, "init")
	tree := writeTreeOf(t, repo)

	tmpIndex := filepath.Join(t.TempDir(), "scoped.index")

	// Snapshot the LIVE index BEFORE (independent oracle — NO GIT_INDEX_FILE).
	before := execGit(t, repo, "ls-files")
	beforeBytes, err := os.ReadFile(filepath.Join(repo, ".git", "index"))
	if err != nil {
		t.Fatalf("read live .git/index before: %v", err)
	}

	g := New(repo)
	if err := g.ReadTreeInto(context.Background(), tree, tmpIndex); err != nil {
		t.Fatalf("ReadTreeInto err = %v, want nil", err)
	}

	// KEYSTONE: the LIVE index is byte-identical (a.txt still staged; scoped read-tree wrote tmpIndex).
	if after := execGit(t, repo, "ls-files"); after != before {
		t.Errorf("live .git/index ls-files changed: before=%q after=%q (scoped variant must NOT touch .git/index)", before, after)
	}
	afterBytes, err := os.ReadFile(filepath.Join(repo, ".git", "index"))
	if err != nil {
		t.Fatalf("read live .git/index after: %v", err)
	}
	if !bytes.Equal(beforeBytes, afterBytes) {
		t.Errorf("live .git/index bytes changed: before=%d bytes after=%d bytes (must be byte-identical)", len(beforeBytes), len(afterBytes))
	}

	// And the THROWAWAY index holds the tree (independent oracle WITH GIT_INDEX_FILE).
	if got := scopedLsFiles(t, repo, tmpIndex); got != "a.txt" {
		t.Errorf("throwaway index = %q, want \"a.txt\"", got)
	}
}

// TestReadTreeInto_BadTree verifies that ReadTreeInto returns a non-nil error for an invalid tree.
func TestReadTreeInto_BadTree(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	tmpIndex := filepath.Join(t.TempDir(), "scoped.index")

	g := New(repo)
	err := g.ReadTreeInto(context.Background(), "0000000000000000000000000000000000000000", tmpIndex)
	if err == nil {
		t.Fatal("ReadTreeInto err = nil, want non-nil (bad tree SHA)")
	}
	if !strings.Contains(err.Error(), "git read-tree (scoped): failed") {
		t.Fatalf("ReadTreeInto err = %v, want it to contain 'git read-tree (scoped): failed'", err)
	}
}

// TestReadTreeInto_GitBinaryMissing verifies that a missing git binary surfaces as a non-nil
// error containing "git binary not found".
func TestReadTreeInto_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes runWithEnv()'s LookPath("git") fail

	tmpIndex := filepath.Join(t.TempDir(), "scoped.index")
	g := New(t.TempDir())
	err := g.ReadTreeInto(context.Background(), "tree", tmpIndex)
	if err == nil {
		t.Fatal("ReadTreeInto err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("ReadTreeInto err = %v, want it to contain 'git binary not found'", err)
	}
}

// TestReadTreeInto_ContextCancelled verifies that a pre-cancelled context surfaces as context.Canceled.
func TestReadTreeInto_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism

	tmpIndex := filepath.Join(t.TempDir(), "scoped.index")
	g := New(t.TempDir())
	err := g.ReadTreeInto(ctx, "tree", tmpIndex)
	if err == nil {
		t.Fatal("ReadTreeInto err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ReadTreeInto err = %v, want errors.Is(err, context.Canceled)", err)
	}
}
