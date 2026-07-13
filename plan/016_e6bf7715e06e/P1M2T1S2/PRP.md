name: "P1.M2.T1.S2 — Cross-cutting chrome-disable docs: how-it-works safety paragraph, docs/README capability index, README FAQ (FR-C1–C5, §9.28)"
description: >
  Pure Mode-B documentation task. Make the three cross-cutting overview docs honestly surface the
  chrome-disable story (FR-C1–C5, §9.28) that the manifests already implement and that the sibling
  P1.M2.T1.S1 just added to docs/providers.md. THREE markdown edits, NO code/tests/schema:
  (a) docs/how-it-works.md — append ONE verbatim sentence to the `### Safety invariant` paragraph
  (chrome-less rendering + link to providers.md#tools-disable-asymmetry);
  (b) docs/README.md — append ONE capability-index line (`Chrome-disable (v2.9)`);
  (c) README.md — append a BRIEF chrome-less mention to the existing `### Will it corrupt my repo?`
  FAQ answer (refine existing safety messaging; NO new top-level section, NO new Features row).
  Does NOT touch docs/providers.md (S1's domain, already landed), builtin.go (source of truth),
  providers/*.toml, or any Go code/test.

---

## Goal

**Feature Goal**: Sync the three cross-cutting overview docs with the chrome-disable implementation
(FR-C1–C5, §9.28, G25) so that a reader of the safety story, the docs capability index, or the project
README learns — honestly and briefly — that every built-in provider renders **chrome-less** (skills,
extensions, context files, MCP servers off) wherever the agent CLI exposes a switch (pi, claude today),
and that surfaces a provider cannot switch off are documented as tracked limitations, not hidden
assumptions. The detailed per-provider table already lives in docs/providers.md (P1.M2.T1.S1, landed);
this task points the overview surfaces at it.

**Deliverable**: Three markdown edits (no new files):
1. **docs/how-it-works.md** — ONE sentence appended to the `### Safety invariant` paragraph (line ~197).
2. **docs/README.md** — ONE bullet appended to the `## Capability index` section.
3. **README.md** — a brief (~2–3 sentence) chrome-less mention appended to the `### Will it corrupt my
   repo?` FAQ answer (line ~337).

**Success Definition**:
- The `### Safety invariant` paragraph in docs/how-it-works.md now names chrome-less rendering + the
  tracked-limitation caveat + links to `providers.md#tools-disable-asymmetry` (relative path correct).
- The docs/README.md `## Capability index` has a new `Chrome-disable (v2.9)` bullet linking to
  `providers.md#tools-disable-asymmetry`.
- The README.md `### Will it corrupt my repo?` answer has a brief chrome-less refinement (no new
  top-level section, no new Features-table row).
- `git diff --name-only` == exactly {docs/how-it-works.md, docs/README.md, README.md}.
- No Go file changed; `make test` stays green (docs edit cannot break Go tests).

## User Persona (if applicable)

**Target User**: A developer reading the *overview* docs (how-it-works safety section, docs README
index, or the project README FAQ) — NOT the detailed providers.md table — who wants to know "is it safe
/ isolated / deterministic to let stagecoach call my agent for a commit message?"

**Use Case**: "I run stagecoach with my Claude Code / pi / codex set up with MCP servers and skills.
When stagecoach calls the agent for a commit message, does it spin up my MCP servers and inject my
skills/extensions?" → the overview docs now answer: chrome is off where the agent allows it; gaps are
documented, not assumed; details in providers.md.

**User Journey**: Open README.md → FAQ "Will it corrupt my repo?" → now also notes the call is
chrome-less where the agent allows it, with a link → (optionally) follow to docs/providers.md for the
per-provider table. OR open docs/how-it-works.md → Safety invariant → now states chrome-less rendering
+ links providers.md. OR open docs/README.md → Capability index → new Chrome-disable entry.

**Pain Points Addressed**: FR-C5's "verification + tracking duty" — chrome status must be surfaced
honestly across the docs, not only in the providers table. Today the three overview surfaces cover
mutation safety only; a reader cannot see the chrome story without finding the providers table.

## Why

- **FR-C5 / §9.28 / §12.7.1**: chrome is a SEPARATE axis from mutation safety. The manifests
  (P1.M1.T1.S1) implement it; docs/providers.md (P1.M2.T1.S1, landed) tabulates it; this task makes the
  cross-cutting overview surfaces honestly state it and point at the detail. Without this, the safety
  story in how-it-works/README is materially incomplete (it implies read-only = isolated, which is false
  for chrome).
- **Discoverability**: a user who reads only the README FAQ or the how-it-works safety section currently
  learns nothing about chrome. The fix adds a one-sentence/brief-mention pointer so they can find the
  detail without reading manifest source comments.
- **Bounded scope**: three small markdown edits, no code, no tests, no schema. The source of truth
  (builtin.go) and the detail table (providers.md) already exist and are verified; this task records and
  links, it does not re-specify.

## What

**User-visible behavior**: the three overview docs now mention chrome-less rendering and link to the
providers.md detail. No runtime behavior change.

**Technical change** (three markdown edits; verbatim content for (a)+(b), crafted-brief for (c)):

### Edit (a) — docs/how-it-works.md (append ONE sentence to the `### Safety invariant` paragraph)
After the existing sentence *…it never runs `git add`, `git commit`, or any write command.* (line ~197),
append (same paragraph):
> Every provider also renders chrome-less — skills, extensions, context files, and MCP servers are disabled wherever the agent CLI exposes a switch for them (pi and claude today); surfaces a provider cannot switch off are documented as tracked limitations rather than hidden assumptions. See [providers.md](providers.md#tools-disable-asymmetry) for the per-provider chrome-disable details.

### Edit (b) — docs/README.md (append ONE bullet to `## Capability index`)
> - **Chrome-disable (v2.9)** → [providers.md#tools-disable-asymmetry](providers.md#tools-disable-asymmetry) — every provider renders chrome-less where the agent CLI allows it.

### Edit (c) — README.md (append a BRIEF chrome-less mention to `### Will it corrupt my repo?`)
After the existing orphan-self-exit paragraph (the FAQ answer's final paragraph, line ~343), append a
short paragraph (~2 sentences), e.g.:
> And the commit-message call itself is **chrome-less** where the agent allows it — skills, extensions, context files, and MCP servers are switched off (pi, claude), so nothing loads, spawns, or injects around the call; providers that expose no such switch document the gap instead ([docs/providers.md](docs/providers.md#tools-disable-asymmetry)).

### Success Criteria
- [ ] docs/how-it-works.md `### Safety invariant` paragraph ends with the chrome-less sentence + providers.md link
- [ ] docs/README.md `## Capability index` has the new `Chrome-disable (v2.9)` bullet
- [ ] README.md `### Will it corrupt my repo?` answer has a brief chrome-less mention linking docs/providers.md
- [ ] The link target `#tools-disable-asymmetry` resolves (S1's `## Tools-disable asymmetry` heading is landed)
- [ ] Only the 3 files changed; no Go/test/schema/providers.md edit
- [ ] No NEW top-level README section and no NEW README Features-table row

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim text for edits (a) and (b) is in the contract/PRP; the crafted-brief wording and
exact placement for edit (c) is specified (with an example); the exact line locations of all three edit
points are given (by grep anchor, since lines drift); the chrome-surface facts the wording encodes are
verified against builtin.go's CHROME-DISABLE notes (findings §2); the version-label question (v2.9) is
resolved (findings §6 — use it verbatim, it's intentional); and the scope fences are explicit (3 files,
no providers.md, no code).

### Documentation & References

```yaml
# MUST READ — the authoritative research (verbatim edits, value verification, version-label resolution, scope fences)
- docfile: plan/016_e6bf7715e06e/P1M2T1S2/research/findings.md
  why: "§1 confirms S1 is ALREADY APPLIED (providers.md has the Chrome-disable column + asymmetry bullet;
        the #tools-disable-asymmetry anchor exists → the links resolve). §2 = the verified chrome facts
        the wording encodes (pi/claude switch off; 5 read-only providers document the gap; MCP-at-startup
        is pi's tracked limitation). §3/§4/§5 = exact edit locations + verbatim/crafted text. §6 = the
        v2.9 label is intentional (new, monotonic, contract-specified). §7 = scope fences."
  critical: "§6: do NOT 'correct' (v2.9) to v2.1/v2.4 — it is the contract's deliberate label for this
             P2/G25 feature batch (the docs' highest existing label is v2.4 for commit hooks). §5: the
             README edit goes in the FAQ 'Will it corrupt my repo?' answer, NOT a new section/row."

# MUST READ — the sibling PRP (the detail table this task links to; CONTRACT)
- docfile: plan/016_e6bf7715e06e/P1M2T1S1/PRP.md
  why: "Defines the docs/providers.md output these links point at: the 9-column Chrome-disable table +
        the 'Chrome is a separate axis' asymmetry bullet under the existing '## Tools-disable asymmetry'
        heading (→ anchor #tools-disable-asymmetry). S1 is already landed in the working tree (findings §1)."
  critical: "Do NOT edit docs/providers.md — that is S1's exclusive domain and it is already complete.
             This task only LINKS to it from the overview surfaces."

# MUST READ — the files being edited (locate edit points by content via grep, not line numbers)
- file: docs/how-it-works.md
  why: "The '### Safety invariant' heading is at line 195; the paragraph to extend is at line 197
        ('No provider mutates the repository (PRD §18.1)…'). LOCATE via
        `grep -n '### Safety invariant' docs/how-it-works.md`. Append the chrome sentence to the END of
        that paragraph (after '…any write command.')."
  pattern: "The paragraph is one long line (multi-sentence). Append the new sentence to the same
            paragraph. The relative link `providers.md#tools-disable-asymmetry` is correct (both files
            are in docs/)."
  gotcha: "Line numbers drift — locate by content. There is NO existing chrome mention in how-it-works.md
           (only an unrelated 'bare mode on every provider' at line 326 in the work-description section),
           so this is a clean APPEND, not a rewrite."

- file: docs/README.md
  why: "The '## Capability index' section sits after the '## Documentation index' table. Existing entries
        use the format `- **Name** → [doc.md#anchor](doc.md#anchor) — description` (RELATIVE paths, no
        `docs/` prefix — docs/README.md lives in docs/). LOCATE via `grep -n '## Capability index'`."
  pattern: "Append the new bullet as the LAST entry in the capability-index list. Keep the existing
            entries (Payload exclusions, Message shaping, Git hook mode, Tool integrations, --edit/--push,
            Discovery, Concurrency & lock reclamation) UNCHANGED."
  gotcha: "The existing capability-index entries carry NO version labels; '(v2.9)' is a deliberate
           contract deviation (findings §6) — keep it. The relative link `providers.md#…` is correct."

- file: README.md
  why: "The FAQ '### Will it corrupt my repo?' heading is at line 337; its answer is PURELY mutation
        safety (atomic snapshot, never touches live index, per-repo run lock, orphan self-exit). LOCATE
        via `grep -n 'Will it corrupt my repo' README.md`. The chrome mention is appended AFTER the
        answer's final (orphan-self-exit) paragraph."
  pattern: "The README FAQ uses short titled subsections (### Question) with 1–3 paragraphs each. Add ONE
            short paragraph (~2 sentences) at the end of the 'Will it corrupt my repo?' answer. Links use
            `docs/providers.md#…` (README.md is at repo root, so the `docs/` prefix IS needed here —
            unlike the docs/README.md entry which omits it)."
  gotcha: "Do NOT add a new top-level (##) section and do NOT add a new Features-table row. The contract
           forbids both. This is a REFINEMENT of the existing 'Will it corrupt my repo?' answer. There is
           no chrome mention anywhere in README.md today (grep confirmed), so this is a clean APPEND."

# MUST READ — the source of truth the wording records (CHROME-DISABLE notes, P1.M1.T1.S1)
- file: internal/provider/builtin.go
  why: "Each provider's CHROME-DISABLE note (pi@43, claude@120, agy@212, qwen-code@266, opencode@310,
        codex@363, cursor@407) is the verified per-provider chrome status. The wording in all three edits
        MUST agree: pi/claude switch chrome off; the five read-only providers document the limitation;
        MCP-at-startup is pi's specific tracked gap. Read them if any wording is questioned."
  critical: "Do NOT edit builtin.go. It is the source of truth this doc RECORDS (Mode B). The contract
             wording is already verified-accurate against these notes (findings §2)."

# MUST READ — the PRD spec (the requirement these docs surface)
- docfile: plan/016_e6bf7715e06e/prd_snapshot.md
  why: "§9.28 (FR-C1–C5) defines chrome surfaces {skills, extensions/prompt-templates, context files,
        MCP servers} and the disable/document discipline. §12.7.1 consequence #4 is the 'chrome is a
        separate axis from mutation safety' statement. The edits cite FR-C1–C5 / §9.28."
  section: "§9.28 Chrome-disable for every provider; §12.7.1 The tools-disable asymmetry"

# CONTEXT — architecture overview (which surfaces this task owns vs S1)
- docfile: plan/016_e6bf7715e06e/architecture/system_context.md
  why: "The 'Documentation surfaces' section maps docs/providers.md → S1 and the cross-cutting overview
        surfaces (how-it-works.md, docs/README.md, README.md) → this task (M2.T1.S2). Confirms the split."
  critical: "It confirms docs/providers.md is NOT this task (S1 owns it)."
```

### Current Codebase tree (relevant slice)

```bash
docs/
  how-it-works.md     # EDIT (a) — +1 sentence in the `### Safety invariant` paragraph (line ~197)
  README.md           # EDIT (b) — +1 bullet in the `## Capability index` section
  providers.md        # READ-ONLY (S1 landed it: Chrome-disable column + asymmetry bullet; the link target)
internal/provider/
  builtin.go          # READ-ONLY — CHROME-DISABLE notes are the source of truth (pi@43 claude@120 ...)
README.md             # EDIT (c) — +brief chrome-less mention in `### Will it corrupt my repo?` FAQ (line ~337)
.markdownlint.json    # READ-ONLY — {MD013:false (line length OFF), MD033:false, MD060:false}; no make target for docs
Makefile              # READ-ONLY — `make lint` is golangci-lint (Go only); docs not linted by make
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (no new files):
docs/how-it-works.md   # +1 sentence appended to the `### Safety invariant` paragraph
docs/README.md         # +1 bullet appended to the `## Capability index`
README.md              # +1 brief paragraph appended to `### Will it corrupt my repo?`
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (relative-link paths differ by file): docs/how-it-works.md and docs/README.md live INSIDE
     docs/, so their links use `providers.md#tools-disable-asymmetry` (NO docs/ prefix). README.md lives
     at the REPO ROOT, so its link MUST use `docs/providers.md#tools-disable-asymmetry` (WITH the docs/
     prefix). Getting this backwards produces a broken link in exactly one of the three files. -->

<!-- CRITICAL (the v2.9 label is intentional — do not "fix" it): a repo-wide grep shows "v2.9" appears
     nowhere else (the docs' highest label is v2.4 for commit hooks). The contract deliberately
     introduces "(v2.9)" for this P2/G25 (§9.28) chrome-disable batch. Keep it verbatim. -->

<!-- CRITICAL (README edit = REFINEMENT, not a new section): the contract explicitly forbids a new
     top-level (##) section AND a new Features-table row. Add the chrome mention ONLY to the existing
     `### Will it corrupt my repo?` FAQ answer (append a short paragraph). -->

<!-- GOTCHA (locate edit points by content, not line numbers): lines drift across parallel tasks.
     - how-it-works.md: grep -n '### Safety invariant' → the heading; the paragraph is the next line.
     - docs/README.md: grep -n '## Capability index' → the section.
     - README.md: grep -n 'Will it corrupt my repo' → the FAQ heading. -->

<!-- GOTCHA (the anchor resolves because S1 is landed): the link target `#tools-disable-asymmetry` is
     GitHub's anchor for the existing `## Tools-disable asymmetry` heading in docs/providers.md. S1
     PRESERVES that heading (it only adds a column + a bullet) — verified live (findings §1). Do not
     reword the heading. -->

<!-- GOTCHA (Mode B — record, don't re-specify): the wording must AGREE with the builtin.go CHROME-DISABLE
     notes (findings §2). If a fact seems wrong, the fix is in the manifest (a different task), not here.
     The contract wording is already verified-accurate; transcribe it faithfully. -->

<!-- GOTCHA (docs are NOT in make): there is no markdownlint make target (.markdownlint.json exists but
     is unwired). Validation = grep guards + manual render + scope guard. MD013 (line length) is OFF, so
     long sentences/lines are fine. -->
```

## Implementation Blueprint

### Data models and structure
None. Pure markdown. No types, no code.

### Implementation Tasks (ordered by dependencies — independent edits, do in any order)

```yaml
Task 1: EDIT docs/how-it-works.md — append the chrome-less sentence to the Safety invariant paragraph
  - LOCATE: `grep -n '### Safety invariant' docs/how-it-works.md` → heading at ~line 195; the paragraph
    is the next line (~197), ending with "…it never runs `git add`, `git commit`, or any write command."
  - APPEND (to the END of that same paragraph, after the final period, one space, then the sentence):
      "Every provider also renders chrome-less — skills, extensions, context files, and MCP servers are disabled wherever the agent CLI exposes a switch for them (pi and claude today); surfaces a provider cannot switch off are documented as tracked limitations rather than hidden assumptions. See [providers.md](providers.md#tools-disable-asymmetry) for the per-provider chrome-disable details."
  - PRESERVE: the existing mutation-safety sentences verbatim. This is an APPEND, not a rewrite.
  - LINK: relative `providers.md#tools-disable-asymmetry` (how-it-works.md is in docs/ → no docs/ prefix).

Task 2: EDIT docs/README.md — append the Chrome-disable capability-index bullet
  - LOCATE: `grep -n '## Capability index' docs/README.md` → the section (after the Documentation index
    table). The list of `- **Name** → …` bullets is the capability index.
  - APPEND (as the LAST bullet in that list, matching the existing `- **Name** → [rel](rel) — desc` format):
      "- **Chrome-disable (v2.9)** → [providers.md#tools-disable-asymmetry](providers.md#tools-disable-asymmetry) — every provider renders chrome-less where the agent CLI allows it."
  - PRESERVE: every existing capability-index bullet (Payload exclusions … Concurrency & lock reclamation).
  - LINK: relative `providers.md#…` (docs/README.md is in docs/ → no docs/ prefix).

Task 3: EDIT README.md — append a brief chrome-less mention to the "Will it corrupt my repo?" FAQ
  - LOCATE: `grep -n 'Will it corrupt my repo' README.md` → the `### Will it corrupt my repo?` heading
    (~line 337). Its answer has 2–3 paragraphs ending with the orphan-self-exit (FR-K1/watchdog) paragraph.
  - APPEND (ONE short paragraph, ~2 sentences, AFTER the answer's final paragraph):
      "And the commit-message call itself is **chrome-less** where the agent allows it — skills, extensions, context files, and MCP servers are switched off (pi, claude), so nothing loads, spawns, or injects around the call; providers that expose no such switch document the gap instead ([docs/providers.md](docs/providers.md#tools-disable-asymmetry))."
  - PRESERVE: the existing mutation-safety answer (atomic snapshot, run lock, orphan self-exit) verbatim.
  - CONSTRAINT: NO new top-level (##) section; NO new Features-table row. Refine THIS FAQ answer only.
  - LINK: `docs/providers.md#tools-disable-asymmetry` (README.md is at repo root → docs/ prefix REQUIRED).

Task 4: VERIFY — grep guards + link-target check + scope guard + render
  - grep guards + anchor check + manual render (see Validation Loop)
  - git diff --name-only  → exactly {docs/how-it-works.md, docs/README.md, README.md}
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: the relative-link path differs by file location -->
# docs/how-it-works.md AND docs/README.md (both in docs/):  ... providers.md#tools-disable-asymmetry ...
# README.md (repo root):                                    ... docs/providers.md#tools-disable-asymmetry ...

<!-- PATTERN: each edit is a clean APPEND to existing content (no rewrite) -->
# how-it-works.md: <existing mutation-safety paragraph>. <NEW chrome sentence + link>.
# docs/README.md:  <existing bullets>\n- **Chrome-disable (v2.9)** → …
# README.md:       <existing FAQ paragraphs>\n\n<NEW short chrome paragraph + link>.

<!-- PATTERN: the load-bearing accuracy phrase -->
# "wherever the agent CLI exposes a switch for them" — does NOT claim MCP is off on pi (no switch exists);
#  the next clause "surfaces a provider cannot switch off are documented as tracked limitations" covers it.
```

### Integration Points

```yaml
NO code / tests / schema / config / routes. THREE markdown files edited (no new files).

DOCS (docs/how-it-works.md):
  - `### Safety invariant` paragraph: +1 sentence (chrome-less rendering + tracked-limitation caveat + link).

DOCS (docs/README.md):
  - `## Capability index`: +1 bullet (`Chrome-disable (v2.9)` → providers.md#tools-disable-asymmetry).

README (README.md):
  - `### Will it corrupt my repo?` FAQ answer: +1 short paragraph (chrome-less mention + link). NO new ## section.

SCOPE FENCES: NO docs/providers.md (S1 landed it); NO builtin.go / builtin_test.go / providers/*.toml
  (source of truth + mirrors); NO Go code/tests; NO PRD.md/tasks.json/prd_snapshot.md (read-only);
  NO new README top-level section or Features row.
```

## Validation Loop

> Docs are NOT linted by `make` (`.markdownlint.json` exists with MD013/MD033/MD060 OFF but is not wired
> to a make target; `make lint` is golangci-lint for Go only). Validation = grep guards + link-target
> check + manual render + scope guard.

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Scope guard: exactly the 3 doc files changed.
git diff --name-only
# Expected: docs/how-it-works.md  docs/README.md  README.md  (exactly these 3, no more).

# Confirm no Go file changed (docs-only task).
git diff --stat -- '*.go'
# Expected: empty.

# (Optional) markdownlint if installed locally — baseline green except known pre-existing.
npx --no-install markdownlint-cli2 'docs/how-it-works.md' 'docs/README.md' 'README.md' 2>/dev/null \
  || echo "(markdownlint not installed / not in make — skip; MD013 is off so long lines are fine)"
```

### Level 2: Unit Tests (Component Validation)

```bash
# No tests are authored by this task. Run the Go suite ONLY to prove the working tree is otherwise clean
# (a docs edit cannot break Go tests, but parallel work may be in flight).
make test
# Expected: green (race detector). Unaffected by these doc edits.
```

### Level 3: Integration Testing (System Validation)

```bash
# Manual render check of the three edited regions.
glow docs/how-it-works.md 2>/dev/null | sed -n '/### Safety invariant/,/### Failure modes/p' \
  || sed -n '195,199p' docs/how-it-works.md
# Expected: the Safety invariant paragraph now ends with the chrome-less sentence + a rendered
#           providers.md link; the following "### Failure modes" heading is intact.

sed -n '/## Capability index/,/## Product specification/p' docs/README.md
# Expected: the capability-index list now includes a `Chrome-disable (v2.9)` bullet linking
#           providers.md#tools-disable-asymmetry; the existing bullets are unchanged.

sed -n '/### Will it corrupt my repo/,/### Does it send my code/p' README.md
# Expected: the FAQ answer now ends with a short chrome-less paragraph linking docs/providers.md;
#           the existing mutation-safety paragraphs are intact; NO new ## section appears.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1 (how-it-works.md): the chrome sentence + link are present.
grep -c 'renders chrome-less' docs/how-it-works.md
grep -c 'providers.md#tools-disable-asymmetry' docs/how-it-works.md
# Expected: each == 1.

# Grep guard 2 (docs/README.md): the capability-index bullet is present.
grep -c 'Chrome-disable (v2.9)' docs/README.md
grep -c 'providers.md#tools-disable-asymmetry' docs/README.md
# Expected: each == 1.

# Grep guard 3 (README.md): the chrome mention + link are present, with the docs/ prefix.
grep -c 'chrome-less' README.md
grep -c 'docs/providers.md#tools-disable-asymmetry' README.md
# Expected: each >= 1.

# Grep guard 4 (link-target consistency — the prefix differs by file location):
#   docs/* files use the BARE relative path; README.md uses the docs/-prefixed path.
grep -c ']providers.md#tools-disable-asymmetry' docs/how-it-works.md docs/README.md  # bare (no docs/)
grep -c 'docs/providers.md#tools-disable-asymmetry' README.md                        # prefixed
# (Adjust the regex to the actual markdown link text; the point is: docs/* = bare, README.md = prefixed.)
# A quick eyeball: `grep -n 'providers.md#tools-disable-asymmetry' docs/how-it-works.md docs/README.md README.md`
# should show the two docs/ files with "(providers.md#…" and README.md with "(docs/providers.md#…".

# Grep guard 5 (anchor target exists — S1 landed it):
grep -c '## Tools-disable asymmetry' docs/providers.md
# Expected: == 1 (the heading whose GitHub anchor is #tools-disable-asymmetry — the links resolve).

# Grep guard 6 (preservation — existing content intact):
grep -c 'No provider mutates the repository' docs/how-it-works.md   # mutation-safety sentence kept
grep -c '## Capability index' docs/README.md                        # section heading kept
grep -c 'Will it corrupt my repo' README.md                         # FAQ heading kept
grep -c 'Stagecoach uses .git write-tree' README.md                 # existing FAQ answer kept
# Expected: each >= 1.

# Grep guard 7 (scope — no other file changed, esp. not providers.md or any .go).
git diff --name-only | grep -Ev 'docs/how-it-works.md|docs/README.md|README.md'
# Expected: empty (no output).
git diff --name-only | grep -E 'providers.md|\.go$|builtin'
# Expected: empty (S1's file + all Go untouched).
```

## Final Validation Checklist

### Technical Validation
- [ ] `git diff --name-only` == exactly {docs/how-it-works.md, docs/README.md, README.md}
- [ ] `git diff --stat -- '*.go'` empty (docs-only)
- [ ] `make test` green (working tree otherwise clean — no behavioral regression)

### Feature Validation
- [ ] docs/how-it-works.md `### Safety invariant` paragraph ends with the chrome-less sentence + providers.md link
- [ ] docs/README.md `## Capability index` has the `Chrome-disable (v2.9)` bullet linking providers.md#tools-disable-asymmetry
- [ ] README.md `### Will it corrupt my repo?` answer ends with a brief chrome-less paragraph linking docs/providers.md#tools-disable-asymmetry
- [ ] Link paths correct per file location (docs/* bare; README.md docs/-prefixed)
- [ ] The `#tools-disable-asymmetry` anchor target exists in docs/providers.md (S1 landed)

### Scope-Boundary Validation
- [ ] Only the 3 overview docs changed
- [ ] docs/providers.md UNCHANGED (S1's domain, already landed)
- [ ] No Go code / test / builtin.go / providers/*.toml change
- [ ] NO new README top-level (##) section
- [ ] NO new README Features-table row
- [ ] Existing Safety-invariant paragraph / capability-index bullets / FAQ answer preserved verbatim

### Code Quality & Docs
- [ ] Wording agrees with the builtin.go CHROME-DISABLE notes (Mode B records the verified state)
- [ ] "(v2.9)" kept verbatim (contract-specified, intentional — findings §6)
- [ ] The load-bearing phrase "wherever the agent CLI exposes a switch" is preserved (does not overclaim MCP-off on pi)
- [ ] Edit points located by content (grep), not stale line numbers

---

## Anti-Patterns to Avoid

- ❌ Don't get the relative-link prefix wrong. docs/how-it-works.md and docs/README.md live INSIDE docs/,
  so their links are `providers.md#tools-disable-asymmetry` (no prefix). README.md lives at the repo
  ROOT, so its link MUST be `docs/providers.md#tools-disable-asymmetry` (with prefix). Getting this
  backwards breaks exactly one link. (Grep guard 4 catches it.)
- ❌ Don't "correct" `(v2.9)` to v2.1/v2.4. A repo-wide grep shows v2.9 appears nowhere else, but the
  contract deliberately introduces it for this P2/G25 (§9.28) batch — it is monotonic (v2.9 > v2.4) and
  intentional (findings §6). Transcribe it verbatim.
- ❌ Don't add a NEW README top-level (##) section or a NEW Features-table row. The contract explicitly
  forbids both. The README chrome mention is a REFINEMENT of the existing `### Will it corrupt my repo?`
  FAQ answer — append one short paragraph, nothing more.
- ❌ Don't rewrite the existing mutation-safety text. All three edits are APPENDS: the how-it-works
  Safety-invariant sentence goes AFTER the existing mutation-safety sentences; the docs/README bullet is
  a NEW list item (existing bullets untouched); the README paragraph goes AFTER the existing FAQ answer.
- ❌ Don't edit docs/providers.md. S1 owns it and it is ALREADY LANDED in the working tree (Chrome-disable
  column + asymmetry bullet; the `## Tools-disable asymmetry` heading → `#tools-disable-asymmetry` anchor
  that these links target). This task only LINKS to it.
- ❌ Don't edit builtin.go, builtin_test.go, providers/*.toml, or any Go file. They are the source of
  truth + mirrors + tests; this is a docs-only task. The wording RECORDS the verified manifest state
  (Mode B).
- ❌ Don't overclaim that MCP is off on pi. The load-bearing phrase is "wherever the agent CLI exposes a
  switch for them" — pi has NO `--no-mcp` (servers may connect at startup). The contract sentence's next
  clause ("surfaces a provider cannot switch off are documented as tracked limitations") is what covers
  that gap. Keep the wording faithful; do not simplify it to "all chrome is off".
- ❌ Don't anchor to line numbers. Lines drift across parallel tasks. Locate each edit point by content:
  `grep -n '### Safety invariant'`, `grep -n '## Capability index'`, `grep -n 'Will it corrupt my repo'`.
- ❌ Don't cite a different PRD section. The chrome-disable requirement is §9.28 (FR-C1–C5); the
  separate-axis statement is §12.7.1 consequence #4. The edits reference these (and §18.1 is the
  mutation-safety invariant, already cited in the existing how-it-works sentence — leave that alone).

---

## Confidence Score: 10/10

This is a three-edit markdown change to three overview docs, with verbatim text for edits (a) and (b)
supplied by the contract, crafted-brief wording and exact placement for edit (c), the edit points
located by content grep, the chrome-surface facts verified against builtin.go's CHROME-DISABLE notes
(findings §2), the version-label question resolved (v2.9 intentional, findings §6), and the link-target
anchor confirmed live (S1 is already landed — findings §1). The two non-obvious traps — the
relative-link prefix differing by file location (docs/* bare vs README.md prefixed) and the README
"refine, don't add a section" constraint — are both spelled out with grep guards. No code, no tests, no
schema. One-pass success is essentially guaranteed.
