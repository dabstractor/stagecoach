name: "P2.M1.T3.S1 — Improve verifyFreezeSubset errors: concept title + clearer phrasing + remedy (amended FR-M1c, §9.14)"
description: >
  A surgical, message-only refactor of `verifyFreezeSubset` in `internal/decompose/stager.go`. The
  function's two freeze-violation `fmt.Errorf` calls (message (A) PATH at L173–176, message (B)
  CONTENT at L193–196) are rewritten VERBATIM per the item description so each error (1) names the
  concept by TITLE via a new `conceptTitle string` parameter (rendered with `%q`), (2) replaces the
  opaque "not present in T_start" / "not traceable to T_start" phrasing with the clear "frozen
  working-tree snapshot" wording, and (3) carries a remedy/explanation suffix ("This indicates
  concurrent working-tree changes were picked up by the stager. Aborting to protect the freeze
  boundary."). Concretely: (a) change the signature at stager.go:159 to insert `conceptTitle string`
  AFTER `i int` and BEFORE `treeI string`; (b) rewrite the two `fmt.Errorf` calls VERBATIM using
  `%w ` (SPACE after %w — NOT the current `%w: ` colon) so the render exactly matches the item's
  literal messages while preserving `errors.Is(err, ErrFreezeViolation)`; (c) update the ONE
  production call site at decompose.go:583 (inside `runLoop`) to pass `concepts[i].Title`;
  (d) update the doc-comment (stager.go:141–157) to mention the concept is named by title;
  (e) update the tests: add the new arg to ALL FOUR direct `verifyFreezeSubset` calls in
  stager_test.go (L370/406/452/483), update the 3 old-phrasing substring assertions
  (stager_test.go:416–417 + 462–463; decompose_test.go:773–774) to the new phrasing while keeping the
  "paths are named" checks, and optionally assert the concept title now appears. NO control-flow,
  git, or config change. NO new files. NO docs-surface change. The authoritative message text is the
  ITEM DESCRIPTION — it SUPERSEDES the older `delta_prd.md:120` append-remedy suggestion. Validates
  via `go build ./...`, `go test ./internal/decompose/... -run 'FreezeSubset|StagerFreezeViolation'
  -race -v`, `go test -race ./...`, `gofmt -l`, `go vet`.

---

## Goal

**Feature Goal**: A freeze-violation error (FR-M1c) that fires during decomposition is now clear,
actionable, and identifies the concept BY TITLE — instead of the current opaque
`freeze violation: concept 0 staged content not traceable to T_start: foo.go` that names only a
numeric index, uses jargon ("not traceable to T_start"), and offers no explanation or remedy.

**Deliverable**: Three edited files — `internal/decompose/stager.go` (signature + 2 message rewrites
+ doc-comment), `internal/decompose/decompose.go` (1 call-site arg), and the two test files
`internal/decompose/stager_test.go` + `internal/decompose/decompose_test.go` (call-site args +
substring-assertion updates). No new files, no new exports, no behavior change to the freeze-check
logic itself.

**Success Definition**:
- A path-check freeze violation renders EXACTLY: `decompose: freeze violation in concept 0 ("c1"):
  staged paths not in the frozen working-tree snapshot: sentinel.txt. This indicates concurrent
  working-tree changes were picked up by the stager. Aborting to protect the freeze boundary.`
- A content-check freeze violation renders EXACTLY: `decompose: freeze violation in concept 0
  ("c1"): staged content differs from the frozen working-tree snapshot for: foo.go. This indicates
  concurrent working-tree changes were picked up by the stager. Aborting to protect the freeze
  boundary.`
- `errors.Is(err, ErrFreezeViolation)` is STILL `true` for both (the `%w` wrap is preserved).
- The offending paths are STILL named (the `%s` join is retained).
- All four `verifyFreezeSubset` unit tests + `TestDecompose_StagerFreezeViolation` pass; `go build
  ./...` + `go test -race ./...` + `gofmt -l` + `go vet` clean.

## User Persona (if applicable)

**Target User**: A Stagecoach end user (or maintainer debugging a real freeze violation) reading the
error after a decompose run aborts because a concurrent tool/editor dirtied the working tree and the
external stager swept it in.
**Use Case**: When FR-M1c fires, the user must understand (a) WHICH concept tripped it (by title),
(b) WHICH paths are offending, and (c) WHY it aborted and what it means — so they can decide whether
to re-run from a clean tree. The old message gave only a numeric index + the opaque "not traceable to
T_start".
**User Journey**: decompose aborts → user reads the error → sees the concept TITLE + the named paths
+ the plain-language cause ("concurrent working-tree changes were picked up by the stager") → knows
the run was protected and can re-run.
**Pain Points Addressed**: Opaque phrasing ("not traceable to T_start"), concept identified by bare
numeric index, no explanation of cause or that the abort is intentional protection.

## Why

- **Clarity over jargon**: `research_decompose_freeze.md` §4 (architecture) confirms the offending
  paths are ALREADY named in both branches — so the improvement is NOT "add path names" (they exist)
  but replace the opaque "not traceable to T_start" with plain language and surface the concept title.
- **Identify the concept by title, not index**: `verifyFreezeSubset` receives only `i int`; the call
  site (`runLoop`) has `concepts[i].Title` (`prompt.PlannerCommit.Title`, planner.go:84). Threading
  the title through makes the error immediately tell the user which logical commit was affected.
- **Explain + reassure**: the new suffix tells the user the abort was intentional freeze-boundary
  protection (concurrent changes were excluded) — not a mysterious internal failure.
- **Amended FR-M1c**: PRD §9.14 FR-M1c (the freeze-enforcement defense-in-depth) requires the hard
  error to be clear and actionable; this PRP is the error-quality half of that amendment (P2.M1.T2.S1
  is the empty-index re-assertion half; P2.M2.T1.S1 will add e2e coverage).

## What

Rewrite the two `fmt.Errorf` calls in `verifyFreezeSubset` (`internal/decompose/stager.go`), change
its signature to accept `conceptTitle string`, update its single production caller in `runLoop`
(`internal/decompose/decompose.go:583`) to pass `concepts[i].Title`, refresh the doc-comment, and
update the tests that pin the old phrasing. No git/control-flow/logic change.

### Success Criteria
- [ ] `verifyFreezeSubset` signature is `(... tStartPaths []string, i int, conceptTitle string, treeI string) error` (title AFTER `i`, BEFORE `treeI`).
- [ ] The ONE production call site (decompose.go:583) passes `concepts[i].Title` as the title arg.
- [ ] Message (A) renders the item's LITERAL text (see §"Implementation Patterns") and still wraps `ErrFreezeViolation` via `%w` (space, not colon).
- [ ] Message (B) renders the item's LITERAL text and still wraps `ErrFreezeViolation` via `%w` (space, not colon).
- [ ] `errors.Is(err, ErrFreezeViolation)` is `true` for both (Touched-test assertion holds).
- [ ] The offending paths remain named (the `%s` join is kept; the "sentinel.txt"/"a.txt" assertions still pass unchanged).
- [ ] The doc-comment notes the concept is named by title.
- [ ] All four `verifyFreezeSubset` test calls (stager_test.go L370/406/452/483) pass the new title arg.
- [ ] The 3 old-phrasing substring assertions are updated to the new phrasing (stager_test.go:416–417, 462–463; decompose_test.go:773–774).
- [ ] `go build ./...`, `go test -race ./...`, `gofmt -l <the 4 files>`, `go vet ./internal/decompose/...` all clean.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact signature change, the exact verbatim new message strings, the CRITICAL `%w`-space
(not `%w:`-colon) format-string gotcha, the single call site + the loop variable provenance, the
complete inventory of test edits (4 call-site args + 3 substring updates + which assertions stay
valid), the no-new-imports fact, and the verified validation commands.

### Documentation & References

```yaml
# MUST READ — the complete verified analysis for THIS item (signatures, line numbers, the %w-space
# gotcha, the full test inventory, the delta_prd.md divergence note, scope).
- docfile: plan/015_b461e4720495/P2M1T3S1/research/findings.md
  why: "§0 the authoritative contract = ITEM DESCRIPTION (supersedes delta_prd.md:120); §1 current
        signatures + both messages + line numbers; §2 the CRITICAL %w-SPACE (not %w: colon) format
        gotcha that makes the render match the item's literal text while preserving errors.Is; §3 new
        signature + call site + loop var provenance; §4 the VERBATIM new messages with exact arg order;
        §5 doc-comment update; §6 the COMPLETE test-edit inventory (4 call-site args + 3 substring
        updates + what stays valid + optional title assertions); §7 scope; §9 validation commands."

# MUST READ — the function to edit (signature, both messages, doc-comment, the sentinel).
- file: internal/decompose/stager.go
  why: "L60 `var ErrFreezeViolation = errors.New(\"decompose: freeze violation\")` — its .Error()
        string IS the message prefix, which is why %w must be followed by a SPACE (§2 of findings).
        L141–157 the doc-comment to refresh. L159 the signature to extend with `conceptTitle string`.
        L173–176 message (A) to rewrite. L193–196 message (B) to rewrite. L165 + L183 the two
        ErrDecomposeFailed DiffTreeNames wraps — DO NOT TOUCH (out of scope)."
  pattern: "Every freeze-violation return is `return fmt.Errorf(\"%w: ...\", ErrFreezeViolation, i,
            strings.Join(...))`. The new ones change `%w: ` → `%w ` and add `conceptTitle` + the
            remedy suffix — but keep the `%w` wrap + `strings.Join` path-naming."
  gotcha: "Use `%w in concept %d (%q):` (SPACE after %w). If you keep the current `%w: ` (colon) the
           render becomes `decompose: freeze violation: in concept ...` — a stray double-prefix that
           does NOT match the item's literal message. The sentinel string already ends at 'freeze
           violation'; the item's message continues with ' in concept'."

# MUST READ — the single production call site + the loop it sits in.
- file: internal/decompose/decompose.go
  why: "L433 `func runLoop(... concepts []prompt.PlannerCommit, ...)` — receives concepts. L567
        `for i, concept := range concepts {` — both `i` and `concept` in scope. L583 the call:
        `if vErr := verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, i, treeI); vErr != nil {`
        → add `concepts[i].Title` between `i` and `treeI`. L419–448 + L575–585 the FR-M1c comment
        context (do not need to change the comment, but it documents the violation handling)."
  pattern: "The title is available as `concepts[i].Title` (item spec) — equivalent to the range var
            `concept.Title`; use `concepts[i].Title` to match the contract verbatim."
  gotcha: "`concepts[i]` is safe here — `i` is the current loop index over `concepts` (range, no
           shadowing at this line)."

# MUST READ — PlannerCommit.Title (the field threaded through).
- file: internal/prompt/planner.go
  why: "L83–87 `type PlannerCommit struct { Title string \`json:\"title\"\`; ... }`. Confirms the
        field exists and is the right one. READ ONLY — do not edit."

# MUST READ — the architecture research that motivated the new phrasing (provenance).
- docfile: plan/015_b461e4720495/architecture/research_decompose_freeze.md
  section: "§4 'Current freeze-violation error messages' — the old/new message table + the finding
            that paths are ALREADY named (so the work is phrasing + title + remedy, not path-naming)."
  why: "Establishes the design intent: replace 'not traceable to T_start' → 'content differs from the
        frozen working-tree snapshot' and surface the concept title. Fully consistent with the item
        description."

# MUST READ — the test files to edit (call sites + substring assertions).
- file: internal/decompose/stager_test.go
  why: "L370 (Happy), L406 (PathViolation), L452 (ContentViolation), L483 (EmptyStaging) — the FOUR
        direct `verifyFreezeSubset(...)` calls that EACH need a title arg added between `0` and `treeI`.
        L413–414 (sentinel.txt — KEEP), L416–417 (not present in T_start → UPDATE to new phrasing),
        L460–461 (a.txt — KEEP), L462–463 (not traceable to T_start → UPDATE). Imports `strings` +
        `errors` already present (L3–16)."
  pattern: "Call shape after edit: `verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, 0,
            \"test-concept\", treeI)`. Assertion shape after edit:
            `strings.Contains(err.Error(), \"staged paths not in the frozen working-tree snapshot\")`."
  gotcha: "ALL FOUR call sites must get the new arg or the package will not compile — not just the two
           the item's prose names. (The item says 'two tests' by approx line number; the complete set
           is 4 call sites + 3 substring updates — see findings §6.)"
- file: internal/decompose/decompose_test.go
  why: "L750 planner JSON `{\"title\":\"c1\",...}` (the concept title for the violation). L770 (sentinel.txt — KEEP).
        L773–774 (not present in T_start → UPDATE to new phrasing). L767 errors.Is(err, ErrFreezeViolation) — KEEP."

# CONTEXT — the PRD provenance (read-only).
- docfile: plan/015_b461e4720495/prd_snapshot.md
  section: "§9.14 FR-M1c (freeze enforcement: 'Any staged path or content not traceable to T_start is
            a hard error') — amended in v2.8 to name offending paths + remedy. §13.6.1 (the freeze
            invariant)."
  why: "Establishes WHY the error exists and why clarity/actionability matter."
```

### Current Codebase tree (relevant slice)

```bash
# EDIT targets:
internal/decompose/stager.go          # EDIT — signature L159 + msg(A) L173–176 + msg(B) L193–196 + doc-comment L141–157
internal/decompose/decompose.go       # EDIT — call site L583 (+concepts[i].Title)
internal/decompose/stager_test.go     # EDIT — 4 call-site args (L370/406/452/483) + 2 substring updates (L416–417,462–463) + optional title asserts
internal/decompose/decompose_test.go  # EDIT — 1 substring update (L773–774) + optional title assert

# READ-ONLY references:
internal/prompt/planner.go            # PlannerCommit.Title (L84) — already exists, no edit
plan/015_b461e4720495/architecture/research_decompose_freeze.md  # §4 old/new message table (provenance)
Makefile                              # test / lint targets
```

### Desired Codebase tree with files to be added/edited

```bash
internal/decompose/stager.go          # EDIT — +conceptTitle param + verbatim message rewrites + doc-comment
internal/decompose/decompose.go       # EDIT — +concepts[i].Title at the call site (L583)
internal/decompose/stager_test.go     # EDIT — +title args at 4 call sites + 2 updated substring asserts
internal/decompose/decompose_test.go  # EDIT — +1 updated substring assert (+optional title assert)
# (no new files)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the %w-SPACE gotcha — this is THE thing that makes the render correct):
// ErrFreezeViolation.Error() == "decompose: freeze violation". The item's literal message (A) is
//   "decompose: freeze violation in concept %d (%q): ..."
// so the "%w" verb (which inserts the sentinel's string) MUST be followed by a SPACE, not a colon:
//   fmt.Errorf("%w in concept %d (%q): ...", ErrFreezeViolation, i, conceptTitle, paths)
// ❌ fmt.Errorf("%w: in concept ...", ...)  → renders "decompose: freeze violation: in concept ..." (WRONG)
// ✅ fmt.Errorf("%w in concept ...", ...)   → renders "decompose: freeze violation in concept ..." (EXACT)
// The current code uses "%w: " (colon) everywhere; the new messages deliberately use "%w " (space).

// CRITICAL (preserve errors.Is): BOTH messages MUST keep the "%w" wrap of ErrFreezeViolation. Tests
// assert errors.Is(err, ErrFreezeViolation) (stager_test.go:410,456; decompose_test.go:767). Dropping
// %w (e.g. a plain fmt.Errorf with the prefix as a literal) would break those assertions.

// CRITICAL (the new title arg slots BETWEEN i and treeI): the item says "after the `i int` parameter".
// treeI is the terminal param today, so conceptTitle goes between `i int` and `treeI string`:
//   (... tStartPaths []string, i int, conceptTitle string, treeI string)
// ALL callers (1 prod + 4 test) must pass it in that position or fail to compile.

// GOTCHA (ALL FOUR test call sites need the new arg, not two): grep shows stager_test.go L370/406/452/
// 483 EACH call verifyFreezeSubset directly. Every one needs a title literal (e.g. "test-concept")
// added between `0` and `treeI`. The item's "two tests" framing is by approx line number; the real
// compile-correct set is 4 call sites + 3 substring-assertion updates.

// GOTCHA (keep these assertions UNCHANGED — the new messages still name the paths): "sentinel.txt"
// (stager_test.go:413, decompose_test.go:770), "a.txt" (stager_test.go:460). The %s join is retained.

// GOTCHA (no new imports): stager.go imports context/errors/fmt/strings (already present); both test
// files import strings + errors (verified). Do NOT touch any import block.

// GOTCHA (the DiffTreeNames ErrDecomposeFailed wraps at L165/L183 use `i` and are OUT OF SCOPE): they
// are infra errors ("freeze check diff-tree-names[%d]"), not user-facing freeze violations. Leave them
// exactly as-is; do NOT add conceptTitle or change their text.

// GOTCHA (delta_prd.md:120 diverges — IGNORE it): that older draft suggested appending "; unstage the
// offending path(s)..." as the remedy. The ITEM DESCRIPTION supersedes it with the full verbatim
// rewrite (concurrent-working-tree-changes remedy). Implement the item's literal messages only.
```

## Implementation Blueprint

### Data models and structure

None — no structs/schemas. The "data" is: one new `string` parameter, two rewritten `fmt.Errorf`
format strings (verbatim), one call-site argument, a doc-comment refresh, and test edits.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/decompose/stager.go — extend the signature + rewrite BOTH messages + refresh the doc-comment
  - SIGNATURE (L159): change
        func verifyFreezeSubset(ctx context.Context, deps Deps, baseTree, tStart string, tStartPaths []string, i int, treeI string) error
    to (insert `conceptTitle string` AFTER `i int`, BEFORE `treeI string`):
        func verifyFreezeSubset(ctx context.Context, deps Deps, baseTree, tStart string, tStartPaths []string, i int, conceptTitle string, treeI string) error
  - MESSAGE (A) PATH (L173–176): replace
        return fmt.Errorf("%w: concept %d staged paths not present in T_start: %s",
            ErrFreezeViolation, i, strings.Join(extra, ", "))
    with (NOTE: "%w " SPACE, args order: ErrFreezeViolation, i, conceptTitle, join):
        return fmt.Errorf("%w in concept %d (%q): staged paths not in the frozen working-tree snapshot: %s. "+
            "This indicates concurrent working-tree changes were picked up by the stager. "+
            "Aborting to protect the freeze boundary.",
            ErrFreezeViolation, i, conceptTitle, strings.Join(extra, ", "))
  - MESSAGE (B) CONTENT (L193–196): replace
        return fmt.Errorf("%w: concept %d staged content not traceable to T_start: %s",
            ErrFreezeViolation, i, strings.Join(mismatch, ", "))
    with (NOTE: "%w " SPACE; "differs ... for:" wording; same arg order):
        return fmt.Errorf("%w in concept %d (%q): staged content differs from the frozen working-tree snapshot for: %s. "+
            "This indicates concurrent working-tree changes were picked up by the stager. "+
            "Aborting to protect the freeze boundary.",
            ErrFreezeViolation, i, conceptTitle, strings.Join(mismatch, ", "))
  - DOC-COMMENT (L141–157): update the final paragraph (currently: "Returns nil if the subset holds;
    ErrFreezeViolation-wrapped error naming the offending path(s) on a path-not-in-T_start or
    content-mismatch; ...") to also state the error names the concept BY TITLE (conceptTitle, via %q)
    and uses plain-language phrasing + remedy. Keep the algorithm description ((A)/(B) logic) intact —
    it is unchanged. One factual sentence or clause addition is enough.
  - PRESERVE: the (A)/(B) control flow, the `extra`/`mismatch` path-collection logic, the %w wrap of
    ErrFreezeViolation, the strings.Join path-naming, AND the two ErrDecomposeFailed DiffTreeNames
    wraps at L165/L183 UNCHANGED.
  - NAMING: `conceptTitle` (matches the item description's parameter name).

Task 2: EDIT internal/decompose/decompose.go — pass the title at the single call site (L583)
  - LOCATE (L583, inside runLoop's `for i, concept := range concepts {` loop):
        if vErr := verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, i, treeI); vErr != nil {
  - REPLACE with (add `concepts[i].Title` between `i` and `treeI`):
        if vErr := verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, i, concepts[i].Title, treeI); vErr != nil {
  - PROVENANCE: `concepts []prompt.PlannerCommit` is a runLoop param (L433); the loop is
    `for i, concept := range concepts {` (L567) so `i` indexes `concepts` validly. `concepts[i].Title`
    is the item-spec form (equivalent to `concept.Title`).
  - PRESERVE: the surrounding FR-M1c comment (L578–582) and the `drainMsg(inflight); return commits,
    nil, vErr` violation handling (L584–586) UNCHANGED.

Task 3: EDIT internal/decompose/stager_test.go — add the title arg to ALL FOUR call sites + update 2 substring asserts
  - CALL SITE L370 (TestVerifyFreezeSubset_Happy): add a title literal, e.g.
        verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, 0, "test-concept", treeI)
  - CALL SITE L406 (TestVerifyFreezeSubset_PathViolation): same — add `, "test-concept"` before treeI.
  - CALL SITE L452 (TestVerifyFreezeSubset_ContentViolation): same — add `, "test-concept"` before treeI.
  - CALL SITE L483 (TestVerifyFreezeSubset_EmptyStaging): same — add `, "test-concept"` before treeI.
  - ASSERT L416–417 (PathViolation, old "not present in T_start"): change the substring to the NEW (A)
    phrasing, e.g.
        if !strings.Contains(err.Error(), "staged paths not in the frozen working-tree snapshot") {
            t.Errorf("error missing 'staged paths not in the frozen working-tree snapshot'; got: %s", err.Error())
        }
  - ASSERT L462–463 (ContentViolation, old "not traceable to T_start"): change the substring to the
    NEW (B) phrasing, e.g.
        if !strings.Contains(err.Error(), "staged content differs from the frozen working-tree snapshot") {
            t.Errorf("error missing 'staged content differs from the frozen working-tree snapshot'; got: %s", err.Error())
        }
  - KEEP UNCHANGED: L413 (sentinel.txt), L460 (a.txt), L410/L456 (errors.Is(err, ErrFreezeViolation)).
  - OPTIONAL (recommended): add a title assertion to PathViolation + ContentViolation, e.g.
        if !strings.Contains(err.Error(), "test-concept") {
            t.Errorf("error missing concept title 'test-concept'; got: %s", err.Error())
        }

Task 4: EDIT internal/decompose/decompose_test.go — update 1 substring assert (+ optional title assert)
  - ASSERT L773–774 (TestDecompose_StagerFreezeViolation, old "not present in T_start"): change to the
    NEW (A) phrasing:
        if !strings.Contains(err.Error(), "staged paths not in the frozen working-tree snapshot") {
            t.Errorf("error missing 'staged paths not in the frozen working-tree snapshot'; got: %s", err.Error())
        }
  - KEEP UNCHANGED: L770 (sentinel.txt), L767 (errors.Is(err, ErrFreezeViolation)).
  - OPTIONAL (recommended): assert the concept title "c1" (from the planner JSON at L750) appears:
        if !strings.Contains(err.Error(), "c1") {
            t.Errorf("error missing concept title 'c1'; got: %s", err.Error())
        }

Task 5: VALIDATE — build, targeted tests, full regression, format, vet, scope
  - go build ./...
  - go test ./internal/decompose/... -run 'FreezeSubset|StagerFreezeViolation' -race -v
  - go test ./internal/decompose/... -race
  - go test -race ./...
  - gofmt -l internal/decompose/stager.go internal/decompose/stager_test.go internal/decompose/decompose.go internal/decompose/decompose_test.go   # must print nothing
  - go vet ./internal/decompose/...
  - git status --porcelain   # ONLY the intended files
```

### Implementation Patterns & Key Details

```go
// PATTERN (the %w-SPACE rewrite — THE critical detail; verbatim from the item description):
//   Message (A) PATH:
return fmt.Errorf("%w in concept %d (%q): staged paths not in the frozen working-tree snapshot: %s. "+
    "This indicates concurrent working-tree changes were picked up by the stager. "+
    "Aborting to protect the freeze boundary.",
    ErrFreezeViolation, i, conceptTitle, strings.Join(extra, ", "))
//   Renders → "decompose: freeze violation in concept 0 ("c1"): staged paths not in the frozen
//              working-tree snapshot: sentinel.txt. This indicates concurrent working-tree changes
//              were picked up by the stager. Aborting to protect the freeze boundary."

//   Message (B) CONTENT (note "differs ... for:" vs (A)'s "... snapshot:"):
return fmt.Errorf("%w in concept %d (%q): staged content differs from the frozen working-tree snapshot for: %s. "+
    "This indicates concurrent working-tree changes were picked up by the stager. "+
    "Aborting to protect the freeze boundary.",
    ErrFreezeViolation, i, conceptTitle, strings.Join(mismatch, ", "))

// PATTERN (call site — title threaded from the loop's concept slice):
if vErr := verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, i, concepts[i].Title, treeI); vErr != nil {
    drainMsg(inflight)
    return commits, nil, vErr
}

// PATTERN (test call after the signature change — title literal between `0` (i) and treeI):
verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, 0, "test-concept", treeI)
//   then assert BOTH the new phrasing AND (optionally) the title, e.g.:
if !strings.Contains(err.Error(), "staged paths not in the frozen working-tree snapshot") { t.Errorf(...) }
if !strings.Contains(err.Error(), "test-concept") { t.Errorf(...) }   // optional: proves the (%q) wiring
```

### Integration Points

```yaml
SIGNATURE (internal/decompose/stager.go:159):
  - add param: `conceptTitle string`, positioned AFTER `i int`, BEFORE `treeI string`.
PROD CALL SITE (internal/decompose/decompose.go:583):
  - add arg: `concepts[i].Title`, in the same position (after `i`, before `treeI`).
TEST CALL SITES (internal/decompose/stager_test.go:370/406/452/483):
  - add arg: a title literal (e.g. "test-concept") in the same position.
DOC-COMMENT (internal/decompose/stager.go:141–157):
  - refresh: note the concept is named by title (conceptTitle via %q) + clearer phrasing.
NO config / routes / migrations / env vars / new imports / new files / new exports.
NO docs-surface change (item DOCS: "none — error message improvement"). docs/how-it-works.md already
  has an FR-M1c bullet; it needs no edit for phrasing.
```

## Validation Loop

### Level 1: Build & format (Immediate Feedback)

```bash
# Compiles the whole module — proves the new signature + call site + all test call sites agree.
go build ./...
# Expected: no output. If it errors on an arg-count mismatch at a verifyFreezeSubset call, a test call
# site (stager_test.go L370/406/452/483) is missing the new title arg — fix it.

# Formatting check — must print NOTHING.
gofmt -l internal/decompose/stager.go internal/decompose/stager_test.go internal/decompose/decompose.go internal/decompose/decompose_test.go
# Expected: empty output. If a file is listed, run `gofmt -w` on it and re-check.
```

### Level 2: Unit Tests (the touched freeze-violation tests)

```bash
# Targeted: the four verifyFreezeSubset unit tests + the integration freeze-violation test, race-enabled.
go test ./internal/decompose/... -run 'FreezeSubset|StagerFreezeViolation' -race -v
# Expected: all PASS. Happy + EmptyStaging return nil; PathViolation asserts ErrFreezeViolation + the
#           NEW path phrasing + (optionally) the title; ContentViolation asserts ErrFreezeViolation +
#           the NEW content phrasing + (optionally) the title; StagerFreezeViolation asserts the same
#           at the integration level (title "c1" from the planner JSON).

# Full decompose package (regression — every other decompose test untouched).
go test ./internal/decompose/... -race
# Expected: PASS. Confirms the message/signature change didn't disturb any other path.
```

### Level 3: Whole-repo regression + static checks

```bash
# Whole repo — catches any downstream compile/test break (e.g. another package asserting the old text).
go test -race ./...
# Expected: PASS. (internal/cmd CLI/e2e paths assert on ErrFreezeViolation via errors.Is, not on the
# exact message text, so the rewording is transparent to them.)

# Static checks.
go vet ./internal/decompose/...
# Expected: clean.

# Coverage gate (PRD §20.3 gates internal/{git,provider,generate,config}, not decompose — no gate
# impact; run for hygiene).
make coverage-gate
# Expected: PASS (decompose is not in the gated set).
```

### Level 4: Scope guard + message-render sanity

```bash
# ONLY the intended files changed.
git status --porcelain
# Expected: M internal/decompose/stager.go  AND  M internal/decompose/decompose.go  AND
#           M internal/decompose/stager_test.go  AND  M internal/decompose/decompose_test.go (nothing else).

# Guard: no out-of-scope / forbidden files touched.
git status --porcelain | grep -E 'internal/prompt/planner\.go|PRD\.md|plan/.*tasks\.json|prd_snapshot|delta_prd|how-it-works\.md|git\.go' && echo "FAIL: forbidden file touched" || echo "OK: scope clean"
# Expected: "OK: scope clean". planner.go (PlannerCommit.Title owner) MUST be untouched; the docs /
# PRD / plan / snapshot files MUST be untouched.

# Render sanity (manual): trigger a path violation and eyeball the exact message. The fastest way is the
# targeted unit test with -v; the ContentViolation/PathViolation error text is printed on failure, or
# add a transient t.Log(err) to confirm the verbatim render, then remove it.
go test ./internal/decompose/... -run TestVerifyFreezeSubset_PathViolation -race -v
# Expected: the (A) message renders exactly "decompose: freeze violation in concept 0 (...): staged
#           paths not in the frozen working-tree snapshot: sentinel.txt. This indicates concurrent ..."
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` succeeds (proves the new signature + all 5 call sites agree)
- [ ] `go test ./internal/decompose/... -run 'FreezeSubset|StagerFreezeViolation' -race -v` passes
- [ ] `go test ./internal/decompose/... -race` passes (full package regression)
- [ ] `go test -race ./...` passes (whole-repo regression)
- [ ] `gofmt -l <the 4 files>` prints nothing
- [ ] `go vet ./internal/decompose/...` clean

### Feature Validation
- [ ] Message (A) renders the item's LITERAL text and still wraps `ErrFreezeViolation` (errors.Is true)
- [ ] Message (B) renders the item's LITERAL text and still wraps `ErrFreezeViolation` (errors.Is true)
- [ ] Both messages name the concept by TITLE via `(%q)` and name the offending paths via `%s`
- [ ] The `%w ` (SPACE, not `%w:` colon) form is used in BOTH messages (the render-match gotcha)
- [ ] The single prod call site passes `concepts[i].Title`
- [ ] The 3 old-phrasing substring assertions are updated; the path-naming + errors.Is assertions hold

### Code Quality Validation
- [ ] Follows existing `fmt.Errorf("%w: ...", ErrFreezeViolation, ...)` style (modulo the deliberate
      `%w `-space change, which is required for the verbatim render)
- [ ] The doc-comment is refreshed to mention the concept is named by title
- [ ] The two `ErrDecomposeFailed` DiffTreeNames wraps (L165/L183) are left UNCHANGED
- [ ] No import block touched; no new files

### Scope-Boundary Validation
- [ ] `git status --porcelain` shows ONLY `internal/decompose/stager.go` +
      `internal/decompose/decompose.go` + `internal/decompose/stager_test.go` +
      `internal/decompose/decompose_test.go`
- [ ] `internal/prompt/planner.go` UNCHANGED (Title already exists)
- [ ] `ErrFreezeViolation` sentinel string UNCHANGED (changing it breaks the %w-render-match + errors.Is)
- [ ] NO edit to PRD.md, plan/**/tasks.json, prd_snapshot.md, delta_prd.md, docs/, the CLI, or any other source

---

## Anti-Patterns to Avoid

- ❌ Don't use `%w: ` (colon) in the new messages — the sentinel string already IS the prefix; use
  `%w ` (SPACE) so the render matches the item's literal text. The current code's `%w: ` style is the
  thing being deliberately changed here.
- ❌ Don't drop the `%w` wrap of `ErrFreezeViolation` to "simplify" — `errors.Is(err,
  ErrFreezeViolation)` is asserted by tests and relied on by the orchestrator's violation handling.
- ❌ Don't change message (A)'s "staged paths not in the frozen working-tree snapshot: %s" to (B)'s
  "staged content differs from the frozen working-tree snapshot for: %s" or vice versa — they are
  deliberately distinct (A = a path not in T_start; B = a path in T_start but with different content).
- ❌ Don't forget ALL FOUR test call sites in stager_test.go (L370/406/452/483) need the new title arg
  — not just the two the item's prose names. Missing any one fails to compile.
- ❌ Don't update the "sentinel.txt"/"a.txt" assertions — the new messages still name the paths; those
  checks stay valid and green.
- ❌ Don't touch the two `ErrDecomposeFailed` DiffTreeNames wraps (L165/L183) or add `conceptTitle` to
  them — they are out-of-scope infra errors, not user-facing freeze violations.
- ❌ Don't implement the `delta_prd.md:120` "append ; unstage..." remedy — the ITEM DESCRIPTION
  supersedes it with the full verbatim rewrite. The item's literal messages are authoritative.
- ❌ Don't change the `ErrFreezeViolation` sentinel string itself — it is the prefix that `%w` renders,
  and changing it breaks the exact-match render AND the existing `errors.Is` semantics.
- ❌ Don't add a docs/how-it-works.md change — the item's DOCS field says "none"; an FR-M1c bullet
  already exists and needs no phrasing edit.
