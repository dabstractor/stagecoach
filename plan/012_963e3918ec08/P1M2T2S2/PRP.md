---
name: "P1.M2.T2.S2 — Rename .stagehandignore → .stagecoachignore (the last 2 .go residue sites)"
description: |
  Finish the `.stagehandignore` → `.stagecoachignore` rename in the **Go source**. The exclusion-file constant,
  its value, and the entire `internal/exclude/` package (+ tests) are ALREADY `.stagecoachignore` — a prior
  bulk rename (M1.T2.S1) converted them. **The item description's premise is STALE**: it says "edit
  `exclude.go:28` `.stagehandignore` → `.stagecoachignore`", but `exclude.go:28` ALREADY reads
  `const StagecoachIgnoreFile = ".stagecoachignore"` and `grep -c stagehandignore internal/exclude/exclude.go`
  is 0. The REAL remaining work is **exactly 2 `.go` sites** the bulk pass missed:
    1. `internal/cmd/root.go:164` — `.stagehandignore` in the `--exclude` flag HELP TEXT.
    2. `internal/ui/verbose.go:101` — `.stagehandignore` in the `VerboseWarn` DOC COMMENT.
  Both are user-facing/comment strings; zero behavioral change; no test asserts on either.

  ⚠️ **THE central design call — do NOT edit `internal/exclude/exclude.go` (it's already done).** Verify first:
  `grep -c stagehandignore internal/exclude/exclude.go` → 0; the const value (L28), the package doc, every
  comment, and `exclude_test.go` all already say `.stagecoachignore`. The item's "INPUT: const NAME renamed,
  VALUE must change" describes a state that no longer exists. Editing exclude.go would be a no-op (the string
  isn't there) and would waste effort / risk a spurious diff. See research §0.

  ⚠️ **THE second design call — `.go`-ONLY scope; docs are deferred to M4 (P1.M4.T1).** The item's LOGIC (b)
  sed is `--include='*.go'`; its DOCS line says "update README.md and docs/cli.md `.stagecoachignore`
  references **in M4**." So `README.md:66`, `docs/cli.md:37`, `docs/README.md:34/42`, `docs/how-it-works.md:
  156/158/162`, `docs/configuration.md:253/255/260/277` are **P1.M4.T1's scope — do NOT touch them.** The
  `bin/*` + root `stagehand`/`stagecoach` grep hits are compiled build artifacts — ignore (they rebuild clean).

  ⚠️ **THE third design call — zero test impact; the rename is comment/help-text only.** `grep` confirms NO
  test asserts on either residue string (the only `.go` hits are the 2 source sites themselves).
  `exclude_test.go` already uses `.stagecoachignore` → `go test ./internal/exclude/... -count=1` passes BEFORE
  and AFTER this task (the 2 edits don't touch exclude at all). So this is a pure string rename with no
  behavioral or test consequence.

  ⚠️ **THE codebase-location gotcha.** The codebase is at **`/home/dustin/projects/stagehand`** (on-disk dir
  name unchanged; the Go MODULE is already `github.com/dustin/stagecoach` — `head -1 go.mod` confirms). The
  plan-staging cwd `/home/dustin/projects/stagecoach` holds only `plan/`. Run ALL commands from
  `/home/dustin/projects/stagehand`. (Matches S1's note.)

  ⚠️ **Disjoint from the parallel S1 (P1.M2.T2.S1, config/lock paths).** S1 edits `internal/config/*` +
  `internal/lock/*` (path literals + `.stagecoach.toml`). This task edits `internal/cmd/root.go` +
  `internal/ui/verbose.go` (the exclusion filename). **Zero file overlap.** S1's PRP explicitly listed
  `.stagecoachignore` as "P1.M2.T2.S2 (a SIBLING task)."

  Deliverable: 2 single-token string edits (`stagehandignore` → `stagecoachignore`) in
  `internal/cmd/root.go:164` + `internal/ui/verbose.go:101`. OUTPUT: zero `.stagehandignore` residue in `.go`;
  `go test ./internal/exclude/... -count=1` passes. DOCS: none here (docs are M4). No new files, no logic
  change, no deps.
---

## Goal

**Feature Goal**: Eliminate the last 2 `.stagehandignore` references in the Go source so the exclusion file is
consistently `.stagecoachignore` everywhere in `.go` (constant value, comments, help text). The constant +
`internal/exclude/` are already done; this closes the residue in the `--exclude` help text + a verbose doc comment.

**Deliverable** (2 single-token edits to 2 existing files — NO new files):
1. `internal/cmd/root.go:164` — `.stagehandignore` → `.stagecoachignore` in the `--exclude` flag help string.
2. `internal/ui/verbose.go:101` — `.stagehandignore` → `.stagecoachignore` in the `VerboseWarn` doc comment.

**Success Definition**: `grep -rn 'stagehandignore' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents'`
returns ZERO; `gofmt -l`, `go vet ./...`, `go build ./...` clean; `go test ./internal/exclude/... -count=1` +
`go test ./... -count=1` green; go.mod/go.sum unchanged; only the 2 files touched; docs (README/docs/*.md)
UNTOUCHED (P1.M4.T1's scope).

## User Persona

**Target User**: Users who read `stagecoach --help` (the `--exclude` line now names `.stagecoachignore`) and
developers reading the `VerboseWarn` doc. Transitively PRD §9.18 (payload exclusions) + h2.30 (the rename mandate).

**Use Case**: A user runs `stagecoach --help` and sees `--exclude` "unions with `.stagecoachignore`" (matching
the actual filename the loader reads). Today it says `.stagehandignore` — a stale name that doesn't match the
file stagecoach actually looks for.

**User Journey**: `stagecoach --help` → `--exclude` line names `.stagecoachignore` (consistent with the
`StagecoachIgnoreFile` constant + the docs that will follow in M4).

**Pain Points Addressed**: removes the last user-visible `.stagehandignore` reference in the binary's own help
text, so the tool's self-description matches the file it reads.

## Why

- **Completes the `.go` rename surface for the exclusion file.** The constant + loader + tests are already
  `.stagecoachignore`; the `--exclude` help text + a doc comment are the only `.go` residue. h2.30 mandates
  the rename; this closes the Go side.
- **User-facing consistency.** `stagecoach --help` is the most-read surface; it must name the actual file
  (`.stagecoachignore`), not the old name (`.stagehandignore`). A user who creates `.stagehandignore` because
  the help said so would find their exclusions silently ignored.
- **Zero risk.** Two comment/help-text string edits; no behavioral change; no test asserts on either string.
- **Scope-disciplined.** `.go`-only; docs deferred to M4 (P1.M4.T1) per the item; no overlap with the parallel
  S1 (config/lock paths). go.mod unchanged.

## What

Rename `stagehandignore` → `stagecoachignore` at exactly 2 `.go` sites. No new files, no logic change, no
docs, no tests added.

### Success Criteria

- [ ] `internal/cmd/root.go:164` help text says `.stagecoachignore` (was `.stagehandignore`).
- [ ] `internal/ui/verbose.go:101` doc comment says `.stagecoachignore` (was `.stagehandignore`).
- [ ] `grep -rn 'stagehandignore' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents'` → **EMPTY**.
- [ ] `internal/exclude/exclude.go` UNCHANGED (already `.stagecoachignore`; do NOT edit it).
- [ ] `gofmt -l internal/cmd/root.go internal/ui/verbose.go` clean; `go vet ./...` + `go build ./...` clean.
- [ ] `go test ./internal/exclude/... -count=1` PASS; `go test ./... -count=1` green (no regression).
- [ ] go.mod/go.sum byte-unchanged; only `internal/cmd/root.go` + `internal/ui/verbose.go` touched.
- [ ] Docs UNTOUCHED: `README.md:66`, `docs/cli.md`, `docs/README.md`, `docs/how-it-works.md`,
      `docs/configuration.md` still reference `.stagehandignore` (P1.M4.T1's scope — NOT this task).

## All Needed Context

### Context Completeness Check

_Pass._ A developer with no prior repo knowledge can implement this from: the reality-check (exclude.go is
already done — don't touch it), the 2 exact sites (quoted below), the `.go`-only scope (docs → M4), the
codebase-location note, and the verification grep. No feature knowledge beyond "rename these 2 strings."

### Documentation & References

```yaml
# MUST READ — the authoritative research (reality check + the 2 sites + scope)
- docfile: plan/012_963e3918ec08/P1M2T2S2/research/stagecoachignore-residue.md
  why: §0 (exclude.go is ALREADY .stagecoachignore — the item's premise is stale; do NOT edit it), §1 (the
       exact 2 residue sites with old→new), §2 (.go-only; docs deferred to M4), §3 (no test impact), §4
       (disjoint from S1), §5 (mechanism + verification), §6 (codebase location).
  critical: §0 (don't edit exclude.go) + §1 (the 2 sites) + §2 (don't touch docs).

# The contract — what the parallel S1 does (disjointness proof)
- docfile: plan/012_963e3918ec08/P1M2T2S1/PRP.md
  why: S1 renames config-discovery PATHS (internal/config/*) + lock PATHS (internal/lock/*). This task
       renames the exclusion filename in internal/cmd/root.go + internal/ui/verbose.go. ZERO file overlap.
       S1's PRP explicitly lists ".stagecoachignore — P1.M2.T2.S2 (a SIBLING task)."
  critical: do NOT duplicate S1's path/filename work; do NOT touch internal/config/ or internal/lock/.

# The PRD basis
- file: PRD.md §9.18 (h3.34 "Payload exclusions") — FR-X1b names the `.stagecoachignore` file; FR-X2 its syntax.
       h2.30 — the rename mandate ("All references to stagehand must be replaced with stagecoach").
  why: the canonical filename is `.stagecoachignore`; the rename is mandated. The 2 sites rename to match.

# The files EDITED (read the exact current text before editing)
- file: internal/cmd/root.go
  section: the `--exclude`/`-x` flag registration (~L163-166); the help string at L164.
  why: the user-facing help text names the wrong file (`.stagehandignore`). Rename to `.stagecoachignore`.
  pattern: the string spans two concatenated lines (L164 + L165); edit the L164 token in place.
  gotcha: do NOT touch the nearby `stagecoach.commits` (L168) or any other line — ONLY the `stagehandignore`
           token on L164. The `--exclude` flag's LOGIC is unchanged (it already unions via the loader).

- file: internal/ui/verbose.go
  section: the `VerboseWarn` doc comment (~L101-102).
  why: the doc comment names the wrong file. Rename to `.stagecoachignore`.
  gotcha: the runtime string VerboseWarn prints comes from exclude.go:85 (already `.stagecoachignore`) — ONLY
           this doc comment (L101) is stale. Do NOT change VerboseWarn's body or signature.

# READ-ONLY context (do NOT edit)
- file: internal/exclude/exclude.go
  section: StagecoachIgnoreFile (L27-28) + the package doc + LoadStagecoachIgnore + ResolveExcludePathspecs.
  why: PROOF the constant + value + all comments are ALREADY `.stagecoachignore` (`grep -c stagehandignore` → 0).
       This task does NOT edit exclude.go. Read it only to confirm the rename is already done.
  critical: if `grep -c stagehandignore internal/exclude/exclude.go` is NOT 0, the premise changed — re-check.
- file: internal/exclude/exclude_test.go
  why: uses `StagecoachIgnoreFile` const + asserts `.stagecoachignore` (L99/151/161/162/193/223) — already
       renamed; the tests pass as-is. This task does NOT edit it.

# OUT OF SCOPE (do NOT touch — P1.M4.T1 / M3 / M5 own these)
- README.md:66 + docs/cli.md:37 + docs/README.md:34/42 + docs/how-it-works.md:156/158/162 +
  docs/configuration.md:253/255/260/277 — all `.stagehandignore` doc references → P1.M4.T1 (Mode A docs ride
  with M4, per the item's DOCS line).
- bin/* + root stagehand/stagecoach binaries — compiled build artifacts (rebuild clean from renamed source).
- internal/config/* + internal/lock/* — S1 (parallel; config/lock PATHS).
- Makefile / .goreleaser.yaml / providers/*.toml / .github/workflows — P1.M3 / P1.M4.
```

### Current Codebase tree (relevant slice)

```bash
# Codebase root: /home/dustin/projects/stagehand   (module github.com/dustin/stagecoach; on-disk name unchanged)
internal/exclude/
  exclude.go            # StagecoachIgnoreFile = ".stagecoachignore" (L28) — ALREADY DONE; NO edit (READ-ONLY proof)
  exclude_test.go       # asserts .stagecoachignore — ALREADY DONE; NO edit
internal/cmd/
  root.go               # --exclude help text L164 (.stagehandignore)  ← EDIT (the one residue site #1)
internal/ui/
  verbose.go            # VerboseWarn doc comment L101 (.stagehandignore)  ← EDIT (the one residue site #2)
go.mod / go.sum         # unchanged (module already stagecoach; content-only rename)
```

### Desired Codebase tree with files to be added

```bash
# NO new files. Two in-place token edits: internal/cmd/root.go:164 + internal/ui/verbose.go:101.
```

### Known Gotchas of our codebase & Library Quirks

```bash
# CRITICAL (the item's premise is STALE): exclude.go:28 is ALREADY ".stagecoachignore". Do NOT edit exclude.go
# (or exclude_test.go) — they're fully renamed. Verify: `grep -c stagehandignore internal/exclude/exclude.go` → 0.
# The REAL work is exactly 2 sites: root.go:164 + verbose.go:101.

# CRITICAL (.go-ONLY scope; docs are M4): the item's sed is --include='*.go'. README.md:66 + docs/*.md are
# P1.M4.T1's scope (the DOCS line says "in M4"). Do NOT touch any non-.go file. The bin/* + root binaries are
# build artifacts — ignore.

# CRITICAL (no test impact): no test asserts on either residue string (grep confirms only the 2 source sites).
# exclude_test.go already uses .stagecoachignore → passes before and after. The 2 edits are comment/help-text
# only → zero behavioral change → zero regression.

# CRITICAL (codebase location): work in /home/dustin/projects/stagehand (NOT /stagecoach — that's plan-only).
# Module is already github.com/dustin/stagecoach (head -1 go.mod). On-disk dir name is unchanged.

# GOTCHA (root.go:164 spans 2 concatenated lines): the help string is "Exclude matching files from the agent
# payload (unions with .stagehandignore and " + "[generation].exclude; never excluded from the commit)".
# Edit ONLY the `stagehandignore` token on L164; leave L165 ([generation].exclude…) and the surrounding
# pf.StringArrayVarP call unchanged.

# GOTCHA (verbose.go:101 is a DOC COMMENT, not runtime): the runtime string VerboseWarn prints comes from
# exclude.go:85 (already .stagecoachignore). Only the L101 doc comment is stale. Do NOT change VerboseWarn's
# body/signature — just the comment token.

# GOTCHA (the sed is safe): `stagehandignore` is unambiguous — it appears NOWHERE else in .go (grep confirms
# exactly 2 files). A scoped `sed -i 's/stagehandignore/stagecoachignore/g'` on the 2 files (or the item's
# grep|xargs form) catches exactly these 2 sites and nothing else. No risk of a collateral hit.
```

## Implementation Blueprint

### Data models and structure

N/A — no types, no data models. A 2-site string-token rename (`stagehandignore` → `stagecoachignore`).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: PRE-CHECK — confirm the actual state (don't trust the stale item premise)
  - RUN (from /home/dustin/projects/stagehand):
      grep -c stagehandignore internal/exclude/exclude.go        # EXPECT: 0 (already .stagecoachignore)
      grep -rn 'stagehandignore' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents'
      # EXPECT exactly 2 hits: internal/cmd/root.go:164 + internal/ui/verbose.go:101
  - IF exclude.go has residue (grep -c != 0): the premise changed — include exclude.go in the sed (Task 1).
    AS OF THIS PRP it is 0 (fully done); do NOT edit it.

Task 1: RENAME the 2 residue sites (pick ONE mechanism)
  - OPTION A (scoped sed — simplest): 
      sed -i 's/stagehandignore/stagecoachignore/g' internal/cmd/root.go internal/ui/verbose.go
  - OPTION B (the item's grep|xargs — auto-discovers exactly these 2 files):
      grep -rl 'stagehandignore' --include='*.go' . | grep -v '.git/' | xargs sed -i 's/stagehandignore/stagecoachignore/g'
  - OPTION C (two precise edit calls — most explicit):
      root.go:164   "unions with .stagehandignore and "   →   "unions with .stagecoachignore and "
      verbose.go:101 "unsupported .stagehandignore"       →   "unsupported .stagecoachignore"
  - All three are equivalent (the token is unambiguous; grep confirms exactly 2 .go files). Pick one.

Task 2: VERIFY (no further edits)
  - RUN: gofmt -w internal/cmd/root.go internal/ui/verbose.go (if sed was used; no-op for edit)
  - GATE 1: grep -rn 'stagehandignore' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' → EMPTY
  - GATE 2: gofmt -l internal/cmd/root.go internal/ui/verbose.go → clean
  - GATE 3: go vet ./... + go build ./... → clean
  - GATE 4: go test ./internal/exclude/... -count=1 → PASS; go test ./... -count=1 → green
  - GATE 5: git diff --name-only → ONLY internal/cmd/root.go + internal/ui/verbose.go (no docs/exclude/Makefile)
  - GATE 6: go.mod/go.sum byte-unchanged.
```

### Implementation Patterns & Key Details

```bash
# THE entire change (2 token edits). From /home/dustin/projects/stagehand:
sed -i 's/stagehandignore/stagecoachignore/g' internal/cmd/root.go internal/ui/verbose.go

# BEFORE/AFTER:
# internal/cmd/root.go:164 (the --exclude help text):
#   BEFORE: "Exclude matching files from the agent payload (unions with .stagehandignore and "+
#   AFTER:  "Exclude matching files from the agent payload (unions with .stagecoachignore and "+
# internal/ui/verbose.go:101 (the VerboseWarn doc comment):
#   BEFORE: // VerboseWarn prints a general warning for diagnostics such as unsupported .stagehandignore
#   AFTER:  // VerboseWarn prints a general warning for diagnostics such as unsupported .stagecoachignore

# GATE 1 (zero .go residue):
grep -rn 'stagehandignore' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' || echo "zero .go residue (good)"
# GATE 5 (scope — only the 2 files changed; docs/exclude untouched):
git diff --name-only | grep -vE '^internal/cmd/root\.go$|^internal/ui/verbose\.go$' && echo "BAD: out-of-scope file" || echo "only root.go + verbose.go (good)"
# Confirm exclude.go was NOT touched (already done):
git diff --exit-code internal/exclude/exclude.go internal/exclude/exclude_test.go && echo "exclude.go UNCHANGED (expected — already .stagecoachignore)"
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — content-only string rename; module already stagecoach (M1). go mod tidy no-op.

PACKAGE EDGES: NONE — no import changes (M1 owned imports). The rename is string-literal/comment content only.

FROZEN / NOT-EDITED:
  - internal/exclude/exclude.go + exclude_test.go — ALREADY .stagecoachignore (do NOT edit; READ-ONLY proof).
  - internal/config/* + internal/lock/* — S1 (parallel; config/lock PATHS).
  - README.md + docs/*.md — P1.M4.T1 (Mode A docs ride with M4, per the item's DOCS line). These still carry
    .stagehandignore references INTENTIONALLY (M4 renames them).
  - Makefile / .goreleaser.yaml / providers/*.toml / .github/workflows — P1.M3 / P1.M4.
  - bin/* + root binaries — build artifacts (ignore; rebuild clean).

DOWNSTREAM:
  - P1.M4.T1.S1/S2: rename README.md + docs/*.md .stagehandignore references (the docs twin of this task).
  - P1.M5.T2.S1: final grep audit ("zero stagehand references in tracked files") — catches the docs residue
    this task deliberately leaves for M4 (and any other stragglers).

NO DATABASE / NO ROUTES / NO CONFIG LOGIC CHANGE (the exclusion loader already reads .stagecoachignore via
the StagecoachIgnoreFile constant; this task only fixes 2 stale user-facing/comment strings).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand
gofmt -l internal/cmd/root.go internal/ui/verbose.go   # expect: empty (content-only; no structural change)
go vet ./...                                            # expect: clean (string/comment tokens; no broken refs)
go build ./...                                          # expect: success
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm only the 2 files changed (no docs/exclude/Makefile):
git diff --name-only | grep -vE '^internal/cmd/root\.go$|^internal/ui/verbose\.go$' && echo "BAD: out-of-scope file" || echo "only root.go + verbose.go (good)"
```

### Level 2: Unit Tests (Component Validation) — the zero-residue + no-regression gate

```bash
cd /home/dustin/projects/stagehand
go test ./internal/exclude/... -count=1 -v
# Expected: PASS (the exclusion loader reads .stagecoachignore via the const; exclude_test.go already asserts
# .stagecoachignore; the 2 edits don't touch exclude at all). -count=1 disables test caching.
go test ./... -count=1
# Expected: ALL PASS (comment/help-text only; no behavioral change; no test asserts on either residue string).
```

### Level 3: Integration Testing (System Validation) — the scope gates

```bash
cd /home/dustin/projects/stagehand
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# GATE 1 (zero .go residue):
grep -rn 'stagehandignore' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' && echo "BAD: .go residue" || echo "zero .go residue (good)"
# GATE 5 (scope — only the 2 files changed):
git diff --name-only | grep -vE '^internal/cmd/root\.go$|^internal/ui/verbose\.go$' && echo "BAD: out-of-scope file" || echo "only root.go + verbose.go (good)"
# Confirm exclude.go was NOT touched (already .stagecoachignore before this task):
git diff --exit-code internal/exclude/exclude.go internal/exclude/exclude_test.go && echo "exclude.go UNCHANGED (expected)"
# Confirm docs are UNTOUCHED (deferred to M4 — P1.M4.T1):
git diff --exit-code README.md docs/cli.md docs/README.md docs/how-it-works.md docs/configuration.md && echo "docs UNCHANGED (expected — M4 scope)"
```

### Level 4: Creative & Domain-Specific Validation

```bash
cd /home/dustin/projects/stagehand
# Smoke: the --exclude help text now names .stagecoachignore:
go build -o /tmp/stagecoach ./cmd/stagecoach && /tmp/stagecoach --help 2>&1 | grep -A1 '\-\-exclude' | grep '.stagecoachignore' && echo "help text renamed (good)" || echo "BAD: help still says .stagehandignore"
# golangci-lint: make lint (project-wide — content-only rename; no lint drift).
# NOTE: a repo-wide grep will STILL show .stagehandignore in README.md + docs/*.md — that is EXPECTED (M4 scope),
# NOT a failure of this task. This task's gate is .go-ONLY (GATE 1 above).
grep -rn 'stagehandignore' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents' | wc -l   # EXPECT: 0
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1 clean: `gofmt -l`, `go vet ./...`, `go build ./...`, `go mod tidy` no-op; only the 2 files changed.
- [ ] Level 2 green: `go test ./internal/exclude/... -count=1` + `go test ./... -count=1`.
- [ ] Level 3: GATE 1 zero `.go` residue; GATE 5 only root.go + verbose.go changed; exclude.go + docs UNCHANGED.

### Feature Validation
- [ ] `root.go:164` `--exclude` help text says `.stagecoachignore`.
- [ ] `verbose.go:101` `VerboseWarn` doc comment says `.stagecoachignore`.
- [ ] `internal/exclude/exclude.go` UNCHANGED (already `.stagecoachignore`).

### Code Quality Validation
- [ ] Scope-disciplined: `.go`-only; exclude.go (already done) + docs (M4) + config/lock (S1) UNTOUCHED.
- [ ] The stale item premise (edit exclude.go:28) was NOT followed — the actual residue (2 sites) was fixed instead.
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment
- [ ] No docs edited here (README.md + docs/*.md `.stagehandignore` refs are P1.M4.T1's scope, per the item).
- [ ] go.mod/go.sum byte-unchanged; no new files.

---

## Anti-Patterns to Avoid

- ❌ **Don't edit `internal/exclude/exclude.go` (or `exclude_test.go`).** They're ALREADY `.stagecoachignore`
  (the const value L28, the package doc, every comment, every test assertion). The item's "edit exclude.go:28"
  premise is STALE. Verify `grep -c stagehandignore internal/exclude/exclude.go` → 0. Editing it is a no-op
  that risks a spurious diff. The REAL work is root.go:164 + verbose.go:101.
- ❌ **Don't touch docs.** `README.md:66`, `docs/cli.md`, `docs/README.md`, `docs/how-it-works.md`,
  `docs/configuration.md` still reference `.stagehandignore` INTENTIONALLY — the item's DOCS line defers them
  to M4 (P1.M4.T1). The sed is `--include='*.go'`. A repo-wide `sed` would clobber M4's scope.
- ❌ **Don't run a repo-wide `sed` without `--include='*.go'`.** It would catch README.md + docs/*.md (M4) +
  potentially `.pi-subagents` transcripts + the `bin/*` binaries. Scope to `.go` (the 2 files, or the
  `--include='*.go'` grep|xargs form).
- ❌ **Don't touch `internal/config/*` or `internal/lock/*`.** Those are S1 (parallel; config/lock PATHS).
  This task is the exclusion filename in root.go + verbose.go — zero file overlap with S1.
- ❌ **Don't change the `--exclude` flag's LOGIC or VerboseWarn's body/signature.** Only the `stagehandignore`
  STRING TOKEN in the help text (root.go:164) and the doc comment (verbose.go:101). The loader already reads
  `.stagecoachignore` via the const; this is purely a stale-string fix.
- ❌ **Don't work in `/home/dustin/projects/stagecoach`.** That's the plan-staging dir (only `plan/`). The
  codebase is at `/home/dustin/projects/stagehand` (module already `github.com/dustin/stagecoach`).
- ❌ **Don't conflate "zero stagehand refs repo-wide" with this task's gate.** A repo-wide grep will STILL
  show `.stagehandignore` in README.md + docs/*.md — that is EXPECTED (M4 scope), NOT a failure. This task's
  gate is `.go`-ONLY (GATE 1: zero `.go` residue).
- ❌ **Don't change go.mod/go.sum or add files.** Two token edits in 2 existing files.
- ❌ **Don't skip the PRE-CHECK (Task 0).** The item premise is stale; confirming the real state first
  (`grep -c stagehandignore internal/exclude/exclude.go` → 0; exactly 2 `.go` hits) prevents wasted effort on
  exclude.go and ensures the 2-site fix is complete.

---

## Confidence Score

**9.5/10** — a 2-site string-token rename with the actual current state verified live (the stale item premise
is documented and redirected), the exact residue mapped (`grep` → exactly root.go:164 + verbose.go:101), zero
test impact confirmed (no test asserts on either string; exclude_test.go already `.stagecoachignore`), and a
clean `.go`-only scope with docs explicitly deferred to M4. The mechanism is a one-line scoped sed; the
verification grep is deterministic. The -0.5 reserves for the slim chance the codebase state shifts between
this PRP and implementation (e.g. someone re-introduces `.stagehandignore` in a new `.go` site) — the PRE-CHECK
(Task 0) + GATE 1 grep catch and absorb that.
