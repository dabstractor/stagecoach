# Bug Fix Requirements

## Overview

Testing performed against PRD §9.28 (FR-C1–C5, G25) — the v2.9 **chrome-disable** feature (backlog item P1). This is a documentation + verification discipline layered on already-shipped provider manifests: it adds CHROME-DISABLE notes to all 7 built-in providers, mirrors them in `providers/*.toml`, adds contract assertions, and extends changeset-level docs.

**Overall quality assessment: GOOD.** The core chrome-disable feature is correctly and completely implemented:
- All 7 built-in providers in `internal/provider/builtin.go` carry CHROME-DISABLE notes (verified count = 7).
- All 7 `providers/*.toml` reference files carry mirroring CHROME-DISABLE notes (verified count = 7/7).
- The notes accurately reflect the actual `bare_flags` in each manifest (cross-checked against the captured `--help` catalog in `plan/001_.../architecture/external_deps.md`).
- `TestBuiltinManifests_ChromeDisableContract` asserts FR-C2 (pi/claude chrome-disable flag presence) and FR-C4(b) (read-only-constraint flag presence for the 5 constrained providers); all tests pass.
- The `Chrome-disable` column was added to the `docs/providers.md` provider table; the "Chrome is a separate axis" bullet was added to the asymmetry section; `docs/how-it-works.md`, `docs/README.md`, and `README.md` all carry accurate chrome mentions.
- FR-C2 verification duty was honored: no chrome-disable switch exposed by any agent CLI was left unset in `bare_flags` (the 5 read-only-constrained providers genuinely expose none; pi/claude set every one they offer).

The two issues below are **documentation drift** in files directly modified by this task's commits (`b8f081d` added the Chrome-disable column to the providers table; `71e57c7a` rewrote the how-it-works safety paragraph). They are not chrome-disable defects per se — the chrome content is accurate — but the surrounding rows/lines that the task re-touched were left stale, which undermines the documentation-consistency goal of a "Mode B" docs-sync task (P1.M2 exists precisely to "prevent a coherent delta from shipping with stale overview docs").

## Major Issues (Should Fix)

### Issue 1: docs/providers.md opencode Delivery column says "positional" but the binary uses "stdin"
**Severity**: Major
**PRD Reference**: PRD §12.6 (opencode manifest); FR-C5 documentation-honesty duty; docs/README.md promise that "the `docs/` directory tracks the shipped binary … the binary is authoritative."
**Expected Behavior**: The `Delivery` column in the 7-provider table in `docs/providers.md` should mirror the shipped binary's `prompt_delivery` value for every provider. The table's own header (and every `providers/*.toml` file) states the reference docs mirror the compiled-in manifest "BYTE-FOR-BYTE." For opencode the binary's value is `stdin` (revised from the PRD's "positional" in commit `010ecee`, documented in `builtin.go` and `providers/opencode.toml` with rationale: stdin avoids the 128 KB `MAX_ARG_STRLEN` ceiling on ~300 KB diffs).
**Actual Behavior**: The opencode row reports `positional`:
```
| `opencode` | positional | (none) | `-m` | (user must set) | (prepended) | Read-only constraint (`run` subcommand) | … |
```
Verified directly against the binary:
```
$ stagecoach providers show opencode | grep prompt_delivery
prompt_delivery = 'stdin'
```
A full cross-check of all 7 providers confirms opencode is the **only** mismatch (pi/claude/codex/agy/qwen-code = stdin✓; cursor = positional✓; opencode = stdin in binary but `positional` in docs ✗).

This matters: the whole point of revising opencode to `stdin` was to avoid arg-length truncation on large diffs. Documenting `positional` hides that behavior and would mislead anyone debugging an opencode large-diff failure, or building tooling that assumes positional delivery. It also directly contradicts the file's stated byte-faithfulness contract.
**Steps to Reproduce**:
1. `go build -o /tmp/stagecoach ./cmd/stagecoach`
2. `/tmp/stagecoach providers show opencode | grep prompt_delivery` → prints `prompt_delivery = 'stdin'`
3. Open `docs/providers.md`, find the opencode row in "## The 7 built-in providers" table → `Delivery` column reads `positional`.
4. `git blame -L 82,82 docs/providers.md` → shows commit `b8f081d` ("Add Chrome-disable column to providers.md table") last touched this exact row (2026-07-13), i.e. the chrome-disable task re-wrote this row to add the Chrome-disable column and carried the stale delivery value forward.
**Suggested Fix**: In `docs/providers.md`, change the opencode row's Delivery cell from `positional` to `stdin` so it matches the binary, `builtin.go` (`PromptDelivery: strPtr("stdin")`), and `providers/opencode.toml` (`prompt_delivery = "stdin"`). The "Rendered command" guidance elsewhere in the file and in `providers/opencode.toml` already correctly shows stdin piping (`< "<sys>\n\n<user payload>"`).

## Minor Issues (Nice to Fix)

### Issue 2: docs/how-it-works.md safety paragraph categorizes only 4 of 7 providers ("codex, cursor" omits agy/qwen-code/opencode)
**Severity**: Minor
**PRD Reference**: PRD §12.7.1 (tools-disable asymmetry categories); FR-C5 documentation completeness.
**Expected Behavior**: The "### Safety invariant" paragraph makes a universal claim — "Every built-in manifest constrains the agent to a read-only mode" — and then partitions providers into two categories. The read-only-constraint category now contains **5** providers (codex, cursor, agy, qwen-code, opencode), per the same file's provider table and the `docs/providers.md` Chrome-disable column.
**Actual Behavior**: The paragraph names only two:
```
… either via explicit tool-disable flags (pi, claude) or read-only constraint flags (codex, cursor). …
```
A reader sees a claim about "every" manifest but only 4 of 7 providers named, leaving opencode/agy/qwen-code uncategorized. Since the sentence reads as exhaustive (not "e.g."), it implies the other three don't fit either bucket, which is incorrect — all three are read-only-constrained (opencode by design via `run`; agy via `--mode plan`; qwen-code via `--approval-mode default`).
`git blame -L 197,197 docs/how-it-works.md` shows commit `71e57c7a` (the chrome-disable docs commit) rewrote this entire line when appending the chrome-less sentence, so the stale parenthetical was carried through the rewrite rather than refreshed.
**Steps to Reproduce**: Open `docs/how-it-works.md`, read the "### Safety invariant" paragraph (line ~197). Cross-reference the 5 read-only-constrained providers in the `docs/providers.md` "Tool-disable approach" column.
**Suggested Fix**: Expand the parenthetical to be either exhaustive or explicitly exemplary, e.g. `read-only constraint flags (codex, cursor, agy, qwen-code; opencode's \`run\` is read-only by design)`, or rephrase as `read-only constraint flags (e.g. codex, cursor)`. The latter is the smaller change; the former is more precise.

## Testing Summary
- Total tests performed: full `go test ./...` (all PASS); targeted provider/manifest/decode-parity/chrome-contract suite; binary `providers show` cross-check for all 7 providers' `prompt_delivery`; CHROME-DISABLE note presence count in `builtin.go` (7) and `providers/*.toml` (7/7); git-blame trace of the two modified doc lines; cross-reference of chrome claims vs the captured `--help` catalog (`plan/001_.../architecture/external_deps.md`).
- Passing: all unit/integration tests (the chrome-disable feature itself is functionally correct and complete).
- Failing: 0 (no test failures — both findings are documentation drift, which the test suite does not cover).
- Areas with good coverage: chrome-disable notes (all 7 providers, both locations); FR-C2 flag-verification (no missed chrome switches); ChromeDisableContract test assertions; Chrome-disable table column accuracy; asymmetry bullet; cross-doc chrome mentions in README/how-it-works/README-index.
- Areas needing more attention: cross-consistency between the `docs/providers.md` provider table and the actual binary field values (the opencode delivery drift would be caught by a "docs table == `providers show`" consistency check, which does not exist); completeness of provider lists in prose that makes universal ("every") claims.
