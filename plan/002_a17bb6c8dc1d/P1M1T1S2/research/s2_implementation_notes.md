# S2 Implementation Notes — MergeManifest for TooledFlags + Experimental

> Scope: P1.M1.T1.S2 — extend `MergeManifest` (`internal/provider/merge.go`) to field-merge the two
> fields S1 added (`TooledFlags []string`, `Experimental *bool`), plus extend `merge_test.go`.
> Verified against the live source on 2026-07-01. **S1 is already landed** in the working tree.

## 0. Baseline state (confirmed by execution)

- `go test ./internal/provider/` → **GREEN** (`ok ... 0.369s`) before any S2 edit. S2 must keep it green.
- `manifest.go` ALREADY has the two fields, the `Resolve()` default (`Experimental→boolPtr(false)`), and
  the updated doc comments ("Slices (Subcommand, BareFlags, TooledFlags)"; "left as-is" line names TooledFlags).
- `merge.go` has **ZERO** references to `TooledFlags`/`Experimental` → an override that sets either field
  is **silently dropped** today (merged result inherits `base`'s value). This is exactly the bug S2 fixes.

## 1. The current MergeManifest structure (internal/provider/merge.go)

`MergeManifest(base, override Manifest) Manifest` opens with `out := base` (shallow copy), then applies
THREE regimes, each already documented in the function's doc comment:

| Regime | Fields (current) | Override signal | Effect |
|--------|------------------|-----------------|--------|
| 1 — scalar pointers | Detect, Command, PromptDelivery, PromptFlag, PrintFlag, ModelFlag, DefaultModel, SystemPromptFlag, ProviderFlag, DefaultProvider, Output, JsonField, **StripCodeFence**, RetryInstruction | `override.X != nil` | `out.X = override.X` (explicit `*false`/`*""` WINS) |
| 2 — slices | **Subcommand, BareFlags** | `len(override.X) > 0` | `out.X = override.X` (wholesale REPLACE, no element merge) |
| 3 — Env map | Env | `len(override.Env) > 0` | fresh map, base keys survive, override keys win |
| (0 — Name) | Name | — | NOT merged (`out.Name == base.Name`) |

**S1 mapped the new fields onto regimes already proven by precedents:**
- `Experimental *bool` → **regime 1** (precedent: `StripCodeFence` — the other `*bool`).
- `TooledFlags []string` → **regime 2** (precedent: `BareFlags` — its tooled-mode analog).

So S2 is the literal application of the two existing regimes to two new fields — NO new regime, NO new
merge semantics, NO new design call. The architecture delta (`manifest_v2_delta.md` §2) prescribes the
exact lines verbatim.

## 2. The exact diff (3 surgical edits to merge.go)

**Edit A — doc comment, line 13** (regime-2 enumeration; mirror the update S1 made to the struct comment):
```diff
-//  2. Slices (Subcommand, BareFlags): len(override.Slice) > 0 → result REPLACES base's slice
+//  2. Slices (Subcommand, BareFlags, TooledFlags): len(override.Slice) > 0 → result REPLACES base's slice
```

**Edit B — regime 1**, immediately after the `StripCodeFence` block (groups the two `*bool` fields):
```go
	if override.Experimental != nil {
		out.Experimental = override.Experimental
	}
```

**Edit C — regime 2**, immediately after the `BareFlags` block (groups the two flag slices):
```go
	if len(override.TooledFlags) > 0 {
		out.TooledFlags = override.TooledFlags
	}
```

That is the ENTIRE production change: 6 added lines (2×3) + 1 doc-token. No behavior change for any
existing field; no edit to `Resolve`/`Validate`/`Name`/`Env` handling.

## 3. Why placement is "after the matching precedent" (not alphabetical/structural)

- `Experimental` placed after `StripCodeFence` (both `*bool`) — keeps the pointer-`*bool` entries adjacent
  and mirrors the "Experimental mirrors StripCodeFence" framing S1 established.
- `TooledFlags` placed after `BareFlags` — mirrors their structural pairing (bare vs tooled flag-set).
Both are readability/consistency choices, not correctness requirements; the override signal is what
matters (`!= nil` for pointers, `len > 0` for slices), and that is dictated by the field's TYPE.

## 4. Test strategy (merge_test.go — same package, same helpers)

**FOLLOW the existing test taxonomy exactly** — one focused test per regime/behavior, reusing
`strPtr`/`boolPtr` + `sampleBase()` + `reflect.DeepEqual`. The existing tests are the template.

**CRITICAL fixture change — `sampleBase()`**: it currently sets neither field. To make the "preserves
base" / "identity" / "no-mutation" tests MEANINGFUL for the new fields, `sampleBase()` MUST gain
non-nil / non-empty values:
```go
TooledFlags:  []string{"--allowed-tools", "git:*", "--approval-mode", "auto"},
Experimental: boolPtr(true),   // non-default true → "preserves base" + "explicit false wins" are real
```
Verified safe: adding two non-zero fields to `sampleBase()` cannot break any existing assertion — every
existing test either asserts "merged matches base" (trivially still true) or asserts a specific override
(unaffected). `TestMergeManifest_EmptyOverrideIsIdentity` (reflect.DeepEqual) now EXERCISES the new fields
because base has non-nil values for them.

**Test changes (extend existing + one new):**
| Test | Change | Proves |
|------|--------|--------|
| `TestMergeManifest_PartialOverride_OnlyTouchedFieldChanges` | ADD: `merged.TooledFlags` DeepEqual base; `merged.Experimental` == base | partial override preserves both new fields (THE keystone — contract OUTPUT) |
| `TestMergeManifest_ExplicitZeroPointerWins` | ADD `Experimental: boolPtr(false)` to the override (base has `true`) + assert `*merged.Experimental == false` | pointer-regime payoff: explicit false OVERRIDES base's true |
| `TestMergeManifest_EmptyOrNilSlicePreservesBase` | ADD: nil override keeps base.TooledFlags; empty `[]string{}` override keeps base.TooledFlags | slice "absent" sentinel (nil AND non-nil-empty both preserve base) |
| `TestMergeManifest_DoesNotMutateInputs` | ADD: snapshot `base.TooledFlags`; assert unmutated after merge | no-aliasing invariant extends to the new slice (reassigning header is safe) |
| `TestMergeManifest_TooledFlagsReplacedWholesale` (NEW) | non-empty `TooledFlags` override → result is exactly the override; `BareFlags` untouched | wholesale replace + the two flag-slices are independent |

`TestMergeManifest_EmptyOverrideIsIdentity` needs NO edit — reflect.DeepEqual now covers the new fields
automatically once `sampleBase()` sets them. (Do not add a `merged.Experimental == base.Experimental`
there; DeepEqual already does it.)

## 5. The aliasing invariant (why slices are safe, maps are not — unchanged by S2)

Regime 2 REASSIGNS the slice header (`out.TooledFlags = override.TooledFlags`); it never writes into the
backing array, so `base.TooledFlags` is never mutated. (Same reason `BareFlags`/`Subcommand` are safe.)
The fresh-map allocation in regime 3 (Env) is the ONLY place mutation would leak without the copy; S2
adds NO map handling, so that invariant is untouched. The `DoesNotMutateInputs` extension for TooledFlags
is a belt-and-suspenders assertion, not a fix for a real hazard.

## 6. Composition with the registry (read-only — NOT modified by S2)

The registry (P1.M2.T3) calls `merged := MergeManifest(builtin, userOverride)` then `merged.Validate()`
then `merged.Resolve()`. S2's merge runs BEFORE Validate/Resolve, so:
- `Experimental` may be nil at merge time (builtin unset + override unset) → `Resolve` defaults to `*false`
  afterward. S2 correctly leaves it nil in that case (only applies a non-nil override).
- `TooledFlags` may be nil at merge time → stays nil → Render-time tooled-mode check (T2, not here) errors.
S2 changes nothing about this pipeline; it only ensures an override's value is no longer silently dropped.

## 7. Scope discipline — what S2 does NOT do

- NOT `manifest.go` (S1 owns the struct + Resolve).
- NOT `render.go` / `RenderMode` (P1.M1.T2.S1 owns the bare/tooled mode param + the "tooled requires
  non-empty tooled_flags" error — that is a RENDER-time check, NOT a merge concern).
- NOT `builtin.go` / `agy` / `tooled_flags` values on pi/claude (P1.M2.T1/T2).
- NOT `registry.go` (already calls MergeManifest correctly; no signature change).
- NOT user-facing `docs/*.md` (contract: internal merge logic, no docs change).
- NOT PRD.md / tasks.json / prd_snapshot.md / plan/*.

## Sources

- `plan/002_a17bb6c8dc1d/architecture/manifest_v2_delta.md` §2 — verbatim merge lines prescribed.
- `internal/provider/merge.go` (current) — the three regimes + the line-13 doc enumeration.
- `internal/provider/merge_test.go` (current) — the test taxonomy to mirror.
- PRD §16.1 (field-by-field manifest merge), §12.1 (manifest schema / tooled_flags), §12.7.2 (experimental).
