// swap_test.go exercises Swap's Unix contract: happy-path swap + tempDir cleanup, the proactive
// non-writable→NeedsPrivilegesError gate (FR-U4/U11), one-deep backup overwrite (FR-U8),
// restore-on-swap-failure (FR-U11), the NeedsPrivilegesError errors.Is/Command contract, and the
// isWritable probe branches. It is NOT parallel: several tests mutate the package-level
// resolveCurrentExe seam, and two concurrent tests racing on that var would trip the race detector
// (exactly stage_test.go's documented constraint for execVersion). The sibling upgrade tests never
// touch resolveCurrentExe and may stay parallel. Stdlib-only.
//
//go:build !windows

package upgrade

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// restoreResolveCurrentExe is the default resolveCurrentExe captured at init; tests that override
// the seam defer restoring it via t.Cleanup so a later test sees the real default again.
var restoreResolveCurrentExe = resolveCurrentExe

// writeExe writes content to path and chmods it 0o755 — a fake runnable "exe" in a temp dir that
// resolveCurrentExe is pointed at (you cannot os.Rename over the REAL running binary in a unit
// test, hence the seam). 0o755 matches the executability stage.go's extractBinary guarantees.
func writeExe(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
}

// readFile reads path (fatally on error) — a tiny helper to assert on swapped/backup contents.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// pointExeAt overrides resolveCurrentExe to report exe and restores the default on cleanup. Every
// test that drives Swap against a temp "exe" must call this (and must NOT be t.Parallel()).
func pointExeAt(t *testing.T, exe string) {
	t.Helper()
	resolveCurrentExe = func() (string, error) { return exe, nil }
	t.Cleanup(func() { resolveCurrentExe = restoreResolveCurrentExe })
}

// TestSwap_HappyPath verifies the full success contract: the current exe is backed up to
// <exe>.stagecoach-backup (== old content), newBinPath is renamed into place (== new content),
// and the staging tempDir is cleaned (it is os.MkdirTemp-backed so the os.TempDir() guard matches).
func TestSwap_HappyPath(t *testing.T) {
	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "stagecoach")
	writeExe(t, exe, "OLD")

	// newBinPath in an os.MkdirTemp staging dir so the os.TempDir() cleanup guard matches (a
	// t.TempDir dir is NOT under os.TempDir on all hosts, so it would skip removal — wrong here).
	newDir, err := os.MkdirTemp("", "stagecoach-swap-test-*")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	newBin := filepath.Join(newDir, "new-stagecoach")
	writeExe(t, newBin, "NEW")

	pointExeAt(t, exe)
	if err := Swap(context.Background(), newBin); err != nil {
		t.Fatalf("Swap: %v", err)
	}

	if got := readFile(t, exe); got != "NEW" {
		t.Errorf("exe content after swap = %q, want %q", got, "NEW")
	}
	if got := readFile(t, exe+".stagecoach-backup"); got != "OLD" {
		t.Errorf("backup content = %q, want %q", got, "OLD")
	}
	if _, err := os.Stat(newDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("staging tempDir after success: stat err = %v, want ErrNotExist (cleaned)", err)
	}
	if _, err := os.Stat(newBin); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("newBin after success: stat err = %v, want ErrNotExist (cleaned with tempDir)", err)
	}
}

// TestSwap_NotWritableNeedsPrivileges verifies FR-U4/U11: a non-writable install dir is detected
// BEFORE any rename, Swap returns *NeedsPrivilegesError (errors.Is(ErrNeedsPrivileges) true), the
// .Command contains sudo + the exe path, and NOTHING on disk changed (no backup, exe unchanged).
// Skipped as root (os.Geteuid()==0) because root bypasses Unix permission bits — the probe would
// succeed and the test would be meaningless.
func TestSwap_NotWritableNeedsPrivileges(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission test meaningless as root")
	}
	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "stagecoach")
	writeExe(t, exe, "OLD")
	newBin := filepath.Join(tempDir, "new-stagecoach")
	writeExe(t, newBin, "NEW")

	// Make the install dir read+execute but NOT writable (0o500) so the CreateTemp probe fails.
	// Restore perms in t.Cleanup so t.TempDir's RemoveAll doesn't fail on the 0500 dir.
	if err := os.Chmod(tempDir, 0o500); err != nil {
		t.Fatalf("chmod dir 0500: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(tempDir, 0o700) })

	pointExeAt(t, exe)
	err := Swap(context.Background(), newBin)

	var npe *NeedsPrivilegesError
	if !errors.As(err, &npe) {
		t.Fatalf("Swap err = %v, want *NeedsPrivilegesError", err)
	}
	if !errors.Is(err, ErrNeedsPrivileges) {
		t.Errorf("errors.Is(err, ErrNeedsPrivileges) = false, want true")
	}
	if !strings.Contains(npe.Command, "sudo") {
		t.Errorf("Command %q does not contain %q", npe.Command, "sudo")
	}
	if !strings.Contains(npe.Command, exe) {
		t.Errorf("Command %q does not contain exe path %q", npe.Command, exe)
	}

	// FR-U11: nothing on disk changed.
	if got := readFile(t, exe); got != "OLD" {
		t.Errorf("exe content after non-writable = %q, want %q (unchanged)", got, "OLD")
	}
	if _, err := os.Stat(exe + ".stagecoach-backup"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("backup should NOT exist (no rename ran); stat err = %v, want ErrNotExist", err)
	}
	if _, err := os.Stat(newBin); err != nil {
		t.Errorf("newBin should still exist (no rename ran); stat err = %v", err)
	}
}

// TestSwap_OverwritesPriorBackup verifies FR-U8 (one-deep backup): a pre-existing
// .stagecoach-backup from a previous run is REPLACED by this swap's backup (== current OLD
// content), not appended or merged.
func TestSwap_OverwritesPriorBackup(t *testing.T) {
	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "stagecoach")
	writeExe(t, exe, "OLD")
	// A stale prior backup from a previous upgrade — one generation ago.
	priorBackup := exe + ".stagecoach-backup"
	writeExe(t, priorBackup, "ANCIENT")

	newDir, err := os.MkdirTemp("", "stagecoach-swap-test-*")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	newBin := filepath.Join(newDir, "new-stagecoach")
	writeExe(t, newBin, "NEW")

	pointExeAt(t, exe)
	if err := Swap(context.Background(), newBin); err != nil {
		t.Fatalf("Swap: %v", err)
	}

	if got := readFile(t, priorBackup); got != "OLD" {
		t.Errorf("backup content = %q, want %q (prior ANNCIENT replaced — FR-U8 one-deep)", got, "OLD")
	}
	if got := readFile(t, exe); got != "NEW" {
		t.Errorf("exe content after swap = %q, want %q", got, "NEW")
	}
}

// TestSwap_RestoresOnSwapFailure verifies FR-U11 (never half-upgraded): if the second rename
// (new→current) fails AFTER the backup succeeded, the backup is restored (os.Rename(backup→current))
// so the installed binary is byte-for-byte unchanged. The failure is induced by making newBinPath
// NOT EXIST for the swap step — os.Rename(nonexistent, current) fails with a no-such-file error,
// but the dir is still writable so os.Rename(backup, current) restore SUCCEEDS. This is the only
// deterministic, root-safe way to make the swap rename fail while the restore rename succeeds
// (making the dir read-only would fail BOTH renames).
func TestSwap_RestoresOnSwapFailure(t *testing.T) {
	tempDir := t.TempDir()
	exe := filepath.Join(tempDir, "stagecoach")
	writeExe(t, exe, "OLD")
	pointExeAt(t, exe)

	// A newBinPath that does not exist: os.Rename(current→backup) succeeds (dir writable), then
	// os.Rename(newBinPath, current) fails with ENOENT, triggering the restore branch.
	missingNewBin := filepath.Join(tempDir, "does-not-exist")
	err := Swap(context.Background(), missingNewBin)
	if err == nil {
		t.Fatalf("Swap: want error from missing newBinPath, got nil")
	}
	if !strings.Contains(err.Error(), "restored from backup") {
		t.Errorf("Swap err %q does not mention restore", err.Error())
	}

	// FR-U11: exe restored to OLD content (byte-for-byte unchanged).
	if got := readFile(t, exe); got != "OLD" {
		t.Errorf("exe content after restore = %q, want %q (FR-U11)", got, "OLD")
	}
	// The backup was moved BACK into place by the restore, so it no longer exists at backupPath.
	if _, err := os.Stat(exe + ".stagecoach-backup"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("backup should have been restored (moved back); stat err = %v, want ErrNotExist", err)
	}
}

// TestNeedsPrivilegesError_ErrorsIs verifies the typed-error-with-Unwrap contract cloned from
// generate.CASError: errors.Is(*NeedsPrivilegesError, ErrNeedsPrivileges) is true AND the .Command
// field is accessible via a direct field read (no type-assert needed in-package).
func TestNeedsPrivilegesError_ErrorsIs(t *testing.T) {
	cmd := `sudo "/x/stagecoach" upgrade`
	err := &NeedsPrivilegesError{Command: cmd}
	if !errors.Is(err, ErrNeedsPrivileges) {
		t.Errorf("errors.Is(*NeedsPrivilegesError, ErrNeedsPrivileges) = false, want true")
	}
	if err.Command != cmd {
		t.Errorf("Command = %q, want %q", err.Command, cmd)
	}
	if !strings.Contains(err.Error(), ErrNeedsPrivileges.Error()) {
		t.Errorf("Error() %q does not contain the sentinel text", err.Error())
	}
	if !strings.Contains(err.Error(), cmd) {
		t.Errorf("Error() %q does not contain the command", err.Error())
	}
}

// TestIsWritable covers the isWritable probe branches: a writable temp dir → true, a 0o500 dir →
// false (skipped as root — root bypasses Unix permission bits), and a non-existent dir → false.
func TestIsWritable(t *testing.T) {
	// Writable temp dir → true.
	dir := t.TempDir()
	if !isWritable(dir) {
		t.Errorf("isWritable(tempDir) = false, want true")
	}

	// Read+execute but not writable (0o500) → false. Skipped as root.
	if os.Geteuid() != 0 {
		roDir := t.TempDir()
		if err := os.Chmod(roDir, 0o500); err != nil {
			t.Fatalf("chmod 0500: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(roDir, 0o700) })
		if isWritable(roDir) {
			t.Errorf("isWritable(0500 dir) = true, want false")
		}
	}

	// Non-existent dir → false.
	if isWritable(filepath.Join(t.TempDir(), "nope")) {
		t.Errorf("isWritable(non-existent) = true, want false")
	}
}
