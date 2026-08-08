# Research Findings — P1.M1.T1.S4 (Decompose() dispatch wiring)

Verified by direct source read of `internal/decompose/decompose.go` + `decompose_test.go`
+ `internal/ui/verbose.go` + `internal/decompose/roles.go` (2026-08-08).

## 0. State of the predecessor subtasks (the contract S4 consumes)

| Subtask | Status (verified) | Location | What S4 consumes |
|---|---|---|---|
| S1 `isFileDisjoint` | **LANDED** | `decompose.go:450` | the gate predicate: `isFileDisjoint([]prompt.PlannerCommit) bool` |
| S2 `runLoopFastPath` sweep | **LANDED** | `decompose.go:662` | the function signature + serial sweep |
| S3 `runLoopFastPath` concurrent | **IN PROGRESS** (sentinel still present: `grep -c "not yet implemented"` == 1) | `decompose.go:662` | the COMPLETE function returning `([]CommitResult, []ChainEntry, error)` |

S4 treats S3's PRP as a CONTRACT: by the time S4 lands, `runLoopFastPath` returns the SAME
`(commits, chainData, err)` shapes as `runLoop`, so the post-loop code (error block + arbiter
phase) feeds identically from either branch. **S4 must NOT edit `runLoopFastPath` or `runLoop`.**

## 1. The dispatch site — exact text to replace (decompose.go:235-237)

```go
	// (5) Safety cap is enforced inside callPlanner (auto mode). Forced mode: user asserted N — no cap.

	// (6) The loop (1-deep overlap, FR-M8 empty-skip, serialized CAS, FR-M12 isolation).
	commits, chainData, err := runLoop(ctx, deps, out.Commits, baseTree, tStart, preRunHEAD, isUnborn)
```

The single line `:237` is the ONLY line that changes. The error block (`:238-242`,
`return DecomposeResult{Commits: commits}, err`) + arbiter phase (`:248+`) are SHARED and
MUST remain byte-identical — both loop variants return the same 3-tuple shape.

**Edit shape**: `:=` (short-decl) → must pre-declare (`var commits []CommitResult; var chainData []ChainEntry; var err error`)
then `=` in each branch, OR wrap the dispatch in an IIFE returning the 3-tuple. Pre-declaring is
more idiomatic; see PRP Implementation Blueprint.

## 2. Variables in scope at :237 (all live — confirmed by reading Decompose)

`ctx`, `deps Deps`, `out prompt.PlannerOutput` (here `out.Single==false`, `len(out.Commits)>=1`),
`baseTree`, `tStart`, `preRunHEAD`, `isUnborn`. `out.Commits` is `[]prompt.PlannerCommit` —
exactly the `isFileDisjoint` input. No new variable needed.

## 3. Logging the gate decision — `ui.Verbose` API

- `deps.Verbose` is `*ui.Verbose` (`roles.go:60`), **nil-safe** — guard `if deps.Verbose != nil`.
- Free-form logger: `v.VerboseWarn(msg string)` (`ui/verbose.go:106`) — the method used for
  one-line progress notes (see `generate.go:472`, `default_action.go:207`). There is NO generic
  `Verbose(msg)`; `VerboseWarn` is the right call.
- `ui.NewVerbose(w io.Writer, on bool) *Verbose` (`ui/verbose.go:35`) — test constructor.
  `dcmDeps` sets `Verbose: nil`, so existing tests observe nothing (fine). A dispatch test that
  wants to assert the log sets `Verbose: ui.NewVerbose(&buf, true)` and greps the buffer.
  **STRONGER signal**: assert the dispatch via the stager seam (called on runLoop, NOT called on
  the fast-path) rather than a log string — see §5.

## 4. ⚠️ THE REGRESSION SURFACE (the thing S4 must not miss)

`isFileDisjoint` returns `true` when no path appears in ≥2 concepts — **including the vacuous
case where every concept has empty `Files`** (S1's literal-count algorithm: empty map → true).

**Almost every existing multi-commit `Decompose` test stubs the planner with empty `files`** and
relies on the stager seam (`deps.stager = dcmStagerSeam(...)`, keyed by concept TITLE) to stage.
Trace after S4 wires the gate:

```
plannerJSON: {"count":3,...,"commits":[{"title":"c1",...},{"title":"c2",...}]}   // NO "files"
→ out.Commits = [{c1, []}, {c2, []}, ...]
→ isFileDisjoint(out.Commits) == true   // vacuous
→ runLoopFastPath(...)
→ for each concept: deps.Git.Add(ctx, [])  // no-op (empty Files)
→ treeI == prevTree (index still == baseTree; untracked files unstaged)
→ FR-M8 skip every concept
→ returns (nil, nil, nil)
→ DecomposeResult{Commits: nil}   // arbiter gate `len(commits)>0` is false
→ test asserts len(Commits)==N  →  FAIL
```

**Affected tests** (multi-commit, non-single planner JSON with empty `files`, driven through
`Decompose`): ~25 — including `TestDecompose_AutoMultiCommit_HappyPath` (:464),
`_TemplateAppliedUniformly` (:540), `_Overlap` (:580), `_EmptyConceptSkip` (:646),
`_StagerMovedHEAD` (:689), `_StagerFreezeViolation` (:727), `_ConcurrentChangeExclusion` (:955),
`_ArbiterFoldsOnlyTStart` (:1042), `_TStartCompleteness` (:1123), `_StagerGuardHappyPath` (:1184),
`_SafetyCap` (:1252), `_ArbiterSkippedOnCleanTree` (:1292), `_ArbiterWiring` (:1334),
`_ErrorPropagation_Stager` (:1393), `_ErrorPropagation_RescueError` (:1433), `_UnbornRepo` (:1463),
`_MessageRescuePartial` (:1528), `_CASAbortPartial` (:1641), `_StagerRetryThenEmpty` (:1757),
`_RescueArbiterSkipped` (:1849), `_StagerRetryThenSuccess` (:1905),
`_OneFileShortcut_Deletion` (:2119), `_ArbiterTipAmend_RereadsFinalSHA` (:2385),
`_ArbiterMidChain_AllSHAsResolve` (:2446), `_HappyPath_CommitsAccurate` (:2504),
`_RoleResolvesSubProvider` (:2539), `_PlannerCoverageLogsUnclaimed` (:2661).

**Already non-disjoint (shared `store.py`) — UNAFFECTED, stay on runLoop naturally**:
`TestDecompose_HunkSplitAcrossConcepts` (:848), `_HunkSplit_RejectsOffTStartContent` (:906).
These are the canonical FR-M5 hunk-split tests and are the EXISTING runLoop coverage S4 must preserve.

**Edge**: `TestDecompose_TokenLimitInvariant_PlannerPromptFits` (:2673) declares DISJOINT real files
(`a.txt`/`b.txt`) + a stager seam. After S4 it routes to the fast-path; the fast-path `Add`s the
declared files → same 2 commits → test likely still PASSES (its seam is never called, but it
asserts commits land, not seam invocation). Confirm during implementation; if it asserts seam
invocation, apply the §6 remediation.

## 5. The stager seam is the dispatch oracle (system_context §6)

`deps.stager` (`roles.go:80`, unexported, nil in prod) is reached ONLY via
`invokeStager` ← `invokeStagerRetry` ← `runLoop`'s per-concept loop. **The fast-path never calls
it.** So a test injects `deps.stager = func(...) error { t.Fatal("stager called on fast-path") }`
and drives `Decompose` with disjoint files → if the test passes, the fast-path was taken (stager
unreachable). Conversely, on the runLoop path a normal seam IS called. This is a far stronger
dispatch assertion than grepping a verbose log.

## 6. Remediation rule (keep existing tests on runLoop — S5 owns fast-path coverage)

To keep an existing test on its ORIGINAL path (runLoop) after the gate is wired, make its planner
partition NON-disjoint so `isFileDisjoint` returns false. **Minimal, mechanical, works for both
single- and multi-concept tests**: ensure at least one path appears ≥2 times across the concepts'
`files`. Concretely:

- **Multi-concept test**: list the SAME file in ≥2 concepts. E.g. add `"files":["store.py"]` to
  two concepts that already share staging (mirror `HunkSplit` :863-865), OR append a sentinel.
- **Single-concept test**: list one file TWICE within the concept (intra-concept duplicate) — S1's
  literal-count algorithm counts it as 2 → returns false (S1's test matrix documents this as the
  "intra-concept duplicate disqualifies" case). E.g. `"files":["a.txt","a.txt"]`.

**Why a fake/duplicated file is safe on runLoop**:
- runLoop's stager seam stages per its OWN map (`conceptFiles[concept.Title]`), IGNORING
  `concept.Files` for actual staging → the duplicated/extra entry is never `git add`-ed.
- `verifyFreezeSubset` checks the REAL staged tree ⊆ T_start → unaffected by the planner's file list.
- `checkPlannerCoverage` (`:291`, diagnostic-only, never aborts) compares claimed vs
  `DiffTreeNames`; an extra claimed-but-not-real path is simply absent from the changed set →
  not flagged. No validation anywhere requires planner `files` to exist.

Net: ZERO change to what gets staged/committed; only the gate decision flips back to runLoop.
This preserves ALL existing runLoop coverage. S5 (`Fast-path regression suite`) adds dedicated
fast-path + fallback tests.

> **Alternative (NOT recommended for S4)**: declare DISJOINT real files in each affected test so it
> flips to the fast-path (same commits). This GUTS runLoop's top-level coverage (only the 2 HunkSplit
> tests remain) — leave it to S5 to add fast-path coverage and KEEP existing tests on runLoop.

## 7. S4's own dispatch tests (2 focused, through Decompose — not direct runLoopFastPath calls)

S3's `TestRunLoopFastPath_*` tests call `runLoopFastPath` DIRECTLY (pre-dispatch). After S4, the
DISPATCH (the gate inside `Decompose`) has no end-to-end coverage until these land:

- **A. `TestDecompose_Dispatch_DisjointFastPath`**: 3 disjoint files (a/b/c) modified; planner
  returns disjoint `files` per concept; `deps.stager = func(...) { t.Fatal("fast-path must not call stager") }`.
  Assert: `Decompose` succeeds, 3 commits in CAS order, HEAD advanced, clean tree. PROVES disjoint → fast-path
  + stager bypassed (FR-M13: "invoking no stager agent").
- **B. `TestDecompose_Dispatch_SharedFileFallback`**: 1 shared file split across 2 concepts (mirror
  HunkSplit :848's `store.py` setup); a stager seam that stages per concept + sets a flag. Assert:
  `Decompose` succeeds, 2 commits, seam WAS called (flag set). PROVES shared → runLoop + stager invoked.

Both reuse the harness idiom of `TestDecompose_AutoMultiCommit_HappyPath` (:464) and
`TestDecompose_HunkSplitAcrossConcepts` (:848).

## 8. No other callers

`grep` confirms `runLoop(`/`isFileDisjoint(`/`runLoopFastPath(` have exactly ONE non-test,
non-definition call site: `decompose.go:237` (the dispatch). So S4's edit is the single wiring
point — no other code paths to update.

## 9. Scope fences (what S4 does NOT touch)

- `runLoop` (byte-identical), `runLoopFastPath` (S2/S3 own it — S4 only CALLS it),
  `isFileDisjoint` (S1 owns it — S4 only CALLS it).
- The error block (`:238-242`) + arbiter phase (`:248+`) + `rereadFinalCommits` + result assembly
  (SHARED, unchanged).
- `arbiter.go`, `chain.go`, `roles.go`, `message.go`, `stager.go`, the Git interface, the signal
  package — UNCHANGED.
- No new config key / CLI flag (FR-M13: "adds no configuration"). No docs (S6).
- The comprehensive fast-path regression suite is S5; S4 adds only the 2 dispatch tests + the
  existing-test remediation needed to keep CI green.