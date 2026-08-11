name: "P1.M1.T2.S1 — buildReadAnswer cursor-exhaustion check → FR-W5 'end of diff' note (BUG-002)"
description: >
  Fix BUG-002 (Minor): in internal/generate/workdesc.go buildReadAnswer, when a file's read cursor is
  exhausted (st.offsets[p] >= len(diff)), emit the FR-W5 "<path> — end of diff (all parts shown)." note
  instead of an empty body ("p:\n\n"). StagedFileDiff returns the full non-empty diff on every call
  (cursor-unaware), so the diff=="" branch never catches exhaustion; nextChunk returns ("",1,0) and the
  total<=1 branch prints an empty body. Insert ONE branch between the diff=="" check and the nextChunk
  call; refine the diff=="" comment (remove the inaccurate "fully read" clause); add a focused unit test
  mirroring TestStagedFileDiff_SinglePath (real git.New(repo)). Only buildReadAnswer touched; the BUG-001
  sibling (len(paths)==0 branch + helpers) is a non-overlapping region of the same file.

---

## Goal

**Feature Goal**: Make a re-READ of an already-fully-read file in work-description mode produce the
explicit FR-W5 "end of diff" note instead of an empty body — so the model gets a clear signal that the
file's diff is fully delivered, rather than an empty `a.txt:\n\n` that looks like a glitch.

**Deliverable**: (1) ONE inserted branch in `buildReadAnswer` (workdesc.go, between the `diff == ""`
check and the `nextChunk` call): `if st.offsets[p] >= len(diff) { emit "end of diff"; continue }`;
(2) a refinement of the `diff == ""` branch's comment (remove the inaccurate "fully read (cursor
exhausted)" clause — that case now routes to the new branch) + an FR-W5 citation on the new branch
(Mode A); (3) a focused unit test `TestBuildReadAnswer_EndOfDiff` (+ a non-exhaustion control).

**Success Definition**:
- A buildReadAnswer call where `st.offsets[p] >= len(diff)` (staged file, cursor exhausted) emits
  `"<p> — end of diff (all parts shown).\n\n"` and does NOT emit the empty-body `"p:\n\n"` form.
- A staged file with cursor at 0 (`st.offsets[p] == 0`) still delivers the diff chunk (unchanged) —
  the new branch does not fire prematurely.
- A non-staged file (`diff == ""`) still emits "is not in the staged changes" (unchanged).
- The existing buildReadAnswer behavior (chunk delivery, part labels, not-staged note) is unchanged.
- `go build ./...`, `go test ./internal/generate/...`, `make test`, `make lint` pass.

## User Persona (if applicable)

**Target User**: A user running `stagecoach --work-description` whose agent re-requests a file it already
fully read.

**Use Case**: The model READs `a.go`, gets part 1; READs `a.go` again, gets part 2 (or part 1 if it fit
in one chunk); READs `a.go` a third time after the diff is exhausted → gets "a.go — end of diff (all
parts shown)." (a clear signal), not an empty body.

**Pain Points Addressed**: BUG-002 — an empty body on cursor exhaustion looks like a glitch and gives
the model no signal that the diff is complete, which can cause it to re-request pointlessly or conclude
on a confusing turn.

## Why

- **BUG-002 (Minor) / FR-W5**: FR-W5 specifies "After the final chunk, a re-request returns '<path> —
  end of diff'." The code never emitted this note for a staged file because `StagedFileDiff` returns the
  full non-empty diff (cursor-unaware), so the `diff == ""` branch (the only place the "no further diff"
  note lived) never fired for exhaustion. The cursor-exhaustion signal was lost, producing an empty body.
  This fix adds the explicit check, bringing the code into line with FR-W5.
- **Bounded, surgical**: one ~3-line branch + a comment refinement + a test. No signature/loop/struct
  change. The BUG-001 sibling is in a non-overlapping region of the same file.

## What

**User-visible behavior**: A re-READ of a fully-read file in `--work-description` mode produces the
"end of diff" note instead of an empty body.

**Technical change (one inserted branch + comment + test):**
1. In `buildReadAnswer`, AFTER `if err != nil || diff == "" { … continue }` and BEFORE
   `chunk, total, advance := nextChunk(diff, st.offsets[p])`, insert:
   `if st.offsets[p] >= len(diff) { fmt.Fprintf(&b, "%s — end of diff (all parts shown).\n\n", p); continue }`.
2. Refine the `diff == ""` branch comment: remove "fully read (cursor exhausted)" (inaccurate — that
   case now routes to the new branch); state it covers not-staged / read-error.
3. Mode A: cite FR-W5 on the new branch's comment.
4. Test: `TestBuildReadAnswer_EndOfDiff` (+ control) in `generate_workdesc_test.go`.

### Success Criteria
- [ ] exhausted cursor (`st.offsets[p] >= len(diff)`) → "end of diff (all parts shown)." note
- [ ] the empty-body `"p:\n\n"` form is NOT produced for an exhausted staged file
- [ ] non-staged (`diff == ""`) → "is not in the staged changes" (unchanged)
- [ ] cursor at 0 → chunk delivered (unchanged; new branch does not fire)
- [ ] `diff == ""` branch comment no longer claims "fully read (cursor exhausted)"
- [ ] FR-W5 cited on the new branch (Mode A)
- [ ] `go build ./...`, `go test ./internal/generate/...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact buggy lines, the exact insert (text + placement), the consumed seams (readState.offsets,
StagedFileDiff, nextChunk), the sibling's non-overlapping region, the test idiom (real git.New(repo),
mirroring TestStagedFileDiff_SinglePath), and the scope fences are all below.

### Documentation & References

```yaml
- file: internal/generate/workdesc.go
  why: "THE file. buildReadAnswer @265; the diff=='' branch @~275 (comment to refine); the nextChunk
        call @280 (insert the new branch just ABOVE it); the part-label line @284; st.offsets[p] +=
        advance @288. readState.offsets @46 (the cursor). nextChunk @299 (offset≥len ⇒ '',1,0)."
  pattern: >
    // CURRENT loop body per path p:
    diff, err := g.StagedFileDiff(ctx, p, opts)
    if err != nil || diff == "" { /* note "not in staged changes"; continue */ }
    // ← INSERT the cursor-exhaustion check HERE
    chunk, total, advance := nextChunk(diff, st.offsets[p])
    if total <= 1 { fmt.Fprintf(&b, "%s:\n%s\n\n", p, chunk) } else { /* part i of N */ }
    st.offsets[p] += advance
  critical: "Insert the new check AFTER `diff == \"\"` (so a non-staged file is noted as 'not in staged
             changes', not 'end of diff') and BEFORE `nextChunk` (so the empty-chunk path is never
             reached for an exhausted cursor). Anchor on `func buildReadAnswer` (symbol name), NOT line
             number — the BUG-001 sibling adds helpers above (~135-205) that may shift lines."

- file: internal/generate/workdesc.go (consumed seams — DO NOT edit)
  why: "readState @43 (offsets map[string]int @46 — the cursor); StagedFileDiff via git.Git
        (git.go:478, returns the FULL non-empty diff cursor-unaware); nextChunk @299 (offset≥len ⇒
        ('', 1, 0); its doc already says 'the caller notes end of diff' — this task makes that true)."
  critical: "StagedFileDiff returns the FULL diff every call — that's WHY diff=='' never catches
             exhaustion for a staged file (the root cause). Do NOT change StagedFileDiff or nextChunk."

- file: internal/generate/generate_workdesc_test.go
  why: "THE test file (package generate — internal ⇒ buildReadAnswer reachable unexported). Mirror
        TestStagedFileDiff_SinglePath @406 (real git.New(repo) on a temp repo + a staged file, NOT a
        mock). The BUG-001 sibling ALSO adds tests here (TestContainsReadVerb etc.) — different function
        names, additive, no conflict."
  pattern: "repo:=t.TempDir(); init git; stage a.go with a change; g:=git.New(repo); diff,_:=g.
            StagedFileDiff(ctx,'a.go',opts); st:=<readState initializer>; st.offsets['a.go']=len(diff)+1;
            got:=buildReadAnswer(ctx,g,cfg,nil,[]string{'a.go'},st); assert Contains(got,'end of diff')."
  critical: "Set st.offsets['a.go'] = len(diff)+1 (deterministic — fetch diff first to learn its length;
             no hardcoded length). Grep for the readState initializer (~line 88) to construct a valid
             readState (set N/rounds to safe values). Add a control: offsets=0 ⇒ chunk delivered, NOT
             'end of diff' (proves the branch doesn't fire prematurely)."

- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/architecture/bugfix_workdesc.md
  why: "§BUG-002 is the authoritative analysis: StagedFileDiff is cursor-unaware (returns full diff) ⇒
        the diff=='' branch never fires for exhaustion ⇒ nextChunk returns ('',1,0) ⇒ empty body. Gives
        the exact fix (the st.offsets[p] >= len(diff) check) + the FR-W5 note text."
  section: "BUG-002 (MINOR): Fully-read file emits empty body instead of 'end of diff' note"

- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M1T1S1/PRP.md
  why: "The parallel sibling (BUG-001). It edits RunWorkDescription's len(paths)==0 branch (~99-102) +
        adds helpers (~135-205). Its scope EXPLICITLY leaves buildReadAnswer/nextChunk to the BUG-002/005/
        006 subtasks. Read it to confirm the non-overlap (buildReadAnswer @265 is OUTSIDE its regions)."
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  workdesc.go                       # THE fix: buildReadAnswer @265 (insert cursor-exhaustion branch before nextChunk @280) + comment refine @~275
  generate_workdesc_test.go         # +TestBuildReadAnswer_EndOfDiff (+ control) — mirror TestStagedFileDiff_SinglePath @406
internal/git/git.go                 # StagedFileDiff @478 — CONSUMED (cursor-unaware), not edited
```

### Desired Codebase tree with files to be added

```bash
internal/generate/workdesc.go                # MODIFY: +1 branch in buildReadAnswer + comment refine (Mode A)
internal/generate/generate_workdesc_test.go  # MODIFY: +TestBuildReadAnswer_EndOfDiff (+ control)
# (no new files; no other package touched)
```

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (insert order — AFTER diff=="", BEFORE nextChunk): the new `st.offsets[p] >= len(diff)`
//   check MUST go after the `diff == ""` branch (so a non-staged file with diff=="" is noted "not in
//   staged changes", NOT "end of diff") and before `nextChunk` (so the empty-chunk/total<=1 path is
//   never reached for an exhausted cursor). Getting the order wrong either mislabels non-staged files
//   as "end of diff" or still hits the empty-body path.

// CRITICAL (StagedFileDiff is cursor-unaware — the root cause): g.StagedFileDiff returns the FULL
//   non-empty diff for a staged file on EVERY call. So `diff == ""` does NOT catch cursor exhaustion
//   (diff is non-empty). The `st.offsets[p] >= len(diff)` check is the ONLY way to detect exhaustion.
//   Do NOT "fix" this by making StagedFileDiff cursor-aware (out of scope; would change its contract).

// CRITICAL (the diff=="" comment is currently inaccurate): it claims "Either not staged, fully read
//   (cursor exhausted), or a read error." The "fully read (cursor exhausted)" clause is WRONG — that
//   case never reaches diff=="" (StagedFileDiff returns the full diff). After the fix, exhaustion is
//   handled by the NEW branch. Update the comment to: "Either not staged or a read error (cursor
//   exhaustion is handled below, before nextChunk)." Don't leave the stale claim.

// GOTCHA (anchor by symbol name, not line number): the BUG-001 sibling (P1.M1.T1.S1, parallel) adds
//   helpers above buildReadAnswer (~135-205), which may shift buildReadAnswer's line numbers DOWN.
//   Locate buildReadAnswer by `grep -n 'func buildReadAnswer' internal/generate/workdesc.go` and edit
//   the branch relative to the `nextChunk(diff, st.offsets[p])` call inside it. Do NOT blind-edit by
//   the line numbers in this PRP (they're a snapshot).

// GOTCHA (non-overlapping with the sibling): BUG-001 edits RunWorkDescription's len(paths)==0 branch
//   (~99-102) + new helpers (~135-205). buildReadAnswer (@265) is OUTSIDE both. No logical conflict;
//   same file ⇒ textual merge only (both add different things in different regions).

// GOTCHA (note text — match FR-W5 / the item): use exactly "%s — end of diff (all parts shown).\n\n"
//   (em-dash, the parenthetical, the trailing blank line). The existing not-staged note uses a DIFFERENT
//   wording ("is not in the staged changes (or has no further diff)") — don't conflate them.

// SCOPE: do NOT touch RunWorkDescription's len(paths)==0 branch or the BUG-001 helpers (~99-205);
//   nextChunk/chunkCount/chunkRuneBudget (BUG-005=P1.M1.T3, BUG-006=P1.M1.T4); the part-label line;
//   StagedFileDiff; readState struct; buildReadAnswer's signature or loop structure.
```

## Implementation Blueprint

### Data models and structure
None. One inserted branch + a comment refinement. No struct/signature/loop change. The `readState.offsets`
cursor and `StagedFileDiff` are consumed unchanged.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/generate/workdesc.go — insert the cursor-exhaustion branch in buildReadAnswer
  - LOCATE buildReadAnswer by symbol: `grep -n 'func buildReadAnswer' internal/generate/workdesc.go`.
  - FIND the `diff == ""` branch (`if err != nil || diff == "" { ... continue }`) and the next line
    (`chunk, total, advance := nextChunk(diff, st.offsets[p])`).
  - INSERT between them:
        // FR-W5: cursor exhausted — the model re-requested a file whose diff was already fully
        // delivered. StagedFileDiff returns the full diff (cursor-unaware), so exhaustion is detected
        // here (offset >= len), not by the diff=="" branch above.
        if st.offsets[p] >= len(diff) {
            fmt.Fprintf(&b, "%s — end of diff (all parts shown).\n\n", p)
            continue
        }
  - DO NOT touch: the diff=="" branch body (the note), the nextChunk call, the total<=1 / part-label
    branches, or st.offsets[p] += advance.
  - DEPENDENCIES: none.

Task 2: EDIT internal/generate/workdesc.go — refine the diff=="" branch comment (Mode A accuracy)
  - The diff=="" branch comment currently says: "Either not staged, fully read (cursor exhausted), or a
    read error. Note it (FR-W3/FR-W5)."
  - REPLACE with: "Either not staged or a read error (cursor exhaustion is handled below, before
    nextChunk — StagedFileDiff returns the full non-empty diff, so exhaustion never reaches diff=='').
    Note it (FR-W3)." (Remove the inaccurate "fully read (cursor exhausted)" clause + the stale FR-W5
    ref; FR-W5 now lives on the new branch.)
  - DEPENDENCIES: Task 1.

Task 3: ADD TestBuildReadAnswer_EndOfDiff (+ control) to internal/generate/generate_workdesc_test.go
  - MIRROR TestStagedFileDiff_SinglePath (@406): real git.New(repo) on a temp repo + a staged file, NOT
    a mock git.Git. Reuse that test's repo-init/seed idiom (grep it for the exact helpers).
  - EXHAUSTION CASE:
      repo := <temp repo>; init git; seed + stage "a.go" with a real change.
      g := git.New(repo)
      opts := git.StagedDiffOptions{DiffContext: 1}   // mirror TestStagedFileDiff_SinglePath's opts
      diff, err := g.StagedFileDiff(context.Background(), "a.go", opts); if err != nil { t.Fatal(err) }
      st := <readState initializer>   // grep workdesc.go ~line 88 for the constructor; set N/rounds safe
      st.offsets["a.go"] = len(diff) + 1              // cursor EXHAUSTED (deterministic; no hardcoded len)
      got := buildReadAnswer(context.Background(), g, cfg, nil, []string{"a.go"}, st)
      if !strings.Contains(got, "a.go — end of diff (all parts shown).") {
          t.Errorf("exhausted cursor: got %q, want the 'end of diff' note", got)
      }
      if strings.Contains(got, "a.go:\n\n") || strings.Contains(got, "a.go:\n") && !strings.Contains(got,"end of diff") {
          t.Errorf("exhausted cursor: got empty-body form %q", got)   // the bug
      }
  - CONTROL CASE (recommended): same setup but st.offsets["a.go"] = 0 (cursor at start) → got must
      contain the diff body AND must NOT contain "end of diff" (proves the branch doesn't fire early).
  - PLACE near TestStagedFileDiff_SinglePath (group the StagedFileDiff/buildReadAnswer tests).
  - DEPENDENCIES: Tasks 1-2.

Task 4: VERIFY build + vet + format + targeted test + full package
  - go build ./...
  - go vet ./internal/generate/...
  - gofmt -l internal/generate/workdesc.go internal/generate/generate_workdesc_test.go
  - go test ./internal/generate/ -run TestBuildReadAnswer_EndOfDiff -v
  - go test ./internal/generate/...            # full package (incl. the sibling's BUG-001 tests)
  - make test && make lint
  - Grep guard: `grep -n 'end of diff (all parts shown)' internal/generate/workdesc.go` → 1 hit.
  - DEPENDENCIES: Tasks 1-3.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the buildReadAnswer loop body AFTER the fix (one branch inserted, order-critical)
diff, err := g.StagedFileDiff(ctx, p, opts)
if err != nil || diff == "" {
    // not-staged / read-error (cursor exhaustion handled below; StagedFileDiff is cursor-unaware)
    fmt.Fprintf(&b, "%s is not in the staged changes (or has no further diff).\n\n", p)
    continue
}
// FR-W5: cursor exhausted — re-request of a fully-delivered diff.
if st.offsets[p] >= len(diff) {
    fmt.Fprintf(&b, "%s — end of diff (all parts shown).\n\n", p)
    continue
}
chunk, total, advance := nextChunk(diff, st.offsets[p])   // now only reached when a chunk remains
// ... (total<=1 / part-label / st.offsets += advance UNCHANGED) ...

// PATTERN: the test (real git.New(repo), deterministic cursor via len(diff)+1)
diff, _ := g.StagedFileDiff(ctx, "a.go", opts)
st.offsets["a.go"] = len(diff) + 1   // exhausted
got := buildReadAnswer(ctx, g, cfg, nil, []string{"a.go"}, st)
// assert Contains(got, "a.go — end of diff (all parts shown).")
```

### Integration Points

```yaml
NO struct / signature / config / public-API / loop-structure changes. One inserted branch + comment + test.

CODE:
  - internal/generate/workdesc.go buildReadAnswer — +cursor-exhaustion branch (before nextChunk); diff=="" comment refined (Mode A)
TESTS:
  - internal/generate/generate_workdesc_test.go — +TestBuildReadAnswer_EndOfDiff (+ control)

CONSUMED (read-only, unchanged):
  - readState.offsets (the cursor map); g.StagedFileDiff (cursor-unaware, returns full diff); nextChunk

UNCHANGED (do NOT touch): RunWorkDescription's len(paths)==0 branch + BUG-001 helpers (~99-205);
  nextChunk/chunkCount/chunkRuneBudget (BUG-005/006); the part-label line; StagedFileDiff; readState
  struct; buildReadAnswer's signature/loop; any other caller.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...
go vet ./internal/generate/...
gofmt -l internal/generate/workdesc.go internal/generate/generate_workdesc_test.go
# Expected: nothing listed. If listed: gofmt -w them.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new cursor-exhaustion test (+ control)
go test ./internal/generate/ -run TestBuildReadAnswer_EndOfDiff -v
# Expected: PASS — exhausted cursor → "end of diff"; control (offset=0) → chunk delivered.

# Full generate package (existing tests + the sibling's BUG-001 tests must pass; buildReadAnswer existing behavior unchanged)
go test ./internal/generate/... -v

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# The within-scope proof is the unit test (buildReadAnswer is a pure-ish function over a real repo's diff).
# Optional manual confirmation of the end-to-end FR-W5 behavior:
#   stagecoach --work-description with a stub agent scripted to READ a.go three times (exhausting a small diff)
#   → the third answer contains "a.go — end of diff (all parts shown)." instead of an empty body.
# (The unit test is the deterministic gate; the manual repro is optional.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the new branch exists with the FR-W5 note text
grep -n 'end of diff (all parts shown)' internal/generate/workdesc.go
# Expected: one hit (the new branch).

# Grep guard: the diff=="" comment no longer claims "fully read (cursor exhausted)"
grep -n 'fully read' internal/generate/workdesc.go
# Expected: empty (the stale clause removed) — or only a correct reference if the phrase appears elsewhere.

# Grep guard: the new branch is BEFORE nextChunk (order-critical)
grep -n 'st.offsets\[p\] >= len(diff)\|nextChunk(diff, st.offsets\[p\])' internal/generate/workdesc.go
# Expected: the exhaustion-check line appears BEFORE the nextChunk line within buildReadAnswer.

# Scope-boundary guard: ONLY workdesc.go + generate_workdesc_test.go changed by this subtask
git diff --name-only
# Expected: only those two files (the BUG-001 sibling touches the same files in different regions —
#           confirm via git diff that this subtask's hunks are within buildReadAnswer + the new test).

# Scope-boundary guard: buildReadAnswer's signature + loop structure unchanged (only an inserted branch)
git diff internal/generate/workdesc.go | grep -E '^[-+].*func buildReadAnswer|nextChunk\(diff, st.offsets' | head
# Expected: the nextChunk call is UNCHANGED (no -/+ on it); only the inserted branch is a +.

# Regression-property check (by reasoning): pre-fix, an exhausted cursor printed "a.go:\n\n" (empty body);
# post-fix it prints the "end of diff" note. TestBuildReadAnswer_EndOfDiff would FAIL on the old code.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/generate/...` clean
- [ ] `gofmt -l internal/generate/workdesc.go internal/generate/generate_workdesc_test.go` empty
- [ ] `make lint` zero errors
- [ ] `go test ./internal/generate/...` passes; `make test` passes

### Feature Validation
- [ ] exhausted cursor (`st.offsets[p] >= len(diff)`) → "end of diff (all parts shown)." note
- [ ] empty-body `"p:\n\n"` form NOT produced for an exhausted staged file
- [ ] non-staged (`diff == ""`) → "is not in the staged changes" (unchanged)
- [ ] cursor at 0 → chunk delivered (new branch does not fire prematurely)
- [ ] diff=="" comment no longer claims "fully read (cursor exhausted)"; FR-W5 cited on the new branch

### Scope-Boundary Validation
- [ ] NO change to RunWorkDescription's len(paths)==0 branch / BUG-001 helpers (~99-205)
- [ ] NO change to nextChunk/chunkCount/chunkRuneBudget (BUG-005/006) or the part-label line
- [ ] NO change to StagedFileDiff, readState struct, buildReadAnswer signature/loop
- [ ] Only buildReadAnswer (1 inserted branch + comment) + generate_workdesc_test.go (1 test) touched

### Code Quality
- [ ] The new branch is inserted in the order-critical position (after diff=="", before nextChunk)
- [ ] Note text matches FR-W5 / the item exactly (em-dash + parenthetical + trailing blank line)
- [ ] Test mirrors TestStagedFileDiff_SinglePath (real git.New(repo), deterministic cursor via len(diff)+1)
- [ ] Anchored by symbol name (`func buildReadAnswer`), not stale line numbers

---

## Anti-Patterns to Avoid

- ❌ Don't insert the check in the wrong order — it MUST be after `diff == ""` (so non-staged files are noted "not in staged changes", not "end of diff") and before `nextChunk` (so the empty-chunk path is never reached). Putting it before `diff == ""` would mislabel non-staged files; putting it after `nextChunk` would still hit the empty-body `total<=1` branch.
- ❌ Don't rely on the `diff == ""` branch to catch exhaustion — StagedFileDiff returns the FULL non-empty diff (cursor-unaware), so exhaustion never reaches `diff == ""`. The `st.offsets[p] >= len(diff)` check is the ONLY detector. (And don't "fix" StagedFileDiff to be cursor-aware — out of scope, changes its contract.)
- ❌ Don't leave the stale `diff == ""` comment claiming "fully read (cursor exhausted)" — that's inaccurate (the case never reaches that branch). Refine it to "not staged or a read error"; move the FR-W5 citation to the new branch.
- ❌ Don't conflate the note wordings — the not-staged note is "is not in the staged changes (or has no further diff)"; the exhaustion note is "<p> — end of diff (all parts shown)." (FR-W5). They are different semantics; don't merge them.
- ❌ Don't touch the BUG-001 sibling's region (RunWorkDescription's len(paths)==0 branch ~99-102 / the new helpers ~135-205) — buildReadAnswer (@265) is outside both. Anchor by `func buildReadAnswer`, not line number (the sibling's helper additions shift lines).
- ❌ Don't touch nextChunk/chunkCount/chunkRuneBudget (BUG-005 = P1.M1.T3) or the part-label line (BUG-006 = P1.M1.T4) — those are sibling subtasks with their own scope.
- ❌ Don't change buildReadAnswer's signature, the loop structure, or `st.offsets[p] += advance` — only INSERT the one branch + refine the comment.
- ❌ Don't hardcode the diff length in the test — fetch `diff, _ := g.StagedFileDiff(...)` first and set `st.offsets[p] = len(diff) + 1` (deterministic; survives diff-content changes).
- ❌ Don't skip the non-exhaustion control test — without it, a check that ALWAYS fires (e.g. `>= 0`) would pass the exhaustion case but break normal delivery. The control (offset=0 → chunk delivered) proves the branch fires ONLY when exhausted.

---

## Confidence Score: 10/10

This is one ~3-line inserted branch in a single function + a comment refinement + a focused test, with
the exact insert text and order-critical placement specified, the root cause (StagedFileDiff is
cursor-unaware) documented, the consumed seams (readState.offsets, StagedFileDiff, nextChunk) verified,
the sibling's non-overlapping region confirmed (BUG-001's PRP explicitly scopes buildReadAnswer out), and
the test idiom pinned (mirror TestStagedFileDiff_SinglePath, real git.New(repo), deterministic cursor via
len(diff)+1, + a non-exhaustion control). The only conceivable failure modes — wrong insert order, leaving
the stale comment, touching the sibling's region, or a test without the control — are each explicitly
guarded by a CRITICAL gotcha + a Level-4 grep check.