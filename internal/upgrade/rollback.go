// rollback.go implements the FR-U8 `--rollback` primitive (§9.29): restore the most-recent backup
// (the .stagecoach-backup / .exe.old sibling created by the LANDED Swap, P1.M3.T2.S1) over the
// current binary, using the SAME sanity-run discipline as the upgrade (a backup whose --version no
// longer runs is refused with current unchanged) — WITHOUT a backup it is a no-op (ErrNoBackup; the
// command layer prints "no backup — nothing to roll back" and exits 0). ONE-STEP undo (FR-U8): the
// backup is CONSUMED by the restore (moved into place); the previous-current is LOST (not re-backed-up).
//
// CRITICAL DESIGN — Rollback does NOT reuse Swap. Swap's platformSwap backs-up-current FIRST
// (os.Rename(currentExe, backupPath)), so calling Swap with the backup as the "new" binary would
// OVERWRITE the backup with the current content (destroying the restore source) then move it back —
// a net no-op that destroys the backup. Rollback instead moves the backup DIRECTLY into place via
// its own platformRollback twins (matches the contract's "NO new backup-of-current … overwrites
// current with backup, previous-current is lost" decision).
//
// CONSUMES the LANDED same-package primitives (treat as contracts; do NOT re-declare):
//   - resolveCurrentExe (swap.go) — the injectable exe-path seam;
//   - backupPath (swap_unix.go / swap_windows.go) — the STABLE backup-name helper (the suffix is
//     ".stagecoach-backup" on Unix, ".old" on Windows; built FOR this item);
//   - execVersion (stage.go) — the sanity-run seam (called DIRECTLY, NOT via sanityCheck: a backup
//     has no target tag — FR-U8 only requires "whose --version no longer runs is refused").
//
// SCOPE: NO privilege gate. The contract pins 3 cases (absent / unrunnable / restored); and
// privilegeCommand is hardcoded `sudo "<exe>" upgrade` (the upgrade re-run form) — wrong for
// `--rollback`. A non-writable install dir surfaces as a plain os.Rename EACCES (propagated); the
// command layer (P1.M4.T2) may layer sudo guidance later. Rollback NEVER os.Exit / NEVER prints —
// the command layer maps ErrNoBackup → "no backup …" + exit 0, ErrBackupUnusable/swap-err → non-zero.
// Walled off (FR-U12: stdlib-only, no internal/* imports). File comment only — releases.go owns the
// package doc. ZERO production callers after this subtask (the consumer is P1.M4.T2 runUpgrade).
package upgrade

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
)

// ErrNoBackup is returned when there is no prior backup to roll back to (FR-U8 "Without a backup it
// is a no-op reported as such"). The command layer (P1.M4.T2 runUpgrade) maps it to "no backup —
// nothing to roll back" + exit 0; Rollback itself never prints / never os.Exit. The sentinel wraps
// with errors.New("upgrade: …") matching the package convention.
var ErrNoBackup = errors.New("upgrade: no prior backup to roll back to (FR-U8)")

// ErrBackupUnusable is returned when the backup binary's --version fails to run / exits non-zero
// (FR-U8 "a backup whose --version no longer runs is refused with an explanation"). The sanity-run
// runs BEFORE platformRollback so BOTH the current binary AND the backup file are byte-for-byte
// unchanged on this error. The command layer surfaces the wrapped message + exits non-zero.
var ErrBackupUnusable = errors.New("upgrade: backup binary no longer runs (FR-U8)")

// Rollback restores the most-recent backup (backupPath(currentExe) — the .stagecoach-backup / .exe.old
// sibling created by Swap) over the current binary and returns the backup's reported version (the
// trimmed --version output). It is the FR-U8 one-step undo of a bad direct-binary upgrade.
//
// Flow:
//
//  1. check ctx (abort before touching disk — same cancellation semantics as Swap, FR-U11);
//  2. resolve the current exe via the injectable resolveCurrentExe seam (swap.go);
//  3. os.Stat the backup (backupPath(currentExe)); os.IsNotExist ⇒ ErrNoBackup (the no-op case —
//     the command layer prints + exits 0); any other stat error ⇒ wrapped;
//  4. sanity-run the backup via execVersion(ctx, backup) DIRECTLY (NOT stage.go's sanityCheck — a
//     backup has no target tag; FR-U8 only requires "whose --version no longer runs is refused"). A
//     non-nil error ⇒ ErrBackupUnusable (wrapped); current AND backup are unchanged (sanity BEFORE
//     the swap). On success the trimmed output is captured as restoredVersion;
//  5. delegate the platform-specific restore to platformRollback (the build-tagged twin: a single
//     atomic os.Rename on Unix; a 3-step .old-routing rotate on Windows) — the backup is CONSUMED
//     (moved into place); the previous-current is LOST (FR-U8 one-step — NO re-back-up); and
//  6. return (restoredVersion, nil) on success, or a wrapped error on failure (FR-U11 — the twin
//     restored current on a mid-swap rename failure).
//
// Rollback does NOT call Swap (Swap's platformSwap backs-up-current first and would destroy this
// backup — see the file comment), does NOT probe isWritable (scope — a non-writable dir surfaces as
// a rename EACCES), and does NOT os.Exit / does NOT print. ZERO production callers after this
// subtask (the consumer is P1.M4.T2 runUpgrade).
func Rollback(ctx context.Context) (restoredVersion string, err error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	currentExe, err := resolveCurrentExe()
	if err != nil {
		return "", fmt.Errorf("upgrade: resolve current exe: %w", err)
	}
	backup := backupPath(currentExe) // ".stagecoach-backup" (Unix) / ".old" (Windows) — the Swap-created sibling
	if _, err := os.Stat(backup); err != nil {
		if os.IsNotExist(err) {
			return "", ErrNoBackup // FR-U8 no-op → command layer prints "no backup …" + exits 0
		}
		return "", fmt.Errorf("upgrade: stat backup %s: %w", backup, err)
	}
	// Sanity-run the backup DIRECTLY (FR-U8: "whose --version no longer runs is refused"). NOT
	// sanityCheck — that needs a wantTag the backup does not have; the backup's version is whatever
	// it reports. Runs BEFORE platformRollback so current AND backup are unchanged on refusal.
	out, runErr := execVersion(ctx, backup)
	if runErr != nil {
		return "", fmt.Errorf("rollback: backup %s unusable: %w", backup, ErrBackupUnusable)
	}
	if err := platformRollback(currentExe, backup); err != nil {
		// FR-U11: the twin restored current on a mid-swap rename failure (Unix: the single rename
		// left nothing changed; Windows: the ordered dance reversed prior steps).
		return "", fmt.Errorf("upgrade: rollback: %w", err)
	}
	return string(bytes.TrimSpace(out)), nil // the restored binary's reported version
}
