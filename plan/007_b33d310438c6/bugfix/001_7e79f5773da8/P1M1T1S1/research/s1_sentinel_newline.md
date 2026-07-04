# S1 Implementation Notes — sentinel-newline fix in truncateByWaterFill

> Scope: P1.M1.T1.S1. Append a trailing `\n` after the `... [truncated]` sentinel in the truncation
> branch of `truncateByWaterFill` so a truncated NON-LAST section's sentinel isn't glued to the next
> section's `diff --git` header. PLUS pure unit regression cases + 3 fixture-delta assertion updates.
> Verified against live source 2026-07-04.

## 1. The bug locus (internal/git/truncatediff.go, truncateByWaterFill)

```go
if EstimateTokens(body) > allotment {
    body = firstNRunes(body, allotment*4) + "\n" + truncatedSentinel   // ← NO trailing \n
}
b.WriteString(headerBlock)
b.WriteString(body)
```
`const truncatedSentinel = "... [truncated]"` (no trailing newline). The for-loop immediately writes
the NEXT section (which starts with `diff --git ...`), so a truncated non-last section produces
`... [truncated]diff --git a/b.go b/b.go` — the next file's header is glued onto the sentinel line.

**The fix (sentinel-only):**
```go
body = firstNRunes(body, allotment*4) + "\n" + truncatedSentinel + "\n"
```
The trailing `\n` separates the sentinel from the next section's `diff --git` (now at a line start).
This is the ONLY change to truncateByWaterFill.

## 2. CRITICAL: the byte-identical pass-through guarantee MUST hold (sentinel-only fix)

The fix adds a `\n` ONLY in the truncation branch. It MUST NOT touch:
- within-budget sections (`EstimateTokens(body) <= allotment` → body unchanged, written via
  `b.WriteString(headerBlock); b.WriteString(body)`) — these stay byte-identical, no sentinel, no extra \n.
- path-miss sections (`!ok || !found || allotment <= 0` → `b.WriteString(section); continue`) — verbatim.
- pure-rename/no-@@ sections (`loc == nil` → `b.WriteString(section); continue`) — verbatim.

Verified against the existing pass-through invariants (all stay GREEN after the fix):
- `all_within_budget_byte_identical_no_sentinels` → `want = s1+s2` (within-budget, no truncation branch).
- `path_miss_pass_through_verbatim` → `out == section`.
- `recompose_in_input_order` → `want = s1+s2+s3` (all within budget).
- `pure_rename_no_hunk_verbatim`, `zero_or_negative_allotment_verbatim`, `body_exactly_at_allotment_no_truncation`.
None enter the truncation branch → the added `\n` never fires → byte-identical preserved.

## 3. CRITICAL: THREE existing assertions break (not one) — the contract named only one

The contract says update `item_sentinel_on_its_own_line`. But THREE subtests in truncatediff_test.go use
the IDENTICAL `!strings.HasSuffix(out, "\n... [truncated]")` pattern, and ALL THREE break because the
output now ends with `... [truncated]\n` (trailing newline) instead of `... [truncated]`:

| line | subtest | current assertion | updated assertion |
|---|---|---|---|
| 287 | `item_sentinel_on_its_own_line` (contract-named) | `!HasSuffix(out, "\n... [truncated]")` | `!HasSuffix(out, "\n... [truncated]\n")` |
| 333 | `multi_hunk_truncated_first_atat_kept_second_cut` | same | same update |
| 352 | `markdown_per_file_section_same_code_path` | same | same update |

(Grep-confirmed: exactly 3 `HasSuffix` references in internal/git/*_test.go, all in truncatediff_test.go
at lines 287/333/352.) An implementer who updates only the contract-named one will see 2 more failures.
This is the #1 one-pass trap — update ALL THREE.

These are EXPECTED fixture deltas (system_context.md §6 invariant 3), NOT regressions: the sentinel is
still on its own line (preceded by `\n`); it now ALSO ends its line (followed by `\n`).

## 4. NO other test file breaks (verified)

The other git test files reference the sentinel only via `Contains`/`Count`, not the suffix:
- difftokenlimit_test.go:105 `Contains(out, "... [truncated]")`; :133 `Count == 1`; :67 legacy-absence.
- tokengate_test.go:95 `Count == 0`; :133 `Count == 1`; :144 legacy-absence.
- stagediff/treediff/workingtreediff_test.go: reference only the LEGACY `... [diff truncated at N bytes/lines]`.
- waterfill_test.go: solver math, no output-shape.
None assert the glued `[truncated]diff --git` shape or a no-trailing-newline suffix → none break.
Baseline: `go test ./internal/git/` is GREEN (`ok`).

## 5. The 4 new pure unit cases (truncate a NON-LAST section) — the regression that catches the bug

The existing tests only truncate the LAST/only section, so the multi-section concatenation gap hid the
bug. Add 4 t.Run subtests in TestTruncateByWaterFill, each with ≥2 sections where a truncated section is
FOLLOWED by another section. For each: `!Contains(out, "[truncated]diff --git")` AND
`Contains(out, "[truncated]\ndiff --git …")` (next header at a line start) AND sentinel count ==
#truncated files. Reuse itoa/tail; canonical section shape; no t.TempDir/IO/testify.

Build a "truncated" body via an inline loop (matching the existing style, e.g. 60 lines × `itoa`) with a
small allotment (10); "within-budget" = canonical 1-line body + large allotment (100000).

(a) `nonmd_truncated_then_nonmd_within_budget`: a.go truncated (allotment 10), b.go within budget.
    → `Contains(out, "[truncated]\ndiff --git a/b.go b/b.go")`, count==1.
(b) `md_truncated_then_nonmd`: README.md truncated (path key README.md), b.go within budget.
    → `Contains(out, "[truncated]\ndiff --git a/b.go b/b.go")`, count==1.
(c) `nonmd_truncated_then_md`: a.go truncated, NOTES.md within budget.
    → `Contains(out, "[truncated]\ndiff --git a/NOTES.md b/NOTES.md")`, count==1.
(d) `both_nonmd_truncated`: a.go AND b.go both truncated.
    → count==2; `Contains(out, "[truncated]\ndiff --git a/b.go b/b.go")`; out ends with `[truncated]\n`.

## 6. Scope discipline (S1 vs S2/S3)

S1 = the 1-line sentinel fix in truncatediff.go + the 3 assertion updates + the 4 new pure subtests in
truncatediff_test.go. truncateByWaterFill's SIGNATURE is FROZEN (consumed unchanged by applyWaterFillGate
and transitively StagedDiff/TreeDiff/WorkingTreeDiff).
- NOT S1: E2E integration regression across the 3 diff functions (difftokenlimit_test.go) = P1.M1.T2.S1.
- NOT S1: diff_context range validation (Issue 2) = P1.M2.T1.S1.
- NOT S1: docs sweep = P1.M3.T1.S1.
- DOCS: none in S1 (internal output line-shape only; docs/how-it-works.md doesn't specify line shape).
