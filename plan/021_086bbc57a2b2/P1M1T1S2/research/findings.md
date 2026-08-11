# Research Findings — P1.M1.T1.S2 (system.go: thread forceBody through BuildSystemPrompt + BuildFallbackPrompt)

## 1. S1 contract + status

S1 ("Implementing" in parallel) adds to internal/prompt/format.go (NOT system.go):
- `func splitFormat(format string) (base string, forceBody bool)` — pure grammar split. Returns
  `(format, false)` for no "+body" suffix; `(base, true)` for `base+"+body"` (case-sensitive).
- `const bodyForceDirective` — VERBATIM §17.8 unconditional body directive (replaces the multilineRule).
- `buildFormatSystemPrompt` rewritten to parse format→base internally + emit bodyForceDirective under
  forceBody (same signature; byte-identical for non-+body).

S1 has NOT landed yet (`grep -n splitFormat bodyForceDirective internal/prompt/format.go` empty). S2
CONSUMES `splitFormat` + `bodyForceDirective` (same package `prompt` — no import needed). S2 does NOT
touch format.go/planner.go (S1) or load.go (S3).

## 2. THE BUG S2 fixes (auto+body special case)

Both builders dispatch on RAW `format == "auto"` (string equality). For `auto+body`, `"auto+body" !=
"auto"` is TRUE → the builder falls into the NON-auto branch → delegates to `buildFormatSystemPrompt` →
LOSES the §17.1 examples block (the auto path's whole point). The fix: parse `base` BEFORE dispatching;
route on `base != "auto"`. (The PRD §17.8: "auto+body keeps the learned subject style and forces only
the body" — it must NOT lose the examples block.)

## 3. BuildSystemPrompt (system.go:190-228) — verified structure

```go
func BuildSystemPrompt(examples []string, hasMultiline bool, subjectTarget int, format, locale string) string {
	if format != "auto" {   // ← L191: S2 changes to `base != "auto"`
		return withLocale(buildFormatSystemPrompt(format, hasMultiline, subjectTarget), locale)  // RAW format (S1 handles forceBody)
	}
	var b strings.Builder
	b.WriteString(maturePromptHeader); b.WriteByte('\n')
	for _, ex := range examples { b.WriteString("---\n"); b.WriteString(ex); b.WriteByte('\n') }
	b.WriteByte('\n'); b.WriteString(antiReuseProhibition)
	b.WriteByte('\n'); b.WriteByte('\n')   // blank line before the rule
	if hasMultiline {                       // ← L217-219: S2 makes this forceBody-aware
		b.WriteString(multilineRuleAllow)
	} else {
		b.WriteString(multilineRuleSingle)
	}
	b.WriteByte('\n'); b.WriteString(subjectTargetLine(subjectTarget))   // follows the rule in BOTH cases
	return withLocale(b.String(), locale)
}
```

S2 changes:
- TOP: `base, forceBody := splitFormat(format)`.
- L191: `if format != "auto"` → `if base != "auto"` (non-auto still passes RAW `format` to buildFormatSystemPrompt — S1 handles forceBody there; UNCHANGED call).
- L217-219 (auto path): the `if hasMultiline {...} else {...}` → `if forceBody { b.WriteString(bodyForceDirective) } else if hasMultiline {...} else {...}`.
- The trailing `b.WriteByte('\n') + subjectTargetLine` is UNCHANGED — it follows the rule/directive in both cases.

BYTE-IDENTITY (FR-F1): splitFormat("auto")=("auto",false) → forceBody false → the `else if hasMultiline`
runs EXACTLY as today. Byte-identical. ✓

## 4. BuildFallbackPrompt (system.go:176-186) — verified structure

```go
func BuildFallbackPrompt(subjectTarget int, format, locale string) string {
	if format != "auto" {   // ← L177: S2 changes to `base != "auto"`
		return withLocale(buildFormatSystemPrompt(format, false, subjectTarget), locale)  // RAW format (S1 handles forceBody)
	}
	s := fallbackPromptBody + "\n\n" +
		fmt.Sprintf("Target ~%d characters (~7 words). Format: type(scope): description", subjectTarget)
	return withLocale(s, locale)
}
```

S2 changes:
- TOP: `base, forceBody := splitFormat(format)`.
- L177: `if format != "auto"` → `if base != "auto"` (non-auto still passes RAW format — UNCHANGED).
- Auto path: when forceBody, append bodyForceDirective after the target/format line:
  ```go
  s := fallbackPromptBody + "\n\n" +
      fmt.Sprintf("Target ~%d characters (~7 words). Format: type(scope): description", subjectTarget)
  if forceBody {
      s += "\n\n" + bodyForceDirective   // blank line between the target line and the directive
  }
  return withLocale(s, locale)
  ```
- Separator is "\n\n" (blank line — matches the inter-block convention; consistent with how fallbackPromptBody
  is separated from the target line by "\n\n").

BYTE-IDENTITY: forceBody false → the `if forceBody {}` is skipped → byte-identical. ✓

## 5. The canonical byte-identity tests (must stay GREEN — S2 does NOT modify them)

internal/prompt/system_test.go:
- TestBuildSystemPrompt_CanonicalExact (:13) — format="auto" → splitFormat→("auto",false) → byte-identical. GREEN.
- TestBuildFallbackPrompt_CanonicalExact (:244) — format="auto" → byte-identical. GREEN.
- TestBuildSystemPrompt_FormatModes_CanonicalExact (:331) — conventional/gitmoji/plain (no +body):
  splitFormat→(base,false); base!="auto" → delegates to buildFormatSystemPrompt(base-ish) → S1's rewrite
  is byte-identical for non-+body → GREEN.
- TestBuildFallbackPrompt_FormatModes_CanonicalExact (:450) — same for fallback → GREEN.
- TestBuildSystemPrompt_UnknownFormat_DefaultsToAutoLike (:485) — unknown "weird" → splitFormat→("weird",false);
  base!="auto" → delegates (formatScaffoldBody default branch → "") → auto-like → GREEN.

These are S2's byte-identity gate. The formal +body test suite (auto+body, conventional+body) is P1.M2.T1.S1 — NOT S2.

## 6. No new imports; doc-comment updates

splitFormat + bodyForceDirective are in format.go (same package `prompt`) — accessible from system.go
with NO import (system.go already references buildFormatSystemPrompt/withLocale/subjectTargetLine
cross-file today). The existing constants (promptPreamble, maturePromptHeader, antiReuseProhibition,
multilineRuleAllow, multilineRuleSingle, fallbackPromptBody) are UNCHANGED.

S2 UPDATES the doc comments on BuildSystemPrompt + BuildFallbackPrompt to document +body / forceBody
behavior + the FR-F1 byte-identity guarantee. (Mode A — system.go has no user-facing surface.)

## 7. Scope boundaries (do NOT do)
- Do NOT touch format.go/planner.go (S1 — owns splitFormat/bodyForceDirective/buildFormatSystemPrompt/planner fix).
- Do NOT touch load.go (S3 — validateFormat grammar).
- Do NOT touch config.go/bootstrap.go/root.go (S4 — user-facing text).
- Do NOT add the formal +body test suite (P1.M2.T1.S1 — system_test.go auto+body/conventional+body cases).
- Do NOT change buildFormatSystemPrompt's signature or the constants.

## 8. Validation

`go build ./...`, `go vet ./...`, `gofmt -l internal/prompt/`, `make lint`, `go test ./internal/prompt/...`
(existing canonical tests GREEN — byte-identity), `make test`. The +body correctness is verified by
reasoning + the formal suite in P1.M2.T1.S1; S2's gate is the existing tests staying green.