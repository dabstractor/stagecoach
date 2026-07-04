// FR3i dynamic water-fill truncation SOLVER (PRD §9.1 FR3i; architecture/git_diff_semantics.md §6).
//
// The classic water-filling / max-min-fair-with-caps allocation. Given per-file sizes and a budget, the
// solver finds the water level L such that Σ min(size_i, L) = budget: files smaller than L are kept whole;
// files larger than L are truncated to exactly L (their unused budget is reclaimed and redistributed to the
// large files). This file implements the two PURE, git-independent solver functions; the per-file
// truncation application (sentinel, header preservation) is the S2 consumer (P1.M4.T2.S2), and the
// body_budget computation (token_limit − skeleton − reserve − margin) is the M4.T3 gate (P1.M4.T.S1).

package git

import "sort"

// waterFillLevel is the FR3i dynamic water-fill level solver (PRD §9.1 FR3i; architecture/git_diff_semantics.md
// §6): the classic water-filling / max-min-fair-with-caps allocation. Given per-file sizes and a budget, it
// finds the water level L such that Σ min(size_i, L) = budget: files smaller than L are kept whole; files
// larger than L are truncated to exactly L (their unused budget is reclaimed and redistributed to the large
// files).
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
