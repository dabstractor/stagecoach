name: "P1.M1.T3.S1 — GitHub Releases metadata client (latest / prerelease-list / pinned) with optional token auth + typed error mapping"
description: >
  Add internal/upgrade/releases.go to the EXISTING package upgrade (joins version.go from P1.M1.T1.S2).
  Exports a Client{HTTP, BaseURL, Repo, Token} that fetches GitHub release metadata — stagecoach's
  SOLE network surface (PRD §19 named exception; §9.29 FR-U5 step 2 / FR-U6). Three methods:
  LatestStable (GET /releases/latest), LatestAdmittingPrereleases (GET /releases array → filter
  draft==false → max-by-upgrade.Compare), ReleaseByTag (GET /releases/tags/{tag}). Exports Release
  {Tag, Assets []Asset{ Name, DownloadURL string; Size int64 }}. Three sentinel errors
  (ErrNoReleases, ErrRateLimited, ErrHTTP) checked via errors.Is, mirroring internal/git.ErrCASFailed:
  404→ErrNoReleases, 403/429→ErrRateLimited (hint: set a token), other/transport→ErrHTTP. Sets
  User-Agent: stagecoach/<ver> (Go default UA sometimes rejected) and Authorization: Bearer <t> when
  Token set. Stdlib-only (net/http + encoding/json) — NO new go.mod dependency. Token is a FIELD (this
  package reads NO env — P1.M4 resolves STAGECOACH_GITHUB_TOKEN/GITHUB_TOKEN). Tests via httptest.
  Mode-A doc: enrich version.go's package doc (the sole package doc) + a file comment in releases.go.

---

## Goal

**Feature Goal**: Provide the release-metadata fetch primitive the self-update feature resolves its
target version through (FR-U5 step 2, FR-U6). It is the **sole network surface** in stagecoach
(PRD §19) and it touches **only** the project's own release tags + asset URLs + sizes — never
credentials, diffs, or repo data. It is walled off from the commit core (FR-U12: no lock, no repo, no
provider, no index/ref) and performs **no** filesystem write (so FR-U11 abort-before-write is
trivially upheld — failures return clean typed errors).

**Deliverable**: One new production file `internal/upgrade/releases.go` and one new test file
`internal/upgrade/releases_test.go`, both in `package upgrade`. Plus a one-paragraph enrichment of the
existing `internal/upgrade/version.go` package doc (Mode A) noting this is the sole network surface +
the §19 scoping. No other file changes. No new go.mod require.

**Success Definition**:
- `Client{BaseURL: ts.URL, Repo:"o/r"}.LatestStable(ctx)` returns a `Release` whose `Tag`/`Assets`
  match a canned GitHub JSON payload from an `httptest.Server`.
- `LatestStable`/`ReleaseByTag` on HTTP 404 → `errors.Is(err, ErrNoReleases)`.
- `LatestStable` on HTTP 403 and 429 → `errors.Is(err, ErrRateLimited)` (message includes a "set a
  token" hint).
- Any other non-2xx (e.g. 500) and transport failures (server closed) → `errors.Is(err, ErrHTTP)`.
- `LatestAdmittingPrereleases` over a list containing a draft, a stable `v1.9.0`, and a prerelease
  `v2.0.0-rc1` returns `Tag=="v2.0.0-rc1"` (draft excluded; `Compare` admits the higher-core
  prerelease). Empty list / all-drafts → `ErrNoReleases`.
- A request with `Token:"xyz"` carries header `Authorization: Bearer xyz`; with `Token:""` it omits
  the header. Every request carries `User-Agent: stagecoach/<ver>` (never the Go default
  `Go-http-client`).
- `go build ./...`, `go vet ./internal/upgrade/...`, `go test ./internal/upgrade/...` all green;
  `gofmt -l` clean; go.mod unchanged (no new require).

## User Persona (if applicable)

**Target User**: The `stagecoach upgrade` command (P1.M4.T1.S1) and the asset/download subtask
(P1.M1.T3.S2), which consume this client to (a) resolve the latest/pinned target release tag and
(b) read the asset `browser_download_url`/`size` to select and verify the platform archive.

**Use Case**: `stagecoach upgrade --check` calls `LatestStable(ctx)` to get the newest stable tag,
then `upgrade.Compare(latestTag, currentTag)` (P1.M1.T1.S2) to decide up-to-date-vs-behind (exit 6).
`--version v1.2.3` calls `ReleaseByTag(ctx, "v1.2.3")`; `--prerelease` calls
`LatestAdmittingPrereleases(ctx)`.

**Pain Points Addressed**: Gives the self-update feature its one, contained, testable network seam —
injectable `*http.Client` + `BaseURL` so every code path is unit-testable against an `httptest` fake
(no real GitHub call, no flakiness, no rate-limit in CI). Typed errors let the command layer print
precise hints (rate-limited → "set a token") rather than opaque failures.

## Why

- **FR-U5 step 2 / FR-U6** (§9.29): "Resolve target version from the GitHub Releases API (default
  repo `dabstractor/stagecoach`…)"; `--check` resolves current-vs-latest. This client IS that API call.
- **§19 named exception** (PRD §19 "Diff content is local"): the commit path makes no network calls;
  `stagecoach upgrade` is the sole, explicit exception, fetching only the project's own release
  artifacts. This client is the implementation of that exception, and its doc must say so.
- **Foundation milestone (P1.M1) primitive #4**: after exit-code 6 (T1.S1), semver Compare (T1.S2),
  and the `[upgrade]` config seam (T2.S1), this is the network primitive. The download+SHA256 subtask
  (T3.S2), the direct-swap path (M3), and the command surface (M4) all build on it.
- **Bounded scope**: metadata-only (tag + asset URL/size). NO asset download, NO checksum parsing,
  NO Compare-vs-current, NO CLI flag, NO env read, NO file write — all deferred to siblings. Pure
  fetch + decode + typed errors.

## What

**User-visible behavior**: none directly (no caller until P1.M4.T1.S1). The artifact is a reusable,
testable HTTP client + the `Release`/`Asset` value types + three sentinel errors.

**Technical change**:

```go
// internal/upgrade/releases.go (NEW) — package upgrade

// Sentinel errors (errors.Is-compatible; mirror internal/git.ErrCASFailed convention).
var (
    ErrNoReleases  = errors.New("upgrade: no releases found")
    ErrRateLimited = errors.New("upgrade: GitHub rate limited")
    ErrHTTP        = errors.New("upgrade: HTTP request failed")
)

type Asset struct {           // exported — consumed by P1.M1.T3.S2 (download) + P1.M4
    Name        string
    DownloadURL string
    Size        int64
}
type Release struct {         // exported — consumed by the command layer + Compare
    Tag    string
    Assets []Asset
}

type Client struct {          // exported; contract field set exactly
    HTTP    *http.Client      // nil ⇒ http.DefaultClient
    BaseURL string            // "" ⇒ "https://api.github.com"; tests inject httptest.Server.URL
    Repo    string            // "owner/repo"
    Token   string            // "" ⇒ unauthenticated; else "Authorization: Bearer <Token>"
}

func (c *Client) LatestStable(ctx context.Context) (Release, error)
func (c *Client) LatestAdmittingPrereleases(ctx context.Context) (Release, error)
func (c *Client) ReleaseByTag(ctx context.Context, tag string) (Release, error)
```

### Success Criteria
- [ ] `releases.go` defines `Client`, `Release`, `Asset`, and the three sentinels exactly as above.
- [ ] `LatestStable` → GET `/repos/{Repo}/releases/latest`; 2xx decoded; 404→ErrNoReleases;
      403/429→ErrRateLimited; else/transport→ErrHTTP.
- [ ] `LatestAdmittingPrereleases` → GET `/repos/{Repo}/releases` (array); filter `draft==false`;
      pick max tag by `Compare` (same package); empty/all-draft→ErrNoReleases.
- [ ] `ReleaseByTag(tag)` → GET `/repos/{Repo}/releases/tags/{tag}`; 2xx decoded; 404→ErrNoReleases;
      403/429→ErrRateLimited; else/transport→ErrHTTP. Empty/invalid `tag` ⇒ ErrHTTP (or validation
      error) — never an unescaped path segment.
- [ ] Every request sets `User-Agent: stagecoach/<ver>` (ver from same-package `currentVersion`); a
      non-empty `Token` adds `Authorization: Bearer <Token>`.
- [ ] `releases_test.go` (white-box `package upgrade`) covers the full test matrix (see Validation
      Level 2) against an `httptest.Server`, asserting typed errors via `errors.Is`.
- [ ] `version.go` package doc enriched with one §19 paragraph (Mode A); `releases.go` has a
      file-level comment (blank-line-separated from `package upgrade` — NOT a competing package doc).
- [ ] `go build ./...` + `go vet ./internal/upgrade/...` + `go test ./internal/upgrade/...` green;
      `gofmt -l internal/upgrade/` clean; go.mod unchanged.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact struct/field/method signatures, the exact endpoint paths + JSON shapes, the exact
status→error mapping, the exact `Compare` signature to reuse (same package, version.go:105), the
sentinel-error convention to mirror (`internal/git.ErrCASFailed`), the §19 quote for the doc, the full
httptest test matrix with canned payloads, the stdlib-only constraint, and the explicit scope fences
(no env, no download, no Compare-vs-current).

### Documentation & References

```yaml
# MUST READ — the authoritative API shapes + error-mapping contract
- docfile: plan/017_397abce9deb1/architecture/external_deps.md
  why: "§1 gives the three endpoints, the JSON shape (tag_name/assets[].{name,browser_download_url,
        size}/prerelease/draft), the auth + User-Agent requirements, and the 404/403-429/other error
        mapping. This file IS the API spec the code must match."
  critical: "User-Agent is REQUIRED (Go default Go-http-client is sometimes rejected). /releases/latest
             ALREADY excludes prereleases+drafts (do not re-filter). The /releases array INCLUDES drafts
             (filter draft==false) and prereleases (admit them, pick highest semver)."

# MUST READ — the sibling you join + the Compare primitive you reuse
- file: internal/upgrade/version.go
  why: "releases.go is ADDED to this package. Compare(a,b string) int (line 105) is reused by
        LatestAdmittingPrereleases — it is full semver precedence (prerelease-aware, §11.4). The
        package doc ALSO lives here (lines 1-16) — enrich it (Mode A); do NOT add a competing one in
        releases.go. var currentVersion (line 21) feeds the User-Agent."
  pattern: "package upgrade; stdlib-only imports (errors, strconv, strings). Comment style cites PRD
            FR-xxx and §xx sections. White-box tests in version_test.go."
  gotcha: "Compare returns 0 if either operand is unparseable (defense-in-depth so dev builds never
           falsely signal an update). For LatestAdmittingPrereleases this means garbage tags compare
           equal — the first survivor is retained; never an error. Do NOT reimplement comparison."

# MUST READ — the §19 exception (the doc deliverable's source quote)
- prd: §19
  why: "Exact scoping language for the package-doc enrichment: 'Stagecoach's commit-generation path
        (§9.1–§9.28) makes no network calls itself; the sole exception is stagecoach upgrade (§9.29),
        which fetches the project's own release artifacts and checksums from GitHub Releases — never
        provider credentials, never a diff, never repo data.'"
- prd: §9.29 FR-U5 (step 2), FR-U6, FR-U11, FR-U12
  why: "FR-U5 step 2 = resolve target from the Releases API; FR-U6 = --check resolves current vs
        latest; FR-U11 = abort-before-write (this subtask writes nothing — trivially upheld); FR-U12 =
        walled off from the commit core (no lock/repo/provider/index-ref)."

# MUST READ — the typed-error convention to mirror
- file: internal/git/updateref.go
  why: "ErrCASFailed sentinel + errors.Is(err, ErrCASFailed) is THE codebase pattern for typed errors.
        Mirror it: `var ErrXxx = errors.New(...)`, wrap concrete with fmt.Errorf(\"...: %w\", ErrXxx)
        so both the sentinel AND the status/message are reachable."
- file: internal/config/load.go
  why: "The codebase's fmt.Errorf(\"context: %w\", err) wrapping style (used pervasively). Match it for
        the message prefixes in releases.go (e.g. 'upgrade: latest stable: %w')."

# CONTEXT — the upstream config seam this client consumes (token/repo resolution is the command's job)
- docfile: plan/017_397abce9deb1/P1M1T2S1/PRP.md
  why: "Defines config.LoadUpgradeConfig() → UpgradeConfig{Channel, SourceRepo} (global-only). The
        command layer (P1.M4.T1.S1) reads SourceRepo → Client.Repo and resolves STAGECOACH_GITHUB_TOKEN
        → GITHUB_TOKEN → Client.Token. THIS package takes them as FIELDS and reads NO env."

# LIBRARY — stdlib, no doc fetch needed; anchor for the API contract
- url: https://docs.github.com/en/rest/releases/releases#get-the-latest-release
  why: "Confirms GET /repos/{owner}/{repo}/releases/latest returns the most recent non-prerelease,
        non-draft release (404 if none); GET /releases returns all (incl. drafts/prereleases);
        GET /releases/tags/{tag} returns one (404 unknown tag). JSON fields: tag_name, prerelease,
        draft, assets[].{name,browser_download_url,size}. 403/429 = rate-limited."
  critical: "Verify at implementation the live field names still match (FR-D5 discipline — record the
             date). external_deps.md §1 is the in-repo authoritative copy; this URL is corroboration."
```

### Current Codebase tree (relevant slice)

```bash
internal/upgrade/
  version.go        # EXISTS (P1.M1.T1.S2) — package doc lives here; Compare(a,b) @105; var currentVersion @21. <- EDIT doc only (Mode A)
  version_test.go   # EXISTS — white-box package upgrade, table-driven. Mirror style.
  releases.go       # DOES NOT EXIST — CREATE (Client + Release/Asset + 3 sentinels + 3 methods)
  releases_test.go  # DOES NOT EXIST — CREATE (httptest fake server, full matrix)
go.mod              # module github.com/dabstractor/stagecoach; go 1.22. UNCHANGED (net/http+encoding/json stdlib).
# NOTE: net/http is used NOWHERE else in the repo (verified) — this is the first/sole HTTP surface.
```

### Desired Codebase tree with files to be added

```bash
internal/upgrade/
  version.go        # +1 paragraph in the package doc (Mode A: sole network surface + §19 scoping)
  releases.go       # NEW — Client/Release/Asset + ErrNoReleases/ErrRateLimited/ErrHTTP + 3 methods
  releases_test.go  # NEW — httptest-driven, white-box package upgrade
go.mod              # UNCHANGED (no new require)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (one package doc, not two): the package doc lives in version.go. Do NOT write a
//   "// Package upgrade ..." comment in releases.go — that creates a DUPLICATE package doc (golint/
//   revive flag it). Satisfy Mode A by APPENDING one §19 paragraph to version.go's existing package
//   doc, and give releases.go a file comment separated from `package upgrade` by a blank line.

// CRITICAL (stdlib-only posture): version.go's doc explicitly commits to "no external semver
//   dependency." releases.go must add ZERO go.mod requires. net/http, encoding/json, context, errors,
//   fmt, io, strings are all stdlib. Do NOT pull in a GitHub SDK or a REST client library.

// CRITICAL (this package reads NO env): Token is a Client FIELD. Do NOT call os.Getenv for
//   STAGECOACH_GITHUB_TOKEN / GITHUB_TOKEN here — that is the command layer's job (P1.M4.T1.S1). The
//   boundary keeps this package pure/testable (no env coupling in unit tests).

// CRITICAL (FR-U12 walled off): this package MUST NOT import internal/git, internal/generate,
//   internal/lock, internal/provider, internal/config, or cmd/*. It fetches metadata only. No repo,
//   no lock, no provider, no index/ref, no file write (FR-U11 trivially upheld — there is no write).

// GOTCHA (drain the body for keep-alive): Go's net/http reuses connections only if the response body
//   is fully drained AND closed. After json.Decode/io.ReadAll, do `defer func(){ io.Copy(io.Discard,
//   resp.Body); resp.Body.Close() }()`. Skipping this leaks connections (matters for repeated calls).

// GOTCHA (/latest already excludes prereleases+drafts): GitHub's /releases/latest returns the most
//   recent NON-prerelease NON-draft release and 404s if none exists. LatestStable must NOT list+filter
//   — just decode the single object (simpler + more correct). Only LatestAdmittingPrereleases lists.

// GOTCHA (Compare is prerelease-aware): upgrade.Compare("v2.0.0-rc1","v1.9.0") == +1 (core 2>1).
//   So max-by-Compare over [stable v1.9.0, prerelease v2.0.0-rc1] correctly yields v2.0.0-rc1 — that
//   IS the "latest admitting prereleases." Do not special-case prereleases; Compare handles §11.4.

// GOTCHA (tag path escaping): ReleaseByTag interpolates the user-supplied tag into the URL path.
//   Use url.PathEscape(tag) (or build via url.URL{Path}) so a tag containing odd chars cannot break
//   the request line. An empty tag should be rejected (ErrHTTP or a validation error) before the call.

// GOTCHA (status mapping is exhaustive): every non-2xx falls into exactly one bucket — 404 →
//   ErrNoReleases; 403|429 → ErrRateLimited; everything else (incl. 401, 5xx, transport errors) →
//   ErrHTTP. Wrap so errors.Is reaches the sentinel AND the message carries status + truncated body:
//   fmt.Errorf("latest stable: %w: %s", ErrHTTP, strings.TrimSpace(string(body))).
```

## Implementation Blueprint

### Data models and structure

```go
// internal/upgrade/releases.go (NEW) — see "What" for the exported types; the decode twins below are
// UNEXPORTED (GitHub's raw JSON names mapped to stagecoach-idiomatic exported field names).

// ghRelease / ghAsset decode the raw GitHub JSON; toRelease() maps to the exported value types.
// Keeping them unexported means a future GitHub field rename touches one converter, not the callers.
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
func (r ghRelease) toRelease() Release {
	a := make([]Asset, 0, len(r.Assets))
	for _, x := range r.Assets {
		a = append(a, Asset{Name: x.Name, DownloadURL: x.BrowserDownloadURL, Size: x.Size})
	}
	return Release{Tag: r.TagName, Assets: a}
}

const defaultBaseURL = "https://api.github.com"
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/upgrade/version.go — enrich the package doc (Mode A), ONE paragraph
  - LOCATE the package doc (lines 1-16; the "All implementation is stdlib-only ..." paragraph).
  - APPEND a new paragraph (BEFORE `package upgrade`) stating:
      "releases.go adds the package's network surface — the GitHub Releases metadata client used by
       stagecoach upgrade to resolve a target release (FR-U5 step 2, FR-U6). It is the SOLE network
       call in stagecoach: §19 scopes 'no network calls' to the commit path (§9.1–§9.28) and names
       stagecoach upgrade as the explicit exception, which fetches ONLY the project's own release
       tags, asset URLs, and asset sizes — never credentials, never a diff, never repo data. Like the
       rest of the package it is stdlib-only (net/http + encoding/json)."
  - DO NOT change any code in version.go — doc only. (P1.M1.T1.S2 is Complete; its tests are unaffected.)
  - GOTCHA: this is the package doc — releases.go must NOT add a competing "// Package upgrade" block.

Task 2: CREATE internal/upgrade/releases.go — types, sentinels, Client, helpers
  - PACKAGE: `package upgrade` (same as version.go). IMPORTS: context, encoding/json, errors, fmt, io,
    net/http, net/url, strings (ALL stdlib).
  - DEFINE the three sentinels (var block) + Release + Asset + Client structs EXACTLY as in "What".
  - DEFINE ghRelease/ghAsset (unexported) + toRelease() + const defaultBaseURL (blueprint above).
  - IMPLEMENT unexported helpers:
      - (c *Client) httpClient() *http.Client     // returns c.HTTP or http.DefaultClient
      - (c *Client) baseURL() string              // returns c.BaseURL or defaultBaseURL (trim trailing /)
      - (c *Client) newReq(ctx, method, path) (*http.Request, error)
            // http.NewRequestWithContext(ctx, method, c.baseURL()+path, nil); set
            //   req.Header.Set("Accept", "application/vnd.github+json")
            //   req.Header.Set("User-Agent", "stagecoach/"+currentVersion)   // currentVersion is same-package
            //   if c.Token != "" { req.Header.Set("Authorization", "Bearer "+c.Token) }
      - (c *Client) do(ctx, path) (body []byte, error)
            // req := newReq(...); resp, err := c.httpClient().Do(req);
            //   if err != nil → return nil, fmt.Errorf("GET %s: %w", path, ErrHTTP) wrapped around err? 
            //   — SEE ERROR-MAPPING NOTE: transport err → ErrHTTP; drain+close body; switch resp.StatusCode:
            //     200-299 → return io.ReadAll(resp.Body) (drained)
            //     404     → ErrNoReleases
            //     403,429 → ErrRateLimited (+ "set STAGECOACH_GITHUB_TOKEN" hint)
            //     other   → ErrHTTP (with status + trimmed body snippet)
  - ERROR-MAPPING NOTE: for the sentinel to be errors.Is-reachable AND carry a message, wrap as
      fmt.Errorf("latest stable: %w", ErrNoReleases)  (sentinel only, no extra msg)  OR
      fmt.Errorf("rate limited (set STAGECOACH_GITHUB_TOKEN): %w", ErrRateLimited)   OR
      fmt.Errorf("status %d: %s: %w", code, snippet, ErrHTTP).
    Transport errors (resp==nil, err!=nil): fmt.Errorf("GET %s: %v: %w", path, err, ErrHTTP).
  - IMPLEMENT the three methods (thin wrappers over do + decode):
      LatestStable(ctx):     body,err := c.do(ctx, "/repos/"+c.Repo+"/releases/latest"); var r ghRelease;
                             json.Unmarshal(body,&r); return r.toRelease(), nil.  (do maps 404→ErrNoReleases)
      ReleaseByTag(ctx,tag): if tag=="" → return ErrHTTP/validation; path-escape tag;
                             body,err := c.do(ctx, "/repos/"+c.Repo+"/releases/tags/"+url.PathEscape(tag));
                             decode single ghRelease; return toRelease().
      LatestAdmittingPrereleases(ctx): body,err := c.do(ctx, "/repos/"+c.Repo+"/releases"); decode []ghRelease;
                             survivors := filter draft==false; if len==0 → ErrNoReleases;
                             best := survivors[0]; for _, r := range survivors[1:] {
                                 if Compare(r.TagName, best.TagName) > 0 { best = r }   // reuse upgrade.Compare
                             }; return best.toRelease().
  - NAMING: exported Client/Release/Asset; exported LatestStable/LatestAdmittingPrereleases/ReleaseByTag;
            unexported ghRelease/ghAsset/newReq/do/httpClient/baseURL. camelCase methods (Go convention).
  - DO NOT: read os.Getenv; import internal/git|generate|lock|provider|config|cmd; download assets;
            call Compare against the running version (that's the command's --check job); write files.

Task 3: CREATE internal/upgrade/releases_test.go — httptest fake-server matrix
  - PACKAGE: `package upgrade` (white-box — so currentVersion/Compare are reachable; matches version_test.go).
  - PATTERN: a helper newFakeClient(t) that starts an httptest.NewServer with a handler switching on
    r.URL.Path (and recording r.Header for the auth/UA assertions), and returns a Client{BaseURL:
    ts.URL, Repo:"owner/repo"} + the server (+ t.Cleanup(ts.Close)).
  - CASES (one t.Run each; canned JSON via a helper returning a GitHub-shaped string):
      * TestClient_LatestStable_OK            → 200 latest; assert Tag + Assets parsed (Name/Size/URL).
      * TestClient_LatestStable_404_NoReleases→ 404; errors.Is(err, ErrNoReleases).
      * TestClient_LatestStable_403_RateLimited→ 403; errors.Is(err, ErrRateLimited); msg has "token".
      * TestClient_LatestStable_429_RateLimited→ 429; errors.Is(err, ErrRateLimited).
      * TestClient_LatestStable_500_HTTP       → 500; errors.Is(err, ErrHTTP); msg has "500".
      * TestClient_Prerelease_PicksHighest     → array [draft v3.0.0(draft), stable v1.9.0, pre v2.0.0-rc1]
                                                 → Tag=="v2.0.0-rc1" (draft excluded; Compare admits pre).
      * TestClient_Prerelease_Empty_NoReleases → []; ErrNoReleases.
      * TestClient_Prerelease_AllDrafts_NoReleases → [draft,draft]; ErrNoReleases.
      * TestClient_ReleaseByTag_OK             → 200 /tags/v1.2.3; assert Tag.
      * TestClient_ReleaseByTag_404_NoReleases → 404; ErrNoReleases.
      * TestClient_AuthHeader                  → Token="xyz": recorded Authorization == "Bearer xyz";
                                                 Token="": header absent.
      * TestClient_UserAgent                   → recorded UA startsWith "stagecoach/" (NOT "Go-http-client").
      * TestClient_TransportFailure_HTTP       → ts.Close() then call → errors.Is(err, ErrHTTP).
      * (optional) TestClient_ContextCanceled  → cancel ctx → error (errors.Is(ctx.Err()) reachable).
  - ASSERTIONS: use errors.Is for ALL typed-error cases; plain == for Tag/Asset fields.
  - DEPENDENCIES: Task 2. Follow version_test.go's table-driven + t.Run style where natural.

Task 4: VERIFY — build, vet, format, targeted + full upgrade tests, go.mod guard
  - go build ./...
  - go vet ./internal/upgrade/...
  - gofmt -l internal/upgrade/releases.go internal/upgrade/releases_test.go internal/upgrade/version.go
  - go test ./internal/upgrade/... -run 'Client_' -v        # the new matrix
  - go test ./internal/upgrade/... -v                       # version_test.go must still pass (doc edit)
  - go test ./...                                           # whole-module no-regression
  - git diff --stat go.mod go.sum                           # EMPTY (stdlib only — no new require)
  - scope guard: grep that releases.go imports no internal/* (see Validation Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN — the do() status mapper (the heart of the error contract). Every non-2xx lands in exactly
// one bucket; the sentinel is wrapped so errors.Is reaches it AND the message carries detail.
func (c *Client) do(ctx context.Context, path string) ([]byte, error) {
	req, err := c.newReq(ctx, http.MethodGet, path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, ErrHTTP) // request-build failure → ErrHTTP
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %v: %w", path, err, ErrHTTP) // transport failure → ErrHTTP
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	switch code := resp.StatusCode; {
	case code >= 200 && code < 300:
		return body, nil
	case code == 404:
		return nil, fmt.Errorf("%s: %w", path, ErrNoReleases)
	case code == 403 || code == 429:
		return nil, fmt.Errorf("%s: rate limited (set STAGECOACH_GITHUB_TOKEN to raise the quota): %w", path, ErrRateLimited)
	default:
		snip := strings.TrimSpace(string(body))
		if len(snip) > 200 { snip = snip[:200] }
		return nil, fmt.Errorf("%s: status %d: %s: %w", path, code, snip, ErrHTTP)
	}
}

// PATTERN — LatestAdmittingPrereleases reuses same-package upgrade.Compare (NOT a reimplementation).
func (c *Client) LatestAdmittingPrereleases(ctx context.Context) (Release, error) {
	body, err := c.do(ctx, "/repos/"+c.Repo+"/releases")
	if err != nil { return Release{}, err }
	var rs []ghRelease
	if err := json.Unmarshal(body, &rs); err != nil {
		return Release{}, fmt.Errorf("decode releases: %v: %w", err, ErrHTTP)
	}
	best := -1
	for i, r := range rs {
		if r.Draft { continue }                 // drafts always excluded (require auth; not real releases)
		if best < 0 || Compare(r.TagName, rs[best].TagName) > 0 { best = i }
	}
	if best < 0 {
		return Release{}, fmt.Errorf("/repos/%s/releases: %w", c.Repo, ErrNoReleases)
	}
	return rs[best].toRelease(), nil
}

// PATTERN — httptest fake client (releases_test.go). BaseURL injectable → no real network in CI.
func newFakeClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return &Client{BaseURL: ts.URL, Repo: "owner/repo"}
}
// handler switches on r.URL.Path and r.Header; writes canned JSON / status. Records headers via a
// closure-captured map for the Auth/UA assertions.
```

### Integration Points

```yaml
PACKAGE (internal/upgrade): joins version.go (same package). Exports Client, Release, Asset, the three
  sentinels, and the three methods. Reuses upgrade.Compare (version.go:105) and var currentVersion (UA).

NETWORK (sole surface): GET https://api.github.com/repos/{owner}/{repo}/releases[/latest|/tags/{tag}].
  Headers: Accept: application/vnd.github+json; User-Agent: stagecoach/<ver>; Authorization: Bearer <t>?
  BaseURL injectable (httptest in tests). No redirects involved at the metadata layer.

CONFIG/ENV BOUNDARY: NONE in this package. Client.Repo + Client.Token are set by the caller (the
  command layer P1.M4.T1.S1: Repo ← config.LoadUpgradeConfig().SourceRepo / --source-repo; Token ←
  STAGECOACH_GITHUB_TOKEN → GITHUB_TOKEN). This package calls os.Getenv NOWHERE.

NO REGISTRATION: no cobra command, no flag, no env var declared here (all P1.M4). No go.mod change.

CONSUMERS (downstream, not this task): P1.M1.T3.S2 (asset+checksum download reads Release.Assets);
  P1.M3.T1.S1 (resolve target release); P1.M4.T1/T2 (command + --check uses Compare vs LatestStable).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Go has no ruff/mypy — gofmt + go vet are the gates. Run after creating each file.
gofmt -w internal/upgrade/releases.go internal/upgrade/releases_test.go internal/upgrade/version.go
go vet ./internal/upgrade/...
go build ./...
gofmt -l internal/upgrade/   # Expected: empty (all formatted).
# Expected: zero errors. If gofmt -l lists a file, re-run `gofmt -w` on it. If vet errors, read + fix.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new httptest matrix, verbose.
go test ./internal/upgrade/... -run 'Client_' -v
# Expected: all t.Run subtests PASS:
#   LatestStable_OK / _404_NoReleases / _403_RateLimited / _429_RateLimited / _500_HTTP
#   Prerelease_PicksHighest (v2.0.0-rc1 over stable v1.9.0; draft excluded) / _Empty_NoReleases / _AllDrafts_NoReleases
#   ReleaseByTag_OK / _404_NoReleases
#   AuthHeader (Bearer xyz present / absent) / UserAgent (stagecoach/…, not Go-http-client)
#   TransportFailure_HTTP (closed server → ErrHTTP)
# All typed-error asserts use errors.Is(err, ErrNoReleases|ErrRateLimited|ErrHTTP).

# The whole upgrade package — version_test.go (P1.M1.T1.S2) must stay green after the doc edit.
go test ./internal/upgrade/... -v
# Expected: all PASS (the version.go change is doc-only; no behavioral regression).
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole module (an additive package file shouldn't ripple, but verify).
go test ./...
# Race detector on the new package (http.Client is concurrency-safe; cheap insurance on the fake server).
go test -race ./internal/upgrade/...
# Coverage (no hard gate for internal/upgrade per §20.3, but keep it high).
go test -cover ./internal/upgrade/...
# Expected: all packages PASS; new package well-covered (all 3 methods × status branches + helpers).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Scope guard: releases.go imports NO internal/* (FR-U12 walled off — no git/generate/lock/provider/config/cmd).
! grep -qE '"github.com/dabstractor/stagecoach/internal/' internal/upgrade/releases.go && echo "OK: no internal imports (walled off)"

# Env guard: releases.go reads NO env (Token is a field; P1.M4 owns env resolution).
! grep -q 'os.Getenv' internal/upgrade/releases.go && echo "OK: no os.Getenv (Token is a Client field)"

# Dep guard: no new go.mod require (stdlib only).
git diff --stat go.mod go.sum   # Expected: empty
! grep -q 'require' <(git diff go.mod) && echo "OK: go.mod unchanged"

# Doc guard (Mode A): version.go's package doc mentions the §19 sole-network-surface exception.
grep -n 'sole.*network\|§19\|named exception' internal/upgrade/version.go   # Expected: ≥1 hit in the package doc

# Doc guard (no competing package doc): releases.go must NOT start a second "// Package upgrade" block.
! grep -B1 'package upgrade' internal/upgrade/releases.go | grep -q 'Package upgrade' && echo "OK: no duplicate package doc"

# Error-contract guard: all three sentinels exist and are wrapped with %w so errors.Is reaches them.
grep -n 'ErrNoReleases\|ErrRateLimited\|ErrHTTP' internal/upgrade/releases.go   # Expected: declared + referenced
grep -c '%w' internal/upgrade/releases.go   # Expected: ≥3 (each sentinel is wrapped at its use site)

# API-shape guard: the three endpoint paths are present and correct.
grep -n 'releases/latest\|/releases"\|releases/tags/' internal/upgrade/releases.go   # Expected: all three
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` succeeds
- [ ] `go vet ./internal/upgrade/...` clean
- [ ] `gofmt -l internal/upgrade/` empty
- [ ] `go test ./internal/upgrade/...` green (new matrix + version_test.go regression)
- [ ] `go test ./...` green (no ripple)
- [ ] `go.mod`/`go.sum` unchanged (stdlib only)

### Feature Validation
- [ ] `LatestStable` decodes `/releases/latest`; 404→ErrNoReleases; 403/429→ErrRateLimited; 500/transport→ErrHTTP
- [ ] `LatestAdmittingPrereleases` filters draft==false, picks max-by-`Compare` (v2.0.0-rc1 > v1.9.0); empty/all-draft→ErrNoReleases
- [ ] `ReleaseByTag(tag)` decodes `/releases/tags/{url.PathEscape(tag)}`; 404→ErrNoReleases; empty tag rejected
- [ ] Token set ⇒ `Authorization: Bearer <t>`; unset ⇒ header absent
- [ ] Every request ⇒ `User-Agent: stagecoach/<ver>` (never `Go-http-client`)
- [ ] All typed errors reachable via `errors.Is(err, ErrNoReleases|ErrRateLimited|ErrHTTP)`

### Scope-Boundary Validation
- [ ] releases.go imports no `internal/*` (FR-U12 walled off)
- [ ] releases.go calls no `os.Getenv` (Token is a field; P1.M4 owns env)
- [ ] No asset download / no checksum parsing (sibling P1.M1.T3.S2)
- [ ] No `Compare`-vs-running-version logic (command's `--check` job, P1.M4)
- [ ] No cobra command / flag / env declaration (P1.M4.T1.S1)
- [ ] No file write (FR-U11 trivially upheld — pure metadata fetch)

### Documentation & Quality
- [ ] version.go package doc enriched with the §19 sole-network-surface paragraph (Mode A)
- [ ] releases.go has a file-level comment (blank-line-separated from `package upgrade` — NOT a competing package doc)
- [ ] Comments cite §19, §9.29 FR-U5/U6/U11/U12 where relevant
- [ ] Follows codebase conventions: sentinel errors mirror `internal/git.ErrCASFailed`; `fmt.Errorf("…: %w")` wrapping; white-box table-driven tests mirror version_test.go
- [ ] Decode twins (`ghRelease`/`ghAsset`) are unexported; exported types use stagecoach-idiomatic names

---

## Anti-Patterns to Avoid

- ❌ **Don't add a second `// Package upgrade` doc in releases.go.** The package doc lives in version.go; a competing block triggers a duplicate-package-doc lint. Enrich version.go's existing doc (Mode A) and give releases.go a blank-line-separated file comment.
- ❌ **Don't reimplement semver comparison.** Reuse `upgrade.Compare` (same package, version.go:105) for `LatestAdmittingPrereleases`. It is already prerelease-aware (§11.4); special-casing prereleases would duplicate and likely mis-order them.
- ❌ **Don't read `os.Getenv` for the token.** `Token` is a `Client` field; the command layer (P1.M4.T1.S1) resolves `STAGECOACH_GITHUB_TOKEN`/`GITHUB_TOKEN`. Env coupling here would make the unit tests env-dependent.
- ❌ **Don't list+filter for `LatestStable`.** GitHub's `/releases/latest` already returns the most recent non-prerelease non-draft release (and 404s if none). Just decode the single object — simpler and more correct.
- ❌ **Don't skip the body drain/close.** `io.Copy(io.Discard, resp.Body); resp.Body.Close()` (in a defer) is required for net/http connection reuse; omitting it leaks connections across repeated calls.
- ❌ **Don't map transport errors (resp==nil) to a fourth sentinel.** The contract has exactly three; transport failures → `ErrHTTP` (wrapped with the underlying error message), so the error surface stays {ErrNoReleases, ErrRateLimited, ErrHTTP}.
- ❌ **Don't interpolate the tag into the URL unescaped.** Use `url.PathEscape(tag)` (or build via `url.URL`) so an odd tag can't break the request line; reject an empty tag before the call.
- ❇️ **Don't pull in a GitHub SDK / REST library.** The package is stdlib-only (`net/http` + `encoding/json`), matching version.go's "no external dependency" posture. A SDK would add a go.mod require and a supply-chain surface for a 3-endpoint client.
- ❌ **Don't import `internal/git`/`generate`/`lock`/`provider`/`config`/`cmd`.** FR-U12: this package is walled off from the commit core. It fetches metadata only — no repo, no lock, no provider, no index/ref.
- ❌ **Don't download assets or parse checksums here.** That is sibling P1.M1.T3.S2. This subtask returns the asset `Name`/`DownloadURL`/`Size`; it does not fetch the bytes or the checksums.txt.
- ❌ **Don't call `Compare(latestTag, currentTag)` here.** That decision is the command layer's `--check` job (P1.M4). This package only FETCHES (and, for the prerelease channel, SELECTS the highest tag from the list) — it does not compare against the running version.

---

## Confidence Score

**9/10** — one-pass success likelihood. The contract pins the struct field set, the three endpoints,
the JSON shape, and the status→error mapping verbatim; the `Compare` primitive to reuse is in the same
package with a known signature and prerelease-aware semantics; the typed-error convention has an
in-repo precedent (`internal/git.ErrCASFailed`); the test matrix is fully enumerated with canned
payloads and an `httptest` helper; and the package is stdlib-only with no existing HTTP pattern to
reverse-engineer (greenfield, but Go-idiomatic). The −1 covers two judgment calls the implementer must
nail: (1) enriching version.go's package doc rather than creating a competing one (lint gotcha —
explicitly fenced), and (2) the `do()` transport-error → `ErrHTTP` wrapping choice (the contract names
three sentinels and leaves transport mapping implied — resolved here as ErrHTTP with the underlying
message wrapped). Both are flagged with grep guards in Validation Level 4.