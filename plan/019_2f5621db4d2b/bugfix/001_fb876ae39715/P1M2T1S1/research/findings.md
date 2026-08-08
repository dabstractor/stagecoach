# Research: P1.M2.T1.S1 — Cross-concept dedupe in runLoopFastPath's serial publish loop (BUG-002)

Fix BUG-002: the file-disjoint fast-path launches all N message generations concurrently BEFORE any
publish, so each generateMessage sees only pre-run history → two disjoint concepts with the same
emitted subject both publish (violates US7/FR30-33). Fix = a `seenSubjects` accumulator in the SERIAL
publish loop: check each concept's subject, re-generate on collision, rescue if still colliding.

All claims verified against fix_design.md Part 3, the current decompose.go (post P1.M1.T1.S1 + T2.S1),
generate/dedupe.go + generate.go (FROZEN signatures), message.go, and test_strategy.md BUG-002.

---

## 0. THE BUG (BUG-002) — confirmed in selected_prd_content h3.1

runLoopFastPath (decompose.go:680) launches ALL N `generateMessageCore` goroutines concurrently
(decompose.go:~745-768), THEN publishes in CAS order in the serial loop (~774-825). Each goroutine's
`generateMessageCore` fetches recent subjects ONCE via `messageRecentSubjects` (pre-run HEAD history)
and dedupes against that snapshot. Because publication is serialized AFTER all generation, NO concept
can observe a sibling's just-produced subject → two disjoint concepts with the same subject both pass
the per-concept dedupe and both publish. (runLoop, the tooled-stager path, does NOT have this: it
generates message[i] only AFTER publishing concept[i-1], so the fresh fetch includes concept[i-1].)

---

## 1. THE FIX SITE — `internal/decompose/decompose.go`, runLoopFastPath

Signature: `func runLoopFastPath(ctx, deps Deps, concepts []prompt.PlannerCommit, baseTree, tStart, preRunHEAD string, isUnborn bool) ([]CommitResult, []ChainEntry, error)` (decompose.go:680).

### (a) seenSubjects init — BEFORE the serial publish loop
After the `if len(staged) == 0 { return … }` guard (~line 741) and `prevSHA := preRunHEAD` (~line 773):
```go
seenSubjects, _ := messageRecentSubjects(ctx, deps.Git, isUnborn) // BUG-002: cross-concept dedupe accumulator
```
`isUnborn` and `preRunHEAD` are runLoopFastPath params (in scope). `messageRecentSubjects` returns
`([]string, error)` (message.go:366); the error is intentionally ignored (`_`) — a history-fetch failure
must not abort the run (matches each goroutine's own best-effort fetch; dedupe degrades to "no history"
which is safe — only sibling-collision detection is weakened, never a false positive).

### (b) dedupe check — INSIDE the serial loop, AFTER the `if res.err != nil` block, BEFORE EditMessage
The serial loop body order (after BOTH this task + parallel T2.S2 land), per fix_design.md "Combined
serial loop order":
```
1. signal.SetSnapshot + res := <-ch + signal.ClearSnapshot    [EXISTING]
2. Error handling (res.err → rescue/hard)                      [EXISTING]
3. Cross-concept dedupe check + re-generation                  [NEW — THIS TASK (BUG-002)]
4. seenSubjects.append(subject)                                [NEW — THIS TASK]
5. EditMessage (if cfg.Edit)                                   [NEW — parallel P1.M1.T2.S2 (BUG-001)]
6. publishCommit (CAS)                                         [EXISTING]
7. buildCommitResult + chainData.append                        [EXISTING]
8. prevSHA = newSHA                                            [EXISTING]
```
**Insertion anchor (by CONTENT, not line — T2.S2 is in-flight and shifts lines):** immediately after
the closing `}` of the `if res.err != nil { … }` block, and BEFORE whichever comes next — the
`if deps.Config.Edit { … }` block (if T2.S2 has landed) OR the `// Publish in CAS order` /
`publishCommit` call (if T2.S2 has not). The dedupe check MUST precede EditMessage so it judges the
GENERATED (pre-edit) subject — preserving FR-E3 ("edited message bypasses the re-check").

---

## 2. THE FROZEN SIGNATURES (verified — do NOT change)

| Symbol | Signature | Location |
|--------|-----------|----------|
| `generate.ExtractSubject` | `func ExtractSubject(message string) string` | generate/dedupe.go:19 |
| `generate.IsDuplicate` | `func IsDuplicate(subject string, recent []string) bool` | generate/dedupe.go:46 |
| `generateMessageCore` | `func generateMessageCore(ctx, deps Deps, treeA, treeB string, seedRejections []string) (string, error)` | decompose/message.go:80 (P1.M1.T1.S1, LANDED) |
| `messageRecentSubjects` | `func messageRecentSubjects(ctx, g git.Git, isUnborn bool) ([]string, error)` | decompose/message.go:366 |
| `generate.RescueError` | `struct{ Kind error; TreeSHA, ParentSHA, Candidate, Cause string }` | generate/generate.go:112 |
| `generate.ErrRescue` | `var ErrRescue = errors.New(...)` | generate/generate.go:95 |
| `generate.FormatRescueMulti` | `func FormatRescueMulti(treeSHA, parentSHA, candidateMsg, conceptTitle string, index, count int) string` | generate/rescue.go:88 |

`generateMessageCore`'s dedupe loop uses `seedRejections` BOTH as extra entries in the dedupe set AND
as the prompt's "avoid these subjects" rejection block (fix_design.md Part 1). So passing `seenSubjects`
as seedRejections tells the LLM upfront which subjects siblings already took.

---

## 3. THE EXISTING res.err HANDLER (the template my regen-error path mirrors)

From the current serial loop (decompose.go:~781-796), the `if res.err != nil` block:
```go
if res.err != nil {
    var re *generate.RescueError
    if errors.As(res.err, &re) {
        title := ""
        if sc.idx < len(concepts) { title = concepts[sc.idx].Title }
        fixed := *re
        fixed.ParentSHA = prevSHA                       // prevSHA is AUTHORITATIVE (concurrent gen may have a stale ParentSHA)
        if deps.Out != nil {
            fmt.Fprintln(deps.Out, generate.FormatRescueMulti(fixed.TreeSHA, fixed.ParentSHA, fixed.Candidate, title, sc.idx, len(concepts)))
        }
        drainMsgs(inflight[i+1:])
        return commits, nil, &DecomposeRescueError{Rescue: &fixed, ConceptTitle: title, Index: sc.idx, Count: len(concepts), Commits: commits}
    }
    drainMsgs(inflight[i+1:])
    return commits, nil, res.err
}
```
**My re-generation error path mirrors this EXACTLY** (RescueError → fix ParentSHA=prevSHA + print
FormatRescueMulti + drainMsgs + return DecomposeRescueError; non-rescue → drainMsgs + return regErr).
NOTE: `title` is computed INSIDE the res.err block, so it is NOT in scope at my dedupe block — compute
it locally in the regen-error path (2-line repeat; do NOT refactor the existing handler).

`DecomposeRescueError` fields (from the handler): `Rescue *generate.RescueError`, `ConceptTitle string`,
`Index int`, `Count int`, `Commits []CommitResult`.

---

## 4. THE DEDUPE BLOCK (verbatim, to insert at step 3)

```go
// BUG-002: cross-concept dedupe. The fast-path generates all N messages concurrently BEFORE any
// publish, so the per-concept generateMessageCore could only see pre-run history — two siblings
// with the same emitted subject both passed. This serial-loop check closes the gap: judge the
// generated subject against a GROWING set (pre-run history + already-published siblings). On
// collision, re-generate with seedRejections=seenSubjects so the LLM avoids them; if it STILL
// collides (or regen fails), rescue (prior commits stand, FR-M12). Placed BEFORE EditMessage so it
// judges the generated subject (FR-E3: edited messages bypass the re-check).
subject := generate.ExtractSubject(res.msg)
if generate.IsDuplicate(subject, seenSubjects) {
    regenerated, regErr := generateMessageCore(ctx, deps, sc.prevTree, sc.tree, seenSubjects)
    if regErr != nil {
        var re *generate.RescueError
        if errors.As(regErr, &re) {
            title := ""
            if sc.idx < len(concepts) { title = concepts[sc.idx].Title }
            fixed := *re
            fixed.ParentSHA = prevSHA
            if deps.Out != nil {
                fmt.Fprintln(deps.Out, generate.FormatRescueMulti(fixed.TreeSHA, fixed.ParentSHA, fixed.Candidate, title, sc.idx, len(concepts)))
            }
            drainMsgs(inflight[i+1:])
            return commits, nil, &DecomposeRescueError{Rescue: &fixed, ConceptTitle: title, Index: sc.idx, Count: len(concepts), Commits: commits}
        }
        drainMsgs(inflight[i+1:])
        return commits, nil, regErr
    }
    res.msg = regenerated
    subject = generate.ExtractSubject(res.msg)
    if generate.IsDuplicate(subject, seenSubjects) {
        // Still a duplicate after re-generation (generateMessageCore's loop exhausted without a
        // distinct subject). Rescue: concept[i] abandoned, commits 0..i-1 stand (FR-M12).
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
```

`sc.prevTree`/`sc.tree` are the staged-concept's treeA/treeB (same args the goroutine generated
against — a faithful re-generation). `prevSHA` is the authoritative publish-time parent.

---

## 5. COORDINATION WITH PARALLEL P1.M1.T2.S2 (EditMessage block)

- T2.S2 adds an `if deps.Config.Edit { generate.EditMessage(...); res.msg = edited; on err drainMsgs+return }`
  block at serial-loop step 5 (between my dedupe block and publishCommit).
- **My dedupe block (step 3-4) MUST come BEFORE T2.S2's EditMessage block (step 5)** so the dedupe
  judges the pre-edit subject (FR-E3).
- **No overlap**: T2.S2 touches ONLY the EditMessage block; I touch ONLY seenSubjects init + the dedupe
  block. Neither edits the other's lines.
- **Anchor by content** (T2.S2 is in-flight → line numbers drift): insert "after the `if res.err != nil`
  block's closing `}`, before the `if deps.Config.Edit` block (or, if T2.S2 hasn't landed, before
  `// Publish in CAS order`)". Either order of landing is safe — the content anchor absorbs the drift.

---

## 6. SCOPE FENCES

- **ONLY `internal/decompose/decompose.go` (the seenSubjects init + dedupe block) + a regression test
  in `internal/decompose/decompose_test.go` (or a new *_test.go).** No edit to message.go (generateMessageCore
  is LANDED + FROZEN), generate/dedupe.go, generate/generate.go (RescueError/FormatRescueMulti FROZEN),
  runLoop, the launch closure, publishCommit, or any config/git/provider file.
- **NO EditMessage logic** (that's T2.S2). My block is dedupe-only.
- **NO changes to ExtractSubject/IsDuplicate/generateMessageCore/messageRecentSubjects signatures**
  (FROZEN). generateMessageCore already accepts seedRejections (P1.M1.T1.S1) — I only CALL it.
- **NO docs** (internal dedupe logic; no user-facing surface).

---

## 7. TEST (mirror TestRunLoopFastPath_ConcurrentPublish @ decompose_test.go:3142)

Per test_strategy.md BUG-002:
- 2 disjoint concepts (a.txt, b.txt), BORN repo, disjoint dirty changes, FreezeWorkingTree → tStart.
- Message stub returns the SAME subject for both: `dcmMessageMatchManifest(t, bin, []messageMatchRule{
    {substr: "a.txt", msg: "chore: update thing"}, {substr: "b.txt", msg: "chore: update thing"}})`.
- `cfg.Edit = false` (isolate BUG-002 from BUG-001).
- Call `runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)` directly.
- **Assertions** (rescue case — expected with the single-response stub):
  - EITHER a `*DecomposeRescueError` is returned AND `len(err.Commits) == 1` (concept 0 published, concept 1 rescued);
  - OR `err == nil` AND `len(commits) == 2` AND `commits[0].Subject != commits[1].Subject`;
  - IN EITHER CASE: no two published commits share a subject (collect subjects from `commits` OR
    `err.Commits`, dedupe, assert all distinct).
- Expected flow (rescue): concept 0 accepted+published; concept 1 collides → regenerate → stub returns
  same subject → generateMessageCore exhausts retries → RescueError → concept 1 rescued, concept 0 stands.
- Optionally set `cfg.MaxDuplicateRetries = 0` to force immediate rescue (simpler/faster; still valid).

Helpers (decompose_test.go): `dcmInitRepo`, `dcmWriteFile`, `dcmRunGit`, `dcmCommitRaw`, `dcmGitOut`,
`dcmHeadSHA`, `dcmMessageMatchManifest`(@147), `messageMatchRule`(@172), `dcmDeps`/`dcmDepsWithConfig`,
`stubtest.Build`.

**NOTE (test-strategy limitation):** `dcmMessageMatchManifest` is single-response per rule, so the
re-generation returns the SAME subject → rescue. That is the expected/sufficient assertion. A
stateful-stub success-case (regen produces a distinct subject → both published) is optional extra
credit; the rescue case fully verifies "no duplicate subjects."

---

## 8. Validation (Makefile)

- Build: `go build ./...`
- Vet: `go vet ./internal/decompose/...`
- Format: `gofmt -l internal/decompose/decompose.go internal/decompose/decompose_test.go` (empty)
- Focused: `go test ./internal/decompose/ -run 'FastPath' -v` (incl. the new BUG-002 test + existing TestRunLoopFastPath_ConcurrentPublish)
- Race: `go test -race ./internal/decompose/...` (the dedupe runs in the serial loop — but race confirms no regression)
- Full: `make test`; lint: `make lint`
- Manual proof: the regression test IS the proof (2 same-subject concepts → no duplicate subjects).