# P1.M3.T1.S2 Research Findings — docs/cli.md + README.md: winget→chocolatey + delete 'Not yet available' note

> Docs-only (Mode A). The cli.md + README.md half of the distribution-docs sync; S1 (P1.M3.T1.S1) owns
> docs/packaging.md. The v3.3 decision (spec §21.2, b0105e5) replaced WinGet with Chocolatey + added the
> install.ps1 PowerShell installer; this task syncs the two remaining doc files.

---

## §0. SCOPE + THE PRIMARY GATE

- **Files: docs/cli.md + README.md ONLY.** S1 owns docs/packaging.md (explicitly fenced off). install.ps1 /
  .goreleaser.yaml / release.yml are P1.M2 (COMPLETE / in-flight) — read-only, NOT edited here.
- **THE contract gate (point 4): `rg -ni winget docs/cli.md README.md` returns ZERO hits.** After the edits
  there must be NO occurrence of "winget"/"WinGet"/"winget-pkgs" in EITHER file. Only 4 such lines exist today;
  all are addressed by the edits below (verified by grep). The "Not yet available" blockquote (which contains
  "WinGet" + "winget-pkgs") is DELETED entirely, so those tokens vanish.
- **S1 precedent (load-bearing):** S1's contract has the SAME `rg -ni winget` gate for packaging.md, and S1
  learned the rationale paragraph must AVOID the literal "winget" token. THIS task has NO rationale paragraph
  (cli.md/README.md are user-facing install/CLI docs, not a maintainer decision-log) — the edits are pure
  channel-list swaps + a deletion, so the zero-winget gate is straightforward (no reconciliation needed).

## §1. THE 3 STALE REFERENCES (verified by grep — exact line content)

### docs/cli.md:404 — the `### upgrade` blurb channel list
```
...stagecoach detects the install method and delegates to that channel's updater (Homebrew, Scoop, winget,
npm, mise, asdf, Nix, AUR, go install), self-swapping only for the direct-binary channel. This is the v3.0
delegate-first updater.
```
ONLY winget hit in cli.md (grep confirmed). The contract: swap winget→Chocolatey AND add the "choco is printed
(admin)" note consistent with §15.3. The spec §15.3 says: "delegates to that channel's native updater
(Homebrew/Scoop/Chocolatey/npm/mise/asdf/go-install), prints the command where running it needs privileges
(AUR/Chocolatey) or is declarative (Nix), and falls back to a ... direct-binary swap only for the direct-binary
channel." → the cli.md blurb should add the print-channel clause (Chocolatey + AUR = privileges; Nix = declarative).

### README.md:6 — the v3.0 features blurb (line 2 paragraph)
```
...v3.0 adds `stagecoach upgrade` (delegate-first self-update) and expands distribution (Homebrew, Scoop,
Winget, npm, Nix, mise/asdf) — see [Features](#features) below.
```
The contract: change 'Winget' → 'Chocolatey'; optionally add 'PowerShell installer' (install.ps1 is a real v3.0
channel — spec §21.2 "Beyond goreleaser" lists it). Adding it makes the blurb accurate.

### README.md:132-135 — the "Not yet available" blockquote (AFTER the install fenced block, before "Verify:")
```
> **Not yet available:** Windows WinGet and Arch AUR (`stagecoach-bin`) are wired up
> in CI but aren't installable today — WinGet is pending acceptance into the
> `microsoft/winget-pkgs` repository, and the AUR publish is waiting on the AUR
> service. Windows users: use **Scoop** or **npm** in the meantime; Arch users: use
> `go install`, the curl\|sh one-liner, or the npm wrapper.
```
The contract: DELETE the entire blockquote. (WinGet is replaced by Chocolatey — no acceptance gate; AUR is now
a SHIPPED goreleaser-native `aurs:` pipe, spec §21.2 — no acceptance gate either.) Both "not yet available"
claims are stale → the whole blockquote goes.

## §2. AUR STATUS — resolved (goreleaser `aurs:` pipe), DELETE the mention

The contract: "If the note also mentions AUR, check whether AUR is now available (it is wired in .goreleaser.yaml);
if AUR is also resolved, remove that mention too." Verified:
- `.goreleaser.yaml` line 74-75: "AUR (`aurs:`" — goreleaser-native pipe.
- spec §21.2: "AUR `stagecoach-bin` (prebuilt-binary package via goreleaser; `yay -S stagecoach` resolves to it via
  `provides`)." → AUR is a shipped channel with NO acceptance gate.
⇒ the AUR "not yet available" claim is STALE → DELETE the whole blockquote (both the WinGet + AUR parts).
NOTE: the spec §21.3 install-paths code block does NOT list an AUR install command (it lists Homebrew/Go/Direct/
Scoop/Chocolatey/PowerShell/npm/Nix/mise-asdf/Debian/Fedora). So the README install section (which mirrors §21.3)
does NOT add an AUR line either — just delete the blockquote. Coherent with the spec.

## §3. THE README WINDOWS INSTALL SECTION — add Chocolatey + PowerShell (mirror §21.3)

README.md "### Package managers" install block (lines 92-126) currently has a "Windows (Scoop)" block (97-99) but
NO Chocolatey / PowerShell installer. The contract point (c): "If a Windows install section exists elsewhere in
README, ensure it lists 'choco install stagecoach' and the 'irm | iex' PowerShell fallback (mirroring spec §21.3)."
Verified a Windows install block EXISTS (Scoop). ⇒ ADD the Chocolatey + PowerShell blocks AFTER the Scoop block,
matching spec §21.3 VERBATIM:
```
# Windows (Chocolatey)
choco install stagecoach

# Windows (PowerShell installer — no package manager needed)
irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex
```
(Spec §21.3 exact wording/comments. The Chocolatey comment in §21.3 is bare "# Windows (Chocolatey)" — mirror it;
the admin nuance is covered in the cli.md upgrade blurb + packaging.md, not the README install comment.)

## §4. THE 4 EDITS (verbatim — see the PRP Blueprint for exact oldText/newText)

1. **docs/cli.md:404** — channel list: `winget` → `Chocolatey`; add the print-channel clause "(printing the
   command where it needs privileges (Chocolatey, AUR) or is declarative (Nix))" — consistent with §15.3.
2. **README.md:6** — blurb: `Winget` → `Chocolatey`; add `PowerShell installer` to the parenthetical.
3. **README.md install block** — insert the Chocolatey + PowerShell blocks after the Windows (Scoop) block.
4. **README.md:132-135** — DELETE the entire "Not yet available" blockquote (+ collapse the surrounding blank lines).

## §5. SCOPE FENCES + STYLE

- ONLY docs/cli.md + README.md. NOT packaging.md (S1), NOT install.ps1/.goreleaser.yaml/release.yml (P1.M2),
  NOT spec/SPEC.md (source of truth, already updated), NOT any code.
- README install block style: `# Channel (note)` comments + bare commands in ONE fenced ```bash block (the existing
  convention — Homebrew/Scoop/npm/Nix/etc. all live in the same fence). Insert the 2 new Windows blocks inside that
  fence, between Scoop and npm, preserving the blank-line separators.
- cli.md blurb style: one prose sentence (no fence); cross-ref "(PRD §9.29 FR-U1)" already present — preserve it.
- Match the spec's channel-naming case: "Chocolatey" (PascalCase, as in §15.3/§21.3), not "chocolatey".

## §6. VALIDATION (project-specific, verified)
- **THE gate:** `rg -ni winget docs/cli.md README.md` → ZERO hits (the 4 edits remove all 4 winget lines).
- Defense-in-depth: `rg -ni 'winget-pkgs|microsoft/winget' docs/cli.md README.md` → ZERO (the blockquote deletion).
- Structure: `rg -n 'choco install stagecoach' README.md` → ≥1; `rg -n 'irm .*install\.ps1 \| iex' README.md` → ≥1;
  `rg -n 'Chocolatey' docs/cli.md README.md` → ≥1 each.
- Scope: `git status --short` → ONLY `M docs/cli.md` + `M README.md` (2 files).
- (Docs-only — no build/test/lint step. The grep gates ARE the validation.)