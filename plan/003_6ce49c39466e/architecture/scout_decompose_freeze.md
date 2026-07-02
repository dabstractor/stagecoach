# Scout: DECOMPOSE Orchestrator + GIT Layer — T_start Freeze & One-File Short-Circuit

## (a) `git.Git` Interface Methods — Signatures & Locations

All in `internal/git/git.go`. Interface declaration block: lines 59–210. Each method's interface
doc-comment + signature line is listed; the implementation (`*gitRunner`) line follows.

| Method | Interface (file:line) | Impl (file:line) | Signature |
|--------|----------------------|------------------|-----------|
| `RevParseHEAD` | git.go:68–69 | git.go:313 | `RevParseHEAD(ctx) (sha string, isUnborn bool, err error)` |
| `WriteTree` | git.go:75–76 | git.go:337 | `WriteTree(ctx) (sha string, err error)` — materializes INDEX → tree SHA |
| `CommitTree` | git.go:81–82 | git.go:365 | `CommitTree(ctx, tree string, parents []string, msg string) (sha string, err error)` |
| `UpdateRefCAS` | git.go:88–89 | git.go:407 | `UpdateRefCAS(ctx, ref, newSHA, expectedOld string) error` — sole ref mutation |
| `DiffTree` | git.go:94–95 | git.go:434 | `DiffTree(ctx, sha string, isRoot bool) ([]FileChange, error)` |
| `StagedDiff` | git.go:101–102 | git.go:530 | `StagedDiff(ctx, opts StagedDiffOptions) (diff string, err error)` — index-vs-HEAD |
| `HasStagedChanges` | git.go:108–109 | git.go:644 | `HasStagedChanges(ctx) (bool, error)` |
| `RecentMessages` | git.go:113–114 | git.go:676 | `RecentMessages(ctx, n int) ([]string, error)` |
| `RecentSubjects` | git.go:118–119 | git.go:728 | `RecentSubjects(ctx, n int) ([]string, error)` |
| `CommitCount` | git.go:123–124 | git.go:765 | `CommitCount(ctx) (int, error)` |
| `AddAll` | git.go:128 | git.go:840 | `AddAll(ctx) error` — `git add -A`, stages all (new/mod/del) |
| `Add` | git.go:138 | git.go:858 | `Add(ctx, paths []string) error` — `git add -- <paths>` |
| `StagedFileCount` | git.go:145 | git.go:896 | `StagedFileCount(ctx) (int, error)` |
| `RevParseTree` | git.go:155 | git.go:924 | `RevParseTree(ctx, ref string) (tree string, err error)` — `git rev-parse <ref>^{tree}`; returns ("",nil) on unborn |
| `ReadTree` | git.go:138 (doc) / 139 (sig) | git.go:944 | `ReadTree(ctx, tree string) error` — **REPLACES** index with tree contents (`git read-tree <tree>`) |
| `TreeDiff` | git.go:163 | git.go:962 | `TreeDiff(ctx, treeA, treeB string, opts StagedDiffOptions) (diff string, err error)` |
| `StatusPorcelain` | git.go:174 | git.go:1058 | `StatusPorcelain(ctx) (output string, err error)` — trimmed stdout of `git status --porcelain` |
| `WorkingTreeDiff` | git.go:188 | git.go:1078 | `WorkingTreeDiff(ctx, opts StagedDiffOptions) (diff string, err error)` — `git diff` (no --cached; working-tree-vs-index) |
| `LogRange` | git.go:207 | git.go:787 | `LogRange(ctx, baseSHA string) ([]LogEntry, error)` |

### Key types for diff methods
`StagedDiffOptions` (git.go:23–32): `MaxDiffBytes int`, `MaxMDLines int`, `Excludes []string`,
`BinaryExtensions []string`.

`FileChange` (git.go:11–14): `Status string`, `SrcPath string`, `Path string`.

`LogEntry` (git.go:20–23): `SHA string`, `Subject string`.

### INDEX-mutating methods (the complete set — there are ONLY three)
1. `AddAll` — git.go:840 — stages everything
2. `Add` — git.go:858 — stages specific paths
3. `ReadTree` — git.go:944 — **REPLACES** index with a tree's contents

### ⚠️ NO reset-index / restore-index helper exists
There is **no** `ResetIndex`, `RestoreIndex`, `git reset`, or `git restore` method anywhere in the
`git.Git` interface or implementation (confirmed by exhaustive grep of `internal/git/`).
The closest operation is `ReadTree(T_start)` — it replaces the index with T_start's contents.
To implement "reset index to T_start", use `ReadTree(T_start)`.

---

## (b) `decompose.go` — Routing, Planner, Loop, Arbiter — Anchor Points

File: `internal/decompose/decompose.go`. Entry point: `Decompose()` at **decompose.go:139**.

### Mode routing (decompose.go:139–196)
```
decompose.go:143  — if Single || Commits==1 → runSingleEscape (planner bypassed, AddAll→CommitStaged)
decompose.go:148  — RevParseHEAD → preRunHEAD, isUnborn
decompose.go:152  — baseTree = EmptyTreeSHA (unborn) or RevParseTree("HEAD")
decompose.go:150  — callPlanner(ctx, deps, Config.Commits, isUnborn)  ← planner call
decompose.go:168  — if out.Single → runSingleShortcut (FR-M11 one-agent shortcut)
decompose.go:176  — runLoop(ctx, deps, out.Commits, baseTree, preRunHEAD, isUnborn)
decompose.go:184  — StatusPorcelain → arbiter gate
decompose.go:187  — if status != "" && len(commits) > 0 → runArbiterPhase
decompose.go:194  — rereadFinalCommits (post-arbiter re-read)
```

### 🔑 T_start capture insertion point
**decompose.go:148–166** is the derivation block. T_start should be captured **after** `isUnborn`/
`preRunHEAD`/`baseTree` are derived (decompose.go:152–158) and **before** `callPlanner` (decompose.go:150).
The capture sequence is: `AddAll()` → `WriteTree()` → returns T_start SHA. This mirrors the
existing `runSingleShortcut` pattern (decompose.go:229–234: `AddAll` → `WriteTree` → `treePrime`).
After capturing T_start, the index must be restored to the working-tree state for the stager loop via
`ReadTree(T_start)` (since `AddAll` staged everything; the per-concept stager works from a clean base).

Natural insertion (pseudocode):
```
// decompose.go:~166, AFTER baseTree derivation, BEFORE callPlanner
tStart, err := freezeTStart(ctx, deps)  // AddAll → WriteTree → SHA; then ReadTree(T_start) to reset index
```

### 🔑 One-file short-circuit insertion point
**decompose.go:148–150** — BEFORE `callPlanner`. The short-circuit checks whether the working tree
has exactly ONE changed file; if so, skip the planner entirely and fall through to the single-commit
escape path (or runSingleShortcut-style: AddAll → WriteTree → message → publish).
Detection options:
- `StatusPorcelain(ctx)` → count non-empty lines (decompose.go already calls it at line 184 for the arbiter gate)
- `WorkingTreeDiff(ctx, opts)` → check the planner's input for single-file

Natural insertion (pseudocode):
```
// decompose.go:~148, AFTER RevParseHEAD, BEFORE callPlanner
status, _ := deps.Git.StatusPorcelain(ctx)
if fileCount(status) == 1 {
    return runSingleShortcut(ctx, deps, "", preRunHEAD, isUnborn, baseTree)
    // (or a dedicated one-file path that generates its own message)
}
```

### Per-concept loop: `runLoop` (decompose.go:287–420)
- `launch` goroutine factory: decompose.go:294–302 (generateMessage in a goroutine, buffered(1) chan)
- `publish` drain-and-publish closure: decompose.go:308–351 (drains msg chan → publishCommit in order)
- `invokeStagerRetry` closure: decompose.go:353–378 (retry-once-then-empty; HEAD-move guard)
- Main loop body: decompose.go:380–410
  - decompose.go:382–386: `invokeStagerRetry(concept)` then `freezeSnapshot(ctx, deps)` → treeI
  - decompose.go:391: `skipped := treeI == prevTree` (FR-M8 empty-skip)
  - decompose.go:394–397: `publish(inflight)` (drain+publish previous concept)
  - decompose.go:400–403: if not skipped → `signal.SetSnapshot(...)` + `launch(i, prevTree, treeI)`
- Final drain: decompose.go:413

### Arbiter phase: `runArbiterPhase` (decompose.go:456–482)
- decompose.go:462: `WorkingTreeDiff` (leftover diff for arbiter input)
- decompose.go:472: `runArbiter` (decides target)
- decompose.go:474: `computeAmended`
- decompose.go:476: `resolveArbiter` (executes the decision — AddAll/Add/ReadTree/WriteTree/CommitTree/UpdateRefCAS)

---

## (c) `stageConcept` and `freezeSnapshot` (stager.go)

File: `internal/decompose/stager.go`.

### `stageConcept` — stager.go:58–91
```go
func stageConcept(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error
```
- Invokes the **tooled** stager agent ONCE (no retry, no output parse) for one concept.
- Pipeline: `ResolveRoleModel("stager")` → `BuildStagerTask(concept.Title, concept.Description)` →
  `Render(mdl, "", "", task, RenderTooled)` → `Execute` once.
- Returns `nil` on success (the agent mutated the INDEX via git add/git apply --cached);
  returns `ErrStagerFailed`-wrapped error on any failure.
- **Does NOT read the working tree itself** — it delegates all git operations to the tooled agent.
  The orchestrator owns retry (FR-M8) and calls `freezeSnapshot` after it returns.

### `freezeSnapshot` — stager.go:108–118
```go
func freezeSnapshot(ctx context.Context, deps Deps) (string, error)
```
- Thin wrapper over `deps.Git.WriteTree(ctx)` → returns the tree SHA.
- Freezes the **current index** into an immutable tree object (PRD §13.6.3 invariant 1).
- WriteTree writes a tree object to the object store but does NOT modify `.git/index` or HEAD.
- Called at decompose.go:386 after each `invokeStagerRetry`.

---

## (d) How the Stager Reads/Uses the Working Tree

The stager (`stageConcept`) does **NOT** directly read the working tree through the `git.Git`
interface. Instead:

1. **The tooled agent** (RenderTooled mode) is given a task built from `concept.Title` +
   `concept.Description` (via `prompt.BuildStagerTask`). The agent itself runs `git add` /
   `git apply --cached` commands to stage the concept's files into the index.
2. `stageConcept` only checks the agent's exit code — the INDEX is the truth source (stager.go:84–88).
3. After `stageConcept` returns, the orchestrator calls `freezeSnapshot` → `WriteTree` to snapshot
   the index (decompose.go:386).

**The working tree is read explicitly in two other places:**
- `callPlanner` (planner.go:70): `WorkingTreeDiff` captures the full working-tree diff for the planner.
- `runArbiterPhase` (decompose.go:462): `WorkingTreeDiff` captures leftovers after the loop.

**The index is read/reset via:**
- `ReadTree` (chain.go:201): mid-chain arbiter rebuild replaces the index with tree[j].
- `WriteTree` (freezeSnapshot stager.go:114 / decompose.go:386): reads the index → tree SHA.
- `HasStagedChanges` / `StagedFileCount`: read-only index queries.

---

## (e) Empty-Tree Constant & Reset-Index Helper

### ✅ `EmptyTreeSHA` constant EXISTS
**git.go:500**:
```go
const EmptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
```
Used as the unborn-repo base tree (tree[-1] / treeA) in:
- decompose.go:140 (`baseTree := git.EmptyTreeSHA`)
- chain.go:107 (`treeA = git.EmptyTreeSHA`)
- TreeDiff callers pass it as treeA for the unborn base case.

### ❌ NO resetIndex / restore-index helper exists
Confirmed: exhaustive grep of `internal/git/` and `internal/decompose/` finds **no** method named
`ResetIndex`, `RestoreIndex`, `Restore`, or any `git reset` / `git restore` invocation. The only
index-mutating primitives are `AddAll`, `Add`, and `ReadTree` (which REPLACES the index).

**To implement "reset index to T_start":** call `deps.Git.ReadTree(ctx, T_start)`.
This is the existing pattern in `resolveMidChain` (chain.go:201).

---

## Architecture Summary

```
Decompose (decompose.go:139)
├── [mode routing]
│   ├── Single || Commits==1 → runSingleEscape (decompose.go:143, AddAll→CommitStaged)
│   └── else → planner path
├── [derive] RevParseHEAD → isUnborn, preRunHEAD; RevParseTree("HEAD") → baseTree (decompose.go:148-166)
│   *** T_start INSERTION POINT: here (AddAll→WriteTree→ReadTree to reset) ***
│   *** one-file short-circuit INSERTION POINT: here (StatusPorcelain count check) ***
├── callPlanner (decompose.go:150) → WorkingTreeDiff → planner agent → PlannerOutput
├── [if out.Single] runSingleShortcut (decompose.go:168)
├── runLoop (decompose.go:176)
│   └── per concept i:
│       ├── invokeStagerRetry (decompose.go:382) → stageConcept (stager.go:58) [tooled agent mutates index]
│       ├── freezeSnapshot (decompose.go:386) → WriteTree [index→tree[i] SHA]
│       ├── empty-skip check (decompose.go:391)
│       ├── publish(inflight) — drain msg[i-1] + publishCommit (decompose.go:394)
│       └── launch msg[i] goroutine (decompose.go:400) — generateMessage uses TreeDiff(prevTree,tree[i])
├── [arbiter gate] StatusPorcelain (decompose.go:184)
│   └── runArbiterPhase (decompose.go:187)
│       ├── WorkingTreeDiff (leftovers) (decompose.go:462)
│       ├── runArbiter → decides target
│       └── resolveArbiter (chain.go:42) → AddAll/Add/ReadTree/WriteTree/CommitTree/UpdateRefCAS
└── rereadFinalCommits (decompose.go:194) → LogRange + DiffTree
```

## Start Here
Open **`internal/decompose/decompose.go:139`** (`Decompose` entry point) — the T_start capture and
one-file short-circuit both insert into the derivation block at decompose.go:148–166, before the
planner call at line 150. The git primitive for the freeze is `AddAll` + `WriteTree`
(existing pattern at decompose.go:229–234); for index restoration use `ReadTree(T_start)`.

## Key Dependencies for the Change
- `git.Git` interface: `internal/git/git.go:59–210` (add methods here if a new one is needed)
- `git.Git` impl: `internal/git/git.go` (mirror any interface addition in `*gitRunner`)
- `EmptyTreeSHA`: `internal/git/git.go:500`
- `freezeSnapshot`: `internal/decompose/stager.go:108` (reusable for T_start)
- `Deps` struct: `internal/decompose/roles.go:54` (the `Git git.Git` field carries the interface)
- Tests: `internal/decompose/stager_test.go`, `internal/decompose/decompose_test.go`

## Open Questions / Risks
1. **ReadTree for index reset**: `ReadTree(T_start)` replaces the index with T_start's tree, but the
   working tree files on disk are untouched — the stager agent may see "modified" files relative to
   the reset index. This may or may not be the desired behavior for the stager (it currently works
   against an index that matches the working tree). Verify the intended semantics.
2. **One-file detection method**: `StatusPorcelain` includes untracked files (`??`); `WorkingTreeDiff`
   does NOT (git diff omits untracked). Choose the right detector based on whether untracked files
   should trigger the short-circuit.
3. **T_start vs baseTree**: `baseTree` (decompose.go:152) is `HEAD^{tree}` (the committed parent tree);
   `T_start` would be the full working-tree change set staged as a tree. They are different objects.
   Ensure the loop's `prevTree := baseTree` initialization (decompose.go:291) still uses baseTree, not T_start.
