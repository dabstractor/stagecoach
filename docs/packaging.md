# Packaging notes (maintainer)

Distribution-surface decisions and one-time bootstraps that are NOT code. This file covers
Chocolatey (/); npm is documented in [`npm/README.md`](../npm/README.md); Homebrew (tap `dabstractor/homebrew-stagecoach`), Scoop (bucket `dabstractor/stagecoach-bucket`),
and AUR (`stagecoach-bin`) are all wired into `release.yml` with **no `--skip` flags** — each pushes to its target repo on tag. (AUR publish was paused for the 2026-08 aur.archlinux.org outage by commenting out its `git_url`; the **AUR section below** is the runbook that re-enables the pipe and bootstraps both AUR packages.)

## Homebrew / Linuxbrew

The tap is `dabstractor/homebrew-stagecoach` (published by `release.yml` on every tag). The same `brew`
tool serves **both** macOS Homebrew (`/opt/homebrew` on Apple Silicon, `/usr/local` on Intel) **and**
Linuxbrew (`/home/linuxbrew/.linuxbrew`) on Linux.

- **`stagecoach upgrade` behavior**: `stagecoach upgrade` detects a brew install by its Cellar path
  (`/opt/homebrew/Cellar/`, `/usr/local/Cellar/`, or `/home/linuxbrew/.linuxbrew/Cellar/`) and
  **delegates** to `brew upgrade stagecoach`. Brew owns the binary under the Cellar, so it is never
  self-swapped (FR-U1/FR-U4); unlike Chocolatey, `brew upgrade` is user-space and is run directly
  (streaming its output), not printed.

## AUR (`stagecoach-bin` automatic, `stagecoach` manual)

Arch users install from the [AUR](https://aur.archlinux.org/) through **two packages** with two ownership models — the `-bin` side is the goreleaser-native `aurs:` pipe in [`.goreleaser.yaml`](../.goreleaser.yaml); the from-source side has no goreleaser equivalent and is maintained by hand:

| Package | Installs | Updated by |
|---|---|---|
| [`stagecoach-bin`](https://aur.archlinux.org/packages/stagecoach-bin) | Prebuilt `stagecoach_<ver>_linux_<arch>.tar.gz` from the GitHub Release | **Automatic** — `aurs:` pipe fires on every `v*` tag (commit author `goreleaserbot`) |
| [`stagecoach`](https://aur.archlinux.org/packages/stagecoach) | Builds the `v$pkgver` source tarball with local `go` — same recipe as the release build (`CGO_ENABLED=0`, `-X main.version=$pkgver`) | **Manual** — maintainer bumps it per release (flow below) |

The `-bin` package declares `provides=('stagecoach')` + `conflicts=('stagecoach')`, so `yay -S stagecoach-bin` satisfies anything depending on `stagecoach`, and the two can never coexist.

### `stagecoach upgrade` behavior

`stagecoach upgrade` detects a `-bin` install with the tier-(b) probe `pacman -Q stagecoach-bin` ([`internal/upgrade/detect.go`](../internal/upgrade/detect.go); channel identifier `"aur"`, FR-U3's "AUR" delegation row) and **prints** the command instead of running it (pacman needs root; FR-U4 never auto-sudo):

```
sudo pacman -Syu stagecoach-bin
# (or, with an AUR helper: yay -Syu stagecoach-bin)
```

> **Known gap:** the probe table queries only `stagecoach-bin`. An install of the from-source `stagecoach` package is *not* detected — `/usr/bin` is deliberately absent from the path heuristics (ambiguous), so such installs classify as direct. Source-package users upgrade with their AUR helper. The candidate fix is a second probe row (`pacman -Q stagecoach` → same `aur` channel); until then this is documented behavior, not a detection bug.

### How AUR publishing works (20-second model)

- Every package is its **own git repo** at `ssh://aur@aur.archlinux.org/<pkgname>.git`. You push exactly two text files: `PKGBUILD` (the recipe) and `.SRCINFO` (metadata, generated — never hand-edited — via `makepkg --printsrcinfo > .SRCINFO`). **Never** binaries, tarballs, or build output.
- The **first push creates the package** (it appears on the website within ~15 min) and the pusher becomes its maintainer.
- **`master` is mandatory.** The AUR enforces a single-branch policy and rejects pushes to any other branch ([bbs](https://bbs.archlinux.org/viewtopic.php?id=257186)). A clone of an *empty* AUR repo lands on whatever `init.defaultBranch` says locally — so rename (`git branch -m master`) before the first push. goreleaser already hardcodes `master` on its side.
- The AUR website's edit interface is never used; the git repo is the only source of truth.

### Auth — two keys, one account

The AUR account carries **both** public keys:

- **CI key** — the `AUR_SSH_PRIVATE_KEY` secret consumed by the `aurs:` pipe in [`release.yml`](../.github/workflows/release.yml) (Settings → Secrets → Actions; its public key must be on the AUR account).
- **Your personal key** — for manual pushes (the source package, and any hand-seeding of `-bin`).

Verify auth any time: `ssh aur@aur.archlinux.org help` (prints the command list ⇒ the key is registered).

### One-time bootstrap (2026-08, post-outage)

The `aurs:` pipe was disabled while aur.archlinux.org was down (its `git_url` commented out), so **v0.1.16 shipped with no AUR publish**. Bootstrap:

1. **Re-enable the pipe** — in `.goreleaser.yaml`, restore `git_url: ssh://aur@aur.archlinux.org/stagecoach-bin.git` (delete the `TEMPORARY (2026-08)` comment block), then `goreleaser check`.
2. **Seed `stagecoach-bin` at 0.1.16 by hand** — the tag already shipped and the pipe only fires on *future* tags. Clone `stagecoach-bin.git`, commit the seed PKGBUILD below, push to `master`. From the next tag goreleaser takes over; its "Update to vX" commit replaces the seed PKGBUILD with its generated one (same shape: binary-only install, `depends=('git')`, per-arch sources) — that swap is expected.
3. **Publish the source package `stagecoach`** — clone (or reuse) `stagecoach.git`, commit the PKGBUILD below, push to `master`.

Both PKGBUILDs at v0.1.16 (checksums verified against the release's `stagecoach_0.1.16_checksums.txt`):

**`stagecoach` (from source; ships shell completions):**

```bash
# Maintainer: Dustin Schultz <dustindschultz@gmail.com>

pkgname=stagecoach
pkgver=0.1.16
pkgrel=1
pkgdesc='Snapshot-based AI commit message generator that uses YOUR local CLI agent'
arch=('x86_64' 'aarch64')
url='https://github.com/dabstractor/stagecoach'
license=('MIT')
makedepends=('go')
conflicts=('stagecoach-bin')
source=("$pkgname-$pkgver.tar.gz::https://github.com/dabstractor/stagecoach/archive/refs/tags/v$pkgver.tar.gz")
sha256sums=('e2cf785c9414e268f547c878bda5c6d3bb984018bc38846de6d06877de3ab80e')

build() {
    cd "$pkgname-$pkgver"
    export CGO_ENABLED=0
    go build -trimpath -ldflags "-X main.version=$pkgver" -o build/stagecoach ./cmd/stagecoach
}

package() {
    cd "$pkgname-$pkgver"
    install -Dm755 build/stagecoach "$pkgdir/usr/bin/stagecoach"
    install -Dm644 LICENSE "$pkgdir/usr/share/licenses/$pkgname/LICENSE"

    # shell completions (cobra `completion` subcommand)
    build/stagecoach completion bash > stagecoach.bash
    build/stagecoach completion zsh  > _stagecoach
    build/stagecoach completion fish > stagecoach.fish
    install -Dm644 stagecoach.bash "$pkgdir/usr/share/bash-completion/completions/stagecoach"
    install -Dm644 _stagecoach      "$pkgdir/usr/share/zsh/site-functions/_stagecoach"
    install -Dm644 stagecoach.fish  "$pkgdir/usr/share/fish/vendor_completions.d/stagecoach.fish"
}
```

**`stagecoach-bin` seed (deliberately mirrors goreleaser's generated PKGBUILD — binary-only, no completions — so the first automated push after this seed is a near-no-op diff):**

```bash
# Maintainer: Dustin Schultz <dustindschultz@gmail.com>

pkgname=stagecoach-bin
pkgver=0.1.16
pkgrel=1
pkgdesc='Snapshot-based AI commit message generator that uses YOUR local CLI agent'
arch=('aarch64' 'x86_64')
url='https://github.com/dabstractor/stagecoach'
license=('MIT')
depends=('git')
provides=('stagecoach')
conflicts=('stagecoach')
source_x86_64=("$pkgname-$pkgver-linux_amd64.tar.gz::https://github.com/dabstractor/stagecoach/releases/download/v$pkgver/stagecoach_${pkgver}_linux_amd64.tar.gz")
sha256sums_x86_64=('fe1859a18dfd36e416ee1c225458abc17e428db9bdd6b3feaf97dd249b808d22')
source_aarch64=("$pkgname-$pkgver-linux_arm64.tar.gz::https://github.com/dabstractor/stagecoach/releases/download/v$pkgver/stagecoach_${pkgver}_linux_arm64.tar.gz")
sha256sums_aarch64=('6f954c2982e90a59571e1bf0ecde96dfab994bc7f25adac6c41f6dbe4ecf777c')

package() {
    install -Dm755 stagecoach "$pkgdir/usr/bin/stagecoach"
}
```

Also commit this `.gitignore` in both AUR clones (guards against committing makepkg build output — the AUR only accepts the two text files):

```
src/
pkg/
*.pkg.tar.*
stagecoach-*.tar.gz
stagecoach-*/
```

### Per-release flow

- **`stagecoach-bin`** — nothing to do. Every `v*` tag regenerates PKGBUILD + `.SRCINFO` and pushes the "Update to vX" commit. If a tag ever ships with the pipe disabled again, re-seed by hand (above) before the next tag.
- **`stagecoach` (manual, ~2 min)**:

  ```bash
  # 1. bump pkgver= (reset pkgrel=1 if it was bumped)
  # 2. refresh checksums + rebuild + republish:
  updpkgsums
  makepkg -si && stagecoach --version
  makepkg --printsrcinfo > .SRCINFO
  git add PKGBUILD .SRCINFO
  git commit -m "Update to X.Y.Z" && git push
  ```

### Rules that bite

- A changed `PKGBUILD` must be pushed **together with a regenerated `.SRCINFO`** — the AUR server rejects the push otherwise. If rejected: regenerate, re-commit. Never force-push.
- No binaries or build output in the repo (see `.gitignore` above).
- Keep history linear — on a rejected push, `git pull --rebase`, never merge commits.
- New packages take ~15 min to appear on the website after the first push; RPC/API lag behind the git interface is normal.

## Chocolatey

Every `v*` tag runs goreleaser's native
[`chocolateys:`](https://goreleaser.com/customization/chocolatey/) pipe, which builds a
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
 a Chocolatey install and **prints** `choco upgrade stagecoach -y` for the user to run
. It does **not** self-swap — Chocolatey owns the binary under `ProgramData\chocolatey`
.

> No one-time bootstrap, no installer YAML, and no pending-acceptance checklist: unlike the old
> manifest-PR flow, Chocolatey publishes on every release via the API key. The previous manifest
> tooling, the nested-portable installer YAML, and the "pending acceptance" steps are all gone.

### PowerShell installer (no package manager)

Windows users without Chocolatey (or Scoop) can use the `irm | iex` one-liner — the Windows analog
of the Unix `curl | sh` installer. It downloads [`install.ps1`](../install.ps1) from the
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
.

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
2. Run `nix build.#default` (or `nix flake check`). It fails with a fixed-output mismatch:

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

> (verify at impl): re-confirm the `cachix/install-nix-action` major version pin, that the
> locked nixpkgs provides Go >= 1.22, and that `nix flake check` defaults to the current system
> (modern Nix 2.21+) — building the x86_64-linux package on the ubuntu runner.

## Documentation site (GitHub Pages)

The public docs site at `https://dabstractor.github.io/stagecoach/` is built from `docs/*.md` with
[mkdocs-material](https://squidfunk.github.io/mkdocs-material/) (`mkdocs.yml`, `requirements-docs.txt`)
and deployed by [`.github/workflows/docs.yml`](../.github/workflows/docs.yml). It lives on the
**same `gh-pages` branch** as the apt/dnf repos above.

Coexistence contract — two workflows, one branch:

- The docs job deploys with `keep_files: true` → it only touches site files at the branch root and
  never deletes `apt/` or `rpm/`.
- The `apt-dnf-repo` release job clones `gh-pages` first (carrying the site files through) before
  its force-push, so a release does not wipe the docs site.

Local preview:

```bash
pip install -r requirements-docs.txt
mkdocs serve      # http://127.0.0.1:8000
```