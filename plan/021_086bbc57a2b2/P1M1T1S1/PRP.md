name: "P1.M1.T1.S1 — splitFormat parser + bodyForceDirective + forceBody threading + planner base-dispatch fix"
description: >
  Add the +body format-grammar parser foundation to internal/prompt: a `splitFormat(format) (base, forceBody)`
  pure function, a `bodyForceDirective` const (verbatim from PRD §17.8), a forceBody-aware rewrite of
  `buildFormatSystemPrompt` (same signature; forceBody replaces the multilineRule selection), and a
  base-dispatch fix in planner.go's `BuildPlannerSystemPrompt` (parse base before the auto/non-auto
  dispatch + scaffold call). Behavior-preserving for all non-+body inputs (existing tests pass
  unchanged). The parser origin point consumed by S2 (system.go) and S3 (load.go validateFormat).

---

## Goal

**Feature Goal**: Provide the pure grammar parser (`splitFormat`) + the forced-body directive constant +
the forceBody-aware prompt assembly that together let a `<base>[+body>` format string (FR-F9) drive the
message-role system prompt — the foundation S2 (system.go threading) and S3 (validateFormat) build on.

**Deliverable**: (1) `bodyForceDirective` const in format.go (verbatim §17.8); (2) `splitFormat(format)
(base, forceBody)` pure function in format.go; (3) `buildFormatSystemPrompt` rewritten to parse
format→base internally and emit bodyForceDirective when forceBody (same signature); (4) planner.go
`BuildPlannerSystemPrompt` base-dispatch fix (splitFormat before the auto/non-auto branch + scaffold
call); (5) updated doc comments.

**Success Definition**:
- `splitFormat` returns `(format, false)` for no suffix, `(base, true)` for `base+"+body"` (case-sensitive).
- `buildFormatSystemPrompt("conventional+body", ...)` emits conventionalScaffold + bodyForceDirective
  (NOT the multilineRule selection); `buildFormatSystemPrompt("conventional", ...)` is byte-identical to today.
- planner.go routes `auto+body` to the examples loop (base="auto") and `conventional+body` to the scaffold
  (base="conventional") — no body-forcing in the planner.
- Existing format_test.go + planner tests pass UNCHANGED (behavior-preserving for non-+body).
- `go build ./...`, `go test ./internal/prompt/...`, `make test`, `make lint` pass.

## Why

- **FR-F9 (P1)**: the `+body` modifier forces a subject-plus-body message regardless of repo history
  shape. It overrides the conditional multi-line rule (FR12) with an unconditional body directive. This
  subtask builds the parser + directive + assembly that the message-role prompt uses; S2 threads it
  through the top-level builders (incl. the auto+body special case), S3 validates the grammar at load.
- **The planner base-dispatch fix is a latent bug the +body feature exposes**: planner.go dispatches on
  raw `format == "auto"` and calls `formatScaffoldBody(format)` with the raw string. For `auto+body`,
  `format == "auto"` is false → it falls to `formatScaffoldBody("auto+body")` → "" (empty scaffold where
  the examples loop should run). The fix (dispatch on base) is correct independent of +body and must
  land here so the planner is +body-safe.

## What

**User-visible behavior**: None in isolation (the +body feature is not end-to-end usable until S2+S3
land — system.go still routes auto+body to the non-auto branch, and load.go still rejects "+body"). S1
is the parser+assembly foundation.

**Technical change (format.go + planner.go only):**
- `bodyForceDirective` const (verbatim §17.8, no trailing newline).
- `splitFormat` pure function (HasSuffix/TrimSuffix on "+body").
- `buildFormatSystemPrompt`: parse internally, emit bodyForceDirective when forceBody (skip multilineRule).
- planner.go: `base, _ := splitFormat(format)` before dispatch; `if base == "auto"`; `formatScaffoldBody(base)`.

### Success Criteria
- [ ] `splitFormat` correct for: no-suffix→(format,false); "+body" suffix→(base,true); case-sensitive
- [ ] `bodyForceDirective` verbatim from §17.8, no trailing newline
- [ ] `buildFormatSystemPrompt` forceBody-aware; byte-identical for non-+body
- [ ] planner.go dispatches on base; scaffold call uses base; no body-forcing in planner
- [ ] Existing format_test.go/planner tests pass unchanged
- [ ] `go build ./...`, `go test ./internal/prompt/...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact line numbers, the verbatim directive text, the splitFormat implementation, the behavior-preservation proof, the constant-location clarification (system.go vs format.go, same package), and the scope boundaries are all enumerated below (verified by reading source).

### Documentation & References

```yaml
- file: internal/prompt/format.go
  why: "THE primary file. conventionalScaffold(14), gitmojiScaffoldInstruction(18), formatScaffoldBody(24), withLocale(40), buildFormatSystemPrompt(52). imports strings ONLY."
  pattern: "consts have NO trailing newline (package convention — callers own inter-block \\n). formatScaffoldBody switches on the BASE string (conventional/gitmoji/default) — that contract is UNCHANGED; buildFormatSystemPrompt will pass it base, not raw format."
  gotcha: "promptPreamble/multilineRuleAllow/multilineRuleSingle are defined in system.go (NOT format.go) but are package-level ⇒ accessible from format.go (buildFormatSystemPrompt already references them cross-file today). bodyForceDirective goes in format.go next to its sole user."

- file: internal/prompt/planner.go
  why: "THE secondary file. BuildPlannerSystemPrompt(128); the auto/non-auto dispatch at 142 (if format == \"auto\"); the scaffold call at 149 (formatScaffoldBody(format) — RAW format, the bug)."
  pattern: "Fix: base, _ := splitFormat(format) before 142; if base == \"auto\"; formatScaffoldBody(base). The planner DISCARDS forceBody (no bodyForceDirective) — only the message-role path forces bodies."
  gotcha: "Do NOT add body-forcing to the planner. The planner's partitioning prompt is unaffected by +body (PRD §17.8: 'The planner's partitioning prompt is unchanged by format modes'). The fix is ONLY the base-dispatch so auto+body keeps the examples loop."

- file: internal/prompt/system.go (reference only — DO NOT edit in S1)
  why: "Defines promptPreamble(~19), multilineRuleAllow(~55), multilineRuleSingle(~57) — the consts buildFormatSystemPrompt references. Also BuildSystemPrompt/BuildFallbackPrompt (the S2 call sites that will thread forceBody). S2 owns the auto+body dispatch fix here."
  gotcha: "S1 must NOT touch system.go. Today BuildSystemPrompt dispatches format == \"auto\" (raw); auto+body routes to the non-auto branch. That's the S2 'auto+body special case' — not S1's job. buildFormatSystemPrompt handles any <base>[+body] string it's given correctly regardless."

- file: internal/prompt/format_test.go (reference — existing tests must pass unchanged)
  why: "TestFormatScaffoldBody(11), TestBuildFormatSystemPrompt(91) — call buildFormatSystemPrompt(\"conventional\"/\"plain\", ...). After S1 these are byte-identical (forceBody false ⇒ same scaffold + same multilineRule). The formal +body test suite is P1.M2.T1.S1."
  pattern: "No test edits required in S1 for behavior-preservation. Optionally add a splitFormat smoke test (recommended — pins the parser before S3 depends on it)."

- docfile: plan/021_086bbc57a2b2/architecture/format_subsystem.md
  why: "The full subsystem map (format.go/system.go/planner.go/load.go entities + the format flow + the DISCREPANCY note on planner base-dispatch)."
- docfile: plan/021_086bbc57a2b2/P1M1T1S1/research/verification_deltas.md
  why: "The verified line numbers, the verbatim directive, the splitFormat impl, the behavior-preservation proof, the constant-location clarification, and the scope boundaries. READ THIS before editing."
```

### Current Codebase tree (relevant slice)

```bash
internal/prompt/
  format.go           # THE primary: +bodyForceDirective const, +splitFormat func, rewrite buildFormatSystemPrompt
  planner.go          # THE secondary: fix BuildPlannerSystemPrompt base-dispatch (L142/L149)
  system.go           # S2 (thread forceBody through BuildSystemPrompt/BuildFallbackPrompt) — UNTOUCHED in S1
  format_test.go      # existing tests pass unchanged; formal +body suite is P1.M2.T1.S1
internal/config/load.go  # S3 (validateFormat <base>[+body] grammar) — UNTOUCHED in S1
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (behavior-preserving): splitFormat must return (format, false) for any non-+body string,
//   so buildFormatSystemPrompt("conventional", ...) today == buildFormatSystemPrompt("conventional", ...)
//   after S1 (splitFormat("conventional")=("conventional",false) ⇒ same scaffold + same multilineRule).
//   The existing format_test.go tests are the proof — they must pass UNCHANGED.

// CRITICAL (verbatim directive): bodyForceDirective is VERBATIM from PRD §17.8 Design Decision 4 —
//   do not paraphrase. Note the em-dash (—) and "(~72-column)". NO trailing newline (package convention).

// GOTCHA (case-sensitive suffix): "+body" is case-sensitive (FR-F1 grammar). strings.HasSuffix is
//   case-sensitive by default ✓. Do NOT lowercase. "Conventional+Body" is NOT a valid +body (S3 rejects it).

// GOTCHA (planner gets NO body-forcing): the planner's partitioning prompt is unaffected by +body
//   (PRD §17.8). The planner fix is ONLY the base-dispatch (base, _ := splitFormat) so auto+body keeps
//   the examples loop. Never reference bodyForceDirective in planner.go.

// GOTCHA (constant location): promptPreamble/multilineRuleAllow/multilineRuleSingle are in system.go,
//   NOT format.go. They're package-level ⇒ accessible cross-file (buildFormatSystemPrompt already does
//   this today). bodyForceDirective goes in format.go (next to buildFormatSystemPrompt, its sole user).

// SCOPE: S1 is format.go + planner.go ONLY. Do NOT touch system.go (S2), load.go (S3), config.go/
//   bootstrap.go/root.go (S4). buildFormatSystemPrompt keeps its signature; formatScaffoldBody's contract
//   (switch on base) is unchanged.
```

## Implementation Blueprint

### Data models and structure

No struct/type/signature changes. One new const, one new pure function, one rewritten function (same
signature), one 3-line planner fix. No new imports (`strings` already imported in format.go; planner.go
already imports what it needs).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD bodyForceDirective const to internal/prompt/format.go
  - PLACE: near conventionalScaffold/gitmojiScaffoldInstruction (after L18), grouped with the scaffold consts.
  - BODY (VERBATIM from PRD §17.8; NO trailing newline):
        // bodyForceDirective is the §17.8 / FR-F9 unconditional body directive that REPLACES the
        // conditional multi-line rule (multilineRuleAllow/multilineRuleSingle) when a +body format
        // suffix is in effect. NO trailing newline (package convention).
        const bodyForceDirective = "ALWAYS follow the subject with a body — a blank line, then a wrapped (~72-column) explanation of what this change does and why. Use a short bullet list only when the change has several distinct parts. The subject above still follows its format contract."
  - DEPENDENCIES: none.

Task 2: ADD splitFormat func to internal/prompt/format.go
  - PLACE: before buildFormatSystemPrompt (so the caller reads top-down: parser then assembler), or
    immediately after formatScaffoldBody (group the format-grammar helpers). Either is fine.
  - BODY:
        // splitFormat parses the FR-F1 <base>[+body] format grammar: returns (base, false) when format
        // has no "+body" suffix, or (base, true) when format == base + "+body" (case-sensitive on the
        // suffix). It does NOT validate base — the caller (S3 validateFormat) does. Pure (no I/O).
        func splitFormat(format string) (base string, forceBody bool) {
            if strings.HasSuffix(format, "+body") {
                return strings.TrimSuffix(format, "+body"), true
            }
            return format, false
        }
  - NO new import (strings already imported).
  - DEPENDENCIES: none.

Task 3: REWRITE buildFormatSystemPrompt in internal/prompt/format.go (SAME signature, forceBody-aware)
  - KEEP signature: func buildFormatSystemPrompt(format string, hasMultiline bool, subjectTarget int) string
  - NEW BODY:
        func buildFormatSystemPrompt(format string, hasMultiline bool, subjectTarget int) string {
            base, forceBody := splitFormat(format)
            var b strings.Builder
            b.WriteString(promptPreamble)
            b.WriteString("\n\n")
            if body := formatScaffoldBody(base); body != "" {   // base, NOT raw format
                b.WriteString(body)
                b.WriteString("\n\n")
            }
            if forceBody {
                b.WriteString(bodyForceDirective)               // REPLACES the multilineRule if/else
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
  - UPDATE the doc comment to mention +body / forceBody (forceBody replaces the multilineRule selection;
    hasMultiline is ignored under +body; byte-identical for non-+body).
  - DEPENDENCIES: Tasks 1-2.

Task 4: FIX BuildPlannerSystemPrompt base-dispatch in internal/prompt/planner.go
  - BEFORE the `if format == "auto"` (L142), add: base, _ := splitFormat(format)  // planner discards forceBody
  - L142: `if format == "auto"`  →  `if base == "auto"`
  - L149: `b.WriteString(formatScaffoldBody(format))`  →  `b.WriteString(formatScaffoldBody(base))`
  - UPDATE the BuildPlannerSystemPrompt doc comment (L108-area mentions "formatScaffoldBody(format)") to
    note base-dispatch (+body suffixes route to the base's branch; the planner does not force bodies).
  - DO NOT add bodyForceDirective to the planner.
  - DEPENDENCIES: Task 2 (splitFormat must exist).

Task 5: VERIFY — existing tests pass unchanged + optional splitFormat smoke test
  - RUN: go test ./internal/prompt/... -v  (EXPECT green — TestBuildFormatSystemPrompt/TestFormatScaffoldBody
    pass byte-identical for non-+body; planner tests pass with base-dispatch).
  - OPTIONAL (recommended): add a TestSplitFormat table test to format_test.go covering: "auto"→("auto",false),
    "conventional+body"→("conventional",true), "plain+body"→("plain",true), "gitmoji+body"→("gitmoji",true),
    "auto+body"→("auto",true), "Conventional+Body"→("Conventional+Body",false) [case-sensitive],
    "+body"→("+body",false)? [edge: base would be "" — S3 validates/rejects; splitFormat returns ("",true) —
    document the chosen behavior]. The formal +body assembly/system suite is P1.M2.T1.S1.
  - DEPENDENCIES: Tasks 1-4.
```

### Implementation Patterns & Key Details

```go
// PATTERN: splitFormat — pure grammar split, case-sensitive, no validation
func splitFormat(format string) (base string, forceBody bool) {
	if strings.HasSuffix(format, "+body") {
		return strings.TrimSuffix(format, "+body"), true
	}
	return format, false
}

// PATTERN: buildFormatSystemPrompt — forceBody replaces the multilineRule selection
base, forceBody := splitFormat(format)
// ... scaffold uses base ...
if forceBody {
	b.WriteString(bodyForceDirective)      // +body: unconditional body, hasMultiline ignored
} else {
	// the EXISTING if/else on hasMultiline (byte-identical for non-+body)
}

// PATTERN: planner base-dispatch — discard forceBody, route on base
base, _ := splitFormat(format)
if base == "auto" { /* examples loop */ } else { b.WriteString(formatScaffoldBody(base)) }
```

### Integration Points

```yaml
NO struct / signature / config / public-API changes. One const + one pure func + one rewritten func (same sig) + one planner fix.

CODE:
  - internal/prompt/format.go — +bodyForceDirective const, +splitFormat func, buildFormatSystemPrompt rewritten
  - internal/prompt/planner.go — BuildPlannerSystemPrompt base-dispatch fix (L142/L149)

UNCHANGED: buildFormatSystemPrompt signature; formatScaffoldBody contract (switch on base);
  system.go (S2); load.go (S3); config.go/bootstrap.go/root.go (S4).

DOWNSTREAM (consumes splitFormat/bodyForceDirective — do NOT implement in S1):
  - P1.M1.T1.S2: system.go threads forceBody through BuildSystemPrompt/BuildFallbackPrompt (auto+body special case)
  - P1.M1.T1.S3: load.go validateFormat validates <base>[+body] grammar (base ∈ {auto,conventional,gitmoji,plain})
  - P1.M2.T1.S1: formal test suite (splitFormat, +body assembly, system auto+body/conventional+body)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...
go vet ./...
gofmt -l internal/prompt/
# Expected: empty.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation — behavior-preservation gate)

```bash
# Existing prompt tests MUST pass UNCHANGED (the refactor is behavior-preserving for non-+body)
go test ./internal/prompt/... -v
# Expected: TestFormatScaffoldBody, TestBuildFormatSystemPrompt, TestWithLocale, planner tests — all PASS.
#   (splitFormat("conventional")=("conventional",false) ⇒ buildFormatSystemPrompt byte-identical to today.)

# Whole suite (race)
make test
# Expected: ALL pass — ZERO test files modified (an optional splitFormat smoke test is the only addition).
```

### Level 3: Integration Testing (System Validation)

```bash
# (S1 is the parser+assembly foundation; the +body feature is not end-to-end usable until S2+S3 land —
#  system.go still routes auto+body to the non-auto branch, load.go still rejects "+body". The unit suite
#  is the within-scope proof. The full +body e2e [--format conventional+body → subject+body message]
#  is gated by P1.M2.T1.S1's suite after S2/S3 land.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the new entities exist
grep -n "func splitFormat\|bodyForceDirective" internal/prompt/format.go
# Expected: the const + the func present.

# Grep guard: buildFormatSystemPrompt uses splitFormat + base + forceBody (the rewrite)
grep -n "splitFormat(format)\|formatScaffoldBody(base)\|forceBody" internal/prompt/format.go
# Expected: the parse + base scaffold + forceBody branch present.

# Grep guard: planner base-dispatch fixed (no raw format in the dispatch/scaffold)
grep -n 'base == "auto"\|formatScaffoldBody(base)\|base, _ := splitFormat' internal/prompt/planner.go
# Expected: all three present; NO remaining `if format == "auto"` or `formatScaffoldBody(format)` in the dispatch.

# Grep guard: planner does NOT reference bodyForceDirective (no body-forcing in the planner)
grep -n "bodyForceDirective" internal/prompt/planner.go
# Expected: empty.

# Scope-boundary guard: system.go/load.go/config.go/bootstrap.go/root.go UNCHANGED
git diff --name-only
# Expected: only internal/prompt/format.go + internal/prompt/planner.go (+ optionally format_test.go for the smoke test).

# Behavior-preservation guard: the diff is a pure enhancement for +body (non-+body unchanged)
git diff internal/prompt/format.go | grep -E '^-.*multilineRuleAllow|^-.*multilineRuleSingle'
# Expected: empty (the multilineRule lines are not DELETED — they move inside the else branch; no logic lost).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/prompt/` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass — existing prompt tests UNCHANGED

### Feature Validation
- [ ] `splitFormat` correct (no-suffix→(format,false); +body→(base,true); case-sensitive)
- [ ] `bodyForceDirective` verbatim §17.8, no trailing newline
- [ ] `buildFormatSystemPrompt` emits bodyForceDirective under +body; byte-identical for non-+body
- [ ] planner dispatches on base; scaffold uses base; no body-forcing in planner
- [ ] Existing format_test.go/planner tests pass unchanged

### Scope-Boundary Validation
- [ ] system.go UNCHANGED (S2)
- [ ] load.go UNCHANGED (S3)
- [ ] config.go/bootstrap.go/root.go UNCHANGED (S4)
- [ ] buildFormatSystemPrompt signature UNCHANGED; formatScaffoldBody contract (switch on base) UNCHANGED
- [ ] Only format.go + planner.go changed (+ optional format_test.go smoke test)

### Code Quality
- [ ] bodyForceDirective co-located with scaffold consts / its sole user
- [ ] splitFormat pure, documented (no validation — S3 owns that)
- [ ] Doc comments on buildFormatSystemPrompt + BuildPlannerSystemPrompt mention +body/base-dispatch

---

## Anti-Patterns to Avoid

- ❌ Don't validate base inside splitFormat — it's a pure grammar split; S3's validateFormat owns base validation. Mixing them couples the parser to the valid-set and breaks the "caller validates" contract.
- ❌ Don't change buildFormatSystemPrompt's signature — S2's system.go callers depend on `(format, hasMultiline, subjectTarget)`; the parse is internal.
- ❌ Don't pass raw `format` to formatScaffoldBody — pass `base`. formatScaffoldBody switches on exact "conventional"/"gitmoji" matches; "conventional+body" would hit the default→"" branch and silently drop the scaffold.
- ❌ Don't add bodyForceDirective to the planner — the planner's partitioning prompt is unaffected by +body (PRD §17.8). The planner fix is ONLY the base-dispatch.
- ❌ Don't lowercase the suffix — "+body" is case-sensitive (FR-F1). "Conventional+Body" is invalid; S3 rejects it. strings.HasSuffix is already case-sensitive.
- ❌ Don't paraphrase bodyForceDirective — copy VERBATIM from §17.8 (em-dash, "(~72-column)", exact wording). The directive is the model's contract.
- ❌ Don't touch system.go (S2), load.go (S3), or user-facing text (S4) — S1 is format.go + planner.go only.
- ❌ Don't drop the multilineRule lines — they move INSIDE the `else` (forceBody false) branch; non-+body behavior must stay byte-identical.

---

## Confidence Score: 9/10

One-pass success is very high: a pure function (splitFormat) with a 2-line idiomatic impl, a verbatim
const, a behavior-preserving rewrite (the nil⇒identical proof is explicit), and a 3-line planner fix —
all line-verified, no new imports, no signature changes. The -1 is for the splitFormat edge case
`"+body"` (base would be "") and `"Conventional+Body"` (case mismatch) — the function's behavior on
these is well-defined (returns ("",true) / ("Conventional+Body",false)) but an implementer might
second-guess it; S3's validateFormat is what rejects them, and a smoke test documenting the chosen
behavior closes the loop. The behavior-preservation guarantee (existing tests pass unchanged) is the
hard gate.