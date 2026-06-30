//go:build !windows

package signal

import (
	"os"
	"syscall"
)

// KillProcessGroup sends sig to the child's entire process group. The caller passes the POSITIVE
// child PID; Setpgid ⇒ PGID==PID, so -pid addresses the whole tree (child + all grandchildren).
// This is the SAME idiom as procgroup_unix.go's cmd.Cancel (F2); duplicated intentionally so
// procgroup_*.go stays frozen. The grace-then-SIGKILL escalation is the executor's cmd.WaitDelay
// (3s) — NOT our job.
func KillProcessGroup(pid int, sig os.Signal) error {
	return syscall.Kill(-pid, sig.(syscall.Signal)) // -pid ⇒ whole group
}

// exitCodeForSignal returns the conventional 128+signum exit code for an aborted run (§18.4 step 2
// "else just exit"). Used only for PRE-snapshot signals (post-snapshot is hardcoded exit 3).
func exitCodeForSignal(sig os.Signal) int {
	switch sig {
	case os.Interrupt, syscall.SIGINT:
		return 130 // 128 + 2
	case syscall.SIGTERM:
		return 143 // 128 + 15
	default:
		return 1
	}
}
