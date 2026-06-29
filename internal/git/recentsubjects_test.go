package git

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRecentSubjects_UnbornRepo(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo) // zero commits — unborn repo

	g := New(repo)
	subjects, err := g.RecentSubjects(context.Background(), 5)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(subjects) != 0 {
		t.Fatalf("len = %d, want 0 (unborn repo)", len(subjects))
	}
}

func TestRecentSubjects_ReturnsSubjects(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "feat: third commit")
	makeEmptyCommit(t, repo, "fix: second commit")
	makeEmptyCommit(t, repo, "docs: first commit")

	g := New(repo)
	subjects, err := g.RecentSubjects(context.Background(), 5)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(subjects) != 3 {
		t.Fatalf("len = %d, want 3", len(subjects))
	}
	// newest-first
	if subjects[0] != "docs: first commit" {
		t.Fatalf("subjects[0] = %q, want %q", subjects[0], "docs: first commit")
	}
	if subjects[1] != "fix: second commit" {
		t.Fatalf("subjects[1] = %q, want %q", subjects[1], "fix: second commit")
	}
	if subjects[2] != "feat: third commit" {
		t.Fatalf("subjects[2] = %q, want %q", subjects[2], "feat: third commit")
	}
	// each subject is single-line (no embedded newline)
	for i, s := range subjects {
		if strings.Contains(s, "\n") {
			t.Fatalf("subjects[%d] = %q contains a newline (%%s should be single-line)", i, s)
		}
	}
}

func TestRecentSubjects_NExceedsCommits(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "first")
	makeEmptyCommit(t, repo, "second")

	g := New(repo)
	subjects, err := g.RecentSubjects(context.Background(), 50) // FR31 default, > commit count
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(subjects) != 2 {
		t.Fatalf("len = %d, want 2 (git returns only what exists)", len(subjects))
	}
}

func TestRecentSubjects_SubjectOnlyExcludesBody(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "feat: add x\n\nThis is the body. It must NOT appear in subjects.")
	g := New(repo)
	subjects, err := g.RecentSubjects(context.Background(), 5)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(subjects) != 1 {
		t.Fatalf("len = %d, want 1", len(subjects))
	}
	if subjects[0] != "feat: add x" {
		t.Fatalf("subject = %q, want %q (the body must be excluded — %%s, not %%B)", subjects[0], "feat: add x")
	}
}

func TestRecentSubjects_MarkdownHRInSubject(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "fix: handle --- edge case")
	g := New(repo)
	subjects, err := g.RecentSubjects(context.Background(), 5)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(subjects) != 1 {
		t.Fatalf("len = %d, want 1 (the '---' must NOT split the subject under --format=%%s)", len(subjects))
	}
	if !strings.Contains(subjects[0], "---") {
		t.Fatalf("subject lost its '---': %q", subjects[0])
	}
	if !strings.Contains(subjects[0], "edge case") {
		t.Fatalf("subject lost its tail: %q", subjects[0])
	}
}

func TestRecentSubjects_ZeroOrNegativeN(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "should not see this")

	g := New(repo)

	// n = 0
	subjects, err := g.RecentSubjects(context.Background(), 0)
	if err != nil {
		t.Fatalf("n=0: err = %v, want nil", err)
	}
	if len(subjects) != 0 {
		t.Fatalf("n=0: len = %d, want 0", len(subjects))
	}

	// n = -5
	subjects, err = g.RecentSubjects(context.Background(), -5)
	if err != nil {
		t.Fatalf("n=-5: err = %v, want nil", err)
	}
	if len(subjects) != 0 {
		t.Fatalf("n=-5: len = %d, want 0", len(subjects))
	}
}

func TestRecentSubjects_NotARepo(t *testing.T) {
	repo := t.TempDir() // no git init — not a repo

	g := New(repo)
	subjects, err := g.RecentSubjects(context.Background(), 5)
	if err != nil {
		t.Fatalf("err = %v, want nil (non-repo exits 128 ⇒ empty, inherited G4)", err)
	}
	if len(subjects) != 0 {
		t.Fatalf("len = %d, want 0 (non-repo)", len(subjects))
	}
}

func TestRecentSubjects_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := New(t.TempDir())
	subjects, err := g.RecentSubjects(context.Background(), 5)
	if err == nil {
		t.Fatal("err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("err = %v, want it to contain 'git binary not found'", err)
	}
	if subjects != nil {
		t.Fatalf("subjects = %v, want nil", subjects)
	}
}

func TestRecentSubjects_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism

	g := New(t.TempDir())
	subjects, err := g.RecentSubjects(ctx, 5)
	if err == nil {
		t.Fatal("err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if subjects != nil {
		t.Fatalf("subjects = %v, want nil", subjects)
	}
}
