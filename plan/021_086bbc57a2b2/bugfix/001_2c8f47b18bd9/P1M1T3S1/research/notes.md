# P1.M1.T3.S1 — Research notes (BUG-005: anchor chunk boundaries to @@ hunk edges)

## 0. Task shape

Surgical bug-fix in `internal/generate/workdesc.go`: `nextChunk` (~398) and `chunkCount` (~423) both
forward-anchor chunk boundaries to a NEWLINE (`strings.IndexByte(diff[end:], '\n')`), which can split a
git diff hunk mid-hunk. Per FR-W5 the boundary must hug `@@` hunk edges. Fix: extract a shared helper
`anchorToHunkEdge(diff, end) int` (scan for `\n@@`; fall back to `\n`; then `len(diff)`) and call it from
BOTH functions so the "part i of N" total stays consistent. + Mode-A doc-comment update. + regression tests.

## 1. The buggy code (internal/generate/workdesc.go) — EXACT current

**nextChunk (398-415):**
```go
func nextChunk(diff string, offset int) (chunk string, total int, advance int) {
	if offset >= len(diff) {
		return "", 1, 0 // cursor exhausted (FR-W5 end-of-diff); the caller notes it
	}
	budget := chunkRuneBudget()
	total = chunkCount(diff, budget)
	end := advanceRunes(diff, offset, budget)
	// Anchor FORWARD to the next newline so a line is never split mid-line.   ← BUG-005
	if i := strings.IndexByte(diff[end:], '\n'); i >= 0 {
		end += i + 1
	} else {
		end = len(diff)
	}
	if end > len(diff) {
		end = len(diff)
	}
	return diff[offset:end], total, end - offset
}
```
**chunkCount (423-444):**
```go
func chunkCount(diff string, runeBudget int) int {
	if runeBudget < 1 { runeBudget = 1 }
	if len(diff) == 0 { return 1 }
	n := 0
	for offset := 0; offset < len(diff); {
		end := advanceRunes(diff, offset, runeBudget)
		if i := strings.IndexByte(diff[end:], '\n'); i >= 0 {   // ← BUG-005 (must match nextChunk)
			end += i + 1
		} else {
			end = len(diff)
		}
		n++
		offset = end
	}
	if n == 0 { n = 1 }
	return n
}
```
- The bug: `end` (from `advanceRunes`) is the rune-budget position; forward-anchoring to the next `\n`
  can land INSIDE a hunk's body, splitting that hunk across two chunks. FR-W5 wants the boundary at the
  START of the NEXT hunk (a line beginning `@@`).
- **CRITICAL consistency**: nextChunk computes `total = chunkCount(...)` and then independently anchors
  `end`. If the two anchor differently, the "part i of N" label (computed in buildReadAnswer @383 as
  `part := (st.offsets[p] / chunkRuneBudget()) + 1`) drifts from the real boundary. ⇒ the anchor MUST
  live in ONE helper called by both.

## 2. Same-package primitives (DO NOT change signatures)

- `advanceRunes(s string, start, n int) int` — **internal/generate/multiturn.go:104** (same package
  `generate`; no import needed). Returns the byte index `end` reached by scanning `n` runes from `start`.
  Used by BOTH nextChunk and chunkCount to get the initial budget position.
- `chunkRuneBudget() int` — workdesc.go:419; returns `readChunkTokenCap * 4` = `16000 * 4` = **64000**
  (fixed; nextChunk uses this internally — it has NO budget param). `chunkCount` takes an explicit
  `runeBudget` param (testable with a small budget).
- `readChunkTokenCap = 16000` — workdesc.go:35 (const).

## 3. FR-W5 (the spec the anchor must satisfy)

> "Chunk boundaries hug `@@` hunk edges so a change is never split mid-hunk; a single hunk exceeding the
> cap falls back to a line cut (noted in the label)." (spec/01-product.md:526, cited in the contract)

⇒ Two-tier anchor: (1) prefer the next `\n@@` (a line starting `@@` = a new hunk header); (2) if none
follows within the remaining diff, fall back to the next `\n` (the "single hunk exceeds cap → line cut"
case); (3) if neither, `len(diff)`. A git diff hunk header is `@@ -start,count +start,c @@`; the pattern
`\n@@` reliably marks hunk starts (the contract: "`@@` does not appear mid-line in diff content" — robust
enough for the polish bug; context/+/- lines begin with space/`+`/`-`).

## 4. The fix — shared helper + both callers

```go
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
		return end + i + 1 // include the newline before @@ → the new hunk starts the next chunk
	}
	if i := strings.IndexByte(diff[end:], '\n'); i >= 0 {
		return end + i + 1 // FR-W5 fallback: line cut (no @@ hunk edge within the remaining diff)
	}
	return len(diff)
}
```
Then in **nextChunk** replace the 5-line `if i := strings.IndexByte(...)` block with `end = anchorToHunkEdge(diff, end)`
(and drop the now-redundant `if end > len(diff)` clamp — the helper clamps). In **chunkCount** replace
the 5-line `if i := strings.IndexByte(...)` block with `end = anchorToHunkEdge(diff, end)`.

**Verified semantics** (trace on a 2-hunk diff, budget lands in hunk-1 body):
- `\n@@` (hunk-2 header) is found after `end` → `end` jumps to just past the `\n` before hunk 2 → chunk 1
  = hunk 1 whole; chunk 2 starts at hunk 2's `@@`. ✅ hunk preserved.
- Single hunk, `end` past the only `@@` → no `\n@@` found → newline fallback → line cut. ✅ FR-W5 fallback.
- No `\n` at all (diff ends mid-line) → `len(diff)`. ✅

## 5. Doc-comment updates (Mode A — both functions reference anchoring)

- **nextChunk doc (392-397)**: the line "Boundaries hug newline edges so a hunk is never split mid-line"
  is now FALSE. Rewrite to: "Boundaries hug `@@` hunk edges so a change is never split mid-hunk (FR-W5);
  a single hunk exceeding the cap falls back to a line cut. The anchor is shared with chunkCount
  (anchorToHunkEdge) so the 'part i of N' total stays consistent with the actual boundaries."
- **chunkCount doc (420-422)**: it claims "approximated here by rune-windowing WITHOUT forward-
  anchoring — the exact boundary is computed in nextChunk". That is now stale (chunkCount forward-anchors
  via the SAME helper). Rewrite to: "…mirrors nextChunk's window+anchor discipline EXACTLY via the shared
  anchorToHunkEdge helper, so the label count matches the real chunk boundaries (FR-W5 'part i of N')."

## 6. Test patterns (internal/generate/generate_workdesc_test.go)

- Package `generate`, WHITE-BOX (unexported `nextChunk`/`chunkCount`/`anchorToHunkEdge` reachable).
- Existing `TestNextChunk_SmallDiffIsOneChunk` (@171) calls `nextChunk(diff, 0)` directly with a synthetic
  string diff + asserts `chunk == diff` (one chunk) and the cursor-exhausted case (`nextChunk(diff, len(diff))`
  → `("",_,0)`). **Must still pass** (a small single-hunk diff is one chunk; the @@ anchor is a no-op when
  no second hunk follows — falls back to newline/end, same result).
- The BUG-001 sibling (@135-205) and BUG-002 sibling (T2.S1) ALSO add tests here (different names,
  additive — no conflict). Anchor new tests by NAME, not line number (lines shift).
- **New tests** (nextChunk uses the fixed 64000 budget internally, so to exercise @@ anchoring cheaply
  test the helper directly + chunkCount with a small explicit budget — chunkCount takes `runeBudget`):
  1. `TestAnchorToHunkEdge` — direct: `\n@@` after end → hunk edge; no `@@` → newline fallback; neither → len(diff).
  2. `TestChunkCount_HunkBoundaries` — a 2-hunk synthetic diff + a small `runeBudget` that lands mid-hunk-1;
     assert `chunkCount` returns 2 and that walking the chunks with `anchorToHunkEdge` lands each boundary
     at a `@@` line or end-of-diff (proves nextChunk/chunkCount consistency, since both call the helper).

## 7. Scope fences (confirmed vs siblings — all same file workdesc.go)

- **T2.S1 (BUG-002)** edits `buildReadAnswer` (@362, the cursor-exhaustion `st.offsets[p] >= len(diff)`
  branch) — DIFFERENT function; explicitly scopes OUT nextChunk/chunkCount/chunkRuneBudget. No conflict.
- **T1.S1 (BUG-001)** adds `containsReadVerb` helper + fixes the `len(paths)==0` branch in
  `RunWorkDescription` — different function; additive helpers/tests; no conflict with the anchor edit.
- **T4.S1 (BUG-006)** will fix the `part := (st.offsets[p] / chunkRuneBudget()) + 1` byte/rune mismatch
  in `buildReadAnswer` (@383) — DIFFERENT line/function; Planned (not parallel). My task does NOT touch
  the part-label computation. (My doc note says the total "stays consistent" — that's about the COUNT
  matching boundaries, not the byte/rune unit, which is T4's separate concern.)
- This task edits ONLY: `anchorToHunkEdge` (new) + the two anchor call-sites + two doc comments in
  workdesc.go; + 2 tests in generate_workdesc_test.go. NOT buildReadAnswer, NOT the part-label, NOT
  multiturn.go's advanceRunes, NOT any CLI/config/user-facing doc (Mode A = code comments only).