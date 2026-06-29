package git

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ---- Fixture helpers (distinct names — no collision with S1/S2/S3 helpers) ----

// setIdentityConfig writes repo-local user.name and user.email so commit-tree
// resolves identity from config (gotcha G6 — no env pollution, robust in CI).
func setIdentityConfig(t *testing.T, dir string) {
	t.Helper()
	for _, kv := range []string{"user.name Test", "user.email test@example.com"} {
		parts := strings.Fields(kv)
		cmd := exec.Command("git", "-C", dir, "config", parts[0], parts[1])
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git config %s failed: %v\n%s", kv, err, out)
		}
	}
}

// writeFile creates a file at dir/name with the given body (0644).
func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("writeFile(%s) failed: %v", name, err)
	}
}

// stageFile runs git add for the named file in dir.
func stageFile(t *testing.T, dir, name string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "add", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add %s failed: %v\n%s", name, err, out)
	}
}

// writeTreeOf runs git write-tree in dir and returns the trimmed TREE_SHA (gotcha G8).
func writeTreeOf(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "write-tree")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git write-tree failed: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// headSHA runs git rev-parse HEAD in dir and returns the trimmed SHA.
func headSHA(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD failed: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// commitMessage retrieves the full commit message for sha via git log --format=%B (gotcha G9).
func commitMessage(t *testing.T, dir, sha string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "log", "--format=%B", "-n", "1", sha)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log --format=%%B failed: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// ---- Test functions ----

func TestCommitTree_RootCommit(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	setIdentityConfig(t, repo)

	writeFile(t, repo, "a.txt", "hello\n")
	stageFile(t, repo, "a.txt")
	tree := writeTreeOf(t, repo)

	g := New(repo)
	sha, err := g.CommitTree(context.Background(), tree, nil, "feat: root commit")
	if err != nil {
		t.Fatalf("CommitTree root: err = %v, want nil", err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40,64}$`).MatchString(sha) {
		t.Fatalf("CommitTree root: sha = %q, want 40/64 hex chars", sha)
	}

	// Verify NO parent line in the commit object (root commit).
	cat := exec.Command("git", "-C", repo, "cat-file", "-p", sha)
	out, _ := cat.CombinedOutput()
	if bytes.Contains(out, []byte("\nparent ")) {
		t.Fatalf("CommitTree root: commit object unexpectedly contains a parent line:\n%s", out)
	}
}

func TestCommitTree_ChildCommit(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	setIdentityConfig(t, repo)

	makeEmptyCommit(t, repo, "initial") // reuse S2's helper to establish HEAD

	writeFile(t, repo, "a.txt", "hello\n")
	stageFile(t, repo, "a.txt")
	tree := writeTreeOf(t, repo)
	parent := headSHA(t, repo)

	g := New(repo)
	sha, err := g.CommitTree(context.Background(), tree, []string{parent}, "feat: child")
	if err != nil {
		t.Fatalf("CommitTree child: err = %v, want nil", err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40,64}$`).MatchString(sha) {
		t.Fatalf("CommitTree child: sha = %q, want 40/64 hex chars", sha)
	}

	// Verify the child commit links the parent.
	cat := exec.Command("git", "-C", repo, "cat-file", "-p", sha)
	out, _ := cat.CombinedOutput()
	if !bytes.Contains(out, []byte("parent "+parent)) {
		t.Fatalf("CommitTree child: commit object missing parent line; got:\n%s", out)
	}
}

func TestCommitTree_MessageViaStdin(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	setIdentityConfig(t, repo)

	writeFile(t, repo, "a.txt", "hello\n")
	stageFile(t, repo, "a.txt")
	tree := writeTreeOf(t, repo)

	// Message with special chars: leading dashes, quotes, newlines — NO trailing \n (gotcha G9).
	msg := "feat: x\n\nbody line\n--weird--leading dashes and \"quotes\" and 'apos'"

	g := New(repo)
	sha, err := g.CommitTree(context.Background(), tree, nil, msg)
	if err != nil {
		t.Fatalf("CommitTree stdin: err = %v, want nil", err)
	}

	retrieved := commitMessage(t, repo, sha)
	if retrieved != strings.TrimSpace(msg) {
		t.Fatalf("CommitTree stdin: message roundtrip mismatch.\ngot:  %q\nwant: %q", retrieved, strings.TrimSpace(msg))
	}
}

func TestCommitTree_BadTree(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	setIdentityConfig(t, repo)

	g := New(repo)
	sha, err := g.CommitTree(context.Background(), "0000000000000000000000000000000000000000", nil, "msg")
	if err == nil {
		t.Fatal("CommitTree bad tree: err = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "git commit-tree: failed") {
		t.Fatalf("CommitTree bad tree: err = %v, want it to contain 'git commit-tree: failed'", err)
	}
	if sha != "" {
		t.Fatalf("CommitTree bad tree: sha = %q, want empty", sha)
	}
}

func TestCommitTree_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes runWithInput's LookPath("git") fail

	g := New(t.TempDir()) // dir need not be a repo; LookPath fails first
	sha, err := g.CommitTree(context.Background(), "tree", nil, "msg")
	if err == nil {
		t.Fatal("CommitTree git-missing: err = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("CommitTree git-missing: err = %v, want it to contain 'git binary not found'", err)
	}
	if sha != "" {
		t.Fatalf("CommitTree git-missing: sha = %q, want empty", sha)
	}
}

func TestCommitTree_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism

	g := New(t.TempDir())
	sha, err := g.CommitTree(ctx, "tree", nil, "msg")
	if err == nil {
		t.Fatal("CommitTree ctx-cancelled: err = nil, want non-nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CommitTree ctx-cancelled: err = %v, want context.Canceled", err)
	}
	if sha != "" {
		t.Fatalf("CommitTree ctx-cancelled: sha = %q, want empty", sha)
	}
}
