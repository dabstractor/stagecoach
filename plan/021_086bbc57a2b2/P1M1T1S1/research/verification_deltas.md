# Research Notes — P1.M1.T1.S1 (splitFormat parser + bodyForceDirective + forceBody threading + planner base-dispatch)

Verification against the CURRENT working tree. The task description + `architecture/format_subsystem.md`
are accurate. These notes record the verified anchors + the constant-location clarification.

## VERIFIED — format.go (line numbers)
- L14: `const conventionalScaffold = "Format: type(scope): description..."`
- L18: `const gitmojiScaffoldInstruction = "Begin the subject with exactly ONE emoji..."`
- L24: `func formatScaffoldBody(format string) string` — switch "conventional"→conventionalScaffold,
  "gitmoji"→instruction+table, default→"" (auto/plain/unknown)
- L40: `func withLocale(s, locale string) string`
- L52: `func buildFormatSystemPrompt(format string, hasMultiline bool, subjectTarget int) string`:
  writes `promptPreamble` + `"\n\n"` + (`formatScaffoldBody(format)` if non-"" + `"\n\n"`) +
  (`multilineRuleAllow` if hasMultiline else `multilineRuleSingle`) + `"\n"` + `subjectTargetLine(...)`
- imports `strings` ONLY.

## CLARIFICATION — where the referenced constants live (package-level access, file doesn't matter)
The task says "constants (multilineRuleAllow, multilineRuleSingle, promptPreamble,
conventionalScaffold, gitmojiScaffoldInstruction) are available in format.go." More precisely:
- `conventionalScaffold` (format.go:14) + `gitmojiScaffoldInstruction` (format.go:18) ARE in format.go.
- `promptPreamble`, `multilineRuleAllow`, `multilineRuleSingle` are in **system.go** (~L19, ~L55, ~L57)
  per format_subsystem.md. But ALL are package-level `prompt` consts ⇒ accessible from format.go
  regardless of defining file. `buildFormatSystemPrompt` (format.go) already references
  `promptPreamble`/`multilineRuleAllow`/`multilineRuleSingle` today (cross-file, same package).
- `bodyForceDirective` (NEW) goes in format.go (co-located with buildFormatSystemPrompt, its sole user).
  It can reference the system.go consts the same way buildFormatSystemPrompt already does.

## VERIFIED — planner.go (line numbers)
- L128: `func BuildPlannerSystemPrompt(examples []string, format, locale string, forcedCount, maxCommits int) string`
- L142: `if format == "auto" {` (examples loop) `} else {`
- L149: `b.WriteString(formatScaffoldBody(format))` — RAW format string
- THE BUG: with `auto+body`, `format == "auto"` is FALSE → falls to else → `formatScaffoldBody("auto+body")`
  → "" (default branch) → empty scaffold where the examples loop should have run. Base-dispatch fix needed.

## THE DELIVERABLES (task-specified)

### (a) bodyForceDirective const in format.go — VERBATIM from PRD §17.8 Design Decision 4
```go
const bodyForceDirective = "ALWAYS follow the subject with a body — a blank line, then a wrapped (~72-column) explanation of what this change does and why. Use a short bullet list only when the change has several distinct parts. The subject above still follows its format contract."
```
NO trailing newline (package convention — callers own inter-block `\n` placement, same as
conventionalScaffold/gitmojiScaffoldInstruction). Note the em-dash (—) and the en-dash-free wrapping;
copy verbatim (the §17.8 text is the single source of truth).

### (b) splitFormat(format string) (base string, forceBody bool) in format.go
```go
func splitFormat(format string) (base string, forceBody bool) {
	if strings.HasSuffix(format, "+body") {
		return strings.TrimSuffix(format, "+body"), true
	}
	return format, false
}
```
- Case-SENSITIVE on the "+body" suffix (the suffix must be the trailing 5 chars). `strings.HasSuffix`
  is case-sensitive by default ✓.
- Caller validates `base` (S3's validateFormat). splitFormat does NOT validate — pure grammar split.
- Idiomatic Go: `strings.HasSuffix` + `strings.TrimSuffix` (both already-imported `strings`).

### (c) buildFormatSystemPrompt — SAME signature, forceBody-aware internally
```go
func buildFormatSystemPrompt(format string, hasMultiline bool, subjectTarget int) string {
	base, forceBody := splitFormat(format)
	var b strings.Builder
	b.WriteString(promptPreamble)
	b.WriteString("\n\n")
	if body := formatScaffoldBody(base); body != "" {   // ← base, NOT raw format
		b.WriteString(body)
		b.WriteString("\n\n")
	}
	if forceBody {
		b.WriteString(bodyForceDirective)               // ← REPLACES the multilineRule selection
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
```
- When forceBody==false: byte-identical to today (splitFormat("conventional")=("conventional",false);
  formatScaffoldBody("conventional") unchanged; the if/else on hasMultiline runs). ⇒ existing
  format_test.go tests pass UNCHANGED.
- When forceBody==true: the if/else on hasMultiline is SKIPPED entirely; bodyForceDirective is written.
  hasMultiline becomes irrelevant under +body (correct — +body overrides FR12).

### (d) planner.go base-dispatch fix (L142, L149)
Before L142, add: `base, _ := splitFormat(format)` (planner discards forceBody — it does NOT get
body-forcing; only the message-role path does).
- L142: `if format == "auto"` → `if base == "auto"`
- L149: `formatScaffoldBody(format)` → `formatScaffoldBody(base)`
Result: `auto+body` → base="auto" → examples loop runs (correct); `conventional+body` → base="conventional"
→ scaffold (correct, no body-forcing in the planner). The planner NEVER references bodyForceDirective.

## BEHAVIOR-PRESERVATION PROOF (non-+body inputs ⇒ byte-identical)
- splitFormat("auto")=("auto",false); splitFormat("conventional")=("conventional",false);
  splitFormat("plain")=("plain",false); splitFormat("gitmoji")=("gitmoji",false).
- buildFormatSystemPrompt: forceBody false ⇒ formatScaffoldBody(base)==formatScaffoldBody(format today)
  + the same multilineRule selection. Byte-identical. ⇒ existing TestBuildFormatSystemPrompt passes.
- planner: base==format for non-+body ⇒ `if base == "auto"` == `if format == "auto"` today;
  formatScaffoldBody(base)==formatScaffoldBody(format). Byte-identical.

## SCOPE BOUNDARIES (sibling subtasks — do NOT implement here)
- **P1.M1.T1.S2**: system.go — thread forceBody through BuildSystemPrompt + BuildFallbackPrompt. This is
  the "auto+body special case": today BuildSystemPrompt dispatches `format == "auto"` (raw), so auto+body
  (format != "auto") wrongly routes to the non-auto branch (no examples). S2 fixes the dispatch so
  auto+body keeps the examples block while forcing the body. S1 does NOT touch system.go.
- **P1.M1.T1.S3**: load.go — extend validateFormat to the `<base>[+body]` grammar (validate base ∈
  {auto,conventional,gitmoji,plain} + optional +body suffix; reject unknown base, +body on unknown,
  repeated +body). S1's splitFormat does NOT validate — S3 does.
- **P1.M1.T1.S4**: user-facing text (config.go Format doc, bootstrap.go template, root.go --format help).
- **P1.M2.T1.S1**: the formal test suite (format_test.go splitFormat + assembly; system_test.go auto+body).
  S1 should ensure existing tests pass; a splitFormat smoke test is recommended but the formal suite is P1.M2.
- Do NOT: add body-forcing to the planner; touch system.go/load.go/config.go/bootstrap.go/root.go;
  change any signature (buildFormatSystemPrompt keeps `(format, hasMultiline, subjectTarget)`);
  change formatScaffoldBody's contract (it still switches on the BASE string — "conventional"/"gitmoji"/default).

## NOTE — what "works" after S1 in isolation
After S1 alone: buildFormatSystemPrompt correctly handles any `<base>[+body>` string it receives
(forceBody-aware). The planner correctly base-dispatches. BUT system.go still routes auto+body to the
non-auto branch (no examples) — a functional-but-not-ideal prompt until S2 lands the auto+body special
case. And load.go's validateFormat still rejects "+body" suffixes (hard error) until S3 lands. So the
+body feature is not end-to-end usable until S1+S2+S3 all land. S1 is the parser+assembly foundation;
S2/S3 are required to complete the feature. This is expected (atomic-by-subtask compile-clean, not
atomic-by-subtask feature-complete).