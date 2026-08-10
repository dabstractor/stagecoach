package config

import "testing"

// TestDefaultModelsForProvider_PerProvider asserts each of the 6 built-in providers returns its
// expected 4-role column (hardcoded — NOT derived from the table, so the test is meaningful).
// PINS a non-empty stager cell for every provider (all six are stager-capable as of 2026-07-09).
func TestDefaultModelsForProvider_PerProvider(t *testing.T) {
	want := map[string]map[string]string{
		"pi": {
			"planner": "gpt-5.4-nano", "stager": "gpt-5.4-mini", "message": "gpt-5.4-nano", "arbiter": "gpt-5.4-nano",
		},
		"claude": {
			"planner": "haiku", "stager": "sonnet", "message": "haiku", "arbiter": "haiku",
		},
		"agy": {
			"planner": "Gemini 3.5 Flash (Low)", "stager": "Gemini 3.5 Flash (Medium)", "message": "Gemini 3.5 Flash (Low)", "arbiter": "Gemini 3.5 Flash (Low)",
		},
		"opencode": {
			"planner": "openai/gpt-5.4-nano", "stager": "openai/gpt-5.4-mini", "message": "openai/gpt-5.4-nano", "arbiter": "openai/gpt-5.4-nano",
		},
		"codex": {
			"planner": "gpt-5.4-nano", "stager": "gpt-5.1-codex-mini", "message": "gpt-5.4-nano", "arbiter": "gpt-5.4-nano",
		},
		"cursor": {
			"planner": "composer-2.5-fast", "stager": "composer-2.5", "message": "composer-2.5-fast", "arbiter": "composer-2.5-fast",
		},
	}
	for name, exp := range want {
		got := DefaultModelsForProvider(name)
		if got == nil {
			t.Errorf("DefaultModelsForProvider(%q) = nil, want a column", name)
			continue
		}
		for role, m := range exp {
			if got[role] != m {
				t.Errorf("DefaultModelsForProvider(%q)[%q] = %q, want %q", name, role, got[role], m)
			}
		}
	}
}

// TestDefaultModelsForProvider_AllRolesPresent asserts every known provider column has exactly the
// 4 canonical role keys (planner/stager/message/arbiter), including stager when its value is "".
func TestDefaultModelsForProvider_AllRolesPresent(t *testing.T) {
	roles := []string{"planner", "stager", "message", "arbiter"}
	for _, name := range []string{"pi", "claude", "agy", "opencode", "codex", "cursor"} {
		col := DefaultModelsForProvider(name)
		if col == nil {
			t.Errorf("DefaultModelsForProvider(%q) = nil, want a column", name)
			continue
		}
		if len(col) != 4 {
			t.Errorf("DefaultModelsForProvider(%q) has %d roles, want 4", name, len(col))
		}
		for _, role := range roles {
			if _, ok := col[role]; !ok {
				t.Errorf("DefaultModelsForProvider(%q) missing role key %q", name, role)
			}
		}
	}
}

// TestDefaultModelsForProvider_StagerCapability asserts every built-in provider has a non-empty
// stager cell (all six — pi, claude, agy, codex, cursor, opencode — are stager-capable as of
// 2026-07-09). No built-in has stager=="" anymore.
// NOTE: pi/opencode's stager cell is a non-empty PLACEHOLDER — the bootstrap blanks their WRITTEN
// [role.*] models (power-user); the cell stays non-empty here so StagerFallback's table lookup treats
// them as capable.
func TestDefaultModelsForProvider_StagerCapability(t *testing.T) {
	for _, capable := range []string{"pi", "claude", "agy", "codex", "cursor", "opencode"} {
		if m := DefaultModelsForProvider(capable)["stager"]; m == "" {
			t.Errorf("%q should be stager-capable (non-empty stager cell), got %q", capable, m)
		}
	}
}

// TestDefaultModelsForProvider_UnknownReturnsNil asserts an unknown provider name returns nil.
func TestDefaultModelsForProvider_UnknownReturnsNil(t *testing.T) {
	if got := DefaultModelsForProvider("nonexistent"); got != nil {
		t.Errorf("DefaultModelsForProvider(\"nonexistent\") = %v, want nil", got)
	}
}

// TestDefaultModelsForProvider_CopySemantics asserts that mutating a returned map does NOT affect
// the package-level table (DefaultModelsForProvider must return a defensive copy).
func TestDefaultModelsForProvider_CopySemantics(t *testing.T) {
	first := DefaultModelsForProvider("pi")
	first["stager"] = "MUTATED"
	second := DefaultModelsForProvider("pi")
	if second["stager"] != "gpt-5.4-mini" {
		t.Errorf("table was mutated via returned map: second call stager = %q, want gpt-5.4-mini (must return a copy)", second["stager"])
	}
}

// TestRoleDefaults_KeySanity asserts the table has exactly the 6 built-in provider keys and no
// provider column contains a role key outside the canonical set {planner, stager, message, arbiter}.
func TestRoleDefaults_KeySanity(t *testing.T) {
	expectedProviders := map[string]bool{
		"pi": true, "claude": true, "opencode": true,
		"codex": true, "cursor": true, "agy": true,
	}
	validRoles := map[string]bool{
		"planner": true, "stager": true, "message": true, "arbiter": true,
	}

	if len(roleDefaults) != len(expectedProviders) {
		t.Errorf("roleDefaults has %d providers, want %d", len(roleDefaults), len(expectedProviders))
	}
	for p := range roleDefaults {
		if !expectedProviders[p] {
			t.Errorf("roleDefaults has unexpected provider key %q", p)
		}
		for role := range roleDefaults[p] {
			if !validRoles[role] {
				t.Errorf("roleDefaults[%q] has unexpected role key %q", p, role)
			}
		}
	}
}
