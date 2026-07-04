# Research — P1.M2.T2.S1: Store canonical path in repo= field + symlink diagnostic test (Issue 3)

> Scope: the FR52 lock-file `repo=` diagnostic field currently stores the RAW `repoPath` (the holder's
> CWD, possibly a symlink) while the lock FILENAME hash uses the CANONICAL path (`filepath.EvalSymlinks`).
> For a repo reached via a symlink, the contention message therefore prints a path that differs from a
> contender's own CWD — confusing "is that my repo?" (PRD §18.5 Issue 3). Fix: compute the canonical path
> ONCE (in `lockHash`, the existing canonicalization source) and reuse it for BOTH the hash and the `repo=`
> field. Add a symlink-diagnostic unit test proving `repo=` is canonical.
>
> **PREREQUISITE LANDED: P1.M2.T1.S1 (Issue 2).** The current `internal/lock/lock.go` `Release()` ALREADY
> calls `os.Remove(path)` after `Close()` (with the Issue-2 doc comment), and `TestRelease_RemovesLockFile`
> already exists. So **Release removes the lock file** — the new symlink test MUST read the lock file
> contents BEFORE `Release()` (or use the in-memory `l.repo` / `HeldError` state).

---

## 1. The bug + the fix (single source of canonical)

`internal/lock/lock.go`:

- **`lockHash(repoPath string) string`** (L216) ALREADY canonicalizes — `canonical, err := filepath.EvalSymlinks(repoPath); if err != nil { canonical, _ = filepath.Abs(repoPath) }` — but DISCARDS `canonical`, returning only the sha256 hex. (This is why the filename is correct for symlinks.)
- **`Acquire(repoPath)`** (L66) calls `lockPath(repoPath)` (→ `lockHash`) for the file path, then at the
  `Locker` literal stores **`repo: repoPath`** (the RAW input — the bug). `repoPath` is `os.Getwd()` from
  `runDefault`, so for a symlinked CWD it is the symlink, not the real path.
- **`handleLockContention`** (`internal/cmd/default_action.go:254`) prints `heldErr.Contents.Repo` verbatim
  (no transformation) → so once `Acquire` stores canonical, the Busy message automatically prints the
  canonical path. **NO edit needed in default_action.go.**

**Fix (Option 1 — the item's preferred): change `lockHash` to return both, reuse the canonical in `Acquire`.**

```go
// lockHash returns the repo's canonical path and its sha256 hex hash. The canonical path
// (EvalSymlinks, falling back to Abs) is the single source of truth reused by BOTH the lock
// filename (hash) and the diagnostic repo= field (Issue 3) — they always agree.
func lockHash(repoPath string) (canonical, hash string) {
	canonical, err := filepath.EvalSymlinks(repoPath)
	if err != nil {
		canonical, _ = filepath.Abs(repoPath)
	}
	sum := sha256.Sum256([]byte(canonical))
	return canonical, hex.EncodeToString(sum[:])
}

// lockPath — UNCHANGED signature (string, error); internally discards the canonical.
func lockPath(repoPath string) (string, error) {
	dir, err := lockDir()
	if err != nil {
		return "", err
	}
	_, hash := lockHash(repoPath)
	return filepath.Join(dir, hash+".lock"), nil
}

// Acquire — add ONE line to obtain the canonical; store it in repo=.
func Acquire(repoPath string) (*Locker, error) {
	path, err := lockPath(repoPath)
	if err != nil {
		return nil, fmt.Errorf("lock path: %w", err)
	}
	canonical, _ := lockHash(repoPath) // Issue 3: canonical path for the repo= diagnostic
	// ... MkdirAll / OpenFile / flock / contention unchanged ...
	l := &Locker{
		file:      f,
		path:      path,
		pid:       pid,
		hostname:  host,
		repo:      canonical, // ← FIX: was repoPath (raw); now canonical (Issue 3)
		timestamp: ts,
	}
	l.writeContents("")
	current.Store(l)
	return l, nil
}
```

**Why this is correct + safe:**
- `lockHash` is deterministic — calling it twice in `Acquire` (once via `lockPath`, once directly) returns
  the same `(canonical, hash)`. The second `EvalSymlinks` is a cheap syscall on a once-per-run path. (The
  item's "computed ONCE" is about a single canonicalization *implementation*, satisfied — `lockHash` is the
  sole source. If the implementer wants to invoke `EvalSymlinks` literally once, they may inline `lockDir`+
  `hash` in `Acquire` and skip `lockPath` — but that bypasses the `lockPath` abstraction the item says to
  keep. The double-call is the lower-risk choice.)
- The hash/filename is UNCHANGED (same canonical input → same sha256). So lock-file identity, contention,
  and the no-op fast path are all unaffected. Only the diagnostic `repo=` string changes (raw → canonical).
- `lockPath`'s SIGNATURE is unchanged → `TestAcquire_PathMatchesLockPath` + `TestLockPath_CanonicalSymlink`
  are UNAFFECTED. Only `lockHash`'s signature changes.

---

## 2. The FULL test ripple (verified by grep — every lockHash/lockPath caller)

`lockHash`/`lockPath` are UNEXPORTED → only same-package callers (lock.go + lock_test.go). No external
callers exist. The ripple is entirely within `internal/lock/`.

### Tests that MUST change

| Test | Why | Edit |
|------|-----|------|
| `TestHash_CanonicalSymlink` (L108/109/114) | calls `lockHash(...)` expecting 1 return | `_, hash1 := lockHash(tmpRepo)` (×3 calls — hash1, hash2, hash3) |
| `TestAcquireRelease_RoundTrip` (L199) | asserts `c.Repo != repo` (raw `t.TempDir()`) | after the fix `c.Repo` is CANONICAL → on macOS `t.TempDir()` is `/var/folders/...` which resolves to `/private/var/folders/...`, so `canonical != repo` ⇒ the assertion BREAKS. Compute `canonical, _ := lockHash(repo)` and assert `c.Repo != canonical`. |
| `TestSetSnapshot_UpdatesFile` (L225) | same `c.Repo != repo` assertion | same fix: compare against `canonical, _ := lockHash(repo)`. |

### The macOS break (the non-obvious ripple — CRITICAL)

`t.TempDir()` on macOS returns a path under `/var/folders/...`, and `/var` is a symlink to `/private/var`.
`filepath.EvalSymlinks(t.TempDir())` therefore resolves to `/private/var/folders/...` ≠ the raw tmpdir.
Before the fix, `Acquire` stored the raw `repo` so `c.Repo == repo` held. After the fix, `c.Repo` is the
canonical resolved path, so `c.Repo == repo` FAILS on macOS (and on any Linux box where `/tmp` or the
tmpdir root is a symlink). **Both `TestAcquireRelease_RoundTrip` and `TestSetSnapshot_UpdatesFile` must
compare against the canonical**, not the raw `repo`. The robust oracle is `canonical, _ := lockHash(repo)`
(reuses the function under test — no separate `EvalSymlinks` that could diverge).

### Tests UNAFFECTED (verify, don't edit)

- `TestLockPath_CanonicalSymlink` — `lockPath` signature unchanged.
- `TestAcquire_PathMatchesLockPath` — `lockPath` signature unchanged; `l.path` still equals `lockPath(repo)`.
- `TestAcquire_Contention_HeldError` — asserts `he.Contents.Pid` + `he.Path`, NOT `Repo`.
- `TestRelease_RemovesLockFile` (Issue 2) — asserts file removal, not `Repo`.
- `TestSetSnapshot_NilSafeNoOp` / `TestSetSnapshot_MethodAfterRelease` — don't assert `Repo`.
- `TestIsHeldError`, all `TestLockDir_*` — unaffected.

### NEW test (the headline deliverable)

`TestAcquire_RepoFieldIsCanonical` — the symlink diagnostic proof:
- `resetCurrent(t)` + isolate XDG (`t.Setenv("XDG_RUNTIME_DIR", t.TempDir())` + clear `XDG_CACHE_HOME`) —
  the clean pattern from `TestRelease_RemovesLockFile`.
- Create `realRepo` under a temp dir + a `link` symlink to it.
- `canonical, _ := lockHash(link)` — the expected canonical (resolves to `realRepo`).
- `l, err := Acquire(link)` — acquire via the SYMLINK (raw path).
- **Read `os.ReadFile(l.path)` BEFORE `l.Release()`** (Issue 2 removes the file on Release).
- `c := parseContents(data)`; assert `c.Repo == canonical` AND `c.Repo != link` (not the raw symlink).
- `l.Release()`.

---

## 3. Composition with Issue 2 (P1.M2.T1.S1 — LANDED)

The current `Release()` already does close → `os.Remove(path)` (best-effort). So:
- The lock file exists between `Acquire` and `Release`; it is REMOVED on `Release`.
- ANY test that reads the lock file contents (the new symlink test, `TestAcquireRelease_RoundTrip`,
  `TestSetSnapshot_UpdatesFile`) must read BEFORE `Release`. All three already do (RoundTrip reads then
  Release×2; SetSnapshot reads then deferred Release; the new test reads then Release). ✓
- The new test MUST NOT defer the read until after Release (the file would be gone → `os.ReadFile` error).

---

## 4. default_action.go is UNCHANGED (the message auto-fixes)

`handleLockContention` (default_action.go:254) formats `heldErr.Contents.Repo` directly into the Busy
message with no transformation. Once `Acquire` stores the canonical path in the `repo=` field, the
contention message automatically prints the canonical path. **No edit to default_action.go or any caller.**
This is a pure lock-package-internal change.

---

## 5. Scope fences (NOT this task)

- **NOT Issue 2** (P1.M2.T1.S1 — Release removes file) — already landed; don't touch `Release()`.
- **NOT Issue 4a** (P1.M2.T3.S1 — atomic `setSnapshot` Seek→Write→Truncate) — different function.
- **NOT Issue 4b** (P1.M2.T4.S1 — guard `handleLockContention` empty fields) — different function/file.
- **NOT the contention message formatting** — `handleLockContention` prints `Contents.Repo` unchanged.
- **NOT `lockPath`'s signature** — only its body (`_, hash :=`). `lockPath` stays `(string, error)`.
- **NOT platform files** (`lock_unix.go`/`lock_windows.go`) — the fix is in shared `lock.go`.
- **NOT docs** (DOCS: none — diagnostic-only; P1.M3 owns the doc sweep).
- **NOT callers** of `Acquire` (default_action passes `os.Getwd()`; the repo field is diagnostic-only).

---

## 6. Validation commands

```bash
gofmt -w internal/lock/lock.go internal/lock/lock_test.go
go vet ./internal/lock/        # catches a malformed lockHash return / unused canonical.
go build ./...
go test -race ./internal/lock/ -v   # the new TestAcquire_RepoFieldIsCanonical + updated RoundTrip/SetSnapshot/Hash + all others.
go test -race ./...                 # no regressions (no caller changed; default_action.go untouched).
git diff --exit-code go.mod go.sum  # unchanged (no new dep; "path/filepath" already imported).
# Confirm lockHash is the sole canonicalization site (no stray EvalSymlinks added elsewhere):
grep -n 'EvalSymlinks\|filepath.Abs' internal/lock/lock.go   # only inside lockHash.
```
