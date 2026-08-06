//go:build windows

// rollback_windows.go is the Windows platformRollback twin for FR-U8 `--rollback` (see rollback.go
// for the shared orchestrator). On Windows the RUNNING .exe is LOCKED — it CANNOT be overwritten in
// place, but it CAN be renamed (the kernel holds the image section open against the file, not the
// path). So a 2-file restore (backup↔current) needs a 3rd scratch name. The dance ROUTES the
// prior-current through .old (backupPath on Windows) so the EXISTING CleanupOldBinary (swap_windows.go)
// reclaims it at the NEXT launch — no new cleanup suffix, no litter.
//
// The 3-step rotate:
//  1. stash the backup content at a fresh `aside` scratch name (os.CreateTemp in the install dir,
//     close+remove to free the name), freeing the .old path;
//  2. os.Rename(currentExe → .old): the running image's rename IS allowed; currentExe is now free;
//  3. os.Rename(aside → currentExe): the backup content is moved into place.
//
// Final state: currentExe = the restored (backup) content; .old = the prior-current (the now-unwanted
// new binary) → CleanupOldBinary deletes it at the next launch. The `aside` scratch name is consumed
// by step 3 (no litter). FR-U11 restore-on-failure: each step's failure reverses the prior steps
// (best-effort) so the path always resolves to a runnable image.
//
// This is DELIBERATELY NOT platformSwap (swap_windows.go), which backs-up-current FIRST
// (os.Rename(currentExe, .old)) — calling that here would OVERWRITE the backup (.old) with the current
// content (destroying the restore source) then move it back, a net no-op that destroys the backup.
// platformRollback moves the backup DIRECTLY into place (consuming it) — the opposite ordering.
//
// `backup` IS currentExe+".old" (backupPath on Windows); Rollback derived it that way. Step 2's
// os.Rename(currentExe, backup) therefore targets the .old path freed in step 1 — correct.

package upgrade

import (
	"fmt"
	"os"
	"path/filepath"
)

// platformRollback restores backup over currentExe via the 3-step .old-routing rotate described
// above. currentExe ends as the backup content; .old ends as the prior-current (reclaimed by
// CleanupOldBinary at the next launch); the aside scratch name is consumed. FR-U8 one-step: the
// backup is CONSUMED (its content is now at currentExe); the previous-current is routed through
// .old (not preserved as a new backup). FR-U11: on a mid-dance failure the prior steps are reversed
// (best-effort) so the path always resolves to a runnable image.
func platformRollback(currentExe, backup string) error {
	dir := filepath.Dir(currentExe)
	tf, err := os.CreateTemp(dir, ".stagecoach-rbk-*") // a fresh scratch name in the install dir
	if err != nil {
		return fmt.Errorf("create rollback aside: %w", err)
	}
	aside := tf.Name()
	_ = tf.Close()
	_ = os.Remove(aside) // free the scratch name so os.Rename can target it

	// 1. Stash the backup content at `aside`; free the .old name.
	if err := os.Rename(backup, aside); err != nil {
		return fmt.Errorf("move backup aside: %w", err)
	}
	// 2. Move the locked running binary to .old (rename of a running image IS allowed; frees current).
	//    backup == currentExe + ".old", freed in step 1.
	if err := os.Rename(currentExe, backup); err != nil {
		_ = os.Rename(aside, backup) // FR-U11: restore the backup to .old so nothing is lost
		return fmt.Errorf("move current aside to .old: %w (backup restored)", err)
	}
	// 3. Move the backup content into place.
	if err := os.Rename(aside, currentExe); err != nil {
		_ = os.Rename(backup, currentExe) // FR-U11: restore the running binary to its path
		_ = os.Rename(aside, backup)      // FR-U11: restore the backup to .old
		return fmt.Errorf("restore backup into place: %w (current restored)", err)
	}
	return nil // currentExe = backup content; .old = prior-current (CleanupOldBinary reclaims next launch)
}
