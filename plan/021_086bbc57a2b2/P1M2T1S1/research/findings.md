# Research: P1.M2.T1.S1 — +body format-modifier TEST suite (load_test / format_test / system_test)

Test-only task. Consumes the +body implementation from P1.M1.T1.S1–S3 (Complete/Ready). Adds the
+body test coverage across 3 files. **CRITICAL: several target tests ALREADY EXIST** (written by the
implementation subtasks as pin-tests) — this task FILLS THE GAPS (assembly + end-to-end), it does NOT
re-create what's there (doing so = a duplicate-func compile error).

All claims verified against the current source (format.go, system.go, load.go) + the 3 test files.

---

## 0. GAP MAP — what EXISTS vs. what to ADD (the key planning fact)

| Test | File | +body status | This task's action |
|------|------|--------------|--------------------|
| `TestValidateFormat` (L1356) | load_test.go | ✅ **ALREADY has** all 4 +body valid + 4 invalid cases + the "auto, conventional, gitmoji, plain" assertion (written by S3) | VERIFY present; optionally AUGMENT the error-substring assertion (see §3). DO NOT re-add. |
| `TestSplitFormat` (L131) | format_test.go | ✅ **ALREADY EXISTS** — full table: all 4 bases, +body on each, case-sensitivity (Conventional+Body, AUTO+BODY), +body alone (""), empty (written by S1) | VERIFY present + covers the contract's cases. DO NOT re-create (duplicate `TestSplitFormat` = compile error). |
| `TestBuildFormatSystemPrompt` (L91) | format_test.go | ❌ has conventional/plain topology subtests but **NO +body subtests** | **ADD** `buildFormatSystemPrompt("<base>+body", …)` subtests (§1). |
| `TestBuildSystemPrompt_FormatModes_Properties` (L383) | system_test.go | ❌ loops conventional/gitmoji/plain; **NO +body** | **ADD** an auto+body topology case (§2a). |
| `TestBuildSystemPrompt_FormatModes_CanonicalExact` (L331) | system_test.go | ❌ conventional/gitmoji/plain exact; **NO +body** | **ADD** a conventional+body end-to-end row (§2b). |
| `TestBuildFallbackPrompt_FormatModes_CanonicalExact` (L450) | system_test.go | ❌ conventional/gitmoji/plain; **NO +body** | **OPTIONALLY ADD** a BuildFallbackPrompt auto+body row (§2c). |
| `TestBuildSystemPrompt_CanonicalExact` (L13) | system_test.go | auto byte-identity baseline | ✅ MUST stay GREEN unchanged (FR-F1). |
| `TestBuildFallbackPrompt_CanonicalExact` (L244) | system_test.go | auto byte-identity baseline | ✅ MUST stay GREEN unchanged (FR-F1). |

**Net new work = (1) format_test +body subtests + (2) system_test auto+body/conventional+body cases.**
TestSplitFormat and the load_test +body cases are DONE (verify-only).

---

## 1. format_test.go — ADD +body subtests to TestBuildFormatSystemPrompt (or a new test)

The assembler `buildFormatSystemPrompt(format, hasMultiline, subjectTarget)` (format.go:74) calls
splitFormat internally; under +body it emits `bodyForceDirective` INSTEAD of multilineRuleAllow/Single
(format.go:84). Add subtests (mirror the existing `t.Run` idiom at L91):

```go
t.Run("conventional+body forces the body directive (rule replaced)", func(t *testing.T) {
    for _, hm := range []bool{false, true} { // hasMultiline is IGNORED under +body
        got := buildFormatSystemPrompt("conventional+body", hm, 50)
        if !strings.Contains(got, bodyForceDirective) { t.Error("conventional+body must contain bodyForceDirective") }
        if strings.Contains(got, multilineRuleAllow)  { t.Error("conventional+body must NOT contain multilineRuleAllow (replaced)") }
        if strings.Contains(got, multilineRuleSingle) { t.Error("conventional+body must NOT contain multilineRuleSingle (replaced)") }
        if !strings.Contains(got, conventionalScaffold){ t.Error("conventional+body must retain conventionalScaffold (subject contract preserved)") }
    }
})
t.Run("gitmoji+body forces the body directive", func(t *testing.T) {
    got := buildFormatSystemPrompt("gitmoji+body", false, 50)
    if !strings.Contains(got, bodyForceDirective) { t.Error("gitmoji+body must contain bodyForceDirective") }
    if strings.Contains(got, multilineRuleAllow) || strings.Contains(got, multilineRuleSingle) { t.Error("rule not replaced") }
    if !strings.Contains(got, gitmojiScaffoldInstruction) { t.Error("gitmoji+body must retain the gitmoji scaffold") }
})
t.Run("plain+body forces the body directive (no scaffold)", func(t *testing.T) {
    got := buildFormatSystemPrompt("plain+body", false, 50)
    if !strings.Contains(got, bodyForceDirective) { t.Error("plain+body must contain bodyForceDirective") }
    if strings.Contains(got, multilineRuleAllow) || strings.Contains(got, multilineRuleSingle) { t.Error("rule not replaced") }
    // plain has NO scaffold body (formatScaffoldBody("plain")=="") — nothing scaffold-ish to assert present
})
```
Key invariant: **hasMultiline is IGNORED under +body** (loop both false/true for conventional+body to
prove the directive wins either way). The subject contract is preserved (conventionalScaffold /
gitmojiScaffoldInstruction still present).

---

## 2. system_test.go — ADD the auto+body + conventional+body end-to-end cases

### (2a) auto+body topology — the auto+body SPECIAL CASE (system.go:202-235)
`BuildSystemPrompt(examples, hasMultiline, subjectTarget, "auto+body", locale)` is special: auto
RETAINS the examples block (examplesIntro + antiReuseProhibition) AND forces the body. Add to
`TestBuildSystemPrompt_FormatModes_Properties` (or a new test):

```go
t.Run("auto+body retains examples block AND forces body", func(t *testing.T) {
    p := BuildSystemPrompt([]string{"feat: a", "fix: b"}, true, 50, "auto+body", "")
    if !strings.Contains(p, examplesIntro)          { t.Error("auto+body must RETAIN the examples intro (auto keeps the block)") }
    if !strings.Contains(p, antiReuseProhibition)   { t.Error("auto+body must RETAIN the anti-reuse block") }
    if !strings.Contains(p, bodyForceDirective)     { t.Error("auto+body must contain bodyForceDirective") }
    if strings.Contains(p, multilineRuleAllow)      { t.Error("auto+body must NOT contain multilineRuleAllow (replaced)") }
    if strings.Contains(p, multilineRuleSingle)     { t.Error("auto+body must NOT contain multilineRuleSingle (replaced)") }
})
```

### (2b) conventional+body end-to-end — add a row to TestBuildSystemPrompt_FormatModes_CanonicalExact
This is an EXACT `==` comparison (the table's idiom). Under conventional+body the body replaces the
multiline rule, so want = `preamble + scaffold + bodyForceDirective + targetline`:

```go
{
    name: "conventional+body, no locale", format: "conventional+body", locale: "",
    want: promptPreamble + "\n\n" + conventionalScaffold + "\n\n" + bodyForceDirective + "\n" +
        "Target ~50 characters for the subject line.",
},
```
(Verify the exact joining against the actual BuildSystemPrompt output — run the test once; if the
want string's separators differ, adjust to match. The structural fact is: scaffold + bodyForceDirective
+ target line, NO multilineRule. NOTE: hasMultiline is passed `false` in the table's existing rows —
under +body it's ignored, so the row is valid with either, but match the table's `false` for consistency.)

### (2c) OPTIONAL — BuildFallbackPrompt auto+body (system.go:178-186)
`BuildFallbackPrompt(subjectTarget, "auto+body", locale)` appends bodyForceDirective to the §17.2
fallback. Add a row to `TestBuildFallbackPrompt_FormatModes_CanonicalExact`:
```go
{
    name: "auto+body, no locale", format: "auto+body", locale: "",
    want: promptPreamble + "\n\n" + /* §17.2 fallback body */ + "\n\n" + bodyForceDirective,
},
```
(Read BuildFallbackPrompt's exact §17.2 assembly to get the want string; this is optional — the
contract says "optionally add a BuildFallbackPrompt auto+body case".)

---

## 3. load_test.go — VERIFY + optionally AUGMENT (DO NOT re-add)

`TestValidateFormat` (L1356) ALREADY asserts +body valid (auto/conventional/gitmoji/plain+body → nil)
and invalid (+body, bogus+body, conventional+body+body, conventional+Body → error + contains the mode +
contains "auto, conventional, gitmoji, plain"). **These pass against the current validateFormat**
(load.go:589 — strips case-sensitive "+body", checks base ∈ validFormats; error message
`invalid format %q (valid: <base>[+body], base ∈ auto, conventional, gitmoji, plain)`).

The contract says "update the asserted error substring from 'auto, conventional, gitmoji, plain' to the
new grammar message (e.g. contains 'base' and '+body')". The NEW message STILL contains
"auto, conventional, gitmoji, plain" (via `strings.Join(validFormats, ", ")`) — so the existing
assertion STILL PASSES. OPTIONAL augmentation: add two more `strings.Contains` checks to the invalid
loop — `strings.Contains(err.Error(), "base")` and `strings.Contains(err.Error(), "+body")` — to also
pin the new grammar framing. This is additive (keeps the existing assertion), not a replacement.

**DO NOT** add a second `TestValidateFormat` or re-add the +body cases (they're already there).

---

## 4. THE BYTE-IDENTITY BASELINES (MUST stay GREEN, untouched)

- `TestBuildSystemPrompt_CanonicalExact` (L13) — auto, no suffix. The FR-F1 guarantee that `auto`
  assembly is byte-identical pre/post the +body refactor.
- `TestBuildFallbackPrompt_CanonicalExact` (L244) — auto fallback, no suffix.
These tests assert the EXACT assembled bytes for `auto` (no suffix). The +body changes must NOT alter
the auto path. DO NOT edit these two tests. Running them green is the proof the refactor preserved auto.

---

## 5. SCOPE & COORDINATION

- **ONLY the 3 test files** (load_test.go, format_test.go, system_test.go). NO production code (the
  implementation is S1/S2/S3/S4), NO docs (P1.M2.T1.S2 is docs/cli.md + docs/configuration.md).
- **Parallel P1.M1.T1.S4** (user-facing config/flag text: config.go Format doc, bootstrap.go template,
  root.go --format help) — NOT a test file. No overlap.
- **DO NOT duplicate TestSplitFormat / the TestValidateFormat +body cases** — they exist. Adding a
  second `func TestSplitFormat` = `redeclared in this block` compile error.
- **NO PRD.md / tasks.json / prd_snapshot.md** (read-only).

---

## 6. VALIDATION

- `go test ./internal/config/... ./internal/prompt/...` → GREEN (the primary gate).
- `go test ./internal/prompt/ -run 'SplitFormat|BuildFormatSystemPrompt|BuildSystemPrompt_FormatModes|BuildFallbackPrompt_FormatModes' -v`
  → the new +body subtests + existing pin-tests pass.
- `go test ./internal/config/ -run 'TestValidateFormat' -v` → the existing +body valid/invalid cases pass.
- `go test -race ./internal/config/... ./internal/prompt/...` → green.
- `make test` / `make lint` → green.
- The byte-identity baselines (CanonicalExact) pass UNCHANGED.