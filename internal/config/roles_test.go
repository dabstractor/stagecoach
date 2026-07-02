package config

import "testing"

func TestResolveRoleModel_GlobalFallbackRolesNil(t *testing.T) {
	cfg := Defaults() // Roles == nil, Provider/Model/Reasoning == ""
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4"
	p, m, r := ResolveRoleModel("message", cfg)
	if p != "pi" || m != "gpt-5.4" {
		t.Errorf("ResolveRoleModel(message) = (%q,%q), want (pi,gpt-5.4) [global fallback, Roles nil]", p, m)
	}
	if r != "" {
		t.Errorf("ResolveRoleModel(message) reasoning = %q, want \"\" (off, no per-role or global set)", r)
	}
}

func TestResolveRoleModel_GlobalFallbackRoleAbsent(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4"
	cfg.Roles = map[string]RoleConfig{"planner": {Provider: "agy"}} // other roles set, but not "message"
	p, m, r := ResolveRoleModel("message", cfg)
	if p != "pi" || m != "gpt-5.4" {
		t.Errorf("ResolveRoleModel(message) = (%q,%q), want (pi,gpt-5.4) [role absent ⇒ global]", p, m)
	}
	if r != "" {
		t.Errorf("ResolveRoleModel(message) reasoning = %q, want \"\" (off)", r)
	}
}

func TestResolveRoleModel_FullOverride(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi" // global
	cfg.Roles = map[string]RoleConfig{
		"planner": {Provider: "agy", Model: "gemini-2.5-pro"},
	}
	p, m, r := ResolveRoleModel("planner", cfg)
	if p != "agy" || m != "gemini-2.5-pro" {
		t.Errorf("ResolveRoleModel(planner) = (%q,%q), want (agy,gemini-2.5-pro) [full override]", p, m)
	}
	if r != "high" {
		t.Errorf("ResolveRoleModel(planner) reasoning = %q, want \"high\" [FR-R6 shipped default]", r)
	}
}

func TestResolveRoleModel_ModelOnlyOverride(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi" // global provider
	cfg.Roles = map[string]RoleConfig{
		"message": {Provider: "", Model: "gpt-5.4-nano"}, // model-only override
	}
	p, m, r := ResolveRoleModel("message", cfg)
	if p != "pi" || m != "gpt-5.4-nano" {
		t.Errorf("ResolveRoleModel(message) = (%q,%q), want (pi,gpt-5.4-nano) [model-only: provider inherits global]", p, m)
	}
	if r != "" {
		t.Errorf("ResolveRoleModel(message) reasoning = %q, want \"\" (off)", r)
	}
}

func TestResolveRoleModel_ProviderOnlyOverride(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4" // global model
	cfg.Roles = map[string]RoleConfig{
		"stager": {Provider: "agy", Model: ""}, // provider-only override
	}
	p, m, r := ResolveRoleModel("stager", cfg)
	if p != "agy" || m != "gpt-5.4" {
		t.Errorf("ResolveRoleModel(stager) = (%q,%q), want (agy,gpt-5.4) [provider-only: model inherits global]", p, m)
	}
	if r != "" {
		t.Errorf("ResolveRoleModel(stager) reasoning = %q, want \"\" (off)", r)
	}
}

func TestResolveRoleModel_BothEmptyManifestSentinel(t *testing.T) {
	cfg := Defaults() // Roles nil, Provider/Model/Reasoning ""
	p, m, r := ResolveRoleModel("planner", cfg)
	if p != "" || m != "" {
		t.Errorf("ResolveRoleModel(planner) = (%q,%q), want (\"\",\"\") [manifest-default sentinel]", p, m)
	}
	if r != "high" {
		t.Errorf("ResolveRoleModel(planner) reasoning = %q, want \"high\" [FR-R6 shipped planner default]", r)
	}
}

func TestResolveRoleModel_UnknownRoleFallsBackToGlobal(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4"
	p, m, r := ResolveRoleModel("palnner", cfg) // typo / non-canonical name
	if p != "pi" || m != "gpt-5.4" {
		t.Errorf("ResolveRoleModel(palnner) = (%q,%q), want (pi,gpt-5.4) [unknown role ⇒ global]", p, m)
	}
	if r != "" {
		t.Errorf("ResolveRoleModel(palnner) reasoning = %q, want \"\" (no shipped default for unknown role)", r)
	}
}

func TestResolveRoleModel_AllCanonicalRoles(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4"
	// Override only planner + stager; leave message + arbiter on the global.
	cfg.Roles = map[string]RoleConfig{
		"planner": {Provider: "agy", Model: "gemini-2.5-pro"},
		"stager":  {Provider: "agy", Model: "gemini-2.5-flash"},
	}
	want := map[string][3]string{
		"planner": {"agy", "gemini-2.5-pro", "high"}, // overridden provider/model; reasoning = shipped high
		"stager":  {"agy", "gemini-2.5-flash", ""},   // overridden provider/model; reasoning = shipped off
		"message": {"pi", "gpt-5.4", ""},             // global; reasoning = shipped off
		"arbiter": {"pi", "gpt-5.4", ""},             // global; reasoning = shipped off
	}
	for _, role := range roleNames { // roleNames is load.go's package-level canonical list (same package)
		p, m, r := ResolveRoleModel(role, cfg)
		w := want[role]
		if p != w[0] || m != w[1] || r != w[2] {
			t.Errorf("ResolveRoleModel(%s) = (%q,%q,%q), want (%q,%q,%q)", role, p, m, r, w[0], w[1], w[2])
		}
	}
}

// --- New reasoning-specific tests ---

func TestResolveRoleModel_ReasoningPerRole(t *testing.T) {
	cfg := Defaults()
	cfg.Reasoning = "medium" // global reasoning
	cfg.Roles = map[string]RoleConfig{
		"planner": {Reasoning: "high"}, // per-role override beats global
	}
	_, _, r := ResolveRoleModel("planner", cfg)
	if r != "high" {
		t.Errorf("ResolveRoleModel(planner) reasoning = %q, want \"high\" [per-role beats global]", r)
	}
	_, _, rm := ResolveRoleModel("stager", cfg)
	if rm != "medium" {
		t.Errorf("ResolveRoleModel(stager) reasoning = %q, want \"medium\" [global fallback, no per-role]", rm)
	}
}

func TestResolveRoleModel_ReasoningGlobalFallback(t *testing.T) {
	cfg := Defaults()
	cfg.Reasoning = "low"
	cfg.Roles = map[string]RoleConfig{
		"stager": {Provider: "agy"}, // no reasoning override
	}
	_, _, r := ResolveRoleModel("stager", cfg)
	if r != "low" {
		t.Errorf("ResolveRoleModel(stager) reasoning = %q, want \"low\" [global fallback]", r)
	}
	// Planner: per-role not set, global "low" wins over shipped "high" (global is higher precedence).
	_, _, rp := ResolveRoleModel("planner", cfg)
	if rp != "low" {
		t.Errorf("ResolveRoleModel(planner) reasoning = %q, want \"low\" [global beats shipped default]", rp)
	}
}

func TestResolveRoleModel_PlannerShippedDefault(t *testing.T) {
	cfg := Defaults() // Roles nil, Provider/Model/Reasoning all ""
	p, m, r := ResolveRoleModel("planner", cfg)
	if p != "" || m != "" {
		t.Errorf("planner provider/model = (%q,%q), want (\"\",\"\") [manifest sentinel]", p, m)
	}
	if r != "high" {
		t.Errorf("planner reasoning = %q, want \"high\" [FR-R6 shipped default, global unset]", r)
	}
	// message has NO shipped non-off default:
	_, _, rm := ResolveRoleModel("message", cfg)
	if rm != "" {
		t.Errorf("message reasoning = %q, want \"\" (off — no shipped non-off default)", rm)
	}
}

func TestResolveRoleModel_ReasoningOffIsNonZero(t *testing.T) {
	cfg := Defaults()
	cfg.Reasoning = "off" // explicitly set off — non-empty, so it's a real override
	_, _, r := ResolveRoleModel("planner", cfg)
	// "off" beats the shipped "high" because global is higher precedence than shipped default
	if r != "off" {
		t.Errorf("ResolveRoleModel(planner) reasoning = %q, want \"off\" [global off beats shipped high]", r)
	}
	// Per-role "off" also beats shipped default
	cfg.Roles = map[string]RoleConfig{
		"planner": {Reasoning: "off"},
	}
	cfg.Reasoning = "high"
	_, _, rp := ResolveRoleModel("planner", cfg)
	if rp != "off" {
		t.Errorf("ResolveRoleModel(planner) reasoning = %q, want \"off\" [per-role off beats global high]", rp)
	}
}
