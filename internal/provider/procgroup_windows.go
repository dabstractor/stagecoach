//go:build windows

package provider

import (
	"os/exec"
	"syscall"
	"time"
)

// procGenerateConsoleCtrlEvent resolves kernel32!GenerateConsoleCtrlEvent lazily. The function is not
// exported by Go's stdlib syscall package; resolving it via syscall.LazyProc (stdlib, GOOS=windows)
// keeps go.mod/go.sum byte-unchanged (no golang.org/x/sys dependency) — matching procgroup_unix.go's
// stdlib-only principle. Resolved on first Call; kernel32 is always present on Windows.
var procGenerateConsoleCtrlEvent = syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")

// setupProcessGroup is the Windows implementation of the cross-platform seam declared in executor.go
// and FROZEN by P1.M2.T5.S1 (`func setupProcessGroup(cmd *exec.Cmd)`). Identical signature to
// procgroup_unix.go's setupProcessGroup — only the platform mechanism differs (critical_findings
// FINDING 10; go_ecosystem_patterns §3.4). executor.go (platform-agnostic, no syscall) calls this
// with no import (same package); on a Windows build the linker selects THIS file.
//
// Windows has no POSIX process groups, no Setpgid, and no syscall.Kill(-pid). The analog of "kill the
// whole child tree on ctx cancel" (PRD §18.4) is the console process-group mechanism:
//
//   - CREATE_NEW_PROCESS_GROUP (0x00000200) in SysProcAttr.CreationFlags: the child becomes a console
//     process-group leader; its PID == its process-group id (the exact parallel of Unix Setpgid ⇒
//     PGID==PID). Do NOT also set CREATE_NEW_CONSOLE — the child must inherit the caller's console or
//     GenerateConsoleCtrlEvent will not reach it.
//   - cmd.Cancel: on ctx cancel (timeout OR signal/parent cancel) call GenerateConsoleCtrlEvent(
//     CTRL_BREAK_EVENT, childPID) to signal the WHOLE group. CTRL_BREAK (not CTRL_C) because
//     CTRL_C is broadcast to every process sharing the caller's console and cannot be limited to one
//     group (Microsoft GenerateConsoleCtrlEvent docs); CTRL_BREAK honors dwProcessGroupId.
//   - cmd.WaitDelay = 3 * time.Second: if the child ignores CTRL_BREAK, os/exec escalates after 3s
//     (on Windows it TerminateProcess'es the direct child).
//
// ERROR CONTRACT (platform-agnostic, inherited from executor.go): on ctx cancel cmd.Wait() errors and
// ctx.Err() == context.Canceled (parent/signal cancel) or context.DeadlineExceeded (timeout); Execute
// returns that sentinel → orchestrator exit 3 / 124 + rescue (§18.2). No change to executor.go.
//
// KNOWN LIMITATION (v1; PRD §12.7.2 — document honestly): if a child detaches from the console
// (e.g. CREATE_NEW_CONSOLE of its own) or installs a handler that swallows CTRL_BREAK, grandchildren
// may survive escalation (os/exec's WaitDelay escalation TerminateProcess'es only the direct child).
// The robust whole-tree kill is Job Objects (golang/go #17608), deferred beyond v1. The work item's
// specified v1 approach is CREATE_NEW_PROCESS_GROUP + GenerateConsoleCtrlEvent (this implementation).
func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP, // child = console process-group leader; PID==PGID
	}
	cmd.Cancel = func() error {
		// cmd.Process is guaranteed non-nil inside Cancel (os/exec calls it only after Start() succeeds).
		// CTRL_BREAK_EVENT (1) targets the specific group == child PID (NOT a negative pid; Windows has no -pid).
		r1, _, err := procGenerateConsoleCtrlEvent.Call(
			uintptr(syscall.CTRL_BREAK_EVENT),
			uintptr(cmd.Process.Pid),
		)
		if r1 == 0 {
			// GenerateConsoleCtrlEvent failed (e.g. child already exited) — non-fatal: WaitDelay escalates
			// to TerminateProcess on the direct child. Return err so os/exec logs it (does not fail Wait).
			return err
		}
		return nil
	}
	cmd.WaitDelay = 3 * time.Second
}
