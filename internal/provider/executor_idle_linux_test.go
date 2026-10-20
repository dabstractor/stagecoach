//go:build linux

// Linux-only idle-engine coverage: the §9.30 FR-A3(b) read-counter channel (groupReadBytes rchar
// growth) as the SOLE activity signal — proving a stdout-silent agent that is genuinely working
// (reading stdin) survives the idle deadline. Portable idle coverage lives in executor_test.go.
package provider

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestExecute_IdleReadCounterKeepsReaderAlive: a stub that reads 10 stdin lines at 300ms each
// (~3s of genuine read activity) with ZERO stdout, under a 900ms idle deadline with
// ProgressiveStdout UNSET — so the pipe channel is unarmed and the rchar counter is the only
// channel that can observe the work. err==nil proves read growth resets the idle clock (FR-A3(b)).
func TestExecute_IdleReadCounterKeepsReaderAlive(t *testing.T) {
	mustBin(t, "sh")
	spec := CmdSpec{
		Command: "sh",
		Args:    []string{"-c", `while IFS= read -r line; do sleep 0.3; done`},
		Stdin:   strings.Repeat("line\n", 10),
		Env:     os.Environ(),
		// ProgressiveStdout deliberately UNSET — the rchar channel alone must keep the reader alive.
	}
	_, _, err := Execute(context.Background(), spec, Deadlines{Idle: 900 * time.Millisecond, Hard: 30 * time.Second}, nil)
	if err != nil {
		t.Fatalf("Execute: err = %v, want nil (rchar growth must count as activity for a stdout-silent reader)", err)
	}
}
