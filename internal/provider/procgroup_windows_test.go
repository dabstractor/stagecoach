//go:build windows

package provider

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// TestSetupProcessGroup_Wiring is the deterministic Windows-side structural check. S1's
// executor_test.go has no build tag and runs on the Windows CI leg, but most of its cases shell out to
// Unix binaries (cat/sleep/printenv/false) absent on windows-2022 → mustBin skips them, leaving the
// Windows setupProcessGroup weakly exercised. This test asserts the wiring directly with no external
// binary — it runs only on Windows (build tag) and compiles via `GOOS=windows go vet`.
func TestSetupProcessGroup_Wiring(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "exit", "0") // cmd.exe is always present on Windows
	setupProcessGroup(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("cmd.SysProcAttr == nil; want non-nil")
	}
	if cmd.SysProcAttr.CreationFlags&syscall.CREATE_NEW_PROCESS_GROUP == 0 {
		t.Errorf("CreationFlags = %#x; want CREATE_NEW_PROCESS_GROUP (%#x) bit set",
			cmd.SysProcAttr.CreationFlags, syscall.CREATE_NEW_PROCESS_GROUP)
	}
	if cmd.Cancel == nil {
		t.Error("cmd.Cancel == nil; want the GenerateConsoleCtrlEvent(CTRL_BREAK) closure")
	}
	if cmd.WaitDelay != 3*time.Second {
		t.Errorf("cmd.WaitDelay = %v; want 3s (match procgroup_unix.go)", cmd.WaitDelay)
	}
}
