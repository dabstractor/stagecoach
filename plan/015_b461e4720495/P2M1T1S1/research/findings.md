# Research Findings — P2.M1.T1.S1: Add StagedNames() to Git interface + gitRunner implementation

## §0 The exact code surface (verified line numbers, git.go)

**The `Git` interface** — `internal/git/git.go:96` (`type Git interface {`).

| Element | File:Line | Detail |
|---|---|---|
| Interface declaration | `internal/git/git.go:96` | `type Git interface {` |
| `StagedFileCount` interface decl | `internal/git/git.go:163` | `StagedFileCount(ctx context.Context) (int, error)` — **`StagedNames` goes IMMEDIATELY after this line (before `RevParseTree` at L165)** |
| `gitRunner.StagedFileCount` impl | `internal/git/git.go:1331` | the EXACT pattern to clone for `StagedNames` |
| `gitRunner.run` helper | `internal/git/git.go:476` | `func (g *gitRunner) run(ctx context.Context, repo string, args ...string) (stdout string, stderr string, exitCode int, err error)` |
| `g.workDir` field | (used at every call site) | the repo root passed to `New(workDir)` |

**New method signature (mandated by contract):**
```go
StagedNames(ctx context.Context) ([]string, error)
```

**Doc comment (mandated verbatim by contract):**
```
StagedNames returns the paths of files currently staged (git diff --cached --name-only).
Returns nil/empty if nothing is staged. Used by FR-M1e to name offending staged paths in
the decompose re-check error.
```

## §1 The gitRunner.StagedFileCount implementation (the template to mirror) — L1331-1349

```go
func (g *gitRunner) StagedFileCount(ctx context.Context) (int, error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "diff", "--cached", "--name-only")
	if err != nil {
		return 0, err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code != 0 {
		return 0, fmt.Errorf("git diff --cached --name-only: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	count := 0
	for _, line := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count, nil
}
```

**`StagedNames` is this method MINUS the count loop, PLUS collecting the trimmed paths into a `[]string`.** The `g.run` invocation, the `err != nil` unwrap, and the `code != 0` error formatting are byte-identical (same git command, same error message prefix `git diff --cached --name-only: failed`).

**Importantly:** the comment block ABOVE `StagedFileCount` (L~1310-1330) explains the `--name-only` (NOT `--quiet`) choice and the newline-split rationale. `StagedNames` shares ALL of that reasoning — place `StagedNames` immediately AFTER `StagedFileCount` so the explanatory comment covers both, and give `StagedNames` its own brief comment noting it returns the paths (where StagedFileCount discards them).

## §2 The `run` helper contract (git.go:476)

`g.run(ctx, repo, args...)` returns `(stdout, stderr, code, err)`:
- `err != nil` ⇒ git binary missing / context cancelled / process failed to start (`code` is `-1`). **ALWAYS unwrap and return first** (do NOT touch `code`/`stderr`).
- `err == nil && code == 0` ⇒ success; parse `stdout`.
- `err == nil && code != 0` ⇒ git ran but failed (non-repo → 129, corrupt → 128, etc.). Branch on `code != 0` (NOT a specific code — convention G5). Format error as `fmt.Errorf("git diff --cached --name-only: failed (exit %d): %s", code, strings.TrimSpace(stderr))`.

This is the SIMPLE/branch form (code != 0 → error), NOT HasStagedChanges' switch form (which treats exit 1 as "changes exist"). `--name-only` never uses `--quiet` (it would suppress the path list).

## §3 Imports — NO new imports needed

`internal/git/git.go` already imports (L3-16): `context`, `fmt`, `strings` (plus bytes, errors, io, os, os/exec, path/filepath, regexp, sort, strconv). `StagedNames` uses only `context`, `strings`, `fmt` — all present. **Do not touch the import block.**

## §4 Test fakes/mocks that implement `Git` — ONLY ONE, and it needs NO change

**Exhaustive search** (`grep -rn "type.*struct" --include="*_test.go"` + manual method grep across all test dirs + pkg/):

| File | Type | How it satisfies `Git` | Action needed? |
|---|---|---|---|
| `internal/cmd/lock_contention_test.go:18` | `contentionFakeGit` | **EMBEDS `git.Git`** (the interface) and overrides ONLY `WriteTree`. | **NONE** — embedding auto-satisfies every interface method, including the new `StagedNames`. The struct is instantiated as `&contentionFakeGit{writeTreeSHA: "..."}` with a nil embedded interface; the tests call only `WriteTree`, so the nil-embedded-method panic never fires. |

- `internal/git/binary_test.go:11` `asRunner(g Git) *gitRunner` is a **type-assertion unwrap** (`g.(*gitRunner)`), NOT a separate implementation. No change.
- **Every other test** (`internal/generate/*_test.go`, `internal/decompose/*_test.go`, `internal/hook/exec_test.go`, `internal/git/*_test.go`) uses `git.New(repo)` — the REAL `*gitRunner` — which will carry `StagedNames` automatically once implemented. No change.

**CRITICAL NUANCE for the contract's step (c):** the contract says "Update ALL test fakes/mocks that implement the Git interface to add a StagedNames method." The implementer must NOT blindly add a `StagedNames` method to `contentionFakeGit` — it embeds the interface, so adding a concrete method is unnecessary and would be dead code. The implementer should instead **VERIFY** via `go build ./...` that nothing breaks (it won't). This is the G8 "embedded interface satisfies the contract" pattern the codebase already documents (`contentionFakeGit` comment L15: "Uncalled methods are nil (panics if invoked)").

## §5 The test template — `internal/git/stagedcount_test.go` (mirror it)

`stagedcount_test.go` (same `package git`) is the canonical pattern for a `diff --cached --name-only` test. Cases to mirror for `StagedNames`, asserting the actual PATHS instead of a count:

| Test case | Setup | Expected `StagedNames` |
|---|---|---|
| NothingStaged | initRepo + makeEmptyCommit("init") | `[]string{}` or `nil` (empty) |
| ThreeFiles | stage a.go, b.go, c.go | `[a.go, b.go, c.go]` |
| AfterAddAll | write a.go (modified) + b.go (untracked), AddAll | `[a.go, b.go]` |
| IncludesDeletion | commit a.go, delete it, AddAll | `[a.go]` |
| FilenameWithSpace | stage `sub/has space.txt` | `["sub/has space.txt"]` (ONE element — space does not split under `--name-only`) |
| UnbornRepoWithStaged | no commit, AddAll f.go | `[f.go]` |
| NotARepo | New(t.TempDir()) plain dir | non-nil err containing `git diff --cached --name-only: failed`, `nil`/`[]` paths |
| GitBinaryMissing | `t.Setenv("PATH","")` | non-nil err containing `git binary not found` |
| ContextCancelled | pre-cancelled ctx | `errors.Is(err, context.Canceled)` |

**Shared test helpers (all in `package git`, already defined):**
- `initRepo(t, dir)` — `internal/git/git_test.go:13`
- `makeEmptyCommit(t, dir, msg)` — `internal/git/revparse_test.go:24`
- `writeFile(t, dir, name, body)` — `internal/git/committree_test.go:31`
- `stageFile(t, dir, name)` — `internal/git/committree_test.go:39`

**NAMING:** new file `internal/git/stagednames_test.go` (matches `stagedcount_test.go`'s `staged`+`count`→`stagednames` derivation; snake_case filename is the codebase convention). Test funcs `TestStagedNames_<Scenario>` (mirror `TestStagedFileCount_<Scenario>`).

**Edge-case assertion note:** for "nothing staged", `git diff --cached --name-only` prints nothing → `strings.Split("", "\n")` returns `[""]` → after filtering empty lines the result is `[]string{}` (a non-nil empty slice) when using `append`, or `nil` if you return the zero value. Either is acceptable (contract says "nil/empty"); the test should assert `len(names) == 0`, NOT `names == nil`, to be robust to both. Document this in the PRP.

## §6 Where StagedNames will be CONSUMED (the NEXT task, NOT this one)

The consumer is **P2.M1.T2.S1 (FR-M1e)**: add a `HasStagedChanges` re-check at the TOP of `Decompose()` (`internal/decompose/decompose.go:141`) with a `StagedNames`-based error. This task (P2.M1.T1.S1) ONLY ships the git primitive — it does NOT touch decompose.go.

Relevant context: `internal/decompose/decompose.go:119-122` currently states the PRECONDITION is owned by the CLI router and "Decompose does NOT re-check this." FR-M1e (next task) adds that re-check and uses `deps.Git.StagedNames(ctx)` to NAME the offending staged paths in the error message. So this task's OUTPUT contract — "`deps.Git.StagedNames(ctx)` returns `[]string` of staged file paths for use in the FR-M1e error message" — is satisfied the moment the primitive lands + compiles; the decompose.go wiring is explicitly deferred.

## §7 Validation commands (Go project — verified against Makefile)

- `go build ./...` — compiles everything (catches the interface-contract break IF any fake didn't embed; it does).
- `go test ./internal/git/... -run StagedNames -v -race` — the new tests, race-enabled.
- `go test ./internal/git/... -race` — full git package (regression: nothing else broke).
- `go test -race ./...` — whole repo (`make test`); fast, catches any downstream compile break.
- `go vet ./internal/git/...` — static checks.
- `gofmt -l internal/git/git.go internal/git/stagednames_test.go` — must print NOTHING (formatting clean).
- `make coverage-gate` — enforces ≥85% statement coverage on `internal/git` (PRD §20.3). New method needs a test (§5 covers it).

## §8 Scope fence (what this task may NOT touch)

- ❌ `internal/decompose/decompose.go` — the FR-M1e re-check is P2.M1.T2.S1, NOT this task.
- ❌ PRD.md, plan/**, tasks.json, prd_snapshot.md, delta_prd.md — orchestrator-owned.
- ❌ Any other source file beyond `internal/git/git.go` (+ the new `internal/git/stagednames_test.go`).
- ❌ No user-facing docs (contract: "internal git primitive, no user-facing surface").
- ✅ ONLY: `internal/git/git.go` (interface decl + gitRunner impl) + `internal/git/stagednames_test.go` (new).

## §9 No StagedNames collision

`grep -rn "StagedNames" --include="*.go" .` → **no existing references.** The name is free; no rename needed.
