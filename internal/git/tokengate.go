// FR3d/FR3i token-limit GATE (PRD §9.1 FR3d/FR3i; architecture/system_context.md §5 the FR3i coupling
// seam + §6 the regression invariants).
//
// The gate chooses the truncation STRATEGY for the three sibling diff functions (StagedDiff/TreeDiff/
// WorkingTreeDiff in git.go) based on opts.TokenLimit:
//
//   - ==0 (unset): the EXISTING legacy per-section caps (per-file markdown line-cap `max_md_lines` +
//     non-markdown aggregate byte-cap `max_diff_bytes` + their `... [diff truncated at N bytes/lines]`
//     sentinels) apply UNCHANGED — byte-identical to pre-M4 (system_context §6 invariant 1 — the
//     regression anchor; the FR3e/-M, FR3f/-U<n>, FR3g skeleton-prepend, FR3h index-strip transforms from
//     M2/M3 still apply around them).
//   - >0  (set):   a dynamic water-fill REPLACES both caps (FR3d: "a non-zero token_limit supersedes both
//     legacy caps"). body_budget = max(0, token_limit − EstimateTokens(skeleton) − promptReserve − margin);
//     each file's body is sized with EstimateTokens; allocByWaterFill allots the budget; truncateByWaterFill
//     applies the per-file level (emitting the shorter `... [truncated]` sentinel per truncated file —
//     system_context §6 invariant 2). The FR3g numstat skeleton is prepended in BOTH branches (it runs at
//     capture, upstream of the gate; the gate RECEIVES the skeleton string ONLY to size it — it is not
//     re-emitted by the gate).
//
// This file holds the PURE helper that implements the >0 branch's budget arithmetic + assembly. It is a
// PURE string/budget-arithmetic function — no git, no ctx, no I/O — so it is exhaustively unit-testable
// without a repo (tokengate_test.go mirrors truncatediff_test.go's pure table-driven style). The git.go
// >0 branches are then trivial wiring: capture uncapped text → delegate to applyWaterFillGate.
//
// COHERENCE (design-decisions D2/D8): sizing and enforcement BOTH use EstimateTokens over the SAME body.
// sectionBody splits a section at its first `@@` via the SIBLING's `atAtRe` — the EXACT same split
// truncateByWaterFill uses to cut the body. So the water-fill's "file > L" condition (on sizes) and
// truncateByWaterFill's "EstimateTokens(body) > allotment" enforcement AGREE exactly ⇒ the FR3i fairness
// guarantees (a–d: every file represented, small files whole, large files capped at a shared water level,
// headers preserved) hold. numstatRows is the dual-use SKELETON (FR3g) + path identity — the body token
// estimate uses the captured body, NOT numstat line counts.
//
// Composition (no new domain logic): sizing = EstimateTokens (tokens.go, in-package); allocation =
// allocByWaterFill (waterfill.go, the consumer-facing allocator — waterFillLevel is the solver's internal
// detail); application = truncateByWaterFill (truncatediff.go, which emits the `... [truncated]` sentinel
// and preserves headers). The gate only computes the budget + assembles the inputs.

package git

// tokenBudgetMargin is the FR3d/FR3i safety buffer subtracted from body_budget (PRD §9.1 FR3i:
// body_budget = token_limit − skeleton − prompt − margin). It absorbs (a) the chars/4 vs actual-
// tokenization density gap (the estimator is conservative; code is ~3-4 chars/token, prose ~4-5), (b)
// the `diff --git`/`---`/`+++` header blocks truncateByWaterFill PRESERVES but that are NOT counted in
// body sizing (they sit before the first `@@`), and (c) the `[binary]`/`[excluded]` placeholders. The
// user's token_limit already carries implicit slack (set below the hard context window); this is the
// deterministic floor. Tunable — raise it for noisier repos; lower it to spend more of the budget on
// bodies.
const tokenBudgetMargin = 1024

// sectionBody returns the BODY of a diff section: the substring from the first hunk-header line (`@@`,
// detected via the sibling's atAtRe — the SAME regex truncateByWaterFill splits on) onward. A section
// with no `@@` (pure rename / mode-only) has an empty body. PURE.
//
// Used by applyWaterFillGate to SIZE each file's body with EstimateTokens — using the SAME body split as
// the enforcement ⇒ the water-fill's "file > L" condition (on sizes) and truncateByWaterFill's
// "EstimateTokens(body) > allotment" AGREE exactly (coherence; design-decisions D2/D8). Do NOT invent a
// different body-split.
func sectionBody(section string) string {
	loc := atAtRe.FindStringIndex(section)
	if loc == nil {
		return "" // no hunk → no body → size 0 → never truncated (matches truncateByWaterFill's pass-through)
	}
	return section[loc[0]:]
}

// applyWaterFillGate is the FR3d/FR3i token-limit gate (PRD §9.1 FR3d/FR3i; system_context.md §5/§6). It
// replaces the legacy max_md_lines/max_diff_bytes caps with a dynamic water-fill over ALL diff bodies
// (markdown + non-markdown) sharing ONE body_budget (FR3i: "across files" — one shared budget, NOT two
// separate md/non-md budgets; design-decisions D3). PURE: no git, no ctx, no I/O — it composes only
// splitDiffSections, EstimateTokens, allocByWaterFill, truncateByWaterFill, diffSectionPath, sectionBody
// (all pure / in-package).
//
// Inputs:
//   - mdDiffs:      the per-file markdown diffs, each a self-contained `diff --git` section, already
//     captured UNCAPPED + FR3h-index-stripped + -M/-U<n>-shaped upstream (the >0 branch in git.go
//     appends stripIndexLines(fileDiff) without the legacy line-cap).
//   - nmDiff:       the non-markdown aggregate, captured UNCAPPED + index-stripped (the >0 branch skips
//     the legacy byte-cap).
//   - skeleton:     the already-prepended FR3g numstat skeleton string. USED ONLY TO SIZE — NOT re-emitted
//     (the skeleton is written to the builder BEFORE the gate runs; the gate receives the string so it can
//     subtract EstimateTokens(skeleton) from the budget).
//   - tokenLimit:   opts.TokenLimit (the caller has already branched on >0; this is the resolved value).
//   - promptReserve: opts.PromptReserveTokens (measured upstream, P1.M4.T1.S2; the stable prompt-portion
//     cost so body_budget = token_limit − skeleton − promptReserve).
//
// Algorithm:
//  1. skeletonTokens := EstimateTokens(skeleton).
//  2. bodyBudget := max(0, tokenLimit − skeletonTokens − promptReserve − tokenBudgetMargin).
//  3. sections = mdDiffs + splitDiffSections(nmDiff)  (ALL files, one shared budget — FR3i "across files").
//  4. sizes[i] = EstimateTokens(sectionBody(sections[i]))  (body tokens; the SAME body the enforcement cuts).
//  5. allocs := allocByWaterFill(sizes, bodyBudget)  (parallel to sections; preserves input order).
//  6. allotments[path] = allocs[i]  (keyed by diffSectionPath — the SAME key truncateByWaterFill looks up).
//  7. return truncateByWaterFill(sections, allotments)  (recomposes in input order; emits `... [truncated]`
//     per over-budget file; within-budget files byte-identical; the `at N bytes/lines` sentinels NEVER
//     appear — §6 invariant 2).
//
// Coherence: sizing + enforcement both use EstimateTokens over the SAME body (sectionBody via atAtRe ≡
// truncateByWaterFill's split) ⇒ the water-fill's fairness guarantees (FR3i a–d) are EXACT.
//
// Degenerate bodyBudget≤0 (token_limit too small for skeleton+reserve+margin): clamped to 0 ⇒
// allocByWaterFill returns all-0 ⇒ truncateByWaterFill treats allotment≤0 as path-miss ⇒ pass-through (no
// truncation; the skeleton is the floor — cutting bodies to 0 helps nothing). Falls out naturally; no
// special-case (design-decisions D10).
//
// Called by StagedDiff/TreeDiff/WorkingTreeDiff in their opts.TokenLimit>0 branch.
func applyWaterFillGate(mdDiffs []string, nmDiff, skeleton string, tokenLimit, promptReserve int) string {
	skeletonTokens := EstimateTokens(skeleton)
	bodyBudget := tokenLimit - skeletonTokens - promptReserve - tokenBudgetMargin
	if bodyBudget < 0 {
		bodyBudget = 0
	}

	nmSections := splitDiffSections(nmDiff)
	sections := make([]string, 0, len(mdDiffs)+len(nmSections))
	sections = append(sections, mdDiffs...)
	sections = append(sections, nmSections...)
	if len(sections) == 0 {
		return ""
	}

	sizes := make([]int, len(sections))
	for i, sec := range sections {
		sizes[i] = EstimateTokens(sectionBody(sec))
	}
	allocs := allocByWaterFill(sizes, bodyBudget)

	allotments := make(map[string]int, len(sections))
	for i, sec := range sections {
		if path, ok := diffSectionPath(sec); ok {
			allotments[path] = allocs[i] // keyed by destination — matches truncateByWaterFill's lookup (D11)
		}
	}
	return truncateByWaterFill(sections, allotments)
}
