# System Context: Config Layer Architecture

## Overview
This changeset fixes two bugs in the stagecoach configuration layer, both related to the
`config init --force` and config-version-advisory code paths. No structural architecture changes
are required — both fixes are surgical edits to existing functions.

## Key Files & Data Flow

### Config Bootstrap Pipeline (`config init`)
```
runConfigInit (internal/cmd/config.go:459)
  ├─ reads --provider flag (may be "")
  ├─ calls config.GenerateBootstrapConfig(providerName)  → returns fresh template TOML
  │    └─ GenerateBootstrapConfigWithOverrides(prov, nil)
  │         └─ buildBootstrapConfig(target, installed, nil)  → writes [defaults] + [role.*] blocks
  │              ├─ StagerFallback(target, models) → routes stager to first stager-capable provider
  │              └─ writeRoleBlock(b, "stager", stagerName, stagerModel, annotation)
  ├─ if --force: mergeExistingActiveSettings(path, content)
  │    └─ config.MergeActiveSettings(fresh, existing)  → preserves user's active settings verbatim
  │         └─ config.ActiveSettings(existing)  → regex scan: section→key→raw value
  └─ writeBootstrapFile(cmd, path, content, force)
       └─ config.WriteTimestampedBackup(path) (FR-B8 reversible-write)
```

### Config Load Pipeline (advisory notices)
```
Load (internal/config/load.go)
  ├─ parses TOML → cfg.ConfigVersion
  ├─ if version < CurrentConfigVersion: migrateV2ToV3 + migrationNotice  (handles older/missing)
  └─ else if configVersionNotice(fileLoaded, version) != "":  (handles newer-than-binary)
       └─ writes notice to noticeOut (stderr by default)
```

## Provider Capability Model (FR-D4)
- **Stager-capable providers** (non-empty `TooledFlags` in builtin.go): `pi`, `claude`
- **Non-stager providers** (nil `TooledFlags`): `agy`, `opencode`, `codex`, `cursor`, `qwen-code`
- When a provider cannot serve as stager, `StagerFallback()` routes the stager role to the first
  stager-capable provider in `preferredBuiltins` order (always resolves to `pi`).
- `pi` is a **multi-backend provider** (FR-R5b): its model must carry an inference backend as a
  slash-prefix (e.g. `zai/glm-5.2`). A bare model (no `/`) on pi is a HARD config error.

## Role Resolution (internal/config/roles.go)
`ResolveRoleModel(role, cfg)` applies per-field precedence:
```
[role.<role>].provider  →  cfg.Provider  (global [defaults] provider)
[role.<role>].model     →  cfg.Model     (global [defaults] model)
```
Fields are resolved INDEPENDENTLY: a role block with `provider = "pi"` but `model = ""` resolves
to `(provider="pi", model=cfg.Model)`. This is the trigger for BUG-001.

## Key Constants
- `CurrentConfigVersion = 3` (internal/config/config.go:20)
- `preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "qwen-code", "codex", "claude"}`
  (internal/config/bootstrap.go:11)

## Test Infrastructure
- Unit tests for pure functions: `internal/config/merge_test.go`, `load_test.go`, `bootstrap_test.go`
- Integration tests via `Execute(context.Background())` with real file I/O: `internal/cmd/config_test.go`
- Test helpers: `writeConfigFile(t, dir, name, content)`, `setupNoRepo(t)`, `loadEnvSetup(t)`
- `configVersionNotice` is tested as a pure function via table-driven tests (load_test.go:1962)

## Documentation Surfaces
- `docs/cli.md` §`config init` (line ~170), §`config upgrade` (line ~203)
- `docs/configuration.md` §Bootstrap (line ~35), §Schema versioning (line ~56)
- FR-B2/B4/B8 requirements referenced in code comments throughout `internal/config/`