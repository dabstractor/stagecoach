# P1.M3.T1.S2 — Design Decisions (skeleton prepend + ordering invariance)

Ground truth read before writing this note:
- **PRD §9.1 FR3g** (h3.17 — the skeleton: `--numstat`, one `added/deleted/path` line per file, prepended
  before any diff body; a fully-truncated file remains represented → the completeness floor).
- **The work-item contract** (S2): build the skeleton from S1's numstatRows; PREPEND before placeholders +
  bodies; "Ordering invariant: skeleton first, then placeholders, then markdown bodies, then non-markdown
  bodies — identical order in all 3 functions"; update golden fixtures.
- **plan/007_…/architecture/system_context.md §7** (data flow) — AUTHORITATIVE for the target payload order:
  `returns payload string (skeleton + placeholders + md + shaped non-md)`. This RESOLVES the central ambiguity
  (see §1): placeholders come BEFORE markdown bodies.
- **plan/007_…/architecture/diff_capture_touchmap.md** — the 3 sibling functions are structurally identical
  (Part 1 markdown → binary placeholders → excluded placeholders → Part 2 non-markdown); they stream into ONE
  `strings.Builder` in computation order.
- **internal/git/numstat.go** (S1 — SHIPPED, on disk): `numstatRow{Added, Deleted int; IsBinary bool; Path string}`,
  `resolveNumstatPath`, `(g *gitRunner) numstatRows(ctx, diffArgs...)`. numstatRows builds its OWN `["diff",…]`
  argv with `--numstat` placed before any `--`. Binary rows have `IsBinary=true, Added=Deleted=0`.
- **internal/git/git.go** — read all 3 diff functions (StagedDiff L732, TreeDiff L1187, WorkingTreeDiff L1324):
  each is a near-verbatim copy; the ONLY difference is the diff-domain positionals (`--cached` / `treeA,treeB` /
  none). Each streams into one builder: Part 1 markdown → binary placeholders → excluded placeholders →
  Part 2 non-markdown. `buildDiffArgs(opts, domain…)` returns `["diff", domain…, "-M", "-U<n>"]` (always-on
  -M; -U from opts.DiffContext). `binaryPlaceholderLine`/`excludedPlaceholderLine` emit the one-line markers.

Verified at research time: `go build ./... && go test ./internal/git/` GREEN (S1 numstat tests pass).

---

## §1 — THE ORDERING QUESTION IS RESOLVED: placeholders move BEFORE markdown (a REORDER, not just a prepend)

**The ambiguity:** the contract says "PREPEND the skeleton … before the placeholders and the diff bodies"
(satisfied by a simple prepend) BUT the "Ordering invariant" lists `skeleton → placeholders → markdown →
non-markdown` — which contradicts the CURRENT order (`markdown → placeholders → non-markdown`). Two readings:
(A) reorder placeholders ahead of markdown; (B) simple prepend only (`skeleton → markdown → placeholders →
non-markdown`).

**Resolution: Reading A (reorder).** system_context.md §7 — the authoritative data-flow diagram — states the
returned payload is `"skeleton + placeholders + md + shaped non-md"`. That string is sequential and explicit:
placeholders precede markdown. The work-item contract's invariant matches it. ⇒ S2 must BOTH prepend the
skeleton AND move the binary/excluded placeholder block to BEFORE Part 1 markdown.

**Why this is feasible (dependency check):** the placeholder block computes `binExcludes` (used by Part 2),
and Part 1 markdown does NOT use `binExcludes` (it diffs each `.md` file individually; binary files are not
`.md`). So moving the placeholder block (detectBinaryFiles/fileStatuses/detectExcludedStatuses) to before
Part 1 is safe — no dependency is violated. Final computation/output order per function:
1. skeleton (numstatRows — independent)
2. binary + excluded placeholders (→ also produces binExcludes)
3. Part 1 markdown (independent of binExcludes)
4. Part 2 non-markdown (uses binExcludes)

---

## §2 — New file `internal/git/skeleton.go` (parallel-clean; no edit to S1's numstat.go)

**Decision:** put S2's render+capture helpers in a NEW `skeleton.go`, NOT in S1's `numstat.go`. Two reasons:
(a) parallel-safety — S1 is being implemented concurrently; an additive function in numstat.go is technically
safe but a dedicated file is the clean, zero-conflict boundary; (b) cohesion — the skeleton RENDER is an S2
concern (FR3g delivery), distinct from S1's numstat PARSE.

Contents (2 artifacts):
- `func renderNumstatSkeleton(rows []numstatRow) string` — PURE: the header + one line per row + blank
  separator. Returns `""` for empty rows (§4).
- `func (g *gitRunner) numstatSkeleton(ctx, diffArgs...) (string, error)` — capture (numstatRows) + render;
  the single call each of the 3 diff functions makes.

---

## §3 — The skeleton block format (header + rows + blank separator)

**Decision** (per the contract's suggested header, verbatim):
```
Change summary (numstat: added	deleted	path):
<added>	<deleted>	<path>
… (one line per changed file) …
<blank line>
```
- **Header:** `Change summary (numstat: added\tdeleted\tpath):\n` — REAL tabs (the `\t` in the contract is a
  tab, mirroring the row column layout so the header is a literal template for the rows; the model reads the
  columns at a glance).
- **Rows:** `fmt.Fprintf("<%d>\t<%d>\t<%s>\n", Added, Deleted, Path)`. Rows come from numstatRows which is
  already sorted by Path (S1) ⇒ deterministic.
- **Binary rows:** `-\t-\t<Path>\n` (LITERAL hyphens, matching git's numstat output for binary). numstatRow
  has `IsBinary=true, Added=Deleted=0`; the RENDER must emit `-`/`-` (NOT `0`/`0`) so the skeleton mirrors
  what `git diff --numstat` literally prints. (§5.)
- **Trailing blank line:** one `\n` after the last row → a blank-line separator before the placeholders/bodies.
  This visually delimits the skeleton from the diff bodies.

---

## §4 — Empty diff ⇒ NO skeleton (preserve the FR5 empty-payload check)

**Decision:** `renderNumstatSkeleton` returns `""` when `len(rows) == 0`. Rationale: the orchestrator gates
on an empty diff (FR5: "if the combined diff is empty, follow the nothing-staged path"; callers check
`diff == ""`). Emitting a header-only skeleton (`"Change summary…:\n\n"`) for 0 changed files would make the
payload non-empty and break that check. An empty change set has nothing to summarize ⇒ no skeleton. (This
also keeps TreeDiff/WorkingTreeDiff returning `""` for an empty concept diff / clean tree.)

---

## §5 — Binary rows render as `-\t-`, NOT `0\t0` (mirror git's numstat)

**Decision:** in `renderNumstatSkeleton`, `if r.IsBinary { write("-\t-\t" + Path) }` BEFORE the numeric
branch. S1's numstatRow sets `Added=Deleted=0` for binary (the `"-"` counts aren't integers), but the
skeleton is a human/model-facing MIRROR of `git diff --numstat`, which prints `-\t-\t<path>` for binary.
Emitting `0\t0` would misrepresent the file as a zero-line text change. The contract is explicit: "binary
rows: `-\t-\t<path>`". (The IsBinary flag is the render switch; the 0 counts are the parse artifact.)

---

## §6 — numstat call uses `<domain> + "-M"` (rename consistency with the diff bodies)

**Decision:** each diff function calls `g.numstatSkeleton(ctx, <domain…>, "-M")`:
- StagedDiff: `numstatSkeleton(ctx, "--cached", "-M")`
- TreeDiff: `numstatSkeleton(ctx, treeA, treeB, "-M")`
- WorkingTreeDiff: `numstatSkeleton(ctx, "-M")`

**Why `-M`:** the diff bodies ALWAYS use `-M` (buildDiffArgs emits it unconditionally — FR3e). For the
skeleton to reflect the SAME file view (a rename = ONE row, not a delete+add pair), the numstat call must
also engage rename detection. (S1 established git ≥2.9 detects renames without -M too, but -M makes it
deterministic across versions/configs and matches the bodies. resolveNumstatPath then resolves `=>`/`{…}`.)
**Why NOT `-U<n>`:** numstat is line COUNTS, not patch context — `-U` is irrelevant and omitted. (numstatRows
places `--numstat` before any `--`, so `["diff", domain…, "-M", "--numstat"]` is correct.)

**Why not pass `buildDiffArgs(opts,…)` output:** buildDiffArgs returns `["diff", …]` (WITH the leading
`"diff"`), but numstatRows prepends its OWN `"diff"` ⇒ double `"diff"`. So pass the post-`diff` tokens
(domain + `-M`) directly. (3 call sites; the `-M` literal is stable and mirrors buildDiffArgs' always-on -M.)

---

## §7 — The 3-function edit (identical shape; only domain args differ)

**Decision:** edit StagedDiff / TreeDiff / WorkingTreeDiff in `internal/git/git.go` with the SAME structural
change (the domain positionals are the only difference). For EACH:
1. AFTER the cap-defaults block, BEFORE Part 1: `skeleton, serr := g.numstatSkeleton(ctx, <domain…>, "-M")`;
   `if serr != nil { return "", serr }`.
2. MOVE the binary-filtering + binary-placeholder block AND the user-exclude-placeholder block to BEFORE
   Part 1 markdown. (They produce `binExcludes`, which Part 2 still consumes after the reorder — fine.)
3. The builder `b` now streams in the target order: `b.WriteString(skeleton)` → binary placeholders →
   excluded placeholders → Part 1 markdown → Part 2 non-markdown.

**GOTCHA (skeleton write placement):** write `skeleton` to `b` FIRST (it is already a complete string incl.
its trailing blank line). Then the placeholder block writes its markers. Then Part 1, then Part 2. Because
`skeleton` is built independently (not streamed), it composes cleanly at the front.

**GOTCHA (binExcludes lifecycle):** `binExcludes` is declared in the (now earlier) placeholder block and
consumed in Part 2 — unchanged semantics, just earlier declaration. Keep it a SEPARATE slice (never append
to `excludes`, which may alias `defaultExcludes`).

---

## §8 — Golden fixture updates (skeleton prepend + placeholder/markdown reorder)

**Decision:** the reorder + skeleton prepend change the output of ALL three functions ⇒ every test that
asserts on output ORDER or PREFIX breaks; `strings.Contains`/count tests mostly survive (content still
present, just relocated). Update per file:
- `stagediff_test.go` / `treediff_test.go` / `workingtreediff_test.go`: re-run; for each failing assertion,
  update the expectation to include the leading skeleton block AND the new placeholder-before-markdown order.
  Sentinel tests (`... [diff truncated at N bytes/lines]`) and `diff --git` count/Contains tests stay valid.
- The binary/excluded placeholder tests (e.g. `TestStagedDiff_BinaryFilePlaceholderAndExcluded` L295) now
  find the placeholder AFTER the skeleton but BEFORE the markdown body — update any order/prefix assertion.

**NEW tests (the contract's "Ordering invariant" + completeness):**
- **Ordering-invariance test** (one per function, or a shared table): set up a repo with a markdown file, a
  binary, and a code file; run the diff; assert `indexOf(skeleton header) < indexOf([binary] placeholder) <
  indexOf(markdown "diff --git a/<md>") < indexOf(code "diff --git a/<code>")`. This is THE contract test.
- **Completeness test:** a repo with N changed files (incl. binary) → the skeleton lists ALL N paths (every
  changed file represented) even though the bodies may later be capped. (The truncation-resilience
  verification itself is M4's job; S2 asserts the skeleton is complete PRE-truncation.)
- **Pure `renderNumstatSkeleton` table test:** empty→"", normal row→"N\tM\tPath", binary row→"-\t-\tPath",
  sorted-order passthrough, header + trailing-blank-line shape.

---

## §9 — Completeness floor under truncation is M4's verification; S2 delivers the floor

**Decision:** S2 ensures the skeleton lists EVERY changed file NOW (the FR3g completeness floor). The
contract's "Confirm the skeleton reflects EVERY changed file even when a body is later truncated (M4)" is a
forward pointer: the water-fill truncation (P1.M4.T2) truncates BODIES, never the skeleton, so a fully-
truncated file remains in the skeleton. S2's completeness test (§8) proves the floor pre-truncation; M4
proves it holds post-truncation. S2 does NOT implement truncation.

---

## §10 — No new deps; go.mod UNCHANGED

**Decision:** skeleton.go uses only `fmt`+`strings` (stdlib). git.go already imports everything needed. No
new import, no go.mod change. `go mod tidy` is a no-op.

---

## Summary table (the 10 calls at a glance)

| § | Decision | Source |
|---|----------|--------|
| 1 | REORDER: placeholders before markdown (Reading A), per system_context §7 "skeleton+placeholders+md+non-md" | contract + system_context §7 |
| 2 | New `skeleton.go` (renderNumstatSkeleton + numstatSkeleton); don't edit S1's numstat.go | parallel-clean |
| 3 | Skeleton = header (real tabs) + rows + trailing blank line | contract |
| 4 | Empty rows → "" (preserve FR5 empty-payload check) | FR5 |
| 5 | Binary rows render `-\t-\t<path>` (NOT 0/0); mirror git numstat | contract |
| 6 | numstat call uses `<domain> + "-M"` (rename consistency); no -U (irrelevant); no buildDiffArgs (double "diff") | buildDiffArgs/numstatRows shapes |
| 7 | 3-function edit: skeleton first, move placeholder block before Part 1, then Part 1, then Part 2 | §1 dependency check |
| 8 | Update golden fixtures (skeleton + reorder); add ordering-invariance + completeness + pure-render tests | contract |
| 9 | Completeness floor delivered now; truncation-resilience verification is M4 | contract (M4 pointer) |
| 10 | No new imports; go.mod UNCHANGED | stdlib only |
