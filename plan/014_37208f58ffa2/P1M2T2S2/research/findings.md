# P1.M2.T2.S2 Research Findings — Wire watchdog arming in default_action.go

> Consolidated, line-numbered verification of every integration point the consumer wiring depends on.
> The change itself is surgical (2 imports + 1 gated arming block in ONE file); these findings exist so
> the implementing agent needs zero "prior knowledge" and makes zero wrong assumptions.

## §0 Contract recap (from the item description + parallel-execution context)

P1.M2.T2.S2 wires the **consumer** of the watchdog. It depends on TWO parallel siblings landing as specified:

| Contract | Source PRP | Symbol | Signature / shape |
|---|---|---|---|
| watchdog API | P1.M2.T2.S1 | `watchdog.Arm` | `func Arm(ctx context.Context, interval time.Duration)` (best-effort, no error return; nil-safe; Windows no-op) |
| config gate | P1.M2.T1.S1 | `Config.NoParentWatchdog` | `NoParentWatchdog bool \`toml:"no_parent_watchdog"\`` on `*config.Config`; default `false`; resolved via 7-layer precedence |

The logic, verbatim from the contract:
```go
// In internal/cmd/default_action.go, immediately AFTER `defer locker.Release()`:
if !cfg.NoParentWatchdog {
    watchdog.Arm(ctx, 1*time.Second)   // FR-K2 default poll cadence
}
```
Plus imports `internal/watchdog` and `time`. Mock nothing — test via the e2e harness (P1.M4.T1.S1).

---

## §1 The ctx is ALREADY signal-aware (the single most important fact)

The contract says: "The ctx passed to runDefault is the signal-aware ctx from main.go." Verified end-to-end:

`cmd/stagecoach/main.go` (lines 87–96):
```go
func main() {
	cmd.Version = resolveVersion(version)
	ctx, _ := signal.Install(context.Background(), signal.Options{
		RescueFormat: generate.FormatRescue,
		OnRescueExit: lock.ReleaseCurrent, // FR52 §18.5: release the lock file before os.Exit orphans it
		Out:          os.Stderr,
	})
	err := cmd.Execute(ctx)
	...
}
```

`internal/cmd/root.go` `Execute` (lines 315–320):
```go
func Execute(ctx context.Context) error {
	rootCmd.Version = Version
	if ctx != nil {
		rootCmd.SetContext(ctx)   // ← the signal-aware ctx lands on the root cobra command
	}
	return rootCmd.Execute()
}
```

`internal/cmd/default_action.go` `runDefault` (line 38):
```go
	ctx := cmd.Context() // S1's Execute set this; P1.M4.T2 swaps it for a signal-aware ctx later.
```
**The inline comment is STALE** — the swap already happened in main.go (signal.Install → Execute → SetContext → cmd.Context).
**Implication**: the watchdog's `ctx` = the signal-aware ctx. When the signal handler calls `h.cancel()` (handle()), that cancellation propagates into `watchdog.Arm`'s internal `armCtx` (Arm does `armCtx, cancel := context.WithCancel(ctx)`), so the poll goroutine is cleaned up on rescue/exit. And on parent death the watchdog calls `signal.Trigger(SIGTERM)` → `os.Exit`, killing the goroutine. No explicit `watchdog.Stop()` is needed on the CLI path (the goroutine dies with the process). `Stop()` exists for library use (pkg/stagecoach without signal.Install).

## §2 The exact insertion site (verified line numbers)

`internal/cmd/default_action.go` lines 71–82 (current):
```go
	locker, lockErr := lock.Acquire(repoDir)
	if lockErr != nil {
		var held *lock.HeldError
		if errors.As(lockErr, &held) { // contention → no-op fast path (0) or Busy (5), both silent
			return handleLockContention(stderr, held, g, ctx)
		}
		return exitcode.New(exitcode.Error, fmt.Errorf("acquire run lock: %w", lockErr))
	}
	defer locker.Release()

	// ---- §9.4 auto-stage-all state machine (FR16–FR20) ----
```
- Insert the arming block **between** `defer locker.Release()` and the `// ---- §9.4 auto-stage-all…` comment.
- `cfg` is `*config.Config` (`cfg := Config()` at line 46) → `cfg.NoParentWatchdog` is valid once P1.M2.T1.S1 lands.
- `ctx` is already in scope (line 38).
- **One arming covers BOTH the single-commit and the decompose paths**: the lock comment (lines 67–70) states "One acquire + one defer covers BOTH the single-commit path and the decompose path (runDecompose is called below)." `runDecompose` is invoked from within `runDefault` AFTER the lock is acquired (and AFTER the arming site). So a SECOND arming in `runDecompose` is WRONG — note this in the code comment to prevent it.

## §3 The exact import edits

Current `default_action.go` import block (lines 3–21):
```go
import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dustin/stagecoach/internal/config"
	"github.com/dustin/stagecoach/internal/decompose"
	"github.com/dustin/stagecoach/internal/exclude"
	"github.com/dustin/stagecoach/internal/exitcode"
	"github.com/dustin/stagecoach/internal/generate"
	"github.com/dustin/stagecoach/internal/git"
	"github.com/dustin/stagecoach/internal/lock"
	"github.com/dustin/stagecoach/internal/provider"
	"github.com/dustin/stagecoach/internal/ui"
	"github.com/dustin/stagecoach/pkg/stagecoach"
)
```
- ADD `"time"` to the stdlib group, after `"strings"` (alphabetical; gofmt-managed).
- ADD `"github.com/dustin/stagecoach/internal/watchdog"` to the stagecoach group, after `internal/ui` and before `pkg/stagecoach` (alphabetical within the contiguous block).
- goimports/gofmt will finalize ordering; placing them correctly pre-gofmt avoids a `gofmt -l` dirty.

## §4 The lock-release seam the watchdog rides (NO internal/lock import in default_action.go)

The watchdog's ONLY effect is `signal.Trigger(syscall.SIGTERM)`. Verified `internal/signal/signal.go`:

`Trigger` (lines 156–164):
```go
// Trigger routes a synthetic signal through the rescue/exit path. … nil-safe … stopped-guarded.
func Trigger(sig os.Signal) {
	if h := active.Load(); h != nil {
		h.handle(sig)
	}
}
```

`handle` (lines 173–203) — the single rescue path Trigger reuses:
```go
func (h *Handler) handle(sig os.Signal) {
	if h.stopped.Load() { return }            // no-op after RestoreDefault (update-ref window)
	if pid := h.childPID.Load(); pid > 0 { _ = h.opts.Kill(int(pid), sig) }  // forward to child group
	h.cancel()                                 // cancel the signal-aware ctx → runDefault unwinds
	...
	if tree != "" {
		fmt.Fprintln(h.opts.Out, h.opts.RescueFormat(...)); h.opts.OnRescueExit(); h.opts.Exit(3)   // rescue
		return
	}
	h.opts.OnRescueExit();  h.opts.Exit(exitCodeForSignal(sig))  // pre-snapshot: 129/130/143
}
```
`OnRescueExit` = `lock.ReleaseCurrent` (wired in main.go; `lock.ReleaseCurrent` at `internal/lock/lock.go:211`).

**Implication for default_action.go**: do NOT add an `internal/lock` import and do NOT call `locker.Release()` from the watchdog path. The watchdog → `signal.Trigger` → `handle` → `OnRescueExit`=lock.ReleaseCurrent releases the lock file before `os.Exit` skips the deferred `locker.Release()`. This is EXACTLY the same path a terminal SIGTERM takes. The consumer (this task) just calls `watchdog.Arm`; everything else is already wired.

## §5 Why NO unit test is added (contract: "mock nothing")

The item contract is explicit: "Mock nothing — test via the e2e harness (P1.M4.T1.S1) which launches real stagecoach subprocesses." Rationale, all verified:

1. The arming is inert in unit tests. `default_action_test.go` invokes `Execute(context.Background())` (≈30 call sites; e.g. line 196, 261, 301, …). The test process's `getppid()` is STABLE (its parent — the `go test` runner / shell — does not die mid-test), so the watchdog's poll NEVER fires. Even if it did, `signal.Active()` is `nil` in those tests (no `signal.Install`) → `signal.Trigger` is a nil-safe no-op. So `watchdog.Arm` cannot affect a unit test's outcome.
2. To unit-test the gate ("armed when false, not armed when true") we would have to inject a fake `watchdog` or an `Arm` seam — the contract forbids mocking. The behavioral proof is a real subprocess whose launcher dies (the e2e harness), which exercises `getppid()` change + real `signal.Install` + real exit. That is P1.M4.T1.S1.
3. Therefore THIS task's validation = build/vet/gofmt/lint clean + the FULL existing regression suite stays green + a manual sanity run. No new test file.

### Harmless goroutine accumulation in the cmd test binary (observed, safe, documented here)
Each `TestRunDefault_*` that reaches the lock (staged-content tests) will arm a 1s-poll goroutine on `context.Background()` (the ctx the test passes — never canceled). Across one `go test ./internal/cmd` binary that is a handful of goroutines ticking at 1s. They never fire (stable ppid) and die at process exit. Under `-race` there is no shared mutable state (each goroutine reads its own `originalPpid` + the nil-safe `Trigger`), so no race and no leak-check failure (Go testing has no built-in leak detector). **`make test` stays green.** If a future task wants goroutine hygiene in tests, the fix belongs in the test harness (pass a cancelable ctx to `Execute`), NOT here.

## §6 Validation surface (verified Makefile targets)

```
build:          line 52  → ./bin/stagecoach
test:           line 70  → go test -race ./...
lint:           line 103 → golangci-lint
coverage-gate:  line 77  → ≥85% on internal/{git,provider,generate,config} ONLY (NOT cmd → not a gate for this task)
```
`make coverage-gate` does NOT cover `internal/cmd`, so the no-unit-test decision does not threaten the coverage gate. The cmd package's existing tests provide the regression net.

## §7 Grep guards (the "did it land correctly" checks)

```bash
# 1. Exactly ONE production watchdog.Arm caller, gated by cfg.NoParentWatchdog.
grep -rn 'watchdog.Arm' --include='*.go' internal/ cmd/ pkg/
# Expect: default_action.go (1 hit) + internal/watchdog/*_test.go (the S1 tests). cmd/ has exactly 1.

# 2. The gate reads the parallel-sibling field, not a flag.
grep -n 'cfg.NoParentWatchdog\|NoParentWatchdog' internal/cmd/default_action.go
# Expect: 1 hit — the `if !cfg.NoParentWatchdog {` block.

# 3. The ctx passed is cmd.Context() (the signal-aware ctx), NOT a fresh context.
grep -n 'watchdog.Arm(ctx' internal/cmd/default_action.go
# Expect: 1 hit using the `ctx` from line 38 (`ctx := cmd.Context()`).

# 4. The arming is AFTER the lock is held (after `defer locker.Release()`).
grep -n -A3 'defer locker.Release()' internal/cmd/default_action.go
# Expect: the arming block immediately follows.

# 5. Imports added: time + internal/watchdog.
grep -n '"time"\|internal/watchdog' internal/cmd/default_action.go
# Expect: 2 hits (1 each).

# 6. NO internal/lock import added in default_action.go (the seam is via signal, not a direct lock call).
grep -c 'internal/lock' internal/cmd/default_action.go
# Expect: 1 (the PRE-EXISTING lock import for lock.Acquire/HeldError). NOT 2.
```

## §8 No-conflict / scope-fence summary

- Modifies ONLY `internal/cmd/default_action.go` (2 imports + 1 arming block).
- Does NOT touch `internal/watchdog/*` (P1.M2.T2.S1), `internal/config/*` (P1.M2.T1.S1), `internal/signal/*` (P1.M1.T2.S1 — Trigger already exported), `internal/lock/*`, `cmd/stagecoach/main.go`, or any test file.
- Adds NO new third-party dependency (go.mod unchanged).
- Adds NO CLI flag (FR-K6 has none) and NO new file.
