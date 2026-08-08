package config

// DefaultModelsVerificationDate is the date the FR-D4 roleDefaults table was last verified (FR-D5).
// Surfaced by `stagecoach models` in the curated-fallback annotation (FR-L1). Update this AND roleDefaults
// together on each re-verification.
const DefaultModelsVerificationDate = "2026-07-02"

// FR-D4 / FR-D5 verification block (PRD §9.16).
//
// Verification date: 2026-07-02
// Primary source:   PRD §9.16 FR-D4 table + work-item exemplars (P1.M3.T3.S1 item_description §1).
// FR-D5 mandate:    Model lineups change fast. The implementing agent MUST re-verify each provider's
//                   current flagship/mid/fast model names against that provider's live docs / --help
//                   and record verified names + date here. Defaults are authored trivially-refreshable
//                   (one cell per provider×role).
//
// FR-D3 rationale: the message tier is the cheapest / free-tier-eligible model (highest-volume role;
// many users on free tiers).
//
// Per-provider status (update on re-verification):
//   pi       — gpt-5.4 / gpt-5.4-mini / gpt-5.4-nano — PRD baseline 2026-07 (bare; sub-provider set
//              separately via --provider; verify pi's OpenAI-routing sub-provider, FR-D4 note).
//   opencode — openai/gpt-5.4 / -mini / -nano — PRD baseline 2026-07 (provider-prefixed; verify upstream).
//   cursor   — gpt-5.4 / gpt-5.4-mini / gpt-5.4-nano — UNVERIFIED: PRD gives tier names (flagship/mid/
//              nano); resolved to best-guess OpenAI tokens (cursor is OpenAI-backed). VERIFY `agent --help`.
//   agy      — "Gemini 3.5 Flash (High)" / "Gemini 3.5 Flash (Medium)" / "Gemini 3.5 Flash (Low)" —
//              refreshed 2026-07-03 per FR-D5 vs live `agy models` + -p runs. agy's --model takes the
//              DISPLAY LABEL verbatim (reasoning baked into the "(Low/Medium/High)" suffix, NOT a separate
//              flag); API-style ids (gemini-3.5-flash) are silently ignored → fallback to agy's default.
//   qwen-code — qwen3-coder-plus / "" (cannot stager) / qwen3-coder-flash / qwen3-coder-plus — # TO CONFIRM
//               per FR-D5 (Alibaba Qwen3-Coder via DashScope; no live CLI lookup this pass).
//   codex    — gpt-5.1-codex-max / gpt-5.1-codex-mini / gpt-5.4-nano — PRD baseline 2026-07.
//   claude   — opus / sonnet / haiku — PRD baseline 2026-07 (bare aliases; opus=4.8, sonnet=5 per FR-D4).
//
// Stager-capability basis: a provider's stager cell is non-empty IFF its built-in manifest
// (internal/provider/builtin.go) has non-empty TooledFlags. As of 2026-07-02 that is ONLY pi + claude.
// agy/opencode/codex/cursor/qwen-code have stager="" (nil TooledFlags ⇒ RenderTooled errors
// ⇒ cannot be stager). The bootstrap (P1.M4.T2) applies the FR-D4 fallback (next TooledFlags-capable
// provider) on stager=="". VERIFY the TooledFlags state in builtin.go at implementation — if a provider
// has since gained TooledFlags, give it the mid-tier stager model.

// RoleModelDefaults is the PRD §9.16 FR-D4 per-provider × per-role default-model table, keyed
// provider → role → model. The four roles are planner/stager/message/arbiter (FR-R1). A stager value
// of "" means the provider cannot serve as the stager (its built-in manifest has nil/empty TooledFlags
// — only pi and claude are stager-capable); the bootstrap (P1.M4.T2) applies the FR-D4 fallback on
// that signal. See the FR-D5 block above for model-name provenance + the re-verification mandate.
type RoleModelDefaults map[string]map[string]string

// roleDefaults is the compiled-in FR-D4 table (unexported; access via DefaultModelsForProvider, which
// returns copies). Stager cells: non-empty IFF the provider's manifest has non-empty TooledFlags
// (pi, claude); "" otherwise (agy, opencode, codex, cursor, qwen-code) — the bootstrap applies the fallback.
var roleDefaults = RoleModelDefaults{
	"pi": {
		// multi-backend (FR-R5b): placeholders — the bootstrap BLANKS pi's written [role.*] models so the
		// user supplies their inference/model prefix; these are the internal fallback + commented suggestions.
		"planner": "gpt-5.4-nano",
		"stager":  "gpt-5.4-mini",
		"message": "gpt-5.4-nano",
		"arbiter": "gpt-5.4-nano",
	},
	"claude": {
		"planner": "haiku",  // fast tier (FR-D3 fast-by-default)
		"stager":  "sonnet", // mid tier — tool-use reliability for git staging
		"message": "haiku",  // fast tier (highest-volume role)
		"arbiter": "haiku",  // fast tier
	},
	"agy": {
		// agy --model takes the `agy models` display label VERBATIM (reasoning baked into the suffix).
		"planner": "Gemini 3.5 Flash (Low)",   // fast tier
		"stager":  "Gemini 3.5 Flash (Medium)", // mid tier (stager-capable, §12.5.1.1 item 4)
		"message": "Gemini 3.5 Flash (Low)",    // fast tier
		"arbiter": "Gemini 3.5 Flash (Low)",    // fast tier
	},
	"qwen-code": {
		"planner": "qwen3-coder-flash", // fast tier. # TO CONFIRM per FR-D5
		"stager":  "",                  // NOT stager-capable (TooledFlags nil) — bootstrap applies FR-D4 fallback
		"message": "qwen3-coder-flash", // fast tier. # TO CONFIRM per FR-D5
		"arbiter": "qwen3-coder-flash", // fast tier. # TO CONFIRM per FR-D5
	},
	"opencode": {
		// plan-/backend-dependent (provider-prefixed) — the bootstrap BLANKS opencode's written [role.*]
		// models (power-user wildcard); these fast placeholders are the internal fallback + suggestions.
		"planner": "openai/gpt-5.4-nano",
		"stager":  "openai/gpt-5.4-mini",
		"message": "openai/gpt-5.4-nano",
		"arbiter": "openai/gpt-5.4-nano",
	},
	"codex": {
		"planner": "gpt-5.4-nano",      // fast tier
		"stager":  "gpt-5.1-codex-mini", // mid tier (stager-capable)
		"message": "gpt-5.4-nano",      // fast tier
		"arbiter": "gpt-5.4-nano",      // fast tier
	},
	"cursor": {
		// cursor model names are plan-entitlement-dependent; composer-2.5* is available on ALL plans
		// (incl. free). Reasoning is baked into the model-name suffix, not a flag.
		"planner": "composer-2.5-fast", // fast tier, all-plans-available
		"stager":  "composer-2.5",      // mid tier (stager-capable)
		"message": "composer-2.5-fast", // fast tier
		"arbiter": "composer-2.5-fast", // fast tier
	},
}

// DefaultModelsForProvider returns a COPY of the named provider's role→model column from the FR-D4 table
// (PRD §9.16 FR-D4), or nil if name is not a built-in provider. The copy is defensive — callers (the
// config bootstrap, P1.M4.T2) may mutate it freely without affecting the package-level table (mirrors
// provider.BuiltinManifests' fresh-per-call discipline).
//
// The bootstrap writes the detected provider's [role.*] block from this column (FR-B1 step 3) and other
// installed providers' blocks commented (step 4). A stager value of "" means the provider cannot serve
// as the stager (nil TooledFlags) — the bootstrap applies the FR-D4 fallback (next TooledFlags-capable
// provider) on that signal. See roleDefaults' FR-D5 block for model-name provenance.
func DefaultModelsForProvider(name string) map[string]string {
	if col, ok := roleDefaults[name]; ok {
		out := make(map[string]string, len(col))
		for role, model := range col {
			out[role] = model
		}
		return out
	}
	return nil
}
