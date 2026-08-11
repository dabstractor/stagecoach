name: "P1.M4.T1.S1 — Narrow the reapStaleLocks TOCTOU window via a re-read-before-remove defense (BUG-009)"
description: >
  A defense-in-depth fix for BUG-009 (PRD §h3.8 / architecture/bugfix_subsystems.md §BUG-009). The stale-file
  reaper `internal/lock/lock.go:reapStaleLocks` (unexported, ~line 128) has a TOCTOU window: it does
  `os.ReadFile(f)` → `processAlive(pid) → dead` → `os.Remove(f)`, and between the liveness check and the
  remove a CONCURRENT acquirer C can take the lock on the SAME path (the dead holder's flock auto-released on
  death → C's flock succeeds) and `writeContents`-rewrite `f` with C's own live pid. The reaper then unlinks a
  file whose inode C now holds flocked → on Unix the unlink removes the directory entry but C's open fd/flock
  survives on the unlinked inode → the NEXT contender `OpenFile(f, O_CREATE)`s a FRESH inode and flocks it
  free → two holders → FR52 defeated. The contract offers (A) defense-in-depth re-read (RECOMMENDED) or (B)
  documentation; this PRP does BOTH — Option A AND the [Mode A] comprehensive comment (which the DOCS clause
  requires regardless). FIX: after `processAlive → dead`, RE-READ `f` just before `os.Remove`; if the bytes
  DIFFER from the contents we liveness-checked (a concurrent acquirer rewrote the file) OR the re-read errored,
  SKIP the remove (the next reap cycle re-evaluates). The comparison is byte-equal (`bytes.Equal`, NOT just
  pid) so even pid-reuse-with-a-new-timestamp is caught (a new holder writes a fresh timestamp). The decision
  is extracted into a pure predicate `staleLockUnchanged(original, reread []byte, rerr error) bool` so the
  "changed → skip" branch is unit-testable DETERMINISTICALLY (the two reads return identical bytes in any
  single-goroutine test, so the branch is unreachable end-to-end without a flaky concurrent writer — the pure
  helper is the idiomatic answer, mirroring validateFormat/parseTimeout/exitcode.For). NEW stdlib import:
  `"bytes"` (lock.go does not currently import it). The residual micro-window (C rewrites BETWEEN the re-read
  and the remove) is narrowed, not fully closed — but it is BOUNDED by the CAS/flock mitigation (a live
  holder's flock survives an unlink; only the path is stale; the next Acquire heals via O_CREATE), documented
  in the rewritten reapStaleLocks comment (Mode A). TESTS: TestStaleLockUnchanged (lock_test.go, pure,
  cross-platform — same/changed/read-error branches) + TestReapStaleLocks_DirectDeadPidRemoved (lock_unix_test.go,
  Unix-only — calls the UNEXPORTED reapStaleLocks DIRECTLY via the white-box `package lock`, plants a dead-pid
  file via the existing writeLockFile helper, asserts removed → proves the defense does not false-skip the happy
  path). The existing TestAcquire_ReapsDeadPidFile_SparesLive STAYS GREEN UNCHANGED (nothing rewrites the dead
  file between the two reads → bytes equal → removed). SCOPE: internal/lock/lock.go (+staleLockUnchanged +
  re-read + comment + "bytes" import) + lock_test.go (+TestStaleLockUnchanged) + lock_unix_test.go
  (+TestReapStaleLocks_DirectDeadPidRemoved). ZERO edits to lock_unix.go/lock_windows.go (the flock/processAlive
  twins), orphan_*.go, Acquire/Release/Status/IsOrphaned, or any non-lock file. Parallel-safe: P1.M3.T4.S1 is
  doc-only (docs/packaging.md + README.md); P1.M4.T2/T3/T4.S1 are PLANNED and touch different files
  (lock_windows.go / orphan_unix.go / user-facing docs). NO user-facing doc change (the Mode A comment is a CODE
  comment; README/docs sync is P1.M4.T4.S1).

---

## Goal

**Feature Goal**: Narrow the BUG-009 TOCTOU window in `reapStaleLocks` so a concurrent acquirer that takes the
lock between the reaper's liveness check and its `os.Remove` is NOT silently unlinked (which would let a future
contender create a fresh inode and flock it free, defeating FR52). Implement the contract's recommended Option A
(re-read before remove; skip if the file changed) AND the [Mode A] comprehensive comment documenting the window
+ the defense + the residual + the CAS/flock mitigation that bounds it.

**Deliverable** (3 files, all in `internal/lock/`):
1. `internal/lock/lock.go` — (a) ADD `func staleLockUnchanged(original, reread []byte, rerr error) bool` (the
   pure defense predicate); (b) EDIT `reapStaleLocks` to re-read `f` before `os.Remove` and gate the remove on
   `staleLockUnchanged`; (c) REWRITE `reapStaleLocks`'s doc comment (Mode A) to document the TOCTOU + the
   defense + the residual + the CAS mitigation; (d) ADD `"bytes"` to the import block.
2. `internal/lock/lock_test.go` — ADD `TestStaleLockUnchanged` (pure, cross-platform: same/changed/error).
3. `internal/lock/lock_unix_test.go` — ADD `TestReapStaleLocks_DirectDeadPidRemoved` (Unix-only: calls
   `reapStaleLocks` directly, plants a dead-pid file via `writeLockFile`, asserts removed).

**Success Definition**:
- `reapStaleLocks`, after `processAlive(pid) → dead`, RE-READS `f` and removes it ONLY IF `staleLockUnchanged(
  data, reread, rerr)` is true (byte-identical + no read error). If a concurrent acquirer rewrote the file
  (bytes differ) or the re-read failed, the remove is SKIPPED (the next reap re-evaluates).
- `staleLockUnchanged(original, reread, rerr)` = `rerr == nil && bytes.Equal(original, reread)` — a pure,
  deterministic predicate.
- The residual micro-window (C rewrites between the re-read and the remove) is documented as narrowed-not-
  closed, bounded by the CAS/flock mitigation (a live holder's flock survives an unlink; the next Acquire
  heals via `O_CREATE`).
- `TestStaleLockUnchanged` pins the 3 branches; `TestReapStaleLocks_DirectDeadPidRemoved` proves the happy
  path (dead → removed; the defense does not false-skip).
- The existing `TestAcquire_ReapsDeadPidFile_SparesLive` STAYS GREEN UNCHANGED (dead file still reaped: nothing
  rewrites it between the two reads → bytes equal → removed).
- `go build ./...` + `GOOS={linux,darwin,windows}` clean; `go vet ./internal/lock/...` clean; `gofmt -l` empty;
  `go test ./internal/lock/ -race` green; `make test` + `make lint` clean.
- `git status --porcelain` == the 3 files. ZERO edits to the flock/processAlive twins, orphan_*.go,
  Acquire/Release/Status/IsOrphaned, or any non-lock file.

## User Persona (if applicable)

**Target User**: The Stagecoach maintainer (developer). This is an internal lock-package hardening, not user-facing.
**Use Case**: A future change to the lock/reap path must not regress the FR52 invariant (one holder per repo).
The defense + its test pin the invariant against the TOCTOU race.
**User Journey**: two `stagecoach` processes on the same repo, one dying mid-run → the next Acquire's
`reapStaleLocks` reaps the dead file WITHOUT ever unlinking a file a concurrent acquirer just took → FR52 holds.
**Pain Points Addressed**: BUG-009 — the narrow window where a concurrent acquirer's just-taken lock file could
be unlinked by the reaper, allowing a second contender to flock a fresh inode.

## Why

- **BUG-009 / §h3.8**: the TOCTOU is a real (if narrow) FR52-defeat vector. The re-read is the contract's
  recommended "cheap, effective defense" — it closes the common race (C rewrote before the re-read) at the cost
  of one extra `ReadFile`, and narrows the residual to a micro-window bounded by the existing CAS/flock mitigation.
- **Defense-in-depth, not a behavior change**: the happy path (dead file, no contender) is byte-identical in
  outcome (the file is still reaped). The defense ONLY adds a skip when the file changed under the reaper —
  exactly the unsafe case. Zero impact on any existing test or user.
- **The pure helper makes it deterministically testable**: the "changed → skip" branch is unreachable in a
  single-goroutine end-to-end test (the two reads return identical bytes). Extracting `staleLockUnchanged`
  (mirroring the codebase's `validateFormat`/`parseTimeout`/`exitcode.For` pure-helper idiom) lets the defense
  logic be pinned without a flaky concurrent test.
- **Bounded, no-conflict scope**: 3 files in `internal/lock/`. The flock/processAlive twins, orphan heuristics,
  and Acquire/Release are untouched. Parallel-safe with the doc-only P1.M3.T4.S1 and the PLANNED P1.M4.T2/T3/T4.

## What

A re-read-before-remove gate in `reapStaleLocks` + a pure `staleLockUnchanged` predicate + a comprehensive
Mode A comment + 2 tests. The user-visible behavior is UNCHANGED (dead lock files are still reaped); the only
new behavior is "skip the remove if the file changed under the reaper" — the unsafe case.

### Success Criteria
- [ ] `internal/lock/lock.go` adds `func staleLockUnchanged(original, reread []byte, rerr error) bool` returning
      `rerr == nil && bytes.Equal(original, reread)`, with a godoc explaining the BUG-009 defense (changed ⇒ skip).
- [ ] `reapStaleLocks` re-reads `f` after `processAlive → dead` and gates `os.Remove(f)` on
      `staleLockUnchanged(data, reread, rerr)` (skip on changed/error).
- [ ] `reapStaleLocks`'s doc comment (Mode A) documents the TOCTOU window, the re-read defense, the residual
      micro-window, and the CAS/flock mitigation bounding the residual.
- [ ] `"bytes"` added to lock.go's import block (stdlib; the only new import).
- [ ] `internal/lock/lock_test.go` adds `TestStaleLockUnchanged`: same-bytes→true; changed-bytes→false;
      read-error→false.
- [ ] `internal/lock/lock_unix_test.go` adds `TestReapStaleLocks_DirectDeadPidRemoved`: isolate XDG; plant a
      dead-pid file (`writeLockFile` + `math.MaxInt32` → ESRCH); call `reapStaleLocks(dir)` directly; assert
      the file is REMOVED (the defense does not false-skip the happy path).
- [ ] `TestAcquire_ReapsDeadPidFile_SparesLive` (lock_unix_test.go:77) is UNCHANGED and still GREEN.
- [ ] `go build ./...` + `GOOS=linux/darwin/windows` clean; `go vet ./internal/lock/...` clean; `gofmt -l` empty.
- [ ] `go test ./internal/lock/ -race` green; `make test` + `make lint` clean.
- [ ] `git status --porcelain` == the 3 files. NO edit to lock_unix.go/lock_windows.go, orphan_*.go,
      Acquire/Release/Status/IsOrphaned, or any non-lock file.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim current `reapStaleLocks` body, the verbatim fix (re-read + `staleLockUnchanged`), the
exact TOCTOU scenario + why the re-read closes the common race + the residual, the existing `writeLockFile`
helper + the `MaxInt32→ESRCH→dead` idiom + the white-box `package lock` direct-call fact, the proof the existing
`TestAcquire_ReapsDeadPidFile_SparesLive` stays green, the new `"bytes"` import, the Mode A comment content, the
scope fences (parallel siblings + which files are off-limits), and the grep guards.

### Documentation & References

```yaml
# MUST READ — codebase-specific findings for THIS item (the TOCTOU trace + the verbatim fix + the helper + tests).
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M4T1S1/research/findings.md
  why: "§0 the bug in one paragraph; §1 WHY the re-read works + the residual; §2 the verbatim current code +
        the verbatim fix; §3 the extracted pure helper staleLockUnchanged (for deterministic testability) +
        the new 'bytes' import; §4 the 2-test design (pure helper + direct reapStaleLocks call); §5 the Mode A
        comment content; §6 scope fences; §7 validation."

# MUST READ — the BUG-009 spec (the contract).
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/architecture/bugfix_subsystems.md
  section: "## BUG-009 (MINOR): TOCTOU window in reapStaleLocks"
  why: "Pins the TOCTOU (check processAlive then os.Remove), the mitigation (CAS-backed acquire + hostname
        check), and the two fix options (A re-read defense RECOMMENDED; B documentation). This PRP does BOTH."
  critical: "'After reaping, verify the file we removed is the one we checked (by reading contents before
             remove)' = the re-read defense. The recommendation is Option A; this PRP also adds the Mode A
             comment the DOCS clause requires."

# MUST READ — the function under edit (reapStaleLocks + its current comment + the import block + writeContents).
- file: internal/lock/lock.go
  why: "reapStaleLocks (~line 128): the verbatim body to edit. Its doc comment (~line 113) is the Mode A target.
        writeContents (~line 248) explains the in-place Seek→Write→Truncate→Sync rewrite a concurrent acquirer
        performs (why the re-read sees different bytes). The import block (~line 14): bufio, crypto/sha256,
        encoding/hex, errors, fmt, os, path/filepath, strconv, strings, sync/atomic, time — ADD 'bytes'."
  pattern: "reapStaleLocks is best-effort (all errors ignored; Glob/ReadFile/Remove failures are no-ops). The
            re-read + staleLockUnchanged MUST preserve that discipline (a re-read error ⇒ skip, not fatal)."
  gotcha: "Do NOT touch Acquire, Release, Status, IsOrphaned, parseContents, writeContents, lockPath, or the
           current singleton. ONLY reapStaleLocks + the new helper + the comment + the import."

# MUST READ — the existing reap test + the writeLockFile helper (the idiom to mirror for the new direct test).
- file: internal/lock/lock_unix_test.go
  why: "writeLockFile(t, path, pid, hostname) (line 58) writes the exact key=value format reapStaleLocks reads.
        TestAcquire_ReapsDeadPidFile_SparesLive (line 77) plants dead/live/foreign/malformed fixtures with
        MaxInt32 (≫ pid_max → ESRCH → dead) + os.Getpid (alive) + a foreign hostname + 'not-a-number', calls
        Acquire (which triggers reapStaleLocks), and asserts dead REAPED / others SPARED. MIRROR this for the
        new direct test — but call reapStaleLocks(dir) DIRECTLY (white-box) instead of via Acquire. The existing
        test STAYS GREEN UNCHANGED (dead file still reaped: nothing rewrites it between the two reads)."
  pattern: "t.Setenv('XDG_RUNTIME_DIR', t.TempDir()) + t.Setenv('XDG_CACHE_HOME','') to isolate; os.MkdirAll(dir,
            0o700); writeLockFile fixtures; call reapStaleLocks(dir); assert via os.Stat + os.IsNotExist."
  gotcha: "The package is white-box (`package lock`) — tests call UNEXPORTED reapStaleLocks/processAlive directly.
           Unix-only (//go:build !windows): the dead-pid assertion needs ESRCH (Windows processAlive is always-true)."

# CONTEXT — the parallel sibling PRP (P1.M3.T4.S1) — confirms ZERO overlap (doc-only vs code).
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M3T4S1/PRP.md
  why: "Confirms the parallel item is DOC-ONLY: edits docs/packaging.md + README.md (the upgrade subsystem doc
        sync). It touches NO .go file. This item's internal/lock/lock.go edit has ZERO overlap. No merge conflict."
  critical: "Do NOT edit docs/packaging.md or README.md (the parallel item owns them). This item's [Mode A] is a
             CODE COMMENT on reapStaleLocks, NOT user-facing docs (README/docs sync is P1.M4.T4.S1)."

# CONTEXT — the lock-package invariant doc (why unlinking a live holder's inode defeats FR52).
- file: internal/lock/lock.go
  section: "the reapStaleLocks SAFETY INVARIANT comment + the Release CRITICAL ORDERING comment + the setSnapshot Issue-4 comment"
  why: "All three explain WHY unlinking/rename-ing a live holder's inode is forbidden (it lets a contender O_CREATE
        a fresh inode and flock it free, defeating FR52). The re-read defense is the SAME invariant applied to the
        reap path: don't unlink a file a concurrent acquirer just took. Read these to phrase the Mode A comment in
        the codebase's own terms (inode-bound flock, O_CREATE fresh inode, FR52)."
```

### Current Codebase tree (relevant slice)

```bash
internal/lock/
  lock.go             # EDIT — reapStaleLocks (re-read gate) + new staleLockUnchanged + Mode A comment + "bytes" import
  lock_unix.go        # READ-ONLY — flock + isWouldBlock (Unix twin); NOT this item
  lock_windows.go     # READ-ONLY — flock/processAlive always-true (Windows twin); BUG-010's territory, NOT this item
  orphan_unix.go      # READ-ONLY — appearsOrphaned (Unix); BUG-011's territory, NOT this item
  orphan_windows.go   # READ-ONLY — appearsOrphaned always-false (Windows twin); NOT this item
  lock_test.go        # EDIT — +TestStaleLockUnchanged (pure, cross-platform)
  lock_unix_test.go   # EDIT — +TestReapStaleLocks_DirectDeadPidRemoved (Unix-only); existing reap tests UNCHANGED
Makefile              # READ-ONLY — test (-race); lint; build
go.mod                # READ-ONLY — UNCHANGED (bytes is stdlib)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/lock/lock.go             # EDIT — +staleLockUnchanged; reapStaleLocks re-read gate; Mode A comment; +"bytes" import
internal/lock/lock_test.go        # EDIT — +TestStaleLockUnchanged
internal/lock/lock_unix_test.go   # EDIT — +TestReapStaleLocks_DirectDeadPidRemoved
# NOTHING ELSE. No edit to lock_unix.go/lock_windows.go, orphan_*.go, Acquire/Release/Status/IsOrphaned,
# parseContents/writeContents/lockPath, any non-lock file, go.mod, or any PRD/task file.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the defense compares BYTES, not just pid): use bytes.Equal(original, reread). A concurrent acquirer
// writes a fresh pid AND timestamp (writeContents rewrites the whole file). Even if the new pid number equals
// the dead one (pid reuse), the timestamp differs → bytes differ → skip. A pid-only check would miss pid reuse.

// CRITICAL (the re-read error ⇒ SKIP, never fatal): reapStaleLocks is best-effort throughout (Glob/ReadFile/
// Remove errors are ignored, never returned). The re-read's error path MUST preserve this: a failed re-read ⇒
// staleLockUnchanged returns false ⇒ skip the remove (conservative) — do NOT propagate an error.

// CRITICAL (do NOT touch Acquire/Release/Status/IsOrphaned or the twins): reapStaleLocks is the ONLY edit site
// in lock.go (+ the new helper + the comment + the import). The flock/processAlive twins (lock_unix.go/
// lock_windows.go) and the orphan heuristics (orphan_*.go) are other bugs' territory (BUG-010/BUG-011). The
// re-read defense is platform-independent (pure os.ReadFile + bytes.Equal) — it has no build tag and survives
// unchanged on every OS.

// GOTCHA (the "changed → skip" branch is unreachable end-to-end in a single goroutine): both os.ReadFile calls
// in reapStaleLocks return identical bytes when nothing concurrently rewrites the file. So the defense branch
// can ONLY be tested via the pure helper (staleLockUnchanged) or a flaky concurrent writer. The pure helper is
// the idiomatic, deterministic answer — extract it, don't try to test the branch through reapStaleLocks.

// GOTCHA (the existing TestAcquire_ReapsDeadPidFile_SparesLive stays GREEN): in that test the dead file is
// planted and nothing rewrites it between the two reads → bytes equal → staleLockUnchanged true → removed. The
// existing "dead-pid file should be REAPED" assertion still holds. Do NOT edit that test. live/foreign/malformed
// fixtures never reach the re-read (processAlive true / Atoi error → skip earlier).

// GOTCHA (white-box tests call UNEXPORTED reapStaleLocks directly): lock_unix_test.go is `package lock` (not
// lock_test), so the new test calls reapStaleLocks(dir) DIRECTLY — no Acquire machinery. This isolates the
// function under test (tighter than the existing Acquire-based test).

// GOTCHA (Unix-only test for the dead-pid assertion): the dead-pid-removed assertion needs ESRCH (Kill(pid,0)
// returns ESRCH for a dead pid). Windows processAlive is always-true → a dead-pid file would NOT be reaped →
// the assertion would fail on Windows CI. Place TestReapStaleLocks_DirectDeadPidRemoved in lock_unix_test.go
// (//go:build !windows), mirroring TestAcquire_ReapsDeadPidFile_SparesLive. The pure TestStaleLockUnchanged is
// cross-platform (lock_test.go, no build tag).

// GOTCHA (NEW "bytes" import): lock.go does NOT currently import "bytes" (its imports are bufio/crypto/sha256/
// encoding/hex/errors/fmt/os/path/filepath/strconv/strings/sync/atomic/time). bytes.Equal needs it. Add "bytes"
// to the import block (stdlib, alphabetized: after bufio). go.mod unchanged.
```

## Implementation Blueprint

### Data models and structure

None NEW. `staleLockUnchanged` is a plain pure function (`func([]byte, []byte, error) bool`); no new types,
structs, fields, or sentinels. The edit reuses the existing `os.ReadFile` / `os.Remove` / `parseContents` /
`processAlive`. One new stdlib import (`"bytes"`).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/lock/lock.go — add "bytes" import + staleLockUnchanged + the reapStaleLocks re-read gate + the Mode A comment
  - STEP 1a — ADD "bytes" to the import block (alphabetized, after "bufio"). It is the ONLY new import.
  - STEP 1b — ADD the pure helper (place it immediately BEFORE reapStaleLocks, since reapStaleLocks is its sole caller):
      // staleLockUnchanged reports whether the lock file's current contents still match the contents we
      // liveness-checked (BUG-009 defense-in-depth). A concurrent acquirer that took the lock between our
      // processAlive check and the os.Remove would have rewritten the file in place (writeContents is a
      // Seek→Write→Truncate→Sync on the inode) → bytes differ → the caller SKIPS the remove (removing would
      // unlink a live holder's inode, letting a contender O_CREATE a fresh inode and flock it free — FR52
      // defeat). A re-read error → skip (conservative; reapStaleLocks is best-effort throughout). Byte-equal
      // comparison (not just pid) so pid-reuse-with-a-new-timestamp is also caught.
      func staleLockUnchanged(original, reread []byte, rerr error) bool {
          return rerr == nil && bytes.Equal(original, reread)
      }
  - STEP 1c — EDIT reapStaleLocks's doc comment (Mode A — the DOCS clause). Rewrite it to document:
      (1) the TOCTOU window (read → processAlive → os.Remove, with a concurrent acquirer able to take the lock
      in between); (2) the defense (re-read before remove; skip if staleLockUnchanged is false); (3) the residual
      micro-window (narrowed, not fully closed — C could rewrite between the re-read and the remove); (4) the
      CAS/flock mitigation bounding the residual (a live holder's flock survives an unlink — only the path is
      stale; the next Acquire recreates the file via O_CREATE; the holder's own live-pid file is never reaped).
      KEEP the existing facts: called from Acquire AFTER the holder's own flock succeeds; the holder's pid is
      os.Getpid (live) so its own file is never reaped; a LIVE pid is NEVER reaped (processAlive conservative);
      malformed/empty pid → skip; all errors ignored (best-effort).
  - STEP 1d — EDIT the reapStaleLocks BODY. CURRENT:
        if !processAlive(pid, c.Hostname) {
            os.Remove(f) // dead pid → safe to unlink (ignore error)
        }
    REPLACE WITH:
        if !processAlive(pid, c.Hostname) {
            // BUG-009 defense-in-depth: re-read just before remove. A concurrent acquirer may have taken the
            // lock on this path after our liveness check (the dead holder's flock auto-released on death) and
            // rewritten this file with its own live pid. Removing now would unlink a live holder's inode → a
            // contender could O_CREATE a fresh inode and flock it free (FR52 defeat). Skip if the contents
            // changed or the re-read failed; the next reap cycle re-evaluates.
            reread, rerr := os.ReadFile(f)
            if staleLockUnchanged(data, reread, rerr) {
                os.Remove(f) // dead + unchanged → safe to unlink (ignore error)
            }
        }
    PRESERVE: the `data, err := os.ReadFile(f)` first read (unchanged); the `if err != nil { continue }`; the
    `parseContents`; the `strconv.Atoi` + `if err != nil { continue }`; the `!processAlive(...)` guard. ONLY the
    body of the `if !processAlive` block changes (one os.Remove → re-read + gate).
  - NAMING: staleLockUnchanged (descriptive; "the stale lock's contents are unchanged since we checked").
  - GOTCHA: `data` (the first read's bytes) is already in scope — pass it as `original`. Do NOT re-parse pid from
    `reread`; compare BYTES (catches timestamp changes too).
  - VERIFY: `go build ./internal/lock/...` clean; `gofmt -w internal/lock/lock.go`.

Task 2: ADD TestStaleLockUnchanged to internal/lock/lock_test.go (PURE, cross-platform — no build tag)
  - PLACEMENT: anywhere in lock_test.go (it tests a pure helper; co-locate near the other lock.go-unit tests,
    e.g. after TestIsHeldError at the end of the file).
  - IMPORTS: "errors" + "testing" already imported in lock_test.go? VERIFY (lock_test.go imports — if "errors"
    is missing, add it). The test constructs two distinct byte slices + an error.
  - BODY:
      func TestStaleLockUnchanged(t *testing.T) {
          dead := []byte("pid=999999\nhostname=h\nrepo=fake\ntimestamp=t1\nsnapshot=\n")
          live := []byte("pid=1000\nhostname=h\nrepo=fake\ntimestamp=t2\nsnapshot=\n") // concurrent acquirer rewrote
          // same bytes, no error → remove (the happy path: nothing rewrote the file).
          if !staleLockUnchanged(dead, dead, nil) {
              t.Errorf("staleLockUnchanged(same) = false, want true (unchanged → remove)")
          }
          // changed bytes → skip (BUG-009 defense: a concurrent acquirer rewrote the file).
          if staleLockUnchanged(dead, live, nil) {
              t.Errorf("staleLockUnchanged(changed) = true, want false (rewrite detected → skip)")
          }
          // re-read error → skip (conservative; best-effort reap).
          if staleLockUnchanged(dead, nil, errors.New("read error")) {
              t.Errorf("staleLockUnchanged(readErr) = true, want false (re-read failed → skip)")
          }
      }
  - NAMING: TestStaleLockUnchanged (mirrors the helper).
  - GOTCHA: cross-platform (no processAlive, no ESRCH) → lock_test.go (NOT lock_unix_test.go). The "changed" case
    is the BUG-009 defense branch (unreachable end-to-end → tested here via the pure helper).

Task 3: ADD TestReapStaleLocks_DirectDeadPidRemoved to internal/lock/lock_unix_test.go (Unix-only)
  - HEADER: the file is already `//go:build !windows` + `package lock`. No new build tag. IMPORTS already present
    (math, os, path/filepath, strconv, testing — VERIFY math is imported; the existing test uses math.MaxInt32).
  - PLACEMENT: immediately AFTER TestAcquire_ReapsDeadPidFile_SparesLive (ends ~line 133), before
    TestAppearsOrphaned_DeadPidIsConservativeFalse (~line 135). Keep the section-separator comment style.
  - BODY (mirror the existing reap test's fixture setup, but call reapStaleLocks DIRECTLY):
      // TestReapStaleLocks_DirectDeadPidRemoved calls the UNEXPORTED reapStaleLocks directly (white-box) with a
      // planted dead-pid fixture and asserts the file is REMOVED. This proves the BUG-009 re-read defense does
      // NOT false-skip the happy path: with no concurrent writer, the two reads return identical bytes →
      // staleLockUnchanged true → remove. (Unix-only: the dead-pid assertion needs ESRCH; Windows processAlive
      // is always-true.) The changed-content branch of the defense is pinned by TestStaleLockUnchanged.
      func TestReapStaleLocks_DirectDeadPidRemoved(t *testing.T) {
          resetCurrent(t)
          t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // isolate — don't touch the real lock dir
          t.Setenv("XDG_CACHE_HOME", "")
          dir, err := lockDir()
          if err != nil { t.Fatalf("lockDir: %v", err) }
          if err := os.MkdirAll(dir, 0o700); err != nil { t.Fatalf("MkdirAll: %v", err) }

          thisHost, _ := os.Hostname()
          deadPath := filepath.Join(dir, "dead.lock")
          writeLockFile(t, deadPath, strconv.Itoa(math.MaxInt32), thisHost) // MaxInt32 ≫ pid_max → ESRCH → dead
          // a live-pid fixture proves the safety invariant still holds (processAlive true → never reaches the re-read).
          livePath := filepath.Join(dir, "live.lock")
          writeLockFile(t, livePath, strconv.Itoa(os.Getpid()), thisHost) // self → alive

          reapStaleLocks(dir) // DIRECT call (white-box) — no Acquire machinery

          if _, err := os.Stat(deadPath); !os.IsNotExist(err) {
              t.Errorf("dead-pid file should be REAPED (ESRCH + unchanged re-read), still present: %v", err)
          }
          if _, err := os.Stat(livePath); err != nil {
              t.Errorf("live-pid file should be SPARED (alive → never reaped), missing: %v", err)
          }
      }
  - NAMING: TestReapStaleLocks_DirectDeadPidRemoved (the "Direct" signals it calls reapStaleLocks, not Acquire).
  - FOLLOW pattern: TestAcquire_ReapsDeadPidFile_SparesLive's XDG isolation + writeLockFile + MaxInt32 + os.Stat/
    os.IsNotExist assertions. Reuse `resetCurrent(t)` + `writeLockFile` + `lockDir` (all package-local).
  - GOTCHA: `resetCurrent(t)` resets the `current` singleton (mirrors the existing test's hygiene). `math.MaxInt32`
    must be imported (the existing test uses it — confirm `math` is in lock_unix_test.go's imports; if not, add it).

Task 4: VERIFY — build (native+cross), vet, format, tests, lint, grep guards
  - go build ./... ; GOOS=linux go build ./... ; GOOS=darwin go build ./... ; GOOS=windows go build ./...
  - go vet ./internal/lock/... ; gofmt -l internal/lock/lock.go internal/lock/lock_test.go internal/lock/lock_unix_test.go
  - go test ./internal/lock/ -run 'StaleLockUnchanged|ReapStaleLocks' -race -v   # the 2 new + the existing reap tests
  - go test ./internal/lock/ -race                                              # full lock regression (existing tests green)
  - make test && make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the pure defense predicate — extracted for deterministic testability, mirroring validateFormat/parseTimeout):
func staleLockUnchanged(original, reread []byte, rerr error) bool {
	return rerr == nil && bytes.Equal(original, reread)
}

// PATTERN (the re-read gate inside reapStaleLocks — best-effort preserved; skip-on-changed/error):
if !processAlive(pid, c.Hostname) {
	reread, rerr := os.ReadFile(f)
	if staleLockUnchanged(data, reread, rerr) {
		os.Remove(f) // dead + unchanged → safe to unlink
	}
	// else: contents changed (concurrent acquirer) OR re-read failed → skip (BUG-009)
}

// PATTERN (the direct white-box test — calls UNEXPORTED reapStaleLocks, no Acquire machinery):
t.Setenv("XDG_RUNTIME_DIR", t.TempDir()); t.Setenv("XDG_CACHE_HOME", "")
dir, _ := lockDir(); os.MkdirAll(dir, 0o700)
writeLockFile(t, filepath.Join(dir, "dead.lock"), strconv.Itoa(math.MaxInt32), thisHost) // ESRCH → dead
reapStaleLocks(dir) // DIRECT
if _, err := os.Stat(deadPath); !os.IsNotExist(err) { t.Errorf("dead-pid file should be REAPED") }
```

### Integration Points

```yaml
PRODUCTION (internal/lock/lock.go):
  - staleLockUnchanged(original, reread []byte, rerr error) bool — NEW pure predicate.
  - reapStaleLocks(dir) — EDIT: re-read gate before os.Remove (skip on changed/error); Mode A comment rewritten.

IMPORTS:
  - lock.go: +"bytes" (stdlib, the only new import). go.mod UNCHANGED.

TESTS (internal/lock/):
  - lock_test.go: +TestStaleLockUnchanged (pure, cross-platform).
  - lock_unix_test.go: +TestReapStaleLocks_DirectDeadPidRemoved (Unix-only, direct reapStaleLocks call).

NO database / migration / routes / config / new types / new sentinels / exit-code / flag / signature change.
SCOPE FENCES:
  - Touches ONLY: internal/lock/lock.go + lock_test.go + lock_unix_test.go.
  - Does NOT edit: lock_unix.go/lock_windows.go (flock/processAlive twins — BUG-010's territory),
    orphan_unix.go/orphan_windows.go (appearsOrphaned — BUG-011's territory), Acquire/Release/Status/IsOrphaned/
    parseContents/writeContents/lockPath, any non-lock file, go.mod, any PRD/task file.
  - NO user-facing doc change (the Mode A comment is a CODE comment; README/docs sync is P1.M4.T4.S1).
  - Parallel-safe: P1.M3.T4.S1 (doc-only, docs/packaging.md + README.md) has ZERO overlap.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native + cross-build (bytes.Equal + the helper compile everywhere; reapStaleLocks has no build tag).
go build ./...
GOOS=linux   go build ./...
GOOS=darwin  go build ./...
GOOS=windows go build ./...
# Expected: all clean.

# Vet the lock package.
go vet ./internal/lock/...
# Expected: clean.

# Format the 3 touched files.
gofmt -l internal/lock/lock.go internal/lock/lock_test.go internal/lock/lock_unix_test.go
# Expected: empty. If listed: gofmt -w <those files>.

# Lint.
make lint   # errcheck/gosimple/govet/ineffassign/staticcheck/unused
# Expected: zero errors. (staleLockUnchanged is used by reapStaleLocks; the new tests reference it. No unused symbols.)

# Scope guard: ONLY the 3 lock files.
git status --porcelain
# Expected: internal/lock/lock.go, internal/lock/lock_test.go, internal/lock/lock_unix_test.go. ZERO changes to
#           lock_unix.go/lock_windows.go, orphan_*.go, any non-lock file, docs/*, go.mod.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The 2 new tests + the existing reap tests.
go test ./internal/lock/ -run 'StaleLockUnchanged|ReapStaleLocks|ReapsDeadPidFile' -race -v
# Expected: ALL PASS —
#   TestStaleLockUnchanged (NEW): same→true; changed→false; readErr→false.
#   TestReapStaleLocks_DirectDeadPidRemoved (NEW): dead file REMOVED; live file SPARED (direct reapStaleLocks call).
#   TestAcquire_ReapsDeadPidFile_SparesLive (EXISTING): UNCHANGED, still green (dead reaped: bytes equal → removed).

# Full lock-package regression (the new tests + all existing: processAlive/orphan/status/IsOrphaned/snapshot).
go test ./internal/lock/ -race
# Expected: green. (Windows CI runs the !windows-file-excluded subset; TestReapStaleLocks_DirectDeadPidRemoved is
#           Unix-only — skipped on windows-latest, mirroring TestAcquire_ReapsDeadPidFile_SparesLive.)

# Full race suite + lint + build.
make test && make lint && make build
# Expected: all green.
```

### Level 3: Integration Testing (System Validation)

```bash
# This item is a unit-level defense in an internal package; there is no CLI/HTTP surface to integration-test.
# The existing §20.5 lock-contention e2e (internal/e2e/lock_scenarios_test.go) exercises reapStaleLocks via real
# Acquire on real temp repos. It is UNAFFECTED (the happy path is byte-identical: dead files are still reaped).
go test -tags=e2e ./internal/e2e/ -run 'Lock' -race 2>/dev/null || echo "(e2e suite optional; the unit tests are the proof)"
# Expected: green (or skipped if the e2e harness isn't run in this env). The defense has no user-visible behavior
#           change beyond "skip remove if the file changed under the reaper" — invisible to the e2e happy paths.
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: staleLockUnchanged exists + is pure (bytes.Equal + rerr-nil).
grep -n 'func staleLockUnchanged(original, reread \[\]byte, rerr error) bool' internal/lock/lock.go  # 1 hit
grep -n 'bytes.Equal(original, reread)' internal/lock/lock.go   # 1 hit (in the helper)

# Guard 2: reapStaleLocks re-reads before remove + gates on staleLockUnchanged.
grep -n 'reread, rerr := os.ReadFile(f)' internal/lock/lock.go               # 1 hit (the defense re-read)
grep -n 'staleLockUnchanged(data, reread, rerr)' internal/lock/lock.go       # 1 hit (the gate)
grep -n 'os.Remove(f)' internal/lock/lock.go                                  # 1 hit (now inside the gate's if)

# Guard 3: the bare unconditional os.Remove is GONE (the old `os.Remove(f)` right after processAlive).
# (Before the fix there was exactly one `os.Remove(f)` inside the `if !processAlive` block, unconditional.)
grep -n -A3 'if !processAlive(pid, c.Hostname)' internal/lock/lock.go | grep -v 'reread\|staleLockUnchanged\|os.Remove\|BUG-009\|--' | grep 'os.Remove' && echo "FAIL: unconditional remove remains" || echo "OK: remove is gated on staleLockUnchanged"

# Guard 4: the "bytes" import is present.
grep -n '"bytes"' internal/lock/lock.go   # 1 hit

# Guard 5: the Mode A comment documents the TOCTOU + the defense.
grep -n 'BUG-009' internal/lock/lock.go                              # ≥1 hit (the comment)
grep -n 'TOCTOU\|concurrent acquirer\|re-read' internal/lock/lock.go # ≥1 hit (the Mode A documentation)

# Guard 6: the 2 new tests exist.
grep -n 'func TestStaleLockUnchanged' internal/lock/lock_test.go            # 1 hit
grep -n 'func TestReapStaleLocks_DirectDeadPidRemoved' internal/lock/lock_unix_test.go  # 1 hit

# Guard 7: scope — only the 3 lock files; the twins/orphan/Acquire are untouched.
git status --porcelain
# Expect: internal/lock/lock.go + internal/lock/lock_test.go + internal/lock/lock_unix_test.go ONLY.
git diff --name-only | grep -vE '^internal/lock/(lock\.go|lock_test\.go|lock_unix_test\.go)$' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
git diff --name-only | grep -E 'lock_unix\.go|lock_windows\.go|orphan_' && echo "FAIL: twin/orphan edited (BUG-010/011 territory)" || echo "OK: twins untouched"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `GOOS=linux/darwin/windows` clean; `go vet ./internal/lock/...` clean; `gofmt -l` empty
- [ ] `make lint` zero errors (staleLockUnchanged used by reapStaleLocks; both new tests reference their targets)
- [ ] `go test ./internal/lock/ -run 'StaleLockUnchanged|ReapStaleLocks|ReapsDeadPidFile' -race -v` green
- [ ] `go test ./internal/lock/ -race` green (full lock regression)
- [ ] `make test` + `make build` clean

### Feature Validation (the BUG-009 defense)
- [ ] `staleLockUnchanged(original, reread, rerr)` = `rerr == nil && bytes.Equal(original, reread)` (grep guards 1,4)
- [ ] `reapStaleLocks` re-reads before remove + gates on `staleLockUnchanged` (grep guards 2,3)
- [ ] The Mode A comment documents the TOCTOU + the defense + the residual (grep guard 5)
- [ ] `TestStaleLockUnchanged` pins same→true / changed→false / readErr→false (grep guard 6)
- [ ] `TestReapStaleLocks_DirectDeadPidRemoved` proves the happy path (dead removed, live spared) (grep guard 6)
- [ ] The existing `TestAcquire_ReapsDeadPidFile_SparesLive` STAYS GREEN UNCHANGED

### Scope-Boundary Validation
- [ ] `git status` shows ONLY the 3 internal/lock/ files (grep guard 7)
- [ ] NO edit to lock_unix.go/lock_windows.go (the flock/processAlive twins — BUG-010), orphan_*.go (BUG-011),
      Acquire/Release/Status/IsOrphaned/parseContents/writeContents/lockPath, any non-lock file, go.mod, any PRD/task file
- [ ] NO new type/sentinel/exit-code/flag/config; the only new import is stdlib `"bytes"`
- [ ] NO user-facing doc change (the Mode A comment is a CODE comment; README/docs is P1.M4.T4.S1)
- [ ] NO overlap with the parallel P1.M3.T4.S1 (doc-only: docs/packaging.md + README.md)

### Code Quality & Docs
- [ ] [Mode A] reapStaleLocks doc comment documents the TOCTOU window, the re-read defense, the residual micro-window,
      and the CAS/flock mitigation bounding the residual (in the codebase's own terms: inode-bound flock, O_CREATE)
- [ ] `staleLockUnchanged` has a godoc explaining the BUG-009 defense (changed ⇒ skip) + the byte-equal rationale
- [ ] The defense preserves reapStaleLocks's best-effort discipline (re-read error ⇒ skip, never fatal)
- [ ] The new tests mirror the existing writeLockFile + MaxInt32 + XDG-isolation idiom; the pure helper test is cross-platform

---

## Anti-Patterns to Avoid

- ❌ Don't compare pid only — compare BYTES (`bytes.Equal(original, reread)`). A concurrent acquirer writes a fresh
  pid AND timestamp; even if the new pid number equals the dead one (pid reuse), the timestamp differs → bytes
  differ → skip. A pid-only check would miss pid reuse (the exact scenario BUG-009 names: "a different process may
  have rewritten the lock with its own pid").
- ❌ Don't propagate the re-read error. `reapStaleLocks` is best-effort throughout (Glob/ReadFile/Remove errors are
  ignored). The re-read's error path MUST be `staleLockUnchanged → false → skip` (conservative), NEVER a returned
  error or a fatal. Don't change reapStaleLocks's signature (it stays `func(dir string)`).
- ❌ Don't try to test the "changed → skip" branch through `reapStaleLocks` end-to-end. Both `os.ReadFile` calls
  return identical bytes in a single-goroutine test (nothing rewrites the file between them), so the branch is
  unreachable without a flaky concurrent writer. Extract the pure `staleLockUnchanged` helper and unit-test IT
  (the idiomatic answer — mirror `validateFormat`/`parseTimeout`/`exitcode.For`).
- ❌ Don't edit `TestAcquire_ReapsDeadPidFile_SparesLive`. It stays GREEN UNCHANGED: the dead file is planted and
  nothing rewrites it between the two reads → bytes equal → `staleLockUnchanged` true → removed (its "dead-pid
  file should be REAPED" assertion still holds). ADD a new direct test; don't modify the existing one.
- ❌ Don't touch the flock/processAlive twins (`lock_unix.go`/`lock_windows.go`) or the orphan heuristics
  (`orphan_*.go`). Those are BUG-010 (Windows reaping) and BUG-011 (subreaper orphan) territory — separate tasks.
  The re-read defense is platform-independent (pure `os.ReadFile` + `bytes.Equal`); it has no build tag and belongs
  in the shared `lock.go`.
- ❌ Don't touch `Acquire`/`Release`/`Status`/`IsOrphaned`/`parseContents`/`writeContents`/`lockPath`. `reapStaleLocks`
  is the ONLY edit site in lock.go (+ the new helper + the comment + the import). The defense is purely additive
  to the reap path.
- ❌ Don't add a build tag to the defense. `reapStaleLocks` is shared (no `//go:build`); the re-read + `bytes.Equal`
  compile and behave identically on every OS. (The dead-pid TEST is Unix-only because the ESRCH assertion needs it,
  mirroring the existing test — but the production defense is cross-platform.)
- ❌ Don't add user-facing docs (README/docs/). The contract's DOCS clause is "[Mode A] Update the code COMMENT on
  reapStaleLocks" — a code comment, not user-facing docs. The README/docs sync for lock/watchdog is P1.M4.T4.S1.
- ❌ Don't forget the `"bytes"` import. lock.go does not currently import it; `bytes.Equal` needs it. Add it to the
  import block (alphabetized, after `"bufio"`). go.mod is unchanged (bytes is stdlib).
- ❌ Don't widen the residual by skipping the remove on a false pretext. `staleLockUnchanged` returns true (remove)
  ONLY when the re-read succeeds AND the bytes are byte-identical. Any difference (changed content, partial write,
  read error) → false → skip. This is the conservative direction: when in doubt, DON'T remove (leave it for the
  next reap cycle). Never remove on ambiguous evidence.