//go:build !windows

package signal

import (
	"bytes"
	"os"
	"syscall"
	"testing"
)

// TestExitCodeForSignal_Unix directly exercises the Unix exitCodeForSignal switch, including the
// new SIGHUP→129 case (and a regression guard for SIGINT→130 / SIGTERM→143). It is a cleaner unit
// test than routing every case through handle(); the handle()-level SIGHUP path is covered by
// TestHandler_Exit129SIGHUP / TestHandler_RescueOnSIGHUPWithSnapshot below.
func TestExitCodeForSignal_Unix(t *testing.T) {
	cases := []struct {
		name string
		sig  os.Signal
		want int
	}{
		{"SIGHUP", syscall.SIGHUP, 129}, // 128 + 1 — NEW (FR-K3)
		{"SIGINT", syscall.SIGINT, 130}, // 128 + 2
		{"os.Interrupt", os.Interrupt, 130},
		{"SIGTERM", syscall.SIGTERM, 143}, // 128 + 15
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := exitCodeForSignal(tc.sig); got != tc.want {
				t.Errorf("exitCodeForSignal(%v) = %d, want %d", tc.sig, got, tc.want)
			}
		})
	}
}

// TestCaughtSignals_UnixIncludesSIGHUP verifies the Unix caught set includes SIGHUP (alongside
// SIGINT and SIGTERM), so a controlling-terminal hangup routes through rescue (FR-K3).
func TestCaughtSignals_UnixIncludesSIGHUP(t *testing.T) {
	sigs := caughtSignals()

	has := func(target os.Signal) bool {
		for _, s := range sigs {
			if s == target {
				return true
			}
		}
		return false
	}

	if !has(syscall.SIGHUP) {
		t.Errorf("caughtSignals() = %v, want it to contain syscall.SIGHUP (FR-K3)", sigs)
	}
	if !has(os.Interrupt) {
		t.Errorf("caughtSignals() = %v, want it to contain os.Interrupt", sigs)
	}
	if !has(syscall.SIGTERM) {
		t.Errorf("caughtSignals() = %v, want it to contain syscall.SIGTERM", sigs)
	}
	if len(sigs) != 3 {
		t.Errorf("len(caughtSignals()) = %d, want 3 (SIGINT/SIGTERM/SIGHUP on Unix)", len(sigs))
	}
}

// TestHandler_Exit129SIGHUP mirrors TestHandler_Exit143SIGTERM: a SIGHUP with NO snapshot armed
// takes the pre-snapshot branch → exit 129 (128 + 1), no rescue message.
func TestHandler_Exit129SIGHUP(t *testing.T) {
	var exitCode int

	h := installTestHandler(t, Options{
		Exit: func(code int) { exitCode = code },
		Out:  new(bytes.Buffer),
	})

	h.handle(syscall.SIGHUP)

	if exitCode != 129 {
		t.Errorf("exitCode = %d, want 129 (SIGHUP pre-snapshot)", exitCode)
	}
}

// TestHandler_RescueOnSIGHUPWithSnapshot mirrors TestHandler_RescueOnSignalWithSnapshot but drives
// SIGHUP: with a snapshot armed, SIGHUP takes the rescue path (exit 3) AND forwards SIGHUP to the
// child process group. Proves SIGHUP is a first-class rescue signal that needs no handle() change.
func TestHandler_RescueOnSIGHUPWithSnapshot(t *testing.T) {
	var killedPid int
	var killedSig os.Signal
	var exitCode int
	buf := &bytes.Buffer{}

	h := installTestHandler(t, Options{
		RescueFormat: func(tree, parent, cand string) string {
			return "RESCUE: Tree=" + tree + " Parent=" + parent + " Cand=" + cand
		},
		Kill: func(pid int, sig os.Signal) error {
			killedPid = pid
			killedSig = sig
			return nil
		},
		Exit: func(code int) { exitCode = code },
		Out:  buf,
	})

	RegisterChild(7777)
	SetSnapshot("abc123", "def456", "feat: hup rescue")
	h.handle(syscall.SIGHUP)

	if exitCode != 3 {
		t.Errorf("exitCode = %d, want 3 (post-snapshot SIGHUP rescue)", exitCode)
	}
	if killedPid != 7777 {
		t.Errorf("Kill pid = %d, want 7777 (SIGHUP forwarded to child group)", killedPid)
	}
	if killedSig != syscall.SIGHUP {
		t.Errorf("Kill sig = %v, want syscall.SIGHUP (forwarded signal)", killedSig)
	}
	got := buf.String()
	if !contains(got, "Tree=abc123") {
		t.Errorf("rescue output missing Tree=abc123: %q", got)
	}
}
