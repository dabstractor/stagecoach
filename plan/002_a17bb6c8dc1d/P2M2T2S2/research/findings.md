# P2.M2.T2.S2 — WorkingTreeDiff: Research Findings

## §1. The contract (authoritative)

The work item, the architecture doc, AND `internal/git/binary.go`'s package doc ALL specify the
same thing:

- **Signature**: `WorkingTreeDiff(ctx context.Context, opts StagedDiffOptions) (string, error)`
  (work item CONTRACT §3; `architecture/binary_git_v2.md` §2 "WorkingTreeDiff").
- **Command**: `git diff` **WITHOUT** `--cached` (working-tree-vs-index). The work item says it
  twice ("`git diff` WITHOUT --cached", "`git diff` (no --cached)"); the architecture doc says
  "Uses: git diff (NO --cached flag) with binary filtering applied. Applies the same
  caps/excludes as StagedDiff."
- **Binary filtering**: same FR3c treatment as StagedDiff/TreeDiff (identical placeholder format).
- **`binary.go` package doc** (already shipped in P2.M1.T1.S1) explicitly lists the three FR3c
  consumers and the diffArgs for each:
  ```
  - StagedDiff (S2):     diffArgs = ["--cached"]
  - WorkingTreeDiff:      diffArgs = []
  - TreeDiff:             diffArgs = [treeA, treeB]
  ```
  `WorkingTreeDiff` ⇒ **empty diffArgs**. The `detectBinaryFiles(ctx)` and `fileStatuses(ctx)`
  helpers take a variadic `diffArgs ...string`, so calling them with NO extra args builds exactly
  `git diff --numstat` and `git diff --name-status` (working-tree domain). VERIFIED.

## §2. WorkingTreeDiff is a mechanical port of TreeDiff/StagedDiff

`internal/git/git.go` contains TWO ready reference implementations:
- `(*gitRunner).StagedDiff` (~line 392) — uses `--cached` everywhere.
- `(*gitRunner).TreeDiff` (~line 856, the parallel sibling, ALREADY SHIPPED) — uses `treeA treeB`
  everywhere instead of `--cached`.

WorkingTreeDiff is TreeDiff with the `treeA treeB` positionals DROPPED (and with no `--cached`).
The three-part structure is byte-identical in shape:

| Section | StagedDiff | TreeDiff | **WorkingTreeDiff** |
|---|---|---|---|
| md list | `diff --cached --name-only -- *.md *.markdown` | `diff <A> <B> --name-only -- *.md *.markdown` | **`diff --name-only -- *.md *.markdown`** |
| per-file | `diff --cached -- <file>` | `diff <A> <B> -- <file>` | **`diff -- <file>`** |
| binary set | `detectBinaryFiles(ctx, "--cached")` | `detectBinaryFiles(ctx, A, B)` | **`detectBinaryFiles(ctx)`** |
| statuses | `fileStatuses(ctx, "--cached")` | `fileStatuses(ctx, A, B)` | **`fileStatuses(ctx)`** |
| aggregate | `diff --cached -- <excl> :!*.md :!*.markdown <binExcl>` | `diff <A> <B> -- ...` | **`diff -- <excl> :!*.md :!*.markdown <binExcl>`** |

Everything else (cap defaults via `defaultMaxMDLines`/`defaultMaxDiffBytes`, `sort.Strings(binPaths)`,
the SEPARATE `binExcludes` slice, the markdown double-count excludes `:!*.md`/`:!*.markdown` appended
structurally, the truncation sentinels, the `strings.Builder` concatenation) is IDENTICAL. Copy
TreeDiff's body, delete the `treeA treeB` positionals, done.

## §3. Exit-code convention (simple branch — same as StagedDiff/TreeDiff)

`git diff` (WITHOUT `--quiet`) exits 0 whether or not there are changes (VERIFIED: empty working
tree → exit 0, empty stdout; dirty → exit 0, non-empty stdout). Exit 128 = bad pathspec / corrupt
repo = a REAL error. So WorkingTreeDiff uses the SIMPLE branch (`if code != 0 → error`), byte-
identical to StagedDiff/TreeDiff. **NO `--quiet`, NO 128-special-case** (this is the diff-list
convention, NOT HasStagedChanges' `--quiet` exit-inversion, and NOT RevParseHEAD's 128-as-unborn).

## §4. CRITICAL GOTCHA — `git diff` (no --cached) OMITS untracked files (per the contract)

VERIFIED empirically:
```
setup: committed seed.go; working tree has tracked-modified tracked.go, untracked untracked.go,
       untracked logo.png, untracked doc.md (NOTHING staged — the decompose trigger state)
git diff            → shows ONLY tracked.go (the tracked-modified file)
git diff --name-only → tracked.go
git diff --numstat  → 1\t0\ttracked.go
untracked.go        → NOT in plain git diff / numstat
```

`git diff` (no `--cached`, no `HEAD`) compares **working tree vs INDEX**, and git NEVER lists
untracked files in a `git diff` (untracked = not in the index = nothing to diff against). Only
**tracked-but-modified** and **tracked-but-deleted** files appear.

**This is the explicit, authoritative contract** — the work item says "`git diff` WITHOUT --cached
(working-tree-vs-index)" and the architecture doc says "git diff (NO --cached flag)". WorkingTreeDiff
implements that command faithfully. The untracked-files gap is a property of the `git diff` domain,
NOT a bug in this task. Resolution of any untracked-file visibility concern is OUT OF SCOPE for this
plumbing task — it would be a decompose-orchestrator (P3) decision (e.g. whether the planner also
needs `git ls-files --others`). The stager (FR-M5) is a TOOLED agent with full repo access and
discovers untracked files itself, so the pipeline is not blind to them.

➡ The PRP DOCUMENTS this prominently and includes ONE test (`TestWorkingTreeDiff_`
`UntrackedFilesOmitted`) that PINS the documented `git diff` domain so a future reader is not
surprised. It does NOT attempt to "fix" it (that would violate the contract).

## §5. Binary detection works in the working-tree domain (VERIFIED)

A **tracked binary MODIFIED in the working tree** (not staged) is detected identically to the staged
case:
```
committed logo.png (binary); modified logo.png in working tree (NOT staged)
git diff --numstat    → -\t-\tlogo.png     (content-sniff ⇒ binary)
git diff --name-status → M\tlogo.png       (status for the placeholder)
```
So `detectBinaryFiles(ctx)` + `fileStatuses(ctx)` produce the right set + statuses, and
`binaryPlaceholderLine("M", "logo.png")` ⇒ `"M\t[binary] logo.png"` — the FR3c placeholder, format
identical to the staged/tree paths. The user `BinaryExtensions` override flows through
`isBinaryByExtension(path, opts.BinaryExtensions)` exactly as in StagedDiff/TreeDiff.

## §6. Test design — the setup differs from StagedDiff/TreeDiff (working-tree deltas, not staged)

StagedDiff/TreeDiff tests create changes by `writeFile` + `stageFile` (the change lands in the
INDEX/trees). WorkingTreeDiff needs the change in the **working tree vs index**, so the idiom is:

  **commit a tracked baseline → then modify the file in the working tree (writeFile, NO stageFile).**

This keeps `index == HEAD` (nothing staged — the exact decompose trigger state) while producing a
working-tree delta that `git diff` will show:
```go
repo := t.TempDir(); initRepo(t, repo)
writeFile(t, repo, "code.go", "package main\n"); stageFile(t, repo, "code.go")
execGit(t, repo, "commit", "-m", "init")                       // tracked baseline; index==HEAD
writeFile(t, repo, "code.go", "package main\n// modified\n")   // WORKING-TREE delta (not staged)
g := New(repo); out, err := g.WorkingTreeDiff(ctx, StagedDiffOptions{})  // out shows code.go
```
For binary tests: commit a binary, then overwrite it in the working tree (no stage). For the
"clean working tree" test: commit a baseline and modify nothing. `execGit` (defined in
`revparsetree_test.go:115`) runs an arbitrary git command in the dir and returns stdout — use it
for the baseline `commit -m init` (`makeEmptyCommit` makes an ALLOW-EMPTY commit and does NOT stage
files, so it is the WRONG helper for establishing a tracked baseline).

## §7. Reusable test helpers (all package-level in `internal/git`, same package — do NOT redefine)

| helper | defined in | purpose |
|---|---|---|
| `initRepo(t, dir)` | `git_test.go:12` | git init + repo-local identity (every test starts here) |
| `writeFile(t, dir, name, body)` | `committree_test.go:31` | write a file |
| `stageFile(t, dir, name)` | `committree_test.go:39` | git add |
| `writeTreeOf(t, dir)` | `committree_test.go:48` | git write-tree → SHA (NOT needed for WorkingTreeDiff) |
| `makeEmptyCommit(t, dir, msg)` | `revparse_test.go:24` | allow-empty commit (NOT for baselines with files) |
| `execGit(t, dir, args...)` | `revparsetree_test.go:115` | run arbitrary git, return stdout (USE for baseline commit) |
| `sdManyLines(n)` | `stagediff_test.go:14` | n-line string (for markdown line-cap tests) |

All are visible from a new `workingtreediff_test.go` (package `git`). Redefining any ⇒ duplicate-
symbol compile error.

## §8. Scope boundaries (frozen / owned elsewhere — do NOT edit)

- `run()` / `runWithInput()` — CONSUMED.
- `detectBinaryFiles` / `fileStatuses` / `isBinaryByExtension` / `binaryPlaceholderLine` /
  `defaultBinaryExtensions` (in `binary.go`) — CONSUMED as-is. The package doc already names
  WorkingTreeDiff (`diffArgs = []`); do NOT modify `binary.go`.
- `StagedDiff`, `TreeDiff`, `StagedDiffOptions`, `defaultExcludes`, `EmptyTreeSHA`,
  `defaultMaxMDLines`, `defaultMaxDiffBytes` — CONSUMED, UNCHANGED.
- `StatusPorcelain` (P2.M2.T2.S1, parallel) — appends to the SAME interface + file. WorkingTreeDiff
  appends AFTER it; appending at the END of both minimizes merge friction.
- Decompose wiring (P3.M2.T2.S1 planner consumes this) — NO caller references yet; this task only
  adds the method + tests.
- go.mod / go.sum — UNCHANGED (stdlib only: context/fmt/sort/strings already imported in git.go).
- The `// Method ownership` provenance comment block — UNCHANGED.

## §9. Signature + role confirmed (architecture doc)

`architecture/binary_git_v2.md` §2:
```go
// WorkingTreeDiff returns the unstaged working-tree diff (planner input).
// Uses: git diff (NO --cached flag) with binary filtering applied.
// Applies the same caps/excludes as StagedDiff.
WorkingTreeDiff(ctx context.Context, opts StagedDiffOptions) (string, error)
```
Consumed by the decompose planner (P3.M2.T2.S1, PRD §13.6.2/FR-M3) as its diff input.
