# Phase 2 — Model-Example Cleanup Scope (zai/glm-5.2 → anthropic/claude-haiku)

## CRITICAL FINDING: PRD Under-Scoped Phase 2

The PRD identifies **~4 files** (pi.toml, config_init_interactive.go, default_action.go, config_test.go) as needing cleanup. Actual codebase sweep found **90+ references across 30+ files**.

The PRD's acceptance criterion (`rg -n 'zai/glm|glm-5\.2|glm-5-turbo|zai/glm-5'` returns zero hits, excluding spec/ and plan/) is achievable but requires coordinated changes across all categories below.

## Full Categorization

### Category A — User-facing examples and error messages (MUST change)
These use `zai/glm-5.2` or `glm-5-turbo` to illustrate the inference-backend prefix-fold mechanism (FR-R5b). They are shown to users in error messages, config templates, help text, and interactive prompts. These MUST be refreshed to `anthropic/claude-haiku`.

| File | Lines | Type | Content |
|------|-------|------|---------|
| `providers/pi.toml` | 18, 24, 53, 61 | Comments | Personal-override example, backend list |
| `internal/cmd/config_init_interactive.go` | 184, 200 | UI strings | Wizard prompt "e.g. zai/glm-5.2" |
| `internal/cmd/default_action.go` | 234 | Comment | "bare "glm-5.2" on pi — FR-R5b" |
| `internal/cmd/config.go` | 94, 687 | Help/docstring | Migration example + template comment |
| `internal/config/bootstrap.go` | 201, 205, 229 | Template strings | Config file comments (written to users' files) |
| `internal/provider/builtin.go` | 34, 59 | Comment | "was glm-5-turbo" historical reference |
| `internal/provider/manifest.go` | 139, 156 | Error messages | "e.g. \"zai/glm-5.2\"" in fmt.Errorf |
| `internal/provider/render.go` | 127, 249 | Error messages | "e.g. \"zai/glm-5.2\"" in fmt.Errorf |
| `internal/decompose/roles.go` | 163, 169 | Error messages | "e.g. \"zai/glm-5.2\"" in fmt.Errorf |

### Category A-2 — User-facing docs (MUST change)
| File | Lines | Content |
|------|-------|---------|
| `README.md` | 304, 309, 310, 313 | Quickstart git-config examples |
| `docs/cli.md` | 70, 172 | `--model` shadowing + `config init` docs |
| `docs/configuration.md` | 40, 232 | Config auto-detect + example |
| `docs/providers.md` | 72 | Multi-backend render example |

### Category B — Provider subsystem test fixtures (MUST change for consistency)
These exercise the prefix-fold mechanism using `zai/glm-5.2` or `glm-5-turbo` as test data. Input + assertion pairs must change in lockstep.

| File | Lines | Content |
|------|-------|---------|
| `internal/provider/render_test.go` | 87,91,152-154,298,306,312-314,478,488,567,572,605,610,641,659,685,726,747 | Render fold tests |
| `internal/provider/manifest_test.go` | 18,63,323,325,338,340,350,383 | ValidateModel + manifest parse |
| `internal/provider/merge_test.go` | 19,42,46,47,325 | Manifest merge |
| `internal/provider/builtin_test.go` | 436,439,884,890 | Pi render personal-override |
| `internal/provider/registry_test.go` | 65,70,71,215,222,315,341 | Registry DefaultModel |
| `internal/decompose/roles_test.go` | 318,323 | Role model validation |
| `internal/generate/realagent_test.go` | 40 | Real-agent test table |
| `internal/ui/output_test.go` | 250 | Label formatting |
| `internal/generate/multiturn_test.go` | 218,244,268,285,306 | Multiturn generation |

### Category C — Config migration test fixtures (MUST change in lockstep)
These test the v2→v3 config migration (prefix-fold `glm-5.2` → `zai/glm-5.2`). Changing the model token requires coordinated input+assertion edits across each test.

| File | Lines | Content |
|------|-------|---------|
| `internal/cmd/config_test.go` | 1350,1356,1379,1384,1396,1399,1405,1431,1439,1445,1456,1458,1461,1480,1505 | v2→v3 migration fixtures |
| `internal/config/migrate_test.go` | 22,26,27,39,44,45,54,59,60,69,73,74,85,86,89,90,92,93,122,126,127,138,139,142,143,151,155,156,164,169,170,178,182,183,191,195,196 | Migration unit tests |
| `internal/config/file_test.go` | 59,90,91,786,800,801 | Config file parse |

### Category D — Generic test fixtures (change for zero-hit goal)
These use `glm-5.2` as arbitrary test data where the literal value is incidental.

| File | Lines | Content |
|------|-------|---------|
| `internal/config/git_test.go` | 76, 104, 105 | Git-config model parse |
| `internal/config/load_test.go` | 148, 161, 162 | Config load env var |
| `internal/cmd/default_action_test.go` | 1500, 1532, 1539, 1540, 1541 | Default action dry-run |
| `internal/cmd/config_init_interactive_test.go` | 589, 593 | Interactive wizard test |
| `internal/cmd/providers_test.go` | 226, 241, 242 | Providers show output |

## Replacement Strategy

All `zai/glm-5.2` → `anthropic/claude-haiku` (matching spec §12.3).
All `glm-5-turbo` → `claude-haiku` (or `claude-sonnet-4` where the override context warrants).
All `zai/glm-5-turbo` → `anthropic/claude-haiku` (matching spec's generalization).

**Migration test fixtures:** `default_provider = "zai"` + `default_model = "glm-5.2"` → `default_provider = "anthropic"` + `default_model = "claude-haiku"` → folded output `model = "anthropic/claude-haiku"`.

## Acceptance Criterion
`rg -n 'zai/glm|glm-5\.2|glm-5-turbo|zai/glm-5' --glob '!spec/**' --glob '!plan/**'` returns **zero hits**.