package config

import (
	"testing"
	"time"
)

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
		"planner": {Provider: "agy", Model: "codex-2.5-pro"},
	}
	p, m, r := ResolveRoleModel("planner", cfg)
	if p != "agy" || m != "codex-2.5-pro" {
		t.Errorf("ResolveRoleModel(planner) = (%q,%q), want (agy,codex-2.5-pro) [full override]", p, m)
	}
	if r != "" {
		t.Errorf("ResolveRoleModel(planner) reasoning = %q, want \"\" (off — no shipped default)", r)
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
	if r != "" {
		t.Errorf("ResolveRoleModel(planner) reasoning = %q, want \"\" (off — no shipped default)", r)
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
		"planner": {Provider: "agy", Model: "codex-2.5-pro"},
		"stager":  {Provider: "agy", Model: "codex-2.5-flash"},
	}
	want := map[string][3]string{
		"planner": {"agy", "codex-2.5-pro", ""},   // overridden provider/model; reasoning off (no shipped default)
		"stager":  {"agy", "codex-2.5-flash", ""}, // overridden provider/model; reasoning = shipped off
		"message": {"pi", "gpt-5.4", ""},          // global; reasoning = shipped off
		"arbiter": {"pi", "gpt-5.4", ""},          // global; reasoning = shipped off
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
	// Planner: per-role reasoning not set, so it inherits the global "low" (no shipped planner default anymore).
	_, _, rp := ResolveRoleModel("planner", cfg)
	if rp != "low" {
		t.Errorf("ResolveRoleModel(planner) reasoning = %q, want \"low\" [global fallback]", rp)
	}
}

func TestResolveRoleModel_NoShippedReasoningDefault(t *testing.T) {
	// FR-R6: NO role has a non-off shipped reasoning default — not even the planner. With nothing
	// set (no per-role override, no global), every role resolves reasoning to "" (off).
	cfg := Defaults()                // Roles nil, Provider/Model/Reasoning all ""
	for _, role := range roleNames { // roleNames: load.go's package-level canonical list (same package)
		_, _, r := ResolveRoleModel(role, cfg)
		if r != "" {
			t.Errorf("ResolveRoleModel(%s) reasoning = %q, want \"\" (off — no shipped default)", role, r)
		}
	}
}

func TestResolveRoleModel_ReasoningOffIsNonZero(t *testing.T) {
	cfg := Defaults()
	cfg.Reasoning = "off" // explicitly set off — non-empty, so it's a real override
	_, _, r := ResolveRoleModel("planner", cfg)
	// Explicit "off" is a real (non-empty) value, so it is respected (planner inherits the global "off").
	if r != "off" {
		t.Errorf("ResolveRoleModel(planner) reasoning = %q, want \"off\" [explicit global off respected]", r)
	}
	// Per-role "off" beats a global "high"
	cfg.Roles = map[string]RoleConfig{
		"planner": {Reasoning: "off"},
	}
	cfg.Reasoning = "high"
	_, _, rp := ResolveRoleModel("planner", cfg)
	if rp != "off" {
		t.Errorf("ResolveRoleModel(planner) reasoning = %q, want \"off\" [per-role off beats global high]", rp)
	}
}

// --- ResolveRoleTimeout tests (FR-R7 per-role generation timeout) ---

func TestResolveRoleTimeout_PerRoleOverride(t *testing.T) {
	cfg := Defaults()
	cfg.Timeout = 120 * time.Second // distinct from the override so the assertion is unambiguous
	cfg.Roles = map[string]RoleConfig{"planner": {Timeout: 600 * time.Second}}
	got := ResolveRoleTimeout("planner", cfg)
	if got != 600*time.Second {
		t.Errorf("ResolveRoleTimeout(planner) = %v, want 600s [per-role beats global 120s]", got)
	}
}

// TestResolveRoleTimeout_PlannerInheritsGlobal replaces the retired built-in test (§9.30): NO role
// ships a built-in deadline any more (the v2.8 planner-480s wall clock was a workaround for the old
// 120s global starving a thinking planner — moot under an idle deadline + a 600s hard cap). With no
// per-role override the planner must resolve to the GLOBAL, exactly like every other role.
func TestResolveRoleTimeout_PlannerInheritsGlobal(t *testing.T) {
	cfg := Defaults()
	cfg.Timeout = 120 * time.Second // explicit distinct global — makes the assertion unambiguous
	// Roles nil ⇒ no per-role override; planner (like every role now) inherits the global. No built-in.
	got := ResolveRoleTimeout("planner", cfg)
	if got != 120*time.Second {
		t.Errorf("ResolveRoleTimeout(planner) = %v, want 120s (planner inherits the global — no built-in)", got)
	}
}

func TestResolveRoleTimeout_NonPlannerGlobalFallback(t *testing.T) {
	cfg := Defaults()
	cfg.Timeout = 120 * time.Second // explicit distinct global (no built-ins exist any more)
	for _, role := range []string{"stager", "message", "arbiter"} {
		got := ResolveRoleTimeout(role, cfg)
		if got != 120*time.Second {
			t.Errorf("ResolveRoleTimeout(%s) = %v, want 120s [no built-in ⇒ global]", role, got)
		}
	}
}

func TestResolveRoleTimeout_FieldMergeTimeoutOnly(t *testing.T) {
	cfg := Defaults()
	cfg.Timeout = 120 * time.Second
	cfg.Roles = map[string]RoleConfig{"message": {Provider: "pi", Timeout: 0}} // Timeout 0 ⇒ inherit global
	got := ResolveRoleTimeout("message", cfg)
	if got != 120*time.Second {
		t.Errorf("ResolveRoleTimeout(message) = %v, want 120s [Timeout 0 inherits global; Provider is irrelevant to timeout]", got)
	}
}

func TestResolveRoleTimeout_UnknownRoleGlobalFallback(t *testing.T) {
	cfg := Defaults()
	cfg.Timeout = 120 * time.Second
	got := ResolveRoleTimeout("palnner", cfg) // typo / non-canonical name
	if got != 120*time.Second {
		t.Errorf("ResolveRoleTimeout(palnner) = %v, want 120s [unknown role ⇒ global, no built-in]", got)
	}
}

func TestResolveRoleTimeout_RolesNilGlobalFallback(t *testing.T) {
	cfg := Defaults() // Roles is nil from Defaults()
	cfg.Timeout = 120 * time.Second
	got := ResolveRoleTimeout("message", cfg)
	if got != 120*time.Second {
		t.Errorf("ResolveRoleTimeout(message) = %v, want 120s [Roles nil ⇒ global fallback]", got)
	}
}

// TestResolveRoleTimeout_MessagePerRoleOverride closes GAP1 (FR-R7 clause (a)): NO LANDED test sets a
// NON-ZERO cfg.Roles["message"].Timeout. TestResolveRoleTimeout_FieldMergeTimeoutOnly uses message with
// Timeout==0 (inherit); _PerRoleOverride uses planner. The message role is the ONLY active role on the
// single-commit path, so its non-zero per-role-override branch is the highest-value untested case — a
// refactor that dropped the non-planner per-role-override path would pass every LANDED test but fail here.
func TestResolveRoleTimeout_MessagePerRoleOverride(t *testing.T) {
	cfg := Defaults()
	cfg.Timeout = 120 * time.Second // distinct from the 90s override so the assertion is unambiguous
	cfg.Roles = map[string]RoleConfig{"message": {Timeout: 90 * time.Second}}
	got := ResolveRoleTimeout("message", cfg)
	if got != 90*time.Second {
		t.Errorf("ResolveRoleTimeout(message) = %v, want 90s [message has no built-in; per-role override beats 120s global]", got)
	}
}

// TestResolveRoleTimeout_AllCanonicalRoles closes GAP2 (FR-R7 consolidation): the timeout axis had no
// "all canonical roles" table, unlike TestResolveRoleModel_AllCanonicalRoles (line 108) for the model
// axis. One table over roleNames proves the full FR-R7 resolution matrix in a single readable place:
// planner → per-role override (beats the global); stager → per-role override (beats the 120s
// global); message/arbiter (absent from cfg.Roles, no built-in — §9.30 retired them all) → 120s global.
// The 6 LANDED ad-hoc tests cover these branches in isolation; this is the missing consolidated twin.
func TestResolveRoleTimeout_AllCanonicalRoles(t *testing.T) {
	cfg := Defaults()
	cfg.Timeout = 120 * time.Second // explicit global (no built-ins — every non-overridden role lands here)
	cfg.Roles = map[string]RoleConfig{
		"planner": {Timeout: 600 * time.Second}, // override beats the global
		"stager":  {Timeout: 60 * time.Second},  // override beats the global
		// message, arbiter: no entry ⇒ global (no built-ins exist — §9.30)
	}
	want := map[string]time.Duration{
		"planner": 600 * time.Second, // per-role override
		"stager":  60 * time.Second,  // per-role override
		"message": 120 * time.Second, // no built-in, no override ⇒ global
		"arbiter": 120 * time.Second, // no built-in, no override ⇒ global
	}
	for _, role := range roleNames { // roleNames: load.go package-level canonical list
		got := ResolveRoleTimeout(role, cfg)
		if got != want[role] {
			t.Errorf("ResolveRoleTimeout(%s) = %v, want %v", role, got, want[role])
		}
	}
}

// TestResolveRoleTimeout_PlannerDefaultFromDefaults pins the §9.30 retirement functionally: the
// planner resolves from the LITERAL Defaults() (no test-set globals) to the 600s global hard cap —
// NO built-in tier exists any more (the v2.8 480s planner wall clock is retired as moot under the
// idle+hard two-deadline model). This makes "the planner is an ordinary role" a named, greppable
// assertion.
func TestResolveRoleTimeout_PlannerDefaultFromDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.Timeout != 600*time.Second {
		t.Fatalf("Defaults().Timeout = %v, want 600s (the §9.30 global hard-cap default)", cfg.Timeout)
	}
	if got := ResolveRoleTimeout("planner", cfg); got != 600*time.Second {
		t.Errorf("ResolveRoleTimeout(planner, Defaults()) = %v, want 600s (planner inherits the global — no built-in)", got)
	}
}

// --- ResolveRoleDeadlines tests (§9.30 FR-R7/FR25 two-deadline model) ---

// TestResolveRoleDeadlines pins the two-deadline ladder: per-role override (0 ⇒ inherit) → the global
// defaults, with the two deadlines resolving INDEPENDENTLY (a role may override either alone — the
// idle-only and hard-only rows are the independent-resolution assertions). Unknown roles / nil Roles /
// zero-value RoleConfig fields all inherit the globals. NO built-in tier exists (§9.30 retired it).
func TestResolveRoleDeadlines(t *testing.T) {
	const (
		gIdle = 60 * time.Second
		gHard = 600 * time.Second
	)
	newCfg := func() Config {
		cfg := Defaults()
		cfg.IdleTimeout = gIdle
		cfg.Timeout = gHard
		return cfg
	}
	cases := []struct {
		name     string
		role     string
		roles    map[string]RoleConfig
		wantIdle time.Duration
		wantHard time.Duration
	}{
		{
			name:     "per-role BOTH overridden",
			role:     "planner",
			roles:    map[string]RoleConfig{"planner": {IdleTimeout: 5 * time.Second, Timeout: 50 * time.Second}},
			wantIdle: 5 * time.Second,
			wantHard: 50 * time.Second,
		},
		{
			name:     "per-role idle ONLY (hard stays global — independent resolution)",
			role:     "planner",
			roles:    map[string]RoleConfig{"planner": {IdleTimeout: 5 * time.Second}},
			wantIdle: 5 * time.Second,
			wantHard: gHard,
		},
		{
			name:     "per-role hard ONLY (idle stays global — independent resolution)",
			role:     "planner",
			roles:    map[string]RoleConfig{"planner": {Timeout: 50 * time.Second}},
			wantIdle: gIdle,
			wantHard: 50 * time.Second,
		},
		{
			name:     "zero-value RoleConfig fields ⇒ both globals",
			role:     "message",
			roles:    map[string]RoleConfig{"message": {}},
			wantIdle: gIdle,
			wantHard: gHard,
		},
		{
			name:     "unknown role ⇒ both globals",
			role:     "palnner", // typo / non-canonical
			roles:    map[string]RoleConfig{"planner": {IdleTimeout: 5 * time.Second, Timeout: 50 * time.Second}},
			wantIdle: gIdle,
			wantHard: gHard,
		},
		{
			name:     "nil Roles ⇒ both globals",
			role:     "planner",
			roles:    nil,
			wantIdle: gIdle,
			wantHard: gHard,
		},
		{
			name:     "a DIFFERENT role's override does not leak (planner override, message resolves)",
			role:     "message",
			roles:    map[string]RoleConfig{"planner": {IdleTimeout: 5 * time.Second, Timeout: 50 * time.Second}},
			wantIdle: gIdle,
			wantHard: gHard,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg := newCfg()
			cfg.Roles = tc.roles
			gotIdle, gotHard := ResolveRoleDeadlines(tc.role, cfg)
			if gotIdle != tc.wantIdle || gotHard != tc.wantHard {
				t.Errorf("ResolveRoleDeadlines(%s) = (%v, %v), want (%v, %v)", tc.role, gotIdle, gotHard, tc.wantIdle, tc.wantHard)
			}
		})
	}
}

// TestResolveRoleTimeout_Delegates pins the delegation contract: ResolveRoleTimeout is the THIN
// hard-cap accessor over ResolveRoleDeadlines — for every resolution shape it must return EXACTLY the
// hard deadline (never an independent copy of the ladder logic).
func TestResolveRoleTimeout_Delegates(t *testing.T) {
	cases := []struct {
		name  string
		role  string
		roles map[string]RoleConfig
	}{
		{"override both", "planner", map[string]RoleConfig{"planner": {IdleTimeout: 5 * time.Second, Timeout: 50 * time.Second}}},
		{"override hard only", "planner", map[string]RoleConfig{"planner": {Timeout: 50 * time.Second}}},
		{"override idle only", "planner", map[string]RoleConfig{"planner": {IdleTimeout: 5 * time.Second}}},
		{"no entry", "stager", nil},
		{"zero-value entry", "arbiter", map[string]RoleConfig{"arbiter": {}}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg := Defaults()
			cfg.Roles = tc.roles
			_, wantHard := ResolveRoleDeadlines(tc.role, cfg)
			if got := ResolveRoleTimeout(tc.role, cfg); got != wantHard {
				t.Errorf("ResolveRoleTimeout(%s) = %v, want the ResolveRoleDeadlines hard return %v (delegation contract)", tc.role, got, wantHard)
			}
		})
	}
}
