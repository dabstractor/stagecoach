//go:build windows

package signal

import (
	"os"
	"syscall"
)

// procGenerateConsoleCtrlEvent resolves kernel32!GenerateConsoleCtrlEvent lazily (stdlib-only — no
// golang.org/x/sys dependency, matching procgroup_windows.go). CTRL_BREAK (not CTRL_C) because CTRL_C
// can't be limited to one console process group.
var procGenerateConsoleCtrlEvent = syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")

// KillProcessGroup is the Windows analog (FINDING 10): CREATE_NEW_PROCESS_GROUP ⇒ the child's PID is
// its console process-group id, so GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, pid) signals the whole
// group. sig is effectively ignored (always CTRL_BREAK for graceful; force-escalation is the executor's
// WaitDelay/TerminateProcess). The caller passes the POSITIVE pid (do NOT negate — Windows has no
// -pid concept).
func KillProcessGroup(pid int, sig os.Signal) error {
	r1, _, err := procGenerateConsoleCtrlEvent.Call(
		uintptr(syscall.CTRL_BREAK_EVENT),
		uintptr(pid),
	)
	if r1 == 0 {
		return err // non-fatal: the executor's WaitDelay escalation handles a stubborn child
	}
	return nil
}

// exitCodeForSignal (Windows). SIGINT via Ctrl-C → 130. SIGTERM is not deliverable on Windows but
// is defined as a const; map it to 143 for consistency with Unix (the branch won't fire in practice).
func exitCodeForSignal(sig os.Signal) int {
	switch sig {
	case os.Interrupt, syscall.SIGINT:
		return 130
	case syscall.SIGTERM:
		return 143
	default:
		return 1
	}
}
