# System Context — Decompose Pipeline (for the file-disjoint fast-path)

Verified by direct source read + 4 parallel scout subagents (2026-08-08).
All citations `file:line`. The fast-path is **greenfield** (zero existing code —
confirmed by grep for `disjoint|fastpath|fast-path|isFileDisjoint|runLoopFastPath`
across `internal/decompose/`).

## 1. Entry point & dispatch site (the single insertion point)

- **`Decompose()`** — `internal/decompose/decompose.go:144`
  `func Decompose(ctx context.Context, deps Deps) (DecomposeResult, error)`
- **Freeze + lock snapshot** — `decompose.go:189-192`
  ```go
  tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)
  lock.SetSnapshot(tStart) // FR52 lock snapshot (nil-safe)
  ```
  After freeze, **index == baseTree (clean)**; working-tree files on disk unchanged.
- **POST-PLANNER DISPATCH SITE** — `decompose.go:228-238` (verbatim, the lines to branch BEFORE):
  ```go
  // (4) FR-M11 single-shortcut
  if out.Single {
      return runSingleShortcut(ctx, deps, out.Message, preRunHEAD, isUnborn, baseTree, tStart)
  }
  // (5) Safety cap enforced inside callPlanner ...
  // (6) The loop (1-deep overlap, FR-M8 empty-skip, serialized CAS, FR-M12 isolation).
  commits, chainData, err := runLoop(ctx, deps, out.Commits, baseTree, tStart, preRunHEAD, isUnborn)
  ```
  The fast-path dispatch (`if isFileDisjoint(out.Commits) { runLoopFastPath(...) } else { runLoop(...) }`)
  inserts between the `runSingleShortcut` block (ends `:230`) and the `runLoop` call (`:237`).
- **Variables in scope at the dispatch site** (all live at line 237):
  `ctx`, `deps`, `out prompt.PlannerOutput` (here `out.Single==false`, `len(out.Commits)>=2`),
  `baseTree`, `tStart`, `preRunHEAD`, `isUnborn`. `signal` (internal/signal) imported at `decompose.go:21`.

## 2. runLoop() — the reference implementation (fast-path mirrors/simplifies it)

- **Signature** — `decompose.go:456`
  `func runLoop(ctx, deps, concepts []prompt.PlannerCommit, baseTree, tStart, preRunHEAD string, isUnborn bool) ([]CommitResult, []ChainEntry, error)`
  Returns `(commits, chainData, err)`. On ANY error, returned `commits` are PARTIAL (0..i-1) and STAND.
- **Algorithm** (`decompose.go:456-615`):
  1. `prevTree := baseTree`; `prevSHA := preRunHEAD`.
  2. `tStartPaths, _ := deps.Git.DiffTreeNames(ctx, baseTree, tStart)` — the FR-M1c subset baseline (computed once).
  3. `launch(i, treeA, treeB) chan msgOut` — buffered(1) channel; goroutine runs `generateMessage` + sends one `msgOut`.
  4. `publish(ch chan msgOut) error` — `res := <-ch`; `signal.ClearSnapshot()`; on `*RescueError` → `FormatRescueMulti` + return `*DecomposeRescueError`; on err → propagate; else `publishCommit(ctx, deps, res.treeB, prevSHA, res.msg)` (CAS expected-old=`prevSHA`); on `*CASError` → print + return `ce`; else `buildCommitResult`; append to `commits`+`chainData`; `prevSHA = newSHA`.
  5. `invokeStagerRetry(concept)` — retry-once-then-empty (FR-M12d) + `ErrStagerMovedHEAD` guard.
  6. Per-concept loop: `invokeStagerRetry` → `freezeSnapshot` → `verifyFreezeSubset` → `skipped := treeI==prevTree` (FR-M8) → `publish(inflight)` (drains msg[i-1]) → if `!skipped`: `signal.SetSnapshot(treeI, prevSHA, "")` + `inflight = launch(i, prevTree, treeI)` + `prevTree = treeI`.
  7. Final `publish(inflight)`; return `(commits, chainData, nil)`.
- **runLoop must stay byte-identical** — the fast-path is a SIBLING function; runLoop is the shared-file fallback.

## 3. Helper primitives to REUSE (do NOT reimplement)

| Primitive | File:line | Signature |
|---|---|---|
| `freezeSnapshot` | `stager.go:141` | `func freezeSnapshot(ctx, deps Deps) (string, error)` — thin `deps.Git.WriteTree` wrapper |
| `verifyFreezeSubset` | `stager.go:168` | `func verifyFreezeSubset(ctx, deps, baseTree, tStart string, tStartPaths []string, i int, conceptTitle, treeI string) error` — FR-M1c path-subset + hunk-aware 3-way check |
| `generateMessage` | `message.go:70` | `func generateMessage(ctx, deps, treeA, treeB string) (string, error)` — tree-to-tree diff; returns `*generate.RescueError` on failure |
| `publishCommit` | `message.go:232` | `func publishCommit(ctx, deps, tree, parentSHA, msg string) (string, error)` — runs commit hooks, `CommitTree`, `UpdateRefCAS`; returns newSHA; `*generate.CASError` on CAS fail |
| `buildCommitResult` | `decompose.go:760` | `func buildCommitResult(ctx, deps, sha, msg string, isRoot bool) (CommitResult, error)` |
| `drainMsg` | `decompose.go:617` | `func drainMsg(ch chan msgOut)` — nil-safe recv-and-discard of a buffered(1) channel. **MUST generalize to a slice** for the fast-path (drain N-i-1 channels on failure). |
| `invokeStager` | `decompose.go:784` | the stager seam — `deps.stager` if non-nil else `stageConcept`. Called ONLY inside `runLoop` (via `invokeStagerRetry`). Fast-path must NEVER reach it. |

**Signal snapshot API** — `internal/signal/signal.go` (package-level, nil-safe):
- `SetSnapshot(treeSHA, parentSHA, candidate string)` — `:204` — arms rescue path
- `ClearSnapshot()` — `:224` — disarms
- `RestoreDefault()` — `:235` — one-shot+permanent (runLoop does NOT use it in the loop; it toggles Set/Clear)

## 4. Key types

- **`PlannerCommit`** — `internal/prompt/planner.go:83-87`:
  ```go
  type PlannerCommit struct {
      Title       string   `json:"title"`
      Description string   `json:"description"`
      Files       []string `json:"files"` // the disjointness-test input + git-add pathspec source
  }
  ```
- **`PlannerOutput`** — `planner.go:92-98`: `{Count int; Single bool; Commits []PlannerCommit; Message string}`
- **`CommitResult`** — `decompose.go:53-62`: `{SHA, Subject, Message string; Files []git.FileChange}`
- **`DecomposeResult`** — `decompose.go:72-75`: `{Commits []CommitResult; Amended int}`
- **`ChainEntry`** — `internal/decompose/chain.go:25-34`: `{SHA, Tree, Message, Parent string}` (parallel array to commits; consumed unchanged by the arbiter)

## 5. Arbiter runs UNCHANGED after either path

After `Decompose` gets `(commits, chainData, err)`, the arbiter gate
(`decompose.go:248-268`, `DiffTreeNames(tipTree, tStart)`) + `runArbiterPhase` consume
`tipTree`/`tStart`/`chainData`. The fast-path produces these in identical shapes, so
`arbiter.go`/`chain.go` are NOT touched. For a file-disjoint partition whose union == T_start's
path-set, `tipTree == tStart` → arbiter naturally skipped (`len(leftoverPaths)==0`).

## 6. Stager test seam (for "stager never called on fast-path" assertions)

`deps.stager` is an unexported field of `Deps` (`internal/decompose/roles.go:~88-95`), nil in
production. The ONLY consumer is `invokeStager` (`decompose.go:784`) ← `invokeStagerRetry`
(`decompose.go:540`) ← `runLoop` per-concept loop (`decompose.go:565`). Because the fast-path
bypasses `runLoop` entirely, the seam is unreachable by construction. A test injects a
`deps.stager` that `t.Fatal`s and drives `Decompose` (or `runLoopFastPath`) with disjoint concepts.