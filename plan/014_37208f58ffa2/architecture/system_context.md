# System Context — Orphaned-Run Lock Reclamation (FR-K1–K7)

## Current state

Stagecoach v2.7 PRD is implemented **except** for §9.27 (FR-K1–K7: orphaned-run lock
reclamation). The v1–v2.6 feature set is complete: single-commit core, multi-commit
decomposition, multi-turn fallback, hook mode, hook execution on commit path, work-description
mode, payload exclusions, message shaping, tool integrations, config bootstrap/versioning, and
the per-repo run lock (FR52 §18.5) with stale-file reaping.

The most recent commits (`18b3b21`, `f9ac039`, `fdb0e3d`) added config-write safety (FR-B8/B9)
and bumped the PRD to v2.7. The **only** unimplemented feature is the v2.7 headline: FR-K1–K7.

## What FR-K1–K7 adds

A third lock state the §18.5 model does not contemplate: a holder whose **launcher closed
without killing it** (closing the lazygit TUI, quitting an IDE, a detaching terminal). The child
is **reparented to init and keeps running**: its `pid` stays alive (so pid-liveness reaping never
fires) and §18.4's `SIGINT`/`SIGTERM`-only handler receives neither signal, so the run never
reaches the exit path and the lock file outlives the launcher. The fix is **self-termination,
never contender-side force-breaking**.

## Seven sub-requirements

| FR  | Requirement | Implementation surface |
|-----|-------------|----------------------|
| K1  | Parent-death self-watchdog: on parent death, route through rescue + lock-release exit path | New `internal/watchdog` package; arming in `default_action.go` post-Acquire |
| K2  | Detection by parent-pid change (reparenting), NOT `getppid()==1` | `os.Getppid()` polling + Linux `prctl(PR_SET_PDEATHSIG)` fast path |
| K3  | `SIGHUP` joins caught signals `{SIGINT, SIGTERM, SIGHUP}` | `internal/signal/signal.go` line 103; `signal_unix.go` exit code |
| K4  | `stagecoach lock status` — read-only diagnostic | New `internal/cmd/lock.go`; exported read path in `internal/lock` |
| K5  | Busy message: lock path on own line, flag orphaned holder | `internal/cmd/default_action.go` `handleLockContention` |
| K6  | Escape hatch: `stagecoach.no_parent_watchdog` opt-out | Config field (7-point copy from `NoVerify`) |
| K7  | Unix-only watchdog; Windows unchanged | Build-tagged platform files |

## Package dependency graph (new components)

```
cmd/stagecoach/main.go
  └─ signal.Install(OnRescueExit: lock.ReleaseCurrent)   [existing, unchanged]

internal/cmd/default_action.go
  ├─ lock.Acquire(repoDir)           [existing]
  ├─ watchdog.Arm(ctx, interval)     [NEW — gated by cfg.NoParentWatchdog]
  └─ defer locker.Release()          [existing]

internal/watchdog/                   [NEW package]
  ├─ arm_unix.go   (!windows)        — getppid polling + prctl fast path (Linux)
  ├─ arm_windows.go (windows)         — no-op stub
  ├─ pdeathsig_linux.go (linux)       — prctl(PR_SET_PDEATHSIG) syscall
  ├─ pdeathsig_nonlinux.go (!linux)   — no-op twin
  └─ watchdog.go                      — Arm/Stop orchestration
     └─ imports internal/signal       — calls signal.Trigger(SIGTERM) on parent death

internal/signal/signal.go            [MODIFIED]
  ├─ signal.Notify now uses caughtSignals() [NEW platform helper]
  ├─ signal_unix.go: caughtSignals() returns {SIGINT, SIGTERM, SIGHUP}
  ├─ signal_windows.go: caughtSignals() returns {SIGINT, SIGTERM}
  ├─ exitCodeForSignal: SIGHUP → 129
  └─ Trigger(sig) [NEW export] — programmatic rescue path entry

internal/lock/lock.go                [MODIFIED]
  └─ Status(repoPath) [NEW export] — read-only path, orphan detection helper

internal/cmd/lock.go                 [NEW — lock status subcommand]
  └─ follows internal/cmd/hook.go pattern
```

## Invariants preserved

1. **FR52 "never force-break"**: the watchdog is the SAME process abandoning its own work, never
   a contender breaking another's lock.
2. **stdlib-only signal package**: `internal/signal` imports NO stagecoach packages. The watchdog
   imports signal one-directionally (`signal.Trigger`), not vice versa.
3. **stdlib-only lock package**: `internal/lock` stays stdlib-only. Orphan detection uses
   `/proc/<pid>/status` (Linux) or `ps -o ppid=` (Darwin) via stdlib `os`/`os/exec`.
4. **Watchdog no-ops past RestoreDefault**: the `update-ref` window where abandonment would lose
   committed work. Routing through `signal.Trigger` → `handle()` gets the `stopped` guard for free.
5. **SIGHUP independent of watchdog opt-out**: SIGHUP handling (FR-K3) and `lock status` (FR-K4)
   are always on; only the polling/prctl watchdog (FR-K1) is gated by `no_parent_watchdog`.
