# Research Notes — P1.M1.T1.S1: Add UpdateAvailable=6 exit code + For()/sentinel wiring

## Current state of internal/exitcode/exitcode.go
- Package doc comment (lines 1-10) lists codes "0/1/2/3/124" — MUST be updated to include 6.
- Const block (lines 20-27): Success=0, Error=1, NothingToCommit=2, Rescue=3, Busy=5, Timeout=124.
  - NOTE the GAP at 4 (reserved per integration_seams §2; Busy is 5, see TestBusyCodeValue).
  - Comment cites PRD §15.4 and explicitly overrides arch/go_ecosystem_patterns.md §1.2.
- ExitError struct + New(code, err) + Error()/Unwrap().
- For(err) ordering (lines ~57-90):
  1. err==nil → Success
  2. errors.As(err, &ee) → ee.Code          ← catches New(6, nil) automatically (no change needed for this path)
  3. errors.Is(generate.ErrNothingToCommit) → 2
  4. errors.Is(generate.ErrEmptyMessage) → 1
  5. errors.Is(generate.ErrTimeout) → 124
  6. errors.Is(generate.ErrRescue) → 3
  7. errors.Is(context.DeadlineExceeded) → 124
  8. errors.Is(generate.ErrCASFailed) → 1
  9. default → Error (1)

## Sentinel style to mirror (generate.go:82-99)
```go
// ErrNothingToCommit is returned when ... CLI → exit 2 (PRD §15.4). ...
var ErrNothingToCommit = errors.New("stagecoach: nothing staged to commit")
```
→ ErrUpdateAvailable goes in exitcode.go (NOT generate), same doc style, citing FR-U6/FR-U12 + §15.4.

## Test pattern (exitcode_test.go)
- Table-driven `TestFor` with {name, err, want} rows.
- `TestBusyCodeValue` asserts exact const value.
- `TestExitError_NilErr` covers New(Error,nil).Error()=="".
- Existing rows already cover commit-path sentinels → 2/1/3/124. The task wants an EXPLICIT
  regression-guard test asserting NO commit-path sentinel yields 6.

## Key invariants (from task contract + v3_scope_boundary.md "Exit-code discipline (FR-U12)")
- 6 = UpdateAvailable is UPGRADE-PATH ONLY; never returned by commit path.
- New(6, nil) MUST map to 6 via the existing errors.As path (no change needed there).
- errors.Is(err, ErrUpdateAvailable) MUST ALSO map to 6 (new sentinel branch in For()).
- Do NOT touch any commit-path mapping. Do NOT remap existing codes.
- 4 stays reserved/absent.

## DOCS impact (Mode A)
Update the package doc comment (lines 1-10) to mention code 6 + upgrade-path-only scope.

## Validation commands VERIFIED in this repo
- `go test ./internal/exitcode/` → "ok ... 0.003s" (PASS today)
- `go vet ./internal/exitcode/` → exit 0 (PASS today)
- `test -z "$(gofmt -l internal/exitcode/)"` → prints GOFMT-CLEAN, exit 0 (PASS today)
- `go test -race ./internal/exitcode/` → cheap, matches repo's `test-race` Makefile target
- golangci-lint is NOT installed in this env → do NOT use as a gate.

## Placement decision for the new sentinel branch
Add `errors.Is(err, ErrUpdateAvailable) → UpdateAvailable` AFTER the errors.As block and BEFORE
the generate.* commit-path branches, grouped as the single upgrade-path branch with an
explanatory comment. Correctness is unaffected by ordering (the sets are disjoint — no error is
both an upgrade-path and commit-path sentinel), but grouping keeps the upgrade path visually
separate from the commit-path generate.* cascade and documents the §15.4 / FR-U12 scope.

## Call-site awareness
exitcode is used at ~262 call sites across cmd/ and internal/. This task changes ONLY
internal/exitcode/{exitcode.go, exitcode_test.go}. No caller needs to change (6 is only
produced by the future upgrade command). Nothing else imports ErrUpdateAvailable yet.
