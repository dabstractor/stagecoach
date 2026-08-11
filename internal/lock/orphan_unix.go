//go:build !windows

package lock

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// appearsOrphaned is a HEURISTIC that reports whether the holder process (pid)
// APPEARS orphaned — i.e. reparented to init/launchd because its launcher exited
// without killing it (PRD §9.27's "orphaned-but-alive" case, the lock-stays-
// forever hazard FR-K4 diagnoses). It is called by Status and IsOrphaned ONLY
// when the holder is alive (a dead pid will be reaped anyway, so its orphan
// status is moot). It is strictly READ-ONLY — it feeds the user's kill/rm
// decision (FR-K4 lock status + FR-K5 busy hint); it changes nothing and NEVER
// force-breaks a live lock (FR52's invariant holds regardless of this hint).
//
// The heuristic: ppid == 1 ⇒ reparented to init. CONSERVATIVE INVARIANT — it
// returns false on ANY error or ambiguity (proc gone, ps failure, parse failure):
// orphan detection is a diagnostic HINT feeding the user's kill/rm decision, so
// a false-positive orphan claim (prompting the user to kill a legitimately-
// parented run) is worse than a false negative. The only `true` is ppid == 1.
//
// LIMITATION (subreapers — BUG-011, accepted): under a subreaper
// (PR_SET_CHILD_SUBREAPER — systemd, systemd-run, Docker/containerd/runc,
// podman, supervisord, some shells) a reparented orphan's ppid is the
// SUBREAPER's pid, not 1, so this hint can MISS orphans (false negative); it
// never false-positives a legitimately-parented process. This is the SAME test
// FR-K2 explicitly REJECTS for the watchdog — "never by the brittle
// 'getppid() == 1' test (wrong under subreapers — systemd-run, some shells —
// and for processes legitimately spawned by init)" — and appearsOrphaned is the
// one place that still uses it. It does so DELIBERATELY: it is a SNAPSHOT
// diagnostic that sees only the holder's CURRENT ppid (via ppidOf), with NO
// startup baseline to diff, so it CANNOT use the parent-pid-CHANGE detection
// the watchdog uses. ppid==1 is the only ZERO-false-positive snapshot answer.
//
// The AUTHORITATIVE orphan detector is internal/watchdog/arm_unix.go's poll
// `osGetppid() != originalPpid` (FR-K2, subreaper-safe): it captures the parent
// pid at startup, fires on the CHANGE (reparenting to init OR a subreaper), and
// routes the holder through the §18.3 rescue + §18.5 lock-release exit path — so
// this hint's false negative NEVER leaves a live lock orphaned-forever.
// appearsOrphaned is DISPLAY-ONLY (FR-K4 lock status + FR-K5 busy hint);
// arm_unix.go is the safety property. The orphan==true path is proven end-to-end
// by the E2E harness (P1.M4.T1.S1).
//
// Option A (a comm-based enhancement — read /proc/<ppid>/comm and flag the ppid
// as a subreaper when comm ∈ {systemd, containerd, dockerd, supervisord, init,
// launchd}) is DELIBERATELY DEFERRED — it would false-positive a process
// legitimately parented to systemd (a unit, a user-session child), violating the
// conservative invariant above. The false negative is the accepted design
// (architecture §BUG-011); the watchdog backstop makes it safe.
//
// Cross-platform: orphan_windows.go provides an always-false twin (FR-K7 —
// Windows has no init-reparenting analog). Platform dispatch is via runtime.GOOS
// in a single file (not build-tag-per-OS): every import is referenced by ≥1
// compiled function, so the whole file compiles on every Unix target.
func appearsOrphaned(pid int) bool {
	ppid, err := ppidOf(pid)
	if err != nil {
		return false // conservative: don't claim orphan on ambiguity
	}
	// Reparented to init/launchd. Under a subreaper (systemd/dockerd/containerd/
	// supervisord — see LIMITATION above) the orphan's ppid is the subreaper's
	// pid, not 1 ⇒ false negative (display-only). The AUTHORITATIVE detector is
	// arm_unix.go's `osGetppid() != originalPpid` poll (FR-K2, subreaper-safe).
	return ppid == 1
}

// ppidOf returns the parent pid of pid, dispatching on runtime.GOOS. Linux reads
// /proc/<pid>/status (no fork); everything else (darwin/BSDs) shells out to ps.
func ppidOf(pid int) (int, error) {
	if runtime.GOOS == "linux" {
		return ppidLinux(pid)
	}
	return ppidViaPs(pid)
}

// ppidLinux reads the PPid: field from /proc/<pid>/status. ENOENT (pid gone) or
// any other Open/scan error propagates; appearsOrphaned maps it to false.
func ppidLinux(pid int) (int, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err // ENOENT when pid is gone → appearsOrphaned returns false
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		// strings.Fields splits on whitespace; the colon stays glued to "PPid",
		// so fields[0] == "PPid:" (the WHOLE token — NOT a bare prefix, to avoid
		// matching the "Pid:" line that precedes "PPid:" in /proc/<pid>/status).
		fields := strings.Fields(s.Text())
		if len(fields) >= 2 && fields[0] == "PPid:" {
			return strconv.Atoi(fields[1])
		}
	}
	if err := s.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("orphan: no PPid field for pid %d", pid)
}

// ppidViaPs runs `ps -o ppid= -p <pid>` (the trailing '=' suppresses the header)
// and parses the right-justified number. A non-zero exit (pid missing) returns
// the *exec.ExitError; appearsOrphaned maps it to false.
func ppidViaPs(pid int) (int, error) {
	out, err := exec.Command("ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, err // *exec.ExitError when pid is missing → appearsOrphaned returns false
	}
	// TrimSpace is MANDATORY: ps right-justifies the number (e.g. "     1"),
	// and strconv.Atoi fails on leading whitespace.
	return strconv.Atoi(strings.TrimSpace(string(out)))
}
