# Research: codex + cursor built-in manifests (P1.M2.T2.S3)

> Scope: the LAST two §12.7 "read-only constraint" providers added to `internal/provider/builtin.go`.
> Both flag surfaces are VERIFIED live against real `--help` (external_deps.md, 2026-06-29). This subtask
> resolves the ONE residual discrepancy the PRD carried (codex `--ask-for-approval`) and encodes two
> `# TO CONFIRM` notes inline per the work-item contract.

## 1. Authoritative sources

| Source | What it gives | Authority |
|---|---|---|
| `plan/001_f1f80943ac34/architecture/external_deps.md` §codex + §cursor | live `--help` captures; §codex flags a DISCREPANCY + a recommended-revision manifest | PRIMARY — verified flag surfaces |
| `PRD.md` §12.7 (h3.43) | the TOML blocks for codex + cursor (the decode-parity base) | PRIMARY — schema/field values |
| `PRD.md` §12.7.1 (h4.0) | framing: codex/cursor are "read-only constraint" providers | design intent |
| `PRD.md` Appendix E (h2.28) item 4 | the two residual `# TO CONFIRM` items (codex stdout; cursor ask-wins) | carries the open questions |
| Work-item contract (item_description) | REVISIONS: codex prompt_delivery=stdin, bare_flags=[--sandbox, read-only, --ephemeral]; cursor detect=agent | binding decision |

The work item (item_description) is the binding contract where it differs from PRD §12.7 — and it is
consistent with external_deps.md's recommended-revision manifest. NO external/online research is needed:
every flag was captured live and is recorded. (See §7 for the few URLs if a future agent wants context.)

## 2. codex — field-by-field table (with the TWO revisions)

```toml
# PRD §12.7 codex TOML — REVISED per work-item + external_deps.md §codex.
name = "codex"
detect = "codex"
command = "codex"
subcommand = ["exec"]                 # `codex exec` = non-interactive runner (alias `e`)
prompt_delivery = "stdin"             # ← REVISED (was "positional"); codex exec reads stdin via "-"
print_flag = ""                       # exec is already non-interactive; explicit empty → NON-NIL
model_flag = "-m"
default_model = ""                    # codex reads ~/.codex/config.toml; explicit empty → NON-NIL
system_prompt_flag = ""               # NO sys flag → prepend; explicit empty → NON-NIL
provider_flag = ""                    # explicit empty → NON-NIL
bare_flags = ["--sandbox", "read-only", "--ephemeral"]   # ← REVISED (dropped --ask-for-approval; +--ephemeral)
output = "raw"
strip_code_fence = true
```

| field | value | nil/non-nil | note |
|---|---|---|---|
| Name | "codex" | plain (always set) | |
| Detect | "codex" | non-nil | == Name |
| Command | "codex" | non-nil | == Name |
| Subcommand | `["exec"]` | NON-NIL 1-element | §12.7 writes `subcommand = ["exec"]` |
| PromptDelivery | "stdin" | non-nil | **REVISED** (was positional) — §3 |
| PromptFlag | — | **nil (ABSENT)** | §12.7 has no key |
| PrintFlag | `""` | **NON-NIL empty** | §12.7 explicit `""` |
| ModelFlag | "-m" | non-nil | |
| DefaultModel | `""` | **NON-NIL empty** | §12.7 explicit `""` (config.toml holds model) |
| SystemPromptFlag | `""` | **NON-NIL empty** | §12.7 explicit `""` → prepend |
| ProviderFlag | `""` | **NON-NIL empty** | §12.7 explicit `""` |
| DefaultProvider | — | **nil (ABSENT)** | §12.7 omits key |
| BareFlags | `["--sandbox","read-only","--ephemeral"]` | NON-NIL 3-element | **REVISED** — §4 |
| Output | "raw" | non-nil | |
| JsonField | — | **nil (ABSENT)** | |
| StripCodeFence | true | non-nil | |
| RetryInstruction | — | **nil (ABSENT)** | |
| Env | — | **nil (ABSENT)** | |

## 3. codex REVISION #1 — prompt_delivery: positional → "stdin"

- §12.7 TOML says `"positional"`. external_deps.md §codex (BONUS FINDING): `codex exec --help` says
  *"If not provided as an argument (or if `-` is used), instructions are read from stdin."* → stdin is
  viable AND avoids arg-length limits on ~300 KB diffs (same rationale as gemini in S2).
- Work item: `prompt_delivery=stdin (revised — codex exec reads stdin with '-')`. **BINDING.**
- Effect on render (§12.2): payload is piped to stdin, NOT in argv. (The literal `-` token that codex
  needs is a renderer/executor concern — P1.M2.T4/T5 will emit it; `renderArgs` models stdin as
  "payload not in argv", which is the right shape. This subtask encodes the DATA; the `-` is downstream.)

## 4. codex REVISION #2 — bare_flags: drop --ask-for-approval; add --ephemeral

- §12.7 TOML: `["--sandbox","read-only","--ask-for-approval","never"]`.
- external_deps.md §codex DISCREPANCY: **`--ask-for-approval` is NOT a `codex exec` flag** — it only
  exists on the interactive `codex` command. `codex exec` is ALREADY non-interactive ("Run Codex
  non-interactively"), so it never blocks on approval anyway. Passing an unknown flag risks rejection.
- Resolution (work-item + external_deps.md "Preferred"): DROP `--ask-for-approval never`. KEEP
  `--sandbox read-only` (read-only, never-mutate). ADD `--ephemeral` ("Run without persisting session
  files" — confirmed in `codex exec --help`) so no session files leak from a one-shot message gen.
- **Result:** `bare_flags = ["--sandbox", "read-only", "--ephemeral"]` (3 tokens). Still read-only,
  still never-asks, now also session-clean. The §12.7.1 "read-only constraint" framing is preserved.

## 5. cursor — field-by-field table (VERBATIM §12.7, no revisions)

```toml
# PRD §12.7 cursor TOML — VERBATIM (no revision). decode-parity fixture is this block unchanged.
name = "cursor"
detect = "agent"                     # ← ≠ Name! the standalone binary is `agent`
command = "agent"
subcommand = []                      # ← explicit empty array → NON-NIL empty slice (FINDING D)
prompt_delivery = "positional"
print_flag = "-p"
model_flag = "--model"
default_model = ""                   # user sets; explicit empty → NON-NIL
system_prompt_flag = ""              # NO sys flag → prepend; explicit empty → NON-NIL
provider_flag = ""                   # explicit empty → NON-NIL
bare_flags = ["--mode", "ask", "--trust"]
output = "raw"
strip_code_fence = true
```

| field | value | nil/non-nil | note |
|---|---|---|---|
| Name | "cursor" | plain | |
| Detect | "agent" | non-nil | **≠ Name** — see §6 gotcha A |
| Command | "agent" | non-nil | ≠ Name |
| Subcommand | `[]` | **NON-NIL empty** | §12.7 explicit `[]` → FINDING D; see §6 gotcha B |
| PromptDelivery | "positional" | non-nil | |
| PromptFlag | — | **nil (ABSENT)** | |
| PrintFlag | "-p" | non-nil | |
| ModelFlag | "--model" | non-nil | |
| DefaultModel | `""` | **NON-NIL empty** | §12.7 explicit `""` (per-account model availability) |
| SystemPromptFlag | `""` | **NON-NIL empty** | §12.7 explicit `""` → prepend |
| ProviderFlag | `""` | **NON-NIL empty** | §12.7 explicit `""` |
| DefaultProvider | — | **nil (ABSENT)** | §12.7 omits key |
| BareFlags | `["--mode","ask","--trust"]` | NON-NIL 3-element | ask=read-only; trust=skip ws-trust prompt |
| Output | "raw" | non-nil | |
| JsonField | — | **nil (ABSENT)** | |
| StripCodeFence | true | non-nil | |
| RetryInstruction | — | **nil (ABSENT)** | |
| Env | — | **nil (ABSENT)** | |

## 6. THE cursor gotchas

### A — `Detect`/`Command` = "agent", NOT "cursor" (the only provider where detect ≠ name)

§12.7: `detect = "agent"` and `command = "agent"`. The standalone binary is `agent`; the map key / Name
is "cursor". Implications:
- `builtinCursor()` sets `Detect: strPtr("agent")`, `Command: strPtr("agent")` — NOT "cursor".
- `TestBuiltinManifests_NameMatchesKey` still passes (it checks `.Name == key` → "cursor"=="cursor").
- The CursorFields test MUST assert `*Detect == "agent"` (a careless copy from codex's "codex" would fail).
- `DetectCommand()` returns "agent" → the registry (P1.M2.T3) runs `exec.LookPath("agent")`.
- §12.7 inline note: "some installs expose this as `cursor agent` (the `agent` subcommand). If `agent` is
  not on $PATH, set command=`cursor` subcommand=[`agent`]." That is a USER override concern, not the
  default. The default stays `command="agent" subcommand=[]` (verbatim §12.7).

### B — `Subcommand = []` decodes to a NON-NIL empty slice (FINDING D — same as opencode.BareFlags in S2)

§12.7 writes `subcommand = []`. Per go-toml-pointer-behavior FINDING D, a PRESENT-but-empty array decodes
to a NON-NIL empty slice (`len 0`, `!= nil`). So `builtinCursor().Subcommand` MUST be `[]string{}` —
NOT nil (omitting the field → nil → `reflect.DeepEqual` fails decode-parity, nil ≠ non-nil-empty).
- This is a fidelity concern only — `renderArgs` does `append(args, Subcommand...)` which is a no-op for
  both nil and empty. But the DecodeParity oracle is `reflect.DeepEqual`, which distinguishes them.
- Write it explicitly: `Subcommand: []string{}`.

### C — the §12.2 render ORDER differs from the §12.7 illustrative "Rendered" block (NOT a bug)

§12.7's hand-written "Rendered" example shows: `agent -p --mode ask --trust --model gpt-5 "<...>"`.
But §12.2's algorithm (ported as `renderArgs`) orders tokens: command, subcommand, [provider], [model],
[sys], bare_flags, [print_flag], [positional payload]. For cursor that yields:
```
agent --model gpt-5 --mode ask --trust -p "<sys>\n\n<payload>"
```
- These differ ONLY in token order. cursor (like all these CLIs) parses flags in any order → semantically
  identical. §12.2 is the AUTHORITATIVE algorithm (the real P1.M2.T4 renderer implements it); the §12.7
  "Rendered" blocks are illustrative and not guaranteed byte-faithful to the algorithm.
- The RenderedCommand_Cursor test asserts the §12.2 algorithm output (`renderArgs` result), with a
  comment noting the §12.7 illustrative ordering differs. This is consistent with how S1/S2 rendered
  their manifests (they always assert `renderArgs` output, never the §12.x prose block).
- gemini/opencode (S2) did NOT hit this because their illustrative blocks happened to match §12.2 order;
  cursor is the first where they diverge. Documented honestly, not papered over (§12.7.2 spirit).

## 7. The two `# TO CONFIRM` items (work item: "Add a comment documenting the two TO CONFIRM items")

Both come from PRD Appendix E item 4 + §12.7 inline notes. They do NOT block the manifest shape (the
fields are confirmed; only runtime behavior is unconfirmed). Carry them as `// TO CONFIRM` comments in
the constructors (honest stubbing per §12.7.2):

1. **codex (stdout-on-success):** that `codex exec` writes the assistant's FINAL answer to stdout and
   exits 0 on success. Expected; external_deps.md notes `-o <file>` / `--json` as alternative output
   channels if stdout proves unreliable. (This subtask sets `output="raw"` + `strip_code_fence=true`;
   the parser P1.M2.T6 will read stdout.)
2. **cursor (ask-wins-over-`-p`):** that `--mode ask` wins over `-p`'s default FULL-tools profile — i.e.
   the combo is genuinely read-only. Expected (`ask` is defined as read-only Q&A); verify against a real
   run. (This subtask sets `bare_flags=["--mode","ask","--trust"]`; the executor P1.M2.T5 will run it.)

Both are integration-time confirmations for P1.M2.T5/T6 (and the real-agent scaffold P1.M5.T1.S2), NOT
for this subtask. This subtask ENCODES the data and DOCUMENTS the open questions inline.

## 8. Test strategy (mirrors S1/S2 exactly — the pattern is now proven)

`builtin_test.go` after S2 has: `assertStr`/`assertNilStr`/`renderArgs` helpers; piTOML/claudeTOML/
geminiTOML/opencodeTOML constants; KeysAndCount(4); NameMatchesKey; PiFields/ClaudeFields/GeminiFields/
OpenCodeFields; Validate; DecodeParity(4 rows); RenderedCommand_Pi/Gemini/OpenCode; FreshEachCall.

S3 EXTENDS (does not recreate):
- ADD `codexTOML` + `cursorTOML` constants (codexTOML = §12.7 codex with the 2 revisions; cursorTOML =
  verbatim §12.7 cursor).
- UPDATE `TestBuiltinManifests_KeysAndCount`: 4 → 6 keys (pi/claude/gemini/opencode/codex/cursor).
- UPDATE `TestBuiltinManifests_DecodeParity`: table +2 rows ({codex, builtinCodex(), codexTOML},
  {cursor, builtinCursor(), cursorTOML}).
- ADD `TestBuiltinManifests_CodexFields` (every field incl. the 4 NON-NIL-empty scalars, PromptDelivery
  "stdin", BareFlags the revised 3-token slice; absent fields nil).
- ADD `TestBuiltinManifests_CursorFields` (Detect/Command "agent" ≠ Name, Subcommand NON-NIL empty
  `[]string{}`, the 3 NON-NIL-empty scalars, BareFlags 3-token; absent fields nil).
- ADD `TestBuiltinManifests_RenderedCommand_Codex` (stdin: `["codex","exec","-m","gpt-5","--sandbox",
  "read-only","--ephemeral"]`, no payload in argv).
- ADD `TestBuiltinManifests_RenderedCommand_Cursor` (positional: append payload → `["agent","--model",
  "gpt-5","--mode","ask","--trust","-p","<sys>\n\n<payload>"]` per §12.2; comment on §12.7 order diff).
- NameMatchesKey + Validate auto-cover codex/cursor (iterate whole map) — no edit needed.
- DO NOT change `renderArgs`/`assertStr`/`assertNilStr` signatures (S1/S2 tests depend on them).

## 9. Decoded render expectations (cross-check vs external_deps.md)

| provider | delivery | expected argv (§12.2, model="gpt-5") | external_deps.md rendered |
|---|---|---|---|
| codex | stdin | `codex exec -m gpt-5 --sandbox read-only --ephemeral` (payload via stdin `-`) | matches (stdin form, revised bare) |
| cursor | positional | `agent --model gpt-5 --mode ask --trust -p "<sys>\n\n<payload>"` | same tokens, different order (§6C) |

## 10. Scope guard (do NOT do)

- Do NOT edit `manifest.go`/`manifest_test.go` (S1) or `merge.go`/`merge_test.go` (S2) — frozen contracts.
- Do NOT touch config/git/main.go/Makefile/go.mod/go.sum (`builtin.go` stays import-free; `go mod tidy`
  is a no-op — builtin_test.go already imports testing/reflect/go-toml).
- Do NOT add the registry (P1.M2.T3), renderer (T4), executor (T5), parser (T6), `providers/*.toml`
  reference files (P1.M5.T2), or `providers show` (P1.M4.T1.S3). This subtask = DATA ONLY.
- Do NOT implement codex's literal `-` stdin token or cursor's `--output-format json` — both downstream.
- Do NOT re-open the codex discrepancy; the work item + external_deps.md RESOLVE it (drop the flag).
