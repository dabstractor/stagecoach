# P2.M2.T1.S1 — Research Findings (freeze-hardening test coverage)

HEAD during research: `3602551` + uncommitted working-tree changes from P2.M1.T3.S1
(stager.go/decompose.go/stager_test.go/decompose_test.go modified; `go test` GREEN).

## 0. The decisive situational finding — most of the item is ALREADY DONE by siblings

The item description was authored at task-breakdown time, BEFORE the implementation subtasks
landed. Two sibling tasks already wrote the bulk of the requested tests (TDD-style, alongside
their implementation). P2.M2.T1.S1's real job is to FILL THE GAPS, not re-create existing tests.

| item | requested | status | owner |
|------|-----------|--------|-------|
| (a) TestDecompose_NonEmptyIndex FR-M1e | temp repo, stage file, call Decompose() directly, assert "requires an empty index" + staged path | **EXISTS** as `TestDecompose_StagedIndex_FRM1e` (decompose_test.go:2672) + `TestDecompose_StagedIndex_SingleBypasses` (:2730). GAP: asserts "2 file(s) are staged" etc. but NOT the verbatim "requires an empty index" substring. | P2.M1.T2.S1 (Complete) |
| (b) update PathViolation/ContentViolation to new format + add conceptTitle test | new error format asserted, concept title present | **DONE** for the existing tests: P2.M1.T3.S1 added `strings.Contains(err.Error(), "test-concept")` to both (stager_test.go:418-420, 467-469) + new-phrasing substrings. GAP: a DEDICATED test with a distinguishable title (existing ones use the generic "test-concept"). | P2.M1.T3.S1 (Implementing/working-tree) |
| (c) e2e scenario for the defense-in-depth path | seed repo, stage files, attempt decompose, verify clear error | **IN-PROCESS EXISTS** (TestDecompose_StagedIndex_FRM1e). SUBPROCESS IMPOSSIBLE — see §3. GAP: a stub-reachable e2e routing-boundary scenario. | this task |

CONCLUSION: the non-duplicative deliverables are D1 (dedicated conceptTitle test), D2 (verbatim
"requires an empty index" gap-fill), D3 (e2e routing-boundary scenario). Detailed below.

## 1. The FR-M1e re-check (the code under test for item a)

`internal/decompose/decompose.go:150-170` (Decompose entry, AFTER single/`--commits 1` escape-hatch,
BEFORE FreezeWorkingTree):
```go
hasStaged, err := deps.Git.HasStagedChanges(ctx)
if err != nil { return DecomposeResult{}, fmt.Errorf("%w: check staged changes: %w", ErrDecomposeFailed, err) }
if hasStaged {
    names, _ := deps.Git.StagedNames(ctx) // best-effort
    return DecomposeResult{}, fmt.Errorf(
        "decompose requires an empty index, but %d file(s) are staged: %s. "+
            "This is a defense-in-depth check (FR-M1e) — the trigger should have routed to the "+
            "single-commit path. Run `git reset` to unstage, or `stagecoach --single` for the "+
            "one-commit behavior",
        len(names), strings.Join(names, ", "))
}
```
NOTE: this error is NOT wrapped in ErrDecomposeFailed (a deliberate design choice — distinct
user-facing actionable category). The existing test asserts `!errors.Is(err, ErrDecomposeFailed)`.

## 2. The FR-M1c verifyFreezeSubset (the code under test for item b)

`internal/decompose/stager.go` — P2.M1.T3.S1's working-tree state:
- L60: `var ErrFreezeViolation = errors.New("decompose: freeze violation")` (sentinel; .Error() IS the prefix)
- L160: signature `(ctx, deps Deps, baseTree, tStart string, tStartPaths []string, i int, conceptTitle string, treeI string) error`
- L174-177 (A) PATH: `fmt.Errorf("%w in concept %d (%q): staged paths not in the frozen working-tree snapshot: %s. This indicates concurrent working-tree changes were picked up by the stager. Aborting to protect the freeze boundary.", ErrFreezeViolation, i, conceptTitle, strings.Join(extra, ", "))`
- L196-199 (B) CONTENT: `fmt.Errorf("%w in concept %d (%q): staged content differs from the frozen working-tree snapshot for: %s. This indicates ...", ErrFreezeViolation, i, conceptTitle, strings.Join(mismatch, ", "))`

The `%w ` (SPACE) renders "decompose: freeze violation in concept 0 (...)". conceptTitle via `%q`.

## 3. WHY a subprocess e2e FR-M1e-trigger scenario is IMPOSSIBLE (item c routing reality)

`internal/cmd/default_action.go:110-125` (the CLI router):
```go
hasStaged, err := g.HasStagedChanges(ctx)
...
if !hasStaged {
    if shouldDecompose(cfg, flagDryRun, flagNoAutoStage) {
        ...
        return runDecompose(...)   // ← Decompose() reached ONLY here
    }
    ...auto-stage-all / nothing-staged paths...
}
// hasStaged==true  →  falls through to the SINGLE-COMMIT generate+commit path
```
`runDecompose` → `decompose.Decompose` is STRICTLY inside `if !hasStaged`. There is NO CLI path
(not `--commits N`, not any flag) that reaches `Decompose()` with a staged index. Therefore:
- FR-M1e (the staged-index re-check INSIDE Decompose) is PURE defense-in-depth against a future
  buggy router. It is structurally unreachable through the compiled binary today.
- The ONLY way to test it is to call `Decompose()` directly (in-process) — which
  `TestDecompose_StagedIndex_FRM1e` already does against a REAL git temp repo.
- A subprocess e2e scenario that "stages files and runs stagecoach" lands on the SINGLE-COMMIT
  path (exit 0, one commit), NOT decompose/FR-M1e. So the achievable e2e contribution is the
  ROUTING-BOUNDARY scenario (proves staged → single, never decompose) — the §20.5 regression net
  for the boundary that makes FR-M1e defense-in-depth. See D3.

## 4. Test patterns to follow (verified at HEAD)

### decompose_test.go in-process helpers (real git, t.TempDir):
- `dcmInitRepo(t, repo)` → git init + identity. `dcmCommitRaw(t, repo, msg)` → commit.
- `dcmWriteFile(t, repo, name, body)`. `dcmStageFile(t, repo, name)` → git add. (L45)
- `dcmRunGit`, `dcmGitOut`, `dcmStatusPorcelain`, `dcmLogCount`.
- `dcmDepsWithConfig(t, repo, roles, cfg)` → Deps{Git: git.New(repo), Config: cfg, Roles: roles}.
- `dcmMessageManifest(t, bin, out)` → stub message manifest. `stubtest.Build(t)` → stub binary.
- Call: `result, err := Decompose(context.Background(), deps)`.

### stager_test.go in-process helpers (prefix `stg`, same shape as `dcm`):
- `stgInitRepo`, `stgCommitRaw`, `stgWriteFile`, `stgRunGit`, `stgGitOut`.
- verifyFreezeSubset is called DIRECTLY: `verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, 0, "title", treeI)`.
- baseTree = `stgGitOut(t, repo, "rev-parse", "HEAD^{tree}")`; tStart = `g.FreezeWorkingTree(ctx, baseTree)`;
  treeI = `g.WriteTree(ctx)` after staging; tStartPaths = `g.DiffTreeNames(ctx, baseTree, tStart)`.

### e2e/scenarios_test.go subprocess helpers (`//go:build e2e`):
- `buildStagecoach(t)` (cached), `buildStub(t)`, `newRepo(t)`, `seedCommit(t, repo, name, body)`,
  `writeFile`, `stageFile`, `runGit`, `headSHA`, `commitCount`, `diffTreeNames`, `statusPorcelain`,
  `writeStubConfig(t, stub, extras)`, `stubEnv(knobs)`, `runStagecoach(t, bin, repo, cfg, env, args...)`,
  `contains(ss, s)`.
- Subtest shape: `t.Run("Sx_Name", func(t *testing.T) { ... })` inside `TestE2EScenarios`.

## 5. CRITICAL e2e gotcha — the build tag

`internal/e2e/*_test.go` all carry `//go:build e2e`. `go test ./internal/e2e/...` WITHOUT `-tags e2e`
prints "matched no packages" and runs nothing. The item's stated validation
`go test ./internal/decompose/... ./internal/e2e/...` is INCORRECT for the e2e half; the correct
command is `go test -tags e2e ./internal/e2e/...`. There is no Makefile e2e target and no CI wiring
for `-tags e2e` (grep of Makefile + .github/ is empty) — e2e is run manually. The PRP's validation
commands MUST use `-tags e2e` for any e2e invocation.

## 6. The existing FR-M1e test's want-slice (the D2 gap-fill target)
decompose_test.go:2694:
```go
for _, want := range []string{"a.txt", "b.go", "2 file(s) are staged", "defense-in-depth check (FR-M1e)", "git reset", "stagecoach --single"} {
```
The item contract names "requires an empty index" — ABSENT from this slice (the message contains it,
but no assertion checks it). D2 = add `"requires an empty index"` to the slice.

## 7. No sentinel exists for the FR-M1e error
grep confirms there is NO `ErrNonEmptyIndex` / `ErrEmptyIndex` sentinel — the staged-index error is
a bare `fmt.Errorf` string (not %w-wrapped). So tests assert on SUBSTRINGS + `!errors.Is(err,
ErrDecomposeFailed)`, NOT on a sentinel. Do not invent one.
