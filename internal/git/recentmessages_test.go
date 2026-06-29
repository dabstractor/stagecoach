package git

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// rmTotalLines computes the total line count across a slice of messages.
func rmTotalLines(msgs []string) int {
	total := 0
	for _, m := range msgs {
		total += strings.Count(m, "\n") + 1
	}
	return total
}

func TestRecentMessages_UnbornRepo(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo) // zero commits — unborn

	g := New(repo)
	msgs, err := g.RecentMessages(context.Background(), 5)

	if err != nil {
		t.Fatalf("RecentMessages err = %v, want nil", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("RecentMessages len = %d, want 0 (unborn)", len(msgs))
	}
}

func TestRecentMessages_SingleLine(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "first")
	makeEmptyCommit(t, repo, "second")

	g := New(repo)
	msgs, err := g.RecentMessages(context.Background(), 5)

	if err != nil {
		t.Fatalf("RecentMessages err = %v, want nil", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len(msgs) = %d, want 2", len(msgs))
	}
	// Newest first (git log order).
	if msgs[0] != "second" {
		t.Fatalf("msgs[0] = %q, want \"second\" (newest first)", msgs[0])
	}
	if msgs[1] != "first" {
		t.Fatalf("msgs[1] = %q, want \"first\"", msgs[1])
	}
	// Single-line messages: no embedded newline.
	for i, m := range msgs {
		if strings.Contains(m, "\n") {
			t.Fatalf("msgs[%d] = %q: single-line commit should have no newline", i, m)
		}
	}
}

func TestRecentMessages_MultiLineBody(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "feat: add feature\n\nBody line one.\nBody line two.")

	g := New(repo)
	msgs, err := g.RecentMessages(context.Background(), 5)

	if err != nil {
		t.Fatalf("RecentMessages err = %v, want nil", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}
	if !strings.Contains(msgs[0], "\n\n") {
		t.Fatalf("message has no body separator: %q", msgs[0])
	}
	if !strings.Contains(msgs[0], "Body line one.") {
		t.Fatalf("message body lost: %q", msgs[0])
	}
	if !strings.Contains(msgs[0], "Body line two.") {
		t.Fatalf("message body lost: %q", msgs[0])
	}
}

func TestRecentMessages_MarkdownHRCollision(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "docs: update\n\n---\n\nThis text is after a horizontal rule")

	g := New(repo)
	msgs, err := g.RecentMessages(context.Background(), 5)

	if err != nil {
		t.Fatalf("RecentMessages err = %v, want nil", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1 (the --- must NOT split the message)", len(msgs))
	}
	if !strings.Contains(msgs[0], "---") {
		t.Fatalf("message lost its '---': %q", msgs[0])
	}
	if !strings.Contains(msgs[0], "after a horizontal rule") {
		t.Fatalf("body lost: %q", msgs[0])
	}
}

func TestRecentMessages_NExceedsCommits(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "only one commit")

	g := New(repo)
	msgs, err := g.RecentMessages(context.Background(), 20)

	if err != nil {
		t.Fatalf("RecentMessages err = %v, want nil", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1 (n=20 but only 1 commit exists)", len(msgs))
	}
}

func TestRecentMessages_LineCap100(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// Each commit is 4 lines. Create 30 commits (120 lines total).
	fourLineMsg := "feat: title\n\nLine two.\nLine three."
	for i := 0; i < 30; i++ {
		makeEmptyCommit(t, repo, fourLineMsg)
	}

	g := New(repo)
	msgs, err := g.RecentMessages(context.Background(), 30)

	if err != nil {
		t.Fatalf("RecentMessages err = %v, want nil", err)
	}
	total := rmTotalLines(msgs)
	if total > 100 {
		t.Fatalf("total lines = %d, want <= 100 (cap exceeded)", total)
	}
	if len(msgs) >= 30 {
		t.Fatalf("len(msgs) = %d, want < 30 (some must be dropped by cap)", len(msgs))
	}
	if len(msgs) < 1 {
		t.Fatal("len(msgs) = 0, want >= 1 (at least one message should fit)")
	}
}

func TestRecentMessages_NotARepo(t *testing.T) {
	// Plain temp dir — no git repo.
	g := New(t.TempDir())
	msgs, err := g.RecentMessages(context.Background(), 5)

	if err != nil {
		t.Fatalf("RecentMessages err = %v, want nil (non-repo exits 128 ⇒ empty)", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("len(msgs) = %d, want 0 (non-repo)", len(msgs))
	}
}

func TestRecentMessages_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := New(t.TempDir())
	_, err := g.RecentMessages(context.Background(), 5)

	if err == nil {
		t.Fatal("RecentMessages err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("RecentMessages err = %v, want it to contain 'git binary not found'", err)
	}
}

func TestRecentMessages_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism

	g := New(t.TempDir())
	_, err := g.RecentMessages(ctx, 5)

	if err == nil {
		t.Fatal("RecentMessages err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RecentMessages err = %v, want context.Canceled", err)
	}
}

// TestRecentMessages_NZeroOrNegative verifies the defensive n<=0 guard returns nil, nil
// without calling git.
func TestRecentMessages_NZeroOrNegative(t *testing.T) {
	g := New(t.TempDir())

	msgs, err := g.RecentMessages(context.Background(), 0)
	if err != nil {
		t.Fatalf("n=0: err = %v, want nil", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("n=0: len = %d, want 0", len(msgs))
	}

	msgs, err = g.RecentMessages(context.Background(), -1)
	if err != nil {
		t.Fatalf("n=-1: err = %v, want nil", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("n=-1: len = %d, want 0", len(msgs))
	}
}
