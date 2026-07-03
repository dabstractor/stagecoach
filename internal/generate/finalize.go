package generate

import (
	"strings"

	"github.com/dustin/stagehand/internal/config"
)

// ApplyTemplate applies the §9.19 FR-F8 message template: every literal "$msg" in tpl is replaced with the
// full generated message. Empty tpl ⇒ msg unchanged (the default; byte-identical to the pre-feature path).
// This is a POST-generation substitution (§17.8: "the model never sees it"), applied AFTER parse/cleanup
// and BEFORE the duplicate check so §9.7 judges the final subject as it will land. Substitution is literal
// and covers the FULL message (subject+body); "$msg" alone is the identity template.
func ApplyTemplate(msg, tpl string) string {
	if tpl == "" {
		return msg
	}
	return strings.ReplaceAll(tpl, "$msg", msg)
}

// FinalizeMessage is the shared message-finalization SEAM (§9.19 FR-F8): the single ordered pipeline every
// commit path funnels a parsed+cleaned message through to obtain the FINAL message as it will land. Today
// it is one stage — ApplyTemplate(msg, cfg.Template). It is invoked AFTER ParseOutput and BEFORE
// ExtractSubject/IsDuplicate in every generation loop, and on the planner's FR-M11 shortcut message before
// its dup-check, so the dedupe check (§9.7) always sees the templated subject.
//
// ORDERING CONTRACT (P1.M5.T1.S1): the --edit editor gate slots AFTER this seam (FR-E3: the template is
// applied before the editor opens). Extend the pipeline as template → (future) editor → publish; keep
// template first.
func FinalizeMessage(msg string, cfg config.Config) string {
	return ApplyTemplate(msg, cfg.Template)
}
