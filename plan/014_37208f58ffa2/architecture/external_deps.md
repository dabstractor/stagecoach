# External Dependencies & Research — FR-K1–K7

## stdlib-only constraint

The codebase has a deliberate convention: `internal/signal` and `internal/lock` import ONLY the
Go standard library (no `golang.org/x/sys`, no third-party packages). This avoids the dependency
bloat that a git plumbing tool should not carry. The new `internal/watchdog` package follows the
same convention.

## Go stdlib APIs used

### prctl(PR_SET_PDEATHSIG) — Linux only
- `syscall.SYS_PRCTL` — available in Go stdlib on Linux (syscall package)
- `PR_SET_PDEATHSIG = 1` — constant from `<sys/prctl.h>`; hardcoded (not exposed by Go's syscall pkg)
- Must be called on `runtime.LockOSThread()`-pinned thread (prctl is per-thread; Go migrates goroutines)
- `syscall.Syscall6(syscall.SYS_PRCTL, 1, uintptr(sig), 0, 0, 0, 0)` — sig is a VALUE, not a pointer
- Race window: verify `os.Getppid()` == originalPpid after arming (parent may have died in fork→prctl gap)

### os.Getppid() — all platforms
- Returns the parent process ID
- On parent death, the child is reparented to init (pid 1) or a subreaper; getppid() CHANGES
- **NOT** `getppid() == 1` (wrong under subreapers: systemd-run, supervisord, docker, some shells)
- Detection signal: parent-pid CHANGE from the value captured at startup

### SIGHUP handling — Unix
- Go's default disposition for SIGHUP is terminate (no cleanup)
- `signal.Notify(ch, syscall.SIGHUP)` catches it (prevents default terminate)
- `signal.Stop(ch)` restores the default disposition
- SIGHUP is delivered when the controlling terminal closes (terminal hangup)
- NOT delivered when a process is detached (nohup/setsid/systemd-run) — that's why the getppid
  watchdog is the complement, not SIGHUP alone

### Orphan detection — read another pid's parent
- **Linux**: read `/proc/<pid>/status`, parse `PPid:\t<N>` line. Pure stdlib (`os.ReadFile` + `bufio.Scanner`).
- **Darwin**: `os/exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()`. Parse the int.
  Uses `os/exec` (stdlib). Works without any libproc/cgo.
- **Windows**: no-op (return false/unknown). FR-K7.

## Platform build tags

| File | Build tag | Contents |
|------|-----------|----------|
| `internal/watchdog/arm_unix.go` | `!windows` | armImpl: prctl (best-effort on Linux) + getppid polling |
| `internal/watchdog/arm_windows.go` | `windows` | armImpl: no-op (FR-K7) |
| `internal/watchdog/pdeathsig_linux.go` | `linux` | armPdeathsig syscall |
| `internal/watchdog/pdeathsig_nonlinux.go` | `!linux` | armPdeathsig no-op (covers darwin + windows) |
| `internal/signal/signal_unix.go` | `!windows` | caughtSignals() includes SIGHUP; exitCodeForSignal SIGHUP=129 |
| `internal/signal/signal_windows.go` | `windows` | caughtSignals() excludes SIGHUP; no SIGHUP exit code |
| `internal/lock/orphan_unix.go` | `!windows` | appearsOrphaned: /proc or ps |
| `internal/lock/orphan_windows.go` | `windows` | appearsOrphaned: always false |

## Verified facts (from codebase research)

1. **signal.Notify line 103** in `internal/signal/signal.go` is the single edit point for SIGHUP.
2. **handle() is unexported** — needs a new exported `Trigger(sig)` wrapper for the watchdog.
3. **OnRescueExit seam** (wired to `lock.ReleaseCurrent` in main.go) fires on BOTH exit paths in
   handle() — the watchdog rides this for free.
4. **RestoreDefault** (signal.go:207) disarms ALL caught signals via `signal.Stop(h.ch)`.
5. **Lock acquired in default_action.go:71** (runDefault), NOT in main.go — watchdog arms there.
6. **NoVerify config field** (config.go:136) is the exact 7-point copy template for NoParentWatchdog.
7. **hook.go command group** is the best template for lock.go — no-op PersistentPreRunE, init() on rootCmd.
8. **processAlive** (lock_unix.go:23) is reusable for lock status liveness check.
9. **LockContents** and **HeldError** are already exported; parseContents/lockPath/lockHash are not.
10. **Exit codes**: Success=0, Error=1, NothingToCommit=2, Rescue=3, Busy=5, Timeout=124. SIGHUP adds 129.

## Open question resolutions

1. **Env var casing**: codebase uses `STAGECOACH_*` (all-caps). Decision: `STAGECOACH_NO_PARENT_WATCHDOG`.
2. **Git config key casing**: codebase uses camelCase for git keys. PRD §16.3 shows `noParentWatchdog`.
   Decision: `stagecoach.noParentWatchdog`.
3. **No CLI flag**: FR-K6 lists only env + git-config. No `--no-parent-watchdog` flag.
4. **Watchdog arming point**: default_action.go post-Acquire (not main.go, which runs before cobra dispatch).
5. **Watchdog package location**: new `internal/watchdog` leaf (not in lock — lock must stay stdlib-only
   without importing signal; not in signal — signal must stay stdlib-only without importing anything).
