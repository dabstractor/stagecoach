# System Context — Delta PRD Validation

## PRD Title
Chocolatey channel + PowerShell installer (replacing Winget) + model-example cleanup

## Validation Summary

| Area | PRD Claim | Codebase Reality | Match? |
|------|-----------|-----------------|--------|
| ChannelWinget exists | detect.go:41 | ✅ Confirmed at line 41 | ✅ |
| Winget pmProbe | detect.go:272 | ✅ Confirmed | ✅ |
| Winget validChannel entry | detect.go:64 | ✅ Confirmed | ✅ |
| "--install-method allowed-values slice" | detect.go:242 | ⚠️ Line 242 is `knownChannelList()` error-hint string, NOT a constraint slice. Flag registration is free-form `StringVar` in upgrade.go:139. The code change point is the same (update knownChannelList), but the naming in the PRD is imprecise. | ⚠️ Minor naming |
| Winget RUN case in delegate | delegate.go:273-274 | ✅ Confirmed | ✅ |
| PRINT set excludes winget | delegate.go:212 | ✅ Confirmed (AUR/Nix/Deb/Rpm) | ✅ |
| Winget has NO path heuristic | PRD says add one | ✅ Confirmed: winget absent from pathHeuristics (lines 350-353). Adding `ProgramData/chocolatey/` is new work. | ✅ |
| .goreleaser.yaml WinGet comments | lines 78,87,104,142 | ✅ All four confirmed exactly | ✅ |
| release.yml winget job | lines ~109-139 | ✅ Confirmed at 109-139 (banner 109-114, job 115-139) | ✅ |
| WINGET_TOKEN secret | release.yml line 13 | ✅ Confirmed | ✅ |
| install.ps1 does not exist | ❌ | ✅ Confirmed — does not exist anywhere | ✅ |
| install.sh exists (Unix analog) | Yes | ✅ Confirmed — 9002 bytes, executable | ✅ |
| No chocolatey: pipe | Absent | ✅ Confirmed | ✅ |
| packaging.md WinGet section | lines 8-73 | ✅ Confirmed (8-73, next section at 75) | ✅ |
| cli.md winget channel list | line ~404 | ✅ Confirmed at line 404 | ✅ |
| README.md v3.0 Winget blurb | line ~6 | ✅ Confirmed at line 6 | ✅ |
| README.md "Not yet available" | lines ~132-134 | ✅ Confirmed at lines 132-135 (1 line longer) | ✅ |

## Phase 2 Scope Expansion (CRITICAL FINDING)

**PRD claims:** ~4 files with stale `zai/glm-5.2` references (pi.toml, config_init_interactive.go, default_action.go, config_test.go), SP=1.

**Actual scope:** 90+ references across 30+ files. See `model_cleanup_scope.md` for full categorization.

The PRD's acceptance criterion (`rg` returns zero hits, OR hits with explanatory comments) IS achievable, but requires touching far more files than the PRD's task breakdown suggests. The breakdown expands Phase 2 into 2 Tasks with 5 Subtasks to cover the real scope.

## Architecture: Upgrade Detection Cascade

The upgrade subsystem uses a 4-tier cascade:
1. **Tier (a) — Explicit override:** `--install-method` flag or `STAGECOACH_INSTALL_METHOD` env var, validated by `validChannel()` (detect.go:62-68).
2. **Tier (b) — PM DB query:** `pmProbes` table (detect.go:267-276) — each probe runs a read-only ownership query. Currently 9 probes (brew, AUR, deb, rpm, scoop, winget, npm, mise, asdf). Confirm predicates: `exit0Confirm` (brew/scoop/pacman/dpkg/rpm) vs `grepConfirm` (winget/npm/mise/asdf).
3. **Tier (c) — Path heuristic:** `pathHeuristics` table (detect.go:350-353) — prefix matching on `realpath(ExePath)`. Cross-GOOS: both sides normalized to `/`. Currently 4 entries (2x brew, scoop, nix).
4. **Tier (d) — Default:** `ChannelDirect` — the only self-swap-eligible channel.

## Architecture: Delegate Dispatch

`Delegate()` (delegate.go:205) routes channels into two categories:
- **PRINT channels** (`ChannelAUR, ChannelNix, ChannelDeb, ChannelRpm`): command text printed to stdout, exits 0. User runs manually (needs root/immutable).
- **RUN channels** (everything else except direct): `runArgv()` builds the argv, streams the updater's output live.
- **DIRECT** (`ChannelDirect`): returns `ErrDirectSwap` sentinel; command layer handles self-swap.

**Chocolatey must be PRINT** (admin — FR-U4), moved from the RUN set to the PRINT set.

## Architecture: Release Pipeline

goreleaser native pipes present: `brews:` (89), `scoops:` (100), `nfpms:` (121), `aurs:` (146).
The `chocolatey:` pipe is absent and must be added. The winget automation is entirely external to goreleaser (CI job in release.yml).

## Key Design Decisions (validated)

1. **Chocolatey detection uses `exit0Confirm`** — `choco list --local-only stagecoach` exits 0 iff installed (unlike winget's `grepConfirm` approach).
2. **Chocolatey path heuristic uses forward slashes** — `ProgramData/chocolatey/` — matching the cross-GOOS normalization convention.
3. **install.ps1 tags `STAGECOACH_INSTALL_METHOD=direct`** — rides the existing direct self-swap channel (no new channel needed).
4. **CHOCOLATEY_API_KEY** is the new goreleaser secret (replaces WINGET_TOKEN).