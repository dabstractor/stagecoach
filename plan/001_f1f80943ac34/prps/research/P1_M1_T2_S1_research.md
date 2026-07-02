# Research Notes — P1.M1.T2.S1: internal/ui/exitcode.go (canonical exit codes)

## Task contract (verbatim from tasks.json)
- LOGIC: Define named constants: Success=0, Error=1, NothingToCommit=2, Rescue=3, Timeout=124.
  Pure constants (no logic). Test: assert exact integer values (golden) so downstream
  exit-code mapping never drifts.
- OUTPUT: `ui.Exit*` consts consumed by cmd/stagehand error mapping (M7.T2.S1) and
  rescue/timeout paths.

## PRD §15.4 Exit codes (the binding table — file:///home/dustin/projects/stagehand-hack/PRD.md, line ~958)
| Code | Meaning |
|------|---------|
| 0    | Success (commit created, or dry-run message printed) |
| 1    | General error (generation failed, parse failed after retries, agent missing, etc.) |
| 2    | Nothing to commit (clean tree after auto-stage, or nothing staged with --no-auto-stage) |
| 3    | Rescue condition (snapshot taken, commit not created — manual recovery printed) |
| 124  | Timeout (generation exceeded --timeout) |

Supporting PRD refs:
- FR17 (line 258): clean tree after auto-stage → exit code 2.
- §9.10 / B.5: rescue protocol → exit code 3.
- The 124 value is the GNU `timeout` convention (`timeout` exits 124 when the command
  exceeds the time limit) — chosen so users/ci piping stagehand under `timeout` see a
  consistent signal. This is a deliberate, documented value, NOT arbitrary.

## Downstream consumers (why the values must be frozen exactly)
- cmd/stagehand error mapping (M7.T2.S1): os.Exit(ui.ExitError) on general failure,
  os.Exit(ui.ExitNothingToCommit) on empty diff (FR17), os.Exit(ui.ExitRescue) after the
  rescue protocol, os.Exit(ui.ExitTimeout) when ctx deadline hit.
- internal/generate/rescue.go (M6.T2.S1): rescue path returns/sets ExitRescue.
- decisions.md §3 / §7: the generate orchestrator's RESCUE branch and CAS-failure branch
  ("print msg + manual recovery; exit 1") both depend on these constants being stable.
  => Golden test on exact ints prevents a refactor from silently shifting 2↔3 etc.

## Codebase state
- This is the FIRST file under internal/ (internal/ does not exist yet — cmd/stagehand is
  the only Go package). It is also the FIRST _test.go in the repo.
- No test framework/runner is configured. Go's stdlib `testing` package is sufficient and
  requires zero extra dependencies. `go test ./...` already exits 0 (M1.T1.S1 Makefile
  target). Verified: `go version go1.26.4`, `go vet ./...` clean.
- Module path: github.com/dustin/stagehand → internal package import path is
  `github.com/dustin/stagehand/internal/ui`, declared as `package ui`.

## Design decisions (binding for the implementer)
1. Typed `int` constants (matches stdlib idiom: io.SeekStart, os.O_RDONLY are typed int).
   Lets os.Exit(ui.ExitError) compile with no conversion.
2. Exported identifiers, `Exit`-prefixed per contract "ui.Exit*":
     ExitSuccess = 0
     ExitError = 1
     ExitNothingToCommit = 2
     ExitRescue = 3
     ExitTimeout = 124
3. EXPLICIT integer literals, NOT iota — values are non-sequential (124 jumps). iota would
   produce 0,1,2,3,4 and silently break ExitTimeout. This is the #1 pitfall to flag.
4. No imports. A pure-constants file imports nothing (compile-time signal of "no logic").
5. Package doc comment lives in this file (first ui file); sibling output.go (S2) should
   use a plain `package ui` line without a duplicate package doc to avoid revive/golint
   "duplicate package comment" — flagged for S2's PRP, not this task's action.

## Test approach (golden, table-driven)
- File: internal/ui/exitcode_test.go, `package ui` (white-box → references consts directly).
- Single table-driven TestExitCodes mapping each const to its exact expected int, with a
  trailing guard that ensures no two distinct semantic names collide (defensive). Assert:
  ExitSuccess==0, ExitError==1, ExitNothingToCommit==2, ExitRescue==3, ExitTimeout==124.
- This file establishes the repo-wide testing pattern: stdlib testing, table-driven,
  t.Errorf on mismatch, go test ./internal/ui/ -v.

## Validation commands verified/known-good on this host
- go build ./internal/ui/   (compiles the package)
- go vet ./internal/ui/      (static checks; clean baseline confirmed via `go vet ./...`)
- test -z "$(gofmt -l internal/ui/)"  (gofmt -l prints names of unformatted files; always
  exits 0 so wrapped in test -z — same pattern proven by M1.T1.S1 validation-results.json)
- go test ./internal/ui/      (runs the golden table)
- go test ./...               (whole-module integrity, Makefile `test` target)

## DOCS impact
None. Exit codes are already specified in PRD §15.4. No README/docs created (README is
Mode B, synced in M8.T4). This task adds code + test only.
