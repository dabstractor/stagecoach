// swap.go implements the FR-U5 step 7 + FR-U7 on-disk swap step of the direct-binary
// self-update: given P1.M3.T1.S2's verified + sanity-run newBinPath (tempDir/new-stagecoach),
// it backs up the running binary and atomically renames newBinPath into its place. The
// per-platform mechanics live in the build-tagged twins swap_unix.go (single os.Rename over
// the running inode) and swap_windows.go (the .old deferred-delete dance) — this file is the
// shared orchestrator + the proactive writability gate (FR-U11: detect BEFORE any rename) +
// the typed NeedsPrivilegesError (FR-U4: never auto-elevate — RETURN the command for the
// command layer to print + exit 0) + the staging tempDir cleanup on success (S2's contract:
// my item cleans tempDir on success, leaves it on failure). Walled off (FR-U12: stdlib-only,
// no internal/* imports). File comment only — releases.go owns the package doc.
package upgrade

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrNeedsPrivileges is returned (wrapped in *NeedsPrivilegesError) when the install directory
// is not writable by the current user (e.g. /usr/local/bin root-owned). FR-U4 forbids
// auto-elevation: Swap leaves everything byte-for-byte unchanged and returns this error so the
// command layer can print the exact sudo/re-run command and exit 0. errors.Is(err,
// ErrNeedsPrivileges) is true via NeedsPrivilegesError.Unwrap.
var ErrNeedsPrivileges = errors.New("upgrade: install path not writable; re-run with privileges (FR-U4)")

// NeedsPrivilegesError carries the exact privilege-elevation command for the command layer to
// print (FR-U4 + FR-U7 "re-run" form, e.g. `sudo "/usr/local/bin/stagecoach" upgrade`). It is
// the typed-error-with-Unwrap shape cloned from generate.CASError: errors.Is(err,
// ErrNeedsPrivileges) works AND the command layer type-asserts to read .Command:
//
//	var npe *upgrade.NeedsPrivilegesError
//	if errors.As(err, &npe) { fmt.Println(npe.Command); os.Exit(0) }
//
// Swap returns *NeedsPrivilegesError (never os.Exit); the command layer (P1.M4.T2 runUpgrade)
// prints .Command and exits 0 (exitcode.Success) — so Swap needs NO exitcode mapping.
type NeedsPrivilegesError struct {
	// Command is the ready-to-paste elevation command. On Unix it is the FR-U7 "re-run" form
	// (sudo "<exe>" upgrade) — robust vs the ephemeral staging tempDir; on Windows an elevation
	// hint. The command layer may refine the framing but echoes this verbatim by default.
	Command string
}

// Error reports the privilege requirement with the command to re-run. Satisfies the error
// interface; the leading text mirrors ErrNeedsPrivileges so a bare %v is self-describing.
func (e *NeedsPrivilegesError) Error() string {
	return fmt.Sprintf("%s: %s", ErrNeedsPrivileges, e.Command)
}

// Unwrap returns ErrNeedsPrivileges so errors.Is(err, ErrNeedsPrivileges) is true while the
// concrete .Command field remains accessible via errors.As (the CASError idiom).
func (e *NeedsPrivilegesError) Unwrap() error { return ErrNeedsPrivileges }

// resolveCurrentExe is the injectable exe-path seam. The default resolves the running binary via
// os.Executable and canonicalizes it with filepath.EvalSymlinks (macOS may report a /private/var
// symlink; Homebrew installs are symlinks into the Cellar), tolerating EvalSymlinks failure by
// falling back to the raw os.Executable result (detect.go:352-354 idiom). CLONE of stage.go's
// execVersion seam: tests override this package-level var to point Swap at a temp-dir "exe" (a
// regular file) because you cannot os.Rename over a real running binary in a unit test.
// PACKAGE-LEVEL ⇒ swap_test.go must NOT call t.Parallel() (two concurrent tests would race on
// the seam — exactly stage_test.go's documented constraint for execVersion).
var resolveCurrentExe = func() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("os.Executable: %w", err)
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		return real, nil
	}
	return exe, nil // tolerate EvalSymlinks failure (detect.go:352-354 fallback idiom)
}

// Swap performs the FR-U5 step 7 + FR-U7 on-disk swap of the running binary for the verified
// newBinPath produced by StageNewBinary (P1.M3.T1.S2). It is the LAST step of the direct-binary
// self-update and the ONLY one that mutates the installed binary on disk. It TRUSTS newBinPath
// (already SHA256-verified + sanity-run by S2) — it does NOT re-download, re-verify, re-extract,
// or re-sanity-run.
//
// The flow:
//
//  1. resolve the current exe via the injectable resolveCurrentExe seam;
//  2. PROACTIVELY probe the install dir is writable (FR-U11: BEFORE any rename). If not, leave
//     everything untouched and return *NeedsPrivilegesError carrying the sudo/re-run command
//     (FR-U4: never auto-elevate — the command layer prints .Command and exits 0);
//  3. delegate the platform-specific backup+rename (+ restore-on-failure) to platformSwap (the
//     build-tagged twin: swap_unix.go on Unix, swap_windows.go on Windows); and
//  4. on success, best-effort clean the staging tempDir (filepath.Dir(newBinPath)) GUARDED by
//     an os.TempDir() prefix check (S2's tempDir is os.MkdirTemp, always under os.TempDir; the
//     guard prevents nuking an arbitrary dir if a caller ever passes a newBinPath outside temp).
//
// On ANY failure (non-writable, backup/rename error) NOTHING is left half-upgraded: the proactive
// gate runs before any rename, and platformSwap restores from the backup on a mid-swap rename
// failure (FR-U11). On failure the staging tempDir is LEFT for inspection (S2's contract); on
// success it is cleaned.
//
// Swap NEVER calls os.Exit (FR-U4 — exit 0 for a printed command is the command layer's job).
// ZERO production callers after this subtask (the consumer is P1.M4.T2 runUpgrade).
//
// ctx is checked for cancellation up front (the synchronous filesystem ops themselves don't take
// it); the cancellation semantics are "abort before we touch disk", matching the FR-U11 invariant.
func Swap(ctx context.Context, newBinPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	currentExe, err := resolveCurrentExe()
	if err != nil {
		return fmt.Errorf("upgrade: resolve current exe: %w", err)
	}
	if !isWritable(filepath.Dir(currentExe)) {
		// FR-U4/U11: install path not writable. Leave everything byte-for-byte unchanged and
		// return the typed error so the command layer can print .Command and exit 0.
		return &NeedsPrivilegesError{Command: privilegeCommand(currentExe)}
	}
	if err := platformSwap(currentExe, newBinPath); err != nil {
		return fmt.Errorf("upgrade: swap: %w", err)
	}
	// Success: best-effort staging tempDir cleanup (S2's contract). GUARDED by an os.TempDir()
	// prefix check so a newBinPath outside temp never nukes an arbitrary dir. On failure the
	// tempDir is LEFT for inspection (never cleaned).
	if dir := filepath.Dir(newBinPath); isTempDir(dir) {
		_ = os.RemoveAll(dir)
	}
	return nil
}

// isWritable reports whether a file can be created in dir by the current user — a CreateTemp
// probe that creates + closes + removes a sentinel file. This is the FR-U11 proactive gate: it
// catches a root-owned /usr/local/bin or a read-only install BEFORE any rename so Swap can return
// the clean NeedsPrivilegesError path instead of a confusing mid-flow EACCES. A non-existent or
// otherwise unusable dir returns false (the probe's CreateTemp fails).
func isWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".stagecoach-writetest-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

// isTempDir reports whether dir is at or under the OS temp directory (os.TempDir). It is the guard
// around the success-path staging tempDir cleanup: S2's tempDir is os.MkdirTemp (always under
// os.TempDir) so this matches in production; the guard prevents os.RemoveAll from nuking an
// arbitrary dir if a caller ever passes a newBinPath outside temp. Both paths are canonicalized
// via filepath.Abs before the prefix check so a relative dir or a trailing-slash variant matches.
func isTempDir(dir string) bool {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	tmp, err := filepath.Abs(os.TempDir())
	if err != nil {
		return false
	}
	return strings.HasPrefix(abs, tmp+string(filepath.Separator))
}
