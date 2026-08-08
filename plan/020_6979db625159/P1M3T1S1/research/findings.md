# Findings — P1.M3.T1.S1 (docs/packaging.md: WinGet → Chocolatey + PowerShell installer)

## 1. The current docs/packaging.md WinGet surface (what to replace)

- **Line 4 (intro)**: "This file covers WinGet (PRD §21.2/§21.3); npm is documented in npm/README.md;
  Homebrew/Scoop/AUR are pending…". The word "WinGet" here MUST also become "Chocolatey" — the
  verification (`rg -ni winget docs/packaging.md` == 0 hits) catches it. (The rest of the intro — npm,
  Homebrew/Scoop/AUR pending — stays; not winget.)
- **Lines 8-73 (the `## WinGet (`dabstractor.Stagecoach`)` section)**: REPLACE entirely. Contains:
  - 8-17: winget-releaser description (vedantmgoyal9/winget-releaser@v2 → microsoft/winget-pkgs manifest PR;
    winget install dabstractor.Stagecoach).
  - 18-26: PackageIdentifier / Installer asset (windows_amd64.zip) / InstallerType=zip / NestedInstallerType=portable
    / WINGET_TOKEN secret (classic PAT, public_repo scope).
  - 28-62: one-time bootstrap (wingetcreate new … + the NestedInstallerType/NestedInstallerFiles YAML block +
    submit New-Package PR to microsoft/winget-pkgs).
  - 64-73: release-day checklist (WINGET_TOKEN secret; FIRST RELEASE bootstrap; winget job ran; winget-pkgs PR opened)
    + the "FR-D5 verify at impl" note.
- **Line 75**: `## Nix (flakes)` — the NEXT section. KEEP (untouched). The replacement must end cleanly before it.
- All `winget`/`WinGet`/`wingetcreate`/`winget-pkgs`/`WINGET_TOKEN` hits are within lines 4-73 (confirmed by grep).

## 2. The Chocolatey facts to document (source of truth: spec §21.2, already updated by b0105e5)

- **Channel**: goreleaser-native `chocolateys:` pipe (`.goreleaser.yaml`, P1.M2.T1.S1 COMPLETE). Publishes a
  `.nupkg` to the Chocolatey community repo on every `v*` tag → `choco install stagecoach` / `choco upgrade stagecoach`.
- **Package metadata** (from .goreleaser.yaml chocolateys: block): name=stagecoach, owners=dabstractor,
  title=Stagecoach, project_url=github.com/dabstractor/stagecoach, source_repo=push.chocolatey.org,
  api_key='{{ .Env.CHOCOLATEY_API_KEY }}'.
- **Secret**: `CHOCOLATEY_API_KEY` — Chocolatey community-source push key (chocolatey.org → Account Settings →
  API Key). Consumed by the goreleaser `chocolateys:` pipe inside the goreleaser job (release.yml header doc
  lines 13-15 + env line 62, P1.M2.T1.S2 COMPLETE).
- **The v3.3 rationale (the NON-reason — document it)**: Chocolatey was chosen OVER the Windows Store channel
  (winget / microsoft/winget-pkgs) because winget-pkgs runs a `validationDefender` install-in-clean-VM Microsoft
  Defender scan that HARD-BLOCKS the unsigned binary every release — an unbounded per-release tax. Chocolatey
  imposes no such gate (it publishes directly via the API key; no PR-based acceptance, no clean-VM scan).
- **Upgrade behavior**: `choco upgrade` needs admin, so `stagecoach upgrade` detects a Chocolatey install (FR-U2)
  and PRINTs `choco upgrade stagecoach -y` (FR-U4) — it does NOT self-swap (FR-U1: choco owns the binary under
  `ProgramData\chocolatey`). (detect.go ChannelChocolatey + delegate.go PRINT case — P1.M1 COMPLETE.)
- **NO bootstrap / NO manifest YAML / NO pending-acceptance checklist** — Chocolatey publishes directly via the
  API key on every release; there is no PR-acceptance gate to bootstrap or track. (The wingetcreate bootstrap,
  the NestedInstallerType/SHA256 YAML, and the "pending winget-pkgs acceptance" checklist are DELETED entirely.)

## 3. The PowerShell installer subsection facts (install.ps1, P1.M2.T1.S3 in-flight — document, don't create)

- **Invocation** (spec §21.3): `irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex` — the
  Windows analog of the Unix `curl|sh` one-liner (install.sh).
- **Audience**: the fallback for Windows users with NO package manager.
- **Behavior** (install.ps1 at repo root): detects arch ($env:PROCESSOR_ARCHITECTURE → amd64/arm64), downloads
  the matching `stagecoach_<v>_windows_<arch>.zip` + `_checksums.txt` from the latest GitHub Release, SHA256-
  verifies, extracts `stagecoach.exe` to `$LOCALAPPDATA\stagecoach` (rustup/starship/uv pattern; user-owned, no
  admin), prepends that dir to the USER `PATH`.
- **Install tag**: `STAGECOACH_INSTALL_METHOD=direct` (User env) → detect.go ChannelDirect → `stagecoach upgrade`
  self-swaps it (FR-U5).
- **Notice**: prints "Re-open your terminal for PATH changes to take effect."
- **Cross-ref**: PRD §21.3.

## 4. Scope fences (Mode A docs — ride with the work)

- **THIS task (S1) edits ONLY `docs/packaging.md`** (the WinGet section 8-73 + the intro line-4 mention).
- **NOT this task**: `docs/cli.md` + `README.md` winget refs → P1.M3.T1.S2 (sibling). `install.ps1` itself →
  P1.M2.T1.S3 (in-flight). `.goreleaser.yaml` chocolateys: + `release.yml` CHOCOLATEY_API_KEY → P1.M2.T1.S1/S2
  (COMPLETE, read-only).
- **Verification**: `rg -ni winget docs/packaging.md` == 0 hits (scrubs the intro line 4 + the whole section).

## 5. Style/conventions to match (existing docs/packaging.md)

- Maintainer-oriented prose + bullet lists + fenced code/sh blocks.
- Cross-refs as "PRD §21.2/§21.3" (the existing WinGet section used this form).
- H2 (`##`) for the channel section; H3 (`###`) for a subsection (the PowerShell installer).
- Secret docs as a `- **Secret**: NAME — …` bullet (matches the old WINGET_TOKEN bullet).