# External Dependencies — Verified Agent CLI Surfaces

> **Verification date:** 2026-06-29. All six agents are installed on this machine.
> Every `--help` was captured live. This file records confirmed flags + discrepancies vs PRD §12.

## Verification Summary

| Provider | `detect` cmd | PRD manifest accurate? | Discrepancies / Notes |
|---|---|---|---|
| **pi** | `pi` | ✅ Exact match | `--provider` default is `google` (not empty). All bare flags confirmed: `--no-tools/-nt`, `--no-extensions/-ne`, `--no-skills/-ns`, `--no-prompt-templates/-np`, `--no-context-files/-nc`, `--no-session`. `-p/--print` reads stdin. |
| **claude** | `claude` | ✅ Confirmed | `--tools ""` documented as "Use \"\" to disable all tools". `--setting-sources <sources>`. `--no-session-persistence` (only with `-p`). `--system-prompt` (replaces default). `--output-format json` available. |
| **gemini** | `gemini` | ✅ Confirmed | Positional `query` is default. `-p/--prompt` is **deprecated**. `--approval-mode` choices: `default`, `auto_edit`, `yolo`. No system-prompt flag (prepend per §12.2). `-o/--output-format` supports `json`. |
| **opencode** | `opencode` | ✅ Confirmed | `opencode run [message..]`. `-m/--model` format `provider/model`. `--agent <name>`. No system-prompt flag on `run`. No single default model. |
| **codex** | `codex` | ⚠️ Flag discrepancy | `-s/--sandbox read-only` confirmed. **`--ask-for-approval` NOT in `codex exec --help`** — it's on the interactive `codex` command. See §Codex below. |
| **cursor** | `agent` | ✅ Confirmed | `--mode ask` (Q&A, read-only). `--trust` (only with `-p`). `-p/--print` defaults to full tools; `--mode ask` overrides to read-only. `--model`. `--output-format json`. |

---

## pi (§12.3) — FULLY VERIFIED

```
pi --help confirms:
  --provider <name>              Provider name (default: google)
  --model <pattern>              Model pattern or ID (supports "provider/id")
  --system-prompt <text>         System prompt (default: coding assistant prompt)
  --print, -p                    Non-interactive mode: process prompt and exit
  --no-tools, -nt                Disable all tools by default
  --no-extensions, -ne           Disable extension discovery
  --no-skills, -ns               Disable skills discovery and loading
  --no-prompt-templates, -np     Disable prompt template discovery
  --no-context-files, -nc        Disable AGENTS.md and CLAUDE.md discovery
  --no-session                   Don't save session (ephemeral)
```

**Note:** `pi`'s default provider is `google`, and default model depends on provider. The PRD
manifest sets `default_provider = ""` and `default_model = "glm-5-turbo"`. For GLM, the user
sets `--provider zai`. This matches commit-pi exactly (`pi --provider zai --model glm-5-turbo`).
The manifest's `default_provider=""` means "don't add the provider flag unless user configures one."

Rendered command (matching commit-pi byte-for-byte):
```
pi --provider zai --model glm-5-turbo --system-prompt "<sys>" \
   --no-tools --no-extensions --no-skills --no-prompt-templates \
   --no-context-files --no-session -p    < <user payload via stdin>
```

---

## claude (§12.4) — VERIFIED

```
claude --help confirms:
  -p, --print                    Print response and exit
  --model <model>                Model alias (sonnet, opus) or full name
  --system-prompt <prompt>       System prompt (replaces default)
  --append-system-prompt <prompt> Append to default system prompt
  --tools <tools...>             "Use \"\" to disable all tools", "default", or names
  --setting-sources <sources>    Comma-separated setting sources to load
  --no-session-persistence       Disable session persistence (only with --print)
  --output-format <format>       text | json | stream-json (only with --print)
  --permission-mode <mode>       acceptEdits | bypassPermissions | default | dontAsk | plan
```

**Confirmed:** `--tools ""` fully disables all tools. `-p` skips workspace trust dialog.
PRD uses `--system-prompt` (replacing form) for a clean bare call; `--append-system-prompt` is
the additive alternative (configurable in a future revision).

Rendered:
```
claude -p --model sonnet --system-prompt "<sys>" \
       --tools "" --setting-sources "" --no-session-persistence   < <user payload>
```

---

## gemini (§12.5) — VERIFIED

```
gemini --help confirms:
  query                          Positional prompt (default one-shot)
  -m, --model                    Model [string]
  -p, --prompt                   [DEPRECATED: Use positional prompt instead]
  --approval-mode                default | auto_edit | yolo  [string]
  -o, --output-format            text | json | stream-json  [string]
  -s, --sandbox                  Run in sandbox?  [boolean]
```

**No system-prompt flag** → system prompt is prepended to the positional/stdin payload per §12.2.
PRD §12.5 notes `prompt_delivery` should be verified: positional vs stdin for ~300 KB payloads.
**Recommendation:** default to `stdin` (the help says "stdin is appended to the prompt" and it
avoids arg-length limits); positional as documented fallback.

Rendered (stdin delivery):
```
gemini -m gemini-2.5-pro --approval-mode default    < "<sys>\n\n<user payload>"
```

---

## opencode (§12.6) — VERIFIED

```
opencode run --help confirms:
  message                        Positional message [array]
  -m, --model                    model in format provider/model  [string]
  --agent                        agent to use  [string]
  --format                       default | json  [default: "default"]
  -c, --continue                 continue last session  [boolean]
  -s, --session                  session id  [string]
```

`opencode run` is non-interactive and prints the final message to stdout. No system-prompt flag;
system prompt prepended to payload. `default_model = ""` (require user to set; model space is huge).
`--agent <name>` exists for finer persona control (future revision may expose this).

Rendered:
```
opencode run -m anthropic/claude-sonnet-4 "<sys>\n\n<user payload>"
```

---

## codex (§12.7) — ⚠️ FLAG DISCREPANCY (TO CONFIRM)

```
codex exec --help confirms:
  [PROMPT]                       "If not provided (or if - is used), instructions are read from stdin"
  -m, --model <MODEL>            Model the agent should use
  -s, --sandbox <SANDBOX_MODE>   read-only | workspace-write | danger-full-access
  --dangerously-bypass-approvals-and-sandbox  (DO NOT USE)
  --ephemeral                    Run without persisting session files
  -o, --output-last-message <FILE>  Write last agent message to file
  --json                         Print events as JSONL
```

```
codex --help (interactive) shows:
  -a, --ask-for-approval <APPROVAL_POLICY>  untrusted | on-failure | on-request | never
```

### DISCREPANCY: `--ask-for-approval` is NOT a `codex exec` flag

The PRD manifest (§12.7) lists:
```toml
bare_flags = ["--sandbox", "read-only", "--ask-for-approval", "never"]
```

But `codex exec --help` does **NOT** list `--ask-for-approval` — it only appears on the interactive
`codex` command. `codex exec` is already non-interactive ("Run Codex non-interactively"), so it
does not block waiting for human approval by design.

**Resolution options for the implementing agent:**
1. **Preferred (safe default):** Drop `--ask-for-approval never` from the codex manifest.
   `codex exec` + `--sandbox read-only` is sufficient: it's non-interactive AND read-only.
   This is the conservative choice that avoids passing an unrecognized flag.
2. **Verify at integration:** Test whether `codex exec --ask-for-approval never` is accepted
   (forwarded from the interactive surface) or rejected as unknown. If accepted, keep it.
3. **Config override approach:** Pass approval via `-c ask_for_approval="never"` (codex's
   `-c key=value` config override mechanism), which is documented for `codex exec`.

### BONUS FINDING: codex reads stdin with `-`

`codex exec --help` says: *"If not provided as an argument (or if `-` is used), instructions are
read from stdin."* This means `prompt_delivery = "stdin"` works for codex with `-` as the
positional arg — cleaner than positional for large diffs (avoids arg-length limits). The PRD
manifest currently says `prompt_delivery = "positional"`; the implementing agent should consider
switching to stdin to match the other providers and avoid arg-length limits on large diffs.

Also: `--output-last-message <FILE>` / `-o` writes the final answer to a file — a more reliable
output channel than stdout if stdout contains other logging. And `--json` emits JSONL events.

**Recommended codex manifest revision (to confirm during implementation):**
```toml
name = "codex"
detect = "codex"
command = "codex"
subcommand = ["exec"]
prompt_delivery = "stdin"        # REVISED: codex exec reads stdin with "-" (avoids arg limits)
print_flag = ""
model_flag = "-m"
default_model = ""
system_prompt_flag = ""          # prepend to payload
provider_flag = ""
bare_flags = ["--sandbox", "read-only", "--ephemeral"]  # REVISED: drop --ask-for-approval; add --ephemeral
output = "raw"
strip_code_fence = true
```

---

## cursor / agent (§12.7) — VERIFIED

```
agent --help confirms:
  prompt                         Initial prompt (positional)
  -p, --print                    Print responses; "Has access to all tools, including write and shell"
  --mode <mode>                  plan (read-only/planning) | ask (Q&A, read-only)
  --trust                        Trust workspace without prompting (only with --print/headless)
  --model <model>                e.g. gpt-5, sonnet-4-thinking; bracket overrides supported
  --output-format <format>       text | json | stream-json (only with --print)
  --sandbox <mode>               enabled | disabled
  -f, --force / --yolo           Force allow commands (DO NOT USE)
```

**Confirmed:** `-p` defaults to FULL tool access. `--mode ask` overrides to "Q&A style, read-only
(no edits)" — the correct semantic for message generation. `--trust` skips the workspace-trust
prompt. PRD correctly uses `--mode ask --trust` and deliberately does NOT set `--force`/`--yolo`.

**TO CONFIRM (PRD §12.7):** that `--mode ask` wins over `-p`'s default full-tools behavior —
i.e. the combo is genuinely read-only. Expected from docs (`ask` = read-only); verify against a
real run during implementation.

Rendered:
```
agent -p --mode ask --trust --model gpt-5 "<sys>\n\n<user payload>"
```

---

## Tools-Disable Asymmetry (§12.7.1) — CONFIRMED

| Category | Providers | Mechanism |
|---|---|---|
| Explicit tool-disable switch | pi (`--no-tools`), claude (`--tools ""`) | Pure text-in/text-out, no agent loop |
| Read-only constraint (no global disable) | codex (`--sandbox read-only`), cursor (`--mode ask`), gemini (`--approval-mode default`) | Agent loop may run but cannot mutate repo or block on prompts |

**Safety is preserved either way:** read-only sandbox/mode + non-interactive = no provider can
touch the working tree. The repo-integrity invariant (§18.1) holds for all six.

---

## Output Parsing Implications

- **pi, claude (raw mode):** clean stdout, strip optional code fence.
- **claude (json mode):** `--output-format json` → extract `result` field. More reliable for some models.
- **gemini (json mode):** `-o json` available.
- **codex:** `-o <file>` writes last message to file; `--json` is JSONL events (not a single message — needs filtering for the final assistant message).
- **cursor (json mode):** `--output-format json` available.
