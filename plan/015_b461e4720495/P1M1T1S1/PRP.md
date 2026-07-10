name: "P1.M1.T1.S1 — Add Timeout to RoleConfig + fileRoleConfig + rewrite materialize loop (FR-R7)"
description: >
  Add a `Timeout time.Duration` field to `RoleConfig` (0 = inherit global) and a `Timeout string`
  field to `fileRoleConfig` (the TOML decode twin), and rewrite the materialize loop to construct
  `RoleConfig` field-by-field (parsing each role's duration string via `parseTimeout`) instead of
  the `RoleConfig(frc)` direct-struct-conversion that breaks once the two structs' field types
  diverge. The foundational, compile-clean config change that the rest of FR-R7 (overlay field-merge,
  env/flag/git wiring, resolution, 13 call sites) builds on. Internal struct change only — no docs,
  no default change, no consumer changes.

---

## Goal

**Feature Goal**: Give `RoleConfig` a per-role `Timeout time.Duration` field (PRD §9.15 FR-R7,
§16.4) and make the file→Config materialize step parse `[role.<role>].timeout` duration strings
into that field — without breaking the `RoleConfig(frc)` direct-struct-conversion that currently
works only because both structs are all-strings.

**Deliverable**: (1) `RoleConfig.Timeout time.Duration` added after `Reasoning` (config.go);
(2) `fileRoleConfig.Timeout string` added after `Reasoning` (file.go, functional toml tag `timeout`);
(3) the materialize loop (file.go:313-317) rewritten to field-by-field construction with per-role
`parseTimeout` parsing + role-context error wrapping; (4) `materialize`'s signature changed to
return `(*Config, error)` (required so the loop can surface parse errors with the role name) and its
single production caller + 9 test call sites updated; (5) a unit test proving `[role.planner].timeout`
parses correctly (string form, bare-int form, empty=inherit, malformed=error).

**Success Definition**:
- `go build ./...` compiles clean across all packages (RoleConfig is used in config, decompose,
  generate, cmd, pkg).
- `materialize` returns a parsed `Timeout` on each role's `RoleConfig` (480s string → 480s duration;
  bare `"480"` → 480s via `parseTimeout`; `""` → 0/inherit).
- A malformed `[role.planner].timeout` makes `materialize` return an error wrapping
  `[role.planner].timeout`; `loadTOML` surfaces it (load fails at config-load time, not generation).
- `make test` + `make lint` pass (the global default stays 480s here — the 120s change is P1.M2.T2,
  so existing 480s-pinning tests are untouched).

## Why

- **FR-R7 / §9.15 / §16.1**: Today there is exactly ONE generation timeout (`Config.Timeout`,
  flat 480s), passed identically to all 13 `provider.Execute` call sites. FR-R7 makes each role
  resolve its OWN timeout (global default 120s; planner built-in 480s; per-role overrides). The
  first prerequisite is a place to CARRY a per-role timeout through the resolved `Config` — i.e. a
  `Timeout` field on `RoleConfig` — and a way to parse `[role.<role>].timeout` from the TOML file
  into it. This subtask is that prerequisite.
- **Why it's a separate, careful subtask (Finding 1, HIGHEST RISK)**: the current materialize loop
  uses `c.Roles[role] = RoleConfig(frc)` — a Go direct-struct-conversion that compiles ONLY because
  `fileRoleConfig` and `RoleConfig` have identical field types (all `string`). The instant
  `RoleConfig.Timeout` becomes `time.Duration` while `fileRoleConfig.Timeout` stays `string`, that
  conversion stops compiling. The fix (field-by-field construction + duration parse) is mechanical
  but must be done atomically with the field addition, or nothing compiles.

## What

**User-visible behavior**: None yet (internal struct change). The TOML key `[role.<role>].timeout`
becomes decodable/parseable, but it is not yet field-merged across layers (overlay = S2), not yet
settable via env/flag/git (T2), not yet resolved (P1.M2.T1), and not yet consumed (P1.M3). Full
end-to-end per-role timeout behavior lands across the milestone; this subtask is the load-bearing
config foundation.

**Technical change (atomic — the field add + loop rewrite must land together or it won't compile):**
1. `RoleConfig` gains `Timeout time.Duration` (0 = inherit).
2. `fileRoleConfig` gains `Timeout string` (the TOML decode field).
3. `materialize` signature → `(*Config, error)`; the roles loop constructs `RoleConfig` field-by-field
   and parses each `frc.Timeout` via `parseTimeout` (handles `"480s"` AND bare `"480"`), wrapping
   parse errors with `[role.<role>]` context; empty string → 0 (inherit).
4. `loadTOML` (the one production caller) updated for the new signature.
5. 9 test call sites of `materialize` updated to the 2-value return.

### Success Criteria
- [ ] `RoleConfig.Timeout time.Duration` present (0 = inherit global)
- [ ] `fileRoleConfig.Timeout string` present (toml tag `timeout`)
- [ ] `materialize` returns `(*Config, error)`; roles loop parses per-role timeout via `parseTimeout`
- [ ] `[role.planner].timeout = "480s"` → materialized `Roles["planner"].Timeout == 480*time.Second`
- [ ] bare `"480"` → 480s; `""`/omitted → 0 (inherit)
- [ ] malformed value → materialize error wraps `[role.planner].timeout`
- [ ] `go build ./...` clean; `make test` + `make lint` pass; global default stays 480s

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact structs, the breaking conversion (and proof it's the only one), the parseTimeout helper, the
materialize-signature-change rationale, every test call site to update, and the scope boundaries against sibling subtasks are all enumerated below (verified by grep).

### Documentation & References

```yaml
- file: internal/config/config.go
  why: "RoleConfig struct (lines 36-40) — add the Timeout field after Reasoning. The resolved per-role table lives at Config.Roles (config.go:170, toml:\"-\")."
  pattern: "RoleConfig is the RESOLVED typed struct; its toml tags are cosmetic (Roles is never unmarshaled). Provider/Model/Reasoning are all string with \"\" = inherit."
  gotcha: "RoleConfig.Timeout's toml tag is cosmetic (Config.Roles is toml:\"-\"); add toml:\"timeout\" only for tag-consistency with siblings. 0 (duration zero) is the inherit sentinel — mirrors Config.Timeout."

- file: internal/config/file.go
  why: "THE change site. fileRoleConfig (lines 23-27) — add Timeout string (FUNCTIONAL toml tag). materialize loop (lines 313-317) — rewrite. materialize signature (line 209) — add error return. loadTOML caller (line 198) — update."
  pattern: "fileRoleConfig is the TOML decode twin; its toml tag IS functional ([role.<role>].timeout decodes here). The global timeout is parsed in loadTOML (line 183) with time.ParseDuration — but per-role MUST use parseTimeout (consistency with env/flag/git; accepts bare int)."
  gotcha: "RoleConfig(frc) at line 316 is the ONLY direct-struct-conversion (grep-confirmed). It compiles only because both structs are all-strings; adding a Duration to one side breaks it. Rewrite field-by-field. materialize has NO error return today but the parse must wrap with the role name (only in scope in the loop) → materialize MUST become (*Config, error)."

- file: internal/config/load.go
  why: "parseTimeout helper (lines 616-628) — the single parse helper to reuse. Unexported, same package (config) → directly callable from file.go (no import)."
  pattern: "parseTimeout(s) tries time.ParseDuration first ('120s','2m'), then strconv.Atoi as bare seconds ('120'). Returns wrapped error if neither. Mirrors STAGECOACH_TIMEOUT/--timeout/git stagecoach.timeout behavior exactly."
  gotcha: "Do NOT use time.ParseDuration for the per-role field — it rejects bare '120' (the global timeout's pre-existing inconsistency, Finding 9). parseTimeout is the consistent choice."

- docfile: plan/015_b461e4720495/architecture/critical_findings.md
  why: "Finding 1 (RoleConfig(frc) breaks — HIGHEST RISK) + Finding 9 (parseTimeout exists, accepts both forms; file layer's time.ParseDuration inconsistency)."
  section: "Finding 1; Finding 9"

- docfile: plan/015_b461e4720495/architecture/research_role_config.md
  why: "The full RoleConfig/fileRoleConfig/materialize/overlay chain + the exact field-merge the NEXT subtask (S2) adds. §1 covers this subtask's structures + the conversion-break proof."
  section: "1. The per-role config struct + TOML parsing"

- docfile: plan/015_b461e4720495/P1M1T1S1/research/verification_deltas.md
  why: "The materialize-signature-change decision, all 9 test call sites, scope boundaries, and the note that end-to-end per-role timeout needs S1+S2 (so the unit test must call materialize directly, not the loadTOML→overlay chain). READ THIS before editing."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  config.go       # RoleConfig (36-40, add Timeout); Config.Roles (170, toml:"-"); Defaults() (Timeout 480s — UNCHANGED here)
  file.go         # fileRoleConfig (23-27, add Timeout string); materialize (209, sig→(*Config,error)); roles loop (313-317, rewrite); loadTOML (198, update caller)
  load.go         # parseTimeout (616-628, reuse — same package, unexported)
  file_test.go    # 6 materialize call sites (837,890,892,997,1060,1065) + role test (~544) to extend
  multiturn_test.go # 3 materialize call sites (46,85,102)
  config_test.go  # Defaults() Timeout pinning (480s) — UNCHANGED here (default change is P1.M2.T2)
internal/decompose/roles.go:196   # config.RoleConfig{...} NAMED-field literal — SAFE, no change
pkg/stagecoach/stagecoach.go:287  # map[string]config.RoleConfig{} empty — SAFE, no change
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (atomic compile): adding Timeout time.Duration to RoleConfig while fileRoleConfig.Timeout
//   stays string BREAKS the `RoleConfig(frc)` conversion at file.go:316 (the ONLY such conversion —
//   grep-confirmed). The field add + loop rewrite MUST land together or nothing compiles.

// CRITICAL (materialize signature): materialize currently returns only *Config (file.go:209/319).
//   The task requires parsing frc.Timeout INSIDE the loop and wrapping errors with the role name
//   (`[role.%s]`), which is only in scope inside the loop. Therefore materialize MUST return
//   (*Config, error). Ripples to loadTOML (file.go:198) + 9 test call sites. This is task-required,
//   not optional.

// GOTCHA (parse helper): use parseTimeout (load.go:616), NOT time.ParseDuration. parseTimeout accepts
//   BOTH "480s" and bare "480"; time.ParseDuration rejects bare ints. The global timeout uses
//   time.ParseDuration (a pre-existing inconsistency); per-role uses parseTimeout for file/env/flag/git
//   consistency. parseTimeout is unexported but SAME PACKAGE (config) — no import needed in file.go.

// GOTCHA (inherit sentinel): time.Duration zero value is 0, and 0 == "inherit global" — exactly how
//   Config.Timeout's non-zero overlay guard already works. Empty fileRoleConfig.Timeout string ("")
//   ⇒ parse skipped ⇒ Timeout stays 0 ⇒ inherit. Do NOT treat 0 as an error.

// GOTCHA (toml tag cosmetics): RoleConfig's toml tags are COSMETIC (Config.Roles is toml:"-").
//   fileRoleConfig's toml tag for Timeout IS functional — [role.<role>].timeout decodes into it.

// SCOPE: overlay field-merge for Timeout is the NEXT subtask (S2). After S1 alone, a per-role timeout
//   in a REPO file is DROPPED at overlay (the branch isn't there yet). So the S1 unit test must call
//   materialize DIRECTLY (not loadTOML→overlay) for its assertion. Do NOT add the overlay branch here.
```

## Implementation Blueprint

### Data models and structure

No new types. Two existing structs each gain one field; `materialize` gains an `error` return. The
`0`-duration "inherit" sentinel mirrors the existing `Config.Timeout` discipline (non-zero overlay).

```go
// config.go — RoleConfig (resolved; toml tags cosmetic since Roles is toml:"-")
type RoleConfig struct {
    Provider  string        `toml:"provider"`
    Model     string        `toml:"model"`
    Reasoning string        `toml:"reasoning"`
    Timeout   time.Duration `toml:"timeout"` // per-role generation timeout (FR-R7); 0 ⇒ inherit global [defaults].timeout
}

// file.go — fileRoleConfig (TOML decode twin; toml tag FUNCTIONAL)
type fileRoleConfig struct {
    Provider  string `toml:"provider"`
    Model     string `toml:"model"`
    Reasoning string `toml:"reasoning"`
    Timeout   string `toml:"timeout"` // §16.4 duration string, e.g. "480s"; parsed in materialize
}
```

### Implementation Tasks (ordered by dependencies)

> **Do Tasks 1–3 as one atomic edit set**, then `go build ./...`. The field add (Tasks 1+2) breaks
> the conversion at file.go:316 until Task 3 rewrites it; incremental compile is impossible mid-way.

```yaml
Task 1: MODIFY internal/config/config.go — add Timeout to RoleConfig
  - EDIT the RoleConfig struct (lines 36-40): add after the Reasoning field:
        Timeout   time.Duration `toml:"timeout"` // per-role generation timeout (FR-R7); 0 ⇒ inherit global [defaults].timeout
  - ENSURE the `time` package is already imported in config.go (it is — Config.Timeout uses it).
  - UPDATE the RoleConfig doc comment (lines 22-35) to mention the new Timeout field's inherit semantics (0 ⇒ global) alongside Provider/Model/Reasoning.
  - NOTE: the toml tag is cosmetic (Config.Roles is toml:"-"); add it for sibling-tag consistency only.
  - DEPENDENCIES: none.

Task 2: MODIFY internal/config/file.go — add Timeout string to fileRoleConfig
  - EDIT the fileRoleConfig struct (lines 23-27): add after Reasoning:
        Timeout   string `toml:"timeout"` // §16.4 duration string, e.g. "480s"; parsed in materialize
  - NOTE: this toml tag IS functional — [role.<role>].timeout decodes here.
  - DEPENDENCIES: Task 1 (both structs gain the field together).

Task 3: MODIFY internal/config/file.go — materialize signature → (*Config, error); rewrite roles loop
  - EDIT the materialize signature (line 209):
        func materialize(fc *fileConfig, timeout, hookTimeout time.Duration) (*Config, error) {
  - EDIT the final return (line 319): `return c` → `return c, nil`
  - REWRITE the roles loop (lines 311-317). Replace:
        for role, frc := range fc.Role {
            c.Roles[role] = RoleConfig(frc)
        }
    with field-by-field construction + parseTimeout:
        for role, frc := range fc.Role {
            var rt time.Duration
            if frc.Timeout != "" {
                d, perr := parseTimeout(frc.Timeout) // parseTimeout: "480s" OR bare "480" (load.go:616)
                if perr != nil {
                    return nil, fmt.Errorf("parse config: [role.%s].timeout: %w", role, perr)
                }
                rt = d
            }
            c.Roles[role] = RoleConfig{
                Provider:  frc.Provider,
                Model:     frc.Model,
                Reasoning: frc.Reasoning,
                Timeout:   rt, // 0 ⇒ inherit global (S2's overlay + P1.M2.T1's ResolveRoleTimeout apply it)
            }
        }
  - UPDATE the loop's preceding comment (lines 311-312) to note the field-by-field copy + timeout parse (the old comment said "direct conversion").
  - CONFIRM `fmt` is imported in file.go (it is — used elsewhere).
  - DEPENDENCIES: Tasks 1+2 (the field must exist on both structs) — AND this is the change that makes it compile again.

Task 4: MODIFY internal/config/file.go — update loadTOML (the production caller)
  - EDIT line 198. Replace:
        return materialize(&fc, timeout, hookTimeout), nil
    with:
        cfg, err := materialize(&fc, timeout, hookTimeout)
        if err != nil {
            return nil, err
        }
        return cfg, nil
  - DEPENDENCIES: Task 3.

Task 5: MODIFY the 9 materialize test call sites (signature churn — 2-value return)
  - Each `X := materialize(...)` → `X, err := materialize(...)` + error check. Exact sites:
      internal/config/file_test.go:837, 890, 892, 997, 1060, 1065
      internal/config/multiturn_test.go:46, 85, 102
  - RECOMMENDED idiom: `X, err := materialize(...); if err != nil { t.Fatalf("materialize: %v", err) }`
    (or a thin mustMaterialize(t, fc) helper in one test file if you prefer DRY). Avoid bare `X, _ :=`
    if errcheck flags ignored errors — capture + t.Fatal is safest.
  - NONE of these tests set a per-role timeout, so none will actually error; this is pure signature churn.
  - DEPENDENCIES: Task 3.

Task 6: CREATE/EXTEND a unit test proving per-role timeout materialize (the load-bearing proof)
  - ADD to internal/config/file_test.go a test TestMaterializeRoleTimeout that calls materialize
    DIRECTLY (NOT loadTOML→overlay — overlay doesn't merge Timeout until S2; see GOTCHA). Table cases:
      * [role.planner] timeout="480s"        → Roles["planner"].Timeout == 480*time.Second
      * [role.planner] timeout="480" (bare)  → 480*time.Second  (proves parseTimeout, not ParseDuration)
      * [role.planner] timeout omitted/""    → 0 (inherit)
      * [role.planner] timeout="not-a-dur"   → materialize returns error wrapping "[role.planner].timeout"
      * two roles, one with timeout one without → only the set role has non-zero Timeout
  - MIRROR the existing role-decode test (TestLoadTOML_RolesProviderModel ~file_test.go:544) for the
    TOML-string form, and the materialize-direct-call style of TestMaterializeOverlay_DiffContext
    (~file_test.go:814) for the direct-call form. Use the writeTempTOML helper (file_test.go:14) for
    the loadTOML error case if you want path context, but the direct-materialize form is sufficient
    and avoids the S2 overlay dependency.
  - For the DIRECT materialize form, construct fileRoleConfig with the Timeout string and call
    materialize(&fileConfig{Role: map[string]fileRoleConfig{"planner": {Timeout: "480s"}}}, 0, 0).
  - DEPENDENCIES: Tasks 3-5.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the field-by-field materialize loop (replaces RoleConfig(frc))
for role, frc := range fc.Role {
    var rt time.Duration
    if frc.Timeout != "" {                       // "" ⇒ inherit (stay 0)
        d, perr := parseTimeout(frc.Timeout)     // load.go:616 — "480s" OR bare "480"
        if perr != nil {
            return nil, fmt.Errorf("parse config: [role.%s].timeout: %w", role, perr)
        }
        rt = d
    }
    c.Roles[role] = RoleConfig{Provider: frc.Provider, Model: frc.Model, Reasoning: frc.Reasoning, Timeout: rt}
}

// PATTERN: the 0-duration "inherit" sentinel (mirrors Config.Timeout)
//   time.Duration zero == 0 == "inherit global". Empty fileRoleConfig.Timeout ⇒ parse skipped ⇒ 0.
//   The overlay guard S2 will add is `if rc.Timeout != 0 { existing.Timeout = rc.Timeout }`
//   (duration-non-zero, identical discipline to Config.Timeout's overlay at file.go:399).

// PATTERN: materialize error propagation (loadTOML — file.go:198)
cfg, err := materialize(&fc, timeout, hookTimeout)
if err != nil {
    return nil, err   // surface role-context error verbatim; do NOT re-wrap (avoids double "parse config")
}
return cfg, nil
```

### Integration Points

```yaml
NO database / routes / CLI / public-API changes. Pure internal config-struct change.

STRUCT CHANGES:
  - RoleConfig (config.go:36):      + Timeout time.Duration  (0 = inherit; toml tag cosmetic)
  - fileRoleConfig (file.go:23):    + Timeout string         (toml:"timeout"; FUNCTIONAL decode field)

FUNCTION CHANGES:
  - materialize (file.go:209):      *Config → (*Config, error)  (roles loop parses + wraps role errors)
  - loadTOML (file.go:198):         updated for the new signature

DOWNSTREAM (this subtask ENABLES but does NOT build — sibling subtasks):
  - P1.M1.T1.S2 (overlay):      adds `if rc.Timeout != 0 { existing.Timeout = rc.Timeout }` to file.go:440-458
  - P1.M1.T2 (env/flag/git):    setRoleTimeout + STAGECOACH_<ROLE>_TIMEOUT + --<role>-timeout + stagecoach.role.<role>.timeout
  - P1.M2.T1 (resolution):      ResolveRoleTimeout + defaultRoleTimeouts{planner:480s}
  - P1.M2.T2 (default):         global 480s→120s + fix ~4 pinning tests
  - P1.M3 (consumption):        13 provider.Execute call sites switch to per-role resolved timeout

UNCHANGED (do NOT touch): Defaults().Timeout stays 480s; the 4 pinning tests stay 480s; Execute;
  all 13 call sites; ResolveRoleModel; docs.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build EVERYTHING — RoleConfig is used across config/decompose/generate/cmd/pkg; the type change
# must compile everywhere (all named-field literals are safe; the one conversion was rewritten).
go build ./...

# Vet (catches the unused-import / shadow risks from the materialize rewrite)
go vet ./...

# Format
gofmt -l internal/config/
# Expected: empty. If listed: gofmt -w internal/config/

# Lint
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Config package — materialize + role parsing
go test ./internal/config/... -run 'Materialize|Role|Timeout' -v
# Expected: all pass, incl. new TestMaterializeRoleTimeout (480s string, bare 480, empty=0, malformed=error).

# Full config package (the 9 signature-churned call sites must still compile + pass)
go test ./internal/config/... -v

# Whole suite (race) — RoleConfig is consumed by decompose/generate; ensure no regressions
make test
# Expected: ALL pass. Global default still 480s (unchanged here) → 480s-pinning tests untouched.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary
make build

# Smoke: config init still works (RoleConfig field addition does not affect the bootstrap template;
# the [role.*] TOML is written from hardcoded strings, not struct marshal — verify no panic):
./bin/stagecoach config init --template 2>/dev/null | grep -i timeout || true

# Smoke: a TOML with [role.planner].timeout loads cleanly (parse happens; no consumer uses it yet,
# but load must not error). This proves loadTOML→materialize end-to-end for the happy path:
cat > /tmp/sc_role_timeout.toml <<'EOF'
[defaults]
provider = "pi"
[role.planner]
timeout = "480s"
EOF
./bin/stagecoach --config /tmp/sc_role_timeout.toml --dry-run --no-color 2>&1 | head -5
# Expected: loads without a "[role.planner].timeout" parse error (it may exit for other reasons —
#           e.g. nothing staged — but NOT a timeout parse error). A malformed value here would print
#           "parse config: [role.planner].timeout: ..." and exit 1.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: prove NO RoleConfig(frc) direct-conversion remains (the break is fully rewritten)
grep -rn "RoleConfig(frc)\|RoleConfig([^){]" --include="*.go" internal/config/
# Expected: empty.

# Grep guard: prove materialize's NEW signature is used everywhere (no stale single-value callers)
grep -rn "materialize(" --include="*.go" internal/config/ | grep -v "func materialize"
# Expected: every call site is the 2-value form (X, err := materialize(...)) or loadTOML's cfg,err.

# Scope-boundary guard: this subtask did NOT add overlay/env/flag/git/resolution/default changes
grep -rn "ResolveRoleTimeout\|setRoleTimeout\|defaultRoleTimeouts" internal/config/
# Expected: empty (those are P1.M2.T1 / P1.M1.T2 — NOT this subtask).
grep -n "rc.Timeout\|existing.Timeout" internal/config/file.go
# Expected: empty in the overlay block (the field-merge is S2; only the materialize loop touches Timeout here).
grep -n "120 \* time.Second" internal/config/config.go
# Expected: empty (global default 480s→120s is P1.M2.T2; Defaults().Timeout must still be 480*time.Second here).

# Confirm parseTimeout (not time.ParseDuration) is used for the per-role field
grep -n "parseTimeout\|time.ParseDuration" internal/config/file.go
# Expected: the roles loop uses parseTimeout; the global timeout + hook_timeout still use time.ParseDuration (unchanged).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean (all packages — RoleConfig is widely consumed)
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/config/` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass, incl. new TestMaterializeRoleTimeout

### Feature Validation
- [ ] `RoleConfig.Timeout time.Duration` present (0 = inherit)
- [ ] `fileRoleConfig.Timeout string` present (toml tag `timeout`)
- [ ] `materialize` returns `(*Config, error)`; roles loop field-by-field + parseTimeout
- [ ] `[role.planner].timeout = "480s"` → materialized Timeout == 480s (unit test)
- [ ] bare `"480"` → 480s (proves parseTimeout used, not ParseDuration)
- [ ] `""`/omitted → 0 (inherit)
- [ ] malformed → error wraps `[role.planner].timeout`
- [ ] loadTOML surfaces the error (load-time failure)

### Scope-Boundary Validation
- [ ] NO overlay Timeout field-merge added (that's S2)
- [ ] NO setRoleTimeout / env / flag / git-config per-role timeout added (P1.M1.T2)
- [ ] NO ResolveRoleTimeout / defaultRoleTimeouts added (P1.M2.T1)
- [ ] `Defaults().Timeout` STILL 480s; 480s-pinning tests UNCHANGED (P1.M2.T2)
- [ ] NO Execute call-site / docs changes (P1.M3 / P1.M4.T2)

### Code Quality & Docs
- [ ] RoleConfig + fileRoleConfig doc comments updated for the new field
- [ ] materialize loop comment updated (no stale "direct conversion" wording)
- [ ] Test call sites use a consistent error-handling idiom (not bare `_`)

---

## Anti-Patterns to Avoid

- ❌ Don't leave `RoleConfig(frc)` in place — it won't compile once the field types diverge (Duration vs string). Rewrite field-by-field.
- ❌ Don't use `time.ParseDuration` for the per-role field — it rejects bare `"120"`. Use `parseTimeout` (load.go:616) for file/env/flag/git consistency.
- ❌ Don't try to keep `materialize` returning only `*Config` — the task requires role-context error wrapping, and the role name is only in scope inside the loop. Change the signature to `(*Config, error)`.
- ❌ Don't re-wrap materialize's error in loadTOML with another "parse config:" prefix — it double-prefixes. Return it verbatim (`return nil, err`).
- ❌ Don't add the overlay Timeout field-merge here (that's S2). After S1 alone a repo-file per-role timeout is dropped at overlay — expected, not a bug. The S1 unit test must call materialize directly to avoid depending on S2.
- ❌ Don't change `Defaults().Timeout` (480s→120s) or any 480s-pinning test here — that's P1.M2.T2 and would break unrelated tests in this subtask's gate.
- ❌ Don't touch the 13 `provider.Execute` call sites, `ResolveRoleModel`, env/flag/git loaders, or docs — all out of scope (P1.M1.T2 / P1.M2 / P1.M3 / P1.M4.T2).
- ❌ Don't add a toml tag to RoleConfig.Timeout expecting it to function — `Config.Roles` is `toml:"-"`, so RoleConfig is never unmarshaled. The tag is cosmetic; the FUNCTIONAL decode tag is on fileRoleConfig.Timeout.
- ❌ Don't treat a 0 duration as an error — 0 is the "inherit global" sentinel (mirrors Config.Timeout).

---

## Confidence Score: 9/10

One-pass success is very high: the change is small and localized, the breaking conversion is the
only one of its kind (grep-confirmed), every test call site is enumerated, and the architecture docs
already proved the field-by-field rewrite. The -1 is for the `materialize` signature change to
`(*Config, error)` — the task description describes the parse + role-wrapped error inside the loop
but does NOT explicitly state materialize must gain an error return; an implementer following the
letter of the task without that inference would hit a compile wall ("cannot use materialize(...)
as *Config value"). This PRP makes that decision explicit (CRITICAL gotcha) and lists all 9
affected test sites, removing the ambiguity.
