# Research Notes — P1.M1.T1.S1 (refactor generateMessage → generateMessageCore + wrapper)

Verification against the CURRENT working tree. The task description + `architecture/fix_design.md`
Part 1 are accurate. These notes record the exact verified anchors for a one-pass behavior-preserving
refactor.

## VERIFIED — current generateMessage (internal/decompose/message.go:70-221)
- Line 70: `func generateMessage(ctx context.Context, deps Deps, treeA, treeB string) (string, error)`
- Step 1 — RevParseHEAD (parentSHA, isUnborn): ~line 75
- Step 2 — system prompt + reserve + measureAssembled: ~lines 86-103
- Step 3 — TreeDiff(treeA, treeB): ~line 104 (empty-diff guard follows)
- Step 4 — `recent, err := messageRecentSubjects(...)`: **line 120** (fetched AFTER the system prompt;
  used ONLY at the dedupe check line 194 — NOT fed to the system prompt)
- Step 5+6 — generation+dedupe loop: lines 135-209
  - Line 135: `var rejected []string` (the rejection list passed to BuildUserPayload)
  - Line 194: `if generate.IsDuplicate(subject, recent)` ← uses `recent`
- Step 7 — EditMessage block: **lines 215-219**:
  - Line 215: `nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, treeA, treeB)` (best-effort)
  - Line 216: `msg, err = generate.EditMessage(ctx, msg, deps.Config, generate.EditContext{Git: deps.Git, TreeSHA: treeB, NameStatus: nameStatus})`
  - Lines 217-219: `if err != nil { return "", err }`
- Line 221: `return msg, nil`

## VERIFIED — ALL callers pass (ctx, deps, treeA, treeB) — signature UNCHANGED by the refactor
- internal/decompose/chain.go:106 — resolveNewCommit (arbiter N+1)
- internal/decompose/decompose.go:371 — single-escape/one-file shortcut
- internal/decompose/decompose.go:415 — single shortcut message regen
- internal/decompose/decompose.go:515 — runLoop (tooled-stager path)
- internal/decompose/decompose.go:742 — runLoopFastPath (concurrent fast-path; S2 switches this to generateMessageCore)
- internal/decompose/message_test.go — 6 call sites (TestGenerateMessage_*)

`generateMessage` keeps signature `(ctx, deps, treeA, treeB) (string, error)` ⇒ ZERO caller/test edits.
Only runLoopFastPath (S2, decompose.go:742) later switches to generateMessageCore.

## THE SPLIT (fix_design Part 1, verified against current code)

### generateMessageCore(ctx, deps, treeA, treeB, seedRejections []string) (string, error)
= current steps 1-6 (lines 75-209), with TWO seedRejections modifications:
1. Line 135 becomes: `rejected := append([]string{}, seedRejections...)` (pre-seed the rejection list
   passed to prompt.BuildUserPayload — tells the LLM upfront which subjects siblings took).
2. After line 120 (`recent`), add:
   ```go
   dedupeRecent := recent
   if len(seedRejections) > 0 {
       dedupeRecent = append(append([]string{}, recent...), seedRejections...)
   }
   ```
   Then line 194 uses `dedupeRecent` instead of `recent`.
- Ends with `return msg, nil` (the success return at line 210-ish). NO EditMessage. NO nameStatus fetch.
- `recent` is STILL fetched via messageRecentSubjects (unchanged) — only the dedupe CHECK uses dedupeRecent.
- parentSHA (step 1) stays — needed for the RescueError{ParentSHA: parentSHA} in the loop.

### generateMessage(ctx, deps, treeA, treeB) (string, error) — the wrapper
```go
msg, err := generateMessageCore(ctx, deps, treeA, treeB, nil)
if err != nil {
    return "", err
}
nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, treeA, treeB) // best-effort; "" on err
msg, err = generate.EditMessage(ctx, msg, deps.Config, generate.EditContext{Git: deps.Git, TreeSHA: treeB, NameStatus: nameStatus})
if err != nil {
    return "", err // ErrEmptyMessage → propagates to runLoop's FR-M12 handling
}
return msg, nil
```
= the EXISTING EditMessage block (lines 215-219) verbatim, prepended by the Core call with seedRejections=nil.

## BEHAVIOR-PRESERVATION PROOF (seedRejections=nil ⇒ identical to today)
When the wrapper passes nil:
- `append([]string{}, nil...)` = empty slice ≡ today's `var rejected []string`. ✓
- `len(nil) > 0` is false ⇒ `dedupeRecent` stays `==` recent ⇒ IsDuplicate(subject, recent) unchanged. ✓
- Core returns the same msg; wrapper applies the same EditMessage. ✓
⇒ Every existing caller + test sees byte-identical behavior. The 6 TestGenerateMessage_* tests MUST pass
without modification (that is the TDD gate for this refactor).

## ERROR-TYPE PRESERVATION (must be EXACT — move verbatim, do not re-wrap)
- `*generate.RescueError` (timeout / canceled / parse-exhaustion): returned DIRECTLY (not wrapped) from
  the loop — errors.As(err, &re) must keep working. Stays in Core.
- `ErrMessageFailed` (rev-parse / system-prompt / tree-diff / recent / render): `fmt.Errorf("%w: ...: %w", ...)`
  wrapped. Stays in Core.
- `generate.ErrEmptyMessage`: from EditMessage, returned bare (exit-1 path). Stays in the WRAPPER.
The refactor is a CUT-AND-PASTE of statements between two functions — no error-handling logic changes.

## THE `recent` vs `dedupeRecent` CLARIFICATION
`recent` (line 120) is fetched fresh each call via messageRecentSubjects (git history, includes
just-committed concepts). It is used ONLY at the IsDuplicate check (line 194) — it is NOT fed to the
system prompt (the system prompt uses messageSystemPrompt, which calls RecentMessages separately).
The task note "recent ... NOT merged with seedRejections for the system prompt building" is satisfied
trivially: recent is never merged into the prompt; only `rejected` (the BuildUserPayload rejection
block) is pre-seeded, and only the dedupe CHECK uses dedupeRecent. Keep `recent` pristine.

## SCOPE BOUNDARIES (sibling subtasks — do NOT implement here)
- **P1.M1.T2.S1**: switch runLoopFastPath's launch closure (decompose.go:742) from generateMessage →
  generateMessageCore (removes EditMessage from the concurrent goroutine). S1 only DEFINES Core.
- **P1.M1.T2.S2**: apply EditMessage in runLoopFastPath's serial publish loop (the BUG-001 fix proper).
- **P1.M2.T1.S1**: incremental cross-concept dedupe in the serial publish loop (BUG-002) — passes
  seedRejections to generateMessageCore. S1 only ADDS the seedRejections PARAMETER; it does not wire it.
- **P1.M3.*** : regression tests + concurrency-safety comment + docs.
- Do NOT: edit any caller (decompose.go:371/415/515/742, chain.go:106); edit EditMessage/finalize.go;
  change error types; or touch publishCommit/messageSystemPrompt/messageRecentSubjects. S1 is a pure
  extract-and-wrap of generateMessage inside message.go.

## DOC NOTES (Mode A)
- generateMessageCore doc comment: "concurrent-safe generation core (no EditMessage, no interactive
  I/O); used by runLoopFastPath's goroutines; seedRejections pre-seeds the dedupe rejection list for
  cross-concept retry."
- generateMessage doc comment: append "now delegates generation+dedupe to generateMessageCore and
  applies EditMessage (the interactive, non-concurrent-safe tail)."