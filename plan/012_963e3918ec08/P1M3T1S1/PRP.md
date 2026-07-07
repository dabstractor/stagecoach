---
name: "P1.M3.T1.S1 — Rename Makefile (binary names, paths, targets, echo messages): stagehand → stagecoach — project rename Layer 4.1"
description: |

  The Makefile rename layer of the stagehand→stagecoach project rename (plan 012). The Makefile at the
  project root still references `stagehand` in 17 places (binary names, paths, targets, echo messages,
  comments). The Go module (`github.com/dustin/stagecoach`) and the `cmd/stagecoach/` directory are
  already renamed (M1.T1 Complete). This task replaces every `stagehand` → `stagecoach` in the Makefile
  so `make build` produces `./bin/stagecoach` (not `./bin/stagehand`).

  THE FIX — one global replace (per the contract):
    sed -i 's/stagehand/stagecoach/g' Makefile

  This changes ALL 17 references:
    - BIN:       `$(BIN_DIR)/stagehand` → `$(BIN_DIR)/stagecoach`
    - BIN_TEST:  `$(BIN_DIR)/stagehand-test` → `$(BIN_DIR)/stagecoach-test`
    - MAIN_PKG:  `./cmd/stagehand` → `./cmd/stagecoach`  (the directory was renamed by M1.T1.S2)
    - Header comment (L2): "Stagehand" → "Stagecoach"
    - All target descriptions/echo messages (L6-9, L52-65): "stagehand" → "stagecoach"
    - Install-test symlink paths (L63-65): `stagehand-test` → `stagecoach-test`
    - Comment about ~/.local/bin symlinks (L35-37): "stagehand" → "stagecoach"

  ⚠️ **#1 — the global sed is SAFE because `stagehand` appears ONLY as the product name in the Makefile.**
      There are no partial-word collisions (no `stagehandler`, no `stagehand_internal`, etc.) — verified
      by `grep -c stagehand Makefile` = 17, all of which are the product name in comments/paths/targets.

  ⚠️ **#2 — MAIN_PKG points to `./cmd/stagecoach` which ALREADY EXISTS (M1.T1.S2).** The `cmd/stagehand`
      directory was renamed to `cmd/stagecoach` by M1.T1.S2. So after the sed, `MAIN_PKG := ./cmd/stagecoach`
      resolves to the existing directory. Verify: `ls cmd/stagecoach/main.go`.

  ⚠️ **#3 — `make build` is the verification gate.** After the sed, `make build` must produce
      `./bin/stagecoach` (not `./bin/stagehand`). And `make build-test` must produce `./bin/stagecoach-test`.

  ⚠️ **#4 — No conflict with the parallel work item.** P1.M2.T3.S2 (session ID prefix, temp dir, bootstrap
      template) touches Go files only (bootstrap.go, multiturn.go, verbose.go, tests) — NOT the Makefile.

  Deliverable: MODIFIED `Makefile` (global stagehand→stagecoach). NO other file. NO go.mod change (already
  stagecoach). OUTPUT: `make build` → `./bin/stagecoach`; `make build-test` → `./bin/stagecoach-test`; zero
  `stagehand` references in the Makefile. DOCS: none — build config is internal.

---

## Goal

**Feature Goal**: Rename all `stagehand` references in the Makefile to `stagecoach` so the build system
produces `./bin/stagecoach` and `./bin/stagecoach-test`, matching the renamed Go module
(`github.com/dustin/stagecoach`) and the renamed `cmd/stagecoach/` directory.

**Deliverable**: MODIFIED `Makefile` — global `stagehand` → `stagecoach` (17 references). No other file.

**Success Definition**: `grep -c stagehand Makefile` → 0; `make build` produces `./bin/stagecoach`;
`make build-test` produces `./bin/stagecoach-test`; `make test` passes (the Makefile change doesn't affect
test behavior — it only renames the binary output paths).

## User Persona

**Target User**: The developer/CI building the project — who runs `make build` and expects `./bin/stagecoach`
(the renamed binary). Transitively: the goreleaser task (P1.M3.T1.S2) and the CI task (P1.M3.T1.S3) which
depend on the Makefile targets producing the correctly-named binary.

**Use Case**: `make build` → `./bin/stagecoach` (not `./bin/stagehand`).

**User Journey**: developer runs `make build` → binary lands at `./bin/stagecoach` → `./bin/stagecoach
--version` works.

**Pain Points Addressed**: The Makefile still produces `./bin/stagehand` (the old name), inconsistent with
the renamed module/cmd directory.

## Why

- **Layer 4.1 of the project rename.** The Go structural rename (M1) is Complete — module path, imports,
  cmd directory are all `stagecoach`. The Makefile is the build-system surface that must match.
- **Unblocks goreleaser + CI.** P1.M3.T1.S2 (.goreleaser.yaml) and P1.M3.T1.S3 (CI workflow) depend on
  the Makefile producing `./bin/stagecoach`.
- **Trivially simple.** One `sed` command on one file. 1 point.

## What

A global `stagehand` → `stagecoach` text replacement in the Makefile. No logic change, no new targets, no
structural change — just the product name in binary paths, target descriptions, and echo messages.

### Success Criteria

- [ ] `grep -c stagehand Makefile` → 0 (zero references remain).
- [ ] `make build` produces `./bin/stagecoach` (verify with `ls ./bin/stagecoach`).
- [ ] `make build-test` produces `./bin/stagecoach-test`.
- [ ] `make test` passes (race detector; no behavioral change).
- [ ] `MAIN_PKG := ./cmd/stagecoach` (the renamed directory, which exists from M1.T1.S2).
- [ ] No other file modified.

## All Needed Context

### Context Completeness Check

_Pass._ A developer with no prior repo knowledge can do this from: the exact `sed` command, the verification
commands, and the knowledge that `cmd/stagecoach` exists (M1.T1.S2). No Go/Make knowledge required.

### Documentation & References

```yaml
# The rename surface map (cited by the contract)
- docfile: plan/012_963e3918ec08/architecture/rename_surface_map.md
  section: "### 4.1 Makefile" — lists every reference: binary name, MAIN_PKG, build output, install-test
           symlink paths, echo messages.
  critical: MAIN_PKG → ./cmd/stagecoach (the directory exists from M1.T1.S2).

# The file being edited
- file: Makefile   (at the project root, where go.mod + cmd/ live)
  section: the full file (68 lines). The 17 `stagehand` references span:
           - L2 header comment; L6-9 target descriptions; L28-30 BIN/BIN_TEST/MAIN_PKG;
             L35-37 symlink comments; L52/55/58/61 target descriptions; L63-65 install-test commands+echo.
  why: the ONLY file edited. The sed replaces all 17 at once.
  critical: verify cmd/stagecoach/main.go exists before the sed (M1.T1.S2 prerequisite). go.mod is already
           `github.com/dustin/stagecoach` (M1.T1.S1).

# The project rename note (in your context as selected_prd_content)
- file: PRD.md h2.30 — "this project was originally named 'stagehand' and has been renamed. All references
         to 'stagehand' must be replaced with 'stagecoach'."
```

### Current Codebase tree (relevant slice)

```bash
Makefile                  # 17 stagehand refs → EDIT (global sed → stagecoach)
go.mod                    # module github.com/dustin/stagecoach (ALREADY renamed — M1.T1.S1)
cmd/stagecoach/main.go    # ALREADY renamed (M1.T1.S2)
cmd/stubagent/            # UNCHANGED (not part of the rename)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. ONE in-place edit: Makefile (global stagehand→stagecoach).
```

### Known Gotchas of our codebase & Library Quirks

```makefile
# CRITICAL (the global sed is safe): 'stagehand' appears ONLY as the product name in the Makefile — no
# partial-word collisions (no stagehandler, no stagehand_internal). Verified: grep -c stagehand Makefile = 17,
# all product-name references in comments/paths/targets.
# CRITICAL (MAIN_PKG → ./cmd/stagecoach which EXISTS): cmd/stagecoach/ was renamed by M1.T1.S2. After the sed,
# MAIN_PKG := ./cmd/stagecoach resolves to the existing directory. Verify: ls cmd/stagecoach/main.go.
# GOTCHA (sed -i on the project root Makefile): the Makefile is at the project root (where go.mod lives),
# not in a subdirectory. Run the sed from the project root.
# GOTCHA (make build verification): after the sed, run `make build` and verify `./bin/stagecoach` exists
# (not `./bin/stagehand`). Also clean old artifacts: `rm -f bin/stagehand bin/stagehand-test`.
# GOTCHA (make test unaffected): the Makefile rename changes binary OUTPUT paths, not test behavior.
# `make test` runs `go test -race ./...` — the test binary names are unaffected.
```

## Implementation Blueprint

### Data models and structure

No code. One command + verification:

```bash
# THE rename (from the project root):
sed -i 's/stagehand/stagecoach/g' Makefile

# VERIFY zero references remain:
grep -c stagehand Makefile   # → 0

# VERIFY the key variables:
grep -E 'BIN |BIN_TEST|MAIN_PKG' Makefile
# Expected:
#   BIN      := $(BIN_DIR)/stagecoach
#   BIN_TEST := $(BIN_DIR)/stagecoach-test
#   MAIN_PKG := ./cmd/stagecoach

# VERIFY the build:
make clean   # remove old bin/stagehand artifacts
make build   # → ./bin/stagecoach
ls ./bin/stagecoach   # → exists

make build-test   # → ./bin/stagecoach-test
ls ./bin/stagecoach-test   # → exists

# VERIFY tests still pass:
make test   # → go test -race ./... PASS
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: APPLY the global sed
  - RUN (from the project root): sed -i 's/stagehand/stagecoach/g' Makefile
  - VERIFY: grep -c stagehand Makefile → 0 (zero references remain).
  - VERIFY: grep -E 'BIN |BIN_TEST|MAIN_PKG' Makefile shows stagecoach paths.

Task 2: VERIFY the build + tests
  - RUN: make clean (remove old bin/stagehand artifacts).
  - RUN: make build → verify ./bin/stagecoach exists.
  - RUN: make build-test → verify ./bin/stagecoach-test exists.
  - RUN: make test → go test -race ./... PASS (the rename doesn't affect test behavior).
  - CONFIRM: only Makefile modified (`git diff --name-only` → Makefile only).
```

### Implementation Patterns & Key Details

```makefile
# THE key variables after the sed:
BIN      := $(BIN_DIR)/stagecoach        # was: stagehand
BIN_TEST := $(BIN_DIR)/stagecoach-test   # was: stagehand-test
MAIN_PKG := ./cmd/stagecoach             # was: ./cmd/stagehand (directory exists from M1.T1.S2)

# THE build output:
# make build → ./bin/stagecoach (was: ./bin/stagehand)
# make build-test → ./bin/stagecoach-test (was: ./bin/stagehand-test)
```

### Integration Points

```yaml
GO MODULE: go.mod is ALREADY github.com/dustin/stagecoach (M1.T1.S1). No change needed here.
CMD DIRECTORY: cmd/stagecoach/ ALREADY exists (M1.T1.S2). MAIN_PKG resolves correctly after the sed.

DOWNSTREAM (NOT this task):
  - P1.M3.T1.S2 (.goreleaser.yaml rename) — depends on the Makefile producing ./bin/stagecoach.
  - P1.M3.T1.S3 (CI workflow rename) — depends on the Makefile targets being stagecoach-named.

FROZEN/LEAVE: go.mod, cmd/, all .go files, docs/, providers/, PRD.md, .goreleaser.yaml, .github/.
NO NEW DATABASE / ROUTES / CLI / FILES / CONFIG / DOCS.
```

## Validation Loop

### Level 1: The sed landed correctly

```bash
grep -c stagehand Makefile   # → 0 (zero references remain)
grep -n stagecoach Makefile | head -5   # → the renamed references (BIN, MAIN_PKG, etc.)
# Expected: 0 stagehand hits; ~17 stagecoach hits (the renamed references).
```

### Level 2: Build + test verification

```bash
make clean        # remove old bin/stagehand artifacts
make build        # → ./bin/stagecoach
ls ./bin/stagecoach   # → exists (the renamed binary)
make build-test   # → ./bin/stagecoach-test
ls ./bin/stagecoach-test   # → exists
make test         # → go test -race ./... PASS (rename doesn't affect tests)
# Expected: ./bin/stagecoach + ./bin/stagecoach-test exist; all tests pass.
```

### Level 3: Scope guard (no other file touched)

```bash
git diff --name-only   # Expect ONLY Makefile.
git diff --exit-code go.mod cmd/ internal/ pkg/ docs/ providers/ PRD.md && echo "frozen files UNCHANGED (expected)"
# Expected: only Makefile modified; everything else byte-unchanged.
```

### Level 4: Downstream readiness

```bash
# The goreleaser task (P1.M3.T1.S2) and CI task (P1.M3.T1.S3) will reference the Makefile's binary name.
# Verify the binary name they'll use:
grep '^BIN ' Makefile   # → BIN := $(BIN_DIR)/stagecoach
# This is the name goreleaser + CI will build/reference.
```

## Final Validation Checklist

### Technical Validation
- [ ] `grep -c stagehand Makefile` → 0.
- [ ] `make build` → `./bin/stagecoach` exists.
- [ ] `make build-test` → `./bin/stagecoach-test` exists.
- [ ] `make test` → PASS.
- [ ] Only `Makefile` modified; go.mod + all other files byte-unchanged.

### Feature Validation
- [ ] `MAIN_PKG := ./cmd/stagecoach` (the renamed directory).
- [ ] `BIN := $(BIN_DIR)/stagecoach`; `BIN_TEST := $(BIN_DIR)/stagecoach-test`.
- [ ] All echo messages + comments say "stagecoach" (not "stagehand").

### Code Quality Validation
- [ ] The global sed is safe (no partial-word collisions — verified).
- [ ] No Make target names changed (build/build-test/install/install-test/test/coverage/lint/clean/help —
      these are TARGET names, not product names; the sed only changes the product-name strings within them).

### Documentation
- [ ] No docs change (build config is internal; P1.M4 owns the docs rename).

---

## Anti-Patterns to Avoid

- ❌ **Don't manually edit individual lines.** The contract specifies `sed -i 's/stagehand/stagecoach/g'`
      — a single global replace. Manual edits risk missing a reference.
- ❌ **Don't rename the Make TARGETS (build, test, lint, etc.).** Those are standard Make target names,
      not product names. The sed replaces `stagehand` (the product name) everywhere it appears, which
      includes target DESCRIPTIONS (`## Compile the stagecoach binary`) and VARIABLES (`BIN := .../stagecoach`)
      but NOT the target NAMES themselves (`build:`, `test:`).
- ❌ **Don't forget to clean old artifacts.** `make clean` removes `bin/stagehand*` so the old binary
      doesn't linger alongside the renamed one.
- ❌ **Don't edit go.mod, .goreleaser.yaml, or CI files.** Those are separate subtasks (M1.T1.S1 done;
      M3.T1.S2/S3 are siblings). This task is Makefile ONLY.

---

## Confidence Score

**10/10** — a single `sed -i 's/stagehand/stagecoach/g' Makefile` on one file with 17 verified references,
all of which are the product name (no partial-word collisions). The prerequisite directory `cmd/stagecoach/`
exists (M1.T1.S2), go.mod is already `stagecoach` (M1.T1.S1), and the verification is deterministic
(`grep -c stagehand Makefile` → 0; `make build` → `./bin/stagecoach`). No conflict with the parallel
P1.M2.T3.S2 (Go files only). No residual risk.
