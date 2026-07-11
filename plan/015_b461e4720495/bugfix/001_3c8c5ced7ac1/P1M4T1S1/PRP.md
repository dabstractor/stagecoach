name: "P1.M4.T1.S1 — Review README.md for changeset-level impacts (Issues 1-6 bugfix); expected outcome: no changes (docs align with the fixed impl)"
description: >
  A DOCUMENTATION-REVIEW task (Mode B). The 6-issue bugfix changeset (P1.M1-P1.M3: Issue 1 bootstrap
  stager-fallback bare pi model; Issue 2 commented-out pi block bare models; Issue 3 config upgrade
  backup; Issue 4 token-gate sub-floor invariant; Issue 5 doubled stagecoach: prefix; Issue 6 auto-stage
  grammar) is COMPLETE. This task reviews README.md against those fixes to correct any STALE references
  — it does NOT add feature blurbs. The contract's working hypothesis ("the changeset fixes bugs so the
  implementation matches existing documentation; most README references should already be accurate after
  the fixes") is CONFIRMED by a line-by-line review: ZERO stale references exist. Every README reference
  in the 4 review areas either (a) already describes the now-fixed correct behavior (line 246 "pi per-role
  models are left empty" = EXACTLY the Issue 1/2 fixed behavior; line 66 "closed-loop guarantee ... never
  exceeds the limit" = now TRUE after Issue 4 rejects sub-floor limits before assembly), or (b) doesn't
  mention the detail at all (the auto-stage notice from Issue 6 appears NOWHERE in the README; config
  upgrade backup from Issue 3 is not mentioned). The DEFAULT deliverable is therefore: README.md UNCHANGED
  + the review documented in the commit/PR message. The ONE judgment call is whether the Issue 4
  floor-rejection detail warrants a parenthetical on README line 66 — recommendation: NO (route it to
  docs/configuration.md:167, which is P1.M4.T1.S2's explicit scope per architecture/test_patterns.md;
  the README is high-level and the contract forbids feature-blurbs-for-bugfixes). If the decision goes
  the other way, a single minimal line-66 edit is specified verbatim (Outcome B). Scope: README.md ONLY —
  do NOT touch docs/* (S2's territory) or any .go file (P1.M1-P1.M3 own those). This is a review task:
  the implementer's job is to (1) perform the deterministic 4-area review, (2) decide the §5 judgment
  call, (3) either make NO edit (Outcome A, default) or the ONE line-66 edit (Outcome B), and (4) document
  the review outcome. NO code, NO tests, NO docs/* changes.

---

## Goal

**Feature Goal**: Ensure README.md is accurate and consistent with the fixed implementation after the
6-issue bugfix changeset (Issues 1-6), by systematically reviewing the 4 areas the contract names
(token_limit/closed-loop, bootstrap/pi models, config upgrade/backup, auto-stage notice) and correcting
any STALE references — without adding feature blurbs for bugfixes. The expected outcome (confirmed by
research) is that NO changes are needed: the bugfixes align the implementation with the existing
(correct) README documentation.

**Deliverable**:
- **Outcome A (DEFAULT, recommended)**: `README.md` unchanged. The 4-area review is documented in the
  changeset's commit message / PR description (the contract: "If no changes needed, note that in the
  commit message"). `git status --porcelain` shows no README change.
- **Outcome B (OPTIONAL, only if the §5 judgment call is decided "yes")**: a SINGLE minimal edit to
  `README.md` line 66 (the Features-table "Payload optimization" row) adding a floor-rejection
  parenthetical (verbatim wording in §5). `git status --porcelain` shows `README.md` ONLY.

**Success Definition**:
- The 4-area review (§1 of the PRP) is performed and each area's finding recorded: (a) token_limit
  line 66 — accurate (fix makes "never exceeds the limit" true); (b) bootstrap/pi lines 232-246 — accurate
  (line 246 "pi per-role models are left empty" = the fixed behavior); (c) config upgrade lines 257/263/293
  — accurate (no backup mention, no stale claim); (d) auto-stage notice — NOT in the README (no edit needed).
- ZERO stale references remain in README.md (every claim consistent with the §2 fixed behavior).
- The §5 judgment call (Issue 4 floor note in README line 66) is DECIDED and the decision is defensible.
- If Outcome A: README.md unchanged; review documented in commit/PR message.
- If Outcome B: the single line-66 edit is applied verbatim, renders correctly (markdown table row intact),
  and is accurate against the §2 fixed behavior.
- `git status --porcelain` shows EITHER nothing (Outcome A) OR `README.md` ONLY (Outcome B). NEVER
  `docs/*` (S2's scope) or any `.go` file.
- `make test` + `make build` + `make lint` clean (sanity — a markdown edit cannot break these, but
  confirm no collateral).

## User Persona (if applicable)

**Target User**: A Stagecoach user reading the README to evaluate/install/configure the tool. The README
is the marketing + quick-start surface; it must not over-promise (stale "works" claims for since-fixed
bugs) or under-describe.
**Use Case**: A user reads the "Payload optimization" Features row or the "Configure your agent" bootstrap
section and relies on the description being accurate.
**User Journey**: user reads README → installs → runs `config init` / `stagecoach` → behavior matches the
README. (After the bugfixes this is now TRUE; before, the bootstrap silently produced invalid pi models.)
**Pain Points Addressed**: stale documentation that contradicts the fixed implementation. (Finding: none
exist — the README already described the correct behavior; the bugs made the IMPL wrong, not the docs.)

## Why

- **Contract obligation**: the changeset's Mode-B documentation sync requires reviewing the README for
  changeset-level impacts. This task IS that review.
- **Truth-in-advertising**: a README that over-promises (e.g. claiming a closed-loop guarantee that was
  silently violated below ~270 tokens, or claiming pi models are "left empty" when the bootstrap wrote
  bare models) misleads users. The bugfixes make these claims TRUE — so the review's job is to CONFIRM
  accuracy, not to invent edits.
- **Minimal-diff discipline**: the contract is explicit — "Do NOT add feature blurbs for bugfixes; only
  correct stale references." A docs-review that rewrites accurate prose is scope creep. The honest outcome
  (no changes) is the CORRECT outcome when the docs already match the fixed impl.
- **Scope hygiene**: the detailed floor-rejection note (Issue 4) and the backup note (Issue 3) belong in
  docs/ (S2's territory), NOT the high-level README. This task routes them there rather than bloating the
  README.

## What

A systematic review of README.md against the 6-issue bugfix, covering the 4 contract-named areas. The
review (performed in research, documented in §1) finds ZERO stale references. The deliverable is therefore
 EITHER no edit (Outcome A) or one optional line-66 edit (Outcome B, per the §5 judgment call).

### Success Criteria
- [ ] The 4-area review is performed (the implementer READS README lines 66, 232-246, 257, 263, 293 and
      confirms each against the §2 fixed behavior — do not blindly trust the research; re-verify).
- [ ] Area (a) line 66: confirmed accurate ("never exceeds the limit" is true after Issue 4 rejects
      sub-floor limits before assembly). NO stale reference.
- [ ] Area (b) lines 232-246: confirmed accurate (line 246 "pi per-role models are left empty" = the
      Issue 1/2 fixed behavior; line 242 "bare model on pi is a config error (FR-R5b)" = enforced by the
      fixes). NO stale reference.
- [ ] Area (c) lines 257/263/293: confirmed accurate (no backup mention; Issue 3 adds backup but the
      README never claimed otherwise). NO stale reference.
- [ ] Area (d) auto-stage notice: confirmed ABSENT from the README (grep "staging all changes" → 0 hits),
      so Issue 6's grammar fix has no README impact. NO stale reference.
- [ ] The §5 judgment call is DECIDED (Outcome A vs Outcome B) with a one-line rationale.
- [ ] If Outcome A: README.md unchanged; the review summary is written into the commit/PR message.
- [ ] If Outcome B: README.md line 66 edited VERBATIM per §5; the markdown table row still renders.
- [ ] `git status --porcelain` shows nothing (Outcome A) OR `README.md` ONLY (Outcome B).
- [ ] NO edit to `docs/*` (S2) or any `.go` file (P1.M1-P1.M3). `make test`/`make build`/`make lint` clean.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the complete line-by-line review with exact README line numbers + the finding per area (no stale
references), the fixed behavior per issue (§2 table, the source of truth the README is measured against),
the scope fence (README only; docs/* is S2), the one judgment call with a recommendation + the verbatim
Outcome B edit, the validation approach for a docs-review task, and the grep guards.

### Documentation & References

```yaml
# MUST READ — the codebase-specific review (the 4 areas + exact line numbers + per-area findings).
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M4T1S1/research/findings.md
  why: "§0 the headline (README needs no changes); §1 the area-by-area review with EXACT line numbers +
        finding (a/b/c/d + bonus e); §2 the fixed-behavior-per-issue table (the source of truth); §3 scope
        fence (README only; docs/* is S2); §4 the validation approach; §5 THE ONE JUDGMENT CALL (Issue 4
        floor note) with the recommendation + the verbatim Outcome B edit; §6 the default deliverable."
  critical: "The conclusion is NO CHANGES (Outcome A) unless the implementer decides §5 the other way.
             Do not invent edits. Re-verify each line against the §2 fixed behavior before concluding."

# MUST READ — the file under review (read it fully; re-verify the 4 areas — do not blindly trust research).
- file: README.md
  why: "The review target. Lines to re-verify: 66 (token_limit closed-loop claim), 232-246 (bootstrap/pi
        models — esp. 246 'per-role models are left empty'), 257/263/293 (config upgrade), and grep for
        'staging all changes' (auto-stage notice — expect 0 hits). Confirm each matches the §2 fixed behavior."

# MUST READ — the architecture spec for Issues 1 & 2 (the bootstrap pi-model bug) so the fixed behavior
#              the README line 246 describes is understood precisely.
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/bootstrap_pi_model_bug.md
  why: "Documents the root cause (role_defaults.go bare pi models) + both buggy paths (stager-fallback,
        commented-block) + the fixes (blank + guidance comment). README line 246 'pi per-role models are
        left empty' describes EXACTLY the fixed output. Confirms area (b) is accurate post-fix."

# MUST READ — the architecture spec for Issues 4/5/6 (the floor-rejection + the two UI-text fixes).
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/minor_fixes.md
  why: "Issue 4: the IrreducibleFloor helper + caller rejection (makes README line 66 'never exceeds the
        limit' TRUE). Issue 5: ErrEmptyMessage prefix drop (not in README). Issue 6: the auto-stage noun
        conditional (not in README — grep confirms). The 'Documentation Impact' note under Issue 4 explicitly
        assigns the floor note to docs/configuration.md:167 — i.e. S2's scope, NOT the README's."

# MUST READ — the architecture spec for Issue 3 (config upgrade backup).
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/config_upgrade_backup.md
  why: "Documents the WriteTimestampedBackup fix. The README does NOT mention backup (grep confirms), so
        there is no stale claim to correct — confirms area (c) is a no-op for the README."

# CONTEXT — the test_patterns doc's "Documentation Files" section (maps each fix to the doc it affects).
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/test_patterns.md
  section: "## Documentation Files (for §5 Documentation Sync)"
  why: "Confirms the README's only flagged lines (66 token_limit, 242 pi bare model) and that the
        floor-rejection 'Mode A impact' is assigned to docs/configuration.md:167 (S2), NOT README. This is
        the basis for the §5 recommendation (route the floor note to S2, don't add it to the README)."

# CONTEXT — the contract (the item description): the 4 review areas + the "no feature blurbs" rule.
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/prd_snapshot.md
  section: "P1.M4.T1.S1 contract (LOGIC + OUTPUT + DOCS clauses)"
  why: "The contract itself: review areas (a)-(d); 'if everything is already accurate, make no changes
        (but document that the review was done)'; 'Do NOT add feature blurbs for bugfixes — only correct
        stale references'; 'If no changes needed, note that in the commit message.' This item IS Mode B."
```

### Current Codebase tree (relevant slice)

```bash
README.md                                  # REVIEW TARGET (Outcome A: unchanged; Outcome B: 1 line-66 edit)
docs/how-it-works.md                       # READ-ONLY — P1.M4.T1.S2's territory (do NOT touch)
docs/cli.md                                # READ-ONLY — P1.M4.T1.S2's territory (do NOT touch)
docs/configuration.md                      # READ-ONLY — S2's territory (the Issue 4 floor note goes HERE, line 167)
docs/providers.md                          # READ-ONLY — S2's territory (the Issue 2 commented-block note goes here)
internal/config/bootstrap.go               # READ-ONLY — Issue 1/2 fix landed (P1.M1/P1.M2)
internal/config/backup.go                  # READ-ONLY — Issue 3 fix landed (P1.M2.T2.S1)
internal/git/tokengate.go + git.go         # READ-ONLY — Issue 4 fix landed (P1.M3.T1.S1)
internal/generate/finalize.go              # READ-ONLY — Issue 5 fix landed (P1.M3.T2.S1)
internal/cmd/default_action.go             # READ-ONLY — Issue 6 fix landed (P1.M3.T3.S1)
plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/*.md  # READ-ONLY — the issue specs (the source of truth)
```

### Desired Codebase tree with files to be added/modified

```bash
# Outcome A (DEFAULT): NO files modified. The review is documented in the commit/PR message.
# Outcome B (OPTIONAL): README.md  (single line-66 edit per §5).
# NOTHING ELSE. No docs/* edit (S2). No .go edit (P1.M1-P1.M3). No PRD/task file edit.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (the README already describes the FIXED behavior — do not "correct" it backwards): the most
     likely mistake is an implementer who hasn't read the issue specs "fixing" README line 246 ("pi per-role
     models are left empty") to say something else, or weakening line 66's "never exceeds the limit" claim.
     BOTH are already correct post-fix. The bugs were in the IMPLEMENTATION; the docs were right. Verify
     against §2 before editing anything. -->

<!-- CRITICAL (the Issue 4 floor note belongs in docs/configuration.md, NOT README): test_patterns.md
     "Mode A impact" explicitly assigns it to docs/configuration.md:167, which is P1.M4.T1.S2's scope.
     Adding it to the README (a) duplicates S2's work, (b) bloats a high-level Features row, (c) edges
     toward a "feature blurb for a bugfix" the contract forbids. Default: route to S2, don't edit README. -->

<!-- GOTCHA (README line 66 is a MARKDOWN TABLE ROW — Outcome B must keep it one row): the "Payload
     optimization" row is a single `| ... |` line. The Outcome B parenthetical must stay INLINE (no
     newlines), or it breaks the Features table. Keep the existing links ([how it works](...) · [knobs](...)). -->

<!-- GOTCHA (the auto-stage notice and the --edit abort are NOWHERE in the README): grep confirms zero
     matches for "staging all changes", "(1 files)", "empty commit message". Issues 5 and 6 are internal
     UI-text fixes with NO README surface. Do not add example output for them (that would be a feature blurb). -->

<!-- GOTCHA (config-upgrade backup is NOT mentioned in the README): Issue 3 adds a backup, but the README
     never claimed upgrade was backup-less. There is no stale claim. Adding "(creates a backup)" is a feature
     blurb — route the detail to docs/cli.md (S2). -->

<!-- GOTCHA (do NOT touch docs/* — that's S2): the bugfix plan splits docs sync into S1 (README) and S2
     (docs/how-it-works.md + docs/cli.md, plus the configuration.md/providers.md Mode-A notes). Editing
     docs/* here collides with S2's scope. README.md ONLY. -->
```

## Implementation Blueprint

### Data models and structure

None. This is a documentation review/edit task. No code, no types, no config.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: RE-VERIFY the 4 review areas against the fixed implementation (do not blindly trust the research)
  - READ README.md fully (391 lines — small). Re-verify each area against the §2 fixed-behavior table.
  - AREA (a) — README line 66 (Features "Payload optimization" row): the "closed-loop guarantee that the
    assembled prompt never exceeds the limit" claim. Confirm: after Issue 4 (P1.M3.T1.S1), a sub-floor
    token_limit ERRORS at StagedDiff/TreeDiff/WorkingTreeDiff before assembly, so the claim is TRUE for
    token_limit >= floor (gate trims) AND vacuously TRUE for token_limit < floor (aborts before send).
    → EXPECTED FINDING: accurate; no stale reference.
  - AREA (b) — README lines 232-246 ("Configure your agent"): line 232 (single-backend bare model),
    235-237 (pi zai/glm-5.2 prefix example), 241-243 (FR-R5b bare-model-is-an-error), 246 ("for pi, the
    default, per-role models are left empty"). Confirm: after Issues 1 & 2 (P1.M1/P1.M2), the bootstrap
    emits BLANK pi models (stager-fallback blanked + commented-block blanked), matching line 246 EXACTLY.
    → EXPECTED FINDING: accurate; no stale reference.
  - AREA (c) — README lines 257, 263, 293 (config upgrade): confirm the README mentions upgrade but NOT
    backup. Issue 3 (P1.M2.T2.S1) ADDS a backup; there is no stale "no backup" claim to correct.
    → EXPECTED FINDING: accurate; no stale reference.
  - AREA (d) — auto-stage notice: `grep -nE "staging all changes|Nothing staged|files\)" README.md` →
    expect ZERO hits. Issue 6 (P1.M3.T3.S1) is UI text the README never shows.
    → EXPECTED FINDING: not present; no stale reference.
  - BONUS (e) — --edit abort (Issue 5): `grep -nE "empty commit|stagecoach:.*abort" README.md` → expect
    ZERO hits (line 136 lists --edit as a flag, no output shown).
    → EXPECTED FINDING: not present; no stale reference.
  - OUTPUT of Task 1: a confirmed per-area finding table. If ANY area diverges from the research finding,
    STOP and re-read the relevant architecture/*.md spec before deciding — the research may have missed
    a post-fix drift, OR the implementer may be misreading. Reconcile before editing.

Task 2: DECIDE the §5 judgment call (the Issue 4 floor note in README line 66)
  - The ONE open decision: does the floor-rejection detail warrant a parenthetical on README line 66?
  - RECOMMENDATION (default): NO → Outcome A. Rationale: (1) line 66 is already accurate post-fix;
    (2) test_patterns.md assigns the floor note to docs/configuration.md:167 (S2's scope); (3) the README
    is high-level (a Features table row); (4) the contract forbids feature-blurbs-for-bugfixes.
  - ALTERNATIVE (Outcome B): YES → apply the single line-66 edit in Task 3b. Use this ONLY if you believe
    a README reader setting token_limit=100 needs the floor-rejection hint inline (defensible, but the
    error message itself is already actionable: "raise it to at least N").
  - RECORD the decision + one-line rationale (goes into the commit/PR message either way).

Task 3a (Outcome A — DEFAULT): MAKE NO EDIT; document the review
  - README.md is UNCHANGED.
  - WRITE the review summary into the changeset's commit message / PR description. Suggested text:
      "Reviewed README.md against the Issues 1-6 bugfix changeset (P1.M4.T1.S1). All 4 contract review
       areas are accurate post-fix — no stale references found:
       (a) line 66 'closed-loop guarantee ... never exceeds the limit' is now TRUE (Issue 4 rejects
           sub-floor token_limit before assembly);
       (b) lines 232-246 already describe the fixed bootstrap behavior (line 246 'pi per-role models are
           left empty' = the Issue 1/2 fixed output; line 242 FR-R5b bare-model-error is now enforced);
       (c) config upgrade (257/263/293) is accurate (Issue 3 adds backup; README never claimed otherwise);
       (d) the auto-stage notice (Issue 6) does not appear in the README.
       The Issue 4 floor-rejection detail is routed to docs/configuration.md (P1.M4.T1.S2). No README
       changes required."
  - PROCEED to Task 4 (validation).

Task 3b (Outcome B — OPTIONAL, only if Task 2 decided "yes"): EDIT README.md line 66 (the ONLY edit)
  - LOCATE: README line 66 (the Features-table "Payload optimization" row). It is a SINGLE markdown table
    row beginning `| Payload optimization |` and ending ` ([knobs](docs/configuration.md#built-in-defaults)). |`.
  - CURRENT (the relevant clause within the row):
      ...optionally capped to your model's context window via `token_limit` — a closed-loop guarantee that
      the assembled prompt never exceeds the limit ([how it works](docs/how-it-works.md#diff-capture-pipeline)...
  - EDIT: insert the floor-rejection parenthetical IMMEDIATELY AFTER "never exceeds the limit" and BEFORE
      the "([how it works]..." link, keeping it INLINE (no newline — it's a table row):
      ...never exceeds the limit (a limit below the irreducible prompt floor is rejected with an error
      rather than silently broken; see [docs](docs/configuration.md#built-in-defaults)) ([how it works]...
  - PRESERVE: the row's pipe structure (`| ... |`), both links ([how it works] + [knobs]), the em-dashes,
    the backticks around `token_limit`. The edit is a PURE INSERT of one parenthetical clause.
  - VERIFY after edit: the markdown table still renders (the row is still ONE physical line); the new
    clause is accurate against §2 (Issue 4: sub-floor → error, not silent violation).
  - PROCEED to Task 4 (validation).

Task 4: VALIDATE — confirm no collateral + scope guard
  - If Outcome A: `git status --porcelain` shows NO README change (the review is in the commit message).
    If Outcome B: `git status --porcelain` shows `README.md` ONLY.
  - SCOPE GUARD: `git status --porcelain` must NOT show any `docs/*` file or any `.go` file.
    `git diff --name-only | grep -vE '^README\.md$' | grep -q . && echo "FAIL: out-of-scope file" || echo "OK"`
  - SANITY (a markdown edit cannot break code, but confirm): `make build && make test && make lint` → green.
  - MARKDOWN render check (Outcome B only): eyeball the Features table in a renderer (or `glow README.md` /
    GitHub preview) — the "Payload optimization" row is still a single well-formed row.
  - GREP guards (see Validation Loop Level 4).
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN (the review — the core of this task): for each of the 4 areas, (1) read the exact README line(s),
     (2) compare against the §2 fixed behavior, (3) classify: ACCURATE (no edit) / STALE (correct it) /
     ABSENT (no edit — not in README). The research pre-classifies all 4 as ACCURATE/ABSENT; the implementer
     RE-VERIFIES. The expected outcome is 0 corrections. -->

<!-- PATTERN (Outcome B edit — the ONLY candidate code change): a single inline parenthetical INSERT in a
     markdown table row. Keep the row one physical line; preserve both links + the pipe structure. -->
```

### Integration Points

```yaml
DOCUMENTATION (README.md — the ONLY file this task may touch):
  - Outcome A (default): NO edit. Review documented in commit/PR message.
  - Outcome B (optional): ONE inline edit to line 66 (the "Payload optimization" Features row).
NO docs/* edit (P1.M4.T1.S2 owns docs/how-it-works.md + docs/cli.md + the configuration.md/providers.md
  Mode-A notes — including the Issue 4 floor note at docs/configuration.md:167 and the Issue 2
  commented-block note at docs/providers.md).
NO source/test edit (P1.M1-P1.M3 own the .go fixes; this task CONSUMES them, reviews against them).
NO PRD/task file edit.
SCOPE FENCES:
  - Touches ONLY: README.md (Outcome B) or NOTHING (Outcome A).
  - Does NOT touch: docs/* (S2), any .go file (P1.M1-P1.M3), go.mod, any PRD/task file.
  - Adds NO code, NO test, NO new section to the README (Outcome B is a 1-clause insert, not a section).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# A markdown edit (Outcome B) has no compiler. Validate structure manually + via grep.
# Outcome A: skip (no edit). Outcome B:
#   - README line 66 is STILL a single physical line (the table row is intact).
awk 'NR==66 { print "line 66 length:", length($0); n=gsub(/\|/,"|"); print "pipes:", n }' README.md
# Expected (Outcome B): line 66 present, pipe count unchanged (the row still has its `| ... |` structure).

# Markdown lint (if the repo has one; else eyeball).
# (No markdown linter is in the Makefile — the gate is `make lint` on the Go code, which a README edit
#  cannot affect. Eyeball the Features table renders.)

# Scope guard: ONLY README.md (Outcome B) or nothing (Outcome A).
git status --porcelain
# Expected (Outcome A): empty (or no README entry). (Outcome B): README.md ONLY.
git diff --name-only | grep -vE '^README\.md$' | grep -q . && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
# Expected: OK.
```

### Level 2: Unit Tests (Component Validation)

```bash
# N/A — this is a documentation task. There are no unit tests for README content. The "test" is the
# Task 1 review (re-verify each area). A README edit cannot affect `go test`.
```

### Level 3: Integration Testing (System Validation)

```bash
# Sanity: a README edit cannot break the build/tests, but confirm no collateral (e.g. an accidental
# stray edit to a .go file via a fat-fingered sed).
make build && make test && make lint
# Expected: all green (identical to pre-edit). If RED, you accidentally edited a .go file — revert it.
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1 (Outcome A vs B): README.md is EITHER unchanged OR has exactly the line-66 parenthetical.
git diff --stat README.md
# Expected (Outcome A): nothing. (Outcome B): "1 file changed, 1 insertion(+), 1 deletion(-)" (the one row).

# Guard 2: NO stale claim survives — re-confirm the 4 areas post-review.
grep -n 'never exceeds the limit' README.md           # (a) present + accurate (Issue 4 makes it true)
grep -n 'per-role models are left empty' README.md    # (b) present + accurate (Issue 1/2 fixed behavior)
grep -n 'bare model (no `/`) on pi is a config error' README.md  # (b) present + accurate (FR-R5b enforced)
grep -cE 'staging all changes|Nothing staged|\(1 files\)' README.md   # (d) expect 0 (notice not in README)
# Expected: (a) + (b) lines present; (d) 0.

# Guard 3 (Outcome B only): the floor-rejection parenthetical is present + inline.
grep -n 'irreducible prompt floor' README.md
# Expected (Outcome B): 1 hit (line 66). (Outcome A): 0 hits (routed to docs/configuration.md via S2).

# Guard 4: scope — NO docs/* and NO .go file touched.
git diff --name-only | grep -E '^docs/|\.go$' && echo "FAIL: out-of-scope (docs/* or .go)" || echo "OK: no docs/* or .go edit"
# Expected: OK.

# Guard 5: the markdown Features table is intact (Outcome B) — the row count is unchanged.
grep -cE '^\| ' README.md   # count table rows
# Expected: unchanged from pre-edit (Outcome B inserts a clause WITHIN a row, not a new row).
```

## Final Validation Checklist

### Technical Validation
- [ ] `make build` + `make test` + `make lint` clean (sanity — a README edit cannot break these)
- [ ] `git status --porcelain`: nothing (Outcome A) OR `README.md` ONLY (Outcome B)
- [ ] NO `docs/*` or `.go` file touched (grep guards 4)

### Feature Validation (the review)
- [ ] Area (a) line 66 reviewed: "never exceeds the limit" accurate post-Issue-4 (sub-floor rejects before assembly)
- [ ] Area (b) lines 232-246 reviewed: line 246 "pi per-role models are left empty" = the Issue 1/2 fixed behavior; line 242 FR-R5b enforced
- [ ] Area (c) lines 257/263/293 reviewed: config upgrade accurate; no backup claim to correct (Issue 3 adds backup)
- [ ] Area (d) auto-stage notice reviewed: ABSENT from README (grep 0 hits); Issue 6 has no README surface
- [ ] ZERO stale references remain (every claim consistent with the §2 fixed behavior)
- [ ] The §5 judgment call (Issue 4 floor note) is DECIDED with a one-line rationale

### Scope-Boundary Validation
- [ ] `git status` shows nothing (Outcome A) OR `README.md` ONLY (Outcome B)
- [ ] NO edit to `docs/*` (P1.M4.T1.S2 — incl. docs/configuration.md:167 the floor note + docs/providers.md the commented-block note)
- [ ] NO edit to any `.go` file (P1.M1-P1.M3 own the fixes)
- [ ] NO new README section (Outcome B is a 1-clause inline insert, not a section/feature blurb)
- [ ] NO PRD/task file edit

### Code Quality & Docs
- [ ] If Outcome A: the review summary is written into the commit/PR message (per the contract)
- [ ] If Outcome B: the line-66 parenthetical is inline (table row intact), accurate, and minimal
- [ ] The decision (Outcome A vs B) is defensible and recorded

---

## Anti-Patterns to Avoid

- ❌ Don't "correct" README line 246 backwards. The line "for pi, the default, per-role models are left
  empty" is ALREADY the correct (post-Issue-1/2-fix) behavior. Before the fix it was inaccurate (the
  bootstrap wrote bare models); after, it's accurate. Do not rewrite it — the bugs were in the
  IMPLEMENTATION, not the docs.
- ❌ Don't weaken README line 66's "never exceeds the limit" claim. After Issue 4, it's TRUE (sub-floor
  limits error before assembly). The fix makes the claim accurate, not the reverse. The floor-rejection
  DETAIL belongs in docs/configuration.md:167 (S2), not the README — unless you deliberately choose
  Outcome B (§5).
- ❌ Don't add feature blurbs for the bugfixes. The contract is explicit: "Do NOT add feature blurbs for
  bugfixes — only correct stale references." There are NO stale references (the review confirms it). Adding
  "now creates a backup!" / "now grammatically correct!" prose is out of scope and bloats the README.
- ❌ Don't touch docs/*. The bugfix plan splits docs sync: S1 = README, S2 = docs/how-it-works.md +
  docs/cli.md + the configuration.md/providers.md Mode-A notes. The Issue 4 floor note and the Issue 2
  commented-block note are S2's. Editing docs/* here collides with S2's scope.
- ❌ Don't edit any .go file. P1.M1-P1.M3 own and have LANDED the fixes. This task CONSUMES them (reviews
  the README against the fixed behavior); it does not modify source or tests.
- ❌ Don't invent a stale reference to "find" work. If the review confirms everything is accurate (it does),
  the correct outcome is Outcome A (no edit, documented review). A docs-review task with a no-change
  outcome is a SUCCESS, not a failure — it means the docs were already right.
- ❌ Don't break the markdown table (Outcome B). README line 66 is a Features-table row (`| ... |`). The
  floor-rejection parenthetical must be INLINE (no newline), or the table breaks. Preserve both links
  ([how it works] + [knobs]) and the pipe structure. Verify the row count is unchanged (grep guard 5).
- ❌ Don't forget to document the review (Outcome A). The contract says "If no changes needed, note that in
  the commit message." An undocumented no-change review looks like a skipped task. Write the §6 summary
  into the commit/PR message.
- ❌ Don't conflate this with S2's scope. S2 reviews docs/how-it-works.md + docs/cli.md (+ the
  configuration.md/providers.md notes). This task (S1) is README.md ONLY. If you find a docs/* issue during
  the README review, NOTE it for S2 — do not fix it here.
