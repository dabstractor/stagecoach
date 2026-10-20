# Research: P1.M2.T1.S1 — activity infrastructure: countingWriter + Linux procfs group read-counter poller

Greenfield leaf task: THREE new files in `internal/provider` (activity.go, activity_linux.go,
activity_other.go) implementing FR-A3's two activity channels as STANDALONE primitives. **NO Execute
changes** (S2/P1.M2.T1.S2 reworks Execute; this task only provides `countingWriter` +
`groupReadBytes`). Consumed by S2's idle watcher.

All claims verified against external_procfs_notes.md (fetched 2026-10-20: man-pages 6.19,
docs.kernel.org, torvalds/linux), provider_subsystem.md, and the current source (procgroup_*.go,
executor_test.go). No activity*.go exists (verified — greenfield).

---

## 0. THE DELIVERABLES (3 files, package provider, stdlib-only)

| File | Build tag | Contents |
|------|-----------|----------|
| `internal/provider/activity.go` | (none — all platforms) | `countingWriter` — mutex-wrapped bytes.Buffer io.Writer with an atomic last-write timestamp |
| `internal/provider/activity_linux.go` | `//go:build linux` | `groupReadBytes(pgid int) (uint64, bool)` — procfs group rchar summer |
| `internal/provider/activity_other.go` | `//go:build !linux` | `groupReadBytes(pgid int) (uint64, bool)` — stub returning `(0, false)` |

Plus tests: `activity_test.go` (portable — countingWriter) + `activity_linux_test.go`
(`//go:build linux` — procfs). Mirrors the procgroup_windows_test.go precedent (platform-tagged test
file alongside the platform source).

---

## 1. countingWriter (activity.go) — the stdout/stderr activity channel

The signal: buffer growth during cmd.Wait + the timestamp of the last Write. S2 will replace
executor.go's `var out, errb bytes.Buffer; cmd.Stdout = &out` with countingWriters; Write fires
incrementally as the agent emits → LastWrite() advances → the idle watcher sees activity.

```go
// countingWriter is an io.Writer that wraps a bytes.Buffer behind a mutex and records the time of
// the last successful Write atomically. It is the stdout/stderr ACTIVITY channel for the FR-A3 idle
// watcher (P1.M2.T1.S2): assigned to cmd.Stdout/cmd.Stderr, every agent-emitted chunk both
// accumulates (S2 returns the buffer contents as Execute's stdout/stderr strings) and bumps
// lastWrite (the idle clock). Zero-value ready.
type countingWriter struct {
    mu        sync.Mutex
    buf       bytes.Buffer
    lastWrite atomic.Pointer[time.Time] // time.Now() carries Go's monotonic reading; nil ⇒ never written
}

var _ io.Writer = (*countingWriter)(nil) // compile-time contract

func (w *countingWriter) Write(p []byte) (int, error) {
    w.mu.Lock()
    n, err := w.buf.Write(p)
    w.mu.Unlock()
    if n > 0 {
        now := time.Now() // monotonic-embedded (Go clock); stored whole via Pointer
        w.lastWrite.Store(&now)
    }
    return n, err
}

// LastWrite returns the time of the last successful (n>0) Write, or the zero time.Time if none.
// Compare with time.Since(w.LastWrite()) — monotonic-accurate when both times carry the reading.
func (w *countingWriter) LastWrite() time.Time {
    if tp := w.lastWrite.Load(); tp != nil { return *tp }
    return time.Time{}
}

// Bytes returns a SNAPSHOT (copy) of the accumulated buffer — safe to call concurrently with
// writes; the returned slice is never mutated afterward. S2 stringifies this for the return contract.
func (w *countingWriter) Bytes() []byte {
    w.mu.Lock()
    defer w.mu.Unlock()
    return append([]byte(nil), w.buf.Bytes()...)
}
```

**Monotonic-timestamp decision**: `atomic.Pointer[time.Time]` storing whole `time.Now()` values
preserves Go's embedded monotonic reading — `time.Since(LastWrite())` in S2 is immune to wall-clock
jumps (NTP). (The alternative — `atomic.Int64` UnixNano — is wall-clock; a jump could fake
idleness/activity. The Pointer form is the faithful reading of "atomic last-write monotonic
timestamp". Stdlib since Go 1.19; go.mod targets 1.24/1.25.)
**Zero-time contract**: never-written ⇒ zero time.Time ⇒ `time.Since(zero)` is huge ⇒ reads as
"idle since forever" — correct idle semantics; S2 arms its baseline at Start time and may take
`max(LastWrite, armTime)` (S2's concern, noted for its PRP).

Imports: bytes, io (the assertion), sync, sync/atomic, time.

---

## 2. groupReadBytes (activity_linux.go) — the procfs group-read channel

`func groupReadBytes(pgid int) (uint64, bool)` — sums `/proc/<pid>/io` `rchar` across every live
member of process group `pgid`. `(_, false)` ONLY when /proc is unavailable (ReadDir fails). All
per-member failures (ENOENT reaped / EACCES non-dumpable) are TOLERATED (skip).

**Algorithm** (order of ops is load-bearing — external_procfs_notes §4):

```
1. entries, err := os.ReadDir("/proc"); if err != nil → return (0, false)      // no /proc
2. for each entry whose name is all decimal digits (pid dirs; skips irq/, net/, kernel threads' dirs are digits though):
3.   stat, err := os.ReadFile("/proc/<pid>/stat")   // world-readable (S_IRUGO) — read FIRST
     if err (ENOENT reaped / anything) → continue                                 // member gone, never error
4.   i := bytes.LastIndexByte(stat, ')'); if i < 0 → continue                    // malformed
     fields := bytes.Fields(stat[i+1:])   // token0=state,1=ppid,2=pgrp,3=session
     if len(fields) < 3 → continue
5.   pgrp, err := strconv.Atoi(string(fields[2])); if err → continue
     if pgrp != pgid → continue                   // FILTER BEFORE opening io (kernel threads'
                                                   // io is root-owned → EACCES; only members' io opened)
6.   ioData, err := os.ReadFile("/proc/<pid>/io")  // same-uid dumpable ⇒ readable (PTRACE_MODE_READ uid match)
     if err → continue                            // reaped between stat and io (ENOENT), or non-dumpable (EACCES)
7.   parse the "rchar: <decimal>" line → ParseUint; on absence/parse error → continue
8.   sum += rchar
9. return (sum, true)
```

**Why parse after the LAST ')`** (notes §1): `comm` (field 2) may contain spaces AND parens
(`foo)bar`, `my agent -x`) — splitting the whole line or scanning for the FIRST `)` misaligns every
field. After the last `)`: token 0 = state, 1 = ppid, **2 = pgrp**, 3 = session. (proc(5) itself
doesn't document this; the maintainer-thread guidance: strrchr(')') from the end.)

**rchar semantics** (notes §2): bytes returned by successful read(2)-family calls — pipes/tty/
sockets count, pagecache hits count, failed reads count nothing. Exactly "the agent read more
input / consumed more output" — the activity notion. NOT read_bytes (block-layer; different thing).
Reaped children's ioac folds into the PARENT's rchar — inflates group sums but can only
**false-positive activity within the group, never false-negative** (safe direction for idle kill).
Counters are not atomic (proc_pid_io(5) CAVEATS) — S2's deltas clamp negatives; the summer just sums.

**Permissions** (notes §3): io is S_IRUSR + PTRACE_MODE_READ_FSCREDS; same-uid dumpable processes
pass (no root, no parent-child relation needed). Yama never restricts READ mode. Agent children are
dumpable ⇒ readable.

**Cost** (notes §4): a full sweep over ~1100 pids ≈ 13ms (≈2.6% of a 500ms cycle at 2Hz); Go lower.
Dominant cost is enumerating ALL pids; the <20-member io reads are noise. Justifies the scan design.

**PGID==PID invariant**: procgroup_unix.go's Setpgid makes the child a NEW group leader ⇒ its
PGID == its PID BY KERNEL GUARANTEE — `groupReadBytes(cmd.Process.Pid)` addresses child + ALL
descendants. (The invariant S2's watcher relies on; cite it in the file header.)

Imports (linux): bytes, os, strconv (maybe strings/unicode for digit check — a small helper
`isAllDigits` or strconv.Atoi on the name and treat error as skip).

---

## 3. activity_other.go (!linux) — the stub

```go
//go:build !linux

// groupReadBytes is unavailable off Linux (macOS/Darwin/Windows have no /proc). Returns (0, false):
// the FR-A3 group-read channel reports "no channel", and Execute's idle watcher (P1.M2.T1.S2)
// falls back to the countingWriter channel alone (the hard cap still bounds the run).
// CONTRACT (FROZEN for P1.M2.T1.S2): the Linux build (activity_linux.go, //go:build linux) MUST
// implement the IDENTICAL signature `func groupReadBytes(pgid int) (uint64, bool)`.
func groupReadBytes(pgid int) (uint64, bool) { return 0, false }
```
Mirrors procgroup_windows.go's header CONTRACT style (identical signature across twins).

---

## 4. TESTS

### activity_test.go (portable — countingWriter; runs on ALL CI platforms)
1. **TestCountingWriter_AccumulatesAndStamps**: 3 sequential Writes ("a","bb","ccc"); Bytes() ==
   "abccc"; LastWrite() non-zero and `time.Since(LastWrite()) < 5s`; Write returns (len(p), nil).
2. **TestCountingWriter_ZeroBeforeFirstWrite**: fresh writer — LastWrite().IsZero() == true;
   Bytes() empty.
3. **TestCountingWriter_SnapshotIsIndependent**: Write("x"); snap := Bytes(); Write("y");
   assert string(snap)=="x" (snapshot unmutated by later writes) — the no-race snapshot contract.
4. **TestCountingWriter_ConcurrentWrites** (THE race test — `go test -race` is the gate): 8
   goroutines × 200 Writes of 4 bytes each; wait; total == 8*200*4 bytes; LastWrite() sane
   (non-zero, within the last minute). The -race detector proves the mutex discipline.

### activity_linux_test.go (`//go:build linux` — groupReadBytes)
1. **TestGroupReadBytes_OwnGroupVisible**: `pgid := syscall.Getpgrp()`; `sum, ok :=
   groupReadBytes(pgid)`; assert ok==true (the test's own group is scannable; the test binary has
   done file I/O so sum is typically >0 — assert ok==true as the hard gate; `sum >= 0` trivially).
2. **TestGroupReadBytes_ChildGroupCounterMoves** (the contract's core test): spawn `cat` with
   Setpgid (its own group, PGID==child PID), Stdin = strings.Repeat("x", 8<<20) (8MiB — cat takes
   measurable time reading the pipe), Stdout = io.Discard. After Start: sample `groupReadBytes(pid)`
   — assert ok==true; per external_procfs_notes, exec alone implies loader reads (rchar > 0), and
   the pipe reads accumulate — assert sum > 0 (sample twice with a 10–50ms sleep between and assert
   non-decreasing; ENOENT/exec-timing races tolerated by re-sampling, never by skipping the ok
   assert). Then `cmd.Wait()`; after reap, `groupReadBytes(pid)` must STILL return ok==true (member
   silently gone — sum excludes it) — THE ENOENT-tolerance assertion.
3. **TestGroupReadBytes_BogusPgid**: `groupReadBytes(1<<30)` (no such group) → ok==true, sum==0
   (scan works; empty match set is (0, true), NOT (0, false)).

### Cross-platform build gates (no test file — commands)
`GOOS=linux go build ./internal/provider/` + `GOOS=darwin go build` + `GOOS=windows go build` —
both twins compile with identical signatures; `go vet` per GOOS.

**Test-harness idiom** (executor_test.go): `mustBin(t, "cat")` skip-guard (LookPath + t.Skipf);
real PATH binaries, no tempdir stubs; stdlib testing, "got %q want %q" phrasing.

---

## 5. SCOPE & COORDINATION

- **ONLY the 3 new files + 2 test files in internal/provider/.** NO Execute/executor.go changes
  (S2 reworks Execute to consume these), NO render/manifest/registry changes, NO config, NO docs.
- **Parallel P1.M1.T1.S3** (in-flight): internal/cmd/root.go + config files + docs (idle_timeout
  flags). ZERO overlap with internal/provider/. No file conflicts.
- **S2 (P1.M2.T1.S2)** consumes: `newCountingWriter`/`countingWriter` for cmd.Stdout/Stderr +
  `groupReadBytes(pid)` polled in its watcher loop. The signatures here are S2's contract — do NOT
  drift them (zero-value-ready countingWriter; Write/LastWrite/Bytes; groupReadBytes's (sum, ok)
  semantics with ok==false ONLY for "no /proc").
- **NO PRD.md / tasks.json / prd_snapshot.md** (read-only).

---

## 6. VALIDATION

- `go build ./...` + `GOOS=linux/darwin/windows go build ./internal/provider/`
- `go vet ./internal/provider/...` (+ per-GOOS)
- `gofmt -l internal/provider/activity*.go` (empty)
- `go test ./internal/provider/ -run 'CountingWriter' -v` (all platforms)
- `go test ./internal/provider/ -run 'GroupReadBytes' -v` (Linux CI; skips elsewhere via build tag)
- `go test -race ./internal/provider/...` — THE gate for countingWriter's mutex discipline
- `make test` + `make lint` (golangci-lint; staticcheck/gosimple/govet/errcheck/ineffassign/unused —
  `unused` clean: both symbols are test-read; production reader is S2)
- grep guard: executor.go UNTOUCHED (`git diff --name-only` == the 5 new files only)
