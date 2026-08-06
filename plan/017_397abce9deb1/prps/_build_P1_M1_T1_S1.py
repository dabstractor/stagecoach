#!/usr/bin/env python3
"""Assemble the PRP JSON for P1.M1.T1.S1 and write it to the target path.
Uses json.dump so all escaping is correct."""
import json, os

TASK_ID = "P1.M1.T1.S1"
OUT = "/home/dustin/projects/stagecoach/plan/017_397abce9deb1/prps/P1_M1_T1_S1.json"

objective = (
    "Add exit code 6 (UpdateAvailable) to internal/exitcode/exitcode.go as the upgrade-path-only "
    "code that `stagecoach upgrade --check` returns when a newer release exists (PRD §15.4, FR-U6, "
    "FR-U12). Wire BOTH a sentinel path (errors.Is(err, ErrUpdateAvailable)) and the existing "
    "explicit-ExitError path (New(UpdateAvailable, nil)) to map to 6, leave all commit-path "
    "mappings untouched, and add a regression-guard test proving no commit-path error yields 6. "
    "Update the package doc comment (Mode A) to mention code 6 + its upgrade-path-only scope. "
    "`go test ./internal/exitcode/` must pass."
)

context = r"""# PRP — P1.M1.T1.S1: Add UpdateAvailable=6 exit code + For()/sentinel wiring

## Goal

**Feature Goal**: Add exit code 6 (`UpdateAvailable`) to `internal/exitcode/exitcode.go` — the upgrade-path-only code that `stagecoach upgrade --check` returns when a newer release exists (PRD §15.4, FR-U6, FR-U12) — wired so BOTH `errors.Is(err, ErrUpdateAvailable)` and the existing `New(UpdateAvailable, nil)`/`errors.As` path resolve to 6, while every commit-path mapping stays byte-for-byte unchanged and a test explicitly proves 6 is unreachable from commit-path errors.

**Deliverable**:
1. Modified `internal/exitcode/exitcode.go` — new `UpdateAvailable = 6` const; new `ErrUpdateAvailable` sentinel error; one new branch in `For()`; updated package doc comment.
2. Modified `internal/exitcode/exitcode_test.go` — new `TestFor` table rows + a dedicated `TestFor_NoCommitPathYieldsUpdateAvailable` regression guard + a `TestUpdateAvailableCodeValue` const-value test.
3. `go test ./internal/exitcode/` passes.

**Success Definition**: `For(ErrUpdateAvailable) == 6`; `For(New(UpdateAvailable, nil)) == 6`; `For(fmt.Errorf("w: %w", ErrUpdateAvailable)) == 6`; all pre-existing commit-path mappings unchanged (2/3/1/124); and a test iterates the commit-path sentinels asserting none produce 6.

## Why

- Code 6 is the FIRST reusable primitive the entire `stagecoach upgrade` feature consumes (Milestone P1.M1). Sibling subtasks (P1.M1.T1.S2 comparable version, P1.M4.T2.S1 the `--check` command) return `exitcode.New(exitcode.UpdateAvailable, nil)` / wrap `ErrUpdateAvailable` — they block on this code existing.
- FR-U6: "`stagecoach upgrade --check` ... Exit 0 if up to date, exit `6` if an update is available, so CI/cron can gate on it." The exit code must exist and be reachable before the command that produces it.
- FR-U12: "Exit codes: `0` ...; `6` update-available (`--check`); `1` ... — all distinct from the commit-path codes 2/3/5/124." This task establishes that distinctness and GUARDS it with a regression test.

## What

User-visible behavior: none yet (no caller produces 6 until the upgrade command lands). This task only defines the code, the sentinel, and the `For()` mapping + their tests. It is pure additive plumbing: a new const value, a new sentinel, one new `errors.Is` branch, updated docs, and tests. **No commit-path code path is modified, and a test asserts that.**

### Success Criteria

- [ ] `UpdateAvailable = 6` exists in the const block, with a comment citing PRD §15.4 / FR-U12 and stating it is upgrade-path only.
- [ ] `var ErrUpdateAvailable = errors.New(...)` exists in `exitcode.go`, documented to mirror `generate.ErrNothingToCommit` style and citing FR-U6/FR-U12.
- [ ] `For(ErrUpdateAvailable) == 6`, including when wrapped (`fmt.Errorf("wrap: %w", ErrUpdateAvailable)`).
- [ ] `For(New(UpdateAvailable, nil)) == 6` (the pre-existing `errors.As` path — verify, do not break).
- [ ] Every pre-existing `TestFor` mapping still holds (2/3/1/124 unchanged) — no commit-path branch is touched.
- [ ] A dedicated regression-guard test iterates commit-path sentinels (ErrNothingToCommit, ErrEmptyMessage, ErrRescue, ErrTimeout, context.DeadlineExceeded, ErrCASFailed, CASError, generic error) and asserts NONE yields 6.
- [ ] Package doc comment mentions code 6 and its upgrade-path-only scope (Mode A docs impact).
- [ ] `go test ./internal/exitcode/`, `go vet ./internal/exitcode/`, and gofmt-clean all pass.

## All Needed Context

### Context Completeness Check

"If someone knew nothing about this codebase, would they have everything needed to implement this successfully?" — YES. This PRP includes the exact current `For()` ordering, the exact sentinel style to mirror, the exact const-block alignment behavior (gofmt-managed), the exact code to add, the exact test rows to add, and the regression-guard test in full. The implementing agent needs no other files.

### Documentation & References

```yaml
# MUST READ — the file you are editing (read it in full first)
- file: internal/exitcode/exitcode.go
  why: The ONLY production file to modify. Contains the const block, ExitError, New(), and For().
  pattern: For() order = nil -> errors.As(*ExitError) -> errors.Is(generate.*) sentinels -> default 1.
  gotcha: The errors.As(err, &ee) check is checked FIRST; it ALREADY maps New(UpdateAvailable, nil) to 6.
          You only need to ADD the errors.Is(ErrUpdateAvailable) sentinel branch for bare/wrapped sentinels.

- file: internal/exitcode/exitcode_test.go
  why: The test file to extend. Table-driven TestFor + TestBusyCodeValue + TestExitError_NilErr.
  pattern: Add rows to the TestFor []struct{name,err,want} table; add a new TestXxx func for the guard.

# SENTINEL STYLE TO MIRROR (do NOT edit this file — only copy its var-Err style)
- file: internal/generate/generate.go
  why: Lines 82-99 define var ErrNothingToCommit/ErrTimeout/ErrRescue with `errors.New("stagecoach: ...")`.
  pattern: `var ErrX = errors.New("stagecoach: <human message>")` with a preceding `// ErrX is ...` doc comment.
  gotcha: ErrUpdateAvailable lives in package EXITCODE (same package as For()), NOT generate. No import needed.

# ARCHITECTURE — the authoritative scope contract (read the "Exit-code discipline (FR-U12)" table)
- docfile: plan/017_397abce9deb1/architecture/v3_scope_boundary.md
  why: Section "Exit-code discipline (FR-U12)": 6 is NEW + upgrade-path-only; 2/3/5/124 are commit-path only;
       "6 must be added to internal/exitcode/exitcode.go as UpdateAvailable and wired so a --check that finds
       a newer version returns exitcode.New(exitcode.UpdateAvailable, nil) (or a sentinel the For() map
       recognizes). It is NOT a commit-path code and must never be returned by the commit path."
  critical: This task does BOTH wiring options (sentinel AND New()), per its contract item 3(b).

- docfile: plan/017_397abce9deb1/architecture/system_context.md
  why: Section "Conventions to follow (verified in code)" — Exit codes bullet:
       "add a const + (if needed) a branch in For(). Return exitcode.New(exitcode.X, err) from RunE.
        main.go already maps via exitcode.For."

# PRD — the requirement source (already provided in the work-item PRD context)
- prd: PRD §9.29 FR-U6  ("--check" ... exit 6 if an update is available, so CI/cron can gate on it)
- prd: PRD §9.29 FR-U12 (exit codes 0/6/1 distinct from commit-path 2/3/5/124; walled off from commit core)
- prd: PRD §15.4        (authoritative exit-code table; 6 = UpdateAvailable)
```

### Current Codebase tree (relevant slice)

```bash
internal/exitcode/
  exitcode.go        # const block + ExitError + New + For  <- MODIFY
  exitcode_test.go   # table-driven TestFor + 2 small tests <- MODIFY
internal/generate/
  generate.go        # sentinel style reference only (lines 82-99) <- DO NOT TOUCH
cmd/stagecoach/
  main.go            # calls exitcode.For(err) already; needs NO change <- DO NOT TOUCH
```

### Desired Codebase tree with files to be added/changed

```bash
internal/exitcode/
  exitcode.go        # +UpdateAvailable const, +ErrUpdateAvailable sentinel, +For() branch, +doc comment
  exitcode_test.go   # +4 TestFor rows, +TestFor_NoCommitPathYieldsUpdateAvailable, +TestUpdateAvailableCodeValue
```

No new files. No new packages. No new dependencies (`errors` is already imported; `errors.Is` already used).

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: For() checks errors.As(err, &ee) BEFORE any sentinel branch. Because New(UpdateAvailable, nil)
//   returns *ExitError{Code:6}, it is ALREADY mapped to 6 by that path — do NOT add a second mapping for it
//   and do NOT reorder the As() check below the sentinel branch. The ONLY new code is the
//   errors.Is(err, ErrUpdateAvailable) branch for a BARE or WRAPPED sentinel.
// CRITICAL: The const block has a deliberate GAP at 4 (reserved per integration_seams §2; Busy=5, see
//   TestBusyCodeValue). Insert UpdateAvailable=6 between Busy(5) and Timeout(124); do NOT use 4.
// CRITICAL: Comment column alignment in const ( ... ) groups is MANAGED BY gofmt — just add your line with a
//   `//` comment and run gofmt; the gate `test -z "$(gofmt -l ...)"` will fail if you forget. Note
//   "UpdateAvailable" is 15 chars (same as "NothingToCommit") so the `=` column does not shift.
// GOTCHA: ErrUpdateAvailable is package-local to `exitcode` (the same package For() lives in) — reference it
//   as a bare identifier (ErrUpdateAvailable), not exitcode.ErrUpdateAvailable, inside exitcode.go/tests.
// GOTCHA: golangci-lint is NOT installed in this environment; rely on `go vet` + `gofmt` + `go test`.
// GOTCHA: This package is imported by ~262 call sites; changing ONLY exitcode.go/exitcode_test.go touches
//   no caller. 6 is not produced anywhere until the upgrade command lands in a later subtask.
```

## Implementation Blueprint

### Data models and structure

None — no structs, no DB, no config. This task adds a single `int` const, a single `error` sentinel var, one branch in an existing `switch`-like cascade, doc text, and tests.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/exitcode/exitcode.go — package doc comment (Mode A docs impact)
  - FIND: the first 10 lines (package doc comment) currently say "...exit codes (0/1/2/3/124)" and
          "...it covers explicit *ExitError overrides, the generate-domain mapping ...".
  - UPDATE the parenthetical to "(0/1/2/3/5/6/124)".
  - ADD one sentence stating: code 6 = UpdateAvailable is the UPGRADE-PATH-ONLY code (PRD §15.4/FR-U12);
        it is produced solely by `stagecoach upgrade --check`, is walled off from the commit path
        (§9.1–§9.28), and is distinct from commit-path codes 2/3/5/124.
  - PRESERVE every other line of the doc comment (the D1 naming note, the §15.4-overrides-§1.2 note).

Task 2: MODIFY internal/exitcode/exitcode.go — const block
  - FIND the `const ( ... )` block (Success=0 ... Busy=5, Timeout=124).
  - ADD between `Busy = 5` and `Timeout = 124`:
        UpdateAvailable = 6   // UPGRADE PATH ONLY (PRD §15.4, FR-U6/FR-U12): `stagecoach upgrade --check`
                            // found a newer version (informational). NEVER returned by the commit path
                            // (§9.1–§9.28); distinct from 0/1/2/3/5/124.
  - DO NOT use value 4 (reserved). DO NOT renumber or remap any existing const.
  - RUN gofmt to fix comment-column alignment (the gate enforces gofmt-clean).

Task 3: MODIFY internal/exitcode/exitcode.go — ErrUpdateAvailable sentinel
  - PLACE immediately after the closing `)` of the const block (before the ExitError type), with a doc comment:
        // ErrUpdateAvailable is the upgrade-path sentinel: `stagecoach upgrade --check` found a newer release
        // (PRD §15.4, FR-U6/FR-U12 — exit 6, informational). For() maps it to UpdateAvailable. It is the ONLY
        // non-zero exit code produced solely by the upgrade path; the commit path (§9.1–§9.28) can never return
        // it (asserted in TestFor_NoCommitPathYieldsUpdateAvailable). Mirrors generate.ErrNothingToCommit style.
        var ErrUpdateAvailable = errors.New("stagecoach: a newer version is available")
  - `errors` is already imported — do NOT add an import. Do NOT put it in package generate.

Task 4: MODIFY internal/exitcode/exitcode.go — For() sentinel branch
  - FIND For(): after the `var ee *ExitError; if errors.As(err, &ee) { return ee.Code }` block and BEFORE the
        `if errors.Is(err, generate.ErrNothingToCommit)` line.
  - INSERT (one branch, with the comment):
        // UPGRADE PATH (PRD §15.4/FR-U12): exit 6 is produced ONLY by `stagecoach upgrade --check`. A bare
        // ErrUpdateAvailable (or any error wrapping it) -> 6. The errors.As path above already covers
        // New(UpdateAvailable, nil). Disjoint from the commit-path generate.* branches below.
        if errors.Is(err, ErrUpdateAvailable) {
            return UpdateAvailable
        }
  - DO NOT touch any generate.* branch or context.DeadlineExceeded. DO NOT reorder existing branches.
  - WHY this position: the sets are disjoint (no error is both upgrade-path and commit-path), so ordering is
        correctness-neutral; placing it first, clearly separated from the commit-path cascade, documents scope.

Task 5: MODIFY internal/exitcode/exitcode_test.go — extend TestFor table
  - FIND the `tests := []struct{name string; err error; want int}{...}` in TestFor.
  - APPEND these rows (match the existing `{"name", <err>, <code>}` shape):
        {"ErrUpdateAvailable -> 6", ErrUpdateAvailable, UpdateAvailable},
        {"wrapped ErrUpdateAvailable -> 6 (errors.Is traverses %w)", fmt.Errorf("wrap: %w", ErrUpdateAvailable), UpdateAvailable},
        {"explicit ExitError UpdateAvailable (nil err) -> 6", New(UpdateAvailable, nil), UpdateAvailable},
        {"explicit ExitError UpdateAvailable (with err) -> 6", New(UpdateAvailable, errors.New("newer")), UpdateAvailable},
  - DEPENDENCIES: ErrUpdateAvailable + UpdateAvailable from Task 2/3; fmt/errors already imported in the test.

Task 6: MODIFY internal/exitcode/exitcode_test.go — regression guard test (NEW func)
  - APPEND a new test mirroring TestBusyCodeValue/TestExitError_NilErr style:
        // TestFor_NoCommitPathYieldsUpdateAvailable asserts the upgrade-path invariant (FR-U12 +
        // v3_scope_boundary.md "Exit-code discipline"): code 6 is upgrade-path ONLY; no commit-path
        // sentinel can produce it. Regression guard for the const/For() additions in this task.
        func TestFor_NoCommitPathYieldsUpdateAvailable(t *testing.T) {
            commitPathErrors := []error{
                generate.ErrNothingToCommit,
                generate.ErrEmptyMessage,
                &generate.RescueError{Kind: generate.ErrRescue},
                &generate.RescueError{Kind: generate.ErrTimeout},
                generate.ErrTimeout,
                context.DeadlineExceeded,
                generate.ErrCASFailed,
                &generate.CASError{Expected: "a", Actual: "b"},
                errors.New("generic commit-path failure"),
            }
            for _, err := range commitPathErrors {
                if got := For(err); got == UpdateAvailable {
                    t.Errorf("commit-path error %v mapped to UpdateAvailable(6) -- 6 must be upgrade-path only", err)
                }
            }
        }
  - DEPENDENCIES: generate + context already imported in the test file.

Task 7: MODIFY internal/exitcode/exitcode_test.go — const-value test (NEW func)
  - APPEND (mirrors the existing TestBusyCodeValue exactly):
        // UpdateAvailable is 6 -- distinct from 0/1/2/3/5/124 (FR-U12 / v3_scope_boundary.md table).
        func TestUpdateAvailableCodeValue(t *testing.T) {
            if UpdateAvailable != 6 {
                t.Errorf("UpdateAvailable = %d, want 6", UpdateAvailable)
            }
        }

Task 8: RUN validation gates
  - gofmt the two files (the gate enforces gofmt-clean), then run every gate below.
```

### Implementation Patterns & Key Details

```go
// PATTERN — the full For() after the edit (new lines marked // NEW). Read this BEFORE editing so you
// preserve the exact ordering. (Only the two // NEW regions change; everything else is verbatim.)

func For(err error) int {
	if err == nil {
		return Success
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code           // already maps New(UpdateAvailable, nil) -> 6. DO NOT reorder.
	}
	if errors.Is(err, ErrUpdateAvailable) {   // NEW — upgrade-path sentinel (Task 4)
		return UpdateAvailable
	}
	if errors.Is(err, generate.ErrNothingToCommit) {
		return NothingToCommit
	}
	if errors.Is(err, generate.ErrEmptyMessage) {
		return Error
	}
	if errors.Is(err, generate.ErrTimeout) {
		return Timeout
	}
	if errors.Is(err, generate.ErrRescue) {
		return Rescue
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Timeout
	}
	if errors.Is(err, generate.ErrCASFailed) {
		return Error
	}
	return Error
}

// CRITICAL: the bare-sentinel and New()-paths are BOTH required by the task contract item 3(b):
//   "Implement For() so that errors.Is(err, ErrUpdateAvailable) -> UpdateAvailable AND keep the existing
//    errors.As(err, &ee) path so New(UpdateAvailable, nil) also maps to 6." Verify both in tests (Task 5).
```

### Integration Points

```yaml
DATABASE: none
CONFIG:   none (the [upgrade] config table is a DIFFERENT subtask, P1.M1.T2.S1 — do not touch config here)
ROUTES:   none
CALLERS:  none to change. main.go already does os.Exit(exitcode.For(err)); 6 simply becomes reachable
          once the upgrade command (later subtask) returns New(UpdateAvailable, nil) / wraps ErrUpdateAvailable.
```

## Validation Loop

### Level 1 — Syntax / Type / Format

Run after editing each file; fix before proceeding.

```bash
go vet ./internal/exitcode/                                 # compile + govet (go vet exit 0 = pass)
test -z "$(gofmt -l internal/exitcode/)" && echo GOFMT-OK   # empty gofmt -l output = formatted
```

### Level 2 — Unit Tests (the core deliverable)

```bash
go test ./internal/exitcode/            # all of TestFor (+new rows), the regression guard, the const test
go test ./internal/exitcode/ -run 'TestFor|TestFor_NoCommitPathYieldsUpdateAvailable|TestUpdateAvailableCodeValue' -v
```

### Level 3 — Race + neighbors (cheap safety; matches repo `test-race` target)

```bash
go test -race ./internal/exitcode/      # no concurrency here, but race-clean guarantees nothing regressed
go build ./...                          # confirms the package still compiles for every importer
```

### Level 4 — Manual sanity (optional, informational)

```bash
go doc ./internal/exitcode              # package doc now lists code 6 + upgrade-path scope (Mode A)
```

## Final Validation Checklist

### Technical
- [ ] `go vet ./internal/exitcode/` exits 0
- [ ] `test -z "$(gofmt -l internal/exitcode/)"` succeeds (files are gofmt-clean)
- [ ] `go test ./internal/exitcode/` passes
- [ ] `go test -race ./internal/exitcode/` passes
- [ ] `go build ./...` succeeds (no importer broken)

### Feature / Contract
- [ ] `For(ErrUpdateAvailable) == 6` (TestFor row)
- [ ] `For(fmt.Errorf("w: %w", ErrUpdateAvailable)) == 6` (wrapped-sentinel TestFor row)
- [ ] `For(New(UpdateAvailable, nil)) == 6` (explicit-ExitError-nil TestFor row)
- [ ] `For(New(UpdateAvailable, errors.New("x"))) == 6` (explicit-ExitError-with-err TestFor row)
- [ ] Pre-existing commit-path mappings unchanged (2/3/1/124) — the table rows for ErrNothingToCommit,
      ErrEmptyMessage, RescueError(ErrRescue/ErrTimeout), context.DeadlineExceeded, ErrCASFailed/CASError,
      generic error all still pass
- [ ] `TestFor_NoCommitPathYieldsUpdateAvailable` passes (6 unreachable from commit path — FR-U12)
- [ ] `UpdateAvailable == 6` (TestUpdateAvailableCodeValue)

### Code Quality
- [ ] No commit-path branch in For() was modified or reordered
- [ ] `ErrUpdateAvailable` lives in package exitcode (not generate); `errors` already imported (no new import)
- [ ] Const block keeps the 4 gap; UpdateAvailable=6 inserted between Busy(5) and Timeout(124)
- [ ] Follows existing table-driven test style and existing const-value-test style

### Docs & Deployment (Mode A — this item's declared DOCS impact)
- [ ] Package doc comment (exitcode.go lines 1-10) updated to list code 6 and state its upgrade-path-only scope
      (citing PRD §15.4/FR-U12). No other docs file is in scope for this subtask (the changeset-level
      README/docs security-narrative update is a later Mode B task, per v3_scope_boundary.md).

## Anti-Patterns to Avoid

- DON'T reorder For()'s existing branches or move the `errors.As` check — it must stay first.
- DON'T add a second/competing mapping for `New(UpdateAvailable, nil)`; the `errors.As` path already covers it.
- DON'T put `ErrUpdateAvailable` in package generate; it belongs in exitcode (same package as For()).
- DON'T use value 4 (reserved) or remap any existing code (2/3/5/124 are frozen — FR-U12).
- DON'T touch any caller (cmd/, internal/cmd/*, main.go) — 6 is not produced until a later subtask.
- DON'T introduce a new dependency or import (`errors` is already imported and used).
- DON'T skip the regression-guard test — it is the explicit assertion that "6 is upgrade-path only" (FR-U12).
"""

implementationSteps = [
    "Read internal/exitcode/exitcode.go and internal/exitcode/exitcode_test.go in full; read internal/generate/generate.go lines 82-99 for the sentinel style to mirror; read plan/017_397abce9deb1/architecture/v3_scope_boundary.md 'Exit-code discipline (FR-U12)' for the scope contract.",
    "Task 1 (DOCS, Mode A): edit the package doc comment (exitcode.go lines 1-10) — change '(0/1/2/3/124)' to '(0/1/2/3/5/6/124)' and add one sentence: code 6 = UpdateAvailable is the upgrade-path-only code (PRD §15.4/FR-U12), produced solely by `stagecoach upgrade --check`, walled off from the commit path (§9.1-§9.28), distinct from 2/3/5/124. Preserve all other doc lines.",
    "Task 2: insert `UpdateAvailable = 6` into the const block between `Busy = 5` and `Timeout = 124`, with a comment citing PRD §15.4/FR-U6/FR-U12 and stating it is upgrade-path only and never returned by the commit path. Do NOT use value 4 (reserved) and do NOT remap any existing const.",
    "Task 3: add `var ErrUpdateAvailable = errors.New(\"stagecoach: a newer version is available\")` immediately after the const block (before the ExitError type) with a doc comment noting it is the upgrade-path sentinel (FR-U6/FR-U12), mirrors generate.ErrNothingToCommit style, and is asserted upgrade-path-only by TestFor_NoCommitPathYieldsUpdateAvailable. `errors` is already imported — add NO new import.",
    "Task 4: in For(), insert one new branch immediately AFTER the `var ee *ExitError; if errors.As(err, &ee) { return ee.Code }` block and BEFORE the `errors.Is(err, generate.ErrNothingToCommit)` line: `if errors.Is(err, ErrUpdateAvailable) { return UpdateAvailable }` with a comment. Do NOT reorder or modify any existing branch (the errors.As path already maps New(UpdateAvailable, nil) to 6).",
    "Task 5: extend the TestFor table in exitcode_test.go with four rows: {ErrUpdateAvailable, UpdateAvailable}; {fmt.Errorf(\"wrap: %w\", ErrUpdateAvailable), UpdateAvailable}; {New(UpdateAvailable, nil), UpdateAvailable}; {New(UpdateAvailable, errors.New(\"newer\")), UpdateAvailable}.",
    "Task 6: add a new test func TestFor_NoCommitPathYieldsUpdateAvailable that iterates the commit-path sentinels [generate.ErrNothingToCommit, generate.ErrEmptyMessage, &generate.RescueError{Kind: generate.ErrRescue}, &generate.RescueError{Kind: generate.ErrTimeout}, generate.ErrTimeout, context.DeadlineExceeded, generate.ErrCASFailed, &generate.CASError{Expected:\"a\",Actual:\"b\"}, errors.New(\"generic failure\")] and asserts For(err) != UpdateAvailable for each (FR-U12 regression guard).",
    "Task 8: add a new test func TestUpdateAvailableCodeValue (mirroring TestBusyCodeValue) asserting UpdateAvailable == 6.",
    "Task 9: run gofmt on both files, then run all validation gates: `go vet ./internal/exitcode/`, `go test ./internal/exitcode/`, `go test -race ./internal/exitcode/`, `test -z \"$(gofmt -l internal/exitcode/)\"`, and `go build ./...`. All must pass."
]

validationGates = [
    {"level": "Level 1 — go vet (compile + govet)", "command": "go vet ./internal/exitcode/"},
    {"level": "Level 1 — gofmt clean", "command": "test -z \"$(gofmt -l internal/exitcode/)\""},
    {"level": "Level 2 — unit tests", "command": "go test ./internal/exitcode/"},
    {"level": "Level 3 — race detector", "command": "go test -race ./internal/exitcode/"},
    {"level": "Level 3 — whole-module build (no importer broken)", "command": "go build ./..."}
]

successCriteria = [
    {"description": "`UpdateAvailable = 6` const exists in internal/exitcode/exitcode.go (between Busy=5 and Timeout=124; value 4 left reserved) with a comment citing PRD §15.4/FR-U12 and stating it is upgrade-path only."},
    {"description": "`var ErrUpdateAvailable = errors.New(\"stagecoach: a newer version is available\")` exists in package exitcode with a doc comment mirroring generate.ErrNothingToCommit style and citing FR-U6/FR-U12."},
    {"description": "For(ErrUpdateAvailable) == 6 and For(fmt.Errorf(\"w: %%w\", ErrUpdateAvailable)) == 6 (new errors.Is branch in For())."},
    {"description": "For(New(UpdateAvailable, nil)) == 6 via the pre-existing errors.As(*ExitError) path (unchanged, verified by a test row)."},
    {"description": "All pre-existing commit-path mappings in For() are unchanged: ErrNothingToCommit->2, ErrEmptyMessage->1, RescueError(ErrRescue)->3, RescueError(ErrTimeout)/ErrTimeout/context.DeadlineExceeded->124, ErrCASFailed/CASError->1, generic->1."},
    {"description": "A dedicated TestFor_NoCommitPathYieldsUpdateAvailable test iterates the commit-path sentinels and asserts none yields 6 (FR-U12 regression guard)."},
    {"description": "TestUpdateAvailableCodeValue asserts UpdateAvailable == 6 (mirrors the existing TestBusyCodeValue)."},
    {"description": "Package doc comment (exitcode.go lines 1-10) lists code 6 and states its upgrade-path-only scope (Mode A docs impact for this subtask)."},
    {"description": "go vet ./internal/exitcode/, go test ./internal/exitcode/, go test -race ./internal/exitcode/, and `test -z \"$(gofmt -l internal/exitcode/)\"` all pass, and `go build ./...` succeeds."}
]

references = [
    "internal/exitcode/exitcode.go (the file to modify: const block, ExitError, New, For)",
    "internal/exitcode/exitcode_test.go (test file to extend: table-driven TestFor, TestBusyCodeValue, TestExitError_NilErr)",
    "internal/generate/generate.go lines 82-99 (sentinel style to mirror: var ErrX = errors.New(\"stagecoach: ...\"))",
    "plan/017_397abce9deb1/architecture/v3_scope_boundary.md — section 'Exit-code discipline (FR-U12)' (6 is NEW + upgrade-path only; 2/3/5/124 commit-path only; both New(6,nil) and sentinel wiring sanctioned)",
    "plan/017_397abce9deb1/architecture/system_context.md — section 'Conventions to follow (verified in code)' (Exit codes: add a const + branch in For(); main.go maps via exitcode.For)",
    "PRD §9.29 FR-U6 (--check exits 6 when an update is available, for CI/cron gating) and FR-U12 (exit codes 0/6/1 distinct from commit-path 2/3/5/124; upgrade walled off from commit core)",
    "PRD §15.4 (authoritative exit-code table; 6 = UpdateAvailable)"
]

doc = {
    "taskId": TASK_ID,
    "objective": objective,
    "context": context,
    "implementationSteps": implementationSteps,
    "validationGates": validationGates,
    "successCriteria": successCriteria,
    "references": references,
}

os.makedirs(os.path.dirname(OUT), exist_ok=True)
with open(OUT, "w") as f:
    json.dump(doc, f, indent=2, ensure_ascii=False)
    f.write("\n")
print("WROTE", OUT, os.path.getsize(OUT), "bytes")
