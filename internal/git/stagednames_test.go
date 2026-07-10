package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStagedNames_NothingStaged(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init") // HEAD exists; nothing NEW staged
	g := New(repo)
	names, err := g.StagedNames(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	// Assert len (NOT == nil): empty output yields either nil or []string{},
	// both of which are valid per the contract.
	if len(names) != 0 {
		t.Fatalf("names = %v, want empty (nothing staged → empty output)", names)
	}
}

func TestStagedNames_ThreeFiles(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init")
	writeFile(t, repo, "a.go", "1\n")
	stageFile(t, repo, "a.go")
	writeFile(t, repo, "b.go", "2\n")
	stageFile(t, repo, "b.go")
	writeFile(t, repo, "c.go", "3\n")
	stageFile(t, repo, "c.go")
	g := New(repo)
	names, err := g.StagedNames(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []string{"a.go", "b.go", "c.go"}
	if !equalUnsorted(names, want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
}

func TestStagedNames_AfterAddAll(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init")
	writeFile(t, repo, "a.go", "modified\n")
	writeFile(t, repo, "b.go", "untracked\n")

	g := New(repo)
	if err := g.AddAll(context.Background()); err != nil {
		t.Fatalf("AddAll err = %v", err)
	}
	names, err := g.StagedNames(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	want := []string{"a.go", "b.go"}
	if !equalUnsorted(names, want) {
		t.Fatalf("names = %v, want %v (one modified + one untracked after AddAll)", names, want)
	}
}

func TestStagedNames_IncludesDeletion(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.go", "initial\n")
	stageFile(t, repo, "a.go")
	makeEmptyCommit(t, repo, "init")
	if err := os.Remove(filepath.Join(repo, "a.go")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	g := New(repo)
	if err := g.AddAll(context.Background()); err != nil {
		t.Fatalf("AddAll err = %v", err)
	}
	names, err := g.StagedNames(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(names) != 1 || names[0] != "a.go" {
		t.Fatalf("names = %v, want [a.go] (deletion should be staged)", names)
	}
}

func TestStagedNames_FilenameWithSpace(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init")
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, repo, "sub/has space.txt", "x\n") // create file with a SPACE in the name
	g := New(repo)
	if err := g.AddAll(context.Background()); err != nil {
		t.Fatalf("AddAll err = %v", err)
	}
	names, err := g.StagedNames(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(names) != 1 {
		t.Fatalf("names = %v, want exactly ONE element (a space in the name must NOT split into "+
			"2 lines under --name-only)", names)
	}
	if names[0] != "sub/has space.txt" {
		t.Fatalf("names[0] = %q, want %q", names[0], "sub/has space.txt")
	}
}

func TestStagedNames_UnbornRepoWithStaged(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "f.go", "package main\n")
	g := New(repo)
	if err := g.AddAll(context.Background()); err != nil {
		t.Fatalf("AddAll err = %v", err)
	}
	names, err := g.StagedNames(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(names) != 1 || names[0] != "f.go" {
		t.Fatalf("names = %v, want [f.go] (unborn repo staged file)", names)
	}
}

func TestStagedNames_NotARepo(t *testing.T) {
	g := New(t.TempDir()) // plain dir, NOT a git repo (no initRepo) → exit 129
	names, err := g.StagedNames(context.Background())
	if err == nil {
		t.Fatal("err = nil, want non-nil (non-repo → exit 129)")
	}
	if !strings.Contains(err.Error(), "git diff --cached --name-only: failed") {
		t.Fatalf("err = %v, want it to contain 'git diff --cached --name-only: failed'", err)
	}
	if len(names) != 0 {
		t.Fatalf("names = %v, want empty on error", names)
	}
}

func TestStagedNames_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "")
	g := New(t.TempDir())
	names, err := g.StagedNames(context.Background())
	if err == nil {
		t.Fatal("err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("err = %v, want it to contain 'git binary not found'", err)
	}
	if len(names) != 0 {
		t.Fatalf("names = %v, want empty on error", names)
	}
}

func TestStagedNames_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	g := New(t.TempDir())
	names, err := g.StagedNames(ctx)
	if err == nil {
		t.Fatal("err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want errors.Is(err, context.Canceled)", err)
	}
	if len(names) != 0 {
		t.Fatalf("names = %v, want empty on error", names)
	}
}

// equalUnsorted reports whether got and want contain the same elements regardless of order.
// git emits staged paths in a deterministic order, but comparing as sets keeps the test
// robust against git's ordering choices across versions/configs.
func equalUnsorted(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]int, len(want))
	for _, w := range want {
		seen[w]++
	}
	for _, g := range got {
		if seen[g]--; seen[g] < 0 {
			return false
		}
	}
	return true
}
