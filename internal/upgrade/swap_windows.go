//go:build windows

package upgrade

import (
	"fmt"
	"os"
)

// backupPath returns the one-deep backup path for exe on Windows: exe + ".old" (FR-U7). It is a
// SHARED, STABLE helper (NOT an inline literal in platformSwap) because P1.M3.T3.S1 (--rollback)
// restores the backup Swap creates and MUST use the SAME name — re-deriving the suffix inline
// there would desync. The suffix differs from the Unix twin (.stagecoach-backup) so the helper
// lives in the build-tagged files, one per platform.
func backupPath(exe string) string {
	return exe + ".old"
}

// platformSwap performs the FR-U7 Windows .old deferred-delete dance: os.Rename the running binary
// to backupPath(currentExe) (currentExe + ".old"), then os.Rename newBinPath into its place. The
// dance exists because on Windows the RUNNING .exe is LOCKED — it cannot be overwritten in place,
// but it CAN be renamed (the kernel holds the image section open against the file, not the path).
// So: rotate running→.old (allowed; the path is now free), then move new→exe. Go's os.Rename uses
// MoveFileEx(MOVEFILE_REPLACE_EXISTING) so the second move replaces any leftover at currentExe.
// There is never zero runnable binaries: the new binary already passed S2's sanity-run, and the
// .old (this process's own image) remains runnable until the process exits.
//
// FR-U8 (one-deep): os.Rename(currentExe, old) REPLACES any prior .old — only one generation is
// kept. FR-U11 (never half-upgraded): if the second rename (new→current) fails, the .old is moved
// back into place (os.Rename(old, currentExe)) so the installed binary is byte-for-byte restored
// (the running image, which had been set aside, returns to its path). The restore is best-effort
// (its error is ignored — surfacing a restore-on-restore failure adds no actionable signal); the
// swap-failure error is annotated "(restored from .old)" so the user knows the rollback ran.
//
// The .old is NOT deleted here: it is this process's own renamed image and is STILL LOCKED while
// this process runs (os.Remove would fail). CleanupOldBinary (called at the NEXT launch) deletes
// a .old left by a PREVIOUS launch. FR-U7.
func platformSwap(currentExe, newBinPath string) error {
	old := backupPath(currentExe)
	if err := os.Rename(currentExe, old); err != nil { // Windows: renaming a running image is allowed
		return fmt.Errorf("rotate current binary to .old: %w", err)
	}
	if err := os.Rename(newBinPath, currentExe); err != nil {
		_ = os.Rename(old, currentExe) // FR-U11 restore (move the running image back to its path)
		return fmt.Errorf("move new binary into place: %w (restored from .old)", err)
	}
	return nil // the .old is cleaned at the NEXT launch (CleanupOldBinary); locked while this runs
}

// privilegeCommand returns a Windows elevation hint for the install path. Most Windows
// self-installs are per-user (%USERPROFILE%\… or a Scoop shim) and the .old dance works without
// elevation; a system-wide C:\Program Files install needs elevation. The message names the exe so
// the user knows what to re-run; the command layer may refine the framing (e.g. a UAC prompt) but
// echoes this verbatim by default. Non-empty per the contract.
func privilegeCommand(exe string) string {
	return fmt.Sprintf("re-run with administrator privileges: %q upgrade", exe)
}

// CleanupOldBinary best-effort deletes the .old sibling of the resolved exe at startup (FR-U7).
// The .old created by THIS run's swap is still locked (it IS this process's image, renamed), so it
// CANNOT be deleted now — CleanupOldBinary instead deletes a .old left by the PREVIOUS launch
// (which is no longer locked because that process has exited). The os.Remove error is ignored
// (best-effort): a missing .old or a transient lock failure is expected and harmless. main.go
// calls this once at startup, unconditionally; the Unix twin (swap_unix.go) is a no-op.
func CleanupOldBinary() {
	exe, err := resolveCurrentExe()
	if err != nil {
		return // best-effort — nothing to clean if we cannot resolve our own path
	}
	_ = os.Remove(backupPath(exe))
}
