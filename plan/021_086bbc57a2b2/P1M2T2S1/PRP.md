name: "P1.M2.T2.S1 — README + overview coherence sweep across the +body delta (Mode B changeset docs)"
description: >
  Pure Mode-B markdown edits to THREE files, finishing the +body changeset. Sync the 4 overview-doc
  format-base enumerations/blurb with the ALREADY-SHIPPED `<base>[+body]` grammar (P1.M1.T1.S4 Complete;
  surfaced in docs/cli.md + docs/configuration.md by the parallel P1.M2.T1.S2). The 4 sites: README.md L63
  (Message shaping features row) + L196 (the `--format conventional` example), docs/how-it-works.md (the
  "Format modes and locale" section — add a dedicated `+body` paragraph mirroring PRD §17.8/FR-F9), and
  docs/index.md L58 (Features bullet — a NEW find from the sweep, not in the contract's named list). NO code,
  NO tests (P1.M1 + parallel P1.M2.T1.S1), NO docs/cli.md or docs/configuration.md (P1.M2.T1.S2's domain,
  already done), NO PRD/spec. The delta ships with zero stale format references across every user-facing
  overview surface.

---

## Goal

**Feature Goal**: Make every user-facing *overview* doc that enumerates the `--format` bases (`auto |
conventional | gitmoji | plain`) state the `+body` modifier (FR-F9), so a reader of the README, the
docs landing page, or the format-modes explainer discovers that any base optionally takes `+body` to force a
subject-plus-body — matching what `stagecoach --help`, `config init`, docs/cli.md, and docs/configuration.md
already print. Today these 4 overview surfaces lag the binary (they list only the 4 bases).

**Deliverable** (3 modified markdown files, no new files):
1. **README.md** — L63 Message shaping row gains the `+body` clause; L196 gains a companion `conventional+body` example.
2. **docs/how-it-works.md** — the "Format modes and locale" section gains a dedicated **Body forcing (`+body`)** paragraph (mirroring PRD §17.8).
3. **docs/index.md** — L58 Features bullet gains the `+body` clause.

**Success Definition**:
- README.md L63, docs/index.md L58 each mention `+body` in their format line.
- docs/how-it-works.md's "Format modes and locale" section documents `+body` (a dedicated paragraph), faithful to PRD §17.8/FR-F9.
- README.md shows a `conventional+body` example (L196 or its companion line).
- The 4 P1.M2.T1.S2 surfaces (docs/cli.md L38, docs/configuration.md L116/216/247) are UNCHANGED and already pass.
- No stale format-base enumeration remains anywhere in README.md / docs/*.md (the sweep is clean).
- `make test` green + `go vet ./...` clean (a docs edit cannot break Go tests — confirms the tree is otherwise clean); `git diff --name-only` == exactly {README.md, docs/how-it-works.md, docs/index.md}.

## User Persona (if applicable)

**Target User**: A developer skimming the README or docs landing page to pick a `--format` value — specifically one who wants a commit body and is looking for how to force one (US28).

**Use Case**: "I want conventional-commit subjects WITH a body, even though my repo's history is single-line." → the README features row / docs index / how-it-works explainer now all surface `+body`, so the user discovers `stagecoach --format conventional+body` without having to find it in `--help` or the PRD.

**User Journey**: README L63 (Message shaping) → sees "append +body to force a subject+body" → README L196 example shows `conventional+body` → clicks the docs link → docs/how-it-works "Format modes and locale" → reads the dedicated `+body` paragraph → runs `stagecoach --format conventional+body` → subject + body lands.

**Pain Points Addressed**: the overview docs currently describe only the 4 bases, so a README/docs reader never discovers `+body` (FR-F9) — they'd have to find it in `--help` or the PRD. This closes that gap across the marketing/overview surfaces.

## Why

- **Mode B (changeset-level docs coherence)**: P1.M1.T1.S4 shipped the canonical `<base>[+body]` wording in root.go/config.go/bootstrap.go; P1.M2.T1.S2 surfaced it in the CLI + config reference docs. This task is the final sweep across the README + overview explainer + landing page so the WHOLE delta is coherent — no surface disagrees with `stagecoach --help`.
- **FR-F9 / §9.19 / §17.8**: `+body` is a shipped feature; the overview surfaces are the first place a user looks, so each must state the grammar.
- **Bounded scope**: 4 localized edits across 3 markdown files. No code, no tests, no schema. The implementation (P1.M1) and the reference-doc sync (parallel P1.M2.T1.S2) are separate; this is the overview/marketing coherence pass.

## What

**User-visible behavior**: the README, docs landing page, and format-modes explainer now describe `<base>[+body]`. No runtime change.

**Technical change** (4 edits across 3 markdown files; verbatim target text in the Blueprint).

### Success Criteria
- [ ] README.md L63 Message shaping row mentions `+body`
- [ ] README.md shows a `conventional+body` example (L196 companion line)
- [ ] docs/how-it-works.md "Format modes and locale" section has a dedicated `+body` paragraph (faithful to §17.8)
- [ ] docs/index.md L58 Features bullet mentions `+body`
- [ ] docs/cli.md L38 + docs/configuration.md L116/216/247 UNCHANGED (P1.M2.T1.S2's domain)
- [ ] `make test` green; `go vet ./...` clean; `git diff --name-only` == {README.md, docs/how-it-works.md, docs/index.md}

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim current text + target text for all 4 edits, the per-surface edit style (inline clause for the single-line rows/bullet; a dedicated paragraph for the multi-bullet how-it-works section), the acceptance-grep nuance (don't require every `gitmoji` bullet to carry +body — the modifier is its own paragraph), the canonical PRD §17.8 wording to mirror, the explicit out-of-scope surfaces (docs/cli.md + docs/configuration.md = P1.M2.T1.S2, already done), and the scope fences.

### Documentation & References

```yaml
# MUST READ — the authoritative research (verbatim target text + the per-surface gate nuance)
- docfile: plan/021_086bbc57a2b2/P1M2T2S1/research/findings.md
  why: "§0 what's already done (NOT mine); §1 the 4 edit sites with verbatim current/target text; §2 the
        acceptance-grep nuance (how-it-works gets a paragraph, not per-bullet +body); §3 the out-of-scope
        'plain' hits (English usage, not format enums); §4 scope fences; §5 validation."
  critical: "§2: the gate for docs/how-it-works is 'the Format modes section documents +body' (a dedicated
             paragraph), NOT 'every gitmoji bullet line contains +body' (that's a false-positive trap — the
             bullets describe bases, the modifier is separate). §1(d): docs/index.md L58 is a NEW find from
             the sweep (not in the contract's named list) — it IS in scope."

# MUST READ — the +body spec (the wording the how-it-works paragraph mirrors)
- docfile: plan/021_086bbc57a2b2/prd_snapshot.md
  why: "§17.8 (h3.90) 'Body forcing (+body, FR-F9)' is the canonical paragraph: a +body variant replaces
        the multi-line rule with an unconditional body directive; the subject contract is unchanged by the
        suffix; auto+body keeps the learned subject style and forces only the body. The how-it-works
        paragraph transcribes this faithfully."
  section: "§17.8 Format modes, locale, and context — the '+body' subsection"

# MUST READ — the research note (named the README + how-it-works surfaces)
- docfile: plan/021_086bbc57a2b2/architecture/test_and_docs_surfaces.md
  why: "§README.md names README L63 + L196 as Mode-B surfaces (this task) and says 'docs/how-it-works.md —
        referenced from README:63 as the format-modes docs link. Check for format lists.' Confirms the 3
        files in scope and that docs/cli.md + docs/configuration.md are the parallel Mode-A task."
  critical: "It flags docs/how-it-works.md explicitly (the contract's literal regex sweep does NOT catch its
             multi-line bullet list, but the research note's INTENT includes it)."

# MUST EDIT — README.md (2 sites)
- file: README.md
  why: "L63 = the Message shaping features table row (single-line format enumeration); L196 = the
        `stagecoach --format conventional` example in a fenced ```bash block."
  pattern: "L63 is a markdown table row (commas separate the bases — no pipe-escaping needed for the format
            list). L196 is inside a ```bash examples fence — add a companion line immediately after it."
  gotcha: "Locate by content (`grep -n 'Message shaping\|--format conventional' README.md`), not line number
           (lines drift). The L63 docs link ([docs](docs/how-it-works.md#format-modes-and-locale)) MUST be
           preserved verbatim."

# MUST EDIT — docs/how-it-works.md (1 section)
- file: docs/how-it-works.md
  why: "The 'Format modes and locale' section (~L289-300): 4 bullets (auto/conventional/gitmoji/plain) + a
        closing 'multi-line rule still apply' paragraph + a `--locale` paragraph. NO +body anywhere today."
  pattern: "Add a dedicated '**Body forcing (`+body`).**' paragraph AFTER the 'multi-line rule still apply'
            paragraph and BEFORE the `--locale` paragraph. Mirror PRD §17.8 (see Blueprint for verbatim text)."
  gotcha: "Do NOT edit the 4 individual base bullets — the +body modifier is its own paragraph. Adding it to
           each bullet would be noisy and conflate the base contract with the suffix. (This is the ONE surface
           where +body is a paragraph, not an inline clause.)"

# MUST EDIT — docs/index.md (1 site)
- file: docs/index.md
  why: "L58 = the Features bullet '- **Message shaping** — `--format` (conventional/gitmoji/plain), …'.
        A single-line feature blurb; inline edit like README L63."
  gotcha: "NEW find from the sweep — not in the contract's named list, but it IS a format-base enumeration
           in scope per the contract's STATED intent ('any feature-blurb or format list that should mention
           +body now does'). Don't miss it."

# MUST NOT EDIT — P1.M2.T1.S2's surfaces (already done)
- file: docs/cli.md            # L38 — already shows <base>[+body]; READ-ONLY
- file: docs/configuration.md  # L116/L216/L247 — already show <base>[+body]; READ-ONLY
  why: "The parallel P1.M2.T1.S2 synced these 4 reference-doc surfaces. They already pass the +body gate.
        Touching them here = duplicate/conflicting work."
  critical: "Confirm via `rg -n gitmoji docs/cli.md docs/configuration.md` (4 hits, each already has +body)
             and LEAVE them alone."

# CONTEXT — the shipped canonical wording (READ-ONLY; the docs mirror it)
- file: internal/cmd/root.go        # --format help (the canonical user-facing phrasing)
- file: internal/config/config.go  # Format field doc (the canonical grammar statement)
- file: internal/config/bootstrap.go # :376 the config init template comment (canonical)
  why: "These 3 (P1.M1.T1.S4, Complete) are the source of truth the docs catch up to. READ-ONLY — do not edit."
```

### Current Codebase tree (relevant slice)

```bash
README.md                  # EDIT — L63 Message shaping row; L196 example (+ companion)
docs/
  how-it-works.md          # EDIT — "Format modes and locale" section (+ dedicated +body paragraph)
  index.md                 # EDIT — L58 Features bullet
  cli.md                   # READ-ONLY — L38 already <base>[+body] (P1.M2.T1.S2)
  configuration.md         # READ-ONLY — L116/L216/L247 already <base>[+body] (P1.M2.T1.S2)
internal/cmd/root.go       # READ-ONLY — canonical --format help (P1.M1.T1.S4)
internal/config/{config,bootstrap}.go  # READ-ONLY — canonical grammar doc + template (P1.M1.T1.S4)
.markdownlint.json         # READ-ONLY — {MD013:false, MD033:false, MD060:false}; no make target for docs
Makefile                   # READ-ONLY — make lint = golangci-lint (Go only); docs not linted by make
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
# MODIFIED (no new files):
README.md                  # L63 +body clause; L196 companion +body example
docs/how-it-works.md       # "Format modes and locale" + dedicated +body paragraph
docs/index.md              # L58 +body clause
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (do NOT touch docs/cli.md or docs/configuration.md): the parallel P1.M2.T1.S2 already synced
     those 4 reference-doc surfaces to <base>[+body> (confirmed via grep). Editing them here duplicates/
     conflicts with that task. My scope is README.md + docs/how-it-works.md + docs/index.md ONLY. -->

<!-- CRITICAL (how-it-works gets a PARAGRAPH, not per-bullet +body): the "Format modes and locale" section
     lists the 4 bases as separate bullets. Do NOT add +body to each bullet (noisy; conflates base contract
     with suffix). Add ONE dedicated "Body forcing (`+body`)" paragraph after the "multi-line rule still
     apply" paragraph. The acceptance gate for this surface is "the section documents +body", NOT "every
     gitmoji line contains +body" (that would false-positive on the gitmoji bullet). -->

<!-- CRITICAL (docs/index.md L58 is in scope — don't miss it): it's a NEW find from the sweep, not in the
     contract's named list, but the contract's STATED intent ("any feature-blurb or format list that should
     mention +body") covers it. It's a single-line Features bullet; inline edit like README L63. -->

<!-- GOTCHA (locate by content, not line number): lines drift on any sibling edit. Locate the 4 edit points
     via grep: README L63 via `grep -n 'Message shaping' README.md`; L196 via `grep -n -- '--format
     conventional' README.md`; how-it-works via `grep -n 'Format modes and locale'`; index.md via
     `grep -n 'Message shaping' docs/index.md`. -->

<!-- GOTCHA (preserve the README L63 docs link): the ([docs](docs/how-it-works.md#format-modes-and-locale))
     link MUST survive the edit verbatim — it's the anchor the how-it-works paragraph is reached through. -->

<!-- GOTCHA (angle/square brackets render literally in markdown): `<base>[+body]` needs no escaping. The
     README/index inline style uses parenthetical "append `+body` to force a subject+body" (backticked +body). -->

<!-- GOTCHA (docs are NOT in make): no markdownlint make target. `.markdownlint.json` has MD013 (line length)
     OFF, so long lines/bullets are fine. Validation = grep gates + manual render + scope guard. -->

<!-- GOTCHA (the "plain" sweep has false positives): `grep -rn 'plain' README.md docs/` also matches English
     usage — "plain `git commit`", "plain `config init`", "plain `push`" (README L441, how-it-works L428/L438,
     cli.md L43/L203/L207, configuration.md L57, ci-validation.md L29). These are NOT format enumerations;
     leave them. Only the 8 format-base enumerations are in scope (my 4 + P1.M2.T1.S2's 4). -->
```

## Implementation Blueprint

### Data models and structure
None. Pure markdown. No code, no types.

### Implementation Tasks (ordered — independent edits; do in any order)

```yaml
Task 1: EDIT README.md L63 — Message shaping features row (inline +body clause)
  - LOCATE: grep -n 'Message shaping' README.md  (the Features table row).
  - CURRENT:
      | Message shaping | `--format` (auto, conventional, gitmoji, plain), `--locale`, `--context`, `--template` ([docs](docs/how-it-works.md#format-modes-and-locale)). |
  - TARGET:
      | Message shaping | `--format` (auto, conventional, gitmoji, plain; append `+body` to force a subject+body), `--locale`, `--context`, `--template` ([docs](docs/how-it-works.md#format-modes-and-locale)). |
  - PRESERVE: the table-row structure, the backticks around each flag, and the ([docs](...)) link verbatim.
  - VERIFY: the row still parses as one markdown table row (the inserted clause uses commas/semicolons, no new pipe).

Task 2: EDIT README.md L196 — add a companion +body example line
  - LOCATE: grep -n -- '--format conventional' README.md  (the ```bash examples block).
  - CURRENT line:
      stagecoach --format conventional  # force conventional-commit style
  - TARGET: leave the current line UNCHANGED and ADD immediately after it:
      stagecoach --format conventional+body  # conventional-commit subject PLUS a descriptive body
  - Read the surrounding fence to place the line cleanly (it's a ```bash block of one-liners); the new line
    follows the same `cmd  # comment` style.
  - VERIFY: the fenced block still renders (no broken fence); both lines present.

Task 3: EDIT docs/how-it-works.md — add the dedicated "Body forcing (`+body`)" paragraph
  - LOCATE: grep -n 'Format modes and locale' docs/how-it-works.md  (the section). The 4 base bullets + the
    "multi-line rule still apply in every mode." paragraph precede the `--locale` paragraph.
  - INSERT a new paragraph BETWEEN the "multi-line rule still apply" paragraph and the `--locale` paragraph:
      **Body forcing (`+body`).** Append `+body` to any format — `auto+body`, `conventional+body`,
      `gitmoji+body`, `plain+body` — to replace the multi-line rule with an unconditional directive: always
      follow the subject with a wrapped (~72-column) body explaining what the change does and why. Use this
      when you want explanatory bodies even on a repo whose history is single-line. The subject contract
      (`conventional` / `gitmoji` / `plain`) is unchanged by the suffix; `auto+body` keeps the learned subject
      style and forces only the body.
  - DO NOT edit the 4 individual base bullets (auto/conventional/gitmoji/plain) — the modifier is its own paragraph.
  - VERIFY: `sed -n '/Format modes and locale/,/--locale/p' docs/how-it-works.md | grep '+body'` matches.

Task 4: EDIT docs/index.md L58 — Features bullet (inline +body clause)
  - LOCATE: grep -n 'Message shaping' docs/index.md  (the Features bullet).
  - CURRENT:
      - **Message shaping** — `--format` (conventional/gitmoji/plain), `--locale`, `--template`.
  - TARGET:
      - **Message shaping** — `--format` (conventional/gitmoji/plain; append `+body` for a subject+body), `--locale`, `--template`.
  - PRESERVE: the bullet structure + backticks.
  - VERIFY: `grep '+body' docs/index.md` matches the L58 line.

Task 5: VERIFY — per-surface grep gates + scope guard + render check
  - grep '+body' README.md docs/index.md                          # both match (L63 + L58 lines)
  - sed -n '/Format modes and locale/,/--locale/p' docs/how-it-works.md | grep '+body'   # matches (the paragraph)
  - grep 'conventional+body' README.md                            # the L196 companion example present
  - rg -n gitmoji docs/cli.md docs/configuration.md               # 4 hits, each ALREADY has +body (P1.M2.T1.S2 — untouched)
  - git diff --name-only                                          # == {README.md, docs/how-it-works.md, docs/index.md}
  - make test ; go vet ./...                                      # green / clean (docs-only; tree otherwise clean)
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: the inline clause (single-line rows/bullets — README L63, index.md L58) -->
#   … (auto, conventional, gitmoji, plain; append `+body` to force a subject+body), …
#   (semicolon before the +body clause; backticked `+body`; preserves the rest of the line verbatim)

<!-- PATTERN: the dedicated paragraph (multi-bullet section — how-it-works) -->
#   **Body forcing (`+body`).** Append `+body` to any format — `auto+body`, `conventional+body`,
#   `gitmoji+body`, `plain+body` — to replace the multi-line rule with an unconditional directive …
#   (mirrors PRD §17.8 / FR-F9; placed AFTER the base bullets, BEFORE --locale)

<!-- PATTERN: the example annotation (README L196) -->
#   stagecoach --format conventional   # force conventional-commit style       (UNCHANGED)
#   stagecoach --format conventional+body  # … subject PLUS a descriptive body (NEW companion line)
```

### Integration Points

```yaml
NO code / tests / schema / config / routes / new deps. THREE markdown files edited (no new files).

DOCS (README.md):
  - L63 Message shaping row → +body clause.
  - L196 → companion conventional+body example line.

DOCS (docs/how-it-works.md):
  - "Format modes and locale" section → dedicated "Body forcing (`+body`)" paragraph (§17.8/FR-F9).

DOCS (docs/index.md):
  - L58 Features bullet → +body clause.

SCOPE FENCES: NO Go code/tests (P1.M1 + parallel P1.M2.T1.S1); NO docs/cli.md or docs/configuration.md
  (P1.M2.T1.S2 — already done); NO PRD.md / tasks.json / prd_snapshot.md / spec; NO new files.
```

## Validation Loop

> Docs are NOT linted by `make` (`.markdownlint.json` exists with MD013/MD033/MD060 OFF but is not wired to a
> make target; `make lint` is golangci-lint for Go only). Validation = grep gates + manual render + scope guard.

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Scope guard: exactly the 3 doc files changed.
git diff --name-only
# Expected: README.md  docs/how-it-works.md  docs/index.md (exactly these 3).

# Confirm no Go file changed (docs-only task).
git diff --stat -- '*.go'
# Expected: empty.

# (Optional) markdownlint if installed locally — baseline green except known pre-existing.
npx --no-install markdownlint-cli2 'README.md' 'docs/how-it-works.md' 'docs/index.md' 2>/dev/null \
  || echo "(markdownlint not installed / not in make — skip; MD013 is off so long lines are fine)"
```

### Level 2: Unit Tests (Component Validation)

```bash
# No Go tests are authored or affected by this docs edit. Run the suite ONLY to prove the tree is clean
# (parallel Go work / the P1.M2.T1.S1 test suite may be in flight, but this docs edit cannot break Go tests).
make test
# Expected: green (race detector). Unaffected by these doc edits.
go vet ./...
# Expected: clean.
```

### Level 3: Integration Testing (System Validation)

```bash
# THE per-surface gates (the mechanical proof the sweep is complete).

# (a) README.md L63 + docs/index.md L58 each mention +body on their format line.
grep '+body' README.md docs/index.md
# Expected: 2 hits (the L63 row + the L58 bullet).

# (b) docs/how-it-works.md "Format modes and locale" section documents +body (the dedicated paragraph).
sed -n '/Format modes and locale/,/--locale/p' docs/how-it-works.md | grep '+body'
# Expected: ≥1 match (the "Body forcing" paragraph).

# (c) README.md shows a conventional+body example.
grep 'conventional+body' README.md
# Expected: ≥1 hit (the L196 companion line).

# (d) The 4 P1.M2.T1.S2 surfaces are UNCHANGED and already pass (NOT mine — confirm + leave alone).
rg -n 'gitmoji' docs/cli.md docs/configuration.md
# Expected: 4 hits (cli L38, config L116/L216/L247), each already contains '+body'.

# Manual render check of the README features table + the how-it-works section.
glow README.md 2>/dev/null | sed -n '/Message shaping/,/Configuration/p' || sed -n '60,66p' README.md
glow docs/how-it-works.md 2>/dev/null | sed -n '/Format modes/,/User payload/p' || sed -n '289,306p' docs/how-it-works.md
# Expected: L63 row renders with the +body clause; the how-it-works "Body forcing" paragraph renders as prose.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1 (THE sweep gate): no format-base enumeration in README + docs omits +body.
# Single-line enumerations (README L63, index.md L58, cli.md L38, config rows) — each must have +body:
rg -n 'conventional.*gitmoji.*plain|gitmoji.*plain|conventional/gitmoji' README.md docs/*.md | rg -v '\+body'
# Expected: empty (every single-line enumeration has +body). Any output = a missed surface.
# (NOTE: docs/how-it-works.md's bullets are MULTI-line so they won't appear here — guard 2 covers them.)

# Grep guard 2 (how-it-works section as a whole documents +body):
sed -n '/## Format modes and locale/,/^## /p' docs/how-it-works.md | grep -c '+body'
# Expected: ≥1 (the dedicated paragraph). (NOT every gitmoji bullet — see Known Gotchas.)

# Grep guard 3 (the 4 sibling surfaces untouched — P1.M2.T1.S2's domain):
git diff --name-only | grep -E '^docs/cli\.md$|^docs/configuration\.md$'
# Expected: empty (I did not touch them).

# Grep guard 4 (scope — only my 3 files):
git diff --name-only | grep -vE '^README\.md$|^docs/how-it-works\.md$|^docs/index\.md$'
# Expected: empty (no output).

# Grep guard 5 (the README L63 docs link survived):
grep -c 'docs/how-it-works.md#format-modes-and-locale' README.md
# Expected: 1 (the link is intact).

# Grep guard 6 (the "plain" false positives are untouched — English usage, not format enums):
grep -c 'plain .git commit.' README.md docs/how-it-works.md   # the "plain git commit" prose
# Expected: unchanged from baseline (these were correctly left alone).
```

## Final Validation Checklist

### Technical Validation
- [ ] `git diff --name-only` == exactly {README.md, docs/how-it-works.md, docs/index.md}
- [ ] `git diff --stat -- '*.go'` empty (docs-only)
- [ ] `make test` green + `go vet ./...` clean (tree otherwise clean — no behavioral regression)

### Feature Validation
- [ ] README.md L63 Message shaping row mentions `+body`
- [ ] README.md shows a `conventional+body` example (L196 companion line)
- [ ] docs/how-it-works.md "Format modes and locale" section has a dedicated `+body` paragraph (faithful to §17.8)
- [ ] docs/index.md L58 Features bullet mentions `+body`
- [ ] no single-line format-base enumeration in README + docs omits `+body` (guard 1 = empty)

### Scope-Boundary Validation
- [ ] Only README.md + docs/how-it-works.md + docs/index.md changed
- [ ] docs/cli.md L38 + docs/configuration.md L116/216/247 UNCHANGED (P1.M2.T1.S2 — already pass)
- [ ] NO Go code / test change (P1.M1 + parallel P1.M2.T1.S1)
- [ ] NO PRD.md / tasks.json / prd_snapshot.md / spec
- [ ] The English-"plain" false positives (plain `git commit` etc.) untouched

### Code Quality & Docs
- [ ] Wording mirrors the shipped canonical form (root.go --format help / config.go Format doc / PRD §17.8)
- [ ] how-it-works +body is a dedicated paragraph (not per-bullet noise)
- [ ] Edit points located by content (grep), not stale line numbers
- [ ] README L63 docs link + table-row structure preserved

---

## Anti-Patterns to Avoid

- ❌ Don't touch docs/cli.md or docs/configuration.md. The parallel P1.M2.T1.S2 already synced those 4
  reference-doc surfaces to `<base>[+body]` (confirmed via grep — each already passes the +body gate). Editing
  them here duplicates/conflicts with that task. My scope is README.md + docs/how-it-works.md + docs/index.md.
- ❌ Don't add `+body` to each of how-it-works's 4 base bullets. The "Format modes and locale" section lists
  auto/conventional/gitmoji/plain as separate bullets describing the BASE contract; the `+body` modifier is a
  distinct concern and gets ONE dedicated paragraph after the "multi-line rule still apply" paragraph. Per-bullet
  +body would be noisy and conflate base with suffix. (The gate for this surface is "the section documents +body",
  not "every gitmoji line has +body" — the latter false-positives on the gitmoji bullet.)
- ❌ Don't miss docs/index.md L58. It's a NEW find from the sweep (not in the contract's named list), but the
  contract's STATED intent ("any feature-blurb or format list that should mention +body") covers it. It's a
  single-line Features bullet; inline edit like README L63. The literal regex sweep catches it (it's single-line).
- ❌ Don't anchor to line numbers (63, 196, 289, 58). They drift on sibling edits. Locate the 4 points via grep
  (`Message shaping`, `--format conventional`, `Format modes and locale`, `Message shaping` in docs/index.md).
- ❌ Don't drop the README L63 docs link. The `([docs](docs/how-it-works.md#format-modes-and-locale))` anchor
  MUST survive the edit — it's how readers reach the how-it-works +body paragraph. Preserve it verbatim.
- ❌ Don't edit any Go file or test. The +body implementation (P1.M1) is Complete; the +body tests are the
  parallel P1.M2.T1.S1; the reference-doc sync is P1.M2.T1.S2. This task is the 3 overview-doc files only.
- ❌ Don't "fix" the English-"plain" hits. `grep -rn 'plain'` also matches "plain `git commit`", "plain
  `config init`", "plain `push`" (README L441, how-it-works L428/L438, cli.md L43/L203/L207, configuration.md
  L57, ci-validation.md L29). These are English usage, NOT format enumerations — leave them. Only the 8
  format-base enumerations are in scope (my 4 + P1.M2.T1.S2's 4).
- ❌ Don't invent per-surface wording. The canonical phrasing is shipped (root.go/config.go) + specified
  (PRD §17.8). Transcribe it (the how-it-works paragraph mirrors §17.8; the inline clauses use the established
  "append `+body` to force a subject+body" form from the sibling doc surfaces). Different wording per surface is
  the inconsistency this sweep removes.
- ❌ Don't change the README L196 existing line. Leave `stagecoach --format conventional  # force
  conventional-commit style` intact and ADD a companion `conventional+body` line after it. Both examples are
  useful (base + base+body); deleting the base example would be a regression.

---

## Confidence Score: 9/10

This is a 4-edit docs change across 3 markdown files, with the verbatim current + target text for every edit,
the per-surface edit style (inline clause for the single-line rows/bullet; a dedicated paragraph for the
multi-bullet how-it-works section), the canonical PRD §17.8 wording to mirror, the acceptance-grep nuance
(don't require every `gitmoji` bullet to carry +body), the explicit out-of-scope surfaces (docs/cli.md +
docs/configuration.md = P1.M2.T1.S2, already done), and the "plain" false-positive trap spelled out. No code,
no tests, no schema. The one residual (not a full 10) is docs/how-it-works.md being a prose section (not a
mechanical cell edit) — the paragraph wording is prescribed but the implementer places it between two named
paragraphs, so placement is unambiguous; the risk is only stylistic fit, which the §17.8 transcription resolves.
One-pass success is highly likely.