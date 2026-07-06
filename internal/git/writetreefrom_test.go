package git

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestWriteTreeFrom_PrimedIndex verifies that WriteTreeFrom returns the tree SHA of a primed
// throwaway index, matching the unscoped writeTreeOf oracle for the same tree.
func TestWriteTreeFrom_PrimedIndex(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "hello\n")
	stageFile(t, repo, "a.txt")
	makeEmptyCommit(t, repo, "init")
	tree := writeTreeOf(t, repo) // the unscoped oracle SHA for this tree

	tmpIndex := filepath.Join(t.TempDir(), "scoped.index")

	g := New(repo)
	// Prime the throwaway index (exercises BOTH new primitives in one test).
	if err := g.ReadTreeInto(context.Background(), tree, tmpIndex); err != nil {
		t.Fatalf("ReadTreeInto err = %v, want nil", err)
	}

	sha, err := g.WriteTreeFrom(context.Background(), tmpIndex)
	if err != nil {
		t.Fatalf("WriteTreeFrom err = %v, want nil", err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40,64}$`).MatchString(sha) {
		t.Fatalf("WriteTreeFrom sha = %q, want 40 or 64 hex chars", sha)
	}
	if sha != tree {
		t.Fatalf("WriteTreeFrom sha = %q, want %q (the primed tree)", sha, tree)
	}
}

// TestWriteTreeFrom_LiveIndexUntouched is THE KEYSTONE: the live .git/index is byte-identical
// before/after priming a throwaway index and capturing its tree. This is what distinguishes
// WriteTreeFrom from WriteTree.
func TestWriteTreeFrom_LiveIndexUntouched(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.txt", "hello\n")
	stageFile(t, repo, "a.txt") // live index now holds a.txt
	makeEmptyCommit(t, repo, "init")
	tree := writeTreeOf(t, repo)

	tmpIndex := filepath.Join(t.TempDir(), "scoped.index")

	// Snapshot the LIVE index BEFORE (independent oracle — NO GIT_INDEX_FILE).
	before := execGit(t, repo, "ls-files")

	g := New(repo)
	if err := g.ReadTreeInto(context.Background(), tree, tmpIndex); err != nil {
		t.Fatalf("ReadTreeInto err = %v, want nil", err)
	}
	if _, err := g.WriteTreeFrom(context.Background(), tmpIndex); err != nil {
		t.Fatalf("WriteTreeFrom err = %v, want nil", err)
	}

	// KEYSTONE: the LIVE index is unchanged (scoped write-tree read tmpIndex, not .git/index).
	if after := execGit(t, repo, "ls-files"); after != before {
		t.Errorf("live .git/index ls-files changed: before=%q after=%q (scoped variant must NOT touch .git/index)", before, after)
	}
}

// TestWriteTreeFrom_EmptyIndex verifies that WriteTreeFrom on a throwaway index primed from the
// canonical empty tree returns the canonical sha-1 empty-tree object ID (gotcha G7, scoped path).
func TestWriteTreeFrom_EmptyIndex(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	const emptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	tmpIndex := filepath.Join(t.TempDir(), "scoped.index")

	g := New(repo)
	// Prime the throwaway index from the empty tree.
	if err := g.ReadTreeInto(context.Background(), emptyTreeSHA, tmpIndex); err != nil {
		t.Fatalf("ReadTreeInto err = %v, want nil", err)
	}
	sha, err := g.WriteTreeFrom(context.Background(), tmpIndex)
	if err != nil {
		t.Fatalf("WriteTreeFrom err = %v, want nil", err)
	}
	if sha != emptyTreeSHA {
		t.Fatalf("WriteTreeFrom sha = %q, want %q (canonical sha-1 empty tree)", sha, emptyTreeSHA)
	}
}

// TestWriteTreeFrom_GitBinaryMissing verifies that a missing git binary surfaces as a non-nil
// error containing "git binary not found" (NOT misread as a conflict).
func TestWriteTreeFrom_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes runWithEnv()'s LookPath("git") fail

	tmpIndex := filepath.Join(t.TempDir(), "scoped.index")
	g := New(t.TempDir())
	sha, err := g.WriteTreeFrom(context.Background(), tmpIndex)
	if err == nil {
		t.Fatal("WriteTreeFrom err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("WriteTreeFrom err = %v, want it to contain 'git binary not found'", err)
	}
	if sha != "" {
		t.Fatalf("WriteTreeFrom sha = %q, want empty string", sha)
	}
}

// TestWriteTreeFrom_ContextCancelled verifies that a pre-cancelled context surfaces as context.Canceled.
func TestWriteTreeFrom_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism

	tmpIndex := filepath.Join(t.TempDir(), "scoped.index")
	g := New(t.TempDir())
	sha, err := g.WriteTreeFrom(ctx, tmpIndex)
	if err == nil {
		t.Fatal("WriteTreeFrom err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WriteTreeFrom err = %v, want errors.Is(err, context.Canceled)", err)
	}
	if sha != "" {
		t.Fatalf("WriteTreeFrom sha = %q, want empty string", sha)
	}
}
