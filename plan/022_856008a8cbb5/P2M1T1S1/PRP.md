name: "P2.M1.T1.S1 — manifest EventStream field + render insertion + ProgressiveStdout arming + no-extractor guard (FR-S1)"
description: >
  The manifest+render half of the event-stream output mode (FR-S1; P2.M1.T1's S1). Four surgical additions to
  internal/provider + one docs row: (a) manifest.go — new `EventStream []string \`toml:"event_stream"\``
  field (nil ⇒ []), a SLICE (not pointer) following the BareFlags/TooledFlags pattern — no Validate enum
  (free token list), no Resolve touch (slices stay nil; consumers use len()); (b) merge.go — regime-2 slice
  merge (non-empty override REPLACES wholesale; nil/empty preserves base) so a user layer can set it;
  (c) render.go — in BOTH Render (insert between the mode-switch close ~:166 and the print_flag guard :167)
  and RenderMultiTurn (between sessionArgs append :281 and print_flag :283) append `r.EventStream...` AFTER
  the mode flag-set, STRICTLY BEFORE print_flag (§12.2 always-last invariant), and set
  `spec.ProgressiveStdout = len(r.EventStream) > 0` on the CmdSpec (the arming flag executor.go:144 already
  consumes: `Idle>0 && (linux || ProgressiveStdout)` — landed in P1.M2.T1.S2, previously never set by Render);
  (d) NEW sibling file internal/provider/events.go — the per-provider extractor REGISTRY TYPE
  (`type Extractor interface` + `map[string]Extractor` + Lookup/Register seam), EMPTY of real providers
  (Registry itself stays pure data); Render returns a HARD configuration error when a manifest declares
  non-empty event_stream for a provider with no registered extractor (D8: a test registers a fake to exercise
  the happy path). NO built-in declares event_stream (all six ship nil — FR-S2: never speculative; verified
  by a new builtin_test loop). Docs (Mode A): docs/providers.md "22 fields"→"23" (:13) + one event_stream
  table row after the session_mode row (:29) — type `list of string`, default `[]` (none). Consumed by
  P2.M1.T1.S2 (execution: NDJSON parse + terminal-event extraction) and populated by P2.M2.T1.S2–S5 after
  dated verification (builtin.go:61 precedent). Tests: manifest_test/merge_test field-merge regime;
  render_test token position (both functions) + ProgressiveStdout propagation + no-extractor error + fake-
  extractor happy path; builtin_test all-empty. File-disjoint from the parallel P1.M3.T1.S4 (tests-only,
  internal/generate/idle_ladder_test.go). Gates: go build/vet/gofmt + `go test ./internal/provider/... -race`
  + scope guard.

---

## Goal

**Feature Goal**: Add the `event_stream` manifest capability field, its render-time flag insertion and
`CmdSpec.ProgressiveStdout` arming, and the per-provider extractor registry type with a no-extractor hard
guard — so that a manifest CAN declare event-stream output, render emits its tokens in the correct position
with progressive-stdout armed, and an unverified provider (no extractor) is refused loudly rather than
silently producing unparseable output. This is the declarative half; P2.M1.T1.S2 adds the execution half
(NDJSON parse + terminal-event extraction) and P2.M2.T1.S2–S5 populate real extractors after dated
verification.

**Deliverable**: (1) manifest.go `EventStream []string` field; (2) merge.go regime-2 merge line; (3) render.go
insertion + guard + ProgressiveStdout arming in BOTH Render and RenderMultiTurn; (4) NEW
`internal/provider/events.go` (Extractor type + registry map + Lookup/Register); (5) docs/providers.md count
bump + one table row; (6) tests in manifest_test.go / merge_test.go / render_test.go / builtin_test.go.

**Success Definition**:
- `go test ./internal/provider/... -race` GREEN (all 4 new test groups pass; every existing render/merge/
  builtin/manifest test unchanged and passing — nil EventStream is invisible to every existing path).
- Render token order for a manifest with EventStream: `subcommand → provider-fold → model → reasoning → sys
  → mode flags (bare OR tooled) → event_stream → print_flag (LAST) → delivery` — in BOTH Render and
  RenderMultiTurn (asserted by index in tests).
- `CmdSpec.ProgressiveStdout == true` iff `len(m.EventStream) > 0` (both render paths); `false` for every
  existing manifest (all built-ins nil).
- A manifest with non-empty EventStream and NO registered extractor ⇒ Render (and RenderMultiTurn) return a
  HARD error naming the provider; with a registered fake extractor ⇒ renders cleanly (D8 happy path).
- `rg -n 'event_stream' providers/*.toml` returns ZERO hits (no built-in manifest declares it — FR-S2);
  builtin_test enforces all six ship nil.
- `git status --porcelain` == exactly: manifest.go, merge.go, render.go, events.go (new), docs/providers.md,
  manifest_test.go, merge_test.go, render_test.go, builtin_test.go (9 files).

## User Persona (if applicable)

**Target User**: A power user who configures a provider's `[provider.<name>]` block (S1 makes `event_stream`
a settable capability), and the stagecoach maintainer wiring per-provider event extraction (S2/P2.M2 consume
the registry). End users see nothing until P2.M2 populates a real provider.
**Use Case**: Once a provider's event mode is verified (P2.M2), its manifest declares `event_stream =
["--output-format", "stream-json"]`; stagecoach then renders those flags and arms the executor's
progressive-stdout activity channel, enabling idle-deadline detection on non-Linux platforms (FR-A) and
in-stream error surfacing (FR-S3). Until verification, the field is refused for that provider — never
speculative.
**Pain Points Addressed**: (1) Off-Linux, idle detection currently depends only on the pipe-channel arming
flag that nothing set — S1 is the missing producer; (2) a user who hand-sets event flags today would get
NDJSON on stdout that ParseOutput can't read — S1's no-extractor guard fails fast with an actionable error
instead of a garbled message/rescue.

## Why

- **FR-S1 (spec §9.30)**: the event-stream output mode is declared per-provider via a manifest capability
  field, exactly like `session_mode` before it (the established optional-capability pattern: manifest field
  → merge → render-time gate; provider_subsystem.md §3.3 documents this as THE template).
- **Completes the P1.M2 arming seam**: P1.M2.T1.S2 landed `CmdSpec.ProgressiveStdout` and the executor's
  `Idle>0 && (linux || ProgressiveStdout)` arming rule (executor.go:144), but nothing ever SET the flag —
  Render-side arming is this task's single purpose on that seam (`len(m.EventStream) > 0`).
- **FR-S2 anti-speculation**: the no-extractor guard + empty built-ins make it structurally impossible to
  ship an unverified event mode — the same discipline as session_mode's "never set speculatively" (FR-T9).
  P2.M2's verification harness then populates only what passes, with dated comments (builtin.go:61 style).
- **Non-breaking by construction**: nil EventStream (every existing manifest) adds zero tokens, sets
  ProgressiveStdout=false, and never consults the registry — all existing render paths are byte-identical.

## What

Four code additions + one docs row. No executor change (its consumer side landed in P1.M2). No built-in
declaration. No execution-path change (S2 owns NDJSON parsing).

### Success Criteria
- [ ] manifest.go: `EventStream []string \`toml:"event_stream"\`` field with a block comment (semantics:
      nil/empty = capability off; non-empty = verified JSON-event flags; "REQUIRES a registered per-provider
      extractor (FR-S2); never set speculatively" — mirroring the SessionMode :67 comment style).
- [ ] merge.go: regime-2 line in the slice block (~:89 area): `if len(override.EventStream) > 0 {
      out.EventStream = override.EventStream }` with the regime-2 comment.
- [ ] render.go Render: after the mode switch closes (~:166), before the print_flag guard (:167): the
      len>0 guard → extractor lookup (hard error if nil) → append tokens. CmdSpec gets
      `ProgressiveStdout: len(r.EventStream) > 0`.
- [ ] render.go RenderMultiTurn: the same insertion between the sessionArgs append (:281) and its print_flag
      guard (:283); ProgressiveStdout set identically.
- [ ] events.go (NEW): `Extractor` interface + unexported `eventExtractors` map (EMPTY) +
      `LookupEventExtractor(name)` + `RegisterEventExtractor(name, e)` + doc comments naming S1/S2/P2.M2 owners.
- [ ] builtin_test: loop over `BuiltinManifests()` asserting `len(m.EventStream) == 0` for all six (FR-S2).
- [ ] docs/providers.md: :13 "22 fields" → "23 fields"; one `event_stream` row after the session_mode row (:29).
- [ ] All gates green (build/vet/gofmt/`go test ./internal/provider/... -race`); scope == the 9 files.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact live line numbers for every edit (verified post-P1.M2), the SessionMode/TooledFlags
precedents to mirror, the verbatim insertion-point code, the Extractor shape + registry design rationale
(seam-var, not Registry member), the D8 fake-extractor test recipe, the FR-S2 empty-builtins invariant, the
docs row wording (type-corrected to `[]string`), and the sibling/parallel scope fences.

### Documentation & References

```yaml
# MUST READ — the findings (live line numbers + all design decisions).
- docfile: plan/022_856008a8cbb5/P2M1T1S1/research/findings.md
  why: "§0 the LIVE verified line numbers (provider_subsystem.md predates P1.M2 — its render line numbers
        shifted; mine are current); §1 every design decision with rationale (slice-not-pointer, no Validate
        enum, no Resolve touch, regime-2 merge, seam-var registry, empty-builtins); §2 the test matrix;
        §3 the docs row (final wording, type-corrected); §4 scope fence."

# MUST READ — the subsystem grounding report (the codebase bible for this package).
- docfile: plan/022_856008a8cbb5/architecture/provider_subsystem.md
  section: "§3.3 (SessionMode precedent — THE field template: block comment w/ semantics + 'never
            speculative', validate :129-131, Resolve :211-212); §4 (the three merge regimes — regime-2 slice
            wholesale-replace at :84-99, TooledFlags example); §5.3 (THE INSERTION POINT — the verbatim
            seam code + 'a v3.5 flag insertion must be mirrored in BOTH functions' + print_flag
            always-last invariant); §7.1/7.3 (BuiltinManifests fresh-map + the dated VERIFIED comment style);
            §7.4 (the event-extractor registry belongs in a NEW sibling file, NOT on Registry — 'pure data
            structure'); §8 (test conventions: stdlib testing, 'got %q, want %q', same-package)."
  gotcha: "Its render.go line numbers are PRE-P1.M2 (Render was :100; live is :106; seam ~:160/:161 → live
           :165-:167; RenderMultiTurn session seam :275/:277 → live :281/:283). Its §5.1 claim 'CmdSpec
           carries no mode/event metadata' is STALE — ProgressiveStdout landed. TRUST the doc's PATTERNS and
           verbatim seam code; use findings §0's live numbers."

# MUST READ — the files under edit (re-verify line numbers at impl; the TEXT anchors are stable).
- file: internal/provider/manifest.go
  why: "Field :67 SessionMode (comment style to mirror); Validate :129-131 (enum pattern — NOT needed here);
        Resolve :211-212 (pointer default pattern — NOT needed for a slice); header :28-36 design call
        (slices stay plain; nil = absent). Place EventStream after SessionMode with its own section comment."
- file: internal/provider/merge.go
  why: "Regime-2 block :79-91 (TooledFlags :89-90 is the line to mirror: len>0 ⇒ wholesale replace).
        Do NOT add a pointer regime — EventStream is a slice."
- file: internal/provider/render.go
  why: "Render :106; mode switch :158-166 (default case :164-165 appends r.BareFlags...); print_flag guard
        :167 (ALWAYS LAST). RenderMultiTurn :223; sessionArgs append :281; print_flag guard :283. CmdSpec
        construction at each delivery switch — add ProgressiveStdout there. spec := &CmdSpec{...} literal."
- file: internal/provider/executor.go
  why: "READ-ONLY (the consumer, landed P1.M2): :144 `if dl.Idle > 0 && (runtime.GOOS == \"linux\" ||
        spec.ProgressiveStdout)` — S1's arming feeds exactly this. DO NOT EDIT."
- file: internal/provider/builtin.go
  why: "READ-ONLY in S1: :61 the dated VERIFIED comment style P2.M2 will copy when populating; :18
        BuiltinManifests fresh-map pattern. NO provider declares the field — nothing to change."
- file: internal/provider/registry.go
  why: "READ-ONLY: deliberately a pure data structure — the extractor registry is a SIBLING (events.go),
        never a Registry method. provider_subsystem.md §7.4."

# MUST READ — the sibling tests to extend (same package, stdlib style).
- file: internal/provider/manifest_test.go   # Unmarshal full/partial-nil (:30,:111), Marshal omits (:202), Validate matrix (:239+)
- file: internal/provider/merge_test.go      # NonEmptySliceReplacesWholesale (:140), EmptyOrNilSlicePreservesBase (:188) — the exact shapes to follow
- file: internal/provider/render_test.go     # GoldenPerProvider (:30), mode tests (:364-:435), RenderMultiTurn table (:566-:744)
- file: internal/provider/builtin_test.go    # KeysAndCount (:199), per-provider field tests, FreshEachCall (:436)

# CONTEXT — the docs edit + the drift inventory.
- file: docs/providers.md
  why: ":13 the count sentence; :29 the session_mode row (mirror its shape). Insert the event_stream row
        directly after :29. NO new capability subsection in S1 (ships with P2.M2 population)."
- docfile: plan/022_856008a8cbb5/architecture/docs_drift_inventory.md
  section: "L243-253 (the manifest-table edit spec + a DRAFT row)."
  gotcha: "The draft row says type 'string', default '\"\"' — written BEFORE the slice decision. The REAL
           field is []string; the row MUST say 'list of string' / '[] (none)' (findings §3 final wording)."

# CONTEXT — FR-S1–S4 spec (the feature this implements).
- docfile: plan/022_856008a8cbb5/prd_snapshot.md
  section: "§9.30 (FR-S1 event-stream manifest field; FR-S2 verification duty — 'gated on an FR-T9-style
            verification'; FR-S3 in-stream error surfacing — S2's; FR-S4)."
- file: plan/022_856008a8cbb5/P1M3T1S4/PRP.md
  why: "The PARALLEL task (tests-only, internal/generate/idle_ladder_test.go). READ-ONLY context: confirms
        executor arming semantics + zero file overlap with S1. Do not touch its file."
```

### Current Codebase tree (relevant slice)

```bash
# EDIT:
internal/provider/manifest.go    # +EventStream field (after :67 SessionMode region)
internal/provider/merge.go       # +regime-2 line (~:89 area)
internal/provider/render.go      # +insertion+guard+arming in Render (~:166) AND RenderMultiTurn (~:282)
internal/provider/events.go      # NEW — Extractor + registry map (empty) + Lookup/Register
docs/providers.md                # :13 count 22→23; +1 row after :29
internal/provider/manifest_test.go   # +unmarshal/marshal/validate/resolve tests
internal/provider/merge_test.go      # +regime-2 tests (mirror :140/:188 shapes)
internal/provider/render_test.go     # +token-position/arming/guard/fake-extractor tests (both fns)
internal/provider/builtin_test.go    # +all-six-ship-empty loop
# READ-ONLY (do NOT edit):
internal/provider/executor.go    # consumer landed (P1.M2) — :144 arming
internal/provider/registry.go    # pure data — the extractor map is a sibling, not a member
internal/provider/builtin.go     # no provider declares the field (FR-S2)
providers/*.toml                 # reference manifests — NO event_stream anywhere
internal/generate/idle_ladder_test.go  # the PARALLEL task's file
```

### Desired Codebase tree with files to be added/edited

```bash
internal/provider/events.go   # NEW — type Extractor interface{ Extract(line string) (message string, terminal bool, err error) };
                              #      var eventExtractors = map[string]Extractor{} (EMPTY — FR-S2);
                              #      func LookupEventExtractor(name string) Extractor;
                              #      func RegisterEventExtractor(name string, e Extractor);
                              #      doc: registry owned by P2.M2 population; seam-var idiom (tests mutate+restore)
# + the 8 edits above (manifest.go, merge.go, render.go, docs/providers.md, 4 test files)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (print_flag is ALWAYS LAST — §12.2): the event_stream tokens insert AFTER the mode flag-set and
// STRICTLY BEFORE the print_flag guard in BOTH Render and RenderMultiTurn. pi treats a trailing -p as the
// print toggle; anything displacing it breaks every print-mode provider. The insertion is between the
// switch's closing brace and `if *r.PrintFlag != "" {`.

// CRITICAL (EventStream is a SLICE — do NOT follow SessionMode's pointer pattern): slices in this codebase
// stay plain (manifest.go header design call); nil is the absent sentinel. No Validate enum (it's a free
// token list — TooledFlags precedent has none), no Resolve touch (slices left as-is). Always test
// len(m.EventStream) > 0, never != nil.

// CRITICAL (regime-2 merge limitation): an empty `[]` override CANNOT clear a non-empty base (documented
// regime-2 semantics — same as BareFlags/TooledFlags). That is the contract's chosen regime; do not invent
// a pointer-to-slice or sentinel to work around it.

// CRITICAL (the no-extractor guard fires in Render, not Validate): mirrors how tooled-mode-with-no-tooled-
// flags errors at render (:155) and session_mode gates at :225 — capability gates live at render time so a
// partial user override still Validates. The error must name m.Name and say the field requires a verified
// extractor (FR-S2).

// CRITICAL (lookup keys on m.Name — the manifest's own name): the registry is map[providerName]Extractor;
// user-defined providers get their own names and therefore their own (currently nonexistent) extractors —
// a user manifest setting event_stream on any name without a registered extractor hits the guard. That is
// correct FR-S2 behavior, not a bug.

// GOTCHA (tests are same-package — the registry seam needs no injector): render_test.go is package
// provider; tests assign eventExtractors directly (or via RegisterEventExtractor) and restore via
// t.Cleanup. No mutex needed (sequential tests). Do NOT add an exported setter-for-tests abstraction.

// GOTCHA (go-toml omits nil slices automatically): MarshalTOML parity is free (nil slice → key omitted);
// no omitempty exists in go-toml/v2 and none is needed for slices.

// GOTCHA (docs row type must be []string): the drift-inventory draft row says "string"/"" — stale. The row
// must say `list of string` / `[] (none)` to match the real field (findings §3).

// GOTCHA (line numbers are post-P1.M2 live): provider_subsystem.md's render numbers are stale (Render :100
// → live :106). Re-verify with grep before editing; anchor edits on TEXT (`args = append(args,
// r.BareFlags...)` default case + following `if *r.PrintFlag != \"\"`), not line numbers.
```

## Implementation Blueprint

### Data models and structure

```go
// manifest.go — after the SessionMode field block (:67 area):
// --- event-stream output (v3.5; §9.30 FR-S1) ---
// EventStream carries the provider's VERIFIED JSON-event output flags (e.g. pi "--mode json",
// claude "--output-format stream-json"). nil/empty = capability off (every built-in today).
// Non-empty: Render appends the tokens after the mode flag-set (before print_flag) and arms
// CmdSpec.ProgressiveStdout. REQUIRES a registered per-provider event extractor (events.go);
// Render errors when the field is set for a provider without one. FR-S2: never set speculatively —
// P2.M2 populates only after dated end-to-end verification (builtin.go:61 comment style).
EventStream []string `toml:"event_stream"`

// events.go (NEW, package provider):
// Extractor is the per-provider terminal-event scanner. S1 defines the shape; P2.M1.T1.S2
// implements the NDJSON parsing + FR-S3 error surfacing; P2.M2.T1.S2–S5 register the verified
// per-provider instances (initially the map is EMPTY — FR-S2).
type Extractor interface {
	// Extract scans one raw stdout line. terminal=true marks the final assistant message;
	// message is its text. err wraps a malformed-event condition (S2's semantics).
	Extract(line string) (message string, terminal bool, err error)
}

var eventExtractors = map[string]Extractor{} // keyed by provider Name; EMPTY in S1 (FR-S2)

func LookupEventExtractor(name string) Extractor { return eventExtractors[name] }
func RegisterEventExtractor(name string, e Extractor) { eventExtractors[name] = e } // P2.M2 seam

// render.go — Render, between the mode switch's close and the print_flag guard:
if len(r.EventStream) > 0 {
	if LookupEventExtractor(m.Name) == nil {
		return nil, fmt.Errorf("provider manifest %q: event_stream is set but no event extractor is registered for this provider (FR-S1/FR-S2): event_stream must not be set for an unverified provider", m.Name)
	}
	args = append(args, r.EventStream...)
}
// ... existing print_flag guard (LAST) ...
// and on the CmdSpec literal at the delivery switch:
spec := &CmdSpec{..., ProgressiveStdout: len(r.EventStream) > 0}
// RenderMultiTurn: the SAME block between `args = append(args, sessionArgs...)` and its print_flag guard.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/provider/events.go (the registry type)
  - The Extractor interface, the EMPTY eventExtractors map, LookupEventExtractor, RegisterEventExtractor,
    and file doc comments naming the owners (S1 shape / S2 semantics / P2.M2 population) + FR refs.
  - NAMING/PLACEMENT: events.go, package provider, sibling of render.go (per provider_subsystem §7.4).

Task 2: EDIT internal/provider/manifest.go (the field)
  - Add the EventStream []string field + section comment + block doc (the blueprint text above), placed
    after the SessionMode field block. FOLLOW the struct's section-comment style.
  - DEPENDENCIES: none (Task 3-4 consume it). NO Validate/Resolve changes (see gotchas).

Task 3: EDIT internal/provider/merge.go (regime-2)
  - In the regime-2 slice block (~:79-91, beside TooledFlags :89-90): add
    `if len(override.EventStream) > 0 { out.EventStream = override.EventStream }` + the regime-2 comment.
  - DEPENDENCIES: Task 2.

Task 4: EDIT internal/provider/render.go (insertion + guard + arming, BOTH functions)
  - Render: insert the len>0 → Lookup → (nil ⇒ hard error) → append block between the mode switch's
    closing brace and the print_flag guard (~:166/:167). Add ProgressiveStdout to the CmdSpec literal.
  - RenderMultiTurn: mirror identically between the sessionArgs append (:281) and its print_flag guard (:283).
  - PRESERVE: print_flag LAST; the FR-R5b fold; the delivery switch; PayloadBytes; Env — untouched.
  - DEPENDENCIES: Tasks 1-2.

Task 5: EDIT docs/providers.md (Mode A)
  - :13 "22 fields" → "23 fields". Insert the event_stream row after :29 (findings §3 wording —
    type `list of string`, default `[]` (none), notes the extractor requirement + FR-S2 empty built-ins).
  - DEPENDENCIES: Task 2 (the field exists).

Task 6: TESTS (4 files, same package, stdlib style — mirror the named neighbor tests)
  - manifest_test.go: unmarshal slice / absent→nil / marshal-omits-nil / Validate-tolerant(nil + tokens).
  - merge_test.go: regime-2 replace-wholesale / nil-preserves / empty-slice-preserves (mirror :140/:188).
  - render_test.go: (1) bare-mode token position (after bare_flags, before print_flag — index assertions);
    (2) tooled-mode position (after tooled_flags); (3) RenderMultiTurn position (after sessionArgs, before
    print_flag); (4) ProgressiveStdout true-when-set / false-when-nil in BOTH functions; (5) no-extractor
    hard error (names the provider); (6) D8 happy path — RegisterEventExtractor(t, name, fakeExtractor) via
    t.Cleanup restore; (7) nil-EventStream regression (existing golden paths unchanged).
  - builtin_test.go: loop over BuiltinManifests() — all six ship len(EventStream)==0 (FR-S2).
  - DEPENDENCIES: Tasks 1-4.

Task 7: VERIFY — build/vet/gofmt, `go test ./internal/provider/... -race`, `rg -n 'event_stream'
  providers/*.toml` (zero), scope guard (9 files).
```

### Implementation Patterns & Key Details

```go
// PATTERN (render guard — mirrors the tooled-empty gate :155 and session gate :225):
if len(r.EventStream) > 0 {
	if LookupEventExtractor(m.Name) == nil {
		return nil, fmt.Errorf("provider manifest %q: event_stream set but no extractor registered (FR-S2: never speculative)", m.Name)
	}
	args = append(args, r.EventStream...)
}

// PATTERN (test fake — D8):
type fakeExtractor struct{}
func (fakeExtractor) Extract(string) (string, bool, error) { return "fake", true, nil }
// in the test: RegisterEventExtractor(m.Name, fakeExtractor{}); t.Cleanup(func(){ delete(eventExtractors, m.Name) })
// (same package — direct map access; restore via delete, the seam-var idiom)

// CRITICAL (both render paths get the IDENTICAL block + arming): Render and RenderMultiTurn are siblings
// by design; a flag added to one and not the other is the classic divergence bug (provider_subsystem §5.3).
```

### Integration Points

```yaml
CONSUMES (READ-ONLY):
  - CmdSpec.ProgressiveStdout + executor.go:144 arming (landed P1.M2.T1.S2) — S1 is the producer.
  - provider_subsystem.md §3.3/§4/§5.3/§7.4 — the field/merge/insertion/registry patterns.
PRODUCES (for siblings):
  - P2.M1.T1.S2 (execution): consumes EventStream via spec.ProgressiveStdout + the Extractor interface +
    LookupEventExtractor to drive NDJSON parsing + FR-S3 error surfacing.
  - P2.M2.T1.S2–S5 (verification/population): RegisterEventExtractor + the builtin.go field declaration +
    dated VERIFIED comments (builtin.go:61 style). The map is EMPTY until then (by design).
NO executor/registry/builtin/generate changes. NO config-schema change (the field rides [provider.*]
  overrides through the existing DecodeUserOverrides → MergeManifest path — a []string TOML key decodes
  with zero loader changes).
```

## Validation Loop

### Level 1: Build & static (Immediate Feedback)

```bash
go build ./...
go vet ./internal/provider/...
gofmt -l internal/provider/ | grep -v '^$' && echo "FAIL: unformatted" || echo "OK"
# Expected: all clean.
```

### Level 2: Unit tests (the contract's test matrix)

```bash
go test ./internal/provider/ -race -run 'EventStream|EventExtractor|RenderMultiTurn' -v   # the new tests
go test ./internal/provider/... -race                                                    # the WHOLE package — proves
# Expected: all green, including every pre-existing golden test (nil EventStream invisible).
```

### Level 3: FR-S2 + position gates (deterministic greps/assertions)

```bash
rg -n 'event_stream' providers/ && echo "FAIL: a built-in declares event_stream (FR-S2)" || echo "OK: none"
rg -c 'ProgressiveStdout' internal/provider/render.go   # Expected: >=2 (both render paths set it)
rg -n 'event_stream' docs/providers.md                  # Expected: the table row (1 hit)
# (the builtin_test loop + render_test index assertions are the real FR-S2/position gates)
```

### Level 4: Scope guard

```bash
git status --porcelain   # Expected EXACTLY: internal/provider/{events.go,manifest.go,merge.go,render.go,
                         # manifest_test.go,merge_test.go,render_test.go,builtin_test.go}, docs/providers.md
git status --porcelain | grep -E 'executor\.go|registry\.go|builtin\.go|providers/.*toml|internal/generate|idle_ladder|spec/|PRD|plan/' && echo "FAIL: out of scope" || echo "OK"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` / `go vet ./internal/provider/...` / gofmt clean
- [ ] `go test ./internal/provider/... -race` GREEN (new + all existing)
- [ ] Level 3 greps: no providers/*.toml event_stream; render.go sets ProgressiveStdout in both paths; docs row present

### Feature Validation
- [ ] Field + regime-2 merge + render insertion (BOTH functions, correct position vs mode-flags/print_flag)
- [ ] `CmdSpec.ProgressiveStdout == (len(EventStream) > 0)` in Render AND RenderMultiTurn
- [ ] No-extractor hard error names the provider; D8 fake-extractor happy path passes
- [ ] All six built-ins ship empty (builtin_test; providers/*.toml grep)
- [ ] docs/providers.md: 23 fields + the event_stream row (type `list of string`)

### Scope-Boundary Validation
- [ ] executor.go / registry.go / builtin.go / providers/*.toml UNTOUCHED
- [ ] No internal/generate change (S2's execution path); idle_ladder_test.go untouched (parallel task)
- [ ] No spec/ / PRD / plan/ edit; no new capability subsection in docs (P2.M2's)

---

## Anti-Patterns to Avoid

- ❌ Don't make EventStream a `*[]string` or add a Validate enum — it's a plain slice like BareFlags/
  TooledFlags; nil is the absent sentinel; capability gates live at render time (the :155/:225 precedents).
- ❌ Don't insert the tokens anywhere except between the mode-switch close and the print_flag guard —
  print_flag is ALWAYS LAST (§12.2); displacing it breaks print-mode providers.
- ❌ Don't add the block to Render only — RenderMultiTurn must mirror it (the sibling-divergence trap).
- ❌ Don't put the extractor registry on Registry (it's deliberately pure data) or export a
  setter-for-tests abstraction — a sibling file + unexported map + same-package test mutation is the idiom.
- ❌ Don't populate any real extractor or declare the field on any built-in — FR-S2: never speculative;
  P2.M2 owns population with dated verification comments.
- ❌ Don't skip the no-extractor guard because the map is empty — the guard IS the FR-S2 enforcement
  (it's why an empty map fails loudly instead of emitting unparseable NDJSON).
- ❌ Don't use the drift-inventory draft row verbatim — its type column ("string"/`""`) predates the slice
  decision; the row must read `list of string` / `[] (none)`.
- ❌ Don't edit executor.go to "help" the arming — the consumer landed in P1.M2; S1 is producer-side only.
- ❌ Don't anchor edits on provider_subsystem.md's line numbers — they're pre-P1.M2; grep the live file and
  anchor on text.

---

## Confidence Score: 9/10

Every edit point is verified against the LIVE post-P1.M2 code with exact line numbers and stable text
anchors; the field/merge/insertion/registry designs each follow a documented in-repo precedent (SessionMode
comment style, TooledFlags regime-2, the §5.3 seam quote, builtin.go's fresh-map + the §7.4 sibling-file
recommendation); the tests are specified against named neighbor tests; and the design is non-breaking by
construction (nil field = zero behavior change). The 1-point deduction: the `Extractor` interface shape is
S1's best-judgment placeholder for S2's semantics (documented as such, and only the registry+guard are the
stable contract — a signature refinement in S2 is cheap but nonzero churn).
