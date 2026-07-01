package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdd_StagesOnlyGivenPaths(t *testing.T) {
	// The key property that distinguishes Add from AddAll: only the specified paths are staged.
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.go", "package main\n")
	writeFile(t, repo, "b.go", "package b\n")
	writeFile(t, repo, "c.go", "package c\n")
	stageFile(t, repo, "a.go")
	stageFile(t, repo, "b.go")
	stageFile(t, repo, "c.go")
	makeEmptyCommit(t, repo, "init")
	// Modify all three.
	writeFile(t, repo, "a.go", "package main\nvar a = 1\n")
	writeFile(t, repo, "b.go", "package b\nvar b = 2\n")
	writeFile(t, repo, "c.go", "package c\nvar c = 3\n")

	g := New(repo)
	if err := g.Add(context.Background(), []string{"a.go", "b.go"}); err != nil {
		t.Fatalf("Add err = %v", err)
	}
	// Verify only a.go + b.go are staged (c.go NOT staged).
	out, err := exec.Command("git", "-C", repo, "diff", "--cached", "--name-only").Output()
	if err != nil {
		t.Fatalf("verify diff: %v", err)
	}
	got := strings.Fields(string(out))
	want := map[string]bool{"a.go": true, "b.go": true}
	for _, p := range got {
		delete(want, p)
	}
	if len(want) != 0 {
		t.Fatalf("Add did not stage expected files; missing %v, got %v", want, got)
	}
}

func TestAdd_StagesModifiedAndUntracked(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")
	makeEmptyCommit(t, repo, "init")
	writeFile(t, repo, "a.go", "package main\nvar x = 1\n") // modified
	writeFile(t, repo, "b.go", "package main\n")            // untracked

	g := New(repo)
	if err := g.Add(context.Background(), []string{"a.go", "b.go"}); err != nil {
		t.Fatalf("Add err = %v", err)
	}
	out, err := exec.Command("git", "-C", repo, "diff", "--cached", "--name-only").Output()
	if err != nil {
		t.Fatalf("verify diff: %v", err)
	}
	got := strings.Fields(string(out))
	want := map[string]bool{"a.go": true, "b.go": true}
	for _, p := range got {
		delete(want, p)
	}
	if len(want) != 0 {
		t.Fatalf("Add did not stage expected files; missing %v, got %v", want, got)
	}
}

func TestAdd_StagesDeletion(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")
	makeEmptyCommit(t, repo, "init")
	if err := os.Remove(filepath.Join(repo, "a.go")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	g := New(repo)
	if err := g.Add(context.Background(), []string{"a.go"}); err != nil {
		t.Fatalf("Add err = %v", err)
	}
	count, err := g.StagedFileCount(context.Background()) // deletion counts as 1 staged
	if err != nil {
		t.Fatalf("StagedFileCount err = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1 (deletion should be staged)", count)
	}
}

func TestAdd_EmptyPathsNoOp(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	g := New(repo)
	if err := g.Add(context.Background(), nil); err != nil {
		t.Fatalf("Add(nil) err = %v, want nil", err)
	}
	if err := g.Add(context.Background(), []string{}); err != nil {
		t.Fatalf("Add([]) err = %v, want nil", err)
	}
}

func TestAdd_CleanTreeNoOp(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "tracked.go", "package main\n")
	stageFile(t, repo, "tracked.go")
	makeEmptyCommit(t, repo, "init")
	// tracked.go is committed and unmodified — staging it is a no-op (no diff).
	g := New(repo)
	if err := g.Add(context.Background(), []string{"tracked.go"}); err != nil {
		t.Fatalf("Add err = %v", err)
	}
	count, err := g.StagedFileCount(context.Background())
	if err != nil {
		t.Fatalf("StagedFileCount err = %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 (clean tree)", count)
	}
}

func TestAdd_UnbornRepoStages(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "f.go", "package main\n")
	g := New(repo)
	if err := g.Add(context.Background(), []string{"f.go"}); err != nil {
		t.Fatalf("Add err = %v", err)
	}
	count, err := g.StagedFileCount(context.Background())
	if err != nil {
		t.Fatalf("StagedFileCount err = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1 (unborn repo staged file)", count)
	}
}

func TestAdd_NotARepo(t *testing.T) {
	g := New(t.TempDir()) // plain dir, NOT a git repo
	err := g.Add(context.Background(), []string{"a.go"})
	if err == nil {
		t.Fatal("Add err = nil, want non-nil (non-repo → exit 128)")
	}
	if !strings.Contains(err.Error(), "git add: failed") {
		t.Fatalf("err = %v, want it to contain 'git add: failed'", err)
	}
}

func TestAdd_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail
	g := New(t.TempDir())
	err := g.Add(context.Background(), []string{"a.go"})
	if err == nil {
		t.Fatal("Add err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("err = %v, want it to contain 'git binary not found'", err)
	}
}

func TestAdd_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism
	g := New(t.TempDir())
	err := g.Add(ctx, []string{"a.go"})
	if err == nil {
		t.Fatal("Add err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want errors.Is(err, context.Canceled)", err)
	}
}
