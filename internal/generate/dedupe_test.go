package generate

// White-box test for the duplicate-rejection primitives firstLine + isDuplicate
// (P1.M6.T1.S2), matching the internal/provider/parse_test.go and
// internal/prompt/payload_test.go house convention: the _test.go file is
// `package generate` (NOT `package generate_test`) so it sits in the same
// package as the (exported but pure) functions under test. It exercises PURE
// string functions over string / []string, so it needs stdlib `testing` ONLY —
// NO testify, NO internal/git, NO os/exec (no real-git integration test is
// needed). It uses table-driven `t.Run(tc.name, ...)` subtests with the
// `tc := tc` capture and asserts exact return values with `t.Errorf` using
// `%q` (so whitespace/newline diffs are visible) for strings and `%v` for
// bools, exactly as parse_test.go does.

import "testing"

// TestFirstLine pins the head-1 + trim contract (PRD FR30; reference_impl.md
// §1 step 4 + §2: `subject = head -1 commit_msg`). The table covers the full
// MOCKING matrix: single-line, multi-line -> line 1, leading/trailing
// whitespace trimmed, CRLF (TrimSpace strips the trailing '\r'), empty,
// whitespace-only-first-line, and trailing-newline-only.
func TestFirstLine(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want string
	}{
		{
			name: "single line no newline -> whole string trimmed",
			msg:  "feat: add parser",
			want: "feat: add parser",
		},
		{
			name: "multi-line -> only line 1 trimmed",
			msg:  "feat: multi\n\nFirst body.\nSecond body.",
			want: "feat: multi",
		},
		{
			name: "leading/trailing whitespace on line 1 -> trimmed",
			msg:  "  feat: trimmed  ",
			want: "feat: trimmed",
		},
		{
			name: "CRLF first line -> trailing CR stripped by trim",
			msg:  "feat: crlf\r\nbody",
			want: "feat: crlf",
		},
		{
			name: "empty input -> empty",
			msg:  "",
			want: "",
		},
		{
			name: "whitespace-only first line -> empty",
			msg:  "   \nbody",
			want: "",
		},
		{
			name: "trailing newline only -> line 1 trimmed",
			msg:  "feat: x\n",
			want: "feat: x",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := firstLine(tc.msg)
			if got != tc.want {
				t.Errorf("firstLine(%q) = %q, want %q", tc.msg, got, tc.want)
			}
		})
	}
}

// TestIsDuplicate pins the EXACT, case-sensitive, byte-equal membership
// contract (PRD FR31/FR32 "exactly matches one of the 50"). The table covers
// the full MOCKING matrix: exact-match-true, no-match-false, case-differs
// (false — case-sensitive), near/paraphrase (false — fuzzy NOT done, proving
// the v1.1 boundary has a regression baseline), internal-whitespace-differs
// (false — exact = byte-equal), empty-slice-false, nil-slice-false, and
// empty-subject-false.
func TestIsDuplicate(t *testing.T) {
	tests := []struct {
		name     string
		subject  string
		subjects []string
		want     bool
	}{
		{
			name:     "exact match -> true",
			subject:  "feat: add parser",
			subjects: []string{"fix: bug", "feat: add parser", "docs: readme"},
			want:     true,
		},
		{
			name:     "no match -> false",
			subject:  "feat: add parser",
			subjects: []string{"fix: bug", "docs: readme", "chore: bump"},
			want:     false,
		},
		{
			name:     "case differs -> false (case-sensitive)",
			subject:  "Feat: Add Parser",
			subjects: []string{"feat: add parser"},
			want:     false,
		},
		{
			name:     "near/paraphrase plural vs singular -> false (fuzzy NOT done)",
			subject:  "feat: add parsers",
			subjects: []string{"feat: add parser"},
			want:     false,
		},
		{
			name:     "near/paraphrase article wording -> false (fuzzy NOT done)",
			subject:  "refactor the parser module",
			subjects: []string{"refactor parser module"},
			want:     false,
		},
		{
			name:     "internal whitespace differs (two vs one space) -> false (byte-equal)",
			subject:  "feat: add  parser",
			subjects: []string{"feat: add parser"},
			want:     false,
		},
		{
			name:     "empty subjects slice -> false",
			subject:  "feat: add parser",
			subjects: []string{},
			want:     false,
		},
		{
			name:     "nil subjects slice -> false",
			subject:  "feat: add parser",
			subjects: nil,
			want:     false,
		},
		{
			name:     "empty subject vs non-empty list -> false",
			subject:  "",
			subjects: []string{"feat: add parser", "fix: bug"},
			want:     false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := isDuplicate(tc.subject, tc.subjects)
			if got != tc.want {
				t.Errorf("isDuplicate(%q, %v) = %v, want %v", tc.subject, tc.subjects, got, tc.want)
			}
		})
	}
}

// TestDedup_FirstLineThenIsDuplicate_WhitespaceTrimmed is the END-TO-END
// MOCKING proof of the orchestrator's intended pipeline (decisions.md §3):
// `subject = firstLine(msg); if !isDuplicate(subject, recentSubjects) { goto
// COMMIT }`. A multi-line message whose first line bears leading/trailing
// whitespace passes through firstLine (trimmed to the subject) and isDuplicate
// then EXACT-matches a history subject (true), covering the
// 'leading/trailing whitespace ignored via trim' + 'firstLine of a multi-line
// message returns only line 1' + 'exact match true' bullets together. A
// NON-matching multi-line message against the SAME recent set returns false.
func TestDedup_FirstLineThenIsDuplicate_WhitespaceTrimmed(t *testing.T) {
	recent := []string{"feat: add parser", "fix: bug"}

	// Matching pipeline: whitespace-bearing multi-line first line -> trimmed ->
	// exact match -> true.
	const matchingMsg = "  feat: add parser  \n\nBody line."
	subj := firstLine(matchingMsg)
	if want := "feat: add parser"; subj != want {
		t.Fatalf("firstLine(%q) = %q, want %q (must trim to the subject before dedupe)", matchingMsg, subj, want)
	}
	if got := isDuplicate(subj, recent); !got {
		t.Errorf("isDuplicate(%q, %v) = %v, want true (trimmed first line must exact-match a history subject)", subj, recent, got)
	}

	// Non-matching pipeline: a different subject in the same recent set -> false.
	const nonMatchingMsg = "docs: update readme\nbody"
	otherSubj := firstLine(nonMatchingMsg)
	if want := "docs: update readme"; otherSubj != want {
		t.Fatalf("firstLine(%q) = %q, want %q", nonMatchingMsg, otherSubj, want)
	}
	if got := isDuplicate(otherSubj, recent); got {
		t.Errorf("isDuplicate(%q, %v) = %v, want false (no matching history subject)", otherSubj, recent, got)
	}
}
