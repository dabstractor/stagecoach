# PART II — TECHNICAL SPECIFICATION

## 11. Architecture overview

### 11.1 High-level data flow

```
                         ┌──────────────┐
   git index (staged) ──▶│  diff capture │── diff payload ──┐
                         └──────────────┘                   │
                                                            ▼
                         ┌──────────────┐          ┌────────────────┐
                         │  snapshot    │          │ prompt builder │
                         │ write-tree   │          │ (style learn)  │
                         └──────┬───────┘          └────────┬───────┘
                                │ TREE_SHA, PARENT  system+user prompt
                                │                           │
                                │                           ▼
                                │                 ┌──────────────────┐
                                │                 │ provider executor│──▶ external CLI agent (stdin→stdout)
                                │                 └────────┬─────────┘
                                │                          │ raw/json output
                                │                          ▼
                                │                 ┌──────────────────┐
                                │                 │  parse + dedupe  │── retry loop ──┐
                                │                 └────────┬─────────┘               │
                                │                          │ commit message          │
                                ▼                          ▼
                         ┌──────────────────────────────────────┐
                         │ commit-tree -p PARENT -m MSG TREE     │──▶ NEW_SHA
                         │ update-ref HEAD NEW PARENT (atomic)   │
                         └──────────────────────────────────────┘
                                │ on failure ──▶ rescue protocol (print TREE_SHA + recovery cmd)
```

The flow is deliberately linear and synchronous for the **single-commit** path. Concurrency (stage-while-generating) is achieved not by backgrounding Stagecoach, but by the user running `git add` in another terminal/pane during the blocking generation call — which is safe precisely because the commit is built from the frozen `TREE_SHA`, not the live index. See §13 for the full mechanics. The **multi-commit** path (§13.6) layers additional, _internal_ concurrency on top of the same invariant: stage[i+1] overlaps generate[i], safe for the identical reason.

### 11.2 Process model

Stagecoach is a single process. It shells out to git (multiple times) and to the agent CLI (once per attempt). All subprocesses inherit stagecoach's working directory (the repo root) and environment, with a controlled, minimal set of extra env vars passed to the agent only if the manifest requests them. Stagecoach owns signal handling: SIGINT/SIGTERM propagates to the currently-running child and then triggers the rescue path. At most one stagecoach process may produce commits in a given repo at a time: a per-repo run lock (FR52 / §18.5) serializes concurrent invocations on the same repo so two cannot race on HEAD.

### 11.3 Design constraints that protect v2 (now realized)

The v2 multi-commit feature needs to: (a) partition the diff, (b) for each partition, stage exactly that subset, snapshot, generate, commit, repeat. The v1 architecture was built to make this trivially composable, and v2 now realizes it (§13.6). Concretely:

- The core is a function `commitStaged(ctx, cfg) error` (or `(sha, error)`) that assumes the index is already in the desired state. It does not decide _what_ to stage; it commits _whatever is staged_.
- The single-commit path is: `maybeAutoStage(); commitStaged()`.
- The multi-commit path (§13.6) is: `plan() → for each concept { stageConcept(); snapshot(); generate() ‖ stageNextConcept(); commit() }; arbitrate()`. It composes `commitStaged`'s primitives (snapshot/commit-tree/update-ref) per concept, but drives staging from the _planner's_ concepts rather than from a user `git add`.

The keystone discipline: **staging policy is never entangled with commit logic.** §14's package layout enforces this.

### 11.4 Multi-commit pipeline (data flow)

```
                 nothing staged + dirty working tree
                              │
                              ▼
            ┌────────────┐   full working-tree diff (binary placeholders)
            │  planner   │◀──── + style examples
            │ (bare)     │
            └─────┬──────┘   JSON: {count, single, commits:[…], message?}
                  │ single? ──yes──▶ git add -A → CommitStaged (one call) → done
                  ▼ no (N concepts)
         for i in 0..N-1:
            ┌────────────┐  concept[i] description        ┌────────────┐
            │  stager[i] │──────────────────────▶ index   │            │
            │ (tooled)   │   (mutates index; no commit)   │            │
            └─────┬──────┘                                │            │
                  ▼ tree[i]=write-tree (FROZEN)            │            │
            ┌────────────┐  diff(tree[i-1],tree[i])  ═══▶ │  message[i]│ (bare)
            │            │                                │ (overlaps) │
            │            │  ‖ stager[i+1] runs here       │            │
            └─────┬──────┘                                └─────┬──────┘
                  ▼ msg[i]                                      │
            commit-tree -p newSHA[i-1] tree[i] msg[i] ◀──────────┘
            update-ref HEAD newSHA[i] newSHA[i-1]   (serialized)
                  ▼
         git status clean? ──yes──▶ done
                  │ no
                  ▼
            ┌────────────┐  commits made + leftover diff   target SHA or null
            │  arbiter   │◀───────────────────────────▶  (stagecoach does all git)
            │ (bare)     │
            └────────────┘
```

The two invariants that make `stager[i+1] ∥ message[i]` safe are: (1) **tree[i] is frozen before stager[i+1] begins**, so it captures exactly concept[i]; and (2) **the concept diff is `tree[i-1]→tree[i]`, never index-vs-HEAD**, so message[i] is immune to concurrent staging and to commits landing. See §13.6.3.

### 11.5 Two invocation modes: bare and tooled

Stagecoach invokes agents in one of two modes, selected by role:

- **Bare mode** (existing; §12.1 `bare_flags`): tools off, session-less, chrome-less, ephemeral. A pure text-in/text-out call. Used by the **planner**, **message**, and **arbiter** roles — none of which touch git; they reason over a diff/context stagecoach hands them and return text/JSON.
- **Tooled mode** (new; §12.1 `tooled_flags`): tools on, constrained to staging-relevant tools (git/read/edit, per provider), non-interactive, repo-scoped. Used **only** by the **stager** role, which must mutate the index. This is the single deliberate exception to stagecoach's "agent never touches git" rule.

Both modes reuse the manifest's command/model/provider/subcommand/print_flag/delivery fields; only the flag-set that makes the call "bare" vs "tooled" differs. §12.1 adds `tooled_flags` and §12.2 adds a `mode` to the rendering algorithm. The safety properties in §12.7.1 still hold for bare roles; the stager's safety is enforced by `tooled_flags` (git-only toolset) plus the hard rule that it never runs `commit`/`update-ref`/`push` (stagecoach intercepts by instruction and, defensively, by not granting commit-capable surfaces where a provider allows scoping).

---

## 12. The provider system

Stagecoach's provider system is the heart of its agent-agnosticism: given a logical intent ("call an agent with this system prompt and this user payload, bare and ephemeral, with this model"), produce a concrete command line for a specific **provider** (the agent platform), run it, and parse the result.

**Terminology — two concepts.** Stagecoach configures a model invocation with TWO concepts. There is no separate "inference provider" field — it folds into the model string (below), which is what eliminates the term overload that caused repeated routing bugs.

| Concept      | What it is                                        | Examples                                                    | Config field | Flag         | Env                   | Git key               |
| ------------ | ------------------------------------------------- | ----------------------------------------------------------- | ------------ | ------------ | --------------------- | --------------------- |
| **provider** | the agent platform / CLI stagecoach shells out to | pi, opencode, claude, codex, cursor, agy | `provider`   | `--provider` | `stagecoach_PROVIDER` | `stagecoach.provider` |
| **model**    | the model identifier                              | `anthropic/claude-haiku`, `openai/gpt-5.4`, `sonnet`, `gemini-3.1-pro` | `model`      | `--model`    | `stagecoach_MODEL`    | `stagecoach.model`    |

**The inference provider lives in the model string, not a separate field.** Some providers route to a choice of upstream inference backend — **pi** (via a separate `--provider <backend>` flag) and **opencode** (via a `backend/model` token like `openai/gpt-5.4`). For these, the model string carries the inference provider as a **slash-prefixed namespace**: `anthropic/claude-haiku`, `openai/gpt-5.4`. Providers with a fixed backend (claude, codex, cursor, agy) take a bare model (`sonnet`, `gemini-3.1-pro`).

- **pi renders the prefix as a separate flag; opencode passes it whole.** At `Render` (§12.2), if the provider's manifest declares a `provider_flag` (pi — the only one today), stagecoach splits the model on the first `/` and emits `--provider <prefix> --model <rest>` (so `anthropic/claude-haiku` → `pi --provider anthropic --model claude-haiku`). Providers without a `provider_flag` (opencode, and every single-backend provider) pass the model string verbatim.
- **A bare model on a `provider_flag` provider is a hard error** (FR-R5b): `model = "claude-haiku"` on pi is rejected with "include the inference provider, e.g. `anthropic/claude-haiku`" — never silently rendered as an unroutable `pi --model claude-haiku`. This is precisely the bug class that motivated the design: there is no separate inference-provider field to forget, because **the prefix IS the field**.
- The manifest block is **`[provider.<name>]`**; the former `default_provider` field is **removed** — the model prefix replaces it.

### 12.1 The manifest schema

Each provider (agent platform) is described by a manifest. Manifests are TOML (human-editable, no quoting hell for flag lists). Built-in manifests are compiled into the binary (so the tool works with zero config); user manifests in config files override or extend them.

````toml
# An agent manifest. All fields except `name` and `command` are optional
# with sensible defaults; shown here fully expanded for pi.

name = "pi"

# --- discovery -----------------------------------------------------------
# Command to look up on $PATH to decide if this provider is "installed".
# If absent, `command` is used.
detect = "pi"

# The executable to run. Resolved via exec.LookPath; may be an absolute path.
command = "pi"

# Optional argv that asks the AGENT CLI to list its reachable models, e.g.
# ["opencode", "models"]. Used by `stagecoach models` (§9.23, FR-L1/L2).
# Empty => stagecoach prints its curated per-role defaults table (FR-D4) instead.
# NEVER an HTTP call (§6.2 N2): the agent CLI is the only model authority.
list_models_command = []

# Optional subcommand tokens inserted between `command` and the flags
# (e.g. opencode uses ["run"], codex uses ["exec"]).
subcommand = []

# --- prompt delivery -----------------------------------------------------
# How the user payload (system-built prompt + diff) reaches the agent.
#   "stdin"      → pipe to the process stdin (DEFAULT; avoids arg-length limits)
#   "positional" → append as the final positional argument
#   "flag"       → append after `prompt_flag`
prompt_delivery = "stdin"
prompt_flag = ""          # used only when prompt_delivery = "flag"

# --- non-interactive / print mode ---------------------------------------
# Token(s) that put the agent into non-interactive "print and exit" mode.
print_flag = "-p"

# --- model ---------------------------------------------------------------
model_flag = "--model"
# Default model if the user specifies none. Empty in the shipped pi default
# (decoupled from any one subscription, §9.16 FR-D2); config init fills per-role.
default_model = ""

# --- system prompt -------------------------------------------------------
# If the agent supports a system prompt. Empty means "prepend to the user
# payload" (fallback for agents with no system-prompt flag).
system_prompt_flag = "--system-prompt"

# --- sub-provider (the inference backend) -------------------------------
# pi has a --provider flag; per FR-R5b/§12.2 the inference backend is the slash-PREFIX
# on `model` (e.g. model "anthropic/claude-haiku" → pi --provider anthropic --model claude-haiku). There is
# NO `default_provider` field in v3 — the prefix on `model` IS the provider. opencode
# has no provider_flag and takes `backend/model` verbatim instead.
provider_flag = "--provider"

# --- session continuation (multi-turn fallback, §9.24) ------------------
# "" (default): provider cannot append turns across one-shot calls → multi-turn
#   fallback unavailable for this provider (one-shot → rescue, unchanged).
# "append": re-invoking the same session id appends a turn the model can recall
#   (pi: `--session-id <id> ... -p`, repeated). REQUIRES a verified append
#   rendering (FR-T9); never set speculatively.
session_mode = ""

# --- bare mode -----------------------------------------------------------
# Flags appended verbatim to make the call tool-less, session-less,
# extension-less, chrome-less, and ephemeral. These are agent-specific.
# Used by the bare roles: planner, message, arbiter (§11.5).
bare_flags = [
  "--no-tools",
  "--no-extensions",
  "--no-skills",
  "--no-prompt-templates",
  "--no-context-files",
  "--no-session",
]

# --- tooled mode (v2; §11.5) ---------------------------------------------
# Flags appended verbatim to make the call TOOL-ENABLED, non-interactive, and
# scoped to staging-relevant tools (git / read / edit), for the stager role —
# the ONLY role that touches git. Each provider expresses "tooled but safe" in
# its own idiom (an allowlist, a sandbox, an approval-mode). nil/empty => this
# provider does not support tooled mode and cannot serve as a stager.
tooled_flags = [
  # e.g. for an agent with an allowlist + auto-approve:
  # "--allowed-tools", "Bash(git:*),Read,Edit",
  # "--approval-mode", "auto",
]

# --- reasoning level (optional; FR-R6) ----------------------------------
# Per-level flag tokens appended to express the model's reasoning/thinking
# effort. Omit the table entirely (or declare only `off = []`) for agents or
# models with no reasoning control — a non-`off` level set on such an agent is
# a silent no-op (never an error). Token lists are agent-specific; verify per
# FR-D5. Shape shown for illustration only.
[reasoning_levels]
off = []
# low    = ["--thinking", "low"]
# medium = ["--thinking", "medium"]
# high   = ["--thinking", "high"]

# --- output --------------------------------------------------------------
#   "raw"  → stdout (cleaned) IS the message            [DEFAULT]
#   "json" → stdout is JSON; extract `json_field`
output = "raw"
json_field = ""           # e.g. "result" when output = "json"

# Strip a single layer of markdown code fence (``` or ~~~) if present.
strip_code_fence = true

# --- retry ---------------------------------------------------------------
# Instruction prepended on a parse-retry (empty/invalid output).
retry_instruction = "Output ONLY the commit message. No preamble, no markdown, no quotes."

# --- environment ---------------------------------------------------------
# Extra env vars to set ONLY for the agent subprocess (never global).
[env]
# PI_OFFLINE = "1"   # example; commented out by default
````

### 12.2 Command rendering algorithm

Given a manifest `m`, a resolved model `model` (which, for a multi-backend provider, is in `inference/model` form — §12/FR-R5b), a system prompt `sys`, a user payload `user`, and a **mode** (`"bare"` | `"tooled"`; default `"bare"`):

```
args = [m.subcommand...]
# FR-R5b: a provider with a provider_flag (pi) takes "inference/model"; split it.
# The prefix becomes the agent's --provider; the rest is the model. A bare model
# (no "/") on such a provider is a hard error, not a silent bare --model.
if m.provider_flag != "":
    if model == "":
        pass                                   # no model → emit neither flag
    elif "/" in model:
        inf, model = split(model, "/", 1)
        args += [m.provider_flag, inf]
    else:
        error("model %q on %s must be inference/model, e.g. anthropic/claude-haiku", model, m.name)
if m.model_flag and model != "":
    args += [m.model_flag, model]               # verbatim for non-provider_flag providers (opencode, single-backend)
# reasoning level (FR-R6): append the resolved level's tokens if the provider
# declares them; absent/empty => silent no-op (provider or model lacks reasoning control).
if reasoning != "" and len(m.reasoning_levels[reasoning]) > 0:
    args += m.reasoning_levels[reasoning]
if m.system_prompt_flag and sys != "":
    args += [m.system_prompt_flag, sys]
# mode selects the flag-set: bare (tools off) for planner/message/arbiter,
# tooled (git tools on, scoped) for the stager (§11.5). tooled with no
# tooled_flags defined => error (provider cannot serve as a stager).
args += (mode == "tooled") ? m.tooled_flags : m.bare_flags
if m.print_flag != "":
    args += [m.print_flag]
switch m.prompt_delivery:
  case "stdin":       (prompt goes to stdin; nothing appended)
  case "positional":  args += [user]
  case "flag":        args += [m.prompt_flag, user]

cmd = exec.Command(m.command, args...)
cmd.Stdin = (m.prompt_delivery == "stdin") ? strings.NewReader(sys? + user) : /dev/null
cmd.Env   = os.Environ() + m.env
```

**Note on system prompt + stdin:** when delivery is `stdin` and a system-prompt flag exists, the system prompt goes via the flag and only the user payload goes via stdin (matching `commit-pi`). If the agent has _no_ system-prompt flag (`system_prompt_flag = ""`), the system prompt is prepended to the stdin payload as a fallback.

**FR-R5b (enforced at this chokepoint):** if `m.provider_flag` is set (a multi-backend provider) and `model` is non-empty but contains no `/`, `Render` returns an error rather than emitting a bare `--model` — it splits `backend/model` into `--provider <backend> --model <model>` (§9.15). This is the single command-emission gate every call path flows through, so no path can produce an unroutable command.

**Reasoning level (FR-R6):** `Render` also receives the role's resolved `reasoning` level (`off|low|medium|high`) and appends `m.reasoning_levels[level]` after the model flag. A level the agent does not declare (no tokens) is a silent no-op — never an error — so a non-reasoning model can be pinned anywhere without configuration.

### 12.3 Built-in provider: pi

Captured from `pi --help` on the author's machine (2026-06-29). pi is a **harness** that routes to model backends via its own sub-providers. Its shipped `default_model` is **deliberately empty** — it is populated per-role by `config init` (§9.17), and because pi is multi-backend the model is supplied in `backend/model` form (FR-R5b), e.g. `anthropic/claude-haiku`. The shipped default does **not** assume the author's personal z.ai/GLM subscription (FR-D2); that is shown as a personal _override_ below.

```toml
name = "pi"
detect = "pi"
command = "pi"
prompt_delivery = "stdin"
print_flag = "-p"
model_flag = "--model"
default_model = ""            # empty in the shipped default; config init fills per-role (§9.16/§9.17)
system_prompt_flag = "--system-prompt"
provider_flag = "--provider"          # FR-R5b: the inference backend is the slash-PREFIX on `model`; no default_provider field in v3
bare_flags = [
  "--no-tools",
  "--no-extensions",
  "--no-skills",
  "--no-prompt-templates",
  "--no-context-files",
  "--no-session",
]
output = "raw"
strip_code_fence = true
```

Rendered (model `<backend>/<m>` — stagecoach splits the prefix per FR-R5b):

```
pi --provider <backend> --model <m> --system-prompt "<sys>" \
   --no-tools --no-extensions --no-skills --no-prompt-templates \
   --no-context-files --no-session -p            < <user payload via stdin>
```

**Personal-override example (NOT the shipped default).** The original `commit-pi` script — and the author's daily setup — routes pi to z.ai GLM: `model = "zai/glm-5-turbo"` (the invocation above with `<backend>=zai`, `<m>=glm-5-turbo`, byte-for-byte the `commit-pi` call). That is the author's _subscription-specific_ override, kept here as the reference shape; it is not a default anyone else would inherit.

### 12.4 Built-in provider: Claude Code

Captured from `claude --help`.

```toml
name = "claude"
detect = "claude"
command = "claude"
prompt_delivery = "stdin"           # claude -p reads stdin when no positional given
print_flag = "-p"                   # also enables non-interactive mode
model_flag = "--model"
default_model = "sonnet"            # alias; user can override with a full name
system_prompt_flag = "--system-prompt"
provider_flag = ""                  # n/a
bare_flags = [
  "--tools", "",                   # disable ALL built-in tools (per --help)
  "--setting-sources", "",         # load no settings sources
  "--no-session-persistence",      # ephemeral
]
output = "raw"                      # could use "json" with json_field="result"
strip_code_fence = true
```

Rendered (model `sonnet`):

```
claude -p --model sonnet --system-prompt "<sys>" \
       --tools "" --setting-sources "" --no-session-persistence   < <user payload>
```

Notes:

- `--tools ""` is documented (`Use "" to disable all tools`).
- `--system-prompt` _replaces_ the default; `--append-system-prompt` _adds to_ it. We use the replacing form for a clean, bare call. (Configurable: a user who wants CC's default persona retained can switch the flag to `--append-system-prompt`.)
- `--output-format json` + `json_field = "result"` is an alternative if raw mode proves unreliable with a given model.

### 12.5 ~~Built-in provider: Gemini CLI~~ — REMOVED (superseded by agy, §12.5.1)

Google's **Gemini CLI (`gemini`) is no longer shipped** — it was superseded by **agy** (the Antigravity CLI, §12.5.1) on 2026-06-18, and agy has since diverged from the gemini-cli lineage. The `gemini` built-in manifest, its reference file (`providers/gemini.toml`), and its role-tier defaults (§9.16 FR-D4) have all been removed. Users on the Gemini line should point stagecoach at `agy`.

### 12.5.1 Built-in provider: Antigravity CLI (`agy`) — the Gemini-CLI successor

Antigravity CLI (`agy`) is Google's terminal coding agent; it **superseded `gemini` (Gemini CLI) on 2026-06-18** and is the Gemini lineage's current surface. It matters to stagecoach for the same structural reason every provider does: **the Antigravity coding-plan quota is reachable only through `agy`**, never over the public API. Flag surface below is **verified against `agy --help` + live end-to-end runs (2026-07-08, agy v1.1.0; tooled/stager combo re-verified 2026-07-09, agy v1.1.11)**. The Antigravity CLI has **diverged** from the gemini-cli lineage it forked from: it dropped `--approval-mode`, made `-p`/`--print`/`--prompt` **value-taking**, and uses `--model` (not `-m`). The bare-roles invocation an earlier draft assumed (`--approval-mode default -p` + stdin) no longer works on v1.1.0; the manifest below is corrected and re-verified. agy is **stager-capable** (§12.5.1.1 item 4, verified 2026-07-09) and still ships `experimental` (§12.7.2) pending a full --help re-verification pass.

```toml
# Antigravity CLI. --help + end-to-end verified 2026-07-08 (agy v1.1.0).
name = "agy"
detect = "agy"
command = "agy"
list_models_command = ["agy", "models"]
prompt_delivery = "stdin"          # agy reads stdin when -p is absent (verified); avoids arg limits
print_flag = ""                    # NON-NIL empty: -p is VALUE-TAKING in v1.1.0; a bare -p fails with
                                   # "flag needs an argument: -p". agy reads stdin without it → no print flag.
model_flag = "--model"             # `-m` is REJECTED (verified). gemini-cli lineage's -m is gone.
default_model = "Gemini 3.5 Flash (Low)"   # `agy models` display label VERBATIM (reasoning suffix included)
system_prompt_flag = ""            # none first-class (like gemini-cli) → prepend to payload (§12.2). Confirmed.
provider_flag = ""
# bare mode: read-only, no tool execution (planner / message / arbiter roles).
# agy v1.1.0 has NO --approval-mode (removed). The read-only equivalent is `--mode plan`
# (choices: accept-edits | plan); verified to produce clean commit-message output, no plan-mode noise.
bare_flags = ["--mode", "plan"]
# tooled mode (STAGER): VERIFIED 2026-07-09 (agy v1.1.11, §12.5.1.1 item 4). agy has NO scoped
# --allowed-tools allowlist; the only non-interactive tool auto-approve knob is --dangerously-skip-permissions
# (all tools — UNSCOPED, §12.7.1, same model as pi). `accept-edits` is the mutating counterpart of --mode plan.
tooled_flags = ["--mode", "accept-edits", "--dangerously-skip-permissions"]
tooled_repo_dir_flag = "--add-dir"   # agy does NOT auto-adopt cwd; the stager appends --add-dir <repo>.
output = "raw"
strip_code_fence = true
experimental = true               # item 4 cleared; stays experimental pending a full --help re-verification pass
```

Rendered, bare (model `Gemini 3.5 Flash (Low)`):

```
agy --model "Gemini 3.5 Flash (Low)" --mode plan   < <sys+user payload via stdin>
```

(No `-p` flag: agy v1.1.0's `-p`/`--print`/`--prompt` is value-taking, so a bare `-p` fails. agy reads the prompt from stdin when `-p` is absent and stdin is a pipe. This also routes ~300 KB diffs over stdin, past the 128 KB `MAX_ARG_STRLEN` ceiling that argv/positional delivery would hit.)

Rendered, tooled (stager; repo at `/repo`):

```
agy --model "Gemini 3.5 Flash (Low)" --mode accept-edits --dangerously-skip-permissions --add-dir /repo   < <stager task via stdin>
```

(agy does not auto-adopt its cwd as the workspace, so the stager appends `--add-dir <repo>` — the manifest's `tooled_repo_dir_flag`, threaded from the run's repo path.)

#### 12.5.1.1 Status (agy) — verified 2026-07-08 against agy v1.1.0

1. **RESOLVED — non-TTY stdout drop (issue [#76](https://github.com/google-antigravity/antigravity-cli/issues/76)):** **no longer reproduces on v1.1.0.** `agy` invoked non-interactively with piped stdin (no `-p`) returns stdout correctly — verified with single-token replies and full commit-message generation. The PTY-shim workaround is no longer needed.
2. **RESOLVED — Model flag:** confirmed `--model` (the gemini-cli lineage's `-m` is rejected with "flags provided but not defined"). Model labels are `agy models` display labels VERBATIM, reasoning suffix included (e.g. "Gemini 3.5 Flash (Low)", "GPT-OSS 120B (Medium)"). NOTE: GPT-OSS 120B is subject to transient backend 503 "No capacity available" errors; retries succeed (external capacity issue, not a stagecoach bug).
3. **RESOLVED — Prompt delivery + read-only mode:** agy v1.1.0 **diverged** from gemini-cli. `-p`/`--print`/`--prompt` is **value-taking** (a bare `-p` fails with "flag needs an argument: -p"); agy reads the prompt from **stdin** when `-p` is absent. `--approval-mode` was **removed**; the read-only equivalent is `--mode plan` (verified to emit clean commit messages with no plan-mode formatting noise). No first-class system-prompt flag (sys is prepended to the payload, §12.2).
4. **RESOLVED (unscoped) — Tooled (stager) flags:** verified 2026-07-09 against agy v1.1.11 (end-to-end: a stdin stager task stages exactly the requested paths and leaves the rest alone). agy exposes **no** scoped `--allowed-tools`/`--allowed-tools-pattern` allowlist — the only non-interactive tool auto-approve knob is `--dangerously-skip-permissions` (auto-approves ALL tools). The verified combo is `--mode accept-edits --dangerously-skip-permissions` (plus `--add-dir <repo>`, since agy does not auto-adopt its cwd as the workspace). This is the **UNSCOPED** model (§12.7.1) — the same one **pi** uses; the §17.6 stager prompt + the HEAD-movement guard (Issue 2 / §19) + `verifyFreezeSubset` (FR-M1c) are the safety net, not flag-scoping. A scoped git/Read/Edit allowlist remains a desirable future hardening if agy adds one; until then agy is stager-capable via the unscoped path (parity with pi).
5. **Print-mode timeout:** `agy` exposes `--print-timeout` (default 5m); stagecoach's own `--timeout` (§9.5) governs the kill, but a shorter `--print-timeout` makes `agy` exit cleanly rather than hang — wire it to the same budget.

Items 1–4 are cleared; agy is stager-capable. It still ships `experimental = true` (§12.7.2) pending a full `--help` re-verification pass (item 4 itself is resolved).

### 12.6 Built-in provider: opencode

Captured from `opencode run --help`.

```toml
name = "opencode"
detect = "opencode"
command = "opencode"
subcommand = ["run"]
prompt_delivery = "positional"      # `opencode run [message..]`
print_flag = ""
model_flag = "-m"                   # format: provider/model, e.g. "anthropic/claude-haiku"
default_model = ""                  # opencode has no single sensible default; require user set
system_prompt_flag = ""             # not exposed as a flag on `run`; use --agent or config
provider_flag = ""                  # provider is part of the model string
bare_flags = []
tooled_flags = ["--agent", "build"]   # STAGER (verified 2026-07-09): run stages correctly; tool exec is gated by the build agent's permission config (unscoped)
output = "raw"
strip_code_fence = true
```

**Stager (tooled):** opencode CAN serve as the stager — verified end-to-end 2026-07-09 (`opencode run -m zhipuai-coding-plan/glm-5.2` staged exactly the requested paths). opencode has **no** CLI permission/approval flag; tool execution is governed by the user's opencode permission config (the `build` agent, typically `"permission": {"*": "allow"}`). `--agent build` selects that write-capable primary agent explicitly. NOTE: the earlier "`run` is read-only by design" claim is **wrong** — `run` mutates when the task asks it to; bare roles stay safe only because a message/planner task does not request mutation. §12.7.1 UNSCOPED (same model as pi/agy/codex).

Rendered (model `anthropic/claude-haiku`):

```
opencode run -m anthropic/claude-haiku "<sys>\n\n<user payload>"
```

Caveats: opencode's `run` subcommand is non-interactive and prints the final message to stdout (good). It has no system-prompt flag; the system prompt is prepended to the payload. For finer control of agent persona, opencode supports `--agent <name>` against a user-defined agent in `opencode.json` — Stagecoach can expose this via an `extra_args` passthrough or a dedicated `agent_flag` field in a later revision. `default_model` is intentionally empty: opencode's model space is huge and user-specific, so we require the user to set `model` (via flag/env/config) rather than guess.

### 12.7 Verified providers: Codex, Cursor Agent

Both were verified against their real `--help` output. They are **not** marked `experimental` — the flag surfaces below are confirmed. Two residual details to confirm at integration time are called out inline; neither blocks the manifest shape.

```toml
# Codex (OpenAI). Verified from `codex --help`.
name = "codex"
detect = "codex"
command = "codex"
subcommand = ["exec"]               # `codex exec` (alias `e`) = non-interactive runner
prompt_delivery = "positional"      # positional [PROMPT] to `codex exec`
print_flag = ""                     # `exec` is already non-interactive; no extra token
model_flag = "-m"                   # `-m` / `--model <MODEL>` (confirmed both forms)
default_model = ""                  # codex reads model from ~/.codex/config.toml; user sets if overriding
system_prompt_flag = ""             # NO system-prompt flag exists → prepend to payload (§12.2)
provider_flag = ""
# Codex has no "disable all tools" switch. Tools are intrinsic to its agent loop.
# We constrain it to a safe, non-interactive, non-mutating profile instead:
#   -s read-only      → sandbox forbids writes / network mutations
#   -a never          → never block waiting for human approval (required for non-interactive)
bare_flags = ["--sandbox", "read-only", "--ask-for-approval", "never"]
output = "raw"
strip_code_fence = true
# TO CONFIRM (integration): that `codex exec` writes the assistant's final answer
# to stdout and exits 0 on success (expected; verify against a real run).
```

Rendered (model e.g. `gpt-5`):

```
codex exec -m gpt-5 --sandbox read-only --ask-for-approval never "<sys>\n\n<user payload>"
```

```toml
# Cursor Agent. Verified from `agent --help` (the Cursor Agent CLI).
name = "cursor"
detect = "agent"                    # the standalone binary is `agent`
command = "agent"                   # NOTE: some installs expose this as `cursor agent`
                                   # (the `agent [prompt...]` subcommand). If `agent`
                                   # is not on $PATH, set command="cursor" subcommand=["agent"].
subcommand = []
prompt_delivery = "positional"      # positional [prompt] to the agent
print_flag = "-p"                   # `-p` / `--print` = non-interactive (writes answer to stdout)
model_flag = "--model"              # e.g. gpt-5, sonnet-4-thinking; bracket overrides supported
default_model = ""                  # user sets; cursor has per-account model availability
system_prompt_flag = ""             # NO system-prompt flag exists → prepend to payload (§12.2)
provider_flag = ""
# Cursor's `-p` print mode defaults to FULL tool access ("all tools, including write
# and shell"). We override to a read-only Q&A profile so it cannot mutate the repo:
#   --mode ask   → "Q&A style, read-only" (no edits) — the right semantic for msg gen
#   --trust      → skip the workspace-trust prompt that would otherwise block `-p`
bare_flags = ["--mode", "ask", "--trust"]
tooled_flags = ["--trust", "--yolo"]   # STAGER (verified 2026-07-09): -p is non-interactive WITH FULL TOOLS; --yolo auto-approves (unscoped)
output = "raw"                      # could use "json" with json_field from --output-format json
strip_code_fence = true
# Bare is read-only via `--mode ask` (which overrides -p's full-tools default); the stager OMITS
# `--mode` so -p's full tool access applies, and adds `--yolo` (= --force) to auto-approve tool calls
# non-interactively. Both verified end-to-end 2026-07-09 (cursor-agent): bare emitted clean JSON;
# the stager staged exactly the requested paths. §12.7.1 UNSCOPED (like pi/agy/codex/opencode).
# NOTE: free Cursor plans allow ONLY `--model auto`; named models need a paid plan. An invalid or
# non-entitled model prints the error to STDERR and leaves stdout EMPTY.
```

Rendered (model e.g. `gpt-5`):

```
agent -p --mode ask --trust --model gpt-5 "<sys>\n\n<user payload>"
```

#### 12.7.1 The tools-disable asymmetry (important, documented honestly)

There is a real architectural split across our built-in agents in **how they become "bare"**:

- **Explicit tool-disable flags:** pi (`--no-tools`), Claude Code (`--tools ""`). These agents offer a literal "turn tools off" switch, so the call is a pure text-in/text-out with no agent loop. Fast and clean.
- **Read-only constraint instead:** Codex (`--sandbox read-only`), Cursor (`--mode ask`), Gemini (`--approval-mode default`). These agents have **no** global "disable all tools" switch — tools are intrinsic to their loop. We constrain them to a read-only, never-ask profile so they _cannot_ mutate the repo or block on a prompt, but the model may still internally reason with tools.

Consequences, stated plainly:

1. **Safety is preserved either way.** Read-only sandbox/mode + never-ask means no provider in the default set can touch the working tree. The repo-integrity invariant (§18.1) holds for all six.
2. **Latency varies.** The read-only-constrained agents may be slightly slower (they run an agent loop the model can choose to use). Acceptable for a one-shot message.
3. **Output is still just the message.** Whichever path the model takes, the final assistant text is what Stagecoach parses; a model that "reads a file" before answering still ends with a commit message on stdout.
4. **Chrome is a separate axis — see §9.28 (v2.9).** Mutation safety says nothing about agent chrome: the read-only-constrained providers may still discover/load skills, extensions, context files, and MCP servers and inject them into the prompt (startup latency, token/quota cost, nondeterminism). §9.28 (FR-C1–C5) requires every provider to disable each chrome surface the agent exposes a switch for and to document any surface it cannot. (pi already disables extensions/skills/prompt-templates/context-files; note pi has no `--no-mcp` — MCP tool *use* is suppressed by `--no-tools`, but servers may still connect at startup.)

This is not a defect to paper over — it is the honest cost of supporting heterogeneous agent CLIs through one manifest schema. The `bare_flags` field exists precisely so each provider expresses "bare" in its own idiom.

**The stager role inverts this (v2; §11.5).** The per-concept staging agent is the one place stagecoach _wants_ tools on — it must run `git add` and apply hunks. There the `tooled_flags` field (§12.1) takes over, expressing "tooled but safe" (a git-scoped allowlist or a read/write sandbox) in each provider's idiom. The safety invariant for the stager is therefore not "no tools" but "tools scoped to staging, and never `commit`/`update-ref`/`push`" — enforced by `tooled_flags` plus the hard rule that ref mutations are stagecoach's alone (§13.6.2, §19). A provider with empty `tooled_flags` simply cannot serve as a stager; it can still serve the bare roles.

#### 12.7.2 On stubbing and progressive verification

We do not pretend to know everything up front. The implementing agent will do its own comprehensive research per task. The contract here is: **the manifest _schema_ and the built-in default manifests are fixed by this document; the exact behavior of each manifest is confirmed by a real end-to-end run during implementation.** Two explicit `# TO CONFIRM` notes are carried above (codex stdout-on-success; cursor `ask`-wins-over-`-p`). Any manifest field that cannot be confirmed is left at a safe default and marked with a `# TO CONFIRM` comment, never silently assumed. The `experimental` flag remains available (see §22.1) for any _future_ provider added from docs alone rather than a verified `--help`.

### 12.8 Extensibility: user-defined providers

A user can define a provider unknown to Stagecoach by dropping a `[provider.<name>]` block into any config file. This is how community support for new agents (or future versions of existing ones) lands without a release:

```toml
# ~/.config/stagecoach/config.toml
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

Then `stagecoach --provider myagent` (or `stagecoach.provider = myagent` in git config) uses it. No recompilation.

### 12.9 Output parsing pipeline (detailed)

`parseOutput(raw string, m Manifest) (msg string, ok bool)`:

1. `s = strings.TrimSpace(raw)`.
2. If `m.strip_code_fence` and `s` starts with ` ``` ` or `~~~`: remove the first line (the fence opener, including any language tag) and everything from the last fence closer onward. Re-trim.
3. Switch on `m.output`:
   - **raw**: `msg = s`.
   - **json**: attempt `json.Unmarshal([]byte(s), &obj)`; if it fails, find the first `{` and the matching last `}` (brace-balanced substring) and retry. Extract `obj[m.json_field]` as a string. If anything fails, fall through to raw (treat `s` as the message) and set a "parse-fallback" flag for logging.
4. Normalize newlines: convert `\r\n` → `\n`; collapse 3+ consecutive newlines to 2.
5. `msg = strings.TrimSpace(msg)`. `ok = msg != ""`.

This pipeline is the reason the v1 commit-prompt uses a **raw** contract ("output only the commit message") rather than the JSON contract `commit-pi` used. JSON required the fragile `sed` extraction and the "no double quotes inside the message" constraint; raw + robust cleanup removes both. JSON remains available for agents (like Claude Code) where `--output-format json` is more reliable than raw stdout.

---

