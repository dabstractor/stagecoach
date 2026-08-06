# Packaging notes (maintainer)

Distribution-surface decisions and one-time bootstraps that are NOT code. This file covers
WinGet (PRD §21.2/§21.3); npm is documented in [`npm/README.md`](../npm/README.md); Homebrew/Scoop/AUR
are pending their target repos (release.yml runs goreleaser with `--skip=homebrew,scoop,aur` until
those exist).

## WinGet (`dabstractor.Stagecoach`)

Every `v*` tag runs a `winget` job in
[`release.yml`](../.github/workflows/release.yml) that uses
[`vedantmgoyal9/winget-releaser@v2`](https://github.com/vedantmgoyal9/winget-releaser) to open a
manifest PR to [`microsoft/winget-pkgs`](https://github.com/microsoft/winget-pkgs). The action
(powered by [Komac](https://github.com/russellbanks/Komac)) matches the release's
`stagecoach_<version>_windows_amd64.zip` via `installers-regex`, then bumps `PackageVersion` + the
`InstallerUrl`/`InstallerSha256` of the existing manifest. Windows users install with
`winget install dabstractor.Stagecoach`.

- **PackageIdentifier**: `dabstractor.Stagecoach` — `<Publisher>.<Product>` convention.
- **Installer asset**: the goreleaser `stagecoach_<version>_windows_amd64.zip` (a ZIP containing
  the bare `stagecoach.exe`). Download URL:
  `https://github.com/dabstractor/stagecoach/releases/download/v<version>/stagecoach_<version>_windows_amd64.zip`.
- **InstallerType**: `zip` (goreleaser ships a zip, not an exe/msi). **NestedInstallerType**:
  `portable` (the zip holds a bare CLI exe). winget REQUIRES these for a zip installer (1.5+).
- **Secret**: `WINGET_TOKEN` — a classic PAT with `public_repo` scope (the default `GITHUB_TOKEN`
  cannot fork winget-pkgs). Add under repo Settings → Secrets → Actions.

### One-time bootstrap (release-day checklist — NOT a code step)

The action's first step HEAD-checks `microsoft/winget-pkgs/manifests/d/dabstractor/Stagecoach`
and EXITS 1 if the PackageIdentifier does not yet exist. So the FIRST release must be preceded
by a one-time manual manifest submission that ESTABLISHES `dabstractor.Stagecoach`. Komac's
`update` then REUSES this manifest's structure for every later version (it cannot infer
`NestedInstallerType` from a URL — the bootstrap pins it).

1. After the first `v*` release exists (goreleaser published the GitHub Release), draft the
   initial manifest. Use `wingetcreate` (the winget-pkgs PR tool) or the New-Package PR template:

   ```sh
   wingetcreate new https://github.com/dabstractor/stagecoach/releases/download/v<version>/stagecoach_<version>_windows_amd64.zip
   ```

   then edit the generated YAML so the installer block is EXACTLY:

   ```yaml
   PackageIdentifier: dabstractor.Stagecoach
   PackageVersion: <version>
   Installers:
     - Architecture: x64
       InstallerType: zip
       InstallerUrl: https://github.com/dabstractor/stagecoach/releases/download/v<version>/stagecoach_<version>_windows_amd64.zip
       InstallerSha256: <the windows_amd64.zip SHA256 from stagecoach_<version>_checksums.txt>
       NestedInstallerType: portable
       NestedInstallerFiles:
         - RelativeFilePath: stagecoach.exe
           PortableCommandAlias: stagecoach
   ```

2. Submit the manifest as a New-Package PR to `microsoft/winget-pkgs` and let it merge.
3. Confirm the PackageIdentifier path now exists. From the NEXT release on, the `release.yml`
   `winget` job auto-opens the version-bump PR. (The job's `continue-on-error: true` keeps the
   pre-bootstrap first release green; it is harmless thereafter.)

### Release-day checklist (WinGet)

- [ ] `WINGET_TOKEN` secret exists (classic PAT, `public_repo` scope).
- [ ] FIRST RELEASE ONLY: bootstrap manifest submitted + merged (above).
- [ ] The `release.yml` `winget` job ran (best-effort; a winget-pkgs PR hiccup does not block).
- [ ] The winget-pkgs manifest PR for `<version>` was opened (check `microsoft/winget-pkgs` PRs).

> FR-D5 (verify at impl): re-confirm the `wingetcreate new` flow, the exact
> `NestedInstallerType`/`NestedInstallerFiles` shape, and the `vedantmgoyal9/winget-releaser`
> version against the current winget manifest spec + action release at implementation time.

## Nix (flakes)

Stagecoach ships a [Nix flake](https://nixos.wiki/wiki/Flakes) (`flake.nix` at the repo root)
that builds the same binary goreleaser ships (`cmd/stagecoach`, `CGO_ENABLED=0`) via nixpkgs
[`buildGoModule`](https://nixos.org/manual/nixpkgs/stable/#buildGoModule), over the four systems
(x86_64-linux, aarch64-linux, x86_64-darwin, aarch64-darwin).

```bash
nix run github:dabstractor/stagecoach               # run without installing
nix profile install github:dabstractor/stagecoach    # install into the user profile
nix develop                                          # hermetic dev shell (go + gopls)
nix build .#default && ./result/bin/stagecoach --help # local build
```

> The Nix-built binary reports version `dev` (the flake does not inject goreleaser's version
> ldflags, and Go's VCS embedding is unavailable in the sandbox — source is copied to the store
> without `.git`). For a real version string, use a goreleaser GitHub Release.

### Keeping `vendorHash` current (maintainer)

`buildGoModule` pins the Go dependency tree with a `vendorHash` in `flake.nix`. Every
`go.mod`/`go.sum` change invalidates it. CI's `nix-flake-check` job fails with the new hash to
paste. The update workflow:

1. Set `vendorHash = pkgs.lib.fakeHash;` (the `sha256-AAAA...=` placeholder) if starting fresh.
2. Run `nix build .#default` (or `nix flake check`). It fails with a fixed-output mismatch:

   ```
   specified: sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
      got:    sha256-<REAL-HASH>
   ```

3. Paste the `got:` value into `vendorHash` in `flake.nix`.
4. Re-run `nix build`/`nix flake check` → green. Commit `flake.nix` (and `flake.lock` if it changed).

`flake.lock` is committed (stagecoach is an application; the lock pins nixpkgs for reproducible
CI). After a nixpkgs bump, run `nix flake update` and commit the new `flake.lock`.

#### First-run note (no local Nix)

If you do not have Nix installed locally, ship the flake with `vendorHash = pkgs.lib.fakeHash;`
and commit it. The first CI `nix-flake-check` run will FAIL with the `got:` hash — copy that value
into `vendorHash` in `flake.nix` and commit the fix. Likewise, the first CI run generates
`flake.lock` (`nix flake check` resolves inputs); commit it in the same follow-up.

### Release-day checklist (Nix)

- [ ] `flake.nix` committed (with the REAL `vendorHash`, not `lib.fakeHash`).
- [ ] `flake.lock` committed (pins nixpkgs for reproducible CI).
- [ ] `.gitignore` has `result`, `result-*`, `.direnv` (and NOT `flake.lock`).
- [ ] The CI `nix-flake-check` job is green on `main`.

> FR-D5 (verify at impl): re-confirm the `cachix/install-nix-action` major version pin, that the
> locked nixpkgs provides Go >= 1.22, and that `nix flake check` defaults to the current system
> (modern Nix 2.21+) — building the x86_64-linux package on the ubuntu runner.