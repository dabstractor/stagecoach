# P2.M2.T1.S1 — Empirical Findings (RevParseTree + ReadTree)

All facts below were **verified empirically** on the host git (`git version 2.54.0`) in throwaway
repos. These are the authoritative wire-format facts the implementation + tests must match.

## 1. `git rev-parse <ref>^{tree}` — the four behavioral shapes

`<ref>` is a "commit-ish": `HEAD`, a branch name, or a full/abbreviated commit SHA. The `^{tree}`
suffix **peels** the commit-ish to its tree object and prints that tree's SHA.

| Scenario | stdout | stderr | exit | Correct return value |
|---|---|---|---|---|
| unborn repo, `ref=HEAD` | `HEAD^{tree}\n` (the LITERAL string!) | `fatal: ambiguous argument 'HEAD^{tree}': unknown revision…` | **128** | `("", nil)` — defensive (callers gate on isUnborn) |
| born repo, `ref=HEAD` | `<treeSHA>\n` | `""` | 0 | `(strings.TrimSpace(stdout), nil)` |
| born repo, `ref=<commitSHA>` | `<treeSHA>\n` | `""` | 0 | `(strings.TrimSpace(stdout), nil)` |
| bogus/invalid ref (`notavalidsha`) | the literal arg | `fatal: Not a valid object name…` | **128** | `("", nil)` — same 128 path (accepted; caller's bug) |

**CRITICAL — branch on exit CODE, not stdout.** On an unborn repo `rev-parse HEAD^{tree}` prints the
**literal** string `HEAD^{tree}` to stdout (verified — identical latent-bug shape to `rev-parse HEAD`
printing `HEAD\n`, which is FINDING 1 of the RevParseHEAD research). A naive `if stdout == ""` check
would WRONG: stdout is non-empty on unborn. **Branch on `code == 128`**, exactly as RevParseHEAD does.

**CRITICAL — `^{tree}` must be ONE argv element.** The peeling suffix `^{tree}` attaches to `<ref>`.
It MUST be passed as the single argument `ref+"^{tree}"`, NOT as two separate `[]string` elements.
(`git rev-parse HEAD "^{tree}"` would treat `^{tree}` as a second positional and fail.) run() already
takes `args ...string` and joins them into one `exec.CommandContext` argv; so call it as
`g.run(ctx, g.workDir, "rev-parse", ref+"^{tree}")`. No shell is involved (PRD §19), so `{`/`}` are
not glob-expanded — the suffix reaches git verbatim.

**Why 128 → ("", nil) and not an error (the contract's "defensive" choice).** 128 is git's catch-all
for "couldn't resolve the object". For `ref=HEAD` on an unborn repo this is the EXPECTED, non-error
signal (mirrors RevParseHEAD/RecentMessages/RecentSubjects/CommitCount — ALL treat `code == 128` as
"unborn is not an error"). For a genuinely-bad ref it is also 128; the contract deliberately collapses
both into `("", nil)` because **callers gate on isUnborn first** (the orchestrator calls
RevParseTree("HEAD") only after RevParseHEAD reports a born repo, or handles the empty return). Do NOT
try to disambiguate "unborn HEAD" from "bad SHA" by parsing stderr — stderr text varies by version.

## 2. `git read-tree <tree>` — a pure index MUTATION

`read-tree` **replaces** the entire index with the contents of `<tree>` (default, no `-m`/`--merge`).
It does NOT touch HEAD or any ref. It does NOT touch the working tree.

| Scenario | stdout | stderr | exit | Correct return value |
|---|---|---|---|---|
| valid tree | `""` | `""` | 0 | `nil` |
| invalid/non-existent tree SHA (`0000…0000`) | `""` | `fatal: failed to unpack tree object 0000…` | **128** | non-nil error |
| not a valid object name (`notavalidsha`) | `""` | `fatal: Not a valid object name notavalidsha` | **128** | non-nil error |
| non-repo directory | `""` | `fatal: not a git repository…` | **128** | non-nil error |

**CRITICAL — ReadTree is a MUTATION → ALL non-zero exits are errors (like AddAll/WriteTree/CommitTree).**
Unlike the read methods (RevParseHEAD/RevParseTree/RecentMessages/CommitCount) that special-case 128 as
"unborn is not an error", ReadTree treats **every** non-zero exit as a real error. This is the
established convention for index/ref-mutating methods: `AddAll`'s doc comment states exactly this
("AddAll treats ALL non-zero exits as errors (it is a mutation, structurally identical to
WriteTree/CommitTree)"). ReadTree is structurally identical. Branch on `code != 0` (NOT `code == 128`).

**CRITICAL — read-tree REPLACES the index (verified).** Loading `HEAD~1^{tree}` into a repo whose
index holds `a.txt`+`b.txt` leaves the index holding ONLY `a.txt` (the older tree). This is the
behavior the arbiter's mid-chain rebuild (P3.M3.T2) RELIES ON: it `read-tree`s a base, folds the
leftovers in via `git add`, then `write-tree`s. The method has no options/flags in this contract —
plain `git read-tree <tree>`. (The PRD §13.6.5 arbiter contract says "for each j, read-tree the
appropriate base, fold the leftovers in at j==i" — that orchestration logic lives in the arbiter, NOT
here; ReadTree is the single primitive.)

## 3. The run() invariant (unchanged helper — CONSUME, do not modify)

Both methods delegate to the existing `(*gitRunner).run(ctx, repo, args...)` helper (NO stdin needed —
neither `rev-parse` nor `read-tree` reads stdin). run()'s contract (from git.go):

- `err != nil` ⟺ infrastructural failure ONLY (LookPath miss / context cancel / start-or-I/O failure),
  with `exitCode == -1`. Git's own non-zero exits do NOT set err.
- Non-zero git exit ⟹ `(stdout, stderr, exitCode, nil)` — err is **nil**, the exit code is the signal.

So both implementations follow the universal 4-step shape every other method uses:
```go
stdout, stderr, code, err := g.run(ctx, g.workDir, <args>...)
if err != nil {
    return <zero>, err            // infrastructural — propagate UNWRAPPED (context.Canceled survives errors.Is)
}
if code == <signal> { ... }       // RevParseTree: code==128 → ("", nil); ReadTree: no signal branch
if code != 0 {
    return <zero>, fmt.Errorf("git <cmd>: failed (exit %d): %s", code, strings.TrimSpace(stderr))
}
return <success>, nil
```

runWithInput is NOT needed (no stdin). Do NOT modify run() or runWithInput().

## 4. Context-cancel and git-binary-missing (the two cross-cutting error paths)

Verified across ALL existing methods — identical for these two:
- **GitBinaryMissing:** `t.Setenv("PATH","")` makes run()'s `exec.LookPath("git")` fail → err is
  non-nil and contains `"git binary not found"`; exitCode is -1. BOTH methods: assert err != nil and
  `strings.Contains(err.Error(), "git binary not found")`.
- **ContextCancelled:** pre-cancel a ctx (`ctx, cancel := context.WithCancel(ctx); cancel()`) → run()
  returns `err = context.Canceled` (via the `cerr := ctx.Err()` branch); exitCode -1. BOTH methods:
  assert `errors.Is(err, context.Canceled)`.

These two tests are MANDATORY for every new method (they appear in revparse_test.go, writetree_test.go,
committree_test.go, addall_test.go, stagediff_test.go, …). They prove the infrastructural-failure
contract holds for the new methods too.

## 5. Test fixtures available for reuse (package `git`, no new helpers needed)

| Helper | Defined in | Purpose |
|---|---|---|
| `initRepo(t, dir)` | git_test.go | `git init` + repo-local user.name/user.email |
| `makeEmptyCommit(t, dir, msg)` | revparse_test.go | empty commit via env-var identity (establishes HEAD) |
| `writeFile(t, dir, name, body)` | committree_test.go | create a file (0644) |
| `stageFile(t, dir, name)` | committree_test.go | `git add <name>` |
| `writeTreeOf(t, dir)` | committree_test.go | `git write-tree` → trimmed TREE_SHA (independent oracle) |
| `headSHA(t, dir)` | committree_test.go | `git rev-parse HEAD` → trimmed SHA (independent oracle) |
| `setIdentityConfig(t, dir)` | committree_test.go | repo-local user.name/user.email (alt to env vars) |
| `minGitEnv()` | revparse_test.go | minimal PATH+HOME env slice |

All are package-level (same package `git`) ⇒ visible from a new test file with NO imports beyond the
standard set. **No new fixture helpers are required** for either method.

**Independent-oracle convention:** existing tests verify a method's SHA output by comparing against an
INDEPENDENT git invocation (e.g. `writeTreeOf(t, repo)` or `headSHA(t, repo)` via `exec.Command`), NOT
by re-calling the method under test. RevParseTree tests MUST compare the returned tree SHA against
`writeTreeOf(t, repo)` (the tree of the staged index) after staging + committing — proving `^{tree}`
peeling yields the SAME SHA git itself computes. ReadTree tests verify index mutation via an
independent `git ls-files` (like addall_test.go's `git diff --cached --name-only` oracle).

## 6. Test-file placement convention (one file per method)

The package uses **one test file per Git interface method**: `revparse_test.go` (RevParseHEAD),
`writetree_test.go`, `committree_test.go`, `updateref_test.go`, `difftree_test.go`, `stagediff_test.go`,
`hasstaged_test.go`, `recentmessages_test.go`, `recentsubjects_test.go`, `commitcount_test.go`,
`addall_test.go`, `stagedcount_test.go`. So:
- **RevParseTree tests → NEW file `internal/git/revparsetree_test.go`** (distinct from
  `revparse_test.go`, which owns RevParseHEAD + the shared `makeEmptyCommit`/`minGitEnv` helpers).
- **ReadTree tests → NEW file `internal/git/readtree_test.go`**.
Test functions MUST have distinct names from all existing ones (prefix `TestRevParseTree_*` /
`TestReadTree_*`). This also avoids any file-level merge conflict with the parallel P2.M2.T1.S2
(TreeDiff) work item, which would touch a `treediff_test.go`.

## 7. Scope / frozen-boundary facts

- **The `Git` interface is EDITED** (add 2 methods + doc comments) — this is in scope and REQUIRED
  (the work item says "New methods are added to the interface AND the gitRunner struct"). Unlike
  P2.M1.T1.S2 (which was forbidden to touch the interface), this task's WHOLE POINT is adding the two
  methods to the interface. The existing `// Method ownership` comment block in git.go lists the v1
  methods but NOT these — leave it (it is a v1 ownership map; do not edit it) OR append a note. Prefer
  leaving it untouched (it documents v1 provenance; the new doc comments are self-documenting).
- **`run()` / `runWithInput()` are CONSUMED, not modified.** No helper changes.
- **No new dependencies.** go.mod/go.sum UNCHANGED. Only stdlib already imported (`context`, `fmt`,
  `strings`) is used.
- **`internal/git/binary.go` is NOT touched** by this task (binary filtering is P2.M1; ReadTree/
  RevParseTree do not diff, so no binary handling). TreeDiff (S2) is the one that reuses binary.go.
- **Nothing outside `internal/git/` is modified.** No caller wires these yet — RevParseTree is consumed
  by the decompose pipeline (P3.M2.T4 for tree[-1]) and ReadTree by the arbiter (P3.M3.T2). Wiring is
  out of scope; this task only adds + tests the two primitives.

## 8. Validation commands (verified to exist in this repo)

- `gofmt -w internal/git/git.go internal/git/revparsetree_test.go internal/git/readtree_test.go`
- `go vet ./...`
- `golangci-lint run ./...` (linters: errcheck/gosimple/govet/ineffassign/staticcheck/unused)
- `go test -race ./internal/git/ -run "TestRevParseTree|TestReadTree" -v`
- `go test ./...` (full regression)
- `git diff --exit-code go.mod go.sum` ⇒ must be empty
- `git status --short` ⇒ EXACTLY: `M internal/git/git.go` + 2 new untracked test files (3 entries)
