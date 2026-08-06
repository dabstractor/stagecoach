package upgrade

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sha256hex returns the lowercase hex SHA256 of b — the same digest VerifySHA256 computes.
func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// randBytes builds a deterministic n-byte slice (deterministic so failures are reproducible).
func randBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i % 251) // 251 is prime; cycles slowly enough to be non-trivial.
	}
	return b
}

// archiveNameFor computes the goreleaser archive name for a v1.2.3 release — mirrors assetName so
// the tests stay coupled to the goreleaser contract via the same derivation the code uses.
func archiveNameFor(goos, goarch string) string {
	return assetName("v1.2.3", goos, goarch)
}

// newDownloadServer spins up an httptest.Server that serves the canned archive bytes at
// /dl/<archiveName> and a checksums.txt line at /dl/<csumName>. The checksums line advertises the
// SHA256 of archive (the happy path). The server is closed via t.Cleanup.
func newDownloadServer(t *testing.T, archive []byte, archiveName, csumName string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/dl/"+archiveName, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/dl/"+csumName, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", sha256hex(archive), archiveName)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// buildRelease returns a Release{Tag:v1.2.3} whose two assets (archive + checksums) point at the
// given server. Callers may edit Assets (e.g. drop the checksums asset for a NoAsset case).
func buildRelease(server *httptest.Server, archiveName, csumName string) Release {
	return Release{
		Tag: "v1.2.3",
		Assets: []Asset{
			{Name: archiveName, DownloadURL: server.URL + "/dl/" + archiveName, Size: 0},
			{Name: csumName, DownloadURL: server.URL + "/dl/" + csumName, Size: 0},
		},
	}
}

// v123ChecksumsName is the goreleaser checksums filename for a v1.2.3 release.
func v123ChecksumsName() string { return checksumsName("v1.2.3") }

// ---------------------------------------------------------------------------
// Pure: SelectAsset
// ---------------------------------------------------------------------------

func TestSelectAsset(t *testing.T) {
	rel := Release{Tag: "v1.2.3", Assets: []Asset{
		{Name: "stagecoach_1.2.3_linux_amd64.tar.gz"},
		{Name: "stagecoach_1.2.3_linux_arm64.tar.gz"},
		{Name: "stagecoach_1.2.3_darwin_amd64.tar.gz"},
		{Name: "stagecoach_1.2.3_darwin_arm64.tar.gz"},
		{Name: "stagecoach_1.2.3_windows_amd64.zip"},
		{Name: "stagecoach_1.2.3_windows_arm64.zip"},
		{Name: "stagecoach_1.2.3_checksums.txt"},
	}}

	tests := []struct {
		name     string
		goos     string
		goarch   string
		wantName string
		wantErr  error
	}{
		{"darwin/arm64 tar.gz", "darwin", "arm64", "stagecoach_1.2.3_darwin_arm64.tar.gz", nil},
		{"darwin/amd64 tar.gz", "darwin", "amd64", "stagecoach_1.2.3_darwin_amd64.tar.gz", nil},
		{"linux/amd64 tar.gz", "linux", "amd64", "stagecoach_1.2.3_linux_amd64.tar.gz", nil},
		{"linux/arm64 tar.gz", "linux", "arm64", "stagecoach_1.2.3_linux_arm64.tar.gz", nil},
		{"windows/amd64 zip", "windows", "amd64", "stagecoach_1.2.3_windows_amd64.zip", nil},
		{"windows/arm64 zip", "windows", "arm64", "stagecoach_1.2.3_windows_arm64.zip", nil},
		{"unknown arch no match", "linux", "mips", "", ErrNoMatchingAsset},
		{"unknown os no match", "solaris", "amd64", "", ErrNoMatchingAsset},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, err := SelectAsset(rel, tc.goos, tc.goarch)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("SelectAsset(%s/%s) err = %v, want errors.Is %v", tc.goos, tc.goarch, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("SelectAsset(%s/%s) unexpected err: %v", tc.goos, tc.goarch, err)
			}
			if a.Name != tc.wantName {
				t.Errorf("SelectAsset(%s/%s).Name = %q, want %q", tc.goos, tc.goarch, a.Name, tc.wantName)
			}
		})
	}
}

func TestSelectAsset_TagWithoutV(t *testing.T) {
	// Some callers construct Release with a Tag that already lacks the leading "v". TrimPrefix is a
	// no-op on such a tag, so the name must still resolve to the goreleaser form (no double-strip).
	rel := Release{Tag: "1.2.3", Assets: []Asset{
		{Name: "stagecoach_1.2.3_linux_amd64.tar.gz"},
	}}
	a, err := SelectAsset(rel, "linux", "amd64")
	if err != nil {
		t.Fatalf("SelectAsset tag-without-v: unexpected err: %v", err)
	}
	if a.Name != "stagecoach_1.2.3_linux_amd64.tar.gz" {
		t.Errorf("Name = %q, want stagecoach_1.2.3_linux_amd64.tar.gz", a.Name)
	}
}

// ---------------------------------------------------------------------------
// Pure: VerifySHA256
// ---------------------------------------------------------------------------

func TestVerifySHA256(t *testing.T) {
	content := []byte("the quick brown fox jumps over the lazy dog")
	dir := t.TempDir()
	path := filepath.Join(dir, "blob")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	correct := sha256hex(content)
	wrong := sha256hex([]byte("different bytes"))

	t.Run("correct lower hex", func(t *testing.T) {
		if err := VerifySHA256(path, correct); err != nil {
			t.Errorf("VerifySHA256(correct) = %v, want nil", err)
		}
	})
	t.Run("correct upper hex normalized", func(t *testing.T) {
		// Uppercase + surrounding whitespace exercises the trim+lower normalization in VerifySHA256.
		upperHex := " " + strings.ToUpper(correct) + " "
		if err := VerifySHA256(path, upperHex); err != nil {
			t.Errorf("VerifySHA256(UPPER) = %v, want nil (normalized)", err)
		}
	})
	t.Run("wrong hex mismatch", func(t *testing.T) {
		err := VerifySHA256(path, wrong)
		if !errors.Is(err, ErrChecksumMismatch) {
			t.Errorf("VerifySHA256(wrong) = %v, want errors.Is ErrChecksumMismatch", err)
		}
	})
	t.Run("missing file io error not mismatch", func(t *testing.T) {
		// A nonexistent file must NOT surface ErrChecksumMismatch — reserve that sentinel for the
		// digest comparison only, so callers can branch "tampered" from "couldn't read".
		err := VerifySHA256(filepath.Join(dir, "nope"), correct)
		if err == nil {
			t.Fatal("VerifySHA256(missing) = nil, want error")
		}
		if errors.Is(err, ErrChecksumMismatch) {
			t.Errorf("VerifySHA256(missing) = %v, must NOT be ErrChecksumMismatch", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Network: DownloadFile
// ---------------------------------------------------------------------------

func TestClient_DownloadFile_OK(t *testing.T) {
	want := randBytes(4096)
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write(want)
	}))
	t.Cleanup(ts.Close)

	dest := filepath.Join(t.TempDir(), "blob")
	c := &Client{}
	if err := c.DownloadFile(context.Background(), ts.URL+"/dl/blob", dest); err != nil {
		t.Fatalf("DownloadFile: unexpected err: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("downloaded bytes differ from served bytes (len got=%d want=%d)", len(got), len(want))
	}
	if gotUA == "" || !startsWith(gotUA, "stagecoach/") {
		t.Errorf("User-Agent = %q, want prefix stagecoach/", gotUA)
	}
}

func startsWith(s, prefix string) bool { return len(s) >= len(prefix) && s[:len(prefix)] == prefix }

func TestClient_DownloadFile_500_HTTP(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server boom"))
	}))
	t.Cleanup(ts.Close)

	dest := filepath.Join(t.TempDir(), "blob")
	c := &Client{}
	err := c.DownloadFile(context.Background(), ts.URL+"/dl/blob", dest)
	if !errors.Is(err, ErrHTTP) {
		t.Errorf("DownloadFile 500 err = %v, want errors.Is ErrHTTP", err)
	}
	// A 500 must not have created the dest file (response body not streamed to disk on non-2xx).
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("dest should not exist after non-2xx, statErr = %v", statErr)
	}
}

func TestClient_DownloadFile_Transport_HTTP(t *testing.T) {
	// Close the server first, then call — the request fails at the transport layer.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	c := &Client{}
	ts.Close()

	dest := filepath.Join(t.TempDir(), "blob")
	err := c.DownloadFile(context.Background(), ts.URL+"/dl/blob", dest)
	if !errors.Is(err, ErrHTTP) {
		t.Errorf("DownloadFile transport-failure err = %v, want errors.Is ErrHTTP", err)
	}
}

// ---------------------------------------------------------------------------
// Network: FetchChecksums
// ---------------------------------------------------------------------------

func TestClient_FetchChecksums_OK(t *testing.T) {
	archive := randBytes(2048)
	archName := archiveNameFor("linux", "amd64")
	csumName := v123ChecksumsName()
	ts := newDownloadServer(t, archive, archName, csumName)
	rel := buildRelease(ts, archName, csumName)

	// Rewrite the checksums handler to serve TWO lines so the multi-line parse is exercised.
	sum := sha256hex(archive)
	body := fmt.Sprintf("%s  %s\n%s  some-other-file.tar.gz\n", sum, archName, sha256hex([]byte("x")))
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dl/" + archName:
			_, _ = w.Write(archive)
		case "/dl/" + csumName:
			_, _ = w.Write([]byte(body))
		}
	})

	c := &Client{}
	sums, err := c.FetchChecksums(context.Background(), rel)
	if err != nil {
		t.Fatalf("FetchChecksums: unexpected err: %v", err)
	}
	if got := sums[archName]; got != sum {
		t.Errorf("sums[%s] = %q, want %q", archName, got, sum)
	}
	if _, ok := sums["some-other-file.tar.gz"]; !ok {
		t.Errorf("second line not parsed; sums = %#v", sums)
	}
}

func TestClient_FetchChecksums_NoAsset(t *testing.T) {
	// Release with NO *_checksums.txt asset.
	rel := Release{Tag: "v1.2.3", Assets: []Asset{
		{Name: "stagecoach_1.2.3_linux_amd64.tar.gz", DownloadURL: "http://example/x"},
	}}
	c := &Client{}
	_, err := c.FetchChecksums(context.Background(), rel)
	if !errors.Is(err, ErrNoChecksumsFile) {
		t.Errorf("FetchChecksums no-asset err = %v, want errors.Is ErrNoChecksumsFile", err)
	}
}

func TestClient_FetchChecksums_Malformed(t *testing.T) {
	archName := archiveNameFor("linux", "amd64")
	csumName := v123ChecksumsName()
	// Serve a checksums.txt with a malformed line (1 field).
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dl/" + csumName:
			fmt.Fprint(w, "lonelytoken\n") // 1 field → malformed
		}
	}))
	t.Cleanup(ts.Close)
	rel := Release{Tag: "v1.2.3", Assets: []Asset{
		{Name: archName, DownloadURL: ts.URL + "/dl/" + archName},
		{Name: csumName, DownloadURL: ts.URL + "/dl/" + csumName},
	}}
	c := &Client{}
	_, err := c.FetchChecksums(context.Background(), rel)
	if !errors.Is(err, ErrChecksumParse) {
		t.Errorf("FetchChecksums malformed err = %v, want errors.Is ErrChecksumParse", err)
	}
}

func TestClient_FetchChecksums_MalformedBadHex(t *testing.T) {
	archName := archiveNameFor("linux", "amd64")
	csumName := v123ChecksumsName()
	// Serve a checksums.txt whose digest is not 64 hex chars.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dl/" + csumName:
			fmt.Fprintf(w, "nothex  %s\n", archName) // 2 fields but bad hex → malformed
		}
	}))
	t.Cleanup(ts.Close)
	rel := Release{Tag: "v1.2.3", Assets: []Asset{
		{Name: archName, DownloadURL: ts.URL + "/dl/" + archName},
		{Name: csumName, DownloadURL: ts.URL + "/dl/" + csumName},
	}}
	c := &Client{}
	_, err := c.FetchChecksums(context.Background(), rel)
	if !errors.Is(err, ErrChecksumParse) {
		t.Errorf("FetchChecksums bad-hex err = %v, want errors.Is ErrChecksumParse", err)
	}
}

// ---------------------------------------------------------------------------
// Network: DownloadAndVerifyArchive (the composer)
// ---------------------------------------------------------------------------

func TestClient_DownloadAndVerifyArchive_OK(t *testing.T) {
	archive := randBytes(8192)
	archName := archiveNameFor("linux", "amd64")
	csumName := v123ChecksumsName()
	ts := newDownloadServer(t, archive, archName, csumName)
	rel := buildRelease(ts, archName, csumName)

	destDir := t.TempDir()
	c := &Client{}
	got, err := c.DownloadAndVerifyArchive(context.Background(), rel, "linux", "amd64", destDir)
	if err != nil {
		t.Fatalf("DownloadAndVerifyArchive: unexpected err: %v", err)
	}
	wantPath := filepath.Join(destDir, archName)
	if got != wantPath {
		t.Errorf("returned path = %q, want %q", got, wantPath)
	}
	f, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("read returned path: %v", err)
	}
	if !bytes.Equal(f, archive) {
		t.Errorf("archive bytes differ from served bytes")
	}
	// Independent re-verify against the checksums line's advertised sum.
	if err := VerifySHA256(got, sha256hex(archive)); err != nil {
		t.Errorf("re-verify failed: %v", err)
	}
}

func TestClient_DownloadAndVerifyArchive_Tampered(t *testing.T) {
	original := randBytes(8192)
	tampered := randBytes(8192)
	// Make sure they actually differ.
	tampered[0] = original[0] ^ 0xff
	archName := archiveNameFor("linux", "amd64")
	csumName := v123ChecksumsName()
	// The checksums.txt advertises the SHA256 of `original`, but the server serves `tampered` → mismatch.
	mux := http.NewServeMux()
	mux.HandleFunc("/dl/"+archName, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tampered)
	})
	mux.HandleFunc("/dl/"+csumName, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", sha256hex(original), archName)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	rel := buildRelease(ts, archName, csumName)

	destDir := t.TempDir()
	c := &Client{}
	got, err := c.DownloadAndVerifyArchive(context.Background(), rel, "linux", "amd64", destDir)
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("DownloadAndVerifyArchive tampered err = %v, want errors.Is ErrChecksumMismatch", err)
	}
	if got != "" {
		t.Errorf("returned path = %q, want empty on failure", got)
	}
	// FR-U11 staging analog: the partial dest file must have been removed.
	dest := filepath.Join(destDir, archName)
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("partial dest must be removed after verify failure; statErr = %v", statErr)
	}
}

func TestClient_DownloadAndVerifyArchive_MissingLine(t *testing.T) {
	archive := randBytes(2048)
	archName := archiveNameFor("darwin", "arm64")
	csumName := v123ChecksumsName()
	// checksums.txt lists a DIFFERENT file — the selected archive has no entry.
	mux := http.NewServeMux()
	mux.HandleFunc("/dl/"+archName, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/dl/"+csumName, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  some-other-archive.tar.gz\n", sha256hex(archive))
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	rel := buildRelease(ts, archName, csumName)

	destDir := t.TempDir()
	c := &Client{}
	_, err := c.DownloadAndVerifyArchive(context.Background(), rel, "darwin", "arm64", destDir)
	if !errors.Is(err, ErrChecksumMissing) {
		t.Errorf("DownloadAndVerifyArchive missing-line err = %v, want errors.Is ErrChecksumMissing", err)
	}
	// dest must not exist (download never started because the missing line is detected first).
	if _, statErr := os.Stat(filepath.Join(destDir, archName)); !os.IsNotExist(statErr) {
		t.Errorf("dest must not exist when the checksums line is missing; statErr = %v", statErr)
	}
}

func TestClient_DownloadAndVerifyArchive_NoAsset(t *testing.T) {
	archName := archiveNameFor("linux", "amd64")
	csumName := v123ChecksumsName()
	archive := randBytes(128)
	ts := newDownloadServer(t, archive, archName, csumName)
	rel := buildRelease(ts, archName, csumName)

	destDir := t.TempDir()
	c := &Client{}
	// Request an (os, arch) the release has no asset for.
	_, err := c.DownloadAndVerifyArchive(context.Background(), rel, "linux", "mips", destDir)
	if !errors.Is(err, ErrNoMatchingAsset) {
		t.Errorf("DownloadAndVerifyArchive no-asset err = %v, want errors.Is ErrNoMatchingAsset", err)
	}
}

// TestClient_DownloadFile_BearerToken asserts the optional Bearer header is set when Token is
// configured on the Client (same header contract as releases.go).
func TestClient_DownloadFile_BearerToken(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(ts.Close)

	c := &Client{Token: "abc123"}
	dest := filepath.Join(t.TempDir(), "blob")
	if err := c.DownloadFile(context.Background(), ts.URL+"/dl/blob", dest); err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	if gotAuth != "Bearer abc123" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer abc123")
	}
}
