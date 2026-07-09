# Gemini Removal + agy Manifest Audit

Scope: verify the stagecoach codebase already reflects (1) the removal of the `gemini`
built-in provider and (2) the corrected `agy` manifest. **Read-only audit — no edits made.**
All four checks PASS. The codebase is already in the desired state.

Verification commands run:
- `go build ./...` → EXIT 0 (clean)
- `go test ./internal/provider/... ./internal/config/...` → both packages `ok` (PASS)

---

## Check 1 — `internal/provider/builtin.go` ✅ MATCHES (all items)

### Built-in provider count + no gemini
`BuiltinManifests()` returns a map built at `internal/provider/builtin.go:22-34`. It contains
**exactly seven** entries — no `gemini` key:

| map key | factory | builtin.go |
|---------|---------|------------|
| `pi`        | `builtinPi()`       | `:26` |
| `claude`    | `builtinClaude()`   | `:27` |
| `opencode`  | `builtinOpenCode()` | `:28` |
| `codex`     | `builtinCodex()`    | `:29` |
| `cursor`    | `builtinCursor()`   | `:30` |
| `agy`       | `builtinAgy()`      | `:31` |
| `qwen-code` | `builtinQwenCode()` | `:32` |

The package docstring (`builtin.go:11-20`) explicitly states "Seven providers: pi, claude,
opencode, codex, cursor, agy, qwen-code" and that "gemini-cli itself is EOL and no longer
shipped". No `builtinGemini` function exists anywhere (grep for `func builtinGemini` → none).

### agy manifest fields — `builtinAgy()` at `internal/provider/builtin.go:199-217`
(doc comment `:154-197`; return literal `:200-216`). All eight required fields MATCH:

| field | expected | actual | line |
|-------|----------|--------|------|
| `PrintFlag`         | `""` (non-nil empty)        | `strPtr("")`                | `:205` ✅ |
| `ModelFlag`         | `"--model"`                 | `strPtr("--model")`         | `:206` ✅ |
| `PromptDelivery`    | `"stdin"`                   | `strPtr("stdin")`           | `:204` ✅ |
| `DefaultModel`      | `"Gemini 3.5 Flash (Low)"`  | `strPtr("Gemini 3.5 Flash (Low)")` | `:207` ✅ |
| `BareFlags`         | `["--mode","plan"]`         | `[]string{"--mode","plan"}` | `:210-212` ✅ |
| `ListModelsCommand` | `["agy","models"]`          | `[]string{"agy","models"}`  | `:203` ✅ |
| `Experimental`      | `true`                      | `boolPtr(true)`             | `:215` ✅ |
| `TooledFlags`       | `nil`                       | omitted (nil)               | `:216` (comment) ✅ |

All eight confirmed. No correction needed.

---

## Check 2 — `internal/provider/registry.go` ✅ MATCHES

`preferredBuiltins` declared at `internal/provider/registry.go:15`:

```go
var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "qwen-code", "codex", "claude"}
```

- 7 entries, **no `gemini`**. ✅
- Doc (`:11-15`) notes it MUST stay in sync with `BuiltinManifests()` keys, enforced by
  `TestPreferredBuiltins_MatchesBuiltinKeys` (test passes).
- FR-D1 ordering: open/self-hostable first (pi, opencode, cursor, agy, qwen-code), closed
  subscription CLIs last (codex, claude).

---

## Check 3 — `internal/config/role_defaults.go` ✅ MATCHES (display labels)

The `agy` column in `roleDefaults` (`internal/config/role_defaults.go:55-61`) uses the agy
display-label form (verbatim from `agy models`), reasoning baked into the parenthesized suffix:

```go
"agy": {
    "planner": "Gemini 3.5 Flash (High)",   // flagship/smart tier = flash with high thinking
    "stager":  "",                          // NOT stager-capable (TooledFlags nil)
    "message": "Gemini 3.5 Flash (Low)",    // fast/cheapest tier = flash with low thinking
    "arbiter": "Gemini 3.5 Flash (Medium)", // mid tier
},
```

- Display labels, not API-style ids. ✅
- `stager` is `""` (consistent with `TooledFlags=nil` → bootstrap applies FR-D4 fallback). ✅
- No `gemini` key in `roleDefaults` (the seven providers match the manifest set). ✅
- Per-provider FR-D5 provenance comment (`:28-32`) documents the agy label refresh.

---

## Check 4 — provider TOML files ✅ MATCHES

- `providers/gemini.toml` — **does NOT exist** (confirmed via `ls providers/` and `grep`).
  The `providers/` directory contains exactly: `agy.toml, claude.toml, codex.toml,
  cursor.toml, opencode.toml, pi.toml, qwen-code.toml`. ✅
- `providers/agy.toml` — **exists and matches the corrected manifest**. Verified each emitted
  field against `builtinAgy()`:
  - `prompt_delivery = "stdin"` ✅
  - `print_flag = ""` (NON-NIL empty, documented: value-taking `-p` breaks delivery) ✅
  - `model_flag = "--model"` ✅
  - `default_model = "Gemini 3.5 Flash (Low)"` ✅
  - `bare_flags = ["--mode","plan"]` ✅
  - `list_models_command = ["agy","models"]` ✅
  - `experimental = true` ✅
  - `tooled_flags` omitted (nil) — intentionally cannot stager ✅
  - The header documents the divergence from the gemini-cli lineage (agy v1.1.0 removed
    `--approval-mode`; `-p` is value-taking) and the re-verification (2026-07-08).

---

## Residual observations (not corrections — audit is satisfied)

These are cosmetic / informational only. The audit requires no changes.

1. **Test fixtures use "gemini" as arbitrary string data** — `internal/config/load_test.go`
   (e.g. `:350`, `:585`, `:619`, `:1563`) and `roles_test.go`, `config_test.go`, `file_test.go`
   use `"gemini"` as a provider NAME and `"gemini-2.5-pro"`/`"gemini-2.5-flash"` as model
   strings. These test the **config field-merge / precedence logic** (which CLI layer wins),
   NOT the provider registry — `gemini` there is an opaque string, not a built-in lookup. All
   these tests pass. They are stale-but-harmless test data; renaming them is optional polish,
   not a correctness fix, and is out of scope for this audit.

2. **Historical plan docs** reference gemini throughout `plan/001..012/` and `PRD.md` snapshots.
   These are immutable planning artifacts (snapshots), not runtime code — correctly left in place.

3. All `gemini` hits in `internal/provider/builtin.go` are comment-only (lineage / model-label
   references: "Gemini-CLI successor", "Gemini 3.5 Flash (Low)", "the former gemini provider").
   No code path references a gemini provider.

## Conclusion
All four audit checks PASS. The gemini provider is fully removed (no built-in, no registry
entry, no role-defaults column, no TOML file), and the agy manifest is corrected and consistent
across `builtin.go`, `registry.go`, `role_defaults.go`, and `providers/agy.toml`. No edits
required.
