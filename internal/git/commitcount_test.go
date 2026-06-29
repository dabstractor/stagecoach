package git

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestCommitCount_UnbornRepo(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo) // zero commits — unborn

	g := New(repo)
	count, err := g.CommitCount(context.Background())

	if err != nil {
		t.Fatalf("CommitCount err = %v, want nil", err)
	}
	if count != 0 {
		t.Fatalf("CommitCount count = %d, want 0 (unborn)", count)
	}
}

func TestCommitCount_ThreeCommits(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	for i := 0; i < 3; i++ {
		makeEmptyCommit(t, repo, strconv.Itoa(i))
	}

	g := New(repo)
	count, err := g.CommitCount(context.Background())

	if err != nil {
		t.Fatalf("CommitCount err = %v, want nil", err)
	}
	if count != 3 {
		t.Fatalf("CommitCount count = %d, want 3", count)
	}
}

func TestCommitCount_TenCommits(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	for i := 0; i < 10; i++ {
		makeEmptyCommit(t, repo, strconv.Itoa(i))
	}

	g := New(repo)
	count, err := g.CommitCount(context.Background())

	if err != nil {
		t.Fatalf("CommitCount err = %v, want nil", err)
	}
	if count != 10 {
		t.Fatalf("CommitCount count = %d, want 10", count)
	}
}

func TestCommitCount_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := New(t.TempDir())
	count, err := g.CommitCount(context.Background())

	if err == nil {
		t.Fatal("CommitCount err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("CommitCount err = %v, want it to contain 'git binary not found'", err)
	}
	if count != 0 {
		t.Fatalf("CommitCount count = %d, want 0", count)
	}
}

func TestCommitCount_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism

	g := New(t.TempDir())
	count, err := g.CommitCount(ctx)

	if err == nil {
		t.Fatal("CommitCount err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CommitCount err = %v, want context.Canceled", err)
	}
	if count != 0 {
		t.Fatalf("CommitCount count = %d, want 0", count)
	}
}
