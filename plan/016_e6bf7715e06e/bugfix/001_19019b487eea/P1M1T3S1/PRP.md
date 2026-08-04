name: "P1.M1.T3.S1 — Verify all 7 provider rows in docs/providers.md match binary values + cross-reference docs/how-it-works.md prose consistency (Mode B docs-sync gate)"
description: >
  VERIFICATION GATE (Mode B, runs last). After P1.M1.T1.S1 (opencode Delivery cell: positional→stdin,
  Complete) and P1.M1.T2.S1 (how-it-works safety parenthetical: codex,cursor → exhaustive 5 providers,
  lands per contract), sweep for ZERO documentation drift: (a) rebuild the binary and cross-check all 7
  providers' `prompt_delivery` against the docs/providers.md Delivery column (7/7); (b) confirm the
  docs/how-it-works.md safety-paragraph partition (2 explicit-switch + 5 read-only-constrained = 7) is
  consistent with the docs/providers.md Tool-disable column; (c) grep docs/ + README.md for any residual
  `opencode`↔`positional` drift (zero expected); (d) verify the docs/providers.md "Chrome is a separate
  axis" bullet names all 5 constrained providers. All four checks are EXPECTED TO PASS — the primary
  deliverable is a verified docs/ directory + green gates (no edits if all pass). If a check
  unexpectedly fails, fix the specific drift per the relevant prior subtask's contract and re-verify.
  No code, no test, no PRD change.

---

## Goal

**Feature Goal**: Prove there is **zero documentation drift** between the shipped binary and the
provider/delivery documentation, and that the cross-references between `docs/providers.md` and
`docs/how-it-works.md` are mutually consistent. This is the final gate that lets the v2.9
chrome-disable changeset ship with a coherent docs surface (the explicit purpose of a Mode B
docs-sync task per `architecture/critical_findings.md` Finding 5).

**Deliverable**: A run of four deterministic verification checks (a-d) against a freshly rebuilt
binary, each with a recorded pass/fail result and exact command output. If all four pass → **no edits**
(the gate's job is to certify, not to rewrite). If any check fails → fix the specific drift in the
offending doc file (applying the relevant prior subtask's contract) and re-verify until green. Plus the
standard no-regression gates (`make test`, markdownlint, scope diff).

**Success Definition**:
- **(a) Binary↔Delivery column 7/7**: every provider's `/tmp/stagecoach_verify providers show <name> |
  grep prompt_delivery` value equals the 2nd-column (Delivery) cell of its row in the
  `docs/providers.md` "## The 7 built-in providers" table. Expected set: pi/claude/opencode/codex/agy/
  qwen-code = `stdin`; cursor = `positional`.
- **(b) how-it-works ↔ providers partition**: `docs/how-it-works.md` line ~197 names the 2
  explicit-switch providers `(pi, claude)` AND the 5 read-only-constrained providers `(codex, cursor,
  agy, qwen-code, … opencode …)`, partitioning all 7 with no overlap/omission, consistent with the
  `docs/providers.md` "Tool-disable approach" column (which has one read-only-constraint row each for
  codex/cursor/agy/qwen-code/opencode and explicit-switch rows for pi/claude).
- **(c) No residual opencode↔positional drift**: `grep -rn 'positional.*opencode\|opencode.*positional'
  docs/ README.md` returns **zero** matches. (The only remaining `positional` in docs is the cursor row
  and the generic schema/rendering enum lines — all correct.)
- **(d) Chrome bullet complete**: the `docs/providers.md` "Chrome is a separate axis" bullet (line ~100)
  names all 5 constrained providers (codex, cursor, opencode, agy, qwen-code).
- **Gates**: `make test` green; `npx markdownlint-cli2 docs/providers.md docs/how-it-works.md` still 0
  errors (current baseline = 0); `git diff` (this subtask's contribution) is either EMPTY (all-pass
  branch) or a single localized doc-cell fix (fail-then-fix branch).

## User Persona (if applicable)

**Target User**: The maintainer / reviewer certifying the v2.9 chrome-disable changeset for release,
who needs assurance that the docs no longer disagree with the binary or with each other.

**Use Case**: "Before I tag this release, prove the provider table and the safety-paragraph prose both
agree with `stagecoach providers show`." — this gate produces that proof (exact commands + recorded
output) in one pass.

**Pain Points Addressed**: PRD Issue 1 (opencode Delivery drift — fixed by T1.S1) and Issue 2
(how-it-works incomplete enumeration — fixed by T2.S1) each fixed ONE file; this gate ensures the two
fixes are mutually consistent AND that no third drift site was missed (the original bug was caused by a
table-rewrite carrying stale cells forward — a sweep guards against the same class of regression in
adjacent rows/lines and in README/docs-README).

## Why

- **Mode B docs-sync purpose**: per `architecture/critical_findings.md` Finding 5, both upstream fixes
  are doc edits (not code-with-doc-implications), so "the Mode B sweep is a verification step, not a
  writing step." This subtask IS that sweep.
- **Defense against the recurrence pattern**: the original drift entered because commit `b8f081d`
  rewrote every providers-table row to add a column and carried the stale opencode `positional` forward;
  commit `71e57c7a` rewrote the safety paragraph and carried the stale `(codex, cursor)` forward. A
  rebuild-and-diff sweep against the binary catches exactly this class of stale-cell-carried-forward
  regression — for ALL 7 rows, not just the one that was reported.
- **Byte-faithfulness contract**: `docs/README.md` promises "the `docs/` directory tracks the shipped
  binary … the binary is authoritative." This gate is the operational check that enforces that contract
  for the provider manifests.
- **Bounded scope**: verification + (only-if-needed) a localized fix. No code, no test, no PRD, no
  changelog. Does not re-open T1.S1/T2.S1 design decisions.

## What

**User-visible behavior**: None at runtime (docs-only). The observable artifact is a consistent docs/
surface: the providers table, the safety-paragraph prose, and the READMEs all agree with the binary and
with each other.

**Technical change**: Primarily verification commands. Edits (if any) are limited to a single doc cell/
substring in the file where a check fails, applying the corresponding prior subtask's contract
verbatim (T1.S1's opencode `positional`→`stdin`, or T2.S1's exhaustive parenthetical). The expected
outcome after T1.S1+T2.S1 is **no edits**.

### Success Criteria
- [ ] Check (a): 7/7 binary↔Delivery-column match (cursor=positional; the other six=stdin)
- [ ] Check (b): how-it-works partition = 2 + 5 = 7, matching providers.md Tool-disable column
- [ ] Check (c): zero `opencode`↔`positional` matches in docs/ + README.md
- [ ] Check (d): "Chrome is a separate axis" bullet names all 5 constrained providers
- [ ] `make test` green; markdownlint still 0 errors on both files
- [ ] Scope: this subtask's diff is EMPTY (all-pass) or a single localized doc fix (fail-then-fix); no
      code/test/PRD/TOML change

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact build + query commands, the exact expected binary value per provider (measured), the
exact docs/providers.md table layout (Delivery = 2nd column) and current row states, the T2.S1 contract
substring to look for (and to apply if missing), the exact grep patterns and their expected outputs, the
markdownlint baseline (0), the scope fences (esp. do-NOT-expand line 96), and the all-pass-vs-fix
decision rule.

### Documentation & References

```yaml
# MUST READ — the measured ground truth (binary values, current doc states, baselines)
- docfile: plan/016_e6bf7715e06e/bugfix/001_19019b487eea/P1M1T3S1/research/verification_ground_truth.md
  why: "Sections 1-7 give the VERIFIED binary values for all 7 providers, the current docs/providers.md
        Delivery-column cell per row (7/7 already match), the T2.S1 dependency note (how-it-works is
        still stale in the tree until T2.S1 lands), the residual-drift grep result (zero), the chrome
        bullet completeness proof, and the markdownlint baseline (0 errors). Read before running."
  critical: "§5 scope note: do NOT expand the 'Read-only constraint (codex, cursor)' parenthetical on
             docs/providers.md line 96. That is NOT a PRD-defined defect (PRD Issue 2 = how-it-works
             line 197 ONLY). The full 5-provider enumeration already lives in the table column + the
             chrome bullet (line 100). Expanding line 96 = out-of-scope scope creep."

# MUST READ — the upstream analysis (defect definitions + Mode B = verification step)
- docfile: plan/016_e6bf7715e06e/bugfix/001_19019b487eea/architecture/critical_findings.md
  why: "Finding 5 establishes this subtask as a verification step (not a writing step). Finding 1
        confirms opencode was the only binary mismatch. Finding 4 confirms no test changes are needed."
- docfile: plan/016_e6bf7715e06e/bugfix/001_19019b487eea/architecture/system_context.md
  why: "§'Full 7-provider cross-check' is the authoritative 7×7 table (binary vs docs) and the per-defect
        fix locations. The cross-check was already performed during research; T3.S1 re-runs it post-fix."
  section: "Full 7-provider cross-check (binary vs docs table)"

# MUST READ — the contracts this gate consumes/verifies (treat as CONTRACT)
- docfile: plan/016_e6bf7715e06e/bugfix/001_19019b487eea/P1M1T1S1/PRP.md
  why: "T1.S1 (Complete) fixed docs/providers.md opencode Delivery cell: positional→stdin. Confirms the
        edit is a single cell; the gate verifies it stuck and that no sibling row regressed."
- docfile: plan/016_e6bf7715e06e/bugfix/001_19019b487eea/P1M1T2S1/PRP.md
  why: "T2.S1 (in-flight, lands per contract) expands docs/how-it-works.md line 197 to the exhaustive
        5-provider parenthetical: 'read-only constraint flags (codex, cursor, agy, qwen-code; opencode's
        \`run\` is read-only by design).' Check (b) verifies THIS exact form is present. If T2.S1 has NOT
        landed when T3.S1 runs, T3.S1 applies this exact substring (contract: 'fix any inconsistency')."
  critical: "The (pi, claude) parenthetical is UNCHANGED and correct — do not touch it. Only the
             read-only-constraint parenthetical is the T2.S1 contract."

# SOURCE OF TRUTH — the binary the docs must mirror (read-only; do NOT edit)
- file: internal/provider/builtin.go
  why: "The compiled-in manifests. opencode = builtinOpenCode() ~line 327, PromptDelivery strPtr(\"stdin\").
        cursor is the ONLY positional provider. Querying the BUILT BINARY (Task 2) is preferred over
        reading source (the binary is what the docs/README.md contract calls 'authoritative')."
  critical: "This gate rebuilds and QUERIES the binary — it does not edit builtin.go. If a check fails it
             means the DOC is wrong (the binary is authoritative), never the reverse."

# THE FILES UNDER VERIFICATION (read-only for the pass-branch; the only edit targets if a check fails)
- file: docs/providers.md
  why: "The '## The 7 built-in providers' table (line 74; rows 80-86; Delivery = 2nd column) is check
        (a)'s target. The '## Tools-disable asymmetry' section (line 90; chrome bullet line 100) is
        check (d)'s target."
  pattern: "Row format: '| `name` | delivery | print_flag | model_flag | default_model | sys_prompt_flag | tool-disable | chrome-disable | stager |'"
  gotcha: "Delivery is the 2ND column (after Provider). The 9-column table is wide — extract column 2
           precisely when comparing (see the awk in Validation Level 4)."
- file: docs/how-it-works.md
  why: "The '### Safety invariant' paragraph (line 197) is check (b)'s target — it must carry T2.S1's
        exhaustive 5-provider parenthetical."
- file: docs/README.md
  why: "States the byte-faithfulness contract ('the binary is authoritative'); also scanned by check (c)
        for residual opencode↔positional drift. Expected: zero drift (verified)."
- file: README.md
  why: "Top-level README; scanned by check (c). opencode appears here only in provider-LIST prose (never
        tied to a delivery method) — expected: zero drift."
```

### Current Codebase tree (relevant slice)

```bash
# Files UNDER VERIFICATION (read-only unless a check fails):
docs/providers.md          # check (a): table Delivery col (rows 80-86); check (d): chrome bullet (line ~100)
docs/how-it-works.md       # check (b): safety paragraph (line ~197, post-T2.S1)
docs/README.md             # check (c): residual drift scan
README.md                  # check (c): residual drift scan
# Source of truth (read-only; queried via the built binary):
internal/provider/builtin.go   # compiled-in manifests; opencode=stdin, cursor=positional (the rest stdin)
# Tooling:
.markdownlint.json         # {default:true, MD013:false, MD033:false, MD060:false}; baseline = 0 errors on both docs
Makefile                   # `make test` (race suite); `make build`
```

### Desired Codebase tree with files to be added/modified

```bash
# Expected (all-pass branch): NO file changes — the gate certifies and produces a verification record.
# Fail-then-fix branch: at most ONE localized doc edit in the single file where a check failed
#   (docs/providers.md Delivery cell, or docs/how-it-works.md parenthetical), per the prior subtask's
#   contract. No new files; no code/test/PRD/TOML change.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (binary is authoritative): docs/README.md promises "the binary is authoritative." If a
     check (a) mismatch appears, the DOC cell is wrong — fix the doc, NEVER edit builtin.go. The binary
     value is the truth (opencode=stdin because stdin avoids the 128 KB MAX_ARG_STRLEN ceiling). -->

<!-- CRITICAL (T2.S1 dependency): check (b) verifies the POST-T2.S1 state. The working tree currently
     STILL has the stale '(codex, cursor)' form (T2.S1 not yet landed). If the exhaustive form is absent
     when T3.S1 runs, that is drift T3.S1 is EMPOWERED to fix — apply T2.S1's exact substring:
     'read-only constraint flags (codex, cursor).' → 'read-only constraint flags (codex, cursor, agy,
     qwen-code; opencode's `run` is read-only by design).' (preserve the trailing period). -->

<!-- CRITICAL (do NOT expand docs/providers.md line 96): the 'Read-only constraint (codex, cursor)'
     parenthetical in the Tools-disable-asymmetry section is NOT a PRD-defined defect. PRD Issue 2 is
     scoped to how-it-works line 197 ONLY. Contract point (d) scopes verification to the 'Chrome is a
     separate axis' bullet (line ~100), which IS complete. The full 5-provider enumeration already lives
     in the table's Tool-disable column (one row each) + the chrome bullet. Expanding line 96 = scope
     creep + a surprise edit. Flag it in the report as an OPTIONAL observation if you like, but make NO
     edit. -->

<!-- GOTCHA (Delivery = column 2, not column 3): the providers table has 9 columns; Delivery is the
     2nd (right after Provider). The Chrome-disable column is 8th; Tool-disable approach is 7th. When
     extracting the Delivery cell programmatically, split on '|' and take index 2 (col 0 is empty from
     the leading '|'). See the awk one-liner in Validation Level 4. -->

<!-- GOTCHA (cursor is the LEGITIMATE positional): there is exactly ONE provider whose Delivery is
     'positional' — cursor. So check (c)'s grep for residual drift must NOT flag the cursor row. The
     patterns 'positional.*opencode' and 'opencode.*positional' specifically require opencode adjacent
     to positional, which correctly excludes the cursor-only row. -->

<!-- GOTCHA (markdownlint baseline = 0): both docs/providers.md and docs/how-it-works.md currently lint
     CLEAN (0 errors). The gate is 'still 0 errors' — any new error means a fix (T2.S1's or T3.S1's)
     introduced a markdown defect. The backticks around `run` in T2.S1's parenthetical are valid inline
     code and do NOT error. -->

<!-- SCOPE FENCE: no code/test/PRD/TOML/.gitignore change. No changelog. No new files. Do not re-open
     T1.S1/T2.S1 design decisions (the exhaustive-vs-exemplary phrasing was settled by T2.S1 = exhaustive).
     The all-pass branch produces NO diff at all — that is a valid, expected outcome (Mode B = verify). -->
```

## Implementation Blueprint

### Data models and structure
None. This is a verification gate; no types, no code. The "data" is the 7×(binary value, doc cell)
comparison and the grep results.

### Implementation Tasks (ordered by dependencies)

```yaml
PREREQUISITE: confirm both upstream subtasks' effects are present in the working tree:
  - T1.S1: docs/providers.md opencode row (line ~82) Delivery cell == 'stdin' (grep it).
  - T2.S1: docs/how-it-works.md line ~197 carries the exhaustive parenthetical '(codex, cursor, agy,
    qwen-code; opencode's `run` is read-only by design)'.
  If T2.S1's effect is ABSENT, proceed — Task 4 (check b) will detect it and Task 6 handles the fix.

Task 1: REBUILD the binary into a verification path (isolated from `make build`'s ./bin/stagecoach)
  - RUN: go build -o /tmp/stagecoach_verify ./cmd/stagecoach
  - EXPECT: clean (no stdout/stderr output); exit 0; /tmp/stagecoach_verify created.
  - WHY a separate path: avoids clobbering the project's ./bin/stagecoach; the contract names
    /tmp/stagecoach_verify explicitly.

Task 2: CHECK (a) — cross-check all 7 providers' prompt_delivery vs docs/providers.md Delivery column
  - FOR each provider in pi claude opencode codex cursor agy qwen-code:
      bin=$(/tmp/stagecoach_verify providers show "$p" | grep prompt_delivery | sed -E "s/.*= '([^']*)'.*/\1/")
  - EXPECT the set: pi=stdin, claude=stdin, opencode=stdin, codex=stdin, cursor=positional, agy=stdin,
    qwen-code=stdin.
  - EXTRACT the doc Delivery cell (column 2) for each provider's row in docs/providers.md (table header
    line ~78; rows ~80-86). The cell is the 2nd pipe-delimited field (between the 1st and 2nd '|').
  - ASSERT: bin value == doc cell, for all 7. Record the 7×2 table in the verification output.
  - EXPECTED VERDICT: 7/7 match (verified baseline). If ANY mismatch → go to Task 6 with the offending
    provider (apply T1.S1's contract: set the cell to the binary value).

Task 3: CHECK (c) — grep for residual opencode↔positional drift (run before b/d; cheapest filter)
  - RUN: grep -rn 'positional.*opencode\|opencode.*positional' docs/ README.md
  - EXPECT: exit 1, ZERO matches.
  - IF non-zero matches → go to Task 6 (fix the offending line so opencode is never paired with
    positional). The only legitimate 'positional' in docs is the cursor row + the generic enum/pseudo-
    code lines (providers.md:22, :64, :84; README.md:308) — none of which pair opencode with positional.

Task 4: CHECK (b) — how-it-works safety partition ↔ providers.md Tool-disable column
  - READ docs/how-it-works.md line ~197 (the '### Safety invariant' paragraph).
  - ASSERT it contains BOTH:
      (i)  'explicit tool-disable flags (pi, claude)'      ← unchanged, correct
      (ii) 'read-only constraint flags (codex, cursor, agy, qwen-code; opencode's `run` is read-only by design)'
           ← T2.S1's exhaustive form
  - ASSERT the partition = 2 (pi, claude) + 5 (codex, cursor, agy, qwen-code, opencode) = 7 unique
    providers, no overlap, no omission.
  - CROSS-REFERENCE docs/providers.md 'Tool-disable approach' column (7th column): it must show
    explicit-switch rows for pi+claude and read-only-constraint rows for codex/cursor/agy/qwen-code/
    opencode (one each). The two docs must agree.
  - EXPECTED VERDICT: pass (IF T2.S1 has landed). If the exhaustive form (ii) is ABSENT → Task 6 (apply
    T2.S1's exact substring replacement to docs/how-it-works.md line ~197; see research §3).

Task 5: CHECK (d) — docs/providers.md 'Chrome is a separate axis' bullet completeness
  - READ docs/providers.md line ~100 (the 'Chrome is a separate axis' bullet under '## Tools-disable
    asymmetry', section at line ~90).
  - ASSERT the bullet's 'providers that do not [expose a switch]' group names ALL 5 constrained
    providers: 'codex, cursor, opencode, agy, qwen-code'.
  - EXPECTED VERDICT: pass (verified baseline). If any of the 5 is missing → Task 6 (add it to that
    group, preserving the existing order/comma style).
  - NOTE: do NOT expand the 'Read-only constraint (codex, cursor)' parenthetical on line ~96 — out of
    scope (see Gotchas). It is acceptable as-is; flag in the report only.

Task 6: (CONDITIONAL — only if a check failed) FIX the specific drift, then re-run the failing check
  - This task runs ONLY for a check that failed. Identify the single offending file + cell/substring:
      * docs/providers.md Delivery cell mismatch → set cell to the binary value (T1.S1 contract).
      * docs/how-it-works.md stale parenthetical → apply T2.S1's exhaustive substring (preserve trailing
        period; keep '(pi, claude)' untouched).
      * residual opencode↔positional line → correct it so opencode is paired only with stdin.
      * missing provider in chrome bullet → add it, preserving order/style.
  - AFTER the fix: re-run the specific check that failed (Tasks 2-5) until it passes; then re-run ALL
    checks to confirm no regression.
  - RECORD: which check failed, the fix applied, and the re-verification result.

Task 7: FINAL GATES — no-regression + scope + lint
  - go build ./...                 # clean (no code change expected; confirms no accidental touch)
  - make test                      # green (race suite; docs-only change can't break it — sanity check)
  - npx --no-install markdownlint-cli2 docs/providers.md docs/how-it-works.md   # still 0 errors
  - git diff --stat -- '*.go' internal/ providers/ PRD.md   # EMPTY (no code/PRD/TOML change)
  - git diff --stat                                        # all-pass → EMPTY; fail-then-fix → only the one doc
```

### Implementation Patterns & Key Details

```bash
# PATTERN: the isolated verification binary (do NOT clobber ./bin/stagecoach)
go build -o /tmp/stagecoach_verify ./cmd/stagecoach
# query one provider's delivery (binary is authoritative)
/tmp/stagecoach_verify providers show opencode | grep prompt_delivery   # → prompt_delivery = 'stdin'

# PATTERN: extract the binary value cleanly (strip the TOML-ish quoting the CLI prints)
/tmp/stagecoach_verify providers show cursor | grep prompt_delivery | sed -E "s/.*= '([^']*)'.*/\1/"   # → positional

# PATTERN: extract a provider's Delivery cell (column 2) from the docs table by provider name
# (table rows look like: | `opencode` | stdin | (none) | -m | ... )
awk -F'|' -v p='opencode' '
  $0 ~ "`"p"`" { gsub(/^ +| +$/,"",$2); gsub(/`/,"",$2); print $2 }
' docs/providers.md
# → stdin   (column 2 = the Delivery cell; gsub strips spaces and backticks)

# PATTERN: the residual-drift grep (must return zero)
grep -rn 'positional.*opencode\|opencode.*positional' docs/ README.md   # expect exit 1, no output

# PATTERN: the T2.S1 exhaustive parenthetical to look for (check b)
grep -n 'read-only constraint flags (codex, cursor, agy, qwen-code' docs/how-it-works.md   # expect 1 hit (line ~197)

# PATTERN: the chrome-bullet completeness check (check d)
grep -n 'Chrome is a separate axis' docs/providers.md   # locate the bullet (line ~100), then eyeball
# the 'providers that do not' group must list: codex, cursor, opencode, agy, qwen-code
```

### Integration Points

```yaml
NO code / database / migration / routes / config. Verification gate over docs + the built binary.

BINARY (rebuilt to /tmp/stagecoach_verify): the authoritative source for prompt_delivery; queried per
  provider via `providers show`. Never edited.
DOCS UNDER VERIFICATION:
  - docs/providers.md: check (a) Delivery column (rows ~80-86); check (d) chrome bullet (~line 100).
  - docs/how-it-works.md: check (b) safety paragraph (~line 197).
  - docs/README.md + README.md: check (c) residual-drift scan.
GATES: make test (green); markdownlint-cli2 (still 0 errors); scope diff (empty or single doc fix).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Markdown lint on the two primary docs (baseline = 0 errors on both)
npx --no-install markdownlint-cli2 docs/providers.md docs/how-it-works.md
# Expected: "Summary: 0 error(s)". If >0, a fix (T2.S1's or T3.S1's) introduced a markdown defect — fix it.

# No Go formatting is involved (no code change). Confirm no .go file changed by this subtask:
git diff --stat -- '*.go'      # Expected: empty
```

### Level 2: Unit Tests (Component Validation)

```bash
# No code change expected — run the suite as a no-regression sanity check (proves no accidental file touch)
make test                      # Expected: green (race detector). A docs-only gate cannot break Go tests.
```

### Level 3: Integration Testing (System Validation)

```bash
# THE GATE: rebuild the verification binary and cross-check all 7 providers' prompt_delivery.
go build -o /tmp/stagecoach_verify ./cmd/stagecoach   # clean, exit 0
for p in pi claude opencode codex cursor agy qwen-code; do
  echo "$p: $(/tmp/stagecoach_verify providers show "$p" | grep prompt_delivery)"
done
# Expected output (exactly):
#   pi:        prompt_delivery = 'stdin'
#   claude:    prompt_delivery = 'stdin'
#   opencode:  prompt_delivery = 'stdin'
#   codex:     prompt_delivery = 'stdin'
#   cursor:    prompt_delivery = 'positional'
#   agy:       prompt_delivery = 'stdin'
#   qwen-code: prompt_delivery = 'stdin'
# Then assert each value equals the docs/providers.md Delivery cell for that provider (column 2).
# Expected verdict: 7/7 match. (This reproduces architecture/system_context.md §'Full 7-provider
# cross-check' at the shell, post-fix — the core deliverable of check (a).)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Check (a) automated: binary value == docs Delivery cell for all 7 (exit non-zero on any mismatch)
for p in pi claude opencode codex cursor agy qwen-code; do
  bin=$(/tmp/stagecoach_verify providers show "$p" | grep prompt_delivery | sed -E "s/.*= '([^']*)'.*/\1/")
  doc=$(awk -F'|' -v p="$p" '$0 ~ "`"p"`" {gsub(/^ +| +$/,"",$2); gsub(/`/,"",$2); print $2}' docs/providers.md)
  printf '%-10s bin=%-10s doc=%-10s %s\n' "$p" "$bin" "$doc" "$([ "$bin" = "$doc" ] && echo OK || echo MISMATCH)"
done
# Expected: 7 lines, all ending 'OK'. Any 'MISMATCH' → Task 6 (fix the doc cell to the binary value).

# Check (c): residual opencode↔positional drift (expect ZERO)
grep -rn 'positional.*opencode\|opencode.*positional' docs/ README.md
echo "exit=$?"   # Expected: exit=1 (grep 'no matches'); zero lines printed.

# Check (b): T2.S1's exhaustive parenthetical is present
grep -n 'read-only constraint flags (codex, cursor, agy, qwen-code' docs/how-it-works.md   # expect 1 hit (~line 197)
grep -n 'explicit tool-disable flags (pi, claude)' docs/how-it-works.md                      # expect 1 hit (UNCHANGED)
grep -n 'read-only constraint flags (codex, cursor)\.' docs/how-it-works.md                  # expect ZERO (stale form gone)

# Check (d): chrome bullet names all 5 constrained providers
sed -n '/Chrome is a separate axis/,/^$/p' docs/providers.md | \
  grep -E 'codex.*cursor.*opencode.*agy.*qwen-code'   # expect 1 match (all 5 in the 'do not' group)

# Scope guard: this subtask changed NO code/PRD/TOML
git diff --stat -- '*.go' internal/ providers/ PRD.md docs/README.md
# Expected: empty (docs/README.md and README.md need NO change — verified zero opencode-delivery drift).

# Scope guard: this subtask's diff is EMPTY (all-pass) or a single doc file (fail-then-fix)
git diff --stat -- docs/
# Expected: either empty, OR exactly one of docs/providers.md / docs/how-it-works.md with a localized change.
```

## Final Validation Checklist

### Technical Validation
- [ ] `/tmp/stagecoach_verify` rebuilt cleanly (`go build -o /tmp/stagecoach_verify ./cmd/stagecoach`)
- [ ] `make test` green (no-regression sanity)
- [ ] `npx markdownlint-cli2 docs/providers.md docs/how-it-works.md` still 0 errors
- [ ] `git diff --stat -- '*.go' internal/ providers/ PRD.md` empty

### Feature Validation (the four checks)
- [ ] **(a)** 7/7 binary↔Delivery-column match (Level 4 loop prints all OK; cursor=positional, rest=stdin)
- [ ] **(b)** how-it-works line ~197 carries the exhaustive 5-provider parenthetical; `(pi, claude)` unchanged; partition = 7
- [ ] **(c)** `grep -rn 'positional.*opencode\|opencode.*positional' docs/ README.md` → zero matches (exit 1)
- [ ] **(d)** "Chrome is a separate axis" bullet names all 5 (codex, cursor, opencode, agy, qwen-code)
- [ ] cross-reference: how-it-works partition agrees with providers.md "Tool-disable approach" column

### Scope-Boundary Validation
- [ ] No code/test/PRD/TOML/.gitignore change
- [ ] No new files; no changelog note
- [ ] docs/providers.md line ~96 'Read-only constraint (codex, cursor)' parenthetical NOT expanded (out of scope)
- [ ] docs/README.md and README.md NOT edited (verified zero opencode-delivery drift — no fix needed)
- [ ] Diff is EMPTY (all-pass, the expected branch) OR a single localized doc fix (fail-then-fix)
- [ ] Did not re-open T1.S1/T2.S1 design decisions (exhaustive phrasing settled by T2.S1)

### Verification Reporting
- [ ] Recorded the 7×2 binary-vs-docs table (check a) in the work output
- [ ] Recorded the partition (check b) and the grep exit codes (checks c/d)
- [ ] If any check failed: recorded the failing check, the fix applied, and the re-verification result

---

## Anti-Patterns to Avoid

- ❌ Don't edit `internal/provider/builtin.go` (or any source) to "match the docs." The binary is authoritative (docs/README.md contract). If a check (a) mismatch appears, the DOC cell is wrong — fix the doc.
- ❌ Don't clobber the project's `./bin/stagecoach` with the verification build — use the contract's isolated path `/tmp/stagecoach_verify`.
- ❌ Don't expand the 'Read-only constraint (codex, cursor)' parenthetical on docs/providers.md line ~96. It is NOT a PRD-defined defect (Issue 2 = how-it-works line 197 ONLY). The full 5-provider set is already in the table column + the chrome bullet. Expanding it = scope creep and a surprise edit.
- ❌ Don't treat the all-pass (zero-diff) outcome as a failure. Mode B is a verification step; "no edits needed" is the EXPECTED and correct result when T1.S1+T2.S1 landed cleanly. Record the green checks and stop.
- ❌ Don't skip a check because "T1.S1/T2.S1 already fixed it." The whole point of this gate is to INDEPENDENTLY re-verify post-fix — the original drift was caused by a rewrite carrying stale cells forward; re-running the full 7-provider sweep guards against the same class of regression in adjacent rows.
- ❌ Don't widen the drift grep to flag the cursor row or the generic enum lines (providers.md:22/:64, README.md:308). 'positional' legitimately appears there (cursor IS positional; the enum lists all three delivery modes). The check (c) patterns require opencode ADJACENT to positional, which correctly excludes those.
- ❌ Don't confuse the Delivery column (column 2) with the Tool-disable (column 7) or Chrome-disable (column 8) columns when extracting doc cells for check (a). The 9-column table is wide; use the awk column-2 extractor.
- ❌ Don't change docs/README.md or README.md speculatively. They were scanned (check c) and have ZERO opencode-delivery drift (opencode appears only in provider-list prose). No edit is needed or warranted.
- ❌ Don't re-litigate the exhaustive-vs-exemplary phrasing for the how-it-works parenthetical — T2.S1 settled on the exhaustive form; check (b) verifies that exact form. If it's absent, apply T2.S1's exact substring (exhaustive), do not substitute the 'e.g.' alternative.

---

## Confidence Score: 10/10

This is a verification gate whose every expected value was DIRECTLY MEASURED in this research session:
the 7 binary `prompt_delivery` values (queried from a fresh build), the current docs/providers.md
Delivery cells (7/7 already match), the residual-drift grep result (zero), the chrome-bullet
completeness (all 5 present), and the markdownlint baseline (0 errors). The only dependency is T2.S1's
landing (its exact contract substring is quoted, so if it hasn't landed, Task 6 applies it verbatim).
The all-pass branch produces no diff — which is the correct, expected Mode-B outcome, explicitly called
out so the implementer does not mistake it for a failure. The four checks are deterministic, scripted
(Level 4), and have unambiguous pass/fail criteria. There is no code, no test, no design judgment
beyond "binary is authoritative; doc agrees."
