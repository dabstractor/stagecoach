name: "P2.M2.T1.S1 — Unit + e2e tests for FR-M1e re-assertion and FR-M1c error improvements (§9.14, §20.5)"
description: |
  A TESTS-ONLY subtask that COMPLETES (does not duplicate) the freeze-hardening test coverage
  requested by the item. Research established that the sibling implementation tasks already wrote
  most of the requested tests TDD-style: `TestDecompose_StagedIndex_FRM1e` (P2.M1.T2.S1, Complete)
  covers item (a) in-process against a real git repo, and P2.M1.T3.S1 (contract; working-tree)
  already updated `TestVerifyFreezeSubset_PathViolation`/`_ContentViolation` with the new phrasing
  AND concept-title assertions. This subtask fills the three GENUINE gaps: (D1) a DEDICATED
  `TestVerifyFreezeSubset_ConceptTitleWiring` proving the conceptTitle is faithfully threaded via
  %q (existing tests use the generic "test-concept"); (D2) gap-fill the verbatim
  "requires an empty index" substring the item contract names but the existing FR-M1e test omits;
  (D3) a stub-reachable e2e routing-boundary scenario `S8_StagedIndex_NoDecompose` — the §20.5
  subprocess regression net for the boundary that makes FR-M1e defense-in-depth (a subprocess
  scenario that TRIGGERS FR-M1e is architecturally impossible: the CLI router only calls
  Decompose() inside `if !hasStaged`, default_action.go:110-125). NO production-code change. NO
  new files except the edits to the two test files. Validates via `go build ./...`,
  `go test ./internal/decompose/... -race`, `go test -tags e2e ./internal/e2e/...`, `gofmt -l`,
  `go vet`.

---

## Goal

**Feature Goal**: Freeze-hardening (FR-M1e empty-index re-assertion + amended FR-M1c freeze-violation
error messages) has COMPREHENSIVE, non-redundant test coverage at both the in-process unit level and
the subprocess e2e level — with every gap the item names either already covered (and confirmed) or
newly filled, and with zero duplication of the tests the sibling tasks already wrote.

**Deliverable**: Three edits across two test files:
1. `internal/decompose/stager_test.go` — ADD `TestVerifyFreezeSubset_ConceptTitleWiring` (D1).
2. `internal/decompose/decompose_test.go` — ADD `"requires an empty index"` to the existing
   `TestDecompose_StagedIndex_FRM1e` want-slice at L2694 (D2).
3. `internal/e2e/scenarios_test.go` — ADD the `S8_StagedIndex_NoDecompose` subtest inside
   `TestE2EScenarios` (D3).
No production-code change. No new source files.

**Success Definition**:
- `TestVerifyFreezeSubset_ConceptTitleWiring` passes with `-race`, asserting a DISTINGUISHABLE title
  (e.g. `"feat: add user auth endpoint"`) appears VERBATIM in both the path- and content-violation
  errors (proving faithful `%q` threading, not a hardcoded literal).
- `TestDecompose_StagedIndex_FRM1e` additionally asserts the substring `"requires an empty index"`
  (the item's named contract substring), still passing.
- `S8_StagedIndex_NoDecompose` passes under `-tags e2e`: a staged index routes to the SINGLE-COMMIT
  path (exit 0, commitCount==2, staged file committed) and does NOT trigger decompose (stderr has no
  `"requires an empty index"`).
- `go build ./...`, `go test -race ./internal/decompose/...`, `go test -tags e2e ./internal/e2e/...`,
  `go test -race ./...`, `gofmt -l`, `go vet` all clean.

## User Persona (if applicable)

**Target User**: The Stagecoach maintainer (and CI). These are regression nets, not user-facing.
**Use Case**: Guard against regressions in (a) the FR-M1e defense-in-depth re-assertion and (b) the
amended FR-M1c freeze-violation error quality (concept title + plain-language remedy), and (c) the
CLI routing boundary that makes FR-M1e defense-in-depth. Per §20.5, "every bug found in the wild
becomes a scenario here."
**Pain Points Addressed**: The freeze-hardening work (P2.M1) shipped with strong in-process coverage
but (D1) no dedicated proof that conceptTitle is faithfully threaded (existing tests reuse the
generic "test-concept" literal — a regression to a hardcoded string would pass them), (D2) the
existing FR-M1e test omits the item's namesake substring, and (D3) no subprocess §20.5 scenario
pins the staged-index routing boundary.

## Why

- **Close the freeze-hardening test milestone (P2.M2.T1):** P2.M1.T2.S1 and P2.M1.T3.S1 implemented
  FR-M1e + the amended FR-M1c and wrote substantial tests alongside. P2.M2.T1.S1 is the dedicated
  test subtask; this PRP directs it to the genuine gaps rather than re-creating existing tests.
- **D1 — prove the title wiring, not just the message:** the amended FR-M1c error's whole point is
  naming the concept BY TITLE. Existing tests pass `"test-concept"` and assert `"test-concept"` —
  which would still pass if the production code hardcoded `"test-concept"` instead of threading
  `conceptTitle`. A dedicated test with a unique, distinguishable title closes that loophole.
- **D2 — match the item's contract verbatim:** the item explicitly requires asserting the error
  contains `"requires an empty index"`; the existing test asserts adjacent substrings but not this
  one. A one-line addition makes the test faithful to the contract.
- **D3 — the §20.5 subprocess angle for the freeze boundary:** the e2e harness exists precisely to
  catch CLI-routing + config-load + real-repo bugs in-process tests cannot reach
  (harness_test.go doc). A subprocess scenario pinning "staged index → single commit, never
  decompose" is the regression net for the boundary that necessitates FR-M1e.

## What

Three edits to two test files. NO production-code change.

### Already covered — DO NOT duplicate (confirmed at HEAD)
- **(a) FR-M1e re-assertion in-process:** `TestDecompose_StagedIndex_FRM1e`
  (`internal/decompose/decompose_test.go:2672`) + `TestDecompose_StagedIndex_SingleBypasses`
  (:2730) already: create a temp repo, stage files, call `Decompose()` directly, assert a non-nil
  error naming the staged paths + remedies, assert `!errors.Is(err, ErrDecomposeFailed)`, assert
  zero commits, assert the index is byte-for-byte untouched. → Item (a) is satisfied; do NOT add a
  second `TestDecompose_NonEmptyIndex`.
- **(b) FR-M1c existing-test updates:** P2.M1.T3.S1 already updated
  `TestVerifyFreezeSubset_PathViolation`/`_ContentViolation` (stager_test.go:370/406/452/483 call
  sites carry the new `conceptTitle` arg; the substring assertions use the new "frozen working-tree
  snapshot" phrasing; AND `strings.Contains(err.Error(), "test-concept")` title assertions are
  present at :418-420 / :467-469). → The first sentence of item (b) is done; do NOT re-edit those
  assertions.

### Success Criteria
- [ ] **D1:** `TestVerifyFreezeSubset_ConceptTitleWiring` added to `stager_test.go`; passes `-race`;
      asserts a DISTINGUISHABLE title appears in BOTH path- and content-violation errors.
- [ ] **D2:** `TestDecompose_StagedIndex_FRM1e` want-slice (decompose_test.go:2694) includes
      `"requires an empty index"`; test still passes.
- [ ] **D3:** `S8_StagedIndex_NoDecompose` subtest added inside `TestE2EScenarios`
      (e2e/scenarios_test.go); passes under `-tags e2e`; asserts staged → single-commit, never
      decompose.
- [ ] NO production-code file modified; NO existing test weakened or duplicated.
- [ ] `go build ./...`, `go test -race ./internal/decompose/...`, `go test -tags e2e ./internal/e2e/...`,
      `gofmt -l`, `go vet ./internal/decompose/...` all clean.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — the exact helpers to call, the exact code under test (with line numbers +
verbatim messages), the architectural reason a subprocess FR-M1e-trigger is impossible, the
`-tags e2e` build-tag gotcha, and the precise edit locations are all below.

### Documentation & References

```yaml
# MUST READ — the complete verified analysis (signatures, line numbers, the routing-impossibility
# proof, the e2e build-tag gotcha, the "already done" table, the test patterns).
- docfile: plan/015_b461e4720495/P2M2T1S1/research/findings.md
  why: "§0 the already-done table (do NOT duplicate); §1 the FR-M1e code under test; §2 the FR-M1c
        verifyFreezeSubset signature + both verbatim messages; §3 the PROOF a subprocess FR-M1e
        scenario is impossible (router only calls Decompose in !hasStaged); §4 the test helper
        inventories; §5 the -tags e2e gotcha; §6 the exact D2 want-slice; §7 no sentinel exists."

# CONTRACT — the previous PRP (assume implemented exactly as specified). It defines the
# conceptTitle signature + verbatim messages + the existing-test updates that D1 builds on.
- docfile: plan/015_b461e4720495/P2M1T3S1/PRP.md
  why: "Defines the verifyFreezeSubset(... conceptTitle string ...) signature, both verbatim error
        messages, the 4 stager_test call-site updates, and the concept-title assertions D1 extends.
        D1's new test calls verifyFreezeSubset with the NEW signature."

# D1 — the function under test + the test pattern to mirror.
- file: internal/decompose/stager.go
  why: "L160 signature (conceptTitle between i and treeI); L174-177 message (A) PATH; L196-199
        message (B) CONTENT; L60 ErrFreezeViolation sentinel. D1 triggers both branches and asserts
        the distinguishable title renders via %q."
  pattern: "verifyFreezeSubset returns fmt.Errorf(\"%w in concept %d (%q): ...\", ErrFreezeViolation,
            i, conceptTitle, join). The (%q) renders conceptTitle with Go-quoting — a title with
            spaces/colons is safe and searchable verbatim."
- file: internal/decompose/stager_test.go
  why: "MIRROR TestVerifyFreezeSubset_PathViolation (:370-) and _ContentViolation (:452-) for D1's
        repo setup + call. Reuse the stg* helpers (stgInitRepo/stgCommitRaw/stgWriteFile/stgRunGit/
        stgGitOut). The only difference: pass a DISTINGUISHABLE title and assert THAT title (not
        'test-concept')."
  gotcha: "D1 must NOT reuse the literal 'test-concept' — its whole point is a distinguishable title
           that would FAIL if conceptTitle were hardcoded. Use e.g. 'feat: add user auth endpoint'."

# D2 — the existing FR-M1e test + the code under test.
- file: internal/decompose/decompose.go
  why: "L150-170 the FR-M1e re-check; the message literally starts 'decompose requires an empty
        index, but ...'. Confirms the verbatim substring 'requires an empty index' is present in the
        rendered error (so D2's assertion is satisfiable)."
- file: internal/decompose/decompose_test.go
  why: "L2694 the want-slice to gap-fill (add 'requires an empty index'). L2672 the test function
        (TestDecompose_StagedIndex_FRM1e) — do NOT duplicate it; only edit its want-slice."

# D3 — the e2e harness + the routing reality.
- file: internal/cmd/default_action.go
  why: "L110-125 the router: runDecompose is STRICTLY inside `if !hasStaged`. Proves a staged index
        ALWAYS routes to single-commit (never Decompose/FR-M1e) via the binary — so D3 asserts the
        routing boundary, not an FR-M1e trigger."
  pattern: "hasStaged==true → falls through past the decompose block to the single-commit
            generate+commit path (L167+)."
- file: internal/e2e/harness_test.go
  why: "The helper inventory for D3: buildStagecoach, buildStub, newRepo, seedCommit, writeFile,
        stageFile, runStagecoach, writeStubConfig, stubEnv, commitCount, diffTreeNames, headSHA,
        contains. The doc comment states the harness catches CLI-routing + config-load bugs
        in-process tests cannot reach."
  gotcha: "ALL e2e files carry //go:build e2e. go test ./internal/e2e/... WITHOUT -tags e2e matches
           NO packages. D3's validation MUST use -tags e2e."
- file: internal/e2e/scenarios_test.go
  why: "MIRROR the Sx_ExcludedFileStillCommitted subtest (end of file) for D3's shape: seed a repo,
        stage a file, runStagecoach with the stub provider, assert exit 0 + commitCount + committed
        file. D3 ADDS the negative assertion: stderr has no 'requires an empty index'."

# PRD provenance (read-only).
- docfile: plan/015_b461e4720495/prd_snapshot.md
  section: "§9.14 FR-M1c/FR-M1e (freeze enforcement + empty-index re-assertion); §20.1 layer 3
            (stub decompose suite) + §20.5 (every bug found in the wild becomes a scenario)."
```

### Current Codebase tree (relevant slice)

```bash
# EDIT targets (TEST files only):
internal/decompose/stager_test.go       # D1: ADD TestVerifyFreezeSubset_ConceptTitleWiring
internal/decompose/decompose_test.go    # D2: gap-fill want-slice @ L2694
internal/e2e/scenarios_test.go          # D3: ADD S8_StagedIndex_NoDecompose subtest

# READ-ONLY references:
internal/decompose/stager.go            # verifyFreezeSubset (conceptTitle signature + messages)
internal/decompose/decompose.go         # Decompose() FR-M1e re-check (L150-170)
internal/cmd/default_action.go          # router: runDecompose only in !hasStaged (L110-125)
internal/e2e/harness_test.go            # e2e helper inventory + //go:build e2e
plan/015_b461e4720495/P2M1T3S1/PRP.md   # CONTRACT (conceptTitle signature/messages)
plan/015_b461e4720495/P2M2T1S1/research/findings.md  # full analysis
```

### Desired Codebase tree with files to be added/edited

```bash
internal/decompose/stager_test.go       # EDIT — +1 test func (TestVerifyFreezeSubset_ConceptTitleWiring)
internal/decompose/decompose_test.go    # EDIT — +1 string in a want-slice (L2694)
internal/e2e/scenarios_test.go          # EDIT — +1 subtest (S8_StagedIndex_NoDecompose)
# (no new files; no production-code change)
```

### Known Gotchas of our codebase & Library Quirks

```text
# CRITICAL (the e2e build tag — D3 validation): internal/e2e/*_test.go carry //go:build e2e.
#   `go test ./internal/e2e/...`        → "matched no packages" (runs NOTHING).
#   `go test -tags e2e ./internal/e2e/...` → runs the suite. The item's stated validation command
#   omits -tags e2e and is WRONG for the e2e half. There is no Makefile e2e target / CI -tags wiring;
#   e2e is manual. Always use -tags e2e for D3.

# CRITICAL (a subprocess FR-M1e-trigger scenario is IMPOSSIBLE): the CLI router (default_action.go:
#   110-125) calls runDecompose→Decompose ONLY inside `if !hasStaged`. A staged index ALWAYS routes
#   to single-commit. FR-M1e (inside Decompose) is pure defense-in-depth, reachable ONLY by calling
#   Decompose() directly in-process — which TestDecompose_StagedIndex_FRM1e already does. So D3 is a
#   ROUTING-BOUNDARY scenario (staged → single, never decompose), NOT an FR-M1e trigger. Do NOT
#   attempt to trigger FR-M1e via the binary; it cannot be done and the test would be wrong.

# GOTCHA (D1 must use a DISTINGUISHABLE title, not "test-concept"): the existing PathViolation /
#   ContentViolation tests pass "test-concept" and assert "test-concept". If production code
#   regressed to hardcode that literal, those tests would still pass. D1's value is a UNIQUE title
#   (e.g. "feat: add user auth endpoint") that proves conceptTitle is actually threaded via %q.

# GOTCHA (D2 is a one-line SLICE edit, not a new test): add "requires an empty index" to the EXISTING
#   want-slice at decompose_test.go:2694. Do NOT write a second FR-M1e test — TestDecompose_StagedIndex
#   _FRM1e already exists (P2.M1.T2.S1) and covers the behavior thoroughly.

# GOTCHA (no sentinel for the FR-M1e error): the staged-index error is a bare fmt.Errorf string,
#   NOT %w-wrapped (deliberate — distinct actionable category). grep confirms no ErrNonEmptyIndex.
#   D2 asserts the SUBSTRING; do not invent/assert a sentinel. The existing test also asserts
#   !errors.Is(err, ErrDecomposeFailed) — leave that intact.

# GOTCHA (verifyFreezeSubset is UNEXPORTED): D1's test lives in internal/decompose (package decompose)
#   — same package as stager_test.go, so the unexported function is callable. Do not try to test it
#   from e2e (different package; e2e can only drive the binary).

# GOTCHA (the %w-SPACE render): ErrFreezeViolation.Error() == "decompose: freeze violation"; the
#   messages use "%w in concept..." so the render is "decompose: freeze violation in concept 0
#   (\"feat: add user auth endpoint\"): ...". When asserting the title, search for the bare title
#   substring (the (%q) quotes it, so search for `feat: add user auth endpoint` — Go's %q quotes are
#   around it but a substring Contains on the unquoted title still matches).
```

## Implementation Blueprint

### Data models and structure
None — no production data. The "data" is three test edits.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1 (D2 — gap-fill the existing FR-M1e test): EDIT internal/decompose/decompose_test.go
  - LOCATE the want-slice in TestDecompose_StagedIndex_FRM1e (L2694):
        for _, want := range []string{"a.txt", "b.go", "2 file(s) are staged", "defense-in-depth check (FR-M1e)", "git reset", "stagecoach --single"} {
  - ADD "requires an empty index" to the slice (the item's namesake substring; present in the
        rendered message at decompose.go:163 but currently unasserted):
        for _, want := range []string{"a.txt", "b.go", "requires an empty index", "2 file(s) are staged", "defense-in-depth check (FR-M1e)", "git reset", "stagecoach --single"} {
  - DO NOT touch any other assertion in the test (the !errors.Is(err, ErrDecomposeFailed), zero-
    commits, and index-untouched checks stay). DO NOT add a second FR-M1e test.
  - NAMING/PLACEMENT: in-place slice edit only.

Task 2 (D1 — dedicated conceptTitle-wiring test): EDIT internal/decompose/stager_test.go
  - ADD a new test function, placed immediately AFTER TestVerifyFreezeSubset_ContentViolation (which
    ends ~L470) and BEFORE TestVerifyFreezeSubset_EmptyStaging (L483) — keep the freeze-subset tests
    contiguous. Suggested name: TestVerifyFreezeSubset_ConceptTitleWiring.
  - MIRROR the repo setup of TestVerifyFreezeSubset_PathViolation (L370-): t.TempDir, stgInitRepo,
    stgCommitRaw "initial", stgWriteFile a.txt+bbb... actually a.txt="aaa\n" + b.txt="bbb\n",
    g := git.New(repo), ctx, baseTree = stgGitOut("rev-parse","HEAD^{tree}"),
    tStart = g.FreezeWorkingTree(ctx, baseTree), tStartPaths = g.DiffTreeNames(ctx, baseTree, tStart).
  - Use a DISTINGUISHABLE title constant, e.g.:
        const title = "feat: add user auth endpoint"
  - SUBTEST 1 (path branch): write a sentinel AFTER the freeze (stgWriteFile sentinel="concurrent\n"),
    stgRunGit "add" sentinel, treeI = g.WriteTree(ctx); call
        err := verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, 0, title, treeI)
    assert err != nil; errors.Is(err, ErrFreezeViolation); AND the title wiring:
        if !strings.Contains(err.Error(), title) { t.Errorf("path-violation error missing concept title %q; got: %s", title, err.Error()) }
        if !strings.Contains(err.Error(), "in concept 0") { t.Errorf(...) }   // index wiring
        if !strings.Contains(err.Error(), "sentinel.txt") { t.Errorf(...) }   // path still named
  - SUBTEST 2 (content branch): RESET the index (stgRunGit "reset"); modify a.txt to different
    content (stgWriteFile a.txt="modified\n"), stgRunGit "add" "a.txt", treeI = g.WriteTree(ctx);
    call verifyFreezeSubset(... 0, title, treeI); assert err != nil, errors.Is(err, ErrFreezeViolation),
    strings.Contains(err.Error(), title), strings.Contains(err.Error(), "in concept 0"),
    strings.Contains(err.Error(), "a.txt").
  - Use t.Run("path_violation", ...) / t.Run("content_violation", ...) subtests OR two asserts in one
    function — match the file's prevailing style (the file uses standalone funcs, but a single func
    with two staged scenarios is acceptable; keep it readable).
  - IMPORTS: strings + errors are already imported (stager_test.go L3-16). No new imports.
  - GOTCHA: reset the index between the two branches (git reset) so the content branch's treeI is
    not contaminated by the path branch's sentinel. Use stgRunGit(t, repo, "reset").

Task 3 (D3 — e2e routing-boundary scenario): EDIT internal/e2e/scenarios_test.go
  - ADD a new subtest inside TestE2EScenarios, placed after S7 (or after Sx_ExcludedFileStillCommitted):
        t.Run("S8_StagedIndex_NoDecompose", func(t *testing.T) { ... })
  - BODY (mirror Sx_ExcludedFileStillCommitted's stub-reachable shape):
        cfg := writeStubConfig(t, stub, "")
        repo := newRepo(t)
        seedCommit(t, repo, "readme.md", "init")
        writeFile(t, repo, "feat.txt", "feat\n")
        stageFile(t, repo, "feat.txt")              // a NON-EMPTY index
        env := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: add feat"})
        res := runStagecoach(t, bin, repo, cfg, env, "--provider", "stub")
        if res.ExitCode != 0 { t.Fatalf("exit code = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr) }
        if n := commitCount(t, repo); n != 2 { t.Fatalf("commit count = %d, want 2 (seed + 1 single commit, NOT decompose)", n) }
        names := diffTreeNames(t, repo, headSHA(t, repo))
        if !contains(names, "feat.txt") { t.Errorf("commit files = %v; want feat.txt (single-commit path committed the staged file)", names) }
        // The routing boundary: a staged index must NOT reach decompose / FR-M1e.
        if strings.Contains(res.Stderr, "requires an empty index") {
            t.Errorf("stderr triggered FR-M1e (decompose reached with a staged index — routing bug):\n%s", res.Stderr)
        }
  - This is stub-reachable (no skipIfNotReal). `bin` and `stub` are already in scope at the top of
    TestE2EScenarios (bin := buildStagecoach(t); stub := buildStub(t)).
  - NAMING: "S8_StagedIndex_NoDecompose" (follows the S<n>_<Name> convention).
  - GOTCHA: validate with -tags e2e (see Known Gotchas). This subtest asserts the ROUTING BOUNDARY;
    it cannot (and must not) trigger FR-M1e — see the impossibility proof in findings §3.

Task 4: VALIDATE
  - go build ./...
  - go test ./internal/decompose/... -run 'StagedIndex|FreezeSubset|StagerFreezeViolation|ConceptTitle' -race -v
  - go test -race ./internal/decompose/...
  - go test -tags e2e ./internal/e2e/... -run 'S8|E2EScenarios' -v
  - go test -race ./...
  - gofmt -l internal/decompose/stager_test.go internal/decompose/decompose_test.go internal/e2e/scenarios_test.go   # must print nothing
  - go vet ./internal/decompose/... ./internal/e2e/...
  - git status --porcelain   # ONLY the 3 test files
```

### Implementation Patterns & Key Details

```go
// PATTERN (D1 — distinguishable title proves faithful %q threading; mirrors stager_test.go:370):
func TestVerifyFreezeSubset_ConceptTitleWiring(t *testing.T) {
	const title = "feat: add user auth endpoint" // DISTINGUISHABLE — would FAIL if conceptTitle were hardcoded
	repo := t.TempDir()
	stgInitRepo(t, repo)
	stgCommitRaw(t, repo, "initial")
	stgWriteFile(t, repo, "a.txt", "aaa\n")
	stgWriteFile(t, repo, "b.txt", "bbb\n")
	g := git.New(repo)
	ctx := context.Background()
	baseTree := stgGitOut(t, repo, "rev-parse", "HEAD^{tree}")
	tStart, err := g.FreezeWorkingTree(ctx, baseTree)
	if err != nil { t.Fatalf("FreezeWorkingTree: %v", err) }
	tStartPaths, err := g.DiffTreeNames(ctx, baseTree, tStart)
	if err != nil { t.Fatalf("DiffTreeNames: %v", err) }
	deps := Deps{Git: g}

	// (1) PATH branch: a post-freeze sentinel not in T_start.
	stgWriteFile(t, repo, "sentinel.txt", "concurrent\n")
	stgRunGit(t, repo, "add", "sentinel.txt")
	treeI, err := g.WriteTree(ctx)
	if err != nil { t.Fatalf("WriteTree: %v", err) }
	perr := verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, 0, title, treeI)
	if perr == nil { t.Fatal("expected path-violation error, got nil") }
	if !errors.Is(perr, ErrFreezeViolation) { t.Fatalf("expected ErrFreezeViolation, got %v", perr) }
	for _, want := range []string{title, "in concept 0", "sentinel.txt", "staged paths not in the frozen working-tree snapshot"} {
		if !strings.Contains(perr.Error(), want) {
			t.Errorf("path-violation error missing %q; got: %s", want, perr.Error())
		}
	}

	// (2) CONTENT branch: reset, then modify a.txt to content not in T_start.
	stgRunGit(t, repo, "reset") // clear the sentinel from the index
	stgWriteFile(t, repo, "a.txt", "modified\n")
	stgRunGit(t, repo, "add", "a.txt")
	treeI2, err := g.WriteTree(ctx)
	if err != nil { t.Fatalf("WriteTree: %v", err) }
	cerr := verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, 0, title, treeI2)
	if cerr == nil { t.Fatal("expected content-violation error, got nil") }
	if !errors.Is(cerr, ErrFreezeViolation) { t.Fatalf("expected ErrFreezeViolation, got %v", cerr) }
	for _, want := range []string{title, "in concept 0", "a.txt", "staged content differs from the frozen working-tree snapshot"} {
		if !strings.Contains(cerr.Error(), want) {
			t.Errorf("content-violation error missing %q; got: %s", want, cerr.Error())
		}
	}
}

// PATTERN (D2 — one-line slice gap-fill at decompose_test.go:2694):
//   before: []string{"a.txt", "b.go", "2 file(s) are staged", "defense-in-depth check (FR-M1e)", "git reset", "stagecoach --single"}
//   after:  []string{"a.txt", "b.go", "requires an empty index", "2 file(s) are staged", "defense-in-depth check (FR-M1e)", "git reset", "stagecoach --single"}

// PATTERN (D3 — e2e routing-boundary subtest; mirrors Sx_ExcludedFileStillCommitted):
t.Run("S8_StagedIndex_NoDecompose", func(t *testing.T) {
	cfg := writeStubConfig(t, stub, "")
	repo := newRepo(t)
	seedCommit(t, repo, "readme.md", "init")
	writeFile(t, repo, "feat.txt", "feat\n")
	stageFile(t, repo, "feat.txt") // non-empty index → router MUST route to single-commit, not decompose
	env := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: add feat"})
	res := runStagecoach(t, bin, repo, cfg, env, "--provider", "stub")
	if res.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
	}
	if n := commitCount(t, repo); n != 2 {
		t.Fatalf("commit count = %d, want 2 (seed + 1 single commit; a staged index must not decompose)", n)
	}
	names := diffTreeNames(t, repo, headSHA(t, repo))
	if !contains(names, "feat.txt") {
		t.Errorf("commit files = %v; want feat.txt committed via the single-commit path", names)
	}
	if strings.Contains(res.Stderr, "requires an empty index") {
		t.Errorf("stderr hit FR-M1e — decompose was reached with a staged index (routing bug):\n%s", res.Stderr)
	}
})
```

### Integration Points

```yaml
TEST FILES (the only edits):
  - internal/decompose/stager_test.go: ADD TestVerifyFreezeSubset_ConceptTitleWiring (in-package;
    calls unexported verifyFreezeSubset with the conceptTitle signature from P2.M1.T3.S1).
  - internal/decompose/decompose_test.go: EDIT the want-slice @ L2694 (+ "requires an empty index").
  - internal/e2e/scenarios_test.go: ADD the S8_StagedIndex_NoDecompose subtest inside TestE2EScenarios.
PRODUCTION CODE: NONE. Do not edit stager.go, decompose.go, default_action.go, or any non-test file.
CONFIG/ROUTES/DB: NONE.
NEW SENTINELS: NONE (do not invent ErrNonEmptyIndex; the FR-M1e error is a bare fmt.Errorf string).
```

## Validation Loop

### Level 1: Build & format (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach
go build ./...   # proves the test files compile (conceptTitle signature from P2.M1.T3.S1 is in place)
# Expected: no output.

gofmt -l internal/decompose/stager_test.go internal/decompose/decompose_test.go internal/e2e/scenarios_test.go
# Expected: empty. If a file is listed, run `gofmt -w` on it and re-check.
```

### Level 2: Unit tests (the decompose package — D1 + D2)

```bash
cd /home/dustin/projects/stagecoach
# Targeted: the new conceptTitle test + the gap-filled FR-M1e test + the existing freeze tests (regression).
go test ./internal/decompose/... -run 'StagedIndex|FreezeSubset|StagerFreezeViolation|ConceptTitle' -race -v
# Expected: all PASS, including the NEW TestVerifyFreezeSubset_ConceptTitleWiring and the gap-filled
#           TestDecompose_StagedIndex_FRM1e (now asserting "requires an empty index").

# Full decompose package regression (every other decompose test untouched).
go test -race ./internal/decompose/...
# Expected: PASS.
```

### Level 3: e2e (D3 — REQUIRES the -tags e2e build tag)

```bash
cd /home/dustin/projects/stagecoach
# CRITICAL: internal/e2e is //go:build e2e. Without -tags e2e this matches NO packages.
go test -tags e2e ./internal/e2e/... -run 'E2EScenarios/S8' -v
# Expected: S8_StagedIndex_NoDecompose PASSES (exit 0, commitCount 2, feat.txt committed, no FR-M1e).

# Full e2e suite (stub-reachable scenarios; stager-dependent ones skip without STAGECOACH_RUN_REAL=1).
go test -tags e2e ./internal/e2e/...
# Expected: PASS (skips are OK for the real-only scenarios).
```

### Level 4: Whole-repo regression + static checks + scope

```bash
cd /home/dustin/projects/stagecoach
go test -race ./...                 # whole-repo regression (decompose changes can't break other packages)
# Expected: PASS.

go vet ./internal/decompose/... ./internal/e2e/...
# Expected: clean.

# Scope guard: ONLY the 3 test files changed.
git status --porcelain
# Expected: M internal/decompose/stager_test.go AND M internal/decompose/decompose_test.go AND
#           M internal/e2e/scenarios_test.go (nothing else — NO production code).
git status --porcelain | grep -vE 'internal/decompose/stager_test\.go|internal/decompose/decompose_test\.go|internal/e2e/scenarios_test\.go' | grep -E '\.go$' && echo "FAIL: out-of-scope .go file touched" || echo "OK: scope clean"
# Expected: "OK: scope clean".
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` succeeds
- [ ] `go test ./internal/decompose/... -run 'StagedIndex|FreezeSubset|StagerFreezeViolation|ConceptTitle' -race -v` passes (incl. the NEW + gap-filled tests)
- [ ] `go test -race ./internal/decompose/...` passes (full package regression)
- [ ] `go test -tags e2e ./internal/e2e/... -run 'E2EScenarios/S8' -v` passes (the `-tags e2e` is REQUIRED)
- [ ] `go test -race ./...` passes (whole-repo)
- [ ] `gofmt -l <the 3 test files>` prints nothing
- [ ] `go vet ./internal/decompose/... ./internal/e2e/...` clean

### Feature Validation
- [ ] D1: `TestVerifyFreezeSubset_ConceptTitleWiring` asserts a DISTINGUISHABLE title in BOTH path- and content-violation errors (proves faithful %q threading)
- [ ] D2: `TestDecompose_StagedIndex_FRM1e` now asserts the verbatim `"requires an empty index"` substring (item contract)
- [ ] D3: `S8_StagedIndex_NoDecompose` asserts staged → single-commit (commitCount 2, feat.txt committed), never decompose (no "requires an empty index" in stderr)
- [ ] FR-M1e + amended FR-M1c have comprehensive, non-redundant coverage (in-process for the error paths; subprocess for the routing boundary)

### Code Quality Validation
- [ ] D1 mirrors the existing TestVerifyFreezeSubset_* repo-setup pattern (stg* helpers, real git)
- [ ] D3 mirrors the existing Sx_* stub-reachable subtest shape (writeStubConfig/stubEnv/runStagecoach)
- [ ] No new imports required (strings + errors already present in both test packages)
- [ ] No existing test weakened, duplicated, or deleted

### Scope-Boundary Validation
- [ ] `git status --porcelain` shows ONLY the 3 test files
- [ ] NO production-code file touched (stager.go, decompose.go, default_action.go, etc. UNCHANGED)
- [ ] NO edit to PRD.md, plan/**/tasks.json, prd_snapshot.md, delta_prd.md, or any non-test source
- [ ] NO new sentinel invented (ErrNonEmptyIndex does not exist and must not be created)

---

## Anti-Patterns to Avoid

- ❌ Don't add a SECOND in-process FR-M1e test (`TestDecompose_NonEmptyIndex`) —
  `TestDecompose_StagedIndex_FRM1e` (P2.M1.T2.S1) already covers item (a) thoroughly. D2 is a
  one-line substring gap-fill of that EXISTING test, not a new test.
- ❌ Don't re-edit the `TestVerifyFreezeSubset_PathViolation`/`_ContentViolation` assertions —
  P2.M1.T3.S1 already updated them (new phrasing + concept-title checks). D1 is a SEPARATE dedicated
  test; touching the existing ones risks conflict with the in-flight P2.M1.T3.S1 work.
- ❌ Don't reuse the literal `"test-concept"` in D1 — its entire value is a DISTINGUISHABLE title
  that fails if conceptTitle is hardcoded. Use something like `"feat: add user auth endpoint"`.
- ❌ Don't try to trigger FR-M1e via the e2e binary — it is architecturally IMPOSSIBLE (the router
  only calls Decompose inside `if !hasStaged`). D3 asserts the ROUTING BOUNDARY (staged → single),
  not an FR-M1e trigger. An e2e test that "stages files and expects the FR-M1e error" would be wrong.
- ❌ Don't run e2e without `-tags e2e` — `go test ./internal/e2e/...` matches NO packages and silently
  runs nothing. The item's stated validation command omits the tag; always use `-tags e2e` for D3.
- ❌ Don't invent an `ErrNonEmptyIndex` sentinel — the FR-M1e error is a bare `fmt.Errorf` string by
  design (distinct actionable category). Assert the SUBSTRING (D2), not a sentinel.
- ❌ Don't forget to `git reset` between D1's two branches — the path branch's staged sentinel would
  contaminate the content branch's treeI and produce a path-violation instead of a content-violation.
- ❌ Don't edit production code to make a test pass — this is a TESTS-ONLY subtask. If a test fails
  because the production message differs, RE-READ the P2.M1.T3.S1 contract / the code under test and
  fix the TEST assertion, not the production code.

---

## Confidence Score

**One-pass success likelihood: 9/10.** The deliverables are small, surgical test edits against a
GREEN baseline (`go test ./internal/decompose/...` passes at HEAD with P2.M1.T3.S1's working-tree
changes). The two risks that keep it from 10/10: (1) D1's content branch needs an index `git reset`
between branches or the sentinel contaminates treeI (called out explicitly); (2) D3 depends on the
exact stub-config routing behavior (staged + auto-stage → single-commit), which is well-established
by the existing Sx_ExcludedFileStillCommitted scenario but should be confirmed by running the test.
The hardest insight — that a subprocess FR-M1e trigger is impossible and D3 must be a routing-
boundary scenario instead — is documented with the routing proof (findings §3) so the implementer
does not chase an impossible test. The `-tags e2e` gotcha is called out in three places.
