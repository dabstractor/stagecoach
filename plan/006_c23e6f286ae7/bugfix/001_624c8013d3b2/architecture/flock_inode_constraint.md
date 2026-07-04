# Critical Constraint — flock is inode-bound (Issue 4 fix MUST respect this)

## The constraint

`flock(2)` (`LOCK_EX | LOCK_NB`) binds the advisory lock to the **inode** of the open
file descriptor, NOT to the filename/path. This is POSIX semantics. Consequence:

**If you `os.Rename(newTempFile, lockPath)` over a lock file that a holder has open and
flock'd, the rename creates a brand-new inode at `lockPath`. The holder's fd still points
to the OLD inode (which retains the flock, but is now unreachable by name). A contender
that then `os.OpenFile(lockPath)` gets the NEW inode, calls `flock` on it, and SUCCEEDS —
because no one holds a flock on the new inode. The lock is effectively BYPASSED.**

This breaks the headline safety guarantee of FR52 (preventing two processes racing on HEAD).

## Why this matters for Issue 4

Issue 4 ("SetSnapshot rewrite can be observed mid-write") suggests two fixes:
1. Write to a sibling temp file and `os.Rename` over the lock file (atomic on POSIX).
2. Write the full contents into a buffer and `Write` in a single call after `Truncate`+`Seek`.

**Option 1 (temp-file + rename) is FORBIDDEN here** because of the inode constraint above.
It would silently disable cross-process contention detection. The PRD lists it generically
("atomic on POSIX") without accounting for the flock-inode binding — the implementing agent
MUST NOT use it.

## The correct fix (in-place rewrite on the SAME fd)

Use `Seek → Write(fullBuffer) → Truncate(len) → Sync` on the holder's existing fd:

```go
func (l *Locker) setSnapshot(sha string) {
    if l.file == nil { return }
    content := fmt.Sprintf("pid=%s\nhostname=%s\nrepo=%s\ntimestamp=%s\nsnapshot=%s\n",
        l.pid, l.hostname, l.repo, l.timestamp, sha)
    l.file.Seek(0, 0)
    l.file.Write([]byte(content))        // overwrite from start (old trailing bytes may remain)
    l.file.Truncate(int64(len(content))) // cut stale trailing bytes to exact length
    l.file.Sync()
}
```

Why this is safe and narrows the race to near-zero:
- The fd/inode never changes → flock semantics are preserved.
- **Order matters:** `Write` BEFORE `Truncate` means the file is NEVER empty — it is either
  the old content, a prefix of the new content, or the complete new content. A contender
  reading via `os.ReadFile` during the window always sees at least the leading
  `pid`/`hostname`/`repo` lines (they come before `snapshot`), so the Busy message is never
  rendered with all-empty diagnostics.
- Contrast with the CURRENT code (`Truncate(0); Seek(0,0); writeContents`) which produces an
  **empty file** between Truncate and the Fprintf completion — that is the window where the
  ugly `"on  (pid  on )"` message appears.

## Residual defense-in-depth (Issue 4 subtask 2)

Even with the in-place fix, a contender could still read a partial file in a microsecond
window. `handleLockContention` (cmd/default_action.go:241) must therefore guard against
empty `repo`/`pid`/`hostname` and substitute sensible fallbacks (e.g. "an unknown repo"
/ "pid <unknown>") so the message is never gibberish. This is the "at minimum" fix the PRD
endorses.

## Summary for implementing agents

- **NEVER** `os.Rename` over the lock path. The lock is flock-on-inode; rename orphans it.
- **ALWAYS** rewrite in place on the held fd: `Seek(0,0) → Write(full) → Truncate(len) → Sync`.
- Write-before-Truncate (not Truncate-before-Write) to avoid the empty-file state.
- Add the empty-field guard in `handleLockContention` as defense-in-depth.
