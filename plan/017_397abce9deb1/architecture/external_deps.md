# External dependencies & integration shapes — Stagecoach v3.0

MCP/web research was unavailable in the planning environment. The shapes below are from the PRD
itself (§9.29, §21.2 — authoritative) plus standard, stable public API/format knowledge. Each
implementing subtask's `context_scope` calls out the verification duty (FR-D5 discipline: verify
against live docs/`--help` at implementation, record the date).

## 1. GitHub Releases REST API (the named network exception — FR-U5 step 2, FR-U6)

`stagecoach upgrade` is the ONLY stagecoach surface that makes HTTP calls (PRD §19 amendment; §9.29
intro). It fetches ONLY the project's own release artifacts + checksums — never credentials, diffs,
or repo data.

- **Latest stable**: `GET https://api.github.com/repos/{owner}/{repo}/releases/latest`
  → JSON: `{ "tag_name": "v1.2.3", "name": "...", "assets": [ {"name": "...", "browser_download_url": "...", "size": N, "api_url": "..."}, ... ], "prerelease": bool, "draft": bool }`.
- **All (for `--prerelease`)**: `GET .../releases` → array of the above; filter `draft==false`,
  admit prereleases only when `--prerelease`; pick highest semver.
- **Pinned (`--version <v>`)**: `GET .../releases/tags/{tag}` → one release object.
- **Auth**: `Authorization: Bearer <token>` header if `STAGECOACH_GITHUB_TOKEN` or `GITHUB_TOKEN` is
  set (raises rate limit; absence → unauthenticated 60 req/h quota). `User-Agent` header REQUIRED by
  the API (Go's default `Go-http-client` is sometimes rejected; set `User-Agent: stagecoach/<ver>`).
- **Asset download**: the checksums file and the platform archive are downloaded via the
  `browser_download_url` (a `objects.githubusercontent.com` redirect). Go's `net/http` follows
  redirects by default; stream to a temp file (archives can be multi-MB).
- **Error mapping**: 404 (no releases / unknown tag) → clear "no releases found" error; 403/429
  (rate-limited) → surface the rate-limit message + hint to set a token; network error → retry-free
  failure (FR-U11: aborts before any write).
- **Verify at impl**: exact asset-name template produced by goreleaser
  (`stagecoach_{version}_{os}_{arch}.tar.gz` / `.zip`) against `.goreleaser.yaml`'s
  `archives[].name_template` (confirmed: `{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}`).
  The checksums file is `{project}_{version}_checksums.txt` (`checksum.name_template`). Match the
  goreleaser output precisely when selecting the asset + the checksums line.

## 2. SHA256 checksums.txt format (goreleaser)

goreleaser's `checksums.txt` is one line per artifact:
`<64-hex-sha256>  <artifact-filename>` (two spaces). v3.0 parses the line for the selected asset
name and compares to the downloaded archive's SHA256 (FR-U5 step 4 — a hard gate). A missing line or
mismatch aborts BEFORE any filesystem write (FR-U11).

## 3. npm wrapper (the esbuild/turbo/prisma pattern — PRD §21.2)

A thin JS package `@dabstractor/stagecoach` whose `postinstall` downloads the matching prebuilt
native binary into a cache and whose `bin` execs it. Verified shape (stable industry pattern):

- **`package.json`**: `"bin": { "stagecoach": "./bin/stagecoach.js" }` (the shim);
  `"optionalDependencies"` of per-platform packages OR a single package with a `postinstall` that
  picks the asset by `process.platform`/`process.arch` from GitHub Releases. The PRD specifies the
  **single-package + postinstall-download** form ("detects process.platform/process.arch, downloads
  the matching prebuilt binary from GitHub Releases into a cache, SHA256-verifies it"). So:
  `postinstall` runs `node install.mjs` (or `.cjs`).
- **`install.mjs`**: maps `process.platform`/`process.arch` → goreleaser asset name
  (`darwin`+`arm64` → `stagecoach_<v>_darwin_arm64.tar.gz`; `win32`+`x64` → `..._windows_amd64.zip`);
  downloads archive + checksums.txt; SHA256-verifies; extracts the single `stagecoach`/`stagecoach.exe`
  binary to a cache dir (e.g. `node_modules/.cache/@dabstractor/stagecoach/` or a versioned path).
  On `--ignore-scripts`/corporate-npm: the binary is absent → the shim prints a fallback message
  pointing at the direct-binary install.
- **`bin/stagecoach.js` (the shim)**: sets `process.env.STAGECOACH_INSTALL_METHOD = "npm"` on every
  invocation (FR-U2), then `child_process.spawn`/`execFileSync` the cached native binary, forwarding
  argv/stdin/stdout/stderr and preserving exit code. This is how `stagecoach upgrade` recognizes an
  npm install and delegates to `npm install -g @dabstractor/stagecoach@latest` instead of self-swapping
  the cached binary (FR-U3 npm row).
- **Module path**: goreleaser has NO native npm pipe (PRD §21.2 "Beyond goreleaser"). The wrapper is
  built/published by a separate CI step (a GitHub Action that `npm publish`s on tag). The native
  binaries come from the goreleaser GitHub Release (already produced). So the wrapper's release step
  only ships the JS; the binaries are downloaded at install time.
- **Verify at impl**: exact asset name (item 1); Node version floor; whether `tar`/`unzip` are
  available or a JS lib (e.g. `tar`, `yauzl`) is needed (prefer a tiny JS dep to avoid system-tool
  dependency on Windows).

## 4. Winget (Windows default package manager — PRD §21.2)

- **Mechanism**: a GitHub Action (`vedantmgoyal9/winget-releaser` or `russellbanks/winget-releaser`)
  opens a PR to `microsoft/winget-pkgs` per release tag, adding a manifest version subdir.
- **Manifest shape** (winget v1.5+ YAML): one `.yaml` per installer + a versioned package, or the
  multi-doc `*.installer.yaml` / `*.locale.*.yaml` / `*.yaml` set. Minimal fields: `PackageIdentifier`
  (`dabstractor.Stagecoach`), `PackageVersion`, `InstallerType` (`zip` for Windows; `portable` for a
  bare exe), `Installers[].InstallerUrl` (the goreleaser `windows_amd64.zip` download URL),
  `InstallerSha256` (from checksums.txt), `Architecture` (`x64`).
- **The winget-releaser Action** auto-generates the manifest from the release; the repo's contribution
  is (a) adding the Action step to release.yml and (b) the `PackageIdentifier` decision. Most impl
  work is wiring the Action + verifying the manifest lands; the Action does the YAML authoring.
- **Verify at impl**: current winget-releaser Action version + its required inputs; the canonical
  `PackageIdentifier`; whether `portable` vs `zip` InstallerType is correct for a CLI binary.

## 5. Nix flake (PRD §21.2)

`flake.nix` in-repo, no external repo/secret. Powers `nix run`, `nix profile install`, devbox/nix-shell.

- Shape: `inputs.nixpkgs.url`; `outputs = { self, nixpkgs }: ...` over `eachSystem ["x86_64-linux"
  "aarch64-linux" "x86_64-darwin" "aarch64-darwin"]`; `packages.default =
  pkgs.buildGoModule { pname, version, src = ./.; vendorHash = ...; subPackages = ["cmd/stagecoach"]; }`;
  optional `devShells.default`.
- **`vendorHash`**: must be kept current (or `vendorHash = lib.fakeHash` while iterating; CI can
  update it). `buildGoModule` fetches deps itself (no `go.sum` vendoring needed).
- **Verify at impl**: current `buildGoModule` API; whether `go 1.22` directive needs a newer nixpkgs
  input; the `vendorHash` workflow (a `nix flake check` CI step or a renovate bot).

## 6. mise / asdf plugins (PRD §21.2)

~30-line shell-script plugins pointing at the GitHub Release archives. Per §21.2 these live in a
separate plugin repo (convention: `mise-stagecoach` / `asdf-stagecoach`), but the scripts are authored
here as the canonical reference.

- **asdf plugin** (`lib/utils.sh` + `bin/install` + `bin/list-all`): `list-all` greps GitHub release
  tags; `install` downloads the platform archive + extracts to `$ASDF_INSTALL_PATH/bin`.
- **mise plugin** (compatible with asdf plugins; `bin/install` + `bin/list-all`): same shape; mise
  also accepts a `[plugins.stagecoach]` config or a `mise.toml`. mise plugins can be a plain asdf plugin.
- **Verify at impl**: current asdf/mise plugin contract (`ASDF_INSTALL_TYPE/VERSION/INSTALL_PATH`
  env vars; mise's `MISE_*` equivalents).

## 7. Install-method detection signals (FR-U2 cascade)

Best-effort; logged at `--verbose`; ambiguous → default `direct` (the only self-swap-eligible channel).

- **(a) Explicit override**: `--install-method <m>` / `STAGECOACH_INSTALL_METHOD` (the npm wrapper
  sets the latter — item 3).
- **(b) Package-manager DB query** (run, parse stdout/exit for ownership):
  `brew list stagecoach` · `scoop list stagecoach` (or check `scoop prefix`) · `winget list` (Windows
  only) · `pacman -Q stagecoach-bin` (AUR) · `npm ls -g --depth=0` (check for the package) ·
  `mise ls` / `asdf list`.
- **(c) Path heuristics** on `os.Executable()` resolved realpath: Homebrew Cellar
  (`/opt/homebrew/Cellar/...` or `/usr/local/Cellar/...`), Scoop shims (`...\scoop\shims\...`),
  npm global node_modules (`...\npm\...` or `nodejs`), Nix `/nix/store/...`, `$GOPATH/bin` (go
  install), AUR `/usr/bin/...` (system-managed).
- **(d) default `direct`**.
- **Verify at impl**: each DB query's exact invocation + output shape on the target OS; path-heuristic
  roots per OS. Keep detection read-only and fast (these queries must not hang — apply a short timeout).

## 8. Atomic swap (FR-U7)

- **Unix (linux/darwin)**: `os.Rename(newTemp, currentExe)` — atomic at the syscall level; the running
  process keeps the old inode open. If the target path is not writable (e.g. `/usr/local/bin` root-
  owned), detect it, leave everything untouched, print the exact `sudo` command (never auto-elevate,
  FR-U4).
- **Windows**: the running `stagecoach.exe` is locked. Two-step: rename running `stagecoach.exe` →
  `stagecoach.exe.old` (Windows permits renaming a running image but not overwriting it in place);
  move the new file → `stagecoach.exe`; clean up `*.old` at next launch (delete any
  `stagecoach.exe.old` sibling of the exe at startup, best-effort). The new binary already passed the
  sanity-run, so the two-file window is clean — never zero runnable binaries.

## Cross-cutting: this is walled off (see v3_scope_boundary.md)

Every external integration above is reached ONLY by `stagecoach upgrade` (items 1–2, 7–8) or by the
distribution plumbing (items 3–6). The commit path (§9.1–§9.28) touches none of them.
