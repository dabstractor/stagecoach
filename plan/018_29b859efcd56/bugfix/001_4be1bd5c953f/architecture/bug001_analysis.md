# BUG-001: config init --force injects inconsistent [role.stager] provider

## Root Cause

### The Data Flow
When `config init --force` runs WITHOUT `--provider`:

1. `runConfigInit` (config.go:479) calls `config.GenerateBootstrapConfig("")` — empty string triggers
   FR-D1 auto-detection, which picks `pi` (highest-priority installed built-in).
2. `GenerateBootstrapConfig("")` → `buildBootstrapConfig("pi", ...)` writes:
   - `[defaults] provider = "pi"`
   - `[role.stager] provider = "pi"` + `model = ""` (pi is stager-capable; model blanked for multi-backend)
3. Then `mergeExistingActiveSettings(path, content)` calls `MergeActiveSettings(fresh, existing)`.
   - `fresh` has `[defaults] provider = "pi"` and `[role.stager] provider = "pi"`.
   - `existing` has `[defaults] provider = "claude"` and `model = "sonnet"`.
   - **Pass 1 (in-place replace)**: replaces `[defaults] provider = "pi"` → `provider = "claude"`.
     The `[role.stager] provider = "pi"` line in fresh is NOT touched because `existing` has no
     `[role.stager]` section at all.
4. **Result**: `[defaults] provider = "claude"` + `[role.stager] provider = "pi"` + `model = ""`.

### The Failure
When decompose runs on this config:
- `ResolveRoleModel("stager", cfg)` sees `[role.stager] provider = "pi"` → provider = "pi".
- `[role.stager] model = ""` → falls back to `cfg.Model` = "sonnet" (from `[defaults]`).
- Render hits FR-R5b: bare model "sonnet" on pi (no `/` prefix) → HARD error, exit 1.

### Why MergeActiveSettings Can't Fix It
`MergeActiveSettings` is designed to carry user values verbatim — it cannot infer that a `[role.stager]`
provider line that exists in `fresh` but not `existing` should be reconciled against the preserved
default. The inconsistency is introduced upstream: the template is generated for `pi` while the
preserved default is `claude`.

## Fix Design (Option A from PRD Recommendations)

**Re-target the template to the PRESERVED `[defaults] provider` before generating it.**

### Implementation Location
`internal/cmd/config.go` → `runConfigInit` function (line 459).

### Current Code (simplified)
```go
func runConfigInit(cmd *cobra.Command, args []string) error {
    // ...
    providerName, _ := cmd.Flags().GetString("provider")
    // ... validate providerName if non-empty ...
    content = config.GenerateBootstrapConfig(providerName)  // ← providerName="" on --force without --provider

    force, _ := cmd.Flags().GetBool("force")
    if force {
        content = mergeExistingActiveSettings(path, content)  // ← preserves defaults but not stager
    }
    // ... write ...
}
```

### Fixed Code (simplified)
```go
func runConfigInit(cmd *cobra.Command, args []string) error {
    // ...
    force, _ := cmd.Flags().GetBool("force")  // ← moved up
    providerName, _ := cmd.Flags().GetString("provider")
    // BUG-001: when --force refreshes an existing config with no --provider pin,
    // re-target the template to the PRESERVED [defaults] provider instead of auto-detecting pi.
    if force && providerName == "" {
        providerName = preservedDefaultProvider(path)
    }
    // ... validate providerName if non-empty ...
    content = config.GenerateBootstrapConfig(providerName)  // ← now uses preserved provider

    if force {
        content = mergeExistingActiveSettings(path, content)  // ← now reconciles cleanly
    }
    // ... write ...
}
```

### Helper Function: `preservedDefaultProvider`
A new function that:
1. Reads the existing config file at `path`.
2. Extracts the active `[defaults] provider` value via `config.ActiveSettings`.
3. Strips surrounding TOML quotes (ActiveSettings stores values verbatim, NOT unquoted).
4. Validates the extracted provider is a known built-in via `provider.NewRegistry(nil).Get(name)`.
5. Returns the validated provider name, or `""` (fall back to auto-detection) for absent/inert/unknown.

**Edge cases handled:**
- Existing file absent → return "" (auto-detect pi, same as today)
- Existing file inert (all-commented) → no active provider → return "" (auto-detect)
- Existing provider is a custom/unknown name → return "" (don't break custom providers)
- Existing provider is a known built-in (claude, codex, etc.) → return it (re-target template)

### Why This Works
When `GenerateBootstrapConfig("claude")` is called:
- Template has `[defaults] provider = "claude"`, `[role.stager]` inherits claude (no explicit provider
  line because claude IS stager-capable), `model = "sonnet"`.
- `MergeActiveSettings` then preserves all active settings — which are all already consistent.

### What This Does NOT Change
- `config init --force --provider pi` still explicitly targets pi (the `--provider` guard wins).
- First-run bootstrap (no existing file) still auto-detects.
- `config init --force --template` still writes the inert template.
- `config init` without `--force` still refuses to overwrite.

## Test Strategy
- **Unit test for `preservedDefaultProvider`**: pure function testing extraction + validation.
- **Integration test**: claude default → `config init --force` → verify NO `[role.stager] provider = "pi"`.
- **Regression test**: verify resulting config produces valid stager resolution (no FR-R5b error).
- Update existing test `TestConfigInit_Force_PreservesActiveSettings` (config_test.go:586) — it uses
  `--provider pi` explicitly so is unaffected, but add a variant WITHOUT `--provider`.

## Documentation Impact
- `docs/cli.md` §config init: note that `--force` re-targets to preserved provider.
- `docs/configuration.md` §Bootstrap: same clarification.
- Code comments in `config.go` and `bootstrap.go` referencing FR-B2/FR-B8.