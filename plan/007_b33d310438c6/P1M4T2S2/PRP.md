---
name: "P1.M4.T2.S2 — Per-file truncation application + sentinel + header preservation (PRD §9.1 FR3i; architecture/git_diff_semantics.md §6; system_context.md §6 invariant 2)"
description: |

  Implement the FR3i water-fill TRUNCATION APPLICATION as three PURE, git-independent functions in
  internal/git: `splitDiffSections(diff string) []string` (split the captured non-markdown aggregate on
  `diff --git ` boundaries — the item_description's "WITHOUT extra git invocations"), `diffSectionPath(
  section string) (path string, ok bool)` (extract the destination b/ path from a section's
  `diff --git a/<p> b/<p>` line, fallback `+++ b/<p>`), and `truncateByWaterFill(sections []string,
  allotments map[string]int) string` (the item's named OUTPUT function).

  CONTRACT (item_description §3 + §4, verbatim): the three functions are PURE (no git, no I/O, no
  context; stdlib `strings` + `unicode/utf8` + `regexp` only). `truncateByWaterFill` takes the per-file
  sections + allotments (a `map[string]int` keyed by numstat destination path, in TOKENS) and, for each
  section whose BODY exceeds its allotment, truncates the BODY (the lines from the first `@@` onward —
  everything after the `diff --git`/extended-header/`---`/`+++` header block) to its token allotment
  (first allotment tokens = first allotment×4 runes), PRESERVING the header block, and appends the
  SHORTER `\n... [truncated]` sentinel (system_context.md §6 invariant 2 — NOT the legacy
  `... [diff truncated at N bytes/lines]` form). Files within their allotment pass through UNTOUCHED
  (byte-identical — no sentinel). Sections are recomposed in ORIGINAL ORDER. Allotments are mapped to
  sections by PATH (the section's destination, resolved identically to numstat's `=>` destination —
  "S1 path keying must agree here"); a section whose path is absent from `allotments` passes through
  untouched (safe over-include, never a wrong truncation). The same `truncateByWaterFill` serves the
  markdown per-file sections (already per-file) uniformly.

  RETURN CONTRACT (FR3i + item_description §3):
    - splitDiffSections: split on `diff --git ` (trailing space); re-prefix each section with `diff --git `
      so it is self-contained; drop a truly-empty leading element; empty input → [].
    - diffSectionPath: `^diff --git a/(.*) b/(.*)$` → group 2 (destination); fallback `^+++ b/(.*)$` →
      group 1; strip one surrounding pair of `"` (basic quote mitigation); ok=false if neither matches.
    - truncateByWaterFill: for each section: path → allotment; if path-miss OR allotment≥EstimateTokens(body)
      → verbatim; else body = firstNRunes(body, allotment×4) + "\n... [truncated]"; result = header block +
      (possibly truncated) body; join sections in input order. Empty sections → "".

  ITEM TESTS (item_description §3, verbatim — MUST appear as table tests with HARDCODED expectations):
    - a 3-file diff where one exceeds L is truncated and the others are whole;
    - headers preserved on the truncated file (diff --git / --- / +++ / first @@ all present);
    - sentinel present (the `... [truncated]` form, on its own line);
    - non-truncated files byte-identical to input.

  EDGE CASES (all table-tested, all pure): empty sections; all-files-within-budget (output byte-identical,
    no sentinels); multi-hunk file truncated mid-body; markdown per-file section (same code path); path-miss
    pass-through; pure-rename section (no `@@` → no body → verbatim); path extraction for new-file /
    deletion / rename / `+++`-fallback; quote-strip.

  INPUT (upstream — READ-ONLY contracts, do NOT modify): `allocByWaterFill`/`waterFillLevel`
    (P1.M4.T2.S1, parallel — the SOLVER; S2 does NOT call it — the GATE does, then passes the resulting
    allotments map into S2). `EstimateTokens` (P1.M4.T1.S1 — ceil(runes/4); S2 uses it for the
    "needs truncation?" check AND for the allotment×4 rune budget, keeping the unit consistent).
    `numstatRow.Path` keying (P1.M3.T1.S1 — the destination path via `resolveNumstatPath`; S2's
    `diffSectionPath` yields the SAME destination string so map keys agree). The captured non-markdown
    aggregate (already -M/-U<n>-shaped and FR3h-index-stripped by M2; the `index <oid>..<oid> <mode>` line
    is ALREADY GONE before S2 sees it — do NOT re-strip).

  OUTPUT (downstream — the frozen consumer contract): M4.T3 (P1.M4.T3.S1 — the token-limit gate) calls
    `sections := splitDiffSections(nmDiff)`, builds `allotments` (path→token allotment from
    `allocByWaterFill` over `EstimateTokens(body)` sizes + the numstatRows path keying), and substitutes
    `truncateByWaterFill(sections, allotments)` for the legacy byte-cap when `token_limit > 0`. The same
    pair serves the markdown per-file sections. SIGNATURES ARE FROZEN — do not change them.

  DELIVERABLES (2 NEW files; nothing else touched):
    NEW internal/git/truncatediff.go      — `package git`. splitDiffSections + diffSectionPath +
      firstNRunes + truncateByWaterFill (pure; stdlib strings + unicode/utf8 + regexp only; no context/
      I/O/git). Doc comments cite FR3i, §6, the sentinel-form invariant (system_context §6 inv 2), the
      header-block/body split, the path-keyed-allotment robustness, and the frozen-signature consumer
      contract.
    NEW internal/git/truncatediff_test.go — `package git` (white-box). Exhaustive pure table tests (all
      item tests + all edge cases) with HARDCODED expectations, mirroring numstat_test.go's table-driven
      style (TestResolveNumstatPath shape). NO t.TempDir, NO git repo, NO I/O.

  SCOPE BOUNDARY (owned by siblings — do NOT implement): the token-limit gate in the 3 diff functions +
    body_budget = token_limit − skeleton − reserve − margin (M4.T3); building the allotments map from
    allocByWaterFill + numstatRows (M4.T3); the SOLVER itself (S1 — frozen, do NOT call); the legacy
    byte/line caps + their sentinels (M4.T3 keeps them byte-identical at token_limit==0); git.go /
    waterfill.go / numstat.go / skeleton.go / tokens.go / binary.go / StagedDiffOptions — UNCHANGED.

  Deliverable: 2 NEW pure functions-group + exhaustive tests. `go build/vet/gofmt` clean; `go test ./...`
  green (pure additions — no behavior change, no consumer reads them yet); only the 2 new files differ.

---

## Goal

**Feature Goal**: Implement the FR3i water-fill TRUNCATION APPLICATION (PRD §9.1 FR3i; git_diff_semantics.md
§6; system_context.md §6 invariant 2) as three pure, git-independent functions — `splitDiffSections` (the
`diff --git ` boundary splitter), `diffSectionPath` (the destination-path extractor), and
`truncateByWaterFill` (the per-file body truncation + shorter `... [truncated]` sentinel + header
preservation) — fully tested (exhaustive pure table tests), ready for M4.T3 (the token-limit gate) to
consume on both the non-markdown aggregate and the markdown per-file sections.

**Deliverable** (2 NEW files; nothing else touched):
1. `internal/git/truncatediff.go` — `package git`. `func splitDiffSections(diff string) []string` + `func
   diffSectionPath(section string) (path string, ok bool)` + `func firstNRunes(s string, n int) string`
   (unexported helper) + `func truncateByWaterFill(sections []string, allotments map[string]int) string`.
   PURE (stdlib `strings` + `unicode/utf8` + `regexp` only; no context/I/O/git). Doc comments cite FR3i +
   §6 + the sentinel-form invariant + the header-block/body split + path-keyed-allotment robustness + the
   frozen consumer contract.
2. `internal/git/truncatediff_test.go` — `package git` (white-box — same package, like numstat_test.go).
   Exhaustive pure table tests (all item tests + all edge cases) with HARDCODED expectations.

**Success Definition**: the item's named test passes — a 3-file diff where one file exceeds its allotment L
is truncated (body cut to L tokens + `\n... [truncated]` sentinel on its own line) and the other two are
BYTE-IDENTICAL to input (no sentinel); the truncated file's `diff --git`/`---`/`+++`/first-`@@` headers are
all preserved. Plus: multi-hunk truncation; markdown per-file section; all-files-within-budget
(byte-identical, no sentinels); path-miss pass-through; pure-rename pass-through; empty sections. `go build
./... && go vet ./... && go test ./...` green; `gofmt -l` clean; only the 2 new files differ.

## User Persona

**Target User**: The downstream gate M4.T3 (the token-limit gate in the 3 diff functions), which calls
`splitDiffSections` + `truncateByWaterFill`. Transitively: every user who sets `token_limit` (PRD §9.1
FR3d) so a large diff fits their model's context window — the truncation application is what actually cuts
each oversized file's body to its water-fill allotment and marks it with the sentinel.

**Use Case**: A user sets `token_limit = 120000`. The gate (M4.T3) computes body_budget, builds per-file
token sizes, calls `allocByWaterFill` (S1) → allotments, then calls S2's `truncateByWaterFill(sections,
allotments)`. Each file larger than the water level L is cut to its first L tokens + `... [truncated]`;
small files pass through whole. The model sees every file (small ones intact, large ones fairly capped at L
with a visible sentinel) — no file monopolizes, no file is silently dropped (the FR3g skeleton is the
completeness floor; the sentinel is the honesty floor).

**User Journey**: (internal) M4.T3 captures nmDiff → `sections = splitDiffSections(nmDiff)` → builds
`allotments` (path→token allotment) → `out = truncateByWaterFill(sections, allotments)` → substitutes `out`
for the legacy byte-cap result. For markdown, `sections` = the list of per-file markdown diffs (same call).

**Pain Points Addressed**: The legacy aggregate byte-cap (git.go L866: `nmDiff[:maxDiffBytes] + "... [diff
truncated at N bytes]"`) cuts the WHOLE aggregate at an arbitrary byte offset — it can bisect a file
mid-token, drop later files entirely, and leave no per-file honesty marker. The water-fill + S2 fixes all
three: truncation is PER-FILE (fair), the header block of every file survives, and each truncation carries
its own `... [truncated]` sentinel so the model knows WHICH file is partial.

## Why

- **It IS the FR3i truncation application.** PRD §9.1 FR3i: "every file larger than L is truncated to
  exactly L (its first L tokens + the `... [truncated]` sentinel)"; "each file's `diff --git`/hunk headers
  are always preserved alongside its (possibly truncated) body"; "the aggregate non-markdown diff is split
  on `diff --git` boundaries to apply the per-file level without extra git invocations." This task implements
  exactly that.
- **The honesty + fairness guarantees live here.** Per-file sentinels (the model knows which file is
  partial — not a single aggregate mystery cut); header preservation (file identity + hunk anchors survive
  every truncation); pass-through byte-identical (files within budget are NOT mangled — no spurious
  sentinel, no off-by-one).
- **Pure + independent of git ⇒ exhaustively testable.** Like S1 and `resolveNumstatPath`, these functions
  are string arithmetic over already-captured text — no git repo, no exec, no context. Every edge case is a
  string literal assertable in microseconds. The header/body split, the sentinel form, the path extraction,
  the byte-identical pass-through are all pinned by HARDCODED table expectations before the gate wires them
  in.
- **Reuses S1's allotments + the single estimator's UNIT.** The allotments (path→tokens) come from S1's
  `allocByWaterFill`; the "needs truncation?" check + the allotment×4 rune budget both use
  `EstimateTokens`'s ceil(runes/4) so the unit is coherent end-to-end (the water-fill's "file > L" condition
  and S2's truncation decision agree when the gate sizes bodies via `EstimateTokens`).

## What

Three pure functions (+ one unexported helper) in `internal/git/truncatediff.go`:

`splitDiffSections(diff string) []string`:
- Split `diff` on the literal `diff --git ` (trailing space). `strings.Split` yields a leading element
  (text before the first `diff --git`) + one element per section (each WITHOUT its `diff --git ` prefix,
  which was the separator).
- RE-PREFIX each non-empty section with `diff --git ` so it is self-contained (first line =
  `diff --git a/<p> b/<p>`). Drop a truly-empty/whitespace-only leading element; KEEP a non-empty leading
  element as its own leading section (defensive — should not occur for a clean non-md aggregate).
- Empty/whitespace input → `[]`.

`diffSectionPath(section string) (path string, ok bool)`:
- Try `diffSectionPathRe` = `(?m)^diff --git a/(.*) b/(.*)$` on the section → return group 2 (destination).
- Fallback `plusPlusRe` = `(?m)^\+\+\+ b/(.*)$` → return group 1. (A deletion's `+++ /dev/null` does not
  match `b/` — but its `diff --git a/x b/x` already supplied the path.)
- Strip ONE surrounding pair of `"` from the result (basic git-quote mitigation for paths with spaces — see
  Known Gotchas).
- `ok = false` if neither regex matches (e.g. a binary placeholder line that leaked in) → caller passes
  the section through untouched.

`firstNRunes(s string, n int) string` (unexported helper):
- Return `s`'s first `n` runes, rune-boundary-safe, with NO full `[]rune` allocation (iterate byte offsets
  via `for i := range s`, stop at the Nth rune's start). `n <= 0` → `""`. Fewer than `n` runes → `s` whole.

`truncateByWaterFill(sections []string, allotments map[string]int) string`:
- For each `section` in `sections` (in input order):
  1. `path, ok := diffSectionPath(section)`. If `!ok` OR `path` absent from `allotments` → append `section`
     verbatim; continue (path-miss pass-through — safe over-include).
  2. `allotment := allotments[path]` (tokens). If `allotment <= 0` → treat as path-miss (append verbatim;
     a zero/negative allotment is degenerate — the gate guards budget>0, but S2 is defensive).
  3. Split the section into `headerBlock` + `body` at the FIRST `@@` line: `headerBlock` = all lines before
     the first line starting with `@@` (exclusive); `body` = that `@@` line + everything after. If NO `@@`
     line → `headerBlock` = the whole section, `body` = "" (pure rename / mode-only → no truncation; append
     verbatim).
  4. `if EstimateTokens(body) > allotment`: `body = firstNRunes(body, allotment*4) + "\n... [truncated]"`.
     (allotment×4 runes ⟺ allotment tokens via ceil(runes/4); the sentinel goes on its own line, matching
     the legacy `\n... [diff truncated at N bytes]` line shape.) Else: `body` unchanged (within budget → no
     sentinel).
  5. Append `headerBlock + body` (preserving the newline boundary between them).
- Join the per-section results in input order and return. Empty `sections` → `""`.

(All four are pure; no git/I/O/context. `EstimateTokens` is in-package — no import statement needed.)

### Success Criteria

- [ ] `internal/git/truncatediff.go` exists, `package git`, imports ONLY stdlib (`strings`, `unicode/utf8`,
      `regexp`) — no context/I/O/git.
- [ ] `splitDiffSections(diff)` splits on `diff --git ` (trailing space), re-prefixes each section with
      `diff --git `, drops a truly-empty leading element, returns `[]` for empty input.
- [ ] `diffSectionPath(section)` returns the destination (group 2 of `^diff --git a/(.*) b/(.*)$`, fallback
      `^+++ b/(.*)$`), strips one `"` pair, returns `ok=false` when neither matches.
- [ ] `truncateByWaterFill(sections, allotments)` splits each section at the first `@@` into header block +
      body; truncates the body to `allotment×4` runes + `\n... [truncated]` when `EstimateTokens(body) >
      allotment`; passes through verbatim otherwise; recomposes in input order.
- [ ] The item's named test passes with HARDCODED expectations: a 3-file diff where one exceeds its
      allotment is truncated (sentinel on its own line); the other two are byte-identical to input (the
      sentinel appears EXACTLY once — only on the truncated file).
- [ ] Headers preserved on the truncated file: `diff --git a/B b/B`, `--- a/B`, `+++ b/B`, and the first
      `@@` are all present after truncation.
- [ ] All edge cases pass: all-files-within-budget (byte-identical output, NO sentinels); multi-hunk file
      truncated mid-body; markdown per-file section (same code path); path-miss pass-through; pure-rename
      section (no `@@` → verbatim); empty sections → "".
- [ ] Doc comments cite FR3i, §6, the sentinel-form invariant (system_context §6 inv 2), the header/body
      split, the path-keyed-allotment robustness, and the frozen consumer contract (M4.T3).
- [ ] `go build ./... && go vet ./... && go test ./...` GREEN; `gofmt -l internal/git/` clean; ONLY the 2
      new files differ (`git status`); git.go/waterfill.go/numstat.go/skeleton.go/tokens.go/binary.go/
      StagedDiffOptions UNCHANGED.

## All Needed Context

### Context Completeness Check

_Pass._ A Go developer with no prior repo knowledge can implement this from: the exact FR3i contract
(per-file body truncation to L tokens + the shorter `... [truncated]` sentinel + header preservation +
`diff --git ` splitting — quoted verbatim in the PRD selection), the precise section format (verified
against stagediff_test.go L605–622, quoted below), the header-block/body split rule (D3), the
token→rune conversion (D5), the path-extraction regexes (D6), the copy-ready skeletons in the
Implementation Blueprint, and the test pattern to mirror (numstat_test.go's `TestResolveNumstatPath`,
quoted). No git plumbing / numstat / prompt knowledge required — the functions are pure string
arithmetic over already-captured, already-FR3h-index-stripped diff text.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE feature contract (FR3i) + the sentinel-form invariant
- docfile: plan/007_b33d310438c6/architecture/system_context.md
  section: "## 6. Regression invariants (acceptance criteria)" — invariant 2: "`token_limit > 0` ⇒
       water-fill replaces the byte/line caps. The `... [truncated]` sentinel (shorter form, per PRD
       FR3i) is emitted per truncated file; the `at N bytes` sentinels do NOT appear."
  why: PINNED the EXACT sentinel string `... [truncated]` (shorter form) and that it is PER-FILE on the
       token_limit>0 path — DISTINCT from the legacy `... [diff truncated at N bytes/lines]` aggregate
       sentinels (git.go L840/L868) which S2 must NEVER emit. Also invariant 1 (FR3f/-U1 always-on —
       orthogonal here) and invariant 4 (payload-only, never commit-affecting — same as [binary]/
       [excluded]).
  critical: the sentinel is the SHORTER `... [truncated]` form — NOT `... [diff truncated at N tokens]`
       or any N-bearing variant. Append on its OWN line (leading `\n`). Append ONLY when content was
       actually removed (within-budget sections are byte-identical, NO sentinel).

# MUST READ — the §6 water-fill spec (the truncation algorithm S2 applies the RESULT of)
- docfile: plan/007_b33d310438c6/architecture/git_diff_semantics.md
  section: "## 6. Water-fill / water-filling truncation" (Algorithm; §4's keep/strip table for which lines
       are headers).
  why: §6 defines "every file larger than L is truncated to exactly L (its first L tokens)" — S2 implements
       the "first L tokens + sentinel" application. §4's table pins which lines are HEADERS (diff --git,
       ---, +++, @@ — KEEP) vs the index line (STRIP — already gone by FR3h before S2 sees the text). §5
       pins the chars/4 estimator whose inverse (allotment×4 runes) S2 uses for the body cutoff.
  critical: "first L tokens" = first allotment×4 RUNES of the BODY (rune-boundary-safe, NOT bytes). The
       header block (diff --git + extended + ---  + +++) is NOT counted against the allotment — it is
       always preserved.

# MUST READ — the design decisions (header/body split, sentinel, path-keying, token→rune, test plan)
- docfile: plan/007_b33d310438c6/P1M4T2S2/research/design-decisions.md
  why: D1 (scope — 3 pure functions; S2 is pure like S1), D2 (path-keyed allotments map — robust to
       git-emission-order vs numstat-sorted-by-path drift; path-miss ⇒ pass-through), D3 (header block =
       lines before first `@@`; body = first `@@` onward; truncation = first-L-tokens-of-body stream),
       D4 (the SHORTER `... [truncated]` sentinel, on its own line, only when content removed), D5 (token→
       rune: allotment×4 runes; firstNRunes byte-offset iteration, no []rune alloc), D6 (path regexes +
       quote-strip mitigation), D7 (split on `diff --git ` trailing-space, re-prefix), D8 (recompose in
       input order; byte-identical pass-through), D9 (the pure test plan), D10 (frozen/out-of-scope).
  critical: D2 (path-keyed, NOT index-parallel — orderings differ), D3 (body starts at first `@@`, NOT
       "all @@ are headers" which would malformed multi-hunk), D4 (the sentinel STRING is pinned), D5
       (rune-budget = allotment×4, rune-boundary slice).

# MUST READ — the consumer contract (S1's solver output → S2's input; the frozen seam)
- docfile: plan/007_b33d310438c6/P1M4T2S1/PRP.md
  section: "CONSUMER.S2 (P1.M4.T2.S2 — per-file truncation application; DO NOT implement here)": "S2
       builds sizes (EstimateTokens per captured file body) + body_budget (from M4.T3), calls
       waterFillLevel/allocByWaterFill, then truncates each file's body to allots[i] tokens (first level
       tokens + the `... [truncated]` sentinel) when allots[i] < sizes[i], preserving diff --git/hunk
       headers." AND the "OUTPUT (downstream — the frozen consumer contract)" block.
  why: confirms (a) the allotments come from S1's allocByWaterFill (S2 does NOT call the solver — the gate
       does, then hands S2 the map); (b) the unit is TOKENS via EstimateTokens; (c) the sentinel is the
       `... [truncated]` form; (d) headers are preserved. S2's signatures (splitDiffSections/diffSectionPath
       /truncateByWaterFill) are the seam M4.T3 calls. Do NOT add params — downstream depends on these shapes.
  critical: S2 does NOT call allocByWaterFill/waterFillLevel (the GATE does). S2 RECEIVES the allotments
       map. Do NOT compute body_budget (M4.T3). The "allots[i] < sizes[i]" decision is realized in S2 as
       "EstimateTokens(body) > allotment" (coherent when the gate sizes via EstimateTokens).

# MUST READ — the EXACT captured-section format (verified; the input S2 operates on)
- file: internal/git/stagediff_test.go   (READ — the real section shape; do NOT edit)
  section: the TestStripIndexLines table around L605–622. input/want pairs like:
       input: "diff --git a/a.go b/a.go\nindex 600d48a..62b056e 100644\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n"
       want:  "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n"
  why: shows EXACTLY what a captured, FR3h-index-stripped section looks like — the `index` line is GONE
       (FR3h, M2.T3.S1, already applied upstream); the section is `diff --git a/p b/p` + `--- a/p` + `+++ b/p`
       + `@@ ...` + content. This is the string S2 splits (at the first `@@`) into headerBlock + body. Use
       these EXACT shapes as the table-test inputs (hardcode them — do not derive from git in a pure test).
  pattern: build test sections as Go string literals with explicit `\n` — exactly like TestStripIndexLines.

# MUST READ — the test pattern to mirror (pure table-driven, HARDCODED expectations)
- file: internal/git/numstat_test.go   (READ — mirror its style; do NOT edit)
  section: TestResolveNumstatPath — a `tests := []struct{ in, want, desc string }{...}` table with HARDCODED
       `want` (never derived from the function), run via `t.Run(tc.desc, ...)`. The comment "Pure function;
       no I/O."
  why: this task's truncatediff_test.go mirrors EXACTLY this style — table-driven, hardcoded expectations,
       t.Run subtests, PURE (no git repo, no t.TempDir, no I/O). The functions under test are pure string
       arithmetic; every case is a string literal.
  pattern: `tests := []struct{ sections []string; allotments map[string]int; want string; desc string }{...}`;
       loop with `t.Run(tc.desc, func(t *testing.T){ if got := truncateByWaterFill(tc.sections, tc.allotments);
       got != tc.want { t.Errorf(...) } })`.

# READ — the estimator whose UNIT + inverse S2 uses (in-package; no import statement)
- file: internal/git/tokens.go   (READ ONLY — do NOT edit)
  section: `func EstimateTokens(s string) int` = ceil(utf8.RuneCountInString(s) / 4) (P1.M4.T1.S1).
  why: S2 uses EstimateTokens for the "needs truncation?" check (`EstimateTokens(body) > allotment`) AND the
       inverse relationship (allotment tokens ⟺ allotment×4 runes) for the body cutoff. Calling the SAME
       estimator keeps the unit coherent with the water-fill (which sizes bodies via EstimateTokens per
       S1's contract). In-package (`package git`) — no import line.
  gotcha: do NOT "improve" EstimateTokens to chars/3 (the architecture doc's ceiling-recommendation) — the
       contract pins chars/4 and the safety margin (applied in M4.T2/M4.T3) absorbs the code-vs-prose gap.
       S2's allotment×4 rune budget is the faithful inverse of chars/4.

# READ — the path keying S2's diffSectionPath must AGREE with (destination resolution)
- file: internal/git/numstat.go   (READ ONLY — do NOT edit)
  section: `resolveNumstatPath` + `numstatRow.Path` (the destination, rename-resolved). numstatRows sorts by
       Path. The allotments map M4.T3 builds is keyed by THIS destination path.
  why: S2's diffSectionPath MUST yield the SAME destination string so the map keys match. For a rename:
       numstat (with -M) gives `old => new` → resolveNumstatPath → `new`; the diff body's `diff --git a/old
       b/new` → diffSectionPath group 2 → `new`. Agreement holds. For a normal edit both give the plain path.
  gotcha: paths with special chars are QUOTED in `diff --git`/`+++` (C-style) but UNQUOTED in numstat
       (tab-separated) → potential key mismatch. S2 strips one `"` pair as basic mitigation; full
       core.quotePath unquoting is out of scope (D6) — a mismatch degrades to safe pass-through (over-include).

# READ — the legacy aggregate cap S2 REPLACES (context only; the gate owns the substitution)
- file: internal/git/git.go   (READ ONLY — do NOT edit)
  section: StagedDiff Part 2 (L848–869): `nmDiff = stripIndexLines(nmDiff)` then `if len(nmDiff) >
       maxDiffBytes { nmDiff = nmDiff[:maxDiffBytes] + fmt.Sprintf("\n... [diff truncated at %d bytes]",
       maxDiffBytes) }`. The markdown per-file cap (L838–840) uses `... [diff truncated at %d lines]`.
  why: shows the LEGACY aggregate byte-cap + its N-bearing sentinel — the behavior M4.T3 REPLACES with S2's
       per-file water-fill when token_limit>0 (system_context §6 inv 2). S2 NEVER emits these legacy forms;
       M4.T3 keeps them byte-identical at token_limit==0. The split point (Part 2 nmDiff capture) is where
       M4.T3 inserts `splitDiffSections` + `truncateByWaterFill`.
  gotcha: do NOT touch git.go (M4.T3's territory). S2 ADDS 2 files only.

- url: (PRD §9.1 FR3i — in context as selected_prd_content h3.17; ALSO plan/007_b33d310438c6/prd_snapshot.md §9.1 FR3i)
  why: FR3i is the AUTHORITATIVE feature contract — "every file larger than L is truncated to exactly L
       (its first L tokens + the `... [truncated]` sentinel)"; "each file's `diff --git`/hunk headers are
       always preserved"; "the aggregate non-markdown diff is split on `diff --git` boundaries to apply the
       per-file level without extra git invocations."
  critical: FR3i's body_budget computation (token_limit − skeleton − prompt − margin) is M4.T3's — NOT S2.
       S2's `allotments` param RECEIVES the already-computed per-file values. FR3i's "split on diff --git
       boundaries … without extra git invocations" IS S2's `splitDiffSections` (pure string split).
```

### Current Codebase tree (relevant slice)

```bash
internal/git/
  tokens.go / tokens_test.go          # P1.M4.T1.S1 — EstimateTokens (ceil(runes/4)). READ ONLY (the UNIT; in-package call, no import).
  numstat.go / numstat_test.go        # P1.M3.T1.S1 — numstatRow.Path + resolveNumstatPath (destination keying S2 must agree with) + the test style to mirror. READ ONLY.
  skeleton.go / skeleton_test.go      # P1.M3.T1.S2 — renderNumstatSkeleton. READ ONLY.
  binary.go / binary_test.go          # FR3a/b/c binary filter + placeholder. READ ONLY (sibling).
  git.go                              # StagedDiff/TreeDiff/WorkingTreeDiff + StagedDiffOptions + buildDiffArgs + stripIndexLines. UNCHANGED (M4.T3's territory).
  stagediff_test.go                   # TestStripIndexLines L605–622 — the EXACT captured-section shape. READ ONLY (test-input reference).
  waterfill.go                        # P1.M4.T2.S1 (parallel) — waterFillLevel + allocByWaterFill (the SOLVER). READ ONLY (S2 does NOT call it; the gate does).
  truncatediff.go                     # *** CREATE *** — splitDiffSections + diffSectionPath + firstNRunes + truncateByWaterFill (pure; stdlib only).
  truncatediff_test.go                # *** CREATE *** — exhaustive pure table tests (all item tests + edge cases).
go.mod / go.sum                       # UNCHANGED (stdlib only; no new deps).
```

### Desired Codebase tree with files to be added/changed

```bash
internal/git/truncatediff.go          # NEW — splitDiffSections + diffSectionPath + firstNRunes + truncateByWaterFill (pure; stdlib strings+unicode/utf8+regexp). Doc: FR3i/§6/sentinel-invariant/header-body-split/path-keying/frozen-consumer.
internal/git/truncatediff_test.go     # NEW — exhaustive pure table (hardcoded): 3-file-truncate, headers-preserved, sentinel, byte-identical-pass-through, multi-hunk, markdown, path-miss, pure-rename, empty.
# NO other files changed. go.mod/go.sum UNCHANGED. git.go/waterfill.go/numstat.go/skeleton.go/tokens.go/binary.go/StagedDiffOptions UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the sentinel is the SHORTER `... [truncated]` form — system_context §6 invariant 2): S2 emits
//   EXACTLY `... [truncated]` (on its own line, leading `\n`), NEVER the legacy `... [diff truncated at N
//   bytes/lines]` (git.go L840/L868 — those stay ONLY on the token_limit==0 path M4.T3 owns). The sentinel
//   is appended ONLY when content was actually removed (EstimateTokens(body) > allotment); a within-budget
//   section is byte-identical (NO sentinel).

// CRITICAL (path-keyed allotments, NOT index-parallel — D2): the diff sections come in git's EMISSION
//   order; the numstatRows that key the allotments map are SORTED BY PATH. The orderings can drift (renames,
//   binary rows). Index-parallel mapping would scramble. diffSectionPath yields the destination (b/) which
//   matches resolveNumstatPath's destination → map keys agree. A path-MISS (ok=false OR absent from map) ⇒
//   pass-through verbatim (safe over-include; never a wrong truncation).

// CRITICAL (header block = lines BEFORE the first `@@`; body = first `@@` onward — D3): do NOT treat "all
//   `@@` lines are headers" — that would preserve every hunk header while truncating only content, producing
//   malformed hunks (@@ with no content). The body is a STREAM from the first `@@`; truncating it to L tokens
//   naturally keeps the @@ headers within the first L tokens (the first @@ is at body start ⇒ always kept)
//   and cuts the rest. This is the only truncation yielding a well-formed partial diff + matches FR3i's
//   "first L tokens".

// CRITICAL (token → rune budget = allotment × 4 — D5): EstimateTokens = ceil(runes/4); its inverse is
//   allotment×4 runes (ceil(r/4) ≤ n ⟺ r ≤ 4n). Truncate the body to its first allotment×4 RUNES (NOT bytes
//   — byte-slicing could split a multi-byte UTF-8 char). "Needs truncation?" = EstimateTokens(body) >
//   allotment ⟺ utf8.RuneCountInString(body) > allotment×4. When the gate sizes bodies via EstimateTokens
//   (S1's unit), this is EXACTLY the water-fill's "file > L" condition ⇒ coherence: whole-file ⇒ no
//   truncation; capped-file ⇒ truncated.

// GOTCHA (split on `diff --git ` with the TRAILING SPACE — D7): the item pins "split on `diff --git `
//   boundaries". The trailing space distinguishes the section header from a content line that happens to
//   start with "diff --git" (vanishingly rare, but the space is the faithful boundary). strings.Split on the
//   separator REMOVES it — RE-PREFIX each section with `diff --git ` so it is self-contained (its first line
//   is `diff --git a/<p> b/<p>`, what diffSectionPath + the header parser expect).

// GOTCHA (the `index` line is ALREADY GONE before S2 sees the text — FR3h, M2.T3.S1): stripIndexLines
//   (git.go L717) runs at capture time, upstream of S2. Do NOT re-strip `index` lines in S2. A captured
//   section is `diff --git` + (extended) + `---` + `+++` + `@@` + content — NO `index` line (see
//   stagediff_test.go L605→L606: the input HAS the index line, the want does NOT; S2 operates on the `want`
//   shape).

// GOTCHA (pure-rename / mode-only section has NO `@@` line): the section is `diff --git` + `similarity
//   index` + `rename from`/`rename to` (or `old mode`/`new mode`) — body = "". No truncation (pass-through).
//   These are tiny anyway (~3 lines). Handle by: if no `@@` line found, headerBlock = whole section, body = "".

// GOTCHA (markdown per-file sections use the SAME code path): each markdown file's diff (Part 1, captured
//   individually with `git diff --cached -- <file>`) IS a section starting with `diff --git a/<p> b/<p>`.
//   truncateByWaterFill handles it identically (extract path → allotment → split at @@ → truncate body).
//   The gate passes the markdown file diffs as a []string; S2 is uniform.

// GOTCHA (path quoting — D6): git quotes paths with bytes >0x7f or "/\/control chars (and sometimes spaces)
//   in `diff --git`/`+++` as C-style `"..."`; numstat paths are UNQUOTED (tab-separated). A quoted diff path
//   would mismatch the unquoted numstat key ⇒ path-miss ⇒ pass-through (the file is NOT truncated — a
//   graceful degradation, not a crash; the FR3g skeleton is the completeness floor). Mitigation: strip ONE
//   surrounding `"` pair (handles common quoted-space paths). Full core.quotePath unquoting is OUT OF SCOPE.

// GOTCHA (frozen signatures — do NOT change): splitDiffSections(string)[]string, diffSectionPath(string)
//   (string,bool), truncateByWaterFill([]string, map[string]int) string are the seam M4.T3 calls. Do NOT add
//   params (a context, an estimator func, a sentinel string). The allotments are a map[string]int (path→tokens).

// GOTCHA (do NOT call allocByWaterFill/waterFillLevel — S1's solver): S2 RECEIVES the allotments map; the
//   GATE (M4.T3) builds it from the solver + numstatRows + EstimateTokens. S2 does not compute body_budget,
//   does not call the solver. S2 calls ONLY EstimateTokens (in-package) for the truncation check.

// GOTCHA (do NOT touch git.go or the 3 diff functions): the gate substitution (token_limit>0 ⇒
//   splitDiffSections + truncateByWaterFill replace the byte cap) is M4.T3's territory. numstat.go/
//   skeleton.go/tokens.go/binary.go/waterfill.go are siblings' frozen territory. S2 ADDS 2 files only.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/git/truncatediff.go
package git

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// diffSectionHeaderRe matches a section's leading `diff --git a/<src> b/<dst>` line and captures the
// DESTINATION path <dst> (group 2). (?m)^ anchors at line start. For a rename <dst> is the NEW path —
// matching resolveNumstatPath's destination resolution (numstat.go), so the path agrees with the
// allotments map key. Pure; compiled once.
var diffSectionHeaderRe = regexp.MustCompile(`(?m)^diff --git a/(.*) b/(.*)$`)

// diffSectionPlusPlusRe is the FALLBACK destination extractor: `+++ b/<dst>` (group 1). Reached only when
// the `diff --git` header is absent/malformed. A deletion's `+++ /dev/null` does not match `b/`. Pure.
var diffSectionPlusPlusRe = regexp.MustCompile(`(?m)^\+\+\+ b/(.*)$`)

// atAtRe matches a hunk header line `@@ -l,s +l,s @@ …` (FR3h keep-table: a hunk anchor). Used to split a
// section into its header block (lines before the first @@) and body (the first @@ onward). (?m)^@@ anchors
// at line start; a content line carrying a leading @@ (vanishingly rare, and would need a ` `/`+`/`-` marker)
// does not match because content lines start with a diff marker. Pure.
var atAtRe = regexp.MustCompile(`(?m)^@@`)

// splitDiffSections splits the captured non-markdown aggregate diff on `diff --git ` boundaries (PRD §9.1
// FR3i: "the aggregate non-markdown diff is split on `diff --git` boundaries to apply the per-file level
// without extra git invocations"). Each returned section is self-contained — its first line is
// `diff --git a/<p> b/<p>` — because the `diff --git ` separator is re-prefixed after the split. A truly-
// empty leading element (text before the first `diff --git`) is dropped; a non-empty leading element is
// preserved as its own leading section (defensive — should not occur for a clean non-md aggregate, which
// always starts with `diff --git`). Empty/whitespace input → [].
//
// PURE: string manipulation only; no git, no I/O, no context. The input is the ALREADY-captured, ALREADY
// FR3h-index-stripped non-markdown aggregate (the `index <oid>..<oid> <mode>` line is gone upstream).
func splitDiffSections(diff string) []string {
	diff = strings.TrimSpace(diff)
	if diff == "" {
		return nil
	}
	// Split on the literal `diff --git ` (trailing space — the faithful section boundary).
	parts := strings.Split(diff, "diff --git ")
	var sections []string
	for i, p := range parts {
		if i == 0 {
			// Leading element: text before the first `diff --git`. Drop if empty; keep if non-empty
			// (defensive — a stray placeholder/comment would be preserved, not lost).
			if strings.TrimSpace(p) != "" {
				sections = append(sections, p)
			}
			continue
		}
		sections = append(sections, "diff --git "+p) // re-prefix so the section is self-contained
	}
	return sections
}

// diffSectionPath extracts the DESTINATION (b/) path from a section so it can be matched against the
// allotments map (keyed by numstat destination path — resolveNumstatPath, numstat.go). Preference:
// (1) the `diff --git a/<src> b/<dst>` line → <dst> (group 2); (2) fallback `+++ b/<dst>` (group 1).
// One surrounding pair of `"` is stripped (basic mitigation for git-quoted paths with spaces/special
// chars — full core.quotePath unquoting is out of scope; a residual mismatch degrades to safe
// pass-through in truncateByWaterFill). ok is false when neither line matches (e.g. a non-diff string).
// PURE.
func diffSectionPath(section string) (path string, ok bool) {
	if m := diffSectionHeaderRe.FindStringSubmatch(section); m != nil {
		return stripOneQuotePair(m[2]), true
	}
	if m := diffSectionPlusPlusRe.FindStringSubmatch(section); m != nil {
		return stripOneQuotePair(m[1]), true
	}
	return "", false
}

// stripOneQuotePair removes a single leading+trailing `"` pair from s if both are present (basic
// mitigation for git-quoted diff paths). Pure helper for diffSectionPath.
func stripOneQuotePair(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// firstNRunes returns s's first n runes, rune-boundary-safe, WITHOUT allocating a full []rune (it iterates
// byte offsets via `for i := range s`, stopping at the n-th rune's start). n <= 0 → "". Fewer than n runes
// → s whole. Used by truncateByWaterFill to cut a body to allotment×4 runes (allotment tokens under the
// chars/4 estimator). PURE.
func firstNRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for i := range s { // i = byte offset of each rune start (utf8-decoded by the range clause)
		if count == n {
			return s[:i]
		}
		count++
	}
	return s // fewer than n runes
}

// truncatedSentinel is the FR3i per-file truncation sentinel — the SHORTER `... [truncated]` form
// (system_context.md §6 invariant 2: "The `... [truncated]` sentinel (shorter form, per PRD FR3i) is
// emitted per truncated file; the `at N bytes` sentinels do NOT appear"). DISTINCT from the legacy
// aggregate sentinels (`... [diff truncated at N bytes/lines]`, git.go) which S2 must NEVER emit. Appended
// on its OWN line (leading `\n`), matching the legacy sentinels' line shape, ONLY when content was removed.
const truncatedSentinel = "... [truncated]"

// truncateByWaterFill applies per-file token allotments (PRD §9.1 FR3i; git_diff_semantics.md §6) to a list
// of per-file diff sections and returns the recomposed diff body under token_limit. For each section:
//
//  1. extract its destination path (diffSectionPath); if not found OR absent from allotments → pass through
//     verbatim (path-miss ⇒ safe over-include; never a wrong truncation).
//  2. split the section into a HEADER BLOCK (the `diff --git`/extended-header/`---`/`+++` lines — everything
//     before the first `@@` line) and a BODY (the first `@@` onward). A section with no `@@` (pure rename /
//     mode-only) has an empty body ⇒ no truncation (pass through).
//  3. if EstimateTokens(body) > allotment: replace the body with its first allotment×4 RUNES (allotment
//     tokens under the chars/4 estimator) + "\n" + truncatedSentinel. Else: body unchanged (within budget ⇒
//     byte-identical, NO sentinel).
//  4. append headerBlock + (possibly truncated) body.
//
// Sections are recomposed in ORIGINAL INPUT ORDER (NOT sorted — FR3i: "Recompose the sections in original
// order"). The same function serves the markdown per-file sections (each is a self-contained `diff --git`
// section) — uniform handling per the item_description. allotments is a map[string]int (numstat destination
// path → token allotment), built by the M4.T3 gate from allocByWaterFill (S1) + EstimateTokens(body) sizes.
//
// PURE: no git, no I/O, no context. Calls ONLY EstimateTokens (in-package). The `index` line is already
// stripped upstream (FR3h); do NOT re-strip. Signature FROZEN — consumed by M4.T3 (P1.M4.T3.S1, the
// token-limit gate). Empty sections → "".
func truncateByWaterFill(sections []string, allotments map[string]int) string {
	if len(sections) == 0 {
		return ""
	}
	var b strings.Builder
	for _, section := range sections {
		path, ok := diffSectionPath(section)
		allotment, found := 0, false
		if ok {
			allotment, found = allotments[path]
		}
		// Path-miss (no diff --git/+++ b/, OR path absent from allotments, OR non-positive allotment):
		// pass through verbatim — safe over-include, never a wrong truncation.
		if !ok || !found || allotment <= 0 {
			b.WriteString(section)
			continue
		}
		// Split into header block (before first @@) + body (first @@ onward).
		loc := atAtRe.FindStringIndex(section)
		if loc == nil {
			// No hunk (pure rename / mode-only) → no body to truncate → pass through.
			b.WriteString(section)
			continue
		}
		headerBlock := section[:loc[0]]
		body := section[loc[0]:]
		if EstimateTokens(body) > allotment {
			// allotment tokens ⟺ allotment×4 runes (inverse of ceil(runes/4)). Rune-boundary slice.
			body = firstNRunes(body, allotment*4) + "\n" + truncatedSentinel
		}
		b.WriteString(headerBlock)
		b.WriteString(body)
	}
	return b.String()
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/git/truncatediff.go (the 3 pure functions + helpers — stdlib only)
  - FILE: NEW internal/git/truncatediff.go. PACKAGE: `package git`. IMPORTS: `regexp`, `strings`,
      `unicode/utf8` ONLY (no context, no I/O, no other internal/git symbols beyond the in-package
      EstimateTokens call). NOTE: `unicode/utf8` is imported only if a direct utf8 call is used; the
      blueprint's firstNRunes uses `for i := range s` (no explicit utf8 call) — drop the import if unused
      (go vet flags unused imports). EstimateTokens is in-package (tokens.go) — NO import line for it.
  - DEFINE the 3 compiled regexes (diffSectionHeaderRe, diffSectionPlusPlusRe, atAtRe) + the
      truncatedSentinel const + the 4 functions EXACTLY as in "Data models" (paste + adapt the doc comments).
  - GOTCHA: splitDiffSections uses `strings.Split(diff, "diff --git ")` (TRAILING SPACE) and RE-PREFIXES
      each non-leading section with `"diff --git "`. Drop a TrimSpace-empty leading element; keep a non-empty
      one. TrimSpace the whole input first; empty → nil.
  - GOTCHA: diffSectionPath tries diffSectionHeaderRe FIRST (group 2 = destination), FALLS BACK to
      diffSectionPlusPlusRe (group 1), strips ONE quote pair (stripOneQuotePair), returns ok=false if
      neither. For a deletion, `+++ /dev/null` does not match `b/` but `diff --git a/x b/x` already gave `x`.
  - GOTCHA: firstNRunes uses `for i := range s` (i = byte offset of each rune start — the range clause
      UTF-8-decodes); stop at count==n, return s[:i]. NO `[]rune` allocation. n<=0 → "". Fewer than n → s.
  - GOTCHA: truncateByWaterFill — path-miss (ok=false OR path absent OR allotment<=0) ⇒ verbatim (continue).
      Split at atAtRe.FindStringIndex; loc==nil ⇒ verbatim. EstimateTokens(body) > allotment ⇒
      firstNRunes(body, allotment*4) + "\n" + truncatedSentinel. Join headerBlock+body; write to builder.
      EMPTY sections → "". Recompose in INPUT order (the loop order IS the input order).
  - RUN: gofmt -w internal/git/truncatediff.go ; go build ./internal/git/ → exit 0.

Task 2: CREATE internal/git/truncatediff_test.go (exhaustive PURE table tests — mirror numstat_test.go)
  - FILE: NEW internal/git/truncatediff_test.go. PACKAGE: `package git` (white-box — same package, like
      numstat_test.go). IMPORT: `strings`, `testing` (stdlib only; NO internal imports — the tests are pure).
      NO t.TempDir, NO git repo, NO I/O. Every case is a string literal with HARDCODED expectations.
  - PATTERN: mirror numstat_test.go's TestResolveNumstatPath — `tests := []struct{…}{...}` with HARDCODED
      `want` (never derived from the function — circular), run via `t.Run(tc.desc, …)`.
  - TestSplitDiffSections cases (HARDCODED):
      * Empty: "" → nil; "   \n  " → nil.
      * Single: "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n" → [that string] (1 elem,
        re-prefixed; since there's no leading text, the split yields ["", "a/x b/x\n..."] → re-prefix → the
        original). Assert len==1 and sections[0] starts with "diff --git a/x b/x".
      * 3-file: concatenate 3 sections; assert len==3 and each starts with "diff --git a/" and the 3 paths
        appear in order.
      * Leading non-diff text: "PREAMBLE\ndiff --git a/x b/x\n..." → [PREAMBLE-section, diff-section] (the
        non-empty leading element is preserved).
  - TestDiffSectionPath cases (HARDCODED path, ok):
      * Normal edit: section with "diff --git a/a.go b/a.go" → ("a.go", true).
      * New file: "diff --git a/x.go b/x.go\nnew file mode 100644\n--- /dev/null\n+++ b/x.go\n@@ ..." →
        ("x.go", true).
      * Deletion: "diff --git a/old.go b/old.go\ndeleted file mode 100644\n--- a/old.go\n+++ /dev/null" →
        ("old.go", true) [from the diff --git line; +++ /dev/null does not match b/].
      * Rename: "diff --git a/old.go b/new.go\nsimilarity index 80%\nrename from old.go\nrename to new.go
        \n--- a/old.go\n+++ b/new.go\n@@ ..." → ("new.go", true).
      * +++ fallback: a section WITHOUT a diff --git line but WITH "+++ b/fallback.go" → ("fallback.go",
        true). (Synthetic — real sections always have diff --git; tests the fallback branch.)
      * Quote-strip: section with `diff --git "a/foo bar" "b/foo bar"` → ("foo bar", true) [one quote pair
        stripped].
      * Non-diff: "M\t[binary] assets/logo.png\n" → ("", false).
  - TestTruncateByWaterFill cases (HARDCODED want — the item's named tests + edge cases):
      * ITEM TEST — 3-file, one exceeds L: build 3 sections (A small, B LARGE body, C small). allotments =
        {"a/A.go": 10000, "b/B.go": 20, "c/C.go": 10000} (A,C within budget; B capped at 20 tokens). Assert:
        (a) B is truncated — strings.Contains(want_B_section, "... [truncated]"); (b) A and C are
        BYTE-IDENTICAL — strings.Contains(out, sectionA) && strings.Contains(out, sectionC); (c) the sentinel
        appears EXACTLY ONCE — strings.Count(out, "... [truncated]") == 1.
      * ITEM TEST — headers preserved on the truncated file: the truncated B section contains "diff --git
        a/b/B.go b/b/B.go", "--- a/b/B.go", "+++ b/b/B.go", AND the first "@@" (it's at body start, within
        the 20-token allotment).
      * ITEM TEST — sentinel on its own line: the truncated B section ends with "\n... [truncated]".
      * All-within-budget: allotments all ≥ body sizes → out BYTE-IDENTICAL to the joined input (assert
        strings.Count(out, "... [truncated]") == 0).
      * Multi-hunk truncated: a section with 2 hunks (2 `@@` blocks), allotment < both-hunks size → body cut
        mid-way, sentinel appended, the FIRST `@@` present, the second `@@` (after the cutoff) ABSENT
        (strings.Count(truncated_section, "@@") == 1).
      * Markdown per-file section: a single .md file diff as sections=[]string{mdSection}, allotments small
        → truncated identically (same code path; assert sentinel present).
      * Path-miss pass-through: a section whose path is absent from allotments → verbatim (assert
        strings.Contains(out, that section) && no sentinel for it).
      * Pure-rename section (no @@): "diff --git a/old b/new\nsimilarity index 100%\nrename from old\nrename
        to new\n" with a SMALL allotment → verbatim (no @@ ⇒ no body ⇒ no truncation; assert no sentinel).
      * Empty: sections=[] → "".
  - GOTCHA: HARDCODE all `want` strings / assertions (do NOT compute want via the function — circular). For
      the byte-identical cases, assert via strings.Contains(out, originalSection). For sentinel counts use
      strings.Count. Build sections as Go string literals with explicit \n (mirror stagediff_test.go L605).
  - RUN: gofmt -w internal/git/truncatediff_test.go ; go test ./internal/git/ -run
      'SplitDiffSections|DiffSectionPath|TruncateByWaterFill|FirstNRunes' -v.

Task 3: VALIDATE (run all gates; fix before declaring done)
  - gofmt -w internal/git/truncatediff.go internal/git/truncatediff_test.go
  - go vet ./internal/git/ && go build ./...
  - go test ./internal/git/ -v -run 'SplitDiffSections|DiffSectionPath|TruncateByWaterFill|FirstNRunes'
      (the new pure table tests)
  - go test ./...   (ALL green — pure additions, no consumer reads them yet ⇒ no behavior change, no
      regression.)
  - git status → expect EXACTLY 2 new files (internal/git/truncatediff.go, internal/git/truncatediff_test.go).
  - git diff --exit-code internal/git/git.go internal/git/waterfill.go internal/git/numstat.go
      internal/git/skeleton.go internal/git/tokens.go internal/git/binary.go go.mod go.sum → empty (frozen
      files UNCHANGED).
  - ! grep -qE 'context\.|os\.|exec\.' internal/git/truncatediff.go   (confirm pure: no context/I/O/exec.)
  - ! grep -qE 'allocByWaterFill|waterFillLevel|numstatRows|numstatRow' internal/git/truncatediff.go
      (confirm: S2 does NOT call the solver or numstat — it only calls EstimateTokens; the solver/numstat are
      the gate's job.)
```

### Implementation Patterns & Key Details

```go
// PATTERN: split on the literal `diff --git ` (trailing space) and RE-PREFIX (D7). strings.Split removes
//   the separator; re-add it so each section is self-contained.
//   parts := strings.Split(diff, "diff --git ")
//   for i, p := range parts { if i==0 { if TrimSpace(p)!="" { append(p) }; continue }; append("diff --git "+p) }

// PATTERN: path-keyed allotments, path-miss ⇒ pass-through (D2). Robust to git-emission-order vs numstat-
//   sorted-by-path drift. diffSectionPath yields the destination (b/) = resolveNumstatPath's destination.
//   allotment, found := allotments[path]; if !ok || !found || allotment<=0 { write verbatim; continue }

// PATTERN: header block = before first @@; body = first @@ onward (D3). atAtRe.FindStringIndex gives the
//   byte range of the first @@ line; headerBlock = section[:loc[0]], body = section[loc[0]:]. loc==nil ⇒
//   pure rename/mode-only ⇒ verbatim.

// PATTERN: token → rune budget = allotment × 4 (D5). firstNRunes(body, allotment*4) is the largest body
//   prefix with EstimateTokens ≤ allotment (ceil(r/4) ≤ n ⟺ r ≤ 4n). Rune-boundary-safe (no split UTF-8).

// CRITICAL: the sentinel is `... [truncated]` (const truncatedSentinel) — the SHORTER form, on its own line
//   (`"\n" + truncatedSentinel`), ONLY when EstimateTokens(body) > allotment. NEVER the legacy N-bearing
//   forms. Within-budget ⇒ byte-identical, NO sentinel.

// CRITICAL: recompose in INPUT order (the loop order). Do NOT sort. The byte-identical pass-through guarantee
//   relies on (a) within-budget sections returned verbatim AND (b) join order == input order.

// GOTCHA: signatures are FROZEN (M4.T3 consumer). splitDiffSections(string)[]string,
//   diffSectionPath(string)(string,bool), truncateByWaterFill([]string, map[string]int) string. No extra
//   params. The allotments are map[string]int (path→tokens).

// GOTCHA: S2 calls ONLY EstimateTokens (in-package). It does NOT call allocByWaterFill/waterFillLevel (the
//   solver — the gate does) or numstatRows (the gate builds the path keying). S2 RECEIVES the allotments map.

// GOTCHA: the `index` line is ALREADY stripped upstream (FR3h, stripIndexLines at capture). Do NOT re-strip.
//   A captured section has NO `index` line (stagediff_test.go L605→L606 confirms).
```

### Integration Points

```yaml
TRUNCATION (internal/git/truncatediff.go):
  - +func splitDiffSections(diff string) []string                                    (FROZEN signature)
  - +func diffSectionPath(section string) (path string, ok bool)                     (FROZEN signature)
  - +func truncateByWaterFill(sections []string, allotments map[string]int) string   (FROZEN signature)
  - +func firstNRunes(s string, n int) string   (unexported helper)
  - +const truncatedSentinel = "... [truncated]"
  - +var diffSectionHeaderRe / diffSectionPlusPlusRe / atAtRe   (compiled once)

CONSUMER.M4.T3 (P1.M4.T3.S1 — the token-limit gate; DO NOT implement here):
  - call (non-markdown): "sections := splitDiffSections(nmDiff); allotments := <path→token allotment from
    allocByWaterFill(EstimateTokens(body) sizes, body_budget) keyed by numstatRow.Path>; nmDiff =
    truncateByWaterFill(sections, allotments). Substitutes for the legacy byte cap when token_limit>0."
  - call (markdown): "sections := <list of per-file markdown diffs>; mdDiff = truncateByWaterFill(sections,
    mdAllotments). Same function — uniform handling."

UNIT (tokens — applied via EstimateTokens, in-package):
  - "allotments values are TOKENS (from allocByWaterFill over EstimateTokens(body) sizes). S2's
    allotment×4 rune budget is the faithful inverse of EstimateTokens = ceil(runes/4). Coherent when the
    gate sizes bodies via EstimateTokens (S1's specified unit)."

SENTINEL (system_context §6 invariant 2):
  - "the SHORTER `... [truncated]` form (const), per-file, on the token_limit>0 path ONLY. The legacy
    `... [diff truncated at N bytes/lines]` aggregate sentinels (git.go L840/L868) remain byte-identical at
    token_limit==0 (M4.T3 owns that branch). S2 NEVER emits the legacy forms."

GO.MODULE: change NONE. stdlib `regexp` + `strings` (+ `unicode/utf8` only if a direct utf8 call is used)
  in truncatediff.go; `strings` + `testing` in truncatediff_test.go.

FROZEN/LEAVE (do NOT edit):
  - internal/git/git.go (StagedDiff/TreeDiff/WorkingTreeDiff + StagedDiffOptions + buildDiffArgs +
    stripIndexLines — M4.T3's territory; the gate substitution lives here).
  - internal/git/waterfill.go (S1 — the solver; S2 does NOT call it).
  - internal/git/{numstat,skeleton,tokens,binary}.go (siblings' frozen territory — READ only; S2 calls only
    EstimateTokens from tokens.go, in-package). go.mod/go.sum. The 6 diff call sites.
```

## Validation Loop

### Level 1: Syntax & Style

```bash
gofmt -w internal/git/truncatediff.go internal/git/truncatediff_test.go
go vet ./internal/git/
# Confirm purity (no context/I/O/exec) + no solver/numstat coupling (S2 calls only EstimateTokens):
! grep -qE 'context\.|os\.|exec\.' internal/git/truncatediff.go   && echo "pure (no I/O) ✓"
! grep -qE 'allocByWaterFill|waterFillLevel|numstatRows|numstatRow' internal/git/truncatediff.go   && echo "no solver/numstat coupling ✓"
# Confirm the signatures are exactly the frozen contract:
grep -n 'func splitDiffSections\|func diffSectionPath\|func truncateByWaterFill\|func firstNRunes' internal/git/truncatediff.go
#   expect: func splitDiffSections(diff string) []string
#           func diffSectionPath(section string) (path string, ok bool)
#           func truncateByWaterFill(sections []string, allotments map[string]int) string
#           func firstNRunes(s string, n int) string
# Confirm the sentinel const is the SHORTER form (NOT the legacy N-bearing form):
grep -n 'truncatedSentinel =' internal/git/truncatediff.go   # expect: `const truncatedSentinel = "... [truncated]"`
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Expected: go vet clean; pure + no-solver-coupling confirmed; signatures + sentinel match the frozen contract.
```

### Level 2: Unit tests (the pure table tests)

```bash
# The item's named tests (3-file truncate, headers preserved, sentinel, byte-identical pass-through):
go test ./internal/git/ -run 'TestTruncateByWaterFill' -v
# Expected: the 3-file case truncates ONLY the over-budget file (sentinel count == 1); the other two are
#           byte-identical (strings.Contains); the truncated file's diff --git/---/+++/first-@@ headers
#           survive; the sentinel is on its own line.

# The splitter + path-extractor tables:
go test ./internal/git/ -run 'TestSplitDiffSections|TestDiffSectionPath' -v
# Expected: split re-prefixes + drops empty leading + preserves non-empty leading; path extraction handles
#           normal/new-file/deletion/rename/+++-fallback/quote-strip/non-diff.

# The edge cases (multi-hunk, markdown, all-within-budget, path-miss, pure-rename, empty):
go test ./internal/git/ -run 'TestTruncateByWaterFill|TestFirstNRunes' -v
# Expected: multi-hunk cuts at the first @@ (second @@ absent after cutoff); markdown uses the same path;
#           all-within-budget is byte-identical (0 sentinels); path-miss ⇒ verbatim; pure-rename ⇒ verbatim;
#           empty sections ⇒ "".

# Full internal/git suite (no regression — pure additions):
go test ./internal/git/ -v
```

### Level 3: Whole-repo build/test + frozen-file check

```bash
go build ./...     # Expect clean.
go test ./...      # Expect all PASS — pure additions; no consumer reads the functions yet ⇒ no behavior change.
# Confirm ONLY the 2 new files differ:
git status --porcelain
# Expected: exactly 2: internal/git/truncatediff.go, internal/git/truncatediff_test.go.
# Confirm the frozen files are byte-unchanged:
git diff --exit-code internal/git/git.go internal/git/waterfill.go internal/git/numstat.go \
  internal/git/skeleton.go internal/git/tokens.go internal/git/binary.go go.mod go.sum \
  && echo "frozen files UNCHANGED (expected)"
```

### Level 4: Correctness reasoning (the truncation math, reproducible by hand)

```bash
# No git/DB/subprocess. Verify by reasoning + the Level-2 table tests:
#   1. allotment×4 runes ⟺ allotment tokens: EstimateTokens = ceil(runes/4). For allotment=20, runeBudget=80.
#      A body of 100 runes ⇒ EstimateTokens=25 > 20 ⇒ truncate to firstNRunes(body,80); EstimateTokens(80
#      runes)=20=allotment ✓. A body of 60 runes ⇒ EstimateTokens=15 ≤ 20 ⇒ verbatim ✓.
#   2. Header preservation: headerBlock = section[:loc[0]] (everything before the first @@); it is written
#      unchanged. The body = section[loc[0]:] starts with @@; for allotment>0 the first @@ (a few runes) is
#      always within the runeBudget ⇒ present after truncation ✓.
#   3. Byte-identical pass-through: within-budget ⇒ body unchanged, NO sentinel appended; headerBlock
#      unchanged; join order == input order ⇒ the section's bytes are identical ✓.
#   4. Sentinel form: the const is `... [truncated]` (shorter); appended as `\n` + const (own line); ONLY
#      when EstimateTokens(body) > allotment (content removed) ✓.
#   5. Path-keyed mapping: diffSectionPath returns the b/ destination = resolveNumstatPath's destination ⇒
#      allotments[path] hits; a miss ⇒ verbatim (safe) ✓.
# All 5 are table-tested with hardcoded expectations in truncatediff_test.go (Level 2).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./...` clean; `gofmt -l internal/git/` clean.
- [ ] `go test ./...` PASS (the new pure table tests; no repo-wide regression — pure additions).
- [ ] go.mod/go.sum byte-unchanged (`git diff --exit-code go.mod go.sum` empty).
- [ ] `git status` shows EXACTLY 2 new files; every frozen file byte-unchanged (git.go/waterfill.go/
      numstat.go/skeleton.go/tokens.go/binary.go).

### Feature Validation
- [ ] `splitDiffSections` splits on `diff --git ` (trailing space), re-prefixes each section, drops an empty
      leading element, returns `[]` for empty input.
- [ ] `diffSectionPath` returns the destination (group 2 of `^diff --git a/(.*) b/(.*)$`, fallback
      `^+++ b/(.*)$`), strips one `"` pair, returns `ok=false` when neither matches.
- [ ] `truncateByWaterFill` splits each section at the first `@@`; truncates the body to `allotment×4` runes
      + `\n... [truncated]` when `EstimateTokens(body) > allotment`; passes through verbatim otherwise;
      recomposes in input order.
- [ ] The item's named test passes: 3-file diff, one over-budget file truncated (sentinel count == 1), the
      other two byte-identical; the truncated file's `diff --git`/`---`/`+++`/first-`@@` headers preserved;
      sentinel on its own line.
- [ ] All edge cases pass: all-within-budget (byte-identical, 0 sentinels); multi-hunk (first `@@` kept,
      later `@@` cut); markdown per-file (same code path); path-miss pass-through; pure-rename pass-through;
      empty sections → "".

### Code Quality Validation
- [ ] `truncateByWaterFill` is PURE (no context/I/O/exec) and calls ONLY `EstimateTokens` (in-package) — no
      solver/numstat coupling.
- [ ] The sentinel is the SHORTER `... [truncated]` form (const); NEVER the legacy N-bearing forms.
- [ ] Header block / body split is at the first `@@` (not "all `@@` are headers"); token→rune budget is
      `allotment×4` (rune-boundary-safe); recomposition preserves input order.
- [ ] Doc comments cite FR3i/§6, the sentinel-form invariant (system_context §6 inv 2), the header/body
      split, the path-keyed-allotment robustness, and the frozen consumer signatures.
- [ ] Tests mirror numstat_test.go (table-driven, hardcoded expectations, pure — no git repo, no I/O).
- [ ] Anti-patterns avoided (see below); no out-of-scope churn; no new dependency.

### Documentation
- [ ] Doc comments are self-documenting (the algorithm, the sentinel form, the header/body split, the
      path-keying, the frozen seam).
- [ ] No new env vars / config / CLI surface (DOCS clause: "none — internal").

---

## Anti-Patterns to Avoid

- ❌ **Don't emit the legacy N-bearing sentinels.** S2 emits ONLY the shorter `... [truncated]` form
  (system_context §6 invariant 2). The `... [diff truncated at N bytes/lines]` forms (git.go L840/L868) stay
  on the token_limit==0 path M4.T3 owns. S2 is the token_limit>0 path exclusively.
- ❌ **Don't use index-parallel allotments.** Diff sections come in git's emission order; numstatRows are
  sorted by path — the orderings drift. Map by PATH (diffSectionPath destination ↔ numstat destination).
  A path-miss ⇒ verbatim pass-through (safe over-include), never a wrong truncation (D2).
- ❌ **Don't treat "all `@@` lines are headers."** That would preserve every hunk header while truncating
  only content, producing malformed hunks. The body is a STREAM from the first `@@`; truncating it to L
  tokens keeps the `@@` headers within the first L tokens and cuts the rest (D3). This is the only
  well-formed partial-diff truncation and matches FR3i's "first L tokens".
- ❌ **Don't truncate at a BYTE boundary.** A multi-byte UTF-8 char would be split. Use RUNE-boundary slicing
  (firstNRunes via `for i := range s`). The budget is allotment×4 RUNES (the faithful inverse of
  EstimateTokens = ceil(runes/4)) (D5).
- ❌ **Don't append the sentinel to within-budget sections.** Files within their allotment pass through
  BYTE-IDENTICAL — NO sentinel (the item: "non-truncated files byte-identical to input"). The sentinel is
  appended ONLY when EstimateTokens(body) > allotment (content was actually removed).
- ❌ **Don't re-strip the `index` line.** It is ALREADY gone (FR3h, stripIndexLines at capture, M2.T3.S1).
  S2 operates on the index-stripped shape (stagediff_test.go L605→L606). Re-stripping is dead code + risks a
  false match on a content line.
- ❌ **Don't call the solver or numstat.** S2 RECEIVES the allotments map; the GATE (M4.T3) builds it from
  allocByWaterFill (S1) + EstimateTokens(body) sizes + numstatRows path keying. S2 calls ONLY EstimateTokens
  (in-package) for the truncation check (D1/D10).
- ❌ **Don't change the frozen signatures.** `splitDiffSections(string)[]string`,
  `diffSectionPath(string)(string,bool)`, `truncateByWaterFill([]string, map[string]int) string` are the seam
  M4.T3 calls. No extra params (a context, an estimator func, a sentinel string) — downstream depends on
  these exact shapes.
- ❌ **Don't sort the sections on recompose.** Recompose in INPUT order (FR3i: "Recompose the sections in
  original order"). Sorting scrambles the diff narrative. The byte-identical pass-through guarantee relies
  on join order == input order (D8).
- ❌ **Don't derive test expectations from the function.** Table `want` values are HARDCODED (deriving via
  the function is circular). For byte-identical cases assert via `strings.Contains(out, originalSection)`;
  for sentinel counts use `strings.Count`. Build sections as `\n`-literal strings (mirror stagediff_test.go).
- ❌ **Don't touch git.go/waterfill.go/numstat.go/skeleton.go/tokens.go/binary.go or the diff functions.**
  They are siblings' frozen territory (M4.T3/S1/P1.M3). This task ADDS 2 files only.
- ❌ **Don't implement the gate (token_limit==0 vs >0 branch, body_budget computation, the allotments-map
  build, the substitution in the 3 diff functions).** That's M4.T3 (P1.M4.T3.S1). S2 provides the pure
  primitives; M4.T3 assembles.
