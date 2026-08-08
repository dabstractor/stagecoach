name: "P1.M1.T1.S1 — isFileDisjoint gate (pure predicate + unit test) for FR-M13 file-disjoint fast-path"
description: >
  Add a pure, deterministic predicate `func isFileDisjoint(concepts []prompt.PlannerCommit) bool` in
  internal/decompose/decompose.go that returns true iff no file path appears in more than one concept's
  Files (the FR-M13 gate that decides whether the fast-path deterministic staging sweep applies vs the
  FR-M5 tooled-stager fallback). No I/O, no git, no state mutation. Plus a table-driven unit test in
  decompose_test.go. This is the first piece of the FR-M13/FR-M14 fast-path; S2-S4 build the staging
  sweep/concurrency/dispatch on top of this gate.

---

## Goal

**Feature Goal**: Provide the pure, deterministic set-membership gate (FR-M13) that decides whether the
planner's partition is pairwise file-disjoint — eligible for the deterministic `git add` fast-path — or
has a shared path (hunk-split intent) requiring the FR-M5 tooled-stager fallback.

**Deliverable**: (1) `isFileDisjoint(concepts []prompt.PlannerCommit) bool` in internal/decompose/decompose.go
(co-located near runLoop, line 456); (2) a table-driven `TestIsFileDisjoint` in internal/decompose/decompose_test.go
covering the disjoint/shared/empty/single/vacuous/intra-dupe cases.

**Success Definition**:
- `isFileDisjoint` returns true iff no path's occurrence count across all concepts' Files exceeds 1.
- It is pure (no I/O, no git, no allocation beyond the count map), deterministic, and uses no new imports.
- The unit test passes for all matrix rows; it FAILS if the predicate is inverted or the early-exit is broken.
- `go build ./...`, `go test ./internal/decompose/...`, `make test`, `make lint` pass.
- runLoop/Decompose are UNCHANGED (S4 wires the gate; S1 is predicate + test only).

## Why

- **FR-M13 (P0)**: When the planner's partition is pairwise file-disjoint, stagecoach can stage each
  concept deterministically with `git add` (no stager agent) — collapsing the critical path and letting
  a provider with no `tooled_flags` (opencode, qwen-code) decompose a disjoint tree. A path declared for
  ≥2 concepts (a hunk split) disqualifies the fast-path for the WHOLE run → fall back to the FR-M5 tooled
  stager. The gate is "automatic and deterministic (a set-membership test over the planner's files)" —
  this subtask IS that test.
- **Why a separate predicate first**: the gate is pure and trivially unit-testable in isolation, while
  the staging sweep (S2) and concurrency (S3) are heavier. Landing the predicate + its test first gives
  S4's dispatch a fully-validated gate to call, and the test matrix pins the exact disjointness contract
  before the downstream fast-path code depends on it.

## What

**User-visible behavior**: None (internal predicate, not yet wired into any dispatch).

**Technical change (one predicate + one test):**
- `isFileDisjoint` iterates every concept's `Files`, tracks a `map[string]int` occurrence count, and
  returns false as soon as any path's count exceeds 1 (early exit); true otherwise.

### Success Criteria
- [ ] `isFileDisjoint` returns true for pairwise-disjoint concepts (incl. empty-Files concepts, single concept, empty slice)
- [ ] Returns false when any path appears in ≥2 concepts (and, per the literal count algorithm, on intra-concept duplicate)
- [ ] Pure: no I/O, no git, no state mutation; no new imports
- [ ] Table-driven test covers the full matrix
- [ ] runLoop/Decompose UNCHANGED
- [ ] `go build ./...`, `go test ./internal/decompose/...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact input type, the co-location anchor, the no-new-import confirmation, the full algorithm, and the complete test matrix (including the two defensive edge cases and the intra-dupe decision) are enumerated below (verified by reading source).

### Documentation & References

```yaml
- file: internal/prompt/planner.go
  why: "The input type. PlannerCommit (83-87) has Files []string (json:\"files\") — the disjointness-test input. PlannerOutput.Commits (95) is []PlannerCommit — the slice the helper receives."
  pattern: "type PlannerCommit struct { Title, Description string; Files []string }"
  gotcha: "Files is GUIDANCE from the planner (FR-M3), not a hard constraint — but for the FR-M13 gate it IS the disjointness input. Empty Files (stages nothing) does NOT disqualify."

- file: internal/decompose/decompose.go
  why: "THE home file. runLoop (456) already takes concepts []prompt.PlannerCommit — co-locate isFileDisjoint near it. Decompose (144) is the S4 dispatch site (DO NOT touch in S1). Imports (29-39) already include prompt — NO new import needed."
  pattern: "runLoop(ctx, deps, concepts []prompt.PlannerCommit, baseTree, tStart, preRunHEAD, isUnborn) — the helper takes just the concepts slice."
  gotcha: "Zero new imports: the helper uses only prompt.PlannerCommit (imported) + map[string]int (builtin). Adding 'strings' or any stdlib would be an unused-import error."

- file: internal/decompose/decompose_test.go
  why: "Test home (package decompose — internal test ⇒ calls isFileDisjoint directly). Existing tests are heavyweight (real git + Deps + stubs); the predicate test is PURE — construct []prompt.PlannerCommit literals, assert bool. No t.TempDir, no git, no mocking."
  pattern: "Table-driven: cases := []struct{name string; in []prompt.PlannerCommit; want bool}{...}; for _, tc := range cases { t.Run(tc.name, func(t *testing.T){ if got := isFileDisjoint(tc.in); got != tc.want { t.Errorf(...) } }) }"

- docfile: plan/019_2f5621db4d2b/P1M1T1S1/research/verification_deltas.md
  why: "The verified anchors, the literal algorithm, the full edge-case matrix (incl. the intra-concept-dupe decision and the vacuous-empty-slice case), and the scope boundaries. READ THIS before editing."
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  decompose.go        # THE home: add isFileDisjoint near runLoop(456). Decompose(144) = S4 dispatch site (UNTOUCHED)
  decompose_test.go   # +TestIsFileDisjoint (table-driven; pure, no git)
internal/prompt/
  planner.go          # PlannerCommit(83)/PlannerOutput(92) — the input types (UNCHANGED)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (pure + no new imports): the helper uses ONLY prompt.PlannerCommit (already imported in
//   decompose.go) + a map[string]int (builtin). Do NOT add any import — an unused import is a compile
//   error. No I/O, no git, no *Deps, no cobra, no config.

// GOTCHA (literal count algorithm): track a map[string]int occurrence count; return false as soon as
//   any path's count > 1 (early exit). This counts INTRA-concept duplicates too (one concept listing
//   "a.go" twice ⇒ count 2 ⇒ false). That is the task's literal algorithm and is defensible (degenerate
//   planner output). Do NOT pre-dedupe within a concept unless a future revision asks for it.

// GOTCHA (empty Files ≠ disqualify): a concept with empty Files shares no path — it stages nothing and
//   hits FR-M8 empty-skip downstream. It must NOT cause isFileDisjoint to return false. The count map
//   simply adds nothing for it.

// GOTCHA (early exit): return false the moment a second occurrence is seen — do NOT count the whole
//   run first. (Functionally identical for the result, but early-exit is the idiomatic, minimal-work
//   form and the test should not depend on full-scan behavior.)

// SCOPE: S1 is the predicate + its unit test. Do NOT edit runLoop/Decompose (S4 wires the gate), do NOT
//   add runLoopFastPath (S2/S3), do NOT change PlannerCommit, do NOT add config or docs.
```

## Implementation Blueprint

### Data models and structure

No struct/type changes. One new unexported function returning bool. Pure; deterministic.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD isFileDisjoint in internal/decompose/decompose.go
  - PLACE: near runLoop (line 456) — co-locate the fast-path gate with the loop it gates.
  - BODY:
        // isFileDisjoint reports whether the planner's partition is pairwise file-disjoint: no path
        // appears in more than one concept's Files (FR-M13). When true, the deterministic git-add
        // fast-path applies (no tooled stager); when false (a path is shared — a hunk-split intent),
        // the run falls back to the FR-M5 tooled stager for every concept. A concept with empty Files
        // shares no path and does NOT disqualify. Pure: no I/O, no git, no state mutation.
        func isFileDisjoint(concepts []prompt.PlannerCommit) bool {
            seen := make(map[string]int)
            for _, c := range concepts {
                for _, p := range c.Files {
                    seen[p]++
                    if seen[p] > 1 {
                        return false
                    }
                }
            }
            return true
        }
  - NO new import (prompt already imported; map is builtin).
  - DEPENDENCIES: none.

Task 2: ADD TestIsFileDisjoint in internal/decompose/decompose_test.go
  - PLACE: anywhere in the file (pure test; group with other helper-level tests if any, else append).
  - TABLE-DRIVEN over the matrix. Construct []prompt.PlannerCommit literals directly.
        func TestIsFileDisjoint(t *testing.T) {
            cases := []struct {
                name string
                in   []prompt.PlannerCommit
                want bool
            }{
                {"empty slice", nil, true},                                          // vacuous
                {"single concept", []prompt.PlannerCommit{{Files: []string{"a.go","b.go"}}}, true},
                {"pairwise disjoint 3 concepts", []prompt.PlannerCommit{
                    {Files: []string{"a.go"}},
                    {Files: []string{"b.go", "c.go"}},
                    {Files: []string{"d.go"}},
                }, true},
                {"empty-Files concept among disjoint", []prompt.PlannerCommit{
                    {Files: []string{"a.go"}},
                    {Files: nil},
                    {Files: []string{"b.go"}},
                }, true},
                {"all empty Files", []prompt.PlannerCommit{{Files: nil}, {Files: []string{}}}, true},
                {"shared path two concepts", []prompt.PlannerCommit{
                    {Files: []string{"a.go", "shared.go"}},
                    {Files: []string{"shared.go", "b.go"}},
                }, false},
                {"shared path across three concepts", []prompt.PlannerCommit{
                    {Files: []string{"x.go"}},
                    {Files: []string{"x.go"}},
                    {Files: []string{"x.go"}},
                }, false},
                {"intra-concept duplicate disqualifies (literal count)", []prompt.PlannerCommit{
                    {Files: []string{"a.go", "a.go"}},
                }, false},
            }
            for _, tc := range cases {
                t.Run(tc.name, func(t *testing.T) {
                    if got := isFileDisjoint(tc.in); got != tc.want {
                        t.Errorf("isFileDisjoint(%+v) = %v; want %v", tc.in, got, tc.want)
                    }
                })
            }
        }
  - IMPORTS: "github.com/dabstractor/stagecoach/internal/prompt" (already imported in decompose_test.go
    if other tests build PlannerCommit; confirm via goimports/gofmt — add only if missing).
  - DEPENDENCIES: Task 1.

Task 3: VERIFY runLoop/Decompose UNCHANGED + no behavior regression
  - CONFIRM runLoop (456) and Decompose (144) were NOT edited (S1 is predicate + test only).
  - CONFIRM existing TestDecompose_* tests still pass (the predicate is additive; nothing wired yet).
  - DEPENDENCIES: Tasks 1-2.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the FR-M13 set-membership gate (occurrence count + early exit)
func isFileDisjoint(concepts []prompt.PlannerCommit) bool {
    seen := make(map[string]int)
    for _, c := range concepts {
        for _, p := range c.Files {
            seen[p]++
            if seen[p] > 1 {
                return false // a path is shared (hunk-split intent) ⇒ disqualify the whole run
            }
        }
    }
    return true
}
// Empty Files ⇒ inner loop is a no-op ⇒ that concept contributes nothing ⇒ does NOT disqualify.
```

### Integration Points

```yaml
NO struct / API / config / build changes. One unexported predicate + one pure unit test.

CODE:
  - internal/decompose/decompose.go — +isFileDisjoint (near runLoop, line 456)
TESTS:
  - internal/decompose/decompose_test.go — +TestIsFileDisjoint (table-driven, pure)

DOWNSTREAM (consumes this gate — do NOT implement in S1):
  - P1.M1.T1.S4: Decompose() dispatch — `if isFileDisjoint(concepts) { runLoopFastPath(...) } else { runLoop(...) }`

UNCHANGED: runLoop; Decompose; PlannerCommit/PlannerOutput; any staging/generation/git logic.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...
go vet ./...
gofmt -l internal/decompose/
# Expected: empty.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new predicate test
go test ./internal/decompose/ -run TestIsFileDisjoint -v
# Expected: all table rows pass (disjoint/empty/single/vacuous → true; shared/intra-dupe → false).

# Full decompose package (existing TestDecompose_* must still pass — predicate is additive, unwired)
go test ./internal/decompose/... -v

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# (S1 is a pure predicate not yet wired into Decompose — no end-to-end fast-path exists until S4 lands.
#  The table-driven unit test IS the within-scope proof. The full FR-M13 fast-path e2e [disjoint tree →
#  git-add staging → no stager agent] is validated in S5's regression-suite gate, not S1's.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the predicate exists with the FR-M13 doc comment
grep -n "func isFileDisjoint\|FR-M13" internal/decompose/decompose.go
# Expected: the function + the FR-M13 citation present.

# Grep guard: NO new import added (prompt was already imported)
git diff internal/decompose/decompose.go | grep -E '^\+' | grep -E '^\+\s*"strings"|"strconv"|"sort"' | grep -v '^\+\+\+'
# Expected: empty (no newly-added stdlib import).

# Grep guard: runLoop/Decompose UNCHANGED in S1 (S4 owns the dispatch wiring)
git diff internal/decompose/decompose.go | grep -nE '^\+.*isFileDisjoint(concepts)|^\-.*func runLoop|^\-.*func Decompose'
# Expected: empty (S1 adds the predicate + test only; no runLoop/Decompose edit).

# Mutation guard: prove the test catches an inverted predicate (manual, then revert):
#   temporarily change `return false`/`return true` or the `> 1` threshold in isFileDisjoint,
#   re-run `go test ./internal/decompose/ -run TestIsFileDisjoint`, confirm FAILS, then REVERT.

# Scope-boundary guard: only decompose.go + decompose_test.go changed
git diff --name-only
# Expected: only internal/decompose/decompose.go + internal/decompose/decompose_test.go.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/decompose/` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass

### Feature Validation
- [ ] `isFileDisjoint` returns true for pairwise-disjoint / empty-Files / single / empty-slice
- [ ] Returns false for any shared path (cross-concept) and intra-concept duplicate (per literal count)
- [ ] Pure: no I/O, no git, no state mutation
- [ ] Early-exit on the second occurrence of any path
- [ ] Table-driven test covers the full matrix

### Scope-Boundary Validation
- [ ] runLoop/Decompose UNCHANGED (S4 wires the gate)
- [ ] NO runLoopFastPath added (S2/S3)
- [ ] NO new import (prompt already imported; map is builtin)
- [ ] PlannerCommit/PlannerOutput untouched
- [ ] Only internal/decompose/decompose.go + decompose_test.go changed

### Code Quality
- [ ] Predicate co-located near runLoop (fast-path helpers grouped)
- [ ] Doc comment cites FR-M13 + the disjoint/fast-path/fallback rationale
- [ ] Test follows the table-driven t.Run idiom; pure (no git/fixtures)

---

## Anti-Patterns to Avoid

- ❌ Don't add any import — `prompt` is already imported in decompose.go and the helper needs nothing else (map is builtin). An unused import is a compile error.
- ❌ Don't make the predicate do I/O, touch git, or take `Deps` — it is a PURE set-membership test. S4 calls it; S1 just defines it.
- ❌ Don't pre-dedupe within a concept — the task's literal algorithm is an occurrence count (any path count > 1 ⇒ false), which counts intra-concept duplicates. Deduping would diverge from the spec; if a future revision wants intra-conduit dedup, that's a deliberate change, not S1.
- ❌ Don't full-scan-then-decide — early-exit (`if seen[p] > 1 { return false }`) on the second occurrence. (Functionally equivalent here, but early-exit is the idiomatic minimal-work form.)
- ❌ Don't edit runLoop/Decompose or add runLoopFastPath — S4/S2/S3 own those. S1 is predicate + test only.
- ❌ Don't treat empty Files as a disqualifier — a concept that stages nothing shares no path; it must not cause false (it hits FR-M8 empty-skip downstream).
- ❌ Don't add config or a flag — FR-M13 is explicit that the gate "adds no configuration."

---

## Confidence Score: 10/10

This is a small, fully-specified pure predicate: the input type is confirmed (PlannerCommit.Files
[]string), the algorithm is given verbatim in the task (occurrence count, any >1 ⇒ false), no new
imports are needed, the co-location anchor (runLoop, line 456) is confirmed, and the test matrix is
fully enumerated (including the two defensive cases and the intra-concept-dupe decision). There is no
I/O, no git, no concurrency, and no downstream coupling (S4 wires it later). The only conceivable
failure mode — an implementer adding an unused import or inverting the predicate — is caught immediately
by `go build`/`go vet` and the mutation-guard step.