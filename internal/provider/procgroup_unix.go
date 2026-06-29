//go:build !windows

package provider

import (
	"os/exec"
	"syscall"
	"time"
)

// setupProcessGroup configures cmd so that on context cancellation (timeout or parent/signal cancel)
// the ENTIRE child process tree is terminated, preventing orphaned grandchildren (PRD §18.4, FINDING 8,
// go_ecosystem_patterns §3). It sets three fields on cmd:
//
//   - SysProcAttr.Setpgid = true → the child is a new process-group leader; its PGID == its PID, so
//     -pid addresses the whole group (child + all descendants).
//   - cmd.Cancel → on ctx cancel, send SIGTERM to the group (-pid). Gentler than the default SIGKILL
//     (lets the agent flush), and reaches grandchildren (the default only kills the direct child).
//     cmd.Process is guaranteed non-nil inside Cancel (os/exec invokes it only after Start()).
//   - cmd.WaitDelay = 3s → after Cancel, wait 3s for exit before Go forcibly SIGKILLs (handles an
//     agent that ignores SIGTERM).
//
// CONTRACT (FROZEN for P1.M2.T5.S2): the Windows build (procgroup_windows.go, //go:build windows) MUST
// implement the IDENTICAL signature `func setupProcessGroup(cmd *exec.Cmd)` using Job Objects /
// CREATE_NEW_PROCESS_GROUP. Do NOT change this signature without coordinating with S2.
//
// Unix implementation (Linux/macOS/darwin). The CI matrix targets Linux + macOS (PRD §20.4).
func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) // -pid ⇒ whole process group
	}
	cmd.WaitDelay = 3 * time.Second
}
