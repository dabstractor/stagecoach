name: "P1.M2.T1.S1 — activity infrastructure: countingWriter + Linux procfs group read-counter poller (FR-A3 channels)"
description: >
  Greenfield leaf task: THREE new files in internal/provider (stdlib-only, package provider)
  implementing FR-A3's two activity channels as STANDALONE primitives with NO Execute changes —
  (1) activity.go (all platforms): countingWriter, a mutex-wrapped bytes.Buffer io.Writer that
  records an atomic last-write timestamp (atomic.Pointer[time.Time] preserving Go's monotonic
  reading) on every Write and exposes LastWrite() + a race-free Bytes() snapshot; (2)
  activity_linux.go (//go:build linux): groupReadBytes(pgid int) (uint64, bool) — scans
  /proc/[0-9]*/stat, parses each AFTER THE LAST ')' (comm may contain spaces/parens), filters
  pgrp==pgid BEFORE opening io (kernel threads' io is root-owned), sums members' /proc/<pid>/io
  rchar; per-member ENOENT/EACCES tolerated; (_, false) ONLY when /proc is unavailable; (3)
  activity_other.go (//go:build !linux): the (0,false) stub (mirrors the procgroup_*.go
  platform-split CONTRACT header). Plus activity_test.go (countingWriter incl. the -race
  concurrency gate) + activity_linux_test.go (own-group visibility, cat-child counter-moves,
  bogus-pgid empty-match, post-reap ENOENT tolerance) + per-GOOS build gates. Consumed by
  P1.M2.T1.S2 (the Execute idle watcher) — signatures are its contract. NO executor.go/render/
  manifest/config/docs changes (S2 owns Execute; parallel S3 of M1 owns cmd/config/docs).

---

## Goal

**Feature Goal**: Provide the two FR-A3 activity-detection primitives the idle watcher
(P1.M2.T1.S2) will consume: (a) a concurrency-safe stdout/stderr writer whose buffer growth +
last-write timestamp ARE the agent-output activity signal, and (b) a Linux procfs poller that sums
the process group's `rchar` (bytes returned by successful read-family syscalls — pipes/tty/sockets
count, pagecache hits count, failed reads count nothing) so an agent silently spinning on INPUT
without emitting output is still detected as active.

**Deliverable** (5 new files, `package provider`, stdlib-only):
1. **internal/provider/activity.go** — `countingWriter` (zero-value-ready): `Write` (mutex +
   atomic lastWrite stamp), `LastWrite() time.Time` (zero if never written),
   `Bytes() []byte` (snapshot copy), + `var _ io.Writer = (*countingWriter)(nil)` assertion.
2. **internal/provider/activity_linux.go** (`//go:build linux`) — `groupReadBytes(pgid int)
   (uint64, bool)`: /proc scan → stat parse-after-last-`)` → pgrp filter → io rchar sum.
3. **internal/provider/activity_other.go** (`//go:build !linux`) — the identical-signature stub
   returning `(0, false)`.
4. **internal/provider/activity_test.go** — countingWriter tests (accumulate/stamp, zero-state,
   snapshot independence, 8-goroutine concurrency race gate).
5. **internal/provider/activity_linux_test.go** (`//go:build linux`) — procfs tests (own-group
   visible, cat-child counter moves, bogus pgid → (0,true), post-reap ENOENT tolerated).

**Success Definition**:
- `(&countingWriter{}).Write` is safe under `go test -race` with concurrent writers; Bytes()
  returns the exact accumulated bytes; LastWrite() is monotonic-accurate (time.Since works).
- On Linux: `groupReadBytes(syscall.Getpgrp())` → `ok==true`; a Setpgid `cat` child fed 8MiB shows
  `sum > 0` while running; after `cmd.Wait()` (reaped) it STILL returns `ok==true` (member gone,
  silently excluded — the ENOENT tolerance); `groupReadBytes(<bogus>)` → `(0, true)`.
- Off Linux: `groupReadBytes` compiles and returns `(0, false)` (no channel; S2's watcher falls
  back to countingWriter alone).
- `go build ./...` + `GOOS=linux/darwin/windows go build ./internal/provider/` all clean;
  `go test -race ./internal/provider/...` green; `make test`/`make lint` pass; `gofmt -l` empty.
- `executor.go` is UNTOUCHED (S2 owns it); only the 5 new files exist in the diff.

## User Persona (if applicable)

**Target User**: The Execute idle watcher (P1.M2.T1.S2) — the sole consumer. End users never touch
these primitives.

**Use Case**: S2 assigns a countingWriter to `cmd.Stdout`/`cmd.Stderr` (buffer growth = the agent
emitted output; lastWrite = when) and polls `groupReadBytes(childPID)` every 500ms (rchar growth =
the agent or a grandchild is READING — input-side activity invisible to the stdout channel).
Together they implement FR-A3's activity definition; a third signal (event-stream heartbeat) is P2.

**User Journey**: agent runs → emits chunks (countingWriter stamps) and/or reads input (rchar
rises) → S2's watcher sees either channel move → the idle clock resets → a productive-but-slow
agent is never falsely killed (FR-A2); a silent one IS killed at idle_timeout.

**Pain Points Addressed**: FR25/FR-A1 — the v1–v3.4 elapsed-time-only kill can't distinguish hung
from productive-slow. These two channels are the raw signals that make the distinction possible.

## Why

- **FR-A3 (§9.30)**: activity = observable byte movement (output AND input side) + heartbeat. This
  task delivers the byte-movement channels as clean, tested, platform-split primitives so S2's
  Execute rework is a thin composition rather than a monolith.
- **Grounding**: external_procfs_notes.md (fetched 2026-10-20 against man-pages 6.19 +
  docs.kernel.org + torvalds/linux) verifies every procfs semantic: the last-`)` parse rule, rchar
  counting rules, same-uid permissions, reaped-child ioac folding (safe direction — only
  false-positives activity within the group, never false-negatives), the ~13ms/~1100-pid sweep cost
  (≈2.6% duty at 2Hz).
- **Bounded scope**: a leaf task — 3 source files + 2 test files, zero edits to existing code. The
  signatures are S2's frozen contract.

## What

**User-visible behavior**: None (internal primitives; no consumer yet — S2 wires them).

**Technical change** (verbatim code in the Blueprint):
- `countingWriter`: sync.Mutex-guarded `bytes.Buffer` + `atomic.Pointer[time.Time]` lastWrite;
  `Write`/`LastWrite`/`Bytes`.
- `groupReadBytes(pgid)` (Linux): ReadDir /proc → numeric dirs → ReadFile stat →
  `bytes.LastIndexByte(stat, ')')` → `bytes.Fields` → fields[2] == pgid filter → ReadFile io →
  parse `rchar:` line → sum. `(0, false)` only on ReadDir("/proc") failure.
- `groupReadBytes(pgid)` (!Linux): `(0, false)` stub.

### Success Criteria
- [ ] activity.go compiles on ALL platforms; countingWriter satisfies io.Writer (compile assert)
- [ ] Write is mutex-safe; lastWrite stored atomically (Pointer to monotonic-embedded time.Now)
- [ ] Bytes() returns an independent snapshot (later writes never mutate a returned slice)
- [ ] LastWrite() zero before any write; non-zero + recent after
- [ ] groupReadBytes: stat parsed AFTER the LAST ')' (comm with spaces/parens can't misalign)
- [ ] groupReadBytes: pgrp filter applied BEFORE opening any io file (kernel threads skipped cheaply)
- [ ] groupReadBytes: per-member stat/io read failures (ENOENT/EACCES/unparseable) skipped, never fatal
- [ ] groupReadBytes: returns (0, false) ONLY when /proc ReadDir fails; empty match set = (0, true)
- [ ] activity_other.go stub compiles on darwin/windows with the IDENTICAL signature
- [ ] both twins carry the CONTRACT header clause (mirror procgroup_*.go)
- [ ] executor.go untouched; `go build ./...`, per-GOOS builds, `-race` tests, make test/lint green

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim source for all three files, the exact procfs parse algorithm with its
load-bearing ordering (stat-first/filter/io-second, last-`)` split), the monotonic-timestamp design
decision, the test matrix with the race/reap-tolerance gates, the platform-split CONTRACT header
style (verbatim from procgroup_unix.go), the mustBin skip-guard idiom, and the explicit scope
fences (executor.go belongs to S2).

### Documentation & References

```yaml
# MUST READ — the authoritative research (verbatim code + algorithm + test matrix)
- docfile: plan/022_856008a8cbb5/P1M2T1S1/research/findings.md
  why: "§1 the verbatim countingWriter (incl. the atomic.Pointer[time.Time] monotonic decision);
        §2 the verbatim groupReadBytes algorithm with the ORDER-OF-OPS rationale (stat-first,
        filter-before-io); §3 the stub; §4 the full test matrix; §5 scope/coordination; §6 validation."
  critical: "§2: the per-member tolerance rules (ENOENT reaped / EACCES non-dumpable / unparseable →
             skip, NEVER (0,false)) and the (0,false)-only-on-ReadDir-failure contract — S2's watcher
             keys off ok==false meaning 'no channel', so an empty match set MUST be (0,true). §1: the
             zero-time contract (never-written ⇒ idle-since-forever semantics)."

# MUST READ — the procfs API semantics (verified external research, dated)
- docfile: plan/022_856008a8cbb5/architecture/external_procfs_notes.md
  why: "Every semantic the parser depends on: §1 stat layout + the last-`)` rule (comm may contain
        spaces/parens — first-`)` or whole-line split misaligns every field); §2 rchar semantics
        (successful read-family bytes; pagecache counts; failed reads count nothing; read_bytes is
        the DIFFERENT block counter) + the reaped-child ioac-fold (inflates, never false-negatives —
        safe direction) + non-atomic-counter caveat (S2 clamps deltas); §3 same-uid dumpable
        permissions (PTRACE_MODE_READ uid-match; Yama never restricts READ mode); §4 the ~13ms
        sweep cost + kernel-thread io EACCES (hence stat-first ordering); §1 lifetime races
        (reaped pid's stat open = ENOENT; re-identify members every tick; never cache FDs)."
  critical: "§1: parse after the LAST ')' — the #1 parsing trap. §4: enumerate ALL pids, parse stat
             FIRST, filter by pgrp, only THEN open io for matching members (kernel threads' io is
             root-owned → EACCES if opened blindly)."

# MUST READ — the platform-split template (verbatim header style to mirror)
- file: internal/provider/procgroup_unix.go
  why: "THE file to mirror for activity_linux.go/activity_other.go: `//go:build !windows` on line 1,
        `package provider`, then the doc comment WITH the frozen CONTRACT clause ('the Windows build
        (procgroup_windows.go, //go:build windows) MUST implement the IDENTICAL signature … Do NOT
        change this signature without coordinating'). Your twins use //go:build linux and
        //go:build !linux with the same clause naming S2 (P1.M2.T1.S2) as the consumer."
  pattern: "Build tag FIRST line; blank line; package clause; imports; a doc comment that (a) explains
            the mechanism, (b) cites the PRD/invariant (here: Setpgid ⇒ PGID==PID — the invariant
            groupReadBytes(pgid) relies on, since the watcher passes cmd.Process.Pid), (c) freezes
            the twin signature via a CONTRACT paragraph."
  gotcha: "Also note procgroup_windows_test.go exists — the precedent for a platform-TAGGED TEST file
           (activity_linux_test.go) alongside the platform source."

# CONFIRMING — the executor these will plug into (READ-ONLY — S2 owns it)
- file: internal/provider/executor.go
  why: "Execute (line 65) currently uses `var out, errb bytes.Buffer; cmd.Stdout = &out` — fully
        buffered, nothing incremental. S2 swaps these for countingWriters and adds the polling loop;
        THIS TASK DOES NOT TOUCH executor.go. Reading it confirms the seam (cmd.Stdout assignment,
        signal.RegisterChild(cmd.Process.Pid) — the same PID the watcher will pass to groupReadBytes)."
  critical: "executor.go is FROZEN for this task. `git diff --name-only` must show ONLY the 5 new files."

# MUST READ — the test idiom to mirror
- file: internal/provider/executor_test.go
  why: "`mustBin(t, names...)` (line ~17): t.Helper() + exec.LookPath + t.Skipf — the skip-guard for
        real-PATH binaries (use for `cat`). Stdlib testing; 'got %q want %q' phrasing; table tests
        where natural; security asserts in verbose tests. The -race suite is the project standard
        (make test runs it) — the countingWriter concurrency test is designed for it."
  critical: "Tests use REAL binaries from PATH (cat), never tempdir stub scripts. The Linux procfs
             tests additionally carry `//go:build linux` so they compile ONLY on Linux CI (skipped
             implicitly elsewhere — no runtime.GOOS guard needed when the file is build-tagged)."

# CONTEXT — the FR-A3 spec (what these channels serve)
- docfile: plan/022_856008a8cbb5/prd_snapshot.md
  why: "§9.30 FR-A1–A5: the idle/hard-cap deadline model and the ACTIVITY DEFINITION (observable
        byte movement on either side + heartbeat). FR-A3 names these two channels; this task is its
        primitive layer. FR-A2 (productive-slow must survive) is the motivation for the input-side
        (rchar) channel."
  section: "§9.30 (FR-A1–A5)"

# CONTEXT — the parallel sibling + the consumer (NO overlap; signatures are their contract)
- docfile: plan/022_856008a8cbb5/P1M1T1S3/PRP.md
  why: "Parallel in-flight item: internal/cmd/root.go + internal/config/* + docs (idle_timeout
        flags/bootstrap). ZERO overlap with internal/provider/. Confirm and proceed."
- file: internal/provider/ (activity consumers — LAND LATER)
  critical: "S2 (P1.M2.T1.S2) reworks Execute to consume newCountingWriter + groupReadBytes. Do NOT
             pre-wire any caller: after this task, grep shows countingWriter/groupReadBytes
             referenced ONLY by their own tests. Zero production callers is the expected end state."
```

### Current Codebase tree (relevant slice)

```bash
internal/provider/
  executor.go            # READ-ONLY — Execute (S2 reworks it; the seam: cmd.Stdout=&out, RegisterChild(pid))
  procgroup_unix.go      # READ-ONLY — THE platform-split template (!windows; Setpgid ⇒ PGID==PID)
  procgroup_windows.go   # READ-ONLY — the twin (windows); CONTRACT-header precedent
  procgroup_windows_test.go  # READ-ONLY — the platform-tagged-test precedent
  executor_test.go       # READ-ONLY — the mustBin skip-guard test idiom
  activity.go            # CREATE — countingWriter (all platforms)
  activity_linux.go      # CREATE — groupReadBytes (linux)
  activity_other.go      # CREATE — groupReadBytes stub (!linux)
  activity_test.go       # CREATE — countingWriter tests (portable, incl. -race concurrency)
  activity_linux_test.go # CREATE — groupReadBytes tests (linux build tag)
```

### Desired Codebase tree with files to be added/modified

```bash
# NEW (no existing file modified):
internal/provider/activity.go            # countingWriter: mutex+buffer+atomic lastWrite; Write/LastWrite/Bytes
internal/provider/activity_linux.go      # groupReadBytes(pgid): /proc stat pgrp filter → io rchar sum
internal/provider/activity_other.go      # groupReadBytes(pgid): (0,false) stub — no channel off Linux
internal/provider/activity_test.go       # countingWriter: accumulate/zero/snapshot/concurrency(-race)
internal/provider/activity_linux_test.go # groupReadBytes: own-group/cat-child/bogus-pgid/post-reap
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (parse stat AFTER THE LAST ')' — the #1 procfs trap): comm (field 2) may contain spaces
//   AND parentheses ("foo)bar", "my agent -x") — splitting the whole line or scanning for the FIRST
//   ')' misaligns every field. bytes.LastIndexByte(stat, ')') then bytes.Fields(stat[i+1:]):
//   token 0=state, 1=ppid, 2=pgrp. (man-pages don't document this; the proc(5) maintainer guidance
//   does — external_procfs_notes §1.)

// CRITICAL (stat FIRST, filter by pgrp, ONLY THEN open io): kernel threads appear in the /proc glob;
//   their io is root-owned → an unprivileged open is EACCES. Parsing every stat (world-readable) and
//   filtering by pgrp BEFORE opening io means only the <20 matching members' io files are opened.
//   Blindly opening every /proc/<pid>/io wastes syscalls and trips EACCES noise.

// CRITICAL ((0,false) vs (0,true) — S2's contract): ok==false means "NO CHANNEL" (/proc unavailable
//   — non-Linux handled by the stub, or procfs unmounted) and S2's watcher falls back to
//   countingWriter alone. An EMPTY MATCH SET (bogus pgid, or all members reaped) is (0, TRUE) — the
//   channel works, the group just read 0 bytes. Returning false for an empty match would silently
//   disable input-side detection on Linux. Only os.ReadDir("/proc") failure returns false.

// CRITICAL (per-member failures are SKIPS, never errors): a reaped member's stat open fails ENOENT
//   (the /proc/<pid> dir disappears after waitpid) → skip. A member reaped BETWEEN stat and io →
//   io open ENOENT → skip. A non-dumpable member's io → EACCES → skip. An unparseable stat/io →
//   skip. None of these abort the sweep or flip ok to false. ("Treat ENOENT as member-gone, not
//   error; re-identify members every tick; never cache FDs across ticks" — notes §1.)

// GOTCHA (rchar, NOT read_bytes): rchar = bytes returned by SUCCESSFUL read(2)-family calls —
//   pipes/tty/sockets count, pagecache hits count, failed reads count nothing. read_bytes is the
//   block-layer counter — a different thing that would MISS pipe/tty activity entirely.

// GOTCHA (monotonic timestamp — use atomic.Pointer[time.Time], not UnixNano int64): store whole
//   time.Now() values (Go embeds the monotonic reading); S2's time.Since(LastWrite()) is then immune
//   to wall-clock jumps. An atomic.Int64 UnixNano is wall-clock — an NTP step could fake idleness or
//   activity. atomic.Pointer is stdlib (Go 1.19+; go.mod targets 1.24/1.25). Never-written ⇒ nil ⇒
//   zero time.Time ⇒ time.Since(zero) is huge ⇒ "idle since forever" — correct semantics; S2 takes
//   max(LastWrite, armTime).

// GOTCHA (snapshot semantics): Bytes() must return a COPY (append([]byte(nil), buf.Bytes()...)) under
//   the mutex — a caller-held slice must never be mutated by later concurrent Writes. The naive
//   `return w.buf.Bytes()` aliases the buffer's internal storage.

// GOTCHA (PGID==PID is the caller's invariant): procgroup_unix.go's Setpgid makes the child a NEW
//   group leader ⇒ PGID == PID by kernel guarantee, so groupReadBytes(cmd.Process.Pid) covers child
//   + ALL descendants. Cite this in activity_linux.go's header (it's why the function takes a pgid
//   that callers will actually pass a pid to).

// GOTCHA (build tags — linux / !linux, mirroring procgroup's !windows / windows): activity_linux.go
//   carries `//go:build linux` (NOT !windows — macOS has no /proc either, so darwin gets the stub).
//   activity_other.go carries `//go:build !linux`. The test file activity_linux_test.go carries the
//   linux tag (procgroup_windows_test.go precedent). Verify with GOOS=linux/darwin/windows builds.
```

## Implementation Blueprint

### Data models and structure

```go
// activity.go (all platforms)
type countingWriter struct {
    mu        sync.Mutex
    buf       bytes.Buffer
    lastWrite atomic.Pointer[time.Time]
}
var _ io.Writer = (*countingWriter)(nil)

// activity_linux.go / activity_other.go (twins, identical signature)
func groupReadBytes(pgid int) (uint64, bool)
```
No exported types; no new deps (bytes/io/os/strconv/sync/sync/atomic/time/syscall-test-only — stdlib).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/provider/activity.go — countingWriter
  - FILE COMMENT: cite FR-A3 (§9.30) + that S2 (P1.M2.T1.S2) assigns these to cmd.Stdout/Stderr and
    returns string(w.Bytes()) for Execute's return contract; the activity signals are buffer growth
    + lastWrite. Note the zero-time contract (never-written ⇒ idle-since-forever; S2 baselines at arm).
  - THE STRUCT + Write/LastWrite/Bytes verbatim from research/findings.md §1 (mutex-wrapped
    bytes.Buffer; atomic.Pointer[time.Time] storing whole time.Now() on every n>0 Write; Bytes()
    returns an independent COPY under the mutex; the `var _ io.Writer` compile assertion).
  - ZERO-VALUE READY: no constructor required (document that &countingWriter{} is ready; S2 may add
    a newCountingWriter() convenience if it wants — not required here).
  - IMPORTS: bytes, io, sync, sync/atomic, time.

Task 2: CREATE internal/provider/activity_linux.go — groupReadBytes (the poller)
  - BUILD TAG: `//go:build linux` (line 1), then package clause.
  - FILE COMMENT (mirror procgroup_unix.go): explain the mechanism (sum /proc/<pid>/io rchar over the
    group's live members = input-side activity invisible to the stdout channel — FR-A3); cite the
    PGID==PID invariant (Setpgid in procgroup_unix.go ⇒ callers pass cmd.Process.Pid); cite the
    grounding (external_procfs_notes.md 2026-10-20: last-`)` rule, rchar semantics, same-uid
    permissions, ~13ms/~1100-pid sweep ≈2.6% duty at 2Hz); the CONTRACT clause freezing the twin:
    "the non-Linux build (activity_other.go, //go:build !linux) MUST implement the IDENTICAL
    signature `func groupReadBytes(pgid int) (uint64, bool)` returning (0,false); consumed by
    P1.M2.T1.S2 — do NOT change this signature without coordinating."
  - THE ALGORITHM verbatim from findings §2: os.ReadDir("/proc") → (0,false) on error ONLY;
    skip non-numeric entry names; os.ReadFile stat (skip on any error); bytes.LastIndexByte(stat,')')
    → skip if <0; bytes.Fields(stat[i+1:]) → skip if len<3; strconv.Atoi(fields[2]) → skip on error;
    if pgrp != pgid → continue (FILTER); os.ReadFile io (skip on any error); parse the "rchar:"
    line (strconv.ParseUint) → skip on absence/parse error; sum += rchar; return (sum, true).
  - A tiny helper for the all-decimal name check (e.g. isPidName) or strconv.Atoi on the name +
    error-as-skip — either is fine; keep it local and unexported.
  - rchar parse: iterate ioData lines (bytes.Split on '\n' or bufio over the read); match prefix
    "rchar:"; ParseUint(strings.TrimSpace(rest), 10, 64).
  - IMPORTS: bytes, os, strconv, strings.

Task 3: CREATE internal/provider/activity_other.go — the stub
  - BUILD TAG: `//go:build !linux`.
  - THE STUB verbatim from findings §3: doc comment explaining (0,false) = "no channel" (S2's watcher
    falls back to countingWriter alone; the hard cap still bounds the run) + the CONTRACT clause
    (identical signature; the Linux twin is activity_linux.go). One-line body: return 0, false.

Task 4: CREATE internal/provider/activity_test.go — countingWriter tests (portable)
  - Mirror executor_test.go style (stdlib testing; t.Helper helpers; "got %q want %q").
  - TestCountingWriter_AccumulatesAndStamps: 3 Writes ("a","bb","ccc") → Bytes()=="abccc";
    Write returns (len(p),nil); LastWrite() !IsZero() && time.Since(LastWrite()) < 5s.
  - TestCountingWriter_ZeroBeforeFirstWrite: fresh &countingWriter{} → LastWrite().IsZero();
    len(Bytes())==0.
  - TestCountingWriter_SnapshotIsIndependent: Write("x"); snap:=Bytes(); Write("y");
    string(snap)=="x" (the copy contract).
  - TestCountingWriter_ConcurrentWrites (THE race gate): 8 goroutines × 200 Writes of "abcd";
    sync.WaitGroup; then len(Bytes())==8*200*4; LastWrite() !IsZero(). Green under -race.

Task 5: CREATE internal/provider/activity_linux_test.go — groupReadBytes tests (//go:build linux)
  - BUILD TAG: `//go:build linux`. Imports: os/exec, syscall, io, strings, testing, time.
  - TestGroupReadBytes_OwnGroupVisible: pgid := syscall.Getpgrp(); sum, ok := groupReadBytes(pgid);
    assert ok (hard gate; sum is typically >0 — the test binary has done I/O — assert sum >= 0).
  - TestGroupReadBytes_ChildGroupCounterMoves: mustBin(t, "cat") (the executor_test idiom);
    cmd := exec.Command("cat"); cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} (own group ⇒
    PGID==cmd.Process.Pid); cmd.Stdin = strings.NewReader(strings.Repeat("x", 8<<20)); cmd.Stdout =
    io.Discard; cmd.Stderr = io.Discard; Start; sum, ok := groupReadBytes(cmd.Process.Pid);
    assert ok && sum > 0 (per notes: exec implies loader reads; the 8MiB pipe reads accumulate —
    if flaky on a very fast box, sample twice 20ms apart and assert non-decreasing + sum>0 on either);
    cmd.Wait(); then again groupReadBytes(cmd.Process.Pid) → assert ok STILL true (member reaped ⇒
    silently excluded — THE ENOENT-tolerance assertion; sum may be 0).
  - TestGroupReadBytes_BogusPgid: groupReadBytes(1<<30) → ok==true, sum==0 (empty match set is
    (0,true), NOT (0,false)).

Task 6: VERIFY — builds (native + 3 GOOS), vet, format, focused + race tests, lint, grep guards
  - go build ./... ; GOOS=linux go build ./internal/provider/ ; GOOS=darwin go build ./internal/provider/ ; GOOS=windows go build ./internal/provider/
  - go vet ./internal/provider/... (native; optionally per-GOOS)
  - gofmt -l internal/provider/activity*.go   # empty
  - go test ./internal/provider/ -run 'CountingWriter' -v
  - go test ./internal/provider/ -run 'GroupReadBytes' -v   (runs on Linux CI; file absent elsewhere)
  - go test -race ./internal/provider/... ; make test ; make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: countingWriter (the stdout/stderr activity channel — verbatim)
type countingWriter struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	lastWrite atomic.Pointer[time.Time] // whole time.Now() — monotonic reading preserved
}
var _ io.Writer = (*countingWriter)(nil)

func (w *countingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	n, err := w.buf.Write(p)
	w.mu.Unlock()
	if n > 0 {
		now := time.Now()
		w.lastWrite.Store(&now)
	}
	return n, err
}
func (w *countingWriter) LastWrite() time.Time {
	if tp := w.lastWrite.Load(); tp != nil { return *tp }
	return time.Time{}
}
func (w *countingWriter) Bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]byte(nil), w.buf.Bytes()...) // COPY — callers keep a stable snapshot
}

// PATTERN: groupReadBytes core loop (linux — the order of ops is load-bearing)
entries, err := os.ReadDir("/proc")
if err != nil { return 0, false } // no /proc — the ONLY (…,false)
var sum uint64
for _, e := range entries {
	pidStr := e.Name()
	if _, err := strconv.Atoi(pidStr); err != nil { continue } // not a pid dir (irq/, net/, …)
	stat, err := os.ReadFile("/proc/" + pidStr + "/stat")
	if err != nil { continue }                       // reaped between readdir and stat — member gone
	i := bytes.LastIndexByte(stat, ')')              // comm may contain spaces AND parens
	if i < 0 || i+1 >= len(stat) { continue }
	fields := bytes.Fields(stat[i+1:])               // 0=state 1=ppid 2=pgrp
	if len(fields) < 3 { continue }
	pgrp, err := strconv.Atoi(string(fields[2]))
	if err != nil || pgrp != pgid { continue }       // FILTER before touching io (kernel threads)
	ioData, err := os.ReadFile("/proc/" + pidStr + "/io")
	if err != nil { continue }                       // reaped mid-sweep / non-dumpable
	if n, ok := parseRchar(ioData); ok { sum += n }
}
return sum, true

// parseRchar: find the "rchar: <uint>" line; ok=false when absent/unparseable (member skipped).
```

### Integration Points

```yaml
NO existing file modified. NO config/manifest/render/registry/flag/docs surface. 5 new files.

PROVIDER PACKAGE (internal/provider):
  - activity.go: countingWriter (Write/LastWrite/Bytes; zero-value ready).
  - activity_linux.go + activity_other.go: groupReadBytes twins (CONTRACT-frozen signature).

DOWNSTREAM (this task PROVIDES; consumers LAND LATER — zero production callers is expected):
  - S2 (P1.M2.T1.S2, the Execute idle watcher): swaps executor.go's `var out, errb bytes.Buffer` for
    countingWriters; polls groupReadBytes(cmd.Process.Pid) every ~500ms; treats ok==false as
    "input-side channel unavailable — countingWriter only"; computes rchar deltas and CLAMPS
    negatives (the counters are not atomic — proc_pid_io(5) CAVEATS; clamping is S2's job, NOT the
    summer's); max(LastWrite, armTime) as the idle baseline.

COORDINATION (parallel P1.M1.T1.S3, in-flight): internal/cmd/root.go + internal/config/* + docs
  (idle_timeout flags) — ZERO overlap with internal/provider/. No file conflicts either direction.

SCOPE FENCES: NO executor.go edit (S2); NO render.go insertion (that's P2's event-stream); NO
  manifest field; NO config; NO docs (internal primitives); NO PRD/tasks.json.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native build.
go build ./...

# Cross-platform builds — BOTH twins must compile with identical signatures (the platform-split gate).
GOOS=linux   go build ./internal/provider/
GOOS=darwin  go build ./internal/provider/
GOOS=windows go build ./internal/provider/
# Expected: all clean. A darwin/windows failure = the stub's build tag or imports are wrong;
#           a linux failure = the poller's (syscall import must NOT appear in activity_linux.go —
#           plain file I/O only).

# Vet.
go vet ./internal/provider/...

# Format.
gofmt -l internal/provider/activity.go internal/provider/activity_linux.go internal/provider/activity_other.go internal/provider/activity_test.go internal/provider/activity_linux_test.go
# Expected: empty.

# Lint.
make lint      # golangci-lint v1.61 — `unused` is the one to watch: both symbols are read by the
#               new tests, so clean. (S2's production reads come later.)
# Expected: zero errors.

# Scope guard: ONLY the 5 new files.
git status --short
# Expected: ?? internal/provider/activity.go …_linux.go …_other.go …_test.go …_linux_test.go (only).
```

### Level 2: Unit Tests (Component Validation)

```bash
# countingWriter (all platforms).
go test ./internal/provider/ -run 'CountingWriter' -v
# Expected: PASS — accumulate/stamp, zero-state, snapshot independence, 8-goroutine concurrency.

# groupReadBytes (Linux CI; the file is build-tagged so it's absent elsewhere).
go test ./internal/provider/ -run 'GroupReadBytes' -v
# Expected: PASS — own-group ok; cat-child ok && sum>0; post-reap ok still true; bogus pgid (0,true).

# THE race gate — the countingWriter mutex discipline.
go test -race ./internal/provider/...
# Expected: green (race detector proves Write/Bytes concurrency safety).

# Full package + full repo.
go test ./internal/provider/... -v
make test
# Expected: green (all 21+ existing provider tests unaffected — additive files only).
```

### Level 3: Integration Testing (System Validation)

```bash
# There is no integration surface — these are primitives with NO production caller yet (S2 wires
# them into Execute; P1.M3.T1.S1-3 do the e2e idle-kill parity). The Linux procfs tests ARE the
# closest-to-integration proof available: they run against the REAL /proc of the CI host, a REAL
# Setpgid child, and the REAL reap race. Sanity-build the binary to confirm no downstream break:
go build ./...
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1: no activity code leaks into existing files (executor.go untouched).
git diff --name-only   # (plus untracked): only the 5 activity files; in particular:
git diff --stat -- internal/provider/executor.go
# Expected: empty (executor.go FROZEN — S2 owns it).

# Grep guard 2: ZERO production callers (expected — S2 lands later).
grep -rn 'countingWriter\|groupReadBytes' --include='*.go' internal/ cmd/ pkg/ | grep -v 'internal/provider/activity'
# Expected: empty (references live only in the activity files themselves).

# Grep guard 3: the twins carry identical signatures + the CONTRACT clause.
grep -n 'func groupReadBytes' internal/provider/activity_linux.go internal/provider/activity_other.go
# Expected: 2 hits, byte-identical signatures.
grep -c 'IDENTICAL signature' internal/provider/activity_linux.go internal/provider/activity_other.go
# Expected: 1 each (the CONTRACT header — the procgroup_*.go pattern).

# Grep guard 4: build tags are correct and FIRST-LINE.
head -1 internal/provider/activity_linux.go internal/provider/activity_other.go internal/provider/activity_linux_test.go
# Expected: "//go:build linux", "//go:build !linux", "//go:build linux" respectively.

# Grep guard 5: the last-')' parse + filter-before-io ordering.
grep -n 'LastIndexByte\|pgrp != pgid' internal/provider/activity_linux.go
# Expected: both present, and the pgrp filter appears BEFORE any ReadFile of "/io".

# Grep guard 6: (0,false) is returned ONLY on ReadDir failure.
grep -B2 -A2 'return 0, false' internal/provider/activity_linux.go
# Expected: exactly one such return, immediately after the ReadDir error check.

# Regression: the whole provider package stays green.
go test ./internal/provider/... -v
# Expected: all PASS (additive files only — zero existing-test churn).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `GOOS=linux/darwin/windows go build ./internal/provider/` all clean (both twins compile)
- [ ] `go vet ./internal/provider/...` clean
- [ ] `gofmt -l` empty on all 5 files
- [ ] `go test -race ./internal/provider/...` green (the mutex-discipline gate)
- [ ] `make test` + `make lint` pass

### Feature Validation
- [ ] countingWriter: accumulates; stamps lastWrite (monotonic-preserving); zero-state contract; snapshot independence; concurrent safety under -race
- [ ] groupReadBytes (linux): own group ok; Setpgid cat-child ok && sum>0; post-reap ok still true (ENOENT tolerance); bogus pgid → (0,true)
- [ ] groupReadBytes (!linux): (0,false) stub; S2's "no channel" semantics
- [ ] stat parsed after the LAST ')'; pgrp filter before any io open; per-member failures skipped

### Scope-Boundary Validation
- [ ] `git status` == only the 5 new activity files
- [ ] executor.go UNTOUCHED (S2 owns it); render.go/manifest.go/registry.go untouched
- [ ] NO config/flag/docs change; NO PRD/tasks.json
- [ ] ZERO production callers of either symbol (tests only — S2 wires them later)

### Code Quality & Docs
- [ ] File headers cite FR-A3 + the PGID==PID invariant + the procfs grounding (dated) + the CONTRACT clause
- [ ] Mirrors the procgroup_*.go platform-split style (build tag first line; frozen twin signature)
- [ ] Tests mirror executor_test.go idioms (mustBin skip-guard, real binaries, stdlib assertions)

---

## Anti-Patterns to Avoid

- ❌ Don't parse /proc/[pid]/stat by splitting the whole line or scanning for the FIRST ')'. comm
  (field 2) may contain spaces AND parentheses (`foo)bar`, `my agent -x`) — that misaligns every
  field. Use `bytes.LastIndexByte(stat, ')')` and parse the remainder (external_procfs_notes §1 —
  the maintainer-endorsed rule the man page doesn't document).
- ❌ Don't open /proc/<pid>/io before filtering by pgrp. Kernel threads appear in the glob and their
  io is root-owned → unprivileged EACCES. Stat is world-readable: parse EVERY stat, filter to the
  group, then open io for the <20 matching members only (notes §4).
- ❌ Don't return (0, false) for an empty match set or a reaped member. ok==false means "NO CHANNEL"
  (non-Linux / no /proc) — S2 disables input-side detection on it. A bogus pgid, an all-reaped
  group, a skipped member: all (0-or-sum, TRUE). Only `os.ReadDir("/proc")` failure returns false.
- ❌ Don't return errors (or abort the sweep) on per-member stat/io failures. ENOENT = member reaped
  (the dir vanishes after waitpid); EACCES = non-dumpable; unparseable = skip. All are expected
  races on a live system — skip and continue. Also never cache member FDs across ticks (an
  already-open FD never sees a recycled pid — later ops fail ESRCH).
- ❌ Don't use read_bytes instead of rchar. read_bytes is the block-layer counter; rchar is
  successful read-family bytes (pipes/tty/sockets count, pagecache hits count). The agent's
  pipe/tty input activity — exactly what we want — is INVISIBLE to read_bytes.
- ❌ Don't store the last-write timestamp as an int64 UnixNano. Use atomic.Pointer[time.Time] holding
  whole time.Now() values — Go's embedded monotonic reading survives, so S2's
  time.Since(LastWrite()) is immune to NTP wall-clock jumps. (UnixNano is wall-clock.)
- ❌ Don't return `w.buf.Bytes()` directly from Bytes(). That aliases the buffer's internal storage —
  a caller-held slice would be mutated (raced) by later concurrent Writes. Copy under the mutex:
  `append([]byte(nil), w.buf.Bytes()...)`.
- ❌ Don't touch executor.go. S2 (P1.M2.T1.S2) owns the Execute rework — the stdout/stderr swap, the
  polling loop, delta clamping (the proc counters are non-atomic per proc_pid_io(5); clamping
  negatives is the DELTA consumer's job, not the summer's), and the arm-time baseline. This task
  ships primitives with zero production callers — that's the correct end state.
- ❌ Don't use `//go:build !windows` for the Linux file. macOS has no /proc either — the split is
  linux vs !linux (the contract's explicit tags). Verify with GOOS=darwin builds (the stub must
  win there).
- ❌ Don't write the Linux tests in the portable test file (or vice versa). activity_linux_test.go
  carries `//go:build linux` — it compiles ONLY on Linux CI (procgroup_windows_test.go precedent);
  the countingWriter tests are portable and must run on every platform. A runtime.GOOS guard is
  redundant when the file is tagged — don't mix the two mechanisms.
- ❌ Don't invent a tempdir stub script for the child test. Mirror executor_test.go: real PATH
  binaries (`cat`) behind the `mustBin(t, …)` skip-guard. The Setpgid+cat+8MiB-stdin construction
  is the deterministic "reads a known number of bytes" child.

---

## Confidence Score: 9/10

Every load-bearing fact is grounded: the verbatim countingWriter and groupReadBytes code, the
procfs semantics (last-`)` rule, rchar counting, permissions, sweep cost, reap races — all verified
against dated external research (man-pages 6.19 + kernel docs + source) and the in-repo
procgroup/executor precedents; the platform-split style and test idioms are copied from working
files; the scope is fenced to 5 additive files with zero existing-code churn. The one residual (not
a full 10): the cat-child "sum > 0" assertion races the child's completion on a very fast machine —
mitigated in the test design (8MiB stdin; re-sample-20ms non-decreasing fallback), and the
ok==true/ENOENT-tolerance assertions (the real contract) are race-free. One-pass success is highly
likely.
