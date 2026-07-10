# Watchdog + Config Architecture — FR-K1/K2/K6/K7

## The parent-death watchdog (FR-K1/K2/K7)

### Design: new `internal/watchdog` package

A new leaf package (not in lock, not in signal) dedicated to the single concern of detecting
parent death and routing through the signal handler's exit path.

**Package dependency**: `internal/watchdog` imports `internal/signal` (for `signal.Trigger`).
One-directional — signal never imports back. No cycle.

### File layout
```
internal/watchdog/
├── watchdog.go              — Arm(ctx, interval), Stop(); polling goroutine orchestration
├── arm_unix.go              — //go:build !windows: armImpl() = prctl(Linux) + getppid poll
├── arm_windows.go           — //go:build windows: armImpl() = no-op stub
├── pdeathsig_linux.go       — //go:build linux: armPdeathsig(sig) syscall
└── pdeathsig_nonlinux.go    — //go:build !linux: armPdeathsig(sig) = no-op
```

### Arm function
```go
// Arm starts the parent-death watchdog. On parent death it calls signal.Trigger(SIGTERM),
// which routes through the signal handler's rescue + lock-release exit path. Unix-only;
// no-op on Windows (FR-K7). The polling interval defaults to 1s (FR-K2). Call after lock
// acquire; Stop() (or ctx cancel) terminates the goroutine.
func Arm(ctx context.Context, interval time.Duration) { ... }
```

### Detection: parent-pid CHANGE, not getppid()==1 (FR-K2)

1. Record `originalPpid := os.Getppid()` at Arm time (startup).
2. Poll `os.Getppid()` at `interval` (default ~1s).
3. If `os.Getppid() != originalPpid` → parent changed (reparented to init/subreaper) → fire.
4. On fire: `signal.Trigger(syscall.SIGTERM)` — routes through handle() which:
   - Checks `stopped` (no-op past RestoreDefault — the update-ref window)
   - Cancels the generation context
   - Runs rescue if snapshot armed (prints TREE_SHA + recovery recipe)
   - Calls `OnRescueExit()` (= `lock.ReleaseCurrent`) — removes lock file
   - Exits

### Linux fast path: prctl(PR_SET_PDEATHSIG)

On Linux, `prctl(PR_SET_PDEATHSIG, SIGTERM)` makes the kernel deliver a real SIGTERM when the
parent dies. SIGTERM is already in the caught signal set, so it flows through `run`→`handle`
naturally — no `signal.Trigger` needed for the kernel-delivered path.

**Critical constraints** (from research):
- `prctl` is **per-thread**: must run on a `runtime.LockOSThread()`-pinned thread. The Go runtime
  migrates goroutines between OS threads; if the prctl'd thread is retired, the setting is lost.
- Best-effort: set as early as possible. There's a fork→prctl race window where the parent could
  die before prctl is called — the `getppid()` poll is the reliable fallback.
- After arming prctl, verify `os.Getppid() == originalPpid` to close the race.
- The polling goroutine ALWAYS runs (on Linux too) as the fallback for the race and edge cases.

```go
// pdeathsig_linux.go
func armPdeathsig(sig syscall.Signal) error {
    runtime.LockOSThread()
    defer runtime.UnlockOSThread()
    _, _, errno := syscall.Syscall6(syscall.SYS_PRCTL,
        uintptr(1),           // PR_SET_PDEATHSIG
        uintptr(sig),         // value, NOT pointer
        0, 0, 0, 0)
    if errno != 0 { return errno }
    return nil
}
```

```go
// pdeathsig_nonlinux.go
func armPdeathsig(sig syscall.Signal) error { return nil }
```

### Arming point: default_action.go post-Acquire

The lock is acquired in `runDefault` (default_action.go:71), NOT in main.go. The watchdog must
arm after `lock.Acquire` succeeds and be gated by `cfg.NoParentWatchdog`:

```go
// default_action.go, after `defer locker.Release()`:
if !cfg.NoParentWatchdog {
    watchdog.Arm(ctx, 1*time.Second)  // FR-K1/K2; best-effort, no error return
}
```

### Stop / disarm

The watchdog goroutine exits when:
1. The ctx is canceled (generation finished, success or rescue).
2. Parent death fires (calls signal.Trigger which exits the process).

No explicit Stop() needed if the ctx is the signal-aware ctx from main.go — when the process
exits (normally or via rescue), the goroutine dies with it. But for library use (pkg/stagecoach
without signal.Install), Stop() allows clean teardown.

## FR-K6: no_parent_watchdog config field

### Pattern: exact 7-point copy from NoVerify

`NoVerify` is the template — a plain `bool`, `toml:"no_verify"`, only-true-propagates in
file/overlay layers, DIRECT set in env (can be false). The new field follows identically.

| Touch point | File | NoVerify reference | NoParentWatchdog addition |
|---|---|---|---|
| Config struct field | config.go:136 | `NoVerify bool toml:"no_verify"` | `NoParentWatchdog bool toml:"no_parent_watchdog"` |
| Defaults() | config.go:214 | `NoVerify: false` | `NoParentWatchdog: false` |
| fileGeneration struct | file.go:68 | `NoVerify bool toml:"no_verify"` | `NoParentWatchdog bool toml:"no_parent_watchdog"` |
| materialize() | file.go:298 | `if g.NoVerify { c.NoVerify = true }` | `if g.NoParentWatchdog { c.NoParentWatchdog = true }` |
| overlay() | file.go:362 | `if src.NoVerify { dst.NoVerify = true }` | `if src.NoParentWatchdog { dst.NoParentWatchdog = true }` |
| loadEnv() | load.go:319 | `STAGECOACH_NO_VERIFY` | `STAGECOACH_NO_PARENT_WATCHDOG` |
| loadGitConfig() | git.go:180 | `stagecoach.noVerify` | `stagecoach.noParentWatchdog` |

### Key decisions

1. **Env var spelling**: the codebase uses ALL-CAPS `STAGECOACH_*` for env vars (every existing
   var — `os.LookupEnv("STAGECOACH_NO_VERIFY")`, etc.). The PRD writes `stagecoach_NO_PARENT_WATCHDOG`
   but the PRD also writes `stagecoach_NO_VERIFY` for the existing var. The actual code uses
   `STAGECOACH_NO_VERIFY`. **Decision: use `STAGECOACH_NO_PARENT_WATCHDOG` (all-caps) to match
   the codebase convention.** The PRD's lowercase prefix is conceptual notation, not literal.

2. **Git config key spelling**: the PRD FR-K6 text says `stagecoach.no_parent_watchdog` (snake_case)
   but §16.3 config example shows `noParentWatchdog` (camelCase). The codebase convention for all
   multi-word git keys is camelCase (`noVerify`, `autoStageAll`, `maxDiffBytes`).
   **Decision: use `stagecoach.noParentWatchdog` (camelCase) to match the codebase convention and
   the PRD's own §16.3 example.**

3. **NO CLI flag**: FR-K6 lists only env + git-config + file — no `--no-parent-watchdog` flag.
   This differs from `no_verify`/`no_color` (which have flags). Respect the PRD: no flag registration
   in root.go, no loadFlags entry.

4. **Bootstrap config template**: the generated config should document the option.
   `internal/config/bootstrap.go`'s template should add a commented `noParentWatchdog = false` line.
