# Research Notes — P1.M1.T5.S1 (work-description docs sync)

## Task
Sync docs to reflect the 4 work-description bug fixes (BUG-001/002/005/006 = T1–T4).
Authority = spec `spec/01-product.md` FR-W3 / FR-W5 (code was wrong, spec is right).

## The 4 fixes (already implemented / being implemented in parallel)
- **T1 / BUG-001 (FR-W3)**: a READ of a non-staged path no longer silently becomes the commit
  subject. New 3-way split in `RunWorkDescription`'s `len(paths)==0` branch; new helper
  `buildNonStagedReadAnswer` emits the note and *continues the loop* (round-cap bounded, NOT bypassed).
- **T2 / BUG-002 (FR-W5)**: a fully-read (cursor-exhausted) file now emits an explicit
  `— end of diff (all parts shown).` note instead of an empty body.
- **T3 / BUG-005 (FR-W5)**: `nextChunk`/`chunkCount` anchor chunk boundaries to `@@` hunk edges
  (shared `anchorToHunkEdge`) so a change is never split mid-hunk; oversize single hunk → line cut.
- **T4 / BUG-006 (FR-W5)**: "part i of N" numerator is now `utf8.RuneCountInString(diff[:offset])`,
  consistent with `chunkRuneBudget()` (was byte÷rune mismatch — wrong for multibyte UTF-8).

## Doc gap (the finding)
`docs/how-it-works.md` work-description section (lines 358–382) is HIGH-LEVEL and correct on what it
covers (description-first inversion, `--work-description`, `session_mode="append"`, bounded rounds,
no-READ = message, auto-stage, no multi-turn cascade). BUT it is **silent** on the three behaviors this
task names — they were never described at the conceptual level:
1. **non-staged READ → note + continue** (FR-W3) — the BUG-001 behavior.
2. **chunked reads: "part i of N" label + end-of-diff terminal note** (FR-W5) — BUG-002/006.
3. **chunk boundaries hug `@@` hunk edges** (FR-W5) — BUG-005.

=> Not "verify, no change". The section needs a focused **Read/answer protocol** paragraph covering
chunking + the non-staged note. Prose must match the exact note strings below.

## README.md finding
README has **zero** `--work-description` mentions (grep confirmed). Features table (lines 53–76) has no
row for it; "More options" (191–210) has no example. => discoverability gap. Recommended: add a Features
row + a More-options example line. Optional but in-scope (task title names README).

## Already-accurate sibling docs (NO change needed — verified)
- `docs/cli.md:45-46` — flags table already says "let the model read staged file diffs on demand via
  `READ <path>`". Accurate.
- `docs/configuration.md:114,148` — `work_desc_read_rounds = 5` documented. Accurate.

## Exact code strings (prose must match these — the accuracy contract)
`internal/generate/workdesc.go`:
- L303 `buildNonStagedReadAnswer`: `` `%s is not in the staged changes.\n\n` ``  → "`<path>` is not in the staged changes."
- L379 `buildReadAnswer` (staged-but-exhausted): `` `%s is not in the staged changes (or has no further diff).\n\n` ``
- L386 end-of-diff: `` `%s — end of diff (all parts shown).\n\n` ``
- L402 part label: `` `%s — part %d of %d; READ %s again for the next part:\n%s\n\n` ``
- L36 `readChunkTokenCap = 16000`; L452 `chunkRuneBudget() int { return readChunkTokenCap * 4 }` → 64000 runes.

## Spec numbers (authoritative)
- per-call read cap ≈ 16K tokens internal = 64000 runes (chunkRuneBudget).
- `work_desc_read_rounds` default 5 (config.go:103, configuration.md:114).

## Validation gate
- `.github/workflows/docs.yml`: `mkdocs build --strict` (broken link/anchor → ERROR). Runs on push to
  main touching `docs/**` / `README.md`. Can run locally: `pip install -r requirements-docs.txt && mkdocs build --strict`.
- `.markdownlint.json`: MD013 (line-len), MD033 (inline HTML), MD060 OFF. All other default rules ON
  (MD001 heading order, MD040 fenced-code lang, MD041 first-line H1, etc.).
- No unit test for prose — accuracy = human diff against the code strings above.

## mkdocs nav check
`docs/how-it-works.md` is a standalone page. New content is INSIDE an existing `##` section, so no
`mkdocs.yml` nav change and no new anchor to wire up. The section's existing heading anchor:
`## Work-description mode (description-first, read-on-demand)` →
`#work-description-mode-description-first-read-on-demand` (used by README Features rows elsewhere).