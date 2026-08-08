# Residual Risk — role_defaults.go drift (code, out of this PRD's docs-only Phase 2 scope)

**Status:** TRACKED, NOT addressed by this delta PRD's tasks. Surface to the human for a separate
code+docs fix. This file exists so the Phase 2 doc agent does not paper over a real inconsistency.

## The drift

`internal/config/role_defaults.go` → `roleDefaults` (the compiled-in FR-D4 per-provider×per-role
default-model table) still marks the stager cell `""` for three providers that ARE stager-capable in
`internal/provider/builtin.go`:

| Provider | role_defaults.go stager | builtin.go TooledFlags | Drift? |
|----------|-------------------------|------------------------|--------|
| agy      | `""` (L69, "NOT stager-capable") | non-nil (L244-247) | **YES** |
| opencode | `""` (L81, "NOT stager-capable") | non-nil (L351-353) | **YES** |
| codex    | `""` (L87, "NOT stager-capable") | non-nil (L415-417) | **YES** |
| qwen-code| `""` (L75) | nil (L303) | no (correct) |
| cursor   | `""` (L93) | nil | no (correct) |
| pi       | `gpt-5.4-mini` | non-nil | no |
| claude   | `sonnet` | non-nil | no |

The comment block at `role_defaults.go:35-40` explicitly anticipates this: "VERIFY the TooledFlags
state in builtin.go at implementation — if a provider has since gained TooledFlags, give it the
mid-tier stager model." The verification was never done for agy/opencode/codex.

## Runtime impact
When `--provider agy` (or opencode/codex) is detected, the config bootstrap writes the detected
provider's `[role.*]` block with `stager = ""` (from roleDefaults) and applies the FR-D4 fallback
(stager role → next stager-capable provider, i.e. pi/claude). So agy/opencode/codex currently use a
DIFFERENT provider's model for the stager role even though they could stage themselves. Not a crash —
a suboptimal default + a doc that (after this PRD's doc fix) will say agy/opencode/codex CAN stage
while role_defaults.go says they cannot.

## Coupling to docs/providers.md
`docs/providers.md` per-role models table (L130-137) claims to mirror role_defaults.go:
> "The compiled-in per-provider table (PRD §9.16 FR-D4) lives in `internal/config/role_defaults.go`."
Its `*(cannot)*` cells for agy/opencode/codex therefore match the STALE role_defaults.go, NOT the
(correct) builtin.go capability. After the Phase 2 main-table fix, the per-role table will be
internally inconsistent (main "Stager?" says yes; per-role "stager" says *(cannot)*).

## Recommended handling (for the human — NOT auto-applied)
This is a 2-part fix that should be its own PRP:
1. **Code:** update `role_defaults.go` — set the mid-tier stager model for agy (`Gemini 3.5 Flash
   (Medium)`), opencode (`openai/gpt-5.4-mini`), codex (`gpt-5.1-codex-mini`) per the §9.16 FR-D4
   rationale (stager = mid tier); update the comment block + `DefaultModelsVerificationDate`.
2. **Docs:** update `docs/providers.md` per-role table `*(cannot)*` cells → the stager model tokens,
   and the footer "currently pi or claude" → "pi, claude, agy, opencode, codex".

## What this PRD's Phase 2 task DOES (bounded, docs-only, PRD-faithful)
- Fixes the main provider table "Stager?" column (CAPABILITY per builtin.go): agy/codex/opencode →
  `✓ yes (unscoped)`. This is unambiguous and within PRD scope.
- Rewrites the prose note + asymmetry section (drops "agy cannot serve as a stager").
- Updates the per-role table FOOTER's capability claim if it lists stager-capable providers.
- Does NOT invent stager model tokens for the per-role `*(cannot)*` cells (those mirror role_defaults.go
  and are the residual risk above). Leaves a cross-reference noting the role_defaults.go drift rather
  than fabricating values.