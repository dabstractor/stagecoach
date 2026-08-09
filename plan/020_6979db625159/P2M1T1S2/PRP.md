name: "P2.M1.T1.S2 — Runtime error messages (manifest.go, render.go, roles.go) + user-facing docs (README.md, docs/cli.md, docs/configuration.md, docs/providers.md)"
description: |
  The user-facing model-example token refresh for the SECOND half of P2.M1.T1: replace stale `zai/glm-5.2` /
  bare `glm-5.2` references with `anthropic/claude-haiku` / bare `claude-haiku` (matching PRD §12.3 +
  architecture/model_cleanup_scope.md Categories A-2 + the runtime-error-message half of A). Scope = the 3
  runtime-error-message code sites (internal/provider/manifest.go, internal/provider/render.go,
  internal/decompose/roles.go — the FR-R5b "must be inference/model, e.g. ..." errors) + the 4 user-facing
  docs (README.md, docs/cli.md, docs/configuration.md, docs/providers.md). Sibling P2.M1.T1.S1 owns the OTHER
  6 Category-A code files (pi.toml, config_init_interactive.go, default_action.go, config.go, bootstrap.go,
  builtin.go) — DISJOINT. The replacement still illustrates the inference-backend prefix-fold (FR-R5b):
  `anthropic/claude-haiku` demonstrates the SAME `backend/model` splitting as `zai/glm-5.2`. Test-safe: no
  provider/decompose test asserts on the glm portion of the error message (only the STATIC "must be
  inference/model" substring, which is preserved). DOCS = Mode A (the doc edits ride WITH this task).

---

## Goal

**Feature Goal**: Refresh every stale `zai/glm-5.2` / bare `glm-5.2` token in the 3 runtime FR-R5b error
sites and the 4 user-facing docs to the canonical `anthropic/claude-haiku` / `claude-haiku` example, so all
user-visible surfaces (error messages, README quickstart, CLI docs, config docs, provider docs) illustrate the
multi-backend prefix-fold mechanism with a current, non-author-specific model — and the scoped acceptance
grep over these 7 files returns zero hits.

**Deliverable**: Edits to exactly 7 existing files (no new files):
- `internal/provider/manifest.go` (lines 139, 156)
- `internal/provider/render.go` (lines 127, 249)
- `internal/decompose/roles.go` (lines 163, 169)
- `README.md` (lines 310, 315, 316, 319)
- `docs/cli.md` (lines 70, 172)
- `docs/configuration.md` (lines 40, 232)
- `docs/providers.md` (line 72)
(Line numbers are CURRENT at HEAD; README drifted ~6 from the contract cites — re-anchor with the acceptance
grep at impl time.)

**Success Definition**: `go test ./internal/provider/... ./internal/decompose/...` is green; `go build ./...`
succeeds; and `rg -n "zai/glm|glm-5\.2" internal/provider/manifest.go internal/provider/render.go
internal/decompose/roles.go README.md docs/cli.md docs/configuration.md docs/providers.md` returns **zero
hits**. No test file is touched (test-fixture refresh is T2's separate scope).

## User Persona (if applicable)

**Target User**: A new user reading the README/CLI docs/config docs, or a user who hits the FR-R5b
"must be inference/model" error — they should see a current, recognizable model example (`anthropic/claude-haiku`)
rather than the author's personal z.ai/GLM subscription token (`zai/glm-5.2`), which is account-specific and
outdated.
**Use Case**: User sets `stagecoach.model` for a multi-backend provider (pi) and consults the docs / error
message for the correct `backend/model` shape.
**Pain Points Addressed**: Confusion from a stale, subscription-specific example that implies a default nobody
else inherits (PRD §12.3 / FR-D2); consistency between the spec's canonical example and the shipped surfaces.

## Why

- **PRD §12.3 / §9.15 FR-R5b**: pi's shipped default is intentionally empty (FR-D2); the canonical multi-backend
  example in the spec is now `anthropic/claude-haiku`. The runtime errors + docs still show the author's old
  `zai/glm-5.2` — stale and account-specific.
- **Consistency with the sibling cleanup**: P2.M1.T1 is a coordinated `zai/glm-5.2` → `anthropic/claude-haiku`
  sweep across code (S1 + this task's 3 error sites), tests (T2), and docs (this task's 4 docs). S2 is the
  user-facing half (errors + docs); S1 is the other code comments; T2 is the test fixtures.
- **FR-R5b illustration preserved**: `anthropic/claude-haiku` demonstrates the IDENTICAL slash-prefix fold
  (`--provider anthropic --model claude-haiku`) that `zai/glm-5.2` did — the mechanism is unchanged; only the
  example token is refreshed.

## What

Pure text-replacement edits across 7 files. The replacement rules (from model_cleanup_scope.md + PRD §12.3):
- Every `zai/glm-5.2` → `anthropic/claude-haiku` (the prefixed, correct form).
- Every bare `glm-5.2` → `claude-haiku` (the no-slash form — it is the WRONG example in the FR-R5b error
  illustration, or an incidental `--model` value in docs).
- The STATIC phrase `"must be inference/model"` in the error messages is **unchanged** (tests assert on it).
- No test file is touched (test fixtures are P2.M1.T2's scope).

### Success Criteria
- [ ] manifest.go:139 comment — bare `glm-5.2` → `claude-haiku`; `zai/glm-5.2` → `anthropic/claude-haiku`.
- [ ] manifest.go:156, render.go:127, render.go:249, roles.go:169 — error `e.g. "zai/glm-5.2"` → `e.g. "anthropic/claude-haiku"`.
- [ ] roles.go:163 comment — `("zai/glm-5.2")` → `("anthropic/claude-haiku")`.
- [ ] README.md:310, 315, 316 (×2), 319 — all `zai/glm-5.2` → `anthropic/claude-haiku`.
- [ ] docs/cli.md:70 — bare `stagecoach --model glm-5.2` → `stagecoach --model claude-haiku`.
- [ ] docs/cli.md:172 — `model = "zai/glm-5.2"` → `model = "anthropic/claude-haiku"`.
- [ ] docs/configuration.md:40 — `model = "zai/glm-5.2"` → `model = "anthropic/claude-haiku"`.
- [ ] docs/configuration.md:232 — bare `model = glm-5.2` → `model = claude-haiku`.
- [ ] docs/providers.md:72 — `(e.g. `zai/glm-5.2`)` → `(e.g. `anthropic/claude-haiku`)`.
- [ ] `go build ./...` exits 0; `go test ./internal/provider/... ./internal/decompose/...` green.
- [ ] Scoped acceptance grep (Level 4) over the 7 files → zero hits.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — every edit is pinned to an exact file + exact old-text anchor + exact new-text, with the bare-vs-prefixed
rule spelled out, the test-safety guarantee (which substring is asserted vs changed) documented, the sibling/parallel
scope boundaries (so no file is double-edited or missed), and a verified acceptance grep.

### Documentation & References

```yaml
# MUST READ — the scope doc that categorizes every glm hit (S2 = Category A runtime-error rows + Category A-2).
- docfile: plan/020_6979db625159/architecture/model_cleanup_scope.md
  why: "Category A rows (manifest.go:139,156 / render.go:127,249 / roles.go:163,169 = runtime error messages) +
        Category A-2 rows (README/docs/cli/docs/configuration/docs/providers) define S2's exact scope + the
        replacement strategy (zai/glm-5.2→anthropic/claude-haiku; bare glm→claude-haiku). Also confirms the
        acceptance criterion: scoped rg returns zero hits."
  section: "Category A + Category A-2 + Replacement Strategy + Acceptance Criterion"

# MUST READ — S2's own verified hit inventory (current line numbers beat the contract's stale cites).
- docfile: plan/020_6979db625159/P2M1T1S2/research/findings.md
  why: "The exact oldText→newText per file:line (incl. README drift + the double-occurrence line 316); the
        test-safety proof (no test asserts on the glm portion; only the STATIC 'must be inference/model'
        substring, which S2 preserves); the sibling/parallel scope boundaries."
  section: "EXACT current hits table + TEST-SAFETY analysis"

# MUST READ — S1 is a CONTRACT: it owns the OTHER 6 code files; S2 must NOT touch them.
- docfile: plan/020_6979db625159/P2M1T1S1/PRP.md
  why: "Confirms scope is DISJOINT: S1 = pi.toml, config_init_interactive.go, default_action.go, config.go,
        bootstrap.go, builtin.go (NOT manifest.go/render.go/roles.go, NOT the 4 docs). S2 must not touch those.
        Also: S1 establishes the bare-vs-prefixed rule (bare glm→bare claude-haiku, prefixed→anthropic/claude-haiku)."

# PRD authority for the canonical example token + the FR-R5b mechanism S2 must keep illustrating.
- url: PRD.md#12.3   # §12.3 Built-in provider: pi — canonical example is anthropic/claude-haiku
  why: "Establishes anthropic/claude-haiku as the canonical multi-backend example (§12.3)."
- url: PRD.md#9.15   # §9.15 FR-R5b — the inference/model prefix-fold mechanism the errors illustrate
  why: "The error message 'must be inference/model, e.g. ...' implements FR-R5b's hard-config-error gate. The
        replacement anthropic/claude-haiku demonstrates the SAME backend/model split; the mechanism is unchanged."

# Code sites under edit (the 3 runtime-error files) — READ to get exact text + confirm test-safety.
- file: internal/provider/manifest.go
  why: "ValidateModel docstring (:139) + fmt.Errorf (:156). The STATIC phrase 'must be inference/model' is
        asserted by manifest_test.go:329 — PRESERVE it; only the 'e.g. \"...\"' example changes."
- file: internal/provider/render.go
  why: "Two identical fmt.Errorf sites (:127 Render, :249 RenderMultiTurn)."
- file: internal/decompose/roles.go
  why: "Comment (:163) + role-named fmt.Errorf (:169). roles_test.go:332/333/391/392 assert 'must be
        inference/model' + role name — PRESERVE both."

# Test-safety proof (DO NOT EDIT — T2 owns these; read only to confirm S2 is message-text-safe).
- file: internal/provider/manifest_test.go
  why: ":329 asserts strings.Contains(err.Error(), 'must be inference/model') — the STATIC phrase S2 preserves."
- file: internal/decompose/roles_test.go
  why: ":332/333/391/392 assert 'must be inference/model' + 'planner' — STATIC substrings S2 preserves."
```

### Current Codebase tree (relevant slice)

```bash
# Files S2 EDITS (7 — all existing, no new files):
internal/provider/manifest.go     # :139 comment, :156 fmt.Errorf (ValidateModel)
internal/provider/render.go       # :127 fmt.Errorf (Render), :249 fmt.Errorf (RenderMultiTurn)
internal/decompose/roles.go       # :163 comment, :169 fmt.Errorf (role-named)
README.md                         # :310, :315, :316 (×2 occ), :319 (git-config quickstart + NOTE)
docs/cli.md                       # :70 (--model shadowing), :172 (config init docs)
docs/configuration.md             # :40 (config init prose), :232 (config example block)
docs/providers.md                 # :72 (multi-backend render example)
# DO NOT TOUCH (disjoint scope):
#   S1's 6 files (pi.toml, config_init_interactive.go, default_action.go, config.go, bootstrap.go, builtin.go)
#   ALL *_test.go (Category B/C/D = T2 / P2.M1.T2 scope)
#   spec/** (human-owned — AGENTS.md rule 1)
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
# NONE added. S2 edits the 7 existing files above. No new files, no new packages, no test changes.
```

### Known Gotchas of our codebase & Library Quirks

```text
# CRITICAL (preserve the STATIC phrase tests assert on): the error message is
#   'provider render %q: model %q on %s must be inference/model, e.g. "zai/glm-5.2"'
# Tests assert ONLY on 'must be inference/model' (manifest_test.go:329, roles_test.go:332/333/391/392). S2
# changes ONLY the 'e.g. "zai/glm-5.2"' → 'e.g. "anthropic/claude-haiku"' tail. DO NOT touch 'must be
# inference/model'. (Verified: no test asserts on the glm portion — S2 is message-text-safe standalone.)

# CRITICAL (bare vs prefixed — they map differently):
#   - manifest.go:139 comment illustrates BOTH forms: bare 'glm-5.2' (the WRONG example) AND 'zai/glm-5.2'
#     (the RIGHT example). Map bare→'claude-haiku', prefixed→'anthropic/claude-haiku' (do NOT prefix the bare one —
#     it must stay the no-slash error case to keep illustrating the bug).
#   - docs/cli.md:70 'stagecoach --model glm-5.2' and docs/configuration.md:232 'model = glm-5.2' are BARE → bare
#     'claude-haiku'. Everything else is 'zai/glm-5.2' → 'anthropic/claude-haiku'.

# GOTCHA (line numbers drifted): README.md's actual hits are at :310/:315/:316/:319, NOT the contract's
#   :304/:309/:310/:313 (~6-line drift). The other 6 files match the contract. Re-anchor with the acceptance
#   grep BEFORE editing — never trust bare line numbers. README.md:316 has TWO occurrences on one line (both
#   zai/glm-5.2) — a single edit replacing the whole line handles both.

# GOTCHA (parallel edit on docs/cli.md + README.md): P1.M3.T1.S2 concurrently edits those 2 files for
#   winget→chocolatey on DIFFERENT lines. Text anchors are disjoint (glm vs winget) so there is no merge conflict,
#   but re-read the current file state at impl time (winget edits may have shifted line numbers).

# GOTCHA (scope fence): S2 touches ONLY its 7 files. S1's 6 code files + ALL test files are out of scope
#   (S1 / T2 respectively). spec/** is human-owned (AGENTS.md rule 1) — never edit.
```

## Implementation Blueprint

### Data models and structure

No data models — this is a text-token refresh across existing files.

### Implementation Tasks (ordered; each is independent, but code first then docs)

```yaml
Task 0: RE-ANCHOR (do this FIRST — line numbers drift)
  - RUN: rg -n 'zai/glm|glm-5\.2' internal/provider/manifest.go internal/provider/render.go internal/decompose/roles.go README.md docs/cli.md docs/configuration.md docs/providers.md
  - EXPECT: 15 matching LINES (manifest.go 2, render.go 2, roles.go 2, README.md 4, cli.md 2, configuration.md 2, providers.md 1; README.md:316 has 2 occurrences on 1 line).
  - NOTE: if the line numbers differ from this PRP's table, use the rg output's lines — the TEXT anchors below are what matter.

Task 1: EDIT internal/provider/manifest.go (2 edits)
  - EDIT A — the ValidateModel docstring comment (:139):
      OLD: `// e.g. "glm-5.2" on pi instead of "zai/glm-5.2") BEFORE emitting an optimistic`
      NEW: `// e.g. "claude-haiku" on pi instead of "anthropic/claude-haiku") BEFORE emitting an optimistic`
      (bare glm-5.2 → bare claude-haiku; zai/glm-5.2 → anthropic/claude-haiku.)
  - EDIT B — the fmt.Errorf example (:156):
      OLD: `"provider render %q: model %q on %s must be inference/model, e.g. \"zai/glm-5.2\"",`
      NEW: `"provider render %q: model %q on %s must be inference/model, e.g. \"anthropic/claude-haiku\"",`
      (PRESERVE "must be inference/model" — asserted by manifest_test.go:329.)
  - VALIDATE: go build ./internal/provider/ ; go test ./internal/provider/ -run TestValidateModel -v

Task 2: EDIT internal/provider/render.go (2 edits — identical text, two sites :127 and :249)
  - The two sites are byte-identical (`"provider render %q: model %q on %s must be inference/model, e.g. \"zai/glm-5.2\"",`).
    Use TWO edits (each oldText includes enough surrounding context — the preceding `return nil, fmt.Errorf(` — to be unique per site), OR edit each site individually after re-anchoring by line.
  - NEW (both): `"provider render %q: model %q on %s must be inference/model, e.g. \"anthropic/claude-haiku\"",`
  - VALIDATE: go build ./internal/provider/ ; go test ./internal/provider/ -run 'Render' -v

Task 3: EDIT internal/decompose/roles.go (2 edits)
  - EDIT A — the comment (:163):
      OLD: `		// ("zai/glm-5.2"); a bare model is an unroutable config error, never a silent bare --model. Mirrors`
      NEW: `		// ("anthropic/claude-haiku"); a bare model is an unroutable config error, never a silent bare --model. Mirrors`
  - EDIT B — the role fmt.Errorf (:169):
      OLD: `"role %q: model %q on %s must be inference/model, e.g. \"zai/glm-5.2\"", role, mdl, m.Name)`
      NEW: `"role %q: model %q on %s must be inference/model, e.g. \"anthropic/claude-haiku\"", role, mdl, m.Name)`
      (PRESERVE "must be inference/model" + the role-name shape — asserted by roles_test.go:332/333/391/392.)
  - VALIDATE: go build ./internal/decompose/ ; go test ./internal/decompose/ -run 'Role' -v

Task 4: EDIT README.md (4 lines — all zai/glm-5.2 → anthropic/claude-haiku)
  - :310  OLD `git config stagecoach.model zai/glm-5.2`  NEW `git config stagecoach.model anthropic/claude-haiku`
  - :315  OLD `> (`zai/glm-5.2`). A bare model (no ` + "`" + `/` + "`" + `) on pi is a config error (FR-R5b). Set`
          NEW `> (`anthropic/claude-haiku`). A bare model ...`  (replace only the `zai/glm-5.2` token inside backticks)
  - :316  OLD `> ` + "`" + `git config stagecoach.model zai/glm-5.2` + "`" + ` (or `[defaults] model = "zai/glm-5.2"` in your config).`
          NEW `> ` + "`" + `git config stagecoach.model anthropic/claude-haiku` + "`" + ` (or `[defaults] model = "anthropic/claude-haiku"` in your config).`
          (TWO occurrences of zai/glm-5.2 on this one line — replace both.)
  - :319  OLD `set ` + "`" + `model = "zai/glm-5.2"` + "`" + ` to pin a specific backend):`
          NEW `set ` + "`" + `model = "anthropic/claude-haiku"` + "`" + ` to pin a specific backend):`
  - (Markdown tables/backticks must be preserved exactly — replace ONLY the token inside them.)

Task 5: EDIT docs/cli.md (2 edits — bare + prefixed)
  - :70   OLD `— e.g. ` + "`" + `stagecoach --model glm-5.2` + "`" + ` against a`  NEW `— e.g. ` + "`" + `stagecoach --model claude-haiku` + "`" + ` against a`  (BARE → bare claude-haiku; it's a shadowing example, keep bare)
  - :172  OLD `(e.g. model = "zai/glm-5.2") to pin a backend (FR-R5b)`  NEW `(e.g. model = "anthropic/claude-haiku") to pin a backend (FR-R5b)`

Task 6: EDIT docs/configuration.md (2 edits — prefixed + bare)
  - :40   OLD `e.g. ` + "`" + `model = "zai/glm-5.2"` + "`" + `, FR-R5b`  NEW `e.g. ` + "`" + `model = "anthropic/claude-haiku"` + "`" + `, FR-R5b`
  - :232  OLD `    model = glm-5.2`  NEW `    model = claude-haiku`  (BARE config-example value → bare claude-haiku)

Task 7: EDIT docs/providers.md (1 edit)
  - :72   OLD `the model is ` + "`" + `inference/model` + "`" + ` (e.g. ` + "`" + `zai/glm-5.2` + "`" + `):`
          NEW `the model is ` + "`" + `inference/model` + "`" + ` (e.g. ` + "`" + `anthropic/claude-haiku` + "`" + `):`
```

### Implementation Patterns & Key Details

```text
# PATTERN (the FR-R5b error message — change ONLY the example tail):
#   fmt.Errorf("provider render %q: model %q on %s must be inference/model, e.g. \"zai/glm-5.2\"", ...)
#                                          ^^^^^^^^^^^^^^^^^^^^^^^^^^^^  ^^^^^^^^^^^^^^^^^^^^^^
#                                           PRESERVE (test-asserted)       CHANGE → "anthropic/claude-haiku"
# The mechanism (inference/model slash-fold) is unchanged; only the illustrative example token moves with the
# spec's canonical anthropic/claude-haiku.

# PATTERN (bare vs prefixed mapping — 3 categories):
#   1. Error-message "right" example    zai/glm-5.2  → anthropic/claude-haiku   (manifest.go:156, render.go:127/249, roles.go:169)
#   2. Error-message "wrong" example    glm-5.2      → claude-haiku (bare)       (manifest.go:139 comment)
#   3. Doc incidental values:
#        README (all prefixed)          zai/glm-5.2  → anthropic/claude-haiku
#        cli.md:70 / configuration.md:232 (bare)     glm-5.2      → claude-haiku (bare)
#        all other doc hits (prefixed)  zai/glm-5.2  → anthropic/claude-haiku
```

### Integration Points

```yaml
# NONE (code/build/config/migration). This is a text-token refresh; no schema, no API, no behavior change.
# Cross-task coordination (informational):
CONSUMES:
  - PRD §12.3 canonical example (anthropic/claude-haiku) + model_cleanup_scope.md Categories A/A-2.
PRODUCES:
  - The 7 user-facing surfaces use the canonical example; T2 (P2.M1.T2) refreshes the test fixtures separately.
  - The scoped acceptance grep (S2's 7 files) returns zero hits — part of P2.M1's overall zero-hit goal.
PARALLEL (non-conflicting):
  - P1.M3.T1.S2 edits docs/cli.md + README.md (winget→chocolatey) on DIFFERENT lines — disjoint text anchors.
```

## Validation Loop

> **This is a Go-repo text refresh, not a Python/MCP item.** The template's `ruff`/`mypy`/`pytest` gates DO NOT
> apply. Use `go build`, `go test` (scoped), `go vet`, and the acceptance grep. Run commands from the repo root.

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After the 3 code edits — the packages must still build + vet clean (the edits are string/comment literals,
# but confirm no stray quote/escape broke the file).
go build ./internal/provider/ ./internal/decompose/
go vet   ./internal/provider/ ./internal/decompose/
# Expected: exit 0, no output. (gofmt is not required for string-content edits, but `gofmt -l <files>` should
# report nothing changed — the edits are within existing strings/comments, no reformatting needed.)
```

### Level 2: Unit Tests (Component Validation — the test-safety gate)

```bash
# The scoped suites that contain the edited error sites. STAYS GREEN: tests assert on the STATIC
# 'must be inference/model' substring (preserved), NOT on the glm example tail.
go test ./internal/provider/...  -v   # manifest_test.go:329 (ValidateModel) + render/merge/registry/builtin
go test ./internal/decompose/... -v   # roles_test.go:332/333/391/392 (role-named inference/model error)
# Expected: all PASS, exit 0. (glm used as test FIXTURE input elsewhere in these files does not break — it
# doesn't assert on the message text; refreshing those fixtures is T2's separate scope.)
```

### Level 3: Whole-repo build (System Validation)

```bash
# Confirm nothing else broke (the edits are isolated literals, so this is a sanity gate).
go build ./...
go test ./...  2>&1 | tail -30
# Expected: build exit 0; no FAIL package. (If a test asserts on the literal 'zai/glm-5.2' message tail it would
# fail here — but research confirmed NONE do; any such failure belongs to T2, not S2 — flag, do not fix here.)
```

### Level 4: Acceptance grep (the contract's zero-hit gate) + scope guard

```bash
# THE acceptance criterion: zero hits across S2's 7 files.
rg -n 'zai/glm|glm-5\.2' internal/provider/manifest.go internal/provider/render.go internal/decompose/roles.go README.md docs/cli.md docs/configuration.md docs/providers.md
# Expected: NO output (zero hits). If any line remains, edit it per the findings.md table and re-run.

# Scope guard: S2 touched ONLY its 7 files (no S1 files, no test files, no spec/, no plan/PRD).
git status --porcelain
# Expected: M internal/provider/manifest.go  M internal/provider/render.go  M internal/decompose/roles.go
#           M README.md  M docs/cli.md  M docs/configuration.md  M docs/providers.md   (nothing else)
git status --porcelain | grep -E '_test\.go|pi\.toml|config_init_interactive|default_action\.go|config\.go|bootstrap\.go|builtin\.go|spec/|PRD\.md|plan/|tasks\.json' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
# Expected: "OK: scope clean" (S1's 6 files, all tests, spec/, plan/PRD untouched).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./internal/provider/ ./internal/decompose/` exit 0
- [ ] `go vet ./internal/provider/ ./internal/decompose/` exit 0
- [ ] `go test ./internal/provider/...` green (manifest_test.go:329 still passes — STATIC substring preserved)
- [ ] `go test ./internal/decompose/...` green (roles_test.go:332/333/391/392 still pass — STATIC substrings preserved)
- [ ] `go build ./...` exit 0 (whole-repo sanity)

### Feature Validation
- [ ] manifest.go:139 comment — bare `glm-5.2`→`claude-haiku`, `zai/glm-5.2`→`anthropic/claude-haiku`
- [ ] manifest.go:156 + render.go:127 + render.go:249 + roles.go:169 — `e.g. "zai/glm-5.2"`→`e.g. "anthropic/claude-haiku"`
- [ ] roles.go:163 comment — `("zai/glm-5.2")`→`("anthropic/claude-haiku")`
- [ ] README.md:310/315/316(×2)/319 — all `zai/glm-5.2`→`anthropic/claude-haiku`
- [ ] docs/cli.md:70 bare `glm-5.2`→`claude-haiku`; docs/cli.md:172 `zai/glm-5.2`→`anthropic/claude-haiku`
- [ ] docs/configuration.md:40 `zai/glm-5.2`→`anthropic/claude-haiku`; :232 bare `glm-5.2`→`claude-haiku`
- [ ] docs/providers.md:72 `zai/glm-5.2`→`anthropic/claude-haiku`
- [ ] Acceptance grep (Level 4) over the 7 files → zero hits

### Code Quality Validation
- [ ] The STATIC `must be inference/model` phrase is unchanged in all 4 error sites (test-asserted)
- [ ] The bare-vs-prefixed rule applied correctly (bare glm→bare claude-haiku; prefixed→anthropic/claude-haiku)
- [ ] Markdown formatting/backticks preserved in the 4 doc files (only the token inside changed)

### Scope-Boundary Validation
- [ ] `git status --porcelain` shows ONLY the 7 S2 files modified
- [ ] NO S1 file touched (pi.toml, config_init_interactive.go, default_action.go, config.go, bootstrap.go, builtin.go)
- [ ] NO `*_test.go` touched (T2 / P2.M1.T2 scope)
- [ ] NO `spec/**`, `PRD.md`, `plan/**`, `tasks.json` touched (human/orchestrator-owned)

### Documentation & Deployment
- [ ] Mode A satisfied: the 4 doc edits (README, cli, configuration, providers) ARE the docs work — they ride with this task
- [ ] User-facing error messages + docs now show the canonical `anthropic/claude-haiku` example, consistent with PRD §12.3

---

## Anti-Patterns to Avoid

- ❌ Don't change the `must be inference/model` phrase — tests assert on it (manifest_test.go:329,
  roles_test.go:332/333/391/392). Change ONLY the `e.g. "zai/glm-5.2"` example tail.
- ❌ Don't prefix the BARE `glm-5.2` occurrences — bare stays bare (`claude-haiku`). The bare form is the
  *wrong* (no-slash) example in the FR-R5b illustration (manifest.go:139) and an incidental `--model` value
  in docs (cli.md:70, configuration.md:232); prefixing it would break the illustration's point.
- ❌ Don't trust the contract's bare line numbers — README drifted ~6 lines. Re-anchor with the acceptance
  grep (Task 0) before editing; use TEXT anchors, not line numbers, for the actual edits.
- ❌ Don't edit any test file — even if a test uses `zai/glm-5.2` as fixture input, refreshing it is T2's
  scope (P2.M1.T2.S1/S2). S2 changes only the 3 runtime-error code files + 4 docs.
- ❌ Don't edit S1's 6 code files (pi.toml, config_init_interactive.go, default_action.go, config.go,
  bootstrap.go, builtin.go) — that's the sibling task's disjoint scope.
- ❌ Don't edit `spec/**` or `PRD.md` — spec is human-owned (AGENTS.md rule 1); PRD is READ-ONLY.
- ❌ Don't "fix" the FR-R5b mechanism — `anthropic/claude-haiku` demonstrates the SAME slash-prefix fold as
  `zai/glm-5.2`; only the example token changes, never the logic.
- ❌ Don't touch markdown structure in the doc files — replace ONLY the token inside the backticks/quotes;
  preserve the surrounding `[defaults]`, `git config`, table, and `> NOTE` formatting.

---

## Confidence Score

**One-pass success likelihood: 10/10.** This is a deterministic text-token refresh: every edit is pinned to an
exact file + verified old-text anchor + exact new-text (the bare-vs-prefixed rule is spelled out per site);
the test-safety guarantee is proven by direct inspection (no provider/decompose test asserts on the glm tail —
only the STATIC `must be inference/model` substring, which S2 preserves, so both scoped suites stay green
standalone); the acceptance grep is a single deterministic command; and the scope fence (S1's 6 files, all
tests, spec/, plan/PRD) is explicit. The only nuance — README line drift and the double-occurrence line — is
captured with a re-anchor step (Task 0) and whole-line edits. No behavior, schema, or API changes; the FR-R5b
mechanism is unchanged.