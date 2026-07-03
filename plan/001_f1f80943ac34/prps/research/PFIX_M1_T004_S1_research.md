# Research Notes — PFIX_M1_T004_S1 (BUG-004: timeout → exit 124)

## 1. Bug summary
A generation timeout (`*provider.TimeoutError`, returned by `executor.Run` when the
`runCtx` deadline elapses) is collapsed inside `generate.CommitStaged` into the bare
sentinel `ErrRescue`, which `cmd/stagehand.run.go:mapErrorToExitCode` maps to
`ui.ExitRescue` (3). PRD §15.4 mandates `124` (`ui.ExitTimeout`) for a generation
timeout. The rescue BLOCK still prints correctly (snapshot + tree SHA + manual
recovery) and the repo is left unchanged — only the EXIT CODE is wrong, so wrappers
(CI, lazygit `customCommand`) that branch on `124` never fire.

## 2. Root cause (why a CLI-only fix is impossible)
The run-error path in `internal/generate/generate.go`:
```go
stdout, runErr := deps.Runner.Run(runCtx, ...)
if runErr != nil {
    if errors.Is(runErr, context.Canceled) {
        return Result{}, ErrRescue          // signal path (bare, by design)
    }
    Rescue(out, treeSHA, parentSHA, "")
    return Result{}, ErrRescue              // timeout OR agent-error -> BARE sentinel
}
```
`ErrRescue` is `errors.New(...)` — it wraps NOTHING. So at the CLI,
`errors.As(err, &provider.TimeoutError{})` and `errors.Is(err, context.DeadlineExceeded)`
are BOTH false. Therefore the bug cannot be fixed by adding a branch in
`mapErrorToExitCode` alone — the timeout TYPE never reaches it. The cause must be
PRESERVED in the error chain so the CLI can detect it.

## 3. The minimal, correct fix (two coordinated edits)
Go 1.22 supports multiple `%w` verbs in `fmt.Errorf` (Go 1.20+).

**A. `internal/generate/generate.go`** (the run-error, non-signal path): wrap the cause
so BOTH `errors.Is(err, ErrRescue)` AND `errors.As(err, &provider.TimeoutError{})` hold:
```go
Rescue(out, treeSHA, parentSHA, "")
return Result{}, fmt.Errorf("%w: %w", ErrRescue, runErr)   // was: return Result{}, ErrRescue
```
- `errors.Is(err, ErrRescue)` stays TRUE → every existing test (TestCommitStaged_Timeout,
  invariants_test "timeout", TestIntegration_Timeout — all use `errors.Is(err, ErrRescue)`)
  still passes.
- timeout cause: `errors.As(err, &provider.TimeoutError{})` → TRUE (new).
- agent-error cause: `errors.As(err, &provider.AgentError{})` → TRUE, but TimeoutError →
  FALSE → still maps to rescue (3). Correct: only a TIMEOUT becomes 124.
- Add `"fmt"` to generate.go's import block (rescue.go uses fmt but generate.go's own
  import list does not include it today).
- The signal-cancel path stays BARE (`return Result{}, ErrRescue`) — unchanged
  (double-rescue guard semantics; it is never a timeout anyway).

**B. `cmd/stagehand/run.go:mapErrorToExitCode`**: add a branch detecting the typed
timeout BEFORE the `errors.Is(stagehand.ErrRescue)` branch (the wrapped error satisfies
both; timeout must win):
```go
var timeoutErr *provider.TimeoutError
if errors.As(err, &timeoutErr) {
    return ui.ExitTimeout   // 124
}
```
- `provider` is ALREADY imported in run.go (used for `provider.Registry`). No new import.
- Use the TYPED `*provider.TimeoutError` (per bug: "detecting the timeout type"; also
  executor.go's TimeoutError.Unwrap doc says the typed error is the detection mechanism),
  NOT `errors.Is(err, context.DeadlineExceeded)` — the typed check is more precise (won't
  misfire on any future transport/CLI-level DeadlineExceeded).

## 4. Behavior preserved (no safety regression)
- Rescue block STILL prints (Rescue is called before the wrap; the rescue print is
  driven by `generate.Rescue`, independent of the error string).
- Agent still killed (executor process-group kill on ctx deadline unchanged).
- Repo refs/index unchanged (UpdateRefCAS never reached).
- `reportError`/`isAlreadyReported` still work (`errors.Is(err, ErrRescue)` true →
  no double-print; the wrapped err is treated as already-reported).
- A non-timeout rescue (agent-error, parse-fail, dup-exhaustion, post-snapshot git
  error, signal-cancel) still returns 3.

## 5. DOCS impact (Mode A — MUST edit)
`docs/CONFIGURATION.md` §9 row 124 currently states the timeout "collapses a timeout
into the rescue path (ErrRescue → exit 3) ... 124 is reserved for a future CLI-level
deadline and is not returned today." After the fix this is FALSE: a generation timeout
returns 124 while STILL printing the rescue block. This row MUST be rewritten.
Also: `mapErrorToExitCode`'s own doc comment ("Timeout note ... RESERVED ... not
returned in v1") and `TestMapErrorToExitCode`'s comment ("timeout is NOT a separate
branch ... 124 is reserved/unused in v1") become false and MUST be updated.

## 6. Tests to add (no new test framework; stdlib only)
- `cmd/stagehand/run_test.go` TestMapErrorToExitCode: add
  - `{"timeout-wrapped-in-rescue", fmt.Errorf("%w: %w", stagehand.ErrRescue, &provider.TimeoutError{Deadline: time.Now()}), ui.ExitTimeout}`
  - `{"timeout-bare", &provider.TimeoutError{Deadline: time.Now()}, ui.ExitTimeout}`
  - `{"agenterror-wrapped-in-rescue-stays-3", fmt.Errorf("%w: %w", stagehand.ErrRescue, &provider.AgentError{Name:"x"}), ui.ExitRescue}`
  - keep existing `{"wrapped-rescue", ...} → ExitRescue` (a wrapped ErrRescue with NO
    timeout cause still → 3).
- `internal/generate/generate_test.go` TestCommitStaged_Timeout: ADD assertion that the
  returned err ALSO `errors.As` into `*provider.TimeoutError` (proves the type survives
  for CLI detection). Existing `errors.Is(err, ErrRescue)` assertion stays.
- The repo already ships an end-to-end timeout test (`TestIntegration_Timeout`,
  internal/generate, `{Hang:true}` stub + 500ms ctx) proving the TimeoutError path fires
  for real; combined with the new mapErrorToExitCode case, the full chain (Run → generate
  wrap → CLI 124) is proven. A separate cmd-level subprocess gate is OPTIONAL; the manual
  reproduction gate (build + a hanging provider + `--timeout`) is documented.

## 7. Verified commands (this environment)
- `go build ./...` → OK.
- `go test ./cmd/stagehand/ -run TestMapErrorToExitCode` → ok.
- `go test ./internal/generate/ -run TestCommitStaged_Timeout` → ok.
- Makefile: `make build test lint` contributor loop; `go vet ./...`; `gofmt -s -l`.

## 8. Scope boundaries (do NOT touch)
- Do NOT change `ui.Exit*` constants (frozen §15.4).
- Do NOT change the rescue render, the executor, or the signal handler.
- Do NOT change `pkg/stagehand` public surface (the wrap is in generate; the public
  ErrRescue alias still works via errors.Is).
- Do NOT wrap the signal-cancel path (keep it bare ErrRescue → 3).
- Do NOT change `ErrHeadMoved`/`ErrNothingToCommit`/`ErrNothingStaged` mapping.
