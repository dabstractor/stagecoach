# Research Findings — P2.M1.T1.S1: Fix the agy/codex/opencode stager rows + prose + Tools-disable asymmetry

## ⚠️ §0 HEADLINE — the cursor discrepancy (a D1-class drift the contract + arch doc BOTH missed)

**The contract and the architecture doc are both STALE about `cursor`.** Both say cursor's `TooledFlags`
is nil → "— no". **The committed code says otherwise:**

```
$ git log --oneline -5 -- internal/provider/builtin.go
f88042c feat(provider): add cursor tooled flags          ← cursor stager, COMMITTED at HEAD
c9e5fb3 feat(provider/opencode): make opencode a stager (unscoped)
f40ac87 feat(provider/codex): make codex a stager (unscoped)
c34f480 feat(provider/agy): make agy a stager — close §12.5.1.1 item 4
```

`internal/provider/builtin.go` (HEAD, `func builtinCursor()` L447, TooledFlags L469):
```go
// TOOLED (stager) — VERIFIED 2026-07-09 (cursor-agent) end-to-end: stages exactly the requested
// paths. ... `--yolo` (= --force) auto-approves tool calls non-interactively; `--trust` skips the
// workspace prompt. cursor defaults to its cwd as the workspace, so NO TooledRepoDirFlag.
// §12.7.1 UNSCOPED (like pi/agy/codex/opencode).
TooledFlags: []string{
    "--trust",
    "--yolo",
},
```
- builtin.go is NOT dirty (committed at HEAD, no working-tree change).
- builtin.go mtime = **Aug 8 00:26**; the architecture docs (`provider_docs_state.md`, `findings_and_divergences.md`,
  `role_defaults_drift.md`) were authored **Aug 7 22:38–22:40** — ~2h BEFORE the cursor commit. So the arch
  docs' "cursor = nil, NO stager" read predates the cursor TooledFlags and is simply stale (the verifier
  missed cursor the same way D1 nearly missed opencode).

**∴ The ACTUAL shipped stager-capable set is SIX, not five:** pi, claude, agy, opencode, codex, **cursor**
(all non-nil TooledFlags). Only **qwen-code** is nil (genuinely cannot stage).

**Resolution (the PRP follows the CODE):** docs/providers.md's cursor row → `✓ yes (unscoped)`. This is the
ONLY way to satisfy the task's own cardinal rule + verification step:
- Contract RESEARCH NOTE: "**Code (`internal/provider/builtin.go`) is the source of truth for stager CAPABILITY** (non-nil TooledFlags ⇔ stager-capable)."
- Contract step 5/OUTPUT: "**confirm every Stager? cell matches builtin.go's TooledFlags (non-nil ⇔ stager-capable) for all seven providers**" / "whose provider table + prose **match the code**."
- The D1 precedent (`findings_and_divergences.md`): the project explicitly resolved the IDENTICAL opencode
  contradiction by following the code over the PRD — "the PRD's own verification step MANDATES this; step 3's
  opencode instruction is the part that's wrong." Step (a)'s "leave cursor as — no" instruction is, by the
  same logic, the part that is wrong.

Leaving cursor "— no" would ship a doc that STILL contradicts the code — the exact failure this task exists
to fix. **This is a deliberate, evidence-based deviation from contract step (a), flagged for human confirmation.
The implementer/human can trivially revert the single cursor cell if they disagree.**

## §1 The authoritative capability matrix — builtin.go (verified direct read 2026-08-09)

| # | Provider | func @line | TooledFlags | TooledRepoDirFlag | Stager? | Scope |
|---|----------|-----------|-------------|-------------------|---------|-------|
| 1 | pi       | L50  (TooledFlags L97)  | non-nil (`--no-*` chrome-strip + tools) | nil | **YES** | UNSCOPED |
| 2 | claude   | L124 (TooledFlags L149) | non-nil (`--allowed-tools Bash(git add:*,git apply:*,git status:*,git diff:*),Read,Edit`) | nil | **YES** | **STRUCTURALLY-scoped** |
| 3 | agy      | L223 (TooledFlags L244) | non-nil (`--mode accept-edits --dangerously-skip-permissions`) | `--add-dir` (L248) | **YES** | UNSCOPED |
| 4 | qwen-code| L286 (TooledFlags nil @L303) | **NIL** | nil | **NO** | — |
| 5 | opencode | L333 (TooledFlags L353) | non-nil (`--agent build`) | nil | **YES** | UNSCOPED |
| 6 | codex    | L395 (TooledFlags L417) | non-nil (`--sandbox danger-full-access`) | nil | **YES** | UNSCOPED |
| 7 | cursor   | L447 (TooledFlags L469) | non-nil (`--trust --yolo`) | nil | **YES** | UNSCOPED |

**Result: 6 stager-capable (pi, claude, agy, opencode, codex, cursor); 1 not (qwen-code).** claude is the
ONLY structurally-scoped stager; the other five are UNSCOPED (safety = §17.6 stager prompt + HEAD-movement
guard + verifyFreezeSubset/FR-M1c, NOT flag-scoping — per builtin.go comments + spec §12.7.1).

## §2 docs/providers.md — the exact regions to edit (current line numbers, verified)

| Region | Lines | What | Stale? |
|--------|-------|------|--------|
| **A** main provider table `Stager?` col | L80-86 | pi `✓ yes`; claude `✓ yes`; opencode `— no`; codex `— no`; cursor `— no`; agy `— no`; qwen-code `— no ⚠️` | opencode, codex, cursor, agy stale (agy/codex/opencode per contract + code; **cursor per code only — §0**) |
| **B** prose note after the table | L88 | "...`agy` ... cannot serve as a stager (empty `tooled_flags`)." + "qwen-code ... cannot serve as a stager (empty `tooled_flags`)." | agy clause stale (has tooled_flags now); qwen-code clause correct |
| **C** "Tools-disable asymmetry" section (bare mode) | L90-100 | bare-mode tool-disable (explicit switch vs read-only constraint). Accurate for BARE mode. | not stale, but contract wants stager-scope content added here |
| **C′** "Tooled mode and the stager role" section | L102-118 | the 3-layer stager safety; layer 1 contrasts claude(scoped) vs pi(unscoped) — does NOT name the full set, does NOT mention verifyFreezeSubset | **the thematically-correct home for the contract (c) stager-scope sentence** |
| **D** per-role models table footer | L142 | "**Stager column:** A value of *(cannot)* means the provider lacks `tooled_flags`... (FR-D4 fallback — currently pi or claude)." | BOTH clauses stale: "(cannot) = lacks tooled_flags" is FALSE for agy/opencode/codex/cursor (they HAVE tooled_flags); "currently pi or claude" is wrong (now 6 stager-capable) |

### The contract's "(L90-100)" location for region (c) — interpretation
Contract step (c) cites "Tools-disable asymmetry section (L90-100)". But in docs/providers.md the "## Tools-
disable asymmetry" heading (L90) is a **bare-mode-only** section (explicit-switch vs read-only-constraint);
the stager scope model lives in the **adjacent** "## Tooled mode and the stager role" section (L102-118),
whose layer-1 bullet ALREADY contrasts claude(scoped) vs pi(unscoped). Placing stager-capability content in
the bare-mode section (L90-100) would be thematically wrong and redundant with L102-118. **∴ The PRP places
the contract-(c) stager-scope sentence in "## Tooled mode and the stager role" (L102-118), extending the
existing layer-1 scoped/unscoped discussion** — the faithful realization of the contract's CONTENT intent
(the line-number was a planning approximation; the two docs sections together mirror PRD §12.7.1's single
section). Flagged as a minor location interpretation.

## §3 Region-by-region edit plan (follows the code)

### Region A — main table `Stager?` column (L80-86)
| Line | Provider | Current | NEW | Source |
|------|----------|---------|-----|--------|
| L80 | pi | `✓ yes` | `✓ yes` | UNCHANGED (contract: leave pi bare) |
| L81 | claude | `✓ yes` | `✓ yes (scoped)` | contract (a): annotate; only structurally-scoped |
| L82 | opencode | `— no` | `✓ yes (unscoped)` | contract (a) + code |
| L83 | codex | `— no` | `✓ yes (unscoped)` | contract (a) + code |
| L84 | cursor | `— no` | `✓ yes (unscoped)` | **§0 DEVIATION (code; contract wrong)** |
| L85 | agy | `— no` | `✓ yes (unscoped)` | contract (a) + code |
| L86 | qwen-code | `— no ⚠️` | `— no ⚠️` | UNCHANGED (code: TooledFlags nil) |
(5 cells change: claude annotated; opencode/codex/agy flipped per contract; **cursor flipped per code**. 2 unchanged: pi, qwen-code.)

### Region B — prose note (L88)
Rewrite the `agy` sentence (drop "cannot serve as a stager (empty tooled_flags)"; state it IS stager-capable
via the unscoped `--mode accept-edits --dangerously-skip-permissions` + `--add-dir <repo>` combo, §12.5.1.1
item 4 RESOLVED 2026-07-09 agy v1.1.11; keep the experimental note — still pending a full --help re-verification
pass). Leave the qwen-code sentence as-is (still genuinely non-stager).

### Region C′ — "## Tooled mode and the stager role" layer-1 bullet (L~112-114)
Extend the existing claude(scoped)/pi(unscoped) contrast to ENUMERATE the full set: **pi, agy, codex, opencode,
and cursor** are unscoped (no git allowlist; a misbehaving unscoped stager CAN run arbitrary Bash); **claude**
is the ONLY structurally-scoped stager. Add the safety-net cross-refs the contract (c) wants: the §17.6 stager
prompt + the HEAD-movement guard + **verifyFreezeSubset (FR-M1c)** are the safety net for the unscoped
providers (not flag-scoping). (The existing layer-1 bullet already names §17.6 + HEAD-guard for pi; the edit
generalizes "pi" → "the unscoped providers" + adds verifyFreezeSubset + the full enumeration.)

### Region D — per-role table footer (L142)
Two fixes (the footer is doubly stale):
1. The "(cannot) = lacks tooled_flags" claim is FALSE for agy/opencode/codex/cursor (they HAVE tooled_flags).
   Reword: *(cannot)* reflects the **compiled-in `role_defaults.go` default (no authored stager model)**, NOT
   a capability gap — agy/opencode/codex/cursor ARE stager-capable (per the main table + builtin.go); only
   **qwen-code** genuinely lacks `tooled_flags`.
2. "currently pi or claude" → enumerate all **6** stager-capable providers (pi, claude, agy, opencode, codex,
   **cursor**). Add a one-line note that the per-role stager assignment is pending a `role_defaults.go`
   update (residual risk — `role_defaults_drift.md`; separate PRP).

**The per-role table CELLS (L131-137) are NOT edited** — they mirror `role_defaults.go` (stale code); inventing
stager model tokens is out of scope (contract (d) step 4: "Do NOT invent stager model tokens"). Handled via the
footer note only.

## §4 Out-of-scope (DO NOT TOUCH — flagged, not fixed)

- **`internal/config/role_defaults.go`** — STALE CODE (stager="" for agy/opencode/codex/cursor). This is a CODE
  change needing its own PRP (AGENTS.md hard rule 2). Surfaced in `role_defaults_drift.md`. The doc footer
  notes the drift; role_defaults.go is NOT edited.
- **`spec/02-providers.md §12.7` codex manifest** — stale (no tooled_flags, old bare_flags, positional
  delivery). Per AGENTS.md hard rule 1 + D4, spec/ is NEVER edited outside an interactive session. The codex
  docs/providers.md edit is grounded in builtin.go (the authoritative capability source); the spec §12.7 codex
  gap is flagged as a tracked item, not fixed.
- **Other docs** — README.md, docs/cli.md, docs/configuration.md, docs/how-it-works.md are NOT touched
  (how-it-works.md is the parallel task P1M1T1S6's file; it explicitly does NOT touch providers.md → no
  conflict). This PRP edits ONLY docs/providers.md.

## §5 Scope-fence + validation

- **Scope:** ONLY `docs/providers.md`. No code (builtin.go, role_defaults.go), no spec/, no other docs, no
  PRD/plan/tasks. Docs-only, no tests.
- **Validation:** (1) grep every `Stager?` cell against builtin.go's TooledFlags for ALL 7 providers (the
  contract's step-5 verification — now requires cursor=yes to PASS); (2) `git diff --stat` == docs/providers.md
  only; (3) markdownlint if available (NOT a CI gate — MD013/033/060 off); (4) manual accuracy read.
- **No `go build`/`go test`/`make`** — docs-only markdown edit, no Go code touched.