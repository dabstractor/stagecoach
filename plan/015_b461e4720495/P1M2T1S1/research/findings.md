# Research: P1.M2.T1.S1 — Add ResolveRoleTimeout + defaultRoleTimeouts map (planner=480s)

Add a sibling resolution function `ResolveRoleTimeout(role string, cfg Config) time.Duration` next to
`ResolveRoleModel` in `internal/config/roles.go`, plus a `defaultRoleTimeouts` map (planner=480s). This
is the FR-R7 resolution surface that P1.M3 consumes at each Execute call site. It does NOT change the
global default (still 480s now; P1.M2.T2 changes it to 120s) and does NOT wire any consumer.

All claims verified against the current working tree (2026-07-10, post-P1.M1-landing).

---

## 0. STATE OF THE WORLD — P1.M1 has LANDED (consume, don't rebuild)

`ResolveRoleTimeout` does NOT exist yet (only a forward-reference comment at file.go:326). But its
INPUTS are all in place from the completed P1.M1 milestone:

| Input | Location | Status |
|-------|----------|--------|
| `RoleConfig.Timeout time.Duration` (0 ⇒ inherit global) | config.go:42 (+ doc @23-40) | LANDED (T1.S1) |
| `Config.Timeout time.Duration` (global, currently 480s) | config.go:71 / Defaults @197 | pre-existing |
| `fileRoleConfig.Timeout string` + materialize parse + overlay `!= 0` merge | file.go:28, 311-326, 467-496 | LANDED (T1.S1/S2) |
| `setRoleTimeout(role, d)` helper + env `_TIMEOUT` branch | load.go:66-78, 319-322 | LANDED (T2.S1) |
| 4 `--<role>-timeout` CLI flags + flag-loop branch | root.go / load.go flag loop | LANDED (T2.S2) |
| per-role git-config `stagecoach.role.<role>.timeout` | git.go (loop after global timeout block) | IN PROGRESS (T2.S3, parallel — assume LANDED) |

So by the time this task runs, `cfg.Roles[role].Timeout` is fully populated from all 5 precedence
layers (file/env/flag/git). `ResolveRoleTimeout` is the pure read-side accessor over that already-
resolved table — exactly as `ResolveRoleModel` is.

---

## 1. THE TEMPLATE — `ResolveRoleModel` in `internal/config/roles.go`

`ResolveRoleModel` is the verbatim structural template. Full function (current source):

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

It has a LARGE godoc comment (the whole block above the func) citing §16.4/§9.15 FR-R1–R3/R6 and
explaining the per-field merge + the ("","","") manifest sentinel. `ResolveRoleTimeout` gets an
equivalent godoc citing FR-R7 + the 3-tier precedence.

**CRITICAL — roles.go has NO import block today.** The file is `package config` then directly the
godoc + `ResolveRoleModel`. `ResolveRoleTimeout` references `time.Duration`, so you MUST add:
```go
import "time"
```
(single import; place it between `package config` and the first comment, per gofmt.)

---

## 2. THE FUNCTION — `ResolveRoleTimeout` (the deliverable)

Precedence (highest → lowest), mirroring ResolveRoleModel's per-field shape but with a 3rd (built-in)
tier that ResolveRoleModel does NOT have:

1. **per-role override** — `cfg.Roles[role].Timeout` (non-zero) — the `[role.<role>].timeout` /
   `--<role>-timeout` / `STAGECOACH_<ROLE>_TIMEOUT` / `stagecoach.role.<role>.timeout` value (all
   already merged into `cfg.Roles` by the loaders).
2. **built-in role default** — `defaultRoleTimeouts[role]` (planner=480s; stager/message/arbiter absent).
3. **global** — `cfg.Timeout` (the `[defaults].timeout` / `--timeout` / `STAGECOACH_TIMEOUT` /
   `stagecoach.timeout` value; currently 480s, becomes 120s in P1.M2.T2).

```go
// ResolveRoleTimeout returns the generation timeout for a single agent role (PRD §9.15 FR-R7, §16.1),
// applying the precedence:
//
//	[role.<role>].timeout  (CLI flag > env > file > git, all merged into cfg.Roles by the loaders)
//	> built-in role default  (planner = 480s; FR-R7 — the planner needs more time than message/stager/arbiter)
//	> [defaults].timeout     (cfg.Timeout — the global; 480s today, 120s after P1.M2.T2)
//
// By the time this runs, Load() has already overlaid every precedence layer into cfg.Roles[role].Timeout,
// so this only checks the per-role entry, then the built-in role default, then the global. It does NOT
// re-walk the layers.
//
// The planner is the ONLY role with a shipped built-in timeout (480s): it does the open-ended decomposition
// planning and most often needs longer than the message/stager/arbiter roles (which inherit cfg.Timeout).
// A non-zero cfg.Roles[role].Timeout ALWAYS wins — even for the planner (a user's --planner-timeout 600s
// beats the 480s built-in). A role absent from cfg.Roles (or with Timeout==0) inherits: planner → 480s
// built-in; stager/message/arbiter → cfg.Timeout.
//
// The zero-value sentinel (RoleConfig.Timeout == 0 ⇒ "inherit") mirrors the "" string fields of
// ResolveRoleModel. A RESOLVED timeout is NEVER 0: even if cfg.Timeout were 0, the planner's 480s built-in
// covers the planner; the consumers (P1.M3) additionally guard against 0 at the Execute site (Execute
// treats timeout<=0 as "no deadline"). ResolveRoleTimeout itself may return cfg.Timeout unchanged for
// non-planner roles (do not collapse 0 → the built-in here; the built-in is role-specific).
//
// role is an arbitrary string; a non-canonical name misses the cfg.Roles lookup AND the built-in map,
// so it inherits cfg.Timeout (no error) — same leniency as ResolveRoleModel.
func ResolveRoleTimeout(role string, cfg Config) time.Duration {
	if rc, ok := cfg.Roles[role]; ok && rc.Timeout != 0 {
		return rc.Timeout
	}
	if d, ok := defaultRoleTimeouts[role]; ok {
		return d
	}
	return cfg.Timeout
}
```

Note the structural divergence from ResolveRoleModel: model/provider/reasoning fall back to ONE global
tier; timeout falls back to TWO tiers (built-in then global). This is intentional and is THE semantic
the tests must lock in (see §5).

---

## 3. THE MAP — `defaultRoleTimeouts` (placement decision)

Item says "roles.go OR role_defaults.go". Architecture note `research_timeout.md` §6 is explicit:
the planner 480s is a **role×timeout** default (NOT provider×role), so it does NOT belong in
`role_defaults.go`'s provider-keyed FR-D4 model table. Place it in **roles.go**, co-located with its
sole reader (`ResolveRoleTimeout`):

```go
// defaultRoleTimeouts are the shipped per-role generation-timeout built-ins (PRD §16.1, FR-R7). The
// planner is the ONLY role with a built-in timeout (480s) — it does open-ended decomposition planning
// and needs longer than the message/stager/arbiter roles, which inherit cfg.Timeout. A user override
// (cfg.Roles[role].Timeout, set via [role.<role>].timeout / --<role>-timeout / env / git-config) ALWAYS
// beats this. This is a role×timeout axis, distinct from role_defaults.go's provider×role→MODEL table
// (FR-D4) — do not conflate the two. Add a role here only when shipping a non-global default for it.
var defaultRoleTimeouts = map[string]time.Duration{
	"planner": 480 * time.Second,
}
```

(Rejected alternative: role_defaults.go. It is thematically the FR-D4 provider×role model table; mixing
a role×timeout map there obscures both axes. roles.go keeps the map next to ResolveRoleTimeout.)

---

## 4. THE GLOBAL DEFAULT — DO NOT CHANGE (it is P1.M2.T2.S1's job)

`internal/config/config.go:197` (Defaults()): `Timeout: 480 * time.Second,`. This stays 480s for THIS
task. P1.M2.T2.S1 changes it to `120 * time.Second` and fixes the pinning tests. Do NOT touch it here.

Consequence for tests: `Defaults().Timeout` is currently 480s. To prove the planner built-in (480s)
is returned from the MAP and not from cfg.Timeout, tests must set `cfg.Timeout` to a DISTINCT value
(e.g. 120s) so the assertion is unambiguous. See §5.

---

## 5. TESTS — mirror roles_test.go's ResolveRoleModel suite (12 existing funcs)

roles_test.go uses the idiom: `cfg := Defaults()` → mutate `cfg.Provider/Model/Timeout` and/or set
`cfg.Roles = map[string]RoleConfig{...}` → call `ResolveRoleX(role, cfg)` → assert. Tests are
plain `func TestX(t *testing.T){}` (no table-driven, no helpers, no subtests in this file).

ResolveRoleTimeout tests to add (clone the matching ResolveRoleModel test shape):

| New test | Mirrors | What it locks |
|----------|---------|---------------|
| `TestResolveRoleTimeout_PerRoleOverride` | FullOverride (@32) | `cfg.Roles["planner"].Timeout=600s` → 600s (per-role beats built-in 480s AND global) |
| `TestResolveRoleTimeout_PlannerBuiltinBeatsGlobal` | (new — THE key test) | Roles nil/absent, `cfg.Timeout=120s` → planner returns **480s** (built-in beats global); proves the 3rd tier |
| `TestResolveRoleTimeout_NonPlannerGlobalFallback` | GlobalFallback (@5/18) | `cfg.Timeout=120s`, message/stager/arbiter absent → each returns **120s** (no built-in → global) |
| `TestResolveRoleTimeout_FieldMergeTimeoutOnly` | ModelOnlyOverride (@47) | `cfg.Roles["message"]={Provider:"pi", Timeout:0}` + `cfg.Timeout=120s` → 120s (Timeout 0 ⇒ inherit; Provider stays set) |
| `TestResolveRoleTimeout_UnknownRoleGlobalFallback` | UnknownRole (@89) | `cfg.Timeout=120s`, role="palnner" → 120s (unknown role: no built-in, no entry → global) |
| `TestResolveRoleTimeout_RolesNilGlobalFallback` | GlobalFallbackRolesNil (@5) | `cfg.Roles=nil`, `cfg.Timeout=120s`, message → 120s |

**THE critical disambiguation**: every test that checks the planner's built-in or a non-planner's
global fallback MUST set `cfg.Timeout` to a value ≠ 480s (use `120 * time.Second`) so the test fails
loudly if someone inverts the precedence (built-in vs global). Using `Defaults()` directly (Timeout
== 480s) would make planner-built-in-vs-global ambiguous. Example:

```go
func TestResolveRoleTimeout_PlannerBuiltinBeatsGlobal(t *testing.T) {
	cfg := Defaults()
	cfg.Timeout = 120 * time.Second // DISTINCT from the 480s built-in — makes the assertion unambiguous
	// Roles nil ⇒ no per-role override; planner must take its 480s BUILT-IN, NOT the 120s global.
	got := ResolveRoleTimeout("planner", cfg)
	if got != 480*time.Second {
		t.Errorf("ResolveRoleTimeout(planner) = %v, want 480s (built-in beats 120s global)", got)
	}
}
```

(`time` import already in roles_test.go? — check: the file currently imports only `"testing"`. Adding
`time.Duration` assertions requires `import ("testing"; "time")`. Verify and add `time` if absent.)

---

## 6. Scope boundaries (what NOT to do)

- **NO consumer wiring.** Do NOT touch the 13 Execute call sites (planner.go/stager.go/message.go/
  arbiter.go/generate.go/multiturn.go/workdesc.go). That is P1.M3. After this task `ResolveRoleTimeout`
  has ZERO production callers (grep guard: only roles.go def + roles_test.go calls).
- **NO global-default change.** Defaults().Timeout stays 480s. P1.M2.T2.S1 owns the 480s→120s flip.
- **NO new config-loading layer.** RoleConfig.Timeout / fileRoleConfig / materialize / overlay /
  setRoleTimeout / env / flag / git are all DONE (P1.M1). This task is pure read-side resolution.
- **NO docs change.** Internal resolution function, no user-facing surface. docs are P1.M4.T2.
- **NO `*time.Duration`.** RoleConfig.Timeout is already a plain `time.Duration` (0 ⇒ inherit) — mirror
  it; do not promote to a pointer (that would be cross-cutting and is already decided against in T1.S1).
- **NO entry in role_defaults.go.** The planner 480s is role×timeout, not provider×role (see §3).

---

## 7. Parallel-execution coordination

Parallel sibling P1.M1.T2.S3 edits `internal/config/git.go` (adds the per-role git-config loop). It
does NOT touch roles.go or roles_test.go → no file overlap. My task touches ONLY roles.go + roles_test.go.
Assume T2.S3's `stagecoach.role.<role>.timeout` reading has LANDED (it populates cfg.Roles[role].Timeout,
which my function reads). No merge conflict regardless of order.

---

## 8. Validation commands (Makefile)

- Build: `go build ./...` (and `GOOS=windows/linux go build ./...` — plain function, cross-safe)
- Vet: `go vet ./internal/config/...`
- Format: `gofmt -l internal/config/roles.go internal/config/roles_test.go` (must be empty)
- Focused: `go test ./internal/config/ -run 'ResolveRoleTimeout' -v`
- Regression: `go test ./internal/config/ -run 'ResolveRoleModel|Role' -v` (the 12 model tests stay green)
- Full suite (race): `make test`
- Lint: `make lint` (golangci-lint v1.61: staticcheck/gosimple/govet/errcheck/ineffassign/unused —
  `unused` would fire if ResolveRoleTimeout had zero readers, but the new tests read it → clean)
- Coverage gate: `make coverage-gate` (≥85% on internal/{git,provider,generate,config})
