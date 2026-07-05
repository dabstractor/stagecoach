# P1.M1.T3.S2 — Arbiter folds only T_start content: Research & Validation Notes

> Research backing the PRP. All facts verified against the live source on this box (HEAD = 26fcf0b).
> The freeze-safe arbiter (P1.M1.T2.S1) and the S1 acceptance tests (P1.M1.T3.S1) are BOTH LANDED.

## 0. Scope boundary vs the sibling S1 (the fence — read first)

| Concern | S1 (P1.M1.T3.S1) — LANDED | S2 (this task) |
|---|---|---|
| Frozen leftover | EMPTY (`DiffTreeNames(tipTree, tStart) == []`) | NON-EMPTY (a legit leftover exists) |
| Arbiter | SKIPPED (gate short-circuits) | RUNS (null/tip/mid paths exercised) |
| Integration tests | `TestDecompose_ConcurrentChangeExclusion` (upgrade) + `TestDecompose_TStartCompleteness` (add) | `TestDecompose_ArbiterFoldsOnlyTStart` (add) — concept-1 no-op ⇒ b.go unclaimed ⇒ arbiter folds it |
| Unit tests (chain_test.go) | (none — S1 is integration-only) | `TestResolveArbiter_FreezeParitySentinelExcluded` (add) — direct resolveArbiter call × {nil, &tip, &sha1} with a post-freeze sentinel.go |

**S1 is LANDED on disk** (verified): `decompose_test.go:848` shows the trimmed 2-entry message script
in `TestDecompose_ConcurrentChangeExclusion`; `TestDecompose_TStartCompleteness` exists at
`decompose_test.go:913`. This task builds on that state — it does NOT touch S1's tests.

## 1. What §5.2 asked for vs what already exists (the "already done" audit)

`arbiter_freeze_parity.md` §5.2 was written BEFORE the freeze-safe arbiter landed. Three of its four
asks are ALREADY DONE on disk; ONLY the sentinel proof remains:

| §5.2 ask | Status on disk | Evidence |
|---|---|---|
| "extend chnBuildChain (or add a variant) to capture tStart" | ✅ DONE | `chain_test.go` `chnBuildChain` returns `(commits, chainData, tStart string, leftoverPaths []string)` — it stages leftover.go, `write-tree`→tStart, `read-tree tree2` + `rm --cached --ignore-unmatch leftover.go` to restore a clean index == tree2. |
| "Call resolveArbiter(... tStart, leftoverPaths) directly with leftoverPaths = []string{\"leftover.go\"}" | ✅ DONE | Every `TestResolveArbiter_*` already calls the 7-arg `resolveArbiter(ctx, deps, target, commits, chainData, tStart, leftoverPaths)`. |
| "Update the existing TestResolveArbiter_* calls to the new 7-arg signature" | ✅ DONE | All six `TestResolveArbiter_*` + `TestResolveArbiter_CleanTreePostcondition` use the 7-arg form. |
| "write a post-freeze sentinel.go … assert HEAD^{tree} (and every rebuilt Cj'^{tree}) does NOT contain sentinel.go … sentinel.go untracked post-call … mid-chain rebuilt tip == T_start" | ❌ REMAINING | NO existing test writes a post-freeze sentinel in chain_test.go. THIS IS THE UNIT DELIVERABLE. |

**So this task = (a) a `chnBuildChainWithSentinel` variant + the freeze-parity unit proof, and (b) the
legitimate-leftover integration case.** No production-code change; no signature change; no edit to
existing `TestResolveArbiter_*` (they already pass on the 7-arg form).

## 2. The three resolution paths (chain.go) — what each commits, verified

`resolveArbiter` (chain.go:50) dispatches on `target`:

| target | path | func | treePrime | HEAD^{tree} after | commits after |
|---|---|---|---|---|---|
| `nil` / N==0 / not-found | A (new) | `resolveNewCommit` | `:= tStart` | **== tStart** | N+1 (arbiter adds a child of tip) |
| `&tipSHA` (idx==N-1) | B (amend) | `resolveTipAmend` | `:= tStart` | **== tStart** | N (tip amended in place) |
| `&sha_i` (idx<N-1) | C (mid) | `resolveMidChain` | `OverlayTreePaths(tree[j], tStart, leftoverPaths)` per j | **== tStart** (rebuilt tip) | N (C_i..C_{N-1} rebuilt; C0..C_{i-1} unchanged) |

**KEY INVARIANT (the thing this task proves): on ALL THREE paths HEAD^{tree} == tStart.** Paths A/B
set `treePrime := tStart` directly (chain.go — `resolveNewCommit`/`resolveTipAmend`, no AddAll/WriteTree).
Path C rebuilds with `OverlayTreePaths(tree[N-1], tStart, leftoverPaths)`; because leftoverPaths ==
`DiffTreeNames(tipTree, tStart)` (every differing path), overlaying them ALL onto tree[N-1] yields
exactly tStart. So `HEAD^{tree} == tStart` is a UNIFORM assertion across all three targets.

Because every committed tree is built from tStart + frozen tree[j] ONLY (never the live working tree),
a post-freeze `sentinel.go` (written after tStart capture, NOT in tStart, NOT in any tree[j]) CANNOT
appear in any commit. It remains untracked in the working tree (ReadTree(tStart) sets index = tStart,
which excludes sentinel.go).

## 3. The unit-level proof (chain_test.go) — design

### 3.1 `chnBuildChainWithSentinel` (NEW helper — a thin variant)

```go
// chnBuildChainWithSentinel is chnBuildChain PLUS a post-freeze sentinel.go (untracked, NOT in
// tStart). It exists to prove resolveArbiter builds every arbiter commit's tree from tStart only —
// the sentinel, written AFTER tStart was captured, can never be swept in (FR-M1d).
func chnBuildChainWithSentinel(t *testing.T, repo string) (commits []CommitInfo, chainData []ChainEntry, tStart string, leftoverPaths []string) {
    commits, chainData, tStart, leftoverPaths = chnBuildChain(t, repo)
    chnWriteFile(t, repo, "sentinel.go", "package sentinel\n") // post-freeze; UNSTAGED; NOT in tStart
    return commits, chainData, tStart, leftoverPaths
}
```

After `chnBuildChain`, the working tree holds untracked `leftover.go` (the legit frozen leftover — it
IS in tStart). The variant adds untracked `sentinel.go` (NOT in tStart). So post-arbiter `git status`
must show `?? sentinel.go` (leftover.go is absorbed into the index == tStart; sentinel.go is not).

### 3.2 `TestResolveArbiter_FreezeParitySentinelExcluded` (NEW test)

Table-driven over `{nil, &tipSHA, &sha1}`, **fresh repo per target** (resolveArbiter mutates HEAD —
calls cannot share a repo). For each target, after `resolveArbiter(...)`:

1. `HEAD^{tree} == tStart` (the "exactly T_start" proof — uniform across all three paths; for mid it
   is also the "rebuilt tip == T_start" proof).
2. `sentinel.go` in NO commit: `git log --name-only --format=` does not contain `sentinel.go` (covers
   loop commits + the arbiter commit +, for mid, every rebuilt Cj'^{tree}).
3. `sentinel.go` REMAINS untracked: `git status --porcelain` contains `?? sentinel.go`.
4. `leftover.go` DID land (the legit frozen leftover was folded): `HEAD^{tree}` (via ls-tree) contains
   `leftover.go`. This distinguishes "arbiter folded the frozen leftover" from "arbiter did nothing".

`chnDeps(t, repo, m)` suffices (only the Message role is used — by the null path's generateMessage;
tip/mid reuse msg verbatim and never call the agent). `m := stubtest.Manifest(bin,
stubtest.Options{Out: "chore: arbiter leftover"})`.

**Why fresh repo per target (not one repo × 3 calls):** resolveArbiter advances HEAD via UpdateRefCAS.
A second call's `tipSHA` (read from chainData, stale) would fail the CAS. Each subtest rebuilds the
chain from scratch — mirroring how every existing `TestResolveArbiter_*` builds its own repo.

## 4. The integration case (decompose_test.go) — design

### 4.1 Fixture: 2-concept run, concept-1 no-op ⇒ b.go unclaimed ⇒ arbiter folds it

Unborn repo (mirrors `TestDecompose_ArbiterWiring` / `_ConcurrentChangeExclusion`). Two dirty files
pre-run: `a.go`, `b.go` ⇒ `T_start = {a.go, b.go}`. Planner returns 2 concepts:
`[{"title":"add b","description":"b.go"},{"title":"add a","description":"a.go"}]`. The seam no-ops on
"add b" (FR-M8 empty-concept skip) and stages a.go on "add a". So the loop makes ONE commit ({a.go});
`tipTree = {a.go}`; `DiffTreeNames(tipTree, tStart) = {b.go}` ⇒ non-empty ⇒ arbiter RUNS. Arbiter
returns `{"target": null}` ⇒ resolveNewCommit ⇒ arbiter commit tree == tStart = {a.go, b.go}.

The seam ALSO writes `concurrent.txt` UNSTAGED post-freeze (via `concurrentSentinelSeam`, reused from
S1). `concurrent.txt` is NOT in tStart ⇒ cannot enter any commit ⇒ remains untracked.

### 4.2 Why `concurrentSentinelSeam` is the right seam (reuse, do NOT redeclare)

`concurrentSentinelSeam(t, repo, conceptFiles, sentinel)` (decompose_test.go:795) stages each concept's
files (like `dcmStagerSeam`) AND writes `sentinel` UNSTAGED on the FIRST concept. Passing
`map[string][]string{"add b": {}, "add a": {"a.go"}}` makes "add b" a no-op (empty slice ⇒ stages
nothing) and "add a" stage a.go. The sentinel (`concurrent.txt`) is written on the first concept
("add b") — post-freeze ⇒ excluded. **This is EXACTLY the seam S1 introduced; reuse it verbatim.**

### 4.3 The expected-T_start capture (the "exactly T_start" oracle)

Before the run, capture the tree of the two dirty files on the unborn base, then restore a clean index:
```go
dcmRunGit(t, repo, "add", "a.go", "b.go")
expectedTStart := dcmGitOut(t, repo, "write-tree")
dcmRunGit(t, repo, "rm", "--cached", "--ignore-unmatch", "a.go", "b.go") // restore clean index (FR-M1 trigger)
```
(This mirrors `chnBuildChain`'s `rm --cached --ignore-unmatch` pattern, which works on an unborn repo.)
`FreezeWorkingTree` captures `git add -A; write-tree` internally == this `expectedTStart` (the two
files are the entire working tree on the unborn base). After the run, assert
`dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}") == expectedTStart`.

### 4.4 Message script (loop + arbiter)

Concept-1 ("add b") is SKIPPED (FR-M8) ⇒ consumes NO message. Concept-2 ("add a") ⇒ message[0].
Arbiter null ⇒ resolveNewCommit ⇒ generateMessage ⇒ message[1]. So:
`dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: arbiter leftover"})` (+ one extra
entry for dedupe-retry safety, matching `TestDecompose_ArbiterWiring`'s defensive extras).

### 4.5 Assertions

- `HEAD^{tree} == expectedTStart` (arbiter commit's tree is EXACTLY T_start — contract: "each arbiter
  commit's tree (HEAD^{tree}) is exactly T_start").
- `concurrent.txt` in NO commit: `git log --name-only --format=` does not contain it.
- `concurrent.txt` REMAINS untracked: `dcmStatusPorcelain` contains `?? concurrent.txt`.
- Exactly 2 commits: `dcmLogCount == 2` (1 loop + 1 arbiter; unborn ⇒ no seed). A non-freeze-safe
  arbiter would ALSO sweep concurrent.txt into the arbiter commit (the bug this proves closed).

## 5. Reusable-helper inventory (do NOT redeclare)

**chain_test.go (package decompose):** `chnInitRepo`, `chnWriteFile`, `chnStageFile`, `chnCommitRaw`,
`chnRunGit`, `chnHeadSHA`, `chnDeps`, `chnBuildChain` (returns commits/chainData/tStart/leftoverPaths).
NEW: `chnBuildChainWithSentinel`.

**decompose_test.go (package decompose):** `dcm*` (InitRepo/WriteFile/StageFile/CommitRaw/RunGit/
GitOut/HeadSHA/LogCount/StatusPorcelain/PlannerManifest/MessageScriptManifest/ArbiterManifest/Deps),
`concurrentSentinelSeam` (:795), `tooledStubManifest` (stager_test.go:73), `stubtest.Build/Manifest`.

## 6. Decisions

- **D1 — variant over in-place extension.** `chnBuildChain` is shared by 7 existing `TestResolveArbiter_*`
  tests that assert `git status == ""` (clean). Writing a sentinel in-place would break all of them.
  Add `chnBuildChainWithSentinel` as a thin wrapper (calls chnBuildChain + writes sentinel.go).
- **D2 — `HEAD^{tree} == tStart` is UNIFORM across all three paths.** The contract phrases it as
  "exactly T_start (null/tip)" + "mid-chain rebuilt tip equals T_start" — but all three paths yield
  tree == tStart (§2). Assert it for every target; the mid case needs no special form.
- **D3 — `git log --name-only --format=` is the uniform "sentinel in no commit" oracle.** It covers
  loop commits + the arbiter commit + every rebuilt Cj'^{tree} (mid) in one check. Reused from
  `TestDecompose_StagerFreezeViolation`/`_ConcurrentChangeExclusion`.
- **D4 — fresh repo per target in the unit test.** resolveArbiter advances HEAD; a stale tipSHA on a
  2nd call would fail CAS. t.Run subtests, each rebuilding the chain.
- **D5 — `dcmLogCount` (not `result.Amended`) for the integration "arbiter ran" proof.** Amended is 0
  for null-path (the arbiter DID run but created a new commit, not an amend). `dcmLogCount == 2`
  (1 loop + 1 arbiter) proves the arbiter ran and folded the leftover. (Mirrors S1's G2.)
- **D6 — unborn repo for the integration case.** Mirrors `TestDecompose_ArbiterWiring` +
  `_ConcurrentChangeExclusion`; makes `expectedTStart` = the two dirty files (no seed to account for).
  S1's `_TStartCompleteness` is the born-repo variant; this task complements it with the unborn
  legitimate-leftover case.
- **D7 — tests PASS against the landed code (permanent regression net).** The freeze-safe arbiter is
  in. Do NOT chase a pre-fix failure; encode the post-fix invariant. (Mirrors S1's G11.)

## 7. Non-overlap / no-touch (scope discipline)

- NO production code (`chain.go`, `decompose.go`, `arbiter.go`, `git/*`) — test-only.
- NO edit to S1's tests (`TestDecompose_ConcurrentChangeExclusion`, `TestDecompose_TStartCompleteness`).
- NO edit to the existing `TestResolveArbiter_*` (they already pass on the 7-arg form).
- NO edit to `concurrentSentinelSeam` / `dcm*` / `chn*` helpers (reused verbatim; one NEW helper only).
- NO edit to `docs/*`, `README.md`, `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.
- Files touched: `internal/decompose/chain_test.go` (1 helper + 1 test) +
  `internal/decompose/decompose_test.go` (1 test). `git diff --stat` shows exactly those two.
