name: "P1.M1.T1.S1 — containsReadVerb helper + fix the len(paths)==0 branch in RunWorkDescription (BUG-001)"
description: >
  Fix BUG-001 (Major): in work-description mode, a model READ request for a NON-STAGED path was silently
  parsed as the commit subject (e.g. "READ typo.go" became the commit message). Add a `containsReadVerb`
  gate + a `readTargets` raw-target extractor + a `buildNonStagedReadAnswer` note-builder, and restructure
  RunWorkDescription's `len(paths)==0` branch into a 3-way split: staged READ (normal path), non-staged
  READ (FR-W3 notes + loop continuation bounded by the round cap), genuine no-READ (FR-W7 message).
  Behavior-preserving for the two non-bug cases. Internal helpers (test file is `package generate`).

---

## Goal

**Feature Goal**: Stop work-description mode from turning a `READ <non-staged-path>` response into a
garbage commit subject (FR-W3/W7). When the model READs a path that isn't in the staged skeleton, emit
an FR-W3 note ("is not in the staged changes") and continue the read loop (bounded by the round cap),
instead of parsing the READ line as the message.

**Deliverable**: (1) `containsReadVerb(response) bool` helper; (2) `readTargets(response) []string`
raw-target extractor; (3) `buildNonStagedReadAnswer(targets) string` note-builder; (4) the restructured
`len(paths)==0` branch in RunWorkDescription (3-way split routing non-staged READs through the round-cap
check); (5) updated code comment + unit tests.

**Success Definition**:
- A response of ONLY `READ typo.go` (typo.go not staged) → NOT parsed as a message; an FR-W3 note turn
  is sent; the loop continues until the model emits a real message or the round cap forces conclusion.
- A STAGED READ (`READ a.txt`, a.txt staged) → unchanged (normal diff-chunk path).
- A genuine no-READ response (`feat: add thing`) → unchanged (FR-W7 message return).
- The loop stays bounded: a model that keeps emitting non-staged READs hits the round cap → forced
  conclusion (stripReadLines + ParseOutput).
- Existing `TestParseReadLines_*` / `TestStripReadLines` pass unchanged; new helper + behavior tests pass.
- `go build ./...`, `go test ./internal/generate/...`, `make test`, `make lint` pass.

## Why

- **BUG-001 (Major)**: silently writes garbage commit subjects. Any real LLM that typos a path or asks
  for a file outside the staged skeleton triggers it. FR-W3 requires non-staged reads be "ignored by the
  caller WITH A NOTE"; FR-W7 ("a response with no valid READ line is the message") does NOT cover a
  response that IS a READ line for a bad path. The forced-conclusion path already does the right thing
  (stripReadLines); only the natural `len(paths)==0` path is broken.

## What

**User-visible behavior**: `stagecoach --work-description` no longer commits a READ line as the subject;
non-staged READs get a note and the loop continues.

**Technical change (workdesc.go only):**
- 3 new unexported helpers (`containsReadVerb`, `readTargets`, `buildNonStagedReadAnswer`).
- Restructure the `len(paths)==0` branch into 3-way (staged-READ / non-staged-READ / no-READ), routing
  the non-staged case THROUGH the existing round-cap check.

### Success Criteria
- [ ] `containsReadVerb` uses the exact parseReadLines/stripReadLines verb tokenization
- [ ] `readTargets` returns raw normalized deduped targets (no skeleton filter)
- [ ] `buildNonStagedReadAnswer` emits FR-W3 notes; handles empty-targets edge
- [ ] `len(paths)==0` branch: non-staged READ → notes + loop (bounded); no-READ → FR-W7 message (unchanged)
- [ ] Staged-READ path + forced-conclusion path unchanged
- [ ] Existing tests pass; new helper + behavior tests pass
- [ ] `go build ./...`, `go test ./internal/generate/...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact buggy lines, the verb tokenization to match, the loop-restructure design (with the round-cap-routing rationale), the three helper implementations, the behavior-preservation proof, the test idiom, and the scope boundaries are all enumerated below (verified by reading source).

### Documentation & References

```yaml
- file: internal/generate/workdesc.go
  why: "THE file. RunWorkDescription loop (63-128); the buggy len(paths)==0 branch (99-102); parseReadLines (145, verb tokenization 158-163); stripReadLines (189, same verb tokenization 196-200); splitPaths (207) / normalizePath (215); buildReadAnswer (263, non-staged note wording 277)."
  pattern: "The verb tokenization (fields[0] uppercased + TrimLeft punctuation) is SHARED by parseReadLines and stripReadLines — containsReadVerb/readTargets MUST use it verbatim so all three agree on what a READ line is."
  gotcha: "The round-cap check (st.rounds >= st.N, line 106) sits AFTER the len(paths)==0 return. A fix that handles non-staged READs inside the len(paths)==0 branch and `continue`s would BYPASS the round cap ⇒ infinite loop. The fix MUST route the non-staged case THROUGH the existing budget check — restructure to a 3-way split, do NOT early-return/continue from the non-staged case."

- file: internal/generate/generate_workdesc_test.go
  why: "Test home (package generate — INTERNAL test ⇒ calls unexported helpers directly; NO export needed). Existing pattern: TestParseReadLines_* (Basic/CaseInsensitive/MultipleComma/NonStagedIgnored/NoReadLineIsMessage/Deduplicates at 84-140) + TestStripReadLines (142) — table-driven, direct calls."
  pattern: "Mirror TestParseReadLines_* for TestContainsReadVerb/TestReadTargets (table-driven, direct call). For the RunWorkDescription behavior test, mirror the existing RunWorkDescription test idiom (stubtest.Manifest with a scripted first response 'READ typo.go')."

- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/architecture/bugfix_workdesc.md
  why: "The BUG-001 root-cause trace + fix strategy (the two-case distinction + the round-cap-bounded continuation)."
  section: "BUG-001 (MAJOR): Non-staged READ silently becomes the commit subject"
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M1T1S1/research/verification_deltas.md
  why: "The verified anchors, the exact helper implementations, the loop-restructure design (with the round-cap-routing rationale), the behavior-preservation proof, and the scope boundaries. READ THIS before editing."
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  workdesc.go                       # THE file: +3 helpers, restructure len(paths)==0 branch (99-102)
  generate_workdesc_test.go         # +TestContainsReadVerb/TestReadTargets + behavior test (package generate)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (round-cap routing): the budget check (st.rounds >= st.N) is AFTER the len(paths)==0 return.
//   A fix that handles the non-staged-READ case inside len(paths)==0 and `continue`s BYPASSES the round
//   cap ⇒ infinite loop on a model that keeps emitting non-staged READs. Restructure to a 3-way split
//   so the non-staged case builds its `answer` and FALLS THROUGH to the budget check + render (do NOT
//   early-return or `continue` from the non-staged case).

// CRITICAL (verb tokenization): containsReadVerb/readTargets MUST use the exact same verb detection as
//   parseReadLines/stripReadLines: fields := strings.Fields(trimmed); verb := strings.ToUpper(
//   strings.TrimLeft(fields[0], " \t,.:;!?-*/`\"'")). Diverging (e.g. case-sensitive, or a different
//   punctuation set) would disagree on edge cases and mis-classify READ lines.

// GOTCHA (raw targets are ALL non-staged in this branch): parseReadLines returns STAGED paths only. So
//   if len(paths)==0, no staged target was requested ⇒ every readTargets() entry is non-staged. The
//   FR-W3 note "is not in the staged changes" is accurate for all of them (do NOT re-check the skeleton).

// GOTCHA (note wording differs from buildReadAnswer): buildReadAnswer's note (L277) is "is not in the
//   staged changes (or has no further diff)" — but THAT path is for a STAGED path with an empty diff,
//   not a genuinely-non-staged target. Use the cleaner "is not in the staged changes." for the new notes.

// GOTCHA (bare "READ" edge): a line "READ" with no path ⇒ containsReadVerb true, readTargets empty.
//   buildNonStagedReadAnswer handles it (emit a minimal generic note; the round cap bounds the loop).

// SCOPE: S1 is workdesc.go ONLY (+ its test). Do NOT touch buildReadAnswer/nextChunk (BUG-002/005/006),
//   the forced-conclusion path (already correct), parseReadLines/stripReadLines signatures, or
//   RunWorkDescription's signature.
```

## Implementation Blueprint

### Data models and structure

No struct/type/signature changes. Three new unexported helpers + a restructured branch. No new imports
(strings, fmt already imported in workdesc.go).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD containsReadVerb + readTargets + buildNonStagedReadAnswer to internal/generate/workdesc.go
  - PLACE: near parseReadLines/stripReadLines (group the READ-protocol helpers, ~L135-205).
  - containsReadVerb(response string) bool — line-level; ANY line whose verb == "READ" (exact
    parseReadLines tokenization). See research notes for the verbatim body.
  - readTargets(response string) []string — raw normalized deduped targets, NO skeleton filter (reuses
    splitPaths + normalizePath). See research notes for the verbatim body.
  - buildNonStagedReadAnswer(targets []string) string — FR-W3 note "<p> is not in the staged changes."
    per target; empty-targets ⇒ "(no staged file matches that read request)".
  - DOC COMMENT on each (containsReadVerb = the BUG-001 gate; readTargets = raw variant of parseReadLines;
    buildNonStagedReadAnswer = FR-W3 notes for the non-staged-READ loop continuation).
  - NO new import.
  - DEPENDENCIES: none (splitPaths/normalizePath already exist).

Task 2: RESTRUCTURE the len(paths)==0 branch in RunWorkDescription (workdesc.go:99-102)
  - REPLACE the single `if len(paths) == 0 { ParseOutput(out); return }` with a 3-way split:
        var answer string
        haveAnswer := false
        if len(paths) > 0 {
            // normal: buildReadAnswer below (after the budget check, as today)
        } else if containsReadVerb(out) {
            // BUG-001 fix: READ lines existed but all targets were non-staged (FR-W3). Build notes and
            // continue the loop — fall through to the budget check so the round cap bounds it.
            answer = buildNonStagedReadAnswer(readTargets(out))
            haveAnswer = true
        } else {
            // FR-W7: genuinely no READ line → the message (byte-identical to today's non-bug case).
            m, parseOK, _ := provider.ParseOutput(out, manifest)
            return m, parseOK, nil
        }
  - THEN the EXISTING budget check + st.rounds++ + render+execute, with:
        if !haveAnswer { answer = buildReadAnswer(ctx, deps.Git, cfg, deps.Excludes, paths, &st) }
    before the render. (The render/execute lines are UNCHANGED.)
  - The forced-conclusion path (st.rounds >= st.N) is UNCHANGED — it now also bounds the non-staged case.
  - UPDATE the code comment on the (former) len(paths)==0 branch to document the 3-way distinction
    (staged-READ / non-staged-READ-with-notes / genuine-no-READ-message), citing FR-W3 + FR-W7 + BUG-001.
  - DEPENDENCIES: Task 1.

Task 3: ADD tests to internal/generate/generate_workdesc_test.go
  - TestContainsReadVerb (table): "READ a.go"→true; "read a.go"→true (case-insensitive); "feat: x"→false;
    ""→false; "READ" (bare)→true; "READ a.go\nfeat: x"→true (any line); "  read  a.go"→true (punctuation/ws).
  - TestReadTargets (table): "READ a.go, b.go"→[a.go,b.go]; "READ `typo.go`"→[typo.go]; dedup; no-READ→[].
  - TestBuildNonStagedReadAnswer: ["typo.go"]→"typo.go is not in the staged changes."; []→generic note.
  - (Recommended) A RunWorkDescription behavior test: stubtest scripted first response "READ typo.go"
    against a skeleton where typo.go is NOT staged → assert the run does NOT return "READ typo.go" as the
    message (it either continues to a second scripted message, or the round cap forces conclusion with
    stripReadLines). Mirror the existing RunWorkDescription test idiom in this file.
  - DEPENDENCIES: Tasks 1-2.

Task 4: VERIFY — existing tests pass + the bug is fixed
  - RUN: go test ./internal/generate/... -v  (EXPECT green — existing TestParseReadLines_*/TestStripReadLines
    pass unchanged; new tests pass).
  - DEPENDENCIES: Tasks 1-3.
```

### Implementation Patterns & Key Details

```go
// PATTERN: containsReadVerb — exact parseReadLines verb tokenization (line-level)
fields := strings.Fields(trimmed)
verb := strings.ToUpper(strings.TrimLeft(fields[0], " \t,.:;!?-*/`\"'"))
if verb == "READ" { return true }

// PATTERN: the 3-way len(paths)==0 split (route non-staged READ THROUGH the round cap)
if len(paths) > 0 {
    // normal (buildReadAnswer after budget check)
} else if containsReadVerb(out) {
    answer = buildNonStagedReadAnswer(readTargets(out))   // FR-W3 notes; fall through to budget check
    haveAnswer = true
} else {
    m, parseOK, _ := provider.ParseOutput(out, manifest)   // FR-W7 genuine message
    return m, parseOK, nil
}
// ... existing budget check (st.rounds >= st.N ⇒ forced conclusion) ...
// ... st.rounds++ ...
if !haveAnswer { answer = buildReadAnswer(...) }
// ... render + execute (UNCHANGED) ...
```

### Integration Points

```yaml
NO struct / signature / config / public-API changes. Three unexported helpers + a restructured branch.

CODE:
  - internal/generate/workdesc.go — +containsReadVerb, +readTargets, +buildNonStagedReadAnswer; restructure len(paths)==0 (99-102)
TESTS:
  - internal/generate/generate_workdesc_test.go — +TestContainsReadVerb, +TestReadTargets, +TestBuildNonStagedReadAnswer (+ behavior test)

UNCHANGED: parseReadLines/stripReadLines/buildReadAnswer/nextChunk; the forced-conclusion path; RunWorkDescription's signature;
  the FR-W6 round-cap logic (it now also bounds the non-staged case — that's the point).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...
go vet ./...
gofmt -l internal/generate/
# Expected: empty.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation — behavior-preservation + bug-fix gate)

```bash
# Existing READ-protocol tests MUST pass UNCHANGED
go test ./internal/generate/ -run 'TestParseReadLines|TestStripReadLines' -v
# Expected: all pass (the helpers don't touch parseReadLines/stripReadLines).

# New helper tests
go test ./internal/generate/ -run 'TestContainsReadVerb|TestReadTargets|TestBuildNonStagedReadAnswer' -v
# Expected: all pass.

# Full generate package (RunWorkDescription behavior)
go test ./internal/generate/... -v

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# The BUG-001 end-to-end repro (the bug doc's reproduction):
#   git init; stage a one-line change to a.txt; stagecoach --provider stub --work-description 'refactor auth'
#   with the agent's first response = "READ typo.go".
# BEFORE the fix: commit subject == "READ typo.go". AFTER: the run notes typo.go as non-staged and
# continues (or the round cap forces a real message). The unit/behavior test in Task 3 is the
# deterministic within-scope proof; this manual repro is optional confirmation.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the 3 helpers exist + the branch is restructured
grep -n "func containsReadVerb\|func readTargets\|func buildNonStagedReadAnswer" internal/generate/workdesc.go
grep -n "containsReadVerb(out)\|buildNonStagedReadAnswer(readTargets" internal/generate/workdesc.go
# Expected: all present.

# Grep guard: the len(paths)==0 branch is NO LONGER a bare ParseOutput(out) return (the bug)
grep -n "len(paths) == 0" internal/generate/workdesc.go
# Expected: the branch now contains the 3-way split (containsReadVerb gate), not a bare ParseOutput(out).

# Scope-boundary guard: only workdesc.go + its test changed
git diff --name-only
# Expected: only internal/generate/workdesc.go + internal/generate/generate_workdesc_test.go.

# Behavior-preservation guard: parseReadLines/stripReadLines/buildReadAnswer UNCHANGED
git diff internal/generate/workdesc.go | grep -E '^[-+].*(func parseReadLines|func stripReadLines|func buildReadAnswer)'
# Expected: empty (those function bodies are untouched).

# Mutation guard (manual, then revert): prove the fix is real — temporarily revert the restructure
#   (put back `if len(paths)==0 { ParseOutput(out); return }`), confirm a "READ typo.go"-only response
#   test FAILS (returns the garbage subject), then re-apply the fix.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/generate/` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass

### Feature Validation
- [ ] `containsReadVerb`/`readTargets`/`buildNonStagedReadAnswer` correct (table tests)
- [ ] len(paths)==0 + non-staged READ → FR-W3 notes + loop continuation (not a garbage subject)
- [ ] len(paths)==0 + no READ → FR-W7 message return (byte-identical to today)
- [ ] Staged-READ path + forced-conclusion path unchanged
- [ ] Loop bounded (round cap fires for a model that keeps emitting non-staged READs)

### Scope-Boundary Validation
- [ ] buildReadAnswer/nextChunk UNCHANGED (BUG-002/005/006 are sibling subtasks)
- [ ] parseReadLines/stripReadLines signatures + behavior UNCHANGED
- [ ] RunWorkDescription signature UNCHANGED; forced-conclusion path UNCHANGED
- [ ] Only workdesc.go + generate_workdesc_test.go changed

### Code Quality
- [ ] Helpers co-located with parseReadLines/stripReadLines
- [ ] containsReadVerb uses the exact shared verb tokenization
- [ ] Code comment documents the 3-way distinction (FR-W3/FR-W7/BUG-001)

---

## Anti-Patterns to Avoid

- ❌ Don't handle the non-staged-READ case with a `continue` inside the len(paths)==0 branch — it bypasses the round-cap check (which sits after the branch) ⇒ infinite loop. Restructure to fall THROUGH the budget check.
- ❌ Don't use a different verb tokenization in containsReadVerb/readTargets than parseReadLines/stripReadLines — they must agree exactly on what a READ line is (same TrimLeft punctuation set, case-insensitive).
- ❌ Don't re-check the skeleton for readTargets — in the len(paths)==0 branch ALL readTargets are already non-staged (a staged target would have made len(paths)>0). The FR-W3 note is accurate as-is.
- ❌ Don't reuse buildReadAnswer's "(or has no further diff)" note wording — that's for a STAGED path with an empty diff, a different semantic. Use "is not in the staged changes."
- ❌ Don't change parseReadLines to return raw targets — that would break its existing contract (callers rely on it returning only staged paths) and the existing tests. Add a SEPARATE readTargets helper.
- ❌ Don't touch buildReadAnswer/nextChunk (BUG-002/005/006) or the forced-conclusion path (already correct).
- ❌ Don't export the helpers — the test file is `package generate` (internal); unexported is reachable.

---

## Confidence Score: 9/10

One-pass success is very high: the bug is precisely located, the verb tokenization to match is spelled
out verbatim, the three helper implementations are given, and the loop-restructure design is worked out
with the critical round-cap-routing rationale (the one real trap — a `continue` would unbound the loop).
The -1 is for the restructure touching the loop control flow (the `haveAnswer` flag + fall-through), which
is slightly more invasive than a pure additive fix — an implementer who gets the control flow wrong could
either re-introduce the bypass or break the normal path. Mitigated by the verbatim restructured body in
the research notes, the behavior-preservation proof (staged-READ + no-READ cases byte-identical), and the
existing test suite as the gate.