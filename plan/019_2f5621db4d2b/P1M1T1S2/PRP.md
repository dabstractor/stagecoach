name: "P1.M1.T1.S2 — runLoopFastPath serial deterministic staging sweep + tree freeze + FR-M1c + FR-M8 empty-skip"
description: >
  Add `runLoopFastPath` as a SIBLING to runLoop in internal/decompose/decompose.go: a strictly-serial
  deterministic staging sweep that, for each pairwise-file-disjoint concept, runs `deps.Git.Add(ctx,
  concept.Files)` → `freezeSnapshot` → `verifyFreezeSubset` (FR-M1c on every tree) → FR-M8 empty-skip,
  collecting a `[]stagedConcept` slice of frozen trees (the FR-M14 precondition: every tree[i] frozen
  before any message starts). S2 implements the SERIAL STAGING PHASE only; the concurrent message
  generation + CAS-ordered publish (FR-M14/FR-M7/FR-M12) is S3, marked by a sentinel return. runLoop
  stays byte-identical. Consumes S1's isFileDisjoint gate (wired by S4). No stager agent on this path.

---

## Goal

**Feature Goal**: Implement the deterministic, serial staging sweep of the FR-M13 file-disjoint
fast-path — the phase that freezes every concept's tree up front via plain `git add` (no stager agent),
under the unchanged accumulate-never-reset index model — producing the `[]stagedConcept` slice that is
the FR-M14 precondition for S3's concurrent message generation.

**Deliverable**: (1) `type stagedConcept struct { idx int; tree, prevTree string }` in decompose.go;
(2) `func runLoopFastPath(ctx, deps Deps, concepts []prompt.PlannerCommit, baseTree, tStart, preRunHEAD string, isUnborn bool) ([]CommitResult, []ChainEntry, error)` as a sibling to runLoop, implementing the serial sweep + a sentinel return marking the S3 boundary; (3) a focused sweep-only test in decompose_test.go (disjoint concepts → sentinel error + staged count). runLoop UNCHANGED.

**Success Definition**:
- runLoopFastPath runs the serial sweep: per concept, `deps.Git.Add(ctx, concept.Files)` → `freezeSnapshot` → `verifyFreezeSubset` → FR-M8 skip-or-append, accumulating `[]stagedConcept`.
- The sweep is STRICTLY SERIAL (plain `for` loop; no goroutines) — gitRunner has no index lock and Add/WriteTree mutate `.git/index`.
- FR-M1c (`verifyFreezeSubset`) runs on EVERY frozen tree (whole-path adds of T_start content pass it trivially, but it MUST still run).
- FR-M8 empty-skip: `treeI == prevTree` ⇒ skip (log, prevTree unchanged, continue). A concept with empty Files hits this (Add is a no-op).
- After the sweep, the function returns a sentinel error naming the S3 boundary + the staged count (so S4 dispatch is not wired to an incomplete fast-path).
- runLoop is byte-identical; existing TestDecompose_* tests pass unchanged.
- `go build ./...`, `go test ./internal/decompose/...`, `make test`, `make lint` pass.

## User Persona (if applicable)

**Target User**: Stagecoach maintainers (internal function; no user-facing surface). The fast-path collapses a disjoint N-concept run's critical path and lets a provider with no `tooled_flags` (opencode, qwen-code) decompose a disjoint tree.

**Use Case**: A user with a dirty tree of unrelated whole-file changes runs `stagecoach`; the planner partitions disjointly; the fast-path stages deterministically (no stager agent) and (after S3) generates all messages concurrently.

**Pain Points Addressed**: FR-M13/FR-M14 — the tooled-stager serial bottleneck and the opencode/qwen-code inability to serve the stager role on disjoint trees.

## Why

- **FR-M13 (P0)**: When the planner's partition is pairwise file-disjoint, stagecoach stages each concept deterministically with `git add` — no stager agent. This subtask is the staging sweep itself. FR-M1c/FR-M7/FR-M8 guarantees are identical to the tooled path (verifyFreezeSubset runs on every tree).
- **FR-M14**: The deterministic sweep freezes every `tree[i]` BEFORE any message agent starts — the precondition that lifts FR-M6's 1-deep overlap to full message concurrency (S3). S2 produces the frozen `[]stagedConcept`; S3 consumes it.
- **Why a sibling, not a modification**: runLoop is the shared-file (hunk-split) fallback and must stay byte-identical. The fast-path is greenfield; S4 dispatches between them via S1's `isFileDisjoint` gate.

## What

**User-visible behavior**: None yet (S2 is internal; S4 wires the dispatch). Once the full fast-path lands (S3+S4), a disjoint decomposition stages deterministically and generates messages concurrently.

**Technical change (one struct + one function + one focused test):**
1. `stagedConcept` struct.
2. `runLoopFastPath` — serial sweep mirroring runLoop's tStartPaths/prevTree setup + per-concept Add/freeze/verify/skip, collecting `[]stagedConcept`, ending in a sentinel return for S3.
3. A focused test proving the sweep runs (sentinel error + staged count).

### Success Criteria
- [ ] `stagedConcept` struct defined
- [ ] `runLoopFastPath` signature matches runLoop's args; returns `([]CommitResult, []ChainEntry, error)`
- [ ] Sweep: per concept `deps.Git.Add(ctx, concept.Files)` → `freezeSnapshot` → `verifyFreezeSubset` → FR-M8 skip-or-append
- [ ] Sweep is strictly serial (plain `for`); no goroutines, no `inflight`/`launch`/`publish`/`drainMsg`
- [ ] FR-M1c verifyFreezeSubset runs on every frozen tree
- [ ] FR-M8 empty-skip (`treeI == prevTree`) logs + continues with prevTree unchanged
- [ ] Sentinel return marks the S3 boundary + carries the staged count
- [ ] runLoop byte-identical; existing tests pass
- [ ] `go build ./...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the runLoop reference (mirrored step-by-step), every helper signature (verified line numbers), the critical serial-concurrency constraint, the Add nil-safe/deletion semantics, the Verbose API, the stagedConcept struct + S3 handoff, the sentinel-return rationale, the test harness, and scope fences are all enumerated below.

### Documentation & References

```yaml
- file: internal/decompose/decompose.go
  why: "THE home + the reference. runLoop is the function to MIRROR (prevTree:=baseTree; tStartPaths via
        DiffTreeNames once; per-concept freeze/verify/skip structure). runLoopFastPath is a NEW sibling —
        do NOT modify runLoop. ErrDecomposeFailed is the wrap sentinel runLoop uses. NOTE: the item cited
        runLoop at :456; in the live tree it is at ~:479 (drift) — ANCHOR ON CONSTRUCTS via grep
        `grep -n 'func runLoop(' internal/decompose/decompose.go`, not the line number."
  pattern: "Mirror runLoop's setup verbatim (prevTree:=baseTree; tStartPaths via DiffTreeNames once),
            then the per-concept loop — but replace invokeStagerRetry with deps.Git.Add, and replace
            publish/inflight/launch with append-to-stagedConcept."
  critical: "runLoop MUST stay byte-identical (shared-file fallback). The sweep has NO inflight message
             channel → the verifyFreezeSubset/Add/freeze error paths return (commits, nil, err) with NO
             drainMsg (drainMsg is for in-flight message channels, which S2 does not create)."

- file: internal/decompose/stager.go
  why: "freezeSnapshot (line 141) + verifyFreezeSubset (line 168) — the helpers to REUSE verbatim."
  pattern: "freezeSnapshot(ctx, deps) (string, error); verifyFreezeSubset(ctx, deps, baseTree, tStart,
            tStartPaths, i, conceptTitle, treeI) error — call with the SAME args runLoop uses."

- file: internal/git/git.go
  why: "Git.Add (impl :1339, iface ~:158) — call deps.Git.Add(ctx, concept.Files) directly, NO wrapper.
        DiffTreeNames (impl :1847) — the FR-M1c baseline."
  pattern: "Add(ctx, paths []string) error — nil-safe on empty (len==0 ⇒ return nil); stages adds/mods/
            deletions via `git add -- <paths>`. A concept with empty Files ⇒ Add no-op ⇒ treeI==prevTree
            ⇒ FR-M8 skip."

- docfile: plan/019_2f5621db4d2b/architecture/system_context.md
  why: "The fast-path design. §2 (runLoop reference), §3 (helpers to REUSE), §5 (arbiter unchanged),
        §6 (stager test seam unreachable on fast-path). Confirms greenfield + the dispatch insertion point."
  section: "2. runLoop() — the reference implementation; 3. Helper primitives to REUSE"

- docfile: plan/019_2f5621db4d2b/architecture/git_primitives.md
  why: "The CRITICAL concurrency finding: NO in-process index lock ⇒ Add/WriteTree mutate .git/index ⇒
        the staging sweep MUST run strictly serially (a plain for loop satisfies it). Confirms Add is the
        second call site (arbiter is the first) — call it directly, no wrapper."
  section: "⚠️ CRITICAL CONCURRENCY FINDING"

- docfile: plan/019_2f5621db4d2b/P1M1T1S1/PRP.md
  why: "S1 is the CONTRACT — it adds isFileDisjoint (the FR-M13 gate S4 wires). S2 does NOT touch it.
        Confirms the fast-path is greenfield and runLoop stays byte-identical."

- docfile: plan/019_2f5621db4d2b/P1M1T1S2/research/findings.md
  why: "Verified helper signatures, the serial constraint, Add semantics, the Verbose API, the
        stagedConcept struct + S3 handoff, the sentinel return, scope fences."
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  decompose.go      # THE home: +stagedConcept +runLoopFastPath (sibling to runLoop; grep `func runLoop(` — live ~:479). runLoop UNCHANGED. Decompose=S4 dispatch site (UNTOUCHED)
  decompose_test.go # +TestRunLoopFastPath_Sweep (focused sweep-only; comprehensive suite is S5)
  stager.go         # freezeSnapshot(141) + verifyFreezeSubset(168) — REUSE, UNCHANGED
internal/git/git.go # Git.Add(1339) + DiffTreeNames(1847) — REUSE, UNCHANGED
internal/prompt/planner.go # PlannerCommit.Files []string — the per-concept paths (UNCHANGED)
```

### Desired Codebase tree with files to be added

```bash
internal/decompose/decompose.go      # MODIFY (additive): +stagedConcept +runLoopFastPath (sibling to runLoop)
internal/decompose/decompose_test.go # MODIFY (additive): +TestRunLoopFastPath_Sweep (focused)
```

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (strictly serial sweep): gitRunner has NO in-process index lock (git_primitives.md). Add +
//   WriteTree(live) MUTATE .git/index. The staging sweep MUST be a plain serial `for` loop — NO goroutines,
//   NO errgroup. (S3's concurrent MESSAGE goroutines are confined to read-only tree reads — never Add/
//   WriteTree.) A serial for loop satisfies the constraint by construction.

// CRITICAL (runLoop byte-identical): runLoopFastPath is a NEW sibling function. Do NOT edit runLoop — it
//   is the shared-file (hunk-split) fallback and existing tests pin it. Add the new function + struct
//   alongside it.

// CRITICAL (no drainMsg in the sweep): the sweep creates NO in-flight message channel (message gen is S3).
//   So the Add/freeze/verify error paths return (commits, nil, err) with NO drainMsg call. drainMsg
//   (decompose.go:617) is for in-flight message channels only. (S3 will generalize it to a slice.)

// CRITICAL (index already reset): Decompose step 3 (FreezeWorkingTree, decompose.go:189) already reset
//   the index to baseTree. So `prevTree := baseTree` is the correct baseline — do NOT re-ReadTree/reset.

// GOTCHA (Add semantics): deps.Git.Add(ctx, concept.Files) — nil-safe on empty (len==0 ⇒ no-op ⇒ return
//   nil) AND stages deletions (`git add -- <paths>` records deletions to the index). A concept with empty
//   Files ⇒ Add no-op ⇒ treeI==prevTree ⇒ FR-M8 skip. Call it directly — NO wrapper (system_context confirms).

// GOTCHA (FR-M1c runs on EVERY tree): verifyFreezeSubset MUST run on every frozen treeI, even though
//   whole-path adds of T_start content pass it trivially. Do NOT skip it "because it always passes" — it
//   is the defense-in-depth content guarantee (FR-M1c), and a concurrent working-tree change picked up by
//   a path glob would be caught here.

// GOTCHA (FR-M8 empty-skip keeps prevTree): when treeI == prevTree, `continue` WITHOUT updating prevTree.
//   The skipped concept stages nothing; the next concept stages on top of the same prevTree. Mirror runLoop's
//   `skipped := treeI == prevTree` (runLoop leaves prevTree unchanged on skip).

// GOTCHA (Verbose API): there is NO generic Verbose(msg) info logger. Use deps.Verbose.VerboseWarn(msg)
//   (the free-form logger) for sweep progress (FR-M8 skip, staged-count summary). Guard with
//   `if deps.Verbose != nil` — some tests pass Verbose: nil. The summary log + the staged count in the
//   sentinel error read `staged` (no unused-local lint).

// GOTCHA (sentinel return is intentional): S2 implements the sweep ONLY. The concurrent phase is S3. The
//   sentinel error (`runLoopFastPath concurrent phase not yet implemented (P1.M1.T1.S3)`) is deliberate —
//   it keeps the function honest and prevents S4 from wiring the dispatch to an incomplete fast-path. S4
//   lands AFTER S3, so the dispatch won't see the sentinel.

// SCOPE: S2 is the struct + the sweep function + a focused test. Do NOT add the concurrent/publish phase
//   (S3), do NOT wire Decompose dispatch (S4), do NOT add the comprehensive suite (S5), do NOT touch
//   isFileDisjoint (S1), invokeStager/invokeStagerRetry, or the Git interface.
```

## Implementation Blueprint

### Data models and structure

One new unexported struct. No changes to existing types. The struct is the S3 handoff contract.

```go
// stagedConcept is one concept's frozen tree from the FR-M13 fast-path serial staging sweep, handed to
// the concurrent message phase (P1.M1.T1.S3). idx is the original concept index (for ordering / messages);
// tree is the frozen write-tree SHA; prevTree is the tree it was staged on top of (tree[i-1], or baseTree
// for the first non-skipped concept) — the (prevTree, tree) pair is the message agent's tree-to-tree diff.
type stagedConcept struct {
	idx      int
	tree     string
	prevTree string
}
```

### Implementation Tasks (ordered by dependencies)

> **Prerequisite — S1 has LANDED** (confirmed: `grep -c 'func isFileDisjoint' internal/decompose/decompose.go`
> == 1). S2 does NOT call isFileDisjoint (S4 wires the dispatch), so S2 is independently compilable
> whether or not S1 had landed — but co-locating the fast-path helpers alongside the now-present gate is
> clean. NOTE: the item cited runLoop at :456; the live tree has it at ~:479 (drift) — anchor on constructs.

```yaml
Task 1: ADD stagedConcept + runLoopFastPath in internal/decompose/decompose.go
  - PLACE: immediately AFTER runLoop (sibling — co-locate the fast-path with the loop it alternatives).
  - BODY:
        // stagedConcept — see Data models above (FR-M13 fast-path sweep → S3 concurrent phase handoff).
        type stagedConcept struct {
            idx      int
            tree     string
            prevTree string
        }

        // runLoopFastPath is the FR-M13 file-disjoint fast-path: a STRICTLY SERIAL deterministic staging
        // sweep (no stager agent) that freezes every concept's tree up front via plain `git add`, under
        // the unchanged accumulate-never-reset index model. It is the sibling of runLoop (the shared-file
        // fallback); S4's dispatch selects it when isFileDisjoint(concepts) is true.
        //
        // SERIAL IS LOAD-BEARING: gitRunner has NO in-process index lock — Add/WriteTree mutate .git/index,
        // so the sweep cannot be concurrent. (S3's concurrent MESSAGE goroutines are confined to read-only
        // tree reads.) The index is already reset to baseTree by Decompose's FreezeWorkingTree step.
        //
        // P1.M1.T1.S2 implements the STAGING SWEEP ONLY (FR-M1c on every tree + FR-M8 empty-skip), producing
        // []stagedConcept — the FR-M14 precondition (every tree[i] frozen before any message starts). The
        // concurrent message generation + CAS-ordered publish (FR-M14/FR-M7/FR-M12) is P1.M1.T1.S3, marked
        // by the sentinel return below.
        func runLoopFastPath(ctx context.Context, deps Deps, concepts []prompt.PlannerCommit, baseTree, tStart, preRunHEAD string, isUnborn bool) ([]CommitResult, []ChainEntry, error) {
            var commits []CommitResult
            var chainData []ChainEntry
            prevTree := baseTree

            // FR-M1c: T_start's changed-path set (invariant across the run) — the subset baseline every
            // tree[i] is verified against. Computed ONCE (mirrors runLoop).
            tStartPaths, err := deps.Git.DiffTreeNames(ctx, baseTree, tStart)
            if err != nil {
                return nil, nil, fmt.Errorf("%w: freeze baseline diff-tree-names: %w", ErrDecomposeFailed, err)
            }

            // FR-M13/FR-M14 SERIAL staging sweep: deterministic per-concept git add (no stager agent).
            var staged []stagedConcept
            for i, concept := range concepts {
                if cerr := ctx.Err(); cerr != nil {
                    return commits, nil, cerr // cancelled — partial (nothing published in the sweep)
                }
                // Deterministic whole-path staging (adds + mods + deletions). Nil-safe on empty Files.
                if err := deps.Git.Add(ctx, concept.Files); err != nil {
                    return commits, nil, fmt.Errorf("%w: fast-path add[%d]: %w", ErrDecomposeFailed, i, err)
                }
                treeI, err := freezeSnapshot(ctx, deps)
                if err != nil {
                    return commits, nil, fmt.Errorf("%w: freeze snapshot[%d]: %w", ErrDecomposeFailed, i, err)
                }
                // FR-M1c: verify EVERY tree is a content-subset of T_start. Whole-path adds of T_start
                // content pass trivially, but it MUST run (defense-in-depth; catches a concurrent
                // working-tree change swept in by a path glob). NON-RESCUE — no in-flight message to drain.
                if vErr := verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, i, concept.Title, treeI); vErr != nil {
                    return commits, nil, vErr
                }
                // FR-M8 empty-skip: concept staged nothing new (incl. empty Files ⇒ Add no-op). Skip the
                // commit, keep prevTree unchanged, continue. (No message, no publish — those are S3.)
                if treeI == prevTree {
                    if deps.Verbose != nil {
                        deps.Verbose.VerboseWarn(fmt.Sprintf("fast-path: concept %d %q staged nothing new (FR-M8 skip)", i, concept.Title))
                    }
                    continue
                }
                staged = append(staged, stagedConcept{idx: i, tree: treeI, prevTree: prevTree})
                prevTree = treeI
            }
            // After the sweep: every non-skipped tree[i] is FROZEN — the FR-M14 precondition for S3's
            // concurrent message generation (each message reasons over diff(sc.prevTree, sc.tree)).

            // --- P1.M1.T1.S3 inserts the concurrent message generation + CAS-ordered publish here ---
            // S2 delivers the frozen []stagedConcept slice; S3 consumes it. Until S3 lands, return a
            // sentinel so the Decompose dispatch (S4, which lands AFTER S3) is not wired to an incomplete
            // fast-path. commits/chainData stay nil (the sweep publishes nothing).
            stagedCount := len(staged)
            if deps.Verbose != nil {
                deps.Verbose.VerboseWarn(fmt.Sprintf("fast-path: staged %d/%d concepts (concurrent phase pending P1.M1.T1.S3)", stagedCount, len(concepts)))
            }
            return nil, nil, fmt.Errorf("%w: runLoopFastPath concurrent phase not yet implemented (staged %d concepts; P1.M1.T1.S3)", ErrDecomposeFailed, stagedCount)
        }
  - NOTE: `preRunHEAD` and `isUnborn` are unused in the sweep (they're for S3's publish). Go flags unused
    FUNCTION PARAMS as no error (only unused LOCALS error) — but to be tidy and silence any linter, you may
    reference them in a comment or `_ = prePreRunHEAD`. Confirm `go build`/`make lint` are clean; if a
    linter complains, add `_ = preRunHEAD; _ = isUnborn` before the sentinel (S3 will use them). Prefer the
    comment-only form first; only add the discards if lint requires it.
  - DEPENDENCIES: none (S2 is independently compilable; S1's gate is wired by S4, not called here).

Task 2: ADD TestRunLoopFastPath_Sweep in internal/decompose/decompose_test.go
  - PLACE: near the other TestDecompose_* tests (group with the loop-level tests).
  - FOCUSED sweep-only test (the comprehensive suite is S5). Mirror the existing harness: real git repo
    (t.TempDir + git init + identity), Deps with Git: git.New(repo), disjoint concepts, then call
    runLoopFastPath directly. Assert the sentinel error + the staged count.
  - BODY (adapt the existing setup idiom — confirm the exact setup helpers via the existing tests):
        func TestRunLoopFastPath_Sweep(t *testing.T) {
            repo := t.TempDir()
            // ... git init -q + user.name/email (mirror existing TestDecompose_* setup) ...
            g := git.New(repo)
            ctx := context.Background()

            // Seed a base commit, then modify 3 disjoint files (the working-tree change set).
            // ... write a.go/b.go/c.go, commit; then modify all three ...

            // baseTree = HEAD's tree; tStart = freeze the working change set against baseTree.
            // (Mirror what Decompose does internally: FreezeWorkingTree resets the index to baseTree and
            // returns the T_start tree SHA.)
            baseTree := /* HEAD tree SHA via g */
            tStart, err := g.FreezeWorkingTree(ctx, baseTree)
            if err != nil { t.Fatalf("FreezeWorkingTree: %v", err) }

            deps := Deps{Git: g, Verbose: /* NewVerbose(buf, true) to observe sweep logs */, /* ... */}

            concepts := []prompt.PlannerCommit{
                {Title: "a", Files: []string{"a.go"}},
                {Title: "b", Files: []string{"b.go"}},
                {Title: "c", Files: []string{"c.go"}},
            }

            _, _, err = runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)
            // S2's sweep returns the S3 sentinel — assert it (proves the sweep ran end-to-end).
            if err == nil {
                t.Fatalf("runLoopFastPath: expected the S3 sentinel error, got nil")
            }
            if !strings.Contains(err.Error(), "runLoopFastPath concurrent phase not yet implemented") {
                t.Fatalf("expected the S3 sentinel error; got: %v", err)
            }
            // All 3 disjoint concepts staged (none empty-skipped): the count appears in the sentinel.
            if !strings.Contains(err.Error(), "staged 3 concepts") {
                t.Errorf("expected 'staged 3 concepts' in sentinel; got: %v", err)
            }
        }
  - NOTE: the exact baseTree/tStart setup mirrors Decompose's internal steps; consult an existing
    TestDecompose_* test for the repo-init + FreezeWorkingTree idiom. If FreezeWorkingTree is awkward to
    drive directly, the test may instead assert the sentinel + the Verbose skip/summary logs for a
    simpler signal. The PRIMARY assertion is "the sweep ran (sentinel + count) without a freeze/verify error".
  - DEPENDENCIES: Task 1.

Task 3: VERIFY runLoop byte-identical + gates
  - go build ./...
  - go vet ./...
  - gofmt -l internal/decompose/
  - go test ./internal/decompose/ -run 'RunLoopFastPath|Decompose' -v   # new sweep test + existing runLoop tests
  - go test ./internal/decompose/... -v
  - make test && make lint
```

### Implementation Patterns & Key Details

```go
// PATTERN: the serial sweep loop (mirrors runLoop's setup; replaces stager/publish with Add/append)
prevTree := baseTree
tStartPaths, err := deps.Git.DiffTreeNames(ctx, baseTree, tStart)   // FR-M1c baseline, once
// ... err check ...
var staged []stagedConcept
for i, concept := range concepts {
    if cerr := ctx.Err(); cerr != nil { return commits, nil, cerr }
    if err := deps.Git.Add(ctx, concept.Files); err != nil { return commits, nil, /* wrap */ }
    treeI, err := freezeSnapshot(ctx, deps)
    if err != nil { return commits, nil, /* wrap */ }
    if vErr := verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, i, concept.Title, treeI); vErr != nil {
        return commits, nil, vErr   // NON-RESCUE — no in-flight message to drain
    }
    if treeI == prevTree {           // FR-M8 empty-skip (incl. empty Files)
        /* VerboseWarn skip */ ; continue   // prevTree UNCHANGED
    }
    staged = append(staged, stagedConcept{idx: i, tree: treeI, prevTree: prevTree})
    prevTree = treeI
}
// sentinel return for S3 (reads len(staged) ⇒ staged is "used")

// PATTERN: error wrapping (mirror runLoop's ErrDecomposeFailed idiom)
fmt.Errorf("%w: fast-path add[%d]: %w", ErrDecomposeFailed, i, err)
fmt.Errorf("%w: freeze snapshot[%d]: %w", ErrDecomposeFailed, i, err)
```

### Integration Points

```yaml
NO struct/schema/API/config changes. One new struct + one new function + one focused test.

CODE:
  - internal/decompose/decompose.go — +stagedConcept +runLoopFastPath (sibling to runLoop)
TESTS:
  - internal/decompose/decompose_test.go — +TestRunLoopFastPath_Sweep (focused; comprehensive suite is S5)

CONSUMED (REUSE, unchanged):
  - deps.Git.Add / DiffTreeNames (internal/git/git.go)
  - freezeSnapshot / verifyFreezeSubset (internal/decompose/stager.go)
  - ErrDecomposeFailed (internal/decompose/decompose.go)

DOWNSTREAM (do NOT implement in S2):
  - P1.M1.T1.S3: concurrent message generation + CAS-ordered publish + FR-M12 isolation (consumes []stagedConcept; generalizes drainMsg to a slice; removes the sentinel)
  - P1.M1.T1.S4: Decompose() dispatch — `if isFileDisjoint(out.Commits) { runLoopFastPath(...) } else { runLoop(...) }` (S1's gate)
  - P1.M1.T1.S5: comprehensive fast-path regression suite
  - P1.M1.T1.S6: docs/how-it-works.md fast-path paragraph

UNCHANGED: runLoop (byte-identical); Decompose; isFileDisjoint (S1); invokeStager/invokeStagerRetry;
  the Git interface; freezeSnapshot/verifyFreezeSubset.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...                # the new function + struct compile; unused params (preRunHEAD/isUnborn) are fine (function params, not locals)
go vet ./...
gofmt -l internal/decompose/  # Expected: empty.
make lint                     # Expected: zero errors. (If a linter flags unused params, add `_ = preRunHEAD; _ = isUnborn`.)
```

### Level 2: Unit Tests (Component Validation)

```bash
# The focused sweep test
go test ./internal/decompose/ -run TestRunLoopFastPath_Sweep -v
# Expected: PASS — the sweep runs, returns the S3 sentinel, staged count == 3 for 3 disjoint concepts.

# Existing runLoop/Decompose tests must pass UNCHANGED (runLoop byte-identical)
go test ./internal/decompose/ -run TestDecompose -v

# Full decompose package
go test ./internal/decompose/... -v

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# (S2's fast-path is not yet wired into Decompose — S4 does the dispatch. The within-scope proof is the
#  focused unit test [sweep → sentinel + staged count]. The full FR-M13 fast-path e2e [disjoint tree →
#  git-add staging → concurrent messages → N commits] is validated in S3+S5's gates, not S2's.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the struct + function exist with the FR-M13/FR-M14 doc comments
grep -n "type stagedConcept\|func runLoopFastPath\|FR-M13\|FR-M14" internal/decompose/decompose.go
# Expected: the struct + the function + the FR citations present.

# Grep guard: runLoop is BYTE-IDENTICAL (S2 adds a sibling, does not edit runLoop)
git diff internal/decompose/decompose.go | grep -E '^-' | grep -v '^---'
# Expected: empty (no deletions — S2 is purely additive). runLoop's body is untouched.

# Grep guard: the sweep uses Add directly (no wrapper, no invokeStager)
grep -n 'deps.Git.Add(ctx, concept.Files)\|invokeStager' internal/decompose/decompose.go | grep -A0 'runLoopFastPath'
# Expected: Add call inside runLoopFastPath; NO invokeStager inside runLoopFastPath (it's runLoop-only).

# Grep guard: the sentinel marks the S3 boundary (prevents premature S4 wiring)
grep -n "runLoopFastPath concurrent phase not yet implemented" internal/decompose/decompose.go
# Expected: 1 match (the sentinel return).

# Grep guard: FR-M1c verifyFreezeSubset runs inside the sweep
grep -n 'verifyFreezeSubset' internal/decompose/decompose.go
# Expected: present in BOTH runLoop and runLoopFastPath.

# Scope-boundary guard: only decompose.go + decompose_test.go changed
git diff --name-only
# Expected: only internal/decompose/decompose.go + internal/decompose/decompose_test.go.

# Scope-boundary guard: no concurrent-phase/dispatch/suite/docs code added (S3/S4/S5/S6)
grep -n 'errgroup\|sync.WaitGroup\|isFileDisjoint(concepts)\|runLoopFastPath(' internal/decompose/decompose.go | grep -v 'func runLoopFastPath'
# Expected: empty (no concurrency primitives, no dispatch call, no isFileDisjoint call — all downstream).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/decompose/` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass

### Feature Validation
- [ ] `stagedConcept` struct defined { idx int; tree, prevTree string }
- [ ] `runLoopFastPath` signature matches runLoop's args; returns `([]CommitResult, []ChainEntry, error)`
- [ ] Sweep: per concept `deps.Git.Add(ctx, concept.Files)` → `freezeSnapshot` → `verifyFreezeSubset` → FR-M8 skip-or-append
- [ ] Sweep is strictly serial (plain `for`; no goroutines)
- [ ] FR-M1c verifyFreezeSubset runs on EVERY frozen tree
- [ ] FR-M8 empty-skip (`treeI == prevTree`) logs via VerboseWarn + continues with prevTree unchanged
- [ ] Sentinel return marks the S3 boundary + carries the staged count
- [ ] Focused test: disjoint concepts → sentinel error + "staged N concepts"

### Scope-Boundary Validation
- [ ] runLoop BYTE-IDENTICAL (the diff is purely additive — no deletions)
- [ ] NO concurrent phase / publish / drainMsg-slice (S3)
- [ ] NO Decompose dispatch wiring / isFileDisjoint call (S4)
- [ ] NO comprehensive regression suite (S5)
- [ ] NO docs (S6)
- [ ] isFileDisjoint (S1), invokeStager/invokeStagerRetry, Git interface UNCHANGED

### Code Quality
- [ ] runLoopFastPath co-located with runLoop (fast-path helpers grouped)
- [ ] Doc comments cite FR-M13/FR-M14 + the serial-concurrency rationale + the S3 boundary
- [ ] Error wrapping mirrors runLoop's ErrDecomposeFailed idiom
- [ ] VerboseWarn guarded with `if deps.Verbose != nil`

---

## Anti-Patterns to Avoid

- ❌ Don't make the sweep concurrent (goroutines/errgroup) — gitRunner has NO index lock; Add/WriteTree mutate `.git/index`. The sweep is a plain serial `for` loop. (S3's concurrency is message-only, confined to read-only tree reads.)
- ❌ Don't edit runLoop — it is the shared-file fallback and must stay byte-identical. runLoopFastPath is a NEW sibling.
- ❌ Don't call `drainMsg` in the sweep — there is no in-flight message channel (message gen is S3). The Add/freeze/verify error paths return `(commits, nil, err)` directly.
- ❌ Don't re-reset the index to baseTree — Decompose's FreezeWorkingTree (step 3) already did. `prevTree := baseTree` is the correct baseline.
- ❌ Don't wrap `deps.Git.Add` — call it directly (`deps.Git.Add(ctx, concept.Files)`). It's nil-safe on empty and stages deletions; system_context confirms it's the second call site (arbiter is first), no wrapper needed.
- ❌ Don't skip `verifyFreezeSubset` "because whole-path adds always pass" — FR-M1c MUST run on every tree (defense-in-depth; catches a concurrent change swept in by a path glob).
- ❌ Don't update `prevTree` on an FR-M8 skip — a skipped concept stages nothing; the next concept stages on top of the same prevTree. Mirror runLoop's skip semantics.
- ❌ Don't implement the concurrent phase / publish / dispatch / comprehensive suite — those are S3/S4/S5. S2 ends at the sentinel return.
- ❌ Don't drop the sentinel and `return nil, nil, nil` — that looks like success with zero commits and would let S4 wire an incomplete fast-path. The sentinel is deliberate and is removed by S3.
- ❌ Don't add a generic `Verbose(msg)` call — it doesn't exist. Use `deps.Verbose.VerboseWarn(msg)` (guarded for nil).

---

## Confidence Score: 9/10

One-pass success is very high: the function is a close mirror of runLoop's verified setup + per-concept
structure (with Add replacing invokeStager and append replacing publish), every helper signature is
confirmed with line numbers, the serial constraint is explicit (a plain `for` loop satisfies it), Add's
nil-safe/deletion semantics are confirmed, and the S3 boundary is a deliberate sentinel. The -1 is for
the focused test's baseTree/tStart setup: driving runLoopFastPath directly requires replicating Decompose's
internal FreezeWorkingTree steps, which is heavier than the existing top-level Decompose tests — the PRP
gives the setup shape but the implementer must confirm the exact repo-init/FreezeWorkingTree idiom from an
existing test. Mitigated by the PRIMARY assertion (sentinel + staged count) being robust to setup details,
and by the option to assert via Verbose logs if direct setup is awkward. The unused-param (preRunHEAD/
isUnborn) lint risk is also flagged with the `_ =` fallback.