// Package cmd implements the config command group for Stagehand (PRD §9.8 FR38, §15.3, §16.2).
// It provides a `config` cobra command with two leaf subcommands: `init` (bootstrap a populated
// working config to the global config path, creating parent dirs, refusing to overwrite unless
// --force) and `path` (print the resolved global config path to stdout). Both are thin views over
// the P1.M1.T4.S2 globalConfigPath resolver (newly exported as config.GlobalConfigPath()).
//
// Both leaves are in shouldSkipConfigLoad (cmd.Name()=="init"/"path"), so root's PersistentPreRunE
// returns nil immediately — they work OUTSIDE a git repo and never need config.Load.
//
// Registered via init() in this file — ZERO edits to root.go (parallel-safe with S2/S3, design D2).
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/exitcode"
	"github.com/dustin/stagehand/internal/provider"
)

// preferredBuiltins is the FR-D1 cascading provider priority order (local copy — mirrors
// internal/provider/registry.go's unexported preferredBuiltins). Used by resolveBootstrapTarget
// for the --provider validation error message, and by stagerFallback / buildBootstrapConfig for
// scanning the first stager-capable provider and ordering commented alternate-provider blocks.
var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "gemini", "codex", "claude"}

// configCmd is the PRD §15.3 "config" command group. It has NO RunE → bare `stagehand config` prints
// help (cobra default). init/path are its leaves (registered in init()). Both leaves are in
// shouldSkipConfigLoad (cmd.Name()=="init"/"path") so root's PersistentPreRunE returns nil immediately
// — they work OUTSIDE a git repo and never need config.Load.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage the Stagehand config file",
	Long: `Inspect or bootstrap the Stagehand global config file.

Subcommands:
  init   Bootstrap a working config (auto-detects your agent).
  path   Print the resolved global config path.`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Bootstrap a working config (auto-detects your agent)",
	Long: `Bootstrap a populated, working config to Stagehand's global config path.

By DEFAULT, detects the highest-priority installed built-in agent (order: pi, opencode,
cursor, agy, gemini, codex, claude) and writes a config with that provider's per-role
default models UNCOMMENTED so the tool works immediately. If no agent is detected, defaults
to "pi". Other installed providers appear as commented-out [role.*] blocks (one-line
uncomment to route a role to a different agent).

Flags:
  --provider <name>  Target a specific built-in provider instead of auto-detecting.
  --force            Overwrite an existing config file.
  --template         Write the inert all-commented reference config (v1 behavior).

Parent directories are created as needed. If a config file already exists, it is NOT
overwritten unless --force is passed (exit code 1).

See ` + "`stagehand config path`" + ` for the target location.`,
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runConfigInit,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the resolved global config path",
	Long: `Print the resolved global config path (the file ` + "`config init`" + ` writes and Stagehand
reads as its global config layer).

This is the DISCOVERED global location ($XDG_CONFIG_HOME/stagehand/config.toml, or
~/.config/stagehand/config.toml by default) — not a --config/STAGEHAND_CONFIG override, which selects
a separate read path.`,
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE:          runConfigPath,
}

func init() {
	configInitCmd.Flags().String("provider", "", "Target a specific provider instead of auto-detecting")
	configInitCmd.Flags().Bool("force", false, "Overwrite an existing config file")
	configInitCmd.Flags().Bool("template", false, "Write the inert all-commented reference config (v1 behavior)")

	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configPathCmd)
	rootCmd.AddCommand(configCmd) // register on S1's root — NO edit to root.go (design D2)
}

// runConfigPath implements `stagehand config path` (FR38). Prints the resolved global config path to
// stdout (one line). Returns nil. Never calls os.Exit. Works outside a git repo (config load skipped).
func runConfigPath(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(cmd.OutOrStdout(), config.GlobalConfigPath())
	return nil
}

// runConfigInit implements `stagehand config init` (PRD §9.17 FR-B1/B2). Bootstraps a populated
// working config by default (auto-detects provider + per-role models from the FR-D4 table), or writes
// the inert exampleConfigTemplate when --template is passed. Refuses to overwrite unless --force.
// Parent dirs are created; the written path is always printed. Never calls os.Exit.
func runConfigInit(cmd *cobra.Command, args []string) error {
	path := config.GlobalConfigPath()

	force, _ := cmd.Flags().GetBool("force")
	if !force {
		if _, err := os.Stat(path); err == nil {
			return exitcode.New(exitcode.Error, fmt.Errorf("config file already exists at %s (not overwritten)", path))
		} else if !os.IsNotExist(err) {
			return exitcode.New(exitcode.Error, fmt.Errorf("check config path %s: %w", path, err))
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("create config dir %s: %w", filepath.Dir(path), err))
	}

	tmpl, _ := cmd.Flags().GetBool("template")
	var content string
	if tmpl {
		content = exampleConfigTemplate
	} else {
		providerName, _ := cmd.Flags().GetString("provider")
		reg := provider.NewRegistry(nil) // built-ins only (config load is skipped for init — F1)
		installed := configInitInstalledNames(reg)
		target, err := resolveBootstrapTarget(reg, providerName, installed)
		if err != nil {
			return exitcode.New(exitcode.Error, err)
		}
		content = buildBootstrapConfig(target, installed)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("write config %s: %w", path, err))
	}

	if tmpl {
		fmt.Fprintf(cmd.OutOrStdout(), "Wrote example config to %s\n", path)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Wrote config to %s\n", path)
	}
	return nil
}

// resolveBootstrapTarget resolves the bootstrap provider: --provider (validated) > cascade > "pi" fallback.
func resolveBootstrapTarget(reg *provider.Registry, providerName string, installed []string) (string, error) {
	if providerName != "" {
		if _, ok := reg.Get(providerName); !ok {
			return "", fmt.Errorf("unknown provider %q (use a built-in: %s)", providerName, strings.Join(preferredBuiltins, ", "))
		}
		return providerName, nil
	}
	if det := reg.DefaultProvider(installed); det != "" {
		return det, nil
	}
	return "pi", nil // nothing installed — valid default; annotated in buildBootstrapConfig
}

// configInitInstalledNames returns the Names of providers whose command is on $PATH (mirrors providers.go's
// unexported helper — local copy keeps the change self-contained). reg.List() is sorted ascending.
func configInitInstalledNames(reg *provider.Registry) []string {
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
func stagerFallback(target string, models map[string]string) (string, string) {
	if m := models["stager"]; m != "" {
		return target, m
	}
	for _, name := range preferredBuiltins {
		if col := config.DefaultModelsForProvider(name); col != nil && col["stager"] != "" {
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

// writeRoleBlock writes an UNCOMMENTED [role.<r>] block. provider is omitted when "" (role inherits
// [defaults]); annotation is printed as a comment before the key=value lines when non-empty.
func writeRoleBlock(b *strings.Builder, role, prov, model, annotation string) {
	fmt.Fprintf(b, "\n[role.%s]\n", role)
	if annotation != "" {
		fmt.Fprintf(b, "# %s\n", annotation)
	}
	if prov != "" {
		fmt.Fprintf(b, "provider = %q\n", prov)
	}
	fmt.Fprintf(b, "model = %q\n", model)
}

// writeCommentedRoleBlock writes a fully-commented [role.<r>] block for an alternate provider.
func writeCommentedRoleBlock(b *strings.Builder, role, prov, model string) {
	fmt.Fprintf(b, "# [role.%s]\n", role)
	fmt.Fprintf(b, "# provider = %q\n", prov)
	fmt.Fprintf(b, "# model = %q\n", model)
}

// buildBootstrapConfig is the PURE populated-config generator (PRD §9.17 FR-B1). NO detection, NO I/O —
// takes an already-resolved target + the installed list, returns the exact TOML. Deterministic ⇒ unit-
// testable. Writes: header docs, config_version (uncommented), [defaults] provider=<target> (uncommented),
// four [role.*] blocks for target (models from DefaultModelsForProvider; stager routed to the fallback
// when target can't stage, annotated), each OTHER installed provider as a commented [role.*] group, then a
// commented [generation] section.
func buildBootstrapConfig(target string, installed []string) string {
	var b strings.Builder

	// --- header (precedence/env/git/cli docs — shared with the inert template) ---
	b.WriteString(bootstrapHeader)

	// config_version (UNCOMMENTED — F6)
	fmt.Fprintf(&b, "config_version = %d\n", config.CurrentConfigVersion)

	// [defaults] — provider uncommented, rest commented
	b.WriteString("\n# [defaults] — top-level Stagehand behavior (PRD §16.2)\n")
	b.WriteString("[defaults]\n")
	fmt.Fprintf(&b, "provider = %q", target)
	if !isInstalledName(target, installed) {
		b.WriteString("  # no built-in agent detected on $PATH; defaulted to \"pi\" — edit if you use a different agent")
	}
	b.WriteString("\n")
	b.WriteString("# model          = \"\"\n# timeout        = \"120s\"\n# auto_stage_all = true\n# verbose        = false\n")

	// [role.*] for the target (UNCOMMENTED), canonical order: planner, stager, message, arbiter
	models := config.DefaultModelsForProvider(target) // non-nil (target is a validated built-in)
	stagerName, stagerModel := stagerFallback(target, models)

	fmt.Fprintf(&b, "\n# --- per-role models for the default provider %q (PRD §16.4, §9.15) ---\n", target)

	// planner — inherits [defaults] provider
	writeRoleBlock(&b, "planner", "", models["planner"], "")

	// stager — may fall back to a different provider
	var stagerAnnotation string
	if stagerName != target {
		stagerAnnotation = target + " cannot serve as the stager (no tooled_flags); routed to " + stagerName + " (the first stager-capable provider)."
	}
	writeRoleBlock(&b, "stager", stagerName, stagerModel, stagerAnnotation)

	// message — inherits [defaults] provider
	writeRoleBlock(&b, "message", "", models["message"], "")

	// arbiter — inherits [defaults] provider
	writeRoleBlock(&b, "arbiter", "", models["arbiter"], "")

	// other installed providers as COMMENTED [role.*] groups
	for _, name := range preferredBuiltins {
		if name == target || !isInstalledName(name, installed) {
			continue
		}
		other := config.DefaultModelsForProvider(name)
		if other == nil {
			continue
		}
		b.WriteString("\n# === " + name + " (installed) — uncomment a [role.*] block to route that role to " + name + " ===\n")
		writeCommentedRoleBlock(&b, "planner", name, other["planner"])
		writeCommentedRoleBlock(&b, "stager", name, other["stager"])
		writeCommentedRoleBlock(&b, "message", name, other["message"])
		writeCommentedRoleBlock(&b, "arbiter", name, other["arbiter"])
	}

	// commented [generation] defaults
	b.WriteString(generationCommented)

	return b.String()
}

// bootstrapHeader is the shared config-file header (precedence, env vars, git keys, CLI flags).
// Used by buildBootstrapConfig for the populated output. exampleConfigTemplate has its own copy.
const bootstrapHeader = `# Stagehand configuration file (populated bootstrap).
#
# Generated by ` + "`stagehand config init`" + `. This file contains a WORKING config with
# a detected (or --provider-pinned) agent and per-role model defaults UNCOMMENTED.
# Edit freely; uncomment any commented section to activate it.
#
# Resolution precedence (highest -> lowest), PRD §9.8 FR34 / §16.1:
#   CLI flags  >  STAGEHAND_* env vars  >  repo git config (stagehand.*)  >
#   repo-local .stagehand.toml  >  THIS global file  >  provider defaults  >  built-in defaults
#
# This is the GLOBAL file. A repo-local file (./.stagehand.toml) and repo git config (stagehand.*)
# both override it; CLI flags and env vars override those.
#
# Environment variables (PRD §9.8 FR35) — override this file, are overridden by CLI flags:
#   STAGEHAND_PROVIDER   default provider/agent (e.g. "pi", "claude", "gemini")
#   STAGEHAND_MODEL      model override ("" -> provider manifest default_model)
#   STAGEHAND_TIMEOUT    generation timeout, e.g. "120s" or 120 (seconds)
#   STAGEHAND_CONFIG     path to a config file, overrides discovery
#   STAGEHAND_VERBOSE    "true"/"false" — print resolved command, raw output, retries
#   STAGEHAND_NO_COLOR   "true"/"false" — disable color (also honors NO_COLOR)
#   STAGEHAND_PLANNER_PROVIDER / _MODEL   per-role override: decomposition planner (PRD §16.4, §9.15)
#   STAGEHAND_STAGER_PROVIDER  / _MODEL   per-role override: (tooled) staging agent
#   STAGEHAND_MESSAGE_PROVIDER / _MODEL   per-role override: bare commit-message agent
#   STAGEHAND_ARBITER_PROVIDER / _MODEL   per-role override: leftover arbiter
#   STAGEHAND_COMMITS                    force exactly N commits when nothing is staged (PRD §9.14); 1 == --single
#
# Git config keys (PRD §9.8 FR36 / §16.3) — alternative to this file, scoped to one repo:
#   git config stagehand.provider pi
#   git config stagehand.model ""
#   git config stagehand.timeout 120s
#   git config stagehand.auto_stage_all true
#   (read via ` + "`git config --get stagehand.<key>`" + `)
#
# ---------------------------------------------------------------------------
# CLI flags (PRD §15.2) — highest precedence; only an EXPLICITLY-passed flag overrides lower layers
# ---------------------------------------------------------------------------
# --provider / --model                       global default for ALL roles (§16.4)
# --<role>-provider / --<role>-model         per-role override (role = planner|stager|message|arbiter)
# --commits <N>                              force exactly N commits (N>=2); --commits 1 == --single (§9.14)
# --single / --no-decompose                  bypass decomposition; force the single-commit path (§9.14)
# --max-commits <N>                          safety cap on auto-decompose (default 12; §9.14 FR-M4)

`

// generationCommented is the commented [generation] defaults section appended to the populated config.
const generationCommented = `
# ---------------------------------------------------------------------------
# [generation] — diff capture & output tuning (PRD §16.2)
# ---------------------------------------------------------------------------
# [generation]
# max_diff_bytes        = 300000  # byte cap on the non-markdown diff section
# max_md_lines          = 100     # per-file line cap for markdown diffs
# max_duplicate_retries = 3       # re-generation attempts when the subject duplicates a recent commit
# subject_target_chars  = 50      # target subject-line length for truncation
# output                = "raw"   # agent output mode: "raw" | "json" — applies to parsing across ALL providers
# strip_code_fence      = true    # strip ` + "`" + ` fences from agent output (all providers)
# max_commits           = 12      # safety cap on auto-decompose (PRD §9.14 FR-M4); default 12
# binary_extensions     = []      # extra non-text extensions to filter beyond the built-in denylist (§9.1 FR3a)
`

// exampleConfigTemplate is the commented example config written by `config init --template` (PRD §16.2 / FR38).
// EVERY option line is commented out (#), so the file is INERT until the user uncomments it. This
// template IS the Mode-A user-facing config documentation: the header explains the §9.8 precedence
// order, STAGEHAND_* env vars, and `stagehand.*` git-config keys; the [defaults]/[generation]/
// [provider.X] sections mirror §16.2 with documented default values and (for providers) field names
// that match internal/provider/manifest.go toml tags.
const exampleConfigTemplate = `# Stagehand configuration file (PRD §16.2).
#
# Generated by ` + "`stagehand config init`" + `. Every option below is COMMENTED OUT (#), so this file
# is inert — it documents the available options without changing any defaults. To use an option,
# copy its line to a new (uncommented) line and adjust the value.
#
# Resolution precedence (highest -> lowest), PRD §9.8 FR34 / §16.1:
#   CLI flags  >  STAGEHAND_* env vars  >  repo git config (stagehand.*)  >
#   repo-local .stagehand.toml  >  THIS global file  >  provider defaults  >  built-in defaults
#
# This is the GLOBAL file. A repo-local file (./.stagehand.toml) and repo git config (stagehand.*)
# both override it; CLI flags and env vars override those.
#
# Environment variables (PRD §9.8 FR35) — override this file, are overridden by CLI flags:
#   STAGEHAND_PROVIDER   default provider/agent (e.g. "pi", "claude", "gemini")
#   STAGEHAND_MODEL      model override ("" -> provider manifest default_model)
#   STAGEHAND_TIMEOUT    generation timeout, e.g. "120s" or 120 (seconds)
#   STAGEHAND_CONFIG     path to a config file, overrides discovery
#   STAGEHAND_VERBOSE    "true"/"false" — print resolved command, raw output, retries
#   STAGEHAND_NO_COLOR   "true"/"false" — disable color (also honors NO_COLOR)
#   STAGEHAND_PLANNER_PROVIDER / _MODEL   per-role override: decomposition planner (PRD §16.4, §9.15)
#   STAGEHAND_STAGER_PROVIDER  / _MODEL   per-role override: (tooled) staging agent
#   STAGEHAND_MESSAGE_PROVIDER / _MODEL   per-role override: bare commit-message agent
#   STAGEHAND_ARBITER_PROVIDER / _MODEL   per-role override: leftover arbiter
#   STAGEHAND_COMMITS                    force exactly N commits when nothing is staged (PRD §9.14); 1 == --single
#
# ---------------------------------------------------------------------------
# config_version — schema version (PRD §9.17 FR-B4). Top-level metadata, NOT a [defaults] key and
# NOT a precedence layer (§16.1): it never overrides another field; it only tells stagehand which
# schema the file was written for. This binary supports config_version = 2.
# ---------------------------------------------------------------------------
# config_version = 2
#
# On load, if this is missing/older than the binary's version, stagehand prints an advisory and
# points you at the remediation; it NEVER auto-migrates your file (no behavior change, just a
# warning on stderr):
#   stagehand config upgrade      # rewrite this file in place to the current schema (P1.M4.T3)
#   stagehand config init --force # regenerate the bootstrap config, overwriting this file

# Git config keys (PRD §9.8 FR36 / §16.3) — alternative to this file, scoped to one repo:
#   git config stagehand.provider pi
#   git config stagehand.model ""
#   git config stagehand.timeout 120s
#   git config stagehand.auto_stage_all true
#   (read via ` + "`git config --get stagehand.<key>`" + `)

# ---------------------------------------------------------------------------
# CLI flags (PRD §15.2) — highest precedence; only an EXPLICITLY-passed flag overrides lower layers
# ---------------------------------------------------------------------------
# --provider / --model                       global default for ALL roles (§16.4)
# --<role>-provider / --<role>-model         per-role override (role = planner|stager|message|arbiter)
# --commits <N>                              force exactly N commits (N>=2); --commits 1 == --single (§9.14)
# --single / --no-decompose                  bypass decomposition; force the single-commit path (§9.14)
# --max-commits <N>                          safety cap on auto-decompose (default 12; §9.14 FR-M4)

# ---------------------------------------------------------------------------
# [defaults] — top-level Stagehand behavior (PRD §16.2)
# ---------------------------------------------------------------------------
# [defaults]
# provider       = "pi"     # default agent; "" -> auto-detect (first installed built-in)
# model          = ""       # "" -> use the provider manifest's default_model
# timeout        = "120s"   # generation timeout (Go duration string, e.g. "2m", or bare seconds)
# auto_stage_all = true     # run ` + "`git add -A`" + ` when nothing is staged
# verbose        = false    # print the resolved command, raw agent output, and retries

# ---------------------------------------------------------------------------
# [generation] — diff capture & output tuning (PRD §16.2)
# ---------------------------------------------------------------------------
# [generation]
# max_diff_bytes        = 300000  # byte cap on the non-markdown diff section
# max_md_lines          = 100     # per-file line cap for markdown diffs
# max_duplicate_retries = 3       # re-generation attempts when the subject duplicates a recent commit
# subject_target_chars  = 50      # target subject-line length for truncation
# output                = "raw"   # agent output mode: "raw" | "json" — applies to parsing across ALL providers
# strip_code_fence      = true    # strip ` + "`" + ` fences from agent output (all providers)
# max_commits           = 12      # safety cap on auto-decompose (PRD §9.14 FR-M4); default 12
# binary_extensions     = []      # extra non-text extensions to filter beyond the built-in denylist (§9.1 FR3a)
# NOTE: [generation] output/strip_code_fence override any per-provider [provider.<name>] values.

# ---------------------------------------------------------------------------
# [provider.<name>] — override a built-in or define a new provider (PRD §16.2, §12.8)
# ---------------------------------------------------------------------------
# A [provider.<name>] section FIELD-MERGES onto a built-in of the same name. A brand-new <name>
# adds a new provider. Use ` + "`stagehand providers show <name>`" + ` to inspect the merged result.
#
# Override a built-in (e.g. pin pi to a different model/provider):
# [provider.pi]
# default_model    = "glm-5.2"
# default_provider = "zai"
#
# Define a brand-new provider (PRD §12.8):
# [provider.myagent]
# command            = "/opt/myagent/bin/agent"
# prompt_delivery    = "stdin"          # stdin | positional | flag
# print_flag         = "--once"
# model_flag         = "--model"
# default_model      = "my-model-7b"
# system_prompt_flag = "--system"
# default_provider   = "zai"
# bare_flags         = ["--no-mcp", "--ephemeral"]
# output             = "raw"            # raw | json

# ---------------------------------------------------------------------------
# [role.<role>] — per-role provider/model overrides (PRD §16.4, §9.15 FR-R1–R5)
# ---------------------------------------------------------------------------
# The four agent roles — planner, stager, message, arbiter — each resolve their provider/model
# independently. A single [defaults] (above) covers ALL roles; a [role.*] table overrides it for the
# roles you care about. Both fields "" -> inherit [defaults]. Precedence (highest wins):
#   flag > STAGEHAND_<ROLE>_* env > [role.*] config > [defaults] > provider manifest default.
#
# [role.planner]
# provider = "agy"
# model    = "gemini-2.5-pro"
#
# [role.stager]            # tooled agent that runs git; needs tooled_flags in its provider manifest
# provider = "agy"
# model    = "gemini-2.5-flash"
#
# [role.message]           # bare commit-message agent — inherits [defaults] (omit to inherit)
# provider = ""            # "" -> inherit [defaults].provider
# model    = ""            # "" -> inherit [defaults].model
#
# [role.arbiter]           # bare leftover arbiter — inherits [defaults]
# provider = ""
# model    = ""
`
