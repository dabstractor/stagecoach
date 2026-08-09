// Package cmd: upgrade_safety_test.go is the FR-U1/U8/U11 safety suite (P1.M4.T3.S3) — the THIRD of
// P1.M4.T3's three test files.
//
// It drives runUpgrade end-to-end via Execute(ctx) with the REAL dispatchUpgrade + runDirectSwap +
// runDelegate branches (LANDED P1.M4.T2.S1), proving the SAFETY contract of `stagecoach upgrade`
// against an httptest fake + injected Detect/ExecRunner/Swap/Rollback seams:
//   - (a) a tampered archive (a WRONG SHA256 in /checksums vs the served archive bytes) aborts
//     INSIDE StageNewBinary's VerifySHA256 (⇒ ErrChecksumMismatch) before any confirm/swap; the
//     on-disk binary is byte-for-byte UNCHANGED, NO backup is created, and a stagecoach-upgrade-*
//     staging tempDir is LEFT for inspection (FR-U11). exit 1.
//   - (b) an archive whose embedded binary prints the WRONG tag (v0.3.0 under a v0.2.0 release) is
//     refused by StageNewBinary's sanityCheck (⇒ ErrSanityVersionMismatch) — same outcome as (a)
//     (unchanged, no backup, temp left, exit 1) because it ALSO fails before confirm/swap (FR-U5/U11).
//   - (c) a detected manager-owned install (brew) is DELEGATED — NEVER self-swapped — recording
//     'brew upgrade stagecoach' via the injected recordingExecRunner (a upgrade.ExecRunner fake) and
//     printing 'stagecoach updated via brew'; the on-disk binary is untouched; exit 0. The delegate
//     path runs with NO httptest fake (upgradeBaseURL left at its "" default) — the strongest proof
//     it never networks (FR-U1). upgradeSwap is wired to a t.Fatal closure (failingSwap) that PROVES
//     the delegation path never swaps.
//   - (d) --force WITH Detect→ChannelBrew overrides the manager-owned install: dispatchUpgrade prints
//     the FR-U1 warning to stderr ("--force overriding a detected brew install …; self-swapping") AND
//     routes to runDirectSwap; the mini-swap runs (real swap happens) → installed exe → v0.2.0,
//     backup → v0.1.0 (FR-U8 one-deep); exit 0.
//   - (e) --rollback: (e1) with NO backup → upgradeRollback returns upgrade.ErrNoBackup →
//     "no backup — nothing to roll back" + exit 0 (a no-op, never an error); (e2) WITH a backup →
//     upgradeRollback (miniRollback) restores the backup over the installed exe → "restored
//     stagecoach v0.1.0" + the installed exe now reports v0.1.0; exit 0 (FR-U8).
//
// REUSES S2's REUSABLE helpers (buildStubVersion/packSwapArchive/newSwapFake/runVersion/
// setupSwapSeams/runUpgradeSwap/exeSuffix/backupSuffix/hostAssetName/hostEntryName/checksumsName —
// ALL in upgrade_swap_test.go, SAME package cmd; redefining them is a compile error) + cmd/stubversion
// (S2, LANDED — built here a third time at v0.3.0 for case (b)). ADDS only: recordingExecRunner
// (upgrade.ExecRunner fake), runUpgradeArgs (a parameterized Execute driver), miniSwap/failingSwap/
// miniRollback closures, and the five test functions.
//
// NO real network, NO real package-manager subprocess, NO rename of the real running test binary
// (FR-U12). (a)/(b)/(d) hit the localhost fake (upgradeBaseURL=ts.URL); (c)/(e) use NO fake at all.
// The swap/rollback closures target a temp stub (NEVER os.Executable/resolveCurrentExe).
//
// Dedicated file: S1 owns upgrade_check_test.go (the --check suite); S2 owns upgrade_swap_test.go
// (the direct-swap happy path); S3 owns this. Zero production edits — this file only READS the
// LANDED dispatchUpgrade + runDirectSwap/runDelegate + the seams in upgrade.go/upgrade_run.go.
package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/dabstractor/stagecoach/internal/exitcode"
	"github.com/dabstractor/stagecoach/internal/upgrade"
)

// recordingExecRunner is the upgrade.ExecRunner fake for the delegation case (c). It records every
// argv it is asked to run (mutex-guarded for -race) and returns a canned (code, err) so runDelegate
// maps it without shelling out to a REAL package manager. The signature is the STREAMING
// upgrade.ExecRunner.Run (Run(ctx, stdout, stderr io.Writer, name string, args ...) (int, error)) —
// DISTINCT from detect.go's capturing Runner.Run(ctx, name, args...) (string, int, error). A signature
// mismatch here would silently fall back to the prod osExecRunner (a REAL brew) and exit 1 on CI.
type recordingExecRunner struct {
	mu    sync.Mutex
	calls [][]string
	code  int
	err   error
}

// Run implements upgrade.ExecRunner: it records append([]string{name}, args...) and returns the
// canned (code, err). The stdout/stderr writers are accepted but ignored (the recorder proves the
// argv; the success message is produced by runDelegate, not by the runner). ctx is unused.
func (r *recordingExecRunner) Run(_ context.Context, _, _ io.Writer, name string, args ...string) (int, error) {
	r.mu.Lock()
	r.calls = append(r.calls, append([]string{name}, args...))
	r.mu.Unlock()
	return r.code, r.err
}

// joinedCalls returns the recorded argvs as a single human-readable string (each argv space-joined,
// multi-step commands " && "-joined — mirrors Delegate's joinArgv). For brew (a single argv) the
// result is exactly "brew upgrade stagecoach".
func (r *recordingExecRunner) joinedCalls() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	parts := make([]string, 0, len(r.calls))
	for _, c := range r.calls {
		parts = append(parts, strings.Join(c, " "))
	}
	return strings.Join(parts, " && ")
}

// runUpgradeArgs drives `stagecoach upgrade <args...>` end-to-end via Execute(ctx), mirroring S2's
// runUpgradeSwap body but taking the args slice so it covers ["upgrade","--yes"],
// ["upgrade","--yes","--force"], and ["upgrade","--rollback"]. saveRootState/restoreRootState +
// resetFlags(upgradeCmd.Flags()) (the latter is SEPARATE from restoreRootState — it resets the
// upgradeCmd LOCAL flags flagYes/flagForce/flagRollback/flagChannel that restoreRootState does NOT
// touch), isolated HOME (t.TempDir + XDG_CONFIG_HOME="" so LoadUpgradeConfig ⇒ Defaults with no real
// config and no bootstrap), and outBuf/errBuf wired to rootCmd. Execute returns runUpgrade's error
// (cobra SilenceErrors=true ⇒ returned, not printed); the caller maps it via exitcode.For.
func runUpgradeArgs(t *testing.T, args ...string) (outBuf, errBuf *bytes.Buffer, err error) {
	t.Helper()
	_, origOut, origErr, origRunE := saveRootState(t)
	t.Cleanup(func() { restoreRootState(t, nil, origOut, origErr, origRunE) })
	t.Cleanup(func() { resetFlags(upgradeCmd.Flags()) }) // SEPARATE from restoreRootState
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "") // LoadUpgradeConfig ⇒ Defaults (no global config, no bootstrap)
	outBuf, errBuf = &bytes.Buffer{}, &bytes.Buffer{}
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs(append([]string{"upgrade"}, args...))
	err = Execute(context.Background())
	return outBuf, errBuf, err
}

// miniSwap returns the backup→rename→cleanup closure for case (d): a FAITHFUL twin of S2's inline
// mini-swap (and upgrade.Swap's success path). It backs up the installedExe (one-deep, FR-U8),
// atomically renames the staged new binary into place, restores from the backup on a mid-swap rename
// failure (best-effort, FR-U11), and cleans the staging tempDir on success. It NEVER calls
// os.Executable/resolveCurrentExe (the real running `go test` binary is safe) — it operates solely on
// the captured temp installedExe. Reused for (d) (the --force self-swap); the swap is wired to
// failingSwap for (c) and is never reached for (a)/(b).
func miniSwap(installedExe string) func(context.Context, string) error {
	return func(ctx context.Context, newBinPath string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		backup := installedExe + backupSuffix()
		if err := os.Rename(installedExe, backup); err != nil { // one-deep backup (FR-U8)
			return fmt.Errorf("backup current binary: %w", err)
		}
		if err := os.Rename(newBinPath, installedExe); err != nil { // atomic rename into place
			_ = os.Rename(backup, installedExe) // FR-U11 restore (best-effort, like platformSwap)
			return fmt.Errorf("rename new binary into place: %w (restored from backup)", err)
		}
		_ = os.RemoveAll(filepath.Dir(newBinPath)) // staging tempDir cleanup (isTempDir-by-construction)
		return nil
	}
}

// failingSwap returns a closure that t.Fatal's — it PROVES case (c) never swaps. If dispatchUpgrade
// (buggily) routed a detected brew install to runDirectSwap, the test fails loudly instead of
// renaming the test runner. The delegation path (FR-U1) must call runDelegate, not upgradeSwap.
func failingSwap(t *testing.T) func(context.Context, string) error {
	t.Helper()
	return func(context.Context, string) error {
		t.Fatal("upgradeSwap must not be called on the delegation path (FR-U1: manager-owned binaries are delegated, not swapped)")
		return nil
	}
}

// miniRollback returns the rollback closure for case (e2): a faithful twin of upgrade.Rollback's
// contract operating on a temp exe (NEVER the real test binary via resolveCurrentExe). It stats the
// backup (os.IsNotExist ⇒ upgrade.ErrNoBackup — the (e1) no-op sentinel), renames the backup over the
// installed exe (consuming the backup; the previous-current is lost — FR-U8 one-step), and returns the
// restored binary's trimmed --version (the dispatch layer prints "restored stagecoach %s"). The REAL
// upgrade.Rollback mechanics are exhaustively covered by rollback_test.go; S3 owns the --rollback
// DISPATCH path + the seam.
func miniRollback(installedExe string) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		backup := installedExe + backupSuffix()
		if _, err := os.Stat(backup); err != nil {
			if os.IsNotExist(err) {
				return "", upgrade.ErrNoBackup // FR-U8 no-op → dispatch layer prints "no backup …" + exit 0
			}
			return "", err
		}
		if err := os.Rename(backup, installedExe); err != nil { // FR-U8 one-step: backup CONSUMED, prev-current LOST
			return "", err
		}
		out, err := exec.Command(installedExe, "--version").Output() // the restored binary's reported version
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}
}

// mustRead reads path's bytes (t.Fatal on error). Used by (a)/(b)/(c) for the byte-for-byte
// "installed exe unchanged" assertion (FR-U11) — bytes.Equal before==after, NOT --version (a
// tampered payload never reaches the installed exe, so its version is trivially the same; bytes.Equal
// is the FR-U11 invariant).
func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

// TestUpgradeFailure_TamperedArchive is FR-U11 case (a): a served archive whose /checksums digest
// does NOT match the archive bytes (a WRONG but valid-hex sha) ⇒ VerifySHA256 ⇒ ErrChecksumMismatch
// INSIDE StageNewBinary (before confirm/swap). The on-disk installed exe is byte-for-byte UNCHANGED,
// NO backup is created, and a stagecoach-upgrade-* staging tempDir is LEFT (FR-U11 inspection).
// exitcode.For(err) == exitcode.Error (1). setupSwapSeams is reused — its mini-swap is NEVER reached
// on this failure path (harmless).
func TestUpgradeFailure_TamperedArchive(t *testing.T) {
	// BUILD + PACK the VALID v0.2.0 payload, then SERVE it with a WRONG (valid-hex) sha — isolates
	// the failure to the SHA check (not a download or checksum-parse error).
	newStub := buildStubVersion(t, "v0.2.0")
	archive, _ := packSwapArchive(t, newStub, "v0.2.0")
	ts := newSwapFake(t, "v0.2.0", archive, strings.Repeat("a", 64)) // 64 hex chars, != real sha

	// INSTALLED STUB (v0.1.0) in its OWN temp dir (SEPARATE from the staging dir runDirectSwap creates).
	oldStub := buildStubVersion(t, "v0.1.0")
	installDir := t.TempDir()
	installedExe := filepath.Join(installDir, "stagecoach"+exeSuffix())
	oldBytes, err := os.ReadFile(oldStub)
	if err != nil {
		t.Fatalf("read old stub %s: %v", oldStub, err)
	}
	if err := os.WriteFile(installedExe, oldBytes, 0o644); err != nil {
		t.Fatalf("write installed stub %s: %v", installedExe, err)
	}

	// SNAPSHOT: installed-exe bytes (the FR-U11 invariant) + the staging tempDir glob (runDirectSwap
	// creates + LEAVES one on failure — the FR-U11 inspection artifact).
	beforeBytes := mustRead(t, installedExe)
	beforeTemp, _ := filepath.Glob(filepath.Join(os.TempDir(), "stagecoach-upgrade-*"))

	// SEAMS (detect→direct + mini-swap [never reached] + baseURL + version) + DRIVE.
	setupSwapSeams(t, ts.URL, installedExe)
	outBuf, errBuf, err := runUpgradeArgs(t, "--yes")

	// ASSERT exit 1 (a checksum failure surfaces as exitcode.New(Error, …) from runDirectSwap).
	if got := exitcode.For(err); got != exitcode.Error {
		t.Fatalf("exit = %d, want %d (Error); stdout=%q stderr=%q",
			got, exitcode.Error, outBuf.String(), errBuf.String())
	}

	// ASSERT on-disk UNCHANGED (FR-U11 byte-for-byte).
	if afterBytes := mustRead(t, installedExe); !bytes.Equal(beforeBytes, afterBytes) {
		t.Errorf("installed exe changed on a failing upgrade (FR-U11: abort-before-swap must leave it byte-for-byte unchanged)")
	}

	// ASSERT NO backup created (FR-U11: the swap step was never reached).
	if _, err := os.Stat(installedExe + backupSuffix()); !os.IsNotExist(err) {
		t.Errorf("backup created on a failing upgrade (FR-U11: no backup must exist when the swap never ran)")
	}

	// ASSERT a staging tempDir is LEFT (FR-U11 inspection artifact — runDirectSwap does NO defer cleanup).
	afterTemp, _ := filepath.Glob(filepath.Join(os.TempDir(), "stagecoach-upgrade-*"))
	if len(afterTemp) <= len(beforeTemp) {
		t.Errorf("no staging tempDir left for inspection (FR-U11); before=%d after=%d", len(beforeTemp), len(afterTemp))
	}
}

// TestUpgradeFailure_WrongVersionSanity is FR-U5/U11 case (b): an archive whose embedded binary
// prints the WRONG tag (v0.3.0 under a v0.2.0 release). VerifySHA256 PASSES (bytes match the real
// sha), but sanityCheck runs '<newBin> --version' ⇒ "v0.3.0\n" which does NOT contain release.Tag
// "v0.2.0" ⇒ ErrSanityVersionMismatch INSIDE StageNewBinary (before confirm/swap). Same outcome as
// (a): unchanged, no backup, temp left, exit 1.
func TestUpgradeFailure_WrongVersionSanity(t *testing.T) {
	// BUILD the WRONG-VERSION payload: bake the stub at v0.3.0, pack it under the v0.2.0 ASSET name
	// (so ResolveTarget/SelectAsset finds it), serve with the MATCHING sha (VerifySHA256 PASSES,
	// sanityCheck fails).
	badStub := buildStubVersion(t, "v0.3.0")
	archive, sha := packSwapArchive(t, badStub, "v0.2.0")
	ts := newSwapFake(t, "v0.2.0", archive, sha)

	// INSTALLED STUB (v0.1.0) as in (a).
	oldStub := buildStubVersion(t, "v0.1.0")
	installDir := t.TempDir()
	installedExe := filepath.Join(installDir, "stagecoach"+exeSuffix())
	oldBytes, err := os.ReadFile(oldStub)
	if err != nil {
		t.Fatalf("read old stub %s: %v", oldStub, err)
	}
	if err := os.WriteFile(installedExe, oldBytes, 0o644); err != nil {
		t.Fatalf("write installed stub %s: %v", installedExe, err)
	}

	beforeBytes := mustRead(t, installedExe)
	beforeTemp, _ := filepath.Glob(filepath.Join(os.TempDir(), "stagecoach-upgrade-*"))

	setupSwapSeams(t, ts.URL, installedExe)
	outBuf, errBuf, err := runUpgradeArgs(t, "--yes")

	if got := exitcode.For(err); got != exitcode.Error {
		t.Fatalf("exit = %d, want %d (Error); stdout=%q stderr=%q",
			got, exitcode.Error, outBuf.String(), errBuf.String())
	}
	if afterBytes := mustRead(t, installedExe); !bytes.Equal(beforeBytes, afterBytes) {
		t.Errorf("installed exe changed on a sanity-failing upgrade (FR-U11)")
	}
	if _, err := os.Stat(installedExe + backupSuffix()); !os.IsNotExist(err) {
		t.Errorf("backup created on a sanity-failing upgrade (FR-U11)")
	}
	afterTemp, _ := filepath.Glob(filepath.Join(os.TempDir(), "stagecoach-upgrade-*"))
	if len(afterTemp) <= len(beforeTemp) {
		t.Errorf("no staging tempDir left for inspection (FR-U11); before=%d after=%d", len(beforeTemp), len(afterTemp))
	}
}

// TestUpgradeDelegation_ManagerOwnedNoForce is FR-U1 case (c): a detected manager-owned install
// (brew) with NO --force is DELEGATED — runDelegate runs, runDirectSwap does NOT. The injected
// recordingExecRunner captures the brew argv; upgradeSwap is wired to failingSwap (t.Fatal — proves
// the delegation path never swaps). NO httptest fake is created (upgradeBaseURL left at its "" default)
// — the strongest proof the delegate path never networks. exit 0; stdout has
// "stagecoach updated via brew"; the installed exe is byte-for-byte unchanged.
func TestUpgradeDelegation_ManagerOwnedNoForce(t *testing.T) {
	// INSTALLED STUB (any version — it must remain UNTOUCHED) in its OWN temp dir.
	installedExe := filepath.Join(t.TempDir(), "stagecoach"+exeSuffix())
	stubBytes, err := os.ReadFile(buildStubVersion(t, "v0.2.0"))
	if err != nil {
		t.Fatalf("read stub: %v", err)
	}
	if err := os.WriteFile(installedExe, stubBytes, 0o644); err != nil {
		t.Fatalf("write installed stub %s: %v", installedExe, err)
	}
	beforeBytes := mustRead(t, installedExe)

	// SEAMS — NO fake needed (delegate never networks). upgradeBaseURL is LEFT at its "" default.
	origDetect := upgradeDetect
	upgradeDetect = func(context.Context, string, func(string)) (upgrade.Channel, string, error) {
		return upgrade.ChannelBrew, "brew", nil
	}
	t.Cleanup(func() { upgradeDetect = origDetect })

	rec := &recordingExecRunner{code: 0, err: nil}
	origExec := upgradeExecRunner
	upgradeExecRunner = rec
	t.Cleanup(func() { upgradeExecRunner = origExec })

	origSwap := upgradeSwap
	upgradeSwap = failingSwap(t) // PROVES the delegation path never swaps (t.Fatal if reached)
	t.Cleanup(func() { upgradeSwap = origSwap })

	t.Cleanup(func() { upgrade.SetCurrentVersion("dev") })
	upgrade.SetCurrentVersion("0.2.0")

	// DRIVE: --yes ⇒ confirmUpgrade short-circuits for brew (a RUN channel). No --force ⇒ delegate.
	outBuf, errBuf, err := runUpgradeArgs(t, "--yes")

	if got := exitcode.For(err); got != exitcode.Success {
		t.Fatalf("exit = %d, want %d (Success); stdout=%q stderr=%q",
			got, exitcode.Success, outBuf.String(), errBuf.String())
	}

	// ASSERT delegated to brew (the POSITIVE proof of delegation).
	if got, want := rec.joinedCalls(), "brew upgrade stagecoach"; got != want {
		t.Errorf("delegated argv = %q; want %q", got, want)
	}
	if !strings.Contains(outBuf.String(), "stagecoach updated via brew") {
		t.Errorf("stdout missing 'stagecoach updated via brew'; got:\n%s", outBuf.String())
	}
	// ASSERT on-disk UNCHANGED (FR-U1: a manager-owned install is delegated, not overwritten).
	// failingSwap never firing ⇒ the test reached here ⇒ the swap path was NOT taken.
	if afterBytes := mustRead(t, installedExe); !bytes.Equal(beforeBytes, afterBytes) {
		t.Errorf("installed exe changed on delegation (FR-U1: manager-owned binaries are delegated, not self-swapped)")
	}
}

// TestUpgradeDelegation_ForceOverride is FR-U1 case (d): --force WITH Detect→ChannelBrew overrides
// the detected manager-owned install. dispatchUpgrade prints the FR-U1 warning to stderr AND routes
// to runDirectSwap (the mini-swap runs — a real swap happens): installed exe → v0.2.0,
// backup → v0.1.0 (FR-U8 one-deep). exit 0. upgradeExecRunner is LEFT at its nil default (runDirectSwap
// does NOT use it — only runDelegate does).
// TestUpgradeDelegation_ChocolateyPrintNoPrompt is the FR-U3/U4/U9 BUG guard: Chocolatey is a PRINT
// channel (FR-U3: "print choco upgrade stagecoach -y; do NOT run it" — needs admin, FR-U4), so like
// AUR/Nix/deb/rpm it MUST NEVER prompt (FR-U9: "a pure PRINT never prompt") and a printed command
// exits 0 (FR-U4/FR-U12). The bug: runDelegate's isRun gate omitted ChannelChocolatey, so on a non-TTY
// without --yes it prompted → refused → exit 1, never printing the command. This drives exactly that
// path (non-TTY, no --yes) and asserts exit 0 + the command printed + no exec + no swap.
func TestUpgradeDelegation_ChocolateyPrintNoPrompt(t *testing.T) {
	setTTY(t, false) // BUG repro: non-interactive stdin, no --yes

	origDetect := upgradeDetect
	upgradeDetect = func(context.Context, string, func(string)) (upgrade.Channel, string, error) {
		return upgrade.ChannelChocolatey, "choco", nil
	}
	t.Cleanup(func() { upgradeDetect = origDetect })

	rec := &recordingExecRunner{code: 0, err: nil}
	origExec := upgradeExecRunner
	upgradeExecRunner = rec // PRINT path never execs; recording proves it (and guards a RUN-routing regression)
	t.Cleanup(func() { upgradeExecRunner = origExec })

	origSwap := upgradeSwap
	upgradeSwap = failingSwap(t) // PROVES the PRINT path never swaps
	t.Cleanup(func() { upgradeSwap = origSwap })

	// DRIVE: NO --yes, non-TTY (the exact BUG #1 repro). Before the fix this exited 1.
	outBuf, errBuf, err := runUpgradeArgs(t)

	if got := exitcode.For(err); got != exitcode.Success {
		t.Fatalf("exit = %d, want %d (Success — FR-U4/FR-U12: a printed command exits 0); stdout=%q stderr=%q",
			got, exitcode.Success, outBuf.String(), errBuf.String())
	}
	if got := rec.joinedCalls(); got != "" {
		t.Errorf("Chocolatey (PRINT) must never exec the updater; recorded %q", got)
	}
	if !strings.Contains(outBuf.String(), "choco upgrade stagecoach -y") {
		t.Errorf("stdout must print the choco update command; got:\n%s", outBuf.String())
	}
	if strings.Contains(outBuf.String(), "Proceed?") || strings.Contains(errBuf.String(), "Proceed?") {
		t.Errorf("Chocolatey (PRINT) must never prompt (FR-U9); stdout=%q stderr=%q", outBuf.String(), errBuf.String())
	}
}

func TestUpgradeDelegation_ForceOverride(t *testing.T) {
	// BUILD + PACK + SERVE the VALID v0.2.0 payload (the happy-path payload, as in S2).
	newStub := buildStubVersion(t, "v0.2.0")
	archive, sha := packSwapArchive(t, newStub, "v0.2.0")
	ts := newSwapFake(t, "v0.2.0", archive, sha)

	// INSTALLED STUB (v0.1.0); chmod runnable on unix for the post-swap --version check.
	oldStub := buildStubVersion(t, "v0.1.0")
	installDir := t.TempDir()
	installedExe := filepath.Join(installDir, "stagecoach"+exeSuffix())
	oldBytes, err := os.ReadFile(oldStub)
	if err != nil {
		t.Fatalf("read old stub %s: %v", oldStub, err)
	}
	if err := os.WriteFile(installedExe, oldBytes, 0o644); err != nil {
		t.Fatalf("write installed stub %s: %v", installedExe, err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(installedExe, 0o755); err != nil {
			t.Fatalf("chmod installed stub %s: %v", installedExe, err)
		}
	}

	// SEAMS: detect→brew + baseURL + mini-swap [RUNS] + version. Leave upgradeExecRunner at default.
	origBase := upgradeBaseURL
	upgradeBaseURL = ts.URL
	t.Cleanup(func() { upgradeBaseURL = origBase })

	origDetect := upgradeDetect
	upgradeDetect = func(context.Context, string, func(string)) (upgrade.Channel, string, error) {
		return upgrade.ChannelBrew, "brew", nil
	}
	t.Cleanup(func() { upgradeDetect = origDetect })

	origSwap := upgradeSwap
	upgradeSwap = miniSwap(installedExe) // the swap RUNS (the --force override path)
	t.Cleanup(func() { upgradeSwap = origSwap })

	t.Cleanup(func() { upgrade.SetCurrentVersion("dev") })
	upgrade.SetCurrentVersion("0.1.0")

	// DRIVE: --yes (confirmUpgrade short-circuits) + --force (warn + route to runDirectSwap).
	outBuf, errBuf, err := runUpgradeArgs(t, "--yes", "--force")

	if got := exitcode.For(err); got != exitcode.Success {
		t.Fatalf("exit = %d, want %d (Success); stdout=%q stderr=%q",
			got, exitcode.Success, outBuf.String(), errBuf.String())
	}

	// ASSERT the FR-U1 warning on STDERR.
	if !strings.Contains(errBuf.String(), "--force overriding a detected brew install") {
		t.Errorf("stderr missing the --force warning; got:\n%s", errBuf.String())
	}
	// ASSERT swapped to NEW (v0.2.0) + backup is OLD (v0.1.0 — FR-U8 one-deep).
	if got := runVersion(t, installedExe); !strings.Contains(got, "v0.2.0") {
		t.Errorf("installed --version = %q; want it to contain v0.2.0 (swapped to new)", got)
	}
	if got := runVersion(t, installedExe+backupSuffix()); !strings.Contains(got, "v0.1.0") {
		t.Errorf("backup --version = %q; want it to contain v0.1.0 (backed up old — FR-U8)", got)
	}
}

// TestUpgradeRollback_NoBackup is FR-U8 case (e1): --rollback with NO backup → upgradeRollback
// returns upgrade.ErrNoBackup → dispatchUpgrade prints "no backup — nothing to roll back" + exit 0
// (a no-op, NEVER an error). The --rollback branch calls upgradeRollback DIRECTLY (no confirmUpgrade),
// so NO --yes is needed. upgradeDetect/upgradeSwap/upgradeBaseURL are unreachable on this path and
// left at their defaults (NO fake needed — rollback never networks).
func TestUpgradeRollback_NoBackup(t *testing.T) {
	origRollback := upgradeRollback
	upgradeRollback = func(context.Context) (string, error) {
		return "", upgrade.ErrNoBackup // FR-U8 no-op sentinel
	}
	t.Cleanup(func() { upgradeRollback = origRollback })

	outBuf, _, err := runUpgradeArgs(t, "--rollback")

	if got := exitcode.For(err); got != exitcode.Success {
		t.Fatalf("exit = %d, want %d (Success); stdout=%q", got, exitcode.Success, outBuf.String())
	}
	if !strings.Contains(outBuf.String(), "no backup — nothing to roll back") {
		t.Errorf("stdout missing 'no backup — nothing to roll back'; got:\n%s", outBuf.String())
	}
}

// TestUpgradeRollback_BackupPresent is FR-U8 case (e2): --rollback WITH a backup → upgradeRollback
// (miniRollback) restores the backup over the installed exe → dispatchUpgrade prints
// "restored stagecoach v0.1.0" + the installed exe now reports v0.1.0 (the current became the backup
// content). exit 0.
func TestUpgradeRollback_BackupPresent(t *testing.T) {
	// INSTALLED STUB (v0.2.0) + a pre-created BACKUP (v0.1.0).
	installDir := t.TempDir()
	installedExe := filepath.Join(installDir, "stagecoach"+exeSuffix())
	curBytes, err := os.ReadFile(buildStubVersion(t, "v0.2.0"))
	if err != nil {
		t.Fatalf("read current stub: %v", err)
	}
	if err := os.WriteFile(installedExe, curBytes, 0o644); err != nil {
		t.Fatalf("write installed stub %s: %v", installedExe, err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(installedExe, 0o755); err != nil {
			t.Fatalf("chmod installed stub %s: %v", installedExe, err)
		}
	}
	backupPath := installedExe + backupSuffix()
	backupBytes, err := os.ReadFile(buildStubVersion(t, "v0.1.0"))
	if err != nil {
		t.Fatalf("read backup stub: %v", err)
	}
	if err := os.WriteFile(backupPath, backupBytes, 0o644); err != nil {
		t.Fatalf("write backup stub %s: %v", backupPath, err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(backupPath, 0o755); err != nil {
			t.Fatalf("chmod backup stub %s: %v", backupPath, err)
		}
	}

	// SEAMS: upgradeRollback → miniRollback(installedExe). miniRollback consumes the backup via rename
	// and returns the restored binary's --version.
	origRollback := upgradeRollback
	upgradeRollback = miniRollback(installedExe)
	t.Cleanup(func() { upgradeRollback = origRollback })

	outBuf, _, err := runUpgradeArgs(t, "--rollback")

	if got := exitcode.For(err); got != exitcode.Success {
		t.Fatalf("exit = %d, want %d (Success); stdout=%q", got, exitcode.Success, outBuf.String())
	}
	if !strings.Contains(outBuf.String(), "restored stagecoach v0.1.0") {
		t.Errorf("stdout missing 'restored stagecoach v0.1.0'; got:\n%s", outBuf.String())
	}
	// ASSERT current became backup content (the installed exe now reports the restored v0.1.0).
	if got := runVersion(t, installedExe); !strings.Contains(got, "v0.1.0") {
		t.Errorf("installed --version = %q; want it to contain v0.1.0 (restored from backup)", got)
	}
}
