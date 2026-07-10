//go:build windows

package signal

import (
	"os"
	"syscall"
	"testing"
)

// TestCaughtSignals_WindowsExcludesSIGHUP verifies the Windows caught set is exactly {os.Interrupt,
// syscall.SIGTERM} — no SIGHUP (FR-K7). syscall.SIGHUP does not exist on Windows, so it MUST NOT be
// named anywhere in this file (the build would break). This test only runs under GOOS=windows
// (Windows CI); it guards against accidentally adding SIGHUP to the Windows set later.
func TestCaughtSignals_WindowsExcludesSIGHUP(t *testing.T) {
	sigs := caughtSignals()

	if len(sigs) != 2 {
		t.Fatalf("len(caughtSignals()) = %d, want 2 (SIGINT/SIGTERM only on Windows — FR-K7)", len(sigs))
	}

	has := func(target os.Signal) bool {
		for _, s := range sigs {
			if s == target {
				return true
			}
		}
		return false
	}

	if !has(os.Interrupt) {
		t.Errorf("caughtSignals() = %v, want it to contain os.Interrupt", sigs)
	}
	if !has(syscall.SIGTERM) {
		t.Errorf("caughtSignals() = %v, want it to contain syscall.SIGTERM", sigs)
	}
}
