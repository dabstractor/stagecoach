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

// buildFormatSystemPrompt assembles the non-auto message system prompt (§17.8 FR-F2/F3/F4): the shared
// preamble (role + output rules + essence — NO "Match the tone…" line), the mode scaffold body (empty
// for "plain"), the retained multi-line rule (selected by hasMultiline — FR12 detection still runs in
// every non-auto mode), and the subject-target line. Locale is applied by the caller via withLocale, not
// here, so this helper stays a pure function of (format, hasMultiline, subjectTarget).
func buildFormatSystemPrompt(format string, hasMultiline bool, subjectTarget int) string {
	var b strings.Builder
	b.WriteString(promptPreamble)
	b.WriteString("\n\n")
	if body := formatScaffoldBody(format); body != "" {
		b.WriteString(body)
		b.WriteString("\n\n")
	}
	if hasMultiline {
		b.WriteString(multilineRuleAllow)
	} else {
		b.WriteString(multilineRuleSingle)
	}
	b.WriteByte('\n')
	b.WriteString(subjectTargetLine(subjectTarget))
	return b.String()
}
