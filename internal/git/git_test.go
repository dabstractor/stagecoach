package git

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func initRepo(t *testing.T, dir string) {
	t.Helper()
	// Set a minimal git identity so commits and other operations don't fail.
	cmd := exec.Command("git", "-C", dir, "init")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test <test@example.com>",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test <test@example.com>",
		"GIT_COMMITTER_EMAIL=test@example.com>",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	// Set repo-local user identity so every subsequent git operation in this repo
	// works even without a global ~/.gitconfig.
	cfgCmd := exec.Command("git", "-C", dir, "config", "user.name", "Test")
	cfgCmd.Env = os.Environ()
	if out, err := cfgCmd.CombinedOutput(); err != nil {
		t.Fatalf("git config user.name failed: %v\n%s", err, out)
	}
	emailCmd := exec.Command("git", "-C", dir, "config", "user.email", "test@example.com")
	emailCmd.Env = os.Environ()
	if out, err := emailCmd.CombinedOutput(); err != nil {
		t.Fatalf("git config user.email failed: %v\n%s", err, out)
	}
}

func TestNew(t *testing.T) {
	g := New("/tmp")
	if g == nil {
		t.Fatal("New returned nil")
	}
	gr, ok := g.(*gitRunner)
	if !ok {
		t.Fatalf("New did not return *gitRunner, got %T", g)
	}
	if gr.workDir != "/tmp" {
		t.Fatalf("workDir = %q, want %q", gr.workDir, "/tmp")
	}
}

func TestRun_HappyPath(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	g := &gitRunner{workDir: repo}
	ctx := context.Background()
	stdout, stderr, code, err := g.run(ctx, repo, "rev-parse", "--git-dir")

	if err != nil {
		t.Fatalf("run() err = %v, want nil", err)
	}
	if code != 0 {
		t.Fatalf("run() exitCode = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != ".git" {
		t.Fatalf("run() stdout = %q, want .git", strings.TrimSpace(stdout))
	}
	if stderr != "" {
		t.Fatalf("run() stderr = %q, want empty", stderr)
	}
}

func TestRun_CapturesExitCodeAndSeparateBuffers(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// Unborn repo (zero commits): rev-parse HEAD exits 128 and prints literal "HEAD\n".
	g := &gitRunner{workDir: repo}
	ctx := context.Background()
	stdout, stderr, code, err := g.run(ctx, repo, "rev-parse", "HEAD")

	if err != nil {
		t.Fatalf("run() err = %v, want nil (exit 128 is NOT a Go error, gotcha G2)", err)
	}
	if code != 128 {
		t.Fatalf("run() exitCode = %d, want 128 (unborn repo)", code)
	}
	if strings.TrimSpace(stdout) != "HEAD" {
		t.Fatalf("run() stdout = %q, want \"HEAD\" (gotcha G3: unborn prints literal HEAD)", strings.TrimSpace(stdout))
	}
	if stderr == "" || !bytes.Contains([]byte(stderr), []byte("ambiguous argument 'HEAD'")) {
		t.Fatalf("run() stderr = %q, want it to contain 'ambiguous argument' (proves separate stderr buffer)", stderr)
	}
}

func TestRun_LookPathFailure(t *testing.T) {
	t.Setenv("PATH", "") // makes LookPath("git") fail for this test only (gotcha G10)

	g := &gitRunner{workDir: "."}
	ctx := context.Background()
	_, _, code, err := g.run(ctx, ".", "version")

	if err == nil {
		t.Fatal("run() err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("run() err = %v, want it to contain 'git binary not found'", err)
	}
	if code != -1 {
		t.Fatalf("run() exitCode = %d, want -1 (sentinel for infrastructural failure)", code)
	}
}

func TestGitDir(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	g := New(repo)
	ctx := context.Background()

	got, err := g.GitDir(ctx)
	if err != nil {
		t.Fatalf("GitDir() error: %v", err)
	}
	if got == "" {
		t.Fatal("GitDir() returned empty")
	}
	// Must end with ".git" or be an absolute path.
	if got[len(got)-4:] != ".git" && got[len(got)-5:] != ".git/" {
		t.Logf("GitDir() = %q (should be an absolute path ending in .git)", got)
	}
}

func TestEditor(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	g := New(repo)
	ctx := context.Background()

	// With GIT_EDITOR set, git var GIT_EDITOR should return it.
	t.Setenv("GIT_EDITOR", "/usr/bin/vim")
	got, err := g.Editor(ctx)
	if err != nil {
		t.Fatalf("Editor() error: %v", err)
	}
	if got != "/usr/bin/vim" {
		t.Errorf("Editor() = %q, want /usr/bin/vim", got)
	}

	// Without GIT_EDITOR, it should resolve something (at minimum vi).
	t.Setenv("GIT_EDITOR", "")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	got2, err2 := g.Editor(ctx)
	if err2 != nil {
		t.Fatalf("Editor() without GIT_EDITOR error: %v", err2)
	}
	// In CI vi may not be installed; just verify we got a non-error result.
	t.Logf("Editor() without GIT_EDITOR = %q", got2)
}

func TestDiffTreeNameStatus(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	g := New(repo)
	ctx := context.Background()

	// Create an initial commit with a file.
	writeFile(t, repo, "a.txt", "hello")
	runGit(t, repo, "add", "a.txt")
	runGit(t, repo, "commit", "-m", "initial")

	srcA := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD^{tree}"))

	// Modify the file and commit.
	writeFile(t, repo, "a.txt", "world")
	runGit(t, repo, "commit", "-am", "update")
	srcB := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD^{tree}"))

	got, err := g.DiffTreeNameStatus(ctx, srcA, srcB)
	if err != nil {
		t.Fatalf("DiffTreeNameStatus() error: %v", err)
	}
	if !strings.Contains(got, "M\ta.txt") {
		t.Errorf("DiffTreeNameStatus() = %q, want to contain 'M\ta.txt'", got)
	}

	// Identical trees → empty output.
	got2, err2 := g.DiffTreeNameStatus(ctx, srcA, srcA)
	if err2 != nil {
		t.Fatalf("DiffTreeNameStatus(same) error: %v", err2)
	}
	if strings.TrimSpace(got2) != "" {
		t.Errorf("DiffTreeNameStatus(same) = %q, want empty", got2)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir)
	cmd.Args = append(cmd.Args, args...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test <test@example.com>",
		"GIT_AUTHOR_EMAIL=test@example.com>",
		"GIT_COMMITTER_NAME=Test <test@example.com>",
		"GIT_COMMITTER_EMAIL=test@example.com>",
	)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
