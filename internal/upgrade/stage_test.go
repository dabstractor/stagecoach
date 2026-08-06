// stage_test.go exercises StageNewBinary's contract end-to-end with a REAL httptest server and a
// REAL compiled cmd/stubcli binary packed into the host-native archive format. It is NOT parallel:
// several tests mutate the package-level execVersion seam, and two parallel tests racing on that var
// would trip the race detector. The sibling upgrade tests (releases/detect/delegate/resolve) never
// touch execVersion and may stay parallel. Stdlib-only (the archive/* + crypto/sha256 + net/http/
// httptest usage is all test-time stdlib; no internal/* — FR-U12 is a production-code wall).
package upgrade

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// exeSuffix returns ".exe" on windows else "" — used to compute the host-native binary entry name
// and the expected staged dest name. Tied to the test HOST (the archive packed for THIS process
// uses the host format); stage.go's extraction keys the suffix off the ASSET NAME (platform-agnostic).
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// hostAssetName returns the host-native archive asset name for tag (via the in-package assetName
// helper — stage_test is package upgrade). "v1.2.3" on linux/amd64 ⇒ "stagecoach_1.2.3_linux_amd64.tar.gz".
func hostAssetName(tag string) string {
	return assetName(tag, runtime.GOOS, runtime.GOARCH)
}

// hostEntryName returns the host-native binary entry base name ("stagecoach" / "stagecoach.exe").
func hostEntryName() string {
	return "stagecoach" + exeSuffix()
}

// stubCLIONCE + stubCLIPath compile cmd/stubcli ONCE per test process (cached). Clones stubtest.Build's
// sync.Once pattern but with a SEPARATE cache (stubtest.Build hardcodes cmd/stubagent + its path).
var (
	stubCLIONCE sync.Once
	stubCLIPath string
)

// buildStubCLI compiles github.com/dabstractor/stagecoach/cmd/stubcli once per test process (cached)
// and returns its path. It skips t if the go toolchain is not on PATH. The build runs with Dir set
// to the module root (discovered via `go env GOMOD`) so it is independent of the process CWD (tests
// may chdir into temp dirs). Mirrors internal/stubtest.Build's CWD-independence trick.
func buildStubCLI(t *testing.T) string {
	t.Helper()
	stubCLIONCE.Do(func() {
		goPath, err := exec.LookPath("go")
		if err != nil {
			t.Skipf("go toolchain not on PATH; cannot build stubcli: %v", err)
			return
		}
		dir, err := os.MkdirTemp("", "stagecoach-stubcli-*")
		if err != nil {
			t.Fatalf("mkdtemp: %v", err)
		}
		name := "stubcli"
		if runtime.GOOS == "windows" {
			name = "stubcli.exe"
		}
		stubCLIPath = filepath.Join(dir, name)
		build := exec.Command(goPath, "build", "-buildvcs=false", "-o", stubCLIPath, "github.com/dabstractor/stagecoach/cmd/stubcli")
		// Resolve the module root from GOMOD so `go build` finds go.mod regardless of the process CWD.
		if out, err := exec.Command(goPath, "env", "GOMOD").Output(); err == nil {
			if gomod := strings.TrimSpace(string(out)); gomod != "" && gomod != "/dev/null" {
				build.Dir = filepath.Dir(gomod)
			}
		}
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go build stubcli: %v\n%s", err, out)
		}
	})
	return stubCLIPath
}

// packArchive packs stubPath's bytes under entryName into the host-native archive format (.zip on
// windows, else .tar.gz), plus a no-op README entry to prove extractBinary pulls ONLY the stagecoach
// entry. Returns (archiveBytes, sha256hex). The tar entry's header Mode is 0o755 so the best-effort
// chmod in stage.go is exercised but not strictly required.
func packArchive(t *testing.T, stubPath, entryName, assetName string) ([]byte, string) {
	t.Helper()
	stubBytes, err := os.ReadFile(stubPath)
	if err != nil {
		t.Fatalf("read stub %s: %v", stubPath, err)
	}
	var buf bytes.Buffer
	if strings.HasSuffix(assetName, ".zip") {
		zw := zip.NewWriter(&buf)
		// A README entry the extractor MUST skip (proves "extract ONLY the stagecoach entry").
		if w, err := zw.Create("README.md"); err != nil {
			t.Fatalf("zip create README: %v", err)
		} else if _, err := w.Write([]byte("this must not be extracted\n")); err != nil {
			t.Fatalf("zip write README: %v", err)
		}
		// The stagecoach binary entry at the archive root.
		fh := &zip.FileHeader{Name: entryName, Method: zip.Deflate}
		fh.SetMode(0o755)
		w, err := zw.CreateHeader(fh)
		if err != nil {
			t.Fatalf("zip create %s: %v", entryName, err)
		}
		if _, err := w.Write(stubBytes); err != nil {
			t.Fatalf("zip write %s: %v", entryName, err)
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
		if err := tw.WriteHeader(&tar.Header{Name: entryName, Mode: 0o755, Size: int64(len(stubBytes))}); err != nil {
			t.Fatalf("tar write %s header: %v", entryName, err)
		}
		if _, err := tw.Write(stubBytes); err != nil {
			t.Fatalf("tar write %s: %v", entryName, err)
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

// archiveServer returns an httptest.Server that serves archiveBytes at /archive and the checksums
// body at /checksums. DownloadFile/FetchChecksums use ABSOLUTE DownloadURLs, so the test's fake
// Release carries the full httptest URLs. The checksums body is built here from the expected sha +
// asset name so a caller can pass a BOGUS sha to simulate tampering.
func archiveServer(t *testing.T, archiveBytes []byte, checksumsBody string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/archive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = io.Copy(w, bytes.NewReader(archiveBytes))
	})
	mux.HandleFunc("/checksums", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, checksumsBody)
	})
	return httptest.NewServer(mux)
}

// fakeRelease builds a Release with the archive + checksums assets pointing at the httptest server,
// plus a &Client{Repo: "o/r"} (HTTP nil ⇒ DefaultClient; BaseURL unused for asset downloads).
func fakeRelease(tag, assetNm, baseURL string) (Release, *Client) {
	rel := Release{
		Tag: tag,
		Assets: []Asset{
			{Name: assetNm, DownloadURL: baseURL + "/archive"},
			{Name: checksumsName(tag), DownloadURL: baseURL + "/checksums"},
		},
	}
	return rel, &Client{Repo: "owner/repo"}
}

// saveExecVersion returns the current execVersion seam (defer restoreExecVersion(saved) to undo an
// override). Centralizes the save/restore so each overriding test can't forget the defer.
func saveExecVersion() func(context.Context, string) ([]byte, error) {
	return execVersion
}

// restoreExecVersion puts back a previously-saved execVersion seam.
func restoreExecVersion(saved func(context.Context, string) ([]byte, error)) {
	execVersion = saved
}

// TestStageNewBinary_HappyPath is the end-to-end success case: a REAL compiled stubcli packed into
// the host-native archive, served over httptest, downloaded + SHA256-verified + extracted, then
// sanity-run with the DEFAULT exec (the stubcli inherits os.Environ incl. the t.Setenv OUT=tag).
func TestStageNewBinary_HappyPath(t *testing.T) {
	// Not run in parallel — the file comment explains the execVersion seam is shared state.
	stub := buildStubCLI(t)
	tag := "v1.2.3"
	assetNm := hostAssetName(tag)
	archive, sha := packArchive(t, stub, hostEntryName(), assetNm)
	checksumsBody := fmt.Sprintf("%s  %s\n", sha, assetNm)
	ts := archiveServer(t, archive, checksumsBody)
	defer ts.Close()

	rel, c := fakeRelease(tag, assetNm, ts.URL)
	tempDir := t.TempDir()

	// The stubcli prints STAGECOACH_STUBCLI_OUT and exits STAGECOACH_STUBCLI_EXIT (unset ⇒ 0).
	// The DEFAULT execVersion leaves cmd.Env unset ⇒ the child inherits os.Environ + this value.
	t.Setenv("STAGECOACH_STUBCLI_OUT", tag)

	newBinPath, err := StageNewBinary(context.Background(), c, rel, rel.Assets[0], tempDir)
	if err != nil {
		t.Fatalf("StageNewBinary happy path: unexpected error: %v", err)
	}

	want := filepath.Join(tempDir, "new-stagecoach"+exeSuffix())
	if newBinPath != want {
		t.Errorf("newBinPath = %q; want %q", newBinPath, want)
	}
	// The staged binary exists and is executable on unix.
	fi, err := os.Stat(newBinPath)
	if err != nil {
		t.Fatalf("stat staged binary %s: %v", newBinPath, err)
	}
	if runtime.GOOS != "windows" && fi.Mode()&0o100 == 0 {
		t.Errorf("staged binary %s is not executable (mode %s)", newBinPath, fi.Mode())
	}
	// The downloaded archive is left for inspection (success-path cleanup is P1.M3.T2's job).
	if _, err := os.Stat(filepath.Join(tempDir, assetNm)); err != nil {
		t.Errorf("archive %s not left for inspection: %v", assetNm, err)
	}
}

// TestStageNewBinary_TamperedArchive proves a checksum mismatch aborts BEFORE extraction: the
// checksums body advertises a bogus sha so VerifySHA256 fails (ErrChecksumMismatch) and NO
// new-stagecoach file appears in tempDir. tempDir (with the downloaded archive) is left for inspection.
func TestStageNewBinary_TamperedArchive(t *testing.T) {
	// Not run in parallel — see the file comment for the shared-seam rationale.
	stub := buildStubCLI(t)
	tag := "v1.2.3"
	assetNm := hostAssetName(tag)
	archive, _ := packArchive(t, stub, hostEntryName(), assetNm)
	// A bogus 64-hex sha (all zeros) — VerifySHA256 will mismatch.
	bogus := strings.Repeat("0", 64)
	checksumsBody := fmt.Sprintf("%s  %s\n", bogus, assetNm)
	ts := archiveServer(t, archive, checksumsBody)
	defer ts.Close()

	rel, c := fakeRelease(tag, assetNm, ts.URL)
	tempDir := t.TempDir()

	_, err := StageNewBinary(context.Background(), c, rel, rel.Assets[0], tempDir)
	if err == nil {
		t.Fatal("StageNewBinary tampered archive: want error; got nil")
	}
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Errorf("errors.Is(err, ErrChecksumMismatch) = false; err = %v", err)
	}

	// CRITICAL: a verify failure MUST NOT extract. No new-stagecoach in tempDir.
	extracted := filepath.Join(tempDir, "new-stagecoach"+exeSuffix())
	if _, statErr := os.Stat(extracted); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("tampered archive was extracted: %s exists (statErr=%v)", extracted, statErr)
	}
	// The downloaded (tampered) archive IS left for inspection — StageNewBinary does no cleanup.
	if _, err := os.Stat(filepath.Join(tempDir, assetNm)); err != nil {
		t.Errorf("tampered archive %s not left for inspection: %v", assetNm, err)
	}
}

// TestStageNewBinary_WrongTag proves a binary that RUNS but MISREPORTS its version is never staged:
// extract succeeds, the sanity-run executes, but the --version output lacks release.Tag ⇒
// ErrSanityVersionMismatch. tempDir is left for inspection (the binary + archive stay).
func TestStageNewBinary_WrongTag(t *testing.T) {
	// Not run in parallel — this test overrides the shared execVersion seam.
	stub := buildStubCLI(t)
	tag := "v1.2.3"
	assetNm := hostAssetName(tag)
	archive, sha := packArchive(t, stub, hostEntryName(), assetNm)
	checksumsBody := fmt.Sprintf("%s  %s\n", sha, assetNm)
	ts := archiveServer(t, archive, checksumsBody)
	defer ts.Close()

	rel, c := fakeRelease(tag, assetNm, ts.URL)
	tempDir := t.TempDir()

	// Override execVersion to run the real extracted stubcli but inject STUBCLI_OUT="v9.9.9-wrong".
	// cmd.Env MUST prepend os.Environ() so the stub's env vars reach the child (see GOTCHA in PRP).
	saved := saveExecVersion()
	defer restoreExecVersion(saved)
	execVersion = func(ctx context.Context, path string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, path, "--version")
		cmd.Env = append(os.Environ(), "STAGECOACH_STUBCLI_OUT=v9.9.9-wrong")
		return cmd.Output()
	}

	_, err := StageNewBinary(context.Background(), c, rel, rel.Assets[0], tempDir)
	if err == nil {
		t.Fatal("StageNewBinary wrong tag: want error; got nil")
	}
	if !errors.Is(err, ErrSanityVersionMismatch) {
		t.Errorf("errors.Is(err, ErrSanityVersionMismatch) = false; err = %v", err)
	}
	// The extracted binary is left for inspection (sanity failed, but extract already happened).
	extracted := filepath.Join(tempDir, "new-stagecoach"+exeSuffix())
	if _, err := os.Stat(extracted); err != nil {
		t.Errorf("extracted binary %s not left for inspection after wrong-tag sanity failure: %v", extracted, err)
	}
}

// TestStageNewBinary_NonZeroExit proves a binary that exits non-zero is never staged: extract
// succeeds, the sanity-run runs but the subprocess exits 1 ⇒ exec.Output returns *ExitError ⇒
// ErrSanityRunFailed. tempDir is left for inspection.
func TestStageNewBinary_NonZeroExit(t *testing.T) {
	// Not run in parallel — this test overrides the shared execVersion seam.
	stub := buildStubCLI(t)
	tag := "v1.2.3"
	assetNm := hostAssetName(tag)
	archive, sha := packArchive(t, stub, hostEntryName(), assetNm)
	checksumsBody := fmt.Sprintf("%s  %s\n", sha, assetNm)
	ts := archiveServer(t, archive, checksumsBody)
	defer ts.Close()

	rel, c := fakeRelease(tag, assetNm, ts.URL)
	tempDir := t.TempDir()

	// Override execVersion to run the real extracted stubcli but inject STUBCLI_EXIT=1 (non-zero).
	saved := saveExecVersion()
	defer restoreExecVersion(saved)
	execVersion = func(ctx context.Context, path string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, path, "--version")
		// Print the tag first (so a naive tag check would pass) then exit 1 ⇒ exec error wins.
		cmd.Env = append(os.Environ(), "STAGECOACH_STUBCLI_OUT="+tag, "STAGECOACH_STUBCLI_EXIT=1")
		return cmd.Output()
	}

	_, err := StageNewBinary(context.Background(), c, rel, rel.Assets[0], tempDir)
	if err == nil {
		t.Fatal("StageNewBinary non-zero exit: want error; got nil")
	}
	if !errors.Is(err, ErrSanityRunFailed) {
		t.Errorf("errors.Is(err, ErrSanityRunFailed) = false; err = %v", err)
	}
	// The extracted binary is left for inspection.
	extracted := filepath.Join(tempDir, "new-stagecoach"+exeSuffix())
	if _, err := os.Stat(extracted); err != nil {
		t.Errorf("extracted binary %s not left for inspection after non-zero-exit sanity failure: %v", extracted, err)
	}
}
