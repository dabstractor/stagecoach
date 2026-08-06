# P1.M1.T3.S1 — Research notes (GitHub Releases metadata client)

## 0. Task shape

Add `internal/upgrade/releases.go` (+ `releases_test.go`) to the **existing** `internal/upgrade`
package (already contains `version.go` from P1.M1.T1.S2). It exposes a `Client` that fetches GitHub
release metadata — the SOLE network surface in stagecoach (PRD §19 named exception). Stdlib-only
(`net/http` + `encoding/json`); NO new go.mod dependency.

## 1. The existing `package upgrade` (P1.M1.T1.S2 — Complete, read-only except a doc enrichment)

- `internal/upgrade/version.go` — semver primitives. **The package doc lives HERE** (the comment
  block immediately preceding `package upgrade`, lines 1–16).
- `func Compare(a, b string) int` (version.go:105) — semver §11 precedence. Returns −1/0/+1.
  **Prerelease-aware**: §11.4 (release > prerelease at equal core; dot-identifier comparison). An
  unparseable operand → 0 (never falsely signals an update). THIS is what `LatestAdmittingPrereleases`
  uses to pick the highest tag from the `/releases` array.
- `var currentVersion` (package-level, "dev" default) + `SetCurrentVersion` / `CurrentSemver` — usable
  in the same package to build the `User-Agent` header value (`"stagecoach/" + currentVersion`).
- `version_test.go` is **white-box `package upgrade`**, table-driven — match that style for the new
  tests.
- **DOC GOTCHA**: do NOT add a second `// Package upgrade …` comment in releases.go (duplicate-package-
  doc lint: golint/revive expect exactly one). Satisfy the Mode-A "package doc comment" requirement by
  APPENDING one paragraph to version.go's package doc (it already lives there) noting the releases
  client is the sole network surface + §19 scoping. releases.go itself gets a **file-level** comment
  separated from `package upgrade` by a blank line (NOT a competing package doc). Enriching version.go's
  doc is additive and does not break any T1.S2 test (P1.M1.T1.S2 is Complete; no parallel sibling
  touches internal/upgrade).

## 2. The GitHub Releases REST API (architecture/external_deps.md §1 — authoritative)

Three endpoints, all under `https://api.github.com` (overridable BaseURL for tests):

| Use | Method + path | Response shape |
|-----|---------------|----------------|
| Latest STABLE | `GET /repos/{owner}/{repo}/releases/latest` | ONE object: `{tag_name, name, assets:[{name, browser_download_url, size, api_url}], prerelease, draft}`. GitHub already excludes prereleases+drafts here. |
| All (prerelease channel) | `GET /repos/{owner}/{repo}/releases` | ARRAY of the above. Filter `draft==false`; admit prereleases; pick highest semver via `Compare`. |
| Pinned (`--version <v>`) | `GET /repos/{owner}/{repo}/releases/tags/{tag}` | ONE object (404 if unknown tag). |

- **Auth**: `Authorization: Bearer <token>` header WHEN a token is present. Raises rate limit;
  absence → unauthenticated 60 req/h. **This package does NOT read env** — it takes `Token` as a
  `Client` field; the command layer (P1.M4.T1.S1) resolves `STAGECOACH_GITHUB_TOKEN` → `GITHUB_TOKEN`.
- **User-Agent REQUIRED**: Go's default `Go-http-client/1.1` is sometimes rejected. Set
  `User-Agent: stagecoach/<ver>` on every request.
- **Redirects**: not relevant to metadata fetch (asset DOWNLOAD + redirect streaming is sibling
  P1.M1.T3.S2, not this subtask — this subtask only fetches the JSON metadata).
- **Error mapping** (external_deps §1 "Error mapping"):
  - 404 (no releases / unknown tag) → `ErrNoReleases`
  - 403 / 429 (rate-limited) → `ErrRateLimited` (hint: set a token)
  - other non-2xx / transport error → `ErrHTTP` (wrapping status + message)
  - FR-U11: network/rate-limit errors abort before any write (this subtask does NO write — pure
    metadata fetch — so the safety property is trivially upheld; just return clean typed errors).

## 3. §19 — the named network exception (PRD §19, exact quotes)

From PRD §19 ("Security considerations", "Diff content is local" bullet):
> "Stagecoach's commit-generation path (§9.1–§9.28) makes no network calls itself; the sole exception
> is `stagecoach upgrade` (§9.29), which fetches the project's own release artifacts and checksums from
> GitHub Releases — never provider credentials, never a diff, never repo data."

From architecture/v3_scope_boundary.md "§19 'no network calls' — now scoped (amendment)":
> "The commit path (§9.1–§9.28) makes NO network calls — unchanged. Verified: `net/http` is used
> NOWHERE in the repo today. `stagecoach upgrade` is the explicit, named exception: it fetches ONLY the
> project's own GitHub release artifacts + checksums."

→ This is the documentation the package doc + file comment must state (Mode A deliverable).

## 4. Codebase conventions to follow

- **Typed errors**: sentinel `var ErrXxx = errors.New(...)` checked via `errors.Is`. Precedent:
  `internal/git.ErrCASFailed` (updateref.go), asserted in tests as `errors.Is(err, ErrCASFailed)`.
  Wrap with `fmt.Errorf("upgrade: <context>: %w", ErrXxx)` (or `: %w` on the sentinel directly) so the
  concrete message AND the sentinel are both reachable. Mirror the codebase's `fmt.Errorf("ctx: %w", err)`
  wrapping style (load.go uses it pervasively).
- **This is the FIRST `net/http` surface** in the repo (verified: grep `net/http` → 0 in internal/).
  No existing HTTP client to mirror; establish the Go-idiomatic shape (injectable `*http.Client` +
  `BaseURL` for `httptest`, `context.Context` per call, `User-Agent` header).
- **No existing `httptest` usage** either; the new tests establish the fake-server pattern for the
  repo. `httptest.NewServer` + a handler that switches on path → returns canned JSON / status codes.
- **stdlib-only posture**: version.go's doc explicitly says "no external semver dependency … matching
  the repo's minimal-deps posture." releases.go must likewise add ZERO requires (`net/http`,
  `encoding/json`, `context`, `errors`, `fmt`, `io`, `strings` are all stdlib). go.mod stays unchanged.
- **Module path**: `github.com/dabstractor/stagecoach`, go 1.22.
- **Test style**: white-box `package upgrade`, table-driven where natural, `errors.Is` for typed errors
  (mirror version_test.go + internal/git updateref_test.go).

## 5. Design decisions (resolved)

- **Release/Asset structs** are EXPORTED (capitalized) — the command layer (P1.M4) and the
  download/verify subtask (P1.M1.T3.S2) consume them. Fields use stagecoach-idiomatic names (Tag,
  Assets, Name, DownloadURL, Size), NOT the raw GitHub JSON names — decode into an UNEXPORTED
  `ghRelease`/`ghAsset` with json tags, then convert.
- **`/latest` semantics**: GitHub's `/releases/latest` already returns the most recent non-prerelease,
  non-draft release (and 404s if none exists). So `LatestStable` does NOT need to filter — it just maps
  404→ErrNoReleases and decodes the single object. (This is simpler and more correct than listing+filtering.)
- **`LatestAdmittingPrereleases`**: GET `/releases` (array); filter `draft==false` (drafts always
  excluded — they require auth and are not real releases); over the survivors, keep the max-by-`Compare`
  tag. Empty survivor set → `ErrNoReleases`. NOTE: `Compare` returning 0 across garbage tags is fine
  (first survivor retained); we never claim an update from unparseable tags.
- **Buffering**: releases JSON is small (a few KB). `io.ReadAll(resp.Body)` + `json.Unmarshal` is
  acceptable and simpler than streaming decode (contract explicitly permits buffering). Always
  `defer io.Copy(io.Discard, resp.Body); resp.Body.Close()` to reuse connections (Go HTTP client
  requires body drain for keep-alive).
- **Token/UA**: set headers in one unexported `newRequest(ctx, method, path)` helper. UA =
  `"stagecoach/" + currentVersion` (same-package var; "dev" in tests is a fine UA). Token header only
  when `c.Token != ""`.
- **`do(...)` helper** centralizes: request build (UA + optional Bearer) → `client.Do` → status map
  (2xx body / 404 / 403/429 / other) → return `(body, err)`. Methods are thin wrappers that call `do`
  and `json.Unmarshal` (single object) or iterate (array).
- **Scope fence**: NO env reads (`os.Getenv`) — Token is a field. NO file writes. NO repo/lock/provider
  access (FR-U12). NO asset DOWNLOAD (that's P1.M1.T3.S2) — only metadata (tag + asset URLs + sizes).
  NO `Compare`-vs-current logic (that's the command layer's `--check` job, P1.M4) — this package only
  FETCHES and (for the prerelease channel) SELECTS the highest tag from the list.

## 6. Test matrix (httptest fake server)

All via a `Client{BaseURL: ts.URL, Repo: "owner/repo"}` against an `httptest.NewServer` whose handler
switches on `r.URL.Path` and `r.Header`. Required cases:

- `LatestStable` happy: `/releases/latest` → 200 canned `{tag_name:"v1.2.3", assets:[{name,size,browser_download_url}]}`;
  assert Tag=="v1.2.3", Assets[0].Name/Size/DownloadURL parsed.
- `LatestStable` 404 → `errors.Is(err, ErrNoReleases)`.
- `LatestStable` 403 → `errors.Is(err, ErrRateLimited)` and message contains "token".
- `LatestStable` 429 → `errors.Is(err, ErrRateLimited)`.
- `LatestStable` 500 → `errors.Is(err, ErrHTTP)` and message contains "500".
- `LatestAdmittingPrereleases` picks highest non-draft admitting prereleases: array with a draft
  (`draft:true`), a stable `v1.9.0`, and a prerelease `v2.0.0-rc1` → result Tag=="v2.0.0-rc1"
  (core 2>1; Compare admits the prerelease as highest). Assert draft excluded.
- `LatestAdmittingPrereleases` empty array `[]` → `ErrNoReleases`.
- `LatestAdmittingPrereleases` all drafts → `ErrNoReleases`.
- `ReleaseByTag` happy: `/releases/tags/v1.2.3` → 200 canned → assert Tag. Also 404 → `ErrNoReleases`.
- **Auth header**: when `Client.Token="xyz"`, handler records `r.Header.Get("Authorization")` ==
  `"Bearer xyz"`; when Token unset, header absent.
- **User-Agent**: handler records `r.Header.Get("User-Agent")` starts with `"stagecoach/"` (NOT the Go
  default `"Go-http-client"`).
- **Transport failure**: point `BaseURL` at a closed server (`ts.Close()`) → `errors.Is(err, ErrHTTP)`.
- (Context cancellation: `ctx, cancel()` then call → error; optional but cheap via `errors.Is(ctx.Err())`.)