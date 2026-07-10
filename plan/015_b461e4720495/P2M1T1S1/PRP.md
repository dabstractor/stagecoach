name: "P2.M1.T1.S1 — Add StagedNames() to Git interface + gitRunner implementation (FR-M1e primitive, §9.14)"
description: >
  Add a read-only git primitive `StagedNames(ctx context.Context) ([]string, error)` that returns the PATHS of
  currently-staged files (`git diff --cached --name-only`), instead of just the COUNT that `StagedFileCount`
  already provides (it runs the identical git command but discards the paths). This is the foundational
  primitive for the FR-M1e decompose-entry re-assertion error (P2.M1.T2.S1 — the NEXT task — will call
  `deps.Git.StagedNames(ctx)` to NAME the offending staged paths in the "something is staged, can't decompose"
  error). This task ships ONLY the primitive + its unit tests; it does NOT touch decompose.go. Concretely:
  (a) add the `StagedNames` method to the `Git` interface in `internal/git/git.go` immediately after
  `StagedFileCount` (interface decl at L163), with the contract-mandated doc comment; (b) implement it on
  `*gitRunner` by CLONING the `StagedFileCount` impl (L1331) — same `g.run(ctx, g.workDir, "diff", "--cached",
  "--name-only")`, same err/code branching, same error-message format — but COLLECT the trimmed non-empty lines
  into a `[]string` instead of counting them; (c) VERIFY the one existing Git fake (`contentionFakeGit`,
  lock_contention_test.go:18) needs NO change because it EMBEDS `git.Git` (the interface auto-satisfies the new
  method) — confirmed via `go build ./...`. No new imports (context/fmt/strings already imported). Validates
  via `go build ./...`, `go test ./internal/git/... -run StagedNames -race -v`, `gofmt -l`, `go vet`,
  `make coverage-gate` (≥85% on internal/git).

---

## Goal

**Feature Goal**: A new read-only git primitive `StagedNames` on the `Git` interface / `*gitRunner` that returns
the `[]string` of currently-staged file paths (the paths `StagedFileCount` currently runs `git diff --cached
--name-only` to obtain but throws away). It is the input the FR-M1e error (next task) needs to NAME the
offending staged paths.

**Deliverable**: Two edits to `internal/git/git.go` (interface method declaration after L163 + `*gitRunner`
implementation after L1349, both with the contract doc comments) and one new file
`internal/git/stagednames_test.go` (unit tests mirroring `stagedcount_test.go`'s scenarios but asserting the
returned paths). NO other files change (the lone Git fake embeds the interface — no method addition needed).

**Success Definition**:
- `deps.Git.StagedNames(ctx)` compiles and returns the staged file paths as `[]string` (empty when nothing is
  staged; the actual paths when files are staged; a non-nil error on non-repo / git-missing / cancelled
  context — same error semantics as `StagedFileCount`).
- `go build ./...` succeeds (proves the interface change breaks nothing — `contentionFakeGit` embeds `Git`).
- `go test ./internal/git/... -run StagedNames -race -v` passes (all §5 scenarios).
- `go test -race ./...` passes (no downstream regression).
- `gofmt -l internal/git/git.go internal/git/stagednames_test.go` prints nothing; `go vet ./internal/git/...`
  clean.
- `git status --porcelain` shows ONLY `internal/git/git.go` + the new `internal/git/stagednames_test.go`.

## User Persona (if applicable)

**Target User**: A Stagecoach developer/maintainer wiring the FR-M1e decompose-entry guard (the immediate
consumer, P2.M1.T2.S1). End users never see this primitive directly.
**Use Case**: When a user accidentally has files staged and runs decompose, the FR-M1e error (next task) will
list the offending paths via `StagedNames` so the user knows exactly what to `git restore --staged` — instead
of a generic "something is staged" message. This task supplies the path-listing primitive that enables that.
**Pain Points Addressed**: Today `StagedFileCount` gives only a COUNT; naming the offending paths requires a
new primitive that returns the paths. Decompose (FR-M1) requires NOTHING staged; without a path-listing
primitive the re-check error cannot be actionable.

## Why

- **FR-M1e primitive prerequisite**: the FR-M1e re-assertion (P2.M1.T2.S1) needs to enumerate staged paths to
  build an actionable "these files are staged, unstage them to decompose" error. `StagedFileCount` already
  runs the exact git command (`git diff --cached --name-only`) but discards the paths to return only a count —
  so the path data is already being fetched and thrown away. `StagedNames` is the zero-new-cost sibling that
  keeps the paths.
- **DRY / no new git invocation**: it reuses the proven `StagedFileCount` git-command + error-handling pattern
  (battle-tested across 9 scenarios in `stagedcount_test.go`), so there is no new git behavior to validate —
  only the return shape changes from `int` to `[]string`.
- **Scope discipline**: shipping the primitive separately from its consumer keeps the change tiny, fully
  unit-testable, and reviewable in isolation; the next task composes it into the decompose flow.

## What

Add `StagedNames(ctx context.Context) ([]string, error)` to the `Git` interface and implement it on
`*gitRunner` by cloning `StagedFileCount`'s git invocation and error handling, returning the staged paths
instead of a count. Add unit tests mirroring `stagedcount_test.go`. No other files change.

### Success Criteria
- [ ] `Git` interface (`internal/git/git.go`) declares `StagedNames(ctx context.Context) ([]string, error)`
      immediately after `StagedFileCount` (interface decl L163), with the contract doc comment.
- [ ] `*gitRunner` implements `StagedNames` in `internal/git/git.go` immediately after the `StagedFileCount`
      impl (L1349), cloning its `g.run(... "diff","--cached","--name-only")` + err/code branching + error
      message format, but collecting trimmed non-empty lines into `[]string`.
- [ ] `go build ./...` succeeds (the interface change is satisfied everywhere — the lone fake embeds `Git`).
- [ ] `internal/git/stagednames_test.go` exists and passes `go test ./internal/git/... -run StagedNames -race -v`
      covering: NothingStaged (empty), ThreeFiles (3 paths), AfterAddAll, IncludesDeletion, FilenameWithSpace
      (1 element, space does NOT split), UnbornRepoWithStaged, NotARepo (error), GitBinaryMissing (error),
      ContextCancelled (error).
- [ ] `go test -race ./...` green; `gofmt -l` clean; `go vet ./internal/git/...` clean.
- [ ] ONLY `internal/git/git.go` + new `internal/git/stagednames_test.go` changed (`git status --porcelain`).

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — exact line numbers for both the interface declaration and the impl to clone, the verbatim git command
+ error-handling pattern, the verbatim doc comment, the complete list of test fakes (one — embeds the interface,
no change), the exact test template file + helpers + scenarios, the no-new-imports fact, and the verified Go
validation commands.

### Documentation & References

```yaml
# MUST READ — the codebase-specific findings for THIS item (verified line numbers + the no-change fake proof).
- docfile: plan/015_b461e4720495/P2M1T1S1/research/findings.md
  why: "§0 exact surface (interface L163, impl L1331, run L476); §1 the FULL StagedFileCount impl to clone;
        §2 the run() contract (err-first unwrap, code!=0 error format); §3 no-new-imports proof; §4 the
        exhaustive fake search — ONLY contentionFakeGit, it EMBEDS git.Git ⇒ NO method addition (the G8
        embedded-interface pattern); §5 the test template + scenarios + helpers; §6 the consumer is the NEXT
        task (do NOT touch decompose.go); §7 validation commands; §8 scope fence; §9 no name collision."

# MUST READ — the interface to extend and the impl to clone (both in one file).
- file: internal/git/git.go
  why: "L96 `type Git interface {`; L163 `StagedFileCount(ctx context.Context) (int, error)` — INSERT the new
        StagedNames decl on the line immediately AFTER it (before RevParseTree's comment at L165). L1310-1349
        the StagedFileCount comment block + impl — CLONE for StagedNames (same g.run call, same err/code
        branching, same error message; replace the count loop with a path-collecting loop). L476 the run()
        helper signature."
  pattern: "Every read-only gitRunner method follows: g.run(ctx, g.workDir, args...) → err!=nil return early →
            code!=0 return fmt.Errorf(...) → parse stdout. Mirror StagedFileCount EXACTLY (it is the closest
            sibling — same git command, same SIMPLE code!=0 branch form, NOT HasStagedChanges' switch form)."
  gotcha: "Do NOT add `--quiet` to the git command — it suppresses the path list and would make StagedNames
           ALWAYS return empty. Do NOT branch on a specific exit code — branch on `code != 0` (convention G5).
           Do NOT add a StagedNames method to contentionFakeGit — it embeds git.Git and is auto-satisfied."

# MUST READ — the test template (clone its scenarios, assert paths not counts).
- file: internal/git/stagedcount_test.go
  why: "The canonical `git diff --cached --name-only` test file. Mirror its 9 scenarios (NothingStaged,
        ThreeFiles, AfterAddAll, IncludesDeletion, FilenameWithSpace, UnbornRepoWithStaged, NotARepo,
        GitBinaryMissing, ContextCancelled) but assert the RETURNED PATHS instead of a count. Same `package
        git`, same helpers, same error-substring assertions."
  pattern: "Test func naming `TestStagedNames_<Scenario>` (mirror `TestStagedFileCount_<Scenario>`). Use
            len(names)==0 for the nothing-staged case (robust to nil OR []string{}); use the shared helpers
            initRepo/makeEmptyCommit/writeFile/stageFile (defined in git_test.go / revparse_test.go /
            committree_test.go)."
  gotcha: "FilenameWithSpace MUST assert exactly ONE element (`sub/has space.txt`) — proves spaces do NOT split
           a path under --name-only (the embedded-newline inflation is an accepted, untested limitation, same
           as StagedFileCount)."

# CONTEXT — the shared test helpers (already defined in package git; just call them).
- file: internal/git/git_test.go
  why: "L13 `initRepo(t, dir)` — `git init` + identity config in a temp dir."
- file: internal/git/revparse_test.go
  why: "L24 `makeEmptyCommit(t, dir, msg)` — a commit with no content (establishes HEAD)."
- file: internal/git/committree_test.go
  why: "L31 `writeFile(t, dir, name, body)` and L39 `stageFile(t, dir, name)` — create + `git add` a file."

# CONTEXT — the ONE fake implementing Git (DO NOT EDIT — embedding satisfies the new method).
- file: internal/cmd/lock_contention_test.go
  why: "L18 `type contentionFakeGit struct { git.Git; ... }` EMBEDS the git.Git interface and overrides ONLY
        WriteTree. Adding a method to the Git interface is auto-satisfied by the embedded interface value;
        the struct needs NO new method (the L15 comment explicitly says 'Uncalled methods are nil'). VERIFY
        with `go build ./...` — it will compile. Do NOT add a StagedNames method to this struct."

# CONTEXT — where the consumer lives (NEXT task; do NOT edit here, but understand the contract).
- file: internal/decompose/decompose.go
  why: "L141 `func Decompose(...)`. L119-122 the PRECONDITION comment ('Decompose does NOT re-check this') —
        FR-M1e (P2.M1.T2.S1) adds the re-check and calls deps.Git.StagedNames(ctx) to name offending paths.
        This task ships the primitive so that next task can consume it; decompose.go is OUT OF SCOPE here."

# CONTEXT — PRD provenance (the FR the primitive serves; read-only).
- docfile: plan/015_b461e4720495/prd_snapshot.md
  section: "§9.14 (FR-M1 trigger: decomposition activates iff HasStagedChanges is false; FR-M1e is the
            re-assertion that names offending staged paths). §13.6.1 (the trigger model)."
  why: "Establishes WHY StagedNames exists: FR-M1e's error must NAME the staged paths, so the primitive
        returns []string (StagedFileCount's count is insufficient for an actionable error)."
```

### Current Codebase tree (relevant slice)

```bash
# READ-ONLY references:
internal/git/git.go                  # EDIT — interface decl (after L163) + *gitRunner impl (after L1349)
internal/git/stagedcount_test.go     # READ-ONLY — the test template to clone
internal/git/git_test.go             # READ-ONLY — initRepo helper
internal/git/revparse_test.go        # READ-ONLY — makeEmptyCommit helper
internal/git/committree_test.go      # READ-ONLY — writeFile / stageFile helpers
internal/cmd/lock_contention_test.go # READ-ONLY — contentionFakeGit (embeds git.Git; NO edit needed)
internal/decompose/decompose.go      # READ-ONLY — consumer is NEXT task (P2.M1.T2.S1); OUT OF SCOPE
Makefile                             # READ-ONLY — test/coverage/lint targets
```

### Desired Codebase tree with files to be added/edited

```bash
internal/git/git.go               # EDIT — +StagedNames interface decl (after L163) + +*gitRunner impl (after L1349)
internal/git/stagednames_test.go  # NEW  — unit tests mirroring stagedcount_test.go (9 scenarios, assert paths)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the lone Git fake EMBEDS the interface — do NOT add a method to it): contentionFakeGit
// (lock_contention_test.go:18) is `struct { git.Git; ... }`. Embedding auto-satisfies every Git method,
// including the new StagedNames. Adding a concrete StagedNames method there is dead code and WRONG.
// The contract's step (c) "update ALL test fakes/mocks" is satisfied by VERIFYING via `go build ./...`
// (it compiles) — there is genuinely nothing to add. (G8 embedded-interface pattern.)

// CRITICAL (mirror StagedFileCount's git command EXACTLY — do NOT add --quiet): the command is
// `git diff --cached --name-only`. Adding `--quiet` would suppress the path list and make StagedNames
// ALWAYS return empty. The err/code branching is the SIMPLE form (code != 0 → error), NOT
// HasStagedChanges' switch form (which treats exit 1 as "changes exist"). --name-only never exits 1 for
// "changes present" — it exits 0 and PRINTS the paths.

// CRITICAL (err-first unwrap, then code!=0 — the run() contract): run() returns (stdout, stderr, code, err).
// `if err != nil { return nil, err }` FIRST (git missing / cancelled / start failure; code is -1 here).
// THEN `if code != 0 { return nil, fmt.Errorf("git diff --cached --name-only: failed (exit %d): %s",
// code, strings.TrimSpace(stderr)) }`. Branch on `code != 0`, NEVER a specific code (G5: non-repo=129,
// corrupt=128 are both errors).

// GOTCHA (empty output → empty slice, robust assertion): when nothing is staged, `git diff --cached
// --name-only` prints nothing → `strings.Split("", "\n")` yields `[""]` → after filtering empties the
// result is either `[]string{}` (if built via append from a non-nil literal) or `nil` (if returned as the
// zero value). The contract accepts "nil/empty". The TEST must assert `len(names) == 0`, NOT
// `names == nil`, to be robust to either.

// GOTCHA (no new imports): git.go already imports context, fmt, strings. StagedNames uses only those.
// Do NOT touch the import block.

// GOTCHA (placement for doc-comment coherence): the long comment block ABOVE StagedFileCount (L~1310-1330)
// explains the --name-only (not --quiet) choice and the newline-split rationale — it applies equally to
// StagedNames. Place StagedNames IMMEDIATELY AFTER StagedFileCount so that explanatory comment covers the
// pair; give StagedNames its own short doc comment noting it returns the PATHS (where StagedFileCount
// discards them) and naming FR-M1e as the consumer.
```

## Implementation Blueprint

### Data models and structure

None — this is a pure read-only git primitive returning `[]string`. No structs, no schemas. The "data model"
is the method signature `StagedNames(ctx context.Context) ([]string, error)` and the verbatim doc comment
(both in the contract).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/git/git.go — add StagedNames to the Git INTERFACE (after L163)
  - LOCATE the interface declaration `StagedFileCount(ctx context.Context) (int, error)` at L163 (it is the
    line immediately before the `// RevParseTree returns the tree SHA of a commit-ish` comment at L165).
  - INSERT immediately AFTER the StagedFileCount signature (and its preceding 2-line doc comment stays above
    StagedFileCount) a new doc comment + signature:
        // StagedNames returns the paths of files currently staged (git diff --cached --name-only).
        // Returns nil/empty if nothing is staged. Used by FR-M1e to name offending staged paths in the
        // decompose re-check error.
        StagedNames(ctx context.Context) ([]string, error)
  - NAMING: StagedNames (CamelCase, matches StagedFileCount sibling).
  - PLACEMENT: inside the `type Git interface {` block (L96..), grouped with StagedFileCount.
  - PRESERVE: the StagedFileCount doc comment + signature (L160-163) UNCHANGED; only ADD below it.

Task 2: EDIT internal/git/git.go — implement StagedNames on *gitRunner (after L1349, i.e. after StagedFileCount impl)
  - LOCATE the end of `func (g *gitRunner) StagedFileCount` (the `return count, nil\n}` at ~L1349) and
    INSERT the new method immediately after it.
  - IMPLEMENT by cloning StagedFileCount (L1331-1349): SAME `g.run(ctx, g.workDir, "diff", "--cached",
    "--name-only")`, SAME `if err != nil { return nil, err }`, SAME `if code != 0 { return nil,
    fmt.Errorf("git diff --cached --name-only: failed (exit %d): %s", code, strings.TrimSpace(stderr)) }`.
  - DIFFERENCE: replace the count loop with a path-collecting loop:
        var names []string
        for _, line := range strings.Split(stdout, "\n") {
            if t := strings.TrimSpace(line); t != "" {
                names = append(names, t) // trailing newline → final "" skipped; empty output → nil/empty
            }
        }
        return names, nil
  - ADD a brief doc comment above the func noting it returns the PATHS (StagedFileCount discards them) and
    references FR-M1e + the shared --name-only rationale (point at StagedFileCount's comment block above).
  - NAMING: `func (g *gitRunner) StagedNames(ctx context.Context) ([]string, error)`.
  - DEPENDENCIES: none (context/fmt/strings already imported).
  - PLACEMENT: directly after StagedFileCount impl, before RevParseTree impl (~L1351).

Task 3: VERIFY the lone Git fake needs NO change (do NOT edit — embedding satisfies it)
  - RUN `go build ./...` — it MUST compile. The interface now has StagedNames; `contentionFakeGit`
    (lock_contention_test.go:18) embeds `git.Git` so it is auto-satisfied. If (and only if) `go build`
    reports a compile error naming a type that does NOT embed git.Git, add the minimal method to THAT type
    only — but the exhaustive search (findings §4) found NO such type, so this branch is not expected to fire.
  - DO NOT add a StagedNames method to contentionFakeGit — it would be dead code (the embedded interface
    already provides it; G8 pattern).

Task 4: CREATE internal/git/stagednames_test.go — unit tests mirroring stagedcount_test.go
  - FILE: `internal/git/stagednames_test.go`, `package git` (same package — uses shared helpers).
  - IMPORTS: context, errors, os, path/filepath, strings, testing (mirror stagedcount_test.go).
  - IMPLEMENT `TestStagedNames_<Scenario>` for each of the 9 scenarios in findings §5, mirroring
    `TestStagedFileCount_<Scenario>` setups exactly but asserting the RETURNED PATHS:
      * NothingStaged: `len(names)==0` (assert len, NOT ==nil).
      * ThreeFiles: names == [a.go, b.go, c.go] (or compare via a set/sort to be order-robust if desired,
        but git emits in a deterministic order; a direct slice compare is acceptable — if flaky, sort both).
      * AfterAddAll: names contains a.go and b.go (len==2).
      * IncludesDeletion: names == [a.go] (len==1).
      * FilenameWithSpace: names == ["sub/has space.txt"] (len==1 — space does NOT split under --name-only).
      * UnbornRepoWithStaged: names == [f.go].
      * NotARepo: err non-nil, strings.Contains(err.Error(), "git diff --cached --name-only: failed"); names empty.
      * GitBinaryMissing: t.Setenv("PATH",""); err contains "git binary not found".
      * ContextCancelled: pre-cancel ctx; errors.Is(err, context.Canceled).
  - FOLLOW pattern: stagedcount_test.go (helper usage: initRepo/makeEmptyCommit/writeFile/stageFile; error-
    substring assertions; t.TempDir()).
  - NAMING: TestStagedNames_NothingStaged, TestStagedNames_ThreeFiles, … (mirror the count tests).
  - COVERAGE: all public behavior — empty, happy path, space-in-name, unborn, and all 3 error paths.
  - PLACEMENT: internal/git/stagednames_test.go (new file; matches stagedcount_test.go naming).

Task 5: VALIDATE — build, targeted test, full test, format, vet, coverage, scope
  - go build ./...
  - go test ./internal/git/... -run StagedNames -race -v
  - go test -race ./...
  - gofmt -l internal/git/git.go internal/git/stagednames_test.go   # must print nothing
  - go vet ./internal/git/...
  - git status --porcelain   # ONLY the two files
```

### Implementation Patterns & Key Details

```go
// PATTERN (the gitRunner StagedNames impl — clone of StagedFileCount with a path-collecting loop):
// StagedNames returns the PATHS StagedFileCount discards. Same git command, same error contract.
func (g *gitRunner) StagedNames(ctx context.Context) ([]string, error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "diff", "--cached", "--name-only")
	if err != nil {
		return nil, err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code != 0 {
		return nil, fmt.Errorf("git diff --cached --name-only: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	var names []string
	for _, line := range strings.Split(stdout, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			names = append(names, t) // trailing newline → final "" skipped; empty output → nil/empty
		}
	}
	return names, nil
}

// CRITICAL: the interface decl doc comment is VERBATIM from the contract:
//   // StagedNames returns the paths of files currently staged (git diff --cached --name-only).
//   // Returns nil/empty if nothing is staged. Used by FR-M1e to name offending staged paths in the
//   // decompose re-check error.
//   StagedNames(ctx context.Context) ([]string, error)

// PATTERN (test assertion robustness — assert len, not nil, for the empty case):
//   names, err := g.StagedNames(ctx)
//   if err != nil { t.Fatalf(...) }
//   if len(names) != 0 { t.Fatalf("names = %v, want empty", names) }   // NOT: if names != nil
```

### Integration Points

```yaml
INTERFACE (internal/git/git.go):
  - add to: the `type Git interface {` block (L96), immediately after `StagedFileCount` (L163).
  - pattern: "StagedNames(ctx context.Context) ([]string, error)" with the contract doc comment.
IMPL (internal/git/git.go):
  - add to: *gitRunner methods, immediately after StagedFileCount impl (~L1349).
  - pattern: clone StagedFileCount's g.run + err/code branching; collect paths into []string.
TESTS (internal/git/stagednames_test.go):
  - new file, package git; mirror stagedcount_test.go scenarios asserting paths.
DOWNSTREAM (NONE in this task):
  - the consumer is P2.M1.T2.S1 (FR-M1e) in internal/decompose/decompose.go:141 — OUT OF SCOPE here.
  - every test using git.New(repo) gets StagedNames automatically (real *gitRunner).
  - contentionFakeGit (the only fake) embeds git.Git → auto-satisfied (no edit).
NO config / routes / migrations / env vars — pure internal library primitive.
```

## Validation Loop

### Level 1: Build & format (Immediate Feedback)

```bash
# Compiles the whole module — PROVES the interface change breaks nothing (the lone fake embeds Git).
go build ./...
# Expected: no output (success). If it errors naming a type missing StagedNames, that type does NOT embed
# git.Git — but findings §4's exhaustive search found no such type, so this should not fire.

# Formatting check — must print NOTHING.
gofmt -l internal/git/git.go internal/git/stagednames_test.go
# Expected: empty output. If a file is listed, run `gofmt -w` on it and re-check.
```

### Level 2: Unit Tests (the new primitive)

```bash
# Targeted: the new StagedNames tests, race-enabled.
go test ./internal/git/... -run StagedNames -race -v
# Expected: all TestStagedNames_* pass (9 scenarios: NothingStaged, ThreeFiles, AfterAddAll,
# IncludesDeletion, FilenameWithSpace, UnbornRepoWithStaged, NotARepo, GitBinaryMissing, ContextCancelled).

# Full git package (regression — StagedFileCount and friends still pass).
go test ./internal/git/... -race
# Expected: PASS. Confirms the interface/impl additions didn't disturb sibling tests.
```

### Level 3: Whole-repo regression + static checks

```bash
# Whole repo (make test) — catches any downstream compile/test break from the interface change.
go test -race ./...
# Expected: PASS. (contentionFakeGit embeds git.Git, so internal/cmd compiles; decompose/generate/hook
# tests use the real git.New, so they carry StagedNames automatically.)

# Static checks.
go vet ./internal/git/...
# Expected: clean.

# Coverage gate (PRD §20.3: ≥85% on internal/git). The new method + tests keep the package above the bar.
make coverage-gate
# Expected: PASS (the new StagedNames has full test coverage from Task 4).
```

### Level 4: Scope guard

```bash
# ONLY the two intended files changed.
git status --porcelain
# Expected: M internal/git/git.go  AND  ?? internal/git/stagednames_test.go  (nothing else).

# Guard: no out-of-scope / forbidden files touched.
git diff --name-only | grep -vE '^internal/git/git\.go$' && echo "FAIL: unexpected changed file" || echo "OK: only git.go edited"
git status --porcelain | grep -E 'decompose\.go|PRD\.md|plan/|tasks\.json|prd_snapshot|delta_prd|lock_contention_test' && echo "FAIL: forbidden file touched" || echo "OK: scope clean"
# Expected: "OK: only git.go edited" and "OK: scope clean". contentionFakeGit (lock_contention_test.go)
# MUST be untouched (it embeds the interface — no method added).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` succeeds (interface change breaks nothing — contentionFakeGit embeds git.Git)
- [ ] `go test ./internal/git/... -run StagedNames -race -v` passes (9 scenarios)
- [ ] `go test -race ./...` passes (whole-repo regression — `make test`)
- [ ] `gofmt -l internal/git/git.go internal/git/stagednames_test.go` prints nothing
- [ ] `go vet ./internal/git/...` clean
- [ ] `make coverage-gate` passes (≥85% on internal/git; new method fully covered)

### Feature Validation
- [ ] `Git` interface declares `StagedNames(ctx context.Context) ([]string, error)` after StagedFileCount (L163)
      with the contract doc comment
- [ ] `*gitRunner.StagedNames` clones StagedFileCount's git invocation + error handling, returns the paths
- [ ] `deps.Git.StagedNames(ctx)` returns `[]string` of staged paths; empty when nothing staged; non-nil error
      on non-repo / git-missing / cancelled (same error semantics as StagedFileCount)
- [ ] Tests cover empty / happy / space-in-name / unborn / all 3 error paths
- [ ] The FilenameWithSpace test asserts exactly ONE element (spaces do NOT split a path under --name-only)

### Scope-Boundary Validation
- [ ] `git status --porcelain` shows ONLY `internal/git/git.go` + new `internal/git/stagednames_test.go`
- [ ] `internal/cmd/lock_contention_test.go` (contentionFakeGit) UNCHANGED — it embeds git.Git, no method added
- [ ] `internal/decompose/decompose.go` UNCHANGED — the FR-M1e consumer is P2.M1.T2.S1 (NEXT task)
- [ ] NO edit to PRD.md, plan/**, tasks.json, prd_snapshot.md, delta_prd.md, or any other source file

---

## Anti-Patterns to Avoid

- ❌ Don't add a `StagedNames` method to `contentionFakeGit` — it EMBEDS `git.Git`; the embedded interface
  auto-satisfies the new method (G8 pattern). Adding one is dead code. Verify via `go build ./...` instead.
- ❌ Don't add `--quiet` to the git command — it suppresses the path list and makes StagedNames ALWAYS return
  empty. Use the EXACT `git diff --cached --name-only` StagedFileCount uses.
- ❌ Don't use HasStagedChanges' switch-form error branching — StagedNames uses the SIMPLE form (`code != 0 →
  error`), byte-identical to StagedFileCount. `--name-only` exits 0 and PRINTS paths when changes exist; it
  never uses exit 1 as a signal.
- ❌ Don't branch on a specific exit code (128/129) — branch on `code != 0` (convention G5).
- ❌ Don't assert `names == nil` for the nothing-staged case — assert `len(names) == 0`. Empty output yields
  either `nil` or `[]string{}` depending on append-from-zero; both are valid per the contract.
- ❌ Don't touch `internal/decompose/decompose.go` — the FR-M1e re-check that CONSUMES StagedNames is
  P2.M1.T2.S1 (the NEXT task). This task ships ONLY the primitive.
- ❌ Don't add new imports — context/fmt/strings are already imported in git.go.
- ❌ Don't duplicate the long `--name-only` rationale comment — place StagedNames right after StagedFileCount
  so the existing explanatory comment covers the pair; give StagedNames only a short doc comment.
