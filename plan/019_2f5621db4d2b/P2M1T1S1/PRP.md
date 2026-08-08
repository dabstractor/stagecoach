name: "P2.M1.T1.S1 — Fix the agy/codex/opencode (+cursor) stager rows + prose + stager-scope in docs/providers.md"
description: >
  A docs-only sync of `docs/providers.md` to the committed code (`internal/provider/builtin.go`).
  Corrects the main provider table's `Stager?` column, the post-table prose note, the stager-scope
  discussion in "## Tooled mode and the stager role", and the per-role table footer — all to match
  builtin.go's TooledFlags (non-nil ⇔ stager-capable). The contract names THREE stale providers
  (agy, codex, opencode); research found a FOURTH — **cursor** — whose TooledFlags (`["--trust","--yolo"]`)
  is COMMITTED at HEAD (`f88042c feat(provider): add cursor tooled flags`, verified 2026-07-09) but which
  BOTH the contract AND the (now-stale) arch doc `provider_docs_state.md` wrongly call "no". The builtin.go
  was edited Aug 8 00:26, AFTER the arch docs were authored Aug 7 22:40, so the arch-doc read predates the
  cursor commit. Per the task's own cardinal rule ("code is the source of truth for CAPABILITY") + verification
  step ("every Stager? cell must match builtin.go's TooledFlags for all seven") + the D1 precedent (the
  project resolved the identical opencode drift by following the code), the PRP follows the CODE: cursor is
  marked `✓ yes (unscoped)` alongside agy/codex/opencode. This is a deliberate, evidence-based DEVIATION from
  contract step (a) — flagged for human confirmation; trivially revertible (one table cell). Edits ONLY
  docs/providers.md: (A) table Stager? col — claude→(scoped), opencode/codex/agy/cursor→(unscoped), pi + qwen-
  code unchanged; (B) prose note — drop "agy cannot serve as a stager", state it IS stager-capable (§12.5.1.1
  item 4 RESOLVED); (C) "## Tooled mode and the stager role" layer-1 bullet — generalize pi→the five unscoped
  providers + claude the only scoped one + add verifyFreezeSubset/FR-M1c; (D) footer — fix the doubly-stale
  "(cannot)=lacks tooled_flags" + "currently pi or claude" claims (now 6 stager-capable) + note the
  role_defaults.go drift (residual risk). NO code (builtin.go/role_defaults.go), NO spec/ (D4 codex gap
  flagged), NO other docs. The parallel task P1M1T1S6 edits docs/how-it-works.md only → no conflict.

---

## ⚠️ DEVIATION FROM CONTRACT — READ FIRST (the cursor discrepancy)

The contract (RESEARCH NOTE + step a) states cursor's `TooledFlags` is nil and instructs: **"Leave cursor
... as `— no` (their TooledFlags ARE nil)."** **This is factually wrong about the current committed code.**

**Evidence (committed at HEAD, not working-tree):**
```
internal/provider/builtin.go:447  func builtinCursor()
internal/provider/builtin.go:469      TooledFlags: []string{
internal/provider/builtin.go:470          "--trust",
internal/provider/builtin.go:471          "--yolo",   // VERIFIED 2026-07-09 (cursor-agent) end-to-end
internal/provider/builtin.go:472      },
$ git log --oneline -1 -- internal/provider/builtin.go
f88042c feat(provider): add cursor tooled flags
```
cursor's `TooledFlags` is **NON-NIL** and **COMMITTED** (`f88042c`). builtin.go is NOT dirty. The arch doc
`provider_docs_state.md` ("verified 2026-08-08") was authored Aug 7 22:40 — ~2h BEFORE the Aug 8 00:26 cursor
commit — so its "cursor = nil, NO stager" read predates the commit and is stale (the verifier missed cursor
the same way D1 nearly missed opencode).

**Why the PRP follows the code (not the contract's cursor instruction):** the contract's OWN rules make the
code authoritative when they conflict with a per-provider instruction:
- RESEARCH NOTE: "**Code (`internal/provider/builtin.go`) is the source of truth for stager CAPABILITY** (non-nil TooledFlags ⇔ stager-capable)."
- step 5 / OUTPUT: "**confirm every Stager? cell matches builtin.go's TooledFlags ... for all seven providers**" / "whose provider table + prose **match the code**."
- D1 precedent (`findings_and_divergences.md`): the project resolved the IDENTICAL opencode contradiction by
  following the code — "the PRD's own verification step MANDATES this; step 3's opencode instruction is the
  part that's wrong."

Leaving cursor `— no` would ship a doc that STILL contradicts the code — the exact failure this task exists
to fix, and it would FAIL the task's own step-5 verification (which checks all seven cells against builtin.go).

**∴ The PRP marks cursor `✓ yes (unscoped)`.** The actual stager-capable set is **SIX** (pi, claude, agy,
opencode, codex, cursor); only **qwen-code** is genuinely non-stager. If the human disagrees (e.g., cursor is
considered not-ready-to-document), revert the single cursor table cell — the rest of the PRP is unaffected.

---

## Goal

**Feature Goal**: Make `docs/providers.md` agree with the committed code (`internal/provider/builtin.go`) on
which providers are stager-capable and their scope model, so a reader learns the TRUE stager set (6 capable,
1 not) and the scoped-vs-unscoped safety distinction — closing the stale-doc gap that agy/codex/opencode
(NOW shipped stagers) and cursor (shipped stager, missed by the arch doc) created.

**Deliverable**: Surgical edits to **ONE file**, `docs/providers.md`: (A) the main provider table `Stager?`
column (5 cells change); (B) the post-table prose note (drop the stale "agy cannot serve as a stager" clause);
(C) the "## Tooled mode and the stager role" layer-1 bullet (generalize pi→the five unscoped + add
verifyFreezeSubset); (D) the per-role table footer (fix two stale claims + note the role_defaults.go drift).

**Success Definition**:
- Every `Stager?` cell in the main table matches builtin.go's TooledFlags for all seven providers (the
  contract's step-5 verification — **requires cursor = yes to PASS**): pi `✓ yes`, claude `✓ yes (scoped)`,
  opencode/codex/cursor/agy `✓ yes (unscoped)`, qwen-code `— no ⚠️`.
- The prose note no longer claims agy "cannot serve as a stager"; it states agy IS stager-capable (§12.5.1.1
  item 4 RESOLVED) and keeps its experimental note + the qwen-code sentence.
- The stager section names all six stager-capable providers with their scope model (5 unscoped + claude the
  only scoped) and cross-refs §17.6 + HEAD-guard + verifyFreezeSubset (FR-M1c).
- The footer's two stale claims are fixed and the role_defaults.go drift is noted (no invented stager tokens).
- `git diff --stat` == `docs/providers.md` ONLY. No code, no spec/, no other docs touched.

## User Persona (if applicable)

**Target User**: A developer/operator reading `docs/providers.md` to pick a stager-capable provider for
multi-commit decomposition, or a contributor maintaining the provider manifests.
**Use Case**: "I want to use agy/codex/opencode/cursor as my stager — does docs/providers.md say it can stage?"
Today it says NO for all four (wrong); after this fix it says YES (unscoped) with the safety model explained.
**Pain Points Addressed**: the doc contradicts the shipped binary (agy/codex/opencode/cursor CAN stage but the
doc says they cannot), so users unnecessarily fall back to pi/claude for the stager role, or believe the
feature is unsupported. The fix makes the doc tell the truth + explains the unscoped safety net.

## Why

- **Close the stale-doc gap**: agy (`c34f480`), codex (`f40ac87`), opencode (`c9e5fb3`), and cursor (`f88042c`)
  all became stager-capable in committed code, but `docs/providers.md` still says "— no" for all four. This
  task is the docs sync the plan exists to perform (Phase 2).
- **The contract's verification step is the authority**: "confirm every Stager? cell matches builtin.go's
  TooledFlags (non-nil ⇔ stager-capable) for all seven." That step is impossible to satisfy with cursor left
  "— no" — so the code governs (§ DEVIATION above + D1 precedent).
- **Explain the unscoped safety model honestly**: five of six stager-capable providers are UNSCOPED (no git
  allowlist); the doc must say WHY that is safe (§17.6 prompt + HEAD-guard + verifyFreezeSubset), not paper
  over it. This is the §12.7.1 honesty the PRD mandates.

## What

Four surgical edits to `docs/providers.md` (table column + prose + stager-section bullet + footer). No code.
No `spec/` (the spec §12.7 codex gap is flagged per D4, not fixed — AGENTS.md hard rule 1). No invented stager
model tokens for the per-role table cells (they mirror stale `role_defaults.go` — residual risk, noted).

### Success Criteria
- [ ] **Region A** — main table `Stager?` column (L80-86): claude `✓ yes (scoped)`; opencode/codex/cursor/agy
      `✓ yes (unscoped)`; pi `✓ yes` (unchanged); qwen-code `— no ⚠️` (unchanged).
- [ ] **Region B** — prose note (L88): the agy sentence no longer says "cannot serve as a stager"; it states
      agy IS stager-capable via the unscoped combo (§12.5.1.1 item 4, 2026-07-09) + keeps the experimental note;
      the qwen-code sentence is unchanged.
- [ ] **Region C** — "## Tooled mode and the stager role" layer-1 bullet: names the five UNSCOPED providers
      (pi, agy, codex, opencode, cursor) + claude as the ONLY structurally-scoped stager + adds
      `verifyFreezeSubset (FR-M1c)` to the safety net.
- [ ] **Region D** — per-role footer (L142): the "(cannot)=lacks tooled_flags" claim corrected (it's a
      role_defaults.go default, not a capability gap for agy/opencode/codex/cursor; only qwen-code truly lacks
      tooled_flags); "currently pi or claude" → all six stager-capable providers; a one-line role_defaults.go
      drift note is added. Per-role CELLS (L131-137) unchanged.
- [ ] `git diff --stat` == `docs/providers.md` only.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact code truth (builtin.go TooledFlags for all 7, with the cursor deviation fully evidenced),
the exact current text of every edit region with line numbers, the exact oldText→newText for each edit (unique
anchors), the scope fence (role_defaults.go + spec/ + other docs OUT), the role_defaults_drift residual, and
the grep-based validation that re-verifies every cell against builtin.go.

### Documentation & References

```yaml
# MUST READ — the codebase findings (the cursor deviation + the exact edit plan + line numbers).
- docfile: plan/019_2f5621db4d2b/P2M1T1S1/research/findings.md
  why: "§0 the cursor discrepancy (COMMITTED f88042c; arch doc stale) — the headline; §1 the authoritative
        capability matrix (builtin.go TooledFlags for all 7); §2 the exact regions + line numbers +
        the contract-(c) location interpretation; §3 the region-by-region edit plan; §4 out-of-scope
        (role_defaults.go + spec §12.7 codex); §5 scope + validation."

# MUST READ — the file under edit.
- file: docs/providers.md
  why: "THE file. Region A = main table L78-86 (Stager? is the last column). Region B = prose note L88.
        Region C = '## Tooled mode and the stager role' L102-118 (layer-1 bullet ~L112-114 already contrasts
        claude(scoped)/pi(unscoped) — EXTEND it). Region D = per-role footer L142."
  pattern: "Match the existing voice: the table uses ✓/— glyphs + ⚠️; the stager section uses **bold** +
            `code spans` + em-dashes + §xx cross-refs. MD013 (line-length) is OFF (.markdownlint.json) → the
            existing long table rows / prose lines are fine; do NOT hard-wrap."

# MUST READ — the authoritative capability source (code is source of truth per the contract).
- file: internal/provider/builtin.go
  why: "THE truth for Stager? = TooledFlags non-nil. pi L97; claude L149 (Bash(git ...),Read,Edit = STRUCTURAL
        scope); agy L244 (--mode accept-edits --dangerously-skip-permissions, TooledRepoDirFlag --add-dir L248);
        qwen-code L303 (NIL — comment); opencode L353 (--agent build); codex L417 (--sandbox danger-full-access);
        cursor L469 (--trust --yolo). Re-read this to satisfy the step-5 verification."
  critical: "cursor L469 is NON-NIL and COMMITTED (f88042c) — the contract's 'cursor = no' is wrong (§ DEVIATION).
             claude is the ONLY structurally-scoped stager; the other five are UNSCOPED (per the builtin.go
             comments + spec §12.7.1)."

# MUST READ — the D1 precedent (the principle that governs the cursor deviation).
- docfile: plan/019_2f5621db4d2b/architecture/findings_and_divergences.md
  why: "D1 resolved the IDENTICAL opencode contradiction by following the code over the PRD's per-provider
        instruction ('the PRD's own verification step MANDATES this; step 3's opencode instruction is the part
        that's wrong'). The same logic governs cursor. D4 = spec §12.7 codex manifest is stale (no tooled_flags)
        → DO NOT edit spec/; ground the codex doc edit in the code."

# MUST READ — the capability-state read (NOTE it is STALE on cursor; cross-check builtin.go).
- docfile: plan/019_2f5621db4d2b/architecture/provider_docs_state.md
  why: "Documents the stale docs/providers.md regions A-D and the correct cells per builtin.go — BUT its
        cursor row says 'NIL (absent), NO stager', which is WRONG (predates f88042c). Use its opencode/codex/
        agy/pi/claude/qwen-code rows; OVERRIDE its cursor row with the live builtin.go (§ DEVIATION)."

# MUST READ — the role_defaults.go drift (the residual risk the footer must note).
- docfile: plan/019_2f5621db4d2b/architecture/role_defaults_drift.md
  why: "Explains WHY the per-role table *(cannot)* cells (agy/opencode/codex/cursor) are stale (they mirror
        role_defaults.go's stager=\"\", which is itself stale code needing its own PRP). The footer must note
        this drift rather than fabricate stager model tokens. NOTE: this doc ALSO predates the cursor commit
        (lists cursor as 'no (correct)') — its cursor row is wrong; cursor joins agy/opencode/codex in the drift."

# CONTEXT — the spec (READ-ONLY; agy §12.5.1 correct, codex §12.7 stale per D4).
- file: spec/02-providers.md
  why: "§12.5.1 (agy) correctly reflects stager-capable (use it to word the agy prose). §12.7 (codex) is STALE
        (no tooled_flags) — DO NOT edit spec/ (AGENTS.md hard rule 1); ground the codex doc edit in builtin.go.
        READ-ONLY: never write to spec/ in this task."
  gotcha: "AGENTS.md hard rule 1: NEVER modify spec/ outside an interactive session. The spec §12.7 codex gap
           is a tracked item, not fixed here."

# CONTEXT — the parallel task (no conflict).
- file: plan/019_2f5621db4d2b/P1M1T1S6/PRP.md
  why: "The parallel item edits docs/how-it-works.md ONLY and explicitly does NOT touch providers.md ('NO
        providers.md change (that is Phase 2 / P2.M1.T1.S1)'). Confirms NO scope conflict — this PRP owns
        docs/providers.md exclusively."

# CONTEXT — markdownlint config (NOT a CI gate).
- file: .markdownlint.json
  why: "{ default:true, MD013:false, MD033:false, MD060:false } — line-length / inline-HTML / heading-punct
        are OFF. The existing table rows + prose are long single lines; do NOT hard-wrap. markdownlint is NOT
        run in CI (it is a local nicety)."
```

### Current Codebase tree (relevant slice)

```bash
# EDIT:
docs/providers.md          # ← the ONLY file edited (regions A/B/C/D)
# READ-ONLY (do NOT edit):
internal/provider/builtin.go          # the capability source of truth (TooledFlags)
internal/config/role_defaults.go      # STALE CODE — residual risk (footer notes it; NOT edited)
spec/02-providers.md                  # human-owned (AGENTS.md rule 1); §12.7 codex stale (D4) — flagged, not fixed
docs/how-it-works.md                  # parallel task P1M1T1S6's file — NOT touched
README.md, docs/cli.md, docs/configuration.md   # NOT touched
plan/019_2f5621db4d2b/architecture/*.md          # READ-ONLY (provider_docs_state.md is stale on cursor)
.markdownlint.json                    # READ-ONLY
```

### Desired Codebase tree with files to be modified

```bash
docs/providers.md   # MODIFY — regions A (table), B (prose), C (stager bullet), D (footer). 4 surgical edits.
# NOTHING ELSE. No code, no spec/, no other docs.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (the cursor deviation — see § DEVIATION): cursor's TooledFlags is NON-NIL and COMMITTED
     (f88042c). The contract + provider_docs_state.md both wrongly say cursor is "no". Mark cursor
     `✓ yes (unscoped)` to match builtin.go; the step-5 verification (all seven cells vs builtin.go)
     is UNSATISFIABLE otherwise. Revertible: one table cell. -->

<!-- CRITICAL (claude is the ONLY structurally-scoped stager): claude's TooledFlags is an --allowed-tools
     git allowlist; pi/agy/codex/opencode/cursor are UNSCOPED (no allowlist). The (scoped)/(unscoped)
     annotation in the table + the stager-section bullet must reflect this exactly. Do NOT call any
     unscoped provider "scoped" or vice versa. -->

<!-- CRITICAL (DO NOT invent stager model tokens for the per-role table cells): agy/opencode/codex/cursor
     *(cannot)* cells (L133-136) mirror the STALE role_defaults.go (stager=""). They are NOT edited in
     this task — inventing tokens is out of scope (role_defaults.go is code, needs its own PRP, AGENTS.md
     rule 2). The FOOTER notes the drift; the CELLS stay. -->

<!-- CRITICAL (spec/ is READ-ONLY): the §12.7 codex manifest is stale (no tooled_flags) per D4, but
     AGENTS.md hard rule 1 forbids editing spec/ outside an interactive session. Ground the codex doc
     edit in builtin.go; flag the spec gap, do not fix it. -->

<!-- GOTCHA (contract's region-C location "L90-100" is the bare-mode section): the "## Tools-disable
     asymmetry" heading (L90) is a bare-mode-only section in docs/providers.md; the stager scope model
     lives in the adjacent "## Tooled mode and the stager role" (L102-118), whose layer-1 bullet already
     contrasts claude(scoped)/pi(unscoped). Place the contract-(c) stager-scope content THERE (extend
     that bullet), NOT in the bare-mode section (thematically wrong + redundant). Flagged as a minor
     location interpretation. -->

<!-- GOTCHA (MD013 line-length OFF): the table rows + prose are long single lines. Do NOT hard-wrap the
     edits to 80 cols — match the surrounding long-line style. -->

<!-- GOTCHA (edit anchors must be unique): codex/cursor/agy rows share the trailing chrome cell
     "no per-surface switch; read-only constraint only — documented limitation | — no |". Anchor each
     row edit on its UNIQUE Tool-disable-approach cell (opencode `run` subcommand; codex --sandbox
     read-only --ephemeral; cursor --mode ask --trust; agy --mode plan) so oldText is unique. -->
```

## Implementation Blueprint

### Data models and structure
None (documentation only). No code, config, schema, or test change.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: REGION A — edit the main provider table Stager? column (L80-86), 5 cells
  Do these as ONE edit call with 5 disjoint edits[] entries (each oldText is unique via its approach cell):
  - claude (L81) — annotate scoped:
      oldText:  '--setting-sources ""` | ✓ yes |'
      newText:  '--setting-sources ""` | ✓ yes (scoped) |'
  - opencode (L82) — flip + unscoped:
      oldText:  '(`run` subcommand) | no per-surface switch; read-only by design — documented limitation | — no |'
      newText:  '(`run` subcommand) | no per-surface switch; read-only by design — documented limitation | ✓ yes (unscoped) |'
  - codex (L83) — flip + unscoped:
      oldText:  '(`--sandbox read-only --ephemeral`) | no per-surface switch; read-only constraint only — documented limitation | — no |'
      newText:  '(`--sandbox read-only --ephemeral`) | no per-surface switch; read-only constraint only — documented limitation | ✓ yes (unscoped) |'
  - cursor (L84) — flip + unscoped  [§ DEVIATION — cursor TooledFlags is COMMITTED non-nil]:
      oldText:  '(`--mode ask --trust`) | no per-surface switch; read-only constraint only — documented limitation | — no |'
      newText:  '(`--mode ask --trust`) | no per-surface switch; read-only constraint only — documented limitation | ✓ yes (unscoped) |'
  - agy (L85) — flip + unscoped:
      oldText:  '(`--mode plan`) | no per-surface switch; read-only constraint only — documented limitation | — no |'
      newText:  '(`--mode plan`) | no per-surface switch; read-only constraint only — documented limitation | ✓ yes (unscoped) |'
  - LEAVE UNCHANGED: pi (L80, `✓ yes`) and qwen-code (L86, `— no ⚠️`).
  - VERIFY: every Stager? cell now matches builtin.go (Task 5 gate).

Task 2: REGION B — rewrite the agy clause in the prose note (L88)
  - oldText (agy clause, unique):
      '`agy` is **experimental** (PRD §12.5.1) pending the remaining §12.5.1.1 checklist items (the non-TTY stdout drop, issue #76, no longer reproduces as of **2026-07-08**) and cannot serve as a stager (empty `tooled_flags`).'
  - newText:
      '`agy` is **experimental** (PRD §12.5.1) pending a full `--help` re-verification pass, and is **stager-capable** via the unscoped `--mode accept-edits --dangerously-skip-permissions` combo (§12.5.1.1 item 4, verified 2026-07-09, agy v1.1.11; the same unscoped model pi uses).'
  - LEAVE the qwen-code clause unchanged (still genuinely non-stager — its TooledFlags IS nil).
  - NOTE: the cursor mention earlier in L88 ("cursor is the only provider where detect and command differ from
    name — the binary is agent, not cursor") is NOT a stager claim — leave it.

Task 3: REGION C — extend the "## Tooled mode and the stager role" layer-1 bullet (L~112-114)
  - oldText (the pi-specific portion of layer 1, unique):
      'pi is **not** flag-scoped — its tooled profile enables tools with chrome stripped (no git-scoped allowlist), so a misbehaving pi stager CAN run arbitrary Bash. pi's safety is therefore **instructional** (the §17.6 stager task prompt) + a **best-effort HEAD-movement guard** (HEAD is snapshotted before each stager call; the run aborts if HEAD moved), not structural.'
  - newText:
      'pi, agy, codex, opencode, and cursor are **not** flag-scoped — their tooled profiles enable tools with no git allowlist (the **UNSCOPED** model, §12.7.1), so a misbehaving unscoped stager CAN run arbitrary Bash. Their safety is therefore **instructional** (the §17.6 stager task prompt) + a **best-effort HEAD-movement guard** (HEAD is snapshotted before each stager call; the run aborts if HEAD moved) + **`verifyFreezeSubset` (FR-M1c)** (every staged tree is verified a content-subset of the frozen `T_start`), not structural. **claude is the ONLY structurally-scoped stager**; the other five stager-capable providers are unscoped.'
  - This realizes contract (c): names the full unscoped set + claude scoped + cross-refs §12.7.1 + §17.6 +
    HEAD-guard + verifyFreezeSubset. (Placed here — not in the bare-mode "## Tools-disable asymmetry" section —
    because this bullet ALREADY has the scoped/unscoped contrast; see Known Gotchas.)

Task 4: REGION D — rewrite the per-role table footer (L142)
  - oldText (the whole footer sentence, unique):
      '**Stager column:** A value of *(cannot)* means the provider lacks `tooled_flags` in its manifest and cannot serve as the stager. When the detected provider cannot be the stager, the bootstrap falls back to the next stager-capable provider (FR-D4 fallback — currently pi or claude).'
  - newText:
      '**Stager column:** A value of *(cannot)* reflects the **compiled-in `role_defaults.go` default** (no authored stager model), NOT a capability gap for every such provider — agy, opencode, codex, and cursor ARE stager-capable (per the main table + `builtin.go`); only **qwen-code** genuinely lacks `tooled_flags`. The per-role stager assignment for those four is pending a `role_defaults.go` update (a separate code change — tracked residual risk). When the detected provider cannot be the stager, the bootstrap falls back to the next stager-capable provider (FR-D4 fallback — currently pi, claude, agy, opencode, codex, or cursor).'
  - This fixes BOTH stale clauses + notes the role_defaults.go drift + enumerates the 6 stager-capable.
  - DO NOT edit the per-role table CELLS (L131-137) — inventing stager model tokens is out of scope.

Task 5: VERIFY (see Validation Loop) — every Stager? cell vs builtin.go for all 7 + scope guard.
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN (unique edit anchors): each table-row edit anchors on the UNIQUE Tool-disable-approach cell so
     oldText is unambiguous (codex/cursor/agy share a chrome-cell suffix; the approach cell disambiguates). -->

<!-- PATTERN (the scope annotation): (scoped) = claude ONLY (--allowed-tools git allowlist); (unscoped) =
     pi/agy/codex/opencode/cursor (no allowlist; safety = §17.6 prompt + HEAD-guard + verifyFreezeSubset).
     Every ✓ cell carries one of these EXCEPT pi (left bare per contract step a). -->

<!-- PATTERN (truth sourcing): every capability claim traces to builtin.go's TooledFlags, NOT to
     provider_docs_state.md (which is stale on cursor) or spec/ (§12.7 codex stale per D4). Re-read
     builtin.go to satisfy the step-5 verification. -->
```

### Integration Points

```yaml
NO code / config / schema / CLI / test integration. ONE markdown file edited (docs/providers.md).

CONSUMES (READ-ONLY):
  - internal/provider/builtin.go — the capability source of truth (TooledFlags non-nil ⇔ stager-capable).
  - spec/02-providers.md §12.5.1 (agy) — correct; used to word the agy prose. §12.7 (codex) stale (D4) — NOT used.
  - plan/019_2f5621db4d2b/architecture/*.md — findings; NOTE provider_docs_state.md is stale on cursor.

FLAGGED (not fixed — out of scope):
  - internal/config/role_defaults.go — STALE CODE (stager="" for agy/opencode/codex/cursor); footer notes the
    drift; the code is NOT edited (needs its own PRP). See role_defaults_drift.md.
  - spec/02-providers.md §12.7 codex manifest — stale (no tooled_flags); flagged per D4, NOT edited
    (AGENTS.md hard rule 1).

NO conflict with the parallel task P1M1T1S6 (it edits docs/how-it-works.md only).
```

## Validation Loop

> **Docs-only markdown edit.** No `go build`/`go test`/`make` — no Go code is touched. The gates are
> grep-based (re-verify every Stager? cell against builtin.go) + scope guard + optional markdownlint.

### Level 1: Capability verification — every Stager? cell vs builtin.go (the contract's step 5)

```bash
# Re-read builtin.go's TooledFlags state for all 7 providers, then assert docs/providers.md matches.
# EXPECTED after the edits: 6 stager-capable (pi, claude, agy, opencode, codex, cursor), 1 not (qwen-code).
for p in pi claude opencode codex cursor agy qwen-code; do
  case $p in
    qwen-code) want="— no" ;;
    *) want="✓ yes" ;;
  esac
  cell=$(grep -oE "\| \`$p\` .* \| (✓ yes[^|]*|— no[^|]*)" docs/providers.md | grep -oE '(✓ yes[^|]*|— no[^|]*)$')
  echo "$p: cell=[$cell] want~[$want]"
done
# Expected: pi [✓ yes]; claude [✓ yes (scoped)]; opencode/codex/cursor/agy [✓ yes (unscoped)]; qwen-code [— no ⚠️].
# Cross-check against builtin.go: every provider EXCEPT qwen-code has a non-nil TooledFlags:
grep -c 'TooledFlags: \[\]string{' internal/provider/builtin.go   # Expected: 6 (pi,claude,agy,opencode,codex,cursor)
# (qwen-code is the ONLY one whose TooledFlags is nil — confirmed by its L303 comment.)

# Direct cell assertions (deterministic):
grep -c '| ✓ yes (unscoped) |' docs/providers.md                  # Expected: 4 (opencode, codex, cursor, agy)
grep -c '| ✓ yes (scoped) |'    docs/providers.md                  # Expected: 1 (claude)
grep    '| `qwen-code` .*| — no ⚠️ |' docs/providers.md >/dev/null && echo "qwen-code OK"   # Expected: OK
grep    '| `cursor` .*| ✓ yes (unscoped) |' docs/providers.md >/dev/null && echo "cursor=unscoped OK (the deviation)"
```

### Level 2: Prose + stager-section + footer correctness

```bash
# The agy "cannot serve as a stager" clause is GONE (Region B):
grep -c 'cannot serve as a stager' docs/providers.md               # Expected: 1 (ONLY the qwen-code clause remains)
grep -c 'agy.*stager-capable\|stager-capable.*agy' docs/providers.md   # Expected: ≥1 (the new agy clause)

# The stager section names all 5 unscoped + claude scoped + verifyFreezeSubset (Region C):
grep -c 'pi, agy, codex, opencode, and cursor' docs/providers.md  # Expected: 1
grep -c 'ONLY structurally-scoped stager' docs/providers.md       # Expected: 1
grep -c 'verifyFreezeSubset' docs/providers.md                    # Expected: ≥1 (the new addition)

# The footer is fixed (Region D): no "currently pi or claude"; no bare "(cannot)=lacks tooled_flags":
grep -c 'currently pi or claude' docs/providers.md                # Expected: 0 (removed)
grep -c 'currently pi, claude, agy, opencode, codex' docs/providers.md  # Expected: 1
grep -c 'role_defaults.go' docs/providers.md                      # Expected: ≥1 (the drift note)
```

### Level 3: Markdown style (optional, NOT a CI gate)

```bash
# .markdownlint.json disables MD013/MD033/MD060. markdownlint is NOT in CI. Run only if a CLI is available.
npx --no-install markdownlint-cli2 docs/providers.md 2>/dev/null \
  || npx markdownlint-cli docs/providers.md 2>/dev/null \
  || echo "markdownlint not installed — skipping (not a CI gate)"
# Expected: no NEW violations vs baseline (do not fix pre-existing issues — out of scope).
```

### Level 4: Scope guard

```bash
# ONLY docs/providers.md changed.
git diff --stat
# Expected: exactly one line — " docs/providers.md | <n> +-…" — nothing else.

# No forbidden files touched.
git diff --name-only | grep -vE '^docs/providers\.md$' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
git diff --name-only | grep -E 'builtin\.go|role_defaults\.go|^spec/|how-it-works\.md|README\.md|cli\.md|configuration\.md|PRD\.md|plan/|tasks\.json' && echo "FAIL: forbidden file" || echo "OK: no forbidden files"
# Expected: "OK: scope clean" + "OK: no forbidden files" (builtin.go, role_defaults.go, spec/, how-it-works.md all untouched).
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1: every Stager? cell matches builtin.go TooledFlags for all 7 (6 yes incl. cursor; qwen-code no)
- [ ] Level 1: `grep -c 'TooledFlags: \[\]string{' builtin.go` == 6 (proves cursor is in the code set)
- [ ] Level 2: prose (agy stager-capable, "cannot serve as a stager" gone for agy); stager section (5 unscoped
      + claude scoped + verifyFreezeSubset); footer (no "currently pi or claude"; role_defaults.go noted)
- [ ] Level 4: `git diff --stat` == docs/providers.md only; no code/spec/other-docs touched

### Feature Validation
- [ ] A reader of docs/providers.md learns the TRUE stager set (6 capable: pi, claude, agy, opencode, codex,
      cursor; 1 not: qwen-code) and the scoped (claude only) vs unscoped (the other five) safety model
- [ ] The doc no longer contradicts the shipped binary on any provider's stager capability
- [ ] The role_defaults.go drift is noted (the per-role table stays consistent-with-stale-code by design,
      with the footer explaining why) rather than papered over with invented tokens

### Scope-Boundary Validation
- [ ] `git status --porcelain` shows ONLY `docs/providers.md`
- [ ] `internal/provider/builtin.go` UNCHANGED (the capability source — read-only here)
- [ ] `internal/config/role_defaults.go` UNCHANGED (stale code — residual risk, footer notes it, NOT edited)
- [ ] `spec/02-providers.md` UNCHANGED (human-owned; §12.7 codex gap flagged per D4, not fixed)
- [ ] `docs/how-it-works.md` UNCHANGED (parallel task P1M1T1S6's file)

---

## Anti-Patterns to Avoid

- ❌ Don't leave cursor as `— no` to literally follow contract step (a). The committed code (f88042c) makes
  cursor stager-capable; the task's cardinal rule + step-5 verification + the D1 precedent all mandate
  following the code. Leaving it stale ships a doc that contradicts the binary and FAILS the verification.
  (See § DEVIATION — revertible: one cell.)
- ❌ Don't invent stager model tokens for the per-role table `*(cannot)*` cells (agy/opencode/codex/cursor).
  They mirror the stale `role_defaults.go`; inventing tokens is a code-aligned fabrication out of scope. The
  FOOTER notes the drift; the CELLS stay.
- ❌ Don't edit `spec/02-providers.md` (even its stale §12.7 codex manifest). AGENTS.md hard rule 1 forbids
  editing spec/ outside an interactive session. Ground the codex doc edit in builtin.go; flag the spec gap.
- ❌ Don't edit `internal/config/role_defaults.go` or `internal/provider/builtin.go`. Both are code; this task
  is docs-only (AGENTS.md hard rule 2 — code edits need a PRP). role_defaults.go's drift is a flagged residual.
- ❌ Don't place the stager-scope sentence in the "## Tools-disable asymmetry" section (L90-100). That section
  is bare-mode-only in the docs; the stager scope model lives in "## Tooled mode and the stager role" (L102-118),
  whose layer-1 bullet already has the scoped/unscoped contrast. Extend THAT bullet (the faithful content
  realization of contract (c); the line-number was a planning approximation).
- ❌ Don't call any unscoped provider "scoped" or claude "unscoped". claude is the ONLY structurally-scoped
  stager (the --allowed-tools git allowlist); the other five are UNSCOPED.
- ❌ Don't hard-wrap the edited lines to 80 columns. MD013 is OFF and the existing table rows / prose are long
  single lines; match that style.
- ❌ Don't touch docs/how-it-works.md, README.md, docs/cli.md, or docs/configuration.md. The parallel task owns
  how-it-works.md; the others are out of scope. This PRP edits ONLY docs/providers.md.

---

## Confidence Score: 9/10

The edit plan is fully specified (exact oldText→newText anchors for 4 regions, all unique), the capability
truth is verified against the COMMITTED code (with the cursor deviation fully evidenced by git history), and
the gates are deterministic (every Stager? cell vs builtin.go). The 1-point deduction is for the cursor
deviation itself: it overrides an explicit contract instruction, so there is a small chance the human
intended cursor left undocumented (e.g., not-ready-to-document) — but the evidence (committed, verified,
dated feature + the cardinal rule + D1 precedent) strongly supports following the code, and the deviation is
a single revertible table cell. If the human overrides, only Region A's cursor row reverts; the rest stands.