# Packaging notes (maintainer)

Distribution-surface decisions and one-time bootstraps that are NOT code. This file covers
Chocolatey (PRD §21.2/§21.3); npm is documented in [`npm/README.md`](../npm/README.md); Homebrew (tap `dabstractor/homebrew-stagecoach`), Scoop (bucket `dabstractor/stagecoach-bucket`),
and AUR (`stagecoach-bin`) are all wired into `release.yml` with **no `--skip` flags** — each pushes to its target repo on tag. (AUR publish is currently disabled: its `git_url` is commented out in
`.goreleaser.yaml` while aur.archlinux.org recovers.)

## Chocolatey

Every `v*` tag runs goreleaser's native
[`chocolateys:`](https://goreleaser.com/customization/chocolatey/) pipe (PRD §21.2), which builds a
`.nupkg` and pushes it to the [Chocolatey community repository](https://community.chocolatey.org/)
(`push.chocolatey.org`). Windows users install and update with Chocolatey directly:

```sh
choco install stagecoach
choco upgrade stagecoach -y     # needs admin
```

- **Package**: `stagecoach` (owners `dabstractor`, title `Stagecoach`) on the community repo. The
  pipe fields live in the `chocolateys:` block of [`.goreleaser.yaml`](../.goreleaser.yaml)
  (`source_repo: https://push.chocolatey.org/`).
- **Secret**: `CHOCOLATEY_API_KEY` — the Chocolatey community-source push key (chocolatey.org →
  Account Settings → API Key). It is consumed by the `chocolateys:` pipe inside the goreleaser job
  (see [`release.yml`](../.github/workflows/release.yml)); add it under repo
  Settings → Secrets → Actions.
- **Why Chocolatey, not the Windows store channel (v3.3)**: the previous Windows store channel
  submitted a manifest to Microsoft's community package manifest repository, whose
  `validationDefender` step runs an install-in-a-clean-VM Microsoft Defender scan that
  **hard-blocks the unsigned binary every release** — an unbounded per-release tax. Chocolatey
  publishes directly via the API key with no such gate, so there is no manifest-acceptance queue
  to bootstrap or track.
- **`stagecoach upgrade` behavior**: `choco upgrade` needs admin, so `stagecoach upgrade` detects
  a Chocolatey install (FR-U2) and **prints** `choco upgrade stagecoach -y` for the user to run
  (FR-U4). It does **not** self-swap — Chocolatey owns the binary under `ProgramData\chocolatey`
  (FR-U1).

> No one-time bootstrap, no installer YAML, and no pending-acceptance checklist: unlike the old
> manifest-PR flow, Chocolatey publishes on every release via the API key. The previous manifest
> tooling, the nested-portable installer YAML, and the "pending acceptance" steps are all gone.

### PowerShell installer (no package manager)

Windows users without Chocolatey (or Scoop) can use the `irm | iex` one-liner — the Windows analog
of the Unix `curl | sh` installer (PRD §21.3). It downloads [`install.ps1`](../install.ps1) from the
repo root and executes it:

```powershell
irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex
```

`install.ps1` detects the Windows arch, downloads the matching
`stagecoach_<version>_windows_<arch>.zip` from the latest GitHub Release, SHA256-verifies it
against `checksums.txt`, extracts `stagecoach.exe` to `$LOCALAPPDATA\stagecoach` (the
`rustup`/`starship`/`uv` pattern — user-owned, no admin), and prepends that directory to the
**user** `PATH`. Because the binary is package-manager-unowned, the installer tags it
`STAGECOACH_INSTALL_METHOD=direct` so `stagecoach upgrade` self-swaps it like any direct install
(FR-U5).

> Re-open your terminal for the `PATH` change to take effect.

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