name: "P1.M2.T1.S1 — +body format-modifier TEST suite: load_test validation, format_test splitFormat+assembly, system_test auto+body + conventional+body"
description: >
  Test-only. Consumes the +body implementation from P1.M1.T1.S1–S3 (Complete/Ready). Adds the +body test
  coverage across 3 files. CRITICAL: TestSplitFormat (format_test.go:131) and the TestValidateFormat
  +body valid/invalid cases (load_test.go:1356) ALREADY EXIST (written by the implementation subtasks as
  pin-tests) — this task FILLS THE GAPS (verify those exist; do NOT re-create them = duplicate-func
  compile error). The actual new work: (1) format_test.go — add buildFormatSystemPrompt("<base>+body")
  subtests asserting bodyForceDirective present + multilineRuleAllow/Single absent + scaffold retained;
  (2) system_test.go — add an auto+body topology case (examplesIntro + antiReuseProhibition RETAINED +
  bodyForceDirective present + multilineRule* absent) and a conventional+body end-to-end exact-match row
  (scaffold + bodyForceDirective, no multilineRule); optionally a BuildFallbackPrompt auto+body row;
  (3) load_test.go — verify TestValidateFormat's +body cases; optionally augment the invalid-loop's
  error-substring assertion with "base"/"+body" (the existing "auto, conventional, gitmoji, plain"
  assertion still passes — additive, not replacement). The auto byte-identity baselines
  (TestBuildSystemPrompt_CanonicalExact L13, TestBuildFallbackPrompt_CanonicalExact L244) MUST stay GREEN
  unchanged (FR-F1). No production code, no docs (P1.M2.T1.S2 owns docs/cli.md + docs/configuration.md).

---

## Goal

**Feature Goal**: Lock the +body format-modifier behavior (FR-F1/F9, §9.19/§17.8) as a permanent test
contract across the 3 test files: the grammar (validateFormat), the parser (splitFormat), the assembler
(buildFormatSystemPrompt), and the end-to-end prompt builders (BuildSystemPrompt / BuildFallbackPrompt) —
so a future change that drops the body-force directive, leaves a stale multiline rule under +body, or
breaks the auto+body "retain examples + force body" special case fails loudly.

**Deliverable**: EDITS to 3 existing test files (no new files):
1. **internal/prompt/format_test.go** — add `buildFormatSystemPrompt("<base>+body", …)` subtests
   (conventional+body / gitmoji+body / plain+body) to `TestBuildFormatSystemPrompt`.
2. **internal/prompt/system_test.go** — add an auto+body topology case (examples block RETAINED +
   bodyForceDirective + multilineRule* absent) and a conventional+body exact-match end-to-end row;
   optionally a BuildFallbackPrompt auto+body row.
3. **internal/config/load_test.go** — VERIFY `TestValidateFormat`'s +body cases exist; OPTIONALLY
   augment the invalid-loop error-substring assertion with "base"/"+body".

**Success Definition**:
- `go test ./internal/config/... ./internal/prompt/...` is GREEN.
- `buildFormatSystemPrompt("conventional+body", …)` test asserts bodyForceDirective present,
  multilineRuleAllow + multilineRuleSingle absent, conventionalScaffold present — for hasMultiline
  false AND true (hasMultiline is ignored under +body).
- `BuildSystemPrompt(…, "auto+body", …)` test asserts examplesIntro + antiReuseProhibition RETAINED +
  bodyForceDirective present + multilineRule* absent (the auto+body special case).
- `BuildSystemPrompt(…, "conventional+body", …)` exact-match row asserts scaffold + bodyForceDirective,
  no multilineRule.
- `TestSplitFormat` + `TestValidateFormat` +body cases ALREADY PASS (verify; do not duplicate).
- `TestBuildSystemPrompt_CanonicalExact` + `TestBuildFallbackPrompt_CanonicalExact` (auto byte-identity)
  PASS UNCHANGED.
- `make test` / `make lint` green; `gofmt -l` clean.

## User Persona (if applicable)

**Target User**: Stagecoach maintainers / CI (regression guard for the +body feature; no user-facing surface).

**Use Case**: A maintainer refactors `buildFormatSystemPrompt` / `BuildSystemPrompt` / `splitFormat` /
`validateFormat`. These tests fail loudly if the +body directive is dropped, a stale multiline rule
leaks through under +body, the auto+body special case loses its examples block, or the grammar rejects a
valid `<base>+body`.

**Pain Points Addressed**: The +body implementation (S1–S3) shipped pin-tests for the parser (S1's
TestSplitFormat) and the grammar (S3's TestValidateFormat cases) but NOT for the assembler's +body
topology or the end-to-end auto+body/conventional+body assembly — the gaps where a regression
(directive dropped, rule not replaced, examples block wrongly dropped under auto+body) would silently
pass. This task closes those gaps.

## Why

- **FR-F1/F9 / §17.8**: +body is an orthogonal body-forcing modifier — the subject contract is preserved
  (conventional still `type(scope): description`) and the conditional multiline rule is REPLACED by
  bodyForceDirective. The tests pin both halves (scaffold retained + rule replaced) so a refactor can't
  silently break either.
- **The auto+body special case**: `auto+body` is the one mode that RETAINS the examples block (auto keeps
  learning the subject style) while forcing the body. Without a test, a refactor could wrongly drop the
  examples block under auto+body (treating it like conventional+body) and no existing test would catch it.
- **Bounded scope**: 3 test files, additive subtests/rows. No production code, no docs. The
  implementation is landed (S1/S2/S3); this is the test suite that locks it.

## What

**User-visible behavior**: None (test-only). Internally, the +body contract is pinned across grammar →
parser → assembler → end-to-end builder.

**Technical change** (additive test subtests/rows; verbatim in the Blueprint):
1. format_test.go: +body subtests in `TestBuildFormatSystemPrompt` (bodyForceDirective present,
   multilineRule* absent, scaffold retained; hasMultiline ignored).
2. system_test.go: +auto+body topology case + conventional+body exact row (+ optional fallback row).
3. load_test.go: verify TestValidateFormat +body cases; optional error-substring augmentation.

### Success Criteria
- [ ] `go test ./internal/config/... ./internal/prompt/...` green
- [ ] format_test.go: `buildFormatSystemPrompt` +body subtests (conventional/gitmoji/plain+body) present + pass
- [ ] system_test.go: auto+body topology case (examplesIntro + antiReuseProhibition + bodyForceDirective; multilineRule* absent)
- [ ] system_test.go: conventional+body end-to-end exact row (scaffold + bodyForceDirective; no multilineRule)
- [ ] TestSplitFormat + TestValidateFormat +body cases pass UNCHANGED (already exist — verify, don't duplicate)
- [ ] TestBuildSystemPrompt_CanonicalExact + TestBuildFallbackPrompt_CanonicalExact (auto) pass UNCHANGED
- [ ] only the 3 test files modified; no production code; `make test`/`make lint` green

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the GAP MAP (what already exists vs. what to add — the #1 planning fact), the verbatim subtest
code, the exact constant names (bodyForceDirective/multilineRuleAllow/multilineRuleSingle/
conventionalScaffold/examplesIntro/antiReuseProhibition/gitmojiScaffoldInstruction), the function
signatures (buildFormatSystemPrompt/BuildSystemPrompt/BuildFallbackPrompt/splitFormat/validateFormat),
the existing test idioms to mirror (t.Run subtests, exact `==` rows, strings.Contains absence checks),
and the byte-identity baselines that must stay green.

### Documentation & References

```yaml
# MUST READ — the authoritative research (GAP MAP + verbatim subtests + the exists-vs-add fact)
- docfile: plan/021_086bbc57a2b2/P1M2T1S1/research/findings.md
  why: "§0 the GAP MAP (TestSplitFormat + TestValidateFormat +body cases ALREADY EXIST — DO NOT duplicate;
        the real work is format_test +body subtests + system_test auto+body/conventional+body); §1 the
        verbatim format_test +body subtests; §2 the system_test cases; §3 the load_test verify/augment;
        §4 the byte-identity baselines."
  critical: "§0: adding a SECOND `func TestSplitFormat` or re-adding the TestValidateFormat +body cases =
             a DUPLICATE-FUNC COMPILE ERROR. VERIFY they exist first; only ADD the gap tests. §1: hasMultiline
             is IGNORED under +body — loop both false/true to prove the directive wins either way."

# MUST READ — the test-patterns reference (the idioms to mirror)
- docfile: plan/021_086bbc57a2b2/architecture/test_and_docs_surfaces.md
  why: "Documents the exact existing test shapes: TestValidateFormat (table + t.Run, valid/invalid groups);
        TestBuildFormatSystemPrompt (t.Run subtests, exact == + Contains); the system_test CanonicalExact
        (exact ==) + FormatModes_Properties (Contains/absence) + FallbackPrompt_FormatModes idioms."
  critical: "It states the existing auto CanonicalExact tests 'must remain GREEN unchanged — this is the
             FR-F1 guarantee.' Mirror the existing subtest/row idioms for the new +body cases."

# MUST READ — the implementation under test (format.go: the assembler + constants)
- file: internal/prompt/format.go
  why: "splitFormat (@58), buildFormatSystemPrompt (@74), conventionalScaffold (@14), bodyForceDirective
        (@24). buildFormatSystemPrompt calls splitFormat; under +body it emits bodyForceDirective INSTEAD
        of multilineRuleAllow/Single (@84). These are the exact behaviors the new subtests assert."
  pattern: "buildFormatSystemPrompt(format, hasMultiline, subjectTarget): if forceBody → bodyForceDirective;
            else if hasMultiline → multilineRuleAllow; else multilineRuleSingle. +body wins over hasMultiline."
  gotcha: "hasMultiline is IGNORED under +body (the directive unconditionally replaces the rule). The
           conventional+body subtest must loop hasMultiline false AND true to prove this."

# MUST READ — the implementation under test (system.go: BuildSystemPrompt + BuildFallbackPrompt)
- file: internal/prompt/system.go
  why: "BuildSystemPrompt (@202) threads forceBody: under +body emits bodyForceDirective (@231) instead of
        multilineRule*; under AUTO retains examplesIntro + antiReuseProhibition. BuildFallbackPrompt (@178)
        appends bodyForceDirective for auto+body (@186). Constants: examplesIntro (@30), antiReuseProhibition
        (@42), multilineRuleAllow (@52), multilineRuleSingle (@54)."
  pattern: "BuildSystemPrompt(examples, hasMultiline, subjectTarget, format, locale): auto path =
            preamble + maturePromptHeader + examples + antiReuse + rule + target; non-auto = preamble +
            scaffold + rule + target; +body replaces 'rule' with bodyForceDirective in BOTH."
  critical: "auto+body is the SPECIAL CASE: it RETAINS the examples block (examplesIntro + antiReuse) —
             unlike conventional+body/gitmoji+body/plain+body which REPLACE it. The auto+body test MUST
             assert examplesIntro + antiReuseProhibition are present."

# MUST READ — the implementation under test (load.go: validateFormat grammar)
- file: internal/config/load.go
  why: "validateFormat (@589): strips case-sensitive '+body', checks base ∈ validFormats; error message
        `invalid format %q (valid: <base>[+body], base ∈ auto, conventional, gitmoji, plain)`. The message
        STILL contains 'auto, conventional, gitmoji, plain' (via Join) — so the existing TestValidateFormat
        assertion still passes; the +body valid/invalid cases there are DONE."
  gotcha: "The contract says 'update the asserted substring' — but the new message still contains the old
           substring. The existing assertion passes; augmentation (add 'base' + '+body' Contains checks) is
           OPTIONAL and ADDITIVE, not a replacement."

# MUST READ — the test files being edited (mirror their idioms; confirm the gaps)
- file: internal/prompt/format_test.go
  why: "TestSplitFormat (@131) ALREADY EXISTS (full table — DO NOT re-create). TestBuildFormatSystemPrompt
        (@91) has conventional/plain topology subtests but NO +body — ADD the +body subtests here (or append
        to it). Mirror the t.Run + strings.Contains/absence idiom."
- file: internal/prompt/system_test.go
  why: "TestBuildSystemPrompt_FormatModes_Properties (@383) — ADD the auto+body topology case (or a new
        test). TestBuildSystemPrompt_FormatModes_CanonicalExact (@331) — ADD a conventional+body exact row.
        TestBuildFallbackPrompt_FormatModes_CanonicalExact (@450) — OPTIONALLY ADD an auto+body row.
        TestBuildSystemPrompt_CanonicalExact (@13) + TestBuildFallbackPrompt_CanonicalExact (@244) — auto
        byte-identity baselines; DO NOT EDIT (must stay green)."
- file: internal/config/load_test.go
  why: "TestValidateFormat (@1356) ALREADY has the 4 +body valid + 4 invalid cases + the grammar assertion
        (written by S3). VERIFY present + green. OPTIONAL: augment the invalid loop with
        strings.Contains(err, 'base') + strings.Contains(err, '+body')."

# CONTEXT — the +body spec (what the tests encode)
- docfile: plan/021_086bbc57a2b2/prd_snapshot.md
  why: "§9.19 FR-F1 (the <base>[+body] grammar) + FR-F9 (the +body modifier: unconditional body directive
        REPLACES the conditional multi-line rule; subject contract preserved). §17.8 (the prompt-assembly
        detail: 'A +body variant replaces the <multi-line rule> block entirely with an unconditional
        directive'). These define the invariants the tests pin."
  section: "§9.19 FR-F1, FR-F9; §17.8 Body forcing"

# CONTEXT — the parallel sibling (NO overlap)
- docfile: plan/021_086bbc57a2b2/P1M1T1S4/PRP.md
  why: "Parallel P1.M1.T1.S4 edits config.go/bootstrap.go/root.go (user-facing text). NOT a test file. No
        overlap with this test-only task. (S4 is in-flight; line numbers in production files may drift —
        but the test files are mine alone.)"
```

### Current Codebase tree (relevant slice)

```bash
internal/prompt/
  format.go           # READ-ONLY — splitFormat, buildFormatSystemPrompt, conventionalScaffold, bodyForceDirective (the code under test)
  system.go           # READ-ONLY — BuildSystemPrompt, BuildFallbackPrompt, examplesIntro, antiReuseProhibition, multilineRule* (under test)
  format_test.go      # EDIT — +buildFormatSystemPrompt +body subtests (TestSplitFormat already exists — verify, don't dupe)
  system_test.go      # EDIT — +auto+body topology + conventional+body exact row (+ optional fallback); CanonicalExact baselines UNCHANGED
internal/config/
  load.go             # READ-ONLY — validateFormat (the grammar under test)
  load_test.go        # EDIT — verify TestValidateFormat +body cases; optional error-substring augmentation
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (no new files):
internal/prompt/format_test.go   # +buildFormatSystemPrompt +body subtests
internal/prompt/system_test.go   # +auto+body topology case + conventional+body exact row (+ optional fallback row)
internal/config/load_test.go     # verify TestValidateFormat +body cases (optional: +augment error-substring asserts)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (TestSplitFormat + TestValidateFormat +body cases ALREADY EXIST): the implementation subtasks
//   S1 (splitFormat) and S3 (validateFormat) each shipped a pin-test — TestSplitFormat (format_test.go:131,
//   full table) and the TestValidateFormat +body valid/invalid cases (load_test.go:1356). Adding a SECOND
//   `func TestSplitFormat` or re-adding those cases = a DUPLICATE-FUNCTION COMPILE ERROR. VERIFY they
//   exist first (grep); only ADD the gap tests (buildFormatSystemPrompt +body, system_test +body).

// CRITICAL (hasMultiline is IGNORED under +body): buildFormatSystemPrompt / BuildSystemPrompt emit
//   bodyForceDirective unconditionally under +body, REGARDLESS of hasMultiline. The conventional+body
//   subtest must loop hasMultiline=false AND true to prove the directive wins either way (a regression
//   that keyed the directive off hasMultiline would pass a single-value test).

// CRITICAL (auto+body RETAINS the examples block — the special case): unlike conventional+body /
//   gitmoji+body / plain+body (which REPLACE the examples block with a scaffold), auto+body KEEPS
//   examplesIntro + antiReuseProhibition (auto keeps learning the subject style) AND adds bodyForceDirective.
//   The auto+body test MUST assert examplesIntro + antiReuseProhibition are PRESENT — a refactor that
//   wrongly drops them under auto+body is the exact regression this test catches.

// GOTCHA (the exact-match want string joining): the conventional+body exact row's `want` is
//   `promptPreamble + "\n\n" + conventionalScaffold + "\n\n" + bodyForceDirective + "\n" + targetLine`.
//   The separator pattern (blank line between sections) mirrors the existing conventional row. RUN the
//   test once; if the want string's separators differ from the actual output, adjust to match (the
//   structural fact — scaffold + bodyForceDirective + target, NO multilineRule — is what matters).

// GOTCHA (the byte-identity baselines must stay GREEN): TestBuildSystemPrompt_CanonicalExact (L13) +
//   TestBuildFallbackPrompt_CanonicalExact (L244) assert the EXACT auto (no-suffix) bytes. The +body
//   refactor must NOT alter the auto path. DO NOT edit these tests; their passing unchanged IS the proof.

// GOTCHA (TestValidateFormat error message still contains the old substring): validateFormat's error is
//   `invalid format %q (valid: <base>[+body], base ∈ auto, conventional, gitmoji, plain)` — the
//   "auto, conventional, gitmoji, plain" substring is STILL THERE (via Join). The existing assertion
//   passes; the contract's "update the substring" is satisfied by ADDITIVE checks ("base", "+body"), not
//   by removing the bases check.

// GOTCHA (constants are unexported, same-package tests): bodyForceDirective, multilineRuleAllow,
//   multilineRuleSingle, conventionalScaffold, examplesIntro, antiReuseProhibition, gitmojiScaffoldInstruction,
//   promptPreamble are unexported consts in package prompt — the test files are `package prompt` (internal),
//   so they're in scope. load_test.go is `package config` — validFormats + validateFormat are in scope there.
```

## Implementation Blueprint

### Data models and structure
None. Test-only: subtests + table rows. No new types, no production code.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: VERIFY the already-existing pin-tests (DO NOT duplicate)
  - RUN: grep -n 'func TestSplitFormat' internal/prompt/format_test.go   → expect 1 hit (L131).
  - RUN: grep -n 'auto+body\|conventional+body\|gitmoji+body\|plain+body\|+body\b' internal/config/load_test.go
    → expect the TestValidateFormat valid+invalid +body cases (L1356). CONFIRM present.
  - If EITHER is missing (it isn't, per research), THAT is the gap to fill. Otherwise proceed to Task 1+.

Task 1: EDIT internal/prompt/format_test.go — add buildFormatSystemPrompt +body subtests
  - In TestBuildFormatSystemPrompt (@91), APPEND t.Run subtests (mirror the existing t.Run idiom):
      "conventional+body forces the body directive (rule replaced)" — loop hasMultiline false+true; assert
        bodyForceDirective present; multilineRuleAllow absent; multilineRuleSingle absent;
        conventionalScaffold present.
      "gitmoji+body forces the body directive" — assert bodyForceDirective present; multilineRule* absent;
        gitmojiScaffoldInstruction present.
      "plain+body forces the body directive (no scaffold)" — assert bodyForceDirective present;
        multilineRule* absent. (plain has no scaffold body — nothing scaffold-ish to assert present.)
  - (Verbatim code in research/findings.md §1.)
  - DO NOT add a second TestSplitFormat (it exists @131).

Task 2: EDIT internal/prompt/system_test.go — add the auto+body + conventional+body cases
  - (2a) auto+body topology: ADD a t.Run case (in TestBuildSystemPrompt_FormatModes_Properties @383, or a
         new test) — BuildSystemPrompt(examples, true, 50, "auto+body", ""); assert examplesIntro present,
         antiReuseProhibition present, bodyForceDirective present, multilineRuleAllow absent,
         multilineRuleSingle absent. (Verbatim in findings §2a.)
  - (2b) conventional+body end-to-end exact: ADD a row to TestBuildSystemPrompt_FormatModes_CanonicalExact
         (@331) — format="conventional+body", want = promptPreamble + "\n\n" + conventionalScaffold +
         "\n\n" + bodyForceDirective + "\n" + "Target ~50 characters for the subject line.". (Run once;
         adjust separators to match actual output if needed.) (Verbatim in findings §2b.)
  - (2c) OPTIONAL: BuildFallbackPrompt auto+body — ADD a row to TestBuildFallbackPrompt_FormatModes_
         CanonicalExact (@450). Read BuildFallbackPrompt's §17.2 assembly for the exact want string.
  - DO NOT edit TestBuildSystemPrompt_CanonicalExact (@13) or TestBuildFallbackPrompt_CanonicalExact (@244).

Task 3: VERIFY + OPTIONALLY AUGMENT internal/config/load_test.go
  - RUN: go test ./internal/config/ -run TestValidateFormat -v → CONFIRM the +body valid (nil) + invalid
    (error) cases pass. (They exist @1356.)
  - OPTIONAL: in TestValidateFormat's invalid loop, ADD two Contains checks:
      if !strings.Contains(err.Error(), "base") { t.Errorf(...) }
      if !strings.Contains(err.Error(), "+body") { t.Errorf(...) }
    (ADDITIVE — keep the existing "auto, conventional, gitmoji, plain" check; the new message contains both.)
  - DO NOT re-add the +body valid/invalid cases (they're there).

Task 4: VERIFY — focused + full tests, race, lint, the byte-identity baselines
  - go test ./internal/config/... ./internal/prompt/... -v
  - go test ./internal/prompt/ -run 'SplitFormat|BuildFormatSystemPrompt|BuildSystemPrompt_FormatModes|BuildFallbackPrompt_FormatModes|CanonicalExact' -v
  - go test -race ./internal/config/... ./internal/prompt/...
  - make test ; make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the +body assembler subtest (hasMultiline ignored — loop both)
t.Run("conventional+body forces the body directive (rule replaced)", func(t *testing.T) {
	for _, hm := range []bool{false, true} { // hasMultiline IGNORED under +body — prove both
		got := buildFormatSystemPrompt("conventional+body", hm, 50)
		if !strings.Contains(got, bodyForceDirective)  { t.Error("conventional+body must contain bodyForceDirective") }
		if strings.Contains(got, multilineRuleAllow)   { t.Error("conventional+body must NOT contain multilineRuleAllow") }
		if strings.Contains(got, multilineRuleSingle)  { t.Error("conventional+body must NOT contain multilineRuleSingle") }
		if !strings.Contains(got, conventionalScaffold){ t.Error("conventional+body must retain conventionalScaffold") }
	}
})

// PATTERN: the auto+body SPECIAL CASE (examples block RETAINED + body forced)
t.Run("auto+body retains examples block AND forces body", func(t *testing.T) {
	p := BuildSystemPrompt([]string{"feat: a", "fix: b"}, true, 50, "auto+body", "")
	if !strings.Contains(p, examplesIntro)        { t.Error("auto+body must RETAIN examplesIntro") }
	if !strings.Contains(p, antiReuseProhibition) { t.Error("auto+body must RETAIN antiReuseProhibition") }
	if !strings.Contains(p, bodyForceDirective)   { t.Error("auto+body must contain bodyForceDirective") }
	if strings.Contains(p, multilineRuleAllow)    { t.Error("auto+body must NOT contain multilineRuleAllow") }
	if strings.Contains(p, multilineRuleSingle)   { t.Error("auto+body must NOT contain multilineRuleSingle") }
})

// PATTERN: the conventional+body exact row (scaffold + directive, NO rule)
{ name: "conventional+body, no locale", format: "conventional+body", locale: "",
  want: promptPreamble + "\n\n" + conventionalScaffold + "\n\n" + bodyForceDirective + "\n" +
      "Target ~50 characters for the subject line." },
```

### Integration Points

```yaml
NO production code / new types / config / routes / deps. EDITS to 3 existing test files.

TEST FILES:
  - internal/prompt/format_test.go: +buildFormatSystemPrompt +body subtests (TestSplitFormat unchanged).
  - internal/prompt/system_test.go: +auto+body topology + conventional+body exact row (+ optional fallback);
    CanonicalExact baselines UNCHANGED.
  - internal/config/load_test.go: verify TestValidateFormat +body cases; optional error-substring augment.

SCOPE FENCES: NO production code (format.go/system.go/load.go — FROZEN, under test); NO docs
  (P1.M2.T1.S2 owns docs/cli.md + docs/configuration.md); NO P1.M2.T2.S1 (README sweep); NO PRD/tasks.json.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build + vet (the test files compile).
go build ./...
go vet ./internal/config/... ./internal/prompt/...

# Format.
gofmt -l internal/prompt/format_test.go internal/prompt/system_test.go internal/config/load_test.go
# Expected: empty. If listed: gofmt -w the file(s).

# Lint.
make lint
# Expected: zero errors.

# Scope guard: only the 3 test files changed.
git diff --name-only
# Expected: internal/prompt/format_test.go  internal/prompt/system_test.go  internal/config/load_test.go (only).
```

### Level 2: Unit Tests (Component Validation)

```bash
# THE primary gate — both packages green.
go test ./internal/config/... ./internal/prompt/...
# Expected: ok (all +body cases pass; existing tests unchanged).

# Focused: the new +body subtests + the existing pin-tests.
go test ./internal/prompt/ -run 'SplitFormat|BuildFormatSystemPrompt|BuildSystemPrompt_FormatModes|BuildFallbackPrompt_FormatModes' -v
go test ./internal/config/ -run 'TestValidateFormat' -v
# Expected: the new +body subtests PASS; TestSplitFormat + TestValidateFormat +body cases PASS (already existed).

# The byte-identity baselines (MUST stay green unchanged — FR-F1).
go test ./internal/prompt/ -run 'TestBuildSystemPrompt_CanonicalExact|TestBuildFallbackPrompt_CanonicalExact' -v
# Expected: PASS (auto, no suffix — byte-identical pre/post refactor).

# Race + full repo.
go test -race ./internal/config/... ./internal/prompt/...
make test
# Expected: green.
```

### Level 3: Integration Testing (System Validation)

```bash
# Test-only task — no integration/e2e/HTTP surface. The unit tests ARE the contract. A full prompt-output
# e2e is out of scope (the +body behavior is a pure string-assembly concern, fully covered by the unit tests).
go test ./internal/config/... ./internal/prompt/... -v
# Expected: green.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1: TestSplitFormat exists EXACTLY ONCE (not duplicated by this task).
grep -c 'func TestSplitFormat' internal/prompt/format_test.go
# Expected: 1. (If 2 → duplicate-func compile error; remove the duplicate.)

# Grep guard 2: buildFormatSystemPrompt +body subtests were added.
grep -c 'conventional+body\|gitmoji+body\|plain+body' internal/prompt/format_test.go
# Expected: ≥3 (the new subtests). (TestSplitFormat also mentions them — that's fine.)

# Grep guard 3: the auto+body topology case + conventional+body row were added to system_test.go.
grep -c 'auto+body\|conventional+body' internal/prompt/system_test.go
# Expected: ≥2 (the new cases).

# Grep guard 4: the auto+body test asserts examplesIntro RETAINED (the special case).
grep -A2 'auto+body retains' internal/prompt/system_test.go | grep -c 'examplesIntro\|antiReuseProhibition'
# Expected: ≥1 (the retention assertion is present).

# Grep guard 5: the byte-identity baselines are UNCHANGED.
git diff internal/prompt/system_test.go | grep -E '^[+-]' | grep -iE 'CanonicalExact|TestBuildSystemPrompt_CanonicalExact \(L13|TestBuildFallbackPrompt_CanonicalExact \(L244'
# Expected: empty (those two funcs are not edited). (They may appear in +/- context lines only if adjacent
#           to an insertion; the guard checks no func signature/body line of THEM changed.)

# Grep guard 6: NO production code changed.
git diff --name-only | grep -vE '_test\.go$'
# Expected: empty (only *_test.go files modified).

# Regression: the existing prompt + config tests stay green.
go test ./internal/prompt/ ./internal/config/ -v
# Expected: all PASS.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `go vet ./internal/config/... ./internal/prompt/...` clean
- [ ] `gofmt -l` empty on the 3 test files
- [ ] `go test ./internal/config/... ./internal/prompt/...` green
- [ ] `go test -race ./internal/config/... ./internal/prompt/...` green
- [ ] `make test` + `make lint` pass

### Feature Validation
- [ ] format_test.go: buildFormatSystemPrompt +body subtests (conventional/gitmoji/plain+body) present + pass; hasMultiline-looped
- [ ] system_test.go: auto+body topology (examplesIntro + antiReuseProhibition + bodyForceDirective; multilineRule* absent)
- [ ] system_test.go: conventional+body exact row (scaffold + bodyForceDirective; no multilineRule)
- [ ] TestSplitFormat + TestValidateFormat +body cases pass (already existed — verified, not duplicated)

### Scope-Boundary Validation
- [ ] `git diff --name-only` == only {format_test.go, system_test.go, load_test.go}
- [ ] NO production code change (format.go/system.go/load.go — under test, FROZEN)
- [ ] NO docs (P1.M2.T1.S2); NO README (P1.M2.T2.S1); NO PRD/tasks.json
- [ ] TestSplitFormat exists exactly ONCE (not duplicated); TestValidateFormat +body cases not re-added
- [ ] TestBuildSystemPrompt_CanonicalExact + TestBuildFallbackPrompt_CanonicalExact (auto) UNCHANGED + green

### Code Quality & Docs
- [ ] New subtests mirror the existing t.Run / exact-row / Contains-absence idioms
- [ ] The auto+body special case (retain examples + force body) is explicitly asserted
- [ ] hasMultiline-ignored-under-+body is proven (loop both values)

---

## Anti-Patterns to Avoid

- ❌ Don't re-create `TestSplitFormat` or re-add the `TestValidateFormat` +body cases. They ALREADY EXIST
  (S1 wrote TestSplitFormat @131; S3 wrote the +body valid/invalid cases @1356). Adding a second
  `func TestSplitFormat` is a duplicate-function COMPILE ERROR. VERIFY they exist first (grep); only ADD
  the gap tests.
- ❌ Don't test `buildFormatSystemPrompt("<base>+body")` with a single hasMultiline value. hasMultiline is
  IGNORED under +body (the directive unconditionally replaces the rule). Loop `false` AND `true` so a
  regression that wrongly keyed the directive off hasMultiline fails the test.
- ❌ Don't forget the auto+body SPECIAL CASE. Unlike conventional+body (which REPLACES the examples block
  with a scaffold), `auto+body` RETAINS examplesIntro + antiReuseProhibition (auto keeps learning the
  subject style) AND adds bodyForceDirective. The auto+body test MUST assert the examples block is present
  — a refactor that wrongly drops it is the exact regression this catches.
- ❌ Don't edit the byte-identity baselines. `TestBuildSystemPrompt_CanonicalExact` (L13) +
  `TestBuildFallbackPrompt_CanonicalExact` (L244) assert the EXACT auto (no-suffix) bytes — the FR-F1
  guarantee that the +body refactor didn't alter the auto path. Their passing UNCHANGED is the proof; do
  not touch them.
- ❌ Don't "replace" the TestValidateFormat error-substring assertion. validateFormat's error still
  contains "auto, conventional, gitmoji, plain" (via Join) — the existing assertion passes. The contract's
  "update the substring" is satisfied by ADDITIVE checks ("base", "+body"), not by removing the bases check.
- ❌ Don't hardcode the conventional+body exact-row `want` string blindly. The joining (blank-line
  separators) mirrors the existing conventional row; RUN the test once and adjust the separators to match
  the actual output if the first run mismatches. The structural invariant (scaffold + bodyForceDirective +
  target, NO multilineRule) is what must hold.
- ❌ Don't edit production code (format.go/system.go/load.go). They are FROZEN — UNDER TEST. If a test
  fails because the implementation is wrong, that's a finding against S1/S2/S3 (a different task), not a
  license to change the implementation here. (Per research, the implementation is correct; the tests should pass.)
- ❌ Don't touch docs. P1.M2.T1.S2 owns docs/cli.md + docs/configuration.md; P1.M2.T2.S1 owns the README
  sweep. This task is the 3 test files ONLY.
- ❌ Don't add the gitmojiScaffoldInstruction / RenderGitmojiTable() assertions to the plain+body subtest.
  plain has NO scaffold body (`formatScaffoldBody("plain")==""`). The plain+body subtest asserts
  bodyForceDirective present + multilineRule* absent — nothing scaffold-ish.

---

## Confidence Score: 9/10

The GAP MAP (what exists vs. what to add) is the load-bearing fact and is verified by grep against the
current test files. The verbatim subtest code, the exact constant names, the function signatures, the
existing test idioms to mirror, the auto+body special case, the hasMultiline-ignored-under-+body rule,
and the byte-identity baselines-that-must-stay-green are all spelled out. The one residual (not a full
10) is the exact separator joining in the conventional+body exact-match `want` string (the structural
invariant is certain; the precise `\n\n` vs `\n` between sections is verified by running the test once
and adjusting — a 1-shot fix, not a guess). No production code, no new types; the implementation is landed
and correct. One-pass success is highly likely.