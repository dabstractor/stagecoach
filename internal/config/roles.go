package config

import "time"

// defaultRoleTimeouts are the shipped per-role generation-timeout built-ins (PRD §16.1, FR-R7). The
// planner is the ONLY role with a built-in timeout (480s) — it does open-ended decomposition planning
// and needs longer than the message/stager/arbiter roles, which inherit cfg.Timeout. A user override
// (cfg.Roles[role].Timeout, set via [role.<role>].timeout / --<role>-timeout / env / git-config) ALWAYS
// beats this. This is a role×timeout axis, distinct from role_defaults.go's provider×role→MODEL table
// (FR-D4) — do not conflate the two. Add a role here only when shipping a non-global default.
var defaultRoleTimeouts = map[string]time.Duration{
	"planner": 480 * time.Second,
}

// ResolveRoleModel returns the (provider, model, reasoning) for a single agent role (PRD §16.4, §9.15
// FR-R1–R3/R6), applying the precedence:
//
//	CLI flag > env > [role.<role>] config   (all already merged into cfg.Roles by the loaders)
//	> [defaults] global                     (cfg.Provider / cfg.Model / cfg.Reasoning)
//
// By the time this runs, Load() has already overlaid every precedence layer into cfg: the per-role flag/env/
// file/git values are per-field-merged into cfg.Roles[role], and the global layers into cfg.Provider/
// cfg.Model / cfg.Reasoning. So this function only checks the per-role entry, then falls back to the global
// for any field still empty. It does NOT re-walk the layers and does NOT consult any manifest.
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
// Reasoning has NO shipped per-role default: every role is off out of the box (FR-R6). "off" is the
// natural "" zero value, so the global [defaults].reasoning — which config init writes as "off" (FR-B1) —
// is the only reasoning layer beneath the per-role override. A user who wants thinking on (most often
// the planner) sets it explicitly, per role or globally.
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
		reasoning = cfg.Reasoning // FR-R6: no shipped per-role default — off (== "") is the only fallback
	}
	return provider, model, reasoning
}

// ResolveRoleTimeout returns the generation timeout for a single agent role (PRD §9.15 FR-R7, §16.1),
// applying the precedence:
//
//	[role.<role>].timeout  (CLI flag > env > file > git, all already merged into cfg.Roles by the loaders)
//	> built-in role default  (planner = 480s; FR-R7 — the planner needs more time than message/stager/arbiter)
//	> [defaults].timeout     (cfg.Timeout — the global; 480s today, 120s after P1.M2.T2)
//
// By the time this runs, Load() has already overlaid every precedence layer into cfg.Roles[role].Timeout,
// so this only checks the per-role entry, then the built-in role default, then the global. It does NOT
// re-walk the layers and does NOT consult any manifest — mirroring ResolveRoleModel.
//
// The planner is the ONLY role with a shipped built-in timeout (480s): it does the open-ended
// decomposition planning and most often needs longer than the message/stager/arbiter roles (which
// inherit cfg.Timeout). A non-zero cfg.Roles[role].Timeout ALWAYS wins — even for the planner (a
// user's --planner-timeout 600s beats the 480s built-in). A role absent from cfg.Roles (or with
// Timeout==0) inherits: planner → 480s built-in; stager/message/arbiter → cfg.Timeout.
//
// The zero-value sentinel (RoleConfig.Timeout == 0 ⇒ "inherit") mirrors the "" string fields of
// ResolveRoleModel. A RESOLVED timeout should never be 0 at an Execute call site; the consumers
// (P1.M3) guard a 0 if it ever occurs (Execute treats timeout<=0 as "no deadline"). This function
// returns cfg.Timeout unchanged for non-planner roles (do not collapse a 0 global into a built-in
// here — the built-in is role-specific, planner-only).
//
// role is an arbitrary string (one of "planner","stager","message","arbiter" in practice); a
// non-canonical name misses the cfg.Roles lookup AND the built-in map, so it inherits cfg.Timeout
// (no error) — same leniency as ResolveRoleModel.
func ResolveRoleTimeout(role string, cfg Config) time.Duration {
	if rc, ok := cfg.Roles[role]; ok && rc.Timeout != 0 {
		return rc.Timeout
	}
	if d, ok := defaultRoleTimeouts[role]; ok {
		return d
	}
	return cfg.Timeout
}
