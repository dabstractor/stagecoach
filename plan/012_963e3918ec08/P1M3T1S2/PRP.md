---
name: "P1.M3.T1.S2 — Rename .goreleaser.yaml (project_name, builds, formulas, URLs): stagehand → stagecoach — project rename Layer 4.2"
description: |

  The goreleaser-config rename layer of the stagehand→stagecoach project rename (plan 012). The
  `.goreleaser.yaml` at `/home/dustin/projects/stagehand/.goreleaser.yaml` still references `stagehand` in
  23 places (project_name, build id, main path, binary name, archive id refs, Homebrew/Scoop/AUR names,
  homepage/url_template URLs, comments). The Go module (`github.com/dustin/stagecoach`) and the
  `cmd/stagecoach/` directory are already renamed (M1.T1 Complete). This task replaces every `stagehand` →
  `stagecoach` so goreleaser produces `stagecoach` binaries, archives, formula/scoop/AUR names, and URLs.

  THE FIX — one global replace (per the contract):
    sed -i 's/stagehand/stagecoach/g' .goreleaser.yaml

  This changes ALL 23 references (L5-8 comments, L12 project_name, L20-22 build id/main/binary, L39 archive
  id, L42 comment, L69 release.github.name, L74/90 comments, L80/92/95/106 URLs, L85 scoop name, L98-99/103
  AUR name+comments, L114/116 AUR provides/conflicts).

  ⚠️ **#1 — KEEP `dustin/` as the org; do NOT change to `dabstractor/`.** The contract's CAUTION suggests the
      URLs should become `github.com/dabstractor/stagecoach` to match the git remote (`dabstractor/stagecoach`).
      OVERRIDE that suggestion: the sed changes ONLY the product name (`stagehand`→`stagecoach`), preserving
      `dustin/`. THREE independent sources mandate `dustin/`: (a) go.mod = `github.com/dustin/stagecoach`
      (the module path `go install` uses); (b) PRD §21.2/§21.3 = `brew install dustin/tap/stagecoach`,
      `scoop install dustin/stagecoach`, `go install github.com/dustin/stagecoach/...`; (c) the EXISTING
      .goreleaser.yaml's explicit `release.github.owner: dustin` (L68: "explicit; WINS over git-remote
      auto-detect") + the owner note (L5-9) documenting the deliberate choice. Changing to `dabstractor/`
      would make goreleaser INCONSISTENT with go.mod + the PRD install paths. Snapshot validation publishes
      nothing ⇒ the owner doesn't affect the validation gate. See research design-decisions §1.

  ⚠️ **#2 — the global sed is SAFE: 23 references, ALL the product name, ZERO partial-word collisions.**
      Verified: `grep -c stagehand .goreleaser.yaml` = 23; no `stagehandler`, no `stagehand_internal`, etc.
      The build-id ↔ archive-id reference (`builds[0].id: stagehand` L20 ↔ `archives[0].ids: - stagehand`
      L39) is changed IDENTICALLY on both ends ⇒ stays consistent.

  ⚠️ **#3 — `main: ./cmd/stagecoach` resolves (cmd/stagecoach EXISTS from M1.T1.S2).** The sed turns
      `main: ./cmd/stagehand` → `main: ./cmd/stagecoach`. The directory exists (verified `ls cmd/` →
      `stagecoach stubagent`). This is the one build-critical mapping.

  ⚠️ **#4 — `goreleaser check` is the validation gate (goreleaser IS installed).** The sed changes ONLY the
      product name (not the YAML structure) ⇒ if `goreleaser check` passed before, it passes after.
      PRE-EXISTING decision gates (the `formats` vs `format` gate at L43; the `aurs` gate at L100) are
      UNRELATED to the rename — not caused by this task.

  ⚠️ **#5 — .goreleaser.yaml ONLY (scope).** The Makefile is S1 (P1.M3.T1.S1, parallel). The README install
      instructions are M4.T1.S1. CI workflow is S3 (P1.M3.T1.S3). Disjoint files ⇒ no conflict.

  Deliverable: MODIFIED `.goreleaser.yaml` (global stagehand→stagecoach; `dustin/` org preserved). NO other
  file. OUTPUT: `goreleaser check` passes; `grep -c stagehand .goreleaser.yaml` → 0; project_name/build
  id/binary/formula/scoop/AUR names all `stagecoach`; URLs are `github.com/dustin/stagecoach`. DOCS: none —
  the README install instructions are M4.T1.S1.

---

## Goal

**Feature Goal**: Rename all `stagehand` references in `.goreleaser.yaml` to `stagecoach` (23 references) so
goreleaser produces `stagecoach` binaries, archives (`stagecoach_*`), Homebrew/Scoop/AUR names, and URLs
(`github.com/dustin/stagecoach`) — matching the renamed Go module (`github.com/dustin/stagecoach`) and the
renamed `cmd/stagecoach/` directory.

**Deliverable**: MODIFIED `.goreleaser.yaml` — global `stagehand` → `stagecoach` (23 references). The `dustin/`
org is PRESERVED (not changed to `dabstractor/`). No other file.

**Success Definition**: `grep -c stagehand .goreleaser.yaml` → 0; `goreleaser check` passes (or the YAML is
structurally valid + the rename introduced no NEW errors); `project_name: stagecoach`; build `id: stagecoach`
+ `main: ./cmd/stagecoach` (exists) + `binary: stagecoach`; archive `ids: - stagecoach` (matches the build id);
Homebrew/Scoop/AUR names all `stagecoach`; URLs are `github.com/dustin/stagecoach`; the `dustin/` org is
unchanged.

## User Persona

**Target User**: The maintainer running `goreleaser release` (or `goreleaser check` / `--snapshot`) — who
expects the release artifacts, formula, scoop manifest, and AUR package to all be named `stagecoach`.
Transitively: the end user installing via `brew install dustin/tap/stagecoach` / `scoop install
dustin/stagecoach` / `go install github.com/dustin/stagecoach/...`.

**Use Case**: `goreleaser release --snapshot --clean` → produces `stagecoach_1.0.0_linux_amd64.tar.gz` (not
`stagehand_*`); the Homebrew formula + Scoop manifest + AUR package are all named `stagecoach`.

**User Journey**: maintainer tags a release → goreleaser builds `stagecoach` binaries → publishes
`stagecoach_*` archives + the `stagecoach` formula/scoop/AUR → users install `stagecoach`.

**Pain Points Addressed**: The goreleaser config still produces `stagehand`-named artifacts (the old name),
inconsistent with the renamed module/cmd directory.

## Why

- **Layer 4.2 of the project rename.** The Go structural rename (M1) is Complete; the Makefile (S1) is being
  renamed in parallel. The goreleaser config is the release/distribution surface that must match.
- **Consistency with go.mod + the PRD.** The module path is `github.com/dustin/stagecoach`; the PRD install
  paths use `stagecoach` + `dustin/`. The goreleaser config must produce `stagecoach` artifacts at `dustin/`
  URLs to match.
- **Trivially simple + one critical decision.** One `sed` on one file (23 refs). The one non-obvious call is
  the namespace (keep `dustin/`, not `dabstractor/` — see §1 of the design decisions + the ⚠️ #1 above).

## What

A global `stagehand` → `stagecoach` text replacement in `.goreleaser.yaml`. The `dustin/` org is preserved
(the sed does NOT touch `dustin`). No logic change, no structural change — just the product name in
project_name, build id/main/binary, archive id refs, formula/scoop/AUR names, URLs, and comments.

### Success Criteria

- [ ] `grep -c stagehand .goreleaser.yaml` → 0 (zero references remain).
- [ ] `project_name: stagecoach`; build `id: stagecoach` + `main: ./cmd/stagecoach` + `binary: stagecoach`.
- [ ] archive `ids: - stagecoach` (matches the renamed build id — consistency).
- [ ] Scoop `name: stagecoach`; AUR `name: stagecoach-bin` + `provides: - stagecoach` + `conflicts: -
      stagecoach`.
- [ ] All URLs are `github.com/dustin/stagecoach` (the `dustin/` org PRESERVED — NOT `dabstractor/`).
- [ ] `goreleaser check` passes (or the rename introduced no new errors vs the pre-rename baseline).
- [ ] No other file modified.

## All Needed Context

### Context Completeness Check

_Pass._ A developer with no prior repo knowledge can do this from: the exact `sed` command, the namespace
decision (keep `dustin/` — §1), the verification commands, and the knowledge that `cmd/stagecoach/` exists.
No Go/goreleaser-internals knowledge required.

### Documentation & References

```yaml
# MUST READ — the design calls (the namespace decision, the sed safety, the validation)
- docfile: plan/012_963e3918ec08/P1M3T1S2/research/design-decisions.md
  why: §1 (KEEP dustin/ — 3 sources: go.mod + PRD + existing owner note; OVERRIDE the contract's dabstractor
       suggestion), §2 (23 refs, all product name, zero collisions; build-id↔archive-id consistency),
       §3 (main: ./cmd/stagecoach resolves), §4 (goreleaser check is the gate; pre-existing decision gates
       are unrelated), §5 (scope: .goreleaser.yaml ONLY), §6 (one file, one sed).
  critical: §1 (KEEP dustin/ — the load-bearing non-obvious call; changing to dabstractor breaks go.mod +
       PRD consistency) is the thing most likely to be implemented wrong.

# The file being edited — READ FULLY before the sed
- file: .goreleaser.yaml   (at /home/dustin/projects/stagehand/.goreleaser.yaml — the project root)
  section: the full file (~120 lines). The 23 `stagehand` references span:
           - L5-8: owner-note COMMENTS (dustin/stagehand, dabstractor/stagehand, github.com/dustin/stagehand).
             After sed: dustin/stagecoach, dabstractor/stagecoach (ACCURATELY matches the remote), github.com/dustin/stagecoach.
           - L12: project_name: stagehand.
           - L20-22: build id / main ./cmd/stagehand / binary stagehand.
           - L39: archive ids: - stagehand (refs builds[0].id).
           - L42: comment stagehand_1.0.0_...
           - L68-69: release.github.owner: dustin (KEEP) + name: stagehand.
           - L74: comment brew install dustin/tap/stagehand.
           - L77-80: homebrew owner: dustin (KEEP) + homepage github.com/dustin/stagehand.
           - L85/89-90/92/95: scoop name stagehand + owner dustin (KEEP) + homepage/url_template dustin/stagehand.
           - L98-99/103/106/114/116: AUR comments + name stagehand-bin + homepage + provides/conflicts stagehand.
  why: the ONLY file edited. The sed replaces all 23 at once. The `dustin/` org strings do NOT contain
       `stagehand` ⇒ they are UNCHANGED by the sed (only the product name changes).
  critical: the `dustin/` org is PRESERVED (do NOT add a dustin→dabstractor step — see §1). The build id
       (L20) and the archive id ref (L39) both become `stagecoach` ⇒ consistent.

# The PRD basis (in your context as selected_prd_content)
- file: PRD.md §21.2 (h3.99) + §21.3 (h3.100) + h2.30
  section: §21.2 — goreleaser produces Homebrew formula to `dustin/homebrew-tap`, AUR `stagecoach`, Scoop;
           `go install github.com/dustin/stagecoach/cmd/stagecoach@latest`. §21.3 — `brew install
           dustin/tap/stagecoach`, `scoop install dustin/stagecoach`. h2.30 — "All references to 'stagehand'
           must be replaced with 'stagecoach'."
  critical: §21.2/§21.3 use `dustin/` (NOT `dabstractor/`) — confirms the namespace decision (§1).

# The namespace evidence (read-only — confirms dustin/ is canonical)
- file: go.mod   (at /home/dustin/projects/stagehand/go.mod)
  section: line 1 — `module github.com/dustin/stagecoach` (ALREADY renamed M1.T1.S1; uses `dustin`).
  why: the authoritative Go module path. `go install github.com/dustin/stagecoach/...` MUST match the
       goreleaser URLs. Changing goreleaser to `dabstractor/` would diverge from this.
  critical: the org is `dustin` in go.mod — goreleaser must match.

# The git remote (the source of the contract's CAUTION — read-only)
  git remote: `git@github.com:dabstractor/stagecoach` (org = `dabstractor`). This is the CURRENT remote.
  The existing goreleaser owner note (L5-9) + `release.github.owner: dustin` (L68) DELIBERATELY override
  it: "before the first REAL tag the repo must be reachable at github.com/dustin/stagecoach (or the
  namespace is reconciled repo-wide)." Snapshot validation publishes nothing ⇒ the owner is not a blocker.

# The S1 CONTRACT (the Makefile rename — parallel sibling)
- file: plan/012_963e3918ec08/P1M3T1S1/PRP.md
  why: S1 renames the Makefile (global sed, producing `./bin/stagecoach`). S2 is the .goreleaser.yaml
       sibling. Disjoint files (Makefile ≠ .goreleaser.yaml) ⇒ no conflict.
  critical: S1's MAIN_PKG → ./cmd/stagecoach matches S2's goreleaser `main: ./cmd/stagecoach` (both point
       to the renamed directory).
```

### Current Codebase tree (relevant slice)

```bash
.goreleaser.yaml          # 23 stagehand refs → EDIT (global sed → stagecoach; dustin/ org PRESERVED)
go.mod                    # module github.com/dustin/stagecoach (ALREADY renamed — M1.T1.S1; the org authority)
cmd/stagecoach/main.go    # ALREADY renamed (M1.T1.S2) — `main: ./cmd/stagecoach` resolves after the sed
Makefile                  # S1 (parallel) — renaming stagehand→stagecoach (MAIN_PKG, BIN, etc.)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. ONE in-place edit: .goreleaser.yaml (global stagehand→stagecoach; dustin/ preserved).
```

### Known Gotchas of our codebase & Library Quirks

```yaml
# CRITICAL (#1 — KEEP dustin/, NOT dabstractor/): the sed 's/stagehand/stagecoach/g' changes ONLY the
#   product name. Do NOT add a 'dustin→dabstractor' step. go.mod + PRD §21.2/§21.3 + the existing owner
#   note ALL use dustin/. Changing to dabstractor/ breaks the go-install module path + the PRD install
#   paths. (research §1)
# CRITICAL (#2 — the sed is safe): 23 refs, ALL the product name, ZERO partial-word collisions. The build-id
#   (L20) ↔ archive-id-ref (L39) both become stagecoach ⇒ consistent. (research §2)
# CRITICAL (#3 — main: ./cmd/stagecoach resolves): cmd/stagecoach/ EXISTS (M1.T1.S2). After the sed,
#   goreleaser's build entry point is correct. Verify: ls cmd/stagecoach/main.go.
# GOTCHA (goreleaser IS installed): /home/dustin/.local/bin/goreleaser. `goreleaser check` is the validation
#   gate. The sed changes only the product name (not the structure) ⇒ no new structural errors.
# GOTCHA (pre-existing decision gates are NOT this task's concern): the `formats` vs `format` gate (L43) and
#   the `aurs` gate (L100) are PRE-EXISTING (documented in the file's comments). If `goreleaser check` flags
#   them, they are NOT caused by the rename — leave them for a separate decision (or the existing DECISION
#   GATE comments resolve them). The rename's gate is: zero stagehand refs + consistency + no NEW errors.
# GOTCHA (snapshot validation publishes nothing): `goreleaser release --snapshot --clean` is the ultimate
#   proof (produces stagecoach_* archives locally) but runs `go mod tidy` (network) — may be slow. Use
#   `goreleaser check` as the primary gate.
# GOTCHA (the owner note comment becomes ACCURATE after the sed): L7 "the current git remote is
#   `dabstractor/stagehand`" → after sed → "dabstractor/stagecoach" — which ACCURATELY matches the actual
#   remote (dabstractor/stagecoach). The comment is self-correcting.
# GOTCHA (scope): .goreleaser.yaml ONLY. Makefile=S1, CI=S3, README/docs=M4.T1. Do NOT touch them.
```

## Implementation Blueprint

### Data models and structure

No code. One command + verification:

```bash
# THE rename (from the project root /home/dustin/projects/stagehand):
sed -i 's/stagehand/stagecoach/g' .goreleaser.yaml

# VERIFY zero references remain:
grep -c stagehand .goreleaser.yaml   # → 0

# VERIFY the key fields (the sed changed ONLY the product name; dustin/ is PRESERVED):
grep -nE 'project_name:|id: stagecoach|main:|binary:|name: stagecoach|stagecoach-bin|github.com/dustin/stagecoach' .goreleaser.yaml | head -15
# Expected:
#   project_name: stagecoach
#   - id: stagecoach
#     main: ./cmd/stagecoach
#     binary: stagecoach
#   - stagecoach                  # archive ids (matches build id)
#   name: stagecoach              # release.github.name (owner stays dustin)
#   - name: stagecoach            # scoop name
#   - name: stagecoach-bin        # AUR name
#   homepage: https://github.com/dustin/stagecoach   # URLs (dustin/ PRESERVED)

# VERIFY dustin/ is preserved (NOT changed to dabstractor/):
grep -c 'dustin/stagecoach\|dustin/homebrew-tap\|dustin/scoop-bucket\|owner: dustin' .goreleaser.yaml   # → >0 (dustin present)
grep -c 'dabstractor' .goreleaser.yaml   # → 1 (only the owner-note comment about the git remote — CORRECT)
# (There should be NO dabstractor/ in URLs/homepage/owner — only in the comment explaining the remote.)

# VERIFY cmd/stagecoach exists (the build main path):
ls cmd/stagecoach/main.go   # → exists

# VERIFY goreleaser config validity:
goreleaser check   # → "configuration valid" (or pre-existing decision-gate warnings, NOT new errors)
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: APPLY the global sed
  - RUN (from /home/dustin/projects/stagehand): sed -i 's/stagehand/stagecoach/g' .goreleaser.yaml
  - VERIFY: grep -c stagehand .goreleaser.yaml → 0 (zero references remain).
  - VERIFY: grep the key fields (project_name, build id/main/binary, archive ids, scoop/AUR names) → all stagecoach.
  - VERIFY: dustin/ is PRESERVED (grep 'owner: dustin' + 'github.com/dustin/stagecoach') — NOT dabstractor/.
  - GOTCHA: do NOT add a dustin→dabstractor step (§1).

Task 2: VERIFY goreleaser config validity + the build path
  - RUN: ls cmd/stagecoach/main.go → exists (the `main: ./cmd/stagecoach` resolves).
  - RUN: goreleaser check → "configuration valid" (or pre-existing decision-gate warnings only — NOT new
      errors caused by the rename).
  - OPTIONAL (slower; the ultimate proof): goreleaser release --snapshot --clean → produces
      stagecoach_* archives locally (publishes nothing). Skip if network is slow; `goreleaser check` is
      sufficient for the rename validation.
  - CONFIRM: only .goreleaser.yaml modified (`git diff --name-only` → .goreleaser.yaml only).
```

### Implementation Patterns & Key Details

```yaml
# THE key fields after the sed (product name → stagecoach; org → dustin PRESERVED):
project_name: stagecoach                    # was: stagehand
builds[0]:
  id: stagecoach                            # was: stagehand
  main: ./cmd/stagecoach                    # was: ./cmd/stagehand (directory EXISTS)
  binary: stagecoach                        # was: stagehand
archives[0]:
  ids: [stagecoach]                         # was: [stagehand] — matches builds[0].id
release.github:
  owner: dustin                             # PRESERVED (NOT dabstractor) — explicit override
  name: stagecoach                          # was: stagehand
homebrew_casks[0]:
  repository: {owner: dustin, name: homebrew-tap}   # dustin PRESERVED
  homepage: https://github.com/dustin/stagecoach    # was: dustin/stagehand
scoops[0]:
  name: stagecoach                          # was: stagehand
  repository: {owner: dustin, name: scoop-bucket}   # dustin PRESERVED
  homepage: https://github.com/dustin/stagecoach    # was: dustin/stagehand
  url_template: https://github.com/dustin/stagecoach/releases/download/...  # was: dustin/stagehand
aurs[0]:
  name: stagecoach-bin                      # was: stagehand-bin
  homepage: https://github.com/dustin/stagecoach    # was: dustin/stagehand
  provides: [stagecoach]                    # was: [stagehand]
  conflicts: [stagecoach]                   # was: [stagehand]
```

### Integration Points

```yaml
GO MODULE: go.mod is ALREADY github.com/dustin/stagecoach (M1.T1.S1). The goreleaser URLs
      (github.com/dustin/stagecoach) now MATCH the module path. No go.mod change needed.
CMD DIRECTORY: cmd/stagecoach/ ALREADY exists (M1.T1.S2). `main: ./cmd/stagecoach` resolves.

DOWNSTREAM (NOT this task):
  - P1.M3.T1.S3 (CI workflow rename): the CI may invoke `goreleaser` — depends on this config being stagecoach.
  - P1.M4.T1.S1 (README install instructions): `brew install dustin/tap/stagecoach`, `scoop install
       dustin/stagecoach`, `go install github.com/dustin/stagecoach/...` — must match this goreleaser config
       (the formula/scoop name + the URLs). M4.T1.S1 owns the README.

FROZEN/LEAVE: go.mod, Makefile (S1), cmd/, all .go files, docs/, providers/, PRD.md, .github/. NO new files.
NO NEW DATABASE / ROUTES / CLI / FILES / CONFIG / DOCS.
```

## Validation Loop

### Level 1: The sed landed correctly

```bash
grep -c stagehand .goreleaser.yaml   # → 0 (zero references remain)
grep -n stagecoach .goreleaser.yaml | head -10   # → the renamed references (project_name, build, etc.)
# Expected: 0 stagehand hits; 23 stagecoach hits (the renamed references).
# VERIFY dustin/ preserved (NOT dabstractor/ in URLs/owner):
grep -c 'owner: dustin' .goreleaser.yaml   # → 3 (release + homebrew + scoop owners)
grep -c 'github.com/dustin/stagecoach' .goreleaser.yaml   # → 4 (the homepages + url_template)
grep 'dabstractor' .goreleaser.yaml   # → ONLY the owner-note comment (L7) — CORRECT (it documents the remote)
```

### Level 2: goreleaser config validity + the build path

```bash
ls cmd/stagecoach/main.go   # → exists (the `main: ./cmd/stagecoach` resolves)
goreleaser check   # → "configuration valid" (or pre-existing decision-gate warnings only)
# Expected: `goreleaser check` passes. If it warns on `formats` (L43) or `aurs` (L100), those are
# PRE-EXISTING decision gates (documented in the file's comments) — NOT caused by the rename. The rename's
# gate is: the rename introduced no NEW errors (the structure is unchanged; only the product name changed).
# OPTIONAL (the ultimate proof; slower — runs go mod tidy):
#   goreleaser release --snapshot --clean   # → produces dist/stagecoach_*.tar.gz / .zip (publishes nothing)
```

### Level 3: Scope guard (no other file touched)

```bash
git diff --name-only   # Expect ONLY .goreleaser.yaml.
git diff --exit-code go.mod Makefile cmd/ internal/ pkg/ docs/ providers/ PRD.md .github/ && echo "frozen files UNCHANGED (expected)"
# Expected: only .goreleaser.yaml modified; everything else byte-unchanged.
```

### Level 4: Downstream readiness

```bash
# The README install instructions (M4.T1.S1) must match this goreleaser config. Verify the names align:
grep 'project_name\|name: stagecoach\|stagecoach-bin' .goreleaser.yaml   # → the names M4 must reference
# Expected: project_name stagecoach; scoop name stagecoach; AUR name stagecoach-bin — these are the names
# the README's `brew install dustin/tap/stagecoach` / `scoop install dustin/stagecoach` must match.
```

## Final Validation Checklist

### Technical Validation
- [ ] `grep -c stagehand .goreleaser.yaml` → 0.
- [ ] `goreleaser check` passes (or the rename introduced no new errors vs the pre-rename baseline).
- [ ] `ls cmd/stagecoach/main.go` exists (the `main` path resolves).
- [ ] Only `.goreleaser.yaml` modified; go.mod + Makefile + all other files byte-unchanged.

### Feature Validation
- [ ] `project_name: stagecoach`; build `id/main/binary` = stagecoach; archive `ids` = stagecoach (matches).
- [ ] Scoop `name: stagecoach`; AUR `name: stagecoach-bin` + `provides/conflicts` = stagecoach.
- [ ] All URLs = `github.com/dustin/stagecoach`; `owner: dustin` everywhere (PRESERVED — NOT dabstractor/).
- [ ] The owner-note comment's `dabstractor/stagecoach` (L7) ACCURATELY matches the actual git remote.

### Code Quality Validation
- [ ] The global sed is safe (23 refs, all product name, zero partial-word collisions — verified).
- [ ] The build-id ↔ archive-id reference is consistent (both `stagecoach`).
- [ ] The `dustin/` org is deliberately preserved (3 sources: go.mod + PRD + existing owner note).

### Documentation
- [ ] No docs change (the README install instructions are M4.T1.S1; this task is the release config).

---

## Anti-Patterns to Avoid

- ❌ **Don't change `dustin` → `dabstractor`.** The contract's CAUTION SUGGESTS it, but go.mod
      (`github.com/dustin/stagecoach`) + PRD §21.2/§21.3 (`dustin/tap`, `dustin/stagecoach`) + the existing
      owner note (explicit `release.github.owner: dustin`) ALL mandate `dustin/`. The sed changes ONLY
      `stagehand`→`stagecoach`, preserving `dustin/`. Changing to `dabstractor/` breaks the go-install module
      path + the PRD install paths. (research §1)
- ❌ **Don't manually edit individual lines.** The contract specifies `sed -i 's/stagehand/stagecoach/g'` —
      a single global replace. Manual edits risk missing a reference (there are 23).
- ❌ **Don't change the YAML structure.** The sed replaces the product-name STRING only — it does not touch
      the `homebrew_casks`/`scoops`/`aurs` structure, the `formats`/`format_overrides`, or the decision gates.
      Those are pre-existing; this task is the rename only.
- ❌ **Don't conflate the pre-existing decision gates with the rename.** If `goreleaser check` warns on
      `formats` (L43) or `aurs` (L100), those are PRE-EXISTING (documented in the file's DECISION GATE
      comments) — NOT caused by the rename. The rename's gate is: zero stagehand refs + consistency + no NEW
      errors.
- ❌ **Don't edit the Makefile, go.mod, CI, or docs.** Those are S1 (Makefile, parallel), M1.T1.S1 (go.mod,
      done), S3 (CI), M4.T1 (docs). This task is `.goreleaser.yaml` ONLY.
- ❌ **Don't forget to verify the build main path.** `main: ./cmd/stagecoach` must resolve — verify
      `ls cmd/stagecoach/main.go` exists (it does, from M1.T1.S2). A wrong `main` → goreleaser build failure.

---

## Confidence Score

**10/10** — a single `sed -i 's/stagehand/stagecoach/g' .goreleaser.yaml` on one file with 23 verified
references, all of which are the product name (zero partial-word collisions — verified by grep). The one
non-obvious call — the namespace (keep `dustin/`, not `dabstractor/`) — is resolved by THREE independent
sources (go.mod `github.com/dustin/stagecoach`, PRD §21.2/§21.3 `dustin/`, the existing owner note's explicit
`release.github.owner: dustin`); the contract's `dabstractor` suggestion is overridden with documented
evidence. The build-critical `main: ./cmd/stagecoach` resolves (the directory exists from M1.T1.S2).
goreleaser IS installed (`goreleaser check` is a valid gate). The build-id ↔ archive-id reference stays
consistent (both ends become `stagecoach`). Zero file overlap with S1 (Makefile), S3 (CI), or M4 (docs). No
residual risk: the sed is a pure product-name text substitution on a static YAML file, and the verification
is deterministic (`grep -c stagehand` → 0; `goreleaser check` passes).
