# Research Notes — P1.M2.T1.S3 (FirstTooledProvider user-defined fallback)

Scope: widen `internal/provider/registry.go` `FirstTooledProvider` so a user-defined (§12.8) tooled
provider serves as the stager fallback when no built-in tooled provider is installed (BUG-003,
architecture option 2). Single .go edit + doc comment + one regression test. Stdlib-only.

## 1. Current implementation — registry.go:127-148 (the ENTIRE function)
```go
func (r *Registry) FirstTooledProvider(installed []string) string {
	present := make(map[string]struct{}, len(installed))
	for _, name := range installed {
		present[name] = struct{}{}
	}
	for _, name := range preferredBuiltins {
		if _, ok := present[name]; !ok { continue }
		m, ok := r.Get(name)
		if !ok { continue }
		if len(m.TooledFlags) > 0 { return name }
	}
	return ""
}
```
- `installed` is a list of provider NAMES (not manifests). Built by `computeInstalled` in roles.go:202
  (`IsInstalled` over `List()`). The `present` set is built once at the top — reuse it in phase 2.
- Only iterates `preferredBuiltins` = `["pi","opencode","cursor","agy","codex","claude"]` (registry.go:16).
- `preferredBuiltins` IS the complete built-in set: registry.go:11-15 comment + enforced by
  `TestPreferredBuiltins_MatchesBuiltinKeys`. So a set built from `preferredBuiltins` exactly identifies
  built-ins for the phase-2 skip — do NOT call `BuiltinManifests()` per-invocation (it allocates a map).

## 2. Consumer — roles.go ResolveRoles stager block (lines 108-178), S1 = Complete
- Line 109: `installed := computeInstalled(reg)` (computed once, shared by all 4 roles + FirstTooledProvider).
- Line 145-147 (S1's landed change): `if role == "stager" && len(m.TooledFlags) == 0 { fb := reg.FirstTooledProvider(installed); if fb == "" { stagerAvailable = false } else { fbm, ok := reg.Get(fb); ... prov, m = fb, fbm ... } }`
- `rm.StagerAvailable = stagerAvailable` set before return. S2's runLoop guard (`decompose.go`) consumes it.
- IMPLICATION for S3: the name S3 returns must be (a) Get()-able — guaranteed, it came from r.List();
  (b) its manifest must have non-empty TooledFlags — guaranteed by S3's filter. So S1's `else` arm
  (reg.Get(fb) + uses fbm as stager) is sound for any name S3 returns. S3 REDUCES how often the
  `fb==""` (StagerAvailable=false) arm fires: a user-defined tooled provider now satisfies it.

## 3. The secondary scan (phase 2) — design
- Iterate `r.List()` (returns built-ins AND user-defined, sorted ascending by Name — deterministic).
- SKIP built-ins (already considered in preference order in phase 1) — build a `builtins` set from
  `preferredBuiltins` once. This is defensive: even if preferredBuiltins ever diverged from
  BuiltinManifests, phase 2 never re-emits a built-in out of preference order.
- Check `present[m.Name]` (installed) AND `len(m.TooledFlags) > 0`; return first match.
- Preference preserved: built-ins (pi first) → user-defined (alphabetical by Name).
- ALTERNATIVE (viable, fewer lines): do NOT skip built-ins — phase-1 exhaustion guarantees no built-in
  can match in phase 2 (it proved none is installed+tooled). But the explicit skip is more obviously
  correct to a reader and robust to future preferredBuiltins drift. RECOMMEND the explicit skip.

## 4. Test site — registry_test.go:355 `TestFirstTooledProvider` (table-driven)
- Uses `r := NewRegistry(nil)` (built-ins only). Cases incl. `{[]string{"pi","claude"}, "pi"}`,
  `{[]string{"claude","agy"}, "agy"}` (agy IS stager-capable), `{[]string{"myagent"}, ""}`
  ("user-defined never auto-selected" — myagent is NOT in the registry, so still "" under S3), `{nil, ""}`.
- THIS TEST STAYS GREEN & UNCHANGED under S3: phase 2 over `NewRegistry(nil).List()` = 6 built-ins,
  all skipped → myagent/nil cases still return "".
- S3 ADDS a SEPARATE test `TestFirstTooledProvider_UserDefinedFallback` that constructs
  `NewRegistry(map[string]Manifest{...})` WITH user-defined providers (one tooled, one empty-tooled) and
  asserts: (a) user-defined tooled selected when no built-in installed; (b) built-in still preferred
  when a built-in tooled IS installed; (c) empty-tooled user-defined never selected.
- NewRegistry does NOT Validate/Resolve (registry.go doc), and FirstTooledProvider reads only Name +
  TooledFlags — so a minimal `{TooledFlags: []string{"--x"}}` manifest suffices (NewRegistry sets
  manifest.Name from the map key). No Command/Detect needed for this pure-function test.

## 5. Manifest field — manifest.go:76
- `TooledFlags []string toml:"tooled_flags"`. The existing `len(m.TooledFlags) > 0` check is the
  stager-capability predicate — reuse it verbatim in phase 2 (do not invent a new one).

## 6. Scope / no-overlap (from S1 & S2 PRPs)
- S1 (Complete) + S2 (Implementing) edit ONLY internal/decompose/roles.go + decompose.go respectively.
  BOTH explicitly fence off registry.go / FirstTooledProvider as S3's exclusive domain.
- S3 edits ONLY internal/provider/registry.go (+ registry_test.go). Zero file overlap with S1/S2.
- Mode A docs: update the FirstTooledProvider code comment (no user-facing doc surface change).
- No external libraries / APIs / patterns — pure stdlib slice + map work. External web research would
  add no value; this note captures the relevant in-repo contracts.