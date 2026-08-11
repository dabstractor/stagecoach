//go:build windows

package lock

// flock is a no-op on Windows. Windows has no POSIX flock; the §13.5 CAS
// (update-ref HEAD compare-and-swap) is the actual safety guarantee per
// PRD §18.5 (per-host limit). A no-op flock is correct for this
// defense-in-depth layer — the CAS catches everything on Windows.
func flock(fd int) error { return nil }

// isWouldBlock always returns false on Windows (no real flock contention).
func isWouldBlock(err error) bool { return false }

// processAlive is an intentional no-op on Windows: it always reports the pid as alive, so
// reapStaleLocks never removes a lock file on Windows. This is ACCEPTED behavior per FR-K7
// (PRD §9.27): the parent-death watchdog is Unix-only, and on Windows flock is already a no-op
// (see flock above), so the §13.5 CAS (update-ref HEAD compare-and-swap) is the SOLE correctness
// guarantee — a leftover lock file is inert disk litter, never a correctness or contention hazard.
//
// Why returning true (never reap) is correct + conservative:
//   - flock is a no-op on Windows (no inode-bound-flock hazard), so unlinking a lock file is always
//     SAFE here — but there is likewise no flock-based staleness to detect, so a real liveness probe
//     adds no safety. The §13.5 CAS guarantees a killed/aborted run can never land a wrong commit,
//     regardless of any leftover lock file.
//   - the "never reap a live pid" safety invariant (PRD §18.5) is trivially satisfied: reap nothing
//     ⇒ never reap a live pid. This is the conservative choice — it cannot cause a wrong reaping,
//     only leave benign litter.
//   - lock files ARE created on Windows (Acquire's os.OpenFile(O_CREATE) runs cross-platform), but
//     they are inert: the deferred Release removes the file on every NORMAL exit, and only an
//     ABNORMAL exit (taskkill /F, a crash) leaves a file behind. That litter is harmless (the CAS is
//     the guarantee) and bounded (normal exits dominate); it is the accepted tradeoff of FR-K7's
//     Unix-only-watchdog scope.
//
// A real Windows liveness probe (OpenProcess + WaitForSingleObject via golang.org/x/sys/windows, or
// syscall.OpenProcess) would let reaping work here, but it is low-value: it would only clean benign
// litter the CAS already renders harmless, adding a platform-API dependency for no correctness gain.
// Documented here as a deferred option (FR-K7's accepted scope), not a gap.
//
// Cross-platform twin of lock_unix.go's processAlive (syscall.Kill(pid, 0)); used by reapStaleLocks.
func processAlive(pid int, hostname string) bool {
	return true
}
