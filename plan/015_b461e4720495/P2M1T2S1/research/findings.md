# Research Findings — P2.M1.T2.S1 (FR-M1e empty-index re-assertion in Decompose)

## §0. Exact placement surface (verified against current `internal/decompose/decompose.go`)

```
141: func Decompose(ctx context.Context, deps Deps) (DecomposeResult, error) {
142:	// (1) Mode routing: single ESCAPE-HATCH (planner bypassed) → v1 path.
143:	if deps.Config.Single || deps.Config.Commits == 1 {
144:		return runSingleEscape(ctx, deps)
145:	}
146:	                                                    ← INSERT FR-M1e re-check HERE
147:	// (2) Derive isUnborn + preRunHEAD + baseTree ONCE ...
148:	preRunHEAD, isUnborn, err := deps.Git.RevParseHEAD(ctx)
   ...
165:	tStart, err := deps.Git.FreezeWorkingTree(ctx, baseTree)   ← MUST be AFTER the check
```

**The new FR-M1e check goes at current line 146** — AFTER the escape-hatch `if` block's closing `}`
(line 145) and BEFORE the step-(2) `RevParseHEAD` comment (line 147).

WHY this exact slot (all three constraints satisfied):
1. **After the escape-hatch** (after L145): `Single`/`Commits==1` returns early via
   `runSingleEscape` → `AddAll` → `generate.CommitStaged`, which **handles staged content normally**
   (v1 single-commit primitive). Putting the check BEFORE the escape-hatch would BREAK the legitimate
   `stagecoach --single` with a hand-staged index. So the check MUST be after L145. ✓
2. **Before `FreezeWorkingTree`** (L165): `FreezeWorkingTree(baseTree)` calls `AddAll` FIRST to build
   `T_start`, so any pre-existing staged content gets **silently folded into `T_start`**. The check
   MUST fire before the freeze to be meaningful. Line 146 ≪ 165. ✓
3. **Before `RevParseHEAD`/baseTree derivation** (L148): cheapest possible placement — fail before any
   git plumbing runs. ✓

## §1. The Deps struct gives the Git handle (roles.go)

`internal/decompose/roles.go` — `type Deps struct { Git git.Git; ... }` (the `Git git.Git` field).
So the check calls `deps.Git.HasStagedChanges(ctx)` and `deps.Git.StagedNames(ctx)`. Both are methods
on the injected `git.Git` interface (real `*gitRunner` via `git.New(repo)` in tests/prod; the lone fake
`contentionFakeGit` embeds the interface — but Decompose tests use the real runner).

## §2. HasStagedChanges — semantics (already exists, git.go:1079)

```go
// git.go:133 (interface) / 1079 (impl)
HasStagedChanges(ctx context.Context) (bool, error)
```
Runs `git diff --cached --quiet`. **Exit-code-inverted**: exit 0 → nothing staged (returns `false,
nil`); exit 1 → staged changes exist (returns `true, nil` — exit 1 is the SIGNAL, not an error); any
other exit → real error. The method structurally encodes the inversion so callers can't misread it.
→ For FR-M1e we just read the `(bool, error)`; no exit-code juggling needed.

## §3. StagedNames — the input (PROVIDED BY P2.M1.T1.S1, the previous PRP)

Per the previous PRP (treat as a CONTRACT — it will be implemented exactly), `StagedNames` is added to
the `Git` interface immediately after `StagedFileCount` and implemented on `*gitRunner`:

```go
// interface (after StagedFileCount at git.go:163)
StagedNames(ctx context.Context) ([]string, error)
```
Runs `git diff --cached --name-only` (same command as `StagedFileCount`, which discards the paths).
Returns the staged PATHS as `[]string` (nil/empty when nothing staged); non-nil error on non-repo /
git-missing / cancelled context (same error semantics as `StagedFileCount`). **This task CONSUMES it;**
it does NOT define it. If P2.M1.T1.S1 has NOT landed yet (parallel execution), the implementer should
expect `deps.Git.StagedNames` to compile because the interface change ships in that sibling task — the
two are sequenced so this task runs after it.

## §4. The error contract (from the item description — follow VERBATIM)

TWO branches, distinct error shapes:

**(a) `HasStagedChanges` itself errors** (non-repo / git missing / cancelled) — wrap the sentinel:
```go
hasStaged, err := deps.Git.HasStagedChanges(ctx)
if err != nil {
    return DecomposeResult{}, fmt.Errorf("%w: check staged changes: %w", ErrDecomposeFailed, err)
}
```

**(b) `hasStaged == true`** — a PLAIN `fmt.Errorf` with NO `%w` wrap of `ErrDecomposeFailed`. The
message itself is the actionable user guidance (names the paths, offers remedies):
```go
if hasStaged {
    names, _ := deps.Git.StagedNames(ctx) // best-effort: we already know something is staged
    return DecomposeResult{}, fmt.Errorf(
        "decompose requires an empty index, but %d file(s) are staged: %s. "+
        "This is a defense-in-depth check (FR-M1e) — the trigger should have routed to the "+
        "single-commit path. Run `git reset` to unstage, or `stagecoach --single` for the "+
        "one-commit behavior",
        len(names), strings.Join(names, ", "))
}
```

**DESIGN NOTE — the `names, _ :=` discards StagedNames' error INTENTIONALLY.** We already proved via
HasStagedChanges that something IS staged; StagedNames is a best-effort enrichment to NAME the paths.
If StagedNames errors, `names` is nil → the message still prints `0 file(s) are staged:` with the
HasStagedChanges truth. This is acceptable degraded behavior; do NOT promote the StagedNames error to a
hard failure.

**DESIGN NOTE — the staged-content error does NOT wrap `ErrDecomposeFailed` INTENTIONALLY.** It is a
distinct, user-facing actionable category, not an orchestrator infra failure. Wrapping it with
"decompose: orchestrator failed:" would muddy the clear remedy. Follow the contract verbatim; do NOT
"helpfully" add `%w` + the sentinel.

## §5. Imports — NONE to add (decompose.go:29-38)

`decompose.go` already imports `context`, `errors`, `fmt`, `strings`. Both `HasStagedChanges` and
`StagedNames` are on the injected `deps.Git git.Git` (also already imported). **Do NOT touch the import
block.**

## §6. Doc-comment to update (decompose.go:121-122)

Current:
```
// PRECONDITION (FR-M1, owned by the CLI router — P4.M1.T1.S1): the caller routed here because NOTHING is
// staged (HasStagedChanges false) AND the working tree has changes. Decompose does NOT re-check this; it
// assumes correct routing.
```
FR-M1e CHANGES this contract → rewrite to say Decompose NOW re-asserts the empty-index precondition
(defense-in-depth, FR-M1e), placed after the escape-hatch (single/--commits 1 still uses the single
primitive, which handles staged content normally).

## §7. Test patterns (decompose_test.go) — all helpers verified

- `dcmInitRepo(t, dir)` — `git init` + identity in a temp dir.
- `dcmWriteFile(t, dir, name, body)` — create a file.
- `dcmStageFile(t, dir, name)` — `git add <name>` (STAGES it — the key setup for the FR-M1e test).
- `dcmRunGit(t, dir, args...)` / `dcmGitOut` — raw git, trimmed stdout.
- `dcmLogCount(t, dir)` — commits reachable from HEAD (0 on unborn); used to assert NO commit created.
- `dcmStatusPorcelain(t, dir)` — `git status --porcelain`; assert staged files REMAIN staged (index untouched).
- `dcmDepsWithConfig(t, repo, roles RoleManifests, cfg config.Config) Deps` — builds Deps with
  `Git: git.New(repo)` (the REAL runner → carries HasStagedChanges + StagedNames automatically).
- `config.Defaults()` — default config (Commits=0 auto, Single=false).
- `stubtest.Build(t)` + `dcmMessageManifest(t, bin, out)` — stub provider for the message role (NOT
  reached by the FR-M1e test, but the Deps needs SOME roles to build).
- Error assertions: `errors.Is(err, ...)` for sentinel checks; `strings.Contains(err.Error(), "...")`
  for message-content checks.

**No existing test stages files BEFORE calling Decompose in the non-escape path** — they all create
un-staged (`dcmWriteFile`) files and let the stager seam stage mid-loop. So the new re-check does NOT
break any existing test (verified via grep: every multi-commit test uses `dcmWriteFile`, not
`dcmStageFile`, in setup). The escape-hatch tests (`SingleEscape`, `Commits1_Mode`) bypass the new
check (it is placed AFTER the escape-hatch).

## §8. Docs update — docs/how-it-works.md "### Trigger" (line 51) / "### Safety" (line 123)

The natural home is the **Trigger** section (the routing model): add one sentence noting the FR-M1e
defense-in-depth re-assertion at Decompose's entry (after the escape-hatch). Keep it to ~3 lines, Mode A
(sync the change-level overview). Optionally cross-reference from Safety.

## §9. Validation commands (verified against Makefile)

- `go build ./...` (compiles; proves the StagedNames call resolves — depends on P2.M1.T1.S1 having landed).
- `go test ./internal/decompose/... -run FRM1e -race -v` (the new tests).
- `go test -race ./...` (whole-repo regression; the re-check must not break the escape-hatch / happy-path tests).
- `gofmt -l internal/decompose/decompose.go internal/decompose/decompose_test.go` (must print nothing).
- `go vet ./internal/decompose/...`.
- `git status --porcelain` — ONLY `decompose.go` + `decompose_test.go` + `docs/how-it-works.md`.

## §10. Scope fence — what NOT to touch

- Do NOT modify the Git interface or `*gitRunner` (StagedNames/HasStagedChanges) — that is P2.M1.T1.S1.
- Do NOT touch `FreezeWorkingTree`, `runSingleEscape`, or the loop — only the Decompose() entry re-check.
- Do NOT change the CLI router — the re-check is a defense-in-depth INSIDE Decompose (the router still
  does its own HasStagedChanges routing upstream).
- Do NOT add the check before the escape-hatch — it would break legitimate `stagecoach --single` with a
  hand-staged index.
