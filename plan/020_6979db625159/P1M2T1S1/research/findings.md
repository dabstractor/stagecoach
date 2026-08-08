# Research: P1.M2.T1.S1 — .goreleaser.yaml: chocolateys: pipe + scrub 4 WinGet comments

Two edits to ONE file, `.goreleaser.yaml` (165 lines): (a) add a goreleaser-native `chocolateys:` pipe
section after the `aurs:` block; (b) scrub all 4 WinGet comments (lines 78, 87, 104, 142). Pure config;
no Go code, no docs (the .goreleaser.yaml comments ARE the config documentation — Mode A; docs/packaging.md
is P1.M3.T1).

All claims verified against the current `.goreleaser.yaml`, release_plumbing.md, and a live
`goreleaser check` baseline.

---

## 0. BASELINE (verified live)

- **goreleaser v2.9.0** installed at `~/.local/bin/goreleaser` → **supports the `chocolateys:` pipe**
  (added in goreleaser v1.19; present in all v2.x).
- **`goreleaser check` on the CURRENT file PASSES** ("1 configuration file(s) validated / thanks for
  using GoReleaser!"). So the chocolateys: addition is the ONLY variable; if check fails after the edit,
  the new block is the cause (DECISION GATE: comment out the rejected field, re-check, ship the rest).

---

## 1. THE YAML KEY IS `chocolateys:` (PLURAL) — critical

The goreleaser Chocolatey struct's YAML key is **`chocolateys:`** (plural) — verified against goreleaser
master `pkg/config/config.go`. This matches the convention of EVERY other native pipe in the file:
`brews:`/`scoops:`/`nfpms:`/`aurs:` are all plural even though the prose says "the brew formula" /
"the nfpm pipe" / "the chocolatey pipe". The PRD §21.2 prose ("via goreleaser's native `chocolatey:`
pipe") and release_plumbing.md's section header ("New `chocolatey:` pipe section") are BOTH loose
singular prose — **the YAML key MUST be `chocolateys:` (plural)** or goreleaser will ignore the section
silently (unknown top-level key → no error, no publish). This is the #1 implementation trap.

---

## 2. THE STRUCTURAL TEMPLATE — the `scoops:` block (lines 100-111)

The chocolateys: block mirrors scoops: for the COMMON fields (name, ids, homepage→project_url,
description, license→license_url, url_template). The PUBLISH mechanism differs: scoops uses
`repository{owner,name,token}` (push to a GitHub bucket repo); chocolatey uses `api_key` + `source_repo`
(push to the Chocolatey community repo at push.chocolatey.org). So:
- KEEP from scoops: name, ids, url_template, description (and the README/LICENSE cross-refs as
  project_url/license_url).
- REPLACE scoops' `repository:` block with chocolatey's `api_key` + `source_repo`.

The `scoops:` block (verbatim, for reference):
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

## 3. THE chocolateys: BLOCK (the deliverable)

Insert AFTER the `aurs:` block (the `aurs:` block is the LAST pipe; it ends at line 165 / EOF). The
`---` YAML document terminator is NOT present (the file is one doc; the `---` I saw is not in the file —
verify; if present, insert before it). Place a `# Chocolatey` preamble comment (mirroring the brews:/
scoops:/nfpms:/aurs: preamble style) + the block.

Fields to use (from the contract; NOT all 24 struct fields — just these):
```yaml
# Chocolatey (Windows package manager) — goreleaser-native `chocolateys:` pipe (PRD §21.2/G27).
# Publishes a .nupkg to the community repo (chocolatey.org) so `choco install stagecoach` /
# `choco upgrade stagecoach` work. Chosen OVER winget (dropped): microsoft/winget-pkgs runs a
# Microsoft Defender install-gate that hard-blocks the unsigned binary every release; Chocolatey
# does not. `choco upgrade` needs admin, so `stagecoach upgrade` detects a Chocolatey install
# (FR-U2) and PRINTs `choco upgrade stagecoach -y` (FR-U4; do NOT self-swap — FR-U1: choco owns
# the binary under ProgramData\chocolatey).
# DECISION GATE: if `goreleaser check` rejects any field below, COMMENT OUT that field and ship
# the rest (same pattern as the `aurs:` block above). Chocolatey does not block the core contract.
chocolateys:
  - name: stagecoach
    ids:
      - default
    owners: dabstractor
    title: Stagecoach
    authors: Dustin Schultz
    project_url: https://github.com/dabstractor/stagecoach
    url_template: 'https://github.com/dabstractor/stagecoach/releases/download/{{ .Tag }}/{{ .ArtifactName }}'
    license_url: 'https://github.com/dabstractor/stagecoach/blob/main/LICENSE'   # repo is MIT (LICENSE file)
    copyright: 'Copyright (c) Dustin Schultz'   # ADJUST to the LICENSE file's actual copyright line/year
    description: 'Snapshot-based AI commit message generator that uses YOUR local CLI agent'
    release_notes: 'https://github.com/dabstractor/stagecoach/releases/tag/{{ .Tag }}'
    api_key: '{{ .Env.CHOCOLATEY_API_KEY }}'
    source_repo: 'https://push.chocolatey.org/'
```

Field-source notes:
- `license_url` → the LICENSE file URL (repo is MIT; `license:` in scoops/brews is the SPDX id `MIT`;
  chocolatey wants a URL). `https://github.com/dabstractor/stagecoach/blob/main/LICENSE`.
- `copyright` → match the LICENSE file. The repo LICENSE is "MIT License" (no explicit copyright line
  in the first 2 lines — the implementer should open LICENSE and use its copyright line, e.g.
  "Copyright (c) 2026 Dustin Schultz"; if none, a reasonable default is fine + an ADJUST comment).
- `release_notes` → the GitHub Release URL for the tag (goreleaser template).
- `api_key` → `CHOCOLATEY_API_KEY` env (the secret; P1.M2.T1.S2 adds it to release.yml's goreleaser job).
- `source_repo` → `https://push.chocolatey.org/` (the Chocolatey community repo push endpoint).
- NOT used (the contract doesn't list them): package_source_url, icon_url, require_license_acceptance,
  project_source_url, docs_url, bug_tracker_url, tags, summary, dependencies, skip_publish, goamd64.
  (Add tags/summary/etc. later if desired; the contract's field set is the minimum.)

---

## 4. THE 4 WinGet COMMENT SCRUBS (lines 78, 87, 104, 142)

Mechanical gate: `rg -ni winget .goreleaser.yaml` → **ZERO hits** after the edits.

### Line 78 (inside the `release:` block's "Beyond goreleaser" inventory)
Current:
`  #   • WinGet    — release.yml \`winget\` job → vedantmgoyal9/winget-releaser → microsoft/winget-pkgs`
Replace with (contract: "replace 'WinGet — release.yml winget job' with 'Chocolatey — goreleaser-native
chocolateys: pipe'"):
`  #   • Chocolatey — goreleaser-native \`chocolateys:\` pipe (Windows package manager; see below)`
(The bullet slot is repurposed: Winget (a Beyond-goreleaser release.yml CI job) is DROPPED; Chocolatey
is native. The new bullet text self-documents that it's native, not Beyond-goreleaser. Must NOT contain
"winget".)

### Lines 87, 104, 142 (the "Beyond goreleaser" enumeration `npm/WinGet/Nix/mise-asdf`)
Each contains the token sequence `npm/WinGet/Nix/mise-asdf`. Remove `WinGet/`:
- Line 87: `…channels — npm/WinGet/Nix/mise-asdf —` → `…channels — npm/Nix/mise-asdf —`
- Line 104: `…channels (npm/WinGet/Nix/mise-asdf)` → `…channels (npm/Nix/mise-asdf)`
- Line 142: `…channels — npm/WinGet/Nix/mise-asdf —` → `…channels — npm/Nix/mise-asdf —`

(Chocolatey is goreleaser-native → it leaves the Beyond-goreleaser list. Winget is dropped entirely.)

---

## 5. SCOPE & COORDINATION

- **ONLY `.goreleaser.yaml`.** No Go code, no release.yml (P1.M2.T1.S2 deletes the winget job + adds
  CHOCOLATEY_API_KEY), no install.ps1 (P1.M2.T1.S3), no docs/packaging.md (P1.M3.T1).
- **Parallel sibling P1.M1.T1.S2** edits `internal/upgrade/delegate.go` (Go delegation code) — NOT
  .goreleaser.yaml. No file overlap. (It renames ChannelWinget→ChannelChocolatey in detect.go; my task
  is the release-plumbing half — independent files.)
- **NO PRD.md / tasks.json / prd_snapshot.md** (read-only).

---

## 6. VALIDATION

- `goreleaser check` → must pass (baseline is green; the chocolateys: block is the only variable).
  DECISION GATE: on a field-rejection, comment out the rejected field, re-check, ship the rest.
- `rg -ni winget .goreleaser.yaml` → ZERO hits.
- `rg -n 'chocolateys:' .goreleaser.yaml` → exactly 1 hit (the new section header).
- (No Go build/test affected — this is a YAML config file. `make test`/`make lint` unchanged.)
- Optional deeper check: `goreleaser release --snapshot --clean` publishes NOTHING and exercises the
  full pipe machinery (would catch a template/field error `check` misses) — but it needs the build to
  succeed + the CHOCOLATEY_API_KEY env (unset → the chocolatey publish step is skipped, which is fine
  for a snapshot). Recommend `goreleaser check` as the primary gate (fast, no env needed).