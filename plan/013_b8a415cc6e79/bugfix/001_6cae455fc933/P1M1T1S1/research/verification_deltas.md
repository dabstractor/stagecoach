# Research Notes — P1.M1.T1.S1 (*bool conversion)

Verification of the task-description line numbers / claims against the CURRENT working tree
(2026-07-09). The pre-existing architecture docs (`architecture/system_context.md`,
`architecture/research_config_precedence.md`) are accurate in substance; these notes record the
DELTAS and the extra sites the task-description bullet list did not enumerate.

## DELTA 1 — Line numbers drifted (use these CURRENT numbers)

| Site | Task desc said | Actual (current tree) |
|------|----------------|------------------------|
| `Config.AutoStageAll` field | config.go:65 | **config.go:69** |
| `Config.MultiTurnFallback` field | config.go:81 | **config.go:84** |
| `Defaults()` AutoStageAll | config.go:345 | **config.go:189** |
| `Defaults()` MultiTurnFallback | config.go:353 | **config.go:199** |
| `DiffContextValue()` accessor | config.go:169-176 | **config.go:228-233** (one-line body) |
| `fileDefaults.AutoStageAll` | file.go:45 | **file.go:45** ✓ |
| `fileGeneration.MultiTurnFallback` | file.go:55 | **file.go:55** ✓ |
| materialize AutoStageAll | file.go:221-222 | **file.go:221-222** ✓ |
| materialize MultiTurnFallback | file.go:247-250 | **file.go:247-250** ✓ (guard body at 249-250) |
| overlay AutoStageAll | file.go:343-344 | **file.go:343-344** ✓ |
| overlay MultiTurnFallback | file.go:386-389 | **file.go:386-389** ✓ (guard body at 388-389) |
| git.go AutoStageAll | git.go:158-161 | **git.go:158-161** ✓ |

## DELTA 2 — `config init` does NOT use the TOML encoder (task point i is MOOT)

Task point (i) worries the TOML encoder might mishandle `*bool` when serializing
`auto_stage_all = true`. VERIFIED: `config init` builds its template from **hardcoded string
literals** (`internal/config/bootstrap.go:161` and `:304`), NOT via `toml.Marshal`/`toml.NewEncoder`.
Grep for `toml.NewEncoder|toml.Marshal` in non-test config code → ZERO hits. So `config init`
output is unaffected by the type change. No encoder helper needed.

## DELTA 3 — git.go has NO `multi_turn_fallback` key (task point g is AutoStageAll-only)

Task point (g) only mentions AutoStageAll. VERIFIED correct: `git.go` reads
`stagecoach.autoStageAll` (git.go:158) but has **no** `stagecoach.multiTurnFallback` read
(grep for MultiTurnFallback in git.go → only comments, no read site). So point (g) is a SINGLE
edit: `git.go:161` `c.AutoStageAll = v` → `c.AutoStageAll = boolPtr(v)`.

## DELTA 4 — More TEST sites than the task listed

The task says "existing file_test.go and load_test.go bool-assertions must be updated." The real
list is larger. All compile-fail after `bool → *bool`.

### Reads (assertions) — package `config` (can use unexported `boolPtr` / accessor)
- `config_test.go:23-24` — `if !c.AutoStageAll` (Defaults test) → `!c.AutoStageAllValue()`
- `config_test.go:49-50` — `if !c.MultiTurnFallback` → `!c.MultiTurnFallbackValue()`
- `file_test.go:76-77` — `if !cfg.AutoStageAll` → `!cfg.AutoStageAllValue()`
- `file_test.go:123-124` — `if !dst.AutoStageAll` → `!dst.AutoStageAllValue()`
- `file_test.go:790-791` — `if dst.AutoStageAll != true` → `!dst.AutoStageAllValue()`
- `git_test.go:108-109, 164, 184-185, 315-316, 372-373` — reads → accessor
- `multiturn_test.go:47-48, 79, 85, 87, 130, 147, 153, 156` — reads. **SEMANTICS CHANGE:**
  old materialize left omitted MultiTurnFallback as Go-zero `false`; new materialize leaves it
  `nil`. Update expectations: omitted ⇒ nil; explicit true ⇒ *true; explicit false ⇒ *false.

### Assignments — package `config` (use unexported `boolPtr`)
- none currently in config pkg test code (multiturn_test reads only).

### Assignments — OTHER packages (CANNOT call `config.boolPtr`)
- `internal/generate/multiturn_test.go:549` — `cfg.MultiTurnFallback = tc.multiTurn`
- `internal/generate/multiturn_test.go:810` — `cfg.MultiTurnFallback = true`
- `internal/generate/generate_multiturn_failure_test.go:120` — `cfg.MultiTurnFallback = true`
- `internal/generate/generate_multiturn_test.go:83` — `cfg.MultiTurnFallback = true`
- `internal/generate/generate_workdesc_test.go:268` — `cfg.MultiTurnFallback = true`
- `pkg/stagecoach/stagecoach_test.go:1327` — `cfg.MultiTurnFallback = true`

**Cross-package `boolPtr` resolution (the key one-pass gotcha):**
- `pkg/stagecoach/stagecoach_test.go` ALREADY defines `func boolPtr(b bool) *bool { return &b }`
  at line 25 → assignments become `boolPtr(true)` / `boolPtr(tc.multiTurn)`.
- `internal/generate` test files are `package generate`; they already have `func strPtr` at
  `multiturn_test.go:758` but NO `boolPtr`. **ADD** a local `func boolPtr(b bool) *bool { return &b }`
  to the generate test package (one definition serves all generate _test.go files). Then convert
  the 5 assignments above to `boolPtr(...)`.
- (DiffContext *int is NOT a precedent for cross-pkg *Config assignment: in generate/stagecoach
  tests DiffContext appears only as a plain-int field of `git.StagedDiffOptions`, never assigned
  to a `Config.DiffContext` pointer. So MultiTurnFallback is genuinely the first cross-pkg
  pointer assignment and needs the helper.)

## DELTA 5 — Docs line numbers (use these CURRENT numbers)

| Docs line | Content | Action for THIS subtask |
|-----------|---------|--------------------------|
| 110 | `# multi_turn_fallback = true ... CANNOT disable via file` | Rewrite comment: CAN now be disabled via TOML/git-config (*bool) |
| 157 | `no_verify` note: "uses the same only-true-propagates limitation as push" | Add that auto_stage_all/multi_turn_fallback are now *bool (no longer only-true-propagates); no_verify/push REMAIN only-true-propagates (default-false) |
| 166 | `multi_turn_fallback` "Limitation" block: "cannot disable via file" | Rewrite: CAN now be disabled via TOML + git-config (`stagecoach.autoStageAll`-style *bool) |
| 87, 133, 140 | `auto_stage_all`/`multi_turn_fallback` default = true | No change (default IS still true) |
| 210, 218 | git-config INI example + table row use snake_case `auto_stage_all` | **OUT OF SCOPE** — that's Issue 2 = P1.M1.T3.S1 (separate subtask). Do NOT touch here. |

## SCOPE BOUNDARIES (do NOT do these — they are sibling subtasks)
- **P1.M1.T2.S1** (Issue 3): add `STAGECOACH_AUTO_STAGE_ALL` / `STAGECOACH_MULTI_TURN_FALLBACK`
  env vars in `loadEnv`. Do NOT add env handling here.
- **P1.M1.T3.S1** (Issue 2): fix git-config key spelling docs (lines 210/218 snake→camel).
  Do NOT touch those docs lines here.
- **P1.M1.T1.S2**: end-to-end integration tests (TOML/git-config false disables behavior).
  This subtask does UNIT-level *bool overlay/materialize/accessor tests only.
