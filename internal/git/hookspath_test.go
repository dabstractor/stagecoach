package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitDo runs a git subcommand against dir using minGitEnv, failing the test on error.
func gitDo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = minGitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestHooksPath_DefaultLayout(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	g := New(repo)
	got, err := g.HooksPath(context.Background())
	if err != nil {
		t.Fatalf("HooksPath err = %v, want nil", err)
	}
	want := filepath.Join(repo, ".git", "hooks")
	if got != want {
		t.Fatalf("HooksPath = %q, want %q", got, want)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("HooksPath = %q, want an absolute path", got)
	}
}

func TestHooksPath_CoreHooksPath_Relative(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	gitDo(t, repo, "config", "core.hooksPath", "myhooks")

	g := New(repo)
	got, err := g.HooksPath(context.Background())
	if err != nil {
		t.Fatalf("HooksPath err = %v, want nil", err)
	}
	want := filepath.Join(repo, "myhooks")
	if got != want {
		t.Fatalf("HooksPath = %q, want %q", got, want)
	}
}

func TestHooksPath_CoreHooksPath_Absolute(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	abs := t.TempDir()
	gitDo(t, repo, "config", "core.hooksPath", abs)

	g := New(repo)
	got, err := g.HooksPath(context.Background())
	if err != nil {
		t.Fatalf("HooksPath err = %v, want nil", err)
	}
	want := filepath.Clean(abs)
	if got != want {
		t.Fatalf("HooksPath = %q, want %q", got, want)
	}
}

func TestHooksPath_FromSubdirectory(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	sub := filepath.Join(repo, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	g := New(sub)
	got, err := g.HooksPath(context.Background())
	if err != nil {
		t.Fatalf("HooksPath err = %v, want nil", err)
	}
	want := filepath.Join(repo, ".git", "hooks")
	if got != want {
		t.Fatalf("HooksPath = %q, want %q", got, want)
	}
}

func TestHooksPath_LinkedWorktree(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	makeEmptyCommit(t, repo, "init") // worktree add requires >= 1 commit

	wt := filepath.Join(t.TempDir(), "wt")
	gitDo(t, repo, "worktree", "add", wt)

	g := New(wt)
	got, err := g.HooksPath(context.Background())
	if err != nil {
		t.Fatalf("HooksPath err = %v, want nil", err)
	}
	want := filepath.Join(repo, ".git", "hooks") // common dir, not the worktree's private dir
	if got != want {
		t.Fatalf("HooksPath = %q, want %q (common dir hooks)", got, want)
	}
}

func TestHooksPath_NonRepo(t *testing.T) {
	g := New(t.TempDir()) // no git init
	_, err := g.HooksPath(context.Background())
	if err == nil {
		t.Fatal("HooksPath err = nil, want non-nil (non-repo)")
	}
}

func TestHooksPath_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := New(t.TempDir())
	_, err := g.HooksPath(context.Background())
	if err == nil {
		t.Fatal("HooksPath err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("HooksPath err = %v, want it to contain 'git binary not found'", err)
	}
}
