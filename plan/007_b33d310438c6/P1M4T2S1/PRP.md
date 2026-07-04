---
name: "P1.M4.T2.S1 — Water-fill level solver (sort-and-walk) (PRD §9.1 FR3i; architecture/git_diff_semantics.md §6)"
description: |

  Implement the FR3i dynamic water-fill truncation SOLVER as two PURE, git-independent functions in
  internal/git: `waterFillLevel(sizes []int, budget int) (level int, truncate bool)` and
  `allocByWaterFill(sizes []int, budget int) []int`. This is the classic water-filling / max-min-fair-
  with-caps algorithm (architecture/git_diff_semantics.md §6 gives the O(n log n) sort-and-walk reference
  implementation + verified numeric cases). Given per-file sizes and a budget, it finds the water level L
  such that Σ min(size_i, L) = budget: files smaller than L are kept whole; files larger than L are
  truncated to exactly L. Small files' unused budget is reclaimed and redistributed to the large files.

  CONTRACT (item_description §3, verbatim): the solver is PURE (no git, no I/O) and UNIT-AGNOSTIC — sizes
  and budget are ints in any consistent unit; in PRODUCTION (FR3i) the unit is TOKENS (sizes derived from
  numstatRows via git.EstimateTokens; budget = body_budget in tokens), but the conversion is the S2
  consumer's job, NOT this task's. The solver just does arithmetic. Output: "pure functions waterFillLevel
  + allocByWaterFill, fully tested independent of git. Consumed by S2 (application) and M4.T3 (gate)."

  RETURN CONTRACT (git_diff_semantics.md §6 + item_description §3):
    - waterFillLevel: if Σ sizes ≤ budget → return (maxSize, false) [no truncation; maxSize = max(sizes),
      0 for empty — so allocByWaterFill's min(size_i, maxSize) = size_i ⇒ every file whole]. Else sort
      ascending and walk: decrement remaining; when s[i]*count > remaining return level=remaining/count
      (INTEGER floor division), truncate=true. Empty sizes → (0, false).
    - allocByWaterFill: delegates to waterFillLevel; returns []int where out[i] = min(sizes[i], level),
      PRESERVING THE ORIGINAL INPUT ORDER (S2 reassociates allotments to files by index — output MUST be
      parallel to the input, NOT sorted).

  VERIFIED NUMERIC CASES (§6, all zero-slack; MUST appear as table tests with HARDCODED expectations):
    - [10,20,30]@30 → level=10, truncate=true, allot=[10,10,10], Σ=30=budget.
    - [5,100]@50   → level=45, truncate=true, allot=[5,45],     Σ=50=budget.
    - [10,10]@50   → level=10, truncate=false, allot=[10,10] (total ≤ budget ⇒ no truncation).

  EDGE CASES (§6, all table-tested): empty sizes; total ≤ budget (no truncation); all-equal sizes with
  n·S > budget (even split at floor(B/n)); one file larger than budget; single file larger than budget;
  B=0 (level=0, all truncated to zero); UNSORTED INPUT (order preservation — allocByWaterFill([100,5],50)
  → [45,5], NOT [5,45]).

  INTEGER-SLACK INVARIANT (the property-test target): level = remaining/count is FLOOR division (no
  round-robin remainder spending — simpler, matches the contract's level=remaining/count). slack =
  budget − Σ allotments = remaining mod count < count ≤ N. Property test asserts, when truncated:
  0 ≤ slack < N (N = len(sizes)). When NOT truncated: Σ allotments == Σ sizes ≤ budget (no full-utilization
  claim — correct, we don't fill the budget when nothing needs truncating).

  INPUT (upstream — READ-ONLY contracts, do NOT modify): `git.EstimateTokens` (P1.M4.T1.S1, parallel —
  the SINGLE chars/4 estimator; this task does NOT call it — it's the unit S2 uses, documented only).
  `numstatRow{Added, Deleted, IsBinary, Path}` (P1.M3.T1.S1 — the per-file size source S2 converts; this
  task does NOT touch numstat.go). `StagedDiffOptions.TokenLimit/PromptReserveTokens` (the gate's inputs,
  M4.T3 — unread here).

  OUTPUT (downstream — the frozen consumer contract): S2 (P1.M4.T2.S2 — per-file truncation application)
  calls waterFillLevel/allocByWaterFill with token sizes + body_budget, then truncates each file's body to
  its allotment (first level tokens + the `... [truncated]` sentinel), preserving diff --git/hunk headers.
  M4.T3 (P1.M4.T3.S1 — the gate) computes body_budget = token_limit − skeleton − reserve − margin and
  triggers the water-fill when token_limit > 0. SIGNATURES ARE FROZEN — do not change them.

  DELIVERABLES (2 NEW files; nothing else touched):
    NEW internal/git/waterfill.go      — `package git`. waterFillLevel + allocByWaterFill (pure; stdlib
      `sort` only; no context/I/O/git). Doc comments cite FR3i, §6, the unit-agnosticism + production-token
      note, the return contract, and the frozen-signature consumer contract.
    NEW internal/git/waterfill_test.go — `package git`. Exhaustive table tests (all §6 edge cases + the 3
      verified numeric cases + the integer-slack case + unsorted-input order preservation) with HARDCODED
      expectations, PLUS a randomized property loop (deterministic seed, non-negative sizes/budgets, ~1000
      cases) asserting the 5 invariants. Mirrors tokens_test.go's table-driven style.

  SCOPE BOUNDARY (owned by siblings — do NOT implement): size derivation numstat→tokens (S2);
  body_budget computation token_limit−skeleton−reserve−margin (M4.T3); per-file truncation application +
  sentinel + header preservation + diff --git splitting (S2); the token-limit gate in the 3 diff functions
  (M4.T3); estimateTokens (S1 — frozen); numstat.go/skeleton.go/tokens.go/the diff functions/
  StagedDiffOptions — UNCHANGED.

  Deliverable: 2 NEW pure functions + exhaustive tests. `go build/vet/gofmt` clean; `go test ./...` green
  (pure additions — no behavior change, no consumer reads them yet); only the 2 new files differ.

---

## Goal

**Feature Goal**: Implement the FR3i dynamic water-fill truncation SOLVER (PRD §9.1 FR3i; git_diff_semantics.md
§6) as two pure, git-independent functions — `waterFillLevel` (the sort-and-walk level solver) and
`allocByWaterFill` (the per-file allotment, order-preserving) — fully tested (exhaustive §6 edge cases + 3
verified numeric cases + a randomized property loop), ready for S2 (application) and M4.T3 (gate) to consume.

**Deliverable** (2 NEW files; nothing else touched):
1. `internal/git/waterfill.go` — `package git`. `func waterFillLevel(sizes []int, budget int) (level int,
   truncate bool)` + `func allocByWaterFill(sizes []int, budget int) []int`. PURE (stdlib `sort` only; no
   context/I/O/git). Doc comments cite FR3i + §6 + the unit-agnostic/production-token note + the frozen
   consumer contract.
2. `internal/git/waterfill_test.go` — `package git`. Exhaustive table tests (hardcoded expectations) +
   randomized property loop.

**Success Definition**: `waterFillLevel([10,20,30],30)=(10,true)`, `([5,100],50)=(45,true)`,
`([10,10],50)=(10,false)`; `allocByWaterFill([100,5],50)=[45,5]` (input order preserved); all §6 edge cases
(empty, B=0, one-large, single-large, all-equal) pass; the randomized property loop (1000 cases) asserts
Σ allotments ≤ budget, allotment_i ≤ size_i, allotment_i ≥ 0, no-truncation ⇒ allotment_i == size_i, and
truncated ⇒ 0 ≤ slack < N. `go build ./... && go vet ./... && go test ./...` green; `gofmt -l` clean; only
the 2 new files differ.

## User Persona

**Target User**: The downstream subtasks S2 (per-file truncation application) and M4.T3 (the token-limit
gate), which call these functions. Transitively: every user who sets `token_limit` (PRD §9.1 FR3d) so a
large diff fits their model's context window — the water-fill decides how much of each file the model sees.

**Use Case**: A user sets `token_limit = 120000`. The gate (M4.T3) computes body_budget; S2 derives each
file's token size; the solver returns the water level + allotments; S2 truncates the large files to the
level (first L tokens + sentinel) and leaves small files whole. The model sees every file (small ones
intact, large ones fairly capped) without any one file monopolizing the budget.

**User Journey**: (internal) S2 builds `sizes` + `body_budget` → `level, truncate := waterFillLevel(sizes,
budget)` → `allots := allocByWaterFill(sizes, budget)` → for each file, if `allots[i] < sizes[i]`, truncate
the body to `allots[i]` tokens + the sentinel; else keep whole.

**Pain Points Addressed**: A static per-file cap would either truncate files that fit or let one giant file
starve the rest. Water-fill reclaims unused budget from small files and redistributes it to large files —
the provably-fairest cap-aware allocation (§6), so a one-line config tweak survives intact while a generated
lockfile-sized artifact is capped, and the cap adapts to how many files compete.

## Why

- **It IS the FR3i truncation algorithm.** PRD §9.1 FR3i mandates "a dynamic, size-aware water-fill — there
  is deliberately no static per-file cap," with the §6 exact algorithm. This task implements that algorithm.
- **The fairness guarantees (FR3i a–d) live here.** (a) only files exceeding L are truncated, minimally;
  (b) no budget wasted (small files' unused share reclaimed); (c) budget fully utilized (modulo integer
  slack); (d) no file monopolizes (all capped at L) yet a large substantive file still gets the bulk. The
  solver is what makes these hold.
- **Pure + independent of git ⇒ exhaustively testable.** The solver has no git/IO/context — it's arithmetic.
  So it can be table-tested + property-tested in milliseconds, in isolation, before S2 wires it into the
  diff path. Bugs caught here (not in a 7-stage integration run).
- **Reuses the single estimator's UNIT, not its code.** Production sizes/budget are in tokens (the
  git.EstimateTokens unit) so the budget arithmetic is coherent — but the solver never calls EstimateTokens
  (S2 does the conversion), keeping it pure.

## What

Two pure functions in `internal/git/waterfill.go`:

`waterFillLevel(sizes []int, budget int) (level int, truncate bool)`:
- If `sum(sizes) ≤ budget`: return `(max(sizes), false)` (maxSize; 0 if sizes empty). No truncation.
- Else: copy + sort sizes ascending; `remaining := budget`; for `i := 0..n-1`: `count := n - i`; if
  `sizes[i] * count ≤ remaining` then `remaining -= sizes[i]` (file i is small, served whole); else
  return `(remaining / count, true)` (floor division — the water level for files i..n-1).
- Empty `sizes`: `sum = 0 ≤ budget` ⇒ `(0, false)`.

`allocByWaterFill(sizes []int, budget int) []int`:
- `level, _ := waterFillLevel(sizes, budget)`; return `out` where `out[i] = min(sizes[i], level)` for every
  `i`, **in input order** (NOT sorted). Empty `sizes` ⇒ `[]`.

(Both are unit-agnostic; production unit = tokens, documented in the doc comment, applied by S2.)

### Success Criteria

- [ ] `internal/git/waterfill.go` exists, `package git`, imports ONLY stdlib (`sort`) — no context/I/O/git.
- [ ] `waterFillLevel(sizes, budget) (level int, truncate bool)` implements the §6 sort-and-walk: `(maxSize,
      false)` when `sum ≤ budget`; `(remaining/count, true)` (floor) at the walk's break point; `(0, false)`
      for empty sizes. Does NOT mutate the caller's slice (sort a copy).
- [ ] `allocByWaterFill(sizes, budget) []int` delegates to `waterFillLevel` and returns `min(sizes[i], level)`
      in INPUT order (parallel to sizes). Empty sizes ⇒ `[]`.
- [ ] The 3 verified numeric cases pass with HARDCODED expectations: `([10,20,30],30)→(10,true)`+allot
      `[10,10,10]`; `([5,100],50)→(45,true)`+`[5,45]`; `([10,10],50)→(10,false)`+`[10,10]`.
- [ ] All §6 edge cases pass: empty sizes; total ≤ budget; all-equal with n·S>budget (`[7,7,7]@10→(3,true)`+
      `[3,3,3]`); one file larger than budget; single file larger than budget (`[100]@30→(30,true)`+`[30]`);
      B=0 (`[5,10]@0→(0,true)`+`[0,0]`); UNSORTED input order preservation (`allocByWaterFill([100,5],50)→[45,5]`).
- [ ] A randomized property loop (deterministic seed, non-negative sizes/budgets, ≥1000 cases) asserts:
      (1) `sum(allots) ≤ budget`; (2) `allots[i] ≤ sizes[i]` ∀i; (3) `allots[i] ≥ 0` ∀i; (4) if not truncated,
      `allots[i] == sizes[i]` ∀i; (5) if truncated, `0 ≤ (budget − sum(allots)) < len(sizes)`.
- [ ] Doc comments cite FR3i, §6, the unit-agnostic + production-token note, the return contract, and the
      frozen-signature consumer contract (S2/M4.T3).
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l internal/git/` clean; ONLY the 2
      new files differ (`git status`); numstat.go/skeleton.go/tokens.go/the diff functions/StagedDiffOptions
      UNCHANGED (`git diff --exit-code` empty for each).

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact §6 sort-and-walk
pseudocode + the 3 verified numeric cases (quoted verbatim below + in research/design-decisions.md F1–F7),
the precise return contract (F2), the order-preservation requirement (F3), the integer-slack invariant (F4),
the full edge-case list (F5), the placement + naming convention (F6), the copy-ready skeletons in the
Implementation Blueprint, and the test pattern to mirror (tokens_test.go, quoted). No git/numstat/diff/prompt
knowledge required — the solver is pure arithmetic.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE algorithm + verified cases
- docfile: plan/007_b33d310438c6/architecture/git_diff_semantics.md
  section: "## 6. Water-fill / water-filling truncation" (Algorithm & correctness; Reference implementation
       — O(n log n) sort-and-walk; Edge cases; Verification cases).
  why: §6 IS the spec — the sort-and-walk pseudocode, the 3 verified numeric cases ([10,20,30]@30→[10,10,10];
       [5,100]@50→[5,45]; [10,10]@50→no truncation), and every edge case (B=0, one-file-larger, all-equal,
       total≤budget). Transcribe the walk FAITHFULLY (the break-point test is `s[i]*count ≤ remaining`).
  critical: the "if Σ s_i ≤ B: keep all whole; done" check MUST be first (the walk's L is undefined at the
       boundary otherwise). Integer floor division (`L = remaining / count`); the OPTIONAL round-robin
       remainder spending is NOT implemented (the contract's floor form). Files i..n-1 (count = n−i) share L.

# MUST READ — the design decisions (unit, return contract, slack, placement, test plan)
- docfile: plan/007_b33d310438c6/P1M4T2S1/research/design-decisions.md
  why: F1 (unit-agnostic solver; production unit = tokens applied by S2 — do NOT derive sizes here), F2 (the
       exact return contract: (maxSize,false) / (remaining/count,true) / (0,false) for empty), F3 (allocByWaterFill
       PRESERVES INPUT ORDER — S2 maps by index; delegates to waterFillLevel), F4 (floor division; slack =
       remaining mod count < N; the property-test target), F5 (every edge case + the unsorted-input regression),
       F6 (placement internal/git/waterfill.go + _test.go, mirrors tokens.go convention; pure stdlib sort only),
       F7 (the consumer contracts S2/M4.T3 — frozen signatures, do NOT implement).
  critical: F1 (don't derive sizes/budget — that's S2/M4.T3), F3 (order preservation — output parallels input),
       F4 (the slack invariant the property test asserts), F7 (signatures are frozen for downstream).

# MUST READ — the test pattern to mirror (table-driven, HARDCODED expectations)
- file: internal/git/tokens_test.go   (READ — mirror its style; do NOT edit)
  section: TestEstimateTokens — a `tests := []struct{in; want; desc}` table with HARDCODED `want` (never
       derived from the function — "that would be circular"), run via `t.Run(tc.desc, …)`. The doc comment
       pins the contract ("pins the … contract … Expectations are HARDCODED … Pure table test; no git repo,
       no I/O").
  why: this task's waterfill_test.go mirrors EXACTLY this style — table-driven, hardcoded expectations, t.Run
       subtests, pure (no git repo, no I/O). Copy the shape; the property loop is an addition (see F7/D7).
  pattern: `tests := []struct{ sizes []int; budget; wantLevel; wantTrunc; desc string}{…}`; loop with
       `t.Run(tc.desc, func(t *testing.T){ gotLevel, gotTrunc := waterFillLevel(tc.sizes, tc.budget); … })`.

# MUST READ — the consumer contract (S2 calls these signatures; do NOT change them)
- docfile: plan/007_b33d310438c6/P1M4T1S2/PRP.md
  section: the "OUTPUT (downstream consumer)" note: M4.T3 reads opts.PromptReserveTokens + opts.TokenLimit;
       when TokenLimit>0 it passes the reserve into "M4.T2's body_budget = token_limit − skeleton − reserve − margin".
  why: confirms WHERE the budget comes from (M4.T3) and that S2 derives the per-file token sizes (EstimateTokens
       per captured body). This task's solver sits BETWEEN them: S2 builds sizes+budget → solver → S2 applies.
       The signatures `waterFillLevel([]int,int)(int,bool)` and `allocByWaterFill([]int,int)[]int` are the
       frozen seam. Do NOT add params (e.g. a "unit" arg) — downstream depends on these exact shapes.
  critical: do NOT implement body_budget computation or size derivation — they are S2/M4.T3. The solver takes
       already-computed ints.

# READ — the size source S2 will use (this task does NOT touch it; context only)
- file: internal/git/numstat.go   (READ ONLY — do NOT edit)
  section: `type numstatRow struct{ Added, Deleted int; IsBinary bool; Path string }` — the per-file size
       source. S2 converts each row to a token size (EstimateTokens of the captured body, or (Added+Deleted)
       × avg-tokens-per-line). `numstatRows` is the `git diff … --numstat` parser (P1.M3.T1.S1).
  why: documents the input S2 will build the `sizes` slice from. This task's solver is agnostic to it — but
       the doc comment should note "production sizes come from numstatRows × EstimateTokens (S2)" so readers
       understand the unit.
  gotcha: do NOT import or call numstatRows/numstatRow in waterfill.go (the solver is pure; S2 does the
       conversion). numstatRow is unexported anyway.

# READ — the estimator whose UNIT production uses (this task does NOT call it)
- file: internal/git/tokens.go   (READ ONLY — do NOT edit)
  section: `func EstimateTokens(s string) int` = ceil(runes/4) (P1.M4.T1.S1). The SINGLE model-agnostic
       estimator; "consumed by … the FR3i water-fill sizing/truncation (P1.M4.T2)."
  why: confirms the production unit (tokens, via this estimator) and that M4.T2 (this milestone) is its
       water-fill consumer. The doc comment cites it as the unit S2 uses; this task does NOT call it.
  gotcha: do NOT call EstimateTokens in waterfill.go — the solver is unit-agnostic; S2 applies the estimator
       to build the sizes slice. (Calling it here would couple the pure solver to a specific unit.)

- url: (PRD §9.1 FR3i — in context as selected_prd_content h3.17; ALSO plan/007_b33d310438c6/prd_snapshot.md §9.1 FR3i)
  why: FR3i is the AUTHORITATIVE feature contract — "dynamic, size-aware water-fill … no static per-file
       cap … find the water level L such that Σ min(size_i, L) = body_budget … every file smaller than L is
       included whole … every file larger than L is truncated to exactly L." Guarantees (a)–(d) map to the
       property-test invariants.
  critical: FR3i's "body_budget = token_limit − skeleton − prompt − margin" is M4.T3's computation (the
       gate) — NOT this task. This task's `budget` param RECEIVES that value. FR3i's "compute each file's
       body size up front as a token estimate derived from the numstat skeleton" is S2's conversion.
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  tokens.go / tokens_test.go        # P1.M4.T1.S1 — EstimateTokens (ceil(runes/4)). READ ONLY (the UNIT; not called here).
  numstat.go / numstat_test.go      # P1.M3.T1.S1 — numstatRow{Added,Deleted,IsBinary,Path} + numstatRows parser. READ ONLY (S2's size source).
  skeleton.go / skeleton_test.go    # P1.M3.T1.S2 — renderNumstatSkeleton. READ ONLY.
  binary.go / binary_test.go        # FR3a/b/c binary filter + placeholder. READ ONLY (the test-style sibling).
  git.go                            # StagedDiff/TreeDiff/WorkingTreeDiff + StagedDiffOptions. UNCHANGED (M4.T3's territory).
  waterfill.go                      # *** CREATE *** — waterFillLevel + allocByWaterFill (pure; stdlib sort only).
  waterfill_test.go                 # *** CREATE *** — exhaustive table + randomized property loop.
go.mod / go.sum                     # UNCHANGED (stdlib sort only; no new deps).
```

### Desired Codebase tree with files to be added/changed

```bash
internal/git/waterfill.go           # NEW — waterFillLevel + allocByWaterFill (pure; stdlib `sort`). Doc: FR3i/§6/unit/order/frozen-consumer.
internal/git/waterfill_test.go      # NEW — exhaustive §6 table (hardcoded) + 3 verified cases + edge cases + order-preservation + randomized property loop.
# NO other files changed. go.mod/go.sum UNCHANGED. numstat.go/skeleton.go/tokens.go/git.go UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (unit-agnostic; do NOT derive sizes/budget here — F1): the solver takes []int + int and does pure
//   arithmetic. Production unit = TOKENS (S2 builds sizes via EstimateTokens of each captured body; M4.T3
//   computes body_budget = token_limit − skeleton − reserve − margin). This task does NEITHER conversion — it
//   just solves. Calling EstimateTokens/numstatRows here would couple the pure solver to a unit + a git call.

// CRITICAL (the no-truncation check MUST be first — §6/F2): if sum(sizes) ≤ budget, return (maxSize, false)
//   IMMEDIATELY. The walk's L is undefined at the boundary (remaining/count when no break occurs). maxSize =
//   max(sizes) (0 for empty) so allocByWaterFill's min(size_i, maxSize) = size_i ⇒ every file whole.

// CRITICAL (allocByWaterFill PRESERVES INPUT ORDER — F3): out[i] = min(sizes[i], level) for each i in INPUT
//   order. NOT sorted order. S2 reassociates allotments to files BY INDEX (sizes[i] ↔ file i). A sorted output
//   would scramble that mapping. Regression test: allocByWaterFill([100,5],50) → [45,5], NOT [5,45].

// CRITICAL (sort a COPY — do NOT mutate the caller's slice): waterFillLevel sorts sizes ascending to walk, but
//   the caller's slice must be unchanged (Go slices are references; sorting in place would surprise S2, which
//   reuses sizes for the min(size_i, level) map in allocByWaterFill). Copy: s := append([]int(nil), sizes…);
//   sort.Ints(s).

// CRITICAL (INTEGER floor division — F4): level = remaining / count (Go integer division = floor for the
//   non-negative production values). Do NOT implement §6's OPTIONAL round-robin remainder spending (the
//   contract's level=remaining/count is the floor form; S2 truncates to exactly level). slack = budget −
//   Σ allotments = remaining mod count < count ≤ N ⇒ the property test asserts 0 ≤ slack < N when truncated.

// GOTCHA (B=0 edge — §6): [5,10]@0 → sum 10 > 0 → walk: i=0, count=2, 5*2=10 > 0 → level=0/2=0, truncate=true.
//   allot [0,0]. §6: "L=0; every file truncated to zero; degenerate; usually guarded out" (M4.T3 guards
//   budget>0, but the solver handles 0 correctly). Table-test it.

// GOTCHA (single file larger than budget): [100]@30 → sum 100 > 30 → walk: i=0, count=1, 100*1=100 > 30 →
//   level=30/1=30, truncate=true. allot [30]. Never negative (sum>budget guarantees headroom).

// GOTCHA (all-equal, n·S > budget): [7,7,7]@10 → sum 21 > 10 → walk: i=0, count=3, 7*3=21 > 10 → level=10/3=3
//   (floor), truncate=true. allot [3,3,3], Σ=9, slack=1<3=N. The integer-slack case — table-test it.

// GOTCHA (property test must use NON-NEGATIVE sizes/budgets): production values are non-negative (token counts).
//   If using testing/quick (which generates full-range ints), clamp to abs; PREFER a manual math/rand loop with
//   a fixed seed + non-negative values for deterministic, controlled property testing (D7).

// GOTCHA (frozen signatures — F7): waterFillLevel([]int,int)(int,bool) and allocByWaterFill([]int,int)[]int are
//   the seam S2/M4.T3 call. Do NOT add params (a "unit" arg, a sentinel string, etc.) — downstream depends on
//   these exact shapes. Return `level int, truncate bool` (named returns are fine but the TYPES are the contract).

// GOTCHA (do NOT touch internal/git/git.go or the diff functions): the 3 diff functions + StagedDiffOptions are
//   M4.T3's territory (the gate). numstat.go/skeleton.go/tokens.go are siblings' frozen territory. This task
//   ADDS 2 files only.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/git/waterfill.go
package git

import "sort"

// waterFillLevel is the FR3i dynamic water-fill level solver (PRD §9.1 FR3i; architecture/git_diff_semantics.md §6):
// the classic water-filling / max-min-fair-with-caps allocation. Given per-file sizes and a budget, it finds
// the water level L such that Σ min(size_i, L) = budget: files smaller than L are kept whole; files larger
// than L are truncated to exactly L (their unused budget is reclaimed and redistributed to the large files).
//
// UNIT-AGNOSTIC + PURE. sizes and budget are ints in ANY consistent unit; in PRODUCTION (FR3i) the unit is
// TOKENS — sizes are derived from numstatRows via EstimateTokens (applied by the S2 consumer to each file's
// captured body), and budget is body_budget = token_limit − skeleton − reserve − margin (computed by the
// M4.T3 gate). This function does NEITHER conversion — it just solves the allocation over ints. No git, no
// I/O, no context; stdlib `sort` only.
//
// RETURN CONTRACT (§6):
//   - sum(sizes) ≤ budget (or sizes empty): return (maxSize, false) where maxSize = max(sizes) (0 for empty).
//     No truncation. (maxSize ⇒ allocByWaterFill's min(size_i, maxSize) = size_i ⇒ every file whole.)
//   - sum(sizes) > budget: sort ascending and walk; at the break point i where sizes[i]*count > remaining
//     (count = n−i), return (remaining / count, true) — INTEGER floor division (no round-robin remainder
//     spending; S2 truncates to exactly level). Files 0..i−1 are served whole; files i..n−1 share level L.
//
// The caller's slice is NOT mutated (a copy is sorted). Signatures are FROZEN — consumed by S2
// (P1.M4.T2.S2, per-file truncation application) and M4.T3 (P1.M4.T3.S1, the token-limit gate).
func waterFillLevel(sizes []int, budget int) (level int, truncate bool) {
	total := 0
	maxSize := 0
	for _, s := range sizes {
		total += s
		if s > maxSize {
			maxSize = s
		}
	}
	if total <= budget {
		return maxSize, false // everything fits (or empty) — no truncation
	}
	// Sort a COPY (do NOT mutate the caller's slice) and walk per §6.
	s := append([]int(nil), sizes...)
	sort.Ints(s)
	remaining := budget
	n := len(s)
	for i := 0; i < n; i++ {
		count := n - i // files still to allocate (all ≥ s[i] since sorted)
		if s[i]*count <= remaining {
			remaining -= s[i] // file i is "small": served whole
			continue
		}
		return remaining / count, true // water level for files i..n-1 (floor division)
	}
	// Unreachable when total > budget (the walk always breaks), but return a safe fallback defensively.
	return 0, true
}

// allocByWaterFill returns each file's allotment under the FR3i water-fill (PRD §9.1 FR3i; §6):
// out[i] = min(sizes[i], level) where level = waterFillLevel(sizes, budget). The output is PARALLEL to the
// input — it PRESERVES THE ORIGINAL INPUT ORDER (the caller reassociates allotments to files BY INDEX;
// the output is NOT sorted). When no truncation (sum ≤ budget), level = max(sizes) ⇒ out[i] = sizes[i]
// (every file whole). When truncated, small files (size_i ≤ level) get size_i (whole); large files get level.
//
// UNIT-AGNOSTIC + PURE (see waterFillLevel). Delegates to waterFillLevel (DRY — single source of the level).
// Signature FROZEN — consumed by S2 (P1.M4.T2.S2). Empty sizes ⇒ [].
func allocByWaterFill(sizes []int, budget int) []int {
	level, _ := waterFillLevel(sizes, budget)
	out := make([]int, len(sizes))
	for i, s := range sizes {
		if s < level {
			out[i] = s
		} else {
			out[i] = level
		}
	}
	return out
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/git/waterfill.go (the solver — pure, stdlib sort only)
  - FILE: NEW internal/git/waterfill.go. PACKAGE: `package git`. IMPORT: `sort` ONLY (no context, no I/O,
      no other internal/git symbols referenced).
  - DEFINE: `func waterFillLevel(sizes []int, budget int) (level int, truncate bool)` + `func allocByWaterFill(
      sizes []int, budget int) []int` — EXACTLY as in "Data models" (paste + adapt the doc comments).
  - GOTCHA: sort a COPY (append([]int(nil), sizes...)); do NOT mutate the caller's slice (F3/CRITICAL). The
      no-truncation check (total ≤ budget → (maxSize, false)) MUST precede the walk (§6). level = remaining/count
      is INTEGER floor division (F4). The walk's unreachable fallback returns (0, true) defensively.
  - GOTCHA: allocByWaterFill uses `if s < level { out[i] = s } else { out[i] = level }` (min) in INPUT order
      (NOT sorted — F3). Delegates to waterFillLevel (DRY). Empty sizes → out is len-0 (make([]int, 0)).
  - RUN: gofmt -w internal/git/waterfill.go ; go build ./internal/git/ → exit 0.

Task 2: CREATE internal/git/waterfill_test.go (exhaustive table + randomized property loop)
  - FILE: NEW internal/git/waterfill_test.go. PACKAGE: `package git` (white-box — same package, like
      tokens_test.go). IMPORT: `math/rand`, `sort", `testing" (stdlib only; NO internal imports — the test is pure).
  - PATTERN: mirror tokens_test.go's table-driven style — `tests := []struct{…}{…}` with HARDCODED expectations
      (never derived from the function — circular), run via `t.Run(tc.desc, …)`.
  - TABLE CASES (waterFillLevel — exact level + truncate):
      * TestWaterFillLevel_NoTruncation_TotalLE: [10,10]@50 → (10, false); [1,2,3]@100 → (3, false);
        [5]@5 → (5, false) (total == budget ⇒ no truncation, level = max).
      * TestWaterFillLevel_Verified_10_20_30_at_30: [10,20,30]@30 → (10, true).
      * TestWaterFillLevel_Verified_5_100_at_50: [5,100]@50 → (45, true).
      * TestWaterFillLevel_AllEqual_Split: [7,7,7]@10 → (3, true) (floor(10/3); Σ=9, slack 1<3).
      * TestWaterFillLevel_SingleFileLargerThanBudget: [100]@30 → (30, true).
      * TestWaterFillLevel_OneFileLarger: [10,10,100]@50 → walk: i=0 count=3 10*3=30≤50 rem=40; i=1 count=2
        10*2=20≤40 rem=30; i=2 count=1 100*1=100>30 → (30, true). allot [10,10,30], Σ=50.
      * TestWaterFillLevel_BudgetZero: [5,10]@0 → (0, true).
      * TestWaterFillLevel_Empty: []@50 → (0, false); []@0 → (0, false).
      * TestWaterFillLevel_DoesNotMutateInput: sizes=[3,1,2]; call; assert sizes == [3,1,2] (caller slice intact).
  - TABLE CASES (allocByWaterFill — exact allot slice, ORDER PRESERVED):
      * TestAllocByWaterFill_Verified_10_20_30_at_30: [10,20,30]@30 → [10,10,10].
      * TestAllocByWaterFill_Verified_5_100_at_50: [5,100]@50 → [5,45].
      * TestAllocByWaterFill_NoTruncation: [10,10]@50 → [10,10] (whole).
      * TestAllocByWaterFill_OrderPreserved_Unsorted: [100,5]@50 → [45,5] (NOT [5,45] — input order kept).
        ALSO [30,10,20]@30 → [10,10,10] (input order, all capped to 10).
      * TestAllocByWaterFill_AllEqual: [7,7,7]@10 → [3,3,3].
      * TestAllocByWaterFill_BudgetZero: [5,10]@0 → [0,0].
      * TestAllocByWaterFill_Empty: []@50 → [] (len 0).
      * TestAllocByWaterFill_LengthParity: for several cases, len(out) == len(sizes).
  - PROPERTY LOOP (TestWaterFill_PropertyInvariants — randomized, deterministic seed, ≥1000 cases):
        r := rand.New(rand.NewSource(1)) // fixed seed — deterministic
        for iter := 0; iter < 1000; iter++ {
            n := r.Intn(10)              // 0..9 files
            sizes := make([]int, n)
            for i := range sizes { sizes[i] = r.Intn(200) } // 0..199 tokens each
            budget := r.Intn(300)        // 0..299 tokens
            level, trunc := waterFillLevel(sizes, budget)
            allots := allocByWaterFill(sizes, budget)
            // (1) length parity
            if len(allots) != len(sizes) { t.Fatalf("len mismatch") }
            sum := 0
            for i, s := range sizes {
                // (2) allotment_i ≤ size_i  (3) allotment_i ≥ 0  (4) no-trunc ⇒ allotment_i == size_i
                if allots[i] < 0 || allots[i] > s { t.Fatalf("allot out of [0,size]") }
                if !trunc && allots[i] != s { t.Fatalf("no-trunc but file truncated") }
                sum += allots[i]
            }
            // (1) Σ allotments ≤ budget
            if sum > budget { t.Fatalf("over-budget") }
            // (5) if truncated, 0 ≤ slack < N
            if trunc {
                slack := budget - sum
                if slack < 0 || slack >= len(sizes) { t.Fatalf("slack %d out of [0,N=%d)", slack, len(sizes)) }
            }
            // (parity) level consistency: when truncated, every allot is min(size, level); when not, == size
            ...
        }
      NOTE: when n==0, waterFillLevel returns (0,false), allots is [], sum=0 ≤ budget, trunc==false ⇒ skip the
      slack check. The loop covers empty, B=0, all-equal, skewed — all non-negative. Fixed seed ⇒ reproducible.
  - GOTCHA: HARDCODE all table expectations (don't compute want via the function — circular). The property loop
      is the only "derived" assertions, and it asserts INVARIANTS (not exact values). Use `sort.Ints` in the
      parity check if needed. Keep it pure (no t.TempDir, no git).
  - RUN: gofmt -w internal/git/waterfill_test.go ; go test ./internal/git/ -run 'WaterFill|AllocByWaterFill' -v.

Task 3: VALIDATE (run all gates; fix before declaring done)
  - gofmt -w internal/git/waterfill.go internal/git/waterfill_test.go
  - go vet ./internal/git/ && go build ./...
  - go test ./internal/git/ -v -run 'WaterFill|AllocByWaterFill'   (the new table + property tests)
  - go test ./...   (ALL green — pure additions, no consumer reads them yet ⇒ no behavior change, no regression.)
  - git status → expect EXACTLY 2 new files (internal/git/waterfill.go, internal/git/waterfill_test.go).
  - git diff --exit-code internal/git/git.go internal/git/numstat.go internal/git/skeleton.go internal/git/tokens.go
      internal/git/binary.go go.mod go.sum → empty (frozen files UNCHANGED).
  - ! grep -q 'context\|os\.\|exec\.' internal/git/waterfill.go   (confirm pure: no context/I/O/exec.)
  - ! grep -q 'EstimateTokens\|numstatRow\|numstatRows' internal/git/waterfill.go   (confirm unit-agnostic: no estimator/numstat coupling.)
```

### Implementation Patterns & Key Details

```go
// PATTERN: sort-and-walk (§6, transcribe faithfully). The break-point test is s[i]*count ≤ remaining.
//   total ≤ budget ⇒ (maxSize, false) FIRST. Else sort a COPY, walk; at the break return (remaining/count, true).
//   count = n - i (files still to allocate, all ≥ s[i] since sorted).

// PATTERN: allocByWaterFill delegates + preserves order (F3). out[i] = min(sizes[i], level) in INPUT order.
//   level, _ := waterFillLevel(sizes, budget)
//   for i, s := range sizes { if s < level { out[i] = s } else { out[i] = level } }

// PATTERN: sort a copy — never mutate the caller's slice.
//   s := append([]int(nil), sizes...); sort.Ints(s)

// CRITICAL: integer floor division (remaining / count). NO round-robin remainder spending. slack =
//   remaining mod count < count ≤ N ⇒ the property invariant.

// CRITICAL: the no-truncation check MUST be first (the walk's L is undefined at the total==budget boundary).

// GOTCHA: signatures are FROZEN (S2/M4.T3 consumers). Do NOT add params. Return (level int, truncate bool) /
//   []int exactly. Named returns are fine; the TYPES are the contract.

// GOTCHA: unit-agnostic — do NOT call EstimateTokens/numstatRows (S2 applies the unit). The solver is pure
//   int arithmetic. The doc comment documents the production-token unit for readers.

// GOTCHA: property test uses NON-NEGATIVE sizes/budgets (production values). A manual math/rand loop with a
//   FIXED seed is deterministic + controlled (prefer over testing/quick's full-range generation, D7).
```

### Integration Points

```yaml
SOLVER (internal/git/waterfill.go):
  - +func waterFillLevel(sizes []int, budget int) (level int, truncate bool)   (FROZEN signature)
  - +func allocByWaterFill(sizes []int, budget int) []int                       (FROZEN signature; order-preserving)

CONSUMER.S2 (P1.M4.T2.S2 — per-file truncation application; DO NOT implement here):
  - call: "S2 builds sizes (EstimateTokens per captured file body) + body_budget (from M4.T3), calls
    waterFillLevel/allocByWaterFill, then truncates each file's body to allots[i] tokens (first level tokens
    + the `... [truncated]` sentinel) when allots[i] < sizes[i], preserving diff --git/hunk headers."

CONSUMER.M4.T3 (P1.M4.T3.S1 — the token-limit gate; DO NOT implement here):
  - call: "M4.T3 computes body_budget = token_limit − skeleton − reserve − margin (reserve from P1.M4.T1.S2),
    and when token_limit > 0 triggers the water-fill (passes body_budget as the solver's budget param)."

UNIT (tokens — applied by S2, NOT here):
  - production: "sizes = EstimateTokens(file body) per file; budget = body_budget in tokens. The solver is
    unit-agnostic (pure int arithmetic); the doc comment documents this. do NOT call EstimateTokens here."

GO.MODULE: change NONE. stdlib `sort` (waterfill.go) + `math/rand`/`sort`/`testing` (waterfill_test.go) only.

FROZEN/LEAVE (do NOT edit):
  - internal/git/git.go (StagedDiff/TreeDiff/WorkingTreeDiff + StagedDiffOptions — M4.T3's territory).
  - internal/git/{numstat,skeleton,tokens,binary}.go (siblings' frozen territory — READ only).
  - internal/prompt/reserve.go (P1.M4.T1.S2 — the reserve source). go.mod/go.sum. The 6 diff call sites.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/git/waterfill.go internal/git/waterfill_test.go
go vet ./internal/git/
# Confirm purity (no context/I/O/exec) + unit-agnosticism (no estimator/numstat coupling):
! grep -qE 'context\.|os\.|exec\.' internal/git/waterfill.go   && echo "pure (no I/O) ✓"
! grep -qE 'EstimateTokens|numstatRow|numstatRows' internal/git/waterfill.go   && echo "unit-agnostic ✓"
# Confirm the signatures are exactly the frozen contract:
grep -n 'func waterFillLevel\|func allocByWaterFill' internal/git/waterfill.go
#   expect: func waterFillLevel(sizes []int, budget int) (level int, truncate bool)
#           func allocByWaterFill(sizes []int, budget int) []int
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; pure + unit-agnostic confirmed; signatures match the frozen contract.
```

### Level 2: Unit tests (the solver table + property loop)

```bash
# The 3 verified numeric cases (§6) — MUST pass with hardcoded expectations:
go test ./internal/git/ -run 'TestWaterFillLevel_Verified|TestAllocByWaterFill_Verified' -v
# Expected: [10,20,30]@30→(10,true)/[10,10,10]; [5,100]@50→(45,true)/[5,45]; [10,10]@50→(10,false)/[10,10].

# The full edge-case table:
go test ./internal/git/ -run 'TestWaterFillLevel|TestAllocByWaterFill' -v
# Expected: no-truncation, all-equal-split, single-file-larger, one-file-larger, B=0, empty,
#           order-preservation ([100,5]@50→[45,5]), does-not-mutate-input — all PASS.

# The randomized property loop (deterministic seed, 1000 cases, the 5 invariants):
go test ./internal/git/ -run TestWaterFill_PropertyInvariants -v
# Expected: PASS (Σ≤budget; allot∈[0,size]; no-trunc⇒allot==size; trunc⇒0≤slack<N; length parity).

# Full internal/git suite (no regression — pure additions):
go test ./internal/git/ -v
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...     # Expect clean.
go test ./...      # Expect all PASS — pure additions; no consumer reads the functions yet ⇒ no behavior change.
# Confirm ONLY the 2 new files differ:
git status --porcelain
# Expected: exactly 2: internal/git/waterfill.go, internal/git/waterfill_test.go.
# Confirm the frozen files are byte-unchanged:
git diff --exit-code internal/git/git.go internal/git/numstat.go internal/git/skeleton.go \
  internal/git/tokens.go internal/git/binary.go go.mod go.sum \
  && echo "frozen files UNCHANGED (expected)"
```

### Level 4: Correctness reasoning (the §6 verification block, reproducible by hand)

```bash
# No git/DB/subprocess. Verify the algorithm by reasoning + the Level-2 table tests:
#   1. [10,20,30]@30: total 60>30. sort [10,20,30]. i=0 count=3 10*3=30≤30 rem=20. i=1 count=2 20*2=40>20 → L=10.
#      allot [10,10,10] Σ=30=budget ✓ (the §6 verified case).
#   2. [5,100]@50: total 105>50. i=0 count=2 5*2=10≤50 rem=45. i=1 count=1 100*1=100>45 → L=45. allot [5,45] Σ=50 ✓.
#   3. [10,10]@50: total 20≤50 → (10,false) allot [10,10] ✓ (no truncation).
#   4. [7,7,7]@10: total 21>10. i=0 count=3 7*3=21>10 → L=floor(10/3)=3. allot [3,3,3] Σ=9 slack=1<3 ✓ (integer slack).
#   5. [100,5]@50: total 105>50. sort→[5,100]. i=0 count=2 5*2=10≤50 rem=45. i=1 count=1 100>45 → L=45.
#      allot (INPUT order [100,5]) → [min(100,45), min(5,45)] = [45,5] ✓ (order preserved — NOT [5,45]).
# All 5 are table-tested with hardcoded expectations in waterfill_test.go (Level 2).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l internal/git/` clean.
- [ ] `go test ./...` PASS (the new table + property tests; no repo-wide regression — pure additions).
- [ ] go.mod/go.sum byte-unchanged (`git diff --exit-code go.mod go.sum` empty).
- [ ] `git status` shows EXACTLY 2 new files; every frozen file byte-unchanged (git.go/numstat.go/skeleton.go/
      tokens.go/binary.go).

### Feature Validation
- [ ] `waterFillLevel` returns `(maxSize, false)` when `sum ≤ budget`; `(remaining/count, true)` (floor) when
      `sum > budget`; `(0, false)` for empty sizes.
- [ ] `allocByWaterFill` returns `min(sizes[i], level)` in INPUT order; empty sizes ⇒ `[]`.
- [ ] The 3 §6 verified cases pass: `[10,20,30]@30→(10,true)`/`[10,10,10]`; `[5,100]@50→(45,true)`/`[5,45]`;
      `[10,10]@50→(10,false)`/`[10,10]`.
- [ ] All §6 edge cases pass: empty; B=0 (`[5,10]@0→(0,true)`/`[0,0]`); single-file-larger (`[100]@30→(30,true)`/
      `[30]`); one-file-larger; all-equal-split (`[7,7,7]@10→(3,true)`/`[3,3,3]`).
- [ ] Order preservation: `allocByWaterFill([100,5],50) == [45,5]` (NOT `[5,45]`); input slice unmutated.
- [ ] The randomized property loop (1000 cases) asserts: `Σ≤budget`; `allot_i∈[0,size_i]`; `allot_i≥0`;
      no-trunc ⇒ `allot_i==size_i`; trunc ⇒ `0≤slack<N`.
- [ ] The solver is PURE (no context/I/O/exec) and UNIT-AGNOSTIC (no EstimateTokens/numstat coupling).

### Code Quality Validation
- [ ] `waterFillLevel` transcribes the §6 sort-and-walk faithfully (break test `s[i]*count ≤ remaining`).
- [ ] `allocByWaterFill` delegates to `waterFillLevel` (DRY) and preserves input order (F3).
- [ ] Sorts a copy (never mutates the caller's slice); integer floor division (no round-robin).
- [ ] Doc comments cite FR3i/§6, the unit-agnostic + production-token note, the return contract, and the
      frozen consumer signatures.
- [ ] Tests mirror tokens_test.go (table-driven, hardcoded expectations); the property loop is deterministic
      (fixed seed, non-negative values).
- [ ] Anti-patterns avoided (see below); no out-of-scope churn; no new dependency.

### Documentation
- [ ] Doc comments are self-documenting (the algorithm, the unit, the order-preservation, the frozen seam).
- [ ] No new env vars / config / CLI surface (DOCS clause: "none — internal algorithm").

---

## Anti-Patterns to Avoid

- ❌ **Don't derive sizes or compute body_budget in the solver.** The solver is pure int arithmetic; sizes
  (numstat→tokens) is S2's job and `body_budget = token_limit − skeleton − reserve − margin` is M4.T3's. Calling
  EstimateTokens/numstatRows here couples the pure solver to a unit + a git call (F1).
- ❌ **Don't skip the `total ≤ budget` first check.** The walk's L is undefined at the boundary (when no break
  occurs). Return `(maxSize, false)` first (§6/F2).
- ❌ **Don't return allotments in SORTED order.** `allocByWaterFill` MUST preserve input order — S2 maps
  allotments to files BY INDEX. Sorted output scrambles the mapping. Regression: `[100,5]@50→[45,5]`, not
  `[5,45]` (F3).
- ❌ **Don't mutate the caller's slice.** Sort a copy (`append([]int(nil), sizes...)`). Go slices are references;
  in-place sorting surprises the caller (which reuses `sizes` in `allocByWaterFill`'s min map).
- ❌ **Don't implement the OPTIONAL round-robin remainder spending (§6).** The contract's `level=remaining/count`
  is the floor form; S2 truncates to exactly `level`. Integer slack (`remaining mod count < N`) is acceptable
  and is the property-test target (F4).
- ❌ **Don't change the frozen signatures.** `waterFillLevel([]int,int)(int,bool)` and `allocByWaterFill([]int,int)[]int`
  are the seam S2/M4.T3 call. No extra params (a "unit" arg, a sentinel string) — downstream depends on these
  exact shapes (F7).
- ❌ **Don't use `testing/quick` blindly.** It generates full-range ints (negatives). Use a manual `math/rand`
  loop with a FIXED seed + non-negative sizes/budgets for deterministic, controlled property testing (D7).
- ❌ **Don't derive test expectations from the function.** Table `want` values are HARDCODED (deriving them via
  the function is circular — it couldn't catch a wrong formula). Mirror tokens_test.go.
- ❌ **Don't touch git.go/numstat.go/skeleton.go/tokens.go/binary.go or the diff functions.** They are siblings'
  frozen territory (M4.T3/S1/P1.M3). This task ADDS 2 files only.
- ❌ **Don't add the truncation application (sentinel, header preservation, `diff --git` splitting).** That's S2
  (P1.M4.T2.S2). This task is ONLY the solver.
