name: "P1.M1.T1.S1 — Correct opencode row Delivery cell from 'positional' to 'stdin' in docs/providers.md"
description: >
  Fix documentation drift (Issue 1): the opencode row's Delivery cell in the 7-provider table in
  docs/providers.md reads `positional`, but the shipped binary, builtin.go, and providers/opencode.toml
  all use `stdin` (revised to avoid the 128 KB MAX_ARG_STRLEN ceiling on ~300 KB diffs). Change exactly
  one cell on one line (line 82) from the literal word `positional` to the literal word `stdin`. No code,
  no test, no other doc line changes.

---

## Goal

**Feature Goal**: Make the opencode row's `Delivery` cell in `docs/providers.md`'s 7-provider table
match the shipped binary's `prompt_delivery` value (`stdin`), restoring the file's stated
"docs track the binary byte-for-byte" contract (docs/README.md).

**Deliverable**: A single one-word edit on line 82 of `docs/providers.md` — the opencode row's Delivery
cell changes from `positional` to `stdin`. No other cell, row, header, file, or prose changes.

**Success Definition**:
- docs/providers.md line 82 opencode row Delivery cell reads `stdin`.
- cursor (line 84) is UNCHANGED (still `positional` — cursor is the only genuinely-positional provider).
- All 7 provider Delivery cells now match the binary: pi=stdin, claude=stdin, opencode=stdin,
  codex=stdin, cursor=positional, agy=stdin, qwen-code=stdin.
- No other doc file references opencode delivery as `positional` (grep-confirmed).
- The file remains valid markdown; the table column alignment is unchanged.

## Why

- **Issue 1 (Major) / FR-C5 documentation-honesty**: The whole point of revising opencode from
  `positional` to `stdin` (commit `010ecee`, verified 2026-07-08 against opencode 1.1.23) was to avoid
  arg-length truncation on ~300 KB diffs (Linux's 128 KB `MAX_ARG_STRLEN` ceiling). Documenting
  `positional` hides that behavior and would mislead anyone debugging an opencode large-diff failure, or
  building tooling that assumes positional delivery. It also directly contradicts the file's stated
  byte-faithfulness contract and PRD §12.6.
- **Root cause** (no action required — context): commit `b8f081d` ("Add Chrome-disable column to
  providers.md table") rewrote line 82 to add the Chrome-disable column and carried the stale delivery
  value forward. This subtask corrects that drift.

## What

**User-visible behavior**: The `docs/providers.md` provider table correctly reports opencode's delivery
method as `stdin`, consistent with the binary, `builtin.go`, and `providers/opencode.toml`.

**Technical change (one word on one line):**
- File: `docs/providers.md`
- Line: 82
- Change: the Delivery cell (2nd column) of the `| \`opencode\` |` row from `positional` → `stdin`.

### Success Criteria
- [ ] Line 82 opencode Delivery cell reads `stdin`
- [ ] Line 84 cursor Delivery cell UNCHANGED (`positional`)
- [ ] No other line in providers.md changed
- [ ] No other file changed
- [ ] grep finds no remaining `opencode ... positional` in docs/ or README.md

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact file, exact line, exact old→new text, the four authoritative sources confirming `stdin`, the one provider (cursor) that must NOT be changed, and the scope boundaries against the sibling how-it-works.md subtask are all enumerated below (verified by reading the file and grepping the source).

### Documentation & References

```yaml
- file: docs/providers.md
  why: "THE file to edit. The '## The 7 built-in providers' table spans lines 77-86; the opencode row is line 82. The Delivery column is the 2nd column (Provider, Delivery, Print flag, …)."
  pattern: "Row shape: | `opencode` | <Delivery> | (none) | `-m` | (user must set) | (prepended) | Read-only constraint (`run` subcommand) | no per-surface switch; read-only by design — documented limitation | — no |"
  gotcha: "Change ONLY the 2nd cell (Delivery). The other 8 cells in the opencode row are correct. Do NOT touch any other row (especially cursor, line 84, which is legitimately positional)."

- file: internal/provider/builtin.go
  why: "Source of truth #1. builtinOpenCode() doc comment records 'REVISION (delivery): PromptDelivery=\"stdin\" (§12.6 said \"positional\")' and the struct field is PromptDelivery: strPtr(\"stdin\")."
  gotcha: "builtin.go ALSO has a `PromptDelivery: strPtr(\"positional\")` (cursor, ~line 418). That is CORRECT for cursor — cursor is the only genuinely-positional provider. Do not be fooled into 'fixing' cursor."

- file: providers/opencode.toml
  why: "Source of truth #2. Line 45: `prompt_delivery = \"stdin\"  # REVISED from §12.6 \"positional\"`. The RENDERED COMMAND header here already shows stdin piping."

- file: docs/providers.md (cursor row, line 84)
  why: "The row that must NOT change. cursor Delivery = positional is CORRECT (matches builtin.go:418). After the fix, cursor is the ONLY `positional` Delivery in the table."

- docfile: plan/016_e6bf7715e06e/bugfix/001_19019b487eea/architecture/system_context.md
  why: "The full binary cross-check + git-blame root-cause trace for Defect 1."
  section: "Defect 1"

- docfile: plan/016_e6bf7715e06e/bugfix/001_19019b487eea/P1M1T1S1/research/verification_deltas.md
  why: "The verified all-7 Delivery values table + the no-cascade grep confirmation + scope boundaries."
```

### Current Codebase tree (relevant slice)

```bash
docs/providers.md          # THE edit: line 82 opencode Delivery cell positional → stdin
internal/provider/builtin.go   # source of truth: opencode=stdin (doc comment + struct field); cursor=positional (line ~418, correct)
providers/opencode.toml    # source of truth: prompt_delivery = "stdin" (line 45)
README.md, docs/how-it-works.md, docs/README.md  # grep-confirmed: NO opencode+positional reference (no change)
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (one-word edit): change ONLY the 2nd cell (Delivery) of line 82. The opencode row's
     other 8 cells (Print flag, Model flag, Default model, System prompt flag, Tool-disable approach,
     Chrome-disable, Stager?) are correct. Do not re-flow or re-align the row. -->

<!-- CRITICAL (do NOT touch cursor): line 84 cursor Delivery = positional is CORRECT — cursor is the
     only genuinely-positional provider (builtin.go:418 PromptDelivery: strPtr("positional")).
     builtin.go contains a `positional` for a reason; do not "fix" it. After this change cursor is the
     ONLY `positional` Delivery in the table, which is correct. -->

<!-- GOTCHA (scope): Issue 2 (docs/how-it-works.md "### Safety invariant" parenthetical naming only
     codex/cursor) is a SEPARATE subtask (P1.M1.T2.S1). Do NOT edit how-it-works.md here. -->

<!-- GOTCHA (no code/test change): this is a markdown doc edit. `go build`/`go test` are unaffected;
     run them only as a no-regression sanity check, not as the primary gate. The primary gate is the
     grep confirming line 82 reads `stdin` and the markdown-lint check. -->
```

## Implementation Blueprint

### Data models and structure

No data models. A single literal-word replacement in one markdown table cell.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT docs/providers.md line 82 — opencode Delivery cell: positional → stdin
  - LOCATE line 82 (the `| \`opencode\` |` table row in the "## The 7 built-in providers" table).
  - CHANGE the 2nd cell (Delivery) from the literal `positional` to the literal `stdin`.
  - BEFORE: | `opencode` | positional | (none) | `-m` | (user must set) | (prepended) | Read-only constraint (`run` subcommand) | no per-surface switch; read-only by design — documented limitation | — no |
  - AFTER:  | `opencode` | stdin      | (none) | `-m` | (user must set) | (prepended) | Read-only constraint (`run` subcommand) | no per-surface switch; read-only by design — documented limitation | — no |
    (only the Delivery cell changes; preserve the exact cell padding/alignment the table currently uses)
  - DO NOT touch: any other cell in the opencode row, any other row (especially cursor line 84), the
    table header (line 78), the separator row (line 79), or any prose.
  - DEPENDENCIES: none.

Task 2: VERIFY no other doc reference to opencode delivery as 'positional'
  - RUN: grep -rn "opencode" docs/ README.md | grep -i "positional"
  - EXPECTED: empty (no remaining opencode+positional reference). If any appears, fix it to stdin;
    the prior research found none.
  - DEPENDENCIES: Task 1.

Task 3 (verification, optional but recommended): binary cross-check
  - RUN: go build -o /tmp/stagecoach ./cmd/stagecoach && /tmp/stagecoach providers show opencode | grep prompt_delivery
  - EXPECTED: `prompt_delivery = 'stdin'` (confirms the doc now matches the binary).
  - DEPENDENCIES: none (independent confirmation of the authoritative value).
```

### Implementation Patterns & Key Details

```markdown
<!-- The edit is a literal token swap in one markdown table cell. Preserve the row's existing
     column alignment. The table uses simple `| cell |` padding; match the surrounding rows' style. -->
```

### Integration Points

```yaml
NO code / test / config / build changes. One markdown table cell.

FILE:
  - docs/providers.md line 82 — opencode Delivery cell: positional → stdin

UNCHANGED: every other cell/row/header; docs/how-it-works.md (Issue 2 = P1.M1.T2.S1); all source/test/TOML files.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Markdown lint (the project ships .markdownlint.json) — the table must stay well-formed
markdownlint docs/providers.md 2>/dev/null || npx markdownlint-cli2 docs/providers.md 2>/dev/null \
  || echo "(markdownlint not installed — skip; verify table shape by eye)"
# Expected: no new errors. The edit preserves column count and alignment.

# Confirm only line 82 changed (the diff is exactly one line, one word)
git diff --stat docs/providers.md      # Expected: 1 file, 1 insertion, 1 deletion
git diff docs/providers.md             # Expected: a single -/+ line pair differing only in positional→stdin
```

### Level 2: Unit Tests (Component Validation)

```bash
# No code change — but run the provider suite as a no-regression sanity check
go test ./internal/provider/... 
# Expected: all pass (unaffected by a docs-only change).

# Full build/test (optional sanity)
go build ./... && go test ./...
# Expected: clean / all pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# Binary cross-check: the doc now matches the shipped binary
go build -o /tmp/stagecoach ./cmd/stagecoach
/tmp/stagecoach providers show opencode | grep prompt_delivery
# Expected: prompt_delivery = 'stdin'   (and docs/providers.md:82 opencode Delivery cell now also reads stdin)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: line 82 opencode Delivery is now stdin
sed -n '82p' docs/providers.md | grep -q '| `opencode` | stdin |' && echo "OK: opencode=stdin" || echo "FAIL"
# Expected: OK: opencode=stdin

# Grep guard: cursor (line 84) is UNCHANGED — still positional (the correct value)
sed -n '84p' docs/providers.md | grep -q '| `cursor` | positional |' && echo "OK: cursor=positional (unchanged)" || echo "FAIL"
# Expected: OK: cursor=positional (unchanged)

# Grep guard: no remaining opencode+positional anywhere in docs or README
grep -rn "opencode" docs/ README.md | grep -i "positional"
# Expected: empty.

# Grep guard: the diff is exactly one line
test "$(git diff docs/providers.md | grep -c '^[-+]')" -eq 2 && echo "OK: exactly 1 line changed" || echo "FAIL: too many lines changed"
# Expected: OK: exactly 1 line changed (the - line and the + line = 2 diff markers)

# All-7 Delivery consistency (post-fix): print each provider's Delivery cell
for p in pi claude opencode codex cursor agy qwen-code; do
  grep -nE "^\| \`${p}\` \|" docs/providers.md | head -1
done
# Expected Delivery values: stdin, stdin, stdin, stdin, positional, stdin, stdin
```

## Final Validation Checklist

### Technical Validation
- [ ] markdownlint on docs/providers.md shows no new errors
- [ ] `git diff docs/providers.md` is exactly one line (positional → stdin)

### Feature Validation
- [ ] Line 82 opencode Delivery cell reads `stdin`
- [ ] Line 84 cursor Delivery cell UNCHANGED (`positional`)
- [ ] All 7 Delivery cells match the binary (stdin, stdin, stdin, stdin, positional, stdin, stdin)
- [ ] `stagecoach providers show opencode` prints `prompt_delivery = 'stdin'` (binary matches doc)

### Scope-Boundary Validation
- [ ] No other cell in the opencode row changed
- [ ] No other row / header / separator changed
- [ ] docs/how-it-works.md UNCHANGED (Issue 2 is P1.M1.T2.S1)
- [ ] No source/test/TOML file changed
- [ ] grep finds no remaining `opencode ... positional` in docs/ or README.md

### Documentation Quality
- [ ] Table column alignment preserved
- [ ] The file's byte-faithfulness contract (doc matches binary) restored for the opencode Delivery cell

---

## Anti-Patterns to Avoid

- ❌ Don't touch any cell other than the opencode Delivery cell on line 82 — the other 8 cells are correct.
- ❌ Don't "fix" cursor (line 84) — its `positional` is correct; cursor is the only genuinely-positional provider (builtin.go:418).
- ❌ Don't edit docs/how-it-works.md — the safety-paragraph enumeration is Issue 2 (P1.M1.T2.S1), a separate subtask.
- ❌ Don't re-flow or re-align the whole table — change exactly one word; preserve existing padding.
- ❌ Don't change source/test/TOML files — the drift is in the doc only; the binary, builtin.go, and providers/opencode.toml already say `stdin`.
- ❌ Don't add commentary or a changelog note in the file — the cell swap is the entire change.

---

## Confidence Score: 10/10

This is a single literal-word replacement on a single verified line, with four independent sources
confirming the target value (`stdin`), a grep-confirmed absence of cascade edits, and a clearly
identified "do not touch" row (cursor). There is no code, no test, no ambiguity. The only conceivable
failure mode — editing the wrong cell/row or also changing cursor — is explicitly guarded by the Level-4
grep checks (line 82 = stdin; line 84 = positional unchanged; diff = exactly one line).
