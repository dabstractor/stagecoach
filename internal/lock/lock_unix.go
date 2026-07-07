//go:build !windows

package lock

import (
	"errors"
	"os"
	"syscall"
)

// flock acquires an exclusive, non-blocking advisory lock on fd (LOCK_EX|LOCK_NB).
// On success the lock is held until fd is closed (auto-released on process death).
// On contention it returns an error wrapping syscall.EWOULDBLOCK.
func flock(fd int) error {
	return syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
}

// isWouldBlock reports whether err indicates the lock is held by another process.
func isWouldBlock(err error) bool {
	return errors.Is(err, syscall.EWOULDBLOCK)
}

// processAlive reports whether pid is a live process on hostname, for stale lock-FILE reaping
// (PRD §18.5). It is the pid-liveness check that makes unlinking a dead holder's lock file safe:
// a dead pid holds no open fd → no flock → unlinking cannot defeat contention the way unlinking a
// LIVE holder's inode-bound flock file would. SAFETY INVARIANT — never reap a live pid; conservative
// on every ambiguity:
//   - hostname == "" or != this host → true (foreign host: don't reap; a recycled pid on THIS host
//     is a benign miss, reaped once the pid is free).
//   - syscall.Kill(pid, 0) == nil → true (alive, ours); EPERM → true (alive, different user).
//   - any other error → false (ESRCH → dead → safe to reap).
//
// Cross-platform: lock_windows.go provides an always-true twin (flock is a no-op there → no reaping;
// the §13.5 CAS is the guarantee). Used by reapStaleLocks (P1.M2.T1.S2).
func processAlive(pid int, hostname string) bool {
	host, _ := os.Hostname()
	if hostname == "" || hostname != host {
		return true // foreign host → don't reap (conservative)
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true // alive
	}
	if errors.Is(err, syscall.EPERM) {
		return true // alive, different user
	}
	return false // ESRCH → dead
}
