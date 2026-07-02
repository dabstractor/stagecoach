// example.go holds the fully-commented PRD §16.2 config-file template — the
// canonical config-file reference written by `stagehand config init`
// (P1.M7.T3.S2, FR38). It is the Mode A surface: every key is documented
// inline, so the generated file IS the per-key reference and can never drift
// from the code that reads it.
//
// This file is a sibling of config.go (which OWNS the "// Package config"
// doc) and therefore carries a plain "package config" line, mirroring how
// defaults.go/file.go defer the package doc to config.go. It imports nothing
// beyond the standard library: ExampleConfig returns a raw-string-literal
// constant, so the template string is the single source of truth consumed by
// `stagehand config init` and asserted by the in-package (white-box) test.
package config

// ExampleConfig returns the fully-commented PRD §16.2 config-file template —
// the canonical config-file reference written to the global path by
// `stagehand config init` (PRD §16.2, FR38). Every line is a TOML comment, so
// the written file is a documented NO-OP: the built-in defaults
// (internal/config/defaults.go) stay in effect until a user deletes the
// leading "# " on a line they want to change.
//
// Comment convention in the returned string (mirrors the test that asserts the
// file "parses back to defaults"):
//
//   - `# key = value ...` lines are UNCOMMENT-TO-ACTIVATE settings. Their
//     scalar values are the TRUE Default* constants (provider=""/model=""
//     documented as empty = auto-resolve; timeout="120s"; auto_stage_all=true;
//     max_diff_bytes=300000; max_md_lines=100; max_duplicate_retries=3;
//     subject_target_chars=50; output="raw"; strip_code_fence=true). Stripping
//     the leading "# " and toml.Unmarshal-ing the result yields [defaults]/
//     [generation] values equal to config.Default().
//   - `## ...` lines are permanent documentation (notes, section headers).
//     After the test strips one leading "#" they remain "#" comments and are
//     ignored by go-toml, so the whole template parses as valid TOML.
//
// The [provider.pi] / [provider.myagent] tables are OPTIONAL examples (PRD
// §12.8): they are NOT defaults, and are present only to show how a user
// overrides a built-in provider or defines a new one. Keeping them commented
// means they do not affect a freshly initialized config.
func ExampleConfig() string {
	return `## stagehand global configuration file.
## Resolved path: 'stagehand config path'
##   XDG_CONFIG_HOME/stagehand/config.toml  (default ~/.config/stagehand/config.toml)
##
## Every setting below is commented out, so this file is a documented NO-OP:
## the built-in defaults (internal/config/defaults.go) are in effect until you
## uncomment a line. To change a setting, delete the leading "# " on that line.
## Lines beginning with "## " are notes and should stay commented.

# [defaults]
# provider = ""            # empty = auto-resolve the first detected agent (e.g. "pi")
# model = ""               # empty = use the provider manifest's default_model
# timeout = "120s"         # per-agent-invocation timeout (Go duration: "90s", "2m")
# auto_stage_all = true    # run 'git add -A' before generating when nothing is staged
# verbose = false          # print the resolved command, raw output, and retries

# [generation]
# max_diff_bytes = 300000        # total staged-diff byte cap
# max_md_lines = 100             # per-markdown-file line cap within the diff
# max_duplicate_retries = 3      # outer duplicate-rejection budget (0..N inclusive)
# output = "raw"                 # raw | json  (how the agent's stdout is read)
# strip_code_fence = true        # strip one layer of ` + "```" + `/~~~ fencing from agent stdout
# subject_target_chars = 50      # target subject-line character count (FR13/FR14)

## The [provider.<name>] tables below are OPTIONAL. They override a built-in
## provider (field-merged onto the built-in manifest) or define a brand-new
## provider (PRD §12.8). They are NOT defaults — leave them commented unless you
## need them.

## Override the built-in 'pi' provider (field-merged with the built-in manifest).
# [provider.pi]
# default_model = "glm-5.2"
# default_provider = "zai"

## Define a brand-new provider (PRD §12.8). Used via 'stagehand --provider myagent'.
# [provider.myagent]
# command = "/opt/myagent/bin/agent"
# prompt_delivery = "stdin"
# print_flag = "--once"
# model_flag = "--model"
# default_model = "my-model-7b"
# system_prompt_flag = "--system"
# bare_flags = ["--no-mcp", "--ephemeral"]
# output = "raw"
`
}
