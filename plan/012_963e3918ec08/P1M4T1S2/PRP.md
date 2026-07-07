---
name: "P1.M4.T1.S2 (project rename Layer 5.2, Mode A) — Rename docs/*.md: stagehand → stagecoach (cli.md, configuration.md, how-it-works.md, providers.md, README.md)"
description: |

  The documentation rename of the project's docs/ tree, as Layer 5.2 of the stagehand→stagecoach project
  rename (plan 012). All Go source, config surfaces, and build/CI have already been renamed (M1–M3); the
  five docs/*.md files still say "stagehand"/"Stagehand"/"STAGEHAND" (~526 occurrences across 5 files).
  Apply a global case-variant sed over all five files, then VERIFY the five contract criteria (env vars,
  git config keys, config paths, exclusion file, binary name) match the already-renamed Go code, plus
  cross-surface consistency (the lazygit integration marker, the go-install path) and anchor-link safety.

  CONTRACT (item_description §3, verbatim):
    `sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g'
    docs/cli.md docs/configuration.md docs/how-it-works.md docs/providers.md docs/README.md`
    Then review: (a) env vars match STAGECOACH_ prefix; (b) git config keys match stagecoach.* section;
    (c) config file paths match ~/.config/stagecoach/config.toml and .stagecoach.toml;
    (d) exclusion file is .stagecoachignore; (e) CLI binary name is stagecoach.

  DELIVERABLE (5 files modified; nothing else): MODIFY the five docs/*.md files — one global case-variant
    sed each (~526 occurrences total), then verification. All env vars, git-config keys, paths, the exclusion
    file, the binary name, the integration marker, and the go-install path become stagecoach/Stagecoach/
    STAGECOACH, matching the already-renamed Go code (M1–M3 LANDED).

  SCOPE NOTE (the sed is complete + safe, design §1): the docs are a clean reference surface — there are NO
    provenance/rename-history notes to preserve (the only "originally" is how-it-works.md:212 "originally
    staged files" = the snapshot recipe, unrelated; the rename-history note is PRD h2.30, not a docs file),
    NO commit-pi refs, and "stagehand" never appears as a substring of an unrelated word. So a global sed is
    correct: after it, `grep -ri stagehand docs/*.md` MUST return ZERO hits (the Layer-5 gate).

  SCOPE NOTE (the review items are VERIFICATION, not extra edits, design §2): the sed ALREADY produces every
    target in (a)–(e). The review is a CHECK that the output matches the renamed Go code — not additional edits.

  SCOPE NOTE (anchor-link safety — the key insight, design §5): several docs HEADINGS contain "stagehand"
    (e.g. how-it-works.md `### Payload exclusions (.stagehandignore)` → slug `#payload-exclusions-stagehandignore`),
    and other docs LINK to those anchors (docs/README.md:42). Because the sed runs over ALL 5 docs files in
    ONE pass, it renames BOTH the heading text (→ the slug) AND the link's `#fragment` consistently, so every
    intra-docs link stays valid. Cross-tree links to the repo-root README (S1's target) use stagehand-free
    filenames/anchors (README.md, #contributing) ⇒ resolve unchanged.

  SCOPE BOUNDARY (what this does NOT do): NO edits to the repo-root README.md (P1.M4.T1.S1's scope — parallel),
    providers/*.toml (P1.M4.T2.S1), FUTURE_SPEC.md (P1.M4.T2.S2), plan/ artifacts (P1.M5.T1.S1), or any
    .go/.yaml/.yml/Makefile/go.mod file (already renamed M1–M3). This is FIVE files in docs/.

  INPUT (upstream — already renamed, do NOT re-rename): go.mod (`github.com/dustin/stagecoach`), the renamed
    `cmd/stagecoach` dir, the Go integration marker `lazygitMarker = "stagecoach-integration"`
    (internal/cmd/integrate_lazygit.go:20), the exclusion constant `StagecoachIgnoreFile = ".stagecoachignore"`
    (internal/exclude/exclude.go:28), the renamed env-var prefix STAGECOACH_ / git-config section stagecoach.* /
    paths (M2 LANDED), the renamed binary `stagecoach` (M3 LANDED). The repo-root README.md is being renamed
    in parallel by P1.M4.T1.S1. OUTPUT: docs/*.md uses stagecoach throughout; no mismatched env vars, config
    keys, paths, or markers; `grep -ri stagehand docs/*.md` == 0.

  ⚠️ Run the sed from the repo root (the module dir) over EXACTLY the 5 docs files (or `docs/*.md`, which
     matches exactly those 5). Do NOT touch the repo-root README.md (S1) or any non-docs file.
  ⚠️ The docs/cli.md lazygit marker `# stagecoach-integration` (post-sed) MUST equal the Go constant
     `lazygitMarker` (integrate_lazygit.go:20). VERIFY they match — a mismatch breaks `integrate uninstall`.
  ⚠️ The docs/README.md go-install path MUST match go.mod (`github.com/dustin/stagecoach/cmd/stagecoach`).
  ⚠️ Namespace is `dustin` (NOT dabstractor) — docs/README.md's 2 GitHub URLs use `dustin/stagehand` → sed →
     `dustin/stagecoach`. Do NOT introduce dabstractor.

  Deliverable: 5 modified docs files; `grep -ri stagehand docs/*.md` == 0; `go build ./... && go test ./...`
  green & unchanged (no code touched — build/test is a no-op regression check).

---

## Goal

**Feature Goal**: Rename the project's docs/ tree (5 files) from stagehand to stagecoach — every env-var
reference (STAGECOACH_), git-config key (stagecoach.*), config path (~/.config/stagecoach/config.toml,
.stagecoach.toml), the exclusion file (.stagecoachignore), the binary name (stagecoach), the lazygit
integration marker, and the go-install path — so the docs present a unified stagecoach identity consistent
with the already-renamed go.mod, .goreleaser, and Go source (M1–M3). Close Layer 5.2 of the project rename.

**Deliverable** (5 files modified; nothing else):
- `docs/cli.md`, `docs/configuration.md`, `docs/how-it-works.md`, `docs/providers.md`, `docs/README.md` —
  global case-variant sed (stagehand→stagecoach, Stagehand→Stagecoach, STAGEHAND→STAGECOACH) across ~526
  occurrences, then verification of the five contract criteria + cross-surface consistency + anchor safety.

**Success Definition**: `grep -ri stagehand docs/*.md` returns ZERO hits; all env vars are `STAGECOACH_*`;
all git-config keys are `stagecoach.*`; config paths are `~/.config/stagecoach/config.toml` +
`.stagecoach.toml`; the exclusion file is `.stagecoachignore`; the binary/examples are `stagecoach …`; the
docs/cli.md lazygit marker is `# stagecoach-integration` (== Go `lazygitMarker`); the docs/README.md
go-install path matches go.mod; ZERO `dabstractor`; no docs anchor heading still contains stagehand; the
known intra-docs link fragment `#payload-exclusions-stagehandignore` is renamed on both ends; `go build ./...
&& go test ./...` green & unchanged; `git status` shows ONLY the 5 docs files.

## User Persona

**Target User**: The user/integrator reading the reference docs (docs/cli.md = the CLI reference, etc.) and
the docs index (docs/README.md). Post-rename every command, env var, git-config key, path, and example must
say `stagecoach` and actually work against the renamed binary + config surfaces.

**Use Case**: A user reads docs/cli.md, copies `stagecoach --provider pi …` + `STAGECOACH_PROVIDER=pi`, sets
`git config stagecoach.provider pi`, creates `~/.config/stagecoach/config.toml` + `.stagecoachignore` — every
name must agree with the shipped `stagecoach` binary and the renamed Go constants.

**User Journey**: docs/README.md index → cli.md (flags/env/git-config) → configuration.md (paths/precedence)
→ how-it-works.md (architecture) → providers.md (manifests). No stale "stagehand" anywhere; no mismatched
env var / config key / path; the lazygit example's marker matches the shipped Go constant.

**Pain Points Addressed**: A half-renamed docs tree (stagecoach prose but `STAGEHAND_*` env vars /
`stagehand.*` git keys / `dustin/stagehand` install paths) would confuse users and break copy-pasted
commands against the renamed binary. This task makes the reference surface whole.

## Why

- **It IS Layer 5.2 of the project rename.** M1–M3 renamed the code, config, and build/CI; the docs/*.md
  tree is the reference surface still saying "stagehand." rename_surface_map §5.2 names it explicitly.
- **The docs are the authoritative reference.** docs/cli.md is THE CLI reference (every flag/env/git-config
  key); docs/configuration.md is THE config model. A stale name here directly misleads users.
- **Cross-surface correctness.** The lazygit marker in docs/cli.md MUST match the Go `lazygitMarker` or
  `stagecoach integrate uninstall` breaks; the go-install path MUST match go.mod or installs fail.
- **Trivial, isolated, no-risk.** Five markdown files; global sed; no code, no tests, no other files.

## What

A global case-variant sed over the five docs/*.md files, followed by a verification pass (the five contract
criteria + cross-surface consistency + anchor safety). No code, no tests, no other files.

### Success Criteria

- [ ] `sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g'
      docs/cli.md docs/configuration.md docs/how-it-works.md docs/providers.md docs/README.md` applied.
- [ ] `grep -ri stagehand docs/*.md` → ZERO hits (the Layer-5 gate).
- [ ] (a) All env-var references are `STAGECOACH_*` (zero `STAGEHAND_`).
- [ ] (b) All git-config keys are `stagecoach.*` (zero `stagehand.`).
- [ ] (c) Config paths are `~/.config/stagecoach/config.toml` + `.stagecoach.toml` (zero `stagehand` paths).
- [ ] (d) The exclusion file is `.stagecoachignore` (zero `.stagehandignore`).
- [ ] (e) The CLI binary/examples are `stagecoach …` (zero `stagehand` as the command).
- [ ] The docs/cli.md lazygit marker is `# stagecoach-integration` AND equals the Go `lazygitMarker`
      (internal/cmd/integrate_lazygit.go:20 = `"stagecoach-integration"`).
- [ ] The docs/README.md go-install path is `github.com/dustin/stagecoach/cmd/stagecoach` AND matches
      go.mod (`module github.com/dustin/stagecoach`).
- [ ] ZERO `dabstractor` in docs; all GitHub-namespace refs are `dustin/stagecoach`.
- [ ] No docs heading still contains "stagehand" (`grep -rniE '^#{1,6} .*stagehand' docs/` → 0); the known
      intra-docs link fragment `#payload-exclusions-stagecoachignore` exists on BOTH the heading and the link.
- [ ] `git status` shows ONLY the 5 docs files modified; NO other file touched; `go build ./... &&
      go test ./...` GREEN & unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A technical writer with no prior repo knowledge can implement this from: the verbatim sed command +
the exact 5-file list, the per-file occurrence counts, the §1 "no exceptions to preserve" finding, the §2
"review = verification, not edits" framing, the §3 cross-surface constants (marker, go-install, exclusion
file — each with its Go source of truth), the §5 anchor-safety insight, and the LEAVE list. No Go/hooks/
generate knowledge required — this is a global text replacement + verification.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE design decisions
- docfile: plan/012_963e3918ec08/P1M4T1S2/research/design-decisions.md
  why: the 7 decisions. §0 (scope: 5 files, ~526 occs, per-file counts; docs/README.md is the INDEX, not the
       repo-root README), §1 (sed is complete + safe — no provenance notes, no commit-pi, no substring traps;
       the lone "originally" is the snapshot recipe, not rename-history), §2 (review items (a)-(e) are
       VERIFICATION not edits), §3 (cross-surface: marker == Go lazygitMarker, go-install == go.mod,
       exclusion == Go StagecoachIgnoreFile), §4 (namespace dustin NOT dabstractor; docs have 2 GitHub URLs),
       §5 (ANCHOR SAFETY — the sed renames heading+fragment consistently in one pass; the known
       #payload-exclusions-stagehandignore pair), §6 (coordination with S1 + downstream), §7 (Mode A validation).
  critical: §5 (the sed is SAFE for intra-docs anchor links because it renames all 5 files in one pass —
       heading text → slug AND link fragment both change; don't "protect" anchors), §3 (the marker + go-install
       + exclusion file must match the renamed Go code), §1 (no exceptions — global sed, grep must be 0).

# MUST READ — the parallel sibling (P1.M4.T1.S1, repo-root README) contract
- docfile: plan/012_963e3918ec08/P1M4T1S1/PRP.md
  section: the identical 3-branch sed; the namespace decision (`dustin` NOT `dabstractor`); the cross-surface
           checks (lazygit marker == Go constant; go-install path == go.mod); the zero-hits gate.
  why: S1 (repo-root README.md) and S2 (docs/*.md) edit DISJOINT files — no merge conflict. Both use the
       identical sed, so any cross-tree link (docs/README.md → ../README.md) is consistent on both ends.
       S1's gotchas (namespace trap, marker contract, anchor safety) apply identically here.
  critical: do NOT touch the repo-root README.md (S1's scope). docs/README.md is a DIFFERENT file (the docs index).

# MUST READ — the surface-map research (the rename plan for this layer)
- docfile: plan/012_963e3918ec08/architecture/rename_surface_map.md
  section: §5.2 docs/ (the 5 files) + the Layer-5 verification gate ("grep -ri stagehand returns ZERO hits").
  why: confirms (a) docs/*.md is Layer 5.2 and the 5-file list, (b) the zero-hits gate is the acceptance check.

# MUST READ — the files being edited (the surface to rename)
- file: docs/cli.md   (EDIT; ~32KB, 238 occs: 182 lower + 4 Title + 52 UPPER)
  section: the full CLI reference — every flag table row (STAGEHAND_* env, stagehand.* git-config), the lazygit
           integration example (L299 `# stagehand-integration` marker, L318/L327 "stagehand entry"), the hook/
           config/models subcommand examples (`stagehand …`), the config-path comments (~/.config/stagehand/…).
  why: the LARGEST rename surface (238 occs) + the cross-surface marker contract (the lazygit example MUST
       match the Go `lazygitMarker`).
  critical: the 3 `stagehand-integration` refs (L299/318/327) → `stagecoach-integration` == Go constant.
- file: docs/configuration.md   (EDIT; ~24KB, 208 occs: 145 lower + 4 Title + 59 UPPER)
  section: the config model — 7-layer precedence (stagecoach_* env, stagehand.* git-config, ~/.config/stagehand/
           paths, .stagehand.toml), the `### .stagehandignore` heading (L253), the full config-file example.
  why: the config-reference surface; the most env-var (59 UPPER) + git-config + path references.
  critical: the `### .stagehandignore` heading (L253) → `### .stagecoachignore` (anchor changes; verify consistency).
- file: docs/how-it-works.md   (EDIT; ~34KB, 53 occs: 40 lower + 13 Title)
  section: the architecture explanation — `# How Stagehand works` (L1 title), `### Payload exclusions
           (.stagehandignore)` (L156 heading), the snapshot/rescue recipe (L212 "originally staged files").
  why: prose-heavy; the heading slugs that other docs link to (#payload-exclusions-stagehandignore).
  critical: the "originally staged files" (L212) is the SNAPSHOT recipe, NOT a rename-history note — leave it
           renamed to "stagecoach" by the sed (it becomes "originally staged files" unchanged in meaning; the
           word "stagehand" elsewhere on the line gets renamed). Verify no provenance note was mangled.
- file: docs/providers.md   (EDIT; ~16KB, 10 occs: 7 lower + 3 Title)
  section: provider documentation — the manifest schema, the 8 built-in providers.
  why: the smallest surface; straightforward.
- file: docs/README.md   (EDIT; ~4KB, the docs INDEX, 17 occs: 13 lower + 3 Title + 1 UPPER)
  section: `# Stagehand documentation` (L1 title); the docs index links (cli.md/configuration.md/providers.md/
           how-it-works.md + ../README.md + ../PRD.md + ../FUTURE_SPEC.md); the go-install line (L17) +
           curl|sh (L20); the capability anchors (L42 `#payload-exclusions-stagehandignore`).
  why: the docs entry point; the go-install path (L17) MUST match go.mod; the anchor fragment (L42) MUST match
       the how-it-works.md heading (renamed in the same sed pass).
  critical: the go-install path `github.com/dustin/stagecoach/cmd/stagecoach` == go.mod; namespace `dustin`.

# MUST READ — the Go constants the docs must MATCH (already renamed M1–M3; do NOT edit these)
- file: internal/cmd/integrate_lazygit.go   (READ ONLY)
  section: L20 `lazygitMarker = "stagecoach-integration"`.
  why: docs/cli.md's `# stagecoach-integration` marker (post-sed) MUST equal this. VERIFY post-sed.
- file: internal/exclude/exclude.go   (READ ONLY)
  section: L28 `const StagecoachIgnoreFile = ".stagecoachignore"`.
  why: docs' `.stagecoachignore` references (post-sed) MUST equal this. VERIFY post-sed.
- file: go.mod   (READ ONLY)
  section: L1 `module github.com/dustin/stagecoach`.
  why: docs/README.md's go-install path `github.com/dustin/stagecoach/cmd/stagecoach` MUST match this module
       path + the renamed `cmd/stagecoach` dir. VERIFY post-sed.

# Confirms the namespace decision (badge/brew/scoop/clone all use dustin/stagecoach)
- file: .goreleaser.yaml   (READ ONLY)
  section: `owner: dustin` (overrides the dabstractor git remote).
  why: the canonical public namespace is `dustin/stagecoach`. docs/README.md's 2 GitHub URLs use `dustin`;
       the sed yields `dustin/stagecoach`. Do NOT introduce dabstractor.

- url: (PRD §15.2 Global flags, §16.1 Resolution order, §9.8 FR34–FR38, §9.15 FR-R1–R6 — in context as
       selected_prd_content `h3.72`/`h3.76`/`h3.24`/`h3.31`; the AUTHORITATIVE post-rename values: STAGECOACH_*
       env, stagecoach.* git-config, ~/.config/stagecoach/config.toml, .stagecoach.toml, the `stagecoach` binary.)
  why: the PRD is already renamed to stagecoach (h2.30) — these sections are the source-of-truth values the
       docs must reproduce. Cross-check a few docs claims against them post-sed.
```

### Current Codebase tree (relevant slice)

```bash
docs/
  cli.md              # *** EDIT *** — ~238 occs. CLI reference + the lazygit integration marker.
  configuration.md    # *** EDIT *** — ~208 occs. Config model + the .stagehandignore heading.
  how-it-works.md     # *** EDIT *** — ~53 occs. Architecture + the payload-exclusions heading (anchor target).
  providers.md        # *** EDIT *** — ~10 occs. Provider docs.
  README.md           # *** EDIT *** — ~17 occs. Docs INDEX + go-install path + the anchor link fragment.
README.md             # NOT this task (repo-root README — P1.M4.T1.S1's scope, parallel).
go.mod                # READ ONLY — `module github.com/dustin/stagecoach` (the go-install source of truth).
.goreleaser.yaml      # READ ONLY — `owner: dustin` (the namespace source of truth).
internal/cmd/integrate_lazygit.go  # READ ONLY — `lazygitMarker = "stagecoach-integration"` (the marker contract).
internal/exclude/exclude.go         # READ ONLY — `StagecoachIgnoreFile = ".stagecoachignore"` (the exclusion contract).
providers/*.toml      # NOT this task (P1.M4.T2.S1).
FUTURE_SPEC.md        # NOT this task (P1.M4.T2.S2).
# NO .go / Makefile / go.mod / .goreleaser / CI edits. Mode A (docs only).
```

### Desired Codebase tree with files to be added/changed

```bash
# NO new files. FIVE in-place edits: docs/{cli,configuration,how-it-works,providers,README}.md (global sed + verification).
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (the sed renames heading + link fragment consistently — anchor safety, design §5): several docs
     HEADINGS contain "stagehand" (how-it-works.md `### Payload exclusions (.stagehandignore)`,
     configuration.md `### .stagehandignore`, the L1 titles), so their GitHub anchor slugs contain it. Other
     docs LINK to those slugs (docs/README.md:42 → #payload-exclusions-stagehandignore). Because the sed runs
     over ALL 5 docs files in ONE pass, it renames BOTH the heading (→ slug) AND the link fragment ⇒ every
     intra-docs link stays valid. Do NOT "protect" anchors or hand-edit them — the global sed is correct.
     VERIFY post-sed: `grep -rniE '^#{1,6} .*stagehand' docs/` → 0 (no heading still contains stagehand). -->

<!-- CRITICAL (the lazygit marker is a cross-surface contract, design §3): docs/cli.md has 3 refs to
     `# stagehand-integration` (L299/318/327) → sed → `# stagecoach-integration`. That string MUST equal the
     Go constant `lazygitMarker` (integrate_lazygit.go:20 = "stagecoach-integration"). The sed produces it;
     VERIFY they match — a mismatch breaks `stagecoach integrate uninstall`'s idempotency lookup. -->

<!-- CRITICAL (the go-install path must match go.mod, design §3): docs/README.md:17 → sed →
     `go install github.com/dustin/stagecoach/cmd/stagecoach@latest`. VERIFY module path == go.mod L1
     (`github.com/dustin/stagecoach`) + cmd dir is `cmd/stagecoach` (renamed M1.T1.S2). -->

<!-- CRITICAL (namespace is dustin, NOT dabstractor, design §4): docs/README.md's 2 GitHub URLs use
     `dustin/stagehand` → sed → `dustin/stagecoach`. The canonical namespace is `dustin` (go.mod,
     .goreleaser owner: dustin, PRD §21.3). docs contain ZERO dabstractor — do NOT introduce it. -->

<!-- GOTCHA (docs/README.md is the INDEX, not the repo-root README): docs/README.md is a DIFFERENT file from
     the repo-root README.md (P1.M4.T1.S1's target). Both are renamed; neither overlaps. docs/README.md links
     to ../README.md (the repo root) — S1 renames that file's content; the link's filename/anchor are
     stagehand-free ⇒ resolves unchanged. -->

<!-- GOTCHA (the "originally staged files" line is NOT rename-history, design §1): how-it-works.md:212
     "To commit the originally staged files manually:" is the snapshot/rescue recipe, NOT a provenance note.
     The sed renames any "stagehand" on that line (the recipe's "stagehand" refs) but "originally staged files"
     is unaffected in meaning. There is NO "formerly/originally stagehand" note to preserve in any docs file
     (that note is PRD h2.30, read-only). -->

<!-- GOTCHA (case-variant sed order is safe): s/stagehand/stagecoach/g, then s/Stagehand/Stagecoach/g, then
     s/STAGEHAND/STAGECOACH/g — the three patterns are case-disjoint, no double-substitution. Grep confirmed
     ZERO non-canonical variants (stageHand/StageHand) in docs. -->

<!-- GOTCHA (run from the repo root): run the sed from the module dir (/home/dustin/projects/stagehand) so the
     docs/ paths resolve. The plan/ artifacts dir is a different tree (P1.M5.T1.S1's scope) — do NOT sed it. -->

<!-- GOTCHA (filenames don't change): the docs filenames (cli.md, configuration.md, etc.) contain NO "stagehand",
     so the sed leaves them and all `](cli.md)` / `](configuration.md)` links resolve unchanged. Only anchor
     FRAGMENTS containing "stagehand" change (and consistently — see the first gotcha). -->
```

## Implementation Blueprint

### Data models and structure

```markdown
<!-- NO data models. A global-text-replace + verification task across 5 markdown files. The "structure" is
     the sed + the verification checklist. -->
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: APPLY the global case-variant sed to the 5 docs files
  - RUN (from the repo root / module dir):
      sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g' \
        docs/cli.md docs/configuration.md docs/how-it-works.md docs/providers.md docs/README.md
  - This transforms ~526 occurrences (env vars, git-config keys, paths, the exclusion file, the binary, the
      integration marker, the go-install path, prose) in one pass per file.
  - GOTCHA: list EXACTLY these 5 files (or `docs/*.md`, which matches exactly those 5). Do NOT include the
      repo-root README.md (S1) or any non-docs file. Case variants are disjoint — no double-substitution.

Task 2: VERIFY the contract's five criteria (a)–(e) — all match the renamed Go code
  - (a) ENV VARS: `grep -rnE 'STAGEHAND_' docs/` → expect ZERO hits. `grep -rnE 'STAGECOACH_' docs/` | wc -l`
      → expect many (was 52+59=111 UPPER, now all STAGECOACH_).
  - (b) GIT CONFIG: `grep -rnE 'stagehand\.' docs/` → expect ZERO. `grep -rn 'stagecoach\.' docs/` | wc -l`
      → expect many (every stagehand.provider/.model/.timeout/etc.).
  - (c) CONFIG PATHS: `grep -rn '~/.config/stagecoach/config.toml' docs/` → hits; `grep -rn '\.stagecoach\.toml'
      docs/` → hits. `grep -rnE 'stagehand.*config\.toml|\.stagehand\.toml' docs/` → ZERO.
  - (d) EXCLUSION FILE: `grep -rn '\.stagecoachignore' docs/` → hits; `grep -rn '\.stagehandignore' docs/`
      → ZERO.
  - (e) BINARY: `grep -rnE '\bstagehand\b' docs/` → ZERO (the command is now `stagecoach`).
  - GOTCHA: these are VERIFICATION greps (the sed already produced every target). A non-zero hit on a
      "expect ZERO" grep means a missed case variant or a stray — inspect + fix.

Task 3: VERIFY cross-surface consistency (marker + go-install + exclusion match the renamed Go code)
  - MARKER: `grep -n '# stagecoach-integration' docs/cli.md` → hits (L299/318/327 post-sed). Then confirm it
      equals the Go constant: `grep -n 'lazygitMarker = "stagecoach-integration"' internal/cmd/integrate_lazygit.go`
      → hit (L20). They MUST match.
  - GO-INSTALL: `grep -n 'go install github.com/dustin/stagecoach/cmd/stagecoach' docs/README.md` → hit (L17).
      Then confirm it matches go.mod: `head -1 go.mod` → `module github.com/dustin/stagecoach`. They MUST match.
  - EXCLUSION: `grep -n '\.stagecoachignore' docs/` → hits; confirm the Go constant:
      `grep -n 'StagecoachIgnoreFile = ".stagecoachignore"' internal/exclude/exclude.go` → hit (L28). Match.
  - NAMESPACE: `grep -rnE 'dabstractor' docs/` → ZERO. `grep -rnE 'github.com/dustin/stagecoach' docs/`
      → hits (docs/README.md L17/L20). Owner is `dustin`.

Task 4: VERIFY anchor-link safety (no broken intra-docs links)
  - NO HEADING STILL CONTAINS stagehand: `grep -rniE '^#{1,6} .*stagehand' docs/` → ZERO.
  - THE KNOWN ANCHOR PAIR renamed consistently: the how-it-works.md heading is now
      `### Payload exclusions (.stagecoachignore)` (slug `#payload-exclusions-stagecoachignore`) AND the
      docs/README.md link is now `](how-it-works.md#payload-exclusions-stagecoachignore)`. VERIFY both ends:
      `grep -rn 'payload-exclusions-stagecoachignore' docs/` → 2 hits (heading-derived slug context + the link).
      `grep -rn 'payload-exclusions-stagehandignore' docs/` → ZERO (no stale fragment).
  - (Optional) scan for any remaining stagehand anchor fragment: `grep -rnE '#[a-z0-9-]*stagehand' docs/`
      → ZERO.

Task 5: VERIFY the zero-hits gate + scope + regression no-op
  - ZERO-HITS: `grep -ri stagehand docs/*.md` → MUST return ZERO hits (the Layer-5 gate). If any hit remains,
      inspect (a missed case variant) and fix.
  - SCOPE: `git status --porcelain` → EXACTLY the 5 docs files modified (docs/cli.md, configuration.md,
      how-it-works.md, providers.md, README.md). NOTHING else.
      `git diff --exit-code README.md providers/ FUTURE_SPEC.md .github/workflows/ .gitignore go.mod
      .goreleaser.yaml Makefile` → all unchanged. `! git diff --name-only | grep -vE '^docs/' ` → no non-docs edit.
  - REGRESSION NO-OP: `go build ./... && go test ./...` → GREEN & unchanged (no code touched; this is a
      docs-only edit — the build/test confirms no .go file was accidentally modified).
```

### Implementation Patterns & Key Details

```markdown
<!-- THE sed (one global pass, three disjoint case variants, over exactly the 5 docs files):
sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g' \
  docs/cli.md docs/configuration.md docs/how-it-works.md docs/providers.md docs/README.md

<!-- THE anchor-safety principle (the key insight): the sed renames heading text (→ GitHub slug) AND link
     fragments in ONE pass across all 5 docs files ⇒ intra-docs links stay valid. Do NOT hand-edit anchors.

<!-- THE cross-surface consistency checks (docs must MATCH the renamed Go code, not re-rename it):
     - docs/cli.md `# stagecoach-integration` == Go lazygitMarker (integrate_lazygit.go:20).
     - docs/README.md go-install path == go.mod (`github.com/dustin/stagecoach`) + cmd/stagecoach.
     - docs `.stagecoachignore` == Go StagecoachIgnoreFile (exclude.go:28).
     - namespace `dustin` (NOT dabstractor) == .goreleaser owner: dustin.

<!-- THE zero-hits gate: `grep -ri stagehand docs/*.md` → 0. This is the Layer-5 acceptance check and the
     input P1.M5.T2.S1's final audit expects. -->

<!-- THE "no exceptions" finding: docs are a clean reference surface — NO provenance/rename-history notes
     (the lone "originally" is the snapshot recipe), NO commit-pi refs, NO substring traps. Global sed, grep must be 0. -->
```

### Integration Points

```yaml
DOCS.FILES (the ONLY edits): docs/{cli,configuration,how-it-works,providers,README}.md (global sed + verification).

LEFT-UNCHANGED (do NOT edit — other layers' scope):
  - README.md              # repo-root README — P1.M4.T1.S1 (Layer 5.1, parallel).
  - providers/*.toml       # P1.M4.T2.S1 (Layer 5.3).
  - FUTURE_SPEC.md         # P1.M4.T2.S2 (Layer 5.4).
  - plan/ artifacts        # P1.M5.T1.S1 (historical).
  - .github/workflows/ci.yml, .gitignore  # P1.M3.T1.S3 (Layer 4.3-4.4, parallel/done).
  - go.mod, .goreleaser.yaml, Makefile, every .go file  # already renamed (M1-M3).

CODE.LEFT-UNCHANGED: NO .go / Makefile / go.mod / .goreleaser / CI changes (Mode A — docs only). The build/test
  is a no-op regression check confirming no .go file was accidentally edited.

CROSS-SURFACE CONSISTENCY (the docs must AGREE with the already-renamed code):
  - docs/cli.md lazygit marker `# stagecoach-integration` == Go `lazygitMarker` (integrate_lazygit.go:20).
  - docs/README.md go-install path == go.mod (`github.com/dustin/stagecoach`) + `cmd/stagecoach`.
  - docs `.stagecoachignore` == Go `StagecoachIgnoreFile` (exclude.go:28).
  - docs namespace `dustin` == .goreleaser `owner: dustin` (NOT dabstractor).

COORDINATION (parallel sibling): P1.M4.T1.S1 (repo-root README.md) edits a DISJOINT file; both use the
  identical sed ⇒ cross-tree links (docs/README.md → ../README.md) are consistent on both ends.

DOWNSTREAM: P1.M5.T2.S1 (final grep audit) sweeps ALL tracked files for zero stagehand — this task lands
  docs' zero-hits ahead of that audit. The audit's gate: `grep -ri stagehand --include='*.md' ...` == 0.
```

## Validation Loop

### Level 1: The sed landed + zero-hits gate

```bash
cd /home/dustin/projects/stagehand   # the repo root / module dir
sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g' \
  docs/cli.md docs/configuration.md docs/how-it-works.md docs/providers.md docs/README.md
grep -ric stagehand docs/*.md   # EXPECT: 0 0 0 0 0  (the Layer-5 gate — every file zero)
grep -rnE 'dabstractor' docs/   # EXPECT: no match (namespace is dustin)
head -1 docs/README.md          # EXPECT: "# Stagecoach documentation"
head -1 docs/how-it-works.md    # EXPECT: "# How Stagecoach works"
# Expected: 0 stagehand hits across all 5 docs files; 0 dabstractor; titles are Stagecoach.
```

### Level 2: The five contract criteria (env / git-config / paths / exclusion / binary)

```bash
cd /home/dustin/projects/stagehand
# (a) env vars:
grep -rnE 'STAGEHAND_' docs/ && echo "FAIL: stale STAGEHAND_" || echo "OK: zero STAGEHAND_"
grep -rncE 'STAGECOACH_' docs/*.md   # EXPECT: many (was 52+59=111 UPPER total)
# (b) git config:
grep -rnE 'stagehand\.' docs/ && echo "FAIL: stale stagehand.*" || echo "OK: zero stagehand.*"
# (c) config paths:
grep -rnE 'stagehand.*config\.toml|\.stagehand\.toml' docs/ && echo "FAIL: stale path" || echo "OK: zero stale paths"
grep -rn '~/.config/stagecoach/config.toml' docs/ && grep -rn '\.stagecoach\.toml' docs/ | head
# (d) exclusion file:
grep -rn '\.stagehandignore' docs/ && echo "FAIL: stale .stagehandignore" || echo "OK: zero .stagehandignore"
grep -rn '\.stagecoachignore' docs/ | head
# (e) binary:
grep -rnE '\bstagehand\b' docs/ && echo "FAIL: stale stagehand command" || echo "OK: zero stagehand command"
# Expected: every "expect ZERO" grep is empty; every positive grep hits.
```

### Level 3: Cross-surface consistency + anchor safety

```bash
cd /home/dustin/projects/stagehand
# Marker matches the Go constant:
grep -n '# stagecoach-integration' docs/cli.md                              # → L299/318/327 hits
grep -n 'lazygitMarker = "stagecoach-integration"' internal/cmd/integrate_lazygit.go  # → L20 (read-only confirm)
# go-install path matches go.mod:
grep -n 'go install github.com/dustin/stagecoach/cmd/stagecoach' docs/README.md  # → L17 hit
test "$(head -1 go.mod)" = "module github.com/dustin/stagecoach" && echo "OK: go.mod matches the docs go-install path"
# Exclusion matches the Go constant:
grep -n 'StagecoachIgnoreFile = ".stagecoachignore"' internal/exclude/exclude.go  # → L28 (read-only confirm)
# Anchor safety — no heading still contains stagehand; the known pair renamed consistently:
grep -rniE '^#{1,6} .*stagehand' docs/ && echo "FAIL: heading still has stagehand" || echo "OK: no stagehand heading"
grep -rn 'payload-exclusions-stagecoachignore' docs/  # → 2 hits (heading-derived + the docs/README.md link)
grep -rn 'payload-exclusions-stagehandignore' docs/ && echo "FAIL: stale anchor fragment" || echo "OK: no stale fragment"
# Expected: the marker/go-install/exclusion match the Go code; zero stagehand headings; the anchor pair is consistent.
```

### Level 4: Scope check + regression no-op (no code touched)

```bash
cd /home/dustin/projects/stagehand
go build ./...   # Expect clean & unchanged (no code touched).
go test ./...    # Expect GREEN & unchanged (docs-only).
git status --porcelain
# Expected: EXACTLY 5 modified files — docs/cli.md, docs/configuration.md, docs/how-it-works.md,
#           docs/providers.md, docs/README.md. NOTHING else.
git diff --exit-code README.md providers/ FUTURE_SPEC.md .github/workflows/ .gitignore go.mod .goreleaser.yaml Makefile \
  && echo "all non-docs files UNCHANGED (expected)"
! git diff --name-only | grep -vE '^docs/' && echo "OK: only docs/ files modified"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean & unchanged; `go test ./...` GREEN & unchanged (docs-only — no code touched).
- [ ] `git status` shows EXACTLY 5 modified files in docs/. No other file touched.
- [ ] go.mod / .goreleaser.yaml / Makefile / .github/ / README.md / providers/ / FUTURE_SPEC.md byte-unchanged.

### Feature Validation
- [ ] `grep -ri stagehand docs/*.md` → ZERO hits across all 5 files (the Layer-5 gate).
- [ ] (a) env vars `STAGECOACH_*` (zero `STAGEHAND_`); (b) git-config `stagecoach.*` (zero `stagehand.`);
      (c) paths `~/.config/stagecoach/config.toml` + `.stagecoach.toml`; (d) `.stagecoachignore`;
      (e) binary `stagecoach`.
- [ ] docs/cli.md lazygit marker `# stagecoach-integration` == Go `lazygitMarker` (integrate_lazygit.go:20).
- [ ] docs/README.md go-install path `github.com/dustin/stagecoach/cmd/stagecoach` == go.mod + `cmd/stagecoach`.
- [ ] ZERO `dabstractor` in docs; all GitHub-namespace refs are `dustin/stagecoach`.
- [ ] No docs heading contains stagehand; the `#payload-exclusions-stagecoachignore` anchor pair is consistent.

### Code Quality Validation
- [ ] The rename is global + consistent across all 5 files (no stragglers, no mixed case variants left).
- [ ] No intra-docs anchor links broken (heading + fragment renamed in one pass).
- [ ] Anti-patterns avoided (see below); no out-of-scope churn (repo-root README / providers / FUTURE_SPEC / code frozen).

### Documentation
- [ ] [Mode A] docs/*.md present a unified stagecoach identity; every command/env-var/git-config/path/marker
      matches the shipped `stagecoach` binary + the renamed Go constants. (This IS the docs update for the
      project's reference surface.)

---

## Anti-Patterns to Avoid

- ❌ **Don't touch any file outside docs/*.md.** The repo-root README.md is P1.M4.T1.S1's scope (parallel);
  providers/*.toml is P1.M4.T2.S1; FUTURE_SPEC.md is P1.M4.T2.S2; plan/ is P1.M5.T1.S1; code/build is already
  done (M1-M3). `git status` must show exactly the 5 docs files. (scope boundary)
- ❌ **Don't hand-edit anchor links.** The sed renames heading text (→ GitHub slug) AND link fragments in ONE
  pass across all 5 docs files ⇒ intra-docs links stay valid. "Protecting" anchors would DESYNC them (a
  renamed heading slug vs. an un-renamed link fragment). Let the global sed handle both ends. (§5)
- ❌ **Don't mismatch the lazygit marker.** docs/cli.md's `# stagecoach-integration` (post-sed) MUST equal the
  Go `lazygitMarker` (integrate_lazygit.go:20). A mismatch breaks `stagecoach integrate uninstall`'s idempotency
  check. The sed produces it; VERIFY they match. (§3)
- ❌ **Don't mismatch the go-install path.** docs/README.md's path must be `github.com/dustin/stagecoach/cmd/
  stagecoach` — matching go.mod's module path AND the renamed `cmd/stagecoach` dir. Verify post-sed; don't
  leave a stale `cmd/stagehand` segment. (§3)
- ❌ **Don't use `dabstractor`.** The namespace is `dustin` (go.mod, .goreleaser `owner: dustin`, PRD §21.3).
  docs/README.md's 2 GitHub URLs use `dustin/stagehand`; the sed yields `dustin/stagecoach`. Do NOT introduce
  dabstractor. (§4)
- ❌ **Don't preserve a "formerly stagehand" note.** There ISN'T one in docs (the lone "originally" is the
  snapshot recipe at how-it-works.md:212; the rename-history note is PRD h2.30, read-only). Every "stagehand"
  → "stagecoach" — global sed, no exceptions. (§1)
- ❌ **Don't run the sed from the wrong directory.** Run from the repo root (the module dir) so the docs/
  paths resolve; the plan/ artifacts dir is a different tree (P1.M5.T1.S1's scope). (gotcha)
- ❌ **Don't list the wrong files.** The sed targets EXACTLY docs/cli.md, configuration.md, how-it-works.md,
  providers.md, README.md (or `docs/*.md`, which matches exactly those 5). Do NOT include the repo-root
  README.md. (gotcha)
- ❌ **Don't skip the zero-hits gate.** `grep -ri stagehand docs/*.md` MUST be 0 — that's the Layer-5
  acceptance check and the input P1.M5.T2.S1's final audit expects. (Task 5)

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: this is a mechanical global-sed rename across 5 markdown files (Mode A), fully isomorphic to the
parallel P1.M4.T1.S1 (repo-root README) which uses the identical sed + verification pattern. The exact sed
command, the 5-file list, and the per-file occurrence counts (~526 total) are quoted from the live tree; the
five contract review criteria are each pinned to a deterministic grep; and the three cross-surface contracts
(the lazygit marker == Go `lazygitMarker`, the go-install path == go.mod, the exclusion file == Go
`StagecoachIgnoreFile`) are each verified against the already-renamed Go code (M1-M3 LANDED — confirmed by
direct grep of integrate_lazygit.go:20, exclude.go:28, go.mod L1). The one non-obvious risk — anchor-link
breakage from headings whose slugs contain "stagehand" — is resolved by the §5 insight that the sed renames
heading + fragment consistently in a single pass over all 5 files (verified: the
`#payload-exclusions-stagehandignore` pair exists on both ends and both contain "stagehand"). The namespace
is `dustin` (not the dabstractor trap; docs have zero dabstractor and 2 `dustin/stagehand` URLs). There are
no provenance/commit-pi exceptions to preserve (verified). Validation is deterministic: `grep -ri stagehand
docs/*.md` == 0, the cross-surface greps match, `git status` == 5 docs files, `go build/test` is a no-op.
Coordination with the parallel S1 is conflict-free (disjoint files, identical sed).
