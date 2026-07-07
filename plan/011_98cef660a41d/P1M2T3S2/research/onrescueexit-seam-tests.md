# P1.M2.T3.S2 — Exit-path release signal tests (signal_test.go): Research Notes

TEST-ONLY subtask. Verifies the `OnRescueExit` injectable seam landed by **P1.M2.T2.S1** (and wired by
P1.M2.T2.S2). The production code is ALREADY IN THE TREE — this task adds ONLY tests.

## §1 — The production code under test (LANDED — read, do NOT edit)

`internal/signal/signal.go` — the seam is complete on BOTH exit branches:

```go
// handle() — the extracted-for-testing method (signal_test.go calls it DIRECTLY, no goroutine):
func (h *Handler) handle(sig os.Signal) {
	if h.stopped.Load() {   // RestoreDefault sets this → early return (no-op)
		return
	}
	if pid := h.childPID.Load(); pid > 0 {
		_ = h.opts.Kill(int(pid), sig)
	}
	h.cancel()
	h.mu.Lock()
	tree, parent, cand := h.snapTree, h.snapParent, h.snapCandidate
	h.mu.Unlock()

	if tree != "" {                                                  // POST-SNAPSHOT branch
		fmt.Fprintln(h.opts.Out, h.opts.RescueFormat(tree, parent, cand))
		h.opts.OnRescueExit()   // ← L149: fires BEFORE Exit(3)
		h.opts.Exit(3)          // ← L150
		return
	}
	h.opts.OnRescueExit()           // ← L153: PRE-SNAPSHOT branch, fires BEFORE Exit(130/143)
	h.opts.Exit(exitCodeForSignal(sig))  // ← L154
}
```

`Options.OnRescueExit func()` — field (L73) + defaulted to `func(){}` in Install (L96-97). Defaulted so
existing tests (which never inject it) stay byte-identical / green.

`exitCodeForSignal` (signal_unix.go, `//go:build !windows`): SIGINT/`os.Interrupt`→130, SIGTERM→143, else 1.
NOTE: this lives in signal_unix.go → **pre-snapshot Exit(130) only exists on Unix**. But signal_test.go has NO
build tag (`package signal`), compiles on ALL platforms. Windows has no `exitCodeForSignal` — but signal_test.go
doesn't reference it; it just calls `h.handle(os.Interrupt)` and observes the Exit recorder's captured code.

WAIT — does signal_test.go compile on Windows? Yes: the WHOLE internal/signal package must compile on Windows.
signal_unix.go (`//go:build !windows`) holds `KillProcessGroup` + `exitCodeForSignal`; signal_windows.go
(`//go:build windows`) must hold equivalents. The EXISTING signal_test.go (no build tag) already calls
`h.handle(syscall.SIGINT)` and asserts `exitCode == 130` — so the Windows build of the package MUST produce 130
for SIGINT, meaning signal_windows.go has its own `exitCodeForSignal`. **Therefore my pre-snapshot test (assert
Exit 130) compiles AND passes on Windows exactly like the existing `TestHandler_Exit130PreSnapshot`.** No build
tag needed on signal_test.go. (Confirmed: signal_integration_test.go is the ONLY `//go:build !windows` test
file — it uses syscall.Kill real-process tests; signal_test.go is cross-platform.)

## §2 — The contract → 3 tests (signal_test.go is the home; NO new file)

Contract (item_description) specifies 3 scenarios:

(a) **POST-SNAPSHOT**: `SetSnapshot("tree","parent","cand")` → inject recording OnRescueExit + recording Exit →
    `h.handle(os.Interrupt)` → assert OnRescueExit called EXACTLY ONCE + Exit code == 3 + OnRescueExit BEFORE Exit.
(b) **PRE-SNAPSHOT**: NO SetSnapshot (tree=="") → same recorders → `h.handle(os.Interrupt)` → assert OnRescueExit
    called EXACTLY ONCE + Exit code == 130 + ordering preserved.
(c) **RESTOREDEFAULT**: `RestoreDefault()` → `h.handle(os.Interrupt)` → assert OnRescueExit NOT called (handler
    stopped; default disposition applies). "No real lock needed — the seam is a recorder."

Mapping → 3 focused tests (matches the existing file's granular 1-test-per-scenario style):
- `TestHandler_OnRescueExit_PostSnapshot`   (contract a)
- `TestHandler_OnRescueExit_PreSnapshot`    (contract b)
- `TestHandler_OnRescueExit_SkippedAfterRestoreDefault` (contract c)

These are ADDITIVE — no existing test covers OnRescueExit. `TestHandler_Exit130PreSnapshot` / `_RescueOnSignalWithSnapshot`
test Exit CODES only; `TestHandler_RestoreDefaultStopsForward` tests Kill+Exit no-op. None assert the OnRescueExit
ordering/frequency/skip — that gap is precisely this task.

## §3 — The ordering-verification technique (per contract: "OnRescueExit sets a flag that Exit checks")

handle() is SYNCHRONOUS within one goroutine — it calls OnRescueExit then Exit in sequence. So a flag set by
OnRescueExit and READ by Exit reliably proves ordering (no goroutine/timing needed — that's WHY handle() was
extracted from run()):

```go
var rescueCalls int            // "called int" counter (contract a/b)
var rescueFired bool           // flag OnRescueExit sets
var exitCode int               // code recorder (Exit)
var exitSawRescueFired bool    // what Exit observed (false ⇒ ordering violation)

opts := Options{
	OnRescueExit: func() {
		rescueCalls++
		rescueFired = true
	},
	Exit: func(code int) {
		exitCode = code
		exitSawRescueFired = rescueFired   // Exit "checks" the flag
	},
	Out: new(bytes.Buffer),
}
// ... drive h.handle(os.Interrupt) ...
// assert: rescueCalls == 1 ; exitCode == 3 (or 130) ; exitSawRescueFired == true
```

If ordering were reversed (Exit before OnRescueExit), `rescueFired` would still be false when Exit runs →
`exitSawRescueFired == false` → test fails with a clear "OnRescueExit must fire BEFORE Exit" message.

## §4 — Direct handle() invocation (no goroutine timing)

The contract is explicit: "call h.handle(os.Interrupt) directly (no goroutine timing needed — handle() is
extracted for direct testing)." Do NOT send a real signal to the process; do NOT rely on the run() goroutine.
`installTestHandler(t, opts)` returns `*Handler`; call `h.handle(os.Interrupt)` directly. This is how EVERY
existing unit test in signal_test.go works (e.g. `TestHandler_ForwardsToChildGroup`, `TestHandler_Exit130PreSnapshot`).

## §5 — os.Interrupt vs syscall.SIGINT

Contract says `os.Interrupt`. `exitCodeForSignal` handles BOTH (`case os.Interrupt, syscall.SIGINT: return 130`).
On Unix `os.Interrupt == syscall.SIGINT`. Use `os.Interrupt` to match the contract verbatim (and it reads as
"the signal Ctrl-C sends"). The `os` package is already imported in signal_test.go.

## §6 — NO new imports, NO build tag, NO helper changes

signal_test.go currently imports: `bytes`, `context`, `os`, `syscall`, `testing`.
My tests use: `os` (os.Interrupt), `bytes` (new(bytes.Buffer) for Out), `context` (installTestHandler →
Install(context.Background(), …)), `testing`. ALL already imported. `syscall` stays (existing tests use it).
→ **Zero import changes.** Zero new helpers (the existing `installTestHandler` + `contains` suffice; I don't
even need `contains` since I assert on recorders, not rescue output text). Zero build tag (signal_test.go is
cross-platform; the pre-snapshot 130 assertion already works on Windows via signal_windows.go's exitCodeForSignal).

## §7 — No t.Parallel (the singleton)

`installTestHandler` does `active.Store(h)` (via Install) + `t.Cleanup(func(){ active.Store(nil) })`. The
`active` singleton is process-global. Existing tests are sequential (none call t.Parallel). My tests follow suit
— call `installTestHandler` (which sets + cleans the singleton). No t.Parallel.

## §8 — RestoreDefault-then-handle detail (contract c)

`RestoreDefault()` does `h.stopped.CompareAndSwap(false,true)` + `signal.Stop(h.ch)` + `close(h.ch)`. After it,
`h.handle(sig)` returns at the FIRST line (`if h.stopped.Load() { return }`) — so neither Kill, nor cancel, nor
OnRescueExit, nor Exit runs. Contract (c) asserts OnRescueExit NOT called. (Also assert Exit NOT called for
completeness — the handler is fully stopped.) `RestoreDefault` is nil-safe + idempotent (already covered by
`TestHandler_RestoreDefaultIdempotent`).

## §9 — Scope fences (what NOT to touch)

- NO production change (signal.go / lock.go / signal_unix.go / signal_windows.go are LANDED by P1.M2.T2.S1 +
  P1.M2.T1 — FROZEN).
- NO main.go (P1.M2.T2.S2 wired OnRescueExit: lock.ReleaseCurrent — FROZEN).
- NO signal_integration_test.go / signal_unix.go (those are the real-binary / KillProcessGroup tests).
- NO lock_test.go / lock_unix_test.go (those are P1.M2.T3.S1's reaping tests — parallel, different concern).
- NO docs (DOCS: none — test-only; P1.M3 owns changeset doc sync).
- NO go.mod / go.sum (stdlib only).
- SOLE EDIT: append 3 tests to `internal/signal/signal_test.go`.

## §10 — Validation commands (verified against the repo)

```bash
gofmt -w internal/signal/signal_test.go
test -z "$(gofmt -l internal/signal/)" && echo "gofmt clean"
go vet ./internal/signal/
go build ./...
go test -race ./internal/signal/ -v -run 'TestHandler_OnRescueExit'   # the 3 new tests
go test -race ./internal/signal/                                       # full signal suite (existing + new)
go test -race ./...                                                    # full module — no regression
GOOS=windows go test ./internal/signal/                                # cross-platform: signal_test.go compiles + passes on Windows too
make lint                                                              # golangci-lint — no unused (the recorders are used)
git diff --name-only                                                   # ONLY internal/signal/signal_test.go
git diff --exit-code go.mod go.sum && echo "deps unchanged"
```
