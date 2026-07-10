name: "P1.M2.T1.S1 — Add ResolveRoleTimeout + defaultRoleTimeouts map (planner=480s) (FR-R7)"
description: >
  Add a sibling resolution function `ResolveRoleTimeout(role string, cfg Config) time.Duration` next to
  `ResolveRoleModel` in `internal/config/roles.go`, plus a `defaultRoleTimeouts` map (planner=480s only).
  This is the pure READ-SIDE accessor for FR-R7 per-role timeouts — the consumers (the 13 Execute call
  sites in P1.M3) will call it at each site instead of the flat `cfg.Timeout`. It applies a 3-tier
  precedence (mirroring ResolveRoleModel's per-field shape, plus a 3rd built-in tier): per-role override
  (`cfg.Roles[role].Timeout`, non-zero) > built-in role default (planner=480s) > global (`cfg.Timeout`).
  Inputs are all already in place from the completed P1.M1 milestone (`RoleConfig.Timeout`,
  `fileRoleConfig.Timeout`, materialize/overlay, `setRoleTimeout`, env/flag/git layers). This task adds
  ONLY the function + the map + 6 unit tests. It does NOT change the global default (stays 480s; that is
  P1.M2.T2.S1), does NOT wire any consumer (P1.M3), and does NOT touch docs (P1.M4.T2). The planner 480s
  built-in is a role×timeout axis and lives in roles.go (NOT role_defaults.go's provider×role model table).

---

## Goal

**Feature Goal**: Provide the read-side resolution function that turns the already-populated
`cfg.Roles[role].Timeout` (plus the global `cfg.Timeout`) into a single per-role `time.Duration`,
with the planner getting a shipped built-in of 480s that **beats the global** for that role (so the
planner keeps a generous deadline even after P1.M2.T2 lowers the global default to 120s). Every other
role (stager/message/arbiter) inherits `cfg.Timeout` (no built-in).

**Deliverable**:
1. **internal/config/roles.go** — (a) `import "time"` (the file has NO import block today); (b) a
   `defaultRoleTimeouts` map `var` (`{"planner": 480 * time.Second}`) with a doc comment; (c) a
   `ResolveRoleTimeout(role string, cfg Config) time.Duration` function with a godoc mirroring
   `ResolveRoleModel`'s style and citing FR-R7. Placed alongside `ResolveRoleModel`.
2. **internal/config/roles_test.go** — 6 unit tests cloning the `ResolveRoleModel` test idiom
   (per-role override; planner built-in beats global; non-planner global fallback; field-merge;
   unknown role; Roles-nil fallback). `import "time"` added (the file imports only `"testing"` today).
3. NO consumer wiring, NO global-default change, NO docs.

**Success Definition**:
- `ResolveRoleTimeout("planner", cfg)` with `cfg.Roles` empty/unset returns `480 * time.Second`
  (the built-in), **even when** `cfg.Timeout` is set to a different value (proves built-in beats global).
- `ResolveRoleTimeout("message", cfg)` / `"stager"` / `"arbiter"` with no per-role override returns
  `cfg.Timeout` unchanged (no built-in for those roles).
- `ResolveRoleTimeout("planner", cfg)` with `cfg.Roles["planner"].Timeout = 600s` returns `600s`
  (per-role override beats the 480s built-in AND the global).
- `ResolveRoleTimeout("palnner", cfg)` (typo) returns `cfg.Timeout` (unknown role: no entry, no
  built-in → global; same leniency as `ResolveRoleModel`).
- `go build ./...` + `GOOS=windows/linux go build ./...` clean; `make test` + `make lint` +
  `make coverage-gate` pass; `gofmt -l` empty; the 12 existing `ResolveRoleModel` tests stay green.

## User Persona (if applicable)

**Target User**: Stagecoach internals — specifically the decompose role functions and the single-commit
generation path (P1.M3), which need a per-role timeout to pass to `provider.Execute` instead of the flat
`cfg.Timeout`. (End users never call `ResolveRoleTimeout` directly.)

**Use Case**: At each Execute call site, `timeout := config.ResolveRoleTimeout("planner", deps.Config)`
yields 480s for the planner (its open-ended decomposition needs the headroom) while
`config.ResolveRoleTimeout("message", deps.Config)` yields the global for the cheap message role —
each role bounded appropriately without per-role boilerplate at every site.

**User Journey**: planner Execute site → `ResolveRoleTimeout("planner", cfg)` → checks
`cfg.Roles["planner"].Timeout` (user override?) → else the 480s built-in → passes to `provider.Execute`.
(That wiring is P1.M3; this task supplies only the function.)

**Pain Points Addressed**: FR-R7 — today every role shares the single `cfg.Timeout`; the planner (which
genuinely needs more time) is bounded by the same value as the cheap message role. `ResolveRoleTimeout`
+ the planner built-in let the planner default to 480s while the rest drop to a tighter global.

## Why

- **FR-R7 / §9.15 / §16.1**: per-role timeouts with a planner-specific 480s built-in. The PRD phrases it
  as "timeout 120s (global fallback for every role; **planner role default 480s**)". This task is the
  resolution surface that makes that phrasing real — a single accessor the consumers call.
- **Consistency**: it mirrors the proven, fully-tested `ResolveRoleModel` accessor (same file, same
  per-field-merge shape, same godoc discipline, same "role is an arbitrary string" leniency). No new
  pattern; the one structural addition (a 3rd built-in tier) is the FR-R7 requirement itself.
- **Bounded scope**: pure read-side function + map + tests. The config-loading layers are DONE (P1.M1);
  the consumers are LATER (P1.M3); the global-default change is LATER (P1.M2.T2); docs are LATER
  (P1.M4.T2). This task lands independently and is verified by unit tests alone.

## What

**User-visible behavior**: None directly (no consumer yet). Internally, `ResolveRoleTimeout` becomes the
authoritative per-role timeout accessor over the already-resolved `cfg`.

**Technical change** (additive — one function, one map, one import, in one file + tests):

```go
// in internal/config/roles.go (NEW import + map + func, alongside ResolveRoleModel)
import "time"

var defaultRoleTimeouts = map[string]time.Duration{
	"planner": 480 * time.Second,
}

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

### Success Criteria
- [ ] `roles.go` has `import "time"` (it has no import block today).
- [ ] `defaultRoleTimeouts` map exists with exactly `{"planner": 480 * time.Second}` (no other roles).
- [ ] `ResolveRoleTimeout(role string, cfg Config) time.Duration` exists alongside `ResolveRoleModel`.
- [ ] planner (no override) → 480s even when `cfg.Timeout` ≠ 480s (built-in beats global).
- [ ] stager/message/arbiter (no override) → `cfg.Timeout` (no built-in).
- [ ] any role with a non-zero `cfg.Roles[role].Timeout` → that value (override beats built-in + global).
- [ ] unknown/non-canonical role → `cfg.Timeout` (no entry, no built-in).
- [ ] 6 new tests pass; the 12 existing `ResolveRoleModel` tests stay green.
- [ ] ZERO production callers of `ResolveRoleTimeout` (consumer is P1.M3) — only roles_test.go calls it.
- [ ] `Defaults().Timeout` UNCHANGED (still 480s) — P1.M2.T2.S1 owns the 480s→120s flip.
- [ ] `make test` + `make lint` + `make coverage-gate` pass; `gofmt -l` empty; cross-build clean.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim `ResolveRoleModel` template (the structural copy source), the exact 3-tier
precedence with the subtle "built-in beats global for the planner" semantic called out, the placement
decision (roles.go, NOT role_defaults.go, with the rationale), the critical "roles.go has no import block
→ add `import time`" detail, the test idiom to clone (with the disambiguation rule that tests must set
`cfg.Timeout` ≠ 480s), and the explicit scope fences (no consumer, no default change, no docs).

### Documentation & References

```yaml
# MUST READ — the authoritative research (verbatim function + map + the precedence semantic + tests)
- docfile: plan/015_b461e4720495/P1M2T1S1/research/findings.md
  why: "§1 has the verbatim ResolveRoleModel template + the 'roles.go has NO import block' detail; §2 the
        verbatim ResolveRoleTimeout function + godoc; §3 the map + the roles.go-vs-role_defaults.go
        placement decision; §5 the 6 tests with the CRITICAL cfg.Timeout≠480s disambiguation rule."
  critical: "§2/§5: the planner built-in (480s) BEATS the global for the planner role — tests MUST set
             cfg.Timeout to a distinct value (120s) or the planner-built-in-vs-global assertion is
             ambiguous. §1: add `import \"time\"` (roles.go has no imports today)."

# MUST READ — the FR-R7 timeout architecture (the whole plan's research)
- docfile: plan/015_b461e4720495/architecture/research_timeout.md
  why: "§5.2 confirms ResolveRoleModel is THE pattern to mirror; §6 confirms the planner 480s is a
        role×timeout default that does NOT belong in role_defaults.go's provider×role model table; §5.1
        confirms RoleConfig.Timeout is a plain time.Duration (0 ⇒ inherit) — do not promote to *duration."
  critical: "§6: place the map in roles.go (or ResolveRoleTimeout), NOT role_defaults.go. The global
             default change (480s→120s) is explicitly a SEPARATE task (P1.M2.T2) — do not touch Defaults()."

# MUST READ — the file being edited (the copy source + the new function's home)
- file: internal/config/roles.go
  why: "The ENTIRE file today is `package config` + the ResolveRoleModel godoc + ResolveRoleModel. Add
        `import \"time\"`, the defaultRoleTimeouts var, and ResolveRoleTimeout here, alongside the model
        accessor."
  pattern: "ResolveRoleModel: `if rc, ok := cfg.Roles[role]; ok { if rc.X != zero { x = rc.X } } … if x ==
            zero { x = cfg.X }`. ResolveRoleTimeout mirrors this but adds a 2nd fallback tier (the built-in
            map) BETWEEN the per-role entry and the global."
  gotcha: "roles.go has NO import block — you MUST add `import \"time\"` (single import) between `package
           config` and the first comment. Forgetting it is a compile error (time.Duration is unresolved)."

# MUST READ — the input field (already LANDED by P1.M1.T1.S1; consume, don't rebuild)
- file: internal/config/config.go
  why: "RoleConfig.Timeout time.Duration @42 (0 ⇒ inherit global; doc @23-40). Config.Timeout @71
        (global; Defaults @197 = 480s — DO NOT CHANGE). These are the two values ResolveRoleTimeout reads."
  pattern: "RoleConfig fields are plain-typed + 0/empty ⇒ inherit (FR-R2/FR-R3 field-merge). Timeout
            follows the same duration-non-zero discipline as Config.Timeout (overlay guard is `!= 0`)."
  gotcha: "Do NOT change Defaults().Timeout (config.go:197). It stays 480s here; P1.M2.T2.S1 flips it to
           120s. Your tests must not depend on its value — set cfg.Timeout explicitly."

# MUST READ — the tests to clone (the idiom + the disambiguation rule)
- file: internal/config/roles_test.go
  why: "12 ResolveRoleModel tests use the idiom: cfg := Defaults() → mutate cfg.Provider/Model/Timeout +
        cfg.Roles = map[string]RoleConfig{...} → call accessor → assert. Clone this for ResolveRoleTimeout.
        The file imports ONLY `\"testing\"` today — add `\"time\"` to the import group."
  pattern: "TestResolveRoleModel_FullOverride (@32), _GlobalFallbackRolesNil (@5), _ModelOnlyOverride (@47,
            field-merge), _UnknownRoleFallsBackToGlobal (@89) are the 4 shapes to clone."
  critical: "EVERY planner/non-planner test MUST set cfg.Timeout = 120*time.Second (≠ the 480s built-in) so
             the precedence assertion is unambiguous. Using Defaults() directly (Timeout==480s) makes
             planner-built-in-vs-global ambiguous and would pass even if precedence is inverted."

# CONTEXT — the consumer (LANDS LATER, not here)
- file: internal/decompose/planner.go
  why: "P1.M3.T2.S1 will replace `deps.Config.Timeout` (the Execute 3rd arg) with
        `config.ResolveRoleTimeout(\"planner\", deps.Config)` at the ~4 decompose Execute sites + the
        single-commit/multiturn/workdesc sites. NOT this task."
  critical: "Do NOT add any Execute-site change. After this subtask, grep must show ResolveRoleTimeout
             called ONLY in roles_test.go (zero production callers)."

# CONTEXT — the parallel sibling (no overlap)
- docfile: plan/015_b461e4720495/P1M1T2S3/PRP.md
  why: "PARALLEL sibling edits internal/config/git.go (adds the per-role git-config loop reading
        stagecoach.role.<role>.timeout). It does NOT touch roles.go or roles_test.go. Assume it LANDED —
        it populates cfg.Roles[role].Timeout, which my function reads. No file overlap → no conflict."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  roles.go           # EDIT — +import "time", +defaultRoleTimeouts var, +ResolveRoleTimeout func
  roles_test.go      # EDIT — +import "time", +6 ResolveRoleTimeout tests
  config.go          # READ-ONLY — RoleConfig.Timeout @42 (input, LANDED); Config.Timeout @71 + Defaults @197 (UNCHANGE)
  role_defaults.go   # READ-ONLY — provider×role MODEL table (FR-D4); the timeout map does NOT go here
  load.go            # READ-ONLY — setRoleTimeout/env/flag layers (LANDED); not touched
  file.go            # READ-ONLY — fileRoleConfig/materialize/overlay (LANDED); not touched
  git.go             # READ-ONLY — per-role git-config loop (parallel T2.S3); not touched
internal/decompose/  # READ-ONLY — consumer sites (P1.M3); NOT touched this task
go.mod               # READ-ONLY — stdlib only (time); no new dependency
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (no new files):
internal/config/roles.go        # +`import "time"` +`defaultRoleTimeouts` var +`ResolveRoleTimeout` func
internal/config/roles_test.go   # +`"time"` import +6 TestResolveRoleTimeout_* funcs
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (roles.go has NO import block): the file today is `package config` then directly the
// ResolveRoleModel godoc. ResolveRoleTimeout references time.Duration, so you MUST add `import "time"`
// (single import) between `package config` and the first comment. Forgetting it = compile error.

// CRITICAL (the 3-tier precedence — built-in BEATS global for the planner): order is
//   cfg.Roles[role].Timeout (non-zero)  >  defaultRoleTimeouts[role] (planner=480s)  >  cfg.Timeout
// The planner's 480s built-in sits ABOVE the global: a planner with no per-role override gets 480s
// EVEN IF cfg.Timeout is 120s. Only a per-role override (--planner-timeout / [role.planner].timeout /
// STAGECOACH_PLANNER_TIMEOUT / stagecoach.role.planner.timeout) beats the built-in. Do NOT put the
// global above the built-in (that would make the planner default meaningless). This divergence from
// ResolveRoleModel (which has only per-role > global) is THE FR-R7 semantic.

// CRITICAL (tests must disambiguate 480s): Defaults().Timeout is 480s TODAY (P1.M2.T2 changes it to 120s
// later). A planner test using Defaults() would be ambiguous (480s from built-in vs 480s from global).
// Set cfg.Timeout = 120*time.Second in every planner/non-planner test so the precedence assertion fails
// loudly if inverted. Do not couple tests to Defaults()'s Timeout value.

// GOTCHA (placement — roles.go, NOT role_defaults.go): the planner 480s is a role×timeout axis; role_defaults.go
// is the FR-D4 provider×role→MODEL table (a different axis). Mixing them obscures both. Keep the map in
// roles.go next to its sole reader (ResolveRoleTimeout). (architecture/research_timeout.md §6.)

// GOTCHA (RoleConfig.Timeout is plain time.Duration, NOT *time.Duration): 0 ⇒ "inherit global" mirrors
// the "" string fields of ResolveRoleModel. Do NOT promote to a pointer (already decided in T1.S1). The
// non-zero-wins overlay guard (`!= 0`) and the `rc.Timeout != 0` check here are the sentinel mechanism.

// GOTCHA (Execute treats timeout<=0 as "no deadline"): a RESOLVED timeout must never be 0 at a call site.
// ResolveRoleTimeout may return cfg.Timeout unchanged for non-planner roles — if cfg.Timeout were ever 0,
// the consumer (P1.M3) guards it. Do NOT collapse a 0 global into the built-in HERE (the built-in is
// role-specific; only the planner has one). The function returns cfg.Timeout verbatim for non-planner roles.

// GOTCHA (no consumer, or `unused`/`staticcheck` fires): ResolveRoleTimeout has zero production callers
// after this task — that's expected (P1.M3 wires them). The new unit tests read it, so `make lint`'s
// `unused` checker stays clean (test usage counts). Do NOT add a throwaway production caller.
```

## Implementation Blueprint

### Data models and structure

No new types. One new plain function returning `time.Duration` and one package-level `map[string]time.Duration`
var. Both consume the already-existing `RoleConfig.Timeout` / `Config.Timeout` fields (P1.M1). Mirrors
`ResolveRoleModel`'s shape, with one added fallback tier.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/config/roles.go — add `import "time"` (THE prerequisite)
  - The file today starts `package config` then directly the ResolveRoleModel godoc — NO import block.
  - ADD, between `package config` and the first comment:
        import "time"
  - VERIFY: `go build ./internal/config/` (or `go vet`) — without this, Task 2/3 won't compile.

Task 2: EDIT internal/config/roles.go — add the `defaultRoleTimeouts` map
  - PLACE: above ResolveRoleModel (or just above ResolveRoleTimeout in Task 3) — co-located with its reader.
  - ADD (verbatim, with the doc comment):
        // defaultRoleTimeouts are the shipped per-role generation-timeout built-ins (PRD §16.1, FR-R7). The
        // planner is the ONLY role with a built-in timeout (480s) — it does open-ended decomposition planning
        // and needs longer than the message/stager/arbiter roles, which inherit cfg.Timeout. A user override
        // (cfg.Roles[role].Timeout, set via [role.<role>].timeout / --<role>-timeout / env / git-config)
        // ALWAYS beats this. This is a role×timeout axis, distinct from role_defaults.go's provider×role→MODEL
        // table (FR-D4) — do not conflate the two. Add a role here only when shipping a non-global default.
        var defaultRoleTimeouts = map[string]time.Duration{
            "planner": 480 * time.Second,
        }
  - NAMING: defaultRoleTimeouts (unexported; the sole reader is the same-package ResolveRoleTimeout).
  - DO NOT add stager/message/arbiter — they intentionally have NO built-in (inherit cfg.Timeout).

Task 3: EDIT internal/config/roles.go — add `ResolveRoleTimeout` (the deliverable)
  - PLACE: immediately after ResolveRoleModel (same file, sibling accessor).
  - ADD (verbatim, with the godoc):
        // ResolveRoleTimeout returns the generation timeout for a single agent role (PRD §9.15 FR-R7, §16.1),
        // applying the precedence:
        //
        //	[role.<role>].timeout  (CLI flag > env > file > git, all already merged into cfg.Roles by the loaders)
        //	> built-in role default  (planner = 480s; FR-R7 — the planner needs more time than message/stager/arbiter)
        //	> [defaults].timeout     (cfg.Timeout — the global; 480s today, 120s after P1.M2.T2)
        //
        // By the time this runs, Load() has already overlaid every precedence layer into cfg.Roles[role].Timeout,
        // so this only checks the per-role entry, then the built-in role default, then the global. It does NOT
        // re-walk the layers and does NOT consult any manifest — mirroring ResolveRoleModel.
        //
        // The planner is the ONLY role with a shipped built-in timeout (480s): it does the open-ended
        // decomposition planning and most often needs longer than the message/stager/arbiter roles (which
        // inherit cfg.Timeout). A non-zero cfg.Roles[role].Timeout ALWAYS wins — even for the planner (a
        // user's --planner-timeout 600s beats the 480s built-in). A role absent from cfg.Roles (or with
        // Timeout==0) inherits: planner → 480s built-in; stager/message/arbiter → cfg.Timeout.
        //
        // The zero-value sentinel (RoleConfig.Timeout == 0 ⇒ "inherit") mirrors the "" string fields of
        // ResolveRoleModel. A RESOLVED timeout should never be 0 at an Execute call site; the consumers
        // (P1.M3) guard a 0 if it ever occurs (Execute treats timeout<=0 as "no deadline"). This function
        // returns cfg.Timeout unchanged for non-planner roles (do not collapse a 0 global into a built-in
        // here — the built-in is role-specific, planner-only).
        //
        // role is an arbitrary string (one of "planner","stager","message","arbiter" in practice); a
        // non-canonical name misses the cfg.Roles lookup AND the built-in map, so it inherits cfg.Timeout
        // (no error) — same leniency as ResolveRoleModel.
        func ResolveRoleTimeout(role string, cfg Config) time.Duration {
            if rc, ok := cfg.Roles[role]; ok && rc.Timeout != 0 {
                return rc.Timeout
            }
            if d, ok := defaultRoleTimeouts[role]; ok {
                return d
            }
            return cfg.Timeout
        }
  - NAMING: ResolveRoleTimeout (exported; PascalCase; matches ResolveRoleModel). Params (role, cfg) match.
  - NO IMPORT beyond `time` (Task 1). DEPENDENCIES: RoleConfig.Timeout + Config.Timeout (both LANDED).

Task 4: EDIT internal/config/roles_test.go — add `import "time"` + 6 tests
  - The file imports only `"testing"` today. Change to:
        import (
            "testing"
            "time"
        )
  - CLONE the ResolveRoleModel idiom (cfg := Defaults(); mutate; cfg.Roles = map[...]; call; assert). 6 tests:
  - TEST A — TestResolveRoleTimeout_PerRoleOverride (clone FullOverride @32):
        cfg := Defaults(); cfg.Timeout = 120 * time.Second
        cfg.Roles = map[string]RoleConfig{"planner": {Timeout: 600 * time.Second}}
        got := ResolveRoleTimeout("planner", cfg)
        // assert got == 600*time.Second (per-role beats built-in 480s AND global 120s)
  - TEST B — TestResolveRoleTimeout_PlannerBuiltinBeatsGlobal (THE key test — new):
        cfg := Defaults(); cfg.Timeout = 120 * time.Second // DISTINCT from 480s built-in
        // Roles nil ⇒ no override; planner takes 480s BUILT-IN, NOT 120s global
        got := ResolveRoleTimeout("planner", cfg)
        // assert got == 480*time.Second
  - TEST C — TestResolveRoleTimeout_NonPlannerGlobalFallback (clone GlobalFallback @5):
        cfg := Defaults(); cfg.Timeout = 120 * time.Second
        for _, role := range []string{"stager", "message", "arbiter"} {
            got := ResolveRoleTimeout(role, cfg)
            // assert got == 120*time.Second (no built-in → global)
        }
  - TEST D — TestResolveRoleTimeout_FieldMergeTimeoutOnly (clone ModelOnlyOverride @47):
        cfg := Defaults(); cfg.Timeout = 120 * time.Second
        cfg.Roles = map[string]RoleConfig{"message": {Provider: "pi", Timeout: 0}} // Timeout 0 ⇒ inherit
        got := ResolveRoleTimeout("message", cfg)
        // assert got == 120*time.Second (Timeout 0 inherits global; Provider is irrelevant to timeout)
  - TEST E — TestResolveRoleTimeout_UnknownRoleGlobalFallback (clone UnknownRole @89):
        cfg := Defaults(); cfg.Timeout = 120 * time.Second
        got := ResolveRoleTimeout("palnner", cfg) // typo
        // assert got == 120*time.Second (no entry, no built-in → global)
  - TEST F — TestResolveRoleTimeout_RolesNilGlobalFallback (clone GlobalFallbackRolesNil @5):
        cfg := Defaults(); cfg.Timeout = 120 * time.Second // Roles is nil from Defaults()
        got := ResolveRoleTimeout("message", cfg)
        // assert got == 120*time.Second
  - COVERAGE: per-role override; built-in-beats-global (planner); non-planner global fallback; field-merge
    (Timeout 0 inherits); unknown role; Roles-nil. Every test sets cfg.Timeout = 120s (≠ 480s built-in).
  - DEPENDENCIES: Task 3.

Task 5: VERIFY — build (native+cross), vet, format, focused + full tests, lint, coverage, grep guards
  - go build ./... ; GOOS=windows go build ./... ; GOOS=linux go build ./...
  - go vet ./internal/config/...
  - gofmt -l internal/config/roles.go internal/config/roles_test.go   # must be empty
  - go test ./internal/config/ -run 'ResolveRoleTimeout' -v
  - go test ./internal/config/ -run 'ResolveRoleModel' -v   # 12 existing tests stay green
  - make test ; make lint ; make coverage-gate
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the 3-tier accessor (mirrors ResolveRoleModel + a built-in tier between per-role and global)
func ResolveRoleTimeout(role string, cfg Config) time.Duration {
	if rc, ok := cfg.Roles[role]; ok && rc.Timeout != 0 { // tier 1: per-role override (non-zero)
		return rc.Timeout
	}
	if d, ok := defaultRoleTimeouts[role]; ok { // tier 2: built-in role default (planner=480s)
		return d
	}
	return cfg.Timeout // tier 3: global fallback
}

// PATTERN: the disambiguating test (THE key semantic — built-in beats global for the planner)
func TestResolveRoleTimeout_PlannerBuiltinBeatsGlobal(t *testing.T) {
	cfg := Defaults()
	cfg.Timeout = 120 * time.Second // DISTINCT from the 480s built-in so the assertion is unambiguous
	got := ResolveRoleTimeout("planner", cfg)
	if got != 480*time.Second {
		t.Errorf("ResolveRoleTimeout(planner) = %v, want 480s (built-in beats 120s global)", got)
	}
}
```

### Integration Points

```yaml
NO database / config-layer / routes / new types / new dependency. One plain function + one map + tests.

CONFIG PACKAGE (internal/config/roles.go):
  - +import "time" (the file has no import block today).
  - +var defaultRoleTimeouts = map[string]time.Duration{"planner": 480*time.Second}.
  - +func ResolveRoleTimeout(role string, cfg Config) time.Duration.

PRECEDENCE (resolved by Load, unchanged model — this function only READS it):
  per-role override (cfg.Roles[role].Timeout, non-zero) > built-in role default (planner 480s) > global (cfg.Timeout)
  - The built-in tier is the ONE structural addition vs ResolveRoleModel (which has only per-role > global).

DOWNSTREAM (this subtask ENABLES but does NOT build):
  - P1.M3.T1.S1/T2 (single-commit path): generate.go/multiturn.go/workdesc.go/hook resolve the MESSAGE-role
    timeout via ResolveRoleTimeout("message", cfg) at each Execute site.
  - P1.M3.T2.S1 (decompose path): planner.go/stager.go/message.go/arbiter.go resolve per-role timeouts.
  - ZERO production callers after this subtask — only roles_test.go calls ResolveRoleTimeout (expected).

SCOPE FENCES: NO Defaults().Timeout change (P1.M2.T2.S1); NO Execute-site consumer (P1.M3); NO new config
  layer (P1.M1 done); NO role_defaults.go edit (different axis); NO *time.Duration (plain duration, T1.S1);
  NO docs (P1.M4.T2).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native + cross build (plain function, no platform tag — must build everywhere).
go build ./...
GOOS=windows go build ./...
GOOS=linux   go build ./...
# Expected: all clean. A native failure usually means you forgot `import "time"` (Task 1).

# Vet.
go vet ./internal/config/...

# Format.
gofmt -l internal/config/roles.go internal/config/roles_test.go
# Expected: empty. If listed: gofmt -w the file(s).

# Lint.
make lint      # golangci-lint v1.61 (staticcheck/gosimple/govet/errcheck/ineffassign/unused)
# Expected: zero errors. `unused` stays clean because the new tests read ResolveRoleTimeout + the map.

# Scope guard: only roles.go + roles_test.go changed.
git diff --name-only
# Expected: internal/config/roles.go  internal/config/roles_test.go  (exactly these 2).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The 6 new tests (focused).
go test ./internal/config/ -run 'ResolveRoleTimeout' -v
# Expected: PASS — per-role override; planner built-in beats global; non-planner global fallback;
#           field-merge (Timeout 0); unknown role; Roles-nil.

# Regression: the 12 ResolveRoleModel tests stay green (the copy source is untouched behaviorally).
go test ./internal/config/ -run 'ResolveRoleModel' -v

# Full config package + full race suite.
go test ./internal/config/ -v
make test
# Expected: green (race detector).

# Coverage gate (PRD §20.3: ≥85% on internal/{git,provider,generate,config}).
make coverage-gate
# Expected: passes (the new function + map + tests ADD coverage).
```

### Level 3: Integration Testing (System Validation)

```bash
# There is no integration/e2e surface for this task — ResolveRoleTimeout has no production caller yet
# (P1.M3 wires the Execute sites). The unit tests ARE the contract. A full e2e (per-role timeout actually
# bounds a planner run) is the deliverable of P1.M4.T1.S1, NOT this subtask.

# Sanity: the package still builds into the binary (no downstream compile break from the new symbol).
make build
# Expected: succeeds.

# Behavioral proof that precedence is correct, expressed as a one-shot test program (optional, for confidence):
# (the unit tests already assert this — included only as a manual confidence check)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Scope guard 1: exactly one ResolveRoleTimeout definition, in roles.go.
grep -rn 'func ResolveRoleTimeout' internal/config/
# Expected: one hit — internal/config/roles.go.

# Scope guard 2: the map has EXACTLY the planner entry (no accidental stager/message/arbiter built-ins).
grep -n 'defaultRoleTimeouts' internal/config/roles.go
# Expected: the var decl + the reader in ResolveRoleTimeout. Inspect the map literal has ONLY "planner".

# Scope guard 3: ZERO production callers (consumer is P1.M3) — only the tests call it.
grep -rn 'ResolveRoleTimeout(' --include='*.go' internal/ cmd/ pkg/ | grep -v '_test.go'
# Expected: EMPTY (no hits outside tests). A hit in internal/decompose or internal/generate = out of scope.

# Scope guard 4: Defaults().Timeout UNCHANGED (still 480s — P1.M2.T2 owns the flip).
grep -n 'Timeout:.*480 \* time.Second' internal/config/config.go
# Expected: 1 hit (Defaults). And NO '120 \* time.Second' Timeout line added in config.go by this task.

# Scope guard 5: roles.go imported "time" (Task 1 landed).
grep -n 'import "time"\|"time"' internal/config/roles.go
# Expected: the `import "time"` line (the file had no import block before).

# Scope guard 6: role_defaults.go UNTOUCHED (the map is NOT in the provider×role table file).
git diff --name-only | grep role_defaults.go
# Expected: empty (role_defaults.go not modified).

# Scope guard 7: ResolveRoleModel (the copy source) behaviorally untouched.
go test ./internal/config/ -run 'ResolveRoleModel' -v
# Expected: all 12 pre-existing tests PASS.

# Precedence semantic guard: the disambiguation tests set cfg.Timeout=120s (proves they don't rely on the 480s default).
grep -n 'cfg.Timeout = 120 \* time.Second' internal/config/roles_test.go
# Expected: ≥1 hit per planner/global-fallback test (the disambiguation rule from findings §5).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `GOOS=windows/linux go build ./...` clean
- [ ] `go vet ./internal/config/...` clean
- [ ] `gofmt -l internal/config/roles.go internal/config/roles_test.go` empty
- [ ] `make lint` zero errors (`unused` clean — tests read the new symbol)
- [ ] `make test` (race) green, incl. the 6 new tests
- [ ] `make coverage-gate` ≥85% on `internal/config`

### Feature Validation
- [ ] planner (no override, cfg.Timeout≠480s) → 480s (built-in beats global — THE key semantic)
- [ ] stager/message/arbiter (no override) → cfg.Timeout (no built-in)
- [ ] any role with non-zero cfg.Roles[role].Timeout → that value (override beats built-in + global)
- [ ] unknown/non-canonical role → cfg.Timeout
- [ ] 6 new tests pass; 12 ResolveRoleModel tests stay green

### Scope-Boundary Validation
- [ ] `git diff --name-only` == only {roles.go, roles_test.go}
- [ ] `Defaults().Timeout` UNCHANGED (still 480s; P1.M2.T2.S1 owns the 120s flip)
- [ ] ZERO production `ResolveRoleTimeout(` callers (only roles_test.go)
- [ ] NO change to role_defaults.go, load.go, file.go, git.go, config.go, or any decompose/generate file
- [ ] NO new config-loading layer (P1.M1 is done; this is pure read-side)
- [ ] NO docs change (P1.M4.T2)

### Code Quality & Docs
- [ ] `import "time"` added to roles.go (it had no import block)
- [ ] `defaultRoleTimeouts` placed in roles.go (NOT role_defaults.go — different axis)
- [ ] ResolveRoleTimeout's godoc cites FR-R7 and documents the 3-tier precedence
- [ ] Tests clone the ResolveRoleModel idiom and disambiguate with cfg.Timeout=120s

---

## Anti-Patterns to Avoid

- ❌ Don't forget `import "time"` in roles.go. The file has NO import block today (it's `package config`
  then directly the ResolveRoleModel godoc). ResolveRoleTimeout + the map reference `time.Duration`, so
  the import is mandatory — omitting it is a compile error. (roles_test.go also needs `"time"` added.)
- ❌ Don't invert the precedence. The planner's 480s built-in BEATS the global — order is per-role
  override > built-in (planner 480s) > global. Putting the global above the built-in makes the planner
  default meaningless (planner would get cfg.Timeout, not 480s). The 3rd tier is the WHOLE point of FR-R7.
- ❌ Don't write the planner test against `Defaults()` without overriding `cfg.Timeout`. `Defaults().Timeout`
  is 480s today (== the built-in), so the planner-built-in-vs-global assertion would be ambiguous and
  would pass even if precedence is inverted. Set `cfg.Timeout = 120*time.Second` (≠ 480s) in every
  planner/global-fallback test.
- ❌ Don't add stager/message/arbiter to `defaultRoleTimeouts`. Only the planner has a shipped built-in
  (480s); the other three inherit `cfg.Timeout`. Adding them would silently change their behavior and
  conflict with P1.M2.T2's global-120s default.
- ❌ Don't place the map in `role_defaults.go`. That file is the FR-D4 provider×role→MODEL table — a
  different axis. The planner 480s is role×timeout; keep it in roles.go next to its sole reader.
- ❌ Don't change `Defaults().Timeout` (config.go:197). It stays 480s in THIS task. The 480s→120s flip is
  P1.M2.T2.S1 (it also fixes the pinning tests). Coupling the two would explode this task's blast radius.
- ❌ Don't wire any consumer (don't touch planner.go/stager.go/message.go/arbiter.go/generate.go/
  multiturn.go/workdesc.go). The 13 Execute call sites are P1.M3. After this task grep must show
  ResolveRoleTimeout called ONLY in roles_test.go.
- ❌ Don't promote RoleConfig.Timeout to `*time.Duration`. It's a plain `time.Duration` (0 ⇒ inherit),
  decided in P1.M1.T1.S1 — mirror it. The `!= 0` overlay guard and the `rc.Timeout != 0` check are the
  sentinel mechanism; a pointer would be a cross-cutting change.
- ❌ Don't collapse a 0 `cfg.Timeout` into the built-in for non-planner roles. The built-in is
  role-specific (planner-only). ResolveRoleTimeout returns `cfg.Timeout` verbatim for non-planner roles;
  guarding a 0 at the Execute site is the consumer's (P1.M3) job, not this function's.
- ❌ Don't anchor placement to line numbers in roles.go — place ResolveRoleTimeout "immediately after
  ResolveRoleModel" and the map "just above it" by NAME. The parallel sibling doesn't touch this file,
  but the adjacency anchor is reviewable and drift-safe regardless.

---

## Confidence Score: 10/10

This is a single plain function that clones a proven, fully-tested accessor (`ResolveRoleModel`) in the
same file, plus a one-entry map and 6 tests that clone an existing 12-test idiom. The verbatim function
body, godoc, and map are all spelled out; the one structural addition (the 3rd built-in tier) is the
FR-R7 requirement and is pinned by a disambiguating test; the two non-obvious gotchas (roles.go has no
import block → add `time`; tests must set cfg.Timeout ≠ 480s to disambiguate) are explicitly fenced; and
the scope is tightly bounded (no consumer, no default change, no docs, no other file). No new pattern, no
new type, no new dependency. One-pass success is essentially guaranteed.
