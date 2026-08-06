// Package cmd: upgrade_check_test.go is the FR-U6 `stagecoach upgrade --check` suite (P1.M4.T3.S1).
//
// It drives runUpgrade end-to-end via Execute(ctx) with args ["upgrade","--check"] against an
// httptest fake GitHub Releases server, overriding the package-cmd upgradeBaseURL seam (the
// single load-bearing NETWORK seam — prodNewClient reads it at call time and builds
// Client{BaseURL: upgradeBaseURL, ...}) plus the package-upgrade SetCurrentVersion seam (which
// pins the running version CurrentSemver reads; the test process never runs main.go, so
// currentVersion starts at the "dev" package default).
//
// NO real network, NO real subprocess, NO real binary swap (FR-U12). Every --check code path
// (validateUpgradeFlags + LoadUpgradeConfig + effChannel resolution + runCheck) is exercised
// EXCEPT the swap/rollback/detect seams, which belong to the normal/rollback paths (S2/S3) and
// are never reached by --check. Exactly one GET hits the fake per case (a request counter proves
// it), and upgradeBaseURL=localhost guarantees no real network call.
//
// Dedicated file (not additions to upgrade_test.go): S1 owns upgrade_test.go (registration +
// flag validation); S2 owns upgrade_prompt_test.go (the confirm/swap path). The per-feature-
// test-file precedent + the three upcoming P1.M4.T3 suites argue for one file each. The item
// contract's "upgrade_test.go" is the logical suite name; the physical file is an implementation
// detail (this one). Zero production edits — this file only READS the LANDED runCheck + seams in
// upgrade_run.go / upgrade.go.
package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dabstractor/stagecoach/internal/exitcode"
	"github.com/dabstractor/stagecoach/internal/upgrade"
)

// newCheckFake spins up an httptest.Server that mimics the GitHub /releases/latest endpoint the
// releases Client decodes (ghRelease{tag_name, prerelease, draft, assets[]}). For a request whose
// URL.Path has suffix "/releases/latest" it responds with `status`; on 200 the body is a canned
// release whose tag_name == tag (mirrors releases_test.go::cannedLatest, parameterized by tag).
// On any non-200 status it serves a GitHub-shaped `{"message":"Not Found"}` body (the 404 ⇒
// ErrNoReleases mapping in releases.go). Every request atomically increments a counter so the
// caller can prove exactly one GET hit the fake (the no-network / no-double-call proof). The
// server is closed via t.Cleanup (no leaked ports across the 5 cases).
// assetNameFor builds the goreleaser platform archive name SelectAsset exact-matches against:
// "stagecoach_<tag-without-v>_<goos>_<goarch>.tar.gz" (".zip" on windows). The asset name in the
// canned body MUST embed the tag + the runtime platform, or runCheck's ResolveTarget→SelectAsset
// rejects the release with ErrNoMatchingAsset before Compare ever runs. (runCheck leaves
// ResolveOptions.GOOS/GOARCH empty ⇒ ResolveTarget defaults to runtime.GOOS/GOARCH.)
func assetNameFor(tag string) string {
	name := "stagecoach_" + strings.TrimPrefix(tag, "v") + "_" + runtime.GOOS + "_" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		return name + ".zip"
	}
	return name + ".tar.gz"
}

func newCheckFake(t *testing.T, tag string, status int) (*httptest.Server, *int32) {
	t.Helper()
	var n int32
	body := fmt.Sprintf(
		`{"tag_name":%q,"name":"Release","prerelease":false,"draft":false,"assets":[{"name":%q,"browser_download_url":"https://example.com/a","size":1}]}`,
		tag, assetNameFor(tag),
	)
	if status != http.StatusOK {
		body = `{"message":"Not Found"}`
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		if status == http.StatusOK && !strings.HasSuffix(r.URL.Path, "/releases/latest") {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	return ts, &n
}

// setupCheckSeams wires the two non-flag seams the --check path reads and registers their
// restoration via t.Cleanup:
//   - upgradeBaseURL (the NETWORK seam): set to tsURL so prodNewClient builds a Client pointed at
//     the localhost fake. Restored to its captured original (the "" default in a fresh process).
//   - upgrade.SetCurrentVersion (the version seam): there is NO getter; the test process never ran
//     main.go, so package upgrade's currentVersion starts at "dev" (the package default). The
//     per-case caller pins it (e.g. "0.1.0"); this helper's t.Cleanup(func{SetCurrentVersion("dev")})
//     is the LAST word because Go runs Cleanups LIFO and it is registered FIRST (before the per-case
//     pin). "dev" round-trips: SetCurrentVersion accepts non-empty and ignores "" only.
//
// The caller still owns the cobra-flag/state restoration (saveRootState/restoreRootState +
// resetFlags(upgradeCmd.Flags())) — those reset cobra flag state, not these non-flag seam vars.
func setupCheckSeams(t *testing.T, tsURL string) {
	t.Helper()
	origBase := upgradeBaseURL
	upgradeBaseURL = tsURL
	t.Cleanup(func() { upgradeBaseURL = origBase })
	t.Cleanup(func() { upgrade.SetCurrentVersion("dev") }) // no getter; "dev" is the package default
}

// runUpgradeCheck drives `stagecoach upgrade --check` end-to-end via Execute(ctx), mirroring the
// TestUpgradeCommand_NoBootstrapOutsideRepo idiom: saveRootState/restoreRootState +
// resetFlags(upgradeCmd.Flags()) (the latter is SEPARATE from restoreRootState — it resets the
// upgradeCmd LOCAL flags flagCheck/flagVersion/flagChannel that restoreRootState does NOT touch),
// isolated HOME (t.TempDir + XDG_CONFIG_HOME="" so LoadUpgradeConfig ⇒ Defaults with no real
// config and no bootstrap), and outBuf/errBuf wired to rootCmd. Execute returns runUpgrade's error
// (cobra SilenceErrors=true ⇒ returned, not printed); the caller maps it via exitcode.For.
func runUpgradeCheck(t *testing.T) (outBuf, errBuf *bytes.Buffer, err error) {
	t.Helper()
	_, origOut, origErr, origRunE := saveRootState(t)
	t.Cleanup(func() { restoreRootState(t, nil, origOut, origErr, origRunE) })
	t.Cleanup(func() { resetFlags(upgradeCmd.Flags()) }) // SEPARATE from restoreRootState
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "") // LoadUpgradeConfig ⇒ Defaults (no global config, no bootstrap)
	outBuf, errBuf = &bytes.Buffer{}, &bytes.Buffer{}
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"upgrade", "--check"})
	err = Execute(context.Background())
	return outBuf, errBuf, err
}

// TestUpgradeCheck_Behind_Exit6 is contract case (a): current "0.1.0", latest tag "v0.2.0" ⇒
// stdout prints the v-prefixed "update available: v0.1.0 → v0.2.0; …" line AND
// exitcode.For(err)==exitcode.UpdateAvailable (6). stderr is EMPTY: runCheck returns
// exitcode.New(UpdateAvailable, nil) ⇒ ExitError.Err==nil ⇒ Error()=="" ⇒ main.go's
// `if err != nil && err.Error() != ""` gate SKIPS the stderr print (no double "stagecoach:"
// prefix); cobra (SilenceErrors) doesn't print either. Exactly one GET hits the fake.
func TestUpgradeCheck_Behind_Exit6(t *testing.T) {
	ts, n := newCheckFake(t, "v0.2.0", http.StatusOK)
	setupCheckSeams(t, ts.URL)
	upgrade.SetCurrentVersion("0.1.0") // restored to "dev" by setupCheckSeams's t.Cleanup (LIFO)

	outBuf, errBuf, err := runUpgradeCheck(t)

	if got := exitcode.For(err); got != exitcode.UpdateAvailable {
		t.Errorf("exit = %d, want %d (UpdateAvailable)", got, exitcode.UpdateAvailable)
	}
	out := outBuf.String()
	for _, want := range []string{"update available", "v0.1.0 → v0.2.0", "→", `run "stagecoach upgrade"`} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q; got:\n%s", want, out)
		}
	}
	// exit-6 stderr is SILENT: ExitError.Err==nil ⇒ Error()=="" ⇒ main's gate skips the print.
	if errBuf.Len() != 0 {
		t.Errorf("exit-6 stderr must be empty (no double 'stagecoach:' prefix); got %q", errBuf.String())
	}
	if got := atomic.LoadInt32(n); got != 1 {
		t.Errorf("fake request count = %d, want exactly 1 (the sole network target)", got)
	}
}

// TestUpgradeCheck_UpToDate_Exit0 is contract case (b): current "0.2.0", latest tag "v0.2.0" ⇒
// Compare >= 0 ⇒ "up to date" line + err==nil (exit 0). stdout contains both v0.2.0 occurrences
// (current + latest) per runCheck's Fprintf.
func TestUpgradeCheck_UpToDate_Exit0(t *testing.T) {
	ts, _ := newCheckFake(t, "v0.2.0", http.StatusOK)
	setupCheckSeams(t, ts.URL)
	upgrade.SetCurrentVersion("0.2.0")

	outBuf, _, err := runUpgradeCheck(t)

	if err != nil {
		t.Fatalf("err = %v, want nil (exit 0)", err)
	}
	out := outBuf.String()
	if !strings.Contains(out, "up to date") {
		t.Errorf("stdout missing 'up to date'; got:\n%s", out)
	}
	if c := strings.Count(out, "v0.2.0"); c < 2 {
		t.Errorf("stdout should contain v0.2.0 twice (current + latest); count=%d; got:\n%s", c, out)
	}
}

// TestUpgradeCheck_Dev_Exit0 is contract case (c): current "dev" (unparseable) ⇒
// CurrentSemver ok=false ⇒ runCheck's dev branch prints the informational dev line and returns
// nil (exit 0). Compare is never called. The dev output must NOT contain "update available"
// (FR-U6: dev never falsely signals an update).
func TestUpgradeCheck_Dev_Exit0(t *testing.T) {
	ts, _ := newCheckFake(t, "v0.2.0", http.StatusOK)
	setupCheckSeams(t, ts.URL)
	upgrade.SetCurrentVersion("dev") // currentVersion default is already "dev"; explicit is robust to ordering

	outBuf, _, err := runUpgradeCheck(t)

	if err != nil {
		t.Fatalf("err = %v, want nil (exit 0)", err)
	}
	out := outBuf.String()
	if !strings.Contains(out, "development build — cannot compare") {
		t.Errorf("stdout missing the dev line (note the em-dash); got:\n%s", out)
	}
	if strings.Contains(out, "update available") {
		t.Errorf("dev output must NOT claim an update (FR-U6); got:\n%s", out)
	}
}

// TestUpgradeCheck_NoReleases_Exit1 is contract case (d): the fake returns 404 ⇒
// Client.LatestStable ⇒ ErrNoReleases ⇒ runCheck wraps it as
// fmt.Errorf("stagecoach: check: %w", ErrNoReleases) and returns exitcode.New(Error, …).
// exitcode.For(err)==1 AND errors.Is(err, upgrade.ErrNoReleases)==true (the *ExitError.Unwrap
// chain reaches the sentinel).
func TestUpgradeCheck_NoReleases_Exit1(t *testing.T) {
	ts, _ := newCheckFake(t, "", http.StatusNotFound) // tag unused; 404 ⇒ {"message":"Not Found"}
	setupCheckSeams(t, ts.URL)
	upgrade.SetCurrentVersion("0.1.0") // any parseable current; the 404 short-circuits before Compare

	_, _, err := runUpgradeCheck(t)

	if got := exitcode.For(err); got != exitcode.Error {
		t.Errorf("exit = %d, want %d (Error)", got, exitcode.Error)
	}
	if !errors.Is(err, upgrade.ErrNoReleases) {
		t.Errorf("err = %v, want errors.Is(upgrade.ErrNoReleases)", err)
	}
}

// TestUpgradeCheck_NoOnDiskChange is contract case (e): --check must leave NO on-disk artifact.
// Pinned current "0.1.0" + fake tag "v0.2.0" force the FULL --check path through the behind
// branch (exit 6) — proving even the "would update" path touches nothing. BEFORE+AFTER Execute:
//   - filepath.Glob of os.TempDir()/stagecoach-upgrade-* is UNCHANGED (runDirectSwap's temp dir is
//     never created by --check);
//   - os.Stat(os.Executable()) Size + ModTime are UNCHANGED (runCheck never calls StageNewBinary/
//     Swap — it only does ResolveTarget + CurrentSemver + Compare).
//
// The test MAKES the guarantee explicit so a future refactor routing --check through a temp dir or
// a binary touch would fail here.
func TestUpgradeCheck_NoOnDiskChange(t *testing.T) {
	ts, _ := newCheckFake(t, "v0.2.0", http.StatusOK)
	setupCheckSeams(t, ts.URL)
	upgrade.SetCurrentVersion("0.1.0") // full --check path incl. the behind branch (exit 6)

	pattern := filepath.Join(os.TempDir(), "stagecoach-upgrade-*")
	glob0, _ := filepath.Glob(pattern)
	exePath, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable: %v (cannot assert exe-untouched on this runner)", err)
	}
	exeStat0, err := os.Stat(exePath)
	if err != nil {
		t.Skipf("os.Stat(exe): %v (cannot assert exe-untouched on this runner)", err)
	}

	_, _, runErr := runUpgradeCheck(t)

	if got := exitcode.For(runErr); got != exitcode.UpdateAvailable {
		t.Fatalf("exit = %d, want %d (confirm the full --check path executed)", got, exitcode.UpdateAvailable)
	}

	glob1, _ := filepath.Glob(pattern)
	if !reflect.DeepEqual(glob0, glob1) {
		t.Errorf("temp-dir glob changed: before=%v after=%v (no 'stagecoach-upgrade-*' may be created by --check)",
			glob0, glob1)
	}
	exeStat1, err := os.Stat(exePath)
	if err != nil {
		t.Fatalf("os.Stat(exe) after run: %v", err)
	}
	if exeStat0.Size() != exeStat1.Size() {
		t.Errorf("exe Size changed: before=%d after=%d (--check must not touch the running binary)",
			exeStat0.Size(), exeStat1.Size())
	}
	if !exeStat0.ModTime().Equal(exeStat1.ModTime()) {
		t.Errorf("exe ModTime changed: before=%s after=%s (--check must not touch the running binary)",
			exeStat0.ModTime(), exeStat1.ModTime())
	}
}
