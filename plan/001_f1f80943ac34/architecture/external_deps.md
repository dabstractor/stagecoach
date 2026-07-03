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

**🟢 END-TO-END VERIFIED 2026-07-03 (integration_real suite, P1.M8.T3.S1):** a full real `generate.CommitStaged` run against a temp repo + the REAL `pi` agent produced a real, non-duplicate commit (e.g. `feat(x): add new feature stub`, HEAD advanced) in ~3–11 s. The pi manifest is the ONLY one of the six fully confirmed end-to-end on the verification host. The others are environment-blocked (see §B.2/§B.3/§B.5/§B.6 + Appendix E note below).

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

### B.2 claude — 🔧 CORRECTED (add `--disable-slash-commands` + `--no-chrome`) — E.2 BLOCKED (host not functional)
`claude --help` (v2.1.69) confirms ALL of these exist: `--setting-sources`, `--tools` ("Use `""` to disable all tools"), `--disable-slash-commands` ("Disable all skills"), `--no-chrome`, `--no-session-persistence`, `--system-prompt` (replaces default), `--append-system-prompt`, `--output-format json`. PRD §12.4 listed only 3 bare flags; the proven `commit-claude` uses 5. **Use the fuller set.**

**⚠️ Appendix E.2 (does `--tools ""` suppress tool use?) — BLOCKED on the verification host (2026-07-03):** the real `claude` invocation does NOT complete a usable generation in the suite flow — it runs ~195 s per attempt then yields `ErrRescue` (no usable message), and the `--disallowed-tools "*"` fallback behaves identically (`TestIntegrationReal_ClaudeToolsSuppressed` failed both forms). Root cause is host-side (claude appears to hang on workspace-trust/headless setup rather than emit output), NOT a manifest defect. Decision: **KEEP** the shipped `--tools ""` manifest (the §12.7.2 safe default) and leave E.2 **TO CONFIRM** in an environment where `claude -p ... <stdin` returns a message within the timeout. The `--disallowed-tools "*"` syntax is confirmed to exist (host `--help`) and remains the documented fallback if `--tools ""` proves insufficient once claude is functional.
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

### B.3 gemini — delivery decision carried to integration — E.1 BLOCKED (CLI deprecated on host)
`gemini --help` (v0.19.4) confirms: `-m/--model`, `--approval-mode` (choices `default|auto_edit|yolo`), NO `--system-prompt` (→ prepend to payload), `-p/--prompt` is **DEPRECATED** ("Use the positional prompt instead"), `-o/--output-format` (text|json|stream-json). Help says `-p ... Appended to input on stdin (if any)` ⇒ stdin is read but the exact stdin-without-`-p` behavior is ambiguous without a real run.
- **⚠️ Appendix E.1 (~300 KB stdin vs positional) — BLOCKED on the verification host (2026-07-03):** the installed `gemini` CLI is **DEPRECATED** — it exits **55** with `using Gemini, please migrate to the Antigravity suite of products` AND additionally refuses headless runs (`Gemini CLI is not running in a trusted directory ... use --skip-trust / GEMINI_CLI_TRUST_WORKSPACE=true`). `TestIntegrationReal_GeminiStdinLargePayload` therefore could not exercise stdin and logged the safe default. Decision: **KEEP `prompt_delivery = "positional"`** (the §12.5 / §C.3 default) **+ the 300 KB diff cap (FR3)**, and leave the stdin-vs-positional question **TO CONFIRM** once a functional (non-deprecated, trusted) gemini CLI is available. The deprecation also means the host cannot confirm whether a bare positional run is non-interactive — that too is deferred with this note.
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

### B.4 opencode — VERIFIED, matches PRD §12.6 ✅ (host auth-blocked at runtime)
`opencode run --help` (v1.1.23) confirms: `run [message..]` positional (array), `-m/--model` ("format provider/model"), `--agent`, `--format` (default|json), `--prompt`, no system-prompt flag. `opencode run` is non-interactive and prints final message to stdout. No change to PRD §12.6.

**⚠️ Runtime note (2026-07-03, integration_real):** the manifest renders correctly, but the host's `opencode` exits 1 with `Error: Verify your account to continue ... Status: 403` (account verification required) before producing output — an auth/account state issue, NOT a manifest defect. The manifest is unchanged; end-to-end confirmation is deferred to an authenticated opencode session.
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

### B.5 codex — 🔧 CORRECTED (stdin delivery + `--ephemeral`); `--ask-for-approval` REMOVED (real run)
`codex exec --help` (codex-cli 0.142.4) is decisive:
- `codex exec` (alias `e`) = "Run Codex non-interactively" — it is the documented non-interactive runner that writes the answer to stdout (E.4a target).
- **`[PROMPT]`: "If not provided as an argument (or if `-` is used), instructions are read from stdin."** ⇒ **codex supports stdin.** Switching `prompt_delivery` from the PRD's `positional` to **`stdin`** avoids arg-length limits and is consistent with pi/claude. Pass NO positional (pipe everything via stdin).
- `-m/--model` ✅, `-s/--sandbox` (choices incl. `read-only`) ✅, plus **`--ephemeral`** ("Run without persisting session files to disk") — a bare-mode flag the PRD missed. **Add it.**
- ❌ **`-a/--ask-for-approval` is NOT a `codex exec` flag** (REAL-RUN VERIFIED 2026-07-03, integration_real P1.M8.T3.S1). The earlier §B.5 note conflated top-level `codex --help` (where `-a/--ask-for-approval` lives) with `codex exec --help` (where it does NOT). The real-agent suite proved `codex exec --sandbox read-only --ask-for-approval never --ephemeral` exits **2 "unexpected argument '--ask-for-approval'"** on the *same* codex-cli 0.142.4. **It has been REMOVED from the manifest.** `codex exec` is non-interactive by definition; sandbox read-only + --ephemeral is the correct minimal set (confirmed: the corrected invocation parses and reaches model invocation).
- ⚠️ **E.4a end-to-end (writes answer to stdout + exit 0) is BLOCKED on the verification host**: the corrected `codex exec` reaches the OpenAI model call and fails with **HTTP 401 Unauthorized** ("Missing bearer or basic authentication" — the host has no codex/OpenAI credential). stdin delivery + `--ephemeral` + `--sandbox read-only` are real-run-confirmed VALID; the full stdout/exit-0 confirmation remains **TO CONFIRM** in an authenticated environment.
```toml
name = "codex"; detect = "codex"; command = "codex"; subcommand = ["exec"]
prompt_delivery = "stdin"          # 🔧 CORRECTED from positional: codex exec reads stdin (no arg-limit) — real-run-confirmed valid
print_flag = ""                    # exec is already non-interactive
model_flag = "-m"; default_model = ""   # reads ~/.codex/config.toml
system_prompt_flag = ""            # none → prepend to payload
provider_flag = ""
bare_flags = ["--sandbox","read-only","--ephemeral"]   # --ephemeral 🔧 ADDED; --ask-for-approval ❌ REMOVED (real-run-invalid for `exec`)
output = "raw"; strip_code_fence = true
```
Rendered: `codex exec -m gpt-5 --sandbox read-only --ephemeral  <stdin "<sys>\n\n<diff>\n\n<instruction>">`.

### B.6 cursor (binary `agent`) — matches PRD §12.7 ✅ (E.4b BLOCKED: host not authenticated)
`agent --help` (Cursor Agent, 2026.06.26) confirms: `-p/--print` ("Print responses to console… Has access to all tools, including write and shell"), `--mode` (choices `plan|ask`; **ask = "Q&A style… (read-only)"**), `--trust` ("only works with --print/headless mode"), `--model`, `--output-format` (text|json|stream-json). No system-prompt flag (→ prepend).
- **⚠️ Appendix E.4b (`--mode ask` read-only over `-p`'s full-tools) — BLOCKED on the verification host (2026-07-03):** `agent` is present but **NOT AUTHENTICATED** — it exits 1 with `Error: Authentication required. Please run 'agent login' first, or set CURSOR_API_KEY environment variable.` `TestIntegrationReal_CursorModeAskReadOnly` thus could not run a generation and could not assert the read-only working-tree invariant. Decision: **KEEP** the shipped `--mode ask --trust` manifest (unchanged) and leave E.4b **TO CONFIRM** in an environment where `agent` is logged in (`agent login` / `CURSOR_API_KEY`). The `--mode ask` = read-only semantics remain as documented by `agent --help`; we do NOT set `--force`/`--yolo`.
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

1. **claude** — add `--disable-slash-commands`, `--no-chrome` to `bare_flags` (proven `commit-claude` uses them; current `--help` confirms they exist). [ref_impl.md D4] *(E.2 tool-suppression still TO CONFIRM — host claude not functional; see §B.2.)*
2. **codex** — change `prompt_delivery` `positional`→**`stdin`**; add `--ephemeral` to `bare_flags`. Resolves Appendix E.4a *delivery*. [B.5] **And (real-run-confirmed 2026-07-03): REMOVE `--ask-for-approval never`** — it is not a `codex exec` flag and exits 2. *(E.4a stdout/exit-0 end-to-end still TO CONFIRM — host codex 401-unauthorized; see §B.5.)*
3. **gemini** — keep `positional` (PRD default); carry stdin-vs-positional to integration per E.1. [B.3] *(E.1 still TO CONFIRM — host gemini CLI deprecated/exit-55; see §B.3.)*
4. **cursor** — unchanged from PRD; E.4b strongly indicated resolved (ask=read-only). [B.6] *(E.4b still TO CONFIRM — host `agent` not authenticated; see §B.6.)*

The manifest **schema** (PRD §12.1) and all field names are **unchanged and fixed**. After the 2026-07-03 real run, the ONLY default-value change beyond the original §C.1–§C.4 set is the codex `--ask-for-approval` removal (real-run-confirmed); all four Appendix-E behavioral questions remain TO CONFIRM pending a fully-authenticated, non-deprecated agent environment, with their shipped safe defaults retained (PRD §12.7.2 — never silently assumed).

## Appendix E resolved (real-agent run, 2026-07-03)

The `internal/generate/integration_real_test.go` suite (PRD §20.1 layer 4, `//go:build integration_real` + `STAGEHAND_RUN_REAL=1`) was DELIVERED and RUN on the verification host (all six agents present on `$PATH`). The suite itself is proven working: `pi` completed a full end-to-end real commit through `generate.CommitStaged` + the REAL `*provider.Executor` + REAL git plumbing in ~3–11 s. The Appendix-E outcomes are recorded honestly below — per PRD §12.7.2 a field is flipped ONLY on real-run confirmation, else the safe default is retained with a `# TO CONFIRM`.

| Item | Question | Outcome (2026-07-03) | Manifest action |
|---|---|---|---|
| **E.1** | gemini ~300 KB stdin vs positional | **BLOCKED** — host `gemini` CLI is deprecated (exit 55, "migrate to Antigravity") + headless-untrusted. `TestIntegrationReal_GeminiStdinLargePayload` logged the safe default. | **KEEP positional** + 300 KB cap (FR3). TO CONFIRM. |
| **E.2** | claude `--tools ""` suppresses tool use | **BLOCKED** — host `claude` does not return a usable message in the suite flow (~195 s → `ErrRescue`); `--disallowed-tools "*"` fallback behaves identically. Host-side, not a manifest defect. | **KEEP** `--tools ""`. TO CONFIRM. (`--disallowed-tools "*"` remains the documented fallback.) |
| **E.4a** | `codex exec` writes answer to stdout + exit 0 | **PARTIAL** — real run FOUND + FIXED a manifest bug (`--ask-for-approval` is not a `codex exec` flag → exit 2; REMOVED). Corrected invocation parses + reaches the model call, then **401 Unauthorized** (no codex/OpenAI credential on host). stdin + `--ephemeral` + `--sandbox read-only` real-run-confirmed VALID. | **codex BareFlags: `--ask-for-approval never` REMOVED** (only confirmed change). E.4a end-to-end TO CONFIRM in an authenticated env. |
| **E.4b** | cursor `--mode ask` read-only over `-p` full-tools | **BLOCKED** — host `agent` not authenticated (exit 1, "run 'agent login' first, or set CURSOR_API_KEY"). `TestIntegrationReal_CursorModeAskReadOnly` could not assert the working-tree invariant. | **KEEP** `--mode ask --trust`. TO CONFIRM. |

**Net manifest change from this run:** `internal/provider/builtin.go` codex `BareFlags` drops `--ask-for-approval`,`never` (mirrored in `providers/codex.toml` + `builtin_test.go` + `manifest_test.go`). No other field changed; the schema is unchanged. Every unconfirmed field keeps its shipped safe default + `# TO CONFIRM`.

**To complete the Appendix-E confirmation**, re-run on a host where the agents are functional/authenticated:
```bash
STAGEHAND_RUN_REAL=1 go test -tags integration_real -run '^TestIntegrationReal' -timeout 60m -v ./internal/generate/
```
(Note: Go's `-run` matches the FULL test-function name, so the pattern must be `^TestIntegrationReal`, not `^IntegrationReal` — the latter matches zero tests. This corrects the illustrative pattern in the PRP's §20.1 layer-4 line.)

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
