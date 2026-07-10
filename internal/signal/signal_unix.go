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

// caughtSignals returns the signals this platform's handler intercepts (FR-K3). Unix adds SIGHUP
// so a controlling-terminal hangup routes through rescue instead of a raw terminate (the kernel
// delivers SIGHUP to the process group when the terminal closes). Windows has no SIGHUP concept
// (FR-K7); its twin in signal_windows.go omits it.
func caughtSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}
}

// exitCodeForSignal returns the conventional 128+signum exit code for an aborted run (§18.4 step 2
// "else just exit"). Used only for PRE-snapshot signals (post-snapshot is hardcoded exit 3).
// SIGHUP→129 (128+1), SIGINT→130 (128+2), SIGTERM→143 (128+15).
func exitCodeForSignal(sig os.Signal) int {
	switch sig {
	case syscall.SIGHUP:
		return 129 // 128 + 1
	case os.Interrupt, syscall.SIGINT:
		return 130 // 128 + 2
	case syscall.SIGTERM:
		return 143 // 128 + 15
	default:
		return 1
	}
}
