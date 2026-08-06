# P1.M4.T1.S1 — cobra `upgrade` command + local flags + registration: findings

## §0 — The task in one paragraph

Add the `stagecoach upgrade` cobra command SURFACE: the command definition, its 9 LOCAL flags, a no-op
`PersistentPreRunE` (so it runs outside a git repo and never triggers `config.Load`'s bootstrap write —
FR-B3/FR-U12), `init()` registration (zero edits to root.go), and the `runUpgrade` RunE prologue (flag
validation + effective channel/source-repo resolution). The actual `--check`/normal/`--rollback` DISPATCH
is P1.M4.T2.S1 — so `runUpgrade` here is a COMPILABLE placeholder that does validation + resolution, then
returns (P1.M4.T2.S1 inserts the dispatch). Plus a registration test + the [Mode A] docs/cli.md addition.

## §1 — The template to clone: internal/cmd/lock.go (the no-op-PersistentPreRunE precedent)

`lock.go` is the EXACT structural template (the contract says "mirror internal/cmd/lock.go exactly"):
- `lockCmd` = `&cobra.Command{Use, Short, Long, SilenceErrors:true, SilenceUsage:true, PersistentPreRunE:
  func(*cobra.Command, []string) error { return nil }}` — the no-op OVERRIDES root's `config.Load`
  (cobra runs only the NEAREST ancestor's PersistentPreRunE; lock.go's package doc states this verbatim).
- `init() { rootCmd.AddCommand(lockCmd) }` — ZERO edits to root.go (the providers/hook/integrate pattern).
- Leaf commands set `RunE` directly (e.g. `lockStatusCmd.RunE = runLockStatus`).

For `upgrade` (a LEAF, not a group), define `upgradeCmd` with the no-op `PersistentPreRunE` + `RunE:
runUpgrade` + `Args: cobra.NoArgs`. The no-op `PersistentPreRunE` is MANDATORY: without it, root's
`PersistentPreRunE` runs `config.Load`, which (a) bootstraps a config file on first run (FR-B3 — upgrade
must NOT write) and (b) reads the per-repo `.stagecoach.toml` (FR-U12 — upgrade is repo-independent).

NOTE: `root.go`'s `shouldSkipConfigLoad` already returns true for `name=="upgrade"` — but that refers to
`config upgrade` (the schema migration). For the TOP-LEVEL `upgrade`, the no-op `PersistentPreRunE` is the
clean guarantee (root's `PersistentPreRunE` never runs); do NOT rely on `shouldSkipConfigLoad`.

## §2 — The 9 LOCAL flags (Flags(), NOT PersistentFlags — FR-U9/U10)

Bound to PACKAGE VARS (the root.go flag-binding convention), registered on `upgradeCmd.Flags()` (LOCAL —
they must NOT pollute the commit-path persistent flag set):

| flag | type | var | shorthand | note |
|---|---|---|---|---|
| `--check` | bool | flagCheck | `-c` | FR-U6 check-only |
| `--version` | string | flagTargetVersion | — | FR-U5 step 2 target pin |
| `--prerelease` | bool | flagPrerelease | — | = `--channel prerelease` (FR-U10) |
| `--force` | bool | flagForce | — | FR-U1 override mis-detected manager |
| `--rollback` | bool | flagRollback | — | FR-U8 |
| `--install-method` | string | flagInstallMethod | — | FR-U2 override |
| `--yes` | bool | flagYes | `-y` | FR-U9 skip confirmation |
| `--channel` | string | flagChannel | — | FR-U10 stable\|prerelease |
| `--source-repo` | string | flagSourceRepo | — | FR-U10 owner/repo |

**The `--version` collision is a NON-issue**: cobra's auto-`--version` is a LOCAL flag on whichever
command has `Version` set (rootCmd, via `rootCmd.Version = Version`). It is NOT inherited by subcommands.
`upgradeCmd` has no `Version` field → no auto-version flag → `upgradeCmd.Flags().String("version", ...)`
is clean. (A user running `stagecoach upgrade --version 1.2.3` sets flagTargetVersion; `stagecoach
--version` on root still prints the binary version. Two different commands, two different flags.)

## §3 — The consumed dependencies (LANDED — read-only)

- **`internal/exitcode`**: `UpdateAvailable = 6`, `ErrUpdateAvailable` sentinel, `New(code, err)`, `For(err)`.
  `runUpgrade`'s dispatch (P1.M4.T2) will return `exitcode.New(exitcode.UpdateAvailable, …)` for `--check`
  behind. THIS item does NOT produce exit 6 (the dispatch is a placeholder) — but the flags/validation are
  the surface that produces it. The `--check`/`-c` flag is wired here.
- **`config.LoadUpgradeConfig() (UpgradeConfig, error)`** (internal/config/upgrade.go): the DEDICATED
  global-only `[upgrade]` reader. Returns `Defaults().Upgrade` (`Channel:"stable"`,
  `SourceRepo:"dabstractor/stagecoach"`) on a missing file — NO bootstrap write, NO error (FR-B3 boundary).
  `runUpgrade` calls this for the channel/source-repo resolution (flag > LoadUpgradeConfig > Defaults).
- **`config.UpgradeConfig`** struct: `Channel string`, `SourceRepo string`.
- **`config.GlobalConfigPath()`** (file.go:112, EXPORTED): the global config path — used by the
  registration test to assert no bootstrap file was written.

## §4 — runUpgrade: validation + resolution + PLACEHOLDER dispatch

`runUpgrade` (defined HERE so the command compiles; the dispatch body is P1.M4.T2.S1):
1. **validateUpgradeFlags(cmd.Flags())** — the 3 contract rules:
   - `--version` + `--prerelease` mutually exclusive (both Changed → error).
   - `--rollback` exclusive with `--check` and `--version` (rollback + either Changed → error).
   - `--channel` rejects unknown values: `flagChannel != "" && flagChannel != "stable" && flagChannel !=
     "prerelease"` → error. (Zero default "" ⇒ "not set"; the enum check is safe on the package var.)
   - Returns `exitcode.New(exitcode.Error, err)` on violation. Bool mutex rules use `fs.Changed` (a
     `--check=false` is indistinguishable from "not set" on the package var alone).
2. **Resolve effective channel/source-repo** (flag > LoadUpgradeConfig > Defaults; NO env for these —
   FR-U10 lists no `STAGECOACH_CHANNEL`): `effChannel = flagChannel if set else ("prerelease" if
   flagPrerelease) else uc.Channel`; `effSourceRepo = flagSourceRepo if set else uc.SourceRepo`.
3. **PLACEHOLDER dispatch** (P1.M4.T2.S1 replaces): a clearly-marked `// TODO(P1.M4.T2.S1): dispatch
   --check/normal/--rollback to internal/upgrade` + a notice that USES effChannel/effSourceRepo (so the
   locals aren't "declared and not used") + `return nil`. P1.M4.T2.S1 removes the notice, keeps the
   validation + resolution, and inserts the real dispatch.

`runUpgrade` NEVER `os.Exit` (returns `exitcode.New(...)`; main maps via `exitcode.For`). Mirrors every
other RunE in internal/cmd.

## §5 — The registration test (internal/cmd/upgrade_test.go, package cmd)

Mirrors root_test.go's `saveRootState`/`restoreRootState` (line 182/188) + the cobra-execute-into-a-buffer
idiom (hook_test.go/lock_test.go). Four cases:
1. **TestUpgradeCommand_Registered**: `rootCmd.Find([]string{"upgrade"})` → non-nil `*cobra.Command`,
   `.Name()=="upgrade"`. (Registration via `init()`.)
2. **TestUpgradeCommand_Flags**: `upgradeCmd.Flags().Lookup(<name>)` non-nil for all 9 flags; `-c` and
   `-y` shorthands resolve (LookupShorthand).
3. **TestUpgradeCommand_NoBootstrapOutsideRepo** (the FR-B3/FR-U12 proof): `t.Setenv("HOME", tempDir)`;
   `t.Setenv("XDG_CONFIG_HOME","")`; assert `config.GlobalConfigPath()` does NOT exist; `saveRootState`;
   `rootCmd.SetArgs([]string{"upgrade"})`; `Execute(ctx)`; restore. Assert `config.GlobalConfigPath()`
   STILL does not exist (the no-op `PersistentPreRunE` prevented `config.Load`'s bootstrap write). Tolerate
   any non-config error from the placeholder (the invariant is "no bootstrap", not "exit 0").
4. **TestUpgradeCommand_HelpDisambiguates** (the naming-collision guard): `upgradeCmd.Long` (or
   `--help` output) contains a phrase distinguishing binary self-update from `config upgrade` (config-
   schema migration). Plus `--help` short-circuits before `PersistentPreRunE` (cobra), so it never
   bootstraps either.
5. (optional) **TestUpgradeCommand_FlagValidation**: set `--version X --prerelease` via a local FlagSet →
   `validateUpgradeFlags` errors; `--rollback --check` → errors; `--channel bogus` → errors.

## §6 — The [Mode A] docs/cli.md addition

- Add a `### upgrade` subsection under "## Subcommands" (after `### lock status`, ~line 397): the
  command, all 9 flags, the "distinct from `config upgrade` (config-schema migration)" disambiguation, the
  "fetches ONLY this project's GitHub release artifacts + checksums; the one named §19 network exception"
  note (FR-U12 / v3_scope_boundary), and exit codes 0/1/6.
- Update the "## Exit codes" table (~line 400): add a `6` row — "Update available (`stagecoach upgrade
  --check`); upgrade-path only, never the commit path."

## §7 — Scope fences (walled off, FR-U12)

- Touches ONLY: `internal/cmd/upgrade.go` (NEW), `internal/cmd/upgrade_test.go` (NEW), `docs/cli.md` (EDIT).
- ZERO edits to `root.go` (registration via `init()`→`rootCmd.AddCommand`). ZERO edits to `internal/upgrade/*`
  (the dispatch consumers are P1.M4.T2/T3). ZERO edits to `config.Load`/`exitcode` (LANDED). ZERO edits to
  the commit path (FR-U12). `runUpgrade`'s dispatch is a PLACEHOLDER — P1.M4.T2.S1 owns the real body.
- Parallel-safe: P1.M3.T3.S1 (`--rollback` primitive) is in `internal/upgrade/` — DIFFERENT package, zero
  overlap. P1.M4.T2.S1 (runUpgrade dispatch) builds ON this item's `runUpgrade` (replaces the placeholder).

## §8 — Validation

- `go build ./...`; `go vet ./internal/cmd/...`; `gofmt -l` on the 2 new files.
- `go test ./internal/cmd/ -run 'TestUpgradeCommand' -v` green.
- `make test` + `make lint` clean (the new flags are all used by runUpgrade/validateUpgradeFlags; the
  placeholder uses effChannel/effSourceRepo so no unused-local).
- grep guards: no-op PersistentPreRunE present; 9 flags on LOCAL Flags(); registration via init();
  runUpgrade never os.Exit; scope (only the 3 files).