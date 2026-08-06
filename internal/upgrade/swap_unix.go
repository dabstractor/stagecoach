//go:build !windows

package upgrade

import (
	"fmt"
	"os"
)

// backupPath returns the one-deep backup path for exe on Unix: exe + ".stagecoach-backup"
// (FR-U5 step 7). It is a SHARED, STABLE helper (NOT an inline literal in platformSwap) because
// P1.M3.T3.S1 (--rollback) restores the backup Swap creates and MUST use the SAME name —
// re-deriving the suffix inline there would desync. The suffix differs from the Windows twin
// (.old) so the helper lives in the build-tagged files, one per platform.
func backupPath(exe string) string {
	return exe + ".stagecoach-backup"
}

// platformSwap performs the FR-U7 Unix atomic swap: back up the current binary to
// backupPath(currentExe) via os.Rename, then os.Rename newBinPath into its place. os.Rename on
// the SAME filesystem is atomic by construction — the running process keeps the old inode open
// (the kernel does not reclaim it until the last fd closes), so the path now resolves to the new
// file and the NEXT invocation runs the new version while THIS process keeps running the old one.
// There is never zero runnable binaries because the swap is a single rename over an existing path.
//
// FR-U8 (one-deep backup): os.Rename(currentExe, backup) REPLACES any prior backup — only one
// generation is kept. FR-U11 (never half-upgraded): if the second rename (new→current) fails, the
// backup is moved back into place (os.Rename(backup, currentExe)) so the installed binary is
// byte-for-byte restored. The restore is best-effort (its error is ignored — the running process's
// inode is safe regardless; surfacing a restore-on-restore failure adds no actionable signal); the
// swap-failure error is annotated "(restored from backup)" so the user knows the rollback ran.
// On success the backup is left on disk for P1.M3.T3.S1 (--rollback) — it is NOT cleaned here.
func platformSwap(currentExe, newBinPath string) error {
	backup := backupPath(currentExe)
	if err := os.Rename(currentExe, backup); err != nil { // one-deep: replaces a prior backup (FR-U8)
		return fmt.Errorf("backup current binary: %w", err)
	}
	if err := os.Rename(newBinPath, currentExe); err != nil {
		_ = os.Rename(backup, currentExe) // FR-U11 restore (running process keeps the old inode safe)
		return fmt.Errorf("rename new binary into place: %w (restored from backup)", err)
	}
	return nil
}

// privilegeCommand returns the FR-U7 "re-run" elevation command for Unix: re-run the WHOLE upgrade
// elevated via sudo. It is NOT a one-shot `sudo install <newBinPath> <exe>` because newBinPath is
// in an EPHEMERAL per-invocation tempDir (os.MkdirTemp) that may be gone by the time the user
// runs the command; re-running elevated re-does detect→…→swap as root and is self-contained.
// exe is quoted via %q so a path with spaces (/opt/my apps/stagecoach) survives a paste.
func privilegeCommand(exe string) string {
	return fmt.Sprintf("sudo %q upgrade", exe)
}

// CleanupOldBinary is a no-op on Unix: the .old deferred-delete dance is Windows-only (a running
// Unix binary is renamed-in-place atomically; there is no locked .old sibling to clean). main.go
// calls it unconditionally at startup; the Windows twin (swap_windows.go) deletes a prior-launch
// .exe.old. FR-U7.
func CleanupOldBinary() {}
