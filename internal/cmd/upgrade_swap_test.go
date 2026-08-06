// Package cmd: upgrade_swap_test.go is the FR-U5/U7/U9/U11 direct-swap happy-path suite
// (P1.M4.T3.S2).
//
// It drives runUpgrade end-to-end via Execute(ctx) with args ["upgrade","--yes"] against an
// httptest fake GitHub Releases server, overriding the package-cmd seams:
//   - upgradeBaseURL (the NETWORK seam — prodNewClient reads it at call time and builds
//     Client{BaseURL: upgradeBaseURL, ...}); set to the httptest fake's localhost URL.
//   - upgradeDetect (→upgrade.ChannelDirect, "injected Detect→direct", nil): "injected Detect→
//     direct" so runUpgrade routes to runDirectSwap without a real subprocess probe.
//   - upgradeSwap (a faithful mini-swap closure): package cmd CANNOT reach upgrade.Swap's
//     unexported resolveCurrentExe seam (swap.go), so upgrade_run.go explicitly designed
//     upgradeSwap as the FUNCTION seam to override "to point at a temp-dir exe". The mini-swap
//     replicates upgrade.Swap's success path (backup→rename→staging-tempDir cleanup) against the
//     captured temp stub — it NEVER calls os.Executable/resolveCurrentExe, so the real running
//     `go test` binary is never touched.
//   - upgrade.SetCurrentVersion (the EXPORTED version seam): pins "0.1.0" for the confirmUpgrade
//     display (restored to "dev" via t.Cleanup).
//
// NO real network, NO real subprocess beyond the stub's own --version, and NO rename of the real
// running test binary (FR-U12). The REAL download→verify→extract→sanity runs via StageNewBinary
// against the fake (using the DEFAULT execVersion — stubversion bakes its version, so NO
// execVersion override is needed); the backup→rename→cleanup runs via the mini-swap (the real
// upgrade.Swap is exhaustively covered by swap_test.go/swap_windows_test.go).
//
// Dedicated file: S1 owns upgrade_check_test.go (the --check suite); S3 owns the failure/rollback
// suite. Zero production edits — this file only READS the LANDED runDirectSwap + seams in
// upgrade_run.go / upgrade.go.
package cmd

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/dabstractor/stagecoach/internal/exitcode"
	"github.com/dabstractor/stagecoach/internal/upgrade"
)

// exeSuffix returns ".exe" on windows else "" — used to compute the host-native binary entry
// name, the installed-stub path, and the backup suffix. Tied to the test HOST.
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// backupSuffix returns the one-deep backup suffix MIRRORED from swap_unix.go / swap_windows.go's
// backupPath: ".stagecoach-backup" on unix, ".old" on windows. The mini-swap AND the test's
// backup assertion MUST use this platform-correct suffix (asserting ".stagecoach-backup" on
// windows would fail — the file is ".old" there).
func backupSuffix() string {
	if runtime.GOOS == "windows" {
		return ".old"
	}
	return ".stagecoach-backup"
}

// hostAssetName returns the host-native archive asset name for tag — a twin of the unexported
// download.go::assetName (package cmd cannot call it): "stagecoach_<v-without-v>_<GOOS>_<GOARCH>"
// + ".zip" (windows) or ".tar.gz" (else). The fake's release JSON MUST list an asset with this
// EXACT name or ResolveTarget→SelectAsset returns ErrNoMatchingAsset.
func hostAssetName(tag string) string {
	name := "stagecoach_" + strings.TrimPrefix(tag, "v") + "_" + runtime.GOOS + "_" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		return name + ".zip"
	}
	return name + ".tar.gz"
}

// hostEntryName returns the host-native binary entry base name ("stagecoach" / "stagecoach.exe") —
// the archive entry extractBinary pulls ONLY.
func hostEntryName() string {
	return "stagecoach" + exeSuffix()
}

// checksumsName returns the checksums file name — a twin of the unexported download.go::
// checksumsName (package cmd cannot call it): "stagecoach_<v-without-v>_checksums.txt".
func checksumsName(tag string) string {
	return "stagecoach_" + strings.TrimPrefix(tag, "v") + "_checksums.txt"
}

// stubVerCache memoizes compiled stubversion binaries per version so multiple S2 cases (and the
// two distinct versions v0.1.0 / v0.2.0 the happy path needs) don't rebuild the same version.
var (
	stubVerMu    sync.Mutex
	stubVerCache = map[string]string{} // version → compiled binary path
)

// buildStubVersion compiles github.com/dabstractor/stagecoach/cmd/stubversion with a build-time
// ldflags-baked `version` (-X main.version=<version>) and returns its path (cached per version).
// It skips t if the go toolchain is not on PATH. It is a CLONE of stage_test.go::buildStubCLI,
// CWD-independent (resolves the module root via `go env GOMOD` so `go build` finds go.mod
// regardless of the process CWD) and cross-platform (picks the .exe suffix on windows for the -o
// path). -buildvcs=false keeps the build reproducible off a VCS checkout.
func buildStubVersion(t *testing.T, version string) string {
	t.Helper()
	stubVerMu.Lock()
	if p, ok := stubVerCache[version]; ok {
		stubVerMu.Unlock()
		return p
	}
	stubVerMu.Unlock()

	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain not on PATH; cannot build stubversion: %v", err)
	}
	dir, err := os.MkdirTemp("", "stagecoach-stubversion-*")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	name := "stubversion-" + version + exeSuffix()
	out := filepath.Join(dir, name)
	build := exec.Command(goPath, "build", "-buildvcs=false",
		"-ldflags", "-X main.version="+version,
		"-o", out,
		"github.com/dabstractor/stagecoach/cmd/stubversion")
	// Resolve the module root from GOMOD so `go build` finds go.mod regardless of the process CWD
	// (tests may chdir into temp dirs). Sentinel "/dev/null" means no go.mod (go env GOMOD outside a module).
	if b, err := exec.Command(goPath, "env", "GOMOD").Output(); err == nil {
		if gomod := strings.TrimSpace(string(b)); gomod != "" && gomod != "/dev/null" {
			build.Dir = filepath.Dir(gomod)
		}
	}
	if b, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build stubversion %s: %v\n%s", version, err, b)
	}

	stubVerMu.Lock()
	stubVerCache[version] = out
	stubVerMu.Unlock()
	return out
}

// packSwapArchive packs stubPath's bytes under hostEntryName() into the host-native archive
// format (.zip on windows, else .tar.gz), plus a throwaway README entry to prove extractBinary
// pulls ONLY the stagecoach entry. Returns (archiveBytes, sha256hex). It is a PORT of
// stage_test.go::packArchive (package cmd copy — package cmd cannot import the package-upgrade
// helper). The tar entry's header Mode is 0o755 (the best-effort chmod in stage.go is exercised
// but not strictly required).
func packSwapArchive(t *testing.T, stubPath, tag string) ([]byte, string) {
	t.Helper()
	stubBytes, err := os.ReadFile(stubPath)
	if err != nil {
		t.Fatalf("read stub %s: %v", stubPath, err)
	}
	asset := hostAssetName(tag)
	entry := hostEntryName()
	var buf bytes.Buffer
	if strings.HasSuffix(asset, ".zip") {
		zw := zip.NewWriter(&buf)
		// A README entry the extractor MUST skip (proves "extract ONLY the stagecoach entry").
		if w, err := zw.Create("README.md"); err != nil {
			t.Fatalf("zip create README: %v", err)
		} else if _, err := w.Write([]byte("this must not be extracted\n")); err != nil {
			t.Fatalf("zip write README: %v", err)
		}
		// The stagecoach binary entry at the archive root.
		fh := &zip.FileHeader{Name: entry, Method: zip.Deflate}
		fh.SetMode(0o755)
		w, err := zw.CreateHeader(fh)
		if err != nil {
			t.Fatalf("zip create %s: %v", entry, err)
		}
		if _, err := w.Write(stubBytes); err != nil {
			t.Fatalf("zip write %s: %v", entry, err)
		}
		if err := zw.Close(); err != nil {
			t.Fatalf("zip close: %v", err)
		}
	} else {
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		// A README entry the extractor MUST skip.
		readme := []byte("this must not be extracted\n")
		if err := tw.WriteHeader(&tar.Header{Name: "README.md", Mode: 0o644, Size: int64(len(readme))}); err != nil {
			t.Fatalf("tar write README header: %v", err)
		}
		if _, err := tw.Write(readme); err != nil {
			t.Fatalf("tar write README: %v", err)
		}
		// The stagecoach binary entry at the archive root.
		if err := tw.WriteHeader(&tar.Header{Name: entry, Mode: 0o755, Size: int64(len(stubBytes))}); err != nil {
			t.Fatalf("tar write %s header: %v", entry, err)
		}
		if _, err := tw.Write(stubBytes); err != nil {
			t.Fatalf("tar write %s: %v", entry, err)
		}
		if err := tw.Close(); err != nil {
			t.Fatalf("tar close: %v", err)
		}
		if err := gz.Close(); err != nil {
			t.Fatalf("gzip close: %v", err)
		}
	}
	archive := buf.Bytes()
	sum := sha256.Sum256(archive)
	return archive, hex.EncodeToString(sum[:])
}

// newSwapFake spins up an httptest.Server that mimics the three GitHub routes StageNewBinary's
// download/verify primitives hit:
//   - /repos/dabstractor/stagecoach/releases/latest → the release JSON (tag_name + assets[]).
//   - /archive → the archive bytes (a browser_download_url target).
//   - /checksums → "<sha>  <assetName>\n" (a browser_download_url target).
//
// The release assets carry ABSOLUTE browser_download_urls back at the fake (DownloadFile/
// FetchChecksums use the url VERBATIM — BaseURL is metadata-only for asset downloads). The JSON is
// built AFTER the server exists (so ts.URL is known) and installed via ts.Config.Handler (the
// documented way to (re)assign a server's handler post-construction). Closed via t.Cleanup.
func newSwapFake(t *testing.T, tag string, archiveBytes []byte, shaHex string) *httptest.Server {
	t.Helper()
	asset := hostAssetName(tag)
	checkAsset := checksumsName(tag)
	// Two-step: create the server first so ts.URL is known, then install the handler that closes
	// over it. The initial no-op handler is replaced before any request is served.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	releaseJSON := fmt.Sprintf(
		`{"tag_name":%q,"name":%q,"prerelease":false,"draft":false,"assets":[`+
			`{"name":%q,"browser_download_url":%q,"size":%d},`+
			`{"name":%q,"browser_download_url":%q,"size":%d}]}`,
		tag, "Release "+strings.TrimPrefix(tag, "v"),
		asset, ts.URL+"/archive", len(archiveBytes),
		checkAsset, ts.URL+"/checksums", len(shaHex)+2+len(asset)+1, // "<sha>  <asset>\n"
	)
	checkBody := fmt.Sprintf("%s  %s\n", shaHex, asset)
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, releaseJSON)
		case strings.HasSuffix(r.URL.Path, "/archive"):
			_, _ = io.Copy(w, bytes.NewReader(archiveBytes))
		case strings.HasSuffix(r.URL.Path, "/checksums"):
			_, _ = io.WriteString(w, checkBody)
		default:
			http.NotFound(w, r)
		}
	})
	t.Cleanup(ts.Close)
	return ts
}

// runVersion runs "<path> --version" and returns its stdout. stubversion ignores argv/env and just
// prints its baked version, so the post-swap assertions can read the installed/backup binary's
// version directly. t.Fatal on any run error (a swapped binary that fails to run is a hard
// contract violation).
func runVersion(t *testing.T, path string) string {
	t.Helper()
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		t.Fatalf("run %s --version: %v", path, err)
	}
	return string(out)
}

// setupSwapSeams wires the four seams the direct-swap happy path reads and registers their
// restoration via t.Cleanup (Go LIFO ⇒ the last-registered restore runs first):
//   - upgradeBaseURL (the NETWORK seam): set to tsURL so prodNewClient builds a Client pointed at
//     the localhost fake. Restored to its captured original (the "" default in a fresh process).
//   - upgradeDetect (→ChannelDirect injection): overridden to return (ChannelDirect, "direct", nil)
//     so runUpgrade routes to runDirectSwap without a real subprocess probe. Restored to its
//     captured original (prodDetect).
//   - upgradeSwap (the FUNCTION swap seam): overridden with a mini-swap closure (see below) that
//     backs up + renames + cleans the staging tempDir against the captured installedExe — package
//     cmd CANNOT reach upgrade.Swap's unexported resolveCurrentExe, so upgrade_run.go designed
//     upgradeSwap as the seam to override. Restored to its captured original (upgrade.Swap).
//   - upgrade.SetCurrentVersion (the version seam): pins "0.1.0" for the confirmUpgrade display.
//     The t.Cleanup(SetCurrentVersion("dev")) restore is registered last; Go runs Cleanups LIFO,
//     so it runs before the seam-var restores above — and none of those touch currentVersion, so
//     the final unwind leaves currentVersion at "dev" (the package default).
//
// The caller still owns the cobra-flag/state restoration (saveRootState/restoreRootState +
// resetFlags(upgradeCmd.Flags())) — those reset cobra flag state, not these non-flag seam vars.
func setupSwapSeams(t *testing.T, tsURL, installedExe string) {
	t.Helper()

	// (a) NETWORK seam. Captured + restored FIRST ⇒ its restore runs LAST (LIFO).
	origBase := upgradeBaseURL
	upgradeBaseURL = tsURL
	t.Cleanup(func() { upgradeBaseURL = origBase })

	// (b) DETECT→direct injection.
	origDetect := upgradeDetect
	upgradeDetect = func(context.Context, string, func(string)) (upgrade.Channel, string, error) {
		return upgrade.ChannelDirect, "direct", nil
	}
	t.Cleanup(func() { upgradeDetect = origDetect })

	// (c) SWAP function seam — the mini-swap. A FAITHFUL twin of upgrade.Swap's success path:
	// one-deep backup (FR-U8) → atomic rename into place → staging-tempDir cleanup (FR-U11). On a
	// mid-swap rename failure it restores from the backup (best-effort, like platformSwap). It
	// NEVER calls os.Executable/resolveCurrentExe ⇒ the real running `go test` binary is safe.
	origSwap := upgradeSwap
	upgradeSwap = func(ctx context.Context, newBinPath string) error {
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
	t.Cleanup(func() { upgradeSwap = origSwap })

	// (d) VERSION seam. Register the "dev" reset BEFORE setting the per-call value: Go runs
	// Cleanups LIFO, so a reset registered here (last) runs FIRST (ahead of the seam-var restores
	// above), then the per-call SetCurrentVersion("0.1.0") sets the display value for THIS run.
	// The net effect across the whole t.Cleanup chain leaves currentVersion at "dev" (the package
	// default) — matching S1's setupCheckSeams discipline.
	t.Cleanup(func() { upgrade.SetCurrentVersion("dev") })
	upgrade.SetCurrentVersion("0.1.0")
}

// runUpgradeSwap drives `stagecoach upgrade --yes` end-to-end via Execute(ctx), mirroring the
// upgrade_test.go::TestUpgradeCommand_NoBootstrapOutsideRepo idiom: saveRootState/restoreRootState
// + resetFlags(upgradeCmd.Flags()) (the latter is SEPARATE from restoreRootState — it resets the
// upgradeCmd LOCAL flags flagYes/flagCheck/flagChannel that restoreRootState does NOT touch),
// isolated HOME (t.TempDir + XDG_CONFIG_HOME="" so LoadUpgradeConfig ⇒ Defaults with no real
// config and no bootstrap), and outBuf/errBuf wired to rootCmd. Execute returns runUpgrade's error
// (cobra SilenceErrors=true ⇒ returned, not printed); the caller maps it via exitcode.For.
func runUpgradeSwap(t *testing.T) (outBuf, errBuf *bytes.Buffer, err error) {
	t.Helper()
	_, origOut, origErr, origRunE := saveRootState(t)
	t.Cleanup(func() { restoreRootState(t, nil, origOut, origErr, origRunE) })
	t.Cleanup(func() { resetFlags(upgradeCmd.Flags()) }) // SEPARATE from restoreRootState
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "") // LoadUpgradeConfig ⇒ Defaults (no global config, no bootstrap)
	outBuf, errBuf = &bytes.Buffer{}, &bytes.Buffer{}
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"upgrade", "--yes"}) // flagYes=true ⇒ confirmUpgrade short-circuits (no TTY needed)
	err = Execute(context.Background())
	return outBuf, errBuf, err
}

// TestUpgradeSwap_DirectHappyPath is the FR-U5/U7/U9/U11 direct-binary self-swap happy path: a
// v0.1.0 "installed" stub gets swapped to a v0.2.0 payload served by an httptest fake, with the
// full runDirectSwap pipeline (download→verify→extract→sanity via the REAL StageNewBinary against
// the fake, then backup→rename→cleanup via the mini-swap). Asserts:
//   - exit 0 (exitcode.For(err) == exitcode.Success); stdout has "stagecoach upgraded to v0.2.0";
//   - the installed exe's --version == v0.2.0 (the NEW binary renamed into place);
//   - the backup's (installed + backupSuffix()) --version == v0.1.0 (the OLD binary backed up — FR-U8);
//   - the "stagecoach-upgrade-*" staging tempDir glob is UNCHANGED before==after Execute (cleaned).
//
// The installed stub lives in its OWN t.TempDir() (SEPARATE from the staging dir runDirectSwap
// creates via os.MkdirTemp) so the mini-swap's os.RemoveAll(filepath.Dir(newBin)) removes the
// staging dir, NOT the installed-stub dir + the backup we assert on.
func TestUpgradeSwap_DirectHappyPath(t *testing.T) {
	// BUILD + PACK + SERVE the v0.2.0 payload.
	oldStub := buildStubVersion(t, "v0.1.0")
	newStub := buildStubVersion(t, "v0.2.0")
	archive, sha := packSwapArchive(t, newStub, "v0.2.0")
	ts := newSwapFake(t, "v0.2.0", archive, sha)

	// INSTALLED STUB (the "current" v0.1.0 binary) in its OWN temp dir, SEPARATE from the staging dir.
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
		// Make the installed stub runnable for the post-swap --version check (the archive extraction
		// already 0o755s the new binary; the installed stub is a plain copy here).
		if err := os.Chmod(installedExe, 0o755); err != nil {
			t.Fatalf("chmod installed stub %s: %v", installedExe, err)
		}
	}

	// PRE-STATE: the staging tempDir glob (runDirectSwap creates + the mini-swap removes one on success).
	before, _ := filepath.Glob(filepath.Join(os.TempDir(), "stagecoach-upgrade-*"))

	// SEAMS + DRIVE.
	setupSwapSeams(t, ts.URL, installedExe)
	outBuf, errBuf, err := runUpgradeSwap(t)

	// ASSERT exit 0 (a SelectAsset/StageNewBinary/swap failure surfaces here with a diagnostic).
	if got := exitcode.For(err); got != exitcode.Success {
		t.Fatalf("exit = %d, want %d (Success); stdout=%q stderr=%q",
			got, exitcode.Success, outBuf.String(), errBuf.String())
	}

	// ASSERT the runDirectSwap success line.
	if !strings.Contains(outBuf.String(), "stagecoach upgraded to v0.2.0") {
		t.Errorf("stdout missing 'stagecoach upgraded to v0.2.0'; got:\n%s", outBuf.String())
	}

	// ASSERT installed swapped to NEW (the v0.2.0 payload renamed into place).
	if got := runVersion(t, installedExe); !strings.Contains(got, "v0.2.0") {
		t.Errorf("installed --version = %q; want it to contain v0.2.0 (swapped to new)", got)
	}

	// ASSERT backup created with OLD (FR-U8 one-deep).
	backup := installedExe + backupSuffix()
	if got := runVersion(t, backup); !strings.Contains(got, "v0.1.0") {
		t.Errorf("backup --version = %q; want it to contain v0.1.0 (backed up old)", got)
	}

	// ASSERT temp cleaned: the staging dir runDirectSwap created was removed by the mini-swap (FR-U11).
	after, _ := filepath.Glob(filepath.Join(os.TempDir(), "stagecoach-upgrade-*"))
	if !reflect.DeepEqual(before, after) {
		t.Errorf("staging tempDir glob changed: before=%v after=%v (the staging dir must be cleaned on success)",
			before, after)
	}
}
