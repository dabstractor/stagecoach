# System Context — Bugfix 002 (PRD Issue 1: `splitDiffSections` fragmentation defeats `token_limit`)

Plan `007_b33d310438c6`, bugfix `002_430650446fcc`. This is the *second* adversarial pass after
bugfix `001_7e79f5773da8` (whose two findings are confirmed fixed in commits `a004b12` and `b7f723d`).

This document is the authoritative research record for the downstream PRP agents. Every claim below was
verified against the live codebase at HEAD on 2026-07-04.

---

## 1. The defect (PRD §9.1 Issue 1 — Major)

`splitDiffSections` (`internal/git/truncatediff.go:74-95`) splits the captured non-markdown diff
aggregate on the **un-anchored** substring `"diff --git "`:

```go
// internal/git/truncatediff.go:76  (the buggy line)
parts := strings.Split(diff, "diff --git ")   // splits on ANY occurrence, not just line starts
```

A **content line** inside a file's diff body that contains the literal `diff --git ` (e.g. an added
line `+diff --git a/foo b/foo` from a fixture/snapshot/doc that embeds a sample diff) is wrongly treated
as a section boundary. One real file is fragmented into many bogus tiny sections.

### Why this defeats `token_limit` (the FR3d contract)

The consumer is `applyWaterFillGate` (`internal/git/tokengate.go:129`), invoked in the
`opts.TokenLimit > 0` branch of all three diff functions. Its algorithm:

1. `nmSections := splitDiffSections(nmDiff)` — the buggy step.
2. `sizes[i] = EstimateTokens(sectionBody(sections[i]))` — sizes each section's body.
3. `allocs := allocByWaterFill(sizes, bodyBudget)` — water-fills the budget.
4. `truncateByWaterFill(sections, allotments)` — cuts over-budget bodies; within-budget bodies pass through byte-identical.

When a real large file is fragmented into N tiny bogus sections, each fragment's `size` is tiny, so the
water-fill concludes the change set fits the body budget and **truncates nothing** (every fragment is
within budget → byte-identical pass-through). The split→re-prefix round-trip is accidentally lossless
when nothing is truncated, so the full un-truncated content is re-emitted and the payload **silently
overflows `token_limit`** — the model's context window — by an unbounded factor (PRD measured: ~7× at
`token_limit=2000`, payload 14543 tokens, zero sentinels).

### Confirmed reproduction (this research session)

A unit probe against the live function confirms the root cause 100%:

```
TestRepro_SplitDiffSections_Unit:
  input  = "diff --git a/real.txt b/real.txt\n@@ -0,0 +1,1 @@\n+diff --git a/foo b/foo\n"
  got    = 2 sections  ["diff --git a/real.txt …+","diff --git a/foo b/foo\n"]
  want   = 1 section (ONE real file)
```

The first real file's body is CUT mid-content at the `+` (the added-line marker), and a bogus
`diff --git a/foo b/foo` section is fabricated. `strings.Count(input, "diff --git ") == 2` (one real
header at line start, one embedded in content) — the un-anchored split cannot distinguish them.

---

## 2. Scope & blast radius

- **Caller graph (verified):** `splitDiffSections` has exactly **ONE production caller** —
  `applyWaterFillGate` (`internal/git/tokengate.go:129`). `applyWaterFillGate` is in turn called by all
  three diff functions in their `opts.TokenLimit > 0` branch:
  - `StagedDiff` (`internal/git/git.go:883`)
  - `TreeDiff` (`internal/git/git.go:1360`)
  - `WorkingTreeDiff` (`internal/git/git.go:1533`)
  → A single-point fix in `splitDiffSections` covers all three (FR3c parity).

- **Triggered ONLY when `token_limit > 0`.** The default `0`/unset path uses the legacy whole-string
  byte-cap (`maxDiffBytes`) and **never calls `splitDiffSections`** → unaffected (regression anchor).

- **Triggered ONLY by non-markdown files.** Markdown files are collected per-file into `mdDiffs`
  (already self-contained `diff --git` sections) and bypass `splitDiffSections`; their truncation uses
  the line-anchored `diffSectionPath`/`atAtRe`. **Markdown needs NO change.**

- **Realistic triggers:** test fixtures / golden snapshots embedding sample diffs, documentation showing
  git diffs, vendored `.patch`/`.diff` files, source of git/diff tooling, changelogs quoting diffs.

---

## 3. The inconsistency (why this function alone is vulnerable)

Every sibling helper in `internal/git/truncatediff.go` is **`(?m)^` line-anchored** — only
`splitDiffSections` is un-anchored:

| Symbol | File:Line | Pattern | Anchored? |
|---|---|---|---|
| `diffSectionHeaderRe` | `truncatediff.go:27` | `(?m)^diff --git a/(.*) b/(.*)$` | ✅ line |
| `diffSectionPlusPlusRe` | `truncatediff.go:33` | `(?m)^\+\+\+ b/(.*)$` | ✅ line |
| `atAtRe` | `truncatediff.go:40` | `(?m)^@@` | ✅ line |
| **`splitDiffSections`** | `truncatediff.go:76` | `strings.Split(…, "diff --git ")` | ❌ **un-anchored** |

The inline comment at line 72 *defends* the trailing-space un-anchored split as "the faithful section
boundary that distinguishes the header from a content line that happens to start with `diff --git`". That
defense is WRONG: it only covers a content line starting with `diff --git` *without* the trailing space.
A content line carrying the full `diff --git ` (with space) is torn into a bogus boundary. **This comment
must be corrected as part of the fix.**

Why line-anchoring is safe for real git diff output: every content line is prefixed with a diff marker
(`+`, `-`, ` `, or `\` for no-newline). A line starting with `diff --git ` at column 0 is therefore
ALWAYS a real file-section header. The only theoretical false-positive — a context/added line whose
*de-prefixed* text starts with `diff --git ` — does not occur because `^` matches the prefixed line,
which starts with the marker, not `diff`.

---

## 4. The fix — recommended shape (Shape B, regex `FindAllStringIndex`)

The PRD offers two shapes. **Shape B is strongly recommended** (it preserves byte-identical round-
tripping and ALL existing test fixtures with zero newline bookkeeping). Shape A (split on
`"\ndiff --git "`) is more error-prone: the `\n` preceding a real header is consumed by the split, so a
non-empty leading element like `"PREAMBLE\n"` loses its trailing newline and would need re-adding — it
breaks the existing `TestSplitDiffSections` "non-empty leading element (PREAMBLE)" case unless carefully
special-cased.

### Shape B (recommended): line-anchored regex slice

Add a package-level regex (mirroring the sibling `diffSectionHeaderRe`/`atAtRe` style):

```go
// diffSectionBoundaryRe matches a real file-section header at a LINE START. (?m)^ anchors at column 0,
// so a content line carrying the literal `diff --git ` (always prefixed with a diff marker +/-/space/\)
// does NOT match. Mirrors the line-anchored siblings diffSectionHeaderRe/atAtRe. Pure; compiled once.
var diffSectionBoundaryRe = regexp.MustCompile(`(?m)^diff --git `)
```

Rewrite `splitDiffSections` to slice at the byte offsets returned by `FindAllStringIndex`:

```go
func splitDiffSections(diff string) []string {
	if strings.TrimSpace(diff) == "" {
		return nil
	}
	matches := diffSectionBoundaryRe.FindAllStringIndex(diff, -1)
	if len(matches) == 0 {
		// No section boundary (no line starts with "diff --git ") → the non-empty input is a single
		// blob. Preserve verbatim (should not occur for a clean non-md aggregate, which always starts
		// with `diff --git`).
		return []string{diff}
	}
	var sections []string
	// Leading content before the first boundary: drop if empty (TrimSpace), preserve if non-empty
	// (defensive — a stray placeholder/comment would be preserved, not lost).
	if first := matches[0][0]; first > 0 {
		if leading := diff[:first]; strings.TrimSpace(leading) != "" {
			sections = append(sections, leading)
		}
	}
	// Each section is sliced AT its header offset, so the `diff --git ` prefix is naturally present
	// (no re-prefixing needed). This preserves byte-identical split→join round-tripping.
	for i, m := range matches {
		start := m[0]
		end := len(diff)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		sections = append(sections, diff[start:end])
	}
	return sections
}
```

### Shape B verified against ALL existing `TestSplitDiffSections` fixtures

The exact algorithm above was simulated against every existing fixture + the bug case. **All pass
byte-identically** (this is critical — the fix must not regress the existing pin points):

| Fixture | Existing `want` | Shape-B `got` | Match? |
|---|---|---|---|
| `""` (empty) | `nil` | `nil` | ✅ |
| `"   \n  \t\n"` (ws-only) | `nil` | `nil` | ✅ |
| single section | 1 element (re-prefixed) | 1 element (prefix naturally present) | ✅ byte-identical |
| 3-file aggregate | 3 elements | 3 elements | ✅ byte-identical |
| `"PREAMBLE\ndiff --git a/x b/x\n…"` | `["PREAMBLE\n", "diff --git a/x b/x\n…"]` | `["PREAMBLE\n", "diff —git a/x b/x\n…"]` | ✅ (leading keeps its `\n`) |
| trailing content (`…extra`) | 1 element with `extra` | 1 element with `extra` | ✅ |
| **bug case** (`+diff --git a/foo b/foo`) | (not previously tested) | **1 section** (embedded literal inert) | ✅ FIXED |
| mid-file embedded (fake header inside file A's body) | (not previously tested) | **2 sections** (A + B; fake inert) | ✅ FIXED |

**Key property:** because Shape B slices at exact byte offsets, each section's `diff --git ` prefix is
already in the slice — no re-prefixing, no newline re-adding, no bookkeeping. Round-trip
`strings.Join(splitDiffSections(x), "") == x` holds for every clean aggregate, exactly as before.

### Shape A (alternative, NOT recommended)

Split on `"\ndiff --git "` and special-case the leading element. Viable but the leading-element newline
handling is fiddly (the consumed `\n` must be re-attached to a non-empty leading element). Only choose
this if there is a specific reason to avoid a regex; the file already imports `regexp` and uses it for
three siblings, so Shape B is the consistent choice.

---

## 5. Test coverage gap (the net that must be added)

### 5.1 Pure unit gap — `TestSplitDiffSections` (`internal/git/truncatediff_test.go`)

The existing table cases use ONLY clean multi-file aggregates where every `diff --git ` is a real header
(see fixtures §4 above). There is **no case with a content-embedded `diff --git ` literal** — the exact
gap that hid this bug. Add a table case (and ideally a mid-file variant):

- **Case (single, body-embedded):** one real file whose added line contains `+diff --git a/foo b/foo`
  → `want` exactly ONE section, byte-identical to the input.
- **Case (mid-file):** a 2-file aggregate where the FIRST file's body embeds a `diff --git ` literal
  → `want` exactly TWO sections (the literal is inert; the second real header is the only other boundary).

These ride WITH the fix in the same TDD subtask (failing test → fix → passing test).

### 5.2 E2E integration gap — `difftokenlimit_test.go` (the gap that hid the bug end-to-end)

`internal/git/difftokenlimit_test.go` stages large files under a truncating `token_limit`, but EVERY
existing test stages content that contains no `diff --git ` literals. The fragmentation path is
therefore never exercised end-to-end. Add E2E tests across **all three diff functions**
(StagedDiff / TreeDiff / WorkingTreeDiff) that:

1. Stage/create a **non-markdown** file whose **content** contains many `diff --git ` literals (e.g. a
   fixture/snapshot file — `strings.Repeat("diff --git a/fX b/fX\n@@ -1 +1 @@\n-old\n+new\n\n", 300)`),
   large enough to require truncation under a small `token_limit`.
2. Call the diff function with `TokenLimit` small enough that the single large file MUST be capped.
3. Assert the FR3d contract is upheld:
   - `(a)` **payload fits the budget:** `EstimateTokens(out)` ≤ `tokenLimit + tokenBudgetMargin` slack
     (the skeleton/header overhead the gate does not count against `bodyBudget`; the existing
     `TestStagedDiff_TokenLimitGt0_WaterFill` uses `≤ tokenLimit + 2*tokenBudgetMargin` as the slack
     ceiling — mirror that).
   - `(b)` **truncation occurred:** `strings.Count(out, "... [truncated]") >= 1` (the file was capped,
     not fragmented-and-passed-through-whole). Asserting `== 1` is ideal for a single-file case.
   - `(c)` **the file was NOT fragmented:** exactly ONE `diff --git a/<that-file>` header appears (not
     hundreds of bogus ones), i.e. `strings.Count(out, "diff --git a/"+<file>) == 1`.

Use the established scaffold (same as `TestStagedDiff_TokenLimitGt0_WaterFill`):
- `repo := t.TempDir(); initRepo(t, repo)` — `initRepo` is in `git_test.go:13` (bakes in identity; there
  is **no** `initRepoWithUser` helper — do not invent one).
- `writeFile(t, repo, name, body)` / `stageFile(t, repo, name)` — `committree_test.go:31`/`:39`.
- `g := New(repo)`; `g.StagedDiff(ctx, StagedDiffOptions{TokenLimit: N, PromptReserveTokens: 0})`.
- TreeDiff needs two tree SHAs via `writeTreeOf(t, repo)` (`committree_test.go`); WorkingTreeDiff needs a
  baseline HEAD via `commitAllowEmpty(t, repo, msg)` (`difftokenlimit_test.go:29`) then working-tree edits.
- Assertion style: plain `if`/`t.Errorf`/`t.Fatalf`, `strings.Contains`/`Count`, **NO testify**.

### 5.3 `initRepoWithUser` does NOT exist

The PRD's reproduction snippet references `initRepoWithUser`. That helper does not exist in this
codebase. Use `initRepo(t, dir)` (which already sets identity via env vars + `git config user.name/email`)
or `setIdentityConfig(t, dir)` separately. The E2E subtask's `context_scope` must use the real names.

---

## 6. Invariants the fix must NOT break

1. **`token_limit == 0` regression anchor (system_context.md §6 invariant 1):** the legacy
   `maxMDLines`/`maxDiffBytes` caps + `... [diff truncated at N bytes/lines]` sentinels apply
   byte-identically. `splitDiffSections` is NOT on this path → unaffected. (Pin: `TestStagedDiff_TokenLimitZero_LegacyCaps`.)

2. **Byte-identical within-budget pass-through (system_context.md §6 invariant 2):** a section whose
   body is within its allotment is emitted byte-identical with NO sentinel. The fix changes how the
   non-md aggregate is *sectioned*, not how within-budget sections are emitted — once each real file is
   correctly one section, a within-budget file passes through unchanged. (Pin: existing
   `TestTruncateByWaterFill` "all_within_budget_byte_identical_no_sentinels".)

3. **The FR3d contract (the point of this fix):** under `token_limit > 0`, the payload fits the budget.
   This is what the bug broke and the fix restores.

4. **Signature frozen:** `splitDiffSections(diff string) []string` — consumed unchanged by
   `applyWaterFillGate`. Do NOT change the signature.

---

## 7. Documentation surface (per SOW §5)

### 7.1 Mode A — doc-with-work (rides with the implementing subtask)

- **`internal/git/truncatediff.go` function doc comment (the `splitDiffSections` godoc, lines ~59-73).**
  The current comment *defends* the un-anchored trailing-space split ("the faithful section boundary
  that distinguishes the header from a content line that happens to start with `diff --git`"). That
  defense is now **factually wrong** and must be rewritten to explain line-anchoring (a content line is
  always diff-marker-prefixed, so `(?m)^diff --git ` matches only real headers). This is internal doc and
  rides WITH the fix subtask — do NOT create a separate docs subtask for it.
- **No user-facing / config / API surface change.** The bug is a correctness defect in an internal pure
  function. `token_limit`'s documented behavior ("the payload always fits your model's context window",
  `docs/configuration.md:146`) describes the *intended* behavior, which the fix *restores* — the docs are
  not made accurate by the bug and do not need a Mode-A edit to describe new behavior.

### 7.2 Mode B — changeset-level sweep (final task)

- `README.md:66` — the "Payload optimization" row; verify it does not imply anything now-incorrect (it
  won't — the fix restores the documented behavior).
- `docs/how-it-works.md:144` — describes the water-fill + `... [truncated]` marker "that ends the file's
  section on its own line, so the next file's `diff --git` begins fresh". Verify it does not imply
  content-embedded `diff --git` literals are a problem (it does not).
- `docs/configuration.md:146` — the FR3d "always fits" contract; now TRUE again. No edit expected.

Per SOW §5 the final Mode-B task MUST exist even though the most likely outcome is "no edit needed" —
the implementing agent confirms. It depends on every implementing subtask and runs last.

---

## 8. Key locations reference (verified line numbers, HEAD)

| Symbol / Site | File:Line | Role |
|---|---|---|
| `splitDiffSections` (buggy `strings.Split`) | `internal/git/truncatediff.go:74` (body :76) | THE FIX SITE |
| `diffSectionHeaderRe` (sibling, line-anchored) | `truncatediff.go:27` | pattern to mirror |
| `atAtRe` (sibling, line-anchored) | `truncatediff.go:40` | pattern to mirror |
| `truncatedSentinel = "... [truncated]"` | `truncatediff.go:57` | unchanged |
| `diffSectionPath` | `truncatediff.go` (~:100) | unchanged consumer |
| `truncateByWaterFill` | `truncatediff.go` (~:165) | unchanged consumer |
| `applyWaterFillGate` (sole caller) | `internal/git/tokengate.go:~95` (call :129) | unchanged consumer |
| `tokenBudgetMargin = 1024` | `tokengate.go:48` | budget slack constant |
| `EstimateTokens` (`ceilDiv(runes,4)`) | `internal/git/tokens.go:25` | unchanged |
| `TestSplitDiffSections` | `internal/git/truncatediff_test.go:~20` | ADD regression case |
| E2E scaffold (`initRepo`/`writeFile`/`stageFile`) | `git_test.go:13` / `committree_test.go:31`/:39 | reuse |
| StagedDiff `>0` branch | `git.go:883` | covered transitively |
| TreeDiff `>0` branch | `git.go:1360` | covered transitively |
| WorkingTreeDiff `>0` branch | `git.go:1533` | covered transitively |

---

## 9. Summary assessment

One systemic Major bug, single-point fix, fully understood. Shape-B (line-anchored regex slice) is
recommended and **pre-verified** to preserve every existing test fixture byte-identically while fixing
both the single-file and multi-file content-embedding cases. The fix lands once in `splitDiffSections`,
covering all three diff functions via the sole caller `applyWaterFillGate`. No critical issues, no minor
issues requiring action. Two regression nets are required (pure unit + E2E across all 3 diff functions)
because both layers had the same gap (clean aggregates only). The existing `internal/git` suite passes at
HEAD (confirmed: `go test ./internal/git/...` → `ok`).
