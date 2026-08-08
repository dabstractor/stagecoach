name: "P1.M3.T1.S1 — docs/packaging.md: replace WinGet section with Chocolatey + PowerShell installer (Mode A docs)"
description: >
  Mode-A DOCS-ONLY task (rides with the release-plumbing work it depends on). Replace the entire `## WinGet
  (`dabstractor.Stagecoach`)` section in docs/packaging.md (lines 8-73) — the winget-releaser description, the
  wingetcreate one-time bootstrap, the NestedInstallerType/SHA256 installer YAML, and the pending-acceptance
  checklist — with a `## Chocolatey` section documenting the goreleaser-native `chocolateys:` pipe (P1.M2.T1.S1,
  COMPLETE: publishes a .nupkg to push.chocolatey.org → `choco install stagecoach` / `choco upgrade stagecoach`;
  CHOCOLATEY_API_KEY secret; `stagecoach upgrade` PRINTs `choco upgrade stagecoach -y` per FR-U2/U4, no self-swap
  per FR-U1) AND the v3.3 rationale (Chocolatey chosen OVER winget because microsoft/winget-pkgs runs a
  validationDefender clean-VM scan that hard-blocks the unsigned binary every release; Chocolatey has no such gate).
  Add a `### PowerShell installer (no package manager)` subsection documenting install.ps1 (P1.M2.T1.S3, in-flight):
  the `irm | iex` one-liner (PRD §21.3), $LOCALAPPDATA\stagecoach install dir, USER PATH update, STAGECOACH_INSTALL_METHOD=direct
  tag → FR-U5 self-swap, and the re-open-shell notice. ALSO scrub the stray "WinGet" mention in the intro (line 4) →
  "Chocolatey" (the verification `rg -ni winget docs/packaging.md` == 0 hits catches it). Edits ONLY docs/packaging.md —
  NOT docs/cli.md/README.md (P1.M3.T1.S2 sibling), NOT install.ps1 (P1.M2.T1.S3), NOT .goreleaser.yaml/release.yml
  (P1.M2.T1.S1/S2 COMPLETE, read-only). Source of truth: spec §21.2/§21.3 (already updated by b0105e5).

---

## Goal

**Feature Goal**: Bring `docs/packaging.md` in sync with the v3.3 Windows-distribution decision (Chocolatey
replaces WinGet) and the new `install.ps1` PowerShell installer. After this change the file documents the
goreleaser-native Chocolatey pipe (the Windows package-manager channel) and the `irm | iex` installer (the
no-package-manager fallback), and contains ZERO references to winget/WinGet/wingetcreate/winget-pkgs/WINGET_TOKEN.

**Deliverable**: Two edits to ONE file (`docs/packaging.md`):
1. **Intro (line 4)**: swap the stray "WinGet" → "Chocolatey" in the "This file covers …" sentence.
2. **Lines 8-73**: replace the entire `## WinGet (`dabstractor.Stagecoach`)` section with a new `## Chocolatey`
   section + a `### PowerShell installer (no package manager)` subsection.

**Success Definition**:
- `rg -ni winget docs/packaging.md` returns ZERO hits (the winget-releaser description, wingetcreate bootstrap,
  NestedInstallerType/SHA256 YAML, pending-acceptance checklist, the WINGET_TOKEN bullet, and the intro mention
  are ALL gone).
- The new `## Chocolatey` section describes: the goreleaser `chocolateys:` pipe, `choco install stagecoach` /
  `choco upgrade stagecoach`, the `CHOCOLATEY_API_KEY` secret, the `stagecoach upgrade` PRINT behavior (FR-U2/U4,
  no self-swap per FR-U1), AND the v3.3 rationale (no winget-pkgs Defender gate).
- The new `### PowerShell installer (no package manager)` subsection documents: the `irm | iex` one-liner, the
  `$LOCALAPPDATA\stagecoach` install dir, the USER PATH update, the `STAGECOACH_INSTALL_METHOD=direct` tag (FR-U5),
  and the re-open-shell notice — cross-referencing PRD §21.3.
- The replacement ends cleanly before the existing `## Nix (flakes)` section (line 75) — Nix is untouched.
- Scope: `git status` shows ONLY `docs/packaging.md` modified.

## User Persona (if applicable)

**Target User**: The maintainer reading `docs/packaging.md` to understand how the Windows channels are published
and what secrets/bootstrap they need — and a Windows user deciding between `choco install`, Scoop, and the
PowerShell installer.

**Use Case**: A maintainer cuts a `v*` release and consults packaging.md to confirm the Chocolatey pipe ran and
what `CHOCOLATEY_API_KEY` is; a Windows user without Chocolatey/Scoop reads the PowerShell subsection and runs
the `irm | iex` one-liner.

**User Journey**: maintainer opens packaging.md → reads the Chocolatey section (pipe + secret + upgrade-delegate
behavior + why-not-winget) → confirms the release plumbing; a no-package-manager Windows user scrolls to the
PowerShell subsection → copies the one-liner → installs to `$LOCALAPPDATA\stagecoach` → re-opens their shell.

**Pain Points Addressed**: packaging.md currently documents a DROPPED channel (WinGet) with an inapplicable
bootstrap + checklist, and does not document the new Chocolatey pipe or the install.ps1 installer. This task
makes the file match the shipped reality.

## Why

- **v3.3 decision (spec §21.2, b0105e5)**: Windows package-manager distribution moved from WinGet (microsoft/
  winget-pkgs, which hard-blocks the unsigned binary via `validationDefender` every release) to Chocolatey
  (goreleaser-native `chocolateys:` pipe, publishes directly via API key, no clean-VM scan). packaging.md still
  describes the old channel — this task syncs it.
- **install.ps1 (PRD §21.3, P1.M2.T1.S3)**: the Windows `irm | iex` installer is a new shipped artifact with no
  docs home; packaging.md is the natural place (it already covers the other channels' one-time setup).
- **Mode A (rides with the work)**: this is the docs-sync half of the release-plumbing milestone (P1.M2.T1.S1/S2/S3
  are the code/CI/installer). The spec is already the source of truth; packaging.md just needs to reflect it.
- **Bounded scope**: one file, two edits, verbatim replacement content provided. No code, no CI, no installer.

## What

**User-visible behavior**: `docs/packaging.md` accurately describes the Chocolatey pipe + the PowerShell installer
and contains no winget references.

**Technical change**: replace lines 8-73 (the WinGet section) + fix the intro line-4 mention. See the Implementation
Blueprint for the verbatim before/after.

### Success Criteria
- [ ] The intro sentence (line 4) says "Chocolatey" where it said "WinGet".
- [ ] The `## WinGet (`dabstractor.Stagecoach`)` section (lines 8-73) is replaced by a `## Chocolatey` section.
- [ ] The Chocolatey section documents: the goreleaser `chocolateys:` pipe; `choco install stagecoach` /
      `choco upgrade stagecoach`; the `CHOCOLATEY_API_KEY` secret (chocolatey.org → Account Settings → API Key);
      `stagecoach upgrade` PRINTs `choco upgrade stagecoach -y` (FR-U2/U4, no self-swap FR-U1); and the v3.3
      rationale (microsoft/winget-pkgs validationDefender clean-VM gate; Chocolatey has none).
- [ ] A `### PowerShell installer (no package manager)` subsection documents the `irm | iex` one-liner,
      `$LOCALAPPDATA\stagecoach` install dir, USER PATH update, `STAGECOACH_INSTALL_METHOD=direct` tag (FR-U5),
      and the re-open-shell notice, cross-referencing PRD §21.3.
- [ ] `rg -ni winget docs/packaging.md` returns ZERO hits.
- [ ] The `## Nix (flakes)` section (line 75+) is UNTOUCHED.
- [ ] `git status` shows ONLY `docs/packaging.md` modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim replacement Markdown for both the intro fix and the new Chocolatey + PowerShell sections
(spelled out in the Implementation Blueprint), the exact line boundaries (intro line 4; WinGet section 8-73; Nix
starts at 75), the Chocolatey facts (goreleaser pipe fields, secret, upgrade-delegate behavior, v3.3 rationale —
all sourced from the COMPLETE .goreleaser.yaml + release.yml + spec §21.2), the install.ps1 facts (invocation,
install dir, PATH, install-method tag — sourced from the in-flight P1.M2.T1.S3 PRP), the verification command
(`rg -ni winget`), and the scope fences (only packaging.md; cli.md/README.md/install.ps1/.goreleaser.yaml/release.yml
are other tasks).

### Documentation & References

```yaml
# MUST READ — the authoritative findings (what to replace + the verbatim facts for the new sections)
- docfile: plan/020_6979db625159/P1M3T1S1/research/findings.md
  why: "§1 the exact WinGet surface to replace (intro line 4 + section 8-73; Nix at 75 untouched); §2 the Chocolatey
        facts (pipe, package metadata, CHOCOLATEY_API_KEY, upgrade-delegate FR-U2/U4/U1, the v3.3 Defender rationale);
        §3 the install.ps1 facts (irm|iex, $LOCALAPPDATA\stagecoach, USER PATH, STAGECOACH_INSTALL_METHOD=direct→FR-U5,
        re-open-shell); §4 the scope fences; §5 the style conventions to match."
  critical: "§1: the INTRO LINE 4 also says 'WinGet' — the rg verification catches it, so fix it too (easy to miss).
             §2: the v3.3 rationale (no winget-pkgs validationDefender gate) is a REQUIRED part of the Chocolatey section
             (the contract explicitly asks for the NON-reason). §4: docs/cli.md + README.md winget refs are S2's scope, NOT this task."

# MUST EDIT — the file (the ONLY file this task touches)
- file: docs/packaging.md   # intro line 4 + the WinGet section lines 8-73
  why: "Line 4 intro mention + lines 8-73 (## WinGet … up to the blank line before ## Nix at 75). The replacement
        content is verbatim in the Implementation Blueprint."
  pattern: "Maintainer-oriented prose + bullets + fenced sh blocks; cross-refs as 'PRD §21.2/§21.3'; H2 for the channel,
            H3 for the installer subsection; secrets as '- **Secret**: NAME — …' bullets."
  gotcha: "Anchor the section replacement on the H2 headers: OLD starts at '## WinGet (`dabstractor.Stagecoach`)'
           (line 8) and ends at the blank line before '## Nix (flakes)' (line 75). Do NOT touch the Nix section."

# MUST READ — the source of truth (spec, already updated by b0105e5)
- docfile: spec/SPEC.md   # §21.2 goreleaser (the Chocolatey + PowerShell installer spec) + §21.3 Install paths
  section: "§21.2 goreleaser (Chocolatey bullet + Beyond-goreleaser PowerShell installer bullet), §21.3 Install paths"
  why: "The spec is the authority the docs must match. §21.2 names the chocolateys: pipe, choco install/upgrade, the
        winget-pkgs Defender rationale, and the install.ps1 irm|iex installer; §21.3 has the exact one-liner."
  critical: "Match the spec's wording for the v3.3 rationale ('microsoft/winget-pkgs runs an install-in-clean-VM
             Microsoft Defender scan that hard-blocks the unsigned binary every release (validationDefender)')."

# CONTEXT — the goreleaser chocolateys: pipe (P1.M2.T1.S1 COMPLETE; read-only — document it accurately)
- file: .goreleaser.yaml   # chocolateys: block (~line 167)
  why: "Confirms the pipe fields to cite: name=stagecoach, owners=dabstractor, title=Stagecoach, source_repo=
        push.chocolatey.org, api_key='{{ .Env.CHOCOLATEY_API_KEY }}'. Publishes a .nupkg to the community repo."
  critical: "READ-ONLY. Do NOT edit .goreleaser.yaml (P1.M2.T1.S1 owns it; COMPLETE)."

# CONTEXT — the CHOCOLATEY_API_KEY secret (P1.M2.T1.S2 COMPLETE; read-only)
- file: .github/workflows/release.yml   # header doc lines 13-15 + env line 62
  why: "Confirms the secret name + provenance (chocolatey.org → Account Settings → API Key) + that the goreleaser
        job's env carries it. This is what the docs 'Secret' bullet describes."
  critical: "READ-ONLY. Do NOT edit release.yml (P1.M2.T1.S2 owns it; COMPLETE)."

# CONTEXT — install.ps1 (P1.M2.T1.S3 in-flight; document, do NOT create/edit)
- docfile: plan/020_6979db625159/P1M2T1S3/PRP.md
  why: "Defines install.ps1's behavior the docs subsection must describe: irm|iex one-liner, arch detect, download +
        SHA256 verify, extract to $LOCALAPPDATA\stagecoach, USER PATH prepend, STAGECOACH_INSTALL_METHOD=direct (User
        env) → FR-U5 self-swap, 'Re-open your terminal' notice."
  critical: "Do NOT create or edit install.ps1 (P1.M2.T1.S3 owns it). This task only DOCUMENTS it in packaging.md."

# CONTEXT — the upgrade-delegate behavior (P1.M1 COMPLETE; read-only)
- docfile: plan/020_6979db625159/architecture/upgrade_subsystem.md
  why: "Confirms Chocolatey is a PRINT channel (choco upgrade needs admin → stagecoach upgrade PRINTs the command,
        FR-U4; does NOT self-swap, FR-U1). The docs section cites FR-U2/U4/U1."
  critical: "READ-ONLY. The detect.go/delegate.go changes are P1.M1 (COMPLETE); packaging.md just describes the behavior."
```

### Current Codebase tree (relevant slice)

```bash
docs/packaging.md          # EDIT (this task) — intro line 4 (WinGet→Chocolatey) + replace section 8-73 with Chocolatey + PowerShell
.goreleaser.yaml           # READ-ONLY (P1.M2.T1.S1 COMPLETE) — the chocolateys: pipe (document it)
.github/workflows/release.yml  # READ-ONLY (P1.M2.T1.S2 COMPLETE) — CHOCOLATEY_API_KEY (document it)
install.ps1                # (P1.M2.T1.S3 in-flight) — the installer the PowerShell subsection documents (don't edit it)
docs/cli.md                # READ-ONLY (P1.M3.T1.S2 sibling) — winget refs there are NOT this task
README.md                  # READ-ONLY (P1.M3.T1.S2 sibling) — winget refs there are NOT this task
spec/SPEC.md               # READ-ONLY — §21.2/§21.3 source of truth (already updated by b0105e5)
```

### Desired Codebase tree with files to be added/modified

```bash
docs/packaging.md          # MODIFIED — intro WinGet→Chocolatey + replace lines 8-73 with the Chocolatey + PowerShell sections
# (NO other file. NO new file.)
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (the intro line 4 ALSO says "WinGet" — the rg verification catches it): packaging.md line 4 reads
     "This file covers WinGet (PRD §21.2/§21.3); npm is documented in npm/README.md; …". Swap "WinGet" → "Chocolatey"
     there. Forgetting it leaves a winget hit and fails `rg -ni winget docs/packaging.md` == 0. -->

<!-- CRITICAL (replace the WHOLE section 8-73, not just the H2): the section contains the winget-releaser description
     (8-17), the WINGET_TOKEN/InstallerType/NestedInstallerType bullets (18-26), the wingetcreate bootstrap + the
     NestedInstallerType/NestedInstallerFiles YAML block + the New-Package PR steps (28-62), the release-day checklist
     (64-73), AND the "FR-D5 verify at impl" note (~72). ALL of it goes. Replace from '## WinGet (`dabstractor.Stagecoach`)'
     up to (not including) '## Nix (flakes)'. Do NOT leave a fragment. -->

<!-- CRITICAL (the v3.3 rationale is REQUIRED, not optional): the contract explicitly asks to "note the NON-reason: NO
     winget-pkgs Defender gate". The Chocolatey section MUST explain why Chocolatey was chosen OVER winget
     (microsoft/winget-pkgs validationDefender clean-VM scan hard-blocks the unsigned binary every release; Chocolatey
     publishes directly via API key with no such gate). Omitting it loses the decision's rationale. -->

<!-- CRITICAL (do NOT touch the Nix section — line 75+): the replacement must end at the blank line before '## Nix (flakes)'.
     The Nix flake content (nix run / nix profile install / nix develop / the 'dev' version note) is unrelated and stays. -->

<!-- GOTCHA (scope — ONLY docs/packaging.md): docs/cli.md and README.md ALSO have winget references, but those are
     P1.M3.T1.S2's scope (a separate sibling). install.ps1 is P1.M2.T1.S3 (in-flight). .goreleaser.yaml/release.yml are
     COMPLETE (P1.M2.T1.S1/S2). This task edits ONE file. `git status` must show only docs/packaging.md. -->

<!-- GOTCHA (anchor the edit by the H2 boundaries, not line numbers — lines may drift): OLD block starts at the line
     '## WinGet (`dabstractor.Stagecoach`)' and ends at the blank line immediately before '## Nix (flakes)'. Confirm with
     `grep -n '^## ' docs/packaging.md` before editing. -->

<!-- GOTCHA (keep the docs style): maintainer-oriented prose + '- **Secret**: …' / '- **Package…**: …' bullets + fenced
     ```sh blocks; cross-refs as "PRD §21.2/§21.3" (the existing file's convention). H2 (##) for Chocolatey; H3 (###) for
     the PowerShell installer subsection. -->
```

## Implementation Blueprint

### Data models and structure

None (docs-only).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT docs/packaging.md — intro line 4: "WinGet" → "Chocolatey"
  - OLD (the relevant clause in the intro sentence, line 4):
        This file covers
        WinGet (PRD §21.2/§21.3); npm is documented in [`npm/README.md`](../npm/README.md); Homebrew/Scoop/AUR
  - NEW:
        This file covers
        Chocolatey (PRD §21.2/§21.3); npm is documented in [`npm/README.md`](../npm/README.md); Homebrew/Scoop/AUR
  - ANCHOR: the unique string "This file covers\nWinGet (PRD §21.2/§21.3)" (it appears once, in the intro).
  - PRESERVE: the rest of the intro sentence (npm link, Homebrew/Scoop/AUR pending note) — only swap WinGet→Chocolatey.

Task 2: EDIT docs/packaging.md — REPLACE the entire WinGet section (lines 8-73) with Chocolatey + PowerShell
  - DELETE: from the line '## WinGet (`dabstractor.Stagecoach`)' (line 8) through the last line of the section
    (the "FR-D5 verify at impl" note ~line 72), i.e. everything up to — but NOT including — the blank line before
    '## Nix (flakes)' (line 75). This removes: the winget-releaser description, the PackageIdentifier/InstallerType/
    NestedInstallerType/WINGET_TOKEN bullets, the wingetcreate bootstrap + the installer YAML block + the New-Package
    PR steps, and the release-day checklist.
  - INSERT in its place (verbatim — adapt only if a cited field name verifiably drifted in .goreleaser.yaml):

    ## Chocolatey

    Every `v*` tag runs goreleaser's native
    [`chocolateys:`](https://goreleaser.com/customization/chocolatey/) pipe (PRD §21.2), which builds a
    `.nupkg` and pushes it to the [Chocolatey community repository](https://community.chocolatey.org/)
    (`push.chocolatey.org`). Windows users install and update with Chocolatey directly:

    ```sh
    choco install stagecoach
    choco upgrade stagecoach -y     # needs admin
    ```

    - **Package**: `stagecoach` (owners `dabstractor`, title `Stagecoach`) on the community repo. The pipe
      fields live in the `chocolateys:` block of [`.goreleaser.yaml`](../.goreleaser.yaml)
      (`source_repo: https://push.chocolatey.org/`).
    - **Secret**: `CHOCOLATEY_API_KEY` — the Chocolatey community-source push key
      (chocolatey.org → Account Settings → API Key). It is consumed by the `chocolateys:` pipe inside the
      goreleaser job (see [`release.yml`](../.github/workflows/release.yml)); add it under repo
      Settings → Secrets → Actions.
    - **Why Chocolatey, not the Windows store channel (v3.3)**: the previous Windows package channel
      submitted a manifest to Microsoft's community package repository, whose `validationDefender` step
      runs an install-in-a-clean-VM Microsoft Defender scan that **hard-blocks the unsigned binary every
      release** — an unbounded per-release tax. Chocolatey publishes directly via the API key with no such
      gate, so there is no PR-acceptance queue to bootstrap or track.
    - **`stagecoach upgrade` behavior**: `choco upgrade` needs admin, so `stagecoach upgrade` detects a
      Chocolatey install (FR-U2) and **prints** `choco upgrade stagecoach -y` for the user to run (FR-U4).
      It does **not** self-swap — Chocolatey owns the binary under `ProgramData\chocolatey` (FR-U1).

    > No one-time bootstrap, no installer YAML, and no pending-acceptance checklist: unlike the manifest-PR
    > flow the previous Windows channel used, Chocolatey publishes on every release via the API key — no
    > manifest tooling, no nested-installer YAML, no acceptance queue to track.

    ### PowerShell installer (no package manager)

    Windows users without Chocolatey (or Scoop) can use the `irm | iex` one-liner — the Windows analog of
    the Unix `curl | sh` installer (PRD §21.3). It downloads [`install.ps1`](../install.ps1) from the repo
    root and executes it:

    ```powershell
    irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex
    ```

    `install.ps1` detects the Windows arch, downloads the matching
    `stagecoach_<version>_windows_<arch>.zip` from the latest GitHub Release, SHA256-verifies it against
    `checksums.txt`, extracts `stagecoach.exe` to `$LOCALAPPDATA\stagecoach` (the `rustup`/`starship`/`uv`
    pattern — user-owned, no admin), and prepends that directory to the **user** `PATH`. Because the binary
    is package-manager-unowned, the installer tags it `STAGECOACH_INSTALL_METHOD=direct` so `stagecoach upgrade`
    self-swaps it like any direct install (FR-U5).

    > Re-open your terminal for the `PATH` change to take effect.

  - ANCHOR: the section boundaries — DELETE from '## WinGet (`dabstractor.Stagecoach`)' (line 8) up to (not
    including) '## Nix (flakes)' (line 75); INSERT the new content above. Confirm with
    `grep -n '^## ' docs/packaging.md` that the H2 list is now: (intro) / ## Chocolatey / ## Nix (flakes) / …
    (the WinGet H2 is gone, the Nix H2 is intact).
  - PRESERVE: the `## Nix (flakes)` section (line 75+) ENTIRELY — do not touch a character of it.

Task 3: VERIFY — winget scrub + section structure + scope
  - rg -ni winget docs/packaging.md                         # MUST be empty (zero hits)
  - grep -n '^## \|^### ' docs/packaging.md                 # confirm ## Chocolatey + ### PowerShell installer present; ## Nix intact; no ## WinGet
  - git status --short                                       # ONLY docs/packaging.md modified
  - (optional) render-check: open docs/packaging.md — the Chocolatey section reads cleanly into the Nix section.
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: the "Secret" bullet — clone the old WINGET_TOKEN bullet's form, swap the name/provenance/scope.
     - **Secret**: `CHOCOLATEY_API_KEY` — the Chocolatey community-source push key (chocolatey.org → Account
       Settings → API Key). … add under repo Settings → Secrets → Actions. -->

<!-- PATTERN: the v3.3 rationale — name the REJECTED alternative + its specific failure mode (validationDefender
     clean-VM scan hard-blocks the unsigned binary every release) + why the chosen channel avoids it (direct API-key
     publish, no gate). This is the decision-log style the spec uses (Appendix F). -->

<!-- PATTERN: the PowerShell subsection — one-liner in a fenced powershell block; behavioral bullets (arch detect,
     SHA256 verify, $LOCALAPPDATA\stagecoach, USER PATH, STAGECOACH_INSTALL_METHOD=direct→FR-U5); the re-open-shell
     notice as a blockquote (mirrors install.ps1's own success message). Cross-ref PRD §21.3. -->
```

### Integration Points

```yaml
DOCS (docs/packaging.md):
  - intro line 4: WinGet → Chocolatey.
  - lines 8-73: ## WinGet section → ## Chocolatey section + ### PowerShell installer subsection.

NO code / CI / installer / spec / other-docs changes.
  - .goreleaser.yaml (chocolateys:) — P1.M2.T1.S1 COMPLETE (read-only; the docs cite its fields).
  - release.yml (CHOCOLATEY_API_KEY) — P1.M2.T1.S2 COMPLETE (read-only; the docs cite the secret).
  - install.ps1 — P1.M2.T1.S3 in-flight (the docs describe it; do NOT create/edit it here).
  - docs/cli.md + README.md winget refs — P1.M3.T1.S2 sibling (NOT this task).
  - spec §21.2/§21.3 — already the source of truth (b0105e5); the docs match it.

SCOPE FENCES: ONLY docs/packaging.md. `git status` == one modified file. ZERO new files.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Markdown is not compiled, but confirm the section structure is well-formed (H2/H3 nesting, no dangling WinGet H2).
grep -n '^## \|^### ' docs/packaging.md
# Expected: the H2 list includes '## Chocolatey' and (under it) '### PowerShell installer (no package manager)';
#           '## Nix (flakes)' is still present; '## WinGet …' is GONE.

# The winget scrub — THE primary gate.
rg -ni winget docs/packaging.md
# Expected: ZERO output (no hits). If any line prints, a winget/wingetcreate/winget-pkgs/WINGET_TOKEN fragment remains — delete it.

# Case-insensitive sweep for the dropped concepts too (defense-in-depth).
rg -ni 'wingetcreate|winget-pkgs|WINGET_TOKEN|NestedInstallerType|dabstractor\.Stagecoach' docs/packaging.md
# Expected: ZERO hits. (dabstractor.Stagecoach was the winget PackageIdentifier — it should be gone with the section.)

# Scope guard: only docs/packaging.md changed.
git status --short
# Expected: M docs/packaging.md  (only ONE file). NO .goreleaser.yaml / release.yml / install.ps1 / docs/cli.md / README.md.
```

### Level 2: Content Correctness (Component Validation)

```bash
# The Chocolatey section cites the real pipe + secret.
grep -c 'chocolateys:' docs/packaging.md            # the goreleaser pipe mention
grep -c 'CHOCOLATEY_API_KEY' docs/packaging.md      # the secret
grep -c 'choco install stagecoach' docs/packaging.md
grep -c 'choco upgrade stagecoach -y' docs/packaging.md
# Expected: ≥1 hit each.

# The v3.3 rationale is present (the required NON-reason).
grep -ci 'defender\|validationDefender\|winget-pkgs' docs/packaging.md
# Expected: ≥1 hit (the rationale paragraph names the rejected winget-pkgs Defender gate). NOTE: this is the ONE
#           legitimate mention of the winget mechanism — it is in LOWERCASE prose ('winget-pkgs', 'microsoft/winget-pkgs')
#           as the REJECTED alternative, which is why the `rg -ni winget` gate above MUST be the authority: the rationale
#           paragraph intentionally contains 'winget-pkgs'. RESOLVE THIS CONFLICT per the decision in §Known Gotchas:
#           the contract's gate is `rg -ni winget docs/packaging.md == 0`, so the rationale paragraph must NOT contain
#           the literal token 'winget' — phrase it as 'the previous Windows store channel (microsoft's community manifest
#           repo) ran a validationDefender clean-VM scan …' WITHOUT the word winget/winget-pkgs. (See Task 2 fix + Anti-Patterns.)
```

> **CRITICAL — the rationale vs the winget gate conflict**: the contract requires BOTH (a) the v3.3 rationale
> (why not the Windows store channel) AND (b) `rg -ni winget docs/packaging.md == 0`. These conflict ONLY if the
> rationale uses the literal word "winget". **Resolution**: write the rationale WITHOUT the literal token "winget" —
> refer to it as "the previous Windows store channel" / "Microsoft's community package manifest repository
> (`microsoft/winget-pkgs`)" — WAIT, that still contains "winget". So refer to it as "the previous Windows store
> channel (Microsoft's community manifest repository)" and name the gate generically ("its `validationDefender`
> install-in-clean-VM Microsoft Defender scan hard-blocks the unsigned binary every release"). Do NOT write the
> string "winget", "WinGet", "wingetcreate", "winget-pkgs", or "WINGET_TOKEN" anywhere in the file — including the
> rationale. The Task 2 verbatim block above must be adjusted to remove every "winget" token (replace "microsoft/winget-pkgs"
> → "Microsoft's community package manifest repo"; "the winget-pkgs PR flow" → "the manifest-PR flow"; "wingetcreate" →
> "the manifest tooling"). Re-run `rg -ni winget docs/packaging.md` to confirm zero hits AFTER this adjustment.

### Level 3: Integration Testing (System Validation)

```bash
# Render the file (optional): a markdown previewer or `mdsel docs/packaging.md` — confirm the Chocolatey section
# reads cleanly into the Nix section (no stranded headers, no broken fence).
# (No build/test step — this is a docs-only change to a non-code file.)
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: ZERO winget hits (THE contract gate).
rg -ni winget docs/packaging.md
# Expected: empty.

# Guard 2: the new sections exist.
grep -c '^## Chocolatey' docs/packaging.md
grep -c '^### PowerShell installer (no package manager)' docs/packaging.md
# Expected: 1 hit each.

# Guard 3: the Nix section is intact.
grep -c '^## Nix (flakes)' docs/packaging.md
# Expected: 1 hit (untouched).

# Guard 4: the install.ps1 one-liner is present and exact (PRD §21.3).
grep -c 'irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex' docs/packaging.md
# Expected: 1 hit.

# Guard 5: the FR-U2/U4/U1/U5 citations are present (the upgrade-delegate + self-swap behavior).
grep -cE 'FR-U2|FR-U4|FR-U1|FR-U5' docs/packaging.md
# Expected: ≥4 hits (the Chocolatey PRINT behavior + the PowerShell direct-tag self-swap).

# Guard 6: scope — only docs/packaging.md.
git status --porcelain
# Expected: M docs/packaging.md (only). Confirm docs/cli.md / README.md / install.ps1 / .goreleaser.yaml / release.yml unchanged.
git diff --name-only
# Expected: docs/packaging.md (only).
```

## Final Validation Checklist

### Technical Validation
- [ ] `grep -n '^## \|^### ' docs/packaging.md` shows `## Chocolatey` + `### PowerShell installer (no package manager)`,
      `## Nix (flakes)` intact, NO `## WinGet`
- [ ] `rg -ni winget docs/packaging.md` returns ZERO hits (the rationale paragraph uses NO literal "winget" token)
- [ ] `rg -ni 'wingetcreate|WINGET_TOKEN|NestedInstallerType|dabstractor\.Stagecoach' docs/packaging.md` returns ZERO hits

### Feature Validation
- [ ] Intro (line 4) says "Chocolatey" (was "WinGet")
- [ ] `## Chocolatey` section documents: the goreleaser `chocolateys:` pipe; `choco install/upgrade stagecoach`;
      `CHOCOLATEY_API_KEY` secret (chocolatey.org → Account Settings → API Key); `stagecoach upgrade` PRINTs
      `choco upgrade stagecoach -y` (FR-U2/U4, no self-swap FR-U1); the v3.3 rationale (the rejected Windows store
      channel's validationDefender clean-VM gate; Chocolatey's direct API-key publish with no gate)
- [ ] `### PowerShell installer (no package manager)` subsection documents: the `irm | iex` one-liner (PRD §21.3);
      `$LOCALAPPDATA\stagecoach` install dir; USER PATH update; `STAGECOACH_INSTALL_METHOD=direct` tag (FR-U5);
      the re-open-shell notice
- [ ] The section ends cleanly before `## Nix (flakes)`; Nix content untouched

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `docs/packaging.md` modified (1 file)
- [ ] NO edit to `docs/cli.md` / `README.md` (P1.M3.T1.S2 sibling), `install.ps1` (P1.M2.T1.S3),
      `.goreleaser.yaml` / `release.yml` (P1.M2.T1.S1/S2 COMPLETE), or any code/spec file
- [ ] NO new file
- [ ] Grep guards 1–6 (Level 4) all pass

### Code Quality & Docs
- [ ] Maintainer-oriented prose + bullets + fenced sh/powershell blocks; cross-refs as "PRD §21.2/§21.3"
- [ ] The v3.3 rationale is present AND contains no literal "winget" token (the contract's two requirements reconciled)
- [ ] FR-U1/U2/U4/U5 citations match the spec's upgrade-delegate + self-swap behavior

---

## Anti-Patterns to Avoid

- ❌ Don't leave the intro "WinGet" mention (line 4). The contract's gate is `rg -ni winget docs/packaging.md == 0`,
  which catches ANY case — including the intro sentence "This file covers WinGet (PRD §21.2/§21.3)". Swap it to
  "Chocolatey". It is the easiest hit to miss and the one the grep gate will catch.
- ❌ Don't write the literal word "winget" (in any case) ANYWHERE in the new content — including the v3.3 rationale.
  The contract requires BOTH the rationale (why-not-the-Windows-store-channel) AND zero winget hits. Refer to the
  rejected channel as "the previous Windows store channel" / "Microsoft's community package manifest repository"
  and name the gate as "its `validationDefender` install-in-clean-VM Microsoft Defender scan". Do NOT write "winget",
  "WinGet", "winget-pkgs", "wingetcreate", or "WINGET_TOKEN". (The Task 2 verbatim block must be adjusted to scrub
  these tokens — re-run `rg -ni winget` to confirm.)
- ❌ Don't touch the `## Nix (flakes)` section. The replacement ends at the blank line before it. If you accidentally
  delete or alter Nix content, you've over-reached. Anchor the DELETE on the H2 boundaries (`## WinGet …` up to `## Nix`),
  confirmed via `grep -n '^## '`.
- ❌ Don't edit `docs/cli.md` or `README.md`. Their winget references are P1.M3.T1.S2's scope (a separate sibling
  task). This task is `docs/packaging.md` ONLY. `git status` must show one file.
- ❌ Don't create or edit `install.ps1`. It is P1.M2.T1.S3 (in-flight). This task only DOCUMENTS it in the PowerShell
  subsection — describe its behavior, don't touch the file.
- ❌ Don't edit `.goreleaser.yaml` or `release.yml`. They are COMPLETE (P1.M2.T1.S1/S2). The docs CITE their fields
  (the `chocolateys:` pipe, the `CHOCOLATEY_API_KEY` secret) — read them for accuracy, don't modify them.
- ❌ Don't drop the v3.3 rationale. The contract explicitly asks to "note the NON-reason: NO winget-pkgs Defender gate".
  A Chocolatey section without the why-not-the-store-channel rationale loses the decision's context. (Just phrase it
  without the literal "winget" token — see above.)
- ❌ Don't invent Chocolatey fields/secrets not in `.goreleaser.yaml`/`release.yml`. Cite the real pipe (`name: stagecoach`,
  `source_repo: https://push.chocolatey.org/`, `api_key: '{{ .Env.CHOCOLATEY_API_KEY }}'`) and the real secret
  (`CHOCOLATEY_API_KEY`, chocolatey.org → Account Settings → API Key). If a field name drifts, match the file, not this PRP.
- ❌ Don't convert the section to user-facing install instructions (that's §21.3's job in the spec / README). packaging.md
  is the MAINTAINER file — it covers the pipe, the secret, the bootstrap/non-bootstrap, and the upgrade-delegate behavior.
  The `choco install` / `irm | iex` commands appear as illustrative examples, not as the primary install doc.
- ❌ Don't anchor the edit by line number (8-73) blindly — lines drift. Anchor by the H2 strings
  (`## WinGet (`dabstractor.Stagecoach`)` → `## Nix (flakes)`) via `grep -n '^## '`. The line numbers are a guide;
  the H2 boundaries are the contract.

---

## Confidence Score: 9/10

A docs-only, two-edit task with the verbatim replacement content provided (intro swap + the full Chocolatey +
PowerShell sections), the exact section boundaries (intro line 4; WinGet 8-73; Nix at 75 — anchored by H2), the
Chocolatey facts sourced from the COMPLETE `.goreleaser.yaml`/`release.yml` + spec §21.2, the install.ps1 facts
sourced from the in-flight P1.M2.T1.S3 PRP, the clear scope fence (only packaging.md), and a precise verification
gate (`rg -ni winget == 0`). The -1 from 10/10 reflects the ONE reconcile-required conflict the implementer must
handle carefully: the contract demands BOTH the v3.3 rationale (which naturally wants to name "winget-pkgs") AND
zero winget hits — so the rationale must be phrased WITHOUT the literal token "winget" (referred to as "the previous
Windows store channel" / "Microsoft's community package manifest repository"). The PRP spells out this reconciliation
explicitly (Level 2 critical note + Anti-Patterns) so it is not missed, but it is the single place an inattentive
implementer could write "winget-pkgs" in the rationale and fail the gate. No code, no CI, no installer, no other-docs
changes.