# Research: Current Timeout Handling (FR-R7 — Per-Role Timeouts)

> Scope: how generation timeouts work TODAY in stagecoach, to enable FR-R7 (per-role timeouts, PRD v2.8 §9.15).
> All paths/line numbers verified against the working tree on 2026-07-10.

## Executive Summary

There is **exactly one** generation timeout today: `Config.Timeout` (`time.Duration`, default **480s**). Every agent invocation — single-commit message, multi-turn fallback, and all four decompose roles (planner/stager/message/arbiter) — passes the **same** `cfg.Timeout` / `deps.Config.Timeout` as the 3rd argument to `provider.Execute`. There is **no per-role timeout concept**: `RoleConfig` has only `{Provider, Model, Reasoning}`, and there is no `ResolveRoleTimeout`.

FR-R7 changes this: each role resolves its OWN timeout, with the **global default dropping to 120s** and a **planner built-in default of 480s**. The new timeout resolution must mirror the existing `ResolveRoleModel` pattern exactly.

---

## 1. How `timeout` is configured today

### 1.1 The Config field (resolved, flat, plain-typed)

**`internal/config/config.go:68`**
```go
Timeout      time.Duration `toml:"timeout"`        // generation timeout; Defaults: 480s
```
This is the fully-resolved `time.Duration` on the flat `Config` struct. Consumers read `cfg.Timeout` directly (zero dereferencing). It is **NOT** unmarshaled from the §16.2 file as a duration — the file holds it as a STRING (see 1.4).

There is a SEPARATE timeout field for hooks (not generation):
**`internal/config/config.go:153-155`**
```go
HookTimeout time.Duration `toml:"hook_timeout"`   // §9.25 FR-V6 per-hook execution timeout (default 10m)
```
This is file+default-only (no env/flag/git) and out of scope for FR-R7.

### 1.2 Default value (Layer 1)

**`internal/config/config.go:197`** (inside `Defaults()`):
```go
Timeout:              480 * time.Second,
```
⚠️ **FR-R7 CHANGES THIS**: the global default must become **120s**, with a *planner* role-specific default of **480s** (PRD §16.1 line 1635, §16.2 example line 1658). Currently nothing is per-role.

### 1.3 Precedence layers (7-layer, §16.1, lowest→highest)

`Load()` (`internal/config/load.go:71`) applies layers:
1. **Built-in `Defaults()`** — `Timeout: 480s` (config.go:197)
2. Global TOML file
3. Repo-local TOML file (`.stagecoach.toml`)
4. Repo git config — `stagecoach.timeout` (git.go:148-156)
5. Env — `STAGECOACH_TIMEOUT` (load.go:260-266)
6. (no layer 6)
7. CLI flag — `--timeout` (load.go:411-416)

Each layer overlays the previous (non-zero wins for scalars). Booleans use DIRECT set for the false-escape-hatch.

#### 1.3a Git-config layer (layer 4)
**`internal/config/git.go:147-156`**
```go
// --- timeout: accepts both "90" (seconds) and "90s" (Go duration) forms. ---
if v, found, err := gitConfigGet(repoDir, "stagecoach.timeout"); err != nil {
    return nil, err
} else if found {
    d, perr := parseTimeout(v) // parseTimeout handles both "90" and "90s"
    if perr != nil {
        return nil, fmt.Errorf("git config stagecoach.timeout: %w", perr)
    }
    c.Timeout = d
}
```
⚠️ **CRITICAL FINDING**: the git-config layer loads **NO per-role keys at all** — not even for provider/model/reasoning. A grep for `role.` / `stagecoach.role` / `roleConfig` in git.go returns nothing, and `git_test.go` has no role/planner tests. The `--planner-model` flag help text says "git stagecoach.role.planner" but that git key is **not actually read** today. Per-role overrides today come only from **file** (`[role.<role>]`), **env** (`STAGECOACH_<ROLE>_*`), and **CLI flags** (`--<role>-*`). FR-R7's changelog mentions `stagecoach.role.<role>.timeout` as a git key — that requires NEW git-config role loading (see §5 note).

#### 1.3b Env layer (layer 5)
**`internal/config/load.go:260-266`**
```go
if v, ok := os.LookupEnv("STAGECOACH_TIMEOUT"); ok && v != "" {
    d, err := parseTimeout(v)
    if err != nil {
        return fmt.Errorf("STAGECOACH_TIMEOUT: %w", err)
    }
    cfg.Timeout = d
}
```
Per-role env uses a loop (load.go:288-303) over `roleNames`:
```go
for _, role := range roleNames {
    prefix := "STAGECOACH_" + strings.ToUpper(role)
    if v, ok := os.LookupEnv(prefix + "_PROVIDER"); ok && v != "" { cfg.setRoleProvider(role, v) }
    if v, ok := os.LookupEnv(prefix + "_MODEL"); ok && v != ""    { cfg.setRoleModel(role, v) }
    if v, ok := os.LookupEnv(prefix + "_REASONING"); ok && v != "" { cfg.setRoleReasoning(role, v) }
}
```
`roleNames` is defined once: **`internal/config/load.go:17`** `var roleNames = []string{"planner", "stager", "message", "arbiter"}`. FR-R7 must add a `_TIMEOUT` branch here (calling a new `setRoleTimeout`).

#### 1.3c CLI flag layer (layer 7)
**`internal/config/load.go:411-416`**
```go
if fs.Changed("timeout") {
    if v, err := fs.GetString("timeout"); err == nil {
        if d, perr := parseTimeout(v); perr == nil {
            cfg.Timeout = d
        }
    }
}
```
Per-role flags use the same loop pattern (load.go:427-445):
```go
for _, role := range roleNames {
    if fs.Changed(role + "-provider") { ... cfg.setRoleProvider(role, v) }
    if fs.Changed(role + "-model")    { ... cfg.setRoleModel(role, v) }
    if fs.Changed(role + "-reasoning"){ ... cfg.setRoleReasoning(role, v) }
}
```
FR-R7 must add a `<role>-timeout` branch (calling `setRoleTimeout` + `parseTimeout`).

### 1.4 The `parseTimeout` helper (shared by env + flag + git)

**`internal/config/load.go:615-625`**
```go
// parseTimeout parses a duration that may be EITHER a Go duration string ("120s", "2m") OR a bare
// integer (seconds: "120"). Used by both STAGECOACH_TIMEOUT (env) and --timeout (CLI).
func parseTimeout(s string) (time.Duration, error) {
    if d, err := time.ParseDuration(s); err == nil {
        return d, nil
    }
    if n, err := strconv.Atoi(s); err == nil {
        return time.Duration(n) * time.Second, nil
    }
    return 0, fmt.Errorf("invalid timeout %q (expected e.g. \"120s\" or 120)", s)
}
```
This is the single parse helper to reuse for per-role timeout strings.

### 1.5 File decode (STRING → duration parsed at load)

**`internal/config/file.go:30`** — `fileDefaults` holds timeout as a STRING (go-toml/v2 cannot decode "120s" into `time.Duration`):
```go
type fileDefaults struct {
    ...
    Timeout      string `toml:"timeout"`        // §16.2 duration string, e.g. "120s"; parsed in loadTOML
    ...
}
```
Parsed up-front in `loadTOML` (**`file.go:179-186`**) and passed into `materialize`:
```go
var timeout time.Duration
if fc.Defaults.Timeout != "" {
    timeout, err = time.ParseDuration(fc.Defaults.Timeout)
    if err != nil {
        return nil, fmt.Errorf("parse config %s: invalid timeout %q: %w", path, fc.Defaults.Timeout, err)
    }
}
```
(Note: file.go's loadTOML uses `time.ParseDuration` directly here, NOT the `parseTimeout` bare-int helper — so the **file layer only accepts Go duration strings like "120s", NOT bare "120"**, unlike env/flag/git which accept bare integers. This is a pre-existing inconsistency; per-role file timeouts should decide which form to accept.)

`materialize` seeds the resolved duration: **`file.go:205`** `func materialize(fc *fileConfig, timeout, hookTimeout time.Duration) *Config { c := &Config{Timeout: timeout, ...}`.

---

## 2. How timeout is consumed in generation

### 2.1 The single-commit path

**`internal/generate/generate.go:335`** — inside `CommitStaged`'s retry loop:
```go
out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
```
On `context.DeadlineExceeded` → immediate `&RescueError{Kind: ErrTimeout, ...}` (generate.go:336-341), which the orchestrator maps to **exit 124**.

The resolved `msgModel` / `msgReasoning` for the message role are resolved at the top of `CommitStaged` via `config.ResolveRoleModel("message", ...)` (search `default_action.go` for the buildDeps resolution). Today the timeout is the flat `cfg.Timeout` — **not** resolved per-role.

### 2.2 Where the deadline is actually applied (the chokepoint)

**`internal/provider/executor.go:44-52`** — `Execute` SHADOWS the ctx with `WithTimeout`:
```go
func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration, vb *ui.Verbose) (...) {
    if timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, timeout) // SHADOW — see doc; do not rename
        defer cancel()
    }
    cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
    ...
}
```
**This is the ONE place the timeout becomes a context deadline.** FR-R7 does NOT need to touch `Execute` — it only needs to pass a *role-resolved* duration instead of the flat `cfg.Timeout`. The `timeout time.Duration` parameter is already per-call.

### 2.3 The `ErrTimeout` sentinel + exit mapping

**`internal/generate/generate.go:87-90`**
```go
var ErrTimeout = errors.New("stagecoach: generation timed out")
```
`*RescueError{Kind: ErrTimeout}` → `exitcode.For` maps Timeout→**124**, Rescue→**3** (generate.go:110-124). Per-role timeouts must keep using this same `ErrTimeout`/`RescueError` machinery so exit codes are unchanged.

---

## 3. How the decompose pipeline uses timeouts (ALL 4 ROLES = SHARED `deps.Config.Timeout`)

Every decompose role calls `provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)` — the SAME global value. There is no per-role resolution today. This is the core thing FR-R7 changes.

| Role | File:line | Code | On timeout |
|------|-----------|------|-----------|
| **planner** | `internal/decompose/planner.go:124` | `out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)` | wrapped `ErrPlannerFailed` — **non-rescue** (no retry; planning precedes staging) |
| **stager** | `internal/decompose/stager.go:110` | `if _, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose); execErr != nil` | wrapped `ErrStagerFailed` |
| **message** | `internal/decompose/message.go:155` | `out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)` | `&generate.RescueError{Kind: generate.ErrTimeout, ...}` (exit 124) |
| **arbiter** | `internal/decompose/arbiter.go:100` | `out, _, execErr := provider.Execute(ctx, *spec, deps.Config.Timeout, deps.Verbose)` | **graceful null** — `return prompt.ArbiterOutput{Target: nil}, nil` (the arbiter OWNS the null decision) |

**Note on the 4 different timeout semantics** (must be preserved by FR-R7):
- **planner**: timeout → `ErrPlannerFailed` (non-rescue, exit 1-ish). The planner has no rescue path (PRD §13.6.6). This is the role FR-R7 specifically wants to give a LONGER default (480s).
- **stager**: timeout → `ErrStagerFailed`.
- **message**: timeout → `RescueError{ErrTimeout}` (exit 124).
- **arbiter**: timeout → graceful **null** (NOT an error).

The model/provider/reasoning for each role are resolved via `config.ResolveRoleModel(role, deps.Config)` inside each role function (e.g. stager.go:91 `_, mdl, rsn := config.ResolveRoleModel("stager", deps.Config)`). **The timeout should be resolved the same way** — a new `config.ResolveRoleTimeout(role, cfg)` accessor.

### 3.1 The orchestrator's `Deps` struct

**`internal/decompose/roles.go:71-88`** — `Deps` carries `Config config.Config`, `Roles RoleManifests`, but NOT pre-resolved timeouts:
```go
type Deps struct {
    Git      git.Git
    Registry *provider.Registry
    Config   config.Config   // <- each role reads deps.Config.Timeout today
    Roles    RoleManifests
    Verbose  *ui.Verbose
    ...
}
```
After FR-R7, each role function should resolve its timeout from `deps.Config` (via the new accessor) instead of reading `deps.Config.Timeout` directly.

---

## 4. How multi-turn fallback uses timeout (FR-T5)

**`internal/generate/multiturn.go:165-187`** — `Run` calls `provider.Execute` once per turn (N+1 turns), each with **`cfg.Timeout`** (the message-role timeout, since multi-turn is a message-role fallback):
```go
if _, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose); execErr != nil {  // turn 1 (line 165)
    return "", false, execErr // FR-T7: any turn error/timeout/cancel/non-zero-exit aborts
}
...
if _, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose); execErr != nil {  // turns 2..N (line 176)
    return "", false, execErr
}
out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)  // turn N+1 final (line 187)
```
**FR-T5 (amended by FR-R7)**: per-turn timeout = the **message role's resolved timeout** (PRD §9.24 line 517). Total wall-clock budget = `message-timeout × (N+1)`.

The progress-line total-budget computation today (generate.go:423-426) uses the flat timeout:
```go
totalMin := int((cfg.Timeout * time.Duration(turns)).Minutes())
```
After FR-R7 this must use the **resolved message-role timeout**.

There is also a parallel work-description path (`internal/generate/workdesc.go:75,106,122`) that passes `cfg.Timeout` to `provider.Execute` three times — also a message-role path that should use the resolved message timeout.

---

## 5. The current role-resolution pattern (MIRROR THIS for timeout)

### 5.1 The `RoleConfig` struct

**`internal/config/config.go:36-40`**
```go
type RoleConfig struct {
    Provider  string `toml:"provider"`
    Model     string `toml:"model"`
    Reasoning string `toml:"reasoning"` // off|low|medium|high (FR-R6); "" ⇒ inherit global
}
```
FR-R7 must add a `Timeout` field here. ⚠️ **Decision needed**: type `time.Duration` (matches resolved `Config`) or `*time.Duration` (to distinguish unset from explicit-0). The `time.Duration` zero-value (0) is the natural "inherit global" sentinel (mirroring the `""` strings), and `Execute` already treats `timeout <= 0` as "no deadline" — so a `time.Duration` field with 0 = inherit is the cleanest mirror of the string fields. A non-zero explicit override survives overlay's non-zero-wins rule.

### 5.2 The `ResolveRoleModel` accessor (THE pattern to mirror)

**`internal/config/roles.go`** — full function. Per-field merge: per-role entry → fall back to global:
```go
func ResolveRoleModel(role string, cfg Config) (provider, model, reasoning string) {
    if rc, ok := cfg.Roles[role]; ok {
        if rc.Provider != ""  { provider = rc.Provider }
        if rc.Model != ""     { model = rc.Model }
        if rc.Reasoning != "" { reasoning = rc.Reasoning }
    }
    if provider == ""  { provider = cfg.Provider }
    if model == ""     { model = cfg.Model }
    if reasoning == "" { reasoning = cfg.Reasoning }
    return provider, model, reasoning
}
```
FR-R7 should add a sibling **`ResolveRoleTimeout(role string, cfg Config) time.Duration`** following the identical shape: read `cfg.Roles[role].Timeout`; if non-zero use it, else fall back to `cfg.Timeout`. (The planner's 480s *built-in* default is a separate concern — see §6.)

### 5.3 The `setRole*` helpers (lazily-alloc map + value-copy write-back)

**`internal/config/load.go:33-62`** — three identical-shape helpers. FR-R7 adds a 4th:
```go
func (c *Config) setRoleProvider(role, provider string) {
    if c.Roles == nil { c.Roles = make(map[string]RoleConfig) }
    rc := c.Roles[role]
    rc.Provider = provider
    c.Roles[role] = rc // write-back REQUIRED (map value-copy)
}
```
New `setRoleTimeout(role string, d time.Duration)` — same idiom. Setting one field must NOT clobber siblings (FR-R3 field-merge).

### 5.4 File decode twin + materialize conversion

**`internal/config/file.go:24-27`** — `fileRoleConfig` (file STRING twin of `RoleConfig`):
```go
type fileRoleConfig struct {
    Provider  string `toml:"provider"`
    Model     string `toml:"model"`
    Reasoning string `toml:"reasoning"`
}
```
FR-R7 adds `Timeout string \`toml:"timeout"\`` here.

⚠️ **BREAKING CONVERSION DETAIL**: materialize currently does a direct struct conversion **`file.go:316`**:
```go
c.Roles[role] = RoleConfig(frc)
```
This works ONLY because `fileRoleConfig` and `RoleConfig` have identical field types (all `string`). If `RoleConfig.Timeout` becomes `time.Duration` but `fileRoleConfig.Timeout` stays `string`, **`RoleConfig(frc)` will not compile** (type conversion requires identical underlying types). The materialize loop (file.go:311-317) must be rewritten to construct `RoleConfig` field-by-field and **parse the per-role timeout string** with `parseTimeout` (or `time.ParseDuration`) — mirroring how the global timeout is parsed in loadTOML. This is the single most error-prone change point.

### 5.5 File overlay (field-merge across config layers)

**`internal/config/file.go:443-458`** — the role FIELD-MERGE in `overlay()`:
```go
for role, rc := range src.Roles {
    existing := dst.Roles[role]
    if rc.Provider != ""  { existing.Provider = rc.Provider }
    if rc.Model != ""     { existing.Model = rc.Model }
    if rc.Reasoning != "" { existing.Reasoning = rc.Reasoning }
    dst.Roles[role] = existing
}
```
FR-R7 adds `if rc.Timeout != 0 { existing.Timeout = rc.Timeout }` (non-zero-wins, matching the scalar overlay discipline).

### 5.6 The decompose role-resolution orchestrator

**`internal/decompose/roles.go`** `ResolveRoles(cfg, reg)` — loops over `["planner","stager","message","arbiter"]`, calls `config.ResolveRoleModel(role, cfg)` per role, resolves manifests, and stores into `RoleManifests`/`RoleModels`. The resolved `(provider, model, reasoning)` triples are stored in `RoleModels` (`roles.go:60-65`). Timeouts are NOT pre-resolved here today — each role function re-derives its model via `config.ResolveRoleModel(role, deps.Config)` at call time. FR-R7 should resolve timeouts the same on-demand way (`config.ResolveRoleTimeout(role, deps.Config)`) at each Execute call site.

---

## 6. `role_defaults.go` — built-in role defaults

**`internal/config/role_defaults.go`** contains the **FR-D4 per-provider × per-role default-MODEL table** only:
```go
var roleDefaults = RoleModelDefaults{
    "pi": { "planner": "gpt-5.4", "stager": "gpt-5.4-mini", "message": "gpt-5.4-nano", "arbiter": "gpt-5.4-mini" },
    "claude": { "planner": "opus", ... },
    ...
}
```
**There are NO per-role timeout defaults here** — only models. FR-R7's "planner default 480s" is a role×timeout (NOT provider×role) default, so it does NOT belong in this provider-keyed table. The planner 480s default is best expressed in `Defaults()` / `ResolveRoleTimeout` as a role-specific built-in (the PRD §16.1 line 1635 phrases it as a role default, not a provider default). This is a distinct axis from the provider×role model table.

---

## 7. CLI flags

### 7.1 The global `--timeout` flag (STRING)

**`internal/cmd/root.go:165`** (in `init()`):
```go
pf.StringVar(&flagTimeout, "timeout", "",
    `Generation timeout, e.g. "120s" or 120 (env STAGECOACH_TIMEOUT, git stagecoach.timeout; default 480s)`)
```
Package var **`root.go:54`** `flagTimeout string // STRING — config.Load reads via fs.GetString("timeout")`. DefValue is `""` (zero default; precedence owned by `config.Load` via `fs.Changed`). Confirmed by test `root_test.go:386-391` (`TestRoot_TimeoutIsString`).

### 7.2 The per-role flag pattern (the template for `--<role>-timeout`)

Per-role provider/model flags (**`root.go:184-194`**):
```go
pf.StringVar(&flagPlannerProvider, "planner-provider", "", "...")
pf.StringVar(&flagPlannerModel,    "planner-model", "", "...")
pf.StringVar(&flagStagerProvider,  "stager-provider", "", "...")
pf.StringVar(&flagStagerModel,     "stager-model", "", "...")
pf.StringVar(&flagArbiterProvider, "arbiter-provider", "", "...")
pf.StringVar(&flagArbiterModel,    "arbiter-model", "", "...")
```
Per-role reasoning flags (**`root.go:239-252`**) — the most complete template (all 4 roles incl. message):
```go
pf.StringVar(&flagReasoning, "reasoning", "", "Global reasoning effort: ...")
pf.StringVar(&flagPlannerReasoning, "planner-reasoning", "", "...")
pf.StringVar(&flagStagerReasoning,  "stager-reasoning", "", "...")
pf.StringVar(&flagMessageProvider,  "message-provider", "", "...")   // (message provider/model at 245-249)
pf.StringVar(&flagMessageModel,     "message-model", "", "...")
pf.StringVar(&flagMessageReasoning, "message-reasoning", "", "...")
pf.StringVar(&flagArbiterReasoning, "arbiter-reasoning", "", "...")
```
⚠️ **FR-R7 must add 4 `--<role>-timeout` flags** (`planner-timeout`, `stager-timeout`, `message-timeout`, `arbiter-timeout`) + a global is already covered by `--timeout`. Each is a `StringVar` with `""` default (parsed via `parseTimeout` in `loadFlags`). The reasoning-flag block is the cleanest template (all 4 roles present).

### 7.3 Other timeout consumption in cmd

- **`internal/cmd/models.go:142-152`** — the `stagecoach models` command bounds the live `list_models_command` with `cfg.Timeout` (fallback 120s). This is the **global** timeout used as a subprocess bound — NOT a role timeout. Likely unaffected by FR-R7, but worth noting it reads `cfg.Timeout`.

---

## 8. Documentation (README + docs/)

- **README.md**: NO timeout documentation (grep "timeout" → no matches).
- **`docs/configuration.md`**:
  - line 86: `# timeout = "480s"` (commented example)
  - line 133: table row `| timeout | 480s | config.Defaults() |`
  - line 184: `| STAGECOACH_TIMEOUT | --timeout | Generation timeout | ...`
  - line 219: git example `timeout = 120s`
  - line 227: `| stagecoach.timeout | string | ... | Generation timeout (duration string) |`
- **`docs/cli.md`**:
  - line 27: `| --timeout <dur> | string | "480s" | STAGECOACH_TIMEOUT | stagecoach.timeout | ...`
  - line 405/407: exit 124 = timeout
  - line 421: precedence table `| --timeout | STAGECOACH_TIMEOUT | stagecoach.timeout |`

⚠️ **All docs cite the current 480s default**. FR-R7 changes the global default to 120s — these docs must be updated. The `--<role>-timeout` flags must be added to cli.md's per-role flag table.

### PRD references for FR-R7 (PRD.md)
- **Line 20** (v2.8 changelog): the full FR-R7 statement — global default 120s, planner default 480s, overrides `--<role>-timeout` / `STAGECOACH_<ROLE>_TIMEOUT` / `[role.<role>].timeout` / `stagecoach.role.<role>.timeout`.
- **Line 321** (FR25, amended): per-role timeout; global `--timeout`/`STAGECOACH_TIMEOUT`/`stagecoach.timeout`/`[defaults].timeout` (default 120s) is the fallback.
- **Line 517** (FR-T5, amended): per-turn timeout = message role's resolved timeout (FR-R7; default 120s).
- **Line 396-418** (§9.15): the per-role field resolution spec (FR-R1–R6) — timeout is the NEW 4th field per role.
- **Line 1635** (§16.1): built-in defaults — "timeout 120s (global fallback for every role; **planner role default 480s** — FR-R7)".
- **Line 1658** (§16.2): `[defaults] timeout = "120s"  # global fallback for every role (FR-R7); planner defaults to 480s`.
- **Line 1715+** (§16.4): per-role config example `[role.planner]` (currently only provider/model/reasoning).

---

## Files that will need changes for FR-R7 (impact map)

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `Timeout time.Duration` to `RoleConfig` (line ~40); **change `Defaults().Timeout` 480s→120s** (line 197); add planner 480s built-in (in `ResolveRoleTimeout` or a role-defaults map). |
| `internal/config/roles.go` | Add `ResolveRoleTimeout(role string, cfg Config) time.Duration` mirroring `ResolveRoleModel`. |
| `internal/config/load.go` | Add `setRoleTimeout` helper (~line 60); add `_TIMEOUT` branch in env loop (~line 295); add `-timeout` branch in flag loop (~line 440). |
| `internal/config/file.go` | Add `Timeout string` to `fileRoleConfig` (~line 27); **rewrite materialize role loop (line 311-317)** to construct `RoleConfig` field-by-field + parse per-role timeout (the `RoleConfig(frc)` conversion will break); add field-merge branch in overlay (~line 452); parse per-role timeout string in loadTOML. |
| `internal/config/git.go` | *(Optional/if PRD requires `stagecoach.role.<role>.timeout`)* add per-role git-config loading — note git layer has NO role loading today, even for provider/model/reasoning. |
| `internal/decompose/planner.go` | Replace `deps.Config.Timeout` (line 124) → `config.ResolveRoleTimeout("planner", deps.Config)`. |
| `internal/decompose/stager.go` | Replace `deps.Config.Timeout` (line 110) → resolved stager timeout. |
| `internal/decompose/message.go` | Replace `deps.Config.Timeout` (line 155) → resolved message timeout. |
| `internal/decompose/arbiter.go` | Replace `deps.Config.Timeout` (line 100) → resolved arbiter timeout. |
| `internal/generate/generate.go` | Resolve message-role timeout once (top of `CommitStaged`) and use it at line 335 + the multi-turn budget line 426. |
| `internal/generate/multiturn.go` | Lines 165/176/187: use resolved message-role timeout (passed in, not flat `cfg.Timeout`). |
| `internal/generate/workdesc.go` | Lines 75/106/122: message-role timeout. |
| `internal/cmd/root.go` | Add 4 `--<role>-timeout` StringVar flags (after the reasoning-flag block ~line 252). |
| `internal/config/bootstrap.go` | *(If bootstrap writes `[role.*]` blocks)* may need to emit per-role timeout comments. |
| `docs/configuration.md`, `docs/cli.md`, `PRD.md` examples | Update 480s→120s default; document `--<role>-timeout`. |

---

## Risks / Open Questions

1. **The `RoleConfig(frc)` conversion break (file.go:316)** — the highest-risk change. Adding a `time.Duration` field to `RoleConfig` while `fileRoleConfig.Timeout` stays `string` makes the direct conversion non-compilable. Must rewrite the materialize loop to build `RoleConfig` explicitly + parse each role's timeout string. Decide parse form: `time.ParseDuration` (file-layer-consistent, "120s" only) vs `parseTimeout` (env/flag-consistent, "120s" OR bare "120"). The global timeout is inconsistent here today.

2. **Default value change 480s→120s is a behavior change** for ALL existing users — every role (incl. single-commit message) currently gets 480s. After FR-R7, the message/stager/arbiter default drops to 120s. This is intentional per the PRD (motivated by a real planner deadline-exceeded) but is a notable migration impact. Planner keeps 480s via its role-specific built-in.

3. **Per-role git-config (`stagecoach.role.<role>.timeout`)** — the PRD v2.8 changelog names this key, but the git layer reads NO per-role keys today. Implementing it requires adding a per-role git-config reader (new to git.go), which would also retroactively enable `stagecoach.role.<role>.provider/model/reasoning` (currently advertised in flag help but NOT implemented). Decide: is git-config per-role in-scope for FR-R7, or does timeout mirror the *actually-implemented* (file/env/flag only) role-resolution?

4. **`ResolveRoleTimeout` zero-value sentinel**: `time.Duration` 0 = "inherit global" mirrors the `""` string fields. But `provider.Execute` treats `timeout <= 0` as "no deadline" — so a *resolved* timeout must NEVER be 0 (the global default 120s prevents this). The sentinel is only meaningful *before* resolution (inside `RoleConfig`/`ResolveRoleTimeout`), never at the Execute call site.

5. **Multi-turn total-budget display (generate.go:426)** must use the resolved message-role timeout, not the flat `cfg.Timeout`.

## Start Here
Open **`internal/config/roles.go`** (the `ResolveRoleModel` function) — it is the exact template for the new `ResolveRoleTimeout`, and reading it alongside **`internal/config/config.go:36-40`** (`RoleConfig`) frames the whole change. Then **`internal/provider/executor.go:44-52`** confirms `Execute` already takes a per-call duration (no executor change needed).
