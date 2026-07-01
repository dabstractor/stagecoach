package config

import "testing"

func TestResolveRoleModel_GlobalFallbackRolesNil(t *testing.T) {
	cfg := Defaults() // Roles == nil, Provider/Model == ""
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4"
	p, m := ResolveRoleModel("message", cfg)
	if p != "pi" || m != "gpt-5.4" {
		t.Errorf("ResolveRoleModel(message) = (%q,%q), want (pi,gpt-5.4) [global fallback, Roles nil]", p, m)
	}
}

func TestResolveRoleModel_GlobalFallbackRoleAbsent(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4"
	cfg.Roles = map[string]RoleConfig{"planner": {Provider: "agy"}} // other roles set, but not "message"
	p, m := ResolveRoleModel("message", cfg)
	if p != "pi" || m != "gpt-5.4" {
		t.Errorf("ResolveRoleModel(message) = (%q,%q), want (pi,gpt-5.4) [role absent ⇒ global]", p, m)
	}
}

func TestResolveRoleModel_FullOverride(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi" // global
	cfg.Roles = map[string]RoleConfig{
		"planner": {Provider: "agy", Model: "gemini-2.5-pro"},
	}
	p, m := ResolveRoleModel("planner", cfg)
	if p != "agy" || m != "gemini-2.5-pro" {
		t.Errorf("ResolveRoleModel(planner) = (%q,%q), want (agy,gemini-2.5-pro) [full override]", p, m)
	}
}

func TestResolveRoleModel_ModelOnlyOverride(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi" // global provider
	cfg.Roles = map[string]RoleConfig{
		"message": {Provider: "", Model: "gpt-5.4-nano"}, // model-only override
	}
	p, m := ResolveRoleModel("message", cfg)
	if p != "pi" || m != "gpt-5.4-nano" {
		t.Errorf("ResolveRoleModel(message) = (%q,%q), want (pi,gpt-5.4-nano) [model-only: provider inherits global]", p, m)
	}
}

func TestResolveRoleModel_ProviderOnlyOverride(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4" // global model
	cfg.Roles = map[string]RoleConfig{
		"stager": {Provider: "agy", Model: ""}, // provider-only override
	}
	p, m := ResolveRoleModel("stager", cfg)
	if p != "agy" || m != "gpt-5.4" {
		t.Errorf("ResolveRoleModel(stager) = (%q,%q), want (agy,gpt-5.4) [provider-only: model inherits global]", p, m)
	}
}

func TestResolveRoleModel_BothEmptyManifestSentinel(t *testing.T) {
	cfg := Defaults() // Roles nil, Provider/Model ""
	p, m := ResolveRoleModel("planner", cfg)
	if p != "" || m != "" {
		t.Errorf("ResolveRoleModel(planner) = (%q,%q), want (\"\",\"\") [manifest-default sentinel]", p, m)
	}
}

func TestResolveRoleModel_UnknownRoleFallsBackToGlobal(t *testing.T) {
	cfg := Defaults()
	cfg.Provider = "pi"
	cfg.Model = "gpt-5.4"
	p, m := ResolveRoleModel("palnner", cfg) // typo / non-canonical name
	if p != "pi" || m != "gpt-5.4" {
		t.Errorf("ResolveRoleModel(palnner) = (%q,%q), want (pi,gpt-5.4) [unknown role ⇒ global]", p, m)
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
	want := map[string][2]string{
		"planner": {"agy", "gemini-2.5-pro"},   // overridden
		"stager":  {"agy", "gemini-2.5-flash"}, // overridden
		"message": {"pi", "gpt-5.4"},           // global
		"arbiter": {"pi", "gpt-5.4"},           // global
	}
	for _, role := range roleNames { // roleNames is load.go's package-level canonical list (same package)
		p, m := ResolveRoleModel(role, cfg)
		w := want[role]
		if p != w[0] || m != w[1] {
			t.Errorf("ResolveRoleModel(%s) = (%q,%q), want (%q,%q)", role, p, m, w[0], w[1])
		}
	}
}
