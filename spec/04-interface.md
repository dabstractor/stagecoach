## 15. CLI reference

### 15.1 Synopsis

```
stagecoach [flags]
stagecoach <command> [flags]
```

With no command, runs the default action: commit staged changes (auto-staging all if nothing is staged and `auto_stage_all` is on).

### 15.2 Global flags

| Flag                                             | Env                                      | Git config                  | Default                      | Description                                                                                                                                                                   |
| ------------------------------------------------ | ---------------------------------------- | --------------------------- | ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `--provider <name>`                              | `stagecoach_PROVIDER`                    | `stagecoach.provider`       | auto-detected                | Provider (agent platform) to use — **global default for all roles** (§16.4).                                                                                                  |
| `--model <name>`                                 | `stagecoach_MODEL`                       | `stagecoach.model`          | per-manifest `default_model` | Model override — `inference/model` form for multi-backend providers (FR-R5b) — **global default for all roles** (§16.4).                                                      |
| `--reasoning <level>`                            | `stagecoach_REASONING`                   | `stagecoach.reasoning`      | off                          | Reasoning level (`off\|low\|medium\|high`) — **global default for all roles** (FR-R6); `off` for every role out of the box.                                                   |
| `--config <path>`                                | `stagecoach_CONFIG`                      | —                           | resolved path                | Path to a config file (overrides discovery).                                                                                                                                  |
| `--timeout <dur>`                                | `stagecoach_TIMEOUT`                     | `stagecoach.timeout`        | `120s`                       | Generation timeout.                                                                                                                                                           |
| `--all`, `-a`                                    | —                                        | —                           | —                            | `git add -A` before snapshotting, even if something is staged.                                                                                                                |
| `--no-auto-stage`                                | —                                        | —                           | —                            | If nothing is staged, exit instead of auto-staging.                                                                                                                           |
| `--commits <N>`                                  | —                                        | `stagecoach.commits`        | `0` (auto)                   | Force exactly N commits when nothing is staged (skips the planner's count decision; §13.6.1). `1` ≡ `--single`.                                                               |
| `--single`, `--no-decompose`                     | —                                        | —                           | —                            | Bypass decomposition; force the v1 single-commit auto-stage-all behavior.                                                                                                     |
| `--max-commits <N>`                              | —                                        | `stagecoach.max_commits`    | `12`                         | Safety cap on auto-decompose commit count.                                                                                                                                    |
| `--planner-provider <p>` / `--planner-model <m>` | `stagecoach_PLANNER_PROVIDER` / `_MODEL` | `stagecoach.role.planner.*` | global                       | Per-role override for the decomposition planner (§16.4).                                                                                                                      |
| `--stager-provider <p>` / `--stager-model <m>`   | `stagecoach_STAGER_PROVIDER` / `_MODEL`  | `stagecoach.role.stager.*`  | global                       | Per-role override for the (tooled) staging agent.                                                                                                                             |
| `--arbiter-provider <p>` / `--arbiter-model <m>` | `stagecoach_ARBITER_PROVIDER` / `_MODEL` | `stagecoach.role.arbiter.*` | global                       | Per-role override for the leftover arbiter.                                                                                                                                   |
| `--exclude <glob>`, `-x` (repeatable)            | —                                        | —                           | —                            | Exclude matching files from the agent payload (placeholder line instead; never excluded from the commit). Unions with `.stagecoachignore` and `[generation].exclude` (§9.18). |
| `--format <mode>`                                | `stagecoach_FORMAT`                      | `stagecoach.format`         | `auto`                       | Message format: `auto` (style learning) \| `conventional` \| `gitmoji` \| `plain` (§9.19, FR-F1).                                                                             |
| `--locale <lang>`                                | `stagecoach_LOCALE`                      | `stagecoach.locale`         | —                            | Write the message in this language (free-form name or BCP-47 tag; FR-F6).                                                                                                     |
| `--context <text>`                               | —                                        | —                           | —                            | Free-text hint appended to the payload for the message + planner roles (FR-F7).                                                                                               |
| `--template <tpl>`                               | `stagecoach_TEMPLATE`                    | `stagecoach.template`       | —                            | Wrap every generated message; `$msg` = the message (hard error if absent; FR-F8).                                                                                             |
| `--edit`                                         | —                                        | —                           | `false`                      | Open `$EDITOR` on the message before the atomic commit; staging stays safe during the edit (§9.22, FR-E1–E4).                                                                 |
| `--push`                                         | `stagecoach_PUSH`                        | `stagecoach.push`           | `false`                      | Plain `git push` after a fully-successful run; never prompts (FR-P1–P3).                                                                                                      |
| `--no-verify`                                    | `stagecoach_NO_VERIFY`                   | `stagecoach.no_verify`      | `false`                      | Bypass `pre-commit` and `commit-msg` hooks for this commit (mirrors `git commit --no-verify`; `prepare-commit-msg` and `post-commit` still run). §9.25, FR-V5.                |
| `--dry-run`                                      | —                                        | —                           | `false`                      | Generate and print the message; do not commit.                                                                                                                                |
| `--verbose`, `-v`                                | `stagecoach_VERBOSE`                     | —                           | `false`                      | Print resolved command, payload size, raw stdout+stderr, retries (FR50).                                                                                                      |
| `--no-color`                                     | `stagecoach_NO_COLOR`                    | —                           | TTY-aware                    | Disable color. Respects `NO_COLOR`.                                                                                                                                           |
| `--version`                                      | —                                        | —                           | —                            | Print version and exit.                                                                                                                                                       |
| `--help`, `-h`                                   | —                                        | —                           | —                            | Help.                                                                                                                                                                         |

### 15.3 Subcommands

- **`stagecoach providers list`** — List all known providers (built-in + user). Mark detected (on `$PATH`) vs not. Show the resolved default (highest-priority _installed_ built-in per FR-D1's order: pi, opencode, cursor, agy, qwen-code, codex, claude).
- **`stagecoach providers show <name>`** — Print the fully-resolved manifest as TOML.
- **`stagecoach config init`** — Bootstrap a **populated, working** config (auto-detects the default provider and writes its per-role models); `--provider <name>` to target one, `--force` to overwrite **while preserving existing active settings (FR-B8)**, `--template` for the inert reference, `--interactive` for the TTY-gated wizard (§9.17, §9.23 FR-L3). To bring an existing config up to the current schema, use `config upgrade`, **not** `init --force` (FR-B4/B5).
- **`stagecoach config path`** — Print the resolved global config path.
- **`stagecoach config upgrade`** — Rewrite an existing config to the current schema version in place (§9.17, FR-B5).
- **`stagecoach hook install|uninstall|status`** — Manage the per-repo `prepare-commit-msg` hook (§9.20). `install --print` emits the script instead of writing; `install --strict` makes generation failures abort the commit (default: never block, FR-H5). Refuses to touch a foreign hook (FR-H2).
- **`stagecoach hook exec <msg-file> [<source> [<sha>]]`** — The hook runtime (called by the installed script, not by users): fills the commit-message file from the staged diff; no-op when a message source already exists (FR-H4).
- **`stagecoach integrate list|install <target>…|remove <target>…`** — Wire stagecoach into installed git tools (§9.21). Targets: `git-alias` (adds `git stagecoach`; `--alias-name` overrides), `lazygit` (customCommands keybind; `--key` overrides `<c-a>`). Every file edit runs the no-mangle protocol (FR-I3): preview diff + `y/N` (skip with `--yes`), timestamped backup, post-write validation with auto-restore.
- **`stagecoach models [<provider>]`** — List models reachable by a provider, via the manifest's `list_models_command` where the agent CLI supports it, else the curated FR-D4 table; `--all` covers every detected provider. Never an HTTP call (§9.23, FR-L1).
- **`stagecoach lock status`** — Read-only diagnostic for this repo's run lock (§9.27, FR-K4): prints the lock path, the holder's `pid`/`hostname`/`repo`/`timestamp`/`snapshot`, whether the holder is alive, and (Unix) whether it appears orphaned (reparented). With no lock held, prints "no run lock for `<repo>`". Changes nothing; the user decides whether to `kill`/`rm`. Never auto-breaks (FR52).
- **`stagecoach upgrade [--check] [--version <v>] [--prerelease] [--force] [--rollback] [--install-method <m>] [--yes]`** — Install-method-aware self-update (§9.29, FR-U1–U12). Detects how the binary was installed and delegates to that channel's native updater (Homebrew/Scoop/Chocolatey/npm/mise/asdf/go-install), prints the command where running it needs privileges (AUR/Chocolatey) or is declarative (Nix), and falls back to a SHA256-verified, atomic, rollback-able direct-binary swap only for the direct-binary channel. `--check`/`-c` probes without changing anything (exit `6` if an update is available). Never touches any repository, run lock, index, or provider — walled off from the commit-generation core.

### 15.4 Exit codes

| Code  | Meaning                                                                                    |
| ----- | ------------------------------------------------------------------------------------------ |
| `0`   | Success (commit created, or dry-run message printed).                                      |
| `1`   | General error (generation failed, parse failed after retries, agent missing, etc.).        |
| `2`   | Nothing to commit (clean tree after auto-stage, or nothing staged with `--no-auto-stage`). |
| `3`   | Rescue condition (snapshot taken, commit not created — manual recovery printed).           |
| `124` | Timeout (generation exceeded `--timeout`).                                                 |
| `5`   | Busy — another stagecoach run holds this repo's run lock (§18.5). Retry after it finishes.  |
| `6`   | Update available — `stagecoach upgrade --check` found a newer version (informational; §9.29). |

### 15.5 Example invocations

```bash
# Default: commit staged changes with the default provider.
stagecoach

# Use a specific provider + model for one commit (claude is single-backend; bare model).
stagecoach --provider claude --model sonnet

# A multi-backend provider carries its inference backend as a model prefix (FR-R5b).
stagecoach --provider pi --model anthropic/claude-haiku

# Set a per-repo default (persisted in the repo's git config).
git config stagecoach.provider pi
git config stagecoach.model anthropic/claude-haiku

# Dry run: see what it would write, commit nothing.
stagecoach --dry-run

# Quick checkpoint: stage everything and commit in one shot.
stagecoach -a

# Wire up lazygit + the git alias automatically (preview + confirm; §9.21):
stagecoach integrate install git-alias lazygit
# …which writes, into lazygit's config.yml:
#   customCommands:
#     - key: '<c-a>'                       # stagecoach-integration
#       context: 'files'
#       command: 'stagecoach'
#       loadingText: 'Generating commit message…'
#       output: 'none'

# Install the prepare-commit-msg hook: plain `git commit` (and IDE commit
# boxes) opens the editor pre-filled with a generated message (§9.20).
stagecoach hook install

# Review the message in $EDITOR before the atomic commit (staging stays safe).
stagecoach --edit

# Team conventions: conventional commits, in German, with a ticket ref.
stagecoach --format conventional --locale de --template '$msg (#205)'

# Tell the agent what the diff can't say.
stagecoach --context "hotfix for the pagination regression in #812"

# Keep noise out of the agent payload (still committed faithfully; §9.18).
stagecoach -x 'dist/**' -x '*.min.js'

# Push once the whole run has landed cleanly.
stagecoach --push

# Pipe the generated message elsewhere (dry-run, stdout = message only).
stagecoach --dry-run --no-color | tee /tmp/msg.txt

# --- multi-commit decomposition (v2; §13.6) ---
# Dirty tree, nothing staged: auto-decompose into as many commits as warranted.
stagecoach

# You know it's three logical changes — skip the "how many?" step.
stagecoach --commits 3

# Force the old one-commit behavior (or equivalently --commits 1).
stagecoach --single

# Route planning to a big context, keep messages on the fast default.
stagecoach --planner-provider agy --planner-model gemini-3.1-pro

# Per-repo: plan with Antigravity's quota, messages with pi's.
#   .stagecoach.toml:
#     [defaults]
#       provider = "pi"
#       model    = "anthropic/claude-haiku"
#     [role.planner]
#       provider = "agy"
#       model    = "gemini-3.1-pro"
```

---

## 16. Configuration model (full)

### 16.1 Resolution order (FR34), lowest to highest

1. **Built-in defaults** (`internal/config/defaults.go`): timeout 120s (global fallback for every role; **planner role default 480s** — FR-R7), auto_stage_all true, max_diff_bytes 300000, max_md_lines 100, token_limit 0 (unset ⇒ legacy per-section caps; FR3d), diff_context 1 (FR3f), max_duplicate_retries 3, output raw, strip_code_fence true, multi_turn_fallback true, multi_turn_chunk_tokens 32000 (§9.24 FR-T1/FR-T3), hook_timeout 10m (§9.25 FR-V6), work_desc_read_rounds 5 (§9.26 FR-W6), upgrade channel "stable" + source_repo "dabstractor/stagecoach" (§9.29 FR-U10).
2. **Built-in provider defaults** (`internal/provider/builtin.go`): the manifests in §12.3–12.7.
3. **Global config file** (`$XDG_CONFIG_HOME/stagecoach/config.toml`, default `~/.config/stagecoach/config.toml`).
4. **Per-repo config file** (`./.stagecoach.toml`, if present; not committed by default — added to a generated `.gitignore` only on `config init` if the user confirms).
5. **Per-repo git config** (`stagecoach.*` keys; read via `git config --get`).
6. **Environment variables** (`stagecoach_*`).
7. **CLI flags.**

Higher wins. Agent-platform manifests merge field-by-field (a user override that sets only `default_model` leaves all other fields from the built-in manifest intact).

**`config_version` is metadata, not a precedence layer.** Every config file carries `config_version = <int>`; on load, stagecoach compares it to its compile-time `CurrentConfigVersion` and emits a purely-advisory staleness notice pointing only at the non-destructive `config upgrade` (never `config init --force`) per §9.17 FR-B4/B5. It does not participate in value resolution.

**`.stagecoachignore` is not a config layer either.** Exclusion patterns are list-valued and **union** across all sources — built-in denylist, `.stagecoachignore`, `[generation].exclude` (global and repo), `--exclude` — rather than overriding each other (§9.18, FR-X1). Precedence applies to scalars; exclusions accumulate.

### 16.2 Full config file example

```toml
# ~/.config/stagecoach/config.toml  (config_version 3)

[defaults]
provider = "pi"          # the AGENT PLATFORM (pi, claude, opencode, …)
model    = "anthropic/claude-haiku"  # inference provider is a slash-PREFIX for multi-backend providers (FR-R5b); bare for single-backend
reasoning = "off"         # off|low|medium|high; shipped default is off for every role (FR-R6)
timeout  = "120s"         # global fallback for every role (FR-R7); planner defaults to 480s
auto_stage_all = true
verbose  = false

[generation]
max_diff_bytes      = 300000   # legacy per-section cap (non-markdown bytes); ignored when token_limit is set (FR3d)
max_md_lines        = 100      # legacy per-section cap (markdown lines/file); ignored when token_limit is set (FR3d)
token_limit         = 0        # holistic token budget for the WHOLE payload (prompt+examples+diff); 0 = unset ⇒ use the legacy caps above. Set to your model's context window, e.g. 120000, so the payload always fits without a per-model registry (FR3d)
diff_context        = 1        # unchanged lines of context around each hunk: 0 = changed lines only (max savings), 1 = one anchor line (default), 3 = git's default (FR3f)
max_duplicate_retries = 3
output              = "raw"     # raw | json
strip_code_fence    = true
subject_target_chars = 50
binary_extensions   = []        # extra non-text extensions to filter (FR3a; merges with built-in denylist)
max_commits         = 12        # safety cap on auto-decompose (FR-M4)
multi_turn_fallback   = true     # on one-shot failure of a large diff, retry via lossless multi-turn session priming (§9.24 FR-T1)
multi_turn_chunk_tokens = 32000  # per-request chunk size (tokens est) for multi-turn fallback (§9.24 FR-T3)
work_desc_read_rounds = 5        # max read rounds (model responses with READ requests) in work-description mode (§9.26 FR-W6)
exclude             = []        # payload-exclusion globs; unions with .stagecoachignore and --exclude (§9.18)
format              = "auto"    # auto | conventional | gitmoji | plain (§9.19, FR-F1)
locale              = ""        # e.g. "German" or "pt-BR"; empty = no language instruction (FR-F6)
template            = ""        # e.g. "$msg (#205)"; $msg is replaced with the generated message (FR-F8)
push                = false     # plain `git push` after a fully-successful run (§9.22, FR-P1)

[upgrade]                         # self-update (§9.29); global config only — no per-repo meaning
channel     = "stable"            # stable | prerelease (admits -rc/-beta tags; = --prerelease)
source_repo = "dabstractor/stagecoach"   # release source; override for a fork. Compile-time default.

# Override a built-in provider/agent platform (field-merged with the built-in manifest).
[provider.pi]
default_model = "gpt-5.4-mini"        # the platform's default model (no default_provider field in v3 — the prefix lives on `model`)
# tooled_flags let this provider serve the STAGER role; omit to exclude it.
# tooled_flags = ["--allowed-tools", "Bash(git:*),Read,Edit", "--approval-mode", "auto"]

# Define a brand-new provider/agent platform (§12.8).
[provider.myagent]
command = "/opt/myagent/bin/agent"
prompt_delivery = "stdin"
print_flag = "--once"
model_flag = "--model"
default_model = "my-model-7b"
system_prompt_flag = "--system"
bare_flags = ["--no-mcp", "--ephemeral"]
output = "raw"
```

### 16.3 Git-config keys (alternative to a file)

For users who prefer to keep config with the repo and don't want a `.stagecoach.toml`:

```ini
[stagecoach]
    provider = pi          # the agent platform
    model = anthropic/claude-haiku    # inference provider is the slash-prefix (FR-R5b)
    timeout = 90
    autoStageAll = true
    noParentWatchdog = false   # v2.7 (§9.27, FR-K6): opt out of the parent-death lock watchdog
```

Read with `git config --get stagecoach.provider`, etc. Booleans via `git config --bool`. This composes naturally with the author's existing `git commit-pi` alias habit and with `git config --local` vs `--global`.

### 16.4 Per-role provider/model configuration (v3; → G12, FR-R1–R6)

The four roles — **planner, stager, message, arbiter** (§13.6.2) — each resolve their **provider** (agent platform), **model** (inference-provider-prefixed for multi-backend providers, FR-R5b), and **reasoning** level (FR-R6) independently. A single global default covers all of them; per-role tables override the fields you care about.

**Resolution for a role's provider/model/reasoning (highest wins), applied independently per field:** CLI flag → env → `[role.<role>]` config → `[defaults]` config (the global) → built-in manifest default. The globals are `[defaults].provider` / `[defaults].model` / `[defaults].reasoning` (i.e. `--provider`/`--model`/`--reasoning`, `stagecoach_PROVIDER`/`stagecoach_MODEL`/`stagecoach_REASONING`). On the single-commit path the only active role is `message`, so setting just the globals is exactly equivalent to v1 — back-compatible.

```toml
# One setting for everything: set only [defaults].
[defaults]
provider = "pi"
model    = "anthropic/claude-haiku"   # multi-backend: inference provider is the prefix (FR-R5b)
reasoning = "off"       # global default for every role; off is the shipped default (FR-R6)

# Granular: route planning to a large-context provider, leave the rest on the global.
[role.planner]
provider = "agy"        # Antigravity quota for the big-context reasoning (single-backend → bare model)
model    = "gemini-3.1-pro"
reasoning = "high"      # OPT-IN: turn thinking on for the planner only (off by default; FR-R6)

[role.stager]           # tooled provider that runs git; needs tooled_flags in its manifest
provider = "agy"
model    = "gemini-3.5-flash"

[role.message]          # bare commit-message role — inherits [defaults] (pi)
# (omit to inherit)

[role.arbiter]          # bare leftover arbiter — inherits [defaults]
# (omit to inherit)
```

Env: `stagecoach_<ROLE>_{PROVIDER,MODEL,REASONING}` (e.g. `stagecoach_PLANNER_MODEL`). Flags: `--<role>-provider` / `--<role>-model` / `--<role>-reasoning` (**all four roles, including `message`**). **Model strings are provider-specific** (FR-R5): a role's `model` is interpreted by _that role's_ resolved provider's manifest; for multi-backend providers it is `inference/model` (FR-R5b). A role routed to a provider whose manifest has empty `tooled_flags` cannot serve as the **stager** (it lacks a safe tooled profile); stagecoach rejects that combination up front. **A bare model (no `/`) on a `provider_flag` provider like pi is a hard error** (FR-R5b) — never silently rendered as a bare `--model`.

---

