# S2 Implementation Notes — Rename cmd/stagehand→cmd/stagecoach and pkg/stagehand→pkg/stagecoach

> Scope: P1.M1.T1.S2 — `git mv` the two directories. ⚠️ **CONTRACT CORRECTION**: S1's module-prefix-only
> sed left the SUBDIR in 3 import/build-path references unchanged (`stagecoach/{pkg,cmd}/stagehand`), so a
> literal `git mv`-only S2 would break the build on import/dir mismatch (not "only package declarations"
> as OUTPUT #4 claims) and orphan the production import in default_action.go. S2 must ALSO sed those 3
> references so they match the renamed dirs. Verified 2026-07-07.

## 0. S1 state + the gap (confirmed empirically)

- `go.mod` → `module github.com/dustin/stagecoach`. **S1 landed the module rename.**
- S1's sed was `s|github.com/dustin/stagehand|github.com/dustin/stagecoach|g` — MODULE PREFIX ONLY. The
  SUBDIR portion of import paths (`/pkg/stagehand`, `/cmd/stagehand`) was NOT changed (the pattern matches
  only the `github.com/dustin/stagehand` prefix; the trailing `/pkg/stagehand` contains `stagehand` but
  not the full prefix, so it's untouched).
- Empirical proof — the 3 references that STILL say `stagecoach/<subdir>/stagehand`:
  - `internal/cmd/default_action.go:21` — `"github.com/dustin/stagecoach/pkg/stagehand"` (PRODUCTION import)
  - `internal/signal/signal_integration_test.go:167` — `exec.Command("go","build","-o",out,"github.com/dustin/stagecoach/cmd/stagehand")` (test build path)
  - `internal/e2e/harness_test.go:65` — `"github.com/dustin/stagecoach/cmd/stagehand"` (test import)

## 1. Why "git mv only" is insufficient (the contract's OUTPUT #4 is unachievable as-written)

The contract OUTPUT #4 states: "The import paths in .go files already point to `.../cmd/stagecoach` and
`.../pkg/stagecoach` from S1. `go build ./...` should still fail ONLY because package declarations don't
match the directory name yet." This premise is **false** given S1's actual sed: the imports point to
`.../cmd/stagehand` / `.../pkg/stagehand` (subdir unchanged).

If S2 does ONLY `git mv`:
- `pkg/stagehand/` → `pkg/stagecoach/`, but `default_action.go:21` still imports `stagecoach/pkg/stagehand`
  → Go looks for `pkg/stagehand/` (gone) → **build breaks on import/dir mismatch** (NOT just package decls).
- The production import #1 is an ORPHAN: S3 renames package DECLARATIONS (not import paths); S4 updates
  TEST build paths (the 2 test refs, maybe) but not the production import. So nobody fixes #1 → build
  stays broken past S3/S4.

## 2. The corrected S2 — git mv BOTH + sed the 3 import/build-path subdir references

The 3 references are the direct import-path SHADOW of the renamed directories — the subdir in an import
path MUST match the directory it resolves to. Updating them is a natural, inseparable part of renaming
the dirs (and the only way to achieve OUTPUT #4's end state: "build fails ONLY on package declarations,
fixed in S3").

```bash
# 1. Rename the directories (preserve history via git mv).
git mv cmd/stagehand cmd/stagecoach
git mv pkg/stagehand pkg/stagecoach

# 2. Fix the 3 import/build-path subdir references S1's module-prefix sed left behind.
sed -i 's|stagecoach/pkg/stagehand|stagecoach/pkg/stagecoach|g' internal/cmd/default_action.go
sed -i 's|stagecoach/cmd/stagehand|stagecoach/cmd/stagecoach|g' internal/signal/signal_integration_test.go internal/e2e/harness_test.go
```

After this: dirs are `cmd/stagecoach/` + `pkg/stagecoach/`; the 3 references point to `stagecoach/{cmd,pkg}/stagecoach`
(matching the dirs). `go build ./...` then fails ONLY because the files inside the renamed dirs still
declare `package stagehand` (and are named stagehand.go) — which is exactly S3's job (OUTPUT #4 achieved).

## 3. What S2 does NOT do

- NOT the package declarations (`package stagehand` → `package stagecoach`) — S3.
- NOT the file renames (stagehand.go → stagecoach.go) — S3.
- NOT the Makefile/.goreleaser/CI build paths (`./cmd/stagehand` → `./cmd/stagecoach`) — P1.M3.T1.
- NOT Go identifiers / env vars / strings — P1.M1.T2 / P1.M2.
- NOT the go.mod module path (S1 — landed) or the module-prefix import sweep (S1 — landed).
- NOT PRD.md / tasks.json / prd_snapshot.md / plan/*.

## 4. The gate for S2

S2's gate is NOT `go build ./...` green — the build is EXPECTED to fail after S2 (package declarations
don't match the renamed dirs yet; S3 fixes that). S2's gate:
- `cmd/stagecoach/` and `pkg/stagecoach/` exist (git-tracked as renames).
- `cmd/stagehand/` and `pkg/stagehand/` NO LONGER exist.
- ZERO remaining `stagecoach/pkg/stagehand` / `stagecoach/cmd/stagehand` references (the 3 are fixed).
- `git status` shows the moves as renames (R) not add+delete.
- `go build ./...` fails ONLY on `package stagehand` vs dir `stagecoach` (S3's domain) — NOT on import/dir
  resolution (no "no package in import" / "directory not found" for pkg/cmd).

## 5. Sources

- `architecture/rename_surface_map.md` §1.3 (the dir renames) + line 36 (the `cmd/stagehand` import ref).
- `P1M1T1S1/PRP.md` (S1 — module-prefix sed; confirms subdir left unchanged by its sed pattern).
- Empirical grep (the 3 references at default_action.go:21 / signal_integration_test.go:167 / harness_test.go:65).
