package config

// defaultRoleReasoning is the FR-R6 SHIPPED per-role reasoning fallback (the LOWEST resolution layer,
// below the global [defaults].reasoning): planner=high (decomposition benefits from reasoning);
// stager/message/arbiter=off. "off" is the natural "" zero value, so ONLY planner needs an entry —
// every other role's shipped default is "" (off). Applied by ResolveRoleModel after per-role and global
// are both empty. (Lives here, co-located with its sole consumer; the FR-D4 provider×role MODEL table
// is a separate concern in role_defaults.go.)
var defaultRoleReasoning = map[string]string{
	"planner": "high",
}

// ResolveRoleModel returns the (provider, model, reasoning) for a single agent role (PRD §16.4, §9.15
// FR-R1–R3/R6), applying the precedence:
//
//	CLI flag > env > [role.<role>] config   (all already merged into cfg.Roles by the loaders)
//	> [defaults] global                     (cfg.Provider / cfg.Model / cfg.Reasoning)
//	> shipped role default                  (defaultRoleReasoning: planner→high; others→"" (off))
//
// By the time this runs, Load() has already overlaid every precedence layer into cfg: the per-role flag/env/
// file/git values are per-field-merged into cfg.Roles[role], and the global layers into cfg.Provider/
// cfg.Model / cfg.Reasoning. So this function only checks the per-role entry, then falls back to the global
// for any field still empty, then (reasoning only) to the shipped role default. It does NOT re-walk the
// layers and does NOT consult any manifest.
//
// Provider, Model, and Reasoning are resolved INDEPENDENTLY (per-field, FR-R3/FR37a): a role that sets
// only its Model inherits the global Provider, and vice versa. A role absent from cfg.Roles inherits
// the global entirely.
//
// The returned ("", "", "") is the "use manifest defaults" sentinel for the downstream consumer (the
// registry / Render): model == "" => use the resolved provider manifest's default_model; provider == ""
// => the registry applies auto-detection (Registry.DefaultProvider, FR-D1); reasoning == "" => off
// (Render's ReasoningLevels table is a graceful no-op, FR-R6). ResolveRoleModel deliberately does NOT
// resolve the manifest layer itself — that is the registry's job (config must not import internal/provider).
//
// On the single-commit path the only active role is "message"; with no per-role override this returns
// (cfg.Provider, cfg.Model, cfg.Reasoning) — exactly v1 (back-compatible), plus reasoning.
//
// role is an arbitrary string (one of "planner","stager","message","arbiter" in practice); a non-canonical
// name simply misses the cfg.Roles lookup and inherits the global (no error).
func ResolveRoleModel(role string, cfg Config) (provider, model, reasoning string) {
	if rc, ok := cfg.Roles[role]; ok {
		if rc.Provider != "" {
			provider = rc.Provider
		}
		if rc.Model != "" {
			model = rc.Model
		}
		if rc.Reasoning != "" {
			reasoning = rc.Reasoning
		}
	}
	if provider == "" {
		provider = cfg.Provider
	}
	if model == "" {
		model = cfg.Model
	}
	if reasoning == "" {
		reasoning = cfg.Reasoning
	}
	if reasoning == "" {
		reasoning = defaultRoleReasoning[role] // FR-R6 shipped fallback: planner→high; others→"" (off)
	}
	return provider, model, reasoning
}
