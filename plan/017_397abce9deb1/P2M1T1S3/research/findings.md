# Research findings — P2.M1.T1.S3 (CI publish + version-sync + smoke)

Item: **CI publish step (npm publish on tag) + version-sync + smoke test**.
This is the THIRD of P2.M1.T1's three subtasks (S1 = package+shim LANDED; S2 = install.cjs
LANDED/in-progress; S3 = THIS — release.yml publish job + ci.yml shim smoke + npm/README publish notes).

All facts below are VERIFIED locally (commands shown) or cited from authoritative docs.

---

## 0. Scope & contracts (the starting state when S3 runs)

- **S1 LANDED** (committed): `npm/package.json` (version `0.0.0`, `bin.stagecoach=./bin/stagecoach.js`,
  `scripts.postinstall=node install.cjs`, `engines.node>=18`, no `dependencies`), `npm/bin/stagecoach.js`
  (the exec shim), `npm/README.md`, `npm/test/smoke.cjs` (shim smoke; SKIPS win32).
- **S2 LANDED** (committed): `npm/install.cjs` (postinstall downloader), `npm/package.json` MODIFIED
  (`"dependencies": { "tar": "^7.5.22", "extract-zip": "^2.0.1" }`), `npm/test/install.test.cjs`
  (install smoke; builds a fixture archive + checksums, serves via local http, asserts extract +
  checksum-mismatch abort).
- **S3 (this item)** ships ONLY: (a) `release.yml` npm-publish JOB, (b) `ci.yml` npm-smoke JOB, (c)
  `npm/README.md` "publish notes" section. NO Go, NO package.json committed-version change (version sync
  is TRANSIENT in the CI runner), NO S1/S2 file edits.
- `.github/workflows/release.yml` TRIGGERS on `v*` tags; job `goreleaser` runs
  `goreleaser-action@v6` with `release --clean --skip=homebrew,scoop,aur` (first release). `permissions:
  contents: write`. Secrets used: GITHUB_TOKEN (auto), HOMEBREW_TAP_GITHUB_TOKEN, SCOOP_BUCKET_GITHUB_TOKEN,
  AUR_SSH_PRIVATE_KEY.
- `.github/workflows/ci.yml` TRIGGERS on push(main)+pull_request; jobs: build-test (matrix ubuntu/macos/
  windows × go 1.24/1.25), lint, vulncheck, coverage. `permissions: contents: read`. concurrency cancels
  superseded PR runs.

---

## 1. The npm-publish job (release.yml) — verified mechanism

### 1a. setup-node + registry-url + NODE_AUTH_TOKEN (the canonical publish pattern)

Official guide: https://docs.github.com/actions/publishing-packages/publishing-nodejs-packages
setup-node README: https://github.com/actions/setup-node

```yaml
- uses: actions/setup-node@v4
  with:
    node-version: '20'                 # contract says Node 20; engines.node>=18 is satisfied
    registry-url: 'https://registry.npmjs.org'   # writes ~/.npmrc auth line
- run: npm publish --access public
  env:
    NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}   # MUST be on the STEP that runs npm publish
```

- setup-node with `registry-url` writes a user-level `~/.npmrc`:
  `//registry.npmjs.org/:_authToken=${NODE_AUTH_TOKEN}`. The `${NODE_AUTH_TOKEN}` is resolved from the
  step's env, so NODE_AUTH_TOKEN **must** be set on the publish STEP (not just the job).
- setup-node latest release is **v7.0.0** (verified via the GitHub API), but **v4** is the version shown
  in the official "Publishing Node.js packages" doc and matches the repo's `actions/checkout@v4`
  convention. **Use @v4** (stable, doc-aligned).
- Scoped packages (`@dabstractor/stagecoach`) default to **PRIVATE** on npm → `--access public` is
  REQUIRED for the first (and every) publish (verified: npm docs + npm publish --help shows
  `[--access <restricted|public|private>]`; "If you do not want your scoped package to be publicly
  viewable..." → without `--access public` a scoped publish is rejected/becomes private).

### 1b. Version sync from the tag (DECISION: `npm version` from `$GITHUB_REF`)

The tag is `v1.0.0`; the npm version must be `1.0.0` (goreleaser `{{.Version}}` = tag WITHOUT leading v,
per .goreleaser.yaml `ldflags -X main.version={{.Version}}`). Strip:

```bash
VERSION="${GITHUB_REF#refs/tags/v}"     # refs/tags/v1.0.0 -> 1.0.0 ; refs/tags/v1.0.0-rc.1 -> 1.0.0-rc.1
```

(`GITHUB_REF_NAME` = `v1.0.0` also works: `${GITHUB_REF_NAME#v}`. Either is fine; GITHUB_REF is universal.)

Then set it with the idiomatic `npm version` (VALIDATED LOCALLY — see §6):

```bash
cd npm && npm version "$VERSION" --no-git-tag-version
```

- `--no-git-tag-version` skips ALL git operations (no tag, no commit, NO dirty-tree refusal). VERIFIED: a
  second `npm version ... --no-git-tag-version` call succeeds immediately after the first (no "Git working
  directory not clean" error) — the dirty-tree check is gated behind `--git-tag-version` (the default),
  which we disable.
- This is TRANSIENT: it edits the CHECKED-OUT `npm/package.json` in the runner only. The committed
  package.json stays at `0.0.0` (the dev placeholder). S3 does NOT commit a bumped version.
- `npm version` VALIDATES semver (errors on a malformed version) — a bonus over raw `sed`/node edits.

### 1c. `npm install` before publish — the package-lock.json trap (VERIFIED)

The contract says "runs `npm install` (to pin the deps)". Intent: validate `tar@^7.5.22` +
`extract-zip@^2.0.1` resolve on the registry before publishing.

**TRAP (verified locally):** `npm install --ignore-scripts` in `npm/` CREATES `package-lock.json`
(8422 bytes), and `npm pack` INCLUDES it in the tarball by default → the published package would carry a
lockfile (unnecessary noise for a binary-downloader wrapper).

**TRAP 2:** `npm install` (WITHOUT `--ignore-scripts`) runs `postinstall` = `node install.cjs`, which
downloads the REAL binary for the just-synced version (e.g. v1.0.0) from GitHub Releases. We do NOT want
to download a binary during publish (publish ships JS only; the binary is fetched at USER install time).

**FIX:** use `npm install --ignore-scripts --no-package-lock`:
- `--ignore-scripts` → postinstall does NOT run (no binary download during publish).
- `--no-package-lock` → no lockfile created (clean tarball).
- VERIFIED: with `--no-package-lock`, `npm pack --dry-run` shows exactly the 6 source files (README.md,
  bin/stagecoach.js, install.cjs, package.json, test/install.test.cjs, test/smoke.cjs) at 26.4KB unpacked
  — NO lockfile, NO node_modules (always excluded by npm pack regardless).

### 1d. Job dependency: `needs: goreleaser`

The npm-publish job must run AFTER goreleaser completes:
- goreleaser creates the GitHub Release with the 6 archives + `checksums.txt`. When a user later runs
  `npm install -g @dabstractor/stagecoach`, the postinstall (`install.cjs`) fetches the binary from THAT
  release. If npm publish happened BEFORE goreleaser, a user installing in the gap would 404.
- Publish itself ships ONLY JS (no binary fetch at publish time) — but the Release must EXIST for user
  installs. `needs: goreleaser` also fail-fasts: if goreleaser failed, do not publish an npm package whose
  binary does not exist.
- npm-publish is INDEPENDENT of goreleaser's `--skip=homebrew,scoop,aur` flags (those gate only the
  goreleaser-internal pipes, not sibling jobs).

---

## 2. The npm-smoke job (ci.yml) — verified mechanism

The contract: "Add a CI smoke (in ci.yml, not release): `node npm/bin/stagecoach.js --version` against a
fake-cached binary to catch shim regressions pre-release." S1's `npm/test/smoke.cjs` IS that smoke (places
a fake binary, runs the shim, asserts exec + `STAGECOACH_INSTALL_METHOD=npm` + exit-code propagation +
fallback). S2's `npm/test/install.test.cjs` is the install.cjs smoke (deferred to S3/CI per S2's PRP).

### 2a. smoke.cjs skips win32 (S1 contract)

S1's smoke.cjs: `if (process.platform === 'win32') { console.log('skipped...'); process.exit(0); }`.
Rationale (S1): spawnSync to a fake `.exe` needs a real PE binary; a `.cmd` won't run without
`shell:true` (which the shim omits for argv fidelity). So on windows, smoke.cjs is a no-op (exit 0).

### 2b. install.test.cjs CANNOT validate the windows zip path (S2 implementation reality)

VERIFIED by reading the LANDED `npm/test/install.test.cjs`: the fixture archive is ALWAYS built with
`tar.c({ file: archivePath, cwd: serveDir, gzip: true }, [fakeBinName])` — i.e. a **tar.gz** — even when
`ext === '.zip'` (win32) makes `assetName` end in `.zip`. On win32:
- `install.cjs` computes `ext='.zip'` → calls `extractZip(archivePath, {dir})`.
- But `archivePath` contains a **tar.gz** stream (the `.zip` name is cosmetic).
- `extract-zip` on a tar.gz stream THROWS → the happy-path assertion `r.status === 0` FAILS.

So running `install.test.cjs` on `windows-latest` WOULD FAIL (red CI). S2's PRP prose claims "the win32
branch builds a `.zip` fixture", but the LANDED code does not — there is a prose/code inconsistency in
S2. **CONSEQUENCE for S3:** the windows zip-extraction path is NOT coverable by S2's test as-shipped.
S3 must GATE `install.test.cjs` to NON-windows runners to keep CI green, and DOCUMENT the gap
(pending S2 shipping a real-zip fixture, e.g. via `extract-zip`/`adm-zip` or Node's zlib).

### 2c. Matrix design (DECISION)

`npm-smoke` job, matrix `[ubuntu-latest, macos-latest, windows-latest]` (mirrors build-test). On ALL:
1. `node --check npm/bin/stagecoach.js` + `node --check npm/install.cjs` (syntax; cross-platform).
2. `cd npm && npm install --ignore-scripts` (fetch tar+extract-zip; validates deps resolve on every OS;
   `--ignore-scripts` so postinstall does NOT run — the 0.0.0 dev guard would also skip it, but
   `--ignore-scripts` is explicit).
3. `node npm/test/smoke.cjs` (shim smoke — full on unix, skip on win32).
4. `node npm/test/install.test.cjs` — **GATED `if: matrix.os != 'windows-latest'`** (S2's tar.gz-fixture
   bug, §2b). On ubuntu+macos it validates the tar.gz download→verify→extract + mismatch-abort path.

### 2d. No new test file

S3 does NOT write a new smoke test — it WIRES S1's `smoke.cjs` + S2's `install.test.cjs` into ci.yml.
(Both files already exist and self-validate; S3 only adds the CI job that runs them.)

---

## 3. npm/README.md publish notes (DOCS, Mode A)

S1's `npm/README.md` is a one-paragraph user doc + `## Install`. S3 APPENDS a maintainer section
documenting the publish pipeline (the contract's "npm/README.md publish notes"). Keep it AFTER the user
content so the npm package page stays user-friendly. Suggested heading: `## Publishing (for maintainers)`.
Content: the package is published automatically on `v*` tag push by release.yml's `npm-publish` job;
requires the `NPM_TOKEN` repo secret (an npm **automation token**); version is synced from the tag
(`vX.Y.Z` → `X.Y.Z`) at publish time (committed package.json stays `0.0.0`); `--access public` because
the package is scoped.

---

## 4. NPM_TOKEN secret — type + gotchas

- npm offers three token types: **Automation** (bypasses 2FA — ideal for CI, non-interactive),
  **Publish** (respects 2FA — may fail in CI if 2FA is on), **Read-only** (cannot publish).
- For CI: use an **Automation** token (or a fine-grained access token with publish permission, scoped to
  this package). Document in release.yml comments: `NPM_TOKEN` = npm automation token (npmjs.com →
  Access Tokens → "Automation"). Add it under repo Settings → Secrets → Actions.
- 2FA gotcha: if the npm ACCOUNT has publish 2FA enabled, a non-automation token will fail in CI with
  `npm ERR! This operation requires a one-time password`. An Automation token bypasses this. Document.

---

## 5. Prerelease dist-tag (OPTIONAL enhancement — NOT in contract)

`npm publish` defaults to the `latest` dist-tag. For a prerelease tag like `v1.0.0-rc.1`, publishing with
the default `latest` would make the RC the "latest" install (`npm install -g @dabstractor/stagecoach` →
RC). The contract uses plain `npm publish --access public` (no `--tag`). To avoid an RC becoming latest,
one could publish prereleases with `--tag next`. **DECISION: follow the contract (plain `--access public`);
document the prerelease-tag consideration as an OPTIONAL future enhancement.** The first real release is
v0.1.0 (non-prerelease), so this is not immediately blocking.

---

## 6. Local validation commands (all VERIFIED in this env)

```bash
# YAML validity (python3 + pyyaml AVAILABLE here; actionlint is NOT via npx)
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml')); print('release.yml OK')"
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml')); print('ci.yml OK')"

# Version sync works in the real git tree (clean-checkout sim), twice in a row (no dirty-tree refusal):
cd npm && npm version 7.7.7 --no-git-tag-version   # -> v7.7.7, package.json now 7.7.7 (NO commit/tag)
# (restore: git checkout -- npm/package.json)

# What would be published (NO auth needed; shows the exact tarball contents):
cd npm && npm version 1.2.3 --no-git-tag-version && npm pack --dry-run
#   -> 6 files: README.md, bin/stagecoach.js, install.cjs, package.json, test/*.cjs ; NO lockfile/node_modules
# (restore: git checkout -- npm/package.json)

# Smoke tests run green locally (S1/S2 deliverables; S3 only wires them into CI):
node npm/test/smoke.cjs                 # -> SMOKE PASS (unix)
cd npm && npm install --ignore-scripts && node test/install.test.cjs  # -> INSTALL-TEST PASS (unix tar.gz)
```

- **actionlint** is a Go tool; NOT installable via `npx actionlint` (verified — "could not determine
  executable"). Install via `go install github.com/rhysd/actionlint/cmd/actionlint@latest` or the release
  binary. RECOMMEND it in the PRP, but the VERIFIED local gate is python3+pyyaml (available here).
- Node here is v26.4.0 / npm 11.18.0 (newer than CI's Node 20 / npm 10). `npm version` + `npm pack`
  behavior is stable across npm 10/11.

---

## 7. Scope-boundary checklist (what S3 does NOT touch)

- ❌ NO Go files, `.goreleaser.yaml`, Makefile, go.mod/sum.
- ❌ NO committed change to `npm/package.json` (version sync is transient in the runner; committed
  version stays `0.0.0`). NO `files` field added (test/ in the tarball is harmless ~9KB; slimming it is
  out of scope and would touch S1/S2's package.json).
- ❌ NO edit to `npm/bin/stagecoach.js`, `npm/install.cjs`, `npm/test/*` (S1/S2's contracts).
- ❌ NO `.gitignore` change (npm/node_modules in the runner is ephemeral; `/node_modules/` at repo root
  is already ignored — note: that anchor does NOT match `npm/node_modules`, but the runner never commits,
  so it is irrelevant for CI; local-dev git noise from `npm/node_modules` is a pre-existing S1/S2 concern,
  not S3's).
- ❌ NO PRD.md, plan/**, tasks.json, prd_snapshot.md.
- ✅ EDITS: `.github/workflows/release.yml` (add npm-publish job + NPM_TOKEN comments),
  `.github/workflows/ci.yml` (add npm-smoke job + comments), `npm/README.md` (append publish-notes
  section).