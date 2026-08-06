name: "P1.M1.T3.S2 — Asset + checksums.txt download + SHA256 verification primitives (goreleaser archives)"
description: >
  Add ONE new file internal/upgrade/download.go to the EXISTING package upgrade (joins version.go from
  P1.M1.T1.S2 and releases.go from P1.M1.T3.S1). Exports the download+verify primitives the direct-swap
  self-update path (P1.M3.T1.S2) resolves its target archive through (§9.29 FR-U5 steps 3–4, FR-U11).
  Five functions: (1) SelectAsset(release, goos, goarch) (Asset, error) — pure; maps GOOS/GOARCH to the
  goreleaser archive name (stagecoach_<ver-no-v>_<os>_<arch>.tar.gz; windows → .zip) and finds it in
  release.Assets. (2) VerifySHA256(path, want) error — pure; streams crypto/sha256 over the file and
  constant-time-compares (crypto/subtle) the hex digest to want. (3) (c *Client) DownloadFile(ctx, url,
  dest) error — method; streams the browser_download_url (a 302 → objects.githubusercontent.com that
  net/http follows) to dest, reusing the Client's *http.Client + User-Agent + Bearer token. (4)
  (c *Client) FetchChecksums(ctx, release) (map[filename]hexsum, error) — method; finds the
  *_checksums.txt asset, downloads it, parses "<64hex>  <filename>" lines. (5) (c *Client)
  DownloadAndVerifyArchive(ctx, release, goos, goarch, destDir) (archivePath, error) — method;
  composes select→fetch-checksums→download→verify, returning the VERIFIED archive path and cleaning its
  partial dest file on any failure (FR-U11 staging analog). Five NEW sentinels (ErrNoMatchingAsset,
  ErrNoChecksumsFile, ErrChecksumMissing, ErrChecksumParse, ErrChecksumMismatch); reuses ErrHTTP from
  releases.go for transport/status failures. Stdlib-only (net/http, crypto/sha256, crypto/subtle,
  encoding/hex, path/filepath, os, io, fmt, strings, context, errors) — NO new go.mod require. Tests via
  httptest (canned archive bytes + checksums.txt); never the real GitHub API in CI. ADDITIVE: does NOT
  edit releases.go (parallel item owns it); download.go gets a file comment only (NO competing package doc).

---

## Goal

**Feature Goal**: Provide the asset-selection + download + SHA256-verify primitives that turn a resolved
`Release` (from P1.M1.T3.S1) into a verified-on-disk platform archive, ready for extract+sanity-run+swap
(P1.M3.T1.S2). This is §9.29 FR-U5 steps 3–4 (select the GOOS/GOARCH asset; download + SHA256-verify
against checksums.txt) and the staging-tier of FR-U11 (abort-and-clean on any failure; never leave a
corrupt/tampered artifact). It is part of stagecoach's SOLE network surface (PRD §19 named exception) and
is walled off from the commit core (FR-U12: no lock, no repo, no provider, no index/ref).

**Deliverable**: One new production file `internal/upgrade/download.go` and one new test file
`internal/upgrade/download_test.go`, both in `package upgrade`. No edits to `releases.go` (owned by the
parallel sibling P1.M1.T3.S1) or `version.go` (owned by P1.M1.T1.S2). No new `go.mod` require (stdlib only).

**Success Definition**:
- `SelectAsset(release, "darwin", "arm64")` returns the `stagecoach_<v-no-v>_darwin_arm64.tar.gz` Asset;
  `SelectAsset(release, "windows", "amd64")` returns the `..._windows_amd64.zip` Asset; an unknown
  `(goos,goarch)` with no matching asset → `errors.Is(err, ErrNoMatchingAsset)`.
- `VerifySHA256(file, correctHex)` returns nil; `VerifySHA256(file, wrongHex)` →
  `errors.Is(err, ErrChecksumMismatch)`; uppercase/tabby `want` is normalized (still matches).
- `(c *Client).DownloadFile(ctx, ts.URL+"/dl/blob", dest)` streams an httptest-served N-byte body to
  `dest` byte-for-byte; a 500 from the server and a closed server → `errors.Is(err, ErrHTTP)`.
- `(c *Client).FetchChecksums(ctx, release)` returns `map[name]hex` parsed from a served
  `stagecoach_<v>_checksums.txt`; a release with no `*_checksums.txt` asset → `ErrNoChecksumsFile`; a
  malformed line → `ErrChecksumParse`.
- `(c *Client).DownloadAndVerifyArchive(ctx, release, goos, goarch, destDir)` happy path returns a path
  whose file exists and whose SHA256 equals the checksums.txt line; a TAMPERED body (sum ≠ line) →
  `ErrChecksumMismatch` AND the partial `dest` file is removed (cleanup); a missing checksums line →
  `ErrChecksumMissing`; no matching asset → `ErrNoMatchingAsset`.
- Every network request sets `User-Agent: stagecoach/<ver>`; a non-empty `Token` adds `Authorization:
  Bearer <t>` (same header contract as releases.go).
- `go build ./...`, `go vet ./internal/upgrade/...`, `go test ./internal/upgrade/...`, `go test -race
  ./internal/upgrade/...` all green; `gofmt -l` clean; `go.mod`/`go.sum` unchanged.

## User Persona (if applicable)

**Target User**: The direct-swap self-update path (P1.M3.T1.S1 resolve + P1.M3.T1.S2 download→verify→
extract→sanity-run), which consumes `DownloadAndVerifyArchive` to obtain a verified archive before the
atomic swap (P1.M3.T2). Also the npm postinstall (P2.M1.T1.S2) is the JS-side analog — this Go side is
the reference impl.

**Use Case**: `stagecoach upgrade` resolves the target `Release` (P1.M1.T3.S1) then calls
`DownloadAndVerifyArchive(ctx, release, runtime.GOOS, runtime.GOARCH, tmpDir)` to fetch the platform
archive and prove (via checksums.txt + SHA256) that the bytes are authentic before extraction. Any
failure (no asset / missing line / tampered bytes / network) aborts cleanly with a typed error and no
leftover corrupt file — the swap never happens.

**Pain Points Addressed**: Turns a `Release` (metadata) into a trustable on-disk artifact via a single
composable call; isolates all download/verify logic in one testable stdlib file; guarantees a tampered
or corrupt download can never reach the binary-swap step.

## Why

- **§9.29 FR-U5 steps 3–4** (PRD §9.29): "Select the GOOS/GOARCH asset; download + verify SHA256 against
  checksums.txt (a hard gate)." These primitives ARE steps 3–4.
- **FR-U11** (abort-before-write): failures here remove the partial staging file; the real write
  (binary swap) is downstream in P1.M3.T2 and only ever runs on a verified artifact.
- **Security gate**: SHA256-verify against the project's own checksums.txt is the integrity guarantee for
  the self-updating binary — a tampered/CDN-corrupt download must be detected and rejected before swap.
- **Foundation milestone (P1.M1) primitive #5** (after exit-code 6, semver Compare, `[upgrade]` config,
  and the Releases metadata client): the download/verify seam the rest of P1 (resolve→swap→rollback→CLI)
  builds on.
- **Bounded scope**: asset select + download + checksum parse + SHA256 verify + one composer. NO extract,
  NO sanity-run, NO binary swap, NO backup, NO cobra command, NO env read, NO Compare-vs-current — all
  deferred to siblings (P1.M3 / P1.M4).

## What

**User-visible behavior**: none directly (no caller until P1.M3.T1.S2). The artifact is a set of
reusable, httptest-testable functions + five typed errors.

**Technical change** (all in `internal/upgrade/download.go`, `package upgrade`):

```go
// New sentinels (declared here; ErrHTTP is REUSED from releases.go for transport/status failures).
var (
	ErrNoMatchingAsset  = errors.New("upgrade: no release asset matches the target GOOS/GOARCH")
	ErrNoChecksumsFile  = errors.New("upgrade: release has no checksums.txt asset")
	ErrChecksumMissing  = errors.New("upgrade: selected asset has no entry in checksums.txt")
	ErrChecksumParse    = errors.New("upgrade: checksums.txt has a malformed line")
	ErrChecksumMismatch = errors.New("upgrade: downloaded archive SHA256 does not match checksums.txt")
)

// Pure functions (no network) ----------------------------------------------------

// SelectAsset returns the goreleaser platform archive for (goos, goarch) from release.Assets.
// Expected name: stagecoach_<Tag-without-leading-v>_<goos>_<goarch>.tar.gz  (windows → .zip).
func SelectAsset(release Release, goos, goarch string) (Asset, error)

// VerifySHA256 streams SHA256 over path and constant-time-compares the hex digest to want
// (64 lowercase hex; want is normalized — trimmed + lowercased). nil on match.
func VerifySHA256(path, want string) error

// Methods on *Client (network; reuse c.HTTP + User-Agent + Bearer token) ----------------

// DownloadFile streams url (a browser_download_url absolute URL; net/http follows the 302 to
// objects.githubusercontent.com) to dest. Multi-MB safe: never buffers the whole body. Non-2xx /
// transport failure → ErrHTTP (wrapped, same surface as releases.go).
func (c *Client) DownloadFile(ctx context.Context, url, dest string) error

// FetchChecksums finds the *_checksums.txt asset in release, downloads it, and parses each
// "<64hex>  <filename>" line into map[filename]hexsum. No checksums asset → ErrNoChecksumsFile;
// malformed line → ErrChecksumParse; transport failure → ErrHTTP.
func (c *Client) FetchChecksums(ctx context.Context, release Release) (map[string]string, error)

// DownloadAndVerifyArchive composes SelectAsset → FetchChecksums → DownloadFile → VerifySHA256,
// returning the verified archive path under destDir. On ANY failure it os.Removes its partial dest
// file (FR-U11 staging analog) and returns a typed error. destDir must already exist (caller-owned temp).
func (c *Client) DownloadAndVerifyArchive(ctx context.Context, release Release, goos, goarch, destDir string) (string, error)
```

### Success Criteria
- [ ] `download.go` defines the five functions and five sentinels exactly as above (signature + placement).
- [ ] `SelectAsset` computes `stagecoach_<TrimPrefix(Tag,"v")>_<goos>_<goarch>` + `.zip` (windows) /
      `.tar.gz` (else), finds the `Asset` with that exact `Name`, returns it or `ErrNoMatchingAsset`.
- [ ] `VerifySHA256` opens+streams the file through `sha256.New()`, hex-encodes, and compares to `want`
      via `subtle.ConstantTimeCompare`; normalizes `want` (trim+lower); mismatch → `ErrChecksumMismatch`;
      open/read I/O error → a plain wrapped error (NOT the mismatch sentinel).
- [ ] `DownloadFile` builds a GET against the ABSOLUTE `url` (NOT BaseURL), sets `User-Agent` + optional
      `Bearer`, streams `resp.Body`→`os.Create(dest)` via `io.Copy`, drains+closes the body; non-2xx and
      transport failures → `ErrHTTP` (wrapped).
- [ ] `FetchChecksums` finds the `*_checksums.txt` asset, downloads (small text — may buffer), splits on
      `\n`, `strings.Fields` each non-blank line, validates 2 fields + `isHex64`, builds the map; no asset
      → `ErrNoChecksumsFile`; bad line → `ErrChecksumParse`.
- [ ] `DownloadAndVerifyArchive` composes the four; on any error between download and verify it
      `os.Remove(dest)` (the partial file) before returning; returns `filepath.Join(destDir, asset.Name)`
      on success.
- [ ] `download_test.go` (white-box `package upgrade`) covers the full matrix (Validation Level 2) against
      an `httptest.Server`, asserting typed errors via `errors.Is`.
- [ ] `download.go` has a file-level comment (blank-line-separated from `package upgrade` — NOT a
      competing package doc) citing FR-U5 steps 3–4 / FR-U11.
- [ ] `go build ./...` + `go vet ./internal/upgrade/...` + `go test ./internal/upgrade/...` +
      `go test -race ./internal/upgrade/...` green; `gofmt -l internal/upgrade/` clean; `go.mod` unchanged.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact function signatures + placement (method vs package-func), the exact goreleaser
filename derivation (tag-strip-v + os/arch identity + windows→zip), the exact checksums.txt line format
(`<64hex>  <name>`, two spaces) with the parse rule (`strings.Fields`, validate 2 fields + isHex64), the
exact SHA256 verify recipe (`sha256.New`+`io.Copy`+`hex.EncodeToString`+`subtle.ConstantTimeCompare`), the
exact download contract (absolute URL, follow-redirect, stream, drain+close, ErrHTTP on non-2xx/transport),
the five sentinels + the reuse of `ErrHTTP`, the types to consume (`Release`/`Asset`/`Client`) and the
strictly-additive file boundary (do not edit the parallel sibling's `releases.go`), the httptest test
matrix, the stdlib-only constraint, and the scope fences (no env, no extract, no swap, no Compare-vs-current).

### Documentation & References

```yaml
# MUST READ — the authoritative artifact-naming + checksum format + redirect contract
- docfile: plan/017_397abce9deb1/architecture/external_deps.md
  why: "§1 gives the asset download contract (browser_download_url → objects.githubusercontent.com 302;
        net/http follows redirects; stream to a temp file — archives are multi-MB) and the EXACT goreleaser
        artifact-name template to match. §2 is the checksums.txt format spec: one line per artifact
        '<64-hex-sha256>  <artifact-filename>' (two spaces); a missing line or mismatch aborts BEFORE any
        write (FR-U11). This file IS the format spec the code must match."
  critical: "goreleaser {{.Version}} = the git tag WITHOUT the leading 'v' (tag v1.0.0 → Version 1.0.0),
             but Release.Tag from the GitHub API RETAINS the 'v'. SelectAsset MUST strings.TrimPrefix(Tag,'v').
             windows → .zip, everything else → .tar.gz. For our 6 targets goreleaser .Os/.Arch == GOOS/GOARCH
             (identity)."

# MUST READ — the ACTUAL release config in THIS repo (confirms name_template + checksum template verbatim)
- file: .goreleaser.yaml
  why: "archives[].name_template = '{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}' (→
        stagecoach_1.0.0_linux_amd64.tar.gz); format_overrides goos:windows → zip; checksum.name_template =
        '{{ .ProjectName }}_{{ .Version }}_checksums.txt' (→ stagecoach_1.0.0_checksums.txt), algorithm
        sha256. ldflags comment: '{{.Version}} = tag without leading v'. This is ground truth for SelectAsset
        and FetchChecksums's expected names."
  pattern: "Match the EXACT name_template output. project_name = stagecoach."
  gotcha: "If a future goreleaser bump changes the template, SelectAsset's computed name must follow. Pin
           the expectation to this file's current templates."

# MUST READ — the sibling whose types/functions this subtask consumes + whose file it must NOT edit
- docfile: plan/017_397abce9deb1/P1M1T3S1/PRP.md
  why: "Defines Client{HTTP,BaseURL,Repo,Token}, Release{Tag,Assets}, Asset{Name,DownloadURL,Size}, and
        ErrNoReleases/ErrRateLimited/ErrHTTP in internal/upgrade/releases.go (same package). THIS subtask
        REUSES Client (as methods), Release, Asset, and ErrHTTP, and ADDS download.go alongside — never
        editing releases.go. Treat P1.M1.T3.S1 as a CONTRACT (it is being implemented in parallel)."
  critical: "ErrHTTP is the shared transport/status sentinel — do NOT invent ErrDownload; reuse it so the
             error surface stays {ErrHTTP, ErrNoMatchingAsset, ErrNoChecksumsFile, ErrChecksumMissing,
             ErrChecksumParse, ErrChecksumMismatch}. Client's newReq() in releases.go sets User-Agent +
             optional Bearer — download.go must set the SAME headers on the asset GET (build its own request
             against the ABSOLUTE DownloadURL; BaseURL is metadata-only and is NOT used for asset downloads)."

# MUST READ — the package this file joins (doc/Compare/currentVersion; do NOT edit)
- file: internal/upgrade/version.go
  why: "download.go is ADDED to package upgrade. The package doc lives HERE (lines 1-16) — do NOT add a
        competing '// Package upgrade' block in download.go (duplicate-package-doc lint). var currentVersion
        (line 21) feeds the User-Agent (same as releases.go). Compare is unused by this subtask."
  gotcha: "One package doc, ever. download.go gets a FILE comment separated from 'package upgrade' by a
           blank line — identical posture to releases.go."

# MUST READ — the typed-error convention to mirror (codebase-wide)
- file: internal/git/git.go
  why: "Line 706: var ErrCASFailed = errors.New(...); line 731: fmt.Errorf('%w (exit %d): %s', ErrCASFailed,
        code, ...). THE codebase pattern: package-level var sentinel + %w wrap at use sites so BOTH the
        sentinel (errors.Is) AND a human message are reachable. internal/generate/generate.go:85/90/95 and
        internal/exitcode/exitcode.go:38 (ErrUpdateAvailable) follow the same pattern."
  pattern: "Declare each sentinel as errors.New('upgrade: ...'). Wrap: fmt.Errorf('select asset %s/%s: %w',
            goos, goarch, ErrNoMatchingAsset). Test via errors.Is(err, ErrX)."

# MUST READ — the §19 scope + the FRs this implements
- prd: §19
  why: "download.go is part of the SOLE network surface (stagecoach upgrade). Doc comment must reflect that
        it fetches ONLY the project's own release artifacts + checksums."
- prd: §9.29 FR-U5 (steps 3–4), FR-U11, FR-U12
  why: "FR-U5 step 3 = select the GOOS/GOARCH asset; step 4 = download + SHA256-verify against checksums.txt
        (a hard gate). FR-U11 = abort-before-write (here: clean partial staging file on failure). FR-U12 =
        walled off from the commit core (no lock/repo/provider/index-ref)."

# LIBRARY — stdlib APIs the code uses (no doc fetch needed; anchors for the contract)
- url: https://pkg.go.dev/crypto/sha256
  why: "sha256.New() → hash.Hash; io.Copy(h, file) streams without buffering the whole file; h.Sum(nil) →
        [32]byte; encoding/hex.EncodeToString → 64-char lowercase hex string."
- url: https://pkg.go.dev/crypto/subtle#ConstantTimeCompare
  why: "subtle.ConstantTimeCompare([]byte(got), []byte(want)) returns 1 iff equal (constant-time). Use for the
        digest compare (both operands are 64 hex chars → length constant; no length leak)."
- url: https://pkg.go.dev/net/http#Client
  why: "Default CheckRedirect follows up to 10 redirects — so the browser_download_url →
        objects.githubusercontent.com 302 is followed automatically. No custom redirect handling needed."

# EXTERNAL — goreleaser checksum format (corroborates external_deps.md §2)
- url: https://goreleaser.com/customization/package/checksum/
  why: "Confirms goreleaser emits 'project_VERSION_checksums.txt', one '<sha256>  <filename>' line per
        artifact (two-space separator; lowercase hex). external_deps.md §2 is the in-repo authoritative copy;
        this URL is corroboration (FR-D5: verify at impl, record date)."
```

### Current Codebase tree (relevant slice)

```bash
internal/upgrade/
  version.go        # EXISTS (P1.M1.T1.S2) — package doc lives here; Compare @105; var currentVersion @21. <- DO NOT EDIT
  version_test.go   # EXISTS — white-box package upgrade, table-driven. Mirror style.
  releases.go       # EXISTS (P1.M1.T3.S1) — Client/Release/Asset + ErrNoReleases/ErrRateLimited/ErrHTTP. <- DO NOT EDIT (parallel sibling)
  releases_test.go  # EXISTS (P1.M1.T3.S1) — httptest newFakeClient(t, handler) helper. MIRROR for download_test.go.
  download.go       # DOES NOT EXIST — CREATE (5 funcs + 5 sentinels; methods reuse Client)
  download_test.go  # DOES NOT EXIST — CREATE (httptest matrix + pure SelectAsset/VerifySHA256 tests)
go.mod              # module github.com/dabstractor/stagecoach; go 1.22. UNCHANGED (all imports stdlib).
# crypto/sha256, crypto/subtle, encoding/hex, net/http, path/filepath, os, io, fmt, strings, context, errors — ALL stdlib.
```

### Desired Codebase tree with files to be added

```bash
internal/upgrade/
  version.go        # UNCHANGED
  releases.go       # UNCHANGED (parallel sibling owns it)
  download.go       # NEW — 5 funcs + 5 sentinels; reuses Client/Release/Asset/ErrHTTP from releases.go
  download_test.go  # NEW — httptest-driven, white-box package upgrade
go.mod              # UNCHANGED (no new require)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (tag-vs-Version, the #1 bug source): goreleaser filenames embed {{.Version}} = the tag WITHOUT
//   the leading "v" (tag v1.0.0 → "1.0.0"). But Release.Tag from the GitHub API RETAINS the "v" ("v1.0.0").
//   SelectAsset/FetchChecksums MUST strings.TrimPrefix(release.Tag, "v") before building the filename.
//   Forgetting this → computed name "stagecoach_v1.0.0_..." → no asset match → false ErrNoMatchingAsset.

// CRITICAL (windows → .zip, else .tar.gz): .goreleaser.yaml format_overrides: goos:windows → zip. SelectAsset
//   appends ".zip" when goos=="windows", else ".tar.gz". FetchChecksums's checksums name is always ".txt".

// CRITICAL (this file is ADDITIVE — never edit releases.go): releases.go is being implemented by the PARALLEL
//   sibling P1.M1.T3.S1. download.go is a NEW file in the same package; it references Client/Release/Asset/
//   ErrHTTP by bare name and declares its OWN five sentinels. Do NOT open releases.go for edits.

// CRITICAL (one package doc, not two): the package doc lives in version.go. Do NOT write a "// Package
//   upgrade ..." comment in download.go. Give it a FILE comment separated from `package upgrade` by a blank
//   line (identical to releases.go's posture).

// CRITICAL (stdlib-only posture): version.go commits to "no external dependency"; releases.go adds ZERO
//   requires. download.go must too. crypto/sha256, crypto/subtle, encoding/hex, net/http, path/filepath,
//   os, io, fmt, strings, context, errors are ALL stdlib. Do NOT pull a hashing helper lib or an HTTP client.

// CRITICAL (BaseURL is metadata-only): asset downloads use the ABSOLUTE DownloadURL
//   (https://github.com/.../releases/download/... → 302 → objects.githubusercontent.com). Do NOT prepend
//   c.BaseURL. Build the asset GET directly against the DownloadURL; set User-Agent + optional Bearer
//   (same headers as releases.go's newReq) so the download benefits from auth/rate-limit + a real UA.

// GOTCHA (net/http follows redirects by default): the browser_download_url 302s to
//   objects.githubusercontent.com. http.Client.Do follows up to 10 redirects automatically (default
//   CheckRedirect). Do NOT set a custom CheckRedirect unless you have a reason. The Bearer token IS sent
//   on the redirect hop (Go re-sends headers on same-scheme redirects); for public releases auth is
//   unnecessary but harmless and raises the rate limit.

// GOTCHA (stream, never buffer): archives are multi-MB. DownloadFile must io.Copy(resp.Body, file) in
//   chunks — NOT io.ReadAll(resp.Body) then os.WriteFile. FetchChecksums's checksums.txt IS small (text),
//   so buffering it (io.ReadAll) is fine.

// GOTCHA (drain+close the body for keep-alive): like releases.go, after io.Copy to the file do
//   `defer func(){ io.Copy(io.Discard, resp.Body); resp.Body.Close() }()`. Skipping it leaks connections.

// GOTCHA (constant-time compare is correct but length is constant anyway): subtle.ConstantTimeCompare
//   returns 0 immediately if lengths differ. Since got+want are always 64 hex chars, length is constant —
//   no timing leak. Still USE subtle (the item requires it + best practice); normalize want to lower+trim
//   first so an uppercase/whitespace checksum from a hand-edited file still matches.

// GOTCHA (checksums.txt parse — two-space separator): each line is "<64hex>  <filename>" (TWO spaces).
//   Use strings.Fields(line) — it collapses any whitespace run to fields, so the two-space vs one-space
//   ambiguity is irrelevant. Require exactly 2 fields + isHex64(fields[0]); store map[fields[1]]fields[0].
//   stagecoach archive names contain NO spaces, so a 2-field split is exact. Malformed line → ErrChecksumParse.

// GOTCHA (FR-U11 staging analog — clean your own partial file): DownloadAndVerifyArchive writes ONLY into
//   the caller-provided destDir (a staging temp dir). On ANY error AFTER it has created dest, os.Remove(dest)
//   before returning, so a tampered/truncated download never lingers. The caller owns destDir cleanup. The
//   REAL FR-U11 gate (never overwrite the running binary) is in P1.M3.T2 — this subtask never touches it.

// GOTCHA (I/O error ≠ checksum mismatch): VerifySHA256 open/read failures return a plain wrapped error
//   (fmt.Errorf("sha256 %s: %w", path, err)), NOT ErrChecksumMismatch. Reserve ErrChecksumMismatch for the
//   digest comparison only, so callers can branch "tampered/failed-verify" from "couldn't read the file".

// GOTCHA (SelectAsset exact-name match): compute the expected name and find the Asset whose Name == it
//   (exact equality, NOT a fuzzy/substring search — the convention is locked in .goreleaser.yaml). This is
//   precise and fails loud if goreleaser changes the template (better than silently picking a wrong asset).
```

## Implementation Blueprint

### Data models and structure

```go
// internal/upgrade/download.go (NEW) — package upgrade.
//
// Asset selection + download + SHA256 verification for stagecoach upgrade (§9.29 FR-U5 steps 3–4,
// FR-U11). Part of the package's SOLE network surface (§19 named exception): it fetches ONLY the
// project's own release archive + checksums — never credentials, diffs, or repo data. Stdlib-only.
package upgrade

// (Release, Asset, Client, ErrHTTP are declared in releases.go — same package; consumed by bare name.)

var (
	ErrNoMatchingAsset  = errors.New("upgrade: no release asset matches the target GOOS/GOARCH")
	ErrNoChecksumsFile  = errors.New("upgrade: release has no checksums.txt asset")
	ErrChecksumMissing  = errors.New("upgrade: selected asset has no entry in checksums.txt")
	ErrChecksumParse    = errors.New("upgrade: checksums.txt has a malformed line")
	ErrChecksumMismatch = errors.New("upgrade: downloaded archive SHA256 does not match checksums.txt")
)

// assetName computes the goreleaser archive name for (goos, goarch): tag with the leading "v"
// stripped, then "stagecoach_<v>_<os>_<arch>.tar.gz" (windows → ".zip"). Matches .goreleaser.yaml's
// archives[].name_template + format_overrides exactly.
func assetName(tag, goos, goarch string) string {
	ver := strings.TrimPrefix(tag, "v") // goreleaser {{.Version}} = tag without leading v
	name := "stagecoach_" + ver + "_" + goos + "_" + goarch
	if goos == "windows" {
		return name + ".zip"
	}
	return name + ".tar.gz"
}

// checksumsName computes the checksums file name: "stagecoach_<v>_checksums.txt".
func checksumsName(tag string) string {
	return "stagecoach_" + strings.TrimPrefix(tag, "v") + "_checksums.txt"
}

// isHex64 reports whether s is exactly 64 lowercase-or-uppercase hex chars.
func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/upgrade/download.go — sentinels, helpers, SelectAsset, VerifySHA256 (pure half)
  - PACKAGE: `package upgrade`. IMPORTS (stdlib only): context, crypto/sha256, crypto/subtle, encoding/hex,
    errors, fmt, io, net/http, os, path/filepath, strings.
  - FILE COMMENT: a blank-line-separated (NOT package-doc) comment citing FR-U5 steps 3–4 / FR-U11 + §19.
  - DECLARE the five sentinels (var block) EXACTLY as in the blueprint above.
  - IMPLEMENT unexported helpers: assetName(tag,goos,goarch), checksumsName(tag), isHex64(s).
  - IMPLEMENT SelectAsset(release, goos, goarch):
        want := assetName(release.Tag, goos, goarch)
        for _, a := range release.Assets { if a.Name == want { return a, nil } }
        return Asset{}, fmt.Errorf("select asset %s/%s (want %s): %w", goos, goarch, want, ErrNoMatchingAsset)
  - IMPLEMENT VerifySHA256(path, want):
        f, err := os.Open(path); if err != nil → return fmt.Errorf("sha256 open %s: %w", path, err)
        defer f.Close()
        h := sha256.New(); if _, err := io.Copy(h, f); err != nil → return fmt.Errorf("sha256 read %s: %w", path, err)
        got := hex.EncodeToString(h.Sum(nil))
        wantNorm := strings.ToLower(strings.TrimSpace(want))
        if subtle.ConstantTimeCompare([]byte(got), []byte(wantNorm)) != 1 {
            return fmt.Errorf("sha256 %s: got %s want %s: %w", path, got, wantNorm, ErrChecksumMismatch)
        }
        return nil
  - NAMING: exported SelectAsset/VerifySHA256; unexported assetName/checksumsName/isHex64.
  - DO NOT (yet): the three Client methods (Task 2).

Task 2: ADD the three *Client methods to internal/upgrade/download.go (network half)
  - (c *Client) newDownloadReq(ctx, url) (*http.Request, error):
        req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil); if err != nil → return nil, err
        req.Header.Set("Accept", "application/octet-stream")   // or omit; harmless
        req.Header.Set("User-Agent", "stagecoach/"+currentVersion)   // currentVersion is same-package (version.go:21)
        if c.Token != "" { req.Header.Set("Authorization", "Bearer "+c.Token) }
        return req, nil
  - (c *Client) httpClient() *http.Client  → reuse/releases.go's helper IF exported; else define a local
        `clientOr(c.HTTP, http.DefaultClient)` unexported helper. (releases.go's httpClient() is lowercase
        = unexported → SAME package → callable here. Prefer reusing it to avoid divergence.)
  - (c *Client) DownloadFile(ctx, url, dest):
        req, err := c.newDownloadReq(ctx, url); if err != nil → return fmt.Errorf("download %s: %w", url, ErrHTTP)
        resp, err := c.httpClient().Do(req)
        if err != nil → return fmt.Errorf("download %s: %v: %w", url, err, ErrHTTP)
        defer func(){ io.Copy(io.Discard, resp.Body); resp.Body.Close() }()
        if resp.StatusCode < 200 || resp.StatusCode >= 300 {
            return fmt.Errorf("download %s: status %d: %w", url, resp.StatusCode, ErrHTTP)
        }
        f, err := os.Create(dest); if err != nil → return fmt.Errorf("download create %s: %w", dest, err)
        // stream; never buffer. On copy error, close + remove the partial file.
        _, copyErr := io.Copy(f, resp.Body); cerr := f.Close()
        if copyErr != nil { os.Remove(dest); return fmt.Errorf("download %s: %v: %w", url, copyErr, ErrHTTP) }
        if cerr  != nil { os.Remove(dest); return fmt.Errorf("download close %s: %w", dest, cerr) }
        return nil
  - (c *Client) FetchChecksums(ctx, release):
        // find the checksums asset (exact checksumsName first; fall back to HasSuffix "_checksums.txt")
        var url string; found := false
        wantName := checksumsName(release.Tag)
        for _, a := range release.Assets {
            if a.Name == wantName || strings.HasSuffix(a.Name, "_checksums.txt") { url = a.DownloadURL; found = true; break }
        }
        if !found → return nil, fmt.Errorf("release %s: %w", release.Tag, ErrNoChecksumsFile)
        // download to a buffer (small text file; buffering is fine)
        req, err := c.newDownloadReq(ctx, url); if err != nil → return nil, fmt.Errorf("checksums: %w", ErrHTTP)
        resp, err := c.httpClient().Do(req)
        if err != nil → return nil, fmt.Errorf("checksums %s: %v: %w", url, err, ErrHTTP)
        defer func(){ io.Copy(io.Discard, resp.Body); resp.Body.Close() }()
        if resp.StatusCode < 200 || resp.StatusCode >= 300 → return nil, fmt.Errorf("checksums %s: status %d: %w", url, resp.StatusCode, ErrHTTP)
        body, err := io.ReadAll(resp.Body); if err != nil → return nil, fmt.Errorf("checksums read: %v: %w", err, ErrHTTP)
        // parse
        sums := map[string]string{}
        for _, line := range strings.Split(string(body), "\n") {
            line = strings.TrimSpace(line); if line == "" { continue }
            fields := strings.Fields(line)
            if len(fields) != 2 || !isHex64(fields[0]) {
                return nil, fmt.Errorf("checksums line %q: %w", line, ErrChecksumParse)
            }
            sums[fields[1]] = strings.ToLower(fields[0])
        }
        return sums, nil
  - (c *Client) DownloadAndVerifyArchive(ctx, release, goos, goarch, destDir):
        asset, err := SelectAsset(release, goos, goarch); if err != nil → return "", err
        sums,  err := c.FetchChecksums(ctx, release);       if err != nil → return "", err
        want,  ok := sums[asset.Name]
        if !ok → return "", fmt.Errorf("asset %q not in checksums.txt: %w", asset.Name, ErrChecksumMissing)
        dest := filepath.Join(destDir, asset.Name)
        if err := c.DownloadFile(ctx, asset.DownloadURL, dest); err != nil { os.Remove(dest); return "", err }
        if err := VerifySHA256(dest, want); err != nil { os.Remove(dest); return "", err }
        return dest, nil
  - NAMING: exported DownloadFile/FetchChecksums/DownloadAndVerifyArchive (methods); unexported newDownloadReq.
  - DO NOT: read os.Getenv; import internal/git|generate|lock|provider|config|cmd; extract archives; sanity-
            run a binary; swap files; call Compare; declare a cobra command/flag/env.

Task 3: CREATE internal/upgrade/download_test.go — pure + httptest matrix (white-box package upgrade)
  - PACKAGE: `package upgrade` (white-box — so currentVersion/Client/Asset/Release/sentinels are reachable;
    matches version_test.go + releases_test.go).
  - HELPERS (mirror releases_test.go's newFakeClient):
      newDownloadServer(t, archive []byte, sum string) (server *httptest.Server, archiveName, csumName string)
        → starts httptest.NewServer; handler routes by r.URL.Path:
            "/dl/<archiveName>" → 200, write archive bytes
            "/dl/<csumName>"    → 200, write "<sum>  <archiveName>\n"
        returns the server (+ t.Cleanup(Close)) + the two filenames for a v1.2.3 release.
      buildRelease(t, server, archiveName, csumName) Release → Release{Tag:"v1.2.3", Assets:[
            {Name:archiveName, DownloadURL: server.URL+"/dl/"+archiveName},
            {Name:csumName,    DownloadURL: server.URL+"/dl/"+csumName}]}
  - PURE CASES (no server):
      * TestSelectAsset (table): darwin/arm64→Asset{Name:"stagecoach_1.2.3_darwin_arm64.tar.gz"};
        windows/amd64→"..._windows_amd64.zip"; linux/amd64→tar.gz; linux/arm64→tar.gz;
        unknown arch→ErrNoMatchingAsset; Tag WITHOUT v ("1.2.3") still resolves (TrimPrefix is a no-op).
      * TestVerifySHA256: write a temp file of known content; compute its sha256 via sha256.Sum256;
        VerifySHA256(path, correctLowerHex)→nil; VerifySHA256(path, wrongHex)→ErrChecksumMismatch;
        VerifySHA256(path, UPPERHexOfCorrect)→nil (normalized); VerifySHA256(nonexistent,...)→error, NOT
        ErrChecksumMismatch (errors.Is must be false).
  - NETWORK CASES (httptest):
      * TestClient_DownloadFile_OK → server serves 4096 random bytes; download to t.TempDir(); assert
        file bytes equal + file size; assert request carried User-Agent "stagecoach/...".
      * TestClient_DownloadFile_500_HTTP → handler 500; errors.Is(err, ErrHTTP).
      * TestClient_DownloadFile_Transport_HTTP → ts.Close() then DownloadFile → errors.Is(err, ErrHTTP).
      * TestClient_FetchChecksums_OK → served "abc...  stagecoach_1.2.3_linux_amd64.tar.gz\n" (+ a 2nd
        line); assert map[name]=sum for both; assert both lines parsed.
      * TestClient_FetchChecksums_NoAsset → release with NO *_checksums.txt asset → ErrNoChecksumsFile.
      * TestClient_FetchChecksums_Malformed → a line with 1 field / non-hex sum → ErrChecksumParse.
      * TestClient_DownloadAndVerifyArchive_OK → full happy path; assert returned path exists + its sha
        equals the served sum.
      * TestClient_DownloadAndVerifyArchive_Tampered → server serves DIFFERENT bytes than the checksums
        line advertises (sum in checksums.txt is the sha of the ORIGINAL; server serves MUTATED bytes) →
        ErrChecksumMismatch AND os.Stat(returnedPath) → not-exist (cleanup removed dest). (Also assert the
        dest filepath.Join(destDir,archiveName) does not exist after the error.)
      * TestClient_DownloadAndVerifyArchive_MissingLine → checksums.txt omits the archive's line (lists a
        different file) → ErrChecksumMissing; dest not created/left.
      * TestClient_DownloadAndVerifyArchive_NoAsset → goos/goarch with no asset → ErrNoMatchingAsset.
  - ASSERTIONS: errors.Is for ALL typed errors; os.Stat/IsNotExist for cleanup checks; bytes.Equal for
    download content; plain == for Asset.Name.
  - DEPENDENCIES: Task 1 + Task 2.

Task 4: VERIFY — build, vet, format, targeted + full upgrade tests, go.mod guard, scope guards
  - gofmt -w internal/upgrade/download.go internal/upgrade/download_test.go
  - go vet ./internal/upgrade/...
  - go build ./...
  - go test ./internal/upgrade/... -run 'SelectAsset|VerifySHA256|Client_Download|Client_FetchChecksums|Client_DownloadAndVerify' -v
  - go test ./internal/upgrade/... -v          # version_test + releases_test (sibling) must stay green
  - go test -race ./internal/upgrade/...       # http.Client is concurrency-safe; cheap insurance
  - go test ./...                              # whole-module no-regression
  - git diff --stat go.mod go.sum              # EMPTY (stdlib only)
  - scope guards (Validation Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN — DownloadFile: stream to disk, never buffer; drain+close the body for keep-alive. Non-2xx and
// transport failures share ErrHTTP (same surface as the metadata layer in releases.go).
func (c *Client) DownloadFile(ctx context.Context, url, dest string) error {
	req, err := c.newDownloadReq(ctx, url)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, ErrHTTP)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %v: %w", url, err, ErrHTTP)
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download %s: status %d: %w", url, resp.StatusCode, ErrHTTP)
	}
	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("download create %s: %w", dest, err)
	}
	_, copyErr := io.Copy(f, resp.Body)   // STREAM — do not io.ReadAll here
	cerr := f.Close()
	if copyErr != nil {
		os.Remove(dest)
		return fmt.Errorf("download %s: %v: %w", url, copyErr, ErrHTTP)
	}
	if cerr != nil {
		os.Remove(dest)
		return fmt.Errorf("download close %s: %w", dest, cerr)
	}
	return nil
}

// PATTERN — VerifySHA256: stream the file through sha256, constant-time-compare the hex digest. I/O
// failures return a plain wrapped error; ONLY the digest comparison yields ErrChecksumMismatch.
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

// PATTERN — DownloadAndVerifyArchive: compose + clean-up-on-failure (FR-U11 staging analog). Every error
// path between download and verify removes the partial dest so no corrupt bytes linger.
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
		os.Remove(dest)
		return "", err
	}
	if err := VerifySHA256(dest, want); err != nil {
		os.Remove(dest)
		return "", err
	}
	return dest, nil
}

// PATTERN — httptest fake server (download_test.go). Routes by path; serves canned archive bytes +
// canned checksums.txt text. Build a Release whose Assets' DownloadURL point at the fake server.
func newDownloadServer(t *testing.T, archive []byte, archiveName, csumName string) (*httptest.Server, string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/dl/"+archiveName, func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
	mux.HandleFunc("/dl/"+csumName, func(w http.ResponseWriter, r *http.Request) {
		sum := sha256hex(archive)
		fmt.Fprintf(w, "%s  %s\n", sum, archiveName)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, ""
}
// TAMPERED variant: wire the checksums handler to a sum computed from DIFFERENT bytes than `archive`.
```

### Integration Points

```yaml
PACKAGE (internal/upgrade): ADDS download.go alongside releases.go + version.go (same package). Exports
  SelectAsset, VerifySHA256, the three *Client methods, and five sentinels. REUSES Client, Release, Asset,
  ErrHTTP (from releases.go) and var currentVersion (from version.go) by bare name. No new package doc.

NETWORK (part of the sole surface): asset GETs hit the ABSOLUTE browser_download_url
  (https://github.com/.../releases/download/<tag>/<file> → 302 → objects.githubusercontent.com). net/http
  follows the redirect. Headers: User-Agent: stagecoach/<ver>; Authorization: Bearer <t>? (if Client.Token
  set). BaseURL is NOT used for asset downloads (metadata-only).

CONFIG/ENV BOUNDARY: NONE in this package. Client.Token (and thus the Bearer header) is a FIELD set by the
  caller (the command layer P1.M4.T1.S1 resolves STAGECOACH_GITHUB_TOKEN/GITHUB_TOKEN). This package calls
  os.Getenv NOWHERE.

NO REGISTRATION: no cobra command, no flag, no env var declared here (all P1.M4). No go.mod change.

CONSUMERS (downstream, not this task): P1.M3.T1.S1 (resolve target release + select GOOS_GOARCH asset —
  may call SelectAsset); P1.M3.T1.S2 (download+verify+extract+sanity-run — calls DownloadAndVerifyArchive);
  P1.M4.T2.S2/S3 (e2e direct-swap + failure-safety tests exercise this through the command).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Go: gofmt + go vet are the gates. Run after creating download.go + download_test.go.
gofmt -w internal/upgrade/download.go internal/upgrade/download_test.go
go vet ./internal/upgrade/...
go build ./...
gofmt -l internal/upgrade/   # Expected: empty (all formatted).
# Expected: zero errors. If gofmt -l lists a file, re-run `gofmt -w`. If vet errors, read + fix.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new matrix, verbose.
go test ./internal/upgrade/... -run 'SelectAsset|VerifySHA256|Client_Download|Client_FetchChecksums|Client_DownloadAndVerify' -v
# Expected: all t.Run subtests PASS:
#   SelectAsset (darwin/arm64→tar.gz, windows/amd64→zip, linux/amd64→tar.gz, linux/arm64→tar.gz,
#                unknown→ErrNoMatchingAsset, tag-without-v resolves)
#   VerifySHA256 (correct→nil, wrong→ErrChecksumMismatch, UPPER→nil normalized, missing-file→error≠mismatch)
#   Client_DownloadFile_OK / _500_HTTP / _Transport_HTTP
#   Client_FetchChecksums_OK / _NoAsset (ErrNoChecksumsFile) / _Malformed (ErrChecksumParse)
#   Client_DownloadAndVerifyArchive_OK / _Tampered (ErrChecksumMismatch + dest removed) /
#     _MissingLine (ErrChecksumMissing) / _NoAsset (ErrNoMatchingAsset)
# All typed-error asserts use errors.Is. Tampered/MissingLine also assert dest does not exist (cleanup).

# The whole upgrade package — version_test + releases_test (sibling) must stay green (this PRP adds files only).
go test ./internal/upgrade/... -v
# Expected: all PASS (no edits to version.go/releases.go → no behavioral regression).
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole module (an additive package file shouldn't ripple, but verify).
go test ./...
# Race detector on the package (http.Client is concurrency-safe; cheap insurance on the fake server).
go test -race ./internal/upgrade/...
# Coverage (no hard gate for internal/upgrade per §20.3, but keep it high).
go test -cover ./internal/upgrade/...
# Expected: all packages PASS; new functions well-covered (all branches: select × match/no-match,
# verify × match/mismatch/io-error, download × 2xx/non-2xx/transport, checksums × parse/no-asset/malformed,
# compose × all the above cleanup paths).
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Scope guard: download.go imports NO internal/* (FR-U12 walled off — no git/generate/lock/provider/config/cmd).
! grep -qE '"github.com/dabstractor/stagecoach/internal/' internal/upgrade/download.go && echo "OK: no internal imports (walled off)"

# Env guard: download.go reads NO env (Token is a Client field; P1.M4 owns env resolution).
! grep -q 'os.Getenv' internal/upgrade/download.go && echo "OK: no os.Getenv"

# Dep guard: no new go.mod require (stdlib only).
git diff --stat go.mod go.sum   # Expected: empty
! grep -q 'require' <(git diff go.mod) && echo "OK: go.mod unchanged"

# Streaming guard: DownloadFile streams (io.Copy to the *file*), never buffers the whole body in memory.
grep -n 'io.Copy(f, resp.Body)\|io.Copy(h, f)' internal/upgrade/download.go   # Expected: ≥1 each (download + verify)

# Constant-time guard: VerifySHA256 uses crypto/subtle (not == / bytes.Equal).
grep -n 'subtle.ConstantTimeCompare' internal/upgrade/download.go   # Expected: ≥1

# Naming-derivation guard: tag-v is stripped before building asset/checksums names.
grep -n 'TrimPrefix(tag, "v")\|TrimPrefix(release.Tag, "v")' internal/upgrade/download.go   # Expected: used in assetName + checksumsName

# Windows-zip guard: windows → .zip, else .tar.gz.
grep -n 'goos == "windows"\|\.zip"\|\.tar.gz"' internal/upgrade/download.go   # Expected: the branch exists

# No-edit guard: releases.go is UNCHANGED by THIS task (parallel sibling owns it).
git diff --stat internal/upgrade/releases.go   # Expected: empty (this PRP adds download.go only)

# Error-contract guard: all five new sentinels exist + are wrapped with %w so errors.Is reaches them.
grep -n 'ErrNoMatchingAsset\|ErrNoChecksumsFile\|ErrChecksumMissing\|ErrChecksumParse\|ErrChecksumMismatch' internal/upgrade/download.go
grep -c '%w' internal/upgrade/download.go   # Expected: ≥5 (each sentinel wrapped at its use site)

# Doc guard (no competing package doc): download.go must NOT start a second "// Package upgrade" block.
! grep -B1 'package upgrade' internal/upgrade/download.go | grep -q 'Package upgrade' && echo "OK: no duplicate package doc"

# FR-U11 cleanup guard: DownloadAndVerifyArchive removes dest on the verify-failure path.
grep -n 'os.Remove(dest)' internal/upgrade/download.go   # Expected: ≥2 (after DownloadFile err + after VerifySHA256 err)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` succeeds
- [ ] `go vet ./internal/upgrade/...` clean
- [ ] `gofmt -l internal/upgrade/` empty
- [ ] `go test ./internal/upgrade/...` green (new matrix + version_test + releases_test regression)
- [ ] `go test -race ./internal/upgrade/...` green
- [ ] `go test ./...` green (no ripple)
- [ ] `go.mod`/`go.sum` unchanged (stdlib only)

### Feature Validation
- [ ] `SelectAsset` strips leading `v` from Tag, appends `.zip` (windows) / `.tar.gz` (else), exact-name matches an Asset or → `ErrNoMatchingAsset`
- [ ] `VerifySHA256` streams sha256, constant-time-compares hex; mismatch → `ErrChecksumMismatch`; I/O error → plain wrapped error (not the sentinel); `want` normalized (trim+lower)
- [ ] `DownloadFile` streams `resp.Body`→file (no whole-body buffer); non-2xx/transport → `ErrHTTP`; User-Agent set; Bearer set iff Token non-empty
- [ ] `FetchChecksums` finds `*_checksums.txt`, parses `<64hex>  <name>` lines via `strings.Fields` + `isHex64`; no asset → `ErrNoChecksumsFile`; bad line → `ErrChecksumParse`
- [ ] `DownloadAndVerifyArchive` composes select→fetch→download→verify; tampered body → `ErrChecksumMismatch` + dest removed; missing line → `ErrChecksumMissing`; no asset → `ErrNoMatchingAsset`
- [ ] All typed errors reachable via `errors.Is(err, ErrNoMatchingAsset|ErrNoChecksumsFile|ErrChecksumMissing|ErrChecksumParse|ErrChecksumMismatch|ErrHTTP)`

### Scope-Boundary Validation
- [ ] download.go imports no `internal/*` (FR-U12 walled off)
- [ ] download.go calls no `os.Getenv` (Token is a Client field; P1.M4 owns env)
- [ ] No extract / sanity-run / binary swap / backup (siblings P1.M3.T1.S2 / P1.M3.T2)
- [ ] No `Compare`-vs-running-version logic (command's `--check` job, P1.M4)
- [ ] No cobra command / flag / env declaration (P1.M4.T1.S1)
- [ ] No edits to `releases.go` (parallel sibling P1.M1.T3.S1 owns it) or `version.go` (P1.M1.T1.S2 owns it)

### Documentation & Quality
- [ ] download.go has a file-level comment (blank-line-separated from `package upgrade` — NOT a competing package doc) citing FR-U5 steps 3–4 / FR-U11 + §19
- [ ] Comments cite §19, §9.29 FR-U5 (steps 3–4), FR-U11, FR-U12 where relevant
- [ ] Follows codebase conventions: sentinel errors mirror `internal/git.ErrCASFailed` + `internal/exitcode.ErrUpdateAvailable`; `fmt.Errorf("…: %w")` wrapping; white-box table-driven tests mirror version_test.go + releases_test.go
- [ ] Unexported helpers (`assetName`, `checksumsName`, `isHex64`, `newDownloadReq`) keep the surface tight; exported names are stagecoach-idiomatic

---

## Anti-Patterns to Avoid

- ❌ **Don't forget to strip the leading `v` from `Release.Tag`.** goreleaser's `{{.Version}}` is the tag WITHOUT `v`, but the GitHub-API `Tag` keeps it. `assetName`/`checksumsName` MUST `strings.TrimPrefix(tag, "v")` or every name is wrong by one char → false `ErrNoMatchingAsset`.
- ❌ **Don't give windows a `.tar.gz`.** `.goreleaser.yaml` `format_overrides` maps `goos: windows → zip`. SelectAsset must branch on `goos == "windows"`.
- ❌ **Don't edit `releases.go`.** It is the parallel sibling P1.M1.T3.S1's deliverable. This subtask ADDS `download.go` to the same package and references `Client`/`Release`/`Asset`/`ErrHTTP` by bare name. Editing the sibling's file risks a merge conflict and violates the additive boundary.
- ❌ **Don't add a second `// Package upgrade` doc.** The package doc lives in version.go. download.go gets a file comment separated from `package upgrade` by a blank line (duplicate-package-doc lint).
- ❌ **Don't buffer the whole archive in memory.** `DownloadFile` must `io.Copy(resp.Body, f)` (streamed). `io.ReadAll` on a multi-MB archive wastes memory and can OOM. `FetchChecksums`'s checksums.txt IS small → buffering there is fine.
- ❌ **Don't invent `ErrDownload`.** Reuse `ErrHTTP` (from releases.go) for transport/HTTP-status failures — it's the same network surface, one sentinel, consistent `errors.Is`. Add ONLY the five domain sentinels (asset/checksums/verify).
- ❌ **Don't prepend `BaseURL` to asset URLs.** `DownloadURL` (browser_download_url) is ABSOLUTE. `BaseURL` is metadata-only (releases.go's `/repos/...` paths). Build the asset GET directly against `DownloadURL`.
- ❌ **Don't use `==` / `bytes.Equal` for the digest compare.** Use `subtle.ConstantTimeCompare` (the item requires it; best practice). Both operands are 64 hex chars so length is constant — no leak — but use subtle anyway.
- ❌ **Don't conflate I/O errors with checksum mismatch.** `VerifySHA256` open/read failures return a plain wrapped error; ONLY the digest comparison yields `ErrChecksumMismatch`, so callers can branch "tampered" from "couldn't read".
- ❌ **Don't leave a partial dest file on failure.** `DownloadAndVerifyArchive` must `os.Remove(dest)` on every error path AFTER it creates dest (FR-U11 staging analog: no corrupt bytes linger for the caller to accidentally extract).
- ❌ **Don't fuzzy-match asset names.** Compute the exact goreleaser name and `==`-match it. Substring/contains matching could pick the wrong asset if names ever overlap; exact match fails loud if the template changes (preferable).
- ❌ **Don't pull a hashing/HTTP helper library.** Stdlib only (`crypto/sha256`, `crypto/subtle`, `encoding/hex`, `net/http`) — matches version.go's "no external dependency" posture. A go.mod require would break the minimal-deps invariant.
- ❌ **Don't import `internal/git`/`generate`/`lock`/`provider`/`config`/`cmd`.** FR-U12: walled off from the commit core. Fetch/verify archive bytes only — no repo, no lock, no provider, no index/ref.
- ❌ **Don't extract / sanity-run / swap here.** That is P1.M3.T1.S2 (extract+sanity) + P1.M3.T2 (backup+swap). This subtask returns a VERIFIED archive PATH; the caller extracts + swaps.
- ❌ **Don't read `os.Getenv`.** `Token` is a `Client` field; the command layer (P1.M4.T1.S1) resolves `STAGECOACH_GITHUB_TOKEN`/`GITHUB_TOKEN`. Env coupling here would make unit tests env-dependent.

---

## Confidence Score

**9/10** — one-pass success likelihood. The contract pins the five function signatures, the goreleaser
filename derivation (tag-strip-v + os/arch identity + windows→zip), the checksums.txt line format
(`<64hex>  <name>`, two-space, parse via `strings.Fields` + `isHex64`), the SHA256 verify recipe
(`sha256.New`+`io.Copy`+`hex.EncodeToString`+`subtle.ConstantTimeCompare`), the download contract
(absolute URL, net/http follows the redirect, stream, drain+close, `ErrHTTP` on non-2xx/transport), the
five new sentinels + the reuse of `ErrHTTP`, and the FR-U11 cleanup-on-failure behavior. The types it
consumes (`Release`/`Asset`/`Client`) are pinned by the sibling PRP (P1.M1.T3.S1), and the codebase's
typed-error + `fmt.Errorf("…: %w")` + white-box-httptest conventions have in-repo precedents
(`internal/git.ErrCASFailed`, `internal/exitcode.ErrUpdateAvailable`, `releases_test.go`). The test
matrix is fully enumerated with canned bytes + checksums.txt and the critical tampered/missing-line/no-asset
branches. The −1 covers two judgment calls the implementer must nail: (1) the tag-vs-`{{.Version}}` strip
(the #1 footgun — fenced with a grep guard + a dedicated test), and (2) reusing releases.go's
unexported `httpClient()` from the same package (callable here; if the sibling renamed it, fall back to a
local `clientOr(c.HTTP, http.DefaultClient)` helper — documented in Task 2). Both are flagged with grep
guards in Validation Level 4.