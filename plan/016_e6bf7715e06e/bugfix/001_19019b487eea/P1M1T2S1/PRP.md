name: "P1.M1.T2.S1 — Expand read-only constraint flags parenthetical to all 5 constrained providers (docs/how-it-works.md)"
description: >
  Fix documentation drift (Issue 2 / Defect 2): the "### Safety invariant" paragraph in
  docs/how-it-works.md (line 197) makes a universal claim ("Every built-in manifest constrains the
  agent to a read-only mode") but the read-only-constraint parenthetical names only `(codex, cursor)` —
  4 of 7 providers. Expand it to the exhaustive form `(codex, cursor, agy, qwen-code; opencode's \`run\`
  is read-only by design)` so the parenthetical partition (2 explicit-switch + 5 read-only-constrained =
  7) matches the universal claim and is consistent with docs/providers.md. One localized substring
  replacement on one line; the `(pi, claude)` parenthetical, the chrome-less sentence, and the
  cross-reference link are UNCHANGED. Documentation-only — no code, no test, no other file.

---

## Goal

**Feature Goal**: Make the "### Safety invariant" paragraph's provider categorization complete and
accurate: the read-only-constraint parenthetical names all 5 read-only-constrained providers (codex,
cursor, agy, qwen-code, opencode) instead of only 2 (codex, cursor), so the sentence's universal
"Every built-in manifest" claim is supported by an exhaustive partition (2 explicit-switch + 5
read-only-constrained = 7) and is consistent with docs/providers.md's "Tool-disable approach" column.

**Deliverable**: A single substring replacement on line 197 of docs/how-it-works.md —
`read-only constraint flags (codex, cursor).` → `read-only constraint flags (codex, cursor, agy,
qwen-code; opencode's \`run\` is read-only by design).`. No other substring, line, heading, file, code,
or test change.

**Success Definition**:
- Line 197's read-only-constraint parenthetical names all 5 providers (codex, cursor, agy, qwen-code,
  opencode) — exhaustive form.
- The `explicit tool-disable flags (pi, claude)` parenthetical is UNCHANGED (pi + claude are the only
  two explicit-switch providers).
- The rest of the paragraph (stdin/argv sentence, chrome-less sentence, the
  `[providers.md](providers.md#tools-disable-asymmetry)` link) is byte-unchanged.
- The provider list is consistent with docs/providers.md's "Tool-disable approach" column and the
  Chrome-disable column.
- No source/test/TOML file changed; the sibling docs/providers.md edit (P1.M1.T1.S1) is a different file.

## User Persona (if applicable)

**Target User**: A stagecoach user reading docs/how-it-works.md to understand the §18.1 repo-integrity
invariant — specifically, WHICH providers are read-only-constrained and HOW.

**Use Case**: "Does the safety invariant cover agy/opencode/qwen-code, or only codex/cursor?" — the user
reads the Safety invariant paragraph and gets the complete, accurate answer (all 5) without having to
cross-reference the providers table to discover the parenthetical was incomplete.

**Pain Points Addressed**: The stale `(codex, cursor)` read as exhaustive (no "e.g."), implying
agy/qwen-code/opencode fit neither the explicit-switch nor the read-only-constraint bucket — incorrect.
A reader would wrongly doubt the invariant's coverage of those three providers.

## Why

- **Issue 2 (Minor) / FR-C5 documentation completeness / PRD §12.7.1**: The paragraph's universal
  quantifier ("Every built-in manifest") demands an exhaustive partition. Naming only 4 of 7 providers
  undermines the claim's credibility and misrepresents the read-only-constraint category (which actually
  has 5 members). The fix makes the doc honest and complete.
- **Root cause** (no action required — context): commit `71e57c7a` (the chrome-disable docs commit)
  rewrote the entire safety paragraph to append the chrome-less sentence and carried the stale
  `(codex, cursor)` parenthetical forward instead of refreshing it. This subtask refreshes it.
- **Documentation-consistency goal**: docs/providers.md's "Tool-disable approach" column already lists
  all 5 read-only-constrained providers; the how-it-works prose should agree. This is the kind of
  cross-doc consistency a "Mode B" docs-sync task exists to maintain.

## What

**User-visible behavior**: The "### Safety invariant" paragraph accurately and exhaustively categorizes
all 7 built-in providers (2 explicit-switch + 5 read-only-constrained).

**Technical change (one substring on one line):**
- File: `docs/how-it-works.md`
- Line: 197
- OLD: `… or read-only constraint flags (codex, cursor). The agent receives …`
- NEW: `… or read-only constraint flags (codex, cursor, agy, qwen-code; opencode's \`run\` is read-only by design). The agent receives …`

### Success Criteria
- [ ] Line 197 read-only-constraint parenthetical is the exhaustive 5-provider form
- [ ] `explicit tool-disable flags (pi, claude)` parenthetical UNCHANGED
- [ ] Chrome-less sentence + `[providers.md](providers.md#tools-disable-asymmetry)` link UNCHANGED
- [ ] No other line/file changed; no code/test/TOML change
- [ ] The 5 named providers match docs/providers.md's "Tool-disable approach" column

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact file, exact line, exact old→new substring, the verified 5-provider mechanism table,
the unchanged `(pi, claude)` parenthetical, and the scope fences against the sibling docs/providers.md
subtask are all enumerated below.

### Documentation & References

```yaml
- file: docs/how-it-works.md
  why: "THE file to edit. The '### Safety invariant' heading is line 196; the paragraph itself is line
        197 (one long line). The target substring 'read-only constraint flags (codex, cursor).' is
        UNIQUE in the file (grep-confirmed)."
  pattern: "Sentence: 'Every built-in manifest constrains the agent to a read-only mode — either via
            explicit tool-disable flags (pi, claude) or read-only constraint flags (codex, cursor).'"
  critical: "Replace ONLY the '(codex, cursor).' portion (the read-only-constraint parenthetical + its
             trailing period). The 'explicit tool-disable flags (pi, claude)' parenthetical is CORRECT
             (pi + claude are the only two explicit-switch providers) — do NOT touch it. Preserve the
             em-dash '—' and the sentence's exact punctuation. The backticks around \`run\` in the new
             text are valid markdown inline-code."

- file: internal/provider/builtin.go
  why: "Source of truth for the 5 read-only-constrained providers and their mechanisms (verified
        2026-07-13): codex (--sandbox read-only + --ephemeral, line 381); cursor (--mode ask + --trust,
        line 425); agy (--mode plan, line 230); qwen-code (--approval-mode default, line 282); opencode
        (empty BareFlags — \`run\` is read-only by design, line 328). pi/claude are the 2 explicit-switch
        providers (correctly named in the unchanged parenthetical)."
  pattern: "builtinXxx() BareFlags per provider."
  critical: "These 5 are the COMPLETE read-only-constraint set (the TestBuiltinManifests_ChromeDisableContract
             test asserts FR-C4(b) for exactly these 5). The doc must agree."

- file: docs/providers.md
  why: "The cross-reference target (the paragraph links to providers.md#tools-disable-asymmetry). Its
        'Tool-disable approach' column lists all 5 read-only-constrained providers — the how-it-works
        prose must be consistent with it. (The sibling P1.M1.T1.S1 edits a DIFFERENT cell in this same
        file — the opencode Delivery column — no conflict with this subtask's how-it-works edit.)"

- docfile: plan/016_e6bf7715e06e/bugfix/001_19019b487eea/architecture/system_context.md
  why: "§Defect 2 is the full analysis: the stale parenthetical, the 5-provider mechanism table, the
        root-cause git-blame trace (commit 71e57c7a). The authoritative source for this fix."
  section: "Defect 2 (Minor): docs/how-it-works.md safety paragraph provider enumeration"

- docfile: plan/016_e6bf7715e06e/bugfix/001_19019b487eea/P1M1T1S1/PRP.md
  why: "The parallel sibling (docs/providers.md opencode Delivery cell). DIFFERENT FILE — no conflict.
        Read it to confirm the non-overlap (it edits providers.md line 82; this task edits how-it-works
        line 197)."
```

### Current Codebase tree (relevant slice)

```bash
docs/how-it-works.md     # THE edit: line 197 read-only-constraint parenthetical (codex, cursor) → exhaustive 5-provider form
internal/provider/builtin.go  # source of truth: 5 read-only-constrained providers' bare_flags (read-only)
docs/providers.md       # cross-reference target; sibling P1.M1.T1.S1 edits a different cell here (no conflict)
```

### Desired Codebase tree with files to be added

```bash
docs/how-it-works.md     # MODIFY (one substring on line 197)
# (no new files; no code/test/TOML changes)
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (one substring, one line): replace ONLY 'read-only constraint flags (codex, cursor).' →
     'read-only constraint flags (codex, cursor, agy, qwen-code; opencode's `run` is read-only by design).'
     The target substring is UNIQUE in the file (grep-confirmed). Do not reflow the whole paragraph;
     line 197 is one long line — edit the parenthetical in place. -->

<!-- CRITICAL (do NOT touch '(pi, claude)'): the 'explicit tool-disable flags (pi, claude)' parenthetical
     is CORRECT — pi and claude are the ONLY two explicit-switch providers (FR-C2). Only the
     read-only-constraint parenthetical is stale. Editing the wrong parenthetical would BREAK a correct
     claim. -->

<!-- CRITICAL (preserve the trailing period): the old substring ends with '(codex, cursor).' including
     the period that closes the sentence. The new substring must also end with the closing period:
     '... read-only by design).' — otherwise the next sentence ("The agent receives...") loses its
     preceding period and runs together. -->

<!-- GOTCHA (backticks around `run`): opencode's read-only mechanism is its `run` subcommand. Write
     opencode's `run` with markdown backticks (inline code) — matches the file's existing style of
     backticking subcommands/flags. The backticks render correctly inside a parenthetical. -->

<!-- GOTCHA (em-dash preservation): the sentence uses an em-dash '—' (U+2014) before 'either via'.
     Do not alter it or the surrounding punctuation. Only the '(codex, cursor)' token group changes. -->

<!-- GOTCHA (exhaustive vs e.g.): the item PREFERS the exhaustive form (matches the universal 'Every'
     claim). Use it. The 'e.g.' alternative is a fallback only if the exhaustive form reads as unwieldy
     — it does not (the parenthetical stays a reasonable length and the sentence reads naturally). -->

<!-- GOTCHA (no code/test impact): this is a markdown prose edit. `go build`/`go test` are unaffected;
     run them only as a no-regression sanity check (they confirm no file was accidentally touched), not
     as the primary gate. The primary gate is the grep on line 197 + markdownlint. -->

<!-- SCOPE: do NOT edit docs/providers.md (P1.M1.T1.S1 — the opencode Delivery cell — parallel, different
     file). do NOT edit any other line of how-it-works.md. do NOT edit source/test/TOML. do NOT add a
     changelog note. The substring swap is the entire change. -->
```

## Implementation Blueprint

### Data models and structure
None. A single literal substring replacement in one markdown paragraph.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT docs/how-it-works.md line 197 — expand the read-only-constraint parenthetical
  - LOCATE line 197 (the "### Safety invariant" paragraph; heading at line 196).
  - REPLACE the unique substring:
      OLD: read-only constraint flags (codex, cursor).
      NEW: read-only constraint flags (codex, cursor, agy, qwen-code; opencode's `run` is read-only by design).
  - The full sentence reads, after the edit:
      "Every built-in manifest constrains the agent to a read-only mode — either via explicit
       tool-disable flags (pi, claude) or read-only constraint flags (codex, cursor, agy, qwen-code;
       opencode's `run` is read-only by design)."
  - PRESERVE: the 'explicit tool-disable flags (pi, claude)' parenthetical; the em-dash '—'; the
    trailing period that closes the sentence; the rest of the paragraph (stdin/argv sentence, chrome-less
    sentence, the [providers.md](providers.md#tools-disable-asymmetry) link).
  - DEPENDENCIES: none.

Task 2: VERIFY the edit + scope boundaries
  - grep the new parenthetical is present on line 197:
      grep -n 'read-only constraint flags (codex, cursor, agy, qwen-code' docs/how-it-works.md  # → one hit, line 197
  - grep the unchanged parenthetical is intact:
      grep -n 'explicit tool-disable flags (pi, claude)' docs/how-it-works.md  # → one hit, line 197
  - grep the stale form is GONE:
      grep -n 'read-only constraint flags (codex, cursor).' docs/how-it-works.md  # → empty
  - markdownlint docs/how-it-works.md (no new errors; backticks/parens well-formed).
  - git diff --stat docs/how-it-works.md  # → exactly 1 line changed.
  - git diff --stat (whole repo)  # → ONLY docs/how-it-works.md changed by this subtask.
  - DEPENDENCIES: Task 1.
```

### Implementation Patterns & Key Details

```markdown
<!-- The edit is a literal substring swap inside one paragraph (line 197 is a single long line).
     OLD token group:  (codex, cursor).
     NEW token group:  (codex, cursor, agy, qwen-code; opencode's `run` is read-only by design).
     The partition now sums to 7: (pi, claude)=2 explicit-switch + (codex, cursor, agy, qwen-code,
     opencode)=5 read-only-constrained — matching the universal "Every built-in manifest" claim. -->

<!-- Sentence after the edit (only the parenthetical changed):
     "Every built-in manifest constrains the agent to a read-only mode — either via explicit tool-disable
      flags (pi, claude) or read-only constraint flags (codex, cursor, agy, qwen-code; opencode's `run`
      is read-only by design). The agent receives the diff via stdin/argv ..." -->
```

### Integration Points

```yaml
NO code / test / config / build changes. One markdown substring on one line.

FILE:
  - docs/how-it-works.md line 197 — read-only-constraint parenthetical expanded to all 5 providers

UNCHANGED: the (pi, claude) parenthetical; the rest of the paragraph; docs/providers.md (P1.M1.T1.S1);
  all source/test/TOML files; every other line of how-it-works.md.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Markdown lint (project ships .markdownlint.json) — the paragraph must stay well-formed
markdownlint docs/how-it-works.md 2>/dev/null || npx markdownlint-cli2 docs/how-it-works.md 2>/dev/null \
  || echo "(markdownlint not installed — skip; verify parens/backticks by eye)"
# Expected: no new errors. The backticks around `run` and the nested parens are valid markdown.

# Confirm exactly one line changed
git diff --stat docs/how-it-works.md   # Expected: 1 file, 1 insertion, 1 deletion
git diff docs/how-it-works.md          # Expected: a single -/+ line pair differing only in the parenthetical
```

### Level 2: Unit Tests (Component Validation)

```bash
# No code change — run the provider suite as a no-regression sanity check (confirms no accidental file touch)
go test ./internal/provider/...
# Expected: all pass (unaffected by a docs-only change).

# Full build/test (optional sanity)
go build ./... && go test ./...
# Expected: clean / all pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# Doc-only subtask — no runtime behavior. The within-scope proof is the grep on line 197 + the
# cross-reference consistency with docs/providers.md's "Tool-disable approach" column.
# Confirm the 5 named providers match that column:
grep -nE 'codex|cursor|agy|qwen-code|opencode' docs/providers.md | grep -iE 'read-only|constraint|run'
# Expected: the 5 read-only-constrained providers appear in docs/providers.md's tool-disable column,
#           consistent with the now-exhaustive how-it-works parenthetical.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the new exhaustive parenthetical is present on line 197
grep -n 'read-only constraint flags (codex, cursor, agy, qwen-code; opencode.s .run. is read-only by design)' docs/how-it-works.md \
  || grep -n 'codex, cursor, agy, qwen-code' docs/how-it-works.md
# Expected: one hit (line 197).

# Grep guard: the stale 2-provider form is GONE
grep -n 'read-only constraint flags (codex, cursor)\.' docs/how-it-works.md
# Expected: empty.

# Grep guard: the (pi, claude) parenthetical is UNCHANGED
grep -n 'explicit tool-disable flags (pi, claude)' docs/how-it-works.md
# Expected: one hit (line 197) — unchanged.

# Scope-boundary guard: ONLY docs/how-it-works.md changed by this subtask
git diff --stat -- docs/providers.md internal/ providers/ README.md
# Expected: empty (the sibling P1.M1.T1.S1 edits providers.md in its own session; this subtask touches
#           only how-it-works.md).

# Partition-sum guard: the two parentheticals name all 7 providers with no overlap
#   (pi, claude) + (codex, cursor, agy, qwen-code, opencode) = 7 unique providers
grep -n '(pi, claude)\|(codex, cursor, agy, qwen-code' docs/how-it-works.md
# Expected: both parentheticals on line 197; together they enumerate all 7 built-in providers.
```

## Final Validation Checklist

### Technical Validation
- [ ] markdownlint on docs/how-it-works.md shows no new errors
- [ ] `git diff docs/how-it-works.md` is exactly one line (the parenthetical substring swap)

### Feature Validation
- [ ] Line 197 read-only-constraint parenthetical names all 5 providers (codex, cursor, agy, qwen-code, opencode)
- [ ] `explicit tool-disable flags (pi, claude)` parenthetical UNCHANGED
- [ ] Chrome-less sentence + `[providers.md](providers.md#tools-disable-asymmetry)` link UNCHANGED
- [ ] The 5 named providers match docs/providers.md's "Tool-disable approach" column

### Scope-Boundary Validation
- [ ] No other line of docs/how-it-works.md changed
- [ ] No other file changed (docs/providers.md = P1.M1.T1.S1; source/test/TOML untouched)
- [ ] No code/test/TOML change; no changelog note added

### Documentation Quality
- [ ] The parenthetical partition sums to 7 (2 + 5), matching the universal "Every built-in manifest" claim
- [ ] Backticks around `run` render as inline code; nested parens well-formed
- [ ] Em-dash and trailing period preserved; the sentence reads naturally

---

## Anti-Patterns to Avoid

- ❌ Don't edit the `explicit tool-disable flags (pi, claude)` parenthetical — it is CORRECT (pi + claude are the only two explicit-switch providers). Only the `(codex, cursor)` read-only-constraint parenthetical is stale. Editing the wrong parenthetical would break a correct claim.
- ❌ Don't drop the trailing period — the old substring is `(codex, cursor).` including the sentence-closing period. The new substring must end `... read-only by design).` or the next sentence ("The agent receives...") runs together.
- ❌ Don't reflow the whole paragraph or split line 197 into multiple lines — edit the parenthetical in place; the paragraph is intentionally one line.
- ❌ Don't edit docs/providers.md — that is the sibling P1.M1.T1.S1 (opencode Delivery cell), a different file. No conflict, but don't touch it here.
- ❌ Don't change source/test/TOML — the drift is in the doc only; the 5 providers' mechanisms are already correctly implemented and tested (`TestBuiltinManifests_ChromeDisableContract` asserts FR-C4(b) for exactly these 5).
- ❌ Don't use the "(e.g. codex, cursor)" alternative unless the exhaustive form truly reads as unwieldy — it does not; the item prefers the exhaustive form because the sentence's "Every" quantifier warrants exhaustiveness.
- ❌ Don't add a changelog note, an editorial comment, or any extra prose — the substring swap is the entire change.
- ❌ Don't rename or reorder the providers in the parenthetical — use the item's specified order `(codex, cursor, agy, qwen-code; opencode's \`run\` is read-only by design)` so the doc agrees with the PRD issue's suggested fix and with docs/providers.md.

---

## Confidence Score: 10/10

This is a single literal substring replacement on a single verified line, with the target substring
grep-confirmed unique, the 5-provider enumeration verified against builtin.go (and against the
existing `TestBuiltinManifests_ChromeDisableContract` FR-C4(b) assertions), the unchanged `(pi, claude)`
parenthetical explicitly identified, and a clear sibling-task scope fence (docs/providers.md is a
different file). There is no code, no test, no ambiguity. The only conceivable failure modes — editing
the wrong parenthetical, dropping the trailing period, or touching docs/providers.md — are explicitly
guarded by the Level-4 grep checks and the Gotchas.
