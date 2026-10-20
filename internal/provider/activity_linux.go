//go:build linux

package provider

import (
	"bytes"
	"os"
	"strconv"
)

// groupReadBytes sums /proc/<pid>/io rchar across every LIVE member of process group pgid — the
// FR-A3 (§9.30) INPUT-SIDE activity channel. rchar counts bytes returned by SUCCESSFUL
// read(2)-family calls: pipes/tty/sockets count, pagecache hits count, failed reads count nothing
// (NOT read_bytes — the block-layer counter, which would miss pipe/tty activity entirely). An agent
// silently spinning on input without emitting stdout is invisible to countingWriter; rchar growth
// is what detects it as active.
//
// PGID==PID invariant: procgroup_unix.go's setupProcessGroup sets SysProcAttr.Setpgid, making the
// child a NEW process-group leader — its PGID equals its PID by kernel guarantee. Callers
// (P1.M2.T1.S2's watcher) therefore pass cmd.Process.Pid, and this function covers the child + ALL
// descendants (the whole group).
//
// Semantics (external_procfs_notes.md, fetched 2026-10-20 against man-pages 6.19 + docs.kernel.org +
// torvalds/linux):
//   - stat is world-readable; io requires same-uid + PTRACE_MODE_READ (dumpable processes pass; Yama
//     never restricts READ mode). Agent children are dumpable ⇒ readable without root.
//   - comm (stat field 2) may contain spaces AND parentheses — parse AFTER THE LAST ')' so the field
//     layout never misaligns (the proc(5)-undocumented maintainer rule).
//   - A full sweep over ~1100 pids ≈ 13ms ≈ 2.6% duty at 2Hz — the scan design is cheap because the
//     pgrp FILTER runs BEFORE any io open (kernel threads' io is root-owned ⇒ EACCES if opened blindly).
//   - Reaped members (dir gone after waitpid) are skipped via ENOENT; reaped children's ioac folds
//     into the parent's rchar, which can only false-positive activity within the group, never
//     false-negative — the safe direction for an idle kill. Counters are non-atomic (proc_pid_io(5)
//     CAVEATS); delta-clamping is the CONSUMER's job (S2), not this summer's.
//
// CONTRACT (FROZEN for P1.M2.T1.S2): the non-Linux build (activity_other.go, //go:build !linux) MUST
// implement the IDENTICAL signature `func groupReadBytes(pgid int) (uint64, bool)` returning (0,false).
// Do NOT change this signature without coordinating with S2. (sum, ok): ok==false means "NO CHANNEL"
// — ONLY os.ReadDir("/proc") failure; an EMPTY MATCH SET is (0, true) (the channel works, the group
// just read nothing), and every per-member stat/io failure is a SKIP, never an error.
func groupReadBytes(pgid int) (uint64, bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, false // no /proc — the ONLY (…,false): the channel itself is unavailable
	}
	var sum uint64
	for _, e := range entries {
		pidStr := e.Name()
		if _, err := strconv.Atoi(pidStr); err != nil {
			continue // not a pid dir (irq/, net/, acpi/, …)
		}
		// Parse stat FIRST (world-readable) and filter by pgrp BEFORE opening any io file — kernel
		// threads match no user pgid, so their root-owned io is never opened.
		stat, err := os.ReadFile("/proc/" + pidStr + "/stat")
		if err != nil {
			continue // reaped between readdir and stat — member gone, never an error
		}
		i := bytes.LastIndexByte(stat, ')') // comm may contain spaces AND parens — last ')' is the field anchor
		if i < 0 || i+1 >= len(stat) {
			continue // malformed stat — skip
		}
		fields := bytes.Fields(stat[i+1:]) // token 0=state, 1=ppid, 2=pgrp
		if len(fields) < 3 {
			continue // malformed — skip
		}
		pgrp, err := strconv.Atoi(string(fields[2]))
		if err != nil || pgrp != pgid {
			continue // unparseable, or not a group member — FILTER (before any io open)
		}
		ioData, err := os.ReadFile("/proc/" + pidStr + "/io")
		if err != nil {
			continue // reaped mid-sweep (ENOENT) or non-dumpable (EACCES) — skip
		}
		if n, ok := parseRchar(ioData); ok {
			sum += n
		}
	}
	return sum, true
}

// parseRchar extracts the "rchar: <decimal>" counter from a /proc/<pid>/io payload. ok is false when
// the line is absent or unparseable (the member is skipped by the caller, not an error).
func parseRchar(ioData []byte) (uint64, bool) {
	for _, line := range bytes.Split(ioData, []byte{'\n'}) {
		rest, found := bytes.CutPrefix(line, []byte("rchar:"))
		if !found {
			continue
		}
		n, err := strconv.ParseUint(string(bytes.TrimSpace(rest)), 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}
