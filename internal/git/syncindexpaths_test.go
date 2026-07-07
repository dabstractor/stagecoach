package git

import (
	"context"
	"strings"
	"testing"
)

// indexEntryBlob returns "<mode> <type> <blob>" for path in the LIVE index (git ls-files -s -z parses
// "100644 <sha1> 0\tpath"), giving the oracle for SyncIndexPaths' live-index mutation.
func indexEntryBlob(t *testing.T, dir, path string) string {
	t.Helper()
	out := execGit(t, dir, "ls-files", "-s", "--", path)
	fields := strings.Fields(out)
	if len(fields) < 2 {
		return "" // not in the index
	}
	return fields[1] // the blob SHA
}

// blobContent cats a blob SHA to its file content (oracle for the index entry's byte content).
func blobContent(t *testing.T, dir, blob string) string {
	t.Helper()
	if blob == "" {
		return ""
	}
	return execGit(t, dir, "cat-file", "blob", blob)
}

// TestSyncIndexPaths_ReconcilesOnlyListedPaths verifies SyncIndexPaths overwrites ONLY the listed
// live-index entries to match <tree>, leaving every OTHER staged entry untouched (the surgical
// reconcile that preserves "stage while generating" paths). Snapshot paths=[a.go]; tree holds a.go=A';
// index also has b.go=B (a path NOT in <tree> and NOT in paths) ⇒ after SyncIndexPaths the index has
// a.go=A' (synced) AND b.go=B (preserved).
func TestSyncIndexPaths_ReconcilesOnlyListedPaths(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// live index: a.go="A" (the pre-hook staged snapshot path) + b.go="B" (a "stage while generating"
	// path the hook did NOT touch — must be preserved).
	writeFile(t, repo, "a.go", "A\n")
	writeFile(t, repo, "b.go", "B\n")
	stageFile(t, repo, "a.go")
	stageFile(t, repo, "b.go")

	// tree (the post-hook committed tree): a.go="A-prime" (the hook formatted it); b.go absent in tree.
	writeFile(t, repo, "a.go", "A-prime\n")
	stageFile(t, repo, "a.go")
	tree := writeTree(t, repo)

	// Reset the live index back to the PRE-hook state (a.go=A, b.go=B) to model the divergence F1 fixes.
	resetIndexTo(t, repo, writeTreePreHook(t, repo))

	g := New(repo)
	if err := g.SyncIndexPaths(context.Background(), tree, []string{"a.go"}); err != nil {
		t.Fatalf("SyncIndexPaths err = %v, want nil", err)
	}

	// a.go index entry must now be the tree's blob (A-prime); b.go must be UNCHANGED (B).
	// (execGit trims trailing whitespace, so compare against the trimmed body.)
	if got := blobContent(t, repo, indexEntryBlob(t, repo, "a.go")); got != "A-prime" {
		t.Errorf("a.go index blob = %q, want %q (synced to tree)", got, "A-prime")
	}
	if got := blobContent(t, repo, indexEntryBlob(t, repo, "b.go")); got != "B" {
		t.Errorf("b.go index blob = %q, want %q (preserved — not in paths)", got, "B")
	}
}

// writeTreePreHook builds a tree with a.go="A", b.go="B" (the snapshot state), then leaves the index
// at that state so the test models the live-index/pre-hook divergence. Returns the tree SHA (unused
// by the caller but required to build it via write-tree).
func writeTreePreHook(t *testing.T, repo string) string {
	t.Helper()
	writeFile(t, repo, "a.go", "A\n")
	writeFile(t, repo, "b.go", "B\n")
	stageFile(t, repo, "a.go")
	stageFile(t, repo, "b.go")
	return writeTree(t, repo)
}

// TestSyncIndexPaths_EmptyPathsIsNoOp verifies EMPTY paths ⇒ no index mutation (early return).
func TestSyncIndexPaths_EmptyPathsIsNoOp(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "a.go", "A\n")
	stageFile(t, repo, "a.go")
	before := execGit(t, repo, "ls-files", "-s")

	g := New(repo)
	if err := g.SyncIndexPaths(context.Background(), EmptyTreeSHA, nil); err != nil {
		t.Fatalf("SyncIndexPaths(nil) err = %v, want nil", err)
	}
	if after := execGit(t, repo, "ls-files", "-s"); after != before {
		t.Errorf("SyncIndexPaths(nil) mutated the index:\nbefore: %s\nafter:  %s", before, after)
	}
}

// TestSyncIndexPaths_DeletionRemovesEntry verifies that a path ABSENT in <tree> is removed from the
// live index (the deletion-overlay case), while non-listed entries are preserved.
func TestSyncIndexPaths_DeletionRemovesEntry(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	// live index: a.go + b.go.
	writeFile(t, repo, "a.go", "A\n")
	writeFile(t, repo, "b.go", "B\n")
	stageFile(t, repo, "a.go")
	stageFile(t, repo, "b.go")

	// tree: EMPTY (a.go absent → the reconcile should REMOVE a.go from the index).
	if err := New(repo).SyncIndexPaths(context.Background(), EmptyTreeSHA, []string{"a.go"}); err != nil {
		t.Fatalf("SyncIndexPaths err = %v, want nil", err)
	}
	if got := indexEntryBlob(t, repo, "a.go"); got != "" {
		t.Errorf("a.go index entry = %q, want absent (removed)", got)
	}
	if got := indexEntryBlob(t, repo, "b.go"); got == "" {
		t.Errorf("b.go index entry absent, want preserved")
	}
}
