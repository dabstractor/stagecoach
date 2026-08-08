name: "P1.M3.T2.S1 — Regression test: fast-path same-subject collision → no duplicate subjects (BUG-002)"
description: >
  A TEST-ONLY regression for BUG-002 (file-disjoint fast-path lost cross-concept duplicate-subject
  detection), whose fix is ALREADY LANDED (P1.M2.T1.S1: the `seenSubjects` accumulator + serial-loop
  collision check + regenerate/rescue in runLoopFastPath, internal/decompose/decompose.go:767/819/820/844).
  This task adds ONE test function to internal/decompose/decompose_test.go (NO new file, NO production
  change): `TestRunLoopFastPath_DuplicateSubjectDedupe`. It mirrors the proven
  `TestRunLoopFastPath_ConcurrentPublish` skeleton (decompose_test.go:3150): a BORN repo (seed "initial"),
  2 pairwise-disjoint working-tree changes (a.txt, b.txt), FreezeWorkingTree→tStart, then
  `runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)` directly. The message stub
  (`dcmMessageMatchManifest`, concurrency-safe + input-derived) returns the SAME subject for BOTH concepts
  (`{substr:"a.txt", msg:"chore: update thing"}, {substr:"b.txt", msg:"chore: update thing"}`), and
  `cfg.Edit = false` (isolates BUG-002 from the BUG-001 editor path). Because the stub is single-response
  (regeneration returns the same subject), the DETERMINISTIC outcome is RESCUE for concept 1: concept 0
  publishes "chore: update thing" (appended to seenSubjects), concept 1 collides → regenerate → still
  collides → `*DecomposeRescueError` with concept 0 as the only published commit. The test asserts:
  errors.As(err, &dre) (a *DecomposeRescueError), len(dre.Commits)==1, dre.Commits[0].Subject=="chore:
  update thing", dre.Index==1, AND git log (dcmLogOneline) contains exactly ONE "chore: update thing" (the
  core US7/FR30-33 guarantee: no two published commits share a subject). Setting cfg.MaxDuplicateRetries=0
  makes regen rescue on the first collision (1 stub invocation, faster — the contract's "simpler" option);
  the default (3) also works. NOT in scope: the production fix (DONE), the BUG-001 test (P1.M3.T1.S1,
  parallel — different name/files/cfg.Edit/manifest, zero conflict), the concurrency-comment tightening
  (P1.M3.T3.S1), docs (P1.M3.T4.S1 — contract: "DOCS: none — test-only file"). go.mod unchanged.

---

## Goal

**Feature Goal**: Add a regression test proving the BUG-002 fix: on the file-disjoint fast-path, when two
disjoint concepts' message agent emits the SAME subject, the run does NOT publish two commits with that
identical subject (US7 / FR30-FR33). The fix (P1.M2.T1.S1 — `seenSubjects` cross-concept dedupe in
runLoopFastPath's serial publish loop) is LANDED; this test locks it against regression.

**Deliverable**: ONE new test function `TestRunLoopFastPath_DuplicateSubjectDedupe` added to
`internal/decompose/decompose_test.go` (the existing file, alongside the other `TestRunLoopFastPath_*`
tests). NO new file, NO production change.

**Success Definition**:
- `TestRunLoopFastPath_DuplicateSubjectDedupe` exercises `runLoopFastPath` with 2 disjoint concepts whose
  message stub returns the same subject ("chore: update thing") for both, `cfg.Edit=false`.
- The test asserts the deterministic rescue outcome: `errors.As(err, &dre)` where `dre` is a
  `*DecomposeRescueError`; `len(dre.Commits) == 1`; `dre.Commits[0].Subject == "chore: update thing"`;
  `dre.Index == 1`; and `dcmLogOneline(t, repo)` contains exactly ONE "chore: update thing" (the base
  commit is "initial", so the log has 2 distinct-subject lines).
- The core invariant is asserted: no two PUBLISHED commits share a subject (trivially true with 1
  published commit, but stated as the US7 guarantee the test pins).
- `go test ./internal/decompose/ -run 'TestRunLoopFastPath_DuplicateSubjectDedupe' -race -v` GREEN.
- `go test ./internal/decompose/ -race` GREEN (full decompose regression incl. the parallel BUG-001 test +
  all existing fast-path tests).
- Scope: `git status --porcelain` == `internal/decompose/decompose_test.go` ONLY. NO production change.

## User Persona (if applicable)

**Target User**: The stagecoach maintainers (the regression test guards BUG-002 from recurring) and the
PRD's US7 user story ("I want Stagecoach to guarantee no duplicate subjects, so that git log doesn't
contain the same line twice"). End users never run this test directly.

**Use Case**: A future refactor of runLoopFastPath that accidentally re-introduces the
generate-all-before-publish dedupe gap (or removes the seenSubjects accumulator) → this test FAILS
("expected *DecomposeRescueError / expected 1 published commit / git log has 2 'chore: update thing'"),
catching the regression before release.

**User Journey**: maintainer edits runLoopFastPath → `go test ./internal/decompose/` → the
DuplicateSubjectDedupe test is the canary that proves cross-concept dedupe still holds.

**Pain Points Addressed**: BUG-002 — the fast-path's concurrent-generate-then-serial-publish ordering
meant each concept's per-concept dedupe saw only pre-run history, so two siblings with the same emitted
subject both published. The fix closed it; this test proves it stays closed.

## Why

- **BUG-002 regression guard**: the fix (P1.M2.T1.S1) is LANDED but had NO test (the existing suite uses
  per-concept DISTINCT messages and never exercises a same-subject collision — confirmed in the bug
  report). This test is the missing coverage.
- **US7 / FR30-FR33 is a headline guarantee**: "no duplicate subjects in git log." A regression that
  silently re-opens the gap is exactly the kind of subtle concurrency bug a dedicated test must catch —
  the existing distinct-message tests cannot.
- **Deterministic by construction**: the serial publish loop processes concepts in order (0 then 1), so
  concept 0 always publishes first and concept 1 always collides — no goroutine-scheduling nondeterminism
  in the ASSERTION (the rescue outcome is fixed). The race detector (`-race`) additionally guards the
  fix's concurrency safety.
- **No-conflict with BUG-001 test**: different test name, different files (a.txt/b.txt vs a.go/b.go),
  cfg.Edit=false vs true, identical vs distinct manifest messages, rescue vs success assertions. Both
  mirror the same skeleton but are fully independent (findings §5).

## What

**User-visible behavior**: none (test-only).

**Technical change**: one test function added to the existing `internal/decompose/decompose_test.go`.

### Success Criteria
- [ ] `TestRunLoopFastPath_DuplicateSubjectDedupe` exists in `internal/decompose/decompose_test.go`.
- [ ] It builds a BORN repo (seed "initial" commit), creates 2 disjoint working-tree changes (a.txt, b.txt),
      freezes tStart via `g.FreezeWorkingTree(ctx, baseTree)`, and calls
      `runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)` directly.
- [ ] It uses `dcmMessageMatchManifest(t, bin, []messageMatchRule{{substr:"a.txt", msg:"chore: update
      thing"}, {substr:"b.txt", msg:"chore: update thing"}})` — the SAME subject for both concepts.
- [ ] It sets `cfg := config.Defaults(); cfg.Edit = false` (isolates BUG-002 from the BUG-001 editor path);
      recommends `cfg.MaxDuplicateRetries = 0` for the faster/immediate-rescue variant (default 3 also works).
- [ ] `concepts := []prompt.PlannerCommit{{Title:"c1", Files:[]string{"a.txt"}}, {Title:"c2", Files:[]string{"b.txt"}}}`.
- [ ] It asserts: `errors.As(err, &dre)` where `dre *DecomposeRescueError`; `len(dre.Commits) == 1`;
      `dre.Commits[0].Subject == "chore: update thing"`; `dre.Index == 1`.
- [ ] It asserts `strings.Count(dcmLogOneline(t, repo), "chore: update thing") == 1` (exactly one published
      commit with that subject; the base "initial" is the only other line).
- [ ] `go test ./internal/decompose/ -run 'TestRunLoopFastPath_DuplicateSubjectDedupe' -race -v` GREEN.
- [ ] `go test ./internal/decompose/ -race` GREEN (full regression); `make test` + `make lint` clean.
- [ ] `git status --porcelain` == `internal/decompose/decompose_test.go` ONLY. NO production change.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim skeleton to mirror (TestRunLoopFastPath_ConcurrentPublish, with the line-precise
repo setup + freeze + runLoopFastPath call), the exact 4 deltas for BUG-002 (cfg.Edit=false, files
a.txt/b.txt, identical manifest msgs, the rescue-case assertions), the deterministic-outcome explanation
(concept 0 publishes → concept 1 collides → regen → rescue), every helper's signature + file location
(dcmInitRepo/dcmWriteFile/dcmRunGit/dcmCommitRaw/dcmGitOut/dcmHeadSHA/dcmLogOneline/dcmMessageMatchManifest/
messageMatchRule/dcmDeps, stubtest.Build), the runLoopFastPath signature, the CommitResult/DecomposeRescueError
field shapes, the no-conflict analysis vs the parallel BUG-001 test, and 5 grep guards.

### Documentation & References

```yaml
# MUST READ — the authoritative BUG-002 test design (Goal/Setup/Assertions)
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/architecture/test_strategy.md
  section: "BUG-002 Regression Test"
  why: "Gives the exact Goal (no duplicate subjects), Setup (2 disjoint concepts, SAME-subject manifest,
        cfg.Edit=false), and Assertions (rescue case: *DecomposeRescueError with concept 0 published +
        concept 1 rescued, OR both published with DIFFERENT subjects; in either case no two published
        commits share a subject). Plus the runLoopFastPath test skeleton (7 steps) and the helper catalog."
  critical: "cfg.Edit=false ISOLATES BUG-002 from BUG-001 (the editor race). The single-response stub makes
             RESCUE the expected outcome (Design A) — no stateful stub needed."

# MUST READ — codebase-specific findings for THIS item (the deterministic-outcome proof + no-conflict analysis)
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/P1M3T2S1/research/findings.md
  why: "§1 the LANDED fix (seenSubjects at :767, collision check :819, regen :820, still-collides rescue
        :844, append :859) — what the test observes; §2 the DETERMINISTIC rescue outcome (concept 0
        publishes → concept 1 collides → regen returns same → rescue; why MaxDuplicateRetries=0 is simpler);
        §3 every helper's location + signature; §4 the verbatim skeleton (TestRunLoopFastPath_ConcurrentPublish);
        §5 the no-conflict analysis vs the BUG-001 test; §6 scope fences; §7 validation cmds; §8 the -race angle."
  critical: "The serial loop processes concepts IN ORDER (0 then 1), so concept 0 ALWAYS publishes first and
             concept 1 ALWAYS collides — the rescue outcome is deterministic regardless of goroutine
             scheduling. Assert *DecomposeRescueError + 1 commit + dre.Index==1 + exactly 1 'chore: update
             thing' in git log."

# MUST READ — the LANDED fix the test proves (read the dedupe + regen + rescue logic)
- file: internal/decompose/decompose.go
  why: "Lines 767 (seenSubjects seed), 819 (IsDuplicate collision check), 820 (generateMessageCore regen
        with seenSubjects), 830 (regen-RescueError → DecomposeRescueError), 844 (still-collides →
        DecomposeRescueError belt-and-suspenders), 859 (append accepted subject). This is EXACTLY what the
        test exercises: concept 0 publishes + appends; concept 1 collides → regen → rescue. Line 680 is the
        runLoopFastPath signature; :72 DecomposeResult; :83 DecomposeRescueError{Rescue, ConceptTitle, Index,
        Count, Commits}; :53 CommitResult{SHA, Subject, Message, Files}."
  pattern: "The fix returns &DecomposeRescueError{Commits: commits (the partial 0..i-1), Index: sc.idx} on a
            collision that regen can't resolve. The test asserts errors.As to that type + reads .Commits/.Index."
  gotcha: "Do NOT edit decompose.go — the fix is LANDED. This task is test-only."

# MUST READ — the verbatim skeleton to mirror (the proven fast-path test)
- file: internal/decompose/decompose_test.go
  why: "TestRunLoopFastPath_ConcurrentPublish (line 3150) is the canonical runLoopFastPath test: BORN repo
        (seed base commit), disjoint working-tree changes, FreezeWorkingTree→tStart, dcmMessageMatchManifest,
        Deps{Git, Config, Roles:{Message}, Verbose}, concepts slice, runLoopFastPath(...) call, assertions on
        commits + git log. MIRROR this skeleton with the 4 BUG-002 deltas (cfg.Edit=false, a.txt/b.txt,
        identical msgs, rescue assertions). dcmMessageMatchManifest (:147), messageMatchRule (:172), dcmDeps
        (:194), dcmGitOut/dcmHeadSHA/dcmLogOneline (:70/76/82) are all in THIS file — reuse them."
  pattern: "bin := stubtest.Build(t); repo := t.TempDir(); dcmInitRepo; seed base; disjoint dirty files;
            baseTree := dcmGitOut('rev-parse','HEAD^{tree}'); tStart := g.FreezeWorkingTree(ctx, baseTree);
            preRunHEAD := dcmHeadSHA; messageM := dcmMessageMatchManifest(...); deps := Deps{...};
            commits, _, err := runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false);
            assert."
  gotcha: "Add the test to THIS file (internal/decompose/decompose_test.go), NOT a new file. Match its import
           block (it already imports context, errors, strings, bytes, testing, config, git, prompt, stubtest,
           ui, provider). Place the new test NEAR the other TestRunLoopFastPath_* tests."

# CONTEXT — the parallel BUG-001 test (avoid conflict; different scenario)
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/P1M3T1S1/PRP.md
  why: "Defines TestRunLoopFastPath_EditGate_NoCrossContamination: cfg.Edit=TRUE, GIT_EDITOR=true, files
        a.go/b.go, DISTINCT msgs ('feat: add a'/'feat: add b'), asserts err==nil + 2 commits + each matches
        its own concept. My BUG-002 test is cleanly separated: cfg.Edit=FALSE, files a.txt/b.txt, IDENTICAL
        msgs, rescue assertions. Different name/files/cfg.Edit/manifest/assertion-shape — zero conflict."
  critical: "Do NOT reuse the BUG-001 test's DISTINCT-message manifest or a.go/b.go files. BUG-002 needs the
             IDENTICAL-message manifest + a.txt/b.txt + cfg.Edit=false. Both mirror ConcurrentPublish but
             diverge on exactly those axes."

# CONTEXT — PRD US7 / FR30-33 (the guarantee this test pins)
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/prd_snapshot.md
  section: "Overview (BUG-002) + Recommendations (regression test #2: fast-path same-subject collision)"
  why: "US7: 'no duplicate subjects, so git log doesn't contain the same line twice.' FR30-33: duplicate-
        rejection guarantee. BUG-002 violated it on the fast-path; the fix restored it; this test locks it."
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  decompose.go            # READ-ONLY — runLoopFastPath (:680) + the LANDED seenSubjects fix (:767/819/820/844/859); DecomposeRescueError (:83); CommitResult (:53)
  message.go              # READ-ONLY — generateMessageCore (the regen path the fix calls); messageRecentSubjects
  decompose_test.go       # EDIT (ADD one test) — TestRunLoopFastPath_DuplicateSubjectDedupe; reuse dcm* helpers + mirror TestRunLoopFastPath_ConcurrentPublish (:3150)
internal/generate/
  generate.go             # READ-ONLY — ExtractSubject / IsDuplicate (the dedupe primitives the fix + test use)
go.mod                    # READ-ONLY — unchanged (test-only; reuses existing imports)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/decompose/
  decompose_test.go       # MODIFIED — +func TestRunLoopFastPath_DuplicateSubjectDedupe (one test, ~50 lines)
# NOTHING ELSE. No new file, no production change, no go.mod edit, no docs.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (cfg.Edit=false ISOLATES BUG-002 from BUG-001): the BUG-001 fix moved EditMessage into the
// serial publish loop. If cfg.Edit were true here, the test would also exercise the editor path and
// conflate the two bugs. cfg.Edit=false (the default) means no editor runs — pure dedupe behavior.

// CRITICAL (the outcome is RESCUE, deterministic — not success): with a single-response manifest (same msg
// for both rules), regeneration ALSO returns "chore: update thing", so concept 1 CANNOT produce a distinct
// subject → it is RESCUED. Assert *DecomposeRescueError + len(.Commits)==1 + .Index==1. Do NOT assert
// err==nil / 2 commits — that would require a STATEFUL stub (Design B) the contract makes optional.

// CRITICAL (concept order is fixed → deterministic): the serial publish loop processes concepts in ORDER
// (i=0 then i=1). concept 0 ALWAYS publishes first (its subject isn't yet in seenSubjects); concept 1
// ALWAYS collides. So dre.Index==1 and len(dre.Commits)==1 are deterministic — no goroutine-scheduling
// flakiness in the assertion. (The concurrent GENERATION scheduling is nondeterministic, but the serial
// PUBLISH order is fixed, and dedupe happens in the publish loop.)

// GOTCHA (MaxDuplicateRetries=0 is the faster variant): with the default (3), the regen loop tries 4 times
// before returning RescueError (4 stub invocations). cfg.MaxDuplicateRetries=0 rescues on the first
// collision (1 invocation) — faster + equally deterministic. The contract explicitly offers this. Both
// reach the same DecomposeRescueError; either is acceptable.

// GOTCHA (git log subject count, not result.Commits count, is the US7 proof): the headline guarantee is
// "git log doesn't contain the same line twice." Assert strings.Count(dcmLogOneline(t, repo), "chore:
// update thing") == 1 — that's the user-visible proof. (len(dre.Commits)==1 is the mechanism; the git-log
// count is the guarantee.) The base commit "initial" is the only other log line.

// GOTCHA (dcmMessageMatchManifest is concurrency-safe + input-derived): each stub process inspects its OWN
// stdin (the concept's tree-to-tree diff) and emits the first matching rule's msg. So concept 0 (diff
// names a.txt) → "chore: update thing"; concept 1 (diff names b.txt) → "chore: update thing". The matchfile
// is "a.txt|chore: update thing\nb.txt|chore: update thing\n". Reuse the helper — do NOT hand-roll a stub.

// GOTCHA (BORN repo, not unborn): seed a base commit (dcmCommitRaw "initial") so baseTree = HEAD^{tree}
// and isUnborn=false. runLoopFastPath(...,false). An unborn repo would change the parent/chain semantics
// and is not what BUG-002 reproduced.

// GOTCHA (run under -race): the fast-path launches N goroutines. The fix mutates seenSubjects ONLY in the
// serial loop (one goroutine) — never in the concurrent generators — so -race should be clean. If -race
// flags anything, it's a real concurrency bug in the fix; surface it (don't suppress).
```

## Implementation Blueprint

### Data models and structure

None NEW. The test reuses the existing `prompt.PlannerCommit`, `DecomposeRescueError`, `CommitResult`,
`messageMatchRule`, `Deps`, `config.Config`, and the `dcm*` helpers. No new types.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD TestRunLoopFastPath_DuplicateSubjectDedupe to internal/decompose/decompose_test.go
  - PLACE: near the other TestRunLoopFastPath_* tests (e.g. right after TestRunLoopFastPath_ConcurrentPublish
    around line 3200, or grouped with the fast-path tests). package decompose (internal test).
  - IMPORTS: the file already imports context, errors, strings, bytes, testing, config, git, prompt,
    stubtest, ui, provider — verify errors + strings are present (they're used by sibling tests); add only
    if missing.
  - BODY (mirror TestRunLoopFastPath_ConcurrentPublish :3150 with the 4 BUG-002 deltas):
      func TestRunLoopFastPath_DuplicateSubjectDedupe(t *testing.T) {
          bin := stubtest.Build(t)
          repo := t.TempDir()
          dcmInitRepo(t, repo)
          // BORN repo: seed a base commit with the two files, then modify both disjointly.
          dcmWriteFile(t, repo, "a.txt", "aaa\n")
          dcmWriteFile(t, repo, "b.txt", "bbb\n")
          dcmRunGit(t, repo, "add", "a.txt", "b.txt")
          dcmCommitRaw(t, repo, "initial")
          // disjoint working-tree change set (FR-M13 disjoint partition)
          dcmWriteFile(t, repo, "a.txt", "AAA\n")
          dcmWriteFile(t, repo, "b.txt", "BBB\n")

          g := git.New(repo)
          ctx := context.Background()
          baseTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}")
          tStart, err := g.FreezeWorkingTree(ctx, baseTree)
          if err != nil { t.Fatalf("FreezeWorkingTree: %v", err) }
          preRunHEAD := dcmHeadSHA(t, repo)

          // BUG-002 delta #1+#2: SAME subject for both concepts + cfg.Edit=false (isolate from BUG-001).
          messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
              {substr: "a.txt", msg: "chore: update thing"},
              {substr: "b.txt", msg: "chore: update thing"},
          })
          cfg := config.Defaults()
          cfg.Edit = false
          cfg.MaxDuplicateRetries = 0   // faster: regen rescues on the first collision (default 3 also works)
          deps := Deps{Git: g, Config: cfg, Roles: RoleManifests{Message: messageM}, Verbose: nil}

          // BUG-002 delta #3: 2 disjoint concepts.
          concepts := []prompt.PlannerCommit{
              {Title: "c1", Files: []string{"a.txt"}},
              {Title: "c2", Files: []string{"b.txt"}},
          }

          commits, _, err := runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)

          // BUG-002 delta #4: DETERMINISTIC rescue. concept 0 publishes "chore: update thing"; concept 1
          // collides → regen (single-response stub returns same) → still collides → rescued.
          var dre *DecomposeRescueError
          if !errors.As(err, &dre) {
              t.Fatalf("runLoopFastPath error = %T (%v), want *DecomposeRescueError (concept 1 rescued after same-subject collision)", err, err)
          }
          if len(commits) != 1 || len(dre.Commits) != 1 {
              t.Errorf("published commits = %d (dre.Commits=%d), want 1 (concept 0 only; concept 1 rescued)", len(commits), len(dre.Commits))
          }
          if dre.Index != 1 {
              t.Errorf("dre.Index = %d, want 1 (concept 1 is the rescued concept)", dre.Index)
          }
          if len(dre.Commits) == 1 && dre.Commits[0].Subject != "chore: update thing" {
              t.Errorf("published subject = %q, want %q", dre.Commits[0].Subject, "chore: update thing")
          }
          // US7 / FR30-33 guarantee: git log contains the subject exactly ONCE (no duplicate subjects).
          log := dcmLogOneline(t, repo)
          if n := strings.Count(log, "chore: update thing"); n != 1 {
              t.Errorf("git log contains %q %d times, want 1 (US7 no-duplicate-subjects):\n%s", "chore: update thing", n, log)
          }
          // Belt-and-suspenders: no two PUBLISHED subjects are equal (the core invariant).
          var subjects []string
          for _, c := range dre.Commits { subjects = append(subjects, c.Subject) }
          if dups := dupSubjects(subjects); len(dups) > 0 {
              t.Errorf("duplicate published subjects: %v", dups)
          }
      }
  - helper (if not already present — check first; a sibling test may have one):
      // dupSubjects returns any subject strings appearing more than once in ss.
      func dupSubjects(ss []string) []string {
          seen := map[string]int{}; var dups []string
          for _, s := range ss { seen[s]++ }
          for s, n := range seen { if n > 1 { dups = append(dups, s) } }
          return dups
      }
    (If a sibling already has an equivalent, reuse it; otherwise add this private helper near the test.)
  - NAMING: TestRunLoopFastPath_DuplicateSubjectDedupe (per the contract). The dupSubjects helper is
    private (lowercase) — package-scoped.
  - FOLLOW pattern: TestRunLoopFastPath_ConcurrentPublish (decompose_test.go:3150) — identical repo setup +
    freeze + runLoopFastPath call; diverge only on the 4 deltas.
  - GOTCHA: do NOT assert err==nil or len(commits)==2 — the rescue outcome means err is non-nil and exactly
    1 commit published. Asserting success would require a stateful stub (Design B, optional).

Task 2: VERIFY — the new test + full decompose regression + race + lint + grep guards
  - go test ./internal/decompose/ -run 'TestRunLoopFastPath_DuplicateSubjectDedupe' -race -v   # GREEN
  - go test ./internal/decompose/ -race          # full regression (BUG-001 test + all fast-path tests GREEN)
  - go vet ./internal/decompose/...
  - gofmt -l internal/decompose/decompose_test.go   # empty
  - make test ; make lint
  - git status --porcelain   # ONLY internal/decompose/decompose_test.go
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (mirror the proven fast-path skeleton, diverge on 4 axes for BUG-002):
func TestRunLoopFastPath_DuplicateSubjectDedupe(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmRunGit(t, repo, "add", "a.txt", "b.txt")
	dcmCommitRaw(t, repo, "initial") // BORN repo → baseTree = HEAD^{tree}, isUnborn=false
	dcmWriteFile(t, repo, "a.txt", "AAA\n") // disjoint working-tree change set
	dcmWriteFile(t, repo, "b.txt", "BBB\n")

	g := git.New(repo)
	ctx := context.Background()
	baseTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}")
	tStart, err := g.FreezeWorkingTree(ctx, baseTree)
	if err != nil {
		t.Fatalf("FreezeWorkingTree: %v", err)
	}
	preRunHEAD := dcmHeadSHA(t, repo)

	messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
		{substr: "a.txt", msg: "chore: update thing"}, // SAME subject for both ← BUG-002 trigger
		{substr: "b.txt", msg: "chore: update thing"},
	})
	cfg := config.Defaults()
	cfg.Edit = false              // isolate BUG-002 from BUG-001
	cfg.MaxDuplicateRetries = 0   // immediate-rescue variant (default 3 also works)
	deps := Deps{Git: g, Config: cfg, Roles: RoleManifests{Message: messageM}, Verbose: nil}
	concepts := []prompt.PlannerCommit{
		{Title: "c1", Files: []string{"a.txt"}},
		{Title: "c2", Files: []string{"b.txt"}},
	}

	commits, _, err := runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)

	// Deterministic rescue: concept 0 publishes; concept 1 collides → regen (same msg) → rescued.
	var dre *DecomposeRescueError
	if !errors.As(err, &dre) {
		t.Fatalf("err = %T (%v), want *DecomposeRescueError", err, err)
	}
	if len(dre.Commits) != 1 || dre.Index != 1 {
		t.Errorf("dre.Commits=%d dre.Index=%d, want 1 / 1", len(dre.Commits), dre.Index)
	}
	if len(dre.Commits) == 1 && dre.Commits[0].Subject != "chore: update thing" {
		t.Errorf("published subject = %q, want %q", dre.Commits[0].Subject, "chore: update thing")
	}
	// US7: git log has the subject exactly once.
	if n := strings.Count(dcmLogOneline(t, repo), "chore: update thing"); n != 1 {
		t.Errorf("git log has %q %d times, want 1", "chore: update thing", n)
	}
	_ = commits // == dre.Commits (the partial published set); asserted via dre for clarity
}
```

### Integration Points

```yaml
TEST (internal/decompose/decompose_test.go — ADD one function):
  - func TestRunLoopFastPath_DuplicateSubjectDedupe(t *testing.T) — mirrors ConcurrentPublish; 4 BUG-002 deltas.
  - (optional private helper) func dupSubjects(ss []string) []string — if no sibling already has one.

NO production change. NO new file. NO new import (the file already imports context/errors/strings/...).
NO database / migration / routes / config / docs.

SCOPE FENCES:
  - Touches ONLY internal/decompose/decompose_test.go (ADD one test function + optional helper).
  - Does NOT edit decompose.go, message.go, generate.go, go.mod, or any PRD/task file.
  - Does NOT touch the BUG-001 test (TestRunLoopFastPath_EditGate_NoCrossContamination) — different name/
    files/cfg.Edit/manifest/assertions (findings §5).
  - Adds NO production code, NO flag, NO type, NO dependency.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (the test compiles + the helper signatures resolve).
go build ./internal/decompose/...
# Expected: clean. (Test-only file — go build skips _test.go, so also run go test -c or go vet.)

# Vet.
go vet ./internal/decompose/...
# Expected: clean.

# Format.
gofmt -l internal/decompose/decompose_test.go
# Expected: empty. If listed: gofmt -w internal/decompose/decompose_test.go

# Lint.
make lint   # golangci-lint
# Expected: zero errors. (The new test + optional dupSubjects helper are both used.)

# Scope guard: ONLY the one test file changed.
git status --porcelain
# Expected: internal/decompose/decompose_test.go ONLY. ZERO changes to decompose.go/message.go/generate.go/go.mod.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new regression test, under the race detector.
go test ./internal/decompose/ -run 'TestRunLoopFastPath_DuplicateSubjectDedupe' -race -v
# Expected: PASS.
#   - errors.As(err, &dre) succeeds (a *DecomposeRescueError).
#   - len(dre.Commits) == 1; dre.Index == 1; dre.Commits[0].Subject == "chore: update thing".
#   - dcmLogOneline has "chore: update thing" exactly once.
#   - -race is clean (the seenSubjects fix has no data race).

# Full decompose-package regression (the new test + the BUG-001 test + all fast-path/chain/arbiter tests).
go test ./internal/decompose/ -race
# Expected: green. (The BUG-001 test TestRunLoopFastPath_EditGate_NoCrossContamination runs alongside —
#           no shared mutable state, no conflict.)

# Full race suite.
make test
# Expected: green.
```

### Level 3: Integration Testing (System Validation)

```bash
# This is a unit-level regression test against runLoopFastPath directly (not the full Decompose entry point
# nor the CLI). The full e2e (stagecoach upgrade... no — Decompose via the CLI with a real provider) is out
# of scope for this bugfix task; the direct runLoopFastPath call is the precise, deterministic proof. The
# git-log assertion (dcmLogOneline) IS the integration with the real temp git repo.
# Manual smoke (optional — the unit test is the real proof):
go test ./internal/decompose/ -run 'TestRunLoopFastPath_DuplicateSubjectDedupe' -race -v -count=3
# Expected: PASS x3 (deterministic across repeats — confirms no goroutine-scheduling flakiness).
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: the test exists with the contract name.
grep -n 'func TestRunLoopFastPath_DuplicateSubjectDedupe' internal/decompose/decompose_test.go
# Expect: 1 hit.

# Guard 2: it uses the SAME-subject manifest (the BUG-002 trigger) + cfg.Edit=false (isolation).
grep -A4 'TestRunLoopFastPath_DuplicateSubjectDedupe' internal/decompose/decompose_test.go | grep 'chore: update thing'
# Expect: 2 hits (both rules). And:
grep -n 'cfg.Edit = false' internal/decompose/decompose_test.go   # at least the BUG-002 test's hit.

# Guard 3: it asserts the *DecomposeRescueError rescue outcome (not success).
grep -n 'errors.As(err, &dre)\|var dre \*DecomposeRescueError' internal/decompose/decompose_test.go
# Expect: hits in the BUG-002 test.

# Guard 4: it asserts git log has the subject exactly once (the US7 proof).
grep -n 'strings.Count(dcmLogOneline' internal/decompose/decompose_test.go
# Expect: 1 hit in the BUG-002 test.

# Guard 5: NO production file changed (test-only).
git diff --name-only
# Expect: internal/decompose/decompose_test.go ONLY.
git diff --name-only | grep -E 'decompose\.go|message\.go|generate\.go|go\.mod' && echo "FAIL: production file edited" || echo "OK: test-only"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./internal/decompose/...` + `go vet ./internal/decompose/...` clean
- [ ] `gofmt -l internal/decompose/decompose_test.go` empty
- [ ] `make lint` zero errors
- [ ] `go test ./internal/decompose/ -run 'TestRunLoopFastPath_DuplicateSubjectDedupe' -race -v` PASS
- [ ] `go test ./internal/decompose/ -race` green (full regression)
- [ ] `make test` green

### Feature Validation
- [ ] `TestRunLoopFastPath_DuplicateSubjectDedupe` exists (grep guard 1)
- [ ] It uses the SAME-subject manifest for both concepts + `cfg.Edit = false` (grep guard 2)
- [ ] It asserts `*DecomposeRescueError` + `len(.Commits)==1` + `.Index==1` (grep guard 3)
- [ ] It asserts git log has "chore: update thing" exactly once (grep guard 4 — the US7 proof)
- [ ] The test is deterministic across `-count=3` (no goroutine-scheduling flakiness)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `internal/decompose/decompose_test.go`
- [ ] NO edit to decompose.go, message.go, generate.go, go.mod, or any PRD/task file (grep guard 5)
- [ ] NO conflict with the BUG-001 test (different name/files/cfg.Edit/manifest/assertions)
- [ ] NO production change, NO new file, NO new dependency

### Code Quality & Docs
- [ ] Mirrors the proven `TestRunLoopFastPath_ConcurrentPublish` skeleton (not a new pattern)
- [ ] Reuses the `dcm*` helpers + `dcmMessageMatchManifest` (no hand-rolled stub)
- [ ] Carries a comment explaining the deterministic rescue outcome (concept 0 publishes → concept 1 collides → rescued)
- [ ] Contract honored: "DOCS: none — test-only file" (no docs edit)

---

## Anti-Patterns to Avoid

- ❌ Don't assert SUCCESS (err==nil / 2 commits) with a single-response manifest. The stub returns the same
  subject on regeneration, so concept 1 CANNOT produce a distinct subject → it is RESCUED. Assert
  `*DecomposeRescueError` + 1 commit + `.Index==1`. Asserting success would silently pass on the UNFIXED
  code (where both publish) AND fail on the FIXED code — the opposite of a regression test. (Use a stateful
  stub only if you deliberately choose Design B; the contract makes Design A — rescue — the primary.)
- ❌ Don't set `cfg.Edit = true`. That exercises the BUG-001 editor path and conflates the two bugs. BUG-002
  is purely about dedupe; `cfg.Edit = false` (the default) isolates it. (The BUG-001 test owns the
  cfg.Edit=true scenario.)
- ❌ Don't reuse the BUG-001 test's a.go/b.go files or DISTINCT-message manifest. BUG-002 needs a.txt/b.txt
  (per the contract) + an IDENTICAL-message manifest. Reusing BUG-001's distinct messages would NOT trigger
  the collision and the test would be vacuous.
- ❌ Don't skip the git-log assertion. `len(dre.Commits)==1` is the mechanism; `strings.Count(dcmLogOneline,
  "chore: update thing")==1` is the US7 USER-VISIBLE guarantee ("git log doesn't contain the same line
  twice"). Assert both — the git-log count is the headline proof.
- ❌ Don't edit any production file. The fix (P1.M2.T1.S1) is LANDED. This task is test-only: add ONE
  function to decompose_test.go. Editing decompose.go/message.go is out of scope and risks re-breaking the
  fix or conflicting with P1.M2.T1.S1 / P1.M3.T3.S1.
- ❌ Don't hand-roll a stub binary. `dcmMessageMatchManifest` is the concurrency-safe, input-derived stub
  (each process inspects its own stdin diff). Use it — it's what the sibling fast-path tests use and it's
  what makes the two concepts emit deterministic per-rule messages under concurrency.
- ❌ Don't run the test without `-race`. The fast-path launches N goroutines; the fix's `seenSubjects` is
  mutated only in the serial loop (no race expected), but `-race` is the cheap guard that catches any
  concurrency regression in the fix. The validation command includes it.
- ❌ Don't add docs. The contract says "DOCS: none — test-only file." The changeset docs sync is P1.M3.T4.S1
  (separate task).
- ❌ Don't worry about goroutine-scheduling nondeterminism in the ASSERTION. The serial publish loop
  processes concepts in ORDER (0 then 1); concept 0 always publishes first, concept 1 always collides. The
  rescue outcome (dre.Index==1, 1 commit) is deterministic. The `-count=3` smoke confirms it.
- ❌ Don't conflate this with the BUG-001 test or the concurrency-comment task. Three distinct deliverables:
  BUG-001 test (P1.M3.T1.S1, cfg.Edit=true, distinct msgs, success), BUG-002 test (THIS, cfg.Edit=false,
  identical msgs, rescue), concurrency-comment (P1.M3.T3.S1). Touch only the BUG-002 test.