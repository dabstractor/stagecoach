name: "P2.M4.T1.S1 — asdf/mise plugin scripts (bin/list-all + bin/install) + plugin-repo README + verify contract"
description: |
  Mode A distribution task (PRD §21.2/G27; external_deps.md §6). Author the CANONICAL asdf/mise
  plugin for stagecoach in-repo under `plugins/asdf-stagecoach/`: two POSIX-sh scripts
  (`bin/list-all` → `git ls-remote --refs --tags`, strip `refs/tags/`+`v`, `sort -V` ascending;
  `bin/install` → detect OS/arch via uname, download the matching goreleaser `.tar.gz` + checksums
  from the DIRECT releases/download URL, SHA256-verify (portable `sha256sum`/`shasum -a 256`),
  extract the single `stagecoach` binary to `$ASDF_INSTALL_PATH/bin/`, `chmod +x`), plus a
  `README.md` documenting `asdf plugin add` / `mise plugin add` against the separate plugin repo
  `dabstractor/asdf-stagecoach` (this dir is the canonical source, mirrored there), and a local
  POSIX-sh **smoke test** that builds a fixture archive + checksums, serves via `python3 -m
  http.server`, runs `bin/install`, and asserts the happy path + the SHA256-mismatch abort-before-
  write. ALSO adds a 7th CI job `asdf-plugin-smoke` to `.github/workflows/ci.yml` (shellcheck the 3
  scripts + run the smoke test) — parity with npm-smoke (job 5) and nix-flake-check (job 6). mise is
  asdf-compatible (sets the SAME `ASDF_*` env vars) so ONE set of scripts serves both. No Go changes;
  no goreleaser/npm/release.yml changes; top-level README install line is P3 (Mode B).

---

## Goal

**Feature Goal**: Ship the canonical asdf/mise plugin for stagecoach so a version-manager user can
`asdf plugin add stagecoach … && asdf install stagecoach latest` (or the `mise plugin add` /
`mise use` equivalent) and get a SHA256-verified, goreleaser-release binary at
`$ASDF_INSTALL_PATH/bin/stagecoach` — from a single set of POSIX-sh scripts that work **unchanged
for both asdf and mise** (mise runs asdf plugins and sets the same `ASDF_*` env vars), and so CI
catches a regression in those scripts on every PR (shellcheck + the fixture smoke test).

**Deliverable** (3 new plugin files + 1 new test + 1 CI job):
- `plugins/asdf-stagecoach/bin/list-all` — **CREATED**: POSIX sh. `git ls-remote --refs --tags
  https://github.com/dabstractor/stagecoach.git`, strip `refs/tags/` + leading `v`, filter to
  version-like tags, `sort -V` ascending (newest last), one per line to stdout.
- `plugins/asdf-stagecoach/bin/install` — **CREATED**: POSIX sh. Read `ASDF_INSTALL_TYPE`/`VERSION`/
  `PATH`; uname→goos/goarch map (Darwin/Linux→darwin/linux; x86_64|amd64→amd64, aarch64|arm64→arm64;
  reject others); download `stagecoach_<v>_<os>_<arch>.tar.gz` + `stagecoach_<v>_checksums.txt` from
  the DIRECT `https://github.com/dabstractor/stagecoach/releases/download/v<v>/…` URL (test-overridable
  via `STAGECOACH_DOWNLOAD_BASE`); SHA256-verify (portable `sha256sum`/`shasum -a 256`, awk field-2
  exact match); `mkdir -p $ASDF_INSTALL_PATH/bin`; `tar -xzf` the binary there; `chmod +x`. Abort
  non-zero on any failure (mismatch / missing checksum line / unsupported platform) — abort-before-
  write (no binary left on failure).
- `plugins/asdf-stagecoach/README.md` — **CREATED (Mode A docs)**: what it is, prerequisites, asdf
  add+install+global, mise add+install+use, supported platforms (macOS+Linux, amd64/arm64; NOT
  Windows), and the canonical-source/mirror note (this dir is the source; mirrored to
  `dabstractor/asdf-stagecoach`).
- `plugins/asdf-stagecoach/test/smoke.sh` — **CREATED**: POSIX-sh local smoke test. Builds a fixture
  archive + checksums at runtime, serves via `python3 -m http.server 0`, runs `bin/install` with
  `ASDF_*` + `STAGECOACH_DOWNLOAD_BASE` set, asserts the happy path (binary installed, executable,
  runs) AND the mismatch path (non-zero exit, mismatch message, NO binary left behind).
- `.github/workflows/ci.yml` — **MODIFIED**: add a 7th job `asdf-plugin-smoke` (ubuntu-latest;
  `shellcheck -s sh` the 3 scripts + `sh test/smoke.sh`).

**Success Definition**:
- `bin/list-all`, run with network, prints version tags ascending (e.g. `1.0.0` / `1.1.0` …) to
  stdout, newest last; `list-all` uses `git ls-remote` (NO GitHub API → no 60-req/h rate limit).
- `bin/install` with `ASDF_INSTALL_TYPE=version ASDF_INSTALL_VERSION=0.0.0 ASDF_INSTALL_PATH=<tmp>
  STAGECOACH_DOWNLOAD_BASE=<local-fixture-server>` exits 0 and places an executable `stagecoach` at
  `<tmp>/bin/stagecoach`; on a tampered checksum it exits non-zero and leaves NO binary behind.
- `sh test/smoke.sh` prints `SMOKE PASS` locally (linux/macos) and in CI.
- `shellcheck -s sh bin/list-all bin/install test/smoke.sh` is clean (POSIX-sh, no bashisms).
- The same two scripts are documented for BOTH asdf and mise (mise runs them unchanged).
- ci.yml is valid YAML with the `asdf-plugin-smoke` job; the 6 existing jobs are unchanged.
- `git status --porcelain` shows ONLY: `plugins/asdf-stagecoach/` (new tree) + `M
  .github/workflows/ci.yml`. NO Go, NO goreleaser/npm/release.yml, NO top-level README install edit,
  NO PRD/plan/tasks.

## User Persona (if applicable)

**Target User**: A developer who manages tool versions with **asdf** or **mise** (the version-manager
audience — PRD §21.3 "mise / asdf (version-manager users)"). They pin `stagecoach 1.2.3` in a
`.tool-versions` / `mise.toml` so every checkout of a repo uses the same stagecoach version.
**Use Case**: `asdf plugin add stagecoach …; asdf install stagecoach latest; asdf global stagecoach
latest` (or the `mise` equivalent) → `stagecoach --version` runs the goreleaser binary.
**Pain Points Addressed**: (1) one SHA256-verified install command instead of manual curl|tar; (2)
per-project version pinning via `.tool-versions`/`mise.toml`; (3) works with EITHER asdf or mise
from one plugin (no duplicated maintenance).

## Why

- **PRD §21.2/G27 distribution surface**: mise/asdf is a named channel for the version-manager
  audience. The plugin is ~30 lines of POSIX sh pointing at the existing goreleaser GitHub Releases
  (external_deps.md §6) — no Go change, no new release artifact.
- **Single source of truth, two managers**: mise is asdf-compatible (it runs an asdf plugin's
  `bin/list-all` + `bin/install` verbatim and sets the SAME `ASDF_*` env vars —
  https://mise.jdx.dev/asdf-legacy-plugins.html), so ONE plugin serves both. No mise-native code.
- **Parity with the released binary**: `bin/install` fetches the SAME goreleaser archive + checksums
  the npm wrapper and `stagecoach upgrade` fetch, and verifies SHA256 the same way — so the
  asdf/mise-installed binary is byte-identical to every other channel.
- **CI keeps it honest**: a `shellcheck` + fixture-smoke CI job catches a regression (a bad uname
  map, a checksum-parse bug, a bashism) on every PR, exactly as `npm-smoke` guards the npm wrapper
  and `nix-flake-check` guards the flake.

## What

### Success Criteria
- [ ] `plugins/asdf-stagecoach/bin/list-all` (POSIX sh, executable): `git ls-remote --refs --tags`,
      strip `refs/tags/`+`v`, version-filter, `sort -V` ascending, one per line to stdout.
- [ ] `plugins/asdf-stagecoach/bin/install` (POSIX sh, executable): reads `ASDF_INSTALL_TYPE`/
      `ASDF_INSTALL_VERSION`/`ASDF_INSTALL_PATH`; uname→goos/goarch; downloads asset + checksums from
      the DIRECT releases/download URL; portable SHA256 verify (field-2 exact match); extracts to
      `$ASDF_INSTALL_PATH/bin/stagecoach`; `chmod +x`. Aborts non-zero (no binary left) on mismatch /
      missing checksum / unsupported platform.
- [ ] `plugins/asdf-stagecoach/README.md`: asdf `plugin add`+`install`+`global`, mise `plugin add`+
      `install`+`use`, prerequisites (asdf|mise, git, curl, tar, awk, sha256sum|shasum), supported
      platforms, canonical-source/mirror note.
- [ ] `plugins/asdf-stagecoach/test/smoke.sh` (POSIX sh, executable): builds fixture archive +
      checksums, serves via `python3 -m http.server 0`, asserts happy path + mismatch abort; prints
      `SMOKE PASS`.
- [ ] `.github/workflows/ci.yml` has a 7th job `asdf-plugin-smoke` (`shellcheck -s sh` the 3 scripts
      + `sh test/smoke.sh`); the 6 existing jobs are unchanged.
- [ ] `shellcheck -s sh bin/list-all bin/install test/smoke.sh` → clean.
- [ ] `git status --porcelain` shows ONLY the `plugins/asdf-stagecoach/` tree + `M .github/workflows/ci.yml`.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — the verified asdf/mise env-var contract (`ASDF_INSTALL_TYPE`/`VERSION`/`PATH`
+ the `$ASDF_INSTALL_PATH/bin/` rule, live-confirmed), the exact `bin/list-all` + `bin/install`
bodies (copy-pasteable POSIX sh), the exact goreleaser asset/checksum naming (verified against
`.goreleaser.yaml` + `internal/upgrade/download.go`), the portable SHA256 idiom, the uname map, the
mise asdf-compatibility proof (live URL), the full smoke-test design (mirroring the npm wrapper's
`install.test.cjs`), the CI job YAML, and the local validation commands are all below. The scripts
are the same shape the npm wrapper's `install.cjs` already ships (download+verify+extract), so the
implementer has a working in-repo reference.

### Documentation & References

```yaml
# MUST READ — the verified research for THIS item (the contract, the script bodies, the smoke test, the CI job).
- docfile: plan/017_397abce9deb1/P2M4T1S1/research/findings.md
  why: "§1 the asdf env-var contract (ASDF_INSTALL_TYPE=version|ref, ASDF_INSTALL_PATH/bin/ rule);
        §2 mise asdf-compat (same scripts work for both; mise sets the ASDF_* vars); §3 git ls-remote
        for list-all (no rate limit) + the exact strip/sort pipeline; §4 the uname map; §5 the
        portable SHA256 idiom + the awk field-2 exact-match checksum parse; §6 the codebase asset/
        checksum naming + the DIRECT download URL + the STAGECOACH_DOWNLOAD_BASE test hook; §7 the
        smoke-test design; §8 the CI job; §9 sibling coordination."

# MUST READ — the authoritative spec (the asdf plugin shape + the mise-compat note).
- docfile: plan/017_397abce9deb1/architecture/external_deps.md
  section: "§6 mise / asdf plugins (PRD §21.2) — 'asdf plugin (bin/install + bin/list-all): list-all
            greps GitHub release tags; install downloads the platform archive + extracts to
            $ASDF_INSTALL_PATH/bin'; 'mise plugin (compatible with asdf plugins; bin/install +
            bin/list-all): same shape; mise plugins can be a plain asdf plugin'; 'Verify at impl:
            current asdf/mise plugin contract (ASDF_INSTALL_TYPE/VERSION/INSTALL_PATH env vars;
            mise's MISE_* equivalents)'."
  critical: "§6 is the contract: bin/list-all + bin/install; extract to $ASDF_INSTALL_PATH/bin; mise
             runs the same asdf plugin. The 'Verify at impl' duty is the FR-D5 gate (record the date)."

# VERIFIED (web, FR-D5) — the asdf plugin author contract (env vars + the bin/ rule).
- url: https://asdf-vm.com/plugins/create.html
  why: "asdf's 'Create a Plugin' page. A plugin = git repo with bin/list-all + bin/install. bin/install
        is called once with ASDF_INSTALL_TYPE (version|ref), ASDF_INSTALL_VERSION, ASDF_INSTALL_PATH
        in the env; the plugin creates $ASDF_INSTALL_PATH/bin/ and puts the executable there. bin/
        list-all prints versions to stdout (errors to stderr). lib/utils.sh is OPTIONAL."
  critical: "THE load-bearing facts: ASDF_INSTALL_TYPE ∈ {version, ref}; the plugin MUST create
             $ASDF_INSTALL_PATH/bin/ and place the binary there (asdf shims resolve there)."

# VERIFIED (web, FR-D5) — mise runs asdf plugins UNCHANGED, setting the SAME ASDF_* env vars.
- url: https://mise.jdx.dev/asdf-legacy-plugins.html
  why: "'asdf plugins have access to these environment variables: ASDF_INSTALL_TYPE - version or ref;
        ASDF_INSTALL_VERSION - Version number or git ref…' → mise sets the SAME ASDF_* vars when
        running an asdf plugin's scripts. So ONE set of scripts serves both asdf and mise."
  critical: "this is the proof the same bin/list-all + bin/install work for mise with NO changes.
             (Also see https://mise.jdx.dev/plugins.html: 'mise can use asdf's plugin ecosystem…
             plugins contain shell scripts like bin/install'.)"

# VERIFIED (web, FR-D5) — why list-all uses git ls-remote, NOT the GitHub Releases API.
- url: https://docs.github.com/en/rest/overview/resources-in-the-rest-api#rate-limiting
  why: "the unauthenticated GitHub REST API is rate-limited to 60 req/h/IP. list-all runs on every
        listing/tab-complete → easily blown. git ls-remote --refs --tags has NO per-IP quota."
  critical: "use git ls-remote for list-all; reserve the API (with a token) for nothing here."

# VERIFIED (codebase) — the closest in-repo analog: download + SHA256-verify + extract (JS twin).
- file: npm/install.cjs
  why: "the npm wrapper's postinstall downloader does EXACTLY what bin/install must do (download
        archive + checksums from the DIRECT releases/download URL, SHA256-verify constant-time,
        extract the single root-level binary, abort-before-write on mismatch). Mirror its
        assetName/checksumsName naming, its DIRECT-URL (not API) decision, and its
        STAGECOACH_DOWNLOAD_BASE test-override env var. It documents the 'GitHub 302-redirects;
        curl -fL follows' and 'binary ships at archive root' facts bin/install relies on."
  pattern: "download → parse checksums → verify → extract → (mismatch → remove partial, abort)."
  gotcha: "the archive's single entry is the binary at the ROOT (goreleaser builds.binary='stagecoach')
           → extract INTO $ASDF_INSTALL_PATH/bin with NO member filter (like install.cjs into destDir)."

# VERIFIED (codebase) — the Go reference for the exact asset/checksum naming (the sh script must match).
- file: internal/upgrade/download.go
  why: "assetName(tag,goos,goarch) = 'stagecoach_' + TrimPrefix(tag,'v') + '_' + goos + '_' + goarch
        + '.zip'(windows)|'.tar.gz'(else); checksumsName(tag) = 'stagecoach_'+TrimPrefix(tag,'v')+
        '_checksums.txt'. bin/install must reproduce this EXACTLY (version WITHOUT leading v in the
        filename; tag WITH leading v in the download URL path). SelectAsset + FetchChecksums +
        VerifySHA256 + DownloadAndVerifyArchive show the verify-then-extract + abort-before-write
        discipline the sh script mirrors."
  gotcha: "checksums.txt is '<64hex>  <filename>' (TWO spaces). The Go code uses strings.Fields
           (collapses whitespace); the sh script uses awk (same effect)."

# VERIFIED (codebase) — the goreleaser config that PRODUCES the assets bin/install consumes.
- file: .goreleaser.yaml
  why: "archives.name_template = '{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}' (Version
        = tag WITHOUT leading v); format tar.gz, format_override windows→zip; builds.binary=
        'stagecoach' (binary at archive root); checksum.name_template = '{{ .ProjectName }}_{{ .Version
        }}_checksums.txt', algorithm sha256; goos {linux,darwin,windows} × goarch {amd64,arm64} (6
        targets, no 32-bit). release.github.owner=dabstractor, name=stagecoach. Do NOT edit this file."

# VERIFIED (codebase) — the smoke-test pattern to mirror (the npm wrapper's fixture-server harness).
- file: npm/test/install.test.cjs
  why: "the EXACT pattern bin/install's smoke test mirrors: build a fixture archive + checksums at
        runtime (no committed binaries), serve via a local http server, run install against it
        (STAGECOACH_DOWNLOAD_BASE → the local server), assert happy path (download+verify+extract+
        executable) AND mismatch path (non-zero exit + NO binary left behind = abort-before-write)."

# VERIFIED (codebase) — the CI file this task ADDS a job to (READ-ONLY context + the job-header style).
- file: .github/workflows/ci.yml
  why: "6 existing jobs (build-test, lint, vulncheck, coverage, npm-smoke, nix-flake-check). This task
        ADDS a 7th asdf-plugin-smoke job. Match the file's conventions: actions/checkout@v4, named
        steps, the '# --- (N) ... ---' job-header comment style, timeout-minutes. NOTE the nix-flake-
        check job (P2.M3.T1.S1) is added concurrently — both are independent additive jobs, no collision."

# REFERENCE — the GitHub-Releases download URL shape (bin/install builds the SAME URL).
- url: https://github.com/dabstractor/stagecoach/releases
  why: "the releases/download/<tag>/<asset> URL pattern (DIRECT, no API). The download URL is
        https://github.com/dabstractor/stagecoach/releases/download/v<version>/stagecoach_<version>_<os>_<arch>.tar.gz
        (tag WITH v; filename version WITHOUT v). 302-redirects to objects.githubusercontent.com;
        curl -fL follows."

# CONTRACT (parallel sibling) — P2.M3.T1.S1 (Nix flake) shares ONLY ci.yml (additive jobs, no collision).
- docfile: plan/017_397abce9deb1/P2M3T1S1/PRP.md
  why: "the Nix task ADDS the nix-flake-check job to ci.yml; THIS task ADDS the asdf-plugin-smoke job.
        The two jobs are independent additive entries in the same `jobs:` map — no collision. Neither
        task touches the other's files (flake.nix vs plugins/asdf-stagecoach/*)."
```

### Current Codebase tree (relevant slice)

```bash
.goreleaser.yaml                          # archives/checksum naming + the 6 targets bin/install consumes
internal/upgrade/download.go              # assetName()/checksumsName() — the Go reference for exact names
npm/install.cjs                           # the JS twin: download+SHA256-verify+extract (mirror its shape)
npm/test/install.test.cjs                 # the fixture-server smoke-test pattern to mirror in POSIX sh
.github/workflows/ci.yml                  # ADD the `asdf-plugin-smoke` job here (7th; nix job is 6th)
plan/017_397abce9deb1/architecture/external_deps.md   # §6 (the asdf/mise plugin spec)
plan/017_397abce9deb1/P2M4T1S1/research/findings.md   # THIS item's verified research
# NOT present yet: plugins/asdf-stagecoach/ (bin/list-all, bin/install, README.md, test/smoke.sh)
```

### Desired Codebase tree with files to be added/edited

```bash
plugins/asdf-stagecoach/bin/list-all      # CREATED (POSIX sh, executable) — git ls-remote → sort -V
plugins/asdf-stagecoach/bin/install       # CREATED (POSIX sh, executable) — download+verify+extract
plugins/asdf-stagecoach/README.md         # CREATED (Mode A docs) — asdf/mise install + mirror note
plugins/asdf-stagecoach/test/smoke.sh     # CREATED (POSIX sh, executable) — fixture-archive smoke test
.github/workflows/ci.yml                  # MODIFIED — + asdf-plugin-smoke job (7th)
# NOT touched: any .go, .goreleaser.yaml, npm/*, release.yml, go.mod/go.sum, flake.*, .gitignore,
#              docs/packaging.md, top-level README.md (install line is Mode B / P3), PRD.md, plan/**, tasks.json
```

### Known Gotchas of our codebase & Library Quirks

```sh
# GOTCHA 1 (ASDF_INSTALL_TYPE is "version" or "ref"; the plugin READS env, takes NO args). bin/install
#   is called by asdf/mise with ASDF_INSTALL_TYPE/VERSION/PATH already set. It does NOT parse argv.
#   Use the `: "${ASDF_INSTALL_TYPE:?msg}"` idiom to require + fail loudly on a missing var. (Normal
#   `asdf install stagecoach <ver>` → TYPE=version; `ref:<gitref>` → TYPE=ref. We only need VERSION +
#   PATH for a release-tarball plugin; TYPE is read for completeness/future-proofing.)

# GOTCHA 2 (the plugin MUST create $ASDF_INSTALL_PATH/bin/ and place the binary there). asdf shims
#   everything in $ASDF_INSTALL_PATH/bin/; `which stagecoach` resolves there. So: mkdir -p
#   "$ASDF_INSTALL_PATH/bin"; extract the binary INTO it (it lands at $ASDF_INSTALL_PATH/bin/stagecoach).
#   This is the single most important contract rule. Missing it = "command not found" after install.

# GOTCHA 3 (version WITHOUT leading v in filenames; tag WITH v in the download URL). goreleaser
#   {{.Version}} = the tag without the leading v (download.go assetName: TrimPrefix(tag,"v")). So the
#   archive is stagecoach_1.0.0_linux_amd64.tar.gz but the download URL is .../releases/download/v1.0.0/
#   stagecoach_1.0.0_linux_amd64.tar.gz. ASDF_INSTALL_VERSION is the bare version (1.0.0); the URL
#   prefix is v$ASDF_INSTALL_VERSION. DO NOT double-prefix.

# GOTCHA 4 (DIRECT download URL, NOT the GitHub API). The unauthenticated API is 60 req/h/IP and
#   breaks installs at scale (external_deps.md §1; npm/install.cjs DECISION note). The direct URL
#   .../releases/download/<tag>/<asset> has no such limit, needs no auth (public repo), and 302-
#   redirects to objects.githubusercontent.com (curl -fL follows). Same decision as the npm wrapper.

# GOTCHA 5 (the binary ships at the archive ROOT; extract INTO bin/ with NO member filter).
#   .goreleaser.yaml builds.binary='stagecoach' → the archive's single entry is ./stagecoach at the
#   root. `tar -xzf "$archive" -C "$ASDF_INSTALL_PATH/bin"` extracts it to
#   $ASDF_INSTALL_PATH/bin/stagecoach. Do NOT pass a member name (the entry is `./stagecoach` or
#   `stagecoach` depending on the archiver; extracting the whole archive is correct + matches
#   npm/install.cjs which extracts into destDir without a member filter).

# GOTCHA 6 (portable SHA256: sha256sum on Linux/coreutils, shasum -a 256 on macOS/BSD). Detect with
#   `command -v`; wrap in a function to avoid word-splitting. Do NOT assume `sha256sum` exists on
#   macOS (it does NOT — macos has `shasum`). The smoke test must ALSO use this detection (it builds
#   the fixture checksum on the host).

# GOTCHA 7 (checksums.txt is "<64hex>  <filename>" with TWO spaces; use awk field-2 EXACT match, not
#   grep regex). `.` in the asset name is a regex wildcard; `1.2.3` substring-matches `1.2.30`. Use
#   `awk -v f="$asset" '$2==f {print $1; exit}'` (awk's default whitespace field-splitting collapses
#   the two-space gap; $2==f is a literal string compare). Matches download.go's strings.Fields parse.

# GOTCHA 8 (git ls-remote --refs --tags, NOT --tags alone; --refs excludes the peeled ^{} duplicates).
#   Without --refs you get both `refs/tags/v1.2.3` and `refs/tags/v1.2.3^{}` lines. Output format is
#   `<40-hex-sha>\trefs/tags/<tag>` (TAB-separated). Strip with `sed 's#^.*refs/tags/##'`.

# GOTCHA 9 (sort -V ascending so newest is LAST). asdf does not hard-enforce order, but ascending is
#   the convention older asdf used to infer "latest" from the last line. `sort -V` gives correct
#   1.2.9 < 1.2.10 ordering; it ships in GNU coreutils AND modern macOS/BSD sort (both supported
#   platforms). Optionally add bin/latest-stable later (NOT required for this subtask).

# GOTCHA 10 (asdf/mise are Unix-only → windows/zip is OUT of scope). The plugin supports macOS + Linux
#   on amd64/arm64 only. bin/install's uname map rejects anything else with a clear stderr message +
#   exit 1. Windows users use Scoop/Winget (PRD §21.3) — NOT this plugin. Do NOT add a windows/zip
#   branch to bin/install.

# GOTCHA 11 (set -eu in POSIX sh; guard command-substitution failures). With `set -e`, a failing
#   command inside `$(...)` may not abort in all sh implementations — capture into a var and test it
#   explicitly (e.g. `got="$(sha256 "$f")"; [ -n "$got" ] || exit 1`). The `: "${VAR:?msg}"` idiom
#   works WITH set -u (it both requires + emits the message). Prefer `command -v x >/dev/null 2>&1`
#   over `which x` (which is non-POSIX).

# GOTCHA 12 (the STAGECOACH_DOWNLOAD_BASE test hook is shared with the npm wrapper). npm/install.cjs
#   already defines STAGECOACH_DOWNLOAD_BASE as the "prefix for releases/download" override. bin/install
#   reuses the SAME env var name for (a) cross-surface consistency and (b) so the smoke test redirects
#   downloads to the local python3 server. It is a TEST/MIRROR hook, not a user-facing option —
#   document it in the script header, not the README.
```

## Implementation Blueprint

### Data models and structure
None — this is POSIX-sh scripts + Markdown + CI YAML. The "data" is the `bin/list-all` pipeline, the
`bin/install` download/verify/extract flow, the smoke-test fixture, and the CI job.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE plugins/asdf-stagecoach/bin/list-all (POSIX sh, executable)
  - CREATE plugins/asdf-stagecoach/bin/list-all. Body (POSIX sh):
        #!/usr/bin/env sh
        # list-all — enumerate stagecoach versions for asdf/mise (P2.M4.T1.S1; external_deps.md §6).
        # Prints release tags (bare versions, leading 'v' stripped) to STDOUT, ascending via sort -V
        # (newest last). Uses `git ls-remote --refs --tags` (no GitHub-API 60-req/h rate limit).
        set -eu
        GIT_REPO="${STAGECOACH_GIT_REPO:-https://github.com/dabstractor/stagecoach.git}"
        git ls-remote --refs --tags "$GIT_REPO" 2>/dev/null \
          | sed 's#^.*refs/tags/##' \   # "<sha>\trefs/tags/v1.2.3" -> "v1.2.3"
          | sed 's/^v//' \              # strip a leading "v" -> "1.2.3"
          | grep -E '^[0-9]' \          # keep only version-like tags (defensive; drop junk)
          | sort -V                     # ascending: 1.2.9 -> 1.2.10
  - `chmod +x` the file. Header comment cites external_deps.md §6 + the git-ls-remote rationale.
  - FOLLOW pattern: research/findings.md §3 (the exact pipeline) + the asdf list-all contract (§1).
  - NAMING: `bin/list-all` (asdf convention — the dash is required). `#!/usr/bin/env sh` (POSIX, not bash).
  - PLACEMENT: plugins/asdf-stagecoach/bin/list-all.
  - GOTCHAS: 8 (--refs), 9 (sort -V ascending). Do NOT print a header/blank line.

Task 2: CREATE plugins/asdf-stagecoach/bin/install (POSIX sh, executable)
  - CREATE plugins/asdf-stagecoach/bin/install. Body (POSIX sh) — see Implementation Patterns for the
    full copy-pasteable script. Outline:
      (0) `#!/usr/bin/env sh` + `set -eu`; header comment (env-var contract + asset-naming + the
          DIRECT-URL/STAGECOACH_DOWNLOAD_BASE decisions + abort-before-write).
      (1) Require env: `: "${ASDF_INSTALL_TYPE:?…}"; : "${ASDF_INSTALL_VERSION:?…}"; : "${ASDF_INSTALL_PATH:?…}"`.
      (2) Compute version, repo-owner/name (defaults dabstractor/stagecoach), base URL
          (STAGECOACH_DOWNLOAD_BASE override else https://github.com/<o>/<r>/releases/download).
      (3) uname→os/arch map (Darwin→darwin, Linux→linux; x86_64|amd64→amd64, aarch64|arm64→arm64;
          reject others → stderr + exit 1). GOTCHA 10.
      (4) asset="stagecoach_${version}_${os}_${arch}.tar.gz"; sums="stagecoach_${version}_checksums.txt".
          url="${base}/v${version}/${asset}"; sums_url="${base}/v${version}/${sums}". (GOTCHA 3: v in
          URL, no-v in filename.)
      (5) tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT. Download asset + sums via `curl -fsSL -o`.
      (6) Portable SHA256: detect sha256sum|shasum -a 256 → sha256() function. GOTCHA 6.
      (7) expected="$(awk -v f="$asset" '$2==f {print $1; exit}' "$tmp/$sums")"; require non-empty;
          got="$(sha256 "$tmp/$asset")"; compare; mismatch → stderr + exit 1 (abort BEFORE extraction).
          GOTCHA 7.
      (8) mkdir -p "$ASDF_INSTALL_PATH/bin"; tar -xzf "$tmp/$asset" -C "$ASDF_INSTALL_PATH/bin"
          (NO member filter — binary ships at archive root). GOTCHA 5. chmod +x the binary.
      (9) printf 'stagecoach: installed %s to %s/bin/stagecoach\n' >&2.
  - `chmod +x` the file.
  - FOLLOW pattern: research/findings.md §6 + npm/install.cjs (download→verify→extract→abort-on-mismatch)
    + internal/upgrade/download.go (exact assetName/checksumsName).
  - NAMING: `bin/install` (asdf convention). `#!/usr/bin/env sh`.
  - PLACEMENT: plugins/asdf-stagecoach/bin/install.
  - GOTCHAS: 1 (env read), 2 (create $ASDF_INSTALL_PATH/bin), 3 (v/no-v), 4 (DIRECT url), 5 (root
    extract), 6 (portable sha256), 7 (awk field-2), 10 (Unix-only), 11 (set -eu).

Task 3: CREATE plugins/asdf-stagecoach/README.md (Mode A docs)
  - CREATE plugins/asdf-stagecoach/README.md. Sections (see research/findings.md §1 task notes):
      - Title: "# stagecoach — asdf / mise plugin" + one-line what-it-is.
      - "## Prerequisites": asdf OR mise; git; curl; tar; awk; a sha256 tool (sha256sum OR shasum).
      - "## Install (asdf)":
            asdf plugin add stagecoach https://github.com/dabstractor/asdf-stagecoach.git
            asdf install stagecoach latest
            asdf global  stagecoach latest
            stagecoach --version
      - "## Install (mise)" (same scripts work — mise runs asdf plugins):
            mise plugin add stagecoach https://github.com/dabstractor/asdf-stagecoach.git
            mise install stagecoach@latest
            mise use -g stagecoach@latest
            stagecoach --version
      - "## Supported platforms": macOS + Linux on amd64/arm64. (Windows is NOT supported via this
        plugin — use Scoop or Winget: https://github.com/dabstractor/stagecoach#install.)
      - "## How it works": bin/list-all uses `git ls-remote --refs --tags` (no GitHub-API rate limit);
        bin/install reads ASDF_INSTALL_TYPE/VERSION/PATH (mise sets the SAME vars), downloads the
        goreleaser `.tar.gz` + checksums from the project's GitHub Releases, SHA256-verifies, and
        extracts the single `stagecoach` binary to `$ASDF_INSTALL_PATH/bin/`.
      - "## Repository / canonical source": the scripts under
        `github.com/dabstractor/stagecoach/tree/main/plugins/asdf-stagecoach` are the CANONICAL source;
        they are MIRRORED to the plugin repo `github.com/dabstractor/asdf-stagecoach` (the URL `asdf
        plugin add` / `mise plugin add` points at). Changes are made in the stagecoach repo first.
      - "## License": match the stagecoach repo (see the main README).
  - FOLLOW pattern: research/findings.md §6 (codebase asset facts) + the asdf-nodejs/asdf-golang
    README shape (findings §1 refs).
  - PLACEMENT: plugins/asdf-stagecoach/README.md.
  - NOTE: do NOT edit the top-level README.md install section here — that's Mode B / P3.M1.T1.S1.

Task 4: CREATE plugins/asdf-stagecoach/test/smoke.sh (POSIX sh, executable) — fixture-archive smoke test
  - CREATE plugins/asdf-stagecoach/test/smoke.sh. Body (POSIX sh) — see Implementation Patterns. It:
      (1) Detects portable sha256 (GOTCHA 6 — the host may be macos: shasum, not sha256sum).
      (2) Detects host os/arch (SAME map as bin/install) for the fixture archive name.
      (3) Builds a fake `stagecoach` (`#!/bin/sh\necho fake-stagecoach`, chmod +x), `tar -czf` into
          $serve/v0.0.0/stagecoach_0.0.0_<os>_<arch>.tar.gz; writes
          $serve/v0.0.0/stagecoach_0.0.0_checksums.txt = "<realhash>  <asset>".
      (4) Serves $serve via `python3 -m http.server 0` (background; scrape the assigned port from
          stderr with a retry loop); STAGECOACH_DOWNLOAD_BASE=http://127.0.0.1:<port>.
      (5) HAPPY: ASDF_INSTALL_TYPE=version ASDF_INSTALL_VERSION=0.0.0 ASDF_INSTALL_PATH=<tmp1>
          STAGECOACH_DOWNLOAD_BASE=<base> sh ../bin/install → assert exit 0; assert
          <tmp1>/bin/stagecoach exists, is executable (mode & 0o111), and runs (prints fake-stagecoach).
      (6) MISMATCH: rewrite checksums with `printf '%064d' 0` (64 zeros); ASDF_INSTALL_PATH=<tmp2>;
          run install → assert NON-ZERO exit; assert stderr mentions the mismatch; assert NO binary
          left at <tmp2>/bin/stagecoach (abort-before-write).
      (7) trap cleanup: rm -rf $serve <tmp1> <tmp2>; kill the http server. print SMOKE PASS.
  - `chmod +x` the file.
  - FOLLOW pattern: npm/test/install.test.cjs (the exact fixture-server + happy/mismatch structure,
    translated to POSIX sh + python3 http.server instead of node:http).
  - PLACEMENT: plugins/asdf-stagecoach/test/smoke.sh.
  - GOTCHA: python3 -m http.server prints the port to STDERR; `2>&1` into a log; scrape with
    `grep -oE 'port [0-9]+' | grep -oE '[0-9]+'` in a retry loop (the bind is async). python3 ships
    on ubuntu-latest + macos-latest (both CI runners). Use `printf '%064d' 0` for the wrong hash
    (NOT `seq`). Guard the trap with `${var:-}` since tmp1/tmp2 are assigned mid-script.

Task 5: MODIFY .github/workflows/ci.yml — add the `asdf-plugin-smoke` job (7th)
  - PRESERVE: all 6 existing jobs (build-test, lint, vulncheck, coverage, npm-smoke, nix-flake-check),
    the triggers, permissions, concurrency block. This task ADDS a 7th job.
  - ADD (match the `# --- (N) ... ---` job-header comment style + actions/checkout@v4 convention):
        # --- (7) asdf/mise plugin smoke (PRD §21.2/G27; external_deps.md §6) ---------------------
        # shellchecks the 3 POSIX-sh plugin scripts (catches bashisms) + runs the fixture-archive
        # smoke test (build a fake stagecoach archive, serve locally, run bin/install, assert the
        # happy path + the SHA256-mismatch abort-before-write). Keeps the plugin scripts from rotting
        # on every PR (parity with npm-smoke + nix-flake-check). ubuntu-latest ships shellcheck +
        # python3 (FR-D5: re-confirm at impl).
        asdf-plugin-smoke:
          name: asdf/mise plugin smoke
          runs-on: ubuntu-latest
          timeout-minutes: 10
          steps:
            - uses: actions/checkout@v4
            - name: shellcheck (POSIX sh plugin scripts)
              run: |
                shellcheck -s sh plugins/asdf-stagecoach/bin/list-all
                shellcheck -s sh plugins/asdf-stagecoach/bin/install
                shellcheck -s sh plugins/asdf-stagecoach/test/smoke.sh
            - name: smoke test (fixture archive install)
              working-directory: plugins/asdf-stagecoach
              run: sh test/smoke.sh
  - FR-D5: re-confirm ubuntu-latest still ships `shellcheck` (it is in the runner's pre-installed
    tool list) + `python3` (it is). If shellcheck is missing, fall back to the `ludeeus/action-
    shellcheck` action (note it in the PR — do NOT silently change the approach).
  - NOTE: P2.M3.T1.S1 adds the nix-flake-check job concurrently. Both jobs are independent additive
    entries in `jobs:` — order them by number (nix=6, asdf=7); no collision.

Task 6: VALIDATE — shellcheck + smoke + ci.yml parse + git scope guard (see Validation Loop)
  - RUN: the Level 1-4 commands. EXPECT: shellcheck clean; smoke prints SMOKE PASS (locally + the
    fixture-server path); ci.yml is valid YAML with the asdf-plugin-smoke job; the 6 existing jobs
    intact; git status shows ONLY plugins/asdf-stagecoach/ + M .github/workflows/ci.yml.
  - GATE: every check must pass. Record the FR-D5 verification date (the asdf/mise env-var contract
    was re-checked against the live docs) in the subtask completion.
```

### Implementation Patterns & Key Details

```sh
# PATTERN — bin/list-all (copy-pasteable; see Task 1):
#!/usr/bin/env sh
# list-all — enumerate stagecoach versions for asdf/mise. (P2.M4.T1.S1; external_deps.md §6)
# Prints bare versions (leading 'v' stripped), ascending via sort -V (newest last), one per line.
# Uses `git ls-remote --refs --tags` (no GitHub-API 60-req/h/IP rate limit — see findings §3).
set -eu
GIT_REPO="${STAGECOACH_GIT_REPO:-https://github.com/dabstractor/stagecoach.git}"
git ls-remote --refs --tags "$GIT_REPO" 2>/dev/null \
  | sed 's#^.*refs/tags/##' \
  | sed 's/^v//' \
  | grep -E '^[0-9]' \
  | sort -V
# CRITICAL: --refs (no peeled duplicates); strip refs/tags/ THEN the leading v; sort -V ascending.
```

```sh
# PATTERN — bin/install (copy-pasteable; see Task 2):
#!/usr/bin/env sh
# install — download + SHA256-verify + extract a stagecoach release into $ASDF_INSTALL_PATH/bin.
# (P2.M4.T1.S1; external_deps.md §6) Called by asdf/mise with ASDF_INSTALL_TYPE (version|ref),
# ASDF_INSTALL_VERSION, ASDF_INSTALL_PATH in the env (mise sets the SAME vars → works for both).
# Abort-before-write: on checksum/platform failure it exits non-zero with NO binary left behind
# (mirrors internal/upgrade/download.go DownloadAndVerifyArchive + npm/install.cjs).
# STAGECOACH_DOWNLOAD_BASE overrides the releases/download prefix (TEST/MIRROR hook; not user-facing).
set -eu

: "${ASDF_INSTALL_TYPE:?bin/install: ASDF_INSTALL_TYPE is required}"
: "${ASDF_INSTALL_VERSION:?bin/install: ASDF_INSTALL_VERSION is required}"
: "${ASDF_INSTALL_PATH:?bin/install: ASDF_INSTALL_PATH is required}"

version="$ASDF_INSTALL_VERSION"
owner="${STAGECOACH_REPO_OWNER:-dabstractor}"
name="${STAGECOACH_REPO_NAME:-stagecoach}"
base="${STAGECOACH_DOWNLOAD_BASE:-https://github.com/${owner}/${name}/releases/download}"

# uname -> goreleaser GOOS/GOARCH (asdf/mise are Unix-only → windows/zip is out of scope).
case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux)  os=linux ;;
  *) printf 'stagecoach: unsupported OS: %s (plugin supports macOS + Linux)\n' "$(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) printf 'stagecoach: unsupported arch: %s (builds amd64/arm64 only)\n' "$(uname -m)" >&2; exit 1 ;;
esac

# goreleaser asset + checksums names (version WITHOUT leading v; URL tag WITH v). Matches
# internal/upgrade/download.go assetName()/checksumsName() + .goreleaser.yaml name_template.
asset="stagecoach_${version}_${os}_${arch}.tar.gz"
sums="stagecoach_${version}_checksums.txt"
asset_url="${base}/v${version}/${asset}"
sums_url="${base}/v${version}/${sums}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

printf 'stagecoach: installing %s (%s/%s)\n' "$version" "$os" "$arch" >&2

# Download archive + checksums from the DIRECT releases/download URL (curl -fL follows the 302).
curl -fsSL -o "${tmp}/${asset}" "$asset_url"
curl -fsSL -o "${tmp}/${sums}"   "$sums_url"

# Portable SHA256 (sha256sum on Linux/coreutils, shasum -a 256 on macOS/BSD).
if command -v sha256sum >/dev/null 2>&1; then
  sha256() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
  sha256() { shasum -a 256 "$1" | awk '{print $1}'; }
else
  printf 'stagecoach: need sha256sum or shasum to verify the download\n' >&2; exit 1
fi

# checksums.txt line is "<64hex>  <asset>" (TWO spaces). awk field-2 EXACT match avoids regex pitfalls.
expected="$(awk -v f="$asset" '$2==f {print $1; exit}' "${tmp}/${sums}")"
[ -n "$expected" ] || { printf 'stagecoach: no checksum line for %s in %s\n' "$asset" "$sums" >&2; exit 1; }
got="$(sha256 "${tmp}/${asset}")"
[ "$expected" = "$got" ] || {
  printf 'stagecoach: SHA256 mismatch for %s\n  expected: %s\n  got:      %s\n' "$asset" "$expected" "$got" >&2
  exit 1
}

# Extract the single root-level binary INTO $ASDF_INSTALL_PATH/bin (asdf shims resolve there).
mkdir -p "${ASDF_INSTALL_PATH}/bin"
tar -xzf "${tmp}/${asset}" -C "${ASDF_INSTALL_PATH}/bin"
chmod +x "${ASDF_INSTALL_PATH}/bin/stagecoach"

printf 'stagecoach: installed %s to %s/bin/stagecoach\n' "$version" "$ASDF_INSTALL_PATH" >&2
# CRITICAL: (1) read env, no argv; (2) create $ASDF_INSTALL_PATH/bin + place binary there;
# (3) v in URL / no-v in filename; (4) DIRECT url (curl -fL follows 302); (5) extract whole archive
# into bin/ (binary is the sole root entry); (6) portable sha256; (7) awk field-2 match; abort on
# any failure BEFORE extraction (no binary left behind).
```

```sh
# PATTERN — test/smoke.sh (sketch; full body in Task 4). Mirrors npm/test/install.test.cjs.
#!/usr/bin/env sh
# smoke.sh — local smoke test for bin/install against a fixture archive. (P2.M4.T1.S1)
set -eu
fail() { printf 'SMOKE FAIL: %s\n' "$*" >&2; exit 1; }

# portable sha256 (host may be macos)
if command -v sha256sum >/dev/null 2>&1; then sha256() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then sha256() { shasum -a 256 "$1" | awk '{print $1}'; }
else fail 'smoke: need sha256sum or shasum'; fi

# host os/arch (SAME map as bin/install) for the fixture archive name
case "$(uname -s)" in Darwin) os=darwin;; Linux) os=linux;; *) fail "unsupported host OS";; esac
case "$(uname -m)" in x86_64|amd64) arch=amd64;; aarch64|arm64) arch=arm64;; *) fail "unsupported host arch";; esac

version=0.0.0
asset="stagecoach_${version}_${os}_${arch}.tar.gz"
sums="stagecoach_${version}_checksums.txt"

serve="$(mktemp -d)"; tmp1=""; tmp2=""; srvpid=""
trap 'rm -rf "${serve:-}" "${tmp1:-}" "${tmp2:-}"; kill "${srvpid:-}" 2>/dev/null || true' EXIT
mkdir -p "${serve}/v${version}"
payload="$(mktemp -d)"
printf '#!/bin/sh\necho fake-stagecoach\n' > "${payload}/stagecoach"; chmod +x "${payload}/stagecoach"
tar -czf "${serve}/v${version}/${asset}" -C "$payload" stagecoach
printf '%s  %s\n' "$(sha256 "${serve}/v${version}/${asset}")" "$asset" > "${serve}/v${version}/${sums}"

# local http server; scrape the assigned port from stderr
( cd "$serve" && exec python3 -m http.server 0 ) >"${serve}/srv.log" 2>&1 & srvpid=$!
port=""
i=0; while [ $i -lt 50 ]; do
  port="$(grep -oE 'port [0-9]+' "${serve}/srv.log" 2>/dev/null | head -1 | grep -oE '[0-9]+')"
  [ -n "$port" ] && break; i=$((i+1)); sleep 0.1
done
[ -n "$port" ] || fail "http server did not start ($(cat "${serve}/srv.log"))"
base="http://127.0.0.1:${port}"
HERE="$(cd "$(dirname "$0")" && pwd)"

# HAPPY PATH
tmp1="$(mktemp -d)"
ASDF_INSTALL_TYPE=version ASDF_INSTALL_VERSION="$version" ASDF_INSTALL_PATH="$tmp1" \
STAGECOACH_DOWNLOAD_BASE="$base" sh "${HERE}/../bin/install" || fail "happy path: install exited non-zero"
[ -x "${tmp1}/bin/stagecoach" ] || fail "happy path: binary not installed/executable"
"${tmp1}/bin/stagecoach" | grep -q fake-stagecoach || fail "happy path: binary did not run"

# MISMATCH PATH (abort-before-write)
printf '%064d  %s\n' 0 "$asset" > "${serve}/v${version}/${sums}"
tmp2="$(mktemp -d)"
if ASDF_INSTALL_TYPE=version ASDF_INSTALL_VERSION="$version" ASDF_INSTALL_PATH="$tmp2" \
   STAGECOACH_DOWNLOAD_BASE="$base" sh "${HERE}/../bin/install" 2>"${serve}/mismatch.err"; then
  fail "mismatch path: install should have exited non-zero on a bad checksum"
fi
grep -qi 'mismatch' "${serve}/mismatch.err" || fail "mismatch path: stderr did not mention mismatch"
[ ! -e "${tmp2}/bin/stagecoach" ] || fail "mismatch path: a binary was left behind (abort-before-write violated)"

printf 'SMOKE PASS\n'
# CRITICAL: portable sha256 on the host; v0.0.0 fixture served at $serve/v0.0.0/ (matches the URL
# shape ${base}/v${version}/...); python3 http.server port-scrape retry; mismatch must leave NO binary.
```

### Integration Points

```yaml
CONSUMES (READ-ONLY):
  - .goreleaser.yaml: produces stagecoach_<v>_<os>_<arch>.tar.gz + stagecoach_<v>_checksums.txt for
    linux/darwin/windows × amd64/arm64. bin/install names assets to match; NOT edited.
  - GitHub Releases (dabstractor/stagecoach): the DIRECT releases/download URL bin/install fetches;
    list-all enumerates tags via git ls-remote. No API, no auth, no token.
PRODUCES:
  - plugins/asdf-stagecoach/{bin/list-all,bin/install,README.md,test/smoke.sh}: the canonical asdf/
    mise plugin (mirrored to the separate dabstractor/asdf-stagecoach repo, out-of-band).
  - ci.yml asdf-plugin-smoke job: shellcheck + fixture smoke on every PR (keeps the scripts honest).
NO Go / goreleaser / npm / release.yml / flake / config changes. NO top-level README install edit
  (that is Mode B / P3.M1.T1.S1). NO .gitignore change (no build artifacts). NO docs/packaging.md
  change (Mode A docs = the plugin's own README).
```

## Validation Loop

> **This is a POSIX-sh + CI-YAML + Markdown task.** The template's `ruff`/`mypy`/`pytest` gates do NOT
> apply. Validation = `shellcheck` (POSIX-sh lint) + `test/smoke.sh` (the fixture-archive smoke test) +
> a ci.yml YAML parse + a git scope guard. `go build ./...` is a no-op here (no Go changes) — skip it.

### Level 1: shellcheck (Immediate Feedback — POSIX-sh lint)

```bash
cd /home/dustin/projects/stagecoach
# shellcheck with -s sh forces POSIX-sh parsing (flags bashisms). ubuntu-latest ships shellcheck;
# if absent locally: apt-get install shellcheck OR brew install shellcheck.
shellcheck -s sh plugins/asdf-stagecoach/bin/list-all
shellcheck -s sh plugins/asdf-stagecoach/bin/install
shellcheck -s sh plugins/asdf-stagecoach/test/smoke.sh
# Expected: clean (no warnings/errors). If SC warns (e.g. SC2086 word-splitting, SC2039 bashism),
# READ the wiki (https://github.com/koalaman/shellcheck/wiki/SCxxxx) and fix before proceeding.
# Common: quote variables (SC2086), avoid `local`/arrays (SC2039), use command -v not which (SC2230).
```

### Level 2: smoke test (Component Validation — the fixture-archive install)

```bash
cd /home/dustin/projects/stagecoach
sh plugins/asdf-stagecoach/test/smoke.sh
# Expected: "SMOKE PASS", exit 0. This builds a fixture archive + checksums, serves via python3 -m
# http.server, runs bin/install against it, asserts the happy path (binary installed/executable/runs)
# AND the mismatch path (non-zero exit + NO binary left behind). python3 must be on PATH (ubuntu/
# macos ship it). If it hangs, the http-server port-scrape retry may need a longer loop (see smoke.sh).

# Optional: prove bin/list-all works against the REAL repo (needs network + git):
sh plugins/asdf-stagecoach/bin/list-all | tail -3   # newest 3 versions, ascending (newest last)
# Expected: 3 bare version numbers (e.g. 1.0.0 / ...), no leading 'v', ascending.
```

### Level 3: CI wiring (System Validation)

```bash
cd /home/dustin/projects/stagecoach
python3 -c "import yaml; d=yaml.safe_load(open('.github/workflows/ci.yml')); assert 'asdf-plugin-smoke' in d['jobs'], 'missing asdf-plugin-smoke job'; j=d['jobs']['asdf-plugin-smoke']; assert j['runs-on']=='ubuntu-latest'; steps=j['steps']; assert any(('shellcheck' in s.get('run','')) for s in steps), 'no shellcheck step'; assert any(('test/smoke.sh' in s.get('run','')) for s in steps), 'no smoke step'; print('ci.yml asdf-plugin-smoke OK')"
# Expected: "ci.yml asdf-plugin-smoke OK".

# The 6 existing jobs are UNCHANGED (build-test, lint, vulncheck, coverage, npm-smoke, nix-flake-check):
python3 -c "import yaml; d=yaml.safe_load(open('.github/workflows/ci.yml')); assert set(['build-test','lint','vulncheck','coverage','npm-smoke','nix-flake-check','asdf-plugin-smoke']) <= set(d['jobs']); print('all 7 jobs present, existing 6 intact')"
# Expected: "all 7 jobs present, existing 6 intact".
```

### Level 4: README content + scope guard (Domain-Specific Validation)

```bash
cd /home/dustin/projects/stagecoach
# The plugin README documents BOTH asdf and mise (same scripts work for both):
grep -q 'asdf plugin add stagecoach' plugins/asdf-stagecoach/README.md && echo "asdf add OK"
grep -q 'mise plugin add stagecoach' plugins/asdf-stagecoach/README.md && echo "mise add OK"
grep -qi 'canonical' plugins/asdf-stagecoach/README.md && echo "canonical-source note OK"
# Expected: all three "OK". (mise runs the SAME asdf plugin — the README must say so.)

# Both scripts are POSIX sh (#!/usr/bin/env sh), executable, and DO NOT source a remote utils.sh:
head -1 plugins/asdf-stagecoach/bin/list-all plugins/asdf-stagecoach/bin/install   # both: #!/usr/bin/env sh
test -x plugins/asdf-stagecoach/bin/list-all && test -x plugins/asdf-stagecoach/bin/install && test -x plugins/asdf-stagecoach/test/smoke.sh && echo "executable OK"
! grep -rE 'source .*(utils\.sh|https?://)' plugins/asdf-stagecoach/bin/ && echo "no remote utils.sh OK"
# Expected: both shebangs are env sh; all 3 executable; no remote utils.sh sourced.

# Scope: ONLY the expected paths changed; no forbidden files.
git status --porcelain
# Expected lines (only): ?? plugins/asdf-stagecoach/bin/list-all , ?? plugins/asdf-stagecoach/bin/install ,
#   ?? plugins/asdf-stagecoach/README.md , ?? plugins/asdf-stagecoach/test/smoke.sh , " M .github/workflows/ci.yml"
git status --porcelain | grep -E '\.go$|\.goreleaser\.yaml|npm/|release\.yml|go\.(mod|sum)|flake\.|^ M README\.md|\.gitignore|docs/packaging|PRD\.md|plan/|tasks\.json' && echo "FAIL: forbidden file" || echo "OK: no forbidden files"
# Expected: "OK: no forbidden files" (the top-level README.md install line is Mode B / P3 — NOT edited here).
```

## Final Validation Checklist

### Technical Validation
- [ ] `shellcheck -s sh` is clean on `bin/list-all`, `bin/install`, `test/smoke.sh` (Level 1).
- [ ] `sh test/smoke.sh` prints `SMOKE PASS` locally (Level 2); `bin/list-all | tail -3` shows versions.
- [ ] ci.yml is valid YAML with the `asdf-plugin-smoke` job (shellcheck + smoke steps) (Level 3).
- [ ] The 6 existing ci.yml jobs are intact (Level 3).
- [ ] All 3 scripts are `#!/usr/bin/env sh` (POSIX) + executable; no remote `utils.sh` sourced (Level 4).
- [ ] FR-D5 recorded: re-verified the asdf env-var contract (asdf-vm.com/plugins/create.html) + mise
      asdf-compat (mise.jdx.dev/asdf-legacy-plugins.html) + ubuntu-latest shellcheck/python3 + the
      goreleaser asset/checksum naming; recorded the date in the subtask completion.

### Feature Validation
- [ ] `bin/list-all`: `git ls-remote --refs --tags` → strip refs/tags/+v → `grep -E '^[0-9]'` → `sort -V`
      ascending; one per line to stdout; no GitHub API (no 60-req/h limit).
- [ ] `bin/install`: reads ASDF_INSTALL_TYPE/VERSION/PATH; uname→goos/goarch; DIRECT releases/download
      URL (curl -fL follows 302); portable SHA256 (sha256sum|shasum -a 256); awk field-2 exact match;
      `mkdir -p $ASDF_INSTALL_PATH/bin` + extract the root binary there + chmod +x; abort-before-write.
- [ ] The SAME two scripts are documented for BOTH asdf and mise (mise runs asdf plugins unchanged).
- [ ] `bin/install` rejects unsupported OS/arch (non-Darwin/Linux, non-amd64/arm64) with a clear error.
- [ ] The smoke test asserts BOTH the happy path (installed/executable/runs) AND the mismatch path
      (non-zero exit + NO binary left behind = abort-before-write).

### Scope-Boundary Validation
- [ ] `git status --porcelain` shows ONLY: `plugins/asdf-stagecoach/{bin/list-all,bin/install,
      README.md,test/smoke.sh}` (new) + `M .github/workflows/ci.yml`.
- [ ] NO edit to any `.go`, `.goreleaser.yaml`, `npm/*`, `release.yml`, `go.mod`/`go.sum`, `flake.*`,
      `.gitignore`, `docs/packaging.md`, `Makefile`, `PRD.md`, `plan/**`, `tasks.json`.
- [ ] NO edit to the top-level `README.md` install section (the mise/asdf install line is Mode B /
      P3.M1.T1.S1 — this task's README is Mode A, self-contained in `plugins/asdf-stagecoach/`).
- [ ] The 6 existing ci.yml jobs are UNCHANGED (only the 7th is added).
- [ ] The nix-flake-check job (P2.M3.T1.S1, added concurrently) is untouched; the two jobs coexist.

### Documentation & Deployment
- [ ] `plugins/asdf-stagecoach/README.md`: prerequisites, asdf add+install+global, mise add+install+use,
      supported platforms (macOS+Linux amd64/arm64; Windows NOT supported here), how-it-works (env
      contract + git ls-remote + DIRECT URL + SHA256), canonical-source/mirror note.
- [ ] Script headers cite external_deps.md §6 + the env-var contract + the abort-before-write invariant.
- [ ] ci.yml inline comments: the shellcheck + smoke rationale + the FR-D5 shellcheck/python3 note.

---

## Anti-Patterns to Avoid

- ❌ Don't use the GitHub Releases **API** for `list-all` — it's 60 req/h/IP unauthenticated and breaks
  at scale (CI behind NAT, tab-completion). Use `git ls-remote --refs --tags` (no quota). See Gotcha: list-all.
- ❌ Don't forget `--refs` on `git ls-remote --tags` — without it you get the peeled `refs/tags/x^{}`
  duplicates. Strip `refs/tags/` THEN the leading `v`. See Gotcha 8.
- ❌ Don't pass a member name to `tar -xzf` (e.g. `tar -xzf archive -C dest stagecoach`) — the archive
  entry is `./stagecoach` or `stagecoach` (archiver-dependent); extract the WHOLE archive into
  `$ASDF_INSTALL_PATH/bin` (the binary is the sole root entry). Matches npm/install.cjs. See Gotcha 5.
- ❌ Don't assume `sha256sum` exists on macOS — it does NOT (macOS has `shasum -a 256`). Detect with
  `command -v`; the SMOKE TEST must also detect (it builds the fixture checksum on the host). See Gotcha 6.
- ❌ Don't `grep` the checksums line with the asset as a regex — `.` is a wildcard and `1.2.3`
  substring-matches `1.2.30`. Use `awk -v f="$asset" '$2==f {print $1; exit}'` (literal field-2 match).
  See Gotcha 7.
- ❌ Don't double-prefix the version: the FILENAME is `stagecoach_1.0.0_...` (no v) but the DOWNLOAD
  URL is `.../releases/download/v1.0.0/...` (with v). ASDF_INSTALL_VERSION is the bare version. See Gotcha 3.
- ❌ Don't forget to create `$ASDF_INSTALL_PATH/bin/` and place the binary THERE — asdf shims resolve
  there; missing it = "command not found" after install. This is THE load-bearing contract rule. See Gotcha 2.
- ❌ Don't write `bin/install` to parse argv — asdf/mise pass everything via ENV (ASDF_INSTALL_TYPE/
  VERSION/PATH). Use `: "${VAR:?msg}"` to require them. See Gotcha 1.
- ❌ Don't add a windows/zip branch to `bin/install` — asdf/mise are Unix-only; Windows users use
  Scoop/Winget (PRD §21.3). Reject non-Darwin/Linux with a clear error. See Gotcha 10.
- ❌ Don't source a remote `lib/utils.sh` — it's OPTIONAL and sourcing a remote script is a supply-chain
  risk (mise discussions #4054). Keep the plugin self-contained (only `bin/list-all` + `bin/install`).
- ❌ Don't use bashisms (`local`, arrays, `[[ ]]`, `which`) — the scripts are POSIX sh (`#!/usr/bin/env
  sh`); shellcheck `-s sh` catches these. Use `command -v` not `which`.
- ❌ Don't edit the top-level `README.md` install section — the mise/asdf line is Mode B / P3.M1.T1.S1.
  This task's docs are Mode A (self-contained in `plugins/asdf-stagecoach/README.md`). See Scope.
- ❌ Don't touch `.goreleaser.yaml`, `npm/*`, `release.yml`, `flake.*`, `.gitignore`, `docs/packaging.md`,
  or any `.go` file — this task adds 4 files under `plugins/asdf-stagecoach/` + 1 ci.yml job. See Scope.
- ❌ Don't forget FR-D5 — re-verify the asdf env-var contract + mise asdf-compat + ubuntu-latest
  shellcheck/python3 at impl; record the date in the subtask completion.

---

## Confidence Score

**One-pass success likelihood: 9/10.** The asdf/mise plugin contract is small and LIVE-verified
(ASDF_INSTALL_TYPE=version|ref; the plugin creates `$ASDF_INSTALL_PATH/bin/` and places the binary
there — confirmed on asdf-vm.com/plugins/create.html; mise sets the SAME ASDF_* vars — confirmed on
mise.jdx.dev/asdf-legacy-plugins.html). The `bin/list-all` and `bin/install` bodies are given
verbatim (copy-pasteable POSIX sh), and they mirror TWO working in-repo references: `npm/install.cjs`
(the JS twin — download+SHA256-verify+extract+abort-on-mismatch) and `internal/upgrade/download.go`
(the exact assetName/checksumsName naming the sh script must reproduce). The goreleaser asset/checksum
naming is verified against `.goreleaser.yaml` (6 targets, binary at archive root). The smoke test is a
direct POSIX-sh port of the npm wrapper's `install.test.cjs` (fixture archive + local http server +
happy/mismatch assertions), and the CI job is additive (parity with npm-smoke + nix-flake-check). The
two residual risks that keep it from 10/10: (1) **`python3 -m http.server`'s port-scrape** in the smoke
test depends on the exact stderr wording ("Serving HTTP on … port NNNN"), which is stable across
CPython 3.x but worth a retry loop (included) — if the wording differs on the CI runner's Python, the
smoke test fails loudly with the server log, an easy fix; (2) **ubuntu-latest shipping `shellcheck`** is
true today (it's in the pre-installed tool list) but FR-D5 flags it for re-confirmation at impl, with a
documented fallback (`ludeeus/action-shellcheck`). No Go / goreleaser / npm / release.yml / flake scope
surprises remain; the only shared file (ci.yml) is an independent additive job (no collision with the
concurrent Nix task). The wiring is low-risk and locally validatable with shellcheck + the smoke test.