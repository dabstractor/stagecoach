name: "P1.M1.T1.S3 — runLoopFastPath concurrent message generation + CAS-ordered publish + FR-M12 failure isolation"
description: >
  Complete runLoopFastPath's CONCURRENT PHASE in internal/decompose/decompose.go: after S2's serial
  staging sweep froze every tree[i] into []stagedConcept (the FR-M14 precondition), (a) launch ALL N
  message generations CONCURRENTLY via the buffered(1)-channel launch() pattern from runLoop — each
  goroutine calls generateMessage(ctx, deps, sc.prevTree, sc.tree) (read-only tree reads; never the
  .git/index); (b) PUBLISH STRICTLY IN CAS ORDER (FR-M7) by consuming results in concept order —
  commit[i] parent = prevSHA (preRunHEAD/root for i=0, newSHA[i-1] otherwise) — reusing publishCommit +
  buildCommitResult + ChainEntry accumulation exactly as runLoop's publish closure; (c) FR-M12 isolation:
  on *generate.RescueError at i → publish 0..i-1 (already landed), print FormatRescueMulti naming concept
  i, drainMsgs the remaining i+1..N-1 channels, return *DecomposeRescueError (partial commits stand);
  on *generate.CASError → print ce.Error(), drainMsgs remaining, return ce. Remove S2's sentinel. Add
  drainMsgs([]chan msgOut) as a sibling to drainMsg (do NOT change drainMsg — runLoop is byte-identical).
  runLoop + Decompose + arbiter.go + chain.go UNCHANGED. No concurrency cap (FR-M14; max_commits bounds N).

---

## Goal

**Feature Goal**: Finish runLoopFastPath — the FR-M13/FR-M14 file-disjoint fast-path — by adding the
concurrent message-generation + strictly-CAS-ordered publish + FR-M12 per-concept failure-isolation that
S2 deferred. After S2's serial sweep froze every concept's tree up front (the FR-M14 precondition), this
subtask launches all N message generations concurrently, publishes them in strict chain order, and isolates
any single concept's failure so prior commits stand and no goroutine leaks.

**Deliverable**: (1) the complete `runLoopFastPath` body — S2's sweep + the concurrent phase replacing the
S2 sentinel; (2) `func drainMsgs(chs []chan msgOut)` as a NEW sibling to `drainMsg` (reuses `drainMsg`
verbatim; `drainMsg` + runLoop stay byte-identical); (3) focused tests in `decompose_test.go`
(happy-path N-commit CAS-ordered publish + concurrency timing + FR-M12a rescue isolation + no-leak drain).
runLoop, drainMsg, Decompose, arbiter.go, chain.go UNCHANGED.

**Success Definition**:
- A disjoint N-concept run: all N messages generate concurrently (total ≈ 1 message latency, not N×),
  then publish in strict CAS order (each commit's parent == the previous newSHA; HEAD advances one at a
  time; FR-M7), producing `(commits, chainData, nil)` in the SAME shapes runLoop returns.
- The Decompose post-loop code (arbiter gate `decompose.go:248-268`, rereadFinalCommits, result assembly)
  is shared and UNCHANGED — runLoopFastPath's return feeds it identically.
- FR-M12a: if message[i] fails (`*generate.RescueError`), commits 0..i-1 stand (HEAD == newSHA[i-1]),
  FormatRescueMulti naming concept i is printed, the remaining i+1..N-1 channels are drained (no leak),
  and a `*DecomposeRescueError` carrying the partial commits is returned.
- FR-M12b: a `*generate.CASError` during publish prints `ce.Error()` (§13.5), drains remaining, returns
  `ce` (prior commits stand).
- The rescue parent is the AUTHORITATIVE `newSHA[i-1]` (the publish loop's `prevSHA`) — NOT the
  potentially-stale `parentSHA` `generateMessage` captured under concurrency (see Known Gotchas §5).
- `go build ./...`, `go vet ./...`, `gofmt -l`, `go test ./internal/decompose/... -race`, `make test`,
  `make lint` all pass.

## User Persona (if applicable)

**Target User**: Stagecoach maintainers (internal function; no user-facing surface until S4 wires the
dispatch). Once the full fast-path lands (S4), a disjoint decomposition generates all messages in parallel
and collapses the critical path from "~N sequential (stager ∥ message) steps" to "one message latency +
N cheap git ops" (FR-M14).

**Use Case**: A user with a dirty tree of unrelated whole-file changes runs `stagecoach`; the planner
partitions disjointly; the fast-path stages deterministically (S2, no stager agent) and generates all
messages concurrently (S3).

**Pain Points Addressed**: FR-M14 (P0) — lifts FR-M6's 1-deep overlap ceiling to full message concurrency
on the disjoint fast-path; the opencode/qwen-code "no tooled_flags" provider can decompose a disjoint tree.

## Why

- **FR-M14 (P0)**: On the FR-M13 fast-path the deterministic sweep freezes every `tree[i]` BEFORE any
  message agent starts, so all per-concept diffs are available at once → launch N concurrently, publish in
  CAS order. The §13.6.3 safety invariants are preserved: every message reasons over a tree-to-tree diff
  (never the live index); the `update-ref`s serialize in order with the same CAS guard.
- **FR-M7**: Serialized publication — each CAS requires HEAD == newSHA[i-1]; the publish loop is the
  serialization point. Generation may overlap, publication may not.
- **FR-M12**: Per-concept failure isolation — a failed concept enters rescue for THAT concept; already-
  published commits stand; remaining work is drained.
- **Why a sibling + reuse, not a rewrite**: runLoop is the shared-file (hunk-split) fallback and must stay
  byte-identical. S3 consumes S2's frozen `[]stagedConcept` and reuses runLoop's exact primitives
  (`generateMessage`, `publishCommit`, `buildCommitResult`, `drainMsg`, signal API, `FormatRescueMulti`,
  `DecomposeRescueError`). The ONLY genuinely new code is the publish loop's ordering + the rescue-parent
  correction + `drainMsgs`.

## What

**User-visible behavior**: None directly (S3 is internal; S4 wires the dispatch). Once S4 lands, a disjoint
decomposition runs end-to-end: deterministic staging (S2) → concurrent messages (S3) → N ordered commits.

**Technical change (one function completed + one small helper + focused tests):**
1. Complete `runLoopFastPath`: replace S2's sentinel with the concurrent phase — launch-all + publish-in-
   order + FR-M12 handling + `drainMsgs` on every error path.
2. Add `drainMsgs(chs []chan msgOut)` (loops `drainMsg`).
3. Focused tests: happy-path CAS-ordered N-commit publish + concurrency timing + FR-M12a rescue + no-leak.

### Success Criteria
- [ ] S2's sentinel is REMOVED; the concurrent phase runs after the sweep.
- [ ] All N (non-skipped) messages launch concurrently (buffered(1) channels; one goroutine each).
- [ ] Publish consumes results in concept order; `commit[i]` parent = `prevSHA` (preRunHEAD/root for i=0,
      `newSHA[i-1]` otherwise); `prevSHA = newSHA` after each.
- [ ] `signal.SetSnapshot(sc.tree, prevSHA, "")` arms in the SERIAL publish loop (authoritative parent);
      `signal.ClearSnapshot()` after each drain.
- [ ] Rescue parent uses the authoritative `prevSHA` (shallow-copy fix), NOT `re.ParentSHA`.
- [ ] FR-M12a `*RescueError` → FormatRescueMulti + `drainMsgs(inflight[i+1:])` + `*DecomposeRescueError`
      (Commits = 0..i-1, they stand); FR-M12b `*CASError` → `ce.Error()` + drainMsgs + return ce.
- [ ] `drainMsgs([]chan msgOut)` added; `drainMsg` + runLoop BYTE-IDENTICAL.
- [ ] Returns `(commits, chainData, nil)` on full success; `(commits, nil, err)` on any failure (partial
      commits stand — same contract as runLoop).
- [ ] `len(staged) == 0` (all concepts empty-skipped) returns `(nil, nil, nil)` (Decompose's arbiter gate
      guards `len(commits) > 0`).
- [ ] `go build ./...`, `make test -race`, `make lint` pass; runLoop/Decompose/arbiter/chain unchanged.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — runLoop's full launch/publish/drain reference (the function to mirror, with line numbers), every
reused primitive's verified signature, the critical concurrency findings (what's safe to concurrentize and
what must stay serial), the ONE place S3 cannot copy runLoop verbatim (the stale rescue parent — with the
exact fix), the `isRoot` robust form, the `drainMsgs` helper, the S2 handoff contract (the sweep + the
sentinel to remove), the Decompose-shared post-loop code, and the test harness conventions are all below.

### Documentation & References

```yaml
- file: internal/decompose/decompose.go
  why: "THE home. runLoop (grep `func runLoop(` — live ~:479) is the function to MIRROR for launch()/publish().
        runLoopFastPath (S2's sibling) is the function to COMPLETE (remove the sentinel, add the concurrent
        phase). drainMsg (~:640) is reused verbatim by the new drainMsgs. msgOut (~:109),
        DecomposeRescueError (~:83), ErrDecomposeFailed (~:46), buildCommitResult (~:783) are all here.
        The Decompose post-loop arbiter gate (~:248-268) is SHARED — do NOT touch."
  pattern: "Mirror runLoop's launch() closure verbatim (buffered(1) chan msgOut; goroutine calls
            generateMessage + sends once). Mirror runLoop's publish() result-handling (RescueError→
            FormatRescueMulti+DecomposeRescueError; CASError→ce.Error()+return; else publishCommit+
            buildCommitResult+append commits/chainData; prevSHA=newSHA) — BUT (1) loop over []staged in
            concept order, (2) arm signal in the serial publish loop with authoritative prevSHA, (3) fix
            the rescue parent to prevSHA, (4) drainMsgs(inflight[i+1:]) on every error."
  critical: "runLoop + drainMsg MUST stay byte-identical (runLoop is the shared-file fallback; S2/S4 pin it).
             Add drainMsgs as a NEW sibling. Do NOT edit Decompose/arbiter.go/chain.go."

- file: internal/decompose/message.go
  why: "generateMessage (:70) + publishCommit (:232) — REUSE, do NOT reimplement."
  pattern: "generateMessage(ctx, deps, treeA, treeB) (string, error) — read-only tree-to-tree diff; returns
            *generate.RescueError on failure; internally captures parentSHA via RevParseHEAD (STALE under
            concurrency — see §5). publishCommit(ctx, deps, tree, parentSHA, msg) (string, error) — commit
            hooks + CommitTree (dangling) + UpdateRefCAS; returns newSHA; *generate.CASError on CAS fail;
            parentSHA==\"\" ⇒ root commit."

- file: internal/signal/signal.go
  why: "SetSnapshot (:204) + ClearSnapshot (:224) — nil-safe; ONE global snapshot."
  pattern: "SetSnapshot(treeSHA, parentSHA, candidate); ClearSnapshot(). The handler holds a SINGLE
            snapTree/snapParent/snapCandidate — concurrent arming would race. Arm in the SERIAL publish loop."

- file: internal/generate/rescue.go
  why: "FormatRescueMulti (:88) — the §18.3 multi-commit rescue message."
  pattern: "FormatRescueMulti(treeSHA, parentSHA, candidateMsg, conceptTitle, index, count) string —
            parentSHA==\"\" omits -p. Pass the AUTHORITATIVE prevSHA (not the stale re.ParentSHA)."

- file: internal/decompose/chain.go
  why: "ChainEntry (:25) — {SHA, Tree, Message, Parent}. PARALLEL to commits. Parent==chainData[i-1].SHA
        (or preRunHEAD / \"\" root). Build IDENTICALLY to runLoop's publish closure."
  pattern: "chainData = append(chainData, ChainEntry{SHA: newSHA, Tree: res.treeB, Message: res.msg, Parent: prevSHA})"

- docfile: plan/019_2f5621db4d2b/architecture/system_context.md
  why: "§2 runLoop reference (launch/publish/drain), §3 helper primitives to REUSE (with signatures),
        §5 arbiter-runs-unchanged-after-either-path, §6 stager-seam-unreachable-on-fast-path."
  section: "2. runLoop(); 3. Helper primitives to REUSE; 5. Arbiter unchanged"

- docfile: plan/019_2f5621db4d2b/architecture/git_primitives.md
  why: "⚠️ CRITICAL CONCURRENCY: no index lock ⇒ message goroutines (read-only tree reads) are safe to
        run concurrently; publishCommit (CommitTree+UpdateRefCAS) touches no index but the update-refs MUST
        serialize in CAS order ⇒ the publish loop is the serialization point. No concurrency cap (FR-M14)."
  section: "⚠️ CRITICAL CONCURRENCY FINDING; No concurrency cap in spec"

- docfile: plan/019_2f5621db4d2b/P1M1T1S2/PRP.md
  why: "S2 is the CONTRACT — it adds stagedConcept + runLoopFastPath's SERIAL SWEEP + the sentinel return
        S3 removes. S3 assumes the sweep + the `var commits/chainData` + `prevTree` + `tStartPaths` + the
        `[]staged stagedConcept` slice all exist exactly as S2 specifies."

- docfile: plan/019_2f5621db4d2b/P1M1T1S3/research/findings.md
  why: "Verified every reused signature, the stale-rescue-parent fix (§5), the isRoot robust form (§6),
        the drainMsgs design (§8), the Decompose-shared post-loop code (§9), the test harness (§10)."
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  decompose.go       # THE home: runLoopFastPath (S2 sweep + S3 concurrent phase — REMOVE sentinel, ADD phase);
                     #   +drainMsgs (sibling to drainMsg). runLoop/drainMsg/Decompose UNCHANGED.
  decompose_test.go  # +TestRunLoopFastPath_ConcurrentPublish + TestRunLoopFastPath_RescueIsolation (focused; suite is S5)
  message.go         # generateMessage(70) + publishCommit(232) — REUSE, UNCHANGED
  stager.go          # freezeSnapshot + verifyFreezeSubset (S2's sweep used them) — UNCHANGED
  chain.go           # ChainEntry(25) — REUSE, UNCHANGED
internal/signal/signal.go  # SetSnapshot(204) + ClearSnapshot(224) — REUSE, UNCHANGED
internal/generate/{generate,rescue}.go  # RescueError/CASError(112/142) + FormatRescueMulti(88) — REUSE, UNCHANGED
```

### Desired Codebase tree with files to be added

```bash
internal/decompose/decompose.go       # MODIFY: runLoopFastPath concurrent phase (replaces S2 sentinel) + drainMsgs
internal/decompose/decompose_test.go  # MODIFY (additive): +focused concurrent/rescue tests
```

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (runLoop's 1-deep overlap invariant is BROKEN on the concurrent fast-path). In runLoop,
//   msg[i] launches AFTER commit[i-1] publishes, so generateMessage's internal RevParseHEAD (→
//   RescueError.ParentSHA) and signal.SetSnapshot(prevSHA) both see the correct newSHA[i-1]. On the
//   fast-path ALL N launch before any publish, so for i≥1: (a) generateMessage's captured parentSHA can
//   be stale (preRunHEAD if the goroutine ran before commit[i-1] landed), and (b) prevSHA is only known
//   at publish time. FIX BOTH by making the SERIAL publish loop the authority: arm signal.SetSnapshot
//   there with prevSA, and on RescueError OVERRIDE the parent to prevSA via a shallow copy (see §5).
//   This is the ONE place S3 is NOT a verbatim copy of runLoop's publish closure.

// CRITICAL (publish loop is the CAS serialization point — FR-M7). update-ref MUST land in strict chain
//   order: commit[i] parent = prevSA (newSHA[i-1], or preRunHEAD/root for i=0); prevSA = newSHA after each.
//   Do NOT publish out of order. The CAS guard (UpdateRefCAS expected-old=prevSA) enforces it; mis-ordering
//   trips *generate.CASError.

// CRITICAL (no concurrency cap — FR-M14). Launch ALL N (non-skipped) messages. max_commits default 12
//   (FR-M4) bounds N. Do NOT add a semaphore / worker-pool / config key.

// CRITICAL (message goroutines are read-only w.r.t. index — git_primitives.md). generateMessage does
//   TreeDiff + RevParseHEAD (read-only); publishCommit does CommitTree (dangling) + UpdateRefCAS (ref).
//   NONE touch .git/index ⇒ concurrency is safe. Do NOT call Add/WriteTree(live)/ReadTree from a goroutine.

// GOTCHA (isRoot must use the FIRST PUBLISHED commit, not concept index 0). runLoop uses
//   `res.conceptIdx == 0 && isUnborn`; on the fast-path concept 0 can be FR-M8-skipped (S2), so the first
//   PUBLISHED concept may have sc.idx>0 yet still be the repo root (prevSA=="" on unborn). buildCommitResult
//   needs isRoot=true there or DiffTree fails to show the root commit's files. USE: isRoot := len(commits)==0 && isUnborn.

// GOTCHA (drainMsgs, not drainMsg, for the remaining slice). On ANY publish-loop error at concept i, drain
//   inflight[i+1:] (N-i-1 channels). drainMsg (single channel) is runLoop's — leave it byte-identical; add
//   drainMsgs([]chan msgOut) that loops drainMsg. Each goroutine sends exactly once to its buffered(1)
//   channel ⇒ every receive completes ⇒ no goroutine leak.

// GOTCHA (Verbose API). There is NO generic Verbose(msg). Use deps.Verbose.VerboseWarn(msg) guarded with
//   `if deps.Verbose != nil` (some tests pass nil). Optional: log a "launched N concurrent messages" line.

// GOTCHA (deps.Out for rescue/CAS messages). Guard `if deps.Out != nil` before Fprintln (library-safe;
//   runLoop does). Tests capture via dcmOutBuffer → *bytes.Buffer.

// GOTCHA (len(staged)==0 early return). If every concept was FR-M8-skipped, staged is empty → no commits.
//   Return (nil, nil, nil) — Decompose's arbiter gate `if len(commits) > 0` handles the empty case.

// SCOPE: S3 completes runLoopFastPath's concurrent phase + adds drainMsgs + focused tests. Do NOT touch
//   runLoop/drainMsg/Decompose/arbiter.go/chain.go (post-loop code is shared + unchanged), do NOT wire the
//   dispatch (S4), do NOT add the comprehensive suite (S5) or docs (S6), do NOT reimplement
//   generateMessage/publishCommit/buildCommitResult/the signal API, do NOT add a concurrency cap.
```

## Implementation Blueprint

### Data models and structure

No new types. S3 consumes S2's `stagedConcept` and reuses `msgOut`, `CommitResult`, `ChainEntry`,
`DecomposeRescueError` unchanged. The one new symbol is the `drainMsgs` helper.

```go
// stagedConcept (S2 — UNCHANGED, the handoff contract S3 consumes):
type stagedConcept struct {
	idx      int    // original concept index (for ordering / messages / concepts[idx].Title)
	tree     string // frozen write-tree SHA for concept i
	prevTree string // tree it was staged on top of (tree[i-1], or baseTree for the first) — message diff base
}
```

### Implementation Tasks (ordered by dependencies)

> **Prerequisite — S1 LANDED** (commit 3d18f57: `isFileDisjoint`). **S2 is the CONTRACT** (parallel/"Ready"):
> assume `stagedConcept` + `runLoopFastPath`'s sweep + the sentinel exist exactly as S2's PRP specifies. S3
> does NOT touch `isFileDisjoint` (S4 wires it), `runLoop`, `drainMsg`, `Decompose`, `arbiter.go`, `chain.go`.
> ANCHOR ON CONSTRUCTS via grep, not line numbers (S2 may have shifted runLoopFastPath's location).

```yaml
Task 1: ADD drainMsgs in internal/decompose/decompose.go (sibling to drainMsg)
  - PLACE: immediately AFTER drainMsg (co-locate the two drain helpers).
  - BODY:
        // drainMsgs receives-and-discards a slice of buffered(1) message channels to avoid goroutine leaks
        // when the FR-M14 concurrent fast-path publish loop aborts with N-i-1 messages still in flight
        // (P1.M1.T1.S3). Each goroutine sends exactly once to its buffered channel before exiting, so each
        // receive completes. nil-safe per channel (drainMsg guards nil). This is the ONLY behavioral
        // difference from runLoop's single-channel drainMsg: it covers a slice. drainMsg (single channel)
        // stays for runLoop — runLoop is byte-identical (the shared-file fallback).
        func drainMsgs(chs []chan msgOut) {
            for _, ch := range chs {
                drainMsg(ch)
            }
        }
  - DEPENDENCIES: none (drainMsg already exists).

Task 2: COMPLETE runLoopFastPath's concurrent phase (REPLACE S2's sentinel)
  - EDIT: in runLoopFastPath, DELETE S2's sentinel block:
        // --- P1.M1.T1.S3 inserts the concurrent message generation + CAS-ordered publish here ---
        ...
        return nil, nil, fmt.Errorf("%w: runLoopFastPath concurrent phase not yet implemented ...", ...)
    and REPLACE it with the concurrent phase below. The sweep above it (prevTree/tStartPaths/the for-loop
    building []staged) is UNCHANGED. `commits`/`chainData` (declared by S2 at the top) are reused.
  - BODY (the concurrent phase — mirrors runLoop's launch/publish, adapted per §5/§6/§8):
        // --- P1.M1.T1.S3: FR-M14 concurrent message generation + FR-M7 CAS-ordered publish + FR-M12 isolation ---

        // Nothing to publish (every concept FR-M8-skipped): return empty — Decompose's arbiter gate
        // guards len(commits) > 0.
        if len(staged) == 0 {
            return commits, chainData, nil
        }

        // FR-M14: launch ALL N (non-skipped) message generations CONCURRENTLY. Safe: each goroutine calls
        // generateMessage, which reasons over a tree-to-tree diff (read-only tree reads) and never touches
        // the live .git/index (git_primitives.md). No cap (FR-M14; max_commits default 12 bounds N).
        launch := func(i int, treeA, treeB string) chan msgOut {
            ch := make(chan msgOut, 1) // buffered(1) — goroutine sends once + exits; never blocks
            go func() {
                m, e := generateMessage(ctx, deps, treeA, treeB)
                ch <- msgOut{conceptIdx: i, treeA: treeA, treeB: treeB, msg: m, err: e}
            }()
            return ch
        }
        inflight := make([]chan msgOut, len(staged))
        for i, sc := range staged {
            inflight[i] = launch(sc.idx, sc.prevTree, sc.tree)
        }
        if deps.Verbose != nil {
            deps.Verbose.VerboseWarn(fmt.Sprintf("fast-path: launched %d concurrent message generations", len(staged)))
        }

        // FR-M7: PUBLISH STRICTLY IN CAS ORDER (concept order). The publish loop is the serialization
        // point: commit[i] parent = prevSHA (preRunHEAD/root for i=0, newSHA[i-1] otherwise); each CAS
        // requires HEAD == prevSHA. prevSHA is AUTHORITATIVE for the rescue parent (see §5 — runLoop's
        // 1-deep overlap guarantees it at launch; the concurrent path knows it only at publish time, so
        // arm signal + fix the rescue parent HERE, in this serial loop).
        prevSHA := preRunHEAD
        for i, ch := range inflight {
            sc := staged[i]
            // Arm rescue for concept i with the AUTHORITATIVE parent. Serial loop ⇒ no race on the single
            // global snapshot; prevSHA is newSHA[i-1] (commit[i-1] just published). (runLoop arms at launch
            // under its 1-deep overlap; that invariant does not hold here.)
            signal.SetSnapshot(sc.tree, prevSHA, "")
            res := <-ch
            signal.ClearSnapshot()

            if res.err != nil {
                var re *generate.RescueError
                if errors.As(res.err, &re) {
                    // FR-M12a: message[i] failed → rescue for concept i ONLY. Commits 0..i-1 already
                    // published (they stand). §5: re.ParentSHA may be STALE (generateMessage captured it
                    // via RevParseHEAD during concurrent generation, possibly before commit[i-1] landed) —
                    // prevSHA is authoritative. Fix a shallow copy so the printed recipe AND the
                    // DecomposeRescueError.Rescue both carry the correct parent. Exit-code mapping checks
                    // Kind, not ParentSHA, so it is unaffected.
                    title := ""
                    if sc.idx < len(concepts) {
                        title = concepts[sc.idx].Title
                    }
                    fixed := *re
                    fixed.ParentSHA = prevSHA
                    if deps.Out != nil {
                        fmt.Fprintln(deps.Out, generate.FormatRescueMulti(fixed.TreeSHA, fixed.ParentSHA, fixed.Candidate, title, sc.idx, len(concepts)))
                    }
                    drainMsgs(inflight[i+1:]) // drain the N-i-1 still-in-flight channels (no leak)
                    return commits, nil, &DecomposeRescueError{Rescue: &fixed, ConceptTitle: title, Index: sc.idx, Count: len(concepts), Commits: commits}
                }
                drainMsgs(inflight[i+1:])
                return commits, nil, res.err // HARD (ErrMessageFailed-wrapped infra) — propagate
            }

            // Publish in CAS order: parent = prevSHA (CAS expected-old). publishCommit runs hooks +
            // CommitTree (dangling) + UpdateRefCAS — touches NO index.
            newSHA, err := publishCommit(ctx, deps, res.treeB, prevSHA, res.msg)
            if err != nil {
                var ce *generate.CASError
                if errors.As(err, &ce) {
                    // FR-M12b: CAS failed → §13.5 message (ce.Error() has tree[i] recovery). Prior commits stand.
                    if deps.Out != nil {
                        fmt.Fprintln(deps.Out, ce.Error())
                    }
                    drainMsgs(inflight[i+1:])
                    return commits, nil, ce // partial; DecomposeResult.Commits = commits (0..i-1)
                }
                drainMsgs(inflight[i+1:])
                return commits, nil, err // HARD (ErrPublicationFailed-wrapped CommitTree)
            }

            // §6: isRoot = the FIRST published commit on an unborn repo (concept 0 may be FR-M8-skipped,
            // so sc.idx is unreliable; len(commits)==0 is authoritative). Correct for commits 2..N too.
            isRoot := len(commits) == 0 && isUnborn
            cr, bErr := buildCommitResult(ctx, deps, newSHA, res.msg, isRoot)
            if bErr != nil {
                drainMsgs(inflight[i+1:])
                return commits, nil, fmt.Errorf("%w: diff-tree[%d]: %w", ErrDecomposeFailed, sc.idx, bErr)
            }
            commits = append(commits, cr)
            chainData = append(chainData, ChainEntry{SHA: newSHA, Tree: res.treeB, Message: res.msg, Parent: prevSHA})
            prevSHA = newSHA
        }
        return commits, chainData, nil
  - NOTE: `concepts` (the function param) is in scope — used for `concepts[sc.idx].Title` + `len(concepts)`,
    exactly as runLoop uses `concepts[res.conceptIdx].Title`. `preRunHEAD` + `isUnborn` (S2 declared them
    unused; S3 now uses both) are the function params. Confirm `go build`/`make lint` are clean — S3 removes
    S2's "unused param" concern entirely.
  - DEPENDENCIES: Task 1 (drainMsgs); S2's sweep + stagedConcept + the sentinel to remove.

Task 3: ADD focused tests in internal/decompose/decompose_test.go
  - PLACE: near S2's TestRunLoopFastPath_Sweep (group the fast-path tests). The comprehensive suite is S5.
  - CALL runLoopFastPath DIRECTLY (S4 dispatch not wired yet) — mirror S2's test setup idiom (real git repo
    via dcmInitRepo/dcmWriteFile; baseTree/tStart via RevParseTree/FreezeWorkingTree; preRunHEAD/isUnborn
    via RevParseHEAD; Roles.Message via dcmMessageScriptManifest; disjoint concepts).
  - TEST A — TestRunLoopFastPath_ConcurrentPublish (happy path + CAS order + concurrency):
        // 3 disjoint files (a.go/b.go/c.go) modified after a base commit.
        // messageM := dcmMessageScriptManifest(t, bin, []string{"feat: a","feat: b","feat: c"})
        // messageM.Env["STAGECOACH_STUB_SLEEP_MS"] = "150"   // concurrency observable
        // concepts := []prompt.PlannerCommit{{Title:"c1",Files:[]string{"a.go"}}, {Title:"c2",...b.go}, {Title:"c3",...c.go}}
        // start := time.Now()
        // commits, chainData, err := runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)
        // elapsed := time.Since(start)
        // ASSERT err==nil; len(commits)==3; len(chainData)==3.
        // CAS ORDER: for i, git rev-parse <commits[i].SHA>^ == (i==0 ? preRunHEAD : commits[i-1].SHA).
        //   HEAD == commits[2].SHA. chainData[i].SHA==commits[i].SHA; chainData[i].Parent==(i==0?preRunHEAD:commits[i-1].SHA).
        // CONCURRENCY: elapsed < 3*150ms (generous CI slack — mirror TestDecompose_Overlap's 350ms-style
        //   threshold; if a regression made it serial, elapsed ≈ 450ms). Log a warning, don't hard-fail
        //   on CI timing jitter (the CAS-order assertion is the hard correctness gate).
  - TEST B — TestRunLoopFastPath_RescueIsolation (FR-M12a + no-leak drain):
        // 3 disjoint files. message script: concept 0 success ("feat: a"), concept 1 EMPTY (parse-fail →
        //   RescueError with cfg.MaxDuplicateRetries=0), concept 2 would-be (not reached).
        // cfg.MaxDuplicateRetries = 0
        // deps, buf := dcmOutBuffer(t, repo, roles)   // capture FormatRescueMulti on deps.Out
        // commits, _, err := runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)
        // ASSERT err != nil; errors.As(&dre *DecomposeRescueError); errors.As(&re *generate.RescueError);
        //   errors.Is(generate.ErrRescue). dre.Index == 1 (concept 1 failed). dre.Count == 3.
        // PARTIAL STAND: len(commits)==1 (concept 0); HEAD == commits[0].SHA (concept 1 never published).
        // NO LEAK: the call returned (concept 2's goroutine was drained). Optionally assert
        //   runtime.NumGoroutine() delta ≈ 0 across the call (drainMsgs received concept 2's buffered send).
        // RESCUE PARENT CORRECT: buf contains FormatRescueMulti; the recipe's -p parent == commits[0].SHA
        //   (the AUTHORITATIVE prevSHA), NOT preRunHEAD — this is the §5 fix's proof. (Parse buf or assert
        //   it contains commits[0].SHA and does NOT contain preRunHEAD in the -p position.)
  - NOTE: confirm the exact setup helpers (dcmInitRepo, dcmWriteFile, dcmMessageScriptManifest, dcmDeps/
    dcmOutBuffer, dcmShaResolves, stubtest.Build) via the existing TestDecompose_* tests. If
    FreezeWorkingTree is awkward to drive directly, mirror how an existing test derives baseTree/tStart
    (the PRIMARY assertions — CAS order + rescue parent — are robust to setup specifics).
  - DEPENDENCIES: Task 1 + Task 2.

Task 4: VERIFY runLoop/drainMsg byte-identical + gates
  - go build ./...
  - go vet ./...
  - gofmt -l internal/decompose/
  - go test ./internal/decompose/ -run 'Decompose|RunLoopFastPath' -race -v   # -race is mandatory (first concurrent path)
  - go test ./internal/decompose/... -race
  - make test && make lint
```

### Implementation Patterns & Key Details

```go
// PATTERN: launch() closure — VERBATIM mirror of runLoop's (buffered(1) chan msgOut; goroutine sends once)
launch := func(i int, treeA, treeB string) chan msgOut {
    ch := make(chan msgOut, 1)
    go func() {
        m, e := generateMessage(ctx, deps, treeA, treeB)
        ch <- msgOut{conceptIdx: i, treeA: treeA, treeB: treeB, msg: m, err: e}
    }()
    return ch
}

// PATTERN: the serial publish loop (FR-M7 CAS order; §5 signal-arm + rescue-parent fix; §6 isRoot)
prevSHA := preRunHEAD
for i, ch := range inflight {
    sc := staged[i]
    signal.SetSnapshot(sc.tree, prevSHA, "") // arm with AUTHORITATIVE prevSA (serial ⇒ no race)
    res := <-ch
    signal.ClearSnapshot()
    if res.err != nil {
        var re *generate.RescueError
        if errors.As(res.err, &re) {
            fixed := *re; fixed.ParentSHA = prevSHA   // §5: override stale capture
            /* FormatRescueMulti(fixed.TreeSHA, fixed.ParentSHA, fixed.Candidate, title, sc.idx, len(concepts)) */
            drainMsgs(inflight[i+1:])
            return commits, nil, &DecomposeRescueError{Rescue: &fixed, ConceptTitle: title, Index: sc.idx, Count: len(concepts), Commits: commits}
        }
        drainMsgs(inflight[i+1:]); return commits, nil, res.err
    }
    newSHA, err := publishCommit(ctx, deps, res.treeB, prevSHA, res.msg)
    if err != nil {
        var ce *generate.CASError
        if errors.As(err, &ce) { /* Fprintln ce.Error() */ drainMsgs(inflight[i+1:]); return commits, nil, ce }
        drainMsgs(inflight[i+1:]); return commits, nil, err
    }
    isRoot := len(commits) == 0 && isUnborn   // §6: first published commit on unborn repo
    cr, _ := buildCommitResult(ctx, deps, newSHA, res.msg, isRoot)
    commits = append(commits, cr)
    chainData = append(chainData, ChainEntry{SHA: newSHA, Tree: res.treeB, Message: res.msg, Parent: prevSHA})
    prevSHA = newSHA
}
return commits, chainData, nil

// PATTERN: error wrapping (mirror runLoop's ErrDecomposeFailed idiom for the diff-tree infra path)
fmt.Errorf("%w: diff-tree[%d]: %w", ErrDecomposeFailed, sc.idx, bErr)
```

### Integration Points

```yaml
NO struct/schema/API/config changes. One function completed + one helper + focused tests.

CODE:
  - internal/decompose/decompose.go — runLoopFastPath concurrent phase (replaces S2 sentinel) + drainMsgs
TESTS:
  - internal/decompose/decompose_test.go — +TestRunLoopFastPath_ConcurrentPublish + _RescueIsolation (focused; suite is S5)

CONSUMED (REUSE, unchanged):
  - generateMessage / publishCommit (internal/decompose/message.go)
  - buildCommitResult / drainMsg / msgOut / DecomposeRescueError / ErrDecomposeFailed (internal/decompose/decompose.go)
  - ChainEntry (internal/decompose/chain.go)
  - signal.SetSnapshot / ClearSnapshot (internal/signal/signal.go)
  - generate.RescueError / generate.CASError / generate.FormatRescueMulti (internal/generate/)
  - stagedConcept + runLoopFastPath's sweep (S2 — the CONTRACT)

SHARED + UNCHANGED (runLoopFastPath's return feeds it identically — do NOT touch):
  - Decompose post-loop arbiter gate (decompose.go:248-268), rereadFinalCommits, DecomposeResult assembly
  - arbiter.go / chain.go (resolveArbiter consumes commits + chainData unchanged)

DOWNSTREAM (do NOT implement in S3):
  - P1.M1.T1.S4: Decompose() dispatch — `if isFileDisjoint(out.Commits) { runLoopFastPath(...) } else { runLoop(...) }`
  - P1.M1.T1.S5: comprehensive fast-path regression suite (CAS-fail injection, freeze, all-skip, unborn, etc.)
  - P1.M1.T1.S6: docs/how-it-works.md fast-path paragraph

UNCHANGED: runLoop (byte-identical); drainMsg (byte-identical); Decompose; isFileDisjoint (S1);
  invokeStager/invokeStagerRetry; arbiter.go; chain.go; the Git interface; the signal package;
  generateMessage/publishCommit/buildCommitResult.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...                       # the concurrent phase + drainMsgs compile; S2's "unused param" concern is now resolved (preRunHEAD/isUnborn are used)
go vet ./...
gofmt -l internal/decompose/         # Expected: empty.
make lint                            # Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The focused concurrent + rescue tests (MANDATORY -race: this is the first concurrent path in decompose)
go test ./internal/decompose/ -run 'TestRunLoopFastPath_ConcurrentPublish|TestRunLoopFastPath_RescueIsolation' -race -v
# Expected: PASS — 3 commits in CAS order; concurrency observable; rescue isolates concept 1, partial stands, no leak, parent correct.

# S2's sweep test must still pass (S3 only replaces the sentinel — but the sentinel is GONE now, so S2's
# TestRunLoopFastPath_Sweep assertion of the sentinel MUST be UPDATED by S3: the sweep no longer returns the
# sentinel; it now publishes. COORDINATE: either S2's test is updated to expect the full happy-path result,
# or (if S2 hasn't landed yet) S2's test as written would now fail — REPLACE its sentinel assertion with the
# happy-path assertion (err==nil, len(commits)==N). See Task 3 TEST A.
go test ./internal/decompose/ -run TestRunLoopFastPath_Sweep -race -v

# Existing runLoop/Decompose tests must pass UNCHANGED (runLoop byte-identical)
go test ./internal/decompose/ -run TestDecompose -race -v

# Full decompose package (race)
go test ./internal/decompose/... -race -v

# Whole suite (race)
make test
# Expected: ALL pass.
```

> **NOTE on S2's sentinel test.** S2's `TestRunLoopFastPath_Sweep` asserts the sentinel error. Once S3 removes
> the sentinel, that assertion fails. S3 owns the transition: update S2's test to assert the full happy-path
> result (err==nil, len(commits)==N, CAS order) — OR fold it into Test A. Do NOT leave a test asserting a
> sentinel that no longer exists. (If S2 has not yet landed when S3 starts, write the happy-path test fresh.)

### Level 3: Integration Testing (System Validation)

```bash
# (S3's fast-path is not yet wired into Decompose — S4 does the dispatch. The within-scope proof is the
#  focused unit tests above: CAS-ordered publish + concurrency + FR-M12 isolation, under -race. The full
#  FR-M13/FR-M14 e2e [disjoint tree → git-add staging → concurrent messages → N commits → arbiter skip]
#  is validated end-to-end in S4 [dispatch] + S5 [suite], not S3.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: drainMsgs exists + reuses drainMsg (runLoop's drainMsg byte-identical)
grep -n "func drainMsgs" internal/decompose/decompose.go
# Expected: 1 match (the new helper). Confirm drainMsg itself is unchanged (git diff shows no - lines in drainMsg).

# Grep guard: the S2 sentinel is GONE
grep -n "runLoopFastPath concurrent phase not yet implemented" internal/decompose/decompose.go
# Expected: 0 matches (S3 removed it).

# Grep guard: the rescue parent uses prevSA (the §5 fix), not re.ParentSHA verbatim
grep -n "fixed.ParentSHA = prevSHA" internal/decompose/decompose.go
# Expected: 1 match (inside runLoopFastPath's RescueError branch).

# Grep guard: signal arms in the publish loop with authoritative prevSA (not at launch with preRunHEAD)
grep -n 'signal.SetSnapshot' internal/decompose/decompose.go
# Expected: present in runLoop (unchanged) AND runLoopFastPath (the publish-loop arm). NOT a launch-loop arm.

# Grep guard: no concurrency cap / semaphore / worker-pool added (FR-M14 — launch all N)
grep -n 'semaphore\|errgroup\|WorkerPool\|MaxConcurr' internal/decompose/decompose.go
# Expected: 0 matches.

# Grep guard: isRoot uses the robust form (§6)
grep -n 'isRoot := len(commits) == 0 && isUnborn' internal/decompose/decompose.go
# Expected: 1 match inside runLoopFastPath.

# Race guard: the focused tests ran under -race (re-run explicitly)
go test ./internal/decompose/ -run 'TestRunLoopFastPath' -race -count=1 -v
# Expected: PASS, zero race reports.

# Scope-boundary guard: runLoop + drainMsg + Decompose byte-identical (the diff is purely additive + the
# runLoopFastPath sentinel removal)
git diff internal/decompose/decompose.go | grep -E '^-' | grep -v '^---'
# Expected: ONLY the S2 sentinel block's deletion (the `return nil, nil, fmt.Errorf(... sentinel ...)` line
#   + its surrounding comment). NO - lines inside runLoop, drainMsg, Decompose, buildCommitResult, etc.

# Scope-boundary guard: only decompose.go + decompose_test.go changed
git diff --name-only
# Expected: only internal/decompose/decompose.go + internal/decompose/decompose_test.go.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/decompose/` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass
- [ ] Focused tests pass under `-race` with ZERO race reports

### Feature Validation
- [ ] S2's sentinel is REMOVED; the concurrent phase runs after the sweep
- [ ] All N (non-skipped) messages launch concurrently (buffered(1) channels)
- [ ] Publish consumes in concept order; `commit[i]` parent = `prevSHA`; `prevSHA = newSHA` after each (FR-M7)
- [ ] `signal.SetSnapshot(sc.tree, prevSHA, "")` arms in the SERIAL publish loop; `ClearSnapshot()` after each drain
- [ ] Rescue parent = authoritative `prevSHA` (shallow-copy fix), NOT `re.ParentSHA` (§5)
- [ ] FR-M12a `*RescueError` → FormatRescueMulti + `drainMsgs(inflight[i+1:])` + `*DecomposeRescueError` (Commits=0..i-1, they stand)
- [ ] FR-M12b `*CASError` → `ce.Error()` + drainMsgs + return ce (prior commits stand)
- [ ] `isRoot := len(commits) == 0 && isUnborn` (§6 — first published commit on unborn repo)
- [ ] `len(staged) == 0` → `(nil, nil, nil)`
- [ ] Returns `(commits, chainData, nil)` on full success; `(commits, nil, err)` on failure (partial stands)
- [ ] TEST A: 3 disjoint concepts → 3 commits in CAS order + concurrency observable
- [ ] TEST B: message[i] fails → DecomposeRescueError, concept 0 stands (HEAD==commits[0].SHA), no leak, rescue parent == commits[0].SHA

### Scope-Boundary Validation
- [ ] runLoop BYTE-IDENTICAL (no - lines inside it)
- [ ] drainMsg BYTE-IDENTICAL; drainMsgs added as a NEW sibling
- [ ] Decompose / arbiter.go / chain.go UNCHANGED (post-loop code shared)
- [ ] NO Decompose dispatch wiring / isFileDisjoint call (S4)
- [ ] NO comprehensive regression suite (S5)
- [ ] NO docs (S6)
- [ ] NO concurrency cap / semaphore / config key (FR-M14)
- [ ] NO reimplemented primitives (generateMessage/publishCommit/buildCommitResult/signal/Git iface)

### Code Quality
- [ ] runLoopFastPath's concurrent phase co-located with the sweep (S2) — the fast-path is one function
- [ ] Doc comments cite FR-M14/FR-M7/FR-M12 + the concurrency rationale + the §5 rescue-parent fix
- [ ] Error wrapping mirrors runLoop's ErrDecomposeFailed idiom (diff-tree path)
- [ ] `deps.Out` / `deps.Verbose` guarded for nil (library-safe)

---

## Anti-Patterns to Avoid

- ❌ Don't trust `re.ParentSHA` under concurrency — `generateMessage` captured it via `RevParseHEAD` during
  concurrent generation, possibly before `commit[i-1]` landed, so for i≥1 it can be stale (`preRunHEAD`).
  Override it to the publish loop's authoritative `prevSHA` (shallow copy). This is the §5 fix — the ONE
  place S3 is not a verbatim copy of runLoop's publish closure.
- ❌ Don't arm `signal.SetSnapshot` at concurrent LAUNCH time — (a) the single global snapshot would race
  if armed from goroutines, and (b) `prevSHA` is `preRunHEAD` for every concept at launch (nothing published).
  Arm it in the SERIAL publish loop where `prevSHA == newSHA[i-1]` is authoritative.
- ❌ Don't compute `isRoot := sc.idx == 0 && isUnborn` — concept 0 can be FR-M8-skipped (S2), so the first
  PUBLISHED concept may have `sc.idx > 0` yet still be the repo root. Use `len(commits) == 0 && isUnborn`.
- ❌ Don't add a concurrency cap / semaphore / worker-pool / config key — FR-M14 says "concurrently" with
  no cap; `max_commits` default 12 (FR-M4) bounds N. Launch all N.
- ❌ Don't touch the live `.git/index` from a message goroutine — `generateMessage` is read-only (tree
  diffs); `publishCommit` does `CommitTree` (dangling) + `UpdateRefCAS` (ref). Add/WriteTree(live)/ReadTree
  mutate the index and MUST stay in the serial sweep (S2). Concurrency is message-only.
- ❌ Don't publish out of CAS order — FR-M7: each CAS requires HEAD == `newSHA[i-1]`; the publish loop is the
  serialization point. Consuming results in concept order satisfies it; out-of-order publish trips `*CASError`.
- ❌ Don't change `drainMsg` — it's runLoop's and must stay byte-identical. Add `drainMsgs([]chan msgOut)`.
- ❌ Don't edit runLoop / Decompose / arbiter.go / chain.go — the post-loop code is shared and unchanged.
- ❌ Don't reimplement `generateMessage`/`publishCommit`/`buildCommitResult`/the signal API — REUSE them.
- ❌ Don't leave S2's `TestRunLoopFastPath_Sweep` asserting the now-removed sentinel — update it to the
  happy-path result (or fold into TEST A). A test asserting a non-existent sentinel is a failure.
- ❌ Don't skip `-race` — this is the first concurrent path in `internal/decompose`. The race detector is
  the net for the §5/§7 reasoning.

---

## Confidence Score: 9/10

One-pass success is very high. The concurrent phase is a close mirror of runLoop's verified launch/publish
closures (the result-handling, the CAS chain, the ChainEntry accumulation, the DecomposeRescueError shape are
all reused verbatim), every consumed primitive's signature is confirmed with line numbers, and the
concurrency model is explicitly bounded (message goroutines read-only w.r.t. index; publish loop serializes
CAS). The two genuine subtleties — the stale rescue `parentSHA` under concurrency (§5) and `isRoot` for a
skipped concept 0 (§6) — are both identified with the exact fix and a test that proves it (TEST B asserts the
rescue parent == commits[0].SHA, not preRunHEAD). The -1 is for the focused test's baseTree/tStart setup:
calling runLoopFastPath directly requires replicating Decompose's internal FreezeWorkingTree steps (heavier
than the top-level Decompose tests), and the concurrency-timing assertion is inherently CI-jitter-sensitive
(hence "log a warning, don't hard-fail"). Mitigated by (a) the PRIMARY correctness gate (CAS order + rescue
parent + partial-stands) being robust to setup details, (b) the existing `TestDecompose_Overlap` /
`TestDecompose_MessageRescuePartial` tests providing a copyable harness idiom, and (c) the explicit note to
update S2's sentinel test (the one cross-subtask coordination point). The `fixed := *re; fixed.ParentSHA =
prevSHA` shallow-copy is the single non-obvious line — it is spelled out in full.