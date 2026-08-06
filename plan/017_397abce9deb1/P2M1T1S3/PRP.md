name: "P2.M1.T1.S3 — CI publish step (npm publish on tag) + version-sync + smoke test"
description: |
  The third of P2.M1.T1's three subtasks: the CI PUBLISH + SMOKE wiring for `@dabstractor/stagecoach`
  (PRD §21.2 "Beyond goreleaser" / §21.4; external_deps.md §3). goreleaser has NO npm pipe, so the wrapper
  is published by a SEPARATE CI step on tag. This PRP adds (a) an `npm-publish` JOB to
  `.github/workflows/release.yml` (after the `goreleaser` job) that: checks out, sets up Node 20 with
  `registry-url`, syncs `npm/package.json` `version` to the Go release tag (`vX.Y.Z`→`X.Y.Z` = goreleaser
  `{{.Version}}`) via `npm version "$VERSION" --no-git-tag-version` (TRANSIENT — committed package.json
  stays `0.0.0`), runs `npm install --ignore-scripts --no-package-lock` (validates tar/extract-zip resolve
  WITHOUT running postinstall or creating a lockfile), then `npm publish --access public` with
  `NODE_AUTH_TOKEN=${{ secrets.NPM_TOKEN }}` (scoped package → `--access public` REQUIRED); (b) an
  `npm-smoke` JOB to `.github/workflows/ci.yml` (matrix ubuntu/macos/windows) that runs S1's
  `npm/test/smoke.cjs` (shim: fake-cached binary → exec + `STAGECOACH_INSTALL_METHOD=npm` + exit-code +
  fallback) and S2's `npm/test/install.test.cjs` (installer: fixture server → download+SHA256-verify+
  extract + mismatch-abort) to catch regressions pre-release; the install smoke is GATED to non-windows
  (S2's test builds a tar.gz even for `.zip`-named assets, so it cannot validate the windows zip path —
  documented gap); (c) `npm/README.md` "publish notes" (Mode A maintainer doc). Documents the `NPM_TOKEN`
  secret requirement (npm AUTOMATION token — bypasses publish 2FA for non-interactive CI) in release.yml
  comments. NO Go, NO committed package.json change, NO S1/S2 file edits. Validates via python3+pyyaml
  workflow parse, `npm pack --dry-run` (no auth — shows the exact tarball), `npm version` round-trip, and
  the already-green smoke tests. NOTE: the task's `prd_selectors` (§9.26 work-description mode) is a
  MISMATCH — the authoritative spec is external_deps.md §3 + .goreleaser.yaml + the contract + research.

---

## Goal

**Feature Goal**: Ship the CI publish pipeline so that pushing a `v*` tag (which already fires goreleaser)
ALSO publishes `@dabstractor/stagecoach` to the npm registry with a version synced to the tag, and so that
every PR runs the npm-wrapper smoke tests (shim + installer) to catch regressions BEFORE a release —
without ever downloading a binary at publish time and without polluting the published tarball with a
lockfile.

**Deliverable**:
- `.github/workflows/release.yml` — **MODIFIED**: a new `npm-publish` JOB (`needs: goreleaser`) + inline
  comments documenting the `NPM_TOKEN` secret (automation token) + the version-sync rationale.
- `.github/workflows/ci.yml` — **MODIFIED**: a new `npm-smoke` JOB (matrix ubuntu/macos/windows) that runs
  S1's `smoke.cjs` + S2's `install.test.cjs` (+ `node --check` syntax gates), with `install.test.cjs`
  gated to non-windows.
- `npm/README.md` — **MODIFIED**: APPEND a `## Publishing (for maintainers)` section (Mode A publish notes).

**Success Definition**:
- Both workflow files are valid YAML: `python3 -c "import yaml; yaml.safe_load(open(p))"` exits 0 for each.
- The publish job's version-sync + tarball logic is validated LOCALLY (no tag/NPM_TOKEN needed):
  `cd npm && npm version 1.2.3 --no-git-tag-version && npm pack --dry-run` shows EXACTLY the 6 source
  files (README.md, bin/stagecoach.js, install.cjs, package.json, test/*.cjs) and NO `package-lock.json` /
  NO `node_modules`; then `git checkout -- npm/package.json` restores `0.0.0`.
- The `npm version` round-trip works in the real git tree: bump → `node -p "require('./npm/package.json').version"`
  prints the new value; restore → prints `0.0.0` (proves the committed package.json is untouched by S3).
- The wired smoke tests are green on a unix host: `node npm/test/smoke.cjs` → `SMOKE PASS`; after
  `cd npm && npm install --ignore-scripts`, `node test/install.test.cjs` → `INSTALL-TEST PASS`.
- `git status --porcelain` shows ONLY the two `.github/workflows/*.yml` (modified) + `npm/README.md`
  (modified). NO Go, NO `npm/package.json` (committed), NO `npm/bin|install.cjs|test/*`, NO PRD/plan.

## User Persona (if applicable)

**Target User**: (1) The JS/TS developer who installs stagecoach via npm — the publish job makes
`npm install -g @dabstractor/stagecoach` resolve to the tag's version with a matching native binary. (2)
The stagecoach maintainer — the publish is fully automated (push tag → npm published) and the smoke job
catches wrapper regressions on every PR before they ship.
**Use Case**: Maintainer pushes `git tag v1.2.0 && git push origin v1.2.0` → release.yml fires goreleaser
(archives + checksums + GitHub Release) THEN the `npm-publish` job syncs `npm/package.json` to `1.2.0`,
validates deps, and runs `npm publish --access public` → `@dabstractor/stagecoach@1.2.0` is live; a user
running `npm install -g @dabstractor/stagecoach` gets `1.2.0` + postinstall fetches the `1.2.0` binary.
Meanwhile every PR runs `npm-smoke` so a shim/installer regression turns the PR red before release.
**Pain Points Addressed**: (1) a wrapper version that DRIFTS from the Go release tag (npm shows 0.0.0 or a
stale version) → version-sync from `$GITHUB_REF`; (2) a publish that downloads a multi-MB binary into the
runner or ships a lockfile → `--ignore-scripts --no-package-lock`; (3) a wrapper regression (shim or
installer) that ships undetected → the `npm-smoke` CI job on every PR.

## Why

- **PRD §21.2 "Beyond goreleaser"**: goreleaser has NO native npm pipe; the wrapper is "built/published by
  a separate CI step (a GitHub Action that `npm publish`s on tag). The native binaries come from the
  goreleaser GitHub Release (already produced). So the wrapper's release step only ships the JS; the
  binaries are downloaded at install time." S3 IS that separate CI step. S1 (package+shim) + S2
  (install.cjs) are LANDED; S3 is the publish + smoke wiring that completes P2.M1.T1.
- **PRD §21.4 versioning**: semver; the npm version must equal the Go release tag's `{{.Version}}` (tag
  without leading v). The version-sync step enforces this invariant at publish time without committing a
  version (the committed `0.0.0` is the dev placeholder).
- **Scope discipline**: S3 ships ONLY release.yml + ci.yml job additions + npm/README publish notes. No Go,
  no committed package.json change, no S1/S2 edits. The Node distribution publish surface is reviewable in
  isolation (two workflow diffs + one doc append).

## What

Two workflow JOBS added (release.yml: `npm-publish`; ci.yml: `npm-smoke`) plus a `npm/README.md` publish-
notes section. The committed `npm/package.json` version stays `0.0.0`; the real version is written
transiently in the publish runner from the tag.

### Success Criteria
- [ ] `release.yml` has an `npm-publish` job with `needs: goreleaser`, `runs-on: ubuntu-latest`,
      `actions/checkout@v4`, `actions/setup-node@v4` (`node-version: '20'`, `registry-url:
      'https://registry.npmjs.org'`), a version-sync step (`VERSION="${GITHUB_REF#refs/tags/v}"` →
      `npm version "$VERSION" --no-git-tag-version` in `npm/`), a dep-validation step
      (`npm install --ignore-scripts --no-package-lock`), and a publish step
      (`npm publish --access public` with `env.NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}`).
- [ ] `release.yml` has an inline comment block documenting `NPM_TOKEN` (npm **automation** token —
      bypasses publish 2FA for non-interactive CI; add under repo Settings → Secrets → Actions).
- [ ] `ci.yml` has an `npm-smoke` job (matrix `ubuntu-latest`, `macos-latest`, `windows-latest`;
      `fail-fast: false`) running `node --check npm/bin/stagecoach.js` + `node --check npm/install.cjs`,
      then `cd npm && npm install --ignore-scripts`, then `node npm/test/smoke.cjs`, then
      `node npm/test/install.test.cjs` GATED `if: matrix.os != 'windows-latest'`.
- [ ] `npm/README.md` APPENDS a `## Publishing (for maintainers)` section (S1's content preserved above it).
- [ ] Both workflow files parse as valid YAML (python3+pyyaml).
- [ ] Local `npm pack --dry-run` (after a transient `npm version`) shows the 6 source files and NO
      `package-lock.json` / NO `node_modules`.
- [ ] `git status --porcelain` shows ONLY the 2 modified workflow files + modified `npm/README.md`.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact setup-node+NODE_AUTH_TOKEN publish pattern (cited official doc), the verified
`npm version --no-git-tag-version` version-sync (with the dirty-tree non-issue proven), the
`package-lock.json`/postinstall traps and their `--ignore-scripts --no-package-lock` fix (verified by
local `npm pack`), the `--access public` requirement for scoped packages (npm docs + help), the
`needs: goreleaser` ordering rationale, the ci.yml matrix design with the windows `install.test.cjs` gate
(justified by S2's tar.gz-fixture reality), the NPM_TOKEN automation-token requirement, and the precise
S1/S2/S3 scope boundary (committed package.json stays 0.0.0; S1/S2 files untouched).

### Documentation & References

```yaml
# MUST READ — the codebase-specific findings for THIS item (verified locally + cited docs).
- docfile: plan/017_397abce9deb1/P2M1T1S3/research/findings.md
  why: "§1 the npm-publish job mechanism (setup-node+NODE_AUTH_TOKEN, version sync, the package-lock +
        postinstall traps + --no-package-lock fix, needs:goreleaser); §2 the ci.yml npm-smoke design +
        the windows install.test.cjs gate (S2 builds tar.gz even for .zip assets); §3 the README publish
        notes shape; §4 NPM_TOKEN automation-token; §5 the optional prerelease --tag note; §6 ALL local
        validation commands (verified); §7 the scope boundary."

# MUST READ — S1's PRP is a CONTRACT: the shim (smoke.cjs) + package.json S3 wires into CI / publishes.
- docfile: plan/017_397abce9deb1/P2M1T1S1/PRP.md
  why: "S1's npm/package.json (version 0.0.0 dev placeholder; bin; scripts.postinstall; engines.node>=18),
        npm/bin/stagecoach.js (the shim), npm/test/smoke.cjs (the shim smoke — SKIPS win32), npm/README.md
        (S1's one-paragraph doc + ## Install — S3 APPENDS to it). S3 publishes this package and wires
        smoke.cjs into ci.yml."

# MUST READ — S2's PRP is a CONTRACT: install.cjs + install.test.cjs S3 wires into CI.
- docfile: plan/017_397abce9deb1/P2M1T1S2/PRP.md
  why: "S2's npm/install.cjs (postinstall downloader) + npm/package.json dependencies field (tar ^7.5.22,
        extract-zip ^2.0.1) + npm/test/install.test.cjs (the install smoke). S3 wires install.test.cjs
        into ci.yml (GATED to non-windows — see findings §2b: S2's test builds tar.gz even for .zip assets)."

# MUST READ — the authoritative wrapper + checksums format spec (the wrapper is the esbuild/turbo/prisma
# pattern; publish ships ONLY JS, binaries download at USER install time from the goreleaser GitHub Release).
- docfile: plan/017_397abce9deb1/architecture/external_deps.md
  section: "§3 npm wrapper (single-package + postinstall-download form; 'goreleaser has NO native npm
            pipe ... The wrapper is built/published by a separate CI step (a GitHub Action that npm
            publishs on tag). The native binaries come from the goreleaser GitHub Release (already
            produced). So the wrapper's release step only ships the JS; the binaries are downloaded at
            install time.'); §1 the GitHub Releases download URL the postinstall fetches at install time."
  critical: "§3 states the publish step ships JS ONLY; the binary fetch happens at USER install time (so
             publish must NOT run postinstall -> --ignore-scripts). §1 notes the 60-req/h API rate limit
             (install.cjs already uses the direct URL, not the API — S2's concern, not S3's)."

# MUST READ — the version source: goreleaser {{.Version}} = tag WITHOUT leading v.
- file: .goreleaser.yaml
  why: "ldflags '-X main.version={{.Version}}' and archives.name_template '{{ .ProjectName }}_{{ .Version }}
        _{{ .Os }}_{{ .Arch }}' — {{.Version}} is the tag WITHOUT leading v (e.g. tag v1.0.0 -> 1.0.0).
        The npm version MUST equal this value; the version-sync step derives it from $GITHUB_REF."

# MUST READ — the existing release.yml (S3 ADDS the npm-publish job after `goreleaser`; preserves all of it).
- file: .github/workflows/release.yml
  why: "The `goreleaser` job (name to depend on), the secrets header block (ADD NPM_TOKEN to the comment),
        the `permissions: contents: write`, the `on: push: tags: v*` trigger (npm-publish rides the same
        trigger). S3 ADDS a sibling job; it does NOT modify the goreleaser job."
  pattern: "Job structure: runs-on ubuntu-latest; steps: actions/checkout@v4 (fetch-depth: 0 NOT needed for
            npm), actions/setup-... , run: ... . Match this shape for the new npm-publish job."

# MUST READ — the existing ci.yml (S3 ADDS the npm-smoke job; preserves all jobs + the concurrency block).
- file: .github/workflows/ci.yml
  why: "The build-test matrix shape (os: [ubuntu-latest, macos-latest, windows-latest]; fail-fast: false;
        shell: bash for cross-platform steps), the `permissions: contents: read`, the concurrency group.
        S3 ADDS a sibling npm-smoke job mirroring the matrix; it does NOT modify existing jobs."

# VERIFIED — the canonical npm-publish-in-GitHub-Actions pattern (official doc + setup-node README).
- url: https://docs.github.com/actions/publishing-packages/publishing-nodejs-packages
  why: "The official guide: setup-node with `registry-url` writes the auth .npmrc; set
        `NODE_AUTH_TOKEN` as env on the publish STEP; `npm publish`. Use actions/setup-node@v4 (the version
        shown in the guide; matches the repo's actions/checkout@v4 convention). setup-node latest is v7 but
        v4 is doc-stable + widely used."
- url: https://github.com/actions/setup-node
  why: "`registry-url` input writes `//<registry>/:_authToken=${NODE_AUTH_TOKEN}` to ~/.npmrc; the token is
        resolved from the step env. `node-version` selects the toolchain (use '20' per the contract)."

# VERIFIED — npm publish (scoped → --access public) + npm version (--no-git-tag-version).
- url: https://docs.npmjs.com/cli/v10/commands/npm-publish
  why: "`--access public` is REQUIRED to publish a scoped (@scope/name) package publicly (scoped packages
        default to private/restricted). `npm publish --dry-run` / `npm pack --dry-run` shows the tarball
        contents WITHOUT uploading (the local validation)."
- url: https://docs.npmjs.com/cli/v10/commands/npm-version
  why: "`npm version <ver> --no-git-tag-version` updates package.json's version WITHOUT a git commit/tag
        and WITHOUT the dirty-tree check (the check is gated behind --git-tag-version). Validated locally."

# OUT OF SCOPE — S1/S2's files (do NOT edit) + Go/release config.
- file: npm/bin/stagecoach.js          # S1's shim — S3 does NOT touch.
- file: npm/install.cjs                # S2's installer — S3 does NOT touch.
- file: npm/test/smoke.cjs             # S1's shim smoke — S3 only RUNS it in ci.yml.
- file: npm/test/install.test.cjs      # S2's installer smoke — S3 only RUNS it in ci.yml (non-windows).
- file: npm/package.json               # committed version stays 0.0.0; S3 bumps it TRANSIENTLY in the runner only.
- out_of_scope: "Go files, .goreleaser.yaml, Makefile, go.mod/sum, PRD.md, plan/**, tasks.json, .gitignore."
```

### Current Codebase tree (relevant slice — READ-ONLY references)

```bash
# S1 LANDED:                       # S2 LANDED:
npm/package.json   (version 0.0.0, dependencies tar^7.5.22 + extract-zip^2.0.1)
npm/bin/stagecoach.js              npm/install.cjs
npm/README.md  (S1's one-para + ## Install)
npm/test/smoke.cjs                 npm/test/install.test.cjs
.goreleaser.yaml   ({{.Version}} = tag w/o v; 6 targets linux/darwin/windows × amd64/arm64)
.github/workflows/release.yml      # S3 ADDS the npm-publish job here
.github/workflows/ci.yml           # S3 ADDS the npm-smoke job here
plan/017_397abce9deb1/architecture/external_deps.md   # §1, §3 (the wrapper spec)
plan/017_397abce9deb1/P2M1T1S3/research/findings.md   # THIS item's verified research
```

### Desired Codebase tree with files to be MODIFIED

```bash
.github/workflows/
  release.yml        # MODIFIED — ADD `npm-publish` job (needs: goreleaser) + NPM_TOKEN comment block
  ci.yml             # MODIFIED — ADD `npm-smoke` job (matrix; runs S1 smoke + S2 install smoke)
npm/
  README.md          # MODIFIED — APPEND `## Publishing (for maintainers)` section (S1's content preserved)
# NOT touched by S3: npm/package.json (committed), npm/bin/*, npm/install.cjs, npm/test/*, all Go, .goreleaser.yaml
```

### Known Gotchas of our codebase & Library Quirks

```yaml
# CRITICAL (scoped package → --access public is MANDATORY): @dabstractor/stagecoach is a scoped package;
#   npm defaults scoped packages to PRIVATE/restricted. `npm publish` WITHOUT `--access public` is REJECTED
#   (or publishes privately). The publish step MUST use `npm publish --access public`. (Verified: npm docs +
#   `npm publish --help` shows --access; multiple sources confirm scoped → public requires the flag.)

# CRITICAL (NODE_AUTH_TOKEN must be on the PUBLISH STEP env, not just the job): setup-node with registry-url
#   writes `//registry.npmjs.org/:_authToken=${NODE_AUTH_TOKEN}` to ~/.npmrc. The ${NODE_AUTH_TOKEN} is
#   resolved from the ENV of the step running `npm publish`. If you set it only at job-level `env:` it MAY
#   work (job env propagates to steps), but the CANONICAL pattern (official doc) sets it on the publish STEP.
#   Set `env: NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}` on the `npm publish` step explicitly.

# CRITICAL (npm install before publish creates package-lock.json → pollutes the tarball): VERIFIED —
#   `npm install --ignore-scripts` in npm/ writes an 8KB package-lock.json, and `npm pack` INCLUDES it.
#   FIX: `npm install --ignore-scripts --no-package-lock` (no lockfile created; npm pack shows only source).

# CRITICAL (npm install WITHOUT --ignore-scripts runs postinstall → downloads the real binary): the
#   publish job's `npm install` MUST use `--ignore-scripts` so install.cjs (postinstall) does NOT run. A real
#   version (e.g. 1.2.0) WOULD trigger a binary download from GitHub Releases (the 0.0.0 dev guard won't fire
#   once version-synced). Publish ships JS ONLY; the binary fetch is a USER-install-time concern. Use
#   `--ignore-scripts --no-package-lock` together.

# CRITICAL (version sync is TRANSIENT — do NOT commit a bumped package.json): the committed npm/package.json
#   version is the `0.0.0` dev placeholder (S1). The publish job bumps it to the tag's version IN THE RUNNER
#   ONLY (the actions/checkout is a clean detached-HEAD copy that is discarded after the job). S3 makes NO
#   committed change to npm/package.json. `npm version "$VERSION" --no-git-tag-version` edits the file in
#   place without a commit/tag — exactly what we want.

# CRITICAL (npm version --no-git-tag-version does NOT refuse a dirty tree): VERIFIED — a second
#   `npm version X --no-git-tag-version` call succeeds immediately after the first (no "Git working directory
#   not clean"). The dirty-tree check is gated behind --git-tag-version (the default), which we disable. So
#   the single version-sync call is robust in the clean CI checkout.

# CRITICAL (the install.test.cjs windows gate): S2's install.test.cjs builds the fixture with
#   `tar.c({gzip:true})` ALWAYS — even when assetName ends in `.zip` (win32). So on a windows runner,
#   install.cjs calls extractZip on a tar.gz stream → THROWS → the happy-path assertion FAILS (red CI).
#   GATE `node npm/test/install.test.cjs` to `if: matrix.os != 'windows-latest'`. The windows
#   zip-extraction path is a DOCUMENTED GAP (pending S2 shipping a real-zip fixture). smoke.cjs also skips
#   win32 (S1's contract), so windows gets: syntax checks + deps-resolve only. Document this in ci.yml comments.

# GOTCHA (needs: goreleaser, not just "after"): the npm-publish job must DECLARE `needs: goreleaser` so it
#   runs only AFTER the goreleaser job succeeds (the GitHub Release with archives + checksums must EXIST
#   before users `npm install` — the postinstall fetches from it; and fail-fast if goreleaser failed).
#   Without `needs:`, the two jobs run CONCURRENTLY (both start at trigger time) — npm could publish before
#   the Release exists, 404'ing early installers.

# GOTCHA (NPM_TOKEN type): use an npm AUTOMATION token (npmjs.com → Access Tokens → "Automation"), NOT a
#   "Publish" token. Automation tokens BYPASS publish 2FA — required for non-interactive CI (a Publish token
#   fails with "This operation requires a one-time password" if the account has publish 2FA on). Document
#   this in the release.yml comment block. Add the secret under repo Settings → Secrets → Actions.

# GOTCHA (Node 20 satisfies engines.node>=18): the contract says "sets up Node 20"; npm/package.json
#   engines.node is ">=18". Node 20 (an LTS) satisfies >=18. Use node-version: '20'. (Do NOT downgrade to 18
#   "to match the floor" — 20 is the contract and is a safer LTS.)

# GOTCHA (the published tarball includes test/ — harmless, do NOT add a `files` field): `npm pack --dry-run`
#   shows test/smoke.cjs + test/install.test.cjs in the tarball (~9KB extra). This is HARMLESS (users could
#   even run them). Slimming via a `files` field would MODIFY package.json (S1/S2's contract) and is OUT OF
#   SCOPE for S3. Leave it; note it as an optional future enhancement in the README publish notes.

# GOTCHA (prerelease tags default to the `latest` dist-tag): for a tag like v1.0.0-rc.1, `npm publish
#   --access public` tags it `latest`, so `npm install -g @dabstractor/stagecoach` would install the RC.
#   The CONTRACT uses plain `--access public` (no `--tag`). Follow the contract; note `--tag next` for
#   prereleases as an OPTIONAL enhancement in the README. The first real release (v0.1.0) is non-prerelease,
#   so this is not immediately blocking.

# GOTCHA (no `fetch-depth: 0` needed for npm-publish): the goreleaser job needs full history for the
#   changelog; the npm-publish job does NOT (it only reads npm/package.json + the tag name). Use a plain
#   `actions/checkout@v4` (default fetch-depth 1) for npm-publish.

# GOTCHA (shell: bash on windows-latest): the ci.yml build-test job pins `shell: bash` for cross-platform
#   steps because windows-latest defaults to PowerShell. The npm-smoke steps use `cd npm && ...` and
#   `node ...` which PowerShell also handles, BUT pin `shell: bash` on any step using bash-isms (&&, $VAR)
#   for safety. `node npm/test/smoke.cjs` (no shell-isms) is fine on any shell.
```

## Implementation Blueprint

### Data models and structure

No data models — this is CI YAML + one Markdown section. The "data" is:
- the `npm-publish` job's step sequence (checkout → setup-node → version-sync → dep-validate → publish);
- the `npm-smoke` job's matrix + step sequence (checkout → setup-node → syntax → deps → smoke.cjs →
  install.test.cjs[gated]);
- the version-derivation expression (`${GITHUB_REF#refs/tags/v}`);
- the README publish-notes content.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY .github/workflows/release.yml — ADD the `npm-publish` job + NPM_TOKEN comment
  - PRESERVE: the entire `goreleaser` job, the `on: push: tags: v*` trigger, `permissions: contents: write`,
    the existing secrets-header comment block. S3 ADDS a sibling job + extends the comment.
  - ADD to the secrets-header comment block (top of file): a line documenting NPM_TOKEN, e.g.:
        #   NPM_TOKEN                 npm AUTOMATION token (npmjs.com → Access Tokens → "Automation";
        #                             bypasses publish 2FA for non-interactive CI). Settings → Secrets → Actions.
  - ADD a new job `npm-publish` AFTER the `goreleaser` job:
      npm-publish:
        name: Publish npm wrapper
        needs: goreleaser              # GitHub Release (archives+checksums) must exist before users install
        runs-on: ubuntu-latest
        steps:
          - name: Checkout
            uses: actions/checkout@v4          # default fetch-depth 1 is fine (no changelog history needed)
          - name: Set up Node 20 + npm registry
            uses: actions/setup-node@v4
            with:
              node-version: '20'               # contract; satisfies package.json engines.node>=18
              registry-url: 'https://registry.npmjs.org'   # writes ~/.npmrc auth line (NODE_AUTH_TOKEN resolved at publish step)
          - name: Sync package.json version from tag
            # The committed npm/package.json version is the 0.0.0 dev placeholder; the RELEASE version is
            # the tag without leading v (goreleaser {{.Version}}). Bump it TRANSIENTLY (no commit/tag);
            # npm version --no-git-tag-version skips all git ops incl. the dirty-tree check.
            working-directory: npm
            run: |
              VERSION="${GITHUB_REF#refs/tags/v}"   # refs/tags/v1.2.0 -> 1.2.0 ; v1.2.0-rc.1 -> 1.2.0-rc.1
              echo "Publishing @dabstractor/stagecoach@$VERSION"
              npm version "$VERSION" --no-git-tag-version
          - name: Validate deps resolve (no postinstall, no lockfile)
            # --ignore-scripts: do NOT run postinstall (install.cjs would download the real binary — publish
            #   ships JS ONLY; the binary is fetched at USER install time).
            # --no-package-lock: do NOT create a lockfile (it would pollute the published tarball).
            working-directory: npm
            run: npm install --ignore-scripts --no-package-lock
          - name: Publish to npm
            # --access public: REQUIRED — @dabstractor/stagecoach is scoped (scoped packages default to
            #   private/restricted). NODE_AUTH_TOKEN is resolved from THIS step's env by the ~/.npmrc that
            #   setup-node wrote.
            working-directory: npm
            run: npm publish --access public
            env:
              NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
  - VALIDATE: python3 -c "import yaml; d=yaml.safe_load(open('.github/workflows/release.yml')); assert 'npm-publish' in d['jobs']; assert d['jobs']['npm-publish']['needs']=='goreleaser'; print('release.yml npm-publish OK')"

Task 2: MODIFY .github/workflows/ci.yml — ADD the `npm-smoke` job
  - PRESERVE: all existing jobs (build-test, lint, vulncheck, coverage), `permissions: contents: read`,
    the concurrency block. S3 ADDS a sibling `npm-smoke` job.
  - ADD a new job `npm-smoke` (mirrors build-test's matrix shape):
      # --- (5) npm wrapper smoke (PRD §21.2; S1 shim + S2 installer) -------------------
      # Catches npm-wrapper regressions on every PR before a release. Runs S1's npm/test/smoke.cjs (shim:
      # fake-cached binary -> exec + STAGECOACH_INSTALL_METHOD=npm + exit-code + fallback) and S2's
      # npm/test/install.test.cjs (installer: local fixture server -> download+SHA256-verify+extract +
      # mismatch-abort). smoke.cjs skips win32 (S1's contract: a fake .exe needs a real PE binary);
      # install.test.cjs is GATED to non-windows (S2's test builds a tar.gz even for .zip-named assets, so
      # the windows zip-extraction path is a documented gap pending a real-zip fixture).
      npm-smoke:
        name: npm wrapper smoke (${{ matrix.os }})
        runs-on: ${{ matrix.os }}
        timeout-minutes: 10
        strategy:
          fail-fast: false
          matrix:
            os: [ubuntu-latest, macos-latest, windows-latest]
        steps:
          - uses: actions/checkout@v4
          - uses: actions/setup-node@v4
            with:
              node-version: '20'
          - name: Shim + installer syntax check
            run: |
              node --check npm/bin/stagecoach.js
              node --check npm/install.cjs
          - name: Fetch wrapper deps (no postinstall)
            # tar + extract-zip for S2's install.test.cjs. --ignore-scripts so install.cjs does NOT run
            # (the 0.0.0 dev guard would also skip it, but be explicit).
            working-directory: npm
            run: npm install --ignore-scripts
          - name: Shim smoke (fake-cached binary)
            # S1's smoke: places a fake binary in a temp cache, runs the shim, asserts exec + env tag +
            # exit-code propagation + the --ignore-scripts fallback. SKIPS on win32 (prints 'skipped', exit 0).
            run: node npm/test/smoke.cjs
          - name: Installer smoke (fixture server; tar.gz path)
            # S2's smoke: builds a fixture archive + checksums, serves via local http, asserts download+
            # SHA256-verify+extract + checksum-mismatch abort-before-write. GATED to non-windows: S2's
            # fixture is tar.gz even when the asset name ends in .zip, so the windows zip path is uncovered.
            if: matrix.os != 'windows-latest'
            working-directory: npm
            run: node test/install.test.cjs
  - VALIDATE: python3 -c "import yaml; d=yaml.safe_load(open('.github/workflows/ci.yml')); assert 'npm-smoke' in d['jobs']; j=d['jobs']['npm-smoke']; assert [m for m in j['strategy']['matrix']['os']]==['ubuntu-latest','macos-latest','windows-latest']; print('ci.yml npm-smoke OK')"

Task 3: MODIFY npm/README.md — APPEND the `## Publishing (for maintainers)` section
  - PRESERVE: S1's H1 + one-paragraph user doc + `## Install` block (do NOT alter them).
  - APPEND at the END of npm/README.md (after S1's content):
      ## Publishing (for maintainers)

      This package is published **automatically** when a `v*` tag is pushed (the same tag that fires
      [goreleaser](https://github.com/dabstractor/stagecoach/blob/main/.github/workflows/release.yml)).
      goreleaser has no native npm pipe, so an `npm-publish` job (in `release.yml`) runs after the
      `goreleaser` job:

      1. Syncs `package.json` `version` to the tag (`v1.2.0` → `1.2.0`; the committed `version` stays
         `0.0.0`, a dev placeholder).
      2. `npm install --ignore-scripts --no-package-lock` — validates the `tar` / `extract-zip` deps
         resolve WITHOUT running `postinstall` (the native binary is fetched at *user* install time, not
         publish time) and WITHOUT writing a lockfile into the tarball.
      3. `npm publish --access public` — the package is scoped (`@dabstractor/...`), so `--access public`
         is required.

      **Required secret:** `NPM_TOKEN` — an npm **automation** token (npmjs.com → Access Tokens →
      "Automation"), which bypasses publish 2FA for non-interactive CI. Add it under the repo's
      Settings → Secrets → Actions. (A "Publish" token will fail in CI if the account has publish 2FA on.)

      The wrapper's smoke tests (the shim + the installer) run on every PR in
      [`ci.yml`](https://github.com/dabstractor/stagecoach/blob/main/.github/workflows/ci.yml) (`npm-smoke`
      job) to catch regressions before a release. See the main repo's
      [distribution notes](https://github.com/dabstractor/stagecoach#install) for all install channels.
  - VALIDATE: the section is present and S1's content is intact:
        grep -q "## Publishing (for maintainers)" npm/README.md && grep -q "## Install" npm/README.md && echo "README OK"

Task 4: VALIDATE — YAML parse, version round-trip, npm pack, smoke, scope
  - python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml')); yaml.safe_load(open('.github/workflows/ci.yml')); print('YAML OK')"
  - ORIG=$(node -p "require('./npm/package.json').version")   # 0.0.0
  - ( cd npm && npm version 1.2.3 --no-git-tag-version && npm pack --dry-run ) | grep -E 'npm notice (📦|[0-9]+\.?[0-9]*[Bk]?)' | head; \
        git checkout -- npm/package.json
  - node -p "require('./npm/package.json').version"           # back to 0.0.0 (proves committed file untouched)
  - node npm/test/smoke.cjs                                    # SMOKE PASS (unix)
  - ( cd npm && npm install --ignore-scripts && node test/install.test.cjs )   # INSTALL-TEST PASS (unix tar.gz)
  - git status --porcelain                                     # ONLY the 2 workflow files + npm/README.md
```

### Implementation Patterns & Key Details

```yaml
# PATTERN (the canonical npm-publish job — setup-node writes the auth .npmrc; NODE_AUTH_TOKEN on the step):
npm-publish:
  needs: goreleaser
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-node@v4
      with:
        node-version: '20'
        registry-url: 'https://registry.npmjs.org'   # <- writes ~/.npmrc auth line
    - name: Sync version from tag
      working-directory: npm
      run: npm version "${GITHUB_REF#refs/tags/v}" --no-git-tag-version
    - name: Validate deps (no postinstall, no lockfile)
      working-directory: npm
      run: npm install --ignore-scripts --no-package-lock
    - name: Publish
      working-directory: npm
      run: npm publish --access public
      env:
        NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}    # <- resolved by the ~/.npmrc setup-node wrote
# CRITICAL: the 3 "working-directory: npm" + the 2 "--ignore-scripts/--no-package-lock" flags + the
# NODE_AUTH_TOKEN on the PUBLISH step are the load-bearing details. Get all three right.

# PATTERN (version derivation): GITHUB_REF on a tag push == "refs/tags/v1.2.0".
#   ${GITHUB_REF#refs/tags/v} -> "1.2.0"  (strips the literal prefix "refs/tags/v").
#   For v1.2.0-rc.1 -> "1.2.0-rc.1" (valid semver; npm version accepts it).
# Do NOT use ${GITHUB_REF##*/} (-> "v1.2.0", keeps the v) — the npm version must NOT have the leading v
#   (it must equal goreleaser {{.Version}}). The "#refs/tags/v" strip removes both the path AND the v.

# PATTERN (the ci.yml matrix step gating):
- name: Installer smoke (tar.gz path only)
  if: matrix.os != 'windows-latest'        # <- the gate; keeps windows CI green (S2's tar.gz-fixture reality)
  working-directory: npm
  run: node test/install.test.cjs
```

### Integration Points

```yaml
CONSUMES (LANDED, READ-ONLY — S3 wires/publishes them):
  - npm/package.json (S1+S2): the package S3 publishes. version 0.0.0 committed; bumped transiently in CI.
  - npm/bin/stagecoach.js (S1), npm/install.cjs (S2): shipped as-is (publish packs them; postinstall runs at user time).
  - npm/test/smoke.cjs (S1), npm/test/install.test.cjs (S2): RUN by ci.yml's npm-smoke job.
  - release.yml `goreleaser` job: npm-publish `needs:` it (GitHub Release must precede user installs).
PRODUCES (consumed downstream / by maintainers):
  - The `npm-publish` job: makes `@dabstractor/stagecoach@<tag-version>` live on npm on every v* tag.
  - The `npm-smoke` job: turns PRs red if the wrapper (shim or installer) regresses.
NO Go/build/config/migration changes — S3 touches only 2 workflow files + npm/README.md. The committed
  npm/package.json is UNCHANGED by S3 (version sync is transient).
```

## Validation Loop

> **This is a CI-YAML + Markdown item, not a Go/Python one.** The template's `ruff`/`mypy`/`pytest`/`go
> test` gates DO NOT apply. Validation = YAML parse (python3+pyyaml, available here) + `npm pack --dry-run`
> (no auth) + `npm version` round-trip + the already-green S1/S2 smoke tests + a scope guard.
> Run commands from the repo root; `cd npm` / `working-directory: npm` where noted.
> **NOTE:** the full publish cannot be exercised locally (it needs a `v*` tag + `NPM_TOKEN`). The local
> gates below validate EVERYTHING EXCEPT the actual registry upload — the publish step's preconditions
> (version sync, tarball contents, deps resolve, scoped-access flag) are all provable without a token.

### Level 1: YAML + Markdown validity (Immediate Feedback)

```bash
# Both workflow files are valid YAML AND the new jobs/keys are present.
python3 -c "import yaml; d=yaml.safe_load(open('.github/workflows/release.yml')); assert 'npm-publish' in d['jobs'], 'missing npm-publish'; j=d['jobs']['npm-publish']; assert j.get('needs')=='goreleaser', 'needs!=goreleaser'; assert any(('npm publish' in (s.get('run',''))) for s in j['steps']), 'no publish step'; assert any((s.get('env',{}) or {}).get('NODE_AUTH_TOKEN')=='\${{ secrets.NPM_TOKEN }}' for s in j['steps']), 'no NODE_AUTH_TOKEN'; print('release.yml OK')"
# Expected: "release.yml OK", exit 0.

python3 -c "import yaml; d=yaml.safe_load(open('.github/workflows/ci.yml')); assert 'npm-smoke' in d['jobs'], 'missing npm-smoke'; j=d['jobs']['npm-smoke']; assert j['strategy']['matrix']['os']==['ubuntu-latest','macos-latest','windows-latest']; steps={s.get('name','') for s in j['steps']}; assert any('Installer smoke' in n for n in steps); assert any(s.get('if','')=='matrix.os != \'windows-latest\'' for s in j['steps'] if 'Installer' in s.get('name','')), 'missing windows gate'; print('ci.yml OK')"
# Expected: "ci.yml OK", exit 0.

# npm/README.md has BOTH the new publish section AND S1's preserved ## Install.
grep -q "## Publishing (for maintainers)" npm/README.md && grep -q "## Install" npm/README.md && grep -qi "NPM_TOKEN" npm/README.md && echo "README OK"
# Expected: "README OK", exit 0.
```

### Level 2: Version sync + tarball (the publish step's preconditions, no token needed)

```bash
# Version sync works in the real git tree; committed package.json is restored afterward.
ORIG=$(node -p "require('./npm/package.json').version")         # 0.0.0
( cd npm && npm version 1.2.3 --no-git-tag-version >/dev/null )
[ "$(node -p "require('./npm/package.json').version")" = "1.2.3" ] || { echo "FAIL: version not bumped"; exit 1; }
git checkout -- npm/package.json
[ "$(node -p "require('./npm/package.json').version")" = "$ORIG" ] || { echo "FAIL: committed package.json changed"; exit 1; }
echo "version round-trip OK (committed unchanged)"
# Expected: "version round-trip OK (committed unchanged)", exit 0.

# npm pack --dry-run shows EXACTLY the source files (no lockfile, no node_modules).
( cd npm && npm version 9.9.9 --no-git-tag-version >/dev/null && npm install --ignore-scripts --no-package-lock >/dev/null 2>&1 && npm pack --dry-run 2>&1 | grep -E 'npm notice (Tarball Contents|[0-9]+(\.[0-9]+)?[BkK]? )' ); git checkout -- npm/package.json
# Expected: a file list containing README.md, bin/stagecoach.js, install.cjs, package.json, test/*.cjs and
#   NOT containing package-lock.json or node_modules. (The dep-install uses --no-package-lock so no lockfile.)
# Belt-and-braces: assert NO package-lock.json would ship:
!( ( cd npm && npm version 9.9.9 --no-git-tag-version >/dev/null && npm install --ignore-scripts --no-package-lock >/dev/null 2>&1 && npm pack --dry-run 2>&1 | grep -q 'package-lock.json' ) ) && echo "no lockfile in tarball OK" || echo "FAIL: lockfile present"; git checkout -- npm/package.json
# Expected: "no lockfile in tarball OK", exit 0.
```

### Level 3: The wired smoke tests run green (Component Validation)

```bash
# S1's shim smoke (the contract's "node npm/bin/stagecoach.js --version against a fake-cached binary"):
node npm/test/smoke.cjs
# Expected: "SMOKE PASS", exit 0 (exec + env-tag + exit-42 + fallback sub-cases; skips cleanly on win32).

# S2's installer smoke (the tar.gz path — what ci.yml runs on ubuntu/macos):
( cd npm && npm install --ignore-scripts && node test/install.test.cjs )
# Expected: "INSTALL-TEST PASS", exit 0 (happy-path extract + checksum-mismatch abort).
```

### Level 4: Scope guard

```bash
# ONLY the 2 modified workflow files + the modified npm/README.md.
git status --porcelain
# Expected: " M .github/workflows/ci.yml"  " M .github/workflows/release.yml"  " M npm/README.md"  (nothing else).
git status --porcelain | grep -vE '^ ?M \.github/workflows/(ci|release)\.yml$|^ ?M npm/README\.md$' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
# Expected: "OK: scope clean".
git status --porcelain | grep -E 'npm/(bin|install|test|package\.json)|\.goreleaser\.yaml|internal/|cmd/|PRD\.md|plan/|tasks\.json|\.gitignore|go\.(mod|sum)|Makefile' && echo "FAIL: forbidden file" || echo "OK: no forbidden files"
# Expected: "OK: no forbidden files" (npm/bin/*, npm/install.cjs, npm/test/*, npm/package.json committed,
#   .goreleaser.yaml, Go, PRD/plan/tasks are all UNTOUCHED).
```

## Final Validation Checklist

### Technical Validation
- [ ] `release.yml` parses as YAML; `npm-publish` job present with `needs: goreleaser` + NODE_AUTH_TOKEN on the publish step (Level 1)
- [ ] `ci.yml` parses as YAML; `npm-smoke` job present with the 3-OS matrix + the `if: matrix.os != 'windows-latest'` gate on install.test.cjs (Level 1)
- [ ] `npm/README.md` has the `## Publishing (for maintainers)` section AND S1's `## Install` is intact (Level 1)
- [ ] `npm version` round-trip bumps + restores the committed package.json to `0.0.0` (Level 2)
- [ ] `npm pack --dry-run` (after `--no-package-lock` dep-install) shows NO `package-lock.json` / NO `node_modules` (Level 2)
- [ ] `node npm/test/smoke.cjs` → `SMOKE PASS`; `cd npm && node test/install.test.cjs` → `INSTALL-TEST PASS` (Level 3)

### Feature Validation
- [ ] The publish job uses `actions/setup-node@v4` with `node-version: '20'` + `registry-url: 'https://registry.npmjs.org'`
- [ ] The publish job syncs version via `npm version "${GITHUB_REF#refs/tags/v}" --no-git-tag-version` (transient; no commit/tag)
- [ ] The publish job runs `npm install --ignore-scripts --no-package-lock` (no postinstall binary download; no lockfile)
- [ ] The publish step is `npm publish --access public` with `env.NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}`
- [ ] The release.yml comment block documents `NPM_TOKEN` as an npm **automation** token (bypasses publish 2FA)
- [ ] The ci.yml npm-smoke job runs S1's smoke.cjs + S2's install.test.cjs (the latter gated to non-windows)
- [ ] ci.yml comments explain the windows install.test.cjs gate (S2's tar.gz-fixture reality) + the smoke.cjs win32 skip

### Scope-Boundary Validation
- [ ] `git status --porcelain` shows ONLY `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `npm/README.md`
- [ ] NO committed change to `npm/package.json` (version stays `0.0.0`; the round-trip restores it)
- [ ] NO edit to `npm/bin/stagecoach.js`, `npm/install.cjs`, `npm/test/*` (S1/S2's contracts)
- [ ] NO Go files, `.goreleaser.yaml`, Makefile, go.mod/sum, `.gitignore`, PRD.md, plan/**, tasks.json touched
- [ ] The `goreleaser` job in release.yml is UNCHANGED (S3 only ADDS a sibling job)

### Documentation & Deployment
- [ ] release.yml inline comments: NPM_TOKEN secret (automation token) + the version-sync + dep-validation rationale
- [ ] ci.yml inline comments: what smoke.cjs + install.test.cjs assert + the windows gate reason
- [ ] npm/README.md publish notes: the 3-step publish flow + the NPM_TOKEN requirement + the smoke job pointer
- [ ] No committed version drift (package.json stays `0.0.0`; the README notes this is the dev placeholder)

---

## Anti-Patterns to Avoid

- ❌ Don't run `npm install` WITHOUT `--ignore-scripts` in the publish job — it runs `postinstall`
  (`install.cjs`), which downloads the REAL binary for the synced version. Publish ships JS ONLY; the
  binary fetch is a USER-install-time concern. Use `--ignore-scripts`.
- ❌ Don't run `npm install` WITHOUT `--no-package-lock` in the publish job — it creates `package-lock.json`,
  which `npm pack`/`npm publish` INCLUDES in the tarball (verified: 8KB lockfile ships). Use `--no-package-lock`.
- ❌ Don't omit `--access public` — `@dabstractor/stagecoach` is scoped and defaults to PRIVATE/restricted;
  a bare `npm publish` is rejected (or publishes privately). `--access public` is mandatory for scoped.
- ❌ Don't set `NODE_AUTH_TOKEN` only at the job `env:` level and assume — set it on the PUBLISH STEP's `env:`
  (the canonical pattern from the official doc). setup-node's ~/.npmrc references `${NODE_AUTH_TOKEN}`,
  resolved from the step env.
- ❌ Don't derive the version with `${GITHUB_REF##*/}` (→ `v1.2.0`, keeps the leading v) — npm version must
  equal goreleaser `{{.Version}}` (tag WITHOUT v). Use `${GITHUB_REF#refs/tags/v}` (→ `1.2.0`).
- ❌ Don't COMMIT a bumped `npm/package.json` — the `0.0.0` is the committed dev placeholder; the bump is
  TRANSIENT in the runner (`--no-git-tag-version`, no commit/tag). S3 makes no committed package.json change.
- ❌ Don't run `npm test/install.test.cjs` on `windows-latest` — S2's test builds a tar.gz even for
  `.zip`-named assets, so `extract-zip` on a tar.gz THROWS and the happy-path assertion FAILS (red CI).
  Gate it `if: matrix.os != 'windows-latest'`. Document the windows zip-path gap.
- ❌ Don't drop `needs: goreleaser` — without it the jobs run concurrently and npm could publish BEFORE the
  GitHub Release (archives+checksums) exists, 404'ing early installers. `needs:` also fail-fasts if
  goreleaser failed.
- ❌ Don't use a "Publish" npm token for `NPM_TOKEN` — if the account has publish 2FA on, it fails in CI
  ("requires a one-time password"). Use an **Automation** token (bypasses 2FA for non-interactive CI).
- ❌ Don't add `fetch-depth: 0` to the npm-publish checkout — only the goreleaser job needs full history
  (changelog). npm-publish reads package.json + the tag name; default fetch-depth 1 is correct + faster.
- ❌ Don't add a `files` field to package.json to slim the tarball — that MODIFIES S1/S2's package.json
  (out of scope). The `test/*` files in the tarball are harmless (~9KB); note it as optional in the README.
- ❌ Don't edit `npm/bin/stagecoach.js`, `npm/install.cjs`, or `npm/test/*` — those are S1/S2's contracts.
  S3 only RUNS the tests in ci.yml and PUBLISHES the package.
- ❌ Don't gate the smoke on a single platform "for speed" — the matrix (ubuntu/macos/windows) catches
  OS-specific regressions (the win32→windows map, deps-resolve on each OS). Windows adds value even though
  smoke.cjs skips and install.test.cjs is gated (syntax check + deps-resolve still run there).

---

## Confidence Score

**One-pass success likelihood: 9/10.** Every load-bearing mechanism is VERIFIED locally in this repo's
toolchain: `npm version X --no-git-tag-version` works in the real git tree (twice in a row — no dirty-tree
refusal); the tag→version strip (`${GITHUB_REF#refs/tags/v}` → `1.2.0` / `1.2.0-rc.1`) is confirmed;
`npm install --ignore-scripts --no-package-lock` produces a clean `npm pack` (6 source files, NO lockfile,
NO node_modules); the setup-node+NODE_AUTH_TOKEN pattern is the official GitHub doc's canonical form; the
`--access public` requirement for scoped packages is npm-documented; the `needs: goreleaser` ordering is
unambiguous. The single residual risk is the **windows `install.test.cjs` gate**: S2's test builds a
tar.gz even for `.zip`-named assets (read directly from the LANDED file), so it cannot validate the
windows zip-extraction path — the gate keeps CI green and the gap is documented (pending a real-zip
fixture from S2 / a future task). The actual `npm publish` upload can only be exercised by a real tag +
NPM_TOKEN (not locally), but every publish PRECONDITION (version, tarball contents, deps, scoped-access,
auth wiring) is provable without a token, so the publish step itself is low-risk. No Go/release-config/
scope surprises remain.