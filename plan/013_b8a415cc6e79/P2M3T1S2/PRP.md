name: "P2.M3.T1.S2 — Fix verification date and provider count in docs/providers.md"
description: |
  Mode A documentation fix (five in-place edits, one optional). Correct the stale **#76 verification
  date** (audit D3) and the stale **provider count "8"→"7"** (audit D4–D7) in `docs/providers.md` so
  the surrounding prose matches both the 7-row quick-reference table at lines 80–85 and the source of
  truth (`internal/provider/builtin.go` = 7 built-ins; PRD §12.5.1.1 re-verified **2026-07-08**).

  Five edits — four REQUIRED (mechanical) + one OPTIONAL accuracy reframe:
    - **D4** (line 3):  `the 8 built-in providers`            → `the 7 built-in providers`
    - **D5** (line 7):  `Eight providers are compiled in…`     → `Seven providers are compiled in…`
    - **D6** (line 74): `## The 8 built-in providers`          → `## The 7 built-in providers`
    - **D3** (line 88): `…no longer reproduces as of **2026-07-03**` → `…no longer reproduces as of **2026-07-08**`
    - **D7** (line 92): `The eight built-in providers achieve…` → `The seven built-in providers achieve…`
    - (OPTIONAL, line 88 reframe): align the "pending" clause with PRD §12.5.1.1 — items 1–3 (incl.
      #76) are RESOLVED; agy is experimental solely pending item 4 (stager/tooled flags).

  Out of scope (leave for siblings): line 85 agy table-row cells = P2.M3.T1.S1; `docs/README.md`
  counts (D8/D9) = P2.M3.T2. No `.go` / `PRD.md` / `tasks.json` / `prd_snapshot.md` / `.gitignore`.

---

## Goal

**Feature Goal**: Make `docs/providers.md` internally consistent (prose count = table count = 7) and
factually current (the agy #76 re-verification date = 2026-07-08) by fixing the four stale
"8"/"eight" provider-count references (audit D4–D7) and the one stale "2026-07-03" date (audit D3).

**Deliverable**: Five surgical in-place edits to `docs/providers.md` (lines 3, 7, 74, 88, 92). After
the edits, no occurrence of `8 built`, `eight built`, `Eight providers`, or `2026-07-03` remains in
the file; the 7-row table at lines 80–85 is no longer contradicted by its own heading/prose.

**Success Definition**: All of the following hold after the edits:
1. `grep -nE '8 built|eight built|Eight providers|2026-07-03' docs/providers.md` → **zero matches**.
2. `grep -nE '7 built|Seven providers|seven built|2026-07-08' docs/providers.md` → **exactly 5 lines**
   (3, 7, 74, 88, 92), each matching its target literal in this PRP.
3. The quick-reference table (lines 80–85) is byte-for-byte unchanged (7 rows; this subtask edits none
   of them — line 85 is sibling S1's domain).
4. `docs/README.md` is unchanged (D8/D9 = sibling P2.M3.T2).
5. No `.go` source changed; `go build ./...` and `go test ./...` stay green (docs-only invariant).

## User Persona (if applicable)

**Target User**: A stagecoach user / integrator reading `docs/providers.md` to learn how many providers
ship out of the box, how they are auto-detected, and whether agy is stable/experimental.

**Use Case**: Skimming the intro + "The 7 built-in providers" section + the table to grasp the lineup,
then trusting that the heading count matches the table rows and the agy status note is current.

**User Journey**: Open `docs/providers.md` → read line 3 (summary), line 7 (what a manifest is),
line 74 (section heading), the 7-row table, line 88 (agy/qwen-code experimental note), line 92
(tools-disable asymmetry) → see **7** everywhere it should be 7 and the **2026-07-08** date.

**Pain Points Addressed**: Today the doc self-contradicts — the heading says "The **8** built-in
providers" directly above a table that lists exactly **7** rows, and three more prose lines repeat
"8"/"eight". The agy #76 note also cites a stale date (2026-07-03) that predates the v1.1.0
re-verification (2026-07-08). A reader is left unsure whether the lineup is 7 or 8 and whether the
agy status is current. This fix removes the contradiction.

## Why

- **Docs ↔ code ↔ PRD parity (the P2 milestone's whole point).** The provider lineup correction
  removed the EOL `gemini` built-in (superseded by `agy` on 2026-06-18), dropping the count 8 → 7,
  and re-verified agy against v1.1.0 on 2026-07-08. The compiled manifest (`builtin.go`,
  `BuiltinManifests()` = 7), the registry (`registry.go` preferredBuiltins = 7), and PRD §12.5.1/§12.5.1.1
  all reflect this. `docs/providers.md` prose was NOT swept in that change — the docs-drift audit
  flagged it as items D3–D7. This subtask closes that residual gap.
- **Internal consistency is the user-facing value (Mode A).** A doc whose heading says 8 over a 7-row
  table erodes trust in every other number on the page. The fix is the documentation update itself.
- **Lowest-risk change class.** Five one-line prose edits, no code, no schema, no behavior, no table
  structure touched. Risk surface = "did I get all 4 count sites + the 1 date, and did I avoid the
  sibling-owned line 85 and the sibling-owned file docs/README.md" — fully covered by deterministic grep.

## What

Five in-place edits to `docs/providers.md` (no lines added/removed; no table cell touched). The edits
are mechanical string swaps on prose/heading lines only:

| # | Line | Current (drifted) | Corrected |
|---|------|-------------------|-----------|
| **D4** | 3 | `…the 8 built-in providers, the tools-disable asymmetry…` | `…the 7 built-in providers, the tools-disable asymmetry…` |
| **D5** | 7 | `Eight providers are compiled in as built-ins (zero config needed).` | `Seven providers are compiled in as built-ins (zero config needed).` |
| **D6** | 74 | `## The 8 built-in providers` | `## The 7 built-in providers` |
| **D3** | 88 | `…no longer reproduces as of **2026-07-03**…` | `…no longer reproduces as of **2026-07-08**…` |
| **D7** | 92 | `The eight built-in providers achieve tool-safety…` | `The seven built-in providers achieve tool-safety…` |

**OPTIONAL accuracy reframe (line 88).** The current line 88 phrase "pending the remaining §12.5.1.1
checklist items (the non-TTY stdout drop, issue #76, no longer reproduces…)" is *slightly* inaccurate:
PRD §12.5.1.1 clears items 1–3 (incl. #76) and states agy is experimental **solely pending item 4**
(the tooled/stager flag combo). If the implementer opts in, replace the agy clause with the PRP-quoted
literal in Task 5 below. The minimal REQUIRED change is the date-only swap (D3).

### Success Criteria

- [ ] All four count references (lines 3, 7, 74, 92) read "7"/"Seven"/"seven" (audit D4–D7 resolved).
- [ ] Line 88 reads "as of **2026-07-08**" (audit D3 resolved); optional reframe applied only if chosen.
- [ ] Zero matches for `8 built|eight built|Eight providers|2026-07-03` in `docs/providers.md`.
- [ ] Exactly 5 lines match `7 built|Seven providers|seven built|2026-07-08` (the 5 edited lines).
- [ ] The 7-row table (lines 80–85) is unchanged — **line 85 NOT touched by this subtask** (sibling S1).
- [ ] `docs/README.md` unchanged (D8/D9 = sibling P2.M3.T2).
- [ ] No `.go` / `PRD.md` / `tasks.json` / `prd_snapshot.md` / `.gitignore` modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** Each edit is fully specified by a verbatim current→desired literal, and the
"7" count and "2026-07-08" date are pinned by three mutually-consistent sources of truth (compiled
manifest comment + `BuiltinManifests()` funcs, registry preferredBuiltins, PRD §12.5.1.1). No live
binary, no Go knowledge, and no external docs are required — read the 5 lines, apply the 5 string
swaps, run the grep gate.

### Documentation & References

```yaml
# MUST READ — the single file being edited (Mode A target = docs/providers.md)
- file: docs/providers.md
  why: Contains the 4 stale "8"/"eight" provider-count references (lines 3, 7, 74, 92) and the 1 stale
       #76 verification date (line 88). The 7-row quick-reference table is at lines 79–86.
  pattern: Read lines 1–10, 74–95 before editing to see the count/date in context and confirm the table
           has exactly 7 data rows (so the "7" correction is self-evidently right).
  gotcha: Line 85 (agy table row) is OUT OF SCOPE — sibling S1 edits it in parallel. Line 88 spans BOTH
          providers (agy + qwen-code) — only the agy date token is swapped; the qwen-code clause is
          unchanged. Do NOT touch docs/README.md (sibling P2.M3.T2 owns D8/D9).

# Source of truth 1 — the compiled manifest (READ-ONLY; the doc count must mirror this)
- file: internal/provider/builtin.go
  why: Confirms the built-in count is 7. Line 17 comment: "Seven providers: pi, claude, opencode, codex,
       cursor, agy, qwen-code." BuiltinManifests() returns 7 funcs (builtinPi/Claude/Agy/QwenCode/
       OpenCode/Codex/Cursor). grep -c builtinGemini == 0 (gemini removed).
  section: "line 17 comment + func BuiltinManifests() at line 18"
  pattern: // ...Seven providers: pi, claude, opencode, codex, cursor, agy, qwen-code.

# Source of truth 2 — the registry (READ-ONLY; the auto-detect order is 7 entries)
- file: internal/provider/registry.go
  why: preferredBuiltins (line 16) lists exactly 7: pi, opencode, cursor, agy, qwen-code, codex, claude.
  section: "var preferredBuiltins = []string{...} (line 16)"

# Source of truth 3 — the spec (READ-ONLY; human-owned; the doc date must mirror this)
- file: PRD.md
  why: §12.5.1.1 heading reads "verified 2026-07-08 against agy v1.1.0"; item 1 (#76) "no longer
       reproduces on v1.1.0"; "Items 1–3 are cleared; agy ships experimental ... solely pending item 4."
       This pins the 2026-07-08 date and the optional reframe.
  section: "§12.5.1.1 (heading `#### 12.5.1.1 Status (agy)`)"
  gotcha: Anchor the date check to §12.5.1.1, NOT to §12.1 (which has a generic schema-example toml with
          placeholder values). The verification date lives only in the §12.5.1.1 status block.

# The audit that named these five drifts (READ-ONLY; defines D3–D7 verbatim)
- docfile: plan/013_b8a415cc6e79/architecture/docs_drift_audit.md
  why: §1b names D3 (line 88 date 2026-07-03→2026-07-08); §1c names D4 (line 3), D5 (line 7), D6 (line 74),
       D7 (line 92) — each with the exact current→correct text. This subtask's contract IS D3–D7.
  section: "§1b (#76 date) and §1c (provider count — 4 occurrences)"

# Sibling task context (CONTRACT — S1 owns line 85; this subtask must NOT touch it)
- docfile: plan/013_b8a415cc6e79/P2M3T1S1/PRP.md
  why: Defines the parallel edit to docs/providers.md line 85 (agy table row cells D1/D2). S1 and S2
       touch DISJOINT line ranges (85 vs 3/7/74/88/92), both in place (no add/remove), so they cannot
       collide. S2 must assert line 85 is left for S1. Do not duplicate S1's work.

# Sibling task context (CONTRACT — P2.M3.T2 owns docs/README.md D8/D9)
- docfile: plan/013_b8a415cc6e79/architecture/docs_drift_audit.md
  why: §2d flags docs/README.md line 35 (provider count 8→7 = D8, field count 21→22 = D9). That is
       sibling P2.M3.T2 — explicitly OUT OF SCOPE here. Do not touch docs/README.md.

# Research notes for this subtask
- docfile: plan/013_b8a415cc6e79/P2M3T1S2/research/source_of_truth.md
  why: Full cross-check of all three code/PRD sources + the verbatim current→desired literals for all
       5 edits + the non-overlap proof with S1 + the deterministic verification command set.
```

### Current Codebase tree (relevant slice)

```bash
# Run from repo root: cd /home/dustin/projects/stagecoach
docs/providers.md                       # ← the ONLY file edited (lines 3, 7, 74, 88, 92)
internal/provider/builtin.go            # source of truth (7 built-ins, READ-ONLY)
internal/provider/registry.go           # source of truth (7-entry preferredBuiltins, READ-ONLY)
PRD.md                                  # source of truth (§12.5.1.1 date 2026-07-08, READ-ONLY)
.github/workflows/ci.yml                # CI (NO markdownlint — gate is grep-based)
.markdownlint.json                      # markdownlint config (MD013/MD033/MD060 off)
plan/013_b8a415cc6e79/
  architecture/docs_drift_audit.md      # the audit naming D3–D7 (READ-ONLY)
  P2M3T1S1/PRP.md                       # sibling: line 85 agy row (CONTRACT — disjoint lines)
  P2M3T1S2/
    PRP.md                              # ← THIS file
    research/source_of_truth.md         # research notes
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
# NO new files. The ONLY changes are 5 in-place prose/heading edits in docs/providers.md. After edits:
docs/providers.md   # line 3:  "8 built-in" -> "7 built-in"
                    # line 7:  "Eight providers are compiled in" -> "Seven providers are compiled in"
                    # line 74: heading "## The 8 built-in providers" -> "## The 7 built-in providers"
                    # line 88: "as of **2026-07-03**" -> "as of **2026-07-08**" (+ optional reframe)
                    # line 92: "The eight built-in providers" -> "The seven built-in providers"
# Nothing else is created, deleted, or modified.
```

### Known Gotchas of our codebase & Library Quirks

```text
# GOTCHA 1 — Only the agy #76 date on line 88 is in scope; the qwen-code clause on the same line is NOT.
#   docs/providers.md line 88 is a single Note paragraph covering BOTH agy and qwen-code. The swap is
#   ONLY the agy date token "as of **2026-07-03**" -> "as of **2026-07-08**". Do not alter the qwen-code
#   sentence ("a Gemini-CLI fork for Qwen3-Coder via DashScope") — qwen-code did not get re-verified and
#   has no date token here. Use a TARGETED string replacement, not a whole-line rewrite (unless doing
#   the optional reframe, which still preserves the qwen-code clause verbatim).

# GOTCHA 2 — Line 85 is OUT OF SCOPE (sibling S1, running in parallel).
#   docs/providers.md line 85 is the agy table row. Sibling P2.M3.T1.S1 fixes its two drifted cells
#   (D1/D2). This subtask must NOT touch line 85 (or any table row). The grep gate asserts line 85 is
#   unchanged by S2. Both subtasks edit in place (no line add/remove) so line numbers stay stable.

# GOTCHA 3 — docs/README.md is OUT OF SCOPE (sibling P2.M3.T2).
#   docs/README.md line 35 also has a stale "8 built-in providers" (D8) and a "21-field" count (D9).
#   Those are sibling P2.M3.T2 — do NOT fix them here. The gate asserts docs/README.md is untouched.

# GOTCHA 4 — "8"/"eight" appears in FOUR separate places; catch all four.
#   The drift is not a single token. It is 4 independent sites: line 3 (digits "8"), line 7 (word
#   "Eight"), line 74 (heading digits "8"), line 92 (word "eight"). A naive single find/replace on
#   one form will miss the others. The negative grep gate (8 built|eight built|Eight providers) catches
#   all three spellings; confirm zero matches before declaring done.

# GOTCHA 5 — "Gemini" is NOT drift; do not touch any "Gemini" token.
#   docs/providers.md contains "Gemini 3.5 Flash (Low/High/Medium)" (model display labels from agy
#   models) and "Gemini-CLI fork" (qwen-code lineage). These are legitimate; the gemini→agy succession
#   removed the gemini BUILT-IN and its name, not the Gemini model-family label. (Audit §1d confirms.)

# GOTCHA 6 — markdownlint is configured but NOT enforced in CI.
#   .markdownlint.json sets default:true with MD013/MD033/MD060 off. .github/workflows/ci.yml runs only
#   `go test` + golangci-lint (no markdownlint step). The authoritative gate is the grep check below.

# GOTCHA 7 (optional reframe only) — PRD §12.5.1.1 is the authority for the "solely pending item 4" claim.
#   If applying the optional line 88 reframe, anchor "items 1–3 resolved; experimental solely pending
#   item 4 (stager/tooled flags)" to PRD §12.5.1.1, which states exactly that. Do not invent new status.
```

## Implementation Blueprint

### Data models and structure

None. This is a documentation edit — no data models, schemas, code, config, or table structure are
touched. The edits are prose/heading string swaps on non-table lines only.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: READ the target sites in context to confirm exact current text and line numbers
  - RUN: sed -n '1,10p;74,95p' docs/providers.md
  - CONFIRM: line 3 "the 8 built-in providers"; line 7 "Eight providers are compiled in as built-ins";
             line 74 heading "## The 8 built-in providers"; the 7-row table at lines 80–85; line 88
             Note (agy + qwen-code, with "as of **2026-07-03**"); line 92 "The eight built-in providers".
  - NOTE: this read-only step prevents editing the wrong line and confirms the table has 7 rows (so the
          "7" correction is self-evidently right). Confirm line 85 is present (do not edit it — S1 owns it).

Task 2: EDIT line 3 (audit D4) — provider count digit
  - OLD: the 8 built-in providers, the tools-disable asymmetry
  - NEW: the 7 built-in providers, the tools-disable asymmetry
  - SCOPE: swap the single digit "8" -> "7" in "the 8 built-in providers" only. Leave the rest of line 3
           (incl. "22-field schema") byte-for-byte unchanged.

Task 3: EDIT line 7 (audit D5) — provider count word
  - OLD: Eight providers are compiled in as built-ins
  - NEW: Seven providers are compiled in as built-ins
  - SCOPE: swap the word "Eight" -> "Seven" at the start of that sentence only.

Task 4: EDIT line 74 (audit D6) — section heading
  - OLD: ## The 8 built-in providers
  - NEW: ## The 7 built-in providers
  - SCOPE: heading only; swap digit "8" -> "7".

Task 5: EDIT line 88 (audit D3 + OPTIONAL reframe) — #76 verification date
  - REQUIRED (D3, minimal): swap the date token only.
      OLD fragment: no longer reproduces as of **2026-07-03**
      NEW fragment: no longer reproduces as of **2026-07-08**
  - OPTIONAL reframe (recommended for accuracy; aligns with PRD §12.5.1.1):
      OLD clause: `agy` is **experimental** (PRD §12.5.1) pending the remaining §12.5.1.1 checklist items
                  (the non-TTY stdout drop, issue #76, no longer reproduces as of **2026-07-03**) and
                  cannot serve as a stager (empty `tooled_flags`).
      NEW clause: `agy` is **experimental** (PRD §12.5.1) — the §12.5.1.1 verification items 1–3
                  (including the non-TTY stdout drop, issue #76, no longer reproduces as of **2026-07-08**)
                  are resolved; it ships experimental solely pending item 4 (the tooled/stager flag combo)
                  and cannot serve as a stager (empty `tooled_flags`).
  - SCOPE: the agy clause ONLY. The qwen-code sentence on the same line ("`qwen-code` is **experimental**
           (PRD §12.5.2) — a Gemini-CLI fork for Qwen3-Coder via DashScope — and cannot serve as a stager
           (empty `tooled_flags`).") MUST stay unchanged. Pick REQUIRED or OPTIONAL; do not do both.

Task 6: EDIT line 92 (audit D7) — provider count word (Tools-disable asymmetry section)
  - OLD: The eight built-in providers achieve tool-safety via two distinct mechanisms (PRD §12.7.1):
  - NEW: The seven built-in providers achieve tool-safety via two distinct mechanisms (PRD §12.7.1):
  - SCOPE: swap the word "eight" -> "seven" at the start of that sentence only.

Task 7: VERIFY — run the deterministic grep gate (Validation Loop Level 2) and confirm all checks pass
  - RUN: the negative-grep + positive-grep + table-integrity + sibling-integrity commands (see below).
  - EXPECT: zero matches for the stale tokens; exactly 5 lines for the corrected tokens; table rows
            80–85 unchanged; line 85 untouched; docs/README.md untouched; no .go modified; build green.
  - GATE: every check must pass before declaring the subtask done.
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: all four count corrections are case-matched to their current spelling.
     - Line 3 + line 74 use the DIGIT "8" -> "7".
     - Line 7 uses the capitalized word "Eight" -> "Seven" (sentence start).
     - Line 92 uses the lowercase word "eight" -> "seven" (mid-sentence phrasing "The eight ...").
     Match the exact casing of each site; do not normalize (e.g. don't turn "eight" into "8"). -->
<!-- CRITICAL: the table at lines 80–85 is ALREADY correct (7 rows). This subtask fixes the prose that
     surrounds and heads the table, not the table. The value is removing the heading-vs-table contradiction. -->
<!-- CRITICAL (optional reframe): PRD §12.5.1.1 is the single authority for "items 1–3 resolved; solely
     pending item 4". Do not restate agy status from memory; mirror §12.5.1.1 verbatim in intent. -->
```

### Integration Points

```yaml
DATABASE: none   # docs-only edit
CONFIG:   none   # no config file changed; providers/*.toml are READ-ONLY reference docs, not edited
ROUTES:   none
# The only "integration" is docs-parity: after the edits, docs/providers.md prose agrees with (a) its own
# 7-row table, (b) internal/provider/builtin.go (7 built-ins), (c) internal/provider/registry.go
# (7-entry preferredBuiltins), and (d) PRD §12.5.1.1 (date 2026-07-08). CI does not lint markdown; the
# grep gate is the authority.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach
# No table lines are edited, so MD056 (table-column-count) is unaffected. Optional markdownlint sanity
# (NOT required; not in CI):
npx --yes markdownlint-cli2 'docs/providers.md' 2>/dev/null | grep -i MD0 || echo "ok: no blocking MD issues (or tool unavailable)"
# Expected: "ok: ..." (or tool unavailable). Docs-only edits to non-table lines cannot break table lint.
```

### Level 2: Pinpoint Verification (the core gate — run after the edits)

```bash
cd /home/dustin/projects/stagecoach
# (a) NEGATIVE — all stale tokens GONE from docs/providers.md (zero matches required):
grep -nE '8 built|eight built|Eight providers|2026-07-03' docs/providers.md && echo "FAIL: stale tokens remain" || echo "ok: no stale 8/eight/2026-07-03 tokens"

# (b) POSITIVE — exactly the 5 corrected lines present:
echo "--- count-corrected lines ---"
grep -nE '7 built|Seven providers|seven built' docs/providers.md          # EXPECT: lines 3, 7, 74, 92 (4 lines)
echo "--- date-corrected lines ---"
grep -nE '2026-07-08' docs/providers.md                                   # EXPECT: line 88 (1 line)

# (c) Pinpoint each edited line reads the target literal:
sed -n '3p'  docs/providers.md | grep -F -- 'the 7 built-in providers'    >/dev/null && echo "ok: L3"   || echo "FAIL: L3"
sed -n '7p'  docs/providers.md | grep -F -- 'Seven providers are compiled in as built-ins' >/dev/null && echo "ok: L7"   || echo "FAIL: L7"
sed -n '74p' docs/providers.md | grep -F -- '## The 7 built-in providers' >/dev/null && echo "ok: L74"  || echo "FAIL: L74"
sed -n '88p' docs/providers.md | grep -F -- 'as of **2026-07-08**'        >/dev/null && echo "ok: L88"  || echo "FAIL: L88"
sed -n '92p' docs/providers.md | grep -F -- 'The seven built-in providers achieve tool-safety' >/dev/null && echo "ok: L92" || echo "FAIL: L92"

# Expected: (a) "ok: no stale ... tokens"; (b) 4 count-lines (3,7,74,92) + 1 date-line (88) = 5 total;
# (c) all five pinpoint checks print "ok". Any FAIL -> fix before proceeding.
```

### Level 3: Cross-Reference & Regression (System Validation)

```bash
cd /home/dustin/projects/stagecoach
# (d) The 7-row table is UNTOUCHED — line 85 (agy row, sibling S1) NOT modified by THIS subtask:
git diff docs/providers.md | grep -nE '^\+.*\| `agy`' && echo "FAIL: S2 touched the agy table row" || echo "ok: line 85 not modified by S2"
# (e) docs/README.md UNTOUCHED (D8/D9 = sibling P2.M3.T2):
git diff --name-only | grep -F -- 'docs/README.md' && echo "FAIL: docs/README.md touched" || echo "ok: docs/README.md untouched"
# (f) Parity proof — the doc count now mirrors the code (7 built-ins):
grep -c 'Seven providers' internal/provider/builtin.go                       # EXPECT: 1 (the line-17 comment)
grep -nE 'Seven providers' internal/provider/builtin.go | head -1            # shows the comment the doc mirrors
# (g) Docs-only invariant — NO Go source changed; build + tests stay green:
git diff --name-only | grep -E '\.go$' && echo "FAIL: Go file touched" || echo "ok: no .go changed"
go build ./... 2>&1 | tail -3          # EXPECT: clean (no errors)
go test ./...   2>&1 | tail -5         # EXPECT: all PASS (docs-only change cannot break Go tests)
# (h) No forbidden files modified:
git diff --name-only | grep -E 'PRD\.md|tasks\.json|prd_snapshot\.md|\.gitignore' && echo "FAIL: forbidden file touched" || echo "ok: no forbidden file touched"
# Expected: (d) ok; (e) ok; (f) 1; (g) no .go changed + clean build + tests pass; (h) ok.
```

### Level 4: Creative & Domain-Specific Validation

```bash
cd /home/dustin/projects/stagecoach
# Parity proof: the corrected doc date mirrors PRD §12.5.1.1, and the count mirrors the compiled manifest.
echo "--- PRD §12.5.1.1 verification date (the authority for the 2026-07-08 swap) ---"
grep -nE 'verified 2026-07-08' PRD.md | head -1                            # EXPECT: §12.5.1.1 heading line
echo "--- compiled manifest count (the authority for the 7 swap) ---"
grep -nE 'Seven providers' internal/provider/builtin.go | head -1          # EXPECT: line 17 comment
# Rendered consistency: the "## The 7 built-in providers" heading should sit directly above a 7-row table.
awk 'NR>=74 && NR<=86' docs/providers.md | grep -cE '^\| `'                # EXPECT: 7 data rows (table rows only)
# Expected: PRD date 2026-07-08 present; builtin.go "Seven providers" present; exactly 7 table data rows.
# This is the audit's core claim — the heading/prose now agree with the table AND with the code/PRD.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 2 (negative): `grep -nE '8 built|eight built|Eight providers|2026-07-03' docs/providers.md` → 0 matches.
- [ ] Level 2 (positive): exactly 5 lines (3, 7, 74, 88, 92) carry the corrected tokens.
- [ ] Level 2 (pinpoint): all five `sed -n '…p'` checks print "ok".
- [ ] Level 3 (table integrity): line 85 (agy row) NOT modified by this subtask (S1 owns it).
- [ ] Level 3 (sibling file): `docs/README.md` untouched (P2.M3.T2 owns D8/D9).
- [ ] Level 3 (invariant): no `.go` file changed; `go build ./...` and `go test ./...` green.
- [ ] Level 3 (forbidden): no `PRD.md` / `tasks.json` / `prd_snapshot.md` / `.gitignore` modified.

### Feature Validation

- [ ] The "## The 7 built-in providers" heading no longer contradicts the 7-row table beneath it.
- [ ] The intro (line 3) and "What a manifest is" (line 7) both say 7/Seven.
- [ ] The Tools-disable asymmetry section (line 92) opens with "The seven built-in providers".
- [ ] The agy #76 note (line 88) cites the current re-verification date 2026-07-08 (reframe applied only if chosen).
- [ ] Manual read confirms the doc is internally consistent (count = 7 everywhere) and current.

### Code Quality Validation

- [ ] Each count correction matches the casing of its site (digit "8"→"7" on L3/L74; "Eight"→"Seven" on L7; "eight"→"seven" on L92).
- [ ] No table row, no model label ("Gemini 3.5 Flash …"), and no qwen-code clause altered.
- [ ] Scope respected: only lines 3, 7, 74, 88, 92 of `docs/providers.md`; no other line or file touched.

### Documentation & Deployment

- [ ] The corrected prose IS the user-facing documentation (Mode A target = docs/providers.md).
- [ ] docs/providers.md now agrees with `internal/provider/builtin.go`, `internal/provider/registry.go`, and PRD §12.5.1.1.
- [ ] No new environment variables, config, or build steps introduced.

---

## Anti-Patterns to Avoid

- ❌ Don't edit line 85 (the agy table row) — that is sibling P2.M3.T1.S1 (D1/D2), running in parallel.
  This subtask edits prose/heading lines only. See Gotcha 2.
- ❌ Don't edit `docs/README.md` — its "8 built-in providers" (D8) and "21-field" (D9) are sibling P2.M3.T2.
  See Gotcha 3.
- ❌ Don't alter the qwen-code sentence on line 88 — only the agy #76 date token (± optional reframe) is
  in scope. qwen-code has no date token and did not get re-verified. See Gotcha 1.
- ❌ Don't touch any "Gemini" token (model labels / lineage) — those are legitimate, not drift. See Gotcha 5.
- ❌ Don't normalize casing — match each site's exact spelling (digit vs word; Eight vs eight). See Pattern.
- ❌ Don't do BOTH the minimal date swap AND the reframe on line 88 — pick one (reframe supersedes minimal
  since it includes the corrected date). See Task 5.
- ❌ Don't edit `PRD.md`, `internal/provider/builtin.go`, `internal/provider/registry.go`, `tasks.json`,
  `prd_snapshot.md`, or `.gitignore` — all READ-ONLY / forbidden.
- ❌ Don't rely on CI markdownlint — it is not in `.github/workflows/ci.yml`. The grep gate is the authority.
  See Gotcha 6.
- ❌ Don't miss a count site — the drift is 4 independent places (3, 7, 74, 92). The negative grep catches
  all three spellings (`8 built` / `eight built` / `Eight providers`); require zero matches. See Gotcha 4.

---

## Confidence Score

**One-pass success likelihood: 10/10.** Five in-place prose/heading edits (one optional) whose contract
is fully specified by verbatim current→desired literals, cross-checked against three mutually-consistent
sources of truth (`builtin.go` line-17 comment + 7 `builtin*` funcs, `registry.go` 7-entry preferredBuiltins,
PRD §12.5.1.1 date 2026-07-08). The implementing agent reads the 5 lines, applies the string swaps, and
runs a deterministic grep gate (negative stale-token scan = 0 matches; positive = exactly 5 lines;
table-row-integrity; sibling-file-integrity; docs-only build invariant). The scope boundaries are sharp:
line 85 (S1) and docs/README.md (P2.M3.T2) are explicitly excluded with regression checks. No code, no
schema, no behavior, no external dependency; the only failure modes are missing a count site or straying
into a sibling's line/file — both caught by the verification gate.
