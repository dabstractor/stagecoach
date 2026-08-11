name: "P1.M1.T1.S2 — system.go: thread forceBody through BuildSystemPrompt + BuildFallbackPrompt (auto+body special case)"
description: >
  Wire the +body (FR-F9) format modifier into the two top-level system-prompt builders in
  internal/prompt/system.go: parse `base, forceBody := splitFormat(format)` at the top of each, change
  the dispatch from raw `format != "auto"` to `base != "auto"` (so `auto+body` keeps the §17.1 examples
  block / the §17.2 fallback body instead of wrongly delegating to buildFormatSystemPrompt), and in the
  auto path handle forceBody (BuildSystemPrompt: bodyForceDirective replaces the multilineRule branch;
  BuildFallbackPrompt: append bodyForceDirective after the target line). Byte-identical for non-+body
  (FR-F1). Consumes splitFormat + bodyForceDirective from sibling S1. Doc-comment updates only (Mode A).

---

## Goal

**Feature Goal**: Make `BuildSystemPrompt` and `BuildFallbackPrompt` correctly handle the `<base>[+body]`
format grammar (FR-F1/FR-F9) for all 4 bases × 2 suffix states — specifically fixing the `auto+body`
special case where the raw `format != "auto"` dispatch would today wrongly route `auto+body` to the
non-auto branch, losing the §17.1 style-examples block (and the §17.2 fallback body). After S2,
`auto+body` keeps the examples/body and forces only the body directive; non-auto bases (incl.
`conventional+body`) still delegate to `buildFormatSystemPrompt` (which S1 made forceBody-aware).

**Deliverable**: (1) `BuildSystemPrompt` (system.go:190) — `splitFormat` at top, dispatch on `base`, auto
path emits `bodyForceDirective` under forceBody (else the unchanged multilineRule selection); (2)
`BuildFallbackPrompt` (system.go:176) — `splitFormat` at top, dispatch on `base`, auto path appends
`bodyForceDirective` under forceBody; (3) updated doc comments on both (Mode A). No new tests (the formal
+body suite is P1.M2.T1.S1); S2's gate is the existing canonical tests staying byte-identical (GREEN).

**Success Definition**:
- `BuildSystemPrompt(..., "auto", ...)` and `BuildFallbackPrompt(..., "auto", ...)` are BYTE-IDENTICAL to pre-delta (FR-F1).
- `BuildSystemPrompt(..., "auto+body", ...)` keeps the §17.1 examples+anti-reuse block AND replaces the multilineRule with bodyForceDirective.
- `BuildFallbackPrompt(..., "auto+body", ...)` keeps the §17.2 fallback body+target line AND appends bodyForceDirective.
- `conventional+body`/`gitmoji+body`/`plain+body` still delegate to `buildFormatSystemPrompt` (raw format passed; S1 handles forceBody internally).
- The existing canonical tests (TestBuildSystemPrompt_CanonicalExact, TestBuildFallbackPrompt_CanonicalExact, the FormatModes tests) pass UNCHANGED.
- `go build ./...`, `go test ./internal/prompt/...`, `make test`, `make lint` pass.

## User Persona (if applicable)

**Target User**: A stagecoach user who runs `--format auto+body` (or `plain+body`, etc.) to force a subject-plus-body commit message while keeping their repo's learned subject style.

**Use Case**: `stagecoach --format auto+body` → the message-role system prompt shows the repo's style examples (learned subject tone) AND an unconditional body directive (always emit a body beneath the subject).

**Pain Points Addressed**: Without S2, `auto+body` would lose the examples block (wrongly treated as non-auto) — the user gets a forced body but loses the style learning they asked `auto` to preserve.

## Why

- **FR-F9 / §17.8 (P1)**: `auto+body` "keeps the learned subject style and forces only the body" (PRD §17.8). The raw `format != "auto"` dispatch defeats this for `auto+body`. S2 parses the base before dispatching so the auto path (examples/fallback body) is preserved while the body is forced.
- **Completes the +body prompt surface**: S1 made `buildFormatSystemPrompt` forceBody-aware and fixed the planner base-dispatch. S2 threads the same parse through the two top-level message-role builders. Together S1+S2 make the message-role prompt fully +body-capable (S3 validates the grammar at load; S4 updates user-facing text).
- **Byte-identity is the hard gate** (FR-F1): `auto` (no suffix) + empty locale must be byte-identical to pre-delta. S2's design guarantees it (splitFormat("auto")=("auto",false) → the unchanged else-branch).

## What

**User-visible behavior**: `--format auto+body` produces a learned-style subject PLUS a forced body. `--format plain+body` / `conventional+body` / `gitmoji+body` already worked via S1's buildFormatSystemPrompt; S2 doesn't change them (still delegates, raw format). `--format auto` (no suffix) is unchanged.

**Technical change (system.go only — 2 functions + doc comments):**
1. Both builders: `base, forceBody := splitFormat(format)` at the top; dispatch `if base != "auto"` (was `if format != "auto"`).
2. Non-auto path: UNCHANGED (passes raw `format` to buildFormatSystemPrompt — S1 handles forceBody).
3. BuildSystemPrompt auto path: `if forceBody { bodyForceDirective } else if hasMultiline {...} else {...}`.
4. BuildFallbackPrompt auto path: append `"\n\n" + bodyForceDirective` when forceBody.

### Success Criteria
- [ ] Both builders call `splitFormat(format)` and dispatch on `base` (not raw `format`)
- [ ] Non-auto path passes raw `format` to buildFormatSystemPrompt (UNCHANGED call)
- [ ] BuildSystemPrompt auto+body: examples+anti-reuse kept + bodyForceDirective replaces multilineRule
- [ ] BuildFallbackPrompt auto+body: fallback body+target line kept + bodyForceDirective appended
- [ ] `auto` (no suffix) byte-identical → existing canonical tests GREEN unchanged
- [ ] Doc comments on both builders document +body / forceBody + FR-F1 byte-identity
- [ ] `go build ./...`, `go test ./internal/prompt/...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — both functions are quoted verbatim with the exact edit points, the byte-identity proof is explicit, the S1 contract (splitFormat/bodyForceDirective) is specified, the canonical-test gate is enumerated, and the scope fences are clear.

### Documentation & References

```yaml
- file: internal/prompt/system.go
  why: "THE file. BuildSystemPrompt (L190): dispatch `if format != \"auto\"` at L191; auto path's
        multilineRule if/else at L217-219; subjectTargetLine after at ~L222. BuildFallbackPrompt (L176):
        dispatch at L177; auto body `fallbackPromptBody + \"\\n\\n\" + Sprintf(...)` at L180-181.
        Constants promptPreamble/maturePromptHeader/antiReuseProhibition/multilineRuleAllow/
        multilineRuleSingle/fallbackPromptBody are UNCHANGED."
  pattern: "BuildSystemPrompt auto path tail: antiReuseProhibition + '\\n\\n' + rule + '\\n' +
            subjectTargetLine. The rule slot is where forceBody swaps in bodyForceDirective."
  critical: "ANCHOR ON CONSTRUCTS (grep `if format != \"auto\"`, `multilineRuleAllow`), not line numbers
             (drift). The non-auto branch MUST pass RAW format (not base) to buildFormatSystemPrompt —
             S1's buildFormatSystemPrompt parses format itself; passing base would double-strip the suffix."

- file: internal/prompt/format.go (S1's output — READ-ONLY for S2)
  why: "splitFormat(format)(base, forceBody) + bodyForceDirective const live here (S1 adds them). Same
        package `prompt` → system.go accesses them with NO import. buildFormatSystemPrompt (S1 rewrote
        it) already handles forceBody internally — S2 does NOT re-parse in the non-auto branch."
  pattern: "splitFormat('auto')=('auto',false); splitFormat('auto+body')=('auto',true);
            splitFormat('conventional+body')=('conventional',true). bodyForceDirective has NO trailing
            newline (package convention)."

- file: internal/prompt/system_test.go (the byte-identity gate — S2 does NOT edit it)
  why: "TestBuildSystemPrompt_CanonicalExact (L13) + TestBuildFallbackPrompt_CanonicalExact (L244) pin
        format='auto' byte-for-byte; TestBuildSystemPrompt_FormatModes_CanonicalExact (L331) +
        TestBuildFallbackPrompt_FormatModes_CanonicalExact (L450) pin non-auto bases;
        TestBuildSystemPrompt_UnknownFormat_DefaultsToAutoLike (L485) pins unknown-format behavior.
        ALL must pass UNCHANGED — they are S2's byte-identity proof."
  pattern: "The formal +body test suite (auto+body/conventional+body cases) is P1.M2.T1.S1 — NOT S2."

- docfile: plan/021_086bbc57a2b2/architecture/format_subsystem.md
  why: "The subsystem map + the DISCREPANCY note (raw format dispatch defeats auto+body) + the
        single-parsing-seam diagram (splitFormat → BuildSystemPrompt/BuildFallbackPrompt). §system.go."
- docfile: plan/021_086bbc57a2b2/P1M1T1S1/PRP.md
  why: "S1 is the CONTRACT — splitFormat/bodyForceDirective/buildFormatSystemPrompt. Confirms S1 does NOT
        touch system.go (S2 owns it) and buildFormatSystemPrompt's signature is UNCHANGED."
- docfile: plan/021_086bbc57a2b2/P1M1T1S2/research/findings.md
  why: "Verbatim function bodies, the byte-identity proof, the canonical-test gate, scope fences."
```

### Current Codebase tree (relevant slice)

```bash
internal/prompt/
  system.go     # THE file: BuildSystemPrompt(L190) + BuildFallbackPrompt(L176) — thread forceBody + doc comments
  format.go     # S1 owns (splitFormat/bodyForceDirective/buildFormatSystemPrompt) — UNCHANGED in S2
  planner.go    # S1 owns (base-dispatch fix) — UNCHANGED in S2
  system_test.go  # canonical byte-identity tests — UNCHANGED in S2 (formal +body suite is P1.M2.T1.S1)
internal/config/load.go  # S3 (validateFormat) — UNCHANGED in S2
```

### Desired Codebase tree with files to be added

```bash
internal/prompt/system.go   # MODIFY: BuildSystemPrompt + BuildFallbackPrompt (splitFormat dispatch + forceBody) + doc comments
```

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (dispatch on base, NOT raw format): the BUG is `if format != "auto"` — for "auto+body" this
//   is TRUE → wrongly delegates to buildFormatSystemPrompt → loses the §17.1 examples block. Fix:
//   `base, forceBody := splitFormat(format); if base != "auto"`. splitFormat("auto")=("auto",false) so
//   plain "auto" still takes the auto path (byte-identical).

// CRITICAL (non-auto branch passes RAW format, not base): buildFormatSystemPrompt parses format itself
//   (S1). Passing base would DOUBLE-STRIP the suffix. Keep the call `buildFormatSystemPrompt(format, ...)`
//   UNCHANGED — only the DISPATCH condition changes (format→base), not the argument.

// CRITICAL (byte-identity is the gate): forceBody must be FALSE for plain "auto" (splitFormat guarantees
//   this). The existing TestBuildSystemPrompt_CanonicalExact / TestBuildFallbackPrompt_CanonicalExact
//   MUST pass UNCHANGED. If they fail, the auto path diverged from pre-delta — fix before proceeding.

// GOTCHA (BuildSystemPrompt forceBody placement): bodyForceDirective REPLACES the entire multilineRule
//   if/else (multilineRuleAllow/multilineRuleSingle) — it does NOT prepend. So:
//     if forceBody { bodyForceDirective } else if hasMultiline { multilineRuleAllow } else { multilineRuleSingle }
//   The trailing '\n' + subjectTargetLine is UNCHANGED (follows the rule/directive in both cases).

// GOTCHA (BuildFallbackPrompt forceBody placement): bodyForceDirective is APPENDED AFTER the target/format
//   line (not replacing anything) — the new-repo auto+body case keeps the §17.2 body + adds the directive.
//   Separator "\n\n" (blank line, matching the fallbackPromptBody↔targetLine convention).

// GOTCHA (no new import): splitFormat + bodyForceDirective are in format.go, same package `prompt` —
//   system.go already references buildFormatSystemPrompt/withLocale cross-file. No import change.

// SCOPE: S2 is system.go ONLY (BuildSystemPrompt + BuildFallbackPrompt + their doc comments). Do NOT
//   touch format.go/planner.go (S1), load.go (S3), config.go/bootstrap.go/root.go (S4), or system_test.go
//   (the formal +body suite is P1.M2.T1.S1). Do NOT change buildFormatSystemPrompt's signature or constants.
```

## Implementation Blueprint

### Data models and structure
None. Pure control-flow edits in two functions + doc-comment updates. No struct/type/signature/import changes.

### Implementation Tasks (ordered by dependencies)

> **Prerequisite — S1 must land `splitFormat` + `bodyForceDirective` in format.go first.** Confirm:
> `grep -n 'func splitFormat\|bodyForceDirective' internal/prompt/format.go`. S2 won't compile without them.
> Anchor on constructs (grep), not line numbers (they drift).

```yaml
Task 1: MODIFY internal/prompt/system.go — BuildSystemPrompt thread forceBody
  - TOP of the func (before the `if format != "auto"` dispatch): add
        base, forceBody := splitFormat(format)
  - L191 dispatch: `if format != "auto"`  →  `if base != "auto"`
        (the non-auto branch body is UNCHANGED — still `return withLocale(buildFormatSystemPrompt(format, hasMultiline, subjectTarget), locale)`
         — passes RAW format; S1 handles forceBody inside buildFormatSystemPrompt.)
  - L217-219 (auto path multilineRule selection): replace
        if hasMultiline {
            b.WriteString(multilineRuleAllow)
        } else {
            b.WriteString(multilineRuleSingle)
        }
    with
        if forceBody {
            b.WriteString(bodyForceDirective)   // +body: unconditional body; hasMultiline ignored
        } else if hasMultiline {
            b.WriteString(multilineRuleAllow)
        } else {
            b.WriteString(multilineRuleSingle)
        }
  - The trailing `b.WriteByte('\n') + b.WriteString(subjectTargetLine(subjectTarget))` is UNCHANGED.
  - UPDATE the BuildSystemPrompt doc comment (currently cites "format==\"auto\" reproduces §17.1 ... any
    other mode replaces the style-examples block"): add the +body behavior — "auto+body keeps the §17.1
    examples+anti-reuse block and replaces the multi-line rule with bodyForceDirective (FR-F9); base
    dispatch (splitFormat) so auto+body is NOT routed to the non-auto branch. Byte-identical for auto
    (no suffix) per FR-F1."
  - DEPENDENCIES: S1 (splitFormat + bodyForceDirective exist).

Task 2: MODIFY internal/prompt/system.go — BuildFallbackPrompt thread forceBody
  - TOP of the func (before the dispatch): add
        base, forceBody := splitFormat(format)
  - L177 dispatch: `if format != "auto"`  →  `if base != "auto"`
        (non-auto branch UNCHANGED — `return withLocale(buildFormatSystemPrompt(format, false, subjectTarget), locale)` — RAW format.)
  - Auto path: replace
        s := fallbackPromptBody + "\n\n" +
            fmt.Sprintf("Target ~%d characters (~7 words). Format: type(scope): description", subjectTarget)
        return withLocale(s, locale)
    with
        s := fallbackPromptBody + "\n\n" +
            fmt.Sprintf("Target ~%d characters (~7 words). Format: type(scope): description", subjectTarget)
        if forceBody {
            s += "\n\n" + bodyForceDirective   // new-repo auto+body: keep §17.2 body + force a body (FR-F9)
        }
        return withLocale(s, locale)
  - UPDATE the BuildFallbackPrompt doc comment: add the +body behavior — "auto+body keeps the §17.2
    fallback body+target line and appends bodyForceDirective; base dispatch (splitFormat); byte-identical
    for auto (no suffix) per FR-F1."
  - DEPENDENCIES: S1 + Task 1.

Task 3: VERIFY — byte-identity gate (existing canonical tests GREEN, unchanged)
  - go build ./...  (splitFormat/bodyForceDirective resolve once S1 lands)
  - go test ./internal/prompt/... -v
    EXPECT: TestBuildSystemPrompt_CanonicalExact, TestBuildFallbackPrompt_CanonicalExact,
            TestBuildSystemPrompt_FormatModes_CanonicalExact, TestBuildFallbackPrompt_FormatModes_CanonicalExact,
            TestBuildSystemPrompt_UnknownFormat_DefaultsToAutoLike — ALL PASS UNCHANGED.
  - gofmt -l internal/prompt/system.go ; make test ; make lint
  - DEPENDENCIES: Tasks 1-2.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the dispatch fix (both builders) — parse base BEFORE dispatching
base, forceBody := splitFormat(format)
if base != "auto" {                                  // was: if format != "auto"
    return withLocale(buildFormatSystemPrompt(format, ...), locale)  // RAW format (S1 parses); UNCHANGED call
}
// ... auto path ...

// PATTERN: BuildSystemPrompt auto path — forceBody swaps the rule slot
if forceBody {
    b.WriteString(bodyForceDirective)        // +body: unconditional body directive
} else if hasMultiline {
    b.WriteString(multilineRuleAllow)        // unchanged
} else {
    b.WriteString(multilineRuleSingle)       // unchanged
}
// '\n' + subjectTargetLine UNCHANGED (follows the rule/directive either way)

// PATTERN: BuildFallbackPrompt auto path — append the directive under forceBody
s := fallbackPromptBody + "\n\n" + fmt.Sprintf("Target ~%d ...", subjectTarget)
if forceBody {
    s += "\n\n" + bodyForceDirective
}
return withLocale(s, locale)
```

### Integration Points

```yaml
NO struct / signature / config / import changes. Two function-body edits + doc comments.

CODE:
  - internal/pprompt/system.go — BuildSystemPrompt + BuildFallbackPrompt (splitFormat dispatch + forceBody) + doc comments

CONSUMED (from S1, must land first):
  - splitFormat(format) (base, forceBody) — internal/prompt/format.go
  - bodyForceDirective const — internal/prompt/format.go
  - buildFormatSystemPrompt (S1 rewrote, forceBody-aware, SAME signature)

UNCHANGED: format.go/planner.go (S1); load.go (S3); config.go/bootstrap.go/root.go (S4);
  system_test.go (the formal +body suite is P1.M2.T1.S1); buildFormatSystemPrompt signature; all constants.

DOWNSTREAM (the 4 call sites gain +body automatically — they pass cfg.Format verbatim):
  - generate.go, decompose/message.go, decompose/planner.go (S1 fixed its base-dispatch), hook/exec.go
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...     # splitFormat/bodyForceDirective resolve once S1 lands; no new import in system.go
go vet ./...
gofmt -l internal/prompt/system.go
# Expected: empty.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation — byte-identity gate)

```bash
# The canonical byte-identity tests MUST pass UNCHANGED (S2 edits no test file)
go test ./internal/prompt/... -v
# Expected: TestBuildSystemPrompt_CanonicalExact, TestBuildFallbackPrompt_CanonicalExact,
#           TestBuildSystemPrompt_FormatModes_CanonicalExact, TestBuildFallbackPrompt_FormatModes_CanonicalExact,
#           TestBuildSystemPrompt_UnknownFormat_DefaultsToAutoLike — ALL PASS.
#           (auto→forceBody false→byte-identical; non-auto bases still delegate; unknown format still auto-like.)

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# (S2 is prompt assembly with no I/O. The +body end-to-end [e.g. --format auto+body → subject+body
#  message] is gated by P1.M2.T1.S1's formal suite + S3's validateFormat landing. S2's within-scope
#  proof is the byte-identity gate above + reasoning about the +body paths.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: both builders parse base + dispatch on base (not raw format)
grep -n 'base, forceBody := splitFormat\|if base != "auto"' internal/prompt/system.go
# Expected: 2 splitFormat + 2 `if base != "auto"` (one per builder). NO remaining `if format != "auto"`.

# Grep guard: the non-auto branch still passes RAW format (not base) to buildFormatSystemPrompt
grep -n 'buildFormatSystemPrompt(format' internal/prompt/system.go
# Expected: 2 hits (both builders) — RAW format. (buildFormatSystemPrompt(base would be a BUG — double-strip.)

# Grep guard: BuildSystemPrompt forceBody branch present
grep -n 'forceBody {' internal/prompt/system.go
# Expected: the BuildSystemPrompt `if forceBody { b.WriteString(bodyForceDirective) }` + the BuildFallbackPrompt `if forceBody { s += ... }`.

# Grep guard: no remaining raw `if format != "auto"` dispatch in system.go
grep -n 'if format != "auto"' internal/prompt/system.go
# Expected: EMPTY (both builders now dispatch on base).

# Byte-identity smoke (manual): auto + empty locale is byte-identical
go test ./internal/prompt/ -run 'CanonicalExact' -v
# Expected: PASS (the FR-F1 guarantee).

# Scope-boundary guard: only system.go changed
git diff --name-only
# Expected: only internal/prompt/system.go. (format.go/planner.go are S1; load.go is S3; system_test.go is P1.M2.T1.S1.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean (S1 landed; splitFormat/bodyForceDirective resolve)
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/prompt/system.go` empty
- [ ] `make lint` zero errors
- [ ] `go test ./internal/prompt/...` green; `make test` green

### Feature Validation
- [ ] Both builders call `splitFormat(format)` and dispatch on `base` (not raw `format`)
- [ ] Non-auto branch passes RAW `format` to buildFormatSystemPrompt (UNCHANGED call)
- [ ] BuildSystemPrompt auto+body: examples+anti-reuse kept + bodyForceDirective replaces multilineRule
- [ ] BuildFallbackPrompt auto+body: fallback body+target line kept + bodyForceDirective appended
- [ ] `auto` (no suffix) byte-identical → TestBuild*_CanonicalExact GREEN unchanged
- [ ] Doc comments on both document +body/forceBody + FR-F1 byte-identity

### Scope-Boundary Validation
- [ ] format.go / planner.go UNCHANGED (S1)
- [ ] load.go UNCHANGED (S3)
- [ ] config.go / bootstrap.go / root.go UNCHANGED (S4)
- [ ] system_test.go UNCHANGED (formal +body suite is P1.M2.T1.S1)
- [ ] buildFormatSystemPrompt signature + all constants UNCHANGED
- [ ] Only internal/prompt/system.go changed

### Code Quality
- [ ] forceBody branch is an `else if`/additive (non-+body path byte-identical — no logic moved/deleted)
- [ ] Doc comments cite FR-F1 (byte-identity) + FR-F9 (+body) + the base-dispatch rationale
- [ ] bodyForceDirective used VERBATIM (defined in format.go; no paraphrase in system.go)

---

## Anti-Patterns to Avoid

- ❌ Don't dispatch on raw `format` (`if format != "auto"`) — that's the BUG: `auto+body` wrongly routes to the non-auto branch and loses the examples/fallback body. Parse `base` first; dispatch on `base`.
- ❌ Don't pass `base` (instead of raw `format`) to buildFormatSystemPrompt in the non-auto branch — buildFormatSystemPrompt parses format itself (S1); passing base would double-strip the suffix and drop the forceBody signal. Keep the call `buildFormatSystemPrompt(format, ...)` UNCHANGED; only the dispatch condition changes.
- ❌ Don't PREPEND bodyForceDirective in BuildSystemPrompt — it REPLACES the multilineRule if/else (the rule slot). The subject-target line follows the rule/directive unchanged.
- ❌ Don't forget the byte-identity guarantee — `auto` (no suffix) MUST be byte-identical. The canonical tests are the proof; if they fail, the auto path diverged. The `else if hasMultiline` (not a restructured if/else) preserves the exact non-+body path.
- ❌ Don't use a single "\n" separator in BuildFallbackPrompt's forceBody append — use "\n\n" (blank line) to match the inter-block convention (fallbackPromptBody↔targetLine is already "\n\n").
- ❌ Don't add the formal +body tests in S2 — that's P1.M2.T1.S1 (system_test.go auto+body/conventional+body cases). S2's gate is the EXISTING canonical tests staying green.
- ❌ Don't touch format.go/planner.go (S1), load.go (S3), user-facing text (S4), or system_test.go.
- ❌ Don't change buildFormatSystemPrompt's signature or any constant — S2 is two function bodies + doc comments only.
- ❌ Don't re-validate the base in system.go — splitFormat is a pure grammar split (no validation); S3's validateFormat owns base validation. system.go just routes on whatever base splitFormat returns.

---

## Confidence Score: 9/10

One-pass success is very high: two small, surgical edits (a dispatch condition swap `format`→`base` + a
forceBody branch) with the function bodies quoted verbatim, the byte-identity proof explicit (splitFormat
guarantees forceBody=false for plain "auto"), and the canonical-test gate enumerated. The -1 is the S1
compile dependency (splitFormat/bodyForceDirective must land first; S2 won't compile without them) and the
BuildFallbackPrompt separator choice ("\n\n" is the consistent convention but the item says "append after
the target line" without pinning the exact separator — the PRP pins "\n\n" with rationale). Mitigated by
the prerequisite check + the byte-identity gate (canonical tests catch any auto-path divergence).