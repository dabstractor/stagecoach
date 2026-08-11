name: "P1.M1.T4.S1 — Fix 'part i of N' label to divide a RUNE offset by the rune budget (BUG-006)"
description: >
  Fix BUG-006 (Minor): internal/generate/workdesc.go:392 (buildReadAnswer's total>1 branch) computes
  `part := (st.offsets[p] / chunkRuneBudget()) + 1`, but st.offsets[p] is a BYTE cursor (advanced by
  nextChunk's byte `advance`) while chunkRuneBudget() returns readChunkTokenCap*4 = 64000 RUNES. The
  byte/rune unit mismatch makes the "part i of N" label over-count for multibyte UTF-8 (can even exceed
  N — "part 3 of 2"). chunkCount already windows by runes (advanceRunes) so `total` is correct; ONLY the
  `part` numerator is in the wrong unit. Fix: convert the byte offset to a rune count before dividing —
  `part := (utf8.RuneCountInString(diff[:st.offsets[p]]) / chunkRuneBudget()) + 1` — plus a Mode-A code
  comment and the `"unicode/utf8"` import (not currently imported). Safe: the T2.S1 cursor-exhaustion guard
  (merged) guarantees st.offsets[p] < len(diff) at this line, and st.offsets[p] is always a rune boundary
  (nextChunk anchors to \n / @@ / len(diff), all byte-aligned) so the slice never splits a multibyte rune.
  + 1 regression test (TestBuildReadAnswer_PartLabelRuneConsistent) over a >64K-rune multibyte staged diff.
  Only workdesc.go + generate_workdesc_test.go touched. Compatible with T3.S1 (parallel) — both anchors
  land on rune boundaries. No signature/const/budget/CLI/config change.

---

## Goal

**Feature Goal**: Make the work-description mode's "part i of N" chunk label unit-consistent — divide a
RUNE offset by the rune budget (64000) instead of a BYTE offset — so a multibyte UTF-8 diff is labeled
correctly (part 2 of 2, not part 3 of 2). The `total` (N) is already rune-correct via `chunkCount`'s
`advanceRunes` windowing; only the `part` (i) numerator was in bytes.

**Deliverable**: (1) one-line change at `internal/generate/workdesc.go:392` — wrap `st.offsets[p]` in
`utf8.RuneCountInString(diff[:st.offsets[p]])` before dividing by `chunkRuneBudget()`; (2) a Mode-A code
comment explaining the byte→rune conversion; (3) add `"unicode/utf8"` to the workdesc.go imports; (4) one
regression test `TestBuildReadAnswer_PartLabelRuneConsistent` in `internal/generate/generate_workdesc_test.go`
that stages a >64K-rune multibyte file, primes the cursor to chunk-1's end, and asserts the 2nd chunk is
labeled "part 2".

**Success Definition**:
- A multibyte UTF-8 diff that spans 2 chunks is labeled "part 1 of 2" … "part 2 of 2" (not "part 1 of 2"
  … "part 3 of 2").
- The new regression test FAILS on the pre-fix byte-offset code (emits "part 3 of") and PASSES post-fix
  (emits "part 2 of").
- ASCII-only diffs are UNAFFECTED (byte offset == rune offset; part unchanged) — existing
  `TestBuildReadAnswer_EndOfDiff` and the happy-path/round-budget tests stay green.
- `go build ./...`, `go vet ./internal/generate/...`, `go test ./internal/generate/ -count=1`, `make test`,
  `make lint` all pass; gofmt clean.
- No panic on multibyte input: `diff[:st.offsets[p]]` never splits a rune (proven — see Context).

## User Persona (if applicable)

**Target User**: A developer whose staged file contains multibyte UTF-8 (CJK, emoji, accented Latin) large
enough to span >1 work-description read chunk, and indirectly the agent consuming the chunked diff.

**Use Case**: In work-description mode (§9.26), the model READs a large staged file; stagecoach returns the
diff in ≤16K-token (64K-rune) chunks, each labeled "part i of N". With the fix, i is correct for multibyte
content, so the model knows how many parts remain and which part it's reading.

**User Journey**: `stagecoach` (work-description mode) → model READs big-UTF-8-file → stagecoach emits
"part 1 of 2" then (on re-READ) "part 2 of 2" → model assembles the full diff → coherent commit message.

**Pain Points Addressed**: BUG-006 — the part label over-counted for multibyte UTF-8 (byte/rune mismatch),
emitting nonsensical "part 3 of 2" labels that confused the model about progress through the diff.

## Why

- **BUG-006 (Minor) / FR-W5**: the "part i of N" label is a user/agent-facing signal of read progress.
  `chunkRuneBudget()` is in RUNES (it realizes `readChunkTokenCap` tokens as `*4` runes, matching
  `git.EstimateTokens`'s ceil(runes/4) and multiturn.go's chunk sizing). `chunkCount` honors that (it
  windows via `advanceRunes`). But the `part` numerator used the raw BYTE cursor `st.offsets[p]`, so for
  multibyte UTF-8 the two sides of "i of N" were in different units — i could exceed N.
- **Root cause (context, no action beyond the fix)**: `nextChunk` returns a byte `advance` (it slices
  `diff[offset:end]`), and `st.offsets[p] += advance` keeps the cursor in BYTES (correct for slicing).
  The bug is purely in the one division that compared that byte cursor to a rune budget.
- **Bounded, surgical**: one numerator wrapped in `utf8.RuneCountInString` + one import + one comment +
  one test. No struct/signature/budget change. Sibling bugs (BUG-001/002/005) are different branches/
  functions/lines.

## What

**User-visible behavior**: marginally more correct "part i of N" labels in work-description mode for
multibyte UTF-8 diffs (a quality polish; the architecture analysis groups it under "chunk/label quality").
No CLI/config/API change. ASCII-only diffs are unchanged (byte offset == rune offset).

**Technical change (one line + import + comment + test):**
1. `workdesc.go` L392: replace `part := (st.offsets[p] / chunkRuneBudget()) + 1` with
   `part := (utf8.RuneCountInString(diff[:st.offsets[p]]) / chunkRuneBudget()) + 1` + a Mode-A comment.
2. `workdesc.go` imports: add `"unicode/utf8"`.
3. `generate_workdesc_test.go`: add `TestBuildReadAnswer_PartLabelRuneConsistent` (+ `"unicode/utf8"` to
   the test file's imports for the rune-count guard).

### Success Criteria
- [ ] L392 uses `utf8.RuneCountInString(diff[:st.offsets[p]])` as the numerator (not the raw byte offset)
- [ ] Mode-A comment explains the byte→rune conversion (why the slice is safe)
- [ ] `"unicode/utf8"` added to workdesc.go imports (and to the test file imports)
- [ ] New test stages a >64K-rune multibyte file, primes cursor to chunk-1 end, asserts "part 2 of"
- [ ] New test FAILS pre-fix ("part 3 of") and PASSES post-fix ("part 2 of")
- [ ] `TestBuildReadAnswer_EndOfDiff` + existing work-desc tests stay green (ASCII unaffected)
- [ ] `go build ./...`, `go vet ./internal/generate/...`, `go test ./internal/generate/ -count=1`, `make test`,
      `make lint` pass; gofmt clean

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact buggy line (verbatim, L392), the exact replacement (verbatim), the import to add (and
that it is NOT currently imported), the proof that the slice is safe (in-bounds via the T2.S1 guard +
rune-boundary via the anchor), the test pattern to mirror (`TestBuildReadAnswer_EndOfDiff` @439, real
`git.New(repo)` + shared helpers in generate_test.go), the divergence math (why pre-fix yields "part 3" and
post-fix yields "part 2"), and the scope fences against the sibling bugs are all enumerated below.

### Documentation & References

```yaml
# MUST READ — the file under the fix
- file: internal/generate/workdesc.go
  why: "THE change site. buildReadAnswer @~372 (cursor-exhaustion guard @~378-382 [T2.S1, merged]; the
        buggy part line @392; st.offsets[p] += advance @396). chunkRuneBudget @443 (=64000). nextChunk
        @~425 (returns byte advance; anchors end to \\n @~433 or len(diff)). readChunkTokenCap @36 (=16000)."
  pattern: "the buggy line (L392, inside `else` after `if total <= 1`):
            `part := (st.offsets[p] / chunkRuneBudget()) + 1`"
  critical: "imports (@18-27) are context/fmt/strings + config/git/provider — unicode/utf8 is NOT present;
             ADD it (precedent: multiturn.go, multiturn_test.go, git/tokens.go all import unicode/utf8).
             The REPLACEMENT must slice diff[:st.offsets[p]] — see the safety proof (it is safe)."

# MUST READ — the test file to extend (mirror its patterns verbatim)
- file: internal/generate/generate_workdesc_test.go
  why: "TestBuildReadAnswer_EndOfDiff @439 is the EXACT pattern: real git.New(repo) + initRepo/commitRaw/
        writeFile/stageFile (defined in generate_test.go @26/55/34/43, same package) + a primed
        readState{offsets: map[string]int{...}} + direct buildReadAnswer call. strings.Repeat for large
        content is used @298. TestNextChunk_SmallDiffIsOneChunk @171 shows nextChunk's return signature."
  pattern: >
    repo := t.TempDir(); initRepo(t, repo); commitRaw(t, repo, "initial")
    writeFile(t, repo, path, body); stageFile(t, repo, path)
    g := git.New(repo); cfg := config.Defaults()
    st := &readState{N: 5, offsets: map[string]int{path: <primed>}}
    got := buildReadAnswer(ctx, g, cfg, nil, []string{path}, st)
  critical: "imports (@15-25) include context/errors/strings/testing/time + config/git/prompt/stubtest.
             ADD \"unicode/utf8\" (for the rune-count guard). Do NOT add regexp — assert via strings.Contains."

# DECISION PROVENANCE
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/architecture/bugfix_workdesc.md
  why: "§BUG-006 is the authoritative analysis: the byte/rune root cause, the exact buggy line, and the two
        fix options (Option 2 = the inline RuneCountInString conversion is chosen here — simplest, no struct
        change). This task implements Option 2."
  section: "BUG-006 (MINOR): Part label divides byte offset by rune budget"
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M1T4S1/research/fix_design.md
  why: "The safety proof (in-bounds via T2.S1 guard; rune-boundary via the anchor), the sibling-compat
        analysis, and the full test design with divergence math."

# THE RUNE PRIMITIVE (stdlib — the entire logic change)
- url: https://pkg.go.dev/unicode/utf8#RuneCountInString
  why: "utf8.RuneCountInString(s) returns the number of runes in s. This is the rune-equivalent of the byte
        offset st.offsets[p]. Precedent in this repo: internal/generate/multiturn.go, internal/git/tokens.go."
  critical: "RuneCountInString panics ONLY if the slice splits a multibyte rune mid-sequence. diff[:st.offsets[p]]
             is safe (st.offsets[p] is always a rune boundary — proven in the research notes). Do NOT decode
             rune-by-rune; RuneCountInString is the idiomatic one-call conversion."

# SIBLING CONTRACTS (same file, different lines — confirm non-overlap; my fix is merge-order-safe)
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M1T2S1/PRP.md
  why: "BUG-002 (COMPLETE/merged) added the st.offsets[p] >= len(diff) cursor-exhaustion guard ABOVE L392.
        My fix DEPENDS on it (it guarantees st.offsets[p] < len(diff) at L392 ⇒ the slice is in-bounds).
        Already in the current source. No conflict."
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M1T3S1/PRP.md
  why: "BUG-005 (IMPLEMENTING in parallel) rewrites nextChunk/chunkCount anchoring (@~425-450) to @@ hunk
        edges + adds anchorToHunkEdge. My line (L392, in buildReadAnswer) is untouched by T3.S1. PROVEN:
        the @@ anchor (\\n@@), its \\n fallback, and len(diff) ALL land on rune boundaries, so diff[:st.offsets[p]]
        stays safe under T3.S1 too ⇒ my fix is correct before/with/after T3.S1."
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  workdesc.go                    # THE fix: L392 numerator + Mode-A comment; +unicode/utf8 import
  generate_workdesc_test.go      # ADD: TestBuildReadAnswer_PartLabelRuneConsistent; +unicode/utf8 import
  generate_test.go               # shared helpers (initRepo@26 writeFile@34 stageFile@43 commitRaw@55) — REUSE, no edit
  multiturn.go                   # advanceRunes (same package; REUSE) — NOT touched
```

### Desired Codebase tree with files to be added/changed

```bash
internal/generate/workdesc.go                # MODIFY: L392 part numerator (byte→rune) + comment; imports +unicode/utf8
internal/generate/generate_workdesc_test.go  # MODIFY: +1 test func; imports +unicode/utf8
# (no new files; no signature/const/budget/CLI/config change; no multiturn.go/buildReadAnswer-branch change)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (unicode/utf8 is NOT imported): workdesc.go imports (@18-27) are context/fmt/strings +
//   config/git/provider. You MUST add "unicode/utf8" or the build breaks. (Precedent: multiturn.go,
//   git/tokens.go.) Place it alphabetically among the stdlib block (after "strings"). The TEST file
//   (generate_workdesc_test.go @15-25) also needs "unicode/utf8" for the rune-count guard.

// CRITICAL (slice safety — but it IS safe): diff[:st.offsets[p]] could panic if (a) st.offsets[p] >
//   len(diff), or (b) it splits a multibyte rune. NEITHER happens: (a) the T2.S1 cursor-exhaustion guard
//   (st.offsets[p] >= len(diff) ⇒ "end of diff" + continue) runs IMMEDIATELY before L392 and is MERGED, so
//   st.offsets[p] < len(diff) is guaranteed here; (b) st.offsets[p] is always a rune boundary (it is the
//   cumulative byte advance from nextChunk, whose `end` lands just past a \n byte or at len(diff) — both
//   character boundaries — under BOTH the current newline anchor AND T3.S1's @@ anchor). Do NOT add a
//   bounds re-check "to be safe" — the guard already ensures it; a redundant check is dead code.

// CRITICAL (do NOT change the offsets type or track rune offsets): Option 1 in the analysis (track rune
//   offsets alongside byte offsets) is rejected — it touches the struct + every advance site. Option 2
//   (convert at the one division site) is the contract. Keep st.offsets as a BYTE map (it MUST stay bytes
//   for diff[offset:end] slicing); convert ONLY in the part numerator.

// GOTCHA (the bug is ONLY the part numerator — leave total/advance/offsets in bytes): chunkCount already
//   counts in runes (advanceRunes) so total=N is correct; nextChunk's advance is bytes (correct for
//   slicing); st.offsets[p] is bytes (correct for slicing). The ONLY wrong unit is the L392 numerator.
//   Do NOT "also fix" advance or offsets to runes — that would break the byte slicing. Wrap ONLY the
//   numerator in utf8.RuneCountInString.

// GOTCHA (ASCII diffs are unaffected — do not "fix" them): for ASCII, byte count == rune count, so
//   part is identical pre/post-fix. Existing tests (ASCII fixtures) stay green unchanged. Do not add an
//   ASCII-specific branch.

// SCOPE: edit ONLY workdesc.go (L392 + comment + import) + generate_workdesc_test.go (1 test + import).
//   Do NOT touch nextChunk/chunkCount/chunkRuneBudget/readChunkTokenCap (T3.S1 owns the anchor; the budget
//   is correct). Do NOT touch buildReadAnswer's other branches (BUG-002 cursor-exhaustion; the single-
//   chunk branch). Do NOT touch multiturn.go. Mode A = code comment only (no user-facing docs file —
//   P1.M1.T5.S1 owns the changeset-level doc sweep).
```

## Implementation Blueprint

### Data models and structure
None. No struct/type/signature change. `st.offsets` stays `map[string]int` in BYTES (required for byte
slicing). One numerator converted to runes at the single division site.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD the unicode/utf8 import to internal/generate/workdesc.go
  - LOCATE the import block (@18-27): context, fmt, strings, then config/git/provider.
  - ADD "unicode/utf8" in the stdlib group, alphabetically after "strings" (so the block reads
    context / fmt / strings / unicode/utf8). gofmt will place it correctly if you group it with stdlib.
  - VERIFY: `go build ./internal/generate/` succeeds (unused import would error, but Task 2 uses it).
  - DEPENDENCIES: none.

Task 2: MODIFY the part-label line (L392) + Mode-A comment in internal/generate/workdesc.go
  - LOCATE (inside buildReadAnswer's `else` branch, after `if total <= 1 { … }`):
        part := (st.offsets[p] / chunkRuneBudget()) + 1
  - REPLACE WITH (the comment is Mode A — explains the byte→rune conversion AND why the slice is safe):
        // part is the 1-based chunk index. st.offsets[p] is a BYTE cursor (nextChunk's advance is bytes,
        // kept in bytes for diff[offset:end] slicing), but chunkRuneBudget() is in RUNES
        // (readChunkTokenCap*4 = 64000, matching chunkCount's advanceRunes windowing). Convert the byte
        // offset to a rune count before dividing so i and N in "part i of N" share a unit (FR-W5).
        // Safe: the cursor-exhaustion guard above guarantees st.offsets[p] < len(diff), and st.offsets[p]
        // is always a rune boundary (nextChunk anchors end to a \n / @@ / len(diff)), so the slice never
        // splits a multibyte rune.
        part := (utf8.RuneCountInString(diff[:st.offsets[p]]) / chunkRuneBudget()) + 1
  - KEEP UNCHANGED: the surrounding `else`, the Fprintf label format string (L393-394), `st.offsets[p] +=
    advance` (L396), the `if total <= 1` branch, and the cursor-exhaustion guard above.
  - DEPENDENCIES: Task 1.

Task 3: ADD the unicode/utf8 import to internal/generate/generate_workdesc_test.go
  - LOCATE the import block (@15-25): context, errors, strings, testing, time, then config/git/prompt/stubtest.
  - ADD "unicode/utf8" in the stdlib group after "strings".
  - DEPENDENCIES: none.

Task 4: CREATE TestBuildReadAnswer_PartLabelRuneConsistent in internal/generate/generate_workdesc_test.go
  - PLACE near TestBuildReadAnswer_EndOfDiff (@439); anchor by name, not line number (T1/T2/T3 siblings
    shift lines).
  - ADD (mirror TestBuildReadAnswer_EndOfDiff's real-git + primed-readState shape; large multibyte content
    via strings.Repeat like @298):
        // TestBuildReadAnswer_PartLabelRuneConsistent is the BUG-006 regression: the "part i of N" label
        // divided a BYTE cursor (st.offsets[p]) by a RUNE budget (chunkRuneBudget=64000), so multibyte
        // UTF-8 over-counted i (e.g. "part 3 of 2"). Post-fix i is the rune count of diff[:cursor] ÷ budget,
        // so the 2nd chunk of a 2-chunk multibyte diff is labeled "part 2".
        func TestBuildReadAnswer_PartLabelRuneConsistent(t *testing.T) {
            repo := t.TempDir()
            initRepo(t, repo)
            commitRaw(t, repo, "initial")
            // High multibyte density (4 CJK chars/line) so byte/rune ratio > 2 — required for the byte vs
            // rune part to diverge across a 64000 boundary after chunk 1. K=14000 ⇒ ~70K runes (>64K ⇒ ≥2 chunks).
            const path = "mb.txt"
            writeFile(t, repo, path, strings.Repeat("文本文本\n", 14000))
            stageFile(t, repo, path)

            g := git.New(repo)
            ctx := context.Background()
            opts := git.StagedDiffOptions{DiffContext: 1}
            diff, err := g.StagedFileDiff(ctx, path, opts)
            if err != nil {
                t.Fatalf("StagedFileDiff: %v", err)
            }
            // Fixture guard: the diff MUST exceed the rune budget (≥2 chunks) AND must not have been
            // truncated by MaxDiffBytes (default 300000; ~182KB here is under it). If this fails, grow K.
            if rc := utf8.RuneCountInString(diff); rc <= chunkRuneBudget() {
                t.Fatalf("fixture too small or truncated: diff is %d runes, need > %d", rc, chunkRuneBudget())
            }

            cfg := config.Defaults()
            // Prime the cursor to chunk-1's end (simulate "chunk 1 already delivered"): nextChunk(diff, 0)
            // returns the byte advance of the first chunk, which is exactly the byte cursor value buildReadAnswer
            // would have stored. The NEXT call must therefore report part == 2.
            _, total, advance := nextChunk(diff, 0)
            if total < 2 {
                t.Fatalf("fixture did not span ≥2 chunks: total=%d (grow K)", total)
            }
            st := &readState{N: 5, offsets: map[string]int{path: advance}}

            got := buildReadAnswer(ctx, g, cfg, nil, []string{path}, st)
            // The 2nd chunk is labeled "part 2 of N". Pre-fix (byte numerator) the high multibyte density
            // makes byte_offset ≥ 128000 after chunk 1 ⇒ part = floor(128000/64000)+1 = 3, emitting "part 3 of"
            // (absent here). Post-fix part = floor(runeCount(~64000)/64000)+1 = 2.
            if !strings.Contains(got, "— part 2 of") {
                t.Errorf("post-fix label must say \"part 2 of …\"; got %q", got)
            }
            // Negative guard: the nonsensical over-count (part > total, e.g. "part 3 of 2") must NOT appear.
            if strings.Contains(got, "— part 3 of") {
                t.Errorf("BUG-006 regression: byte/rune mismatch produced \"part 3 of …\"; got %q", got)
            }
        }
  - NOTE on K/robustness: if 14000 lines does not yield total≥2 on your platform (diff framing varies),
    the `total < 2` fatalf catches it — grow K. The load-bearing assertion is "part 2 of" (the 2nd chunk is
    labeled 2); the exact pre-fix value (3 vs higher) depends on density but is always >2 for ratio>2 content,
    so the negative guard ("part 3 of" absent) is robust.
  - DEPENDENCIES: Tasks 1-3.

Task 5: VERIFY build + vet + format + targeted tests + full generate package
  - go build ./...
  - go vet ./internal/generate/...
  - gofmt -l internal/generate/workdesc.go internal/generate/generate_workdesc_test.go   # must list nothing
  - go test ./internal/generate/ -run 'TestBuildReadAnswer' -v    # new + EndOfDiff + (any other buildReadAnswer tests)
  - go test ./internal/generate/ -count=1                         # full package (incl. T1/T2 sibling tests)
  - make test && make lint
  - Grep guards (Validation Level 4): the fix line uses RuneCountInString; unicode/utf8 imported in both files;
    no sibling lines touched.
  - REGRESSION-CHECK (by reasoning, optional empirical): pre-fix, TestBuildReadAnswer_PartLabelRuneConsistent
    FAILS (emits "part 3 of"); post-fix PASSES ("part 2 of"). (Optional: temporarily restore the byte
    numerator, re-run, observe FAIL, restore.)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the entire logic change — convert the byte cursor to runes at the ONE division site
// (st.offsets stays a byte map for slicing; only the numerator is converted)
// BEFORE (L392, buggy — byte ÷ rune):
//   part := (st.offsets[p] / chunkRuneBudget()) + 1
// AFTER (rune ÷ rune):
part := (utf8.RuneCountInString(diff[:st.offsets[p]]) / chunkRuneBudget()) + 1

// PATTERN: the regression test primes the cursor to chunk-1's byte advance (the real cursor value after
// delivering chunk 1), then asserts the 2nd chunk is labeled "part 2"
_, total, advance := nextChunk(diff, 0)            // advance = byte length of chunk 1
st := &readState{offsets: map[string]int{path: advance}}
got := buildReadAnswer(ctx, g, cfg, nil, []string{path}, st)
if !strings.Contains(got, "— part 2 of") { t.Errorf(...) }   // post-fix; pre-fix emits "part 3 of"
```

### Integration Points

```yaml
NO CLI / config / API / struct / signature / const / budget change. One numerator conversion + import + comment + test.

CODE:
  - internal/generate/workdesc.go imports: +"unicode/utf8" (stdlib block, after "strings")
  - internal/generate/workdesc.go L392: part numerator byte→rune via utf8.RuneCountInString(diff[:st.offsets[p]])
    + Mode-A comment (why convert + why the slice is safe)
TEST:
  - internal/generate/generate_workdesc_test.go imports: +"unicode/utf8"
  - internal/generate/generate_workdesc_test.go: +TestBuildReadAnswer_PartLabelRuneConsistent

CONSUMED (read-only, unchanged):
  - utf8.RuneCountInString (stdlib) — the byte→rune conversion
  - chunkRuneBudget() (=64000) / readChunkTokenCap (=16000) — the rune budget (correct as-is)
  - nextChunk(diff, offset) → (chunk, total, advance) — returns the byte `advance` used to prime the test cursor
  - st.offsets (byte map) — stays bytes (required for diff[offset:end] slicing)

DEPENDS ON (merged): the T2.S1 cursor-exhaustion guard (st.offsets[p] >= len(diff) ⇒ "end of diff") — it
  guarantees st.offsets[p] < len(diff) at L392 so the slice is in-bounds. Already COMPLETE/merged.

UNCHANGED (do NOT touch): nextChunk/chunkCount/chunkRuneBudget/readChunkTokenCap (T3.S1 owns the anchor; the
  budget is correct); buildReadAnswer's other branches (cursor-exhaustion [T2.S1], single-chunk label);
  multiturn.go advanceRunes; any CLI/config/user-facing doc (Mode A = code comment only; P1.M1.T5.S1 owns
  the changeset doc sweep).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...                                                   # Expected: clean (unicode/utf8 now used)
go vet ./internal/generate/...                                   # Expected: clean
gofmt -l internal/generate/workdesc.go internal/generate/generate_workdesc_test.go   # Expected: nothing listed
make lint                                                        # Expected: zero errors
# If gofmt -l lists a file, run `gofmt -w` on it. If the build fails on "imported and not used", you
# added the import but didn't use it — confirm Task 2 references utf8.RuneCountInString.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The buildReadAnswer tests: the new regression + EndOfDiff (must stay green)
go test ./internal/generate/ -run 'TestBuildReadAnswer' -v
# Expected:
#   TestBuildReadAnswer_PartLabelRuneConsistent  PASS  (BUG-006 fix — "part 2 of", not "part 3 of")
#   TestBuildReadAnswer_EndOfDiff                PASS  (BUG-002, unchanged — ASCII fixture)

# Full generate package (incl. the BUG-001/BUG-002 sibling tests + nextChunk/chunkCount tests)
go test ./internal/generate/ -count=1
# Expected: ALL pass.

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# buildReadAnswer/nextChunk are pure functions over a diff string; the unit test IS the authoritative gate.
# (The end-to-end work-description read-loop is exercised by the existing CommitStaged stub tests, which
#  use small ASCII diffs — unaffected by the byte/rune fix.) Optional: confirm a real multibyte staged file
#  labels correctly through the full path:
#   stage a >64K-rune CJK file, run work-description mode against the stub agent, capture the delivered
#   "part i of N" turns, assert i never exceeds N. (Heavy; the unit test covers the property.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the fix uses RuneCountInString (the byte→rune conversion is present)
grep -n 'utf8.RuneCountInString(diff\[:st.offsets\[p\]\])' internal/generate/workdesc.go
# Expected: 1 match (L392 numerator).

# Grep guard: the buggy byte numerator is GONE from the part computation
! grep -n 'part := (st.offsets\[p\] / chunkRuneBudget())' internal/generate/workdesc.go && echo "OK: byte numerator replaced"
# Expected: "OK: …".

# Grep guard: unicode/utf8 imported in BOTH files
grep -n '"unicode/utf8"' internal/generate/workdesc.go internal/generate/generate_workdesc_test.go
# Expected: 1 match in each file.

# Grep guard: the Mode-A comment is present (notes the byte→rune conversion + slice safety)
grep -n 'BYTE cursor\|rune boundary\|chunkRuneBudget() is in RUNES' internal/generate/workdesc.go
# Expected: the comment block above L392.

# Grep guard: the new test exists
grep -n 'TestBuildReadAnswer_PartLabelRuneConsistent' internal/generate/generate_workdesc_test.go
# Expected: the func name present.

# Scope guard: only workdesc.go + generate_workdesc_test.go changed; siblings untouched
git diff --stat -- internal/generate/multiturn.go internal/generate/generate_test.go   # Expected: empty
git diff internal/generate/workdesc.go | grep -E '^[+-]' | grep -E 'nextChunk|chunkCount|chunkRuneBudget|readChunkTokenCap|offsets\[p\] \+= advance|end of diff' || echo "OK: anchor/budget/advance/cursor-guard untouched"
# Expected: "OK: …" (those lines are sibling/unchanged scope).

# Regression-property check (by reasoning): pre-fix, TestBuildReadAnswer_PartLabelRuneConsistent emits
# "part 3 of" (byte_offset≥128000 ⇒ floor(/64000)+1=3) ⇒ FAILS; post-fix emits "part 2 of" ⇒ PASSES.
# (Optional empirical: temporarily restore `(st.offsets[p] / chunkRuneBudget()) + 1`, re-run, observe FAIL,
# then restore the rune conversion.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean (unicode/utf8 imported AND used)
- [ ] `go vet ./internal/generate/...` clean
- [ ] `gofmt -l internal/generate/workdesc.go internal/generate/generate_workdesc_test.go` empty
- [ ] `make lint` zero errors
- [ ] `go test ./internal/generate/ -count=1` passes; `make test` passes

### Feature Validation
- [ ] L392 numerator is `utf8.RuneCountInString(diff[:st.offsets[p]])` (rune ÷ rune budget)
- [ ] Multibyte 2-chunk diff labeled "part 1 of 2" … "part 2 of 2" (i never exceeds N)
- [ ] New test asserts "— part 2 of" and guards "— part 3 of" absent
- [ ] New test FAILS pre-fix / PASSES post-fix
- [ ] ASCII diffs unchanged (`TestBuildReadAnswer_EndOfDiff` + happy-path tests green)

### Scope-Boundary Validation
- [ ] `st.offsets` stays a BYTE map (not converted to runes — required for slicing)
- [ ] `nextChunk`/`chunkCount`/`chunkRuneBudget`/`readChunkTokenCap` unchanged (T3.S1 owns the anchor)
- [ ] buildReadAnswer's cursor-exhaustion guard (T2.S1) and single-chunk branch untouched
- [ ] `multiturn.go` (advanceRunes) byte-unchanged
- [ ] Only workdesc.go (L392 + comment + import) + generate_workdesc_test.go (1 test + import) touched

### Code Quality
- [ ] Mode-A comment explains BOTH the unit conversion AND why the slice is safe (in-bounds + rune boundary)
- [ ] Import placed alphabetically in the stdlib block (gofmt-clean)
- [ ] Test uses the shared helpers (initRepo/writeFile/stageFile/commitRaw) and a primed readState (mirrors
      TestBuildReadAnswer_EndOfDiff); fixture guard catches a too-small/truncated diff loudly

---

## Anti-Patterns to Avoid

- ❌ **Don't convert `st.offsets` to a rune map.** Option 1 in the analysis (track rune offsets) is rejected — it would break byte slicing (`diff[offset:end]`) at every site. The cursor MUST stay in bytes; convert ONLY at the one division site (Option 2, the contract).
- ❌ **Don't add a bounds re-check before the slice.** The T2.S1 cursor-exhaustion guard (merged) already guarantees `st.offsets[p] < len(diff)` at L392. A redundant `if st.offsets[p] > len(diff)` check is dead code (and would suggest the guard is missing — it isn't).
- ❌ **Don't decode rune-by-rune.** `utf8.RuneCountInString(s)` is the idiomatic one-call conversion (used in multiturn.go, git/tokens.go). Rolling your own `range diff[:off]` loop is needless and slower.
- ❌ **Don't forget the import in BOTH files.** `unicode/utf8` is not imported in workdesc.go (the build breaks) nor in generate_workdesc_test.go (the rune-count guard needs it). Add it to each.
- ❌ **Don't touch `nextChunk`/`chunkCount`/the anchor.** That's BUG-005 (T3.S1, parallel). Your fix is correct under BOTH the current newline anchor and T3.S1's @@ anchor (both land on rune boundaries), so leave the anchor alone — editing it would collide with T3.S1.
- ❌ **Don't change the label format string.** Only the `part` value is wrong; `"%s — part %d of %d; READ %s again for the next part:\n%s\n\n"` stays byte-identical. The fix is the numerator feeding `%d`, not the template.
- ❌ **Don't assert an exact pre-fix part value that's fragile to multibyte density.** The load-bearing assertion is "the 2nd chunk is labeled part 2" (`strings.Contains(got, "— part 2 of")`), which holds for any ratio>2 content with total≥2. The negative guard ("part 3 of" absent) is robust because byte/rune divergence always pushes i>2 for chunk 2 under high density.
- ❌ **Don't write a small-diff test and call it a regression.** A diff ≤64000 runes has `total==1`, so the `else` (part-label) branch is never reached — such a test passes on BOTH buggy and fixed code and guards nothing. The fixture MUST exceed 64000 runes (guard with `utf8.RuneCountInString(diff) > chunkRuneBudget()`).

---

## Confidence Score

**9/10** — one-pass success likelihood. The exact buggy line is quoted verbatim (L392), the exact replacement
is specified word-for-word (with the Mode-A comment), the missing import is flagged (unicode/utf8, with
in-repo precedent), the slice safety is PROVEN (in-bounds via the merged T2.S1 guard + rune-boundary via
the anchor, under both the current and T3.S1 anchoring), the sibling non-overlap is confirmed (different
lines; merge-order-independent), the test pattern to mirror is named (`TestBuildReadAnswer_EndOfDiff` @439
with shared helpers in generate_test.go), and the new test is written out with the divergence math (why
pre-fix yields "part 3" and post-fix yields "part 2"). The −1 covers two judgment calls: (1) the exact `K`
(lines of multibyte content) that yields total≥2 depends on diff framing — the test guards this with a
`total < 2` fatalf so a too-small fixture fails loudly rather than passing vacuously; and (2) the assertion
relies on byte/rune ratio >2 (high multibyte density), which the chosen `文本文本\n` content (ratio ~2.3–2.6
with diff framing) satisfies but the implementer should keep dense (≥3-byte chars, several per line). Both
are flagged with guards/notes.