# Research: P1.M1.T2.S1 — Cursor-exhaustion check in buildReadAnswer → 'end of diff' note (BUG-002)

**Scope**: Fix BUG-002 (Minor) in internal/generate/workdesc.go buildReadAnswer: when a file's read
cursor is exhausted, emit the FR-W5 "end of diff" note instead of an empty body. ~3-line insert + a
comment refinement + a focused unit test. All verified against the working tree this session.

## 1. The bug (exact location + root cause)

**buildReadAnswer** (workdesc.go:265) loops over each READ path `p`:
```go
diff, err := g.StagedFileDiff(ctx, p, opts)      // returns the FULL diff (cursor-unaware)
if err != nil || diff == "" {                     // @~275 — covers not-staged / read-error
    fmt.Fprintf(&b, "%s is not in the staged changes (or has no further diff).\n\n", p)
    continue
}
chunk, total, advance := nextChunk(diff, st.offsets[p])   // @280 — cursor exhausted ⇒ ("", 1, 0)
if total <= 1 {                                          // @281 — TRUE (total==1)
    fmt.Fprintf(&b, "%s:\n%s\n\n", p, chunk)             // prints "p:\n\n" — EMPTY BODY (the bug)
} else { ... part i of N ... }
st.offsets[p] += advance                                 // advance==0 (no progress)
```

**Root cause:** `StagedFileDiff` returns the FULL non-empty diff for a staged file on every call
(cursor-unaware). So when the cursor is exhausted (`st.offsets[p] >= len(diff)`), `diff` is NON-EMPTY ⇒
the `diff == ""` branch does NOT fire. `nextChunk` then returns `("", 1, 0)` (offset≥len ⇒ empty chunk,
total==1, advance==0), and the `total <= 1` branch prints `"p:\n\n"` — an empty body. The FR-W5
"end of diff" note never appears. (The bug analysis confirms: the `diff == ""` branch "never fires for a
staged file" — exhaustion is invisible to it.)

## 2. The fix (insert ONE branch, refine ONE comment)

**Insert** (between the `diff == ""` check @~275 and the `nextChunk` call @~280):
```go
if st.offsets[p] >= len(diff) {
    // FR-W5: cursor exhausted — the model re-requested a file whose diff was already fully delivered.
    fmt.Fprintf(&b, "%s — end of diff (all parts shown).\n\n", p)
    continue
}
```
Note text per FR-W5: "After the final chunk, a re-request returns '<path> — end of diff'." The item's
exact text: `"%s — end of diff (all parts shown).\n\n"`.

**Comment refinement (Mode A):** the CURRENT `diff == ""` branch comment (@~276) claims it covers
"Either not staged, fully read (cursor exhausted), or a read error." That "fully read (cursor
exhausted)" clause is INACCURATE — cursor exhaustion never reaches `diff == ""` (StagedFileDiff returns
the full diff). After the fix, exhaustion is handled by the NEW branch. Update the `diff == ""` comment
to: "Either not staged or a read error (cursor exhaustion is handled below, before nextChunk)." And cite
FR-W5 on the new branch.

**Why the order is correct:** fetch diff → if empty/error (not-staged/read-error) note it → if cursor
exhausted (staged, fully read) note "end of diff" → else nextChunk (deliver the next chunk). The new
check MUST go AFTER `diff == ""` (so a non-staged file with diff=="" is noted as "not in staged
changes", not "end of diff") and BEFORE `nextChunk` (so the empty-chunk path is never reached for an
exhausted cursor).

## 3. Anchors (workdesc.go — NON-OVERLAPPING with the BUG-001 sibling)

| Symbol | Line | Owner |
|---|---|---|
| `RunWorkDescription` `len(paths)==0` branch | ~99-102 | **BUG-001 (P1.M1.T1.S1)** — NOT this task |
| parseReadLines / stripReadLines / new helpers | ~135-205 | **BUG-001** — NOT this task |
| `buildReadAnswer` | 265 | **THIS task (BUG-002)** |
| the `diff == ""` branch | ~275 | THIS task (comment refinement) |
| the `nextChunk` call | 280 | THIS task (insert the new check just above it) |
| `nextChunk` / `chunkCount` / `chunkRuneBudget` | 299 / ~330 / 319 | BUG-005/006 (P1.M1.T3/T4) — NOT this task |
| `readState.offsets` | 46 | consumed (read cursor), not modified |

**Sibling coordination:** BUG-001 (P1.M1.T1.S1, Implementing in parallel) edits the `len(paths)==0`
branch (~99-102) + adds helpers (~135-205). Its PRP EXPLICITLY scopes `buildReadAnswer`/`nextChunk` OUT
("BUG-002/005/006 are sibling subtasks"). So the regions are NON-OVERLAPPING. Same file ⇒ anchor by
SYMBOL NAME (`func buildReadAnswer`), not line number (the sibling's helper additions above may shift
lines down). No logical conflict.

## 4. readState + StagedFileDiff + nextChunk contracts (the consumed seams)

- **readState** (workdesc.go:43): `offsets map[string]int` — path → byte offset (the implicit cursor).
  Constructed via the package's readState initializer (~line 88: `offsets: make(map[string]int)`).
- **StagedFileDiff** (git.Git, git.go:478): `(ctx, path, opts) (diff string, err error)` — returns the
  FULL staged diff body for one path; `""` for a non-staged/missing path. Cursor-unaware.
- **nextChunk** (workdesc.go:299): `(diff, offset) (chunk, total, advance)` — offset≥len(diff) ⇒
  `("", 1, 0)`. Its doc comment already says "the caller notes 'end of diff'" — this task makes that true.

## 5. Test design (mirror TestStagedFileDiff_SinglePath — real git.New(repo), no mock)

The existing idiom (generate_workdesc_test.go:406) uses a REAL `git.New(repo)` on a temp repo + a staged
file, NOT a mock git.Git. Mirror it for `TestBuildReadAnswer_EndOfDiff`:
1. `repo := t.TempDir()`; init git; seed a staged file `a.go` with a real change (so StagedFileDiff
   returns a non-empty diff). Use the package's existing repo-init helpers if present (grep
   `generate_workdesc_test.go` for the init/seed idiom TestStagedFileDiff_SinglePath uses).
2. `g := git.New(repo)`; fetch `diff, _ := g.StagedFileDiff(ctx, "a.go", opts)` to learn its length.
3. Build a readState with the cursor EXHAUSTED: `st := <readState initializer>; st.offsets["a.go"] =
   len(diff) + 1` (deterministic — no hardcoded length). (Grep for the readState constructor at ~line 88
   to construct a valid one; set N/rounds to safe values.)
4. `got := buildReadAnswer(ctx, g, cfg, nil, []string{"a.go"}, st)`.
5. ASSERT `strings.Contains(got, "a.go — end of diff (all parts shown).")` AND assert `got` does NOT
   contain the empty-body form (`"a.go:\n\n"` / a bare `"a.go:"` with no diff body).

Also add a NON-exhaustion control (recommended): the same setup with `st.offsets["a.go"] = 0` (cursor at
start) → `buildReadAnswer` delivers the chunk (contains the diff body, NOT "end of diff"). This proves
the new branch doesn't fire prematurely.

Test file: `internal/generate/generate_workdesc_test.go` (`package generate` — internal ⇒ buildReadAnswer
is reachable unexported). The sibling (BUG-001) ALSO adds tests here (TestContainsReadVerb etc.) —
different function names, additive, no conflict.

## 6. What this task does NOT do (scope fences)

- Does NOT touch RunWorkDescription's `len(paths)==0` branch / the new BUG-001 helpers (~99-205).
- Does NOT touch nextChunk / chunkCount / chunkRuneBudget (BUG-005 = P1.M1.T3, BUG-006 = P1.M1.T4).
- Does NOT change the `part i of N` label (BUG-006).
- Does NOT change buildReadAnswer's signature, the loop structure, or the not-staged/read-error note.
- Does NOT change StagedFileDiff, readState struct, or RunWorkDescription's signature.

## 7. Validation

- `go build ./...`; `go vet ./internal/generate/...`; `gofmt -l internal/generate/workdesc.go`.
- `go test ./internal/generate/ -run TestBuildReadAnswer_EndOfDiff -v` (the new test).
- `go test ./internal/generate/...` (full package — existing tests incl. the sibling's BUG-001 tests must pass).
- `make test && make lint`.
- Grep guard: `grep -n 'end of diff (all parts shown)' internal/generate/workdesc.go` → 1 hit (the new branch).
- Regression-property check: pre-fix the exhausted-cursor case printed `"a.go:\n\n"` (empty body);
  post-fix it prints the "end of diff" note. The test would FAIL on the old code.