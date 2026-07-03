package generate

import (
	"testing"

	"github.com/dustin/stagehand/internal/config"
)

func TestApplyTemplate(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		tpl  string
		want string
	}{
		{"empty template is identity", "Fix parser", "", "Fix parser"},
		{"$msg-only template is identity", "Fix parser", "$msg", "Fix parser"},
		{"suffix template", "Fix parser", "$msg (#205)", "Fix parser (#205)"},
		{"prefix template", "Fix parser", "[skip ci] $msg", "[skip ci] Fix parser"},
		{"multiple $msg occurrences", "X", "$msg-$msg", "X-X"},
		{
			"multi-line message: full message substituted, suffix lands after body",
			"Sub\n\nBody line 1\nBody line 2",
			"$msg (#205)",
			"Sub\n\nBody line 1\nBody line 2 (#205)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ApplyTemplate(tc.msg, tc.tpl)
			if got != tc.want {
				t.Errorf("ApplyTemplate(%q, %q) = %q, want %q", tc.msg, tc.tpl, got, tc.want)
			}
		})
	}
}

func TestFinalizeMessage(t *testing.T) {
	t.Run("empty cfg.Template is identity", func(t *testing.T) {
		cfg := config.Defaults()
		got := FinalizeMessage("Fix parser", cfg)
		if got != "Fix parser" {
			t.Errorf("FinalizeMessage = %q, want %q (byte-identical to today)", got, "Fix parser")
		}
	})

	t.Run("non-empty cfg.Template is applied", func(t *testing.T) {
		cfg := config.Defaults()
		cfg.Template = "$msg (#205)"
		got := FinalizeMessage("Fix parser", cfg)
		want := "Fix parser (#205)"
		if got != want {
			t.Errorf("FinalizeMessage = %q, want %q", got, want)
		}
	})
}
