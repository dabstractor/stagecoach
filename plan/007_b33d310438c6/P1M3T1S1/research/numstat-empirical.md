# numstat capture/parse — empirical git semantics + design decisions (P1.M3.T1.S1)

> Empirically verified against **git 2.54.0** (the project's floor is 2.20; the dev box runs 2.54.0) in a
> throwaway temp repo (2026-07-04). This resolves a **contradiction between the contract and an existing
> code comment**, and grounds the `numstatRows` parser + tests. Numbered for cross-reference from the PRP.

## §0 — The contradiction, resolved empirically

- `internal/git/git.go:686` (the `buildDiffArgs` doc comment) ASSERTS: *"binary.go's detectBinaryFiles/
  fileStatuses build their OWN argv and are NOT routed through buildDiffArgs — **-M would corrupt
  numstat's path column with `=>`/`{...}` rename notation** … sizing/status parsing stays simple."* — i.e.
  it claims OMITTING `-M` keeps numstat paths clean.
- The item contract ASSERTS the opposite: *"git 2.54 shows [`=>`] even without -M since renames default
  on."*

**The contract is correct; the comment is wrong (for git ≥ 2.9).** git flipped `diff.renames` to ON by
default in the 2.8–2.9 era (~2016); on git 2.54.0 a pure rename produces the `=>` notation in
`git diff --cached --numstat` **with OR without `-M`**. ⇒ `numstatRows` MUST resolve `=>`→destination
**unconditionally** (it cannot rely on the absence of `-M` to keep paths clean). (This also means
`detectBinaryFiles` has a LATENT bug — a renamed binary keys as `"old => new"` — but that is existing
behavior, NOT this task's scope; do not touch `detectBinaryFiles`.)

## §1 — Empirical numstat shapes (git 2.54.0)

| Change | `git diff --cached --numstat` (no -M) | `git diff --cached -M --numstat` |
|---|---|---|
| edit a.txt | `1\t1\ta.txt` | `1\t1\ta.txt` |
| **pure rename** (mv old.txt new.txt) | **`0\t0\told.txt => new.txt`** ← `=>` WITHOUT -M | `0\t0\told.txt => new.txt` |
| rename+edit (low similarity) | `1\t0\tlib.go` + `0\t1\tsrc.go` (delete+add) | same (delete+add; similarity < threshold) |
| **brace-collapse rename** (mv dir/a.go dir/b.go) | (not observed w/o -M) | **`0\t0\tdir/{a.go => b.go}`** |
| binary (bin.png) | `-\t-\tbin.png` | `-\t-\tbin.png` |

Fields are **TAB-separated** (`added\tdeleted\tpath`); `strings.SplitN(line, "\t", 3)` puts the whole path
(incl. spaces, `=>`, braces) in `fields[2]` — **tab-safe** by construction (spaces in paths never split a
field). Binary ⇒ both counts are `-`. A rename+edit below the similarity threshold falls back to a
delete+add pair (two rows) — the parser handles rows uniformly; `=>` only appears when git detects a rename.

## §2 — The `=>` / brace-collapse resolver (REQUIRED, pure function)

Resolve the numstat path field to the DESTINATION (`b/`) path:

- **no `=>`** → verbatim (normal add/modify/delete, or a path that happens to contain `{`/`}` but no rename).
- **simple** `old => new` → `new` (trimmed right of the first `=>`).
- **brace** `prefix{old => new}suffix` → `prefix` + `new` + `suffix` (git collapses the common prefix/suffix
  into the braces; verified `dir/{a.go => b.go}` → `dir/b.go`; the resolver also handles suffix-collapsed
  `dir/{a => b}.go` → `dir/b.go`, which git can emit).

Algorithm: if the field contains `=>`: find the first `{`…`}` (if present); destination = (text before `{`)
+ TrimSpace(right of `=>` inside the braces) + (text after `}`); with no braces, destination =
TrimSpace(right of `=>`). Factor this as a **pure `resolveNumstatPath(string) string`** so the brace forms
(which depend on git's collapse heuristics and are fiddly to reproduce via real git) are covered by
deterministic table tests independent of the integration suite.

## §3 — File placement: NEW `numstat.go` + `numstat_test.go` (do NOT touch binary.go/git.go)

- The contract: *"prefer a dedicated [function] to avoid coupling"* with `detectBinaryFiles`. A new file
  `internal/git/numstat.go` holds `type numstatRow`, the pure `resolveNumstatPath`, and the
  `(g *gitRunner) numstatRows` method. Keeps `binary.go` focused on binary detection.
- `numstatRows` is a `*gitRunner` method (like `detectBinaryFiles`/`fileStatuses` — **NOT on the `Git`
  interface**; git.go:686 confirms those are internal helpers that build their own argv). Adding a method
  on `*gitRunner` in a NEW file is legal Go (methods may live in any file of the package). ⇒ **no `Git`
  interface edit, no mock changes, no `git.go`/`binary.go` edit.**

## §4 — `--numstat` placement: BEFORE any `--` pathspec separator

`buildDiffArgs` produces `["diff", domain..., "-M", "-U<n>]` — **no `--`** (callers append
`-- <pathspecs>` themselves, e.g. `append(buildDiffArgs(opts,"--cached"), "--name-only","--","*.md")`).
The contract: *"compose with excludes the same way the aggregate diff does."* So a caller (the FR3i sizing
path, P1.M4.T2) may pass `-- <excludes>` inside `diffArgs`. If `numstatRows` simply appended `--numstat`,
it would land AFTER `--` and be swallowed as a pathspec. ⇒ `numstatRows` **inserts `--numstat` before the
first `--`** in `diffArgs` (if any), else appends. This is a strict superset of simple append (the no-`--`
case is identical) and makes excludes compose correctly. 6 lines; documented.

## §5 — `numstatRow` shape + binary + sort

`type numstatRow struct { Added, Deleted int; IsBinary bool; Path string }`. Binary (`added == "-"`) ⇒
`IsBinary=true`, `Added=Deleted=0` (the `-` is not a count). Non-binary ⇒ `strconv.Atoi` both counts.
`Path` is the destination (rename-resolved). Rows returned **sorted by Path** (deterministic output for
the skeleton/sizing consumers). `sort.Slice` by `Path`.

## §6 — Tests: pure table (resolver) + integration (numstatRows)

- **`resolveNumstatPath` pure table** (no git): `a.txt`→`a.txt`; `old => new`→`new`;
  `dir/{a.go => b.go}`→`dir/b.go`; `dir/{a => b}.go`→`dir/b.go`; `{old => new}`→`new`;
  `prefix{a => b}suffix`→`prefixbsuffix`; no-`=>`-with-braces `weird{name}.txt`→verbatim; spaces
  `my file.txt`→`my file.txt`. (The brace forms are deterministic HERE — no git dependency.)
- **`numstatRows` integration** (temp repo via `asRunner(New(repo))`, mirroring `binary_test.go`'s
  `detectBinaryFiles` tests): edit (counts); binary (`IsBinary`); pure rename (`git mv`, path resolved to
  destination — `=>` appears even without `-M` per §1); brace-collapse rename (`mkdir dir; git mv dir/a.go
  dir/b.go` with `-M`, expect `dir/b.go`); path with spaces; **empty diff** (nil/empty rows, no error).
  Each asserts the parsed row(s) + the sort order. The test calls `numstatRows` with the diffArgs that
  produce the shape (e.g. `-M` for the brace case; domain-only for edit/binary/empty).

## §7 — No conflict with the parallel work item

P1.M2.T3.S1 (FR3h index-line stripping) touches `internal/git/binary.go`, `internal/git/git.go`, and
`internal/git/stagediff_test.go`. This task creates **NEW** `numstat.go` + `numstat_test.go` and edits
NEITHER `binary.go` NOR `git.go`. Zero file overlap ⇒ no merge conflict; the two are fully independent.
(`stripIndexLines` lives in `git.go` and is the parallel task's concern; `numstatRows` never emits index
lines — numstat output has none — so the features don't interact.)
