---
name: "P1.M5.T2.S1 — Final grep audit: zero stagehand references in tracked files (rename complete + verified)"
description: |

  THIS IS A VERIFICATION + REMEDIATION TASK. It certifies the stagehand→stagecoach rename (PRD h2.30) is
  complete across the whole repo's SHIPPED surface, fixes the small set of real stragglers M1–M4 missed,
  and documents the legitimate exceptions the contract's literal audit command wrongly counts as failures.

  ⚠️ HEADLINE RESEARCH FINDING (grep-verified in the live repo): the contract's literal audit command is
  STALE — its exclusion (`plan/012_963e3918ec08/architecture/` ONLY) is too narrow, the SAME defect the
  sibling P1.M5.T1.S1 found. Run today it returns ~2654 lines, of which ~2650 are three LEGITIMATE
  categories that are NOT rename failures:
    ZONE B — ~2470 lines in plan/012/ NON-architecture files: these ARE the rename's own PRPs/research
             ("rename stagehand.* keys → stagecoach.*"). Renaming them corrupts them (⇒ "stagecoach →
             stagecoach"). The rename's docs MUST reference both names. P1.M5.T1.S1 prunes the WHOLE
             plan/012 tree for this reason; the contract's architecture/-only exclusion predates plan/012.
    ZONE C — ~180 lines / ~19 `tasks.json` files under plan/001–011: orchestrator-owned (FORBIDDEN) AND
             .json (NOT in the rename's text surface of .md/.go/.toml/.txt/.yaml/.yml/Makefile). Flagged by
             P1.M5.T1.S1 for the orchestrator; this task does NOT touch them.
    PRD    — 1 line: PRD.md:2366, the rename DIRECTIVE itself ("…originally named 'stagehand'…"). PRD.md is
             human-owned/READ-ONLY; it MUST name the old product.

  The ACTUAL shipped-product residue (ZONE A: `git grep -i stagehand | grep -v plan/`) is exactly FOUR
  lines — ONE legitimate (PRD directive) + THREE real stragglers to FIX:
    1. internal/git/git.go:390      — stale COMMENT `.git/STAGEHAND_EDITMSG` (the real constant is
                                     STAGECOACH_EDITMSG, internal/generate/finalize.go:78). FIX → STAGECOACH_EDITMSG.
    2. .goreleaser.yaml:1           — stale COMMENT "Stagehand release config" (project_name is already
                                     stagecoach). FIX → "Stagecoach release config".
    3. .golangci.yml:40             — stale lint-exclusion PATH `pkg/stagehand/stagehand_test.go` (that file
                                     no longer exists; it is now pkg/stagecoach/stagecoach_test.go). FIX the path.

  CONTRACT (item_description §1–§5; PRD h2.30):
    1. RESEARCH NOTE: "verify completeness. The exception: plan/012/architecture/ files intentionally
       reference 'stagehand' as the OLD name being documented." ← STALE: the exception is the WHOLE
       plan/012 tree (the rename's own PRPs/research), not just architecture/; plus tasks.json + PRD directive.
    2. INPUT: "The fully renamed project (M1–M5.T1 complete)."
    3. LOGIC: "Run `git grep -i 'stagehand' | grep -v 'plan/012_963e3918ec08/architecture/' | wc -l`. This
       must return 0. If any references remain, fix them. Also verify: no stale directories (cmd/stagehand,
       pkg/stagehand should not exist), no stale files (stagehand.go, stagehand_test.go should not exist
       anywhere outside the architecture docs)." ← the command's exclusion is too narrow; use the
       CORRECTED audit (§"Implementation Patterns"). The "fix them" clause = the 3 ZONE-A fixes.
    4. OUTPUT: "the corrected audit == 0; the rename is complete and verified."
    5. DOCS: "none — verification step."

  SCOPE BOUNDARY (do NOT touch):
    - PRD.md — human-owned, READ-ONLY (the §FORBIDDEN OPERATIONS list). Its line 2366 is the rename
      directive; it is the one ACCEPTED PRD exception.
    - **tasks.json anywhere** — orchestrator-owned, FORBIDDEN (the ~19 residue files under plan/001–011).
    - plan/012_963e3918ec08/ — the rename changeset (PRPs/research/architecture). PRESERVE; renaming its
      text corrupts it. (Excluded from the corrected audit, same as P1.M5.T1.S1.)
    - plan/001..011 non-tasks.json text — already CLEAN (0 in-scope residue; certified by P1.M5.T1.S1).
    - Go source beyond the single git.go:390 comment, and all of internal/, cmd/, pkg/, docs/, README.md,
      Makefile, .github/, providers/, go.mod — already renamed M1–M4; UNCHANGED.

  DELIVERABLE (3 one-line FIXES + the corrected audit returning 0; NO new files):
    MODIFY internal/git/git.go      (line 390 comment: STAGEHAND_EDITMSG → STAGECOACH_EDITMSG)
    MODIFY .goreleaser.yaml          (line 1 comment: Stagehand → Stagecoach)
    MODIFY .golangci.yml             (line 40 lint-exclusion path: pkg/stagehand/stagehand_test.go → pkg/stagecoach/stagecoach_test.go)
    # Then run the CORRECTED audit (excludes plan/012, tasks.json, PRD.md) → expect 0.

  SUCCESS: corrected audit returns 0; the 3 fixes land; stale-dir/stale-file checks clean; `go build ./...
  && go vet ./... && go test ./...` green; `goreleaser check` clean; golangci-lint v1.61 (or go vet smoke)
  clean; the 3 legitimate exception categories (plan/012, tasks.json, PRD directive) documented &
  accounted for; `git status` shows ONLY the 3 modified files.

---

## Goal

**Feature Goal**: Certify — and make true — that the stagehand→stagecoach rename (PRD h2.30) leaves ZERO
`stagehand` references anywhere in the repo's SHIPPED surface (Go source, build/release configs, lint
config, docs), with the only remaining matches being three fully-documented, legitimate exception
categories (the rename's own plan/012 documentation; orchestrator-owned tasks.json; the PRD rename
directive). This closes the rename: every line a user/contributor/release encounters says "stagecoach".

**Deliverable**: Three one-line remediations to real stragglers M1–M4 missed (a stale comment in
`internal/git/git.go`, a stale comment in `.goreleaser.yaml`, a stale lint-exclusion path in
`.golangci.yml`), plus the corrected whole-repo audit command returning 0 (excluding the three documented
exception categories). No new files (DOCS: none — verification step).

**Success Definition**:
- The corrected audit (`git grep -i stagehand | grep -v plan/012 | grep -v tasks.json | grep -v '^PRD.md:' | wc -l`) returns **0**.
- The 3 fixes land and are individually validated (go build/vet/test green; `goreleaser check` clean;
  the .golangci.yml path now points at the existing file).
- Stale-dir/stale-file checks pass: no `cmd/stagehand`, no `pkg/stagehand`, no `stagehand.go`/
  `stagehand_test.go` outside plan/012.
- The 3 legitimate exception categories are enumerated and accounted for (plan/012 rename docs >0 &
  preserved; tasks.json >0 & forbidden; PRD.md directive = 1 & read-only).
- `git status --short` shows ONLY the 3 modified files; nothing in plan/, PRD.md, tasks.json, or Go source
  beyond the git.go comment is touched.

## User Persona

**Target User**: the maintainer and the release engineer certifying the rename before the first
`stagecoach` tag. Secondary: any future contributor grep-ing the repo who must never be misled by a stale
"stagehand" reference into thinking the old name is still in force.

**Use Case**: "Is the rename actually done? Show me the audit." The maintainer runs the corrected audit
(0), eyeballs the 3 documented exceptions, and ships. A stale `.golangci.yml` path or `.goreleaser.yaml`
comment would silently mis-configure CI/release; this task removes that risk.

## Why

- **It IS the rename's close-out gate (PRD h2.30).** Every other rename subtask (M1–M4, P1.M5.T1.S1)
  targeted a surface; this one proves the UNION is clean. Without it, the rename is "probably done".
- **Catches the stragglers the structural passes miss.** M1–M4 renamed identifiers, imports, paths, and
  config VALUES. They cannot mechanically guarantee every COMMENT and every LINT-EXCLUSION PATH followed.
  This audit's whole point is the net that catches those three: a comment naming a now-renamed constant, a
  header comment, and a path-exclusion that points at a file that no longer exists.
- **The `.golangci.yml` fix is a latent CI bug, not cosmetic.** The stale path matches nothing, so the
  errcheck+unused exclusions for that test file silently stop applying — a real (if minor) lint-regression
  risk on the renamed test file. Fixing it restores the intended suppression.
- **Documents the exceptions honestly (§12.7.2 spirit).** A bare "grep == 0" achieved by corrupting
  plan/012 or by hiding tasks.json would be a lie. This task achieves 0 on the SHIPPED surface while
  explicitly accounting for the three categories that legitimately retain the old name.

## What

1. **Fix the 3 real stragglers** (ZONE A, the only shipped-surface residue):
   - `internal/git/git.go:390` comment: `.git/STAGEHAND_EDITMSG` → `.git/STAGECOACH_EDITMSG` (the actual
     constant written by `internal/generate/finalize.go:78` is `filepath.Join(gitDir, "STAGECOACH_EDITMSG")`).
   - `.goreleaser.yaml:1` comment: `Stagehand release config` → `Stagecoach release config`.
   - `.golangci.yml:40` lint-exclusion `path`: `pkg/stagehand/stagehand_test.go` → `pkg/stagecoach/stagecoach_test.go`.
2. **Run the corrected audit** (excludes the 3 legitimate categories) → expect 0.
3. **Verify the stale-dir/stale-file checks** (cmd/stagehand, pkg/stagehand, stagehand.go) — expect clean.
4. **Document** the 3 legitimate exception categories (plan/012 rename docs; tasks.json; PRD directive) so
   the 0 is not achieved by hiding residue.

The contract's LITERAL command (`… | grep -v 'plan/012/architecture/' | wc -l`) is NOT run verbatim — its
exclusion is too narrow (predates plan/012's own PRPs) and would count ~2650 legitimate lines as failures.
The corrected command broadens the exclusion to the whole plan/012 tree + tasks.json + PRD.md, matching
the contract's STATED INTENT ("exclude the rename's own docs") and the sibling P1.M5.T1.S1's treatment.

### Success Criteria

- [ ] `internal/git/git.go:390` comment reads `.git/STAGECOACH_EDITMSG` (matches the runtime constant).
- [ ] `.goreleaser.yaml:1` comment reads `Stagecoach release config`.
- [ ] `.golangci.yml:40` `path` reads `pkg/stagecoach/stagecoach_test.go` (the file exists).
- [ ] Corrected audit returns 0: `git grep -i 'stagehand' | grep -v 'plan/012_963e3918ec08/' | grep -v
      'tasks\.json' | grep -v '^PRD\.md:' | wc -l` == 0.
- [ ] Stale-dir check: `ls -d cmd/stagehand pkg/stagehand 2>/dev/null` → no output (both absent).
- [ ] Stale-file check: `git ls-files | grep -i 'stagehand\.go$' | grep -v 'plan/012_963e3918ec08/'` → none.
- [ ] `go build ./... && go vet ./... && go test ./...` green.
- [ ] `goreleaser check` clean (validates .goreleaser.yaml after the comment edit).
- [ ] The 3 exception categories accounted for: plan/012 refs >0 (preserved); tasks.json refs >0 (forbidden,
      untouched); PRD.md ref == 1 (the directive, read-only).
- [ ] `git status --short` shows ONLY the 3 modified files.

## All Needed Context

### Context Completeness Check

_Pass._ An implementer with no prior repo knowledge can do this from: the exact 3 lines to change (with
before/after), the corrected audit command (verbatim), the stale-dir/file checks (verbatim), the rationale
for each exception category, and the validation commands. No Go/provider/goreleaser internals are required
(all 3 edits are mechanical: a comment, a comment, a path string).

### Documentation & References

```yaml
# MUST READ — THE decisive doc (the verified state + the corrected audit + fix safety)
- docfile: plan/012_963e3918ec08/P1M5T2S1/research/findings.md
  why: §1 WHY the contract's literal command can't return 0 (3 legitimate categories); §2 the ZONE-A table
       (the 3 stragglers + the PRD exception, with before/after); §3 stale-dir/file checks verified clean;
       §4 the project's own rename_surface_map Gate 5; §5 the corrected audit command (verbatim); §6 fix
       safety; §7 out-of-scope.
  critical: §2 (the exact 3 fixes), §5 (the corrected audit), §1 (why the literal command is stale).

# MUST READ — the sibling task that established the plan/012 + tasks.json treatment (the precedent)
- docfile: plan/012_963e3918ec08/P1M5T1S1/PRP.md
  why: P1.M5.T1.S1 (parallel, treated as a CONTRACT) determined that (a) the WHOLE plan/012 tree is the
       rename preserve target (not just architecture/), (b) tasks.json is FORBIDDEN/orchestrator-owned and
       flagged for THIS task (P1.M5.T2.S1), (c) the in-scope plan/001–011 TEXT is already clean. This task
       builds on those outputs: it accepts plan/012 + tasks.json as exceptions and audits the REST.
  critical: the plan/012-whole-tree preserve + the tasks.json flag are ESTABLISHED by T1.S1; do not re-litigate.

# MUST READ — the runtime constant the git.go comment must match
- file: internal/generate/finalize.go   (READ ONLY — the --edit editor gate)
  section: line 78 — `editMsgPath := filepath.Join(gitDir, "STAGECOACH_EDITMSG")`. The ACTUAL file the tool
           writes is `<gitDir>/STAGECOACH_EDITMSG`. So the git.go:390 interface doc comment that says
           `.git/STAGEHAND_EDITMSG` is a STALE comment (rename missed it). Fixing it to STAGECOACH_EDITMSG
           makes the comment truthful. (Comment-only change; finalize.go is NOT edited.)
  gotcha: do NOT "fix" the constant in finalize.go — it is ALREADY STAGECOACH_EDITMSG. The only straggler
          is the COMMENT in git.go:390. Editing finalize.go would be a bug.

# MUST READ — the release config whose header comment is stale
- file: .goreleaser.yaml   (READ ONLY except line 1)
  section: line 12 `project_name: stagecoach`, line 20 `id: stagecoach`, line 21 `main: ./cmd/stagecoach`,
           line 22 `binary: stagecoach` — all already renamed (P1.M3.T1.S2). ONLY line 1's header comment
           still says "Stagehand release config". Fix line 1 → "Stagecoach release config".
  why: confirms the comment is the SOLE stale token in the file (the real config is correct), so the fix
       is comment-only and `goreleaser check` will pass.
  gotcha: the file notes the git remote is `dabstractor/stagecoach` and must be reachable at
          github.com/dustin/stagecoach before the first real tag — OUT OF SCOPE for this audit (naming/
          namespace reconciliation is a release-engineering concern, not a rename-residue concern).

# MUST READ — the lint config with the stale exclusion path
- file: .golangci.yml   (READ ONLY except line 40)
  section: the `issues.exclude-rules` list (lines ~33–43) suppresses errcheck/unused on specific test
           files. Line 40 `- path: pkg/stagehand/stagehand_test.go` points at a file that NO LONGER EXISTS
           (it is now pkg/stagecoach/stagecoach_test.go — verified present). Fix the path.
  why: a stale `path` matches nothing, so the exclusion silently stops applying — a latent CI lint
       regression on that test file. Fixing it restores the intended suppression.
  gotcha: CI pins golangci-lint v1.61 (the file's schema note); v2 rejects this v1 schema. Validate with
          v1.61 locally if available, else fall back to `go vet ./...` as a smoke check (the fix is a plain
          path string; risk is minimal). Do NOT migrate the schema here (out of scope).

# READ — the rename directive (the one accepted PRD exception; READ-ONLY)
- file: PRD.md   (READ ONLY — line 2366, heading h2.30)
  section: "Note: this project was originally named 'stagehand' and has been renamed. All references to
           'stagehand' must be replaced with 'stagecoach'."
  why: this line is the rename DIRECTIVE — it MUST name the old product. PRD.md is human-owned and on the
       FORBIDDEN-TO-MODIFY list. It is the single accepted PRD exception in the audit.
  gotcha: do NOT edit PRD.md. The corrected audit excludes '^PRD.md:' so this line does not count against 0.

# READ — the project's own rename verification gate (authority for the extension allow-list)
- file: plan/012_963e3918ec08/architecture/rename_surface_map.md   (READ ONLY — "Verification Gates" §)
  section: Gate 5 — `grep -ri 'stagehand' --include='*.go' --include='*.md' --include='*.toml'
           --include='*.yaml' --include='*.yml' --include='Makefile' . | grep -v '.git/' | wc -l == 0`.
  why: the project's AUTHORITATIVE gate. Note its `--include` set deliberately OMITS `.json` — corroborating
       that tasks.json is NOT part of the rename's text surface (do not count it as a failure). It predates
       plan/012, so it needs the plan/012 + PRD-directive exceptions added for the FINAL audit (done here).

# READ — the historical-plan finding (why tasks.json residue is expected and benign)
- file: plan/012_963e3918ec08/architecture/critical_findings.md   (READ ONLY — §F8)
  section: F8 — "plan/001_* through plan/011_* … reference 'stagehand' extensively but are never compiled,
           never executed, and never shipped." (Describes the PRE-rename state; the .md/.go/.toml/.txt text
           is now clean per P1.M5.T1.S1. The remaining plan/001–011 residue is the .json tasks.json files.)
  why: corroborates that tasks.json residue is non-functional and orchestrator-owned.
```

### Current Codebase tree (relevant slice)

```bash
internal/git/git.go            # line 390: stale comment .git/STAGEHAND_EDITMSG  ← FIX (comment only)
.goreleaser.yaml               # line 1:  stale comment "Stagehand release config" ← FIX (comment only)
.golangci.yml                  # line 40: stale path pkg/stagehand/stagehand_test.go ← FIX (path string)
PRD.md                         # line 2366: rename directive (READ-ONLY; the one accepted PRD exception)
internal/generate/finalize.go  # line 78: STAGECOACH_EDITMSG (the real constant; READ-ONLY — already correct)
pkg/stagecoach/stagecoach_test.go  # EXISTS (the path .golangci.yml SHOULD point to)
cmd/stagecoach/  pkg/stagecoach/   # present (cmd/stagehand, pkg/stagehand ABSENT — verified clean)
plan/012_963e3918ec08/         # the rename changeset (PRESERVE — its PRPs/research/architecture reference both names)
plan/001..011/**/tasks.json    # ~19 orchestrator-owned residue files (FORBIDDEN — do not touch)
```

### Desired Codebase tree with files to be changed

```bash
# 3 one-line edits, NO new files, NO deletions, NO renames.
internal/git/git.go            # MODIFY line 390 comment: STAGEHAND_EDITMSG → STAGECOACH_EDITMSG
.goreleaser.yaml               # MODIFY line 1 comment:  Stagehand → Stagecoach
.golangci.yml                  # MODIFY line 40 path:    pkg/stagehand/stagehand_test.go → pkg/stagecoach/stagecoach_test.go
# EVERYTHING ELSE UNCHANGED: PRD.md, tasks.json, plan/012/*, plan/001..011/*, all Go source/cmd/pkg/docs.
```

### Known Gotchas of our codebase & Library Quirks

```bash
# CRITICAL (the contract's literal command is STALE — do NOT run it verbatim): its exclusion
# (`plan/012_963e3918ec08/architecture/` ONLY) predates plan/012's own PRPs/research. Run today it returns
# ~2654 lines: ~2470 in plan/012 NON-architecture (the rename's own docs — PRESERVE), ~180 in plan/001–011
# tasks.json (FORBIDDEN), 1 in PRD.md (the directive — READ-ONLY). The CORRECTED audit (below) excludes the
# whole plan/012 tree + tasks.json + PRD.md and returns 0 AFTER the 3 fixes. This is the SAME staleness the
# sibling P1.M5.T1.S1 found; the fix is the same shape (broaden the exclusion to match reality).

# CRITICAL (the git.go:390 fix is a COMMENT, not the constant): the runtime constant is ALREADY
# STAGECOACH_EDITMSG (internal/generate/finalize.go:78). Only the INTERFACE DOC COMMENT in git.go:390 still
# says STAGEHAND_EDITMSG. Fix the COMMENT. Do NOT touch finalize.go (it is correct; editing it reintroduces
# the bug). Comment-only ⇒ go build/vet/test unaffected.

# CRITICAL (the .golangci.yml fix is a latent CI bug, not cosmetic): the stale `path:
# pkg/stagehand/stagehand_test.go` matches NO file (it was renamed). golangci-lint silently stops applying
# the errcheck+unused exclusion ⇒ those findings would surface in CI for that test file. Fix the path to
# pkg/stagecoach/stagecoach_test.go (verified present) to restore the suppression.

# CRITICAL (THREE exception categories are LEGITIMATE, not failures — document, don't hide):
#   - plan/012_963e3918ec08/  → the rename changeset; its PRPs/research/architecture reference BOTH names
#     ("rename stagehand.* → stagecoach.*"). Renaming them corrupts them (⇒ "stagecoach → stagecoach").
#     PRESERVE. (P1.M5.T1.S1 prunes the whole tree for the same reason.)
#   - **/tasks.json           → orchestrator-owned (FORBIDDEN to modify) AND .json (not in the rename's
#     text surface .md/.go/.toml/.txt/.yaml/.yml/Makefile). ~19 files under plan/001–011. Flagged by T1.S1.
#   - PRD.md:2366             → the rename DIRECTIVE itself; PRD is human-owned/READ-ONLY. MUST name the old
#     product. The one accepted PRD exception.

# GOTCHA (git grep vs grep -ri): `git grep` searches ONLY tracked files — it cannot see .git/ internals or
# gitignored build output (bin/), so NO `.git/` guard is needed. The contract uses `git grep`; match it.
# (The rename_surface_map Gate 5 uses `grep -ri .` which DOES need `grep -v '.git/'` — don't conflate them.)

# GOTCHA (the local checkout dir is /home/dustin/projects/stagehand): `pwd` shows "stagehand" but that is a
# LOCAL working-tree path, NOT a tracked file — `git grep` cannot see it and it ships in no artifact. It is
# OUT OF SCOPE (do not try to "fix" the directory name; it has no bearing on the rename's completeness).

# GOTCHA (every stagehand substring is a desired rename EXCEPT the documented exceptions): the binary name,
# prose name, .stagecoachignore, stagecoach.* git keys, STAGECOACH_* env, github.com/dustin/stagecoach
# import — all already stagecoach. The 3 stragglers are the last shipped-surface holdouts. `commit-pi` (the
# originating tool) is unrelated and untouched.

# GOTCHA (golangci-lint version): CI pins v1.61 (the .golangci.yml schema note). v2 rejects this v1 schema.
# Validate with v1.61 if installed (`go install …@v1.61.0`); else `go vet ./...` is an acceptable smoke
# check for a one-line path-string edit. Do NOT migrate the schema (out of scope for this audit).
```

## Implementation Blueprint

### Data models and structure

_None._ Three one-line textual edits (two comments, one path string) + a verification audit. No code, no
data models, no new files.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ESTABLISH the baseline (READ/count — frames the task; no edits)
  - RUN the ZONE-A enumeration (the shipped-surface residue):
        git grep -i 'stagehand' | grep -v 'plan/' | tee /tmp/zoneA.txt
    EXPECTED: exactly 4 lines — PRD.md:2366 (directive, leave), internal/git/git.go:390 (fix),
    .goreleaser.yaml:1 (fix), .golangci.yml:40 (fix). Record them.
  - RUN the exception-category counts (to document, not fix):
        git grep -i 'stagehand' -- 'plan/012_963e3918ec08/' | wc -l        # EXPECT >0 (rename docs, preserve)
        git grep -i 'stagehand' | grep 'tasks\.json' | wc -l               # EXPECT >0 (~19 files, forbidden)
        git grep -i 'stagehand' -- PRD.md | wc -l                          # EXPECT 1 (the directive, read-only)
  - RUN the stale-dir/stale-file checks:
        ls -d cmd/stagehand pkg/stagehand 2>/dev/null                       # EXPECT no output
        git ls-files | grep -i 'stagehand\.go$' | grep -v 'plan/012_963e3918ec08/'   # EXPECT none
    EXPECTED: all clean (M1 renamed dirs/files). If ANY stale dir/file exists, that is a REAL regression
    outside this task's 3 fixes — STOP and flag it (it means an earlier rename subtask regressed).
  - WHY: Task 1 proves the state matches the research (4 ZONE-A lines; 3 exception categories; clean dirs).
      If the counts differ (e.g. a 5th shipped-surface straggler appeared), add it to Task 2's fix list.

Task 2: FIX the 3 ZONE-A stragglers (the only edits)
  - EDIT internal/git/git.go line 390:
      OLD: `	// --edit editor gate (§9.22 FR-E1) to locate .git/STAGEHAND_EDITMSG. \`--absolute-git-dir\` succeeds on`
      NEW: `	// --edit editor gate (§9.22 FR-E1) to locate .git/STAGECOACH_EDITMSG. \`--absolute-git-dir\` succeeds on`
      (change ONLY the token STAGEHAND_EDITMSG → STAGECOACH_EDITMSG; leave the rest of the line untouched.)
      WHY: the runtime constant is STAGECOACH_EDITMSG (finalize.go:78); the comment must match. Comment-only.
  - EDIT .goreleaser.yaml line 1:
      OLD: `# .goreleaser.yaml — Stagehand release config (goreleaser v2). PRD §21.2.`
      NEW: `# .goreleaser.yaml — Stagecoach release config (goreleaser v2). PRD §21.2.`
      (change ONLY "Stagehand" → "Stagecoach"; project_name/builds/etc. are already stagecoach.)
  - EDIT .golangci.yml line 40:
      OLD: `    - path: pkg/stagehand/stagehand_test.go`
      NEW: `    - path: pkg/stagecoach/stagecoach_test.go`
      WHY: the file was renamed (now pkg/stagecoach/stagecoach_test.go, verified present); the exclusion
      must point at it to keep applying. This restores the intended errcheck+unused suppression.
  - GOTCHA: make EXACTLY these 3 edits. Do not touch PRD.md, tasks.json, plan/012, or any other line.

Task 3: VALIDATE each fix in isolation (cheap, before the whole-repo audit)
  - git.go comment: `go build ./... && go vet ./...` — clean (comment-only; nothing can break).
      (Optional: `go test ./internal/git/...` — green; the comment is not exercised.)
  - .goreleaser.yaml: `goreleaser check` — clean (validates the schema; the comment edit cannot break it,
      but this confirms the file is still well-formed after the edit).
  - .golangci.yml: `go vet ./...` (smoke); if golangci-lint v1.61 is installed, `golangci-lint run
      ./pkg/stagecoach/...` — the path edit restores the exclusion (no new errcheck/unused findings surface
      for stagecoach_test.go beyond what was previously suppressed).
  - WHY: isolating each fix's validation makes a regression attributable if the whole-repo audit (Task 4)
      later surprises. All three are low-risk (comment, comment, path-string).

Task 4: RUN the corrected whole-repo audit (THE gate) + account for the exceptions
  - THE GATE (corrected audit — excludes plan/012, tasks.json, PRD.md):
        git grep -i 'stagehand' \
          | grep -v 'plan/012_963e3918ec08/' \
          | grep -v 'tasks\.json' \
          | grep -v '^PRD\.md:' \
          | tee /tmp/audit_residue.txt | wc -l
    EXPECTED: 0 (and /tmp/audit_residue.txt empty). If >0, the remaining lines are NEW stragglers — read
    them, add fixes to Task 2, re-run. (Task 1's ZONE-A enumeration + the 3 fixes should make this 0.)
  - ACCOUNT for the 3 legitimate exception categories (document; each is EXPECTED non-zero):
        git grep -i 'stagehand' -- 'plan/012_963e3918ec08/' | wc -l   # >0 — rename docs (PRESERVED)
        git grep -i 'stagehand' | grep -c 'tasks\.json'               # >0 — orchestrator-owned (FORBIDDEN)
        git grep -i 'stagehand' -- PRD.md | wc -l                     # 1   — the directive (READ-ONLY)
  - STALE-DIR/STALE-FILE re-check (must still be clean):
        ls -d cmd/stagehand pkg/stagehand 2>/dev/null                 # no output
        git ls-files | grep -i 'stagehand\.go$' | grep -v 'plan/012_963e3918ec08/'   # none
  - WHY: the corrected audit + the exception accounting together prove the rename is complete on the
      shipped surface WITHOUT hiding the legitimate old-name references.

Task 5: FULL regression + scope audit
  - `go build ./... && go vet ./... && go test ./...` — green (the git.go comment cannot affect this; runs
      as the rename's whole-repo regression check; plan/ is not compiled).
  - `git status --short` — shows ONLY the 3 modified files (internal/git/git.go, .goreleaser.yaml,
      .golangci.yml). NOTHING in plan/, PRD.md, tasks.json, or other Go source.
  - `git status --porcelain | grep -cE 'tasks\.json|PRD\.md|plan/012'` — 0 (untouched).
  - DOCUMENT (in the implementation summary, not a new file): the 3 fixes; the corrected audit == 0; the 3
      exception categories accounted for; the stale-dir/file checks clean. The rename is COMPLETE.
```

### Implementation Patterns & Key Details

```bash
# PATTERN: the corrected whole-repo audit (the contract's command + the 3 proper exclusions). This is THE
# gate. After Task 2's fixes it returns 0. `git grep` searches only tracked files (no .git/, no gitignored
# build output), so no .git guard is needed.
git grep -i 'stagehand' \
  | grep -v 'plan/012_963e3918ec08/' \
  | grep -v 'tasks\.json' \
  | grep -v '^PRD\.md:' \
  | wc -l
# Expected: 0.

# PATTERN: the ZONE-A enumeration (the shipped-surface residue the audit exists to catch/fix):
git grep -i 'stagehand' | grep -v 'plan/'
# Expected BEFORE fixes: PRD.md:2366, internal/git/git.go:390, .goreleaser.yaml:1, .golangci.yml:40 (4 lines).
# Expected AFTER fixes:  PRD.md:2366 only (1 line — the directive; excluded by the corrected audit's '^PRD.md:').

# CRITICAL: the contract's literal command (`… | grep -v 'plan/012/architecture/' | wc -l`) is NOT run — its
#   exclusion is too narrow (architecture/ only, predating plan/012's own PRPs). Run the CORRECTED audit.
# CRITICAL: git.go:390 is a COMMENT fix — the constant (finalize.go:78) is ALREADY STAGECOACH_EDITMSG. Do not
#   edit finalize.go.
# CRITICAL: .golangci.yml:40 is a latent CI bug (stale path matches nothing) — fixing it restores the
#   errcheck+unused suppression for the renamed test file.
# CRITICAL: the 3 exception categories (plan/012, tasks.json, PRD directive) are LEGITIMATE — document them,
#   do NOT achieve 0 by corrupting plan/012 or editing forbidden tasks.json / read-only PRD.md.

# GOTCHA: the local checkout dir name (/home/dustin/projects/stagehand) is untracked — git grep can't see
#   it; it ships in nothing. Out of scope.
# GOTCHA: golangci-lint CI pin is v1.61; v2 rejects the v1 schema. Validate with v1.61 or fall back to go vet.
```

### Integration Points

```yaml
RENAME CLOSE-OUT (PRD h2.30):
  - this task is the UNION gate: every prior rename subtask (M1 Go, M2 config, M3 build/CI, M4 docs,
    P1.M5.T1.S1 plan/ text) targeted a surface; this proves the WHOLE shipped surface is clean.
  - the corrected audit (0) + the 3 documented exception categories = the rename is COMPLETE.

SHIPPED SURFACE (the 3 fixes):
  - internal/git/git.go:390 — a public-interface doc comment (the GitDir method). Making it say
    STAGECOACH_EDITMSG keeps the godoc truthful vs the runtime constant.
  - .goreleaser.yaml:1 — the release config header comment. `goreleaser check` validates it post-edit.
  - .golangci.yml:40 — the lint-exclusion path. The fix restores the suppression (latent CI correctness).

LEGITIMATE EXCEPTIONS (documented, not fixed):
  - plan/012_963e3918ec08/ — the rename changeset; PRESERVE (P1.M5.T1.S1 prunes the whole tree).
  - **/tasks.json — orchestrator-owned, FORBIDDEN; ~19 files under plan/001–011 (flagged by T1.S1).
  - PRD.md:2366 — the rename directive; READ-ONLY (human-owned).

DOWNSTREAM (P1.M5.T2.S2 — full build+test with stagecoach identity): runs AFTER this task. Its `make build`
  produces ./bin/stagecoach and its test suite runs green; this task's git.go comment edit cannot affect it
  (comments aren't compiled). The .golangci.yml fix makes S2's `make lint` (if it runs golangci-lint) apply
  the intended exclusion again.
```

## Validation Loop

### Level 1: The 3 fixes (immediate, isolated)

```bash
# git.go comment — comment-only; go build/vet are the check (nothing can break).
go build ./... && go vet ./...      # Expect clean.

# .goreleaser.yaml — schema validation (the comment edit cannot break it, but this confirms well-formedness).
goreleaser check                    # Expect: "config is valid" (or equivalent). Needs goreleaser installed.

# .golangci.yml — smoke check; full lint if v1.61 available.
go vet ./...                                                        # smoke (always available).
golangci-lint run ./pkg/stagecoach/...   2>/dev/null || true        # if v1.61 installed: no NEW errcheck/unused
                                                                    # findings for stagecoach_test.go beyond prior suppression.
# Expected: all clean. (The path edit is a plain string; risk is minimal.)
```

### Level 2: The corrected whole-repo audit (THE gate)

```bash
# THE gate — excludes plan/012 (rename docs), tasks.json (forbidden), PRD.md (directive). Expect 0.
git grep -i 'stagehand' | grep -v 'plan/012_963e3918ec08/' | grep -v 'tasks\.json' | grep -v '^PRD\.md:' | wc -l
# Expected: 0. If >0, the residue lines are NEW stragglers — fix them (extend Task 2), re-run.

# ZONE-A after fixes — the only remaining shipped-surface ref should be the PRD directive:
git grep -i 'stagehand' | grep -v 'plan/'      # Expected: ONLY "PRD.md:2366 …" (1 line).
```

### Level 3: Stale-dir / stale-file + full regression

```bash
# Stale dirs (must be absent):
ls -d cmd/stagehand pkg/stagehand 2>/dev/null || echo "clean: no stale stagehand dirs"
# Stale source files (must be none outside plan/012):
git ls-files | grep -i 'stagehand\.go$' | grep -v 'plan/012_963e3918ec08/' || echo "clean: no stale stagehand.go"
# Module path:
head -1 go.mod                       # Expected: "module github.com/dustin/stagecoach"

# Full build + test (the rename's whole-repo regression check; plan/ not compiled):
go build ./... && go vet ./... && go test ./...      # Expect all PASS.
```

### Level 4: Exception accounting + scope audit (documentation completeness)

```bash
# The 3 legitimate exception categories — each EXPECTED non-zero (document, don't fix):
echo "plan/012 rename docs (PRESERVE):    $(git grep -i 'stagehand' -- 'plan/012_963e3918ec08/' | wc -l)"
echo "tasks.json orchestrator (FORBIDDEN): $(git grep -i 'stagehand' | grep -c 'tasks\.json')"
echo "PRD directive (READ-ONLY):          $(git grep -i 'stagehand' -- PRD.md | wc -l)"
# Expected: plan/012 >0; tasks.json >0; PRD.md == 1. All accounted for in the implementation summary.

# Scope — ONLY the 3 files changed; nothing forbidden touched:
git status --short                                                   # Expect exactly 3 modified files.
git status --porcelain | grep -cE 'tasks\.json|^.. PRD\.md|plan/012_963e3918ec08'   # Expect 0 (untouched).
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1: `go build ./... && go vet ./...` clean; `goreleaser check` valid; `.golangci.yml` path points
      at the existing `pkg/stagecoach/stagecoach_test.go`.
- [ ] Level 2: corrected audit == 0; ZONE-A (`git grep -i stagehand | grep -v plan/`) == 1 (PRD directive only).
- [ ] Level 3: no `cmd/stagehand` / `pkg/stagehand`; no `stagehand.go` outside plan/012; go.mod = stagecoach;
      `go build ./... && go vet ./... && go test ./...` green.
- [ ] Level 4: the 3 exception categories counted & documented; `git status` shows ONLY the 3 modified files.

### Feature Validation
- [ ] `internal/git/git.go:390` comment says `.git/STAGECOACH_EDITMSG` (matches finalize.go:78).
- [ ] `.goreleaser.yaml:1` comment says `Stagecoach release config`.
- [ ] `.golangci.yml:40` `path` is `pkg/stagecoach/stagecoach_test.go`.
- [ ] The corrected whole-repo audit returns 0 (the rename is complete on the shipped surface).
- [ ] The 3 legitimate exceptions (plan/012, tasks.json, PRD directive) are documented, not hidden.

### Code Quality Validation
- [ ] Each edit is MINIMAL (a single token: STAGEHAND_EDITMSG→STAGECOACH_EDITMSG; Stagehand→Stagecoach; the
      path). No surrounding text reworded.
- [ ] No production-code BEHAVIOR changed (git.go edit is a comment; finalize.go UNTOUCHED).
- [ ] The contract's literal (too-narrow) command was NOT run verbatim; the corrected audit was used, with
      the deviation documented (same precedent as P1.M5.T1.S1).
- [ ] Scope respected: PRD.md, tasks.json, plan/012, plan/001–011, and all other Go source UNTOUCHED.

### Documentation & Deployment
- [ ] DOCS: none (verification step — no new doc file). The exception accounting is recorded in the
      implementation summary, not a tracked file.
- [ ] The corrected audit command is recorded (so the next maintainer can re-run it without re-deriving the
      exclusion set).

---

## Anti-Patterns to Avoid

- ❌ **Don't run the contract's literal command verbatim.** Its exclusion (`plan/012/architecture/` ONLY) is
  too narrow and returns ~2654 today — it would flag ~2650 legitimate lines (plan/012 rename docs,
  forbidden tasks.json, the PRD directive) as failures. Run the CORRECTED audit (excludes the whole plan/012
  tree + tasks.json + PRD.md). Same precedent as P1.M5.T1.S1. (findings §1)
- ❌ **Don't edit finalize.go.** The runtime constant is ALREADY `STAGECOACH_EDITMSG`. The ONLY straggler is
  the COMMENT in `internal/git/git.go:390`. "Fixing" finalize.go would reintroduce the bug. (findings §2/§6)
- ❌ **Don't touch the 3 exception categories.** plan/012 (rename docs — PRESERVE; renaming corrupts them),
  tasks.json (orchestrator-owned — FORBIDDEN), PRD.md (human-owned — READ-ONLY). Achieving "0" by editing
  any of these is a violation, not a success. Document them instead. (findings §1/§7)
- ❌ **Don't widen beyond the 3 fixes.** M1–M4 already renamed the rest (Go identifiers/imports, config
  values, paths, docs). The ZONE-A enumeration is exactly 4 lines (3 fixes + 1 PRD exception). If a 5th
  shipped-surface straggler appears, fix it — but do NOT re-run a broad `sed` across the repo (it would
  corrupt plan/012 and can't touch tasks.json anyway).
- ❌ **Don't conflate `git grep` with `grep -ri`.** `git grep` searches only tracked files (no .git/, no
  gitignored output) — no `.git/` guard needed. The rename_surface_map Gate 5 uses `grep -ri .` (which DOES
  need `grep -v '.git/'`). Match the contract's tool: `git grep`. (gotcha)
- ❌ **Don't try to "fix" the local checkout directory name** `/home/dustin/projects/stagehand`. It is
  untracked, invisible to `git grep`, and ships in no artifact. It has zero bearing on rename completeness.
- ❌ **Don't migrate the .golangci.yml schema to v2.** CI pins v1.61; v2 rejects the v1 schema. The path edit
  is a plain string valid in both. Schema migration is out of scope (a separate release-engineering task).
- ❌ **Don't skip `goreleaser check`.** The `.goreleaser.yaml` edit is comment-only and can't break the
  schema, but the check is cheap insurance and the file's own header documents it as the validation command.
- ❌ **Don't hide the no-op-ish nature.** If the 3 fixes are already landed (state drift) and the audit is
  already 0, say so — but STILL run the corrected audit + exception accounting + stale checks to CERTIFY it.
  Verification is the deliverable, not file churn.
