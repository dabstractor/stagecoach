// rollback_windows_test.go exercises the Windows platformRollback 3-step .old-routing dance + the
// FR-U8 no-op case on Windows. The running-.exe lock is a Windows kernel property; here regular
// files prove the rename sequence (the end-to-end lock behavior is covered by P1.M4.T3's e2e). NOT
// parallel where it mutates resolveCurrentExe (TestRollback_NoBackup); the platformRollback unit test
// touches only its own temp dir. Stdlib-only. Runs natively on windows-latest CI (.github/workflows/
// ci.yml matrix).
//
// REUSES the LANDED same-file helpers from swap_windows_test.go (writeExe / readFile / pointExeAt /
// restoreResolveCurrentExe) — they are package-level test helpers in the //go:build windows file,
// reachable from this //go:build windows file. Do NOT redeclare them.
//
//go:build windows

package upgrade

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestPlatformRollback_OldDance verifies the Windows 3-step rotate: with currentExe = "NEW" and .old
// (the backup) = "OLD", after platformRollback currentExe holds "OLD" (the backup content restored)
// and .old holds "NEW" (the prior-current, routed through .old for CleanupOldBinary to reclaim next
// launch), and ONLY those two files remain in the dir (the aside scratch name is consumed — no litter).
func TestPlatformRollback_OldDance(t *testing.T) {
	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "stagecoach.exe")
	writeExe(t, exe, "NEW")
	old := exe + ".old" // == backupPath(exe) on Windows
	writeExe(t, old, "OLD")

	if err := platformRollback(exe, old); err != nil {
		t.Fatalf("platformRollback: unexpected error: %v", err)
	}

	if got := readFile(t, exe); got != "OLD" {
		t.Errorf("exe content after rollback = %q, want %q (restored)", got, "OLD")
	}
	if got := readFile(t, exe+".old"); got != "NEW" {
		t.Errorf(".old content after rollback = %q, want %q (prior-current routed to .old)", got, "NEW")
	}
	// The aside scratch name MUST be consumed by step 3 — only exe + .old remain (no litter).
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("readdir %s: %v", tempDir, err)
	}
	if len(entries) != 2 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("dir entries after rollback = %v; want exactly 2 (exe + .old — no aside litter)", names)
	}
}

// TestRollback_NoBackup mirrors the Unix no-backup test: with no backup present, Rollback returns
// ErrNoBackup and leaves the current binary byte-for-byte unchanged. NOT parallel (mutates
// resolveCurrentExe).
func TestRollback_NoBackup(t *testing.T) {
	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "stagecoach.exe")
	writeExe(t, exe, "CURRENT")
	pointExeAt(t, exe)

	version, err := Rollback(context.Background())
	if !errors.Is(err, ErrNoBackup) {
		t.Fatalf("Rollback no-backup: errors.Is(err, ErrNoBackup) = false; err = %v", err)
	}
	if version != "" {
		t.Errorf("version = %q; want empty (no-op)", version)
	}
	if got := readFile(t, exe); got != "CURRENT" {
		t.Errorf("exe content after no-backup = %q, want %q (unchanged)", got, "CURRENT")
	}
}
