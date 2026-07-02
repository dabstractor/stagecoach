// This file adds the two duplicate-rejection primitives that sit at the bottom
// of the generate OUTER loop (P1.M6.T1.S2): firstLine extracts the generated
// message's subject (PRD FR30; reference_impl.md §1 step 4 + §2:
// `subject = head -1 commit_msg`) and isDuplicate performs the EXACT,
// case-sensitive membership test against the recent-subject set (PRD FR31/FR32;
// decisions.md §3: `if !isDuplicate(subject, recentSubjects) { goto COMMIT }`).
// They are PURE functions over string / []string with NO git binary, NO exec,
// NO filesystem, NO time — the recent-subjects []string comes from the shipped
// dependency git.RecentSubjects(50) (P1.M3.T4.S1), already trimmed. It uses a
// plain "package generate" line because [generate.go] (P1.M6.T1.S1) OWNS the
// // Package generate doc comment, mirroring how internal/git/log.go defers to
// git.go and internal/prompt/system.go / payload.go defer to examples.go.
package generate

import "strings"

// firstLine extracts the generated message's subject — the FIRST
// '\n'-delimited line of msg, strings.TrimSpace'd (PRD FR30: "Extract the
// generated subject (first line of the message)"; decisions.md §3:
// `subject = firstLine(msg)`; reference_impl.md §1 step 4 + §2:
// `subject = head -1 commit_msg`).
//
// It is the generated-subject half of the trim-responsibility boundary
// (decisions.md §3): firstLine trims the subject that is then fed to
// [isDuplicate], while git.RecentSubjects trims each history subject, so
// isDuplicate trims NOTHING. It uses strings.IndexByte(msg, '\n')
// (allocation-free head -1 — NOT strings.Split, which allocates and
// mishandles trailing-newline/empty), then strings.TrimSpace on the resulting
// first line. CRLF is handled for free: for "x\r\nbody", IndexByte('\n')
// yields the line "x\r" and TrimSpace strips the trailing '\r' (it is
// whitespace) -> "x". Empty input -> ""; a whitespace-only first line
// ("   \nbody") -> "" (TrimSpace collapses it); "trailing\n" -> "trailing".
func firstLine(msg string) string {
	if i := strings.IndexByte(msg, '\n'); i != -1 {
		msg = msg[:i] // head -1: take the substring up to the FIRST newline
	}
	return strings.TrimSpace(msg) // trim leading/trailing whitespace from that first line
}

// isDuplicate reports whether subject EXACTLY (byte-equal, case-sensitive)
// matches one of the recent commit subjects (PRD FR31: "Fetch the last 50
// commit subjects"; FR32: "If the subject exactly matches one of the 50,
// retry"; decisions.md §3: the `if !isDuplicate(subject, recentSubjects) {
// goto COMMIT }` call site; reference_impl.md §1/§2: reject if subject in
// `git log --format=%s -50`).
//
// subjects is the []string returned by the shipped dependency
// git.RecentSubjects(50) (P1.M3.T4.S1): trimmed, newest-first, empties
// dropped, nil on an unborn repo. The 50-element max makes the O(n) linear
// scan trivially fine.
//
// The match is a PURE EXACT, case-sensitive `==` — NO trimming, NO
// lowercasing, NO normalization. This is the trim-responsibility boundary
// (decisions.md §3): callers MUST pass already-trimmed strings (firstLine
// trims the generated subject; git.RecentSubjects trims each of its
// elements), so isDuplicate trims NOTHING. Fuzzy / Levenshtein / substring
// matching is explicitly DEFERRED to v1.1 (decisions.md §3 RESEARCH NOTE);
// near/paraphrased subjects therefore return false, giving a v1.1 fuzzy pass
// a regression baseline that proves the behavior CHANGED intentionally.
//
// A nil slice and an empty slice both yield false (the range is empty). A
// subject == "" yields false unless "" is literally an element (RecentSubjects
// drops empties, so in practice never).
func isDuplicate(subject string, subjects []string) bool {
	for _, s := range subjects {
		if s == subject {
			return true
		}
	}
	return false
}
