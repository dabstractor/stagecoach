// releases.go implements the upgrade package's GitHub Releases metadata client — the SOLE network
// surface in stagecoach (PRD §19 named exception; §9.29 FR-U5 step 2 / FR-U6). It fetches ONLY the
// project's own release tags, asset URLs, and asset sizes — never credentials, never a diff, never
// repo data — and writes nothing to the filesystem (FR-U11 abort-before-write is trivially upheld
// because there is no write). It is walled off from the commit core (FR-U12: imports no internal/*
// package and touches no lock/repo/provider/index/ref). Token resolution from the environment is the
// command layer's job (P1.M4.T1.S1); here Token is a plain Client field so the unit tests stay
// env-independent. Stdlib-only (net/http + encoding/json); adds ZERO go.mod requires.
package upgrade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Sentinel errors for the release-metadata client. Each is wrapped with %w at its use site so
// errors.Is reaches the sentinel AND the concrete message (path / status / body snippet) survives.
// This mirrors internal/git.ErrCASFailed's convention (§13.5 detection via errors.Is).
var (
	// ErrNoReleases is returned when the endpoint yields no candidate release — HTTP 404 on
	// /releases/latest or /releases/tags/{tag}, or a /releases array that is empty / all-drafts.
	ErrNoReleases = errors.New("upgrade: no releases found")

	// ErrRateLimited is returned on HTTP 403 or 429. The wrapping message carries a hint to set
	// STAGECOACH_GITHUB_TOKEN (the command layer, P1.M4.T1.S1, resolves it from the environment).
	ErrRateLimited = errors.New("upgrade: GitHub rate limited")

	// ErrHTTP is returned for any other HTTP failure (401, 5xx, …) or a transport-layer error
	// (DNS, connection refused, server closed, context-deadline during the request, malformed URL).
	ErrHTTP = errors.New("upgrade: HTTP request failed")
)

// Asset is a single downloadable artifact attached to a release. It is consumed by the download +
// SHA256 subtask (P1.M1.T3.S2) and the command layer (P1.M4) to select and verify the platform
// archive. Field names are stagecoach-idiomatic; the raw GitHub JSON names are mapped in ghAsset.
type Asset struct {
	Name        string // GitHub "name" — e.g. "stagecoach_1.2.3_linux_amd64.tar.gz".
	DownloadURL string // GitHub "browser_download_url" — the objects.githubusercontent.com redirect.
	Size        int64  // GitHub "size" — bytes, for a pre-download sanity check.
}

// Release is the metadata view of a single GitHub release: its tag plus its downloadable assets.
// It does NOT carry the prerelease/draft flags (those are consumed server-side by /releases/latest
// or filtered locally for the prerelease channel) — callers need only the tag and asset URLs/sizes.
type Release struct {
	Tag    string  // GitHub "tag_name" — e.g. "v1.2.3"; fed to Compare (version.go) by --check.
	Assets []Asset // ordered as the API returns them; empty for a release with no artifacts.
}

// Client fetches GitHub Releases metadata. It is the sole network surface in stagecoach (§19). All
// fields are set by the caller (the command layer, P1.M4.T1.S1: Repo ← config.SourceRepo,
// Token ← STAGECOACH_GITHUB_TOKEN/GITHUB_TOKEN); this package reads NO environment variable.
// HTTP and BaseURL are injectable so every code path is unit-testable against an httptest fake.
type Client struct {
	HTTP    *http.Client // nil ⇒ http.DefaultClient.
	BaseURL string       // "" ⇒ "https://api.github.com"; tests inject an httptest.Server.URL.
	Repo    string       // "owner/repo" — interpolated into /repos/{Repo}/releases….
	Token   string       // "" ⇒ unauthenticated request; else adds "Authorization: Bearer <Token>".
}

// defaultBaseURL is the production GitHub REST API root. Overridden in tests via Client.BaseURL.
const defaultBaseURL = "https://api.github.com"

// ghRelease / ghAsset decode the raw GitHub JSON. They are unexported so a future GitHub field rename
// touches one converter (toRelease) rather than every caller. The exported Release/Asset types keep
// stagecoach-idiomatic names independent of GitHub's JSON naming.
type ghRelease struct {
	TagName    string    `json:"tag_name"`
	Prerelease bool      `json:"prerelease"`
	Draft      bool      `json:"draft"`
	Assets     []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// toRelease maps the raw GitHub decode twin to the exported Release/Asset value types. Assets is
// always non-nil (a release with no artifacts yields an empty slice, not nil) so callers can range
// without a nil check.
func (r ghRelease) toRelease() Release {
	a := make([]Asset, 0, len(r.Assets))
	for _, x := range r.Assets {
		a = append(a, Asset{Name: x.Name, DownloadURL: x.BrowserDownloadURL, Size: x.Size})
	}
	return Release{Tag: r.TagName, Assets: a}
}

// httpClient returns the configured client or the default — http.DefaultClient is concurrency-safe
// and follows redirects (asset download URLs redirect to objects.githubusercontent.com).
func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

// baseURL returns the configured API root, trimming any trailing slash so path concatenation always
// yields a single slash separator. Empty falls back to defaultBaseURL (production GitHub API).
func (c *Client) baseURL() string {
	if c.BaseURL == "" {
		return defaultBaseURL
	}
	return strings.TrimRight(c.BaseURL, "/")
}

// newReq builds a GET request against path with the headers GitHub requires: Accept (the canonical
// releases media type), User-Agent (Go's default "Go-http-client" is sometimes rejected — §1), and
// Authorization when a token is configured. The context is attached for cancellation/timeouts.
func (c *Client) newReq(ctx context.Context, method, path string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL()+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "stagecoach/"+currentVersion) // currentVersion is same-package (version.go).
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return req, nil
}

// do executes a GET against path and maps the response to the three-sentinel error contract:
// 2xx ⇒ body bytes; 404 ⇒ ErrNoReleases; 403/429 ⇒ ErrRateLimited (+token hint); everything else
// (incl. transport failures and request-build failures) ⇒ ErrHTTP. The body is fully drained and
// closed so net/http can reuse the underlying connection (keep-alive) across repeated calls.
func (c *Client) do(ctx context.Context, path string) ([]byte, error) {
	req, err := c.newReq(ctx, http.MethodGet, path)
	if err != nil {
		// Malformed URL / cancelled context before the call: surface as ErrHTTP (no transport happened).
		return nil, fmt.Errorf("%s: %v: %w", path, err, ErrHTTP)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		// Transport-layer failure (DNS, refused, server closed, context deadline during the request).
		// The contract names exactly three sentinels; transport failures fold into ErrHTTP.
		return nil, fmt.Errorf("%s: %v: %w", path, err, ErrHTTP)
	}
	defer func() {
		// Drain + close so the connection returns to the pool (net/http requires full drain for reuse).
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	body, _ := io.ReadAll(resp.Body)
	switch code := resp.StatusCode; {
	case code >= 200 && code < 300:
		return body, nil
	case code == 404:
		// /releases/latest 404s when no non-prerelease release exists; /tags/{tag} 404s on unknown tag.
		return nil, fmt.Errorf("%s: %w", path, ErrNoReleases)
	case code == 403 || code == 429:
		// Rate-limited. The hint names the env var the command layer resolves (P1.M4.T1.S1); this
		// package itself reads no env.
		return nil, fmt.Errorf("%s: rate limited (set STAGECOACH_GITHUB_TOKEN to raise the quota): %w", path, ErrRateLimited)
	default:
		// Any other non-2xx (401 unauthenticated-private, 5xx server, …). Carry status + a trimmed
		// body snippet (capped at 200 chars) for diagnostics without dumping a multi-KB error page.
		snip := strings.TrimSpace(string(body))
		if len(snip) > 200 {
			snip = snip[:200]
		}
		return nil, fmt.Errorf("%s: status %d: %s: %w", path, code, snip, ErrHTTP)
	}
}

// LatestStable resolves the most recent non-prerelease, non-draft release. GitHub's
// /releases/latest endpoint already excludes prereleases and drafts and 404s when none exists, so
// this method decodes the single returned object directly (no list+filter) — simpler and more
// correct than re-implementing GitHub's selection. FR-U5 step 2 / FR-U6.
func (c *Client) LatestStable(ctx context.Context) (Release, error) {
	body, err := c.do(ctx, "/repos/"+c.Repo+"/releases/latest")
	if err != nil {
		return Release{}, err
	}
	var r ghRelease
	if err := json.Unmarshal(body, &r); err != nil {
		return Release{}, fmt.Errorf("decode latest release: %v: %w", err, ErrHTTP)
	}
	return r.toRelease(), nil
}

// ReleaseByTag resolves the release for an exact tag (the --version <v> path, FR-U5 step 2). The tag
// is path-escaped so an odd character cannot break the request line; an empty tag is rejected before
// the call to avoid a malformed path. A 404 maps to ErrNoReleases (unknown tag).
func (c *Client) ReleaseByTag(ctx context.Context, tag string) (Release, error) {
	if strings.TrimSpace(tag) == "" {
		// Empty tag would yield a bare /releases/tags/ path (the array endpoint) — reject explicitly.
		return Release{}, fmt.Errorf("release by tag: empty tag: %w", ErrHTTP)
	}
	path := "/repos/" + c.Repo + "/releases/tags/" + url.PathEscape(tag)
	body, err := c.do(ctx, path)
	if err != nil {
		return Release{}, err
	}
	var r ghRelease
	if err := json.Unmarshal(body, &r); err != nil {
		return Release{}, fmt.Errorf("decode release by tag: %v: %w", err, ErrHTTP)
	}
	return r.toRelease(), nil
}

// LatestAdmittingPrereleases resolves the highest-precedence non-draft release from the full
// /releases array (the --prerelease channel, FR-U5 step 2). Drafts are always excluded (they require
// auth and are not real releases). Prereleases are admitted and ordered by the same-package
// upgrade.Compare (version.go:105) — which is full semver precedence, prerelease-aware (§11.4), so a
// v2.0.0-rc1 correctly outranks a stable v1.9.0. An empty array or an all-drafts array yields
// ErrNoReleases. Compare returns 0 for unparseable operands (dev-build defense), so garbage tags
// simply tie and never cause an error here.
func (c *Client) LatestAdmittingPrereleases(ctx context.Context) (Release, error) {
	body, err := c.do(ctx, "/repos/"+c.Repo+"/releases")
	if err != nil {
		return Release{}, err
	}
	var rs []ghRelease
	if err := json.Unmarshal(body, &rs); err != nil {
		return Release{}, fmt.Errorf("decode releases: %v: %w", err, ErrHTTP)
	}
	best := -1
	for i, r := range rs {
		if r.Draft {
			continue // drafts always excluded (require auth; not real releases).
		}
		if best < 0 || Compare(r.TagName, rs[best].TagName) > 0 {
			best = i
		}
	}
	if best < 0 {
		return Release{}, fmt.Errorf("/repos/%s/releases: %w", c.Repo, ErrNoReleases)
	}
	return rs[best].toRelease(), nil
}
