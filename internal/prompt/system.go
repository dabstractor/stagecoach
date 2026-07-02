// This file adds the system-prompt builder for the generate step
// (P1.M4.T1.S2): BuildSystemPrompt assembles the RAW-output contract, essence
// instruction, style examples with the explicit anti-reuse prohibition, the
// conditional multi-line rule, and the subject-length target — committed from
// the PRD §17.1/§17.2/Appendix A canonical strings (raw default per §17.4, NOT
// JSON). It uses a plain "package prompt" line because [examples.go] OWNS the
// package doc, mirroring how internal/git/log.go defers to git.go.
package prompt

import (
	"fmt"
	"strings"
)

const (
	// DefaultSubjectTargetChars is the canonical `target` (subject-line
	// character count) the generate layer (M6) passes to [BuildSystemPrompt]
	// before M5 config wiring (PRD §[generation] subject_target_chars = 50;
	// PRD FR13/FR14: the "~50 characters" subject target). It mirrors S1's
	// exported DefaultExampleCount = 20: a named default the caller references
	// by symbol rather than hardcoding.
	DefaultSubjectTargetChars = 50
)

// The canonical §17.1 (mature) / §17.2 (new-repo) / Appendix A strings,
// committed VERBATIM as named Go constants so the exact wording is reviewable
// and stable. Only the diff/examples/rejection-list (interpolated by the
// caller) and the target char count (the `target` parameter, via fmt.Sprintf
// per research D1) are NOT literals here. See PRD §17.1/§17.2/Appendix A.
const (
	// systemRole is the shared role line on BOTH paths (PRD §17.1, §17.2).
	systemRole = "You are a commit message generator."

	// matureRawContract is the FULL raw-output contract (§17.1): the "no
	// quoting" line AND the "If a body is warranted..." line are present. This
	// is the RAW default per §17.4 (NOT JSON) — reference_impl.md §4 D1.
	matureRawContract = "Output ONLY the commit message. No preamble, no markdown, no code fences,\nno quoting. If a body is warranted, use a blank line between subject and body."

	// newRepoRawContract is the SHORTER raw-output contract (§17.2): it is
	// MISSING the "no quoting" line AND the "If a body is warranted..." line.
	// Do NOT reuse matureRawContract for the new-repo path.
	newRepoRawContract = "Output ONLY the commit message. No preamble, no markdown, no code fences."

	// essenceInstruction is the shared essence-not-filenames instruction
	// (PRD §17.1, §17.2). The em-dash is UTF-8 U+2014, copied VERBATIM from PRD.
	essenceInstruction = "Focus on the ESSENCE of the change (the intent/purpose), not implementation\ndetails like filenames or function names."

	// examplesHeader opens the style-examples block (§17.1, mature path only).
	examplesHeader = "Match the tone and style of these recent commits from this repository:"

	// examplesFooter closes the examples block (§17.1). The ≤ is UTF-8 U+2264,
	// copied VERBATIM from PRD — do NOT replace with "<=".
	examplesFooter = "(up to 20, \u2264100 lines total)"

	// antiReuseClause is the verbatim CRITICAL anti-reuse prohibition (§17.1,
	// mature path only). The em-dash is UTF-8 U+2014, copied VERBATIM from PRD.
	antiReuseClause = "CRITICAL: You MUST NOT copy or reuse ANY phrasing from the examples above.\nThey show the STYLE to match — format, tone, length, conventions. Producing\nthe same text you have seen is STRICTLY FORBIDDEN. Your output must be\nentirely original wording describing THIS specific change. Reusing example\ntext is a critical failure."

	// multilineRuleMulti is the multi-line rule chosen when hasMultiline==true
	// (PRD §17.1: "If history has multi-line commits: ..."). Mature path only.
	multilineRuleMulti = "Only add a body (blank line + description) if the history shows multi-line commits AND these changes truly warrant detailed explanation. Otherwise, use a single-line subject only."

	// multilineRuleSingle is the multi-line rule chosen when hasMultiline==false
	// (PRD §17.1: "Else: ..."). Mature path only.
	multilineRuleSingle = "Only output a single-line subject (no body)."

	// exampleSeparator is the per-message separator wrapping each example in
	// the mature examples block (§17.1) — the same token splitExampleGroups
	// splits the raw log on in examples.go.
	exampleSeparator = "---"
)

// BuildSystemPrompt assembles the system-prompt string the generate step
// (P1.M6.T1.S1) hands to the agent, from the S1-provided style examples +
// multi-line signal plus two scalar settings. It is a PURE string builder — no
// git, no config, no error return (PRD FR13/FR14, plan_overview §M4: M4 is
// decoupled; builders take scalar settings; generate (M6) is the integrator
// that derives newRepo = (CommitCount<=1) — the SAME gate [FetchExamples] uses
// — and reads target from config subject_target_chars, default 50).
//
// It branches on newRepo FIRST (research D2):
//
//   - newRepo==true ⇒ PRD §17.2 conventional-commit fallback (FR14): the role
//     line, the SHORTER raw-output contract [newRepoRawContract] (no "no
//     quoting", no body line), the essence instruction, and
//     "Target ~<target> characters (~7 words). Format: type(scope): description".
//     It IGNORES examples, hasMultiline, and renders NO examples block and NO
//     anti-reuse clause (a repo with ≤1 commit has no history to learn from).
//
//   - newRepo==false ⇒ PRD §17.1 mature prompt (FR13): the role line, the FULL
//     raw-output contract [matureRawContract] (incl "no quoting" + the body
//     line), the essence instruction, the style examples each wrapped by the
//     "---" separator (newest-first, because [FetchExamples] returns
//     newest-first and this builder renders in slice order), the verbatim
//     CRITICAL anti-reuse prohibition [antiReuseClause], the conditional
//     multi-line rule ([multilineRuleMulti] when hasMultiline else
//     [multilineRuleSingle]), and "Target ~<target> characters for the subject
//     line.".
//
// The RAW-output contract is present on BOTH paths and the substring "json"
// (case-insensitive) appears NOWHERE — §17.4 / reference_impl.md §4 D1 (raw
// default, NOT JSON). JSON remains a per-provider manifest parse option (M2),
// never a prompt instruction.
//
// The `target` char count is interpolated via fmt.Sprintf (research D1), NOT
// hardcoded 50: with target=50 the rendered text is byte-identical to the PRD;
// with target=72 it renders "Target ~72 characters…". target<=0 is the caller's
// concern (config validation in M5); the builder just renders "~<target>".
//
// Each []string element is ONE whole commit message, blank-trimmed, lines
// joined by "\n", newest-first, ≤100 total lines — exactly what
// [FetchExamples] yields when newRepo==false (count>1). The builder does NOT
// validate inputs: when newRepo==false and examples is empty it still renders
// the mature prompt (header directly followed by the footer) — that is the
// caller's responsibility (FetchExamples yields examples only when count>1).
// Sections are joined by "\n\n"; no trailing newline is appended (the tests use
// strings.Contains, which is robust to the trailing-whitespace choice).
func BuildSystemPrompt(examples []string, hasMultiline bool, newRepo bool, target int) string {
	if newRepo {
		// PRD §17.2 conventional-commit fallback (FR14): role + SHORTER raw
		// contract + essence + the conventional-commit target line. IGNORES
		// examples, hasMultiline, and renders NO examples block / NO anti-reuse.
		return strings.Join([]string{
			systemRole,
			newRepoRawContract,
			essenceInstruction,
			fmt.Sprintf("Target ~%d characters (~7 words). Format: type(scope): description", target),
		}, "\n\n")
	}
	// PRD §17.1 mature prompt (FR13): role + FULL raw contract + essence +
	// examples block + anti-reuse + the selected multi-line rule + target line.
	sections := []string{
		systemRole,
		matureRawContract,
		essenceInstruction,
		renderExamples(examples),
		antiReuseClause,
	}
	if hasMultiline {
		sections = append(sections, multilineRuleMulti)
	} else {
		sections = append(sections, multilineRuleSingle)
	}
	sections = append(sections, fmt.Sprintf("Target ~%d characters for the subject line.", target))
	return strings.Join(sections, "\n\n")
}

// renderExamples builds the §17.1 examples block: the header, then each
// example in slice order wrapped by the "---" separator, then the footer. With
// an empty examples slice it renders the header directly followed by the footer
// — the caller's concern (FetchExamples yields examples only when count>1, and
// the mature path is only taken when count>1). Slice order is render order:
// [FetchExamples] returns newest-first, so the rendered examples are
// newest-first too (research D4).
func renderExamples(examples []string) string {
	var b strings.Builder
	b.WriteString(examplesHeader)
	b.WriteByte('\n')
	for _, ex := range examples {
		b.WriteString(exampleSeparator)
		b.WriteByte('\n')
		b.WriteString(ex)
		b.WriteByte('\n')
	}
	b.WriteString(examplesFooter)
	return b.String()
}
