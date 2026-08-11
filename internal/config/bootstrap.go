package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dabstractor/stagecoach/internal/provider"
)

// preferredBuiltins is the FR-D1 cascading provider priority order (local copy — mirrors
// internal/provider/registry.go's unexported preferredBuiltins). Used by stagerFallback + commented-block
// ordering. (Moved from internal/cmd/config.go; P1.M4.T4.S1.)
var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "codex", "claude"}

// GenerateBootstrapConfig returns the populated bootstrap TOML (PRD §9.17 FR-B1/B3). provider != "" is
// used directly (caller validates); "" ⇒ cascading auto-detect (FR-D1) ⇒ "pi" fallback. NO I/O; $PATH
// detection via the registry. Shared by `config init` and the Load() first-run fallback. (P1.M4.T4.S1.)
// Delegates to GenerateBootstrapConfigWithOverrides(prov, nil) — byte-identical to the pre-refactor
// output (nil overrides = no edits → golden test).
func GenerateBootstrapConfig(prov string) string {
	return GenerateBootstrapConfigWithOverrides(prov, nil)
}

// GenerateBootstrapConfigWithOverrides returns the populated bootstrap TOML with optional per-role
// model overrides applied (role→model: "planner"|"stager"|"message"|"arbiter" → model string).
// overrides is applied AFTER the pi-blank + stagerFallback computation (so structural routing is
// preserved; only MODEL values change). nil/empty overrides ⇒ byte-identical to GenerateBootstrapConfig.
// The interactive wizard (FR-L3, PRD §9.23/§15.3) calls this seam.
func GenerateBootstrapConfigWithOverrides(prov string, overrides map[string]string) string {
	reg := provider.NewRegistry(nil) // built-ins only
	installed := bootstrapProviderNames(reg)
	target := prov
	if target == "" {
		if det := reg.DefaultProvider(installed); det != "" {
			target = det
		} else {
			target = "pi" // nothing on $PATH — valid default; annotated by buildBootstrapConfig
		}
	}
	return buildBootstrapConfig(target, installed, overrides)
}

// bootstrapWriteConfig writes the populated bootstrap config to path (MkdirAll + WriteFile), used by the
// Load() first-run fallback (FR-B3). Returns a wrapped error on failure. (P1.M4.T4.S1.)
func bootstrapWriteConfig(path string) error {
	content := GenerateBootstrapConfig("")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

// bootstrapProviderNames returns built-in provider names whose command is on $PATH (moved from cmd's
// configInitInstalledNames). reg.List() is sorted ascending.
func bootstrapProviderNames(reg *provider.Registry) []string {
	var installed []string
	for _, m := range reg.List() {
		if reg.IsInstalled(m) {
			installed = append(installed, m.Name)
		}
	}
	return installed
}

// stagerFallback returns the (provider, model) for the [role.stager] block: target's own if
// stager-capable (models["stager"] != ""), else the first stager-capable provider in preferredBuiltins
// order. Always resolves to "pi" today (pi and claude are the only stager-capable providers; pi is first).
// StagerFallback returns the (provider, model) for the [role.stager] block: target's own if
// stager-capable (models["stager"] != ""), else the first stager-capable provider in preferredBuiltins
// order. Exported for use by the interactive wizard (P1.M6.T2.S1).
func StagerFallback(target string, models map[string]string) (string, string) {
	if m := models["stager"]; m != "" {
		return target, m
	}
	for _, name := range preferredBuiltins {
		if col := DefaultModelsForProvider(name); col != nil && col["stager"] != "" {
			return name, col["stager"]
		}
	}
	return target, models["stager"] // unreachable (pi is always stager-capable) — defensive
}

// isInstalledName reports whether name is in the installed list.
func isInstalledName(name string, installed []string) bool {
	for _, n := range installed {
		if n == name {
			return true
		}
	}
	return false
}

// writeCommentedRoleBlock writes a fully-commented [role.<r>] block with NO separator — consecutive
// blocks form one solid commented run (the caller emits any leading blank line). The provider line is
// OMITTED when prov is empty — the role then inherits [defaults].provider; it is emitted only when the
// role routes to a DIFFERENT provider than [defaults] (e.g. a stager that fell back to pi), never as a
// redundant echo of [defaults]. model is always written: blank "" when no good default exists (the
// multi-backend pi/opencode, which the user must supply), or the shipped smallest/fastest tier for
// every other provider. Used for BOTH the target provider's own per-role blocks and the
// alternate-installed-provider blocks, so the two never drift in shape.
func writeCommentedRoleBlock(b *strings.Builder, role, prov, model string) {
	fmt.Fprintf(b, "# [role.%s]\n", role)
	if prov != "" {
		fmt.Fprintf(b, "# provider = %q\n", prov)
	}
	fmt.Fprintf(b, "# model = %q\n", model)
}

// applyOverrides applies per-role model overrides onto the computed models map and stager model.
// overrides control only MODEL values — stagerName (structural routing) is untouched.
// A nil overrides map is a no-op (byte-identity contract: GenerateBootstrapConfig delegates with nil).
func applyOverrides(models map[string]string, stagerModel *string, overrides map[string]string) {
	if overrides == nil {
		return
	}
	for _, role := range []string{"planner", "message", "arbiter"} {
		if v, ok := overrides[role]; ok {
			models[role] = v
		}
	}
	if v, ok := overrides["stager"]; ok {
		*stagerModel = v
	}
}

// buildBootstrapConfig is the PURE populated-config generator (PRD §9.17 FR-B1). NO detection, NO I/O —
// takes an already-resolved target + the installed list + optional per-role model overrides, returns the
// exact TOML. Deterministic ⇒ unit-testable. Writes: header docs, config_version (uncommented),
// [defaults] provider=<target> (uncommented), four COMMENTED [role.*] blocks for target (models from
// DefaultModelsForProvider, overridden by overrides — blank for the multi-backend pi/opencode, the
// smallest/fastest shipped default otherwise; stager routed to the fallback when target can't stage,
// annotated), each OTHER installed provider as a commented [role.*] group, then a commented
// [generation] section. overrides is applied AFTER pi-blank + stagerFallback (structural routing
// preserved; only MODEL values change). nil/empty overrides ⇒ no edits.
func buildBootstrapConfig(target string, installed []string, overrides map[string]string) string {
	var b strings.Builder

	// --- header (precedence/env/git/cli docs — shared with the inert template) ---
	b.WriteString(bootstrapHeader)

	// config_version (UNCOMMENTED — F6)
	fmt.Fprintf(&b, "config_version = %d\n", CurrentConfigVersion)

	// [defaults] — provider uncommented, rest commented
	b.WriteString("\n# [defaults] — top-level Stagecoach behavior\n")
	b.WriteString("[defaults]\n")
	fmt.Fprintf(&b, "provider = %q", target)
	if !isInstalledName(target, installed) {
		b.WriteString("  # no built-in agent detected on $PATH; defaulted to \"pi\" — edit if you use a different agent")
	}
	b.WriteString("\n")
	b.WriteString("reasoning = \"off\"   # off|low|medium|high; off by default for every role — opt in per role below\n")
	b.WriteString("# model          = \"\"\n# timeout        = \"120s\"\n# auto_stage_all = true\n# verbose        = false\n")

	// [role.*] for the target (UNCOMMENTED), canonical order: planner, stager, message, arbiter
	models := DefaultModelsForProvider(target) // non-nil (target is a validated built-in)
	piBlanked := target == "pi"
	opencodeBlanked := target == "opencode" // power-user wildcard: plan-/backend-dependent
	if piBlanked || opencodeBlanked {
		// pi is multi-backend (FR-R5b: a bare model is an error) and opencode's provider-prefixed models
		// are plan-dependent wildcards. Blank the written [role.*] models so the user supplies their own;
		// the roleDefaults placeholders remain the internal fallback + commented suggestions only.
		for role := range models {
			models[role] = ""
		}
	}
	stagerName, stagerModel := StagerFallback(target, models)
	if piBlanked || opencodeBlanked {
		// stagerFallback re-pulls pi’s stager model from the FR-D4 table (a fresh copy); force
		// it blank so all four roles stay empty. pi remains the stager (stager-capable).
		stagerModel = ""
	}
	// Stager fell back to pi for a non-pi target whose manifest had empty tooled_flags. As of
	// 2026-07-09 all six built-ins are stager-capable, so this path is for user-defined providers;
	// pi is a multi-backend provider: a bare fallback model (gpt-5.4-mini) is a
	// hard FR-R5b error. Blank it so the user supplies their own backend/model. pi REMAINS the
	// stager (stager-capable) — only the MODEL is blanked. Placed before applyOverrides so an
	// explicit override can still set a model (mirrors the pi-target path's blank-then-override).
	if stagerName == "pi" && stagerName != target {
		stagerModel = ""
	}

	// Apply overrides AFTER pi-blank + stagerFallback (structural routing preserved; only MODEL values).
	piHasOverrides := piBlanked && len(overrides) > 0
	applyOverrides(models, &stagerModel, overrides)

	fmt.Fprintf(&b, "\n# --- per-role models for the default provider %q ---\n", target)
	fmt.Fprintf(&b, "# All commented — a role with no uncommented [role.*] inherits [defaults]. Uncomment a block\n")
	fmt.Fprintf(&b, "# to pin that role's model (pi/opencode ship blank; others ship the smallest/fastest default).\n")
	if opencodeBlanked {
		b.WriteString("# NOTE: opencode's models are plan-/backend-dependent (provider-prefixed, e.g. openai/gpt-5.4).\n")
		b.WriteString("# The shipped per-role models are EMPTY so you supply your own — opencode then uses whatever\n")
		b.WriteString("# model/backend your plan provides. opencode is a power-user provider.\n")
	} else if piBlanked && !piHasOverrides {
		b.WriteString("# NOTE: pi is a multi-backend provider — prefix the model with your inference backend,\n")
		b.WriteString("# e.g. model = \"anthropic/claude-haiku\". A bare model (no '/') on pi is a config error.\n")
		b.WriteString("# The shipped per-role models are empty so you can supply your own backend/model.\n")
	} else if piBlanked && piHasOverrides {
		b.WriteString("# NOTE: pi is a multi-backend provider — each model carries the inference backend as a\n")
		b.WriteString("# slash-prefix (e.g. model = \"anthropic/claude-haiku\"). A bare model (no '/') on pi is a config\n")
		b.WriteString("# error.\n")
	}
	// agy and cursor-agent bake the reasoning level into the MODEL NAME (agy's "(Low)/(Medium)/(High)"
	// suffix; cursor's "-none/-low/-medium" suffix). For them the `reasoning` setting is a NO-OP — the
	// only reasoning dial is the model name. Warn the user so they don't expect --reasoning to do anything.
	if target == "agy" || target == "cursor" {
		b.WriteString("# NOTE: " + target + " bakes the reasoning level into the MODEL NAME — agy's \"(Low)/(Medium)/(High)\"\n")
		b.WriteString("# suffix or cursor's \"-none/-low/-medium/-high\" suffix. The `reasoning` setting is a NO-OP for this\n")
		b.WriteString("# provider: pick the reasoning tier by choosing the model (e.g. gemini-3.6-flash-low).\n")
	}

	// Blank line separates the explanatory notes above from the solid commented role block below.
	b.WriteString("\n")

	// planner — inherits [defaults] provider (provider line omitted; uncomment to pin its model)
	writeCommentedRoleBlock(&b, "planner", "", models["planner"])

	// stager — may fall back to a different provider. Emit the provider line ONLY when the stager
	// routes somewhere other than [defaults] (a real fallback); otherwise the role inherits [defaults]
	// and a redundant provider line would just echo it. The fallback note prints as a comment before
	// the block so an uncommenting user sees why the stager is routed elsewhere.
	stagerProv := ""
	if stagerName != target {
		stagerProv = stagerName
		annotation := target + " cannot serve as the stager (no tooled_flags); routed to " + stagerName + " (the first stager-capable provider)."
		// When the stager fell back to pi and no override supplied a model, the bare fallback was
		// blanked — append the multi-backend guidance so the user knows to prefix their inference backend.
		if stagerName == "pi" && stagerModel == "" {
			annotation += " pi is a multi-backend provider — prefix the model with your inference backend, e.g. model = \"anthropic/claude-haiku\". A bare model (no '/') on pi is a config error."
		}
		fmt.Fprintf(&b, "# %s\n", annotation)
	}
	writeCommentedRoleBlock(&b, "stager", stagerProv, stagerModel)

	// message — inherits [defaults] provider
	writeCommentedRoleBlock(&b, "message", "", models["message"])

	// arbiter — inherits [defaults] provider
	writeCommentedRoleBlock(&b, "arbiter", "", models["arbiter"])

	// other installed providers as COMMENTED [role.*] groups
	for _, name := range preferredBuiltins {
		if name == target || !isInstalledName(name, installed) {
			continue
		}
		other := DefaultModelsForProvider(name)
		if other == nil {
			continue
		}
		piCommented := name == "pi"
		if piCommented {
			// pi is a multi-backend provider (FR-R5b): a bare model (no '/') is a hard error. Blank the
			// commented models so uncommenting yields a valid (model-less) config the user fills in —
			// mirroring the target=="pi" active-block blanking above. (other is a fresh per-call copy from
			// DefaultModelsForProvider, so this mutation is isolated to this map.)
			for role := range other {
				other[role] = ""
			}
		}
		b.WriteString("\n# === " + name + " (installed) — uncomment a [role.*] block to route that role to " + name + " ===\n")
		if piCommented {
			b.WriteString("# NOTE: pi is a multi-backend provider — prefix the model with your inference backend,\n")
			b.WriteString("# e.g. model = \"openai/gpt-5.4\". A bare model (no '/') on pi is a config error.\n")
		}
		if name == "agy" || name == "cursor" {
			b.WriteString("# NOTE: " + name + " bakes reasoning into the MODEL NAME (suffix); the `reasoning` setting is a no-op here — pick the tier via the model.\n")
		}
		writeCommentedRoleBlock(&b, "planner", name, other["planner"])
		writeCommentedRoleBlock(&b, "stager", name, other["stager"])
		writeCommentedRoleBlock(&b, "message", name, other["message"])
		writeCommentedRoleBlock(&b, "arbiter", name, other["arbiter"])
	}

	// shared [generation] defaults (GenerationSection — the SAME block the inert --template reference
	// emits, so the documented key set never drifts), with token_limit UNCOMMENTED so the populated
	// config ships an active 50000-token budget. Generated-file affordance only: Defaults() itself
	// stays 0 (no config ⇒ no holistic cap), so a user who deletes the line or sets 0 gets no limit.
	// The inert reference keeps token_limit commented (it stays functionally inert).
	b.WriteString(strings.Replace(GenerationSection, "\n# token_limit", "\ntoken_limit", 1))

	return b.String()
}

// bootstrapHeader is the populated config-file header (precedence, env vars, git keys, CLI flags).
// Used by buildBootstrapConfig. The inert exampleConfigTemplate (cmd/config.go) carries its own
// header prose, but BOTH share the [generation] documentation via GenerationSection. The two
// scope-sensitive sentences below ("THIS global file" + "This is the GLOBAL file") are rewritten to
// repo-local framing by RewriteHeaderForLocalScope for `config init --local`.
const bootstrapHeader = `# Stagecoach configuration file (populated bootstrap).
#
# Generated by ` + "`stagecoach config init`" + `. This file contains a WORKING config with
# a detected (or --provider-pinned) agent and per-role model defaults UNCOMMENTED.
# Edit freely; uncomment any commented section to activate it.
#
# Resolution precedence (highest -> lowest):
#   CLI flags  >  STAGECOACH_* env vars  >  repo git config (stagecoach.*)  >
#   repo-local .stagecoach.toml  >  THIS global file  >  provider defaults  >  built-in defaults
#
# This is the GLOBAL file. A repo-local file (./.stagecoach.toml) and repo git config (stagecoach.*)
# both override it; CLI flags and env vars override those.
#
# Environment variables — override this file, are overridden by CLI flags:
#   STAGECOACH_PROVIDER   default provider/agent (e.g. "pi", "claude", "agy")
#   STAGECOACH_MODEL      model override ("" -> provider manifest default_model)
#   STAGECOACH_TIMEOUT    generation timeout, e.g. "120s" or 120 (seconds)
#   STAGECOACH_CONFIG     path to a config file, overrides discovery
#   STAGECOACH_VERBOSE    "true"/"false" — print resolved command, raw output, retries
#   STAGECOACH_NO_COLOR   "true"/"false" — disable color (also honors NO_COLOR)
#   STAGECOACH_NO_PARENT_WATCHDOG=1   # opt out of the parent-death lock watchdog
#   STAGECOACH_PLANNER_PROVIDER / _MODEL   per-role override: decomposition planner
#   STAGECOACH_STAGER_PROVIDER  / _MODEL   per-role override: (tooled) staging agent
#   STAGECOACH_MESSAGE_PROVIDER / _MODEL   per-role override: bare commit-message agent
#   STAGECOACH_ARBITER_PROVIDER / _MODEL   per-role override: leftover arbiter
#   STAGECOACH_REASONING                  global reasoning effort: off|low|medium|high
#   STAGECOACH_<ROLE>_REASONING           per-role reasoning override (role = planner|stager|message|arbiter)
#   STAGECOACH_COMMITS                    force exactly N commits when nothing is staged; 1 == --single
#
# Git config keys — alternative to this file, scoped to one repo:
#   git config stagecoach.provider pi
#   git config stagecoach.model ""
#   git config stagecoach.timeout 120s
#   git config stagecoach.autoStageAll true
#   (read via ` + "`git config --get stagecoach.<key>`" + `)
#
# ---------------------------------------------------------------------------
# CLI flags — highest precedence; only an EXPLICITLY-passed flag overrides lower layers
# ---------------------------------------------------------------------------
# --provider / --model                       global default for ALL roles
# --<role>-provider / --<role>-model         per-role override (role = planner|stager|message|arbiter)
# --commits <N>                              force exactly N commits (N>=2); --commits 1 == --single
# --single / --no-decompose                  bypass decomposition; force the single-commit path
# --max-commits <N>                          safety cap on auto-decompose (default 12)

`

// GenerationSection is the shared, commented [generation] documentation block used by BOTH the
// populated bootstrap config (config init) and the inert reference (config init --template). It is
// the single source of truth for which [generation] keys are documented — closing the drift where the
// two templates each documented a different subset (the populated one lacked format/template/locale/
// push/exclude; the inert one lacked token_limit/diff_context/multi_turn_*/no_parent_watchdog).
//
// The [generation] header is intentionally UNCOMMENTED while every key stays commented: in TOML a key
// attaches to the most-recent ACTIVE table header, so a commented header would let a user uncomment a
// single key and have it silently attach to the WRONG table (e.g. the preceding [role.*] block).
// Keeping the header active means uncommenting any one key lands it in [generation]. The section is
// still functionally INERT (IsInert == true): an active table header with no uncommented keys holds
// zero active settings, so `config upgrade` on it is a no-op and the load-time migration notice fires
// only when a real (keyed) setting predates the current schema.
const GenerationSection = `
# ---------------------------------------------------------------------------
# [generation] — diff capture & output tuning
# ---------------------------------------------------------------------------
# The [generation] header below is intentionally UNCOMMENTED while every key stays commented
#: uncommenting any one key lands it in the right table,
# and an all-commented-keys section is an inert empty table — built-in defaults still apply.
[generation]
# max_diff_bytes          = 300000   # byte cap on the non-markdown diff section; ignored when token_limit is set
# max_md_lines            = 100      # per-file line cap for markdown diffs; ignored when token_limit is set
# token_limit             = 50000    # holistic token budget for the WHOLE payload (prompt+examples+diff); the populated config ships 50000 active — set 0 (or delete the line) for no holistic cap (legacy per-section caps above)
# diff_context            = 1        # unchanged context lines around each hunk: 0 = changed lines only, 1 = one anchor line (default), 3 = git's default; valid 0–3
# max_duplicate_retries   = 3        # re-generation attempts when the subject duplicates a recent commit
# subject_target_chars    = 50       # target subject-line length for truncation
# output                  = "raw"    # agent output mode: raw | json — applies to parsing across ALL providers
# strip_code_fence        = true     # strip ` + "`" + ` fences from agent output (all providers)
# max_commits             = 12       # safety cap on auto-decompose; default 12
# binary_extensions       = []       # extra non-text extensions to filter beyond the built-in denylist
# exclude                 = []       # gitignore-style globs; UNION across global+repo+flag
# multi_turn_fallback     = true     # lossless multi-turn fallback on one-shot exhaustion; set false to DISABLE
# multi_turn_chunk_tokens = 32000    # per-turn chunk budget in tokens for multi-turn; does NOT interact with token_limit
# no_parent_watchdog      = false    # opt out of the parent-death lock watchdog — set true if you launch via nohup/setsid/systemd-run
# format                  = "auto"   # <base>[+body]: auto|conventional|gitmoji|plain, each optionally +body; unknown = hard error (exit 1)
# locale                  = ""       # free-form language name or BCP-47 tag; never validated
# template                = ""       # wrap every message; must contain literal $msg, e.g. "$msg (#205)"
# push                    = false    # run ` + "`" + `git push` + "`" + ` after a fully-successful run; on failure commits stand
# NOTE: [generation] output/strip_code_fence override any per-provider [provider.<name>] values.
`

// RewriteHeaderForLocalScope rewrites the scope-specific header sentences in a generated config
// template (populated OR inert) from GLOBAL framing to REPO-LOCAL framing, for `config init --local`.
// It touches only the two scope-sensitive passages (the precedence line and the "This is the GLOBAL
// file" note); every other line is preserved verbatim. Both the populated header (bootstrapHeader)
// and the inert header (exampleConfigTemplate) share the identical source sentences, so one transform
// serves both. Idempotent: the source substrings are absent from already-local content, so reapplying
// it is a no-op. PURE (no I/O) — unit-tested directly.
func RewriteHeaderForLocalScope(content string) string {
	// (1) Precedence line: in a global file the chain reads "... repo-local .stagecoach.toml > THIS
	// global file > ...". In the repo-local file THIS file IS the repo-local layer, so the two collapse
	// to "THIS file (.stagecoach.toml) > the global config file".
	content = strings.Replace(content,
		"repo-local .stagecoach.toml  >  THIS global file",
		"THIS file (.stagecoach.toml)  >  the global config file", 1)
	// (2) The scope-note paragraph (two comment lines).
	content = strings.Replace(content,
		"# This is the GLOBAL file. A repo-local file (./.stagecoach.toml) and repo git config (stagecoach.*)\n# both override it; CLI flags and env vars override those.",
		"# This is the REPO-LOCAL config (./.stagecoach.toml). It overrides the global config file;\n# repo git config (stagecoach.*), STAGECOACH_* env vars, and CLI flags override it.", 1)
	return content
}
