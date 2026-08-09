# P2.M1.T2.S2 — Findings: Config-subsystem test-fixture token refresh

> Verified against the working tree via `rg`. Every line below is anchored to real file content.
> This task is the **Category C + D** slice of `model_cleanup_scope.md` (config migration tests + generic
> config tests). It is **DISJOINT** from sibling P2.M1.T2.S1 (Category B = provider-subsystem tests) and from
> P2.M1.T1.S1/S2 (runtime code + user-facing docs), so no merge conflict.

## §1 — The 3 token mappings (NO glm-5-turbo exists in these 8 files)

Confirmed by grep: `rg -n 'glm-5-turbo' internal/cmd/ internal/config/` → **zero hits**. Only three forms exist here:

| # | Old token | New token | Where |
|---|-----------|-----------|-------|
| M1 | bare `glm-5.2` | bare `claude-haiku` | generic fixtures + bare/no-fold migration cases |
| M2 | `zai/glm-5.2` (prefixed, already-folded output) | `anthropic/claude-haiku` | fold-output assertions + idempotent inputs |
| M3 | `default_provider = "zai"` / `"default_provider": "zai"` (bare, the fold INPUT) | `default_provider = "anthropic"` / `"default_provider": "anthropic"` | v2→v3 fold INPUTS in migrate_test.go + config_test.go |

**M3 is the heart of this task.** The v2→v3 fold concatenates `default_provider` + `/` + `default_model` into the
model prefix. So `default_provider="anthropic"` + `default_model="claude-haiku"` → `model="anthropic/claude-haiku"`,
which is exactly the spec's canonical example (PRD §16.3: `model = anthropic/claude-haiku` for pi) and matches the
runtime token P2.M1.T1 landed. If M3 is skipped, the fold yields `zai/claude-haiku` and the `anthropic/claude-haiku`
assertion **fails** — so M3 is mandatory, not optional.

## §2 — CRITICAL: the contract's acceptance grep UNDER-SPECIFIES the zai rename

The contract's zero-hit gate is `rg -n "zai/glm|glm-5\.2|glm-5-turbo" <8 files>`. This regex does **not** match a
bare `default_provider = "zai"` (no `/glm`). Consequence:

- A correct implementation (M1+M2+M3 applied) → grep is zero AND `go test` is green. ✅
- A **buggy** implementation that applies M1+M2 but **forgets M3** → grep is **also zero** (bare `default_provider =
  "zai"` slips through), BUT the migrate fold produces `zai/claude-haiku` while the assertion says
  `anthropic/claude-haiku` → **`go test` FAILS**. ❌

**Therefore the acceptance grep alone is necessary but NOT sufficient.** Two complementary gates are required:
- **Correctness gate**: `rg -n 'zai' internal/cmd/config_test.go internal/config/migrate_test.go` → **must be zero**
  (proves every bare-zai `default_provider` input + every `"\ndefault_provider = \"zai\"\n"` / `"# default_provider
  = \"zai\""` assertion was renamed — so no fold can emit a stray `zai/`).
- **go test is authoritative**: `go test ./internal/cmd/... ./internal/config/...` green. A missed M3 fails here.

## §3 — OUT-OF-SCOPE guards (a blind `zai`→`anthropic` / `glm`→`claude` replace corrupts these)

Discovered via `rg -n 'zai'` (broader than the contract grep). These survive the acceptance grep and MUST NOT change:

| File | Lines | Content | Why out of scope |
|------|-------|---------|------------------|
| `internal/config/file_test.go` | 187, 204, 205 | `"pi": {"default_model": "A", "default_provider": "zai"}`; `if got := dst.Providers["pi"]["default_provider"]; got != "zai"`; `want zai (field-merge must preserve lower-layer fields)` | This is **TestOverlayProvidersFieldMerge** — a provider-table MERGE test. The model is the arbitrary sentinel `"A"` (not glm). `default_provider "zai"` is an incidental sentinel proving lower-layer fields survive a merge. NOT a model-token illustration; the scope doc's file_test.go line list is L59/90/91/786/800/801 only. **Leave as-is.** |
| `internal/cmd/config_init_interactive_test.go` | 105, 107, 119, 120 | `zai/gpt-5.4`, `zai/gpt-5.4-mini`, `zai/gpt-5.4-nano` (input `"\ngpt-5.4\nzai/gpt-5.4\nzai/gpt-5.4-mini\n…"`) | A **DIFFERENT model family** (gpt-5.4, not glm). The acceptance grep (`zai/glm|glm-5\.2|glm-5-turbo`) does NOT match `zai/gpt-5.4`. This is the exact analog of P2.M1.T2.S1's `zai/gpt-5.4*` guard in `decompose/roles_test.go`. **Leave as-is.** |

**Scope-preservation gate** (proves the guards survived a rename): after edits,
`rg -n 'zai/gpt-5' internal/cmd/config_init_interactive_test.go` still shows L105/107/119/120, and
`rg -n '"zai"' internal/config/file_test.go` still shows L187/204/205.

## §4 — Per-file / per-line inventory (old → new)

### `internal/config/migrate_test.go` — TestMigrateV2ToV3 (12 cases; 6 carry a `default_provider:"zai"` that MUST change)

The migration mutates a `Config` struct in memory; assertions check the mutated `cfg.Model` / `cfg.Roles[*].Model` /
`cfg.Providers[*]["default_model"]` and that `default_provider` key is *deleted* (never its value). So only the
**input** `default_provider:"zai"` + the **model-value** assertions change; the "deleted" checks are untouched.

| Case (name) | Lines | Old (input/assert) | New |
|--------------|-------|--------------------|-----|
| 1 global pi model folded | 22,26,27 + **23** | input `Model:"glm-5.2"` + `default_provider:"zai"`; assert `cfg.Model=="zai/glm-5.2"` + `want zai/glm-5.2` | `claude-haiku` + `default_provider:"anthropic"`; assert `anthropic/claude-haiku` |
| 2 per-role folded explicit provider | 39,44,45 + **41** | role `planner:{Provider:"pi",Model:"glm-5.2"}` + `default_provider:"zai"`; assert `zai/glm-5.2` | `claude-haiku` + `anthropic`; assert `anthropic/claude-haiku` |
| 3 per-role inherits global | 54,59,60 + **56** | role `message:{Model:"glm-5.2"}` + `default_provider:"zai"`; assert `zai/glm-5.2` | `claude-haiku` + `anthropic`; assert `anthropic/claude-haiku` |
| 4 raw map folded | 69,73,74 | `"pi":{"default_provider":"zai","default_model":"glm-5.2"}`; assert `default_model=="zai/glm-5.2"` | `{"default_provider":"anthropic","default_model":"claude-haiku"}`; assert `anthropic/claude-haiku` |
| 5 idempotent already-prefixed | 85,86,89,90,92,93 | input `Model:"zai/glm-5.2"` + `{"default_provider":"zai","default_model":"zai/glm-5.2"}`; asserts `==zai/glm-5.2 (unchanged)` | all `zai/glm-5.2`→`anthropic/claude-haiku`; `default_provider:"zai"`→`"anthropic"` |
| 6 single-backend claude untouched | (L98–111) | `claude`, `Model:"opus"`, `default_provider:"anthropic"` | **NO CHANGE** — no glm/zai token; grep doesn't match. (Pre-existing "anthropic" for claude is incidental; unrelated to pi.) |
| 7 empty default_provider no-fold | 122,126,127 | input `Model:"glm-5.2"` + `default_provider:""`; assert bare `glm-5.2` | input `claude-haiku` + `default_provider:""` (unchanged); assert bare `claude-haiku` |
| 8 bare no default_provider (no-invent) | 138,142,143 | input `Model:"glm-5.2"` + `{"default_model":"glm-5.2"}`; assert bare `glm-5.2` | bare `claude-haiku` (both input + assert) |
| 9 nil Providers no-op | 151,155,156 | input `Model:"glm-5.2"`, `Providers:nil`; assert bare `glm-5.2` | bare `claude-haiku` |
| 10 nil Roles (global folds) | 164,169,170 + **166** | input `Model:"glm-5.2"` + `default_provider:"zai"`; assert `zai/glm-5.2` | `claude-haiku` + `anthropic`; assert `anthropic/claude-haiku` |
| 11 no providers empty map | 178,182,183 | input `Model:"glm-5.2"`, empty `Providers`; assert bare `glm-5.2` | bare `claude-haiku` |
| 12 non-string dp no-fold | 191,195,196 | input `Model:"glm-5.2"` + `default_provider:42`; assert bare `glm-5.2` | bare `claude-haiku` |

**Bare-zai `default_provider` count in this file = 6** (L23/41/56/69/86/166) — all pair with a glm model and fold;
all → `"anthropic"`. (`default_provider:""` at L125, `:42` at L191 are not `"zai"` — untouched.)

### `internal/cmd/config_test.go` — TestUpgradeConfigVersion_V3Rewrite + TestConfigUpgrade_V2ToV3Rewrite

These assert on **substrings of the on-disk TOML** produced by `upgradeConfigVersion` / the `config upgrade`
command. So both the INPUT TOML literals AND the assertion substrings carry tokens — **all in lockstep**.

| Sub-test | Line(s) | Old | New |
|----------|---------|-----|-----|
| folds provider default_model + comments dp + bumps ver | 1349 input `default_provider=\"zai\"`; 1350 input `default_model=\"glm-5.2\"`; 1356 assert `default_model=\"zai/glm-5.2\"`; 1359 NEG assert `\"\\ndefault_provider=\\\"zai\\\"\\n\"`; 1362 assert `# default_provider=\"zai\"` | → `anthropic` / `claude-haiku` / `anthropic/claude-haiku` / `anthropic` / `anthropic` |
| folds global model | 1379 input `model=\"glm-5.2\"`; 1382 input `default_provider=\"zai\"`; 1384 assert `model=\"zai/glm-5.2\"` | → `claude-haiku` / `anthropic` / `anthropic/claude-haiku` |
| folds per-role model (×2) | 1396/1399 input `model=\"glm-5.2\"`; 1402 input `default_provider=\"zai\"`; 1405 assert Count `model=\"zai/glm-5.2\"`==2 | → `claude-haiku` / `anthropic` / `anthropic/claude-haiku` (count still 2) |
| single-backend claude NOT prefixed | (L1413–1431) | `claude`, `default_provider=\"anthropic\"`, `default_model=\"opus\"` | **NO CHANGE** — no glm/zai token; grep doesn't match. |
| agent table header renamed | 1430 input `default_provider=\"zai\"`; 1431 input `default_model=\"glm-5.2\"`; 1439 assert `default_model=\"zai/glm-5.2\"` | → `anthropic` / `claude-haiku` / `anthropic/claude-haiku` |
| idempotent v3 no-op | 1445 input+assert `default_model=\"zai/glm-5.2\"` | → `anthropic/claude-haiku` |
| bare no default_provider stays bare | 1456 input `default_model=\"glm-5.2\"`; 1458 assert `default_model=\"glm-5.2\"`; **1461 NEG assert `strings.Contains(got,\"/glm-5.2\\\"\")`** | → `claude-haiku` / `claude-haiku` / **`/claude-haiku\"`** (negative — asserts NO prefix invented) |
| TestConfigUpgrade_V2ToV3Rewrite (command round-trip) | 1480 input `model=\"glm-5.2\"`; 1483 input `default_provider=\"zai\"`; 1505 assert `model=\"zai/glm-5.2\"`; 1508 assert `# default_provider=\"zai\"` | → `claude-haiku` / `anthropic` / `anthropic/claude-haiku` / `anthropic` |

**Bare-zai `default_provider` count in this file = 8** (L1349/1359/1362/1382/1402/1430/1483/1508) — input + the
active-form + commented-form assertions all → `anthropic`.

### `internal/config/file_test.go` — config-file parse (ONLY L59/90/91/786/800/801)

Two near-identical TOML-parse fixtures (default_model value + its assertion). Model is bare `glm-5.2` → bare
`claude-haiku`. **L187/204/205 are OUT OF SCOPE (§3) — do NOT touch.**

| Lines | Old | New |
|-------|-----|-----|
| 59 (TOML literal), 90 (`!= "glm-5.2"`), 91 (`want glm-5.2`) | `default_model = "glm-5.2"`; `!= "glm-5.2"`; `want glm-5.2` | `claude-haiku` (all three) |
| 786, 800, 801 (second fixture, identical pattern) | same | `claude-haiku` (all three) |

### `internal/config/git_test.go` — git-config model parse (L76/104/105, bare)

| Line | Old | New |
|------|-----|-----|
| 76 | `setGitConfig(t, repo, "stagecoach.model", "glm-5.2")` | `"claude-haiku"` |
| 104 | `if cfg.Model != "glm-5.2"` | `!= "claude-haiku"` |
| 105 | `t.Errorf("Model=%q want glm-5.2", cfg.Model)` | `want claude-haiku` |

### `internal/config/load_test.go` — config load env var (L148/161/162, bare)

| Line | Old | New |
|------|-----|-----|
| 148 | `t.Setenv("STAGECOACH_MODEL", "glm-5.2")` | `"claude-haiku"` |
| 161 | `if cfg.Model != "glm-5.2"` | `!= "claude-haiku"` |
| 162 | `t.Errorf("Model=%q want glm-5.2", cfg.Model)` | `want claude-haiku` |

### `internal/cmd/default_action_test.go` — default action dry-run + FR51b label (L1500/1532/1539/1540/1541)

L1500 is the BARE-model-on-pi FR-R5b error case (asserts the STATIC `must be inference/model`); L1532–1541 is the
stub-provider happy path whose FR51b stderr label embeds the model token — **input + label assertion in lockstep**.

| Line | Old | New |
|------|-----|-----|
| 1500 | `rootCmd.SetArgs([]string{"--provider","pi","--model","glm-5.2","--dry-run"})` (bare → error) | `claude-haiku` (stays bare, stays the error case) |
| 1532 | `rootCmd.SetArgs([]string{"--provider","stub","--model","glm-5.2"})` | `claude-haiku` |
| 1539 | `// FR51b: stderr shows "↳ Generating with glm-5.2 in stub…"` | `claude-haiku` |
| 1540 | `strings.Contains(errBuf.String(), "↳ Generating with glm-5.2 in stub…")` | `claude-haiku` |
| 1541 | `t.Errorf("…FR51b label '↳ Generating with glm-5.2 in stub…'", …)` | `claude-haiku` |

> Note: the FR51b label format (`↳ Generating with %s in %s…`) is **runtime behavior**, unchanged by P2.M1.T1.
> The label embeds the actual `--model` value, so changing the SetArgs arg to `claude-haiku` makes the runtime emit
> `claude-haiku` — and the assertion must match. Pure lockstep rename.

### `internal/cmd/config_init_interactive_test.go` — interactive wizard override (ONLY L589/593)

| Line | Old | New |
|------|-----|-----|
| 589 | `overrides := map[string]string{"planner": "zai/glm-5.2"}` | `"anthropic/claude-haiku"` |
| 593 | `if !strings.Contains(content, \`model = "zai/glm-5.2"\`)` | `model = "anthropic/claude-haiku"` |

> **L105/107/119/120 (`zai/gpt-5.4*`) are OUT OF SCOPE (§3) — different family; do NOT touch.**

### `internal/cmd/providers_test.go` — providers show override (L226/241/242, bare; note SINGLE quotes in output)

| Line | Old | New |
|------|-----|-----|
| 226 (TOML literal, double quotes) | `default_model = "glm-5.2"` | `default_model = "claude-haiku"` |
| 241 (output assertion, **SINGLE** quotes) | `strings.Contains(got, "default_model = 'glm-5.2'")` | `default_model = 'claude-haiku'` |
| 242 (error msg, single quotes) | `t.Error(\`…overridden "default_model = 'glm-5.2'"\`)` | `'claude-haiku'` |

> The single quotes at L241/242 are how the `providers show` command renders TOML values — keep the quoting style,
> change ONLY the model token. Input (L226, double-quoted TOML) and output (L241, single-quoted render) move in
> lockstep: pass `claude-haiku` in → expect `'claude-haiku'` out.

## §5 — Test-safety (why every rename stays green)

- **Lockstep rule**: where an input and its assertion reference the same token, BOTH move. Verified sites:
  migrate_test.go cases 1–5/10 (input `default_provider`+model ↔ asserted folded model); config_test.go ALL sub-tests
  (input TOML ↔ substring assertions, including the **negative** asserts at L1359/1461); default_action L1532↔1540/1541
  (model arg ↔ FR51b label); providers_test L226↔241/242 (TOML input ↔ rendered output); config_init_interactive
  L589↔593 (override ↔ written content).
- **Static-substring preserved**: default_action_test.go:1500 asserts ONLY on the STATIC `must be inference/model`
  error tail (preserved by P2.M1.T1.S2), NOT on the model token — so the bare input rename is input-only and safe.
- **Bare stays bare / prefixed stays prefixed**: M1 (bare glm → bare claude-haiku) and M2 (prefixed → prefixed) never
  cross. The bare/no-fold migration cases (migrate cases 7/8/9/11/12, config_test L1456–1461) STAY bare to keep
  illustrating "no default_provider ⇒ no prefix invented" (FR-B7 no-invent). Prefixing them would invert the test's
  meaning.
- **The fold mechanism is unchanged**: `anthropic`+`/`+`claude-haiku` exercises the IDENTICAL FR-B7/FR-R5b concat that
  `zai`+`/`+`glm-5.2` did. Only the example token moves.

## §6 — Validation (verified commands, run from repo root)

```bash
# L1 build + format
go build ./...
gofmt -l internal/cmd/config_test.go internal/config/migrate_test.go internal/config/file_test.go \
  internal/config/git_test.go internal/config/load_test.go internal/cmd/default_action_test.go \
  internal/cmd/config_init_interactive_test.go internal/cmd/providers_test.go   # → empty

# L2 scoped unit tests (the green gate — authoritative)
go test ./internal/cmd/... ./internal/config/...

# L3 whole-repo sanity
go test ./... 2>&1 | tail -30   # no FAIL in these 2 packages; flag (don't fix) unrelated failures

# L4 acceptance (contract zero-hit) — PRIMARY gate
rg -n "zai/glm|glm-5\.2|glm-5-turbo" internal/cmd/config_test.go internal/config/migrate_test.go \
   internal/config/file_test.go internal/config/git_test.go internal/config/load_test.go \
   internal/cmd/default_action_test.go internal/cmd/config_init_interactive_test.go \
   internal/cmd/providers_test.go   # → ZERO hits

# L4b CORRECTNESS gate (the gap the contract grep misses) — zai fully purged from the 2 fold-test files
rg -n 'zai' internal/cmd/config_test.go internal/config/migrate_test.go   # → ZERO hits

# L4c SCOPE-PRESERVATION gate (the out-of-scope guards survived)
rg -n 'zai/gpt-5' internal/cmd/config_init_interactive_test.go            # → still L105/107/119/120
rg -n '"zai"' internal/config/file_test.go                               # → still L187/204/205
```

## §7 — Scope fence (what this task does NOT touch)

- **No production code, no docs, no config templates.** Only the 8 *_test.go files above.
- **No sibling-scope files**: NOT P2.M1.T1 code/docs (manifest.go, render.go, roles.go, pi.toml, builtin.go,
  bootstrap.go, config.go, config_init_interactive.go, default_action.go, README, docs/*); NOT P2.M1.T2.S1
  provider-subsystem tests (render_test.go, manifest_test.go, merge_test.go, builtin_test.go, registry_test.go,
  roles_test.go, realagent_test.go, output_test.go, multiturn_test.go) — disjoint files, no conflict.
- **No spec/PRD/plan/tasks.json** (human/orchestrator-owned — AGENTS.md rules 1–3).
- **Out-of-scope within the touched files**: file_test.go L187/204/205 (field-merge sentinel, model "A");
  config_init_interactive_test.go L105/107/119/120 (zai/gpt-5.4* family); migrate_test.go case 6 + config_test.go
  single-backend claude sub-test (no glm/zai token; uses pre-existing "anthropic"/"opus" for the claude provider).