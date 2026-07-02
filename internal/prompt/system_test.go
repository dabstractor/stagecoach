package prompt

// White-box test for [BuildSystemPrompt] (P1.M4.T1.S2), matching the
// internal/ui, internal/provider, internal/git, and S1's examples_test.go
// house convention: the _test.go file is `package prompt` (NOT
// `package prompt_test`) so it can reference the unexported canonical-string
// constants (systemRole, matureRawContract, newRepoRawContract, …) directly.
// It exercises a PURE string-building function, so it needs stdlib `testing`
// + `strings` ONLY — NO internal/git, NO os/exec, NO testify (no real-git
// integration test is needed).

import (
	"strings"
	"testing"
)

// contains reports whether s has substr. A tiny helper to keep the contract
// MOCKING assertions readable (no testify; the project uses stdlib testing
// only). One behavior per Test*: each Test* below asserts one aspect of the
// built prompt via strings.Contains (robust to the exact join-whitespace
// choice, per the contract MOCKING).
func contains(s, substr string) bool { return strings.Contains(s, substr) }

// mustContain fails the test if p does NOT contain every want substring.
func mustContain(t *testing.T, p string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !contains(p, w) {
			t.Errorf("prompt missing %q\n--- prompt ---\n%s", w, p)
		}
	}
}

// mustNotContain fails the test if p DOES contain any unwanted substring.
func mustNotContain(t *testing.T, p string, unwants ...string) {
	t.Helper()
	for _, u := range unwants {
		if contains(p, u) {
			t.Errorf("prompt unexpectedly contains %q\n--- prompt ---\n%s", u, p)
		}
	}
}

// assertNoJSON fails if the lowercased prompt contains "json" anywhere — the
// §17.4 / reference_impl.md §4 D1 invariant: the RAW-output contract default,
// NOT JSON. JSON is a per-provider manifest parse option (M2), never a prompt
// instruction.
func assertNoJSON(t *testing.T, p string) {
	t.Helper()
	if contains(strings.ToLower(p), "json") {
		t.Errorf("prompt contains the forbidden substring \"json\" (case-insensitive); §17.4/raw default, NOT JSON\n--- prompt ---\n%s", p)
	}
}

// TestBuildSystemPrompt_NewRepo covers the PRD §17.2 conventional-commit
// fallback (FR14): newRepo==true produces the role line, the SHORTER raw
// contract (no "no quoting", no body line), the essence instruction, and the
// "type(scope): description" target line. It IGNORES examples/hasMultiline and
// renders NO examples block and NO anti-reuse clause.
func TestBuildSystemPrompt_NewRepo(t *testing.T) {
	p := BuildSystemPrompt(nil, false, true, 50)

	mustContain(t, p,
		systemRole,
		newRepoRawContract,
		essenceInstruction,
		"Target ~50 characters (~7 words). Format: type(scope): description",
		"type(scope): description",
	)

	// No examples block, no anti-reuse clause, and NOT the mature "no quoting"
	// line (the new-repo raw contract is deliberately SHORTER).
	mustNotContain(t, p,
		examplesHeader,
		"You MUST NOT copy or reuse",
		"no quoting",
		"If a body is warranted",
	)
	assertNoJSON(t, p)
}

// TestBuildSystemPrompt_Mature_SingleLine covers the PRD §17.1 mature prompt
// (FR13) with hasMultiline==false: the role line, the FULL raw contract (incl
// "no quoting" + the body line), the essence instruction, the examples block
// (each example wrapped by "---" with the "(up to 20, ≤100 lines total)"
// footer), the CRITICAL anti-reuse clause, the SINGLE-line variant, and the
// subject-line target.
func TestBuildSystemPrompt_Mature_SingleLine(t *testing.T) {
	p := BuildSystemPrompt([]string{"feat: a", "fix: b"}, false, false, 50)

	mustContain(t, p,
		systemRole,
		matureRawContract, // full contract incl "no quoting" + body line
		essenceInstruction,
		examplesHeader,
		"feat: a",                    // first example, verbatim
		"fix: b",                     // second example, verbatim
		exampleSeparator,             // "---"
		examplesFooter,               // "(up to 20, ≤100 lines total)"
		"You MUST NOT copy or reuse", // anti-reuse clause fragment
		multilineRuleSingle,          // the SINGLE-line variant
		"Target ~50 characters for the subject line.",
	)
	// The FULL mature raw contract is present (both the "no quoting" line and
	// the "If a body is warranted" line) — NOT the shorter new-repo contract.
	mustContain(t, p, "no quoting", "If a body is warranted, use a blank line between subject and body.")

	mustNotContain(t, p,
		multilineRuleMulti, // wrong multi-line variant for hasMultiline==false
	)
	assertNoJSON(t, p)
}

// TestBuildSystemPrompt_Mature_MultilineVariant is ★ THE CORRECT multi-line
// VARIANT ★ check (PRD §17.1): with hasMultiline==true the mature prompt
// carries multilineRuleMulti ("Only add a body ...") and NOT
// multilineRuleSingle. The multi-line rule is selected by hasMultiline (two
// variants), only on the MATURE path — never by newRepo.
func TestBuildSystemPrompt_Mature_MultilineVariant(t *testing.T) {
	p := BuildSystemPrompt([]string{"feat: a\nbody"}, true, false, 50)

	mustContain(t, p, multilineRuleMulti)
	mustNotContain(t, p, multilineRuleSingle)
	assertNoJSON(t, p)
}

// TestBuildSystemPrompt_TargetInterpolation proves the `target` parameter is
// interpolated via fmt.Sprintf (research D1), NOT hardcoded 50: with
// target=72 BOTH the new-repo and the mature paths render
// "Target ~72 characters…" and neither renders the default "Target ~50
// characters…".
func TestBuildSystemPrompt_TargetInterpolation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		newRepo bool
	}{
		{"newRepo", true},
		{"mature", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := BuildSystemPrompt([]string{"feat: a"}, true, tc.newRepo, 72)
			if !contains(p, "Target ~72 characters") {
				t.Errorf("target=72: prompt missing %q\n--- prompt ---\n%s", "Target ~72 characters", p)
			}
			if contains(p, "Target ~50 characters") {
				t.Errorf("target=72: prompt unexpectedly renders the default %q (target must be interpolated, not hardcoded)\n--- prompt ---\n%s", "Target ~50 characters", p)
			}
		})
	}
}

// TestBuildSystemPrompt_NoJSONAnywhere asserts the §17.4 / reference_impl.md
// §4 D1 invariant on BOTH paths: the RAW-output contract default, NOT JSON —
// the substring "json" (case-insensitive) appears NOWHERE in either rendered
// prompt. JSON remains a per-provider manifest parse option (M2), never a
// prompt instruction.
func TestBuildSystemPrompt_NoJSONAnywhere(t *testing.T) {
	newRepoPrompt := BuildSystemPrompt(nil, false, true, 50)
	maturePrompt := BuildSystemPrompt([]string{"feat: a"}, false, false, 50)
	assertNoJSON(t, newRepoPrompt)
	assertNoJSON(t, maturePrompt)
}

// TestBuildSystemPrompt_RawContractPresent asserts the RAW-output contract
// ("Output ONLY the commit message. No preamble, no markdown, no code fences")
// — the shared prefix of both raw contracts — is present on BOTH paths (the
// new-repo contract is that exact string; the mature contract is its superset).
func TestBuildSystemPrompt_RawContractPresent(t *testing.T) {
	const shared = "Output ONLY the commit message. No preamble, no markdown, no code fences"
	newRepoPrompt := BuildSystemPrompt(nil, false, true, 50)
	maturePrompt := BuildSystemPrompt([]string{"feat: a"}, false, false, 50)
	if !contains(newRepoPrompt, shared) {
		t.Errorf("newRepo prompt missing raw contract %q\n--- prompt ---\n%s", shared, newRepoPrompt)
	}
	if !contains(maturePrompt, shared) {
		t.Errorf("mature prompt missing raw contract %q\n--- prompt ---\n%s", shared, maturePrompt)
	}
}

// TestBuildSystemPrompt_ExamplesNewestFirstRendered asserts the examples block
// renders the header, then each example in SLICE order wrapped by a "---"
// separator, then the footer. [FetchExamples] returns newest-first, so slice
// order is render order: "newest msg" precedes "older msg", each is wrapped by
// a "---" line (the substring "---\n<msg>" appears), the header precedes the
// first example, and the footer follows the last (research D4).
func TestBuildSystemPrompt_ExamplesNewestFirstRendered(t *testing.T) {
	p := BuildSystemPrompt([]string{"newest msg", "older msg"}, false, false, 50)

	// Each example is wrapped by a "---" separator line (the line before the
	// message text is exactly "---").
	mustContain(t, p,
		examplesHeader,
		"---\nnewest msg",
		"---\nolder msg",
		examplesFooter,
	)

	// Order: the header precedes the first example, which precedes the second
	// example, which precedes the footer (slice order = render order).
	headerIdx := strings.Index(p, examplesHeader)
	firstIdx := strings.Index(p, "newest msg")
	secondIdx := strings.Index(p, "older msg")
	footerIdx := strings.Index(p, examplesFooter)
	if headerIdx < 0 || firstIdx < 0 || secondIdx < 0 || footerIdx < 0 {
		t.Fatalf("could not locate all anchors; header=%d first=%d second=%d footer=%d\n--- prompt ---\n%s",
			headerIdx, firstIdx, secondIdx, footerIdx, p)
	}
	if !(headerIdx < firstIdx && firstIdx < secondIdx && secondIdx < footerIdx) {
		t.Errorf("examples not in slice order; want header(%d) < newest(%d) < older(%d) < footer(%d)\n--- prompt ---\n%s",
			headerIdx, firstIdx, secondIdx, footerIdx, p)
	}
}
