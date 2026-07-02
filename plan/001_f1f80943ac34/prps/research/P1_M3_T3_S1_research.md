# Research — P1.M3.T3.S1: internal/git/diff.go — StagedDiff(cfg) (string, error)

All behaviors below verified empirically on the host (git 2.54.0) by driving
the REAL git binary in temp repos, exactly as `internal/git/git.go`'s
`(g *Git) run(args ...string)` does. Baseline `go test ./internal/git/` is
GREEN (S1 git.go + S1 git_test.go + S2 gittestutil_test.go + T2 plumbing all
shipped).

## 0. Contract (verbatim from work item)
- INPUT: `git.Git` (P1.M3.T1.S1 — DONE/shipped): `New(dir)`, unexported
  `run(args...) (string,error)`, typed `*ExitError{Args,Code,Stderr}`. Plus a
  small `cfg` carrying two scalar ints: `MaxMdLines` (default 100) and
  `MaxDiffBytes` (default 300000). NOTE: the M5 `config.Config` package does
  NOT exist yet (M5 is after M3), so `cfg` MUST be a small struct defined
  WITHIN `package git` (mirrors plan_overview §2.2 "Builders take scalar
  settings / a small prompt.Settings"). Decided name: `DiffSettings{ MaxMdLines int;
  MaxDiffBytes int }` — field names ALIGNED with the future M5 `Config` fields
  so M6 bridges `config.Config → git.DiffSettings` field-for-field.
- LOGIC (FR1-FR5): md_files = `git diff --cached --name-only -- '*.md' '*.markdown'`;
  per md file `git diff --cached -- <file>` capped at MaxMdLines lines (head -n,
  PER-FILE); other_diff = single `git diff --cached -- <exclusions>` capped at
  MaxDiffBytes TOTAL bytes (head -c); concatenate markdown_diff + other_diff in
  that order; return ("", nil) — NOT an error — when nothing staged.
- OUTPUT: diff payload string consumed by prompt.AssemblePayload (P1.M4.T1.S3).
- RESEARCH: reference_impl.md §1; external_deps.md §D; decisions.md §9.

## 1. ★ CRITICAL PATHSPEC GOTCHA ★ (`:!.md` does NOT exclude `README.md`)

The contract pathspec for other_diff is EXACTLY (verbatim, no `*` on md):
```
git diff --cached -- ':!*.lock' ':!package-lock.json' ':!pnpm-lock.yaml' \
  ':!yarn.lock' ':!*.snap' ':!*.map' ':!vendor/*' ':!.md' ':!.markdown'
```
Empirically (git 2.54.0), the `:!.md` / `:!.markdown` (NO star) tokens DO NOT
exclude `.md`/`.markdown` files — git treats a pathspec WITHOUT a wildcard as a
LITERAL/PREFIX match (matches a path named exactly `.md` or starting `.md/`),
NOT a suffix match. So `README.md`, `doc.md`, etc. are NOT excluded.

| pathspec form | excludes `doc.md`? |
|---|---|
| `:!.md`        (contract / reference, NO star) | **NO** — doc.md STILL appears |
| `:!*.md`       (glob, WITH star)               | YES — correctly excluded |

Verified (`git diff --cached --name-only -- ':!.md' ':!.markdown'` on a repo
with doc.md + main.go + x.lock → returns doc.md, main.go, x.lock; md NOT removed).
Only `:!*.md` removes them.

**Consequence:** with the contract's literal pathspec, markdown content appears
in BOTH the per-file `markdown_diff` AND the `other_diff`. This is FAITHFUL to
the proven `commit-pi` reference (reference_impl.md §1 uses `:!.md` verbatim)
and to the work item's pinned pathspec string. It is a LATENT quirk of the
reference, NOT a bug to "fix": do NOT change `:!.md`→`:!*.md` without explicit
sign-off, and do NOT write a test asserting "md excluded from other_diff" (it
would fail). The contract MOCKING scenarios are compatible with this: they
require `.lock/.snap/.map/vendor/*` excluded (those forms DO work — see §2) and
"md+other present, concatenated md-first" (md-first ordering holds regardless).

## 2. Verified exclusion behaviors (the forms that DO work)
With the full contract pathspec on a repo staging a.lock, b.snap, c.map,
vendor/lib.go, package-lock.json, pnpm-lock.yaml, yarn.lock, main.go, doc.md:
- other_diff name-only = `doc.md`, `main.go` ONLY.
- `.lock`, `.snap`, `.map`, `vendor/*`, `package-lock.json`, `pnpm-lock.yaml`,
  `yarn.lock` are ALL correctly EXCLUDED (grep for lock|snap|map|vendor → none).
  (`:!*.lock`/`:!*.snap`/`:!*.map` glob-match; `:!vendor/*` matches the dir;
  `:!package-lock.json` etc. are exact-filename matches.)
- `doc.md` LEAKS (§1). Expected & faithful.

## 3. Verified cap semantics
- Per-file md diff, raw line count for a 250-line added .md = 257 lines (preamble
  `diff --git`/`index`/`---`/`+++`/`@@` + 250 `+` content + ...). `| head -n N`
  → exactly N lines. ✓ Go equivalent `capLines`: count `'\n'` BYTES, return
  `s[:i+1]` after the Nth newline (inclusive); if fewer than N newlines, return
  whole `s`. This matches `head -n` exactly (incl. a final partial line).
- Byte cap: `| head -c N` → first N bytes (cuts at a byte boundary, may split a
  UTF-8 rune — acceptable, matches `head -c`). Go: `if len(s) > N { return s[:N] }`.
- Nothing staged: both `git diff --cached --name-only -- '*.md' '*.markdown`
  and `git diff --cached -- <exclusions>` print EMPTY stdout, exit 0 (NOT an
  error). So StagedDiff naturally returns ("", nil). ✓
- `git diff --cached` (no `--quiet`) exits 0 even when empty; StagedDiff's
  `g.run` calls therefore succeed on an empty index. Non-zero exit (e.g.
  not-a-repo) is the only error path → surface the typed `*ExitError`.

## 4. Cap scope (per-FILE vs TOTAL) — precise
- `markdown_diff`: cap is PER-FILE. Each `git diff --cached -- <file>` output is
  independently `capLines(_, MaxMdLines)`, THEN the per-file results are
  concatenated. (NOT a total line cap across all md files.)
- `other_diff`: cap is TOTAL bytes on the WHOLE single-command output:
  `capBytes(git diff --cached -- <exclusions>, MaxDiffBytes)`.
- The byte cap applies ONLY to other_diff, NOT to markdown_diff, NOT to the
  concatenated result. The line cap applies ONLY to markdown_diff per-file.
- Concatenation = plain `markdown_diff + other_diff` (NO separator; per-file and
  other diffs already end in `\n` from git). Matches reference `diff = markdown_diff + other_diff`.

## 5. md_files parsing
`git diff --cached --name-only -- '*.md' '*.markdown'` returns one path per line
(trailing `\n`). Parse by splitting on `\n`, skipping empty elements (the
trailing empty from the final newline). Filenames with embedded newlines are a
pathological edge the reference also ignores (line-based parsing); preserve
leading/trailing SPACES in names (only strip `\r` defensively, do NOT TrimSpace
a filename). `--` separates the pathspec; the `*.md`/`*.markdown` globs are
passed as literal args (no shell — PRD §19).

## 6. Defaults / zero-value handling (defensive design decision)
DiffSettings is a plain value struct. The contract pins defaults MaxMdLines=100,
MaxDiffBytes=300000 (same values M5 Config.Default() will carry). To keep M3
self-contained + testable WITHOUT M5, and to prevent a zero-value footgun
(MaxMdLines=0 would cap to zero lines), StagedDiff CLAMPS non-positive values to
the contract defaults: `if cfg.MaxMdLines <= 0 { cfg.MaxMdLines = 100 }`;
`if cfg.MaxDiffBytes <= 0 { cfg.MaxDiffBytes = 300000 }`. cfg is passed BY VALUE
so the clamp mutates only the local copy.

## 7. Test design (maps 1:1 to the 5 contract MOCKING scenarios), all via S2 harness + REAL git
1. `TestStagedDiff_MarkdownCappedPerFile`: stage ONE .md with many changed lines
   (>MaxMdLines); set a SMALL MaxMdLines (e.g. 5) for determinism + a large
   MaxDiffBytes so the byte cap doesn't bite. Compute rawMdDiff =
   `g.run("diff","--cached","--",file)`. Assert `len(lines(rawMdDiff)) > 5` (cap
   matters) AND `strings.HasPrefix(got, capLines(rawMdDiff, 5))` (the per-file md
   diff is capped to 5 lines and placed FIRST). Proves head -n per-file.
2. `TestStagedDiff_OtherCappedTotalBytes`: stage a LARGE non-md file (e.g. 10KB);
   set a SMALL MaxDiffBytes (e.g. 64) + MaxMdLines=100. md_files empty → result
   = other_diff only. Assert `len(got) <= 64` and `got == capBytes(rawOther, 64)`.
   Proves head -c total.
3. `TestStagedDiff_ExcludesLockSnapMapVendor`: stage a.lock, b.snap, c.map,
   vendor/v.go, package-lock.json, main.go; MaxDiffBytes large. Assert the result
   CONTAINS main.go's diff but does NOT contain the .lock/.snap/.map/vendor/
   package-lock content (strings.Contains negative/positive). Proves exclusions.
4. `TestStagedDiff_MdBeforeOther`: stage a SMALL doc.md + main.go; MaxMdLines=100
   (so the small md diff isn't truncated). Compute mdDiff =
   `g.run("diff","--cached","--","doc.md")` and goDiff likewise. Assert
   `strings.HasPrefix(got, mdDiff)` (md-first) AND `strings.Contains(got, goDiff)`
   (other present). Proves md+other present, md-first concatenation.
5. `TestStagedDiff_NothingStaged`: newTempRepo (unborn, nothing staged). Assert
   `got, err := g.StagedDiff(DiffSettings{})` → `err == nil` AND `got == ""`.
   Proves the ("", nil) non-error contract.

## 8. Dependency / scope discipline (anti-regression)
- Depends on S1 (DONE) + S2 harness (DONE) + T2 plumbing (DONE, unrelated) ONLY.
  Uses `g.run` + `errors.As(&ee)` into `*ExitError` only when surfacing errors.
- ONE NEW file `internal/git/diff.go` + ONE NEW file `internal/git/diff_test.go`
  (white-box `package git`, stdlib `testing` ONLY, drives the REAL binary via the
  S2 harness newTempRepo/writeFileStage).
- DO NOT touch git.go/git_test.go/gittestutil_test.go/plumbing.go/plumbing_test.go,
  main.go, Makefile, go.mod, go.sum, internal/ui, internal/provider. DO NOT run
  `go mod tidy`. No go-git, no testify. Do NOT create M5 config (out of scope).
- DOCS = Mode A: godoc on `DiffSettings` and `StagedDiff` citing PRD §22.3/§19,
  reference_impl.md §1, external_deps.md §D, decisions.md §9, FR1-FR5. No README.

## 9. Validation gates (verified working on host)
- `go build ./internal/git/` → 0 (diff.go added; non-test files compile).
- `go vet ./internal/git/` → clean (compiles _test.go incl. diff_test.go).
- `test -z "$(gofmt -l internal/git/)"` → empty.
- `go test ./internal/git/` → PASS (diff_test.go + existing S1/S2/T2 tests).
- `go test ./...` → whole-module green.
