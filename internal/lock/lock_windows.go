//go:build windows

package lock

// flock is a no-op on Windows. Windows has no POSIX flock; the §13.5 CAS
// (update-ref HEAD compare-and-swap) is the actual safety guarantee per
// PRD §18.5 (per-host limit). A no-op flock is correct for this
// defense-in-depth layer — the CAS catches everything on Windows.
func flock(fd int) error { return nil }

// isWouldBlock always returns false on Windows (no real flock contention).
func isWouldBlock(err error) bool { return false }

// processAlive is a conservative no-op on Windows: it always reports the pid as alive (never reap).
// flock is a no-op on Windows (no inode-bound-flock hazard — see flock above), so there is no
// dead-file reaping to do; the §13.5 CAS (update-ref HEAD compare-and-swap) is the safety guarantee.
// The "never reap a live pid" invariant is trivially satisfied (reap nothing). Cross-platform twin
// of lock_unix.go's processAlive; used by reapStaleLocks (P1.M2.T1.S2).
func processAlive(pid int, hostname string) bool {
	return true
}
