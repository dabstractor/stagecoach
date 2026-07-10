# Research Notes — P1.M1.T1.S1 (Platform signal set helper + SIGHUP exit code)

Verification of the task-description claims against the CURRENT working tree (2026-07-09).
The architecture doc (`plan/014_37208f58ffa2/architecture/signal_extension.md`) is accurate.
These notes record the DELTAS / extra facts the task-description bullet list did not surface.

## DELTA 1 — CRITICAL: `syscall` import in signal.go becomes UNUSED (must be removed)

signal.go imports `syscall` (import block lines 17-24) and references it in EXACTLY ONE place:
`signal.go:103` — `signal.Notify(h.ch, os.Interrupt, syscall.SIGTERM)`.

After replacing line 103 with `signal.Notify(h.ch, caughtSignals()...)`, NOTHING in signal.go
uses `syscall` anymore → `go build` fails: `imported and not used: "syscall"`.

FIX: remove `"syscall"` from the signal.go import block. (`os` stays — used by os.Stderr /
os.Signal / os.Exit. `os/signal` stays — used by signal.Notify / signal.Stop.) The task
description's step (c) did NOT mention this; it is a hard compile blocker.

Confirmed by grep: `grep -n "syscall\." internal/signal/signal.go` → only line 103.

## DELTA 2 — SIGHUP test MUST be build-tagged (syscall.SIGHUP absent on Windows)

`syscall.SIGHUP` does NOT exist in Go's Windows syscall package. The existing
`signal_test.go` is UN-TAGGED (compiles on both linux/darwin AND windows) and only ever uses
`syscall.SIGINT` / `syscall.SIGTERM` (which DO exist on Windows). A SIGHUP test placed in
signal_test.go would break the Windows build.

FIX: put SIGHUP tests in a NEW `internal/signal/signal_unix_test.go` with build tag
`//go:build !windows`. Precedent: `signal_integration_test.go` is already `//go:build !windows`.
Optionally add `internal/signal/signal_windows_test.go` (`//go:build windows`) asserting
`caughtSignals()` has len 2 and contains SIGTERM (cannot name SIGHUP there).

## DELTA 3 — "comment on line 1" is imprecise

signal_unix.go line 1 is the build tag `//go:build !windows` (line 3 is `package signal`).
There is no file-level doc comment. The meaningful doc comment to update is the
`exitCodeForSignal` doc comment (currently "conventional 128+signum exit code … Used only for
PRE-snapshot signals"). Update it to mention SIGHUP→129. Also add a doc comment to the new
`caughtSignals()` in both files. And signal.go's package comment (lines 1-2) — see DELTA 4.

## DELTA 4 — signal.go has MULTIPLE SIGINT/SIGTERM comments to refresh for accuracy

After SIGHUP joins the caught set, these signal.go comments become slightly stale and should
mention SIGHUP:
- Lines 1-2: package doc comment — "Stagecoach's SIGINT/SIGTERM safety net". Task step (DOCS) calls this out explicitly.
- Line 60: `ch chan os.Signal // buffered; signal.Notify delivers SIGINT/SIGTERM here`
- Line 77: `// Install sets up SIGINT/SIGTERM interception …`
- Line 102: `// SIGTERM is a no-op path on Windows (harmless …)` — reword: the caught set is platform-specific now; on Windows SIGHUP isn't in the set.
- Line 212: `signal.Stop(h.ch) // stop delivering SIGINT/SIGTERM to h.ch` — now also SIGHUP (Unix).

## DELTA 5 — Exit-code semantics (verified against PRD §15.4 + §18.4)

- §15.4 exit table lists 0/1/2/3/124. It does NOT list 129/130/143 — those are the conventional
  128+signum SIGNAL-ABORT codes, documented inline in signal_unix.go's exitCodeForSignal (not in
  the §15.4 table). 129 = 128 + 1 (SIGHUP is signal 1 on Unix). Consistent with 130 (128+2 SIGINT)
  and 143 (128+15 SIGTERM). So `case syscall.SIGHUP: return 129` is correct.
- PRE-snapshot SIGHUP → exitCodeForSignal(SIGHUP) == 129 (was the `default: 1` before this change;
  now an explicit case). POST-snapshot SIGHUP → exit 3 (rescue), identical to SIGINT/SIGTERM,
  because handle() hardcodes Exit(3) on the snapshot-armed branch (signal.go:152). SIGHUP needs NO
  special-case in handle() — it is fully signal-agnostic.
- PRD §18.4 (verified verbatim): "Stagecoach installs a signal.Notify handler for SIGINT, SIGTERM,
  and (Unix) SIGHUP (FR-K3)." + "SIGHUP (Unix; FR-K3) takes the same rescue path … RestoreDefault
  (step 3) disarms all three for the update-ref window."

## DELTA 6 — RestoreDefault disarms SIGHUP for free (confirmed, no change needed)

RestoreDefault (signal.go:207-215) calls `signal.Stop(h.ch)`, which stops delivery of ALL signals
that were in the Notify set — so once caughtSignals() includes SIGHUP, signal.Stop disarms it too.
handle()/run() are signal-agnostic (range over h.ch; forward whatever sig arrived). NO changes to
handle/run/RestoreDefault/RegisterChild/SetSnapshot/etc. Confirmed.

## DELTA 7 — Test pattern to mirror (verified)

`TestHandler_Exit143SIGTERM` (signal_test.go) is the exact template for a SIGHUP exit-code test:
install handler with an `Exit func(code int){...}` recorder, call `h.handle(syscall.SIGTERM)`,
assert exitCode==143. Mirror as `TestHandler_Exit129SIGHUP` (in signal_unix_test.go). There is no
existing direct unit test of `exitCodeForSignal` itself (it's tested through handle()); a small
direct table test in signal_unix_test.go is cleaner and recommended.

## SCOPE BOUNDARIES (sibling subtasks — do NOT implement here)
- **P1.M1.T2.S1**: `signal.Trigger(sig)` exported wrapper (FR-K1 enabler for the watchdog). Do NOT add it here.
- **P1.M2.*** : parent-death watchdog + `no_parent_watchdog` config + arming. Do NOT add.
- **P1.M3.*** : `lock status` subcommand + Busy-message orphan hint. Do NOT add.
- **P1.M4.T2.S1**: README/docs external documentation. This subtask updates ONLY internal code comments (signal.go package doc + exitCodeForSignal doc), NOT README.md / docs/.
