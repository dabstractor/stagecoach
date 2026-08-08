name: "P1.M2.T1.S1 — .goreleaser.yaml: add chocolateys: pipe section + scrub 4 WinGet comments"
description: >
  Two edits to ONE file (.goreleaser.yaml): (a) add a goreleaser-native `chocolateys:` pipe section after
  the `aurs:` block (the Windows package manager — replaces the dropped WinGet channel; PRD §21.2/G27),
  mirroring the existing scoops: block for common fields but using api_key+source_repo (push to
  chocolatey.org) instead of repository{owner,name,token}; (b) scrub all 4 WinGet comments (lines 78, 87,
  104, 142): line 78's bullet → a Chocolatey-native pointer; lines 87/104/142 remove `WinGet/` from the
  `npm/WinGet/Nix/mise-asdf` Beyond-goreleaser enumeration. The YAML key is the PLURAL `chocolateys:`
  (verified against goreleaser master config; matches brews:/scoops:/nfpms:/aurs: — NOT the singular
  "chocolatey:" the PRD/release_plumbing prose uses). DECISION GATE: run `goreleaser check` (v2.9.0 is
  installed; current file PASSES as a baseline) — if it rejects any field, comment it out and ship the
  rest (same pattern as the aurs: block). Mode A: the .goreleaser.yaml comments ARE the config docs.
  No Go code, no release.yml (P1.M2.T1.S2), no install.ps1 (P1.M2.T1.S3), no docs/packaging.md (P1.M3.T1).

---

## Goal

**Feature Goal**: Make goreleaser publish a Chocolatey `.nupkg` to the community repo
(push.chocolatey.org) on every release, so `choco install stagecoach` / `choco upgrade stagecoach` work —
the Windows package-manager channel that REPLACES the dropped WinGet channel (whose microsoft/winget-pkgs
Microsoft Defender install-gate blocked every release). Simultaneously scrub every WinGet reference from
`.goreleaser.yaml` so the config reflects the Chocolatey-only Windows story.

**Deliverable**: ONE modified file, `.goreleaser.yaml`:
1. A new `chocolateys:` pipe section (plural key) after the `aurs:` block, with a Chocolatey preamble
   comment + the contract's field set (name/ids/owners/title/authors/project_url/url_template/license_url/
   copyright/description/release_notes/api_key/source_repo).
2. Four WinGet comment scrubs: line 78 bullet → Chocolatey-native pointer; lines 87/104/142 → drop
   `WinGet/` from the Beyond-goreleaser enumeration.

**Success Definition**:
- `goreleaser check` PASSES with the chocolateys: section present (baseline: current file passes on
  goreleaser v2.9.0). On any field rejection, the rejected field is commented out and the rest ships
  (DECISION GATE, mirroring the aurs: block).
- `rg -ni winget .goreleaser.yaml` → **ZERO hits**.
- `rg -n '^chocolateys:' .goreleaser.yaml` → exactly 1 hit.
- The chocolateys: section is structurally valid YAML at the top level (plural key, 2-space indent under
  `  - name:`, matching the scoops:/aurs: style).
- No other pipe (brews:/scoops:/nfpms:/aurs:) or the release:/builds:/archives:/checksum:/changelog:
  sections are changed.

## User Persona (if applicable)

**Target User**: A Windows developer who wants to install/upgrade stagecoach via the standard Windows
package manager (`choco install stagecoach`), and the release engineer running the tag-triggered release.

**Use Case**: User runs `choco install stagecoach` → chocolatey.org serves the `.nupkg` goreleaser
pushed → stagecoach lands on PATH. Later, `stagecoach upgrade` detects the choco install (FR-U2) and
PRINTs `choco upgrade stagecoach -y` (FR-U4; admin) instead of self-swapping a choco-owned binary (FR-U1).

**User Journey**: maintainer tags `v1.2.3` → release.yml's goreleaser job runs → the native chocolateys:
pipe builds + pushes the `.nupkg` to push.chocolatey.org (using CHOCOLATEY_API_KEY) → Windows users
`choco install stagecoach` immediately.

**Pain Points Addressed**: G27/§21.2 — WinGet's per-release Microsoft Defender install-gate
(validationDefender in microsoft/winget-pkgs) hard-blocked the unsigned binary EVERY release, an
unbounded tax. Chocolatey imposes no such gate and is goreleaser-native (one pipe, no separate CI job).

## Why

- **G27/§21.2 (Chocolatey replaces WinGet)**: Chocolatey is the established Windows package manager, and
  crucially it is a goreleaser-NATIVE pipe — so it needs NO separate CI job (unlike WinGet's
  winget-releaser Action) and NO microsoft/winget-pkgs PR (no Defender gate). This task adds the pipe.
- **Consistency**: the chocolateys: block mirrors the existing scoops: block (the Windows-native pipe
  that already works) for all common fields; only the publish mechanism differs (api_key+source_repo).
- **Bounded scope**: one config file, two edits. The Go-side detection/delegation (ChannelChocolatey) is
  the parallel P1.M1 milestone; release.yml's winget-job deletion + CHOCOLATEY_API_KEY is P1.M2.T1.S2;
  install.ps1 is P1.M2.T1.S3; docs/packaging.md is P1.M3.T1. This task is the goreleaser pipe only.

## What

**User-visible behavior**: None directly (this is release plumbing). Effect: on the next tag-triggered
release, goreleaser publishes a Chocolatey `.nupkg`, enabling `choco install stagecoach`.

**Technical change** (one file, two edits; verbatim content in the Blueprint):
1. **chocolateys: section** — inserted after the `aurs:` block (end of file), plural key, mirroring
   scoops: for common fields, api_key+source_repo for publish.
2. **4 WinGet comment scrubs** — line 78 bullet repurposed to Chocolatey-native; lines 87/104/142 drop
   `WinGet/` from the Beyond-goreleaser enumeration.

### Success Criteria
- [ ] `goreleaser check` passes with the chocolateys: section (or, per DECISION GATE, passes with the
  rejected field commented out + a note)
- [ ] `rg -ni winget .goreleaser.yaml` → 0 hits
- [ ] `rg -n '^chocolateys:' .goreleaser.yaml` → 1 hit (the PLURAL key)
- [ ] chocolateys: block has the contract's field set (name/ids/owners/title/authors/project_url/
  url_template/license_url/copyright/description/release_notes/api_key/source_repo)
- [ ] `license_url` points at the LICENSE file; `api_key` is `{{ .Env.CHOCOLATEY_API_KEY }}`;
  `source_repo` is `https://push.chocolatey.org/`
- [ ] no other pipe/section changed; 2-space indent; valid YAML
- [ ] only `.goreleaser.yaml` modified

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim chocolateys: block (every field + value), the plural-key trap, the exact 4
WinGet-comment before/after texts, the structural template (scoops:), the live baseline (goreleaser v2.9.0
installed; current file passes check), the DECISION GATE procedure, and the explicit scope fences.

### Documentation & References

```yaml
# MUST READ — the authoritative research (verbatim block + the plural-key trap + the 4 scrubs + baseline)
- docfile: plan/020_6979db625159/P1M2T1S1/research/findings.md
  why: "§0 the live baseline (goreleaser v2.9.0; current file passes check); §1 the PLURAL-key trap;
        §2 the scoops: structural template; §3 the verbatim chocolateys: block + field-source notes;
        §4 the 4 WinGet scrubs with exact before/after; §5 scope/coordination; §6 validation."
  critical: "§1: the YAML key is `chocolateys:` (PLURAL) — the PRD/release_plumbing prose says singular
             `chocolatey:` but the struct key is plural (matches brews:/scoops:/nfpms:/aurs:). A singular
             key is silently ignored (no error, no publish). §0: run `goreleaser check` — it's installed
             and the baseline is green, so the chocolateys: block is the only variable."

# MUST READ — the release-plumbing findings (the chocolateys field list + the 4 WinGet lines confirmed)
- docfile: plan/020_6979db625159/architecture/release_plumbing.md
  why: "The '.goreleaser.yaml' section confirms the native-pipe inventory (brews:89/scoops:100/nfpms:121/
        aurs:146), tabulates all 4 WinGet comment lines (78/87/104/142) with their exact text, gives the
        chocolateys field list (package_name/owners/title/ids/repository-or-community-source/api_key/
        source_repo/url_template/icon/copyright/license_url/project_url/description/release_notes), the
        'mirror scoops:' structural guidance, and the DECISION GATE ('if goreleaser check rejects any
        field, comment it out and ship the rest — same pattern as aurs:')."
  critical: "Its header says 'chocolatey:' (singular) — that is LOOSE PROSE; the YAML key is plural
             `chocolateys:` (findings §1, verified against goreleaser master config). Follow the plural key."

# MUST READ — the file being edited
- file: .goreleaser.yaml
  why: "165 lines. The `scoops:` block (100-111) is the STRUCTURAL TEMPLATE (name/ids/repository/
        homepage/description/license/url_template). The `aurs:` block (146-165) is the LAST pipe —
        insert chocolateys: AFTER it (end of file). The 4 WinGet comments are at 78 (release: block
        Beyond-goreleaser bullet), 87 (brews: preamble), 104 (scoops: inline), 142 (aurs: preamble)."
  pattern: "Every native pipe is a top-level PLURAL key (`brews:`/`scoops:`/`nfpms:`/`aurs:`) with a
            `  - name: …` list item, 2-space indent, field-per-line. A `# Preamble` comment block above
            each explains the pipe + carries a DECISION GATE note where fields are uncertain (aurs:)."
  gotcha: "LOCATE the insertion point + the 4 comments by CONTENT (grep), not line number — any sibling
           edit shifts lines. `grep -n 'aurs:' .goreleaser.yaml` finds the last pipe; insert after its
           last field. `grep -ni winget .goreleaser.yaml` finds the 4 comments."

# MUST READ — the goreleaser Chocolatey docs (the struct field reference)
- url: https://goreleaser.com/customization/chocolatey/
  why: "The authoritative field list for the chocolateys: pipe: name, ids, package_source_url, owners,
        title, authors, project_url, url_template, icon_url, copyright, license_url,
        require_license_acceptance, project_source_url, docs_url, bug_tracker_url, tags, summary,
        description, release_notes, dependencies, skip_publish, api_key, source_repo, goamd64. Confirms
        the PLURAL `chocolateys:` key and the api_key+source_repo publish mechanism (vs scoops' repository)."
  critical: "This task uses only the contract's SUBSET of fields (name/ids/owners/title/authors/
             project_url/url_template/license_url/copyright/description/release_notes/api_key/source_repo).
             If `goreleaser check` rejects one, comment it out (DECISION GATE) — do NOT substitute a
             different field name guessing."

# CONTEXT — the PRD spec (Chocolatey replaces WinGet; §21.2)
- docfile: plan/020_6979db625159/prd_snapshot.md
  why: "§21.2 (h3.103) documents the Chocolatey-over-WinGet decision (Defender install-gate) and the
        FR-U1/U2/U4 upgrade behavior (choco owns the binary; PRINT choco upgrade -y). §21.3 (h3.104)
        shows `choco install stagecoach` in the install-paths list. Cite these in the chocolateys: preamble."
  section: "§21.2 goreleaser (Chocolatey bullet); §21.3 Install paths (choco install)"

# CONTEXT — the parallel sibling (NO overlap; confirms the channel rename)
- docfile: plan/020_6979db625159/P1M1T1S2/PRP.md
  why: "Parallel P1.M1.T1.S2 edits internal/upgrade/delegate.go (Go: ChannelWinget→ChannelChocolatey,
        RUN→PRINT). It does NOT touch .goreleaser.yaml. No file overlap. Read it to confirm this task is
        the release-plumbing half (independent file)."
  critical: "Do NOT edit release.yml (P1.M2.T1.S2 deletes the winget job + adds CHOCOLATEY_API_KEY) or
             install.ps1 (P1.M2.T1.S3) or docs/packaging.md (P1.M3.T1) — each is a separate subtask."
```

### Current Codebase tree (relevant slice)

```bash
.goreleaser.yaml        # EDIT — +chocolateys: section (after aurs:) + scrub 4 WinGet comments (78/87/104/142)
LICENSE                 # READ-ONLY — MIT; the license_url target (github.com/dabstractor/stagecoach/blob/main/LICENSE)
.github/workflows/release.yml  # READ-ONLY / NOT this task — P1.M2.T1.S2 deletes the winget job + adds CHOCOLATEY_API_KEY
internal/upgrade/delegate.go   # READ-ONLY / NOT this task — parallel P1.M1.T1.S2 (Go ChannelChocolatey)
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (no new files):
.goreleaser.yaml   # +chocolateys: pipe section (plural key) + 4 WinGet comment scrubs
```

### Known Gotchas of our codebase & Library Quirks

```yaml
# CRITICAL (the YAML key is PLURAL `chocolateys:`): goreleaser's Chocolatey struct key is `chocolateys:`
#   (verified against goreleaser master pkg/config/config.go; matches brews:/scoops:/nfpms:/aurs:). The
#   PRD §21.2 prose ("via goreleaser's native `chocolatey:` pipe") and release_plumbing.md's header
#   ("New `chocolatey:` pipe section") are BOTH singular LOOSE PROSE. A singular `chocolatey:` key is a
#   top-level UNKNOWN key → goreleaser SILENTLY ignores it (no error in check, no publish). Use `chocolateys:`.

# CRITICAL (DECISION GATE — run goreleaser check, it's installed): goreleaser v2.9.0 is at
#   ~/.local/bin/goreleaser and the CURRENT file passes `goreleaser check` (baseline green). After adding
#   chocolateys:, re-run `goreleaser check`. If it rejects a field, COMMENT OUT that one field (with a
#   `# DECISION GATE:` note) and re-check until green — same pattern as the `aurs:` block (line ~143:
#   "if `goreleaser check` rejects any field below, COMMENT OUT this whole `aurs:` block and ship the
#   rest"). Do NOT guess alternative field names.

# CRITICAL (api_key + source_repo, NOT repository): unlike scoops:/brews: (which push to a GitHub repo
#   via repository{owner,name,token}), chocolatey pushes to the community repo via `api_key` + `source_repo`
#   ('https://push.chocolatey.org/'). Do NOT copy scoops' `repository:` block into chocolateys:.

# GOTCHA (license_url is a URL, not SPDX): scoops:/brews:/aurs: use `license: MIT` (SPDX id). Chocolatey
#   has NO `license` field — it has `license_url` (a URL). Point it at the LICENSE file:
#   https://github.com/dabstractor/stagecoach/blob/main/LICENSE (repo is MIT).

# GOTCHA (copyright must match LICENSE): open the LICENSE file and use its copyright line. The first 2
#   lines are "MIT License" (no copyright line visible in the first 2) — the implementer should read the
#   full LICENSE for the holder/year and use e.g. "Copyright (c) 2026 Dustin Schultz". If unclear, use a
#   reasonable default + an `# ADJUST` comment.

# GOTCHA (scrub ALL winget, case-insensitive): the success gate is `rg -ni winget` = 0 hits. The 4
#   comments use "WinGet" (camelCase) and "winget" (lowercase, in `release.yml \`winget\` job` and
#   `winget-releaser`/`winget-pkgs`). The line-78 replacement text must NOT contain "winget" in ANY case
#   (so no "replaces the dropped WinGet channel" — that would fail the gate). Use "Windows package manager".

# GOTCHA (locate by content, not line number): the 4 WinGet lines (78/87/104/142) and the aurs: insertion
#   point (146-165) are line-numbers-as-of-now; any sibling edit shifts them. Use
#   `grep -ni winget .goreleaser.yaml` and `grep -n 'aurs:' .goreleaser.yaml` to locate.

# GOTCHA (indentation + YAML validity): the chocolateys: block is a top-level PLURAL key with a
#   `  - name: stagecoach` list item (2-space indent for the dash, 4-space for fields under it), matching
#   scoops:/aurs:. Run `goreleaser check` (which parses the YAML) — a parse error means a bad indent.
```

## Implementation Blueprint

### Data models and structure
None. Pure YAML config. No code, no types.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT .goreleaser.yaml — add the chocolateys: section after the aurs: block
  - LOCATE the insertion point: `grep -n 'aurs:' .goreleaser.yaml` → the aurs: block is the LAST pipe.
    Insert the new section AFTER the aurs: block's last field (`commit_author{...}`) and before EOF.
  - INSERT the Chocolatey preamble comment + the chocolateys: block (verbatim from research/findings.md §3):
        # Chocolatey (Windows package manager) — goreleaser-native `chocolateys:` pipe (PRD §21.2/G27).
        # Publishes a .nupkg to the community repo (chocolatey.org) so `choco install stagecoach` /
        # `choco upgrade stagecoach` work. Chosen OVER the dropped WinGet channel: microsoft/winget-pkgs
        # runs a Microsoft Defender install-gate that hard-blocks the unsigned binary every release;
        # Chocolatey does not. `choco upgrade` needs admin, so `stagecoach upgrade` detects a Chocolatey
        # install (FR-U2) and PRINTs `choco upgrade stagecoach -y` (FR-U4; do NOT self-swap — FR-U1:
        # choco owns the binary under ProgramData\chocolatey).
        # DECISION GATE: if `goreleaser check` rejects any field below, COMMENT OUT that field and ship
        # the rest (same pattern as the `aurs:` block above).
        chocolateys:
          - name: stagecoach
            ids:
              - default
            owners: dabstractor
            title: Stagecoach
            authors: Dustin Schultz
            project_url: https://github.com/dabstractor/stagecoach
            url_template: 'https://github.com/dabstractor/stagecoach/releases/download/{{ .Tag }}/{{ .ArtifactName }}'
            license_url: 'https://github.com/dabstractor/stagecoach/blob/main/LICENSE'
            copyright: 'Copyright (c) Dustin Schultz'   # ADJUST to the LICENSE file's actual copyright line/year
            description: 'Snapshot-based AI commit message generator that uses YOUR local CLI agent'
            release_notes: 'https://github.com/dabstractor/stagecoach/releases/tag/{{ .Tag }}'
            api_key: '{{ .Env.CHOCOLATEY_API_KEY }}'
            source_repo: 'https://push.chocolatey.org/'
  - KEY: `chocolateys:` (PLURAL). INDENT: 2-space dash, 4-space fields (mirror scoops:).
  - NOTE the preamble contains NO "winget" (the Defender-gate sentence says "the dropped WinGet channel"
    — WAIT that has Winget; reword to "a dropped Windows store channel" to keep `rg -ni winget` = 0).
    (See Task 1 correction below: the preamble must be winget-free.)

Task 1 (correction — preamble must be winget-free): the preamble's third sentence must NOT contain
  "WinGet"/"winget" (the success gate is `rg -ni winget` = 0). Reword:
    "...Chosen for the Windows package-manager audience; the previous Windows store channel was dropped
    (its repo ran a Microsoft Defender install-gate that hard-blocked the unsigned binary every release;
    Chocolatey does not)...."
  (No occurrence of winget in any case.)

Task 2: EDIT .goreleaser.yaml — scrub the 4 WinGet comments (locate via `grep -ni winget .goreleaser.yaml`)
  - LINE ~78 (release: block, Beyond-goreleaser bullet):
      REPLACE `  #   • WinGet    — release.yml \`winget\` job → vedantmgoyal9/winget-releaser → microsoft/winget-pkgs`
      WITH    `  #   • Chocolatey — goreleaser-native \`chocolateys:\` pipe (Windows package manager; see below)`
  - LINE ~87 (brews: preamble):
      REPLACE the `npm/WinGet/Nix/mise-asdf` token sequence WITH `npm/Nix/mise-asdf` (drop `WinGet/`).
  - LINE ~104 (scoops: inline):
      REPLACE `(npm/WinGet/Nix/mise-asdf)` WITH `(npm/Nix/mise-asdf)`.
  - LINE ~142 (aurs: preamble):
      REPLACE the `npm/WinGet/Nix/mise-asdf` token sequence WITH `npm/Nix/mise-asdf`.
  - VERIFY: `rg -ni winget .goreleaser.yaml` → ZERO hits.

Task 3: VERIFY — goreleaser check (DECISION GATE) + grep guards
  - goreleaser check                       # MUST pass; on a field rejection, comment out the field + re-check
  - rg -ni winget .goreleaser.yaml         # MUST be 0 hits
  - rg -n '^chocolateys:' .goreleaser.yaml # MUST be 1 hit (plural)
  - (optional deeper) goreleaser release --snapshot --clean   # publishes nothing; exercises the pipe
    machinery (needs CHOCOLATEY_API_KEY unset → chocolatey publish skipped, which is fine for a snapshot)
```

### Implementation Patterns & Key Details

```yaml
# PATTERN: the chocolateys: block (plural key; mirror scoops: for common fields; api_key+source_repo for publish)
chocolateys:
  - name: stagecoach
    ids:
      - default
    owners: dabstractor
    title: Stagecoach
    authors: Dustin Schultz
    project_url: https://github.com/dabstractor/stagecoach
    url_template: 'https://github.com/dabstractor/stagecoach/releases/download/{{ .Tag }}/{{ .ArtifactName }}'
    license_url: 'https://github.com/dabstractor/stagecoach/blob/main/LICENSE'
    copyright: 'Copyright (c) Dustin Schultz'   # ADJUST to LICENSE
    description: 'Snapshot-based AI commit message generator that uses YOUR local CLI agent'
    release_notes: 'https://github.com/dabstractor/stagecoach/releases/tag/{{ .Tag }}'
    api_key: '{{ .Env.CHOCOLATEY_API_KEY }}'
    source_repo: 'https://push.chocolatey.org/'

# PATTERN: the WinGet scrubs (token removal / bullet repurpose)
#   line 78 bullet:  WinGet-via-release.yml bullet  →  Chocolatey-native pointer
#   lines 87/104/142: `npm/WinGet/Nix/mise-asdf`  →  `npm/Nix/mise-asdf`
```

### Integration Points

```yaml
NO code / tests / routes / new deps / new CLI flags. ONE YAML config file edited.

RELEASE PLUMBING (.goreleaser.yaml):
  - +chocolateys: pipe section (plural key, after aurs:) → publishes .nupkg to push.chocolatey.org on release.
  - 4 WinGet comments scrubbed (line 78 bullet repurposed; 87/104/142 drop WinGet/ from Beyond-goreleaser list).

COORDINATION (separate subtasks, NOT this one):
  - release.yml: P1.M2.T1.S2 deletes the winget job (lines ~109-139) + the WINGET_TOKEN header doc +
    adds CHOCOLATEY_API_KEY to the goreleaser job env. (The chocolateys: pipe runs WITHIN the goreleaser
    job, so the API key must be in that job's env — S2's concern, not this file's.)
  - install.ps1: P1.M2.T1.S3 (Windows curl|sh analog).
  - docs/packaging.md: P1.M3.T1 (WinGet→Chocolatey section rewrite).
  - internal/upgrade/delegate.go: parallel P1.M1.T1.S2 (Go ChannelChocolatey PRINT path).

SCOPE FENCES: NO release.yml; NO install.ps1; NO docs; NO Go code; NO PRD.md/tasks.json/prd_snapshot.md.
```

## Validation Loop

> This is a YAML config file. The primary gate is `goreleaser check` (installed; baseline green) + grep
> guards. `make test`/`make lint` (Go) are unaffected but confirm the tree is otherwise clean.

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# THE gate: goreleaser config validation (v2.9.0 installed; current file passes as baseline).
goreleaser check
# Expected: "1 configuration file(s) validated / thanks for using GoReleaser!"
# DECISION GATE: on a field rejection, COMMENT OUT the rejected field (with a `# DECISION GATE:` note),
# re-run `goreleaser check`, ship the rest. Do NOT guess alternative field names.

# YAML parse sanity (independent of goreleaser; catches a bad indent).
python3 -c "import yaml,sys; yaml.safe_load(open('.goreleaser.yaml')); print('YAML OK')" 2>/dev/null || \
  yq e '.' .goreleaser.yaml >/dev/null 2>&1 && echo 'YAML OK' || echo "(no yaml/yq — rely on goreleaser check)"

# Scope guard: only .goreleaser.yaml changed.
git diff --name-only
# Expected: .goreleaser.yaml (only).
```

### Level 2: Unit Tests (Component Validation)

```bash
# No Go tests are authored or affected by this config edit. Run the suite ONLY to prove the tree is clean
# (parallel Go work may be in flight, but this YAML edit cannot break Go tests).
make test
# Expected: green (race detector). Unaffected by this YAML edit.
```

### Level 3: Integration Testing (System Validation)

```bash
# The definitive integration check: a SNAPSHOT release exercises the full pipe machinery (build + every
# native pipe, including chocolateys:) WITHOUT publishing anything. Needs the build to succeed; the
# CHOCOLATEY_API_KEY env is unset → the chocolatey publish step is SKIPPED (goreleaser warns, does not
# fail) — which is the correct, expected behavior for a local snapshot.
goreleaser release --snapshot --clean 2>&1 | grep -iE 'chocolatey|nupkg|publishing|skipping' || true
# Expected: a line showing the chocolateys: pipe ran (built the .nupkg) and skipped the push (no API key).
# This is OPTIONAL and slower than `check` — `goreleaser check` (Level 1) is the primary gate. If the
# build itself fails for an unrelated reason, rely on `check`.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1 (THE mechanical gate): zero WinGet references in any case.
rg -ni winget .goreleaser.yaml
# Expected: NO output (0 hits).

# Grep guard 2: the chocolateys: section is present exactly once, with the PLURAL key.
rg -n '^chocolateys:' .goreleaser.yaml
# Expected: exactly 1 hit. (Confirm it is NOT `chocolatey:` singular — `rg -n '^chocolatey:' .goreleaser.yaml` → 0.)

# Grep guard 3: the contract's field set is present in the chocolateys: block.
rg -n 'name: stagecoach|owners: dabstractor|title: Stagecoach|authors: Dustin Schultz|project_url:|url_template:|license_url:|copyright:|api_key: .*CHOCOLATEY_API_KEY|source_repo: .*push.chocolatey.org' .goreleaser.yaml
# Expected: ≥1 hit for each (the field is present in the new block).

# Grep guard 4: api_key uses the CHOCOLATEY_API_KEY env var (the secret P1.M2.T1.S2 wires into release.yml).
rg -n "api_key: '{{ .Env.CHOCOLATEY_API_KEY }}'" .goreleaser.yaml
# Expected: 1 hit.

# Grep guard 5: the OTHER pipes are UNCHANGED.
rg -c '^brews:|^scoops:|^nfpms:|^aurs:' .goreleaser.yaml
# Expected: each present exactly once (4 total) — unchanged.

# Grep guard 6: no stray `repository:` block inside chocolateys: (it uses api_key+source_repo, NOT repository).
awk '/^chocolateys:/{f=1} f&&/^  - /{print} /^chocolateys:/{p=1} p&&/repository:/{print "BAD: repository in chocolateys:"; exit}' .goreleaser.yaml
# Expected: no "BAD: repository in chocolateys:" output. (The chocolateys: block has api_key+source_repo.)

# Scope guard: only .goreleaser.yaml modified; no release.yml/install.ps1/docs touched.
git diff --name-only | grep -v '^\.goreleaser\.yaml$'
# Expected: empty (no output).
```

## Final Validation Checklist

### Technical Validation
- [ ] `goreleaser check` passes with the chocolateys: section (or, per DECISION GATE, with the rejected field commented out + a note)
- [ ] YAML parses (goreleaser check confirms; optional python/yq sanity)
- [ ] `make test` green (unaffected — confirms a clean tree)

### Feature Validation
- [ ] `rg -ni winget .goreleaser.yaml` → 0 hits
- [ ] `rg -n '^chocolateys:' .goreleaser.yaml` → 1 hit (PLURAL key; `^chocolatey:` singular → 0)
- [ ] chocolateys: block has the contract field set (name/ids/owners/title/authors/project_url/url_template/license_url/copyright/description/release_notes/api_key/source_repo)
- [ ] `api_key: '{{ .Env.CHOCOLATEY_API_KEY }}'`; `source_repo: 'https://push.chocolatey.org/'`; `license_url` → LICENSE file
- [ ] (optional) `goreleaser release --snapshot --clean` shows the chocolateys: pipe ran + skipped push (no API key)

### Scope-Boundary Validation
- [ ] `git diff --name-only` == only `.goreleaser.yaml`
- [ ] NO release.yml edit (P1.M2.T1.S2); NO install.ps1 (P1.M2.T1.S3); NO docs/packaging.md (P1.M3.T1)
- [ ] NO Go code edit (parallel P1.M1.T1.S2 owns detect.go/delegate.go)
- [ ] brews:/scoops:/nfpms:/aurs:/release:/builds:/archives:/checksum:/changelog: sections unchanged

### Code Quality & Docs
- [ ] chocolateys: preamble cites PRD §21.2/G27 + FR-U1/U2/U4 + the Defender-gate rationale (winget-free wording)
- [ ] 2-space indent; field-per-line; mirror scoops:/aurs: style
- [ ] DECISION GATE note present (mirrors the aurs: block's pattern)

---

## Anti-Patterns to Avoid

- ❌ Don't use the singular `chocolatey:` key. The goreleaser struct key is PLURAL `chocolateys:` (matches
  brews:/scoops:/nfpms:/aurs:). The PRD §21.2 prose and release_plumbing.md's header both say singular —
  that's loose prose; a singular key is silently ignored (no error, no publish). Use `chocolateays:`...
  use `chocolateys:`.
- ❌ Don't copy scoops' `repository:` block into chocolateys:. Chocolatey pushes to the community repo via
  `api_key` + `source_repo` (`https://push.chocolatey.org/`), NOT a GitHub bucket repo. A `repository:`
  block in chocolateys: is either rejected or wrong.
- ❌ Don't use `license: MIT` in chocolateys:. Chocolatey has `license_url` (a URL), not `license` (SPDX).
  Point it at `https://github.com/dabstractor/stagecoach/blob/main/LICENSE`.
- ❌ Don't leave any "WinGet"/"winget" string in the file. The success gate is `rg -ni winget` = 0. The
  line-78 replacement AND the chocolateys: preamble must both be winget-free (no "replaces the dropped
  WinGet channel" — reword to "Windows store channel" or similar). Check with the grep before finishing.
- ❌ Don't guess alternative field names if `goreleaser check` rejects one. Comment out the rejected field
  (with a `# DECISION GATE:` note) and ship the rest — exactly the aurs: block's documented pattern.
  Guessing a wrong field name either fails check again or is silently ignored.
- ❌ Don't edit release.yml, install.ps1, docs/packaging.md, or any Go file. Each is a separate subtask
  (P1.M2.T1.S2/S3, P1.M3.T1, P1.M1.T1.S2). This task is the `.goreleaser.yaml` pipe + comment scrub ONLY.
- ❌ Don't locate the 4 comments or the insertion point by line number. Lines shift on any sibling edit.
  Use `grep -ni winget .goreleaser.yaml` (the 4 comments) and `grep -n 'aurs:' .goreleaser.yaml` (insert
  after the last pipe).
- ❌ Don't add fields the contract doesn't list (package_source_url, icon_url, tags, summary, etc.) unless
  `goreleaser check` is already green and you're explicitly enriching. The contract's field set is the
  minimum; extra fields add review surface without value. (Tags/summary can be a later enhancement.)
- ❌ Don't forget the preamble is part of the deliverable. Each native pipe in this file has a `# Preamble`
  comment block explaining the pipe + (where relevant) a DECISION GATE note. The chocolateys: block needs
  the same (citing §21.2/G27 + the Defender-gate rationale + FR-U1/U2/U4) — winget-free.

---

## Confidence Score: 9/10

The verbatim chocolateys: block (every field + value), the plural-key trap, the exact 4 WinGet scrubs,
the live baseline (goreleaser v2.9.0 installed; current file passes check), and the DECISION GATE
procedure are all spelled out and verified. The one residual (not a full 10) is the exact `copyright`
line (the LICENSE file's first 2 lines are "MIT License" with no visible copyright holder/year — the
implementer reads the full LICENSE and adjusts, with an `# ADJUST` note if unclear) and the possibility
that `goreleaser check` rejects one of the contract's fields on this goreleaser version (the DECISION
GATE handles it deterministically: comment out + re-check). The mechanical gate (`rg -ni winget` = 0) is
unambiguous. One-pass success is highly likely.