package prompt

import (
	"strings"
	"testing"
)

// TestBuildSystemPrompt_CanonicalExact asserts the FULL assembled string for a known input, pinning the
// PRD §17.1 blank-line topology byte-for-byte (including the em-dash, the raw-output contract, the
// "---"-before-each-example format, the excluded annotation, and the rule/target placement). This is the
// strongest guard against accidental newline/dash drift. Independently derived from PRD §17.1 (not from
// the implementation) so a match is meaningful.
func TestBuildSystemPrompt_CanonicalExact(t *testing.T) {
	examples := []string{
		"feat: add foo",
		"fix: handle nil deref\n\nThe parser panicked on an unresolved manifest.",
	}
	const subjectTarget = 50
	got := BuildSystemPrompt(examples, true, subjectTarget, "auto", "")

	const want = "You are a commit message generator.\n" +
		"\n" +
		"Output ONLY the commit message. No preamble, no markdown, no code fences,\n" +
		"no quoting. If a body is warranted, use a blank line between subject and body.\n" +
		"\n" +
		"Focus on the ESSENCE of the change (the intent/purpose), not implementation\n" +
		"details like filenames or function names.\n" +
		"\n" +
		"Match the tone and style of these recent commits from this repository:\n" +
		"---\n" +
		"feat: add foo\n" +
		"---\n" +
		"fix: handle nil deref\n" +
		"\n" +
		"The parser panicked on an unresolved manifest.\n" +
		"\n" +
		"CRITICAL: You MUST NOT copy or reuse ANY phrasing from the examples above.\n" +
		"They show the STYLE to match — format, tone, length, conventions. Producing\n" +
		"the same text you have seen is STRICTLY FORBIDDEN. Your output must be\n" +
		"entirely original wording describing THIS specific change. Reusing example\n" +
		"text is a critical failure.\n" +
		"\n" +
		"Only add a body (blank line + description) if the history shows multi-line commits AND these changes truly warrant detailed explanation. Otherwise, use a single-line subject only.\n" +
		"Target ~50 characters for the subject line."

	if got != want {
		// Diff-friendly failure: show where the strings diverge.
		t.Errorf("BuildSystemPrompt mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestBuildSystemPrompt_Properties is a table of structural invariants on the assembled prompt, each
// guarding a specific design decision. These complement the exact-match test by pinning the properties
// that matter most (em-dash, raw-not-JSON contract, "---" count, excluded annotation, rule selection,
// subjectTarget formatting, example ordering).
func TestBuildSystemPrompt_Properties(t *testing.T) {
	singleLine := []string{"feat: one", "chore: two"}
	multiLine := []string{"feat: one\n\nBody one.", "chore: two"}
	cases := []struct {
		name          string
		examples      []string
		hasMultiline  bool
		subjectTarget int
		check         func(t *testing.T, p string)
	}{
		{
			name: "em-dash present (NOT ascii hyphen)", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, "match — format") {
					t.Errorf("anti-reuse block missing em-dash (U+2014); got substring near 'match': %q", near(p, "match"))
				}
				if strings.Contains(p, "match - format") { // ASCII hyphen variant
					t.Errorf("anti-reuse block uses ASCII hyphen '-', expected em-dash '—'")
				}
			},
		},
		{
			name: "raw-output contract present", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, "Output ONLY the commit message. No preamble, no markdown, no code fences") {
					t.Error("raw-output contract missing")
				}
			},
		},
		{
			name: "JSON contract ABSENT (ported PRD not commit-pi)", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if strings.Contains(p, "Return valid JSON") {
					t.Error("commit-pi JSON contract leaked into the PRD prompt")
				}
				if strings.Contains(p, "no double quotes") {
					t.Error("commit-pi 'no double quotes' constraint leaked in")
				}
			},
		},
		{
			name: "(up to 20) annotation ABSENT", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if strings.Contains(p, "(up to 20") || strings.Contains(p, "≤100 lines total") {
					t.Error("structural annotation '(up to 20, ≤100 lines total)' must NOT be in the runtime prompt")
				}
			},
		},
		{
			name: "--- count == len(examples)", examples: []string{"a", "b", "c"}, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if got := strings.Count(p, "---"); got != 3 {
					t.Errorf("--- count = %d, want 3 (one before each example)", got)
				}
			},
		},
		{
			name: "examples appear in order", examples: []string{"ALPHA", "BETA", "GAMMA"}, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				i := strings.Index(p, "ALPHA")
				j := strings.Index(p, "BETA")
				k := strings.Index(p, "GAMMA")
				if i < 0 || j < 0 || k < 0 || !(i < j && j < k) {
					t.Errorf("examples out of order: ALPHA@%d BETA@%d GAMMA@%d", i, j, k)
				}
			},
		},
		{
			name: "hasMultiline=false → single-line rule, allow rule ABSENT", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, multilineRuleSingle) {
					t.Error("expected the single-line rule")
				}
				if strings.Contains(p, multilineRuleAllow) {
					t.Error("the allow-body rule must be ABSENT when hasMultiline=false")
				}
			},
		},
		{
			name: "hasMultiline=true → allow rule, single-line rule ABSENT", examples: multiLine, hasMultiline: true, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, multilineRuleAllow) {
					t.Error("expected the allow-body rule")
				}
				if strings.Contains(p, multilineRuleSingle) {
					t.Error("the single-line rule must be ABSENT when hasMultiline=true")
				}
			},
		},
		{
			name: "subjectTarget interpolated (72)", examples: singleLine, hasMultiline: false, subjectTarget: 72,
			check: func(t *testing.T, p string) {
				if !strings.Contains(p, "Target ~72 characters for the subject line.") {
					t.Error("subjectTarget=72 not interpolated")
				}
				if strings.Contains(p, "~50 characters") {
					t.Error("subjectTarget leaked a hardcoded 50")
				}
			},
		},
		{
			name: "no blank line between rule and target", examples: singleLine, hasMultiline: false, subjectTarget: 50,
			check: func(t *testing.T, p string) {
				want := multilineRuleSingle + "\n" + "Target ~50 characters for the subject line."
				if !strings.Contains(p, want) {
					t.Error("expected the rule immediately followed by the target line (no blank line between)")
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, BuildSystemPrompt(tc.examples, tc.hasMultiline, tc.subjectTarget, "auto", ""))
		})
	}
}

// TestBuildSystemPrompt_EmptyExamples verifies the defensive path: nil/empty examples must not panic
// and must omit all "---" lines while keeping the header, anti-reuse, rule, and target.
func TestBuildSystemPrompt_EmptyExamples(t *testing.T) {
	for _, ex := range [][]string{nil, {}} {
		p := BuildSystemPrompt(ex, false, 50, "auto", "") // must not panic
		if strings.Contains(p, "---") {
			t.Errorf("empty examples must emit no '---' lines; got %q", p)
		}
		for _, must := range []string{
			"You are a commit message generator.",
			antiReuseProhibition,
			multilineRuleSingle,
			"Target ~50 characters for the subject line.",
		} {
			if !strings.Contains(p, must) {
				t.Errorf("empty-examples prompt missing required block %q", must)
			}
		}
	}
}

// TestDetectMultiline is the table for the FR12 detection (faithful awk port: >1 non-blank line ⇒ true).
func TestDetectMultiline(t *testing.T) {
	cases := []struct {
		name     string
		examples []string
		want     bool
	}{
		{"nil → false", nil, false},
		{"empty → false", []string{}, false},
		{"all single-line → false", []string{"feat: a", "fix: b"}, false},
		{"one single-line → false", []string{"feat: a"}, false},
		{"one multi-line (body) → true", []string{"feat: a\n\nBody text."}, true},
		{"mixed, one multi-line → true", []string{"feat: a", "fix: b\n\nBody."}, true},
		{"whitespace-only body line counts (awk-faithful) → true", []string{"feat: a\n   \nbody"}, true},
		{"subject + trailing blanks trimmed upstream ⇒ single-line here → false", []string{"subject"}, false},
		{"only blanks → 0 non-blank lines → false", []string{"\n\n"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectMultiline(tc.examples); got != tc.want {
				t.Errorf("DetectMultiline(%v) = %v, want %v", tc.examples, got, tc.want)
			}
		})
	}
}

// TestCountNonBlankLines targets the helper directly (the awk's per-message `lines` counter).
func TestCountNonBlankLines(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"one", 1},
		{"a\nb", 2},
		{"a\n\nb", 2},    // internal blank not counted
		{"a\n   \nb", 2}, // whitespace-only not blank-counted as content but still non-blank line
		{"\n\n", 0},
		{"\n\nfoo\n\n", 1},
	}
	for _, c := range cases {
		if got := countNonBlankLines(c.in); got != c.want {
			t.Errorf("countNonBlankLines(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestBuildFallbackPrompt_CanonicalExact asserts the FULL assembled string for subjectTarget=50, pinning
// PRD §17.2 byte-for-byte (role + blank + short output contract + blank + 2-line essence + blank + the
// type(scope) target/format line). Independently derived from PRD §17.2 (not from the implementation).
func TestBuildFallbackPrompt_CanonicalExact(t *testing.T) {
	got := BuildFallbackPrompt(50, "auto", "")

	const want = "You are a commit message generator.\n" +
		"\n" +
		"Output ONLY the commit message. No preamble, no markdown, no code fences.\n" +
		"\n" +
		"Focus on the ESSENCE of the change (the intent/purpose), not implementation\n" +
		"details like filenames or function names.\n" +
		"\n" +
		"Target ~50 characters (~7 words). Format: type(scope): description"

	if got != want {
		t.Errorf("BuildFallbackPrompt(50) mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

// TestBuildFallbackPrompt_Properties is a table of structural invariants. It guards (a) every §17.2
// block is present, (b) the §17.2 ADDITIONS are present, and (c) — the anti-copy-paste guards — every
// §17.1 MATURE-prompt element is ABSENT (the #1 implementation risk is copy-pasting S1's constants).
func TestBuildFallbackPrompt_Properties(t *testing.T) {
	p := BuildFallbackPrompt(50, "auto", "")
	cases := []struct {
		name      string
		needle    string
		mustExist bool
	}{
		// §17.2 blocks present.
		{"role present", "You are a commit message generator.", true},
		{"short output contract present", "Output ONLY the commit message. No preamble, no markdown, no code fences.", true},
		{"essence line 1 present", "Focus on the ESSENCE of the change (the intent/purpose), not implementation", true},
		{"essence line 2 present", "details like filenames or function names.", true},
		// §17.2 ADDITIONS present (vs §17.1).
		{"conventional-commit format present", "Format: type(scope): description", true},
		{"~7 words gloss present", "(~7 words)", true},
		// §17.1 MATURE elements ABSENT (anti-copy-paste guards).
		{"§17.1 'no quoting' clause ABSENT", "no quoting", false},
		{"§17.1 body clause ABSENT", "If a body is warranted", false},
		{"§17.1 examples intro ABSENT", "Match the tone and style", false},
		{"§17.1 '---' markers ABSENT", "---", false},
		{"§17.1 anti-reuse block ABSENT", "CRITICAL: You MUST NOT copy", false},
		{"§17.1 'for the subject line' wording ABSENT", "for the subject line", false},
		{"§17.1 multi-line rule ABSENT", "multi-line", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			has := strings.Contains(p, tc.needle)
			if tc.mustExist && !has {
				t.Errorf("expected %q in BuildFallbackPrompt(50); not found", tc.needle)
			}
			if !tc.mustExist && has {
				t.Errorf("BuildFallbackPrompt(50) must NOT contain §17.1 element %q (copy-paste leak)", tc.needle)
			}
		})
	}

	// Blank-line topology: body + exactly ONE blank line + target line; no trailing newline.
	if !strings.HasSuffix(p, "Format: type(scope): description") {
		t.Errorf("prompt must end with the format line (no trailing newline); got suffix %q", suffix(p, 40))
	}
	if strings.HasSuffix(p, "\n") {
		t.Error("prompt must NOT end with a trailing newline")
	}
	if n := strings.Count(p, "\n\n"); n != 3 {
		t.Errorf("expected exactly 3 blank-line separators (\\n\\n) in §17.2; got %d", n)
	}
}

// TestBuildFallbackPrompt_SubjectTargetInterpolated pins §2: a non-default subjectTarget changes ONLY
// the char count; "(~7 words)" survives verbatim; no hardcoded 50 leaks.
func TestBuildFallbackPrompt_SubjectTargetInterpolated(t *testing.T) {
	p := BuildFallbackPrompt(72, "auto", "")
	if !strings.Contains(p, "Target ~72 characters (~7 words). Format: type(scope): description") {
		t.Errorf("subjectTarget=72 not interpolated as expected; got %q", suffix(p, 80))
	}
	if strings.Contains(p, "~50 characters") {
		t.Error("subjectTarget=72 must NOT leak a hardcoded '~50 characters'")
	}
	if !strings.Contains(p, "(~7 words)") {
		t.Error("the fixed '(~7 words)' gloss must survive a non-default subjectTarget (§2)")
	}
}

// TestBuildSystemPrompt_FormatModes_CanonicalExact pins the exact §17.8 assembly for each non-auto mode
// (mature-repo builder), with and without a locale, proving the scaffold REPLACES the style-examples
// block + anti-reuse warning while retaining the preamble, multi-line rule, and subject-target line
// (FR-F2/F3/F4), and that locale appends exactly one line (FR-F6).
func TestBuildSystemPrompt_FormatModes_CanonicalExact(t *testing.T) {
	examples := []string{"feat: add foo", "fix: handle nil deref"} // IGNORED in non-auto modes (FR-F1)

	cases := []struct {
		name   string
		format string
		locale string
		want   string
	}{
		{
			name: "conventional, no locale", format: "conventional", locale: "",
			want: promptPreamble + "\n\n" + conventionalScaffold + "\n\n" + multilineRuleSingle + "\n" +
				"Target ~50 characters for the subject line.",
		},
		{
			name: "conventional, locale French", format: "conventional", locale: "French",
			want: promptPreamble + "\n\n" + conventionalScaffold + "\n\n" + multilineRuleSingle + "\n" +
				"Target ~50 characters for the subject line.\nWrite the commit message in French.",
		},
		{
			name: "gitmoji, no locale", format: "gitmoji", locale: "",
			want: promptPreamble + "\n\n" + gitmojiScaffoldInstruction + "\n\n" + RenderGitmojiTable() + "\n\n" +
				multilineRuleSingle + "\n" + "Target ~50 characters for the subject line.",
		},
		{
			name: "gitmoji, locale French", format: "gitmoji", locale: "French",
			want: promptPreamble + "\n\n" + gitmojiScaffoldInstruction + "\n\n" + RenderGitmojiTable() + "\n\n" +
				multilineRuleSingle + "\n" + "Target ~50 characters for the subject line.\nWrite the commit message in French.",
		},
		{
			name: "plain, no locale", format: "plain", locale: "",
			want: promptPreamble + "\n\n" + multilineRuleSingle + "\n" + "Target ~50 characters for the subject line.",
		},
		{
			name: "plain, locale French", format: "plain", locale: "French",
			want: promptPreamble + "\n\n" + multilineRuleSingle + "\n" +
				"Target ~50 characters for the subject line.\nWrite the commit message in French.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildSystemPrompt(examples, false, 50, tc.format, tc.locale)
			if got != tc.want {
				t.Errorf("BuildSystemPrompt(%q, %q) mismatch:\n--- got ---\n%q\n--- want ---\n%q", tc.format, tc.locale, got, tc.want)
			}
		})
	}
}

// TestBuildSystemPrompt_FormatModes_Properties asserts the structural invariants shared by every non-auto
// mode: the style-examples block, the "Match the tone…" intro, and the anti-reuse block are ABSENT; the
// multi-line rule + subject-target line are retained; and each mode's scaffold marker is present.
func TestBuildSystemPrompt_FormatModes_Properties(t *testing.T) {
	examples := []string{"feat: a", "fix: b"}
	for _, format := range []string{"conventional", "gitmoji", "plain"} {
		for _, locale := range []string{"", "French"} {
			t.Run(format+"/locale="+locale, func(t *testing.T) {
				p := BuildSystemPrompt(examples, true, 50, format, locale)

				if strings.Contains(p, "Match the tone and style") {
					t.Error("style-examples intro must be ABSENT in non-auto modes")
				}
				if strings.Contains(p, "CRITICAL: You MUST NOT copy") {
					t.Error("anti-reuse block must be ABSENT in non-auto modes")
				}
				if strings.Contains(p, "---") {
					t.Error("no '---' example markers must appear in non-auto modes")
				}
				if strings.Contains(p, "feat: a") || strings.Contains(p, "fix: b") {
					t.Error("history examples must NOT be embedded in non-auto modes (FR-F1)")
				}
				if !strings.Contains(p, multilineRuleAllow) {
					t.Error("multi-line rule must be retained (FR12 detection still runs)")
				}
				if !strings.Contains(p, "Target ~50 characters for the subject line.") {
					t.Error("subject-target line must be retained")
				}

				switch format {
				case "conventional":
					if !strings.Contains(p, "type(scope): description") {
						t.Error("conventional scaffold missing format contract")
					}
					for _, ty := range []string{"feat", "fix", "docs", "style", "refactor", "perf", "test", "build", "ci", "chore", "revert"} {
						if !strings.Contains(p, ty) {
							t.Errorf("conventional scaffold missing type %q", ty)
						}
					}
				case "gitmoji":
					if !strings.Contains(p, "Begin the subject with exactly ONE emoji") {
						t.Error("gitmoji scaffold missing instruction")
					}
					if !strings.Contains(p, "🎨 - ") {
						t.Error("gitmoji scaffold missing a RenderGitmojiTable() row")
					}
				case "plain":
					if strings.Contains(p, "type(scope): description") {
						t.Error("plain mode must have no format contract")
					}
					if strings.Contains(p, "Begin the subject with exactly ONE emoji") {
						t.Error("plain mode must have no gitmoji instruction")
					}
				}

				if locale == "French" {
					if !strings.HasSuffix(p, "\nWrite the commit message in French.") {
						t.Errorf("locale line missing/misplaced; suffix: %q", suffix(p, 60))
					}
				} else if strings.Contains(p, "Write the commit message in") {
					t.Error("empty locale must NOT produce a locale line")
				}
			})
		}
	}
}

// TestBuildFallbackPrompt_FormatModes_CanonicalExact mirrors the mature-repo scaffold test for the
// new-repo (fallback) builder: hasMultiline is implicitly false (no history), and the same scaffold +
// locale rules apply (FR-F2/F3/F4/F6).
func TestBuildFallbackPrompt_FormatModes_CanonicalExact(t *testing.T) {
	cases := []struct {
		name   string
		format string
		locale string
		want   string
	}{
		{
			name: "conventional, no locale", format: "conventional", locale: "",
			want: promptPreamble + "\n\n" + conventionalScaffold + "\n\n" + multilineRuleSingle + "\n" +
				"Target ~50 characters for the subject line.",
		},
		{
			name: "gitmoji, locale ja", format: "gitmoji", locale: "ja",
			want: promptPreamble + "\n\n" + gitmojiScaffoldInstruction + "\n\n" + RenderGitmojiTable() + "\n\n" +
				multilineRuleSingle + "\n" + "Target ~50 characters for the subject line.\nWrite the commit message in ja.",
		},
		{
			name: "plain, no locale", format: "plain", locale: "",
			want: promptPreamble + "\n\n" + multilineRuleSingle + "\n" + "Target ~50 characters for the subject line.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildFallbackPrompt(50, tc.format, tc.locale)
			if got != tc.want {
				t.Errorf("BuildFallbackPrompt(%q, %q) mismatch:\n--- got ---\n%q\n--- want ---\n%q", tc.format, tc.locale, got, tc.want)
			}
		})
	}
}

// TestBuildSystemPrompt_UnknownFormat_DefaultsToAutoLike verifies the defensive path: an unreachable
// unknown format value must not panic and must behave like "plain" (no scaffold body) rather than
// crashing — the builder never re-validates cfg.Format (S1 owns validation).
func TestBuildSystemPrompt_UnknownFormat_DefaultsToAutoLike(t *testing.T) {
	p := BuildSystemPrompt([]string{"feat: a"}, false, 50, "bogus-mode", "")
	if strings.Contains(p, "Match the tone and style") {
		t.Error("unknown format must NOT take the auto examples-block path")
	}
	if strings.Contains(p, "type(scope): description") || strings.Contains(p, "Begin the subject with exactly ONE emoji") {
		t.Error("unknown format must NOT emit any mode's scaffold body")
	}
	if !strings.Contains(p, "Target ~50 characters for the subject line.") {
		t.Error("unknown format must still retain the subject-target line")
	}
}

// suffix returns the last n bytes of s (for readable failure output).
func suffix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// near returns a short window around the first occurrence of needle in s (for readable failure output).
func near(s, needle string) string {
	i := strings.Index(s, needle)
	if i < 0 {
		return "(needle not found)"
	}
	start := i - 20
	if start < 0 {
		start = 0
	}
	end := i + len(needle) + 20
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}
