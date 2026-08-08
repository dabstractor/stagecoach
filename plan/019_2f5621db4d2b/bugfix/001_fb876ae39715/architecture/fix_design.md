# Fix Design — generateMessageCore Extraction + Serial Loop Restructure

## Design Overview

Both bugs are fixed by restructuring `runLoopFastPath`'s serial publish loop. The fix has three
parts:

1. **Extract `generateMessageCore`** from `generateMessage` (generation+dedupe without EditMessage)
2. **Move EditMessage to the serial publish loop** (BUG-001 fix)
3. **Add incremental cross-concept dedupe to the serial publish loop** (BUG-002 fix)

## Part 1: `generateMessageCore` Extraction

### Current `generateMessage` (message.go:70-222)

```
generateMessage(ctx, deps, treeA, treeB):
    1. RevParseHEAD → parentSHA, isUnborn
    2. Build system prompt
    3. TreeDiff(treeA, treeB) → diff
    4. messageRecentSubjects → recent
    5. Resolve model/timeout
    6. Generation+dedupe loop (retry up to MaxDuplicateRetries+1)
       - Render → Execute → ParseOutput → FinalizeMessage → ExtractSubject
       - IsDuplicate(subject, recent) → retry with augmented rejection list
    7. EditMessage(ctx, msg, cfg, EditContext{...})   ← THE PROBLEMATIC CALL
    8. Return msg
```

### Proposed split

```
generateMessageCore(ctx, deps, treeA, treeB, seedRejections []string) (string, error):
    1-6. SAME as above (steps 1-6 of generateMessage)
         BUT: dedupe check uses dedupeRecent = recent ∪ seedRejections
         AND: rejected list starts with seedRejections (for the prompt's rejection block)
    7. Return msg, nil   ← NO EditMessage

generateMessage(ctx, deps, treeA, treeB) (string, error):
    msg, err := generateMessageCore(ctx, deps, treeA, treeB, nil)
    if err != nil: return "", err
    nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, treeA, treeB)
    msg, err = generate.EditMessage(ctx, msg, deps.Config, generate.EditContext{...})
    if err != nil: return "", err
    return msg, nil
```

### `seedRejections` integration

In `generateMessageCore`'s dedupe loop (current message.go:142-209):

```go
// BEFORE (in generateMessage):
var rejected []string                    // starts empty
recent := messageRecentSubjects(...)     // git history

// AFTER (in generateMessageCore):
rejected := append([]string{}, seedRejections...)  // pre-seeded
recent := messageRecentSubjects(...)
dedupeRecent := recent
if len(seedRejections) > 0 {
    dedupeRecent = append(append([]string{}, recent...), seedRejections...)
}
// In the loop:
if generate.IsDuplicate(subject, dedupeRecent) {   // checks recent ∪ seedRejections
    rejected = append(rejected, subject)
    continue
}
```

The `rejected` slice is passed to `prompt.BuildUserPayload` (which builds the "avoid these subjects"
rejection block in the prompt). Pre-seeding it with `seedRejections` tells the LLM upfront which
subjects siblings already took.

### Impact on existing callers

All existing callers of `generateMessage` are UNCHANGED:
- `runLoop` (decompose.go:~515): calls `generateMessage` → `generateMessageCore(..., nil)` + EditMessage. Same behavior.
- `runSingleEscape` / `runSingleShortcut` / `runOneFileShortcut`: unchanged.
- `chain.go resolveNewCommit`: unchanged.

Only `runLoopFastPath` switches from `generateMessage` to `generateMessageCore` in its goroutines.

## Part 2: BUG-001 Fix — EditMessage in serial publish loop

### Modified `runLoopFastPath` launch closure (decompose.go:~749)

```go
// BEFORE:
launch := func(i int, treeA, treeB string) chan msgOut {
    ch := make(chan msgOut, 1)
    go func() {
        m, e := generateMessage(ctx, deps, treeA, treeB)  // includes EditMessage
        ch <- msgOut{conceptIdx: i, treeA: treeA, treeB: treeB, msg: m, err: e}
    }()
    return ch
}

// AFTER:
launch := func(i int, treeA, treeB string) chan msgOut {
    ch := make(chan msgOut, 1)
    go func() {
        m, e := generateMessageCore(ctx, deps, treeA, treeB, nil)  // NO EditMessage
        ch <- msgOut{conceptIdx: i, treeA: treeA, treeB: treeB, msg: m, err: e}
    }()
    return ch
}
```

### Modified serial publish loop (decompose.go:~770-828)

After `res := <-ch` and error handling, BEFORE `publishCommit`:

```go
// BUG-001 fix: apply EditMessage in the SERIAL loop (one editor at a time).
// The serial loop guarantees only one EditMessage is active at a time, so the
// shared STAGECOACH_EDITMSG file is safe. (FR-E4: --edit gates each commit's
// message before its already-serialized publication.)
if deps.Config.Edit {
    nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, sc.prevTree, sc.tree)
    edited, eErr := generate.EditMessage(ctx, res.msg, deps.Config, generate.EditContext{
        Git: deps.Git, TreeSHA: sc.tree, NameStatus: nameStatus,
    })
    if eErr != nil {
        // ErrEmptyMessage or editor abort — same handling as generateMessage's current behavior.
        // Treat as a non-rescue hard error (exit 1 for ErrEmptyMessage; wrapped error otherwise).
        drainMsgs(inflight[i+1:])
        return commits, nil, eErr
    }
    res.msg = edited
}
```

## Part 3: BUG-002 Fix — Incremental cross-concept dedupe

### In the serial publish loop, BEFORE the EditMessage block:

```go
// BUG-002 fix: cross-concept dedupe. Check this concept's generated subject
// against a growing set of seen subjects (pre-run history + already-decided siblings).
// Because concept[i-1] was published in the previous iteration, its subject is also
// in git history — but we check explicitly here for clarity and safety.
subject := generate.ExtractSubject(res.msg)
if generate.IsDuplicate(subject, seenSubjects) {
    // Collision with a sibling or prior commit. Re-generate with seedRejections.
    regenerated, regErr := generateMessageCore(ctx, deps, sc.prevTree, sc.tree, seenSubjects)
    if regErr != nil {
        // Re-generation failed (rescue or hard error) — handle same as initial gen failure.
        var re *generate.RescueError
        if errors.As(regErr, &re) {
            fixed := *re; fixed.ParentSHA = prevSHA
            // ... (same rescue handling as existing code)
            return commits, nil, &DecomposeRescueError{...}
        }
        drainMsgs(inflight[i+1:])
        return commits, nil, regErr
    }
    res.msg = regenerated
    subject = generate.ExtractSubject(res.msg)
    // Belt-and-suspenders: if STILL a duplicate after re-generation, rescue.
    if generate.IsDuplicate(subject, seenSubjects) {
        // This should not happen (generateMessageCore's dedupe loop should have caught it),
        // but handle it safely.
        drainMsgs(inflight[i+1:])
        return commits, nil, &DecomposeRescueError{...}
    }
}
seenSubjects = append(seenSubjects, subject)
```

### `seenSubjects` initialization

Before the serial publish loop:
```go
seenSubjects, _ := messageRecentSubjects(ctx, deps.Git, isUnborn)
```

This is the same snapshot each goroutine fetches, but maintained as a GROWING set that accumulates
each concept's accepted subject as the loop progresses.

## Combined serial loop order

```
for i, ch := range inflight:
    1. signal.SetSnapshot + res := <-ch + signal.ClearSnapshot    [EXISTING]
    2. Error handling (rescue/hard)                                [EXISTING]
    3. Cross-concept dedupe check + re-generation                  [NEW — BUG-002]
    4. seenSubjects.append(subject)                                [NEW — BUG-002]
    5. EditMessage (if cfg.Edit)                                   [NEW — BUG-001]
    6. publishCommit (CAS)                                         [EXISTING]
    7. buildCommitResult + chainData.append                        [EXISTING]
    8. prevSHA = newSHA                                            [EXISTING]
```

## Files Modified

1. **`internal/decompose/message.go`**: Extract `generateMessageCore` from `generateMessage`. Add
   `seedRejections` parameter to the core function. `generateMessage` becomes a thin wrapper.

2. **`internal/decompose/decompose.go`**: In `runLoopFastPath`:
   - Change `launch` closure to call `generateMessageCore` instead of `generateMessage`.
   - Add `seenSubjects` initialization before the serial publish loop.
   - Add cross-concept dedupe check + re-generation in the serial loop.
   - Add EditMessage application in the serial loop.
   - Update the concurrency-safety comment (decompose.go:745-749).

3. **`internal/decompose/decompose_test.go`** (or a new `*_test.go`): Regression tests for both bugs.

## What Does NOT Change

- `runLoop` — unaffected (1-deep overlap is safe for both EditMessage and dedupe).
- `runSingleEscape` / `runSingleShortcut` / `runOneFileShortcut` — call `generateMessage` unchanged.
- `chain.go resolveNewCommit` — calls `generateMessage` unchanged.
- `generate.EditMessage` signature — FROZEN, no changes.
- `generate.IsDuplicate` / `generate.ExtractSubject` signatures — FROZEN, no changes.
- `generate/finalize.go` — no changes (EditMessage called from a new site, same signature).
- `generate/dedupe.go` — no changes.
- `internal/git/` — no changes.
- `internal/config/` — no changes.
- `spec/SPEC.md` — READ-ONLY, no changes (these are bug fixes against existing requirements).