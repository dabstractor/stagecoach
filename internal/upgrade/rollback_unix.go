//go:build !windows

// rollback_unix.go is the Unix platformRollback twin for FR-U8 `--rollback` (see rollback.go for the
// shared orchestrator). A SINGLE atomic os.Rename(backup, currentExe) replaces the running binary's
// directory entry — the running process keeps its OLD inode open (the kernel does not reclaim it
// until the last fd closes), so THIS process keeps running the now-overwritten binary while the NEXT
// invocation runs the restored (prior) version. Never zero runnable binaries.
//
// FR-U8 one-step semantics: os.Rename MOVES the backup into place — the backup FILE ceases to exist
// (its content is now at currentExe); the previous-current is LOST (not re-backed-up). After rollback
// there is NO backup to roll back to again.
//
// This is DELIBERATELY NOT platformSwap (swap_unix.go), which backs-up-current FIRST
// (os.Rename(currentExe, backupPath)) — calling that here would OVERWRITE the backup with the current
// content (destroying the restore source) then move it back, a net no-op that destroys the backup.
// platformRollback moves the backup DIRECTLY into place (consuming it) — the opposite ordering.
//
// backup and currentExe are siblings in the install dir ⇒ same filesystem ⇒ rename(2) succeeds
// (rename does not cross filesystems). A non-writable install dir surfaces as a rename EACCES
// (propagated) — Rollback does NOT probe isWritable (scope; privilegeCommand is wrong for --rollback).

package upgrade

import (
	"fmt"
	"os"
)

// platformRollback atomically replaces currentExe with backup via a single os.Rename. On Unix this
// is the whole restore: the backup is CONSUMED (moved into place), the running process keeps its
// old inode safe, and the next invocation runs the restored version. FR-U8 one-step: no
// re-back-up of the previous-current (the backup FILE is gone after the rename). FR-U11 on failure:
// a failed rename changes nothing on disk (the backup and current are both intact).
func platformRollback(currentExe, backup string) error {
	if err := os.Rename(backup, currentExe); err != nil { // atomic replace; backup consumed; running inode safe
		return fmt.Errorf("restore backup into place: %w", err)
	}
	return nil
}
