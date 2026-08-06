name: "P2.M1.T1.S1 — npm package.json + bin/stagecoach.js shim (STAGECOACH_INSTALL_METHOD=npm, exec cached binary)"
description: >
  The first of P2.M1.T1's three subtasks: the npm-wrapper PACKAGE + SHIM for `@dabstractor/stagecoach`
  (PRD §21.2 / §9.29 FR-U2; the esbuild/turbo/prisma pattern — external_deps.md §3). Creates a new `npm/`
  subdir (NOT the Go module root) with: (a) `npm/package.json` — name `@dabstractor/stagecoach`, version
  synced to the Go release tag, `bin.stagecoach = ./bin/stagecoach.js`, `scripts.postinstall = node install.cjs`
  (install.cjs is S2's deliverable — S1 DECLARES it, does NOT create it), `engines.node >=18`, MIT license
  (mirrors goreleaser), github repo/homepage/bugs metadata; (b) `npm/bin/stagecoach.js` — a zero-dependency
  Node CommonJS shim that (1) sets `process.env.STAGECOACH_INSTALL_METHOD='npm'` (FR-U2 — the explicit-
  override tier the LANDED Go self-update detection cascade reads, so `stagecoach upgrade` DELEGATES to
  `npm install -g @dabstractor/stagecoach@latest` instead of self-swapping), (2) resolves the cached native
  binary path for THIS wrapper version + platform (versioned cache at
  `STAGECOACH_CACHE_DIR || ~/.stagecoach/versions/<v>/<goos>-<goarch>/stagecoach[.exe]`), (3) if absent prints
  the verbatim --ignore-scripts fallback message to stderr + exit 1, (4) else `child_process.spawnFileSync`
  the cached binary with `process.argv.slice(2)`, `{stdio:'inherit', env:process.env}`, and exit its code;
  (c) `npm/README.md` — one Mode-A paragraph (what the wrapper is + the --ignore-scripts fallback) with an
  `## Install` heading so the fallback URL's `#install` anchor resolves; (d) `npm/test/smoke.cjs` — a cross-
  platform smoke test that places a fake binary in a temp `STAGECOACH_CACHE_DIR`, runs the shim, and asserts
  the fake was exec'd AND `STAGECOACH_INSTALL_METHOD=npm` was set on the child env + exit-code propagation.
  NO Go code, NO install.cjs (S2), NO CI wiring (S3). Validates via `node --check`, `node npm/test/smoke.cjs`,
  JSON parse, and a scope guard. NOTE: the task's `prd_selectors` (§9.26 work-description mode) is a MISMATCH —
  the authoritative spec is external_deps.md §3 + the contract + .goreleaser.yaml (asset names).

---

## Goal

**Feature Goal**: Ship the npm-distribution PACKAGE + SHIM so that `npm install -g @dabstractor/stagecoach`
produces a working `stagecoach` command that runs the matching prebuilt native binary, tags every invocation
with `STAGECOACH_INSTALL_METHOD=npm` (so `stagecoach upgrade` delegates instead of self-swapping — FR-U2/U3),
and fails with a clear, actionable message when postinstall was blocked (`--ignore-scripts`).

**Deliverable**: Four new files under a new `npm/` subdir:
- `npm/package.json` — the wrapper manifest (name, version, bin, postinstall, engines, license, repo metadata).
- `npm/bin/stagecoach.js` — the zero-dependency Node CJS shim (env tag → resolve cached binary → spawn or fallback).
- `npm/README.md` — one-paragraph Mode-A doc (wrapper purpose + `--ignore-scripts` fallback) + an `## Install` heading.
- `npm/test/smoke.cjs` — the self-validating smoke test (fake binary → exec + env-tag assertion + exit-code propagation).

**Success Definition**:
- `node --check npm/bin/stagecoach.js && node --check npm/test/smoke.cjs` exit 0 (syntax clean).
- `node -e "JSON.parse(require('fs').readFileSync('npm/package.json','utf8'))"` exits 0 (valid JSON; `bin`,
  `scripts.postinstall`, `engines.node`, `name`, `version`, `license` all present).
- `node npm/test/smoke.cjs` PASSes: prints `SMOKE-OK` + `install_method=npm` (proves the shim exec'd the fake
  AND set `STAGECOACH_INSTALL_METHOD=npm` on the child env), and the exit-42 sub-case propagates exit 42.
- Manual: with no cached binary present, `node npm/bin/stagecoach.js --version` prints the verbatim fallback
  message to stderr and exits 1.
- `git status --porcelain` shows ONLY the four `npm/*` files (no Go, no .goreleaser.yaml, no install.cjs, no plan/PRD).

## User Persona (if applicable)

**Target User**: A developer who prefers npm-managed CLIs (`npm install -g @dabstractor/stagecoach`) over
Homebrew/Scoop/direct-binary install — the Node/JS-tooling persona (cf. how they install prettier/esbuild/turbo).
**Use Case**: `npm install -g @dabstractor/stagecoach && stagecoach` — npm's postinstall (S2) fetches the
matching native binary into the cache, and the shim (S1) execs it on every `stagecoach` invocation. When they
later run `stagecoach upgrade`, the npm-set env tag makes stagecoach delegate to
`npm install -g @dabstractor/stagecoach@latest` (FR-U3) instead of self-swapping the cached binary.
**Pain Points Addressed**: (1) "I manage all my CLIs via npm" — now stagecoach is installable that way;
(2) the `--ignore-scripts` / corporate-npm trap (postinstall blocked → silent broken install) → the shim
prints a clear fallback pointing at the direct install URL; (3) self-update correctness — an npm install must
NOT be self-swapped in place (the cache would diverge from the npm package version); the env tag prevents that.

## Why

- **PRD §21.2 / §9.29 FR-U2**: npm is a first-class distribution channel for v3.0; the wrapper is the
  esbuild/turbo/prisma single-package + postinstall-download pattern (external_deps.md §3 — verified). S1 is
  the PACKAGE + SHIM half (S2 = the postinstall downloader; S3 = CI publish + version-sync).
- **FR-U2 wire**: the shim's `process.env.STAGECOACH_INSTALL_METHOD='npm'` is consumed by the ALREADY-LANDED
  install-method detection cascade (`internal/upgrade/detect.go`, P1.M2.T1.S1 Complete) — without it, an npm
  install would be misdetected as `direct` and `stagecoach upgrade` would self-swap the cached binary,
  diverging it from the npm package version (a real correctness bug). The env tag is the cheap, robust fix.
- **Scope discipline**: S1 ships ONLY the wrapper package + shim + doc + smoke (no Go, no install.cjs, no CI).
  This keeps the Node distribution surface reviewable in isolation and lets S2/S3 build on a stable package.json
  contract.

## What

New `npm/` subdir containing the wrapper package.json, the exec shim, a one-paragraph README, and a smoke test.
No Go code. No `install.cjs` (S2). No `.github/workflows` wiring (S3).

### Success Criteria
- [ ] `npm/package.json`: `name=@dabstractor/stagecoach`; `version` present (dev placeholder, S3 bumps);
      `bin={stagecoach:./bin/stagecoach.js}`; `scripts.postinstall=node install.cjs`; `engines.node=>=18`;
      `license=MIT`; `repository`/`homepage`/`bugs` point at `github.com/dabstractor/stagecoach`.
- [ ] `npm/bin/stagecoach.js`: `#!/usr/bin/env node` + `'use strict'`; zero deps (only `node:` builtins);
      sets `process.env.STAGECOACH_INSTALL_METHOD='npm'`; resolves
      `path.join(process.env.STAGECOACH_CACHE_DIR || path.join(os.homedir(),'.stagecoach','versions'), version, `${goos}-${goarch}`, binaryName)`
      (goos/goarch/binaryName from the Node→goreleaser map); absent → verbatim fallback message to stderr + `exit 1`;
      present → `spawnFileSync(cachedBin, process.argv.slice(2), {stdio:'inherit', env:process.env})` + `exit(status ?? 1)`.
- [ ] `npm/README.md`: one paragraph (wrapper purpose + `--ignore-scripts` fallback) + an `## Install` heading
      (so the fallback URL `#install` resolves).
- [ ] `npm/test/smoke.cjs`: places a fake binary in a temp `STAGECOACH_CACHE_DIR`, runs the shim, asserts the
      fake exec'd (`SMOKE-OK`) AND `install_method=npm` was set; asserts exit-code propagation (fake `exit 42`
      → shim exits 42). Skips cleanly on `win32` (S3/CI owns the Windows stub).
- [ ] `node --check`, `node npm/test/smoke.cjs`, JSON parse all pass; scope guard shows only `npm/*`.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim asset-name map (goreleaser + Go `assetName` confirmed), the exact cache-path scheme +
why home-dir beats package-dir, the verbatim fallback message, the verbatim env-tag wire, every package.json
field with provenance, the smoke-test algorithm with the win32-skip rationale, the Node stdlib confirmation,
and the S1/S2/S3 scope boundary (install.cjs is S2's; CI is S3's).

### Documentation & References

```yaml
# MUST READ — the codebase-specific findings for THIS item (the cache-path decision + asset map + env wire).
- docfile: plan/017_397abce9deb1/P2M1T1S1/research/findings.md
  why: "§0 PRD-selector mismatch (resolved — use external_deps.md §3, NOT §9.26); §1 the authoritative wrapper
        spec; §2 the EXACT Node→goreleaser asset/binary map (verified vs download.go assetName); §3 the
        cache-path DECISION (home-dir versioned + STAGECOACH_CACHE_DIR override — S2 MUST match); §4 the
        STAGECOACH_INSTALL_METHOD=FR-U2 wire (consumed by LANDED detect.go); §5 verbatim fallback message;
        §6 EVERY package.json field w/ provenance + the no-LICENSE-file flag; §7 README shape; §8 smoke-test
        algorithm + win32-skip; §9 Node stdlib; §10 scope; §11 validation (node, not go)."

# MUST READ — the authoritative wrapper spec (the esbuild/turbo/prisma pattern).
- docfile: plan/017_397abce9deb1/architecture/external_deps.md
  section: "§3 npm wrapper (single-package + postinstall-download form; the shim sets STAGECOACH_INSTALL_METHOD
            + execs the cached binary; the --ignore-scripts fallback); §7 install-method detection signals
            (STAGECOACH_INSTALL_METHOD = explicit-override tier 7a)."
  why: "The PRIMARY spec. §3 mandates: bin={stagecoach:./bin/stagecoach.js}; postinstall=node install.cjs;
        the shim sets STAGECOACH_INSTALL_METHOD='npm' then child_process.spawn the cached binary forwarding
        argv/stdio + preserving exit code; on --ignore-scripts the binary is absent → print a fallback. §7
        confirms the env var is the explicit-override tier the detection cascade reads first."
  critical: "§3 says the PRD specifies the SINGLE-PACKAGE + postinstall-download form — NOT optionalDependencies.
             Do not add per-platform optionalDependencies packages."

# MUST READ — the goreleaser asset names (the JS map MUST produce these EXACT strings).
- file: .goreleaser.yaml
  why: "archives.name_template='{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}' → stagecoach_<v>_<os>_<arch>;
        formats=[tar.gz], format_overrides windows→zip; checksum.name_template='stagecoach_<v>_checksums.txt';
        builds.binary='stagecoach' (stagecoach.exe on windows); release.github={owner:dabstractor, name:stagecoach}."
  pattern: "The 6 targets: linux|darwin|windows × amd64|arm64. Binary inside archive: stagecoach (unix) /
            stagecoach.exe (windows). S1's cache path uses goos-goarch keys matching these goreleaser values."

# MUST READ — the Go-side assetName (PROVES the JS map matches the goreleaser output the Go downloader selects).
- file: internal/upgrade/download.go
  why: "L57 assetName(tag,goos,goarch): TrimPrefix(tag,'v') + 'stagecoach_'+v+'_'+goos+'_'+goarch + '.zip'(windows)/
        '.tar.gz'(else). L73 checksumsName: 'stagecoach_'+TrimPrefix(tag,'v')+'_checksums.txt'. The JS side must
        map Node platform/arch to the SAME goos/goarch values so the cache path S2 writes == the path S1 resolves."
  pattern: "goos = process.platform==='win32' ? 'windows' : process.platform; goarch = process.arch==='x64' ?
            'amd64' : process.arch. binaryName = process.platform==='win32' ? 'stagecoach.exe' : 'stagecoach'."

# MUST READ — the FR-U2 consumer (ALREADY LANDED — the env var's whole purpose).
- file: internal/upgrade/detect.go
  why: "The install-method detection cascade. Override = STAGECOACH_INSTALL_METHOD (tier 7a, external_deps §7) is
        read FIRST. The npm value routes `stagecoach upgrade` to delegation (FR-U3 npm row →
        `npm install -g @dabstractor/stagecoach@latest`), NOT a self-swap. So the shim MUST set the env var on
        every invocation (it's how upgrade correctness for npm installs is achieved). READ-ONLY — S1 adds no Go."

# CONTEXT — README tone to mirror for npm/README.md's one paragraph.
- file: README.md
  why: "L82 '## Install' heading (the anchor the fallback URL #install must resolve to — but that's the MAIN repo
        README; npm/README.md ALSO needs its own ## Install heading so the npm package page's #install works).
        goreleaser description tone: 'Snapshot-based AI commit message generator that uses YOUR local CLI agent'."

# CONTEXT — .gitignore (confirms npm/ node_modules won't pollute git).
- file: .gitignore
  why: "L13 '/node_modules/' already ignored — npm/ can have a dev node_modules without git noise. The npm/ SOURCE
        (package.json, bin/, test/, README.md) IS committed."

# OUT OF SCOPE — install.cjs (S2's deliverable) + CI publish (S3's). Do NOT create either.
- file: (npm/install.cjs — DOES NOT EXIST YET; S2 creates it)
  why: "S1's package.json DECLARES scripts.postinstall='node install.cjs' but S1 does NOT create install.cjs (that
        is S2's contract — P2.M1.T1.S2). Until S2 lands, `npm install` in npm/ errors on the missing postinstall
        script — EXPECTED. S1 validates via `node bin/stagecoach.js` directly. Do NOT ship a stub install.cjs."
- out_of_scope: ".github/workflows/* (S3 — P2.M1.T1.S3 owns `npm publish` on tag + version-sync + the full
                 cross-platform smoke matrix)."
```

### Current Codebase tree (relevant slice)

```bash
# READ-ONLY references (do NOT edit):
.goreleaser.yaml                        # READ-ONLY — asset-name templates (stagecoach_<v>_<os>_<arch>)
internal/upgrade/download.go            # READ-ONLY — assetName()/checksumsName() (the map the JS must match)
internal/upgrade/detect.go              # READ-ONLY — the FR-U2 consumer (STAGECOACH_INSTALL_METHOD cascade)
README.md                               # READ-ONLY — tone mirror + the #install-anchor convention
.gitignore                              # READ-ONLY — confirms /node_modules/ already ignored
plan/017_397abce9deb1/architecture/external_deps.md  # READ-ONLY — §3 the wrapper spec, §7 the detection signals
# (no existing npm/, package.json, or *.js in the repo — S1 is the first Node code)
```

### Desired Codebase tree with files to be added

```bash
npm/                       # NEW subdir (keeps the wrapper out of the Go module root)
  package.json             # NEW — @dabstractor/stagecoach manifest (bin, postinstall, engines, license, repo)
  bin/
    stagecoach.js          # NEW — zero-dep Node CJS shim (env tag → resolve cached binary → spawn | fallback)
  README.md                # NEW — one-paragraph Mode-A doc + ## Install heading (#install anchor)
  test/
    smoke.cjs              # NEW — self-validating smoke test (fake binary → exec + env-tag + exit-code)
# NOT created by S1: npm/install.cjs (S2), .github/workflows npm-publish step (S3).
```

### Known Gotchas of our codebase & Library Quirks

```javascript
// CRITICAL (the env tag MUST propagate to the child): set process.env.STAGECOACH_INSTALL_METHOD='npm' FIRST,
// then spawn with { env: process.env } (NOT a local var). The contract's exact { stdio:'inherit', env:process.env }
// is what lets the native child inherit the npm tag so `stagecoach upgrade` delegates (FR-U2/U3). Omitting
// env:process.env still inherits process.env by default in spawnFileSync, but set it EXPLICITLY per the contract.

// CRITICAL (cache-path scheme S1 + S2 MUST share — home-dir versioned, env-overridable):
//   cacheRoot = process.env.STAGECOACH_CACHE_DIR || path.join(os.homedir(), '.stagecoach', 'versions')
//   cachedBin = path.join(cacheRoot, version, `${goos}-${goarch}`, binaryName)
// Home-dir (NOT package-dir) because a global root-owned install's node_modules is UNWRITABLE for postinstall;
// the home dir always is. STAGECOACH_CACHE_DIR is REQUIRED for the smoke test (else it pollutes ~/.stagecoach).
// S2's install.cjs MUST write to the identical path or the shim won't find the binary.

// CRITICAL (Node→goreleaser map — must match download.go assetName EXACTLY):
//   goos = process.platform==='win32' ? 'windows' : process.platform
//   goarch = process.arch==='x64' ? 'amd64' : process.arch
//   binaryName = process.platform==='win32' ? 'stagecoach.exe' : 'stagecoach'
// Do NOT add other platform/arch values — goreleaser builds only linux|darwin|windows × amd64|arm64.

// CRITICAL (spawnFileSync exit-code propagation): result = spawnFileSync(...). If result.error → spawn failure
// (EACCES on a non-executable file, ENOENT race) → print + exit 1. Else process.exit(result.status ?? 1):
// status is null when the child was killed by a SIGNAL (unix) → exit 1 (conventional fallback). Do NOT use
// execSync (it buffers stdout — breaks --edit/interactive/progress TTY streaming); spawnFileSync+stdio:'inherit'
// is the correct stream-through choice.

// GOTCHA (no "type":"module" in package.json): leaving "type" unset defaults to CommonJS, so the .js shim uses
// require() — correct. Adding "type":"module" would make the CJS shim fail (SyntaxError on require). install.cjs
// is CJS-by-extension regardless. Do NOT add os/cpu arrays (single package, runtime detection).

// GOTCHA (install.cjs is S2's — S1 declares the postinstall but must NOT create the file): package.json has
// scripts.postinstall='node install.cjs' but install.cjs arrives in P2.M1.T1.S2. Between S1 and S2, `npm install`
// in npm/ errors on the missing script — EXPECTED and fine (S1 validates via `node bin/stagecoach.js`). Shipping
// a stub install.cjs would collide with S2's contract.

// GOTCHA (the fallback URL #install anchor): the verbatim fallback message ends with
// https://github.com/dabstractor/stagecoach#install — that anchor resolves to the MAIN repo README's ## Install.
// npm/README.md ALSO needs its own ## Install heading (the npm package page renders npm/README.md).

// GOTCHA (no LICENSE file in the repo): package.json license="MIT" mirrors .goreleaser.yaml's MIT assumption
// (L84/98/114). The repo has NO LICENSE file (verified) — flag it; do NOT create one in S1 (out of scope). If the
// repo's real license differs, package.json + .goreleaser.yaml must be reconciled together (a docs/legal task).

// GOTCHA (version field is a dev placeholder): package.json version="0.0.0" is NEVER published — S3/CI bumps it
// to match the Go release tag (e.g. "1.0.0") at `npm publish` time. The smoke test reads version from package.json
// and builds the cache path from it, so "0.0.0" works as a path segment during dev. Do NOT hand-set a real version.
```

## Implementation Blueprint

### Data models and structure

No data models — this is a Node wrapper (JSON manifest + two .js/.cjs scripts + a README). The "data" is:
- the package.json field set (§6 of findings, every field with provenance);
- the cache-path scheme (`STAGECOACH_CACHE_DIR || ~/.stagecoach/versions/<v>/<goos>-<goarch>/<binary>`);
- the Node→goreleaser platform map.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE npm/package.json — the wrapper manifest
  - FIELDS (exact):
      name: "@dabstractor/stagecoach"
      version: "0.0.0"                          # dev placeholder; S3/CI bumps to match the Go tag at publish
      description: "Thin npm wrapper that downloads and runs the stagecoach native binary — a snapshot-based
                    AI commit-message generator that uses YOUR local CLI agent."   # Mode A doc (contract DOCS)
      bin: { "stagecoach": "./bin/stagecoach.js" }
      scripts: { "postinstall": "node install.cjs" }   # install.cjs is S2's deliverable — DECLARED, not created
      engines: { "node": ">=18" }               # Node 18 LTS baseline (S2's https/fetch floor; S1 needs less)
      license: "MIT"                            # mirrors .goreleaser.yaml — FLAG: no LICENSE file exists yet
      repository: { type: "git", url: "https://github.com/dabstractor/stagecoach.git" }
      homepage: "https://github.com/dabstractor/stagecoach#readme"
      bugs: { url: "https://github.com/dabstractor/stagecoach/issues" }
      keywords: ["commit", "git", "ai", "commit-message", "cli"]
  - DO NOT add: "type" (default commonjs is correct), "os"/"cpu" (single package, runtime detection),
    "optionalDependencies" (§3 mandates single-package + postinstall-download, NOT per-platform packages).
  - VALIDATE: `node -e "JSON.parse(require('fs').readFileSync('npm/package.json','utf8'))"` exits 0.

Task 2: CREATE npm/bin/stagecoach.js — the zero-dependency exec shim
  - LINE 1: `#!/usr/bin/env node` (shebang — npm sets +x on bin entries at install; the shebang is mandatory).
  - LINE 2: `'use strict';`
  - IMPORTS (node: builtins ONLY, no deps): child_process.spawnFileSync, fs.existsSync, os.homedir, path.join.
  - BODY (exact order — the env tag FIRST so it propagates):
      // (a) FR-U2: tag every invocation so `stagecoach upgrade` detects npm + delegates (FR-U3).
      process.env.STAGECOACH_INSTALL_METHOD = 'npm';
      const pkg = require('../package.json');   // version from the wrapper's own manifest
      // (b) Node → goreleaser map (must match internal/upgrade/download.go assetName EXACTLY).
      const goos = process.platform === 'win32' ? 'windows' : process.platform;
      const goarch = process.arch === 'x64' ? 'amd64' : process.arch;
      const binaryName = process.platform === 'win32' ? 'stagecoach.exe' : 'stagecoach';
      const cacheRoot = process.env.STAGECOACH_CACHE_DIR || path.join(os.homedir(), '.stagecoach', 'versions');
      const cachedBin = path.join(cacheRoot, pkg.version, `${goos}-${goarch}`, binaryName);
      // (c) absent → verbatim --ignore-scripts fallback to stderr + exit 1.
      if (!fs.existsSync(cachedBin)) {
        process.stderr.write(
          'stagecoach native binary not installed (npm --ignore-scripts or postinstall blocked). ' +
          'Install directly: https://github.com/dabstractor/stagecoach#install\n'
        );
        process.exit(1);
      }
      // (d) present → exec the cached binary, forwarding argv + inheriting stdio + env; exit its code.
      const result = spawnFileSync(cachedBin, process.argv.slice(2), { stdio: 'inherit', env: process.env });
      if (result.error) {                                   // spawn failure (EACCES/ENOENT) — NOT the child's exit
        process.stderr.write('stagecoach: failed to launch native binary: ' + result.error.message + '\n');
        process.exit(1);
      }
      process.exit(typeof result.status === 'number' ? result.status : 1);   // null status (signal) → exit 1
  - NAMING/PLACEMENT: npm/bin/stagecoach.js (matches package.json bin path). No deps.
  - VALIDATE: `node --check npm/bin/stagecoach.js` exits 0.

Task 3: CREATE npm/README.md — one-paragraph Mode-A doc + ## Install heading
  - STRUCTURE:
      # @dabstractor/stagecoach   (or "stagecoach" — the npm package name as the H1)
      <ONE paragraph>: what the wrapper is (a thin npm package that downloads the stagecoach native binary at
      install time and execs it on every invocation; stagecoach is a snapshot-based AI commit-message generator
      that uses YOUR local CLI agent) + the --ignore-scripts fallback (if postinstall is blocked, the shim
      prints a message pointing at https://github.com/dabstractor/stagecoach#install; install directly or use
      a manager that allows postinstall).
      ## Install
      ```sh
      npm install -g @dabstractor/stagecoach
      stagecoach
      ```
      <optional one-liner>: see the main repo for features, config, and the full CLI reference.
  - CRITICAL: the `## Install` heading makes the fallback URL's #install anchor resolve on the npm page.
  - Keep it ONE paragraph + the install snippet (npm/README is a wrapper page, not a re-doc of stagecoach).

Task 4: CREATE npm/test/smoke.cjs — the self-validating smoke test
  - STRUCTURE (Node CommonJS, no deps; cross-platform with a win32 skip):
      const { spawnFileSync } = require('node:child_process');
      const fs = require('node:fs'); const os = require('node:os'); const path = require('node:path');
      const pkg = require('../package.json');
      // WIN32 SKIP: spawnFileSync to a .exe needs a real PE binary; a .cmd won't run without shell:true (which
      // the shim omits for argv fidelity). S3/CI owns the Windows stub. S1 proves the platform-independent LOGIC.
      if (process.platform === 'win32') { console.log('smoke: skipped on win32 (S3/CI adds a Windows stub)'); process.exit(0); }
      function assert(cond, msg) { if (!cond) { console.error('SMOKE FAIL: ' + msg); process.exit(1); } }
      // 1. temp cache root
      const cacheRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'sc-smoke-'));
      const goos = process.platform; const goarch = process.arch === 'x64' ? 'amd64' : process.arch;  // unix: platform==goos
      const binDir = path.join(cacheRoot, pkg.version, `${goos}-${goarch}`);
      fs.mkdirSync(binDir, { recursive: true });
      const cachedBin = path.join(binDir, 'stagecoach');
      // 2. fake binary that echoes the env tag + its args
      fs.writeFileSync(cachedBin, '#!/bin/sh\necho "SMOKE-OK install_method=$STAGECOACH_INSTALL_METHOD"\n', { mode: 0o755 });
      fs.chmodSync(cachedBin, 0o755);
      // 3. run the shim with the redirected cache + capture stdout
      const r = spawnFileSync(process.execPath, [path.join(__dirname, '..', 'bin', 'stagecoach.js'), '--version'],
                              { stdio: ['ignore', 'pipe', 'pipe'], env: { ...process.env, STAGECOACH_CACHE_DIR: cacheRoot } });
      const out = r.stdout.toString();
      assert(out.includes('SMOKE-OK'), 'shim did not exec the fake binary; stderr=' + r.stderr.toString());
      assert(out.includes('install_method=npm'), 'STAGECOACH_INSTALL_METHOD not set to npm on the child env');
      // 4. exit-code propagation: a fake that exits 42 → shim exits 42
      fs.writeFileSync(cachedBin, '#!/bin/sh\nexit 42\n', { mode: 0o755 }); fs.chmodSync(cachedBin, 0o755);
      const r2 = spawnFileSync(process.execPath, [path.join(__dirname, '..', 'bin', 'stagecoach.js')],
                               { stdio: ['ignore', 'pipe', 'pipe'], env: { ...process.env, STAGECOACH_CACHE_DIR: cacheRoot } });
      assert(r2.status === 42, 'exit-code propagation failed: shim exited ' + r2.status + ' (want 42)');
      // 5. fallback path: NO cached binary → verbatim message + exit 1
      const emptyCache = fs.mkdtempSync(path.join(os.tmpdir(), 'sc-smoke-empty-'));
      const r3 = spawnFileSync(process.execPath, [path.join(__dirname, '..', 'bin', 'stagecoach.js')],
                               { stdio: ['ignore', 'pipe', 'pipe'], env: { ...process.env, STAGECOACH_CACHE_DIR: emptyCache } });
      assert(r3.status === 1, 'fallback should exit 1, got ' + r3.status);
      assert(r3.stderr.toString().includes('stagecoach native binary not installed'), 'missing fallback message');
      console.log('SMOKE PASS');
  - NAMING/PLACEMENT: npm/test/smoke.cjs. Run with `node npm/test/smoke.cjs`.
  - VALIDATE: `node --check npm/test/smoke.cjs` && `node npm/test/smoke.cjs` → prints `SMOKE PASS`, exit 0.

Task 5: VALIDATE — syntax, JSON, smoke, scope
  - node --check npm/bin/stagecoach.js && node --check npm/test/smoke.cjs   # both exit 0
  - node -e "const p=require('./npm/package.json'); ['name','version','bin','scripts','engines','license'].forEach(k=>{if(!(k in p))process.exit(1)}); if(p.bin.stagecoach!=='./bin/stagecoach.js')process.exit(1); if(p.scripts.postinstall!=='node install.cjs')process.exit(1)"   # field guard
  - node npm/test/smoke.cjs                                                 # SMOKE PASS (3 sub-cases)
  - git status --porcelain                                                  # ONLY npm/* (scope guard — Level 4)
```

### Implementation Patterns & Key Details

```javascript
// PATTERN (the shim — env tag FIRST, then resolve, then spawn-or-fallback):
#!/usr/bin/env node
'use strict';
const { spawnFileSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

process.env.STAGECOACH_INSTALL_METHOD = 'npm';                 // FR-U2 — MUST precede the spawn
const pkg = require('../package.json');
const goos = process.platform === 'win32' ? 'windows' : process.platform;
const goarch = process.arch === 'x64' ? 'amd64' : process.arch;
const binaryName = process.platform === 'win32' ? 'stagecoach.exe' : 'stagecoach';
const cacheRoot = process.env.STAGECOACH_CACHE_DIR || path.join(os.homedir(), '.stagecoach', 'versions');
const cachedBin = path.join(cacheRoot, pkg.version, `${goos}-${goarch}`, binaryName);

if (!fs.existsSync(cachedBin)) {
  process.stderr.write(
    'stagecoach native binary not installed (npm --ignore-scripts or postinstall blocked). ' +
    'Install directly: https://github.com/dabstractor/stagecoach#install\n'
  );
  process.exit(1);
}
const result = spawnFileSync(cachedBin, process.argv.slice(2), { stdio: 'inherit', env: process.env });
if (result.error) {
  process.stderr.write('stagecoach: failed to launch native binary: ' + result.error.message + '\n');
  process.exit(1);
}
process.exit(typeof result.status === 'number' ? result.status : 1);

// CRITICAL: spawnFileSync (NOT execSync) + stdio:'inherit' — execSync buffers stdout, breaking interactive
// TTY streaming (--edit, progress, multi-turn). spawnFileSync streams through, preserving the native UX.

// CRITICAL: result.status is null when the child is signal-killed (unix) → exit 1 (conventional). Do NOT
// pass null to process.exit (it exits 0, hiding the kill). The typeof-number guard handles it.
```

### Integration Points

```yaml
CONSUMES (the FR-U2 wire — ALREADY LANDED Go code, READ-ONLY):
  - internal/upgrade/detect.go: STAGECOACH_INSTALL_METHOD is the explicit-override tier (7a) of the detection
    cascade (P1.M2.T1.S1 Complete). The npm value routes `stagecoach upgrade` to delegation (FR-U3 npm row).
    S1 SETS the env var; it adds no Go.
PRODUCES (consumed by sibling tasks):
  - npm/package.json: S2's install.cjs is DECLARED via scripts.postinstall; S2 creates the file. S3/CI bumps
    `version` to match the Go tag and runs `npm publish npm/` + the full cross-platform smoke matrix.
  - The cache-path scheme (~/.stagecoach/versions/<v>/<goos>-<arch>/<binary>, STAGECOACH_CACHE_DIR-overridable):
    S2's install.cjs MUST write to the identical path; this PRP is the contract for that path.
NO Go/build/config/migration changes — npm/ is a self-contained Node subpackage (`.gitignore` already ignores
  /node_modules/, so a dev node_modules won't pollute git).
```

## Validation Loop

> **This is a Node item, not a Go/Python one.** The template's `ruff`/`mypy`/`pytest`/`go test` gates DO NOT
> apply. Use `node --check`, `node` direct-run, and the smoke test. No `go build` (S1 adds no Go code).

### Level 1: Syntax & manifest validity (Immediate Feedback)

```bash
# Syntax-check both JS files (catches parse errors / typos before runtime).
node --check npm/bin/stagecoach.js && echo "bin OK"
node --check npm/test/smoke.cjs   && echo "test OK"
# Expected: "bin OK" + "test OK", exit 0 each.

# package.json is valid JSON AND has the mandatory fields + exact bin/postinstall values.
node -e "const p=require('./npm/package.json'); ['name','version','bin','scripts','engines','license','repository','homepage'].forEach(k=>{if(!(k in p)){console.error('missing '+k);process.exit(1)}}); if(p.name!=='@dabstractor/stagecoach'){console.error('bad name');process.exit(1)}; if(p.bin.stagecoach!=='./bin/stagecoach.js'){console.error('bad bin');process.exit(1)}; if(p.scripts.postinstall!=='node install.cjs'){console.error('bad postinstall');process.exit(1)}; if(p.engines.node!=='>=18'){console.error('bad engines');process.exit(1)}; console.log('package.json OK')"
# Expected: "package.json OK", exit 0.
```

### Level 2: Shim behavior + smoke test (Component Validation)

```bash
# Fallback path (no cached binary): verbatim message to stderr + exit 1.
STAGECOACH_CACHE_DIR="$(mktemp -d)" node npm/bin/stagecoach.js --version
# Expected: exit code 1; stderr contains "stagecoach native binary not installed (npm --ignore-scripts or postinstall blocked). Install directly: https://github.com/dabstractor/stagecoach#install".

# The full smoke test (exec + env-tag + exit-code propagation + fallback).
node npm/test/smoke.cjs
# Expected: prints "SMOKE PASS", exit 0 (3 sub-cases: SMOKE-OK+install_method=npm, exit 42, fallback exit 1).
```

### Level 3: End-to-end manual (System Validation)

```bash
# Place a REAL-ish fake binary (the shim is the only new code; prove it execs a real binary + forwards args).
CACHE="$(mktemp -d)"; mkdir -p "$CACHE/$(node -p "require('./npm/package.json').version")/$(uname -s | tr A-Z a-z)-amd64"
cat > "$CACHE/$(node -p "require('./npm/package.json').version")/$(uname -s | tr A-Z a-z)-amd64/stagecoach" <<'EOF'
#!/bin/sh
echo "real-binary-here args=$* method=$STAGECOACH_INSTALL_METHOD"
EOF
chmod +x "$CACHE/$(node -p "require('./npm/package.json').version")/$(uname -s | tr A-Z a-z)-amd64/stagecoach"
STAGECOACH_CACHE_DIR="$CACHE" node npm/bin/stagecoach.js --version --foo bar
# Expected: "real-binary-here args=--version --foo bar method=npm" — proves argv forwarding + env-tag inheritance.
# (Note: install.cjs is S2 — this manual step uses a hand-placed binary, NOT npm install.)
```

### Level 4: Scope guard

```bash
# ONLY the four npm/* files changed (no Go, no .goreleaser.yaml, no install.cjs, no plan/PRD).
git status --porcelain
# Expected: ?? npm/bin/stagecoach.js  ?? npm/package.json  ?? npm/README.md  ?? npm/test/smoke.cjs  (nothing else).
git status --porcelain | grep -vE '^\?\? npm/(bin/stagecoach\.js|package\.json|README\.md|test/smoke\.cjs)$' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
# Expected: "OK: scope clean".
git status --porcelain | grep -E 'install\.cjs|\.goreleaser\.yaml|\.github/|internal/|PRD\.md|plan/|tasks\.json' && echo "FAIL: forbidden file" || echo "OK: no forbidden files"
# Expected: "OK: no forbidden files" (install.cjs is S2's; .goreleaser.yaml + CI + Go are untouched).
```

## Final Validation Checklist

### Technical Validation
- [ ] `node --check npm/bin/stagecoach.js` exit 0; `node --check npm/test/smoke.cjs` exit 0
- [ ] package.json field guard (Level 1) passes — name/bin/postinstall/engines/license exact
- [ ] `node npm/test/smoke.cjs` prints `SMOKE PASS` (exec + env-tag + exit-42 + fallback sub-cases)
- [ ] Fallback path (Level 2): no cached binary → verbatim message + exit 1

### Feature Validation
- [ ] The shim sets `process.env.STAGECOACH_INSTALL_METHOD='npm'` BEFORE spawning (FR-U2)
- [ ] The shim resolves the cached binary at `STAGECOACH_CACHE_DIR || ~/.stagecoach/versions/<v>/<goos>-<goarch>/<binary>`
- [ ] The shim `spawnFileSync`s with `{stdio:'inherit', env:process.env}` (streams TTY; inherits env incl. the tag)
- [ ] The shim propagates the child exit code (signal-killed → exit 1)
- [ ] The Node→goreleaser map (`win32`→`windows`, `x64`→`amd64`) matches `internal/upgrade/download.go assetName`
- [ ] package.json declares `scripts.postinstall='node install.cjs'` (the file is S2's; S1 does NOT create it)

### Scope-Boundary Validation
- [ ] `git status --porcelain` shows ONLY `npm/bin/stagecoach.js`, `npm/package.json`, `npm/README.md`, `npm/test/smoke.cjs`
- [ ] NO `npm/install.cjs` created (S2's contract — P2.M1.T1.S2)
- [ ] NO `.github/workflows` edit (S3's contract — P2.M1.T1.S3)
- [ ] NO Go files, `.goreleaser.yaml`, README.md (root), PRD.md, plan/**, tasks.json touched
- [ ] NO Go build/test run for this item (S1 adds no Go code — it is a Node subpackage)

---

## Anti-Patterns to Avoid

- ❌ Don't use `execSync` / `child_process.exec` — they BUFFER stdout, breaking `--edit`, progress lines, and
  multi-turn TTY streaming. Use `spawnFileSync` with `stdio:'inherit'` (stream-through).
- ❌ Don't set `STAGECOACH_INSTALL_METHOD` only in a local variable — set it on `process.env` so the spawned
  child inherits it (the whole point of FR-U2). Spawn with `env: process.env` explicitly.
- ❌ Don't use a package-dir cache (`node_modules/@dabstractor/.../`) — a global root-owned install makes it
  unwritable for postinstall. Use the home-dir versioned cache (`~/.stagecoach/versions/<v>/...`) with a
  `STAGECOACH_CACHE_DIR` override for tests.
- ❌ Don't add `"type":"module"` to package.json — the CJS shim uses `require`; CommonJS (the default) is correct.
- ❌ Don't add `os`/`cpu` arrays or `optionalDependencies` — §3 mandates a SINGLE package with RUNTIME platform
  detection (the postinstall download). Restricting os/cpu would refuse install on other platforms.
- ❌ Don't create `npm/install.cjs` — it is S2's deliverable (P2.M1.T1.S2). S1 only DECLARES the postinstall
  script. Shipping a stub would collide with S2's contract.
- ❌ Don't hand-set a real `version` in package.json — `"0.0.0"` is the dev placeholder; S3/CI bumps it to match
  the Go release tag at `npm publish` time.
- ❌ Don't deviate from the Node→goreleaser map (`win32`→`windows`, `x64`→`amd64`) — it must match
  `internal/upgrade/download.go assetName` so S2 writes the path S1 resolves.
- ❌ Don't pass `result.status` (possibly `null`) straight to `process.exit` — `process.exit(null)` exits 0,
  hiding a signal kill. Guard with `typeof result.status === 'number' ? result.status : 1`.
- ❌ Don't run `npm install` in `npm/` as a validation — install.cjs doesn't exist yet (S2), so postinstall will
  error. Validate the shim directly via `node npm/bin/stagecoach.js` + `node npm/test/smoke.cjs`.