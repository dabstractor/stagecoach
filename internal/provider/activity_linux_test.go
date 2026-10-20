//go:build linux

package provider

import (
	"io"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestGroupReadBytes_OwnGroupVisible: the test's own process group is scannable — ok==true is the
// hard gate (the channel works); the test binary has done file I/O so sum is typically >0, but only
// sum>=0 is asserted (the value is timing-dependent).
func TestGroupReadBytes_OwnGroupVisible(t *testing.T) {
	pgid := syscall.Getpgrp()
	sum, ok := groupReadBytes(pgid)
	if !ok {
		t.Fatal("groupReadBytes(own pgid) ok = false, want true (the /proc channel must work on Linux)")
	}
	t.Logf("own group pgid=%d rchar sum=%d", pgid, sum)
}

// TestGroupReadBytes_ChildGroupCounterMoves is the contract's core test: a Setpgid `cat` child in
// its OWN group (PGID==PID, the procgroup_unix.go invariant) reading an 8MiB pipe shows a moving
// rchar counter while it runs; after cmd.Wait() reaps it, the sweep must STILL return ok==true —
// the reaped member silently excluded (ENOENT tolerance), never a "no channel".
func TestGroupReadBytes_ChildGroupCounterMoves(t *testing.T) {
	mustBin(t, "cat")
	cmd := exec.Command("cat")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // own group ⇒ PGID == cmd.Process.Pid
	cmd.Stdin = strings.NewReader(strings.Repeat("x", 8<<20))
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// While the child runs: ok==true and the counter moves (>0). exec alone implies loader reads,
	// and the 8MiB pipe reads accumulate. Re-sample (20ms apart) to absorb a very fast box: the
	// first sample may land before the loader finished; either sample must show sum>0, and the
	// sequence must be non-decreasing (rchar is a monotonic counter for a live process).
	pid := cmd.Process.Pid
	sum1, ok := groupReadBytes(pid)
	if !ok {
		t.Fatalf("groupReadBytes(child pgid=%d) ok = false while child runs, want true", pid)
	}
	time.Sleep(20 * time.Millisecond)
	sum2, ok := groupReadBytes(pid)
	if !ok {
		t.Fatalf("groupReadBytes(child pgid=%d) ok = false on re-sample, want true", pid)
	}
	if sum2 < sum1 {
		t.Errorf("rchar moved backwards for a live child: %d -> %d (counter must be non-decreasing)", sum1, sum2)
	}
	if sum1 <= 0 && sum2 <= 0 {
		t.Errorf("rchar = %d then %d for a child that read a loader + 8MiB pipe, want >0 on either sample", sum1, sum2)
	}

	// Reap the child; the group is now empty of live members. The sweep must STILL report ok==true
	// (an empty match set is (0,true) — the channel works; the member is silently excluded).
	_ = cmd.Wait()
	_, ok = groupReadBytes(pid)
	if !ok {
		t.Error("groupReadBytes(child pgid) ok = false after reap, want true (post-reap ENOENT tolerance)")
	}
}

// TestGroupReadBytes_BogusPgid: a pgid matching no process yields (0, true) — the scan works and the
// match set is empty; it must NOT flip ok to false (false means "no channel", which would silently
// disable input-side detection).
func TestGroupReadBytes_BogusPgid(t *testing.T) {
	sum, ok := groupReadBytes(1 << 30)
	if !ok {
		t.Fatal("groupReadBytes(bogus pgid) ok = false, want true (empty match set is (0,true))")
	}
	if sum != 0 {
		t.Errorf("groupReadBytes(bogus pgid) sum = %d, want 0", sum)
	}
}
