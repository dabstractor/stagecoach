# System Context — Fast-Path Concurrency Bugs

## Overview

Stagecoach's multi-commit decomposition (`internal/decompose`) turns an un-staged dirty working
tree into N logically-coherent commits via a four-role pipeline (planner → stager → message →
arbiter). Two loop implementations exist:

1. **`runLoop`** (decompose.go:497) — the FR-M5 tooled-stager loop. 1-deep overlap: at most one
   message goroutine in flight at a time. `publish(inflight)` PRECEDES `launch(i, ...)` for each
   concept, so concept[i-1]'s commit is published before concept[i]'s message generation starts.
   This implicitly serializes editing (one editor at a time) and ensures the recent-subjects
   snapshot includes sibling commits (cross-concept dedupe works).

2. **`runLoopFastPath`** (decompose.go:680) — the FR-M13/FR-M14 file-disjoint fast-path. ALL N
   message generations launch CONCURRENTLY before any publish. The serial publish loop then
   receives messages in CAS order and publishes them one at a time. This is where both bugs live.

### Dispatch

`Decompose` (decompose.go:~286) selects the loop:
```go
if isFileDisjoint(out.Commits) {
    commits, chainData, err = runLoopFastPath(...)
} else {
    commits, chainData, err = runLoop(...)
}
```

`isFileDisjoint` (decompose.go:468) is a pure function: returns `true` if no path appears in more
than one concept's `Files` slice.

## Key Types

### `msgOut` (decompose.go:113) — result of a generateMessage goroutine
```go
type msgOut struct {
    conceptIdx int
    treeA      string  // parent concept's tree (or baseTree/EmptyTreeSHA)
    treeB      string  // this concept's frozen tree
    msg        string  // the generated commit message (subject + body)
    err        error
}
```

### `stagedConcept` (decompose.go:~658) — product of the fast-path serial staging sweep
```go
type stagedConcept struct {
    idx      int     // original concept index
    tree     string  // frozen write-tree SHA
    prevTree string  // tree it was staged on top of (tree[i-1], or baseTree)
}
```

### `CommitResult` (decompose.go:75)
```go
type CommitResult struct {
    SHA     string
    Subject string           // ExtractSubject(Message)
    Message string           // full commit message committed verbatim
    Files   []git.FileChange
}
```

## `generateMessage` (message.go:70) — the shared message-generation primitive

Signature: `func generateMessage(ctx, deps Deps, treeA, treeB string) (string, error)`

Pipeline:
1. RevParseHEAD → parentSHA, isUnborn (line 74)
2. System prompt building (line 83)
3. Tree-to-tree diff: `deps.Git.TreeDiff(ctx, treeA, treeB, ...)` (line 101)
4. Recent subjects: `messageRecentSubjects(ctx, deps.Git, isUnborn)` → `recent []string` (line 120)
5. Model/timeout resolution (line 127)
6. **Generation+dedupe loop** (lines 142-209): bounded by `MaxDuplicateRetries` (default 3 → 4 attempts)
   - Render → Execute → ParseOutput → FinalizeMessage → ExtractSubject
   - `generate.IsDuplicate(subject, recent)` → retry with augmented rejection list (FR32)
7. **EditMessage** (lines 212-219): `generate.EditMessage(ctx, msg, deps.Config, EditContext{...})`
   - Post-dedupe, pre-publish editor gate (FR-E1)
   - Writes to `<gitDir>/STAGECOACH_EDITMSG`, opens editor, strips comments
   - **THIS IS THE BUG-001 CALL SITE** — invoked inside the concurrent goroutine

### Callers of `generateMessage`
1. decompose.go:~371 — single-concept fast-path (planner bypassed)
2. decompose.go:~415 — re-generation after stager retree (runSingleShortcut)
3. decompose.go:~515 — **runLoop** (tooled-stager loop, 1-deep overlap)
4. decompose.go:~742 — **runLoopFastPath** (file-disjoint fast-path, concurrent) ← BUG-001 site
5. chain.go:~106 — resolveNewCommit (arbiter null/new-concept path)

## `EditMessage` (finalize.go:67) — the shared-file editor gate

```go
func EditMessage(ctx, msg string, cfg config.Config, editCtx EditContext) (string, error)
```

- `cfg.Edit == false` → identity (no-op, returns msg unchanged)
- `cfg.Edit == true`:
  1. Resolves `gitDir` via `editCtx.Git.GitDir(ctx)`
  2. Constructs **FIXED path**: `filepath.Join(gitDir, "STAGECOACH_EDITMSG")` (finalize.go:78)
  3. Writes `msg + commented summary` to that path
  4. Opens editor via `runEditorCommand(ctx, editor, editMsgPath)`
  5. Reads back, strips comments, returns edited message

**The fixed path is the root cause of BUG-001**: N concurrent goroutines all write to and read from
the same `STAGECOACH_EDITMSG` file, racing so that a concept may receive another concept's message.

## `messageRecentSubjects` (message.go:323) — the dedupe snapshot

```go
func messageRecentSubjects(ctx, g git.Git, isUnborn bool) ([]string, error)
```

- Unborn → `(nil, nil)`
- Otherwise → `g.RecentSubjects(ctx, 50)` → `git log --format=%s -50`

**Fetched ONCE per `generateMessage` call** at step 4 (line 120). On the fast-path, all N
goroutines fetch this snapshot concurrently BEFORE any concept is published, so they all see the
same pre-run HEAD history. No concept can observe a sibling's just-produced subject. **This is the
root cause of BUG-002.**

## Concurrency Primitives

- **Channels**: `chan msgOut`, always `make(chan msgOut, 1)` — buffered(1), goroutine sends once + exits.
  - `runLoop`: single `inflight chan msgOut` (1-deep overlap)
  - `runLoopFastPath`: `inflight []chan msgOut` of length N (all launched up-front)
- **Goroutine leak prevention**: `drainMsg(ch)` and `drainMsgs(chs)` drain channels on error/abort paths.
- **No mutexes, no WaitGroups** — channels are the only concurrency primitive.
- **Global snapshot toggle**: `signal.SetSnapshot/ClearSnapshot` — safe only because the publish loop is serial.