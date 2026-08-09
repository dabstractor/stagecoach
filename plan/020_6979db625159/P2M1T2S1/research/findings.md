# Research Findings — P2.M1.T2.S1 (Provider subsystem test-fixture token refresh)

## §0. What this task IS

A **mechanical test-fixture rename** across 9 test files in 4 packages: refresh every `glm-5.2` /
`glm-5-turbo` / `zai/glm-*` / standalone-`zai`-with-a-glm-model token to `claude-haiku` /
`anthropic/claude-haiku` / `anthropic`, so the test fixtures use the same canonical example token as the
runtime code+docs (P2.M1.T1.S1/S2) — and the P2.M1 acceptance grep returns zero hits.

It is NOT a behavior change. It consumes the NEW error-message tail from P2.M1.T1.S2 (`e.g.
"anthropic/claude-haiku"`) and renames fixture inputs + any token-bearing assertions IN LOCKSTEP.

## §1. The 5 replacement mappings (from P2.M1.T1.S2 contract + model_cleanup_scope.md)

| Old token | New token | Where it applies |
|-----------|-----------|------------------|
| `zai/glm-5.2` | `anthropic/claude-haiku` | prefixed (valid fold input) |
| `glm-5.2` (bare) | `claude-haiku` (bare) | the no-slash error case / incidental fixture |
| `zai/glm-5-turbo` | `anthropic/claude-haiku` | prefixed personal-override input |
| `glm-5-turbo` (bare) | `claude-haiku` (bare) | bare fixture / standalone model |
| `zai` (standalone provider, paired with a GLM model) | `anthropic` | the `--provider zai` arg / `{"model","zai"}` row |

**Bare vs prefixed rule (CRITICAL — do not cross them):** a bare model stays bare (`glm-5.2`→`claude-haiku`);
a prefixed model stays prefixed (`zai/glm-5.2`→`anthropic/claude-haiku`). Prefixing a bare one would break
the FR-R5b "no slash = error" test illustration.

## §2. CRITICAL OUT-OF-SCOPE: `zai/gpt-5.4*` in roles_test.go — DO NOT TOUCH

`internal/decompose/roles_test.go` has a SECOND model family using the `zai/` prefix:
- L102/122/123: `zai/gpt-5.4-nano`
- L408/419/420: `zai/gpt-5.4`
- L504/522: `zai/gpt-5.4-mini`

These are **gpt**, not **glm**. The acceptance grep `rg "zai/glm|glm-5\.2|glm-5-turbo"` does NOT match them,
and the contract scopes roles_test.go to ONLY L318/L323 (the glm-5-turbo lines). **A blind global
`zai`→`anthropic` replace would corrupt these.** The rename is TARGETED to glm tokens only. In roles_test.go,
edit ONLY L318 (comment) + L323 (fixture); leave every `zai/gpt-5.4*` line byte-identical.

## §3. The complete hit inventory (verified via grep — every token-bearing line)

### internal/provider/manifest_test.go (8 lines)
- **L18** TOML fixture: `default_model = "glm-5-turbo"` → `default_model = "claude-haiku"` (bare)
- **L63** assertion: `assertStr(t, "DefaultModel", m.DefaultModel, "glm-5-turbo")` → `"claude-haiku"` (lockstep with L18)
- **L323** fixture: `DefaultModel: strPtr("zai/glm-5.2")` → `strPtr("anthropic/claude-haiku")`
- **L325** input: `m.ValidateModel("glm-5.2")` (bare, the error case) → `"claude-haiku"` (bare)
- **L338** fixture: `DefaultModel: strPtr("zai/glm-5.2")` → `strPtr("anthropic/claude-haiku")`
- **L340** input: `m.ValidateModel("zai/glm-5.2")` (valid) → `"anthropic/claude-haiku"`
- **L350** fixture: `DefaultModel: strPtr("glm-5.2")` (bare) → `strPtr("claude-haiku")` (bare)
- **L383** input: `m.ValidateModel("zai/glm-5.2")` (incidental — manifest is nameless) → `"anthropic/claude-haiku"`
- **L329** STATIC assertion `strings.Contains(err.Error(), "must be inference/model")` → UNCHANGED (test-asserted static phrase; P2.M1.T1.S2 preserves it)

### internal/provider/render_test.go (19 lines — the biggest file)
- **L87** input+comment: `builtinPi().Render("zai/glm-5-turbo", ...)` + comment `// ... → --provider zai --model glm-5-turbo` → input `"anthropic/claude-haiku"`; comment `--provider anthropic --model claude-haiku`
- **L91** assertion: `wantArgs := []string{"--provider", "zai", "--model", "glm-5-turbo", ...}` → `"--provider", "anthropic", "--model", "claude-haiku"`
- **L152** comment: `// pi + "zai/glm-5.2" ... → --provider zai --model glm-5.2` → `anthropic/claude-haiku` / `--provider anthropic --model claude-haiku`
- **L153** input: `builtinPi().Render("zai/glm-5.2", ...)` → `"anthropic/claude-haiku"`
- **L154** assertion: `containsPair(s.Args, "--provider", "zai") || containsPair(s.Args, "--model", "glm-5.2") || containsToken(s.Args, "zai/glm-5.2")` → `"anthropic"`, `"claude-haiku"`, `"anthropic/claude-haiku"`
- **L298** input: `pi.Render("glm-5.2", ...)` (bare, error case) → `"claude-haiku"` (bare); assertion is `err == nil` only
- **L306** fixture: `DefaultModel: strPtr("glm-5.2")` (bare) → `strPtr("claude-haiku")` (bare)
- **L312** comment: `// (3) fold success: "zai/glm-5.2" → --provider zai --model glm-5.2` → tokens updated
- **L313** input: `pi.Render("zai/glm-5.2", ...)` → `"anthropic/claude-haiku"`
- **L314** assertion: `containsPair(s.Args, "--provider", "zai") || containsPair(s.Args, "--model", "glm-5.2")` → `"anthropic"`, `"claude-haiku"`
- **L478** input+comment: `m.Render("zai/glm-5.2", "", "", lvl) // folds to --provider zai --model glm-5.2` → input+comment tokens updated (assertion L481 checks `--thinking`, NOT the model — safe)
- **L488** input: `m.Render("zai/glm-5.2", "", "", lvl)` → `"anthropic/claude-haiku"` (assertion checks `--thinking`)
- **L567** input: `mtPiManifest().RenderMultiTurn("zai/glm-5.2", ...)` → `"anthropic/claude-haiku"`
- **L572** assertion: `"--provider", "zai", "--model", "glm-5.2"` → `"--provider", "anthropic", "--model", "claude-haiku"`
- **L605** input: `RenderMultiTurn("zai/glm-5.2", ...)` → `"anthropic/claude-haiku"`
- **L610** assertion: `"--provider", "zai", "--model", "glm-5.2"` → `"anthropic"`, `"claude-haiku"`
- **L641** input: `RenderMultiTurn("zai/glm-5.2", ...)` → `"anthropic/claude-haiku"` (assertion is structural)
- **L659** input: `RenderMultiTurn("zai/glm-5.2", ...)` → `"anthropic/claude-haiku"` (return values ignored)
- **L685** input: `RenderMultiTurn("zai/glm-5.2", ...)` → `"anthropic/claude-haiku"`
- **L726** input: `RenderMultiTurn("zai/glm-5.2", ...)` → `"anthropic/claude-haiku"`
- **L747** input: `RenderMultiTurn("zai/glm-5.2", ...)` → `"anthropic/claude-haiku"`

### internal/provider/merge_test.go (5 lines — all BARE glm)
- **L19** `DefaultModel: strPtr("glm-5-turbo")` → `strPtr("claude-haiku")` (bare)
- **L42** `Manifest{DefaultModel: strPtr("glm-5.2")}` → `strPtr("claude-haiku")` (bare)
- **L46** assertion `*merged.DefaultModel != "glm-5.2"` → `"claude-haiku"`
- **L47** assertion `"want \"glm-5.2\""` → `"want \"claude-haiku\""`
- **L325** `Manifest{DefaultModel: strPtr("glm-5.2")}` → `strPtr("claude-haiku")` (bare)

### internal/provider/builtin_test.go (6 lines — provider+model pairs)
- **L436** `renderArgs(builtinPi(), "zai", "glm-5-turbo", "<sys>")` → `("anthropic", "claude-haiku")`
- **L438** `"--provider", "zai",` → `"--provider", "anthropic",`
- **L439** `"--model", "glm-5-turbo",` → `"--model", "claude-haiku",`
- **L884** `builtinPi().Render("zai/glm-5-turbo", ...)` → `"anthropic/claude-haiku"`
- **L889** `"--provider", "zai",` → `"--provider", "anthropic",`
- **L890** `"--model", "glm-5-turbo",` → `"--model", "claude-haiku",`

### internal/provider/registry_test.go (7 lines — all BARE glm)
- **L65** `{"pi": {DefaultModel: strPtr("glm-5.2")}}` → `strPtr("claude-haiku")` (bare)
- **L70** assertion `*got.DefaultModel != "glm-5.2"` → `"claude-haiku"`
- **L71** assertion `"want glm-5.2"` → `"want claude-haiku"`
- **L215** `{"pi": {DefaultModel: strPtr("glm-5.2")}}` → `strPtr("claude-haiku")` (bare)
- **L222** assertion `*decoded.DefaultModel != "glm-5.2"` → `"claude-haiku"`
- **L315** `{"pi": {"default_model": "glm-5.2"}}` → `"claude-haiku"` (bare)
- **L341** assertion `*pi.DefaultModel != "glm-5.2"` → `"claude-haiku"`

### internal/decompose/roles_test.go (2 lines — ONLY these; gpt-5.4* lines OUT OF SCOPE)
- **L318** comment `// because "glm-5-turbo" has no slash-prefix.` → `// because "claude-haiku" has no slash-prefix.`
- **L323** fixture `"planner": {Model: "glm-5-turbo"}` (bare) → `{Model: "claude-haiku"}` (bare)
- L332/333/391/392 (the error assertions) assert STATIC `"must be inference/model"` + `"planner"` — UNCHANGED (P2.M1.T1.S2 preserved them; they don't reference glm)

### internal/generate/realagent_test.go (1 line)
- **L40** `{"glm-5-turbo", "zai"}` (provider→model table row) → `{"claude-haiku", "anthropic"}`

### internal/ui/output_test.go (1 line — BOTH input AND expected output carry the token)
- **L250** `{"slash-prefixed model", "Generating", "zai/glm-5.2", "pi", "Generating with zai/glm-5.2 in pi…"}` → input + expected both → `anthropic/claude-haiku`: `{"slash-prefixed model", "Generating", "anthropic/claude-haiku", "pi", "Generating with anthropic/claude-haiku in pi…"}`

### internal/generate/multiturn_test.go (5 lines — model arg to Run)
- **L218/244/268/285/306** the 7th arg `"zai/glm-5.2"` to `Run(...)` → `"anthropic/claude-haiku"` (assertions check msg/ok/cause, not the token)

## §4. Test-safety confirmation (no test asserts on the RENAMED error-message tail)

Verified by reading manifest_test.go:329 + roles_test.go error tests + the S2 contract:
- The FR-R5b error tests assert ONLY on the STATIC `"must be inference/model"` substring (manifest_test.go:329)
  + role name (roles_test.go:332/333/391/392). P2.M1.T1.S2 PRESERVES that static phrase while changing only
  the `e.g. "..."` tail. So after S2 lands, those error tests pass regardless of the fixture input rename.
- The bare-model error tests (render_test.go:298, manifest_test.go:325/350, roles_test.go:323) assert only
  `err == nil` (an error IS returned) — never the message text. Renaming the bare input is safe.
- The fold tests (render_test.go:154/314, builtin_test.go) assert the STRUCTURE (`--provider X --model Y`)
  via `containsPair` — the assertion token renames IN LOCKSTEP with the input. Verified: assertion lines
  that reference the token appear in the grep (they're not hidden).

**∴ After the rename, all scoped suites pass green. The rename is fixture-input + token-bearing-assertion
only; no behavior or static-phrase change.**

## §5. Validation commands (Go repo — NOT Python/ruff)

- `go test ./internal/provider/... ./internal/decompose/... ./internal/generate/... ./internal/ui/...` — green.
- `go build ./...` — exit 0 (sanity; tests don't change build but confirm no compile break).
- `gofmt -l <the 9 files>` — clean (the edits are within existing literals; no reformatting).
- **Acceptance grep (the contract's zero-hit gate):**
  `rg -n "zai/glm|glm-5\.2|glm-5-turbo" internal/provider/*_test.go internal/decompose/*_test.go internal/generate/*_test.go internal/ui/*_test.go` → zero hits.
- Scope guard: `git status --porcelain` shows ONLY the 9 test files; NO code (P2.M1.T1 scope), NO spec/, NO
  config-subsystem tests (those are P2.M1.T2.S2 — Category C/D), NO docs.

## §6. Scope fence — what NOT to touch

- **P2.M1.T1.S1/S2's code+docs files** (manifest.go, render.go, roles.go, pi.toml, builtin.go, the 6 S1 files,
  README/docs) — those are the runtime surface; T2.S1 is TESTS only.
- **P2.M1.T2.S2's config-subsystem tests** (config_test.go, migrate_test.go, file_test.go, git_test.go,
  load_test.go, default_action_test.go, config_init_interactive_test.go, providers_test.go — Category C/D) —
  a separate sibling subtask. T2.S1 owns ONLY the 9 provider-subsystem test files listed above.
- **`zai/gpt-5.4*` lines in roles_test.go** (L102/122/123/408/419/420/504/522) — different model family, out of scope.
- **spec/**, **PRD.md**, **plan/**, **tasks.json** — human/orchestrator-owned.