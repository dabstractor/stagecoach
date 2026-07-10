name: "P2.M1.T2.S1 — Add HasStagedChanges re-check at top of Decompose() with StagedNames-based error (FR-M1e, §9.14)"
description: >
  Add the FR-M1e defense-in-depth empty-index re-assertion at the TOP of Decompose() in
  internal/decompose/decompose.go, placed AFTER the mode-routing escape-hatch (`if deps.Config.Single ||
  deps.Config.Commits == 1 { return runSingleEscape(...) }`, current L143-145) and BEFORE the step-(2)
  RevParseHEAD/baseTree derivation (current L147-148) — so it fires before FreezeWorkingTree (L165),
  which calls AddAll and would otherwise silently fold any pre-existing staged content into T_start.
  The Deps struct (decompose/roles.go) already carries `Git git.Git` for the two calls. Concretely:
  (a) at current L146 (after the escape-hatch `}`, before the step-(2) comment), add:
  `hasStaged, err := deps.Git.HasStagedChanges(ctx)` → on err wrap ErrDecomposeFailed; on hasStaged==true
  call `names, _ := deps.Git.StagedNames(ctx)` (best-effort — error discarded intentionally) and return a
  PLAIN fmt.Errorf (NO sentinel wrap) naming the staged paths and offering `git reset` / `stagecoach
  --single`; (b) rewrite the Decompose doc-comment PRECONDITION block (current L121-122, which says
  "Decompose does NOT re-check this") to state that Decompose NOW re-asserts the empty-index precondition
  (FR-M1e, defense-in-depth, after the escape-hatch); (c) add unit tests for the re-check; (d) [Mode A]
  add a one-sentence FR-M1e defense-in-depth note to docs/how-it-works.md decompose "### Trigger" section.
  INPUT: `StagedNames()` (P2.M1.T1.S1 — the previous PRP) + the existing `HasStagedChanges()`. NO new
  imports (context/errors/fmt/strings already imported in decompose.go). Validates via `go build ./...`,
  `go test ./internal/decompose/... -run FRM1e -race -v`, `go test -race ./...`, `gofmt -l`, `go vet`.

---

## Goal

**Feature Goal**: A stale/buggy CLI trigger that routes to `Decompose()` with a NON-EMPTY index now fails
LOUDLY with a clear, actionable error NAMING the offending staged paths — instead of silently sweeping
them into `T_start` via `FreezeWorkingTree`'s internal `AddAll`. This is the FR-M1e defense-in-depth
re-assertion of the empty-index precondition that FR-M1 establishes at the trigger layer.

**Deliverable**: One edited function body in `internal/decompose/decompose.go` (`Decompose()`: the new
re-check block at L146 + the rewritten doc-comment PRECONDITION block at L121-122), one edited doc
(`docs/how-it-works.md`: one sentence in the decompose "### Trigger" section), and new unit tests in
`internal/decompose/decompose_test.go` (the FR-M1e error + the escape-hatch-bypasses regressions).

**Success Definition**:
- Calling `Decompose(ctx, deps)` with files staged AND in auto/forced mode (not `Single`, not
  `Commits==1`) returns a non-nil error whose message contains each staged path, the FR-M1e note, and the
  `git reset` / `stagecoach --single` remedies — and creates ZERO commits and leaves the index UNTOUCHED.
- Calling `Decompose` with `Single=true` or `Commits==1` still works WITH staged content (the escape-hatch
  runs `runSingleEscape` → `CommitStaged`, which handles a hand-staged index normally) — proves the check
  is placed AFTER the escape-hatch.
- All existing decompose tests still pass (no regression — none stage files before the non-escape call).
- `go build ./...`, `go test -race ./...`, `gofmt -l`, `go vet` all clean.

## User Persona (if applicable)

**Target User**: A Stagecoach end user who (via a bug or a stale state) ends up with a staged index AND
reaches the decompose path — OR a maintainer hardening the trigger boundary.
**Use Case**: Defense-in-depth. FR-M1 routes to decompose ONLY when nothing is staged; FR-M1e makes
`Decompose()` not TRUST that routing. If the trigger ever mis-routes, the user gets an immediate, named,
actionable error rather than a silently-corrupted multi-commit run that folded hand-staged content into
`T_start`.
**Pain Points Addressed**: Today `FreezeWorkingTree` calls `AddAll` first, so pre-existing staged content
is invisibly merged into `T_start` and committed across concepts — silent data corruption. FR-M1e turns
that into a loud, recoverable failure.

## Why

- **Closes the silent-corruption loophole**: `FreezeWorkingTree(baseTree)` (decompose.go:165) internally
  runs `AddAll` to capture the working-tree change set; if the index was already non-empty, that staged
  content is folded into `T_start` and committed as if it were part of the working-tree change set. The
  only correct input to decompose is an EMPTY index (FR-M1). Re-asserting it at entry (BEFORE the freeze)
  is the cheapest possible guard.
- **Defense-in-depth, not a new routing decision**: the CLI router already does the FR-M1
  `HasStagedChanges` routing upstream. FR-M1e does NOT change routing — it makes the library entry point
  not TRUST the caller, exactly like the stager HEAD-movement guard (§19 / Issue 2). A stale trigger now
  fails closed.
- **Actionable over generic**: by calling `StagedNames` (P2.M1.T1.S1) the error NAMES the offending paths
  and offers two concrete remedies (`git reset` to unstage + re-run, or `stagecoach --single` to commit
  the hand-staged index as one) — not a vague "something is staged".

## What

Add the FR-M1e re-check inside `Decompose()` (internal/decompose/decompose.go), placed AFTER the
mode-routing escape-hatch `if deps.Config.Single || deps.Config.Commits == 1 { ... }` block (current
L143-145) and BEFORE the step-(2) `RevParseHEAD`/baseTree derivation (current L147-148). The check calls
`deps.Git.HasStagedChanges(ctx)`; on error it wraps `ErrDecomposeFailed`; on `true` it calls
`deps.Git.StagedNames(ctx)` (error discarded — best-effort enrichment) and returns a plain `fmt.Errorf`
naming the paths. Update the `Decompose` doc-comment PRECONDITION block (current L121-122) to reflect the
new contract. Add unit tests. Update `docs/how-it-works.md` "### Trigger" with a one-sentence note.

### Success Criteria
- [ ] `Decompose()` contains the FR-M1e re-check AFTER the escape-hatch `if`/`}` and BEFORE
      `RevParseHEAD`, in that exact position (after L145, before L147).
- [ ] On `HasStagedChanges` error → returns `DecomposeResult{}, fmt.Errorf("%w: check staged changes: %w",
      ErrDecomposeFailed, err)` (sentinel-wrapped).
- [ ] On `hasStaged == true` → calls `names, _ := deps.Git.StagedNames(ctx)` and returns a PLAIN
      `fmt.Errorf` (NO `%w` wrap of `ErrDecomposeFailed`) whose message contains the count, the joined
      paths, the "defense-in-depth check (FR-M1e)" phrase, and both remedies (`git reset`,
      `stagecoach --single`).
- [ ] The `Decompose` doc-comment (current L121-122) no longer says "Decompose does NOT re-check this";
      it states Decompose NOW re-asserts the empty-index precondition (FR-M1e, defense-in-depth, after the
      escape-hatch so single/`--commits 1` is unaffected).
- [ ] `Decompose` with staged content + `Single=true` still commits normally (escape-hatch bypasses the
      check); same for `Commits==1`.
- [ ] `Decompose` with staged content + auto mode (Commits=0, Single=false) errors and creates ZERO
      commits; the index is left UNTOUCHED (the staged files remain staged — the check runs before any
      git mutation).
- [ ] New unit tests pass; `go test -race ./...` green; `gofmt -l` / `go vet` clean.
- [ ] `docs/how-it-works.md` "### Trigger" mentions the FR-M1e defense-in-depth re-check.
- [ ] ONLY `internal/decompose/decompose.go`, `internal/decompose/decompose_test.go`, and
      `docs/how-it-works.md` changed.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — exact line numbers for the insertion slot and the doc-comment, the verbatim error contract (both
branches), the verbatim doc-comment rewrite, the proven test helpers + setup recipes, the no-new-imports
fact, the dependency on P2.M1.T1.S1's `StagedNames`, and the verified Go validation commands.

### Documentation & References

```yaml
# MUST READ — the codebase-specific findings for THIS item (verified line numbers + error contract).
- docfile: plan/015_b461e4720495/P2M1T2S1/research/findings.md
  why: "§0 the exact insertion slot (after escape-hatch L145, before RevParseHEAD L147, before
        FreezeWorkingTree L165) + WHY that slot; §2 HasStagedChanges semantics; §3 StagedNames (from the
        previous PRP); §4 the FULL verbatim error contract (BOTH branches + the two intentional design
        notes on the discarded StagedNames error and the non-sentinel-wrapped staged-content error);
        §5 no-new-imports; §6 the doc-comment to rewrite; §7 test helpers; §9 validation commands."

# MUST READ — the function to edit (Decompose body) + the doc-comment to rewrite.
- file: internal/decompose/decompose.go
  why: "L141 `func Decompose(...)`; L121-122 the PRECONDITION doc-comment to REWRITE (currently says
        'Decompose does NOT re-check this'); L142-145 the escape-hatch `if deps.Config.Single ||
        deps.Config.Commits == 1 { return runSingleEscape(ctx, deps) }` — INSERT the FR-M1e check on the
        line immediately AFTER its closing `}` (L145) and BEFORE the `// (2) Derive isUnborn ...` comment
        (L147); L148 `preRunHEAD, isUnborn, err := deps.Git.RevParseHEAD(ctx)`; L165
        `tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)` (the freeze the check must precede).
        L47 `var ErrDecomposeFailed = errors.New(\"decompose: orchestrator failed\")` (the sentinel)."
  pattern: "Every early-return guard in Decompose follows `return DecomposeResult{}, fmt.Errorf(...)` —
            match that shape. The escape-hatch (L143-145) is the model for a clean pre-everything guard."
  gotcha: "Place the check AFTER the escape-hatch, NEVER before it — before would break legitimate
           `stagecoach --single` / `--commits 1` with a hand-staged index (runSingleEscape → CommitStaged
           handles staged content normally). Place it BEFORE FreezeWorkingTree (L165) — the freeze calls
           AddAll internally and would fold staged content into T_start."

# MUST READ — the previous PRP (the CONTRACT for StagedNames, the input primitive).
- docfile: plan/015_b461e4720495/P2M1T1S1/PRP.md
  why: "Defines `StagedNames(ctx context.Context) ([]string, error)` on the Git interface (after
        StagedFileCount at git.go:163) + its *gitRunner impl. THIS task CONSUMES it as
        `deps.Git.StagedNames(ctx)`. Treat it as implemented. If it has not landed yet (parallel
        execution), `deps.Git.StagedNames` will resolve once P2.M1.T1.S1 ships — the two are sequenced."

# MUST READ — the Deps struct (the Git handle lives here).
- file: internal/decompose/roles.go
  why: "`type Deps struct { Git git.Git; ... }` — the field the check calls through. No edit needed here;
        it already exposes `Git git.Git` (HasStagedChanges + StagedNames are methods on that interface)."

# MUST READ — the test file to extend (helpers + the escape-hatch tests to model the regressions on).
- file: internal/decompose/decompose_test.go
  why: "L139 `dcmDeps(t, repo, roles)` / L150 `dcmDepsWithConfig(t, repo, roles, cfg)` build Deps with
        `Git: git.New(repo)` (the REAL runner → HasStagedChanges + StagedNames carry automatically).
        L243 `TestDecompose_SingleEscape` + L1376 `TestDecompose_Commits1_Mode` are the EXACT templates
        for the escape-hatch-bypasses regressions (model the staged variants on them). dcmWriteFile /
        dcmStageFile / dcmInitRepo / dcmLogCount / dcmStatusPorcelain / config.Defaults() / errors.Is /
        strings.Contains are all the helpers you need."
  pattern: "Test func naming `TestDecompose_<Scenario>` (mirror the existing names, e.g.
            `TestDecompose_StagedIndex_FRM1e`). Setup: dcmInitRepo → dcmWriteFile → dcmStageFile (to make
            the index non-empty) → dcmDepsWithConfig(repo, roles, cfg with Commits=0,Single=false) →
            Decompose(ctx, deps) → assert non-nil error with substring checks; assert dcmLogCount==0 and
            the files still show as staged in dcmStatusPorcelain."
  gotcha: "Roles in Deps need SOMETHING (a stub manifest) to build even though the message/stager roles
           are never reached by the FR-M1e path — model on TestDecompose_SingleEscape's
           `dcmMessageManifest(t, bin, \"...\")`. Use `stubtest.Build(t)` for the stub binary."

# CONTEXT — HasStagedChanges impl (the semantics behind the bool you read).
- file: internal/git/git.go
  why: "L133 (interface) + L1079 (impl) `HasStagedChanges(ctx) (bool, error)` — exit-code-inverted
        (`git diff --cached --quiet`: exit 0 = nothing staged, exit 1 = staged, other = error). For FR-M1e
        you just read the (bool, error) — no exit-code juggling. L163 `StagedFileCount` (the sibling whose
        path-returning twin, StagedNames, is P2.M1.T1.S1)."

# CONTEXT — PRD provenance (read-only).
- docfile: plan/015_b461e4720495/prd_snapshot.md
  section: "§9.14 FR-M1 (trigger: decomposition activates iff HasStagedChanges is false) — FR-M1e is the
            re-assertion that names offending staged paths. §13.6.1 (the trigger model: 'iff nothing
            staged and the working tree has changes')."
  why: "Establishes WHY the re-check exists and why the error must be actionable (name the paths)."

# CONTEXT — the docs file to update (Mode A).
- file: docs/how-it-works.md
  why: "L47 `## Multi-commit decomposition` → L51 `### Trigger` is where the FR-M1e defense-in-depth note
        belongs (one sentence, after the existing trigger paragraph at L53). L123 `### Safety` is an
        optional cross-reference home."
```

### Current Codebase tree (relevant slice)

```bash
# EDIT targets:
internal/decompose/decompose.go          # EDIT — Decompose() FR-M1e re-check (L146) + doc-comment (L121-122)
internal/decompose/decompose_test.go     # EDIT — add TestDecompose_StagedIndex_FRM1e + escape-hatch-bypass regressions
docs/how-it-works.md                     # EDIT — one FR-M1e sentence in "### Trigger"

# READ-ONLY references:
internal/decompose/roles.go              # Deps.Git git.Git (the handle — no edit)
internal/git/git.go                      # HasStagedChanges (L133/L1079) + StagedNames (P2.M1.T1.S1, after L163)
plan/015_b461e4720495/P2M1T1S1/PRP.md    # the CONTRACT for StagedNames (the input primitive)
Makefile                                 # test / coverage-gate / lint targets
```

### Desired Codebase tree with files to be added/edited

```bash
internal/decompose/decompose.go          # EDIT — +FR-M1e re-check block in Decompose() (after L145) + doc-comment rewrite (L121-122)
internal/decompose/decompose_test.go     # EDIT — +TestDecompose_StagedIndex_FRM1e + 2 escape-hatch-bypass regressions
docs/how-it-works.md                     # EDIT — +1 sentence (FR-M1e defense-in-depth) in "### Trigger"
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (placement — AFTER the escape-hatch, BEFORE the freeze): the check MUST go after the
// `if deps.Config.Single || deps.Config.Commits == 1 { return runSingleEscape(ctx, deps) }` block (L143-145)
// and before `RevParseHEAD` (L148). Putting it BEFORE the escape-hatch breaks legitimate `stagecoach
// --single` with a hand-staged index (runSingleEscape → CommitStaged handles staged content normally).
// Putting it AFTER FreezeWorkingTree (L165) is useless — the freeze already folded staged content into
// T_start. Line 146 (after the escape-hatch `}`, before the step-(2) comment) is the ONLY correct slot.

// CRITICAL (two DISTINCT error shapes — follow the contract VERBATIM):
//  (a) HasStagedChanges ERRORS → wrap the sentinel:  fmt.Errorf("%w: check staged changes: %w", ErrDecomposeFailed, err)
//  (b) hasStaged == TRUE       → PLAIN fmt.Errorf, NO `%w` wrap of ErrDecomposeFailed. The message IS the
//      remedy. Do NOT "helpfully" wrap it with the sentinel — it is a distinct user-facing actionable
//      category, not an orchestrator infra failure. Wrapping would muddy the clear `git reset` / `--single`
//      guidance.

// CRITICAL (the `names, _ := deps.Git.StagedNames(ctx)` discards the error INTENTIONALLY): we already
// proved via HasStagedChanges that something IS staged; StagedNames is best-effort enrichment to NAME the
// paths. If StagedNames errors, names is nil → the message prints "0 file(s) are staged:" with the
// HasStagedChanges truth still correct. Do NOT promote the StagedNames error to a hard failure.

// GOTCHA (no new imports): decompose.go (L29-38) already imports context, errors, fmt, strings. Both
// HasStagedChanges and StagedNames are methods on the injected deps.Git git.Git (git already imported).
// Do NOT touch the import block.

// GOTCHA (strings.Join on nil/empty is safe): when nothing is staged we never reach the Join (hasStaged
// is false). If StagedNames returned nil (it won't, since HasStagedChanges was true, but defensively),
// strings.Join(nil, ", ") == "" and len(nil) == 0 — the message degrades gracefully to "0 file(s)".

// GOTCHA (the error string contains backticks + an em-dash): the staged-content error message includes
// `git reset` and `stagecoach --single` in backticks and an em-dash (—). These are fine inside a Go
// double-quoted string literal and are valid UTF-8 under gofmt. Keep the message on one fmt.Errorf with
// string concatenation (+) across lines for readability — gofmt will align it.

// GOTCHA (dependency on P2.M1.T1.S1): deps.Git.StagedNames resolves only once the previous PRP ships the
// StagedNames method on the Git interface. This task is sequenced AFTER it; if building in isolation
// before it lands, the call will not compile. In the integrated plan both land together.
```

## Implementation Blueprint

### Data models and structure

None — this is a control-flow guard returning an error. No structs, no schemas. The "data" is the two
error strings (verbatim from the contract in §4 of findings) and the doc-comment rewrite.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/decompose/decompose.go — add the FR-M1e re-check in Decompose() (after L145)
  - LOCATE the escape-hatch block at L142-145:
        // (1) Mode routing: single ESCAPE-HATCH (planner bypassed) → v1 path.
        if deps.Config.Single || deps.Config.Commits == 1 {
            return runSingleEscape(ctx, deps)
        }
  - INSERT immediately AFTER its closing `}` (L145) and BEFORE the `// (2) Derive isUnborn ...` comment
    (L147) a new labeled guard block (becomes step "(1b)"):
        // (1b) FR-M1e defense-in-depth: re-assert the empty-index precondition the trigger (FR-M1) owns.
        //      A stale/buggy trigger that routes here with a non-empty index would otherwise have its
        //      staged content silently folded into T_start by FreezeWorkingTree's internal AddAll (step 3).
        //      Placed AFTER the escape-hatch (single/--commits 1 still uses the single primitive, which
        //      handles a hand-staged index normally) and BEFORE FreezeWorkingTree. Fail loudly, naming the
        //      offending paths.
        hasStaged, err := deps.Git.HasStagedChanges(ctx)
        if err != nil {
            return DecomposeResult{}, fmt.Errorf("%w: check staged changes: %w", ErrDecomposeFailed, err)
        }
        if hasStaged {
            names, _ := deps.Git.StagedNames(ctx) // best-effort: HasStagedChanges already proved something is staged
            return DecomposeResult{}, fmt.Errorf(
                "decompose requires an empty index, but %d file(s) are staged: %s. "+
                    "This is a defense-in-depth check (FR-M1e) — the trigger should have routed to the "+
                    "single-commit path. Run `git reset` to unstage, or `stagecoach --single` for the "+
                    "one-commit behavior",
                len(names), strings.Join(names, ", "))
        }
  - NAMING: `hasStaged`, `names` (local, idiomatic Go).
  - DEPENDENCIES: `deps.Git.HasStagedChanges` (existing), `deps.Git.StagedNames` (P2.M1.T1.S1),
    `ErrDecomposeFailed` (decompose.go:47), `fmt` + `strings` (already imported).
  - PRESERVE: the escape-hatch block ABOVE it and the step-(2) RevParseHEAD block BELOW it UNCHANGED.

Task 2: EDIT internal/decompose/decompose.go — rewrite the Decompose doc-comment PRECONDITION block (L121-122)
  - LOCATE the PRECONDITION comment block at L121-122 (inside the Decompose doc-comment, ~L115-140):
        // PRECONDITION (FR-M1, owned by the CLI router — P4.M1.T1.S1): the caller routed here because NOTHING is
        // staged (HasStagedChanges false) AND the working tree has changes. Decompose does NOT re-check this; it
        // assumes correct routing.
  - REPLACE with (the FR-M1e contract — Decompose NOW re-asserts the empty-index precondition):
        // PRECONDITION (FR-M1, owned by the CLI router — P4.M1.T1.S1): the caller routed here because NOTHING is
        // staged (HasStagedChanges false) AND the working tree has changes. FR-M1e (defense-in-depth):
        // Decompose re-asserts the empty-index precondition at entry — AFTER the single/`--commits 1`
        // escape-hatch (which handles a hand-staged index normally) and BEFORE FreezeWorkingTree. A stale or
        // buggy trigger that routes here with a non-empty index fails loudly, naming the staged paths, instead
        // of silently folding them into T_start.
  - PRESERVE: the surrounding MODE ROUTING / FR-M1b FREEZE / Error contract comment paragraphs UNCHANGED.

Task 3: EDIT docs/how-it-works.md — add the FR-M1e defense-in-depth sentence to "### Trigger"
  - LOCATE the decompose "### Trigger" paragraph (L51-53), which ends: "...If something is already
    staged, the single-commit path runs unchanged. `--dry-run` also forces the single-commit preview
    (decompose commits, so dry-run honors the single preview)."
  - APPEND a new short paragraph immediately after it:
        **Defense-in-depth (FR-M1e).** Decompose re-asserts the empty-index precondition at its entry: if
        a stale or buggy trigger ever routes to it with a non-empty index, it fails loudly — naming the
        offending staged paths and pointing to `git reset` (unstage, then re-run) or `stagecoach --single`
        (commit the hand-staged index as one) — instead of silently sweeping them into the start-of-run
        freeze. The single-commit escape hatch (`--single`, `--commits 1`) is checked first and is
        unaffected, since it handles a hand-staged index normally.
  - NAMING/STYLE: match the surrounding markdown (bold lead-in, backticks for flags/commands).
  - PLACEMENT: end of "### Trigger", before "### The four roles". Do NOT edit "### Safety" (its existing
    FR-M1c/FR-M1d bullets already cover the freeze surfaces; the re-check is a trigger-layer concern).

Task 4: EDIT internal/decompose/decompose_test.go — add the FR-M1e tests
  - FILE: append to the existing `internal/decompose/decompose_test.go` (`package decompose`).
  - IMPLEMENT three tests (model on TestDecompose_SingleEscape L243 + TestDecompose_Commits1_Mode L1376):
    * TestDecompose_StagedIndex_FRM1e (MAIN): born repo, write+STAGE 2 files (dcmWriteFile + dcmStageFile
      for "a.txt" and "b.go"), cfg = config.Defaults() (Commits=0 auto, Single=false), roles with a stub
      message manifest, deps := dcmDepsWithConfig(t, repo, roles, cfg). Call Decompose(ctx, deps). Assert:
      err != nil; err.Error() contains "a.txt", "b.go", "2 file(s) are staged", "defense-in-depth check
      (FR-M1e)", "git reset", "stagecoach --single"; result.Commits is empty; dcmLogCount(t,repo)==0 (NO
      commit created); dcmStatusPorcelain shows BOTH files STILL staged (index untouched — they have the
      `A ` / `M ` staged marker). Use errors.Is(err, ErrDecomposeFailed) == FALSE to assert the
      staged-content error is NOT sentinel-wrapped (the design choice).
    * TestDecompose_StagedIndex_SingleBypasses (REGRESSION): same setup but cfg.Single = true. Assert
      Decompose SUCCEEDS (err == nil), 1 commit created, no FR-M1e error — proves the check is AFTER the
      escape-hatch. (The stub message manifest provides the commit message; CommitStaged handles the
      staged content normally.)
    * TestDecompose_StagedIndex_Commits1Bypasses (REGRESSION): same setup but cfg.Commits = 1 (Single
      false). Assert Decompose SUCCEEDS, 1 commit — proves `--commits 1` also bypasses the check.
  - FOLLOW pattern: dcmInitRepo / dcmWriteFile / dcmStageFile / dcmDepsWithConfig / dcmLogCount /
    dcmStatusPorcelain / config.Defaults() / stubtest.Build(t) / dcmMessageManifest / errors.Is /
    strings.Contains (all defined in decompose_test.go).
  - NAMING: TestDecompose_StagedIndex_FRM1e, TestDecompose_StagedIndex_SingleBypasses,
    TestDecompose_StagedIndex_Commits1Bypasses.
  - COVERAGE: the staged-error path (main), both escape-hatch bypasses, index-untouched invariant,
    no-commit invariant, non-sentinel-wrapped error.
  - PLACEMENT: append at end of decompose_test.go (after the last existing TestDecompose_* func).
  - OPTIONAL (lower value): a 4th test for the HasStagedChanges-error branch via a pre-cancelled context
    (ctx, cancel := context.WithCancel(...); cancel()) — assert errors.Is(err, ErrDecomposeFailed) == TRUE
    (the sentinel-WRAPPED branch). Include only if cheap; the main value is the staged-content path.

Task 5: VALIDATE — build, targeted tests, full regression, format, vet, scope
  - go build ./...
  - go test ./internal/decompose/... -run FRM1e -race -v   # also catches the bypass tests via -run Staged
    (use -run 'Staged|FRM1e' or just -run TestDecompose_Staged to cover all three)
  - go test ./internal/decompose/... -race                  # full decompose package (escape-hatch + happy-path regressions)
  - go test -race ./...                                      # whole-repo regression
  - gofmt -l internal/decompose/decompose.go internal/decompose/decompose_test.go docs/how-it-works.md  # wait — gofmt only on .go
  - gofmt -l internal/decompose/decompose.go internal/decompose/decompose_test.go   # must print nothing
  - go vet ./internal/decompose/...
  - git status --porcelain   # ONLY the 3 files
```

### Implementation Patterns & Key Details

```go
// PATTERN (the FR-M1e re-check — verbatim from the contract; two distinct error shapes):
//   AFTER the escape-hatch `if ... { return runSingleEscape(ctx, deps) }`, BEFORE RevParseHEAD.
hasStaged, err := deps.Git.HasStagedChanges(ctx)
if err != nil {
    // (a) infra error → sentinel-wrapped (consistent with the other Decompose guards).
    return DecomposeResult{}, fmt.Errorf("%w: check staged changes: %w", ErrDecomposeFailed, err)
}
if hasStaged {
    // (b) staged content → PLAIN error (NO %w sentinel wrap). names, _ is INTENTIONAL best-effort.
    names, _ := deps.Git.StagedNames(ctx)
    return DecomposeResult{}, fmt.Errorf(
        "decompose requires an empty index, but %d file(s) are staged: %s. "+
            "This is a defense-in-depth check (FR-M1e) — the trigger should have routed to the "+
            "single-commit path. Run `git reset` to unstage, or `stagecoach --single` for the "+
            "one-commit behavior",
        len(names), strings.Join(names, ", "))
}

// PATTERN (test — the index-untouched + no-commit invariants):
//   The check runs BEFORE any git mutation (RevParseHEAD / FreezeWorkingTree / AddAll), so on the error
//   path the repo state is byte-for-byte the pre-call state: zero new commits, staged files still staged.
if dcmLogCount(t, repo) != 0 { t.Fatalf("expected 0 commits, got %d", dcmLogCount(t, repo)) }
status := dcmStatusPorcelain(t, repo)
if !strings.Contains(status, "a.txt") || !strings.Contains(status, "b.go") {
    t.Fatalf("index mutated; status = %q", status)
}
// The staged-content error is NOT sentinel-wrapped (design choice) — assert it:
if errors.Is(err, ErrDecomposeFailed) { t.Errorf("staged-content error must NOT wrap ErrDecomposeFailed") }
```

### Integration Points

```yaml
DECOMPOSE() (internal/decompose/decompose.go):
  - add to: the Decompose() body, after the escape-hatch if/} (L145), before the step-(2) RevParseHEAD
    comment (L147). Becomes step "(1b)".
  - pattern: "hasStaged, err := deps.Git.HasStagedChanges(ctx)" → sentinel-wrapped err / plain staged error.
DOC-COMMENT (internal/decompose/decompose.go):
  - rewrite: the PRECONDITION block at L121-122 (Decompose doc-comment) — drop "does NOT re-check", add
    the FR-M1e re-assertion statement.
DOCS (docs/how-it-works.md):
  - add to: "### Trigger" (L51-53), one new paragraph after the existing trigger paragraph.
TESTS (internal/decompose/decompose_test.go):
  - add: TestDecompose_StagedIndex_FRM1e + 2 escape-hatch-bypass regressions (+ optional ctx-cancel test).
NO config / routes / migrations / env vars / new imports — pure control-flow guard + docs.
NO CLI change — the router keeps its own FR-M1 HasStagedChanges routing upstream; FR-M1e is the
  defense-in-depth INSIDE the library entry point.
```

## Validation Loop

### Level 1: Build & format (Immediate Feedback)

```bash
# Compiles the whole module — also PROVES deps.Git.StagedNames resolves (depends on P2.M1.T1.S1 landed).
go build ./...
# Expected: no output (success). If it errors on deps.Git.StagedNames (undeclared), P2.M1.T1.S1 has not
# shipped yet — this task is sequenced after it; in the integrated plan both land together.

# Formatting check — must print NOTHING.
gofmt -l internal/decompose/decompose.go internal/decompose/decompose_test.go
# Expected: empty output. If a file is listed, run `gofmt -w` on it and re-check.
```

### Level 2: Unit Tests (the new FR-M1e guard)

```bash
# Targeted: the new staged-index tests (main + both bypasses), race-enabled.
go test ./internal/decompose/... -run TestDecompose_Staged -race -v
# Expected: all three pass — StagedIndex_FRM1e (error names paths + no commit + index untouched +
#           non-sentinel-wrapped), SingleBypasses (escape-hatch succeeds with staged content),
# Commits1Bypasses (same for --commits 1).

# Full decompose package (regression — escape-hatch, happy-path, one-file shortcut, arbiter all intact).
go test ./internal/decompose/... -race
# Expected: PASS. Confirms the new guard at Decompose's entry didn't disturb any existing path (none of
# them stage files before the non-escape call).
```

### Level 3: Whole-repo regression + static checks

```bash
# Whole repo — catches any downstream compile/test break.
go test -race ./...
# Expected: PASS. (internal/cmd uses Decompose via the CLI; the re-check only fires on a non-empty index,
# which the CLI never passes to Decompose — so the CLI/e2e paths are unaffected.)

# Static checks.
go vet ./internal/decompose/...
# Expected: clean.

# Coverage gate (PRD §20.3: ≥85% on internal/{git,provider,generate,config} — decompose is not gated, but
# the new guard should keep internal/decompose coverage healthy; run for hygiene).
make coverage-gate
# Expected: PASS (decompose is not in the gated set, but no gate should regress).
```

### Level 4: Scope guard + docs sanity

```bash
# ONLY the three intended files changed.
git status --porcelain
# Expected: M internal/decompose/decompose.go  AND  M internal/decompose/decompose_test.go  AND
#           M docs/how-it-works.md  (nothing else).

# Guard: no out-of-scope / forbidden files touched.
git status --porcelain | grep -E 'internal/git/git\.go|PRD\.md|plan/.*tasks\.json|prd_snapshot|delta_prd|roles\.go' && echo "FAIL: forbidden file touched" || echo "OK: scope clean"
# Expected: "OK: scope clean". internal/git/git.go (StagedNames owner = P2.M1.T1.S1) and roles.go (Deps)
# MUST be untouched; PRD/plan/tasks/snapshot MUST be untouched.

# Docs sanity: the FR-M1e sentence is present and markdown renders (the repo has .markdownlint.json).
grep -n "FR-M1e" docs/how-it-works.md
# Expected: at least one hit in the "### Trigger" section.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` succeeds (proves the `deps.Git.StagedNames` call resolves — P2.M1.T1.S1 dependency)
- [ ] `go test ./internal/decompose/... -run TestDecompose_Staged -race -v` passes (main + 2 bypasses)
- [ ] `go test ./internal/decompose/... -race` passes (full package regression — escape-hatch/happy-path intact)
- [ ] `go test -race ./...` passes (whole-repo regression)
- [ ] `gofmt -l internal/decompose/decompose.go internal/decompose/decompose_test.go` prints nothing
- [ ] `go vet ./internal/decompose/...` clean

### Feature Validation
- [ ] `Decompose()` with staged content + auto mode returns a non-nil error naming each staged path, the
      FR-M1e phrase, and both remedies (`git reset`, `stagecoach --single`)
- [ ] That error is NOT sentinel-wrapped (`errors.Is(err, ErrDecomposeFailed)` is false) — the design choice
- [ ] That error path creates ZERO commits and leaves the index byte-for-byte untouched (files still staged)
- [ ] `Decompose()` with staged content + `Single=true` still commits normally (escape-hatch bypasses)
- [ ] `Decompose()` with staged content + `Commits==1` still commits normally (escape-hatch bypasses)
- [ ] The check is placed AFTER the escape-hatch `if`/`}` and BEFORE `RevParseHEAD`/`FreezeWorkingTree`

### Documentation Validation
- [ ] The `Decompose` doc-comment (L121-122) no longer says "does NOT re-check"; states the FR-M1e
      re-assertion (defense-in-depth, after the escape-hatch)
- [ ] `docs/how-it-works.md` "### Trigger" has the one-sentence FR-M1e defense-in-depth note

### Scope-Boundary Validation
- [ ] `git status --porcelain` shows ONLY `internal/decompose/decompose.go` +
      `internal/decompose/decompose_test.go` + `docs/how-it-works.md`
- [ ] `internal/git/git.go` UNCHANGED (StagedNames/HasStagedChanges owner = P2.M1.T1.S1 / existing)
- [ ] `internal/decompose/roles.go` UNCHANGED (Deps.Git already exposes the interface — no edit needed)
- [ ] NO edit to PRD.md, plan/**/tasks.json, prd_snapshot.md, delta_prd.md, the CLI, or any other source

---

## Anti-Patterns to Avoid

- ❌ Don't place the check BEFORE the escape-hatch — it would break legitimate `stagecoach --single` /
  `--commits 1` with a hand-staged index (`runSingleEscape` → `CommitStaged` handles staged content
  normally). The check MUST be after the `if deps.Config.Single || deps.Config.Commits == 1 { ... }` block.
- ❌ Don't place the check AFTER `FreezeWorkingTree` (L165) — the freeze calls `AddAll` internally and
  would already have folded the staged content into `T_start`. The check must precede the freeze (L146).
- ❌ Don't wrap the staged-content error with `ErrDecomposeFailed` (`%w`) — the contract specifies a PLAIN
  `fmt.Errorf` for the `hasStaged==true` branch. Only the `HasStagedChanges`-ERRORS branch wraps the
  sentinel. Two distinct error shapes, by design.
- ❌ Don't promote the `StagedNames` error to a hard failure — `names, _ :=` discards it INTENTIONALLY.
  `HasStagedChanges` already proved something is staged; `StagedNames` is best-effort path-naming. On its
  error, `names` is nil → the message degrades gracefully to "0 file(s)".
- ❌ Don't add `--quiet` or change `HasStagedChanges`'s semantics — just read the `(bool, error)` it
  returns (it structurally encodes the exit-1-means-staged inversion). And don't re-implement
  `StagedNames` here — it ships in P2.M1.T1.S1; just call `deps.Git.StagedNames(ctx)`.
- ❌ Don't touch the import block — context/errors/fmt/strings are already imported in decompose.go, and
  git is imported for the Deps.Git interface.
- ❌ Don't change the CLI router — FR-M1e is a defense-in-depth INSIDE the library entry point; the router
  keeps its own upstream FR-M1 `HasStagedChanges` routing. Two layers, by design.
- ❌ Don't stage files in the SETUP of any non-escape existing test — none do (verified); only the NEW
  FR-M1e test stages files before the non-escape call (that is its whole point).
