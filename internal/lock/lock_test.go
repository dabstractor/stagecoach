package lock

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// resetCurrent stores nil in the package singleton when the test finishes.
// Prevents singleton poisoning between tests (especially under -race).
func resetCurrent(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { current.Store(nil) })
}

// TestLockDir_RuntimePreferred verifies XDG_RUNTIME_DIR takes precedence.
func TestLockDir_RuntimePreferred(t *testing.T) {
	tmpAbs := filepath.Join(t.TempDir(), "runtime")
	t.Setenv("XDG_RUNTIME_DIR", tmpAbs)
	t.Setenv("XDG_CACHE_HOME", "") // clear so it doesn't interfere

	dir, err := lockDir()
	if err != nil {
		t.Fatalf("lockDir: %v", err)
	}
	want := filepath.Join(tmpAbs, "stagecoach", "locks")
	if dir != want {
		t.Errorf("lockDir = %q, want %q", dir, want)
	}
}

// TestLockDir_CacheFallback verifies XDG_CACHE_HOME is used when XDG_RUNTIME_DIR is unset.
func TestLockDir_CacheFallback(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	tmpAbs := filepath.Join(t.TempDir(), "cache")
	t.Setenv("XDG_CACHE_HOME", tmpAbs)

	dir, err := lockDir()
	if err != nil {
		t.Fatalf("lockDir: %v", err)
	}
	want := filepath.Join(tmpAbs, "stagecoach", "locks")
	if dir != want {
		t.Errorf("lockDir = %q, want %q", dir, want)
	}
}

// TestLockDir_HomeFallback verifies ~/.cache/stagecoach/locks when both XDG vars are unset.
func TestLockDir_HomeFallback(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("XDG_CACHE_HOME", "")
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	dir, err := lockDir()
	if err != nil {
		t.Fatalf("lockDir: %v", err)
	}
	want := filepath.Join(tmpHome, ".cache", "stagecoach", "locks")
	if dir != want {
		t.Errorf("lockDir = %q, want %q", dir, want)
	}
}

// TestLockDir_RejectedRelative verifies a relative XDG_RUNTIME_DIR is skipped.
func TestLockDir_RejectedRelative(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", "rel/path")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", tmpHome)

	dir, err := lockDir()
	if err != nil {
		t.Fatalf("lockDir: %v", err)
	}
	// Should fall through to home fallback, NOT use the relative path.
	want := filepath.Join(tmpHome, ".cache", "stagecoach", "locks")
	if dir != want {
		t.Errorf("lockDir = %q, want %q (relative XDG_RUNTIME_DIR should be skipped)", dir, want)
	}
}

// TestLockDir_NoCwdFallbackError verifies lockDir returns an error when no XDG
// vars are set and UserHomeDir fails (NO CWD fallback).
func TestLockDir_NoCwdFallbackError(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	// On most systems, unsetting HOME makes os.UserHomeDir() fail.
	_, err := lockDir()
	if err == nil {
		t.Error("lockDir should return an error when no resolution path exists")
	}
}

// TestHash_CanonicalSymlink verifies two paths to the same repo (one a symlink)
// produce the same lock hash.
func TestHash_CanonicalSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	tmpRepo := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(tmpRepo, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	tmpLink := filepath.Join(tmpDir, "link")
	if err := os.Symlink(tmpRepo, tmpLink); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	_, hash1 := lockHash(tmpRepo)
	_, hash2 := lockHash(tmpLink)
	if hash1 != hash2 {
		t.Errorf("lockHash(symlink)=%q != lockHash(real)=%q", hash2, hash1)
	}
	// Determinism: same path → same hash always.
	_, hash3 := lockHash(tmpRepo)
	if hash1 != hash3 {
		t.Errorf("lockHash not deterministic: %q != %q", hash1, hash3)
	}
}

// TestAcquire_PathMatchesLockPath verifies the path Acquire creates equals
// lockPath(repo) — the two must agree since Acquire resolves its path via
// lockPath. A regression here would mean Acquire and lockPath drift apart,
// breaking the no-op fast path (which keys off the same path on re-run).
func TestAcquire_PathMatchesLockPath(t *testing.T) {
	resetCurrent(t)
	repo := t.TempDir()

	l, err := Acquire(repo)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer l.Release()

	want, err := lockPath(repo)
	if err != nil {
		t.Fatalf("lockPath: %v", err)
	}
	if l.path != want {
		t.Errorf("Acquire path = %q, want lockPath = %q", l.path, want)
	}
}

// TestLockPath_CanonicalSymlink verifies lockPath keys off the canonical path,
// so a symlinked repo and its real target resolve to the same lock file.
func TestLockPath_CanonicalSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	tmpRepo := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(tmpRepo, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	tmpLink := filepath.Join(tmpDir, "link")
	if err := os.Symlink(tmpRepo, tmpLink); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	p1, err := lockPath(tmpRepo)
	if err != nil {
		t.Fatalf("lockPath(real): %v", err)
	}
	p2, err := lockPath(tmpLink)
	if err != nil {
		t.Fatalf("lockPath(symlink): %v", err)
	}
	if p1 != p2 {
		t.Errorf("lockPath(symlink)=%q != lockPath(real)=%q", p2, p1)
	}
}

// TestAcquireRelease_RoundTrip verifies Acquire creates the lock file with
// correct contents and Release is idempotent.
func TestAcquireRelease_RoundTrip(t *testing.T) {
	resetCurrent(t)
	repo := t.TempDir()

	l, err := Acquire(repo)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// File exists with correct contents.
	data, err := os.ReadFile(l.path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	c := parseContents(data)
	if c.Pid == "" {
		t.Error("pid is empty")
	}
	if c.Hostname == "" {
		t.Error("hostname is empty")
	}
	// Issue 3: repo= stores the canonical path (EvalSymlinks-resolved). On macOS
	// t.TempDir() is under /var → /private/var, so c.Repo != raw repo; compare
	// against the same canonical oracle lockHash uses.
	canonical, _ := lockHash(repo)
	if c.Repo != canonical {
		t.Errorf("repo = %q, want canonical %q", c.Repo, canonical)
	}
	if c.Timestamp == "" {
		t.Error("timestamp is empty")
	}
	if c.Snapshot != "" {
		t.Errorf("snapshot = %q, want empty", c.Snapshot)
	}

	// Release is idempotent.
	l.Release()
	l.Release() // second call must not panic
}

// TestAcquire_RepoFieldIsCanonical verifies Issue 3's diagnostic fix: when a
// repo is reached via a symlink, the repo= field stores the CANONICAL real path
// (the same path the lock filename hash uses), not the raw symlink. This is the
// headline deliverable — it makes the contention message unambiguous for two
// terminals in the same repo (one via the symlink, one via the real path).
func TestAcquire_RepoFieldIsCanonical(t *testing.T) {
	resetCurrent(t)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // isolate — don't touch the real lock dir
	t.Setenv("XDG_CACHE_HOME", "")

	tmpDir := t.TempDir()
	realRepo := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(realRepo, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	link := filepath.Join(tmpDir, "link")
	if err := os.Symlink(realRepo, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	canonical, _ := lockHash(link) // the expected canonical (EvalSymlinks(link) == realRepo)

	l, err := Acquire(link) // acquire via the SYMLINK (raw path)
	if err != nil {
		t.Fatalf("Acquire via symlink: %v", err)
	}
	// Read the lock file BEFORE Release — Issue 2 (P1.M2.T1.S1) removes the file on Release.
	data, err := os.ReadFile(l.path)
	if err != nil {
		l.Release()
		t.Fatalf("ReadFile before Release: %v", err)
	}
	l.Release()

	c := parseContents(data)
	if c.Repo != canonical {
		t.Errorf("repo field = %q, want canonical %q (Issue 3: repo= must be canonical, not the raw symlink %q)",
			c.Repo, canonical, link)
	}
	if c.Repo == link {
		t.Errorf("repo field is the raw symlink path %q — Issue 3 not fixed", link)
	}
}

// TestRelease_RemovesLockFile verifies Issue 2's disk-hygiene fix: Release removes
// the lock file after closing the fd (close-then-remove), the removal is
// idempotent (a second Release is a no-op), and a fresh Acquire recreates the
// file (OpenFile O_CREATE) so removal never breaks re-acquisition.
func TestRelease_RemovesLockFile(t *testing.T) {
	resetCurrent(t)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // isolate — don't touch the real lock dir
	t.Setenv("XDG_CACHE_HOME", "")
	repo := t.TempDir()

	l, err := Acquire(repo)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	path := l.path

	// Sanity: the lock file exists while held.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock file missing immediately after Acquire: %v", err)
	}

	l.Release()

	// Issue 2 fix: Release removes the lock file.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("after Release, os.Stat(lock) err = %v, want os.IsNotExist (file should be removed)", err)
	}

	// Idempotency: a second Release is a no-op (no panic on the now-absent file).
	l.Release()

	// Re-acquisition recreates the file (OpenFile O_CREATE) — removal must not break re-Acquire.
	l2, err := Acquire(repo)
	if err != nil {
		t.Fatalf("re-Acquire after Release: %v", err)
	}
	if _, err := os.Stat(l2.path); err != nil {
		t.Errorf("lock file missing after re-Acquire: %v", err)
	}
	l2.Release()
}

// TestSetSnapshot_UpdatesFile verifies SetSnapshot rewrites the snapshot= line.
func TestSetSnapshot_UpdatesFile(t *testing.T) {
	resetCurrent(t)
	repo := t.TempDir()

	l, err := Acquire(repo)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer l.Release()

	SetSnapshot("abc123")

	data, err := os.ReadFile(l.path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	c := parseContents(data)
	if c.Snapshot != "abc123" {
		t.Errorf("snapshot = %q, want %q", c.Snapshot, "abc123")
	}
	if c.Pid == "" {
		t.Error("pid was cleared by SetSnapshot")
	}
	// Issue 3: repo= is the canonical path; compare against the canonical oracle
	// (macOS-symlink-tmpdir-safe).
	canonical, _ := lockHash(repo)
	if c.Repo != canonical {
		t.Errorf("repo changed: %q, want canonical %q", c.Repo, canonical)
	}
}

// TestSetSnapshot_FileNeverEmptyWellFormed verifies Issue 4a's write-ordering
// invariant: after every setSnapshot (Seek→Write→Truncate→Sync, Write-before-
// Truncate), the lock file is NEVER empty and is well-formed (all 5 key=value
// lines; the 4 diagnostic fields non-empty). A contender's os.ReadFile in
// Acquire's EWOULDBLOCK branch therefore never observes an empty/partial-
// diagnostic file. Also verifies the SHRINK case (long snapshot → short
// snapshot): Truncate must cut the stale trailing bytes so no garbage remains —
// proven by a raw-bytes suffix check (parseContents alone CANNOT catch a
// missing Truncate, since it silently skips the trailing malformed line).
//
// (The nil/released no-op is already covered by TestSetSnapshot_NilSafeNoOp +
// TestSetSnapshot_MethodAfterRelease, which this change leaves unchanged. A
// deterministic cross-process race test is not feasible — the window is
// microsecond-wide and contention is nondeterministic; this invariant test is
// the contract-specified proxy.)
func TestSetSnapshot_FileNeverEmptyWellFormed(t *testing.T) {
	resetCurrent(t)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // isolate — don't touch the real lock dir
	t.Setenv("XDG_CACHE_HOME", "")
	repo := t.TempDir()

	l, err := Acquire(repo)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer l.Release() // reads happen in the body before this runs (Issue 2 removes the file on Release)

	// assertWellFormed reads the file immediately and checks the Issue-4 invariant:
	// non-empty + all 4 diagnostic fields present + the expected snapshot. The
	// "never empty immediately after setSnapshot" check is the len(data)==0 assertion.
	assertWellFormed := func(t *testing.T, wantSnapshot string) {
		t.Helper()
		data, err := os.ReadFile(l.path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if len(data) == 0 {
			t.Fatal("lock file is EMPTY after writeContents (Issue 4 invariant violated — write-before-truncate broken)")
		}
		c := parseContents(data)
		if c.Pid == "" || c.Hostname == "" || c.Repo == "" || c.Timestamp == "" {
			t.Errorf("empty diagnostic field after rewrite: pid=%q hostname=%q repo=%q timestamp=%q",
				c.Pid, c.Hostname, c.Repo, c.Timestamp)
		}
		if c.Snapshot != wantSnapshot {
			t.Errorf("snapshot = %q, want %q", c.Snapshot, wantSnapshot)
		}
	}

	// (a) The initial write (Acquire → writeContents("")) is well-formed.
	assertWellFormed(t, "")

	// (b) A snapshot update keeps the file non-empty + well-formed.
	SetSnapshot("abc123def456")
	assertWellFormed(t, "abc123def456")

	// (c) SHRINK case: a LONG snapshot followed by a SHORT one. Truncate must cut the stale tail.
	SetSnapshot("lorem-ipsum-dolor-sit-amet-XXXX-YYYY") // 36-char snapshot
	assertWellFormed(t, "lorem-ipsum-dolor-sit-amet-XXXX-YYYY")
	SetSnapshot("short") // shorter than the previous → trailing bytes would remain WITHOUT Truncate
	assertWellFormed(t, "short")

	// The raw-bytes suffix check is the meaningful Truncate proof: if Truncate
	// didn't run, the file tail would be the leftover "...XXXX-YYYY\n" instead of
	// "snapshot=short\n". (parseContents would still report snapshot="short" — it
	// skips the malformed trailing line — so this raw check is required.)
	data, err := os.ReadFile(l.path)
	if err != nil {
		t.Fatalf("ReadFile (shrink): %v", err)
	}
	if !strings.HasSuffix(string(data), "snapshot=short\n") {
		tail := string(data)
		if len(tail) > 80 {
			tail = tail[len(tail)-80:]
		}
		t.Errorf("Truncate did not cut stale trailing bytes (Issue 4): file tail = %q, want suffix %q",
			tail, "snapshot=short\n")
	}
}

// TestSetSnapshot_NilSafeNoOp verifies the package-level SetSnapshot is a
// no-op when no lock is held.
func TestSetSnapshot_NilSafeNoOp(t *testing.T) {
	resetCurrent(t)
	current.Store(nil)
	// Must not panic.
	SetSnapshot("should-be-noop")
}

// TestSetSnapshot_MethodAfterRelease verifies SetSnapshot is a no-op after Release.
func TestSetSnapshot_MethodAfterRelease(t *testing.T) {
	resetCurrent(t)
	repo := t.TempDir()

	l, err := Acquire(repo)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	l.Release()
	// Must not panic.
	l.SetSnapshot("after-release-noop")
}

// TestAcquire_Contention_HeldError verifies that a second Acquire on the same
// repo returns *HeldError with the holder's parsed contents, and that after
// Release a third Acquire succeeds (auto-release on close).
func TestAcquire_Contention_HeldError(t *testing.T) {
	resetCurrent(t)
	repo := t.TempDir()

	l1, err := Acquire(repo)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}

	var l2 *Locker
	var l2err error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		l2, l2err = Acquire(repo)
	}()
	wg.Wait()

	if l2 != nil {
		t.Error("second Acquire should return nil Locker on contention")
		l2.Release()
	}
	if l2err == nil {
		t.Fatal("second Acquire should return an error on contention")
	}

	var he *HeldError
	if !errors.As(l2err, &he) {
		t.Fatalf("second Acquire error type = %T, want *HeldError", l2err)
	}
	if he.Contents.Pid != l1.pid {
		t.Errorf("HeldError.Pid = %q, want %q", he.Contents.Pid, l1.pid)
	}
	if he.Path != l1.path {
		t.Errorf("HeldError.Path = %q, want %q", he.Path, l1.path)
	}

	// Release and re-acquire should succeed.
	l1.Release()
	l3, err := Acquire(repo)
	if err != nil {
		t.Fatalf("third Acquire after Release: %v", err)
	}
	l3.Release()
}

// TestIsHeldError verifies the IsHeldError helper.
func TestIsHeldError(t *testing.T) {
	if IsHeldError(nil) {
		t.Error("IsHeldError(nil) = true, want false")
	}
	he := &HeldError{Contents: LockContents{Pid: "42"}, Path: "/tmp/x.lock"}
	if !IsHeldError(he) {
		t.Error("IsHeldError(*HeldError) = false, want true")
	}
}
