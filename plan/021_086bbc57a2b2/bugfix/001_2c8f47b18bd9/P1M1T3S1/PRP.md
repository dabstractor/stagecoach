name: "P1.M1.T3.S1 — Anchor nextChunk/chunkCount chunk boundaries to @@ hunk edges (BUG-005)"
description: >
  Fix BUG-005 (Minor): internal/generate/workdesc.go nextChunk (~398) and chunkCount (~423) forward-anchor
  chunk boundaries to a NEWLINE (strings.IndexByte(diff[end:], '\n')), which can split a git diff hunk
  mid-hunk. Per FR-W5 ("Chunk boundaries hug @@ hunk edges so a change is never split mid-hunk; a single
  hunk exceeding the cap falls back to a line cut"), anchor to the next \n@@ (a new hunk header) instead,
  falling back to a newline when no @@ hunk edge follows. Extract a SHARED helper anchorToHunkEdge(diff,
  end) int called by BOTH nextChunk and chunkCount so the "part i of N" total stays consistent with the
  real boundaries. Mode-A doc-comment updates on both functions. + 2 regression tests (direct helper +
  chunkCount consistency). Only workdesc.go + generate_workdesc_test.go touched. No signature/behavior
  change beyond the boundary anchor; buildReadAnswer (BUG-002) and the part-label (BUG-006) are siblings.

---

## Goal

**Feature Goal**: Make the work-description mode's diff-chunk boundaries hug `@@` hunk edges (FR-W5) so
a git diff hunk is never split across two "part i of N" chunks delivered to the agent — improving the
quality of the read-on-demand diff payload (a split hunk forces the agent to reason about an incomplete
change). A single hunk exceeding the per-call cap still falls back to a line cut (FR-W5).

**Deliverable**: (1) a new shared helper `anchorToHunkEdge(diff, end) int` in `internal/generate/workdesc.go`;
(2) `nextChunk` and `chunkCount` both call it (replacing their two inline `strings.IndexByte(diff[end:], '\n')`
blocks); (3) Mode-A doc-comment rewrites on both functions (the current comments claim "newline"/"without
forward-anchoring" which become false); (4) two regression tests in `internal/generate/generate_workdesc_test.go`.

**Success Definition**:
- For a multi-hunk diff where the rune budget lands mid-hunk, the chunk boundary falls at the START of
  the next hunk (the `\n@@` edge), not mid-hunk.
- For a single hunk exceeding the cap (no later `@@`), the boundary falls back to the next newline
  (FR-W5 "line cut") — unchanged from today for that case.
- `nextChunk` and `chunkCount` use the IDENTICAL anchoring (one shared helper) → the `total` count
  matches the real chunk boundaries → the "part i of N" label is consistent.
- Existing `TestNextChunk_SmallDiffIsOneChunk` still passes (a small single-hunk diff is one chunk;
  the @@ anchor is a no-op when no second hunk follows).
- `go build ./...`, `go vet ./internal/generate/...`, `go test ./internal/generate/ -count=1`, `make test`,
  `make lint` all pass; gofmt clean. No signature/const/budget change.

## User Persona (if applicable)

**Target User**: The agent consuming a work-description-mode diff chunk (read-on-demand, §9.26 FR-W5),
and indirectly the user whose large staged file is being described in chunks.

**Use Case**: A model READs a large staged file; stagecoach returns the diff in ≤16K-token chunks. With
the fix, each chunk boundary is a hunk edge, so the model never sees a hunk cut in half (e.g. the `-` line
in one chunk and the `+` line in the next), yielding a more coherent description.

**Pain Points Addressed**: BUG-005 — chunk boundaries anchored to newlines could split a hunk mid-hunk,
degrading the agent's understanding of an individual change.

## Why

- **BUG-005 (Minor) / FR-W5**: the spec explicitly mandates `@@` hunk-edge boundaries; the current
  newline anchor violates it whenever a hunk straddles the rune-budget position. This is a quality polish
  (the architecture analysis groups it under "chunk/label quality").
- **Consistency is the load-bearing requirement**: `nextChunk` reports `total = chunkCount(...)` and the
  caller (`buildReadAnswer` @383) derives the "part i of N" label. If `nextChunk` and `chunkCount`
  anchor differently, the label lies about which chunk you're in. Extracting ONE helper used by both
  makes consistency structural, not coincidental.
- **Bounded, surgical**: one new helper + two call-site replacements + two comment rewrites + two tests.
  No signature change (nextChunk keeps its fixed budget; chunkCount keeps its explicit budget param).
  Sibling bugs (BUG-002 buildReadAnswer, BUG-006 part-label) are different lines/functions.

## What

**User-visible behavior**: marginally better diff-chunk coherence in work-description mode (hunk-aligned
boundaries). No CLI/config/API change. The "part i of N" count is unchanged for diffs that already
chunked on hunk edges; it may change (become accurate) for diffs that previously split mid-hunk.

**Technical change**:
1. `workdesc.go` — add `anchorToHunkEdge(diff, end) int` (scan `\n@@` → newline → len(diff)).
2. `workdesc.go` `nextChunk` — replace the 5-line `if i := strings.IndexByte(diff[end:], '\n'); ...` block
   (and the now-redundant `if end > len(diff)` clamp) with `end = anchorToHunkEdge(diff, end)`.
3. `workdesc.go` `chunkCount` — replace its 5-line `if i := strings.IndexByte(diff[end:], '\n'); ...`
   block with `end = anchorToHunkEdge(diff, end)`.
4. `workdesc.go` — Mode-A rewrite of the `nextChunk` doc comment (the "hug newline edges … never split
   mid-line" sentence) and the `chunkCount` doc comment (the "without forward-anchoring" sentence).
5. `generate_workdesc_test.go` — add `TestAnchorToHunkEdge` + `TestChunkCount_HunkBoundaries`.

### Success Criteria
- [ ] `anchorToHunkEdge` exists and is called by BOTH `nextChunk` and `chunkCount`
- [ ] multi-hunk diff: boundary lands at the `\n@@` edge (next hunk header)
- [ ] single-hunk-exceeds-cap / no later `@@`: falls back to the newline anchor (FR-W5 line cut)
- [ ] no `\n` follows: returns `len(diff)`
- [ ] `nextChunk` and `chunkCount` anchor identically (one helper) → `total` matches boundaries
- [ ] existing `TestNextChunk_SmallDiffIsOneChunk` still passes
- [ ] `nextChunk` doc comment cites FR-W5 @@ hunk-edge anchoring; `chunkCount` doc comment notes the shared helper
- [ ] `go build ./...`, `go vet ./internal/generate/...`, `go test ./internal/generate/ -count=1`, `make test`, `make lint` pass; gofmt clean

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact current `nextChunk`/`chunkCount` source (verbatim, with the buggy inline blocks), the
exact replacement helper, the same-package `advanceRunes` location (multiturn.go:104), the FR-W5 quote,
the doc-comment sentences that become false, the existing test to mirror (`TestNextChunk_SmallDiffIsOneChunk`
@171), the verbatim new tests, and the scope fences against the sibling bugs (different functions) are all below.

### Documentation & References

```yaml
# MUST READ — the file under the fix
- file: internal/generate/workdesc.go
  why: "THE change site. nextChunk @398-415 (buggy anchor @406-410); chunkCount @423-444 (buggy anchor
        @433-437); chunkRuneBudget @419 (=64000); nextChunk doc @392-397; chunkCount doc @420-422."
  pattern: "both anchor blocks are the identical 5-line `if i := strings.IndexByte(diff[end:], '\\n');
            i >= 0 { end += i + 1 } else { end = len(diff) }`. Replace each with `end = anchorToHunkEdge(diff, end)`."
  critical: "nextChunk also has a redundant `if end > len(diff) { end = len(diff) }` clamp AFTER the
             anchor block — the helper clamps internally, so DROP that clamp when you swap the block.
             chunkCount has NO such clamp (its else-branch sets len(diff)); just swap its block."

# MUST READ — the same-package helper both functions already use (DO NOT change)
- file: internal/generate/multiturn.go
  why: "advanceRunes(s string, start, n int) int @104 — package generate (no import). Returns the byte
        index reached scanning n runes from start. nextChunk and chunkCount call it to get the initial
        budget position BEFORE the anchor. Not modified by this task."
  critical: "do NOT move/duplicate advanceRunes — just call anchorToHunkEdge AFTER it, exactly as the
             current code calls the newline anchor after it."

# DECISION PROVENANCE
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/architecture/bugfix_workdesc.md
  why: "§BUG-005 is the authoritative analysis: the newline-anchor root cause, the FR-W5 @@ requirement,
        the 'chunkCount MUST use the same anchoring so total stays consistent' invariant, and the
        'files: nextChunk AND chunkCount' scope. This task implements it."
  section: "BUG-005 (MINOR) — Chunk boundaries anchor to newlines, not @@ hunk edges"

# SPEC — the requirement the anchor satisfies
- file: spec/01-product.md
  why: "FR-W5 @~526: 'Chunk boundaries hug @@ hunk edges so a change is never split mid-hunk; a single
        hunk exceeding the cap falls back to a line cut (noted in the label).' Cite this in the Mode-A
        doc comment."

# TEST PATTERNS (mirror; do not edit the existing tests)
- file: internal/generate/generate_workdesc_test.go
  why: "package generate (white-box — nextChunk/chunkCount/anchorToHunkEdge reachable). TestNextChunk_
        SmallDiffIsOneChunk @171 is the EXACT shape to mirror: `chunk, total, advance := nextChunk(diff, 0)`;
        assert `chunk == diff` for one chunk; `nextChunk(diff, len(diff))` → (\"\",_,0). MUST still pass."
  pattern: "direct unit calls on synthetic string diffs; `errors`/`strings`/`testing` imports (check the
            import block — ADD \"strings\" if a new test uses strings.Index and it isn't imported)."
  critical: "anchor new tests by NAME not line number — the BUG-001 sibling (@135-205) and BUG-002 sibling
             add helpers/tests that shift line numbers. nextChunk uses the FIXED 64000 budget internally
             (no budget param) ⇒ exercise @@ anchoring via the helper directly + chunkCount (which takes
             an explicit runeBudget)."

# SIBLING CONTRACTS (same file, different functions — confirm non-overlap)
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M1T2S1/PRP.md
  why: "BUG-002 edits buildReadAnswer (@362, the cursor-exhaustion branch) — DIFFERENT function; explicitly
        scopes OUT nextChunk/chunkCount/chunkRuneBudget. No conflict with the anchor edit."
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  workdesc.go                    # THE fix: +anchorToHunkEdge; nextChunk + chunkCount call it; 2 doc-comment rewrites
  multiturn.go                   # advanceRunes @104 (same package; REUSE — do NOT edit)
  generate_workdesc_test.go      # ADD: TestAnchorToHunkEdge + TestChunkCount_HunkBoundaries
```

### Desired Codebase tree with files to be added

```bash
internal/generate/workdesc.go                # MODIFY: +anchorToHunkEdge helper; 2 anchor call-sites; 2 doc comments (Mode A)
internal/generate/generate_workdesc_test.go  # MODIFY: +2 test funcs
# (no new files; no signature/const/budget change; no multiturn.go/buildReadAnswer/part-label change)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (BOTH functions MUST call the SAME helper): nextChunk reports total = chunkCount(...) and the
//   caller derives "part i of N" from offsets/budget. If nextChunk anchors to @@ but chunkCount anchors
//   to newline (or vice versa), the label lies about which chunk you're in. Extract anchorToHunkEdge and
//   call it from BOTH — do not leave one on the inline newline anchor "because it's equivalent" (it isn't,
//   and a future edit would silently desync them).

// CRITICAL (search "\n@@", not "@@"): a git diff hunk HEADER is a line beginning "@@" — i.e. preceded by
//   a newline (or the start of the diff). Scanning for "\n@@" from diff[end:] finds the next hunk boundary.
//   end += i + 1 INCLUDES the newline before @@, so the @@ header line starts the NEXT chunk (the current
//   chunk ends at the line before the new hunk). Scanning for bare "@@" would also match the @@ of the
//   hunk the cursor is already inside, or a @@ inside the hunk body — wrong. Use "\n@@".

// CRITICAL (FR-W5 two-tier fallback): if NO "\n@@" follows (a single hunk exceeding the cap, or the last
//   hunk), fall back to the next "\n" (the documented "line cut"). If no "\n" follows either, return
//   len(diff). Do NOT error and do NOT return a position past len(diff). The helper clamps end <= len(diff)
//   at the top so the redundant `if end > len(diff)` clamp in nextChunk can be DROPPED.

// GOTCHA (drop nextChunk's redundant clamp): nextChunk currently has `if end > len(diff) { end = len(diff) }`
//   AFTER the anchor block. anchorToHunkEdge clamps internally, so when you swap the block for the helper
//   call, REMOVE that clamp (it becomes dead code; leaving it is harmless but gofmt/vet-clean is expected).
//   chunkCount has no such clamp — just swap its block.

// GOTCHA (nextChunk uses a FIXED budget; chunkCount takes a param): nextChunk calls chunkRuneBudget()
//   (=64000) internally and has NO budget parameter — you CANNOT cheaply force @@ chunking through
//   nextChunk in a test without a >64K-rune diff. Exercise @@ anchoring via (a) anchorToHunkEdge directly
//   and (b) chunkCount(diff, smallBudget). nextChunk's anchoring is covered TRANSITIVELY (it calls the
//   same helper) + the existing TestNextChunk_SmallDiffIsOneChunk (one chunk, anchor is a no-op).

// GOTCHA (the @@ anchor is a no-op for a single-hunk diff that fits): when diff[end:] has no "\n@@" (only
//   one hunk, or end is past the last @@), the helper falls back to the newline anchor = the OLD behavior.
//   So TestNextChunk_SmallDiffIsOneChunk (small single-hunk diff) is unaffected — it still returns one
//   chunk. Do not "fix" that test; it must stay green unchanged.

// SCOPE: edit ONLY workdesc.go (helper + 2 call-sites + 2 doc comments) + generate_workdesc_test.go
//   (2 tests). Do NOT touch buildReadAnswer (BUG-002 = T2.S1) or the part-label line (BUG-006 = T4.S1).
//   Do NOT touch multiturn.go's advanceRunes. Do NOT change nextChunk/chunkCount signatures, chunkRuneBudget,
//   or readChunkTokenCap. Mode A = code comments only (no user-facing docs file).
```

## Implementation Blueprint

### Data models and structure
None. No struct/type/signature change. One new pure helper + two call-site swaps + two comment rewrites.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE the anchorToHunkEdge helper in internal/generate/workdesc.go
  - PLACE: just above nextChunk (after buildReadAnswer / chunkRuneBudget region), so the two callers read
    top-down helper→callers. (Exact spot is not load-bearing — same package.)
  - IMPLEMENT (verbatim from the research notes §4):
        // anchorToHunkEdge advances end FORWARD to the next @@ hunk boundary (a line beginning "@@") so a
        // hunk is never split mid-hunk (FR-W5). It first scans for "\n@@" from diff[end:]; if found at i it
        // returns end+i+1 (the newline before @@ ends the current chunk, so the @@ header starts the next).
        // If no @@ hunk edge follows, it falls back to the next newline (FR-W5: a single hunk exceeding the
        // cap falls back to a line cut); if neither follows, it returns len(diff). SHARED by nextChunk and
        // chunkCount so the "part i of N" total stays consistent with the actual boundaries.
        func anchorToHunkEdge(diff string, end int) int {
            if end > len(diff) {
                end = len(diff)
            }
            if i := strings.Index(diff[end:], "\n@@"); i >= 0 {
                return end + i + 1
            }
            if i := strings.IndexByte(diff[end:], '\n'); i >= 0 {
                return end + i + 1
            }
            return len(diff)
        }
  - IMPORTS: strings is already imported in workdesc.go (nextChunk uses strings.IndexByte). No new import.
  - DEPENDENCIES: none.

Task 2: MODIFY nextChunk — swap the anchor block + drop the redundant clamp + Mode-A doc comment
  - REPLACE the 5-line block:
        // Anchor FORWARD to the next newline so a line is never split mid-line.
        if i := strings.IndexByte(diff[end:], '\n'); i >= 0 {
            end += i + 1
        } else {
            end = len(diff)
        }
        if end > len(diff) {
            end = len(diff)
        }
    WITH:
        // Anchor FORWARD to the next @@ hunk edge so a change is never split mid-hunk (FR-W5); falls back
        // to a line cut when no hunk edge follows. Shared with chunkCount (anchorToHunkEdge) so the
        // "part i of N" total matches the real boundaries.
        end = anchorToHunkEdge(diff, end)
  - REWRITE the nextChunk doc comment's anchoring sentence (currently "Boundaries hug newline edges so a
    hunk is never split mid-line; …") to:
        "Boundaries hug @@ hunk edges so a change is never split mid-hunk (FR-W5); a single hunk exceeding
        the cap falls back to a line cut. The anchor is shared with chunkCount (anchorToHunkEdge) so the
        'part i of N' total stays consistent with the actual boundaries."
  - KEEP UNCHANGED: the signature, the `offset >= len(diff)` early return, `budget := chunkRuneBudget()`,
    `total = chunkCount(diff, budget)`, `end := advanceRunes(diff, offset, budget)`, and the final return.
  - DEPENDENCIES: Task 1.

Task 3: MODIFY chunkCount — swap its anchor block + Mode-A doc comment
  - REPLACE the 5-line block inside the loop:
        if i := strings.IndexByte(diff[end:], '\n'); i >= 0 {
            end += i + 1
        } else {
            end = len(diff)
        }
    WITH:
        end = anchorToHunkEdge(diff, end)
  - REWRITE the chunkCount doc comment (currently "…mirrors multiturn's window+forward-anchor discipline,
    approximated here by rune-windowing without forward-anchoring — the exact boundary is computed in
    nextChunk…") to:
        "…mirrors nextChunk's window+anchor discipline EXACTLY via the shared anchorToHunkEdge helper, so
        the label count matches the real chunk boundaries (FR-W5 'part i of N')."
  - KEEP UNCHANGED: the signature, the `runeBudget < 1` / `len(diff) == 0` guards, the loop structure, the
    `if n == 0 { n = 1 }` tail.
  - DEPENDENCIES: Task 1.

Task 4: CREATE the two regression tests in internal/generate/generate_workdesc_test.go
  - PLACE near TestNextChunk_SmallDiffIsOneChunk (@171); anchor by name not line number.
  - ADD (mirror the direct-call style; ADD "strings" to imports if the test uses strings.Index and it is
    not already imported — check the import block first):
        // TestAnchorToHunkEdge verifies the @@ hunk-edge anchor (FR-W5): prefer \n@@, fall back to \n,
        // then len(diff).
        func TestAnchorToHunkEdge(t *testing.T) {
            diff := "@@ -1,2 +1,2 @@\n-old\n+new\n@@ -10,2 +10,2 @@\n-old2\n+new2\n"
            // end inside hunk-1 body, before the hunk-2 @@ header → anchor to the \n@@ edge.
            end := strings.Index(diff, "+new\n") + len("+new")
            got := anchorToHunkEdge(diff, end)
            want := strings.Index(diff, "\n@@ -10") + 1 // just past the \n before hunk-2's @@
            if got != want {
                t.Errorf("anchorToHunkEdge hunk-edge = %d, want %d (the \\n@@ before hunk 2)", got, want)
            }
            // No @@ after end (single remaining hunk) → fall back to the next newline (FR-W5 line cut).
            end2 := strings.Index(diff, "+new2") // inside hunk-2 body; no further @@
            got2 := anchorToHunkEdge(diff, end2)
            want2 := len(diff) // no newline after "+new2\n" except the trailing one → len(diff)
            if got2 != want2 {
                t.Errorf("anchorToHunkEdge fallback = %d, want %d (len(diff))", got2, want2)
            }
        }

        // TestChunkCount_HunkBoundaries verifies chunkCount (and thus nextChunk, which shares
        // anchorToHunkEdge) chunks a multi-hunk diff on @@ edges, keeping the "part i of N" total
        // consistent with the real boundaries.
        func TestChunkCount_HunkBoundaries(t *testing.T) {
            hunk1 := "@@ -1,3 +1,3 @@\n ctx1\n-del1\n+add1\n"
            hunk2 := "@@ -20,3 +20,3 @@\n ctx2\n-del2\n+add2\n"
            diff := hunk1 + hunk2
            // A budget small enough that advanceRunes from 0 lands inside hunk-1 body (well under
            // len(hunk1) runes), so the @@ anchor — not the budget — decides the first boundary.
            budget := 8
            total := chunkCount(diff, budget)
            // With @@ anchoring, hunk-1 is one chunk and hunk-2 is one chunk ⇒ total == 2. (A newline
            // anchor would also split mid-hunk-1 and could yield a different count; the @@ guarantee is
            // that each boundary is a hunk edge.) Assert each chunk boundary is a @@ line or end-of-diff.
            if total != 2 {
                t.Errorf("chunkCount = %d, want 2 (one chunk per hunk under @@ anchoring)", total)
            }
            // Walk the chunks with the SAME anchor and assert no boundary splits a hunk.
            for off := 0; off < len(diff); {
                end := anchorToHunkEdge(diff, advanceRunes(diff, off, budget))
                if end < len(diff) {
                    // The next chunk must start at a line beginning "@@" (a hunk header).
                    if !strings.HasPrefix(diff[end:], "@@") {
                        t.Errorf("chunk boundary at %d does not start a hunk: %q", end, diff[end:end+10])
                    }
                }
                if end <= off {
                    t.Fatalf("chunk boundary did not advance at off=%d end=%d", off, end)
                }
                off = end
            }
        }
  - NOTE: the exact `total` value depends on budget vs hunk size; if your chosen budget yields a different
    count on your constructed diff, adjust the budget so hunk-1 fits in one chunk (total==2) OR relax the
    assertion to "each boundary is a @@ edge or end-of-diff" (the load-bearing property). The boundary
    property (the loop assertion) is the real regression guard; the exact count is secondary.
  - DEPENDENCIES: Tasks 1-3.

Task 5: VERIFY build + vet + format + targeted tests + full generate package
  - go build ./...
  - go vet ./internal/generate/...
  - gofmt -l internal/generate/workdesc.go internal/generate/generate_workdesc_test.go   # must list nothing
  - go test ./internal/generate/ -run 'TestAnchorToHunkEdge|TestChunkCount_HunkBoundaries|TestNextChunk' -v
  - go test ./internal/generate/ -count=1      # full package (incl. the BUG-001/BUG-002 sibling tests)
  - make test && make lint
  - Grep guards (see Validation Level 4): both call-sites use anchorToHunkEdge; no inline IndexByte anchor remains.
  - REGRESSION-CHECK (by reasoning): pre-fix, a 2-hunk diff whose budget landed mid-hunk-1 would split
    hunk-1 across two chunks (newline anchor); post-fix the boundary is the @@ edge. TestChunkCount_
    HunkBoundaries's boundary-property loop would FAIL on the pre-fix code (a mid-hunk boundary doesn't
    start with "@@").
```

### Implementation Patterns & Key Details

```go
// PATTERN: the shared anchor helper (the entire logic change — \n@@ first, \n fallback, len(diff) last)
func anchorToHunkEdge(diff string, end int) int {
	if end > len(diff) {
		end = len(diff)
	}
	if i := strings.Index(diff[end:], "\n@@"); i >= 0 {
		return end + i + 1 // newline before @@ ends the chunk; the @@ header starts the next
	}
	if i := strings.IndexByte(diff[end:], '\n'); i >= 0 {
		return end + i + 1 // FR-W5 fallback: single hunk exceeds cap → line cut
	}
	return len(diff)
}

// PATTERN: both callers collapse to one line (was a 5-line block each)
end = anchorToHunkEdge(diff, end)   // in nextChunk (drop the redundant len clamp) AND in chunkCount's loop

// PATTERN: the regression test's load-bearing assertion — every boundary is a hunk edge or end-of-diff
for off := 0; off < len(diff); {
	end := anchorToHunkEdge(diff, advanceRunes(diff, off, budget))
	if end < len(diff) && !strings.HasPrefix(diff[end:], "@@") {
		t.Errorf("chunk boundary at %d does not start a hunk", end)
	}
	off = end
}
```

### Integration Points

```yaml
NO CLI / config / API / struct / signature / const change. One helper + two call-sites + two comments + two tests.

CODE:
  - internal/generate/workdesc.go: +anchorToHunkEdge; nextChunk anchor block → helper call (drop len clamp);
    chunkCount anchor block → helper call; 2 Mode-A doc-comment rewrites
TEST:
  - internal/generate/generate_workdesc_test.go: +TestAnchorToHunkEdge + TestChunkCount_HunkBoundaries

CONSUMED (read-only, unchanged):
  - advanceRunes (multiturn.go:104, same package) — the rune-window position before the anchor
  - chunkRuneBudget (=64000) / readChunkTokenCap (=16000) — nextChunk's fixed budget

DOWNSTREAM (do NOT implement here):
  - BUG-002 (buildReadAnswer cursor-exhaustion) = T2.S1; BUG-006 (part-label byte/rune unit) = T4.S1

UNCHANGED (do NOT touch): buildReadAnswer (@362); the part-label `part := (st.offsets[p] / chunkRuneBudget()) + 1`
  (@383, BUG-006); multiturn.go advanceRunes; nextChunk/chunkCount signatures; chunkRuneBudget;
  readChunkTokenCap; any CLI/config/user-facing doc (Mode A = code comments only).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...
go vet ./internal/generate/...
gofmt -l internal/generate/workdesc.go internal/generate/generate_workdesc_test.go   # Expected: nothing listed
make lint                                                                            # Expected: zero errors
# If gofmt -l lists a file, run `gofmt -w` on it. If vet/lint errors, read + fix.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The anchor/count tests + the existing nextChunk test (must stay green)
go test ./internal/generate/ -run 'TestAnchorToHunkEdge|TestChunkCount_HunkBoundaries|TestNextChunk' -v
# Expected:
#   TestAnchorToHunkEdge              PASS  (the @@ anchor + newline fallback + len(diff))
#   TestChunkCount_HunkBoundaries     PASS  (boundaries are @@ edges; total consistent)
#   TestNextChunk_SmallDiffIsOneChunk PASS  (small single-hunk diff → one chunk; @@ anchor is a no-op)

# Full generate package (incl. the BUG-001/BUG-002 sibling tests, which must stay green)
go test ./internal/generate/ -count=1
# Expected: ALL pass.

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# nextChunk/chunkCount are pure functions over a diff string; the unit tests ARE the authoritative gate.
# (The end-to-end work-description read-loop is exercised by the existing CommitStaged stub tests, which
#  are unaffected — a small diff is one chunk regardless of the anchor.) Optional: confirm a multi-hunk
# staged file chunks on @@ edges via the real path:
#   stage a file with 2 separated hunks >64K runes total, run work-description mode, capture the stub's
#   delivered chunks, assert each "part i of N" boundary starts with "@@". (Heavy; the unit test covers it.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: BOTH callers use the shared helper (no inline newline anchor remains in either)
grep -n 'anchorToHunkEdge' internal/generate/workdesc.go
# Expected: the helper definition + 2 call-sites (nextChunk + chunkCount).

# Grep guard: the buggy inline IndexByte anchor is GONE from both functions
! grep -n 'IndexByte(diff\[end:\], .\\n.)' internal/generate/workdesc.go && echo "OK: inline newline anchor removed"
# Expected: the inline `strings.IndexByte(diff[end:], '\n')` anchor blocks are gone (the helper uses IndexByte
#           internally — that one line inside anchorToHunkEdge is fine; the guard targets the call-sites).

# Grep guard: the doc comments cite FR-W5 @@ anchoring (Mode A)
grep -n '@@ hunk edges\|hunk is never split mid-hunk\|FR-W5' internal/generate/workdesc.go
# Expected: the rewritten nextChunk + chunkCount doc comments.

# Grep guard: the two new tests exist
grep -n 'TestAnchorToHunkEdge\|TestChunkCount_HunkBoundaries' internal/generate/generate_workdesc_test.go
# Expected: both func names present.

# Scope guard: only workdesc.go + generate_workdesc_test.go changed; multiturn.go untouched
git diff --stat -- internal/generate/multiturn.go
# Expected: empty (advanceRunes is reused, not edited).

# Scope guard: buildReadAnswer (BUG-002) and the part-label (BUG-006) NOT touched
git diff internal/generate/workdesc.go | grep -E '^[+-]' | grep -E 'buildReadAnswer|part :=|offsets\[p\] / chunkRuneBudget' || echo "OK: buildReadAnswer/part-label untouched"
# Expected: "OK: …" (those lines are sibling-scope).

# Regression-property check (by reasoning): TestChunkCount_HunkBoundaries's boundary-property loop would
# FAIL on the pre-fix code (a mid-hunk newline boundary does not start with "@@"). Post-fix it PASSES.
# (Optional empirical: temporarily restore the inline IndexByte anchor in chunkCount, re-run, observe FAIL,
# then restore the helper call.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/generate/...` clean
- [ ] `gofmt -l internal/generate/workdesc.go internal/generate/generate_workdesc_test.go` empty
- [ ] `make lint` zero errors
- [ ] `go test ./internal/generate/ -count=1` passes; `make test` passes

### Feature Validation
- [ ] `anchorToHunkEdge` exists and is called by BOTH `nextChunk` and `chunkCount`
- [ ] multi-hunk diff: boundary lands at the `\n@@` edge (next hunk header)
- [ ] single-hunk/no-later-`@@`: falls back to the newline anchor (FR-W5 line cut)
- [ ] no `\n` follows: returns `len(diff)`
- [ ] `nextChunk` and `chunkCount` anchor identically (one helper) → `total` matches boundaries
- [ ] existing `TestNextChunk_SmallDiffIsOneChunk` still passes

### Scope-Boundary Validation
- [ ] `multiturn.go` (advanceRunes) byte-unchanged
- [ ] `buildReadAnswer` (BUG-002 = T2.S1) NOT touched
- [ ] the part-label `part := (st.offsets[p] / chunkRuneBudget()) + 1` (BUG-006 = T4.S1) NOT touched
- [ ] nextChunk/chunkCount signatures, chunkRuneBudget, readChunkTokenCap unchanged
- [ ] Only workdesc.go (helper + 2 call-sites + 2 doc comments) + generate_workdesc_test.go (2 tests) touched

### Code Quality
- [ ] The shared helper is the SINGLE source of the anchor (DRY — both callers delegate)
- [ ] nextChunk's redundant `if end > len(diff)` clamp dropped (helper clamps internally)
- [ ] Mode-A doc comments cite FR-W5 @@ hunk-edge anchoring + the shared-helper consistency note
- [ ] Tests cover the @@ edge, the newline fallback, and the boundary-property (no mid-hunk split)

---

## Anti-Patterns to Avoid

- ❌ **Don't fix only `nextChunk` (or only `chunkCount`).** They MUST share one anchor or the "part i of N" total drifts from the real boundaries. Extract `anchorToHunkEdge` and call it from BOTH. Leaving one on the inline newline anchor "because it looks equivalent" is the exact desync this task exists to prevent.
- ❌ **Don't scan for bare `"@@"`.** A bare `@@` matches the hunk the cursor is already inside (or a `@@` in hunk-body content). Scan for `"\n@@"` (a line beginning `@@` = a new hunk header) and advance `end += i + 1` so the `@@` line starts the NEXT chunk.
- ❌ **Don't drop the FR-W5 newline fallback.** When no `\n@@` follows (single hunk exceeding the cap, or the last hunk), you MUST fall back to the next `\n` (the documented "line cut") and then `len(diff)`. Returning `end` unchanged (no forward anchor) or erroring breaks the single-hunk-over-cap case.
- ❌ **Don't change `nextChunk`'s signature or add a budget param.** It uses the fixed `chunkRuneBudget()` (64000). To exercise @@ anchoring in tests, use the helper directly + `chunkCount(diff, smallBudget)` (chunkCount takes the param). nextChunk's anchoring is covered transitively (same helper) + the existing one-chunk test.
- ❌ **Don't touch `buildReadAnswer` or the part-label line.** BUG-002 (cursor exhaustion) and BUG-006 (byte/rune unit) are sibling subtasks on different lines. This task is ONLY the boundary anchor.
- ❌ **Don't touch `advanceRunes` (multiturn.go:104).** It is the rune-window primitive both functions already call BEFORE the anchor; reuse it as-is. Duplicating or moving it is out of scope.
- ❌ **Don't leave the stale doc comments.** `nextChunk`'s "Boundaries hug newline edges … never split mid-line" and `chunkCount`'s "without forward-anchoring" are now FALSE. Rewrite both (Mode A) to cite @@ hunk-edge anchoring (FR-W5) and the shared helper.
- ❌ **Don't break `TestNextChunk_SmallDiffIsOneChunk`.** A small single-hunk diff has no second `@@`, so the helper falls back to the newline/end anchor = the old behavior = one chunk. That test must stay green unchanged.
- ❌ **Don't assert an exact `chunkCount` total that's fragile to budget/hunk-size.** The load-bearing regression property is "every boundary is a @@ edge or end-of-diff" (the loop assertion). If your constructed diff's exact count is sensitive to the chosen budget, assert the boundary property first and treat the exact total as secondary.

---

## Confidence Score

**9/10** — one-pass success likelihood. The exact current `nextChunk`/`chunkCount` source is quoted
verbatim (with the two identical buggy inline blocks), the replacement helper is specified word-for-word,
the same-package `advanceRunes` location is confirmed (multiturn.go:104), the FR-W5 two-tier anchor
semantics are traced on a 2-hunk diff, the doc-comment sentences that become false are identified, the
existing test to mirror is named (`TestNextChunk_SmallDiffIsOneChunk` @171), and the two new tests are
written out. The −1 covers two judgment calls: (1) the exact `chunkCount` total for a chosen small budget
is sensitive to budget-vs-hunk-size (the PRP makes the boundary-property loop the load-bearing assertion
and treats the exact count as secondary); and (2) the `\n@@` pattern is robust but not provably immune to
a hunk-body line that happens to start with `@@` (the contract explicitly prescribes `\n@@`, so this is
accepted as the spec'd behavior). Both are flagged with notes/guards.