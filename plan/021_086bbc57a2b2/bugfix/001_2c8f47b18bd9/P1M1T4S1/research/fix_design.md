# BUG-006 fix research — part-label byte/rune unit consistency

## The bug (current source, verified)
`internal/generate/workdesc.go:392`, inside `buildReadAnswer`'s `else` (total>1) branch:
```go
chunk, total, advance := nextChunk(diff, st.offsets[p])   // L388
if total <= 1 {                                            // L389
    fmt.Fprintf(&b, "%s:\n%s\n\n", p, chunk)               // L390-391  (single-chunk label)
} else {
    part := (st.offsets[p] / chunkRuneBudget()) + 1        // L392  ← THE BUG
    fmt.Fprintf(&b, "%s — part %d of %d; READ %s again for the next part:\n%s\n\n",
        p, part, total, p, chunk)                          // L393-394
}
st.offsets[p] += advance                                   // L396
```
- `st.offsets[p]` is a **BYTE** offset (accumulated from `nextChunk`'s byte `advance` = `end - offset`).
- `chunkRuneBudget()` (L443) returns `readChunkTokenCap * 4` = `16000 * 4` = **64000 RUNES**.
- So the division mixes byte ÷ rune. For multibyte UTF-8, byte offset grows faster than rune offset →
  `part` over-counts (can even exceed `total`, emitting nonsense like "part 3 of 2").
- `chunkCount` (L445) already windows by RUNES (`advanceRunes`), so `total` is rune-correct. ONLY the
  `part` numerator is in the wrong unit.

## The fix (one-liner + Mode A comment + import)
```go
// part is the 1-based chunk index. st.offsets[p] is a BYTE cursor (nextChunk's advance is bytes), but
// chunkRuneBudget() is in RUNES (readChunkTokenCap*4=64000). Convert the byte offset to a rune count
// before dividing so the part label matches the rune-windowed total (FR-W5 "part i of N").
part := (utf8.RuneCountInString(diff[:st.offsets[p]]) / chunkRuneBudget()) + 1
```
+ add `"unicode/utf8"` to workdesc.go imports (NOT currently imported — verified; precedent: multiturn.go,
  multiturn_test.go, git/tokens.go all use unicode/utf8).

## SAFETY PROOF — why `diff[:st.offsets[p]]` never splits a multibyte rune / never panics
1. **In-bounds (no panic):** the T2.S1 cursor-exhaustion guard runs IMMEDIATELY before this line:
   `if st.offsets[p] >= len(diff) { … "end of diff" … continue }` (L~378-382, merged/COMPLETE). So at L392,
   `st.offsets[p] < len(diff)` is GUARANTEED → `diff[:st.offsets[p]]` is a valid in-bounds slice. (This is a
   hard dependency on T2.S1 being merged — it is.)
2. **Rune-boundary (no split):** `st.offsets[p]` is the cumulative sum of `nextChunk`'s `advance` values
   from offset 0. Each `advance = end - offset` where `end` is positioned by the FORWARD anchor:
   - CURRENT code (L406-410): `end += i+1` past a `\n` byte, or `end = len(diff)`.
   - T3.S1 code (IMPLEMENTING in parallel): `anchorToHunkEdge` returns `end+i+1` past `\n@@`, or past `\n`,
     or `len(diff)`.
   A `\n` is a single byte (0x0A) = a complete UTF-8 character; `len(diff)` is the string end. So `end` ALWAYS
   lands at the START of a character (a rune boundary). By induction from offset 0 (also a boundary),
   `st.offsets[p]` is ALWAYS a rune-boundary byte index → `diff[:st.offsets[p]]` is valid UTF-8 that never
   splits a multibyte rune → `utf8.RuneCountInString` is correct. **True under BOTH the current newline
   anchor AND T3.S1's @@ anchor** → my fix is merge-order-independent w.r.t. T3.S1.

## Sibling compatibility (same file, different lines — no conflict)
- **T1.S1 (BUG-001, COMPLETE/merged):** edits RunWorkDescription's `len(paths)==0` branch + added
  `containsReadVerb`/`readTargets`/`buildNonStagedReadAnswer`. Different function. No overlap.
- **T2.S1 (BUG-002, COMPLETE/merged):** added the `st.offsets[p] >= len(diff)` cursor-exhaustion block
  (ABOVE L392). My fix DEPENDS on it (in-bounds guarantee). Already present in current source. No conflict.
- **T3.S1 (BUG-005, IMPLEMENTING in parallel):** rewrites nextChunk/chunkCount anchoring (L398-444) +
  adds `anchorToHunkEdge`. My line (L392) is in buildReadAnswer, untouched by T3.S1. As proven above, the
  @@ anchor also lands on rune boundaries, so `diff[:st.offsets[p]]` stays safe regardless of T3.S1's state.
  → SAFE to land before, with, or after T3.S1.

## Test design — `TestBuildReadAnswer_PartLabelRuneConsistent`
**Constraint:** `chunkRuneBudget()` is FIXED at 64000 (no param) and `part` is computed inside
`buildReadAnswer`, so exercising the `total>1` branch REQUIRES a staged diff > 64000 runes. No shortcut
(T3.S1 could test `chunkCount(diff, smallBudget)` because chunkCount takes a budget param; buildReadAnswer
cannot). → real-git large-multibyte-file test (mirrors `strings.Repeat("line\n", 2000)` @L298 + the
real-git `buildReadAnswer` pattern @L439).

**Divergence requirement:** for pre-fix `part` (byte) to DIFFER from post-fix `part` (rune) after chunk 1,
the diff region must have byte/rune ratio > 2 (so byte_offset crosses 128000 while rune_offset ≈ 64000).
ASCII framing (`+`, `\n`) is 1 byte=1 rune and dilutes the ratio, so use HIGH multibyte density: lines of
SEVERAL 3-byte CJK chars. `文本文本\n` = 4 CJK + \n = 5 runes / 13 bytes (ratio 2.6); with diff `+` framing
`+文本文本\n` = 6 runes / 14 bytes (ratio 2.33 > 2 ✓).

**Steps:**
1. Temp repo; write a file with `strings.Repeat("文本文本\n", K)` for K large enough that the staged diff
   > 64000 runes (K≈14000 → ~70000 runes, ~182KB — under cfg.MaxDiffBytes default 300000, so no truncation).
2. `diff, _ := g.StagedFileDiff(ctx, path, opts)`. Guard: `if utf8.RuneCountInString(diff) <= chunkRuneBudget()
   { t.Fatalf("fixture too small / truncated") }` (catches both a too-small file AND MaxDiffBytes truncation).
3. Prime cursor to chunk-1's end: `_, total, advance := nextChunk(diff, 0); st := &readState{offsets:
   map[string]int{path: advance}}`. (advance is bytes = the real byte cursor after delivering chunk 1.)
4. `got := buildReadAnswer(ctx, g, cfg, nil, []string{path}, st)`.
5. Assert `strings.Contains(got, "— part 2 of")` — the 2nd chunk is labeled "part 2". Pre-fix (byte) yields
   "part 3 of" (ratio>2 ⇒ byte_offset ≥ 128000 ⇒ floor(…/64000)+1 = 3), so this substring is ABSENT pre-fix.
   Robust for any total≥2 (fixed always says "part 2 of N"; buggy says "part 3+ of N"). No regexp/new import
   needed (strings already imported). Add `import "unicode/utf8"` to the TEST file too (for the guard).
6. Sanity: also assert `total >= 2` (from nextChunk) so a regression that breaks chunking is caught.

**Why this fails pre-fix / passes post-fix:** after delivering chunk 1 (64000 runes), byte_offset ≈
64000×2.33 ≈ 149000 → pre-fix part = floor(149000/64000)+1 = 3. Rune count of diff[:advance] ≈ 64000 →
post-fix part = floor(64000/64000)+1 = 2. Substring "part 2 of" present post-fix only. ✓

## Mode A doc surface
Code comment on the part line only (above). No user-facing doc change (work-desc READ protocol is an
internal transport detail; README/how-it-works already describe "part i of N" generically). The
changeset-level doc sweep (P1.M1.T5.S1) handles any user-facing review separately.