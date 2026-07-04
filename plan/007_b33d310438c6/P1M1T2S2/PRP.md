---
name: "P1.M1.T2.S2 — Map cfg fields at the 6 production call-site struct literals"
description: |
  Thread `TokenLimit` + `DiffContext` from `config.Config` into the `git.StagedDiffOptions{...}`
  literal at all 6 production call sites, so the FR3d (token_limit) and FR3f (diff_context) knobs flow
  from config to the diff functions. The 6 sites: internal/generate/generate.go:163 (StagedDiff),
  internal/hook/exec.go:104 (StagedDiff), pkg/stagehand/stagehand.go:423 (StagedDiff),
  internal/decompose/planner.go:69 (TreeDiff), internal/decompose/message.go:71 (TreeDiff),
  internal/decompose/decompose.go:608 (TreeDiff). Two variable shapes: sites 1-3 use a local
  `cfg config.Config`; sites 4-6 use `deps.Config`.
  CRITICAL: `config.DiffContext` is `*int` but `StagedDiffOptions.DiffContext` is plain `int` (S1, LANDED),
  so a literal `DiffContext: cfg.DiffContext` is a TYPE ERROR. S1's struct doc mandates "the call site
  dereferences with a default-1 fallback." Therefore add ONE resolver method `Config.DiffContextValue() int`
  (nil→1, *0→0, *n→n) to internal/config/config.go and map `DiffContext: <cfg>.DiffContextValue()` at all
  6 sites. `TokenLimit` maps directly (`TokenLimit: <cfg>.TokenLimit`). Do NOT set PromptReserveTokens
  (leave zero; M4.T1.S2 wires it). The 3 fields are UNREAD by the diff functions until M2/M4, so this is
  behavior-free — all existing tests pass unchanged. Plus a 3-case unit test for the resolver.
---

## Goal

**Feature Goal**: Complete the config→diff-options seam for the two resolved v2.1 knobs
(`token_limit` / FR3d, `diff_context` / FR3f) by populating `TokenLimit` and `DiffContext` on the
`git.StagedDiffOptions` struct literal at all 6 production call sites. After this task, a user setting
`token_limit` / `diff_context` in config has those values flowing into the diff functions' option
struct (ready for M2 to read `DiffContext` → `-U<n>` and M4 to read `TokenLimit` → the gate/water-fill).

**Deliverable**:
1. **ADD** `func (c Config) DiffContextValue() int` to `internal/config/config.go` (resolves the `*int`
   `DiffContext` to a plain int: nil → default 1, `*0` → 0, `*n` → n).
2. **MODIFY** 6 production call sites — append `TokenLimit` + `DiffContext` to each
   `git.StagedDiffOptions{...}` literal (keep the existing 4 fields byte-identical):
   - `internal/generate/generate.go:163` (StagedDiff, `cfg`)
   - `internal/hook/exec.go:104` (StagedDiff, `cfg`)
   - `pkg/stagehand/stagehand.go:423` (StagedDiff, `cfg`)
   - `internal/decompose/planner.go:69` (TreeDiff, `deps.Config`)
   - `internal/decompose/message.go:71` (TreeDiff, `deps.Config`)
   - `internal/decompose/decompose.go:608` (TreeDiff, `deps.Config`)
3. **ADD** `TestDiffContextValue` to `internal/config/config_test.go` (nil→1, `*0`→0, `*3`→3).

**Success Definition**: `go build/vet/gofmt` clean; `go test ./...` green (existing suites unchanged —
the new option fields are unread by the diff functions, so diff output is byte-identical); each of the 6
literals carries `TokenLimit` + `DiffContext`; `PromptReserveTokens` is left at zero everywhere;
`Config.DiffContextValue()` resolves nil→1 and preserves an explicit `*0`.

## User Persona

**Target User**: The contributors implementing the downstream diff-payload tasks — M2.T2 (reads
`opts.DiffContext` → injects `-U<DiffContext>`) and M4.T3/T2 (read `opts.TokenLimit` → the gate +
water-fill). After this task those values are populated and waiting.

**Use Case**: A user sets `token_limit = 120000` and `diff_context = 0` in `.stagehand.toml`. Config
resolves them to `cfg.TokenLimit = 120000` (plain int) and `cfg.DiffContext = *0` (pointer). At each of
the 6 diff call sites, the literal now carries `TokenLimit: 120000` and `DiffContext: 0` (via
`DiffContextValue()` preserving the `*0`). M2 then emits `-U0`; M4 then runs the water-fill.

**User Journey**: `config.toml` → `Defaults()`+`materialize()`+`overlay()` → `config.Config{TokenLimit,
DiffContext *int}` → **(this task)** 6 call sites map into `git.StagedDiffOptions{TokenLimit,
DiffContext}` → diff functions (unread until M2/M4).

**Pain Points Addressed**: Closes the last gap between the landed config knobs (P1.M1.T1) and the landed
struct fields (S1) — without this, the struct fields stay zero at every call site and M2/M4 have nothing
to read. Centralizes the `*int` nil→1 resolution in ONE method instead of duplicating it across 6 sites
(where a forgotten guard would nil-deref or silently drop `-U0`).

## Why

- **PRD §9.1 FR3d/FR3f are the knobs.** FR3d (`token_limit` holistic overlay; 0/unset ⇒ legacy
  per-section caps) and FR3f (`diff_context` reduced `-U<n>`, 0–3, default 1; `0` = changed-lines-only).
  Both ride on `StagedDiffOptions` (consumed by all three diff paths — FR3c parity). This task populates
  them at every call site so the values reach the diff functions.
- **S1 explicitly delegates the resolution to the call site.** S1's `StagedDiffOptions.DiffContext` doc
  comment (LANDED) states: *"callers MUST pass the resolved context (default 1 when the user omits it)
  explicitly … the call site dereferences with a default-1 fallback before constructing this struct."*
  This task IS that dereference. `config.DiffContext` is `*int` precisely so the config layer can
  distinguish "unset" (nil → default 1) from "explicit 0" (`*0` → `-U0`); the resolver method carries
  that distinction through to the plain-int field.
- **Unblocks M2/M4 cleanly.** With `TokenLimit`/`DiffContext` populated at all 6 sites, M2 (FR3f
  `-U<n>`) and M4 (FR3d gate + FR3i water-fill) can read `opts.DiffContext`/`opts.TokenLimit` knowing
  the resolved values are there — no per-call-site resolution logic for them to duplicate.
- **Behavior-free by construction.** S1 landed the 3 fields as UNREAD seam-threaders (the diff functions
  don't read them until M2/M4). Populating them changes no diff output — every existing golden test
  passes unchanged. The only new logic is the resolver method (covered by its own unit test).
- **No user-facing/docs surface (contract: "DOCS: none — internal plumbing").**

## What

Add a resolver method, then append two fields to each of 6 struct literals. Specifically:

1. **`Config.DiffContextValue() int`** — the `*int` → `int` resolver with the nil→1 default (FR3f).
   Added to `internal/config/config.go` next to the existing `intPtr` helper. Value receiver.
2. **6 literal edits** — at each `git.StagedDiffOptions{...}`:
   - `TokenLimit: <cfg|deps.Config>.TokenLimit,` (direct — both are plain int)
   - `DiffContext: <cfg|deps.Config>.DiffContextValue(),` (the resolver call)
   - The existing `MaxDiffBytes`/`MaxMDLines`/`BinaryExtensions`/`Excludes` mappings stay byte-identical.
   - `PromptReserveTokens` is NOT set (Go zero-value = 0; M4.T1.S2 wires it).
3. **One unit test** — `TestDiffContextValue` (nil→1, `*0`→0, `*3`→3).

### Success Criteria

- [ ] `internal/config/config.go` has `func (c Config) DiffContextValue() int` (nil→1, non-nil verbatim).
- [ ] All 6 production literals carry `TokenLimit` + `DiffContext` (sourced correctly per shape).
- [ ] `PromptReserveTokens` is NOT set at any of the 6 sites (left zero).
- [ ] The existing 4 fields at each literal (`MaxDiffBytes`/`MaxMDLines`/`BinaryExtensions`/`Excludes`) unchanged.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean.
- [ ] `go test ./...` green — existing suites unchanged (diff functions don't read the new fields yet).
- [ ] `TestDiffContextValue` passes (nil→1, `*0`→0, `*3`→3).
- [ ] No change to `StagedDiffOptions` struct, the 3 diff functions, the `Git` interface, or config materialize/overlay/Defaults.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes all 6 call sites verbatim (current code, both variable shapes),
the exact resolver method body, the exact 2-line addition per site (with the per-shape receiver name),
the verified type facts (`config.DiffContext` is `*int`; `StagedDiffOptions.DiffContext` is plain `int`),
the explicit S1 mandate to dereference, and the "fields unread → behavior-free" guarantee. The only
inference is gofmt alignment. No guesswork.

### Documentation & References

```yaml
# MUST READ — the binding knob specs + the authoritative seam
- file: PRD.md
  why: "§9.1 FR3d (token_limit holistic overlay; 0/unset⇒legacy caps; mutually exclusive with per-section
        caps) and FR3f (diff_context reduced -U<n>, 0–3, default 1; 0 = changed-lines-only). These two FRs
        ARE the field semantics being threaded. FR3c (binary filtering / every diff path) is why all 6
        sites (3 StagedDiff + 3 TreeDiff) take the SAME StagedDiffOptions struct."
  critical: "FR3f's '0 = changed lines only' is why DiffContext==0 is VALID and must be preserved by the
             resolver (a nil-guard that defaulted *0→1 would be a bug). FR3d's '0/unset ⇒ legacy caps' is
             why TokenLimit maps directly (0 IS its unset sentinel)."

- docfile: plan/007_b33d310438c6/architecture/diff_capture_touchmap.md
  why: "§2 is the authoritative 6-site map: the table (file:line, function, method called), the
        confirmation that there is NO central bridge function (each site maps cfg→opts inline), and that
        all 6 take git.StagedDiffOptions (even the TreeDiff sites — there is no separate TreeDiffOptions).
        §4 states the bridge-function refactor is OPTIONAL/future, not this task."
  critical: "§2's representative literal + 'The new TokenLimit/DiffContext map from cfg.TokenLimit/
             cfg.DiffContext' is the task spec. NOTE the touchmap predates the *int decision — it writes
             cfg.DiffContext as if plain int; the LIVE config (P1.M1.T1.S2) made it *int, so the resolver
             (this PRP) is the faithful realization, not the touchmap's shorthand."

- docfile: plan/007_b33d310438c6/P1M1T2S1/PRP.md
  why: "The CONTRACT for the struct being populated: StagedDiffOptions has TokenLimit int, DiffContext int
        (PLAIN int — the resolved value), PromptReserveTokens int. S1 is LANDED (verified in the live
        struct). S1's DiffContext doc comment EXPLICITLY mandates: 'the call site dereferences with a
        default-1 fallback before constructing this struct' — that sentence is this task's core instruction."
  critical: "S1 made StagedDiffOptions.DiffContext a plain int ON PURPOSE (the git layer takes resolved
             values). The *int→int dereference is the CALL SITE's job — THIS task. Do NOT push *int into
             the struct or the git layer. S1 also says the fields are UNREAD until M2/M4 — confirms
             behavior-free."

- docfile: plan/007_b33d310438c6/P1M1T1S2/research/  (and config.go/file.go live code)
  why: "The config source is LANDED: config.TokenLimit int (config.go:81) + config.DiffContext *int
        (config.go:82, pointer). Defaults() sets TokenLimit:0, DiffContext:intPtr(1) (config.go:174-175).
        materialize (file.go:226) and overlay (file.go:340) guard DiffContext with `!= nil` (NEVER `!= 0`)
        — the pattern the resolver mirrors."
  critical: "config.DiffContext is *int so an explicit 0 (-U0) is distinguishable from unset (nil→1). The
             resolver MUST replicate this: nil→1, *0→0 (NOT *0→1). Verified by
             TestMaterializeOverlay_DiffContext_TokenLimit (file_test.go:814)."

- docfile: plan/007_b33d310438c6/P1M1T2S2/research/call_site_mapping_notes.md
  why: "THIS task's research: all 6 sites quoted verbatim (both variable shapes), the *int type-mismatch
        gotcha, the resolver decision (method on Config, value receiver, placed next to intPtr), the
        per-site exact additions, the behavior-free guarantee, and decisions D1–D7."
  critical: "§1.3 (the type mismatch) and §3 (the resolver body + the 6-site mapping) are the copy-paste
             source. §4 (behavior-free) explains why no existing test changes. §5 is the do-NOT-do list."

- file: internal/config/config.go
  why: "EDIT TARGET #1 (the resolver method). TokenLimit (line 81, plain int) + DiffContext (line 82,
        *int) + Defaults() (line 174-175) are the verified source. The helper intPtr (line 11) is the
        placement neighbor for the new method. Currently config.go has only free functions (no Config
        methods) — DiffContextValue will be the first, with a value receiver."
  pattern: "Free-function helpers at the top (boolPtr/strPtr/intPtr, lines 7-11). Add the method near
            them (or just after the Config struct / before Defaults). Value receiver: func (c Config)."
  gotcha: "Config is passed BY VALUE at all 6 call sites (cfg config.Config / deps.Config) — so a value
           receiver works everywhere. Do NOT change TokenLimit/DiffContext field types or
           materialize/overlay/Defaults (P1.M1.T1 COMPLETE)."

- file: internal/generate/generate.go
  why: "EDIT TARGET #2 (site 1). Line ~163: the StagedDiff literal inside CommitStaged. Uses local `cfg`."
  pattern: "Append TokenLimit: cfg.TokenLimit, and DiffContext: cfg.DiffContextValue(), after the existing
            4 fields (MaxDiffBytes/MaxMDLines/BinaryExtensions/Excludes). Keep gofmt alignment."
- file: internal/hook/exec.go
  why: "EDIT TARGET #3 (site 2). Line ~104: the StagedDiff literal inside Run. Uses local `cfg`."
- file: pkg/stagehand/stagehand.go
  why: "EDIT TARGET #4 (site 3). Line ~423: the StagedDiff literal inside runPipeline. Uses local `cfg`.
        NOTE: pkg/stagehand already imports internal/config (uses cfg.Output/cfg.StripCodeFence at :379-383)
        — so cfg.DiffContextValue() resolves without a new import."
- file: internal/decompose/planner.go
  why: "EDIT TARGET #5 (site 4). Line ~69: the TreeDiff literal inside callPlanner. Uses `deps.Config`."
- file: internal/decompose/message.go
  why: "EDIT TARGET #6 (site 5). Line ~71: the TreeDiff literal inside generateMessage. Uses `deps.Config`."
- file: internal/decompose/decompose.go
  why: "EDIT TARGET #7 (site 6). Line ~608: the TreeDiff literal inside runArbiterPhase. Uses `deps.Config`."
  gotcha: "Sites 4-6 access fields via deps.Config (NOT a local cfg). The addition is
           TokenLimit: deps.Config.TokenLimit, + DiffContext: deps.Config.DiffContextValue(),. The
           internal/decompose package already imports internal/config (uses config.ResolveRoleModel) —
           deps.Config.DiffContextValue() resolves without a new import."

# External references
- url: https://go.dev/ref/spec#Method_sets
  why: "Confirms a value receiver `(c Config)` is callable on both a Config value and a dereferenced
        *Config — all 6 sites hold/pass Config by value, so `cfg.DiffContextValue()` and
        `deps.Config.DiffContextValue()` both compile. (Pointer receiver would still work on value via
        auto-addressing only when the value is addressable; a value receiver is the safe, unambiguous choice.)"
```

### Current Codebase Tree (relevant slice — S1 LANDED, P1.M1.T1 COMPLETE)

```bash
stagehand/
└── internal/
    ├── config/
    │   ├── config.go          # EDIT: +Config.DiffContextValue() method (TokenLimit int + DiffContext *int already present)
    │   └── config_test.go     # EDIT: +TestDiffContextValue
    ├── generate/
    │   └── generate.go        # EDIT (site 1): StagedDiffOptions literal +TokenLimit +DiffContext  [cfg]
    ├── hook/
    │   └── exec.go            # EDIT (site 2): StagedDiffOptions literal +TokenLimit +DiffContext  [cfg]
    └── decompose/
        ├── planner.go         # EDIT (site 4): StagedDiffOptions literal +TokenLimit +DiffContext  [deps.Config]
        ├── message.go         # EDIT (site 5): StagedDiffOptions literal +TokenLimit +DiffContext  [deps.Config]
        └── decompose.go       # EDIT (site 6): StagedDiffOptions literal +TokenLimit +DiffContext  [deps.Config]
└── pkg/stagehand/
    └── stagehand.go           # EDIT (site 3): StagedDiffOptions literal +TokenLimit +DiffContext  [cfg]
# (internal/git/git.go is READ-ONLY — S1 already landed the 3 StagedDiffOptions fields.)
```

### Desired Codebase Tree After This Subtask

```bash
stagehand/
└── (only existing files modified — no new files)
    internal/config/config.go          # +func (c Config) DiffContextValue() int
    internal/config/config_test.go     # +TestDiffContextValue
    internal/generate/generate.go      # site 1: +TokenLimit +DiffContext in the literal
    internal/hook/exec.go              # site 2: +TokenLimit +DiffContext in the literal
    pkg/stagehand/stagehand.go         # site 3: +TokenLimit +DiffContext in the literal
    internal/decompose/planner.go      # site 4: +TokenLimit +DiffContext in the literal
    internal/decompose/message.go      # site 5: +TokenLimit +DiffContext in the literal
    internal/decompose/decompose.go    # site 6: +TokenLimit +DiffContext in the literal
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/config/config.go` | MODIFY | Add `DiffContextValue() int` method (the `*int`→`int` resolver, nil→1). |
| `internal/config/config_test.go` | MODIFY | Add `TestDiffContextValue` (nil→1, *0→0, *3→3). |
| `internal/generate/generate.go` | MODIFY | Site 1: +`TokenLimit: cfg.TokenLimit` +`DiffContext: cfg.DiffContextValue()`. |
| `internal/hook/exec.go` | MODIFY | Site 2: same (cfg). |
| `pkg/stagehand/stagehand.go` | MODIFY | Site 3: same (cfg). |
| `internal/decompose/planner.go` | MODIFY | Site 4: +`TokenLimit: deps.Config.TokenLimit` +`DiffContext: deps.Config.DiffContextValue()`. |
| `internal/decompose/message.go` | MODIFY | Site 5: same (deps.Config). |
| `internal/decompose/decompose.go` | MODIFY | Site 6: same (deps.Config). |

**Explicitly NOT touched**: `internal/git/git.go` (S1 LANDED the struct + 3 diff functions — do not
edit), the `Git` interface, `internal/config` materialize/overlay/Defaults/git-config keys (P1.M1.T1
COMPLETE — only the additive method is added), any docs (contract: none), `PRD.md`, `tasks.json`,
`prd_snapshot.md`, `plan/*`.

### Known Gotchas of our codebase & toolchain

```go
// CRITICAL (G1 — the *int type mismatch): config.DiffContext is *int (config.go:82); StagedDiffOptions.
// DiffContext is plain int (S1, LANDED). A literal `DiffContext: cfg.DiffContext,` is a COMPILE ERROR
// (cannot use *int as int). The faithful mapping is `DiffContext: cfg.DiffContextValue(),` where the
// resolver dereferences with a nil→1 default. S1's struct doc EXPLICITLY mandates this dereference.
// Do NOT "fix" it by making StagedDiffOptions.DiffContext a *int (that violates S1's "git takes resolved
// values" seam and would need reverting in M2).

// CRITICAL (G2 — DiffContext==0 is VALID, the resolver MUST preserve it): FR3f says 0 = changed-lines-only
// (-U0). The resolver returns *0 verbatim (NOT default-1). Only a nil pointer → 1. A resolver that did
// `if c.DiffContext != nil && *c.DiffContext != 0` would silently drop -U0 — a bug. The TestDiffContextValue
// *0→0 case guards this. Mirror the config layer's `!= nil` guard (file.go:226/340), NEVER `!= 0`.

// CRITICAL (G3 — TokenLimit maps DIRECTLY, no resolver): config.TokenLimit is plain int (config.go:81),
// StagedDiffOptions.TokenLimit is plain int (S1). 0 IS the unset sentinel (FR3d — no meaningful "explicit
// 0"). So `TokenLimit: cfg.TokenLimit,` — no method, no dereference. Do NOT wrap it in a resolver.

// GOTCHA (G4 — two variable shapes; don't mix them): sites 1-3 (generate/hook/stagehand) use a local
// `cfg config.Config`; sites 4-6 (decompose planner/message/decompose) use `deps.Config`. The receiver in
// the two new lines differs: `cfg.TokenLimit`/`cfg.DiffContextValue()` vs `deps.Config.TokenLimit`/
// `deps.Config.DiffContextValue()`. Verify the receiver name at EACH site before editing.

// GOTCHA (G5 — DO NOT set PromptReserveTokens): the contract is explicit — leave it zero; M4.T1.S2 wires
// it (where the token estimator exists). Setting it now (e.g. to a guessed constant) would feed garbage
// into the M4 water-fill. Go's zero-value handles "unset". Only TokenLimit + DiffContext are mapped here.

// GOTCHA (G6 — behavior-free; existing tests MUST stay green unchanged): S1 landed the 3 fields as UNREAD
// (the diff functions StagedDiff/TreeDiff/WorkingTreeDiff do not read them until M2/M4). Populating
// TokenLimit/DiffContext at the 6 literals changes ZERO diff output. So every golden diff test
// (stagediff/treediff/workingtreediff) and every generate/hook/decompose/stagehand test passes AS-IS.
// If any test changes, something beyond the mapping was edited — re-check scope.

// GOTCHA (G7 — value receiver, not pointer): Config is passed BY VALUE at all 6 sites (cfg config.Config;
// deps.Config is a value field). Define `func (c Config) DiffContextValue() int` (value receiver) so both
// `cfg.DiffContextValue()` and `deps.Config.DiffContextValue()` compile without auto-addressing concerns.

// GOTCHA (G8 — gofmt re-aligns the literals): adding 2 fields shifts the struct literal's `:` alignment.
// Run `gofmt -w` on each edited file; do NOT hand-align. The existing 4 fields' values are unchanged.

// GOTCHA (G9 — no new imports needed): pkg/stagehand and internal/decompose already import internal/config
// (ResolveRoleModel / cfg.Output etc.); internal/generate and internal/hook already import internal/config
// (the cfg param type is config.Config). So cfg.DiffContextValue() / deps.Config.DiffContextValue() resolve
// with no import additions. (internal/git is NOT imported-into; the resolver lives in config, not git.)
```

## Implementation Blueprint

### Data models and structure

No new types. One new method on the existing `Config` struct (value receiver). The "model" fact is the
resolution semantics: `*int` DiffContext (nil⇒1, `*0`⇒0, `*n`⇒n) → plain int.

### The resolver method (exact — add to internal/config/config.go)

Place near the `intPtr` helper (line 11) or just before `Defaults()` (line 161):

```go
// DiffContextValue resolves the *int DiffContext to the plain int the git diff functions consume
// (StagedDiffOptions.DiffContext is a plain int holding the RESOLVED value — see internal/git/git.go,
// P1.M1.T2.S1). Returns the FR3f default 1 (-U1) when the user omitted the key (nil pointer); a non-nil
// pointer is returned verbatim, so an explicit 0 (-U0 = changed-lines-only) is preserved exactly.
// Called by the 6 StagedDiffOptions production call sites (P1.M1.T2.S2).
func (c Config) DiffContextValue() int {
	if c.DiffContext != nil {
		return *c.DiffContext
	}
	return 1
}
```

### The 6 literal edits (exact — per shape)

**Shape A — sites 1-3 (local `cfg`):** append the two lines inside the existing literal.
```go
diff, err := deps.Git.StagedDiff(ctx, git.StagedDiffOptions{
	MaxDiffBytes:     cfg.MaxDiffBytes,
	MaxMDLines:       cfg.MaxMdLines,
	BinaryExtensions: cfg.BinaryExtensions,
	Excludes:         deps.Excludes,
	TokenLimit:       cfg.TokenLimit,       // FR3d (P1.M1.T2.S2) — read by the M4 gate/water-fill
	DiffContext:      cfg.DiffContextValue(), // FR3f (P1.M1.T2.S2) — *int→int (nil⇒1, *0⇒0); read by M2's -U<n>
})
```
*(Site 1 generate.go:163 StagedDiff; site 2 hook/exec.go:104 StagedDiff; site 3 pkg/stagehand/stagehand.go:423 StagedDiff.)*

**Shape B — sites 4-6 (`deps.Config`):** same two lines, receiver = `deps.Config`.
```go
diff, err := deps.Git.TreeDiff(ctx, baseTree, tStart, git.StagedDiffOptions{
	MaxDiffBytes:     deps.Config.MaxDiffBytes,
	MaxMDLines:       deps.Config.MaxMdLines,
	BinaryExtensions: deps.Config.BinaryExtensions,
	Excludes:         deps.Excludes,
	TokenLimit:       deps.Config.TokenLimit,       // FR3d (P1.M1.T2.S2)
	DiffContext:      deps.Config.DiffContextValue(), // FR3f (P1.M1.T2.S2)
})
```
*(Site 4 planner.go:69 TreeDiff; site 5 message.go:71 TreeDiff; site 6 decompose.go:608 TreeDiff. The
positional args before the literal differ per site — `baseTree, tStart` / `treeA, treeB` / `tipTree, tStart`
— leave them exactly as-is; only the literal body changes.)*

### The unit test (exact — add to internal/config/config_test.go)

```go
func TestDiffContextValue(t *testing.T) {
	// nil ⇒ the FR3f default 1 (-U1). Non-nil (incl. *0) ⇒ verbatim — an explicit 0 (-U0) is preserved.
	tests := []struct {
		name string
		in   *int
		want int
	}{
		{"nil omits the key → default 1", nil, 1},
		{"explicit 0 → -U0 (changed-lines-only)", intPtr(0), 0},
		{"explicit 3 → -U3", intPtr(3), 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := Config{DiffContext: tc.in}
			if got := c.DiffContextValue(); got != tc.want {
				t.Errorf("DiffContextValue() = %d, want %d", got, tc.want)
			}
		})
	}
}
```
*(Uses the existing package-local `intPtr` helper. `Config{DiffContext: tc.in}` — no Defaults() needed;
the method reads only the one field.)*

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD Config.DiffContextValue() to internal/config/config.go
  - FILE: internal/config/config.go
  - ADD the method (§"The resolver method") near intPtr (line 11) or just before Defaults() (line 161).
  - RECEIVER: value receiver `func (c Config)` (gotcha G7).
  - SEMANTICS: nil → 1; non-nil → *c.DiffContext verbatim (incl. *0) (gotcha G2).
  - DO NOT: change the DiffContext field type, Defaults(), materialize, overlay, or git-config keys.
  - RUN: gofmt -w internal/config/config.go ; go build ./internal/config/ → exit 0.

Task 2: ADD TestDiffContextValue to internal/config/config_test.go
  - FILE: internal/config/config_test.go (same package — uses intPtr + Config directly).
  - ADD the 3-row table test (§"The unit test").
  - RUN: go test ./internal/config/ -run TestDiffContextValue -v → PASS.

Task 3: EDIT site 1 — internal/generate/generate.go (StagedDiff, cfg)
  - LOCATE the StagedDiffOptions literal in CommitStaged (~line 163).
  - APPEND: TokenLimit: cfg.TokenLimit, and DiffContext: cfg.DiffContextValue(), (Shape A, §edits).
  - KEEP MaxDiffBytes/MaxMDLines/BinaryExtensions/Excludes byte-identical. Do NOT set PromptReserveTokens.
  - RUN: gofmt -w ; go build ./internal/generate/ → exit 0.

Task 4: EDIT site 2 — internal/hook/exec.go (StagedDiff, cfg)
  - Same as Task 3, literal at ~line 104 (Shape A).

Task 5: EDIT site 3 — pkg/stagehand/stagehand.go (StagedDiff, cfg)
  - Same as Task 3, literal at ~line 423 (Shape A). (internal/config already imported — no new import.)

Task 6: EDIT site 4 — internal/decompose/planner.go (TreeDiff, deps.Config)
  - LOCATE the StagedDiffOptions literal in callPlanner (~line 69).
  - APPEND: TokenLimit: deps.Config.TokenLimit, and DiffContext: deps.Config.DiffContextValue(), (Shape B).
  - LEAVE the positional args (baseTree, tStart) before the literal untouched.
  - RUN: gofmt -w ; go build ./internal/decompose/ → exit 0.

Task 7: EDIT site 5 — internal/decompose/message.go (TreeDiff, deps.Config)
  - Same as Task 6, literal at ~line 71 (Shape B; positional args treeA, treeB).

Task 8: EDIT site 6 — internal/decompose/decompose.go (TreeDiff, deps.Config)
  - Same as Task 6, literal at ~line 608 in runArbiterPhase (Shape B; positional args tipTree, tStart).

Task 9: VALIDATE — full gate set + scope discipline
  - RUN: go build ./... ; go vet ./... ; gofmt -l .
  - RUN: go test ./...   (ALL green — existing suites unchanged; the new option fields are unread.)
  - RUN targeted: go test ./internal/config/ ./internal/generate/ ./internal/hook/ ./internal/decompose/ ./pkg/stagehand/
  - RUN: git grep -n 'TokenLimit:\|DiffContext:' internal/generate internal/hook pkg/stagehand internal/decompose
         (expect: 2 matches per file = 12 total, the TokenLimit + DiffContext lines.)
  - RUN: git grep -n 'PromptReserveTokens:' internal/generate internal/hook pkg/stagehand internal/decompose
         (expect: NO matches — PromptReserveTokens is NOT set at any site.)
  - RUN: git diff --stat → expect ONLY the 8 files listed in the Desired Codebase Tree.
```

### Implementation Patterns & Key Details

```go
// === Why a resolver method (and not inline dereference ×6) ===
// The nil→1 default is the FR3f-critical resolution. Inlining it 6× means 6 chances to write the guard
// wrong (e.g. `!= 0` instead of `!= nil`, silently dropping -U0). One method = one rule, used 6×. The
// method lives on Config (the type whose field is resolved) — idiomatic Go. Value receiver because Config
// is passed by value at every call site.

// === Why the resolver preserves *0 (the FR3f invariant) ===
// FR3f: diff_context 0 = changed-lines-only (-U0), a legitimate value. config.DiffContext is *int so
// materialize/overlay can distinguish unset (nil→1) from explicit 0 (*0→0). The resolver mirrors that:
// `if c.DiffContext != nil { return *c.DiffContext }; return 1`. A `*0` returns 0, NOT 1. The
// TestDiffContextValue *0→0 case is the regression guard. (config layer: file.go:226 and :340 both guard
// `!= nil` — never `!= 0`. Same discipline.)

// === Why TokenLimit needs no resolver ===
// Both config.TokenLimit and StagedDiffOptions.TokenLimit are plain int; 0 is the unset sentinel (FR3d:
// "0/unset ⇒ legacy caps" — there is no meaningful "explicit 0"). So `TokenLimit: cfg.TokenLimit,` is a
// direct value copy. Wrapping it would be pointless indirection.

// === Why this is behavior-free (the regression guarantee) ===
// S1 landed TokenLimit/DiffContext/PromptReserveTokens as UNREAD fields (the diff functions don't read
// them until M2/M4). Populating TokenLimit/DiffContext at the literals puts values into a struct that
// nobody reads yet — zero observable effect. Hence every existing golden diff test (stagediff/treediff/
// workingtreediff) and every pipeline test (generate/hook/decompose/stagehand) passes UNCHANGED. The
// contract's "all must pass unchanged (no behavior change)" is satisfied by construction, not by luck.

// === Why no new imports ===
// All 4 files with call sites already import internal/config (the cfg param / deps.Config type is
// config.Config; decompose uses config.ResolveRoleModel; stagehand uses cfg.Output). So
// cfg.DiffContextValue() / deps.Config.DiffContextValue() resolve with no import additions.
```

### Integration Points

```yaml
CONFIG (internal/config/config.go):
  - +func (c Config) DiffContextValue() int   (the *int→int resolver; nil→1, *0→0, *n→n)
  - TokenLimit (int) + DiffContext (*int) fields UNCHANGED; Defaults/materialize/overlay UNCHANGED

CONFIG TEST (internal/config/config_test.go):
  - +TestDiffContextValue (nil→1, *0→0, *3→3)

CALL SITES (6 production literals — each +TokenLimit +DiffContext):
  - internal/generate/generate.go:163    (StagedDiff, cfg)            [Shape A]
  - internal/hook/exec.go:104            (StagedDiff, cfg)            [Shape A]
  - pkg/stagehand/stagehand.go:423       (StagedDiff, cfg)            [Shape A]
  - internal/decompose/planner.go:69     (TreeDiff,   deps.Config)    [Shape B]
  - internal/decompose/message.go:71     (TreeDiff,   deps.Config)    [Shape B]
  - internal/decompose/decompose.go:608  (TreeDiff,   deps.Config)    [Shape B]

NOT SET (intentionally):
  - PromptReserveTokens at all 6 sites (left zero; wired in P1.M4.T1.S2)

NO-TOUCH (explicitly — owned by sibling/completed subtasks):
  - internal/git/git.go (StagedDiffOptions struct + 3 diff functions)   # S1 LANDED + M2/M4 consumption
  - internal/config materialize/overlay/Defaults/git-config             # P1.M1.T1 COMPLETE
  - internal/prompt/*                                                   # M4.T1.S2 measures PromptReserveTokens here
  - any docs                                                            # contract: none
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

DOWNSTREAM CONSUMERS (informational — owned by LATER subtasks, NOT this one):
  - P1.M2.T2 (FR3f): the flag helper reads opts.DiffContext → injects `-U<opts.DiffContext>`
  - P1.M4.T3 (FR3d): the token-limit gate reads opts.TokenLimit → switches off legacy caps when >0
  - P1.M4.T1.S2: measures PromptReserveTokens upstream and sets it at the 6 sites (this task leaves it 0)
  - P1.M4.T2 (FR3i): the water-fill reads opts.TokenLimit + opts.PromptReserveTokens
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand

gofmt -l internal/config/config.go internal/config/config_test.go internal/generate/generate.go \
       internal/hook/exec.go pkg/stagehand/stagehand.go internal/decompose/planner.go \
       internal/decompose/message.go internal/decompose/decompose.go
# Expected: empty (run gofmt -w on any listed file — it re-aligns the struct literals).

go vet ./...
# Expected: exit 0. (A `*int as int` vet/build error at a call site means DiffContextValue() was omitted.)

go build ./...
# Expected: exit 0. Confirms the resolver compiles + all 6 literals type-check (cfg/​deps.Config shapes).
```

### Level 2: Unit Tests (the resolver + behavior-free regression)

```bash
cd /home/dustin/projects/stagehand

# The new resolver logic:
go test ./internal/config/ -run TestDiffContextValue -v
# Expected: PASS — nil→1, *0→0, *3→3.

# The 6 call-site packages — existing suites unchanged (fields unread ⇒ no behavior change):
go test ./internal/generate/ ./internal/hook/ ./internal/decompose/ ./pkg/stagehand/ ./internal/git/
# Expected: ALL green. No existing test alters (the diff functions do not read the new fields).

go test ./...
# Expected: ALL packages green.
```

### Level 3: Whole-Repository Regression (no collateral)

```bash
cd /home/dustin/projects/stagehand

go test -race ./...     # Expected: ALL green.
go vet ./...            # Expected: exit 0.

# Confirm the 6 sites each carry TokenLimit + DiffContext (12 matches total):
git grep -n 'TokenLimit:\|DiffContext:' internal/generate/generate.go internal/hook/exec.go \
    pkg/stagehand/stagehand.go internal/decompose/planner.go internal/decompose/message.go \
    internal/decompose/decompose.go | wc -l
# Expected: 12 (2 per file × 6 files). Each 'DiffContext:' line must call DiffContextValue().

# Confirm PromptReserveTokens is NOT set at any call site (M4.T1.S2 owns it):
git grep -n 'PromptReserveTokens:' internal/generate internal/hook pkg/stagehand internal/decompose || echo "OK: PromptReserveTokens not set at any call site"
# Expected: "OK: PromptReserveTokens not set at any call site".

# Confirm ONLY the 8 in-scope files changed:
git diff --stat -- internal/config/ internal/generate/ internal/hook/ internal/decompose/ pkg/stagehand/
# Expected: config.go + config_test.go + the 6 call-site files. Nothing else.

# Confirm S1's territory (StagedDiffOptions struct + diff functions) UNTOUCHED:
git diff --stat -- internal/git/git.go
# Expected: EMPTY (S1 already landed the struct; this task does not edit git.go).
```

### Level 4: Resolver-Semantics Cross-Check (prove *0 is preserved)

```bash
cd /home/dustin/projects/stagehand

# Throwaway main: proves the resolver's three semantics (the exact thing the 6 sites depend on).
cat > /tmp/sh_dc_check.go <<'EOF'
package main
import "fmt"
func main() {
    resolve := func(p *int) int { if p != nil { return *p }; return 1 }
    var nilp *int
    zero := 0
    three := 3
    fmt.Printf("nil→%d (want 1)\n*0→%d (want 0)\n*3→%d (want 3)\n",
        resolve(nilp), resolve(&zero), resolve(&three))
}
EOF
go run /tmp/sh_dc_check.go && rm -f /tmp/sh_dc_check.go
# Expected: nil→1, *0→0, *3→3. (The *0→0 line is the FR3f guard — if it printed 1, the resolver is wrong.)

# Docs/contract cross-check: no docs changed (contract: "DOCS: none").
git diff --stat -- docs/ README.md || echo "OK: no docs changed"
```

## Final Validation Checklist

### Technical Validation

- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test ./...` (and `go test -race ./...`) — all packages green.

### Feature Validation

- [ ] `Config.DiffContextValue() int` exists (value receiver): nil→1, `*0`→0, `*n`→n.
- [ ] All 6 production literals carry `TokenLimit` + `DiffContext` (correct per shape: `cfg` vs `deps.Config`).
- [ ] Each `DiffContext:` line calls `DiffContextValue()` (NOT a raw `cfg.DiffContext` — that's a type error).
- [ ] `PromptReserveTokens` is NOT set at any of the 6 sites (Level 3 grep → none).
- [ ] `TestDiffContextValue` passes (nil→1, *0→0, *3→3).
- [ ] The existing 4 fields at each literal unchanged; the diff functions / struct / interface unchanged.

### Scope Discipline Validation

- [ ] ONLY the 8 files in the Desired Codebase Tree modified (`git diff --stat`).
- [ ] Did NOT edit `internal/git/git.go` (S1 LANDED; M2/M4 own consumption).
- [ ] Did NOT change `config.DiffContext` field type or materialize/overlay/Defaults/git-config (P1.M1.T1 COMPLETE).
- [ ] Did NOT set `PromptReserveTokens` (M4.T1.S2) or add `-M`/`-U<n>`/skeleton/water-fill (M2/M3/M4).
- [ ] Did NOT add a `config.DiffOpts()` bridge function returning the whole struct (optional future refactor; out of scope).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/` (except this PRP + research).

### Code Quality Validation

- [ ] Resolver is a value-receiver method on `Config` (matches the by-value call sites; gotcha G7).
- [ ] Resolver guards `!= nil` (NOT `!= 0`) — mirrors the config layer's materialize/overlay discipline.
- [ ] Field names map symmetrically (`cfg.TokenLimit` → `TokenLimit:`).
- [ ] gofmt re-aligns the literals; no hand-alignment.
- [ ] The behavior-free rationale is honored: no existing test altered.

---

## Anti-Patterns to Avoid

- ❌ Don't write `DiffContext: cfg.DiffContext,` — `cfg.DiffContext` is `*int`, the field is plain `int`;
  it's a compile error. Use `cfg.DiffContextValue()` (gotcha G1). S1's struct doc explicitly mandates the
  call-site dereference.
- ❌ Don't write a resolver that collapses `*0` → 1 (`if != nil && != 0`) — that silently drops `-U0`, a
  legitimate FR3f value. Guard `!= nil` only; return `*c.DiffContext` verbatim (gotcha G2).
- ❌ Don't wrap `TokenLimit` in a resolver — both sides are plain `int` and 0 is the unset sentinel (FR3d);
  map it directly: `TokenLimit: cfg.TokenLimit,` (gotcha G3).
- ❌ Don't set `PromptReserveTokens` at the call sites — the contract leaves it zero; M4.T1.S2 wires it
  (where the token estimator exists). Setting a guessed value now poisons the M4 water-fill (gotcha G5).
- ❌ Don't mix the two variable shapes — sites 1-3 use `cfg`, sites 4-6 use `deps.Config`. Verify the
  receiver at each site (gotcha G4).
- ❌ Don't edit `internal/git/git.go` (S1 landed the struct; M2/M4 own the diff-function consumption), the
  config materialize/overlay/Defaults (P1.M1.T1 COMPLETE), or any diff-function behavior. This task maps
  2 fields at 6 literals + adds 1 resolver method — nothing more (gotcha G6).
- ❌ Don't make the resolver a pointer receiver (`func (c *Config)`) — Config is passed by value at all 6
  sites; a value receiver is correct and avoids addressability edge cases (gotcha G7).
- ❌ Don't add a `config.DiffOpts(excludes)` bridge returning the whole `StagedDiffOptions` — the touchmap
  flags it as an OPTIONAL future refactor; the contract scoped this task to mapping the 2 fields inline.
- ❌ Don't hand-align the struct literals — run `gofmt -w`; it re-aligns the `:` column (gotcha G8).
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a mechanical, fully-prescribed mapping (append 2 lines to each of 6 literals) plus one
tiny resolver method — with every call site quoted verbatim from the live tree (both variable shapes), the
exact method body, the exact 2-line addition per shape, and the verified type facts (`config.DiffContext`
is `*int`; `StagedDiffOptions.DiffContext` is plain `int` per LANDED S1). S1 is confirmed LANDED (the struct
already has the 3 fields with the explicit "call site dereferences with a default-1 fallback" doc comment),
and P1.M1.T1 is confirmed COMPLETE (`config.TokenLimit int` + `config.DiffContext *int` + tests). The one
non-obvious trap — the `*int`→`int` type mismatch that makes the contract's literal shorthand a compile
error — is the central gotcha (G1), resolved by the mandated resolver method, and the second trap —
preserving `*0` (FR3f `-U0`) rather than defaulting it — is guarded by `TestDiffContextValue`'s `*0→0` row
and the `!= nil` discipline (G2). The task is behavior-free by construction (S1's fields are unread until
M2/M4), so `go test ./...` staying green IS the regression proof; no existing test should change. The two
residual uncertainties (gofmt column re-alignment and the exact insertion line numbers drifting from the
~markers, since S1 just landed) are both caught by the deterministic `gofmt -l .` + `go build ./...` +
`git grep` (12 matches / 0 PromptReserveTokens) gates. M2/M4 (the downstream consumers) are cleanly fenced
and cannot be broken by populating values they will later read.
