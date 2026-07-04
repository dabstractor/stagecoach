---
name: "P1.M3.T1.S1 — numstat capture + parse (rename `=>`/brace notation, binary, destination-path keying): the shared per-file size/shape primitive for the FR3g compact skeleton and FR3i water-fill sizing — PRD §9.1 FR3g/FR3i"
description: |

  Land the FIRST subtask of the Compact numstat change skeleton (P1.M3.T1): a NEW `internal/git/numstat.go`
  exporting `(g *gitRunner) numstatRows(ctx, diffArgs ...string) ([]numstatRow, error)` — the parsed
  `git diff <diffArgs> --numstat` output, one `{Added, Deleted int; IsBinary bool; Path string}` per
  changed file, keyed on the DESTINATION path (rename-resolved), sorted by path. It is the shared per-file
  size/shape primitive consumed by S2 (FR3g skeleton prepend) and P1.M4.T2.S2 (FR3i water-fill sizing) —
  ONE git call, dual-use (the skeleton is both the model's completeness view AND the sizing input, FR3i).

  ⚠️ **#1 — the `=>` resolver is REQUIRED (a comment in the repo is WRONG; verified empirically).**
  `internal/git/git.go:686` claims omitting `-M` keeps numstat paths clean. That is FALSE on git ≥ 2.9
  (`diff.renames` defaulted ON in the 2.8–2.9 era): on git 2.54.0 a pure rename emits
  `0\t0\told.txt => new.txt` **with OR without `-M`** (verified in a temp repo, research numstat-empirical.md
  §0/§1). So `numstatRows` MUST resolve `=>`→destination UNCONDITIONALLY — it cannot rely on the absence of
  `-M`. (This also means `detectBinaryFiles` has a latent renamed-binary keying bug, but that is EXISTING
  behavior — do NOT touch `detectBinaryFiles`.) See research §0/§2.

  ⚠️ **#2 — handle BOTH rename forms: simple `old => new` and brace-collapsed `prefix{old => new}suffix`.**
  git collapses the common prefix/suffix into `{...}`: verified `dir/{a.go => b.go}` (→ `dir/b.go`) with
  `-M`; git can also emit suffix-collapsed `dir/{a => b}.go` (→ `dir/b.go`). The resolver: if the field
  contains `=>` and a `{…}`: destination = (before `{`) + TrimSpace(right of `=>` inside braces) + (after
  `}`); else destination = TrimSpace(right of `=>`); no `=>` → verbatim. Factor this as a PURE
  `resolveNumstatPath(string) string` so the brace forms (fiddly to reproduce via real git) get deterministic
  table tests. See research §2.

  ⚠️ **#3 — NEW file `numstat.go`; do NOT edit `binary.go` or `git.go`.** The contract: "prefer a dedicated
  [function] to avoid coupling" with `detectBinaryFiles`. `numstat.go` holds `type numstatRow`, the pure
  `resolveNumstatPath`, and the `(g *gitRunner) numstatRows` method. `numstatRows` is a `*gitRunner` method
  (like `detectBinaryFiles`/`fileStatuses` — NOT on the `Git` interface; git.go:686 confirms those are
  internal helpers). Adding a method on `*gitRunner` in a new file is legal Go ⇒ **no `Git` interface edit,
  no mock changes, no `git.go`/`binary.go` edit.** See research §3.

  ⚠️ **#4 — place `--numstat` BEFORE any `--` pathspec separator in diffArgs.** `buildDiffArgs` emits
  `["diff", domain..., "-M", "-U<n>]` (no `--`). The contract: "compose with excludes the same way the
  aggregate diff does" — a caller (the FR3i sizing path, P1.M4.T2) may pass `-- <excludes>` inside
  `diffArgs`. A simple append would put `--numstat` after `--` (swallowed as a pathspec). So `numstatRows`
  inserts `--numstat` before the first `--` (if any), else appends. Strict superset of simple append (the
  no-`--` case is identical). 6 lines. See research §4.

  ⚠️ **#5 — binary = `added == "-"` (git emits `-\t-\t<path>`); rows sorted by path.** Binary ⇒
  `IsBinary=true`, `Added=Deleted=0`. Non-binary ⇒ `strconv.Atoi` both. Return rows SORTED by `Path` for
  deterministic skeleton/sizing output. TAB-separated (`SplitN(line,"\t",3)`) ⇒ paths with spaces are
  tab-safe (the whole path is `fields[2]`). See research §1/§5.

  ⚠️ **#6 — NO conflict with the parallel work item.** P1.M2.T3.S1 (FR3h index-line stripping) touches
  `binary.go`, `git.go`, `stagediff_test.go`. This task creates NEW `numstat.go` + `numstat_test.go` and
  edits neither `binary.go` nor `git.go`. Zero file overlap; the two are independent (numstat output has no
  `index` lines, so the features don't interact). See research §7.

  Deliverable: NEW `internal/git/numstat.go` (`numstatRow` + `resolveNumstatPath` + `numstatRows`) + NEW
  `internal/git/numstat_test.go` (pure table tests for `resolveNumstatPath` incl. brace forms + integration
  tests for `numstatRows`: edit/binary/rename/brace-rename/spaces/empty). NO other file touched. NO go.mod
  change (stdlib `context`+`fmt`+`sort`+`strconv`+`strings`). OUTPUT: `numstatRows(...)` returns
  deterministic parsed rows keyed on destination path; consumed by S2 + M4.T2.S2. DOCS: none — internal.

---

## Goal

**Feature Goal**: Provide the shared per-file size/shape primitive — `numstatRows` — that captures
`git diff <diffArgs> --numstat` and parses it into deterministic, destination-keyed rows
(`{Added, Deleted, IsBinary, Path}`), correctly resolving git's `=>`/`{...}` rename notation (which git
≥2.9 emits even without `-M`) and binary `-`/`-` rows. This is the dual-use input for the FR3g compact
skeleton (S2) and the FR3i water-fill sizing (P1.M4.T2): one git call gives both the model's completeness
view and the per-file sizing.

**Deliverable** (NEW files only — no edits to existing files, no new deps):
1. **NEW `internal/git/numstat.go`** (`package git`, imports `context`+`fmt`+`sort`+`strconv`+`strings`):
   - `type numstatRow struct { Added, Deleted int; IsBinary bool; Path string }`,
   - `func resolveNumstatPath(p string) string` (pure: the `=>`/brace → destination resolver),
   - `func (g *gitRunner) numstatRows(ctx context.Context, diffArgs ...string) ([]numstatRow, error)`.
2. **NEW `internal/git/numstat_test.go`** (`package git`): pure table tests for `resolveNumstatPath`
   (incl. both brace forms + spaces + no-`=>`-with-braces) + integration tests for `numstatRows` over temp
   repos (edit / binary / pure rename / brace-collapse rename / path-with-spaces / empty diff), mirroring
   `binary_test.go`'s `asRunner(New(repo))` pattern.

**Success Definition**: `go build ./... && go vet ./... && go test ./internal/git/` GREEN; `gofmt -l` clean;
`numstatRows` returns rows keyed on the destination path (rename `=>` resolved, brace-collapse resolved),
binary rows flagged `IsBinary`, sorted by path; pure `resolveNumstatPath` passes every brace/simple/verbatim
case; no edit to `binary.go`/`git.go`/the `Git` interface; go.mod/go.sum byte-unchanged.

## User Persona

**Target User**: The downstream diff-payload subtasks — S2 (P1.M3.T1.S2, FR3g skeleton prepend) and
P1.M4.T2.S2 (FR3i water-fill truncation). Both need a deterministic per-file size/shape view: S2 renders it
as the compact skeleton line per file; M4.T2 uses each row's add/delete magnitude as the file's body-size
estimate for the water-fill. Transitively, every user whose diff is large enough to truncate (FR3d/FR3i).

**Use Case**: A staged change with a rename, a binary, and several edits. `numstatRows(ctx, "--cached")`
returns one row per file (the rename resolved to its destination, the binary flagged), sorted — ready to
render as the skeleton and to size for water-fill.

**User Journey**: (internal) StagedDiff/WorkingTreeDiff/TreeDiff (S2/M4.T2) call `numstatRows` with the
domain diffArgs → parse → render skeleton (S2) / size bodies (M4.T2). S1 is the primitive they both call.

**Pain Points Addressed**: Without a correct numstat parser, renames would key on `"old => new"` strings
(breaking the size map + skeleton), binaries would mis-parse (`"-"` isn't an int), and brace-collapsed
paths would be unreadable. S1 is the robust foundation.

## Why

- **Unblocks FR3g (skeleton) + FR3i (water-fill).** Both need the per-file numstat view; FR3i explicitly
  specifies "one `git` call, dual-use: the skeleton is both the model's completeness view *and* the sizing
  input." S1 is that one call's parser.
- **Correctness the existing code got wrong.** The repo assumed `-M`-less numstat has clean paths
  (git.go:686); empirically git ≥2.9 emits `=>` regardless. S1's resolver makes the size map/skeleton
  correct under renames on any modern git.
- **Decoupled, no surface change.** A new file + a `*gitRunner` method (not on the interface). No caller
  changes in S1 (S2/M4.T2 wire it later). No new deps. No behavioral change to existing diff functions.

## What

A new `numstat.go` with a row type, a pure rename-path resolver, and the `numstatRows` method; and a new
`numstat_test.go` with pure table tests (resolver) + integration tests (method, over temp repos). No other
file changes. No config/API/CLI/doc surface.

### Success Criteria

- [ ] `numstat.go` exists, `package git`, imports EXACTLY `context`,`fmt`,`sort`,`strconv`,`strings`.
- [ ] `numstatRow{Added, Deleted int; IsBinary bool; Path string}`; `Path` is the destination (rename-resolved).
- [ ] `resolveNumstatPath` (pure): `=>`-less → verbatim; `old => new` → `new`; `prefix{old => new}suffix` →
      `prefix`+`new`+`suffix`; handles both prefix-only and prefix+suffix brace collapse.
- [ ] `numstatRows(ctx, diffArgs...)`: runs `git diff <diffArgs> --numstat` with `--numstat` placed BEFORE
      any `--` in diffArgs; binary (`added == "-"`) ⇒ `IsBinary=true`, counts 0; else `Atoi` both; rows
      sorted by `Path`; follows the `run` error convention (infrastructural `err` propagated; `code!=0` wrapped).
- [ ] `numstat_test.go`: pure table tests for `resolveNumstatPath` (incl. both brace forms, spaces,
      no-`=>`-with-braces) + integration tests for `numstatRows` (edit/binary/pure-rename/brace-rename/
      spaces/empty) via `asRunner(New(repo))`, mirroring `binary_test.go`.
- [ ] `go build ./... && go vet ./... && go test ./internal/git/` GREEN; `gofmt -l` clean.
- [ ] go.mod/go.sum byte-unchanged; NO edit to `binary.go`, `git.go`, the `Git` interface, or any other file.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the empirical numstat shapes
(research §1 — the exact `=>`/brace/binary formats), the copy-ready `numstat.go` in the Blueprint, the pure
resolver algorithm (§2), the `--numstat`-before-`--` placement (§4), the `run`/`*gitRunner` conventions
(read from `binary.go`'s `detectBinaryFiles`), and the `binary_test.go` integration-test pattern. No
diff-body/snapshot/decompose knowledge required — S1 is a self-contained parser + one git call.

### Documentation & References

```yaml
# MUST READ — the empirical git semantics (resolves the comment-vs-contract contradiction) + design calls
- docfile: plan/007_b33d310438c6/P1M3T1S1/research/numstat-empirical.md
  why: §0 (the git.go:686 comment is WRONG on git ≥2.9; `=>` appears without `-M` — verified), §1 (the exact
       numstat shapes: edit/rename/brace/binary), §2 (the resolver algorithm + the pure-function factoring),
       §3 (NEW file; no binary.go/git.go/interface edit), §4 (--numstat before `--`), §5 (binary+sort),
       §6 (tests), §7 (no conflict with P1.M2.T3.S1).
  critical: §0/§2 (the resolver is REQUIRED; handle both `=>` forms) and §4 (--numstat placement) are the
       things most likely to be implemented wrong.

# The authoritative git-semantics reference (cited by the contract)
- docfile: plan/007_b33d310438c6/architecture/git_diff_semantics.md
  section: "## 3. `git diff --numstat` output format" + "### Renames in numstat" + "### Version notes"
  why: confirms `--numstat` is TAB-separated (`added\tdeleted\tpath`), binary is `-`/`-`, `-M` puts `=>`/
       `{...}` in the path column, and the `diff.renames` default flip (git 2.8–2.9) means renames appear
       WITHOUT `-M` on modern git.
  critical: the "Version notes" — "-M is the only cross-version-safe choice" but the DEFAULT is already on
       for git ≥2.9, so the parser must handle `=>` regardless. (The doc's "option 1: run without -M for a
       clean size map" is INVALID on git ≥2.9 — see research §0.)

# The pattern to mirror — detectBinaryFiles (the existing numstat caller)
- file: internal/git/binary.go
  section: detectBinaryFiles (~L98) — runs `git diff <diffArgs> --numstat`, parses via
           `strings.SplitN(line,"\t",3)`, follows the `run` error convention (`err!=nil` propagate;
           `code!=0` → `fmt.Errorf("git diff --numstat: failed (exit %d): %s",…)`), and the "do NOT TrimSpace
           the line; preserve paths" note.
  why: the EXACT argv-build + parse + error idiom `numstatRows` mirrors. Note detectBinaryFiles does NOT
       resolve `=>` (its latent bug) — `numstatRows` is the corrected, general version.
  critical: mirror the `run` convention and the no-TrimSpace rule; do NOT modify detectBinaryFiles.

# The arg-composition helper (for the --numstat-before--- placement)
- file: internal/git/git.go
  section: buildDiffArgs (L689) — emits `["diff", domain..., "-M", "-U<n>]` (NO `--`; callers append
           `-- <pathspecs>` themselves); the L686 doc comment (the wrong "-M corrupts numstat" claim).
  why: confirms `--numstat` must be placed before any `--` the caller passes, and that `numstatRows` is a
       `*gitRunner` method (not routed through buildDiffArgs, not on the Git interface).
  critical: do NOT route numstatRows through buildDiffArgs (it doesn't emit --numstat); build the argv in
       numstatRows itself, inserting --numstat before `--`.

# The test pattern to mirror
- file: internal/git/binary_test.go
  section: asRunner (L13) `func asRunner(g Git) *gitRunner`; the detectBinaryFiles integration tests
           (~L94-160) — `repo := t.TempDir()`, `exec.Command("git","-C",repo,"init"/"add"/"mv")`,
           `os.WriteFile`, `g := asRunner(New(repo))`, `g.detectBinaryFiles(ctx,"--cached")`; AND the pure
           table tests at the top (TestIsBinaryByExtension — the style for resolveNumstatPath).
  why: the EXACT test idiom — white-box `package git`, `asRunner(New(repo))` for `*gitRunner` methods,
       `exec.Command` for repo setup, pure table tests for pure helpers.
  critical: the brace-collapse rename integration test must use `-M` (the brace form needs rename detection
       engaged) — pass it in diffArgs. The pure resolver table covers brace forms without git.

# The contract basis (in your context as selected_prd_content)
- file: PRD.md (or plan/007_…/prd_snapshot.md)
  section: "9.1 Diff capture" FR3g (skeleton: `--numstat`, one added/deleted/path line per file) + FR3i
           (water-fill sizes from the numstat skeleton's per-file line counts — "one git call, dual-use").
  critical: FR3i explicitly makes the numstat call dual-use (skeleton + sizing) — `numstatRows` is that call.

# The parallel PRP (scope check — no conflict)
- file: plan/007_b33d310438c6/P1M2T3S1/PRP.md
  why: confirms P1.M2.T3.S1 (FR3h index-line stripping) touches binary.go/git.go/stagediff_test.go — NOT
       numstat.go/numstat_test.go. This task creates those NEW files; zero overlap. (numstat output has no
       `index` lines ⇒ the features don't interact.)
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  binary.go          # detectBinaryFiles/fileStatuses (numstat/name-status, *gitRunner methods, NOT on Git interface) — UNCHANGED
  git.go             # gitRunner + run + buildDiffArgs (L689) + the L686 comment — UNCHANGED
  numstat.go         # NEW (this subtask) ← numstatRow + resolveNumstatPath + (g *gitRunner) numstatRows
  numstat_test.go    # NEW (this subtask) ← resolveNumstatPath table tests + numstatRows integration tests
  binary_test.go     # the PATTERN to mirror (asRunner/New/exec.Command/pure-table) — UNCHANGED
go.mod / go.sum      # UNCHANGED (stdlib only)
```

### Desired Codebase tree with files to be added

```bash
internal/git/numstat.go         # NEW — the numstat primitive (FR3g/FR3i dual-use input)
internal/git/numstat_test.go    # NEW — pure resolver table tests + integration tests
# NO other file added or edited.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 — the => resolver is REQUIRED even without -M): git ≥2.9 defaults diff.renames ON, so a
// pure rename emits "0\t0\told => new" in `git diff --cached --numstat` WITH OR WITHOUT -M (verified git
// 2.54.0). The git.go:686 comment claiming "-M corrupts numstat paths" is wrong for modern git. numstatRows
// MUST resolve => → destination unconditionally. (research §0)

// CRITICAL (#2 — both rename forms): simple "old => new" AND brace "prefix{old => new}suffix". git collapses
// common prefix/suffix into {...}: "dir/{a.go => b.go}"→"dir/b.go"; also "dir/{a => b}.go"→"dir/b.go".
// Resolver: if => present + {...}: dest = before-{ + TrimSpace(right of => in braces) + after-}; else
// TrimSpace(right of =>); no => → verbatim. Factor as pure resolveNumstatPath. (research §2)

// CRITICAL (#3 — NEW file; do NOT edit binary.go/git.go/interface): numstatRows is a *gitRunner method
// (like detectBinaryFiles — NOT on the Git interface). Add it in numstat.go (methods can live in any file
// of the package). NO Git interface edit, NO mock changes, NO detectBinaryFiles change. (research §3)

// CRITICAL (#4 — place --numstat BEFORE any -- in diffArgs): buildDiffArgs emits no --; a caller may pass
// "-- <excludes>". Simple append would swallow --numstat as a pathspec. Insert --numstat before the first
// "--" (if any), else append. (research §4)

// GOTCHA (binary = added == "-"): git emits "-\t-\t<path>" for binary (both counts "-"). Set IsBinary=true,
// Added=Deleted=0. Non-binary: strconv.Atoi both (Atoi("-") wouldn't be reached — the "-" guard is first).
// GOTCHA (TAB-separated, do NOT TrimSpace the line): SplitN(line,"\t",3) puts the whole path (spaces, =>,
// braces) in fields[2] — tab-safe. Trailing "\n" from strings.Split → empty last line → skip (len<3). Do
// NOT TrimSpace the line (preserves paths with leading/trailing spaces); only TrimSpace INSIDE the resolver
// (around the => right-hand side).
// GOTCHA (sort by Path): return rows sorted by Path for deterministic skeleton/sizing output.
// GOTCHA (run error convention): err!=nil (infrastructural: git missing / ctx cancel / start fail) → return
// nil,err unwrapped; code!=0 → fmt.Errorf("git diff --numstat: failed (exit %d): %s", code, strings.TrimSpace(stderr)).
// GOTCHA (no new imports beyond stdlib): context, fmt, sort, strconv, strings. go mod tidy is a no-op.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/git/numstat.go
package git

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// numstatRow is one parsed line of `git diff --numstat`: the added/deleted line counts (0 for a binary
// file, whose numstat counts are "-"), whether git content-sniffed it as binary, and the destination (b/)
// path. For a rename the path is the NEW path (numstat's `old => new` notation resolved by
// resolveNumstatPath). Consumed by the FR3g compact skeleton (P1.M3.T1.S2) and the FR3i water-fill sizing
// (P1.M4.T2) — one git call, dual-use (the skeleton is both the model's completeness view and the sizing
// input). (PRD §9.1 FR3g/FR3i.)
type numstatRow struct {
	Added    int
	Deleted  int
	IsBinary bool
	Path     string // destination (b/) path, rename-resolved
}

// resolveNumstatPath resolves a `git diff --numstat` path field to the DESTINATION (b/) path. For a rename,
// numstat emits git's `=>` notation — and git ≥2.9 emits it EVEN WITHOUT -M (diff.renames defaulted on in
// the 2.8–2.9 era; verified on git 2.54.0). Two forms:
//   - simple:   `old => new`               → `new`
//   - brace:    `prefix{old => new}suffix` → `prefix` + `new` + `suffix`
//     (git collapses the common prefix/suffix into the braces; e.g. `dir/{a.go => b.go}` → `dir/b.go`,
//     and the suffix-collapsed `dir/{a => b}.go` → `dir/b.go`).
// A field with no `=>` is returned verbatim (a normal path, or a path that legitimately contains `{`/`}`
// but no rename). Pure function; no I/O.
func resolveNumstatPath(p string) string {
	if !strings.Contains(p, "=>") {
		return p
	}
	// Brace-collapsed form: prefix{old => new}suffix.
	if bi := strings.Index(p, "{"); bi >= 0 {
		if bj := strings.Index(p, "}"); bj > bi {
			return p[:bi] + rightOfArrow(p[bi+1:bj]) + p[bj+1:]
		}
	}
	// Simple form: old => new.
	return rightOfArrow(p)
}

// rightOfArrow returns the trimmed text after the first `=>` in s (the rename destination). Pure helper.
func rightOfArrow(s string) string {
	if i := strings.Index(s, "=>"); i >= 0 {
		return strings.TrimSpace(s[i+len("=>"):])
	}
	return s
}

// numstatRows runs `git diff <diffArgs> --numstat` and returns one numstatRow per changed file, keyed on
// the DESTINATION path (rename-resolved), sorted by path for deterministic output. diffArgs selects the
// diff domain and is forwarded verbatim — ["--cached"] (staged), [] (working tree), or [treeA, treeB]
// (tree-to-tree) — and may include caller-composed flags (-M, -U<n> via buildDiffArgs). `--numstat` is
// placed BEFORE any `--` pathspec separator in diffArgs so a caller that appends `-- <excludes>` (the FR3i
// sizing path, P1.M4.T2) composes correctly (a trailing --numstat would be swallowed as a pathspec).
// Binary rows (`added == "-"`, git emits "-\t-\t<path>") set IsBinary with Added=Deleted=0. Read-only
// w.r.t. refs/index. (PRD §9.1 FR3g/FR3i.)
//
// NOT routed through buildDiffArgs (which doesn't emit --numstat) and NOT on the Git interface — it is an
// internal *gitRunner helper, the general counterpart to binary.go's detectBinaryFiles (which keeps only
// binary rows; numstatRows keeps ALL rows for the skeleton/sizing).
func (g *gitRunner) numstatRows(ctx context.Context, diffArgs ...string) ([]numstatRow, error) {
	args := []string{"diff"}
	// Place --numstat before any `--` pathspec separator so excludes compose correctly.
	splitAt := len(diffArgs)
	for i, a := range diffArgs {
		if a == "--" {
			splitAt = i
			break
		}
	}
	args = append(args, diffArgs[:splitAt]...)
	args = append(args, "--numstat")
	args = append(args, diffArgs[splitAt:]...)

	stdout, stderr, code, err := g.run(ctx, g.workDir, args...)
	if err != nil {
		return nil, err // infrastructural (git missing / ctx cancel / start failure) — propagate unwrapped
	}
	if code != 0 {
		return nil, fmt.Errorf("git diff --numstat: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	var rows []numstatRow
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 3 {
			continue // trailing "\n" → empty line; or malformed — skip (do NOT TrimSpace the line; preserve paths)
		}
		added, deleted, path := fields[0], fields[1], fields[2]
		row := numstatRow{Path: resolveNumstatPath(path)}
		if added == "-" { // git emits "-\t-\t<path>" for binary (both counts "-")
			row.IsBinary = true
		} else {
			row.Added, _ = strconv.Atoi(added)
			row.Deleted, _ = strconv.Atoi(deleted)
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Path < rows[j].Path })
	return rows, nil
}
```

```go
// internal/git/numstat_test.go — white-box (package git). Two test classes:
//  (1) pure table tests for resolveNumstatPath (incl. both brace forms — deterministic, no git);
//  (2) integration tests for numstatRows over temp repos (mirrors binary_test.go's asRunner(New(repo))).
package git

import (
	"context"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// (1) Pure resolver table — covers every form, including brace variants that are fiddly via real git.
func TestResolveNumstatPath(t *testing.T) {
	tests := []struct{ in, want, desc string }{
		{"a.txt", "a.txt", "no rename — verbatim"},
		{"src/main.go", "src/main.go", "path with slash, no rename"},
		{"old => new", "new", "simple rename"},
		{"old.txt => new.txt", "new.txt", "simple rename with extensions"},
		{"dir/{a.go => b.go}", "dir/b.go", "brace collapse, prefix only"},
		{"dir/{a => b}.go", "dir/b.go", "brace collapse, prefix + suffix"},
		{"{old => new}", "new", "brace collapse, no prefix/suffix"},
		{"prefix{x => y}suffix", "prefixysuffix", "brace collapse, arbitrary prefix+suffix"},
		{"weird{name}.txt", "weird{name}.txt", "braces but no => → verbatim"},
		{"my file.txt", "my file.txt", "spaces preserved"},
		{"a => b => c", "b => c", "only the FIRST => splits (right side kept verbatim incl. any further =>)"},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			if got := resolveNumstatPath(tc.in); got != tc.want {
				t.Errorf("resolveNumstatPath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// (2) Integration tests — mirror binary_test.go's asRunner(New(repo)) + exec.Command setup.
// helper: init a repo, return its dir.
func numstatInitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, c := range [][]string{
		{"git", "-C", repo, "init", "-q"},
		{"git", "-C", repo, "config", "user.email", "t@t"},
		{"git", "-C", repo, "config", "user.name", "t"},
	} {
		if out, err := exec.Command(c[0], c[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", c, err, out)
		}
	}
	return repo
}

func numstatRunGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestNumstatRows_EditAndBinary(t *testing.T) {
	repo := numstatInitRepo(t)
	os.WriteFile(repo+"/a.txt", []byte("alpha\nbeta\n"), 0o644)
	os.WriteFile(repo+"/bin.png", []byte("\x00\x01BIN"), 0o644)
	numstatRunGit(t, repo, "add", ".")
	numstatRunGit(t, repo, "commit", "-q", "-m", "init")
	os.WriteFile(repo+"/a.txt", []byte("alpha\nBETA\nGAMMA\n"), 0o644) // +1 -1 net (+1 added, but counts: 2 added 1 deleted region)
	os.WriteFile(repo+"/bin2.png", []byte("\x00\x02MORE"), 0o644)
	numstatRunGit(t, repo, "add", ".")

	rows, err := asRunner(New(repo)).numstatRows(context.Background(), "--cached")
	if err != nil {
		t.Fatalf("numstatRows: %v", err)
	}
	// Sort assertion: rows come back sorted by path.
	if !sort.SliceIsSorted(rows, func(i, j int) bool { return rows[i].Path < rows[j].Path }) {
		t.Errorf("rows not sorted by path: %+v", rows)
	}
	byPath := map[string]numstatRow{}
	for _, r := range rows {
		byPath[r.Path] = r
	}
	if r, ok := byPath["a.txt"]; !ok {
		t.Errorf("a.txt missing: %+v", rows)
	} else if r.IsBinary || r.Added == 0 {
		t.Errorf("a.txt = %+v, want non-binary with Added>0", r)
	}
	if r, ok := byPath["bin2.png"]; !ok {
		t.Errorf("bin2.png missing: %+v", rows)
	} else if !r.IsBinary {
		t.Errorf("bin2.png = %+v, want IsBinary", r)
	}
}

func TestNumstatRows_PureRenameResolvedToDestination(t *testing.T) {
	repo := numstatInitRepo(t)
	os.WriteFile(repo+"/old.txt", []byte("content\n"), 0o644)
	numstatRunGit(t, repo, "add", ".")
	numstatRunGit(t, repo, "commit", "-q", "-m", "init")
	numstatRunGit(t, repo, "mv", "old.txt", "new.txt")

	rows, err := asRunner(New(repo)).numstatRows(context.Background(), "--cached")
	if err != nil {
		t.Fatalf("numstatRows: %v", err)
	}
	// A pure rename emits "0\t0\told.txt => new.txt" (=> appears even WITHOUT -M on git ≥2.9).
	// Resolved destination key = "new.txt" (NOT "old.txt => new.txt").
	for _, r := range rows {
		if strings.Contains(r.Path, "=>") {
			t.Errorf("rename not resolved — path still contains =>: %+v", r)
		}
	}
	found := false
	for _, r := range rows {
		if r.Path == "new.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("destination new.txt not present in rows: %+v", rows)
	}
}

func TestNumstatRows_BraceCollapseRename(t *testing.T) {
	repo := numstatInitRepo(t)
	os.MkdirAll(repo+"/dir", 0o755)
	os.WriteFile(repo+"/dir/a.go", []byte("x\n"), 0o644)
	numstatRunGit(t, repo, "add", ".")
	numstatRunGit(t, repo, "commit", "-q", "-m", "init")
	numstatRunGit(t, repo, "mv", "dir/a.go", "dir/b.go")

	// The brace form `dir/{a.go => b.go}` needs -M engaged (pass it in diffArgs).
	rows, err := asRunner(New(repo)).numstatRows(context.Background(), "--cached", "-M")
	if err != nil {
		t.Fatalf("numstatRows: %v", err)
	}
	for _, r := range rows {
		if strings.ContainsAny(r.Path, "{}=>") {
			t.Errorf("brace rename not resolved — path still contains brace/arrow tokens: %+v", r)
		}
	}
	found := false
	for _, r := range rows {
		if r.Path == "dir/b.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("destination dir/b.go not present in rows: %+v", rows)
	}
}

func TestNumstatRows_PathWithSpaces(t *testing.T) {
	repo := numstatInitRepo(t)
	os.WriteFile(repo+"/my file.txt", []byte("x\n"), 0o644)
	numstatRunGit(t, repo, "add", ".")
	numstatRunGit(t, repo, "commit", "-q", "-m", "init")
	os.WriteFile(repo+"/my file.txt", []byte("x\ny\n"), 0o644)
	numstatRunGit(t, repo, "add", ".")

	rows, err := asRunner(New(repo)).numstatRows(context.Background(), "--cached")
	if err != nil {
		t.Fatalf("numstatRows: %v", err)
	}
	found := false
	for _, r := range rows {
		if r.Path == "my file.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("path with spaces not preserved: %+v", rows)
	}
}

func TestNumstatRows_EmptyDiff(t *testing.T) {
	repo := numstatInitRepo(t)
	os.WriteFile(repo+"/a.txt", []byte("x\n"), 0o644)
	numstatRunGit(t, repo, "add", ".")
	numstatRunGit(t, repo, "commit", "-q", "-m", "init") // clean tree, nothing staged

	rows, err := asRunner(New(repo)).numstatRows(context.Background(), "--cached")
	if err != nil {
		t.Fatalf("numstatRows on empty diff: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("empty diff rows = %+v, want none", rows)
	}
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/git/numstat.go (numstatRow + resolveNumstatPath + numstatRows)
  - PACKAGE git; IMPORTS EXACTLY context, fmt, sort, strconv, strings.
  - DEFINE numstatRow{Added, Deleted int; IsBinary bool; Path string}.
  - IMPLEMENT pure resolveNumstatPath (+ rightOfArrow helper) per the Blueprint: `=>`+brace → prefix+new+suffix;
      `=>` simple → TrimSpace(right); no `=>` → verbatim.
  - IMPLEMENT (g *gitRunner) numstatRows: build argv with --numstat placed BEFORE the first `--` (else
      append); run via g.run; parse SplitN(line,"\t",3); added=="-" ⇒ IsBinary; else Atoi both;
      resolveNumstatPath(path); sort by Path. Follow the run error convention (err propagate; code!=0 wrap).
  - GOTCHA: do NOT route through buildDiffArgs; do NOT add -M yourself (the caller passes it); do NOT
      TrimSpace the line (only inside the resolver).

Task 2: CREATE internal/git/numstat_test.go (pure table + integration)
  - PACKAGE git (white-box). IMPORTS: context, os, os/exec, sort, strings, testing.
  - ADD TestResolveNumstatPath table (every form incl. both brace variants, spaces, no-`=>`-with-braces,
      multi-`=>`).
  - ADD integration tests (asRunner(New(repo)) + exec.Command, mirroring binary_test.go): EditAndBinary,
      PureRenameResolvedToDestination (=> appears WITHOUT -M), BraceCollapseRename (pass -M in diffArgs),
      PathWithSpaces, EmptyDiff. Assert sort + destination keying + IsBinary + empty.
  - HELPERS: numstatInitRepo / numstatRunGit (local to this test file) — or reuse binary_test.go's if shared.
  - GOTCHA: the pure-rename test does NOT pass -M (asserts => is resolved even without it — the §0 point);
      the brace test DOES pass -M (the brace form needs rename detection).

Task 3: VERIFY
  - RUN: go build ./... && go vet ./... && go test ./internal/git/ -v. gofmt -l internal/git/.
  - CONFIRM: no edit to binary.go/git.go/Git interface; go.mod/go.sum byte-unchanged.
  - CONFIRM the resolver table covers both brace forms (deterministic) + the integration tests cover real
      git shapes (edit/binary/rename/brace/spaces/empty).
```

### Implementation Patterns & Key Details

```go
// THE resolver (pure — both forms):
func resolveNumstatPath(p string) string {
	if !strings.Contains(p, "=>") { return p }
	if bi := strings.Index(p, "{"); bi >= 0 {
		if bj := strings.Index(p, "}"); bj > bi {
			return p[:bi] + rightOfArrow(p[bi+1:bj]) + p[bj+1:] // prefix{old=>new}suffix → prefix+new+suffix
		}
	}
	return rightOfArrow(p) // old => new → new
}

// THE --numstat-before--- placement (so excludes compose):
splitAt := len(diffArgs)
for i, a := range diffArgs { if a == "--" { splitAt = i; break } }
args = append(append(append([]string{"diff"}, diffArgs[:splitAt]...), "--numstat"), diffArgs[splitAt:]...)

// THE binary signal + parse (mirror detectBinaryFiles' run convention):
if added == "-" { row.IsBinary = true } else { row.Added, _ = strconv.Atoi(added); row.Deleted, _ = strconv.Atoi(deleted) }
sort.Slice(rows, func(i,j int) bool { return rows[i].Path < rows[j].Path })
```

### Integration Points

```yaml
GO MODULE (go.mod/go.sum): change NONE. numstat.go uses stdlib only (context+fmt+sort+strconv+strings).
      `go mod tidy` is a no-op.

PACKAGE EDGES: NONE added. numstat.go is package git; it uses g.run/g.workDir (same package). It does NOT
      import any other internal package. numstatRows is a *gitRunner method (NOT on the Git interface).

UPSTREAM (the inputs — consume, do NOT edit):
  - g.run / g.workDir — the existing runner (git.go).
  - buildDiffArgs (git.go:689) — callers may pass its output as diffArgs (it has no `--`, so --numstat appends cleanly).

DOWNSTREAM (the consumers — NOT this task):
  - P1.M3.T1.S2 (FR3g skeleton): renders numstatRows as the compact per-file skeleton block.
  - P1.M4.T2.S2 (FR3i water-fill): uses each row's Added+Deleted as the file body-size estimate.

FROZEN/LEAVE (do NOT edit):
  - internal/git/binary.go (detectBinaryFiles/fileStatuses — keep; they have their own numstat call + a
    latent renamed-binary keying quirk that is NOT this task's scope).
  - internal/git/git.go (gitRunner/run/buildDiffArgs/the L686 comment).
  - the Git interface (numstatRows is NOT added to it — it's an internal helper).
  - every other file. PRD.md, go.mod, Makefile.

NO NEW DATABASE / ROUTES / CLI / CONFIG / DOCS.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/git/numstat.go internal/git/numstat_test.go
go vet ./internal/git/
head -10 internal/git/numstat.go   # → package git; import ( "context" "fmt" "sort" "strconv" "strings" )
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; imports exactly the 5 stdlib packages; go.mod/go.sum byte-unchanged.
```

### Level 2: Unit + integration tests

```bash
go test ./internal/git/ -v -run 'TestResolveNumstatPath|TestNumstatRows'
go test ./internal/git/
# Expected PASS — verify explicitly:
#   TestResolveNumstatPath/* ......... every form incl. both brace variants, spaces, no-`=>`-with-braces
#   TestNumstatRows_EditAndBinary ... counts parsed; IsBinary set; rows sorted by path
#   TestNumstatRows_PureRenameResolvedToDestination ... => resolved WITHOUT -M (the §0 point); dest key = new name
#   TestNumstatRows_BraceCollapseRename ... dir/{a.go => b.go} → dir/b.go (with -M)
#   TestNumstatRows_PathWithSpaces ... "my file.txt" preserved (tab-safe)
#   TestNumstatRows_EmptyDiff ........ 0 rows, no error
# If PureRename fails (path still has =>), the resolver isn't applied; if Brace fails, pass -M in diffArgs.
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...    # Expect clean (numstat.go compiles into package git).
go test ./...     # Expect all PASS (the new tests + no regression in the existing git/provider/config suites).
# Confirm ONLY the two new files were added (no edit to binary.go/git.go/interface):
git diff --name-only; git status --short internal/git/
git diff --exit-code internal/git/binary.go internal/git/git.go && echo "binary.go/git.go UNCHANGED (expected)"
# Confirm numstatRows is NOT on the Git interface (internal helper only):
! grep -q "numstatRows" <(sed -n '/type Git interface/,/^}/p' internal/git/git.go) && echo "numstatRows NOT on Git interface (good)"
```

### Level 4: Correctness reasoning (the empirical contract)

```bash
# The parser's correctness rests on the empirical git shapes (research §1). Verify by reasoning + the tests:
#   1. Pure rename → "0\t0\told => new" even WITHOUT -M (git ≥2.9 default). Resolver → "new". (TestPureRename)
#   2. Brace rename → "prefix{old => new}suffix" with -M. Resolver → prefix+new+suffix. (TestBraceCollapse)
#   3. Binary → "-\t-\t<path". IsBinary=true, counts 0. (TestEditAndBinary)
#   4. Tab-separated ⇒ paths with spaces are fields[2] whole. (TestPathWithSpaces)
#   5. Empty diff ⇒ 0 rows. (TestEmptyDiff)
#   6. --numstat placed before any -- ⇒ excludes (M4.T2) compose. (unit-covered by the placement logic; the
#      integration tests pass no --, exercising the append branch.)
# (No Level-4 commands beyond Levels 1–3 — the tests + the empirical research ARE the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l` clean on the two new files.
- [ ] `go test ./...` GREEN (new tests + no regression).
- [ ] go.mod/go.sum byte-unchanged; numstat.go imports ONLY context/fmt/sort/strconv/strings.

### Feature Validation
- [ ] `numstatRow{Added, Deleted int; IsBinary bool; Path string}`; `Path` = destination (rename-resolved).
- [ ] `resolveNumstatPath` pure: handles simple `=>`, brace `prefix{old => new}suffix`, no-`=>`-verbatim, spaces.
- [ ] `numstatRows`: `--numstat` before any `--`; binary ⇒ IsBinary; rows sorted by Path; run-error convention.
- [ ] Pure-rename test resolves `=>` WITHOUT `-M`; brace test resolves with `-M`; empty diff ⇒ 0 rows.

### Code Quality Validation
- [ ] Mirrors `binary.go`'s `detectBinaryFiles` idiom (argv build, SplitN("\t",3), run convention, no-TrimSpace).
- [ ] NEW file only; no edit to `binary.go`/`git.go`/the `Git` interface (numstatRows is a `*gitRunner` helper).
- [ ] Anti-patterns avoided (see below); no out-of-scope churn.

### Documentation
- [ ] Doc comments on `numstatRow`/`resolveNumstatPath`/`numstatRows` cite PRD §9.1 FR3g/FR3i, the git ≥2.9
      `=>`-without-`-M` fact, and the `--numstat`-before-`--` rationale. No docs/*.md edits (internal; S1 is
      a primitive — the skeleton render + docs sync are S2/P1.M5).

---

## Anti-Patterns to Avoid

- ❌ **Don't rely on the absence of `-M` to keep numstat paths clean.** git ≥2.9 defaults `diff.renames` ON;
      a pure rename emits `old => new` WITHOUT `-M` (verified git 2.54.0). The git.go:686 comment is wrong.
      `numstatRows` MUST resolve `=>` unconditionally. (research §0)
- ❌ **Don't handle only the simple `old => new` form.** git also emits brace-collapsed `prefix{old => new}
      suffix` (verified `dir/{a.go => b.go}`). The resolver must combine prefix + new + suffix. (research §2)
- ❌ **Don't edit `binary.go`, `git.go`, or the `Git` interface.** `numstatRows` is a NEW `*gitRunner` method
      in a NEW file (methods can live in any package file). `detectBinaryFiles` stays untouched (its latent
      renamed-binary quirk is NOT this task). (research §3)
- ❌ **Don't simply append `--numstat` to diffArgs.** A caller may pass `-- <excludes>`; `--numstat` must
      come BEFORE the `--` or git swallows it as a pathspec. Insert before the first `--`. (research §4)
- ❌ **Don't TrimSpace the line before splitting.** Paths can have leading/trailing spaces; TrimSpace would
      corrupt them. Split on `\t` first (tab-safe); TrimSpace ONLY inside the resolver (around the `=>`
      right-hand side). (research §5)
- ❌ **Don't add `-M` inside `numstatRows`.** Forward diffArgs verbatim; the caller passes `-M` (via
      buildDiffArgs) when it wants rename detection engaged. (S1's brace test passes `-M`; the pure-rename
      test does NOT — proving `=>` is resolved either way.)
- ❌ **Don't route `numstatRows` through `buildDiffArgs`.** buildDiffArgs doesn't emit `--numstat`; build the
      argv in `numstatRows` itself (mirroring `detectBinaryFiles`, which also builds its own argv — git.go:686).
- ❌ **Don't add numstatRows to the `Git` interface.** It's an internal helper (like detectBinaryFiles/
      fileStatuses). Adding it to the interface would force mock/stub implementations — out of scope.
- ❌ **Don't touch the parallel P1.M2.T3.S1's files.** It edits binary.go/git.go/stagediff_test.go; this task
      is numstat.go/numstat_test.go only. Zero overlap. (research §7)

---

## Confidence Score

**9/10** — a self-contained parser (one git call + a pure resolver) whose every input shape is pinned by
EMPIRICAL git 2.54.0 evidence (research §1: edit/pure-rename/brace/binary — the `=>`-without-`-M` fact
defuses the repo's wrong comment), whose resolver algorithm is specified for both rename forms (simple +
brace-collapse, with the suffix-collapsed variant), whose `--numstat`-before-`--` placement handles the
excludes case for the downstream sizing caller, and whose tests are dual (deterministic pure table for the
brace forms + integration for the real git shapes). No edit to any existing file (NEW numstat.go +
numstat_test.go; numstatRows is a `*gitRunner` method, not on the interface), no new deps. The one residual
risk — git's brace-collapse heuristics varying slightly across versions for which prefix/suffix it folds —
is covered by the pure resolver table (deterministic) plus an integration test that asserts the RESOLVED
destination (not the raw brace string), so a git-version difference in collapse style still yields the
correct destination path.
