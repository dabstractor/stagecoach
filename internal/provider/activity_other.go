//go:build !linux

package provider

// groupReadBytes is unavailable off Linux (macOS/Darwin/Windows have no /proc). Returns (0, false):
// the FR-A3 group-read channel reports "no channel", and Execute's idle watcher (P1.M2.T1.S2)
// falls back to the countingWriter channel alone (the hard cap still bounds the run).
//
// CONTRACT (FROZEN for P1.M2.T1.S2): the Linux build (activity_linux.go, //go:build linux) MUST
// implement the IDENTICAL signature `func groupReadBytes(pgid int) (uint64, bool)`. Do NOT change
// this signature without coordinating with S2.
func groupReadBytes(pgid int) (uint64, bool) { return 0, false }
