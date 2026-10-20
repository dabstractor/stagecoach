name: "P1.M1.T1.S1 — config core + role resolution: IdleTimeout fields, 600s default, ResolveRoleDeadlines, planner 480s deleted"
description: >
  The config-core half of the two-deadline model (§9.30/FR-R7): add Config.IdleTimeout + RoleConfig.IdleTimeout
  (0 ⇒ inherit; toml "idle_timeout"), change the global hard-cap default 120s→600s and seed IdleTimeout 60s,
  DELETE the defaultRoleTimeouts map (planner 480s built-in retired — no role ships a built-in deadline),
  add exported ResolveRoleDeadlines(role, cfg) (idle, hard) with the per-role→global ladder (no built-in tier,
  both deadlines resolve independently), and keep ResolveRoleTimeout as a thin hard-cap accessor delegating
  to it (11 callers keep compiling). Plus the complete test break-list fix + new ladder tests. Pure config
  code — no layers, no flags, no executor (S2/S3/P1.M2 own those).

---

## Goal

**Feature Goal**: Establish the two-deadline configuration core (idle = primary productivity signal, 60s
default; hard cap = absolute backstop, 600s default) with per-role resolution on the FR-R3 ladder minus
the built-in tier, retiring the v2.8 planner-480s special case as moot.

**Deliverable**: (1) `Config.IdleTimeout` / `RoleConfig.IdleTimeout` fields (0 ⇒ inherit global);
(2) `Defaults()` seeds `IdleTimeout: 60s` + `Timeout: 600s` (was 120s); (3) `defaultRoleTimeouts` deleted;
(4) exported `ResolveRoleDeadlines(role, cfg) (idle, hard time.Duration)`; (5) `ResolveRoleTimeout` kept
as a thin hard-cap accessor delegating to it; (6) updated doc comments; (7) the full test break-list fixed
+ new ladder tests.

**Success Definition**:
- `Defaults().Timeout == 600s`, `Defaults().IdleTimeout == 60s`.
- `ResolveRoleDeadlines` returns per-role overrides when non-zero, else the globals — each deadline
  INDEPENDENTLY; unknown role / nil Roles / zero-value fields all inherit the globals.
- NO role (incl. planner) resolves to a built-in deadline; planner inherits the global like every role.
- All 11 ResolveRoleTimeout callers compile unchanged (thin delegation).
- `go build ./...`, `go test ./internal/config/... ./internal/decompose/... ./internal/generate/...`,
  `make test`, `make lint` pass (the break-list tests updated, everything else untouched).

## Why

- **§9.30 / FR25 / FR-R7 (P0, → G30)**: a wall clock alone cannot distinguish a hung agent from a
  productive-but-slow one. The v1–v3.4 elapsed-time-only kill (default 120s) is superseded by an idle
  deadline (default 60s) as the PRIMARY productivity signal with a hard cap (600s) as the backstop. Each
  role resolves both independently.
- **The planner 480s built-in is retired as moot**: it existed because a 120s global starved a thinking
  planner; under idle+600s every role is already granted as much time as it stays productive. This
  subtask is the config foundation S2 (precedence layers) and P1.M2.T1.S3 (call-site threading) build on.

## What

**User-visible behavior**: None yet (config core; the executor still uses the old path until P1.M2).

**Technical change (config.go + roles.go + their tests only):**
- Add IdleTimeout fields; reseed defaults; delete the built-in map + branch; add ResolveRoleDeadlines;
  thin-delegate ResolveRoleTimeout; update docs; fix the break-list + add ladder tests.

### Success Criteria
- [ ] `Config.IdleTimeout` (toml "idle_timeout") + `RoleConfig.IdleTimeout` (0 ⇒ inherit) added
- [ ] `Defaults()`: `IdleTimeout: 60 * time.Second`, `Timeout: 600 * time.Second`; doc list updated
- [ ] `defaultRoleTimeouts` deleted; ResolveRoleTimeout has NO built-in branch
- [ ] `ResolveRoleDeadlines(role, cfg) (idle, hard)` exported; deadlines resolve independently
- [ ] `ResolveRoleTimeout` = thin hard accessor (delegates to ResolveRoleDeadlines); 11 callers unchanged
- [ ] Break-list tests updated; new Defaults/deadline-ladder tests added
- [ ] `go build ./...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact field/default line numbers, the verbatim new function, the delegation design (and why ResolveRoleTimeout must be KEPT), the COMPLETE verified break-list (which tests break vs. which merely set 120s explicitly), the compile-safety rationale for the field additions, and the scope boundaries are all enumerated below.

### Documentation & References

```yaml
- file: internal/config/config.go
  why: "Config.Timeout field L85 ('; Defaults: 120s' comment to rewrite); Defaults() doc list L210; seed L224; RoleConfig Timeout L42 (the '0 ⇒ inherit' comment to mirror); HookTimeout (~L170/~L229) is the structural precedent for a new duration field."
  pattern: "Field: IdleTimeout immediately after L85 with a '; Defaults: 60s' + two-deadline-contract comment. Seed: 'IdleTimeout: 60 * time.Second,' immediately after L224. Doc list L210: 'timeout 120s,' → 'idle_timeout 60s, timeout 600s,'."
  gotcha: "time is already imported — NO new imports. RoleConfig is NEVER decoded from TOML (Config.Roles is toml:\"-\"), so adding the field is compile-safe (all literals are named-field)."

- file: internal/config/roles.go
  why: "defaultRoleTimeouts L5-13 (DELETE); ResolveRoleTimeout doc L70-95 (rewrite — drop all planner-480s prose) + body L96-104 (delete the built-in branch L100-102); the home for ResolveRoleDeadlines."
  pattern: "ResolveRoleDeadlines mirrors the existing ladder MINUS the built-in tier; both deadlines resolve independently; ResolveRoleTimeout delegates (_, hard := ResolveRoleDeadlines(...); return hard) so the 11 callers keep compiling."
  gotcha: "Do NOT delete ResolveRoleTimeout — it has 11 production callers (arbiter.go:83, message.go:145, planner.go:69, stager.go:98, generate.go:303, multiturn.go:153, workdesc.go:71, hook/exec.go:167). Deleting it breaks compile far outside S1's scope."

- file: internal/config/roles_test.go (the break-list)
  why: "TestResolveRoleTimeout_PlannerBuiltinBeatsGlobal (207-215) and _PlannerDefaultFromDefaults (304-311) BREAK (they assert the 480s built-in + the 120s default) — REWRITE. Comments at 199/209/210/219/274/279/281 cite the built-in — update prose only (assertions pass). Tests at :217/:228/:238/:247/:261/:277 set 120s EXPLICITLY — PASS unchanged."
- file: internal/config/config_test.go (L20-21) + load_test.go (L712-713)
  why: "Pin Defaults().Timeout==120s / resolved default==120s — BREAK; update to 600s (config_test also gains an IdleTimeout==60s assert). load_test.go:104-105 (parseTimeout table) and file_test.go:700/731 (explicit arbiter 120s) do NOT break."
  gotcha: "file_test.go:898 (dst.Timeout != 120*time.Second) — verify the construction: explicit 120s passes; Defaults()-derived breaks → update to 600s."

- docfile: plan/022_856008a8cbb5/architecture/config_ladder.md
  why: "The verified line-for-line map of every timeout surface (§1 config.go, §2 roles.go) + the slot plan for IdleTimeout + the HookTimeout precedent."
- docfile: plan/022_856008a8cbb5/P1M1T1S1/research/verification_deltas.md
  why: "The verified anchors, the verbatim new function, the COMPLETE break-list table (breaks vs. explicit-120s), the compile-safety rationale, and the scope boundaries. READ THIS before editing."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  config.go        # +IdleTimeout field (after L85) + seed (after L224); Timeout default 120→600; doc comments
  roles.go         # -defaultRoleTimeouts; +ResolveRoleDeadlines; ResolveRoleTimeout → thin delegate; doc comments
  config_test.go   # Defaults pins 120s→600s (+ IdleTimeout 60s)
  roles_test.go    # rewrite 2 planner-480s tests; comment updates; +new ResolveRoleDeadlines tests
  load_test.go     # default-Timeout pin 120s→600s (file_test.go:898 — verify)
```

### Known Gotchas of our Codebase & Library Quirks

```go
// CRITICAL (keep ResolveRoleTimeout): it has 11 production callers. Deleting it breaks compile across
//   decompose/generate/hook. Keep it as a thin hard-cap accessor DELEGATING to ResolveRoleDeadlines
//   (_, hard := ResolveRoleDeadlines(role, cfg); return hard) — single source of truth, callers intact.

// CRITICAL (the break-list is bigger than the obvious): changing 120s→600s breaks not just
//   config_test.go:20 but ALSO load_test.go:712-713 (resolved-default pin) and BOTH planner-480s tests
//   in roles_test.go (207-215, 304-311). Meanwhile roles_test.go:217/:228/:238/:247/:261/:277 and
//   file_test.go:700/731 set 120s EXPLICITLY as a test value and do NOT break — do not touch them
//   beyond the cited stale comments. Run the full config suite to confirm.

// GOTCHA (independent resolution): the two deadlines resolve INDEPENDENTLY on the same ladder — a role
//   may override only IdleTimeout (hard stays global) or only Timeout (idle stays global). The new
//   tests must assert both directions.

// GOTCHA (defaults are the globals): ResolveRoleDeadlines' fallbacks are cfg.IdleTimeout/cfg.Timeout —
//   Defaults() seeds them 60s/600s so they're non-zero in practice; a zero-value cfg keeps the existing
//   leniency (returns 0s, no error — same as ResolveRoleTimeout today).

// GOTCHA (toml tags on RoleConfig are cosmetic): Config.Roles is toml:"-", so RoleConfig.IdleTimeout's
//   tag is documentation only; the FUNCTIONAL decode (fileRoleConfig) is S2's job. Do not add
//   fileRoleConfig.IdleTimeout or overlay guards in S1.

// SCOPE: S1 is config.go + roles.go + their tests ONLY. Do NOT touch load.go/file.go/git.go (S2 env/git/
//   file keys), root.go/bootstrap.go (S3 flags + template), or the executor (P1.M2). Do NOT bump
//   CurrentConfigVersion (additive optional keys never bump — FR-B4).
```

## Implementation Blueprint

### Data models and structure

No new types. Two new `time.Duration` fields + one new exported function (two return values, per the
task contract) + one deletion + one thin delegation. No new imports.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/config/config.go — IdleTimeout fields + 600s default
  - L85 area: change the Timeout trailing comment to the two-deadline contract + "Defaults: 600s";
    ADD immediately after: IdleTimeout time.Duration `toml:"idle_timeout"` // idle deadline...primary
    productivity signal (§9.30); Defaults: 60s
  - L224: Timeout: 120 * time.Second → 600 * time.Second; ADD immediately after:
    IdleTimeout: 60 * time.Second,
  - L210 doc list: "timeout 120s," → "idle_timeout 60s, timeout 600s," (+ mention the planner-480s
    retirement in the Defaults() doc if space allows).
  - RoleConfig: ADD IdleTimeout immediately after L42, mirroring the existing '0 ⇒ inherit' comment.
  - DEPENDENCIES: none.

Task 2: MODIFY internal/config/roles.go — delete the built-in, add ResolveRoleDeadlines, thin-delegate
  - DELETE defaultRoleTimeouts (L5-13, doc + var) entirely.
  - ADD ResolveRoleDeadlines (place above ResolveRoleTimeout):
        // ResolveRoleDeadlines returns the (idle, hard) generation deadlines for a single agent role
        // (PRD §9.30 FR-R7/FR25): per-role override (cfg.Roles[role], 0 ⇒ inherit) → the global
        // defaults (cfg.IdleTimeout / cfg.Timeout; Defaults() seeds 60s/600s). The two deadlines
        // resolve INDEPENDENTLY — a role may override either alone. NO role ships a built-in deadline
        // (the v2.8 planner 480s wall clock is retired: under an idle deadline + 600s hard cap every
        // role is granted as much time as it stays productive). role is an arbitrary string (canonical:
        // planner/stager/message/arbiter); a non-canonical name or absent Roles entry inherits both
        // globals — same leniency as ResolveRoleModel.
        func ResolveRoleDeadlines(role string, cfg Config) (idle, hard time.Duration) {
            idle, hard = cfg.IdleTimeout, cfg.Timeout
            if rc, ok := cfg.Roles[role]; ok {
                if rc.IdleTimeout != 0 { idle = rc.IdleTimeout }
                if rc.Timeout != 0 { hard = rc.Timeout }
            }
            return idle, hard
        }
  - REWRITE ResolveRoleTimeout (body + doc): thin hard-cap accessor delegating to ResolveRoleDeadlines;
    doc drops ALL planner-480s/built-in prose (ladder = per-role override → global; cites FR-R7 §9.30;
    kept for the 11 existing callers — new code prefers ResolveRoleDeadlines).
  - DEPENDENCIES: Task 1 (the IdleTimeout fields).

Task 3: FIX the break-list tests
  - config_test.go L20-21: 120s → 600s; ADD an IdleTimeout == 60*time.Second assertion alongside.
  - load_test.go L712-713: want 120s (default) → 600s.
  - roles_test.go TestResolveRoleTimeout_PlannerBuiltinBeatsGlobal (207-215): REWRITE — the planner now
    inherits the global (repurpose e.g. TestResolveRoleTimeout_PlannerInheritsGlobal: cfg.Timeout=120s
    explicit, planner → 120s; the override-beats-global half at 199-206 stays valid as-is).
  - roles_test.go TestResolveRoleTimeout_PlannerDefaultFromDefaults (304-311): REWRITE —
    Defaults().Timeout==600s; ResolveRoleTimeout("planner", Defaults())==600s (inherits; no built-in).
  - roles_test.go stale comments (199/209/210/219/274/279/281 citing 480s): prose updates only.
  - file_test.go:898: VERIFY construction — explicit 120s passes (leave); Defaults()-derived → 600s.
  - DEPENDENCIES: Tasks 1-2.

Task 4: ADD the new tests (task-specified ladder coverage)
  - config_test.go TestDefaults: Timeout==600s AND IdleTimeout==60s.
  - roles_test.go TestResolveRoleDeadlines (table-driven):
      * per-role BOTH overridden (idle=5s, hard=50s) → (5s, 50s)
      * per-role idle ONLY → (override, global hard)
      * per-role hard ONLY → (global idle, override)
      * zero-value RoleConfig fields → both globals
      * unknown role / nil Roles → both globals
      * independent-resolution assertion (the idle-only and hard-only rows)
  - TestResolveRoleTimeout_Delegates: ResolveRoleTimeout(r, cfg) == hard return of ResolveRoleDeadlines
    (for a couple of rows) — pins the delegation contract.
  - DEPENDENCIES: Tasks 1-2.

Task 5: VERIFY — full green
  - RUN: go test ./internal/config/... -v  (EXPECT green)
  - RUN: go test ./internal/decompose/... ./internal/generate/... ./internal/hook/...  (the ResolveRoleTimeout
    caller packages — EXPECT green, no caller edits)
  - RUN: grep -rn "defaultRoleTimeouts\|480" internal/config/roles.go → zero hits (retirement complete;
    note 480 may legitimately appear in OTHER files' comments — only roles.go must be clean).
  - DEPENDENCIES: Tasks 1-4.
```

### Implementation Patterns & Key Details

```go
// PATTERN: ResolveRoleDeadlines — the ladder minus the built-in tier, deadlines independent
func ResolveRoleDeadlines(role string, cfg Config) (idle, hard time.Duration) {
	idle, hard = cfg.IdleTimeout, cfg.Timeout
	if rc, ok := cfg.Roles[role]; ok {
		if rc.IdleTimeout != 0 { idle = rc.IdleTimeout }
		if rc.Timeout != 0     { hard = rc.Timeout }
	}
	return idle, hard
}

// PATTERN: the thin hard-cap accessor (11 callers keep compiling)
func ResolveRoleTimeout(role string, cfg Config) time.Duration {
	_, hard := ResolveRoleDeadlines(role, cfg)
	return hard
}
```

### Integration Points

```yaml
NO layer / flag / executor / public-behavior changes. Config core + role resolution only.

CODE:
  - internal/config/config.go — +Config.IdleTimeout, +RoleConfig.IdleTimeout; Timeout default 120s→600s; docs
  - internal/config/roles.go — -defaultRoleTimeouts; +ResolveRoleDeadlines; ResolveRoleTimeout → thin delegate
TESTS:
  - config_test.go / roles_test.go / load_test.go — break-list fixes + new ladder tests

DOWNSTREAM (consumes this core — do NOT implement in S1):
  - P1.M1.T1.S2: STAGECOACH_IDLE_TIMEOUT env, stagecoach.idleTimeout git key, [defaults].idle_timeout /
    [role.X].idle_timeout file keys (fileRoleConfig + materialize/overlay guards)
  - P1.M1.T1.S3: --idle-timeout + per-role flags, bootstrap template, docs
  - P1.M2.T1.S2/S3: the Deadlines struct in Execute + threading the 13 call sites to ResolveRoleDeadlines

UNCHANGED: load.go/file.go/git.go (S2); root.go/bootstrap.go (S3); the executor (P1.M2);
  CurrentConfigVersion (additive optional keys never bump — FR-B4); the 11 ResolveRoleTimeout callers.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...
go vet ./...
gofmt -l internal/config/
# Expected: empty.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Config package — the break-list fixes + new tests
go test ./internal/config/... -v
# Expected: ALL pass — TestDefaults (600s/60s), the rewritten planner tests, TestResolveRoleDeadlines,
#           the delegation test, and every previously-passing explicit-120s test.

# The ResolveRoleTimeout caller packages (must be green with ZERO caller edits)
go test ./internal/decompose/... ./internal/generate/... ./internal/hook/... 

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# (S1 is pure config core — the two-deadline behavior is not yet exercised end-to-end until P1.M2's
#  executor rework lands. The unit ladder tests are the within-scope proof. A quick sanity: the resolved
#  config still loads —)
go test ./internal/config/ -run 'TestLoad|TestDefaults' -v
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the retirement is complete in roles.go
grep -n "defaultRoleTimeouts\|480" internal/config/roles.go
# Expected: ZERO hits (no built-in map, no 480s anywhere in roles.go).

# Grep guard: the new surface exists
grep -n "IdleTimeout\|ResolveRoleDeadlines" internal/config/config.go internal/config/roles.go
# Expected: the field, the seed, and the function present.

# Grep guard: the 600s/60s defaults
grep -n "600 \* time.Second\|60 \* time.Second" internal/config/config.go
# Expected: both seeds present; NO remaining "120 \* time.Second" as a DEFAULT (explicit test values elsewhere are fine).

# Scope-boundary guard: only config.go/roles.go + tests changed
git diff --name-only
# Expected: internal/config/{config.go,roles.go,config_test.go,roles_test.go,load_test.go} (+ file_test.go only if :898 needed the 600s fix).

# Caller-intactness guard: the 11 ResolveRoleTimeout call sites untouched
git diff --stat internal/decompose/ internal/generate/ internal/hook/
# Expected: empty.

# Version-stability guard: no config-version bump
git diff internal/config/config.go | grep CurrentConfigVersion
# Expected: empty (additive optional keys never bump — FR-B4).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/config/` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass

### Feature Validation
- [ ] `Config.IdleTimeout` + `RoleConfig.IdleTimeout` present (0 ⇒ inherit; toml "idle_timeout")
- [ ] `Defaults()`: Timeout==600s, IdleTimeout==60s; doc list updated
- [ ] `defaultRoleTimeouts` deleted; no built-in branch anywhere
- [ ] `ResolveRoleDeadlines` exported; deadlines resolve independently; per-role → global ladder
- [ ] `ResolveRoleTimeout` thin-delegates; planner resolves to the global (no 480s)
- [ ] Break-list tests fixed; new ladder + delegation tests pass

### Scope-Boundary Validation
- [ ] load.go / file.go / git.go UNCHANGED (S2)
- [ ] root.go / bootstrap.go UNCHANGED (S3)
- [ ] The 11 ResolveRoleTimeout callers UNCHANGED (P1.M2.T1.S3 threads them)
- [ ] CurrentConfigVersion UNCHANGED (FR-B4: additive optional keys never bump)
- [ ] No executor / activity-infra changes (P1.M2)

### Code Quality
- [ ] Doc comments state the two-deadline contract (idle = primary signal, hard = backstop, §9.30)
- [ ] The retirement rationale recorded (planner 480s was a 120s-starvation workaround; moot under idle+600s)
- [ ] Delegation single-sources the ladder (ResolveRoleTimeout has no independent copy of the logic)

---

## Anti-Patterns to Avoid

- ❌ Don't delete ResolveRoleTimeout — it has 11 production callers; deleting it breaks compile across three packages. Keep it as a thin delegate; P1.M2.T1.S3 migrates callers to ResolveRoleDeadlines.
- ❌ Don't return a struct from ResolveRoleDeadlines — the task contract (and the OUTPUT section) specifies `(idle, hard time.Duration)` two-value returns, consumed as-is by S2/S3/P1.M2.
- ❌ Don't couple the two deadlines — they resolve INDEPENDENTLY (a role may override idle alone). The new tests must pin both directions.
- ❌ Don't touch the explicit-120s tests — roles_test.go :217/:228/:238/:247/:261/:277 and file_test.go:700/731 construct 120s as an explicit TEST VALUE; only the default-pinning tests (config_test:20, load_test:712) and the two planner-480s tests break. Over-updating those would weaken the suite.
- ❌ Don't add fileRoleConfig.IdleTimeout or overlay guards in S1 — the file layer is S2. A file-layer idle_timeout simply doesn't merge yet (deliberate split).
- ❌ Don't bump CurrentConfigVersion — FR-B4: additive/optional keys (idle_timeout) must not bump the version or emit an advisory.
- ❌ Don't keep any planner-480s prose in roles.go's doc comments — the retirement must be complete there (grep gate).
- ❌ Don't seed the defaults anywhere but Defaults() — everything funnels through it (Layer 1); duplicating 60s/600s elsewhere invites drift.

---

## Confidence Score: 9/10

One-pass success is very high: every line number verified, the new function given verbatim, the
delegation design resolving the 11-caller compile hazard, and — most importantly — the COMPLETE
break-list enumerated and cross-checked (which tests break vs. which merely set 120s explicitly,
including the file_test.go:898 verify-flag). The -1 is for that one unverified context
(file_test.go:898) plus the possibility of an overlooked 120s pin outside the greppable patterns
(e.g. a prose assertion); mitigated by running the full config + caller-package suites as the gate
and the scope-boundary git-diff guard.
