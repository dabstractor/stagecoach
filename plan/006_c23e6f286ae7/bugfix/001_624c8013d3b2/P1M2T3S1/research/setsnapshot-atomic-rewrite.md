# Research: Atomic in-place setSnapshot rewrite (Issue 4a) — write-ordering + inode constraint + invariant test

> Subtask P1.M2.T3.S1. Fix for Issue 4 (Minor): `setSnapshot`'s `Truncate(0) → Seek(0,0) →
> writeContents` creates an empty-file window where a contender's `os.ReadFile` reads "" / a
> partial file → `parseContents` yields empty fields → the Busy message renders as
> `"on  (pid  on )"`. This note pins the fix (Seek→Write→Truncate→Sync, Write-before-Truncate),
> the HARD inode constraint (no temp-file+Rename), the chosen consolidation, and the invariant test.

---

## 1. Current code (the bug) — `internal/lock/lock.go`

```go
// setSnapshot (current — BUGGY): Truncate(0) empties the file BEFORE writeContents refills it.
func (l *Locker) setSnapshot(sha string) {
	if l.file == nil { return }
	l.file.Truncate(0)   // ← file becomes EMPTY here (the window opens)
	l.file.Seek(0, 0)
	l.writeContents(sha) // ← refills (window closes). A contender reading between = empty/partial.
}

// writeContents (current): single Fprintf of all 5 lines + Sync. No Seek/Truncate of its own.
func (l *Locker) writeContents(snapshot string) {
	fmt.Fprintf(l.file, "pid=%s\nhostname=%s\nrepo=%s\ntimestamp=%s\nsnapshot=%s\n",
		l.pid, l.hostname, l.repo, l.timestamp, snapshot)
	l.file.Sync()
}
```

`writeContents` is called from TWO sites: `Acquire` (initial write to a fresh/existing file, `l.writeContents("")`)
and `setSnapshot` (the rewrite). Errors from Seek/Truncate/Write/Sync are ALL ignored today — that style
is preserved (the public `SetSnapshot` signatures are void; surfacing would change them = forbidden).

### The contender read path (why the window bites)
`Acquire` on `EWOULDBLOCK` does a SEPARATE `os.ReadFile(path)` (open/read/close on a different fd) and
`parseContents` on the bytes. If the holder's `Truncate(0)` has landed but `writeContents` hasn't
finished, the contender reads `""` → all fields empty → Busy message with empty diagnostics.

---

## 2. THE HARD CONSTRAINT — flock is inode-bound (NO temp-file + Rename)

`flock(LOCK_EX|LOCK_NB)` binds the advisory lock to the **inode** of the held fd, NOT the filename
(`architecture/flock_inode_constraint.md`). Therefore **`os.Rename(temp, lockPath)` is FORBIDDEN**:
rename installs a brand-new inode at `lockPath`; the holder's fd still points at the OLD inode (flock
retained but now nameless); a contender `OpenFile(lockPath)` → new inode → `flock` SUCCEEDS → **FR52
contention detection is silently bypassed.** The PRD's generic "temp-file + rename (atomic on POSIX)"
suggestion does NOT account for this — the implementer MUST NOT use it.

**The only correct fix is an in-place rewrite on the SAME held fd**, with **Write BEFORE Truncate**:

```go
content := fmt.Sprintf("pid=%s\nhostname=%s\nrepo=%s\ntimestamp=%s\nsnapshot=%s\n",
    l.pid, l.hostname, l.repo, l.timestamp, sha)
l.file.Seek(0, 0)
l.file.Write([]byte(content))         // overwrite from start; old trailing bytes may remain
l.file.Truncate(int64(len(content)))  // cut stale trailing bytes to exact length
l.file.Sync()
```

**Why Write-before-Truncate is the invariant:** during the rewrite the fd/inode content is one of
{old content, a prefix of the new content, the complete new content} — NEVER empty. The leading
`pid`/`hostname`/`repo`/`timestamp` lines precede `snapshot`, so a contender always reads at least the
diagnostic lines. (Truncate-before-Write, the current bug, yields an empty file in the window.)

---

## 3. Chosen consolidation — make `writeContents` the uniform Seek→Write→Truncate→Sync primitive

The contract offers two valid shapes; this PRP chooses **Option A (consolidate into `writeContents`)**:

```go
// setSnapshot keeps the nil guard; delegates to writeContents.
func (l *Locker) setSnapshot(sha string) {
	if l.file == nil { return }
	l.writeContents(sha)
}

// writeContents is now the single atomic-ish rewrite primitive, used by BOTH Acquire and setSnapshot.
func (l *Locker) writeContents(snapshot string) {
	content := fmt.Sprintf("pid=%s\nhostname=%s\nrepo=%s\ntimestamp=%s\nsnapshot=%s\n",
		l.pid, l.hostname, l.repo, l.timestamp, snapshot)
	l.file.Seek(0, 0)
	l.file.Write([]byte(content))
	l.file.Truncate(int64(len(content)))
	l.file.Sync()
}
```

**Why Option A over "inline in setSnapshot, leave writeContents" (the flock_inode_constraint.md sketch):**
1. **DRY** — one rewrite path, not two.
2. **Fixes a latent Acquire bug as a bonus.** Today `Acquire` calls `writeContents("")` on a file opened
   `O_CREATE|O_RDWR`. If the file already existed (a prior process crashed without `Release`, leaving a
   LONGER stale file), the current `Fprintf`-without-Truncate overwrites the prefix but LEAVES stale
   trailing bytes → `parseContents` reads the new prefix + a malformed trailing line (silently skipped, so
   not catastrophic, but the on-disk file is corrupt). The new `writeContents` (Seek→Write→**Truncate**)
   cuts the stale tail → the file is exactly the new content. Strictly better; no regression.
3. Both call sites inherit the never-empty guarantee.
4. Minimal code: `setSnapshot` stays a thin nil-guard + delegate; `writeContents` gains 3 lines.

**Acceptable variant (Option B):** inline the buffered Write into `setSnapshot` and leave `writeContents`
as the `Fprintf`-only initial-write primitive. Also correct (satisfies the invariant for the rewrite
path). Option A is chosen because it is DRY + fixes Acquire. The KEY invariants either way:
Seek→Write→Truncate→Sync order, Write-before-Truncate, NEVER `os.Rename` over the lock path.

### Error handling
Preserve the codebase style: **ignore** Seek/Write/Truncate/Sync errors (the public `SetSnapshot`
signatures are void; the current code already ignores them). Do NOT add error returns — that would
change the unexported helpers' shape needlessly and risk rippling. (A write failure here is
non-fatal — the lock is still held via flock; the snapshot is a fast-path/diagnostic nicety.)

---

## 4. The invariant test (the headline deliverable)

The contract: "assert the write-ordering INVARIANT: after setSnapshot, the file is non-empty and
well-formed (contains all 5 keys)… a full race test is non-deterministic; instead assert the file is
never empty immediately after a setSnapshot call." The nil/released no-op is ALREADY covered by
`TestSetSnapshot_NilSafeNoOp` + `TestSetSnapshot_MethodAfterRelease` (unchanged, stay green).

**New test `TestSetSnapshot_FileNeverEmptyWellFormed`:**
1. `Acquire(repo)` → the initial write (now via the new `writeContents`) is well-formed (assert all 5
   keys present, snapshot=="").
2. `SetSnapshot("abc123def456")` → read immediately → non-empty + well-formed + snapshot=="abc123def456".
3. **SHRINK case** (the meaningful Truncate proof): `SetSnapshot(<36-char>)` then `SetSnapshot("short")`.
   A buggy implementation that omits `Truncate` leaves the stale tail of the longer previous write on
   disk. `parseContents` would STILL report `snapshot="short"` (the trailing garbage is a malformed line,
   silently skipped) — so parse-only assertions CANNOT catch a missing Truncate. **The robust check is on
   the RAW bytes:** `strings.HasSuffix(data, "snapshot=short\n")` — if Truncate didn't run, the suffix
   would be the leftover `…XXXX-YYYY\n` instead. This is the assertion that distinguishes the correct
   implementation from a Truncate-omitting one.
4. Also assert `len(data) > 0` after every `SetSnapshot` (the never-empty invariant — directly checks
   Issue 4's "file is never empty immediately after setSnapshot").

**Why no deterministic race test:** the empty-file window is microsecond-wide and contention is
inherently nondeterministic; the invariant test (write-ordering + shrink) is the contract-specified
proxy. A contender-side guard (empty-field fallbacks in `handleLockContention`) is the DEFENSE-IN-DEPTH
and is owned by **P1.M2.T4.S1** (Issue 4b) — explicitly NOT this subtask.

**Test conventions to mirror** (from `lock_test.go`): white-box `package lock`; `resetCurrent(t)` at the
top (the `current` singleton is process-global; NO `t.Parallel`); isolate XDG
(`t.Setenv("XDG_RUNTIME_DIR", t.TempDir())` + clear `XDG_CACHE_HOME`) like
`TestRelease_RemovesLockFile` / `TestAcquire_RepoFieldIsCanonical`; read `os.ReadFile(l.path)` BEFORE
`Release()` (Issue 2 removes the file on Release — `defer l.Release()` is fine because reads happen in
the test body before the deferred Release runs).

---

## 5. Scope fences (do NOT touch)

- `Acquire` body — unchanged (it already calls `l.writeContents("")`; it transparently gets the new
  primitive). The Issue-3 `repo: canonical` and the `lockHash` (canonical, hash) signature are already
  landed — leave them.
- `Release` — unchanged (Issue 2 close-then-remove already landed).
- `lock_unix.go` / `lock_windows.go` — unchanged (flock semantics; the fix is in shared `lock.go`).
- `handleLockContention` (`internal/cmd/default_action.go`) — UNCHANGED here (Issue 4b owns the
  empty-field guard; this subtask is 4a only).
- `SetSnapshot` method + package-level signatures — UNCHANGED (void; the contract forbids signature
  changes).
- `go.mod` / `go.sum` — UNCHANGED (stdlib only; `fmt` already imported in lock.go).
- docs — UNCHANGED (DOCS: none; P1.M3 owns the doc sweep).

## 6. Files touched

- `internal/lock/lock.go` — EDIT `setSnapshot` (doc + body) + `writeContents` (doc + body).
- `internal/lock/lock_test.go` — ADD `TestSetSnapshot_FileNeverEmptyWellFormed` (+ `"strings"` import).
- Nothing else.
