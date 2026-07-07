---
name: "P1.M1.T1.S2 — Rename directories cmd/stagehand→cmd/stagecoach and pkg/stagehand→pkg/stagecoach"
description: |
  `git mv` the two directories (preserve history). ⚠️ CONTRACT CORRECTION: S1's module-prefix-only sed
  left the SUBDIR in 3 import/build-path references unchanged — they now read `stagecoach/pkg/stagehand`
  and `stagecoach/cmd/stagehand` (verified). A literal "git mv only" S2 (as the contract LOGIC says) would
  break the build on import/dir mismatch and orphan the production import in default_action.go (nobody
  else fixes it — S3=package decls, S4=test build paths). So S2 must ALSO sed those 3 references to match
  the renamed dirs — this is the only way to achieve the contract's OUTPUT #4 ("build fails ONLY on
  package declarations, fixed in S3"). After S2: dirs are stagecoach/; the 3 refs point to stagecoach/;
  `go build ./...` fails ONLY because files inside still declare `package stagehand` (S3's job).
---

## Goal

**Feature Goal**: Rename the two Go source directories `cmd/stagehand/` → `cmd/stagecoach/` and
`pkg/stagehand/` → `pkg/stagecoach/` (preserving git history), AND fix the 3 import/build-path references
S1's module-prefix-only sed left pointing at the old subdir names — so the renamed directories are
self-consistent with their import paths and the build fails only on the package-declaration mismatch S3 owns.

**Deliverable** (2 git moves + 3 targeted seds):
1. `git mv cmd/stagehand cmd/stagecoach`
2. `git mv pkg/stagehand pkg/stagecoach`
3. `sed -i 's|stagecoach/pkg/stagehand|stagecoach/pkg/stagecoach|g' internal/cmd/default_action.go`
4. `sed -i 's|stagecoach/cmd/stagehand|stagecoach/cmd/stagecoach|g' internal/signal/signal_integration_test.go internal/e2e/harness_test.go`

**Success Definition**: `cmd/stagecoach/` and `pkg/stagecoach/` exist (git-tracked as renames); the old
dirs are gone; zero remaining `stagecoach/pkg/stagehand` / `stagecoach/cmd/stagehand` references; `git status`
shows renames (R); `go build ./...` fails ONLY on `package stagehand`-in-`stagecoach/`-dir (S3's domain),
NOT on import/dir resolution.

## User Persona

**Target User**: The contributor implementing S3 (package declarations + file renames inside the renamed
dirs) and S4 (binary build-path verification), and the reviewer confirming the structural rename is clean.

**Use Case**: After S1 renamed the module + import prefixes, the directories and their import-path
shadows must follow. S2 is the structural move; S3 finishes the package identities inside.

**Pain Points Addressed**: Closes the gap S1's narrow sed left — without the 3 import-path fixes, the dir
rename would produce a confusing import/dir-mismatch build break (and an orphaned production import).

## Why

- **The directories must follow the module rename.** S1 made the module `stagecoach`; the `cmd/stagehand/`
  and `pkg/stagehand/` directories are the last structural holdouts of the old name. `git mv` preserves
  history (a plain mv would show as delete+add).
- **Contract correction (necessary, not optional).** The contract LOGIC says "no other code changes," but
  its OUTPUT #4 says "import paths already point to `cmd/stagecoach`." Empirically S1's sed changed only
  the module PREFIX, leaving 3 references at `stagecoach/{pkg,cmd}/stagehand` (subdir unchanged). A
  `git mv`-only S2 would (a) break the build on import/dir mismatch, not "only package declarations," and
  (b) orphan the production import in `default_action.go:21` (S3 doesn't touch import paths; S4 owns only
  test build paths). The 3 sed fixes are the import-path SHADOW of the dir rename — inseparable from it.
- **Lowest-risk completion.** Two `git mv`s + three targeted, full-substring seds. No logic change; the
  references merely follow the directories they name.

## What

Two git-tracked directory renames plus three import/build-path string updates (the subdir portion that
S1's module-prefix sed didn't reach). After S2 the directories are `stagecoach/`, the 3 references match,
and the build fails only on the package-declaration mismatch S3 fixes.

### Success Criteria

- [ ] `cmd/stagecoach/` exists; `cmd/stagehand/` does not.
- [ ] `pkg/stagecoach/` exists; `pkg/stagehand/` does not.
- [ ] `git status` shows the 4 files (main.go, stagehand.go, stagehand_test.go + the dir moves) as renames (R), not add+delete.
- [ ] ZERO `stagecoach/pkg/stagehand` or `stagecoach/cmd/stagehand` references remain (grep confirms).
- [ ] The 3 references now read `stagecoach/pkg/stagecoach` / `stagecoach/cmd/stagecoach`.
- [ ] `go build ./...` fails ONLY on `package stagehand` declared in a `stagecoach/` directory (S3's fix) — NOT on "no package in import" / "directory not found".
- [ ] No package declarations, file names, identifiers, env vars, Makefile/.goreleaser/CI paths, or docs changed (S3 / P1.M1.T2 / P1.M2 / P1.M3 / P1.M4 own those).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP names the exact 2 `git mv` commands, the exact 3 references (file:line +
the precise sed pattern), the empirical proof that S1 left them, and the exact expected post-state (build
fails only on package decls). The contract correction (why "git mv only" is insufficient) is documented
with the orphan-import reasoning.

### Documentation & References

```yaml
# MUST READ — the contract correction (why git-mv-only fails)
- docfile: plan/012_963e3918ec08/P1M1T1S2/research/s2_implementation_notes.md
  why: "§0 proves empirically that S1's module-prefix sed left 3 subdir references unchanged (default_action.go:21, signal_integration_test.go:167, harness_test.go:65). §1 explains why git-mv-only breaks the build on import/dir mismatch and orphans the production import. §2 gives the exact corrected commands."
  critical: "The #1 trap is following the contract LOGIC literally ('git mv only, no code changes'): that leaves the 3 references pointing at the old subdir, breaking the build on import resolution (not just package decls) and orphaning default_action.go:21. S2 MUST sed the 3 refs."

- docfile: plan/012_963e3918ec08/architecture/rename_surface_map.md
  why: "§1.3 lists the two dir renames (cmd/stagehand→cmd/stagecoach, pkg/stagehand→pkg/stagecoach); line 36 explicitly calls out the signal_integration_test.go:167 'cmd/stagehand' import reference as a rename target — confirming the subdir-import refs are in scope of the rename (S1's sed didn't reach them)."

- docfile: plan/012_963e3918ec08/P1M1T1S1/PRP.md
  why: "S1's contract: the sed was 's|github.com/dustin/stagehand|github.com/dustin/stagecoach|g' — MODULE PREFIX ONLY. This is why the /pkg/stagehand and /cmd/stagehand SUBDIR portions of import paths survived S1 (the pattern matches only the prefix; the trailing /pkg/stagehand contains 'stagehand' but not the full prefix string). S1 is LANDED (go.mod = stagecoach); S2 consumes its output."

# The files under edit
- file: cmd/stagehand/ → cmd/stagecoach/   # git mv (the directory + main.go inside)
  why: "git mv preserves history. main.go moves with the directory (no separate file move needed)."
- file: pkg/stagehand/ → pkg/stagecoach/   # git mv (stagehand.go + stagehand_test.go inside)
  why: "git mv preserves history. Both .go files move with the directory. (S3 renames the FILES stagehand.go→stagecoach.go and the package declarations — NOT S2.)"
- file: internal/cmd/default_action.go:21
  why: "PRODUCTION import: `\"github.com/dustin/stagecoach/pkg/stagehand\"` → the subdir MUST match the renamed pkg/stagecoach/ dir. sed 's|stagecoach/pkg/stagehand|stagecoach/pkg/stagecoach|g'."
- file: internal/signal/signal_integration_test.go:167
  why: "TEST build-path exec arg: `\"github.com/dustin/stagecoach/cmd/stagehand\"` → must match cmd/stagecoach/. sed 's|stagecoach/cmd/stagehand|stagecoach/cmd/stagecoach|g'."
- file: internal/e2e/harness_test.go:65
  why: "TEST import: `\"github.com/dustin/stagecoach/cmd/stagehand\"` → must match cmd/stagecoach/. Same sed as above."

# Read-only refs (do NOT edit in S2)
- file: go.mod   # S1 LANDED: module github.com/dustin/stagecoach
  why: "The module path S2's import refs resolve against. No edit."
- file: cmd/stagehand/main.go, pkg/stagehand/stagehand.go, pkg/stagehand/stagehand_test.go   # contents
  why: "These move WITH the directories (git mv). Their package declarations (package stagehand) and file names (stagehand.go) are S3's job — S2 does NOT edit file contents or names."

# PRD authority (already in the selected content)
- prd: PRD.md §14 (package layout shows cmd/stagecoach/ + pkg/stagecoach/) + §h2.30 ("all references to 'stagehand' must be replaced with 'stagecoach'").
  why: "The target layout + the global rename mandate."
```

### Current Codebase Tree (relevant slice)

```bash
stagehand/                      # repo dir name unchanged (the rename is in-repo)
├── cmd/
│   ├── stagehand/              # RENAME → stagecoach/  (contains main.go)
│   └── stubagent/              # unchanged
├── pkg/
│   └── stagehand/              # RENAME → stagecoach/  (contains stagehand.go, stagehand_test.go)
└── internal/
    ├── cmd/default_action.go        # EDIT line 21: import subdir stagehand→stagecoach
    ├── signal/signal_integration_test.go  # EDIT line 167: build-path subdir
    └── e2e/harness_test.go          # EDIT line 65: import subdir
```

### Desired Codebase Tree After S2

```bash
stagehand/
├── cmd/
│   ├── stagecoach/             # RENAMED (main.go inside; still 'package main', unchanged name main.go)
│   └── stubagent/
├── pkg/
│   └── stagecoach/             # RENAMED (stagehand.go + stagehand_test.go inside; names + 'package stagehand' UNCHANGED — S3's job)
└── internal/
    ├── cmd/default_action.go        # import → github.com/dustin/stagecoach/pkg/stagecoach
    ├── signal/signal_integration_test.go  # → .../cmd/stagecoach
    └── e2e/harness_test.go          # → .../cmd/stagecoach
# NOTE: the files inside the renamed dirs STILL declare 'package stagehand' and are named stagehand.go —
# that mismatch is the EXPECTED build failure S3 fixes. S2 is dirs + import-path shadow only.
```

| Path | Action | Responsibility |
|---|---|---|
| `cmd/stagehand/` → `cmd/stagecoach/` | git mv | Directory rename (history preserved). |
| `pkg/stagehand/` → `pkg/stagecoach/` | git mv | Directory rename (history preserved). |
| `internal/cmd/default_action.go` | sed (line 21) | Import subdir → stagecoach (the orphan production import). |
| `internal/signal/signal_integration_test.go` | sed (line 167) | Build-path subdir → stagecoach. |
| `internal/e2e/harness_test.go` | sed (line 65) | Import subdir → stagecoach. |

**Explicitly NOT touched**: the package declarations / file names inside the renamed dirs (S3), Go
identifiers (P1.M1.T2), env vars / git config / strings (P1.M2), Makefile / .goreleaser / CI (P1.M3),
docs (P1.M4), `go.mod` (S1 — landed), `PRD.md`, `tasks.json`, `prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & Library Quirks

```bash
# CRITICAL (contract correction — do NOT do "git mv only"): S1's sed was module-prefix-only, leaving 3
# references at stagecoach/{pkg,cmd}/stagehand. A literal "git mv only" S2 breaks the build on import/dir
# mismatch and orphans the production import default_action.go:21 (S3 doesn't fix import paths; S4 owns
# only test build paths). S2 MUST sed the 3 refs. This achieves OUTPUT #4 (build fails only on package
# declarations, which S3 fixes).

# CRITICAL (use git mv, not plain mv): git mv records the move as a rename (R) in git status, preserving
# file history. A plain mv + git add would show as delete+add, losing blame history. Both directories are
# git-tracked (confirmed: git ls-files lists all 3 files).

# CRITICAL (build is EXPECTED to fail after S2): the files inside the renamed dirs still declare
# 'package stagehand' and are named stagehand.go. Go requires the package declaration to match the
# directory (pkg/stagecoach/ must contain 'package stagecoach'). So 'go build ./...' FAILS after S2 —
# this is the EXPECTED state S3 fixes. S2's gate is NOT 'go build green'; it's 'dirs renamed + 3 refs
# fixed + build fails only on package-decl mismatch (not import/dir resolution)'.

# GOTCHA (precise sed patterns): use the FULL substring 'stagecoach/pkg/stagehand' and
# 'stagecoach/cmd/stagehand' as the patterns (NOT bare '/stagehand' — too broad; NOT 'stagehand' alone —
# would hit package decls/identifiers, which are S3/P1.M1.T2). The module prefix is already 'stagecoach'
# (S1), so these full-substring patterns are unambiguous and target exactly the 3 import/build-path refs.

# GOTCHA (don't touch the dir contents): the files move WITH the directories. Do NOT separately move/
# rename main.go, stagehand.go, stagehand_test.go, or edit their 'package' lines — that's S3. S2 is the
# container move + the external import-path shadow.
```

## Implementation Blueprint

### Data models and structure

No data-model change — two directory renames + three string updates. The relevant existing state (unchanged):

```go
// internal/cmd/default_action.go:21 (current, post-S1)
import "github.com/dustin/stagecoach/pkg/stagehand"   // subdir still 'stagehand' — S2 fixes to 'stagecoach'
// internal/signal/signal_integration_test.go:167
exec.Command("go", "build", "-o", out, "github.com/dustin/stagecoach/cmd/stagehand")
// internal/e2e/harness_test.go:65
"github.com/dustin/stagecoach/cmd/stagehand"
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: git mv the directories (preserve history)
  - RUN: git mv cmd/stagehand cmd/stagecoach
  - RUN: git mv pkg/stagehand pkg/stagecoach
  - VERIFY: ls -d cmd/stagecoach pkg/stagecoach → both exist; ls -d cmd/stagehand pkg/stagehand → both gone
  - VERIFY: git status shows R (rename) for the 3 files, not D+A.
  - DO NOT: rename the files inside (main.go, stagehand.go, stagehand_test.go) — S3.

Task 2: fix the 3 import/build-path subdir references (the contract correction)
  - RUN: sed -i 's|stagecoach/pkg/stagehand|stagecoach/pkg/stagecoach|g' internal/cmd/default_action.go
  - RUN: sed -i 's|stagecoach/cmd/stagehand|stagecoach/cmd/stagecoach|g' internal/signal/signal_integration_test.go internal/e2e/harness_test.go
  - VERIFY the 3 lines now read stagecoach:
        grep -n "stagecoach/pkg/stagecoach\|stagecoach/cmd/stagecoach" internal/cmd/default_action.go internal/signal/signal_integration_test.go internal/e2e/harness_test.go
        # → 3 matches (the corrected refs)
  - VERIFY zero old-subdir refs remain:
        grep -rn "stagecoach/pkg/stagehand\|stagecoach/cmd/stagehand" --include="*.go" . | grep -v './.git/'   # → ZERO

Task 3: VALIDATE (S2's gate — build fails ONLY on package-decl mismatch, not import/dir)
  - RUN: go build ./... 2>&1 | head
  - EXPECT: a FAILURE, but ONLY of the form "package stagehand (in pkg/stagecoach/...) doesn't match
    directory" / "found package stagehand @ pkg/stagecoach/stagehand.go" — i.e. the package-declaration
    mismatch S3 fixes. NOT "no package in import / directory not found" (that would mean an import/dir
    mismatch — Task 2 missed a reference).
  - RUN: git status --short   # confirm renames (R) + the 3 edited files (M), nothing unexpected.
  - NOTE: do NOT chase the build to green — S3 renames the package declarations + files to make it green.
    S2's job is dirs + import-path shadow.
```

### Implementation Patterns & Key Details

```bash
# === the complete S2 (2 git moves + 3 seds) ===
git mv cmd/stagehand cmd/stagecoach
git mv pkg/stagehand pkg/stagecoach
sed -i 's|stagecoach/pkg/stagehand|stagecoach/pkg/stagecoach|g' internal/cmd/default_action.go
sed -i 's|stagecoach/cmd/stagehand|stagecoach/cmd/stagecoach|g' internal/signal/signal_integration_test.go internal/e2e/harness_test.go

# === verify the 3 refs are fixed + no old-subdir refs remain ===
grep -rn "stagecoach/pkg/stagehand\|stagecoach/cmd/stagehand" --include="*.go" . | grep -v './.git/' || echo "OK: zero old-subdir refs"

# === the EXPECTED build failure (S3's domain, not a bug) ===
go build ./... 2>&1 | grep -E "package stagehand|doesn't match" | head
# → e.g. "package github.com/dustin/stagecoach/pkg/stagecoach: found package stagehand in pkg/stagecoach/stagehand.go"
# (S3 renames 'package stagehand' → 'package stagecoach' + the file to stagecoach.go to clear this.)
```

### Integration Points

```yaml
DIRECTORIES (git mv, history preserved):
  - cmd/stagehand/ → cmd/stagecoach/   (main.go moves with it)
  - pkg/stagehand/ → pkg/stagecoach/   (stagehand.go + stagehand_test.go move with it)

IMPORT/BUILD-PATH SHADOW (sed — the 3 refs S1's module-prefix sed left):
  - internal/cmd/default_action.go:21          → .../pkg/stagecoach
  - internal/signal/signal_integration_test.go:167 → .../cmd/stagecoach
  - internal/e2e/harness_test.go:65            → .../cmd/stagecoach

CONSUMED (read-only — S1 landed):
  - go.mod module = github.com/dustin/stagecoach
  - all module-prefix import paths = .../stagecoach/... (S1's sed)

NO-TOUCH (explicitly — owned by sibling/later subtasks):
  - package declarations inside the renamed dirs (package stagehand → package stagecoach)   # S3
  - file names (stagehand.go → stagecoach.go)                                                # S3
  - Go identifiers (Stagehand→Stagecoach)                                                    # P1.M1.T2
  - env vars / git config / strings / .stagehandignore                                       # P1.M2
  - Makefile / .goreleaser / CI build paths (./cmd/stagehand → ./cmd/stagecoach)             # P1.M3.T1
  - docs / providers/*.toml / FUTURE_SPEC.md                                                 # P1.M4
  - go.mod (S1 — landed); PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM HOOKS (informational — owned by OTHER subtasks, NOT S2):
  - S3: rename package declarations + files inside cmd/stagecoach/ + pkg/stagecoach/ → makes 'go build ./...' green.
  - S4: verify binary build paths in test code (the 2 test refs are already fixed by S2; S4 verifies + checks for any others).
  - P1.M3.T1: Makefile MAIN_PKG ./cmd/stagehand → ./cmd/stagecoach (a DIFFERENT ./cmd/stagehand ref, in the Makefile — not S2's).
```

## Validation Loop

### Level 1: The Renames + Ref Fixes (the deliverable)

```bash
cd /home/dustin/projects/stagehand

# dirs renamed (old gone, new present)
ls -d cmd/stagecoach pkg/stagecoach && ! ls -d cmd/stagehand pkg/stagehand 2>/dev/null
# Expected: cmd/stagecoach pkg/stagecoach (and the old dirs error out / don't exist)

# git tracked as renames (R), not delete+add
git status --short | grep -E "cmd/stage|pkg/stage"
# Expected: R  cmd/stagehand/main.go → cmd/stagecoach/main.go (and the 2 pkg files), M for the 3 edited files.

# zero old-subdir import refs remain
grep -rn "stagecoach/pkg/stagehand\|stagecoach/cmd/stagehand" --include="*.go" . | grep -v './.git/' || echo "OK: zero old-subdir refs"
# Expected: OK: zero old-subdir refs
```

### Level 2: The Expected Build Failure (S3's domain — confirms S2 is correct)

```bash
cd /home/dustin/projects/stagehand

go build ./... 2>&1 | head
# Expected: FAILS, but ONLY on package-declaration mismatch (e.g. "found package stagehand in
# pkg/stagecoach/stagehand.go" / "package ... doesn't match directory"). This is the EXPECTED state —
# S3 renames 'package stagehand' → 'package stagecoach' + the files to clear it.

# CRITICAL CHECK: the failure must NOT be an import/dir resolution error (that would mean Task 2 missed a ref):
go build ./... 2>&1 | grep -E "no package in import|directory not found|cannot find package"
# Expected: ZERO matches (the import/dir shadow is fully fixed by Task 2). If any match appears, a
# stagecoach/{pkg,cmd}/stagehand reference was missed — grep + fix it.
```

### Level 3: Scope Discipline

```bash
cd /home/dustin/projects/stagehand

# S2 touches ONLY: 2 dir renames + 3 .go files (the import/build-path shadow). Confirm:
git diff --stat -- internal/
# Expected: only internal/cmd/default_action.go, internal/signal/signal_integration_test.go, internal/e2e/harness_test.go (M).
git status --short | grep -E "cmd/|pkg/"
# Expected: the 3 renamed files (R). No edits to files INSIDE the renamed dirs (main.go, stagehand.go, stagehand_test.go are R only — content unchanged).

# Confirm S2 did NOT touch package declarations / file names inside the renamed dirs (S3's job):
git diff -- cmd/stagecoach/main.go pkg/stagecoach/stagehand.go pkg/stagecoach/stagehand_test.go
# Expected: empty (the files moved via git mv, content byte-identical; their 'package stagehand' lines + names are UNCHANGED — S3's job).
```

## Final Validation Checklist

### Technical Validation

- [ ] `cmd/stagecoach/` + `pkg/stagecoach/` exist; `cmd/stagehand/` + `pkg/stagehand/` don't.
- [ ] `git status` shows the moves as renames (R).
- [ ] ZERO `stagecoach/pkg/stagehand` / `stagecoach/cmd/stagehand` references remain.
- [ ] `go build ./...` fails ONLY on package-declaration mismatch (S3's domain), NOT on import/dir resolution.

### Feature Validation

- [ ] The 3 import/build-path refs read `stagecoach/pkg/stagecoach` / `stagecoach/cmd/stagecoach`.
- [ ] No "no package in import" / "directory not found" error in `go build ./...` (the import shadow is fixed).

### Scope Discipline Validation

- [ ] ONLY the 2 dir renames + 3 .go files modified by S2 (git diff --stat confirms).
- [ ] Did NOT edit package declarations / file names inside the renamed dirs (S3).
- [ ] Did NOT touch go.mod (S1), identifiers (P1.M1.T2), env/config/strings (P1.M2), Makefile/.goreleaser/CI (P1.M3), docs (P1.M4).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

### Code Quality Validation

- [ ] `git mv` used (history preserved — renames, not delete+add).
- [ ] The sed patterns are the full substring (`stagecoach/pkg/stagehand`, `stagecoach/cmd/stagehand`) — unambiguous, target exactly the 3 refs.
- [ ] The contract correction (3 import-path fixes) is documented + achieves OUTPUT #4.

---

## Anti-Patterns to Avoid

- ❌ Don't do "git mv only" — the contract LOGIC says "no code changes" but its OUTPUT #4 requires the
  import paths to point at `stagecoach`. S1's module-prefix sed left 3 subdir refs at `stagehand`; a
  git-mv-only S2 breaks the build on import/dir mismatch and orphans the production import
  (default_action.go:21 — nobody else fixes it). S2 MUST sed the 3 refs.
- ❌ Don't use plain `mv` — use `git mv` to preserve history (rename, not delete+add). Both dirs are
  git-tracked.
- ❌ Don't rename the files inside the dirs (main.go, stagehand.go, stagehand_test.go) or edit their
  `package` lines — that's S3. S2 is the container move + the external import-path shadow.
- ❌ Don't chase `go build ./...` to green — it's EXPECTED to fail after S2 (package declarations don't
  match the renamed dirs; S3 fixes that). S2's gate is "fails ONLY on package-decl mismatch, not on
  import/dir resolution."
- ❌ Don't use a bare `/stagehand` or `stagehand` sed pattern — too broad (would hit package decls,
  identifiers, strings — S3 / P1.M1.T2 / P1.M2 territory). Use the full substring
  `stagecoach/pkg/stagehand` / `stagecoach/cmd/stagehand` — unambiguous now that S1 made the module prefix `stagecoach`.
- ❌ Don't touch the Makefile's `./cmd/stagehand` or .goreleaser's `./cmd/stagehand` — those are build-
  config paths owned by P1.M3.T1, not Go import paths. S2 is Go structural only (the 2 dirs + their 3
  Go import/build refs).
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or `plan/*`.

---

## Confidence Score

**9/10** for one-pass implementation success.

Rationale: Two `git mv`s (mechanical, history-preserving, both dirs confirmed git-tracked) plus three
targeted full-substring seds. The key de-risking is the empirical verification that EXACTLY 3 references
survived S1's module-prefix sed (default_action.go:21 / signal_integration_test.go:167 / harness_test.go:65),
so the sed surface is fully enumerated — no guesswork. The contract correction (why git-mv-only is
insufficient) is documented with the orphan-import reasoning, so the implementer doesn't follow the literal
"no code changes" LOGIC into a confusing import/dir-mismatch build break. The EXPECTED post-state (build
fails only on package-decl mismatch, S3's fix) is precisely characterized, with a grep check that
distinguishes the expected failure from an import/dir-resolution failure (which would signal a missed ref).
The residual uncertainty (not 10/10): the precise wording of the Go compiler's package-mismatch error
(could vary by Go version), but the grep distinguishing "package stagehand/doesn't match directory" (expected)
from "no package in import/directory not found" (a missed ref) is robust regardless of wording. S3 (package
decls + file names), P1.M1.T2 (identifiers), P1.M2 (env/strings), P1.M3 (Makefile/.goreleaser) are cleanly
fenced and untouched.
