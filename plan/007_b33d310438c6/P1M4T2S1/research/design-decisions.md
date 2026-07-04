# P1.M4.T2.S1 — Water-fill level solver: Design Decisions & Findings

> Companion to `../PRP.md`. The non-obvious design calls an implementer must internalize. The algorithm
> itself is specified authoritatively in `architecture/git_diff_semantics.md §6`; this file resolves the
> unit question, the exact return contract, the integer-division slack, the placement, and the test plan.

## The contract (PRD §9.1 FR3i; item_description; git_diff_semantics.md §6)

Two PURE functions in `internal/git`:
- `waterFillLevel(sizes []int, budget int) (level int, truncate bool)` — the sort-and-walk water-fill
  solver. Returns the water level `L` and whether any file is truncated.
- `allocByWaterFill(sizes []int, budget int) []int` — each file's allotment (`min(size_i, level)`),
  parallel to the input (preserves original order).

Edge cases verified in §6: `[10,20,30]@30→[10,10,10]`, `[5,100]@50→[5,45]`, `[10,10]@50→no truncation`,
one-file-larger-than-budget, `B=0`.

---

## F1 — The solver is UNIT-AGNOSTIC pure int arithmetic (the unit decision)

The contract's RESEARCH NOTE debates line-counts vs tokens at length and concludes "pick ONE consistent
unit (tokens throughout is cleanest) and document it." **Resolution:** the solver itself takes `sizes []int`
+ `budget int` and does pure arithmetic — it neither knows nor cares what unit the ints are. **Production
unit (documented in the doc comment, applied by the S2 consumer) = TOKENS**: sizes derived from numstatRows
via `git.EstimateTokens` applied to each file's captured body (the accurate path) or numstat
(added+deleted) × an avg-tokens-per-line factor; budget = `body_budget` in tokens. The solver's doc comment
states this; the conversion is S2's job (S2 has the captured bodies). **This keeps the solver pure and
fully testable independent of git** (the contract's OUTPUT clause: "pure functions … independent of git").

**Do NOT derive sizes or compute body_budget in this task** — that is S2 (numstat→token sizes) and M4.T3
(`body_budget = token_limit − skeleton − reserve − margin`). This task delivers ONLY the solver + tests.

## F2 — The return contract for `waterFillLevel`

- **No truncation** (`Σ sizes ≤ budget`): return `(maxSize, false)` where `maxSize = max(sizes)` (0 for an
  empty slice). Rationale: `allocByWaterFill` then computes `min(size_i, maxSize) = size_i` for every file
  ⇒ every file whole. The contract specifies exactly `(maxSize, false)`.
- **Truncation** (`Σ sizes > budget`): sort ascending, walk per §6; at the break point `i` where
  `s[i]*count > remaining` (count = n−i), return `(remaining / count, true)` — **integer (floor) division**.
- **Empty sizes**: `Σ = 0 ≤ budget` ⇒ `(0, false)` (covered by the no-truncation branch; maxSize of empty = 0).

## F3 — `allocByWaterFill` delegates to `waterFillLevel` and PRESERVES INPUT ORDER

`allocByWaterFill` calls `waterFillLevel(sizes, budget)` → `(level, _truncate)`, then returns
`[]int{ min(sizes[i], level) for i }` **in the ORIGINAL input order** (NOT sorted). This is load-bearing:
S2 maps the allotment slice back to files **by index** (sizes[i] ↔ file i's body). If the output were in
sorted order, S2 couldn't reassociate allotments with files. `min(size_i, level)` is correct in BOTH
branches: when `truncate=false`, `level=maxSize ≥ size_i` ⇒ `min = size_i` (whole); when `truncate=true`,
small files (`size_i ≤ level`) get `size_i` (whole), large files get `level` (truncated).

## F4 — Integer floor division; the slack invariant (the property-test target)

The contract specifies `level = remaining / count` (Go integer division = floor toward zero; for the
non-negative values in production this is plain floor). §6 mentions an OPTIONAL "round-robin remainder
spending" to use the budget fully — we do **NOT** do that (the contract's `level=remaining/count` is the
floor form; simpler, and S2 truncates to exactly `level`). The resulting slack:

`slack = budget − Σ allotments = remaining − count*floor(remaining/count) = remaining mod count`

which is `< count` (the count at the break point) and thus `< N` (total files). This IS the contract's
property clause: "if any file truncated then Σ allotments == budget (full utilization) OR the remainder is
< count (integer slack)." The property test asserts `0 ≤ slack < N` when truncated (N = len(sizes), a safe
upper bound on count_at_break). When not truncated, `Σ allotments == Σ sizes ≤ budget` (slack ≥ 0, no claim
on full utilization — correct, we deliberately don't fill the budget when nothing needs truncating).

Verified numeric cases (all zero-slack): `[10,20,30]@30→Σ=30=budget`; `[5,100]@50→Σ=50=budget`;
`[10,10]@50→no truncation, Σ=20`. Integer-slack case: `[7,7,7]@10→level=3, allot=[3,3,3], Σ=9, slack=1<3`.

## F5 — Edge cases (all from §6, all table-tested)

- **Empty sizes**: `waterFillLevel([]) → (0, false)`; `allocByWaterFill([]) → []`.
- **Total ≤ budget (no truncation)**: `[10,10]@50 → (10, false)`, allot `[10,10]`. MUST be checked first
  (the walk's L is undefined at the boundary otherwise).
- **All equal sizes, n·S > budget**: `[7,7,7]@10 → (3, true)`, allot `[3,3,3]` (even split at floor(B/n)).
- **One file larger than budget**: `[5,100]@50 → (45, true)`, allot `[5,45]` (small whole, big gets reclaim).
- **Single file larger than budget**: `[100]@30 → (30, true)`, allot `[30]`.
- **B=0, non-empty**: `[5,10]@0 → (0, true)`, allot `[0,0]` (§6: "L=0; every file truncated to zero;
  degenerate; usually guarded out" — M4.T3 guards budget>0, but the solver handles 0 correctly).
- **Unsorted input (order preservation)**: `allocByWaterFill([100,5],50) → [45,5]` (NOT `[5,45]`) —
  output parallels the input. **Critical regression test** (F3).

## F6 — Placement + naming (mirror internal/git's one-concept-per-file convention)

New file `internal/git/waterfill.go` + `internal/git/waterfill_test.go` — mirrors `tokens.go`/`tokens_test.go`,
`numstat.go`/`numstat_test.go`, `skeleton.go`/`skeleton_test.go`, `binary.go`/`binary_test.go` (one concept
per file, paired test). Package `git`. The functions are PURE (no `context`, no I/O, no git binary, no
imports beyond stdlib `sort`). The doc comment states the unit-agnosticism + the production-token-unit note
(F1) so future readers don't think the solver is git-coupled.

## F7 — The consumer contracts (S2 + M4.T3) — DO NOT implement, just satisfy

- **S2 (P1.M4.T2.S2 — per-file truncation application)** calls `waterFillLevel`/`allocByWaterFill` with
  token sizes (EstimateTokens per captured body) + `body_budget`, then truncates each file's body to its
  allotment (first `level` tokens + the `... [truncated]` sentinel), preserving `diff --git`/hunk headers.
- **M4.T3 (P1.M4.T3.S1 — the gate)** computes `body_budget = token_limit − skeleton − reserve − margin`
  (reserve from P1.M4.T1.S2) and triggers the water-fill when `token_limit > 0`.
Both consume the signatures this task freezes: `waterFillLevel([]int, int) (int, bool)` and
`allocByWaterFill([]int, int) []int`. Do NOT change these signatures (downstream contract). Do NOT implement
S2/M4.T3.

---

## D1–D7 — Decision summary (maps to PRP contract clauses)

- **D1 (unit-agnostic solver; production unit = tokens):** pure int arithmetic; sizes/budget are "any
  consistent unit"; doc comment says production = tokens via EstimateTokens (applied by S2). No derivation
  in this task.
- **D2 (return contract):** `(maxSize, false)` when `Σ ≤ budget`; `(remaining/count, true)` (floor) when
  `Σ > budget`; `(0, false)` for empty.
- **D3 (order preservation):** `allocByWaterFill` returns `min(size_i, level)` in INPUT order (S2 maps by
  index). Delegates to `waterFillLevel` (DRY).
- **D4 (floor division; no round-robin):** `level = remaining/count` (Go floor); slack = `remaining mod
  count < N`; property test asserts `0 ≤ slack < N` when truncated.
- **D5 (edge cases):** empty, total≤budget, all-equal, one-large, single-large, B=0, unsorted-input — all
  table-tested with HARDCODED expectations.
- **D6 (placement):** `internal/git/waterfill.go` + `_test.go`; package `git`; pure (stdlib `sort` only).
- **D7 (testing):** exhaustive table (all §6 cases + 3 verified + integer-slack + order-preservation) +
  randomized property loop (deterministic seed, non-negative sizes/budgets, ~1000 cases) asserting the 5
  invariants. Mirror `tokens_test.go`'s hardcoded-expectation style.

## SCOPE BOUNDARY (owned by siblings — do NOT implement)
- Size derivation numstat→tokens (S2 / P1.M4.T2.S2). body_budget computation (M4.T3 / P1.M4.T3.S1).
- Per-file truncation application, sentinel, header preservation, `diff --git` splitting (S2).
- The token-limit gate logic in the 3 diff functions (M4.T3). estimateTokens (S1 / P1.M4.T1.S1 — frozen).
- numstat.go / skeleton.go / tokens.go / the diff functions / StagedDiffOptions — UNCHANGED.
