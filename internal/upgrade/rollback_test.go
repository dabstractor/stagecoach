// rollback_test.go exercises Rollback's FR-U8 contract on Unix: the restore case (backup present +
// runnable → restored, backup consumed), the no-op case (no backup → ErrNoBackup, current unchanged),
// and the refuse case (backup unrunnable → ErrBackupUnusable, current AND backup unchanged). It is
// NOT parallel: the tests mutate the package-level resolveCurrentExe seam, and two concurrent tests
// racing on that var would trip the race detector (exactly swap_test.go's documented constraint for
// resolveCurrentExe / stage_test.go's for execVersion). The sibling upgrade tests never touch these
// seams and may stay parallel. Stdlib-only.
//
// The backup binary is the REAL compiled cmd/stubcli (via buildStubCLI, stage_test.go) — a cross-
// platform compiled binary that ignores argv, prints STAGECOACH_STUBCLI_OUT, and exits
// STAGECOACH_STUBCLI_EXIT. So `backup --version` is exactly "does the backup run + exit 0" (FR-U8).
// The DEFAULT execVersion is used (no saveExecVersion): cmd.Env is unset ⇒ the stub child inherits
// os.Environ incl. the t.Setenv values (proven by stage_test.go's HappyPath). Only resolveCurrentExe
// is overridden (pointed at a temp-dir "exe" — you cannot os.Rename over the REAL running binary).
//
//go:build !windows

package upgrade

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestRollback_RestoresBackup verifies the FR-U8 success case: a backup that runs (--version exits 0)
// is restored over the current binary, the current binary now equals the backup content, the backup
// FILE is CONSUMED (gone — one-step), and restoredVersion is the backup's trimmed --version output.
func TestRollback_RestoresBackup(t *testing.T) {
	// NOT parallel — mutates resolveCurrentExe.
	stub := buildStubCLI(t)
	// The stub's --version output. The DEFAULT execVersion leaves cmd.Env unset ⇒ the child inherits
	// os.Environ incl. this t.Setenv (proven by stage_test.go's HappyPath).
	t.Setenv("STAGECOACH_STUBCLI_OUT", "v9.9.9\n")

	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "stagecoach")
	writeExe(t, exe, "CURRENT") // the (unwanted) current binary
	backup := exe + ".stagecoach-backup"
	stubBytes, err := os.ReadFile(stub)
	if err != nil {
		t.Fatalf("read stub %s: %v", stub, err)
	}
	writeExe(t, backup, string(stubBytes)) // the backup = a runnable stub copy

	pointExeAt(t, exe)

	version, err := Rollback(context.Background())
	if err != nil {
		t.Fatalf("Rollback: unexpected error: %v", err)
	}
	if !bytes.Contains([]byte(version), []byte("v9.9.9")) {
		t.Errorf("version = %q; want it to contain %q", version, "v9.9.9")
	}
	// The current binary now equals the backup (restored) content.
	if got := readFile(t, exe); got != string(stubBytes) {
		t.Errorf("exe content after rollback = stub bytes? %v; want match (restored)", bytes.Equal([]byte(got), stubBytes))
	}
	// FR-U8 one-step: the backup FILE is CONSUMED (moved into place) — it no longer exists.
	if _, statErr := os.Stat(backup); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("backup should be CONSUMED (gone) after rollback; statErr = %v, want ErrNotExist", statErr)
	}
}

// TestRollback_NoBackup verifies the FR-U8 no-op case: with no backup present, Rollback returns
// ErrNoBackup (so the command layer prints "no backup — nothing to roll back" + exits 0) and leaves
// the current binary byte-for-byte unchanged.
func TestRollback_NoBackup(t *testing.T) {
	// NOT parallel — mutates resolveCurrentExe.
	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "stagecoach")
	writeExe(t, exe, "CURRENT")
	pointExeAt(t, exe)

	version, err := Rollback(context.Background())
	if !errors.Is(err, ErrNoBackup) {
		t.Fatalf("Rollback no-backup: errors.Is(err, ErrNoBackup) = false; err = %v", err)
	}
	if version != "" {
		t.Errorf("version = %q; want empty (no-op)", version)
	}
	// FR-U8: nothing on disk changed.
	if got := readFile(t, exe); got != "CURRENT" {
		t.Errorf("exe content after no-backup = %q, want %q (unchanged)", got, "CURRENT")
	}
}

// TestRollback_BackupUnusable verifies the FR-U8 refuse case: a backup whose --version exits non-zero
// (STAGECOACH_STUBCLI_EXIT=1 ⇒ execVersion returns *ExitError) is refused with ErrBackupUnusable,
// and the sanity-run ran BEFORE platformRollback so BOTH the current binary AND the backup file are
// byte-for-byte unchanged.
func TestRollback_BackupUnusable(t *testing.T) {
	// NOT parallel — mutates resolveCurrentExe.
	stub := buildStubCLI(t)
	// The stub exits non-zero ⇒ execVersion errors ⇒ ErrBackupUnusable. (The DEFAULT execVersion
	// leaves cmd.Env unset ⇒ the child inherits os.Environ incl. this t.Setenv.)
	t.Setenv("STAGECOACH_STUBCLI_EXIT", "1")

	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "stagecoach")
	writeExe(t, exe, "CURRENT")
	backup := exe + ".stagecoach-backup"
	stubBytes, err := os.ReadFile(stub)
	if err != nil {
		t.Fatalf("read stub %s: %v", stub, err)
	}
	writeExe(t, backup, string(stubBytes)) // the backup = a runnable stub copy (that exits 1)

	pointExeAt(t, exe)

	version, err := Rollback(context.Background())
	if !errors.Is(err, ErrBackupUnusable) {
		t.Fatalf("Rollback unusable: errors.Is(err, ErrBackupUnusable) = false; err = %v", err)
	}
	if version != "" {
		t.Errorf("version = %q; want empty (refused)", version)
	}
	// FR-U8 "refused with an explanation": sanity-run BEFORE swap ⇒ current AND backup unchanged.
	if got := readFile(t, exe); got != "CURRENT" {
		t.Errorf("exe content after unusable = %q, want %q (unchanged)", got, "CURRENT")
	}
	if got := readFile(t, backup); got != string(stubBytes) {
		t.Errorf("backup content after unusable = stub bytes? %v; want match (unchanged)", bytes.Equal([]byte(got), stubBytes))
	}
}
