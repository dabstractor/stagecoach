name: "P1.M2.T1.S1 — Add seenSubjects tracking + cross-concept dedupe (re-generation/rescue) to runLoopFastPath (BUG-002)"
description: >
  Fix BUG-002 (Major): the file-disjoint fast-path (runLoopFastPath, internal/decompose/decompose.go:680)
  launches all N message generations CONCURRENTLY before any publish, so each generateMessage sees only
  pre-run history — two disjoint concepts with the same emitted subject both publish (violates US7/FR30-33).
  Fix = a `seenSubjects` accumulator in the SERIAL publish loop: (a) init
  `seenSubjects, _ := messageRecentSubjects(ctx, deps.Git, isUnborn)` before the loop; (b) inside the
  loop, AFTER the `if res.err != nil` block and BEFORE the EditMessage block (parallel P1.M1.T2.S2) /
  publishCommit, check `generate.ExtractSubject(res.msg)` against seenSubjects; on collision re-generate
  via `generateMessageCore(ctx, deps, sc.prevTree, sc.tree, seenSubjects)` (seedRejections tells the LLM
  which subjects to avoid); mirror the res.err handler on regen failure (RescueError → fix ParentSHA=
  prevSHA + FormatRescueMulti + drainMsgs + DecomposeRescueError; hard → drainMsgs + return); if STILL a
  duplicate, rescue (prior commits stand, FR-M12); then `seenSubjects = append(seenSubjects, subject)`.
  Uses FROZEN generate.ExtractSubject/IsDuplicate/generateMessageCore/messageRecentSubjects signatures
  (no signature changes). Placed BEFORE EditMessage so it judges the generated (pre-edit) subject
  (FR-E3). Plus the BUG-002 regression test (2 disjoint concepts, same-subject stub → no duplicate
  subjects; rescue or both-published-distinct). NO EditMessage logic (T2.S2), NO message.go/dedupe.go/
  generate.go changes, NO runLoop/launch-closure/publishCommit changes, NO docs.

---

## Goal

**Feature Goal**: Close the cross-concept duplicate-subject gap on the file-disjoint fast-path so a
single `stagecoach` decompose run can NEVER publish two commits with the same subject (US7, FR30-FR33).
The fast-path's concurrent-then-serial structure made each per-concept generation blind to siblings;
this task adds an incremental dedupe accumulator in the serial publish loop that sees every
already-decided sibling and re-generates (or rescues) any colliding concept.

**Deliverable**:
1. **internal/decompose/decompose.go** — in `runLoopFastPath`: (a) a `seenSubjects` init before the
   serial publish loop; (b) a cross-concept dedupe block inside the loop (ExtractSubject → IsDuplicate →
   re-generate via generateMessageCore → rescue-on-still-duplicate), inserted after the `if res.err !=
   nil` block and before the EditMessage block (T2.S2) / publishCommit; (c) `seenSubjects =
   append(seenSubjects, subject)` after the check.
2. **internal/decompose/decompose_test.go** (or a new `*_test.go`) — the BUG-002 regression test:
   2 disjoint concepts whose message stub emits the SAME subject; assert no two published commits share
   a subject (rescue-with-1-commit OR both-published-with-distinct-subjects).

**Success Definition**:
- Two disjoint concepts whose message agent emits the same subject NEVER both publish that subject: the
  second either re-generates a distinct subject (both publish) or is rescued (concept 0 stands, concept
  1 abandoned via DecomposeRescueError with `len(Commits)==1`).
- `seenSubjects` accumulates pre-run history (messageRecentSubjects) + each accepted concept's subject
  as the serial loop progresses.
- A re-generation failure is handled IDENTICALLY to the existing `res.err` handler (RescueError → fixed
  ParentSHA=prevSHA + FormatRescueMulti + drainMsgs + DecomposeRescueError; non-rescue → drainMsgs +
  return the error).
- The dedupe block precedes EditMessage (judges the pre-edit subject; FR-E3 honored).
- The FROZEN signatures (ExtractSubject/IsDuplicate/generateMessageCore/messageRecentSubjects) are
  unchanged; message.go/dedupe.go/generate.go untouched.
- `go build ./...`, `go test ./internal/decompose/...`, `go test -race ./internal/decompose/...`,
  `make test`, `make lint` pass; `gofmt -l` clean.

## User Persona (if applicable)

**Target User**: A developer running multi-commit decomposition (`stagecoach` with nothing staged + a
dirty tree) whose planner partition is pairwise file-disjoint (the common cleanly-separated case).

**Use Case**: Two unrelated changes (e.g. `fix typo in README` + `add config`) that the model
coincidentally describes with the same subject. Pre-fix, both published identical subjects → git log
has a repeated line (exactly what US7 exists to prevent). Post-fix, the second is re-generated or
rescued — never a duplicate.

**User Journey**: `stagecoach` → fast-path launches N concurrent generations → serial publish loop:
concept 0 accepted+published; concept 1's subject collides with concept 0's → re-generate (LLM told to
avoid the taken subjects) → distinct subject → publish (or rescue if the LLM can't produce one).

**Pain Points Addressed**: BUG-002 (Major) — US7/FR30-33 silently violated on the fast-path; git log
with duplicate subjects from a single run, undermining the duplicate-rejection guarantee users rely on.

## Why

- **BUG-002 / US7 / FR30-33**: "no duplicate subjects" is a headline guarantee. The fast-path (G29,
  FR-M13/14) preserved CAS serialization, freeze, and tree-to-tree diff but silently dropped cross-
  concept dedupe (FR-M14 enumerates the preserved invariants and is silent on dedupe). This task
  restores it without giving up the concurrency win (re-generation is the only serial step, and only on
  the rare collision).
- **Consistency with runLoop**: the tooled-stager path (runLoop) already catches cross-concept
  duplicates via its 1-deep generate-after-publish overlap. This makes the fast-path match that
  guarantee.
- **Bounded scope**: one accumulator + one serial-loop block + one regression test. The re-generation
  reuses the already-landed `generateMessageCore` (P1.M1.T1.S1) and its `seedRejections` plumbed for
  exactly this. No new types, no signature changes, no concurrency-model change.

## What

**User-visible behavior**: A fast-path decompose run no longer emits two commits with the same subject.
On a collision, the colliding concept is re-generated (the LLM is told which subjects siblings took); if
the LLM still can't produce a distinct subject, the concept is rescued (prior commits stand).

**Technical change** (one accumulator + one serial-loop block; verbatim code in the Blueprint):

```go
// before the serial publish loop:
seenSubjects, _ := messageRecentSubjects(ctx, deps.Git, isUnborn)

// inside the loop, after the `if res.err != nil` block, before EditMessage/publishCommit:
subject := generate.ExtractSubject(res.msg)
if generate.IsDuplicate(subject, seenSubjects) {
    regenerated, regErr := generateMessageCore(ctx, deps, sc.prevTree, sc.tree, seenSubjects)
    // ... handle regErr same as res.err (RescueError→fix ParentSHA+FormatRescueMulti+drainMsgs+DecomposeRescueError; hard→drainMsgs+return)
    res.msg = regenerated
    subject = generate.ExtractSubject(res.msg)
    if generate.IsDuplicate(subject, seenSubjects) { /* drainMsgs + return DecomposeRescueError */ }
}
seenSubjects = append(seenSubjects, subject)
```

### Success Criteria
- [ ] `seenSubjects` initialized from `messageRecentSubjects(ctx, deps.Git, isUnborn)` before the loop
- [ ] dedupe block in the loop, after `if res.err != nil`, before EditMessage/publishCommit
- [ ] collision → re-generate via `generateMessageCore(ctx, deps, sc.prevTree, sc.tree, seenSubjects)`
- [ ] regen RescueError → fix ParentSHA=prevSHA + FormatRescueMulti + drainMsgs + DecomposeRescueError (mirrors res.err)
- [ ] regen non-rescue error → drainMsgs + return regErr (mirrors res.err)
- [ ] still-duplicate after regen → drainMsgs + DecomposeRescueError (concept[i] abandoned, prior stand)
- [ ] `seenSubjects = append(seenSubjects, subject)` after the check (happy path + regen-success path)
- [ ] FROZEN signatures unchanged; message.go/dedupe.go/generate.go untouched
- [ ] BUG-002 regression test passes (2 same-subject concepts → no duplicate subjects)
- [ ] `make test`/`make lint`/`gofmt -l`/`go test -race` green

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim dedupe block, the exact insertion anchors (by content), the verbatim res.err
handler to mirror, the FROZEN signatures table, the seedRejections semantics, the FR-E3
"before-EditMessage" ordering rationale, the T2.S2 coordination (content anchor, no overlap), the
test setup/assertions (with the rescue-vs-both-published dichotomy and the single-response-stub
limitation), and the explicit scope fences.

### Documentation & References

```yaml
# MUST READ — the authoritative fix design (Part 3 = THIS task, verbatim)
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/architecture/fix_design.md
  why: "Part 3 ('BUG-002 Fix — Incremental cross-concept dedupe') gives the verbatim block + the
        'Combined serial loop order' (step 3 dedupe → 4 append → 5 EditMessage → 6 publish). Part 1
        documents generateMessageCore's seedRejections semantics (the LLM 'avoid these subjects' block)."
  critical: "The dedupe MUST come before EditMessage (step 3 < step 5) so it judges the pre-edit subject
             (FR-E3). seedRejections is BOTH the dedupe-set augmentation AND the prompt's rejection block."

# MUST READ — the research notes (verbatim block + frozen signatures + coordination + test)
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/P1M2T1S1/research/findings.md
  why: "§1 the fix site + insertion anchors; §2 the FROZEN signatures table; §3 the verbatim res.err
        handler to mirror; §4 the verbatim dedupe block; §5 the T2.S2 coordination; §7 the test recipe."
  critical: "§3: title is computed INSIDE the res.err block → NOT in scope at the dedupe block → compute
             it locally in the regen-error path (2-line repeat; do NOT refactor the existing handler).
             §1: messageRecentSubjects error is intentionally ignored (_) — a history-fetch failure must
             not abort (safe degradation)."

# MUST READ — the file being edited (the serial publish loop)
- file: internal/decompose/decompose.go
  why: "runLoopFastPath @680. The serial publish loop is ~774-825: `prevSHA := preRunHEAD` (~773); the
        `for i, ch := range inflight` loop with signal.SetSnapshot/res:=<-ch/ClearSnapshot, the
        `if res.err != nil {…}` handler (the template for the regen-error path), `// Publish in CAS
        order` + publishCommit, buildCommitResult, chainData append, `prevSHA = newSHA`."
  pattern: "The res.err handler is the EXACT template for the regen-error path: errors.As(*RescueError)
            → fixed:=*re; fixed.ParentSHA=prevSHA → FormatRescueMulti to deps.Out → drainMsgs(inflight[i+1:])
            → return &DecomposeRescueError{Rescue:&fixed, ConceptTitle, Index:sc.idx, Count:len(concepts),
            Commits:commits}. Non-rescue → drainMsgs + return the err."
  gotcha: "Anchor by CONTENT not line number — parallel T2.S2 (EditMessage block) is in-flight and shifts
           lines. Insert 'after the `if res.err != nil` block's closing }, before the `if deps.Config.Edit`
           block (or before `// Publish in CAS order` if T2.S2 hasn't landed)'. Either landing order is safe."

# MUST READ — the FROZEN signatures (do NOT change any of these)
- file: internal/generate/dedupe.go
  why: "ExtractSubject(message string) string @19; IsDuplicate(subject string, recent []string) bool @46.
        These are FROZEN — call them, do not modify."
- file: internal/decompose/message.go
  why: "generateMessageCore(ctx, deps, treeA, treeB, seedRejections []string) (string, error) @80
        (LANDED by P1.M1.T1.S1) — call it for re-generation, passing seenSubjects as seedRejections.
        messageRecentSubjects(ctx, g git.Git, isUnborn bool) ([]string, error) @366 — call it for the
        seenSubjects init."
- file: internal/generate/generate.go
  why: "RescueError struct @112 {Kind, TreeSHA, ParentSHA, Candidate, Cause}; ErrRescue @95;
        FormatRescueMulti @rescue.go:88. FROZEN — use them in the constructed rescue, do not modify."
  gotcha: "DecomposeRescueError fields (from the existing handler): Rescue *generate.RescueError,
           ConceptTitle string, Index int, Count int, Commits []CommitResult. Construct the
           still-duplicate rescue with Kind=ErrRescue, TreeSHA=sc.tree, ParentSHA=prevSHA, Candidate=res.msg."

# MUST READ — the test recipe + infrastructure
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/architecture/test_strategy.md
  why: "The BUG-002 section gives the exact setup (2 disjoint concepts, dcmMessageMatchManifest with the
        SAME msg for both, cfg.Edit=false), the rescue-vs-both-published assertion dichotomy, and the
        single-response-stub limitation (→ rescue is the expected case; optionally MaxDuplicateRetries=0
        to force immediate rescue). Lists the helpers: dcmInitRepo/dcmWriteFile/dcmRunGit/dcmCommitRaw/
        dcmGitOut/dcmHeadSHA/dcmMessageMatchManifest/messageMatchRule/dcmDeps/stubtest.Build."
  critical: "Mirror TestRunLoopFastPath_ConcurrentPublish (@decompose_test.go:3142) for the repo/baseTree/
             tStart/concepts setup. runLoopFastPath signature: (ctx, deps, concepts []prompt.PlannerCommit,
             baseTree, tStart, preRunHEAD string, isUnborn bool)."

# CONTEXT — the parallel sibling (MUST coordinate; MUST NOT conflict)
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/P1M1T2S2/PRP.md
  why: "Parallel P1.M1.T2.S2 adds the EditMessage block at serial-loop step 5 (between my dedupe block
        and publishCommit). My dedupe block (step 3-4) MUST precede it (FR-E3: judge pre-edit subject).
        T2.S2 touches ONLY EditMessage; I touch ONLY seenSubjects+dedupe — no overlap. Read it to confirm
        the non-overlap + to place my block before its `if deps.Config.Edit` block."
  critical: "If T2.S2 has landed, the order is: res.err block → [MY dedupe block] → [T2.S2 EditMessage
             block] → publishCommit. If T2.S2 has NOT landed: res.err block → [MY dedupe block] →
             publishCommit. Anchor by content either way."
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  decompose.go       # EDIT — runLoopFastPath: +seenSubjects init + dedupe block (THIS TASK)
  decompose_test.go  # EDIT — +BUG-002 regression test (THIS TASK)
  message.go         # READ-ONLY — generateMessageCore (@80, LANDED) + messageRecentSubjects (@366); FROZEN
internal/generate/
  dedupe.go          # READ-ONLY — ExtractSubject (@19) + IsDuplicate (@46); FROZEN
  generate.go        # READ-ONLY — RescueError (@112) + ErrRescue (@95); FROZEN
  rescue.go          # READ-ONLY — FormatRescueMulti (@88); FROZEN
cmd/stubagent/       # READ-ONLY — the message stub (driven by dcmMessageMatchManifest via STAGECOACH_STUB_*)
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (no new files):
internal/decompose/decompose.go       # +seenSubjects init (before loop) + dedupe block (in loop, step 3-4)
internal/decompose/decompose_test.go  # +TestRunLoopFastPath_CrossConceptDedupe (BUG-002 regression)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (insertion order — dedupe BEFORE EditMessage): the dedupe block MUST precede the EditMessage
//   block (parallel T2.S2) so it judges the GENERATED subject, not the edited one. FR-E3 says an edited
//   message bypasses the re-check; if dedupe ran AFTER EditMessage, a user's edit to a colliding subject
//   would be silently rejected/regenerated, violating FR-E3. fix_design.md "Combined serial loop order":
//   step 3 (dedupe) → 4 (append) → 5 (EditMessage) → 6 (publish).

// CRITICAL (anchor by content, not line — T2.S2 is in-flight): parallel P1.M1.T2.S2 adds the EditMessage
//   block, shifting line numbers. Locate the insertion point by content: "after the closing } of the
//   `if res.err != nil {…}` block, before the `if deps.Config.Edit` block (T2.S2) OR before `// Publish
//   in CAS order`/publishCommit". Do NOT hardcode line numbers.

// CRITICAL (title is NOT in scope at the dedupe block): the existing res.err handler computes
//   `title := ""; if sc.idx < len(concepts) { title = concepts[sc.idx].Title }` INSIDE its block. That
//   title is out of scope at the dedupe block. Compute it LOCALLY in the regen-error path (and the
//   still-duplicate path) — a 2-line repeat. Do NOT refactor the existing handler to hoist title (that
//   widens the diff and risks the res.err path).

// CRITICAL (mirror the res.err handler EXACTLY for regen errors): on a re-generation error, the handling
//   is byte-for-byte the res.err handler's: errors.As(*generate.RescueError) → fixed:=*re; fixed.ParentSHA
//   = prevSHA (authoritative at publish time; the concurrent regen may carry a stale ParentSHA) →
//   FormatRescueMulti to deps.Out → drainMsgs(inflight[i+1:]) → &DecomposeRescueError{...}. Non-rescue →
//   drainMsgs + return regErr. Do not invent new error handling.

// GOTCHA (messageRecentSubjects error is ignored): `seenSubjects, _ := messageRecentSubjects(...)`. A
//   history-fetch failure must NOT abort the run — dedupe degrades to "no history" (only sibling-collision
//   detection weakens; never a false positive). This matches each goroutine's own best-effort fetch.

// GOTCHA (regeneration uses sc.prevTree/sc.tree — the SAME trees the goroutine generated against):
//   generateMessageCore(ctx, deps, sc.prevTree, sc.tree, seenSubjects). This is a faithful re-generation
//   of the same concept's diff (treeA=sc.prevTree, treeB=sc.tree), with seedRejections=seenSubjects so
//   the LLM's dedupe set AND prompt-rejection-block both include the taken subjects.

// GOTCHA (the still-duplicate rescue is a CONSTRUCTED RescueError, not from generateMessageCore): after
//   a SUCCESSFUL regen (regErr==nil) that STILL produces a duplicate subject, construct the rescue
//   directly: &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: sc.tree, ParentSHA: prevSHA,
//   Candidate: res.msg} wrapped in DecomposeRescueError. (generateMessageCore's own loop would normally
//   catch this via seedRejections; this is belt-and-suspenders for a stub/LLM that ignores the seed.)

// GOTCHA (FROZEN signatures — call only, never modify): ExtractSubject/IsDuplicate (dedupe.go),
//   generateMessageCore/messageRecentSubjects (message.go), RescueError/ErrRescue/FormatRescueMulti
//   (generate.go). This task adds NO new types and changes NO signatures.
```

## Implementation Blueprint

### Data models and structure
None new. Reuses `[]string` (seenSubjects), the FROZEN `generate.ExtractSubject`/`IsDuplicate`, the
landed `generateMessageCore` (seedRejections), and the existing `DecomposeRescueError`/`generate.RescueError`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/decompose/decompose.go — seenSubjects init before the serial publish loop
  - LOCATE runLoopFastPath (@680). Find the `if len(staged) == 0 { return commits, chainData, nil }`
    guard, then `prevSHA := preRunHEAD` (~line 773), then `for i, ch := range inflight {`.
  - INSERT, immediately before `prevSHA := preRunHEAD` (or immediately after — both are before the loop;
    place it right after the staged==0 guard for clarity):
        // BUG-002: cross-concept dedupe accumulator. Pre-seeded with pre-run history; each accepted
        // concept's subject is appended in the serial publish loop below. The fast-path generates all N
        // messages concurrently BEFORE any publish, so the per-concept generateMessageCore can only see
        // this pre-run history — the serial-loop check below closes the sibling-collision gap.
        seenSubjects, _ := messageRecentSubjects(ctx, deps.Git, isUnborn)
  - (Error intentionally ignored — see gotchas.)

Task 2: EDIT internal/decompose/decompose.go — the dedupe block inside the serial loop
  - LOCATE the serial loop body. Find the `if res.err != nil { … }` block's closing `}`. The dedupe
    block goes IMMEDIATELY AFTER it, BEFORE the next step (the `if deps.Config.Edit` block if T2.S2 has
    landed, else `// Publish in CAS order` / `publishCommit`).
  - INSERT the verbatim block from research/findings.md §4 (ExtractSubject → IsDuplicate → regenerate
    via generateMessageCore(ctx, deps, sc.prevTree, sc.tree, seenSubjects) → regen-error handling that
    MIRRORS the res.err handler → res.msg=regenerated → still-duplicate → drainMsgs+DecomposeRescueError),
    followed by `seenSubjects = append(seenSubjects, subject)`.
  - PRESERVE: the res.err handler (unchanged), the EditMessage block (T2.S2 — do not touch), publishCommit,
    buildCommitResult, chainData append, `prevSHA = newSHA`.
  - NAMING: `subject`, `regenerated`, `regErr`, `re`, `fixed`, `title` — match the res.err handler's
    locals for review symmetry.
  - DEPENDENCIES: generateMessageCore (LANDED), ExtractSubject/IsDuplicate (FROZEN), messageRecentSubjects.

Task 3: CREATE/EDIT internal/decompose/decompose_test.go — BUG-002 regression test
  - MIRROR TestRunLoopFastPath_ConcurrentPublish (@3142) for setup: temp repo, base commit (BORN),
    2 disjoint dirty files (a.txt, b.txt), FreezeWorkingTree → tStart, preRunHEAD.
  - MESSAGE STUB (SAME subject for both concepts):
        dcmMessageMatchManifest(t, bin, []messageMatchRule{
            {substr: "a.txt", msg: "chore: update thing"},
            {substr: "b.txt", msg: "chore: update thing"},
        })
  - cfg.Edit = false (isolate BUG-002 from BUG-001). Optionally cfg.MaxDuplicateRetries = 0 (force
    immediate rescue on the single-response stub — simpler/faster).
  - CALL: commits, chainData, err := runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)
  - ASSERT (the no-duplicate-subjects guarantee — covers BOTH outcomes):
        // collect published-commit subjects from whichever outcome occurred
        var subjects []string
        var dre *DecomposeRescueError
        if errors.As(err, &dre) { subjects = commitSubjects(dre.Commits) } else { subjects = commitSubjects(commits) }
        // no two published commits share a subject
        seen := map[string]bool{}
        for _, s := range subjects { if seen[s] { t.Errorf("duplicate published subject %q", s) }; seen[s] = true }
        // AND one of the two valid outcomes:
        if dre != nil {
            // rescue case: concept 0 published, concept 1 rescued
            if len(dre.Commits) != 1 { t.Errorf("rescue: want 1 prior commit, got %d", len(dre.Commits)) }
        } else if err == nil {
            // success case: both published with DISTINCT subjects
            if len(commits) != 2 { t.Errorf("success: want 2 commits, got %d", len(commits)) }
            if commits[0].Subject == commits[1].Subject { t.Errorf("both published same subject %q", commits[0].Subject) }
        } else { t.Fatalf("unexpected error: %v", err) }
    (commitSubjects helper: extract Subject from each CommitResult — mirror however existing tests read it.)
  - NAMING: TestRunLoopFastPath_CrossConceptDedupe (or TestRunLoopFastPath_NoDuplicateSubjects).

Task 4: VERIFY — build, vet, format, focused + race tests, lint, grep guards
  - go build ./... ; go vet ./internal/decompose/...
  - gofmt -l internal/decompose/decompose.go internal/decompose/decompose_test.go   # empty
  - go test ./internal/decompose/ -run 'FastPath' -v   # the new test + existing ConcurrentPublish
  - go test -race ./internal/decompose/...
  - make test ; make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the dedupe block (step 3-4), inserted after the `if res.err != nil` block, before EditMessage/publish
subject := generate.ExtractSubject(res.msg)
if generate.IsDuplicate(subject, seenSubjects) {
	regenerated, regErr := generateMessageCore(ctx, deps, sc.prevTree, sc.tree, seenSubjects)
	if regErr != nil {
		var re *generate.RescueError
		if errors.As(regErr, &re) {                       // ← MIRROR the res.err handler from here
			title := ""
			if sc.idx < len(concepts) { title = concepts[sc.idx].Title }
			fixed := *re
			fixed.ParentSHA = prevSHA
			if deps.Out != nil {
				fmt.Fprintln(deps.Out, generate.FormatRescueMulti(fixed.TreeSHA, fixed.ParentSHA, fixed.Candidate, title, sc.idx, len(concepts)))
			}
			drainMsgs(inflight[i+1:])
			return commits, nil, &DecomposeRescueError{Rescue: &fixed, ConceptTitle: title, Index: sc.idx, Count: len(concepts), Commits: commits}
		}                                                 // ← to here
		drainMsgs(inflight[i+1:])
		return commits, nil, regErr
	}
	res.msg = regenerated
	subject = generate.ExtractSubject(res.msg)
	if generate.IsDuplicate(subject, seenSubjects) {      // belt-and-suspenders: still colliding → rescue
		title := ""
		if sc.idx < len(concepts) { title = concepts[sc.idx].Title }
		drainMsgs(inflight[i+1:])
		return commits, nil, &DecomposeRescueError{
			Rescue:       &generate.RescueError{Kind: generate.ErrRescue, TreeSHA: sc.tree, ParentSHA: prevSHA, Candidate: res.msg},
			ConceptTitle: title, Index: sc.idx, Count: len(concepts), Commits: commits,
		}
	}
}
seenSubjects = append(seenSubjects, subject)

// PATTERN: the seenSubjects init (before the loop, after the staged==0 guard)
seenSubjects, _ := messageRecentSubjects(ctx, deps.Git, isUnborn)
```

### Integration Points

```yaml
NO new types / signatures / config / routes / deps. One accumulator + one serial-loop block + one test.

DECOMPOSE (internal/decompose/decompose.go):
  - runLoopFastPath: +seenSubjects init (messageRecentSubjects seed) + dedupe block (ExtractSubject →
    IsDuplicate → generateMessageCore re-gen → rescue-on-fail/still-dup) + seenSubjects.append.

DOWNSTREAM (unchanged callers): runLoopFastPath is called ONLY from Decompose (@decompose.go:249); its
  signature is unchanged. The dedupe is transparent to callers (same return shapes; on collision either
  one fewer commit + a DecomposeRescueError, or all commits with distinct subjects).

SCOPE FENCES: NO EditMessage logic (parallel T2.S2); NO message.go/generate.go/dedupe.go changes (FROZEN
  signatures); NO runLoop/launch-closure/publishCommit/buildCommitResult changes; NO config/git/provider;
  NO docs (internal dedupe logic).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build + vet.
go build ./...
go vet ./internal/decompose/...

# Format.
gofmt -l internal/decompose/decompose.go internal/decompose/decompose_test.go
# Expected: empty. If listed: gofmt -w the file(s).

# Lint.
make lint      # golangci-lint v1.61 (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors.

# Scope guard: only decompose.go + decompose_test.go changed.
git diff --name-only
# Expected: internal/decompose/decompose.go  internal/decompose/decompose_test.go  (only).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new BUG-002 regression test + the existing fast-path tests.
go test ./internal/decompose/ -run 'FastPath' -v
# Expected: PASS — TestRunLoopFastPath_CrossConceptDedupe (2 same-subject concepts → no duplicate
#           subjects; rescue-with-1-commit OR both-published-distinct) + the existing ConcurrentPublish.

# Race detector (the dedupe runs in the serial loop, but race confirms no regression in the concurrent phase).
go test -race ./internal/decompose/...

# Full decompose package + full repo suite.
go test ./internal/decompose/... -v
make test
# Expected: green. The parallel T2.S2 EditMessage test (if landed) also passes — the two blocks compose.
```

### Level 3: Integration Testing (System Validation)

```bash
# The regression test IS the integration proof (it calls runLoopFastPath end-to-end against a real temp
# git repo + the stub message agent). No separate e2e needed — this task has no CLI/HTTP surface.

# Manual confidence (optional): run the new test with -v and inspect the flow:
go test ./internal/decompose/ -run TestRunLoopFastPath_CrossConceptDedupe -v
# Expected: either "rescue: 1 prior commit" (concept 0 published, concept 1 rescued) or "success: 2
# commits, distinct subjects" — and NEVER a duplicate-subject error.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1: seenSubjects is initialized from messageRecentSubjects (the pre-run history seed).
grep -n 'seenSubjects, _ := messageRecentSubjects' internal/decompose/decompose.go
# Expected: 1 hit (before the serial publish loop).

# Grep guard 2: the dedupe check uses the FROZEN helpers.
grep -n 'generate.ExtractSubject(res.msg)\|generate.IsDuplicate(subject, seenSubjects)' internal/decompose/decompose.go
# Expected: ExtractSubject ≥2 (initial + post-regen); IsDuplicate ≥2 (initial + still-dup check).

# Grep guard 3: re-generation calls generateMessageCore with seenSubjects as seedRejections.
grep -n 'generateMessageCore(ctx, deps, sc.prevTree, sc.tree, seenSubjects)' internal/decompose/decompose.go
# Expected: 1 hit (inside the dedupe block). NOTE: the launch closure calls generateMessageCore(..., nil) — that's T2.S1, unchanged.

# Grep guard 4: seenSubjects is appended after the check (accumulator grows).
grep -n 'seenSubjects = append(seenSubjects, subject)' internal/decompose/decompose.go
# Expected: 1 hit (after the dedupe block, before EditMessage/publishCommit).

# Grep guard 5: the dedupe block precedes EditMessage (FR-E3 — judges pre-edit subject).
#   (Find the line numbers of the dedupe IsDuplicate vs the EditMessage block; dedupe < EditMessage.)
awk '/generate.IsDuplicate\(subject, seenSubjects\)/{dup=NR} /deps.Config.Edit/{edit=NR} END{print "dedupe="dup" edit="edit; exit (dup<edit?0:1)}' internal/decompose/decompose.go && echo "OK: dedupe before EditMessage"
# Expected: "OK: dedupe before EditMessage" (dup < edit). If T2.S2 hasn't landed, the EditMessage line is absent — the guard then confirms dedupe precedes publishCommit instead.

# Grep guard 6: FROZEN files untouched.
git diff --name-only | grep -E 'internal/generate/|message.go'
# Expected: empty (dedupe.go/generate.go/rescue.go/message.go unchanged).

# Grep guard 7: the EditMessage block (T2.S2) is NOT duplicated by this task (no second EditMessage).
grep -c 'generate.EditMessage' internal/decompose/decompose.go
# Expected: 1 (T2.S2's block, if landed) or 0 (if T2.S2 hasn't landed) — NOT 2. This task adds NO EditMessage call.

# Regression: the existing fast-path + runLoop tests stay green.
go test ./internal/decompose/ -run 'RunLoop|FastPath' -v
# Expected: all PASS.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/decompose/...` clean
- [ ] `gofmt -l internal/decompose/decompose.go internal/decompose/decompose_test.go` empty
- [ ] `go test -race ./internal/decompose/...` green
- [ ] `make test` + `make lint` pass

### Feature Validation
- [ ] 2 disjoint same-subject concepts → no two published commits share a subject (regression test)
- [ ] collision → re-generate via generateMessageCore(..., seenSubjects); regen fail → mirrors res.err handler
- [ ] still-duplicate after regen → rescue (concept[i] abandoned, prior stand; DecomposeRescueError)
- [ ] seenSubjects accumulates (pre-run history + each accepted concept's subject)
- [ ] dedupe block precedes EditMessage (FR-E3 — judges pre-edit subject)

### Scope-Boundary Validation
- [ ] `git diff --name-only` == only {internal/decompose/decompose.go, internal/decompose/decompose_test.go}
- [ ] NO EditMessage logic added (T2.S2 owns it; grep guard 7)
- [ ] NO signature change to ExtractSubject/IsDuplicate/generateMessageCore/messageRecentSubjects (FROZEN)
- [ ] NO edit to message.go, generate/dedupe.go, generate/generate.go, generate/rescue.go (grep guard 6)
- [ ] NO edit to runLoop, the launch closure, publishCommit, buildCommitResult, config/git/provider
- [ ] NO docs change (internal dedupe logic)

### Code Quality & Docs
- [ ] Regen-error path mirrors the res.err handler byte-for-byte (review symmetry)
- [ ] Comment cites BUG-002 + the concurrency-then-serial root cause + the FR-E3 before-EditMessage rationale
- [ ] Test mirrors TestRunLoopFastPath_ConcurrentPublish setup; asserts the no-duplicate guarantee across both outcomes

---

## Anti-Patterns to Avoid

- ❌ Don't place the dedupe check AFTER EditMessage. It must judge the GENERATED (pre-edit) subject so a
  user's edit to a colliding subject is honored (FR-E3: edited messages bypass the re-check). Order is
  res.err → dedupe → EditMessage → publish (fix_design.md "Combined serial loop order").
- ❌ Don't anchor to line numbers. Parallel T2.S2 (EditMessage block) is in-flight and shifts lines.
  Locate by content: "after the `if res.err != nil` block's closing `}`, before `if deps.Config.Edit`
  (T2.S2) or `// Publish in CAS order`". Both landing orders are safe.
- ❌ Don't refactor the existing res.err handler to "share" the title computation. `title` is computed
  inside that block and is out of scope at the dedupe block — repeat the 2-line title computation LOCALLY
  in the regen-error + still-duplicate paths. Hoisting it widens the diff and risks the proven res.err path.
- ❌ Don't invent new error handling for re-generation failures. Mirror the res.err handler EXACTLY:
  errors.As(*RescueError) → fixed.ParentSHA=prevSHA → FormatRescueMulti → drainMsgs → DecomposeRescueError;
  non-rescue → drainMsgs + return. Consistency is the point (same exit codes, same partial-commit semantics).
- ❌ Don't abort the run on a messageRecentSubjects failure. `seenSubjects, _ := messageRecentSubjects(...)`
  — the error is intentionally ignored. A history-fetch failure degrades dedupe to "no history" (only
  sibling-collision detection weakens; never a false positive). Aborting would make a transient git error
  fatal to every fast-path run.
- ❌ Don't pass the wrong trees to generateMessageCore. Use `sc.prevTree, sc.tree` (the SAME treeA/treeB
  the goroutine generated against) — a faithful re-generation of this concept's diff, with
  seedRejections=seenSubjects so the LLM avoids the taken subjects.
- ❌ Don't modify any FROZEN signature. ExtractSubject/IsDuplicate (dedupe.go), generateMessageCore/
  messageRecentSubjects (message.go), RescueError/ErrRescue/FormatRescueMulti (generate.go) are called
  only. This task adds NO new types and changes NO signatures.
- ❌ Don't add EditMessage logic. That's parallel T2.S2 (BUG-001). This task is dedupe-only (BUG-002).
  Adding a second EditMessage call would double-edit (grep guard 7 catches it).
- ❌ Don't write the regression test to assert ONLY the rescue outcome (or ONLY the both-published
  outcome). The single-response stub makes rescue the likely path, but the CONTRACT is "no duplicate
  subjects" — assert that invariant across BOTH outcomes (rescue-with-1-commit OR both-distinct), so the
  test stays valid if a future stateful stub enables the success path.
- ❌ Don't set `cfg.Edit = true` in the BUG-002 test. That entangles BUG-002 with BUG-001 (T2.S2). Keep
  `cfg.Edit = false` to isolate the dedupe logic.

---

## Confidence Score: 9/10

The verbatim dedupe block, the verbatim res.err handler to mirror, the FROZEN signatures, the
seedRejections semantics, the FR-E3 before-EditMessage ordering, the T2.S2 content-anchor coordination,
and the test recipe (with the both-outcomes assertion) are all spelled out and verified against
fix_design.md Part 3 + the current source. The one residual (not a full 10) is the precise
commitSubject helper shape (how existing tests read `CommitResult.Subject`) — the implementer mirrors
whichever idiom the surrounding tests use, and the no-duplicate assertion is outcome-agnostic so a
helper-shape mismatch fails loudly, not silently. No signature changes, no new types, no concurrency-
model change; the regen reuses the landed generateMessageCore. One-pass success is highly likely.