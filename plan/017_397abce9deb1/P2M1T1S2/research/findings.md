# Research Findings — P2.M1.T1.S2: install.cjs postinstall (download + SHA256-verify + extract to cache)

## §0 Task framing

This is S2 of the npm-wrapper trio (P2.M1.T1). S1 (in flight) creates `npm/package.json`
(`scripts.postinstall = "node install.cjs"`), `npm/bin/stagecoach.js` (the exec shim),
`npm/README.md`, `npm/test/smoke.cjs`. **S2's deliverable is `npm/install.cjs` + the `dependencies`
field in package.json + `npm/test/` fixtures for a local download/verify/extract test.** The
contract's `prd_selectors` (§9.26 work-description mode) is a MISMATCH — authoritative sources are
`architecture/external_deps.md` §1–§3 + §7, the contract, `.goreleaser.yaml`, and the LANDED Go
downloader `internal/upgrade/download.go` (install.cjs is its JS analog).

## §1 The EXACT Go analog (install.cjs must mirror these) — internal/upgrade/download.go

install.cjs is the JS twin of the Go download/verify surface. Mirror these exactly so the npm
cache binary is byte-identical to what the Go self-update downloads:

- **assetName(tag, goos, goarch)** (download.go:57): `ver = TrimPrefix(tag,"v")`;
  `stagecoach_<ver>_<goos>_<goarch>` + `.zip` (windows) / `.tar.gz` (else). e.g.
  `stagecoach_1.0.0_linux_amd64.tar.gz`, `stagecoach_1.0.0_windows_amd64.zip`.
- **checksumsName(tag)** (download.go:74): `stagecoach_<ver-without-v>_checksums.txt`.
- **checksums.txt format**: `<64-hex-sha256>  <artifact-filename>` (two spaces, but parse with
  whitespace-split — require exactly 2 fields + valid 64-hex digest). download.go `FetchChecksums`.
- **VerifySHA256** (download.go:138): stream hash, hex-encode, **constant-time** compare
  (`crypto/subtle.ConstantTimeCompare`). Normalize want (trim + lowercase) before compare.
- **FR-U11 staging analog** (download.go `DownloadAndVerifyArchive`): on ANY failure after creating
  the dest file, `os.Remove(dest)` so a tampered/truncated archive never lingers.

## §2 The cache-path CONTRACT (S1 resolved it; S2 MUST write to the identical path)

From S1's research `findings.md` §3 + the shim in S1's PRP Task 2:
```
cacheRoot       = process.env.STAGECOACH_CACHE_DIR || path.join(os.homedir(), '.stagecoach', 'versions')
cachedBin       = path.join(cacheRoot, version, `${goos}-${goarch}`, binaryName)
goos            = process.platform === 'win32' ? 'windows' : process.platform
goarch          = process.arch === 'x64' ? 'amd64' : process.arch
binaryName      = process.platform === 'win32' ? 'stagecoach.exe' : 'stagecoach'
version         = require('./package.json').version
```
install.cjs must **extract the archive INTO `path.join(cacheRoot, version, `${goos}-${goarch}`)`** so the
binary (shipped at the archive ROOT by goreleaser — `builds.binary: stagecoach`) lands exactly at
`cachedBin`, which is the path S1's shim resolves. `STAGECOACH_CACHE_DIR` is REQUIRED for the local
test (else it pollutes the dev's real `~/.stagecoach`). Home-dir (NOT package-dir) because a global
root-owned install's node_modules is UNWRITABLE for postinstall.

## §3 DECISION: direct download URLs — NO GitHub API call

The contract INPUT lists "GitHub Releases URLs (api + download)". **DECISION: use the DIRECT
download URLs, not the API.** Reason: the unauthenticated GitHub API is rate-limited to **60
req/hour per IP** (external_deps.md §1) — a postinstall that hits the API breaks at scale (CI
behind NAT, corporate networks). The direct download URL has no such limit and needs no auth for a
public repo. This is the esbuild/turbo/prisma pattern. URLs:
```
base           = process.env.STAGECOACH_DOWNLOAD_BASE || 'https://github.com/dabstractor/stagecoach/releases/download'
tag            = 'v' + version                              // npm version "1.0.0" → tag "v1.0.0"
assetName      = `stagecoach_${version}_${goos}_${goarch}${ext}`   // version has NO leading v (matches goreleaser {{.Version}})
checksumsName  = `stagecoach_${version}_checksums.txt`
checksumsURL   = `${base}/${tag}/${checksumsName}`
archiveURL     = `${base}/${tag}/${assetName}`
```
`STAGECOACH_DOWNLOAD_BASE` is the testability seam — the local test points it at
`http://127.0.0.1:PORT/` serving fixtures. (goreleaser's scoop `url_template` confirms the
`releases/download/{tag}/{artifact}` path shape.) `STAGECOACH_GITHUB_TOKEN` (optional) → adds
`Authorization: Bearer <token>` (helps CI behind rate limits; harmless for the local test).

## §4 CRITICAL: GitHub's download URL 302-redirects; Node https does NOT auto-follow

`github.com/.../releases/download/<tag>/<asset>` returns **HTTP 302 →
`objects.githubusercontent.com/...`** (a signed redirect). Node's `https.get`/`http.get` does **NOT**
follow redirects automatically (unlike Go's net/http). install.cjs MUST implement redirect-following
(max ~5 hops, handle 301/302/303/307/308, resolve relative Location against the prior URL). The local
test uses `http://127.0.0.1` (no redirect) so it exercises the happy path; the redirect logic is
exercised against real GitHub only at publish/CI — note this. Helper picks `http` vs `https` by the
URL protocol so the local test (http) and prod (https) share one code path.

## §5 Extraction: `tar` (tar.gz) + `extract-zip` (windows zip) — APIs + versions (verified)

DECISION (contract): two runtime JS deps, no shell-out to system tar/unzip (Windows may lack them).

- **`tar`** (isaacs/node-tar) — **latest 7.5.22**, `engines.node >=18` (== S1's package.json
  `engines.node>=18` floor — perfect). API (from the v7 README, verified):
  ```js
  const tar = require('tar');
  await tar.x({ file: archivePath, cwd: destDir });   // async + file specified → returns a Promise; auto-detects gzip
  // tar.x === tar.extract; 'f' alias for file, 'C' alias for cwd.
  ```
  Safety: by default strips leading `/`, refuses `..` paths and extraction through symlinks
  (maxDepth 1024). `sync:true` is available (returns undefined, throws on error) but the async
  Promise form is cleaner and lets install.cjs stay uniform with extract-zip.
- **`extract-zip`** (maxogden/extract-zip) — **latest 2.0.1**, `engines.node >=10.17.0`. API:
  ```js
  const extract = require('extract-zip');
  await extract(zipPath, { dir: destDir });            // returns a Promise
  ```
Both promise-based → install.cjs uses an async main + try/catch. **package.json `dependencies`** =
`{ "tar": "^7.5.22", "extract-zip": "^2.0.1" }` (S1 created package.json WITHOUT a dependencies
field — verified: S1's PRP Task 1 lists no dependencies; S2 ADDS this field, preserving S1's fields).

## §6 SHA256 + constant-time compare (Node stdlib, verified available)

```js
const crypto = require('node:crypto');
function sha256File(p) { /* readFile or stream; createHash('sha256'); return hex */ }
function timingSafeEqualHex(a, b) {
  const ab = Buffer.from(a, 'hex'), bb = Buffer.from(b, 'hex');   // both 32 bytes from 64 hex
  return ab.length === bb.length && crypto.timingSafeEqual(ab, bb);
}
```
`crypto.timingSafeEqual` + `crypto.createHash` both confirmed present (Node 18+). Mirror Go's
normalization: trim + lowercase the checksums.txt digest before compare.

## §7 Dev-placeholder guard (makes `npm install` in npm/ work during dev)

package.json `version` is `"0.0.0"` (S1's dev placeholder; S3 bumps it at publish). If install.cjs
tried to download `releases/download/v0.0.0/...` it 404s and would break `npm install` during dev.
**Guard**: if `version === '0.0.0' && !process.env.STAGECOACH_DOWNLOAD_BASE` → print a one-line dev
notice and **exit 0** (deps still install; the binary is absent → S1's shim prints the fallback). The
local test sets `STAGECOACH_DOWNLOAD_BASE`, which bypasses the guard so the full logic runs against
the fixture (version 0.0.0 is served by the fixture server). Published packages have a real version
(≠ 0.0.0) so the guard never triggers in the wild.

Alternative considered: require `npm install --ignore-scripts` for dev (clean, dogfoods the
contract) — ALSO recommended for installing the test's deps, but the 0.0.0 guard is belt-and-suspenders
so a plain `npm install` doesn't surface a confusing 404.

## §8 The local test design (contract: "local fixture server + fixture archive in npm/test/")

`npm/test/install.test.cjs` (Node CommonJS; uses the `tar` dep to BUILD the fixture; serves via
`node:http`):

1. Read `version` from `../package.json`; compute `goos`/`goarch`/`assetName`/`checksumsName`
   (SAME map as install.cjs).
2. Build a FIXTURE in a temp dir:
   - fake binary file `stagecoach` (unix) with content `#!/bin/sh\necho fake-stagecoach-$STAGECOACH_INSTALL_METHOD\n`, chmod 0o755. (win32: skip the executable test — see §9.)
   - `await tar.c({ file: fixtureArchive, cwd: tempDir, gzip: true }, ['stagecoach'])` → packs
     `stagecoach_<version>_<goos>_<goarch>.tar.gz` (named EXACTLY as install.cjs's assetName).
   - compute sha256 of the archive; write `stagecoach_<version>_checksums.txt` =
     `<hash>  stagecoach_<version>_<goos>_<goarch>.tar.gz\n`.
3. Start `http.createServer` serving the temp dir (express-style request → read file → 200, else 404).
4. Spawn `node ../install.cjs` with env
   `{ ...process.env, STAGECOACH_DOWNLOAD_BASE: 'http://127.0.0.1:PORT', STAGECOACH_CACHE_DIR: tempCache }`.
5. Assert: child exit 0; `<tempCache>/<version>/<goos>-<goarch>/stagecoach` EXISTS and is executable
   and echoes `fake-stagecoach-npm` when run with `STAGECOACH_INSTALL_METHOD=npm` (proves extract +
   that the shim's env-tag is honored by the binary — optional deep check).
6. FAILURE case: rewrite the fixture archive's checksums.txt with a WRONG hash (or serve a different
   file), re-spawn install.cjs → assert child exit NON-ZERO and `<tempCache>/.../stagecoach` does
   NOT exist (proves the mismatch aborts before write — FR-U11 analog). Also assert stderr contains
   the fallback URL.
7. Server cleanup: `server.close()`.

## §9 Windows / zip path

The host dev machine (linux) exercises the tar.gz branch end-to-end via §8. The zip branch
(windows) is structurally identical (extract-zip is the standard lib) and is covered by **S3/CI on a
windows runner** (P2.M1.T1.S3 owns the cross-platform smoke matrix). S2's local test skips the
executable assertion on win32 (a real PE binary is needed; a .cmd won't spawn without shell:true,
which the shim omits for argv fidelity — same rationale as S1's smoke.cjs win32 skip) but still
asserts the .zip fixture extracts a `stagecoach.exe`. Keep deps minimal: `tar` + `extract-zip` ONLY.

## §10 Scope & validation (Node item — no Go)

- Files S2 creates: `npm/install.cjs`; `npm/test/install.test.cjs`; `npm/test/` fixtures are
  GENERATED at test runtime (not committed binaries) — only the .cjs harness is committed.
- S2 MODIFIES `npm/package.json` to ADD the `dependencies` field (preserving every S1 field).
- Validation: `node --check npm/install.cjs`; `cd npm && npm install --ignore-scripts` (fetches
  tar+extract-zip without running postinstall); `cd npm && node test/install.test.cjs` →
  PASS (download+verify+extract happy path + checksum-mismatch abort). NO go build/test.
- Scope guard: `git status --porcelain` shows ONLY `npm/install.cjs`, `npm/test/install.test.cjs`,
  and the MODIFIED `npm/package.json`. NO Go, no .goreleaser.yaml, no bin/stagecoach.js (S1's),
  no .github/workflows (S3's), no PRD/plan/tasks.