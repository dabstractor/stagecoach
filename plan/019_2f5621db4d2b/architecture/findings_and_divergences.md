# PRD-vs-Reality Divergences (executive summary for downstream agents + human)

Research-driven reality check performed before decomposition. The delta PRD is largely accurate and
its contracts are sound, but FOUR divergences were found. Each is resolved in the task breakdown or
flagged as a residual risk.

## D1 — opencode IS stager-capable (PRD Phase 2 is factually WRONG about opencode) — RESOLVED IN BREAKDOWN

- **PRD claim (Phase 2, contract step 3):** "Leave opencode and qwen-code as `— no` (their
  TooledFlags are still nil — confirmed builtin.go:303)."
- **REALITY:** opencode's `TooledFlags` is **NON-NIL** = `["--agent", "build"]`
  (`internal/provider/builtin.go:351-353`), verified 2026-07-09 (opencode 1.1.23). Only **qwen-code**
  is correctly "no" (its TooledFlags IS nil at L303). The "builtin.go:303" citation in the PRD points
  at qwen-code's nil-comment, NOT opencode — the PRD conflated the two.
- **Consequence:** `docs/providers.md` is stale for **THREE** providers (agy, codex, **opencode**),
  not two. The PRD's instruction to leave opencode as `— no` would ship a doc that STILL contradicts
  the code — the exact failure Phase 2 exists to fix.
- **Resolution:** the Phase 2 task context_scope CORRECTS this — opencode is updated to `✓ yes
  (unscoped)` alongside agy and codex. The PRD's own verification step (step 4/5: "confirm every Stager?
  cell matches builtin.go's TooledFlags (non-nil ⇔ stager-capable) for all seven providers") MANDATES
  this; step 3's opencode instruction is the part that's wrong.

## D2 — role_defaults.go is ALSO stale (code drift, NOT mentioned by the PRD) — RESIDUAL RISK (flagged)

- **Finding:** `internal/config/role_defaults.go` `roleDefaults` still has `stager: ""` for agy (L69),
  opencode (L81), codex (L87) — all annotated "NOT stager-capable (TooledFlags nil)". The code comment
  block (L35-40) even says "As of 2026-07-02 that is ONLY pi + claude ... VERIFY the TooledFlags state
  in builtin.go at implementation — if a provider has since gained TooledFlags, give it the mid-tier
  stager model." builtin.go HAS since gained TooledFlags for all three, but role_defaults.go was never
  updated.
- **Runtime impact:** the config bootstrap applies the FR-D4 fallback for the stager role of
  agy/opencode/codex (falls back to pi/claude's stager model) even though the provider CAN stage.
- **Why NOT in the breakdown:** the PRD explicitly scopes Phase 2 to `docs/providers.md` (docs-only)
  and "Out of scope: No new CLI flag, config key, provider manifest, or public-API change." A code edit
  to role_defaults.go (the compiled-in FR-D4 default table) is a code change not authorized by this PRD.
  Per AGENTS.md hard rule 2, code edits need a PRP. The PRD's authority for Phase 2 is docs-only.
- **Resolution:** surfaced as a residual risk; see `role_defaults_drift.md`. The Phase 2 task fixes the
  docs/providers.md CAPABILITY column (main table) to match builtin.go; the per-role stager column +
  role_defaults.go are flagged for a SEPARATE human-authorized code+docs fix.

## D3 — FR-M13/M14 definitions live in spec/01-product.md §9.14, NOT spec/03-generation.md — RESOLVED IN BREAKDOWN

- **PRD Phase 1 selector hint:** "`§9.14` (FR-M13/FR-M14), `§13.6.3` ...".
- **REALITY:** FR-M13/M14 are DEFINED in `spec/01-product.md:363-364` (§9.14). `spec/03-generation.md`
  only references them (§13.6.3 addendum, L120). §20.2/§20.5 live in `spec/06-reliability.md`.
- **Resolution:** prd_selectors use the merged-document structure index (h3.30 for §9.14, h4.5 for
  §13.6.3, h3.98/h3.100 for §20.2/§20.5, h4.8 for §13.6.6) so downstream agents extract the right text
  regardless of which spec file holds it. Captured in `spec_requirements.md`.

## D4 — spec §12.7 codex manifest is itself stale (no tooled_flags) — RESIDUAL RISK (flagged, DO-NOT-EDIT)

- **Finding:** `spec/02-providers.md §12.7` codex manifest (L470-500) has no `tooled_flags`, old
  `bare_flags` (`--ask-for-approval`), `prompt_delivery="positional"`. Code (builtin.go L415-417) is
  ahead of spec. (agy §12.5.1 IS correctly up to date.)
- **PRD Phase 2 claim:** "The spec `spec/02-providers.md` already reflects this." — true for agy,
  FALSE for codex.
- **Resolution:** docs/providers.md codex edit is grounded in the CODE (the authoritative capability
  source), with the spec §12.7 codex gap flagged as a tracked item. Per AGENTS.md hard rule 1, `spec/`
  is NEVER edited outside an interactive session — surfaced, not fixed.

## Net effect on the breakdown
- Phase 1 (fast-path): unchanged in scope; selectors + contracts enriched with verified line numbers.
- Phase 2 (docs sync): CORRECTED to fix THREE provider rows (agy, codex, opencode) instead of two;
  per-role table + role_defaults.go flagged as residual risk; spec codex gap flagged.
- All divergences are documented so the implementing PRP agents and the human have the full picture.