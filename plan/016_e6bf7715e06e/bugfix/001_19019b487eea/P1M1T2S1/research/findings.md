# Research: P1.M1.T2.S1 — Expand read-only constraint flags parenthetical (docs/how-it-works.md)

**Scope**: One localized edit to docs/how-it-works.md line 197 — the "### Safety invariant"
paragraph. Replace the incomplete parenthetical `(codex, cursor)` with an exhaustive enumeration of all
5 read-only-constrained providers. Documentation-only — no code, no test, no other file. All facts
verified against the working tree (2026-07-13).

## 1. The exact edit (one substring on one line)

**File:** docs/how-it-works.md
**Line:** 197 (the entire "### Safety invariant" paragraph is one long line)

**OLD substring (unique — grep-confirmed it appears exactly once in the file):**
```
read-only constraint flags (codex, cursor).
```

**NEW substring (PREFERRED exhaustive form — matches the universal "Every built-in manifest" claim):**
```
read-only constraint flags (codex, cursor, agy, qwen-code; opencode's `run` is read-only by design).
```

**ALTERNATIVE (if the exhaustive form reads as unwieldy):**
```
read-only constraint flags (e.g. codex, cursor).
```
Use the exhaustive form (the item's preferred choice) — it is precise and the sentence's "Every"
quantifier warrants exhaustiveness. The sentence still reads naturally.

## 2. The full sentence BEFORE and AFTER (for context — only the parenthetical changes)

BEFORE (the relevant sentence only):
> Every built-in manifest constrains the agent to a read-only mode — either via explicit tool-disable
> flags (pi, claude) or read-only constraint flags (codex, cursor).

AFTER:
> Every built-in manifest constrains the agent to a read-only mode — either via explicit tool-disable
> flags (pi, claude) or read-only constraint flags (codex, cursor, agy, qwen-code; opencode's `run` is
> read-only by design).

## 3. What stays UNTOUCHED on line 197

- The `explicit tool-disable flags (pi, claude)` parenthetical — CORRECT (pi + claude are the ONLY two
  explicit-switch providers; FR-C2). Do NOT change.
- The remainder of the paragraph: the stdin/argv sentence, the "Every provider also renders chrome-less"
  sentence, and the `See [providers.md](providers.md#tools-disable-asymmetry)` cross-reference link.
- The section heading `### Safety invariant` (line 196).

## 4. Verified: all 5 read-only-constrained providers + their mechanisms (builtin.go)

Cross-checked against internal/provider/builtin.go bare_flags (grep-verified 2026-07-13):

| Provider | Constraint mechanism (builtin.go) | Line |
|----------|-----------------------------------|------|
| codex | `--sandbox read-only` (+ `--ephemeral`) | 381 |
| cursor | `--mode ask` (+ `--trust`) | 425 |
| agy | `--mode plan` | 230 |
| qwen-code | `--approval-mode default` | 282 |
| opencode | (empty BareFlags) `run` subcommand is read-only by design | 328 |

The 2 explicit-switch providers (pi, claude) are correctly named in the UNCHANGED parenthetical.

## 5. Why the exhaustive form is right (the universal claim)

The sentence opens with "Every built-in manifest constrains the agent to a read-only mode" — a UNIVERSAL
quantifier over all 7 providers. The two parentheticals partition those 7 into the two categories:
- explicit tool-disable flags (pi, claude) = 2 providers
- read-only constraint flags (codex, cursor, agy, qwen-code; opencode) = 5 providers

2 + 5 = 7 ✓. The exhaustive form makes the partition complete and consistent with the universal claim,
with docs/providers.md's "Tool-disable approach" column, and with the Chrome-disable column. The stale
`(codex, cursor)` form named only 4 of 7 and (reading as exhaustive) implied agy/qwen-code/opencode fit
neither bucket — incorrect.

## 6. COORDINATION WITH THE PARALLEL SIBLING (no conflict)

- P1.M1.T1.S1 (Implementing in parallel): edits docs/providers.md line 82 (opencode Delivery cell
  `positional` → `stdin`). DIFFERENT FILE from docs/how-it-works.md. No merge conflict.
- THIS task (P1.M1.T2.S1): edits docs/how-it-works.md line 197 ONLY.
- P1.M1.T3.S1 (Planned): a broader verification task — separate; do not preempt it.

## 7. Validation

- The edit is a markdown prose change on one line. No build/test impact.
- Primary gate: grep confirming line 197 contains the exhaustive parenthetical AND that
  `explicit tool-disable flags (pi, claude)` is unchanged.
- markdownlint (project ships .markdownlint.json) — the line must stay well-formed (no broken parens/
  backticks). The backticks around `run` are valid inline-code markdown.
- `go build ./...` / `go test ./...` as a no-regression sanity check (the change is doc-only; they are
  unaffected, but run them to confirm no accidental file touch).

## 8. Anchors (verified)

| What | Location |
|---|---|
| "### Safety invariant" heading | docs/how-it-works.md:196 |
| The paragraph to edit (one line) | docs/how-it-works.md:197 |
| Target substring `read-only constraint flags (codex, cursor)` | unique (grep-confirmed once) |
| Unchanged parenthetical `explicit tool-disable flags (pi, claude)` | same line, same paragraph |
| Cross-ref link `[providers.md](providers.md#tools-disable-asymmetry)` | end of the same paragraph |
| 5 providers' mechanisms (source of truth) | internal/provider/builtin.go (codex:381, cursor:425, agy:230, qwen-code:282, opencode:328) |
| docs/providers.md "Tool-disable approach" column | the cross-reference target (consistency check) |
