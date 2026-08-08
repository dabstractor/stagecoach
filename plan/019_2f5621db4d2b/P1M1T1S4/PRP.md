name: "P1.M1.T1.S4 — Decompose() dispatch wiring: gate isFileDisjoint → runLoopFastPath else runLoop (FR-M13)"
description: >
  Wire the FR-M13 dispatch inside Decompose() (internal/decompose/decompose.go:237): replace the single
  `commits, chainData, err := runLoop(...)` line with a gate `if isFileDisjoint(out.Commits) {
  runLoopFastPath(...) } else { runLoop(...) }`, logging the decision at deps.Verbose (VerboseWarn,
  nil-guarded). Everything after (the FR-M12 error block, arbiter phase, rereadFinalCommits,
  DecomposeResult assembly) is SHARED and byte-identical — both loop variants return the same
  (commits, chainData, err) 3-tuple shape. runLoop + runLoopFastPath + isFileDisjoint are UNCHANGED
  (S1/S2/S3 own them; S4 only CALLS them). No config key / CLI flag / docs (FR-M13; docs are S6).
  CRITICAL SECONDARY DELIVERABLE: ~25 existing multi-commit Decompose tests stub the planner with EMPTY
  `files` (relying on the stager seam), which the new gate routes to the fast-path → stages nothing → 0
  commits → regression. S4 MUST remediate by making each affected test's planner partition NON-disjoint
  (so it stays on its original runLoop path; S5 owns new fast-path coverage) — see Implementation Task 3.
  S4 also adds 2 focused DISPATCH tests (disjoint→fast-path/stager-bypassed; shared→runLoop/stager-called)
  proving the gate end-to-end through Decompose.

---

## Goal

**Feature Goal**: Activate the FR-M13 file-disjoint fast-path end-to-end by wiring the dispatch gate inside
`Decompose()`: when the planner's partition is pairwise file-disjoint (`isFileDisjoint(out.Commits)` true),
route to `runLoopFastPath` (the deterministic, concurrent-message path S1+S2+S3 built); otherwise route to
`runLoop` (the shared-file / hunk-split fallback, byte-identical). Both paths feed the shared post-loop
code (error block + arbiter) identically. This is the single insertion point that turns the greenfield
fast-path into live behavior.

**Deliverable**:
1. The dispatch gate in `Decompose()` (`decompose.go:237`) — replaces the one `runLoop` call line with the
   `if isFileDisjoint(...) { runLoopFastPath } else { runLoop }` gate + a `VerboseWarn` decision log.
2. **Existing-test remediation**: every affected multi-commit `Decompose` test (≈25) gets a non-disjoint
   planner partition so it stays on runLoop (its original path) after the gate is wired — keeping CI green
   and preserving all existing runLoop coverage. (Comprehensive fast-path coverage is S5.)
3. **2 focused dispatch tests** through `Decompose`: disjoint→fast-path (stager bypassed) and
   shared→runLoop (stager called).

**Success Definition**:
- A disjoint partition routes to `runLoopFastPath`; a shared-file partition routes to `runLoop`. The gate's
  decision is logged at `deps.Verbose` (when non-nil).
- Both paths return `(commits, chainData, err)` in identical shapes, so the FR-M12 error block, the arbiter
  gate (`DiffTreeNames(tipTree, T_start)`), `rereadFinalCommits`, and `DecomposeResult` assembly run
  UNCHANGED on either branch.
- `runLoop`, `runLoopFastPath`, and `isFileDisjoint` are byte-identical (S1/S2/S3 own them; S4 only calls).
- The full existing `decompose` test suite passes (the ≈25 remediated tests stay green on runLoop; the 2
  HunkSplit tests were already non-disjoint and are unaffected).
- `go build ./...`, `go vet ./...`, `gofmt -l`, `go test ./internal/decompose/... -race`, `make test`,
  `make lint` all pass.

## User Persona (if applicable)

**Target User**: Stagecoach end users (finally). S4 is the wiring that makes S1–S3 user-visible: a user with
a dirty tree of unrelated whole-file changes runs `stagecoach`; the planner partitions disjointly; the run
now (a) stages deterministically with `git add` — **no stager agent** — and (b) generates all N messages
concurrently (FR-M13/FR-M14), collapsing the critical path. A provider with no `tooled_flags` (opencode,
qwen-code) can now decompose a disjoint tree where it previously could not serve the stager role at all.

**Use Case**: "I edited 6 unrelated files; stagecoach should make 6 clean commits fast, without spinning up
a tooled stager agent per concept."

**Pain Points Addressed**: FR-M13 (P0) — the automatic, deterministic, config-free file-disjoint fast-path.
Also closes the FR-D4 gap (opencode/qwen-code stager) for the common disjoint case.

## Why

- **FR-M13 (P0)**: "When the planner's partition is pairwise file-disjoint — no path appears in more than one
  concept's `files` — stagecoach stages each concept deterministically with `git add`, invoking no stager
  agent... The gate is automatic and deterministic (a set-membership test over the planner's `files`); it
  adds no configuration, and the fast-path and the fallback produce commits held to the same FR-M1c/FR-M7/
  FR-M8 guarantees." S1 built the gate; S2/S3 built the fast-path; **S4 is the single line that connects
  them to `Decompose`** — without it, the fast-path is dead code.
- **FR-M1d / arbiter parity**: the fast-path and runLoop produce `tipTree`/`chainData` in identical shapes,
  so the arbiter (which keys on the frozen `DiffTreeNames(tipTree, T_start)`) runs identically after either
  path. For a disjoint partition whose union == T_start's path-set, `tipTree == T_start` → arbiter naturally
  skipped (`len(leftoverPaths)==0`).
- **Why a single-line gate**: the post-loop code was always path-agnostic (it consumes `commits`/`chainData`,
  not loop internals). S4's gate is the ONLY seam; everything downstream is shared.

## What

**User-visible behavior**: A disjoint decomposition now runs the fast-path (deterministic staging +
concurrent messages). A shared-file decomposition runs runLoop (tooled stager + 1-deep overlap) exactly as
before. `--verbose` logs which path was taken.

**Technical change (one function edited + existing tests remediated + 2 dispatch tests):**
1. `Decompose()` (:237): replace the one `runLoop` call with the `isFileDisjoint` gate + a `VerboseWarn`
   decision log. Pre-declare `commits`/`chainData`/`err` so both branches assign.
2. Remediate ≈25 existing multi-commit tests so their planner partitions are non-disjoint (stay on runLoop).
3. Add 2 focused dispatch tests through `Decompose`.

### Success Criteria
- [ ] `Decompose()` dispatches disjoint → `runLoopFastPath`, shared → `runLoop`, via `isFileDisjoint(out.Commits)`.
- [ ] The gate decision is logged once via `deps.Verbose.VerboseWarn(...)` (nil-guarded).
- [ ] The error block (:238-242), arbiter phase (:248+), `rereadFinalCommits`, and `DecomposeResult` assembly are byte-identical.
- [ ] `runLoop`, `runLoopFastPath`, `isFileDisjoint` are byte-identical.
- [ ] ≈25 existing multi-commit tests remediated to non-disjoint partitions (stay on runLoop) — full suite green.
- [ ] `TestDecompose_Dispatch_DisjointFastPath`: disjoint files → fast-path → N commits; injected stager seam is NEVER called (t.Fatal guard).
- [ ] `TestDecompose_Dispatch_SharedFileFallback`: shared file → runLoop → N commits; stager seam IS called.
- [ ] No new config key / CLI flag / docs surface.
- [ ] `go build ./...`, `make test -race`, `make lint` pass.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact line to replace (with the exact surrounding comment block), every variable in scope
(named + type), the two loop-variant signatures to call, the `VerboseWarn` logging method + nil-guard, the
regression-surface analysis (WHY ≈25 tests break + the minimal mechanical remediation with a worked example),
the stager-seam dispatch oracle (how to ASSERT which path was taken), the 2 dispatch tests' bodies, the test
harness idiom to mirror, and the scope fences are all below.

### Documentation & References

```yaml
- file: internal/decompose/decompose.go
  why: "THE home. Decompose() (:144) contains the dispatch site at :237 — the ONE line to change. runLoop
        (:479) + runLoopFastPath (:662) + isFileDisjoint (:450) are the functions to CALL (UNCHANGED). The
        error block (:238-242) + arbiter phase (:248+) are SHARED — do NOT touch. ErrDecomposeFailed is the
        wrap sentinel (unused by the gate itself). msgOut/ChainEntry/CommitResult types are unchanged."
  pattern: "Replace `commits, chainData, err := runLoop(ctx, deps, out.Commits, baseTree, tStart, preRunHEAD, isUnborn)`
            with a pre-declared var block + the if/else gate calling runLoopFastPath else runLoop (same 7 args)."
  critical: "Both loop variants have the IDENTICAL signature
             (ctx, deps Deps, concepts []prompt.PlannerCommit, baseTree, tStart, preRunHEAD string, isUnborn bool)
             ([]CommitResult, []ChainEntry, error) — confirmed by grep. The gate is a pure routing decision; it
             does NO work itself. runLoop + runLoopFastPath + isFileDisjoint MUST stay byte-identical."

- file: internal/decompose/decompose_test.go
  why: "Test home (package decompose). The dispatch tests call Decompose() top-level (mirror TestDecompose_Auto
        MultiCommit_HappyPath :464 + TestDecompose_HunkSplitAcrossConcepts :848). Helpers: dcmInitRepo, dcmWriteFile,
        dcmDeps, dcmAllRoles, dcmPlannerManifest, dcmMessageScriptManifest, dcmStagerSeam, dcmGitOut, dcmLogCount,
        dcmStatusPorcelain, dcmHeadSHA. ≈25 existing multi-commit tests need non-disjoint remediation (Task 3)."
  pattern: "Mirror HappyPath :464 (disjoint real files + a stager seam that t.Fatals → fast-path proof) and
            HunkSplit :848 (shared store.py + a stager seam that stages + flags → runLoop proof)."
  gotcha: "Existing tests set Verbose: nil (dcmDeps). To assert the gate log, set Verbose: ui.NewVerbose(&buf, true)
           and grep the buffer — but the STAGER-SEAM signal (called vs t.Fatal) is stronger; prefer it."

- file: internal/ui/verbose.go
  why: "VerboseWarn (:106) is the free-form one-line logger (used by generate.go:472, default_action.go:207).
        NewVerbose(w io.Writer, on bool) *Verbose (:35) — test constructor. NO generic Verbose(msg) exists."
  pattern: "if deps.Verbose != nil { deps.Verbose.VerboseWarn(\"...\") }"

- file: internal/decompose/roles.go
  why: "Deps struct (:55): Git, Config, Roles, Verbose *ui.Verbose (:60, nil-safe), Out io.Writer, stager
        (unexported test seam :80). deps.stager is reached ONLY via runLoop (invokeStager ← invokeStagerRetry);
        the fast-path NEVER calls it (system_context §6). This is the dispatch oracle for tests."

- docfile: plan/019_2f5621db4d2b/architecture/system_context.md
  why: "§1 the dispatch site (exact insertion point + variables in scope); §5 arbiter-runs-unchanged-after
        either path; §6 stager-seam-unreachable-on-fast-path (the dispatch test oracle)."

- docfile: plan/019_2f5621db4d2b/P1M1T1S4/research/findings.md
  why: "The regression-surface analysis (§4), the remediation rule with worked example (§6), the stager-seam
        oracle (§5), the 2 dispatch tests (§7), scope fences (§9). READ THIS before editing."
  critical: "§4 is the reason S4 is NOT a 5-minute edit: wiring the gate makes every empty-files planner stub
             route to the fast-path (stages nothing → 0 commits → ≈25 tests fail). Task 3 is mandatory."

- docfile: plan/019_2f5621db4d2b/P1M1T1S3/PRP.md
  why: "S3 is the CONTRACT (in parallel). By the time S4 lands, runLoopFastPath returns (commits, chainData, err)
        in the SAME shapes as runLoop. S4 does NOT edit it; S4 only CALLS it. Confirm the sentinel
        ('runLoopFastPath concurrent phase not yet implemented') is GONE before wiring (S3 removes it)."

- docfile: plan/019_2f5621db4d2b/P1M1T1S1/PRP.md
  why: "S1 LANDED isFileDisjoint (:450). Confirms empty-Files is VACUOUSLY disjoint (returns true) — the root
        cause of the §4 regression — and intra-concept duplicate returns false (the §6 single-concept fix)."
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  decompose.go        # THE home: Decompose() (:144) dispatch site (:237) ← EDIT HERE. runLoop(:479)/
                      #   runLoopFastPath(:662)/isFileDisjoint(:450) UNCHANGED (S4 only calls them).
  decompose_test.go   # ≈25 multi-commit tests remediated (Task 3) + 2 dispatch tests (Task 4)
  roles.go            # Deps struct (:55) + stager seam (:80) — UNCHANGED
internal/ui/verbose.go  # VerboseWarn(:106) + NewVerbose(:35) — REUSE, UNCHANGED
internal/prompt/planner.go  # PlannerCommit.Files / PlannerOutput.Commits — UNCHANGED
```

### Desired Codebase tree with files to be added

```bash
internal/decompose/decompose.go        # MODIFY: Decompose() dispatch gate (replaces :237 line) + VerboseWarn log
internal/decompose/decompose_test.go   # MODIFY: ≈25 tests → non-disjoint partitions (stay runLoop) + 2 dispatch tests
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the regression). isFileDisjoint returns true when NO path appears in ≥2 concepts — INCLUDING
//   the vacuous case where every concept has EMPTY Files (S1's literal-count: empty map → true). ~25 existing
//   multi-commit Decompose tests stub the planner with empty `files` + a stager seam. After the gate is wired,
//   they route to runLoopFastPath → Add([]) no-op → every concept FR-M8-skipped → 0 commits → test FAILS.
//   Task 3 remediates by making each affected test's partition NON-disjoint (stay on runLoop). Do NOT skip Task 3.

// CRITICAL (pre-declare, don't re-declare). The current line uses `:=` (short-decl): `commits, chainData, err := runLoop(...)`.
//   An if/else with `=` in both branches needs the names declared first. Pre-declare with a var block, then
//   assign in each branch. (Or wrap the gate in an IIFE returning the 3-tuple with `:=` — but pre-declaring
//   is more idiomatic and keeps the error block below unchanged.)

// CRITICAL (runLoop + runLoopFastPath + isFileDisjoint byte-identical). S4 is a CONSUMER. Do NOT edit those
//   functions. The fast-path is complete after S3; the gate just selects. If the S3 sentinel
//   ('runLoopFastPath concurrent phase not yet implemented') is still present when you start, S3 has not
//   landed — STOP and surface it (the dispatch must not wire to an incomplete fast-path).

// CRITICAL (shared post-loop code). The error block (:238-242: `if err != nil { return DecomposeResult{Commits: commits}, err }`)
//   + arbiter phase (:248+: tipTree/leftoverPaths/runArbiterPhase/rereadFinalCommits) consume
//   commits/chainData — NOT loop internals. Both loop variants produce them in the same shape, so this code
//   runs IDENTICALLY on either branch. Do NOT touch it.

// GOTCHA (log ONCE, at the gate — not per concept). One VerboseWarn naming the chosen path (e.g.
//   "decompose: file-disjoint partition → fast-path (FR-M13)" vs "...shared-file partition → tooled-stager loop").
//   Guard `if deps.Verbose != nil`. The loop functions already log their own per-concept progress; don't duplicate.

// GOTCHA (deps.stager is the dispatch oracle, not the verbose log). On the fast-path deps.stager is UNREACHABLE
//   (system_context §6 — it's only called via runLoop's invokeStagerRetry). So a test injects
//   deps.stager = func(...) error { t.Fatal("fast-path must not invoke the stager"); return nil } and drives
//   Decompose with disjoint files → passing PROVES the fast-path was taken. Stronger than grepping a log.

// GOTCHA (a fake/duplicated file is safe on runLoop — the remediation). Making a test non-disjoint by listing a
//   path ≥2 times in the planner JSON does NOT change runLoop's behavior: the stager seam stages per its OWN map
//   (conceptFiles[concept.Title]), ignoring concept.Files; verifyFreezeSubset checks the REAL staged tree ⊆ T_start;
//   checkPlannerCoverage (:291) is diagnostic-only (never aborts; an extra claimed-but-not-real path is absent
//   from DiffTreeNames → not flagged). So the duplicated entry is never git-add-ed; commits are identical.

// SCOPE: S4 wires the dispatch + remediates existing tests + adds 2 dispatch tests. Do NOT edit runLoop/
//   runLoopFastPath/isFileDisjoint, the error block, the arbiter phase, arbiter.go/chain.go/roles.go/message.go/
//   stager.go, the Git interface, or the signal package. Do NOT add config/flag/docs (S6). Do NOT add the
//   comprehensive fast-path regression suite (S5) — only the 2 dispatch tests + the remediation.
```

## Implementation Blueprint

### Data models and structure

None. No types, no struct changes, no new symbols outside `Decompose()`'s body. S4 edits ONE call site and
calls three existing functions.

### Implementation Tasks (ordered by dependencies)

> **Prerequisites (verified)**: S1 `isFileDisjoint` LANDED (`decompose.go:450`); S2 `runLoopFastPath` sweep
> LANDED (`:662`); S3 `runLoopFastPath` concurrent phase — treat as CONTRACT (in parallel). **Before wiring**,
> confirm the S3 sentinel is GONE: `grep -c 'not yet implemented' internal/decompose/decompose.go` == 0. If it
> is still 1, S3 has not landed — surface it and stop (do not wire to an incomplete fast-path).
> ANCHOR ON CONSTRUCTS via grep, not line numbers (`grep -n 'commits, chainData, err := runLoop' internal/decompose/decompose.go`).

```yaml
Task 1: WIRE THE DISPATCH GATE in Decompose() (internal/decompose/decompose.go, ~:237)
  - EDIT: replace the single line
        commits, chainData, err := runLoop(ctx, deps, out.Commits, baseTree, tStart, preRunHEAD, isUnborn)
    with the pre-declared var block + the gate:
        // (6) The loop. FR-M13: if the planner's partition is pairwise file-disjoint (no path in ≥2 concepts),
        //     stage deterministically with the fast-path (no stager agent, concurrent messages — FR-M14);
        //     otherwise the tooled-stager runLoop (1-deep overlap, hunk-split capable — FR-M5/FR-M6). Both
        //     return (commits, chainData, err) in identical shapes, so the error block + arbiter phase below
        //     run unchanged on either branch.
        var (
            commits   []CommitResult
            chainData []ChainEntry
            err       error
        )
        if isFileDisjoint(out.Commits) {
            if deps.Verbose != nil {
                deps.Verbose.VerboseWarn("decompose: file-disjoint partition → fast-path (FR-M13/FR-M14)")
            }
            commits, chainData, err = runLoopFastPath(ctx, deps, out.Commits, baseTree, tStart, preRunHEAD, isUnborn)
        } else {
            if deps.Verbose != nil {
                deps.Verbose.VerboseWarn("decompose: shared-file partition → tooled-stager loop (FR-M5)")
            }
            commits, chainData, err = runLoop(ctx, deps, out.Commits, baseTree, tStart, preRunHEAD, isUnborn)
        }
    The block immediately following (`if err != nil { return DecomposeResult{Commits: commits}, err }`) and
    everything after it (arbiter phase, rereadFinalCommits, DecomposeResult assembly) STAYS byte-identical.
    Update the old comment `// (6) The loop (1-deep overlap, FR-M8 empty-skip, serialized CAS, FR-M12 isolation).`
    (it now describes only the else branch) — fold it into the new comment or drop it (the new comment covers both).
  - CONFIRM: the 7 args (ctx, deps, out.Commits, baseTree, tStart, preRunHEAD, isUnborn) are IDENTICAL for both
    calls — they match runLoop's + runLoopFastPath's signatures exactly (grep-verified).
  - DEPENDENCIES: S1 + S2 + S3 landed (call isFileDisjoint + runLoopFastPath). go build must resolve both.

Task 2: VERIFY the dispatch compiles + existing runLoop-only tests still pass (pre-remediation baseline)
  - go build ./...                # the gate compiles; runLoopFastPath + isFileDisjoint resolve
  - go test ./internal/decompose/ -run 'TestDecompose_HunkSplit|TestIsFileDisjoint|TestRunLoopFastPath' -race -v
    # These should PASS (HunkSplit is non-disjoint → runLoop; the predicate + fast-path unit tests are dispatch-independent).
  - EXPECTED FAILURES (the regression — Task 3 fixes): ~25 TestDecompose_* multi-commit tests now route to the
    fast-path (empty files) → 0 commits. `go test ./internal/decompose/ -run TestDecompose -count=1` will show
    them failing with "Commits len = 0, want N". This is EXPECTED at this checkpoint; do not stop here.
  - DEPENDENCIES: Task 1.

Task 3: REMEDIATE the ~25 existing multi-commit tests (keep them on runLoop — their original path)
  - GOAL: each affected test's planner partition becomes NON-disjoint so isFileDisjoint returns false → runLoop,
    preserving its original behavior + all existing runLoop coverage. (New fast-path coverage is S5.)
  - RULE (minimal, mechanical, works for single- AND multi-concept tests): ensure at least one path appears
    ≥2 times across the concepts' `files`:
      * MULTI-concept tests: list the SAME file in ≥2 concepts. Cleanest when two concepts already touch one
        file — mirror TestDecompose_HunkSplitAcrossConcepts (:848: both concepts declare "files":["store.py"]).
        Otherwise append a shared sentinel to the first two concepts.
      * SINGLE-concept tests: list one file TWICE within the concept (intra-concept duplicate). S1's literal-count
        algorithm counts it as 2 → returns false (S1's test matrix: "intra-concept duplicate disqualifies").
  - WHY THIS IS SAFE (no behavior change on runLoop): the stager seam stages per its OWN map
    (dcmStagerSeam: conceptFiles[concept.Title]), IGNORING concept.Files; verifyFreezeSubset checks the REAL
    staged tree ⊆ T_start; checkPlannerCoverage (:291) is diagnostic-only. The duplicated/extra entry is never
    git-add-ed; commits are byte-identical to before.
  - AFFECTED TESTS (multi-commit, empty-`files` planner JSON, driven through Decompose — fix each):
      TestDecompose_AutoMultiCommit_HappyPath (:464), _TemplateAppliedUniformly (:540), _Overlap (:580),
      _EmptyConceptSkip (:646), _StagerMovedHEAD (:689), _StagerFreezeViolation (:727),
      _ConcurrentChangeExclusion (:955), _ArbiterFoldsOnlyTStart (:1042), _TStartCompleteness (:1123),
      _StagerGuardHappyPath (:1184), _SafetyCap (:1252), _ArbiterSkippedOnCleanTree (:1292),
      _ArbiterWiring (:1334), _ErrorPropagation_Stager (:1393), _ErrorPropagation_RescueError (:1433),
      _UnbornRepo (:1463), _MessageRescuePartial (:1528), _CASAbortPartial (:1641),
      _StagerRetryThenEmpty (:1757), _RescueArbiterSkipped (:1849), _StagerRetryThenSuccess (:1905),
      _OneFileShortcut_Deletion (:2119), _ArbiterTipAmend_RereadsFinalSHA (:2385),
      _ArbiterMidChain_AllSHAsResolve (:2446), _HappyPath_CommitsAccurate (:2504),
      _RoleResolvesSubProvider (:2539), _PlannerCoverageLogsUnclaimed (:2661).
  - ALREADY NON-DISJOINT (UNAFFECTED — leave alone): TestDecompose_HunkSplitAcrossConcepts (:848) +
    _HunkSplit_RejectsOffTStartContent (:906) (shared store.py → runLoop naturally).
  - CHECK TestDecompose_TokenLimitInvariant_PlannerPromptFits (:2733 — disjoint a.txt/b.txt + stager seam):
    after Task 1 it routes to the fast-path. If it asserts only "commits land", it PASSES (fast-path stages the
    declared files → same commits) — leave it. If it asserts stager invocation, apply the Rule above.
  - WORKED EXAMPLE (TestDecompose_AutoMultiCommit_HappyPath :474 — multi-concept):
      BEFORE: {"count":3,"single":false,"commits":[{"title":"c1","description":"a.txt"},{"title":"c2","description":"b.txt"},{"title":"c3","description":"c.txt"}]}
      AFTER : {"count":3,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt"]},{"title":"c2","description":"b.txt","files":["a.txt","b.txt"]},{"title":"c3","description":"c.txt","files":["c.txt"]}]}
      (a.txt now appears in c1 AND c2 → isFileDisjoint false → runLoop; the stager seam still stages c1→a.txt,
       c2→b.txt, c3→c.txt per its map; commits unchanged.)
  - WORKED EXAMPLE (single-concept, e.g. _HappyPath_CommitsAccurate :2510 or _UnbornRepo :1470):
      BEFORE: {"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt"}]}
      AFTER : {"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt","a.txt"]}]}
      (intra-concept duplicate → isFileDisjoint false → runLoop; unchanged staging.)
  - AFTER: `go test ./internal/decompose/ -run TestDecompose -race -count=1` → ALL green.
  - DEPENDENCIES: Task 1.

Task 4: ADD the 2 DISPATCH tests in internal/decompose/decompose_test.go
  - PLACE: near TestDecompose_HunkSplitAcrossConcepts (:848) / _AutoMultiCommit_HappyPath (:464) (group with the
    multi-commit end-to-end tests). These are the FIRST tests to exercise the gate end-to-end through Decompose
    (S3's TestRunLoopFastPath_* call runLoopFastPath DIRECTLY, pre-dispatch).
  - TEST A — TestDecompose_Dispatch_DisjointFastPath (disjoint → fast-path → stager BYPASSED):
        func TestDecompose_Dispatch_DisjointFastPath(t *testing.T) {
            bin := stubtest.Build(t)
            repo := t.TempDir()
            dcmInitRepo(t, repo)
            // 3 disjoint untracked files (the working-tree change set).
            dcmWriteFile(t, repo, "a.txt", "aaa\n")
            dcmWriteFile(t, repo, "b.txt", "bbb\n")
            dcmWriteFile(t, repo, "c.txt", "ccc\n")
            // Planner declares DISJOINT files → isFileDisjoint true → runLoopFastPath.
            plannerJSON := `{"count":3,"single":false,"commits":[` +
                `{"title":"c1","description":"a","files":["a.txt"]},` +
                `{"title":"c2","description":"b","files":["b.txt"]},` +
                `{"title":"c3","description":"c","files":["c.txt"]}]}`
            plannerM := dcmPlannerManifest(t, bin, plannerJSON)
            messageM := dcmMessageScriptManifest(t, bin, []string{"feat: a", "feat: b", "feat: c"})
            roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
            roles.Planner = plannerM
            roles.Message = messageM
            deps := dcmDeps(t, repo, roles)
            // THE DISPATCH ORACLE: the fast-path must NEVER call the stager (system_context §6). If it does,
            // the run mis-routed to runLoop. t.Fatal makes this a hard correctness gate.
            deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
                t.Fatalf("fast-path must not invoke the stager (concept %q routed to runLoop)", concept.Title)
                return nil
            }
            result, err := Decompose(context.Background(), deps)
            if err != nil { t.Fatalf("Decompose: %v", err) }
            if len(result.Commits) != 3 { t.Fatalf("Commits len = %d, want 3", len(result.Commits)) }
            // CAS order + clean tree (mirror HappyPath :464-538 assertions).
            if dcmLogCount(t, repo) != 3 { t.Fatalf("commit count = %d, want 3", dcmLogCount(t, repo)) }
            if status := dcmStatusPorcelain(t, repo); status != "" { t.Errorf("status = %q, want empty", status) }
            // (Optional) observe the gate log:
            //   var vbuf bytes.Buffer; deps.Verbose = ui.NewVerbose(&vbuf, true) ... ; assert vbuf contains "fast-path".
        }
  - TEST B — TestDecompose_Dispatch_SharedFileFallback (shared → runLoop → stager CALLED):
        func TestDecompose_Dispatch_SharedFileFallback(t *testing.T) {
            bin := stubtest.Build(t)
            repo := t.TempDir()
            dcmInitRepo(t, repo)
            // One file split across 2 concepts (mirror HunkSplit :848-870's store.py setup).
            base := "def a():\n    return 0\n\ndef b():\n    return 0\n"
            tStart := "def a():\n    return 1\n\ndef b():\n    return 2\n"
            dcmWriteFile(t, repo, "store.py", base); dcmStageFile(t, repo, "store.py"); dcmCommitRaw(t, repo, "init")
            dcmWriteFile(t, repo, "store.py", tStart) // dirty, unstaged → triggers decompose
            // Planner declares a SHARED file across both concepts → isFileDisjoint false → runLoop.
            plannerJSON := `{"count":2,"single":false,"commits":[` +
                `{"title":"c1","description":"a","files":["store.py"]},` +
                `{"title":"c2","description":"b","files":["store.py"]}]}`
            plannerM := dcmPlannerManifest(t, bin, plannerJSON)
            messageM := dcmMessageScriptManifest(t, bin, []string{"feat: a", "feat: b"})
            roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
            roles.Planner = plannerM; roles.Message = messageM
            deps := dcmDeps(t, repo, roles)
            deps.Config.Commits = 2 // forced count overrides the FR-M2b one-file short-circuit (mirror HunkSplit)
            // THE DISPATCH ORACLE: runLoop MUST call the stager. A flag proves the fallback was taken.
            stagerCalled := false
            deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
                stagerCalled = true
                // Stage this concept's hunk — mirror HunkSplit :875's per-concept stager (writes the partial
                // tree). Simplest correct stage: for c1 write base+a's-change; for c2 write tStart (full).
                // (Exact hunk-staging mirroring HunkSplit :875; the PRIMARY assertion is stagerCalled + 2 commits.)
                return nil
            }
            result, err := Decompose(context.Background(), deps)
            if err != nil { t.Fatalf("Decompose: %v", err) }
            if !stagerCalled { t.Fatal("shared-file partition must route to runLoop (stager not called)") }
            if len(result.Commits) != 2 { t.Fatalf("Commits len = %d, want 2", len(result.Commits)) }
        }
    NOTE for TEST B's stager body: mirror TestDecompose_HunkSplitAcrossConcepts (:875) exactly — it stages
    concept 0's hunk (basePlusA) then concept 1 appends to reach tStart. Copy that stager closure verbatim;
    the PRIMARY assertions (stagerCalled + 2 commits) are robust to the staging details.
  - DEPENDENCIES: Task 1 (the gate) + Task 3 (suite green). Task 4 can be written before/after Task 3.

Task 5: VERIFY gates + scope boundaries
  - go build ./... && go vet ./... && gofmt -l internal/decompose/   # all clean
  - go test ./internal/decompose/... -race -count=1                   # FULL suite green (incl. the 2 dispatch tests)
  - make test && make lint
  - grep guards (Level 4 below): runLoop/runLoopFastPath/isFileDisjoint byte-identical; only the :237 line changed
    in Decompose; only decompose.go + decompose_test.go touched.
  - DEPENDENCIES: Tasks 1-4.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the dispatch gate (pre-declare + assign in each branch — keeps the error block below unchanged)
var (
    commits   []CommitResult
    chainData []ChainEntry
    err       error
)
if isFileDisjoint(out.Commits) {
    if deps.Verbose != nil {
        deps.Verbose.VerboseWarn("decompose: file-disjoint partition → fast-path (FR-M13/FR-M14)")
    }
    commits, chainData, err = runLoopFastPath(ctx, deps, out.Commits, baseTree, tStart, preRunHEAD, isUnborn)
} else {
    if deps.Verbose != nil {
        deps.Verbose.VerboseWarn("decompose: shared-file partition → tooled-stager loop (FR-M5)")
    }
    commits, chainData, err = runLoop(ctx, deps, out.Commits, baseTree, tStart, preRunHEAD, isUnborn)
}
// ↓↓↓ byte-identical from here ↓↓↓
if err != nil {
    return DecomposeResult{Commits: commits}, err
}
// ... arbiter phase (unchanged) ...

// PATTERN: the dispatch test oracle (stager seam — called on runLoop, FATAL on fast-path)
deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
    t.Fatalf("fast-path must not invoke the stager (concept %q)", concept.Title)
    return nil
}
// (inverse: a bool flag set true inside the seam proves runLoop was taken)

// PATTERN: the remediation (make a planner partition non-disjoint WITHOUT changing runLoop behavior)
//   multi-concept:  {"files":["a.txt"]} on c1 AND c2  → a.txt appears twice → isFileDisjoint false
//   single-concept: {"files":["a.txt","a.txt"]}        → intra-dupe count 2 → isFileDisjoint false
//   (the stager seam stages per conceptFiles[Title], ignoring concept.Files; commits unchanged)
```

### Integration Points

```yaml
NO struct/schema/API/config/build changes. One call site edited + existing tests remediated + 2 tests added.

CODE:
  - internal/decompose/decompose.go — Decompose() dispatch gate (replaces the :237 runLoop call) + 1 VerboseWarn per branch
TESTS:
  - internal/decompose/decompose_test.go — ≈25 tests → non-disjoint partitions (stay runLoop) + 2 dispatch tests

CALLED (REUSE, byte-identical — S1/S2/S3 own them):
  - isFileDisjoint (decompose.go:450)        — the FR-M13 gate (S1)
  - runLoopFastPath (decompose.go:662)        — the deterministic fast-path (S2 sweep + S3 concurrent)
  - runLoop (decompose.go:479)                — the tooled-stager fallback (UNCHANGED)
  - deps.Verbose.VerboseWarn (ui/verbose.go:106) — nil-guarded one-line log

SHARED + UNCHANGED (feeds identically from either branch — do NOT touch):
  - Decompose error block (:238-242), arbiter gate (:248-268: DiffTreeNames(tipTree, T_start)),
    runArbiterPhase, rereadFinalCommits, DecomposeResult assembly

DOWNSTREAM (do NOT implement in S4):
  - P1.M1.T1.S5: comprehensive fast-path regression suite (disjoint/fallback/concurrency/CAS/isolation/skip/freeze)
  - P1.M1.T1.S6: docs/how-it-works.md fast-path paragraph

UNCHANGED: runLoop; runLoopFastPath; isFileDisjoint; the error block + arbiter phase; arbiter.go; chain.go;
  roles.go (Deps + stager seam); message.go; stager.go; the Git interface; the signal package; any config/CLI.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...                      # the gate compiles; runLoopFastPath + isFileDisjoint resolve
go vet ./...
gofmt -l internal/decompose/        # Expected: empty.
make lint                           # Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The 2 new dispatch tests (the FIRST end-to-end exercise of the gate)
go test ./internal/decompose/ -run 'TestDecompose_Dispatch_DisjointFastPath|TestDecompose_Dispatch_SharedFileFallback' -race -v
# Expected: PASS — disjoint→fast-path (stager t.Fatal never tripped); shared→runLoop (stager flag set).

# The remediated suite (≈25 tests now non-disjoint → runLoop, green again)
go test ./internal/decompose/ -run TestDecompose -race -count=1
# Expected: ALL pass. (Before Task 3, ~25 of these fail with "Commits len = 0, want N" — that IS the regression.)

# S1/S2/S3 dispatch-independent tests (must still pass)
go test ./internal/decompose/ -run 'TestIsFileDisjoint|TestRunLoopFastPath|TestDecompose_HunkSplit' -race -v
# Expected: PASS.

# Full decompose package (race)
go test ./internal/decompose/... -race -v

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# End-to-end through the binary (the FR-M13 user-visible behavior). Requires a configured provider (or a stub).
# Disjoint tree → fast-path (no stager agent) → N concurrent messages → N commits.
#   echo "unrelated changes across 3 files" → stagecoach → 3 commits, --verbose shows "fast-path".
# Shared-file tree → runLoop (tooled stager) → N commits, --verbose shows "tooled-stager loop".
# (The within-scope proof is the Level 2 unit tests; this is the optional manual smoke.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the dispatch gate exists + calls both loop variants
grep -n 'isFileDisjoint(out.Commits)\|runLoopFastPath(ctx, deps\|runLoop(ctx, deps' internal/decompose/decompose.go
# Expected: the gate's isFileDisjoint call + a runLoopFastPath call + a runLoop call inside Decompose (plus the
#   unchanged func definitions + runLoop's other internal uses). Confirm the gate has BOTH branches.

# Grep guard: runLoop + runLoopFastPath + isFileDisjoint byte-identical (S4 only CALLS them)
git diff internal/decompose/decompose.go | grep -E '^-' | grep -v '^---'
# Expected: ONLY the old :237 line + its comment are removed. NO - lines inside runLoop/runLoopFastPath/
#   isFileDisjoint/the error block/the arbiter phase.

# Grep guard: the decision is logged exactly once per branch (VerboseWarn)
grep -n 'VerboseWarn' internal/decompose/decompose.go | grep -i 'fast-path\|tooled-stager\|disjoint'
# Expected: 2 matches inside Decompose (one per branch), nil-guarded.

# Grep guard: S3 sentinel is GONE (the fast-path is complete — S4 wires a complete fast-path)
grep -c 'runLoopFastPath concurrent phase not yet implemented' internal/decompose/decompose.go
# Expected: 0 (S3 removed it). If 1 → S3 not landed; STOP (do not wire an incomplete fast-path).

# Grep guard: no new config key / CLI flag / env var added (FR-M13: "adds no configuration")
git diff --all internal/ | grep -E '^\+.*stagecoach\.(fast|disjoint)|FastPath|Disjoint' | grep -iv 'test\|verbose\|comment\|//'
# Expected: empty (no new config/flag — the gate is purely internal dispatch).

# Scope-boundary guard: only decompose.go + decompose_test.go changed
git diff --name-only
# Expected: only internal/decompose/decompose.go + internal/decompose/decompose_test.go.

# Dispatch-oracle guard: TEST A's stager t.Fatals (fast-path bypass proof)
grep -n 'fast-path must not invoke the stager' internal/decompose/decompose_test.go
# Expected: 1 match (inside TestDecompose_Dispatch_DisjointFastPath).

# Race guard: the dispatch tests ran under -race (re-run explicitly — the fast-path is concurrent)
go test ./internal/decompose/ -run 'TestDecompose_Dispatch' -race -count=1 -v
# Expected: PASS, zero race reports.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/decompose/` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass
- [ ] Dispatch tests pass under `-race` with ZERO race reports

### Feature Validation
- [ ] `Decompose()` dispatches `isFileDisjoint(out.Commits)` true → `runLoopFastPath`, false → `runLoop`
- [ ] Decision logged once per branch via `deps.Verbose.VerboseWarn(...)` (nil-guarded)
- [ ] TEST A: disjoint files → fast-path → 3 commits + the stager seam is NEVER called (t.Fatal guard)
- [ ] TEST B: shared file → runLoop → 2 commits + the stager seam IS called (flag set)
- [ ] The arbiter runs identically after either path (no test change needed — it consumes commits/chainData)

### Regression-Remediation Validation
- [ ] ≈25 existing multi-commit tests remediated to non-disjoint partitions (stay on runLoop)
- [ ] `go test ./internal/decompose/ -run TestDecompose -race -count=1` → ALL green (no "Commits len = 0" failures)
- [ ] The 2 HunkSplit tests (already non-disjoint) UNAFFECTED
- [ ] `TestDecompose_TokenLimitInvariant_PlannerPromptFits` checked (passes on fast-path OR remediated)

### Scope-Boundary Validation
- [ ] `runLoop` BYTE-IDENTICAL (no `-` lines inside it)
- [ ] `runLoopFastPath` BYTE-IDENTICAL (S2/S3 own it; S4 only calls it)
- [ ] `isFileDisjoint` BYTE-IDENTICAL (S1 owns it; S4 only calls it)
- [ ] The error block (:238-242) + arbiter phase (:248+) + `rereadFinalCommits` + `DecomposeResult` assembly UNCHANGED
- [ ] NO new config key / CLI flag / env var (FR-M13)
- [ ] NO docs (S6)
- [ ] NO comprehensive regression suite (S5) — only the 2 dispatch tests + the remediation
- [ ] arbiter.go / chain.go / roles.go / message.go / stager.go / Git interface / signal package UNCHANGED

### Code Quality
- [ ] The gate is a pure routing decision (no work of its own)
- [ ] One `VerboseWarn` per branch, nil-guarded, naming the chosen path + FR citation
- [ ] The new comment documents BOTH branches + that the post-loop code is shared
- [ ] Remediation uses the minimal non-disjoint form (shared file / intra-dupe) — no test scenario changed

---

## Anti-Patterns to Avoid

- ❌ Don't wire the gate while the S3 sentinel is still present (`grep -c 'not yet implemented'` == 1). The
  dispatch must wire to a COMPLETE fast-path. If S3 hasn't landed, surface it and stop.
- ❌ Don't re-declare `commits/chainData/err` with `:=` in both branches — the error block below uses those
  names. Pre-declare with a `var (...)` block, then `=` in each branch (keeps the error block byte-identical).
- ❌ Don't edit `runLoop` / `runLoopFastPath` / `isFileDisjoint` — S4 is a CONSUMER. They are byte-identical
  (S1/S2/S3 own them; existing tests + S3's tests pin them).
- ❌ Don't touch the error block (:238-242) or the arbiter phase — they consume `commits`/`chainData`, not loop
  internals, so they run identically on either branch. Editing them is out of scope and risks the shared path.
- ❌ Don't skip Task 3 (the remediation). Wiring the gate makes ≈25 empty-`files` planner stubs route to the
  fast-path (stages nothing → 0 commits). CI WILL be red until they're made non-disjoint. This is the single
  most important thing to get right in S4 — see research/findings.md §4.
- ❌ Don't "fix" the regression by making `isFileDisjoint` return false for empty Files — S1 is LANDED and the
  spec (FR-M13) is clear (the gate is a pure set-membership test). Fix the TESTS (make partitions non-disjoint),
  not the gate.
- ❌ Don't flip the ≈25 tests to the fast-path (by declaring disjoint real files) "to also cover the fast-path" —
  that GUTS runLoop's top-level coverage (only the 2 HunkSplit tests would remain). Keep them on runLoop (non-
  disjoint); S5 owns the comprehensive fast-path + fallback suite.
- ❌ Don't assert the dispatch via a verbose-log grep as the PRIMARY signal — `deps.Verbose` is nil in most
  tests (`dcmDeps`). The stager seam (t.Fatal on fast-path / flag on runLoop) is a stronger, config-independent
  oracle. (A verbose-log assertion is a fine OPTIONAL secondary check with `ui.NewVerbose(&buf, true)`.)
- ❌ Don't log per-concept inside the gate — one `VerboseWarn` per branch naming the path. The loop functions
  already log their own progress.
- ❌ Don't add a config key / CLI flag / env var to force one path — FR-M13 is explicit: "adds no configuration".
  The gate is automatic and deterministic.
- ❌ Don't add docs (how-it-works.md) — that's S6.
- ❌ Don't add the comprehensive fast-path regression suite — that's S5. S4 adds only the 2 dispatch tests
  (proving the gate routes correctly end-to-end) + the remediation that keeps CI green.

---

## Confidence Score: 9/10

One-pass success is very high. The core edit is a pure routing decision at a single, exactly-located line
(:237), with every variable in scope named + typed, both loop-variant signatures confirmed identical (grep-
verified), the `VerboseWarn` logging method confirmed, and the post-loop code proven path-agnostic (it
consumes commits/chainData, not loop internals). The 2 dispatch tests have copyable bodies reusing the
verified harness idiom + the stager-seam oracle. The -1 is entirely for the **regression-remediation task
(Task 3)**: wiring the gate breaks ≈25 existing tests (empty-`files` planner stubs route to the fast-path →
0 commits), and while the remediation RULE is minimal and mechanical (make each partition non-disjoint via a
shared file or intra-concept duplicate — proven safe on runLoop because the stager seam ignores concept.Files),
it requires touching ~25 test files' planner JSONs without changing their scenarios, which is where an
inattentive implementer could slip (e.g., accidentally making a stager-specific test disjoint so it flips to
the fast-path and loses its tested behavior). Mitigated by: (a) the worked examples (multi-concept + single-
concept), (b) the explicit enumerated list of affected tests, (c) the clear principle (keep ALL existing
tests on runLoop; S5 owns fast-path coverage), (d) the grep guards (the full `TestDecompose` suite must be
green post-remediation), and (e) the stager-seam oracle that makes the dispatch tests config-independent. The
dispatch gate itself is the trivial part; the remediation is the load-bearing work, and it is fully specified.