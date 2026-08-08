name: "P1.M1.T1.S6 — docs/how-it-works.md: add the 'File-disjoint fast-path' paragraph (Mode A)"
description: >
  A surgical Mode-A documentation edit: add ONE bold-lead-in paragraph, "**File-disjoint fast-path
  (FR-M13/FR-M14).**", to the `### Key design points` block of docs/how-it-works.md's
  `## Multi-commit decomposition` section — placed DIRECTLY AFTER the existing "Overlapped staging
  and generation" paragraph (line 103), the natural contrast point (the fast-path lifts that 1-deep
  overlap). The doc currently presents the tooled-stager + 1-deep-overlap as THE decompose model;
  this paragraph documents the SECOND mode so the doc is accurate to the implemented fast-path
  (S1–S5) and the spec (FR-M13/FR-M14, §13.6.3 addendum, G29). Content floor: pairwise file-disjoint
  partition → no stager, no 1-deep overlap → deterministic `git add` per concept (adds/mods/deletions,
  accumulate-never-reset) → N messages concurrent → CAS-ordered publish → critical path collapses to
  "planner + one message latency"; any shared file falls back transparently to the tooled-stager loop
  above; `verifyFreezeSubset` (FR-M1c) still guards every tree; unclaimed paths still flow to the
  arbiter. SURGICAL — do not reflow the rest of the section (the pipeline-flow diagram, the other
  Key-design-points paragraphs, the "Output quality" paragraph at line 123, the Safety bullets). NO
  README change (fast-path is invisible/additive, no CLI/config surface — confirmed). NO cli.md /
  configuration.md change (config-free). NO providers.md change (that is Phase 2 / P2.M1.T1.S1). NO
  spec/ change (human-owned, read-only). The only file touched is docs/how-it-works.md.

---

## Goal

**Feature Goal**: Make `docs/how-it-works.md` accurately document **both** decompose modes — the
existing tooled-stager loop (1-deep overlap) AND the file-disjoint fast-path (FR-M13/FR-M14,
implemented by S1–S5) — by adding one self-contained "File-disjoint fast-path" paragraph to the
decompose section. Today the doc presents the stager + 1-deep overlap as *the* decompose model
("Overlapped staging and generation … This 1-deep overlap keeps latency low", line 103) and only
nods to disjoint files in passing ("…is the most robust against this", line 123); a reader cannot
learn from the doc that disjoint partitions take a faster, stager-free, fully-concurrent path.

**Deliverable**: Exactly one new `**bold-lead-in**` paragraph —
`**File-disjoint fast-path (FR-M13/FR-M14).**` — inserted into the `### Key design points` block of
`docs/how-it-works.md`, immediately after the "Overlapped staging and generation." paragraph and
before the "Stage-while-editing (FR-E2)." paragraph. Matches the doc's existing voice (bold lead-in
+ prose, backtick code spans, em-dashes, FR-Mxx tags). No other prose is rewritten; the edit is a
pure insertion (added lines only in `git diff`).

**Success Definition**:
- `docs/how-it-works.md` documents both decompose modes; the new paragraph is accurate to FR-M13 /
  FR-M14 / §13.6.3 / G29 (every factual claim cross-checks against the spec text in
  [research/findings.md](research/findings.md) §2).
- The edit is **surgical**: `git diff docs/how-it-works.md` shows ONLY added lines (the new
  paragraph + the blank lines separating it from its neighbors); every other paragraph in the
  decompose section — the pipeline-flow ASCII diagram, "Overlapped staging and generation",
  "Stage-while-editing (FR-E2)", "Frozen tree snapshots", "Tree-to-tree diffs", "Serialized
  publication", "Start-of-run freeze (T_start)", "Freeze enforcement", "One-file short-circuit",
  "Mode-conditional planner rules", "Arbiter leftover reconciliation", "Output quality …"
  (line 123), and the Safety bullets — is **byte-identical**.
- `git diff --stat` lists **only** `docs/how-it-works.md`. No README, cli.md, configuration.md,
  providers.md, or spec/ file is touched.
- The paragraph passes `.markdownlint.json` if a markdown linter is available (MD013 line-length is
  OFF in the project config, so long lines are fine; markdownlint is NOT a CI gate here).

## User Persona (if applicable)

**Target User**: A developer or contributor reading the architecture overview to understand how
multi-commit decomposition works internally — either evaluating Stagecoach, onboarding, or
maintaining the decompose pipeline.

**Use Case**: The reader scans `## Multi-commit decomposition` → `### Key design points` to learn the
pipeline's shape and its performance/safety trade-offs. They currently leave thinking the stager +
1-deep overlap is the *only* model; for a disjoint change set (the common case, line 123) they
would wrongly infer ~N sequential LLM steps when in fact the run collapses to one message latency.

**User Journey**: Read "Overlapped staging and generation" → immediately read the new
"File-disjoint fast-path" paragraph → understand that for the disjoint common case there is no
stager and messages run concurrently → continue to "Frozen tree snapshots" / "Tree-to-tree diffs" /
"Serialized publication" / "Arbiter leftover reconciliation" to see the unchanged invariants.

**Pain Points Addressed**: The doc silently understates disjoint-run performance and overstates the
stager's role (the stager is bypassed entirely on disjoint trees). A provider with no `tooled_flags`
(opencode, qwen-code) can decompose a disjoint tree — a capability the doc never mentions.

## Why

- **Mode A — rides with the work (SOW §5)**: the fast-path ships in this changeset (S1–S5); the doc
  must ship the matching narrative in the same changeset so the documentation is not stale on
  landing. A doc that describes a mode the binary no longer (or not-yet) exhibits is worse than no
  doc.
- **Spec accuracy**: FR-M13/FR-M14 (spec/01-product.md:363-364) and the §13.6.3 addendum
  (spec/03-generation.md:120) DEFINE a second decompose mode. `docs/how-it-works.md` is the
  architecture overview; presenting only one mode is a documentation defect against the spec.
- **Coherence with the existing doc**: line 123 already asserts "The disjoint-files common case … is
  the most robust against this" — but never explains *why*. The fast-path paragraph supplies the
  mechanism (no stager model in the loop, deterministic staging, full concurrency) that makes that
  assertion true and actionable. The two paragraphs become complementary rather than the latter
  dangling.
- **G29 side effect**: the fast-path lets `tooled_flags`-less providers (opencode, qwen-code)
  decompose disjoint trees they otherwise could not serve as stager. One clause captures a real user
  benefit with no extra surface area.

## What

**User-visible behavior**: none (documentation only — no binary, CLI, config, or behavior change).

**Technical change**: ONE markdown paragraph added to `docs/how-it-works.md`, in the `### Key design
points` block, positioned directly after the "Overlapped staging and generation." paragraph (line
103) and before the "Stage-while-editing (FR-E2)." paragraph (line 105).

### Content floor (every claim below MUST appear; verify each against the spec in research/findings.md §2)

1. **Trigger**: the planner's partition is **pairwise file-disjoint** — no path appears in more than
   one concept's `files`.
2. **No stager, no 1-deep overlap**: stagecoach stages each concept **deterministically with
   `git add`** (covers adds, modifications, **and deletions** for those whole paths), under the
   unchanged **accumulate-never-reset** index model, **invoking no stager agent at all**.
3. **Concurrency + CAS order**: because every `tree[i]` is frozen *before* any message starts, the N
   message generations run **concurrently** and publish **in CAS order** (serialized `update-ref`,
   same as the loop).
4. **Critical path**: a disjoint run's critical path collapses from "planner + ~N sequential steps"
   to **"planner + one message latency"**.
5. **Fallback**: any **shared file** (a path in ≥2 concepts' `files`) **falls back transparently to
   the tooled-stager loop above** — for the whole run, not per-concept.
6. **Invariants preserved** (accuracy to FR-M13/FR-M14): `verifyFreezeSubset` (**FR-M1c**) still
   guards every fast-path tree; paths the planner declared for no concept still **flow to the
   arbiter** as leftovers; the tree-to-tree-diff and serialized-CAS invariants are **unchanged**.
7. **(Tie-in, optional but recommended)**: this is why the disjoint-files case is the most robust —
   and now the fastest (no stager model in the loop); and a provider with no `tooled_flags`
   (opencode, qwen-code) can decompose a disjoint tree it otherwise could not.

### Success Criteria
- [ ] `docs/how-it-works.md` contains exactly ONE new paragraph whose bold lead-in is
      `**File-disjoint fast-path (FR-M13/FR-M14).**`.
- [ ] The paragraph sits between the "Overlapped staging and generation." paragraph and the
      "Stage-while-editing (FR-E2)." paragraph (i.e. adjacent to the overlap paragraph it contrasts).
- [ ] Every claim in the Content Floor (points 1–6) is present and matches the spec verbatim text in
      research/findings.md §2.
- [ ] `git diff docs/how-it-works.md` shows ONLY `+` lines for the new paragraph (+ the blank lines
      flanking it); NO `-`/changed lines in the surrounding paragraphs (surgical).
- [ ] `git diff --stat` lists ONLY `docs/how-it-works.md`.
- [ ] No reflow of the pipeline-flow diagram, the other Key-design-points paragraphs, the line-123
      "Output quality" paragraph, or the Safety bullets.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes.** The implementer needs: the exact file and insertion line (given), the exact anchor text to
target (given, lines 103–105), the canonical spec text the paragraph must mirror (given verbatim in
research/findings.md §2), the doc's voice/style (given in research/findings.md §1), the content floor
and scope fences (given above), and the validation commands (given below). No codebase knowledge
beyond "open docs/how-it-works.md at line 103 and insert a paragraph" is required.

### Documentation & References

```yaml
# MUST READ — the spec is the authority for the paragraph's CONTENT
- url: spec/01-product.md  (§9.14, lines 363-364)   # FR-M13 + FR-M14, verbatim
  why: "FR-M13 = the fast-path gate + deterministic `git add` staging (no stager, whole-path adds/mods/
        deletions, FR-M1c guards every tree, unclaimed paths → arbiter, shared file → fallback for the
        whole run, tooled_flags-less provider benefit). FR-M14 = N messages concurrent + CAS order, 1-deep
        overlap lifted, §13.6.3 invariants preserved, critical-path collapse. These two lines ARE the
        content floor; mirror their claims, adapted to the doc's voice."
  critical: "Do NOT invent claims. If a claim is not in FR-M13/FR-M14 or §13.6.3, do not put it in the
             paragraph. (e.g. 'tooled_flags-less provider' benefit is in FR-M13 — include at most one
             clause; do not expand into a providers.md-style discussion — that's Phase 2.)"

- url: spec/03-generation.md  (§13.6.3 addendum, ~line 120)  # the model PROSE to mirror
  why: "The addendum's 'File-disjoint fast-path (FR-M13/M14)' paragraph is the prose template — same
        logical flow (stager is the only serial role → exists solely to hunk-split → disjoint partition
        → deterministic git add → trees frozen up front → N messages concurrent + CAS → verifyFreezeSubset
        guards → unclaimed → arbiter → shared falls back). Adapt its register to the doc's slightly more
        conversational tone."
  critical: "The doc is the architecture OVERVIEW (readable, one paragraph); the spec is the precise
             reference. Keep the paragraph to ONE bold-lead-in block, not a sub-section with headings."

- url: spec/01-product.md  (G29, line 165)  # the one-sentence what+why (the goal)
  why: "G29 is the goal statement: pairwise file-disjoint → git add (bypass stager) → N concurrent →
        CAS order → critical path ~N → one message latency → FR-M1c guards → shared → tooled stager
        fallback → tooled_flags-less provider benefit. Use it as the accuracy cross-check."

# MUST READ — the file under edit
- file: docs/how-it-works.md
  why: "THE file to edit. The decompose section is `## Multi-commit decomposition`; the design
        paragraphs live under `### Key design points`, each a **bold-lead-in** paragraph. The new
        paragraph joins them as a peer."
  pattern: "Mirror the existing **bold-lead-in** + prose style EXACTLY. See 'Overlapped staging and
            generation' (line 103), 'Serialized publication' (line 111), 'Start-of-run freeze (T_start)'
            (line 113), 'Arbiter leftover reconciliation' (line 121) — same shape: `**Title.**` then
            2–5 sentences with `code spans` for git cmds / tree vars / `files`, em-dashes, FR-Mxx tags."
  gotcha: "MD013 (line-length) is OFF in .markdownlint.json → long lines are fine (the existing doc uses
           them). Do NOT hard-wrap the paragraph to 80 cols; match the surrounding paragraphs' flowing
           style. There is NO table-of-contents at the file top and NO per-paragraph anchors to update."

- file: docs/how-it-works.md  (lines 103-105)   # the exact insertion anchor
  why: "Line 103 is the 'Overlapped staging and generation.' paragraph (ends '…This 1-deep overlap
        keeps latency low.'); line 104 is blank; line 105 begins '**Stage-while-editing (FR-E2).**'.
        Insert the new paragraph BETWEEN them — directly contrasting the 1-deep overlap the fast-path
        lifts. This is the item's preferred placement ('adjacent to the Overlapped paragraph (~line 103)')."
  pattern: "edit-tool anchor (unique in the file): oldText =
            'This 1-deep overlap keeps latency low.\n\n**Stage-while-editing (FR-E2).**'
            → newText = 'This 1-deep overlap keeps latency low.\n\n**File-disjoint fast-path
            (FR-M13/FR-M14).** <PARAGRAPH>\n\n**Stage-while-editing (FR-E2).**'. One insertion; no
            surrounding lines touched."

- file: docs/how-it-works.md  (line 123)   # the 'Output quality … most robust' paragraph — DO NOT EDIT
  why: "The 'Output quality is bounded by stager-model discipline' paragraph ends '…The disjoint-files
        common case … is the most robust against this.' It COMPLEMENTS the new paragraph (explains WHY
        disjoint is robust). The item says 'Surgical edit — do not reflow the rest of the section.'"
  gotcha: "Do NOT edit line 123. The new paragraph may REFERENCE the concept ('…which is why the
           disjoint-files case is the most robust…') but must not modify the existing sentence. If the
           tie-in feels redundant, drop the tie-in clause — keep the edit surgical."

# MUST READ — research notes (verified facts; the cross-check source)
- docfile: plan/019_2f5621db4d2b/P1M1T1S6/research/findings.md
  why: "§1 = the file's exact current text + style map (which lines say what). §2 = FR-M13/FR-M14 +
        §13.6.3 addendum + G29 VERBATIM (the authority). §3 = the content-floor checklist. §4 = scope
        fences (README/cli/configuration/providers/spec all OUT). §5 = validation surface (markdownlint
        config + the fact markdownlint is NOT a CI gate). Cross-check every claim in the new paragraph
        against §2 before committing."

- docfile: plan/019_2f5621db4d2b/architecture/spec_requirements.md
  why: "The plan's verbatim extraction of FR-M13/FR-M14 + §13.6.3 + G29 + §20.2/§20.5. Confirmed
        accurate against spec/. Use as a second cross-check (it is a copy of the spec, not the spec)."

# CROSS-REFERENCE (read-only — to AVOID scope creep)
- file: README.md
  why: "README mentions 'decompose'/'stager' (planner → stager → message → arbiter) but NEVER overlap /
        1-deep / fast-path / disjoint. The fast-path is invisible at the user surface (no flag, no
        config). Confirms NO README change. DO NOT edit README."
- file: docs/providers.md
  why: "The stager/tooled-flags provider table lives here and is OUT OF SCOPE (Phase 2 / P2.M1.T1.S1).
        The new paragraph's optional one-clause 'tooled_flags-less provider' mention is the limit; do
        NOT expand it or touch providers.md."
```

### Current Codebase tree (the doc slice)

```bash
docs/
  how-it-works.md   # ← EDIT: add ONE paragraph in `### Key design points` (after line 103)
  cli.md            # UNCHANGED (fast-path is config-free — no flag)
  configuration.md  # UNCHANGED (fast-path adds no config key)
  providers.md      # UNCHANGED (Phase 2 scope — P2.M1.T1.S1)
  README.md         # UNCHANGED (fast-path is invisible/additive at the user surface)
  packaging.md      # UNCHANGED
  ci-validation.md  # UNCHANGED
  windows-test-support.md  # UNCHANGED
spec/               # READ-ONLY — human-owned (AGENTS.md hard rule #1)
.markdownlint.json  # READ-ONLY — { default:true, MD013:false, MD033:false, MD060:false }
```

### Desired Codebase tree with files to be added/modified

```bash
docs/how-it-works.md   # MODIFY: +1 paragraph ("File-disjoint fast-path (FR-M13/FR-M14).") after line 103.
                        #   Pure insertion — the rest of the file is byte-identical.
# NOTHING ELSE. No README, no cli.md, no configuration.md, no providers.md, no spec/, no code.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (source of truth). The paragraph's CONTENT is fixed by the SPEC (FR-M13/FR-M14 +
     §13.6.3 addendum), not by the implementation or the S5 tests. Mirror the spec's claims; do not
     invent. If unsure whether a claim is spec-backed, check research/findings.md §2. -->

<!-- CRITICAL (surgical = added lines only). Use ONE edit-tool call whose oldText is the unique join
     between the 'Overlapped staging' paragraph and the 'Stage-while-editing' paragraph, and whose
     newText inserts the new paragraph (with its flanking blank lines) between them. There must be
     ZERO `-` lines in `git diff docs/how-it-works.md` except possibly the blank-line reflow right at
     the insertion seam — minimize even that by anchoring on the two non-blank lead-ins. -->

<!-- GOTCHA (MD013 is OFF). .markdownlint.json disables line-length. The existing doc uses long,
     flowing lines. Do NOT hard-wrap the new paragraph to 80 columns — it would be inconsistent with
     every sibling paragraph and is unnecessary. -->

<!-- GOTCHA (no TOC / no anchors). how-it-works.md has no table of contents and the Key-design-points
     block has no per-paragraph heading anchors. Adding a bold-lead-in paragraph needs no index/TOC
     update. -->

<!-- GOTCHA (consistency, not reflow). Line 123 already says disjoint is 'most robust'. The new
     paragraph explains WHY (no stager model in the loop, deterministic staging, full concurrency).
     The two are complementary — do not delete or rewrite line 123. If a tie-in clause would duplicate
     it, prefer dropping the clause over editing line 123. -->

<!-- GOTCHA (FR tag style). The doc uses BOTH the FR-Mxx tag AND the symbol name. 'verifyFreezeSubset'
     appears as a name in the 'Freeze enforcement' paragraph; 'FR-M1c' appears as a tag in the Safety
     bullet (line 132). Use 'verifyFreezeSubset (FR-M1c)' once (the doc's own composite form) — matches
     the §13.6.3 addendum exactly. -->

<!-- GOTCHA (placement precision). Insert AFTER 'Overlapped staging and generation' (the contrast
     point), NOT at the end of Key design points. The fast-path is the direct lift of that paragraph's
     '1-deep overlap'; adjacency is what makes the narrative land. -->
```

## Implementation Blueprint

### Data models and structure

None (documentation only). No code, config, schema, or test change.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: WRITE the "File-disjoint fast-path (FR-M13/FR-M14)." paragraph (the content)
  - SOURCE: spec/01-product.md:363-364 (FR-M13/FR-M14, verbatim) + spec/03-generation.md:120
    (§13.6.3 addendum — the prose template) + spec/01-product.md:165 (G29). All three reproduced
    verbatim in research/findings.md §2. READ §2 before drafting.
  - COVER the Content Floor (points 1-7 in the PRP's "What" section). Points 1-6 are MANDATORY;
    point 7 (tie-in + tooled_flags-less provider clause) is recommended.
  - VOICE: **bold lead-in** + prose, matching 'Overlapped staging and generation' / 'Serialized
    publication' / 'Arbiter leftover reconciliation'. `code spans` for git cmds (`git add`,
    `write-tree`), tree vars (`tree[i]`), the partition field (`files`). Em-dashes. FR-Mxx tags
    (FR-M13, FR-M14, FR-M6, FR-M7, FR-M1c). ONE paragraph (no sub-headings, no bullets).
  - LENGTH: comparable to its siblings (the §13.6.3 addendum paragraph is ~7 sentences; aim for
    5-8 sentences — enough to cover the floor, not a sub-section).
  - REFERENCE DRAFT (adapt to doc voice; this is the FLOOR content, not a straitjacket):
      "**File-disjoint fast-path (FR-M13/FR-M14).** The stager is the only tooled role and the only
      inherently serial step in the pipeline — every stager mutates the same live index, so the loop
      above can overlap staging and generation only 1-deep. But the stager exists solely to hunk-split
      a file shared across concepts; the planner already declares a file-level partition (`files` per
      concept). When that partition is **pairwise file-disjoint** — no path appears in two concepts'
      `files` — stagecoach stages each concept **deterministically** with `git add` (adds, modifications,
      and deletions for those whole paths), under the unchanged accumulate-never-reset index model,
      **invoking no stager agent at all**. With every `tree[i]` frozen before any message starts, the N
      message generations then run **concurrently** and publish in CAS order, collapsing a disjoint
      run's critical path from \"planner + ~N sequential steps\" to \"planner + one message latency.\"
      `verifyFreezeSubset` (FR-M1c) still guards every fast-path tree; paths the planner declared for no
      concept still flow to the arbiter as leftovers; and the tree-to-tree-diff and serialized-CAS
      invariants are unchanged. Any shared file falls back transparently to the tooled stager above —
      for the whole run, not per concept. This is why the disjoint-files case is both the most robust
      and the fastest path (no stager model in the loop), and it lets a provider with no `tooled_flags`
      (opencode, qwen-code) decompose a disjoint tree it otherwise could not."
  - CROSS-CHECK: before finalizing, re-read each sentence against research/findings.md §2 and confirm
    every claim is spec-backed. Delete any sentence that is not.

Task 2: INSERT the paragraph surgically at line 103 (the placement)
  - TOOL: one `edit` call on docs/how-it-works.md.
  - ANCHOR (unique in the file):
      oldText:  "This 1-deep overlap keeps latency low.\n\n**Stage-while-editing (FR-E2).**"
      newText:  "This 1-deep overlap keeps latency low.\n\n**File-disjoint fast-path (FR-M13/FR-M14).**"
                + " " + <Task 1 paragraph body> + "\n\n**Stage-while-editing (FR-E2).**"
  - RATIONALE: anchoring on the join of the two non-blank lead-ins makes the insertion a PURE ADD —
    `git diff` will show only `+` lines (the new paragraph + its two flanking blank lines). The two
    anchored lead-ins ('…keeps latency low.' and '**Stage-while-editing (FR-E2).**') are unchanged in
    the result, so they contribute no `-`/`+` noise.
  - PLACEMENT CHECK: after the edit, the order in `### Key design points` MUST be:
      Overlapped staging and generation.  →  File-disjoint fast-path (FR-M13/FR-M14).  →  Stage-while-editing (FR-E2).
  - DO NOT: reflow the pipeline-flow ASCII diagram (lines ~75-93); edit any other Key-design-points
    paragraph; touch the line-123 'Output quality' paragraph; change the Safety bullets.

Task 3: VERIFY gates (see Validation Loop)
  - git diff docs/how-it-works.md  → only `+` lines (the one paragraph + flanking blanks).
  - git diff --stat                → ONLY docs/how-it-works.md.
  - grep -c '\*\*File-disjoint fast-path (FR-M13/FR-M14)\.\*\*' docs/how-it-works.md  → exactly 1.
  - accuracy cross-check vs research/findings.md §2 (every claim spec-backed).
  - markdownlint (if available) clean against .markdownlint.json; MD013 OFF so long lines pass.
  - DEPENDS: Tasks 1-2.
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: the edit is a pure insertion anchored on two unchanged lead-ins. -->
oldText (exact, unique):
  This 1-deep overlap keeps latency low.

  **Stage-while-editing (FR-E2).**
newText:
  This 1-deep overlap keeps latency low.

  **File-disjoint fast-path (FR-M13/FR-M14).** <PARAGRAPH BODY>

  **Stage-while-editing (FR-E2).**

<!-- RESULT: `git diff docs/how-it-works.md` shows ONLY:
       +<blank>
       +**File-disjoint fast-path (FR-M13/FR-M14).** ...
       +<blank>
     The two anchored lines are byte-identical before/after → no `-` noise. -->

<!-- PATTERN: voice match — bold lead-in + prose, same as siblings. Compare:
  **Overlapped staging and generation.** `stager[i+1]` runs in parallel with `message[i]` — the stager
  prepares the next concept's index while the message agent generates the current commit message. This
  1-deep overlap keeps latency low.
  → new paragraph uses the SAME register: **Title.** <prose with `code spans`, em-dashes, FR tags>. -->

<!-- PATTERN: FR-tag + symbol composite form (already used elsewhere in the doc + in the spec addendum):
  `verifyFreezeSubset` (FR-M1c)   — name first, tag parenthetically. Matches §13.6.3 addendum exactly. -->
```

### Integration Points

```yaml
NO code, config, schema, CLI, or test integration. ONE markdown file edited.

DOCS (the sole edit):
  - docs/how-it-works.md — +1 paragraph ("File-disjoint fast-path (FR-M13/FR-M14).") in
    `### Key design points`, after the "Overlapped staging and generation." paragraph (line 103).

CROSS-DOCS (read-only — do NOT edit; confirmed accurate as-is or out of scope):
  - README.md            — fast-path is invisible/additive at the user surface; no change.
  - docs/cli.md          — fast-path is config-free (no flag); no change.
  - docs/configuration.md— fast-path adds no config key; no change.
  - docs/providers.md    — stager/tooled_flags table is Phase 2 (P2.M1.T1.S1); OUT of scope here.
  - spec/*.md            — human-owned, read-only (AGENTS.md hard rule #1).

BUILD/TEST (unaffected):
  - make test / make lint (golangci-lint) / go test — all UNCHANGED (prose edit only). No need to
    re-run the Go suite as a gate; harmless if run.
```

## Validation Loop

### Level 1: Markdown Style (Immediate Feedback)

```bash
# .markdownlint.json = { default:true, MD013:false, MD033:false, MD060:false }.
# MD013 (line-length) is OFF → long flowing lines are fine (the whole doc uses them).
# markdownlint is NOT run in .github/workflows/ci.yml — this is a LOCAL nicety, not a CI gate.
# Run it only if a CLI is available; if unavailable, skip (it is not a gate).
npx --no-install markdownlint-cli2 docs/how-it-works.md 2>/dev/null \
  || npx markdownlint-cli docs/how-it-works.md 2>/dev/null \
  || echo "markdownlint not installed — skipping (not a CI gate for this repo)"

# Expected: no NEW violations introduced by the paragraph. (Existing doc state is the baseline;
#   do not fix pre-existing issues — out of scope.) The new paragraph is plain prose (no heading,
#   no list, no inline HTML) so MD001/MD032/MD033 etc. cannot trip on it.
```

### Level 2: Surgical-Insertion Gate (the real check)

```bash
# (a) ONLY docs/how-it-works.md changed.
git diff --stat
# Expected: exactly one line — " docs/how-it-works.md | <n> +++++… " — and nothing else.

# (b) The diff is a PURE INSERTION: only `+` lines (the paragraph + its flanking blank lines).
#     ZERO `-` lines except possibly the blank-line seam right at the insertion.
git diff docs/how-it-works.md
# Expected: context = the tail of 'Overlapped staging and generation' + the head of
#   'Stage-while-editing (FR-E2)'; additions = the new paragraph (its bold lead-in line + body +
#   surrounding blanks). No `-` lines on ANY other paragraph, the diagram, line 123, or the Safety
#   bullets. If you see `-` lines anywhere else, the edit was not surgical — redo it.

# (c) The lead-in appears exactly once.
grep -c '\*\*File-disjoint fast-path (FR-M13/FR-M14)\.\*\*' docs/how-it-works.md
# Expected: 1.

# (d) The two anchored neighbors are byte-identical (the insertion did not mangle them).
grep -n 'This 1-deep overlap keeps latency low.' docs/how-it-works.md      # Expected: 1 line (line ~103)
grep -n '\*\*Stage-while-editing (FR-E2)\.\*\*' docs/how-it-works.md        # Expected: 1 line
grep -n '\*\*Overlapped staging and generation\.\*\*' docs/how-it-works.md  # Expected: 1 line (unchanged)
```

### Level 3: Accuracy Cross-Check (spec conformance)

```bash
# For every factual claim in the new paragraph, confirm it is backed by the spec. Open these two
# sources side-by-side with the new paragraph and verify claim-by-claim (manual, but the gate):
#   - spec/01-product.md lines 363-364  (FR-M13 + FR-M14, verbatim)
#   - spec/03-generation.md ~line 120   (§13.6.3 "File-disjoint fast-path (FR-M13/M14)" addendum)
# Reproduced verbatim in plan/019_2f5621db4d2b/P1M1T1S6/research/findings.md §2.

# CLAIM CHECKLIST (each must be TRUE in the new paragraph AND in the spec):
#   [ ] pairwise file-disjoint = no path in >1 concept's files                 (FR-M13)
#   [ ] deterministic git add per concept (adds + mods + deletions, whole paths)(FR-M13)
#   [ ] accumulate-never-reset index model unchanged                            (FR-M13 / §13.6.3)
#   [ ] NO stager agent invoked on the fast-path                                (FR-M13)
#   [ ] every tree[i] frozen before any message starts                          (FR-M14)
#   [ ] N message generations run CONCURRENTLY                                  (FR-M14)
#   [ ] publish in CAS order (serialized update-ref)                            (FR-M14 / FR-M7)
#   [ ] critical path: "planner + one message latency"                          (FR-M14 / G29)
#   [ ] verifyFreezeSubset (FR-M1c) still guards every tree                     (FR-M13)
#   [ ] unclaimed paths still flow to the arbiter                               (FR-M13 / FR-M9)
#   [ ] shared file → tooled-stager fallback for the WHOLE run                  (FR-M13 / §13.6.3)
#   [ ] (if included) tooled_flags-less provider (opencode/qwen-code) benefit   (FR-M13 / G29)
# Any claim NOT in the spec MUST be removed. No invented detail.
```

### Level 4: Scope & Anti-Reflow Guards

```bash
# Scope guard: NO file outside docs/how-it-works.md is touched.
git diff --name-only
# Expected: docs/how-it-works.md   (and only it).

# Anti-reflow guard: the pipeline-flow ASCII diagram is untouched.
git diff docs/how-it-works.md | grep -E '^[+-]' | grep -iE 'planner|stager\[|message\[|update-ref HEAD|arbiter'
# Expected: EMPTY (the diagram lines are unchanged; the only +/- lines are the new paragraph).

# Anti-reflow guard: the line-123 'Output quality / most robust' paragraph is untouched.
git diff docs/how-it-works.md | grep -E '^-' | grep -iE 'most robust|stager-model discipline'
# Expected: EMPTY (line 123 is not edited).

# Anti-reflow guard: the Safety bullets (FR-M1c line) are untouched.
git diff docs/how-it-works.md | grep -E '^-' | grep -iE 'FR-M1c|content-subset of T_start'
# Expected: EMPTY.

# Render sanity (optional): confirm the file is still valid markdown the eye can read.
# (No renderer is required; this is a eyeball check that the bold lead-in + prose parses.)
sed -n '100,112p' docs/how-it-works.md
# Expected: Overlapped paragraph, blank, NEW File-disjoint paragraph, blank, Stage-while-editing paragraph.
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1: markdownlint (if available) introduces no new violations vs the baseline doc.
- [ ] Level 2: `git diff --stat` lists ONLY `docs/how-it-works.md`.
- [ ] Level 2: `git diff docs/how-it-works.md` is a PURE INSERTION (only `+` lines for the paragraph
      + flanking blanks; no `-` lines outside the seam).
- [ ] Level 2: `grep -c '**File-disjoint fast-path (FR-M13/FR-M14).**'` == 1.
- [ ] Level 3: every claim in the new paragraph is backed by spec/01-product.md:363-364 or
      spec/03-generation.md §13.6.3 (the checklist above, all boxes ticked).
- [ ] Level 4: `git diff --name-only` == `docs/how-it-works.md` only.

### Feature Validation (documentation correctness)
- [ ] `docs/how-it-works.md` now documents BOTH decompose modes (tooled-stager loop + file-disjoint
      fast-path) in the `### Key design points` block.
- [ ] The new paragraph sits adjacent to "Overlapped staging and generation." (it contrasts the
      1-deep overlap the fast-path lifts).
- [ ] Content Floor points 1–6 all present and spec-accurate; point 7 included if it reads cleanly.
- [ ] The narrative is self-consistent: the fast-path paragraph does not contradict "Overlapped
      staging and generation", "Tree-to-tree diffs", "Serialized publication", "Arbiter leftover
      reconciliation", or the line-123 "Output quality" paragraph.
- [ ] Manual read: a developer new to the codebase can, from this paragraph alone, correctly predict
      that a disjoint change set takes the stager-free concurrent path and collapses to one message
      latency, and that a shared file falls back to the loop.

### Code Quality & Scope
- [ ] The edit matches the existing **bold-lead-in** + prose voice (code spans, em-dashes, FR tags).
- [ ] MD013 (line-length) respected as the project does — i.e. flowing long lines, NOT hard-wrapped.
- [ ] NO reflow of the pipeline-flow diagram, any sibling Key-design-points paragraph, the line-123
      paragraph, or the Safety bullets.
- [ ] NO README / cli.md / configuration.md / providers.md / spec/ change.
- [ ] No code, config, schema, or test change (docs-only).

### Documentation & Deployment
- [ ] The paragraph is self-contained (no dangling forward-reference that the doc doesn't already
      support — "the tooled stager above", "the arbiter", `verifyFreezeSubset`/FR-M1c are all defined
      earlier in the same section).
- [ ] Cross-links to other docs (configuration.md / cli.md / providers.md) are NOT added (none are
      needed — the fast-path has no config/CLI surface).

---

## Anti-Patterns to Avoid

- ❌ Don't invent claims. The paragraph's content is FIXED by the spec (FR-M13/FR-M14 + §13.6.3
  addendum). Every sentence must be spec-backed (cross-check research/findings.md §2). If a sentence
  isn't in the spec, delete it.
- ❌ Don't reflow the section. One insertion only. Do not "tidy" the pipeline-flow diagram, the sibling
  paragraphs, the line-123 "Output quality" paragraph, or the Safety bullets — the item says surgical.
- ❌ Don't hard-wrap the paragraph to 80 columns. MD013 is OFF in `.markdownlint.json` and every sibling
  paragraph uses flowing long lines. Hard-wrapping would be inconsistent and noisy in `git diff`.
- ❌ Don't add a sub-heading, a bullet list, or a new `###`/`####` for the fast-path. It is a peer
  bold-lead-in paragraph in `### Key design points`, matching "Overlapped staging and generation".
- ❌ Don't edit README, cli.md, configuration.md, or providers.md. The fast-path is invisible/additive
  (no CLI flag, no config key); providers.md's stager table is Phase 2 (P2.M1.T1.S1). Scope = ONE file.
- ❌ Don't edit `spec/*.md` — human-owned, read-only (AGENTS.md hard rule #1). The spec is the source
  you READ, not the file you WRITE.
- ❌ Don't duplicate the §13.6.3 addendum verbatim into the doc. The doc is the readable overview;
  adapt the addendum's register to the doc's voice (slightly more conversational, one paragraph).
- ❌ Don't over-explain the `tooled_flags`-less-provider angle. At most one clause (it's a G29 side
  effect). Expanding it into provider-by-provider detail is providers.md's job (Phase 2).
- ❌ Don't place the paragraph at the END of Key design points. Place it ADJACENT to "Overlapped
  staging and generation" — the contrast (lifting the 1-deep overlap) is what makes the narrative land.
- ❌ Don't contradict line 123. "Output quality" says disjoint is "most robust"; the fast-path paragraph
  explains WHY (no stager model in the loop). The two are complementary — reference the concept, don't
  rewrite line 123.
- ❌ Don't run the whole Go test suite / `make test` as a gate for this change. It is a prose edit; the
  Go suite is unaffected. (Running it is harmless but wastes a cycle — the real gates are Levels 2–4.)

---

## Confidence Score: 9.5/10

This is a single-paragraph surgical documentation edit with a fully-specified insertion point, a
verbatim spec source of truth (reproduced in research/findings.md §2), a ready reference draft, a
precise edit-tool anchor, and deterministic gates (`git diff` must be a pure insertion; the lead-in
must appear exactly once; every claim cross-checks against FR-M13/FR-M14). The only residual risk is
voice/register drift — the implementer might over- or under-write relative to the siblings — which is
mitigated by the reference draft, the explicit voice-match instruction, and the accuracy cross-check
(Level 3). Scope is airtight: one file, one insertion, no code/config/spec/README/providers touch.
The 0.5 deduction is for the subjective voice match, which the gates can verify structurally but not
fully stylistically.