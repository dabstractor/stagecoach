# Research: Role Resolution & Config CLI Flag Patterns (for FR-R7 per-role timeout)

Codebase: `/home/dustin/projects/stagecoach`. Goal: capture the EXACT patterns to follow when
adding **per-role generation timeouts** (PRD §9.15 FR-R7; v2.8). This is reference research, not an
implementation. Every path/line below was read from source.

> ⚠️ NOTE on task ↔ actual file mapping: the task brief swapped the roles of two files. The ACTUAL
> locations are:
> - `RoleConfig` struct → `internal/config/config.go:36` (NOT roles.go)
> - `ResolveRoleModel` function → `internal/config/roles.go:34`
> - `roleDefaults` (built-in default models) table → `internal/config/role_defaults.go`

---

## 1. The per-role config struct + TOML parsing

**File: `internal/config/config.go:36-43`** — `RoleConfig` (the RESOLVED typed struct):
```go
type RoleConfig struct {
	Provider  string `toml:"provider"`
	Model     string `toml:"model"`
	Reasoning string `toml:"reasoning"` // off|low|medium|high (FR-R6); "" ⇒ inherit global
}
```
All three fields are **strings**; `""` is the "inherit global" sentinel. There is **no `Timeout`
field** today — FR-R7 adds it. Because a `time.Duration` zero value is `0`, the existing "non-zero
overlay" + "string `""` ⇒ inherit" idioms both need a new discipline for timeout (duration-zero
`0` is the natural "inherit" sentinel, mirroring how `Config.Timeout` works).

**File: `internal/config/config.go:63`** — `Config` struct carries the RESOLVED per-role table at
**`config.go:170`**:
```go
Roles map[string]RoleConfig `toml:"-"`
```
`toml:"-"` — never decoded directly; populated by file/env/flag loaders.

**File: `internal/config/file.go:23-28`** — `fileRoleConfig` (the FILE decode twin; string-shaped):
```go
type fileRoleConfig struct {
	Provider  string `toml:"provider"`
	Model     string `toml:"model"`
	Reasoning string `toml:"reasoning"`
}
```
This is the decode target for `[role.planner]` / `[role.stager]` / `[role.message]` /
`[role.arbiter]` tables. The containing `fileConfig` declares it at `file.go:32`:
```go
Role map[string]fileRoleConfig `toml:"role"`
```

**Materialize (file → Config): `internal/config/file.go:313-317`**:
```go
if len(fc.Role) > 0 {
	c.Roles = make(map[string]RoleConfig, len(fc.Role))
	for role, frc := range fc.Role {
		c.Roles[role] = RoleConfig(frc)
	}
}
```
NOTE: this is a direct struct conversion (`RoleConfig(frc)`). Adding a `Timeout time.Duration` to
`RoleConfig` while keeping `fileRoleConfig.Timeout string` will **break this conversion** — the
materialize loop will need explicit field-by-field copy WITH duration parsing (mirror how
`Timeout`/`HookTimeout` are parsed in `loadTOML` at `file.go:131-145` via `time.ParseDuration`).

**Overlay (cross-layer field-merge): `internal/config/file.go:440-457`** — per-role field-merge:
```go
if len(src.Roles) > 0 {
	if dst.Roles == nil {
		dst.Roles = make(map[string]RoleConfig, len(src.Roles))
	}
	for role, rc := range src.Roles {
		existing := dst.Roles[role]
		if rc.Provider != "" {
			existing.Provider = rc.Provider
		}
		if rc.Model != "" {
			existing.Model = rc.Model
		}
		if rc.Reasoning != "" {
			existing.Reasoning = rc.Reasoning
		}
		dst.Roles[role] = existing
	}
}
```
FR-R7 adds a `Timeout` branch here: `if rc.Timeout != 0 { existing.Timeout = rc.Timeout }`
(duration-zero = inherit, mirrors the `Config.Timeout` overlay guard at `file.go:399`).

---

## 2. Built-in role defaults (models) — `internal/config/role_defaults.go`

This file holds the FR-D4 per-provider×per-role **model** default table (NOT timeouts). Key pieces:

- `DefaultModelsVerificationDate = "2026-07-02"` (`role_defaults.go:9`)
- `roleDefaults` var (`role_defaults.go:54`) — `map[string]map[string]string` keyed provider→role→model.
  Example `pi`: planner=`gpt-5.4`, stager=`gpt-5.4-mini`, message=`gpt-5.4-nano`, arbiter=`gpt-5.4-mini`.
- `DefaultModelsForProvider(name)` (`role_defaults.go:97`) — returns a defensive COPY of one
  provider's role→model column, or nil if unknown.

There is **no built-in role-timeout table here today**. Per PRD §16.1, FR-R7 introduces role-default
timeouts: global `120s`, **planner `480s`**, others inherit the global. The natural home for a
`defaultRoleTimeouts` map (keyed role→duration) is this file or `config.go` `Defaults()`. NOTE:
`Defaults()` (`config.go:192`) currently hardcodes the GLOBAL timeout at **`480 * time.Second`**
(`config.go:197`) — FR-R7 changes that to **`120 * time.Second`** (PRD §16.1: "timeout 120s (global
fallback for every role; planner role default 480s)").

**`ResolveRoleModel` — `internal/config/roles.go:34-64`** (the role-resolution function):
```go
func ResolveRoleModel(role string, cfg Config) (provider, model, reasoning string) {
	if rc, ok := cfg.Roles[role]; ok {
		if rc.Provider != "" {
			provider = rc.Provider
		}
		if rc.Model != "" {
			model = rc.Model
		}
		if rc.Reasoning != "" {
			reasoning = rc.Reasoning
		}
	}
	if provider == "" {
		provider = cfg.Provider
	}
	if model == "" {
		model = cfg.Model
	}
	if reasoning == "" {
		reasoning = cfg.Reasoning // FR-R6: off (=="") fallback
	}
	return provider, model, reasoning
}
```
This returns 3 values; FR-R7 needs a 4th (timeout). Two options for the implementer:
1. **Change the signature** to return `(provider, model, reasoning string, timeout time.Duration)`
   and update both call sites (`decompose/roles.go` ResolveRoles + `generate/generate.go`).
2. **Add a sibling `ResolveRoleTimeout(role, cfg) time.Duration`** that applies the same
   per-role→global→built-in-default-precedence (and the special planner-480s default), and update
   the 13 `provider.Execute` call sites to call it.

`ResolveRoleModel` deliberately does NOT touch manifests (no `internal/provider` import) — the same
separation should hold for timeout resolution.

---

## 3. Decompose role resolution — `internal/decompose/roles.go`

**`ResolveRoles(cfg, reg)` — `roles.go:70-…`** loops over the 4 canonical roles and for each:
1. `config.ResolveRoleModel(role, cfg)` → (provider, model, reasoning) — `roles.go:80`
2. auto-detect provider via `reg.DefaultProvider` if `""` — `roles.go:82`
3. `reg.Get → Validate → IsInstalled` — `roles.go:88-97`
4. FR-D4 stager fallback (TooledFlags-less → capable) — `roles.go:100-128`
5. FR-R5b bare-model guard — `roles.go:130-136`
6. `setRole(...)` stores manifest + `RoleConfig{Provider, Model, Reasoning}` — `roles.go:200-211`

**`setRole` — `roles.go:200-211`** builds the `RoleModels` typed fields:
```go
func setRole(rm *RoleManifests, rmodels *RoleModels, role string, m provider.Manifest, prov, mdl, rsn string) {
	rc := config.RoleConfig{Provider: prov, Model: mdl, Reasoning: rsn}
	switch role {
	case "planner":
		rm.Planner, rmodels.Planner = m, rc
	case "stager":
		rm.Stager, rmodels.Stager = m, rc
	case "message":
		rm.Message, rmodels.Message = m, rc
	case "arbiter":
		rm.Arbiter, rmodels.Arbiter = m, rc
	}
}
```
`RoleModels` (`roles.go:46-52`) holds four `config.RoleConfig` fields (Planner/Stager/Message/Arbiter).
FR-R7 plumbing: `setRole` + `RoleModels` need the timeout; each decompose agent (planner/stager/
message/arbiter) currently passes **`deps.Config.Timeout`** (the global) to `provider.Execute`
(see §7) — those 4 sites must switch to the per-role resolved timeout.

---

## 4. CLI flag registration — `internal/cmd/root.go`

**Per-role flag vars — `root.go:78-97`** (package-level `var` block):
```go
var (
	flagPlannerProvider  string
	flagPlannerModel     string
	flagPlannerReasoning string
	flagStagerProvider   string
	flagStagerModel      string
	flagStagerReasoning  string
	flagMessageProvider  string
	flagMessageModel     string
	flagMessageReasoning string
	flagArbiterProvider  string
	flagArbiterModel     string
	flagArbiterReasoning string
)
```
There are NO `flagXxxTimeout` vars — FR-R7 adds 4 (`flagPlannerTimeout`, etc.).

**Flag registration — `root.go` `init()` (`root.go:127-…`)** uses `pf.StringVar(...)` per flag. The
per-role registrations are **individual** (NOT a loop) for provider/model/reasoning, e.g.
(`root.go:159-178`):
```go
pf.StringVar(&flagPlannerProvider, "planner-provider", "", "Per-role provider override for the decomposition planner (env STAGECOACH_PLANNER_PROVIDER; git stagecoach.role.planner)")
pf.StringVar(&flagPlannerModel, "planner-model", "", "Per-role model override for the decomposition planner (env STAGECOACH_PLANNER_MODEL; git stagecoach.role.planner)")
...
pf.StringVar(&flagStagerProvider, "stager-provider", "", "...")
pf.StringVar(&flagStagerModel, "stager-model", "", "...")
pf.StringVar(&flagArbiterProvider, "arbiter-provider", "", "...")
pf.StringVar(&flagArbiterModel, "arbiter-model", "", "...")
```
and the reasoning + message set (`root.go:235-259`):
```go
pf.StringVar(&flagReasoning, "reasoning", "", "Global reasoning effort: off|low|medium|high ...")
pf.StringVar(&flagPlannerReasoning, "planner-reasoning", "", "...")
pf.StringVar(&flagStagerReasoning, "stager-reasoning", "", "...")
pf.StringVar(&flagMessageProvider, "message-provider", "", "...")
pf.StringVar(&flagMessageModel, "message-model", "", "...")
pf.StringVar(&flagMessageReasoning, "message-reasoning", "", "...")
pf.StringVar(&flagArbiterReasoning, "arbiter-reasoning", "", "...")
```

**Global timeout flag — `root.go:71`** (the `flagTimeout` var) + `root.go:133`:
```go
var (
	...
	flagTimeout  string // STRING — config.Load reads via fs.GetString("timeout") (FINDING 7)
	...
)
// in init():
pf.StringVar(&flagTimeout, "timeout", "", "Generation timeout, e.g. \"120s\" or 120 (env STAGECOACH_TIMEOUT, git stagecoach.timeout; default 480s)")
```
NOTE: `--timeout` is registered as a **STRING** flag (zero default `""`), and `config.Load` parses
it via `parseTimeout` in `loadFlags`. FR-R7's `--<role>-timeout` flags should follow the SAME
string-flag + `parseTimeout` pattern (consistent error messages). The help text says "default 480s"
today — FR-R7 changes that to 120s.

**flag reading via fs.Changed** — `load.go` `loadFlags` (§5).

---

## 5. Env-var parsing — `internal/config/load.go`

**`roleNames` — `load.go:17`**: `[]string{"planner", "stager", "message", "arbiter"}` — the single
source looped for per-role env/flag reading.

**Global `STAGECOACH_TIMEOUT` — `load.go:260-266`** (inside `loadEnv`):
```go
if v, ok := os.LookupEnv("STAGECOACH_TIMEOUT"); ok && v != "" {
	d, err := parseTimeout(v)
	if err != nil {
		return fmt.Errorf("STAGECOACH_TIMEOUT: %w", err)
	}
	cfg.Timeout = d
}
```

**Per-role env loop — `load.go:293-305`** (inside `loadEnv`):
```go
for _, role := range roleNames {
	prefix := "STAGECOACH_" + strings.ToUpper(role)
	if v, ok := os.LookupEnv(prefix + "_PROVIDER"); ok && v != "" {
		cfg.setRoleProvider(role, v)
	}
	if v, ok := os.LookupEnv(prefix + "_MODEL"); ok && v != "" {
		cfg.setRoleModel(role, v)
	}
	if v, ok := os.LookupEnv(prefix + "_REASONING"); ok && v != "" {
		cfg.setRoleReasoning(role, v)
	}
}
```
FR-R7 adds: `if v, ok := os.LookupEnv(prefix + "_TIMEOUT"); ok && v != "" { d, err := parseTimeout(v); ... cfg.setRoleTimeout(role, d) }`.

**Per-role setter helpers — `load.go:33-69`** (map-value-copy write-back idiom — REQUIRED for Go
maps):
```go
func (c *Config) setRoleProvider(role, provider string) {
	if c.Roles == nil { c.Roles = make(map[string]RoleConfig) }
	rc := c.Roles[role]
	rc.Provider = provider
	c.Roles[role] = rc
}
func (c *Config) setRoleModel(role, model string) { /* same idiom, sets rc.Model */ }
func (c *Config) setRoleReasoning(role, reasoning string) { /* same idiom, sets rc.Reasoning */ }
```
FR-R7 adds `setRoleTimeout(role string, d time.Duration)` following the identical idiom.

**`parseTimeout(s)` — `load.go:616-628`** — accepts BOTH `"120s"`/`"2m"` (Go duration) AND bare
integer `"120"` (seconds). Reuse this for the per-role timeout everywhere (env, flag, git-config,
file).

---

## 6. Git-config reading — `internal/config/git.go`

**`loadGitConfig(repoDir)` — `git.go:113-…`** reads per-repo `stagecoach.*` keys via
`gitConfigGet`/`gitConfigBool`. It returns a PARTIAL `*Config` (only found keys set).

**Global `stagecoach.timeout` — `git.go:148-157`**:
```go
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

⚠️ **CRITICAL: there is NO `stagecoach.role.<role>.*` git-config reading anywhere.** The current
per-role config (provider/model/reasoning) flows ONLY through TOML file → env → flags; git-config
has no per-role support. `grep "role\." internal/config/git.go` returns nothing. So FR-R7's
`stagecoach.role.<role>.timeout` is **NEW infrastructure** — the implementer must add a per-role
loop over `roleNames` in `loadGitConfig` reading `stagecoach.role.<role>.timeout` (and, per PRD,
optionally provider/model/reasoning too). The key shape would be e.g.
`stagecoach.role.planner.timeout` (git rejects underscores in keys — camelCase not needed for the
single word "timeout"; but the dotted `role.planner` segment is valid git-config).

---

## 7. Single-commit path timeout (the `message` role) — `internal/generate/generate.go`

**Message role resolution — `generate.go:287-289`** (inside `CommitStaged`, before the loop):
```go
// FR-R3: resolve the message role so --message-model / [role.message] drive Render.
_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)
```
The message role's **provider** is discarded here (the manifest is `deps.Manifest`, selected
upstream by `buildDeps`).

**The global timeout passed to Execute — `generate.go:335`**:
```go
out, _, execErr := provider.Execute(ctx, *spec, cfg.Timeout, deps.Verbose)
```
FR-R7 changes this to the **message role's resolved timeout** (e.g.
`config.ResolveRoleTimeout("message", cfg)` or a 4th return from `ResolveRoleModel`). The multi-turn
total-budget line (`generate.go:386-391`) computes `cfg.Timeout * turns` today — it must use the
message role's timeout (`message-timeout × (N+1)`, PRD FR-T5).

**Sibling single-commit call sites that ALSO pass `cfg.Timeout` (all are the `message` role)**:
- `internal/generate/multiturn.go:165, 176, 187` (multi-turn turns)
- `internal/generate/workdesc.go:75, 106, 122` (work-description mode)
- `internal/hook/exec.go:182` (hook mode — §9.20; resolves the `message` role)

All of these are the `message` role and must resolve its per-role timeout.

---

## 8. How the executor applies the timeout — `internal/provider/executor.go`

**`Execute(ctx, spec, timeout, vb)` — `executor.go:46-110`**. Signature:
```go
func Execute(ctx context.Context, spec CmdSpec, timeout time.Duration, vb *ui.Verbose) (stdout string, stderr string, err error)
```
Timeout application — **`executor.go:48-52`**:
```go
if timeout > 0 {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, timeout) // SHADOW — see doc; do not rename
	defer cancel()
}
```
`timeout ≤ 0 ⇒ no timeout` (the parent ctx still applies). On timeout `err IS context.DeadlineExceeded`
(`executor.go:90`). **`Execute` already takes a per-call `timeout` parameter** — so FR-R7 needs NO
change to `Execute` itself; it just needs each caller to pass the per-role resolved timeout instead
of the global `cfg.Timeout`/`deps.Config.Timeout`. This is the single chokepoint — change the value
passed in at each of the 13 call sites.

---

## All 13 `provider.Execute` timeout call sites (the change surface)

Single-commit / `message` role:
- `internal/generate/generate.go:335` — main generate loop
- `internal/generate/multiturn.go:165, 176, 187` — multi-turn turns
- `internal/generate/workdesc.go:75, 106, 122` — work-description mode
- `internal/hook/exec.go:182` — hook mode

Decompose / per-role:
- `internal/decompose/planner.go:124` — planner (`deps.Config.Timeout`)
- `internal/decompose/stager.go:110` — stager (`deps.Config.Timeout`)
- `internal/decompose/message.go:155` — message (`deps.Config.Timeout`)
- `internal/decompose/arbiter.go:100` — arbiter (`deps.Config.Timeout`)

The decompose package resolves models via `decompose.ResolveRoles` → `RoleModels`; the cleanest
plumbing is to carry a resolved timeout alongside (add a `Timeout time.Duration` to `RoleConfig`, so
`RoleModels.X.Timeout` is read at each call site). The single-commit callers should call
`config.ResolveRoleTimeout("message", cfg)` (or the 4th return of `ResolveRoleModel`).

---

## ⚠️ DEFAULT CHANGE (breaks existing tests) — global timeout 480s → 120s

PRD §16.1 (FR-R7) changes the global `[defaults].timeout` default from the current **480s** to
**120s**, and adds a **planner role default of 480s**. These spots pin 480s as the global default
and WILL need updating:
- `internal/config/config.go:197` — `Timeout: 480 * time.Second` → `120 * time.Second`
- `internal/config/config.go:68` — comment "Defaults: 480s"
- `internal/config/config.go:180` — doc comment "timeout 480s"
- `internal/config/bootstrap.go:161` — commented template `# timeout = "480s"` → `"120s"`
- `internal/cmd/root.go:133` — `--timeout` help text "default 480s" → "default 120s"
- `internal/config/config_test.go:20-21` — `if c.Timeout != 480*time.Second` (FAILS after change)
- `internal/config/file_test.go:113, 787` — pins `Timeout=480s` default baseline
- `internal/config/load_test.go:589-590` — `if cfg.Timeout != 480*time.Second` (FAILS after change)

The **planner 480s role default** is a NEW layer between the global default and per-role config —
implement as a `defaultRoleTimeouts` map (e.g. `{"planner": 480s}`) consulted in
`ResolveRoleTimeout` when the per-role + global are both unset/zero. Stager/message/arbiter inherit
the global 120s (absent from the default map).

---

## Precedence summary (FR-R7, per PRD §16.1/§9.15 FR-R3), highest wins, per field

```
CLI flag --<role>-timeout
  > env STAGECOACH_<ROLE>_TIMEOUT
  > [role.<role>].timeout (TOML file)
  > stagecoach.role.<role>.timeout (git-config — NEW)
  > [defaults].timeout / --timeout / STAGECOACH_TIMEOUT / stagecoach.timeout (global, default 120s)
  > built-in role default (planner=480s) — NEW
```
NOTE: the existing provider/model/reasoning resolution does NOT have the git-config per-role layer
(only TOML/env/flag); FR-R7's git-config layer is new and should probably be added uniformly for all
four role fields for consistency, but the minimum for FR-R7 is the timeout key.

---

## Architecture / how the pieces connect

```
cmd/root.go (registers --<role>-* flags, --timeout)
    │  (PersistentPreRunE → config.Load with cmd.Flags())
    ▼
config/load.go Load()
    ├─ Defaults()           [config.go:192] — Layer 1 (Timeout 120s after FR-R7)
    ├─ loadTOML             [file.go] — [role.<role>] → fileRoleConfig → materialize → overlay (field-merge)
    ├─ loadGitConfig        [git.go:113] — stagecoach.timeout (global); stagecoach.role.<role>.* is NEW
    ├─ loadEnv              [load.go:243] — STAGECOACH_*; per-role loop uses setRoleProvider/Model/Reasoning
    └─ loadFlags            [load.go:394] — fs.Changed gating; per-role loop reads --<role>-*
    │
    ▼  resolved *Config (Roles map[string]RoleConfig)
config.ResolveRoleModel(role, cfg)   [roles.go:34] — per-field role → global fallback
    │
    ▼
decompose.ResolveRoles [decompose/roles.go:70] — 4 roles → RoleManifests + RoleModels
    └─ planner.go / stager.go / message.go / arbiter.go — each calls
       provider.Execute(ctx, spec, deps.Config.Timeout, …)  ← must become per-role timeout
generate.CommitStaged [generate.go] — message role; provider.Execute(..., cfg.Timeout, …)
provider.Execute [executor.go:46] — applies timeout via context.WithTimeout; NO signature change needed
```

---

## Start Here

**`internal/config/config.go:36`** — add the `Timeout time.Duration` field to `RoleConfig`. Then
follow the chain: `fileRoleConfig` (`file.go:23`) + materialize parse (`file.go:313`), overlay
field-merge (`file.go:440`), `setRoleTimeout` + env loop (`load.go`), flag vars + registration
(`cmd/root.go`), `ResolveRoleModel`/`ResolveRoleTimeout` (`roles.go`), then the 13
`provider.Execute` call sites. Change the global default 480s→120s last (`config.go:197`) and fix
the 4 pinning tests. The executor itself needs no change.

## Supervisor coordination
None needed — research-only task; no decisions required. Returning findings normally.
