package prompt

import "strings"

// Format-mode scaffolds + locale line (PRD §9.19 FR-F1..F6 / §17.8). A non-"auto" cfg.Format REPLACES
// the style-examples block (the "Match the tone…" intro + the example loop + antiReuseProhibition) with
// an explicit per-mode contract; a non-empty cfg.Locale APPENDS one line in every mode (auto included) and
// both repo-age variants. See system.go (BuildSystemPrompt/BuildFallbackPrompt) and planner.go
// (BuildPlannerSystemPrompt) for the callers that dispatch here.

// conventionalScaffold is the §17.8 "conventional" scaffold body (FR-F2): the Conventional Commits
// format contract and type vocabulary. NO trailing newline (package convention — the caller owns
// inter-block newline placement).
const conventionalScaffold = "Format: type(scope): description. type ∈ feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert; scope optional."

// gitmojiScaffoldInstruction is the §17.8 "gitmoji" scaffold instruction (FR-F3), followed by a blank
// line and prompt.RenderGitmojiTable() (S2, compiled-in — no network fetch, ever). NO trailing newline.
const gitmojiScaffoldInstruction = "Begin the subject with exactly ONE emoji from the gitmoji list below (the emoji character itself, not a :shortcode:), followed by a space and the description."

// bodyForceDirective is the §17.8 / FR-F9 unconditional body directive that REPLACES the conditional
// multi-line rule (multilineRuleAllow/multilineRuleSingle) when a +body format suffix is in effect —
// forcing a subject-plus-body message regardless of repo-history shape. NO trailing newline (package
// convention — the caller owns inter-block newline placement). VERBATIM from PRD §17.8 Design Decision 4.
const bodyForceDirective = "ALWAYS follow the subject with a body — a blank line, then a wrapped (~72-column) explanation of what this change does and why. Use a short bullet list only when the change has several distinct parts. The subject above still follows its format contract."

// formatScaffoldBody returns the §17.8 mode-specific contract block that REPLACES the style-examples
// block. Empty string for "auto" (the caller keeps the examples block instead) and "plain" (FR-F4: no
// format contract, no examples). Any unknown format (should be unreachable — S1 validates cfg.Format)
// also returns "" — defensive, auto-like; never panics.
func formatScaffoldBody(format string) string {
	switch format {
	case "conventional":
		return conventionalScaffold
	case "gitmoji":
		return gitmojiScaffoldInstruction + "\n\n" + RenderGitmojiTable()
	default: // "auto", "plain", or an unknown/unreachable value
		return ""
	}
}

// withLocale appends the FR-F6 locale instruction as ONE line — "Write the commit message in <lang>." —
// or returns s unchanged when locale is empty. locale is passed VERBATIM (no validation, no BCP-47
// parsing, no i18n table — FR-F6). Trims any trailing newline from s first so every caller (auto,
// non-auto message scaffold, planner) shares one single-newline separator rule regardless of whether s
// itself ends in "\n".
func withLocale(s, locale string) string {
	if locale == "" {
		return s
	}
	return strings.TrimRight(s, "\n") + "\nWrite the commit message in " + locale + "."
}

// splitFormat parses the FR-F1 <base>[+body] format grammar: returns (base, false) when format has no
// "+body" suffix, or (base, true) when format == base + "+body" (case-sensitive on the suffix — "Conventional+Body"
// is NOT a +body form; S3's validateFormat rejects it). It does NOT validate base — the caller (S3
// validateFormat) owns base-set validation. Pure (no I/O). splitFormat("+body") returns ("", true): the empty
// base is likewise left for S3 to reject; splitFormat is a pure grammar split, never an error source.
func splitFormat(format string) (base string, forceBody bool) {
	if strings.HasSuffix(format, "+body") {
		return strings.TrimSuffix(format, "+body"), true
	}
	return format, false
}

// FormatForcesBody reports whether format carries the FR-F9 +body suffix. Exported so cross-package
// callers (e.g. decompose.runSingleShortcut) can branch on it without re-implementing the grammar
// split or importing the unexported splitFormat. Pure; case-sensitive on the suffix (mirrors
// splitFormat / config.validateFormat). It does NOT validate the base — the caller is expected to
// pass an already-validated cfg.Format (Load rejects unknown bases + the suffix).
func FormatForcesBody(format string) bool {
	_, forceBody := splitFormat(format)
	return forceBody
}

// buildFormatSystemPrompt assembles the non-auto message system prompt (§17.8 FR-F2/F3/F4/F9): the shared
// preamble (role + output rules + essence — NO "Match the tone…" line), the mode scaffold body (empty
// for "plain"), and EITHER the retained multi-line rule (selected by hasMultiline — FR12 detection still
// runs in every non-+body mode) OR the unconditional bodyForceDirective when the format carries a +body
// suffix (FR-F9 — forceBody REPLACES the conditional multi-line rule; hasMultiline is ignored under +body).
// Locale is applied by the caller via withLocale, not here, so this helper stays a pure function of
// (format, hasMultiline, subjectTarget). The +body suffix is parsed internally via splitFormat; for any
// non-+body format the output is byte-identical to the pre-FR-F9 behavior (splitFormat returns
// (format,false) ⇒ same scaffold + same multilineRule selection).
func buildFormatSystemPrompt(format string, hasMultiline bool, subjectTarget int) string {
	base, forceBody := splitFormat(format)
	var b strings.Builder
	b.WriteString(promptPreamble)
	b.WriteString("\n\n")
	if body := formatScaffoldBody(base); body != "" { // base, NOT raw format (formatScaffoldBody switches on the exact base)
		b.WriteString(body)
		b.WriteString("\n\n")
	}
	if forceBody {
		b.WriteString(bodyForceDirective) // +body: unconditional body directive REPLACES the conditional multi-line rule
	} else {
		if hasMultiline {
			b.WriteString(multilineRuleAllow)
		} else {
			b.WriteString(multilineRuleSingle)
		}
	}
	b.WriteByte('\n')
	b.WriteString(subjectTargetLine(subjectTarget))
	return b.String()
}
