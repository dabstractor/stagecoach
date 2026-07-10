# Test Patterns & Documentation Map

## Test Conventions

The project follows standard Go testing with table-driven tests. Key patterns:

### Bootstrap Tests (`internal/config/bootstrap_test.go`)

- `TestBuildBootstrapConfig_Pi` — tests target==pi (all models blanked, no gpt-5.4 anywhere)
- `TestBuildBootstrapConfig_AgyStagerFallback` — tests target==agy (stager routed to pi)
- `TestBuildBootstrapConfig_OtherInstalledCommented` — tests commented-out blocks
- `TestBuildBootstrapConfig_ValidTOML` — table-driven TOML validity for (target, installed) combos
- Helper: `assertContains(t, content, substrs...)` — checks content contains all substrings

### Config Command Tests (`internal/cmd/config_test.go`)

- `TestConfigInit_Force*` — backup assertions via `filepath.Glob("config.toml.bak.*")`
- `TestConfigUpgrade_*` — upgrade behavior tests (content assertions only, NO backup assertions)
- Pattern: temp home dir → writeConfigFile → Execute command → assert stdout/stderr/file content

### Provider Tests (`internal/provider/manifest_test.go`)

- `TestValidateModel_BareModelOnProviderFlagProvider_Errors` — FR-R5b enforcement
- `TestValidateModel_SlashModelOnProviderFlagProvider_OK` — valid inference/model
- `TestValidateModel_DefaultModelNoSlash_Errors` — manifest default_model must be prefixed
- `TestValidateModel_BareModelOnSingleBackendProvider_OK` — claude "sonnet" is fine
- `TestValidateModel_NoModelOnProviderFlagProvider_OK` — empty model is OK (blank)
- `TestValidateModel_InvalidManifest_Errors` — invalid manifest catches first

### Token Gate Tests (`internal/git/tokengate_test.go`)

- Water-fill allocation, truncation, closed-loop re-trim behavior

### Config/Provider Decoupling Invariant

`internal/config` MUST NOT import `internal/provider`. This is enforced by the package structure.
Tests that need both must use an external test package (`package config_test`) or live in a
package that can import both.

## Documentation Files (for §5 Documentation Sync)

### docs/configuration.md
- Line 40: bootstrap section — "EXCEPT for pi, whose per-role models are left EMPTY"
- Line 54: interactive wizard — mentions pi multi-backend prefix
- Line 68: config_version migration — points at `config upgrade`
- Line 167: token_limit — "a closed-loop guarantee (§9.1 FR3j) that the payload never exceeds token_limit"
- **Mode A impact**: Issue 4 fix (floor rejection) should update line 167 to note the floor.

### docs/providers.md
- Line 72: FR-R5b explanation — "A model with no / on such a provider is a HARD configuration error"
- Line 125: bootstrap note — "EXCEPT for pi, whose per-role models are written EMPTY"
- Line 129: FR-D4 table — pi row shows bare gpt-5.4 models (compiled-in defaults, not bootstrap output)
- **Mode A impact**: Issue 2 fix (commented block) may need a note that commented pi blocks also
  use blank/prefixed models.

### docs/cli.md
- CLI flags documentation — unlikely to need updates for these bugfixes.

### docs/how-it-works.md
- Diff capture pipeline — may reference token_limit behavior.

### README.md
- Line 66: mentions token_limit "closed-loop guarantee"
- Line 242: mentions pi bare model error (FR-R5b)
- **Mode B impact**: final changeset-level docs task should review README for accuracy after fixes.

## Build & Test Commands

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/config/ -v -run TestBuildBootstrap
go test ./internal/cmd/ -v -run TestConfigUpgrade
go test ./internal/git/ -v -run TestTokenGate
go test ./internal/generate/ -v -run TestEdit
go test ./internal/provider/ -v -run TestValidateModel

# Build
go build ./cmd/stagecoach/
```
