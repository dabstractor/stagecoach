# P1.M1.T3.S1 — Verification Ground Truth (binary vs docs provider consistency)

This is a **verification-gate** subtask (Mode B). It runs LAST, after P1.M1.T1.S1 (Complete) and
P1.M1.T2.S1 (in-flight). Its job is to PROVE zero documentation drift, not to write new content.
This file records the ground truth established by direct measurement so the PRP's "expected" values
are verified, not guessed.

## 1. Binary `prompt_delivery` for all 7 providers (measured 2026-07-13)

Build: `go build -o /tmp/stagecoach_verify ./cmd/stagecoach` (clean, no output).
Query: `/tmp/stagecoach_verify providers show <name> | grep prompt_delivery`.

| Provider   | Binary `prompt_delivery` |
|------------|--------------------------|
| pi         | `stdin`                  |
| claude     | `stdin`                  |
| opencode   | `stdin`                  |
| codex      | `stdin`                  |
| cursor     | `positional`             |
| agy        | `stdin`                  |
| qwen-code  | `stdin`                  |

So: **6× stdin + 1× positional (cursor)**. This is the authoritative set the docs must mirror.

## 2. docs/providers.md Delivery column — CURRENT state (table at line 74)

The 7-provider table (header line 78; rows 80-86). Delivery = 2nd column:

| Line | Provider  | Delivery cell | Binary | Match? |
|------|-----------|---------------|--------|--------|
| 80   | pi        | stdin         | stdin  | ✅ |
| 81   | claude    | stdin         | stdin  | ✅ |
| 82   | opencode  | stdin         | stdin  | ✅ (T1.S1 ALREADY APPLIED — was `positional`) |
| 83   | codex     | stdin         | stdin  | ✅ |
| 84   | cursor    | positional    | positional | ✅ |
| 85   | agy       | stdin         | stdin  | ✅ |
| 86   | qwen-code | stdin         | stdin  | ✅ |

→ **7/7 match today.** Check (a) is expected to PASS (verify it still holds post-rebuild).

## 3. docs/how-it-works.md safety paragraph (line 197) — T2.S1 dependency

CURRENT working-tree state (T2.S1 NOT yet landed):
> "... either via explicit tool-disable flags (pi, claude) or **read-only constraint flags
> (codex, cursor).** The agent receives the diff via stdin/argv ..."

This is the STALE form. T2.S1's contract changes the parenthetical to the exhaustive 5-provider form:
> "... read-only constraint flags **(codex, cursor, agy, qwen-code; opencode's `run` is read-only by design).**"

⇒ T3.S1's check (b) must verify the POST-T2.S1 state. If the exhaustive form is NOT present when T3.S1
runs, that is a sequencing failure OR a drift T3.S1 is empowered to fix (contract: "If any
inconsistency is found, fix it in the same doc file"). Apply T2.S1's exact contract substring.

## 4. Residual opencode↔positional drift (check c) — measured

`grep -rn 'positional.*opencode\|opencode.*positional' docs/ README.md` → **exit 1, ZERO matches.**

All remaining `positional` occurrences in docs/README are GENERIC (never tied to opencode):
- `docs/providers.md:22` — schema enum: "`stdin`, `positional`, or `flag`"
- `docs/providers.md:64` — rendering pseudo-code: "positional → trailing argument"
- `docs/providers.md:84` — cursor row Delivery (correct: cursor IS positional)
- `README.md:308` — config example comment: "# stdin | positional | flag"

→ Check (c) PASSES. No README/docs-README opencode-delivery fix is needed (opencode appears in those
files only in provider-LIST prose, never tied to a delivery method).

## 5. Tools-disable asymmetry section (docs/providers.md lines 90-100) — check (d)

- Line 94: "Explicit switch **(pi, claude)**" — correct (the only 2 explicit-switch providers).
- Line 96: "Read-only constraint **(codex, cursor)**: ... opencode's `run` subcommand is inherently
  non-interactive and read-only." — names codex+cursor in the parenthetical, opencode in prose.
  (agy/qwen-code are NOT in this bullet; they appear in the table column and the chrome bullet.)
- Line 100: "Chrome is a separate axis (all providers): ... providers that do not document the
  limitation honestly **(codex, cursor, opencode, agy, qwen-code)**" — names ALL 5 constrained providers.

⇒ Check (d) target — the 'Chrome is a separate axis' bullet (line 100) — is COMPLETE (all 5). PASS.

**Scope note (do NOT expand line 96):** the 'Read-only constraint (codex, cursor)' parenthetical on
line 96 is NOT a PRD-defined defect (PRD Issue 2 is scoped to how-it-works line 197 ONLY). The contract
point (d) scopes verification to the chrome bullet (line 100), which is complete. The full 5-provider
enumeration already lives in (i) the table's Tool-disable column (lines 82-86, one row each) and (ii)
the chrome bullet (line 100). Expanding line 96 would be out-of-scope scope creep and a surprise edit —
T3.S1 must NOT do it. (If the agent feels strongly, flag it in the verification report as an OPTIONAL
observation, but make no edit.)

## 6. markdownlint baseline (gate accuracy)

`npx markdownlint-cli2 docs/providers.md docs/how-it-works.md` → **0 errors** on BOTH files.
`.markdownlint.json` = `{default:true, MD013:false, MD033:false, MD060:false}`.
Tool: markdownlint-cli2 v0.23.0 (available via `npx --no-install`).

⇒ The lint gate is "still 0 errors" — a strong, clean invariant. T2.S1's backtick-`run` parenthetical
must not introduce an MD error (it doesn't — inline code in a parenthetical is well-formed).

## 7. Expected verification verdict

After T1.S1 (done) + T2.S1 (lands per contract), ALL FOUR checks (a)-(d) are expected to PASS with
**NO edits required** from T3.S1. T3.S1's deliverable is therefore primarily a VERIFICATION REPORT +
the green gates; incidental edits apply ONLY if a check unexpectedly fails (then fix per the relevant
prior subtask's contract and re-verify). The critical_findings.md confirms this: "Mode B sweep is a
verification step, not a writing step."
