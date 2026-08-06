# Research Findings — P2.M1.T1.S1: npm package.json + bin/stagecoach.js shim

## §0 PRD-selector mismatch (resolved)

The task's `prd_selectors` resolved to **§9.26 Work-description mode**, which is UNRELATED to this item.
The authoritative spec for this npm-wrapper task is **`plan/017_397abce9deb1/architecture/external_deps.md §3`
(the npm wrapper — the esbuild/turbo/prisma pattern)** + the `<item_description>` contract + the goreleaser
config (`.goreleaser.yaml`) for asset names. This findings file is built from those three (verified on disk),
NOT from §9.26.

## §1 The authoritative wrapper spec — external_deps.md §3 (verbatim essentials)

A thin JS package `@dabstractor/stagecoach` whose `postinstall` downloads the matching prebuilt native binary
into a cache and whose `bin` execs it. **Single-package + postinstall-download form** (NOT optionalDependencies).
Key shape:
- `package.json`: `"bin": { "stagecoach": "./bin/stagecoach.js" }`; `"scripts": { "postinstall": "node install.cjs" }`.
- `install.cjs` (S2's deliverable, NOT S1's): maps `process.platform`/`process.arch` → goreleaser asset name;
  downloads archive + checksums.txt; SHA256-verifies; extracts the single `stagecoach`/`stagecoach.exe` to a
  cache dir. On `--ignore-scripts`/corporate-npm the binary is absent → the shim prints a fallback message.
- **`bin/stagecoach.js` (the shim — THIS task's core deliverable)**: sets
  `process.env.STAGECOACH_INSTALL_METHOD = "npm"` on every invocation (FR-U2), then `child_process.spawn`/
  `execFileSync` the cached native binary, forwarding argv/stdin/stdout/stderr and preserving exit code.
  This is how `stagecoach upgrade` recognizes an npm install and **delegates to
  `npm install -g @dabstractor/stagecoach@latest` instead of self-swapping** (FR-U3 npm row).
- **Module path**: goreleaser has NO native npm pipe. The wrapper is published by a separate CI step (S3:
  a GitHub Action that `npm publish`s on tag). The wrapper ships ONLY JS; binaries are downloaded at install
  time from the goreleaser GitHub Release.
- Verify-at-impl duties (handed to S2/S3): exact asset name (§2 below); Node floor; tar/unzip vs JS lib.

## §2 goreleaser asset names — the EXACT strings the JS platform map must produce (verified)

`.goreleaser.yaml` (verified on disk):
```yaml
archives.name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"   # stagecoach_<v>_<os>_<arch>
formats: [tar.gz] ; format_overrides: windows → zip
checksum.name_template: "{{ .ProjectName }}_{{ .Version }}_checksums.txt"            # stagecoach_<v>_checksums.txt
builds.binary: stagecoach   # name INSIDE the archive (stagecoach.exe on windows via GOOS)
```
Confirmed by the Go side (`internal/upgrade/download.go:57` `assetName`):
```go
func assetName(tag, goos, goarch string) string {
	ver := strings.TrimPrefix(tag, "v")            // tag v1.0.0 → "1.0.0"
	name := "stagecoach_" + ver + "_" + goos + "_" + goarch
	if goos == "windows" { return name + ".zip" }
	return name + ".tar.gz"
}
```
→ The **6 supported targets**: `linux|darwin|windows` × `amd64|arm64`. Asset examples:
`stagecoach_1.0.0_darwin_arm64.tar.gz`, `stagecoach_1.0.0_linux_amd64.tar.gz`,
`stagecoach_1.0.0_windows_amd64.zip`. Binary inside archive: `stagecoach` (unix) / `stagecoach.exe` (windows).

**Node → goreleaser mapping (S1 needs it for the binary NAME + cache path; S2 also needs it for the asset):**
| `process.platform` | → goos | `process.arch` | → goarch |
|---|---|---|---|
| `darwin` | `darwin` | `arm64` | `arm64` |
| `linux` | `linux` | `x64` | `amd64` |
| `win32` | `windows` | (others) | identity |
| (else) | identity | | |

So: `goos = platform==='win32' ? 'windows' : platform`; `goarch = arch==='x64' ? 'amd64' : arch`.
`binaryName = platform==='win32' ? 'stagecoach.exe' : 'stagecoach'`.

## §3 The cache-path DECISION (S1 resolves; S2 writes — MUST agree)

The contract offers two options (`node_modules/.cache` vs `os.homedir()/.stagecoach/versions/<v>`).
**DECISION: versioned cache under a home dir root, env-overridable for tests:**
```
cacheRoot  = process.env.STAGECOACH_CACHE_DIR || path.join(os.homedir(), '.stagecoach', 'versions')
cachedBin  = path.join(cacheRoot, version, `${goos}-${goarch}`, binaryName)
version    = require('../package.json').version   // the wrapper's own version (synced to the Go tag)
```
Rationale:
1. **Writable in EVERY install layout** — a package-dir cache (`node_modules/@dabstractor/.../`) is NOT
   writable in a root-owned global install (`/usr/local/lib/node_modules`); the home dir always is. This is
   the #1 robustness property for postinstall (S2) and the reason home-dir beats package-dir.
2. **Version-keyed** → the shim finds the EXACT binary for the installed wrapper version; a local + global
   install of different versions coexist without collision.
3. **`STAGECOACH_CACHE_DIR` env override** — REQUIRED for the smoke test to redirect the cache to a temp dir
   (otherwise it pollutes the developer's real `~/.stagecoach`). This is the testability seam; S2 honors it too.
4. Matches the contract's explicit `os.homedir()/.stagecoach/versions/<v>` example.
Stale-cache accumulation (older versions linger) is a known minor cost, addressed by S2's postinstall
(best-effort prune) — NOT S1's concern. **S2's PRP MUST use the identical scheme.**

## §4 STAGECOACH_INSTALL_METHOD — the FR-U2 wire (already consumed by LANDED Go code)

The shim sets `process.env.STAGECOACH_INSTALL_METHOD = 'npm'` BEFORE spawning and passes `env: process.env`
so the native child inherits it. This is the **explicit-override tier (7a)** of the install-method detection
cascade (`internal/upgrade/detect.go`, P1.M2.T1.S1 LANDED; external_deps.md §7). Effect: when a user runs
`stagecoach upgrade` from an npm install, detection returns `ChannelNpm` (or the npm row) and `runDelegate`
runs `npm install -g @dabstractor/stagecoach@latest` (FR-U3) instead of self-swapping the cached binary.
**CRITICAL**: the env var must be set on `process.env` AND the child spawned with `env: process.env` (the
contract's exact `{ stdio:'inherit', env:process.env }`). Setting it only in a local var would NOT propagate.

## §5 The --ignore-scripts fallback message (verbatim from contract)

When the cached binary is absent (postinstall was skipped: `npm install --ignore-scripts`, corporate npm
proxy, or a flaky postinstall), the shim writes to STDERR and exits 1:
```
stagecoach native binary not installed (npm --ignore-scripts or postinstall blocked). Install directly: https://github.com/dabstractor/stagecoach#install
```
(URL anchor `#install` must match README's install-heading slug — see §7.)

## §6 package.json — every field, with provenance

| Field | Value | Provenance / note |
|---|---|---|
| `name` | `@dabstractor/stagecoach` | external_deps.md §3 + goreleaser owner `dabstractor`. |
| `version` | `0.0.0` (dev placeholder) | Contract: "synced to the Go release tag". S3/CI bumps it to match the Go tag (e.g. `1.0.0`) at publish. NEVER publish `0.0.0`. |
| `description` | Mode A doc string (contract DOCS) | e.g. "Thin npm wrapper that downloads and runs the stagecoach native binary — a snapshot-based AI commit-message generator that uses YOUR local CLI agent." |
| `bin` | `{ "stagecoach": "./bin/stagecoach.js" }` | §3 / contract. |
| `scripts.postinstall` | `node install.cjs` | Declares S2's postinstall; **install.cjs is S2's deliverable** — S1 does NOT create it (would conflict with S2's contract). Until S2 lands, `npm install` in npm/ will error on the missing postinstall script; that's EXPECTED — S1 validates via `node bin/stagecoach.js` directly, not via `npm install`. |
| `engines.node` | `>=18` | Contract ("fetch is stdlib https/fs"); matches Node 18 LTS baseline. S1 itself only needs child_process/fs/os/path (ancient), but the floor is set for S2 (https/fetch). |
| `license` | `MIT` | goreleaser assumes MIT (.goreleaser.yaml L84/98/114). **⚠️ NO LICENSE FILE exists in the repo** (`ls LICENSE*` empty) — flag: the repo must reconcile its actual license; `"MIT"` here mirrors goreleaser's assumption. |
| `repository` | `{ "type":"git", "url":"https://github.com/dabstractor/stagecoach.git" }` | goreleaser `release.github.owner=dabstractor, name=stagecoach`. npm metadata. |
| `homepage` | `https://github.com/dabstractor/stagecoach#readme` | npm metadata. |
| `bugs` | `{ "url":"https://github.com/dabstractor/stagecoach/issues" }` | npm metadata. |
| `keywords` | `["commit","git","ai","commit-message","cli"]` | discoverability (optional but cheap). |
| (NO `type`) | — | Default `commonjs` ⇒ `.js` shim uses `require` (correct). Do NOT add `"type":"module"` (would break the CJS shim). install.cjs is CJS-by-extension. |
| (NO `os`/`cpu`) | — | Single package with RUNTIME platform detection — do NOT restrict os/cpu or npm refuses install on other platforms. |

## §7 npm/README.md (Mode A docs — contract DOCS bullet)

ONE paragraph (the contract says "one paragraph: what the wrapper is, the --ignore-scripts fallback").
Mirror the goreleaser `description` tone ("Snapshot-based AI commit message generator that uses YOUR local
CLI agent"). State: (a) this package is a thin wrapper that downloads + runs the native stagecoach binary at
install time; (b) `npm install -g @dabstractor/stagecoach` then `stagecoach …`; (c) the `--ignore-scripts`
fallback: if postinstall was blocked, the shim prints a message pointing at
`https://github.com/dabstractor/stagecoach#install`. **The README MUST have an `## Install` heading** so the
fallback URL's `#install` anchor resolves (the fallback message hardcodes `#install`).

## §8 The smoke test (contract: "A local smoke test: place a fake binary, run node bin/stagecoach.js --version,
assert it execs the fake and the env tag is set")

Deliver as `npm/test/smoke.cjs` (Node CommonJS, cross-platform, no deps). It:
1. Reads `version` from `../package.json` and recomputes the cache path (same §2/§3 scheme) so it matches
   the shim EXACTLY.
2. `mkdtemp`s a temp cache root, sets `process.env.STAGECOACH_CACHE_DIR` to it.
3. Writes a FAKE binary at the computed path. On `win32`: SKIP with a console note ("smoke: skipped on win32;
   S3/CI adds a Windows stub binary"), exit 0 (the shim LOGIC is platform-independent; unix proof suffices).
   On unix: write `#!/bin/sh\necho "SMOKE-OK install_method=$STAGECOACH_INSTALL_MODE"` etc., chmod 0o755.
4. `spawnFileSync(process.execPath, [shimPath, '--version'], { stdio:['ignore','pipe','pipe'], env })`.
5. Assert stdout contains `SMOKE-OK` AND `install_method=npm` (proves the shim exec'd the fake AND set
   STAGECOACH_INSTALL_METHOD on the child env).
6. Exit-code-propagation sub-case: a fake that `exit 42`; assert the shim exits 42.
Cross-platform note: on win32 spawnFileSync to a `.exe` requires a real PE binary; a `.cmd`/`.bat` won't run
without `shell:true` (which the shim deliberately omits for argv fidelity). Hence the unix-only smoke for S1;
S3 (CI) owns the full cross-platform matrix using a real built stub.

## §9 Node-version stdlib confirmation (S1 needs ONLY these — all ancient, stable)

`node:child_process` (`spawnFileSync`), `node:fs` (`existsSync`), `node:os` (`homedir`), `node:path` (`join`),
`require` (CJS). All stable since Node 0.x — no fetch/undici needed for the shim (S2's download is the only
network surface). `spawnFileSync` returns `{ status, signal, error }`: `error` set ⇒ spawn failure (print +
exit 1); else `process.exit(result.status ?? 1)` (null status on signal-killed → exit 1).

## §10 .gitignore / scope

`.gitignore` L13 already ignores `/node_modules/` — so the npm/ package can have a local node_modules
(postinstall / dev) without polluting git. The npm/ source (package.json, bin/, test/, README.md) IS committed.
**Scope fence**: S1 touches ONLY new files under `npm/` (package.json, bin/stagecoach.js, README.md,
test/smoke.cjs). NO Go files, NO .goreleaser.yaml, NO PRD/plan/tasks, NO install.cjs (S2), NO CI wiring (S3).

## §11 Validation (Go project, but THIS item is Node — use node, not go)

- `node npm/bin/stagecoach.js` with no cached binary → prints fallback, exit 1 (manual Level 1).
- `node npm/test/smoke.cjs` → SMOKE-OK + install_method=npm + exit 42 propagation (Level 2).
- `node -e "JSON.parse(require('fs').readFileSync('npm/package.json'))"` → valid JSON (Level 1).
- `node --check npm/bin/stagecoach.js && node --check npm/test/smoke.cjs` → syntax OK (Level 1).
- Scope guard: `git status --porcelain` shows ONLY `npm/*` new files.
- (NO `go build`/`go test` for S1 — it adds no Go code. S3 wires the smoke into CI.)