name: "P1.M2.T2.S1 — Correct the stale arbiter staging comment to reflect FR-M1d frozen-tree staging (BUG-012)"
description: >
  Single-comment documentation fix in internal/decompose/decompose.go (the runArbiterPhase doc block,
  ~L989-990). The stale comment claims the arbiter's STAGING is "UNCHANGED — it stages from the working tree",
  which was true pre-FR-M1d (v2.0–v2.1: only the diff was frozen) but is FALSE since v2.2's FR-M1d
  (spec/SPEC.md:14), which extended the freeze boundary into the arbiter: gate, diff, AND staging all derive
  from the frozen T_start via the OverlayTreePaths primitive. The CODE is already correct (resolveArbiter/
  resolveNewCommit/amendTip resolve via tStart/treePrime, not a live AddAll — verified). This is a comment-
  only catch-up: replace the two stale sentences with text referencing FR-M1b+FR-M1d, the frozen T_start, and
  the OverlayTreePaths primitive. NO code change, NO test change, NO behavioral delta, NO user-facing docs.

---

## Goal

**Feature Goal**: Make the `runArbiterPhase` doc comment accurately describe the arbiter's freeze boundary —
staging derives from the frozen `T_start` via `OverlayTreePaths` (FR-M1d), NOT the live working tree — so the
comment stops contradicting both the shipped code and the spec.

**Deliverable**: ONE edit to `internal/decompose/decompose.go` — replace the two stale comment lines
(~L989-990: "… STAGING … is UNCHANGED — it stages from the working tree (== T_start's source under the
invariant); the freeze OUTPUT guarantee for staging is P3.M2.T1.S1.") with a corrected block that references
FR-M1b + FR-M1d, the frozen `T_start`, and the `OverlayTreePaths(treePrime := tStart)` staging path.

**Success Definition**:
- The stale phrase "stages from the working tree" / "UNCHANGED" is gone from the runArbiterPhase doc block.
- The corrected comment references **FR-M1d** and the **OverlayTreePaths** primitive, and states staging
  derives from the frozen `T_start` (gate + diff + staging all frozen).
- The preserved FR-M1b sentence above it (the leftover-diff-frozen line) is unchanged and still accurate.
- `go build ./...`, `go vet ./internal/decompose/...`, `make test` all green; `gofmt -l` empty.
- `git diff --name-only` == exactly `internal/decompose/decompose.go` (comment-only — no `.orig`, no other file).

## User Persona (if applicable)

**Target User**: A future contributor reading `runArbiterPhase` to understand the arbiter's freeze invariant
before changing staging/diff/gate logic.

**Use Case**: The contributor reads the doc block to learn whether the arbiter stages from the live tree or the
frozen snapshot. The corrected comment tells them staging is frozen (FR-M1d) and points at the
`OverlayTreePaths` primitive — so they don't "fix" frozen staging by reverting to a live `AddAll` (which would
reintroduce the FR-M1b loophole FR-M1d closed).

**User Journey**: contributor opens decompose.go → reads runArbiterPhase doc → sees "FR-M1d: staging derives
from T_start via OverlayTreePaths" → trusts the freeze is intentional → preserves it.

**Pain Points Addressed**: BUG-012 — a stale comment that actively misleads (it claims the arbiter reads the
live working tree, contradicting both FR-M1d and the shipped code). A misleading comment is worse than none;
it invites a regression.

## Why

- **FR-M1d / §13.6.5 / spec v2.2**: the freeze boundary was extended into the arbiter precisely to close the
  loophole where a post-`T_start` working-tree change was silently swept into an arbiter commit. The code
  implements this; the comment predates it. This task brings the comment into line.
- **Docs-as-contract hygiene**: the runArbiterPhase doc block is the in-source specification of the arbiter
  phase. A comment that says "stages from the working tree" while the code stages from `tStart` is a
  contradiction that erodes trust in the surrounding (correct) comments.
- **Bounded scope**: 2 stale comment lines → a corrected block. No code, no tests, no schema, no migration.
  The arbiter's frozen-staging behavior is already shipped and (per BUG-012's severity = MINOR) already
  functionally correct.

## What

**User-visible behavior**: None (a code comment). No runtime change.

**Technical change**: one comment replacement in the `runArbiterPhase` doc block. See the Blueprint for the
verbatim before/after.

### Success Criteria
- [ ] The stale "stages from the working tree" / "UNCHANGED" / "P3.M2.T1.S1" phrasing is removed from the runArbiterPhase doc block
- [ ] The corrected comment cites **FR-M1d** and the **OverlayTreePaths** primitive
- [ ] The corrected comment states staging derives from the frozen `T_start` (gate+diff+staging all frozen)
- [ ] The FR-M1b leftover-diff sentence above the edit is preserved verbatim
- [ ] `go build ./...` + `go vet ./internal/decompose/...` + `make test` green; `gofmt -l` empty
- [ ] `git diff --name-only` == `internal/decompose/decompose.go` only

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact stale text (2 lines, with line numbers), the verbatim replacement text, the FR-M1d spec
clause to mirror, the proof the code is already correct (so NO code edit — comment only), and the scope fence.

### Documentation & References

```yaml
# MUST READ — the authoritative research (verbatim before/after + the code-is-correct proof)
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M2T2S1/research/findings.md
  why: "§0 the exact stale lines; §1 grep proof the arbiter stages from tStart/treePrime (code correct ⇒
        comment-only fix); §2 the FR-M1d spec clause; §3 the verbatim replacement text; §4-5 scope + validation."
  critical: "§1: do NOT change resolveArbiter/resolveNewCommit/amendTip — they already stage from the frozen
             tree (runArbiterPhase gets tStart at L278; treePrime := tStart at L416). This is a COMMENT fix."

# MUST READ — the bug definition
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/architecture/bugfix_subsystems.md
  why: "§BUG-012 names the stale comment, its location (runArbiterPhase doc block), and that the code is
        correct (the comment contradicts FR-M1d, not the implementation)."
  critical: "BUG-012 severity = MINOR; it is a documentation/comment fix, NOT a code fix."

# MUST READ — the FR-M1d spec (the wording the corrected comment mirrors)
- file: spec/SPEC.md
  why: "Line 14 (v2.2 revision row): 'FR-M1d extends the freeze boundary into the arbiter: gate, diff, and
        staging all derive from T_start; concurrent changes are left untouched in the working tree. Adds one
        git primitive (OverlayTreePaths, §13.6.5).' — the canonical statement the comment must reflect."
  gotcha: "READ-ONLY (spec is human-owned; never edit). Mirror its wording; do not paraphrase the requirement away."

# MUST EDIT — the file + the exact stale comment
- file: internal/decompose/decompose.go
  why: "The runArbiterPhase doc block (~L985-996). The stale two lines are ~L989-990. Locate by content:
        grep -n 'stages from the working tree' internal/decompose/decompose.go (one hit)."
  pattern: "The block is a godoc-style // comment above func runArbiterPhase. Preserve the FR-M1b line above
            the edit (the leftover-diff-is-frozen line) and the rereadFinalCommits paragraph below it."
  gotcha: "Lines drift on sibling edits. Anchor by STRING (grep 'stages from the working tree'), not line
           number. The replacement is 4 // lines in place of 2 // lines (a net +2 line comment expansion)."

# CONFIRMING — the code is correct (proof this is comment-only; do NOT edit these)
- file: internal/decompose/decompose.go   # L278 runArbiterPhase(... tStart ...) ; L416 treePrime := tStart
  why: "Confirms the arbiter stages from the frozen T_start (via treePrime/OverlayTreePaths), NOT a live
        AddAll — so the corrected comment describes real behavior, and no code change is needed."
  critical: "READ-ONLY. Do not touch resolveArbiter/resolveNewCommit/amendTip/runArbiterPhase bodies."

# CONTEXT — the parallel sibling (no overlap; different concern)
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M2T1S3/PRP.md
  why: "Parallel sibling extends FirstTooledProvider to consider user-defined tooled providers (BUG-003 stager
        fast-path). It edits resolveRoles/FirstTooledProvider paths, NOT the runArbiterPhase doc block. No
        overlap; no conflict (different functions, different concerns)."
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  decompose.go   # EDIT — runArbiterPhase doc block (~L989-990): replace the 2 stale staging-comment lines
spec/SPEC.md     # READ-ONLY — L14 the FR-M1d spec clause (the source of truth the comment mirrors)
# Makefile, go.mod, tests — READ-ONLY (comment-only change touches nothing else)
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
# MODIFIED (no new files):
internal/decompose/decompose.go   # runArbiterPhase doc block: stale "stages from working tree" → FR-M1d frozen-tree staging
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (this is a COMMENT-ONLY fix — the code is already correct): the arbiter already stages from the
//   frozen T_start (runArbiterPhase gets tStart at L278; runSingleShortcut sets treePrime := tStart at L416;
//   resolveArbiter/resolveNewCommit/amendTip resolve via the frozen tree). Do NOT edit any function body.
//   Editing the code would be a fabricated behavioral change for a MINOR docs bug — out of scope and risky.

// CRITICAL (anchor by STRING, not line number): the runArbiterPhase doc block sits ~L985-996 but lines drift
//   on any sibling edit (the parallel P1.M2.T1.S3 may touch nearby resolveRoles code). Locate the stale text
//   via `grep -n 'stages from the working tree' internal/decompose/decompose.go` (exactly one hit) and
//   replace those two // lines.

// GOTCHA (preserve the surrounding doc block): the FR-M1b line ABOVE the edit ("the leftover diff is FROZEN —
//   TreeDiff(tipTree, tStart), not a live WorkingTreeDiff.") is still accurate — keep it verbatim. The
//   rereadFinalCommits paragraph BELOW the edit is unrelated — keep it verbatim. Only the two stale staging
//   lines change.

// GOTCHA (the P3.M2.T1.S1 forward-reference is moot): the stale comment defers the staging freeze to a future
//   "P3.M2.T1.S1" subtask. That freeze LANDED (FR-M1d, v2.2). Drop the forward-reference; state the freeze as
//   a shipped fact (FR-M1d), not a TODO.

// GOTCHA (godoc formatting): keep the replacement as consecutive `// ` lines (the block is a godoc-style
//   comment above func runArbiterPhase). No blank-comment lines mid-block (gofmt is fine with them but the
//   surrounding style is a contiguous // block). Run gofmt -w to be safe.
```

## Implementation Blueprint

### Data models and structure
None. Pure comment edit. No code, no types.

### Implementation Tasks (ordered — one edit)

```yaml
Task 1: EDIT internal/decompose/decompose.go — correct the stale arbiter staging comment
  - LOCATE: grep -n 'stages from the working tree' internal/decompose/decompose.go   (one hit, ~L989-990).
    The stale two lines (inside the runArbiterPhase godoc block):
      // The arbiter's STAGING (resolveArbiter via AddAll/Add) is UNCHANGED — it stages from the working tree
      // (== T_start's source under the invariant); the freeze OUTPUT guarantee for staging is P3.M2.T1.S1.
  - REPLACE those two lines with (verbatim):
      // FR-M1b/FR-M1d: the arbiter's gate, diff, AND staging all derive from the FROZEN T_start —
      // TreeDiff(tipTree, tStart) for the leftover diff; resolveArbiter/resolveNewCommit/amendTip stage via
      // OverlayTreePaths(treePrime := tStart), never the live working tree. A change written to the working
      // tree after T_start capture is left untouched (not swept into an arbiter commit).
  - PRESERVE: the FR-M1b line ABOVE ("the leftover diff is FROZEN — TreeDiff(tipTree, tStart), not a live
    WorkingTreeDiff.") verbatim; the rereadFinalCommits paragraph BELOW verbatim; func runArbiterPhase body
    UNCHANGED (the code is correct).
  - VERIFY the block is still a contiguous // comment (gofmt -w if needed).

Task 2: VERIFY — build, vet, format, tests, grep guards
  - go build ./... ; go vet ./internal/decompose/...
  - gofmt -l internal/decompose/decompose.go   # empty
  - make test                                   # green (no behavioral change)
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the corrected comment mirrors the FR-M1d spec clause + names the shipped primitive
//   (spec/SPEC.md:14): "gate, diff, and staging all derive from T_start … OverlayTreePaths".
//   The comment names treePrime := tStart (the variable the code uses at L416) so a reader can grep straight
//   to the implementation. It drops the moot P3.M2.T1.S1 forward-reference and the false "UNCHANGED" claim.

// PATTERN: comment-only — the function bodies (resolveArbiter/resolveNewCommit/amendTip/runArbiterPhase) are
//   untouched. The corrected comment DESCRIBES existing correct behavior; it does not change it.
```

### Integration Points

```yaml
NO code / tests / schema / config / routes / new deps / migration. ONE comment edit in ONE file.

DOCS (internal/decompose/decompose.go):
  - runArbiterPhase godoc block: stale "stages from working tree" → FR-M1d frozen-tree staging via OverlayTreePaths.

SCOPE FENCES: NO function-body edit (the arbiter staging is correct); NO test change; NO user-facing docs
  (README/how-it-works — BUG-012 is an in-source comment); NO PRD.md / tasks.json / prd_snapshot.md / spec.
  No overlap with the parallel P1.M2.T1.S3 (FirstTooledProvider/resolveRoles — different functions).
```

## Validation Loop

> A comment-only edit cannot break Go semantics. Validation = build/vet/format/tests stay green + grep guards
> proving the stale text is gone and FR-M1d/OverlayTreePaths are present.

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build + vet (a comment change must still compile/vet — guards against an accidental code touch).
go build ./...
go vet ./internal/decompose/...
# Expected: clean.

# Format.
gofmt -l internal/decompose/decompose.go
# Expected: empty. If listed: gofmt -w internal/decompose/decompose.go.

# Scope guard: only decompose.go changed (comment-only — no .orig, no other file).
git diff --name-only
# Expected: internal/decompose/decompose.go (only).
git diff --stat -- internal/decompose/decompose.go
# Expected: a small insert/delete (net +2 lines) in the comment block ONLY — no function-body hunks.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The decompose suite must stay green (the arbiter staging behavior is unchanged — this is a comment fix).
go test ./internal/decompose/... -v
# Expected: green (incl. the freeze-invariant / arbiter tests). A failure means the edit strayed into code.

# Full race suite (sanity — the tree is otherwise clean).
make test
# Expected: green (race detector).
```

### Level 3: Integration Testing (System Validation)

```bash
# There is no integration surface for a comment-only edit. The decompose freeze/arbiter e2e coverage (if any,
# under the e2e build tag) already passes against the correct code; the comment change doesn't alter behavior.

# Sanity: the binary still builds (no downstream compile break from the edit).
go build ./...
# Expected: succeeds.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1 (THE gate): the stale phrase is GONE.
grep -n 'stages from the working tree' internal/decompose/decompose.go
# Expected: empty (no output).

# Grep guard 2: the stale "UNCHANGED" + "P3.M2.T1.S1" forward-reference are gone from the arbiter block.
grep -n 'UNCHANGED.*stages\|P3.M2.T1.S1' internal/decompose/decompose.go
# Expected: empty.

# Grep guard 3: the corrected comment cites FR-M1d + OverlayTreePaths + the frozen T_start.
grep -n 'FR-M1d' internal/decompose/decompose.go
grep -n 'OverlayTreePaths' internal/decompose/decompose.go
# Expected: each ≥1 hit in the runArbiterPhase doc block (the new comment lines).

# Grep guard 4: the preserved FR-M1b line (the leftover-diff-is-frozen line) is intact.
grep -n 'leftover diff is FROZEN' internal/decompose/decompose.go
# Expected: 1 hit (unchanged).

# Grep guard 5 (scope — comment-only, no code touched): the diff has NO non-comment hunks.
git diff internal/decompose/decompose.go | grep -E '^[+-]' | grep -vE '^[+-]//|^[++-]\s*//|^\+\+\+|^---'
# Expected: empty (every changed line is a // comment line). Any non-// hunk = an accidental code edit.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `go vet ./internal/decompose/...` clean
- [ ] `gofmt -l internal/decompose/decompose.go` empty
- [ ] `make test` green (no behavioral change — the arbiter staging was already frozen)
- [ ] `git diff` is comment-only (every `+`/`-` line starts with `//`)

### Feature Validation
- [ ] the stale "stages from the working tree" / "UNCHANGED" / "P3.M2.T1.S1" phrasing is removed
- [ ] the corrected comment cites **FR-M1d** and the **OverlayTreePaths** primitive
- [ ] the corrected comment states staging derives from the frozen `T_start` (gate+diff+staging all frozen)
- [ ] the FR-M1b leftover-diff line above the edit is preserved verbatim

### Scope-Boundary Validation
- [ ] `git diff --name-only` == `internal/decompose/decompose.go` only
- [ ] NO function-body edit (resolveArbiter/resolveNewCommit/amendTip/runArbiterPhase unchanged)
- [ ] NO test change; NO user-facing docs (README/how-it-works); NO PRD/spec/tasks.json
- [ ] No overlap with the parallel P1.M2.T1.S3 (FirstTooledProvider/resolveRoles)

### Code Quality & Docs
- [ ] the corrected comment mirrors the FR-M1d spec clause (spec/SPEC.md:14) + names the shipped primitive
- [ ] names `treePrime := tStart` so a reader can grep to the implementation
- [ ] the doc block remains a contiguous godoc-style `// ` comment

---

## Anti-Patterns to Avoid

- ❌ Don't edit the function bodies. The arbiter ALREADY stages from the frozen `T_start` (runArbiterPhase gets
  `tStart` at L278; `treePrime := tStart` at L416). BUG-012 is a comment/docs fix — the code is correct.
  Editing resolveArbiter/resolveNewCommit/amendTip would be a fabricated behavioral change for a MINOR docs
  bug, and would risk regressing the shipped FR-M1d freeze.
- ❌ Don't anchor to line number 989/990. Lines drift on sibling edits (the parallel P1.M2.T1.S3 may touch
  nearby resolveRoles code). Locate the stale text via `grep -n 'stages from the working tree'` (one hit) and
  replace those exact two `//` lines.
- ❌ Don't drop or alter the FR-M1b line above the edit. "the leftover diff is FROZEN — TreeDiff(tipTree,
  tStart), not a live WorkingTreeDiff." is still accurate (the diff was always frozen; FR-M1d added STAGING
  to the freeze). Preserve it verbatim. Only the two stale staging lines change.
- ❌ Don't keep the "P3.M2.T1.S1" forward-reference. That subtask LANDED (it IS FR-M1d, v2.2). State the
  staging freeze as a shipped fact ("FR-M1d: … derives from T_start"), not a TODO ("the freeze OUTPUT
  guarantee for staging is P3.M2.T1.S1"). A stale TODO invites confusion about whether the freeze shipped.
- ❌ Don't paraphrase the spec away. Mirror spec/SPEC.md:14's wording ("gate, diff, and staging all derive
  from T_start … OverlayTreePaths"). The comment is the in-source spec for the arbiter phase; weakening it
  ("stages against a snapshot", etc.) loses the FR-M1d/OverlayTreePaths traceability a contributor needs.
- ❌ Don't touch user-facing docs. BUG-012 is an in-source comment fix; the README/how-it-works accuracy sweep
  is P1.M2.T3.S1 (a separate task). This task edits exactly one code comment.
- ❌ Don't add a blank `//` line mid-block or reformat the surrounding doc. The block is a contiguous godoc
  comment; the replacement is 4 `//` lines in place of 2 (a clean net +2). Run `gofmt -w` to be safe.

---

## Confidence Score: 10/10

This is a single comment replacement in one file, with the exact stale text (2 lines), the verbatim
replacement (4 lines mirroring spec/SPEC.md:14's FR-M1d clause + naming the shipped OverlayTreePaths/
treePrime primitive), grep proof the code is already correct (so NO code edit — comment only), the explicit
scope fences (no function-body/test/user-doc/PRD/spec change), and a grep guard proving the diff is
comment-only. The one thing that could go wrong — accidentally editing a function body — is caught by the
Level-4 "every diff line is a // line" guard and the green decompose test suite. No new pattern, no new type,
no new dependency. One-pass success is essentially guaranteed.