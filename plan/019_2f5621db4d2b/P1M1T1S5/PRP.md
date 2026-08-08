name: "P1.M1.T1.S5 — Fast-path regression suite in decompose_test.go (disjoint/fallback/concurrency/CAS/isolation/skip/freeze)"
description: >
  Add a comprehensive FR-M13 fast-path regression suite to internal/decompose/decompose_test.go covering
  the 9 cases in the work item: (1) disjoint→N commits + stager-never-invoked + concept isolation +
  T_start completeness (both arbiter sub-cases); (2) shared-file→tooled-stager fallback for the WHOLE
  run (matches runLoop); (3) concurrency is REAL (per-goroutine start/end intervals pairwise overlap,
  not strictly serial); (4) out-of-order message completion still publishes in strict CAS order;
  (5) FR-M12 failure isolation on the fast-path — (a) rescue at position 1 (partial stands, concept 1
  named, in-flight drained), (b) CAS failure at position 1 (prior commits stand, run aborts); (6) FR-M8
  empty-skip on the fast-path (a concept staging nothing → skipped, no empty commit, run continues);
  (7) FR-M1c verifyFreezeSubset is provably wired (called once per concept via a counting git wrapper);
  (8) tooled_flags-less provider (G29 side effect) — disjoint SUCCEEDS via the fast-path, shared
  surfaces the existing 'tooled mode requires non-empty tooled_flags' error; (9) start-of-run freeze
  (FR-M1b) — a sentinel written after T_start is frozen lands in NO commit and stays in the worktree.
  Cases 1/2/3/4/5/6/7/8 drive through Decompose() (end-to-end, exercises S4's dispatch); case 9 calls
  runLoopFastPath directly (controlled freeze→sentinel→run timing, S3's direct-call idiom). MOST cases
  reuse existing helpers (dcm*, dcmMessageMatchManifest, arbiter counter). TWO additive, default-off,
  backward-compatible stub hooks are added to cmd/stubagent/main.go (a test-only binary S3 already
  extended): STAGECOACH_STUB_INTERVAL_FILE (case 3) + per-match sleep (case 4/5b, extends the match-file
  line to an optional 3rd `|sleepMs` field). runLoopFastPath/runLoop/isFileDisjoint/drains + the
  Decompose dispatch + arbiter phase + all non-test source are BYTE-IDENTICAL. No config/flag/docs (S6).

---

## Goal

**Feature Goal**: Prove the FR-M13 file-disjoint fast-path (S1 gate + S2 sweep + S3 concurrent +
S4 dispatch) holds every invariant the PRD/spec demand, end-to-end through `Decompose`, with a
deterministic stub harness — and lock those invariants against silent future regressions. The fast-path
collapses N sequential LLM steps into one message latency and bypasses the stager entirely; that is a
large behavioral surface that unit tests on `runLoop` (the shared-file fallback) cannot reach, so a
dedicated regression suite is the net (PRD §20.1 layer 3, §20.2, §20.5).

**Deliverable**: ~9 new `Test*` functions in `internal/decompose/decompose_test.go` (grouped,
`TestDecompose_FastPath_*` / `TestRunLoopFastPath_*` per case) + 2 minimal additive hooks in the
test-only `cmd/stubagent/main.go`. A green `go test ./internal/decompose/... ./internal/git/...
./internal/config/...` (PRD §5 OUTPUT).

**Success Definition**:
- All 9 cases pass under `-race` with ZERO race reports.
- Case 1 proves disjoint routing (stager t.Fatal never trips), concept isolation (each commit's
  diff-tree == exactly its concept's Files), correct CAS-ordered parenting, and T_start completeness in
  BOTH sub-cases (frozen-leftover-empty → arbiter skipped/counter 0; leftover-present → arbiter
  folds/counter 1, arbiter-commit tree == T_start).
- Case 3 asserts HARD per-goroutine interval overlap (≥1 consecutive pair where `start[j] < end[i]`),
  not merely elapsed-time — the regression guard a future silent re-serialization cannot evade.
- Case 4 asserts the chain is strictly ordered preRunHEAD→c0→c1→… even when message 0 finishes LAST.
- Case 7 proves `verifyFreezeSubset` fires once per concept (MergeTrees count == len(concepts)).
- Case 8 proves a TooledFlags-less provider decomposes a disjoint tree (fast-path) but errors on a
  shared-file tree with the unchanged `tooled mode requires non-empty tooled_flags` message.
- Case 9 proves a post-freeze sentinel lands in no commit and survives in the worktree.
- `runLoopFastPath`/`runLoop`/`isFileDisjoint`/`drainMsg`/`drainMsgs` + the dispatch + arbiter phase +
  every non-`*_test.go`/non-`cmd/stubagent` source file is BYTE-IDENTICAL (grep-verified).

## User Persona (if applicable)

**Target User**: The maintainer (regression net). PRD §20.5: "The concurrency and routing invariants
above are easy to specify, easy to regress, and — as repeated field discoveries have shown — easy to
break silently (unit tests with stub agents cannot reach them). … Every bug found in the wild becomes a
scenario here." This suite is that net for the fast-path.

**Use Case**: A future refactor "tidies up" the fast-path into a serial loop (or drops a
`verifyFreezeSubset` call, or routes a disjoint partition to `runLoop` by accident). CI turns red on the
specific invariant violated, naming the FR.

## Why

- **FR-M13/M14 (P0, Goal G29)**: the fast-path is the v2.1 critical-path win. It has the LARGEST
  behavioral surface of any single feature in this milestone, and the MOST concurrency. Without a
  dedicated suite, a regression (re-serialization, a dropped freeze guard, a mis-routed partition) ships
  silently — exactly the §20.5 gap that let prior concurrency bugs through.
- **PRD §20.2 / §20.5**: the authoritative invariant list (concept isolation, T_start completeness,
  start-of-run freeze, atomic HEAD, concurrency) names these as must-cover; the e2e harness's
  "N unrelated files → N commits" + "concurrent file mid-run excluded" scenarios are driven through the
  fast-path here.
- **FR-M1c defense-in-depth**: `verifyFreezeSubset` is the orchestrator-owned freeze guard. Case 7 proves
  it is WIRED on the fast-path (called per concept), so a future edit that drops the call fails CI rather
  than silently trusting a tooled stager that the fast-path doesn't even invoke.
- **G29 side effect (FR-D4)**: the fast-path lets TooledFlags-less providers (opencode, qwen-code)
  decompose disjoint trees they otherwise could not serve as stager. Case 8 locks that in.

## What

**User-visible behavior**: none (tests). The suite asserts the fast-path's observable contracts:
disjoint trees decompose into N correctly-isolated, correctly-ordered commits with no stager agent; a
shared file falls back; messages genuinely overlap; failures isolate; the freeze holds.

**Technical change (tests + 2 additive stub hooks):**
1. ~9 `Test*` functions in `internal/decompose/decompose_test.go`.
2. Two additive, default-off env hooks in `cmd/stubagent/main.go` (a test-only binary): a new
   `STAGECOACH_STUB_INTERVAL_FILE` (append `start_ns end_ns\n` per invocation) and per-match sleep (the
   match-file line gains an OPTIONAL 3rd `|sleepMs` field; 2-field lines behave exactly as before).

### Success Criteria
- [ ] Case 1 `TestDecompose_FastPath_DisjointIsolationAndCompleteness` (two sub-tests/sub-cases).
- [ ] Case 2 `TestDecompose_FastPath_SharedFallbackMatchesRunLoop`.
- [ ] Case 3 `TestDecompose_FastPath_ConcurrencyIntervalOverlap`.
- [ ] Case 4 `TestDecompose_FastPath_OutOfOrderCompletesOrderedPublish`.
- [ ] Case 5a `TestDecompose_FastPath_RescueIsolation` + 5b `TestDecompose_FastPath_CASAbortPartial`.
- [ ] Case 6 `TestDecompose_FastPath_EmptyConceptSkip`.
- [ ] Case 7 `TestDecompose_FastPath_FreezeGuardWired`.
- [ ] Case 8 `TestDecompose_FastPath_TooledFlagsLessProvider` (disjoint-success + shared-error sub-cases).
- [ ] Case 9 `TestRunLoopFastPath_StartOfRunFreezeExcludesSentinel` (direct-call).
- [ ] `cmd/stubagent/main.go`: +`STAGECOACH_STUB_INTERVAL_FILE` + per-match sleep; existing stub tests
      (TestStub_*) still pass unchanged.
- [ ] `go test ./internal/decompose/... ./internal/git/... ./internal/config/... -race -count=1` green.
- [ ] No non-test / non-stubagent source file changed.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — every case has: the exact drive-path (Decompose vs direct runLoopFastPath), the exact helpers to
reuse (named, with their file:line), the exact stub manifest to build (including the new hooks), the exact
git commands for assertions, the precise count/error semantics, and the scope fences. See research/findings.md
for the verified per-case facts.

### Documentation & References

```yaml
- file: internal/decompose/decompose.go
  why: "runLoopFastPath (:662) is the function under test (S2 sweep + S3 concurrent). isFileDisjoint
        (:450), runLoop (:479), drainMsgs (:825) are the BYTE-IDENTICAL siblings — S5 EXERCISES them,
        never edits them. Decompose() dispatch is S4's gate (:237)."
  pattern: "read the two-phase algorithm (serial sweep → concurrent launch → serial CAS publish) to
            know WHAT each case asserts. The sweep calls verifyFreezeSubset (FR-M1c) BEFORE the FR-M8
            skip check; the publish loop drains inflight[i+1:] on ANY abort."

- file: internal/decompose/decompose_test.go
  why: "THE home for the new tests (package decompose). Reuse: dcmInitRepo/dcmWriteFile/dcmStageFile/
        dcmCommitRaw/dcmRunGit/dcmGitOut/dcmHeadSHA/dcmLogCount/dcmStatusPorcelain (:29-110);
        dcmPlannerManifest/dcmMessageScriptManifest/dcmArbiterManifest (:110-134); dcmDeps/dcmDepsWithConfig/
        dcmOutBuffer (:140-234); dcmStagerSeam (:176); dcmMessageMatchManifest + messageMatchRule (:146-170,
        S3 — input-derived, concurrency-safe message selection); tooledStubManifest (stager_test.go:73,
        adds TooledFlags); stubtest.Manifest/Build/NewScript."
  pattern: "MIRROR TestDecompose_AutoMultiCommit_HappyPath (:465) for the end-to-end Decompose shape;
            TestDecompose_HunkSplitAcrossConcepts (:849) for the shared-file/stager-seam shape;
            TestRunLoopFastPath_ConcurrentPublish (:2986, S3) for the direct-call fast-path shape +
            CAS-order assertions; TestDecompose_MessageRescuePartial (:1529) for the RescueError shape
            (errors.As *DecomposeRescueError/*generate.RescueError, errors.Is ErrRescue, 'concept N of M'
            in Out); TestDecompose_CASAbortPartial (:1642) for the poll-log+commit-tree HEAD-move idiom;
            TestDecompose_SentinelAfterFreezeExcluded (:2597) for the sentinel-in-no-commit shape."
  gotcha: "dcmDeps sets Verbose: nil. For the 'launched N concurrent' log or the FR-M3b unclaimed-path
           log, build deps with Verbose: ui.NewVerbose(&buf, true) (see TestRunLoopFastPath_ConcurrentPublish
           :3057). dcmOutBuffer (:220) gives a Deps + *bytes.Buffer for Out (rescue/CAS message capture)."

- file: cmd/stubagent/main.go
  why: "The test-only fake-agent binary. S5 adds TWO additive hooks here (case 3 + case 4/5b). S3 ALREADY
        extended it (added STAGECOACH_STUB_MATCHFILE + selectMatched) — so stub edits are an accepted
        pattern in this workstream. main() order: drain stdin (→buffer if MATCHFILE) → marker → argsfile
        → sleep(SLEEP_MS) → stderr → select output (MATCHFILE > SCRIPT > OUT) → print → exit."
  pattern: "ADD: (a) after stdin drain, if STAGECOACH_STUB_INTERVAL_FILE set, record start_ns; after
            output-select, append 'start_ns end_ns\\n' (one os.OpenFile O_APPEND Write, <4096B ⇒ atomic
            on POSIX). (b) extend selectMatched to return (msg, sleepMs) by parsing an OPTIONAL 3rd
            '|sleepMs' field; when MATCHFILE set, sleep sleepMs (S3's 2-field lines ⇒ sleepMs 0 ⇒ unchanged)."
  gotcha: "Both hooks are STRICTLY additive + default-off (no env ⇒ identical behavior). The stub package
           doc says 'tiny fake-agent binary for Stagecoach's integration/property tests' — test infra, not
           product. Re-run internal/stubtest/stubtest_test.go (TestStub_*) after editing to confirm no
           regression in the existing stub contract."

- file: internal/decompose/stager.go
  why: "verifyFreezeSubset (:168) is what case 7 counts. It calls deps.Git.MergeTrees(baseTree, treeI,
        tStart) exactly ONCE per invocation (part B content check; NO short-circuit). MergeTrees is called
        ONLY by verifyFreezeSubset (grep-confirmed). ErrFreezeViolation (:~80) / ErrStagerFailed (:~46) /
        ErrStagerMovedHEAD (:~62) sentinels for assertions."
  pattern: "case 7: a counting git wrapper that embeds git.Git and overrides MergeTrees to Add(1); assert
            count == len(concepts) (verifyFreezeSubset runs for EVERY concept in the sweep, before the
            FR-M8 skip — so with 3 disjoint non-skipped concepts, count == 3)."

- file: internal/provider/render.go
  why: "case 8's shared-path error. RenderTooled (render.go:153-157): if len(r.TooledFlags)==0 →
        fmt.Errorf('provider %q: tooled mode requires non-empty tooled_flags', m.Name). stageConcept
        (stager.go) wraps it: '%w: render: %v', ErrStagerFailed."
  pattern: "case 8 shared: set Roles.Stager = stubtest.Manifest(bin, stubtest.Options{Out:\"\"}) (a BARE
            manifest — nil TooledFlags, the opencode/qwen-code shape). Do NOT inject deps.stager (let it
            hit the real stageConcept → RenderTooled error). Assert errors.Is(err, ErrStagerFailed) AND
            strings.Contains(err.Error(), 'tooled mode requires non-empty tooled_flags')."

- file: internal/decompose/roles.go
  why: "Deps.Git is the git.Git INTERFACE (:56; git.New returns Git, git.go:489) ⇒ the case-7 counting
        wrapper works via embedding. deps.stager (:~88) is the unexported test seam (nil in prod); on the
        fast-path it is UNREACHABLE (the routing oracle). ResolveRoles is NOT used in tests (dcmDeps sets
        Roles directly)."

- docfile: plan/019_2f5621db4d2b/architecture/system_context.md
  why: "§6 stager-seam-unreachable-on-fast-path (the case 1/2 routing oracle: inject deps.stager that
        t.Fatal's). §1 the dispatch site + in-scope variables. §5 arbiter-runs-unchanged."

- docfile: plan/019_2f5621db4d2b/architecture/git_primitives.md
  why: "§'CRITICAL CONCURRENCY FINDING' — the staging sweep MUST be serial (Add/WriteTree mutate .git/
        index; no in-process lock); message goroutines are confined to read-only tree reads. Confirms the
        fast-path design is sound + why case 3 (true overlap) is a meaningful regression guard."

- docfile: plan/019_2f5621db4d2b/architecture/spec_requirements.md
  why: "§13.6.6 failure handling (the cases 5/6 basis) + §20.2/§20.5 invariant list (cases 1/3/9 basis)."

- docfile: plan/019_2f5621db4d2b/P1M1T1S3/PRP.md
  why: "S3 is the CONTRACT: it LANDED runLoopFastPath's concurrent phase, drainMsgs, AND the stub's
        MATCHFILE hook + dcmMessageMatchManifest + messageMatchRule{substr,msg}. S5 EXTENDS messageMatchRule
        with an OPTIONAL sleepMs field + the stub's selectMatched to honor it (backward-compatible), and
        adds STAGECOACH_STUB_INTERVAL_FILE. Confirm S3's tests (_ConcurrentPublish, _RescueIsolation) are
        present + green before adding S5 (they are the fast-path's baseline)."

- docfile: plan/019_2f5621db4d2b/P1M1T1S5/research/findings.md
  why: "The per-case design table + every verified fact (exact error strings, count semantics, helper
        file:lines, distinctness from S3/S4). READ THIS before writing the tests."
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  decompose.go        # runLoopFastPath(:662) under test; isFileDisjoint(:450)/runLoop(:479)/drainMsgs(:825) BYTE-IDENTICAL
  decompose_test.go   # ← ADD ~9 Test* (S5); existing dcm* helpers + dcmMessageMatchManifest (S3) reused
  stager.go           # verifyFreezeSubset(:168) — case 7 counts its MergeTrees call; ErrFreezeViolation/ErrStagerFailed
  roles.go            # Deps.Git(git.Git interface), deps.stager seam — UNCHANGED
cmd/stubagent/main.go # test-only fake agent — ADD 2 hooks (INTERVAL_FILE + per-match sleep); S3 already edited it
internal/stubtest/    # stubtest.Manifest/Build/NewScript/Options — UNCHANGED
internal/git/git.go   # git.Git interface(:96); New(:489)→Git; MergeTrees(:450/2195) unique to verifyFreezeSubset
```

### Desired Codebase tree with files to be added/modified

```bash
internal/decompose/decompose_test.go   # MODIFY: +~9 TestDecompose_FastPath_* / TestRunLoopFastPath_* (S5)
cmd/stubagent/main.go                  # MODIFY: +STAGECOACH_STUB_INTERVAL_FILE (case 3) + per-match sleep (case 4/5b)
                                        #   (both additive, default-off, backward-compatible)
# NOTHING ELSE. runLoopFastPath/runLoop/isFileDisjoint/drains/dispatch/arbiter/all other source: BYTE-IDENTICAL.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (counter race on the concurrent fast-path). The stub's SCRIPT+COUNTFR mode races when N message
//   processes launch concurrently (they read/increment ONE counter file) → duplicate/garbled responses.
//   S5 MUST use dcmMessageMatchManifest (input-derived, per-process stdin inspection) for ANY case needing
//   deterministic per-concept message behavior (cases 3, 4, 5a, 5b). NEVER NewScript for fast-path messages.

// CRITICAL (verifyFreezeSubset runs BEFORE the skip check). runLoopFastPath calls verifyFreezeSubset for EVERY
//   concept in the sweep, then checks treeI==prevTree (FR-M8 skip). So MergeTrees (verifyFreezeSubset's part-B
//   call) is invoked len(concepts) times — including would-be-skipped concepts. For case 7 use a scenario where
//   ALL concepts are non-skipped (3 disjoint files) ⇒ count == 3 == len(concepts) == num non-skipped. Do NOT
//   assert count == (len(concepts) - numSkipped); that is wrong for this code.

// CRITICAL (case 8: do NOT inject deps.stager on the shared path). Injecting a stager seam would MASK the
//   'tooled mode requires non-empty tooled_flags' error (the seam short-circuits stageConcept). Leave deps.stager
//   nil so the run hits the REAL stageConcept → RenderTooled → error. The disjoint sub-case never reaches the
//   stager at all (fast-path bypass), so it succeeds regardless.

// CRITICAL (case 5b window). The fast-path launches ALL messages, THEN publishes serially. With uniform sleep,
//   all messages finish ~together ⇒ no gap between c0-publish and c1-publish (CAS window ≈ 0). To create a
//   reliable window, use per-match sleep (case 4's hook): c0's message FAST, c1's message SLOW. Then c0 publishes
//   while c1 is still in flight → poll 'feat: add a' in log + c1 not yet → commit-tree/update-ref HEAD → c1's
//   CAS fails. Mirror TestDecompose_CASAbortPartial (:1642)'s poll+commit-tree idiom (NOT git commit --allow-empty).

// CRITICAL (case 9 must be direct-call). There is NO Go-level seam between Decompose's FreezeWorkingTree and
//   runLoopFastPath (the planner is an external stub process; the fast-path has no stager seam). So the
//   post-freeze sentinel cannot be written at the right moment through Decompose. Mirror S3's direct-call idiom
//   (TestRunLoopFastPath_ConcurrentPublish setup): capture baseTree, call g.FreezeWorkingTree, write the sentinel
//   to the worktree, THEN call runLoopFastPath directly. This gives exact control of the freeze→sentinel→run order.

// GOTCHA (atomic interval append). cmd/stubagent is a separate PROCESS per message invocation. Cross-process
//   "shared slice under a mutex" (the item's phrasing) is implemented as a shared append FILE: each process
//   appends ONE 'start_ns end_ns\n' line via a single os.OpenFile(O_APPEND).Write of <4096B ⇒ atomic on POSIX
//   (PIPE_BUF). The test reads all lines, parses N intervals, sorts by start, asserts ≥1 consecutive overlap
//   (start[j] < end[i]) — NOT strictly serial. On the fast-path all N launch ~simultaneously + sleep equally ⇒
//   all overlap robustly (a 100ms sleep dominates ms-scale exec jitter). NOT flaky.

// GOTCHA (concept isolation assertion). For commit i: dcmGitOut(repo, "diff-tree","--no-commit-id","--name-only",
//   "-r", commits[i].SHA, parentSHA) → split+sort lines == sort(concept[i].Files). parentSHA = i==0 ? preRunHEAD
//   : commits[i-1].SHA. Commits may be FR-M8-skipped (absent from result.Commits), so index by result.Commits,
//   not by concept index.

// GOTCHA (arbiter counter). stubtest.Manifest(bin, Options{Script: dir+"/script.txt", Counter: counterFile})
//   writes the call index to counterFile. Post-run, read it: "0"/"" ⇒ arbiter NOT called; ≥1 ⇒ called. Mirror
//   TestDecompose_MessageRescuePartial (:1586) / TestDecompose_CASAbortPartial (:1682). The arbiter stub returns
//   {"target": null} (→ new commit) for case 1 sub-case B.

// GOTCHA (born vs unborn). Most cases seed an initial commit (born repo) so baseTree = HEAD^{tree} and commit[0]
//   has a parent. For the fast-path this matches TestRunLoopFastPath_ConcurrentPublish. Unborn (case 1 may also
//   add an unborn sub-check) ⇒ commit[0] is a root commit (no -p); mirror TestDecompose_UnbornRepo (:1464).
```

## Implementation Blueprint

### Data models and structure

None new (tests). The two stub hooks reuse existing env-string plumbing (`os.Getenv` +
`os.OpenFile`). The case-7 counting wrapper is a tiny test-local struct embedding `git.Git`.

### Implementation Tasks (ordered by dependencies)

> **Prerequisites (verified present at S5 start)**: S1 `isFileDisjoint`, S2+S3 `runLoopFastPath`
> (sentinel GONE), `drainMsgs`; S3's `dcmMessageMatchManifest` + `messageMatchRule{substr,msg}` +
> the stub's `STAGECOACH_STUB_MATCHFILE`/`selectMatched`; S4's dispatch gate + 2 dispatch tests.
> **Before writing tests**: `grep -c 'not yet implemented' internal/decompose/decompose.go` == 0
> (S3 landed) AND `grep -n 'isFileDisjoint(out.Commits)' internal/decompose/decompose.go` == 1 (S4
> landed) AND `go test ./internal/decompose/ -run 'TestRunLoopFastPath|TestDecompose_Dispatch' -race`
> is GREEN. If any fails, S3/S4 hasn't landed — STOP and surface it.

```yaml
Task 1: ADD STAGECOACH_STUB_INTERVAL_FILE hook to cmd/stubagent/main.go (case 3 infra)
  - EDIT main(): after the stdin-drain block (which already retains stdin to a buffer when MATCHFILE
    is set), capture start_ns := time.Now().UnixNano() into a local (only if INTERVAL_FILE set).
    After the output-select + before os.Exit, if STAGECOACH_STUB_INTERVAL_FILE != "", append
    "<start_ns> <end_ns>\n" (end_ns = time.Now().UnixNano()) via:
        f, _ := os.OpenFile(intervalFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
        f.WriteString(fmt.Sprintf("%d %d\n", startNs, endNs)); f.Close()
    GUARD: only act when STAGECOACH_STUB_INTERVAL_FILE != "". Default-off ⇒ zero behavior change.
  - PLACE: minimal, near the existing MARKER/ARGSFILE write blocks. STDLIB only (os, fmt, time —
    already imported).
  - VERIFY: go test ./internal/stubtest/ -run TestStub -race  → existing stub tests unchanged.

Task 2: ADD per-match sleep (extend MATCHFILE) to cmd/stubagent/main.go (case 4/5b infra)
  - EDIT selectMatched(matchFile, stdin string) (string) → selectMatched(...) (msg string, sleepMs int):
    parse each line as fields := strings.Split(line, "|"); if len >= 3, sleepMs = atoi(fields[2]) (0
    on parse error / absence). Return the FIRST rule whose fields[0] (substr) is in stdin, with its
    fields[1] (msg) + sleepMs. A 2-field line (S3's format) ⇒ sleepMs 0 ⇒ UNCHANGED behavior.
  - EDIT main(): when MATCHFILE set, resolve (msg, sleepMs) = selectMatched(...) EARLY (right after
    stdin drain, before the uniform sleep), then sleep max(STAGECOACH_STUB_SLEEP_MS, sleepMs) (or
    sleepMs when SLEEP_MS==0). Keep the OUT/SCRIPT precedence as-is when MATCHFILE is unset.
    GUARD: the new sleep path is only when MATCHFILE set; default-off ⇒ zero behavior change for
    every existing caller.
  - BACKWARD-COMPAT: 2-field match lines (S3's tests) parse to sleepMs 0 ⇒ no extra sleep ⇒ S3's
    _ConcurrentPublish / _RescueIsolation UNCHANGED.
  - VERIFY: go test ./internal/decompose/ -run 'TestRunLoopFastPath_ConcurrentPublish|TestRunLoopFastPath_RescueIsolation' -race
    → S3's tests still GREEN (proves backward-compat).

Task 3: EXTEND messageMatchRule with an OPTIONAL sleepMs field (case 4/5b helper) in decompose_test.go
  - EDIT messageMatchRule (decompose_test.go:165): add sleepMs int (zero-value ⇒ no sleep ⇒ S3 callers
    that use the 2-field literal {...} still compile — Go struct literals with positional fields break,
    so S5 MUST update S3's call sites to NAMED fields OR keep substr,msg first and add sleepMs third).
    SAFEST: change S3's two dcmMessageMatchManifest call sites to named-field literals {substr:"…",msg:"…"}
    (semantically identical) so adding sleepMs is non-breaking. dcmMessageMatchManifest writes each line
    as substr|msg (+ "|sleepMs" if sleepMs>0).
  - DEPENDS: Task 2 (the stub honors the 3rd field).

Task 4: Case 1 — TestDecompose_FastPath_DisjointIsolationAndCompleteness (decompose_test.go)
  - DRIVE: Decompose(context.Background(), deps). Born repo; 3 disjoint untracked files (a.txt/b.txt/c.txt).
  - PLANNER: disjoint partition (each concept's Files = [its file]); union == all 3 ⇒ sub-case A.
  - STAGER ORACLE: deps.stager = func(...) error { t.Fatalf("fast-path must not invoke the stager
    (concept %q)", concept.Title); return nil }.
  - MESSAGE: dcmMessageMatchManifest (a.txt→"feat: add a", b.txt→"feat: add b", c.txt→"feat: add c").
  - SUB-CASE A (arbiter skipped): arbiter stub counter; assert len(result.Commits)==3, each commit's
    diff-tree vs parent == exactly its concept's Files (sorted), parents chain preRunHEAD→c0→c1→c2,
    counter=="0"/"" (arbiter NOT called — leftover empty), dcmStatusPorcelain=="" (clean). Mirror
    HappyPath (:465) assertion shape.
  - SUB-CASE B (arbiter folds): add a 4th file d.txt to the worktree but declare it for NO concept
    (planner omits it) ⇒ frozen leftover non-empty ⇒ arbiter called. Arbiter stub returns
    {"target": null}. Assert counter ≥1, len(result.Commits)==4, the (N+1)-th commit's tree == T_start
    (dcmGitOut "rev-parse <tip>^{tree}" == tStart — capture tStart via a direct FreezeWorkingTree OR
    assert the arbiter commit's file list includes d.txt and excludes a/b/c).
  - DEPENDS: Task 1 (no), just S3/S4. PLACE near TestDecompose_Dispatch_* (S4) / HappyPath (:465).

Task 5: Case 2 — TestDecompose_FastPath_SharedFallbackMatchesRunLoop
  - DRIVE: Decompose. One file (store.py) split across 2 concepts (mirror HunkSplit :849 setup: base →
    tStart; planner declares store.py in BOTH concepts ⇒ isFileDisjoint FALSE ⇒ runLoop).
  - STAGER SEAM: deps.stager stages concept 0's hunk (basePlusA) then concept 1 reaches tStart (copy
    HunkSplit :875's stager closure verbatim via stagePartialBlob).
  - ASSERT: stager called for BOTH concepts (flag), len(result.Commits)==2, tip's store.py == tStart
    (reconstructs the full change) — i.e. byte-identical to the pre-fast-path runLoop behavior. This
    proves the fallback is the UNCHANGED runLoop (the fast-path never participates on a shared file).
  - DEPENDS: S4 (the dispatch). PLACE near HunkSplit (:849).

Task 6: Case 3 — TestDecompose_FastPath_ConcurrencyIntervalOverlap (uses Task 1 INTERVAL hook)
  - DRIVE: Decompose. 3 disjoint files; dcmMessageMatchManifest (distinct msgs); messageM.Env
    ["STAGECOACH_STUB_SLEEP_MS"]="100"; messageM.Env["STAGECOACH_STUB_INTERVAL_FILE"]= intervalFile
    (t.TempDir()+"/intervals.txt"). stager seam t.Fatal.
  - RUN: result, err := Decompose(...); assert err==nil, len(result.Commits)==3.
  - ASSERT (HARD overlap): read intervalFile; split lines; parse each "start end" (strconv.ParseInt);
    assert len==3; sort by start; assert NOT strictly serial — there EXISTS i where start[i+1] < end[i]
    (overlap). On the fast-path ALL 3 launch ~together + sleep 100ms ⇒ all overlap. A silently
    re-serialized impl ⇒ strictly serial (end[i] <= start[i+1]) ⇒ FAIL. This is the regression guard
    S3's soft-elapsed-gate cannot provide.
  - CORROBORATE (optional): assert the "launched 3 concurrent message generations" log (Verbose buf).
  - DEPENDS: Task 1.

Task 7: Case 4 — TestDecompose_FastPath_OutOfOrderCompletesOrderedPublish (uses Task 2/3 match-sleep)
  - DRIVE: Decompose. 3 disjoint files. dcmMessageMatchManifest with per-match sleep so LATER concepts
    finish FIRST: {substr:"a.txt", msg:"feat: add a", sleepMs:300}, {substr:"b.txt", msg:"feat: add b",
    sleepMs:200}, {substr:"c.txt", msg:"feat: add c", sleepMs:100}. (message 0 sleeps longest ⇒ finishes
    LAST; message 2 finishes FIRST.) stager seam t.Fatal.
  - ASSERT: result, err := Decompose(...); err==nil; len==3; the chain is STRICTLY ordered
    preRunHEAD→c0→c1→c2: for each i, dcmGitOut("rev-parse", commits[i].SHA+"^") == (i==0?preRunHEAD:
    commits[i-1].SHA). Subjects in concept order ["feat: add a","feat: add b","feat: add c"] (proves the
    publish loop blocked on inflight[0] until message 0 — the slowest — finished, THEN published in order).
  - DEPENDS: Task 2 + Task 3.

Task 8: Case 5a — TestDecompose_FastPath_RescueIsolation (through Decompose; S3's is direct-call)
  - DRIVE: Decompose. 3 disjoint files. dcmMessageMatchManifest: {a.txt→"feat: add a"},
    {b.txt→"" (empty ⇒ parse-fail ⇒ RescueError)}, {c.txt→"feat: add c"}. cfg.MaxDuplicateRetries=0.
    stager seam t.Fatal. deps.Out = &buf (dcmOutBuffer).
  - ASSERT: errors.As(err, *DecomposeRescueError) with .Index==1, .Count==3; errors.As(*generate.
    RescueError); errors.Is(generate.ErrRescue); len(result.Commits)==1 (concept 0); buf contains
    "concept 2 of 3" (1-indexed in the message) + "update-ref HEAD"; arbiter NOT called (counter 0).
    (Mirrors MessageRescuePartial :1529 assertions, now through Decompose on the fast-path.)
  - DEPENDS: S3 (MATCHFILE). PLACE near MessageRescuePartial (:1529).

Task 9: Case 5b — TestDecompose_FastPath_CASAbortPartial (uses Task 2/3 match-sleep for the window)
  - DRIVE: Decompose. 3 disjoint files. dcmMessageMatchManifest with per-match sleep so c0 finishes
    FAST and c1 SLOW: {a.txt→"feat: add a", sleepMs:0}, {b.txt→"feat: add b", sleepMs:400},
    {c.txt→"feat: add c", sleepMs:0}. stager seam tFatal. deps.Out=&buf.
  - HEAD-MOVE goroutine (mirror CASAbortPartial :1642's poll+commit-tree idiom): poll until "feat: add
    a" in log AND "feat: add b" NOT yet (c0 published, c1 still in its 400ms message) → armed brief
    sleep → tree=dcmGitOut("rev-parse","HEAD^{tree}"); c=dcmGitOut("commit-tree",tree,"-p","HEAD","-m",
    "external"); dcmGitOut("update-ref","HEAD",c). (commit-tree, NOT --allow-empty, per CASAbortPartial
    :1666's note.) The move lands while c1's message is in flight ⇒ c1's publish CAS fails.
  - ASSERT: errors.As(err, *generate.CASError); errors.Is(git.ErrCASFailed); buf contains "HEAD moved";
    len(result.Commits)==1 (only c0); arbiter NOT called (counter 0).
  - DEPENDS: Task 2 + Task 3.

Task 10: Case 6 — TestDecompose_FastPath_EmptyConceptSkip
  - DRIVE: Decompose. 3 concepts; concept[1].Files is EMPTY (or a path not in T_start). Disjoint still
    holds (empty Files ⇒ vacuously disjoint). a.txt + c.txt are real disjoint changes; b.txt's concept
    stages nothing. dcmMessageMatchManifest (a→msg, c→msg). stager seam tFatal.
  - ASSERT: err==nil; len(result.Commits)==2 (concept 1 FR-M8-skipped — no empty commit); commits' files
    == {a.txt} and {c.txt}; dcmLogCount == initial + 2; chain parents correct (c0→c1, where c1 is the
    c.txt commit — the skipped concept leaves no gap in the CAS chain).
  - DEPENDS: none (S2's sweep owns FR-M8 skip). PLACE near TestDecompose_EmptyConceptSkip (:647).

Task 11: Case 7 — TestDecompose_FastPath_FreezeGuardWired (counting git wrapper)
  - DEFINE (test-local): type countingGit struct { git.Git; n *atomic.Int64 }; func (c *countingGit)
    MergeTrees(ctx, b, o, t string) (string,bool,error) { c.n.Add(1); return c.Git.MergeTrees(ctx,b,o,t) }.
  - DRIVE: Decompose with deps.Git = &countingGit{git.New(repo), &n}. 3 disjoint files (ALL non-skipped).
    disjoint partition; dcmMessageMatchManifest; stager seam tFatal.
  - ASSERT: err==nil; len(result.Commits)==3; n.Load() == 3 (verifyFreezeSubset called once per concept
    in the sweep — MergeTrees is its unique part-B call; the arbiter does NOT call MergeTrees).
    COMMENT the assertion with the exact semantics (runs before the FR-M8 skip ⇒ == len(concepts)).
  - DEPENDS: none. PLACE near TestDecompose_StagerFreezeViolation (:728) / StagerGuardHappyPath (:1185).

Task 12: Case 8 — TestDecompose_FastPath_TooledFlagsLessProvider (two sub-cases)
  - SETUP: Roles.Stager = stubtest.Manifest(bin, stubtest.Options{Out:""}) — a BARE manifest, nil
    TooledFlags (opencode/qwen-code shape). Do NOT inject deps.stager (let the real stageConcept run on
    the shared path). Planner + Message as usual; stager seam NOT set.
  - SUB-CASE disjoint (SUCCEEDS): 3 disjoint files; disjoint partition. Decompose → err==nil; len==3;
    the stager was NEVER invoked (fast-path bypass ⇒ no RenderTooled). (Optional: also assert
    dcmStatusPorcelain=="".)
  - SUB-CASE shared (ERRORS): store.py split across 2 concepts (mirror HunkSplit setup). Decompose →
    errors.Is(err, ErrStagerFailed) AND strings.Contains(err.Error(), "tooled mode requires non-empty
    tooled_flags"). Assert len(result.Commits)==0 (runLoop aborted at the first stager render).
  - DEPENDS: none. PLACE near the dispatch tests / HunkSplit.

Task 13: Case 9 — TestRunLoopFastPath_StartOfRunFreezeExcludesSentinel (DIRECT-CALL)
  - SETUP (mirror TestRunLoopFastPath_ConcurrentPublish :2986 direct-call idiom): born repo; 3 files;
    modify all 3 disjointly. baseTree := dcmGitOut("rev-parse","HEAD^{tree}"); tStart :=
    g.FreezeWorkingTree(ctx, baseTree) (freezes T_start + resets index to baseTree). preRunHEAD :=
    dcmHeadSHA(repo).
  - WRITE SENTINIEL (post-freeze, pre-run): dcmWriteFile(repo, "sentinel.txt", "concurrent change").
  - RUN: commits, _, err := runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false).
    concepts = 3 disjoint {a.go},{b.go},{c.go}. dcmMessageMatchManifest (distinct msgs). stager seam
    NOT needed (fast-path). assert err==nil, len(commits)==3.
  - ASSERT: (a) sentinel.txt in NO commit's Files (for each commits[i].Files, none == "sentinel.txt");
    (b) sentinel.txt remains in the worktree: strings.Contains(dcmStatusPorcelain(repo), "sentinel.txt").
  - DEPENDS: none (direct-call). PLACE near TestRunLoopFastPath_* (S3) / SentinelAfterFreezeExcluded (:2597).

Task 14: VERIFY gates + scope boundaries
  - go build ./... && go vet ./... && gofmt -l internal/ cmd/stubagent/   # all clean
  - go test ./internal/stubtest/ -race -count=1                            # stub contract unchanged
  - go test ./internal/decompose/ -run 'TestDecompose_FastPath|TestRunLoopFastPath_StartOfRun' -race -count=1 -v  # the new suite
  - go test ./internal/decompose/... ./internal/git/... ./internal/config/... -race -count=1   # PRD §5 OUTPUT
  - make test && make lint
  - grep guards (Level 4): runLoopFastPath/runLoop/isFileDisjoint/drains/dispatch/arbiter byte-identical;
    only decompose_test.go + cmd/stubagent/main.go touched.
  - DEPENDS: Tasks 1-13.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the routing oracle — stager t.Fatal proves the fast-path was taken (system_context §6)
deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
    t.Fatalf("fast-path must not invoke the stager (concept %q)", concept.Title)
    return nil
}

// PATTERN: deterministic per-concept message on the concurrent fast-path (S3's helper — REUSE)
messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
    {substr: "a.txt", msg: "feat: add a"},
    {substr: "b.txt", msg: ""},            // empty ⇒ parse-fail ⇒ RescueError (with MaxDuplicateRetries=0)
    {substr: "c.txt", msg: "feat: add c"},
})
messageM.Env["STAGECOACH_STUB_SLEEP_MS"] = "100"   // widen the concurrency window

// PATTERN (Task 3 extension): per-match sleep — later concepts finish FIRST (case 4)
messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
    {substr: "a.txt", msg: "feat: add a", sleepMs: 300},  // message 0 finishes LAST
    {substr: "b.txt", msg: "feat: add b", sleepMs: 200},
    {substr: "c.txt", msg: "feat: add c", sleepMs: 100},  // message 2 finishes FIRST
})

// PATTERN: concept isolation assertion (case 1) — diff-tree vs parent == concept's files
for i, c := range result.Commits {
    parent := preRunHEAD
    if i > 0 { parent = result.Commits[i-1].SHA }
    got := dcmGitOut(t, repo, "diff-tree", "--no-commit-id", "--name-only", "-r", c.SHA, parent)
    if !sortedEq(got, conceptFilesFor(c)) { t.Errorf("commit[%d] not isolated: got %q", i, got) }
}

// PATTERN: case 7 counting git wrapper (Deps.Git is the git.Git interface)
type countingGit struct {
    git.Git
    n *atomic.Int64
}
func (c *countingGit) MergeTrees(ctx context.Context, b, o, t string) (string, bool, error) {
    c.n.Add(1)
    return c.Git.MergeTrees(ctx, b, o, t)
}
// ... deps.Git = &countingGit{git.New(repo), &n} ...; assert n.Load() == 3

// PATTERN: case 8 tooled_flags-less (BARE manifest ⇒ nil TooledFlags)
roles.Stager = stubtest.Manifest(bin, stubtest.Options{Out: ""})  // NOT tooledStubManifest
// disjoint → fast-path bypass → succeeds; shared → stageConcept → RenderTooled → ErrStagerFailed

// PATTERN: case 5b HEAD-move window via per-match sleep (mirror CASAbortPartial :1642 poll+commit-tree)
messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
    {substr: "a.txt", msg: "feat: add a", sleepMs: 0},    // c0 publishes fast
    {substr: "b.txt", msg: "feat: add b", sleepMs: 400},  // c1 in flight → the window
    {substr: "c.txt", msg: "feat: add c", sleepMs: 0},
})
go func() { /* poll "feat: add a" in log & "feat: add b" not yet → commit-tree/update-ref HEAD */ }()
```

### Integration Points

```yaml
NO production source changes. ONE test-only binary extended (additively). Tests added.

CODE (test-only, additive):
  - cmd/stubagent/main.go — +STAGECOACH_STUB_INTERVAL_FILE (Task 1) + per-match sleep (Task 2)
  - internal/decompose/decompose_test.go — +~9 Test* (Tasks 4-13); messageMatchRule gains optional sleepMs (Task 3)

REUSED (BYTE-IDENTICAL — S1/S2/S3/S4 own them; S5 only EXERCISES):
  - runLoopFastPath (decompose.go:662), isFileDisjoint (:450), runLoop (:479), drainMsgs (:825)
  - Decompose dispatch gate (S4, :237) + error block + arbiter phase + rereadFinalCommits
  - verifyFreezeSubset (stager.go:168) — case 7 counts its MergeTrees call
  - dcmMessageMatchManifest + messageMatchRule (decompose_test.go:146, S3) — concurrency-safe messages
  - all dcm* helpers, stubtest.Manifest/Build/NewScript, tooledStubManifest, stagePartialBlob

UNCHANGED: decompose.go (beyond S4's dispatch — already landed); stager.go; roles.go; arbiter.go;
  chain.go; message.go; planner.go; the git.Git interface; internal/stubtest/*; any config/CLI; docs (S6).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...                       # the new tests + stub hooks compile
go vet ./...
gofmt -l internal/decompose/ cmd/stubagent/   # Expected: empty.
make lint                           # Expected: zero errors.
```

### Level 2: Unit / Component Tests (the suite itself + no regressions)

```bash
# The new fast-path regression suite (the deliverable)
go test ./internal/decompose/ -run 'TestDecompose_FastPath|TestRunLoopFastPath_StartOfRun' -race -count=1 -v
# Expected: ALL PASS, ZERO race reports.

# Stub contract unchanged (Tasks 1-2 are additive/default-off)
go test ./internal/stubtest/ -run TestStub -race -count=1 -v
# Expected: PASS (proves INTERVAL_FILE + per-match sleep didn't break the existing stub behavior).

# S3's fast-path tests still green (Task 3's messageMatchRule change is backward-compatible)
go test ./internal/decompose/ -run 'TestRunLoopFastPath_ConcurrentPublish|TestRunLoopFastPath_RescueIsolation' -race -count=1 -v
# Expected: PASS.

# S4's dispatch tests still green
go test ./internal/decompose/ -run 'TestDecompose_Dispatch' -race -count=1 -v
# Expected: PASS.
```

### Level 3: Integration / Full Suite (PRD §5 OUTPUT)

```bash
# PRD §5: "Run the full `go test ./internal/decompose/... ./internal/git/... ./internal/config/...` green."
go test ./internal/decompose/... ./internal/git/... ./internal/config/... -race -count=1
# Expected: ALL PASS.

# Whole project
make test
# Expected: ALL PASS.
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Grep guard: the new suite exists with the expected names
grep -nE 'func (TestDecompose_FastPath_|TestRunLoopFastPath_StartOfRun)' internal/decompose/decompose_test.go
# Expected: ≥9 matches (cases 1-9).

# Grep guard: the two stub hooks are additive + default-off
grep -n 'STAGECOACH_STUB_INTERVAL_FILE' cmd/stubagent/main.go          # Expected: ≥2 (the getenv + the append)
grep -n 'sleepMs' cmd/stubagent/main.go                                 # Expected: the selectMatched parse + the sleep

# Grep guard: NO non-test / non-stubagent source changed
git diff --name-only
# Expected: ONLY internal/decompose/decompose_test.go + cmd/stubagent/main.go.
git diff internal/decompose/decompose.go internal/decompose/stager.go internal/decompose/roles.go \
          internal/decompose/arbiter.go internal/decompose/chain.go internal/decompose/message.go
# Expected: EMPTY (byte-identical).

# Grep guard: runLoopFastPath/runLoop/isFileDisjoint/drains byte-identical (S5 only EXERCISES them)
git diff internal/decompose/decompose.go | grep -E '^-' | grep -v '^---'
# Expected: EMPTY (S4's dispatch already landed before S5; S5 adds no - lines to decompose.go).

# Grep guard: messageMatchRule's sleepMs field is OPTIONAL (S3's callers use named fields or it defaults 0)
grep -n 'messageMatchRule{' internal/decompose/decompose_test.go
# Expected: S3's call sites + S5's; none break (named fields or 0-default third field).

# Race guard: the suite ran under -race with zero reports (the fast-path is concurrent)
go test ./internal/decompose/ -run 'TestDecompose_FastPath|TestRunLoopFastPath' -race -count=1
# Expected: PASS, "WARNING: DATA RACE" absent from output.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/decompose/ cmd/stubagent/` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass
- [ ] The new suite passes under `-race` with ZERO race reports

### Feature Validation (the 9 cases)
- [ ] Case 1: disjoint → 3 commits, stager t.Fatal never trips, concept isolation (diff-tree==Files),
      CAS-ordered parents, AND T_start completeness BOTH sub-cases (arbiter skipped counter 0 / arbiter
      folds counter 1, arbiter-commit tree==T_start)
- [ ] Case 2: shared file → runLoop, stager called for both concepts, tip reconstructs tStart (== runLoop)
- [ ] Case 3: HARD interval overlap (≥1 consecutive start[j]<end[i]); corroborating "launched N concurrent" log
- [ ] Case 4: per-match sleep (msg 0 slowest) → chain STILL strictly ordered preRunHEAD→c0→c1→c2
- [ ] Case 5a: rescue at position 1 → *DecomposeRescueError(Index1), 1 partial commit, "concept 2 of 3",
      arbiter skipped
- [ ] Case 5b: CAS at position 1 → *CASError, 1 partial commit, "HEAD moved", arbiter skipped
- [ ] Case 6: empty concept → skipped (2 commits), no empty commit, CAS chain gap-free
- [ ] Case 7: MergeTrees count == 3 (verifyFreezeSubset wired, once per concept)
- [ ] Case 8: disjoint on TooledFlags-less stager → succeeds; shared → ErrStagerFailed +
      "tooled mode requires non-empty tooled_flags"
- [ ] Case 9: post-freeze sentinel in NO commit + remains in worktree

### Code Quality & Scope
- [ ] Tests mirror existing idioms (dcm* helpers, dcmMessageMatchManifest, arbiter counter, poll+commit-tree)
- [ ] New tests placed near their conceptual siblings (HappyPath/HunkSplit/EmptyConceptSkip/StagerFreezeViolation/
      MessageRescuePartial/CASAbortPartial/SentinelAfterFreezeExcluded/Dispatch)
- [ ] The two stub hooks are ADDITIVE + default-off (internal/stubtest/stubtest_test.go still green)
- [ ] messageMatchRule's sleepMs is OPTIONAL + backward-compatible (S3's tests still green)
- [ ] `runLoopFastPath`/`runLoop`/`isFileDisjoint`/`drainMsg`/`drainMsgs` BYTE-IDENTICAL
- [ ] The `Decompose` dispatch gate + error block + arbiter phase + `rereadFinalCommits` UNCHANGED
- [ ] `arbiter.go`/`chain.go`/`roles.go`/`message.go`/`stager.go`/`decompose.go` UNCHANGED
- [ ] NO new config key / CLI flag / docs (S6 owns docs)

---

## Anti-Patterns to Avoid

- ❌ Don't use `dcmMessageScriptManifest`/`NewScript` for fast-path MESSAGES — the file-backed counter
  RACES under concurrent launch (N processes read/increment one counter). Use `dcmMessageMatchManifest`
  (input-derived, per-process stdin) for ANY per-concept message behavior (cases 3/4/5a/5b).
- ❌ Don't assert `verifyFreezeSubset` count == (len(concepts) − numSkipped). It runs BEFORE the FR-M8
  skip, so count == len(concepts). Use a 3-disjoint-non-skipped scenario ⇒ count == 3.
- ❌ Don't inject `deps.stager` on case 8's shared path — that masks the `tooled mode requires non-empty
  tooled_flags` error (the seam short-circuits `stageConcept`). Leave it nil so the REAL render error fires.
- ❌ Don't use `git commit --allow-empty` to move HEAD in case 5b — it captures the staged index and can
  trip "nothing to do" instead of "HEAD moved". Use `commit-tree <HEAD^{tree}> -p HEAD` + `update-ref HEAD`
  (mirror CASAbortPartial :1666).
- ❌ Don't try to write the case-9 sentinel "through Decompose" — there's no Go seam between FreezeWorkingTree
  and runLoopFastPath (planner is an external stub; fast-path has no stager seam). Call runLoopFastPath
  directly (S3's idiom) for exact freeze→sentinel→run control.
- ❌ Don't make case 3 flaky by asserting on raw elapsed time alone — that's S3's SOFT gate. Use the
  INTERVAL_FILE hook for HARD per-goroutine interval overlap. A uniform 100ms sleep makes all N overlap
  robustly (ms-scale exec jitter << 100ms).
- ❌ Don't edit `runLoopFastPath`/`runLoop`/`isFileDisjoint`/`drainMsg`/`drainMsgs` or the dispatch/arbiter
  — S5 EXERCISES them. If a case reveals a real bug, STOP and surface it (do not patch production code
  inside a "tests" PRP without a separate PRP).
- ❌ Don't duplicate S3's `_ConcurrentPublish`/`_RescueIsolation` or S4's dispatch tests. Cases 3/5a
  STRENGTHEN/extend (interval overlap / through-Decompose); cases 4/6/7/8/9 are net-new. If a case would
  exactly re-test S3/S4, narrow it to the distinct angle (see the distinctness table in research/findings.md §4).
- ❌ Don't add docs (how-it-works.md) — that's S6. Don't add config/flag — FR-M13 is config-free.
- ❌ Don't break backward compat in the stub hooks. INTERVAL_FILE fires ONLY when its env is set;
  per-match sleep's 3rd field is OPTIONAL (2-field lines ⇒ sleepMs 0 ⇒ S3's behavior unchanged). Re-run
  internal/stubtest/stubtest_test.go + S3's fast-path tests after every stub edit.

---

## Confidence Score: 9/10

One-pass success is very high. Every case has a verified drive-path, named reusable helpers (with
file:line), the exact stub manifest to build (including the two new hooks, both specified additively and
backward-compatible), exact assertion commands (diff-tree for isolation, commit-tree/update-ref for the
CAS window, MergeTrees count for the freeze guard, the exact `tooled mode requires non-empty
tooled_flags` string), and the precise count/error/timing semantics. The stub-agent extension is an
established pattern in this workstream (S3 already did it). The -1 is for the **case 5b CAS-window
timing**: injecting a HEAD move between c0-publish and c1-publish depends on the per-match-sleep window
holding (c0 fast, c1 slow) + a poll-and-commit-tree goroutine landing inside it — it mirrors the proven
CASAbortPartial (:1642) idiom but is the most timing-sensitive case, so a rare CI-jitter flake is
possible. Mitigated by: (a) a generous 400ms c1 sleep, (b) the poll+armed-sleep pattern from
CASAbortPartial, (c) commit-tree (deterministic tree) not --allow-empty, (d) the deadline-guarded
goroutine that leaves HEAD alone if the window is missed (the test's own assertions fail loudly rather
than race a finished run). The other 8 cases are deterministic by construction (input-derived message
matching, atomic interval appends, a counting wrapper, a controlled direct-call sentinel). The two stub
hooks are default-off and re-validated by the existing stub + S3 test suites.