# Codebase Findings — P1.M3.T1.S1 (ResolveTarget + GOOS_GOARCH asset selection)

## 1. The dependency contracts (ALL LANDED — consume, don't rebuild)

### Client (internal/upgrade/releases.go:60) — the injectable network seam
```go
type Client struct {
    HTTP    *http.Client // nil ⇒ http.DefaultClient
    BaseURL string       // "" ⇒ "https://api.github.com"; tests inject httptest.Server.URL
    Repo    string       // "owner/repo"
    Token   string       // "" ⇒ unauthenticated
}
// The THREE target-resolution methods (all return (Release, error)):
func (c *Client) LatestStable(ctx) (Release, error)              // releases.go:178 — /releases/latest
func (c *Client) ReleaseByTag(ctx, tag string) (Release, error)  // releases.go:193 — /releases/tags/{tag}; rejects empty tag
func (c *Client) LatestAdmittingPrereleases(ctx) (Release, error)// releases.go:217 — /releases array, Compare-highest non-draft
```
All map HTTP failures to THREE exported sentinels: `ErrNoReleases` (404 / empty), `ErrRateLimited` (403/429),
`ErrHTTP` (everything else + transport). ResolveTarget PROPAGATES these as-is (no re-wrap) so errors.Is works.

### Release / Asset (releases.go:42,51)
```go
type Asset struct { Name, DownloadURL string; Size int64 }
type Release struct { Tag string; Assets []Asset }  // Assets always non-nil (empty slice, not nil)
```

### SelectAsset (download.go:93) — the asset picker (PURE; no network)
```go
func SelectAsset(release Release, goos, goarch string) (Asset, error)
// want = assetName(release.Tag, goos, goarch) = "stagecoach_<tag-no-v>_<os>_<arch>.tar.gz" (or .zip on windows)
// exact-matches Asset.Name; else returns Asset{}, fmt.Errorf("select asset %s/%s (want %s): %w", goos, goarch, want, ErrNoMatchingAsset)
```
`ErrNoMatchingAsset` is EXPORTED (download.go:35): `errors.New("upgrade: no release asset matches the target GOOS/GOARCH")`.

### Compare (version.go:112) — NOT USED by ResolveTarget (see §3 DECISION)
```go
func Compare(a, b string) int  // -1/0/+1; 0 if either unparseable
func CurrentSemver() (string, bool)
```

## 2. The CRITICAL scope decision — ResolveTarget does NOT call Compare

Contract point 3 reads: "Validate the target tag is newer than current via Compare (skip the 'already up to date'
case here OR in the command layer — DECISION: the --check/up-to-date determination is in the command layer
P1.M4.T2; ResolveTarget just returns the chosen release+asset)."

The parenthetical **DECISION OVERRIDES the "Validate" clause.** ResolveTarget does NOT call Compare and does NOT
gate on "already up to date" / "target not newer". It is a THIN composition: pick the right Client method per the
channel/version flags → SelectAsset → return (Release, Asset). The newer-than-current / up-to-date determination
(including `--check`'s exit-6-when-behind, exit-0-when-current) is the command layer's job (P1.M4.T2 runUpgrade),
which has Compare + CurrentSemver. Putting the check in ResolveTarget would (a) duplicate the command layer and
(b) force ResolveTarget to know the running version (a concern it shouldn't own). **ResolveTarget = fetch + select.**

## 3. The API design (resolve.go — NOT swap.go)

File name: **resolve.go**. The contract offers "swap.go (or resolve.go)" — but P1.M3.T2.S1 (Backup + atomic swap)
owns swap.go. Using swap.go here would conflict with that sibling. resolve.go is semantically correct (ResolveTarget
lives there; the on-disk swap is P1.M3.T2's swap.go).

```go
// resolve.go (package upgrade; FILE comment, not package doc — releases.go owns the package doc)

// ResolveOptions selects the target channel and the platform asset.
type ResolveOptions struct {
    Version    string // non-empty (--version <v>) ⇒ ReleaseByTag; PRECEDENCE over Prerelease
    Prerelease bool   // true (--prerelease) ⇒ LatestAdmittingPrereleases; only when Version==""
    GOOS       string // "" ⇒ runtime.GOOS (injectable for per-platform tests)
    GOARCH     string // "" ⇒ runtime.GOARCH (injectable for per-platform tests)
}

func ResolveTarget(ctx context.Context, c *Client, opts ResolveOptions) (Release, Asset, error) {
    release, err := resolveRelease(ctx, c, opts)
    if err != nil { return Release{}, Asset{}, err }
    goos, goarch := opts.GOOS, opts.GOARCH
    if goos == ""  { goos  = runtime.GOOS }
    if goarch == ""{ goarch = runtime.GOARCH }
    asset, err := SelectAsset(release, goos, goarch)
    if err != nil { return Release{}, Asset{}, err }
    return release, asset, nil
}

func resolveRelease(ctx context.Context, c *Client, opts ResolveOptions) (Release, error) {
    if v := strings.TrimSpace(opts.Version); v != "" {
        return c.ReleaseByTag(ctx, v)        // pinned version wins
    }
    if opts.Prerelease {
        return c.LatestAdmittingPrereleases(ctx)
    }
    return c.LatestStable(ctx)               // default channel
}
```
Imports (stdlib only): `context`, `runtime`, `strings`. NO fmt (errors propagate as-is from Client/SelectAsset).
NO internal/* (FR-U12 walled off). go.mod unchanged (module github.com/dabstractor/stagecoach).

### Precedence: Version > Prerelease > LatestStable
- `--version v1.2.3` (even with `--prerelease`) → ReleaseByTag("v1.2.3"). A pinned version is explicit; the channel is irrelevant.
- `--prerelease` (no --version) → LatestAdmittingPrereleases.
- (default) → LatestStable.

### Why GOOS/GOARCH are injectable (defaulting to runtime)
The contract says "SelectAsset(release, runtime.GOOS, runtime.GOARCH)" AND the test output requires "asset selection
per GOOS/GOARCH". To test linux/darwin/windows selection OFF the test host's platform, GOOS/GOARCH must be injectable.
Production passes empty ⇒ runtime.GOOS/GOARCH (the contract's runtime call). This matches the package's injectable-seam
convention (Client.HTTP/BaseURL, DelegateOptions.Env). The `assetName` helper (download.go, in-package) lets a test
build the runtime-platform asset name to pin the default-to-runtime path.

## 4. Error handling — propagate, don't re-wrap

- Client method fails (ErrNoReleases/ErrRateLimited/ErrHTTP) → return `(Release{}, Asset{}, err)` UNWRAPPED so
  `errors.Is(err, upgrade.ErrNoReleases)` etc. work in the command layer.
- SelectAsset fails (ErrNoMatchingAsset) → return `(Release{}, Asset{}, err)` UNWRAPPED.
- Return ZERO values on any error (standard Go; the SelectAsset error message already names the wanted asset +
  GOOS/GOARCH, so no diagnostic info is lost by zeroing the Release).

## 5. Test patterns to REUSE (internal/upgrade/releases_test.go — package upgrade, internal tests)

```go
// The httptest fake-client idiom (releases_test.go:15) — REUSE verbatim:
func newFakeClient(t *testing.T, handler http.HandlerFunc) *Client {
    ts := httptest.NewServer(handler); t.Cleanup(ts.Close)
    return &Client{BaseURL: ts.URL, Repo: "owner/repo"}
}
// Canned payloads (releases_test.go:24,38):
func cannedLatest() string   // single v1.2.3 with linux_amd64 + darwin_arm64 assets
func cannedReleases() string // array: v3.0.0(draft), v1.9.0(stable), v2.0.0-rc1(pre) — pre wins after draft excluded
func statusServer(status int, body string) http.HandlerFunc
```
`assetName(tag, goos, goarch)` (download.go, in-package, unexported) is callable from the test to build expected names.

## 6. Scope fences (S1 vs siblings)

- **S1 (THIS)**: resolve.go — ResolveTarget + ResolveOptions + resolveRelease + tests. The direct-swap RESOLVE step.
- **S2 (P1.M3.T1.S2)**: download+verify+extract+sanity-run (download.go already has the primitives; S2 composes them
  into the swap pipeline). NO overlap with resolve.go.
- **P1.M3.T2.S1**: backup+atomic swap → swap.go. THIS is why S1's file is resolve.go, NOT swap.go.
- **P1.M4.T2**: command-layer runUpgrade — calls ResolveTarget, THEN does the up-to-date/newer-than determination
  (Compare + CurrentSemver), THEN branches to delegate (P1.M2.T2) or self-swap (P1.M3.T2). Owns exit codes 0/1/6.
- **P1.M2.T2.S1 (parallel, in-flight)**: delegate.go — the delegation path; ResolveTarget is the DIRECT-swap path.
  NO overlap (different code path; delegate handles manager-owned installs, ResolveTarget+self-swap handles direct).

## 7. Validation (verified against Makefile + the package)

- `go build ./...` + cross-build; `go vet ./internal/upgrade/...`; `gofmt -l internal/upgrade/resolve.go resolve_test.go`.
- `go test ./internal/upgrade/ -run 'ResolveTarget' -v` (the new tests).
- `go test -race ./internal/upgrade/...` (full package incl. parallel detect/download/delegate tests — no shared state).
- `make test` ; `make lint`. (NO coverage-gate on internal/upgrade — the gate is internal/{git,provider,generate,config}.)
- grep guards (see PRP Validation Level 4).