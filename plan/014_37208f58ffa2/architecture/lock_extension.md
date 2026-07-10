# Lock System Architecture — Extension Points for FR-K1/K4/K5

## Current lock package (internal/lock/lock.go)

### Types
```go
// lock.go:38-46
type Locker struct {
    file      *os.File
    path      string
    pid       string
    hostname  string
    repo      string
    timestamp string
}

// lock.go:47-50 (EXPORTED)
type LockContents struct {
    Pid, Hostname, Repo, Timestamp, Snapshot string
}

// lock.go:54-58 (EXPORTED)
type HeldError struct {
    Contents LockContents
    Path     string
}
```

### Singleton
```go
// lock.go:69
var current atomic.Pointer[Locker]
```

### Acquire (lock.go:73-116)
Key ordering: `MkdirAll` → `OpenFile(O_CREATE|O_RDWR)` → `flock(LOCK_EX|LOCK_NB)` → on
`EWOULDBLOCK` return `*HeldError` → on success: build `Locker`, `writeContents("")`,
`current.Store(l)`, `reapStaleLocks(dir)`.

**Watchdog arming point**: right after `current.Store(l)` (line ~114). The holder now owns the
lock and should arm the parent-death watchdog.

### Release / ReleaseCurrent (lock.go:139-150, 208-217)
```go
func (l *Locker) Release() {
    if l == nil || l.file == nil { return }
    l.file.Close()          // release flock FIRST
    path := l.path
    l.file = nil
    if current.Load() == l { current.Store(nil) }
    os.Remove(path)         // best-effort cleanup
}

func ReleaseCurrent() {
    if l := current.Load(); l != nil { l.Release() }
}
```
Critical ordering: close fd (release flock) BEFORE remove file. The watchdog reuses
`ReleaseCurrent` via the signal handler's `OnRescueExit` seam.

### Unexported helpers that lock status needs
```go
// lock.go:259-279 — parseContents: parses key=value file into LockContents
func parseContents(data []byte) LockContents { ... }

// lock.go:289-300 — lockHash: returns (canonicalPath, sha256Hex)
func lockHash(repoPath string) (string, string) { ... }

// lock.go:302-310 — lockPath: returns the lock file path
func lockPath(repoPath string) (string, error) { ... }
```

### processAlive (lock_unix.go:23-46)
```go
func processAlive(pid int, hostname string) bool {
    host, _ := os.Hostname()
    if hostname == "" || hostname != host { return true }  // foreign host → don't reap
    err := syscall.Kill(pid, 0)
    if err == nil { return true }                           // alive
    if errors.Is(err, syscall.EPERM) { return true }        // alive, different user
    return false                                            // ESRCH → dead
}
```
**Reused by lock status for holder liveness**. On Windows (lock_windows.go) always returns true.

## FR-K4: Lock status read path (NEW exports)

The lock package currently has NO exported read-only path. `lock status` must NOT call `Acquire`
(that takes the lock). New exports needed:

```go
// Status returns the parsed lock-file state for repoPath. Read-only: never acquires,
// never breaks (FR52 preserved). Returns (path, contents, alive, orphan, err).
// path=="" / ok==false → no lock held for this repo.
func Status(repoPath string) (path string, contents LockContents, alive bool, orphan bool, err error)
```

Implementation wraps: `lockPath(repoPath)` → `os.ReadFile(path)` → `parseContents(data)` →
`processAlive(pid, hostname)` → orphan check.

### Orphan detection (NEW — detecting ANOTHER pid's parent)
`getppid()` returns OUR parent. Detecting the HOLDER's parent requires:
- **Linux**: read `/proc/<pid>/status`, parse the `PPid:\t<N>` field. If N == 1 → orphaned.
- **Darwin**: `os/exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()`, parse int. If == 1 → orphaned.
- **Windows**: report false/"unknown" (FR-K7).

This is net-new platform code. Create:
- `internal/lock/orphan_unix.go` (`//go:build !windows`): `func appearsOrphaned(pid int) bool`
- `internal/lock/orphan_windows.go` (`//go:build windows`): `func appearsOrphaned(pid int) bool { return false }`

The `Status` function calls `appearsOrphaned` only when the holder is alive (a dead pid's
orphan status is irrelevant).

## FR-K5: Busy message reformat

### Current contention message (default_action.go:314-322)
```go
fmt.Fprintf(stderr,
    "stagecoach: another stagecoach run is already in progress on %s (pid %s on %s). "+
        "Your newly-staged changes will remain staged — re-run stagecoach after it finishes. Lock: %s.\n",
    repo, pid, hostname, heldErr.Path)
return exitcode.New(exitcode.Busy, nil)
```

### FR-K5 target format
```
stagecoach: another stagecoach run is already in progress on <repo> (pid <N> on <host>).
Your newly-staged changes will remain staged — re-run stagecoach after it finishes.

Lock: <path>

[if orphaned: "The holder's launcher has exited — it is orphaned and holding this lock uselessly.
You may safely `kill <N>` or `rm <path>` to clear it. See `stagecoach lock status`."]
```

Changes:
1. Lock path on its OWN line (not buried mid-sentence) — scriptable.
2. When the holder appears orphaned (reuse FR-K4's orphan check), add the orphan hint.
3. Preserve the fallback diagnostics (`<unknown>` for empty pid/hostname, `an unknown repo` for
   empty repo — lines 298-312 in default_action.go).

## Lock file location
- `$XDG_RUNTIME_DIR/stagecoach/locks/<sha256hex>.lock` (preferred — tmpfs)
- `$XDG_CACHE_HOME/stagecoach/locks/<sha256hex>.lock` (fallback)
- `~/.cache/stagecoach/locks/<sha256hex>.lock` (last resort)
- `<sha256hex>` = sha256 of the repo's canonical absolute path
- File format: `key=value` per line (`pid=`, `hostname=`, `repo=`, `timestamp=`, `snapshot=`)

## Lock states (the three cases)
1. **Dead holder** — `flock` auto-released on process death; `reapStaleLocks` removes the inert
   file on next `Acquire` (pid is ESRCH). **Already handled.**
2. **Live legitimate holder** — pid alive, never reaped. **Already handled.**
3. **Orphaned-but-alive (THE GAP)** — launcher closed without killing; child reparented to init,
   pid alive, holds flock forever. No signal delivered. **FR-K1 closes this via self-termination.**
