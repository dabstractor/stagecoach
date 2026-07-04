package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// ---- Pure-function table tests (no git repo needed) ----

// TestResolveNumstatPath covers every numstat path form — including the brace variants that are
// fiddly to reproduce deterministically via real git. Pure function; no I/O. (PRP §2.)
func TestResolveNumstatPath(t *testing.T) {
	tests := []struct{ in, want, desc string }{
		{"a.txt", "a.txt", "no rename — verbatim"},
		{"src/main.go", "src/main.go", "path with slash, no rename"},
		{"old => new", "new", "simple rename"},
		{"old.txt => new.txt", "new.txt", "simple rename with extensions"},
		{"dir/{a.go => b.go}", "dir/b.go", "brace collapse, prefix only"},
		{"dir/{a => b}.go", "dir/b.go", "brace collapse, prefix + suffix"},
		{"{old => new}", "new", "brace collapse, no prefix/suffix"},
		{"prefix{x => y}suffix", "prefixysuffix", "brace collapse, arbitrary prefix+suffix"},
		{"weird{name}.txt", "weird{name}.txt", "braces but no => → verbatim"},
		{"my file.txt", "my file.txt", "spaces preserved"},
		{"a => b => c", "b => c", "only the FIRST => splits (right side kept verbatim incl. any further =>)"},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			if got := resolveNumstatPath(tc.in); got != tc.want {
				t.Errorf("resolveNumstatPath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// ---- numstatRows: temp-repo integration tests (mirror binary_test.go's idiom) ----

// TestNumstatRows_EditAndBinary asserts counts are parsed, IsBinary is set for a real binary, and
// rows come back sorted by path. (PRP §1/§5.)
func TestNumstatRows_EditAndBinary(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	setIdentityConfig(t, repo)

	// Initial commit with a text file and a real binary (PNG header → numstat content-sniffs as binary).
	writeFile(t, repo, "a.txt", "alpha\nbeta\n")
	writeFile(t, repo, "bin.png", "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	stageFile(t, repo, "a.txt")
	stageFile(t, repo, "bin.png")
	makeEmptyCommit(t, repo, "init")

	// Modify the text file (non-zero added/deleted) and add a second real binary.
	writeFile(t, repo, "a.txt", "alpha\nBETA\nGAMMA\n") // +1 added, -1 deleted
	writeFile(t, repo, "bin2.png", "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR2")
	stageFile(t, repo, "a.txt")
	stageFile(t, repo, "bin2.png")

	rows, err := asRunner(New(repo)).numstatRows(context.Background(), "--cached")
	if err != nil {
		t.Fatalf("numstatRows err = %v, want nil", err)
	}
	if !sort.SliceIsSorted(rows, func(i, j int) bool { return rows[i].Path < rows[j].Path }) {
		t.Errorf("rows not sorted by path: %+v", rows)
	}
	byPath := map[string]numstatRow{}
	for _, r := range rows {
		byPath[r.Path] = r
	}
	if r, ok := byPath["a.txt"]; !ok {
		t.Errorf("a.txt missing: %+v", rows)
	} else if r.IsBinary || r.Added == 0 {
		t.Errorf("a.txt = %+v, want non-binary with Added>0", r)
	}
	if r, ok := byPath["bin2.png"]; !ok {
		t.Errorf("bin2.png missing: %+v", rows)
	} else if !r.IsBinary {
		t.Errorf("bin2.png = %+v, want IsBinary", r)
	}
}

// TestNumstatRows_PureRenameResolvedToDestination asserts the `=>` notation (which git ≥2.9 emits
// EVEN WITHOUT -M) is resolved to the destination path. The key under the size map / skeleton is
// the NEW name, never the "old => new" string. (PRP §0/§2 — the §0 point: => appears without -M.)
func TestNumstatRows_PureRenameResolvedToDestination(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	setIdentityConfig(t, repo)

	writeFile(t, repo, "old.txt", "content\n")
	stageFile(t, repo, "old.txt")
	makeEmptyCommit(t, repo, "init")

	// Pure rename via git mv — no -M passed in diffArgs.
	mvCmd := exec.Command("git", "-C", repo, "mv", "old.txt", "new.txt")
	if out, err := mvCmd.CombinedOutput(); err != nil {
		t.Fatalf("git mv failed: %v\n%s", err, out)
	}

	rows, err := asRunner(New(repo)).numstatRows(context.Background(), "--cached")
	if err != nil {
		t.Fatalf("numstatRows err = %v, want nil", err)
	}
	for _, r := range rows {
		if strings.Contains(r.Path, "=>") {
			t.Errorf("rename not resolved — path still contains =>: %+v", r)
		}
	}
	found := false
	for _, r := range rows {
		if r.Path == "new.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("destination new.txt not present in rows: %+v", rows)
	}
}

// TestNumstatRows_BraceCollapseRename asserts the brace-collapsed form
// `dir/{a.go => b.go}` → `dir/b.go`. The brace form needs rename detection engaged, so -M is
// passed in diffArgs (the caller opts in; numstatRows forwards diffArgs verbatim — it never adds
// -M itself). (PRP §2/§6.)
func TestNumstatRows_BraceCollapseRename(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	setIdentityConfig(t, repo)

	if err := os.MkdirAll(repo+"/dir", 0o755); err != nil {
		t.Fatalf("mkdir dir: %v", err)
	}
	writeFile(t, repo, "dir/a.go", "x\n")
	stageFile(t, repo, "dir/a.go")
	makeEmptyCommit(t, repo, "init")

	mvCmd := exec.Command("git", "-C", repo, "mv", "dir/a.go", "dir/b.go")
	if out, err := mvCmd.CombinedOutput(); err != nil {
		t.Fatalf("git mv failed: %v\n%s", err, out)
	}

	// The brace form needs -M engaged — pass it in diffArgs.
	rows, err := asRunner(New(repo)).numstatRows(context.Background(), "--cached", "-M")
	if err != nil {
		t.Fatalf("numstatRows err = %v, want nil", err)
	}
	for _, r := range rows {
		if strings.ContainsAny(r.Path, "{}=>") {
			t.Errorf("brace rename not resolved — path still contains brace/arrow tokens: %+v", r)
		}
	}
	found := false
	for _, r := range rows {
		if r.Path == "dir/b.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("destination dir/b.go not present in rows: %+v", rows)
	}
}

// TestNumstatRows_PathWithSpaces asserts paths with spaces are preserved. numstat is TAB-separated
// (SplitN(line,"\t",3)), so the whole path — spaces included — is fields[2]. (PRP §5.)
func TestNumstatRows_PathWithSpaces(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	setIdentityConfig(t, repo)

	writeFile(t, repo, "my file.txt", "x\n")
	stageFile(t, repo, "my file.txt")
	makeEmptyCommit(t, repo, "init")
	writeFile(t, repo, "my file.txt", "x\ny\n")
	stageFile(t, repo, "my file.txt")

	rows, err := asRunner(New(repo)).numstatRows(context.Background(), "--cached")
	if err != nil {
		t.Fatalf("numstatRows err = %v, want nil", err)
	}
	found := false
	for _, r := range rows {
		if r.Path == "my file.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("path with spaces not preserved: %+v", rows)
	}
}

// TestNumstatRows_EmptyDiff asserts a clean tree yields 0 rows with no error. (PRP §5.)
func TestNumstatRows_EmptyDiff(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	setIdentityConfig(t, repo)

	writeFile(t, repo, "a.txt", "x\n")
	stageFile(t, repo, "a.txt")
	makeEmptyCommit(t, repo, "init") // clean tree, nothing staged

	rows, err := asRunner(New(repo)).numstatRows(context.Background(), "--cached")
	if err != nil {
		t.Fatalf("numstatRows on empty diff err = %v, want nil", err)
	}
	if len(rows) != 0 {
		t.Errorf("empty diff rows = %+v, want none", rows)
	}
}

// TestNumstatRows_GitBinaryMissing mirrors detectBinaryFiles' infrastructural-error test: a missing
// git binary propagates the unwrapped run error (the run-error convention; PRP §5).
func TestNumstatRows_GitBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "") // makes run()'s LookPath("git") fail

	rows, err := asRunner(New(t.TempDir())).numstatRows(context.Background(), "--cached")
	if err == nil {
		t.Fatal("expected error when git binary is missing, got nil")
	}
	if !strings.Contains(err.Error(), "git binary not found") {
		t.Fatalf("expected error to contain 'git binary not found', got: %v", err)
	}
	if rows != nil {
		t.Fatalf("expected nil rows, got: %v", rows)
	}
}

// TestNumstatRows_ContextCancelled mirrors detectBinaryFiles' context test: a cancelled context
// surfaces context.Canceled from run (the run-error convention; PRP §5).
func TestNumstatRows_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel BEFORE call for determinism

	rows, err := asRunner(New(t.TempDir())).numstatRows(ctx, "--cached")
	if err == nil {
		t.Fatal("expected error when context is cancelled, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if rows != nil {
		t.Fatalf("expected nil rows, got: %v", rows)
	}
}
