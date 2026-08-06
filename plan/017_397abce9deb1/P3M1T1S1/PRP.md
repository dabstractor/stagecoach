name: "P3.M1.T1.S1 — README install rewrite + upgrade CLI/FAQ + §19 amendment + FUTURE_SPEC reconcile + goreleaser notes"
description: |

  Mode B changeset-level documentation task (the LAST task in plan 017 / v3.0). Sync the PUBLIC docs
  to match what v3.0 ships: (a) replace the README's "Coming soon" install block with the LIVE install
  commands for all 8 channels (Homebrew/Scoop/Winget/npm/Nix/mise·asdf/go-install/curl|sh) per PRD
  §21.3, demote "build from source" to one option among many, and add a new "Updating" subsection
  documenting `stagecoach upgrade` (delegate-first; `--check`→exit 6 for CI/cron; `--rollback`) that
  LINKS to docs/cli.md#upgrade (does NOT re-derive the flag table — that's P1.M4.T1.S1's Mode A doc);
  (b) add/reconcile the README FAQ security cluster — a NEW "Does stagecoach make network calls?"
  entry, scope the existing "Does it send my code anywhere new?" answer to the commit path + name
  `upgrade` as the named exception, and remove the now-wrong "self-update" mention from the
  "What about PR generation…" deferred/rejected list; (c) amend the README "what's new" line (v2.1 →
  append v3.0); (d) FUTURE_SPEC.md: mark the "Self-update command" rejection row SUPERSEDED by PRD
  §9.29 (v3.0) with the delegate-first rationale, mirroring Appendix F's superseded entry; (e)
  docs/how-it-works.md: append the §19 scoping sentence to the "Safety invariant" section (the
  technical security narrative; the README FAQ is the user one — do both); (f) .goreleaser.yaml: add
  YAML `#` comments under `release:`/`brews:`/`scoops:`/`aurs:` pointing at the "Beyond goreleaser"
  channels (npm/winget/nix/mise-asdf) so a future reader sees the full distribution picture in one
  place. **OUTPUT: modified README.md, FUTURE_SPEC.md, docs/how-it-works.md, .goreleaser.yaml
  (comments only). NO code changes** — no .go/.sh/.yml-fields/flake/npm/providers/release.yml/ci.yml.
  This task does NOT duplicate docs/cli.md `upgrade` (P1.M4.T1.S1) or docs/configuration.md
  `[upgrade]` (P1.M1.T2.S1); it summarizes + links them.

---

## Goal

**Feature Goal**: Make the v3.0 public documentation (README + FUTURE_SPEC + how-it-works security
narrative + .goreleaser comments) internally consistent and accurate: every install channel is live
and documented, `stagecoach upgrade` is discoverable from the README, the §19 "no network calls"
claim is correctly scoped to the commit path with `upgrade` named as the single exception, the
self-update rejection is reconciled to SUPERSEDED everywhere it appears, and a future reader of
.goreleaser.yaml sees the whole distribution surface (goreleaser-native + beyond-goreleaser) in one
file.

**Deliverable** (4 MODIFIED files, 0 created, 0 code changes):
- `README.md` — MODIFIED: rewrite the `## Install` block (8 live channels + demoted "build from
  source" + a new `### Updating` subsection); amend the v2.1 "what's new" line to v3.0; add a new
  `### Does stagecoach make network calls?` FAQ entry + scope the existing `### Does it send my
  code anywhere new?` answer to the commit path; remove "self-update" from the
  `### What about PR generation…` deferred/rejected list.
- `FUTURE_SPEC.md` — MODIFIED: the "Self-update command" rejection row (line 101) → **SUPERSEDED**
  note citing PRD §9.29 (FR-U1–U12) + Appendix F with the delegate-first rationale.
- `docs/how-it-works.md` — MODIFIED: append the §19 scoping sentence to the "Safety invariant"
  section (commit path = no network calls; `upgrade` = the named exception).
- `.goreleaser.yaml` — MODIFIED: additive `#` comments under `release:`, `brews:`, `scoops:`,
  `aurs:` pointing at the "Beyond goreleaser" channels (npm/winget/nix/mise-asdf) implemented as
  release.yml jobs / flake.nix / plugins/asdf-stagecoach/.

**Success Definition**:
- A user reading the README can install via any of the 8 channels and update via `stagecoach
  upgrade`, and the security narrative never overclaims "no network calls" without naming the
  upgrade exception.
- `FUTURE_SPEC.md` no longer lists self-update as rejected without the SUPERSEDED qualifier.
- `docs/how-it-works.md` and the README FAQ carry the SAME §19 framing (commit path = none; upgrade
  = named exception, release artifacts only).
- `.goreleaser.yaml` still parses (`goreleaser check` clean) and a reader sees all distribution
  channels named in one file.
- `git status --porcelain` shows ONLY the 4 files modified; no code/script/release file touched.
- `self-update` is nowhere in README/FUTURE_SPEC still called "deferred"/"rejected"/"coming soon".

## User Persona (if applicable)

**Target User**: A new visitor to the GitHub repo (the README is the marketing surface, PRD §21.5)
who wants to install stagecoach and keep it current — on macOS (Homebrew), Windows (Scoop/Winget),
cross-platform JS (npm), Nix, a version manager (mise/asdf), Go, or a curl|sh bootstrap.
**Use Case**: skim README → pick an install channel → `stagecoach --version` → later
`stagecoach upgrade` (or `--check` in CI). **User Journey**: Hero → "Why not opencommit/aicommits?"
→ Install (channel) → Updating (`upgrade`) → Quick start. **Pain Points Addressed**: (1) the current
README says channels are "coming soon" when they're shipping in v3.0; (2) no single "how do I
update?" command surfaced; (3) the security FAQ overclaims "no network calls" now that `upgrade`
exists.

## Why

- **Doc/code drift at a release boundary**: plan 017 built the self-update core (P1, Complete) and
  the distribution surface (P2). The public docs (README "Coming soon", FUTURE_SPEC rejection, the
  §19 absolute "no network calls", the goreleaser file showing only brews/scoops/aurs) all reflect
  the PRE-v3.0 world. This task closes that drift in one consistent pass.
- **PRD §19 is amended (not relaxed)**: v3_scope_boundary.md + the merged PRD §19 scope the "no
  network calls" claim to the commit path (§9.1–§9.28, still zero network calls — verified) with
  `stagecoach upgrade` (§9.29) as the explicit, named exception. The security narrative must say so
  or it is now FALSE.
- **Self-update graduated, not rejected**: FUTURE_SPEC + the README "What about…" list + the
  version one-liner must all agree self-update SHIPPED (§9.29), mirroring Appendix F's superseded
  entry. The delegate-first architecture (never overwrites a manager-owned file) is the reason the
  original rejection no longer holds.
- **Discoverability of the full distribution surface**: .goreleaser.yaml shows only the
  goreleaser-native channels (brews/scoops/aurs). The beyond-goreleaser four (npm/winget/nix/mise-
  asdf) live in release.yml + flake.nix + plugins/. Comments in the goreleaser file make the whole
  picture visible in one place (the task's stated rationale).

## What

### Success Criteria
- [ ] `README.md` `## Install` lists all 8 channels (Homebrew/Scoop/Winget/npm/Nix/mise·asdf/go-
      install/curl|sh) with the EXACT install commands from PRD §21.3 (findings §4); "Build from
      source" is ONE option (no "(works today)"/"only working method" framing); the stale
      "> [!NOTE] … coming with the first public release" is gone or rewritten.
- [ ] `README.md` has a new `### Updating` subsection (in Install, before `## Quick start`) covering
      `stagecoach upgrade` (delegate-first, one command across channels), `--check` (exit 6 for
      CI/cron), `--rollback`; LINKS `docs/cli.md#upgrade` for the full flag table; notes it's walled
      off from the commit core (no repo/lock/provider).
- [ ] `README.md` FAQ: NEW `### Does stagecoach make network calls?` entry citing the §19 scoping;
      the EXISTING `### Does it send my code anywhere new?` answer scoped to the commit path + names
      `upgrade` as the exception; "self-update" removed from the `### What about PR generation…`
      deferred/rejected list.
- [ ] `README.md` line-6 "what's new" appends a v3.0 clause (self-update `stagecoach upgrade` +
      expanded distribution).
- [ ] `FUTURE_SPEC.md` line 101 "Self-update command" row is a **SUPERSEDED** note citing
      PRD §9.29 (FR-U1–U12) + Appendix F with the delegate-first rationale (keep the row + annotate,
      mirroring the in-file chunking-row pattern + Appendix F).
- [ ] `docs/how-it-works.md` "Safety invariant" section carries the §19 scoping sentence (commit
      path = no network calls; `stagecoach upgrade` = the named exception, release artifacts only).
- [ ] `.goreleaser.yaml` has additive `#` comments under `release:`, `brews:`, `scoops:`, `aurs:`
      pointing at the beyond-goreleaser channels (npm = release.yml `npm-publish`; winget =
      release.yml `winget`; nix = `flake.nix`; mise/asdf = `plugins/asdf-stagecoach/`). Comments
      ONLY — no field/value change.
- [ ] `git status --porcelain` shows ONLY: `M README.md`, `M FUTURE_SPEC.md`,
      `M docs/how-it-works.md`, `M .goreleaser.yaml`.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — the EXACT current text of every section to edit (with line numbers), the
verbatim §19 scoping language to reproduce, the verbatim PRD §21.3 install commands, the verbatim
Appendix F superseded text to mirror, the in-file FUTURE_SPEC precedent for a graduated row, the
docs/cli.md + docs/configuration.md anchors to LINK (not duplicate), the goreleaser comment-anchor
line numbers, and the markdownlint rules in force are all below. The one open issue (install.sh does
not exist) is explicitly resolved with a defensible instruction (document per §21.3; flag as residual
risk; do NOT create it).

### Documentation & References

```yaml
# MUST READ — the verified research for THIS item (exact text to edit + the §19/§21.3/Appendix F sources).
- docfile: plan/017_397abce9deb1/P3M1T1S1/research/findings.md
  why: "§1 the §19 scoping language (verbatim from v3_scope_boundary.md + merged PRD §19 + §9.29);
        §2 the EXACT README sections/line-numbers to edit (Install block, line-6 note, the two FAQ
        entries, the 'What about…' list); §3 the FUTURE_SPEC row + the Appendix F text to mirror +
        the in-file chunking-row precedent for a graduated entry; §4 the PRD §21.3 install commands
        for all 8 channels; §5 the install.sh gap + resolution; §6 the how-it-works 'Safety
        invariant' anchor; §7 what docs/cli.md + docs/configuration.md ALREADY cover (LINK, don't
        duplicate); §8 the goreleaser comment-anchor line numbers; §9 the release.yml/sibling
        pointers; §10 consistency checks; §11 the validation approach."
  critical: "§5 (install.sh does NOT exist) is the one landmine — read it before writing the curl|sh
             README line."

# MUST READ — the §19 scoping amendment (the authoritative narrative to reproduce).
- docfile: plan/017_397abce9deb1/architecture/v3_scope_boundary.md
  section: "§19 'no network calls' — now scoped (amendment, not a relaxation)"
  critical: "the EXACT user-facing phrasing: 'no network calls on the commit path; stagecoach upgrade
             is the one exception and it touches only release artifacts.' commit path = §9.1–§9.28,
             unchanged (net/http used nowhere). upgrade fetches ONLY the project's own GitHub release
             artifacts + checksums."

# MUST READ — the PRD sources (the merged PRD in the working tree is current; this task is doc-only).
- file: PRD.md
  why: "§9.29 intro (2nd para) = the upgrade-is-the-named-exception wording; §19 'Diff content is
        local' bullet = already-amended scoping (reproduce its phrasing in README FAQ); §21.3 (lines
        2203–2228) = the canonical install commands for all 8 channels (the README source of truth);
        §21.5 = the README section ORDER (Install is #4, Quick start #5 — Updating goes IN Install);
        Appendix F line 2456 = the 'Self-update SUPERSEDED (v3.0; was rejected v2.1)' entry to mirror
        in FUTURE_SPEC."
  critical: "§21.3 commands are VERBATIM the README install lines. Appendix F is VERBATIM the
             FUTURE_SPEC superseded rationale. Do not paraphrase either."

# MUST READ (to NOT duplicate) — the Mode A per-command/per-table docs the README LINKS to.
- file: docs/cli.md
  section: "### upgrade (lines 400–432)"
  why: "the AUTHORITATIVE upgrade flag table + exit codes 0/1/6 + the Network bullet. The README
        'Updating' subsection is a SUMMARY + a link to docs/cli.md#upgrade — it MUST NOT re-list the
        flag table (that is P1.M4.T1.S1's deliverable). The Network bullet there already states
        'upgrade is the one named exception to the no-network-calls commit path' — the README FAQ
        can echo it but docs/cli.md is the source."
- file: docs/configuration.md
  section: "[upgrade] table (lines 121–163)"
  why: "the [upgrade].channel / [upgrade].source_repo config (global-only; P1.M1.T2.S1's deliverable).
        The README 'Updating' does NOT need the config detail — a link suffices (or omit)."

# READ (target files — exact current text is in findings §2/§3/§6/§8).
- file: README.md
  why: "the file being edited: §2a Install block (lines 82–115), §2b line-6 note, §2c 'Does it send
        my code anywhere new?' (lines 347–349), §2d new FAQ entry slot, §2e 'What about…' list
        (lines 377–379)."
- file: FUTURE_SPEC.md
  why: "the file being edited: §3 the 'Self-update command' rejection row (line 101)."
- file: docs/how-it-works.md
  why: "the file being edited: §6 the 'Safety invariant' section (line 195)."
- file: .goreleaser.yaml
  why: "the file being edited: §8 additive # comments under release:/brews:/scoops:/aurs:."

# REFERENCE (sibling deliverables the goreleaser comments + README channel docs point at).
- docfile: plan/017_397abce9deb1/P2M4T1S1/PRP.md
  why: "the parallel sibling (mise/asdf plugin). It produces plugins/asdf-stagecoach/ (the mise/asdf
        install channel). The README mise/asdf install line + the goreleaser mise/asdf pointer cite
        it. Treat its outputs as the contract: one set of POSIX-sh scripts serving BOTH mise and asdf
        (mise runs asdf plugins unchanged); the install line is `mise use stagecoach@latest` or
        `asdf plugin add stagecoach && asdf install stagecoach latest` (PRD §21.3)."
- file: docs/packaging.md
  why: "the Mode A distribution doc (WinGet + Nix details). The README WinGet/Nix install lines +
        the goreleaser winget/nix pointers are consistent with it. It already documents
        `winget install dabstractor.Stagecoach` and `nix profile install github:dabstractor/stagecoach`."
- file: .github/workflows/release.yml
  why: "the beyond-goreleaser channels' CI home: jobs npm-publish (npm) + winget (WinGet). The
        goreleaser comments point at these by job name."

# REFERENCE — markdownlint rules in force (the validation gate).
- file: .markdownlint.json
  why: "default:true; MD013 (line-length) OFF, MD033 (inline-HTML) OFF, MD060 OFF. So MD041 (first
        line = H1), MD022 (blanks around headings), MD032 (blanks around lists), MD040 (fenced code
        needs a language), MD024 (no dup headings), MD009 (no trailing spaces), MD012 (≤1 blank) are
        ON. Every new code fence needs a language tag; every new heading needs blank lines around it."
```

### Current Codebase tree (relevant slice)

```bash
README.md                       # EDIT: Install block + Updating + line-6 + 3 FAQ touchpoints
FUTURE_SPEC.md                  # EDIT: self-update row → SUPERSEDED (line 101)
docs/how-it-works.md            # EDIT: Safety invariant §19 scoping sentence (line ~195)
.goreleaser.yaml                # EDIT: additive # comments under release/brews/scoops/aurs
docs/cli.md                     # READ-ONLY (the authoritative upgrade docs the README LINKS to)
docs/configuration.md           # READ-ONLY (the authoritative [upgrade] table; README doesn't duplicate)
docs/packaging.md               # READ-ONLY (WinGet/Nix Mode A doc; README is consistent with it)
PRD.md                          # READ-ONLY (§9.29, §19, §21.3, §21.5, Appendix F = the sources)
.github/workflows/release.yml   # READ-ONLY (npm-publish + winget jobs the goreleaser comments cite)
.markdownlint.json              # READ-ONLY (rules: default true; MD013/033/060 OFF)
plan/017_397abce9deb1/P3M1T1S1/research/findings.md   # THIS item's verified research
```

### Desired Codebase tree with files to be edited

```bash
README.md                       # MODIFIED (Install rewrite + Updating + line-6 + FAQ cluster)
FUTURE_SPEC.md                  # MODIFIED (self-update row → SUPERSEDED note)
docs/how-it-works.md            # MODIFIED (Safety invariant: + §19 scoping sentence)
.goreleaser.yaml                # MODIFIED (additive # comments under release/brews/scoops/aurs)
# NOT touched: any .go/.sh/.json, flake.nix, npm/*, providers/*, release.yml, ci.yml, go.mod/sum,
#              .gitignore, docs/cli.md, docs/configuration.md, docs/packaging.md, PRD.md, plan/**,
#              tasks.json, install.sh (does NOT exist — see Gotcha 1). NO new files.
```

### Known Gotchas of our codebase & Library Quirks

```
GOTCHA 1 (install.sh DOES NOT EXIST — the one landmine). PRD §21.3 + the current README both give
  `curl -fsSL https://github.com/dabstractor/stagecoach/raw/main/install.sh | bash`, but an
  exhaustive `find . -iname install.sh` returns NOTHING. system_context.md:49 lists install.sh among
  the artifacts that did NOT exist pre-plan, and NO P2 PRP creates it. CREATING install.sh IS OUT OF
  SCOPE (OUTPUT: "No code changes"). RESOLUTION: document the curl|sh line per PRD §21.3 VERBATIM
  (§21.3 + the task INPUT are the source of truth; do NOT invent a different command), and FLAG this
  as a residual risk (install.sh must be created before the v3.0 release for the curl|sh channel to
  function). Do NOT create install.sh. Do NOT silently drop the curl|sh channel. Record the gap in
  the subtask completion. (The other 7 channels have working plumbing — goreleaser brews/scoops/aurs
  + release.yml npm-publish/winget + flake.nix + plugins/asdf-stagecoach.)

GOTCHA 2 (the §19 claim must be SCOPED, not deleted, in TWO places). The README FAQ
  "Does it send my code anywhere new?" currently says "Stagecoach never opens an HTTP connection to
  any API" — that is now FALSE for `stagecoach upgrade`. Scope it to the commit path + name the
  upgrade exception. Do the SAME in docs/how-it-works.md "Safety invariant" (different audience).
  Use the IDENTICAL framing in both (findings §1): "no network calls on the commit path;
  stagecoach upgrade is the one exception and it touches only release artifacts." Never leave either
  narrative making the absolute claim.

GOTCHA 3 (FUTURE_SPEC: SUPERSEDED, not deleted). The file's own footer says "delete it here" when an
  idea moves to the PRD — but the TASK explicitly says to MIRROR APPENDIX F's superseded entry (keep
  + annotate). The in-file precedent is the chunking row (line ~99): kept + a NOTE "has graduated to
  the spec — see PRD §9.24". Do the SAME for self-update: keep the row, prepend **SUPERSEDED**, cite
  §9.29 FR-U1–U12 + Appendix F, give the delegate-first rationale (never overwrites a manager-owned
  file → the original concern is resolved). Do NOT delete the row.

GOTCHA 4 (the README "Updating" subsection must NOT duplicate docs/cli.md). docs/cli.md `### upgrade`
  (P1.M4.T1.S1, Mode A) already has the full flag table, exit codes, and the Network bullet.
  docs/configuration.md `[upgrade]` (P1.M1.T2.S1) has the config table. The README "Updating" is a
  SUMMARY + a LINK to docs/cli.md#upgrade — a handful of example commands + the --check/--rollback
  facts + the walled-off note. It MUST NOT re-list every flag or the [upgrade] keys. (Duplicating
  would create a third source of truth that drifts.)

GOTCHA 5 (mise + asdf are ONE install line). PRD §21.3 + the P2.M4.T1.S1 PRP: mise runs asdf plugins
  unchanged (same scripts, same ASDF_* env vars). The README install line is the §21.3 two-form line:
  `mise use stagecoach@latest   # or: asdf plugin add stagecoach && asdf install stagecoach latest`.
  Do NOT give mise and asdf separate code blocks or separate channel headings.

GOTCHA 6 (markdownlint rules in force — .markdownlint.json). default:true with MD013 (line-length),
  MD033 (inline HTML), MD060 OFF. So every NEW fenced block needs a language tag (MD040, e.g.
  ```bash), every heading needs a blank line before AND after (MD022/MD032), no trailing whitespace
  (MD009), no duplicate heading text (MD024 — the new FAQ heading must be unique), ≤1 consecutive
  blank line (MD012). The existing README already uses `> [!NOTE]` admonitions (those are fine).
  markdownlint-cli is NOT on PATH locally — validate with `npx --yes markdownlint-cli@latest` (needs
  network) OR follow the rules by hand + a grep sweep (findings §11).

GOTCHA 7 (the README line-6 "what's new" is ONE sentence). It currently ends at v2.1. Appending a
  v3.0 clause keeps the "what's new" accurate AND makes the FUTURE_SPEC reconciliation consistent
  (no source may still imply self-update is unreleased). Keep it to a clause, not a paragraph:
  "v3.0 adds `stagecoach upgrade` (delegate-first self-update) and expands distribution (Homebrew,
  Scoop, Winget, npm, Nix, mise/asdf)." Match the existing sentence's voice.

GOTCHA 8 (.goreleaser.yaml comments are ADDITIVE # lines; never edit a field). The file header
  already has an owner note (dabstractor/stagecoach + tap + scoop-bucket). brews:/scoops:/aurs: each
  already have inline field comments. The new comments ADD a "Beyond goreleaser" pointer block
  (npm/winget/nix/mise-asdf) so the full distribution picture is visible. `#` comments never break
  YAML parsing — but run `goreleaser check` anyway (goreleaser IS on PATH at ~/.local/bin/goreleaser)
  to prove no accidental field edit slipped in.

GOTCHA 9 (Winget PackageIdentifier casing differs between §21.3 and packaging.md — both are
  correct, different surfaces). PRD §21.3 says `winget install dabstractor.stagecoach` (the
  winget-install CLI form, lowercase dot). docs/packaging.md uses the manifest PackageIdentifier
  `dabstractor.Stagecoach` (PascalCase, the manifest form). The README INSTALL line uses the §21.3
  CLI form `winget install dabstractor.stagecoach` (that is what a user types). Do not "fix" the
  casing to match packaging.md — they are two different identifiers for two different contexts.

GOTCHA 10 (consistency across 4 files — the core of a Mode B task). self-update's status must agree
  everywhere: README line-6 (shipped), README "What about…" list (removed from deferred), FUTURE_SPEC
  (SUPERSEDED). §19 framing must agree: README FAQ + how-it-works.md (identical scoping). Channel
  names must agree: README install block + goreleaser comments + docs/packaging.md. A grep sweep
  (findings §10/§11) enforces this — run it before declaring done.
```

## Implementation Blueprint

### Data models and structure
None — Markdown prose + YAML comments. The "data" is the install-command table (PRD §21.3), the §19
scoping sentence (findings §1), and the FUTURE_SPEC superseded note (Appendix F).

### Implementation Tasks (ordered by file; no inter-file dependencies)

```yaml
Task 1: README.md — rewrite the `## Install` block (the main deliverable)
  - LOCATE: README.md lines 82–115 (findings §2a). Current: a `> [!NOTE]` claiming "build from
    source is the only working method … coming with the first public release"; `### Build from source
    (works today)`; `### Coming soon` (Homebrew/Scoop/go-install/curl|sh).
  - DO (in order, preserving the `## Install` heading + the Prerequisite line + the build-from-source
    block):
    (a) REWRITE the `> [!NOTE]` (lines 86–89): drop the "only working method / coming soon" claim.
        Replace with a short note that stagecoach ships via the package-managed channels below +
        `stagecoach upgrade` keeps them current (one sentence; cross-link the Updating subsection).
    (b) DEMOTE `### Build from source (works today)` → `### Build from source` (drop "(works
        today)"); keep its body (git clone → make install → verify → symlink TIP) unchanged. It is
        now one option, not the default.
    (c) REPLACE `### Coming soon` (lines 108–115) with `### Package managers` (or keep no subheading
        if cleaner) listing ALL 8 channels per PRD §21.3 (findings §4) — VERBATIM commands, grouped
        by platform/audience as §21.3 groups them:
          Homebrew (macOS/Linuxbrew):  brew install dabstractor/tap/stagecoach
          Scoop (Windows):             scoop install dabstractor/stagecoach
          Winget (Windows, Win11 default): winget install dabstractor.stagecoach
          npm (zero-install trial: npx stagecoach; or global): npm install -g @dabstractor/stagecoach
          Nix (flake):                 nix profile install github:dabstractor/stagecoach
          mise / asdf (version-manager): mise use stagecoach@latest
                                        # or: asdf plugin add stagecoach && asdf install stagecoach latest
          go install (Go):             go install github.com/dabstractor/stagecoach/cmd/stagecoach@latest
          Direct binary (curl|sh):     curl -fsSL https://github.com/dabstractor/stagecoach/raw/main/install.sh | bash
        One fenced ```bash block (or a compact list — match the existing README style, which uses
        inline-code bullets; a single ```bash block with # comments per line is the §21.3 shape and
        is markdownlint-clean with a language tag — GOTCHA 6). Keep mise+asdf on ONE line (GOTCHA 5).
  - CRITICAL: the curl|sh line is per §21.3 VERBATIM even though install.sh does not exist yet
    (GOTCHA 1). Do NOT invent a different command; do NOT create install.sh.
  - FOLLOW: PRD §21.3 (findings §4) is the source of truth for every command; §21.5 dictates Install
    is README section #4 and the channel grouping.
  - NAMING: `## Install` (unchanged); `### Updating` (new, Task 2); channel subheadings only if they
    aid skimmability (the existing README used prose bullets — keep that voice).

Task 2: README.md — ADD the `### Updating` subsection (after the install commands, before `## Quick start`)
  - LOCATE: README.md line 117 (`## Quick start`). INSERT `### Updating` immediately before it,
    inside `## Install`.
  - DO: a short subsection (≈6–10 lines + one ```bash block) covering:
    - `stagecoach upgrade` — one command across EVERY channel; detect→delegate to the channel's own
      updater (Homebrew/Scoop/Winget/npm/mise/asdf/go-install), print where it needs privileges (AUR)
      or is declarative (Nix), self-swap ONLY for the direct (curl|sh/manual) channel. Never fights a
      package manager (never overwrites a manager-owned file).
    - `stagecoach upgrade --check` (or `-c`) — exit 6 if an update is available, 0 if up to date;
      the side-effect-free CI/cron gate.
    - `stagecoach upgrade --rollback` — one-step restore of the prior binary.
    - NOTE: walled off from the commit core — acquires no run lock, reads no repo, invokes no
      provider; repo-independent.
    - LINK: "See [docs/cli.md#upgrade](docs/cli.md#upgrade) for the full flag reference" (GOTCHA 4 —
      do NOT re-derive the flag table; docs/cli.md is authoritative).
  - ```bash example block (3 lines): `stagecoach upgrade` / `stagecoach upgrade --check` /
    `stagecoach upgrade --rollback`. Language tag (MD040, GOTCHA 6).
  - FOLLOW: docs/cli.md `### upgrade` (findings §7) for the precise wording of detect→delegate and
    the exit codes; PRD §9.29 FR-U1/U6/U8/U12 for the contract.
  - NAMING: `### Updating` (sentence-case heading; unique under Install — MD024).

Task 3: README.md — FAQ security cluster (scope existing + add new)
  - 3a. SCOPE `### Does it send my code anywhere new?` (lines 347–349, findings §2c):
        Current: "… Stagecoach never opens an HTTP connection to any API — your agent does, exactly as
        it would if you ran it manually."
        EDIT the sentence to scope it: "On the commit path, stagecoach makes no network calls itself
        — it shells out to your agent under your existing auth and billing, and your agent opens the
        connection exactly as it would if you ran it manually. The single exception is
        `stagecoach upgrade` (see below), which fetches only this project's own release artifacts and
        checksums." (Use findings §1 framing — identical to how-it-works.md, GOTCHA 2.)
  - 3b. ADD `### Does stagecoach make network calls?` (NEW entry, findings §2d; the task explicitly
        requires it). Place it right AFTER `### Does it send my code anywhere new?` (the security
        cluster). Body (≈3–4 sentences): commit path (§9.1–§9.28) = zero network calls; the ONE named
        exception is `stagecoach upgrade` (§9.29), which fetches ONLY the project's own GitHub
        release artifacts + checksums (never provider credentials, never a diff, never repo data);
        cross-link `### Updating` above + docs/cli.md#upgrade. Use the §1 phrasing verbatim.
  - FOLLOW: v3_scope_boundary.md §19 (findings §1) + merged PRD §19 "Diff content is local" bullet +
    §9.29 intro 2nd para. Identical framing to how-it-works.md (Task 6).
  - NAMING: the new heading text is unique (MD024); sentence case to match the existing FAQ headings.

Task 4: README.md — reconcile the `### What about PR generation…` list + the line-6 note
  - 4a. EDIT `### What about PR generation, editor extensions, a GitHub Action, API-key providers?`
        (lines 377–379, findings §2e): REMOVE the word "self-update" from the deferred/rejected list
        ("VS Code/neovim extensions, a GitHub Action, gitui integration, API-key HTTP providers,
        generate-N-and-pick, diff chunking, self-update, and more" → drop "self-update,"). Optionally
        add a half-sentence: "(self-update shipped in v3.0 as `stagecoach upgrade` — see
        [Updating](#updating).)" Keep every other item.
  - 4b. EDIT line 6 (findings §2b, GOTCHA 7): append a v3.0 clause to the "what's new" sentence:
        "… model discovery — see [Features](#features) below. v3.0 adds `stagecoach upgrade`
        (delegate-first self-update) and expands distribution (Homebrew, Scoop, Winget, npm, Nix,
        mise/asdf)." Match the existing sentence's voice; keep it ONE clause.

Task 5: FUTURE_SPEC.md — mark the self-update row SUPERSEDED (line 101)
  - LOCATE: FUTURE_SPEC.md line 101 (findings §3): the row
    `| **Self-update command** (aicommits) | Distribution is Homebrew/AUR/Scoop/\`go install\`; a self-updating binary fights its package manager and breaks checksums. |`
  - DO (GOTCHA 3 — SUPERSEDED, not deleted; mirror Appendix F + the in-file chunking-row pattern):
    REWRITE the row's right cell to:
      "**SUPERSEDED by PRD §9.29 (v3.0; FR-U1–U12) — see also Appendix F.** The v2.1 rejection
      ('package managers own binary updates') assumed self-update meant a tool that downloads and
      overwrites its own binary, which fights whatever package manager installed it and gets silently
      reverted on that manager's next upgrade. With the distribution surface widening to
      npm/Winget/Nix/mise/asdf on top of Brew/Scoop/AUR/go-install, that concern got sharper, not
      softer. v3.0 reverses the rejection via the **inverse architecture — install-method-aware,
      delegate-first**: `stagecoach upgrade` detects the install method and delegates to that
      channel's native updater wherever one exists, prints the command where it needs privileges (AUR)
      or is declarative (Nix), and self-swaps ONLY for the direct-binary (curl|sh/manual) channel —
      it never overwrites a package-manager-owned file, so it cannot fight a manager. The original
      rejection (a self-overwriting binary) does not apply to the delegate-first design."
    (Mirror Appendix F's superseded entry, PRD line 2456; cite §9.29 FR-U1–U12 + Appendix F.)
  - FOLLOW: PRD Appendix F line 2456 (verbatim rationale) + the in-file chunking row (line ~99) for
    the keep-and-annotate shape.
  - NAMING: keep the left cell `**Self-update command** (aicommits)` unchanged; edit only the right
    cell. The row stays in the "## 3. Rejected" table.
  - GOTCHA: do NOT delete the row (the task says mirror Appendix F, which keeps superseded entries);
    do NOT move it to §1 (it's a superseded REJECTION, not a deferred idea).

Task 6: docs/how-it-works.md — append the §19 scoping sentence to "Safety invariant"
  - LOCATE: docs/how-it-works.md "### Safety invariant" (line 195, findings §6). Current paragraph
    (line 197) covers no-mutation + read-only manifests + chrome-disable.
  - DO: APPEND one sentence (GOTCHA 2 — same framing as README Task 3) to the paragraph:
    "The commit-generation path (§9.1–§9.28) makes no network calls itself; the single named
    exception is `stagecoach upgrade` (§9.29), which fetches only this project's own GitHub release
    artifacts and checksums — never provider credentials, never a diff, never repo data."
  - FOLLOW: v3_scope_boundary.md §19 (findings §1) + merged PRD §19 "Diff content is local" bullet.
    Identical framing to README Task 3a/3b.
  - PLACEMENT: end of the "Safety invariant" paragraph (it's a security claim; belongs with the
    other security invariants). Do NOT add a new heading.

Task 7: .goreleaser.yaml — add "Beyond goreleaser" pointer comments (YAML `#` only)
  - LOCATE: four anchors (findings §8):
      `release:` block (~line 50); `brews:` block (~line 62); `scoops:` block (~line 78);
      `aurs:` block (~line 95, already commented).
  - DO (GOTCHA 8 — ADDITIVE `#` comments ONLY; never edit a field/value):
    (a) Under `release:` (after the `prerelease: auto` line, before `# Homebrew formula`): a comment
        block naming this Release as the single source ALL channels fetch from, and that goreleaser
        publishes ONLY brews/scoops/aurs + archives/checksums here; the OTHER channels (npm, WinGet,
        Nix, mise/asdf) are "Beyond goreleaser" (PRD §21.2) — implemented as: release.yml `npm-publish`
        job (npm wrapper @dabstractor/stagecoach), release.yml `winget` job
        (vedantmgoyal9/winget-releaser → microsoft/winget-pkgs), `flake.nix` (Nix), and
        `plugins/asdf-stagecoach/` (mise+asdf). So a future reader sees the full picture here.
    (b) Under `brews:` (after the existing `# Users install with: brew install …` comment): one line
        noting Homebrew is a goreleaser-native pipe; the beyond-goreleaser channels are listed under
        `release:` above (cross-reference, don't repeat the whole list).
    (c) Under `scoops:`: same one-line cross-reference as (b).
    (d) Under `aurs:` (optional, for completeness — it's already heavily commented): one line noting
        AUR is best-effort + the beyond-goreleaser channels are under `release:` above.
  - FOLLOW: the file header's existing owner note + the release.yml `npm-publish`/`winget` job
    comment headers ("PRD §21.2 'Beyond goreleaser'", findings §9). Use "Beyond goreleaser" as the
    discoverable phrase (it's the PRD §21.2 heading + the release.yml comment phrase).
  - NAMING: `#` comment lines only; no field/value edits. Match the file's existing comment style
    (capitalized, trailing rationale).

Task 8: VALIDATE — markdownlint rules + link integrity + consistency greps + goreleaser parse + git scope
  - RUN: the Level 1–4 commands. EXPECT: markdownlint-clean (or rule-compliant by hand); all internal
    links resolve (docs/cli.md#upgrade, docs/how-it-works.md anchors, #updating, #install); the §19
    scoping present in BOTH README FAQ + how-it-works.md with identical framing; FUTURE_SPEC row says
    SUPERSEDED + cites §9.29; README no longer lists self-update as deferred/rejected/coming-soon;
    .goreleaser.yaml parses (`goreleaser check` clean); git status shows ONLY the 4 files.
  - GATE: every check passes; the install.sh gap is recorded in the completion notes (GOTCHA 1).
```

### Implementation Patterns & Key Details

```
# PATTERN — the README "Updating" subsection (Task 2; a SUMMARY + LINK, not a flag table — GOTCHA 4):

### Updating

Keep stagecoach current with one command — it detects how you installed it and delegates to that
channel's own updater (Homebrew, Scoop, Winget, npm, mise, asdf, `go install`), prints the command
where running it needs privileges (AUR) or is declarative (Nix), and self-swaps only for a direct
(curl\|sh / manual) install. It never overwrites a package-manager-owned file, so it never fights
your package manager. (PRD §9.29; walled off from the commit core — no repo, run lock, or provider.)

```bash
stagecoach upgrade             # detect → delegate (or self-swap), confirm, apply
stagecoach upgrade --check     # exit 6 if an update is available, 0 if up to date (CI / cron)
stagecoach upgrade --rollback  # one-step restore of the prior binary
```

See [docs/cli.md#upgrade](docs/cli.md#upgrade) for the full flag reference.
# CRITICAL: 3 example lines only (not the full flag table — cli.md owns that); the --check→exit-6
# fact for CI/cron; the walled-off note; ONE link to docs/cli.md#upgrade.
```

```
# PATTERN — the README "Does stagecoach make network calls?" FAQ entry (Task 3b; §1 framing verbatim):

### Does stagecoach make network calls?

On the commit path (writing commit messages), **no** — stagecoach makes no network calls itself; it
shells out to your agent, and your agent makes the connection. The one named exception is
`stagecoach upgrade` (see [Updating](#updating)), which fetches **only** this project's own GitHub
release artifacts and checksums — never provider credentials, never a diff, never your repo data.
(PRD §19, scoped to the commit path in v3.0; §9.29 is the exception.) See
[docs/cli.md#upgrade](docs/cli.md#upgrade).
# CRITICAL: identical framing to docs/how-it-works.md (Task 6) — "commit path = none; upgrade = the
# named exception, release artifacts only". Do NOT leave the overbroad "never opens an HTTP
# connection" claim anywhere (Task 3a scopes the sibling FAQ entry).
```

```
# PATTERN — the FUTURE_SPEC superseded row (Task 5; mirror Appendix F, keep+annotate — GOTCHA 3):

| **Self-update command** (aicommits) | **SUPERSEDED by PRD §9.29 (v3.0; FR-U1–U12) — see Appendix F.** The v2.1 rejection ("package managers own binary updates") assumed self-update meant a tool that downloads and overwrites its own binary, which fights whatever package manager installed it and gets silently reverted on that manager's next upgrade. With the distribution surface widening to npm/Winget/Nix/mise/asdf on top of Brew/Scoop/AUR/go-install, that concern got sharper, not softer. v3.0 reverses the rejection via the **inverse architecture — install-method-aware, delegate-first**: `stagecoach upgrade` detects the install method and delegates to that channel's native updater wherever one exists, prints the command where it needs privileges (AUR) or is declarative (Nix), and self-swaps ONLY for the direct-binary (curl\|sh/manual) channel — it never overwrites a package-manager-owned file, so it cannot fight a manager. The original rejection (a self-overwriting binary) does not apply to the delegate-first design. |
# CRITICAL: left cell UNCHANGED; right cell rewritten to the superseded note; row STAYS in "## 3.
# Rejected" (it's a superseded rejection, not a deferred idea). Mirrors the in-file chunking-row shape.
```

```
# PATTERN — the .goreleaser.yaml "release:" comment block (Task 7a; # comments ONLY — GOTCHA 8):

release:
  github:
    owner: dabstractor
    name: stagecoach
  draft: false
  prerelease: auto
  # ── Distribution surface (PRD §21.2/§21.3) ──────────────────────────────────────────────────
  # This GitHub Release is the SINGLE source every install channel fetches from. goreleaser publishes
  # its NATIVE pipes here only — Homebrew (`brews:` below), Scoop (`scoops:` below), AUR (`aurs:`
  # below, best-effort) — plus the 6 archives + checksums.txt. The REMAINING channels are "Beyond
  # goreleaser" (PRD §21.2), implemented as separate CI/release artifacts, NOT goreleaser pipes:
  #   • npm      — release.yml `npm-publish` job → @dabstractor/stagecoach (npm/ wrapper)
  #   • WinGet   — release.yml `winget` job → vedantmgoyal9/winget-releaser → microsoft/winget-pkgs
  #   • Nix      — flake.nix (nix profile install github:dabstractor/stagecoach)
  #   • mise/asdf— plugins/asdf-stagecoach/ (one POSIX-sh plugin serves both; see docs/packaging.md)
  # `go install` and the curl|sh direct download consume the archives published here directly.
# CRITICAL: pure `#` comments; no field touched. brews:/scoops:/aurs: get a one-line cross-reference
# to this block (not the full list — DRY). goreleaser check must still pass.
```

### Integration Points

```yaml
CONSUMES (READ-ONLY — the authoritative sources this task surfaces/links):
  - PRD.md §9.29 / §19 / §21.3 / §21.5 / Appendix F: the upgrade contract, the §19 scoping, the
    install commands, the README section order, the superseded rationale. NOT edited.
  - docs/cli.md `### upgrade` (P1.M4.T1.S1): the authoritative upgrade flag table/exit codes. The
    README LINKS it; does NOT duplicate. NOT edited.
  - docs/configuration.md `[upgrade]` (P1.M1.T2.S1): the authoritative config table. NOT edited.
  - docs/packaging.md (P2 Mode A): the WinGet/Nix detail. README is consistent with it. NOT edited.
  - .github/workflows/release.yml: the npm-publish + winget jobs the goreleaser comments cite. NOT edited.
  - plugins/asdf-stagecoach/ (P2.M4.T1.S1, parallel sibling): the mise/asdf channel. README cites it.
PRODUCES:
  - README.md, FUTURE_SPEC.md, docs/how-it-works.md, .goreleaser.yaml — MODIFIED (doc/comments only).
NO code/script/release/config/PRD/plan/tasks changes. install.sh is NOT created (does not exist;
creating it is out of scope — GOTCHA 1, flagged as a residual risk).
```

## Validation Loop

> **This is a Markdown + YAML-comment task.** The template's `ruff`/`mypy`/`pytest`/`go build` gates
> do NOT apply. Validation = markdownlint rules + link integrity + consistency greps + a goreleaser
> parse + a git-scope guard. `markdownlint-cli` is NOT on PATH locally — use `npx` (needs network) or
> follow `.markdownlint.json` rules by hand + the grep sweeps below (findings §11).

### Level 1: Markdown quality (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach
# If network is available, run the linter the repo is configured for (.markdownlint.json):
npx --yes markdownlint-cli@latest README.md FUTURE_SPEC.md docs/how-it-works.md 2>/dev/null \
  && echo "markdownlint clean" \
  || echo "markdownlint not available or found issues — apply the rules by hand (GOTCHA 6)"
# Rules in force (.markdownlint.json): default true; MD013 (line-len) OFF, MD033 (inline HTML) OFF,
# MD060 OFF. So: MD040 (every fenced block needs a language tag — ```bash), MD022/MD032 (blank line
# around every heading/list), MD024 (no duplicate heading text — "### Updating" + "### Does
# stagecoach make network calls?" must be unique), MD009 (no trailing whitespace), MD012 (≤1 blank).
# Hand-check each NEW heading/fence you added against those five.
```

### Level 2: Link & anchor integrity (Component Validation)

```bash
cd /home/dustin/projects/stagecoach
# Internal links the README adds must resolve:
grep -n 'docs/cli.md#upgrade\|#updating\|#install' README.md   # the links exist in the text
test -f docs/cli.md && grep -q '^### `upgrade`' docs/cli.md && echo "docs/cli.md#upgrade anchor OK"
grep -q '## Install' README.md && echo "#install anchor OK"
grep -q '### Updating' README.md && echo "#updating anchor OK (added)"
# External/channel links are well-formed GitHub/npm URLs (spot-check, no network needed):
grep -oE 'https://github.com/dabstractor/stagecoach[^ )]*|https://registry.npmjs.org|@dabstractor/stagecoach' README.md | sort -u
# Expected: the install URLs are present and consistent with PRD §21.3 (findings §4).
```

### Level 3: .goreleaser.yaml still parses (System Validation)

```bash
cd /home/dustin/projects/stagecoach
# goreleaser IS on PATH (~/.local/bin/goreleaser). Comments never break YAML, but this PROVES no
# accidental field edit slipped in. (Needs no network for `check`.)
goreleaser check 2>&1 | tail -5
# Expected: "config is valid" / "configuration passed". If it errors, a `#` comment accidentally
# landed mid-value or a field was edited — revert to comments-only (GOTCHA 8).
# Fallback (no goreleaser): prove the YAML parses at all:
python3 -c "import yaml; yaml.safe_load(open('.goreleaser.yaml')); print('goreleaser.yaml parses OK')"
# Expected: "… parses OK".
```

### Level 4: Consistency & scope guards (Domain-Specific Validation)

```bash
cd /home/dustin/projects/stagecoach
# (1) §19 scoping present in BOTH security narratives with the upgrade-as-exception framing:
grep -qi 'commit path' README.md && grep -qi 'stagecoach upgrade' README.md && echo "README §19 scoping OK"
grep -qi 'commit-generation path\|commit path' docs/how-it-works.md && grep -qi 'stagecoach upgrade' docs/how-it-works.md && echo "how-it-works §19 scoping OK"
# Expected: both OK. (GOTCHA 2 — identical framing.)

# (2) self-update is RECONCILED everywhere (no source still calls it deferred/rejected/coming-soon):
grep -ni 'self-update' FUTURE_SPEC.md | grep -i SUPERSEDED && echo "FUTURE_SPEC SUPERSEDED OK"
! grep -qi 'coming soon' README.md && echo "README no 'coming soon' OK"
grep -q '### Updating' README.md && echo "README Updating subsection OK"
# The README "What about…" list no longer includes self-update as deferred:
grep -A2 'What about PR generation' README.md | grep -qi 'self-update' && echo "FAIL: self-update still in deferred list" || echo "README deferred-list reconciled OK"

# (3) FUTURE_SPEC row cites §9.29 + Appendix F:
grep -q 'SUPERSEDED by PRD §9.29' FUTURE_SPEC.md && grep -qi 'delegate-first\|delegate.first' FUTURE_SPEC.md && echo "FUTURE_SPEC rationale OK"

# (4) All 8 channels present in README install block (PRD §21.3 — findings §4):
for c in 'brew install' 'scoop install' 'winget install' 'npm install -g' 'nix profile install' 'mise use' 'asdf plugin add' 'go install' 'install.sh'; do
  grep -q "$c" README.md && echo "channel OK: $c" || echo "MISSING channel: $c"
done
# Expected: all 9 greps OK (mise+asdf share a line; curl|sh → install.sh).

# (5) .goreleaser.yaml has the beyond-goreleaser pointer comments:
grep -qi 'Beyond goreleaser' .goreleaser.yaml && echo "goreleaser beyond-goreleaser pointer OK"
grep -qi 'npm-publish\|winget\|flake.nix\|asdf-stagecoach' .goreleaser.yaml && echo "goreleaser channel pointers OK"

# (6) SCOPE GUARD — ONLY the 4 files changed; no code/script/release file touched:
git status --porcelain
# Expected EXACTLY: " M README.md", " M FUTURE_SPEC.md", " M docs/how-it-works.md", " M .goreleaser.yaml"
git status --porcelain | grep -vE '^ M (README\.md|FUTURE_SPEC\.md|docs/how-it-works\.md|\.goreleaser\.yaml)$' \
  && echo "FAIL: unexpected file changed" || echo "scope OK (only the 4 doc/comment files)"
# Forbidden: any .go/.sh/.json, flake.nix, npm/*, providers/*, release.yml, ci.yml, go.mod/sum,
# .gitignore, docs/cli.md, docs/configuration.md, docs/packaging.md, PRD.md, plan/**, tasks.json, install.sh.
```

## Final Validation Checklist

### Technical Validation
- [ ] markdownlint-clean on README.md / FUTURE_SPEC.md / docs/how-it-works.md (Level 1) — or hand-
      verified against `.markdownlint.json` (MD040/022/032/024/009/012; MD013/033/060 off).
- [ ] All internal links resolve: `docs/cli.md#upgrade`, `#updating`, `#install`, the how-it-works
      anchors (Level 2).
- [ ] `goreleaser check` is clean (Level 3) — proves the .goreleaser.yaml edits are comments-only.
- [ ] Level 4 consistency greps all pass (§19 in both narratives; self-update reconciled everywhere;
      FUTURE_SPEC cites §9.29 + delegate-first; all 8 channels present; goreleaser pointers present).
- [ ] `git status --porcelain` shows EXACTLY the 4 files; the scope guard prints "scope OK".

### Feature Validation
- [ ] README `## Install`: all 8 channels with PRD §21.3 commands verbatim; "Build from source"
      demoted (no "(works today)"/"only method"); the stale "coming soon" NOTE gone/rewritten.
- [ ] README `### Updating`: `stagecoach upgrade` (delegate-first) + `--check` (exit 6, CI/cron) +
      `--rollback` + walled-off note + a LINK to docs/cli.md#upgrade (no flag-table duplication).
- [ ] README FAQ: new `### Does stagecoach make network calls?` entry; existing
      `### Does it send my code anywhere new?` scoped to the commit path + upgrade exception;
      "self-update" removed from `### What about PR generation…`.
- [ ] README line-6 appends the v3.0 clause (self-update + expanded distribution).
- [ ] FUTURE_SPEC line 101: SUPERSEDED note citing §9.29 (FR-U1–U12) + Appendix F + delegate-first.
- [ ] docs/how-it-works.md "Safety invariant": §19 scoping sentence appended (identical framing to
      the README FAQ).
- [ ] .goreleaser.yaml: "Beyond goreleaser" pointer comments under release/brews/scoops/aurs naming
      npm/winget/nix/mise-asdf with their actual locations (release.yml jobs / flake.nix / plugins/).

### Scope-Boundary Validation
- [ ] `git status --porcelain` shows ONLY: `M README.md`, `M FUTURE_SPEC.md`,
      `M docs/how-it-works.md`, `M .goreleaser.yaml`.
- [ ] NO edit to any `.go`/`.sh`/`.json`, `flake.nix`, `npm/*`, `providers/*`, `release.yml`,
      `ci.yml`, `go.mod`/`go.sum`, `.gitignore`, `Makefile`, `PRD.md`, `plan/**`, `tasks.json`.
- [ ] NO edit to `docs/cli.md` / `docs/configuration.md` / `docs/packaging.md` (the authoritative
      Mode A docs the README LINKS to — duplication would create drift).
- [ ] `.goreleaser.yaml` changes are `#` comments ONLY (no field/value edit) — `goreleaser check` clean.
- [ ] `install.sh` is NOT created (does not exist; creating it is out of scope — GOTCHA 1). The gap
      is RECORDED in the subtask completion as a release-day follow-up.

### Documentation & Deployment
- [ ] §19 framing is IDENTICAL across the README FAQ + docs/how-it-works.md (commit path = none;
      upgrade = named exception, release artifacts only).
- [ ] self-update's status agrees everywhere (README line-6 shipped; README deferred-list removed;
      FUTURE_SPEC SUPERSEDED) — no source still calls it deferred/rejected/coming-soon.
- [ ] Channel names agree across README install block + goreleaser comments + docs/packaging.md.
- [ ] The install.sh gap (GOTCHA 1) is recorded in the subtask completion: install.sh must be
      created before the v3.0 release for the curl|sh channel to function.

---

## Anti-Patterns to Avoid

- ❌ Don't DUPLICATE the `upgrade` flag table or the `[upgrade]` config keys in the README —
  docs/cli.md (P1.M4.T1.S1) and docs/configuration.md (P1.M1.T2.S1) own those; the README SUMMARIZES
  + LINKS (GOTCHA 4). A third copy drifts.
- ❌ Don't DELETE the FUTURE_SPEC self-update row — the task says MIRROR Appendix F's superseded
  entry (keep + annotate). The file's "delete it here" footer is overridden by the explicit task
  instruction (GOTCHA 3).
- ❌ Don't leave the absolute "Stagecoach never opens an HTTP connection" claim anywhere — it's now
  false for `stagecoach upgrade`. Scope it to the commit path + name the exception in BOTH the README
  FAQ and docs/how-it-works.md (GOTCHA 2).
- ❌ Don't INVENT a curl|sh command or a different install URL — PRD §21.3 is verbatim the README
  source (GOTCHA 1). Don't create install.sh (out of scope).
- ❌ Don't edit any `.goreleaser.yaml` FIELD — comments only (`#`). A field edit breaks the release
  config and fails `goreleaser check` (GOTCHA 8).
- ❌ Don't give mise and asdf separate channel headings/code blocks — they share one §21.3 line and
  one plugin (GOTCHA 5).
- ❌ Don't "fix" the Winget casing to match packaging.md — `dabstractor.stagecoach` (CLI install form,
  README) ≠ `dabstractor.Stagecoach` (manifest form, packaging.md); both are correct (GOTCHA 9).
- ❌ Don't touch any code/script/release/PRD/plan/tasks file — this is doc/comments only.