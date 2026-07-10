package config

import (
	"reflect"
	"testing"
)

// TestActiveSettings verifies the FR-B8/B9 active-settings scanner: it must detect every uncommented
// key=value grouped by [table] heading, ignore comments/blanks, and treat top-level keys as the ""
// section. This is the shared helper for all three validation-report fixes (Issues 1–3).
func TestActiveSettings(t *testing.T) {
	t.Run("inert all-commented file yields empty map", func(t *testing.T) {
		inert := `# All-commented reference template (inert — zero active settings)
# config_version = 3
# [defaults]
# provider = "pi"
# model = ""
`
		got := ActiveSettings(inert)
		if len(got) != 0 {
			t.Errorf("ActiveSettings(inert) = %v, want empty map", got)
		}
		if !IsInert(inert) {
			t.Error("IsInert(inert) = false, want true")
		}
	})

	t.Run("active top-level + table keys grouped by section", func(t *testing.T) {
		content := `config_version = 3
[defaults]
provider = "claude"
model = "sonnet"
timeout = "60s"
# verbose = false   # commented-out — NOT active

[role.planner]
provider = "agy"
model = "gemini-3.1-pro"
`
		got := ActiveSettings(content)
		want := map[string]map[string]string{
			"":              {"config_version": "3"},
			"defaults":      {"provider": "\"claude\"", "model": "\"sonnet\"", "timeout": "\"60s\""},
			"role.planner":  {"provider": "\"agy\"", "model": "\"gemini-3.1-pro\""},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ActiveSettings =\n%v\nwant\n%v", got, want)
		}
		if IsInert(content) {
			t.Error("IsInert = true, want false (file has active settings)")
		}
	})

	t.Run("boolean and int and array values are active", func(t *testing.T) {
		// FR-B8/B9 care about ANY active key=value, not just strings.
		content := `[generation]
max_diff_bytes = 300000
exclude = ["a", "b"]
strip_code_fence = true
`
		got := ActiveSettings(content)
		gen := got["generation"]
		if gen["max_diff_bytes"] != "300000" {
			t.Errorf("max_diff_bytes = %q, want 300000", gen["max_diff_bytes"])
		}
		if gen["exclude"] != `["a", "b"]` {
			t.Errorf("exclude = %q, want [\"a\", \"b\"]", gen["exclude"])
		}
		if gen["strip_code_fence"] != "true" {
			t.Errorf("strip_code_fence = %q, want true", gen["strip_code_fence"])
		}
		if IsInert(content) {
			t.Error("IsInert = true, want false")
		}
	})

	t.Run("commented table header does not create a section", func(t *testing.T) {
		// The exampleConfigTemplate has only commented headers — inert.
		content := `# [defaults]
# provider = "pi"
# [generation]
# max_diff_bytes = 300000
`
		if !IsInert(content) {
			t.Errorf("IsInert = false, want true (all-commented headers/keys)")
		}
	})

	t.Run("empty file is inert", func(t *testing.T) {
		if !IsInert("") {
			t.Error("IsInert(\"\") = false, want true")
		}
		if !IsInert("# just a comment\n# another\n") {
			t.Error("IsInert(comments-only) = false, want true")
		}
	})

	t.Run("array-of-tables header tracked as section", func(t *testing.T) {
		content := `[[foo]]
bar = "baz"
`
		got := ActiveSettings(content)
		if got["foo"]["bar"] != "\"baz\"" {
			t.Errorf("ActiveSettings[[foo]] = %v, want foo.bar=\"baz\"", got)
		}
	})
}

// TestIsInert_RealTemplate confirms the shipped exampleConfigTemplate (all-commented) is classified
// inert — the exact false-alarm surface FR-B9 was written to kill (Issue 2 in the validation report).
func TestIsInert_RealBootstrapTemplate(t *testing.T) {
	// A populated bootstrap config is NOT inert (uncommented config_version/provider/[role.*]).
	populated := GenerateBootstrapConfig("pi")
	if IsInert(populated) {
		t.Error("GenerateBootstrapConfig(pi) is inert; want active (has uncommented keys)")
	}
	// It must have active settings under "" (config_version), "defaults" (provider), and role.* .
	as := ActiveSettings(populated)
	if as[""]["config_version"] == "" {
		t.Error("bootstrap missing top-level config_version")
	}
	if as["defaults"]["provider"] == "" {
		t.Error("bootstrap missing [defaults] provider")
	}
}
