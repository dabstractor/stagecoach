# Research Findings — P1.M1.T1.S3 (runLoopFastPath concurrent message gen + CAS-ordered publish + FR-M12 isolation)

Verified by direct source read of `internal/decompose/{decompose,message}.go`, `internal/signal/signal.go`,
`internal/generate/{generate,rescue}.go`, `internal/decompose/chain.go`, and the S1/S2 contracts.
All citations `file:line` are from the LIVE tree (S2 may still be in-flight — its PRP is the CONTRACT).

## 1. S2 CONTRACT (the precondition S3 builds on)

S2 (parallel, "Ready") adds, in `internal/decompose/decompose.go` as a SIBLING to runLoop:
- `type stagedConcept struct { idx int; tree, prevTree string }` — one concept's frozen tree.
- `func runLoopFastPath(ctx, deps Deps, concepts []prompt.PlannerCommit, baseTree, tStart, preRunHEAD string, isUnborn bool) ([]CommitResult, []ChainEntry, error)`
  - Implements ONLY the SERIAL staging sweep: `prevTree := baseTree`; `tStartPaths := DiffTreeNames(baseTree, tStart)` (FR-M1c baseline, once); per-concept `deps.Git.Add(ctx, concept.Files)` → `freezeSnapshot` → `verifyFreezeSubset` → FR-M8 empty-skip (`treeI == prevTree` ⇒ continue, prevTree unchanged) → append `stagedConcept{idx: i, tree: treeI, prevTree: prevTree}`; `prevTree = treeI`.
  - Declares `var commits []CommitResult` + `var chainData []ChainEntry` at the top (S3 reuses both).
  - Ends in a SENTINEL return: `return nil, nil, fmt.Errorf("%w: runLoopFastPath concurrent phase not yet implemented (staged %d concepts; P1.M1.T1.S3)", ErrDecomposeFailed, len(staged))`.
- S3's job: **REMOVE the sentinel, insert the concurrent phase** that consumes `staged`. The sweep stays.

## 2. runLoop (decompose.go:479-636) — THE reference for the concurrent phase

Structure (verified by reading the full body):
1. `launch(i, treeA, treeB) chan msgOut` closure — `ch := make(chan msgOut, 1)`; goroutine: `m, e := generateMessage(ctx, deps, treeA, treeB); ch <- msgOut{conceptIdx: i, treeA, treeB, msg: m, err: e}`. Sends exactly once; buffered(1) ⇒ never blocks.
2. `publish(ch chan msgOut) error` closure:
   - `if ch == nil { return nil }`; `res := <-ch`; `signal.ClearSnapshot()`.
   - `if res.err != nil`: `errors.As(&re *generate.RescueError)` ⇒ `title := concepts[res.conceptIdx].Title`; `fmt.Fprintln(deps.Out, FormatRescueMulti(re.TreeSHA, re.ParentSHA, re.Candidate, title, res.conceptIdx, len(concepts)))`; `return &DecomposeRescueError{Rescue: re, ConceptTitle: title, Index: res.conceptIdx, Count: len(concepts), Commits: commits}`. Else `return res.err` (HARD, ErrMessageFailed-wrapped).
   - `newSHA, err := publishCommit(ctx, deps, res.treeB, prevSHA, res.msg)`; `errors.As(&ce *generate.CASError)` ⇒ `fmt.Fprintln(deps.Out, ce.Error())`; `return ce`. Else `return err` (HARD, ErrPublicationFailed-wrapped).
   - `isRoot := res.conceptIdx == 0 && isUnborn`; `cr, err := buildCommitResult(ctx, deps, newSHA, res.msg, isRoot)`; append `commits` + `chainData = append(chainData, ChainEntry{SHA: newSHA, Tree: res.treeB, Message: res.msg, Parent: prevSHA})`; `prevSHA = newSHA`; `return nil`.
3. Per-concept loop: stager → freeze → verify → `publish(inflight)` (drains msg[i-1]) → if !skipped: `signal.SetSnapshot(treeI, prevSHA, "")` + `inflight = launch(i, prevTree, treeI)` + `prevTree = treeI`.
4. Final `publish(inflight)`; `return commits, chainData, nil`.

`drainMsg(ch chan msgOut)` (decompose.go:640): `if ch == nil { return }; <-ch`. Nil-safe recv-and-discard of ONE buffered(1) channel.

**THE 1-deep overlap invariant that runLoop relies on:** msg[i] is launched AFTER commit[i-1] publishes. So at launch, `prevSHA == newSHA[i-1]` is KNOWN and correct. This makes BOTH (a) `generateMessage`'s internal `RevParseHEAD` (→ RescueError.ParentSHA) and (b) `signal.SetSnapshot(treeI, prevSHA, "")` see the correct parent. **This invariant is BROKEN on the concurrent fast-path** (see §5).

## 3. Primitives to REUSE (verified signatures — do NOT reimplement)

| Primitive | File:line | Signature / contract |
|---|---|---|
| `generateMessage` | message.go:70 | `func generateMessage(ctx, deps, treeA, treeB string) (string, error)` — tree-to-tree diff (read-only; never touches `.git/index`); returns `*generate.RescueError` on failure. Internally captures `parentSHA` via `RevParseHEAD` (for its RescueError) — see §5 staleness. |
| `publishCommit` | message.go:232 | `func publishCommit(ctx, deps, tree, parentSHA, msg string) (string, error)` — runs commit hooks + `CommitTree` (dangling) + `UpdateRefCAS`; returns newSHA; `*generate.CASError` on CAS fail; `generate.ErrEmptyMessage` if a hook empties the message (bare, NOT a rescue). parentSHA=="" ⇒ root commit. |
| `buildCommitResult` | decompose.go:783 | `func buildCommitResult(ctx, deps, sha, msg string, isRoot bool) (CommitResult, error)` — `DiffTree(sha, isRoot)` for Files. isRoot MUST be correct (root ⇒ `--root` diff). |
| `drainMsg` | decompose.go:640 | single-channel nil-safe drain. KEEP for runLoop. |
| `signal.SetSnapshot` | signal.go:204 | `SetSnapshot(treeSHA, parentSHA, candidate string)` — nil-safe; sets ONE global snapshot (`active.Load()` handler has one `snapTree/snapParent/snapCandidate`). |
| `signal.ClearSnapshot` | signal.go:224 | `ClearSnapshot()` — nil-safe; zeroes the single global snapshot. |
| `FormatRescueMulti` | rescue.go:88 | `FormatRescueMulti(treeSHA, parentSHA, candidateMsg, conceptTitle string, index, count int) string` — parentSHA=="" omits `-p`. |

## 4. Key types (verified)

- `msgOut{ conceptIdx int; treeA, treeB, msg string; err error }` (decompose.go:109) — UNCHANGED; reuse.
- `DecomposeRescueError{ Rescue *generate.RescueError; ConceptTitle string; Index, Count int; Commits []CommitResult }` (decompose.go:83) — has `Error()` + `Unwrap()` (→ RescueError → Kind → ErrRescue/ErrTimeout for exitcode 3/124). Construct IDENTICALLY to runLoop.
- `CommitResult{ SHA, Subject, Message string; Files []git.FileChange }` (decompose.go:53).
- `ChainEntry{ SHA, Tree, Message, Parent string }` (chain.go:25) — PARALLEL to commits; `Parent == chainData[i-1].SHA` (or preRunHEAD / "" root).
- `generate.RescueError{ Kind error; TreeSHA, ParentSHA, Candidate string; Cause error }` (generate.go:112). `Unwrap() → Kind`.
- `generate.CASError{ TreeSHA, Expected, Actual, ActualTree, Message string }` (generate.go:142). `Error()` prints the §13.5 recovery (branches on ActualTree==TreeSHA ⇒ "already committed").

## 5. ⚠️ CRITICAL SUBTLETY — the rescue `parentSHA` is STALE under concurrency; the signal snapshot must arm in the SERIAL publish loop

**The problem.** On the concurrent fast-path, ALL N messages launch BEFORE any commit publishes. Therefore:
- `generateMessage`'s internal `RevParseHEAD` (→ `RescueError.ParentSHA`) RACES: for concept i≥1 it may capture `preRunHEAD` (if the goroutine ran before commit[i-1] landed) instead of the correct `newSHA[i-1]`. The published commit i's TRUE parent is `newSHA[i-1]`, so a stale `re.ParentSHA` would print a WRONG manual-recovery `commit-tree -p` recipe.
- `signal.SetSnapshot` writes a SINGLE GLOBAL snapshot (`snapParent`). Arming it "per in-flight message" at LAUNCH time would (a) race if done from goroutines, and (b) even if done serially in the launch loop, pass `prevSHA == preRunHEAD` for every concept (nothing published yet) — wrong for i≥1.

**The fix (both stem from one principle: the publish loop is the authoritative serialization point).**
1. **Arm the signal in the SERIAL publish loop, not at concurrent launch.** Right before draining concept i's channel: `signal.SetSnapshot(sc.tree, prevSHA, "")` where `prevSHA == newSHA[i-1]` (authoritative — commit[i-1] just published). `signal.ClearSnapshot()` immediately after `<-ch`. The publish loop is serial ⇒ no race on the global snapshot, and the parent is always correct. (Best-effort during pure generation: the loop arms concept 0's snapshot as soon as it starts draining; a SIGINT during the overlap window rescues concept 0 with parent preRunHEAD — a correct, sensible recovery command for the first commit that would land. This is the best a single-global snapshot can do for N concurrent messages.)
2. **Override the stale rescue parent.** On `*generate.RescueError` at concept i, do NOT trust `re.ParentSHA`. Use the publish loop's authoritative `prevSHA`. Fix a shallow copy so the printed recipe AND the `DecomposeRescueError.Rescue` both carry the correct parent:
   ```go
   fixed := *re              // shallow copy — don't mutate the received pointer
   fixed.ParentSHA = prevSHA // authoritative newSHA[i-1]; generateMessage's capture may be stale
   fmt.Fprintln(deps.Out, generate.FormatRescueMulti(fixed.TreeSHA, fixed.ParentSHA, fixed.Candidate, title, sc.idx, len(concepts)))
   return commits, nil, &DecomposeRescueError{Rescue: &fixed, ConceptTitle: title, Index: sc.idx, Count: len(concepts), Commits: commits}
   ```
   Exit-code mapping is unaffected (it checks `Kind`, not ParentSHA). This is the one place S3 CANNOT be a verbatim copy of runLoop's publish closure.

## 6. `isRoot` must use the FIRST PUBLISHED commit, not concept index 0

runLoop computes `isRoot := res.conceptIdx == 0 && isUnborn`. On the fast-path, concept 0 can be FR-M8-empty-skipped (S2), so the first PUBLISHED concept may have `sc.idx > 0` yet still be the repo's ROOT commit (its `prevSHA == preRunHEAD == ""` on an unborn repo ⇒ `publishCommit` makes it a root). `buildCommitResult` needs `isRoot=true` there or `DiffTree` fails to show the root commit's files. **Use the robust form:** `isRoot := len(commits) == 0 && isUnborn` (true for the first commit appended, iff unborn). For non-unborn repos prevSHA is non-empty ⇒ never root. Correct for commit 2..N too (len(commits) > 0 by then).

## 7. CRITICAL CONCURRENCY (git_primitives.md) — what is and isn't safe

- gitRunner has NO in-process index lock. `.git/index` is shared disk state.
- **Safe to run concurrently (read-only w.r.t. index):** the message goroutines — `generateMessage` does `TreeDiff(treeA, treeB)` + `RevParseHEAD` + reads (all read-only tree reads). `publishCommit`'s `CommitTree` (dangling object) + `UpdateRefCAS` (ref only) touch NO index. ✅
- **MUST stay serial:** the staging sweep (S2 — Add/WriteTree mutate index) and the **publish loop** — `UpdateRefCAS` MUST serialize in CAS order (FR-M7: each CAS requires HEAD == newSHA[i-1]). The publish loop IS the serialization point.
- **No concurrency cap (FR-M14).** Launch all N. `max_commits` default 12 (FR-M4) bounds N. DO NOT add a semaphore / config key.

## 8. drainMsg generalization → add `drainMsgs([]chan msgOut)` (do NOT change drainMsg)

`drainMsg` (single channel) is used by runLoop and MUST stay byte-identical (runLoop is the shared-file fallback; S2/S4 pin it). Add a NEW sibling:
```go
func drainMsgs(chs []chan msgOut) {
    for _, ch := range chs { drainMsg(ch) }   // reuses drainMsg verbatim; nil-safe per channel
}
```
On ANY publish-loop error at concept i, call `drainMsgs(inflight[i+1:])` to drain the N-i-1 still-in-flight channels (the ONLY behavioral difference from runLoop's single `drainMsg(inflight)`). Each goroutine sends exactly once to its buffered(1) channel ⇒ every receive completes ⇒ no goroutine leak.

## 9. Decompose post-loop code is SHARED + UNCHANGED (decompose.go:240-288)

After `runLoop`/`runLoopFastPath` returns `(commits, chainData, err)`:
- On err: `return DecomposeResult{Commits: commits}, err` (partial commits stand; arbiter does NOT run — §18.3).
- Arbiter gate: `if len(commits) > 0 { tipTree := chainData[len-1].Tree; leftoverPaths := DiffTreeNames(tipTree, tStart); if len(leftoverPaths) > 0 { runArbiterPhase(...); rereadFinalCommits(...) } }`.
S3 returns the SAME `(commits, chainData, err)` shapes ⇒ this code is shared and UNTOUCHED. For a disjoint partition whose union == T_start's path-set, `tipTree == tStart` ⇒ arbiter naturally skipped. **Do NOT touch arbiter.go / chain.go / Decompose.**

## 10. Test harness conventions (decompose_test.go) — S3's focused test

- Helpers: `dcmInitRepo(t, repo)` (git init + identity), `dcmWriteFile`, `dcmRunGit`, `dcmMessageScriptManifest(t, bin, []string{...})` (call-varying stub message manifest), `dcmMessageManifest` (single), `dcmDeps(t, repo, roles)`, `dcmOutBuffer(t, repo, roles) (Deps, *bytes.Buffer)` (captures rescue/CAS on `deps.Out`), `dcmShaResolves(t, repo, sha)`, `stubtest.Build(t)` (compile stub binary). `stubtest.NewScript` supports `Env["STAGECOACH_STUB_SLEEP_MS"]` for timing.
- S4 dispatch is NOT wired yet ⇒ tests call `runLoopFastPath` DIRECTLY (like S2's `TestRunLoopFastPath_Sweep`). Replicate Decompose's baseTree/tStart derivation: `baseTree, _ := g.RevParseTree(ctx, "HEAD")`; `tStart, err := g.FreezeWorkingTree(ctx, baseTree)` (resets index to baseTree; leaves working-tree files on disk for the sweep's `Add`). `preRunHEAD, isUnborn, _ := g.RevParseHEAD(ctx)`.
- Disjoint concepts: `[]prompt.PlannerCommit{{Title:"c1",Files:[]string{"a.go"}}, ...}`. Roles need at least `Message` (a stub manifest); `Planner/Stager/Arbiter` can be minimal stubs (runLoopFastPath only uses Message). `deps.stager` is NOT reached on the fast-path.
- Concurrency observable: message stub sleep (e.g. 150ms each); N launched concurrently ⇒ total ≈ 1× latency, NOT N× (assert `elapsed < N×sleep` with generous CI slack — mirror `TestDecompose_Overlap`).
- CAS-order assertion: `git rev-parse <commits[i].SHA>^` == `commits[i-1].SHA` (or preRunHEAD for i==0); `chainData[i].SHA == commits[i].SHA`; `HEAD == commits[last].SHA`.
- FR-M12a rescue: message script fails concept i (empty output + MaxDuplicateRetries=0) ⇒ `errors.As(&dre *DecomposeRescueError)`, `errors.As(&re *generate.RescueError)`, `errors.Is(generate.ErrRescue)`; `HEAD == commits[i-1].SHA` (0..i-1 stand); no goroutine leak (remaining drained — assert via `runtime.NumGoroutine()` delta or just that the call returns).
- The COMPREHENSIVE regression suite (CAS-fail injection, freeze, all-skip, unborn, etc.) is S5. S3 adds a FOCUSED happy-path + rescue test only.

## 11. Scope boundaries (do NOT do in S3)
- Do NOT modify runLoop (byte-identical) or drainMsg (reused by runLoop; add drainMsgs instead).
- Do NOT modify Decompose / arbiter.go / chain.go (post-loop code is shared + unchanged).
- Do NOT wire the dispatch (S4) or add the comprehensive suite (S5) or docs (S6).
- Do NOT touch isFileDisjoint (S1), generateMessage/publishCommit/buildCommitResult (reuse), the Git interface, or the signal package.
- Do NOT add a concurrency cap / semaphore / config key (FR-M14 — launch all N).

## 12. Validation
Edit `internal/decompose/decompose.go` (runLoopFastPath concurrent phase + drainMsgs) + `internal/decompose/decompose_test.go` (focused tests). Gates: `go build ./...`, `go vet ./...`, `gofmt -l internal/decompose/`, `go test ./internal/decompose/ -run 'Decompose|RunLoopFastPath' -race -v`, `go test ./internal/decompose/... -race`, `make test`, `make lint`. **Run with `-race`** — this is the first concurrent path in decompose; the race detector is the net for the §5 signal/CAS reasoning. No external libs.