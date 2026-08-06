name: "P1.M3.T1.S1 — ResolveTarget: resolve target release + select GOOS_GOARCH asset (FR-U5 steps 2-3)"
description: >
  The RESOLVE step of the direct-binary self-swap path (FR-U5): a thin composition `ResolveTarget(ctx, c *Client, opts)
  (Release, Asset, error)` in a NEW file internal/upgrade/resolve.go (NOT swap.go — P1.M3.T2.S1 owns swap.go). It composes
  the already-landed Client methods (P1.M1.T3.S1) per the channel/version flags: `opts.Version != ""` ⇒ `c.ReleaseByTag(ctx,
  v)` (pinned --version, precedence over channel); else `opts.Prerelease` ⇒ `c.LatestAdmittingPrereleases(ctx)` (--prerelease
  channel); else `c.LatestStable(ctx)` (default) — then `SelectAsset(release, goos, goarch)` (P1.M1.T3.S2, pure) for the
  platform archive, where goos/goarch come from `opts.GOOS/GOARCH` defaulting to `runtime.GOOS/GOARCH` (injectable so per-platform
  selection is testable off-host). CRITICAL SCOPE DECISION (contract point 3): ResolveTarget does NOT call Compare / does NOT
  gate on "already up to date" / "target not newer than current" — the parenthetical DECISION overrides the "Validate" clause;
  the --check/up-to-date determination is the COMMAND LAYER's job (P1.M4.T2 runUpgrade, which has Compare + CurrentSemver).
  ResolveTarget is fetch + select, nothing more. Errors propagate UNWRAPPED (Client's ErrNoReleases/ErrRateLimited/ErrHTTP +
  SelectAsset's ErrNoMatchingAsset — all exported) so `errors.Is` works in the command layer; zero values returned on error.
  Stdlib-only (context, runtime, strings — NO fmt, NO internal/*; FR-U12 walled off); go.mod unchanged. File comment, NOT a
  package doc (releases.go owns it — the established convention). Plus resolve_test.go (package upgrade, internal tests)
  reusing the proven newFakeClient/cannedLatest/cannedReleases/statusServer httptest idiom (releases_test.go): the 3 channel
  branches (latest/prerelease/version) against httptest fakes + Version-precedence-over-Prerelease + per-platform asset
  selection (linux/darwin/windows incl. ErrNoMatchingAsset) + empty-GOOS⇒runtime default + Client-error propagation
  (ErrNoReleases/ErrRateLimited via errors.Is). Consumed by P1.M4.T2 (the upgrade command's runUpgrade dispatcher). ZERO
  production callers after this subtask (only resolve_test.go) — expected.

---

## Goal

**Feature Goal**: Provide the resolution+selection step of the FR-U5 direct-binary self-swap: given a `Client` and the
channel/version flags, fetch the correct target release from GitHub Releases and pick the platform archive asset for the
current (or injected) GOOS/GOARCH — returning `(Release, Asset, error)` the swap pipeline (P1.M3.T1.S2 download+verify,
P1.M3.T2 backup+rename) consumes. This is steps 2-3 of FR-U5: "resolve the target release" + "select the GOOS_GOARCH asset".
It is deliberately the THINNEST possible composition — all network logic is in the already-landed Client (P1.M1.T3.S1) and
all asset-matching is in the already-landed SelectAsset (P1.M1.T3.S2); ResolveTarget just picks the right Client method per
the flags and delegates selection.

**Deliverable** (2 new files in package upgrade; no edits to existing files):
1. **internal/upgrade/resolve.go** — `ResolveOptions` struct + `ResolveTarget(ctx, c *Client, opts) (Release, Asset, error)`
   + the private `resolveRelease` dispatcher. File comment (not package doc). Stdlib-only.
2. **internal/upgrade/resolve_test.go** — httptest-backed tests (reuse `newFakeClient`/`cannedLatest`/`cannedReleases`/
   `statusServer` from releases_test.go) covering: the 3 channel branches, Version-precedence-over-Prerelease, per-platform
   asset selection incl. ErrNoMatchingAsset, empty-GOOS⇒runtime default, and Client-error propagation.

**Success Definition**:
- `ResolveTarget(ctx, c, ResolveOptions{})` calls `c.LatestStable` and returns its release + the runtime-platform asset.
- `ResolveTarget(ctx, c, ResolveOptions{Prerelease:true})` calls `c.LatestAdmittingPrereleases` (Version empty).
- `ResolveTarget(ctx, c, ResolveOptions{Version:"v1.2.3"})` calls `c.ReleaseByTag("v1.2.3")` — EVEN when Prerelease is also
  true (Version precedence).
- `opts.GOOS`/`opts.GOARCH` empty ⇒ `runtime.GOOS`/`runtime.GOARCH`; non-empty ⇒ the injected values drive SelectAsset.
- No matching asset ⇒ returns `(Release{}, Asset{}, err)` with `errors.Is(err, ErrNoMatchingAsset)`.
- A Client failure (404/403/etc.) propagates with the sentinel intact (`errors.Is(err, ErrNoReleases)` / `ErrRateLimited`).
- ResolveTarget does NOT call `Compare` / does NOT gate on up-to-date (command layer P1.M4.T2 owns that).
- `go build ./...`, `go vet ./internal/upgrade/...`, `go test -race ./internal/upgrade/...`, `make test`/`make lint` green;
  `gofmt -l` empty; `go.mod`/`go.sum` unchanged; NO `internal/*` import (FR-U12); ZERO production callers (consumer is P1.M4.T2).

## User Persona (if applicable)

**Target User**: The `stagecoach upgrade` command layer (P1.M4.T2 `runUpgrade`), which calls `ResolveTarget` to get the
target release+asset, then does the newer-than-current check and branches to delegate (P1.M2.T2) or self-swap (P1.M3.T2).
End users never call ResolveTarget directly.

**Use Case**: `stagecoach upgrade` (default) → ResolveTarget picks LatestStable + the runtime asset → the swap pipeline
downloads+verifies+swaps. `stagecoach upgrade --prerelease` → LatestAdmittingPrereleases. `stagecoach upgrade --version v1.2.3`
→ that exact tag. `stagecoach upgrade --check` → the command layer calls ResolveTarget, compares tag to CurrentSemver, exits
6 if behind / 0 if current (ResolveTarget itself just resolves; it doesn't decide up-to-date).

**User Journey**: user runs `stagecoach upgrade` → runUpgrade (P1.M4.T2) builds a Client + calls ResolveTarget → GitHub
Releases is queried for /releases/latest → the linux_amd64 asset is selected → ResolveTarget returns (Release{Tag:"v1.2.3"},
Asset{Name:"stagecoach_1.2.3_linux_amd64.tar.gz",...}, nil) → runUpgrade compares v1.2.3 to the running version → proceeds to
download+verify+swap (P1.M3.T1.S2 + P1.M3.T2).

**Pain Points Addressed**: FR-U5 steps 2-3 — the direct-binary swap needs a target release + a platform asset to download;
without ResolveTarget, the command layer would inline channel-flag→method dispatch + asset selection (duplicated logic, hard
to test in isolation). ResolveTarget centralizes it as a pure, injectable, fully-tested composition.

## Why

- **FR-U5 steps 2-3 / §9.29**: the self-swap's resolve step is "fetch the target release (latest stable / prerelease / pinned
  tag) and select the GOOS_GOARCH archive". The Client (P1.M1.T3.S1) and SelectAsset (P1.M1.T3.S2) are LANDED; ResolveTarget
  is the missing composition that turns the three flag combinations into a (Release, Asset) pair.
- **Thin by design**: every network call is in Client; every asset match is in SelectAsset. ResolveTarget adds ZERO new
  network/match logic — it is a 3-way dispatch + a delegation. This keeps the swap pipeline's resolve step auditable and the
  test surface tiny (the Client and SelectAsset already have their own exhaustive tests).
- **Scope discipline (the DECISION)**: the contract considered having ResolveTarget validate "target newer than current" via
  Compare, then DECIDED the up-to-date determination belongs in the command layer (P1.M4.T2). ResolveTarget fetches+selects
  only; it does not know the running version and does not decide "already up to date". This separation means `--check` (which
  needs the comparison) and the normal swap (which needs the asset) can both reuse ResolveTarget without it second-guessing.
- **Bounded, no-conflict scope**: one new production file + tests. resolve.go (NOT swap.go — P1.M3.T2 owns swap.go). No edit
  to releases.go/download.go/version.go/detect.go/delegate.go. The parallel delegate.go (P1.M2.T2.S1) is the delegation path;
  ResolveTarget is the direct-swap path — no overlap. Lands independently; needs only the Client + SelectAsset (both landed).

## What

**User-visible behavior**: None directly (no caller yet — the command is P1.M4). Internally, `ResolveTarget` becomes the
authoritative target-resolution+asset-selection entry point for the direct-swap path.

**Technical change** (one new file + tests; verbatim API in the Blueprint): a 3-way channel/version dispatcher + a
runtime-defaulting GOOS/GOARCH pass-through to SelectAsset, with errors propagated unwrapped.

### Success Criteria
- [ ] `internal/upgrade/resolve.go` exports `ResolveOptions{Version, Prerelease string/bool; GOOS, GOARCH string}` and
      `ResolveTarget(ctx context.Context, c *Client, opts ResolveOptions) (Release, Asset, error)`.
- [ ] Version != "" (after TrimSpace) ⇒ `c.ReleaseByTag(ctx, v)` (precedence over Prerelease, even when Prerelease is true).
- [ ] Version == "" && Prerelease ⇒ `c.LatestAdmittingPrereleases(ctx)`.
- [ ] Version == "" && !Prerelease ⇒ `c.LatestStable(ctx)`.
- [ ] `opts.GOOS`/`opts.GOARCH` empty ⇒ `runtime.GOOS`/`runtime.GOARCH`; non-empty ⇒ the injected values.
- [ ] The release is passed to `SelectAsset(release, goos, goarch)`; its (Asset, error) is returned (Release alongside).
- [ ] Errors propagate UNWRAPPED (no fmt.Errorf re-wrap) so `errors.Is(err, ErrNoReleases|ErrRateLimited|ErrHTTP|ErrNoMatchingAsset)` works.
- [ ] On any error, returns `(Release{}, Asset{}, err)` (zero non-error values).
- [ ] ResolveTarget does NOT call `Compare` / `CurrentSemver` / any up-to-date gate.
- [ ] resolve.go is a FILE comment (not `// Package upgrade`); imports stdlib only (context, runtime, strings); NO internal/*.
- [ ] resolve_test.go covers: 3 channel branches (httptest), Version-precedence-over-Prerelease, per-platform selection
      (linux/darwin + windows→ErrNoMatchingAsset), empty-GOOS⇒runtime default, Client-error propagation (ErrNoReleases/ErrRateLimited).
- [ ] `go build ./...` + cross-build clean; `go vet ./internal/upgrade/...` clean; `gofmt -l` empty; `make test`/`make lint` green.
- [ ] Scope: `git status` == only `internal/upgrade/resolve.go` + `internal/upgrade/resolve_test.go` (new).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim ResolveTarget/ResolveOptions/resolveRelease API, the THREE Client method signatures it composes
(LatestStable/ReleaseByTag/LatestAdmittingPrereleases, all `(Release, error)`), the SelectAsset signature + the exported
ErrNoMatchingAsset sentinel, the precedence rule (Version > Prerelease > LatestStable), the DECISION that ResolveTarget does
NOT call Compare (the load-bearing scope boundary), the GOOS/GOARCH-runtime-default + why it must be injectable for tests,
the unwrap-don't-rewrap error rule, the resolve.go-vs-swap.go file-name choice (swap.go is P1.M3.T2's), the file-comment-not-
package-doc convention, and the exact httptest test idiom to clone (newFakeClient/cannedLatest/cannedReleases/statusServer +
the in-package assetName helper for the runtime-default test).

### Documentation & References

```yaml
# MUST READ — the authoritative findings (verbatim API + the DECISION + the test plan + scope fences)
- docfile: plan/017_397abce9deb1/P1M3T1S1/research/findings.md
  why: "§1 the Client/Release/Asset/SelectAsset/Compare contracts (all LANDED); §2 THE DECISION (ResolveTarget does NOT call
        Compare — the command layer owns up-to-date); §3 the verbatim API + precedence + why GOOS/GOARCH are injectable; §4
        error handling (propagate unwrapped, zero-on-error); §5 the test idiom to REUSE (newFakeClient/cannedLatest/etc. +
        the in-package assetName); §6 the S1/S2/P1.M3.T2/P1.M4.T2/P1.M2.T2 scope fences; §7 validation."
  critical: "§2 + §3: the single most important boundary — do NOT add a Compare/up-to-date check. §3: file is resolve.go
             (NOT swap.go — P1.M3.T2.S1 owns swap.go). §3: GOOS/GOARCH default to runtime but are injectable opts fields
             (required for the per-platform tests). §4: propagate errors UNWRAPPED (errors.Is must reach the sentinels)."

# MUST READ — the Client this task composes (TREAT AS LANDED; consume, don't rebuild)
- file: internal/upgrade/releases.go
  why: "Client (:60) + LatestStable (:178) + ReleaseByTag (:193, rejects empty tag) + LatestAdmittingPrereleases (:217) — the
        THREE methods ResolveTarget dispatches to. Release (:51) + Asset (:42) types. The exported sentinels ErrNoReleases /
        ErrRateLimited / ErrHTTP (the do() error contract, :135). The package doc (line 1) — resolve.go must NOT add a competing one."
  pattern: "Client is an injectable-struct (HTTP/BaseURL/Repo/Token fields); errors are exported sentinels wrapped with %w at
            the use site. ResolveTarget reuses Client AS-IS (no new network code) and propagates its sentinels unchanged."
  critical: "ReleaseByTag already rejects an empty/whitespace tag (returns ErrHTTP) — but ResolveTarget decides the BRANCH on
             TrimSpace(Version)!=\"\", so an empty Version never reaches ReleaseByTag. Do not duplicate the empty-tag guard."

# MUST READ — SelectAsset (the asset picker ResolveTarget delegates to; TREAT AS LANDED)
- file: internal/upgrade/download.go   # SelectAsset :93; ErrNoMatchingAsset :35; assetName (in-package) helper
  why: "SelectAsset(release, goos, goarch) (Asset, error) is PURE (no network) and exact-matches the goreleaser archive name
        (stagecoach_<tag-no-v>_<os>_<arch>.tar.gz, or .zip on windows). ErrNoMatchingAsset is exported. assetName(tag,goos,goarch)
        is in-package/unexported — callable from resolve_test.go (also package upgrade) to build expected names."
  critical: "ResolveTarget calls SelectAsset verbatim — do NOT re-implement asset matching. The .zip-on-windows rule lives in
             assetName; ResolveTarget is OS-agnostic (it just passes goos/goarch through)."

# MUST READ — the test idiom to clone (releases_test.go's httptest fakes)
- file: internal/upgrade/releases_test.go   # newFakeClient :15, cannedLatest :24, cannedReleases :38, statusServer :52
  why: "package upgrade (internal test); newFakeClient(t, handler) spins an httptest.Server + returns &Client{BaseURL: ts.URL,
        Repo:\"owner/repo\"} + t.Cleanup(ts.Close). cannedLatest() = v1.2.3 with linux_amd64 + darwin_arm64 assets.
        cannedReleases() = [v3.0.0 draft, v1.9.0 stable, v2.0.0-rc1 pre] (pre wins after draft excluded). statusServer(code,body)."
  pattern: "REUSE these helpers verbatim in resolve_test.go (same package — no re-declaration; just call them). A test asserts
            which endpoint was hit by inspecting r.URL.Path in the handler (e.g. strings.HasSuffix(r.URL.Path, \"/releases/latest\"))."
  gotcha: "cannedLatest has NO windows asset ⇒ it doubles as the ErrNoMatchingAsset fixture (GOOS=windows ⇒ SelectAsset errors)."

# CONTEXT — Compare / CurrentSemver (the version primitives ResolveTarget deliberately does NOT use)
- file: internal/upgrade/version.go   # Compare :112, CurrentSemver :39
  why: "Confirms Compare/CurrentSemver EXIST and are what the command layer (P1.M4.T2) will use for the up-to-date check.
        ResolveTarget does NOT import or call them — the DECISION (findings §2) assigns that determination to the command layer."
  critical: "Do NOT add a Compare call to ResolveTarget. If you find yourself importing version logic into resolve.go, STOP —
             that is P1.M4.T2's scope. resolve.go imports only context/runtime/strings."

# CONTEXT — the sibling swap file (DO NOT create swap.go — P1.M3.T2 owns it)
- docfile: (P1.M3.T2.S1 PRP, when written)
  why: "P1.M3.T2.S1 (Backup + atomic swap) creates internal/upgrade/swap.go. If S1 also used swap.go the two would collide.
        resolve.go is the contract's offered alternative and is semantically correct (ResolveTarget ≠ swap)."
  critical: "Name the file resolve.go. Do NOT name it swap.go."

# CONTEXT — the command layer that consumes ResolveTarget (LANDS LATER; not this task)
- docfile: plan/017_397abce9deb1/P1M2T2S1/PRP.md   # the delegate.go sibling (the OTHER upgrade path)
  why: "Shows the package conventions S1 must match (file-comment-not-package-doc, injectable seam, sentinel errors, canned-fake
        tests, stdlib-only, walled-off FR-U12). delegate.go is the DELEGATION path; ResolveTarget is the DIRECT-swap path — no
        overlap, but the conventions are identical. P1.M4.T2 (runUpgrade) will call ResolveTarget for the direct path and
        Delegate for the manager path, then do the Compare/up-to-date check itself."
  critical: "After this subtask, grep must show ResolveTarget called ONLY in resolve_test.go (ZERO production callers). The
             command layer (P1.M4.T2) is the consumer — do NOT add it here."
```

### Current Codebase tree (relevant slice)

```bash
internal/upgrade/
  releases.go        # READ-ONLY — Client + the 3 methods ResolveTarget composes; OWNS the package doc
  releases_test.go   # READ-ONLY — newFakeClient/cannedLatest/cannedReleases/statusServer (REUSE in resolve_test.go)
  download.go        # READ-ONLY — SelectAsset (:93) + ErrNoMatchingAsset (:35) + assetName (in-package helper)
  version.go         # READ-ONLY — Compare/CurrentSemver (ResolveTarget deliberately does NOT use them)
  detect.go          # READ-ONLY (P1.M2.T1.S1) — Channel/Runner; no overlap with resolve.go
  delegate.go        # READ-ONLY (P1.M2.T2.S1, in-flight) — the delegation path; no overlap
  resolve.go         # CREATE — ResolveOptions + ResolveTarget + resolveRelease (THIS TASK)
  resolve_test.go    # CREATE — httptest tests reusing releases_test.go's fakes (THIS TASK)
  # (swap.go is P1.M3.T2.S1's — do NOT create it here)
internal/cmd/
  upgrade.go         # READ-ONLY / not-yet-created — the command is P1.M4 (consumes ResolveTarget later)
go.mod               # READ-ONLY — module github.com/dabstractor/stagecoach; stdlib only; NO new require
```

### Desired Codebase tree with files to be added

```bash
internal/upgrade/resolve.go        # NEW — ResolveOptions + ResolveTarget + resolveRelease (stdlib only; file comment)
internal/upgrade/resolve_test.go   # NEW — httptest tests (3 branches + precedence + per-platform + runtime-default + errors)
# (NO swap.go — that is P1.M3.T2.S1. NO edit to any existing file.)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (ResolveTarget does NOT call Compare — THE scope boundary): the contract's point 3 says "Validate the target tag
// is newer than current via Compare" but is immediately overridden by the parenthetical DECISION: "the --check/up-to-date
// determination is in the command layer P1.M4.T2; ResolveTarget just returns the chosen release+asset". ResolveTarget is
// fetch+select ONLY. Adding a Compare/CurrentSemver/up-to-date gate here (a) duplicates the command layer, (b) forces
// ResolveTarget to own the running-version concern, and (c) breaks --check (which needs the comparison but a different exit
// path). Do NOT import version.go into resolve.go. If you are, you are in the wrong task.

// CRITICAL (file name = resolve.go, NOT swap.go): the contract offers "swap.go (or resolve.go)". P1.M3.T2.S1 (Backup + atomic
// swap) creates swap.go — using swap.go here would collide with that sibling. resolve.go is semantically correct (ResolveTarget
// resolves; the on-disk swap is P1.M3.T2's job) and avoids the collision. Name it resolve.go.

// CRITICAL (precedence: Version > Prerelease > LatestStable): a non-empty (after TrimSpace) opts.Version ⇒ ReleaseByTag,
// EVEN IF Prerelease is also true. A pinned --version is an explicit user instruction that overrides the channel. Check
// Version FIRST in resolveRelease, then Prerelease, then the LatestStable default. Inverting this (channel before version)
// would silently ignore --version when --prerelease is also set.

// CRITICAL (decide the branch on TrimSpace(Version), then PASS the trimmed value): ReleaseByTag rejects empty tags itself
// (releases.go:196), but ResolveTarget must decide WHICH method to call. Use `if v := strings.TrimSpace(opts.Version); v != ""`
// and pass that `v` to ReleaseByTag. Checking raw `opts.Version != ""` would send a whitespace-only " " to ReleaseByTag (which
// then errors ErrHTTP) instead of falling through to the channel — surprising. Trim once, use the trimmed value both ways.

// CRITICAL (propagate errors UNWRAPPED — do not fmt.Errorf re-wrap): the Client methods return errors wrapping the exported
// sentinels (ErrNoReleases/ErrRateLimited/ErrHTTP) with %w; SelectAsset wraps ErrNoMatchingAsset with %w. ResolveTarget
// returns them AS-IS so the command layer's `errors.Is(err, upgrade.ErrNoReleases)` etc. work. Re-wrapping (e.g.
// fmt.Errorf("resolve target: %w", err)) is HARMLESS for errors.Is but ADDS NOTHING and diverges from the package style
// (the Client/SelectAsset errors already carry path/status/asset-name diagnostics). Just `return Release{}, Asset{}, err`.

// CRITICAL (GOOS/GOARCH default to runtime but are INJECTABLE opts): production passes empty ⇒ runtime.GOOS/GOARCH (the
// contract's "SelectAsset(release, runtime.GOOS, runtime.GOARCH)"). Tests inject e.g. "linux"/"amd64" to assert per-platform
// selection OFF the test host. If GOOS/GOARCH were hardcoded to runtime, the per-platform tests (linux/darwin/windows) could
// only ever test the HOST platform — defeating the contract's "asset selection per GOOS/GOARCH" requirement. The injectable
// default mirrors the package convention (Client.HTTP/BaseURL, DelegateOptions.Env).

// GOTCHA (file comment, NOT package doc): releases.go line 1 owns `// Package upgrade`. Start resolve.go with a FILE comment
// (`// resolve.go implements the FR-U5 resolve+select step…`) — exactly as detect.go/download.go/delegate.go do. A second
// `// Package upgrade` splits the package overview.

// GOTCHA (zero values on error — standard Go): on any error return (Release{}, Asset{}, err). The SelectAsset error message
// already names the wanted asset + GOOS/GOARCH ("select asset windows/amd64 (want stagecoach_1.2.3_windows_amd64.zip): …"),
// so returning a zero Release (rather than the fetched release) loses no diagnostic. Do not half-populate the return.

// GOTCHA (stdlib only — no fmt, no internal/*): resolve.go needs ONLY context, runtime, strings. No fmt (errors propagate
// unwrapped — see above). No internal/* (FR-U12 walled-off; the upgrade package imports nothing stagecoach-internal). Adding
// fmt/internal is a sign of scope creep (re-wrapping errors, or pulling in version/config logic).

// GOTCHA (c *Client is concrete, not an interface): the contract's test mocking is "httptest fake server + injectable Client"
// — the seam is Client.BaseURL/HTTP (the NETWORK layer), NOT the Client type. ResolveTarget takes `c *Client` (concrete),
// matching releases_test.go's newFakeClient (which returns *Client). Do NOT define a Client interface for testability — the
// httptest server IS the injection point.
```

## Implementation Blueprint

### Data models and structure

```go
// ResolveOptions — selects the target channel (Version/Prerelease) and the platform (GOOS/GOARCH).
// Every field is an opt-in override; the zero value resolves LatestStable for runtime.GOOS/GOARCH.
type ResolveOptions struct {
	Version    string // non-empty (--version <v>) ⇒ ReleaseByTag; precedence over Prerelease
	Prerelease bool   // true (--prerelease) ⇒ LatestAdmittingPrereleases; only when Version == ""
	GOOS       string // "" ⇒ runtime.GOOS (injectable for per-platform tests)
	GOARCH     string // "" ⇒ runtime.GOARCH (injectable for per-platform tests)
}
// ResolveTarget returns (Release, Asset) — the two things the swap pipeline (P1.M3.T1.S2 download+verify,
// P1.M3.T2 backup+rename) needs. No new types beyond ResolveOptions.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/upgrade/resolve.go — file comment + imports + ResolveOptions + ResolveTarget + resolveRelease
  - FILE COMMENT header (NOT a package doc): "// resolve.go implements the FR-U5 target-resolution + asset-selection step of
    the direct-binary self-swap (§9.29 FR-U5 steps 2-3). It composes the GitHub Releases Client (releases.go) per the
    channel/version flags and delegates platform-asset matching to SelectAsset (download.go). It is a THIN composition: no
    new network or match logic, no up-to-date determination (that is the command layer's job, P1.M4.T2), and it is walled
    off (FR-U12: stdlib-only, no internal/* imports). File comment only — releases.go owns the package doc."
  - IMPORTS: context, runtime, strings. (NO fmt, NO internal/*.)
  - ResolveOptions struct (4 fields as above; godoc each — Version precedence, Prerelease channel, GOOS/GOARCH runtime-default).
  - ResolveTarget(ctx, c *Client, opts ResolveOptions) (Release, Asset, error):
      release, err := resolveRelease(ctx, c, opts)
      if err != nil { return Release{}, Asset{}, err }
      goos, goarch := opts.GOOS, opts.GOARCH
      if goos == ""  { goos  = runtime.GOOS }
      if goarch == "" { goarch = runtime.GOARCH }
      asset, err := SelectAsset(release, goos, goarch)
      if err != nil { return Release{}, Asset{}, err }
      return release, asset, nil
  - resolveRelease(ctx, c, opts) (Release, error) — the 3-way dispatch (private):
      if v := strings.TrimSpace(opts.Version); v != "" { return c.ReleaseByTag(ctx, v) }
      if opts.Prerelease                                  { return c.LatestAdmittingPrereleases(ctx) }
      return c.LatestStable(ctx)
  - GODOC on ResolveTarget: cite FR-U5 steps 2-3; state precedence (Version > Prerelease > LatestStable); state GOOS/GOARCH
    default to runtime + are injectable; state it does NOT determine up-to-date (command layer P1.M4.T2); state errors
    propagate the Client/SelectAsset sentinels unchanged (errors.Is-reachable).
  - NAMING: ResolveTarget (exported, PascalCase); ResolveOptions (exported); resolveRelease (private helper).
  - GOTCHA: do NOT import version.go (no Compare). do NOT re-wrap errors. do NOT nil-guard c (the package doesn't).

Task 2: CREATE internal/upgrade/resolve_test.go — REUSE releases_test.go's httptest fakes (package upgrade, internal test)
  - IMPORTS: context, errors, net/http, net/http/httptest, strings, testing. (NO new helpers — call newFakeClient/
    cannedLatest/cannedReleases/statusServer from releases_test.go directly; they are in the same package.)
  - (a) TestResolveTarget_LatestStable — newFakeClient(t, handler serving cannedLatest at /releases/latest); assert the handler
        saw a GET whose Path HasSuffix "/releases/latest"; ResolveTarget(ctx, c, ResolveOptions{GOOS:"linux",GOARCH:"amd64"})
        ⇒ Release.Tag=="v1.2.3", Asset.Name=="stagecoach_1.2.3_linux_amd64.tar.gz", err==nil.
  - (b) TestResolveTarget_Prerelease — newFakeClient(t, handler serving cannedReleases at /releases); assert Path HasSuffix
        "/releases" (the array endpoint, NOT /latest); opts{Prerelease:true, GOOS:"linux",GOARCH:"amd64"} ⇒ the prerelease
        wins (cannedReleases's highest non-draft is v2.0.0-rc1) — NOTE cannedReleases's pre asset is "pre.tar.gz" which won't
        match the goreleaser name, so assert the RELEASE tag (v2.0.0-rc1) and expect ErrNoMatchingAsset for the asset (OR use a
        custom canned array whose pre release HAS a linux_amd64 asset). Cleanest: use a custom handler+payload for the
        prerelease-asset-success case (a pre release with a real stagecoach_2.0.0-rc1_linux_amd64.tar.gz asset) so the test
        proves BOTH the endpoint AND the asset; keep cannedReleases only for the endpoint/assert-tag part.
  - (c) TestResolveTarget_Version — newFakeClient(t, handler serving cannedLatest at /releases/tags/v1.2.3); assert Path
        HasSuffix "/releases/tags/v1.2.3"; opts{Version:"v1.2.3", GOOS:"linux",GOARCH:"amd64"} ⇒ Release.Tag=="v1.2.3", asset
        linux_amd64, err==nil.
  - (d) TestResolveTarget_VersionPrecedenceOverPrerelease — a handler that FAILS if /releases (array) is hit and serves
        cannedLatest at /releases/tags/v1.2.3; opts{Version:"v1.2.3", Prerelease:true, GOOS:"linux",GOARCH:"amd64"} ⇒ succeeds
        (the tags endpoint was hit, NOT the array) ⇒ proves Version wins.
  - (e) TestResolveTarget_AssetSelectionPerPlatform — newFakeClient + cannedLatest (has linux_amd64 + darwin_arm64):
        - GOOS:"linux",GOARCH:"amd64"  ⇒ Asset.Name contains "linux_amd64"
        - GOOS:"darwin",GOARCH:"arm64" ⇒ Asset.Name contains "darwin_arm64"
        - GOOS:"windows",GOARCH:"amd64"⇒ err != nil, errors.Is(err, ErrNoMatchingAsset) (cannedLatest has NO windows asset)
  - (f) TestResolveTarget_EmptyGOOSDefaultsRuntime — build a canned release whose asset is assetName("v1.2.3", runtime.GOOS,
        runtime.GOARCH) (the in-package helper); opts{GOOS:"",GOARCH:""} (zero) ⇒ the returned Asset.Name == that runtime name
        (proves the empty⇒runtime default path). Use a custom handler/payload so the asset name is exactly the runtime one.
  - (g) TestResolveTarget_ClientErrorPropagates — newFakeClient(t, statusServer(404, "")) for LatestStable ⇒ err != nil,
        errors.Is(err, ErrNoReleases). AND statusServer(403, "") ⇒ errors.Is(err, ErrRateLimited). (Proves UNWRAPPED propagation.)
  - (h) TestResolveTarget_NoMatchingAssetZeroReturn — the windows case from (e) ALSO asserts the returned Release is zero
        (Release{} == got) and Asset is zero — the zero-on-error contract.
  - Each test: ctx := context.Background(); c := newFakeClient(t, handler); no t.Cleanup needed (newFakeClient owns it).
  - COVERAGE: 3 branches + precedence + 3 platforms + runtime-default + 2 client errors + zero-on-error. Every public path.
  - GOTCHA: for the prerelease SUCCESS asset case, cannedReleases's pre asset ("pre.tar.gz") does NOT match the goreleaser name,
    so either (i) assert only the tag + expect ErrNoMatchingAsset, or (ii) use a custom payload. Prefer (ii) for a clean
    success assertion; keep cannedReleases for a separate "endpoint hit + tag" assertion if desired.

Task 3: VERIFY — build, vet, format, focused + full tests, lint, grep guards
  - go build ./... ; GOOS=windows go build ./... ; GOOS=linux go build ./... ; GOOS=darwin go build ./...
  - go vet ./internal/upgrade/...
  - gofmt -l internal/upgrade/resolve.go internal/upgrade/resolve_test.go   # empty
  - go test ./internal/upgrade/ -run 'ResolveTarget' -v
  - go test -race ./internal/upgrade/...   # incl. parallel detect/download/delegate tests (no shared state)
  - make test ; make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the 3-way dispatcher (Version > Prerelease > LatestStable). TrimSpace decides the branch AND feeds ReleaseByTag.
func resolveRelease(ctx context.Context, c *Client, opts ResolveOptions) (Release, error) {
	if v := strings.TrimSpace(opts.Version); v != "" {
		return c.ReleaseByTag(ctx, v) // pinned --version wins, even with --prerelease
	}
	if opts.Prerelease {
		return c.LatestAdmittingPrereleases(ctx)
	}
	return c.LatestStable(ctx) // default channel
}

// PATTERN: the composition (resolve + runtime-default + select). Errors propagate UNWRAPPED; zero on error.
func ResolveTarget(ctx context.Context, c *Client, opts ResolveOptions) (Release, Asset, error) {
	release, err := resolveRelease(ctx, c, opts)
	if err != nil {
		return Release{}, Asset{}, err
	}
	goos, goarch := opts.GOOS, opts.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	asset, err := SelectAsset(release, goos, goarch)
	if err != nil {
		return Release{}, Asset{}, err
	}
	return release, asset, nil
}

// PATTERN: the httptest test (REUSE releases_test.go's newFakeClient + canned payloads).
func TestResolveTarget_LatestStable(t *testing.T) {
	c := newFakeClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/releases/latest") {
			t.Errorf("hit %q, want /releases/latest", r.URL.Path)
		}
		_, _ = w.Write([]byte(cannedLatest()))
	})
	rel, asset, err := ResolveTarget(context.Background(), c, ResolveOptions{GOOS: "linux", GOARCH: "amd64"})
	if err != nil { t.Fatalf("ResolveTarget: %v", err) }
	if rel.Tag != "v1.2.3" { t.Errorf("Tag = %q, want v1.2.3", rel.Tag) }
	if asset.Name != "stagecoach_1.2.3_linux_amd64.tar.gz" { t.Errorf("asset = %q", asset.Name) }
}
```

### Integration Points

```yaml
UPGRADE PACKAGE (internal/upgrade/resolve.go):
  - +ResolveOptions struct; +ResolveTarget(ctx, c *Client, opts) (Release, Asset, error); +resolveRelease (private).

DOWNSTREAM (this subtask ENABLES but does NOT build):
  - P1.M3.T1.S2 (download+verify+extract+sanity-run): takes the (Release, Asset) ResolveTarget returns; downloads
    asset.DownloadURL, verifies via FetchChecksums/VerifySHA256, extracts, sanity-runs BEFORE swap (FR-U11 abort-before-swap).
  - P1.M3.T2.S1 (backup+atomic swap): the on-disk change after S2's sanity-run succeeds.
  - P1.M4.T2 (runUpgrade dispatcher): builds the Client, calls ResolveTarget, does the newer-than-current check
    (Compare + CurrentSemver) — the up-to-date/exit-6 determination ResolveTarget deliberately omits — then branches to
    Delegate (P1.M2.T2, manager channels) or the self-swap (P1.M3 direct channel).
  - ZERO production callers of ResolveTarget after this subtask (only resolve_test.go) — expected.

SCOPE FENCES: NO Compare/up-to-date check (command layer P1.M4.T2); NO download/verify/extract/swap (P1.M3.T1.S2 + P1.M3.T2);
  NO cobra/command (P1.M4); NO --force gate / --check / y-N prompt (command layer); NO edit to releases.go / download.go /
  version.go / detect.go / delegate.go; NO swap.go file (P1.M3.T2 owns it); NO internal/* import (FR-U12); NO package doc
  (releases.go owns it); NO new go.mod require.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build + vet (resolve.go must compile alongside the parallel delegate.go + detect.go — no symbol clash).
go build ./...
GOOS=windows go build ./...
GOOS=linux   go build ./...
GOOS=darwin  go build ./...
# Expected: clean. A failure likely means a symbol clash (ResolveOptions/ResolveTarget already exist elsewhere — they don't)
#           or an unwanted import (fmt/internal/*). resolve.go imports only context/runtime/strings.

# Vet.
go vet ./internal/upgrade/...
# Expected: clean.

# Format.
gofmt -l internal/upgrade/resolve.go internal/upgrade/resolve_test.go
# Expected: empty. If listed: gofmt -w the file(s).

# Lint.
make lint      # golangci-lint (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors. `unused` stays clean (ResolveTarget + ResolveOptions are EXPORTED ⇒ not unused even before a caller;
#           and the tests read them). errcheck satisfied (SelectAsset's + resolveRelease's errors are checked).

# Scope guard: only resolve.go + resolve_test.go added; no existing file edited; NO swap.go.
git status --short
# Expected: ?? internal/upgrade/resolve.go  ?? internal/upgrade/resolve_test.go  (only).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new ResolveTarget tests (focused).
go test ./internal/upgrade/ -run 'ResolveTarget' -v
# Expected: PASS — LatestStable (endpoint + asset); Prerelease (endpoint + pre-asset); Version (tags endpoint + asset);
#           Version-precedence-over-Prerelease (tags hit, NOT array); per-platform (linux/darwin success + windows→ErrNoMatchingAsset);
#           empty-GOOS⇒runtime default; Client-error propagation (ErrNoReleases/ErrRateLimited via errors.Is); zero-on-error.

# The full upgrade package incl. the parallel detect/download/delegate tests (no shared mutable state — each test its own httptest server).
go test ./internal/upgrade/... -v
go test -race ./internal/upgrade/...
# Expected: green (race detector). releases_test.go + download_test.go + detect_test.go (parallel) + delegate_test.go (parallel)
#           all pass unaffected — resolve.go is additive and imports none of them.

# Full repo suite.
make test
# Expected: green. resolve.go is additive, stdlib-only, imports no internal/*.

# NOTE: there is NO coverage-gate on internal/upgrade (the §20.3 gate is internal/{git,provider,generate,config} only). The
# 3-branch + per-platform + runtime-default + error tests give strong coverage of resolveRelease + ResolveTarget regardless.
```

### Level 3: Integration Testing (System Validation)

```bash
# There is no integration/e2e surface for this task — ResolveTarget has no production caller yet (the command is P1.M4; the
# runUpgrade dispatcher is P1.M4.T2). The unit tests (Level 2) ARE the contract: they inject an httptest fake server + a real
# *Client and assert the exact endpoint + asset + sentinel for every branch/platform/error. A full e2e (real GitHub release
# resolved → downloaded → swapped) is P1.M4.T3.S2/S3.

# Sanity: the package still builds into the binary (no downstream compile break from the new symbols).
go build ./...

# Confidence check (optional, NOT CI — hits the REAL GitHub API): a scratch program can confirm ResolveTarget returns a real
# release+asset against the production endpoint (delete after; do NOT commit a network test):
cat > /tmp/sc_resolve_check.go <<'EOF'
package main
import ("context";"fmt";"runtime";"github.com/dabstractor/stagecoach/internal/upgrade")
func main() {
	c := &upgrade.Client{Repo: "dabstractor/stagecoach"} // real GitHub; unauthenticated (rate-limited)
	rel, asset, err := upgrade.ResolveTarget(context.Background(), c, upgrade.ResolveOptions{})
	fmt.Printf("rel=%+v asset=%+v err=%v (GOOS=%s GOARCH=%s)\n", rel, asset, err, runtime.GOOS, runtime.GOARCH)
}
EOF
# (MANUAL only — CI uses the httptest fakes; a real-API test is flaky/rate-limited. Do not commit it.)
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: resolve.go is a FILE comment, not a package doc.
head -3 internal/upgrade/resolve.go | grep -q '^// resolve.go' && echo "OK: file comment"
grep -c '^// Package upgrade' internal/upgrade/resolve.go
# Expected: 0 (releases.go owns the package doc).

# Guard 2: no internal/* imports (walled off, FR-U12); no fmt (errors propagate unwrapped).
grep -nE 'stagecoach/internal|"fmt"' internal/upgrade/resolve.go
# Expected: empty. resolve.go imports only context/runtime/strings.

# Guard 3: stdlib-only — no new go.mod require.
git diff go.mod go.sum
# Expected: empty.

# Guard 4: ResolveTarget exists + calls the 3 Client methods + SelectAsset (not a re-implementation).
grep -n 'func ResolveTarget' internal/upgrade/resolve.go
grep -cE 'c\.ReleaseByTag|c\.LatestAdmittingPrereleases|c\.LatestStable|SelectAsset\(' internal/upgrade/resolve.go
# Expected: 1 ResolveTarget; ≥4 hits across the 3 Client methods + SelectAsset (resolveRelease has the 3; ResolveTarget has SelectAsset).

# Guard 5: NO Compare / CurrentSemver call (THE scope boundary — up-to-date is the command layer's job).
grep -nE 'Compare\(|CurrentSemver' internal/upgrade/resolve.go
# Expected: empty. (If non-empty, you've added the up-to-date gate — remove it; that's P1.M4.T2.)

# Guard 6: GOOS/GOARCH default to runtime when empty (the injectable-default pattern).
grep -nE 'runtime\.GOOS|runtime\.GOARCH' internal/upgrade/resolve.go
# Expected: ≥2 hits (the two `if x == "" { x = runtime.GO… }` lines).

# Guard 7: NO swap.go created (P1.M3.T2 owns it).
ls internal/upgrade/swap.go 2>/dev/null && echo "FAIL: swap.go exists (P1.M3.T2 conflict)" || echo "OK: no swap.go"
git status --short | grep swap.go
# Expected: "OK: no swap.go" + empty grep.

# Guard 8: ZERO production callers of ResolveTarget (consumer is P1.M4.T2).
grep -rn 'upgrade.ResolveTarget(\|\.ResolveTarget(' --include='*.go' internal/ cmd/ pkg/ | grep -v '_test.go' | grep -v 'func ResolveTarget'
# Expected: empty (no caller outside resolve.go + tests).

# Guard 9: scope — only 2 files added.
git status --porcelain
# Expected: ?? internal/upgrade/resolve.go  ?? internal/upgrade/resolve_test.go  (only).

# Regression: the parallel detect/download/delegate/releases tests still pass (no shared state).
go test ./internal/upgrade/ -run 'Detect|Download|SelectAsset|Delegate|Client_|Release' -v
# Expected: all PASS (the sibling tests, unaffected).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + cross-build (windows/linux/darwin) clean
- [ ] `go vet ./internal/upgrade/...` clean
- [ ] `gofmt -l internal/upgrade/resolve.go internal/upgrade/resolve_test.go` empty
- [ ] `go test -race ./internal/upgrade/...` green (incl. the parallel detect/download/delegate tests)
- [ ] `make test` + `make lint` pass; `go.mod`/`go.sum` unchanged

### Feature Validation
- [ ] `ResolveOptions{Version, Prerelease, GOOS, GOARCH}` exists with the 4 documented fields
- [ ] Version != "" ⇒ ReleaseByTag (precedence over Prerelease); Version=="" && Prerelease ⇒ LatestAdmittingPrereleases; else LatestStable
- [ ] empty GOOS/GOARCH ⇒ runtime.GOOS/GOARCH; non-empty ⇒ injected (per-platform tests pass off-host)
- [ ] SelectAsset's (Asset, error) returned alongside the Release; ErrNoMatchingAsset propagates (errors.Is-reachable)
- [ ] Client errors propagate UNWRAPPED (errors.Is(err, ErrNoReleases|ErrRateLimited) works)
- [ ] zero values returned on any error
- [ ] ResolveTarget does NOT call Compare/CurrentSemver (grep guard 5 empty)

### Scope-Boundary Validation
- [ ] `git status` == only `internal/upgrade/resolve.go` + `internal/upgrade/resolve_test.go` (new)
- [ ] NO swap.go file (P1.M3.T2 owns it); NO edit to releases.go/download.go/version.go/detect.go/delegate.go
- [ ] NO `internal/*` import (FR-U12); stdlib only (context/runtime/strings); NO fmt
- [ ] NO package doc in resolve.go (releases.go owns it); file comment only
- [ ] NO Compare/up-to-date gate (command layer P1.M4.T2); NO download/verify/swap (P1.M3.T1.S2 + P1.M3.T2); NO cobra/command (P1.M4)
- [ ] Grep guards 1–9 (Level 4) all pass

### Code Quality & Docs
- [ ] Mirrors the package conventions (injectable seam, exported sentinels propagated, file-comment-not-package-doc, canned httptest tests)
- [ ] ResolveTarget godoc cites FR-U5 steps 2-3 + precedence + GOOS/GOARCH-runtime-default + the no-up-to-date-check boundary
- [ ] Tests REUSE releases_test.go's newFakeClient/cannedLatest/cannedReleases/statusServer (no re-declaration)

---

## Anti-Patterns to Avoid

- ❌ Don't add a Compare/up-to-date check to ResolveTarget. The contract's point 3 says "Validate the target tag is newer than
  current via Compare" but is OVERRIDDEN by the parenthetical DECISION ("the --check/up-to-date determination is in the command
  layer P1.M4.T2; ResolveTarget just returns the chosen release+asset"). ResolveTarget is fetch+select only — it does not know
  the running version and does not decide "already up to date" / "target older than current". Adding it here duplicates the
  command layer, breaks `--check` (which needs the comparison but a different exit path), and forces a version.go import (scope
  creep + a walled-off violation if it pulled internal/*). Grep guard 5 enforces this.
- ❌ Don't name the file swap.go. The contract offers "swap.go (or resolve.go)", but P1.M3.T2.S1 (Backup + atomic swap) creates
  swap.go — using it here collides with that sibling. resolve.go is the correct name (ResolveTarget resolves; the on-disk swap
  is P1.M3.T2's job). Grep guard 7 enforces this.
- ❌ Don't re-wrap errors with fmt.Errorf. The Client methods and SelectAsset already wrap the exported sentinels (ErrNoReleases/
  ErrRateLimited/ErrHTTP/ErrNoMatchingAsset) with %w and carry full diagnostics (path/status/asset-name). ResolveTarget returns
  them AS-IS so the command layer's `errors.Is` works cleanly. Re-wrapping (`fmt.Errorf("resolve: %w", err)`) adds nothing and
  means importing fmt for no reason. Just `return Release{}, Asset{}, err`.
- ❌ Don't invert the precedence (channel before version). `opts.Version` (after TrimSpace) MUST be checked FIRST — a pinned
  `--version v1.2.3` is an explicit user instruction that overrides `--prerelease`. Checking Prerelease first would silently
  ignore `--version` when both are set. Order in resolveRelease: Version → Prerelease → LatestStable.
- ❌ Don't check `opts.Version != ""` raw — use `strings.TrimSpace(opts.Version) != ""`. A whitespace-only " " is effectively
  empty; checking raw would send it to ReleaseByTag (which then errors ErrHTTP) instead of falling through to the channel.
  Trim once and pass the trimmed value to ReleaseByTag.
- ❌ Don't hardcode GOOS/GOARCH to runtime. The contract's test output requires "asset selection per GOOS/GOARCH" — to test
  linux/darwin/windows OFF the test host's platform, GOOS/GOARCH must be injectable opts fields (defaulting to runtime when
  empty). Hardcoding to runtime means the per-platform tests can only ever exercise the host platform. The injectable default
  mirrors the package convention (Client.HTTP/BaseURL, DelegateOptions.Env).
- ❌ Don't define a Client interface for testability. The contract's mocking is "httptest fake server + injectable Client" —
  the seam is Client.BaseURL/HTTP (the NETWORK layer), not the Client type. ResolveTarget takes a concrete `c *Client`, matching
  releases_test.go's newFakeClient (which returns *Client). An interface here is needless abstraction.
- ❌ Don't re-implement asset matching or network logic. SelectAsset (download.go) and the Client methods (releases.go) are
  LANDED and fully tested. ResolveTarget COMPOSES them — it adds zero new network or match code. If you find yourself writing
  a URL builder or an asset-name matcher in resolve.go, you are duplicating landed code.
- ❌ Don't half-populate the error return. On any error, return `(Release{}, Asset{}, err)` — standard Go. The SelectAsset
  error already names the wanted asset + GOOS/GOARCH, so zeroing the Release loses no diagnostic. Returning a partial Release
  is inconsistent and surprising.
- ❌ Don't write a `// Package upgrade` doc in resolve.go. releases.go line 1 owns it; a second one splits the package overview.
  Start resolve.go with a FILE comment (`// resolve.go implements…`) — exactly as detect.go/download.go/delegate.go do.
- ❌ Don't add a nil-guard for `c`. The package convention (releases.go) does not nil-guard the Client receiver — `c.LatestStable`
  panics on a nil `c`, and the command layer (P1.M4) always constructs one. A nil-guard here diverges from the convention for
  no real safety gain. (If you want defense, document "c must be non-nil" in the godoc — don't add code.)
- ❌ Don't duplicate the empty-tag guard from ReleaseByTag. ReleaseByTag (releases.go:196) already rejects empty/whitespace tags.
  ResolveTarget decides the BRANCH on TrimSpace(Version) and only calls ReleaseByTag with a non-empty trimmed value — it does
  not need its own "if Version is empty return error" guard (an empty Version correctly falls through to the channel path).
- ❌ Don't run real GitHub API calls in tests. CI must be hermetic + not rate-limited. REUSE releases_test.go's `newFakeClient`
  (httptest.Server) + the canned payloads. A real-API "confidence check" is a manual scratch program you delete, never a
  committed test.
- ❌ Don't edit releases.go / download.go / version.go / detect.go / delegate.go. resolve.go is a standalone additive file in
  package upgrade; it calls the landed Client methods + SelectAsset but shares no symbol with those files. The parallel
  delegate.go (P1.M2.T2.S1) and detect.go (P1.M2.T1.S1) are in flight — no overlap (delegate = manager path; resolve = direct path).

---

## Confidence Score: 9/10

The verbatim ResolveTarget/ResolveOptions/resolveRelease API, the THREE Client method signatures it composes (LatestStable/
ReleaseByTag/LatestAdmittingPrereleases, all `(Release, error)`, all LANDED), the SelectAsset signature + the exported
ErrNoMatchingAsset sentinel, the precedence rule (Version > Prerelease > LatestStable), the load-bearing DECISION that
ResolveTarget does NOT call Compare (command-layer scope), the GOOS/GOARCH-runtime-default + why it must be injectable for
the per-platform tests, the unwrap-don't-rewrap error rule, the resolve.go-vs-swap.go file-name choice (swap.go is P1.M3.T2's),
the file-comment convention, and the exact httptest test idiom to clone (newFakeClient/cannedLatest/cannedReleases/statusServer
+ the in-package assetName helper) are all verified against the real code. The -1 from 10/10 reflects the one judgment call in
the test plan: the prerelease-channel SUCCESS-asset case needs a custom payload (cannedReleases's pre-release asset "pre.tar.gz"
doesn't match the gorereaser name, so it can't double as an asset-success fixture) — the PRP spells out the clean custom-payload
approach, but it's the one place an implementer could fumble the fixture and get a confusing ErrNoMatchingAsset instead of a
clean success assertion. No new dep, no internal/* import, walled off, file-comment-not-package-doc, no symbol clash with the
parallel delegate.go/detect.go, zero production callers by design.