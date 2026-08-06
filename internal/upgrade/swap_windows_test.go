// swap_windows_test.go exercises the Windows .old rename-dance + CleanupOldBinary contract. It
// runs natively on the windows-latest CI matrix entry (.github/workflows/ci.yml) where the
// //go:build windows tag selects it. The rename-dance logic is verified against regular files
// (the running-image lock is a kernel property exercised by the P1.M4.T3.S2 e2e harness; here we
// prove the os.Rename sequence and the CleanupOldBinary .old-sibling delete). NOT parallel: it
// mutates the package-level resolveCurrentExe seam (clone of stage_test.go's constraint).
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

// restoreResolveCurrentExe is the default resolveCurrentExe captured at init; tests that override
// the seam defer restoring it via t.Cleanup so a later test sees the real default again.
var restoreResolveCurrentExe = resolveCurrentExe

// writeExe writes content to path — a fake binary that platformSwap renames around. (No chmod
// needed on Windows; 0o644 is the os.WriteFile default and fine for a regular-file swap test.)
func writeExe(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// readFile reads path (fatally on error) — a tiny helper to assert on swapped/.old contents.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// pointExeAt overrides resolveCurrentExe to report exe and restores the default on cleanup. Every
// test that drives CleanupOldBinary against a temp "exe" must call this (and must NOT be
// t.Parallel()).
func pointExeAt(t *testing.T, exe string) {
	t.Helper()
	resolveCurrentExe = func() (string, error) { return exe, nil }
	t.Cleanup(func() { resolveCurrentExe = restoreResolveCurrentExe })
}

// TestPlatformSwap_OldDance verifies the FR-U7 Windows sequence: running exe → .old (rotate),
// new → exe (move). After a successful swap the exe holds the NEW content and the .old holds the
// OLD content. (Regular files prove the rename order; the running-image lock is verified by the
// windows-latest e2e in P1.M4.T3.S2.)
func TestPlatformSwap_OldDance(t *testing.T) {
	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "stagecoach.exe")
	writeExe(t, exe, "OLD")
	newBin := filepath.Join(tempDir, "new.exe")
	writeExe(t, newBin, "NEW")

	if err := platformSwap(exe, newBin); err != nil {
		t.Fatalf("platformSwap: %v", err)
	}

	if got := readFile(t, exe); got != "NEW" {
		t.Errorf("exe content after swap = %q, want %q", got, "NEW")
	}
	if got := readFile(t, exe+".old"); got != "OLD" {
		t.Errorf(".old content = %q, want %q", got, "OLD")
	}
}

// TestPlatformSwap_RestoresOnSwapFailure verifies FR-U11: if the move-new step fails AFTER the
// rotate-to-.old succeeded, the .old is restored so the exe path is byte-for-byte unchanged. The
// failure is induced by a newBinPath that does not exist (os.Rename fails ENOENT) while the dir
// remains writable so the restore os.Rename(.old, exe) succeeds.
func TestPlatformSwap_RestoresOnSwapFailure(t *testing.T) {
	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "stagecoach.exe")
	writeExe(t, exe, "OLD")

	missingNewBin := filepath.Join(tempDir, "does-not-exist.exe")
	err := platformSwap(exe, missingNewBin)
	if err == nil {
		t.Fatalf("platformSwap: want error from missing newBinPath, got nil")
	}

	if got := readFile(t, exe); got != "OLD" {
		t.Errorf("exe content after restore = %q, want %q (FR-U11)", got, "OLD")
	}
	if _, err := os.Stat(exe + ".old"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf(".old should have been restored (moved back); stat err = %v, want ErrNotExist", err)
	}
}

// TestCleanupOldBinary_DeletesOldSibling verifies FR-U7: CleanupOldBinary best-effort deletes the
// .old sibling of the resolved exe (a .old left by a PREVIOUS launch). After the call the .old is
// gone; a missing .old is also tolerated (no error surfaced).
func TestCleanupOldBinary_DeletesOldSibling(t *testing.T) {
	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "stagecoach.exe")
	writeExe(t, exe, "X")
	old := exe + ".old"
	writeExe(t, old, "STALE")

	pointExeAt(t, exe)
	CleanupOldBinary()

	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Errorf(".old after CleanupOldBinary: stat err = %v, want ErrNotExist (deleted)", err)
	}
}

// TestCleanupOldBinary_NoOldIsNoop verifies CleanupOldBinary tolerates a missing .old (the common
// case on a fresh install or after a prior clean launch) without surfacing an error.
func TestCleanupOldBinary_NoOldIsNoop(t *testing.T) {
	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "stagecoach.exe")
	writeExe(t, exe, "X")
	// No .old present.
	pointExeAt(t, exe)
	CleanupOldBinary() // must not panic / must not fail to return
}

// TestSwap_HappyPathWindows exercises the Swap orchestrator end-to-end on Windows: it resolves the
// exe (seam), probes writability, runs platformSwap (.old dance), and cleans the staging tempDir.
func TestSwap_HappyPathWindows(t *testing.T) {
	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "stagecoach.exe")
	writeExe(t, exe, "OLD")

	newDir, err := os.MkdirTemp("", "stagecoach-swap-test-*")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	newBin := filepath.Join(newDir, "new-stagecoach.exe")
	writeExe(t, newBin, "NEW")

	pointExeAt(t, exe)
	if err := Swap(context.Background(), newBin); err != nil {
		t.Fatalf("Swap: %v", err)
	}

	if got := readFile(t, exe); got != "NEW" {
		t.Errorf("exe content after swap = %q, want %q", got, "NEW")
	}
	if got := readFile(t, exe+".old"); got != "OLD" {
		t.Errorf(".old content = %q, want %q", got, "OLD")
	}
	if _, err := os.Stat(newDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("staging tempDir after success: stat err = %v, want ErrNotExist (cleaned)", err)
	}
}
