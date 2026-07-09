name: "P2.M3.T2.S1 — Correct provider count and field count in docs/README.md"
description: |
  Mode A documentation fix (a single one-line, two-token edit). Correct the "Provider manifests"
  index row in `docs/README.md` (line 35) so it stops contradicting its own referenced page and the
  source of truth. Audit `docs_drift_audit.md` flagged TWO drifts on this ONE line:
    - **D8** (provider count): `the 8 built-in providers` → `the 7 built-in providers`
    - **D9** (field count, adjacent drift): `21-field manifest schema` → `22-field manifest schema`
  After the edit, line 35 reads:
    `| [Provider manifests](providers.md) | 22-field manifest schema, command rendering, the 7 built-in providers (incl. agy and qwen-code), and adding a new agent. |`

  Out of scope (leave for siblings): `docs/providers.md` (provider-count + agy-row + date drift) =
  sibling P2.M3.T1.S1/S2 (running in parallel — DIFFERENT FILE, zero collision). Top-level
  `README.md` is a different file and is ALREADY CLEAN — do not touch it. No `.go` / `PRD.md` /
  `tasks.json` / `prd_snapshot.md` / `.gitignore`.

---

## Goal

**Feature Goal**: Make the `docs/README.md` "Provider manifests" index row (line 35) factually
correct and internally consistent — provider count `8 → 7` (audit D8) and field count `21 → 22`
(audit D9) — so the index no longer contradicts (a) the page it links to (`docs/providers.md`, which
already says "22-field schema"), (b) the compiled manifest (`internal/provider/manifest.go` =
22 `toml:` tags), and (c) the built-in set (`internal/provider/builtin.go` = 7 providers).

**Deliverable**: One surgical in-place edit to `docs/README.md` line 35 — exactly two token swaps
(`21`→`22`, `8`→`7`); every other character on the line and in the file stays byte-for-byte identical.

**Success Definition**: All of the following hold after the edit:
1. `grep -nE '21-field|the 8 built-in providers' docs/README.md` → **zero matches**.
2. Line 35 contains BOTH `22-field manifest schema` AND `the 7 built-in providers`.
3. `git diff --stat docs/README.md` shows exactly **1 insertion + 1 deletion** (one line changed).
4. Only `docs/README.md` is modified; no other file touched (incl. top-level `README.md`).
5. No `.go` source changed; `go build ./...` and `go test ./...` stay green (docs-only invariant).

## User Persona (if applicable)

**Target User**: A stagecoach user / integrator landing on `docs/README.md` (the docs index) to find
the provider-manifest page and quickly learn how many providers ship and how big the schema is.

**Use Case**: Skimming the "Documentation index" table to decide whether to click into
`[Provider manifests](providers.md)` — trusting the index row's headline numbers (field count +
built-in count) as an accurate teaser of the page's contents.

**User Journey**: Open `docs/README.md` → read the "Documentation index" table → spot the
"Provider manifests" row → click through to `providers.md` → the index row's numbers (22 fields,
7 providers) must MATCH the page's own intro ("22-field schema", "the 7 built-in providers") and the
7-row quick-reference table. Today they DO NOT match (index says 21 / 8; page says 22 / 7).

**Pain Points Addressed**: Today the index row is the lone outlier — it advertises a "21-field
manifest schema" and "8 built-in providers" while the very page it links to says "22-field schema"
and "7 built-in providers", and the compiled binary ships exactly 7 built-ins over a 22-tag schema.
A reader clicking through sees the numbers change and loses trust in the docs. This fix makes the
index consistent with its own linked page and the binary.

## Why

- **Index ↔ page ↔ code parity (the P2 milestone's whole point).** The provider lineup correction
  removed the EOL `gemini` built-in (superseded by `agy`), dropping the count 8 → 7. Separately, the
  compiled `Manifest` struct carries 22 `toml:` tags (the 22nd being `experimental`, which the PRD
  §12.1 schema *example* omits). `docs/providers.md` already reflects both truths ("22-field schema",
  7-row table). `docs/README.md`'s index row was NOT swept — the docs-drift audit flagged it as items
  D8 + D9. This subtask closes that residual gap on the docs landing page.
- **The index is the highest-traffic surface; its accuracy matters most.** `docs/README.md` is the
  directory's entry point. A wrong headline number here is the most visible drift a reader can hit.
- **Lowest-risk change class.** One line, two digit swaps, no code, no schema, no behavior, no table
  structure. Risk surface = "did I edit line 35 (not 34/36, and not the top-level README.md), and did
  I change exactly the two tokens" — fully covered by a deterministic grep + `git diff` gate.

## What

A single in-place edit to `docs/README.md` line 35. The edit swaps exactly two tokens on one line;
no lines are added or removed; no other file is touched.

| # | Line | Current (drifted) token | Corrected token |
|---|------|--------------------------|-----------------|
| **D9** | 35 | `21-field manifest schema` | `22-field manifest schema` |
| **D8** | 35 | `the 8 built-in providers` | `the 7 built-in providers` |

Full line (before → after):

```
BEFORE: | [Provider manifests](providers.md) | 21-field manifest schema, command rendering, the 8 built-in providers (incl. agy and qwen-code), and adding a new agent. |
AFTER : | [Provider manifests](providers.md) | 22-field manifest schema, command rendering, the 7 built-in providers (incl. agy and qwen-code), and adding a new agent. |
```

### Success Criteria

- [ ] Line 35 reads `22-field manifest schema` (audit D9 resolved).
- [ ] Line 35 reads `the 7 built-in providers` (audit D8 resolved).
- [ ] Zero matches for `21-field|the 8 built-in providers` in `docs/README.md`.
- [ ] Exactly one line changed in `docs/README.md` (`git diff --stat` = 1 ins / 1 del); line 35 only.
- [ ] No file other than `docs/README.md` modified (top-level `README.md`, all `.go`, etc. untouched).
- [ ] No `.go` / `PRD.md` / `tasks.json` / `prd_snapshot.md` / `.gitignore` modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** The edit is fully specified by a verbatim before→after line. The "7" count is
pinned by the compiled manifest (`builtin.go`: 7 built-ins, comment "Seven providers") and registry
(`registry.go`: 7-entry preferredBuiltins). The "22" field count is pinned by the compiled struct
(`manifest.go`: exactly 22 `toml:` tags) and the indexed page itself (`providers.md` line 3:
"22-field schema"). No live binary, no Go knowledge, no external docs required — read line 35, swap
the two tokens, run the grep gate.

### Documentation & References

```yaml
# MUST READ — the ONLY file being edited (Mode A target = docs/README.md)
- file: docs/README.md
  why: Contains the single drifted line — line 35, the "Provider manifests" index row — carrying both
       the stale provider count "8" (D8) and the stale field count "21" (D9).
  pattern: Read lines 27–40 (the "Documentation index" table) before editing to see line 35 in context
           and confirm it is a markdown table row (the edit must preserve the table cell structure).
  gotcha: This is docs/README.md — NOT the repo-root README.md (which is a different, already-clean
          file). Confirm you are in docs/README.md before editing.

# Source of truth 1 — the compiled manifest (READ-ONLY; the doc count must mirror this)
- file: internal/provider/builtin.go
  why: Confirms the built-in count is 7. Line 17 comment: "Seven providers: pi, claude, opencode,
       codex, cursor, agy, qwen-code." func BuiltinManifests() returns exactly 7 entries.
  section: "line 17 comment + func BuiltinManifests() (line 18)"

# Source of truth 2 — the compiled manifest struct (READ-ONLY; the field count must mirror this)
- file: internal/provider/manifest.go
  why: Confirms the field count is 22. `grep -c 'toml:' internal/provider/manifest.go` == 22. The 22
       tags are: name, detect, command, list_models_command, subcommand, prompt_delivery, prompt_flag,
       print_flag, model_flag, default_model, system_prompt_flag, provider_flag, session_mode,
       bare_flags, tooled_flags, experimental, output, json_field, strip_code_fence, retry_instruction,
       env, reasoning_levels. The 22nd (`experimental`, manifest.go:81) is the one PRD §12.1's EXAMPLE
       omits — which is why docs/README.md drifted to 21. The compiled struct (22) is the authority.
  section: "Manifest struct toml: tags (line ~70–110)"

# The indexed page (READ-ONLY; the index row must agree with the page it points at)
- file: docs/providers.md
  why: Line 3 already reads "the 22-field schema" — proving docs/README.md ("21-field") is the lone
       outlier. The index row should mirror the page it indexes. (Do NOT edit providers.md — sibling
       P2.M3.T1 owns it.)
  section: "line 3 (intro sentence)"

# The audit that named this drift (READ-ONLY; defines D8/D9 verbatim)
- docfile: plan/013_b8a415cc6e79/architecture/docs_drift_audit.md
  why: §2d (line 64) flags docs/README.md line 35 for BOTH drifts. Line 76 (D9 table row) explains
       "manifest.go has 22 toml: tags; providers.md ... says '22-field schema'; docs/README.md is the
       lone outlier at 21." Lines 105–106 give the exact current→correct text. Line 114 confirms D8+D9
       are intentionally batched into one one-line edit ("D9 ... lives on the same docs/README.md line
       being edited for D8"). This subtask's contract IS D8 + D9.
  section: "§2d (D8 + D9 on docs/README.md line 35)"

# Sibling task context (CONTRACT — P2.M3.T1 owns docs/providers.md, a DIFFERENT file)
- docfile: plan/013_b8a415cc6e79/P2M3T1S2/PRP.md
  why: Defines the PARALLEL edit to docs/providers.md (lines 3/7/74/88/92 + line 85). That sibling
       edits a DIFFERENT FILE, so there is zero collision with this task (docs/README.md). Its PRP
       explicitly asserts "docs/README.md unchanged (D8/D9 = sibling P2.M3.T2)" — confirming disjoint
       ownership in both directions. Do not duplicate or touch its work.

# Research notes for this subtask
- docfile: plan/013_b8a415cc6e79/P2M3T2S1/research/source_of_truth.md
  why: Full cross-check of all three sources of truth + the verbatim before→after line + the
       21-vs-22 explanation (`experimental` is the 22nd tag) + the non-collision proof with the
       parallel sibling + the deterministic verification command set.
```

### Current Codebase tree (relevant slice)

```bash
# Run from repo root: cd /home/dustin/projects/stagecoach
docs/README.md                          # ← the ONLY file edited (line 35 only)
README.md                               # top-level README — DIFFERENT file, ALREADY CLEAN (do NOT touch)
internal/provider/builtin.go            # source of truth (7 built-ins, READ-ONLY)
internal/provider/manifest.go           # source of truth (22 toml: tags, READ-ONLY)
docs/providers.md                       # the indexed page — already says "22-field schema" (sibling-owned, READ-ONLY here)
PRD.md                                  # source of truth (READ-ONLY)
plan/013_b8a415cc6e79/
  architecture/docs_drift_audit.md      # the audit naming D8/D9 (READ-ONLY)
  P2M3T1S2/PRP.md                       # sibling: edits docs/providers.md (CONTRACT — different file)
  P2M3T2S1/
    PRP.md                              # ← THIS file
    research/source_of_truth.md         # research notes
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
# NO new files. The ONLY change is one in-place edit in docs/README.md. After the edit:
docs/README.md   # line 35: "21-field manifest schema" -> "22-field manifest schema"
                #           "the 8 built-in providers" -> "the 7 built-in providers"
                #           (both swaps on the SAME line; nothing else changed anywhere)
# Nothing else is created, deleted, or modified.
```

### Known Gotchas of our codebase & Library Quirks

```text
# GOTCHA 1 — Edit docs/README.md, NOT the repo-root README.md.
#   There are TWO README files: docs/README.md (the docs index, the one WITH the drift) and the
#   top-level README.md (marketing surface, ALREADY CLEAN — audit §3 confirms it says "Seven
#   built-ins"). This task edits ONLY docs/README.md. The verification gate (Level 3 check (e))
#   asserts the top-level README.md is untouched. Confirm your buffer/path is docs/README.md.

# GOTCHA 2 — Both drifts are on the SAME line (35); edit it ONCE, not twice.
#   D8 (provider count) and D9 (field count) live on the same table row. Do them in a single edit
#   (both token swaps), not two separate edits to the "same" line — that risks a stale-match error or
#   a double application. The current line text in Task 2 below is the FULL line with BOTH old tokens.

# GOTCHA 3 — Why the field count is 22 (not 21): the 22nd tag is `experimental`.
#   If you count the fields in the PRD §12.1 schema EXAMPLE you get 21 (it OMITS `experimental`). But
#   the COMPILED Manifest struct in manifest.go has 22 toml: tags — the 22nd is
#   `experimental *bool toml:"experimental"` (manifest.go:81). The contract chooses the COMPILED struct
#   (22) as the source of truth, matching docs/providers.md (the indexed page, which already says
#   "22-field schema"). Do NOT "correct" 22 back to 21 if you miscount the §12.1 example. The verified
#   authority is: grep -c 'toml:' internal/provider/manifest.go == 22.

# GOTCHA 4 — Preserve the markdown table structure; only the cell TEXT changes.
#   Line 35 is a row in the "Documentation index" table (| Page | Description |). The edit swaps two
#   substrings INSIDE the Description cell. Keep the leading `| [Provider manifests](providers.md) |`,
#   the trailing ` |`, and all the commas/parentheses exactly as-is. Do not add/remove pipes or the
#   `(incl. agy and qwen-code)` parenthetical.

# GOTCHA 5 — docs/providers.md is OUT OF SCOPE (sibling P2.M3.T1, running in parallel).
#   The page this row indexes (docs/providers.md) has its OWN residual drift (provider count 8→7 at
#   lines 3/7/74/92, date 2026-07-03→07-08 at line 88, agy table row at line 85) being fixed by
#   sibling P2.M3.T1.S1/S2. It is a DIFFERENT FILE, so no collision — but do NOT fix it here. This
#   task's gate asserts only docs/README.md is modified.

# GOTCHA 6 — markdownlint is configured but NOT enforced in CI.
#   .markdownlint.json sets default:true (MD013/MD033/MD060 off). .github/workflows/ci.yml runs only
#   `go test` + golangci-lint (no markdownlint step). You are editing inline table-cell text (no pipe
#   count change), so MD056/MD058 cannot trip anyway. The authoritative gate is the grep + git diff check.

# GOTCHA 7 — "Gemini" model labels are NOT drift; none appear on line 35 anyway.
#   Line 35's `(incl. agy and qwen-code)` parenthetical is CORRECT (both are current built-ins). Leave
#   it. There is no "gemini" token on this line to worry about.
```

## Implementation Blueprint

### Data models and structure

None. This is a one-line documentation edit — no data models, schemas, code, config, or table
structure are touched. The edit swaps two substrings inside an existing markdown table cell.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: READ the target line in context to confirm exact current text and that it is the table row
  - RUN: sed -n '27,40p' docs/README.md
  - CONFIRM: line 35 is the "Provider manifests" table row and reads (verbatim):
             | [Provider manifests](providers.md) | 21-field manifest schema, command rendering, the 8 built-in providers (incl. agy and qwen-code), and adding a new agent. |
  - NOTE: this read-only step prevents editing the wrong line/file and confirms the table structure
          (you are swapping text inside one Description cell, not touching the | pipes).

Task 2: EDIT line 35 (audits D8 + D9 together — both tokens on the same line, one edit)
  - OLD (FULL line, copy verbatim):
      | [Provider manifests](providers.md) | 21-field manifest schema, command rendering, the 8 built-in providers (incl. agy and qwen-code), and adding a new agent. |
  - NEW (FULL line, both swaps applied):
      | [Provider manifests](providers.md) | 22-field manifest schema, command rendering, the 7 built-in providers (incl. agy and qwen-code), and adding a new agent. |
  - SCOPE: swap exactly two substrings — "21-field" -> "22-field" and "the 8 built-in providers"
           -> "the 7 built-in providers" — on line 35 only. Everything else (the link text, the
           pipe structure, "command rendering", "(incl. agy and qwen-code)", "and adding a new agent.")
           stays byte-for-byte identical.

Task 3: VERIFY — run the deterministic grep + git diff gate (Validation Loop Levels 2–3)
  - RUN: the negative-grep + positive-pinpoint + diff-scope + sibling-file + forbidden-file commands.
  - EXPECT: zero stale-token matches; line 35 carries both corrected literals; exactly one line
            changed in docs/README.md; no other file touched; no .go/PRD/tasks changed; build green.
  - GATE: every check must pass before declaring the subtask done.
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: this is a two-token, one-line edit. Prefer a SINGLE edit operation with the FULL current
     line as oldText and the FULL corrected line as newText (see Task 2). That guarantees both tokens
     are swapped atomically and avoids "oldText not unique" or partial-application pitfalls. -->
<!-- CRITICAL: oldText MUST be the exact current line (with "21-field" and "8 built-in"). If the editor
     can't match it, re-run `sed -n '35p' docs/README.md` and reconcile whitespace/punctuation before
     retrying — do not guess. -->
<!-- CRITICAL: edit docs/README.md ONLY. The top-level README.md is a different file and is clean. -->
```

### Integration Points

```yaml
DATABASE: none   # docs-only edit
CONFIG:   none   # no config file changed
ROUTES:   none
# The only "integration" is docs-parity: after the edit, docs/README.md line 35 agrees with
# (a) docs/providers.md line 3 ("22-field schema"), (b) internal/provider/manifest.go (22 toml: tags),
# (c) internal/provider/builtin.go (7 built-ins). CI does not lint markdown; the grep + git diff gate
# is the authority.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach
# You edited inline table-cell text only (no pipe added/removed), so MD056 (table-column-count) and
# MD058 (blanks around tables) cannot trip. Optional markdownlint sanity (NOT required; not in CI):
npx --yes markdownlint-cli2 'docs/README.md' 2>/dev/null | grep -i 'MD0[0-9]' || echo "ok: no blocking MD issues (or tool unavailable)"
# Expected: "ok: ..." (or tool unavailable). A one-token text swap inside a cell cannot break table lint.
```

### Level 2: Pinpoint Verification (the core gate — run after the edit)

```bash
cd /home/dustin/projects/stagecoach
# (a) NEGATIVE — both stale tokens GONE from docs/README.md (zero matches required):
grep -nE '21-field|the 8 built-in providers' docs/README.md && echo "FAIL: stale tokens remain" || echo "ok: no stale 21-field / 8 built-in tokens"

# (b) POSITIVE — line 35 carries BOTH corrected literals:
sed -n '35p' docs/README.md | grep -F -- '22-field manifest schema'      >/dev/null && echo "ok: field=22 (D9)" || echo "FAIL: D9 field count"
sed -n '35p' docs/README.md | grep -F -- 'the 7 built-in providers'      >/dev/null && echo "ok: count=7 (D8)" || echo "FAIL: D8 provider count"

# (c) The rest of line 35 is preserved (the link, the parenthetical, the tail) — sanity:
sed -n '35p' docs/README.md | grep -F -- '(incl. agy and qwen-code), and adding a new agent.' >/dev/null && echo "ok: line tail intact" || echo "FAIL: line tail changed"

# Expected: (a) "ok: no stale ... tokens"; (b) both print "ok: ..."; (c) "ok: line tail intact".
```

### Level 3: Diff-Scope & Regression (System Validation)

```bash
cd /home/dustin/projects/stagecoach
# (d) EXACTLY ONE line changed in docs/README.md (1 insertion + 1 deletion), and it is line 35:
git diff --numstat docs/README.md                       # EXPECT: "1 1 docs/README.md" (1 add, 1 del)
git diff docs/README.md | grep -E '^[+-]\|' | wc -l     # EXPECT: 2 (one '-' old line + one '+' new line)
git diff docs/README.md | grep -E '^\+' | grep -F '22-field manifest schema' >/dev/null && echo "ok: added line has 22-field" || echo "FAIL"

# (e) NO other file touched — only docs/README.md (top-level README.md must be untouched):
git diff --name-only | grep -vE '^docs/README.md$' | grep . && echo "FAIL: other files touched" || echo "ok: only docs/README.md modified"
git diff --name-only | grep -E '^README.md$' && echo "FAIL: top-level README.md touched" || echo "ok: top-level README.md untouched"

# (f) Docs-only invariant — NO Go source changed; build + tests stay green:
git diff --name-only | grep -E '\.go$' && echo "FAIL: Go file touched" || echo "ok: no .go changed"
go build ./... 2>&1 | tail -3          # EXPECT: clean (no errors)
go test ./...   2>&1 | tail -5         # EXPECT: all PASS (a doc text swap cannot break Go tests)

# (g) No forbidden files modified:
git diff --name-only | grep -E 'PRD\.md|tasks\.json|prd_snapshot\.md|\.gitignore' && echo "FAIL: forbidden file touched" || echo "ok: no forbidden file touched"

# Expected: (d) 1/1 numstat + 2 diff lines + added line has "22-field"; (e) ok; (f) no .go + clean build + tests pass; (g) ok.
```

### Level 4: Parity Proof (Domain-Specific Validation)

```bash
cd /home/dustin/projects/stagecoach
# Prove the corrected index row now agrees with the THREE authorities it should mirror.
echo "--- (1) indexed page: docs/providers.md says '22-field schema' (the page the row links to) ---"
grep -nE '22-field schema' docs/providers.md | head -1                   # EXPECT: line 3 match
echo "--- (2) compiled struct: manifest.go toml: tag count (the schema authority) ---"
grep -c 'toml:' internal/provider/manifest.go                            # EXPECT: 22
echo "--- (3) compiled built-ins: builtin.go says 'Seven providers' (the count authority) ---"
grep -nE 'Seven providers' internal/provider/builtin.go | head -1        # EXPECT: line 17 comment
echo "--- rendered row (the deliverable) ---"
sed -n '35p' docs/README.md
# Expected: providers.md has "22-field schema"; manifest.go == 22 tags; builtin.go has "Seven providers";
# and the rendered line 35 now reads "22-field manifest schema ... the 7 built-in providers".
# This is the audit's core claim — the docs index now agrees with the page it indexes and the binary.
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 2 (negative): `grep -nE '21-field|the 8 built-in providers' docs/README.md` → 0 matches.
- [ ] Level 2 (positive): line 35 contains both `22-field manifest schema` and `the 7 built-in providers`.
- [ ] Level 2 (tail): line 35's `(incl. agy and qwen-code), and adding a new agent.` tail is intact.
- [ ] Level 3 (scope): `git diff --numstat docs/README.md` shows `1 1` (exactly one line changed).
- [ ] Level 3 (other files): no file other than `docs/README.md` modified.
- [ ] Level 3 (top-level README): `README.md` (repo root) untouched.
- [ ] Level 3 (invariant): no `.go` file changed; `go build ./...` and `go test ./...` green.
- [ ] Level 3 (forbidden): no `PRD.md` / `tasks.json` / `prd_snapshot.md` / `.gitignore` modified.

### Feature Validation

- [ ] The "Provider manifests" index row no longer advertises a "21-field" schema (now 22).
- [ ] The row no longer advertises "8 built-in providers" (now 7).
- [ ] The row's numbers now MATCH the page it links to (docs/providers.md "22-field schema", 7-row table).
- [ ] Manual read confirms the docs index is internally consistent with the binary's actual lineup/schema.

### Code Quality Validation

- [ ] Exactly two token swaps (`21`→`22`, `8`→`7`) on line 35; nothing else on the line changed.
- [ ] Markdown table structure preserved (no pipes added/removed; link text + cell structure intact).
- [ ] Scope respected: only line 35 of `docs/README.md`; no other line or file touched.

### Documentation & Deployment

- [ ] The corrected index row IS the user-facing documentation (Mode A target = docs/README.md).
- [ ] docs/README.md line 35 now agrees with `internal/provider/manifest.go` (22 tags),
      `internal/provider/builtin.go` (7 built-ins), and `docs/providers.md` ("22-field schema").
- [ ] No new environment variables, config, or build steps introduced.

---

## Anti-Patterns to Avoid

- ❌ Don't edit the top-level `README.md` — it is a DIFFERENT file and is ALREADY CLEAN ("Seven
  built-ins"). This task edits ONLY `docs/README.md` (line 35). See Gotcha 1.
- ❌ Don't edit `docs/providers.md` — its residual drift (provider count, agy row, date) is owned by
  sibling P2.M3.T1.S1/S2, running in parallel on a DIFFERENT file. See Gotcha 5.
- ❌ Don't make two separate edits to "line 35" — do D8 + D9 in ONE edit (both tokens on the same line)
  using the full current→corrected line from Task 2. See Gotcha 2.
- ❌ Don't "correct" the field count back to 21 after counting the PRD §12.1 schema EXAMPLE — that
  example OMITS `experimental`. The compiled struct (`manifest.go` = 22 toml: tags) is the authority,
  and it matches `docs/providers.md` ("22-field schema"). See Gotcha 3.
- ❌ Don't alter the markdown table structure — only swap the two substrings inside the Description cell;
  keep the `| [Provider manifests](providers.md) | … |` skeleton and the `(incl. agy and qwen-code)`
  parenthetical. See Gotcha 4.
- ❌ Don't edit `PRD.md`, `internal/provider/manifest.go`, `internal/provider/builtin.go`, `tasks.json`,
  `prd_snapshot.md`, or `.gitignore` — all READ-ONLY / forbidden.
- ❌ Don't rely on CI markdownlint — it is not in `.github/workflows/ci.yml`. The grep + `git diff` gate
  is the authority. See Gotcha 6.

---

## Confidence Score

**One-pass success likelihood: 10/10.** A single one-line edit with two digit swaps, whose contract
is fully specified by a verbatim before→after line, cross-checked against three mutually-consistent
sources of truth (`manifest.go` = 22 toml: tags; `builtin.go` = 7 built-ins; `providers.md` =
"22-field schema"). The implementing agent reads line 35, swaps `21`→`22` and `8`→`7`, and runs a
deterministic gate (negative stale-token scan = 0 matches; positive pinpoint on line 35; `git diff
--numstat` = 1/1; no other file / top-level README / `.go` / forbidden file touched; build green).
The scope boundaries are sharp: top-level `README.md` (clean, different file) and `docs/providers.md`
(sibling-owned, different file) are explicitly excluded with regression checks. No code, no schema,
no behavior, no external dependency; the only failure modes are editing the wrong file (README.md vs
docs/README.md) or straying beyond line 35 / beyond the two tokens — all caught by the verification gate.
