# Research: final grep audit — zero stagehand references (P1.M5.T2.S1)

> Scope: the whole-repo verification that the stagehand→stagecoach rename is complete (PRD h2.30). The
> contract's literal audit command is STALE (its exclusion set is too narrow, same defect the sibling
> P1.M5.T1.S1 found). This file records the TRUE state and the corrected audit.

## 1. The contract's literal command CANNOT return 0 (verified live)

Contract: `git grep -i 'stagehand' | grep -v 'plan/012_963e3918ec08/architecture/' | wc -l` must == 0.

Live run today returns ~2654 lines, because the exclusion (`architecture/` ONLY) misses three LEGITIMATE
categories that are NOT rename failures:

| zone | what | line count (today) | status |
|---|---|---|---|
| **B** | `plan/012_963e3918ec08/` NON-architecture files (the rename's OWN PRPs + research) | ~2470 | PRESERVE — these ARE the rename documentation ("rename stagehand.* → stagecoach.*"); renaming them corrupts them (stagehand→stagecoach ⇒ stagecoach→stagecoach). The sibling P1.M5.T1.S1 prunes the WHOLE plan/012 tree for the same reason. |
| **C** | `plan/001..011/**/tasks.json` | ~180 lines / ~19 files | FORBIDDEN — orchestrator-owned ("NEVER MODIFY **/tasks.json"); .json is NOT in the rename's text surface (.md/.go/.toml/.txt/.yaml/.yml/Makefile); flagged by P1.M5.T1.S1 for the orchestrator. |
| **PRD** | `PRD.md:2366` | 1 | LEGITIMATE + READ-ONLY — the rename directive itself ("this project was originally named 'stagehand'…"). PRD.md is human-owned; it MUST name the old product. |

The contract's `architecture/`-only exclusion was written before the rename's own PRPs existed (plan/012/*
was created BY this rename changeset). Its SPIRIT ("exclude the rename's own docs") requires excluding the
**whole plan/012 tree**, not just architecture/ — exactly as P1.M5.T1.S1 already determined.

## 2. ZONE A — the REAL audit targets (3 stragglers + 1 legit exception)

`git grep -i 'stagehand' | grep -v 'plan/'` = exactly 4 files, 1 line each. These are the audit's actual
work (the non-plan tracked surface):

| file:line | content | classification | action |
|---|---|---|---|
| `PRD.md:2366` | `…originally named "stagehand" and has been renamed…` | LEGITIMATE (rename directive) + READ-ONLY (PRD is human-owned) | EXCLUDE from gate (document as the one accepted PRD ref) |
| `internal/git/git.go:390` | `…to locate .git/STAGEHAND_EDITMSG…` (a COMMENT) | STALE STRAGGLER — the actual constant is `STAGECOACH_EDITMSG` (`internal/generate/finalize.go:78`: `filepath.Join(gitDir, "STAGECOACH_EDITMSG")`) | FIX comment: STAGEHAND_EDITMSG → STAGECOACH_EDITMSG |
| `.goreleaser.yaml:1` | `# .goreleaser.yaml — Stagehand release config …` (a COMMENT) | STALE STRAGGLER — `project_name: stagecoach` (line 12), `builds[0].id: stagecoach`, `main: ./cmd/stagecoach` are all already renamed (P1.M3.T1.S2); only the header comment missed | FIX comment: Stagehand → Stagecoach |
| `.golangci.yml:40` | `- path: pkg/stagehand/stagehand_test.go` (lint EXCLUSION path) | STALE STRAGGLER — that file NO LONGER EXISTS; it is now `pkg/stagecoach/stagecoach_test.go` (verified present). The exclusion currently matches nothing → errcheck/unused findings in that test file would no longer be suppressed (latent CI lint issue). | FIX path: pkg/stagehand/stagehand_test.go → pkg/stagecoach/stagecoach_test.go |

**Why these were missed by M1–M4:** each is a comment/path/identifier substring, not an import/identifier
the structural rename passes (M1 Go, M2 config, M3 build) keyed on. The audit (this task) is exactly the
"if any references remain, fix them" safety net the contract asks for.

## 3. The stale-dir / stale-file checks are CLEAN (verified)

- `cmd/stagehand/` — ABSENT (now `cmd/stagecoach/`). ✓
- `pkg/stagehand/` — ABSENT (now `pkg/stagecoach/`). ✓
- `stagehand.go` / `stagehand_test.go` anywhere outside plan/012/architecture — NONE (`git ls-files | grep
  -i 'stagehand\.go$'` empty). ✓
- `go.mod` module path = `github.com/dustin/stagecoach`. ✓
- The repo CHECKOUT directory is still `/home/dustin/projects/stagehand`, but that is a LOCAL working-tree
  path, NOT a tracked file — `git grep` cannot see it and it ships in no artifact. Out of scope (noting it
  only so the implementer isn't confused by `pwd`).

## 4. The project's OWN verification gate (rename_surface_map.md Gate 5)

```
grep -ri 'stagehand' --include='*.go' --include='*.md' --include='*.toml' --include='*.yaml' \
  --include='*.yml' --include='Makefile' . | grep -v '.git/' | wc -l == 0
```

This was the M1–M4 per-layer target. It uses `--include` for the shipped-text extensions (so `.json`/
tasks.json is OUT by design — corroborating that tasks.json is not a rename-failure). It predates plan/012
(the rename changeset), so it does not exclude plan/012; today it would also catch plan/012's rename docs.
Like the contract's command, it needs the plan/012 + PRD-directive exceptions added for the FINAL audit.

## 5. The corrected audit (the deliverable's verification gate)

After fixing the 3 stragglers (§2), the whole shipped-product surface is clean. The corrected audit
excludes the three legitimate categories (plan/012 rename docs, tasks.json orchestrator files, PRD.md
rename directive) and MUST return 0:

```bash
git grep -i 'stagehand' \
  | grep -v 'plan/012_963e3918ec08/' \
  | grep -v 'tasks\.json' \
  | grep -v '^PRD\.md:' \
  | wc -l
# Expected AFTER the 3 fixes: 0.
```

And the documented exceptions (each must be accounted for, not zero):
- `git grep -i 'stagehand' -- 'plan/012_963e3918ec08/' | wc -l` → >0 (rename docs, PRESERVED).
- `git grep -i 'stagehand' | grep 'tasks\.json' | wc -l` → >0 (orchestrator-owned, FORBIDDEN).
- `git grep -i 'stagehand' -- PRD.md | wc -l` → 1 (the rename directive line, read-only).

`git grep` searches only TRACKED files (it cannot see `.git/` internals or gitignored build output), so no
`.git/` guard is needed (unlike the `grep -ri` form). Plain `grep -ri .` is an acceptable alternative that
DOES need `grep -v '.git/'` — use `git grep` to match the contract's tool and avoid that.

## 6. Fix safety (all three are mechanical, low-risk)

- `internal/git/git.go:390` — COMMENT ONLY. The runtime constant is already `STAGECOACH_EDITMSG`
  (finalize.go:78). `go build`/`go test`/`go vet` unaffected (comments aren't compiled).
- `.goreleaser.yaml:1` — COMMENT ONLY. `project_name: stagecoach` etc. are correct. Validate with
  `goreleaser check` (a no-publish schema validation; the file's own header documents it).
- `.golangci.yml:40` — a lint EXCLUSION path. The target file exists. This is a real correctness fix:
  after it, golangci-lint re-applies the errcheck+unused exclusion to the renamed test file. NOTE CI pins
  golangci-lint v1.61 (per the file's schema note); validate locally with v1.61 if available, else `go vet`
  as a smoke check (the path fix is a plain string, low risk).

## 7. NOT in scope / do NOT touch

- `PRD.md` (human-owned, READ-ONLY) — the directive line is the legitimate exception.
- `tasks.json` anywhere (orchestrator-owned, FORBIDDEN).
- `plan/012_963e3918ec08/` (the rename changeset — PRESERVE; renaming its PRPs corrupts them).
- `plan/001..011` non-tasks.json text (already clean — verified 0 in-scope residue; P1.M5.T1.S1 certifies).
- The local checkout directory name `/home/dustin/projects/stagehand` (untracked; ships in nothing).
- Go source beyond the one git.go:390 comment (M1 already renamed identifiers/imports).
