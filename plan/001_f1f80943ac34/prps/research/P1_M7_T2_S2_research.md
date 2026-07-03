# Research — P1.M7.T2.S2: maybeAutoStage — auto-stage-all & nothing-staged CLI path

This task OWNS the staging POLICY entry-point (FR16–FR20). It is the **sole
staging entry-point** the CLI default action calls; v2 swaps it whole for a
partitioned-staging loop, so the seam must be one clean function
(decisions.md §1, plan_overview key decision 6).

## 0. S1 already shipped a CLOSE-but-not-contract-faithful version (DO NOT re-create — RECONCILE)

P1.M7.T2.S1 (Complete) wired the WHOLE default action and, because S2 was not
yet done, **inlined** the staging policy as `maybeAutoStagePolicy` in
`cmd/stagehand/run.go`. It is functionally close to FR16–FR20 but **diverges
from S2's contract on three points**. S2's job is to reconcile those three gaps
WITHOUT disturbing S1's working default-action flow:

| Aspect | S1 shipped (`maybeAutoStagePolicy`) | S2 contract requires |
|---|---|---|
| Signature | `(g stager, out, cfg, allFlag, noAutoStage) int` (exit code) | `maybeAutoStage(...) error` (sentinel errors) |
| FR17 vs FR19 | BOTH collapse to one `ui.ExitNothingToCommit` (2) | DISTINCT sentinels: `ErrNothingToCommit` (FR17, clean after add) vs `ErrNothingStaged` (FR19, --no-auto-stage / auto_stage_all=false) |
| FR18 notice | `"Nothing staged — staging all changes.\n"` (no count) | `"Nothing staged — staging all changes (N files).\n"` (file count) |

S1's own research note (§5) **flagged maybeAutoStage as "sibling S2 test
surface"** and annotated the FR18 line `// FR18 (file count optional)` — i.e.
S1 deliberately deferred the count + the separable entry-point to S2. So this
is the planned reconciliation, NOT a rework of broken code.

Verified current call site in `cmd/stagehand/run.go`:
```go
allFlag, _ := cmd.Flags().GetBool("all")
noAutoStage, _ := cmd.Flags().GetBool("no-auto-stage")
if code := maybeAutoStagePolicy(g, out, cfg, allFlag, noAutoStage); code != ui.ExitSuccess {
    return code
}
```
And `mapErrorToExitCode` currently handles only the GENERATE sentinels
(`ErrNothingToCommit→2`, `ErrRescue→3`, other→1).

## 1. `ErrNothingStaged` does NOT exist yet

`grep -rn ErrNothingStaged` → nothing. Only `ErrNothingToCommit` exists:
- `internal/generate/generate.go:162` `ErrNothingToCommit = errors.New("nothing staged to commit")` (the diff=="" gate, FR5/FR17).
- `pkg/stagehand/stagehand.go:104` `ErrNothingToCommit = generate.ErrNothingToCommit` (public alias).

`ErrNothingStaged` is a **CLI-layer concept** (FR19: the user declined
auto-staging OR `auto_stage_all=false`, so the index was never touched). It is
NOT a generate error — generate only sees `ErrNothingToCommit` after the diff
gate. ⟹ Define `ErrNothingStaged` as a **package-level sentinel in
`cmd/stagehand`** (package main; the tests are white-box package main so they
can `errors.Is` it). Both `ErrNothingStaged` and `ErrNothingToCommit` map to
**exit 2** (`ui.ExitNothingToCommit`); the distinction is the SENTINEL (for
programmatic `errors.Is`), not the exit code — so the PRD §15.4 exit-code table
is unchanged.

## 2. The `(N files)` count needs a minimal new git primitive

`(*git.Git).AddAll() error` returns only an error — no count. Go forbids adding
a method to a type from another package, so a count method MUST live in
`package git` (i.e. `internal/git/stage.go`). The minimal, pattern-consistent
addition is a sibling of `HasStagedChanges`:

```go
// StagedFileCount counts the files currently staged (index vs HEAD) via
// `git diff --cached --name-only` (literal args — PRD §19 no sh -c). Sibling
// of HasStagedChanges; the CLI auto-stage path consumes it for the FR18
// "(N files)" notice. On a non-zero git exit it surfaces the typed *ExitError.
func (g *Git) StagedFileCount() (int, error) {
    out, err := g.run("diff", "--cached", "--name-only")
    if err != nil { return 0, err }
    n := 0
    for _, line := range strings.Split(out, "\n") {
        if strings.TrimRight(line, "\r") != "" { n++ }
    }
    return n, nil
}
```
- `internal/git/stage.go` currently imports only `"errors"` → ADD `"strings"`.
- Test pattern: follow `TestHasStagedChanges_*` in `internal/git/stage_test.go`
  (package git, REAL host git binary, `newTempRepo`/`seedCommits`/`writeFileStage`
  harness from `gittestutil_test.go`): `TestStagedFileCount_ZeroOnClean` (clean
  index → 0) + `TestStagedFileCount_AfterStagingFiles` (stage 3 files → 3).
- Scope note: this touches M3 (`internal/git`), but it is ADDITIVE, sibling of a
  shipped primitive, and the minimal way to honor the contract's exact FR18
  text. It does not change any existing method or harm downstream tasks.

This is the ONLY change outside `cmd/stagehand`. Everything else is CLI-layer.

## 3. The error-returning shape (binding spec)

`maybeAutoStage(g stager, out *ui.Output, cfg config.Config, allFlag, noAutoStage bool) error`:
```go
staged, err := g.HasStagedChanges()
if err != nil { return fmt.Errorf("stage: %w", err) }          // → exit 1
if staged {
    if allFlag { if err := g.AddAll(); err != nil { return fmt.Errorf("stage: %w", err) } }  // FR20
    return nil                                                   // proceed
}
if noAutoStage || !cfg.AutoStageAll {                            // FR19
    out.Progressf("Nothing staged; nothing to commit.\n")
    return ErrNothingStaged
}
if err := g.AddAll(); err != nil { return fmt.Errorf("stage: %w", err) }  // FR16
n, err := g.StagedFileCount()
if err != nil { return fmt.Errorf("stage: %w", err) }
out.Progressf("Nothing staged — staging all changes (%d files).\n", n)     // FR18
staged, err = g.HasStagedChanges()
if err != nil { return fmt.Errorf("stage: %w", err) }
if !staged {                                                     // FR17 clean after add
    out.Progressf("Nothing to commit.\n")
    return ErrNothingToCommit
}
return nil                                                        // proceed
```
- The `stager` interface gains `StagedFileCount() (int, error)`; `*git.Git`
  satisfies it; the test `fakeStager` stubs it.
- `runDefault` call site: `if err := maybeAutoStage(...); err != nil { return mapErrorToExitCode(err) }`.
- `mapErrorToExitCode` gains one branch:
  `errors.Is(err, ErrNothingStaged) → ui.ExitNothingToCommit` (placed before the
  generic fallback; both staging sentinels → exit 2).

## 4. Test coverage (the MOCKING contract) — 7 cases

The shipped tests cover 4 of 7; the **"proceed after auto-stage" happy path is
MISSING** and the assertions must move from int→`errors.Is`. The stateful
`fakeStager` flips `staged` to `stagedAfterAdd` when `AddAll()` is called:

1. **NEW** nothing-staged + auto → `AddAll` called + notice contains `(%d files)`
   + `stagedAfterAdd=true` → returns **nil** (proceed).
2. nothing-staged + auto + still-clean (`stagedAfterAdd=false`) →
   `errors.Is(err, ErrNothingToCommit)` + stderr "Nothing to commit" (FR17).
3. nothing-staged + `--no-auto-stage` → `errors.Is(err, ErrNothingStaged)` +
   `addCalled==false` + stderr mentions nothing-to-commit (FR19).
4. nothing-staged + `cfg.AutoStageAll=false` (no --no-auto-stage) →
   `ErrNothingStaged` + `addCalled==false` (FR19 second trigger).
5. already-staged + `--all` → `addCalled==true` + nil (FR20 force-add on top).
6. already-staged, no `--all` → `addCalled==false` + nil (common path).
7. `mapErrorToExitCode(ErrNothingStaged) == ui.ExitNothingToCommit (2)`.

## 5. Docs impact (Mode B)

S2's context_scope has NO `DOCS:` line (S1 owns Mode A `docs/CONFIGURATION.md`).
docs already documents exit-2 conflating FR17/FR19
(`docs/CONFIGURATION.md:304` "...a clean tree after auto-staging, or nothing
staged with --no-auto-stage (FR17/FR19)") and the `--no-auto-stage` flag
(`:271`). The FR18 notice text is NOT quoted verbatim anywhere in docs, so
adding "(N files)" breaks no doc reference. ⟹ S2 makes NO required doc edits
(Mode B). OPTIONAL nicety: if the agent touches docs, refine the exit-2 row to
mention both sentinels — but this is not required and S1 owns that file.

## 6. v2-seam preservation (decisions.md §1)

Staging POLICY stays in the CLI layer (`maybeAutoStage`); `CommitStaged` /
`GenerateCommit` STILL never stage and still assume the index is pre-staged.
Reshaping the return type int→error does NOT touch the generate/core seam. v2
replaces `maybeAutoStage` whole with a partition loop — keeping it a single,
named function with a tight interface is exactly what preserves that swap.

## 7. External references
- Git plumbing: `git diff --cached --quiet` (--exit-code boolean) and
  `git diff --cached --name-only` (file list) —
  https://git-scm.com/docs/git-diff#Documentation/git-diff.txt---cached
- Go sentinel-error idiom: https://go.dev/wiki/Errors#checking-errors
