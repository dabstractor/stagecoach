name: "P2.M1.T1.S2 — install.cjs postinstall (platform→asset, download+SHA256-verify, extract to cache)"
description: |
  The second of P2.M1.T1's three subtasks: the npm-wrapper POSTINSTALL DOWNLOADER for
  `@dabstractor/stagecoach` (PRD §21.2 / §9.29 FR-U2; external_deps.md §1–§3). Creates
  `npm/install.cjs` — a Node CommonJS script run by npm's postinstall — that (a) maps
  process.platform/process.arch → the goreleaser asset name + checksums filename for this version;
  (b) GETs the checksums.txt (HTTP, following GitHub's 302 redirect), parses the line for the asset;
  (c) downloads the archive to a temp file, computes SHA256, constant-time-compares — on mismatch
  throws loudly (postinstall fails, never silently half-installs); (d) extracts the single
  stagecoach/stagecoach.exe to the versioned cache dir (the EXACT path S1's shim resolves); (e) on
  ANY error prints a clear message + the direct-install fallback URL and exits non-zero. Also
  MODIFIES npm/package.json to add the two runtime deps (`tar` for tar.gz, `extract-zip` for windows
  zip — S1 created package.json WITHOUT deps). Adds `npm/test/install.test.cjs` — a self-validating
  harness that builds a fixture archive + checksums, serves them via a local http server, and asserts
  install.cjs downloads+verifies+extracts a fake binary into the cache (plus a checksum-mismatch abort
  case). install.cjs is the JS analog of the LANDED Go downloader `internal/upgrade/download.go`
  (P1.M1.T3.S2 Complete) — it mirrors the Go assetName/checksumsName/SHA256-verify/abort-before-write
  logic. NO Go code, NO bin/stagecoach.js (S1's), NO CI wiring (S3's). Validates via `node --check`
  + a local fixture-server test (no go build). NOTE: the task's prd_selectors (§9.26) is a MISMATCH —
  the authoritative spec is external_deps.md §1–§3 + the contract + .goreleaser.yaml + download.go.

---

## Goal

**Feature Goal**: Ship the npm postinstall downloader so that `npm install -g @dabstractor/stagecoach`
fetches the matching prebuilt native binary from GitHub Releases, SHA256-verifies it against the
goreleaser checksums.txt, and extracts it into the versioned cache at the exact path S1's shim looks
for — failing loudly (with the direct-install fallback URL) on any network/checksum/extract error so
an npm install is never silently half-installed.

**Deliverable**: Two new files + one modification under `npm/`:
- `npm/install.cjs` — the postinstall downloader (map platform→asset → fetch checksums → download →
  SHA256-verify constant-time → extract single binary to cache → loud-failure on any error).
- `npm/test/install.test.cjs` — a self-validating harness: builds a fixture archive + checksums,
  serves them from a local `node:http` server, asserts install.cjs downloads+verifies+extracts a fake
  binary into the cache + a checksum-mismatch aborts before write.
- `npm/package.json` — **MODIFIED** to add `"dependencies": { "tar": "^7.5.22", "extract-zip": "^2.0.1" }`
  (S1 created package.json without this field; S2 adds ONLY this field, preserving every S1 field).

**Success Definition**:
- `node --check npm/install.cjs && node --check npm/test/install.test.cjs` exit 0 (syntax clean).
- `cd npm && npm install --ignore-scripts` succeeds and installs `tar` + `extract-zip` (deps fetchable,
  no postinstall run). package.json `dependencies` field present + exact.
- `cd npm && node test/install.test.cjs` PASSes: prints `INSTALL-TEST PASS`, the fake binary exists at
  `<tempCache>/<version>/<goos>-<goarch>/stagecoach` and is executable (the happy path), AND the
  checksum-mismatch sub-case exits non-zero without writing the binary (the abort-before-write case).
- Manual: `STAGECOACH_DOWNLOAD_BASE=<fixture-server> STAGECOACH_CACHE_DIR=<tmp> node npm/install.cjs`
  extracts the fake binary; a tampered fixture makes it print the fallback URL + exit 1.
- `git status --porcelain` shows ONLY `npm/install.cjs`, `npm/test/install.test.cjs`, and the modified
  `npm/package.json` (no Go, no .goreleaser.yaml, no bin/stagecoach.js, no CI, no plan/PRD).

## User Persona (if applicable)

**Target User**: A developer who installs stagecoach via npm (`npm install -g @dabstractor/stagecoach`)
and expects the binary to just work post-install — plus the maintainer running the self-validating
test harness.
**Use Case**: `npm install -g @dabstractor/stagecoach && stagecoach` — npm runs `node install.cjs` as
postinstall, which fetches+verifies+extracts the matching native binary; S1's shim then execs it.
**Pain Points Addressed**: (1) a silently-half-installed npm package (postinstall swallowed an error)
→ install.cjs fails LOUDLY with the direct-install fallback URL; (2) a tampered/truncated archive
landing in the cache → SHA256 gate + abort-before-write; (3) `npm install --ignore-scripts` /
corporate-npm blocking postinstall → the binary is absent and S1's shim prints the fallback (this
subtask honors that contract by NOT masking a missing binary).

## Why

- **PRD §21.2 / external_deps.md §3**: the npm wrapper is the single-package + postinstall-download
  form (the esbuild/turbo/prisma pattern). S1 shipped the PACKAGE + SHIM; S2 is the DOWNLOADER; S3 is
  CI publish + version-sync.
- **Parity with the Go self-update downloader**: install.cjs is the JS twin of the LANDED Go
  `internal/upgrade/download.go` (P1.M1.T3.S2 Complete). It must produce the SAME asset selection,
  checksum verification, and abort-before-write discipline so the npm-cached binary is identical to
  what `stagecoach upgrade` would fetch — and so the cache path S1's shim resolves is populated.
- **Scope discipline**: S2 ships ONLY install.cjs + its deps + its local test. No Go code, no shim
  (S1's), no `.github/workflows` (S3's). The postinstall downloader is reviewable in isolation.

## What

`npm/install.cjs` (Node CJS, async main, deps: `tar` + `extract-zip`) plus the `dependencies` field in
package.json and a self-validating local test. No Go code. No shim (S1 owns `bin/stagecoach.js`). No CI.

### Success Criteria
- [ ] `npm/install.cjs` maps platform/arch to the goreleaser asset name + checksums name, matching
      `internal/upgrade/download.go` `assetName`/`checksumsName` EXACTLY (version without leading "v";
      `.zip` windows / `.tar.gz` else).
- [ ] Fetches `checksums.txt` over HTTP **following the GitHub 302 redirect** (Node https does NOT
      auto-follow), parses `<64hex>  <filename>` lines (whitespace-split, exactly 2 fields + valid
      64-hex), and looks up the asset's expected SHA256.
- [ ] Downloads the archive to a temp file and **SHA256-verifies it constant-time** (`crypto.timingSafeEqual`)
      against the checksums line; on mismatch **removes the partial file and throws** (abort-before-write).
- [ ] Extracts the single `stagecoach`/`stagecoach.exe` to
      `<STAGECOACH_CACHE_DIR||~/.stagecoach/versions>/<version>/<goos>-<goarch>/` (the EXACT path S1's
      shim resolves) — `tar.x` for tar.gz, `extract-zip` for zip.
- [ ] On ANY error, prints a clear message + the direct-install fallback URL
      (`https://github.com/dabstractor/stagecoach#install`) to stderr and `process.exit(1)` (loud failure).
- [ ] Unsupported platform (`aix`/`sunos`/etc., i.e. not `linux`/`darwin`/`win32`) → loud failure with
      the fallback URL (the goreleaser matrix only builds those 3 × amd64/arm64).
- [ ] Dev-placeholder guard: `version === '0.0.0' && !STAGECOACH_DOWNLOAD_BASE` → print a dev notice +
      `exit 0` (so `npm install` during dev doesn't 404 on the nonexistent v0.0.0 release).
- [ ] Honors optional `STAGECOACH_GITHUB_TOKEN` (adds `Authorization: Bearer <t>`) and
      `STAGECOACH_DOWNLOAD_BASE` (the testability seam; default `https://github.com/dabstractor/stagecoach/releases/download`).
- [ ] `npm/package.json` gains `"dependencies": { "tar": "^7.5.22", "extract-zip": "^2.0.1" }` (S1's fields preserved).
- [ ] `npm/test/install.test.cjs` PASSes (happy path extract + checksum-mismatch abort).
- [ ] `node --check` both .cjs files; scope guard clean (only `npm/install.cjs`, `npm/test/install.test.cjs`, modified `npm/package.json`).

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim asset-name/checksums-name algorithms (from download.go), the EXACT cache-path
scheme S1's shim resolves (must match), the direct-URL decision (avoiding the 60-req/h API rate
limit), the mandatory redirect-following gotcha, the verified `tar`/`extract-zip` APIs + versions,
the constant-time SHA256 compare, the dev-placeholder guard, the abort-before-write discipline, the
local fixture-server test algorithm, and the S1/S2/S3 scope boundary (shim is S1's; CI is S3's).

### Documentation & References

```yaml
# MUST READ — the codebase-specific findings for THIS item.
- docfile: plan/017_397abce9deb1/P2M1T1S2/research/findings.md
  why: "§1 the Go analog to mirror (assetName/checksumsName/VerifySHA256/abort-before-write); §2 the
        EXACT cache-path CONTRACT (must match S1's shim); §3 the direct-URL decision (no API → no rate
        limit); §4 the GitHub-302 redirect gotcha; §5 tar/extract-zip APIs + versions; §6 SHA256 +
        crypto.timingSafeEqual; §7 the 0.0.0 dev guard; §8 the local test algorithm; §9 windows/zip."

# MUST READ — S1's PRP is a CONTRACT: it defines the cache path + platform map install.cjs must match.
- docfile: plan/017_397abce9deb1/P2M1T1S1/PRP.md
  why: "Task 2 (bin/stagecoach.js) + findings §2/§3 define cachedBin = STAGECOACH_CACHE_DIR||
        ~/.stagecoach/versions/<version>/<goos>-<goarch>/<binary> with goos/goarch/binaryName from the
        Node→goreleaser map. install.cjs MUST extract into <...>/<goos>-<goarch>/ so the binary lands
        exactly there. S1's package.json declares scripts.postinstall='node install.cjs' and has NO
        dependencies field — S2 ADDS it."

# MUST READ — the Go downloader install.cjs mirrors (asset name, checksums parse, SHA256, abort-before-write).
- file: internal/upgrade/download.go
  why: "assetName (:57), checksumsName (:74), isHex64, FetchChecksums (parse '<64hex>  <filename>'),
        VerifySHA256 (constant-time compare), DownloadAndVerifyArchive (on any post-download failure,
        os.Remove the partial file). Mirror these so the npm binary == the Go-self-update binary."
  pattern: "assetName: 'stagecoach_'+TrimPrefix(tag,'v')+'_'+goos+'_'+goarch + '.zip'(windows)/'.tar.gz'(else)."

# MUST READ — the authoritative wrapper + checksums format spec.
- docfile: plan/017_397abce9deb1/architecture/external_deps.md
  section: "§1 GitHub Releases API + the 302 redirect download URL (releases/download/{tag}/{artifact});
            §2 checksums.txt format '<64hex>  <filename>'; §3 npm wrapper (postinstall-download form);
            §7 STAGECOACH_INSTALL_METHOD detection (the env the shim sets)."
  critical: "§1 notes the 60-req/h unauthenticated API rate limit — hence the DIRECT-URL decision (no API call)."

# MUST READ — goreleaser asset names (the JS map MUST produce these EXACT strings).
- file: .goreleaser.yaml
  why: "archives.name_template='stagecoach_<version>_<os>_<arch>'; formats=[tar.gz], windows→zip;
        checksum.name_template='stagecoach_<version>_checksums.txt'; builds.binary=stagecoach;
        release.github={owner:dabstractor, name:stagecoach}. 6 targets: linux|darwin|windows × amd64|arm64."

# VERIFIED — the extraction dep APIs (from the package READMEs; versions checked via npm view).
- url: https://github.com/isaacs/node-tar/blob/master/README.md
  why: "tar.x({ file, cwd }) → Promise; auto-detects gzip; tar.x === tar.extract; 'f'/'C' aliases.
        v7.5.22, engines.node>=18 (matches package.json engines.node>=18)."
- url: https://github.com/maxogden/extract-zip
  why: "await extract(source, { dir: target }) → Promise. v2.0.1, engines.node>=10.17.0."

# OUT OF SCOPE — the shim (S1's) and CI publish (S3's). Do NOT create/edit either.
- file: (npm/bin/stagecoach.js — S1's deliverable; EXISTS after S1 lands)
  why: "S1 owns the exec shim. install.cjs must populate the cache path S1's shim resolves, but must NOT
        touch bin/stagecoach.js."
- out_of_scope: ".github/workflows/* (S3 — P2.M1.T1.S3 owns `npm publish` on tag + version-sync + the
                 cross-platform smoke matrix, incl. the windows zip path on a windows runner)."
```

### Current Codebase tree (relevant slice)

```bash
# READ-ONLY references (do NOT edit):
.goreleaser.yaml                          # asset-name templates (stagecoach_<v>_<os>_<arch>.tar.gz/.zip)
internal/upgrade/download.go              # the Go analog: assetName/checksumsName/VerifySHA256/FetchChecksums
plan/017_397abce9deb1/architecture/external_deps.md   # §1–§3 + §7 (wrapper spec, checksums format, detection)
plan/017_397abce9deb1/P2M1T1S1/PRP.md     # CONTRACT: cache path + platform map + package.json (S1)
# After S1 lands (ASSUME PRESENT — treat as the starting state):
npm/package.json                          # S1 created it: name, version=0.0.0, bin, scripts.postinstall='node install.cjs', engines.node>=18, NO dependencies
npm/bin/stagecoach.js                     # S1's shim (resolves cachedBin; install.cjs must populate that path)
npm/README.md  npm/test/smoke.cjs         # S1's (do not touch)
```

### Desired Codebase tree with files to be added/modified

```bash
npm/
  package.json             # MODIFIED — ADD "dependencies": { "tar": "^7.5.22", "extract-zip": "^2.0.1" } (S1's fields preserved)
  install.cjs              # NEW — the postinstall downloader (map→fetch checksums→download→SHA256→extract→loud-fail)
  bin/stagecoach.js        # (S1's — unchanged)
  README.md  test/smoke.cjs# (S1's — unchanged)
  test/
    install.test.cjs       # NEW — self-validating harness: fixture archive + checksums + local http server
# Fixtures are GENERATED at test runtime (not committed binaries) — only install.test.cjs is committed.
# NOT created by S2: npm/bin/stagecoach.js (S1), .github/workflows npm-publish (S3).
```

### Known Gotchas of our codebase & Library Quirks

```javascript
// CRITICAL (GitHub download URL 302-redirects; Node https does NOT auto-follow):
//   https://github.com/.../releases/download/<tag>/<asset>  →  HTTP 302  →  objects.githubusercontent.com/...
//   Node's https.get/http.get does NOT follow redirects (unlike Go net/http). install.cjs MUST implement
//   redirect-following (max ~5 hops; 301/302/303/307/308; resolve a relative Location against the prior URL).
//   The helper must pick http vs https by URL protocol so the local test (http://127.0.0.1) and prod (https)
//   share one code path.

// CRITICAL (no GitHub API call — use the DIRECT download URL):
//   The unauthenticated GitHub API is rate-limited to 60 req/h/IP (external_deps §1). A postinstall that
//   hits the API breaks at scale. Use releases/download/<tag>/<asset> directly (no auth needed for a public
//   repo; no rate limit). This is the esbuild/turbo pattern. URL = base + '/' + tag + '/' + assetName, where
//   base = STAGECOACH_DOWNLOAD_BASE || 'https://github.com/dabstractor/stagecoach/releases/download'.

// CRITICAL (version → tag mapping): npm version field is the goreleaser {{.Version}} = tag WITHOUT leading "v"
//   (tag v1.0.0 → version "1.0.0"). So: tag = 'v' + version; assetName uses version (no v):
//   `stagecoach_${version}_${goos}_${goarch}${ext}`. checksumsName = `stagecoach_${version}_checksums.txt`.
//   Mirrors download.go assetName (TrimPrefix(tag,"v")). Do NOT put the "v" in the asset name.

// CRITICAL (cache path MUST match S1's shim EXACTLY): extract INTO
//   path.join(STAGECOACH_CACHE_DIR || path.join(os.homedir(),'.stagecoach','versions'), version, `${goos}-${goarch}`)
//   so the binary (shipped at the archive ROOT by goreleaser) lands at <...>/<goos>-<goarch>/stagecoach[.exe],
//   which is the EXACT path bin/stagecoach.js resolves. goos = platform==='win32'?'windows':platform;
//   goarch = arch==='x64'?'amd64':arch; binaryName = platform==='win32'?'stagecoach.exe':'stagecoach'.

// CRITICAL (abort-before-write — FR-U11 staging analog): on ANY failure after creating the temp archive
//   file (checksum mismatch, non-2xx, copy error), fs.rmSync it so a tampered/truncated archive never lingers.
//   Mirror download.go DownloadAndVerifyArchive's os.Remove on failure.

// CRITICAL (constant-time SHA256 compare): use crypto.timingSafeEqual on the raw digest BYTES (32 bytes from
//   64 hex), guarding equal length first (timingSafeEqual throws on length mismatch). Normalize the checksums
//   digest (trim + lowercase) before compare — mirror download.go VerifySHA256.

// GOTCHA (dev-placeholder guard): package.json version is "0.0.0" until S3 bumps it at publish. If install.cjs
//   tried to download v0.0.0 it 404s and breaks `npm install` during dev. Guard: if version==='0.0.0' &&
//   !process.env.STAGECOACH_DOWNLOAD_BASE → print a dev notice + exit 0. The local test sets STAGECOACH_DOWNLOAD_BASE
//   (bypassing the guard) so the full logic runs against the fixture. Published packages have a real version.

// GOTCHA (extraction is async/promise-based): tar.x({file,cwd}) and extract-zip(source,{dir}) both return
//   Promises. Use an async main + try/catch → on error console.error(message+fallback URL) + process.exit(1).
//   Do NOT use sync shell-out to system tar/unzip (Windows may lack them; the contract mandates JS deps).

// GOTCHA (npm install during dev runs postinstall): to fetch the test's deps (tar, extract-zip) WITHOUT
//   triggering the real download, use `npm install --ignore-scripts`. The 0.0.0 guard (above) also makes a
//   plain `npm install` safe (it skips + exits 0).

// GOTCHA (the fallback URL #install anchor): the loud-failure message ends with
//   https://github.com/dabstractor/stagecoach#install — the same anchor S1's shim fallback message uses.
//   Keep them identical for a consistent user experience.

// GOTCHA (deps are S2's — S1's package.json has NONE): S2 ADDS the dependencies field to package.json,
//   preserving every field S1 set (name, version, bin, scripts, engines, license, repository, ...).
//   Do NOT add "type":"module" (the .cjs extension forces CJS regardless, but keep package.json commonjs).
```

## Implementation Blueprint

### Data models and structure

No data models — this is a Node script. The "data" is:
- the platform→asset map (must match download.go + S1's shim);
- the cache-path scheme (must match S1's shim);
- the URL scheme (`base/tag/asset`);
- the deps set (`tar` + `extract-zip`).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY npm/package.json — ADD the dependencies field (S1's fields preserved)
  - ADD: "dependencies": { "tar": "^7.5.22", "extract-zip": "^2.0.1" }
    (tar 7.5.22 engines.node>=18 == package.json engines.node>=18; extract-zip 2.0.1.)
  - PRESERVE every S1 field (name, version, description, bin, scripts.postinstall='node install.cjs',
    engines.node>=18, license, repository, homepage, bugs, keywords). Do NOT add "type"/"os"/"cpu".
  - VALIDATE: node -e "const p=require('./npm/package.json'); if(!p.dependencies||p.dependencies.tar!=='^7.5.22'||p.dependencies['extract-zip']!=='^2.0.1')process.exit(1); if(p.scripts.postinstall!=='node install.cjs')process.exit(1); if(p.engines.node!=='>=18')process.exit(1)" exits 0.

Task 2: CREATE npm/install.cjs — the postinstall downloader
  - LINE 1: `#!/usr/bin/env node` ; then `'use strict';`
  - IMPORTS: node:https, node:http, node:crypto, node:fs, node:os, node:path, node:url (URL). Deps: tar, extract-zip.
  - CONSTANTS/MAP (must match download.go + S1's shim):
      const pkg = require('./package.json');
      const version = pkg.version;                                   // e.g. "1.0.0" (no leading v)
      const goos = process.platform === 'win32' ? 'windows' : process.platform;
      const goarch = process.arch === 'x64' ? 'amd64' : process.arch;
      const binaryName = process.platform === 'win32' ? 'stagecoach.exe' : 'stagecoach';
      const ext = process.platform === 'win32' ? '.zip' : '.tar.gz';
      const assetName = `stagecoach_${version}_${goos}_${goarch}${ext}`;
      const checksumsName = `stagecoach_${version}_checksums.txt`;
      const SUPPORTED = new Set(['linux','darwin','win32']);         // the goreleaser matrix
      const FALLBACK_URL = 'https://github.com/dabstractor/stagecoach#install';
  - BODY (async main):
      const tag = 'v' + version;
      const base = process.env.STAGECOACH_DOWNLOAD_BASE || 'https://github.com/dabstractor/stagecoach/releases/download';
      const headers = { 'User-Agent': `stagecoach-npm/${version}` };
      if (process.env.STAGECOACH_GITHUB_TOKEN) headers.Authorization = 'Bearer ' + process.env.STAGECOACH_GITHUB_TOKEN;
      async function main() {
        // (0) unsupported platform → loud failure.
        if (!SUPPORTED.has(process.platform)) throw new Error(`unsupported platform '${process.platform}' (stagecoach ships linux/darwin/windows x amd64/arm64). Install directly: ${FALLBACK_URL}`);
        // (dev guard) version 0.0.0 + no override → skip (dev placeholder, never published).
        if (version === '0.0.0' && !process.env.STAGECOACH_DOWNLOAD_BASE) {
          console.log('stagecoach: dev placeholder version 0.0.0 — skipping binary download (publish sets a real version).');
          return;
        }
        // (1) cache dir (must match S1's shim resolution).
        const cacheRoot = process.env.STAGECOACH_CACHE_DIR || path.join(os.homedir(), '.stagecoach', 'versions');
        const destDir = path.join(cacheRoot, version, `${goos}-${goarch}`);
        // (2) fetch checksums.txt (follow redirects), parse the asset's SHA256.
        const sumsText = await fetchText(`${base}/${tag}/${checksumsName}`, headers);
        const expected = parseChecksum(sumsText, assetName);   // throws if missing/malformed
        // (3) download the archive to a temp file.
        const tmpArchive = path.join(os.tmpdir(), `${assetName}.part`);
        await downloadFile(`${base}/${tag}/${assetName}`, headers, tmpArchive);
        // (4) SHA256-verify constant-time; on mismatch remove the temp file + throw (abort-before-write).
        const got = await sha256File(tmpArchive);
        if (!timingSafeEqualHex(got, expected)) { fs.rmSync(tmpArchive, {force:true}); throw new Error(`SHA256 mismatch for ${assetName}: got ${got} want ${expected}`); }
        // (5) extract the single binary into destDir (mkdir -p first). tar.gz → tar.x; zip → extract-zip.
        fs.mkdirSync(destDir, { recursive: true });
        if (process.platform === 'win32') await extractZip(tmpArchive, { dir: destDir });
        else await tar.x({ file: tmpArchive, cwd: destDir });
        fs.rmSync(tmpArchive, { force: true });
        // (6) confirm the binary landed where S1's shim expects (defense-in-depth).
        const binPath = path.join(destDir, binaryName);
        if (!fs.existsSync(binPath)) throw new Error(`extraction did not produce ${binPath}`);
        console.log(`stagecoach: installed ${version} (${goos}-${goarch}) to ${binPath}`);
      }
      (async () => { try { await main(); } catch (e) {
        process.stderr.write('stagecoach postinstall failed: ' + (e && e.message ? e.message : e) + '\n' +
                             'Install directly: ' + FALLBACK_URL + '\n');
        process.exit(1);
      } })();
  - HELPERS (top-level functions):
      httpRequest(url, headers) → Promise<http.IncomingMessage> using https/http by protocol (URL.protocol),
        with a 60s timeout (req.on('timeout') → req.destroy), surfacing req.on('error').
      fetchFollowingRedirects(url, headers, hops=5) → returns the FINAL 2xx IncomingMessage stream; on 3xx +
        Location resolve relative via `new URL(location, url).href` and recurse (dec hops); on non-2xx throw.
      fetchText(url, headers) → collect the final stream into a string (Buffer.concat chunks).
      downloadFile(url, headers, dest) → pipe the final stream into fs.createWriteStream(dest); resolve on
        finish; on any error remove dest + reject (abort-before-write).
      sha256File(p) → fs.createReadStream(p) piped through crypto.createHash('sha256'); return hex digest.
        (Or fs.readFileSync for small files — archives are multi-MB, prefer streaming.)
      timingSafeEqualHex(a, b) → const ab=Buffer.from(a,'hex'), bb=Buffer.from(b,'hex'); return
        ab.length===bb.length && crypto.timingSafeEqual(ab, bb). (Both 32 bytes from 64-hex; normalize b
        to trim+lowercase first, mirroring download.go VerifySHA256.)
      parseChecksum(text, name) → split lines; for each non-empty line split on /\s+/; require exactly 2
        fields AND /^[0-9a-f]{64}$/i.test(fields[0]); build map; return map[name] or throw if name absent
        or a line is malformed. (Mirrors download.go FetchChecksums.)
  - NAMING/PLACEMENT: npm/install.cjs. package.json scripts.postinstall already runs `node install.cjs` (S1).
  - VALIDATE: `node --check npm/install.cjs` exit 0.

Task 3: CREATE npm/test/install.test.cjs — the self-validating fixture-server harness
  - STRUCTURE (Node CommonJS; uses the `tar` dep to BUILD the fixture + node:http to serve it):
      const http = require('node:http'); const crypto = require('node:crypto'); const fs = require('node:fs');
      const os = require('node:os'); const path = require('node:path'); const { spawnSync } = require('node:child_process');
      const tar = require('tar');    // dep — available after `npm install --ignore-scripts` in npm/
      const pkg = require('../package.json');
      function assert(c,m){ if(!c){ console.error('INSTALL-TEST FAIL: '+m); process.exit(1); } }
      (async () => {
        const version = pkg.version;
        const goos = process.platform === 'win32' ? 'windows' : process.platform;
        const goarch = process.arch === 'x64' ? 'amd64' : process.arch;
        const ext = process.platform === 'win32' ? '.zip' : '.tar.gz';
        const assetName = `stagecoach_${version}_${goos}_${goarch}${ext}`;
        const checksumsName = `stagecoach_${version}_checksums.txt`;
        // (1) build fixture: fake binary → tar.gz (zip on win32 — see notes)
        const serveDir = fs.mkdtempSync(path.join(os.tmpdir(),'sc-srv-'));
        const fakeBinName = process.platform === 'win32' ? 'stagecoach.exe' : 'stagecoach';
        const fakeContent = process.platform === 'win32' ? 'fake' : '#!/bin/sh\necho fake-stagecoach-$STAGECOACH_INSTALL_METHOD\n';
        fs.writeFileSync(path.join(serveDir, fakeBinName), fakeContent);
        if (process.platform !== 'win32') fs.chmodSync(path.join(serveDir, fakeBinName), 0o755);
        const archivePath = path.join(serveDir, assetName);
        await tar.c({ file: archivePath, cwd: serveDir, gzip: true }, [fakeBinName]);
        const hash = crypto.createHash('sha256').update(fs.readFileSync(archivePath)).digest('hex');
        fs.writeFileSync(path.join(serveDir, checksumsName), `${hash}  ${assetName}\n`);
        // (2) local http server
        const server = http.createServer((req,res)=>{ const f=path.join(serveDir, decodeURIComponent(req.url.replace(/^\//,'')));
          if (req.method!=='GET'||!fs.existsSync(f)||fs.statSync(f).isDirectory()){ res.statusCode=404; return res.end('404'); }
          res.statusCode=200; fs.createReadStream(f).pipe(res); });
        await new Promise(r=>server.listen(0,'127.0.0.1',r));
        const base = `http://127.0.0.1:${server.address().port}`;
        // (3) HAPPY PATH: run install.cjs against the fixture
        const cache = fs.mkdtempSync(path.join(os.tmpdir(),'sc-cache-'));
        let r = spawnSync(process.execPath, [path.join(__dirname,'..','install.cjs')],
              { env:{ ...process.env, STAGECOACH_DOWNLOAD_BASE:base, STAGECOACH_CACHE_DIR:cache }, encoding:'utf8' });
        assert(r.status===0, 'happy path exit '+r.status+' stderr='+r.stderr);
        const installed = path.join(cache, version, `${goos}-${goarch}`, fakeBinName);
        assert(fs.existsSync(installed), 'binary not extracted to '+installed);
        if (process.platform!=='win32'){ assert(fs.statSync(installed).mode & 0o111, 'binary not executable'); }
        // (4) CHECKSUM-MISMATCH ABORT: corrupt the checksums → install.cjs must exit non-zero + NOT leave a binary
        const cache2 = fs.mkdtempSync(path.join(os.tmpdir(),'sc-cache2-'));
        fs.writeFileSync(path.join(serveDir, checksumsName), `${'0'.repeat(64)}  ${assetName}\n`);
        let r2 = spawnSync(process.execPath, [path.join(__dirname,'..','install.cjs')],
              { env:{ ...process.env, STAGECOACH_DOWNLOAD_BASE:base, STAGECOACH_CACHE_DIR:cache2 }, encoding:'utf8' });
        assert(r2.status!==0, 'mismatch should fail loudly, got exit '+r2.status);
        assert(/mismatch|postinstall failed/i.test(r2.stderr), 'stderr should report the mismatch');
        assert(/#install/.test(r2.stderr), 'stderr should include the fallback URL');
        assert(!fs.existsSync(path.join(cache2, version, `${goos}-${goarch}`, fakeBinName)), 'mismatch must NOT leave a binary');
        server.close(); console.log('INSTALL-TEST PASS');
      })().catch(e=>{ console.error('INSTALL-TEST ERROR', e); process.exit(1); });
  - NOTE: the win32 branch builds a `.zip` fixture for the production code path; S2's local test on a
    linux/darwin host covers the tar.gz path end-to-end. The zip path is structurally identical (extract-zip
    is the standard lib) and is exercised on a windows runner by S3/CI. Keep deps minimal (tar + extract-zip).
  - NAMING/PLACEMENT: npm/test/install.test.cjs. Run with `node test/install.test.cjs` from npm/.
  - VALIDATE: `node --check npm/test/install.test.cjs`; after `npm install --ignore-scripts` in npm/,
    `node test/install.test.cjs` → prints `INSTALL-TEST PASS`, exit 0 (happy path + mismatch abort).

Task 4: VALIDATE — syntax, deps install, the fixture test, scope
  - node --check npm/install.cjs && node --check npm/test/install.test.cjs
  - cd npm && npm install --ignore-scripts          # fetches tar + extract-zip WITHOUT running postinstall
  - cd npm && node test/install.test.cjs            # INSTALL-TEST PASS (happy + mismatch)
  - node -e "field guard on npm/package.json dependencies + S1 fields" (Task 1 validate)
  - git status --porcelain                          # ONLY npm/install.cjs, npm/test/install.test.cjs, modified npm/package.json
```

### Implementation Patterns & Key Details

```javascript
// PATTERN (redirect-following HTTP GET — GitHub's download URL 302s; Node does NOT auto-follow):
const https = require('node:https'), http = require('node:http'), { URL } = require('node:url');
function httpRequest(url, headers) {
  return new Promise((resolve, reject) => {
    const U = new URL(url);
    const lib = U.protocol === 'https:' ? https : http;     // prod=https; local test=http
    const req = lib.get(url, { headers, timeout: 60000 }, resolve);
    req.on('error', reject);
    req.on('timeout', () => req.destroy(new Error('request timeout')));
  });
}
async function fetchFollowingRedirects(url, headers, hops = 5) {
  const res = await httpRequest(url, headers);
  if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
    if (hops <= 0) throw new Error('too many redirects');
    res.resume();                                            // drain before redirect
    return fetchFollowingRedirects(new URL(res.headers.location, url).href, headers, hops - 1);
  }
  if (res.statusCode < 200 || res.statusCode >= 300) { res.resume(); throw new Error(`HTTP ${res.statusCode} for ${url}`); }
  return res;                                                // final 2xx stream
}

// PATTERN (stream download with abort-before-write on error):
async function downloadFile(url, headers, dest) {
  const res = await fetchFollowingRedirects(url, headers);
  await new Promise((resolve, reject) => {
    const ws = fs.createWriteStream(dest);
    res.on('error', (e) => { ws.close(() => { fs.rmSync(dest, {force:true}); reject(e); }); });
    ws.on('error', (e) => { fs.rmSync(dest, {force:true}); reject(e); });
    ws.on('finish', resolve);
    res.pipe(ws);
  });
}

// PATTERN (constant-time SHA256 — mirrors download.go VerifySHA256):
async function sha256File(p) {
  return new Promise((resolve, reject) => {
    const h = crypto.createHash('sha256');
    fs.createReadStream(p).on('data', d => h.update(d)).on('end', () => resolve(h.digest('hex'))).on('error', reject);
  });
}
function timingSafeEqualHex(a, b) {
  const want = Buffer.from(String(b).trim().toLowerCase(), 'hex');   // normalize (download.go does the same)
  const got = Buffer.from(a, 'hex');
  return got.length === want.length && crypto.timingSafeEqual(got, want);
}
// CRITICAL: timingSafeEqual throws on length mismatch — guard with got.length===want.length first.
```

### Integration Points

```yaml
CONSUMES (the cache-path CONTRACT — from S1's shim, READ-ONLY):
  - npm/bin/stagecoach.js (S1): resolves cachedBin = STAGECOACH_CACHE_DIR||~/.stagecoach/versions/<version>/<goos>-<goarch>/<binary>.
    install.cjs MUST extract the archive INTO <...>/<goos>-<goarch>/ so the binary lands there.
  - npm/package.json (S1): scripts.postinstall='node install.cjs' RUNS install.cjs on npm install; version field
    (0.0.0 dev / real at publish) drives the asset name + tag + cache path.
PRODUCES (consumed by sibling tasks):
  - The populated cache (<version>/<goos>-<goarch>/stagecoach[.exe]) — S1's shim execs it; S3/CI's smoke
    matrix (incl. windows zip) asserts the full npm install works cross-platform.
NO Go/build/config/migration changes — npm/ is a self-contained Node subpackage (.gitignore already ignores
  /node_modules/, so the dev node_modules won't pollute git).
```

## Validation Loop

> **This is a Node item, not a Go/Python one.** The template's `ruff`/`mypy`/`pytest`/`go test` gates DO NOT
> apply. Use `node --check`, `node` direct-run, and the fixture-server test. No `go build` (S2 adds no Go code).
> Run commands from the repo root; `cd npm` where noted.

### Level 1: Syntax & manifest validity (Immediate Feedback)

```bash
# Syntax-check both .cjs files (catches parse errors / typos before runtime).
node --check npm/install.cjs && echo "install.cjs OK"
node --check npm/test/install.test.cjs && echo "install.test.cjs OK"
# Expected: "install.cjs OK" + "install.test.cjs OK", exit 0 each.

# package.json has the dependencies field AND S1's fields are preserved.
node -e "const p=require('./npm/package.json'); if(!p.dependencies)process.exit(1); if(p.dependencies.tar!=='^7.5.22')process.exit(1); if(p.dependencies['extract-zip']!=='^2.0.1')process.exit(1); if(p.scripts.postinstall!=='node install.cjs')process.exit(1); if(p.engines.node!=='>=18')process.exit(1); if(p.bin.stagecoach!=='./bin/stagecoach.js')process.exit(1); console.log('package.json deps + S1 fields OK')"
# Expected: "package.json deps + S1 fields OK", exit 0.
```

### Level 2: Deps install + the fixture-server test (Component Validation)

```bash
# Fetch the runtime deps (tar + extract-zip) WITHOUT running postinstall (no real download attempted).
cd npm && npm install --ignore-scripts && cd ..
# Expected: adds npm/node_modules/{tar,extract-zip,...}; exit 0. (The 0.0.0 guard would also make plain
# `npm install` safe, but --ignore-scripts is the explicit dev path.)

# The self-validating harness: builds a fixture archive + checksums, serves via local http, runs install.cjs.
cd npm && node test/install.test.cjs && cd ..
# Expected: prints "INSTALL-TEST PASS", exit 0. Covers the happy path (extract + executable) AND the
# checksum-mismatch abort (exit non-zero + no binary written + fallback URL in stderr).
```

### Level 3: End-to-end manual (System Validation)

```bash
# Manual happy path against the fixture server (proves download→verify→extract outside the test harness).
# Reuse the test's fixture by running install.cjs directly with STAGECOACH_DOWNLOAD_BASE pointed at a
# fixture dir served ad hoc — OR, simplest, trust Level 2's harness (it IS this end-to-end test).
# Direct check the cache layout matches S1's shim:
CACHE="$(mktemp -d)"; node npm/test/install.test.cjs >/dev/null 2>&1   # (the harness sets its own cache)
# Then verify the path S1's shim resolves would be populated: <CACHE>/<version>/<goos>-<goarch>/stagecoach
node -e "const p=require('./npm/package.json'); const goos=process.platform==='win32'?'windows':process.platform; const goarch=process.arch==='x64'?'amd64':process.arch; console.log('shim resolves: <CACHE>/'+p.version+'/'+goos+'-'+goarch+'/'+(process.platform==='win32'?'stagecoach.exe':'stagecoach'))"
# Expected: the printed path matches where install.cjs extracted the binary in Level 2 (sanity: same map).

# Manual loud-failure: point install.cjs at a base that 404s → it must print the fallback URL + exit 1.
STAGECOACH_DOWNLOAD_BASE="http://127.0.0.1:1" STAGECOACH_CACHE_DIR="$(mktemp -d)" node npm/install.cjs; echo "exit=$?"
# Expected: exit=1; stderr contains "postinstall failed" + "#install".
```

### Level 4: Scope guard

```bash
# ONLY npm/install.cjs (new), npm/test/install.test.cjs (new), and the MODIFIED npm/package.json.
git status --porcelain
# Expected: ?? npm/install.cjs   ?? npm/test/install.test.cjs   M npm/package.json   (nothing else).
git status --porcelain | grep -vE '^\?\? npm/(install\.cjs|test/install\.test\.cjs)$|^ ?M npm/package\.json$' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
# Expected: "OK: scope clean".
git status --porcelain | grep -E 'bin/stagecoach\.js|\.goreleaser\.yaml|\.github/|internal/|PRD\.md|plan/|tasks\.json' && echo "FAIL: forbidden file" || echo "OK: no forbidden files"
# Expected: "OK: no forbidden files" (bin/stagecoach.js is S1's; .goreleaser.yaml + CI + Go are untouched).
```

## Final Validation Checklist

### Technical Validation
- [ ] `node --check npm/install.cjs` exit 0; `node --check npm/test/install.test.cjs` exit 0
- [ ] `cd npm && npm install --ignore-scripts` succeeds (tar + extract-zip fetchable; postinstall not run)
- [ ] package.json field guard (Level 1): dependencies present + exact; S1's fields (postinstall, engines, bin) preserved
- [ ] `cd npm && node test/install.test.cjs` prints `INSTALL-TEST PASS` (happy path + checksum-mismatch abort)

### Feature Validation
- [ ] install.cjs maps platform/arch → asset name matching `internal/upgrade/download.go assetName`/`checksumsName`
- [ ] install.cjs fetches checksums.txt **following the GitHub 302 redirect**; parses `<64hex>  <filename>` lines
- [ ] install.cjs downloads the archive + **SHA256-verifies constant-time**; mismatch removes the temp file + throws
- [ ] install.cjs extracts into `<cacheRoot>/<version>/<goos>-<goarch>/` so the binary lands where S1's shim resolves
- [ ] install.cjs fails LOUDLY (stderr message + `#install` fallback URL + exit 1) on any error / unsupported platform
- [ ] install.cjs dev-guard: version 0.0.0 + no override → notice + exit 0 (npm install during dev doesn't 404)
- [ ] install.cjs honors `STAGECOACH_DOWNLOAD_BASE` (test seam) + optional `STAGECOACH_GITHUB_TOKEN` (Bearer auth)

### Scope-Boundary Validation
- [ ] `git status --porcelain` shows ONLY `npm/install.cjs`, `npm/test/install.test.cjs`, modified `npm/package.json`
- [ ] NO `npm/bin/stagecoach.js` edit (S1's contract)
- [ ] NO `.github/workflows` edit (S3's contract — P2.M1.T1.S3 owns npm publish + windows zip CI)
- [ ] NO Go files, `.goreleaser.yaml`, README.md (root/npm), PRD.md, plan/**, tasks.json touched
- [ ] NO Go build/test run for this item (S2 adds no Go code — it is a Node subpackage)

### Documentation & Deployment
- [ ] package.json `dependencies` documented (tar for tar.gz; extract-zip for windows zip) — minimal + listed
- [ ] The loud-failure fallback URL matches S1's shim fallback URL (`#install`) for a consistent UX
- [ ] No committed binary fixtures (npm/test/ generates them at runtime) — node_modules is gitignored

---

## Anti-Patterns to Avoid

- ❌ Don't call the GitHub **API** (`api.github.com/.../releases/...`) in postinstall — the 60 req/h
  unauthenticated rate limit breaks installs at scale. Use the DIRECT `releases/download/<tag>/<asset>` URL
  (no rate limit, no auth needed for a public repo). This is the esbuild/turbo pattern.
- ❌ Don't assume Node's `https.get` follows redirects — it does NOT. GitHub's download URL 302-redirects to
  `objects.githubusercontent.com`; implement explicit redirect-following (or the download silently fails).
- ❌ Don't compare the SHA256 with `===` on hex strings — use `crypto.timingSafeEqual` on the digest BYTES
  (constant-time; mirror download.go VerifySHA256). Guard equal length first (timingSafeEqual throws otherwise).
- ❌ Don't leave a partial/tampered archive on disk on verify failure — remove the temp file before throwing
  (FR-U11 abort-before-write; mirror download.go DownloadAndVerifyArchive).
- ❌ Don't extract to a package-dir cache (`node_modules/...`) — a global root-owned install makes it unwritable.
  Extract to the home-dir versioned cache (`~/.stagecoach/versions/<v>/<goos>-<goarch>/`) S1's shim resolves.
- ❌ Don't put the leading "v" in the asset name — goreleaser `{{.Version}}` is the tag WITHOUT "v". Asset =
  `stagecoach_<version-no-v>_<os>_<arch>.ext`; the DOWNLOAD URL path uses the TAG (with v): `…/download/v<v>/…`.
- ❌ Don't shell out to system `tar`/`unzip` — Windows may lack them. Use the `tar` + `extract-zip` JS deps.
- ❌ Don't use sync shell-out for the whole flow — extraction (`tar.x`, `extract-zip`) is Promise-based; use an
  async main + try/catch that prints the fallback URL + `process.exit(1)` on any error.
- ❌ Don't edit `npm/bin/stagecoach.js` or add CI — those are S1's and S3's contracts. S2 populates the cache
  path S1's shim reads; it does not touch the shim.
- ❌ Don't drop the 0.0.0 dev guard — without it, a plain `npm install` during dev 404s on the nonexistent
  v0.0.0 release and fails confusingly. (The test sets `STAGECOACH_DOWNLOAD_BASE` to bypass it.)
- ❌ Don't add `"type":"module"` or `os`/`cpu` arrays — keep package.json CommonJS + single-package runtime
  detection (S1's contract). The `.cjs` extension forces CJS regardless.

---

## Confidence Score

**One-pass success likelihood: 9/10.** install.cjs is the JS analog of a LANDED, tested Go downloader
(`internal/upgrade/download.go`), so the asset-name/checksums/verify/abort logic is fully specified with
verified line-level precedent; the extraction deps (`tar` 7.5.22, `extract-zip` 2.0.1) have confirmed
Promise APIs (read from the package READMEs) and engines.node>=18 matches the package.json floor; the
cache-path scheme and platform map are an exact contract from S1's PRP; the local fixture-server test is
fully specified (build fixture with `tar.c`, serve with `node:http`, assert extract + mismatch-abort). The
one residual risk is the **GitHub 302-redirect-following** path, which the local test (plain http, no
redirect) does NOT exercise — it's exercised only against real GitHub at publish/CI; the gotcha is called
out explicitly with a verified helper pattern, and the failure mode (silent download failure) is loud
(non-2xx → throw → fallback URL). No Go/build/rate-limit surprises remain for the local-test scope.