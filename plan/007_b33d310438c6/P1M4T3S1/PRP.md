---
name: "P1.M4.T3.S1 — Token-limit gate in the 3 diff functions; token_limit==0 byte-identical legacy path (PRD §9.1 FR3d/FR3i; system_context.md §5/§6; diff_capture_touchmap.md §1)"
description: |

  Wire the FR3d/FR3i token-limit GATE into the three sibling diff functions (`StagedDiff`/`TreeDiff`/
  `WorkingTreeDiff` in `internal/git/git.go`). Branch on `opts.TokenLimit`: (a) `==0` → the EXISTING legacy
  path — markdown per-file line-cap (`max_md_lines`) + non-markdown aggregate byte-cap (`max_diff_bytes`) +
  their `... [diff truncated at N bytes/lines]` sentinels, BYTE-IDENTICAL to pre-M4 (the regression anchor —
  the always-on FR3e/-M, FR3f/-U<n>, FR3g skeleton-prepend, FR3h index-strip transforms from M2/M3 still apply
  AROUND them unchanged); (b) `>0` → dynamic water-fill REPLACES both caps: compute
  `body_budget = max(0, TokenLimit − EstimateTokens(skeleton) − PromptReserveTokens − tokenBudgetMargin)`,
  size each file's body via `EstimateTokens`, allocate via `allocByWaterFill`, and apply `truncateByWaterFill`
  (which emits the shorter `... [truncated]` sentinel per truncated file). The FR3g numstat skeleton is
  prepended in BOTH branches (unchanged). Implemented as a PURE helper `applyWaterFillGate` (NEW
  `tokengate.go`) + a small `>0` branch in each of the 3 functions.

  CONTRACT (PRD §9.1 FR3d/FR3i; system_context.md §6 invariants 1+2; item_description §1–§5):
    - FR3d: "When `token_limit` is `0`/unset, the legacy per-section caps apply unchanged … A non-zero
      `token_limit` supersedes both legacy caps for that run." (mutually exclusive modes)
    - FR3i: "body_budget = token_limit − skeleton − prompt − margin … split on `diff --git` boundaries …
      every file larger than L is truncated to exactly L (its first L tokens + the `... [truncated]`
      sentinel) … each file's `diff --git`/hunk headers are always preserved."
    - system_context §6 inv 1: "`==0` ⇒ byte-identical legacy BODY caps (FR3e/f/g/h still apply around them)."
    - system_context §6 inv 2: "`>0` ⇒ water-fill replaces the byte/line caps; the `... [truncated]` sentinel
      (shorter form) per truncated file; the `at N bytes` sentinels do NOT appear."
    - item_description §3: "==0 → existing path … byte-identical to pre-M4 (the stagediff/treediff/
      workingtreediff byte/line-cap tests must still pass). >0 → compute skeleton_tokens = estimateTokens(
      skeletonBlock); body_budget = max(0, TokenLimit − skeleton_tokens − PromptReserveTokens − margin);
      sizes = per-file body token estimates; allotments = allocByWaterFill(sizes, body_budget); apply
      truncateByWaterFill to the markdown + non-markdown sections; emit `... [truncated]` per truncated file.
      The skeleton (M3) is ALWAYS prepended in both branches."

  INPUT (upstream — all EXIST, READ/CONSUME only, do NOT modify):
    - `opts.TokenLimit`, `opts.PromptReserveTokens` — `StagedDiffOptions` (P1.M1.T2.S1). READ; do NOT touch the struct.
    - `splitDiffSections` / `diffSectionPath` / `truncateByWaterFill` / `atAtRe` / `truncatedSentinel` —
      `internal/git/truncatediff.go` (PARALLEL P1.M4.T2.S2 — EXISTS, verified). The frozen pure primitives.
    - `allocByWaterFill(sizes, budget) []int` / `waterFillLevel` — `internal/git/waterfill.go` (P1.M4.T2.S1, COMPLETE).
    - `EstimateTokens(s string) int` — `internal/git/tokens.go` (P1.M4.T1.S1, COMPLETE). In-package.
    - The captured bodies — already `-M`/`-U<n>`-shaped + FR3h-index-stripped by M2; the `index` line is GONE.

  OUTPUT: `token_limit` gates the truncation strategy in all 3 diff paths; `==0` is byte-identical legacy;
  `>0` is water-fill. Completes FR3d + FR3i end-to-end.

  DELIVERABLES (1 NEW source + 1 NEW pure-test + 1 EDIT + 1 NEW e2e-test):
    NEW internal/git/tokengate.go              — `package git`. PURE `applyWaterFillGate(mdDiffs, nmDiff,
      skeleton string, tokenLimit, promptReserve int) string` + `sectionBody(section string) string` +
      `tokenBudgetMargin` const. Composes splitDiffSections/EstimateTokens/allocByWaterFill/
      truncateByWaterFill only (no git/ctx/I/O).
    NEW internal/git/tokengate_test.go         — `package git` (white-box). PURE deterministic table tests
      for applyWaterFillGate (budget arithmetic, sizing, allocation, truncation integration, fairness).
    EDIT internal/git/git.go                    — a `>0` branch in StagedDiff, TreeDiff, WorkingTreeDiff:
      Part 1 md collects UNCAPPED diffs into `mdDiffs`; Part 2 non-md calls `applyWaterFillGate`. The `==0`
      path is UNCHANGED (byte-identical caps + sentinels).
    NEW internal/git/difftokenlimit_test.go     — e2e (temp repo) tests: one huge + one small file,
      `token_limit` set → small whole, large capped, skeleton present, total ≤ limit. Plus `==0` regression.

  SCOPE BOUNDARY (owned by siblings — do NOT implement/edit):
    - `truncatediff.go` (the split/extract/truncate primitives + sentinel) — PARALLEL P1.M4.T2.S2. CONSUME.
    - `waterfill.go` (the solver) — P1.M4.T2.S1, COMPLETE. CONSUME (`allocByWaterFill`); do NOT call
      `waterFillLevel` directly (use `allocByWaterFill`).
    - `tokens.go` / `numstat.go` / `skeleton.go` / `binary.go` — siblings. READ ONLY.
    - `StagedDiffOptions` struct — P1.M1.T2.S1. READ `TokenLimit`/`PromptReserveTokens`; do NOT edit.
    - The 6 call sites that MAP cfg→opts — P1.M1.T2.S2. UNCHANGED (they already thread the fields).

  PARALLEL-EXECUTION COORDINATION: P1.M4.T2.S2 CREATES `truncatediff.go` (+`truncatediff_test.go`) and its
  scope leaves `git.go` UNCHANGED. This task EDITS `git.go` + CREATES `tokengate.go`/`tokengate_test.go`/
  `difftokenlimit_test.go`. NO file overlap — the only shared surface is READ-ONLY consumption of
  `truncatediff.go`'s package-internal symbols (`splitDiffSections`, `diffSectionPath`, `truncateByWaterFill`,
  `atAtRe`). `truncatediff.go` already exists with the exact signatures (verified) — assume it is final.

  Deliverable: `token_limit==0` diffs are byte-identical to pre-M4 (all existing cap tests pass unchanged);
  `token_limit>0` water-fills (small files whole, large capped at L, `... [truncated]` per file, skeleton
  present, total ≤ token_limit). `go build ./... && go test ./...` green; `go vet ./...` clean; go.mod/go.sum
  unchanged; only the 4 listed files differ (git.go + 3 new files).

---

## Goal

**Feature Goal**: Close FR3d + FR3i (PRD §9.1) by gating the truncation strategy on `opts.TokenLimit` in all
three diff functions: `==0` keeps the legacy per-section caps byte-identical (the regression anchor); `>0`
replaces them with the dynamic water-fill (body_budget = token_limit − skeleton − reserve − margin; per-file
sizing via `EstimateTokens`; `allocByWaterFill` allotments; `truncateByWaterFill` application with the shorter
`... [truncated]` sentinel per truncated file). The FR3g skeleton is prepended in both branches. The `>0`
logic is a PURE, git-independent helper (`applyWaterFillGate`) so it is unit-testable without a repo, wired
into a minimal `>0` branch in each function.

**Deliverable** (1 NEW source + 1 NEW pure-test + 1 EDIT + 1 NEW e2e-test):
1. NEW `internal/git/tokengate.go` — `applyWaterFillGate(mdDiffs []string, nmDiff, skeleton string, tokenLimit,
   promptReserve int) string` + `sectionBody(section string) string` + `tokenBudgetMargin` const. PURE (no
   git/ctx/I/O; composes only `splitDiffSections`/`EstimateTokens`/`allocByWaterFill`/`truncateByWaterFill`).
2. NEW `internal/git/tokengate_test.go` — pure deterministic table tests (no repo).
3. EDIT `internal/git/git.go` — a `>0` branch in `StagedDiff`, `TreeDiff`, `WorkingTreeDiff` (Part 1 md
   collects uncapped; Part 2 non-md delegates to `applyWaterFillGate`); `==0` path byte-identical.
4. NEW `internal/git/difftokenlimit_test.go` — e2e (temp repo) tests for the `>0` path + a `==0` regression.

**Success Definition**: `go test ./internal/git/` green — (a) ALL existing stagediff/treediff/workingtreediff
byte/line-cap tests pass UNCHANGED at `token_limit==0` (byte-identical legacy); (b) the new `>0` tests pass: a
payload exceeding `token_limit` is water-filled (small file WHOLE — no `... [truncated]` — large file CAPPED
with the sentinel, skeleton present, total ≤ token_limit + reserve + margin); (c) the e2e test (temp repo, one
huge + one small file) shows the small file whole and the large capped. `make build` then a real repo with
`token_limit` set in `.stagehand.toml` produces a water-filled payload; without `token_limit` the output is
byte-identical to pre-M4. `go vet ./...` clean; `gofmt -l` empty; go.mod/go.sum unchanged; only the 4 files.

## User Persona

**Target User**: A user whose diff exceeds their model's context window — they set `token_limit` (e.g.
`120000`) so stagehand shrinks the diff to fit, fairly (every file represented via the skeleton; small files
whole; large files capped at a shared water level with a visible `... [truncated]` marker), without stagehand
maintaining a per-model context registry (FR3d). Transitively: users who DON'T set `token_limit` get the
exact pre-M4 behavior (no change).

**Use Case**: `token_limit = 120000` in `.stagehand.toml` + a large multi-file change → the diff fits the
budget; a 2-line `README.md` tweak appears whole; a 9000-line generated file is capped at the water level L
with `... [truncated]`. Without `token_limit`, the legacy 300000-byte / 100-line caps apply exactly as before.

**User Journey**: set `token_limit` → run stagehand → the payload fits the model → generation succeeds
(no truncation failure / OOM). The `... [truncated]` markers tell the model WHICH files are partial (honesty);
the skeleton tells it EVERY file that changed (completeness). Unset `token_limit` → identical to before.

**Pain Points Addressed**: (1) "my diff is too big for the model" — solved by the holistic budget; (2) "the
old byte-cap cut a file mid-token and dropped later files silently" — solved by per-file water-fill (fair,
header-preserving, sentinel-marked); (3) "did setting token_limit change my normal output?" — NO: `==0` is
byte-identical (the regression anchor).

## Why

- **Completes FR3d + FR3i (P0).** The estimator (M4.T1), the solver (M4.T2.S1), and the truncation
  application (M4.T2.S2) all land their outputs into THIS gate — the only place the budget is computed and
  the strategy is chosen. Without it, `token_limit` is a plumbed-but-unused field.
- **The `==0` regression anchor is a hard contract.** system_context §6 inv 1 + the item_description demand
  byte-identical legacy caps at `==0`. The gate MUST be a clean branch that leaves the `==0` path untouched
  (the 6 cap-line literals + sentinels are the anchor the existing tests pin).
- **Pure helper ⇒ exhaustively testable.** `applyWaterFillGate` is pure string/budget arithmetic over
  already-captured text — every fairness case is a deterministic table assertion (no git repo needed), like
  the sibling's `truncatediff_test.go`. The git.go edits are then trivial wiring (capture uncapped → delegate).
- **Reuses frozen upstream (zero new domain logic).** Sizing = `EstimateTokens`; allocation =
  `allocByWaterFill`; application = `truncateByWaterFill`. The gate only computes the budget + assembles the
  inputs. No new algorithm.

## What

A PURE helper `applyWaterFillGate` and a `>0` branch in each diff function.

`applyWaterFillGate(mdDiffs []string, nmDiff, skeleton string, tokenLimit, promptReserve int) string`:
1. `skeletonTokens := EstimateTokens(skeleton)`.
2. `bodyBudget := tokenLimit - skeletonTokens - promptReserve - tokenBudgetMargin`; clamp `<0 → 0`.
3. `nmSections := splitDiffSections(nmDiff)`; `sections := append(append([]string{}, mdDiffs…), nmSections…)`.
4. For each section: `sizes[i] = EstimateTokens(sectionBody(section))` (body = from the first `@@` onward,
   via `atAtRe` — same split as `truncateByWaterFill` ⇒ coherent). `if len(sections)==0 { return "" }`.
5. `allocs := allocByWaterFill(sizes, bodyBudget)` (parallel to `sections`; preserves order).
6. Build `allotments map[string]int`: for each section, `if path, ok := diffSectionPath(section); ok {
   allotments[path] = allocs[i] }`.
7. `return truncateByWaterFill(sections, allotments)` (recomposes in input order ⇒ md block then non-md block;
   emits `... [truncated]` per over-budget file; within-budget files byte-identical).

`sectionBody(section string) string`: `loc := atAtRe.FindStringIndex(section); if loc==nil { return "" };
return section[loc[0]:]`. (No `@@` ⇒ pure-rename/mode-only ⇒ body "" ⇒ size 0 ⇒ never truncated.)

`tokenBudgetMargin` (const): a flat safety buffer absorbing the chars/4 estimation gap + the uncounted
header-block/placeholder overhead (e.g. `1024`).

The git.go `>0` branch (each of the 3 functions):
- Part 1 (md loop): declare `var mdDiffs []string` before the loop; inside, after `fileDiff = stripIndexLines(
  fileDiff)`: `if opts.TokenLimit > 0 { mdDiffs = append(mdDiffs, fileDiff); continue }` — else the EXISTING
  line-cap + write (byte-identical).
- Part 2 (non-md): after `nmDiff = stripIndexLines(nmDiff)`: `if opts.TokenLimit > 0 {
  b.WriteString(applyWaterFillGate(mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens)) }
  else { <existing byte cap + b.WriteString(nmDiff)> }`.

The `==0` path is byte-identical (the 6 cap literals + sentinels untouched).

### Success Criteria

- [ ] `internal/git/tokengate.go` exists, `package git`, PURE (no git/ctx/I/O; composes only
      `splitDiffSections`/`EstimateTokens`/`allocByWaterFill`/`truncateByWaterFill` + `atAtRe`).
- [ ] `applyWaterFillGate` computes `bodyBudget = max(0, tokenLimit − EstimateTokens(skeleton) − promptReserve
      − tokenBudgetMargin)`; sizes each section's body via `EstimateTokens(sectionBody(…))`; calls
      `allocByWaterFill`; builds the path→allotment map via `diffSectionPath`; returns `truncateByWaterFill`.
- [ ] `StagedDiff`/`TreeDiff`/`WorkingTreeDiff` each branch on `opts.TokenLimit`: `>0` collects md diffs
      uncapped + delegates Part 2 to `applyWaterFillGate`; `==0` is the EXISTING line/byte-cap path unchanged.
- [ ] `token_limit==0` ⇒ byte-identical legacy (the existing stagediff/treediff/workingtreediff cap tests
      pass UNCHANGED; the 6 `... [diff truncated at %d bytes/lines]` literals + sentinels untouched).
- [ ] `token_limit>0` ⇒ water-fill: a payload exceeding the budget has small files WHOLE (no sentinel) and
      large files CAPPED (the `... [truncated]` sentinel, headers preserved); the skeleton is present; the
      `at N bytes/lines` sentinels do NOT appear.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l internal/git/` empty; go.mod/go.sum
      unchanged; only git.go + tokengate.go + tokengate_test.go + difftokenlimit_test.go differ.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact `applyWaterFillGate` +
`sectionBody` skeletons (below), the precise git.go edit points (Part 1 md loop / Part 2 non-md cap — quoted
per function), the frozen upstream signatures (all quoted + in design-decisions F1–F11), the coherence
rationale for sizing via `EstimateTokens(body)` (D2), the `==0` regression-anchor contract (D1/F5), and the
test patterns to mirror (`truncatediff_test.go` pure tables; `stagediff_test.go` repo helpers). No
prompt/provider/decompose knowledge required.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE design decisions
- docfile: plan/007_b33d310438c6/P1M4T3S1/research/design-decisions.md
  why: the 12 decisions + 11 findings. D1 (==0 byte-identical anchor), D2 (size via EstimateTokens(body) for
       coherence — reconciles item_description "numstatRows" + sibling contract "EstimateTokens(body)"), D3
       (ONE shared budget over md+non-md), D4 (PURE helper in tokengate.go), D5 (body_budget formula + the
       margin const), D6 (skeleton+placeholders unchanged), D7 (the minimal-diff >0 wiring per function), D8
       (sectionBody reuses atAtRe ⇒ coherence), D9 (NO file conflict w/ sibling), D10 (degenerate budget≤0),
       D11 (path-keyed via diffSectionPath), D12 (e2e test).
  critical: D1 (the regression anchor — don't touch ==0), D2 (sizing source — the central decision), D7/D8
       (the exact wiring + body split), F1 (truncatediff.go EXISTS — consume), F4 (3 functions are
       near-verbatim; only diffArgs differs).

- docfile: plan/007_b33d310438c6/P1M4T3S1/research/external-research.md
  why: water-fill = max-min-fairness (§1 — the guarantees a–d); the chars/4 estimator is the SINGLE unit
       (§2); the TWO sentinels must not be confused (§3 — `at N bytes/lines` legacy vs `... [truncated]`
       water-fill).
  critical: §3 (the >0 path emits ONLY `... [truncated]` via truncateByWaterFill; NEVER the legacy forms).

# MUST READ — the parallel sibling's OUTPUT contract (the frozen primitives this gate consumes)
- docfile: plan/007_b33d310438c6/P1M4T2S2/PRP.md
  section: "OUTPUT (downstream — the frozen consumer contract)": "M4.T3 calls `sections :=
       splitDiffSections(nmDiff)`, builds `allotments` (path→token allotment from `allocByWaterFill` over
       `EstimateTokens(body)` sizes + the numstatRows path keying), and substitutes `truncateByWaterFill(
       sections, allotments)` for the legacy byte-cap when `token_limit > 0`."
  why: confirms (a) sizes from EstimateTokens(body) — the gate's sizing source (D2); (b) allotments path-
       keyed; (c) substitute truncateByWaterFill for the byte-cap on >0; (d) splitDiffSections on nmDiff.
       This task IMPLEMENTS exactly that consumer. Signatures are FROZEN.
  critical: the sibling owns truncatediff.go; this task CONSUMES it read-only. No edit overlap (D9).

# MUST READ — the file being EDITED (the 3 diff functions + their cap regions)
- file: internal/git/git.go   (EDIT — add the >0 branch to StagedDiff/TreeDiff/WorkingTreeDiff)
  section: StagedDiff Part 1 md loop (~L825–842: `fileDiff = stripIndexLines(fileDiff)` → `if lines :=
       strings.Split(fileDiff,"\n"); len(lines)>maxMDLines { … "\n... [diff truncated at %d lines]" }` →
       `b.WriteString(fileDiff)` + newline). Part 2 non-md (~L859–869: `nmDiff = stripIndexLines(nmDiff)` →
       `if len(nmDiff)>maxDiffBytes { nmDiff = nmDiff[:maxDiffBytes] + "\n... [diff truncated at %d bytes]" }`
       → `b.WriteString(nmDiff)`). TreeDiff (~L1295–1327) and WorkingTreeDiff (~L1450–1483) are near-verbatim
       (only diffArgs differs). The skeleton capture (`skeleton, serr := g.numstatSkeleton(…)`;
       `b.WriteString(skeleton)`) is BEFORE Part 1 and UNCHANGED. `buildDiffArgs(opts, domain…)` (L689) is the
       shared argv helper.
  why: this is THE file. The >0 branch wraps the cap step in each function's Part 1 + Part 2. The `==0` path
       (the cap literals + sentinels) stays byte-identical.
  pattern: see "Implementation Blueprint" for the exact oldText→newText per Part 1 / Part 2 (identical shape
       in all 3 functions; only the domain args in the surrounding capture differ).
  gotcha: declare `var mdDiffs []string` BEFORE Part 1 in each function. The skeleton var is `skeleton` in all
       3 (confirm). Do NOT touch the `==0` cap literals. Do NOT touch the skeleton capture / binary /
       excludes / binExcludes logic (cap-independent).

# MUST READ — the frozen primitives consumed (READ ONLY — the sibling's file, verified to exist)
- file: internal/git/truncatediff.go   (READ — do NOT edit; sibling P1.M4.T2.S2)
  section: `splitDiffSections(diff string) []string` (split on `diff --git `, re-prefix), `diffSectionPath(
       section string) (path string, ok bool)` (destination b/; fallback `+++ b/`; strip one `"`), `
       truncateByWaterFill(sections []string, allotments map[string]int) string` (split at first `@@` →
       header block + body; truncate body to allotment×4 runes + `\n... [truncated]` when EstimateTokens(body)
       > allotment; path-miss/allotment≤0 ⇒ verbatim; recompose in input order), `truncatedSentinel =
       "... [truncated]"`, `atAtRe = (?m)^@@`, `firstNRunes`.
  why: the gate calls exactly these. `atAtRe` is reused by `sectionBody` so the sized body ≡ the truncated
       body (D8 coherence). truncateByWaterFill enforces `EstimateTokens(body) > allotment` — the gate's
       sizes (also EstimateTokens(body)) AGREE ⇒ exact fairness.
  gotcha: truncateByWaterFill treats allotment≤0 as path-miss (pass-through) ⇒ degenerate budget≤0 ⇒ no
       truncation (D10). diffSectionPath keys the map; the gate MUST key identically (D11).

# MUST READ — the solver (COMPLETE; consume allocByWaterFill only)
- file: internal/git/waterfill.go   (READ — do NOT edit)
  section: `allocByWaterFill(sizes []int, budget int) []int` — out[i]=min(sizes[i], level); PARALLEL to sizes;
       PRESERVES INPUT ORDER; does NOT mutate the caller's slice. `sum≤budget` (or empty) ⇒ every file whole.
  why: the gate's allocator. Pass `sizes` (EstimateTokens(body) per section, in sections order) + bodyBudget.
       The returned slice is parallel to `sections` → map by index → path via diffSectionPath.
  gotcha: call `allocByWaterFill`, NOT `waterFillLevel` (the solver's level is an internal detail; the
       allotments slice is the consumer-facing API).

# MUST READ — the estimator (in-package; the SINGLE unit)
- file: internal/git/tokens.go   (READ — do NOT edit)
  section: `EstimateTokens(s string) int = ceil(utf8.RuneCountInString(s)/4)`. In-package (no import line).
  why: used for skeleton cost, body sizes, AND (inside truncateByWaterFill) the enforcement ⇒ one consistent
       unit. Do NOT "improve" to chars/3 (the margin absorbs the gap).

# MUST READ — the §6 invariants (the acceptance criteria) + §5 seam
- docfile: plan/007_b33d310438c6/architecture/system_context.md
  section: "## 6. Regression invariants" (inv 1: ==0 byte-identical BODY caps; inv 2: >0 water-fill + shorter
       sentinel, `at N bytes` gone) AND "## 5. The FR3i coupling seam" (PromptReserveTokens measured
       upstream; git layer computes body_budget = token_limit − skeleton − promptReserve).
  why: inv 1 is the regression-anchor CONTRACT; inv 2 pins the sentinel form; §5 pins the budget formula +
       that the git layer RECEIVES promptReserve (never imports internal/prompt).

# MUST READ — the touch map (the 3 functions are near-verbatim; diffArgs is the only difference)
- docfile: plan/007_b33d310438c6/architecture/diff_capture_touchmap.md
  section: "## 1. The THREE sibling diff functions" — the table (StagedDiff `--cached` / TreeDiff `treeA,treeB`
       / WorkingTreeDiff none) + the shared 3-part structure (Part 1 md line-cap / binary / excludes / Part 2
       non-md byte-cap).
  why: confirms the gate must be applied to ALL THREE (FR3c parity) and that the cap logic is INLINE in each
       (no shared cap helper) ⇒ the >0 branch is added inline in each, delegating to the shared pure helper.

# READ — the test patterns to mirror
- file: internal/git/truncatediff_test.go   (READ — mirror for tokengate_test.go)
  section: the pure table-driven style (`tests := []struct{…}{…}`, `t.Run(tc.desc, …)`, HARDCODED `want`,
       no t.TempDir/no repo). "Pure function; no I/O."
  why: tokengate_test.go mirrors this EXACTLY (applyWaterFillGate is pure).
- file: internal/git/stagediff_test.go   (READ — reuse repo helpers for the e2e test)
  section: the repo helpers (initRepo/writeFile/stageFile/gitOut/etc.) + the existing `==0` cap tests (the
       regression anchor — these MUST still pass).
  why: difftokenlimit_test.go reuses these helpers for the temp-repo e2e test. The existing cap tests pin ==0.
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  git.go                       # EDIT — add the >0 branch to StagedDiff/TreeDiff/WorkingTreeDiff (3 sites). ==0 path byte-identical.
  tokengate.go                 # *** CREATE *** — applyWaterFillGate + sectionBody + tokenBudgetMargin (pure).
  tokengate_test.go            # *** CREATE *** — pure deterministic table tests for applyWaterFillGate.
  difftokenlimit_test.go       # *** CREATE *** — e2e (temp repo) >0 tests + a ==0 regression check.
  truncatediff.go              # P1.M4.T2.S2 (parallel, EXISTS) — splitDiffSections/diffSectionPath/truncateByWaterFill/atAtRe/truncatedSentinel. READ ONLY.
  waterfill.go                 # P1.M4.T2.S1 (COMPLETE) — allocByWaterFill/waterFillLevel. READ ONLY (consume allocByWaterFill).
  tokens.go                    # P1.M4.T1.S1 (COMPLETE) — EstimateTokens. READ ONLY (in-package call).
  numstat.go / skeleton.go / binary.go  # siblings. READ ONLY.
  stagediff_test.go / treediff_test.go / workingtreediff_test.go  # the ==0 cap tests (regression anchor). UNCHANGED.
go.mod / go.sum                # UNCHANGED (stdlib only; no new deps).
```

### Desired Codebase tree with files to be added/changed

```bash
internal/git/tokengate.go              # NEW — applyWaterFillGate (pure) + sectionBody + tokenBudgetMargin const.
internal/git/tokengate_test.go         # NEW — pure table tests (budget/sizing/allocation/truncation/fairness).
internal/git/difftokenlimit_test.go    # NEW — e2e temp-repo >0 tests (huge+small file) + ==0 regression.
internal/git/git.go                    # EDIT — >0 branch in StagedDiff/TreeDiff/WorkingTreeDiff (Part 1 collect-uncapped; Part 2 applyWaterFillGate). ==0 byte-identical.
# NO other files changed. go.mod/go.sum UNCHANGED. truncatediff.go/waterfill.go/tokens.go/numstat.go/skeleton.go/binary.go/StagedDiffOptions UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the ==0 regression anchor — D1/system_context §6 inv 1): the >0 branch MUST be additive. The 6
//   `... [diff truncated at %d bytes/lines]` literals (git.go 840/868/1302/1326/1458/1482) + their caps stay
//   BYTE-IDENTICAL. The existing stagediff/treediff/workingtreediff cap tests are the anchor — they MUST pass
//   unchanged. Do NOT refactor the cap logic out of the ==0 path; wrap it in `if opts.TokenLimit > 0 {…} else {<existing>}`.

// CRITICAL (size via EstimateTokens(body), NOT numstat line counts — D2): the item_description says "from
//   numstatRows" and PRD FR3i says "numstat line counts", BUT the sibling's frozen consumer contract says
//   "EstimateTokens(body) sizes" AND coherence DEMANDS it: truncateByWaterFill truncates iff EstimateTokens(
//   body) > allotment, so sizes[i] MUST be EstimateTokens(body_i) for the water-fill's "file > L" to agree
//   with the enforcement. numstatRows is the dual-use SKELETON (FR3g) + path identity; the body token estimate
//   uses the captured body. Documented reconciliation (design-decisions D2).

// CRITICAL (sectionBody reuses atAtRe — D8): the sized body MUST be the EXACT substring truncateByWaterFill
//   truncates. Both split at atAtRe.FindStringIndex (the sibling's `(?m)^@@`). So sectionBody(sec) = loc==nil
//   ? "" : sec[loc[0]:] using the SAME atAtRe ⇒ coherence. Do NOT invent a different body-split.

// CRITICAL (ONE shared budget over md+non-md — D3): size md AND non-md sections TOGETHER against ONE
//   bodyBudget (FR3i "across files"). Two separate budgets would double-spend. applyWaterFillGate takes
//   mdDiffs + nmDiff, concatenates the sections, sizes/allocates/truncates all in one pass.

// CRITICAL (path-keyed allotments via diffSectionPath — D11): the allotments map MUST be keyed by the SAME
//   path truncateByWaterFill looks up (diffSectionPath's destination). Building the map with diffSectionPath
//   guarantees agreement. Do NOT key by numstat path via a separate match (key drift ⇒ miss ⇒ wrong pass-through).

// GOTCHA (the skeleton is prepended in BOTH branches — D6): the skeleton capture + b.WriteString(skeleton)
//   runs BEFORE the gate. applyWaterFillGate RECEIVES the skeleton string ONLY to size it (EstimateTokens(
//   skeleton) for bodyBudget) — it does NOT re-emit it. Binary/exclude placeholders + binExcludes also run
//   identically in both branches (cap-independent).

// GOTCHA (the >0 path captures bodies UNCAPPED — D7): Part 1 md appends stripIndexLines(fileDiff) to mdDiffs
//   WITHOUT the line cap; Part 2 non-md captures nmDiff WITHOUT the byte cap. The cap is REPLACED by the
//   water-fill (the whole point of FR3d: ">0 supersedes both legacy caps"). -M/-U<n>/index-strip still apply
//   (they're in the capture step, unchanged).

// GOTCHA (degenerate bodyBudget ≤ 0 — D10): token_limit too small for skeleton+reserve ⇒ bodyBudget clamped
//   to 0 ⇒ allocByWaterFill → all-0 allotments ⇒ truncateByWaterFill treats allotment≤0 as path-miss ⇒
//   pass-through (no truncation). Graceful: the skeleton is the floor; cutting bodies to 0 helps nothing.

// GOTCHA (the `index` line is ALREADY GONE — FR3h, M2.T3.S1): stripIndexLines runs at capture, upstream of
//   the gate. The sections the gate splits have NO `index` line. Do NOT re-strip.

// GOTCHA (no file conflict with the parallel sibling — D9): the sibling CREATES truncatediff.go + its test;
//   its scope leaves git.go UNCHANGED. This task EDITS git.go + CREATES tokengate.go/tests. The only shared
//   surface is READ-ONLY consumption of truncatediff.go's package-internal symbols. truncatediff.go already
//   exists (verified) — assume final.

// GOTCHA (mdDiffs declaration + skeleton var): declare `var mdDiffs []string` BEFORE Part 1 in each function.
//   The skeleton local var is `skeleton` in all 3 (confirm by reading). Pass it to applyWaterFillGate.

// GOTCHA (tokenBudgetMargin is a NEW const — F9): not yet defined. Put it in tokengate.go (e.g. 1024). It
//   absorbs the chars/4 estimation gap + the uncounted header-block/placeholder overhead. The user's
//   token_limit carries its own slack; the margin is the deterministic floor. Document tunability.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/git/tokengate.go
package git

// tokenBudgetMargin is the FR3d/FR3i safety buffer subtracted from body_budget (PRD §9.1 FR3i:
// body_budget = token_limit − skeleton − prompt − margin). It absorbs (a) the chars/4 vs actual-tokenization
// density gap (the estimator is conservative; code is ~3-4 chars/token, prose ~4-5), (b) the `diff --git`/
// `---`/`+++` header blocks truncateByWaterFill PRESERVES but that are NOT counted in body sizing (they sit
// before the first `@@`), and (c) the `[binary]`/`[excluded]` placeholders. The user's token_limit already
// carries implicit slack (set below the hard context window); this is the deterministic floor. Tunable.
const tokenBudgetMargin = 1024

// sectionBody returns the BODY of a diff section: the substring from the first hunk-header line (`@@`,
// detected via the sibling's atAtRe — the SAME regex truncateByWaterFill splits on) onward. A section with no
// `@@` (pure rename / mode-only) has an empty body. PURE. Used by applyWaterFillGate to SIZE each file's body
// with EstimateTokens — using the SAME body split as the enforcement ⇒ the water-fill's "file > L" condition
// (on sizes) and truncateByWaterFill's "EstimateTokens(body) > allotment" AGREE exactly (coherence).
func sectionBody(section string) string {
	loc := atAtRe.FindStringIndex(section)
	if loc == nil {
		return "" // no hunk → no body → size 0 → never truncated (matches truncateByWaterFill's pass-through)
	}
	return section[loc[0]:]
}

// applyWaterFillGate is the FR3d/FR3i token-limit gate (PRD §9.1 FR3d/FR3i; system_context.md §5/§6). It
// replaces the legacy max_md_lines/max_diff_bytes caps with a dynamic water-fill over ALL diff bodies (md +
// non-md) sharing ONE body_budget. PURE: no git, no ctx, no I/O — it composes only splitDiffSections,
// EstimateTokens, allocByWaterFill, truncateByWaterFill, diffSectionPath, sectionBody (all pure/in-package).
//
// Inputs: mdDiffs = the per-file markdown diffs (already captured UNCAPPED + FR3h-index-stripped, each a
// self-contained `diff --git` section); nmDiff = the non-markdown aggregate (captured UNCAPPED + index-
// stripped); skeleton = the already-prepended FR3g numstat skeleton string (used ONLY to size — NOT re-
// emitted); tokenLimit = opts.TokenLimit (>0, the gate's caller has already branched); promptReserve =
// opts.PromptReserveTokens (measured upstream, P1.M4.T1.S2).
//
// Algorithm:
//  1. skeletonTokens := EstimateTokens(skeleton).
//  2. bodyBudget := max(0, tokenLimit − skeletonTokens − promptReserve − tokenBudgetMargin).
//  3. sections = mdDiffs + splitDiffSections(nmDiff)  (ALL files, one shared budget — FR3i "across files").
//  4. sizes[i] = EstimateTokens(sectionBody(sections[i]))  (body tokens; same body the enforcement cuts).
//  5. allocs := allocByWaterFill(sizes, bodyBudget)  (parallel to sections; preserves order).
//  6. allotments[path] = allocs[i]  (keyed by diffSectionPath — the SAME key truncateByWaterFill looks up).
//  7. return truncateByWaterFill(sections, allotments)  (recomposes in input order; emits `... [truncated]`
//     per over-budget file; within-budget files byte-identical).
//
// Coherence: sizing + enforcement both use EstimateTokens over the SAME body (sectionBody via atAtRe ≡
// truncateByWaterFill's split) ⇒ the water-fill's fairness guarantees (FR3i a–d) are EXACT. Degenerate
// bodyBudget≤0 ⇒ allocByWaterFill returns all-0 ⇒ truncateByWaterFill pass-through (no truncation; the
// skeleton is the floor). Called by StagedDiff/TreeDiff/WorkingTreeDiff in their opts.TokenLimit>0 branch.
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
```

```go
// internal/git/git.go — EDIT StagedDiff (and the near-verbatim TreeDiff / WorkingTreeDiff).
// Declare mdDiffs BEFORE Part 1; branch on opts.TokenLimit in Part 1 (collect) and Part 2 (delegate).

// ---- in StagedDiff, BEFORE the Part 1 markdown loop (after the skeleton/placeholders) ----
	var mdDiffs []string // collected UNCAPPED when token_limit>0 (FR3d: >0 supersedes the line cap)

// ---- inside the Part 1 md loop, AFTER `fileDiff = stripIndexLines(fileDiff)` ----
	fileDiff = stripIndexLines(fileDiff)
	if opts.TokenLimit > 0 {
		mdDiffs = append(mdDiffs, fileDiff) // collect uncapped; the gate truncates md via the shared budget
		continue
	}
	// ==0 legacy per-file line cap (BYTE-IDENTICAL — regression anchor)
	if lines := strings.Split(fileDiff, "\n"); len(lines) > maxMDLines {
		fileDiff = strings.Join(lines[:maxMDLines], "\n") +
			fmt.Sprintf("\n... [diff truncated at %d lines]", maxMDLines)
	}
	b.WriteString(fileDiff)
	if !strings.HasSuffix(fileDiff, "\n") {
		b.WriteByte('\n')
	}

// ---- Part 2 non-md, AFTER `nmDiff = stripIndexLines(nmDiff)` ----
	nmDiff = stripIndexLines(nmDiff)
	if opts.TokenLimit > 0 {
		b.WriteString(applyWaterFillGate(mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens))
		return b.String(), nil // md + non-md both emitted by the gate (shared water-fill budget)
	}
	if len(nmDiff) > maxDiffBytes { // ==0 legacy aggregate byte cap (BYTE-IDENTICAL)
		nmDiff = nmDiff[:maxDiffBytes] +
			fmt.Sprintf("\n... [diff truncated at %d bytes]", maxDiffBytes)
	}
	b.WriteString(nmDiff)
	return b.String(), nil
```
```go
// NOTE on the `return b.String(), nil` inside the >0 branch: in StagedDiff the function ends right after
// Part 2 with `return b.String(), nil`. When the >0 branch delegates to applyWaterFillGate it has emitted
// everything (md + non-md), so it returns immediately. KEEP the ==0 fall-through to the existing byte-cap +
// b.WriteString(nmDiff) + return. TreeDiff/WorkingTreeDiff are identical in shape (confirm their exact
// return tail; some append `b.WriteString(nmDiff)` then `return b.String(), nil` on the next line).
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/git/tokengate.go (PURE helper + sectionBody + const)
  - FILE: NEW internal/git/tokengate.go. PACKAGE: `package git`. NO imports (it uses only in-package symbols:
      splitDiffSections, diffSectionPath, truncateByWaterFill, atAtRe, EstimateTokens, allocByWaterFill — all
      in `package git`). Paste the "Data models" skeleton. Add the package doc comment citing FR3d/FR3i + §5/§6.
  - DEFINE: `const tokenBudgetMargin = 1024`; `func sectionBody(section string) string`; `func
      applyWaterFillGate(mdDiffs []string, nmDiff, skeleton string, tokenLimit, promptReserve int) string`.
  - GOTCHA: NO `import` block (all deps are in-package). If go vet flags an unused import, remove it. Do NOT
      call waterFillLevel (use allocByWaterFill). Do NOT call git/run/exec. sectionBody MUST use atAtRe (D8).
  - RUN: gofmt -w internal/git/tokengate.go ; go build ./internal/git/ → exit 0.

Task 2: CREATE internal/git/tokengate_test.go (PURE deterministic table tests — mirror truncatediff_test.go)
  - FILE: NEW internal/git/tokengate_test.go. PACKAGE: `package git` (white-box). IMPORT: `strings`, `testing`
      (stdlib only). NO t.TempDir, NO git repo, NO I/O.
  - PATTERN: mirror truncatediff_test.go's table-driven style (`tests := []struct{…}{…}`; `t.Run(tc.desc, …)`).
  - Cases (HARDCODED expectations; build mdDiffs/nmDiff/skeleton as Go string literals with explicit \n):
      * BodyBudget_clamped: tokenLimit tiny (smaller than skeleton+reserve+margin) ⇒ bodyBudget 0 ⇒ output =
        sections passed through UNTRUNCATED (no `... [truncated]`; truncateByWaterFill pass-through). Assert
        strings.Count(out, "... [truncated]") == 0.
      * AllWithinBudget: sizes ≤ bodyBudget ⇒ allocByWaterFill returns sizes ⇒ no truncation ⇒ out contains
        every section verbatim (strings.Contains for each); Count("... [truncated]") == 0.
      * OneLargeCapped_fairness: 3 sections (A small body, B LARGE body, C small body); tokenLimit set so
        bodyBudget < size_B but > size_A+size_C. Assert: A and C WHOLE (strings.Contains(out, bodyA) &&
        Contains(out, bodyC) — note: assert the BODY content survives), B truncated (Count for B's section of
        "... [truncated]" == 1), total Count("... [truncated]") == 1. (Hardcode the bodies so sizes are known.)
      * SharedBudget_md_and_nm: mdDiffs=[mdSection], nmDiff= 2 sections; one nm section large ⇒ it's capped,
        the md section (small) whole. Asserts the md+nm share ONE budget (the large nm is capped even though
        md is separate). Count("... [truncated]") == 1 (on the nm section).
      * Skeleton_subtracted: two runs, same bodies, run1 skeleton="" run2 skeleton=<big>; run2's bodyBudget is
        smaller ⇒ a body that was whole in run1 is capped in run2. (Demonstrates skeleton_tokens subtracted.)
      * PromptReserve_subtracted: same idea with promptReserve (run1 reserve=0, run2 reserve=large).
      * PureRename_notTruncated: a section with no `@@` (rename only) ⇒ sectionBody "" ⇒ size 0 ⇒ never
        truncated even at tiny bodyBudget (Count("... [truncated]")==0 for it).
      * Empty: mdDiffs=nil, nmDiff="" ⇒ applyWaterFillGate returns "".
      * PathKeying: a section whose path diffSectionPath can't resolve (synthetic — ok=false) ⇒ the allotment
        map omits it ⇒ truncateByWaterFill pass-through (not truncated).
  - GOTCHA: to make sizes deterministic, build bodies of KNOWN rune length (e.g. a body of 400 runes ⇒
      EstimateTokens=100). Hardcode tokenLimit so the water level is predictable. Assert STRUCTURE
      (Contains/Count) not exact bytes where truncation is involved (the cutoff is allotment×4 runes —
      compute it to assert the body prefix survives if desired).
  - RUN: gofmt -w ; go test ./internal/git/ -run TestApplyWaterFillGate -v.

Task 3: EDIT internal/git/git.go — the >0 branch in StagedDiff
  - FILE: internal/git/git.go. In StagedDiff: (a) declare `var mdDiffs []string` before the Part 1 md loop;
      (b) inside the loop after `fileDiff = stripIndexLines(fileDiff)`, add `if opts.TokenLimit > 0 {
      mdDiffs = append(mdDiffs, fileDiff); continue }` BEFORE the existing line-cap; (c) in Part 2 after
      `nmDiff = stripIndexLines(nmDiff)`, add `if opts.TokenLimit > 0 { b.WriteString(applyWaterFillGate(
      mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens)); return b.String(), nil }` BEFORE
      the existing byte-cap.
  - GOTCHA: the `==0` path (line-cap + byte-cap + their sentinels + the final `b.WriteString(nmDiff)` +
      `return`) stays BYTE-IDENTICAL. Confirm `skeleton` is the local var name (it is — `skeleton, serr :=
      g.numstatSkeleton(…)`). Do NOT touch skeleton capture / binary / excludes / binExcludes.
  - RUN: gofmt -w internal/git/git.go ; go build ./... ; go test ./internal/git/ -run TestStagedDiff (the
      existing ==0 tests MUST still pass — regression anchor).

Task 4: EDIT internal/git/git.go — the >0 branch in TreeDiff and WorkingTreeDiff (near-verbatim of Task 3)
  - FILE: internal/git/git.go. Apply the EXACT same 3-point edit (declare mdDiffs; Part 1 collect; Part 2
      delegate) to TreeDiff (~L1295–1327) and WorkingTreeDiff (~L1450–1483). They are near-verbatim copies of
      StagedDiff (only the surrounding capture's diffArgs differ — `treeA, treeB` / none — which the gate does
      NOT touch; the gate works on already-captured text).
  - GOTCHA: confirm each function's return tail. Some do `b.WriteString(nmDiff)` then `return b.String(), nil`
      on the next line — the >0 branch returns inside the `if`; the ==0 fall-through reaches the existing
      WriteString+return. Keep both paths. Do NOT alter the ==0 cap literals (L1302/1326/1458/1482).
  - RUN: gofmt -w ; go build ./... ; go test ./internal/git/ -run 'TreeDiff|WorkingTreeDiff' (==0 regression).

Task 5: CREATE internal/git/difftokenlimit_test.go (e2e temp-repo >0 tests + ==0 regression)
  - FILE: NEW internal/git/difftokenlimit_test.go. PACKAGE: `package git`. Reuse the repo helpers from
      stagediff_test.go (initRepo/writeFile/stageFile/gitOut — same package, accessible). IMPORT stdlib + testing.
  - TestStagedDiff_TokenLimitZero_LegacyCaps (REGRESSION): temp repo, stage a file exceeding maxDiffBytes,
      call StagedDiff with TokenLimit=0 → assert the `... [diff truncated at %d bytes]` sentinel IS present
      (the legacy path; byte-identical). Also a markdown file exceeding maxMDLines → the `... lines` sentinel.
      (This pins the ==0 anchor at the gate level.)
  - TestStagedDiff_TokenLimitGt0_WaterFill (the item's e2e): temp repo; commit a baseline; create ONE HUGE
      file (e.g. 20000 runes of generated content) + ONE SMALL file (a 1-line tweak); stage both. Call
      StagedDiff with TokenLimit set small enough that the huge file must be capped (e.g. TokenLimit=4000,
      PromptReserveTokens=0). Assert: (a) the small file's body is WHOLE (strings.Contains(out, smallFileMarker)
      — e.g. its unique changed line); (b) the huge file is CAPPED (strings.Contains(out, "... [truncated]"));
      (c) the FR3g skeleton is present (strings.Contains(out, "Change summary (numstat")); (d) the legacy
      `at N bytes` sentinel is ABSENT (strings.Count(out, "diff truncated at") == 0); (e) EstimateTokens(out)
      ≤ TokenLimit + a small slack (the bodies fit the budget; the skeleton/headers are overhead).
  - TestStagedDiff_TokenLimitGt0_AllFits (common case): TokenLimit LARGE → no truncation (Count(
      "... [truncated]")==0), every file whole.
  - TestTreeDiff_TokenLimitGt0 / TestWorkingTreeDiff_TokenLimitGt0: same shape against TreeDiff (two trees)
      and WorkingTreeDiff (unstaged) — proves the gate is wired into all 3 (FR3c parity).
  - GOTCHA: the huge file's body must exceed the water level L (set TokenLimit small). Use a deterministic
      generator (e.g. strings.Repeat("x\n", N)) so the test is stable. The small file needs a unique marker
      line to assert wholeness.
  - RUN: gofmt -w ; go test ./internal/git/ -run 'TokenLimit' -v.

Task 6: VALIDATE (run all gates; fix before declaring done)
  - gofmt -w internal/git/tokengate.go internal/git/tokengate_test.go internal/git/difftokenlimit_test.go
       internal/git/git.go
  - go vet ./internal/git/ && go build ./...
  - go test ./internal/git/ -v   (NEW pure tests + e2e + ALL existing ==0 cap tests — the regression anchor)
  - go test ./...   (ALL green — the >0 path only activates when opts.TokenLimit>0; default 0 ⇒ byte-identical)
  - gofmt -l internal/git/ empty; git diff --exit-code go.mod go.sum empty.
  - git status shows EXACTLY 4 files: git.go (edited), tokengate.go, tokengate_test.go, difftokenlimit_test.go
       (new). truncatediff.go/waterfill.go/tokens.go/numstat.go/skeleton.go/binary.go/StagedDiffOptions
       UNCHANGED.
```

### Implementation Patterns & Key Details

```go
// PATTERN: pure helper + capture/IO split (D4). applyWaterFillGate is the ONLY thing that knows the budget +
//   assembly; the 3 diff functions only CAPTURE text (their existing git calls) and DELEGATE. Pure ⇒ the
//   fairness logic is unit-testable without a repo (tokengate_test.go); the git.go edits are 3-line branches.

// PATTERN: branch AROUND the cap, not through it (D1/D7). `if opts.TokenLimit > 0 {<delegate>} else {<existing
//   cap>}`. The ==0 else-branch is byte-identical to pre-M4 — the regression anchor. Never refactor the cap
//   into the >0 path.

// CRITICAL (coherence — D2/D8): sizes[i] = EstimateTokens(sectionBody(sections[i])) where sectionBody splits
//   at atAtRe — the SAME split truncateByWaterFill uses. Same body, same estimator ⇒ the water-fill's "file >
//   L" (sizes) and the enforcement's "EstimateTokens(body) > allotment" AGREE ⇒ exact fairness.

// CRITICAL (one shared budget — D3): md + non-md sections sized/allocated/truncated TOGETHER. applyWaterFillGate
//   concatenates mdDiffs + splitDiffSections(nmDiff) before allocByWaterFill.

// GOTCHA (the >0 Part 2 returns inside the if): the gate emits md + non-md (both), so the >0 branch returns
//   b.String() immediately; the ==0 fall-through reaches the existing byte-cap + WriteString + return.

// GOTCHA (tokenBudgetMargin is the ONLY new const — F9): define it in tokengate.go. It is the safety floor;
//   do NOT also subtract a second margin or change EstimateTokens.

// GOTCHA (don't touch the ==0 cap literals): the 6 `... [diff truncated at %d bytes/lines]` lines are the
//   regression anchor. The >0 path emits ONLY `... [truncated]` (via truncateByWaterFill); the legacy forms
//   NEVER appear on the >0 path.
```

### Integration Points

```yaml
GATE (applyWaterFillGate — pure, tokengate.go):
  - consume: "splitDiffSections(nmDiff), EstimateTokens(skeleton + each body), allocByWaterFill(sizes, bodyBudget),
    diffSectionPath (map key), truncateByWaterFill(sections, allotments)."
  - budget: "bodyBudget = max(0, tokenLimit − EstimateTokens(skeleton) − promptReserve − tokenBudgetMargin)."

DIFF FUNCTIONS (git.go — 3 sites):
  - StagedDiff: "Part 1 md: collect mdDiffs when >0; Part 2: applyWaterFillGate(mdDiffs, nmDiff, skeleton, …) when >0."
  - TreeDiff / WorkingTreeDiff: "identical edit (near-verbatim functions; only diffArgs differs — untouched)."
  - preserve: "the ==0 path (line/byte caps + sentinels) byte-identical; skeleton/binary/excludes unchanged."

FROZEN/LEAVE (do NOT edit):
  - truncatediff.go (P1.M4.T2.S2), waterfill.go (P1.M4.T2.S1), tokens.go (P1.M4.T1.S1).
  - numstat.go / skeleton.go / binary.go (siblings). StagedDiffOptions (P1.M1.T2.S1 — READ TokenLimit/PromptReserveTokens).
  - The 6 call sites mapping cfg→opts (P1.M1.T2.S2) — they already thread the fields.

GO.MODULE: change NONE. stdlib only; all deps in-package.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/git/tokengate.go internal/git/tokengate_test.go internal/git/difftokenlimit_test.go internal/git/git.go
go vet ./internal/git/
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; gofmt -l empty. tokengate.go should need NO import block (all deps in-package).
```

### Level 2: Unit + Component tests

```bash
# The pure gate — deterministic, no repo:
go test ./internal/git/ -run TestApplyWaterFillGate -v
# The e2e gate tests (temp repo):
go test ./internal/git/ -run 'TokenLimit' -v
# The REGRESSION ANCHOR — existing ==0 cap tests MUST still pass byte-identical:
go test ./internal/git/ -run 'StagedDiff|TreeDiff|WorkingTreeDiff' -v
# Full git package + whole repo:
go test ./internal/git/ -v
go test ./...
# Expected: all green. The >0 path only activates when opts.TokenLimit>0; default 0 ⇒ byte-identical legacy.
```

### Level 3: Integration Testing (the real gate, end-to-end)

```bash
make build

# --- ==0 regression: a huge staged file with NO token_limit ⇒ the legacy byte-cap sentinel ---
T=$(mktemp -d); cd "$T"; git init -q .; git config user.email t@t.co; git config user.name t
git commit -q --allow-empty -m base
python3 -c "print('x\n'*200000)" > big.txt; git add big.txt
HOME="$T" XDG_CONFIG_HOME="$T" ../../bin/stagehand --dry-run --no-color 2>/dev/null 1>msg.txt
grep -q 'diff truncated at' msg.txt && echo "PASS: ==0 legacy byte-cap sentinel present" || echo "FAIL"
# Expected: the `... [diff truncated at N bytes]` sentinel IS present (byte-identical legacy).

# --- >0 water-fill: token_limit set ⇒ big file capped (short sentinel), skeleton present, no legacy sentinel ---
cat > "$T/.stagehand.toml" <<'EOF'
[generation]
token_limit = 4000
EOF
HOME="$T" XDG_CONFIG_HOME="$T" ../../bin/stagehand --dry-run --no-color 2>/dev/null 1>msg2.txt
grep -q '\.\.\. \[truncated\]'       msg2.txt && echo "PASS: >0 water-fill sentinel present"
grep -q 'Change summary (numstat'    msg2.txt && echo "PASS: FR3g skeleton present"
! grep -q 'diff truncated at'        msg2.txt && echo "PASS: legacy 'at N bytes' sentinel ABSENT on >0"
# Expected: `... [truncated]` present, skeleton present, `diff truncated at` absent.

# --- >0 fairness: a tiny file alongside the huge one ⇒ tiny file WHOLE ---
echo "tiny tweak" > small.txt; git add small.txt
HOME="$T" XDG_CONFIG_HOME="$T" ../../bin/stagehand --dry-run --no-color 2>/dev/null 1>msg3.txt
grep -q 'tiny tweak' msg3.txt && echo "PASS: small file body WHOLE under water-fill"
grep -q '\.\.\. \[truncated\]' msg3.txt && echo "PASS: large file still capped"
# Expected: the small file's content survives; the large file carries the truncated sentinel.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Coherence check: the >0 output's body tokens ≤ bodyBudget (within the margin/overhead slack).
# (Compute via the same chars/4 estimator the gate uses — a Go one-liner or python ceil(len_runes/4).)
go test ./internal/git/ -run TestStagedDiff_TokenLimitGt0_WaterFill -v   # asserts EstimateTokens(out) ≤ limit + slack

# 3-path parity: run the >0 gate through StagedDiff, TreeDiff, WorkingTreeDiff — each must water-fill
# (FR3c parity across all three diff paths):
go test ./internal/git/ -run 'Test(TreeDiff|WorkingTreeDiff)_TokenLimitGt0' -v

# Race + full regression (the gate):
go test -race ./...
go vet ./...
gofmt -l internal/ pkg/ cmd/
# Expected: all green; exactly 4 files changed (git.go + 3 new). Default token_limit==0 ⇒ zero behavior change.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `go test ./internal/git/` green: NEW pure tests + e2e + ALL existing `==0` cap tests (regression anchor).
- [ ] `go test ./...` green; `go vet ./...` clean; `gofmt -l internal/git/` empty; go.mod/go.sum unchanged.

### Feature Validation

- [ ] `token_limit==0` ⇒ byte-identical legacy caps + `... [diff truncated at N bytes/lines]` sentinels (the 6
      literals untouched; existing tests pass unchanged).
- [ ] `token_limit>0` ⇒ water-fill: small files WHOLE (no sentinel), large files CAPPED (`... [truncated]`,
      headers preserved), skeleton present, `at N bytes/lines` sentinels ABSENT, total ≤ token_limit + slack.
- [ ] The gate is wired into ALL THREE diff functions (StagedDiff/TreeDiff/WorkingTreeDiff — FR3c parity).
- [ ] applyWaterFillGate is PURE (no git/ctx/I/O); sizes via EstimateTokens(sectionBody) (coherent with enforcement).

### Code Quality Validation

- [ ] applyWaterFillGate is a pure helper in a new file (unit-testable without a repo); git.go edits are 3-line branches.
- [ ] The `==0` path is byte-identical (no refactor of the cap logic); only an additive `>0` branch.
- [ ] File placement matches the desired tree (git.go edited; tokengate.go + 2 test files new).
- [ ] Anti-patterns avoided (see below): no numstat-line-count sizing, no separate md/non-md budgets, no ==0
      cap changes, no re-strip of index lines, no allocByWaterFill bypass.

### Documentation & Deployment

- [ ] Doc comments cite FR3d/FR3i, system_context §5/§6, the coherence rationale, the regression anchor, and
      the frozen consumer contract (the sibling P1.M4.T2.S2).
- [ ] tokenBudgetMargin documented (what it absorbs; tunable).

---

## Anti-Patterns to Avoid

- ❌ Don't size from numstat line counts (a proxy) — size from `EstimateTokens(sectionBody)` so the water-fill's
  fairness AGREES with `truncateByWaterFill`'s enforcement (coherence; D2). numstatRows is the skeleton (dual-use).
- ❌ Don't give markdown and non-markdown SEPARATE water-fill budgets — FR3i is "across files" (one shared
  budget; D3). applyWaterFillGate concatenates md + nm sections before allocating.
- ❌ Don't touch the `==0` cap path — it is the byte-identical regression anchor (system_context §6 inv 1). Wrap
  it in `if >0 {…} else {<existing>}`; never refactor the cap literals/sentinels into the `>0` path.
- ❌ Don't emit the legacy `... [diff truncated at N bytes/lines]` sentinel on the `>0` path — the `>0` path
  emits ONLY `... [truncated]` (via `truncateByWaterFill`). The two forms are separated by the branch (§6 inv 2).
- ❌ Don't re-strip `index` lines or re-apply `-M`/`-U<n>` — those are in the capture step (M2/M3), upstream of
  the gate; the sections the gate sees are already shaped + index-stripped.
- ❌ Don't call `waterFillLevel` directly — call `allocByWaterFill` (the consumer-facing allocator; the level is
  the solver's internal detail).
- ❌ Don't edit `truncatediff.go`/`waterfill.go`/`tokens.go`/`StagedDiffOptions` — they're the siblings' frozen
  territory. CONSUME read-only. The only edit is git.go (+ 3 new files).
- ❌ Don't compute `body_budget` without the margin, or change `EstimateTokens` to "improve" accuracy — the
  margin absorbs the estimation gap; the estimator is the single consistent unit (tokens.go).
- ❌ Don't special-case idempotency or the degenerate `bodyBudget≤0` — it falls out of `allocByWaterFill`→all-0
  ⇒ `truncateByWaterFill` pass-through (D10). Just clamp the budget at 0.
