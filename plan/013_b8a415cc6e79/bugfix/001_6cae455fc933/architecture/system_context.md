# System Context — Stagecoach Config-Precedence & Diagnostics Bugfix

## Project
`github.com/dustin/stagecoach` — a Go CLI (module `go 1.22`) that automates atomic git
commits via AI agent providers. The commit pipeline, freeze invariants, hook lifecycle, and
rescue/CAS error handling are all verified sound. This bugfix targets the **config-precedence
layer for default-`true` boolean fields** and several **minor diagnostics gaps**.

## Config Precedence Architecture (the core of Issues 1–3)

Stagecoach resolves configuration across 7 layers, lowest → highest precedence
(`internal/config/load.go` `Load()`):

1. **`Defaults()`** (`config.go:Defaults()`) — built-in defaults (by value)
2. **Global TOML** (`~/.config/stagecoach/config.toml`) — `loadTOML` → `materialize` → `overlay`
3. **Repo-local TOML** (`.stagecoach.toml`) — `loadRepoLocalConfig` → `overlay`
4. **Repo git config** (`git config stagecoach.*`) — `loadGitConfig` → `overlay`
5. **Environment** (`STAGECOACH_*`) — `loadEnv` (DIRECT set on `Config`)
6/7. **CLI flags** (`--flag`) — `loadFlags` (DIRECT set, `fs.Changed` only)

### The Merge Mechanism (`overlay` / `materialize`)

Layers 2–4 merge through `overlay(dst, src *Config)` (`file.go:326`) and
`materialize(d *fileConfig) *Config` (`file.go:208`). These use **non-zero/non-empty/only-true
guards** so a layer cannot set a field to its zero value (false/0/""). The env (5) and flag
(6/7) layers compensate by writing DIRECTLY (`cfg.X = v`), bypassing overlay.

### The Bug: default-`true` booleans `AutoStageAll` and `MultiTurnFallback`

Both are plain `bool` (default `true`) in the resolved `Config` AND the file-decode struct.
At every merge site the guard is one-way:

```go
// file.go materialize:221-222 / overlay:343-344
if d.AutoStageAll { c.AutoStageAll = true }  // ← only-true-propagates; false is SILENTLY DROPPED
```

Because `false` is the zero value, the overlay cannot distinguish "not set in this layer"
from "explicitly set to false." A user who writes `auto_stage_all = false` in TOML gets
**silently ignored** — stagecoach behaves as `true`. The same applies to `multi_turn_fallback`.

**No escape hatch exists for these two fields:** unlike `Verbose`/`Push`/`NoVerify` (which
have env+flag DIRECT-set hatches), `AutoStageAll` and `MultiTurnFallback` have:
- No env var (no `STAGECOACH_AUTO_STAGE_ALL` / `STAGECOACH_MULTI_TURN_FALLBACK` in `loadEnv`)
- No `--auto-stage` flag (only the inverting `--no-auto-stage`, which is a separate local bool,
  not part of `Config`)
- A working git-config key (`stagecoach.autoStageAll`, camelCase) but it hits the same overlay bug

### The Proven Fix Pattern: `*bool` (mirrors `DiffContext *int`)

`DiffContext *int` (`config.go:77`) is the existing, field-tested precedent for
"nil = inherit lower layer; non-nil (incl. zero) = explicit override." It works end-to-end:
- `Config.DiffContext *int` (resolved struct)
- `fileDefaults.DiffContext *int` (file-decode struct)
- `materialize` (`file.go:237-243`): `if g.DiffContext != nil { c.DiffContext = g.DiffContext }`
- `overlay` (`file.go:375-382`): `if src.DiffContext != nil { dst.DiffContext = src.DiffContext }`
- `Defaults()` (`config.go:351`): `DiffContext: intPtr(1)` (non-nil default so higher layer can clear)
- Accessor `DiffContextValue()` (`config.go:169-176`): nil → fallback 1; non-nil → `*c.DiffContext`
- Helper `boolPtr` already exists at `config.go:7`

**Applying this to `AutoStageAll` and `MultiTurnFallback`:** convert both to `*bool` end-to-end.
This is a cross-cutting type change but follows a 1:1 proven pattern. Blast radius (verified):

| Site | File:Line | Current | After |
|------|-----------|---------|-------|
| Struct field | `config.go:65,81` | `bool` | `*bool` |
| File-decode field | `file.go:45,55` | `bool` | `*bool` |
| Defaults | `config.go:345,353` | `true` | `boolPtr(true)` |
| materialize | `file.go:221-222,247-250` | `if d.X { c.X = true }` | `if d.X != nil { c.X = d.X }` |
| overlay | `file.go:343-344,386-389` | `if src.X { dst.X = true }` | `if src.X != nil { dst.X = src.X }` |
| git.go | `git.go:158-161` | `c.AutoStageAll = v` | `c.AutoStageAll = boolPtr(v)` |
| Consumer | `default_action.go:121` | `cfg.AutoStageAll` | `cfg.AutoStageAllValue()` |
| Consumer | `default_action.go:382` | `cfg.AutoStageAll` | `cfg.AutoStageAllValue()` |
| Consumer | `generate.go:394` | `cfg.MultiTurnFallback` | `cfg.MultiTurnFallbackValue()` |
| Consumer | `hook/exec.go:226` | `cfg.MultiTurnFallback` | `cfg.MultiTurnFallbackValue()` |
| Consumer | `stagecoach.go:623` | `cfg.MultiTurnFallback` | `cfg.MultiTurnFallbackValue()` |

New accessors `AutoStageAllValue()` and `MultiTurnFallbackValue()` mirror `DiffContextValue()`.

## Git-Config Key Naming (Issue 2)

`internal/config/git.go` reads **camelCase** keys exclusively (git forbids underscores in the
final config-key segment). All 8 multi-word keys are camelCase: `autoStageAll`, `stripCodeFence`,
`noVerify`, `maxDiffBytes`, `maxMdLines`, `tokenLimit`, `diffContext`, `maxDuplicateRetries`,
`subjectTargetChars`. The implementation is internally consistent.

**`docs/configuration.md` is the side that's wrong:** lines 210 (INI example) and 218 (table row)
use snake_case `auto_stage_all`, which git rejects as an invalid key. Two edits fix it.
See `research_gitconfig_keys.md` for the complete key list and exact line numbers.

## Diagnostics Layer (Issues 4–6)

### Verbose (Issue 4)
`Config.Verbose` is a plain `bool`; `STAGECOACH_VERBOSE` is parsed via `strconv.ParseBool`
which rejects `"2"`. The code explicitly documents VERBOSE=2 as deferred (`verbose.go:30-33`).
PRD §19 advertises `STAGECOACH_VERBOSE=2` for stdin-contents logging. **Minimal fix:**
gracefully reject `"2"` with a clear message ("VERBOSE=2 is not yet supported") instead of an
opaque parse error. (Full implementation would promote `Verbose` to `int` — cross-cutting,
~10 sites — out of scope for a bugfix.)

### Payload Size (Issue 5)
`executor.go:64` calls `vb.VerbosePayload(len(spec.Stdin))`. For positional/flag-delivery
providers, `spec.Stdin == ""` (payload appended to `spec.Args`), so `VerbosePayload(0)` is a
no-op (`verbose.go:97` `bytes <= 0` guard). `CmdSpec` deliberately does not carry the delivery
mode. **Clean fix:** add a `PayloadBytes int` field to `CmdSpec`, set it in `Render`'s delivery
switch (`render.go:163-178` and multi-turn `render.go:274-282`) for ALL three modes, then
`executor.go:64` calls `vb.VerbosePayload(spec.PayloadBytes)`. Delivery modes: `stdin`
(default, most providers), `positional` (cursor), `flag` (user-defined manifests).

### Model Shadowing (Issue 6, by-spec)
`ResolveRoleModel` (`roles.go:34-57`) makes per-role config beat global `--model`. This is
correct per FR-R3 but is a UX footgun: `stagecoach --model glm-5.2` with `[role.message] model`
set silently ignores `--model`. **Fix:** add a `--verbose` hint in `default_action.go` (~line 176)
when `fs.Changed("model")`/`fs.Changed("provider")` AND `cfg.Roles["message"]` is non-empty.
No behavioral change required.

## Research Artifacts
- `research_config_precedence.md` — detailed Issue 1 & 3 validation (struct types, overlay code, env list, *int precedent)
- `research_gitconfig_keys.md` — detailed Issue 2 validation (all 18 git keys, all docs lines, exact edits)
- `research_provider_verbose.md` — detailed Issues 4, 5, 6 validation (executor, render, verbose, roles)
- `external_deps.md` — git config naming rules, ParseBool behavior, delivery-mode semantics
