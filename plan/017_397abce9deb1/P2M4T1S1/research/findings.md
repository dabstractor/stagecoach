# Research: asdf / mise plugin for `stagecoach` (P2.M4.T1.S1)

Verified contract + codebase facts consolidated for the PRP. Two live web searches + one
researcher subagent (recall from canonical docs) + direct codebase reads. Load-bearing facts
are live-confirmed where marked [LIVE].

## 1. asdf plugin author contract — LIVE VERIFIED

Source: **https://asdf-vm.com/plugins/create.html** ("Create a Plugin" — confirmed exists, content
matches). Backed by **https://github.com/asdf-vm/asdf** (install command flow) and the
`asdf-vm/asdf-plugin-template` (proves `lib/utils.sh` is optional).

- A plugin is a **git repo** with two required executable scripts:
  - **`bin/list-all`** — print installable versions to **stdout**, one per line (newline-separated;
    asdf tokenizes on whitespace, so newline is the modern convention). **Errors → stderr.**
    Ordering: asdf does NOT hard-enforce order; convention is **ascending via `sort -V`** (oldest →
    newest, last line newest) so `latest`-style resolution works on older asdf. **No leading blank
    line needed.**
  - **`bin/install`** — invoked **once per install**. Reads env vars (takes NO CLI args). Must create
    **`$ASDF_INSTALL_PATH/bin/`** and place the executable(s) there. This is THE load-bearing rule:
    asdf later shims everything in `$ASDF_INSTALL_PATH/bin/`, and `which stagecoach` resolves there.
- **Env vars set before `bin/install` runs** [LIVE confirmed by both asdf docs + mise asdf-legacy-plugins page]:
  - `ASDF_INSTALL_TYPE` ∈ {`"version"`, `"ref"`} — normal `asdf install <tool> <ver>` → `version`;
    `asdf install <tool> ref:<gitref>` → `ref`. `latest` is resolved to a real version first, so it
    still arrives as `type=version`.
  - `ASDF_INSTALL_VERSION` — the version string (e.g. `1.2.3`) or the git ref.
  - `ASDF_INSTALL_PATH` — the absolute install dir the plugin must populate (create `bin/` inside).
  - (Newer asdf also sets `ASDF_DOWNLOAD_PATH`, `ASDF_CONCURRENCY`, `ASDF_PLUGIN_PATH`,
    `ASDF_PLUGIN_SOURCE_URL|TYPE|REVISION` — all OPTIONAL for a tarball plugin.)
- **`lib/utils.sh` is OPTIONAL.** The asdf-plugin-template ships one with shared helpers, but a
  minimal self-contained plugin needs ONLY `bin/list-all` + `bin/install`. **Decision: do NOT source
  a remote `utils.sh`** (keeps the plugin dependency-free + auditable; matches the supply-chain mood
  in mise discussions #4054).

## 2. mise asdf-compatibility — LIVE CONFIRMED: same scripts work unchanged

Sources:
- **https://mise.jdx.dev/asdf-legacy-plugins.html** — *"asdf plugins have access to these environment
  variables: ASDF_INSTALL_TYPE - version or ref; ASDF_INSTALL_VERSION - Version number or git ref…"*
  → **mise sets the SAME `ASDF_*` vars** when running an asdf plugin's scripts.
- **https://mise.jdx.dev/plugins.html** — *"mise can use asdf's plugin ecosystem under the hood for
  backward compatibility. These plugins contain shell scripts like bin/install…"*
- **https://github.com/mise-plugins** (the org) — *"While all asdf plugins should work in mise, the
  inverse is not necessarily the case."*

**Conclusion: one set of POSIX-sh scripts (`bin/list-all` + `bin/install`) serves BOTH asdf and mise
unchanged.** `ASDF_INSTALL_PATH` under mise points at `~/.local/share/mise/installs/stagecoach/<ver>`;
the same `…/bin/` rule applies. We do NOT author a mise-native plugin.

**mise commands** (for the README):
- `mise plugin add stagecoach https://github.com/dabstractor/asdf-stagecoach.git`  (task-contract form)
- `mise plugins install stagecoach https://github.com/dabstractor/asdf-stagecoach.git`  (also valid; older canonical form)
- then `mise install stagecoach@latest` / `mise use -g stagecoach@latest`

## 3. list-all: `git ls-remote` (preferred) vs GitHub Releases API

- **GitHub Releases API** unauthenticated rate limit = **60 req/h/IP**
  (https://docs.github.com/en/rest/overview/resources-in-the-rest-api#rate-limiting). `list-all`
  runs on every `asdf list-all stagecoach` / tab-completion / `asdf install stagecoach` listing →
  easily blown. **Do NOT use the API.**
- **`git ls-remote --refs --tags <url>` has NO per-IP quota.** Popular Go-binary plugins
  (asdf-golang, asdf-hashicorp, asdf-kubectl) use this. `--refs` excludes the peeled
  `refs/tags/x^{}` dereferenced duplicates.
- Output format (TAB-separated): `<40-hex-sha>\trefs/tags/<tag>`. Extraction:
  ```sh
  git ls-remote --refs --tags https://github.com/dabstractor/stagecoach.git 2>/dev/null \
    | sed 's#^.*refs/tags/##' \   # drop "<sha>\trefs/tags/" → "v1.2.3"
    | sed 's/^v//' \              # strip a leading "v" → "1.2.3"
    | grep -E '^[0-9]' \          # keep only version-like tags (defensive)
    | sort -V                     # ascending: 1.2.9 → 1.2.10 ordering correct
  ```
- `sort -V` ships in GNU coreutils AND modern macOS/BSD sort. (Fallback for ancient sort:
  `sort -t. -k1,1n -k2,2n -k3,3n` — not needed; `sort -V` is universal on supported platforms.)

## 4. uname → goreleaser GOOS/GOARCH map (for POSIX sh bin/install)

`uname -s` → GOOS: `Darwin`→`darwin`, `Linux`→`linux` (asdf/mise are Unix-only → **windows/zip is
out of scope for this plugin**; Windows users use Scoop/Winget per PRD §21.3).
`uname -m` → GOARCH: `x86_64|amd64`→`amd64`, `aarch64|arm64`→`arm64`.
stagecoach's `.goreleaser.yaml` builds ONLY `amd64` + `arm64` (6 targets: linux/darwin/windows ×
amd64/arm64). **Reject any other arch with a clear error** (do not download a non-existent 32-bit
asset). Canonical idiom (matches asdf-golang style):
```sh
case "$(uname -s)" in
  Darwin) os=darwin ;; Linux) os=linux ;;
  *) printf 'stagecoach: unsupported OS: %s (plugin supports macOS + Linux)\n' "$(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;;
  *) printf 'stagecoach: unsupported arch: %s (builds amd64/arm64 only)\n' "$(uname -m)" >&2; exit 1 ;;
esac
```

## 5. SHA256 verification (portable POSIX sh) against goreleaser checksums.txt

- Two tools by platform: **`sha256sum`** (Linux/coreutils), **`shasum -a 256`** (macOS/BSD). Detect:
  ```sh
  if command -v sha256sum >/dev/null 2>&1; then
    sha256() { sha256sum "$1" | awk '{print $1}'; }
  elif command -v shasum >/dev/null 2>&1; then
    sha256() { shasum -a 256 "$1" | awk '{print $1}'; }
  else printf 'stagecoach: need sha256sum or shasum\n' >&2; exit 1; fi
  ```
  (Function form avoids word-splitting pitfalls of a variable-stored command; POSIX-sh safe.)
- goreleaser `checksums.txt` line = **`<64hex>  <filename>`** (TWO spaces;
  https://goreleaser.com/customization/checksum/). awk collapses whitespace, so exact-match the
  asset name in field 2 (avoids regex/`.` wildcard issues + substring matches like 1.2.3 vs 1.2.30):
  ```sh
  expected="$(awk -v f="$asset" '$2==f {print $1; exit}' "${tmp}/${sums}")"
  [ -n "$expected" ] || { printf '... no checksum line for %s\n' "$asset" >&2; exit 1; }
  got="$(sha256 "${tmp}/${asset}")"
  [ "$expected" = "$got" ] || { printf '... SHA256 mismatch ...\n' >&2; exit 1; }
  ```

## 6. Codebase facts (verified against the repo — the install script must match these EXACTLY)

From `.goreleaser.yaml` + `internal/upgrade/download.go` (assetName/checksumsName) + `npm/install.cjs`:
- **Asset name**: `stagecoach_<version>_<os>_<arch>.tar.gz` (windows→`.zip`, but plugin is Unix-only
  so always `.tar.gz`). `<version>` = **tag WITHOUT leading `v`** (goreleaser `{{.Version}}`).
  → e.g. `stagecoach_1.0.0_linux_amd64.tar.gz`, `stagecoach_1.0.0_darwin_arm64.tar.gz`.
- **Checksums file**: `stagecoach_<version>_checksums.txt`.
- **Download URL** (DIRECT, not the API — matches npm wrapper's `STAGECOACH_DOWNLOAD_BASE` decision
  in external_deps.md §1): `https://github.com/dabstractor/stagecoach/releases/download/v<version>/<asset>`.
  Tag in the URL = `v<version>` (WITH the v). GitHub 302-redirects to objects.githubusercontent.com;
  `curl -fL` follows redirects. No auth needed (public repo). The **npm wrapper uses the same direct
  URL + a `STAGECOACH_DOWNLOAD_BASE` override for testing** → reuse that exact env var name in
  `bin/install` for cross-surface consistency + a test hook.
- **Archive layout**: the single `stagecoach` binary ships at the archive ROOT
  (`.goreleaser.yaml` `builds.binary: stagecoach`). Extract into `$ASDF_INSTALL_PATH/bin/` → lands
  at `$ASDF_INSTALL_PATH/bin/stagecoach`. (Matches `npm/install.cjs` which extracts into the version
  cache dir without a member filter.)
- **Repo namespace**: `dabstractor/stagecoach`. **Plugin-repo convention** (task contract): separate
  repo `dabstractor/asdf-stagecoach`; scripts authored canonically HERE under
  `plugins/asdf-stagecoach/` and mirrored to that repo. (One-time bootstrap to create
  `dabstractor/asdf-stagecoach` is out-of-band, like homebrew-tap/scoop-bucket.)

## 7. Smoke-test pattern (mirror npm/test/install.test.cjs in POSIX sh)

The npm wrapper's `install.test.cjs` is the exact analog: it (1) builds a fixture archive + checksums
at runtime (no committed binaries), (2) serves via a local http server, (3) runs install against it
asserting download+SHA256-verify+extract+executable-bit, (4) rewrites checksums with a WRONG digest
and asserts install **aborts non-zero with NO binary left behind** (abort-before-write analog).

The asdf plugin smoke test (`plugins/asdf-stagecoach/test/smoke.sh`, POSIX sh) mirrors this:
- Build a fake `stagecoach` binary (`#!/bin/sh\necho fake-stagecoach`), `tar -czf` into a fixture
  archive named `stagecoach_0.0.0_<os>_<arch>.tar.gz`, compute its real SHA256, write
  `stagecoach_0.0.0_checksums.txt`.
- Serve the fixture dir with `python3 -m http.server 0` (binds a random free port; prints the port
  to stderr; scrape it with a retry loop). `python3` ships on all GitHub runners (ubuntu + macos).
- Set `ASDF_INSTALL_TYPE=version ASDF_INSTALL_VERSION=0.0.0 ASDF_INSTALL_PATH=<temp> STAGECOACH_DOWNLOAD_BASE=http://127.0.0.1:<port>`
  and run `bin/install`. Assert exit 0 + `$ASDF_INSTALL_PATH/bin/stagecoach` exists, is executable
  (mode & 0o111), and runs (prints `fake-stagecoach`).
- **Mismatch path**: rewrite checksums with `printf '%064d' 0` (64 zeros), fresh install path, run
  `bin/install`; assert it exits NON-ZERO, prints a mismatch message, and leaves NO binary behind
  (abort-before-write: the tampered archive is removed before extraction).
- Clean up (rm -rf temp dirs; kill the http server) via a trap.

## 8. CI integration (parity with npm-smoke + nix-flake-check)

`.github/workflows/ci.yml` already has 6 jobs: build-test, lint, vulncheck, coverage, npm-smoke,
nix-flake-check. A 7th `asdf-plugin-smoke` job (ubuntu-latest) runs (a) `shellcheck -s sh` on the
three POSIX-sh scripts (catches bashisms — ubuntu-latest ships shellcheck) and (b)
`sh plugins/asdf-stagecoach/test/smoke.sh`. This keeps the plugin scripts from rotting on every PR,
exactly as npm-smoke guards the npm wrapper and nix-flake-check guards the flake. The contract
language "verify the asdf/mise env-var contract at impl (record date)" is satisfied by this repeatable
gate + a one-time live-doc check recorded in the PRP/README.

## 9. Sibling coordination (no collisions)

- **P2.M3.T1.S1 (Nix flake)** — concurrent; touches `flake.nix`, `flake.lock`, `.gitignore`,
  `.github/workflows/ci.yml` (adds nix-flake-check job), `docs/packaging.md` (Nix section). THIS task
  touches `.github/workflows/ci.yml` (adds asdf-plugin-smoke job) — **additive job; no collision with
  the nix job**. `.gitignore`: this task does NOT touch it. `docs/packaging.md`: this task does NOT
  touch it (Mode A docs = the plugin's own README; README install rewrite is P3/Mode B). So the only
  shared file is `ci.yml`, and the two jobs are independent additive entries.
- **P2.M2.T1.S1 (Winget)** + **P2.M1 (npm)** — complete/parallel; no file overlap with this task.
- **P3.M1.T1.S1** — README install rewrite (Mode B); will LINK to the asdf plugin. This task's README
  is Mode A (self-contained in `plugins/asdf-stagecoach/README.md`); the top-level README `## Install`
  "Coming soon" list gets the mise/asdf line in P3, NOT here (the contract says "link from README
  install is Mode B/P3").

## Open FR-D5 verification duties (at impl, record date)
1. Re-confirm `https://asdf-vm.com/plugins/create.html` still documents the `ASDF_*` env-var contract
   + the `$ASDF_INSTALL_PATH/bin/` rule (page has been reorganized across versions).
2. Re-confirm `mise.jdx.dev/asdf-legacy-plugins.html` still lists `ASDF_INSTALL_TYPE`/`VERSION` (mise
   sets them for asdf plugins) → proves the SAME scripts work for mise.
3. Confirm ubuntu-latest runner still ships `shellcheck` + `python3` (for the CI job + smoke test).
4. Confirm goreleaser `name_template`/`checksum.name_template` still emit
   `stagecoach_<v>_<os>_<arch>.tar.gz` / `stagecoach_<v>_checksums.txt` (already verified against
   `.goreleaser.yaml` — re-check if it changed).