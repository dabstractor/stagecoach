# Bug Fix Analysis: Work-Description Subsystem

## BUG-001 (MAJOR): Non-staged READ silently becomes the commit subject

### Root Cause
In `RunWorkDescription` (`internal/generate/workdesc.go`), the main loop (line ~109):

```go
for turn := 2; ; turn++ {
    paths := parseReadLines(out, skeleton)
    if len(paths) == 0 {
        // FR-W7: a response with no valid READ line is the commit-message candidate.
        m, parseOK, _ := provider.ParseOutput(out, manifest)
        return m, parseOK, nil
    }
    // ...
}
```

When the model emits `READ typo.go` (a path NOT in the staged skeleton), `parseReadLines` filters it out (the path is not in `skeletonPaths(skeleton)`), so `len(paths) == 0`. The code then treats the ENTIRE raw response as the commit message via `ParseOutput(out, manifest)` — **WITHOUT calling `stripReadLines`**.

The forced-conclusion path (later in the same function) DOES call `stripReadLines`:
```go
m, parseOK, _ := provider.ParseOutput(stripReadLines(out2), manifest)
```

So the bug is ONLY in the natural `len(paths) == 0` path.

### Fix Strategy
The fix must distinguish two cases in the `len(paths) == 0` branch:
1. **The response contained NO READ verb lines at all** → this IS a valid commit message (FR-W7) → parse it.
2. **The response contained READ verb lines, but ALL paths were non-staged** → this is NOT a message. Instead, emit FR-W3 notes for each non-staged path and continue the loop (bounded by the round cap).

A helper function `containsReadVerb(response string) bool` (or similar) can detect case 2. If case 2 is detected:
- Build an answer with FR-W3 notes (`<path> is not in the staged changes`) for the non-staged paths
- Continue the loop (the round counter still advances)
- The loop is still bounded by the round cap (FR-W6 forced conclusion eventually fires)

**Alternative approach**: Modify `parseReadLines` to also return the raw READ targets (before skeleton filtering) so the caller can detect case 2 and build the notes.

### Test Strategy
- Unit test: `parseReadLines("READ typo.go", skeleton)` returns `[]` (already tested via `TestParseReadLines_NonStagedIgnored`)
- New test: A response of ONLY `READ typo.go` should NOT be parsed as a commit message — instead it should produce an FR-W3 note and continue the loop.
- E2E test (via stubtest): Set the stub agent's first response to `READ typo.go`, assert the resulting commit subject is NOT `READ typo.go`.

### Files to Modify
- `internal/generate/workdesc.go` — the `len(paths) == 0` branch in `RunWorkDescription`

---

## BUG-002 (MINOR): Fully-read file emits empty body instead of 'end of diff' note

### Root Cause
In `buildReadAnswer` (line ~260):
```go
diff, err := g.StagedFileDiff(ctx, p, opts)
if err != nil || diff == "" {
    fmt.Fprintf(&b, "%s is not in the staged changes (or has no further diff).\n\n", p)
    continue
}
chunk, total, advance := nextChunk(diff, st.offsets[p])
if total <= 1 {
    fmt.Fprintf(&b, "%s:\n%s\n\n", p, chunk)
}
```

`StagedFileDiff` returns the FULL diff every call (cursor-unaware). When the cursor is exhausted (`st.offsets[p] >= len(diff)`), `nextChunk` returns `("", 1, 0)`. Then `total <= 1` is true, and `chunk` is `""`, so it prints `a.txt:\n\n` — an empty body.

The "end of diff" note only fires in the `diff == ""` branch (line ~270), which never fires for a staged file (diff is always the full non-empty diff).

### Fix Strategy
Before or after calling `nextChunk`, check whether the cursor is exhausted:
```go
if st.offsets[p] >= len(diff) {
    fmt.Fprintf(&b, "%s — end of diff (all parts shown).\n\n", p)
    continue
}
```
Per FR-W5: "After the final chunk, a re-request returns '<path> — end of diff (all N parts shown).'"

This check must go AFTER the `diff == ""` check (which covers non-staged files) and BEFORE the `nextChunk` call.

### Files to Modify
- `internal/generate/workdesc.go` — `buildReadAnswer` function

---

## BUG-005 (MINOR): Chunk boundaries anchor to newlines, not @@ hunk edges

### Root Cause
In `nextChunk` (line ~300):
```go
end := advanceRunes(diff, offset, budget)
// Anchor FORWARD to the next newline so a line is never split mid-line.
if i := strings.IndexByte(diff[end:], '\n'); i >= 0 {
    end += i + 1
} else {
    end = len(diff)
}
```

The forward anchor uses `strings.IndexByte(diff[end:], '\n')` — a newline boundary, NOT a `@@` hunk boundary. Per FR-W5: "Chunk boundaries hug `@@` hunk edges so a change is never split mid-hunk."

### Fix Strategy
Replace the newline anchor with a hunk-edge anchor. A git diff hunk boundary is a line starting with `@@` (the `@@ ... @@` hunk header). The forward scan should look for the next `\n@@` pattern after `end`.

Important: `chunkCount` (line ~330) must use the same anchoring logic so the `total` count stays consistent.

### Files to Modify
- `internal/generate/workdesc.go` — `nextChunk` function AND `chunkCount` function (must stay in sync)

---

## BUG-006 (MINOR): Part label divides byte offset by rune budget

### Root Cause
In `buildReadAnswer` (line ~284):
```go
part := (st.offsets[p] / chunkRuneBudget()) + 1
```

`st.offsets[p]` is a BYTE offset (advanced by `nextChunk`'s byte `advance` return value), but `chunkRuneBudget()` returns `readChunkTokenCap * 4` (64000 RUNES). For multibyte UTF-8, the byte offset grows faster than the rune offset, so the division produces a wrong part index.

### Fix Strategy
Either:
1. Track rune offsets alongside byte offsets in `readState.offsets`, OR
2. Convert the byte offset to a rune offset before dividing: `part := (utf8.RuneCountInString(diff[:st.offsets[p]]) / chunkRuneBudget()) + 1`

Option 2 is simpler and doesn't require changing the offsets type. However, note that `chunkCount` also needs to stay consistent — it already uses rune-windowing via `advanceRunes`, so the total count is correct in runes; only the `part` label is wrong.

### Files to Modify
- `internal/generate/workdesc.go` — the `part` computation in `buildReadAnswer`