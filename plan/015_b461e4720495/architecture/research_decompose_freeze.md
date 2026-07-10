# Research: Decompose Entrypoint & Freeze Enforcement (FR-M1c / FR-M1e)

Scope: how decomposition activates, where `T_start` is frozen, how FR-M1c
(subset check) is enforced today, what the violation error looks like, and which
git primitives are available for listing staged paths. Findings are for planning
FR-M1e (re-assert empty-index precondition with a clear error) and improving
FR-M1c (name offending paths in the error).

---

## 1. Decomposition trigger (`internal/cmd/default_action.go`)

### Trigger logic — `runDefault`, the `!hasStaged` branch

After the lock is acquired and `HasStagedChanges` is computed, the `!hasStaged`
branch runs the FR-M1 routing. The exact block (approx **L130–L145**):

```go
if !hasStaged {
    // FR-M1 (P4.M1.T1.S1): nothing staged + dirty tree + decompose enabled → decompose (NO AddAll).
    if shouldDecompose(cfg, flagDryRun, flagNoAutoStage) {
        status, err := g.StatusPorcelain(ctx)
        if err != nil {
            return exitcode.New(exitcode.Error, fmt.Errorf("git status --porcelain: %w", err))
        }
        if status == "" {
            return exitcode.New(exitcode.NothingToCommit, errors.New("Nothing to commit.")) // clean tree
        }
        return runDecompose(ctx, stdout, stderr, u, cfg, g, repoDir) // planner gets the working-tree diff
    }
    // ... auto-stage-all / nothing-staged paths follow
}
```

### The routing predicate — `shouldDecompose` (approx **L300**)

PURE, no I/O, no package-flag reads:

```go
func shouldDecompose(cfg *config.Config, dryRun, noAutoStage bool) bool {
    if cfg == nil {
        return false
    }
    if cfg.Single || cfg.Commits == 1 { // --single/--no-decompose/--commits 1 → v1
        return false
    }
    if dryRun { // decompose commits; --dry-run → single preview
        return false
    }
    return cfg.AutoStageAllValue() && !noAutoStage // FR-M1 trigger context (auto-stage on)
}
```

So: decompose activates iff **nothing staged** (caller guarantees via `hasStaged`) +
**dirty tree** (`StatusPorcelain != ""`) + **auto-stage-all on** + **not single**
+ **not dry-run**.

### `runDecompose` — the CLI wrapper (approx **L330**)

Builds `decompose.Deps` (resolves the four roles, excludes, verbose) and calls
`decompose.Decompose(ctx, deps)` directly. It does **not** itself re-check the
empty-index precondition — it trusts the `runDefault` routing.

---

## 2. Decompose entrypoint (`internal/decompose/decompose.go`)

### `Decompose()` signature and first steps (approx **L140**)

```go
func Decompose(ctx context.Context, deps Deps) (DecomposeResult, error) {
    // (1) Mode routing: single ESCAPE-HATCH (planner bypassed) → v1 path.
    if deps.Config.Single || deps.Config.Commits == 1 {
        return runSingleEscape(ctx, deps)
    }

    // (2) Derive isUnborn + preRunHEAD + baseTree ONCE.
    preRunHEAD, isUnborn, err := deps.Git.RevParseHEAD(ctx)
    ...
    baseTree := git.EmptyTreeSHA
    if !isUnborn {
        baseTree, err = deps.Git.RevParseTree(ctx, "HEAD")
        ...
    }

    // (3) FR-M1b: Freeze the entire working-tree change set into T_start.
    tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)
    ...
    lock.SetSnapshot(tStart)
    ...
}
```

### Does `Decompose()` re-check the empty-index precondition? **NO.**

The `Decompose` doc-comment (approx L125) states this explicitly:

> PRECONDITION (FR-M1, owned by the CLI router — P4.M1.T1.S1): the caller routed
> here because NOTHING is staged (HasStagedChanges false) AND the working tree has
> changes. **Decompose does NOT re-check this; it assumes correct routing.**

This is the core gap for **FR-M1e**: there is no defense-in-depth re-assertion.
The only callers are `runDecompose` (CLI) and tests (which inject `deps.stager`).
A stale index from a bug or a future caller would be silently swept into `T_start`
by `FreezeWorkingTree`'s first step (`AddAll`).

### Where is `T_start` frozen?

`T_start` is assigned at step (3) — **`deps.Git.FreezeWorkingTree(ctx, baseTree)`**
(approx **L170**). This happens AFTER mode routing + baseTree derivation, and
BEFORE the one-file short-circuit, the planner, the loop, and the arbiter. The
escape-hatch (`runSingleEscape`) returns at step (1) above and does **NOT** freeze.

`lock.SetSnapshot(tStart)` immediately publishes the frozen tree for the FR52
no-op fast path.

---

## 3. FR-M1c freeze enforcement (`internal/decompose/stager.go`)

### The sentinel

```go
// stager.go:75
var ErrFreezeViolation = errors.New("decompose: freeze violation")
```

### `verifyFreezeSubset` — the two-part content-subset check (stager.go **L158–L196**)

```go
func verifyFreezeSubset(ctx context.Context, deps Deps, baseTree, tStart string,
    tStartPaths []string, i int, treeI string) error {
    // (A) PATH check: tree[i]'s changed paths must all be in T_start's changed set.
    changedTreeI, err := deps.Git.DiffTreeNames(ctx, baseTree, treeI)
    ...
    tStartSet := pathSet(tStartPaths)
    var extra []string
    for _, p := range changedTreeI {
        if _, ok := tStartSet[p]; !ok {
            extra = append(extra, p)
        }
    }
    if len(extra) > 0 {
        return fmt.Errorf("%w: concept %d staged paths not present in T_start: %s",
            ErrFreezeViolation, i, strings.Join(extra, ", "))
    }

    // (B) CONTENT check: tree[i]'s changed paths must carry T_start's blob content.
    delta, err := deps.Git.DiffTreeNames(ctx, treeI, tStart)
    ...
    deltaSet := pathSet(delta)
    var mismatch []string
    for _, p := range changedTreeI {
        if _, ok := deltaSet[p]; ok {
            mismatch = append(mismatch, p)
        }
    }
    if len(mismatch) > 0 {
        return fmt.Errorf("%w: concept %d staged content not traceable to T_start: %s",
            ErrFreezeViolation, i, strings.Join(mismatch, ", "))
    }
    return nil
}
```

### Where it's called — `runLoop` (decompose.go **L558–L562**)

```go
treeI, err := freezeSnapshot(ctx, deps)
...
// FR-M1c freeze enforcement: verify tree[i] is a content-subset of T_start.
if vErr := verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, i, treeI); vErr != nil {
    drainMsg(inflight)
    return commits, nil, vErr
}
```

`tStartPaths` is computed ONCE at the top of `runLoop` (decompose.go ~L485):
`deps.Git.DiffTreeNames(ctx, baseTree, tStart)` — the frozen changed-path baseline.

On violation: `drainMsg(inflight)` (avoid goroutine leak) + return partial commits
+ the error (HARD, non-rescue). Mirrors the `ErrStagerMovedHEAD` ref-axis guard.

---

## 4. Current freeze-violation error messages

**Both error messages ALREADY name the offending paths.** The paths are joined
with `strings.Join(..., ", ")`:

| Check | Format string | Example output |
|-------|--------------|----------------|
| (A) Path | `freeze violation: concept %d staged paths not present in T_start: %s` | `decompose: freeze violation: concept 0 staged paths not present in T_start: sentinel.txt` |
| (B) Content | `freeze violation: concept %d staged content not traceable to T_start: %s` | `decompose: freeze violation: concept 0 staged content not traceable to T_start: foo.go` |

**Function that generates them:** `verifyFreezeSubset` (stager.go L158).

### Key finding for "improve FR-M1c (name offending paths)"

The offending paths are **already** named in both branches. The improvement
opportunity is therefore NOT "add path names" (they exist), but possibly:
- Making the messages more actionable (e.g., showing expected-vs-actual content
  for the content-mismatch case, or indicating the concept title).
- The phrase "not traceable to T_start" is the opaque part — it could be made
  clearer (e.g., "content differs from the frozen working-tree snapshot").
- The concept is identified by numeric index `i`, not by its title (`concepts[i].Title`)
  — `verifyFreezeSubset` does not currently receive the concept title.

Tests assert the path IS named: `decompose_test.go:768` checks
`strings.Contains(err.Error(), "sentinel.txt")` and `stager_test.go:376` checks
`"not present in T_start"`.

---

## 5. `FreezeWorkingTree` (`internal/git/git.go` **L1744**)

```go
func (g *gitRunner) FreezeWorkingTree(ctx context.Context, baseTree string) (string, error) {
    // 1. Stage the full working-tree change set.
    if err := g.AddAll(ctx); err != nil {
        return "", err
    }
    // 2. Freeze the index into the immutable tree object T_start.
    tStart, err := g.WriteTree(ctx)
    if err != nil {
        return "", err
    }
    // 3. Reset the index to the clean base so the per-concept stager starts clean.
    if err := g.ReadTree(ctx, baseTree); err != nil {
        return "", err
    }
    return tStart, nil
}
```

Three-step orchestration: **AddAll → WriteTree → ReadTree(baseTree)**. Mutates
the index (transitively) but touches no ref. After return, index == baseTree,
working-tree files unchanged. **CRITICAL for FR-M1e:** `AddAll` (step 1) stages
everything — so any pre-existing staged content gets folded into `T_start`
silently. An empty-index re-check MUST run **before** `FreezeWorkingTree` is
called (i.e., at the very top of `Decompose`, before step 3).

Interface signature (git.go ~L270):
```go
FreezeWorkingTree(ctx context.Context, baseTree string) (tStart string, err error)
```

---

## 6. `HasStagedChanges` (`internal/git/git.go` **L1079**)

```go
func (g *gitRunner) HasStagedChanges(ctx context.Context) (bool, error) {
    _, stderr, code, err := g.run(ctx, g.workDir, "diff", "--cached", "--quiet")
    if err != nil {
        return false, err
    }
    switch code {
    case 0:
        return false, nil // nothing staged (index == HEAD)
    case 1:
        return true, nil  // staged changes exist — exit 1 is the signal, NOT an error
    default:
        msg := strings.TrimSpace(stderr)
        if code == 129 && strings.Contains(msg, "not a git repository") {
            return false, fmt.Errorf("not a git repository (or any of the parent directories): .git")
        }
        return false, fmt.Errorf("git diff --cached --quiet: failed (exit %d): %s", code, msg)
    }
}
```

Runs `git diff --cached --quiet`. Exit 0 = clean index, exit 1 = staged changes.
This is the primitive to use for the FR-M1e re-assertion.

---

## 7. Git functions available for listing staged paths

| Function | File:Line | What it returns | Notes |
|----------|-----------|-----------------|-------|
| `StagedFileCount` | git.go **L1331** | `int` (count only) | Runs `git diff --cached --name-only`, counts non-empty lines. **Discards the paths.** |
| `StagedDiff` | git.go **L906** | `string` (full diff payload) | The staged diff with caps/placeholders — too heavy for an error message. |
| `StatusPorcelain` | git.go **L1571** | `string` (raw porcelain) | Raw `git status --porcelain` output (includes unstaged + untracked too — NOT staged-only). |
| `HasStagedChanges` | git.go **L1079** | `bool` | Boolean only — no paths. |
| `StagedNumstatSkeleton` | git.go **L2142** | `string` | numstat skeleton of staged set (has paths but embedded in formatted output). |

### **There is NO function that returns staged paths as `[]string`.**

`StagedFileCount` (L1331) runs the exact command needed
(`git diff --cached --name-only`) but throws away the paths and returns only a
count:

```go
func (g *gitRunner) StagedFileCount(ctx context.Context) (int, error) {
    stdout, stderr, code, err := g.run(ctx, g.workDir, "diff", "--cached", "--name-only")
    ...
    count := 0
    for _, line := range strings.Split(stdout, "\n") {
        if strings.TrimSpace(line) != "" {
            count++ // ← paths discarded here
        }
    }
    return count, nil
}
```

**For FR-M1e (clear error naming offending staged paths) and any staged-path
listing, a new git method is needed** — e.g. `StagedNames(ctx) ([]string, error)`
reusing the `git diff --cached --name-only` pattern, or refactoring
`StagedFileCount` to share a path-listing helper. This would be a new method on
the `Git` interface (git.go L100+) + `gitRunner` implementation + test file.

---

## Architecture: data flow

```
runDefault (cmd/default_action.go)
  ├── HasStagedChanges → false
  ├── shouldDecompose() → true
  ├── StatusPorcelain != "" (dirty tree)
  └── runDecompose()
        └── decompose.Decompose(ctx, deps)        [decompose.go L140]
              ├── (1) escape-hatch if Single/Commits==1
              ├── (2) RevParseHEAD → preRunHEAD, isUnborn
              ├── (2) RevParseTree("HEAD") → baseTree
              ├── (3) FreezeWorkingTree(baseTree) → tStart   ★ T_start frozen here
              │       └── AddAll → WriteTree → ReadTree(baseTree)
              ├── one-file short-circuit (FR-M2b)
              ├── callPlanner(diff frozen T_start)
              └── runLoop(concepts, baseTree, tStart)        [decompose.go L470]
                    ├── tStartPaths = DiffTreeNames(base, tStart)  [baseline]
                    └── per concept i:
                          ├── invokeStagerRetry → stageConcept (tooled agent)
                          ├── freezeSnapshot → treeI (WriteTree)
                          ├── verifyFreezeSubset(base, tStart, tStartPaths, i, treeI)  ★ FR-M1c
                          │     ├── (A) path: DiffTreeNames(base,treeI) ⊆ tStartPaths
                          │     └── (B) content: changedTreeI ∩ DiffTreeNames(treeI,tStart) == ∅
                          └── on violation → drainMsg + return partial + ErrFreezeViolation
```

---

## Key types / structs

**`decompose.Deps`** (roles.go **L55**):
```go
type Deps struct {
    Git      git.Git
    Registry *provider.Registry
    Config   config.Config
    Roles    RoleManifests
    Verbose  *ui.Verbose
    Excludes []string
    Out      io.Writer
    stager   func(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error // test seam
}
```

**`git.Git` interface** (git.go L100–L820): the boundary. Adding a staged-paths
method requires adding it here + implementing on `*gitRunner`.

**`git.FileChange`** (git.go L19): `{Status, SrcPath, Path string}`.

**`git.StagedDiffOptions`** (git.go L46): diff config struct.

---

## Start Here

**`internal/decompose/decompose.go` — the `Decompose()` function (L140).**

This is where FR-M1e belongs: add a `HasStagedChanges` re-check at the very top
of `Decompose` (before step 2/3), returning a clear error if the index is
non-empty. The check MUST precede `FreezeWorkingTree` (L170) because
`FreezeWorkingTree`'s first step (`AddAll`) would otherwise sweep the stale
staged content into `T_start`.

**`internal/decompose/stager.go` — `verifyFreezeSubset` (L158)** is where FR-M1c
error-message improvements belong (note: paths are already named — see §4).

**`internal/git/git.go` — the `Git` interface (L100)** needs a new staged-paths
method if FR-M1e should name the offending staged paths in its error (§7 shows
none exists today; `StagedFileCount` at L1331 is the closest template).

---

## Constraints, risks, and open questions

1. **FR-M1e ordering is critical.** The empty-index check must run BEFORE
   `FreezeWorkingTree` (which calls `AddAll`). If placed after, the check is
   meaningless (everything is already staged by the freeze).

2. **FR-M1e ownership question.** The precondition is currently documented as
   "owned by the CLI router" (`runDefault`). Adding a re-check inside
   `Decompose` makes it defense-in-depth (library-safe). Decide: should
   `Decompose` re-check (defense-in-depth) or only `runDecompose` (CLI layer)?
   The doc-comment currently says Decompose does NOT re-check — adding one
   changes that contract.

3. **FR-M1c — paths are already named.** The task's premise ("name offending
   paths") appears to be already satisfied. Confirm with the planner whether the
   improvement is about (a) the opaque phrasing "not traceable to T_start", (b)
   showing the concept title instead of index `i`, or (c) showing content diffs.
   `verifyFreezeSubset` does NOT currently receive the concept title — passing it
   in would require a signature change + updating the one call site in `runLoop`.

4. **No staged-paths git primitive exists.** FR-M1e's "clear error naming the
   offending paths" requires a new `Git` method (e.g. `StagedNames`). This is a
   new interface method → every mock/fake implementing `Git` must be updated
   (check for test fakes).

5. **Test seams.** `deps.stager` is the injection point for orchestrator tests.
   Existing freeze tests: `TestDecompose_StagerFreezeViolation`
   (decompose_test.go L736), `TestVerifyFreezeSubset_PathViolation` /
   `_ContentViolation` / `_EmptyStaging` (stager_test.go L310+). These are the
   patterns to extend.
