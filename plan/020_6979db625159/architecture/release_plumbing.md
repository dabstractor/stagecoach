# Phase 1 — Release Plumbing Findings (.goreleaser.yaml + release.yml + install.ps1)

## File: `.goreleaser.yaml` (165 lines)

### Native pipe inventory (for structural reference)
| Pipe | Line | Repo Target | Secret |
|------|------|-------------|--------|
| `brews:` | 89 | `dabstractor/homebrew-stagecoach` | `HOMEBREW_TAP_GITHUB_TOKEN` |
| `scoops:` | 100 | `dabstractor/stagecoach-bucket` | `SCOOP_BUCKET_GITHUB_TOKEN` |
| `nfpms:` | 121 | (GitHub Release assets) | (none — local publish) |
| `aurs:` | 146 | AUR SSH push | `AUR_SSH_PRIVATE_KEY` |

### WinGet comments (all 4 confirmed)
| Line | Context | Text |
|------|---------|------|
| 78 | `release:` block inventory | `# • WinGet — release.yml winget job → vedantmgoyal9/winget-releaser → microsoft/winget-pkgs` |
| 87 | After `brews:` preamble | `# ...the "Beyond goreleaser" channels — npm/WinGet/Nix/mise-asdf —` |
| 104 | Inside `scoops:` block | `# Scoop is a goreleaser-native pipe; the "Beyond goreleaser" channels (npm/WinGet/Nix/mise-asdf)` |
| 142 | Inside `aurs:` preamble | `# ...the "Beyond goreleaser" channels — npm/WinGet/Nix/mise-asdf —` |

**Action:** Replace "WinGet" with "Chocolatey" in all four. Lines 78+142 note Chocolatey as a goreleaser-native pipe; lines 87+104 remove WinGet from the "Beyond goreleaser" list (Chocolatey IS goreleaser-native).

### New `chocolatey:` pipe section
Place after `aurs:` block (line ~165). Structural template: mirror `brews:`/`scoops:` (repository + token pattern). Required fields per goreleaser docs (verify at impl with `goreleaser check`):
- `package_name: stagecoach`
- `owners: dabstractor`
- `title: Stagecoach`
- `ids: [default]`
- `repository` (if publishing to a Chocolatey source) or direct community source
- `api_key: '{{ .Env.CHOCOLATEY_API_KEY }}'`
- `source_repo:` (Chocolatey community repo URL)
- `url_template`, `icon`, `copyright`, `license_url`, `project_url`, `description`, `release_notes`

**DECISION GATE:** If `goreleaser check` rejects any field, comment it out and ship the rest (same pattern as the `aurs:` block).

### Structural example from `scoops:` (line 100):
```yaml
scoops:
  - name: stagecoach
    ids:
      - default
    repository:
      owner: dabstractor
      name: stagecoach-bucket
      token: '{{ .Env.SCOOP_BUCKET_GITHUB_TOKEN }}'
    homepage: https://github.com/dabstractor/stagecoach
    description: 'Snapshot-based AI commit message generator that uses YOUR local CLI agent'
    license: MIT
    url_template: 'https://github.com/dabstractor/stagecoach/releases/download/{{ .Tag }}/{{ .ArtifactName }}'
```

---

## File: `.github/workflows/release.yml` (236 lines)

### winget: job — lines 109-139 (DELETE ENTIRELY)
```yaml
  # --- WinGet ... banner ---   (lines 109-114)
  winget:                       (line 115)
    name: WinGet manifest PR    (line 116)
    needs: goreleaser           (line 117)
    if: ${{ !cancelled() }}     (line 118)
    runs-on: windows-latest     (line 119)
    steps:                      (line 120)
      - name: Open winget-pkgs manifest PR ... (line 121)
        continue-on-error: true (line 126)
        uses: vedantmgoyal9/winget-releaser@v2  (line 127)
        with:
          identifier: dabstractor.Stagecoach  (line 129)
          installers-regex: 'windows_amd64\.zip$'  (line 133)
          fork-user: dabstractor  (line 137)
          max-versions-to-keep: '0'  (line 138)
          token: ${{ secrets.WINGET_TOKEN }}  (line 139)
```

### WINGET_TOKEN header doc — line 13-14 (DELETE)
```yaml
#   WINGET_TOKEN                classic PAT, public_repo scope — forks microsoft/winget-pkgs to
#                              dabstractor/winget-pkgs + opens the manifest PR. Settings → Secrets → Actions.
```

**Safe to remove:** The winget job uses `continue-on-error: true` + `if: ${{ !cancelled() }}`. No other job depends on it (npm-publish, asdf-mirror, apt-dnf-repo all use `needs: goreleaser`, not winget).

### CHOCOLATEY_API_KEY addition
Add `CHOCOLATEY_API_KEY` to the goreleaser job's env (the native `chocolatey:` pipe runs within the goreleaser job). Also document it in the header secrets section.

---

## File: `install.sh` (repo root, 9002 bytes) — the template for install.ps1

### What install.sh does (the pattern to mirror):
1. Detects architecture (uname -m → amd64/arm64)
2. Resolves the latest release tag (GitHub Releases API, unauthenticated)
3. Downloads `stagecoach_<version>_<os>_<arch>.tar.gz` + `checksums.txt`
4. SHA256-verifies the archive against the checksums line (hard gate — abort on mismatch)
5. Extracts `stagecoach` to a user-local dir
6. Prepends to PATH
7. Tags `STAGECOACH_INSTALL_METHOD=direct`

### install.ps1 requirements (from PRD P1.M2.T1):
- `$env:PROCESSOR_ARCHITECTURE` → amd64/arm64 detection
- GitHub Releases API (unauthenticated) → latest release tag
- Download `stagecoach_<v>_windows_<arch>.zip` + `checksums.txt`
- SHA256-verify the zip (hard gate — abort on mismatch, like `internal/upgrade/download.go`)
- Extract `stagecoach.exe` to `$LOCALAPPDATA\stagecoach`
- Prepend to **user** PATH (`[Environment]::SetEnvironmentVariable(..., 'User')` — NOT Machine, no admin)
- Set `STAGECOACH_INSTALL_METHOD=direct` in **user** environment
- Print "re-open your shell" notice
- Dependency-free (no PowerShell gallery modules)
- Header comment cross-referencing spec §21.3
- Invocable via: `irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex`