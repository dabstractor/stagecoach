# P1.M4.T1.S1 — Narrow the reapStaleLocks TOCTOU window (BUG-009): findings

## §0 — The bug in one paragraph

`internal/lock/lock.go:reapStaleLocks` (unexported, ~line 128) reaps orphaned `*.lock` files whose recorded
pid is dead. The TOCTOU: it does `data := os.ReadFile(f)` → `processAlive(pid) → dead` → `os.Remove(f)`.
Between the liveness check and the remove, a CONCURRENT acquirer C can take the lock on the SAME path (the
dead holder's flock was auto-released on death → C's `flock` succeeds) and `writeContents`-rewrite `f` with
C's OWN live pid. The reaper then `os.Remove(f)` — unlinking a file whose inode C now holds flocked. On Unix
the unlink removes the directory entry but C's open fd (and flock) survive on the now-unlinked inode; the
NEXT contender `OpenFile(f, O_CREATE)`s a FRESH inode and flocks it free → two holders → FR52 defeated.

This is the contract's BUG-009. The contract offers two fixes: (A) defense-in-depth re-read before remove
(RECOMMENDED — "cheap, effective"), or (B) documentation only. This PRP implements **Option A** (the re-read
defense) AND the **[Mode A] comprehensive comment** (which the DOCS clause requires regardless) — the
strongest outcome.

## §1 — Why the re-read defense works (and its residual)

The defense: after `processAlive(pid) → dead`, RE-READ `f` just before `os.Remove`; if the bytes DIFFER from
the `data` we liveness-checked, a concurrent acquirer rewrote the file → SKIP the remove. Comparison is
`bytes.Equal(data, reread)` (byte-identical, NOT just pid — catches pid-reuse-with-new-timestamp too: a new
holder writes a fresh timestamp, so even an identical reused pid number yields different bytes).

- **C rewrote BEFORE the re-read** → re-read sees C's bytes → differ → skip. ✓ (the common race, closed)
- **C rewrites AFTER the remove** → irrelevant (already removed; C's `O_CREATE` makes a fresh inode, which
  is the normal post-reap path — C flocks it legitimately). ✓
- **Residual micro-window**: C rewrites BETWEEN the re-read and `os.Remove` → re-read still saw the dead
  bytes → remove fires → C's inode is unlinked. Vanishingly small (a few instructions), and BOUNDED by the
  CAS/flock mitigation: C's flock survives the unlink (open fd holds the inode); only the PATH is stale; the
  next Acquire recreates the file via `O_CREATE`. So even the residual never produces a corrupt state — at
  worst a stale path that the next Acquire heals. This is the documented, accepted residual.

The re-read is `os.ReadFile(f)` (a fresh fd; the reaper holds no flock on other repos' files). A partial
read (C mid-`writeContents`) → `bytes.Equal` false → skip (conservative). Safe.

## §2 — The exact current code + the verbatim fix

CURRENT (`internal/lock/lock.go`, reapStaleLocks body):
```go
func reapStaleLocks(dir string) {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.lock"))
	for _, f := range matches {
		data, err := os.ReadFile(f)
		if err != nil { continue }
		c := parseContents(data)
		pid, err := strconv.Atoi(c.Pid)
		if err != nil { continue } // malformed/empty pid → skip
		if !processAlive(pid, c.Hostname) {
			os.Remove(f) // dead pid → safe to unlink (ignore error)
		}
	}
}
```

FIX (Option A — re-read before remove, skip if changed):
```go
func reapStaleLocks(dir string) {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.lock"))
	for _, f := range matches {
		data, err := os.ReadFile(f)
		if err != nil { continue }
		c := parseContents(data)
		pid, err := strconv.Atoi(c.Pid)
		if err != nil { continue }
		if !processAlive(pid, c.Hostname) {
			// BUG-009 defense-in-depth: re-read just before remove. A concurrent acquirer may have taken
			// the lock (same path) AFTER our liveness check and rewritten this file with its own live pid
			// (writeContents is an in-place Seek→Write→Truncate→Sync on the inode). Removing now would
			// unlink a live holder's inode, letting a contender O_CREATE a fresh inode and flock it (FR52
			// defeat). If the contents changed (or the re-read failed), SKIP — the next reap re-evaluates.
			if staleLockUnchanged(data, os.ReadFile(f)) {   // os.ReadFile returns (data, err); helper is (original, reread, rerr)
				os.Remove(f)
			}
		}
	}
}
```
NOTE the helper signature — see §3. The `os.ReadFile(f)` second read is the defense; `staleLockUnchanged`
is the extracted pure predicate (byte-identical + no read error).

## §3 — The extracted pure helper `staleLockUnchanged` (for deterministic testability)

The defense's two `os.ReadFile` calls return identical bytes in any single-goroutine test (nothing changes
the file between them), so the "changed → skip" branch is UNREACHABLE end-to-end without a concurrent writer
(flaky) or a seam. The clean, idiomatic answer (the codebase extracts pure helpers: `validateFormat`,
`parseTimeout`, `exitcode.For`) is to extract the DECISION into a pure predicate and unit-test it directly:

```go
// staleLockUnchanged reports whether the lock file's current contents still match the contents we
// liveness-checked (BUG-009 defense-in-depth). A concurrent acquirer that took the lock between our
// processAlive check and the os.Remove would have rewritten the file (writeContents is in-place on the
// inode) → bytes differ → the caller SKIPS the remove. A re-read error → skip (conservative). Byte-equal
// comparison (not just pid) so pid-reuse-with-a-new-timestamp is also caught.
func staleLockUnchanged(original, reread []byte, rerr error) bool {
	return rerr == nil && bytes.Equal(original, reread)
}
```
Signature `(original, reread []byte, rerr error) bool` — takes the second `os.ReadFile`'s `(data, err)` pair
verbatim. The call site is `staleLockUnchanged(data, os.ReadFile(f))` — Go evaluates `os.ReadFile(f)` into
the `([]byte, error)` pair and binds `rerr`. (Or spell it out: `reread, rerr := os.ReadFile(f); if
staleLockUnchanged(data, reread, rerr) { os.Remove(f) }` — equally fine; pick the spelled-out form for clarity.)

NEW IMPORT: `"bytes"` (lock.go does NOT currently import it; stdlib, safe). All other symbols (`os.ReadFile`,
`os.Remove`, `parseContents`, `processAlive`, `strconv`, `filepath`) are already imported.

## §4 — The test design (deterministic, no flakiness)

`reapStaleLocks` is UNEXPORTED but the tests are white-box `package lock` (lock_unix_test.go:19 calls
`processAlive` directly) → the test calls `reapStaleLocks(dir)` DIRECTLY (no Acquire machinery). Two tests:

1. **`TestStaleLockUnchanged`** (lock_test.go — PURE, cross-platform): pins the 3 branches of the helper.
   - `staleLockUnchanged(dead, dead, nil)` → true (remove).
   - `staleLockUnchanged(dead, liveBytes, nil)` → false (skip — the defense).
   - `staleLockUnchanged(dead, nil, errors.New("read err"))` → false (skip — conservative).
2. **`TestReapStaleLocks_DirectDeadPidRemoved`** (lock_unix_test.go — Unix-only, mirrors
   `TestAcquire_ReapsDeadPidFile_SparesLive`): isolate XDG; `os.MkdirAll(dir)`; plant a dead-pid file via
   the existing `writeLockFile(t, path, strconv.Itoa(math.MaxInt32), thisHost)` helper (MaxInt32 ≫ pid_max →
   ESRCH → dead); call `reapStaleLocks(dir)` DIRECTLY; assert the file is REMOVED. Proves the re-read defense
   does NOT false-skip the happy path (the file is unchanged between the two reads → `bytes.Equal` true → remove).

The existing `TestAcquire_ReapsDeadPidFile_SparesLive` (lock_unix_test.go:77) STAYS GREEN UNCHANGED: in that
test the dead file is planted and nothing rewrites it between the two reads → bytes equal → removed (the
existing "dead-pid file should be REAPED" assertion still holds). live/foreign/malformed fixtures never reach
the re-read (processAlive true / Atoi error → skip earlier). NO edit to that test.

## §5 — The [Mode A] comment (the DOCS clause — required regardless of A/B)

Rewrite `reapStaleLocks`'s doc comment to document: (a) the TOCTOU window (check-then-remove); (b) the
defense (re-read before remove; skip if changed via `staleLockUnchanged`); (c) the residual micro-window
(narrowed, not fully closed); (d) the CAS/flock mitigation that bounds the residual (a live holder's flock
survives an unlink; only the path is stale; the next Acquire heals via `O_CREATE`). This is the contract's
"[Mode A] Update the code comment on reapStaleLocks to document the TOCTOU mitigation. No user-facing doc
surface change." (User-facing docs — README/docs/ — are P1.M4.T4.S1, NOT this item.)

## §6 — Scope fences (parallel-aware)

- Touches ONLY: `internal/lock/lock.go` (reapStaleLocks + the new `staleLockUnchanged` helper + the
  rewritten comment + the `"bytes"` import) + `internal/lock/lock_test.go` (`TestStaleLockUnchanged`) +
  `internal/lock/lock_unix_test.go` (`TestReapStaleLocks_DirectDeadPidRemoved`).
- **P1.M3.T4.S1** (parallel, doc-only): edits `docs/packaging.md` + `README.md` — ZERO overlap (code vs docs).
- **P1.M4.T2.S1** (BUG-010, Windows reaping): PLANNED. Its scope is `processAlive` in `lock_windows.go`
  (always-true twin), NOT `reapStaleLocks` (lock.go, shared, no build tag). No overlap. (If that task later
  splits reapStaleLocks per-OS it would be a SEPARATE edit; this item's re-read is platform-independent and
  survives any such split.)
- **P1.M4.T3.S1** (BUG-011, orphan subreaper): PLANNED. Touches `appearsOrphaned` in `orphan_unix.go`. No overlap.
- **P1.M4.T4.S1** (lock/watchdog docs sync): PLANNED. README + docs/. This item's [Mode A] is a CODE COMMENT
  (not user-facing docs) — no overlap.
- NO edit to `lock_unix.go`/`lock_windows.go` (the `flock`/`processAlive` twins), `orphan_*.go`, `Acquire`,
  `Release`, `Status`, `IsOrphaned`, or any non-lock file.

## §7 — Validation

- `go build ./...` + `GOOS={linux,darwin,windows}` clean (`bytes.Equal` + the helper compile everywhere;
  `reapStaleLocks` has no build tag).
- `go vet ./internal/lock/...`; `gofmt -l` on the 3 files.
- `go test ./internal/lock/ -run 'StaleLockUnchanged|ReapStaleLocks' -race -v` green.
- `go test ./internal/lock/ -race` green (incl. the existing `TestAcquire_ReapsDeadPidFile_SparesLive` —
  unchanged, still green: the dead file is still reaped because nothing rewrites it between the two reads).
- `make test` + `make lint`.
- grep guards: `bytes.Equal` present; `staleLockUnchanged` defined + called; re-read before remove; no
  edit to lock_unix/windows/orphan/Acquire/Release.