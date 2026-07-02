package git

// White-box tests for the three history-query primitives in log.go
// (CommitCount / RecentMessages / RecentSubjects). They are package git (NOT
// git_test) so they can call the unexported (g *Git).run seam and compose the
// S2 harness helpers (newTempRepo/seedCommits in gittestutil_test.go) which
// live as package-git _test.go files in this SAME directory. They drive the
// REAL host git binary (git 2.54.0, PRD §20.1 layer 2) — no mocks of git, no
// go-git — with one behavior per Test* function, mirroring
// plumbing_test.go/diff_test.go's posture.

import (
	"strings"
	"testing"
)

// TestCommitCount_MatchesSeededCount proves CommitCount returns the exact
// number of commits in a seeded LINEAR history (reference_impl.md §1:
// `commit_count = git rev-list --count HEAD || 0`). seedCommits commits in
// order, so len(msgs) commits ⇒ CommitCount == len(msgs).
func TestCommitCount_MatchesSeededCount(t *testing.T) {
	g := newTempRepo(t)
	msgs := []string{"a: one", "b: two", "c: three"}
	seedCommits(t, g, msgs)

	got, err := g.CommitCount()
	if err != nil {
		t.Fatalf("CommitCount returned error %v; want nil", err)
	}
	if want := len(msgs); got != want {
		t.Errorf("CommitCount = %d; want %d (seeded count)", got, want)
	}
}

// TestCommitCount_UnbornReturnsZeroNoError proves the (0, nil) rootless
// contract: on an unborn repo `git rev-list --count HEAD` exits 128 with
// "unknown revision", and CommitCount swallows that into (0, nil) — the normal
// root-commit case (FR39), NOT a failure (faithful port of `|| 0`).
func TestCommitCount_UnbornReturnsZeroNoError(t *testing.T) {
	g := newTempRepo(t) // unborn: no commits

	got, err := g.CommitCount()
	if err != nil {
		t.Fatalf("CommitCount on unborn repo returned error %v; want nil", err)
	}
	if got != 0 {
		t.Errorf("CommitCount = %d on unborn repo; want 0 (|| 0 contract)", got)
	}
}

// TestRecentSubjects_NewestFirstInOrder proves RecentSubjects returns subjects
// NEWEST-FIRST in order, truncated to the n newest, and returns ALL when n
// exceeds history. seedCommits commits msgs[0],msgs[1],msgs[2] in order, so
// the LAST (msgs[2]) is HEAD/newest and git log emits newest-first.
func TestRecentSubjects_NewestFirstInOrder(t *testing.T) {
	g := newTempRepo(t)
	seedCommits(t, g, []string{"sub zero", "sub one", "sub two"})

	// Truncated to the 2 newest: [sub two, sub one] (newest-first, in order).
	got2, err := g.RecentSubjects(2)
	if err != nil {
		t.Fatalf("RecentSubjects(2) returned error %v; want nil", err)
	}
	want2 := []string{"sub two", "sub one"}
	if len(got2) != len(want2) {
		t.Fatalf("RecentSubjects(2) = %v; want %v", got2, want2)
	}
	for i := range want2 {
		if got2[i] != want2[i] {
			t.Errorf("RecentSubjects(2)[%d] = %q; want %q (newest-first, in order)", i, got2[i], want2[i])
		}
	}

	// n larger than history: returns ALL, newest-first.
	gotAll, err := g.RecentSubjects(50)
	if err != nil {
		t.Fatalf("RecentSubjects(50) returned error %v; want nil", err)
	}
	wantAll := []string{"sub two", "sub one", "sub zero"}
	if len(gotAll) != len(wantAll) {
		t.Fatalf("RecentSubjects(50) = %v; want %v", gotAll, wantAll)
	}
	for i := range wantAll {
		if gotAll[i] != wantAll[i] {
			t.Errorf("RecentSubjects(50)[%d] = %q; want %q (all, newest-first)", i, gotAll[i], wantAll[i])
		}
	}
}

// TestRecentSubjects_UnbornReturnsEmpty proves the (empty, nil) rootless
// contract: on an unborn repo `git log` exits 128 with "does not have any
// commits yet", and RecentSubjects returns an empty slice with a NIL error —
// the normal root-commit case (FR39), NOT a failure.
func TestRecentSubjects_UnbornReturnsEmpty(t *testing.T) {
	g := newTempRepo(t) // unborn: no commits

	got, err := g.RecentSubjects(20)
	if err != nil {
		t.Fatalf("RecentSubjects on unborn repo returned error %v; want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("RecentSubjects = %v on unborn repo; want an empty slice", got)
	}
}

// TestRecentMessages_RawFormat proves RecentMessages returns the RAW
// `---`-separated %B output (reference_impl.md §1 + §6): newest-first, with
// the `---` separators and the multi-line body's embedded blank lines INTACT
// (no trim — the prompt layer does `sed '/^$/d' | head -100` and the
// multi-line awk scan; trimming here would corrupt the heuristic). The raw
// output is also compared verbatim against a direct g.run to prove NO
// transformation is applied.
func TestRecentMessages_RawFormat(t *testing.T) {
	g := newTempRepo(t)
	// Distinctive subjects + a multi-line body with an internal blank line.
	seedCommits(t, g, []string{
		"feat: multi\n\nFirst body line.\nSecond body line.",
		"fix: single", // LAST committed ⇒ newest ⇒ appears FIRST in git log
	})

	got, err := g.RecentMessages(2)
	if err != nil {
		t.Fatalf("RecentMessages(2) returned error %v; want nil", err)
	}

	// The '---' separator markers must be present (the --format=---%n%B emits
	// a '---' line before each body).
	if !strings.Contains(got, "---") {
		t.Errorf("RecentMessages output missing the '---' separators:\n%s", got)
	}

	// Newest-first: "fix: single" (HEAD) must appear BEFORE "feat: multi".
	idxFix := strings.Index(got, "fix: single")
	idxFeat := strings.Index(got, "feat: multi")
	if idxFix < 0 {
		t.Errorf("RecentMessages missing newest subject %q:\n%s", "fix: single", got)
	}
	if idxFeat < 0 {
		t.Errorf("RecentMessages missing older subject %q:\n%s", "feat: multi", got)
	}
	if idxFix >= 0 && idxFeat >= 0 && idxFix > idxFeat {
		t.Errorf("newest-first order violated: %q (idx %d) after %q (idx %d):\n%s",
			"fix: single", idxFix, "feat: multi", idxFeat, got)
	}

	// The multi-line body content must be present.
	if !strings.Contains(got, "Second body line.") {
		t.Errorf("RecentMessages missing multi-line body content %q:\n%s", "Second body line.", got)
	}

	// The no-trim contract: the multi-line body's embedded blank line (a
	// "\n\n" block) must be INTACT. Trimming would collapse it and corrupt the
	// multi-line awk scan (reference_impl.md §6).
	if !strings.Contains(got, "\n\n") {
		t.Errorf("RecentMessages missing a blank-line block (\\n\\n); the RAW no-trim contract is broken:\n%s", got)
	}

	// Prove NO transformation: RecentMessages(2) must equal the raw
	// `git log --format=---%n%B -2` output byte-for-byte.
	raw, err := g.run("log", "--format=---%n%B", "-2")
	if err != nil {
		t.Fatalf("raw git log --format=---%%n%%B -2 returned error %v; want nil", err)
	}
	if got != raw {
		t.Errorf("RecentMessages(2) != raw git log output (no-transformation contract):\ngot =%q\nraw =%q", got, raw)
	}
}

// TestRecentMessages_UnbornReturnsEmpty proves the ("", nil) rootless
// contract: on an unborn repo `git log` exits 128 with "does not have any
// commits yet", and RecentMessages returns "" with a NIL error — the normal
// root-commit case (FR39), NOT a failure.
func TestRecentMessages_UnbornReturnsEmpty(t *testing.T) {
	g := newTempRepo(t) // unborn: no commits

	got, err := g.RecentMessages(20)
	if err != nil {
		t.Fatalf("RecentMessages on unborn repo returned error %v; want nil", err)
	}
	if got != "" {
		t.Errorf("RecentMessages = %q on unborn repo; want \"\"", got)
	}
}
