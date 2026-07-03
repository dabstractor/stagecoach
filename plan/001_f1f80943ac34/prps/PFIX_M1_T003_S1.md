# PRP for PFIX_M1_T003_S1

## Objective

Fix BUG-003 (severity minor): the `stagehand providers list` (FR46) and `stagehand providers show <name>` (FR47) subcommands ignore the global `--config`/`--provider`/`--model` flags and all `STAGEHAND_*` environment variables (FR34 layers 6-7) because their RunE closures call `config.Load(config.Flags{}, "")` with an EMPTY Flags struct. The fix reuses the ALREADY-SHIPPED `buildFlags(cmd)` helper from `cmd/stagehand/run.go` (which `runDefault` uses as `buildFlags(cmd) -> config.Load(flags, ".")`) in both closures, so the env+CLI-flag precedence layers are applied and an explicit `--config myconfig.toml` (or `STAGEHAND_CONFIG=...`) loads that file's `[provider.*]` overrides. Minimal, pattern-faithful, no new imports (both files are `package main`), no regressions, plus a white-box regression test proving `--config` and `STAGEHAND_CONFIG` are honored.

## Context

## Goal

**Feature Goal**: BUG-003 is fixed: `stagehand providers list --config <path>` and `STAGEHAND_CONFIG=<path> stagehand providers list` both load the custom `[provider.*]` definitions from `<path>` (so a user-defined provider like `customagent` APPEARS in the listing), and `--provider`/`STAGEHAND_PROVIDER` correctly influence the resolved `(default)` marker. Equivalently the PRD §15.2 global-flag contract and the FR34 precedence chain (FR34 layers 6-7: env, then CLI flag) are honored by the providers subcommands exactly as they already are by the default action.

**Deliverable**: A focused patch to `cmd/stagehand/providers.go` — the two RunE closures (`newProvidersListCmd` and `newProvidersShowCmd`) are rewired from `config.Load(config.Flags{}, "")` to `buildFlags(cmd); config.Load(flags, ".")` (the exact `runDefault` pattern), and the now-stale doc comment on `newProvidersListCmd` ("the flags layer is empty because the persistent flags are not yet wired by P1.M7.T2.S1") is corrected. Plus white-box regression tests in `cmd/stagehand/providers_test.go` (executing the real command tree with `--config` and with `STAGEHAND_CONFIG`). No other files change; the pure render helpers (`renderProvidersList`, `showProviderManifest`, `resolveDefault`) and `buildFlags`/`config.Load` are byte-for-byte unchanged.

**Success Definition**: The reproduction from BUG-003 no longer reproduces — (a) `stagehand providers list --config /tmp/myconf.toml` (where myconf.toml defines `[provider.customagent]`) now SHOWS `customagent` in the listing; (b) `STAGEHAND_CONFIG=/tmp/myconf.toml stagehand providers list` likewise SHOWS `customagent`; (c) `stagehand --provider claude providers list` marks `claude` as the resolved default (proving the flag layer is applied). `go build ./...`, `go vet ./...`, `gofmt -s -l` (empty), and `go test ./...` all pass.

## Why

- PRD §15.2 defines `--config` as a GLOBAL persistent flag ("Path to a config file, overriding discovery"), and the FR34 precedence chain (PRD §16.1; decisions.md §6) mandates env (layer 6) then CLI flag (layer 7) apply across the WHOLE binary. The providers subcommands silently violate both.
- A user who defines a custom provider in an external config file and runs `stagehand --config myconfig.toml providers list` sees only the 6 built-ins — invisible custom provider, resolved default that ignores `--provider`/`STAGEHAND_PROVIDER`. Hostile UX for the exact debugging surface `providers show` exists to serve (US10).
- The fix is FREE: the wiring helper `buildFlags(cmd)` already shipped in P1.M7.T2 (used by `runDefault`); these two subcommands were simply never updated to call it. The code comment in `newProvidersListCmd` even admits the gap.
- Bug reference: BUG-003 (severity minor). Recorded in `plan/001_f1f80943ac34/bug_hunt_results.json` with a `suggestedFix` that this PRP implements verbatim.

## Root Cause

`cmd/stagehand/providers.go`:
```go
cfg, reg, _, err := config.Load(config.Flags{}, "")   
_, reg, _, err := config.Load(config.Flags{}, "")     

```
`config.Flags{}` means `Flags.Env` and `Flags.Flag` are both zero-value `FlagsLayer` (every pointer nil). In `internal/config/load.go` `Load`, `applyFlagsLayer(&cfg, flags.Env)` / `applyFlagsLayer(&cfg, flags.Flag)` are then no-ops, and `resolvedConfigPath(flags)` returns `""` (both `ConfigPath` pointers nil) so Load takes the normal global+repo discovery branch and NEVER the `--config` override branch. Consequently `--config`, `--provider`, `--model`, and every `STAGEHAND_*` var are invisible to these two subcommands. Only repo-local `.stagehand.toml` works, because `readRepoFile(repoDir)` reads `<repoDir>/.stagehand.toml` relative to the process cwd regardless of the flags.

## All Needed Context

### Documentation & References

```yaml
- file: cmd/stagehand/providers.go
  why: the TARGET FILE. Owns newProvidersListCmd + newProvidersShowCmd (the two buggy RunE closures) and the pure helpers renderProvidersList / showProviderManifest / resolveDefault (which must NOT change).
  pattern: RunE closures return error (RunE, not Run — cobra surfaces a non-nil err as exit 1 via main.go). The existing `config.Load` error is handled with `if err != nil { return err }` — the SAME shape the new `buildFlags` error must use.
  gotcha: newProvidersListCmd's doc comment contains the now-false line 'the flags layer is empty because the persistent flags are not yet wired by P1.M7.T2.S1' — it MUST be updated to reflect that buildFlags now wires the env+flag layers (P1.M7.T2 shipped it).
- file: cmd/stagehand/run.go
  why: the REFERENCE PATTERN. buildFlags(cmd) (the free function to reuse) and runDefault's `buildFlags(cmd) -> config.Load(flags, ".")` sequence. buildFlags reads STAGEHAND_* env into flags.Env and persistent flags via cmd.Flags().Changed(name) into flags.Flag; parses STAGEHAND_TIMEOUT (errors on unparseable); returns (config.Flags, error).
  pattern: runDefault handles the buildFlags error with `fmt.Fprintln(os.Stderr, err); return ui.ExitError` (it returns an int). providers RunE return error, so use the simpler `return err` (matches the in-closure config.Load error handling).
  gotcha: buildFlags is `package main` (same package as providers.go) — NO new import is needed to call it.
- file: cmd/stagehand/main.go
  why: registerPersistentFlags(rootCmd) registers --config/--provider/--model/--timeout/--all/--no-auto-stage/--dry-run/--verbose/--no-color as PERSISTENT flags on rootCmd, so they are INHERITED by the providers subcommands and cmd.Flags().Changed("config") works inside their RunE. This is WHY the fix works without any flag-registration change.
- file: internal/config/load.go
  why: config.Load(flags Flags, repoDir string) — the entry point. resolvedConfigPath(flags) picks Flag.ConfigPath over Env.ConfigPath; when non-empty it parses THAT file in place of global+repo (the --config override). applyFlagsLayer writes Provider/Model/Timeout/Verbose/NoColor from each layer. readGitConfig(repoDir) is called UNCONDITIONALLY (even under --config) but cannot express [provider.*] tables, so it cannot fabricate a false custom provider.
  gotcha: repoDir "" and "." are EQUIVALENT for readRepoFile (filepath.Join both yield .stagehand.toml) and readGitConfig (cmd.Dir "" vs "." both = cwd). Use "." to exactly mirror runDefault.
- file: cmd/stagehand/providers_test.go
  why: white-box `package main` test conventions; stdlib + cobra + internal/* + go-toml only, NO testify. The existing tests target the PURE render helpers and avoid config.Load I/O. The regression test for THIS bug must execute the real command tree (RunE) with --config, so it mirrors the newTestCmd/registerPersistentFlags + cmd.Execute() pattern from run_test.go (TestVersionShortCircuit) rather than the pure-helper style.
- file: cmd/stagehand/run_test.go
  why: newTestCmd(t) builds a fresh *cobra.Command via registerPersistentFlags (the hermetic flag-set builder); TestVersionShortCircuit shows the cmd.SetArgs/SetOut/cmd.Execute() pattern for driving a RunE end-to-end. Mirror BOTH for the providers regression test.
- docfile: PRD.md
  section: §15.2 (global flags table — --config 'Path to a config file, overriding discovery') + FR34/§16.1 (precedence chain layers 6-7) + FR46/FR47 (providers list/show)
  why: the contract this fix honors. §15.2 makes --config GLOBAL (applies to subcommands); the bug is code that did not honor this.
- file: plan/001_f1f80943ac34/bug_hunt_results.json
  why: BUG-003 entry — suggestedFix: 'Replace config.Load(config.Flags{}, "") in both subcommand RunE closures with the same buildFlags(cmd) -> config.Load(flags, ".") pattern used by runDefault.' This PRP implements it verbatim.

```

### Current vs Desired code at the two call sites

CURRENT (`cmd/stagehand/providers.go`):
```go
RunE: func(cmd *cobra.Command, _ []string) error {
    cfg, reg, _, err := config.Load(config.Flags{}, "")
    if err != nil {
        return err
    }
    detected := reg.Detect()
    name, model := resolveDefault(cfg, reg, detected)
    return renderProvidersList(cmd.OutOrStdout(), reg, detected, name, model)
},
RunE: func(cmd *cobra.Command, args []string) error {
    _, reg, _, err := config.Load(config.Flags{}, "")
    if err != nil {
        return err
    }
    return showProviderManifest(cmd.OutOrStdout(), reg, args[0])
},

```

DESIRED:
```go
RunE: func(cmd *cobra.Command, _ []string) error {
    flags, err := buildFlags(cmd)
    if err != nil {
        return err
    }
    cfg, reg, _, err := config.Load(flags, ".")
    if err != nil {
        return err
    }
    detected := reg.Detect()
    name, model := resolveDefault(cfg, reg, detected)
    return renderProvidersList(cmd.OutOrStdout(), reg, detected, name, model)
},
RunE: func(cmd *cobra.Command, args []string) error {
    flags, err := buildFlags(cmd)
    if err != nil {
        return err
    }
    _, reg, _, err := config.Load(flags, ".")
    if err != nil {
        return err
    }
    return showProviderManifest(cmd.OutOrStdout(), reg, args[0])
},

```

### Known Gotchas

```go

```

## Implementation Blueprint

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY cmd/stagehand/providers.go newProvidersListCmd RunE
  - REPLACE `cfg, reg, _, err := config.Load(config.Flags{}, "")` with:
      flags, err := buildFlags(cmd)
      if err != nil { return err }
      cfg, reg, _, err := config.Load(flags, ".")
  - FOLLOW pattern: cmd/stagehand/run.go runDefault (buildFlags(cmd) -> config.Load(flags, "."))
  - PRESERVE: the rest of the RunE body unchanged (detected := reg.Detect(); name, model :=
    resolveDefault(...); return renderProvidersList(...)).
  - NAMING/PLACEMENT: the `flags, err :=` shadow of err is fine in Go (new block reuse);
    keep `cfg` for resolveDefault downstream.
Task 2: MODIFY cmd/stagehand/providers.go newProvidersListCmd doc comment
  - UPDATE the stale sentence 'resolves cfg+reg via config.Load (defaults->global file->
    repo file->repo git-config; the flags layer is empty because the persistent flags are
    not yet wired by P1.M7.T2.S1)' to reflect that cfg+reg are now resolved via
    buildFlags(cmd) -> config.Load(flags, ".") (the full FR34 chain incl. env+flag layers,
    mirroring runDefault). Keep the rest of the comment (FR46 / detect / resolveDefault /
    renderProvidersList notes).
Task 3: MODIFY cmd/stagehand/providers.go newProvidersShowCmd RunE
  - REPLACE `_, reg, _, err := config.Load(config.Flags{}, "")` with:
      flags, err := buildFlags(cmd)
      if err != nil { return err }
      _, reg, _, err := config.Load(flags, ".")
  - FOLLOW pattern: same as Task 1 (runDefault).
  - PRESERVE: `return showProviderManifest(cmd.OutOrStdout(), reg, args[0])` unchanged.
  - NOTE: the newProvidersShowCmd doc comment ('resolves reg via config.Load...') stays
    accurate; no comment edit required there (it never claimed the flags layer was empty).
Task 4: ADD regression tests in cmd/stagehand/providers_test.go (white-box package main)
  - ADD TestProvidersList_HonorsConfigFlag: write a temp .toml defining a custom provider
    (e.g. [provider.customagent] command="customagent-bin" detect="customagent-bin"),
    build a fresh root *cobra.Command, registerPersistentFlags(root),
    root.AddCommand(newProvidersListCmd()), root.SetOut(&buf),
    root.SetArgs(["providers","list","--config",path]), root.Execute(); assert buf contains
    "customagent" (it is ABSENT today — the bug). Mirror the newTestCmd + cmd.Execute()
    pattern from run_test.go (TestVersionShortCircuit).
  - ADD TestProvidersList_HonorsConfigEnv: same as above but use t.Setenv("STAGEHAND_CONFIG",
    path) and SetArgs(["providers","list"]) (NO --config flag) — proves the ENV path is
    also fixed (FR34 layer 6).
  - ADD TestProvidersShow_HonorsConfigFlag: build the tree, SetArgs(["providers","show",
    "customagent","--config",path]); Execute; assert output contains the override command
    token (e.g. "customagent-bin") — proves show loads the override manifest via --config.
  - DO NOT modify existing tests (the pure-helper tests stay untouched).
  - GOTCHA: tests run from the repo root, which is a git repo with no .stagehand.toml and
    no stagehand.* git-config (verified), so config.Load(flags, ".") is not polluted; the
    fabricated custom provider is the ONLY [provider.*] source, guaranteeing the assertion
    is a true positive.

```

### Integration Points

```yaml
NO database / routes / config-schema changes.
NO new env vars or flags (the persistent flags already exist on rootCmd).
NO public API change (cmd package; nothing exported).
DOCS: Mode B (no per-item doc edit). PRD §15.2 already defines --config as a GLOBAL
  persistent flag and docs/CONFIGURATION.md documents the flag>env>git-config>file>default
  precedence; the bug was code that did not honor the docs, and this fix makes code match
  docs, so no doc correction is required. (Mirrors BUG-002's Mode B resolution.)

```

## Validation Loop

### Level 1 — Syntax/Type/Vet/Fmt (one command per gate)
- `go build ./...`
- `go vet ./...`
- `test -z "$(gofmt -s -l internal/ cmd/ pkg/)"`   # prints nothing & exits 0 when formatted

### Level 2 — Unit tests
- `go test ./cmd/stagehand/`   # the affected package (providers.go + providers_test.go)
- `go test ./...`              # full suite (must stay green; no regressions)

### Level 3 — End-to-end reproduction (proves BUG-003 is fixed)
Build the binary, write a temp config defining a custom provider, then:
- CONFIG FLAG: `stagehand providers list --config /tmp/myconf.toml` -> output CONTAINS the
  custom provider name (it was MISSING before the fix).
- CONFIG ENV: `STAGEHAND_CONFIG=/tmp/myconf.toml stagehand providers list` -> output CONTAINS
  the custom provider name (it was MISSING before the fix).
- PROVIDER FLAG: `stagehand --provider <builtin> providers list` -> the trailing
  'default provider:' line names the --provider value (proves the flag layer applies).
- SHOW: `stagehand providers show <custom> --config /tmp/myconf.toml` -> prints the
  override manifest TOML (proves show also honors --config).
- NO-REGRESSION (no flags): `stagehand providers list` from a dir whose .stagehand.toml
  defines the provider STILL lists it (repo-local discovery unchanged).

## Final Validation Checklist

### Technical
- [ ] `go build ./...` succeeds.
- [ ] `go vet ./...` is clean.
- [ ] `gofmt -s -l internal/ cmd/ pkg/` lists nothing.
- [ ] `go test ./...` is green (no regressions; new providers regression tests pass).

### Feature (BUG-003 / FR34 / §15.2)
- [ ] `providers list --config X` lists providers defined in X (custom provider APPEARS).
- [ ] `STAGEHAND_CONFIG=X providers list` likewise lists them (env path fixed).
- [ ] `--provider`/`STAGEHAND_PROVIDER` influences the resolved default marker.
- [ ] `providers show <name> --config X` prints the override manifest from X.
- [ ] repo-local `.stagehand.toml` discovery still works (no --config) — no regression.

### Code quality
- [ ] buildFlags is reused (not duplicated) — no new helper, no new import.
- [ ] config.Load / buildFlags / the pure render helpers are byte-for-byte unchanged.
- [ ] Both RunE closures mirror runDefault's `buildFlags(cmd) -> config.Load(flags, ".")`.
- [ ] The stale 'flags layer is empty' comment is corrected.
- [ ] Fix is minimal: only providers.go (2 RunE call sites + 1 comment) + providers_test.go.

## Anti-Patterns to Avoid
- Do NOT duplicate buildFlags logic inline (reuse the existing helper).
- Do NOT add a new import (buildFlags is already in package main).
- Do NOT change repoDir to anything other than "." (mirror runDefault exactly).
- Do NOT swallow the buildFlags error (return it so cobra surfaces exit 1).
- Do NOT touch config.Load, buildFlags, renderProvidersList, showProviderManifest, resolveDefault,
  or any other file (out of scope; regression risk).
- Do NOT broaden the fix into a refactor (e.g. extracting a shared provider-load helper) —
  keep it minimal and focused on the two call sites.

## DOCS Impact
No per-item doc edit is required (Mode B). PRD §15.2 already defines `--config` as a
GLOBAL persistent flag and docs/CONFIGURATION.md documents the
flag>env>git-config>file>default precedence. The bug was that the CODE did not honor
what the docs already specify; this fix makes the code match the docs, so no doc
correction is needed. Changeset-level docs already describe the post-fix behavior.

## Implementation Steps

1. MODIFY cmd/stagehand/providers.go newProvidersListCmd RunE: replace the single line `cfg, reg, _, err := config.Load(config.Flags{}, "")` with `flags, err := buildFlags(cmd)` + `if err != nil { return err }` + `cfg, reg, _, err := config.Load(flags, ".")`. Keep the rest of the RunE body byte-for-byte unchanged (detected := reg.Detect(); name, model := resolveDefault(cfg, reg, detected); return renderProvidersList(cmd.OutOrStdout(), reg, detected, name, model)). buildFlags is in package main (run.go) so NO new import is needed. The `flags, err :=` reuse of `err` in the same block is valid Go.
2. MODIFY cmd/stagehand/providers.go newProvidersListCmd doc comment: update the stale sentence that claims 'the flags layer is empty because the persistent flags are not yet wired by P1.M7.T2.S1' to state that cfg+reg are now resolved via buildFlags(cmd) -> config.Load(flags, ".") (the full FR34 chain including the env+flag layers, mirroring runDefault). Keep the rest of the comment (FR46, detect, resolveDefault, renderProvidersList notes) intact.
3. MODIFY cmd/stagehand/providers.go newProvidersShowCmd RunE: replace `_, reg, _, err := config.Load(config.Flags{}, "")` with `flags, err := buildFlags(cmd)` + `if err != nil { return err }` + `_, reg, _, err := config.Load(flags, ".")`. Keep `return showProviderManifest(cmd.OutOrStdout(), reg, args[0])` unchanged. The newProvidersShowCmd doc comment needs no edit (it never claimed the flags layer was empty).
4. ADD regression tests in cmd/stagehand/providers_test.go (white-box package main, NO testify, mirror the newTestCmd/registerPersistentFlags + cmd.Execute() pattern from run_test.go TestVersionShortCircuit): (a) TestProvidersList_HonorsConfigFlag — write a temp .toml defining [provider.customagent] (command/detect a non-existent binary so it shows 'not detected' but is still LISTED), build a fresh root *cobra.Command, registerPersistentFlags(root), root.AddCommand(newProvidersListCmd()), root.SetOut(&buf), root.SetArgs(["providers","list","--config",path]), root.Execute(), assert buf contains "customagent". (b) TestProvidersList_HonorsConfigEnv — same but t.Setenv("STAGEHAND_CONFIG", path) and SetArgs(["providers","list"]) with NO --config flag, asserting "customagent" appears. (c) TestProvidersShow_HonorsConfigFlag — root.AddCommand(newProvidersShowCmd()), SetArgs(["providers","show","customagent","--config",path]), Execute, assert output contains the override command token (e.g. "customagent-bin"). Do not modify existing pure-helper tests. NOTE: tests run from the repo root which is a git repo with no .stagehand.toml and no stagehand.* git-config (verified), so config.Load(flags, ".") is not polluted and the fabricated provider is the only [provider.*] source.

## Validation Gates

### Level 1: 1

go build ./...

### Level 2: 1

go vet ./...

### Level 3: 1

test -z "$(gofmt -s -l internal/ cmd/ pkg/)"

### Level 4: 2

go test ./cmd/stagehand/

### Level 5: 2

go test ./...

## Success Criteria

- [ ] BUG-003 no longer reproduces via the --config flag: with a temp config file defining [provider.customagent], `stagehand providers list --config <path>` (and `go run ./cmd/stagehand providers list --config <path>`) now LISTS customagent — it was MISSING before the fix. Satisfies PRD §15.2 (--config is a global flag) and FR34 layers 6-7.
- [ ] BUG-003 no longer reproduces via the env var: `STAGEHAND_CONFIG=<path> stagehand providers list` likewise LISTS customagent (the env layer is now applied — it was ignored before).
- [ ] `--provider`/`STAGEHAND_PROVIDER` now influence the resolved default in `providers list`: the trailing 'default provider:' line reflects the flag/env value (proves the flag layer is wired, not just --config).
- [ ] `providers show <name> --config <path>` prints the override manifest TOML from the explicit config file (proves show also honors --config, not just list).
- [ ] No regression in repo-local discovery: `providers list` with NO --config from a directory whose .stagehand.toml defines a provider still lists it (readRepoFile path unchanged).
- [ ] buildFlags is REUSED (not duplicated) with no new import (both files are package main); config.Load, buildFlags, and the pure render helpers (renderProvidersList, showProviderManifest, resolveDefault) are byte-for-byte unchanged.
- [ ] Both RunE closures exactly mirror runDefault's `buildFlags(cmd) -> config.Load(flags, ".")` pattern, and the stale 'flags layer is empty' comment on newProvidersListCmd is corrected.
- [ ] All gates green: `go build ./...`, `go vet ./...`, `gofmt -s -l internal/ cmd/ pkg/` (empty), `go test ./cmd/stagehand/`, and `go test ./...` pass with no regressions; new TestProvidersList_HonorsConfigFlag / TestProvidersList_HonorsConfigEnv / TestProvidersShow_HonorsConfigFlag pass.

## References

- cmd/stagehand/providers.go (TARGET: newProvidersListCmd + newProvidersShowCmd RunE closures calling config.Load(config.Flags{}, ""); pure render helpers renderProvidersList/showProviderManifest/resolveDefault to leave UNCHANGED; stale doc comment to correct)
- cmd/stagehand/run.go (REFERENCE PATTERN: buildFlags(cmd) free function to REUSE + runDefault's `buildFlags(cmd) -> config.Load(flags, ".")` sequence; buildFlags is package main so no new import)
- cmd/stagehand/main.go (registerPersistentFlags(rootCmd) registers --config/--provider/--model as PERSISTENT flags — WHY the fix works without flag-registration changes: subcommands inherit them and cmd.Flags().Changed works)
- internal/config/load.go (config.Load(flags Flags, repoDir string): resolvedConfigPath picks Flag.ConfigPath>Env.ConfigPath and parses THAT file in place of global+repo when set; applyFlagsLayer applies env then flag layers; readGitConfig runs unconditionally but cannot express [provider.*] tables)
- cmd/stagehand/providers_test.go (white-box package main test conventions; existing tests target PURE render helpers and avoid config.Load I/O — the regression test must instead drive the real RunE via cmd.Execute)
- cmd/stagehand/run_test.go (newTestCmd via registerPersistentFlags = hermetic flag-set builder; TestVersionShortCircuit = cmd.SetArgs/SetOut/cmd.Execute pattern to mirror for the providers regression test)
- PRD.md §15.2 (global flags table: --config 'Path to a config file, overriding discovery'), FR34/§16.1 (precedence chain layers 6-7 env/flag), FR46/FR47 (providers list/show) — the contract this fix honors
- plan/001_f1f80943ac34/bug_hunt_results.json (BUG-003 entry suggestedFix: replace config.Load(config.Flags{}, "") with buildFlags(cmd) -> config.Load(flags, ".") in both subcommand RunE closures)
- plan/001_f1f80943ac34/prps/research/PFIX_M1_T003_S1_research.md (full root-cause + reproduction + reference-pattern + hermeticity analysis)
