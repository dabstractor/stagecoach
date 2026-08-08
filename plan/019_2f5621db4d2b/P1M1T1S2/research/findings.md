# Research Findings — P1.M1.T1.S2 (runLoopFastPath serial staging sweep)

## 1. S1 contract + greenfield status

S1 ("Implementing" in parallel) adds `isFileDisjoint(concepts []prompt.PlannerCommit) bool` — the pure
FR-M13 gate. S2 does NOT touch it; S4 wires `if isFileDisjoint(...) { runLoopFastPath } else { runLoop }`.
The fast-path is GREENFIELD (system_context.md: "zero existing code — confirmed by grep"). runLoopFastPath
is a NEW sibling function; runLoop stays BYTE-IDENTICAL (the shared-file fallback).

## 2. runLoop (decompose.go:456) — the reference to mirror

Structure (verified by reading):
1. `prevTree := baseTree`; `prevSHA := preRunHEAD`
2. `tStartPaths, err := deps.Git.DiffTreeNames(ctx, baseTree, tStart)` — FR-M1c baseline, computed ONCE
3. Per-concept loop: `invokeStagerRetry` → `freezeSnapshot` → `verifyFreezeSubset` → `skipped := treeI==prevTree`
   (FR-M8) → `publish(inflight)` → if `!skipped`: `signal.SetSnapshot(...)` + `inflight = launch(...)` + `prevTree = treeI`

**S2's fast-path DIFFERS from runLoop in the loop body ONLY:**
- NO `invokeStagerRetry`/`invokeStager` (no stager agent) → instead `deps.Git.Add(ctx, concept.Files)`
  (deterministic per-path add).
- NO `inflight`/`launch`/`publish`/`drainMsg` in the sweep — the sweep ONLY freezes trees. Message
  generation + CAS publish = S3 (concurrent phase). There is NO in-flight message during the sweep, so
  the verifyFreezeSubset error path returns `(commits, nil, vErr)` with NO drainMsg.
- Collects `[]stagedConcept` instead of publishing per concept.

## 3. CRITICAL concurrency: the sweep MUST be strictly serial

git_primitives.md "⚠️ CRITICAL CONCURRENCY FINDING": gitRunner has NO in-process index lock. `Add` +
`WriteTree`(live) MUTATE `.git/index`. The staging sweep MUST run strictly serially — the caller serializes.
**A plain serial `for` loop (no goroutines) satisfies this.** (S3's concurrent MESSAGE goroutines are
confined to read-only tree reads — DiffTreeNames/TreeDiff/CommitTree/UpdateRefCAS — never Add/WriteTree.)
The index is ALREADY reset to baseTree by Decompose step 3 (FreezeWorkingTree, decompose.go:189), so
`prevTree := baseTree` is the correct baseline (identical to runLoop:459).

## 4. Helper signatures to REUSE (verified — do NOT reimplement)

| Helper | Location | Signature |
|---|---|---|
| `Git.Add` | git.go:1339 (iface ~158) | `Add(ctx, paths []string) error` — nil-safe on empty (`if len==0 return nil`); stages adds/mods/DELETIONS via `git add -- <paths>`. Call `deps.Git.Add(ctx, concept.Files)` directly — NO wrapper. |
| `freezeSnapshot` | stager.go:141 | `freezeSnapshot(ctx, deps Deps) (string, error)` — thin WriteTree wrapper. |
| `verifyFreezeSubset` | stager.go:168 | `verifyFreezeSubset(ctx, deps, baseTree, tStart string, tStartPaths []string, i int, conceptTitle, treeI string) error` — FR-M1c path-subset + hunk-aware check. |
| `Git.DiffTreeNames` | git.go:1847 | `DiffTreeNames(ctx, treeA, treeB) ([]string, error)` — the FR-M1c baseline. |
| `ErrDecomposeFailed` | decompose.go | the wrap sentinel runLoop uses (`fmt.Errorf("%w: ...: %w", ErrDecomposeFailed, err)`). |

**drainMsg (decompose.go:617) is NOT needed in S2** — there is no in-flight message channel during the
sweep. (S3 will need drainMsg GENERALIZED to a slice per system_context.md:58, but that's S3.)

## 5. Verbose API for sweep observability

Available methods: `VerboseCommand`, `VerboseRawOutput`, `VerboseStderr`, `VerbosePayload`, `VerboseWarn(msg)`,
`VerboseRetry(attempt, reason)`, `VerboseRoles`. There is NO generic `Verbose(msg)` info logger — `VerboseWarn(msg string)`
is the free-form logger. Use it for sweep progress (frozen tree / FR-M8 skip / staged-count summary). Guard
with `if deps.Verbose != nil` (some tests pass Verbose: nil). The summary log + the staged count in the
sentinel error consume `staged` (avoids any unused-local lint) AND give S2's test observability.

## 6. The stagedConcept struct + the S3 handoff

```go
type stagedConcept struct {
	idx      int
	tree     string  // frozen write-tree SHA for concept i
	prevTree string  // the tree concept i was staged on top of (tree[i-1], or baseTree for the first)
}
```
S2's sweep produces `[]stagedConcept`. S3 consumes it: for each staged concept, `generateMessage(ctx, deps,
sc.prevTree, sc.tree)` concurrently, then CAS-ordered `publishCommit`. The full function signature is
`(... []CommitResult, []ChainEntry, error)`; S2 implements the sweep + a SENTINEL return marking the S3
boundary (so S4 dispatch is not wired to an incomplete fast-path). After the sweep, every non-skipped
tree[i] is FROZEN — the FR-M14 precondition ("every tree[i] frozen before any message starts").

## 7. The sentinel return (S3 boundary)

```go
stagedCount := len(staged)  // reads `staged` unconditionally (no unused-local/lint issue)
if deps.Verbose != nil {
    deps.Verbose.VerboseWarn(fmt.Sprintf("fast-path: staged %d/%d concepts (concurrent phase pending P1.M1.T1.S3)", stagedCount, len(concepts)))
}
return nil, nil, fmt.Errorf("%w: runLoopFastPath concurrent phase not yet implemented (staged %d concepts; P1.M1.T1.S3)", ErrDecomposeFailed, stagedCount)
```
Honest + prevents premature S4 wiring + testable (assert the sentinel + the count). `commits`/`chainData`
stay nil (the sweep publishes nothing).

## 8. Error/edge handling in the sweep (mirror runLoop)
- ctx cancellation: check `ctx.Err()` at loop top → `return commits, nil, cerr` (partial; none published).
- Add error → `return commits, nil, fmt.Errorf("%w: fast-path add[%d]: %w", ErrDecomposeFailed, i, err)`.
- freezeSnapshot error → `return commits, nil, fmt.Errorf("%w: freeze snapshot[%d]: %w", ErrDecomposeFailed, i, err)`.
- verifyFreezeSubset violation → `return commits, nil, vErr` (NON-RESCUE — no in-flight message; mirrors runLoop but WITHOUT drainMsg).
- FR-M8 empty-skip (`treeI == prevTree`): log via VerboseWarn, `continue` (prevTree unchanged). This is
  also the path for a concept with empty Files (Add is a no-op → treeI==prevTree → skip).

## 9. Scope boundaries (do NOT do)
- Do NOT modify runLoop (stays byte-identical — the shared-file fallback).
- Do NOT add the concurrent phase / publish / drainMsg-slice generalization (S3).
- Do NOT wire Decompose dispatch (S4).
- Do NOT add the comprehensive regression suite (S5) — S2's test is a focused sweep-only validation.
- Do NOT touch isFileDisjoint (S1), invokeStager/invokeStagerRetry, GenerateBootstrapConfig, or the Git interface.
- Do NOT add docs (internal function — Mode A docs land in S6).

## 10. Validation

decompose-package edit + focused test. Gates: `go build ./...`, `go vet ./...`, `gofmt -l internal/decompose/`,
`go test ./internal/decompose/...` (existing tests pass — runLoop unchanged), `make test`, `make lint`.
S2's test: real git repo + disjoint concepts → call runLoopFastPath → assert sentinel error + staged count.
No external libs.