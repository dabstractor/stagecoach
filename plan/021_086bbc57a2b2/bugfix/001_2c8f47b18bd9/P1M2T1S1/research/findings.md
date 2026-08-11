# Research: P1.M2.T1.S1 — Defer the stager requirement in ResolveRoles (BUG-003, FR-M13)

Fix BUG-003 (Major): `ResolveRoles` (internal/decompose/roles.go:132) hard-errors when the stager has
no `TooledFlags` and no installed tooled fallback — which blocks Decompose BEFORE the planner runs,
making the FR-M13 file-disjoint fast-path (which needs NO stager) unreachable for no-tooled providers.
This task (S1) makes ResolveRoles RETURN SUCCESS with a `StagerAvailable=false` sentinel instead of
erroring; the actual "stager needed but unavailable" error is deferred to the tooled-stager runLoop
(S2, a separate subtask). S3 extends FirstTooledProvider (also separate).

All claims verified against roles.go, the callers, and roles_test.go.

---

## 0. THE BUG + THE FIX (S1 scope only)

**Bug site** — `internal/decompose/roles.go`, inside the `for role …` loop, the stager fallback block:
```go
if role == "stager" && len(m.TooledFlags) == 0 {
    fb := reg.FirstTooledProvider(installed)
    if fb == "" {
        return RoleManifests{}, RoleModels{}, fmt.Errorf(           // ← THE BUG: hard-error
            "role %q: provider %q cannot stage (tooled_flags empty) and no other installed "+
                "provider is stager-capable", role, prov)
    }
    fbm, ok := reg.Get(fb)
    if !ok { return … "stager fallback provider %q not found" }
    prov = fb; m = fbm
    // … model-clearing + default-table logic …
}
// then FR-R5b check, then setRole(&rm, &rmodels, "stager", m, prov, mdl, rsn)
```
This fires at Decompose ENTRY (ResolveRoles is called BEFORE the planner + BEFORE disjointness is
known), so a no-tooled provider (opencode, or a user-defined provider) with no installed tooled
built-in can never reach `runLoopFastPath` (decompose.go:249), which needs NO stager. Contradicts
FR-M13 (v3.2): "a provider whose manifest declares no tooled_flags can now decompose a disjoint tree."

**The S1 fix** — do NOT error in the `fb == ""` branch. Instead set a sentinel + return success:
1. Add `StagerAvailable bool` to `RoleManifests`.
2. In the `fb == ""` branch: set `stagerAvailable = false` (don't error; don't return). Wrap the
   existing fallback logic in `else` so it's skipped when there's no fallback.
3. After the loop: `rm.StagerAvailable = stagerAvailable`.
4. The no-tooled manifest `m` is still stored in `rm.Stager` via `setRole` (carried for diagnostics;
   the fast-path never touches it, and S2's runLoop errors before invoking it).

The downstream contract (S2, NOT this task): `runLoop` reads `deps.Roles.StagerAvailable`; if false AND
the partition is non-disjoint → clear error "this partition requires a tooled stager (shared files
across concepts), but provider X has no tooled_flags and no stager-capable provider is installed".

---

## 1. THE SENTINEL CARRIER — `RoleManifests.StagerAvailable` (the design decision)

`RoleManifests` (roles.go:32) is ResolveRoles' return value and flows into `Deps.Roles`:
- `internal/cmd/default_action.go:498` → `deps := decompose.Deps{ … Roles: roleManifests, … }`
- `pkg/stagecoach/stagecoach.go:211` → `deps := decompose.Deps{ … Roles: roleManifests, … }`

BOTH callers pass `roleManifests` WHOLE (not field-by-field), so adding `StagerAvailable bool` to
`RoleManifests` flows to `deps.Roles.StagerAvailable` with **NO caller change** — the field rides along.

Why RoleManifests (not Deps, not a nil sentinel): `provider.Manifest` is a VALUE type (struct with
pointer fields) — it cannot be nil. A bool on RoleManifests (the ResolveRoles product that becomes
Deps.Roles) is the minimal, natural carrier; S2 reads `deps.Roles.StagerAvailable`.

```go
type RoleManifests struct {
    Planner provider.Manifest // bare
    Stager  provider.Manifest // tooled (TooledFlags non-empty) when StagerAvailable; the no-tooled manifest when !StagerAvailable (FR-M13 deferred — diagnostics only)
    Message provider.Manifest // bare
    Arbiter provider.Manifest // bare
    // StagerAvailable reports whether the resolved Stager is tooled (native TooledFlags OR the FR-D4
    // fallback found a tooled provider). When false, the stager CANNOT stage: Decompose's file-disjoint
    // fast-path (FR-M13, runLoopFastPath) still works (it needs no stager), but the tooled-stager runLoop
    // (non-disjoint partition) MUST error (S2). BUG-003: this defers the stager requirement out of
    // ResolveRoles so a no-tooled provider can reach the fast-path.
    StagerAvailable bool
}
```

`StagerAvailable` is true on the happy path (stager has its own TooledFlags) AND on the fallback path
(fallback found → m=fbm which has TooledFlags). False ONLY in the no-fallback branch (this fix).

---

## 2. THE EXACT CODE CHANGE (roles.go)

Declare `stagerAvailable` before/inside the loop; restructure the fallback block:

```go
func ResolveRoles(cfg config.Config, reg *provider.Registry) (RoleManifests, RoleModels, error) {
    installed := computeInstalled(reg)
    var rm RoleManifests
    var rmodels RoleModels
    stagerAvailable := true // a tooled stager resolves unless the no-fallback branch (below) flips it

    for _, role := range []string{"planner", "stager", "message", "arbiter"} {
        // … existing resolve/validate/install logic unchanged …
        if role == "stager" && len(m.TooledFlags) == 0 {
            fb := reg.FirstTooledProvider(installed)
            if fb == "" {
                // BUG-003 / FR-M13: do NOT error. A no-tooled provider can still decompose a FILE-DISJOINT
                // tree via runLoopFastPath (decompose.go:249 — stages deterministically with `git add`,
                // no stager agent). Defer the stager requirement to the tooled-stager runLoop (S2), which
                // errors ONLY when the partition is non-disjoint (shared files → a stager is actually
                // needed). Carry the no-tooled manifest in rm.Stager + StagerAvailable=false so runLoop
                // can detect the gap and produce a clear, partition-named error.
                stagerAvailable = false
                // fall through to setRole — rm.Stager carries the no-tooled manifest (diagnostics only)
            } else {
                fbm, ok := reg.Get(fb)
                if !ok {
                    return RoleManifests{}, RoleModels{}, fmt.Errorf("role %q: stager fallback provider %q not found", role, fb)
                }
                prov = fb
                m = fbm
                if isMultiProvider(m) && mdl != "" && !strings.Contains(mdl, "/") { mdl = "" }
                if col := config.DefaultModelsForProvider(fb); col != nil {
                    if candidate := col["stager"]; candidate != "" && !(isMultiProvider(m) && !strings.Contains(candidate, "/")) {
                        mdl = candidate
                    }
                }
            }
        }
        // … FR-R5b check unchanged …
        setRole(&rm, &rmodels, role, m, prov, mdl, rsn)
    }

    rm.StagerAvailable = stagerAvailable
    return rm, rmodels, nil
}
```
**The ONLY structural change** is: `if fb == "" { return error }` → `if fb == "" { stagerAvailable = false }
else { …fallback logic… }` (the existing fallback moves into `else`), + the `stagerAvailable` var +
`rm.StagerAvailable = stagerAvailable`. Everything else (resolve/validate/install/FR-R5b/setRole) is
byte-identical. The fallback "stager fallback provider not found" error STAYS (it's a different
failure — a registry inconsistency, not the no-tooled case).

---

## 3. THE EXISTING TEST THAT MUST FLIP — `TestResolveRoles_NoStagerCapable` (roles_test.go:291)

**This test currently ASSERTS THE BUG** — it expects `ResolveRoles` to error with "cannot stage" +
"stager-capable". My fix flips the behavior, so this test MUST be updated (it's the regression test for
the bug; flipping it is the regression test for the fix). Current (roles_test.go:291-309):
```go
func TestResolveRoles_NoStagerCapable(t *testing.T) {
    reg := bogusRegistry(t, []string{"bareprov"})
    cfg := config.Config{Provider: "bareprov", Roles: map[string]config.RoleConfig{"stager": {Provider: "bareprov"}}}
    _, _, err := ResolveRoles(cfg, reg)
    if err == nil { t.Fatal("…want stager-capable error") }
    errMsg := err.Error()
    if !strings.Contains(errMsg, "cannot stage") || !strings.Contains(errMsg, "stager-capable") { … }
}
```
**After the fix** — assert SUCCESS + the sentinel (rename to `TestResolveRoles_NoStagerCapable_Deferred`):
```go
func TestResolveRoles_NoStagerCapable_Deferred(t *testing.T) {
    reg := bogusRegistry(t, []string{"bareprov"})            // bareprov = no-tooled; all built-ins bogus
    cfg := config.Config{Provider: "bareprov", Roles: map[string]config.RoleConfig{"stager": {Provider: "bareprov"}}}
    rm, _, err := ResolveRoles(cfg, reg)
    if err != nil { t.Fatalf("ResolveRoles returned %v, want nil (FR-M13: stager deferred, not a hard error)", err) }
    if rm.StagerAvailable { t.Error("StagerAvailable=true, want false (bareprov has no tooled_flags and no fallback)") }
    if rm.Stager.Name != "bareprov" { t.Errorf("rm.Stager.Name=%q, want bareprov (no-tooled manifest carried)", rm.Stager.Name) }
    if len(rm.Stager.TooledFlags) != 0 { t.Error("rm.Stager.TooledFlags non-empty, want empty (the no-tooled manifest)") }
}
```

**The OTHER ResolveRoles tests stay GREEN UNCHANGED** — they exercise the happy/fallback paths where a
tooled stager resolves (StagerAvailable=true); they assert on `rm.Stager.TooledFlags` (non-empty), not
on StagerAvailable (the field is new). OPTIONALLY add `if !rm.StagerAvailable { t.Error(...) }` to
`TestResolveRoles_StagerFallback` (and HappyPath) to pin the bool=true on the success paths — cheap,
strengthens the contract.

---

## 4. TEST HELPERS (verified)

- `bogusRegistry(t, installed []string)` (roles_test.go:41) — overrides all 6 built-ins
  (pi/opencode/cursor/agy/codex/claude) to a bogus command, then overrides the `installed` names back
  to "go". `provider.NewRegistry(overrides)` accepts arbitrary names (so "bareprov" is a test provider).
- "bareprov" — the canonical no-tooled test provider (Manifest{Command:"go", Detect:"go"}, no
  TooledFlags); used by the existing stager-fallback tests. It is NOT a built-in (it's a test override).
- `config.Config{Provider: …, Roles: map[string]config.RoleConfig{"stager": {Provider: "bareprov"}}}`
  — the cfg shape that routes the stager to bareprov (the no-tooled path).

---

## 5. SCOPE & COORDINATION

- **S1 ONLY: `internal/decompose/roles.go` (ResolveRoles + RoleManifests) + `internal/decompose/roles_test.go`
  (flip NoStagerCapable + optional asserts).** NO runLoop/invokeStager/stageConcept (S2 adds the deferred
  error there); NO FirstTooledProvider (S3 extends it); NO callers (default_action.go/stagecoach.go —
  `Roles: roleManifests` whole-struct flows StagerAvailable automatically); NO docs (Mode A = the code
  comment in ResolveRoles; no user-facing surface — behavior matches FR-M13 documented in spec).
- **Parallel P1.M1.T5.S1** is work-description DOCS (how-it-works.md + README) — NOT roles.go. No overlap.
- **The deferred error (S2)** lives in `runLoop` (decompose.go) / `invokeStager` (stager.go:106, which
  Renders `deps.Roles.Stager`). S2 reads `deps.Roles.StagerAvailable`; if false AND non-disjoint → error.
  S1 only SETS the bool; S2 consumes it. (If S2 hasn't landed when S1 does, the no-tooled manifest in
  rm.Stager would be Rendered by invokeStager on a non-disjoint partition and fail at Render/exec —
  ugly but not silent; S2 makes it a clean, partition-named error. S1+S2 should land together.)
- **NO PRD.md / tasks.json / prd_snapshot.md** (read-only).

---

## 6. VALIDATION

- `go build ./...` / `go vet ./internal/decompose/...`
- `gofmt -l internal/decompose/roles.go internal/decompose/roles_test.go`
- `go test ./internal/decompose/ -run 'ResolveRoles' -v` → the flipped NoStagerCapable_Deferred PASSES
  (err nil, StagerAvailable false) + the existing HappyPath/StagerFallback tests PASS (StagerAvailable true).
- `go test -race ./internal/decompose/...` / `make test` / `make lint`
- NOTE: a full `Decompose` e2e over a non-disjoint tree with a no-tooled provider will NOT cleanly error
  until S2 lands (it would fail later at Render/exec). That's expected — S1 is the ResolveRoles half;
  the runLoop check is S2. The S1 unit test (ResolveRoles-level) is the within-scope proof.