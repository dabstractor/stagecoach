# External Dependencies & Verified Provider Manifests

## A. Go dependencies (PRD §22.3)

| Module | Purpose | Status |
|---|---|---|
| `github.com/spf13/cobra` | CLI framework, subcommands (`providers`, `config`), familiar UX | mature, recommended |
| `github.com/pelletier/go-toml/v2` | config + manifest TOML parse/encode | mature, v2 API |
| Go stdlib (`os/exec`, `os/signal`, `encoding/json`, `context`, `time`, `flag`) | subprocess, signals, json parse, timeouts | stdlib |
| **`go-git` — explicitly NOT used.** | — | Shells out to real `git` binary (matches reference, smaller dep, identical semantics). |
| `goreleaser` (build-time) | cross-compile + Homebrew/Scoop/AUR/release | CI tool, not a runtime dep |

All are compatible with Go 1.22+; host has go1.26.4. `go mod init` target module path: `github.com/dustin/stagehand` (per PRD §21 `go install` path). The author should confirm the exact org/user segment; the plan assumes `github.com/dustin/stagehand`.

## B. Provider manifests — verified against LIVE `--help` (2026-06-30)

All six agents are installed. The PRD's six manifests were cross-checked against real `--help`. **Four corrections** improve fidelity vs the PRD text; they are marked 🔧 below and justified inline. The PRD's two `# TO CONFIRM` items (Appendix E.4) are resolved/strongly-indicated.

### B.1 pi — VERIFIED, matches PRD §12.3 ✅
Verified present in `pi --help` (v0.80.2): `--provider`, `--model`, `--system-prompt`, `--print/-p`, `--no-tools/-nt`, `--no-extensions/-ne`, `--no-skills/-ns`, `--no-prompt-templates/-np`, `--no-context-files/-nc`, `--no-session`. **All confirmed.** pi reads prompt from **stdin** when no positional given (matches old CC shape). No change to PRD §12.3.

```toml
name = "pi"; detect = "pi"; command = "pi"
prompt_delivery = "stdin"; print_flag = "-p"
model_flag = "--model"; default_model = "glm-5-turbo"   # commit-pi default; user-config dependent
system_prompt_flag = "--system-prompt"
provider_flag = "--provider"; default_provider = ""     # user sets "zai"
bare_flags = ["--no-tools","--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session"]
output = "raw"; strip_code_fence = true
```
Rendered: `pi --provider zai --model glm-5-turbo --system-prompt "<sys>" --no-tools --no-extensions --no-skills --no-prompt-templates --no-context-files --no-session -p  <stdin>`. **Byte-identical to `commit-pi`.**

### B.2 claude — 🔧 CORRECTED (add `--disable-slash-commands` + `--no-chrome`)
`claude --help` (v2.1.69) confirms ALL of these exist: `--setting-sources`, `--tools` ("Use `""` to disable all tools"), `--disable-slash-commands` ("Disable all skills"), `--no-chrome`, `--no-session-persistence`, `--system-prompt` (replaces default), `--append-system-prompt`, `--output-format json`. PRD §12.4 listed only 3 bare flags; the proven `commit-claude` uses 5. **Use the fuller set.**
```toml
name = "claude"; detect = "claude"; command = "claude"
prompt_delivery = "stdin"; print_flag = "-p"             # -p also skips workspace-trust dialog (only in trusted dirs)
model_flag = "--model"; default_model = "sonnet"         # alias; user can override with full name
system_prompt_flag = "--system-prompt"                   # REPLACES default persona (use --append-system-prompt to add)
provider_flag = ""
bare_flags = ["--setting-sources","","--tools","","--disable-slash-commands","--no-chrome","--no-session-persistence"]
output = "raw"                                           # alt: output="json" + add "--output-format","json", json_field="result"
strip_code_fence = true
```
Rendered: `claude -p --model sonnet --system-prompt "<sys>" --setting-sources "" --tools "" --disable-slash-commands --no-chrome --no-session-persistence  <stdin>`. **Byte-identical to `commit-claude`.** (PRD's subset would work but is less "bare" than the proven script.)

### B.3 gemini — delivery decision carried to integration
`gemini --help` (v0.19.4) confirms: `-m/--model`, `--approval-mode` (choices `default|auto_edit|yolo`), NO `--system-prompt` (→ prepend to payload), `-p/--prompt` is **DEPRECATED** ("Use the positional prompt instead"), `-o/--output-format` (text|json|stream-json). Help says `-p ... Appended to input on stdin (if any)` ⇒ stdin is read but the exact stdin-without-`-p` behavior is ambiguous without a real run.
- **Resolution for PRD Appendix E.1:** keep `prompt_delivery = "positional"` as the PRD §12.5 default (verified: positional `query` ⇒ one-shot). Note stdin as the *preferred-if-verified* alternative (avoid arg-length limits); the 300 KB diff cap (FR3) mitigates the positional arg-length risk. The implementer confirms at integration which handles ~300 KB. This matches PRD §12.5's own caveat.
```toml
name = "gemini"; detect = "gemini"; command = "gemini"
prompt_delivery = "positional"      # default; stdin preferred-if-verified at integration (E.1)
print_flag = ""                     # positional implies one-shot; -p is DEPRECATED — do not use
model_flag = "-m"; default_model = "gemini-2.5-pro"
system_prompt_flag = ""             # none → prepend to payload (§12.2)
provider_flag = ""
bare_flags = ["--approval-mode","default"]
output = "raw"; strip_code_fence = true
```
Rendered: `gemini -m gemini-2.5-pro --approval-mode default "<sys>\n\n<diff>\n\n<instruction>"`.

### B.4 opencode — VERIFIED, matches PRD §12.6 ✅
`opencode run --help` (v1.1.23) confirms: `run [message..]` positional (array), `-m/--model` ("format provider/model"), `--agent`, `--format` (default|json), `--prompt`, no system-prompt flag. `opencode run` is non-interactive and prints final message to stdout. No change to PRD §12.6.
```toml
name = "opencode"; detect = "opencode"; command = "opencode"; subcommand = ["run"]
prompt_delivery = "positional"; print_flag = ""
model_flag = "-m"; default_model = ""        # require user set (provider/model e.g. anthropic/claude-sonnet-4)
system_prompt_flag = ""                      # none → prepend to payload
provider_flag = ""; bare_flags = []
output = "raw"                              # alt: --format json
strip_code_fence = true
```
Rendered: `opencode run -m anthropic/claude-sonnet-4 "<sys>\n\n<diff>\n\n<instruction>"`. (`--agent <name>` persona control is a v1.1 enhancement, PRD Appendix E.3.)

### B.5 codex — 🔧 CORRECTED (stdin delivery + `--ephemeral`)
`codex exec --help` (codex-cli 0.142.4) is decisive:
- `codex exec` (alias `e`) = "Run Codex non-interactively" ✅ **resolves Appendix E.4a** (it is the documented non-interactive runner; writes the answer to stdout).
- **`[PROMPT]`: "If not provided as an argument (or if `-` is used), instructions are read from stdin."** ⇒ **codex supports stdin.** Switching `prompt_delivery` from the PRD's `positional` to **`stdin`** avoids arg-length limits and is consistent with pi/claude. Pass NO positional (pipe everything via stdin).
- `-m/--model` ✅, `-s/--sandbox` (choices incl. `read-only`) ✅, `-a/--ask-for-approval` (choices incl. `never`) ✅, plus **`--ephemeral`** ("Run without persisting session files to disk") — a perfect bare-mode flag the PRD missed. **Add it.**
```toml
name = "codex"; detect = "codex"; command = "codex"; subcommand = ["exec"]
prompt_delivery = "stdin"          # 🔧 CORRECTED from positional: codex exec reads stdin (no arg-limit)
print_flag = ""                    # exec is already non-interactive
model_flag = "-m"; default_model = ""   # reads ~/.codex/config.toml
system_prompt_flag = ""            # none → prepend to payload
provider_flag = ""
bare_flags = ["--sandbox","read-only","--ask-for-approval","never","--ephemeral"]   # --ephemeral 🔧 ADDED
output = "raw"; strip_code_fence = true
```
Rendered: `codex exec -m gpt-5 --sandbox read-only --ask-for-approval never --ephemeral  <stdin "<sys>\n\n<diff>\n\n<instruction>">`.

### B.6 cursor (binary `agent`) — matches PRD §12.7 ✅ (E.4b strongly indicated)
`agent --help` (Cursor Agent, 2026.06.26) confirms: `-p/--print` ("Print responses to console… Has access to all tools, including write and shell"), `--mode` (choices `plan|ask`; **ask = "Q&A style… (read-only)"**), `--trust` ("only works with --print/headless mode"), `--model`, `--output-format` (text|json|stream-json). No system-prompt flag (→ prepend).
- **Resolution for PRD Appendix E.4b:** `--mode ask` is defined as **read-only** in the help text, so it constrains `-p`'s default full-tools profile by its documented semantics. ✅ Strongly indicated resolved; a single real run is the final confirmation (per PRD §12.7.2 progressive verification). We do NOT set `--force`/`--yolo`.
```toml
name = "cursor"; detect = "agent"; command = "agent"
prompt_delivery = "positional"; print_flag = "-p"      # -p writes answer to stdout (default = full tools)
model_flag = "--model"; default_model = ""
system_prompt_flag = ""                                # none → prepend to payload
provider_flag = ""
bare_flags = ["--mode","ask","--trust"]                # ask = read-only; --trust skips workspace prompt (headless)
output = "raw"; strip_code_fence = true
```
Rendered: `agent -p --mode ask --trust --model gpt-5 "<sys>\n\n<diff>\n\n<instruction>"`.
**Note:** `agent` does not document stdin reading → keep `positional` + 300 KB cap (FR3). (Some installs expose this as `cursor agent` — if `agent` is absent, set `command="cursor" subcommand=["agent"]`; PRD §12.7.)

## C. Summary of manifest corrections vs PRD text (apply in `internal/provider/builtin.go`)

1. **claude** — add `--disable-slash-commands`, `--no-chrome` to `bare_flags` (proven `commit-claude` uses them; current `--help` confirms they exist). [ref_impl.md D4]
2. **codex** — change `prompt_delivery` `positional`→**`stdin`**; add `--ephemeral` to `bare_flags`. Resolves Appendix E.4a. [B.5]
3. **gemini** — keep `positional` (PRD default); carry stdin-vs-positional to integration per E.1. [B.3]
4. **cursor** — unchanged from PRD; E.4b strongly indicated resolved (ask=read-only). [B.6]

The manifest **schema** (PRD §12.1) and all field names are **unchanged and fixed**. Only four default-provider field values differ from the PRD's illustrative TOML.

## D. git plumbing commands used (all verified against git 2.54)

| Operation | Command | Notes |
|---|---|---|
| staged files (md) | `git diff --cached --name-only -- '*.md' '*.markdown'` | per-file diff, `head -n 100` |
| staged diff (other) | `git diff --cached -- ':!*.lock' ... :!vendor/* :!.md :!.markdown` | `head -c 300000` |
| has staged? | `git diff --cached --quiet` | exit status |
| stage all | `git add -A` | |
| parent sha | `git rev-parse HEAD` | empty allowed (root repo) |
| snapshot tree | `git write-tree` | fails on unresolved merge conflicts |
| commit count | `git rev-list --count HEAD` | `\|\| 0` |
| last 20 msgs | `git log --format='---%n%B' -20` | trim blanks, `head -100` |
| last 50 subjects | `git log --format=%s -50` | dedupe set |
| create commit | `git commit-tree [-p <parent>] -m <msg> <tree>` | dangling until update-ref |
| advance HEAD (CAS) | `git update-ref HEAD <new> <expected-old>` | 2-arg form; refuses if HEAD moved |
| success print | `git --no-pager diff-tree --no-commit-id --name-status -r <new>` | |

No `git commit` is ever called (PRD §13.1).
