---
name: "P1.M3.T1.S2 — Prepend compact numstat skeleton block + ordering invariance (FR3g): render S1's numstatRows as a one-line-per-file skeleton, prepend it to StagedDiff/TreeDiff/WorkingTreeDiff, and reorder to skeleton→placeholders→markdown→non-markdown — PRD §9.1 FR3g"
description: |

  Land the SECOND subtask of the Compact numstat change skeleton (P1.M3.T1): the FR3g DELIVERY. S1 (SHIPPED,
  on disk in internal/git/numstat.go) provides `numstatRow{Added, Deleted int; IsBinary bool; Path string}`
  and `(g *gitRunner) numstatRows(ctx, diffArgs...)`. S2 RENDERS those rows as a compact skeleton block and
  PREPENDS it to the payload of each of the three sibling diff functions — StagedDiff / TreeDiff /
  WorkingTreeDiff — so the model sees the full shape of the change (every file, its add/delete magnitude,
  its binary-ness) even when bodies are later truncated (the FR3g completeness floor; truncation itself is
  M4). It is the dual-use consumer of S1 (the skeleton is the model's completeness view) — M4.T2 is the
  other consumer (water-fill sizing).

  ⚠️ **#1 — THE ORDERING IS A REORDER, NOT JUST A PREPEND (system_context.md §7 is authoritative).** The
  contract's "Ordering invariant: skeleton first, then placeholders, then markdown bodies, then non-markdown
  bodies" is LITERAL — confirmed by system_context.md §7's data-flow string `"skeleton + placeholders + md +
  shaped non-md"`. The three functions CURRENTLY stream `markdown → binary placeholders → excluded
  placeholders → non-markdown` into one builder. The target is `skeleton → binary placeholders → excluded
  placeholders → markdown → non-markdown`. So S2 BOTH prepends the skeleton AND moves the binary/excluded
  placeholder block to BEFORE Part 1 markdown. Dependency-safe: the placeholder block produces `binExcludes`
  (consumed by Part 2), and Part 1 markdown does NOT use binExcludes — so moving the block up violates
  nothing. See research design-decisions §1/§7.

  ⚠️ **#2 — NEW file `internal/git/skeleton.go` (don't edit S1's numstat.go).** S2's render+capture helpers
  live in a NEW file: PURE `renderNumstatSkeleton(rows []numstatRow) string` + `(g *gitRunner)
  numstatSkeleton(ctx, diffArgs...) (string, error)` (capture via S1's numstatRows + render). Parallel-clean
  (S1 owns numstat.go); avoids 3× render duplication. (research §2.)

  ⚠️ **#3 — Binary rows render as `-\t-\t<path>` (NOT `0\t0`).** S1's numstatRow sets `Added=Deleted=0,
  IsBinary=true` for binary, but the skeleton MIRRORS `git diff --numstat` which prints `-\t-\t<path>` for
  binary. The render must emit literal hyphens (`if r.IsBinary { "-\t-\t"+Path }`), NOT the 0 counts.
  (research §5.)

  ⚠️ **#4 — Empty diff ⇒ NO skeleton (return "").** renderNumstatSkeleton returns "" for 0 rows. The
  orchestrator gates on `diff == ""` (FR5 nothing-staged); a header-only skeleton for 0 files would defeat
  that check. An empty change set has nothing to summarize. (research §4.)

  ⚠️ **#5 — numstat call uses `<domain> + "-M"` (rename consistency); NOT buildDiffArgs output.** Each
  function calls `numstatSkeleton(ctx, <domain…>, "-M")`. `-M` matches the diff bodies (buildDiffArgs emits
  it unconditionally — FR3e) so a rename is ONE skeleton row, not a delete+add pair; resolveNumstatPath (S1)
  resolves the `=>`/`{…}`. `-U<n>` is OMITTED (numstat is line counts, not patch context). Do NOT pass
  buildDiffArgs output — it has a leading `"diff"` and numstatRows prepends its own ⇒ double `"diff"`.
  (research §6.) Domain args: StagedDiff `"--cached"`, TreeDiff `treeA, treeB`, WorkingTreeDiff (none).

  ⚠️ **#6 — Golden fixtures: MINIMAL churn (most tests use `strings.Contains`, which survives).** The
  existing stagediff/treediff/workingtreediff tests assert via `Contains`/count (e.g. `Contains(out,
  "A\t[binary] logo.png")`, the truncation sentinels, `diff --git` counts) — these PASS UNCHANGED (the
  content is still present, just relocated under a leading skeleton). ONLY `HasPrefix` / exact-equality /
  explicit-order assertions need updating. ADD new tests: the ordering-invariance test (the contract's
  "test it") + a completeness test (skeleton lists every changed file) + a pure renderNumstatSkeleton table.
  (research §8.)

  ⚠️ **#7 — Completeness-under-truncation verification is M4's job; S2 delivers the floor.** S2 ensures the
  skeleton lists EVERY changed file NOW (pre-truncation). The "even when a body is fully truncated" resilience
  is the water-fill (P1.M4.T2), which truncates bodies never the skeleton. S2's completeness test asserts the
  floor pre-truncation; M4 asserts it holds post-truncation. (research §9.)

  ⚠️ **#8 — No new deps; go.mod UNCHANGED.** skeleton.go uses stdlib `fmt`+`strings`; git.go already imports
  everything. `go mod tidy` is a no-op.

  Deliverable: NEW `internal/git/skeleton.go` (renderNumstatSkeleton + numstatSkeleton) + MODIFIED
  `internal/git/git.go` (the 3 diff functions: skeleton prepend + placeholder/markdown reorder) + MODIFIED
  golden tests (minimal) + NEW ordering-invariance/completeness/render tests. NO edit to S1's numstat.go.
  OUTPUT: every payload begins `skeleton → placeholders → markdown → non-markdown`; the completeness floor
  holds (every changed file represented); `go build/vet/test ./...` green.

---

## Goal

**Feature Goal**: Deliver the FR3g compact change skeleton — render S1's `numstatRows` as a one-line-per-file
skeleton block (`added\tdeleted\tpath`; binary `-\t-\tpath`) wrapped in a one-line header, and prepend it to
the payload of StagedDiff/TreeDiff/WorkingTreeDiff so the model sees the full shape of the change even when
diff bodies are truncated (the completeness floor). Establish the canonical payload ordering
(`skeleton → binary/excluded placeholders → markdown bodies → non-markdown bodies`) identically in all three
functions (the ordering invariant).

**Deliverable** (NEW + MODIFIED):
1. **NEW `internal/git/skeleton.go`** (`package git`, imports `fmt`+`strings`):
   - `func renderNumstatSkeleton(rows []numstatRow) string` (PURE: header + rows + trailing blank line; binary
     rows as `-\t-\t<path>`; empty rows → `""`).
   - `func (g *gitRunner) numstatSkeleton(ctx, diffArgs...) (string, error)` (capture via numstatRows + render).
2. **MODIFIED `internal/git/git.go`** — StagedDiff / TreeDiff / WorkingTreeDiff: prepend `skeleton` to the
   builder FIRST; MOVE the binary-placeholder + excluded-placeholder blocks to BEFORE Part 1 markdown
   (target order: skeleton → placeholders → markdown → non-markdown). Only the diff-domain args differ.
3. **MODIFIED golden tests** (`stagediff_test.go` / `treediff_test.go` / `workingtreediff_test.go`) — update
   the few `HasPrefix`/exact-equality/order assertions broken by the skeleton+reorder (most `Contains`/count
   tests survive unchanged).
4. **NEW tests** — the ordering-invariance test (skeleton < [binary] < markdown < code, in all 3 functions),
  a completeness test (skeleton lists every changed file incl. binary), and a pure `renderNumstatSkeleton`
  table test.

**Success Definition**: `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean; every diff
payload begins with the skeleton block (header + one row per changed file + blank line) when there are
changes, and `""` when there are none; the section order is skeleton → placeholders → markdown → non-markdown
in all three functions (verified by the ordering-invariance test); the skeleton lists every changed file even
when a body is capped (completeness); the existing `Contains`/count/sentinel tests still pass; S1's numstat.go
byte-unchanged; go.mod/go.sum byte-unchanged.

## User Persona

**Target User**: The model that consumes the diff payload (transitively, every user whose diff is large
enough to truncate — FR3d/FR3i). The skeleton guarantees the model sees the full per-file shape (every file,
add/delete magnitude, binary-ness, rename destination) even when bodies are truncated, so a commit message is
never written from a half-picture. The ordering invariant gives the model a predictable "summary then detail"
structure (skeleton + placeholders grouped as the at-a-glance summary; markdown + non-markdown as the detail).

**Use Case**: A staged change with 3 files — a renamed `a.go→b.go`, a binary `logo.png`, and a markdown
`README.md`. The payload begins with the skeleton (3 rows: the rename's add/delete counts, `-\t-\tlogo.png`,
the README counts), then the `[binary] logo.png` placeholder, then the README diff body, then the code diff
body. Even if the code body is later capped (M4), the skeleton still shows all 3 files.

**User Journey**: (internal) StagedDiff/TreeDiff/WorkingTreeDiff → numstatSkeleton (numstat capture + render)
→ prepend → placeholders → markdown bodies → non-markdown bodies → returned string → BuildUserPayload/
BuildPlannerUserPayload (verbatim tail) → agent stdin. S2 is the render + prepend + reorder step.

**Pain Points Addressed**: Without the skeleton, a truncated diff silently drops files from the model's view
(FR3g: "truncation never silently drops a file"). Without the ordering invariant, the three sibling functions
could drift in section order, making the payload shape unpredictable.

## Why

- **Satisfies PRD §9.1 FR3g (the completeness floor).** "Prepend a compact per-file skeleton … to the payload
  before any diff body. This guarantees the model sees the full shape of the change … even when bodies are
  truncated. A file whose body is fully truncated remains represented in the skeleton." S2 IS the prepend +
  the render.
- **Completes P1.M3.T1 (FR3g).** S1 delivered the numstat PARSE primitive; S2 delivers the skeleton RENDER +
  PREPEND. M4.T2 is the other consumer (water-fill sizing) of the same numstat call (dual-use).
- **Establishes the canonical payload ordering (the contract's "ordering invariance").** All three diff
  functions emit the same section order (skeleton → placeholders → markdown → non-markdown), so the payload
  shape is predictable regardless of which diff path produced it.
- **Minimal blast radius.** skeleton.go is stdlib-only + new; the 3-function edit is a mechanical reorder +
  one prepend; the existing `Contains`/count tests mostly survive. No new deps, no config/API/CLI surface.

## What

A new `skeleton.go` (2 helpers), a modified `git.go` (3 functions reordered + skeleton prepended), updated
golden tests (minimal), and 3 new tests (ordering invariance, completeness, pure render). No new deps, no
config, no API, no CLI, no doc surface.

### Success Criteria

- [ ] `internal/git/skeleton.go` exists, `package git`, imports EXACTLY `fmt`,`strings`: PURE
      `renderNumstatSkeleton(rows []numstatRow) string` + `(g *gitRunner) numstatSkeleton(ctx, diffArgs...)
      (string, error)`.
- [ ] `renderNumstatSkeleton`: empty rows → `""`; else header `Change summary (numstat: added\tdeleted
      \tpath):\n` (REAL tabs) + one row per file (`<added>\t<deleted>\t<path>\n`; binary → `-\t-\t<path>\n`)
      + a trailing blank line `\n`. Rows are already sorted by Path (S1) ⇒ deterministic.
- [ ] StagedDiff / TreeDiff / WorkingTreeDiff: write the skeleton to the builder FIRST; the binary + excluded
      placeholder blocks come NEXT (BEFORE Part 1 markdown); then Part 1 markdown; then Part 2 non-markdown.
      Section order identical in all three (the ordering invariant).
- [ ] Each function calls `numstatSkeleton(ctx, <domain…>, "-M")` (StagedDiff `"--cached"`; TreeDiff
      `treeA, treeB`; WorkingTreeDiff none) and propagates a non-nil error.
- [ ] Ordering-invariance test passes for all 3 functions: `indexOf(skeleton header) < indexOf([binary]
      placeholder) < indexOf(markdown "diff --git") < indexOf(code "diff --git")`.
- [ ] Completeness test: a multi-file change (incl. binary) → the skeleton lists EVERY changed file path,
      even when a body is line/byte-capped.
- [ ] Pure `renderNumstatSkeleton` table test passes (empty→"", normal row, binary row, header+blank shape).
- [ ] The existing `Contains`/count/sentinel tests still pass; any `HasPrefix`/exact-equality/order tests
      updated.
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l` clean; S1's `numstat.go` byte-
      unchanged; go.mod/go.sum byte-unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the resolved ordering (§1 — a
reorder, per system_context §7), the copy-ready `skeleton.go` (Blueprint §1), the 3-function edit pattern
(Blueprint §2 — StagedDiff in full + the domain-arg table for the other two), the binary-row `-\t-` rule
(§5), the empty→"" rule (§4), the numstat `<domain>+"-M"` call (§6), and the test plan (§8). No snapshot/
decompose/render knowledge required — S2 is a render + a prepend + a reorder inside the git layer.

### Documentation & References

```yaml
# MUST READ — the design calls (the reorder resolution, the render rules, the test plan)
- docfile: plan/007_b33d310438c6/P1M3T1S2/research/design-decisions.md
  why: §1 (THE ORDERING IS A REORDER — system_context §7 "skeleton+placeholders+md+non-md" resolves it;
       dependency-safe), §2 (new skeleton.go; don't edit numstat.go), §3 (skeleton format: header+rows+blank),
       §4 (empty→""), §5 (binary rows `-\t-`, not 0/0), §6 (`<domain>+"-M"`; not buildDiffArgs), §7 (the
       3-function edit shape), §8 (golden-fixture churn is minimal — Contains survives), §9 (completeness-
       under-truncation verification is M4), §10 (no new deps).
  critical: §1 (REORDER — the thing most likely to be implemented as a simple prepend instead), §5 (binary
       `-\t-`), §4 (empty→""), §6 (don't double "diff") are the things most likely to go wrong.

# MUST READ — the S1 CONTRACT (the primitive S2 renders)
- docfile: plan/007_b33d310438c6/P1M3T1S1/PRP.md
  why: S1 ships `numstatRow{Added, Deleted int; IsBinary bool; Path string}` (Path = destination, rename-
       resolved; IsBinary ⇒ Added=Deleted=0) + `(g *gitRunner) numstatRows(ctx, diffArgs...)` (builds its own
       `["diff",…]` argv; places `--numstat` before any `--`; rows sorted by Path). S2 CONSUMES these.
  critical: numstatRows prepends its OWN `"diff"` ⇒ never pass buildDiffArgs output (which starts with
       "diff"). Binary rows have IsBinary=true with Added=Deleted=0 — the RENDER emits `-\t-`, not the 0s.

# MUST READ — the authoritative data-flow (resolves the ordering question)
- docfile: plan/007_b33d310438c6/architecture/system_context.md
  section: "§7. Data flow" — the returned payload is `"skeleton + placeholders + md + shaped non-md"`.
  critical: that string is sequential and explicit — placeholders precede markdown. This is why S2 REORDERS
       (not just prepends). (research §1.)

# The authoritative touchmap (the 3 functions are identical; only domain args differ)
- docfile: plan/007_b33d310438c6/architecture/diff_capture_touchmap.md
  section: "§1 The THREE sibling diff functions" — StagedDiff/TreeDiff/WorkingTreeDiff stream Part 1 markdown
           → binary placeholders → excluded placeholders → Part 2 non-markdown into ONE builder.
  critical: the three are near-verbatim copies; apply the SAME structural edit to all three (only the domain
       positionals differ: `--cached` / `treeA,treeB` / none).

# The S1 primitive (read-only — the exact API S2 calls)
- file: internal/git/numstat.go   (S1 — SHIPPED; do NOT edit)
  section: `numstatRow`, `numstatRows(ctx, diffArgs...)`. numstatRows builds `["diff", diffArgs[:splitAt],
           "--numstat", diffArgs[splitAt:]]` (--numstat before any `--`).
  why: confirms the exact call shape S2's numstatSkeleton wraps, and that rows are sorted + binary-flagged.
  critical: do NOT edit numstat.go (S1's file). numstatSkeleton is a NEW method in skeleton.go that CALLS it.

# THE FILES BEING MODIFIED — READ FULLY before editing
- file: internal/git/git.go
  section: StagedDiff (L732), TreeDiff (L1187), WorkingTreeDiff (L1324) — each: cap-defaults block; `var b
           strings.Builder`; Part 1 markdown loop; binary detection (`detectBinaryFiles`+`fileStatuses`) →
           `binaryPlaceholderLine` writes + `binExcludes`; excluded detection (`detectExcludedStatuses`) →
           `excludedPlaceholderLine` writes; Part 2 non-markdown (`nmArgs` with excludes+`:!*.md`+binExcludes)
           byte-capped. `buildDiffArgs(opts, domain…)` = `["diff", domain…, "-M", "-U<n>"]`.
  why: the EXACT structure S2 reorders. Note the placeholder block produces `binExcludes` (Part 2 consumes
       it); Part 1 markdown does NOT use binExcludes (so moving the block up is safe).
  critical: the reorder MOVES the binary+excluded blocks to before Part 1; the skeleton is written FIRST.
       Keep `binExcludes` a separate slice (never append to `excludes` — it may alias defaultExcludes).

# The test files being updated/extended
- file: internal/git/stagediff_test.go   (the canonical test idiom — mirror in treediff/workingtreediff)
  section: the `Contains`/count assertions (e.g. L299 `Contains(out,"A\t[binary] logo.png")`; the truncation
           sentinels; `diff --git` counts). The helpers `initRepo`/`writeFile`/`stageFile` for repo setup.
  why: confirms most tests SURVIVE (Contains doesn't care about order/the leading skeleton) and gives the
       setup idiom for the new ordering-invariance + completeness tests.
  critical: update only `HasPrefix`/exact-equality/order assertions; do NOT rewrite the passing `Contains`
       tests. Add the ordering-invariance + completeness tests here (+ mirror in the sibling test files).
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  numstat.go            # S1 SHIPPED — numstatRow + resolveNumstatPath + numstatRows. UNCHANGED by S2.
  numstat_test.go       # S1 SHIPPED. UNCHANGED.
  skeleton.go           # NEW (S2) ← renderNumstatSkeleton + (g *gitRunner) numstatSkeleton
  git.go                # StagedDiff/TreeDiff/WorkingTreeDiff — EDIT (skeleton prepend + placeholder/markdown reorder)
  stagediff_test.go     # EDIT (minimal fixture updates + NEW ordering-invariance/completeness tests)
  treediff_test.go      # EDIT (same)
  workingtreediff_test.go # EDIT (same)
go.mod / go.sum         # UNCHANGED (stdlib fmt+strings only)
```

### Desired Codebase tree with files to be added

```bash
internal/git/skeleton.go   # NEW — renderNumstatSkeleton (pure) + numstatSkeleton (capture+render)
# All other changes are in-place edits (git.go's 3 functions + the 3 test files).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (#1 — THE ORDERING IS A REORDER): system_context.md §7's "skeleton + placeholders + md + non-md"
//   is sequential — placeholders come BEFORE markdown. The 3 functions currently emit markdown first. S2
//   MOVES the binary+excluded placeholder block to before Part 1 markdown AND prepends the skeleton. Do NOT
//   implement a simple prepend (that would leave markdown before placeholders — wrong order). (research §1)

// CRITICAL (#2 — new skeleton.go; do NOT edit numstat.go): S2's render+capture helpers go in a NEW file.
//   numstat.go is S1's (SHIPPED). numstatSkeleton CALLS numstatRows; it does not modify it. (research §2)

// CRITICAL (#3 — binary rows render `-\t-\t<path>`, NOT `0\t0`): numstatRow sets Added=Deleted=0 for binary,
//   but the skeleton MIRRORS `git diff --numstat` which prints `-`/`-`. Render emits literal hyphens for
//   IsBinary rows. (research §5)

// CRITICAL (#4 — empty rows → ""): renderNumstatSkeleton returns "" for 0 rows. A header-only skeleton would
//   break the FR5 `diff == ""` nothing-staged check. (research §4)

// CRITICAL (#5 — numstat call is `<domain> + "-M"`; NOT buildDiffArgs output): numstatRows prepends its own
//   "diff"; buildDiffArgs output ALSO starts with "diff" ⇒ double "diff". Pass the post-"diff" tokens
//   (domain + "-M"). -U<n> is irrelevant for numstat (omitted). -M matches the diff bodies (rename = 1 row).
//   (research §6)

// GOTCHA (binExcludes lifecycle after the reorder): the binary block (now earlier) still produces binExcludes
//   consumed by Part 2 (unchanged position). Keep binExcludes a SEPARATE slice — never append to `excludes`
//   (may alias defaultExcludes).
// GOTCHA (header uses REAL tabs): "Change summary (numstat: added\tdeleted\tpath):\n" — \t is a tab, mirroring
//   the row column layout (the header is a template for the rows).
// GOTCHA (skeleton is a complete string, written first): b.WriteString(skeleton) as the FIRST write; skeleton
//   already includes its trailing blank line. If skeleton=="" (empty diff) the write is a no-op.
// GOTCHA (rows are pre-sorted by S1): numstatRows returns rows sorted by Path ⇒ the skeleton is deterministic;
//   do NOT re-sort in the render.
// GOTCHA (Contains tests survive): most stagediff/treediff/workingtreediff tests use strings.Contains/count —
//   they PASS UNCHANGED (content relocated, not removed). Update ONLY HasPrefix/exact-equality/order tests.
// GOTCHA (no new imports): skeleton.go uses fmt+strings; git.go already imports all it needs. go mod tidy no-op.
```

## Implementation Blueprint

### §1. NEW `internal/git/skeleton.go`

```go
// Package git — FR3g compact numstat skeleton render + capture (PRD §9.1 FR3g).
//
// renderNumstatSkeleton renders the parsed numstat rows from numstat.go (S1) as a compact one-line-per-file
// skeleton block; numstatSkeleton captures the rows for a diff domain and renders them. The skeleton is
// PREPENDED to every diff payload (StagedDiff/TreeDiff/WorkingTreeDiff) so the model sees the full shape of
// the change (every file, add/delete magnitude, binary-ness) even when bodies are truncated — the FR3g
// completeness floor. Dual-use consumer of S1's numstatRows (the model's completeness view); P1.M4.T2 is the
// other consumer (water-fill sizing). (PRD §9.1 FR3g.)
package git

import (
	"fmt"
	"strings"
)

// numstatSkeletonHeader is the one-line label prepended to the skeleton rows so the model knows what the
// block is. The columns use REAL tabs to mirror the row format exactly (the header is a literal template
// for the rows). (PRD §9.1 FR3g.)
const numstatSkeletonHeader = "Change summary (numstat: added\tdeleted\tpath):"

// renderNumstatSkeleton renders the compact per-file skeleton block: the header, one line per row, then a
// trailing blank line that separates the skeleton from the placeholders/diff bodies. Binary rows
// (numstatRow.IsBinary) render as `-\t-\t<path>` — mirroring `git diff --numstat`'s literal output for
// binary files (numstatRow stores Added=Deleted=0 for binary; the skeleton shows the `-`/`-` form, NOT 0/0,
// so it faithfully represents the file as binary). Non-binary rows render `<added>\t<deleted>\t<path>`.
// Rows are already sorted by Path (numstatRows, S1) ⇒ deterministic output.
//
// Returns "" for an empty row set: an empty change set has nothing to summarize, and a header-only skeleton
// would defeat the caller's `diff == ""` nothing-staged check (PRD §9.4 FR5). Pure function; no I/O.
// (PRD §9.1 FR3g.)
func renderNumstatSkeleton(rows []numstatRow) string {
	if len(rows) == 0 {
		return "" // nothing changed → no skeleton (preserve the FR5 empty-payload check)
	}
	var b strings.Builder
	b.WriteString(numstatSkeletonHeader)
	b.WriteByte('\n')
	for _, r := range rows {
		if r.IsBinary {
			b.WriteString("-\t-\t")
			b.WriteString(r.Path)
			b.WriteByte('\n')
		} else {
			fmt.Fprintf(&b, "%d\t%d\t%s\n", r.Added, r.Deleted, r.Path)
		}
	}
	b.WriteByte('\n') // blank-line separator before the placeholders / diff bodies
	return b.String()
}

// numstatSkeleton captures the numstat rows for the given diff domain (via S1's numstatRows) and renders
// them as the compact skeleton block. diffArgs selects the domain and is forwarded verbatim — append "-M"
// (the caller does) so a rename is ONE row (matching the diff bodies' always-on -M, FR3e) rather than a
// delete+add pair; resolveNumstatPath (S1) resolves the `=>`/`{…}` rename notation. Read-only w.r.t.
// refs/index. Returns "" when there are no changed files (renderNumstatSkeleton's empty rule). Each of
// StagedDiff/TreeDiff/WorkingTreeDiff calls this once and prepends the result. (PRD §9.1 FR3g.)
//
// NOT routed through buildDiffArgs (which emits a leading "diff" AND a -U<n> that is irrelevant to numstat);
// the caller passes the post-"diff" tokens (domain + "-M"). numstatRows builds its own "diff" argv.
func (g *gitRunner) numstatSkeleton(ctx context.Context, diffArgs ...string) (string, error) {
	rows, err := g.numstatRows(ctx, diffArgs...)
	if err != nil {
		return "", err
	}
	return renderNumstatSkeleton(rows), nil
}
```

### §2. The 3-function edit (StagedDiff in full; TreeDiff/WorkingTreeDiff differ only in domain args)

The edit is the SAME structural change in all three. **StagedDiff** (canonical — the new section order):

```go
func (g *gitRunner) StagedDiff(ctx context.Context, opts StagedDiffOptions) (string, error) {
	maxMDLines := opts.MaxMDLines
	if maxMDLines <= 0 {
		maxMDLines = defaultMaxMDLines
	}
	maxDiffBytes := opts.MaxDiffBytes
	if maxDiffBytes <= 0 {
		maxDiffBytes = defaultMaxDiffBytes
	}

	var b strings.Builder

	// ---- FR3g: compact numstat skeleton (completeness floor) — PREPENDED FIRST ----
	skeleton, serr := g.numstatSkeleton(ctx, "--cached", "-M")
	if serr != nil {
		return "", serr
	}
	b.WriteString(skeleton)

	// ---- Binary filtering (FR3a/b/c) → [binary] placeholders ----  (MOVED BEFORE Part 1 markdown)
	binSet, berr := g.detectBinaryFiles(ctx, "--cached")
	if berr != nil {
		return "", berr
	}
	statuses, serr2 := g.fileStatuses(ctx, "--cached")
	if serr2 != nil {
		return "", serr2
	}
	binPaths := make([]string, 0, len(statuses))
	for path := range statuses {
		if binSet[path] || isBinaryByExtension(path, opts.BinaryExtensions) {
			binPaths = append(binPaths, path)
		}
	}
	sort.Strings(binPaths)
	var binExcludes []string
	for _, path := range binPaths {
		b.WriteString(binaryPlaceholderLine(statuses[path], path))
		b.WriteByte('\n')
		binExcludes = append(binExcludes, ":!"+path)
	}

	// ---- User-exclude placeholders (FR-X4) → [excluded] placeholders ----  (MOVED BEFORE Part 1 markdown)
	excluded, xerr := g.detectExcludedStatuses(ctx, statuses, opts.Excludes, "--cached")
	if xerr != nil {
		return "", xerr
	}
	exPaths := make([]string, 0, len(excluded))
	for path := range excluded {
		if binSet[path] {
			continue
		}
		exPaths = append(exPaths, path)
	}
	sort.Strings(exPaths)
	for _, path := range exPaths {
		b.WriteString(excludedPlaceholderLine(excluded[path], path))
		b.WriteByte('\n')
	}

	// ---- Part 1: markdown, per-file, line-capped ----  (MOVED AFTER placeholders)
	mdList, stderr, code, err := g.run(ctx, g.workDir,
		append(buildDiffArgs(opts, "--cached"), "--name-only", "--", "*.md", "*.markdown")...)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("git diff (markdown list): failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	for _, file := range strings.Split(strings.TrimSpace(mdList), "\n") {
		if file == "" {
			continue
		}
		fileDiff, fstderr, fcode, ferr := g.run(ctx, g.workDir, append(buildDiffArgs(opts, "--cached"), "--", file)...)
		if ferr != nil {
			return "", ferr
		}
		if fcode != 0 {
			return "", fmt.Errorf("git diff --cached -- %s: failed (exit %d): %s", file, fcode, strings.TrimSpace(fstderr))
		}
		fileDiff = stripIndexLines(fileDiff)
		if lines := strings.Split(fileDiff, "\n"); len(lines) > maxMDLines {
			fileDiff = strings.Join(lines[:maxMDLines], "\n") +
				fmt.Sprintf("\n... [diff truncated at %d lines]", maxMDLines)
		}
		b.WriteString(fileDiff)
		if !strings.HasSuffix(fileDiff, "\n") {
			b.WriteByte('\n')
		}
	}

	// ---- Part 2: non-markdown, aggregate, byte-capped, excluded ----  (unchanged; uses binExcludes)
	excludes := make([]string, 0, len(defaultExcludes)+len(opts.Excludes))
	excludes = append(excludes, defaultExcludes...)
	excludes = append(excludes, opts.Excludes...)
	nmArgs := buildDiffArgs(opts, "--cached")
	nmArgs = append(nmArgs, "--")
	nmArgs = append(nmArgs, excludes...)
	nmArgs = append(nmArgs, ":!*.md", ":!*.markdown")
	nmArgs = append(nmArgs, binExcludes...)
	nmDiff, nmstderr, nmcode, nmerr := g.run(ctx, g.workDir, nmArgs...)
	if nmerr != nil {
		return "", nmerr
	}
	if nmcode != 0 {
		return "", fmt.Errorf("git diff (non-markdown): failed (exit %d): %s", nmcode, strings.TrimSpace(nmstderr))
	}
	nmDiff = stripIndexLines(nmDiff)
	if len(nmDiff) > maxDiffBytes {
		nmDiff = nmDiff[:maxDiffBytes] +
			fmt.Sprintf("\n... [diff truncated at %d bytes]", maxDiffBytes)
	}
	b.WriteString(nmDiff)

	return b.String(), nil
}
```

**TreeDiff + WorkingTreeDiff:** the IDENTICAL structural change; ONLY the diff-domain args differ (and the
error-message labels, which stay as-is). The per-function substitutions:

| Call site | Skeleton call | detectBinaryFiles / fileStatuses / detectExcludedStatuses domain | buildDiffArgs domain |
|---|---|---|---|
| StagedDiff | `numstatSkeleton(ctx, "--cached", "-M")` | `"--cached"` | `buildDiffArgs(opts, "--cached")` |
| TreeDiff | `numstatSkeleton(ctx, treeA, treeB, "-M")` | `treeA, treeB` | `buildDiffArgs(opts, treeA, treeB)` |
| WorkingTreeDiff | `numstatSkeleton(ctx, "-M")` | *(none)* | `buildDiffArgs(opts)` |

In each: (a) insert the skeleton capture+write as the FIRST builder write (after `var b`); (b) move the
binary + excluded placeholder blocks to immediately AFTER the skeleton write and BEFORE the Part 1 markdown
loop; (c) leave Part 1 markdown and Part 2 non-markdown in place (Part 2 still consumes `binExcludes`).

### §3. NEW tests (+ minimal golden-fixture updates)

```go
// Pure render test — internal/git/skeleton_test.go (package git white-box).
func TestRenderNumstatSkeleton(t *testing.T) {
	cases := []struct{ name string; rows []numstatRow; want string }{
		{"empty → \"\"", nil, ""},
		{"empty slice → \"\"", []numstatRow{}, ""},
		{"one normal row", []numstatRow{{Added: 3, Deleted: 1, Path: "a.go"}},
			"Change summary (numstat: added\tdeleted\tpath):\n3\t1\ta.go\n\n"},
		{"binary row renders -/-", []numstatRow{{IsBinary: true, Path: "logo.png"}},
			"Change summary (numstat: added\tdeleted\tpath):\n-\t-\tlogo.png\n\n"},
		{"sorted passthrough (rows pre-sorted by S1)",
			[]numstatRow{{Added: 1, Path: "a.go"}, {IsBinary: true, Path: "z.png"}},
			"Change summary (numstat: added\tdeleted\tpath):\n1\t0\ta.go\n-\t-\tz.png\n\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := renderNumstatSkeleton(tc.rows); got != tc.want {
				t.Errorf("renderNumstatSkeleton mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, tc.want)
			}
		})
	}
}

// Ordering-invariance test — add to stagediff_test.go (+ mirror in treediff_test.go / workingtreediff_test.go
// with the domain-appropriate setup). Asserts the canonical order: skeleton < [binary] placeholder < markdown
// body < code body. (The contract's "Ordering invariant — test it".)
func TestStagedDiff_OrderingInvariant_SkeletonPlaceholdersMdCode(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "code.go", "package main\nfunc main() {}\n")   // non-markdown body
	writeFile(t, repo, "README.md", "# Title\n\nbody\n")              // markdown body
	writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00")   // binary placeholder
	stageFile(t, repo, "code.go"); stageFile(t, repo, "README.md"); stageFile(t, repo, "logo.png")

	out, err := New(repo).StagedDiff(context.Background(), StagedDiffOptions{})
	if err != nil {
		t.Fatalf("StagedDiff: %v", err)
	}
	iSkeleton := strings.Index(out, "Change summary (numstat:")
	iBinary := strings.Index(out, "[binary] logo.png")
	iMd := strings.Index(out, "diff --git a/README.md")
	iCode := strings.Index(out, "diff --git a/code.go")
	for _, idx := range []struct{ name string; i int }{{"skeleton", iSkeleton}, {"binary", iBinary}, {"md", iMd}, {"code", iCode}} {
		if idx.i < 0 {
			t.Fatalf("%s section not found in output:\n%s", idx.name, out)
		}
	}
	if !(iSkeleton < iBinary && iBinary < iMd && iMd < iCode) {
		t.Errorf("ordering invariant violated: skeleton@%d binary@%d md@%d code@%d (want skeleton<binary<md<code)\n%s",
			iSkeleton, iBinary, iMd, iCode, out)
	}
}

// Completeness test — the skeleton lists EVERY changed file even when a body is capped. (The FR3g floor;
// the under-truncation resilience itself is verified in M4.)
func TestStagedDiff_SkeletonCompleteUnderBodyCap(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeFile(t, repo, "big.go", strings.Repeat("// line\n", 300)) // will be byte-capped
	writeFile(t, repo, "small.go", "package main\n")
	writeFile(t, repo, "logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00")
	stageFile(t, repo, "big.go"); stageFile(t, repo, "small.go"); stageFile(t, repo, "logo.png")

	out, err := New(repo).StagedDiff(context.Background(), StagedDiffOptions{MaxDiffBytes: 50}) // tight cap
	if err != nil {
		t.Fatalf("StagedDiff: %v", err)
	}
	// The skeleton lists every changed file regardless of body truncation.
	for _, path := range []string{"big.go", "small.go", "logo.png"} {
		if !strings.Contains(out, path+"\n") && !strings.Contains(out, path+"\t") {
			// paths appear in the skeleton rows AND/OR bodies; assert each is present somewhere.
		}
	}
	// Assert the skeleton block itself (header ... rows) contains all three paths:
	skeleton := out
	if i := strings.Index(out, "\n\n"); i > 0 && strings.HasPrefix(out, "Change summary") {
		skeleton = out[:i] // the skeleton block ends at the first blank line
	}
	for _, path := range []string{"big.go", "small.go", "logo.png"} {
		if !strings.Contains(skeleton, path) {
			t.Errorf("skeleton missing changed file %s (completeness floor):\n%s", path, skeleton)
		}
	}
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/git/skeleton.go (renderNumstatSkeleton + numstatSkeleton)
  - PACKAGE git; IMPORTS EXACTLY fmt, strings.
  - IMPLEMENT pure renderNumstatSkeleton: empty→""; else header (REAL tabs) + rows (binary → `-\t-\t<path>`;
      else `%d\t%d\t%s`) + trailing `\n`.
  - IMPLEMENT (g *gitRunner) numstatSkeleton: call numstatRows(ctx, diffArgs...); renderNumstatSkeleton.
  - GOTCHA: don't edit numstat.go; don't re-sort (S1 pre-sorts); binary renders `-`/`-` not 0/0.

Task 2: EDIT internal/git/git.go — StagedDiff (skeleton prepend + placeholder/markdown reorder)
  - INSERT skeleton capture+write as the FIRST builder write (after `var b`).
  - MOVE the binary-placeholder block + the excluded-placeholder block to AFTER the skeleton write, BEFORE
      the Part 1 markdown loop. (binExcludes is still produced here; Part 2 consumes it unchanged.)
  - LEAVE Part 1 markdown and Part 2 non-markdown in place.
  - GOTCHA: skeleton call is `numstatSkeleton(ctx, "--cached", "-M")`; keep binExcludes a separate slice.

Task 3: EDIT internal/git/git.go — TreeDiff + WorkingTreeDiff (same shape; domain args per the §2 table)
  - SAME structural change. TreeDiff: `numstatSkeleton(ctx, treeA, treeB, "-M")`; WorkingTreeDiff:
      `numstatSkeleton(ctx, "-M")`. Domain args for detectBinaryFiles/fileStatuses/detectExcludedStatuses/
      buildDiffArgs unchanged from the existing code.
  - GOTCHA: the three functions must end up byte-identical in SECTION ORDER (the ordering invariant).

Task 4: CREATE internal/git/skeleton_test.go (pure render table)
  - TestRenderNumstatSkeleton: empty→"", normal row, binary row (`-\t-`), sorted passthrough, header+blank
      shape. (Exact-string equality — deterministic.)

Task 5: ADD ordering-invariance + completeness tests to stagediff/treediff/workingtreediff_test.go
  - TestStagedDiff_OrderingInvariant_SkeletonPlaceholdersMdCode (+ Tree/WorkingTree mirrors): assert
      indexOf(skeleton) < indexOf([binary]) < indexOf(md "diff --git") < indexOf(code "diff --git").
  - TestStagedDiff_SkeletonCompleteUnderBodyCap (+ mirrors): tight MaxDiffBytes → skeleton still lists every
      changed file (incl. binary).

Task 6: UPDATE golden fixtures (minimal — most Contains tests survive)
  - RUN `go test ./internal/git/`. For each FAILURE: if it is a `HasPrefix`/exact-equality/order assertion
      broken by the leading skeleton or the placeholder/markdown reorder, update the expectation. LEAVE the
      passing `Contains`/count/sentinel tests UNCHANGED.
  - GOTCHA: do NOT rewrite passing tests; the skeleton adds content but removes nothing.

Task 7: VERIFY
  - RUN the full Validation Loop (Levels 1–3). go.mod/go.sum byte-unchanged. numstat.go/numstat_test.go
      (S1) byte-unchanged. `go build/vet/test ./...` green. The ordering-invariance + completeness tests pass.
```

### Implementation Patterns & Key Details

```go
// THE skeleton render (binary `-\t-`, empty→"", real-tab header, trailing blank line):
func renderNumstatSkeleton(rows []numstatRow) string {
	if len(rows) == 0 { return "" }
	var b strings.Builder
	b.WriteString("Change summary (numstat: added\tdeleted\tpath):\n")
	for _, r := range rows {
		if r.IsBinary { b.WriteString("-\t-\t" + r.Path + "\n") } else { fmt.Fprintf(&b, "%d\t%d\t%s\n", r.Added, r.Deleted, r.Path) }
	}
	b.WriteByte('\n')
	return b.String()
}

// THE per-function skeleton call (domain + "-M"; NOT buildDiffArgs — no double "diff"):
skeleton, serr := g.numstatSkeleton(ctx, "--cached", "-M")   // StagedDiff
// skeleton, serr := g.numstatSkeleton(ctx, treeA, treeB, "-M") // TreeDiff
// skeleton, serr := g.numstatSkeleton(ctx, "-M")               // WorkingTreeDiff
if serr != nil { return "", serr }
b.WriteString(skeleton)   // FIRST write

// THE reorder: placeholders come NEXT (before Part 1 markdown); Part 2 still uses binExcludes.
```

### Integration Points

```yaml
GO MODULE (go.mod/go.sum): change NONE. skeleton.go uses stdlib fmt+strings. `go mod tidy` is a no-op.

PACKAGE EDGES: NONE added. skeleton.go is package git; calls S1's numstatRows (same package). No new imports
      in git.go.

UPSTREAM (consume, do NOT edit):
  - S1: numstatRow + numstatRows (numstat.go). numstatSkeleton WRAPS numstatRows.
  - buildDiffArgs / detectBinaryFiles / fileStatuses / detectExcludedStatuses / binaryPlaceholderLine /
    excludedPlaceholderLine / stripIndexLines — all unchanged (just reordered in the builder).

DOWNSTREAM (NOT this task):
  - P1.M4.T2.S2 (FR3i water-fill): uses numstatRows (S1) for per-file sizing; truncates BODIES, never the
        skeleton — so the FR3g completeness floor holds under truncation (M4 verifies it).
  - prompt.BuildUserPayload / BuildPlannerUserPayload: consume the returned payload as the verbatim tail
        (unchanged — the skeleton is inside the git layer, before the string is handed to these builders).

FROZEN/LEAVE (do NOT edit):
  - internal/git/numstat.go + numstat_test.go (S1 — SHIPPED).
  - internal/git/binary.go (detectBinaryFiles/fileStatuses/binaryPlaceholderLine — unchanged).
  - buildDiffArgs, stripIndexLines, the Git interface, every non-git file.
  - PRD.md, go.mod, Makefile.

NO NEW DATABASE / ROUTES / CLI / CONFIG / DOCS.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/git/skeleton.go internal/git/skeleton_test.go internal/git/git.go
go vet ./internal/git/
head -8 internal/git/skeleton.go   # → package git; import ( "fmt" "strings" )
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
git diff --exit-code internal/git/numstat.go internal/git/numstat_test.go && echo "S1 numstat files UNCHANGED (expected)"
# Expected: go vet clean; skeleton.go imports only fmt+strings; go.mod/numstat.go byte-unchanged.
```

### Level 2: Unit + integration tests

```bash
go test ./internal/git/ -v -run 'TestRenderNumstatSkeleton|TestStagedDiff_Ordering|TestStagedDiff_SkeletonComplete|TestTreeDiff_Ordering|TestWorkingTreeDiff_Ordering'
go test ./internal/git/
# Expected PASS — verify:
#   TestRenderNumstatSkeleton/* ........ empty→"", normal, binary `-\t-`, sorted, header+blank shape
#   TestStagedDiff_OrderingInvariant_*. skeleton < [binary] < md < code (the contract's invariant)
#   TestStagedDiff_SkeletonComplete_*. every changed file in the skeleton under a tight body cap
#   (TreeDiff/WorkingTreeDiff mirrors) — same invariants for the other two domains
#   The existing Contains/count/sentinel tests — still PASS (content relocated, not removed).
# If OrderingInvariant fails on iBinary<iMd, the placeholder block wasn't moved before Part 1 (still a
# simple prepend — fix the reorder). If SkeletonComplete fails, a capped file is missing from the skeleton.
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...   # Expect clean (skeleton.go compiles into package git; the 3 functions reordered).
go test ./...    # Expect all PASS — the git suite (new + updated fixtures) + no regression elsewhere.
# Confirm ONLY the expected files changed:
git diff --name-only | grep -E 'internal/git/(skeleton|git|stagediff|treediff|workingtreediff)' && echo "(expected files)"
git diff --exit-code internal/git/numstat.go internal/git/numstat_test.go internal/git/binary.go && echo "S1/binary UNCHANGED (expected)"
git diff --exit-code go.mod go.sum PRD.md && echo "go.mod/PRD UNCHANGED (expected)"
```

### Level 4: Correctness reasoning (the FR3g contract)

```bash
# The skeleton + ordering's correctness rests on FR3g + system_context §7. Verify by reasoning + the tests:
#   1. Every payload with changes begins with the skeleton (header + one row per changed file + blank line);
#      an empty change set returns "" (FR5 empty-payload preserved). (TestRenderNumstatSkeleton + the empty path)
#   2. The section order is skeleton → [binary]/[excluded] placeholders → markdown bodies → non-markdown
#      bodies in ALL THREE functions. (TestStagedDiff/TreeDiff/WorkingTreeDiff_OrderingInvariant)
#   3. The skeleton lists EVERY changed file (incl. binary) even when a body is byte/line-capped (the FR3g
#      completeness floor, pre-truncation). (TestStagedDiff_SkeletonCompleteUnderBodyCap)
#   4. Binary rows render `-\t-` (mirror git numstat), not 0/0. (TestRenderNumstatSkeleton binary case)
#   5. The water-fill (M4) truncates bodies, never the skeleton ⇒ the floor holds under truncation (M4
#      verifies the post-truncation case; S2 delivers the floor).
# (No Level-4 commands beyond Levels 1–3 — the tests ARE the proof.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l` clean on the edited files.
- [ ] `go test ./...` GREEN (new tests + updated fixtures + no regression).
- [ ] go.mod/go.sum byte-unchanged; skeleton.go imports ONLY fmt/strings.
- [ ] S1's numstat.go/numstat_test.go byte-unchanged; binary.go/buildDiffArgs/the Git interface unchanged.

### Feature Validation
- [ ] `renderNumstatSkeleton`: empty→""; header (real tabs) + rows (binary `-\t-`) + trailing blank line.
- [ ] StagedDiff/TreeDiff/WorkingTreeDiff prepend the skeleton FIRST; placeholders come BEFORE markdown; then
      markdown; then non-markdown (identical order in all three).
- [ ] Each function calls `numstatSkeleton(ctx, <domain…>, "-M")` (no double "diff"; no -U).
- [ ] Ordering-invariance test passes for all 3; completeness test passes (skeleton lists every file under a cap).

### Code Quality Validation
- [ ] NEW skeleton.go (parallel-clean; doesn't touch numstat.go); the 3-function edit is a mechanical reorder
      + prepend (same shape, only domain args differ).
- [ ] Anti-patterns avoided (see below); existing Contains/count tests not churned unnecessarily.

### Documentation
- [ ] Doc comments on renderNumstatSkeleton/numstatSkeleton cite PRD §9.1 FR3g, the dual-use role, the
      binary `-\t-` rationale, the empty→""/FR5 rationale, and the `<domain>+"-M"` call shape. No docs/*.md
      edits (the user-facing description belongs in the M5 how-it-works sweep).

---

## Anti-Patterns to Avoid

- ❌ **Don't implement a simple prepend (leave markdown before placeholders).** system_context §7's
      `"skeleton + placeholders + md + non-md"` is sequential — placeholders come before markdown. MOVE the
      placeholder block ahead of Part 1 markdown. A simple prepend yields the wrong order. (research §1)
- ❌ **Don't edit S1's numstat.go.** S2's render+capture helpers go in a NEW skeleton.go. numstat.go is S1's
      (SHIPPED). numstatSkeleton CALLS numstatRows. (research §2)
- ❌ **Don't render binary rows as `0\t0`.** numstatRow stores 0/0 for binary, but the skeleton MIRRORS git
      numstat's `-\t-\t<path>`. Emit literal hyphens for IsBinary rows. (research §5)
- ❌ **Don't emit a header-only skeleton for an empty diff.** Return "" for 0 rows — the caller gates on
      `diff == ""` (FR5). A header-only skeleton defeats that check. (research §4)
- ❌ **Don't pass buildDiffArgs output to numstatRows/numstatSkeleton.** buildDiffArgs returns `["diff", …]`
      and numstatRows prepends its own `"diff"` ⇒ double `"diff"`. Pass `<domain…> + "-M"` (the post-"diff"
      tokens). Omit `-U<n>` (irrelevant for numstat). (research §6)
- ❌ **Don't forget `-M` on the numstat call.** The diff bodies always use -M (buildDiffArgs); without -M the
      skeleton could show a rename as delete+add (two rows) on older git — inconsistent with the bodies.
      resolveNumstatPath handles `=>` either way, but -M makes it deterministic. (research §6)
- ❌ **Don't break the FR5 empty-payload check.** A repo with no changes must still return "" — the skeleton's
      empty→"" rule + empty Part 1 + no placeholders + empty Part 2 ⇒ "". (research §4)
- ❌ **Don't churn the passing Contains/count tests.** The skeleton adds content but removes nothing; most
      existing assertions survive unchanged. Update ONLY HasPrefix/exact-equality/order assertions. (research §8)
- ❌ **Don't verify truncation resilience here.** S2 delivers the completeness floor (pre-truncation); the
      "holds under water-fill truncation" verification is M4.T2's job. (research §9)
- ❌ **Don't change the three functions inconsistently.** The ordering invariant requires ALL THREE to emit
      skeleton → placeholders → markdown → non-markdown. Apply the same structural edit; only domain args
      differ. (research §1/§7)

---

## Confidence Score

**9/10** — the central ambiguity (prepend-only vs. reorder) is RESOLVED by system_context.md §7's explicit
`"skeleton + placeholders + md + non-md"` string (sequential — placeholders before markdown), and the reorder
is dependency-safe (the placeholder block's binExcludes is consumed by Part 2; Part 1 markdown doesn't use
it). The render rules are pinned by the contract verbatim (header + `<added>\t<deleted>\t<path>` rows;
binary `-\t-\t<path>`; trailing blank line; empty→""). S1's numstatRow/numstatRows API is confirmed on disk
(rows pre-sorted, binary-flagged, numstatRows builds its own "diff" argv — hence the `<domain>+"-M"` call, not
buildDiffArgs). The blast radius is small: most existing tests use `strings.Contains` (survive the relocate),
so golden-fixture churn is minimal. The copy-ready skeleton.go + the canonical StagedDiff edit + the
domain-arg table make the 3-function change mechanical. The one residual risk — a `HasPrefix`/exact-equality
test the audit didn't surface — is caught by Task 6's "run the suite, fix only what breaks" step. The -1
reserves for an ordering-invariance assertion needing a tweak if a test repo's markdown/code `diff --git`
marker doesn't appear exactly as indexed (the test asserts presence + order, robustly).
