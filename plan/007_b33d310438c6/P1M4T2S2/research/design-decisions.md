# P1.M4.T2.S2 — Design decisions (per-file truncation application + sentinel + header preservation)

> Pure, git-independent truncation application for the FR3i water-fill (PRD §9.1 FR3i; architecture/
> git_diff_semantics.md §6; system_context.md §6 invariant 2). Consumes S1's `allocByWaterFill`
> allotments; consumed by the M4.T3 token-limit gate. This file is the reasoning behind the PRP — the
> frozen contracts live in PRP.md.

## D1 — Scope boundary: S2 is PURE (mirrors S1's discipline)

S2 delivers THREE pure, git-independent functions in `internal/git` (stdlib `strings` + `unicode/utf8`
+ `regexp` only; no context/I/O/git):

1. `splitDiffSections(diff string) []string` — split the captured non-markdown aggregate on
   `diff --git ` boundaries into per-file sections (the item_description's "split on `diff --git `
   boundaries … WITHOUT extra git invocations"). Pure string manipulation.
2. `diffSectionPath(section string) (path string, ok bool)` — extract the DESTINATION (b/) path from a
   section's `diff --git a/<p> b/<p>` line, falling back to `+++ b/<p>`. Pure.
3. `truncateByWaterFill(sections []string, allotments map[string]int) string` — the item's named output
   function. For each section: extract its path → look up its token allotment → split the section into a
   HEADER block + BODY → if the body exceeds the allotment (in tokens), truncate the body to the
   allotment (allotment×4 runes) and append `\n... [truncated]` → else pass through byte-identical.
   Recompose the sections in ORIGINAL ORDER. Pure.

The GATE (M4.T3, a SEPARATE task — do NOT implement) wires S2 into the 3 diff functions: capture the
non-md aggregate (already done, FR3h-index-stripped), `splitDiffSections` it, build the allotments map
(path→token allotment, from `allocByWaterFill` over `EstimateTokens(body)` sizes + the numstatRows path
keying), call `truncateByWaterFill`, and substitute the result for the byte-cap. S2 owns NONE of that.

**Why pure (and thus exhaustively table-testable like S1 / resolveNumstatPath):** the truncation logic is
string arithmetic over already-captured diff text. There is no git repo, no exec, no context. Every edge
case (multi-hunk, rename, new-file, deletion, pure-rename-no-body, markdown per-file section, path-miss,
empty) is constructible as a string literal and assertable in microseconds. Bugs are caught here, not in a
7-stage integration run.

## D2 — `truncateByWaterFill` signature: path-keyed allotments (robust to ordering)

Signature (FROZEN seam — M4.T3 calls it): `func truncateByWaterFill(sections []string, allotments map[string]int) string`.

- `sections` = the per-file diff sections (`splitDiffSections(nmDiff)` for non-markdown; the list of
  per-file markdown diffs for markdown — uniform, the same function serves both, per item_description
  "Handle the markdown per-file section similarly").
- `allotments` = a `map[string]int` (numstat destination-path → token allotment). Built by the GATE from
  `allocByWaterFill` output + numstatRows (S1 path keying).

**Why path-keyed (not index-parallel to a `sizes` slice)?** The diff sections come out in **git's
emission order**; the numstatRows that key `sizes`/allotments come out **sorted by path**
(`sort.Slice(rows, …rows[i].Path < rows[j].Path…)` in numstat.go). These orderings are NOT guaranteed to
agree (renames, binary rows, git's internal ordering). Index-parallel mapping would scramble under any
ordering drift. **Path-matching is the robust, contract-faithful choice** — the item_description mandates
it: "Map allotments back to files by matching the section's path … to the numstat destination-path key
(S1 path keying must agree here — same `=>` destination resolution)." `diffSectionPath` produces the same
destination string `resolveNumstatPath` does (both yield the NEW path), so the keys match.

**Path-miss policy:** if `diffSectionPath` returns `ok=false` (no `diff --git`/`+++ b/` line) OR the path
is absent from `allotments`, the section is passed through UNTOUCHED (no truncation). Rationale: it is
safer to over-include an unidentified section than to wrongly truncate or drop it. Binary placeholders
(`<status>\t[binary] <path>`) are NOT `diff --git` sections (they're one-line placeholders emitted
separately in git.go), so they never reach this function.

## D3 — Header block vs body: the split that preserves file identity

A captured, FR3h-index-stripped section looks like (verified against stagediff_test.go L605–622):

```
diff --git a/a.go b/a.go          ┐
--- a/a.go                         ├─ HEADER BLOCK (always preserved)
+++ b/a.go                        ┘
@@ -1 +1 @@                       ┐
-old                              │  BODY (truncated to allotment when over budget)
+new                              ┘
```

- **HEADER BLOCK** = every leading line that is NOT a hunk: `diff --git a/<p> b/<p>`, the extended
  headers (`new file mode`, `deleted file mode`, `old mode`, `new mode`, `similarity index`,
  `dissimilarity index`, `rename from`, `rename to`, `copy from`, `copy to`), `--- a/<p>`, `+++ b/<p>`.
  Concretely: all lines BEFORE the first `@@` line. These carry FILE IDENTITY and are cheap — always kept.
- **BODY** = everything from the first `@@` line onward (the hunks: each `@@ -l,s +l,s @@` header + its
  context/`+`/`-` content lines).

**Truncation = take the first `allotment` tokens of the BODY** (the FR3i "its first L tokens + sentinel").
The body stream INCLUDES the `@@` hunk headers; truncating the body at the allotment naturally preserves
every `@@` that falls within the first L tokens and cuts off everything after. The first `@@` is at the
very start of the body, so (for any allotment > 0) it is always retained — satisfying the
"headers preserved on the truncated file" test (`diff --git`/`---`/`+++`/`@@` all present after
truncation). A truncated body that ends mid-hunk or mid-content-line is a VALID, faithful "first L tokens"
representation; the `... [truncated]` sentinel signals incompleteness.

**Edge cases of the split (all pass-through — no body to truncate):**
- **Pure rename / mode-only change:** no `@@` line → the whole section is header block, body = "" → no
  truncation. (Pure renames are tiny anyway — `similarity index` + `rename from`/`rename to`, ~3 lines.)
- **Empty section** (`""`): header block = "", body = "" → no-op.

**Why "before the first `@@" (not "all `@@` lines are headers"):** preserving ALL `@@` lines while
truncating only content would produce malformed hunks (a `@@` header with no following content). The
"first-L-tokens-of-the-body" stream is the only truncation that yields a well-formed (if partial) diff
and matches FR3i's "its first L tokens" verbatim. The item_description's mention of `@@` among "the
headers" enumerates the structural line TYPES that exist in a diff (cf. FR3h's keep/strip table); it does
NOT mean every `@@` across all hunks survives an L-token body cutoff.

## D4 — The sentinel: the SHORTER `... [truncated]` form (system_context §6 invariant 2)

The truncation sentinel is the literal `... [truncated]` — the SHORTER form mandated by
system_context.md §6 invariant 2 and PRD FR3i:

> "`token_limit > 0` ⇒ water-fill replaces the byte/line caps. The `... [truncated]` sentinel (shorter
> form, per PRD FR3i) is emitted per truncated file; the `at N bytes` sentinels do NOT appear."

This is DISTINCT from the LEGACY per-section sentinels (`... [diff truncated at N bytes]` /
`... [diff truncated at N lines]` at git.go L840/L868) which remain ONLY on the `token_limit==0` path
(M4.T3 keeps the byte/line caps byte-identical there). S2 NEVER emits the legacy forms — it is the
`token_limit>0` path exclusively. Confirmed: the item_description pins "the SHORTER `... [truncated]`
form (NOT the legacy `... [diff truncated at N bytes]` — system_context.md §6 invariant 2)".

**Append rules:**
- The sentinel is appended on its OWN line: a leading `\n` then `... [truncated]` (matches the legacy
  sentinels' `\n... [diff truncated at N bytes]` line shape — the model sees a clean standalone marker).
- Appended ONLY when content was actually removed (the body strictly exceeded the allotment). A section
  within its allotment is returned byte-identical — NO sentinel (the item_description: "Files within their
  allotment pass through untouched" / "non-truncated files byte-identical to input").

## D5 — Token → rune conversion (consistent with EstimateTokens = ceil(runes/4))

The allotment is in TOKENS; the body is a STRING. To truncate a body to `allotment` tokens, convert to a
RUNE budget and slice:

- `EstimateTokens(s) = ceil(runeCount(s) / 4)` (tokens.go). The inverse: `allotment` tokens ⟺
  `allotment × 4` runes (proof: `ceil(r/4) ≤ n ⟺ r ≤ 4n`, so the largest prefix with `EstimateTokens ≤
  allotment` has exactly `allotment×4` runes; `EstimateTokens(firstNRunes(body, allotment×4)) = allotment`).
- **"Needs truncation?"** = `EstimateTokens(body) > allotment` ⟺ `utf8.RuneCountInString(body) >
  allotment×4`. (When sizes are derived via `EstimateTokens(body)` upstream — S1's specified unit — this
  is EXACTLY the water-fill's "file larger than L" condition, so coherence holds: a file the water-fill
  kept whole has `allotment == size == EstimateTokens(body)` ⇒ no truncation here; a file the water-fill
  capped has `allotment = L < EstimateTokens(body)` ⇒ truncated here.)
- **Truncation:** slice the body to its first `allotment×4` RUNES (NOT bytes — slicing bytes could split a
  multi-byte UTF-8 char). Use an efficient byte-offset iteration (decode runes via `for i := range s`,
  stop at the Nth rune's start) — do NOT allocate a full `[]rune` (a diff body can be large).

```go
// firstNRunes returns s's first n runes (rune-boundary-safe; no full []rune allocation).
func firstNRunes(s string, n int) string {
    if n <= 0 { return "" }
    count := 0
    for i := range s {          // i = byte offset of each rune start
        if count == n { return s[:i] }
        count++
    }
    return s // fewer than n runes
}
```

## D6 — Path extraction (`diffSectionPath`)

`diffSectionPath(section) (path string, ok bool)` recovers the DESTINATION path (b/) so the section can be
matched against the `allotments` map (keyed by numstat destination path). Order of preference:

1. **`diff --git a/<p> b/<p>`** → take the part after the LAST ` b/` token, strip the `b/` prefix.
   (Destination is the SECOND path. For a rename this is the NEW path — matches `resolveNumstatPath`'s
   destination resolution. For a deletion `diff --git a/old.go b/old.go` → `old.go` — correct, numstat
   keys deletions by the deleted path.) Implemented with a regex: `^diff --git a/(.*) b/(.*)$` → group 2.
2. **Fallback `+++ b/<p>`** → strip `b/`. (Used only if the `diff --git` line is absent/malformed. For a
   deletion `+++ /dev/null` is NOT a destination — but the `diff --git` line already supplied it, so the
   fallback is rarely reached.)
3. `ok = false` if neither line is found → caller passes the section through untouched (D2 path-miss).

**Quoting gotcha (paths with special chars):** git quotes paths containing bytes > 0x7f or `"`, `\`,
control chars (and, in some versions, spaces) in `diff --git`/`+++` lines as C-style `\"`/`\\`/`\nnn`
inside surrounding `"`. numstat is TAB-separated, so its paths are UNQUOTED. A quoted diff path would
mismatch the unquoted numstat key → path-miss → pass-through (the file would NOT be truncated — a
correctness gap, but only for paths with special chars, rare in typical codebases). **Mitigation for v1:**
strip a SINGLE pair of surrounding `"` from the extracted path (handles the common quoted-space case
without a full C-string unquoter). Full git `core.quotePath` unquoting is OUT OF SCOPE — note as a known
limitation; the binary/[excluded] placeholders and numstat already handle the common paths, and the
water-fill's completeness floor (the skeleton, FR3g) means a missed truncation degrades gracefully (the
file is over-included, not dropped).

## D7 — `splitDiffSections`: the `diff --git ` boundary

The item_description pins the boundary: "split on `diff --git ` boundaries to apply the per-file level
WITHOUT extra git invocations." So:

- Split on the literal `diff --git ` (note the trailing space — distinguishes `diff --git a/foo` from a
  content line that happens to start with `diff`). `strings.Split(diff, "diff --git ")` yields a leading
  empty/whitespace element (the text before the first `diff --git`) + one element per section (each
  MISSING its `diff --git ` prefix, which was the separator). RE-PREFIX each section with `diff --git `
  so the section is self-contained (its first line is `diff --git a/<p> b/<p>` — what `diffSectionPath`
  and the header-block parser expect). Drop a truly-empty leading element; keep a non-empty leading
  element as its own leading "section" (defensive — should not occur for a clean non-md aggregate, which
  always starts with `diff --git`, but a stray placeholder/comment would be preserved rather than lost).
- An empty/whitespace input → `[]` (no sections) → `truncateByWaterFill` returns "" (the nothing-staged
  shape; the caller's FR5 check is upstream).

## D8 — Recomposition order + the byte-identical pass-through guarantee

`truncateByWaterFill` joins the (possibly truncated) sections in their ORIGINAL input order — NOT sorted.
This is the FR3i contract: "Recompose the sections in original order." Sorting would scramble the
narrative (the model reads diffs in git's emission order). The item's regression test "non-truncated
files byte-identical to input" is satisfied because: a section within its allotment is returned VERBATIM
(unchanged bytes), and the join order matches the input order — so the only bytes that differ from the
input are the truncated bodies + their sentinels.

## D9 — Test plan (pure, table-driven — mirror numstat_test.go + the S1 waterfill style)

ALL tests are PURE (no `t.TempDir`, no git repo, no I/O) — the functions are string arithmetic. Mirror
`numstat_test.go`'s `TestResolveNumstatPath` table shape (`tests := []struct{…}{…}` with HARDCODED `want`,
run via `t.Run(tc.desc, …)`).

**`TestSplitDiffSections`:** empty input → `[]`; single section → `[section]`; 3-file aggregate → 3
sections each starting with `diff --git `; leading non-`diff --git` text preserved as a leading section.

**`TestDiffSectionPath`:** normal edit (`a/a.go b/a.go` → `a.go`); new file (`new file mode`, `diff --git
a/x b/x` → `x`); deletion (`+++ /dev/null`, `diff --git a/x b/x` → `x`); rename (`a/old b/new` → `new`);
fallback to `+++ b/x` when `diff --git` line is absent; `ok=false` for a non-diff string (e.g. a binary
placeholder line); basic quote-strip (`"b/foo bar"` → `foo bar`).

**`TestTruncateByWaterFill` (the item's named test):**
- **3-file diff, one exceeds L:** construct a 3-section aggregate where file B's body is large; allotments
  `{A: <whole>, B: <small>, C: <whole>}`; assert B is truncated (body cut + `... [truncated]` sentinel
  on its own line), A and C are BYTE-IDENTICAL to input (assert `strings.Contains(out, sectionA)` /
  `sectionC` verbatim, and the sentinel appears exactly once — only on B).
- **Headers preserved on the truncated file:** after truncating B, `diff --git a/B b/B`, `--- a/B`,
  `+++ b/B`, and the first `@@` are all present in B's section.
- **Sentinel present + on its own line:** the truncated section ends with `\n... [truncated]`.
- **Multi-hunk file truncated:** a section with 2 hunks, allotment < both-hunks size → body cut mid-way,
  sentinel appended, the first `@@` retained, the second `@@` (after the cutoff) dropped.
- **Markdown per-file section:** a single markdown file's diff as a 1-element `sections` slice, allotment
  small → truncated identically (same code path).
- **All files within budget:** allotments ≥ each body size → output byte-identical to input (NO sentinels).
- **Path-miss pass-through:** a section whose path is absent from `allotments` → returned verbatim.
- **Pure-rename section (no `@@`):** no body → returned verbatim regardless of allotment.
- **Empty sections / empty allotments:** `[]` → `""`; `{}` allotments → all sections pass through.

## D10 — Frozen / out-of-scope (siblings own these — do NOT touch)

- `internal/git/waterfill.go` (S1, parallel — `waterFillLevel`/`allocByWaterFill`): the SOLVER; S2 calls
  neither directly (the GATE does). READ-ONLY contract.
- `internal/git/git.go` (the 3 diff functions, `StagedDiffOptions`, `buildDiffArgs`, `stripIndexLines`):
  M4.T3's territory (the gate). S2 ADDS files only; git.go is UNCHANGED.
- `internal/git/{numstat,skeleton,tokens,binary}.go`: siblings' frozen territory. S2 imports `EstimateTokens`
  from tokens.go (same package — no import statement) for the "needs truncation?" check; it does NOT touch
  the file.
- The gate wiring (body_budget = token_limit − skeleton − reserve − margin; the token_limit==0 vs >0
  branch; building the allotments map): M4.T3. S2 provides the primitives; M4.T3 assembles.
- `go.mod`/`go.sum`: UNCHANGED (stdlib only).
