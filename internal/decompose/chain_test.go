package decompose

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/generate"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/stubtest"
)

// --- Fixture helpers (chn*-prefixed to avoid collisions with arb*/stg*/msg*/un-prefixed) ---

// chnInitRepo creates a git repo in dir with repo-local identity config.
func chnInitRepo(t *testing.T, dir string) {
	t.Helper()
	chnRunGit(t, dir, "init")
	chnRunGit(t, dir, "config", "user.name", "Test")
	chnRunGit(t, dir, "config", "user.email", "test@example.com")
}

// chnWriteFile creates a file at dir/name with the given body.
func chnWriteFile(t *testing.T, dir, name, body string) {
	t.Helper()
	full := dir + string(os.PathSeparator) + name
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("chnWriteFile %s: %v", full, err)
	}
}

// chnStageFile runs git add for name in dir.
func chnStageFile(t *testing.T, dir, name string) {
	t.Helper()
	chnRunGit(t, dir, "add", name)
}

// chnCommitRaw creates an empty commit with the given message.
func chnCommitRaw(t *testing.T, dir, msg string) {
	t.Helper()
	chnRunGit(t, dir, "commit", "--allow-empty", "-m", msg)
}

// chnRunGit executes git -C dir args... and returns trimmed stdout.
func chnRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// chnHeadSHA runs git rev-parse HEAD in dir and returns the trimmed SHA.
func chnHeadSHA(t *testing.T, dir string) string {
	t.Helper()
	return chnRunGit(t, dir, "rev-parse", "HEAD")
}

// chnDeps builds a minimal Deps for chain tests (no ResolveRoles).
func chnDeps(t *testing.T, repo string, msgManifest provider.Manifest) Deps {
	t.Helper()
	return Deps{
		Git:     git.New(repo),
		Config:  config.Defaults(),
		Roles:   RoleManifests{Message: msgManifest},
		Verbose: nil,
	}
}

// chnBuildChain builds a 3-commit chain (C0, C1, C2) with distinct files, returns the parallel
// []CommitInfo + []ChainEntry arrays. Each commit carries a unique file so the tree is easy to reason
// about. Leaves a leftover file "leftover.go" uncommitted in the working tree.
func chnBuildChain(t *testing.T, repo string) ([]CommitInfo, []ChainEntry) {
	t.Helper()
	// C0: commit c0.go
	chnWriteFile(t, repo, "c0.go", "package c0\n")
	chnStageFile(t, repo, "c0.go")
	chnCommitRaw(t, repo, "feat: add c0")
	sha0 := chnHeadSHA(t, repo)
	tree0 := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	msg0 := chnRunGit(t, repo, "log", "--format=%B", "-1")

	// C1: commit c1.go
	chnWriteFile(t, repo, "c1.go", "package c1\n")
	chnStageFile(t, repo, "c1.go")
	chnCommitRaw(t, repo, "feat: add c1")
	sha1 := chnHeadSHA(t, repo)
	tree1 := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	msg1 := chnRunGit(t, repo, "log", "--format=%B", "-1")

	// C2 (tip): commit c2.go
	chnWriteFile(t, repo, "c2.go", "package c2\n")
	chnStageFile(t, repo, "c2.go")
	chnCommitRaw(t, repo, "feat: add c2")
	sha2 := chnHeadSHA(t, repo)
	tree2 := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	msg2 := chnRunGit(t, repo, "log", "--format=%B", "-1")

	// Create leftover (untracked, NOT staged/committed).
	chnWriteFile(t, repo, "leftover.go", "package leftover\n")

	commits := []CommitInfo{
		{SHA: sha0, Subject: "feat: add c0", Files: nil},
		{SHA: sha1, Subject: "feat: add c1", Files: nil},
		{SHA: sha2, Subject: "feat: add c2", Files: nil},
	}
	chainData := []ChainEntry{
		{SHA: sha0, Tree: tree0, Message: msg0, Parent: ""}, // root — parent was pre-repo HEAD (empty)
		{SHA: sha1, Tree: tree1, Message: msg1, Parent: sha0},
		{SHA: sha2, Tree: tree2, Message: msg2, Parent: sha1},
	}
	return commits, chainData
}

// --- Tests ---

func TestResolveArbiter_NullNewCommit(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	chnInitRepo(t, repo)
	commits, chainData := chnBuildChain(t, repo)

	m := stubtest.Manifest(bin, stubtest.Options{Out: "chore: leftover"})
	deps := chnDeps(t, repo, m)

	N := len(chainData)
	tipSHA := chainData[N-1].SHA

	err := resolveArbiter(context.Background(), deps, nil, commits, chainData)
	if err != nil {
		t.Fatalf("resolveArbiter(nil): %v", err)
	}

	// Should have N+1 commits now.
	count := strings.TrimSpace(chnRunGit(t, repo, "rev-list", "--count", "HEAD"))
	if count != "4" { // was 3, now 4
		t.Fatalf("commit count = %s, want 4", count)
	}

	// HEAD's subject should be the generated message.
	subject := chnRunGit(t, repo, "log", "--format=%s", "-1")
	if subject != "chore: leftover" {
		t.Fatalf("HEAD subject = %q, want \"chore: leftover\"", subject)
	}

	// HEAD's parent should be the old tip.
	parent := chnRunGit(t, repo, "log", "--format=%P", "-1")
	if parent != tipSHA {
		t.Fatalf("HEAD parent = %q, want %q", parent, tipSHA)
	}

	// git status should be clean.
	status := chnRunGit(t, repo, "status", "--porcelain")
	if status != "" {
		t.Fatalf("git status not clean: %s", status)
	}

	// HEAD.tree should contain leftover.go.
	tree := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	ls := chnRunGit(t, repo, "ls-tree", "-r", "--name-only", tree)
	if !strings.Contains(ls, "leftover.go") {
		t.Fatalf("HEAD tree missing leftover.go; ls-tree: %s", ls)
	}
}

func TestResolveArbiter_TipAmend(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	chnInitRepo(t, repo)
	commits, chainData := chnBuildChain(t, repo)

	// Use a message manifest that returns something different to prove tip amend doesn't call it.
	m := stubtest.Manifest(bin, stubtest.Options{Out: "SHOULD NOT BE USED"})
	deps := chnDeps(t, repo, m)

	N := len(chainData)
	tip := chainData[N-1]
	tipSHA := tip.SHA
	tipMsg := strings.TrimSpace(tip.Message)
	tipParent := tip.Parent

	target := tipSHA
	err := resolveArbiter(context.Background(), deps, &target, commits, chainData)
	if err != nil {
		t.Fatalf("resolveArbiter(&tipSHA): %v", err)
	}

	// Should STILL have 3 commits (no extra).
	count := strings.TrimSpace(chnRunGit(t, repo, "rev-list", "--count", "HEAD"))
	if count != "3" {
		t.Fatalf("commit count = %s, want 3 (amend should not add)", count)
	}

	// HEAD's subject should be the ORIGINAL tip subject (reused verbatim).
	subject := chnRunGit(t, repo, "log", "--format=%s", "-1")
	if subject != "feat: add c2" {
		t.Fatalf("HEAD subject = %q, want \"feat: add c2\" (original)", subject)
	}

	// HEAD's parent should be the tip's original parent.
	parent := chnRunGit(t, repo, "log", "--format=%P", "-1")
	if parent != tipParent {
		t.Fatalf("HEAD parent = %q, want %q (tip's original parent)", parent, tipParent)
	}

	// HEAD.tree should contain both c2.go AND leftover.go.
	tree := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	ls := chnRunGit(t, repo, "ls-tree", "-r", "--name-only", tree)
	if !strings.Contains(ls, "c2.go") {
		t.Fatalf("HEAD tree missing c2.go; ls-tree: %s", ls)
	}
	if !strings.Contains(ls, "leftover.go") {
		t.Fatalf("HEAD tree missing leftover.go; ls-tree: %s", ls)
	}

	// git status should be clean.
	status := chnRunGit(t, repo, "status", "--porcelain")
	if status != "" {
		t.Fatalf("git status not clean: %s", status)
	}

	// Verify the original tip message was preserved (not regenerated).
	fullMsg := chnRunGit(t, repo, "log", "--format=%B", "-1")
	if strings.TrimSpace(fullMsg) != tipMsg {
		t.Fatalf("message was regenerated; got %q, want %q", strings.TrimSpace(fullMsg), tipMsg)
	}
}

func TestResolveArbiter_MidChainRebuild(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	chnInitRepo(t, repo)
	commits, chainData := chnBuildChain(t, repo)

	m := stubtest.Manifest(bin, stubtest.Options{Out: "SHOULD NOT BE USED"})
	deps := chnDeps(t, repo, m)

	// Target C1 (index 1, i=1).
	sha1 := chainData[1].SHA
	sha0 := chainData[0].SHA

	target := sha1
	err := resolveArbiter(context.Background(), deps, &target, commits, chainData)
	if err != nil {
		t.Fatalf("resolveArbiter(&sha1): %v", err)
	}

	// Should STILL have 3 commits (no extra).
	count := strings.TrimSpace(chnRunGit(t, repo, "rev-list", "--count", "HEAD"))
	if count != "3" {
		t.Fatalf("commit count = %s, want 3 (mid-chain rebuild should not add)", count)
	}

	// C0 should be UNCHANGED (same SHA).
	// The rebuilt C1' and C2' should have new SHAs.
	shas := strings.Split(chnRunGit(t, repo, "log", "--format=%H", "--reverse"), "\n")
	if len(shas) != 3 {
		t.Fatalf("expected 3 SHAs, got %d", len(shas))
	}
	if shas[0] != sha0 {
		t.Fatalf("C0 changed: was %q, now %q (should be unchanged)", sha0, shas[0])
	}
	if shas[1] == sha1 {
		t.Fatal("C1 was NOT rebuilt — same SHA as before")
	}

	// git status should be CLEAN (no leftover reverted).
	status := chnRunGit(t, repo, "status", "--porcelain")
	if status != "" {
		t.Fatalf("git status not clean: %s (fold-at-every-j failed?)", status)
	}

	// FINAL HEAD.tree should contain leftover.go.
	headTree := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	lsHead := chnRunGit(t, repo, "ls-tree", "-r", "--name-only", headTree)
	if !strings.Contains(lsHead, "leftover.go") {
		t.Fatalf("HEAD tree missing leftover.go; ls-tree: %s", lsHead)
	}

	// C1' tree should also contain leftover.go (folded at j==i).
	c1tree := chnRunGit(t, repo, "rev-parse", shas[1]+"^{tree}")
	lsC1 := chnRunGit(t, repo, "ls-tree", "-r", "--name-only", c1tree)
	if !strings.Contains(lsC1, "leftover.go") {
		t.Fatalf("C1' tree missing leftover.go (fold-at-every-j correction not applied); ls-tree: %s", lsC1)
	}

	// C2' tree should also contain leftover.go (folded at j==i+1).
	c2tree := chnRunGit(t, repo, "rev-parse", shas[2]+"^{tree}")
	lsC2 := chnRunGit(t, repo, "ls-tree", "-r", "--name-only", c2tree)
	if !strings.Contains(lsC2, "leftover.go") {
		t.Fatalf("C2' tree missing leftover.go (fold-at-every-j correction not applied); ls-tree: %s", lsC2)
	}

	// Subjects should be preserved verbatim.
	subj1 := chnRunGit(t, repo, "log", "--format=%s", "-2", "--skip=1")
	if !strings.Contains(subj1, "feat: add c1") {
		t.Fatalf("C1' subject wrong: %s", subj1)
	}
	subj2 := chnRunGit(t, repo, "log", "--format=%s", "-1")
	if subj2 != "feat: add c2" {
		t.Fatalf("C2' subject = %q, want \"feat: add c2\"", subj2)
	}
}

func TestResolveArbiter_TargetNotFound(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	chnInitRepo(t, repo)
	commits, chainData := chnBuildChain(t, repo)

	m := stubtest.Manifest(bin, stubtest.Options{Out: "chore: leftover"})
	deps := chnDeps(t, repo, m)

	// Bogus SHA — should degrade to null (new commit path).
	bogus := "0123456789abcdef0123456789abcdef01234567"
	err := resolveArbiter(context.Background(), deps, &bogus, commits, chainData)
	if err != nil {
		t.Fatalf("resolveArbiter(bogus): %v", err)
	}

	// Should have N+1 commits (same as null path).
	count := strings.TrimSpace(chnRunGit(t, repo, "rev-list", "--count", "HEAD"))
	if count != "4" {
		t.Fatalf("commit count = %s, want 4 (target-not-found → new commit)", count)
	}
}

func TestResolveArbiter_CASFailure(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	chnInitRepo(t, repo)
	commits, chainData := chnBuildChain(t, repo)

	m := stubtest.Manifest(bin, stubtest.Options{Out: "chore: leftover"})
	deps := chnDeps(t, repo, m)

	// Move HEAD externally between resolveArbiter's tree-build and its UpdateRefCAS.
	// We do this by making a concurrent commit BEFORE calling resolveArbiter — wait,
	// resolveArbiter is synchronous. Instead, we directly test by manipulating the expectedOld.
	// The cleanest way: make an external commit AFTER the tree is built but before UpdateRefCAS.
	// But resolveArbiter is atomic in its sequence.
	//
	// Instead, we use a wrapper: call resolveArbiter but with an external commit injected
	// between. Since resolveArbiter is synchronous, we need to test the CAS path differently.
	//
	// Strategy: build the chain, then move HEAD externally (make a commit), then call
	// resolveArbiter. The tipSHA in chainData won't match HEAD anymore → CAS fails.

	// Make an external commit to move HEAD.
	chnWriteFile(t, repo, "external.go", "package external\n")
	chnStageFile(t, repo, "external.go")
	chnCommitRaw(t, repo, "external: moved HEAD")
	movedHEAD := chnHeadSHA(t, repo)

	err := resolveArbiter(context.Background(), deps, nil, commits, chainData)
	if err == nil {
		t.Fatal("resolveArbiter returned nil on CAS failure")
	}

	// Should be *generate.CASError (errors.As-able).
	var ce *generate.CASError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %T (%v), want *generate.CASError", err, err)
	}
	if ce.Expected != chainData[2].SHA {
		t.Errorf("CASError.Expected = %q, want %q (tipSHA)", ce.Expected, chainData[2].SHA)
	}
	if ce.Actual != movedHEAD {
		t.Errorf("CASError.Actual = %q, want %q (moved HEAD)", ce.Actual, movedHEAD)
	}

	// HEAD should NOT have been force-updated.
	currentHEAD := chnHeadSHA(t, repo)
	if currentHEAD != movedHEAD {
		t.Errorf("HEAD changed to %q after CAS failure (should be unchanged at %q)", currentHEAD, movedHEAD)
	}
}

func TestResolveArbiter_RescueErrorPropagation(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	chnInitRepo(t, repo)
	commits, chainData := chnBuildChain(t, repo)

	// Stub that sleeps longer than the timeout → generateMessage times out.
	m := stubtest.Manifest(bin, stubtest.Options{Out: "chore: leftover", SleepMS: 2000})
	cfg := config.Defaults()
	cfg.Timeout = 100 * time.Millisecond

	deps := chnDeps(t, repo, m)
	deps.Config = cfg

	err := resolveArbiter(context.Background(), deps, nil, commits, chainData)
	if err == nil {
		t.Fatal("resolveArbiter returned nil on timeout")
	}

	// Should be *generate.RescueError (errors.As-able, NOT wrapped in ErrArbiterResolutionFailed).
	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Fatalf("err = %T (%v), want *generate.RescueError", err, err)
	}

	// ErrArbiterResolutionFailed should NOT be wrapping it.
	if errors.Is(err, ErrArbiterResolutionFailed) {
		t.Error("RescueError was wrapped in ErrArbiterResolutionFailed — should be propagated directly")
	}
}

func TestLeftoverPaths(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLen   int
		wantPaths []string
	}{
		{
			name:      "empty input",
			input:     "",
			wantLen:   0,
			wantPaths: nil,
		},
		{
			name:      "single modified",
			input:     " M leftover.go",
			wantLen:   1,
			wantPaths: []string{"leftover.go"},
		},
		{
			name:      "untracked",
			input:     "?? newfile.go",
			wantLen:   1,
			wantPaths: []string{"newfile.go"},
		},
		{
			name:      "deletion",
			input:     " D gone.go",
			wantLen:   1,
			wantPaths: []string{"gone.go"},
		},
		{
			name:      "multiple entries",
			input:     " M a.go\n?? b.go\n D c.go",
			wantLen:   3,
			wantPaths: []string{"a.go", "b.go", "c.go"},
		},
		{
			name:      "rename takes destination",
			input:     "R100 old.go -> new.go",
			wantLen:   1,
			wantPaths: []string{"new.go"},
		},
		{
			name:      "short lines skipped",
			input:     "??\n M",
			wantLen:   0,
			wantPaths: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := leftoverPaths(tc.input)
			if len(got) != tc.wantLen {
				t.Fatalf("len = %d, want %d; got %v", len(got), tc.wantLen, got)
			}
			for i, w := range tc.wantPaths {
				if i >= len(got) {
					break
				}
				if got[i] != w {
					t.Errorf("paths[%d] = %q, want %q", i, got[i], w)
				}
			}
		})
	}
}

func TestFindTargetIndex(t *testing.T) {
	cd := []ChainEntry{
		{SHA: "aaa"},
		{SHA: "bbb"},
		{SHA: "ccc"},
	}

	tests := []struct {
		sha  string
		want int
	}{
		{"aaa", 0},
		{"bbb", 1},
		{"ccc", 2},
		{"ddd", -1},
		{"", -1},
	}

	for _, tc := range tests {
		got := findTargetIndex(tc.sha, cd)
		if got != tc.want {
			t.Errorf("findTargetIndex(%q) = %d, want %d", tc.sha, got, tc.want)
		}
	}
}

func TestResolveArbiter_CleanTreePostcondition(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	chnInitRepo(t, repo)

	// Build a simple 2-commit chain with a leftover.
	chnWriteFile(t, repo, "a.go", "package a\n")
	chnStageFile(t, repo, "a.go")
	chnCommitRaw(t, repo, "feat: add a")
	sha0 := chnHeadSHA(t, repo)
	tree0 := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	msg0 := chnRunGit(t, repo, "log", "--format=%B", "-1")

	chnWriteFile(t, repo, "b.go", "package b\n")
	chnStageFile(t, repo, "b.go")
	chnCommitRaw(t, repo, "feat: add b")
	sha1 := chnHeadSHA(t, repo)
	tree1 := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	msg1 := chnRunGit(t, repo, "log", "--format=%B", "-1")

	// Leftover.
	chnWriteFile(t, repo, "leftover.go", "package leftover\n")

	commits := []CommitInfo{
		{SHA: sha0, Subject: "feat: add a", Files: nil},
		{SHA: sha1, Subject: "feat: add b", Files: nil},
	}
	chainData := []ChainEntry{
		{SHA: sha0, Tree: tree0, Message: msg0, Parent: ""},
		{SHA: sha1, Tree: tree1, Message: msg1, Parent: sha0},
	}

	m := stubtest.Manifest(bin, stubtest.Options{Out: "chore: leftover"})
	deps := chnDeps(t, repo, m)

	err := resolveArbiter(context.Background(), deps, nil, commits, chainData)
	if err != nil {
		t.Fatalf("resolveArbiter: %v", err)
	}

	// Verify the 3 clean-tree postconditions:
	// 1. git status --porcelain == ""
	status := chnRunGit(t, repo, "status", "--porcelain")
	if status != "" {
		t.Fatalf("git status not clean: %s", status)
	}
	// 2. index == HEAD.tree (write-tree matches HEAD.tree)
	indexTree := chnRunGit(t, repo, "write-tree")
	headTree := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}")
	if indexTree != headTree {
		t.Errorf("index tree %q != HEAD tree %q", indexTree, headTree)
	}
	// 3. working tree == index (no unstaged changes — already verified by clean status)
}
