# PFIX_M1_T003_S1 — BUG-003 Research Notes

## Bug Summary
`cmd/stagehand/providers.go` `newProvidersListCmd` (FR46) and
`newProvidersShowCmd` (FR47) call `config.Load(config.Flags{}, "")` with an
**EMPTY** `config.Flags` struct, so the env layer (FR34 layer 6) and CLI-flag
layer (FR34 layer 7) are never applied. Result: `--config`, `--provider`,
`--model`, and all `STAGEHAND_*` env vars are silently ignored by these two
subcommands. Only repo-local `.stagehand.toml` discovery works (because
`readRepoFile` uses the cwd / repoDir).

## Reproduction (CONFIRMED in this environment)
```
$ cat > /tmp/myconf_test.toml <<'EOF'
[provider.customagent]
command = "/bin/echo"
detect = "/bin/echo"
default_model = "test-model"
EOF
$ go run ./cmd/stagehand providers list --config /tmp/myconf_test.toml
# customagent is MISSING (only the 6 built-ins shown) — BUG
$ STAGEHAND_CONFIG=/tmp/myconf_test.toml go run ./cmd/stagehand providers list
# customagent still MISSING — BUG (env also ignored)
$ grep -c customagent <(STAGEHAND_CONFIG=/tmp/myconf_test.toml go run ./cmd/stagehand providers list)
0
```

## Root Cause (exact code)
`cmd/stagehand/providers.go`:
```go
// newProvidersListCmd RunE:
cfg, reg, _, err := config.Load(config.Flags{}, "")   // BUG: empty Flags, empty repoDir

// newProvidersShowCmd RunE:
_, reg, _, err := config.Load(config.Flags{}, "")     // BUG: empty Flags, empty repoDir
```

`config.Flags{}` means `Flags.Env` and `Flags.Flag` are both zero-value
`FlagsLayer` (every pointer nil) → `applyFlagsLayer` is a no-op → the env/flag
layers contribute nothing. `resolvedConfigPath(flags)` returns "" (both
ConfigPath pointers nil) → Load takes the normal global+repo discovery branch,
never the `--config` override branch.

## Reference Pattern (the correct wiring — ALREADY EXISTS in run.go)
`cmd/stagehand/run.go` `runDefault(cmd)`:
```go
flags, err := buildFlags(cmd)          // reads STAGEHAND_* env + persistent flags
if err != nil {
    fmt.Fprintln(os.Stderr, err)
    return ui.ExitError
}
cfg, reg, notice, err := config.Load(flags, ".")
```

`buildFlags(cmd)` (`cmd/stagehand/run.go`) is the FREE FUNCTION that:
- reads the six `STAGEHAND_*` env vars into `flags.Env` (FR34 layer 6)
- reads the persistent flags into `flags.Flag` via `cmd.Flags().Changed(name)`
  (FR34 layer 7)
- parses `STAGEHAND_TIMEOUT` (errors on unparseable → returned as `err`)
- returns `(config.Flags, error)`

It is in `package main`, so it is **directly callable from `providers.go` with
NO new import** (both files are `package main`).

## The Fix (mirror runDefault exactly)
Both RunE closures change from:
```go
X, reg, _, err := config.Load(config.Flags{}, "")
```
to:
```go
flags, err := buildFlags(cmd)
if err != nil {
    return err
}
X, reg, _, err := config.Load(flags, ".")
```

Notes:
- `buildFlags` returns `(config.Flags, error)`; the error path (e.g. bad
  `STAGEHAND_TIMEOUT`) is handled with `return err`, matching the existing
  `config.Load` error handling in the SAME RunE (cobra surfaces it → exit 1).
- repoDir `""` → `"."` to exactly mirror `runDefault`. These are equivalent for
  `readRepoFile` (`filepath.Join("", ".stagehand.toml")` ==
  `filepath.Join(".", ".stagehand.toml")` == `.stagehand.toml`) and for
  `readGitConfig` (`cmd.Dir = ""` and `"."` both resolve to cwd). `"."` is the
  canonical form used by the reference implementation.

## Why persistent flags ARE available on the subcommands
`cmd/stagehand/main.go` `init()` → `registerPersistentFlags(rootCmd)` registers
`--config`, `--provider`, `--model`, `--timeout`, `--all`, `--no-auto-stage`,
`--dry-run`, `--verbose`, `--no-color` as **PERSISTENT** flags on `rootCmd`.
Cobra propagates persistent flags to all descendants, so
`providers list`/`providers show` inherit them, and
`cmd.Flags().Changed("config")` works inside their RunE. (Confirmed by
`TestPersistentFlags_Registered` in run_test.go.)

## Stale comment to update
`newProvidersListCmd` doc comment currently says:
"the flags layer is empty because the persistent flags are not yet wired by
P1.M7.T2.S1" — this is now FALSE (P1.M7.T2 shipped the wiring). The fix updates
this comment to state that `buildFlags` now wires the env+flag layers, matching
`runDefault`.

## Test environment verification (hermeticity of regression test)
- No `.stagehand.toml` in repo root (verified: `ls` fails).
- No `stagehand.*` git config (verified: `git config --get-regexp stagehand\.`
  returns empty).
- Therefore `config.Load(flags, ".")` in a test run from the repo root is not
  polluted by real repo-local config; git-config can't define
  `[provider.*]` tables anyway, so it can't fabricate a false custom provider.

## Testing strategy (regression test)
The existing `providers_test.go` tests the PURE render helpers
(`renderProvidersList`, `showProviderManifest`) and deliberately avoids
`config.Load` I/O. A faithful regression test for THIS bug must prove the RunE
honors `--config`. Approach (white-box `package main`, mirrors the
`newTestCmd`/`registerPersistentFlags` pattern + `cmd.Execute()` from
`TestVersionShortCircuit`):
1. Write a temp config file defining a custom provider (e.g. `customagent`).
2. Build a fresh root `*cobra.Command`, `registerPersistentFlags(root)`,
   `root.AddCommand(newProvidersListCmd())`.
3. `root.SetOut(&buf)`; `root.SetArgs([]string{"providers","list","--config",path})`.
4. `root.Execute()`; assert `buf` contains `"customagent"`.
5. Mirror for `providers show customagent` → assert output contains the
   provider's `command` (proves the override manifest was loaded via --config).
6. Add a second sub-test using `t.Setenv("STAGEHAND_CONFIG", path)` with NO
   `--config` flag to prove the ENV path is also fixed.

Use a fabricated provider name like `customagent` (or
`definitely-not-an-agent-xyz` to match existing test style) with a
non-existent command so it shows as "not detected" but is still LISTED.

## DOCS Impact
Mode B (no per-item doc edit). PRD §15.2 already defines `--config` as a GLOBAL
persistent flag ("Path to a config file, overriding discovery"), and
docs/CONFIGURATION.md documents the flag precedence. The bug is that the CODE
did not honor what the docs already specify; the fix makes code match docs, so
no doc correction is required. (Mirrors BUG-002's Mode B resolution.)

## Validation commands verified working
- `go build ./...` → exit 0 (clean)
- `go vet ./cmd/stagehand/...` → exit 0 (clean)
- `go test ./cmd/stagehand/` → passes (cached)
- `gofmt -s -l internal/ cmd/ pkg/` → empty (formatted)
- `golangci-lint` is NOT installed in this environment; rely on `go vet` +
  `gofmt` as the static gates (matches what BUG-002's gates used).
