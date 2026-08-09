name: "P1.M3.T1.S2 — docs/cli.md + README.md: winget→chocolatey in upgrade channel list + features blurb + delete 'Not yet available' note (Mode A docs)"
description: >
  Mode-A DOCS-ONLY task (the cli.md + README.md half of the distribution-docs sync; S1 P1.M3.T1.S1 owns
  docs/packaging.md). The v3.3 decision (spec §21.2, b0105e5) replaced WinGet (microsoft/winget-pkgs, whose
  validationDefender clean-VM scan hard-blocked the unsigned binary every release) with Chocolatey (goreleaser-native
  `chocolateys:` pipe, direct API-key publish, no gate) + added the install.ps1 PowerShell installer (P1.M2.T1.S3).
  This task syncs the TWO remaining doc files with FOUR edits: (1) docs/cli.md:404 — in the `### upgrade` blurb's
  channel list, swap `winget` → `Chocolatey` AND add the print-channel clause "(printing the command where it needs
  privileges (Chocolatey, AUR) or is declarative (Nix))" so the blurb matches spec §15.3's "prints the command where
  running it needs privileges (AUR/Chocolatey) or is declarative (Nix)" — this is the contract's "note that choco is
  printed (admin)"; (2) README.md:6 — in the v3.0 features blurb, swap `Winget` → `Chocolatey` and add `PowerShell
  installer` to the parenthetical (install.ps1 is a real v3.0 channel); (3) README.md install block — INSERT the
  `# Windows (Chocolatey)` + `choco install stagecoach` and `# Windows (PowerShell installer — no package manager
  needed)` + `irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex` blocks AFTER the existing
  "Windows (Scoop)" block, mirroring spec §21.3 VERBATIM (the contract: "ensure it lists 'choco install stagecoach'
  and the 'irm | iex' PowerShell fallback"); (4) README.md:132-135 — DELETE the entire "Not yet available" blockquote
  (WinGet is replaced by Chocolatey which has NO acceptance gate; AUR is now a SHIPPED goreleaser-native `aurs:` pipe
  per §21.2 — no gate either — so the AUR "not yet available" claim is also stale; the whole blockquote goes). THE
  contract gate (point 4): `rg -ni winget docs/cli.md README.md` returns ZERO hits — the 4 edits remove all 4 winget
  lines (cli.md:404 winget; README:6 Winget; README:132 WinGet; README:133 winget-pkgs). NO rationale paragraph is
  added (these are user-facing install/CLI docs, not a decision-log), so the zero-winget gate is straightforward (no
  S1-style reconciliation needed). Edits ONLY docs/cli.md + README.md — NOT packaging.md (S1), NOT install.ps1 /
  .goreleaser.yaml / release.yml (P1.M2 COMPLETE/in-flight, read-only), NOT spec/SPEC.md (source of truth, b0105e5).
  Source of truth: spec §15.3 (upgrade subcommand), §21.2 (goreleaser: Chocolatey + AUR + PowerShell), §21.3 (install paths).

---

## Goal

**Feature Goal**: Bring `docs/cli.md` and `README.md` in sync with the v3.3 Windows-distribution decision (Chocolatey
replaces WinGet) + the new `install.ps1` PowerShell installer, so both files contain ZERO references to winget/WinGet/
winget-pkgs and accurately list Chocolatey + the PowerShell installer as Windows channels. After this change the docs
match the shipped code (Chocolatey pipe + install.ps1) and the spec (§15.3/§21.2/§21.3).

**Deliverable**: Four edits across TWO files (no new files, no code):
1. **docs/cli.md:404** — `winget` → `Chocolatey` in the upgrade-blurb channel list + add the print-channel clause.
2. **README.md:6** — `Winget` → `Chocolatey` + add `PowerShell installer` in the v3.0 features blurb.
3. **README.md install block** — insert Chocolatey + PowerShell installer blocks after the Windows (Scoop) block.
4. **README.md:132-135** — delete the entire "Not yet available" blockquote.

**Success Definition**:
- `rg -ni winget docs/cli.md README.md` returns ZERO hits (all 4 winget lines removed).
- `rg -ni 'winget-pkgs|microsoft/winget' docs/cli.md README.md` returns ZERO hits.
- docs/cli.md's upgrade blurb lists "Chocolatey" (not winget) and notes the print channels (privileges: Chocolatey,
  AUR; declarative: Nix) — consistent with §15.3.
- README's v3.0 blurb lists "Chocolatey" (not Winget) and "PowerShell installer".
- README's install section lists `choco install stagecoach` and the `irm | iex` PowerShell one-liner (mirroring §21.3).
- README's "Not yet available" blockquote (WinGet + AUR) is GONE.
- Scope: `git status` shows ONLY `docs/cli.md` + `README.md` modified (2 files).

## User Persona (if applicable)

**Target User**: A user reading README.md to install stagecoach on Windows (now sees Chocolatey + PowerShell installer,
not a stale "WinGet pending" note); a user reading docs/cli.md's `upgrade` section (now sees Chocolatey in the channel
list + the print-channel behavior); a maintainer confirming the docs match the v3.3 shipping channels.

**Use Case**: A Windows user without Scoop reads README's Package Managers section → sees `choco install stagecoach`
and the `irm | iex` fallback → installs. A user runs `stagecoach upgrade` after a Chocolatey install → cli.md's blurb
explains it PRINTs `choco upgrade stagecoach -y` (needs admin), not self-swap.

**User Journey**: user opens README → Package Managers → Windows (Chocolatey) / Windows (PowerShell installer) →
installs → `stagecoach --version`. No "Not yet available" caveat blocks them. The docs match the spec §21.3 install paths.

**Pain Points Addressed**: README currently tells Windows users WinGet is "pending acceptance" (stale — WinGet was
DROPPED for Chocolatey) and omits the `choco`/`irm|iex` install commands; cli.md's upgrade blurb lists the dropped
"winget" channel. This task makes both files reflect the shipped Chocolatey + PowerShell reality.

## Why

- **v3.3 decision (spec §21.2, b0105e5)**: Windows package-manager distribution moved from WinGet (dropped:
  microsoft/winget-pkgs validationDefender clean-VM scan hard-blocks the unsigned binary every release) to Chocolatey
  (goreleaser-native `chocolateys:` pipe, direct API-key publish, no gate). cli.md + README still list "winget" —
  this task syncs them.
- **install.ps1 (PRD §21.3, P1.M2.T1.S3)**: the Windows `irm | iex` installer is a shipped v3.0 artifact; README's
  install section omits it. This task adds it (mirroring §21.3).
- **AUR is now shipped (spec §21.2)**: the README "Not yet available" blockquote claims AUR is "waiting on the AUR
  service" — stale; AUR is a goreleaser-native `aurs:` pipe publishing on every release. The whole blockquote is stale.
- **Mode A (rides with the work)**: this is the docs-sync half of the distribution milestone (P1.M2 code/CI/installer
  + P1.M3 docs). The spec is already the source of truth; cli.md + README just need to reflect it.
- **Bounded scope**: two files, four edits, verbatim content provided. No code, no CI, no installer, no spec.

## What

**User-visible behavior**: README's install section lists Chocolatey + the PowerShell installer and has no "Not yet
available" caveat; the features blurb names Chocolatey (not Winget); cli.md's upgrade blurb names Chocolatey and the
print-channel behavior. Both files are winget-free.

**Technical change**: 4 edits (1 in cli.md, 3 in README). See the Implementation Blueprint for verbatim before/after.

### Success Criteria
- [ ] docs/cli.md:404 channel list says "Chocolatey" (was "winget") + the print-channel clause is present.
- [ ] README.md:6 blurb says "Chocolatey" (was "Winget") + "PowerShell installer" is in the parenthetical.
- [ ] README.md install block has `# Windows (Chocolatey)` + `choco install stagecoach` AND
      `# Windows (PowerShell installer — no package manager needed)` + the `irm | iex` one-liner, after the Scoop block.
- [ ] README.md:132-135 "Not yet available" blockquote is DELETED (no WinGet, no AUR caveat).
- [ ] `rg -ni winget docs/cli.md README.md` returns ZERO hits.
- [ ] `rg -ni 'winget-pkgs|microsoft/winget' docs/cli.md README.md` returns ZERO hits.
- [ ] `git status` shows ONLY `docs/cli.md` + `README.md` modified (2 files).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim oldText/newText for all 4 edits (anchored by unique strings, with the exact insertion point
for the README install blocks), the proof that only 4 winget lines exist (grep-confirmed: cli.md:404, README:6/132/133)
and all are addressed, the AUR-resolved fact (goreleaser `aurs:` pipe per §21.2 → delete the whole blockquote, no AUR
install line added since §21.3 doesn't list one), the §15.3 + §21.3 wording to mirror (Chocolatey case, the
print-channel clause, the exact install comments), the zero-winget gate + how it's satisfied (no rationale paragraph
is added, so no S1-style reconciliation), and the scope fences (only cli.md + README.md).

### Documentation & References

```yaml
# MUST READ — the authoritative findings (the 4 edits + the AUR status + the install-section structure)
- docfile: plan/020_6979db625159/P1M3T1S2/research/findings.md
  why: "§0 scope + the zero-winget gate; §1 the 3 stale references (exact line content); §2 AUR STATUS (resolved —
        goreleaser aurs: pipe per §21.2 → delete the whole blockquote, NO AUR install line); §3 the README install
        block structure + the verbatim Chocolatey/PowerShell blocks to insert (mirror §21.3); §4 the 4 edits; §5
        scope fences + style; §6 validation."
  critical: "§2: AUR is RESOLVED (shipped aurs: pipe) — delete the WHOLE blockquote (WinGet + AUR both stale), and do
             NOT add an AUR install line (§21.3 doesn't list one). §1/§4: only 4 winget lines exist — all addressed.
             §0: the zero-winget gate is straightforward (no rationale paragraph added)."

# MUST READ — the S1 sibling (owns packaging.md; sets the zero-winget precedent + the scope split)
- docfile: plan/020_6979db625159/P1M3T1S1/PRP.md
  why: "S1 edits docs/packaging.md ONLY and EXPLICITLY fenced docs/cli.md + README.md as THIS task's scope. S1's
        contract gate is the SAME `rg -ni winget` (== 0 hits) — S1 had to reconcile a rationale paragraph that wanted
        to name 'winget-pkgs' (phrased it without the literal token). THIS task adds NO rationale paragraph (user-facing
        docs), so no reconciliation is needed — the edits are pure channel-list swaps + a deletion."
  critical: "Do NOT edit docs/packaging.md (S1's scope). Do NOT add a 'why not winget' rationale to cli.md/README.md
             (that rationale lives in packaging.md §21.2, S1's job) — keep cli.md/README.md as clean install/CLI docs."

# MUST EDIT — docs/cli.md (the upgrade blurb channel list)
- file: docs/cli.md   # line 404, the `### upgrade` blurb
  why: "The ONLY winget hit in cli.md. The blurb lists the delegate channels; swap winget→Chocolatey + add the
        print-channel clause to match §15.3."
  pattern: "One prose sentence with a parenthetical channel list + '(PRD §9.29 FR-U1)' cross-ref (preserve the cross-ref).
            Spec §15.3 names the print channels: 'prints the command where running it needs privileges (AUR/Chocolatey)
            or is declarative (Nix)'."
  gotcha: "Anchor on the unique string '(Homebrew, Scoop, winget, npm, mise, asdf, Nix, AUR, go install)' — it appears once.
           Preserve 'This is the v3.0 delegate-first updater.' and the §9.29 cross-ref; only swap the list + add the clause."

# MUST EDIT — README.md (3 edits: blurb line 6, install block ~97-99, blockquote 132-135)
- file: README.md
  why: "3 of the 4 winget hits. The features blurb (line 6), the Package Managers install block (Windows (Scoop) at
        97-99, needs Chocolatey + PowerShell additions), and the 'Not yet available' blockquote (132-135, deleted)."
  pattern: "Blurb = one sentence paragraph. Install block = ONE fenced ```bash block with `# Channel (note)` comments
            + bare commands (Homebrew/Scoop/npm/Nix/mise/etc. all in the same fence). Blockquote = `> ` prefixed."
  gotcha: "For the install-block insertion, place the 2 new Windows blocks BETWEEN the Scoop block (ends 'scoop install
           stagecoach/stagecoach') and the npm block (starts '# npm'), preserving the blank-line separators inside the
           fence. For the blockquote deletion, also collapse the surrounding blank lines so there's no double-blank."

# MUST READ — the source of truth (spec, already updated by b0105e5)
- docfile: spec/SPEC.md   # §15.3 (upgrade subcommand), §21.2 (goreleaser: Chocolatey + AUR + PowerShell installer), §21.3 (install paths)
  section: "§15.3 Subcommands (the upgrade bullet), §21.2 goreleaser, §21.3 Install paths"
  why: "The authority the docs must match. §15.3 gives the print-channel wording (privileges: AUR/Chocolatey; declarative:
        Nix). §21.2 confirms Chocolatey (chocolateys: pipe) + AUR (aurs: pipe, shipped) + the PowerShell installer
        (install.ps1, irm|iex). §21.3 gives the EXACT install commands/comments to mirror."
  critical: "Mirror §21.3's install comments VERBATIM: '# Windows (Chocolatey)' + 'choco install stagecoach' and
             '# Windows (PowerShell installer — no package manager needed)' + the irm|iex one-liner. §21.3 does NOT list
             an AUR install command — so do NOT add one to README. Case: 'Chocolatey' (PascalCase)."

# CONTEXT — AUR status (resolved — delete the blockquote mention, do NOT add an AUR install line)
- file: .goreleaser.yaml   # lines 74-75: "AUR (`aurs:`" — goreleaser-native pipe, ships on every release
  why: "Confirms AUR is a SHIPPED channel (no acceptance gate), so the README 'Not yet available: ... AUR' claim is stale
        and the whole blockquote is deleted. §21.2 corroborates ('AUR stagecoach-bin ... via goreleaser')."
  critical: "READ-ONLY. Do NOT edit .goreleaser.yaml (P1.M2.T1.S1 COMPLETE). Do NOT add an AUR install line to README
             (§21.3 doesn't list one)."

# CONTEXT — the upgrade-delegate behavior (P1.M1 COMPLETE; read-only — cli.md's blurb describes it)
- docfile: plan/020_6979db625159/architecture/upgrade_subsystem.md
  why: "Confirms Chocolatey is a PRINT channel (choco upgrade needs admin → stagecoach upgrade PRINTs the command,
        FR-U4; does NOT self-swap, FR-U1). The cli.md blurb's print-channel clause reflects this."
  critical: "READ-ONLY. The detect.go/delegate.go changes are P1.M1 (COMPLETE); cli.md just describes the behavior."
```

### Current Codebase tree (relevant slice)

```bash
docs/cli.md            # EDIT (this task) — line 404 upgrade blurb: winget→Chocolatey + print-channel clause
README.md              # EDIT (this task) — line 6 blurb (Winget→Chocolatey + PowerShell installer); install block
                       #   (+ Chocolatey + PowerShell blocks); lines 132-135 (delete "Not yet available" blockquote)
docs/packaging.md      # READ-ONLY (P1.M3.T1.S1 sibling) — its winget refs are NOT this task
install.ps1            # READ-ONLY (P1.M2.T1.S3) — the installer the README PowerShell block documents (don't edit)
.goreleaser.yaml       # READ-ONLY (P1.M2.T1.S1 COMPLETE) — confirms AUR (aurs:) + Chocolatey (chocolateys:)
spec/SPEC.md           # READ-ONLY — §15.3/§21.2/§21.3 source of truth (b0105e5)
```

### Desired Codebase tree with files to be modified

```bash
docs/cli.md            # MODIFIED — line 404 upgrade blurb (winget→Chocolatey + print-channel clause)
README.md              # MODIFIED — line 6 blurb + install block (Chocolatey + PowerShell) + delete blockquote 132-135
# (NO other file. NO new file.)
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (THE gate: `rg -ni winget docs/cli.md README.md` == 0 hits): only 4 winget lines exist today
     (cli.md:404 'winget'; README:6 'Winget'; README:132 'WinGet'; README:133 'winget-pkgs') — ALL removed by the
     4 edits (1 swap in cli.md; 1 swap + 1 blockquote-delete in README). NO rationale paragraph is added (unlike S1's
     packaging.md), so there is NO reconciliation needed — the gate is satisfied by pure swaps + a deletion. Verify
     with `rg -ni winget docs/cli.md README.md` (must print nothing). -->

<!-- CRITICAL (delete the WHOLE "Not yet available" blockquote, not just the WinGet clause): the blockquote (README:132-135)
     covers BOTH WinGet (stale — replaced by Chocolatey, no acceptance gate) AND AUR (stale — AUR is now a shipped
     goreleaser-native `aurs:` pipe per §21.2, no acceptance gate). Both claims are stale → the entire blockquote goes.
     Collapse the surrounding blank lines so the "Verify:" section follows the install fence without a double-blank. -->

<!-- CRITICAL (do NOT add an AUR install line to README): even though AUR is now shipped, spec §21.3's install-paths
     code block does NOT list an AUR install command (it lists Homebrew/Go/Direct/Scoop/Chocolatey/PowerShell/npm/Nix/
     mise-asdf/Debian/Fedora). README mirrors §21.3, so do NOT add an AUR line — just delete the blockquote. Adding one
     would diverge from the spec. -->

<!-- CRITICAL (mirror §21.3's install comments VERBATIM): the README Windows blocks must be exactly
     '# Windows (Chocolatey)' + 'choco install stagecoach' and '# Windows (PowerShell installer — no package manager
     needed)' + 'irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex'. Do NOT add a '— needs admin'
     note to the Chocolatey comment (the admin nuance lives in the cli.md upgrade blurb + packaging.md; §21.3's comment
     is bare). Case: 'Chocolatey' (PascalCase), not 'chocolatey'. -->

<!-- GOTCHA (the README install block is ONE fenced bash block): the channels (Homebrew/Scoop/npm/Nix/mise/Debian/Fedora/
     Go/Direct) all live in the SAME ```bash fence with `# Channel (note)` comments + blank-line separators. Insert the 2
     new Windows blocks INSIDE that fence, between the Scoop block ('scoop install stagecoach/stagecoach') and the npm
     block ('# npm'), preserving the blank-line separators. Do NOT open a new fence. -->

<!-- GOTCHA (anchor edits by unique strings, not line numbers — lines drift): cli.md:404 anchor =
     '(Homebrew, Scoop, winget, npm, mise, asdf, Nix, AUR, go install)'. README:6 anchor = '(Homebrew, Scoop, Winget, npm,
     Nix, mise/asdf)'. README install anchor = the Scoop block + the blank line + '# npm'. README blockquote anchor =
     '> **Not yet available:** Windows WinGet and Arch AUR'. Confirm with grep before/after. -->

<!-- GOTCHA (scope — ONLY docs/cli.md + README.md): docs/packaging.md is P1.M3.T1.S1 (sibling). install.ps1 is P1.M2.T1.S3.
     .goreleaser.yaml/release.yml are P1.M2.T1.S1/S2 (COMPLETE). spec/SPEC.md is the source of truth (b0105e5). This task
     edits TWO files. `git status` must show only docs/cli.md + README.md. -->

<!-- GOTCHA (do NOT add a "why not winget" rationale to cli.md/README.md): that decision-log rationale lives in
     packaging.md (S1's scope) + spec §21.2/Appendix F. cli.md/README.md are user-facing install/CLI docs — keep them
     clean (channel lists + install commands), no v3.3 defense. This is WHY this task has no zero-winget reconciliation
     (unlike S1): it adds no prose that would want to name 'winget'. -->
```

## Implementation Blueprint

### Data models and structure

None (docs-only).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT docs/cli.md:404 — upgrade blurb: winget→Chocolatey + print-channel clause
  - OLD (the channel-list sentence in the `### upgrade` blurb):
        stagecoach detects the install method and delegates to that channel's updater (Homebrew, Scoop, winget, npm, mise, asdf, Nix, AUR, go install), self-swapping only for the direct-binary channel. This is the v3.0 delegate-first updater.
  - NEW:
        stagecoach detects the install method and delegates to that channel's updater (Homebrew, Scoop, Chocolatey, npm, mise, asdf, Nix, AUR, go install) — printing the command where it needs privileges (Chocolatey, AUR) or is declarative (Nix) — self-swapping only for the direct-binary channel. This is the v3.0 delegate-first updater.
  - ANCHOR: the unique string "(Homebrew, Scoop, winget, npm, mise, asdf, Nix, AUR, go install)" (appears once).
  - PRESERVE: "(PRD §9.29 FR-U1)." and "This is the v3.0 delegate-first updater." — only swap the list + insert the
    em-dash print-channel clause. The clause mirrors §15.3 ("prints the command where running it needs privileges
    (AUR/Chocolatey) or is declarative (Nix)") — this IS the contract's "note that choco is printed (admin)".
  - GOTCHA: case "Chocolatey" (PascalCase, matching §15.3/§21.3).

Task 2: EDIT README.md:6 — features blurb: Winget→Chocolatey + PowerShell installer
  - OLD (the parenthetical in the v3.0 blurb):
        expands distribution (Homebrew, Scoop, Winget, npm, Nix, mise/asdf)
  - NEW:
        expands distribution (Homebrew, Scoop, Chocolatey, PowerShell installer, npm, Nix, mise/asdf)
  - ANCHOR: the unique string "(Homebrew, Scoop, Winget, npm, Nix, mise/asdf)" (appears once, line 6).
  - PRESERVE: the rest of the sentence ("— see [Features](#features) below.").

Task 3: EDIT README.md install block — insert Chocolatey + PowerShell blocks after Windows (Scoop)
  - OLD (the Scoop block + the blank line + the npm comment, inside the ```bash fence):
        # Windows (Scoop)
        scoop bucket add stagecoach https://github.com/dabstractor/stagecoach-bucket
        scoop install stagecoach/stagecoach

        # npm (zero-install trial: npx stagecoach-ai; or global install)
  - NEW (insert the 2 Windows blocks between Scoop and npm, mirroring spec §21.3 VERBATIM):
        # Windows (Scoop)
        scoop bucket add stagecoach https://github.com/dabstractor/stagecoach-bucket
        scoop install stagecoach/stagecoach

        # Windows (Chocolatey)
        choco install stagecoach

        # Windows (PowerShell installer — no package manager needed)
        irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex

        # npm (zero-install trial: npx stagecoach-ai; or global install)
  - ANCHOR: the unique 2-line boundary "scoop install stagecoach/stagecoach\n\n# npm (zero-install trial" — insert the
    2 new blocks in the blank line between them. Stay INSIDE the existing ```bash fence (do NOT open a new fence).
  - GOTCHA: mirror §21.3's comments exactly ("# Windows (Chocolatey)" — bare, no admin note; "# Windows (PowerShell
    installer — no package manager needed)" — with "needed"). The `irm | iex` URL is the EXACT §21.3 string.

Task 4: EDIT README.md:132-135 — DELETE the "Not yet available" blockquote
  - DELETE (the entire 4-line blockquote + collapse surrounding blank lines):
        > **Not yet available:** Windows WinGet and Arch AUR (`stagecoach-bin`) are wired up
        > in CI but aren't installable today — WinGet is pending acceptance into the
        > `microsoft/winget-pkgs` repository, and the AUR publish is waiting on the AUR
        > service. Windows users: use **Scoop** or **npm** in the meantime; Arch users: use
        > `go install`, the curl\|sh one-liner, or the npm wrapper.
  - ANCHOR: the unique block (starts "> **Not yet available:** Windows WinGet"). It sits between the install ```bash
    fence's closing ``` and the "Verify:" ```bash fence. After deletion, the closing ``` of the install fence is
    followed by ONE blank line then "Verify:" (collapse the blockquote's leading+trailing blanks to a single blank).
  - WHY delete the whole thing: WinGet is REPLACED by Chocolatey (no acceptance gate); AUR is now a SHIPPED goreleaser
    `aurs:` pipe (§21.2, no gate). Both "not yet available" claims are stale. (Do NOT add an AUR install line — §21.3
    doesn't list one.)
  - GOTCHA: also remove the blank line(s) immediately around the blockquote so there's no double/triple blank between
    the install fence and "Verify:".

Task 5: VERIFY — winget scrub + content + scope
  - rg -ni winget docs/cli.md README.md                       # MUST be empty (zero hits) — THE contract gate
  - rg -ni 'winget-pkgs|microsoft/winget' docs/cli.md README.md   # MUST be empty (defense-in-depth)
  - rg -n 'Chocolatey' docs/cli.md                            # ≥1 (the upgrade blurb)
  - rg -n 'Chocolatey' README.md                              # ≥2 (the blurb + the install block)
  - rg -n 'choco install stagecoach' README.md                # ≥1
  - rg -n 'irm .*install\.ps1 | iex' README.md                # ≥1
  - rg -n 'Not yet available' README.md                       # MUST be empty (the blockquote is gone)
  - git status --short                                        # ONLY M docs/cli.md + M README.md (2 files)
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: the cli.md channel-list swap + print-channel clause — mirror §15.3's wording exactly:
     "...delegates to that channel's updater (Homebrew, Scoop, Chocolatey, npm, mise, asdf, Nix, AUR, go install) —
      printing the command where it needs privileges (Chocolatey, AUR) or is declarative (Nix) — self-swapping only
      for the direct-binary channel." The print clause IS the "choco is printed (admin)" note (§15.3 parity). -->

<!-- PATTERN: the README install blocks — mirror spec §21.3 VERBATIM (comments + commands):
     # Windows (Chocolatey)
     choco install stagecoach

     # Windows (PowerShell installer — no package manager needed)
     irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex
     Insert inside the existing ```bash fence, between Scoop and npm. -->

<!-- PATTERN: the blockquote deletion — remove the WHOLE block (WinGet + AUR both stale) + collapse surrounding blanks. -->
```

### Integration Points

```yaml
DOCS (docs/cli.md + README.md):
  - cli.md:404 upgrade blurb: winget→Chocolatey + print-channel clause (§15.3 parity).
  - README:6 blurb: Winget→Chocolatey + PowerShell installer.
  - README install block: + Windows (Chocolatey) + Windows (PowerShell installer) blocks (§21.3 mirror).
  - README:132-135: DELETE the "Not yet available" blockquote (WinGet + AUR both stale/resolved).

NO code / CI / installer / spec / other-docs changes.
  - docs/packaging.md — P1.M3.T1.S1 sibling (NOT this task; its winget→Chocolatey + PowerShell sections are S1's).
  - install.ps1 — P1.M2.T1.S3 (the installer the README PowerShell block documents; do NOT edit).
  - .goreleaser.yaml / release.yml — P1.M2.T1.S1/S2 COMPLETE (read-only; confirm Chocolatey/AUR pipes).
  - spec/SPEC.md §15.3/§21.2/§21.3 — source of truth (b0105e5); the docs match it.

SCOPE FENCES: ONLY docs/cli.md + README.md. `git status` == two modified files. ZERO new files.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Markdown is not compiled. Confirm the README install fence is still ONE well-formed block (the 2 new Windows blocks
# sit inside it, between Scoop and npm) and no blockquote fragment remains.
sed -n '92,130p' README.md
# Expected: the ```bash fence now contains Homebrew / Scoop / Chocolatey / PowerShell installer / npm / Nix / mise /
#           Debian / Fedora / Go / Direct; the "Not yet available" blockquote is GONE; "Verify:" follows cleanly.

# The winget scrub — THE primary gate.
rg -ni winget docs/cli.md README.md
# Expected: ZERO output. If any line prints, a winget/WinGet/winget-pkgs fragment remains (cli.md:404 or README:6/132/133).

# Case-insensitive sweep for the dropped concepts (defense-in-depth).
rg -ni 'winget-pkgs|microsoft/winget|Not yet available' docs/cli.md README.md
# Expected: ZERO hits (the blockquote deletion removed winget-pkgs + "Not yet available").

# Scope guard: only docs/cli.md + README.md changed.
git status --short
# Expected: M docs/cli.md  M README.md  (only TWO files). NO packaging.md / install.ps1 / .goreleaser.yaml / release.yml / spec.
```

### Level 2: Content Correctness (Component Validation)

```bash
# cli.md's upgrade blurb now names Chocolatey + the print-channel clause (§15.3 parity).
rg -n 'Chocolatey' docs/cli.md
# Expected: ≥1 hit (the channel list). And:
rg -n 'printing the command where it needs privileges' docs/cli.md
# Expected: ≥1 hit (the print-channel clause).

# README's blurb + install block name Chocolatey + PowerShell installer + the commands.
rg -n 'Chocolatey' README.md
# Expected: ≥2 hits (line 6 blurb + the install block).
rg -n 'PowerShell installer' README.md
# Expected: ≥2 hits (line 6 blurb + the install block comment).
rg -n 'choco install stagecoach' README.md
# Expected: ≥1 hit.
rg -n 'irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex' README.md
# Expected: ≥1 hit (the EXACT §21.3 one-liner).

# The blockquote is gone.
rg -n 'Not yet available' README.md
# Expected: ZERO hits.
```

### Level 3: Integration Testing (System Validation)

```bash
# (No build/test step — docs-only.) Optional render-check: open README.md in a markdown previewer — confirm the
# Package Managers section reads cleanly (Homebrew → Scoop → Chocolatey → PowerShell installer → npm → …) and the
# "Verify:" section follows the install fence with no stranded blockquote. `mdsel README.md` for a quick TOC.
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: ZERO winget hits in BOTH files (THE contract gate).
rg -ni winget docs/cli.md README.md
# Expected: empty.

# Guard 2: ZERO "Not yet available" / winget-pkgs in README.
rg -ni 'Not yet available|winget-pkgs' README.md
# Expected: empty.

# Guard 3: cli.md names Chocolatey + the print-channel clause.
rg -c 'Chocolatey' docs/cli.md
rg -c 'printing the command where it needs privileges' docs/cli.md
# Expected: ≥1 each.

# Guard 4: README install block has choco + the irm|iex one-liner (§21.3 verbatim).
rg -c 'choco install stagecoach' README.md
rg -c 'irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex' README.md
# Expected: ≥1 each.

# Guard 5: scope — only 2 files.
git status --porcelain
# Expected: M docs/cli.md  M README.md (only). Confirm packaging.md / install.ps1 / .goreleaser.yaml / release.yml / spec unchanged.
git diff --name-only
# Expected: docs/cli.md  README.md (only).

# Guard 6: the README install fence is still ONE block (the 2 new Windows blocks are inside it, not a new fence).
sed -n '92,126p' README.md | grep -c '```'
# Expected: 2 (the opening + closing ``` of the single install fence — NOT 4, which would mean a stray new fence).
```

## Final Validation Checklist

### Technical Validation
- [ ] `rg -ni winget docs/cli.md README.md` returns ZERO hits (THE contract gate)
- [ ] `rg -ni 'winget-pkgs|microsoft/winget|Not yet available' docs/cli.md README.md` returns ZERO hits
- [ ] The README install fence is still ONE ```bash block (grep guard 6)

### Feature Validation
- [ ] docs/cli.md:404 lists "Chocolatey" (was "winget") + the print-channel clause "(printing the command where it needs
      privileges (Chocolatey, AUR) or is declarative (Nix))" — §15.3 parity
- [ ] README.md:6 blurb lists "Chocolatey" (was "Winget") + "PowerShell installer"
- [ ] README install block has `# Windows (Chocolatey)` + `choco install stagecoach` and
      `# Windows (PowerShell installer — no package manager needed)` + the `irm | iex` one-liner (§21.3 verbatim)
- [ ] README.md:132-135 "Not yet available" blockquote is DELETED

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `docs/cli.md` + `README.md` modified (2 files)
- [ ] NO edit to `docs/packaging.md` (P1.M3.T1.S1 sibling), `install.ps1` (P1.M2.T1.S3),
      `.goreleaser.yaml` / `release.yml` (P1.M2.T1.S1/S2 COMPLETE), or any code/spec file
- [ ] NO new file; NO AUR install line added to README (§21.3 doesn't list one)
- [ ] Grep guards 1–6 (Level 4) all pass

### Code Quality & Docs
- [ ] Channel name case is "Chocolatey" (PascalCase, matching §15.3/§21.3) in both files
- [ ] The README install comments mirror §21.3 verbatim (bare "# Windows (Chocolatey)"; "no package manager needed")
- [ ] No "why not winget" rationale added to cli.md/README.md (that lives in packaging.md §21.2, S1's scope)

---

## Anti-Patterns to Avoid

- ❌ Don't leave ANY winget token (any case) in either file. The contract gate is `rg -ni winget docs/cli.md README.md
  == 0`. Only 4 winget lines exist (cli.md:404; README:6/132/133) and all are removed by the 4 edits (2 swaps + 1
  blockquote-delete). Re-run the grep after editing to confirm zero hits.
- ❌ Don't delete only the WinGet clause of the blockquote. The blockquote (README:132-135) covers WinGet (stale —
  replaced by Chocolatey) AND AUR (stale — now a shipped goreleaser `aurs:` pipe per §21.2). BOTH claims are stale →
  delete the WHOLE blockquote. Leaving an AUR-only caveat would still be stale (AUR is shipped).
- ❌ Don't add an AUR install line to README. Even though AUR is now shipped, spec §21.3's install-paths code block
  does NOT list an AUR command (Homebrew/Go/Direct/Scoop/Chocolatey/PowerShell/npm/Nix/mise/Debian/Fedora only). README
  mirrors §21.3, so just delete the blockquote — do NOT add an AUR line. Adding one diverges from the spec.
- ❌ Don't add a "— needs admin" note to the README Chocolatey install comment. The admin nuance lives in the cli.md
  upgrade blurb (the print-channel clause) + packaging.md (S1). README's install comment mirrors §21.3 VERBATIM: bare
  "# Windows (Chocolatey)". Adding "— needs admin" diverges from §21.3.
- ❌ Don't open a new ```bash fence for the Windows blocks. The README Package Managers install block is ONE fenced
  block (Homebrew/Scoop/npm/Nix/mise/Debian/Fedora/Go/Direct all inside it). Insert the 2 new Windows blocks INSIDE
  that fence, between Scoop and npm. A new fence would fragment the install section. (Grep guard 6 catches this.)
- ❌ Don't add a "why not winget" rationale to cli.md or README.md. That decision-log rationale (validationDefender
  clean-VM scan) lives in packaging.md (S1's scope) + spec §21.2/Appendix F. cli.md/README.md are user-facing install/CLI
  docs — keep them clean. This is WHY this task has no zero-winget reconciliation (unlike S1): it adds no prose that
  would want to name "winget".
- ❌ Don't edit docs/packaging.md, install.ps1, .goreleaser.yaml, release.yml, or spec/SPEC.md. packaging.md is S1's
  scope (sibling); install.ps1 is P1.M2.T1.S3; .goreleaser.yaml/release.yml are P1.M2.T1.S1/S2 (COMPLETE); spec is the
  source of truth (b0105e5). This task edits ONLY docs/cli.md + README.md. `git status` must show two files.
- ❌ Don't anchor edits by line number blindly (404, 6, 132-135) — lines drift. Anchor by the unique strings (the
  channel-list parenthetical; the blurb parenthetical; the Scoop→npm boundary; the blockquote's "**Not yet available:**"
  opener). The line numbers are a guide; the strings are the contract.
- ❌ Don't change the cli.md upgrade blurb's cross-ref or closing sentence. Preserve "(PRD §9.29 FR-U1)." and "This is
  the v3.0 delegate-first updater." — only swap the channel list (winget→Chocolatey) and insert the em-dash print-channel
  clause. The clause mirrors §15.3 ("privileges (AUR/Chocolatey) ... declarative (Nix)").
- ❌ Don't lowercase "Chocolatey". The spec uses PascalCase "Chocolatey" (§15.3, §21.2, §21.3). Match it in both files
  (not "chocolatey"). The COMMAND is lowercase (`choco install stagecoach`); the CHANNEL NAME is PascalCase.

---

## Confidence Score: 10/10

A docs-only, four-edit task with the verbatim oldText/newText for every edit (anchored by unique strings), the proof
that only 4 winget lines exist (grep-confirmed) and all are addressed, the AUR-resolved fact (goreleaser `aurs:` pipe
per §21.2 → delete the whole blockquote, no AUR install line since §21.3 doesn't list one), the §15.3 + §21.3 wording
to mirror (Chocolatey case, the print-channel clause, the exact install comments + commands), the clear zero-winget
gate (satisfied by pure swaps + a deletion — NO rationale paragraph added, so NO S1-style reconciliation), and the
tight scope (only cli.md + README.md; packaging.md is S1's). No code, no CI, no installer, no spec, no new files.
The README install-fence integrity (grep guard 6) and the blockquote-collapse (no double-blank) are the only two
mechanical details an implementer must handle carefully — both spelled out in the Blueprint + Anti-Patterns.