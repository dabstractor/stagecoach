# Research Findings — P3.M1.T1.S2 (Wire T_start into the Decompose orchestrator)

Scope: MODIFY the decompose orchestrator so the planner, single-shortcut, and arbiter draw from a
frozen `T_start` (the working-tree change set captured at run start) instead of live working-tree
re-reads. Consumes P3.M1.T1.S1's `FreezeWorkingTree` + the EXISTING `TreeDiff` (git.go:1011). No NEW
files; pure wiring. The stager loop, message gen, and arbiter *resolution* (staging) are UNCHANGED —
they rely on the working-tree-unchanged invariant; the freeze ENFORCEMENT (subset check) is the NEXT
task (P3.M2.T1.S1).

## §1 — The freeze capture: WHERE + WHY baseTree ≠ T_start

**Insertion point (decompose.go, `Decompose()`):** AFTER the baseTree/preRunHEAD derivation block
(currently the `if !isUnborn { baseTree, err = RevParseTree(ctx, "HEAD") }` block) and BEFORE
`callPlanner`. Add exactly:
```go
tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)
if err != nil {
    return DecomposeResult{}, fmt.Errorf("%w: freeze working tree: %w", ErrDecomposeFailed, err)
}
```
`FreezeWorkingTree(ctx, baseTree)` does AddAll → WriteTree(tStart) → ReadTree(baseTree). The index is
left == baseTree (reset); the working tree is UNCHANGED (read-tree rewrites .git/index only). This is
S1's contract (verified implemented at git.go:1223).

**baseTree ≠ T_start (the contract's load-bearing distinction):**
- `baseTree` = `HEAD^{tree}` (or `git.EmptyTreeSHA` for unborn) — the COMMITTED parent tree. It is the
  loop's `tree[-1]` (the diff base for concept 0's message).
- `tStart` = the working-tree change set staged as a tree object — every modified/added/deleted/untracked
  path AND its byte content at run start. It is the FROZEN record the planner/shortcut/arbiter read.

**KEEP `prevTree := baseTree` (runLoop, decompose.go:291) UNCHANGED.** The per-concept loop's
`prevTree` is the message[i] diff base (tree-to-tree, never index-vs-HEAD — §13.6.3 invariant 2). It
starts at baseTree (the committed parent) and advances to each frozen treeI. The freeze does NOT change
this — `prevTree` is the COMMITTED base, not T_start. (If it were tStart, concept 0's message would diff
tStart→treeI = only concept 0's staging, but against the wrong base — breaking the chain.)

**The escape-hatch (`runSingleEscape`) does NOT freeze.** It returns at decompose.go:132 (BEFORE baseTree
derivation + the freeze). FR-M2(c) defines `--single`/`--commits 1` as "v1 behavior (git add -A → one
CommitStaged)". FR-M1b's enumeration of freeze consumers (planner/stagers/arbiter/one-file-short-circuit
FR-M2b/single-shortcuts FR-M11) deliberately OMITS the escape-hatch. So `runSingleEscape` is UNCHANGED.
This is structurally guaranteed: the freeze insertion is below the escape-hatch return.

## §2 — The freeze is INDEX-IDEMPOTENT (the §18.1 invariant holds on failure)

Before Decompose: nothing is staged (FR-M1: HasStagedChanges false) ⇒ index == HEAD.tree == baseTree.
After FreezeWorkingTree: AddAll (stages all) → WriteTree (captures) → ReadTree(baseTree) (resets index
to baseTree). Net: index == baseTree == HEAD.tree — byte-identical to the starting state. So if
Decompose fails AFTER the freeze (e.g. planner fails, non-rescue), the index is unchanged and the
working tree is unchanged (user's changes still unstaged). The ONLY artifact is a dangling tStart tree
object (harmless; `git gc` reaps it). This preserves §18.1's idempotent-index invariant. The freeze
mutates the index TRANSIENTLY (AddAll then ReadTree back) but the NET effect is zero.

## §3 — callPlanner: WorkingTreeDiff → TreeDiff(baseTree, tStart) (the FROZEN planner diff)

Currently callPlanner (planner.go:59, :65) does:
```go
diff, err := deps.Git.WorkingTreeDiff(ctx, git.StagedDiffOptions{...})  // LIVE working-tree-vs-index
```
At planner time the index == baseTree (nothing staged; post-freeze the index is explicitly reset to
baseTree), so WorkingTreeDiff == working-tree-vs-baseTree == the change set — BUT it's a LIVE read: a
file a concurrent process writes AFTER the freeze appears in it. Replace with:
```go
diff, err := deps.Git.TreeDiff(ctx, baseTree, tStart, git.StagedDiffOptions{...})  // FROZEN
```
`TreeDiff` (git.go:1011, ALREADY EXISTS — used by generateMessage at message.go:53) runs `git diff
treeA treeB` with binary placeholders per FR3c (the SAME opts struct WorkingTreeDiff takes). TreeDiff(
baseTree, tStart) = the change set frozen at run start — a concurrent change is NOT in tStart ⇒
invisible to the planner. Same content as WorkingTreeDiff-under-the-invariant, but frozen.

**callPlanner needs baseTree + tStart threaded.** Current signature: `callPlanner(ctx, deps,
forcedCount, isUnborn)`. New: `callPlanner(ctx, deps, forcedCount, isUnborn, baseTree, tStart string)`.
The error wrap changes from `"working-tree diff"` to `"tree diff"` (no test asserts this substring —
verified: planner_test.go checks ErrPlannerFailed / safety-cap text, not the diff-error substring).

**Edge case (unchanged from today):** if tStart == baseTree (a race emptied the working tree between the
router's FR-M1 check and the freeze), TreeDiff returns "". The planner then partitions an empty diff.
This is the SAME behavior as today's WorkingTreeDiff-returns-"" path — no regression. The router's FR-M1
gate guarantees non-empty in practice.

## §4 — runSingleShortcut: AddAll→WriteTree → treePrime := tStart (the FROZEN shortcut)

Currently runSingleShortcut (decompose.go:227) does:
```go
if err := deps.Git.AddAll(ctx); err != nil { ... }       // LIVE add -A
treePrime, err := deps.Git.WriteTree(ctx)                // snapshot the live-staged index
...
if dupCheckMessage(...) { msg, err = generateMessage(ctx, deps, baseTree, treePrime) }
publishCommit(ctx, deps, treePrime, preRunHEAD, msg)
```
The AddAll is a LIVE re-read — a concurrent change would be swept into the single commit. Replace with:
```go
treePrime := tStart   // FR-M1b: commit the frozen T_start directly (no live AddAll)
msg := plannerMsg
if dupCheckMessage(ctx, deps, plannerMsg, isUnborn) {
    msg, err = generateMessage(ctx, deps, baseTree, tStart)  // message from baseTree→tStart (the whole change set)
    if err != nil { return DecomposeResult{}, err }
}
publishCommit(ctx, deps, treePrime, preRunHEAD, msg)
```
The AddAll + WriteTree calls are REMOVED; `treePrime := tStart`. baseTree is still needed (generateMessage's
treeA). New signature: `runSingleShortcut(ctx, deps, plannerMsg, preRunHEAD string, isUnborn bool,
baseTree, tStart string)`. publishCommit takes the tree SHA directly (CommitTree doesn't touch the
index), so committing tStart works regardless of the live index state.

## §5 — runArbiterPhase: WorkingTreeDiff → TreeDiff(tipTree, tStart) (the FROZEN leftover diff)

Currently runArbiterPhase (decompose.go:465, :467) does:
```go
leftoverDiff, err := deps.Git.WorkingTreeDiff(ctx, git.StagedDiffOptions{...})  // LIVE
```
KEY: `runArbiter` (arbiter.go:79) takes `leftoverDiff string` as a PARAMETER — it does NOT compute the
diff. runArbiterPhase pre-computes it and passes it in. So changing the diff SOURCE is localized to
runArbiterPhase. After the loop, the index == the last committed tree (== chainData[last].Tree), and the
working tree == tStart's source (unchanged). So WorkingTreeDiff (working-tree-vs-index) ==
tStart-vs-tipTree == the leftovers — but LIVE. Replace with:
```go
tipTree := chainData[len(chainData)-1].Tree
leftoverDiff, err := deps.Git.TreeDiff(ctx, tipTree, tStart, git.StagedDiffOptions{...})  // FROZEN
```
tipTree = the last published commit's tree (== HEAD^{tree} post-loop; chainData is guaranteed non-empty
— runArbiterPhase is only called when `len(commits) > 0`, decompose.go:187, and chainData is parallel to
commits). TreeDiff(tipTree, tStart) = "changes from tipTree to tStart" = the leftovers (what's in tStart
but not committed) — FROZEN. New signature: `runArbiterPhase(ctx, deps, commits, chainData, tStart string)`.

**The arbiter's STAGING (resolveNewCommit/resolveTipAmend via AddAll; resolveMidChain via Add) is
UNCHANGED** — it stages from the working tree (== tStart's source under the invariant). The freeze's
OUTPUT guarantee for the staging path is closed by the ENFORCEMENT task (P3.M2.T1.S1, FR-M1c subset
check). This task freezes the arbiter's REASONING input (the diff); the staging mechanism relies on the
working-tree-unchanged invariant + (next task) enforcement.

## §6 — runLoop + stager.go + message.go + chain.go: UNCHANGED

- **runLoop (decompose.go:287):** unchanged. `prevTree := baseTree` is kept (§1). The stagers run
  `git add <path>` against the working tree, which == tStart's source (the freeze didn't touch the
  working tree; stagers/commit-tree/update-ref don't touch it). freezeSnapshot (WriteTree) is unchanged.
  The freeze's safety for the stager path is the ENFORCEMENT task (P3.M2.T1.S1) — THIS task only wires
  tStart into the diff INPUTS + the shortcut.
- **stager.go (stageConcept, freezeSnapshot):** unchanged. stageConcept builds the §17.6 task from the
  concept's title+description; the tooled agent runs git add against the working tree (== tStart's
  source). No tStart threading into stageConcept.
- **message.go (generateMessage, publishCommit):** unchanged. generateMessage already uses TreeDiff(
  treeA, treeB) — already tree-to-tree (frozen). The treeA/treeB are explicit params (baseTree/tStart
  for the shortcut; prevTree/treeI for the loop; tipTree/treePrime for arbiter-new-commit). All frozen.
- **chain.go (resolveArbiter + resolveNewCommit/resolveTipAmend/resolveMidChain):** unchanged. The
  staging uses AddAll/Add against the working tree (== tStart's source). handleUpdateRefErr unchanged.
- **arbiter.go (runArbiter):** unchanged. Takes leftoverDiff as a param (runArbiterPhase now passes the
  frozen diff).
- **roles.go (Deps):** unchanged. T_start/baseTree are threaded as FUNCTION PARAMS (see §7), NOT Deps
  fields.

## §7 — Threading: FUNCTION PARAMS (not a Deps field) — matches the baseTree precedent

The contract says "Thread T_start + baseTree through Deps/closures as needed." DECISION: thread as
function params (trailing), matching the EXISTING baseTree threading pattern (runSingleShortcut +
runLoop already take baseTree as a bare param). Rationale:
- baseTree is ALREADY a bare function param in runSingleShortcut/runLoop — tStart follows the same
  pattern (consistency).
- Deps is for COLLABORATORS + test injection (Git/Registry/Config/Roles/Verbose/Out/stager-seam), NOT
  per-run state. T_start is per-run state (it's empty/meaningless in the escape-hatch path, which
  returns before the freeze). Adding T_start to Deps would be a smell + confusing (zero in escape-hatch).
- Explicit params are clearer + type-checked than struct-field access.
- No closure needs tStart (runLoop is unchanged; it doesn't reference tStart).

New/changed signatures (trailing baseTree, tStart):
- `callPlanner(ctx, deps, forcedCount, isUnborn, baseTree, tStart string)` — +2 params.
- `runSingleShortcut(ctx, deps, plannerMsg, preRunHEAD string, isUnborn bool, baseTree, tStart string)` — +1 param.
- `runArbiterPhase(ctx, deps, commits, chainData, tStart string)` — +1 param (baseTree NOT needed: the
  arbiter diff uses tipTree = chainData[last].Tree, not baseTree).
- `runLoop(...)` — UNCHANGED (already takes baseTree; doesn't need tStart).
- `Decompose(...)` — UNCHANGED signature (tStart is a local var, derived inside).

(If the implementer prefers grouping baseTree+tStart into a small `freeze` struct to keep param counts
≤6, that is acceptable — the contract allows it. But bare trailing params are the established pattern.)

## §8 — Test impact (this is a MODIFICATION task — call sites must update)

**planner_test.go (the most affected):** ~12 direct `callPlanner(ctx, deps, forcedCount, isUnborn)`
calls (lines 84, 112, 138, 158, 183, 206, 227, 258, 301, 324, 348, 382). EACH must add `baseTree, tStart`.
The tests set up an unstaged file then call callPlanner. Today callPlanner reads WorkingTreeDiff (live);
now it reads TreeDiff(baseTree, tStart). So each test must capture baseTree + tStart BEFORE callPlanner.
Add a helper to reduce boilerplate:
```go
// freezeForPlanner captures baseTree + tStart for a callPlanner test (matures: rev-parse HEAD^{tree};
// unborn: EmptyTreeSHA). Mirrors what Decompose() does after baseTree derivation.
func freezeForPlanner(t *testing.T, repo string, isUnborn bool) (baseTree, tStart string) {
    if isUnborn { baseTree = git.EmptyTreeSHA } else { baseTree = runGit(t, repo, "rev-parse", "HEAD^{tree}") }
    g := git.New(repo)
    tStart, err := g.FreezeWorkingTree(context.Background(), baseTree)
    if err != nil { t.Fatalf("freeze: %v", err) }
    return baseTree, tStart
}
```
Then `baseTree, tStart := freezeForPlanner(t, repo, false); ... callPlanner(ctx, deps, 0, false, baseTree, tStart)`.
The stub agent ignores the payload, so most tests just need a valid (baseTree, tStart) pair.

**decompose_test.go:** `runSingleShortcut`/`runArbiterPhase` are NOT called directly (only via Decompose)
— verified by grep. So end-to-end Decompose tests need NO signature changes (the freeze happens INSIDE
Decompose). BUT any direct `callPlanner` calls in decompose_test.go must add the 2 params. The stager
seam (`dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}})`) stages SPECIFIC paths (a
well-behaved stager) — this is what makes the sentinel test (§9) feasible.

## §9 — The NEW freeze-exclusion tests (the behavior THIS task adds)

The freeze's observable effect (testable WITHOUT the enforcement task): the planner/shortcut/arbiter
REASON over the FROZEN tStart, so a working-tree change made AFTER the freeze is invisible to them.

1. **callPlanner diffs tStart (unit):** capture baseTree+tStart; write a sentinel file `sentinel.txt`
   AFTER the freeze; call callPlanner with a planner stub that RECORDS its stdin payload; assert
   `sentinel.txt` (and its content) is ABSENT from the payload (the diff is TreeDiff(baseTree, tStart),
   frozen — the post-freeze sentinel is not in tStart). This is the strongest direct test of §3.
2. **runSingleShortcut commits tStart (unit):** capture baseTree+tStart; write a sentinel AFTER the
   freeze; call runSingleShortcut(..., baseTree, tStart); assert the published commit's tree == tStart
   (the sentinel is absent — the shortcut committed the frozen tree, not a live AddAll).
3. **runArbiterPhase leftover diff is frozen (unit / Decompose-level):** after a loop that leaves
   leftovers, write a sentinel AFTER the freeze; verify the arbiter's leftoverDiff payload (captured via
   an arbiter stub recording stdin) does NOT contain the sentinel.
4. **Decompose-level sentinel (§20.2 "Start-of-run freeze (v2)"):** inject a stager stub that writes a
   sentinel file on its FIRST invocation (simulating a concurrent change mid-run, AFTER the freeze) but
   stages only the concept's path; run Decompose; assert the sentinel appears in NO produced commit and
   remains untracked in the working tree afterward. (Works with the well-behaved dcmStagerSeam that
   stages specific paths; the ENFORCEMENT task hardens the misbehaving-stager case.)

NOTE: tests 1–3 are the cleanest for THIS task (they directly pin the frozen-diff/commit behavior).
Test 4 is the §20.2 property test; its full strength (excluding a misbehaving stager's `git add -A`) is
the ENFORCEMENT task (P3.M2.T1.S1). THIS task's freeze makes the planner/shortcut/arbiter frozen; the
stager staging is enforced next.

## §10 — Scope boundaries + parallel-work safety

**Consumes (from S1, parallel — ALREADY IMPLEMENTED at git.go:1223):** `FreezeWorkingTree(ctx, baseTree)
(tStart, err)`. S1 is done (verified: interface at git.go:211, impl at git.go:1223). DiffTreeNames (also
S1) is NOT used by this task (it's the enforcement subset-check primitive, P3.M2.T1.S1).
**Consumes (existing):** `TreeDiff(ctx, treeA, treeB, opts)` (git.go:1011), `git.EmptyTreeSHA`, the
existing `StagedDiffOptions` struct, `ErrDecomposeFailed`.
**MODIFIES (2 files):** `internal/decompose/decompose.go` (Decompose freeze capture + 3 call sites +
runSingleShortcut body + runArbiterPhase body + doc comments), `internal/decompose/planner.go`
(callPlanner signature + diff source).
**MODIFIES (tests):** `internal/decompose/planner_test.go` (~12 call sites + helper), possibly
`internal/decompose/decompose_test.go` (any direct callPlanner calls + new sentinel tests).
**UNCHANGED (frozen / owned elsewhere):** stager.go, message.go, chain.go, arbiter.go, roles.go (Deps),
git.go, go.mod/go.sum. The stager loop + arbiter staging rely on the working-tree-unchanged invariant;
the freeze ENFORCEMENT (subset check) is P3.M2.T1.S1 (next).
**Parallel-work safety:** S1 (git.go) is done — no conflict. This task edits decompose.go + planner.go,
which S1 does NOT touch. No merge friction.

## §11 — Doc-comment requirement (DOCS: [Mode A])

The contract requires a decompose.go doc comment documenting the T_start freeze boundary (FR-M1b: "the
run owns the freeze, not the stager"). Update:
- The `Decompose()` doc comment: add a paragraph on the freeze — "the first action (after baseTree
  derivation) is FreezeWorkingTree → T_start; the planner/shortcut/arbiter draw from T_start, not the
  live tree; a concurrent working-tree change is invisible to every commit (FR-M1b). The run owns the
  freeze boundary; the stager is an untrusted external agent (enforcement is FR-M1c/P3.M2.T1.S1)."
- The package doc comment (decompose.go top): note the freeze as the run's first action post-routing.
- callPlanner/runSingleShortcut/runArbiterPhase doc comments: note they receive tStart and read the
  FROZEN tree-to-tree diff (not a live re-read).

## §12 — Why this is medium-risk (not low): it's a behavioral change to 3 code paths + ~12 test sites

Unlike S1 (purely-additive primitives), THIS task MODIFIES behavior: the planner/shortcut/arbiter now
read frozen diffs instead of live trees. The change is semantically equivalent UNDER the
working-tree-unchanged invariant (no concurrent process) — so all existing tests pass UNCHANGED in
content (the diff is the same bytes), but the test CALL SITES must add 2 params. The residual risks:
(a) the arbiter leftover diff direction (tipTree→tStart, not tStart→tipTree) — pinned by a test; (b) the
escape-hatch correctly NOT freezing (structurally guaranteed by the insertion point); (c) the index-
idempotency on failure (§2 — holds because index == baseTree before and after). High one-pass confidence
given the code is a direct, localized wiring of one new call + 3 diff-source swaps.
