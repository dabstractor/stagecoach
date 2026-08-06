// download.go adds asset selection + download + SHA256 verification to the upgrade package — the
// second half of stagecoach's SOLE network surface (§19 named exception; §9.29 FR-U5 steps 3–4 +
// FR-U11). It fetches ONLY the project's own release archive + checksums.txt — never credentials,
// never a diff, never repo data — and, like releases.go, writes nothing into the commit core
// (FR-U12: imports no internal/* package and touches no lock/repo/provider/index/ref). On any
// failure between the download and the verify it removes the partial staging file so a tampered or
// truncated archive can never linger for a caller to accidentally extract (FR-U11 staging analog;
// the real never-overwrite-the-running-binary gate is downstream in P1.M3.T2). Token resolution
// from the environment remains the command layer's job (P1.M4.T1.S1); here Token is a plain Client
// field so the unit tests stay env-independent. Stdlib-only — adds ZERO go.mod requires.
package upgrade

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Sentinel errors for asset selection + download + verification. Each is wrapped with %w at its
// use site so errors.Is reaches the sentinel AND the concrete message survives. This mirrors
// internal/git.ErrCASFailed's convention (§13.5 detection via errors.Is). ErrHTTP (from
// releases.go) is REUSED for transport/HTTP-status failures — there is exactly one network
// surface, so exactly one network sentinel.
var (
	// ErrNoMatchingAsset is returned when the release has no archive for the requested
	// GOOS/GOARCH (computed goreleaser name not found among release.Assets).
	ErrNoMatchingAsset = errors.New("upgrade: no release asset matches the target GOOS/GOARCH")

	// ErrNoChecksumsFile is returned when the release has no *_checksums.txt asset.
	ErrNoChecksumsFile = errors.New("upgrade: release has no checksums.txt asset")

	// ErrChecksumMissing is returned when the selected archive has no line in checksums.txt.
	ErrChecksumMissing = errors.New("upgrade: selected asset has no entry in checksums.txt")

	// ErrChecksumParse is returned when checksums.txt contains a malformed line (not exactly
	// "<64hex>  <filename>" with a valid 64-char hex digest).
	ErrChecksumParse = errors.New("upgrade: checksums.txt has a malformed line")

	// ErrChecksumMismatch is returned when the downloaded archive's SHA256 differs from the
	// checksums.txt line. Open/read I/O failures in VerifySHA256 return a plain wrapped error
	// instead, so callers can branch "tampered" from "couldn't read the file".
	ErrChecksumMismatch = errors.New("upgrade: downloaded archive SHA256 does not match checksums.txt")
)

// assetName computes the goreleaser archive name for (goos, goarch): the tag with its leading "v"
// stripped (goreleaser {{.Version}} = tag without leading v; tag v1.0.0 → "1.0.0"), then
// "stagecoach_<v>_<os>_<arch>" + ".zip" (windows) or ".tar.gz" (everything else). This matches
// .goreleaser.yaml's archives[].name_template + format_overrides exactly.
func assetName(tag, goos, goarch string) string {
	ver := strings.TrimPrefix(tag, "v")
	name := "stagecoach_" + ver + "_" + goos + "_" + goarch
	if goos == "windows" {
		return name + ".zip"
	}
	return name + ".tar.gz"
}

// checksumsName computes the checksums file name: "stagecoach_<v-without-v>_checksums.txt",
// matching .goreleaser.yaml's checksum.name_template.
func checksumsName(tag string) string {
	return "stagecoach_" + strings.TrimPrefix(tag, "v") + "_checksums.txt"
}

// isHex64 reports whether s is exactly 64 hex characters ([0-9a-fA-F]). It is the validity gate
// for a checksums.txt digest field before it is stored in the map.
func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

// SelectAsset returns the goreleaser platform archive for (goos, goarch) from release.Assets. It
// computes the expected name (stagecoach_<Tag-without-v>_<os>_<arch>.tar.gz, or .zip on windows)
// and exact-matches it against an Asset.Name. It is pure (no network, no I/O). FR-U5 step 3.
func SelectAsset(release Release, goos, goarch string) (Asset, error) {
	want := assetName(release.Tag, goos, goarch)
	for _, a := range release.Assets {
		if a.Name == want {
			return a, nil
		}
	}
	return Asset{}, fmt.Errorf("select asset %s/%s (want %s): %w", goos, goarch, want, ErrNoMatchingAsset)
}

// VerifySHA256 streams SHA256 over the file at path and constant-time-compares the hex digest to
// want. want is normalized (trimmed + lowercased) so an uppercase or tabby checksum from a
// hand-edited file still matches. A digest mismatch yields ErrChecksumMismatch; an open or read
// I/O failure yields a plain wrapped error (NOT the sentinel) so callers can distinguish
// "tampered/failed-verify" from "couldn't read the file". FR-U5 step 4.
func VerifySHA256(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("sha256 open %s: %w", path, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("sha256 read %s: %w", path, err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	wantNorm := strings.ToLower(strings.TrimSpace(want))
	if subtle.ConstantTimeCompare([]byte(got), []byte(wantNorm)) != 1 {
		return fmt.Errorf("sha256 %s: got %s want %s: %w", path, got, wantNorm, ErrChecksumMismatch)
	}
	return nil
}

// newDownloadReq builds a GET against the ABSOLUTE url (a browser_download_url that 302-redirects
// to objects.githubusercontent.com, which net/http follows automatically). It sets the SAME headers
// as releases.go's newReq — User-Agent: stagecoach/<ver> and, when a token is configured,
// Authorization: Bearer <t> — so asset downloads benefit from auth/rate-limit and a real UA.
// BaseURL is NOT prepended (it is metadata-only). The context is attached for cancellation/timeouts.
func (c *Client) newDownloadReq(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "stagecoach/"+currentVersion) // currentVersion is same-package (version.go).
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return req, nil
}

// DownloadFile streams url to dest byte-for-byte (multi-MB safe — it never buffers the whole body
// in memory). Non-2xx responses and transport-layer failures (DNS, refused, server closed, context
// deadline) all map to ErrHTTP — the same surface as the metadata layer in releases.go. On a copy
// or close error the partial dest file is removed so a truncated download never lingers.
func (c *Client) DownloadFile(ctx context.Context, url, dest string) error {
	req, err := c.newDownloadReq(ctx, url)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, ErrHTTP)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %v: %w", url, err, ErrHTTP)
	}
	defer func() {
		// Drain + close so the connection returns to the pool (net/http requires a full drain for reuse).
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download %s: status %d: %w", url, resp.StatusCode, ErrHTTP)
	}
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("download create %s: %w", dest, err)
	}
	// STREAM to disk — do not io.ReadAll here (archives are multi-MB). On a copy or close error
	// remove the partial file so a truncated download never lingers.
	_, copyErr := io.Copy(f, resp.Body)
	cerr := f.Close()
	if copyErr != nil {
		_ = os.Remove(dest)
		return fmt.Errorf("download %s: %v: %w", url, copyErr, ErrHTTP)
	}
	if cerr != nil {
		_ = os.Remove(dest)
		return fmt.Errorf("download close %s: %w", dest, cerr)
	}
	return nil
}

// FetchChecksums finds the *_checksums.txt asset in release, downloads it, and parses each
// "<64hex>  <filename>" line into map[filename]hexsum. The checksums.txt body is small text, so it
// is buffered (io.ReadAll) — unlike DownloadFile, streaming is unnecessary here. No checksums asset
// yields ErrNoChecksumsFile; a malformed line (not exactly 2 fields, or a non-64-hex digest) yields
// ErrChecksumParse; a transport/status failure yields ErrHTTP.
func (c *Client) FetchChecksums(ctx context.Context, release Release) (map[string]string, error) {
	// Find the checksums asset: prefer the exact goreleaser name; fall back to any *_checksums.txt
	// asset (covers a future template tweak without false-matching an unrelated file).
	wantName := checksumsName(release.Tag)
	var url string
	found := false
	for _, a := range release.Assets {
		if a.Name == wantName || strings.HasSuffix(a.Name, "_checksums.txt") {
			url = a.DownloadURL
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("release %s: %w", release.Tag, ErrNoChecksumsFile)
	}

	req, err := c.newDownloadReq(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("checksums %s: %w", url, ErrHTTP)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("checksums %s: %v: %w", url, err, ErrHTTP)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("checksums %s: status %d: %w", url, resp.StatusCode, ErrHTTP)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("checksums read %s: %v: %w", url, err, ErrHTTP)
	}

	// Parse "<64hex>  <filename>" lines. strings.Fields collapses any whitespace run to fields, so
	// the two-space vs one-space separator ambiguity is irrelevant. Require exactly 2 fields and a
	// valid 64-hex digest; store lowercased so later compares against a lowercased file digest match.
	sums := make(map[string]string)
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || !isHex64(fields[0]) {
			return nil, fmt.Errorf("checksums line %q: %w", line, ErrChecksumParse)
		}
		sums[fields[1]] = strings.ToLower(fields[0])
	}
	return sums, nil
}

// DownloadAndVerifyArchive composes SelectAsset → FetchChecksums → DownloadFile → VerifySHA256 and
// returns the verified archive path under destDir. On ANY failure after it has created dest it
// os.Removes the partial file (FR-U11 staging analog: no tampered/truncated bytes linger for the
// caller to accidentally extract). destDir must already exist and is caller-owned (a staging temp
// dir); only the archive file itself is this method's responsibility. FR-U5 steps 3–4 + FR-U11.
func (c *Client) DownloadAndVerifyArchive(ctx context.Context, release Release, goos, goarch, destDir string) (string, error) {
	asset, err := SelectAsset(release, goos, goarch)
	if err != nil {
		return "", err
	}
	sums, err := c.FetchChecksums(ctx, release)
	if err != nil {
		return "", err
	}
	want, ok := sums[asset.Name]
	if !ok {
		return "", fmt.Errorf("asset %q not in checksums.txt: %w", asset.Name, ErrChecksumMissing)
	}
	dest := filepath.Join(destDir, asset.Name)
	if err := c.DownloadFile(ctx, asset.DownloadURL, dest); err != nil {
		_ = os.Remove(dest)
		return "", err
	}
	if err := VerifySHA256(dest, want); err != nil {
		_ = os.Remove(dest)
		return "", err
	}
	return dest, nil
}
