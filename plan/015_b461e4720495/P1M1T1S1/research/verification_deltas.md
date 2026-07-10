# Research Notes — P1.M1.T1.S1 (Add Timeout to RoleConfig + fileRoleConfig + rewrite materialize loop)

Verification of the task-description claims against the CURRENT working tree (2026-07-10). The
architecture docs (`critical_findings.md` Finding 1/9, `research_role_config.md` §1,
`research_timeout.md`) are accurate. These notes record the KEY design decision + extra sites.

## KEY DESIGN DECISION — materialize MUST gain an `error` return (task-driven)

The task (3c) explicitly says: parse `frc.Timeout` INSIDE the materialize loop and wrap parse errors
with the ROLE name: `fmt.Errorf("parse config: [role.%s].timeout: %w", role, err)`. The loop
variable `role` is ONLY in scope inside materialize's loop (file.go:315-317). Therefore the parse +
error wrapping MUST live inside materialize.

BUT `materialize` currently returns ONLY `*Config` (no error):
```go
// file.go:209
func materialize(fc *fileConfig, timeout, hookTimeout time.Duration) *Config {
    ...
    return c   // file.go:319
}
```
So materialize's signature MUST change to `(*Config, error)`. This is a deliberate, task-required
design choice — NOT optional. Ripples to exactly 1 production caller + 9 test call sites (below).

(Alternative considered: pre-parse per-role timeouts in loadTOML and pass a map to materialize,
mirroring how the global timeout/hook_timeout are parsed in loadTOML today. REJECTED because the
task explicitly specifies the parse + role-wrapped error happen in the materialize loop.)

## DELTA 1 — Production caller of materialize (loadTOML)

`internal/config/file.go:198` — currently:
```go
return materialize(&fc, timeout, hookTimeout), nil
```
Must become:
```go
cfg, err := materialize(&fc, timeout, hookTimeout)
if err != nil {
    return nil, err
}
return cfg, nil
```
(This is the ONLY non-test caller — grep `func materialize\|materialize(` in non-test config code
returns just file.go:198 + the def at :209.)

## DELTA 2 — All 9 test call sites of materialize (signature churn)

Each `X := materialize(...)` must become `X, err := materialize(...)` (+ check err). None of these
tests set a per-role timeout string, so none will actually error — but the 2-value return is
required to compile. Exact sites:
- `internal/config/file_test.go:837`  — `c := materialize(fc, 0, 0)`
- `internal/config/file_test.go:890`  — `g := materialize(&fileConfig{...}, 0, 0)`
- `internal/config/file_test.go:892`  — `r := materialize(&fileConfig{...}, 0, 0)`
- `internal/config/file_test.go:997`  — `c := materialize(fc, 0, 0)`
- `internal/config/file_test.go:1060` — `g := materialize(&fileConfig{...}, 0, 0)`
- `internal/config/file_test.go:1065` — `r := materialize(&fileConfig{...}, 0, 0)`
- `internal/config/multiturn_test.go:46`  — `c := materialize(fc, 0, 0)`
- `internal/config/multiturn_test.go:85`  — `g := materialize(&fileConfig{...}, 0, 0)`
- `internal/config/multiturn_test.go:102` — `global := materialize(&fileConfig{...}, 0, 0)`

Recommended idiom: `X, err := materialize(...); if err != nil { t.Fatalf("materialize: %v", err) }`
(or a thin `mustMaterialize(t, fc)` helper if preferred). Do NOT use bare `X, _ := ...` if the
project's errcheck linter flags ignored errors — capture + t.Fatal is safest.

## DELTA 3 — The ONLY thing that breaks: `RoleConfig(frc)` at file.go:316

grep `RoleConfig(` (positional/conversion) across internal/ pkg/ cmd/ returns EXACTLY ONE hit:
`internal/config/file.go:316: c.Roles[role] = RoleConfig(frc)`. Every other RoleConfig construction
uses NAMED fields and is safe to leave alone:
- `internal/decompose/roles.go:196` — `config.RoleConfig{Provider: prov, Model: mdl, Reasoning: rsn}` (named) ✓
- `pkg/stagecoach/stagecoach.go:287` — `map[string]config.RoleConfig{}` (empty) ✓
- test literals — all named-field (`{Provider: "agy", Model: ...}`) ✓

So the ONLY structural rewrite is the materialize loop body. Adding the field compiles clean
everywhere else.

## DELTA 4 — Use `parseTimeout` (NOT `time.ParseDuration`) for per-role, per task

`parseTimeout` is at `internal/config/load.go:616-628` (unexported, same package → accessible from
file.go). It accepts BOTH `"120s"`/`"2m"` (Go duration) AND bare `"120"` (seconds). The global
timeout in loadTOML is parsed with `time.ParseDuration` (file.go:183) which REJECTS bare `"120"`
— a pre-existing inconsistency (Finding 9). The task explicitly says use `parseTimeout` for the
per-role field so file/env/flag/git all behave identically. Import path: none needed (same package).

## DELTA 5 — Field placement + toml tags

- `RoleConfig` (config.go:36-40): add `Timeout time.Duration` AFTER Reasoning. The toml tag is
  COSMETIC here — `Config.Roles` is `toml:"-"` (config.go:170), so RoleConfig is never
  unmarshaled/serialized. Add `toml:"timeout"` for consistency with the sibling Provider/Model/
  Reasoning tags, but it has no functional effect. Comment per task:
  `// per-role generation timeout (FR-R7); 0 ⇒ inherit global [defaults].timeout`.
- `fileRoleConfig` (file.go:23-27): add `Timeout string` AFTER Reasoning. The toml tag HERE IS
  FUNCTIONAL — `[role.<role>].timeout` decodes into it. Use `toml:"timeout"`. Comment per task:
  `// §16.4 duration string, e.g. "480s"; parsed in materialize`.

## DELTA 6 — Error message format (follow task literally)

Task specifies the exact wrap: `fmt.Errorf("parse config: [role.%s].timeout: %w", role, err)`.
Use it verbatim inside materialize. loadTOML returns it as-is (`return nil, err`). (The global
timeout error in loadTOML includes the file PATH, but materialize has no path in scope; the role
name is the more actionable context anyway. Do NOT double-wrap with "parse config" in loadTOML.)

## SCOPE BOUNDARIES (sibling subtasks — do NOT implement here)
- **P1.M1.T1.S2** (next): add the `if rc.Timeout != 0 { existing.Timeout = rc.Timeout }` field-merge
  branch to overlay (file.go:440-458). The overlay COMPILES FINE without it after this subtask (it
  just doesn't merge Timeout yet) — but a per-role timeout in a REPO file won't survive into the
  resolved Config until S2 lands. Out of scope here.
- **P1.M1.T2.S1-S3**: `setRoleTimeout` + env loop `_TIMEOUT` branch, the 4 `--<role>-timeout` CLI
  flags, and the NEW `stagecoach.role.<role>.timeout` git-config reading. Do NOT add.
- **P1.M2.T1.S1**: `ResolveRoleTimeout` + `defaultRoleTimeouts` map (planner=480s). Do NOT add.
- **P1.M2.T2.S1**: change global default 480s→120s + fix the ~4 pinning tests. Do NOT do here —
  `Defaults().Timeout` STAYS 480s in this subtask, so the existing 480s-pinning tests still pass.
- **P1.M3.*** : the 13 `provider.Execute` call sites switching to per-role timeout. Do NOT touch.
- **P1.M4.T2.S1**: README/docs. No docs change in this subtask (internal struct change only).

## NOTE — what "works" after THIS subtask in isolation
After S1 alone: `[role.<role>].timeout` decodes (fileRoleConfig.Timeout), parses to a Duration, and
lands on `RoleConfig.Timeout` in the materialized *Config. But because overlay (S2) doesn't merge it
yet, a per-role timeout in a REPO file that overlays a global file is DROPPED at the overlay step.
So end-to-end per-role timeout only fully works after S1+S2. This is expected (atomic-by-subtask
compile-clean, not atomic-by-subtask feature-complete). The unit test in S1 should call materialize
DIRECTLY (not the full loadTOML→overlay chain) to avoid the S2 dependency in the assertion.
