# PRP — P1.M1.T3.S2: Arbiter folds only T_start content — paired integration case + chain_test.go unit proof

---

## Goal

**Feature Goal**: Encode the **freeze-parity regression net** for the arbiter's three resolution paths
(new / tip-amend / mid-chain), proving that **every arbiter commit's tree is built strictly from
`T_start` (or an `OverlayTreePaths` overlay of frozen trees)** and that a working-tree file written
**after** `T_start` capture (a "sentinel" / "concurrent" file) is swept into **no** arbiter commit and
remains untouched in the working tree. Two paired proofs:

1. **Unit-level (chain_test.go)** — drive `resolveArbiter(...)` directly for `target ∈ {nil, &tipSHA, &sha1}`
   against a fixture that holds a post-freeze `sentinel.go`; assert `HEAD^{tree} == T_start` and
   `sentinel.go` is in no commit / remains untracked, for all three targets.
2. **Integration-level (decompose_test.go)** — a 2-concept `Decompose()` run where concept-1's stager
   is a no-op (empty-skip) so a **legitimate frozen leftover** exists (`b.go` unclaimed); the arbiter
   RUNS (null target) and folds `b.go` into an arbiter commit whose tree **== T_start**, while a
   post-freeze `concurrent.txt` is excluded from every commit and left untracked.

**Deliverable**: One new helper + one new test in `internal/decompose/chain_test.go`, and one new
test in `internal/decompose/decompose_test.go`. **No production code changes** — the freeze-safe
arbiter (`resolveArbiter` 7-arg signature) and `OverlayTreePaths` are already landed and passing
(P1.M1.T1.S1 + P1.M1.T2.S1). This task is the **acceptance proof** that those changes hold the
invariant permanently.

**Success Definition**: `go test -race ./internal/decompose/` passes (zero failures), the two new
tests assert the exact-T_start / sentinel-excluded invariants, and the existing S1 tests
(`TestDecompose_ConcurrentChangeExclusion`, `TestDecompose_TStartCompleteness`) and the six existing
`TestResolveArbiter_*` tests are untouched and still green.

---

## User Persona (if applicable)

N/A — this is an internal test artifact. The "user" is the future maintainer / regression suite. The
PRD names these as §20.2 "Property/invariant tests" (specifically "Start-of-run freeze" + "Arbiter
freeze parity") and §20.5 "End-to-end scenario harness" entries.

---

## Why

- **Closes the FR-M1d acceptance gap.** FR-M1d (P1.M1) rewrote the arbiter so its gate, diff, and
  staging all derive from frozen `T_start`/`tipTree` SHAs instead of the live working tree. S1
  (P1.M1.T3.S1, LANDED) proved the **empty-frozen-leftover** case (arbiter skipped). This task (S2)
  proves the **non-empty-frozen-leftover** case (arbiter RUNS and folds `T_start` only). Together
  they are the complete §20.2 "Arbiter freeze parity" proof.
- **The bug this guards against.** In v2.0–v2.1 the arbiter gate read live `git status --porcelain`
  and the resolution ran `git add -A` against the live tree, so a file written by a concurrent tool
  *during* the planner call was silently swept into an arbiter commit. This regression net makes any
  such reintroduction fail loudly.
- **Integration with existing features.** S2 is the success-path sibling of S1's
  `TestDecompose_ConcurrentChangeExclusion` (which has an *empty* leftover) and complements S1's
  `TestDecompose_TStartCompleteness` (born-repo, empty leftover) with the **unborn-repo,
  legitimate-leftover** case. The unit proof is the sibling of the six existing
  `TestResolveArbiter_*` tests, which assert loop-level correctness but do **not** write a post-freeze
  sentinel.

---

## What

Two test additions, both in `package decompose` (`internal/decompose`). Both assert the **same
three invariants** that S1 encoded, generalized to the arbiter-RUNS case:

1. **Sentinel in no commit** — `git log --name-only --format=` across the whole run (loop commits +
   arbiter commit +, for mid-chain, every rebuilt `Cj'` tree) does **not** contain the sentinel.
2. **Sentinel remains untracked** — `git status --porcelain` shows `?? <sentinel>` post-run.
3. **Arbiter tree == T_start** — `HEAD^{tree} == T_start` (uniform across all three paths: A/B set
   `treePrime := T_start`; C rebuilds the tip via `OverlayTreePaths(tree[N-1], T_start, leftoverPaths)`
   which equals `T_start`). For the integration case: the arbiter commit's tree is **exactly** the
   pre-run-captured `expectedTStart`.

### Success Criteria

- [ ] `chnBuildChainWithSentinel` helper exists in `chain_test.go` and wraps `chnBuildChain` + writes
      an unstaged `sentinel.go` (NOT in `T_start`).
- [ ] `TestResolveArbiter_FreezeParitySentinelExcluded` is table-driven over `{nil, &tipSHA, &sha1}`,
      fresh repo per target, and for each target asserts: (a) `HEAD^{tree} == tStart`, (b) `sentinel.go`
      in no commit, (c) `?? sentinel.go` in status, (d) `leftover.go` DID land in `HEAD^{tree}`.
- [ ] `TestDecompose_ArbiterFoldsOnlyTStart` drives `Decompose()` on an unborn repo with a no-op
      concept-1 stager so `b.go` is a legitimate leftover; asserts: (a) `HEAD^{tree} == expectedTStart`,
      (b) `concurrent.txt` in no commit, (c) `?? concurrent.txt` in status, (d) `dcmLogCount == 2`
      (1 loop + 1 arbiter).
- [ ] `git diff --stat` touches **exactly** `internal/decompose/chain_test.go` and
      `internal/decompose/decompose_test.go` — nothing else.

---

## All Needed Context

### Context Completeness Check

**Passes the "No Prior Knowledge" test.** All assertions, helper signatures, line numbers, and the
exact fixture shapes are specified below and were verified against the live source at HEAD. The
executing agent needs no further codebase spelunking — every referenced helper is named with its
definition line, and the two new tests are described at the assertion level.

### Documentation & References

```yaml
# MUST READ — the design + decisions for THIS task (authoritative)
- docfile: plan/008_82253c999440/P1M1T3S2/research/arbiter_folds_tstart_proof.md
  why: The complete design for this task: scope fence vs S1, the three resolution paths, the
       chnBuildChainWithSentinel variant, the unit-proof table, the integration fixture, the
       reusable-helper inventory, and 7 numbered decisions (D1–D7).
  critical: §0 (the S1-vs-S2 fence — read first so you do NOT duplicate S1's empty-leftover case);
            §1 (the "already done" audit — only the sentinel proof + legit-leftover case remain);
            §2 (why HEAD^{tree}==T_start is UNIFORM across all three paths); §3 (unit proof design);
            §4 (integration case design, incl. expectedTStart capture at §4.3 and message script §4.4).

# MUST READ — the freeze-parity architecture (the WHAT and WHY of FR-M1d)
- docfile: plan/008_82253c999440/docs/architecture/arbiter_freeze_parity.md
  why: Defines the invariant being tested, the three resolution paths, OverlayTreePaths semantics,
       and (§5.2) the exact original ask this task satisfies.
  critical: §5.2 is the contract for this task; §2 (TARGET state) explains why treePrime==T_start.

# PRD — the invariants being encoded (acceptance oracle)
- url: PRD.md §20.2 "Arbiter freeze parity (v2.2)" and "Start-of-run freeze (v2)"
  why: Wording of the invariants to assert verbatim in test comments.
  critical: "the arbiter commit's tree is exactly T_start (FR-M1d/M9/M10)" — the exact-T_start oracle.

# PATTERN FILES — the exact helpers/tests to reuse and mirror
- file: internal/decompose/chain_test.go
  why: Contains chnBuildChain (line 83), chnDeps (line 68), the chn* helpers, and the six
       existing TestResolveArbiter_* tests to mirror in style/signature.
  pattern: chnBuildChain returns (commits, chainData, tStart, leftoverPaths); every
           TestResolveArbiter_* calls the 7-arg resolveArbiter(ctx, deps, target, commits,
           chainData, tStart, leftoverPaths). The new variant + test slot in alongside these.
  gotcha: chnBuildChain leaves the index == tree2 (clean). Writing sentinel.go UNSTAGED (os.WriteFile,
          NO git add) keeps it OUT of tStart and OUT of the index — do NOT stage it.

- file: internal/decompose/decompose_test.go
  why: Contains concurrentSentinelSeam (line 795), dcmDeps, dcmLogCount, dcmStatusPorcelain,
       dcmMessageScriptManifest, dcmArbiterManifest, dcmGitOut, dcmRunGit, dcmLogOneline, and the
       two S1 tests to mirror (TestDecompose_ConcurrentChangeExclusion line 832,
       TestDecompose_TStartCompleteness line ~940).
  pattern: deps.stager = concurrentSentinelSeam(t, repo, map[string][]string{...}, "sentinel") overrides
           the stager seam; the seam stages each concept's files AND writes the sentinel UNSTAGED on the
           FIRST concept (post-freeze ⇒ excluded). An empty slice value = no-op concept (FR-M8 skip).
  gotcha: Reuse concurrentSentinelSeam VERBATIM — do NOT redeclare it. For the integration case pass
          map[string][]string{"add b": {}, "add a": {"a.go"}} so "add b" is the no-op that leaves b.go
          unclaimed. The sentinel must be "concurrent.txt" (item contract), not S1's "sentinel.txt".

- file: internal/decompose/chain.go
  why: The production code under test (read-only — do NOT modify). Confirms the 7-arg signature and
       the three paths' treePrime values.
  pattern: resolveArbiter (line 51) dispatches: nil/N==0/not-found → resolveNewCommit (treePrime:=tStart);
           &tipSHA → resolveTipAmend (treePrime:=tStart); &sha_i (idx<N-1) → resolveMidChain
           (treePrime=OverlayTreePaths(tree[j], tStart, leftoverPaths) per j). On ALL paths
           HEAD^{tree} ends == tStart + ReadTree(tStart) syncs the index.
  gotcha: resolveArbiter ADVANCES HEAD via UpdateRefCAS. A second call on the same repo would read a
          STALE tipSHA from chainData and fail the CAS. ⇒ the unit test MUST use a fresh repo per target.
```

### Current Codebase tree (the two files touched)

```bash
internal/decompose/
├── chain.go              # resolveArbiter + 3 paths (UNDER TEST — read-only)
├── chain_test.go         # chn* helpers + TestResolveArbiter_*  ← ADD helper + 1 test
├── decompose.go          # gate + runArbiterPhase (UNDER TEST — read-only)
└── decompose_test.go     # dcm* helpers + concurrentSentinelSeam + S1 tests  ← ADD 1 test
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
internal/decompose/
├── chain_test.go         # MODIFIED: + chnBuildChainWithSentinel (helper)
│                         #           + TestResolveArbiter_FreezeParitySentinelExcluded (unit proof)
└── decompose_test.go     # MODIFIED: + TestDecompose_ArbiterFoldsOnlyTStart (integration proof)
# (no new files; no production-code edits)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: resolveArbiter advances HEAD. The unit test MUST use a fresh repo per target
// (t.Run subtests, each calling chnBuildChainWithSentinel). A shared repo fails CAS on call #2.

// CRITICAL: the sentinel MUST be written UNSTAGED (os.WriteFile / chnWriteFile with NO git add).
// chnBuildChain already restored index == tree2 (clean) and tStart = tree2 + leftover.go.
// Writing sentinel.go as an untracked file keeps it OUT of tStart AND out of the index, so:
//   - it cannot enter any commit (arbiter builds trees from tStart/tree[j] only); and
//   - it remains "?? sentinel.go" in git status post-call (ReadTree(tStart) sets index == tStart).

// CRITICAL: HEAD^{tree} == tStart is UNIFORM across all three targets. The contract phrases
// "exactly T_start (null/tip)" + "mid-chain rebuilt tip == T_start" separately, but resolveMidChain's
// final j overlays leftoverPaths (== every differing path) onto tree[N-1] ⇒ exactly tStart.
// Assert the SAME form for every target; the mid case needs no special assertion.

// GOTCHA: concept-1 ("add b") in the integration case is SKIPPED per FR-M8 (empty stager slice).
// It consumes NO message. The message script order is therefore: message[0] → "add a" (concept-2),
// message[1] → arbiter resolveNewCommit. NOT message[0]→"add b". Getting this wrong makes the
// stub run out of responses / mismatch subjects.

// GOTCHA: the integration repo is UNBORN (no initial commit), mirroring S1's
// TestDecompose_ConcurrentChangeExclusion + TestDecompose_ArbiterWiring. On an unborn repo the two
// dirty files (a.go, b.go) ARE the entire working tree, so FreezeWorkingTree's internal
// `git add -A; write-tree` == the hand-built `git add a.go b.go; write-tree` (expectedTStart).

// GOTCHA: reuse concurrentSentinelSeam VERBATIM (decompose_test.go:795). Do NOT redeclare it and
// do NOT edit it. Its only behavior this task relies on: empty slice value ⇒ no-op (stages nothing),
// and it writes the sentinel UNSTAGED on the first concept processed.

// GOTCHA: the message-script stub pops responses in call order. Provide exactly the calls that fire
// (concept-2 loop message + arbiter null-path message) plus one defensive extra, matching
// TestDecompose_ArbiterWiring's defensive-extras convention.
```

---

## Implementation Blueprint

### Data models and structure

No new data models. The test reuses the existing `CommitInfo` / `ChainEntry` types and the `Deps` /
`RoleManifests` structs already used by every `TestResolveArbiter_*` and `TestDecompose_*` test. The
only "structure" added is the table of targets for the unit test:

```go
// Per-target unit subtest. Fresh repo each (resolveArbiter advances HEAD via UpdateRefCAS).
targets := []struct {
    name   string
    target func(commits []CommitInfo) *string
}{
    {"null",    func(_ []CommitInfo) *string { return nil }},                   // path A (new commit)
    {"tip",     func(c []CommitInfo) *string { sha := c[2].SHA; return &sha }}, // path B (tip amend)
    {"mid",     func(c []CommitInfo) *string { sha := c[1].SHA; return &sha }}, // path C (mid rebuild)
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD helper chnBuildChainWithSentinel to internal/decompose/chain_test.go
  - IMPLEMENT: a thin wrapper that calls chnBuildChain(t, repo), then writes sentinel.go UNSTAGED.
  - SIGNATURE: func chnBuildChainWithSentinel(t *testing.T, repo string) (commits []CommitInfo,
               chainData []ChainEntry, tStart string, leftoverPaths []string)
  - BODY:
      commits, chainData, tStart, leftoverPaths = chnBuildChain(t, repo)
      chnWriteFile(t, repo, "sentinel.go", "package sentinel\n") // UNSTAGED; NOT in tStart
      return commits, chainData, tStart, leftoverPaths
  - FOLLOW pattern: chnBuildChain (chain_test.go:83) — same return tuple shape; chnWriteFile
                   (chain_test.go:30) for the unstaged write (NO chnStageFile).
  - NAMING: chnBuildChainWithSentinel (mirrors the chn* + chnBuildChain family).
  - PLACEMENT: immediately after chnBuildChain (chain_test.go, ~line 135, before "// --- Tests ---").
  - GOTCHA: do NOT stage sentinel.go. After chnBuildChain the index == tree2 (clean) and tStart already
            captured; an unstaged sentinel.go is outside tStart by construction.
  - WHY a variant, not in-place (Decision D1): chnBuildChain is shared by 7 existing tests that assert
            `git status == ""` (clean). Writing a sentinel in-place would break all of them.

Task 2: ADD TestResolveArbiter_FreezeParitySentinelExcluded to internal/decompose/chain_test.go
  - IMPLEMENT: table-driven test over targets {nil, &tipSHA(=commits[2].SHA), &sha1(=commits[1].SHA)},
               fresh repo per target via t.Run subtests.
  - PER TARGET:
      1. repo := t.TempDir(); chnInitRepo(t, repo)
      2. commits, chainData, tStart, leftoverPaths := chnBuildChainWithSentinel(t, repo)
      3. bin := stubtest.Build(t)
         m := stubtest.Manifest(bin, stubtest.Options{Out: "chore: arbiter leftover"})
         deps := chnDeps(t, repo, m)  // only Message role is exercised (null path generateMessage)
      4. target := <per-table>
         err := resolveArbiter(context.Background(), deps, target, commits, chainData, tStart, leftoverPaths)
         assert err == nil
      5. ASSERTIONS (use chnRunGit / dcm-style oracle reads; the chn* helpers already wrap git):
         a. HEAD^{tree} == tStart:  got := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}"); got == tStart
         b. sentinel.go in NO commit:
              names := chnRunGit(t, repo, "log", "--name-only", "--format=")
              !strings.Contains(names, "sentinel.go")
            (covers loop commits + arbiter commit + every rebuilt Cj' tree for mid)
         c. sentinel.go REMAINS untracked:
              status := chnRunGit(t, repo, "status", "--porcelain")
              strings.Contains(status, "?? sentinel.go")
         d. leftover.go DID land (legit frozen leftover was folded — proves arbiter ran, not a no-op):
              ls := chnRunGit(t, repo, "ls-tree", "-r", "--name-only", "HEAD^{tree}")  (or HEAD)
              strings.Contains(ls, "leftover.go")
  - FOLLOW pattern: TestResolveArbiter_NullNewCommit (chain_test.go:137), _TipAmend (:186),
                   _MidChainRebuild (:249) — same fixture (chnBuildChain), same 7-arg call, same
                   chnDeps(bin, stubtest.Options{Out:...}) manifest.
  - DEPENDS: Task 1 (chnBuildChainWithSentinel). Imports: "strings", "context" (already in file).
  - NAMING: TestResolveArbiter_FreezeParitySentinelExcluded (matches TestResolveArbiter_* family).
  - PLACEMENT: alongside the other TestResolveArbiter_* tests (e.g. after
               TestResolveArbiter_CleanTreePostcondition, chain_test.go:456).

Task 3: ADD TestDecompose_ArbiterFoldsOnlyTStart to internal/decompose/decompose_test.go
  - IMPLEMENT: integration test — 2-concept Decompose() run, concept-1 no-op ⇒ b.go unclaimed ⇒
               arbiter RUNS (null) ⇒ arbiter commit tree == T_start; concurrent.txt excluded.
  - FIXTURE (unborn repo, mirrors TestDecompose_ConcurrentChangeExclusion decompose_test.go:832):
      1. repo := t.TempDir(); dcmInitRepo(t, repo)   // NO initial commit (unborn)
      2. dcmWriteFile(t, repo, "a.go", "package a\n")
         dcmWriteFile(t, repo, "b.go", "package b\n")
      3. Capture expectedTStart (the exactly-T_start oracle) BEFORE the run:
           dcmRunGit(t, repo, "add", "a.go", "b.go")
           expectedTStart := dcmGitOut(t, repo, "write-tree")
           dcmRunGit(t, repo, "rm", "--cached", "--ignore-unmatch", "a.go", "b.go") // restore clean index (FR-M1 trigger)
         (mirrors chnBuildChain's rm --cached --ignore-unmatch restore; works on unborn repo)
      4. Manifests:
           plannerJSON := `{"count":2,"single":false,"commits":[{"title":"add b","description":"b.go"},{"title":"add a","description":"a.go"}]}`
           plannerM := dcmPlannerManifest(t, bin, plannerJSON)
           // concept-1 "add b" SKIPPED (no message); concept-2 "add a" → message[0]; arbiter null → message[1].
           messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: arbiter leftover"})
           stagerM  := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
           arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`)   // IS invoked (leftover non-empty)
           roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
           deps := dcmDeps(t, repo, roles)
           deps.stager = concurrentSentinelSeam(t, repo,
               map[string][]string{"add b": {}, "add a": {"a.go"}},  // "add b" no-op ⇒ b.go unclaimed
               "concurrent.txt")                                     // written UNSTAGED on first concept (post-freeze)
  - ACTION:
      result, err := Decompose(context.Background(), deps)   // assert err == nil
  - ASSERTIONS (mirror S1's TestDecompose_ConcurrentChangeExclusion assertion style):
      a. EXACTLY-T_start: dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}") == expectedTStart
         (the arbiter commit's tree is EXACTLY T_start — contract: "each arbiter commit's tree
          (HEAD^{tree}) is exactly T_start".)
      b. concurrent.txt in NO commit:
           names := dcmGitOut(t, repo, "log", "--name-only", "--format=")
           !strings.Contains(names, "concurrent.txt")
      c. concurrent.txt REMAINS untracked:
           strings.Contains(dcmStatusPorcelain(t, repo), "?? concurrent.txt")
      d. Arbiter RAN (folded the leftover): dcmLogCount(t, repo) == 2  (1 loop + 1 arbiter; unborn ⇒ no seed)
         (use dcmLogCount, NOT result.Amended — Amended is 0 for null-path; Decision D5.)
  - FOLLOW pattern: TestDecompose_ConcurrentChangeExclusion (decompose_test.go:832) for the unborn-repo
                   fixture, the planner/message/arbiter manifest wiring, deps.stager override, and the
                   log/status assertions. Differs ONLY in: concept-1 is a no-op (empty slice),
                   arbiter IS invoked (returns null), and the HEAD^{tree}==expectedTStart oracle is added.
  - DEPENDS: concurrentSentinelSeam (decompose_test.go:795, reuse verbatim), dcmDeps, dcmPlannerManifest,
             dcmMessageScriptManifest, dcmArbiterManifest, dcmGitOut, dcmRunGit, dcmLogCount,
             dcmStatusPorcelain, tooledStubManifest (stager_test.go:73), stubtest.Build — all existing.
  - NAMING: TestDecompose_ArbiterFoldsOnlyTStart (matches TestDecompose_* family).
  - PLACEMENT: immediately after TestDecompose_ConcurrentChangeExclusion (decompose_test.go, ~line 890)
              and/or adjacent to TestDecompose_TStartCompleteness (its born-repo complement).
  - GOTCHA: planner concept order is [{"add b"},{"add a"}]; the first concept PROCESSED is "add b" —
            concurrentSentinelSeam writes the sentinel then (post-freeze ⇒ excluded). message[0] goes to
            the SECOND concept "add a" because "add b" is skipped (FR-M8). Do not reorder.
```

### Implementation Patterns & Key Details

```go
// === UNIT TEST (chain_test.go) — the table-driven per-target shape ===
func TestResolveArbiter_FreezeParitySentinelExcluded(t *testing.T) {
	bin := stubtest.Build(t)
	targets := []struct {
		name   string
		target func(c []CommitInfo) *string
	}{
		{"null", func(_ []CommitInfo) *string { return nil }},
		{"tip",  func(c []CommitInfo) *string { s := c[2].SHA; return &s }}, // C2 == tip
		{"mid",  func(c []CommitInfo) *string { s := c[1].SHA; return &s }}, // C1, mid-chain
	}
	for _, tc := range targets {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			chnInitRepo(t, repo) // FRESH repo per target — resolveArbiter advances HEAD
			commits, chainData, tStart, leftoverPaths := chnBuildChainWithSentinel(t, repo)
			deps := chnDeps(t, repo, stubtest.Manifest(bin, stubtest.Options{Out: "chore: arbiter leftover"}))
			target := tc.target(commits)
			if err := resolveArbiter(context.Background(), deps, target, commits, chainData, tStart, leftoverPaths); err != nil {
				t.Fatalf("resolveArbiter(%s): %v", tc.name, err)
			}
			// (a) HEAD^{tree} == T_start  (uniform across all three paths)
			if got := chnRunGit(t, repo, "rev-parse", "HEAD^{tree}"); got != tStart {
				t.Errorf("%s: HEAD^{tree} = %s, want tStart = %s", tc.name, got, tStart)
			}
			// (b) sentinel.go in NO commit
			if names := chnRunGit(t, repo, "log", "--name-only", "--format="); strings.Contains(names, "sentinel.go") {
				t.Errorf("%s: sentinel.go swept into a commit:\n%s", tc.name, names)
			}
			// (c) sentinel.go REMAINS untracked
			if status := chnRunGit(t, repo, "status", "--porcelain"); !strings.Contains(status, "?? sentinel.go") {
				t.Errorf("%s: status = %q, want it to contain '?? sentinel.go'", tc.name, status)
			}
			// (d) leftover.go DID land (legit frozen leftover folded ⇒ arbiter ran, not a no-op)
			if ls := chnRunGit(t, repo, "ls-tree", "-r", "--name-only", "HEAD"); !strings.Contains(ls, "leftover.go") {
				t.Errorf("%s: leftover.go missing from HEAD tree — arbiter did not fold the frozen leftover", tc.name)
			}
		})
	}
}

// === INTEGRATION TEST (decompose_test.go) — the exactly-T_start oracle ===
// (fixture + assertion shapes shown in Task 3; the key oracle line:)
headTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}")
if headTree != expectedTStart {
	t.Errorf("arbiter commit tree = %s, want EXACTLY T_start = %s", headTree, expectedTStart)
}
```

### Integration Points

```yaml
NONE — pure test additions. No git/index/ref migration, no config, no routes, no new packages.
The two new symbols (chnBuildChainWithSentinel, TestResolveArbiter_FreezeParitySentinelExcluded,
TestDecompose_ArbiterFoldsOnlyTStart) are package-internal to internal/decompose and consumed only by
`go test`. They reference only existing exported/unexported helpers in the same _test.go files.
```

---

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Run after writing each test — fix before proceeding.
go vet ./internal/decompose/...
gofmt -l internal/decompose/chain_test.go internal/decompose/decompose_test.go   # empty output = OK
golangci-lint run ./internal/decompose/...    # (or: make lint)

# Expected: zero errors. If go vet/lint reports, READ the output and fix (usually an unused import
# or a shadowed variable) before continuing.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The two NEW tests, verbose — the primary acceptance gate.
go test -race -run 'TestResolveArbiter_FreezeParitySentinelExcluded|TestDecompose_ArbiterFoldsOnlyTStart' \
    ./internal/decompose/ -v

# The full chain_test.go arbiter family (must stay green — the new test is their sibling).
go test -race -run 'TestResolveArbiter' ./internal/decompose/ -v

# The full decompose integration family (S1's tests must stay green alongside the new one).
go test -race -run 'TestDecompose_' ./internal/decompose/ -v

# Expected: ALL pass. If a NEW test fails, the invariant is real — investigate whether the freeze-safe
# arbiter regressed (it should not; the code is landed). Do NOT weaken the assertions to make it pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# Full package with the race detector — the regression net (no other test must break).
go test -race ./internal/decompose/

# Whole-repo suite (the Makefile target).
make test      # == go test -race ./...

# Expected: zero failures across the entire repo. The new tests encode the POST-FIX invariant
# (Decision D7) — they must PASS against the landed freeze-safe arbiter; do NOT chase a pre-fix failure.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Confirm scope discipline (the FORBIDDEN-OPERATIONS guard for this task).
git diff --stat
# Expected: EXACTLY two files — internal/decompose/chain_test.go and internal/decompose/decompose_test.go.
# If anything else appears (chain.go, decompose.go, git/*, docs/*, PRD.md, tasks.json), STOP and revert.

# Spot-check the three invariants hold for the mid-chain path specifically (the path most likely to
# leak): the subtest name 'mid' must appear PASSED in the -v output above.
go test -race -run 'TestResolveArbiter_FreezeParitySentinelExcluded/mid' ./internal/decompose/ -v
```

---

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `go vet ./internal/decompose/...` clean; `gofmt -l` empty; `golangci-lint run` clean.
- [ ] Level 2: `TestResolveArbiter_FreezeParitySentinelExcluded` PASSes for all three subtests
      (`null`, `tip`, `mid`).
- [ ] Level 2: `TestDecompose_ArbiterFoldsOnlyTStart` PASSes.
- [ ] Level 3: `go test -race ./...` (or `make test`) — entire repo green, no regression.

### Feature Validation

- [ ] **Unit**: for `target ∈ {nil, &tipSHA, &sha1}`, `HEAD^{tree} == tStart`, `sentinel.go` is in
      no commit, `?? sentinel.go` remains in status, and `leftover.go` landed in `HEAD^{tree}`.
- [ ] **Integration**: `HEAD^{tree} == expectedTStart` (arbiter commit tree is EXACTLY T_start);
      `concurrent.txt` in no commit; `?? concurrent.txt` in status; `dcmLogCount == 2`.
- [ ] The two new tests are the **success-path siblings** of S1's
      `TestDecompose_ConcurrentChangeExclusion` (empty leftover / arbiter skipped) — this task covers
      the **non-empty leftover / arbiter runs** complement.
- [ ] Decision D7 honored: tests PASS against the landed code (permanent regression net), not a
      pre-fix-failure chase.

### Code Quality Validation

- [ ] `chnBuildChainWithSentinel` is a thin wrapper over `chnBuildChain` (no duplicated fixture logic).
- [ ] `concurrentSentinelSeam` is REUSED verbatim (not redeclared, not edited).
- [ ] No existing `TestResolveArbiter_*` or S1 test was modified.
- [ ] No production code (`chain.go`, `decompose.go`, `arbiter.go`, `internal/git/*`) touched.
- [ ] File placement matches the desired tree (helper + test in `chain_test.go`; test in `decompose_test.go`).

### Documentation & Deployment

- [ ] N/A — internal tests, no docs (per item contract: "DOCS: none — internal tests").
- [ ] Test comments cite the invariant source (PRD §20.2 "Arbiter freeze parity" / "Start-of-run
      freeze"; FR-M1d), mirroring the comment style of S1's tests.

---

## Anti-Patterns to Avoid

- ❌ Don't write the sentinel STAGED (`git add`) — that turns it into a freeze violation
  (`ErrFreezeViolation`), which is `TestDecompose_StagerFreezeViolation`'s job, not this task's.
  Write it UNSTAGED (`chnWriteFile` / `os.WriteFile`).
- ❌ Don't reuse one repo across the three unit-test targets — `resolveArbiter` advances HEAD; the
  second call's stale `tipSHA` fails the CAS. Fresh repo per `t.Run` subtest.
- ❌ Don't extend `chnBuildChain` in-place to write the sentinel — 7 existing tests assert
  `git status == ""` (clean) against it and would break. Use the `chnBuildChainWithSentinel` variant.
- ❌ Don't reorder the integration planner concepts or mismatch the message script — concept-1 ("add b")
  is SKIPPED (FR-M8) and consumes no message; `message[0]` is concept-2 ("add a"), `message[1]` is the
  arbiter's null-path message.
- ❌ Don't weaken an assertion to make a test pass. These encode the POST-FIX invariant; a failure
  means the freeze-safe arbiter regressed — investigate the production code, not the test.
- ❌ Don't touch `concurrentSentinelSeam`, `dcm*`, `chn*` (except the one new helper), `docs/*`,
  `PRD.md`, `tasks.json`, `prd_snapshot.md`, or any `plan/*` artifact.

---

## Confidence Score

**9 / 10** for one-pass implementation success.

Rationale: this is a narrowly-scoped, test-only task with an exceptionally complete, line-verified
research note (`plan/008_82253c999440/P1M1T3S2/research/arbiter_folds_tstart_proof.md`). Every helper
referenced (`chnBuildChain`, `chnDeps`, `concurrentSentinelSeam`, `dcmDeps`, `dcmLogCount`,
`dcmStatusPorcelain`, `dcmMessageScriptManifest`, `dcmArbiterManifest`, `tooledStubManifest`,
`stubtest.Build/Manifest`) was confirmed to exist with the cited signatures, and the production code
under test (`resolveArbiter` 7-arg, three paths, `HEAD^{tree} == tStart`) is landed and passing. The
two sibling S1 tests provide an exact template to mirror. The residual 1/10 risk is the message-script
call-order subtlety (concept-1 skip consumes no message) and the unborn-repo `expectedTStart` capture —
both explicitly flagged above with the exact command sequence.
