# P1.M4.T1.S1 — README.md changeset-level review: findings

## §0 — THE HEADLINE: the README needs NO changes (the fixes make the implementation match the existing docs)

The contract itself states the working hypothesis: *"The changeset does NOT add new features — it fixes
bugs so the implementation matches existing documentation. Most README references should already be
accurate after the fixes (they describe correct behavior that was previously not implemented correctly)."*

After a line-by-line review of README.md against all 6 issues, **this hypothesis is CONFIRMED**. Every
README reference in the 4 review areas either (a) already describes the now-fixed correct behavior, or
(b) doesn't mention the detail at all. There are ZERO stale references to correct. The bugfixes make the
README MORE accurate, not less.

The default deliverable is therefore: **README.md unchanged + the review documented in the commit/PR
message** (per the contract: "If no changes needed, note that in the commit message"). The ONE judgment
call (area (a), the Issue 4 floor note) is documented in §5 with a recommendation to route it to
docs/configuration.md (S2's territory), NOT the high-level README.

## §1 — Area-by-area review (exact line numbers + finding)

### (a) token_limit / closed-loop guarantee — Issue 4 (floor rejection)
**README line 66** (Features table, "Payload optimization" row):
> "...optionally capped to your model's context window via `token_limit` — a closed-loop guarantee that
> the assembled prompt never exceeds the limit..."

- **Before the fix**: sub-floor limits (below ~270 tokens) silently violated the invariant — the claim was
  FALSE for those values.
- **After the fix** (P1.M3.T1.S1, `git.IrreducibleFloor` + caller rejection in git.go): a sub-floor
  `token_limit` ERRORS at StagedDiff/TreeDiff/WorkingTreeDiff (`"token_limit %d is below the irreducible
  prompt floor %d..."`) — the run aborts BEFORE the prompt is assembled/sent.
- **Finding**: the claim "never exceeds the limit" is now TRUE in ALL cases:
  - `token_limit >= floor` → the closed-loop gate trims to fit (never exceeds). TRUE.
  - `token_limit < floor` → the run errors before assembly (the prompt is never sent). Vacuously TRUE.
- **STALE REFERENCE? NO.** The fix makes the README accurate. No edit required.
- **The floor-rejection DETAIL** (sub-floor limits error) is a configuration-level concern → belongs in
  **docs/configuration.md line 167** (per test_patterns.md "Mode A impact"), which is **S2's territory**,
  NOT the high-level README. See §5 for the decision.

### (b) Config bootstrap / pi models — Issues 1 & 2 (bare pi models)
**README lines 232-246** ("Configure your agent" section):
- L232: "single-backend providers use a bare model" — still correct (claude/agy/codex/etc. take bare models).
- L235-237: pi multi-backend prefix example `zai/glm-5.2` — correct.
- L241-243: *"A bare model (no `/`) on pi is a config error (FR-R5b)."* — correct. The fixes (P1.M1/P1.M2)
  ENFORCE this at bootstrap; before, the bootstrap VIOLATED it (Issue 1 stager-fallback wrote bare
  `gpt-5.4-mini`; Issue 2 commented-block wrote bare `gpt-5.4`/`gpt-5.4-mini`/`gpt-5.4-nano`).
- L246: *"for **pi**, the default, per-role models are left empty so you can supply your own
  inference-backend/model prefix"* — this is EXACTLY the fixed behavior. Before the fix this was FALSE
  (stager-fallback + commented block emitted bare models). After: TRUE.
- **STALE REFERENCE? NO.** The README already describes the correct (now-fixed) behavior. The fixes align
  the implementation with the existing doc. No edit required.

### (c) config upgrade / backup — Issue 3 (FR-B8 backup)
**README lines 257, 263, 293**:
- L257: `stagecoach config upgrade` example — accurate (it upgrades).
- L263: notes `config upgrade` honors `--config` — accurate.
- L293: "upgrades an existing config to the current schema version" — accurate.
- **No mention of backup anywhere in the README.** Issue 3 (P1.M2.T2.S1) ADDS a timestamped backup
  (`WriteTimestampedBackup` before `os.WriteFile` in `runConfigUpgrade`). There is no stale claim to fix.
- Adding "(creates a timestamped backup)" to the one-line example would be a **feature blurb for a bugfix**,
  which the contract explicitly forbids ("Do NOT add feature blurbs for bugfixes — only correct stale
  references"). The backup detail belongs in **docs/cli.md** (S2's territory).
- **STALE REFERENCE? NO.** No edit required.

### (d) Auto-stage notice grammar — Issue 6 ("(1 files)" → "(1 file)")
- `grep -nE "Nothing staged|staging all changes|auto.stage|files\)" README.md` → **ZERO matches** (exit 1).
- **The README does NOT show the auto-stage notice output anywhere.** There is no "(1 files)" string to
  correct. The notice is internal/cmd UI text; the README's Quick-start examples show staged-input usage
  (`git add ... ; stagecoach`), not the nothing-staged auto-stage path.
- **STALE REFERENCE? NO.** No edit required.

### (e) [bonus] --edit doubled prefix — Issue 5
- `grep -nE "stagecoach:|empty commit|abort|--edit" README.md` → L136 lists `stagecoach --edit` as a flag
  with NO output shown. No "stagecoach:" doubled-prefix output, no "empty commit message" text.
- **STALE REFERENCE? NO.** No edit required.

## §2 — The fixed behavior per issue (the source of truth the README is measured against)

| Issue | Fix subtask | Fixed behavior (now matches README) |
|---|---|---|
| 1 (Critical) | P1.M1.T1.S1 | bootstrap stager-fallback: when stager routes to pi and target != pi, the stager model is BLANKED + a multi-backend guidance comment is emitted (was bare `gpt-5.4-mini`) |
| 1 (net) | P1.M1.T2.S1 | post-bootstrap ValidateModel regression net over all (target, installed) combos |
| 2 (Major) | P1.M2.T1.S1 | commented-out pi block: models blanked + guidance comment (was bare `gpt-5.4`/etc.) |
| 3 (Major) | P1.M2.T2.S1 | `runConfigUpgrade` calls `WriteTimestampedBackup` before `os.WriteFile` (was no backup) |
| 4 (Minor) | P1.M3.T1.S1 | `git.IrreducibleFloor` exported; StagedDiff/TreeDiff/WorkingTreeDiff reject `token_limit < floor` with a named error (was silent invariant violation) |
| 5 (Minor) | P1.M3.T2.S1 | `ErrEmptyMessage` literal drops the `"stagecoach: "` prefix (main.go already prepends it) (was doubled) |
| 6 (Minor) | P1.M3.T3.S1 | auto-stage notice `noun := "files"; if n == 1 { noun = "file" }` (was always "files") |

## §3 — Scope fence: README.md ONLY

- **This task (S1)** touches `README.md` ONLY.
- **P1.M4.T1.S2** (sibling) owns `docs/how-it-works.md` + `docs/cli.md` (and, per test_patterns.md "Mode A
  impact" notes, the floor detail in `docs/configuration.md:167` and the commented-block note in
  `docs/providers.md`). **Do NOT edit any `docs/*` file here — that's S2's territory.**
- The implementing subtasks (P1.M1–P1.M3) own the source + tests. This task CONSUMES their fixes (reviews
  the README against them); it does NOT modify source/tests.

## §4 — Validation approach for a docs-review task

- The primary "test" is a **manual read + grep verification** that each README claim is consistent with the
  fixed behavior (§1). No `go test` is affected by a README edit, but `make test`/`make build`/`make lint`
  are sanity gates (confirm no collateral — there can be none from a markdown edit, but run them).
- If Outcome B (§5) is chosen, validate the markdown renders (no broken table/code-fence) and the new
  sentence is accurate against the §2 fixed behavior.
- Scope guard: `git status --porcelain` shows EITHER nothing (Outcome A — review documented in commit msg)
  OR `README.md` ONLY (Outcome B). NEVER `docs/*` or any `.go` file.

## §5 — THE ONE JUDGMENT CALL: the Issue 4 floor note in README line 66

The contract leaves area (a) open: *"if the floor rejection behavior (Issue 4) warrants a note, add it."*

**Recommendation: do NOT add it to the README. Route it to docs/configuration.md (S2).** Rationale:
1. The README claim ("never exceeds the limit") is already accurate after the fix (§1a). There is no
   inaccuracy to correct.
2. The floor-rejection DETAIL (sub-floor limits error) is a configuration-level concern. test_patterns.md
   explicitly assigns it to **docs/configuration.md line 167** ("Mode A impact: Issue 4 fix (floor
   rejection) should update line 167 to note the floor") — that is **S2's scope**, not the README's.
3. The README is high-level (Features table, one row per capability). A floor-rejection parenthetical is
   implementation detail that doesn't belong in a marketing/overview row.
4. The contract forbids "feature blurbs for bugfixes." A floor-rejection note edges toward that.

**If the decision is to add it anyway (Outcome B)**, the minimal accurate edit to README line 66 is:
- CURRENT: `...via \`token_limit\` — a closed-loop guarantee that the assembled prompt never exceeds the limit...`
- PROPOSED: `...via \`token_limit\` — a closed-loop guarantee that the assembled prompt never exceeds the
  limit (a limit below the irreducible prompt floor is rejected with an error rather than silently broken;
  see [docs](docs/configuration.md#built-in-defaults))...`

This is the ONLY candidate edit. The default (Outcome A) is to leave it unchanged.

## §6 — Default deliverable

**Outcome A (default, recommended)**: README.md is unchanged. The review (§1 + §2) is documented in the
changeset's commit message / PR description: "Reviewed README.md against the 6-issue bugfix changeset
(Issues 1-6). All README references in the 4 review areas (token_limit/closed-loop, bootstrap/pi models,
config upgrade, auto-stage notice) already describe the correct now-fixed behavior; no stale references
found. The Issue 4 floor-rejection detail is routed to docs/configuration.md (P1.M4.T1.S2). No README
changes required."

**Outcome B (optional, only if the §5 decision goes the other way)**: the single line-66 edit in §5.
