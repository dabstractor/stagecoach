# Research: Config-Precedence Layer — Default-True Boolean Fields

Validates PRD Issue 1 (only-true-propagates bool bug) and Issue 3 (missing env vars).
Scope: `internal/config/{config.go,file.go,load.go}`. All line numbers verified against working tree.

## 1. CONFIRMED: `auto_stage_all` and `multi_turn_fallback` use only-true-propagates

Both fields are plain `bool` in both the FILE-decode struct and the resolved `Config`.
At every merge site the guard is `if X { dst.X = true }`, silently ignoring `false`.

### `materialize` (file.go:208) — file → partial `*Config`
```go
// file.go:221-222
if d.AutoStageAll {
    c.AutoStageAll = true // v1 limitation: cannot set false via file
}
// file.go:247-250
if g.MultiTurnFallback {
    c.MultiTurnFallback = true
}
```

### `overlay` (file.go:326) — layer merge (global → repo → gitconfig)
```go
// file.go:343-344
if src.AutoStageAll { dst.AutoStageAll = true }
// file.go:386-389
if src.MultiTurnFallback { dst.MultiTurnFallback = true }
```

Same pattern also affects `Verbose`, `Push`, `NoVerify` — but those have env/flag DIRECT-set escape hatches. `AutoStageAll` and `MultiTurnFallback` do NOT.

## 2. CONFIRMED: No `STAGECOACH_AUTO_STAGE_ALL` / `STAGECOACH_MULTI_TURN_FALLBACK` in loadEnv

`loadEnv` is at load.go:229-323. Grep across `internal/` and `cmd/` returns NONE FOUND.

### Complete `STAGECOACH_*` env vars handled in loadEnv
| Env var | load.go line | Type |
|---|---|---|
| `STAGECOACH_PROVIDER` | 230 | string |
| `STAGECOACH_MODEL` | 233 | string |
| `STAGECOACH_REASONING` | 236 | string |
| `STAGECOACH_TIMEOUT` | 239 | duration |
| `STAGECOACH_VERBOSE` | 246 | `ParseBool` DIRECT |
| `STAGECOACH_NO_COLOR` | 253 | `ParseBool` DIRECT |
| `STAGECOACH_<ROLE>_PROVIDER/MODEL/REASONING` | 264-270 | string per-role |
| `STAGECOACH_COMMITS` | 277 | int |
| `STAGECOACH_FORMAT` | 290 | string |
| `STAGECOACH_LOCALE` | 293 | string |
| `STAGECOACH_TEMPLATE` | 296 | string |
| `STAGECOACH_PUSH` | 301 | `ParseBool` DIRECT |
| `STAGECOACH_NO_VERIFY` | 310 | `ParseBool` DIRECT |
| `STAGECOACH_WORK_DESCRIPTION` | 320 | string |

## 3. Config struct field types (config.go:60-159)

### Plain `bool` fields (cannot distinguish unset from explicit false)
- `AutoStageAll bool` (config.go:65) — **default TRUE** (Defaults: config.go:345)
- `Verbose bool` (config.go:66/70) — default false
- `MultiTurnFallback bool` (config.go:81) — **default TRUE** (Defaults: config.go:353)
- `Push bool` (config.go:123) — default false
- `NoVerify bool` (config.go:131) — default false

### `*bool` pointer fields
- `StripCodeFence *bool` (config.go:99) — nil ⇒ true (manifest)

### `*int` pointer fields (nil = unset; non-nil incl. *0 = explicit)
- `DiffContext *int` (config.go:77) — **THE canonical precedent for the *bool fix**

## 4. The `*int` "explicitly set" pattern (DiffContext)

Helper `intPtr` at config.go:9; `boolPtr` already exists at config.go:7.

Defaults (config.go:351): `DiffContext: intPtr(1)` — non-nil default.
materialize (file.go:237-243): `if g.DiffContext != nil { c.DiffContext = g.DiffContext }`
overlay (file.go:375-382): `if src.DiffContext != nil { dst.DiffContext = src.DiffContext }`
Accessor (config.go:169-176): `DiffContextValue()` — nil → 1; non-nil → `*c.DiffContext`

## 5. STAGECOACH_VERBOSE parsing — `strconv.ParseBool`, rejects `2`

load.go:246-251. `"2"` → error `strconv.ParseBool: parsing "2": invalid syntax`. Hard load failure (exit 1).

## Architecture / data-flow

Precedence (load.go `Load`, layers low → high):
1. `Defaults()` → 2. global TOML → 3. repo TOML → 4. git config → 5. env (DIRECT) → 6/7. flags (DIRECT)

`materialize` and `overlay` use non-zero guards, so file+gitconfig layers CANNOT set false.
The env/flag layers compensate by writing DIRECTLY. AutoStageAll/MultiTurnFallback lack env/flag hatches.
