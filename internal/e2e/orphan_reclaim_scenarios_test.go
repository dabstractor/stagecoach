//go:build e2e && !windows

// orphan_reclaim_scenarios_test.go is the PRD §20.5 end-to-end regression net for the LANDED §9.27
// orphaned-run lock reclamation machinery: the parent-death watchdog (FR-K1), SIGHUP-on-terminal-close
// rescue (FR-K3), the read-only `stagecoach lock status` diagnostic (FR-K4), and the
// STAGECOACH_NO_PARENT_WATCHDOG opt-out (FR-K6). It exercises these against REAL stagecoach
// subprocesses on REAL temp git repos — the cross-process flock, the kernel signal delivery, and the
// real prctl/getppid reparenting that the in-process library tests (§20.1 layers 1–3) cannot reach.
//
// Unix-only (the !windows build tag): SIGHUP, init/subreaper reparenting, and the parent-death
// watchdog are Unix concepts (Windows is a documented no-op, FR-K7, covered by per-OS unit tests).
// The file-local helpers use syscall.Kill/syscall.SIGHUP (Unix-only symbols).
//
// CONSUMES — adds NO production code. Reuses the shared harness primitives (buildStagecoach/buildStub/
// newRepo/runStagecoach/waitForMarker/writeStubConfig/stubEnv/headSHA/commitCount/statusPorcelain/
// seedCommit/writeFile/stageFile/runGit) + the stub's STAGECOACH_STUB_MARKER + STAGECOACH_STUB_SLEEP_MS
// blocking pattern. Adds ONE file-local process-management + lock-path helper layer so the shared
// harness_test.go stays frozen (blast radius = this ONE new file).
//
// Determinism keys (see findings.md):
//   - generate.go arms the snapshot (signal.SetSnapshot, L242) BEFORE provider.Execute (L335) runs the
//     stub. The stub writes its marker AFTER stdin drain (post-Execute-start). So once waitForMarker
//     returns, snapshot != "" → SIGHUP and parent-death BOTH route through handle()'s POST-snapshot
//     branch → exit 3 + OnRescueExit (lock release). NEVER assert 129/143 for a signal sent AFTER
//     waitForMarker (those are the PRE-snapshot codes, unreachable at marker time).
//   - The parent MUST survive past watchdog.Arm() (the documented prctl/getppid race): the scenario-(a)
//     wrapper backgrounds stagecoach, writes its PID, `sleep`s ~1.5s, THEN exits → stagecoach reparented.
//   - A reparented process can't be Wait'd by the test (it is no longer the test's child) → poll
//     syscall.Kill(pid, 0)==ESRCH (waitProcessGone) and assert OUTCOMES (lock removed / HEAD unchanged),
//     NOT the exit code.
package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// e2eCmd wraps a started stagecoach subprocess so waitForExit can read the captured buffers + map
// the exit code without the caller closing over them. It mirrors runStagecoach's e2eResult shape.
// File-local — NOT promoted to harness_test.go (scope guard: one new file).
type e2eCmd struct {
	*exec.Cmd
	stdout  bytes.Buffer
	stderr  bytes.Buffer
	cancel  context.CancelFunc
	started bool
}

// startStagecoach is runStagecoach MINUS the Run: it builds the exec.CommandContext (60s), sets
// cmd.Dir=repo, cmd.Env=env, wires stdout/stderr capture, calls Start(), and returns an *e2eCmd
// (cmd.Process.Pid ready). The caller drives it to completion via waitForExit (test is the parent,
// exit code readable — scenario b) or, for a reparented process, waitProcessGone (scenario a/d).
// t.Cleanup ensures the process + ctx never leak if the test aborts mid-run.
func startStagecoach(t *testing.T, bin, repo, cfg string, env []string, args ...string) *e2eCmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	allArgs := append([]string{"--config", cfg, "--no-color"}, args...)
	cmd := exec.CommandContext(ctx, bin, allArgs...)
	cmd.Dir = repo
	cmd.Env = env
	ec := &e2eCmd{Cmd: cmd, cancel: cancel}
	cmd.Stdout = &ec.stdout
	cmd.Stderr = &ec.stderr
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start stagecoach: %v", err)
	}
	ec.started = true
	t.Cleanup(func() {
		cancel()
		if ec.started {
			_ = cmd.Wait() // best-effort reap so a leaked process doesn't zombify the test run
		}
	})
	return ec
}

// waitForExit waits for cmd (with timeout) and returns e2eResult{Stdout,Stderr,ExitCode}. Maps
// (*exec.ExitError).ExitCode(); a context-deadline/other error → t.Fatalf (a hang is a test
// failure). Cancels the 60s ctx on the happy path so the test process doesn't leak a live ctx.
func waitForExit(t *testing.T, ec *e2eCmd, timeout time.Duration) e2eResult {
	t.Helper()
	waitCh := make(chan error, 1)
	go func() { waitCh <- ec.Cmd.Wait() }()
	select {
	case err := <-waitCh:
		ec.started = false
		ec.cancel() // release the 60s ctx now that Wait returned
		r := e2eResult{Stdout: ec.stdout.String(), Stderr: ec.stderr.String()}
		if err == nil {
			return r // exit 0
		}
		if ee := (*exec.ExitError)(nil); errors.As(err, &ee) {
			r.ExitCode = ee.ExitCode()
			return r
		}
		t.Fatalf("wait stagecoach: %v", err)
		return r
	case <-time.After(timeout):
		ec.cancel() // nudge the 60s ctx (KillProcessGroup via ctx-cancel) so Wait can return
		t.Fatalf("waitForExit: stagecoach did not exit after %v (stdout=%q stderr=%q)", timeout, ec.stdout.String(), ec.stderr.String())
		return e2eResult{}
	}
}

// waitProcessGone polls syscall.Kill(pid, 0) for ESRCH (process exited) up to timeout. For a
// reparented process (scenario a/d) the test is no longer its parent, so cmd.Wait() can't reap it
// and the exit code is unreadable — this is the exit detector. Fatalf on timeout.
func waitProcessGone(t *testing.T, pid int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// Kill(pid, 0) sends no signal; it only checks the process exists.
		// ESRCH ⇒ process gone; nil/EPERM ⇒ still alive (or alive as another user).
		if err := syscall.Kill(pid, 0); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return // process exited
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("waitProcessGone: pid %d still alive after %v", pid, timeout)
}

// lockFilePath replicates lock.lockPath under an isolated XDG_RUNTIME_DIR: EvalSymlinks(repo)
// (→Abs on error), sha256, hex, join(XDG_RUNTIME_DIR, "stagecoach", "locks", hash+".lock").
// The caller MUST t.Setenv("XDG_RUNTIME_DIR", tmpDir) + t.Setenv("XDG_CACHE_HOME", "") BEFORE
// stubEnv so the stagecoach subprocess agrees on the same lock dir (stubEnv = os.Environ() + knobs).
// Mirrors internal/lock/lock.go lockPath + lockHash exactly.
func lockFilePath(repo string) string {
	canonical, err := filepath.EvalSymlinks(repo)
	if err != nil {
		canonical, _ = filepath.Abs(repo)
	}
	sum := sha256.Sum256([]byte(canonical))
	return filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "stagecoach", "locks", hex.EncodeToString(sum[:])+".lock")
}

// plantLockFile writes a *.lock with known pid/hostname/repo in the exact key=value format
// parseContents reads (mirrors internal/lock/lock_unix_test.go writeLockFile + parseContents keys).
// MkdirAll the dir (0o700); WriteFile 0o600. Used by scenario (c) Dead + Orphan. hostname MUST be
// THIS host (else processAlive short-circuits to true for a foreign host).
func plantLockFile(t *testing.T, path, pid, hostname, repo string) {
	t.Helper()
	content := fmt.Sprintf("pid=%s\nhostname=%s\nrepo=%s\ntimestamp=2026-07-10T00:00:00Z\nsnapshot=\n",
		pid, hostname, repo)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("plantLockFile MkdirAll %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("plantLockFile WriteFile %s: %v", path, err)
	}
}

// ppidOf returns the parent pid of pid (Linux /proc/<pid>/status PPid:; else `ps -o ppid= -p <pid>`).
// Used by scenario (c) Orphan to verify ppid==1 (skip if subreaper-reparented, ppid≠1, so the test
// never flakes under systemd/containers). Mirrors internal/lock/orphan_unix.go ppidOf exactly.
func ppidOf(pid int) (int, error) {
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
		if err != nil {
			return 0, err
		}
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "PPid:" {
				return strconv.Atoi(fields[1])
			}
		}
		return 0, fmt.Errorf("ppidOf: no PPid field for pid %d", pid)
	}
	out, err := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

// shQuote wraps a path in POSIX single quotes for embedding in a `sh -c` script. t.TempDir paths
// are usually space-free but quoting is cheap insurance against spaces / metacharacters.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// readPidFile reads + trims + parses the pidfile the scenario-(a)/(d) wrapper writes.
func readPidFile(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pidfile %s: %v", path, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("parse pidfile %s (%q): %v", path, strings.TrimSpace(string(data)), err)
	}
	return pid
}

// isolateLocks sets XDG_RUNTIME_DIR/XDG_CACHE_HOME so the stagecoach subprocess's lock dir is
// isolated per test (and lockFilePath computes the SAME path). MUST be called BEFORE stubEnv.
func isolateLocks(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", "")
}

// assertLockRemoved fails the test if the lock file still exists at path.
func assertLockRemoved(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("lock file %s still present, want removed (rescue OnRescueExit=lock.ReleaseCurrent)", path)
	}
}

// TestE2EOrphanReclaim exercises the §9.27 reclamation machinery end-to-end: the parent-death
// watchdog self-exit (FR-K1), the SIGHUP rescue (FR-K3), and the no_parent_watchdog opt-out (FR-K6).
// (The `lock status` diagnostic, FR-K4, is TestE2ELockStatus — a separate top-level test.)
func TestE2EOrphanReclaim(t *testing.T) {
	bin := buildStagecoach(t)
	stub := buildStub(t)
	cfg := writeStubConfig(t, stub, "")

	// B_SIGHUPRescue is FIRST: cleanest, fully deterministic, and validates the rescue-exit-3 +
	// lock-removed invariants the other scenarios lean on (FR-K3).
	t.Run("B_SIGHUPRescue", func(t *testing.T) {
		// FR-K3: a terminal hangup (closing the lazygit TUI, quitting an IDE) delivers SIGHUP to the
		// process group. Our handler catches SIGHUP (signal_unix.go) → handle() → snapshot armed →
		// rescue exit 3 + OnRescueExit (lock release). Assert exit 3 (NOT 129 — 129 is the PRE-snapshot
		// code, unreachable once the marker exists because generate.go arms the snapshot at L242<L335).
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "a.txt", "a\n")
		stageFile(t, repo, "a.txt")

		isolateLocks(t) // MUST precede stubEnv so XDG flows to the subprocess
		lockPath := lockFilePath(repo)
		marker := t.TempDir() + "/ready.marker"
		env := stubEnv(map[string]string{
			"STAGECOACH_STUB_OUT":      "feat: a",
			"STAGECOACH_STUB_MARKER":   marker,
			"STAGECOACH_STUB_SLEEP_MS": "3000", // hold generation mid-flight so SIGHUP lands during the sleep
		})

		seedHead := headSHA(t, repo) // capture BEFORE generation
		ec := startStagecoach(t, bin, repo, cfg, env, "--provider", "stub")
		waitForMarker(t, marker, 10*time.Second) // snapshot armed (generate.go:242), stub blocked in sleep

		if err := syscall.Kill(ec.Process.Pid, syscall.SIGHUP); err != nil {
			t.Fatalf("SIGHUP stagecoach (pid %d): %v", ec.Process.Pid, err)
		}

		res := waitForExit(t, ec, 10*time.Second)
		if res.ExitCode != 3 {
			t.Fatalf("SIGHUP exit = %d, want 3 (rescue — snapshot armed at marker time); stderr:\n%s", res.ExitCode, res.Stderr)
		}
		assertLockRemoved(t, lockPath)
		if got := headSHA(t, repo); got != seedHead {
			t.Errorf("HEAD advanced after SIGHUP rescue = %s, want unchanged %s (no commit lands)", got, seedHead)
		}
		if got := statusPorcelain(t, repo); !strings.Contains(got, "a.txt") {
			t.Errorf("index changed after SIGHUP rescue; status --porcelain = %q, want a.txt still staged", got)
		}
	})

	// A_ParentDeathWatchdog: the hard one. A parent shell backgrounds stagecoach, sleeps ~1.5s (past
	// watchdog.Arm so originalPpid is captured as sh's pid, NOT init), then exits → stagecoach is
	// reparented → the watchdog's getppid poll (≤1s) detects the change → signal.Trigger(SIGTERM) →
	// rescue exit 3 + lock release. The test CAN'T Wait() a reparented process → poll ESRCH + assert
	// OUTCOMES (lock removed, HEAD/index unchanged), NOT the exit code (FR-K1).
	t.Run("A_ParentDeathWatchdog", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "a.txt", "a\n")
		stageFile(t, repo, "a.txt")

		isolateLocks(t)
		lockPath := lockFilePath(repo)
		marker := t.TempDir() + "/ready.marker"
		pidfile := t.TempDir() + "/child.pid"
		// STUB_SLEEP_MS=5000 keeps the stub in generation LONG past the watchdog's 1s poll so the
		// watchdog fires DURING the sleep (the stub is then SIGTERM'd via KillProcessGroup and
		// stagecoach exits 3 before commit).
		env := stubEnv(map[string]string{
			"STAGECOACH_STUB_OUT":      "feat: a",
			"STAGECOACH_STUB_MARKER":   marker,
			"STAGECOACH_STUB_SLEEP_MS": "5000",
		})
		seedHead := headSHA(t, repo)

		// The race-defeating wrapper: background stagecoach (child of sh), write its PID, sleep ~1.5s
		// (stay alive past watchdog.Arm), then sh exits → stagecoach orphaned → reparented to init/subreaper.
		// Non-interactive `sh -c 'cmd &'` does NOT SIGHUP the backgrounded job on exit (huponexit is an
		// interactive-shell feature), so stagecoach is cleanly orphaned, not SIGHUP'd.
		script := fmt.Sprintf("%s --config %s --provider stub &\necho $! > %s\nsleep 1.5\n",
			shQuote(bin), shQuote(cfg), shQuote(pidfile))
		wrapper := exec.Command("sh", "-c", script)
		wrapper.Env = env
		wrapper.Dir = repo
		if err := wrapper.Start(); err != nil {
			t.Fatalf("start wrapper sh: %v", err)
		}
		if err := wrapper.Wait(); err != nil {
			t.Fatalf("wrapper sh exited non-zero: %v", err)
		}

		pid := readPidFile(t, pidfile)
		// Best-effort sync: the marker proves the stub reached generation (snapshot armed). If the
		// wrapper raced stagecoach's start (marker missing), the test will surface it as a failure.
		waitForMarker(t, marker, 10*time.Second)

		// Can't Wait() a reparented process (init/subreaper reaps it) → poll ESRCH.
		waitProcessGone(t, pid, 10*time.Second)

		assertLockRemoved(t, lockPath)
		if got := headSHA(t, repo); got != seedHead {
			t.Errorf("HEAD advanced after parent-death rescue = %s, want unchanged %s (no commit lands)", got, seedHead)
		}
		if got := statusPorcelain(t, repo); !strings.Contains(got, "a.txt") {
			t.Errorf("index changed after parent-death rescue; status --porcelain = %q, want a.txt still staged", got)
		}
	})

	// D_NoParentWatchdogOptOut: scenario (a)'s wrapper re-run with STAGECOACH_NO_PARENT_WATCHDOG=1 →
	// the arming gate (default_action.go:108 !cfg.NoParentWatchdog) skips watchdog.Arm → no prctl, no
	// poll goroutine → reparenting is undetected → stagecoach runs the stub to completion and COMMITS.
	// HEAD advancement (NOT lock-removed — the clean deferred Release also removes it) is the proof.
	t.Run("D_NoParentWatchdogOptOut", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "a.txt", "a\n")
		stageFile(t, repo, "a.txt")

		isolateLocks(t)
		pidfile := t.TempDir() + "/child.pid"
		env := stubEnv(map[string]string{
			"STAGECOACH_STUB_OUT":           "feat: a",
			"STAGECOACH_STUB_SLEEP_MS":      "3000",
			"STAGECOACH_NO_PARENT_WATCHDOG": "1", // FR-K6 opt-out — Arm() is NEVER called
		})

		script := fmt.Sprintf("%s --config %s --provider stub &\necho $! > %s\nsleep 1.5\n",
			shQuote(bin), shQuote(cfg), shQuote(pidfile))
		wrapper := exec.Command("sh", "-c", script)
		wrapper.Env = env
		wrapper.Dir = repo
		if err := wrapper.Start(); err != nil {
			t.Fatalf("start wrapper sh: %v", err)
		}
		if err := wrapper.Wait(); err != nil {
			t.Fatalf("wrapper sh exited non-zero: %v", err)
		}

		pid := readPidFile(t, pidfile)
		// The watchdog is OFF → stagecoach runs the full 3s stub, commits, then exits. Give it a
		// GENEROUS timeout — the process must NOT be gone at the ~2-3s watchdog window (if it were,
		// opt-out failed). The OUTCOME assertion (commitCount==2) is the proof; do not rely on timing.
		waitProcessGone(t, pid, 15*time.Second)

		if n := commitCount(t, repo); n != 2 {
			t.Errorf("commit count = %d, want 2 (seed + the stub's commit LANDED — opt-out suppressed the watchdog); stderr n/a (subprocess)", n)
		}
		if msg := runGit(t, repo, "log", "-1", "--format=%s"); msg != "feat: a" {
			t.Errorf("HEAD subject = %q, want 'feat: a' (the stub's commit)", msg)
		}
	})
}

// TestE2ELockStatus exercises the `stagecoach lock status` diagnostic (FR-K4) for the four holder
// states: NoLock (no lock held → "no run lock for <repo>"), Live (a real sleeping stub holder →
// alive:true + pid/hostname/Lock present), Dead (a planted dead-pid lock → alive:false +
// "orphaned:  unknown (holder is dead)"), and Orphan (best-effort, skip-guarded — a genuine ppid==1
// holder → alive:true + orphaned:true, t.Skip'd when the env reparents to a subreaper so it never flakes).
func TestE2ELockStatus(t *testing.T) {
	bin := buildStagecoach(t)
	stub := buildStub(t)
	cfg := writeStubConfig(t, stub, "")

	// NoLock: with no lock file present, `lock status` prints "no run lock for <repo>" + exits 0.
	t.Run("NoLock", func(t *testing.T) {
		repo := newRepo(t)
		isolateLocks(t) // also isolate the read path (lockPath is XDG-derived)
		env := stubEnv(nil)

		res := runStagecoach(t, bin, repo, cfg, env, "lock", "status")
		if res.ExitCode != 0 {
			t.Fatalf("lock status (no lock) exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
		}
		if !strings.Contains(res.Stdout, "no run lock for") {
			t.Errorf("stdout missing 'no run lock for'; got:\n%s", res.Stdout)
		}
		if !strings.Contains(res.Stdout, repo) {
			t.Errorf("stdout missing repo path %q; got:\n%s", repo, res.Stdout)
		}
	})

	// Live: spawn a real sleeping-stub holder (mirror lock_scenarios_test.go's goroutine+marker+drain
	// idiom), then `lock status` must report alive:true + the holder's pid/hostname/Lock path.
	t.Run("Live", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "a.txt", "a\n")
		stageFile(t, repo, "a.txt")

		isolateLocks(t)
		marker := t.TempDir() + "/ready.marker"
		holderEnv := stubEnv(map[string]string{
			"STAGECOACH_STUB_OUT":      "feat: a",
			"STAGECOACH_STUB_MARKER":   marker,
			"STAGECOACH_STUB_SLEEP_MS": "5000", // hold the lock while we read status
		})

		resCh := make(chan e2eResult, 1)
		go func() { resCh <- runStagecoach(t, bin, repo, cfg, holderEnv, "--provider", "stub") }()
		waitForMarker(t, marker, 10*time.Second) // #1 holds the lock

		baseEnv := stubEnv(nil) // stubEnv re-reads os.Environ() (XDG isolation already set)
		res := runStagecoach(t, bin, repo, cfg, baseEnv, "lock", "status")
		if res.ExitCode != 0 {
			t.Fatalf("lock status (live) exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
		}
		if !strings.Contains(res.Stdout, "Lock:") {
			t.Errorf("stdout missing 'Lock:'; got:\n%s", res.Stdout)
		}
		if !strings.Contains(res.Stdout, "alive:     true") {
			t.Errorf("stdout missing 'alive:     true'; got:\n%s", res.Stdout)
		}
		if !strings.Contains(res.Stdout, "pid:") {
			t.Errorf("stdout missing 'pid:'; got:\n%s", res.Stdout)
		}
		if !strings.Contains(res.Stdout, "hostname:") {
			t.Errorf("stdout missing 'hostname:'; got:\n%s", res.Stdout)
		}
		// The orphaned field is PRESENT under any holder (true/false/unknown); pin presence, not value
		// (CI-under-init could legitimately report true).
		if !strings.Contains(res.Stdout, "orphaned:") {
			t.Errorf("stdout missing 'orphaned:' field; got:\n%s", res.Stdout)
		}

		if res := <-resCh; res.ExitCode != 0 { // drain the holder so its lock releases
			t.Fatalf("holder exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
		}
	})

	// Dead: a planted lock file whose recorded pid is a GUARANTEED-dead pid (Start+Wait a `true`)
	// → processAlive returns false (ESRCH) → alive:false + "orphaned:  unknown (holder is dead)".
	t.Run("Dead", func(t *testing.T) {
		repo := newRepo(t)
		isolateLocks(t)
		// Capture a guaranteed-dead pid: Start a `true`, grab its pid, Wait (reaped → dead). Do NOT
		// use a magic number (999999) which could (rarely) be recycled to a live process.
		dead := exec.Command("true")
		if err := dead.Start(); err != nil {
			t.Skipf("cannot fork to obtain a dead pid (true not on PATH?): %v", err)
		}
		deadPid := dead.Process.Pid
		_ = dead.Wait()              // reaped → dead
		thisHost, _ := os.Hostname() // MUST be this host so processAlive takes the Kill(pid,0) path

		lp := lockFilePath(repo)
		plantLockFile(t, lp, strconv.Itoa(deadPid), thisHost, repo)
		env := stubEnv(nil)

		res := runStagecoach(t, bin, repo, cfg, env, "lock", "status")
		if res.ExitCode != 0 {
			t.Fatalf("lock status (dead) exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
		}
		if !strings.Contains(res.Stdout, "alive:     false") {
			t.Errorf("stdout missing 'alive:     false'; got:\n%s", res.Stdout)
		}
		if !strings.Contains(res.Stdout, "orphaned:  unknown (holder is dead)") {
			t.Errorf("stdout missing 'orphaned:  unknown (holder is dead)'; got:\n%s", res.Stdout)
		}
	})

	// Orphan (BEST-EFFORT, skip-guarded): produce a genuine ppid==1 holder (a `sleep` reparented to
	// init when its sh parent exits). appearsOrphaned returns true ONLY for ppid==1; under a subreaper
	// (systemd, containers) the reparented ppid is the subreaper, not 1 → verify ppid==1 and t.Skip
	// otherwise (no flake). The RELIABLE core of FR-K4 is NoLock/Live/Dead above.
	t.Run("Orphan", func(t *testing.T) {
		repo := newRepo(t)
		isolateLocks(t)

		// Fork a `sleep` that is reparented to init when sh exits (sh exits immediately after echo). The
		// `>/dev/null 2>&1` on the backgrounded sleep is MANDATORY: without it the grandchild inherits sh's
		// stdout pipe, so orph.Output()'s Wait blocks ~30s waiting for the pipe to close (Go's copy-all
		// pipes semantics). Redirecting the sleep's fds breaks the inheritance → Output returns instantly.
		orph := exec.Command("sh", "-c", "sleep 30 >/dev/null 2>&1 &\necho $!")
		out, err := orph.Output()
		if err != nil {
			t.Skipf("cannot produce a reparented sleep: %v", err)
		}
		sleepPid, err := strconv.Atoi(strings.TrimSpace(string(out)))
		if err != nil {
			t.Skipf("cannot parse reparented sleep pid (%q): %v", strings.TrimSpace(string(out)), err)
		}
		defer syscall.Kill(sleepPid, syscall.SIGKILL) // cleanup so the sleep doesn't linger

		ppid, err := ppidOf(sleepPid)
		if err != nil || ppid != 1 {
			t.Skipf("subreaper environment: reparented sleep ppid=%d (err=%v), need ppid==1 for orphaned:true (best-effort subtest)", ppid, err)
		}

		thisHost, _ := os.Hostname()
		lp := lockFilePath(repo)
		plantLockFile(t, lp, strconv.Itoa(sleepPid), thisHost, repo)
		env := stubEnv(nil)

		res := runStagecoach(t, bin, repo, cfg, env, "lock", "status")
		if res.ExitCode != 0 {
			t.Fatalf("lock status (orphan) exit = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
		}
		if !strings.Contains(res.Stdout, "alive:     true") {
			t.Errorf("stdout missing 'alive:     true'; got:\n%s", res.Stdout)
		}
		if !strings.Contains(res.Stdout, "orphaned:  true (holder reparented") {
			t.Errorf("stdout missing 'orphaned:  true (holder reparented…)'; got:\n%s", res.Stdout)
		}
	})
}
