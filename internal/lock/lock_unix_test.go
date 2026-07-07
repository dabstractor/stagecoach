//go:build !windows

package lock

import (
	"os"
	"os/exec"
	"testing"
)

func TestProcessAlive_SelfAlive(t *testing.T) {
	host, _ := os.Hostname()
	if !processAlive(os.Getpid(), host) {
		t.Errorf("processAlive(self, thisHost) = false, want true (self is alive)")
	}
}

func TestProcessAlive_ForeignHostConservative(t *testing.T) {
	if !processAlive(os.Getpid(), "definitely-not-this-host-zzz-999") {
		t.Errorf("processAlive(self, foreignHost) = false, want true (foreign host → don't reap)")
	}
}

func TestProcessAlive_EmptyHostnameConservative(t *testing.T) {
	if !processAlive(os.Getpid(), "") {
		t.Errorf("processAlive(self, emptyHost) = false, want true (empty host → don't reap)")
	}
}

func TestProcessAlive_DeadPID(t *testing.T) {
	// Fork a child that exits immediately; after Wait its pid is dead → ESRCH → processAlive == false.
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot fork to obtain a dead pid (true not on PATH?): %v", err)
	}
	deadPID := cmd.Process.Pid
	_ = cmd.Wait() // child exits; pid is now free/dead
	host, _ := os.Hostname()
	// Negligible race: the OS could recycle the freed pid in the microsecond window (pids are assigned
	// sequentially, so this won't happen until the counter wraps). A real bug (e.g. always-true) fails
	// this deterministically.
	if processAlive(deadPID, host) {
		t.Errorf("processAlive(deadPID=%d, thisHost) = true, want false (ESRCH → dead → reapable)", deadPID)
	}
}
