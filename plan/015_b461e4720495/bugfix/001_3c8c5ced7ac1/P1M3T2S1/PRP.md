name: "P1.M3.T2.S1 — Remove 'stagecoach:' prefix from ErrEmptyMessage literal + pinning test (Issue 5)"
description: >
  A ONE-LINE production fix for Issue 5 (the doubled "stagecoach:" prefix in the --edit / hooks
  empty-message abort). `internal/generate/finalize.go:45` defines `var ErrEmptyMessage = errors.New(
  "stagecoach: empty commit message — aborted")`; `cmd/stagecoach/main.go:67` prints every error as
  `fmt.Fprintf(os.Stderr, "stagecoach: %v\n", err)`, so the user sees `stagecoach: stagecoach: empty
  commit message — aborted`. The fix: drop the `"stagecoach: "` prefix from the literal (line 45 ONLY),
  yielding `errors.New("empty commit message — aborted")` → single-prefixed output. The sentinel's
  IDENTITY is preserved (it's the same `*errors.errorString` pointer — only its string content changes),
  so EVERY `errors.Is(err, ErrEmptyMessage)` check (exitcode.go:65 → exit 1; all 6+ test assertions) still
  works unchanged. The sentinel propagates BARE at every site (finalize.go:118, generate.go:486/517,
  decompose.go:409, message.go:213/254, pkg/stagecoach/stagecoach.go:709/754) — none wrap it with a
  prefix — so fixing the ONE literal fixes BOTH the --edit path and the hooks-empty-message path
  simultaneously. CRITICAL RESEARCH RESULT: NO existing test asserts on the literal STRING — every test
  uses `errors.Is` (sentinel identity), which is invariant under the edit (exhaustive grep proof in
  findings §3). So the contract's "update test assertions" step is a NO-OP (there are none to update).
  The rigorous move is to ADD one tiny pinning test (`TestErrEmptyMessage_NoStagecoachPrefix`) in
  generate_test.go that asserts the literal equals `"empty commit message — aborted"` (no prefix) — this
  PROVES the fix and LOCKS it against regression. The godoc on finalize.go:41-44 ALREADY describes the
  un-prefixed message (it documents the user-facing string after main.go adds the prefix) — NO doc change
  needed. EM-DASH GOTCHA: the literal uses a Unicode EM DASH `—` (U+2014), not a hyphen; the edit MUST
  preserve it exactly. NOT in scope: main.go (the prefix site — correct as-is), exitcode.go, generate.go,
  decompose/*, pkg/stagecoach/*, docs (P1.M4.T1 — contract: "DOCS: none"), the parallel P1.M3.T1.S2
  (Issue 4, internal/git/tokengate.go — zero overlap), P1.M3.T3.S1 (Issue 6, default_action.go — zero
  overlap).

---

## Goal

**Feature Goal**: Eliminate the doubled `"stagecoach:"` prefix in the --edit / hooks empty-message abort
output (Issue 5). Today the user sees `stagecoach: stagecoach: empty commit message — aborted` because the
`ErrEmptyMessage` sentinel literal includes its own `"stagecoach: "` prefix AND `main.go:67` prepends
`"stagecoach: "` to every printed error. The fix removes the prefix from the sentinel literal so the final
output is the correct single-prefixed `stagecoach: empty commit message — aborted`, WITHOUT changing the
sentinel's identity (so `errors.Is` and the exit-1 mapping still work).

**Deliverable**:
1. `internal/generate/finalize.go` — change line 45's literal from `"stagecoach: empty commit message — aborted"` to `"empty commit message — aborted"` (preserve the em-dash `—`).
2. `internal/generate/generate_test.go` — add `TestErrEmptyMessage_NoStagecoachPrefix` (a ~6-line unit test pinning the literal has no `"stagecoach: "` prefix; proves the fix + locks regression). (NO existing test needs updating — all use `errors.Is`.)

**Success Definition**:
- `ErrEmptyMessage.Error()` returns `"empty commit message — aborted"` (no `"stagecoach: "` prefix); the
  em-dash `—` is preserved.
- The sentinel IDENTITY is unchanged: `errors.Is(err, ErrEmptyMessage)` still returns true at every site,
  and `exitcode.For(ErrEmptyMessage)` still returns `Error` (exit 1) — verified by the green exitcode tests.
- The new `TestErrEmptyMessage_NoStagecoachPrefix` passes (and would fail if the prefix were re-added).
- ALL existing `errors.Is(err, ErrEmptyMessage)` / `errors.Is(err, generate.ErrEmptyMessage)` tests stay
  GREEN UNCHANGED (generate_test.go:882, hooks_freeze_test.go:198, message_test.go:546,
  stagecoach_test.go:1681/1703, exitcode_test.go:25) — they assert identity, not the string.
- `go build ./...` clean; `gofmt -l` empty; `go vet ./internal/generate/...` clean;
  `go test ./internal/generate/ ./internal/decompose/ -v` green (the contract's command);
  `make test` + `make lint` clean.
- Scope: `git status --porcelain` == finalize.go + generate_test.go. NO edit to main.go, exitcode.go,
  generate.go, decompose/*, pkg/stagecoach/*, or any PRD/task file.

## User Persona (if applicable)

**Target User**: A developer using `stagecoach --edit` (the §9.22 FR-E1 editor gate) who empties the
message file to abort, OR a user whose `prepare-commit-msg` / `commit-msg` hook empties the message file
(a rejection pattern). Both hit the `ErrEmptyMessage` abort path.

**Use Case**: User runs `stagecoach --edit`, clears the message in the editor, saves+quits → stagecoach
aborts (exit 1). Today the stderr line is confusingly double-prefixed
(`stagecoach: stagecoach: empty commit message — aborted`); after the fix it reads naturally
(`stagecoach: empty commit message — aborted`).

**User Journey**: `stagecoach --edit` → editor opens → user empties it → save/quit → EditMessage returns
ErrEmptyMessage → generate/decompose propagates it BARE → main.go:67 prints `"stagecoach: %v"` → single
clean prefix.

**Pain Points Addressed**: Issue 5 — a cosmetic-but-noticeable defect (doubled prefix) on a user-visible
abort message. Low severity (Minor) but trivial to fix and the abort path is one a careful user hits often.

## Why

- **Issue 5 (Minor)**: the doubled prefix is a polish defect on a user-facing error string. The fix is a
  single literal edit with zero behavioral risk (sentinel identity preserved).
- **The fix is safe by construction**: the entire codebase reaches `ErrEmptyMessage` via `errors.Is`
  (identity comparison), never via a string match. Editing the literal's text cannot break any `errors.Is`
  check. `exitcode.For`'s `errors.Is(err, generate.ErrEmptyMessage) → Error` (exit 1) is invariant.
- **One fix, two paths**: both the --edit abort (finalize.go:118 → generate.go:486 / decompose.go:409) and
  the hooks-empty-message guard (generate.go:517 / message.go:254) return the SAME sentinel BARE, so
  editing the one literal fixes every abort path simultaneously — no per-path edit needed.
- **main.go:67 is correct as-is**: it's the CLI's single error-printing site, designed to prefix every
  error uniformly. The bug is the sentinel carrying its OWN prefix, not main.go. Fix the sentinel, leave
  main.go alone (the prefix-once discipline lives in one place).

## What

**User-visible behavior**: the --edit empty-message abort and the hooks-empty-message abort now print
`stagecoach: empty commit message — aborted` (single prefix) instead of
`stagecoach: stagecoach: empty commit message — aborted` (double). Exit code is unchanged (1). No other
behavior changes.

**Technical change**: one literal edit (drop `"stagecmd: "` → drop `"stagecoach: "` prefix) + one pinning test.

### Success Criteria
- [ ] `internal/generate/finalize.go:45` reads `var ErrEmptyMessage = errors.New("empty commit message — aborted")`
      (no `"stagecoach: "` prefix; em-dash `—` preserved).
- [ ] `internal/generate/generate_test.go` adds `TestErrEmptyMessage_NoStagecoachPrefix` asserting
      `ErrEmptyMessage.Error() == "empty commit message — aborted"` AND that it does NOT start with
      `"stagecoach: "`.
- [ ] NO existing test is modified (all use `errors.Is`; the literal edit is identity-preserving).
- [ ] `errors.Is(err, ErrEmptyMessage)` still true at every site; `exitcode.For(ErrEmptyMessage) == Error` (1).
- [ ] `go build ./...` clean; `go vet ./internal/generate/...` clean; `gofmt -l` empty on the 2 files.
- [ ] `go test ./internal/generate/ ./internal/decompose/ -v` green (contract command) + new pinning test passes.
- [ ] `make test` + `make lint` clean.
- [ ] `git status --porcelain` == `internal/generate/finalize.go` + `internal/generate/generate_test.go`.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact literal to edit (quoted, incl. the em-dash gotcha), the exact replacement, the
identity-preservation safety argument (why `errors.Is` is invariant), the full propagation-path table
(every site returns the sentinel BARE — no second prefix to chase), the exhaustive proof that NO test
asserts on the literal string (so "update test assertions" is a no-op), the ready-to-paste pinning test,
the fact the godoc is already correct (no doc edit), the scope fences (main.go/exitcode.go untouched;
parallel Issue 4 / Issue 4 have zero overlap), and 6 grep guards.

### Documentation & References

```yaml
# MUST READ — the authoritative Issue 5 spec (the fix + the propagation path + the test note)
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/minor_fixes.md
  section: "Issue 5: Doubled \"stagecoach:\" Prefix in --edit Abort"
  why: "Gives the exact bug (finalize.go:45 literal + main.go:67 'stagecoach: %v' = double prefix), the
        exact fix (drop the prefix from the literal → 'empty commit message — aborted'), the full
        propagation path (finalize.go:118 → generate.go:486 → decompose.go:409 → message.go:254 →
        exitcode.go:65), and the note that the sentinel identity is preserved (errors.Is still works)."
  critical: "The fix is ONE literal at finalize.go:45. main.go:67 is the prefix site and is CORRECT — do
             NOT edit it. The architecture doc's 'any assertion on the literal string must be updated' is
             defensive — findings §3 proves NONE exist (all tests use errors.Is)."

# MUST READ — codebase-specific findings for THIS item (the no-test-asserts-literal proof + pinning test)
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M3T2S1/research/findings.md
  why: "§0-1 the bug + the one-literal fix + the EM-DASH gotcha (U+2014, not a hyphen); §2 the propagation
        table (all BARE — no second prefix); §3 the EXHAUSTIVE proof NO test asserts on the literal string
        (every test uses errors.Is → the 'update test assertions' step is a NO-OP); §4 the ready-to-paste
        pinning test; §5 the godoc is ALREADY correct (no doc edit); §6 scope fences; §7 validation cmds;
        §8 the errors.Is-invariance safety argument."
  critical: "There is NO test to UPDATE. Do not hunt for a string assertion — there are none. ADD the
             pinning test instead (§4). The em-dash MUST be preserved exactly in the edit."

# MUST READ — the file being edited (the literal + the godoc + the EditMessage return site)
- file: internal/generate/finalize.go
  why: "Line 45: var ErrEmptyMessage = errors.New(\"stagecoach: empty commit message — aborted\") — the ONE
        line to edit (drop \"stagecoach: \"). Line 118: return \"\", ErrEmptyMessage (the --edit abort
        site — unchanged). Lines 41-44: the godoc ALREADY says 'maps it to exit 1 with \"empty commit
        message — aborted\"' (no prefix) — it documents the user-facing string, so it's already correct;
        DO NOT edit the godoc."
  pattern: "errors.New sentinel; identity-based (errors.Is); the literal is the user-visible text LESS the
            main.go prefix."
  gotcha: "The literal uses an EM DASH — (U+2014, UTF-8 E2 80 94), NOT a hyphen. The edit's oldText AND
           newText must carry the exact same em-dash byte. A hyphen would fail to match AND change the text."

# CONTEXT — the prefix site (main.go:67) — READ-ONLY (correct as-is; do NOT edit)
- file: cmd/stagecoach/main.go
  why: "Line 67: fmt.Fprintf(os.Stderr, \"stagecoach: %v\\n\", err) — the CLI's single error printer. It
        prepends 'stagecoach: ' to EVERY non-empty err. This is the CORRECT behavior (uniform prefix); the
        bug is the sentinel duplicating it. Do NOT edit main.go — fixing the sentinel fixes the output."
  critical: "main.go is out of scope. If you find yourself editing main.go to special-case ErrEmptyMessage,
             stop — that breaks the uniform-prefix discipline. Fix the one literal in finalize.go."

# CONTEXT — the exit-code mapper (verify errors.Is → exit 1 is invariant)
- file: internal/exitcode/exitcode.go
  why: "Line 65: if errors.Is(err, generate.ErrEmptyMessage) { return Error } → exit 1. Identity-based
        (errors.Is compares the pointer, not the string) → UNCHANGED by the literal edit. exitcode_test.go:25
        {'ErrEmptyMessage → 1', generate.ErrEmptyMessage, Error} passes the sentinel VALUE → stays GREEN."
  critical: "Do NOT edit exitcode.go. The errors.Is check is invariant under the literal edit (findings §8)."

# CONTEXT — the existing edit-abort test (the errors.Is pattern the pinning test sits beside)
- file: internal/generate/generate_test.go
  why: "Line 858: t.Run('fake editor empties message → ErrEmptyMessage', …) — line 882 asserts
        errors.Is(err, ErrEmptyMessage) (IDENTITY, not string) → stays GREEN. ADD the new
        TestErrEmptyMessage_NoStagecoachPrefix near here (it tests the package-level sentinel directly,
        no harness). Confirm 'strings' is imported (the new test uses strings.HasPrefix); add it if missing."
  pattern: "errors.Is(err, ErrEmptyMessage) is the established assertion form for this sentinel — identity,
            invariant under the literal edit. The new pinning test is the ONLY place that asserts the STRING."

# CONTEXT — PRD §9.22 FR-E1 (the --edit abort the sentinel signals) + Issue 5 severity (Minor)
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/prd_snapshot.md
  section: "Overview (Issue 5 is one of 3 Minor issues) + §9.22 FR-E1 (--edit empty-message abort)"
  why: "FR-E1: an empty edited message aborts (exit 1, NOT a rescue). Issue 5 is the doubled-prefix polish
        defect on that abort's user-visible string. Confirms exit code is unchanged (1) — only the text."
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  finalize.go          # EDIT — line 45 literal: drop "stagecoach: " prefix (preserve em-dash)
  generate_test.go     # EDIT — +TestErrEmptyMessage_NoStagecoachPrefix (pinning test; ~6 lines)
  generate.go          # READ-ONLY — :486/:517 propagate the sentinel BARE (no edit; no second prefix)
  hooks_freeze_test.go # READ-ONLY — :198 errors.Is(err, generate.ErrEmptyMessage) stays GREEN
internal/decompose/
  message.go           # READ-ONLY — :213/:254 propagate generate.ErrEmptyMessage BARE
  decompose.go         # READ-ONLY — :409 propagates editErr BARE
  message_test.go      # READ-ONLY — :546 errors.Is(err, generate.ErrEmptyMessage) stays GREEN
internal/exitcode/
  exitcode.go          # READ-ONLY — :65 errors.Is(err, generate.ErrEmptyMessage) → Error (invariant)
  exitcode_test.go     # READ-ONLY — :25 For(ErrEmptyMessage) == Error stays GREEN
cmd/stagecoach/
  main.go              # READ-ONLY — :67 "stagecoach: %v" is the prefix site (CORRECT; do NOT edit)
pkg/stagecoach/
  stagecoach.go        # READ-ONLY — :709/:754 public-API twins (BARE propagation)
Makefile               # coverage-gate=line 77 (generate IS gated; literal swap adds no branch; pinning test adds coverage)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/generate/
  finalize.go          # MODIFIED — line 45 literal: "stagecoach: empty commit message — aborted" → "empty commit message — aborted"
  generate_test.go     # MODIFIED — +TestErrEmptyMessage_NoStagecoachPrefix
# NOTHING ELSE. No edit to main.go, exitcode.go, generate.go, decompose/*, pkg/stagecoach/*, go.mod, docs.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (EM DASH, not hyphen): the literal uses Unicode EM DASH — (U+2014; UTF-8 E2 80 94). The edit's
// oldText AND newText MUST carry the exact em-dash byte. cat -v shows it as M-bM-^@M-^T. A plain hyphen -
// would (a) fail to match oldText and (b) alter the user-visible message. Copy the byte from the existing line.

// CRITICAL (sentinel IDENTITY is preserved — errors.Is is invariant): errors.New returns *errors.errorString
// holding the string; errors.Is(err, target) compares the POINTER first. ErrEmptyMessage is a package-level
// var — its address does NOT change when you edit the string it holds. So errors.Is(err, ErrEmptyMessage)
// is true before AND after the edit. exitcode.go:65's errors.Is → exit 1 is invariant. DO NOT touch any
// errors.Is site or exitcode.go.

// CRITICAL (NO test asserts on the literal string — "update test assertions" is a NO-OP): exhaustive grep
// proves every test uses errors.Is (generate_test.go:882, hooks_freeze_test.go:198, message_test.go:546,
// stagecoach_test.go:1681/1703) or For(sentinel)→code (exitcode_test.go:25). NONE do strings.Contains on
// "stagecoach: empty commit message". So there is NOTHING to update. ADD the pinning test instead.

// GOTCHA (main.go:67 is CORRECT — do NOT edit it): main.go prepends "stagecoach: " to every error uniformly.
// The bug is the sentinel DUPLICATING that prefix. Fix the sentinel (finalize.go:45), leave main.go alone.
// Editing main.go to special-case ErrEmptyMessage would break the uniform-prefix discipline.

// GOTCHA (the godoc on finalize.go:41-44 is ALREADY correct): it says 'maps it to exit 1 with "empty
// commit message — aborted"' (no prefix) — documenting the USER-FACING string (after main.go's prefix).
// The literal was out of sync with its own godoc; the fix aligns them. DO NOT edit the godoc.

// GOTCHA (all propagation paths return the sentinel BARE — no second prefix to chase): finalize.go:118,
// generate.go:486/517, decompose.go:409, message.go:213/254, pkg/stagecoach/stagecoach.go:709/754 all
// `return ErrEmptyMessage` / `return generate.ErrEmptyMessage` / propagate a bare `err` that IS the sentinel.
// None wrap with fmt.Errorf("stagecoach: %w", ...). So the ONE literal edit fixes every path at once.
```

## Implementation Blueprint

### Data models and structure

None NEW. `ErrEmptyMessage` is an existing `*errors.errorString` sentinel (finalize.go:45). The edit changes
only the string it holds; the type, the var, and its identity are unchanged. No new types, fields, or packages.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/generate/finalize.go:45 — drop the "stagecoach: " prefix from the literal
  - CHANGE (one line):
      FROM: var ErrEmptyMessage = errors.New("stagecoach: empty commit message — aborted")
      TO:   var ErrEmptyMessage = errors.New("empty commit message — aborted")
  - PRESERVE the em-dash — (U+2014) EXACTLY in both the oldText match and the newText (not a hyphen).
  - DO NOT edit the godoc on lines 41-44 (it already describes the un-prefixed user-facing message).
  - DO NOT edit line 118 (return "", ErrEmptyMessage — unchanged).
  - GOTCHA: this is the ONLY production change. main.go, exitcode.go, generate.go, decompose/* are untouched.

Task 2: ADD internal/generate/generate_test.go — TestErrEmptyMessage_NoStagecoachPrefix (pinning test)
  - PLACE: near the existing `t.Run("fake editor empties message → ErrEmptyMessage", …)` (line 858), or as a
    standalone top-level test func. It tests the PACKAGE-LEVEL sentinel directly (no harness/repo needed).
  - BODY (ready to paste — preserve the em-dash):
      func TestErrEmptyMessage_NoStagecoachPrefix(t *testing.T) {
          // Issue 5: ErrEmptyMessage's literal must NOT start with "stagecoach: " — cmd/stagecoach/main.go:67
          // prepends "stagecoach: " to every error, so a prefixed literal doubles to
          // "stagecoach: stagecoach: empty commit message — aborted". errors.Is (identity) is unaffected;
          // this pins the LITERAL TEXT (the user-facing string after main.go adds its prefix).
          if strings.HasPrefix(ErrEmptyMessage.Error(), "stagecoach: ") {
              t.Errorf("ErrEmptyMessage literal has a 'stagecoach: ' prefix (%q); main.go:67 would double it",
                  ErrEmptyMessage.Error())
          }
          const want = "empty commit message — aborted"
          if got := ErrEmptyMessage.Error(); got != want {
              t.Errorf("ErrEmptyMessage.Error() = %q, want %q", got, want)
          }
      }
  - IMPORTS: the test uses strings.HasPrefix — verify generate_test.go already imports "strings" (it very
    likely does — many tests use strings.Contains); if NOT, add "strings" to the import block.
  - NAMING: TestErrEmptyMessage_NoStagecoachPrefix (descriptive; matches the TestXxx_Yyy convention).
  - COVERAGE: this is the ONLY test that asserts the literal STRING (every other ErrEmptyMessage test uses
    errors.Is identity). It both PROVES the fix and LOCKS regression (re-adding the prefix fails it).

Task 3: VERIFY — build, vet, format, the contract's test command, full regression, lint, grep guards
  - go build ./...
  - go vet ./internal/generate/...
  - gofmt -l internal/generate/finalize.go internal/generate/generate_test.go   # empty
  - go test ./internal/generate/ ./internal/decompose/ -v   # the CONTRACT's command — all errors.Is tests GREEN + new pinning test
  - go test ./internal/exitcode/ -v                          # For(ErrEmptyMessage) == Error (1) GREEN
  - go test ./pkg/stagecoach/ -v                             # public-API twins' errors.Is tests GREEN
  - make test ; make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the one-line fix — identity-preserving literal edit):
//   BEFORE (finalize.go:45):
var ErrEmptyMessage = errors.New("stagecoach: empty commit message — aborted") // ← doubled prefix w/ main.go:67
//   AFTER:
var ErrEmptyMessage = errors.New("empty commit message — aborted") // ← single prefix after main.go:67 adds "stagecoach: "

// PATTERN (why it's safe — errors.Is compares the POINTER, not the string):
//   ErrEmptyMessage is a package-level *errors.errorString. Editing the string it holds does NOT change
//   its address. So errors.Is(err, ErrEmptyMessage) (which does `err == target` first) is invariant.
//   exitcode.go:65 `errors.Is(err, generate.ErrEmptyMessage) → Error` keeps mapping to exit 1.

// PATTERN (the pinning test — the only STRING assertion for this sentinel):
func TestErrEmptyMessage_NoStagecoachPrefix(t *testing.T) {
	if strings.HasPrefix(ErrEmptyMessage.Error(), "stagecoach: ") { // the regression guard
		t.Errorf("ErrEmptyMessage literal has a 'stagecoach: ' prefix (%q); main.go:67 would double it",
			ErrEmptyMessage.Error())
	}
	const want = "empty commit message — aborted" // preserve the em-dash —
	if got := ErrEmptyMessage.Error(); got != want {
		t.Errorf("ErrEmptyMessage.Error() = %q, want %q", got, want)
	}
}
```

### Integration Points

```yaml
PRODUCTION (internal/generate/finalize.go):
  - LINE 45 literal: "stagecoach: empty commit message — aborted" → "empty commit message — aborted"
    (the ONLY production change; em-dash preserved; identity preserved).

TESTS (internal/generate/generate_test.go):
  - ADD TestErrEmptyMessage_NoStagecoachPrefix (~6 lines + ensure "strings" imported).
  - NO existing test modified (all use errors.Is; invariant under the edit).

NO-OP CONFIRMATIONS (verify, do NOT edit):
  - main.go:67 "stagecoach: %v" — the prefix site; correct as-is.
  - exitcode.go:65 errors.Is(err, generate.ErrEmptyMessage) → Error — invariant.
  - generate.go:486/517, decompose.go:409, message.go:213/254, pkg/stagecoach/stagecoach.go:709/754 —
    all propagate the sentinel BARE (no second prefix).

NO database / migration / routes / new types / new flag / config change / signature change / docs.
  - Docs (README/how-it-works/cli) are P1.M4.T1 — NOT here (contract: "DOCS: none — internal error message
    formatting, no user-facing config/API surface"). The fix changes a user-visible string but no config/API/flag.
  - The parallel P1.M3.T1.S2 (Issue 4, internal/git/tokengate.go) and P1.M3.T3.S1 (Issue 6, default_action.go)
    touch DIFFERENT files — zero overlap.

SCOPE FENCES:
  - Touches ONLY internal/generate/finalize.go (1 literal) + internal/generate/generate_test.go (1 test).
  - Does NOT edit main.go, exitcode.go, generate.go, internal/decompose/*, pkg/stagecoach/*, go.mod, docs,
    or any PRD/task file.
  - Adds NO flag, NO type, NO import (except possibly "strings" in generate_test.go if not already present),
    NO third-party dependency, NO signature change.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (the literal edit + the new test compile).
go build ./...
# Expected: clean.

# Vet.
go vet ./internal/generate/...
# Expected: clean.

# Format.
gofmt -l internal/generate/finalize.go internal/generate/generate_test.go
# Expected: empty. If listed: gofmt -w internal/generate/finalize.go internal/generate/generate_test.go

# Lint.
make lint   # golangci-lint
# Expected: zero errors.

# Scope guard: ONLY the 2 files changed.
git status --porcelain
# Expected: internal/generate/finalize.go, internal/generate/generate_test.go. ZERO changes elsewhere
#           (esp. NOT main.go, exitcode.go, generate.go, decompose/*, pkg/stagecoach/*).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The CONTRACT's command — the generate + decompose tests (all errors.Is tests GREEN + new pinning test).
go test ./internal/generate/ ./internal/decompose/ -v
# Expected: ALL PASS.
#   - TestErrEmptyMessage_NoStagecoachPrefix (NEW): passes (literal == "empty commit message — aborted", no prefix).
#   - The "fake editor empties message → ErrEmptyMessage" subtest (generate_test.go:858): GREEN unchanged
#     (errors.Is(err, ErrEmptyMessage) — identity, invariant).
#   - hooks_freeze_test.go ErrEmptyMessage tests: GREEN unchanged.
#   - message_test.go ErrEmptyMessage tests: GREEN unchanged.

# The exit-code mapper (errors.Is → exit 1 invariant).
go test ./internal/exitcode/ -v
# Expected: GREEN. The "ErrEmptyMessage → 1" case (exitcode_test.go:25) passes For(ErrEmptyMessage) == Error.

# The public-API twins.
go test ./pkg/stagecoach/ -v
# Expected: GREEN (stagecoach_test.go:1681/1703 errors.Is(err, generate.ErrEmptyMessage) — invariant).

# Full race suite.
make test
# Expected: green. The literal edit changes no control flow; the pinning test adds a pure string check.
```

### Level 3: Integration Testing (System Validation)

```bash
# This is a one-literal polish fix with no API/config/flag surface change. The user-visible proof is the
# actual stderr output of a --edit abort. Manual smoke (optional — the unit pinning test is the real proof):
make build
cd "$(mktemp -d)" && git init && git config user.email t@t && git config user.name t && echo hi > f.txt && git add f.txt && git commit -m init
echo change > f.txt && git add f.txt
# Empty the editor to trigger the abort:
export GIT_EDITOR='sh -c ": > \"$1\"" --'
./bin/stagecoach --edit 2>&1 | head -1
# Expected (AFTER the fix): "stagecoach: empty commit message — aborted"  (SINGLE prefix)
# Before the fix it was:     "stagecoach: stagecoach: empty commit message — aborted"  (DOUBLE)
echo "exit=$?"  # 1 (ErrEmptyMessage → exitcode.Error)

# No e2e test captures this stderr (verified — findings §3), so the manual smoke + the pinning unit test
# are the proof. The exit code (1) is covered by exitcode_test.go:25.
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: the literal no longer carries the "stagecoach: " prefix.
grep -n 'var ErrEmptyMessage = errors.New' internal/generate/finalize.go
# Expect: var ErrEmptyMessage = errors.New("empty commit message — aborted")  (NO "stagecoach: " prefix).

# Guard 2: the em-dash is preserved (not a hyphen).
grep -nP 'empty commit message \x{2014} aborted' internal/generate/finalize.go
# Expect: 1 hit (the U+2014 em-dash). If 0 hits, someone used a hyphen — fix it.

# Guard 3: the godoc is UNCHANGED (it was already correct).
grep -c 'maps it to exit 1 with "empty commit message' internal/generate/finalize.go
# Expect: 1 (the godoc line 44, untouched).

# Guard 4: NO other file was edited (scope fence).
git diff --name-only
# Expect: internal/generate/finalize.go + internal/generate/generate_test.go ONLY.
git diff --name-only | grep -E 'main\.go|exitcode\.go|generate\.go|decompose/|pkg/stagecoach/' && echo "FAIL: out-of-scope file edited" || echo "OK: scope clean"

# Guard 5: the pinning test exists.
grep -n 'func TestErrEmptyMessage_NoStagecoachPrefix' internal/generate/generate_test.go
# Expect: 1 hit.

# Guard 6: no test asserts on the OLD doubled literal (confirm none regressed into a string match).
grep -rn 'stagecoach: empty commit message\|stagecoach: stagecoach' --include="*_test.go" .
# Expect: ZERO hits (no test pins the old prefixed string; the bug is fixed and locked by the pinning test).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/generate/...` clean
- [ ] `gofmt -l` empty on the 2 files
- [ ] `make lint` zero errors
- [ ] `go test ./internal/generate/ ./internal/decompose/ -v` green (contract command) + new pinning test passes
- [ ] `go test ./internal/exitcode/ -v` + `go test ./pkg/stagecoach/ -v` green (errors.Is invariant)
- [ ] `make test` (full race suite) green

### Feature Validation
- [ ] `ErrEmptyMessage.Error()` == `"empty commit message — aborted"` (no `"stagecoach: "` prefix; em-dash preserved)
- [ ] The new `TestErrEmptyMessage_NoStagecoachPrefix` passes (grep guard 5)
- [ ] `errors.Is(err, ErrEmptyMessage)` still true at every site; `exitcode.For(ErrEmptyMessage) == Error` (1)
- [ ] All existing `errors.Is(…ErrEmptyMessage)` tests GREEN UNCHANGED (no test was modified)
- [ ] Manual smoke (Level 3): --edit abort prints single-prefixed `stagecoach: empty commit message — aborted`

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `internal/generate/finalize.go` + `internal/generate/generate_test.go`
- [ ] NO edit to main.go, exitcode.go, generate.go, internal/decompose/*, pkg/stagecoach/*, go.mod, docs,
      or any PRD/task file (grep guard 4)
- [ ] NO new flag/type/third-party dependency/signature change (the only possible import add is "strings" in generate_test.go)
- [ ] NO overlap with P1.M3.T1.S2 (Issue 4, internal/git) or P1.M3.T3.S1 (Issue 6, default_action.go)
- [ ] NO docs edit (P1.M4.T1 owns the docs sync; contract: "DOCS: none")

### Code Quality & Docs
- [ ] The literal edit preserves the em-dash `—` (U+2014) — not a hyphen (grep guard 2)
- [ ] The godoc on finalize.go:41-44 is untouched (it was already correct)
- [ ] The pinning test carries a comment explaining WHY the literal must not carry the prefix (main.go:67 doubles it)
- [ ] Contract honored: "DOCS: none — internal error message formatting, no user-facing config/API surface"

---

## Anti-Patterns to Avoid

- ❌ Don't edit main.go. `main.go:67` (`"stagecoach: %v"`) is the CLI's uniform error-prefix site and is
  CORRECT. The bug is the sentinel `ErrEmptyMessage` carrying its OWN prefix. Fix the sentinel (finalize.go:45),
  leave main.go alone. Special-casing ErrEmptyMessage in main.go would break the one-prefix discipline.
- ❌ Don't edit exitcode.go or any `errors.Is` site. The `errors.Is(err, generate.ErrEmptyMessage) → Error`
  mapping is IDENTITY-based (pointer compare), invariant under the literal edit. Touching it is unnecessary
  and risks breaking the exit-1 mapping. (findings §8.)
- ❌ Don't replace the em-dash with a hyphen. The literal uses `—` (U+2014, UTF-8 E2 80 94). The edit's
  oldText AND newText must carry the exact byte (grep guard 2). A hyphen fails to match AND changes the
  user-visible text.
- ❌ Don't hunt for a string-assertion test to "update" — there are NONE. The contract's step (b) is
  defensive; exhaustive grep (findings §3) proves every ErrEmptyMessage test uses `errors.Is` (identity),
  which is invariant. ADD the pinning test instead of searching for a non-existent assertion to edit.
- ❌ Don't edit the godoc on finalize.go:41-44. It ALREADY says `"empty commit message — aborted"` (no prefix)
  because it documents the user-facing string (after main.go's prefix). The literal was the only thing out
  of sync; the fix aligns it with the existing godoc.
- ❌ Don't wrap ErrEmptyMessage with `fmt.Errorf("…: %w", ErrEmptyMessage)` anywhere. Every propagation path
  returns it BARE (finalize.go:118, generate.go:486/517, decompose.go:409, message.go:213/254). Wrapping it
  would (a) add a second prefix source and (b) still pass `errors.Is` but change the displayed text. The fix
  is the ONE literal, nothing else.
- ❌ Don't add docs. The contract says "DOCS: none — internal error message formatting, no user-facing
  config/API surface." The README/how-it-works/cli sync is P1.M4.T1 (separate task). The fix changes a
  user-visible string but no config/API/flag, so it's polish, not a feature-doc item.
- ❌ Don't conflate this with the sibling bugfixes. P1.M3.T1.S2 is Issue 4 (token gate, internal/git);
  P1.M3.T3.S1 is Issue 6 (auto-stage grammar, default_action.go). This item is Issue 5 (finalize.go literal).
  Three different files, three different issues — zero overlap. Touch only finalize.go + generate_test.go.
- ❌ Don't skip the pinning test. Without it, the literal has no test coverage and the doubling can silently
  regress (someone re-adding `"stagecoach: "`). The pinning test is ~6 lines, needs no harness, and is the
  ONLY positive proof + regression lock for the fix. It's the rigorous interpretation of the contract's
  "update test assertions" (ADD, since none exist to UPDATE).
