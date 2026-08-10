package config

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// ---------------------------------------------------------------------------
// buildBootstrapConfig — pure unit tests (no Execute, no $PATH)
// Moved from internal/cmd/config_test.go (P1.M4.T4.S1 — byte-identical).
// ---------------------------------------------------------------------------

func TestBuildBootstrapConfig_Pi(t *testing.T) {
	content := buildBootstrapConfig("pi", []string{"pi"}, nil)

	// config_version = 3 uncommented (CurrentConfigVersion)
	if !strings.Contains(content, fmt.Sprintf("config_version = %d", CurrentConfigVersion)) {
		t.Errorf("missing config_version = %d", CurrentConfigVersion)
	}

	// provider = "pi" uncommented
	if !strings.Contains(content, `provider = "pi"`) {
		t.Error("missing provider = \"pi\"")
	}

	// reasoning = "off" uncommented in [defaults] (FR-B1 — emitted so the field is discoverable
	// in the generated file rather than hidden; off is the shipped default for every role, FR-R6)
	if !strings.Contains(content, `reasoning = "off"`) {
		t.Error("missing uncommented reasoning = \"off\" in [defaults]")
	}

	// pi's four role models blanked (no sub-provider in bootstrap — pi picks its own backend default)
	assertContains(t, content, "[role.planner]", `model = ""`)
	assertContains(t, content, "[role.stager]", `model = ""`)
	assertContains(t, content, "[role.message]", `model = ""`)
	assertContains(t, content, "[role.arbiter]", `model = ""`)

	// Negative: a pi-only config must contain NO gpt-5.4 anywhere (catches the stager re-pull bug)
	if strings.Contains(content, "gpt-5.4") {
		t.Errorf("pi bootstrap must not ship un-routable gpt-5.4* models; got:\n%s", content)
	}

	// Multi-backend model-prefix annotation present
	if !strings.Contains(content, "prefix the model with your inference backend") {
		t.Error("pi bootstrap missing the model-prefix annotation")
	}

	// pi IS stager-capable — no fallback annotation
	if strings.Contains(content, "cannot serve as the stager") {
		t.Error("pi config should NOT have stager fallback annotation")
	}

	// No other-provider commented blocks (only pi in installed)
	if strings.Contains(content, "=== claude (installed)") {
		t.Error("pi-only config should not have claude commented block")
	}

	// Regression for Issue 2 (P1.M2.T7.S1): the git-config hint must advertise the SETTABLE
	// camelCase key (stagecoach.autoStageAll). git rejects underscores in the final config-key
	// segment, so shipping `git config stagecoach.auto_stage_all` gives users `error: invalid key`.
	// NOTE: assert on the git-config hint only — the TOML field `auto_stage_all` (snake_case) is
	// correct and remains elsewhere in this file.
	if strings.Contains(content, "git config stagecoach.auto_stage_all") {
		t.Errorf("bootstrap config advertises un-settable snake_case git key stagecoach.auto_stage_all; use camelCase autoStageAll")
	}
	if !strings.Contains(content, "stagecoach.autoStageAll") {
		t.Errorf("bootstrap config missing camelCase git key stagecoach.autoStageAll")
	}
}

func TestBuildBootstrapConfig_AgyStagerCapable(t *testing.T) {
	content := buildBootstrapConfig("agy", nil, nil)

	// provider = "agy"
	if !strings.Contains(content, `provider = "agy"`) {
		t.Error("missing provider = \"agy\"")
	}

	// agy is stager-capable now (§12.5.1.1 item 4) — fast-by-default planner.
	assertContains(t, content, "[role.planner]", `model = "Gemini 3.5 Flash (Low)"`)

	// stager stays on agy (inherits [defaults]; mid tier), NOT routed to pi.
	if strings.Contains(content, `provider = "pi"`) {
		t.Errorf("agy must not fall back to pi for the stager (it is stager-capable); got:\n%s", content)
	}
	if strings.Contains(content, "cannot serve as the stager") {
		t.Errorf("agy is stager-capable; the config should not carry a stager-fallback annotation")
	}

	// agy's message and arbiter (fast tier)
	assertContains(t, content, "[role.message]", `model = "Gemini 3.5 Flash (Low)"`)
	assertContains(t, content, "[role.arbiter]", `model = "Gemini 3.5 Flash (Low)"`)
}

func TestBuildBootstrapConfig_OtherInstalledCommented(t *testing.T) {
	content := buildBootstrapConfig("pi", []string{"pi", "claude"}, nil)

	// The TARGET's (pi) per-role blocks are COMMENTED with blank models (pi is multi-backend — no
	// good default; the user supplies the inference/model prefix). No provider line: the roles inherit
	// [defaults].provider = "pi".
	assertContains(t, content, "# [role.planner]", `# model = ""`)
	assertContains(t, content, "# [role.message]", `# model = ""`)

	// claude (other installed) appears as a commented block too, with its real default models.
	if !strings.Contains(content, "=== claude (installed)") {
		t.Error("missing claude commented block header")
	}
	if !strings.Contains(content, `# provider = "claude"`) {
		t.Error("missing commented claude provider line")
	}
	if !strings.Contains(content, `# model = "haiku"`) {
		t.Error("missing commented claude haiku model")
	}

	// NO uncommented [role.*] blocks anywhere: every role block is commented (a role with no
	// uncommented [role.*] inherits [defaults]). Guards against regressing back to active target blocks.
	if got := strings.Count(content, "\n[role."); got != 0 {
		t.Errorf("expected 0 uncommented [role.*] blocks (all commented); got %d", got)
	}
}

// TestBuildBootstrapConfig_CommentedPiBlockBlanked guards Issue 2 (FR-R5b/FR-B1): when a non-pi
// target is generated with pi ALSO installed, the commented-out pi provider block must ship BLANK
// models + a multi-backend guidance NOTE — NOT the bare gpt-5.4* defaults that are a hard FR-R5b
// error when uncommented. Mirror of TestBuildBootstrapConfig_OtherInstalledCommented (which proves
// the non-pi claude block is NOT blanked). See findings §1/§2/§7.
func TestBuildBootstrapConfig_CommentedPiBlockBlanked(t *testing.T) {
	content := buildBootstrapConfig("claude", []string{"claude", "pi"}, nil)

	// (a) commented pi block header present
	if !strings.Contains(content, "# === pi (installed)") {
		t.Fatalf("missing commented pi block header; content:\n%s", content)
	}

	piBlock := extractCommentedProviderBlock(content, "pi") // scope assertions to the pi block only

	// (b) ships BLANK models (`# model = ""`)
	if !strings.Contains(piBlock, `# model = ""`) {
		t.Errorf("commented pi block missing blank `# model = \"\"`; pi block:\n%s", piBlock)
	}
	// all FOUR roles blanked
	if got := strings.Count(piBlock, `# model = ""`); got != 4 {
		t.Errorf("commented pi block: want 4 blank `# model = \"\"` (planner/stager/message/arbiter), got %d; pi block:\n%s", got, piBlock)
	}

	// (c) multi-backend guidance NOTE present
	if !strings.Contains(piBlock, "multi-backend provider") {
		t.Errorf("commented pi block missing the multi-backend guidance NOTE; pi block:\n%s", piBlock)
	}

	// (d) NEGATIVE: no BARE gpt-5.4* model ASSIGNMENT (the actual bug shape).
	// NOTE: the literal substring "gpt-5.4" ALSO appears inside S1's NOTE example
	// (`# e.g. model = "zai/gpt-5.4"`) — a slash-PREFIXED model in a `# e.g.` comment, which is
	// CORRECT and must NOT trip the guard. So we assert on the bare ASSIGNMENT form
	// `# model = "gpt-5.4`, which matches the OLD bug and only the old bug. See findings §2.
	if strings.Contains(piBlock, `# model = "gpt-5.4`) {
		t.Errorf("commented pi block must not ship bare gpt-5.4* model assignments (FR-R5b); pi block:\n%s", piBlock)
	}

	// Companion sanity: the ACTIVE claude role models are NOT blanked (claude has no ProviderFlag →
	// bare aliases are legal). Confirms the pi-blank didn't collateral-damage the active block.
	if !strings.Contains(content, `model = "haiku"`) {
		t.Errorf("active claude block unexpectedly missing `model = \"haiku\"` (claude must NOT be blanked)")
	}
}

func TestBuildBootstrapConfig_NoInstallFallback(t *testing.T) {
	content := buildBootstrapConfig("pi", nil, nil)

	// Should have the fallback annotation on the provider line
	if !strings.Contains(content, "no built-in agent detected on $PATH") {
		t.Error("missing no-install fallback annotation")
	}
}

func TestBuildBootstrapConfig_ValidTOML(t *testing.T) {
	cases := []struct {
		target    string
		installed []string
	}{
		{"pi", []string{"pi"}},
		{"pi", []string{"pi", "claude"}},
		{"claude", []string{"claude"}},
		{"claude", []string{"claude", "pi"}},
		{"agy", []string{"agy", "pi", "claude"}},
		{"opencode", nil},
	}
	for _, tc := range cases {
		t.Run(tc.target+"_"+strings.Join(tc.installed, ","), func(t *testing.T) {
			content := buildBootstrapConfig(tc.target, tc.installed, nil)
			var m map[string]any
			if err := toml.Unmarshal([]byte(content), &m); err != nil {
				t.Errorf("buildBootstrapConfig(%q, %v) produced invalid TOML: %v", tc.target, tc.installed, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GenerateBootstrapConfig — shared entry point tests (P1.M4.T4.S1)
// ---------------------------------------------------------------------------

func TestGenerateBootstrapConfig_AutoDetectPi(t *testing.T) {
	content := GenerateBootstrapConfig("")

	// Should contain provider = "pi" (nothing on $PATH in CI → fallback)
	if !strings.Contains(content, `provider = "pi"`) {
		t.Error("expected provider = \"pi\" (auto-detect fallback)")
	}

	// Should be valid TOML
	var m map[string]any
	if err := toml.Unmarshal([]byte(content), &m); err != nil {
		t.Fatalf("GenerateBootstrapConfig(\"\") produced invalid TOML: %v", err)
	}

	// config_version = 3 (CurrentConfigVersion)
	if cv, ok := m["config_version"]; !ok || cv != int64(CurrentConfigVersion) {
		t.Errorf("config_version = %v, want %d", cv, CurrentConfigVersion)
	}
}

func TestGenerateBootstrapConfig_NamedProvider(t *testing.T) {
	content := GenerateBootstrapConfig("claude")

	if !strings.Contains(content, `provider = "claude"`) {
		t.Error("expected provider = \"claude\"")
	}

	// claude's role models
	assertContains(t, content, "[role.planner]", `model = "haiku"`)
	assertContains(t, content, "[role.stager]", `provider = "claude"`)
	assertContains(t, content, "[role.stager]", `model = "sonnet"`)
	assertContains(t, content, "[role.message]", `model = "haiku"`)

	// Valid TOML
	var m map[string]any
	if err := toml.Unmarshal([]byte(content), &m); err != nil {
		t.Fatalf("GenerateBootstrapConfig(\"claude\") produced invalid TOML: %v", err)
	}
}

// TestBuildBootstrapConfig_HeaderDocumentsReasoningEnvVars guards Issue 4: the generated config
// header must document the FR-R6 reasoning env vars (global + per-role), matching docs/cli.md.
func TestBuildBootstrapConfig_HeaderDocumentsReasoningEnvVars(t *testing.T) {
	content := buildBootstrapConfig("pi", nil, nil)
	assertContains(t, content,
		"STAGECOACH_REASONING",
		"STAGECOACH_<ROLE>_REASONING",
	)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// extractCommentedProviderBlock returns the commented-provider block for `provider`: from the
// `# === <provider> (installed)` header up to (not including) the next section boundary (another
// commented-provider block, the [generation] dashed separator, or EOF). Idiom mirrors
// extractStagerBlock. Scopes assertions to a single commented provider so active role models
// elsewhere don't interfere. See findings §4.
func extractCommentedProviderBlock(content, provider string) string {
	header := "# === " + provider + " (installed)"
	start := strings.Index(content, header)
	if start < 0 {
		return ""
	}
	rest := content[start:]
	nextIdx := len(rest)
	for _, marker := range []string{"\n# ===", "\n# ---"} {
		if i := strings.Index(rest[1:], marker); i >= 0 && i+1 < nextIdx {
			nextIdx = i + 1
		}
	}
	return rest[:nextIdx]
}

// assertContains checks that content contains all the specified substrings.
func assertContains(t *testing.T, content string, substrs ...string) {
	t.Helper()
	for _, s := range substrs {
		if !strings.Contains(content, s) {
			t.Errorf("content missing %q", s)
		}
	}
}

// TestRewriteHeaderForLocalScope covers config.RewriteHeaderForLocalScope — the header-scope
// transform `config init --local` applies so a repo-local .stagecoach.toml does not claim to be the
// GLOBAL file. It rewrites exactly the two scope-sensitive passages (the precedence line + the
// "This is the GLOBAL file" note), preserves everything else, and is idempotent.
func TestRewriteHeaderForLocalScope(t *testing.T) {
	global := GenerateBootstrapConfig("pi") // carries bootstrapHeader (GLOBAL framing)

	// Fixture sanity: the source sentences are present.
	if !strings.Contains(global, "This is the GLOBAL file") {
		t.Fatalf("test fixture: global config missing the GLOBAL scope note")
	}
	if !strings.Contains(global, "repo-local .stagecoach.toml  >  THIS global file") {
		t.Fatalf("test fixture: global config missing the GLOBAL precedence line")
	}

	local := RewriteHeaderForLocalScope(global)

	// Source scope sentences gone; target sentences present.
	if strings.Contains(local, "This is the GLOBAL file") {
		t.Errorf("GLOBAL scope note not rewritten to REPO-LOCAL")
	}
	if !strings.Contains(local, "This is the REPO-LOCAL config") {
		t.Errorf("REPO-LOCAL scope note not present after rewrite")
	}
	if !strings.Contains(local, "THIS file (.stagecoach.toml)") {
		t.Errorf("precedence line not rewritten to local framing")
	}
	if strings.Contains(local, "THIS global file") {
		t.Errorf("precedence line still says \"THIS global file\"")
	}

	// Idempotent: reapplying is a no-op (the source substrings are gone).
	if RewriteHeaderForLocalScope(local) != local {
		t.Errorf("RewriteHeaderForLocalScope is not idempotent")
	}

	// Only the two scope passages change: the body (config_version, [defaults], roles, [generation]
	// keys) is byte-identical. Prove it by checking the rewritten content still carries everything
	// and that the ONLY deltas are the two known substitutions.
	for _, mustKeep := range []string{"config_version = 3", "[defaults]", "[generation]", "[role.message]", "# format"} {
		if !strings.Contains(local, mustKeep) {
			t.Errorf("rewrite dropped non-scope content %q", mustKeep)
		}
	}
	// Reverse-engineer the delta: global vs local differ ONLY by the two substitutions.
	restored := strings.Replace(local, "THIS file (.stagecoach.toml)  >  the global config file", "repo-local .stagecoach.toml  >  THIS global file", 1)
	restored = strings.Replace(restored, "This is the REPO-LOCAL config (./.stagecoach.toml). It overrides the global config file;\n# repo git config (stagecoach.*), STAGECOACH_* env vars, and CLI flags override it.", "This is the GLOBAL file. A repo-local file (./.stagecoach.toml) and repo git config (stagecoach.*)\n# both override it; CLI flags and env vars override those.", 1)
	if restored != global {
		t.Errorf("rewrite touched more than the two scope passages; diff remains after restoring them")
	}
}

// TestBuildBootstrapConfig_RoleBlocksAllCommented locks in the bootstrap shape (the FR-B1 amendment
// the user directed): the target's four [role.*] blocks are ALWAYS commented — a role with no
// uncommented [role.*] inherits [defaults], so the config works with just [defaults] above. The
// multi-backend pi/opencode ship BLANK commented models (no good default; the user supplies the
// inference/model prefix); every other provider ships the smallest/fastest appropriate default,
// ready to uncomment. The stager emits a provider line ONLY on a real fallback (a different provider
// than [defaults]) — never a redundant echo of [defaults] (the original complaint).
func TestBuildBootstrapConfig_RoleBlocksAllCommented(t *testing.T) {
	t.Run("pi: commented, BLANK, no provider line (multi-backend — no default)", func(t *testing.T) {
		content := buildBootstrapConfig("pi", []string{"pi"}, nil)
		for _, role := range []string{"planner", "stager", "message", "arbiter"} {
			blk := extractCommentedTargetRoleBlock(content, role)
			if !strings.Contains(blk, "# [role."+role+"]") {
				t.Errorf("[role.%s] is not a commented block; got:\n%s", role, blk)
			}
			if !strings.Contains(blk, `# model = ""`) {
				t.Errorf("[role.%s] should ship a BLANK model (pi multi-backend); block:\n%s", role, blk)
			}
			if strings.Contains(blk, "# provider =") {
				t.Errorf("[role.%s] must NOT carry a provider line (inherits [defaults]=pi); block:\n%s", role, blk)
			}
		}
	})
	t.Run("claude: commented, filled with the smallest/fastest default, no provider line", func(t *testing.T) {
		content := buildBootstrapConfig("claude", []string{"claude"}, nil)
		// claude's shipped defaults (FR-D4): planner/message/arbiter = haiku, stager = sonnet.
		want := map[string]string{"planner": "haiku", "stager": "sonnet", "message": "haiku", "arbiter": "haiku"}
		for role, model := range want {
			blk := extractCommentedTargetRoleBlock(content, role)
			if !strings.Contains(blk, "# [role."+role+"]") {
				t.Errorf("[role.%s] missing", role)
			}
			if !strings.Contains(blk, `# model = "`+model+`"`) {
				t.Errorf("[role.%s] should ship default model %q (commented); block:\n%s", role, model, blk)
			}
			if strings.Contains(blk, "# provider =") {
				t.Errorf("[role.%s] must NOT carry a provider line (inherits [defaults]=claude); block:\n%s", role, blk)
			}
		}
	})

}

// extractCommentedTargetRoleBlock returns the commented [role.<role>] block that belongs to the
// TARGET provider — the one under the "# --- per-role models for the default provider" header, BEFORE
// any "# === <other> (installed)" alternate-provider section — so assertions are not confused by the
// alternate-provider commented blocks further down. Boundaries: from "# [role.<role>]" up to the next
// "# [role." or "[generation]" line, capped at the first alternate-provider header.
func extractCommentedTargetRoleBlock(content, role string) string {
	header := "# [role." + role + "]"
	start := strings.Index(content, header)
	if start < 0 {
		return ""
	}
	end := len(content)
	if cut := strings.Index(content, "\n# === "); cut >= 0 && cut > start {
		end = cut // the target's role blocks precede the first alternate-provider header
	}
	rest := content[start:end]
	nextIdx := len(rest)
	for _, marker := range []string{"\n# [role.", "\n[generation"} {
		if i := strings.Index(rest[1:], marker); i >= 0 && i+1 < nextIdx {
			nextIdx = i + 1
		}
	}
	return rest[:nextIdx]
}
