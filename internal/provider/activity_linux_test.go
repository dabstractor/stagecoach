//go:build linux

package provider

import (
	"io"
	"os"
	"os/exec"
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
	// LIVENESS IS THE LOAD-BEARING PART of the "counter moves while it runs" contract, so the test
	// must GUARANTEE it, not hope for it. cat is fed through a pipe the test HOLDS OPEN: it drains the
	// 8MiB and then blocks on the next read, provably alive at both samples. A strings.Reader closes
	// stdin at EOF, and cat drains 8MiB well inside the 20ms sample gap on a fast runner — the child
	// then sits exited-unreaped, and a vanished member is indistinguishable in the group sum from a
	// counter that moved backwards (CI kernels fail this way 100%; some retain zombie io counters,
	// some do not — the test must not depend on which).
	feedR, feedW, perr := os.Pipe()
	if perr != nil {
		t.Fatalf("os.Pipe: %v", perr)
	}
	cmd := exec.Command("cat")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // own group ⇒ PGID == cmd.Process.Pid
	cmd.Stdin = feedR
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Feed 8MiB; the write returns once ≤ one pipe buffer (64KiB) remains unread, so the child has
	// read ≥ 8MiB−64KiB by the time sampling starts. The write end stays open: no EOF, no exit.
	if _, werr := feedW.Write(make([]byte, 8<<20)); werr != nil {
		t.Fatalf("feed pipe: %v (child died early?)", werr)
	}

	// While the child runs: ok==true and the counter moves (>0). The 8MiB drain is already in rchar
	// (plus the loader's reads), and a live process's counters are monotonic in the kernel, so two
	// samples 20ms apart must be non-decreasing — the assertion is sound because the child CANNOT
	// have exited between them (it is blocked on a read from a pipe we hold open).
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
	feedW.Close() // EOF — cat exits
	_ = feedR.Close()
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
