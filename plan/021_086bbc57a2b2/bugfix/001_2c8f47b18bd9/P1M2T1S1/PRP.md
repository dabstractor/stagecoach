name: "P1.M2.T1.S1 — Defer the stager requirement in ResolveRoles: return success + StagerAvailable sentinel instead of erroring (BUG-003, FR-M13)"
description: >
  Fix BUG-003 (Major): ResolveRoles (internal/decompose/roles.go:132) hard-errors when the stager has
  no TooledFlags and no installed tooled fallback — blocking Decompose BEFORE the planner runs, making
  the FR-M13 file-disjoint fast-path (runLoopFastPath, which needs NO stager) unreachable for no-tooled
  providers. S1 fix (this task): in ResolveRoles, when role=='stager' && len(m.TooledFlags)==0 &&
  FirstTooledProvider(installed)=='' , do NOT error — set a `StagerAvailable=false` sentinel and return
  success (carrying the no-tooled manifest in rm.Stager for diagnostics). Add `StagerAvailable bool` to
  RoleManifests (flows deps.Roles.StagerAvailable via the whole-struct `Roles: roleManifests` in both
  callers — NO caller change). Wrap the existing fallback logic in `else` (skipped when no fallback).
  The deferred "stager needed but unavailable" error is S2's runLoop check (separate subtask); extending
  FirstTooledProvider is S3. Flip the existing TestResolveRoles_NoStagerCapable (which ASSERTS THE BUG)
  to expect success + StagerAvailable=false. NO runLoop/invokeStager (S2), NO FirstTooledProvider (S3),
  NO callers, NO docs (Mode A = the code comment). Stdlib-only; no new deps.

---

## Goal

**Feature Goal**: Let a no-tooled provider (opencode, or any user-defined provider) reach Decompose's
FR-M13 file-disjoint fast-path by removing the eager stager-resolution hard-error from `ResolveRoles`.
Resolution now succeeds with a `StagerAvailable=false` sentinel when the stager has no `TooledFlags`
and no tooled fallback; the actual "a stager is required" error is deferred to the tooled-stager
`runLoop` (S2), which fires ONLY when the partition is non-disjoint (i.e. a stager is genuinely needed).

**Deliverable**:
1. **internal/decompose/roles.go** — (a) add `StagerAvailable bool` to `RoleManifests`; (b) in
   `ResolveRoles`, replace the `if fb == "" { return error }` with `if fb == "" { stagerAvailable = false }
   else { …existing fallback… }`; (c) set `rm.StagerAvailable = stagerAvailable` before return; (d) a
   code comment documenting the deferred stager requirement (FR-M13).
2. **internal/decompose/roles_test.go** — FLIP `TestResolveRoles_NoStagerCapable` (currently asserts the
   error) to assert `err==nil` + `rm.StagerAvailable==false` + `rm.Stager.Name=="bareprov"`; OPTIONALLY
   add `StagerAvailable==true` asserts to the existing StagerFallback/HappyPath tests.

**Success Definition**:
- `ResolveRoles(cfg, reg)` with a no-tooled stager provider + NO installed tooled built-in returns
  `(rm, rmodels, nil)` — NO error — with `rm.StagerAvailable==false`, `rm.Stager.Name` == the no-tooled
  provider, `len(rm.Stager.TooledFlags)==0`.
- `ResolveRoles` with a tooled stager (native or fallback) returns `rm.StagerAvailable==true` (the
  happy/fallback paths are unchanged behaviorally).
- `RoleManifests.StagerAvailable` flows to `deps.Roles.StagerAvailable` (both callers pass
  `Roles: roleManifests` whole — no caller edit).
- The flipped `TestResolveRoles_NoStagerCapable_Deferred` passes; the existing HappyPath/StagerFallback
  tests pass unchanged.
- `go build ./...`, `go test ./internal/decompose/...`, `go test -race ./internal/decompose/...`,
  `make test`, `make lint` green; `gofmt -l` clean.

## User Persona (if applicable)

**Target User**: A developer using a no-tooled provider (opencode, or a user-defined provider) as their
default agent, decomposing a cleanly-separated (file-disjoint) dirty tree.

**Use Case**: `stagecoach` with nothing staged + a disjoint working tree, provider=opencode (no
`tooled_flags`). Pre-fix: hard-error "provider opencode cannot stage … and no other installed provider
is stager-capable" BEFORE the planner runs — the fast-path that needs no stager is unreachable. Post-fix:
the fast-path runs (deterministic `git add` staging, no stager agent) and the commits land.

**User Journey**: `stagecoach` → ResolveRoles succeeds (StagerAvailable=false, silently — the user sees
no stager error) → planner partitions → `isFileDisjoint==true` → `runLoopFastPath` (no stager) → commits
published. (If the partition were NON-disjoint, S2's runLoop would error clearly naming the partition
property — but that's S2, not this task.)

**Pain Points Addressed**: BUG-003 — FR-M13 violated: the spec says a no-tooled provider "can now
decompose a disjoint tree, where it otherwise could not serve the stager role," but the eager resolution
gate made that impossible. This task removes the gate.

## Why

- **BUG-003 / FR-M13 (v3.2)**: the file-disjoint fast-path exists precisely so a no-tooled provider can
  decompose a disjoint tree without a stager. The eager stager-resolution error defeats the feature.
  Deferring the requirement (resolve-soft, error-late-only-when-needed) is the PRD-recommended fix.
- **Minimal, surgical**: one bool field + one branch flip. The fast-path needs no change (it never
  touched the stager); the tooled path's deferred error is S2; the fallback widening is S3. This task is
  the ResolveRoles half alone.
- **No caller change**: `RoleManifests` is the ResolveRoles product and flows whole into `Deps.Roles` in
  both callers — the new field rides along with zero edits outside `internal/decompose/`.

## What

**User-visible behavior**: A no-tooled provider decomposing a disjoint tree no longer hard-errors at
role resolution; it proceeds to the fast-path. (A non-disjoint tree with a no-tooled provider will be
handled by S2's runLoop error — not yet present; until S2 lands, such a run fails later at Render/exec,
which is ugly but not silent. S1+S2 should land together.)

**Technical change** (verbatim in the Blueprint):
1. `RoleManifests` gains `StagerAvailable bool`.
2. `ResolveRoles`: declare `stagerAvailable := true`; in the stager block, `if fb == "" { stagerAvailable
   = false } else { …fallback… }`; after the loop `rm.StagerAvailable = stagerAvailable`.
3. Flip the regression test; optionally pin `StagerAvailable=true` on the success paths.

### Success Criteria
- [ ] `RoleManifests` has `StagerAvailable bool` (documented)
- [ ] ResolveRoles no-tooled-no-fallback case returns `nil` error + `StagerAvailable=false` (not an error)
- [ ] the fallback logic is preserved verbatim inside `else` (the "stager fallback provider not found" error stays)
- [ ] `rm.StagerAvailable = stagerAvailable` set before return
- [ ] `TestResolveRoles_NoStagerCapable` flipped to assert success + the sentinel (renamed _Deferred)
- [ ] existing HappyPath/StagerFallback/FR-R5b tests pass unchanged
- [ ] no caller/Decompose/runLoop/FirstTooledProvider change; `make test`/`make lint` green

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim code change (the `if/else` restructure + the bool), the exact field placement +
doc, the proof that both callers pass `Roles: roleManifests` whole (so no caller edit), the existing
test that asserts the bug (must flip) + the helpers to write the regression test, and the explicit scope
fences (S2/S3/callers/docs are NOT this task).

### Documentation & References

```yaml
# MUST READ — the authoritative research (verbatim code + the test flip + the carrier proof)
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M2T1S1/research/findings.md
  why: "§0 the bug site + the fix; §1 the RoleManifests.StagerAvailable carrier + the proof both callers
        pass `Roles: roleManifests` whole (no caller change); §2 the verbatim code change; §3 the EXACT
        existing test (TestResolveRoles_NoStagerCapable:291) that ASSERTS THE BUG and must flip; §4 the
        test helpers (bogusRegistry, bareprov); §5 scope/coordination (S2/S3); §6 validation."
  critical: "§3: TestResolveRoles_NoStagerCapable currently expects the error — it MUST flip to expect
             success + StagerAvailable=false. §2: the fallback logic moves into `else` (else the no-fallback
             branch would fall through to reg.Get(fb='') and break). §1: StagerAvailable rides through
             `Roles: roleManifests` — DO NOT edit the callers."

# MUST READ — the bug analysis (root cause + fix strategy)
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/architecture/bugfix_subsystems.md
  why: "The BUG-003 section: root cause (ResolveRoles gates the fast-path), the two-option fix (defer
        vs. widen FirstTooledProvider), the deferred-error site (runLoop/invokeStager → S2), and the test
        strategy (ResolveRoles no-tooled-no-fallback → no error; non-disjoint → S2 runLoop error)."
  critical: "It confirms option 1 (defer) is the PRIMARY fix (this task) and option 2 (widen
             FirstTooledProvider) is a separate enhancement (S3). The runLoop error is S2."

# MUST READ — the file being edited (ResolveRoles + RoleManifests)
- file: internal/decompose/roles.go
  why: "ResolveRoles (@98), RoleManifests (@32), the stager fallback block (@131-157), setRole (@203).
        The bug is the `if fb == "" { return … }` inside the `if role=='stager' && len(m.TooledFlags)==0`
        block. The fix wraps the fallback in `else` and sets stagerAvailable=false in the no-fallback arm."
  pattern: "ResolveRoles loops the 4 roles; for each it resolve→validate→install-check, then (stager
            only) the FR-D4 fallback, then FR-R5b, then setRole. StagerAvailable is true unless the
            stager no-fallback arm fires."
  gotcha: "The existing fallback logic (reg.Get(fb), prov=fb, m=fbm, model-clearing, default-table) MUST
           move into an `else` — otherwise the no-fallback arm (stagerAvailable=false) falls through to
           reg.Get('') and breaks. Keep that logic byte-identical inside `else`."

# MUST READ — the test file (the test that asserts the bug + the helpers)
- file: internal/decompose/roles_test.go
  why: "TestResolveRoles_NoStagerCapable (@291) ASSERTS THE BUG (expects the 'cannot stage' error) — it
        MUST flip to expect err==nil + StagerAvailable=false. bogusRegistry (@41) + 'bareprov' (the
        no-tooled test provider) are the helpers for the regression test. The HappyPath/StagerFallback
        tests (@94/@137) stay green (StagerAvailable=true for them)."
  critical: "Do NOT delete TestResolveRoles_NoStagerCapable — FLIP it (rename _Deferred, assert success
             + the sentinel). Deleting it loses the regression coverage; flipping it pins the fix."

# CONFIRMING — the callers pass roleManifests whole (the field rides along; no caller edit)
- file: internal/cmd/default_action.go
  why: "Line 498: `Roles: roleManifests,` (inside decompose.Deps{…}). Whole-struct assignment —
        StagerAvailable flows to deps.Roles.StagerAvailable with NO edit. ResolveRoles is called @452."
- file: pkg/stagecoach/stagecoach.go
  why: "Line 211: `Roles: roleManifests,`. Same whole-struct flow. ResolveRoles is called @203."

# CONTEXT — the consumer of the sentinel (LANDS in S2, not here)
- file: internal/decompose/decompose.go
  why: "runLoopFastPath (@249 — the FR-M13 fast-path) needs NO stager (the sentinel is irrelevant to it).
        runLoop (the tooled path) → invokeStager → stageConcept → deps.Roles.Stager.Render (stager.go:106).
        S2 adds the `if !deps.Roles.StagerAvailable && <non-disjoint> { error }` check. This task only
        SETS the bool; S2 consumes it."
  critical: "Do NOT add the runLoop error here (S2). Do NOT touch runLoopFastPath/invokeStager/stageConcept."

# CONTEXT — the FR-M13 spec (what this defers to)
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/prd_snapshot.md
  why: "FR-M13 (§9.14): a no-tooled provider 'can now decompose a disjoint tree, where it otherwise
        could not serve the stager role' via the fast-path. This task makes that reachable. The code
        comment cites FR-M13."
  section: "§9.14 FR-M13 (File-disjoint staging fast-path)"
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  roles.go            # EDIT — RoleManifests +StagerAvailable; ResolveRoles defer the stager error (+ comment)
  roles_test.go       # EDIT — flip TestResolveRoles_NoStagerCapable → _Deferred (+ optional true-asserts)
  decompose.go        # READ-ONLY — runLoopFastPath (@249, no stager) + runLoop (S2 adds the check here)
  stager.go           # READ-ONLY — invokeStager Renders deps.Roles.Stager (@106) — S2's error site
internal/cmd/
  default_action.go   # READ-ONLY — caller; `Roles: roleManifests` (@498) flows StagerAvailable, NO edit
pkg/stagecoach/
  stagecoach.go       # READ-ONLY — caller; `Roles: roleManifests` (@211) flows StagerAvailable, NO edit
internal/provider/
  registry.go         # READ-ONLY — FirstTooledProvider (@127, built-ins only) — S3 extends it
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (no new files):
internal/decompose/roles.go        # +RoleManifests.StagerAvailable; ResolveRoles defer (+ comment)
internal/decompose/roles_test.go   # flip TestResolveRoles_NoStagerCapable → _Deferred (+ optional asserts)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (wrap the fallback in `else`): the existing code is `if fb == "" { return error }; fbm := reg.Get(fb);
//   prov=fb; m=fbm; …` — SEQUENTIAL after the error return. Replacing the `return` with `stagerAvailable=false`
//   WITHOUT wrapping the fallback in `else` would fall through to `reg.Get("")` and break. The fallback
//   logic MUST move into an `else` block; keep it byte-identical inside.

// CRITICAL (the existing test ASSERTS THE BUG — it must FLIP, not be deleted): TestResolveRoles_NoStagerCapable
//   (roles_test.go:291) expects the 'cannot stage'/'stager-capable' error. After the fix ResolveRoles
//   returns nil. Flip the test to assert err==nil + StagerAvailable=false + rm.Stager.Name=='bareprov'.
//   Deleting it loses the regression coverage; flipping it pins the fix.

// CRITICAL (StagerAvailable rides through `Roles: roleManifests` — NO caller edit): both callers
//   (default_action.go:498, stagecoach.go:211) assign the WHOLE roleManifests struct to Deps.Roles.
//   Adding the field to RoleManifests flows it to deps.Roles.StagerAvailable automatically. Do NOT edit
//   the callers (and do NOT field-by-field copy RoleManifests anywhere — that would drop the new field).

// GOTCHA (StagerAvailable defaults true; flipped false ONLY in the no-fallback arm): the happy path
//   (stager has its own TooledFlags) and the fallback path (fb found → m=fbm tooled) both leave it true.
//   Only `role=='stager' && len(m.TooledFlags)==0 && fb==''` sets it false. Initialize `stagerAvailable := true`.

// GOTCHA (the no-tooled manifest is still stored in rm.Stager): the no-fallback arm does NOT skip setRole —
//   rm.Stager carries the no-tooled manifest (Name=the provider, TooledFlags empty) for diagnostics +
//   S2's error message. The fast-path never touches it; S2's runLoop errors before Render. This is intentional.

// GOTCHA (the "stager fallback provider not found" error STAYS): that error (reg.Get(fb) fails after a
//   fallback WAS chosen) is a registry-consistency failure, NOT the no-tooled case. Keep it inside `else`.
//   Only the `fb == ""` (no fallback at all) arm is de-errored.

// GOTCHA (S1 + S2 should land together): with only S1, a non-disjoint partition + no-tooled provider
//   would reach runLoop → invokeStager → Render the no-tooled manifest → fail at exec (ugly, not silent).
//   S2 adds the clean `if !StagerAvailable { error }` check before that. S1 is correct in isolation for
//   the disjoint case (the fast-path); the non-disjoint error is S2's concern.
```

## Implementation Blueprint

### Data models and structure

```go
// RoleManifests gains one field (the sentinel carrier).
type RoleManifests struct {
    Planner provider.Manifest
    Stager  provider.Manifest  // tooled when StagerAvailable; the no-tooled manifest when !StagerAvailable (FR-M13 deferred — diagnostics only)
    Message provider.Manifest
    Arbiter provider.Manifest
    StagerAvailable bool  // NEW (BUG-003/FR-M13): false ⇒ stager cannot stage; fast-path still works, runLoop (S2) must error
}
```
No other types change. `StagerAvailable` is a plain bool set in ResolveRoles, read by S2's runLoop.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/decompose/roles.go — add StagerAvailable to RoleManifests
  - In `type RoleManifests struct` (@32), ADD the field with the doc comment (verbatim in findings §1):
        // StagerAvailable reports whether the resolved Stager is tooled (native TooledFlags OR the FR-D4
        // fallback found a tooled provider). When false, the stager CANNOT stage: the FR-M13 file-disjoint
        // fast-path (runLoopFastPath) still works (no stager needed), but the tooled-stager runLoop
        // (non-disjoint partition) MUST error (S2). BUG-003: defers the stager requirement out of
        // ResolveRoles so a no-tooled provider can reach the fast-path.
        StagerAvailable bool
  - PLACE: after the four manifest fields (Planner/Stager/Message/Arbiter).

Task 2: EDIT internal/decompose/roles.go — defer the stager error in ResolveRoles
  - In `ResolveRoles` (@98), DECLARE before the loop: `stagerAvailable := true`.
  - In the stager fallback block (@131-157), RESTRUCTURE:
        if role == "stager" && len(m.TooledFlags) == 0 {
            fb := reg.FirstTooledProvider(installed)
            if fb == "" {
                // BUG-003 / FR-M13: do NOT error — defer to runLoop (S2). [the comment from findings §2]
                stagerAvailable = false
            } else {
                // … the EXISTING fallback logic, byte-identical, moved into this else …
                // fbm, ok := reg.Get(fb); if !ok { return …"fallback provider not found" }; prov=fb; m=fbm;
                // model-clearing + default-table …
            }
        }
  - BEFORE `return rm, rmodels, nil` (after the loop), ADD: `rm.StagerAvailable = stagerAvailable`.
  - PRESERVE: the resolve/validate/install logic, the FR-R5b check, setRole, the "fallback provider not
    found" error (inside else). ONLY the `fb == ""` arm changes (error → sentinel).

Task 3: EDIT internal/decompose/roles_test.go — flip TestResolveRoles_NoStagerCapable
  - RENAME `TestResolveRoles_NoStagerCapable` (@291) → `TestResolveRoles_NoStagerCapable_Deferred` (or keep
    the name; renaming signals the flipped intent).
  - REPLACE the body (findings §3): resolve; assert err==nil; assert !rm.StagerAvailable; assert
    rm.Stager.Name=="bareprov"; assert len(rm.Stager.TooledFlags)==0.
  - REMOVE the old "cannot stage"/"stager-capable" error-substring assertions (they're gone).
  - OPTIONALLY add `if !rm.StagerAvailable { t.Error("want true") }` to TestResolveRoles_StagerFallback
    (@137) and TestResolveRoles_HappyPath_AllPi (@94) to pin the bool=true on success paths.

Task 4: VERIFY — build, vet, format, focused + race tests, lint, grep guards
  - go build ./... ; go vet ./internal/decompose/...
  - gofmt -l internal/decompose/roles.go internal/decompose/roles_test.go   # empty
  - go test ./internal/decompose/ -run 'ResolveRoles' -v   # flipped test PASSES; HappyPath/Fallback PASS
  - go test -race ./internal/decompose/... ; make test ; make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the deferred-stager branch (error → sentinel; fallback moves into else)
if role == "stager" && len(m.TooledFlags) == 0 {
    fb := reg.FirstTooledProvider(installed)
    if fb == "" {
        // BUG-003 / FR-M13: defer — a no-tooled provider can still hit the disjoint fast-path.
        stagerAvailable = false
    } else {
        fbm, ok := reg.Get(fb)
        if !ok { return RoleManifests{}, RoleModels{}, fmt.Errorf("role %q: stager fallback provider %q not found", role, fb) }
        prov, m = fb, fbm
        if isMultiProvider(m) && mdl != "" && !strings.Contains(mdl, "/") { mdl = "" }
        if col := config.DefaultModelsForProvider(fb); col != nil {
            if c := col["stager"]; c != "" && !(isMultiProvider(m) && !strings.Contains(c, "/")) { mdl = c }
        }
    }
}
// … FR-R5b check …
setRole(&rm, &rmodels, role, m, prov, mdl, rsn)  // stager: stores the no-tooled m when !stagerAvailable

// PATTERN: the flipped regression test
rm, _, err := ResolveRoles(cfg, reg)
if err != nil { t.Fatalf("ResolveRoles=%v, want nil (FR-M13 deferred)", err) }
if rm.StagerAvailable          { t.Error("StagerAvailable=true, want false") }
if rm.Stager.Name != "bareprov"{ t.Errorf("Stager.Name=%q, want bareprov", rm.Stager.Name) }
if len(rm.Stager.TooledFlags)!=0 { t.Error("Stager.TooledFlags non-empty, want the no-tooled manifest") }
```

### Integration Points

```yaml
NO new types (one bool field) / config / routes / deps / CLI flags. TWO files edited.

ROLES (internal/decompose/roles.go):
  - RoleManifests: +StagerAvailable bool.
  - ResolveRoles: the `fb==""` arm sets stagerAvailable=false (no error); fallback moves into else;
    rm.StagerAvailable set before return.

DOWNSTREAM (this task SETS the bool; consumers are SEPARATE subtasks):
  - S2 (runLoop check): reads deps.Roles.StagerAvailable; errors if false AND partition non-disjoint.
  - S3 (FirstTooledProvider): widens the fallback to user-defined tooled providers (fewer !StagerAvailable cases).
  - The fast-path (runLoopFastPath @249) is UNAFFECTED — it never touches the stager.

CALLERS (NO edit): default_action.go:498 + stagecoach.go:211 pass `Roles: roleManifests` whole →
  StagerAvailable flows to deps.Roles.StagerAvailable automatically.

SCOPE FENCES: NO runLoop/invokeStager/stageConcept (S2); NO FirstTooledProvider (S3); NO callers;
  NO docs (Mode A = the code comment); NO PRD/tasks.json.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build + vet.
go build ./...
go vet ./internal/decompose/...

# Format.
gofmt -l internal/decompose/roles.go internal/decompose/roles_test.go
# Expected: empty. If listed: gofmt -w the file(s).

# Lint.
make lint
# Expected: zero errors. (`unused` clean — StagerAvailable is read by tests now and by S2 later.)

# Scope guard: only roles.go + roles_test.go changed.
git diff --name-only
# Expected: internal/decompose/roles.go  internal/decompose/roles_test.go (only).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The ResolveRoles suite — the flipped test + the existing happy/fallback tests.
go test ./internal/decompose/ -run 'ResolveRoles' -v
# Expected: TestResolveRoles_NoStagerCapable_Deferred PASSES (err nil, StagerAvailable false, Stager=bareprov);
#           HappyPath_AllPi / StagerFallback* / FR-R5b tests PASS UNCHANGED (StagerAvailable true for them).

# Race + full decompose package.
go test -race ./internal/decompose/...
go test ./internal/decompose/... -v
# Expected: green. (A Decompose e2e over a non-disjoint tree + no-tooled provider won't cleanly error
#           until S2 lands — that's expected; S1 is the ResolveRoles half. The ResolveRoles unit test is
#           the within-scope proof.)

# Full repo.
make test
# Expected: green.
```

### Level 3: Integration Testing (System Validation)

```bash
# S1 is unit-test-scoped (ResolveRoles). The full e2e — "no-tooled provider decomposes a disjoint tree
# via the fast-path" — is the P1.M2.T1 epic's integration coverage (S2 + the fast-path compose). This
# task's proof is the Level 2 ResolveRoles test. A manual end-to-end check (optional, needs the stub
# agent harness) would set provider=opencode + a disjoint tree and confirm no stager error at resolution;
# defer the full e2e to the bugfix suite's integration layer.

# Sanity: the package still builds into the binary.
go build ./...
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1: RoleManifests has the new field.
grep -n 'StagerAvailable bool' internal/decompose/roles.go
# Expected: 1 hit (the struct field).

# Grep guard 2: ResolveRoles no longer errors in the no-fallback arm (the old error string is GONE).
grep -c 'cannot stage (tooled_flags empty) and no other installed' internal/decompose/roles.go
# Expected: 0. (The "stager fallback provider not found" error STAYS — grep for it separately to confirm.)
grep -c 'stager fallback provider' internal/decompose/roles.go
# Expected: 1 (the reg.Get(fb)-fails error, inside else).

# Grep guard 3: stagerAvailable is set false in the no-fallback arm + assigned to rm before return.
grep -n 'stagerAvailable = false\|rm.StagerAvailable = stagerAvailable' internal/decompose/roles.go
# Expected: 2 hits.

# Grep guard 4: the flipped test asserts success (not the old error).
grep -c 'cannot stage\|stager-capable' internal/decompose/roles_test.go
# Expected: 0 (the old assertions are removed). And:
grep -c 'StagerAvailable' internal/decompose/roles_test.go
# Expected: ≥1 (the flipped test asserts the bool; optional true-asserts in fallback tests add more).

# Grep guard 5: NO caller change (the field rides through Roles: roleManifests).
git diff --name-only | grep -E 'default_action\.go|stagecoach\.go'
# Expected: empty (those callers are unedited).

# Grep guard 6: NO runLoop / FirstTooledProvider / docs change (S2/S3/Mode-B-docs are separate).
git diff --name-only | grep -vE 'internal/decompose/roles\.go|internal/decompose/roles_test\.go'
# Expected: empty (only the 2 files).

# Regression: the existing ResolveRoles happy/fallback tests stay green.
go test ./internal/decompose/ -run 'ResolveRoles_HappyPath|ResolveRoles_StagerFallback|ResolveRoles_StagerFallback_Pi|ResolveRoles_R5b' -v
# Expected: all PASS.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `go vet ./internal/decompose/...` clean
- [ ] `gofmt -l` empty on roles.go + roles_test.go
- [ ] `go test -race ./internal/decompose/...` green
- [ ] `make test` + `make lint` pass

### Feature Validation
- [ ] ResolveRoles no-tooled-no-fallback → `(rm, rmodels, nil)` + `rm.StagerAvailable==false` + `rm.Stager.Name`==the provider + `len(rm.Stager.TooledFlags)==0`
- [ ] ResolveRoles happy/fallback paths → `rm.StagerAvailable==true` (behavior unchanged)
- [ ] `TestResolveRoles_NoStagerCapable_Deferred` passes (flipped from the bug-asserting original)

### Scope-Boundary Validation
- [ ] `git diff --name-only` == only {roles.go, roles_test.go}
- [ ] NO caller edit (default_action.go/stagecoach.go — `Roles: roleManifests` flows the field)
- [ ] NO runLoop/invokeStager/stageConcept change (S2)
- [ ] NO FirstTooledProvider change (S3)
- [ ] NO docs (Mode A = the code comment only)
- [ ] the "stager fallback provider not found" error PRESERVED (inside else)

### Code Quality & Docs
- [ ] StagerAvailable field documented (FR-M13 deferred-stager rationale)
- [ ] ResolveRoles comment cites BUG-003/FR-M13 + that the deferred error is S2's runLoop check
- [ ] the fallback logic is byte-identical inside `else` (only the `fb==""` arm changed)

---

## Anti-Patterns to Avoid

- ❌ Don't leave the fallback logic sequential after the de-errored arm. The existing code is
  `if fb == "" { return }; fbm := reg.Get(fb); …` — sequential, relying on the `return` to skip it.
  Replacing `return` with `stagerAvailable = false` WITHOUT an `else` falls through to `reg.Get("")` and
  breaks. The fallback MUST move into an `else` block.
- ❌ Don't DELETE `TestResolveRoles_NoStagerCapable`. It currently asserts the bug (expects the error);
  the fix flips the behavior. FLIP the test (assert success + StagerAvailable=false), don't remove it —
  deleting loses the regression coverage that pins the fix.
- ❌ Don't edit the callers. Both pass `Roles: roleManifests` WHOLE (default_action.go:498,
  stagecoach.go:211), so the new `StagerAvailable` field flows to `deps.Roles.StagerAvailable`
  automatically. A field-by-field RoleManifests copy anywhere would DROP the field — don't introduce one.
- ❌ Don't set `StagerAvailable` only in the no-fallback arm. Default it `true` and flip it `false` ONLY in
  the `fb == ""` arm; the happy path (stager has TooledFlags) and the fallback path (fb found) both leave
  it true. Forgetting the default makes every success path report `false`.
- ❌ Don't skip storing the no-tooled manifest in `rm.Stager`. The no-fallback arm still falls through to
  `setRole` — `rm.Stager` carries the no-tooled manifest (for diagnostics + S2's error message). The
  fast-path never touches it; S2 errors before Render. Skipping setRole would leave `rm.Stager` zero-value
  and break S2's named-provider error.
- ❌ Don't remove the "stager fallback provider not found" error. That error (reg.Get(fb) fails AFTER a
  fallback was chosen) is a registry-consistency failure, distinct from the no-tooled case. Keep it inside
  `else`; only the `fb == ""` arm is de-errored.
- ❌ Don't add the runLoop error here. The deferred "stager needed but unavailable" check is S2's job
  (runLoop reads `deps.Roles.StagerAvailable`). This task only SETS the bool. Adding the runLoop check
  here would collide with S2.
- ❌ Don't touch FirstTooledProvider. Widening it to user-defined tooled providers is S3. This task defers
  the requirement; S3 reduces how often the no-fallback arm fires.
- ❌ Don't add user-facing docs. Mode A = the code comment in ResolveRoles (cite FR-M13). The behavior
  matches FR-M13 already documented in spec; no user-facing doc surface changes. docs sync is a separate task.
- ❌ Don't anchor to line numbers. `roles.go` / the callers may drift on sibling edits. Locate
  ResolveRoles via `grep -n 'func ResolveRoles'`, the struct via `grep -n 'type RoleManifests'`, and the
  test via `grep -n 'TestResolveRoles_NoStagerCapable'`.

---

## Confidence Score: 9/10

The verbatim code change (the `if/else` restructure + the bool field), the proof that both callers pass
`Roles: roleManifests` whole (so no caller edit), the exact existing test that asserts the bug (must
flip) + the helpers to write the regression test, and the explicit S2/S3/caller/docs scope fences are
all spelled out and verified against the current source. The one residual (not a full 10): with only S1,
a non-disjoint partition + no-tooled provider reaches runLoop and fails at Render/exec (ugly, not silent)
until S2 lands — so S1+S2 should land together; S1 alone is correct only for the disjoint fast-path (the
BUG-003 scenario). The ResolveRoles unit test is the within-scope proof; the full e2e composes with S2.
One-pass success on the S1 scope is highly likely.