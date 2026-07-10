# Critical Findings — FR-K1–K7 Orphaned-Run Lock Reclamation

## 1. The ONLY unimplemented feature

The project is at v2.6 implementation completeness. The v2.7 PRD revision adds exactly ONE new
feature: **§9.27 FR-K1–K7 (orphaned-run lock reclamation)**. Every other PRD section (v1 core,
v2.0 decomposition, v2.1–v2.6 additions) is already implemented and tested.

The most recent git commits confirm: config-write safety (FR-B8/B9) was the last implementation
work; `fdb0e3d` bumped the PRD text to v2.7 but no code implements FR-K1–K7 yet.

## 2. Current lock model gap

The §18.5 lock model has two states:
- **Dead holder**: `flock` auto-releases on process death; lock file reaped by pid-liveness.
- **Live holder**: pid alive; never reaped (legitimate run).

**Missing third state (the bug)**: a holder whose launcher **closed without killing it** (closing
lazygit, quitting an IDE). The child is reparented to init, its pid stays alive (reaping never
fires), and §18.4's `SIGINT`/`SIGTERM`-only handler receives neither signal (the orphaning parent
delivers neither). The lock file outlives the launcher indefinitely.

## 3. Implementation is additive — no existing behavior changes

FR-K1–K7 is purely additive:
- **Signal handler**: add SIGHUP to the caught set (1 line change via platform helper) + new
  `Trigger` export. `handle()`/`run()`/`RestoreDefault` are signal-agnostic and need zero changes.
- **Lock package**: add exported `Status()` read path + orphan detection helper. Existing
  `Acquire`/`Release`/`reapStaleLocks` are unchanged.
- **Config**: add `NoParentWatchdog` field via 7-point copy from `NoVerify`. No existing fields touched.
- **CLI**: add `lock status` subcommand (new file, follows `hook.go` pattern, no root.go edit).
- **Default action**: add watchdog arming after `lock.Acquire` (gated by config). Existing flow unchanged.
- **Contention message**: reformat `handleLockContention` output (lock path on own line + orphan hint).

## 4. Exact extension points (confirmed by code reading)

| Component | File | Line | Change |
|---|---|---|---|
| signal.Notify | internal/signal/signal.go | 103 | `signal.Notify(h.ch, caughtSignals()...)` |
| caughtSignals (Unix) | internal/signal/signal_unix.go | new | Returns `{Interrupt, SIGTERM, SIGHUP}` |
| caughtSignals (Windows) | internal/signal/signal_windows.go | new | Returns `{Interrupt, SIGTERM}` |
| exitCodeForSignal | internal/signal/signal_unix.go | ~25 | Add `case SIGHUP: return 129` |
| Trigger export | internal/signal/signal.go | new | `func Trigger(sig os.Signal) { if h := active.Load(); h != nil { h.handle(sig) } }` |
| Config field | internal/config/config.go | ~142 | Add `NoParentWatchdog bool toml:"no_parent_watchdog"` |
| Defaults | internal/config/config.go | ~214 | Add `NoParentWatchdog: false` |
| fileGeneration | internal/config/file.go | ~68 | Add field |
| materialize | internal/config/file.go | ~299 | Add only-true-propagates copy |
| overlay | internal/config/file.go | ~363 | Add only-true-propagates copy |
| loadEnv | internal/config/load.go | ~326 | Add `STAGECOACH_NO_PARENT_WATCHDOG` block |
| loadGitConfig | internal/config/git.go | ~186 | Add `stagecoach.noParentWatchdog` block |
| Lock Status export | internal/lock/lock.go | new | `func Status(repoPath) (path, contents, alive, orphan, err)` |
| Orphan detection | internal/lock/orphan_unix.go | new | `func appearsOrphaned(pid int) bool` |
| Orphan detection (Win) | internal/lock/orphan_windows.go | new | Always false |
| Watchdog package | internal/watchdog/ | new | Arm/Stop + prctl + getppid polling |
| Lock subcommand | internal/cmd/lock.go | new | `lock status` cobra cmd |
| Watchdog arming | internal/cmd/default_action.go | ~78 | After `defer locker.Release()` |
| Busy message | internal/cmd/default_action.go | ~314 | Lock path on own line + orphan hint |

## 5. Dependency graph

```
P1.M1.T1.S1 (SIGHUP)     ──────────────────────────────────────┐
P1.M1.T2.S1 (Trigger)    ───────┐                               │
P1.M2.T1.S1 (config)     ──┐    │                               │
P1.M3.T1.S1 (lock status  │    │                                │
                   API)   │    │                                │
                          │    │                                │
P1.M2.T2.S1 (watchdog)   ←┼────┘  (needs Trigger)               │
P1.M2.T2.S2 (arming)     ←┼───┐                                   │
                          │   │ (needs config + watchdog)         │
P1.M3.T2.S1 (lock cmd)   ←┼───┼──────────────────┐ (needs Status) │
P1.M3.T3.S1 (busy msg)   ←┼───┼──────┐ (needs orphan)             │
                          │   │      │                             │
P1.M4.T1.S1 (e2e tests)  ←─────────────────────────────────────────┘
P1.M4.T2.S1 (docs)       ←─────────────────────────────────────────┘
```

## 6. Key design decisions

1. **New `internal/watchdog` package** (not in lock or signal): avoids breaking the stdlib-only
   invariant of both. Watchdog imports `internal/signal` (one-directional).
2. **`signal.Trigger(sig)` export**: reuses `handle()` for the full rescue path. The single new
   signal package API the watchdog needs.
3. **getppid polling as the primary mechanism** (both Linux and Darwin): reliable, subreaper-safe.
   prctl is a Linux-only best-effort fast path.
4. **Env var**: `STAGECOACH_NO_PARENT_WATCHDOG` (all-caps, matching codebase convention).
5. **Git config key**: `stagecoach.noParentWatchdog` (camelCase, matching codebase convention).
6. **No CLI flag**: FR-K6 lists only env + git-config (unlike no_verify/no_color which have flags).
7. **Windows**: watchdog + orphan detection are no-ops (FR-K7). `caughtSignals()` omits SIGHUP.
