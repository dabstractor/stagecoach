# P2.M2.T1.S1 — Research Findings (winget-releaser Action + PackageIdentifier + asset wiring)

Research date: verified against live GitHub (vedantmgoyal9/winget-releaser action.yml + latest
release + marketplace README) and the repo's .goreleaser.yaml / release.yml / external_deps.md §4.

## 0. The fork question — `vedantmgoyal9/winget-releaser` is the canonical Action

- `vedantmgoyal9/winget-releaser` EXISTS and is ACTIVE (latest release **`v2`**, published
  **2025-01-27**). Its README credits "Russell Banks, the creator of Komac" — Komac is the Rust
  CLI that is the *core* of the action (`cargo binstall komac`). So "Russell Banks" = the Komac
  author, NOT a separate `russellbanks/winget-releaser` action.
- `russellbanks/winget-releaser` → **repo not found** (GitHub API / search_doc). There is NO
  maintained `russellbanks/` action fork. The item's "or russellbanks/winget-releaser — VERIFY the
  maintained fork" resolves to: **use `vedantmgoyal9/winget-releaser@v2`** (the only action; it
  embeds Komac). FR-D5 satisfied: the maintained action is `vedantmgoyal9/winget-releaser@v2`,
  recorded 2025-01-27 (v2 release date).

## 1. The action.yml — EXACT inputs (verified verbatim from the repo)

`vedantmgoyal9/winget-releaser@v2` is a `using: composite` action with `shell: pwsh` steps.
Inputs (`action.yml`):
- `identifier` (**required**) — the PackageIdentifier.
- `version` (optional) — defaults to the release tag WITHOUT leading `v` (the action does
  `$ReleaseInfo.tag_name -replace '^v'`). Leave unset → matches goreleaser `{{.Version}}`.
- `installers-regex` (required, **default `'.(exe|msi|msix|appx)(bundle){0,1}$'`**) — a .NET regex
  the action matches against each release asset's `name`; matched assets' `browser_download_url`
  are passed to `komac update --urls`.
- `max-versions-to-keep` (required, default `'0'` = keep all).
- `release-repository` (required, default `${{ github.event.repository.name }}` = `stagecoach`).
- `release-tag` (required, default `${{ github.event.release.tag_name || github.ref_name }}`).
- `release-notes-url` (optional).
- `token` (**required**) — a GitHub token. See §3.
- `fork-user` (required, default `${{ github.repository_owner }}` = `dabstractor`).

## 2. CRITICAL #1 — the default `installers-regex` does NOT match `.zip`

goreleaser produces the Windows asset as a **ZIP**: `.goreleaser.yaml` `format_overrides: goos:
windows → zip`, so the asset name is **`stagecoach_<version>_windows_amd64.zip`** (e.g.
`stagecoach_1.0.0_windows_amd64.zip`). The action's DEFAULT `installers-regex`
(`.(exe|msi|msix|appx)(bundle){0,1}$`) matches NONE of that. With the default, the action's
release-asset filter returns ZERO urls → `komac update --urls ` (empty) → a broken/empty manifest.
**MUST override `installers-regex` to match the zip.** Use `windows_amd64\.zip$` (matches exactly
the one x64 zip; `windows_arm64.zip` is excluded). PowerShell `-match` is .NET regex; escape the
dot: `windows_amd64\.zip$`.

## 3. CRITICAL #2 — the action EXITS 1 if the PackageIdentifier is not yet in winget-pkgs

The action's FIRST step (`action.yml`) does:
```pwsh
Invoke-WebRequest -Uri "https://github.com/microsoft/winget-pkgs/tree/master/manifests/$($PkgId.ToLower()[0])/$($PkgId.Replace('.', '/'))" -Method Head
if (-not $?) { Write-Output "::error::Package $PkgId does not exist in the winget-pkgs repository. Please add atleast one version of the package before using this action."; exit 1 }
```
So `dabstractor.Stagecoach` MUST already exist in `microsoft/winget-pkgs` before the action can run.
On the FIRST release it does NOT → the action exits 1. **The winget job MUST be gated
(`continue-on-error: true` on the step)** so a pre-bootstrap first release does not fail the whole
release workflow. The one-time bootstrap (manual manifest submission to winget-pkgs) is a
release-day CHECKLIST item, not a code step — item contract §3/§4.

## 4. CRITICAL #3 — `komac update` REUSES the existing manifest's structure (why the bootstrap
        is load-bearing, not just a registration formality)

`komac update <id> --version <v> --urls <urls> --submit` fetches the EXISTING manifest for `<id>`
from winget-pkgs and creates a new version subdir, bumping `InstallerUrl`/`InstallerSha256`/
`PackageVersion` but REUSING the prior structure. So the bootstrap manifest (the first, manually-
authored submission) is the TEMPLATE that defines:
- `InstallerType` (for our goreleaser zip → **`zip`**)
- `NestedInstallerType` (winget REQUIRES this for zip installers since 1.5; the goreleaser zip
  contains a bare `stagecoach.exe` portable CLI → **`portable`**)
- `NestedInstallerFiles` → `RelativeFilePath: stagecoach.exe`, `PortableCommandAlias: stagecoach`
- `Architecture: x64` (amd64 maps to x64 in winget)

The action + Komac do NOT inspect the zip contents (they only have the URL), so they CANNOT infer
`NestedInstallerType`/`NestedInstallerFiles` from scratch — the bootstrap manifest carries that.
FR-D5: verify the exact `NestedInstallerType`/`NestedInstallerFiles` shape against the current
winget manifest spec at impl (winget 1.6+); the portable+zip combo is the correct call for a
goreleaser zip-of-a-bare-exe, but the human-authored bootstrap is where it is pinned.

## 5. The token — `WINGET_TOKEN`, a classic PAT with `public_repo` scope

The default `GITHUB_TOKEN` is scoped to `dabstractor/stagecoach` ONLY and CANNOT fork or push to
`dabstractor/winget-pkgs` (Komac's `sync-fork` forks microsoft/winget-pkgs →
`dabstractor/winget-pkgs`). The action README + marketplace listing require a **classic PAT with
`public_repo` scope** (the winget-pkgs repo is public). Store it as the repo secret
**`WINGET_TOKEN`** (Settings → Secrets → Actions). This mirrors NPM_TOKEN (P2.M1.T1.S3) and the
existing HOMEBREW_TAP_GITHUB_TOKEN / SCOOP_BUCKET_GITHUB_TOKEN pattern in release.yml.

## 6. The goreleaser windows asset — exact names (from .goreleaser.yaml)

- Archive name: `stagecoach_{{ .Version }}_{{ .Os }}_{{ .Arch }}` with windows → zip override ⇒
  **`stagecoach_<version>_windows_amd64.zip`** (+ `..._windows_arm64.zip`).
- Download URL (matches scoop `url_template`): `https://github.com/dabstractor/stagecoach/releases/download/v<version>/stagecoach_<version>_windows_amd64.zip`.
- Checksums file: `stagecoach_<version>_checksums.txt`; the windows zip's line is
  `<64-hex-sha256>  stagecoach_<version>_windows_amd64.zip` (two spaces). Komac computes the SHA256
  from the asset URL itself (the action passes `--urls <browser_download_url>`); the checksums.txt
  is NOT passed to the action — Komac downloads + hashes the asset. (No manual SHA wiring needed in
  release.yml; the asset IS the source of truth.)

## 7. runs-on: windows-latest (convention + pwsh-native)

The action is `shell: pwsh` throughout (Invoke-WebRequest, Invoke-RestMethod, .NET regex). pwsh is
present on ubuntu/macos runners too, but the winget-releaser README + marketplace examples pin
`runs-on: windows-latest`. Use `windows-latest` to match convention and avoid any cross-OS pwsh
quirk. cargo-binstall + komac install cleanly on windows-latest.

## 8. Sibling coordination — P2.M1.T1.S3 (npm-publish job) runs in parallel

S3 adds an `npm-publish` job (needs: goreleaser) + an NPM_TOKEN comment to release.yml. This task
(S1) adds a `winget` job (needs: goreleaser) + a WINGET_TOKEN comment. They are SIBLING jobs — no
conflict. The release.yml secrets-header comment block is EXTENDED by both: S3 adds NPM_TOKEN,
S1 adds WINGET_TOKEN. Each task adds ONLY its own line. Do NOT touch the other's job or comment.

## 9. The goreleaser winget pipe is an ALTERNATIVE — NOT the chosen route

goreleaser.com has a `winget` publish pipe (it can generate the manifest + PR to winget-pkgs). The
contract (external_deps.md §4 + the item) explicitly chooses the **winget-releaser Action** route,
NOT the goreleaser pipe. Do NOT add a `wingets:` block to .goreleaser.yaml (it would also need
--skip handling and a token, duplicating the Action). The Action route is authoritative.

## 10. Validation reality

The winget job CANNOT be fully exercised locally (it needs a real `v*` tag + a real GitHub Release
+ a WINGET_TOKEN + the PackageIdentifier already in winget-pkgs). Local validation = YAML parse
(python3+pyyaml, available here) + an assertion that the job/inputs/continue-on-error are present
+ the docs/packaging.md bootstrap note exists. The action's own per-step behavior is verified by
the action's maintainers; our contribution is correct WIRING (identifier, installers-regex,
token, fork-user, continue-on-error gate) + the bootstrap doc.