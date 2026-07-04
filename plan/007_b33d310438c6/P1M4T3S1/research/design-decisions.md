# P1.M4.T3.S1 — Design Decisions & Findings

Scope: the FR3d/FR3i **token-limit GATE** in the three sibling diff functions (`StagedDiff`/`TreeDiff`/
`WorkingTreeDiff` in `internal/git/git.go`). Branch on `opts.TokenLimit`: `==0` ⇒ the existing legacy
`max_md_lines`/`max_diff_bytes` caps + their `... [diff truncated at N bytes/lines]` sentinels,
**byte-identical** (the regression anchor); `>0` ⇒ dynamic water-fill REPLACES both caps and emits the
shorter `... [truncated]` sentinel per truncated file. The FR3g numstat skeleton is prepended in BOTH
branches (unchanged). Implemented as a PURE helper `applyWaterFillGate` (new `tokengate.go`) wired into a
small `>0` branch in each of the 3 functions.

## Decisions

**D1 — The gate is a branch on `opts.TokenLimit` in all 3 functions; `==0` is byte-identical legacy.**
PRD §9.17/§9.1 FR3d: "When `token_limit` is `0`/unset, the legacy per-section caps apply unchanged … The two
modes are mutually exclusive: a non-zero `token_limit` supersedes both legacy caps for that run."
system_context.md §6 invariant 1 pins the `==0` body byte-identical (the always-on FR3e/f/g/h transforms
still apply AROUND the caps). The 6 cap-line literals (git.go 840/868/1302/1326/1458/1482) + their
`... [diff truncated at %d bytes/lines]` sentinels are the regression anchor — DO NOT alter them. The
existing stagediff/treediff/workingtreediff cap tests MUST still pass unchanged.

**D2 — Sizing source = `EstimateTokens(body)` per section (NOT numstat line counts) — REQUIRED for coherence.**
The item_description says "sizes = per-file body token estimates from numstatRows" and PRD FR3i says
"derived from the numstat skeleton's per-file line counts"; BUT the parallel sibling P1.M4.T2.S2's FROZEN
CONSUMER CONTRACT (its PRP "OUTPUT (downstream)") states the gate builds allotments from "`allocByWaterFill`
over `EstimateTokens(body)` sizes + the numstatRows path keying". Coherence DECIDES this: `truncateByWaterFill`
(the enforcement, frozen) truncates a section iff `EstimateTokens(body) > allotment`. For the water-fill's
"file > L" condition (which uses `sizes`) to AGREE with the enforcement, `sizes[i]` MUST be
`EstimateTokens(body_i)` — otherwise a file sized "small" by a line-count proxy but huge in real tokens would
be mis-allotted (incoherent fairness). The PRD's "numstat line counts" is realized as: **numstatRows is the
dual-use skeleton** (the FR3g completeness floor + per-file identity/path), while the body TOKEN estimate (the
SIZE) is the actual captured body measured by the SAME `EstimateTokens` the enforcement uses ⇒ EXACT fairness.
This aligns with the sibling contract AND with waterfill.go/tokens.go ("the same function so the budget
arithmetic is measured in consistent units"). [Documented reconciliation of the PRD/item wording.]

**D3 — ONE shared body_budget over ALL files (markdown + non-markdown combined).** FR3i: "the diff body is
allocated across files by a dynamic, size-aware water-fill." "files" = every changed file (md + non-md). So
the water-fill sizes md AND non-md sections TOGETHER against ONE budget (not two separate budgets — that would
double-spend). The pure helper takes `mdDiffs []string` + `nmDiff string`, splits nm via `splitDiffSections`,
concatenates `allSections = mdDiffs + nmSections`, sizes all, allocates over all, and truncates all in one
`truncateByWaterFill` call (which recomposes in input order ⇒ md block then non-md block).

**D4 — A PURE helper `applyWaterFillGate` in a NEW file `internal/git/tokengate.go`.** It composes only pure
functions (`splitDiffSections`, `EstimateTokens`, `allocByWaterFill`, `truncateByWaterFill`, + a local
`sectionBody`) — NO git, NO ctx, NO I/O. ⇒ fully unit-testable without a repo (deterministic table tests),
mirroring the sibling's pure-function testing strategy. The 3 diff functions just CAPTURE text (their existing
git calls, unchanged) and DELEGATE the truncation to this helper in the `>0` branch.

**D5 — `body_budget = max(0, TokenLimit − EstimateTokens(skeleton) − PromptReserveTokens − tokenBudgetMargin)`.**
PRD FR3i / system_context §5: `body_budget = token_limit − skeleton − prompt − margin`. `EstimateTokens(
skeleton)` measures the already-prepended skeleton (header + rows + blank line); `PromptReserveTokens` is
measured UPSTREAM (P1.M4.T1.S2) and passed in via opts; `tokenBudgetMargin` is a NEW const (this task owns
the margin — waterfill.go: "body_budget computation … is the M4.T3 gate"). The margin absorbs (a) the chars/4
vs actual-tokenization gap, (b) the `diff --git`/`---`/`+++` header blocks `truncateByWaterFill` PRESERVES but
that are NOT counted in body sizing (they sit before the first `@@`), (c) the `[binary]`/`[excluded]`
placeholders. Clamp at 0 (a budget ≤ 0 is the degenerate "token_limit too small for skeleton+reserve" case).

**D6 — Skeleton + binary/exclude placeholders are UNCHANGED in both branches.** The gate replaces ONLY the
cap step. The FR3g skeleton is captured + prepended first (both branches — identical code). The binary
detection + `[binary]`/`[excluded]` placeholders + `binExcludes` run identically in both branches (FR3b/FR-X4
are cap-independent). The helper RECEIVES the skeleton string ONLY to size it (`EstimateTokens(skeleton)`) —
it does NOT re-emit it (the skeleton is already in the output buffer).

**D7 — The `>0` branch wiring (per function), minimal diff:**
- Part 1 (markdown loop): capture each file's diff UNCAPPED + index-stripped (same `buildDiffArgs(opts,
  domain…) + "--" + file` call, same `stripIndexLines`), but instead of the line-cap + immediate write,
  APPEND to a `mdDiffs []string` and `continue` (skip the cap). The `==0` path keeps the line-cap + write
  verbatim.
- Part 2 (non-md aggregate): capture the aggregate UNCAPPED + index-stripped (same `nmArgs` incl. excludes +
  `:!*.md`/`:!*.markdown` + `binExcludes`, same `stripIndexLines`). Then:
  `if opts.TokenLimit > 0 { b.WriteString(applyWaterFillGate(mdDiffs, nmDiff, skeleton, opts.TokenLimit,
  opts.PromptReserveTokens)) } else { <existing byte cap + write> }`.
- `mdDiffs` is declared before Part 1. The skeleton var name is `skeleton` in all 3 (near-verbatim — confirm).

**D8 — `sectionBody` for sizing reuses `atAtRe` (the sibling's package-internal `(?m)^@@` regex).** The body
for sizing MUST be the EXACT same substring `truncateByWaterFill` truncates (it splits each section at
`atAtRe.FindStringIndex` → header block before, body from loc[0]). So `sectionBody(sec) = loc==nil ? "" :
sec[loc[0]:]` using the SAME `atAtRe` ⇒ guaranteed coherence (the sized body ≡ the truncated body). `atAtRe`
is package-internal in `truncatediff.go` (verified to exist); same-package reuse is fine and DRY. A section
with no `@@` (pure rename/mode-only) ⇒ body "" ⇒ size 0 ⇒ never truncated (matches S2's pass-through).

**D9 — NO file conflict with the parallel sibling.** P1.M4.T2.S2 CREATES `internal/git/truncatediff.go`
(+`truncatediff_test.go`) and its scope says `git.go`/`waterfill.go`/`numstat.go`/`skeleton.go`/`tokens.go`/
`binary.go`/`StagedDiffOptions` are UNCHANGED. This task EDITS `git.go` + CREATES `tokengate.go`/
`tokengate_test.go` + adds e2e tests. The ONLY shared surface is the READ-ONLY consumption of
`truncatediff.go`'s exported-ish (package-internal) symbols — no edit overlap. (Confirmed: `truncatediff.go`
already exists with the exact signatures — F1.)

**D10 — Degenerate `body_budget ≤ 0` ⇒ no truncation (graceful).** `allocByWaterFill(sizes, 0)` → all-0
allotments → `truncateByWaterFill` treats `allotment ≤ 0` as path-miss → pass-through verbatim. So a too-small
`token_limit` (skeleton + reserve already exceed it) yields the full bodies (the skeleton is the completeness
floor; cutting bodies to 0 helps nothing). Document; do not special-case.

**D11 — Path-keyed allotments via `diffSectionPath` (matches `truncateByWaterFill`'s internal lookup).** The
helper builds `allotments map[string]int` keyed by `diffSectionPath(section)` (the destination b/ path) — the
SAME key `truncateByWaterFill` uses internally to look up each section's allotment ⇒ guaranteed agreement
(no miss from key drift). `diffSectionPath` yields the same destination as `resolveNumstatPath` (numstat), so
the item_description's "numstatRows path keying" is honored transitively (D2). A path collision (two sections,
same destination) is impossible in practice (md/non-md are disjoint via `:!*.md`; `-M` collapses renames to one
section) — if it occurred, the later section's allotment would win (harmless).

**D12 — e2e test: temp repo, one huge + one small file, `token_limit` set.** Assert the small file is WHOLE
(body intact, no `... [truncated]` sentinel) and the large file is CAPPED (sentinel present, body cut), the
skeleton is present, and the total ≤ token_limit (+ reserve + margin slack). Per the item_description §3.

## Findings

**F1 — `internal/git/truncatediff.go` EXISTS with the exact signatures** (verified): `splitDiffSections(diff
string) []string`, `diffSectionPath(section string) (path string, ok bool)`, `firstNRunes`, `truncateByWaterFill(
sections []string, allotments map[string]int) string`, `truncatedSentinel = "... [truncated]"`, `atAtRe =
(?m)^@@`, `diffSectionHeaderRe`, `diffSectionPlusPlusRe`. The sibling S2 has landed it. CONSUME read-only.

**F2 — `allocByWaterFill(sizes []int, budget int) []int`** (waterfill.go:72) — PARALLEL to sizes, PRESERVES
INPUT ORDER, `out[i] = min(sizes[i], level)`. Delegates to `waterFillLevel(sizes, budget) (level int,
truncate bool)`: `sum ≤ budget` ⇒ `(maxSize, false)` (no truncation); else sort-and-walk ⇒ `(remaining/count,
true)`. UNIT-AGNOSTIC ints; production unit = TOKENS. Does NOT mutate the caller's slice.

**F3 — `EstimateTokens(s string) int` = `ceil(utf8.RuneCountInString(s) / 4)`** (tokens.go:25). In-package
(no import). The SINGLE estimator — sizing + enforcement + skeleton + reserve all use it ⇒ consistent units.

**F4 — The 3 functions are near-verbatim copies** (diff_capture_touchmap.md §1); the ONLY difference is the
diff-domain positional args threaded into `buildDiffArgs(opts, domain…)` / the capture `g.run`: `--cached`
(StagedDiff), `treeA, treeB` (TreeDiff), none (WorkingTreeDiff). `buildDiffArgs` (git.go:689) is the shared
argv helper (M2.T1.S1) emitting `-M` + `-U<diff_context>`. The CAP logic (line/byte cap) is INLINE in each
(not shared) — so the gate branch is added inline in each too (D7), delegating the `>0` work to the shared
pure helper (D4).

**F5 — system_context.md §6 invariants:** (1) `==0` ⇒ byte-identical legacy BODY caps (FR3e/f/g/h still
apply around them); (2) `>0` ⇒ water-fill replaces the caps, the shorter `... [truncated]` sentinel per
truncated file, the `at N bytes` sentinels do NOT appear; (4) payload-only, never commit-affecting.

**F6 — system_context.md §5 seam:** `PromptReserveTokens` is measured UPSTREAM (prompt/generate layers,
P1.M4.T1.S2) and passed in via `StagedDiffOptions`; the git layer computes `body_budget` from it. The git
layer NEVER imports internal/prompt (the reserve is just an int). `StagedDiffOptions.TokenLimit`/
`PromptReserveTokens`/`DiffContext` already exist (P1.M1.T2.S1) — READ them; do NOT touch the struct.

**F7 — The skeleton is already captured + prepended in each function.** StagedDiff: `skeleton, serr :=
g.numstatSkeleton(ctx, skeletonArgs…)` then `b.WriteString(skeleton)` (git.go ~755/~762). The `skeleton`
local var is the string to pass to the helper for sizing. TreeDiff/WorkingTreeDiff are identical (var `skeleton`).

**F8 — Part 1 (md) capture:** `g.run(ctx, g.workDir, append(buildDiffArgs(opts, domain…), "--", file)…)` →
`stripIndexLines(fileDiff)` → (==0: line cap + write). Part 2 (non-md): `nmArgs := buildDiffArgs(opts,
domain…); nmArgs = append(nmArgs, "--", excludes…, ":!*.md", ":!*.markdown", binExcludes…)` → `g.run` →
`stripIndexLines(nmDiff)` → (==0: byte cap + write). The `>0` branch replaces ONLY the cap step in each.

**F9 — `tokenBudgetMargin` is NOT yet defined** (grep: only comments reference "margin"). This task DEFINES
it (a const in `tokengate.go`). Value: a flat safety buffer (e.g. 1024) absorbing the chars/4 estimation gap +
the uncounted header-block/placeholder overhead. The user's `token_limit` already carries implicit slack
(they set it below the hard context window); the margin is the deterministic safety floor. Document tunability.

**F10 — Test layers:** (a) PURE unit tests for `applyWaterFillGate` in `tokengate_test.go` (deterministic,
no repo — mirror `truncatediff_test.go`/`numstat_test.go` table style); (b) e2e tests in a new
`difftokenlimit_test.go` (or stagediff_test.go) using the existing repo helpers (initRepo/writeFile/stageFile)
— temp repo with one huge + one small file, `token_limit` set, assert small-whole + large-capped + skeleton +
total ≤ limit. The existing `==0` cap tests (stagediff_test.go etc.) are the regression anchor — UNCHANGED.

**F11 — `EstimateTokens(body)` sizing + `truncateByWaterFill` enforcement share `atAtRe`.** The body for
sizing = `sec[atAtRe.FindStringIndex(sec)[0]:]`; the body for truncation = the same split inside
`truncateByWaterFill`. Using the SAME regex ⇒ the sized bytes ≡ the truncated bytes ⇒ the water-fill's
"file > L" (on sizes) and the enforcement's "EstimateTokens(body) > allotment" AGREE exactly (coherence).
