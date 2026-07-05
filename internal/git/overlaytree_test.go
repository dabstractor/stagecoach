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

// writeTree captures the current index as a tree and returns its SHA (test-local convenience wrapper
// around the execGit oracle). Used to build baseTree/sourceTree/tree1/tStart from staged file sets.
func writeTree(t *testing.T, dir string) string {
	t.Helper()
	return execGit(t, dir, "write-tree")
}

// lsTreeOf runs `git ls-tree -r <tree>` and returns the trimmed stdout (the oracle for a tree's
// full file listing: "<mode> <type> <blob>\t<path>" per line).
func lsTreeOf(t *testing.T, dir, tree string) string {
	t.Helper()
	return execGit(t, dir, "ls-tree", "-r", tree)
}

// resetIndexTo runs `git read-tree <tree>` directly (oracle) so the runner starts from a known index.
func resetIndexTo(t *testing.T, dir, tree string) {
	t.Helper()
	execGit(t, dir, "read-tree", tree)
}

// fileExists reports whether path exists on disk (working-tree oracle for DoesNotTouchWorkingTree).
func fileExists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return err == nil
}

// TestOverlayTreePaths_OverlayOnlyListedPaths verifies that OverlayTreePaths overwrites ONLY the
// listed paths from sourceTree, leaving baseTree's other paths untouched. base={a.go=A,b.go=B};
// source={a.go=A',c.go=C} (b.go unchanged from base); paths=[a.go,c.go] ⇒ result has a.go=A', b.go=B,
// c.go=C.
func TestOverlayTreePaths_OverlayOnlyListedPaths(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// base tree: a.go="A", b.go="B".
	writeFile(t, repo, "a.go", "A\n")
	writeFile(t, repo, "b.go", "B\n")
	stageFile(t, repo, "a.go")
	stageFile(t, repo, "b.go")
	baseTree := writeTree(t, repo)

	// source tree: a.go="A'", c.go="C" (b.go absent in source).
	writeFile(t, repo, "a.go", "A-prime\n")
	writeFile(t, repo, "c.go", "C\n")
	stageFile(t, repo, "a.go")
	stageFile(t, repo, "c.go")
	sourceTree := writeTree(t, repo)

	// Reset the index to baseTree so the runner starts clean.
	resetIndexTo(t, repo, baseTree)

	g := New(repo)
	result, err := g.OverlayTreePaths(context.Background(), baseTree, sourceTree, []string{"a.go", "c.go"})
	if err != nil {
		t.Fatalf("OverlayTreePaths err = %v, want nil", err)
	}

	// Oracle: ls-tree -r result must show a.go=A' blob, b.go=B blob (base value, NOT removed), c.go=C blob.
	got := lsTreeOf(t, repo, result)
	wantPaths := map[string]string{ // path → substring of blob content (verified via blob cat below)
		"a.go": "A-prime\n",
		"b.go": "B\n",
		"c.go": "C\n",
	}
	for path, wantBody := range wantPaths {
		blob := treePathBlob(t, repo, result, path)
		if blob == "" {
			t.Fatalf("result tree missing %s; ls-tree=\n%s", path, got)
		}
		body := catFileBlob(t, repo, blob)
		if body != wantBody {
			t.Fatalf("result %s = %q, want %q", path, body, wantBody)
		}
	}
}

// TestOverlayTreePaths_DeletionOverlay verifies that a path ABSENT in sourceTree is removed from the
// result (deletion-overlay via --force-remove). base={a.go,b.go}; source={a.go} (b.go absent);
// paths=[b.go] ⇒ result has a.go present (base value), b.go ABSENT.
func TestOverlayTreePaths_DeletionOverlay(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// base tree: a.go, b.go.
	writeFile(t, repo, "a.go", "A\n")
	writeFile(t, repo, "b.go", "B\n")
	stageFile(t, repo, "a.go")
	stageFile(t, repo, "b.go")
	baseTree := writeTree(t, repo)

	// source tree: only a.go (b.go absent).
	resetIndexTo(t, repo, baseTree) // clear index, then stage only a.go
	execGit(t, repo, "rm", "--cached", "-q", "b.go")
	sourceTree := writeTree(t, repo)

	// Reset index to baseTree so the runner starts clean.
	resetIndexTo(t, repo, baseTree)

	g := New(repo)
	result, err := g.OverlayTreePaths(context.Background(), baseTree, sourceTree, []string{"b.go"})
	if err != nil {
		t.Fatalf("OverlayTreePaths err = %v, want nil", err)
	}

	// Oracle: result must contain a.go but NOT b.go.
	if blob := treePathBlob(t, repo, result, "a.go"); blob == "" {
		t.Fatalf("result missing a.go; ls-tree=\n%s", lsTreeOf(t, repo, result))
	}
	if blob := treePathBlob(t, repo, result, "b.go"); blob != "" {
		t.Fatalf("result still contains b.go (deletion-overlay failed); ls-tree=\n%s", lsTreeOf(t, repo, result))
	}
}

// TestOverlayTreePaths_EmptyPathsNoop verifies that empty paths returns baseTree verbatim AND mutates
// neither the index nor the object store (no read-tree ran). Covers both nil and a zero-length slice.
func TestOverlayTreePaths_EmptyPathsNoop(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// base tree.
	writeFile(t, repo, "a.go", "A\n")
	stageFile(t, repo, "a.go")
	baseTree := writeTree(t, repo)

	// A different source tree (must NOT be consulted).
	writeFile(t, repo, "a.go", "A-prime\n")
	stageFile(t, repo, "a.go")
	sourceTree := writeTree(t, repo)

	// Capture the index BEFORE (reset to baseTree so the "before" state is well-defined).
	resetIndexTo(t, repo, baseTree)
	indexBefore := writeTree(t, repo)

	g := New(repo)

	// nil paths.
	if result, err := g.OverlayTreePaths(context.Background(), baseTree, sourceTree, nil); err != nil {
		t.Fatalf("OverlayTreePaths(nil) err = %v, want nil", err)
	} else if result != baseTree {
		t.Fatalf("OverlayTreePaths(nil) = %q, want baseTree %q (verbatim)", result, baseTree)
	}

	// Zero-length slice paths.
	if result, err := g.OverlayTreePaths(context.Background(), baseTree, sourceTree, []string{}); err != nil {
		t.Fatalf("OverlayTreePaths([]) err = %v, want nil", err)
	} else if result != baseTree {
		t.Fatalf("OverlayTreePaths([]) = %q, want baseTree %q (verbatim)", result, baseTree)
	}

	// Index MUST be unchanged (no read-tree ran against sourceTree).
	indexAfter := writeTree(t, repo)
	if indexAfter != indexBefore {
		t.Fatalf("index changed after empty-paths call: before=%s after=%s (read-tree must NOT have run)", indexBefore, indexAfter)
	}
}

// TestOverlayTreePaths_MidChainLeftoverSimulation verifies the exact fold the FR-M1d arbiter (P1.M1.T2.S1)
// will rely on: leftoverPaths = DiffTreeNames(tree1, tStart); OverlayTreePaths(tree1, tStart, leftoverPaths)
// must yield a tree whose CONTENT == tStart (every leftover folded in from tStart).
func TestOverlayTreePaths_MidChainLeftoverSimulation(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// tStart: full tree (a.go + b.go + c.go).
	writeFile(t, repo, "a.go", "A\n")
	writeFile(t, repo, "b.go", "B\n")
	writeFile(t, repo, "c.go", "C\n")
	stageFile(t, repo, "a.go")
	stageFile(t, repo, "b.go")
	stageFile(t, repo, "c.go")
	tStart := writeTree(t, repo)

	// tree1: a subset (only a.go staged). Reset index to empty, stage only a.go.
	resetIndexTo(t, repo, EmptyTreeSHA)
	writeFile(t, repo, "a.go", "A\n")
	stageFile(t, repo, "a.go")
	tree1 := writeTree(t, repo)

	// leftoverPaths = the paths that differ between tree1 and tStart (oracle: diff-tree).
	leftoverPaths := strings.Split(
		strings.TrimSpace(execGit(t, repo, "diff-tree", "-r", "--name-only", "--no-commit-id", tree1, tStart)),
		"\n",
	)
	if len(leftoverPaths) == 0 {
		t.Fatalf("diff-tree(tree1,tStart) empty; tree1=%s tStart=%s", tree1, tStart)
	}

	// Reset index to tree1 so the runner starts clean.
	resetIndexTo(t, repo, tree1)

	g := New(repo)
	result, err := g.OverlayTreePaths(context.Background(), tree1, tStart, leftoverPaths)
	if err != nil {
		t.Fatalf("OverlayTreePaths err = %v, want nil", err)
	}

	// The folded result must have NO path differences from tStart (leftovers folded in ⇒ content == tStart).
	diffNames, err := g.DiffTreeNames(context.Background(), result, tStart)
	if err != nil {
		t.Fatalf("DiffTreeNames(result, tStart) err = %v, want nil", err)
	}
	if len(diffNames) != 0 {
		t.Fatalf("OverlayTreePaths fold failed; DiffTreeNames(result, tStart) = %v, want empty", diffNames)
	}
}

// TestOverlayTreePaths_BadTree verifies that a bogus baseTree SHA surfaces as a wrapped error
// (mirrors readtree_test.go TestReadTree_BadTree).
func TestOverlayTreePaths_BadTree(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	g := New(repo)
	_, err := g.OverlayTreePaths(context.Background(),
		"0000000000000000000000000000000000000000",
		"0000000000000000000000000000000000000000",
		[]string{"a.go"})
	if err == nil {
		t.Fatal("OverlayTreePaths err = nil, want non-nil (bad tree SHA)")
	}
	if !strings.Contains(err.Error(), "git read-tree") {
		t.Fatalf("OverlayTreePaths err = %v, want it to contain 'git read-tree'", err)
	}
}

// TestOverlayTreePaths_NotARepo verifies a non-repo directory surfaces a non-nil error (mirrors
// readtree_test.go TestReadTree_NotARepo).
func TestOverlayTreePaths_NotARepo(t *testing.T) {
	g := New(t.TempDir()) // plain dir, NOT a git repo
	_, err := g.OverlayTreePaths(context.Background(),
		"2e81171448eb9f2ee3821e3d447aa6b2fe3ddba1",
		"2e81171448eb9f2ee3821e3d447aa6b2fe3ddba1",
		[]string{"a.go"})
	if err == nil {
		t.Fatal("OverlayTreePaths err = nil, want non-nil (non-repo)")
	}
	if !strings.Contains(err.Error(), "git read-tree") {
		t.Fatalf("OverlayTreePaths err = %v, want it to contain 'git read-tree'", err)
	}
}

// TestOverlayTreePaths_GitBinaryMissing verifies a missing git binary surfaces as a non-nil error
// containing "git binary not found" (mirrors readtree_test.go TestReadTree_GitBinaryMissing).
func TestOverlayTreePaths_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	g := New(t.TempDir())
	_, err := g.OverlayTreePaths(context.Background(), "tree", "tree", []string{"a.go"})
	if err == nil {
		t.Fatal("OverlayTreePaths err = nil, want non-nil (git binary not found)")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("OverlayTreePaths err = %v, want it to contain 'git binary not found'", err)
	}
}

// TestOverlayTreePaths_ContextCancelled verifies a pre-cancelled context surfaces as context.Canceled
// (mirrors readtree_test.go TestReadTree_ContextCancelled). Non-empty paths is required so the call
// reaches the first g.run (read-tree).
func TestOverlayTreePaths_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism

	g := New(t.TempDir())
	_, err := g.OverlayTreePaths(ctx, "tree", "tree", []string{"a.go"})
	if err == nil {
		t.Fatal("OverlayTreePaths err = nil, want non-nil (context cancelled)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("OverlayTreePaths err = %v, want errors.Is(err, context.Canceled)", err)
	}
}

// TestOverlayTreePaths_DoesNotTouchWorkingTree verifies OverlayTreePaths leaves working-tree files
// UNCHANGED on disk — it mutates ONLY .git/index + the object store (mirrors freezeworkingtree_test.go
// TestFreezeWorkingTree_LeavesWorkingTreeUnchanged).
func TestOverlayTreePaths_DoesNotTouchWorkingTree(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// Build base + source trees from a.go / b.go (staged then write-tree).
	writeFile(t, repo, "a.go", "A\n")
	stageFile(t, repo, "a.go")
	baseTree := writeTree(t, repo)

	writeFile(t, repo, "b.go", "B\n")
	stageFile(t, repo, "b.go")
	sourceTree := writeTree(t, repo)

	// A working-tree file that is NOT part of any tree — it must remain UNCHANGED on disk.
	writeFile(t, repo, "untouched.txt", "keep-me\n")

	// Reset index to baseTree so the runner starts clean.
	resetIndexTo(t, repo, baseTree)

	g := New(repo)
	if _, err := g.OverlayTreePaths(context.Background(), baseTree, sourceTree, []string{"b.go"}); err != nil {
		t.Fatalf("OverlayTreePaths err = %v, want nil", err)
	}

	// Oracle: untouched.txt must still exist with its original content.
	body, err := os.ReadFile(filepath.Join(repo, "untouched.txt"))
	if err != nil {
		t.Fatalf("untouched.txt missing after OverlayTreePaths: %v (working tree MUST NOT be touched)", err)
	}
	if string(body) != "keep-me\n" {
		t.Fatalf("untouched.txt = %q, want \"keep-me\\n\" (working tree MUST NOT be touched)", string(body))
	}

	// a.go was written by writeFile but never removed — it must still exist on disk.
	if !fileExists(t, filepath.Join(repo, "a.go")) {
		t.Fatal("a.go missing on disk after OverlayTreePaths (working tree MUST NOT be touched)")
	}
}

// treePathBlob returns the blob SHA for path within tree (oracle via `git ls-tree --object-only`), or
// "" if the path is absent. Used to assert per-path content of an OverlayTreePaths result.
func treePathBlob(t *testing.T, dir, tree, path string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "ls-tree", "-r", "--object-only", tree, "--", path).Output()
	if err != nil {
		t.Fatalf("git ls-tree --object-only %s %s failed: %v", tree, path, err)
	}
	return strings.TrimSpace(string(out))
}

// catFileBlob returns the RAW (untrimmed) contents of a blob SHA (oracle). execGit trims whitespace,
// which would corrupt body comparisons — so read the blob directly here.
func catFileBlob(t *testing.T, dir, blob string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "cat-file", "blob", blob).Output()
	if err != nil {
		t.Fatalf("git cat-file blob %s failed: %v", blob, err)
	}
	return string(out)
}
