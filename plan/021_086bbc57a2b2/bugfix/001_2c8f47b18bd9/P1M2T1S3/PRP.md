# PRP — P1.M2.T1.S3: FirstTooledProvider user-defined tooled-provider fallback (BUG-003, option 2)

> **Single .go edit + one regression test.** Widen `internal/provider/registry.go`
> `FirstTooledProvider` so that, when no built-in tooled provider is installed, a **user-defined**
> (§12.8) provider with non-empty `TooledFlags` is selected as the stager fallback. Built-in
> preference order is preserved (pi > … > claude, then user-defined in registry order). Bug fix —
> no PRP-as-scope-change needed (brings code INTO line with BUG-003's option-2 recommendation +
> FR-M13/FR-D4). Stdlib-only; no new deps.

---

## Goal

**Feature Goal**: `FirstTooledProvider(installed)` returns a user-defined tooled provider's name
when (a) no built-in tooled provider is installed AND (b) a user-defined provider with non-empty
`TooledFlags` IS installed — instead of returning `""` in that case. This is BUG-003 architecture
option 2 ("let FirstTooledProvider also consider user-defined tooled providers as a fallback").

**Deliverable**: Edit `internal/provider/registry.go` `FirstTooledProvider` (add phase-2 scan +
update its doc comment — Mode A docs, no user-facing doc change) and add
`TestFirstTooledProvider_UserDefinedFallback` to `internal/provider/registry_test.go`.

**Success Definition**:
- `FirstTooledProvider` scans built-ins first (unchanged phase 1), then user-defined providers in
  `List()` order (new phase 2), returning the first installed+tooled user-defined match when phase 1
  found none.
- Built-in preference is preserved: a built-in tooled provider (pi/claude/agy) is ALWAYS preferred
  over any user-defined provider.
- The existing `TestFirstTooledProvider` (built-ins only) stays green and UNCHANGED.
- The new test asserts: user-defined-tooled fallback works; built-in still wins; empty-tooled
  user-defined is never selected.

---

## Why

- **BUG-003 (PRD §h2.2/h3.1)**: a no-tooled provider (e.g. opencode, or a user-defined provider with
  empty `TooledFlags`) hard-errored at `ResolveRoles` even for a file-disjoint tree, blocking the
  FR-M13 fast-path. S1 (Complete) deferred the error to the tooled-stager `runLoop` via a
  `StagerAvailable` sentinel; S2 (Implementing) makes `runLoop` error only when a stager is genuinely
  required (non-disjoint partition). S3 is the **enhancement half**: it reduces how often
  `StagerAvailable=false` by finding more stager fallbacks, so a non-disjoint decompose run with a
  user-defined tooled provider installed does NOT get deferred.
- **The widened return feeds S1's consumer unchanged**: `roles.go:147` does
  `fb := reg.FirstTooledProvider(installed)`; the `else` branch `reg.Get(fb)` uses the returned name
  as the stager. A user-defined name is Get()-able (it came from `r.List()`) and its manifest is tooled
  (S3's filter guarantees it) — so the consumer is sound with zero changes.
- **User value**: a user who configures a custom agent with `tooled_flags` can decompose non-disjoint
  trees without also installing pi/claude.

---

## What

`FirstTooledProvider` gains a second scan loop AFTER the existing `preferredBuiltins` loop. The
existing loop (phase 1) is byte-for-byte unchanged. Phase 2 iterates `r.List()`, skips built-ins
(identified via a set built from `preferredBuiltins`), and returns the first user-defined provider
that is both installed (in the `present` set) and stager-capable (`len(m.TooledFlags) > 0`).

### Success Criteria

- [ ] `FirstTooledProvider` returns a user-defined tooled provider when no built-in tooled provider is installed.
- [ ] Built-in preference order is preserved (pi > opencode > cursor > agy > codex > claude, then user-defined alphabetical).
- [ ] A user-defined provider with empty/nil `TooledFlags` is never selected (the predicate is unchanged: `len(m.TooledFlags) > 0`).
- [ ] The doc comment on `FirstTooledProvider` documents the user-defined fallback scan (Mode A).
- [ ] The existing `TestFirstTooledProvider` passes unchanged.
- [ ] New `TestFirstTooledProvider_UserDefinedFallback` covers: fallback hits, built-in-preferred, empty-tooled-rejected.

---

## All Needed Context

### Context Completeness Check

A developer who has never seen this repo can implement S3 from: the current `FirstTooledProvider`
body (below), the consumer contract (roles.go:147, below), the `List()`/`Get()`/`IsInstalled`
signatures (below), the `preferredBuiltins == builtins` invariant (below), and the target code +
test (below). All present here. No external libraries involved.

### Documentation & References

```yaml
# MUST READ — the function under edit (verbatim current body)
- file: internal/provider/registry.go
  why: FirstTooledProvider (line 127) is the edit site; preferredBuiltins (line 16) is the built-in
       priority list; Get/List/IsInstalled/NewRegistry are the surrounding API.
  pattern: "present := map[string]struct{}{}; for _,name := range installed {present[name]=struct{}{}}";
       "if len(m.TooledFlags) > 0 { return name }" is the stager-capability predicate — reuse verbatim.
  gotcha: "preferredBuiltins is the COMPLETE built-in set (TestPreferredBuiltins_MatchesBuiltinKeys
       enforces sync with BuiltinManifests keys). Build the phase-2 skip-set from preferredBuiltins,
       NOT from BuiltinManifests() (which allocates a map each call)."

# MUST READ — the consumer (S1, Complete) — proves the widened return is sound
- file: internal/decompose/roles.go
  why: ResolveRoles stager block (lines 145-178) is the ONLY caller of FirstTooledProvider. Confirms
       installed = computeInstalled(reg) (line 109/202: IsInstalled over List() → a list of NAMES);
       fb=="" → StagerAvailable=false (deferred); else reg.Get(fb) uses fb as the stager.
  pattern: "fb := reg.FirstTooledProvider(installed); if fb == \"\" { stagerAvailable = false } else { ... reg.Get(fb) ... }"
  gotcha: "The returned name is passed to reg.Get(fb) and its manifest used as the stager — so it MUST
       be Get()-able (guaranteed: S3 returns a name drawn from r.List()) and its manifest MUST have
       non-empty TooledFlags (guaranteed: S3's filter is len(m.TooledFlags) > 0)."

# MUST READ — the test site + table pattern
- file: internal/provider/registry_test.go
  why: TestFirstTooledProvider (line 355) is table-driven with NewRegistry(nil). Shows the
       {installed, want} shape and that agy IS stager-capable (a second built-in tooled provider,
       so preference order is exercised). S3 mirrors this style for the new test.
  pattern: "r := NewRegistry(nil); cases := []struct{installed []string; want string}{...};
       for _,c := range cases { if got := r.FirstTooledProvider(c.installed); got != c.want {...} }"
  gotcha: "The existing {[]string{\"myagent\"}, \"\"} case stays GREEN under S3 — myagent is NOT in
       NewRegistry(nil), so phase 2 (which skips built-ins and finds no other provider) returns \"\".
       Do NOT modify this case; ADD a separate test that registers a user-defined provider."

# MUST READ — the field under the predicate
- file: internal/provider/manifest.go
  why: TooledFlags []string (line 76, toml:"tooled_flags"). Confirms len()>0 is the right check.
  pattern: "TooledFlags []string `toml:\"tooled_flags\"`"

# Reference — the architecture spec for this exact fix
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/architecture/bugfix_subsystems.md
  section: "BUG-003 ... Fix Strategy ... 2. Let FirstTooledProvider also consider user-defined tooled
       providers: Add a secondary scan over user-defined providers with non-empty TooledFlags."
  why: Confirms S3 is option 2 of the BUG-003 fix (S1+S2 are option 1). Lists the test strategy.
```

### Current Codebase tree (relevant slice)

```bash
internal/provider/
  registry.go          # FirstTooledProvider @127 (EDIT); preferredBuiltins @16; Get/List/IsInstalled/NewRegistry
  registry_test.go     # TestFirstTooledProvider @355 (ADD a sibling test; leave existing unchanged)
  manifest.go          # TooledFlags []string @76 (the predicate field)
  builtin.go           # BuiltinManifests @18 (preferredBuiltins mirrors its keys — do not call per-invocation)
internal/decompose/
  roles.go             # ResolveRoles @108 — the ONLY caller (S1 Complete: fb==""→StagerAvailable=false)
  decompose.go         # runLoop guard consumes StagerAvailable (S2 — NOT this task; no overlap)
```

### Desired Codebase tree (files this task touches)

```bash
internal/provider/registry.go       # MODIFY FirstTooledProvider (add phase-2 scan + doc comment)
internal/provider/registry_test.go  # ADD TestFirstTooledProvider_UserDefinedFallback
```

### Known Gotchas of our codebase & Go quirks

```go
// CRITICAL 1 — preferredBuiltins IS the complete built-in set. The phase-2 "is this a built-in?" test
// MUST use a set built from preferredBuiltins (not BuiltinManifests(), which allocates a map per call).
// TestPreferredBuiltins_MatchesBuiltinKeys enforces preferredBuiltins == BuiltinManifests() keys.
builtins := make(map[string]struct{}, len(preferredBuiltins))
for _, name := range preferredBuiltins { builtins[name] = struct{}{} }

// CRITICAL 2 — List() returns built-ins AND user-defined (NewRegistry seeds built-ins first). Phase 2
// MUST skip built-ins so preference order holds (built-ins in pi-first order, then user-defined
// alphabetical) and a built-in is never re-emitted out of order. (Phase-1 exhaustion would technically
// make the skip redundant TODAY, but the explicit skip is robust to future preferredBuiltins drift
// and self-documents intent — prefer it over relying on exhaustion.)

// CRITICAL 3 — installed is a list of NAMES (computeInstalled appends m.Name). The present set built
// at the top of the function is keyed by name; reuse it in phase 2 (present[m.Name]). Do NOT call
// IsInstalled inside FirstTooledProvider (it's the caller's job — keeps the function pure/testable).

// CRITICAL 4 — the returned name flows to reg.Get(fb) in roles.go and its manifest becomes the stager.
// So the name MUST be Get()-able (guaranteed: drawn from r.List()) and the manifest MUST be tooled
// (guaranteed: the phase-2 filter is len(m.TooledFlags) > 0). Do NOT weaken the predicate.

// GOTCHA 5 — NewRegistry does NOT Validate/Resolve, and FirstTooledProvider reads only Name +
// TooledFlags. So the regression test can use a minimal Manifest{TooledFlags: []string{"--x"}}
// (NewRegistry sets manifest.Name from the map key). No Command/Detect needed.
```

---

## Implementation Blueprint

### Target: `FirstTooledProvider` (after edit)

```go
// FirstTooledProvider returns the first provider that the caller reports installed AND whose manifest
// has non-empty TooledFlags (i.e. can serve as the stager), or "" if none qualifies (FR-D4 — PRD §9.16).
// It scans in two phases, preserving preference order:
//  1. Built-ins first, in FR-D1 preference order (pi first) — mirrors DefaultProvider but adds the
//     TooledFlags filter. (Today pi, claude, and agy are stager-capable per builtin.go.)
//  2. BUG-003/FR-M13 (S3): if no built-in tooled provider is installed, widen the fallback to user-
//     defined (§12.8) providers with non-empty TooledFlags, scanned in registry List() order
//     (deterministic, ascending by Name). This lets a user-defined tooled provider serve as the
//     stager fallback so a non-disjoint decompose run need not be deferred to the fast-path.
//
// installed is the caller's list of installed provider NAMES (computed via IsInstalled over List() in
// decompose.computeInstalled). Taking it as a param keeps this pure/testable (no exec inside).
func (r *Registry) FirstTooledProvider(installed []string) string {
	present := make(map[string]struct{}, len(installed))
	for _, name := range installed {
		present[name] = struct{}{}
	}
	// Phase 1 — built-ins in FR-D1 preference order (pi first). UNCHANGED.
	for _, name := range preferredBuiltins {
		if _, ok := present[name]; !ok {
			continue
		}
		m, ok := r.Get(name)
		if !ok {
			continue
		}
		if len(m.TooledFlags) > 0 {
			return name
		}
	}
	// Phase 2 — BUG-003/FR-M13 (S3): user-defined tooled providers as the fallback. Built-ins are
	// skipped (already considered in preference order above); preferredBuiltins is the complete built-in
	// set (TestPreferredBuiltins_MatchesBuiltinKeys), so this set-membership check is exact.
	builtins := make(map[string]struct{}, len(preferredBuiltins))
	for _, name := range preferredBuiltins {
		builtins[name] = struct{}{}
	}
	for _, m := range r.List() {
		if _, isBuiltin := builtins[m.Name]; isBuiltin {
			continue
		}
		if _, ok := present[m.Name]; !ok {
			continue
		}
		if len(m.TooledFlags) > 0 {
			return m.Name
		}
	}
	return ""
}
```

### Target test: `TestFirstTooledProvider_UserDefinedFallback` (new; leave existing test unchanged)

```go
// TestFirstTooledProvider_UserDefinedFallback (BUG-003/FR-M13, S3): a user-defined (§12.8) provider
// with non-empty TooledFlags serves as the stager fallback when no built-in tooled provider is
// installed; built-ins remain preferred; empty-tooled user-defined providers are never selected.
func TestFirstTooledProvider_UserDefinedFallback(t *testing.T) {
	r := NewRegistry(map[string]Manifest{
		"custom-tooled": {TooledFlags: []string{"--allowed-tools", "git:*"}}, // stager-capable
		"custom-empty":  {},                                                  // nil TooledFlags — not capable
	})
	cases := []struct {
		name      string
		installed []string
		want      string
	}{
		{"user-defined tooled when no built-in installed", []string{"custom-tooled"}, "custom-tooled"},
		{"built-in still preferred over user-defined", []string{"custom-tooled", "pi"}, "pi"},
		{"user-defined tooled preferred over empty user-defined", []string{"custom-tooled", "custom-empty"}, "custom-tooled"},
		{"empty-tooled user-defined never selected", []string{"custom-empty"}, ""},
		{"nothing installed", nil, ""},
	}
	for _, c := range cases {
		if got := r.FirstTooledProvider(c.installed); got != c.want {
			t.Errorf("%s: FirstTooledProvider(%v) = %q, want %q", c.name, c.installed, got, c.want)
		}
	}
}
```

> NOTE: `NewRegistry(map[string]Manifest{...})` sets each manifest's `Name` from the map key, so the
> `{TooledFlags: ...}` literal is sufficient. Do NOT add a `strPtr`/Command helper — FirstTooledProvider
> never inspects Command/Detect (installed is caller-supplied).

### Implementation Tasks (ordered)

```yaml
Task 1: MODIFY internal/provider/registry.go — FirstTooledProvider
  - KEEP phase 1 (the existing preferredBuiltins loop) byte-for-byte unchanged.
  - ADD phase 2: build a `builtins` set from preferredBuiltins; iterate r.List(); skip built-ins;
    skip not-installed (present check); return first with len(m.TooledFlags) > 0.
  - UPDATE the doc comment to document the two-phase scan + the user-defined fallback (Mode A docs).
  - DO NOT touch preferredBuiltins, Get, List, IsInstalled, NewRegistry, DefaultProvider, or any other
    function. DO NOT call BuiltinManifests() (allocate-free set from preferredBuiltins instead).
  - NAMING: local `builtins` set; reuse the existing `present` set + `m` loop var idiom.

Task 2: ADD internal/provider/registry_test.go — TestFirstTooledProvider_UserDefinedFallback
  - PLACE: immediately after the existing TestFirstTooledProvider (line ~378), inside the same
    "FirstTooledProvider (FR-D4)" section banner.
  - FOLLOW pattern: the existing table-driven style (cases []struct{...}; range + t.Errorf on mismatch).
  - CONSTRUCT: NewRegistry(map[string]Manifest{"custom-tooled": {TooledFlags: ...}, "custom-empty": {}}).
  - COVER: fallback-hits, built-in-preferred (pi wins), empty-tooled-rejected, nothing-installed.
  - DO NOT modify the existing TestFirstTooledProvider (it stays green unchanged).
```

### Integration Points

```yaml
CALLER (read-only — S1 is Complete, do NOT edit roles.go):
  - internal/decompose/roles.go:147 — `fb := reg.FirstTooledProvider(installed)`. The widened return
    feeds S1's `else` branch (reg.Get(fb) → stager). No caller change: a user-defined name is already
    Get()-able and its manifest is tooled by construction.

SIBLING SCOPE (no overlap — respect the fences):
  - S1 (Complete): internal/decompose/roles.go (+roles_test.go) — StagerAvailable sentinel.
  - S2 (Implementing): internal/decompose/decompose.go (+decompose_test.go) — runLoop guard.
  - S3 (THIS TASK): internal/provider/registry.go (+registry_test.go) — FirstTooledProvider only.
  - Zero file overlap. S3 is independently correct: it widens a fallback; S1/S2 still function if S3
    returns "" (StagerAvailable=false path) or a name (the else path).

DATA:
  - No schema/migration change. TooledFlags is an existing []string field (manifest.go:76).
```

---

## Validation Loop

### Level 1: Build & Vet (immediate)

```bash
cd /home/dustin/projects/stagecoach
go build ./internal/provider/...        # compiles
go vet  ./internal/provider/...         # no vet issues (loop var capture etc.)
# Expected: clean. Fix any reported issue before proceeding.
```

### Level 2: Unit Tests (the gate)

```bash
# The new test + the existing one (must both pass).
go test ./internal/provider/ -run TestFirstTooledProvider -v
# Expected: TestFirstTooledProvider PASS (unchanged) + TestFirstTooledProvider_UserDefinedFallback PASS.

# Full provider package (regression — ensure List/Get/merge/etc. untouched).
go test ./internal/provider/ -v
# Expected: all PASS.
```

### Level 3: Consumer integration (S1's caller still sound with the widened return)

```bash
# ResolveRoles calls FirstTooledProvider; S1's roles_test.go must still pass.
go test ./internal/decompose/ -run 'TestResolveRoles|Stager' -v
# Expected: PASS — confirms the widened fallback doesn't break S1's stager resolution (including the
# S1 _Deferred case and any fallback-arm case). NOTE: if a roles_test.go case asserts a SPECIFIC
# user-defined-no-fallback outcome that S3 now changes, READ it — S3 is the intended enhancement and
# the assertion may need updating to reflect the new (correct) behavior. Surface this in the commit msg.

# Full decompose package (S1 + S2 surface — should be unaffected by a registry.go-only change,
# but run it to be sure the integration holds once S2 lands).
go test ./internal/decompose/ -v
```

### Level 4: Whole-repo smoke

```bash
go build ./...                           # nothing else broke
go test  ./... -count=1 2>&1 | tail -30  # full suite green (or only pre-existing unrelated failures)
```

---

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./internal/provider/...` clean.
- [ ] `go vet ./internal/provider/...` clean.
- [ ] `go test ./internal/provider/ -run TestFirstTooledProvider -v` — both tests PASS.
- [ ] `go test ./internal/provider/ -v` — full package PASS.
- [ ] `go test ./internal/decompose/ -run 'TestResolveRoles|Stager' -v` — consumer still sound.
- [ ] `go build ./...` clean.

### Feature Validation (Success Criteria)
- [ ] User-defined tooled provider returned when no built-in tooled provider is installed.
- [ ] Built-in preference preserved (pi/claude/agy always win over user-defined).
- [ ] Empty/nil-TooledFlags user-defined provider never selected.
- [ ] Doc comment documents the user-defined fallback scan (Mode A — no user-facing doc change).
- [ ] Existing `TestFirstTooledProvider` unchanged and green.

### Scope & Boundaries
- [ ] Only `internal/provider/registry.go` + `internal/provider/registry_test.go` changed.
- [ ] No edits to roles.go / decompose.go (S1/S2 domains) or any other package.
- [ ] No new dependencies (stdlib only); no schema/migration change.
- [ ] Commit message names the requirement brought into line: BUG-003 option 2 / FR-M13 / FR-D4.

---

## Anti-Patterns to Avoid

- ❌ Don't call `BuiltinManifests()` inside `FirstTooledProvider` to test "is built-in" — it allocates a map every call. Build a set from `preferredBuiltins` once (gotcha 1).
- ❌ Don't drop the built-in skip in phase 2 "because phase 1 already exhausted them" — the explicit skip preserves preference order robustly and self-documents (gotcha 2).
- ❌ Don't invent a new stager-capability predicate — `len(m.TooledFlags) > 0` is THE predicate (manifest.go:76); reuse it verbatim.
- ❌ Don't modify the existing `TestFirstTooledProvider` (its `myagent→""` case is still correct; it's a built-ins-only registry). Add a sibling test.
- ❌ Don't edit roles.go or decompose.go — those are S1 (Complete) and S2 (Implementing) domains; S3 is registry.go-only and feeds S1's consumer unchanged.
- ❌ Don't call `IsInstalled` inside `FirstTooledProvider` — it's the caller's job (computeInstalled); the function stays pure/testable by taking `installed` as a param.

---

## Confidence Score: 9/10

Small, fully-specified bugfix: one function gains a second scan loop + a doc-comment update + one
table-driven test. The contract is pinned by the current code (verbatim above), the sole consumer
(S1, Complete — the widened return is provably sound), the test pattern (existing sibling), and the
architecture doc (BUG-003 option 2). Stdlib-only; no external research value. Not 10/10 only because
S2 is still Implementing in parallel — but S3 is independently correct and has zero file overlap with
S2, so parallel landings compose (S3 widens the fallback; S2's runLoop guard fires less often; both
are correct alone or together).