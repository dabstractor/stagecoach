name: "P1.M1.T1.S1 — *bool conversion: AutoStageAll & MultiTurnFallback precedence-aware overlay"
description: >
  Convert the two default-`true` boolean config fields (`AutoStageAll`, `MultiTurnFallback`) from
  plain `bool` to `*bool` end-to-end so a TOML file or git-config can set them `false` and have
  that `false` survive the merge chain (Issue 1: `auto_stage_all = false` silently ignored).
  Mirrors the proven, field-tested `DiffContext *int` pattern exactly.

---

## Goal

**Feature Goal**: Make `AutoStageAll` and `MultiTurnFallback` precedence-aware `*bool` fields (nil =
inherit lower layer; non-nil incl. `*false` = explicit override) across the resolved `Config`, the
file-decode structs, the `materialize`/`overlay` merge guards, the git-config layer, the `Defaults()`
seeds, and every consumer read — so that writing `auto_stage_all = false` (or
`multi_turn_fallback = false`, or `git config stagecoach.autoStageAll false`) in a config source
actually disables the feature instead of being silently dropped.

**Deliverable**: An atomic, compile-clean type refactor of `internal/config` plus all consumer/test
adaptations, plus two new accessor methods (`AutoStageAllValue()`, `MultiTurnFallbackValue()`) and
unit tests proving the `*false` override survives the full materialize→overlay chain. Docs
(`docs/configuration.md`) updated to remove the now-false "cannot disable via file" limitation notes.

**Success Definition**:
- `go build ./...` and `go vet ./...` are clean; the whole test suite (`go test ./...`) passes.
- `Defaults().AutoStageAllValue() == true` and `Defaults().MultiTurnFallbackValue() == true`.
- A `[generation] multi_turn_fallback = false` (or `[defaults] auto_stage_all = false`) TOML file,
  run through `Defaults() → overlay(loadTOML(...))`, yields a `Config` whose accessor returns
  **`false`** end-to-end (table-driven unit test proves nil/`*true`/`*false` and global/repo layering).
- `git config stagecoach.autoStageAll false` → `loadGitConfig` produces `*bool` false → overlay
  propagates `*false` (existing git_test.go false-path still passes, adapted to accessor).
- No behavior change for the default-`true` path (everything still defaults on).
- Docs lines 110, 157, 166 no longer claim these fields "cannot be disabled via file".

## Why

- **Issue 1 (Major)**: `auto_stage_all = false` in any TOML config file is **silently ignored** —
  stagecoach commits anyway, doing the *opposite* of what the user configured. Same silent failure
  for `multi_turn_fallback = false`. Root cause: the merge guards use the one-way
  `if d.X { c.X = true }` form, which can never propagate `false` (the bool zero value is
  indistinguishable from "unset"). PRD §9.8 FR34 (config file is a precedence layer that overrides
  built-in defaults), §9.4 FR16, §16.2.
- This is the foundational, blocking fix for the whole P1.M1 milestone: until the `*bool` overlay
  exists, the env-var subtask (T2) and the docs/git-key subtask (T3) cannot produce a working,
  fully-documented persistent-disable path. The blast radius is large but every site follows the
  1:1 proven `DiffContext *int` pattern.

## What

**User-visible behavior**: A user who writes `auto_stage_all = false` (or
`multi_turn_fallback = false`) in `~/.config/stagecoach/config.toml` or `.stagecoach.toml`, or sets
`git config stagecoach.autoStageAll false`, gets that `false` honored — stagecoach does NOT auto-stage
/ does NOT use multi-turn fallback, as configured. The default (`true`) is unchanged.

**Technical change (atomic — touches every read/write site together or it won't compile)**:
1. `Config.AutoStageAll` and `Config.MultiTurnFallback` → `*bool`.
2. `fileDefaults.AutoStageAll` and `fileGeneration.MultiTurnFallback` → `*bool`.
3. `Defaults()` seeds them with `boolPtr(true)` (non-nil so a higher layer's `*false` is meaningful).
4. `materialize`/`overlay` guards change from `if d.X { c.X = true }` → `if d.X != nil { c.X = d.X }`.
5. `git.go` produces `*bool` (`c.AutoStageAll = boolPtr(v)`).
6. New accessors `AutoStageAllValue()` / `MultiTurnFallbackValue()` mirror `DiffContextValue()`.
7. All 5 production consumer reads switch to the accessors.
8. All test reads/assignments adapted (`bool` → `*bool`/accessor/`boolPtr`).
9. Docs: remove the "cannot disable via file" limitation notes for these two fields.

### Success Criteria
- [ ] `auto_stage_all = false` in `[defaults]` of a TOML file ⇒ resolved `AutoStageAllValue() == false` through `Defaults → overlay(loadTOML)`.
- [ ] `multi_turn_fallback = false` in `[generation]` of a TOML file ⇒ resolved `MultiTurnFallbackValue() == false`.
- [ ] Omitted keys ⇒ accessor returns the `true` default.
- [ ] `git config stagecoach.autoStageAll false` ⇒ `*false` survives overlay.
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` all pass.
- [ ] Docs lines 110/157/166 updated; git-config key spelling (lines 210/218) UNTOUCHED (that is T3).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — this PRP gives exact current line numbers, the 1:1 mirror pattern (`DiffContext *int`),
every consumer and test site (verified by grep), the cross-package `boolPtr` gotcha, the
"config-init-uses-hardcoded-strings-not-encoder" fact, and the scope boundaries against sibling subtasks.

### Documentation & References

```yaml
# MUST READ — the proven pattern to mirror 1:1
- file: internal/config/config.go
  why: "DiffContext *int is the field-tested precedent for 'nil=inherit; non-nil incl. zero=override'."
  pattern: >
    Field `DiffContext *int` (config.go:77); Defaults() seeds `DiffContext: intPtr(1)` (config.go:~195);
    accessor `DiffContextValue()` (config.go:228-233) returns the fallback when nil, dereferences when non-nil;
    helpers `boolPtr` (config.go:7) + `intPtr` (config.go:9) already exist.
  critical: "Accessor MUST return the bool default (true) when the pointer is nil. boolPtr is UNEXPORTED (package config only)."

- file: internal/config/file.go
  why: "materialize (file.go:208) and overlay (file.go:326) are the two merge sites with the broken one-way guards."
  pattern: "DiffContext materialize guard (file.go:237-243) `if g.DiffContext != nil { c.DiffContext = g.DiffContext }`; overlay guard (file.go:375-382) `if src.DiffContext != nil { dst.DiffContext = src.DiffContext }`."
  critical: "overlay sits between EVERY layer and the final config (load.go), so the guard MUST be `!= nil`, NEVER `!= false`/`!= 0` — a non-nil-guard failure would silently revert an explicit false to the default. This is exactly the bug class being fixed."

- file: internal/config/git.go
  why: "Layer 4 (repo git config). Reads `stagecoach.autoStageAll` (camelCase — git forbids underscores in the final key segment)."
  pattern: "DiffContext git path (git.go:214-221) parses into a local `n` then wraps `c.DiffContext = intPtr(n)`."
  critical: "git.go has NO `stagecoach.multiTurnFallback` key (only autoStageAll). So only `c.AutoStageAll = v` (git.go:161) changes here — to `c.AutoStageAll = boolPtr(v)`. The `gitConfigBool` helper returns a plain bool `v`."

# Architecture notes already in the repo (READ for the full precedence model)
- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/architecture/system_context.md
  why: "Full 7-layer precedence model + the blast-radius table mapping every site to its fix."
  section: "The Proven Fix Pattern: *bool (mirrors DiffContext *int)"

- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/architecture/research_config_precedence.md
  why: "Confirms the only-true-propagates bug at each materialize/overlay site; lists every STAGECOACH_* env var (none for these fields — that is T2, out of scope)."

- docfile: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/P1M1T1S1/research/verification_deltas.md
  why: "Line-number drift vs. the task description + the EXTRA test/consumer sites + the cross-package boolPtr gotcha. READ THIS — it corrects the task's bullet list."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  config.go        # Config struct + Defaults() + DiffContextValue() accessor + boolPtr/intPtr helpers
  file.go          # fileDefaults/fileGeneration decode structs + materialize() + overlay()
  git.go           # loadGitConfig() — Layer 4 (repo git config)
  load.go          # Load(): chains Defaults → overlay(global) → overlay(repo) → overlay(git) → env → flags
  config_test.go   # Defaults() assertions (lines 23, 49)
  file_test.go     # materialize/overlay tests (lines 76, 113-124, 790) + DiffContext table test (814-946) = PATTERN TO MIRROR
  git_test.go      # loadGitConfig tests incl. the autoStageAll=false path (lines 108,164,184,315,372)
  multiturn_test.go# MultiTurnFallback materialize/overlay assertions (lines 47,79,85,87,130,147,153,156) — SEMANTICS CHANGE
internal/cmd/default_action.go      # consumer reads: cfg.AutoStageAll (lines 121, 382)
internal/generate/generate.go       # consumer read:  cfg.MultiTurnFallback (line 394)
internal/generate/*_test.go         # ASSIGNMENTS cfg.MultiTurnFallback = true (lines: multiturn_test 549,810; generate_multiturn_failure_test 120; generate_multiturn_test 83; generate_workdesc_test 268)
internal/hook/exec.go               # consumer read:  cfg.MultiTurnFallback (line 226)
pkg/stagecoach/stagecoach.go        # consumer read:  cfg.MultiTurnFallback (line 623)
pkg/stagecoach/stagecoach_test.go   # ASSIGNMENT cfg.MultiTurnFallback = true (line 1327); ALREADY has boolPtr helper (line 25)
internal/config/bootstrap.go        # config init template — HARDCODED strings (lines 161, 304), NOT toml encoder
docs/configuration.md               # limitation notes to fix (lines 110, 157, 166)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (compile-order): this is an ATOMIC type change. `bool → *bool` breaks every site that
//   reads (`cfg.AutoStageAll` is now a pointer, not a bool) OR assigns (`cfg.AutoStageAll = true`
//   won't compile) in one shot. Do ALL edits in config.go/file.go/git.go + consumers + tests
//   before running `go build`. Partial changes will not compile.

// CRITICAL (overlay guard): the new guard MUST be `if src.X != nil { dst.X = src.X }` — pointer COPY,
//   NOT `if *src.X { dst.X = src.X }` and NOT `if src.X != nil { *dst.X = *src.X }`. Copying the
//   pointer matches the DiffContext pattern exactly. A deref-guard would re-introduce the
//   only-true-propagates bug (false would fail `*src.X`).

// CRITICAL (cross-package boolPtr): config.boolPtr is UNEXPORTED. Test files OUTSIDE package config
//   (internal/generate, pkg/stagecoach) CANNOT call it. pkg/stagecoach/stagecoach_test.go already
//   defines its own `boolPtr` (line 25) — use it. The internal/generate test package has `strPtr`
//   (multiturn_test.go:758) but NO boolPtr — ADD one `func boolPtr(b bool) *bool { return &b }`
//   there, then convert the 5 MultiTurnFallback assignments to boolPtr(...).

// CRITICAL (Defaults must stay non-nil): Defaults() must seed `AutoStageAll: boolPtr(true)` and
//   `MultiTurnFallback: boolPtr(true)` (non-nil). A nil default + nil file value would make the
//   accessor fall back to true anyway (correct), but the non-nil seed is what lets an explicit
//   *false in a HIGHER layer be the final word after overlay — mirrors DiffContext's intPtr(1) seed.

// config init is UNAFFECTED: bootstrap.go builds the template from hardcoded string literals
//   (b.WriteString "# auto_stage_all = true"), NOT via toml.Marshal. No *bool/encoder helper needed.

// load.go env/flag layers are UNAFFECTED: there is currently NO STAGECOACH_AUTO_STAGE_ALL /
//   STAGECOACH_MULTI_TURN_FALLBACK env case and NO --auto-stage flag (only the local --no-auto-stage
//   bool in default_action.go). Adding env vars is sibling task T2 — do NOT do it here.
```

## Implementation Blueprint

### Data models and structure

No new types. Two existing fields change type from `bool` → `*bool` (resolved struct AND file-decode
structs), mirroring `DiffContext *int`. Two new accessor methods are added.

### Implementation Tasks (ordered by dependencies)

> **Do Tasks 1–8 as one atomic edit set**, then run `go build ./...` once. The type change is
> cross-cutting; incremental compilation is impossible until all sites are converted.

```yaml
Task 1: MODIFY internal/config/config.go — change field types + Defaults seeds + add accessors
  - EDIT field `AutoStageAll bool` (config.go:69) → `AutoStageAll *bool` (KEEP toml tag `auto_stage_all`)
  - EDIT field `MultiTurnFallback bool` (config.go:84) → `MultiTurnFallback *bool` (KEEP toml tag `multi_turn_fallback`)
  - EDIT Defaults() `AutoStageAll: true,` (config.go:189) → `AutoStageAll: boolPtr(true),`
  - EDIT Defaults() `MultiTurnFallback: true,` (config.go:199) → `MultiTurnFallback: boolPtr(true),`
  - ADD accessor after DiffContextValue() (config.go:~233), mirroring it exactly:
        func (c Config) AutoStageAllValue() bool {
            if c.AutoStageAll != nil {
                return *c.AutoStageAll
            }
            return true
        }
        func (c Config) MultiTurnFallbackValue() bool {
            if c.MultiTurnFallback != nil {
                return *c.MultiTurnFallback
            }
            return true
        }
  - boolPtr already exists at config.go:7 — DO NOT re-add it.
  - UPDATE the inline field comments: remove/adjust "only-true-propagates"-style notes; note *bool (nil ⇒ inherit default true).

Task 2: MODIFY internal/config/file.go — file-decode structs + materialize + overlay
  - EDIT `fileDefaults.AutoStageAll bool` (file.go:45) → `*bool`
  - EDIT `fileGeneration.MultiTurnFallback bool` (file.go:55) → `*bool`
  - materialize (file.go:221-222): change
        if d.AutoStageAll { c.AutoStageAll = true }   →   if d.AutoStageAll != nil { c.AutoStageAll = d.AutoStageAll }
    (pointer copy — matches DiffContext materialize at file.go:237-243)
  - materialize (file.go:249-250): change
        if g.MultiTurnFallback { c.MultiTurnFallback = true }   →   if g.MultiTurnFallback != nil { c.MultiTurnFallback = g.MultiTurnFallback }
  - overlay (file.go:343-344): change
        if src.AutoStageAll { dst.AutoStageAll = true }   →   if src.AutoStageAll != nil { dst.AutoStageAll = src.AutoStageAll }
  - overlay (file.go:388-389): change
        if src.MultiTurnFallback { dst.MultiTurnFallback = true }   →   if src.MultiTurnFallback != nil { dst.MultiTurnFallback = src.MultiTurnFallback }
  - UPDATE the inline comments at these sites (they currently say "only-true-propagates / cannot set false via file") to describe the *bool nil-override behavior.
  - DEPENDENCIES: Task 1 (the *Config field must be *bool for the pointer copy to typecheck).

Task 3: MODIFY internal/config/git.go — Layer 4 produces *bool
  - EDIT git.go:161 `c.AutoStageAll = v` → `c.AutoStageAll = boolPtr(v)`   (v is the plain bool from gitConfigBool)
  - CRITICAL: there is NO stagecoach.multiTurnFallback git key — do NOT add one. Only this one line changes.
  - DEPENDENCIES: Task 1. (Mirrors DiffContext git path at git.go:221 `c.DiffContext = intPtr(n)`.)
  - UPDATE comment at git.go:104/109 that claims "autoStageAll=false is a documented no-op" — it is no longer a no-op; *false now propagates.

Task 4: MODIFY the 5 production consumer reads to use accessors
  - internal/cmd/default_action.go:121   `case cfg.AutoStageAll || forceAutoStage:`    → `case cfg.AutoStageAllValue() || forceAutoStage:`
  - internal/cmd/default_action.go:382   `return cfg.AutoStageAll && !noAutoStage`     → `return cfg.AutoStageAllValue() && !noAutoStage`
  - internal/generate/generate.go:394    `if cfg.MultiTurnFallback && !workDescActive &&` → `if cfg.MultiTurnFallbackValue() && !workDescActive &&`
  - internal/hook/exec.go:226            `if cfg.MultiTurnFallback &&`                  → `if cfg.MultiTurnFallbackValue() &&`
  - pkg/stagecoach/stagecoach.go:623     `if cfg.MultiTurnFallback && !workDescActive &&` → `if cfg.MultiTurnFallbackValue() && !workDescActive &&`
  - NOTE: default_action.go:119 and :145 are COMMENTS referencing cfg.AutoStageAll — update wording if needed, but they are not reads.
  - NOTE: internal/generate/multiturn.go:142 is a COMMENT ("Run does NOT check cfg.MultiTurnFallback") — still accurate; no change required.
  - DEPENDENCIES: Task 1.

Task 5: MODIFY existing config-package TEST reads (use accessor or *bool comparison)
  - config_test.go:23    `if !c.AutoStageAll`            → `if !c.AutoStageAllValue()`
  - config_test.go:49    `if !c.MultiTurnFallback`       → `if !c.MultiTurnFallbackValue()`
  - file_test.go:76      `if !cfg.AutoStageAll`          → `if !cfg.AutoStageAllValue()`
  - file_test.go:123     `if !dst.AutoStageAll`          → `if !dst.AutoStageAllValue()`
  - file_test.go:790     `if dst.AutoStageAll != true`   → `if !dst.AutoStageAllValue()`  (TestOverlayNilSrc: nil src must leave the default true)
  - git_test.go:108      `if !cfg.AutoStageAll`          → `if !cfg.AutoStageAllValue()`
  - git_test.go:164      `if cfg.AutoStageAll || ...`    → `if cfg.AutoStageAllValue() || ...`
  - git_test.go:184      `if cfg.AutoStageAll {` (want false) → `if cfg.AutoStageAllValue() {`  (loadGitConfig sets boolPtr(false) for --bool 'off')
  - git_test.go:315      `if !cfg.AutoStageAll`          → `if !cfg.AutoStageAllValue()`
  - git_test.go:372      `if !cfg.AutoStageAll`          → `if !cfg.AutoStageAllValue()`
  - DEPENDENCIES: Task 1-3.

Task 6: MODIFY internal/config/multiturn_test.go — SEMANTICS CHANGE (omit⇒nil now, not false)
  - Lines 47-48, 79, 85, 87, 130, 147, 153, 156 read cfg/c/dst.MultiTurnFallback as bool.
  - OLD materialize left an OMITTED key as Go-zero `false` (the test comment at :48 says so explicitly).
    NEW materialize leaves it `nil`. So:
      * For materialize-only assertions (c.MultiTurnFallback): omitted ⇒ expect `nil`; explicit true ⇒ expect non-nil `*true`; explicit false ⇒ expect non-nil `*false`.
      * For resolved-through-Defaults assertions (cfg.MultiTurnFallback after overlay): use `cfg.MultiTurnFallbackValue()` and expect `true` when omitted (default), `false` when explicitly set false.
  - REWRITE the affected sub-tests' expectations + comments to reflect *bool semantics. The "cannot disable via file" framing in the test messages (lines 85-87, 153-156) is now FALSE — update or replace those messages.
  - DEPENDENCIES: Task 2.

Task 7: MODIFY cross-package TEST assignments (generate + stagecoach) — add/boolPtr
  - pkg/stagecoach/stagecoach_test.go:1327  `cfg.MultiTurnFallback = true` → `cfg.MultiTurnFallback = boolPtr(true)` (boolPtr already defined at :25)
  - internal/generate: ADD `func boolPtr(b bool) *bool { return &b }` to ONE generate _test.go file (e.g. multiturn_test.go, next to the existing `strPtr` at :758), then:
      * multiturn_test.go:549            `cfg.MultiTurnFallback = tc.multiTurn` → `cfg.MultiTurnFallback = boolPtr(tc.multiTurn)`
      * multiturn_test.go:810            `cfg.MultiTurnFallback = true`        → `cfg.MultiTurnFallback = boolPtr(true)`
      * generate_multiturn_failure_test.go:120  `cfg.MultiTurnFallback = true`   → `cfg.MultiTurnFallback = boolPtr(true)`
      * generate_multiturn_test.go:83           `cfg.MultiTurnFallback = true`   → `cfg.MultiTurnFallback = boolPtr(true)`
      * generate_workdesc_test.go:268           `cfg.MultiTurnFallback = true`   → `cfg.MultiTurnFallback = boolPtr(true)`
  - DEPENDENCIES: Task 1.

Task 8: CREATE table-driven *bool overlay/materialize unit test (the load-bearing proof)
  - ADD to internal/config/file_test.go a new test `TestMaterializeOverlay_AutoStageAll_MultiTurnFallback`,
    MIRRORING the structure of `TestMaterializeOverlay_DiffContext_TokenLimit` (file_test.go:798-946):
      (a) materialize-only: file → Config. Table: omitted⇒nil, explicit true⇒*true, explicit false⇒*false.
      (b)/(c)/(d) overlay chain: Defaults (boolPtr(true)) → overlay(global) → overlay(repo). Table covering:
          global_only/false, repo_only/false, global_true_repo_false_repo_wins_false,
          global_false_repo_unset_inherits_false, omitted-everywhere⇒Value()==true.
      End-to-end via loadTOML: a real TOML with `[defaults] auto_stage_all = false` and
          `[generation] multi_turn_fallback = false` must yield accessor()==false after Defaults→overlay.
  - Also add an accessor-focused unit test (can live in config_test.go):
      AutoStageAllValue()/MultiTurnFallbackValue() return true for nil, *true, and false for *false.
  - Use the existing `writeTempTOML(t, body)` helper (file_test.go:14) for the loadTOML sub-test.
  - DEPENDENCIES: Tasks 1-2.

Task 9: MODIFY docs/configuration.md — remove the now-false "cannot disable via file" notes
  - Line 110: rewrite the `multi_turn_fallback` comment — it CAN now be disabled via TOML/git-config (*bool). Remove "CANNOT disable via file".
  - Line 157 (no_verify note): it currently says no_verify "uses the same only-true-propagates limitation as push". Add a clarifying clause that auto_stage_all and multi_turn_fallback are NOW *bool (disabling-via-file works), while no_verify/push REMAIN only-true-propagates because they are default-FALSE (false is already the default, so only-true-propagates is harmless for them).
  - Line 166 (multi_turn_fallback "Limitation" block): rewrite to state multi_turn_fallback CAN now be disabled via TOML (`multi_turn_fallback = false`) and git-config (`stagecoach.autoStageAll`-style *bool behavior); drop the "set session_mode = '' instead" escape hatch as the PRIMARY advice (it can remain as an alternative).
  - DO NOT touch lines 210 or 218 (the snake_case `stagecoach.auto_stage_all` git-config spelling) — that is Issue 2 = sibling task P1.M1.T3.S1.
  - DO NOT touch lines 87/133/140 (the default=true statements) — the default IS still true.
  - DO NOT add env-var docs (STAGECOACH_AUTO_STAGE_ALL) — that is sibling task P1.M1.T2.S1.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the *bool overlay guard (copy the pointer, never deref — file.go + git.go)
// Mirrors file.go:237-243 / file.go:375-382 for DiffContext exactly.
if src.AutoStageAll != nil {        // nil ⇒ this layer omits the key ⇒ inherit lower layer
    dst.AutoStageAll = src.AutoStageAll // pointer COPY (NOT *dst.AutoStageAll = *src.AutoStageAll)
}

// PATTERN: the accessor (config.go, mirrors DiffContextValue at :228-233)
func (c Config) AutoStageAllValue() bool {
    if c.AutoStageAll != nil {   // non-nil (incl. *false) ⇒ explicit override
        return *c.AutoStageAll
    }
    return true                  // nil ⇒ the default-true fallback
}

// PATTERN: git-config layer produces *bool (git.go:161, mirrors DiffContext git path :221)
if v, found, err := gitConfigBool(repoDir, "stagecoach.autoStageAll"); err != nil {
    return nil, err
} else if found {
    c.AutoStageAll = boolPtr(v)   // wrap the plain bool v; omit ⇒ stays nil
}

// PATTERN: cross-package test assignment of *bool (internal/generate tests)
func boolPtr(b bool) *bool { return &b }   // add once to the generate test package
cfg.MultiTurnFallback = boolPtr(true)
```

### Integration Points

```yaml
NO database / migration / routes changes. Pure internal Go type refactor.

CONFIG STRUCT (internal/config/config.go):
  - AutoStageAll:  bool → *bool   (toml tag unchanged: auto_stage_all)
  - MultiTurnFallback: bool → *bool (toml tag unchanged: multi_turn_fallback)
  - Defaults(): both seeded boolPtr(true) (non-nil)
  - +2 accessors: AutoStageAllValue(), MultiTurnFallbackValue()

MERGE LAYER (internal/config/file.go): materialize() + overlay() guards → != nil + pointer copy
GIT LAYER (internal/config/git.go): 1 line (autoStageAll → boolPtr(v))
CONSUMERS (5 reads → accessors): default_action.go x2, generate.go, hook/exec.go, stagecoach.go
DOCS (docs/configuration.md): lines 110, 157, 166 (NOT 210/218 — that's T3)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build everything after the atomic edit set (must be clean before proceeding)
go build ./...

# Vet (catches shadowed vars, unreachable code, printf misuse)
go vet ./...

# Format check
gofmt -l internal/ pkg/ cmd/
# Expected: no files listed. If listed, run `gofmt -w <file>`.

# Lint (project uses golangci-lint; .golangci.yml present)
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Config package — the heart of this change
go test ./internal/config/... -run 'AutoStageAll|MultiTurnFallback|MaterializeOverlay|Overlay|LoadGitConfig|Defaults' -v

# Full config package
go test ./internal/config/... -v

# Affected consumer packages (their MultiTurnFallback tests must still pass)
go test ./internal/generate/... -v
go test ./internal/hook/... -v
go test ./internal/cmd/... -v
go test ./pkg/stagecoach/... -v

# Whole suite with race detector (the project's standard `make test`)
make test
# Expected: ALL pass. The new TestMaterializeOverlay_AutoStageAll_MultiTurnFallback must pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary
make build

# Manual end-to-end proof of Issue 1's fix (mirror the PRD reproduction):
#   1. git init a throwaway repo, seed a commit, leave a dirty un-staged tree (echo b > b.txt)
#   2. write a config file with [defaults] auto_stage_all = false + a stub provider
#   3. ./bin/stagecoach --config <file>  → EXPECT exit 2 "Nothing to commit." (NOT an auto-staged commit)
#
# (A scripted version of this is the deliverable of sibling task P1.M1.T1.S2; here, the unit-level
#  loadTOML→overlay test in Task 8 is the within-scope proof. A quick manual smoke test is recommended
#  but not strictly required for this subtask's gates.)

# Confirm config init still emits a valid template (hardcoded strings — must be byte-identical)
./bin/stagecoach config init 2>/dev/null | grep -E 'auto_stage_all|multi_turn_fallback'
# Expected: comment lines still present and correct.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: prove NO plain-bool reads/writes of these fields remain (should be empty)
grep -rn '\.AutoStageAll\b\|\.MultiTurnFallback\b' --include='*.go' . \
  | grep -vE 'Value\(\)|\*bool|boolPtr|!= nil|== nil|//|/\*'
# Expected: empty (every remaining reference is an accessor call, a *bool-typed expr, or a comment).

# Confirm the two sibling-subtask scope boundaries were respected:
#  (a) NO env var added for these fields (T2's job)
grep -rn 'STAGECOACH_AUTO_STAGE_ALL\|STAGECOACH_MULTI_TURN_FALLBACK' internal/config/load.go
# Expected: empty.
#  (b) docs git-config spelling lines untouched (T3's job)
git diff docs/configuration.md   # lines 210 & 218 must NOT appear in the diff
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/ pkg/ cmd/` lists nothing
- [ ] `make lint` zero errors
- [ ] `make test` (race) — all pass, including new `TestMaterializeOverlay_AutoStageAll_MultiTurnFallback`

### Feature Validation
- [ ] `Defaults().AutoStageAllValue() == true` and `MultiTurnFallbackValue() == true`
- [ ] `[defaults] auto_stage_all = false` → resolved `AutoStageAllValue() == false` (unit test, Task 8)
- [ ] `[generation] multi_turn_fallback = false` → resolved `MultiTurnFallbackValue() == false`
- [ ] `git config stagecoach.autoStageAll false` → git_test.go false-path passes via accessor
- [ ] Omitted keys ⇒ accessor returns `true` (default preserved)
- [ ] `config init` template output unchanged (hardcoded strings)

### Scope-Boundary Validation
- [ ] NO `STAGECOACH_AUTO_STAGE_ALL`/`STAGECOACH_MULTI_TURN_FALLBACK` added to load.go (that's T2)
- [ ] docs/configuration.md lines 210 & 218 UNTOUCHED (that's T3)
- [ ] Only `auto_stage_all`/`multi_turn_fallback` fields changed; `Verbose`/`Push`/`NoVerify` stay plain `bool`

### Code Quality & Docs
- [ ] Accessors mirror `DiffContextValue()` exactly
- [ ] Inline comments at changed sites updated (no stale "only-true-propagates" / "cannot set false" claims)
- [ ] docs/configuration.md lines 110/157/166 updated; cross-package `boolPtr` added to generate test pkg
- [ ] No new exported helpers added to the config package (boolPtr stays unexported)

---

## Anti-Patterns to Avoid

- ❌ Don't use a deref guard `if *src.AutoStageAll` or `*dst.AutoStageAll = *src.AutoStageAll` — that re-introduces only-true-propagates. Use the pointer-copy `!= nil` guard, exactly like DiffContext.
- ❌ Don't add the env vars (STAGECOACH_AUTO_STAGE_ALL / MULTI_TURN_FALLBACK) here — that's sibling task T2.
- ❌ Don't "fix" the docs git-config snake_case spelling (lines 210/218) here — that's sibling task T3.
- ❌ Don't add a `stagecoach.multiTurnFallback` git-config key — the implementation has none today and adding one is out of scope for this subtask (Issue 1 is about the existing overlay bug, not new keys).
- ❌ Don't convert `Verbose`/`Push`/`NoVerify` — they are default-FALSE, so only-true-propagates is harmless for them and they have env/flag escape hatches. Only the two default-TRUE booleans need *bool.
- ❌ Don't try to compile/run tests after a partial edit set — `bool→*bool` is all-or-nothing. Convert every site first.
- ❌ Don't export `boolPtr` from the config package to satisfy cross-package tests — add a local `boolPtr` in the consuming test package (pkg/stagecoach already has one; generate needs one). Mirrors the existing local-helper convention.

---

## Confidence Score: 9/10

One-pass success is very high: the change is a 1:1 mirror of the already-working `DiffContext *int`
pattern, all sites are enumerated with current line numbers (verified by grep), the cross-package
`boolPtr` gotcha is called out, and the "config-init-is-hardcoded-strings" fact removes the one
ambiguous point in the task description. The -1 is for the `multiturn_test.go` semantics rewrite
(Task 6) — those tests encoded the OLD broken behavior in their assertion messages, so the implementer
must read each affected sub-test and re-derive the *bool-correct expectation rather than blindly
s/bool/*bool/; a careless pass there could leave a stale message (though the assertion logic itself
is mechanical).
