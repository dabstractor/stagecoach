# Research Findings — P1.M3.T2.S1: Test fast-path with same-subject collision (BUG-002 regression)

## 0. What this task is — a regression TEST for an ALREADY-LANDED fix

BUG-002 (file-disjoint fast-path loses cross-concept duplicate-subject detection) was FIXED in
P1.M2.T1.S1 (Complete). This task writes the regression TEST that proves the fix: when two disjoint
concepts' message agent emits the SAME subject, no two PUBLISHED commits share a subject. The fix is in
the tree; my deliverable is the test only (no production change).

## 1. The BUG-002 fix (LANDED — what the test must observe)

`internal/decompose/decompose.go` runLoopFastPath (verified by reading :759-865):
- **:767** `seenSubjects, _ := messageRecentSubjects(ctx, deps.Git, isUnborn)` — the cross-concept dedupe
  accumulator, seeded with pre-run history. (History-fetch error ignored — degrades to "no history".)
- The N message generations run CONCURRENTLY (seedRejections=nil — cross-concept dedupe is the serial
  loop's job, :738).
- **Serial publish loop (CAS order, :776+):** for each concept in order:
  - extract `subject := generate.ExtractSubject(res.msg)`.
  - **:819** `if generate.IsDuplicate(subject, seenSubjects)` → collision:
    - **:820** regenerate `generateMessageCore(ctx, deps, sc.prevTree, sc.tree, seenSubjects)`.
    - if regen returns a `*generate.RescueError` (regErr) → **:830** return `&DecomposeRescueError{Rescue:
      &fixed, …, Commits: commits}` (concept[i] rescued; commits 0..i-1 stand; FR-M12).
    - if regen succeeds → `res.msg = regenerated; subject = …`.
    - **:844** if STILL a duplicate → **:846** return `&DecomposeRescueError{Rescue: {Kind: ErrRescue, …},
      Commits: commits}` (belt-and-suspenders for a stub that ignores seedRejections).
  - **:859** `seenSubjects = append(seenSubjects, subject)` — the accepted subject is now visible to later
    concepts.

**Determinism for the test:** the serial loop processes concepts in ORDER (i=0, then i=1). concept 0 is
always published first; concept 1 always collides. So the outcome is fully deterministic regardless of
goroutine scheduling.

## 2. The deterministic expected outcome (rescue — Design A)

With `dcmMessageMatchManifest` returning the SAME subject ("chore: update thing") for BOTH concepts
(single-response per rule — the stub is NOT stateful), the run proceeds:
1. Both concepts generate concurrently; each emits "chore: update thing" (passes per-concept dedupe — not
   in pre-run history; seedRejections=nil in the concurrent phase).
2. Serial loop, concept 0: "chore: update thing" not in seenSubjects (pre-run history) → PUBLISHED.
   seenSubjects now = [pre-run history…, "chore: update thing"].
3. Serial loop, concept 1: "chore: update thing" IS in seenSubjects → **collision** → regenerate.
4. Regen: the stub returns "chore: update thing" AGAIN (single-response). Either generateMessageCore's
   own dedupe flags it (if seedRejections feeds the check) and exhausts retries → RescueError; OR regen
   returns it and the :844 outer check catches it. **BOTH paths → concept 1 is RESCUED.**
5. runLoopFastPath returns `&DecomposeRescueError{Commits: [concept 0], Index: 1, …}`.

So: `err` is `*DecomposeRescueError`; `len(err.Commits) == 1`; `err.Commits[0].Subject == "chore: update
thing"`; `err.Index == 1`; git log has exactly ONE "chore: update thing".

**Why MaxDuplicateRetries=0 is the simpler/faster option:** with the default (3), the regen loop tries
MaxDuplicateRetries+1=4 times before returning RescueError (4 stub invocations). With
`cfg.MaxDuplicateRetries=0`, regen rescues on the FIRST collision (1 invocation) — faster + equally
deterministic. The contract explicitly offers this ("set cfg.MaxDuplicateRetries=0 to force immediate
rescue (simpler)"). The default also works; both reach the same DecomposeRescueError outcome.

(Design B — a STATEFUL stub that returns a DIFFERENT subject on regen — would let concept 1 PUBLISH with a
distinct subject → err==nil, 2 commits, distinct subjects. The contract allows it but it needs a custom
stateful stub; Design A needs none. I specify Design A as primary.)

## 3. The test infrastructure — all helpers exist in decompose_test.go

Verified (`grep` + read). `package decompose` internal tests share these helpers:
- `dcmInitRepo(t, repo)`, `dcmWriteFile`, `dcmRunGit`, `dcmCommitRaw`, `dcmGitOut`, `dcmHeadSHA`,
  `dcmLogOneline`, `dcmLogCount` (decompose_test.go:70-88 + the dcm* block).
- `dcmMessageMatchManifest(t, bin, []messageMatchRule)` (:147) + `messageMatchRule{substr, msg, sleepMs}`
  (:172) — INPUT-DERIVED, concurrency-safe (each stub process inspects its OWN stdin diff, emits the first
  matching rule's msg). The matchfile is `substr|msg[\n]…` written to a temp file; the stub reads
  STAGECOACH_STUB_MATCHFILE.
- `dcmDeps(t, repo, roles)` / `dcmDepsWithConfig(t, repo, roles, cfg)` (:194/203) — minimal Deps.
- `stubtest.Build(t)` — compiles cmd/stubagent once per process (cached).
- `prompt.PlannerCommit{Title, Files []string}` — the concepts slice element type.

## 4. The reference skeleton — TestRunLoopFastPath_ConcurrentPublish (decompose_test.go:3150)

The proven fast-path test to MIRROR (read in full). The exact recipe:
```go
bin := stubtest.Build(t)
repo := t.TempDir()
dcmInitRepo(t, repo)
// seed a base commit (BORN repo → baseTree = HEAD^{tree})
dcmWriteFile(t, repo, "a.txt", "aaa\n"); dcmWriteFile(t, repo, "b.txt", "bbb\n")
dcmRunGit(t, repo, "add", "a.txt", "b.txt")
dcmCommitRaw(t, repo, "initial")
// disjoint working-tree change set (the FR-M13 disjoint partition)
dcmWriteFile(t, repo, "a.txt", "AAA\n"); dcmWriteFile(t, repo, "b.txt", "BBB\n")
g := git.New(repo); ctx := context.Background()
baseTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}")
tStart, err := g.FreezeWorkingTree(ctx, baseTree); if err != nil { t.Fatalf("freeze: %v", err) }
preRunHEAD := dcmHeadSHA(t, repo)
// message manifest + Deps + concepts
messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{ … })
deps := Deps{Git: g, Config: cfg, Roles: RoleManifests{Message: messageM}, Verbose: nil}
concepts := []prompt.PlannerCommit{ … }
commits, _, err := runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)
// assert
```
`runLoopFastPath` signature (decompose.go:680): `func runLoopFastPath(ctx, deps Deps, concepts
[]prompt.PlannerCommit, baseTree, tStart, preRunHEAD string, isUnborn bool) ([]CommitResult, []ChainEntry,
error)`. isUnborn=false (BORN repo — it has the "initial" commit).

`CommitResult` (decompose.go:53): `{SHA, Subject, Message string; Files []git.FileChange}` — assert on
`.Subject`.

## 5. No conflict with the parallel BUG-001 test (P1.M3.T1.S1)

The parallel item (read its PRP) writes `TestRunLoopFastPath_EditGate_NoCrossContamination`:
- cfg.Edit=**true**, GIT_EDITOR=true (no-op editor), files a.go/b.go, **DISTINCT** msgs ("feat: add a" /
  "feat: add b"), asserts err==nil + 2 commits + each matches its own concept.

My test (BUG-002) is cleanly separated:
- Test name: `TestRunLoopFastPath_DuplicateSubjectDedupe` (per the contract).
- cfg.Edit=**false** (isolates BUG-002 from the BUG-001 editor path).
- files a.txt/b.txt (per the contract — different from BUG-001's a.go/b.go).
- **IDENTICAL** msgs ("chore: update thing" for both rules).
- asserts `*DecomposeRescueError` + 1 commit + no dup subjects.

Different name, different files, different cfg.Edit, different manifest (same vs distinct), different
assertion shape (rescue vs success). Both mirror the same skeleton but are fully independent — zero
conflict, zero shared mutable state.

## 6. Scope boundaries

- **P1.M2.T1.S1** (DONE) — the seenSubjects fix my test proves. No production change from me.
- **P1.M3.T1.S1** (parallel, Implementing) — the BUG-001 test. Different test, different scenario. No
  overlap (see §5).
- **P1.M3.T3.S1** (planned) — tighten the runLoopFastPath concurrency-safety comment. Separate.
- **P1.M3.T4.S1** (planned) — changeset docs sync. NOT here (contract: "DOCS: none — test-only file").
- This item touches ONLY: `internal/decompose/decompose_test.go` (ADD one test function). NO production
  change, NO new file (the test goes in the existing decompose_test.go alongside the other
  TestRunLoopFastPath_* tests), NO edit to decompose.go/message.go/generate.go, NO PRD/task file.

## 7. Validation commands (verified)

```bash
go test ./internal/decompose/ -run 'TestRunLoopFastPath_DuplicateSubjectDedupe' -race -v   # the new test GREEN
go test ./internal/decompose/ -race          # full decompose regression (incl. BUG-001 test + all fast-path tests)
go vet ./internal/decompose/...
gofmt -l internal/decompose/decompose_test.go   # empty
make test ; make lint
git status --porcelain                          # ONLY internal/decompose/decompose_test.go
```

`internal/decompose` is NOT in the coverage-gate list (Makefile gates `internal/{git,provider,generate,
config}` only) — no coverage-threshold pressure (it's a test-only addition anyway).

## 8. The race-detector angle (-race)

The fast-path launches N goroutines; the test should run under `-race` (the validation command includes it).
The fix's `seenSubjects` is mutated ONLY in the serial publish loop (one goroutine) — never in the
concurrent generation goroutines (they use seedRejections=nil) — so there's no data race on seenSubjects.
The BUG-001 fix (EditMessage in the serial loop) similarly removed the shared-file race. So `-race` should
be clean. If `-race` ever flags anything, it's a REAL concurrency bug in the fix, not a test artifact —
surface it.