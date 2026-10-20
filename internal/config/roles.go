package config

import "time"

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

// ResolveRoleDeadlines returns the (idle, hard) generation deadlines for a single agent role (PRD
// §9.30 FR-R7/FR25), applying the per-role→global ladder (NO built-in tier):
//
//	[role.<role>].idle_timeout / .timeout   (CLI flag > env > file > git, all already merged into cfg.Roles by the loaders; 0 ⇒ inherit)
//	> [defaults].idle_timeout / .timeout    (cfg.IdleTimeout / cfg.Timeout — the global Defaults() seeds 60s/600s)
//
// The two deadlines resolve INDEPENDENTLY — a role may override either alone (an idle-only override
// keeps the global hard cap; a hard-only override keeps the global idle deadline). IDLE is the PRIMARY
// productivity signal (kill when the agent goes quiet for idle); HARD is the absolute backstop (kill
// when the total elapsed time exceeds hard, hung or not — §9.30: a wall clock alone cannot distinguish
// a hung agent from a productive-but-slow one).
//
// NO role ships a built-in deadline: the v2.8 per-role wall-clock built-in (a planner-only special
// case) is RETIRED as moot — it existed only because the old 120s global starved a thinking planner,
// and under an idle deadline + a 600s hard cap every role is already granted as much time as it stays
// productive. The planner inherits the globals exactly like every other role. role is an arbitrary
// string (canonical: planner/stager/message/arbiter); a non-canonical name or an absent Roles entry
// inherits both globals — same leniency as ResolveRoleModel.
func ResolveRoleDeadlines(role string, cfg Config) (idle, hard time.Duration) {
	idle, hard = cfg.IdleTimeout, cfg.Timeout
	if rc, ok := cfg.Roles[role]; ok {
		if rc.IdleTimeout != 0 {
			idle = rc.IdleTimeout
		}
		if rc.Timeout != 0 {
			hard = rc.Timeout
		}
	}
	return idle, hard
}

// ResolveRoleTimeout returns the HARD-CAP generation deadline for a single agent role (PRD §9.15
// FR-R7, §9.30) — the thin hard-cap accessor over ResolveRoleDeadlines (which owns the whole ladder):
// per-role override (cfg.Roles[role].Timeout, 0 ⇒ inherit) > the global cfg.Timeout (Defaults() seeds
// 600s; no role ships a built-in — the retired v2.8 planner wall-clock built-in is gone, §9.30).
//
// KEPT for its 11 existing callers (decompose arbiter/planner/stager, generate message/multiturn/
// workdesc/generate, hook/exec); new code should prefer ResolveRoleDeadlines (which also returns the
// idle deadline — the PRIMARY §9.30 productivity signal). The zero-value sentinel
// (RoleConfig.Timeout == 0 ⇒ "inherit") mirrors the "" string fields of ResolveRoleModel; a RESOLVED
// timeout should never be 0 at an Execute call site (Execute treats timeout<=0 as "no deadline").
//
// role is an arbitrary string (canonical: planner/stager/message/arbiter); a non-canonical name simply
// misses the cfg.Roles lookup and inherits the global (no error) — same leniency as ResolveRoleModel.
func ResolveRoleTimeout(role string, cfg Config) time.Duration {
	_, hard := ResolveRoleDeadlines(role, cfg)
	return hard
}
