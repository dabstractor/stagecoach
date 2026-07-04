package prompt

import (
	"strings"
	"testing"
)

// TestFormatScaffoldBody covers every branch of the §17.8 scaffold dispatch: auto/plain → "" (the caller
// keeps the examples block or omits any format contract); conventional → the verbatim scaffold constant;
// gitmoji → the instruction + a blank line + RenderGitmojiTable(); any unknown/unreachable format → "".
func TestFormatScaffoldBody(t *testing.T) {
	cases := []struct {
		format string
		want   string
	}{
		{"auto", ""},
		{"plain", ""},
		{"conventional", conventionalScaffold},
		{"gitmoji", gitmojiScaffoldInstruction + "\n\n" + RenderGitmojiTable()},
		{"bogus-unknown-mode", ""},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.format, func(t *testing.T) {
			got := formatScaffoldBody(tc.format)
			if got != tc.want {
				t.Errorf("formatScaffoldBody(%q) = %q, want %q", tc.format, got, tc.want)
			}
		})
	}
}

// TestFormatScaffoldBody_ConventionalTypeVocab pins the full Conventional Commits type vocabulary is
// present verbatim (FR-F2's required list), independent of the CanonicalExact constant match above.
func TestFormatScaffoldBody_ConventionalTypeVocab(t *testing.T) {
	body := formatScaffoldBody("conventional")
	if !strings.Contains(body, "type(scope): description") {
		t.Error("conventional scaffold missing the format shape")
	}
	for _, ty := range []string{"feat", "fix", "docs", "style", "refactor", "perf", "test", "build", "ci", "chore", "revert"} {
		if !strings.Contains(body, ty) {
			t.Errorf("conventional scaffold missing type %q", ty)
		}
	}
}

// TestFormatScaffoldBody_GitmojiEmbedsTable verifies the gitmoji scaffold body embeds the LIVE
// RenderGitmojiTable() output (S2) rather than a re-fetched or re-embedded copy (FR-F3: no network
// fetch, ever) — the two must be reference-equal in content.
func TestFormatScaffoldBody_GitmojiEmbedsTable(t *testing.T) {
	body := formatScaffoldBody("gitmoji")
	table := RenderGitmojiTable()
	if !strings.HasSuffix(body, table) {
		t.Error("gitmoji scaffold body must end with RenderGitmojiTable() verbatim")
	}
	if !strings.Contains(body, "Begin the subject with exactly ONE emoji") {
		t.Error("gitmoji scaffold body missing the instruction line")
	}
}

// TestWithLocale covers the FR-F6 locale-append rule: empty locale leaves s unchanged; a non-empty
// locale appends exactly one "\nWrite the commit message in <lang>." regardless of whether s already
// ends in a trailing newline (both shapes must normalize to a single "\n" separator).
func TestWithLocale(t *testing.T) {
	cases := []struct {
		name   string
		s      string
		locale string
		want   string
	}{
		{"empty locale leaves s unchanged", "hello", "", "hello"},
		{"empty locale leaves trailing-newline s unchanged", "hello\n", "", "hello\n"},
		{"non-empty locale, s has no trailing newline", "hello", "French", "hello\nWrite the commit message in French."},
		{"non-empty locale, s has ONE trailing newline", "hello\n", "French", "hello\nWrite the commit message in French."},
		{"non-empty locale, s has TWO trailing newlines", "hello\n\n", "French", "hello\nWrite the commit message in French."},
		{"locale passed verbatim (BCP-47 tag)", "hello", "ja", "hello\nWrite the commit message in ja."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := withLocale(tc.s, tc.locale)
			if got != tc.want {
				t.Errorf("withLocale(%q, %q) = %q, want %q", tc.s, tc.locale, got, tc.want)
			}
		})
	}
}

// TestBuildFormatSystemPrompt asserts the shared assembler's topology directly: preamble, then the
// scaffold body (omitted entirely for "plain"), then the multi-line rule (selected by hasMultiline),
// then the subject-target line. Locale is NOT applied here (the caller wraps via withLocale).
func TestBuildFormatSystemPrompt(t *testing.T) {
	t.Run("conventional, hasMultiline=false", func(t *testing.T) {
		got := buildFormatSystemPrompt("conventional", false, 50)
		want := promptPreamble + "\n\n" + conventionalScaffold + "\n\n" + multilineRuleSingle + "\n" +
			"Target ~50 characters for the subject line."
		if got != want {
			t.Errorf("buildFormatSystemPrompt mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
		}
	})
	t.Run("conventional, hasMultiline=true", func(t *testing.T) {
		got := buildFormatSystemPrompt("conventional", true, 50)
		if !strings.Contains(got, multilineRuleAllow) {
			t.Error("hasMultiline=true must select the allow-body rule")
		}
		if strings.Contains(got, multilineRuleSingle) {
			t.Error("hasMultiline=true must not also contain the single-line rule")
		}
	})
	t.Run("plain has no scaffold body at all", func(t *testing.T) {
		got := buildFormatSystemPrompt("plain", false, 50)
		want := promptPreamble + "\n\n" + multilineRuleSingle + "\n" + "Target ~50 characters for the subject line."
		if got != want {
			t.Errorf("buildFormatSystemPrompt(plain) mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
		}
	})
	t.Run("subjectTarget is interpolated, not hardcoded", func(t *testing.T) {
		got := buildFormatSystemPrompt("plain", false, 72)
		if !strings.Contains(got, "Target ~72 characters for the subject line.") {
			t.Error("subjectTarget=72 not interpolated")
		}
		if strings.Contains(got, "~50 characters") {
			t.Error("subjectTarget leaked a hardcoded 50")
		}
	})
}
