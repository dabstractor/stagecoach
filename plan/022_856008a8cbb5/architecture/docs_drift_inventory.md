# Docs drift inventory — spec v3.5 (activity-based deadlines)

- **Base:** HEAD `df98a9a6d971345081141060b8ccd1f0f7dd6714` (read-only inventory; no file outside this report was modified).
- **Scope:** user-facing documentation drift only. `docs/packaging.md` is explicitly OUT of scope. `internal/` hits are classified below as context for the implementing task, not as docs work.
- **v3.5 change set this inventory is written against** (from `spec/SPEC.md` revision row): idle deadline added as the primary bound (`idle_timeout`, default 60s, activity-based: stdout/stderr writes + Linux `/proc/<pid>/io` reads); hard-cap `--timeout` default raised 120s→600s (retained, MUST NOT be removed); the v2.8 planner 480s built-in is retired as moot; both deadlines resolve per role exactly like provider/model/reasoning (FR-R7) and bind each invocation individually (fresh idle clock per multi-turn turn / work-description round); idle kill is downstream-identical to today's timeout (exit 124, rescue, no-retry — only the `--verbose` cause line differs, "Deadline exceeded (idle or hard cap)"); new manifest field `event_stream` (pi `--mode json`, claude/cursor/agy `--output-format stream-json`, codex `--json`, opencode `--format json`) gated on an FR-T9-style verification duty.

Per-anchor format: **line → current text (quoted) → required v3.5 change.** Line numbers are as of HEAD `df98a9a`.

---

## 1. `docs/cli.md`

### L27 — `--timeout` global flag row (table "Global flags", L20–L65)

```
| `--provider <name>` | string | "" (auto-detect) | `STAGECOACH_PROVIDER` | `stagecoach.provider` | Provider/agent to use |
| `--model <name>` | string | "" (manifest default) | `STAGECOACH_MODEL` | `stagecoach.model` | Model override. Sets the GLOBAL default — … |
| `--config <path>` | string | "" | `STAGECOACH_CONFIG` | — | Path to a config file, overrides discovery. … |
| `--timeout <dur>` | string | "120s" | `STAGECOACH_TIMEOUT` | `stagecoach.timeout` | Generation timeout (e.g. `"120s"` or `120`) |
| `--verbose`, `-v` | bool | false | `STAGECOACH_VERBOSE` | — | Print resolved command, raw output, retries … |
```

**Required change:** `--timeout` is now the **hard cap**; Default column `"120s"` → `"600s"`; description reworded to "Hard-cap generation timeout (backstop for the idle deadline; e.g. `"600s"` or `600`)". Immediately after this row, add the new global row `--idle-timeout <dur>` (default `60s`, description: idle deadline — kills on N seconds with no observable byte activity from the agent's process group; env/git-config/config-file homes per the timeout layer pattern; exact key names follow the implementing PRP).

### L60–63 — per-role timeout flag rows (end of the Global flags table)

```
| `--reasoning <level>` | string | "" (off) | `STAGECOACH_REASONING` | `stagecoach.reasoning` | Global reasoning effort: off\|low\|medium\|high. … |
| `--planner-reasoning <level>` | string | "" | `STAGECOACH_PLANNER_REASONING` | — | Per-role reasoning for the planner |
| `--stager-reasoning <level>` | string | "" | `STAGECOACH_STAGER_REASONING` | — | Per-role reasoning for the stager |
| `--message-reasoning <level>` | string | "" | `STAGECOACH_MESSAGE_REASONING` | — | Per-role reasoning for the message composer |
| `--arbiter-reasoning <level>` | string | "" | `STAGECOACH_ARBITER_REASONING` | — | Per-role reasoning for the arbiter |
| `--planner-timeout <dur>` | string | "" | `STAGECOACH_PLANNER_TIMEOUT` | — | Per-role generation timeout for the planner (e.g. `"600s"` or `600`) |
| `--stager-timeout <dur>` | string | "" | `STAGECOACH_STAGER_TIMEOUT` | — | Per-role generation timeout for the (tooled) staging agent (e.g. `"300s"` or `300`) |
| `--message-timeout <dur>` | string | "" | `STAGECOACH_MESSAGE_TIMEOUT` | — | Per-role generation timeout for the message composer (e.g. `"120s"` or `120`) |
| `--arbiter-timeout <dur>` | string | "" | `STAGECOACH_ARBITER_TIMEOUT` | — | Per-role generation timeout for the leftover arbiter (e.g. `"120s"` or `120`) |
| `--version` | — | — | — | — | Print the build version … |
```

**Required change:** reword the four `--<role>-timeout` rows to "Per-role hard-cap generation timeout…" (each role now resolves an idle deadline **and** a hard cap like provider/model/reasoning — FR-R7; no planner special-case anymore). The `"120s"` example values in L62/L63 are arbitrary durations and may stay, but should stop implying the default. Directly after L63, add four per-role idle rows `--<role>-idle-timeout <dur>` (planner/stager/message/arbiter, mirroring the timeout rows' column shape: Flag | Type | Default | Env var | Git config | Description).

### L452 — exit-code table row (section "## Exit codes", table L447–452)

```
| `2` | Nothing to commit (clean tree after auto-stage, or nothing staged with `--no-auto-stage`). |
| `3` | Rescue condition (snapshot taken, commit not created — manual recovery printed). |
| `5` | Busy — another stagecoach run holds the per-repo lock; retry after it finishes. |
| `6` | Update available (`stagecoach upgrade --check` found a newer release); upgrade-path only — never returned by the commit path. |
| `124` | Timeout (generation exceeded `--timeout`). |
```

**Required change:** L452 → `| `124` | Deadline exceeded (idle or hard cap) — generation exceeded the idle deadline (`--idle-timeout`, default 60s of silence) or the hard cap (`--timeout`, default 600s). |` — exit code, rescue, and no-retry semantics are unchanged. The following prose (L454: "A timeout is reported as `124` (matching GNU `timeout`), not `3`…") stays valid but should say "A deadline kill (idle or hard cap) is reported as `124`…".

### Flag-table structure (home for the new rows)

- **Global flags table (L23–L65):** header `| Flag | Type | Default | Env var | Git config | Description |`. New `--idle-timeout` row slots immediately after the `--timeout` row (L27); per-role idle rows slot immediately after L63 (planner/stager/message/arbiter order, matching the existing per-role timeout ordering).
- **"Flag ↔ env ↔ git-config map" table (L462–L500):** current timeout rows are `| `--timeout` | `STAGECOACH_TIMEOUT` | `stagecoach.timeout` |` (L468) and the four per-role rows `| `--<role>-timeout` | `STAGECOACH_<ROLE>_TIMEOUT` | `stagecoach.role.<role>.timeout` |` (L496–499). Add the `--idle-timeout` row after L468 and per-role idle rows after L499 using the same three-column shape (exact env/git-config key names per the implementing PRP).

---

## 2. `docs/configuration.md`

Section titles around the anchors: `## File format` (L78, holds the commented template), `## Built-in defaults` (L131, holds the defaults table), `## Environment variables` (L188, env table), `## Git-config keys` (L225, ini example + key table + NOTE), `### Decompose config keys` (L256).

### L91 — `[defaults]` template comment (in `## File format`)

```
[defaults]
provider = "claude"
reasoning = "off"   # off|low|medium|high; off by default for every role
# model          = ""
# timeout        = "120s"   # global fallback for every role; the planner role defaults to 480s
# auto_stage_all = true
# verbose        = false
```

**Required change:** comment value `"120s"` → `"600s"` with wording "global hard-cap fallback for every role; every role also resolves a 60s idle deadline". Delete the "the planner role defaults to 480s" clause (built-in retired). Add a commented `# idle_timeout   = "60s"` line directly below, mirroring the `timeout` line's style.

### L99 — `[role.planner]` template comment (in `## File format`)

```
# --- per-role models for the default provider "claude" ---

# [role.planner]
# model = "haiku"
# timeout = "600s"   # per-role generation timeout; overrides the planner's 480s built-in
# [role.stager]
# model = "sonnet"
# [role.message]
# model = "haiku"
# [role.arbiter]
# model = "haiku"
```

**Required change:** L99 comment → "per-role hard-cap timeout; the planner has no special built-in anymore (roles resolve idle+hard-cap exactly like model/provider)". Optionally add a commented per-role idle example (`# idle_timeout = "90s"`).

### L138 — built-in defaults table row (in `## Built-in defaults`)

```
| Option | Default | Source |
|--------|---------|--------|
| `provider` | `""` (auto-detect) | `config.Defaults` |
| `model` | `""` (manifest `default_model`) | `config.Defaults` |
| `timeout` | `120s` | `config.Defaults` |
| `auto_stage_all` | `true` | `config.Defaults` |
| `verbose` | `false` | `config.Defaults` |
```

**Required change:** L138 → `| `timeout` | `600s` (hard cap) | `config.Defaults` |` and insert a new row right after it: `| `idle_timeout` | `60s` | `config.Defaults` |`.

### L193 — `STAGECOACH_TIMEOUT` env row (in `## Environment variables`)

```
| `STAGECOACH_PROVIDER` | `--provider` | Default provider/agent | `STAGECOACH_PROVIDER=claude stagecoach` |
| `STAGECOACH_MODEL` | `--model` | Model override | `STAGECOACH_MODEL=sonnet stagecoach` |
| `STAGECOACH_TIMEOUT` | `--timeout` | Global generation timeout — the fallback for every role; the planner role defaults to 480s and is NOT changed by this | `STAGECOACH_TIMEOUT=60s stagecoach` |
| `STAGECOACH_CONFIG` | `--config` | Config file path | `STAGECOACH_CONFIG=./alt.toml stagecoach` |
```

**Required change:** delete the "the planner role defaults to 480s and is NOT changed by this" clause (planner special-case retired — the global now IS the fallback for every role); reword to "Global hard-cap generation timeout — the fallback for every role; every role also resolves a 60s idle deadline (`STAGECOACH_IDLE_TIMEOUT`)". Example value `60s` currently collides with the new idle default — change the example (e.g. `STAGECOACH_TIMEOUT=600s stagecoach`). Add a `STAGECOACH_IDLE_TIMEOUT` row mirroring this one.

### L212–215 — per-role timeout env rows (in `## Environment variables`)

```
| `STAGECOACH_PLANNER_TIMEOUT` | `--planner-timeout` | Per-role: planner generation timeout (; planner built-in 480s) | `STAGECOACH_PLANNER_TIMEOUT=600s stagecoach` |
| `STAGECOACH_STAGER_TIMEOUT` | `--stager-timeout` | Per-role: stager generation timeout (; inherits 120s) | `STAGECOACH_STAGER_TIMEOUT=300s stagecoach` |
| `STAGECOACH_MESSAGE_TIMEOUT` | `--message-timeout` | Per-role: message generation timeout (; the single-commit path's only role; inherits 120s) | `STAGECOACH_MESSAGE_TIMEOUT=120s stagecoach` |
| `STAGECOACH_ARBITER_TIMEOUT` | `--arbiter-timeout` | Per-role: arbiter generation timeout (; inherits 120s) | `STAGECOACH_ARBITER_TIMEOUT=120s stagecoach` |
```

**Required change:** L212 drop "(; planner built-in 480s)"; L213–215 drop "(; inherits 120s)" / change to "(; inherits the 600s global hard cap)". Keep the "single-commit path's only role" note on L214. Add four per-role idle rows `STAGECOACH_<ROLE>_IDLE_TIMEOUT` directly below (exact env names per the implementing PRP), noting each binds per invocation (fresh idle clock per multi-turn turn / work-description round).

### L232 — git-config ini example (in `## Git-config keys`)

```
```ini
[stagecoach]
    provider = pi
    model = claude-haiku
    timeout = 120s
    autoStageAll = true
```
```

**Required change:** example mirrors the default → `timeout = 600s`; add an `idleTimeout = 60s` line beneath it (exact git-config key spelling per the implementing PRP — the section's key table is the naming authority).

### L240–241 — git-config key table rows (in `## Git-config keys`)

```
| `stagecoach.provider` | string | `git config --get stagecoach.provider` | Default provider |
| `stagecoach.model` | string | `git config --get stagecoach.model` | Model override |
| `stagecoach.timeout` | string | `git config --get stagecoach.timeout` | Generation timeout (duration string) |
| `stagecoach.role.<role>.timeout` | string | `git config --get stagecoach.role.<role>.timeout` | Per-role generation timeout; `<role>` ∈ `planner`\|`stager`\|`message`\|`arbiter`. Duration string (`"600s"` or bare `600`); unset ⇒ inherit global `stagecoach.timeout`. |
| `stagecoach.autoStageAll` | bool | `git config --get --bool stagecoach.autoStageAll` | Auto-stage all when nothing staged |
```

**Required change:** L240 description → "Hard-cap generation timeout (duration string; default 600s)"; add a `stagecoach.idleTimeout` row after it. L241: the `unset ⇒ inherit global stagecoach.timeout` logic survives but the row should note the planner no longer has a 480s built-in exception; add a parallel `stagecoach.role.<role>.idleTimeout` row (exact key names per the implementing PRP).

### L254 — NOTE + surrounding section titles (`## Git-config keys` NOTE → `### Decompose config keys`)

```
> [!NOTE]
> The git-config layer has per-role **timeout** keys (`stagecoach.role.<role>.timeout`), but **no** per-role provider/model/reasoning keys — those are CLI flags (`--planner-provider`, etc.), env vars (`STAGECOACH_PLANNER_*`), and config-file `[role.*]` blocks only. There is no `stagecoach.commits` and no `stagecoach.max_commits` (decompose settings `--commits`/`--single`/`--no-decompose` are flag/env only; `--max-commits` also reads from the `[generation]` config-file section). There is also no `stagecoach.exclude` git-config key and no `STAGECOACH_EXCLUDE` env var (deliberate — see [Exclusion globs](#exclusion-globs-generationexclude) below); exclusions are config-file + `--exclude`/`-x` only.

### Decompose config keys
```

**Required change:** extend the NOTE's first sentence to cover the per-role **idle** keys alongside the timeout keys ("per-role **timeout** and **idle-timeout** keys (`stagecoach.role.<role>.timeout` / `stagecoach.role.<role>.idleTimeout`)…"). Immediately below, in the `### Decompose config keys` prose paragraph at **L264** (an rg hit, see §6): "Each role also resolves its **own generation timeout**: the **planner defaults to 480s** (the heavy role that reasons over the full frozen diff), while **stager/message/arbiter inherit the global `120s`**… Note the global `--timeout` does **not** change the planner — it has a 480s built-in that wins; set `--planner-timeout` to override it." → rewrite with no planner special-case: every role resolves an idle deadline (default 60s) and a hard cap (default 600s) via the same flag > env > `[role.<role>]` > `[defaults]` chain; the global now applies to the planner too; the precedence sentence drops the `built-in` tier.

---

## 3. `docs/how-it-works.md`

### L173 — per-role timeout sentence (end of `### Safety`, before `### Diff capture pipeline`)

```
- **Start-of-run freeze** — T_start captures the full working-tree change set at decompose activation; concurrent edits never enter any commit. …

See [configuration.md](configuration.md) for per-role provider/model/reasoning/**timeout** configuration and [cli.md](cli.md) for the decompose and per-role flags. Each role resolves its own generation timeout: the **planner defaults to 480s** (the heavy role), while stager/message/arbiter inherit the global `120s`; override with `--<role>-timeout`.
```

**Required change:** rewrite the second sentence: each role resolves its own **idle deadline (default 60s, kills on silence)** and **hard cap (default 600s)** — like provider/model/reasoning, no planner special-case; override with `--<role>-timeout` / `--<role>-idle-timeout`. Optionally add the design rationale (activity-based kill; hard cap is the false-alive backstop).

### L245 — failure-modes table row (in `### Failure modes and exit codes`)

```
| Unresolved merge conflicts in the index | 1 (Error) | Resolve the conflicts, then re-run `stagecoach` (caught before the snapshot) |
| Generation failed (parse/retry exhaustion) | 3 (Rescue) | Rescue message with tree SHA |
| Generation timed out | 124 (Timeout) | Rescue message with tree SHA |
| CAS failure (HEAD moved meanwhile) | 1 (Error) | HEAD-moved message |
| Nothing to commit (clean tree) | 2 (NothingToCommit) | Stage files and retry |
```

**Required change:** L245 trigger → "Generation deadline exceeded (idle or hard cap)" with exit `124 (Timeout)` and the same rescue outcome — semantics unchanged, only the trigger wording and the idle-vs-hard-cap cause line (`--verbose`) are new.

### L346 — multi-turn fallback per-turn timeout (in `## Multi-turn generation fallback`)

```
  normal parse + duplicate-rejection pipeline, then commits like any other message.

Each turn is a separate provider invocation bounded by the **message role's** resolved timeout (/); total wall-clock ≈ `message-timeout × (N+1)`,
surfaced on the progress line at fallback time. That progress line also reports the per-chunk token budget each chunk targets; with `--verbose`, each turn additionally prints its payload size and raw agent output.

**Failure handling.** If any turn errors, times out, or the final output fails to parse/dedupe, the
```

**Required change:** "resolved timeout" → "resolved deadlines (idle + hard cap), each binding the invocation individually — a fresh idle clock per turn"; the wall-clock approximation becomes `hard-cap × (N+1)` as the outer bound with the idle deadline as the per-turn silence bound. "times out" in the Failure-handling sentence → "hits a deadline (idle or hard cap)".

---

## 4. `README.md`

### L252 — per-role config example (in the examples block ending L253)

```
# Route planning to a bigger model (per-repo .stagecoach.toml):
# [role.planner]
# provider = "claude"
# model = "opus"
# timeout = "600s"   # per-role generation timeout; the planner defaults to 480s
```

**Required change:** drop "; the planner defaults to 480s" (built-in retired) → `# timeout = "600s"   # per-role hard-cap timeout; roles also resolve a 60s idle deadline`.

### L310 — exit-code prose (after the `--push` example)

```
stagecoach --push
```

Exit codes are machine-readable, so a workflow can branch on them: `0` success; `1` generation/config/usage failure; `2` nothing staged; `3` rescue (snapshot armed, nothing committed; recovery printed); `5` busy, another run holds the lock, retry; `124` timeout. See the [full exit-code table](docs/cli.md#exit-codes).
```

**Required change:** `124` description → "deadline exceeded (idle or hard cap)". The code itself and the link are unchanged.

---

## 5. `docs/providers.md`

### Manifest-field table (section `## The schema`, L13–L38)

```
Each manifest has 22 fields (matching the TOML tags in `internal/provider/manifest.go`):

| Field | Type | Default | Purpose |
|-------|------|---------|---------|
| `name` | string | (required) | Provider identity; set from the `[provider.<name>]` table key. |
…
| `session_mode` | string | `""` | Multi-turn fallback capability. `""` (default) = the provider cannot append turns across one-shot calls → multi-turn unavailable; `"append"` = re-invoking the same session id appends a recallable turn. **Only pi ships `"append"`** (VERIFIED 2026-07-05). Requires a verified, reproducible append-turn rendering — see below. |
| `bare_flags` | list of string | `[]` (none) | Extra flags appended verbatim before `print_flag` in bare mode. |
```

**Required change:** the count sentence "22 fields" → "23 fields" (v3.5 adds `event_stream`), and insert a new row next to `session_mode` (L29), e.g. `| `event_stream` | string | `""` | Event-stream output capability for idle detection off Linux and in-stream error surfacing. Values declare the provider's verified JSON-event flags (pi `--mode json`, claude/cursor/agy `--output-format stream-json`, codex `--json`, opencode `--format json`). `""` (default) = no event stream; idle detection falls back to byte-activity polling. See below. |` — final row wording per the implementing PRP.

### `session_mode` capability-section precedent (L40–49, `### Multi-turn capability (\`session_mode\`)`)

```
### Multi-turn capability (`session_mode`)

A provider supports Stagecoach's **lossless multi-turn fallback** (— used when a one-shot generation repeatedly fails on a diff too large for a single reliable request) if and only if re-invoking the SAME session id appends a turn the model can recall. The `session_mode` manifest field declares this:

- `"append"` — re-invoking the same session id appends a recallable turn (multi-turn available).
- `""` (default) — the provider cannot append turns across one-shot calls (multi-turn unavailable; the run proceeds one-shot → rescue, unchanged).

**Only `pi` ships `session_mode = "append"` today** — VERIFIED 2026-07-05 via a live run (`pi --session-id X <isolation-flags-minus-no-session> -p "remember BANANA"`, then a same-`--session-id` recall turn returning "BANANA"). Every other built-in (claude, opencode, codex, cursor, agy) ships `""`.

** verification bar.** A manifest MUST NOT declare `"append"` speculatively. Setting it requires a verified, reproducible append-turn rendering — the exact flag set confirmed per provider … Until a provider's append mechanism is verified, its `session_mode` stays `""` and multi-turn is silently skipped for it.
```

**Required change (new section to mirror):** add a sibling capability section — `### Event-stream output (\`event_stream\`)` — directly after the `session_mode` section (i.e. before `## Command rendering` at L51), copying this precedent's structure: (1) what the field declares and why (portable idle detection off Linux — procfs is Linux-only — plus in-stream error surfacing that today appears as opaque empty-stdout parse failures), (2) the value list with which built-ins ship what, (3) a **verification bar** paragraph modeled on the `session_mode` one: no speculative declarations; the FR-T9-style verification duty applies; the `json`-vs-`stream-json` buffered/progressive trap and heartbeat-only streams are the recorded pitfalls to document.

---

## 6. Full `rg -n '480s|120s' README.md docs/ internal/` classification — 89 hits

Command as specified truncates at `head -60`; all 89 hits are classified below (67 MUST-CHANGE, 22 UNRELATED). "MUST-CHANGE" = the line's text encodes v3.4 deadline semantics (old 120s default, planner 480s built-in, or elapsed-timeout-only wording) and drifts under v3.5. "UNRELATED" = the `120s`/`480s` string is an arbitrary duration-format example or a user-supplied fixture value with no default/built-in meaning.

### User-facing docs (15 — all MUST-CHANGE)

| Hit | Class | Reason (one line) |
|-----|-------|-------------------|
| README.md:252 | MUST-CHANGE | Example comment cites the retired planner 480s default. |
| docs/how-it-works.md:173 | MUST-CHANGE | States planner 480s built-in + 120s global inheritance. |
| docs/configuration.md:91 | MUST-CHANGE | Template comment: 120s global default + planner 480s clause. |
| docs/configuration.md:99 | MUST-CHANGE | Template comment cites "the planner's 480s built-in". |
| docs/configuration.md:138 | MUST-CHANGE | Built-in defaults table: `timeout` 120s (now 600s; idle row missing). |
| docs/configuration.md:193 | MUST-CHANGE | Env row: "planner role defaults to 480s and is NOT changed by this". |
| docs/configuration.md:212 | MUST-CHANGE | Env row: "(; planner built-in 480s)". |
| docs/configuration.md:213 | MUST-CHANGE | Env row: "(; inherits 120s)" (now 600s hard cap). |
| docs/configuration.md:214 | MUST-CHANGE | Env row: "(; … inherits 120s)". |
| docs/configuration.md:215 | MUST-CHANGE | Env row: "(; inherits 120s)". |
| docs/configuration.md:232 | MUST-CHANGE | Git-config ini example mirrors the 120s default; idleTimeout home. |
| docs/configuration.md:264 | MUST-CHANGE | Decompose prose: planner 480s / 120s inheritance / built-in precedence tier. |
| docs/cli.md:27 | MUST-CHANGE | Flag row default "120s" (hard cap now 600s; idle row missing). |
| docs/cli.md:62 | MUST-CHANGE | `--message-timeout` row: reword for idle/hard-cap split ("120s" example itself arbitrary). |
| docs/cli.md:63 | MUST-CHANGE | `--arbiter-timeout` row: same reword ("120s" example itself arbitrary). |

### `internal/` — code + tests (74 hits: 52 MUST-CHANGE, 22 UNRELATED)

Context for the implementing task; classified with the same rule.

**MUST-CHANGE (52):**

| Hit | Class | Reason (one line) |
|-----|-------|-------------------|
| internal/config/roles.go:6 | MUST-CHANGE | Comment: "planner is the ONLY role with a built-in timeout (480s)". |
| internal/config/roles.go:74 | MUST-CHANGE | Precedence comment: "built-in role default (planner = 480s …)". |
| internal/config/roles.go:75 | MUST-CHANGE | Precedence comment: "[defaults].timeout … the global; 120s". |
| internal/config/roles.go:81 | MUST-CHANGE | Comment: "ONLY role with a shipped built-in timeout (480s)". |
| internal/config/roles.go:84 | MUST-CHANGE | Comment: "--planner-timeout 600s beats the 480s built-in". |
| internal/config/roles.go:85 | MUST-CHANGE | Comment: "planner → 480s built-in; stager/message/arbiter → cfg.Timeout". |
| internal/config/roles_test.go:199 | MUST-CHANGE | Test setup comment: "distinct from the 480s built-in". |
| internal/config/roles_test.go:203 | MUST-CHANGE | Assertion text: "per-role beats built-in 480s AND global 120s". |
| internal/config/roles_test.go:209 | MUST-CHANGE | Test setup comment: "DISTINCT from the 480s built-in". |
| internal/config/roles_test.go:210 | MUST-CHANGE | Comment: "planner must take its 480s BUILT-IN, NOT the 120s global". |
| internal/config/roles_test.go:213 | MUST-CHANGE | Assertion: "want 480s (built-in beats 120s global)". |
| internal/config/roles_test.go:219 | MUST-CHANGE | Setup comment: "distinct from the 480s planner built-in". |
| internal/config/roles_test.go:223 | MUST-CHANGE | Assertion: "want 120s [no built-in ⇒ global]". |
| internal/config/roles_test.go:234 | MUST-CHANGE | Assertion: "want 120s [Timeout 0 inherits global…]". |
| internal/config/roles_test.go:243 | MUST-CHANGE | Assertion: "want 120s [unknown role ⇒ global, no built-in]". |
| internal/config/roles_test.go:252 | MUST-CHANGE | Assertion: "want 120s [Roles nil ⇒ global fallback]". |
| internal/config/roles_test.go:267 | MUST-CHANGE | Assertion: "beats 120s global". |
| internal/config/roles_test.go:274 | MUST-CHANGE | Comment: "planner → per-role override (beats the 480s built-in)…". |
| internal/config/roles_test.go:275 | MUST-CHANGE | Comment: "(beats the 120s global)". |
| internal/config/roles_test.go:279 | MUST-CHANGE | Setup comment: "distinct from 480s so the planner built-in is unambiguous". |
| internal/config/roles_test.go:281 | MUST-CHANGE | Fixture comment: "override beats the 480s built-in". |
| internal/config/roles_test.go:282 | MUST-CHANGE | Fixture comment: "override beats the 120s global". |
| internal/config/roles_test.go:283 | MUST-CHANGE | Fixture comment: "no built-in ⇒ 120s global". |
| internal/config/roles_test.go:300 | MUST-CHANGE | Comment narrates "the global 480s→120s" history. |
| internal/config/roles_test.go:301 | MUST-CHANGE | Comment: "Defaults()+set Timeout=120s — identical, since Defaults().Timeout IS 120s". |
| internal/config/roles_test.go:302 | MUST-CHANGE | Comment: "makes 'the planner still gets 480s…' a named assertion". |
| internal/config/roles_test.go:303 | MUST-CHANGE | Comment: same 480s-built-in narrative. |
| internal/config/roles_test.go:307 | MUST-CHANGE | Assertion: "want 120s (the FR-R7 global default)". |
| internal/config/roles_test.go:310 | MUST-CHANGE | Assertion: "want 480s (built-in beats 120s global)". |
| internal/decompose/planner.go:66 | MUST-CHANGE | Comment: "planner resolves its OWN timeout — the built-in 480s default". |
| internal/decompose/planner.go:67 | MUST-CHANGE | Comment: "longer than the 120s global". |
| internal/decompose/planner_test.go:345 | MUST-CHANGE | Test comment: "the planner now resolves its OWN timeout (the 480s built-in…". |
| internal/decompose/planner_test.go:346 | MUST-CHANGE | Test comment: "longer than the 120s global". |
| internal/generate/realagent_test.go:62 | MUST-CHANGE | Comment: "inherit config.Defaults() (120s/3)". |
| internal/generate/realagent_test.go:136 | MUST-CHANGE | Comment: "Timeout per attempt = cfg.Timeout (120s)". |
| internal/cmd/models.go:144 | MUST-CHANGE | Comment: "FR25 knob; default 120s". |
| internal/cmd/root_test.go:324 | MUST-CHANGE | Assertion: "want 120s (default)". |
| internal/cmd/root.go:169 | MUST-CHANGE | User-visible `--timeout` flag help: "default 120s". |
| internal/cmd/root.go:264 | MUST-CHANGE | `--message-timeout` help string; reword for idle/hard-cap split ("120s" example arbitrary). |
| internal/cmd/root.go:266 | MUST-CHANGE | `--arbiter-timeout` help string; same reword. |
| internal/provider/executor.go:49 | MUST-CHANGE | Comment: "Config.Timeout default is 120s (PRD FR25)" — elapsed-timeout ctx becomes the hard cap. |
| internal/config/config_test.go:21 | MUST-CHANGE | Assertion: "Timeout = %v, want 120s". |
| internal/config/load_test.go:713 | MUST-CHANGE | Assertion: "want 120s (default)". |
| internal/config/bootstrap.go:174 | MUST-CHANGE | User-facing config-init template writes `# timeout = "120s"`; idle_timeout home. |
| internal/config/bootstrap.go:359 | MUST-CHANGE | User-facing env help: `STAGECOACH_TIMEOUT … e.g. "120s" or 120 (seconds)`. |
| internal/config/bootstrap.go:375 | MUST-CHANGE | User-facing example: `git config stagecoach.timeout 120s`. |
| internal/config/config.go:85 | MUST-CHANGE | Field comment: "Defaults: 120s". |
| internal/config/config.go:207 | MUST-CHANGE | Doc comment: "timeout 120s". |
| internal/cmd/config.go:649 | MUST-CHANGE | User-facing env help block: `STAGECOACH_TIMEOUT generation timeout, e.g. "120s" or 120`. |
| internal/cmd/config.go:675 | MUST-CHANGE | User-facing example: `git config stagecoach.timeout 120s`. |
| internal/cmd/config.go:694 | MUST-CHANGE | User-facing template line: `# timeout = "120s"  # generation timeout …`. |
| internal/config/file_test.go:113 | MUST-CHANGE | Comment documents the Layer-1 baseline "Timeout=120s". |

**UNRELATED (22):**

| Hit | Class | Reason (one line) |
|-----|-------|-------------------|
| internal/config/load_test.go:104 | UNRELATED | parseTimeout format-table fixture (`"120s"` as an arbitrary duration literal). |
| internal/config/load_test.go:335 | UNRELATED | User-set planner fixture value (TOML field-merge test) — value not a default. |
| internal/config/load_test.go:370 | UNRELATED | User-set env fixture proving duration-form parsing. |
| internal/config/load_test.go:378 | UNRELATED | Assertion of a user-set fixture value, not a built-in. |
| internal/config/load_test.go:1148 | UNRELATED | User-set env fixture ("env layer 5" precedence test). |
| internal/config/load.go:318 | UNRELATED | Duration-format comment; "480s" is an arbitrary example. |
| internal/config/load.go:673 | UNRELATED | parseTimeout doc comment; "120s"/"2m" are format examples. |
| internal/config/load.go:683 | UNRELATED | parseTimeout error message; format example only. |
| internal/config/config.go:65 | UNRELATED | TOML string-duration format comment; example only. |
| internal/config/file.go:28 | UNRELATED | Duration-format comment; "480s" example. |
| internal/config/file.go:41 | UNRELATED | Format comment on string Timeout; example only. |
| internal/config/file.go:56 | UNRELATED | Format comment; "120s" example. |
| internal/config/file.go:339 | UNRELATED | parseTimeout format comment ("480s" OR bare "480"). |
| internal/config/file.go:348 | UNRELATED | parseTimeout format comment. |
| internal/config/file_test.go:577 | UNRELATED | User-set planner fixture ("480s" as a user value). |
| internal/config/file_test.go:585 | UNRELATED | Assertion of a user-set fixture value. |
| internal/config/file_test.go:707 | UNRELATED | Higher-layer-wins fixture; user-set value. |
| internal/config/file_test.go:732 | UNRELATED | User-set arbiter fixture ("…120s"). |
| internal/config/file_test.go:1281 | UNRELATED | Parse-format coverage comment. |
| internal/config/file_test.go:1293 | UNRELATED | Parse-format table row (`"480s"` → 480s). |
| internal/config/file_test.go:1337 | UNRELATED | User-set fixture. |
| internal/config/file_test.go:1345 | UNRELATED | Assertion of a user-set fixture value. |

---

## 7. `rg -n 'idle|event_stream' README.md docs/` — zero hits

The search returns no matches (exit 1) across `README.md` and all of `docs/` — nothing already mentions `idle` or `event_stream`, including in the out-of-scope `docs/packaging.md` (which the search covered; it contributed no hits). Every v3.5 doc surface above is new text, not rewording of existing idle/event-stream mentions.

## 8. Out of scope / notes for the implementer

- `docs/packaging.md` is explicitly OUT of scope per the task brief.
- Exact flag/env/git-config/config-file key names for the idle deadline (`--idle-timeout`, `STAGECOACH_IDLE_TIMEOUT`, `stagecoach.idleTimeout`, `stagecoach.role.<role>.idleTimeout`, `[defaults].idle_timeout`, `[role.<role>].idle_timeout`) follow the existing timeout layer pattern shown above; the implementing PRP is the naming authority — the homes for each row are identified per anchor.
- The v3.5 spec states idle kills keep exit 124 + rescue + no-retry semantics; only the `--verbose` cause line ("Deadline exceeded (idle or hard cap)") is new — docs changes above are wording/defaults/rows, not behavioral rewrites.
- `internal/` MUST-CHANGE hits (52) are drift context for the implementing task — they are code comments, user-visible help/template strings, and tests asserting the retired 480s/120s semantics; they are not part of this docs inventory's edit scope.
