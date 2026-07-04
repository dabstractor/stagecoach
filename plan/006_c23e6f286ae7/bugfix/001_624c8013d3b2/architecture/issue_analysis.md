# Issue Analysis & Fix Approaches (Bugfix 001)

All four issues confirmed against the live codebase. Root causes, chosen fixes, and exact
edit sites below. Cross-references the architecture findings.

---

## Issue 1 (Major) — No-op fast path never fires on the decompose path

### Confirmed root cause
- Holder on decompose path publishes `snapshot = T_start` (working-tree tree) at
  `internal/decompose/decompose.go:169` (`lock.SetSnapshot(tStart)`).
- `T_start` comes from `FreezeWorkingTree` (`internal/git/git.go:1340-1361`): `AddAll →
  WriteTree → tStart → ReadTree(baseTree)`. After step 3, the index is reset to `baseTree`
  (= `HEAD^{tree}`).
- Contender's `handleLockContention` computes `contenderTree = g.WriteTree(ctx)` which
  snapshots the **index**. The contender also has nothing staged (decompose context), so its
  index == baseTree (also reset by the holder's FreezeWorkingTree on the shared `.git/index`).
- `contenderTree (baseTree) == snap (T_start)` is **always false** when the working tree has
  changes → contender exits Busy(5), never the documented exit 0.

### Chosen fix: Option 1 (qualify documentation + regression e2e) — PRD-recommended, lowest-risk
The PRD explicitly recommends Option 1 ("the lowest-risk change and makes the docs honest")
over the invasive Option 2 (publishing baseTree or having the contender mirror
FreezeWorkingTree, which risks false no-ops and violates the "index-read-only, safe without
the lock" G4 invariant).

**Deliverables:**
1. **Qualify docs** to scope the no-op fast path to the single-commit (staged) path; state
   that decompose accidental double-runs exit Busy(5):
   - `README.md:330` — the "Safe to run twice" paragraph.
   - `docs/cli.md:379` — contention-behavior prose.
   - `docs/how-it-works.md:155` — "No-op fast path" subsection.
   - **DO NOT edit `PRD.md` §18.5** (read-only spec, owned by humans).
2. **Add e2e regression scenario** (`internal/e2e/lock_scenarios_test.go` scenario F) that
   reproduces the decompose accidental double-run and asserts exit 5 (Busy) + the busy
   message, so the documented behavior cannot regress silently.

### Why Option 2 (code fix) is NOT chosen
- Publishing `baseTree` as the snapshot loses the "same change set" guarantee and could cause
  false no-ops (two unrelated dirty-tree runs).
- Having the contender mirror `FreezeWorkingTree` (`AddAll → WriteTree → ReadTree` restore)
  mutates the contender's index without the lock — violates the documented G4 invariant
  ("index-read-only, safe to take without the lock") and the §13.4 stage-while-generating
  safety boundary.
- The contender would need to know it's on the decompose path BEFORE acquiring the lock
  (`shouldDecompose` is evaluated after Acquire succeeds) — non-trivial restructure.

---

## Issue 2 (Minor) — Lock files accumulate, never removed

### Confirmed root cause
`Release()` (`internal/lock/lock.go:110-123`) only closes the fd + clears the singleton. It
does **not** `os.Remove(l.path)`. Every distinct repo path leaves a permanent `<hash>.lock`.

### Chosen fix: remove on release (PRD option b — "the conventional pattern for flock lock files")
After `l.file.Close()`, attempt `os.Remove(l.path)` ignoring errors. Safe because:
- flock auto-released on fd close → by the time another process calls `Acquire`, it
  `OpenFile(O_CREATE|O_RDWR)` which recreates the file if absent.
- If a process is concurrently blocked... actually nothing blocks (LOCK_NB never blocks), and
  a removed file just gets recreated on next Acquire. `os.Remove` on a file whose fd is
  already closed is a no-op on the inode (link count drops; the holder already closed its fd).
- The remove-after-close ordering is critical: close FIRST (release flock), THEN remove.

### Test
Unit test: Acquire → Release → assert file no longer exists at `l.path`. Also assert Release
idempotency still holds (second Release no-op, no panic on missing file).

---

## Issue 3 (Minor) — `repo=` field uses the non-canonical CWD path

### Confirmed root cause
`Acquire` (`internal/lock/lock.go:102`) sets `repo: repoPath` (the raw input = `os.Getwd()`
from `runDefault`). But `lockHash` (`lock.go:204-213`) canonicalizes via
`filepath.EvalSymlinks` for the filename. So two terminals reaching the same repo via
symlink vs real path share one lock file but write different `repo=` values — last writer
wins, and the contender's Busy message shows the holder's raw CWD, not the canonical path.

### Chosen fix: store the canonical path in the `repo=` field
Refactor `lockHash` to return `(canonical string, hash string)` (DRY — single
`EvalSymlinks` call), update `lockPath` to use the hash, and have `Acquire` store the
canonical path in `l.repo`. This makes the diagnostic `repo=` agree with the hash key.

### Edit sites
- `lockHash(repoPath) string` → `lockHash(repoPath string) (canonical, hash string)` (or add
  a sibling `canonicalRepo(repoPath) string` reused by both `lockHash` and `Acquire`).
- `lockPath` (lock.go:215-222): call the refactored function, use hash only.
- `Acquire` (lock.go:102): `repo: canonical` instead of `repo: repoPath`.
- `lockHash`/`lockPath` tests: update for new signature; `TestHash_CanonicalSymlink` etc.

### Test
New unit test: create a repo, symlink it elsewhere, Acquire via the symlink path, assert the
written `repo=` field equals the canonical (real) path, not the symlink path. (Currently no
test exercises this drift.)

---

## Issue 4 (Minor) — SetSnapshot rewrite observable mid-write (truncated/partial read)

### Confirmed root cause
`setSnapshot` (`internal/lock/lock.go:131-140`) does `Truncate(0) → Seek(0,0) →
writeContents(sha)`. A contender's `os.ReadFile(path)` (separate open/read/close in `Acquire`
on EWOULDBLOCK) landing between Truncate and writeContents completion reads an EMPTY or
partial file → `parseContents` yields empty fields → Busy message renders as
`"on  (pid  on )"` (ugly, uninformative). Functionally conservative (empty snapshot → skip
no-op → Busy is safe), but the diagnostic is broken.

### Chosen fix: in-place atomic-ish rewrite + empty-field guard
See `architecture/flock_inode_constraint.md` for the **critical** constraint: **temp-file +
`os.Rename` is FORBIDDEN** (flock is inode-bound; rename orphans the holder's flock and
breaks contention detection).

1. **Rewrite in place** on the held fd: build the full content string, then
   `Seek(0,0) → Write(full) → Truncate(len) → Sync` (Write-BEFORE-Truncate so the file is
   never empty — always old content or new content, never empty).
2. **Guard `handleLockContention`**: if `repo`/`pid`/`hostname` are empty, substitute
   sensible fallbacks so the message is never gibberish.

### Edit sites
- `setSnapshot` / `writeContents` (`lock.go:131-140, 167-174`): restructure to
  Seek→Write→Truncate on the full buffer (single Write call).
- `handleLockContention` (`cmd/default_action.go:251-254`): guard empty fields.

### Tests
- Unit test: a contender reading during/after a snapshot rewrite always sees a non-empty,
  well-formed file (or at least non-empty pid/hostname/repo). Hard to test the race
  deterministically; focus on the write-ordering invariant (file is never empty after
  setSnapshot — write-before-truncate).
- Unit test: `handleLockContention` with empty repo/pid/hostname → message contains
  sensible fallback text, not empty parens.
