name: "P2.M3.T1.S1 — Correct the agy quick-reference table row (print flag + tool-disable cells)"
description: |
  Mode A documentation fix (single-line edit). Correct exactly TWO cells of the `agy` row in the
  providers quick-reference table at `docs/providers.md` line 85 so the table matches the compiled
  agy manifest (`internal/provider/builtin.go builtinAgy()` / `providers/agy.toml`) and PRD §12.5.1,
  which were re-verified against agy v1.1.0 on 2026-07-08.
    - Audit item **D1** — Print flag cell: `` `-p` `` → `(none)`  (agy v1.1.0's `-p` is value-taking;
      a bare `-p` fails with "flag needs an argument: -p"; `PrintFlag=""` → agy reads stdin, no flag).
    - Audit item **D2** — Tool-disable cell: `Read-only constraint (`--approval-mode default`)` →
      `Read-only constraint (`--mode plan`)`  (`--approval-mode` was removed in v1.1.0;
      `BareFlags=["--mode","plan"]` is the read-only equivalent).
  The six other cells on the agy row (delivery `stdin`, model flag `--model`, default model
  `Gemini 3.5 Flash (Low)`, system prompt `(prepended)`, stager `— no`, name `` `agy` ``) are
  ALREADY CORRECT and must NOT be touched. No other line, file, or source is modified. This is
  subtask 1 of the Mode B residual-docs-drift milestone P2.M3 (siblings S2 fixes the #76 date +
  provider count; P2.M3.T2 fixes docs/README.md — both out of scope here).

---

## Goal

**Feature Goal**: Make the `agy` row of the providers quick-reference table in `docs/providers.md`
faithfully mirror the corrected, re-verified (2026-07-08, agy v1.1.0) agy manifest by fixing its two
drifted cells — the Print flag cell and the Tool-disable cell — so the documented invocation matches
the invocation the binary actually emits (and that PRD §12.5.1 specifies).

**Deliverable**: A one-line edit to `docs/providers.md:85` that changes exactly two cells and leaves
the other six (plus all other table rows and all other lines) byte-for-byte unchanged. The resulting
line 85 must read, verbatim:
```
| `agy` | stdin | (none) | `--model` | `Gemini 3.5 Flash (Low)` | (prepended) | Read-only constraint (`--mode plan`) | — no |
```

**Success Definition**: All of the following hold after the edit:
1. `sed -n '85p' docs/providers.md` equals the verbatim desired line above.
2. The OLD drift tokens (`` `-p` `` and `--approval-mode`) no longer appear on line 85.
3. The NEW tokens (`(none)` and `--mode plan`) appear on line 85.
4. The six untouched agy cells are unchanged (delivery/model/default/sys/stager/name intact).
5. The agy row column count equals the header row column count (MD056 — table integrity preserved).
6. No other table row regressed (qwen-code keeps `-p`+`--approval-mode default`; opencode/codex keep `(none)`).
7. No `.go` source file changed; `go build ./...` and `go test ./...` stay green (docs-only invariant).

## User Persona (if applicable)

**Target User**: A stagecoach user / integrator reading `docs/providers.md` to learn how each
built-in provider is invoked (which delivery mode, which print/model flags, how tools are disabled,
whether it can stage).

**Use Case**: Skimming the quick-reference table to copy the correct agy invocation (e.g. for an
override config, a manual run, or to understand why a bare `-p` fails).

**User Journey**: Open `docs/providers.md` → scan the "built-in providers" table → read the `agy` row
→ trust that the Print flag `(none)` and Tool-disable `--mode plan` cells match the real agy v1.1.0
binary (which they now will, after this fix).

**Pain Points Addressed**: The current agy row documents an invocation (`-p` + `--approval-mode
default`) that **does not work on agy v1.1.0** — a bare `-p` fails ("flag needs an argument"), and
`--approval-mode` was removed. A reader following the table would get a broken command. This fix
aligns the doc with the verified binary and PRD.

## Why

- **Docs ↔ code ↔ PRD parity.** The provider lineup correction (milestone P2) re-verified agy against
  v1.1.0 on 2026-07-08 and corrected the compiled manifest (`builtinAgy()`) and PRD §12.5.1 (both now
  `PrintFlag=""`, `BareFlags=["--mode","plan"]`). The `docs/providers.md` table was NOT updated in that
  change, so it still shows the old `-p` / `--approval-mode default` surface — the highest-signal
  residual drift flagged by the docs-drift audit (items D1, D2). This subtask closes that gap.
- **User-facing accuracy is the whole point of the table.** The quick-reference table is the
  single most-read surface for "how do I invoke agy?" Documenting an invocation that errors out
  undermines trust in the rest of the doc. Fixing two cells restores correctness with zero risk.
- **Lowest-risk change in the milestone.** It touches one markdown line, two cells, no code, no
  schema, no behavior. The risk surface is "did I edit the right cells and not break the table" —
  fully covered by deterministic grep + column-count checks.

## What

A surgical, single-line edit to `docs/providers.md` line 85 (the `agy` row of the "built-in providers"
quick-reference table). Replace the two drifted cells only:

| Cell (column) | Current (drifted) | Corrected |
|---------------|-------------------|-----------|
| **Print flag** (col 3) | `` `-p` `` | `(none)` |
| **Tool-disable approach** (col 7) | `Read-only constraint (`--approval-mode default`)` | `Read-only constraint (`--mode plan`)` |

The remaining six cells of the agy row are correct and are NOT edited:
- Provider: `` `agy` `` — Delivery: `stdin` — Model flag: `` `--model` `` — Default model:
  `Gemini 3.5 Flash (Low)` — System prompt flag: `(prepended)` — Stager?: `— no`.

The corrected line 85 must read exactly:
```
| `agy` | stdin | (none) | `--model` | `Gemini 3.5 Flash (Low)` | (prepended) | Read-only constraint (`--mode plan`) | — no |
```

### Success Criteria

- [ ] `docs/providers.md:85` equals the verbatim corrected line above.
- [ ] OLD tokens (`` `-p` ``, `--approval-mode`) absent from line 85.
- [ ] NEW tokens (`(none)`, `--mode plan`) present on line 85.
- [ ] The six untouched agy cells unchanged (delivery/model/default/sys/stager/name intact).
- [ ] agy row pipe/column count == header row (MD056 table integrity preserved).
- [ ] No sibling row regressed (qwen-code row line 86 unchanged; opencode/codex `(none)` intact).
- [ ] No `.go` / `PRD.md` / `tasks.json` / `prd_snapshot.md` / `.gitignore` modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** The edit is fully specified by the verbatim current→desired line 85 literal,
cross-checked against four mutually-consistent sources of truth (compiled manifest, reference toml,
PRD §12.5.1, the drift audit). No inference is required: the implementing agent reads `docs/providers.md`
line 85, applies the exact two-cell replacement, and runs the deterministic grep + column-count
verification gate. No live `agy` binary, no Go knowledge, and no external docs are needed.

### Documentation & References

```yaml
# MUST READ — the single file being edited (Mode A target = docs/providers.md)
- file: docs/providers.md
  why: Contains the providers quick-reference table. The `agy` row at line 85 has TWO drifted cells
       (print flag, tool-disable) that this subtask corrects. The 8-column header is at line 79
       (| Provider | Delivery | Print flag | Model flag | Default model | System prompt flag |
        Tool-disable approach | Stager? |); the separator row is line 80.
  pattern: Read the WHOLE table (lines 79–86) before editing so the column alignment is unambiguous.
           Sibling rows show the conventions: opencode/codex use `(none)` for the print flag;
           codex/cursor/qwen-code use the `Read-only constraint (`<flags>`)` form for tool-disable.
  gotcha: Line 85 is the AGY row; line 86 is the QWEN-CODE row. Both currently end with `--approval-mode
          default` and `-p`, but ONLY the agy row is in scope — qwen-code did NOT diverge (it is a
          Gemini-CLI fork) and MUST stay at `-p` / `--approval-mode default`. Do not edit line 86.

# Source of truth 1 — the compiled manifest (READ-ONLY; the doc must mirror this)
- file: internal/provider/builtin.go
  why: builtinAgy() (~lines 199–217) is what the binary actually emits. Confirms PrintFlag=strPtr("")
       and BareFlags=["--mode","plan"] → table cells should be (none) and --mode plan.
  section: "func builtinAgy() Manifest (search for `func builtinAgy`)"
  pattern: PrintFlag: strPtr("") // NON-NIL empty; BareFlags: []string{"--mode", "plan"} // NO --approval-mode.
  gotcha: strPtr("") is an EXPLICIT non-nil empty, not a missing field — it means "emit NO print flag",
          which the table renders as (none).

# Source of truth 2 — the reference manifest (READ-ONLY; human-readable mirror of the compiled manifest)
- file: providers/agy.toml
  why: Byte-for-byte mirror of builtinAgy() with explanatory comments. Confirms print_flag = "" and
       bare_flags = ["--mode", "plan"], plus the rationale (agy v1.1.0 -p is value-taking;
       --approval-mode removed).

# Source of truth 3 — the spec (READ-ONLY; human-owned; the doc must mirror this)
- file: PRD.md
  why: §12.5.1 (h3.58, ~lines 947–978) is the corrected agy manifest TOML re-verified 2026-07-08.
       print_flag = "" and bare_flags = ["--mode", "plan"]. This is the spec the code mirrors and the
       doc must mirror.
  section: "§12.5.1 (heading `### 12.5.1 Built-in provider: Antigravity CLI (`agy`)`)"
  gotcha: PRD §12.1 has a GENERIC schema-EXAMPLE toml block with placeholder values (print_flag = "-p")
          — that is the schema illustration, NOT the agy manifest. Anchor agy checks to §12.5.1.

# The audit that named these two drifts (READ-ONLY; defines D1/D2 verbatim)
- docfile: plan/013_b8a415cc6e79/architecture/docs_drift_audit.md
  why: §1a names audit items D1 (print flag -p→(none)) and D2 (tool-disable --approval-mode default→
       --mode plan) and quotes the exact current and proposed line 85 literal. This subtask's contract
       IS D1+D2.
  section: "§1a Quick-reference table — `agy` row (line 85) — TWO drifted cells"

# Sibling task context (CONTRACT — the previous milestone produces the verified manifest this doc mirrors)
- docfile: plan/013_b8a415cc6e79/P2M2T1S2/PRP.md
  why: Confirmed PRD §12.5.1/§12.5.1.1/§22.1 carry the agy re-verification (print_flag="", bare_flags
       =["--mode","plan"]). The doc fix here mirrors exactly what that verification locked into the spec
       and code. Do not duplicate its PRD work; this subtask only touches docs/providers.md.

# Research notes for this subtask
- docfile: plan/013_b8a415cc6e79/P2M3T1S1/research/source_of_truth.md
  why: Full cross-check of all four sources of truth + the exact current/desired line 85 literal +
       the deterministic verification command set. The implementing agent should consult it for the
       raw evidence behind every claim in this PRP.
```

### Current Codebase tree (relevant slice)

```bash
# Run from repo root: cd /home/dustin/projects/stagecoach
docs/providers.md                       # ← the ONLY file edited (line 85, two cells)
internal/provider/builtin.go            # source of truth (builtinAgy, READ-ONLY)
providers/agy.toml                      # source of truth (reference manifest, READ-ONLY)
PRD.md                                  # source of truth (§12.5.1, READ-ONLY)
.github/workflows/ci.yml                # CI (no markdownlint in CI — gate is grep-based)
.markdownlint.json                      # markdownlint config (MD013/MD033/MD060 off; MD056 on)
plan/013_b8a415cc6e79/
  architecture/docs_drift_audit.md      # the audit naming D1/D2 (READ-ONLY)
  P2M2T1S2/PRP.md                       # sibling: PRD re-verification (CONTRACT)
  P2M3T1S1/
    PRP.md                              # ← THIS file
    research/source_of_truth.md         # research notes
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
# NO new files. The ONLY change is two cells on docs/providers.md:85. After the edit:
docs/providers.md   # line 85: agy row corrected (print flag -> (none); tool-disable -> --mode plan)
# Nothing else is created, deleted, or modified.
```

### Known Gotchas of our codebase & Library Quirks

```text
# GOTCHA 1 — TWO table rows share the OLD tokens; only the AGY row is in scope.
#   docs/providers.md line 85 (agy) AND line 86 (qwen-code) BOTH currently show `-p` (print flag) and
#   `--approval-mode default` (tool-disable). agy DIVERGED from the gemini-cli lineage in v1.1.0
#   (print_flag now "", bare_flags now ["--mode","plan"]); qwen-code did NOT (it is a Gemini-CLI fork
#   and keeps -p / --approval-mode default). Edit ONLY line 85. Line 86 MUST stay as-is. The
#   verification gate explicitly checks qwen-code is untouched.

# GOTCHA 2 — `(none)` is the established table convention for "no print flag", not a new invention.
#   The opencode row (line 82) and codex row (line 83) already use `(none)` in the Print flag column
#   because they too deliver via stdin/positional with no print flag. agy joining them is consistent.
#   Do not invent a new token (e.g. "", "—", "n/a") — use exactly `(none)`.

# GOTCHA 3 — Preserve the 8-column structure / pipe count (MD056).
#   The header (line 79) and every row have 8 columns / 9 pipes. The edit swaps cell CONTENTS, not
#   the number of cells. Do not add or remove any `|`. The verification gate counts pipes on line 85
#   vs line 79 and requires equality.

# GOTCHA 4 — The six untouched agy cells are correct and delicate.
#   Delivery=stdin, Model flag=`--model`, Default model=`Gemini 3.5 Flash (Low)`, System prompt
#   =(prepended), Stager?=— no, and the name cell `agy` are all already correct. A careless find/replace
#   could clobber them. Use a TARGETED edit on exactly the two cells, not a whole-row regex that might
#   reflow. The verification gate asserts all six are still present on line 85.

# GOTCHA 5 — `Gemini 3.5 Flash (Low)` is a MODEL display label, not a provider name.
#   The word "Gemini" in the agy default-model cell is the `agy models` display label verbatim (the
#   model family agy runs), NOT the removed `gemini` provider. It is CORRECT and must remain. The
#   gemini→agy provider succession removed the gemini BUILT-IN, not the Gemini model family label.

# GOTCHA 6 — markdownlint is configured but NOT enforced in CI.
#   .markdownlint.json sets default:true with MD013/MD033/MD060 off; MD056 (table-column-count) is ON.
#   .github/workflows/ci.yml runs only `go test` + golangci-lint (no markdownlint step). So the
#   authoritative validation gate for this edit is the grep + pipe-count checks below; an optional
#   `npx markdownlint-cli2 docs/providers.md` MD056 check is a bonus, not a blocker.
```

## Implementation Blueprint

### Data models and structure

None. This is a documentation edit — no data models, schemas, code, or config are touched. The only
"structure" is the 8-column markdown table row, whose column count must be preserved (MD056).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: READ the full table to confirm column layout and locate the exact agy row
  - RUN: sed -n '79,86p' docs/providers.md
  - CONFIRM: 8-column header at line 79 (Provider|Delivery|Print flag|Model flag|Default model|
             System prompt flag|Tool-disable approach|Stager?); separator at line 80; agy row at 85;
             qwen-code row at 86 (OUT OF SCOPE — must stay `-p` / `--approval-mode default`).
  - NOTE: this read-only step prevents editing the wrong row or the wrong cell.

Task 2: EDIT docs/providers.md line 85 — change exactly the Print flag cell (col 3) and the
        Tool-disable cell (col 7), leaving the other six cells and all other lines untouched
  - OLD (exact current line 85):
      | `agy` | stdin | `-p` | `--model` | `Gemini 3.5 Flash (Low)` | (prepended) | Read-only constraint (`--approval-mode default`) | — no |
  - NEW (exact target line 85):
      | `agy` | stdin | (none) | `--model` | `Gemini 3.5 Flash (Low)` | (prepended) | Read-only constraint (`--mode plan`) | — no |
  - TWO surgical sub-replacements (either replace the whole line, or do two cell-scoped edits):
      (D1)  | stdin | `-p` | `--model` |   →   | stdin | (none) | `--model` |     (the Print flag cell only)
      (D2)  Read-only constraint (`--approval-mode default`)   →   Read-only constraint (`--mode plan`)
  - PRESERVE: the name cell `| `agy` |`, Delivery `stdin`, Model flag `` `--model` ``,
              Default model `Gemini 3.5 Flash (Low)`, System prompt `(prepended)`, Stager `— no`,
              and the leading/trailing `|` and the 9-pipe count (8 columns).
  - DO NOT EDIT: line 86 (qwen-code), lines 82–83 (opencode/codex), line 88 (#76 date — sibling S2),
                 lines 3/7/74/92 (provider count — sibling S2), any docs/README.md line (sibling P2.M3.T2).
  - NAMING/PLACEMENT: edit happens IN PLACE on line 85; no line is added or removed.

Task 3: VERIFY — run the deterministic gate (Validation Loop Level 2) and confirm all checks pass
  - RUN: the pinpoint + negative grep + column-count + sibling-integrity commands (see Validation Loop).
  - EXPECT: line 85 == verbatim target; no stale -p/--approval-mode on 85; (none)+--mode plan present on 85;
            6 untouched cells intact; pipe count 85==79; qwen-code (86) and opencode/codex (82/83)
            unchanged; no .go modified; go build ./... clean.
  - GATE: every check must pass before declaring the subtask done.
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: a correct "read-only constraint" cell in this table wraps the bare flags in
     backticks inside the cell, prefixed by "Read-only constraint (". The codex and cursor rows
     set the precedent (verbatim from docs/providers.md:83,84):
     | `codex`  | stdin      | (none) | `-m` | (user must set) | (prepended) | Read-only constraint (`--sandbox read-only --ephemeral`) | — no |
     | `cursor` | positional | `-p`   | `--model` | (user must set) | (prepended) | Read-only constraint (`--mode ask --trust`) | — no |
     The corrected agy cell follows the SAME form: `Read-only constraint (`--mode plan`)`.
-->
<!-- CRITICAL: the corrected agy row mirrors the compiled manifest EXACTLY. Cross-check after edit:
     PrintFlag=strPtr("")   →  (none)
     BareFlags=["--mode","plan"]  →  --mode plan
     (internal/provider/builtin.go builtinAgy(); providers/agy.toml; PRD §12.5.1) -->
```

### Integration Points

```yaml
DATABASE: none   # docs-only edit
CONFIG:   none   # no config file changed; providers/agy.toml is READ-ONLY reference doc, not edited
ROUTES:   none
# The only "integration" is docs-parity: the corrected table cell is the user-facing documentation
# of how agy is invoked, and it must mirror internal/provider/builtin.go builtinAgy(), providers/agy.toml,
# and PRD §12.5.1. CI does not lint markdown; the grep gate is the authority.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach
# markdown table integrity — agy row must still have the same pipe/column count as the header (MD056):
hdr=$(sed -n '79p' docs/providers.md | tr -cd '|' | wc -c); row=$(sed -n '85p' docs/providers.md | tr -cd '|' | wc -c)
[ "$hdr" = "$row" ] && echo "ok: MD056 cols match ($row pipes)" || echo "FAIL: MD056 mismatch hdr=$hdr row=$row"
# OPTIONAL — full markdownlint pass if the tool is available locally (NOT required; not in CI):
npx --yes markdownlint-cli2 'docs/providers.md' 2>/dev/null | grep -i MD056 && echo "note: MD056 issue" || echo "ok: no MD056 issues (or tool unavailable)"

# Expected: "ok: MD056 cols match (9 pipes)". If MD056 fails, you added/removed a pipe — restore 9.
```

### Level 2: Pinpoint Verification (the core gate — run after the edit)

```bash
cd /home/dustin/projects/stagecoach
# (a) Pinpoint the corrected line 85 — must equal the target EXACTLY:
sed -n '85p' docs/providers.md
# EXPECT (verbatim):
# | `agy` | stdin | (none) | `--model` | `Gemini 3.5 Flash (Low)` | (prepended) | Read-only constraint (`--mode plan`) | — no |

# (b) Negative — OLD drift tokens GONE from line 85:
sed -n '85p' docs/providers.md | grep -F -- '`-p`'             && echo "FAIL: stale -p"          || echo "ok: no -p on 85"
sed -n '85p' docs/providers.md | grep -F -- '--approval-mode'  && echo "FAIL: stale approval-mode" || echo "ok: no approval-mode on 85"

# (c) NEW tokens PRESENT on line 85:
sed -n '85p' docs/providers.md | grep -F -- '(none)'           && echo "ok: (none) present"     || echo "FAIL: missing (none)"
sed -n '85p' docs/providers.md | grep -F -- '--mode plan'      && echo "ok: --mode plan present" || echo "FAIL: missing --mode plan"

# (d) The SIX untouched cells are intact (no collateral change):
sed -n '85p' docs/providers.md \
  | grep -F -- '| stdin |' | grep -F -- '`--model`' | grep -F -- 'Gemini 3.5 Flash (Low)' \
  | grep -F -- '(prepended)' | grep -F -- '— no' >/dev/null && echo "ok: 6 cells intact" || echo "FAIL: collateral change"

# Expected: line 85 matches target verbatim; (b) both report "ok: no ..."; (c) both report "ok: present";
# (d) "ok: 6 cells intact". Any FAIL → fix before proceeding.
```

### Level 3: Cross-Reference & Regression (System Validation)

```bash
cd /home/dustin/projects/stagecoach
# (e) Sibling qwen-code row (line 86) is UNTOUCHED — still -p + --approval-mode default (it did NOT diverge):
sed -n '86p' docs/providers.md | grep -F -- '`-p`' >/dev/null \
  && grep -F -- '--approval-mode default' <(sed -n '86p' docs/providers.md) >/dev/null \
  && echo "ok: qwen-code untouched" || echo "FAIL: qwen-code regressed"
# (f) opencode (82) + codex (83) still show (none) in print flag (convention intact):
sed -n '82p;83p' docs/providers.md | grep -c '(none)'   # EXPECT: 2

# (g) Docs-only invariant — NO Go source changed; build + tests stay green (unchanged by a docs edit):
git diff --name-only | grep -E '\.go$' && echo "FAIL: Go file touched" || echo "ok: no .go changed"
go build ./... 2>&1 | tail -3          # EXPECT: clean (no errors)
go test ./...   2>&1 | tail -5         # EXPECT: all PASS (docs-only change cannot break Go tests)

# (h) No forbidden files modified:
git diff --name-only | grep -E 'PRD\.md|tasks\.json|prd_snapshot\.md|\.gitignore' \
  && echo "FAIL: forbidden file touched" || echo "ok: no forbidden file touched"

# Expected: (e) ok; (f) 2; (g) no .go changed + clean build + tests pass; (h) ok.
```

### Level 4: Creative & Domain-Specific Validation

```bash
cd /home/dustin/projects/stagecoach
# Parity proof: the corrected doc cell mirrors the COMPILED manifest and PRD. Cross-reference values:
#   doc print flag cell  ↔ builtinAgy().PrintFlag = strPtr("")  ↔ providers/agy.toml print_flag = ""
#   doc tool-disable cell↔ builtinAgy().BareFlags = ["--mode","plan"] ↔ agy.toml bare_flags = [...]
sed -n '958p;967p' PRD.md                                   # PRD §12.5.1: print_flag = "" ; bare_flags = ["--mode", "plan"]
grep -nE 'PrintFlag:.*strPtr\(""\)|BareFlags' internal/provider/builtin.go | sed -n '1,4p'  # compiled manifest
# EXPECT: PRD print_flag="" / bare_flags=["--mode","plan"] agree with the doc cells (none) / --mode plan.
# Rendered-cmd cross-check (PRD §12.5.1 example line ~:978 should show no -p and --mode plan):
grep -nE 'agy --model.*--mode plan' PRD.md | head -1
# Expected: doc cells match PRD+code exactly (parity confirmed). This is the audit's core claim (D1/D2).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 (MD056 table integrity): agy row pipe count == header row; no MD056 issue.
- [ ] Level 2 (pinpoint): `sed -n '85p' docs/providers.md` equals the verbatim target line.
- [ ] Level 2 (negative): no `` `-p` `` and no `--approval-mode` on line 85.
- [ ] Level 2 (positive): `(none)` and `--mode plan` present on line 85.
- [ ] Level 2 (collateral): the six untouched cells intact on line 85.
- [ ] Level 3 (sibling rows): qwen-code row (86) unchanged; opencode/codex `(none)` intact.
- [ ] Level 3 (invariant): no `.go` file changed; `go build ./...` and `go test ./...` green.
- [ ] Level 3 (forbidden): no `PRD.md` / `tasks.json` / `prd_snapshot.md` / `.gitignore` modified.

### Feature Validation

- [ ] The agy quick-reference row now documents the invocation the binary actually emits on agy v1.1.0
      (no `-p` flag; `--mode plan` read-only constraint) — matching `builtinAgy()` and PRD §12.5.1.
- [ ] The two drifted cells (audit D1 print flag, D2 tool-disable) are corrected to the audit's
      proposed values verbatim.
- [ ] Manual read confirms a user following the agy row would get a working command.

### Code Quality Validation

- [ ] Edit follows the table's existing conventions (`` `(none)` `` for no print flag;
      `Read-only constraint (`<flags>`)` for tool-disable) — precedent from opencode/codex/codex/cursor rows.
- [ ] No new pattern introduced; column count, cell order, and backtick wrapping all preserved.
- [ ] Scope respected: only line 85's two cells changed; no other line or file touched.

### Documentation & Deployment

- [ ] The corrected cell IS the user-facing documentation (Mode A target = docs/providers.md).
- [ ] The agy row is internally consistent with the rest of docs/providers.md (Tools-disable asymmetry
      section at ~line 92 classifies agy under "read-only constraint", which the cell now reflects accurately).
- [ ] No new environment variables, config, or build steps introduced.

---

## Anti-Patterns to Avoid

- ❌ Don't edit the qwen-code row (line 86) — it shares the OLD `-p` / `--approval-mode default` tokens
  but did NOT diverge (Gemini-CLI fork); it is OUT OF SCOPE. See Gotcha 1.
- ❌ Don't invent a new "no print flag" token (e.g. `""`, `—`, `n/a`) — use exactly `(none)`, matching
  the opencode/codex rows. See Gotcha 2.
- ❌ Don't add or remove any `|` — preserve the 9-pipe / 8-column structure (MD056). See Gotcha 3.
- ❌ Don't use a broad whole-row regex replace that could reflow the six correct cells — target the two
  drifted cells specifically. See Gotcha 4.
- ❌ Don't treat `Gemini 3.5 Flash (Low)` or the word "Gemini" as drift — it is the model display label
  agy runs, NOT the removed gemini provider. It MUST stay. See Gotcha 5.
- ❌ Don't fix the #76 date (line 88) or the provider count (lines 3/7/74/92) — those are sibling
  P2.M3.T1.S2. Don't fix docs/README.md — that is sibling P2.M3.T2. This subtask is ONLY line 85's two cells.
- ❌ Don't edit `PRD.md`, `internal/provider/builtin.go`, `providers/agy.toml`, `tasks.json`,
  `prd_snapshot.md`, or `.gitignore` — all READ-ONLY / forbidden.
- ❌ Don't rely on CI markdownlint — it is not in `.github/workflows/ci.yml`. The grep + pipe-count gate
  is the authority. See Gotcha 6.

---

## Confidence Score

**One-pass success likelihood: 10/10.** This is a single-line, two-cell documentation edit whose
contract is fully specified by a verbatim current→desired literal (audited as D1/D2), cross-checked
against four mutually-consistent sources of truth (compiled manifest `builtinAgy()`, reference
`providers/agy.toml`, PRD §12.5.1, and the drift audit). The implementing agent reads line 85, applies
the two exact cell replacements, and runs a deterministic gate (pinpoint equality + negative grep +
positive grep + six-cell-intact + pipe-count + sibling-row-integrity + docs-only-invariant). The
single hardest nuance — "agy and qwen-code both currently show the old tokens; edit ONLY agy" — is
called out with an explicit regression check on line 86. There is no code, no schema, no behavior, and
no external dependency; the only failure modes are editing the wrong row or clobbering a correct cell,
both caught by the verification gate.
