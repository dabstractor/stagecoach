package config

// ResolveRoleModel returns the (provider, model) for a single agent role (PRD §16.4, §9.15 FR-R1–R3),
// applying the precedence:
//
//	CLI flag > env > [role.<role>] config   (all already merged into cfg.Roles by the loaders)
//	> [defaults] global                     (cfg.Provider / cfg.Model)
//	> built-in manifest default             (the ("","") sentinel — see below)
//
// By the time this runs, Load() has already overlaid every precedence layer into cfg: the per-role flag/env/
// file/git values are per-field-merged into cfg.Roles[role], and the global layers into cfg.Provider/
// cfg.Model. So this function only checks the per-role entry, then falls back to the global for any field
// still empty. It does NOT re-walk the layers and does NOT consult any manifest.
//
// Provider and Model are resolved INDEPENDENTLY (per-field, FR-R3/FR37a): a role that sets only its Model
// inherits the global Provider, and vice versa. A role absent from cfg.Roles inherits the global entirely.
//
// The returned ("", "") is the "use manifest defaults" sentinel for the downstream consumer (the registry /
// Render): model == "" => use the resolved provider manifest's default_model; provider == "" => the registry
// applies auto-detection (Registry.DefaultProvider, FR-D1). ResolveRoleModel deliberately does NOT resolve
// the manifest layer itself — that is the registry's job (config must not import internal/provider).
//
// On the single-commit path the only active role is "message"; with no per-role override this returns
// (cfg.Provider, cfg.Model) — exactly v1 (back-compatible).
//
// role is an arbitrary string (one of "planner","stager","message","arbiter" in practice); a non-canonical
// name simply misses the cfg.Roles lookup and inherits the global (no error).
func ResolveRoleModel(role string, cfg Config) (provider, model string) {
	if rc, ok := cfg.Roles[role]; ok {
		if rc.Provider != "" {
			provider = rc.Provider
		}
		if rc.Model != "" {
			model = rc.Model
		}
	}
	if provider == "" {
		provider = cfg.Provider
	}
	if model == "" {
		model = cfg.Model
	}
	return provider, model
}
