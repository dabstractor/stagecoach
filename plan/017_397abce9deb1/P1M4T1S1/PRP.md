name: "P1.M4.T1.S1 — cobra `upgrade` command + local flags + no-op PersistentPreRunE + registration (FR-U9/U10/U12)"
description: >
  The CLI SURFACE for `stagecoach upgrade` (PRD §9.29, the v3.0 self-update command). A NEW
  `internal/cmd/upgrade.go` defines `upgradeCmd` (Use:"upgrade"; Short; Long that DISAMBIGUATES from
  `config upgrade` — config-schema migration vs binary self-update; a no-op PersistentPreRunE mirroring
  internal/cmd/lock.go EXACTLY so it runs outside a git repo and never triggers config.Load's first-run
  bootstrap write, FR-B3/FR-U12) + the 9 FR-U9/U10 LOCAL flags (Flags(), NOT PersistentFlags — they must
  NOT pollute the commit-path flag set) bound to package vars + `init(){ rootCmd.AddCommand(upgradeCmd) }`
  (ZERO edits to root.go, the providers/hook/integrate/lock pattern) + `runUpgrade` RunE. The 9 flags:
  --check/-c, --version <v> (target pin — NOT cobra's auto-version; upgradeCmd has no Version field so no
  collision), --prerelease (=-channel prerelease), --force, --rollback, --install-method <m>, --yes/-y,
  --channel <stable|prerelease>, --source-repo <owner/repo>. `runUpgrade` does the flag VALIDATION (3
  contract rules: --version+--prerelease mutex; --rollback exclusive with --check/--version; --channel
  rejects unknown) + resolves effective channel/source-repo (flag > config.LoadUpgradeConfig() > Defaults;
  NO env for channel/source-repo per FR-U10). The actual --check/normal/--rollback DISPATCH is P1.M4.T2.S1,
  so `runUpgrade`'s dispatch body here is a COMPILABLE PLACEHOLDER (validation + resolution stand; a
  clearly-marked TODO + notice + return nil that P1.M4.T2.S1 replaces with the real dispatch). runUpgrade
  NEVER os.Exit (returns exitcode.New(...); main maps via exitcode.For — mirrors every RunE). CONSUMES the
  LANDED P1.M1 foundation: exitcode.UpdateAvailable=6/ErrUpdateAvailable + config.LoadUpgradeConfig/
  UpgradeConfig (global-only [upgrade] reader, no bootstrap). Plus a NEW registration test
  (internal/cmd/upgrade_test.go): rootCmd finds 'upgrade'; all 9 flags + shorthands present; running
  `upgrade` outside a git repo does NOT bootstrap-write a config (the no-op PersistentPreRunE proof,
  FR-B3); --help/Long disambiguates from `config upgrade`; flag-validation mutex/enum rules. Plus the
  [Mode A] docs/cli.md addition: a `### upgrade` Subcommands subsection (flags + disambiguation + the
  GitHub-Releases-only §19 network note) + a `6` row in the Exit codes table. SCOPE: internal/cmd/upgrade.go
  (NEW) + internal/cmd/upgrade_test.go (NEW) + docs/cli.md (EDIT). ZERO edits to root.go, internal/upgrade/*
  (the dispatch is P1.M4.T2/T3), config.Load, exitcode.go, or the commit path (FR-U12 walled off).
  Parallel-safe: P1.M3.T3.S1 (--rollback primitive) is internal/upgrade/ (different package, zero overlap);
  P1.M4.T2.S1 (runUpgrade dispatch) BUILDS ON this item (replaces the placeholder).

---

## Goal

**Feature Goal**: Add the `stagecoach upgrade` cobra command SURFACE (PRD §9.29, FR-U9/U10/U12) — the
command definition, its 9 LOCAL flags, a no-op `PersistentPreRunE` (repo-independent, no bootstrap write),
`init()` registration, and the `runUpgrade` RunE prologue (flag validation + effective channel/source-repo
resolution) — walled off from the commit path. This is the CLI shell that P1.M4.T2.S1's dispatch
(--check/normal/--rollback) plugs into.

**Deliverable**:
1. `internal/cmd/upgrade.go` (NEW) — `upgradeCmd` (`&cobra.Command` with no-op `PersistentPreRunE`, `RunE:
   runUpgrade`, `Args: cobra.NoArgs`) + 9 flag package vars + the flags registered on `upgradeCmd.Flags()`
   (LOCAL) + `init(){ rootCmd.AddCommand(upgradeCmd) }` + `runUpgrade` (validation + resolution +
   PLACEHOLDER dispatch) + `validateUpgradeFlags(fs)` helper.
2. `internal/cmd/upgrade_test.go` (NEW) — registration, flags-present, no-bootstrap-outside-repo,
   help-disambiguates, and flag-validation tests.
3. `docs/cli.md` (EDIT, Mode A) — `### upgrade` Subcommands subsection + `6` row in Exit codes.

**Success Definition**:
- `stagecoach upgrade` is registered (rootCmd finds it) and shows all 9 flags in `--help`.
- It runs OUTSIDE a git repo with NO global config and does NOT bootstrap-write a config file (the no-op
  `PersistentPreRunE` prevented `config.Load`'s FR-B3 write) — the FR-U12 wall.
- Its `Long`/`--help` disambiguates binary self-update from `config upgrade` (config-schema migration).
- Flag validation enforces the 3 contract rules (--version+--prerelease mutex; --rollback vs
  --check/--version mutex; --channel enum).
- `runUpgrade` resolves effective channel/source-repo (flag > LoadUpgradeConfig > Defaults) and is a
  COMPILABLE placeholder for the dispatch (P1.M4.T2.S1 replaces the placeholder; validation + resolution
  stay). `runUpgrade` never `os.Exit`.
- `go build ./...` clean; `go vet ./internal/cmd/...` clean; `gofmt -l` empty; `go test ./internal/cmd/
  -run TestUpgradeCommand` green; `make test` + `make lint` clean.
- `git status --porcelain` == the 3 files (2 new + docs/cli.md). ZERO edits to root.go, internal/upgrade/*,
  config.Load, exitcode.go, the commit path, or any PRD/task file.

## User Persona (if applicable)

**Target User**: A Stagecoach user who wants to update the binary: `stagecoach upgrade` (normal),
`stagecoach upgrade --check` (CI/cron gate, exit 6 if behind), or `stagecoach upgrade --rollback` (undo).
**Use Case**: keep stagecoach current via its own command (the v3.0 delegate-first updater) without
re-running the install channel manually.
**User Journey**: `stagecoach upgrade --check` → "update available: X → Z" (exit 6) → `stagecoach upgrade`
→ detects install method, delegates/swaps, confirms (FR-U9) → `stagecoach --version` reports Z. (The
dispatch is P1.M4.T2; THIS item is the command shell + flags + validation the user types.)
**Pain Points Addressed**: FR-U1/U9/U10/U12 — a single, walled-off upgrade command whose flags are LOCAL
(not polluting the commit path), that runs anywhere (no git repo / no config needed), and is unmistakably
distinct from `config upgrade`.

## Why

- **PRD §9.29 / §10.5**: v3.0 promotes self-update to a core command. This item is its CLI surface — the
  shell P1.M4.T2's dispatch plugs into. Without it there is no `stagecoach upgrade` to call.
- **FR-U12 (walled off)**: the no-op `PersistentPreRunE` is the structural guarantee that `upgrade`
  acquires no lock, reads no repo, invokes no provider, and never bootstraps a config — it is repo-
  independent by construction (mirrors lock.go).
- **FR-U9/U10 (flags)**: the 9 flags are LOCAL to `upgradeCmd` so they do NOT appear on the commit path
  (`stagecoach --help` / `stagecoach commit-flags`). This is the established LOCAL-flags convention
  (config init's `--template`, models' `--all`) applied to a repo-independent subcommand.
- **Naming collision guard**: `stagecoach upgrade` (binary self-update) vs `stagecoach config upgrade`
  (config-schema migration) are TWO DIFFERENT commands. The `Long`/`--help` MUST disambiguate (the v3
  scope boundary flags this explicitly).
- **Bounded, parallel-safe**: 2 new files + 1 doc edit. The LANDED P1.M1 foundation (exitcode 6,
  LoadUpgradeConfig) is CONSUMED. P1.M3.T3.S1 (internal/upgrade rollback) is a different package. P1.M4.T2.S1
  builds on this item's `runUpgrade` (replaces the placeholder dispatch).

## What

A new `stagecoach upgrade` cobra command, repo-independent (no-op `PersistentPreRunE`), with 9 LOCAL flags,
validated + resolved in `runUpgrade` (dispatch placeholder pending P1.M4.T2.S1), registered via `init()`,
documented in docs/cli.md.

### Success Criteria
- [ ] `internal/cmd/upgrade.go` defines `upgradeCmd` (`Use:"upgrade"`, `Short`, `Long` disambiguating from
      `config upgrade`, `SilenceErrors:true`, `SilenceUsage:true`,
      `PersistentPreRunE: func(*cobra.Command, []string) error { return nil }` (mirror lock.go),
      `Args: cobra.NoArgs`, `RunE: runUpgrade`).
- [ ] The 9 flags are registered on `upgradeCmd.Flags()` (LOCAL — NOT PersistentFlags), bound to package
      vars: `--check/-c` (flagCheck bool), `--version` (flagTargetVersion string), `--prerelease`
      (flagPrerelease bool), `--force` (flagForce bool), `--rollback` (flagRollback bool), `--install-method`
      (flagInstallMethod string), `--yes/-y` (flagYes bool), `--channel` (flagChannel string),
      `--source-repo` (flagSourceRepo string).
- [ ] `init()` calls `rootCmd.AddCommand(upgradeCmd)` — ZERO edits to root.go.
- [ ] `runUpgrade(cmd, args)`: calls `validateUpgradeFlags(cmd.Flags())` (returns `exitcode.New(
      exitcode.Error, err)` on violation); resolves `effChannel`/`effSourceRepo` (flag > LoadUpgradeConfig >
      Defaults); then a clearly-marked PLACEHOLDER (`// TODO(P1.M4.T2.S1): dispatch --check/normal/
      --rollback`) that USES effChannel/effSourceRepo (no unused-local) + `return nil`. NEVER `os.Exit`.
- [ ] `validateUpgradeFlags(fs *pflag.FlagSet) error`: (1) `--version`+`--prerelease` both `Changed` →
      error; (2) `--rollback` `Changed` AND (`--check` OR `--version` `Changed`) → error; (3) `flagChannel`
      not in {"","stable","prerelease"} → error.
- [ ] `internal/cmd/upgrade_test.go`: TestUpgradeCommand_Registered (rootCmd.Find), _Flags (all 9 +
      shorthands via Lookup), _NoBootstrapOutsideRepo (temp HOME + empty XDG; `config.GlobalConfigPath()`
      absent before AND after `Execute(["upgrade"])`), _HelpDisambiguates (Long mentions distinct from
      `config upgrade`), _FlagValidation (the 3 mutex/enum rules).
- [ ] `docs/cli.md` (Mode A): `### upgrade` subsection (flags + disambiguation + GitHub-Releases-only §19
      note + exit 0/1/6) under Subcommands; `6` row added to the Exit codes table.
- [ ] `go build ./...` clean; `go vet ./internal/cmd/...` clean; `gofmt -l` empty on the 2 new files.
- [ ] `go test ./internal/cmd/ -run TestUpgradeCommand -v` green; `make test` + `make lint` clean.
- [ ] `git status --porcelain` == `internal/cmd/upgrade.go` + `internal/cmd/upgrade_test.go` + `docs/cli.md`.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim lock.go template (the no-op-PersistentPreRunE + init()-registration structure to
clone), the 9 flags with types/shorthands/vars, the `--version` non-collision proof, the 3 validation rules
+ why bool mutex needs `fs.Changed`, the resolution order (flag > LoadUpgradeConfig > Defaults; no env),
the runUpgrade PLACEHOLDER design (validation+resolution stand; P1.M4.T2.S1 replaces the dispatch), the
LANDED consumed APIs (exitcode.New/For, config.LoadUpgradeConfig, config.GlobalConfigPath), the test idiom
(saveRootState/restoreRootState + Execute-into-buffer + the no-bootstrap assertion), the docs/cli.md
placement + exit-code-row edit, the scope fences, and the grep guards.

### Documentation & References

```yaml
# MUST READ — the codebase-specific findings (the design + the lock.go template + the placeholder rationale).
- docfile: plan/017_397abce9deb1/P1M4T1S1/research/findings.md
  why: "§1 lock.go is the EXACT template (no-op PersistentPreRunE + init() registration); §2 the 9 flags +
        the --version non-collision proof; §3 the LANDED consumed APIs (exitcode.UpdateAvailable/ErrUpdateAvailable,
        config.LoadUpgradeConfig/UpgradeConfig/GlobalConfigPath); §4 runUpgrade validation+resolution+placeholder;
        §5 the 5 registration tests; §6 the docs/cli.md edit; §7 scope fences; §8 validation."

# MUST READ — the template to CLONE (internal/cmd/lock.go — the no-op PersistentPreRunE + registration).
- file: internal/cmd/lock.go
  why: "The verbatim structure: `var lockCmd = &cobra.Command{Use, Short, Long, SilenceErrors:true,
        SilenceUsage:true, PersistentPreRunE: func(*cobra.Command, []string) error { return nil }}` +
        `func init() { rootCmd.AddCommand(lockCmd) }`. Its package doc explains the no-op OVERRIDES root's
        config.Load (cobra runs only the nearest ancestor's PersistentPreRunE). upgradeCmd is the LEAF
        twin (RunE directly on it; Args: cobra.NoArgs)."
  pattern: "no-op PersistentPreRunE + SilenceErrors+SilenceUsage + init()→rootCmd.AddCommand. The RunE
            returns exitcode.New(...) (never os.Exit)."
  gotcha: "upgrade is a LEAF (no subcommands) — set RunE + Args directly on upgradeCmd (lock is a GROUP
           with a leaf; upgrade has no children). The no-op PersistentPreRunE is MANDATORY (without it
           root's config.Load runs → bootstrap write (FR-B3) + per-repo read (FR-U12))."

# MUST READ — root.go (what the no-op OVERRIDES + the flag-registration convention to NOT follow here).
- file: internal/cmd/root.go
  why: "rootCmd's PersistentPreRunE (line ~165) calls config.Load (which bootstraps on a missing global
        file, FR-B3). upgrade's no-op PersistentPreRunE OVERRIDES it (cobra runs only the nearest). The
        flag-binding convention: root.go registers PERSISTENT flags on rootCmd.PersistentFlags() bound to
        package vars — BUT upgrade's flags are LOCAL (upgradeCmd.Flags()), NOT persistent (they must NOT
        pollute the commit-path flag set). shouldSkipConfigLoad returns true for name=='upgrade' but that
        refers to `config upgrade` — do NOT rely on it for the top-level upgrade (the no-op PreRunE is the
        guarantee)."
  critical: "ZERO edits to root.go. Register via init()→rootCmd.AddCommand(upgradeCmd) in upgrade.go. Do
             NOT add upgrade's flags to rootCmd.PersistentFlags()."

# MUST READ — models.go (the LOCAL-flag-on-a-leaf precedent + the inherited-flag-shadow note).
- file: internal/cmd/models.go
  why: "modelsCmd is a LEAF with a LOCAL flag (`modelsCmd.Flags().BoolVarP(&flagModelsAll, 'all', 'a', ...)`)
        registered in its own init(). Clone this LOCAL-flags structure for upgrade. (models also documents
        the inherited-persistent-shadow mechanism — NOT needed for upgrade since none of its 9 flag names
        collide with root's persistents, EXCEPT note --version is NOT inherited (cobra auto-version is
        local to the command with Version set).)"
  pattern: "`var xCmd = &cobra.Command{...}` + `var flagX bool` + `func init(){ xCmd.Flags().BoolVarP(...);
            rootCmd.AddCommand(xCmd) }`. upgrade follows this for all 9 flags."

# MUST READ — the LANDED config.LoadUpgradeConfig (the global-only [upgrade] reader runUpgrade calls).
- file: internal/config/upgrade.go
  why: "`func LoadUpgradeConfig() (UpgradeConfig, error)` — reads ONLY the global file; returns
        Defaults().Upgrade (Channel:'stable', SourceRepo:'dabstractor/stagecoach') on a missing file with
        NO write + NO error (FR-B3 boundary). runUpgrade calls this for the channel/source-repo resolution
        (flag > LoadUpgradeConfig > Defaults). It is the DEDICATED reader, deliberately NOT config.Load."
  gotcha: "Do NOT call config.Load from runUpgrade (it would bootstrap + read per-repo). Call
           LoadUpgradeConfig ONLY. It never writes."

# MUST READ — the LANDED exitcode package (New/For/UpdateAvailable — runUpgrade's error discipline).
- file: internal/exitcode/exitcode.go
  why: "`exitcode.New(code, err)` wraps an error with a forced code; `For(err)` maps. UpdateAvailable=6 +
        ErrUpdateAvailable sentinel (FR-U6) exist. runUpgrade's VALIDATION errors use exitcode.New(
        exitcode.Error, err). (The dispatch in P1.M4.T2 will use exitcode.New(exitcode.UpdateAvailable, …)
        for --check-behind; THIS item does not produce exit 6 — the placeholder returns nil.)"
  pattern: "Every RunE returns exitcode.New(exitcode.Error, fmt.Errorf('stagecoach: %w', err)) on failure,
            nil on success. NEVER os.Exit (only main does, via exitcode.For)."

# MUST READ — the FR-U9/U10/U12 spec (the flag semantics + the walled-off contract).
- docfile: plan/017_397abce9deb1/prd_snapshot.md
  section: "§9.29 FR-U1–U12 (lines ~590–625) + §15.4 exit codes"
  why: "FR-U9 (--yes/-y confirmation skip), FR-U10 (--channel/--source-repo + [upgrade] config; --prerelease
        = --channel prerelease), FR-U12 (walled off; exit 0/1/6). Pins the 9 flags + the exit codes this
        command surface produces."

# MUST READ — the v3 scope boundary (the negative-space checklist — what upgrade MUST NOT do).
- docfile: plan/017_397abce9deb1/architecture/v3_scope_boundary.md
  why: "Confirms: no run lock, no repo read, no provider invoke, no config.Load bootstrap (no-op PreRunE),
        no auto-elevate. Exit codes 0/1/2/3/5/124 semantics UNCHANGED — only ADD 6. The existing `config
        upgrade` is UNTOUCHED (do not rename/repurpose). The help text MUST disambiguate the two."

# CONTEXT — the test-helpers to REUSE (saveRootState/restoreRootState + the Execute-into-buffer idiom).
- file: internal/cmd/root_test.go
  why: "`saveRootState(t)` / `restoreRootState(t, ...)` (line 182/188) snapshot rootCmd's args/out/err/RunE
        so a test can SetArgs+Execute without leaking state into siblings. Clone hook_test.go/lock_test.go's
        `rootCmd.SetOut(&buf); rootCmd.SetErr(&buf); rootCmd.SetArgs(...); Execute(ctx)` idiom."

# CONTEXT — docs/cli.md (the [Mode A] edit target: Subcommands + Exit codes sections).
- file: docs/cli.md
  why: "Subcommands section (line 72) — add `### upgrade` after `### lock status` (~line 397). Exit codes
        table (line 400) — add the `6` row. The existing `### config upgrade` entry (line 203) is the
        config-schema migration — the new `### upgrade` entry MUST cross-reference/disambiguate it."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  root.go               # READ-ONLY — rootCmd + PersistentPreRunE (config.Load); ZERO edits (register via init())
  lock.go               # READ-ONLY — the no-op-PersistentPreRunE + init()-registration TEMPLATE to clone
  models.go             # READ-ONLY — the LOCAL-flag-on-a-leaf precedent
  config.go             # READ-ONLY — configUpgradeCmd (config-schema migration; the name collision to disambiguate)
  hook.go, integrate.go, providers.go  # READ-ONLY — other init()-registration precedents
  upgrade.go            # NEW — upgradeCmd + 9 flags + init() + runUpgrade + validateUpgradeFlags
  upgrade_test.go       # NEW — registration/flags/no-bootstrap/disambiguation/validation tests
  root_test.go          # READ-ONLY — saveRootState/restoreRootState helpers (REUSED)
internal/config/
  upgrade.go            # READ-ONLY (LANDED) — LoadUpgradeConfig (CONSUMED by runUpgrade)
  config.go             # READ-ONLY (LANDED) — UpgradeConfig struct + Defaults
  file.go               # READ-ONLY (LANDED) — GlobalConfigPath() (CONSUMED by the test)
internal/exitcode/
  exitcode.go           # READ-ONLY (LANDED) — UpdateAvailable=6/ErrUpdateAvailable/New/For (CONSUMED)
internal/upgrade/      # READ-ONLY — the dispatch consumers are P1.M4.T2/T3 (NOT this item)
docs/cli.md             # EDIT (Mode A) — + `### upgrade` subsection + `6` exit-code row
go.mod                  # READ-ONLY — UNCHANGED (cobra/pflag already present)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/cmd/upgrade.go       # NEW — upgradeCmd + 9 flag vars + init() + runUpgrade + validateUpgradeFlags
internal/cmd/upgrade_test.go  # NEW — TestUpgradeCommand_{Registered,Flags,NoBootstrapOutsideRepo,HelpDisambiguates,FlagValidation}
docs/cli.md                   # EDIT — + `### upgrade` Subcommands subsection + `6` Exit codes row
# NOTHING ELSE. No edit to root.go, internal/upgrade/*, config.Load, exitcode.go, the commit path, go.mod,
# or any PRD/task file.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the no-op PersistentPreRunE is MANDATORY — it is the FR-U12/FR-B3 wall): without it, cobra
// runs root's PersistentPreRunE → config.Load → (a) bootstraps a config file on a missing global (FR-B3;
// upgrade must NOT write) + (b) reads the per-repo .stagecoach.toml (FR-U12; upgrade is repo-independent).
// Cobra runs ONLY the nearest ancestor's PersistentPreRunE, so defining it on upgradeCmd (return nil)
// OVERRIDES root's. Mirror lock.go EXACTLY. Do NOT rely on root.go's shouldSkipConfigLoad (its "upgrade"
// case is `config upgrade`, a different command).

// CRITICAL (flags are LOCAL — Flags(), NOT PersistentFlags): registering upgrade's 9 flags on
// rootCmd.PersistentFlags() would pollute the commit-path flag set (`stagecoach --help` would list
// --check/--rollback/etc.). Register on upgradeCmd.Flags() in init() (the models.go/config-init precedent).
// They are still bound to PACKAGE VARS (the root.go flag-binding convention) — but the FlagSet is local.

// CRITICAL (--version is NOT a collision with cobra's auto-version): cobra adds the auto --version flag
// ONLY to the command whose `Version` field is set (rootCmd, via rootCmd.Version = Version). It is a LOCAL
// flag on rootCmd, NOT inherited by subcommands. upgradeCmd has NO Version field → no auto-version flag →
// upgradeCmd.Flags().String("version", ...) is clean. (Two different commands, two different --version flags.)

// CRITICAL (runUpgrade is a PLACEHOLDER — P1.M4.T2.S1 owns the dispatch): runUpgrade does validation +
// resolution (THIS item) + a clearly-marked TODO + return nil (P1.M4.T2.S1 replaces with the real
// --check/normal/--rollback dispatch to internal/upgrade). The validation + resolution STAY (P1.M4.T2
// builds on them). The placeholder MUST use effChannel/effSourceRepo (e.g. in a stderr notice) so they
// are not "declared and not used" (Go compile error for locals / lint for package vars).

// GOTCHA (bool mutex rules need fs.Changed, not the package var): --check/--prerelease/--rollback/--force/
// --yes are bools with default false. flagCheck==false is ambiguous (not-set vs --check=false). For the
// mutex rules (--version+--prerelease, --rollback+--check/--version) use fs.Changed("<name>"), which is
// true ONLY if the user passed it. validateUpgradeFlags takes the *pflag.FlagSet. (The --channel enum check
// CAN use the package var: default "" ⇒ not-set, and valid values are stable/prerelease ⇒ a non-empty
// non-enum value is a real error.)

// GOTCHA (NO env for channel/source-repo — FR-U10): the resolution is flag > LoadUpgradeConfig > Defaults.
// There is NO STAGECOACH_CHANNEL / STAGECOACH_SOURCE_REPO (FR-U10 lists only [upgrade] config + flags).
// (--install-method DOES read STAGECOACH_INSTALL_METHOD — but its resolution is consumed by P1.M4.T2's
// dispatch; THIS item reads the flag + may read the env into a local, but the detection cascade is P1.M4.T2.
// Keep this item's resolution to channel/source-repo to stay in scope.)

// GOTCHA (runUpgrade NEVER os.Exit — returns exitcode.New): mirror every other RunE. main maps via
// exitcode.For. The placeholder returns nil (exit 0) — acceptable for the transient pre-P1.M4.T2 state.

// GOTCHA (cobra short-circuits --help BEFORE PersistentPreRunE): so `stagecoach upgrade --help` never
// runs the no-op PreRunE (and never bootstraps). The no-bootstrap TEST must run `upgrade` WITHOUT --help
// (which hits runUpgrade) to actually exercise the no-op PreRunE path.

// GOTCHA (Long MUST disambiguate from `config upgrade`): `stagecoach upgrade` = binary self-update (FR-U1);
// `stagecoach config upgrade` = config-schema migration (FR-B5). The v3 scope boundary explicitly requires
// the help text to distinguish them. State it in upgradeCmd.Long (and the docs/cli.md `### upgrade` entry).
```

## Implementation Blueprint

### Data models and structure

None beyond the 9 flag package vars (8 scalars) + the `upgradeCmd` `*cobra.Command`. No new types/structs.
`runUpgrade`/`validateUpgradeFlags` are plain functions returning `error`. No new config fields, no new
exit codes (exitcode.UpdateAvailable=6 is LANDED), no new deps (cobra/pflag already imported by the package).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/cmd/upgrade.go — upgradeCmd + 9 flags + init() + runUpgrade + validateUpgradeFlags
  - PACKAGE DOC (Mode A): explain this is the v3.0 binary self-update command (FR-U1–U12), DISAMBIGUATE
    from `config upgrade` (config-schema migration — a different command), state the FR-U12 wall (no lock,
    no repo, no provider, no config.Load bootstrap; the no-op PersistentPreRunE mirrors lock.go), and that
    it is the one named §19 network exception (fetches ONLY this project's GitHub release artifacts +
    checksums). Note the dispatch is P1.M4.T2 (this file owns the command shell + validation + resolution).
  - IMPORTS: fmt, os (for STAGECOACH_INSTALL_METHOD env read if you resolve it — optional), github.com/spf13/cobra,
    github.com/spf13/pflag, github.com/dabstractor/stagecoach/internal/config, github.com/dabstractor/stagecoach/internal/exitcode.
    (NO internal/upgrade import — the dispatch is P1.M4.T2; this item is walled off from it at the CLI layer.)
  - FLAG PACKAGE VARS (zero defaults so presence is detectable; bound in init()):
      var (
          flagCheck          bool
          flagTargetVersion  string
          flagPrerelease     bool
          flagForce          bool
          flagRollback       bool
          flagInstallMethod  string
          flagYes            bool
          flagChannel        string
          flagSourceRepo     string
      )
  - upgradeCmd (mirror lock.go's structure; LEAF so RunE+Args directly on it):
      var upgradeCmd = &cobra.Command{
          Use:   "upgrade",
          Short: "Update the stagecoach binary to the latest release",
          Long: `<DISAMBIGUATING Long: stagecoach upgrade = BINARY self-update (FR-U1): detects the install
method and delegates to that channel's updater (Homebrew/Scoop/winget/npm/mise/asdf/Nix/AUR/go-install),
self-swapping only for the direct-binary channel. This is NOT config upgrade (a config-schema migration
run as stagecoach config upgrade) — two different commands. Makes network calls to GitHub Releases only
(this project's own release artifacts + checksums); it is the one named exception to the no-network-calls
commit path (FR-U12). Acquires no run lock, reads no repo, invokes no provider. Runs outside a git repo.`,
          SilenceErrors:     true,
          SilenceUsage:      true,
          PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, // OVERRIDES root's config.Load (FR-B3/FR-U12); mirror lock.go
          Args:              cobra.NoArgs,
          RunE:              runUpgrade,
      }
  - init(): register all 9 flags on upgradeCmd.Flags() (LOCAL), then rootCmd.AddCommand:
      func init() {
          fs := upgradeCmd.Flags()
          fs.BoolVarP(&flagCheck, "check", "c", false, "Check for an update without applying it (exit 6 if behind, 0 if up to date) (FR-U6)")
          fs.StringVar(&flagTargetVersion, "version", "", "Pin a target version to install (default: latest in the channel) (FR-U5)")
          fs.BoolVar(&flagPrerelease, "prerelease", false, "Admit pre-release tags (shorthand for --channel prerelease) (FR-U10)")
          fs.BoolVar(&flagForce, "force", false, "Override a detected package-manager install and self-swap (FR-U1)")
          fs.BoolVar(&flagRollback, "rollback", false, "Restore the most recent backup (one-step undo) (FR-U8)")
          fs.StringVar(&flagInstallMethod, "install-method", "", "Override install-method detection (env STAGECOACH_INSTALL_METHOD) (FR-U2)")
          fs.BoolVarP(&flagYes, "yes", "y", false, "Skip the confirmation prompt (for scripting) (FR-U9)")
          fs.StringVar(&flagChannel, "channel", "", "Release channel: stable (default) | prerelease (FR-U10)")
          fs.StringVar(&flagSourceRepo, "source-repo", "", "owner/repo to fetch releases from (default dabstractor/stagecoach; for forks) (FR-U10)")
          rootCmd.AddCommand(upgradeCmd) // ZERO edits to root.go (providers/hook/integrate/lock pattern)
      }
  - validateUpgradeFlags(fs *pflag.FlagSet) error — the 3 contract rules:
      func validateUpgradeFlags(fs *pflag.FlagSet) error {
          // (1) --version + --prerelease mutually exclusive.
          if fs.Changed("version") && fs.Changed("prerelease") {
              return fmt.Errorf("--version and --prerelease are mutually exclusive")
          }
          // (2) --rollback exclusive with --check and --version.
          if fs.Changed("rollback") && (fs.Changed("check") || fs.Changed("version")) {
              return fmt.Errorf("--rollback cannot be combined with --check or --version")
          }
          // (3) --channel rejects unknown values ("" ⇒ not-set ⇒ OK).
          if flagChannel != "" && flagChannel != "stable" && flagChannel != "prerelease" {
              return fmt.Errorf("--channel %q: must be stable or prerelease", flagChannel)
          }
          return nil
      }
  - runUpgrade(cmd, args) — validation + resolution + PLACEHOLDER dispatch:
      func runUpgrade(cmd *cobra.Command, _ []string) error {
          if err := validateUpgradeFlags(cmd.Flags()); err != nil {
              return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))
          }
          // Resolve effective channel/source-repo (FR-U10): flag > [upgrade] config (global only) > Defaults.
          // NO env for channel/source-repo (FR-U10 lists only config + flags). LoadUpgradeConfig reads the
          // global file ONLY (never bootstraps — the no-op PersistentPreRunE skipped config.Load).
          uc, err := config.LoadUpgradeConfig()
          if err != nil {
              return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))
          }
          effChannel := uc.Channel
          if flagChannel != "" {
              effChannel = flagChannel
          } else if flagPrerelease {
              effChannel = "prerelease" // --prerelease = --channel prerelease (FR-U10)
          }
          effSourceRepo := uc.SourceRepo
          if flagSourceRepo != "" {
              effSourceRepo = flagSourceRepo
          }
          // TODO(P1.M4.T2.S1): dispatch --check / normal (detect→delegate|swap) / --rollback to internal/upgrade.
          // The dispatch consumes effChannel/effSourceRepo (+ flagInstallMethod/STAGECOACH_INSTALL_METHOD +
          // flagTargetVersion/flagForce/flagYes). Until then this is a no-op placeholder.
          fmt.Fprintf(cmd.ErrOrStderr(), "stagecoach upgrade: not yet implemented (channel=%s source=%s)\n", effChannel, effSourceRepo)
          return nil
      }
  - NAMING: upgradeCmd, runUpgrade, validateUpgradeFlags, flag{Check,TargetVersion,Prerelease,Force,Rollback,InstallMethod,Yes,Channel,SourceRepo}.
  - GOTCHA: the placeholder stderr notice USES effChannel/effSourceRepo (avoids "declared and not used").
    P1.M4.T2.S1 removes the notice + inserts the dispatch. Do NOT import internal/upgrade (the dispatch is
    P1.M4.T2; this item is the shell). Do NOT os.Exit.

Task 2: CREATE internal/cmd/upgrade_test.go — registration/flags/no-bootstrap/disambiguation/validation
  - PACKAGE: `package cmd` (white-box — reuses saveRootState/restoreRootState + rootCmd + upgradeCmd).
    IMPORTS: bytes, context, os, path/filepath, strings, testing (+ cobra/pflag as needed).
  - TestUpgradeCommand_Registered:
      c, err := rootCmd.Find([]string{"upgrade"})
      if err != nil { t.Fatalf(...) }
      if c == nil || c.Name() != "upgrade" { t.Errorf("upgrade not registered") }
  - TestUpgradeCommand_Flags (all 9 + the 2 shorthands):
      fs := upgradeCmd.Flags()
      for _, name := range []string{"check","version","prerelease","force","rollback","install-method","yes","channel","source-repo"} {
          if fs.Lookup(name) == nil { t.Errorf("flag --%s not registered on upgrade", name) }
      }
      if fs.ShorthandLookup("c") == nil { t.Errorf("-c shorthand missing") }
      if fs.ShorthandLookup("y") == nil { t.Errorf("-y shorthand missing") }
      // NEGATIVE: the flags are LOCAL (NOT on root's persistent set) — the commit path must not see them.
      if rootCmd.PersistentFlags().Lookup("check") != nil { t.Errorf("--check leaked onto root persistent flags") }
  - TestUpgradeCommand_NoBootstrapOutsideRepo (the FR-B3/FR-U12 proof):
      // Isolate HOME + XDG so GlobalConfigPath() lands in a temp dir; assert NO bootstrap write.
      tmpHome := t.TempDir()
      t.Setenv("HOME", tmpHome)
      t.Setenv("XDG_CONFIG_HOME", "") // force the ~/.config fallback inside tmpHome
      cfgPath := config.GlobalConfigPath()
      if _, err := os.Stat(cfgPath); err == nil { t.Fatalf("precondition: global config already exists at %s", cfgPath) }
      origArgs, origOut, origErr, origRunE := saveRootState(t)
      defer restoreRootState(t, origArgs, origOut, origErr, origRunE)
      var outBuf, errBuf bytes.Buffer
      rootCmd.SetOut(&outBuf); rootCmd.SetErr(&errBuf)
      rootCmd.SetArgs([]string{"upgrade"}) // runs runUpgrade (placeholder) → exercises the no-op PersistentPreRunE
      _ = Execute(context.Background())    // the placeholder returns nil; tolerate any non-config err
      // THE INVARIANT: config.Load never ran ⇒ no bootstrap file was written.
      if _, err := os.Stat(cfgPath); err == nil {
          t.Errorf("upgrade bootstrapped a config at %s — the no-op PersistentPreRunE must prevent config.Load (FR-B3/FR-U12)", cfgPath)
      }
      // And the stderr must NOT contain a config-load/bootstrap error.
      if strings.Contains(errBuf.String(), "config:") || strings.Contains(outBuf.String(), "wrote bootstrap config") {
          t.Errorf("upgrade triggered config.Load (stderr=%q stdout=%q) — the no-op PersistentPreRunE must skip it", errBuf.String(), outBuf.String())
      }
  - TestUpgradeCommand_HelpDisambiguates (the naming-collision guard):
      // upgradeCmd.Long MUST distinguish binary self-update from `config upgrade` (config-schema migration).
      long := upgradeCmd.Long
      if !strings.Contains(long, "upgrade") { t.Errorf("Long missing 'upgrade'") } // sanity
      // It must reference the OTHER command by name so a user reading --help can tell them apart:
      if !strings.Contains(long, "config upgrade") && !strings.Contains(long, "config-upgrade") {
          t.Errorf("Long must disambiguate from 'config upgrade' (the config-schema migration); got:\n%s", long)
      }
      // And it must NOT claim to be a config-schema migration:
      if strings.Contains(long, "schema") && strings.Contains(long, "migration") {
          t.Errorf("Long must not describe itself as a schema migration (that's config upgrade)")
      }
      // --help short-circuits before PersistentPreRunE (cobra) — verify it does NOT bootstrap either:
      tmpHome := t.TempDir(); t.Setenv("HOME", tmpHome); t.Setenv("XDG_CONFIG_HOME", "")
      cfgPath := config.GlobalConfigPath()
      origArgs, origOut, origErr, origRunE := saveRootState(t); defer restoreRootState(t, origArgs, origOut, origErr, origRunE)
      var b bytes.Buffer; rootCmd.SetOut(&b); rootCmd.SetErr(&b); rootCmd.SetArgs([]string{"upgrade", "--help"})
      _ = Execute(context.Background())
      if _, err := os.Stat(cfgPath); err == nil { t.Errorf("upgrade --help bootstrapped a config") }
      if !strings.Contains(b.String(), "check") { t.Errorf("upgrade --help must list --check") }
  - TestUpgradeCommand_FlagValidation (the 3 contract rules):
      // Helper: build a fresh FlagSet mirroring upgradeCmd's flags, set the named flags Changed, call validateUpgradeFlags.
      newFS := func() *pflag.FlagSet {
          fs := pflag.NewFlagSet("upgrade", pflag.ContinueOnError)
          fs.Bool("check", false, ""); fs.String("version", "", ""); fs.Bool("prerelease", false, "")
          fs.Bool("rollback", false, ""); fs.String("channel", "", "")
          return fs
      }
      // (1) --version + --prerelease mutex.
      fs := newFS(); fs.Set("version", "1.2.3"); fs.Set("prerelease", "true")
      if err := validateUpgradeFlags(fs); err == nil { t.Errorf("--version+--prerelease must error") }
      // (2a) --rollback + --check mutex.
      fs = newFS(); fs.Set("rollback", "true"); fs.Set("check", "true")
      if err := validateUpgradeFlags(fs); err == nil { t.Errorf("--rollback+--check must error") }
      // (2b) --rollback + --version mutex.
      fs = newFS(); fs.Set("rollback", "true"); fs.Set("version", "1.2.3")
      if err := validateUpgradeFlags(fs); err == nil { t.Errorf("--rollback+--version must error") }
      // (3) --channel unknown value.
      // NOTE: rule (3) reads the flagChannel PACKAGE VAR (not fs). Set it directly + restore.
      orig := flagChannel; defer func() { flagChannel = orig }(); flagChannel = "bogus"
      if err := validateUpgradeFlags(newFS()); err == nil { t.Errorf("--channel bogus must error") }
      // (negative) valid combos pass:
      flagChannel = ""; fs = newFS(); fs.Set("check", "true")
      if err := validateUpgradeFlags(fs); err != nil { t.Errorf("--check alone must pass; got %v", err) }
      flagChannel = "prerelease"
      if err := validateUpgradeFlags(newFS()); err != nil { t.Errorf("--channel prerelease must pass; got %v", err) }
  - FOLLOW pattern: root_test.go's saveRootState/restoreRootState + hook_test.go/lock_test.go's
    SetOut/SetErr/SetArgs/Execute idiom. The validateUpgradeFlags unit test uses a fresh pflag.FlagSet
    (so it doesn't depend on the package singleton).
  - GOTCHA: the --channel rule reads the package var flagChannel (validateUpgradeFlags uses it); the unit
    test sets flagChannel directly + restores via defer. The bool-mutex rules use fs.Changed (the test
    fs.Set marks them Changed). Keep the two mechanisms straight.

Task 3: EDIT docs/cli.md (Mode A) — add `### upgrade` + the `6` exit-code row
  - ADD a `### upgrade` subsection under "## Subcommands", immediately AFTER `### lock status` (~line 397,
    before "## Exit codes"). Content: one-line summary (binary self-update, FR-U1), the DISAMBIGUATION note
    ("Distinct from `config upgrade` (a config-schema migration) — two different commands"), the 9 flags
    (one line each, mirroring the flag help text), the FR-U12 wall note ("acquires no run lock, reads no
    repo, invokes no provider; runs outside a git repo"), the §19 network note ("the one named exception
    to the no-network-calls commit path — fetches ONLY this project's GitHub release artifacts + checksums"),
    and the exit codes (0 up-to-date/upgraded/printed; 1 failure; 6 update-available via --check). Add a
    short example block (`stagecoach upgrade --check` → exit 6 if behind).
  - EDIT the "## Exit codes" table (~line 400): add a row — `| \`6\` | Update available (\`stagecoach upgrade
    --check\` found a newer release); upgrade-path only — never returned by the commit path. |` — placed
    after the `5` (Busy) row and before `124` (Timeout), matching the exitcode.go const order. Add a one-
    sentence note under the table that 6 is upgrade-path-only (FR-U12), distinct from the commit-path codes.
  - GOTCHA: do NOT touch the existing `### config upgrade` entry (line 203) — it stays as the config-schema
    migration. The new `### upgrade` entry cross-REFERENCES it for disambiguation. Do NOT renumber existing
    sections. Preserve the markdown table pipe structure.

Task 4: VERIFY — build, vet, format, tests, lint, grep guards
  - go build ./... ; go vet ./internal/cmd/... ; gofmt -l internal/cmd/upgrade.go internal/cmd/upgrade_test.go  # empty
  - go test ./internal/cmd/ -run 'TestUpgradeCommand' -v   # the 5 new tests
  - go test ./internal/cmd/ -race                           # full cmd regression (no sibling test broke)
  - make test && make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the command shell — clone of lock.go's structure for a LEAF):
var upgradeCmd = &cobra.Command{
	Use:               "upgrade",
	Short:             "Update the stagecoach binary to the latest release",
	Long:              `…disambiguating from config upgrade…`,
	SilenceErrors:     true,
	SilenceUsage:      true,
	PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, // OVERRIDES root's config.Load (FR-B3/FR-U12)
	Args:              cobra.NoArgs,
	RunE:              runUpgrade,
}

// PATTERN (LOCAL flags on the command's own FlagSet — NOT persistent; bound to package vars):
func init() {
	fs := upgradeCmd.Flags()
	fs.BoolVarP(&flagCheck, "check", "c", false, "…(FR-U6)")
	// …8 more…
	rootCmd.AddCommand(upgradeCmd) // ZERO edits to root.go
}

// PATTERN (runUpgrade = validation + resolution + PLACEHOLDER; never os.Exit):
func runUpgrade(cmd *cobra.Command, _ []string) error {
	if err := validateUpgradeFlags(cmd.Flags()); err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))
	}
	uc, err := config.LoadUpgradeConfig() // global-only; never bootstraps (FR-B3 boundary)
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))
	}
	effChannel := uc.Channel
	if flagChannel != "" {
		effChannel = flagChannel
	} else if flagPrerelease {
		effChannel = "prerelease"
	}
	effSourceRepo := uc.SourceRepo
	if flagSourceRepo != "" {
		effSourceRepo = flagSourceRepo
	}
	// TODO(P1.M4.T2.S1): dispatch --check / normal / --rollback to internal/upgrade.
	fmt.Fprintf(cmd.ErrOrStderr(), "stagecoach upgrade: not yet implemented (channel=%s source=%s)\n", effChannel, effSourceRepo)
	return nil
}

// PATTERN (bool mutex uses fs.Changed; enum uses the package var):
func validateUpgradeFlags(fs *pflag.FlagSet) error {
	if fs.Changed("version") && fs.Changed("prerelease") {
		return fmt.Errorf("--version and --prerelease are mutually exclusive")
	}
	if fs.Changed("rollback") && (fs.Changed("check") || fs.Changed("version")) {
		return fmt.Errorf("--rollback cannot be combined with --check or --version")
	}
	if flagChannel != "" && flagChannel != "stable" && flagChannel != "prerelease" {
		return fmt.Errorf("--channel %q: must be stable or prerelease", flagChannel)
	}
	return nil
}
```

### Integration Points

```yaml
CLI SURFACE (the new command):
  - `stagecoach upgrade [--check|-c] [--version <v>] [--prerelease] [--force] [--rollback]
    [--install-method <m>] [--yes|-y] [--channel <stable|prerelease>] [--source-repo <owner/repo>]`.
  - Registered via init() → rootCmd.AddCommand(upgradeCmd). ZERO edits to root.go.
  - Flags are LOCAL (upgradeCmd.Flags()) — NOT on root's persistent set (do NOT pollute the commit path).

COMMAND BEHAVIOR:
  - No-op PersistentPreRunE (mirror lock.go) → runs outside a git repo, never bootstraps a config (FR-B3/FR-U12).
  - runUpgrade: validateUpgradeFlags → resolve (flag > LoadUpgradeConfig > Defaults) → PLACEHOLDER dispatch
    (P1.M4.T2.S1 replaces with the real --check/normal/--rollback). Never os.Exit; returns exitcode.New/nil.

CONSUMES (LANDED — read-only):
  - exitcode.New / exitcode.For / exitcode.Error / exitcode.UpdateAvailable (P1.M1.T1.S1).
  - config.LoadUpgradeConfig / config.UpgradeConfig (P1.M1.T2.S1) — the global-only [upgrade] reader.
  - config.GlobalConfigPath (file.go) — the test's no-bootstrap assertion target.

DOCS (Mode A):
  - docs/cli.md: + `### upgrade` Subcommands subsection + `6` Exit codes table row.

NO database / migration / routes / config-struct change / exitcode-const change (6 is LANDED) / go.mod change.
SCOPE FENCES:
  - Touches ONLY: internal/cmd/upgrade.go (NEW), internal/cmd/upgrade_test.go (NEW), docs/cli.md (EDIT).
  - Does NOT edit: root.go, internal/upgrade/* (the dispatch is P1.M4.T2/T3), config.Load, exitcode.go,
    internal/cmd/{lock,models,config,hook,integrate,providers}.go, the commit path (FR-U12), go.mod, any PRD/task file.
  - runUpgrade's dispatch is a PLACEHOLDER; P1.M4.T2.S1 owns the real body. The validation + resolution STAY.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (the new command links into rootCmd via init(); cobra/pflag already imported).
go build ./...
# Expected: clean. A failure on "flagTargetVersion declared and not used" ⇒ the placeholder doesn't use
#           effChannel/effSourceRepo (fix: the stderr notice must reference them).

# Vet the changed package.
go vet ./internal/cmd/...
# Expected: clean.

# Format the 2 new files.
gofmt -l internal/cmd/upgrade.go internal/cmd/upgrade_test.go
# Expected: empty. If listed: gofmt -w internal/cmd/upgrade.go internal/cmd/upgrade_test.go

# Lint.
make lint   # errcheck/gosimple/govet/ineffassign/staticcheck/unused
# Expected: zero errors. (All 9 flag vars are used by runUpgrade/validateUpgradeFlags or are the bound
#           targets of init(); effChannel/effSourceRepo used by the placeholder notice. The unused linter
#           won't fire on flagForce/flagYes/flagInstallMethod/flagTargetVersion/flagCheck/flagRollback —
#           they ARE read: flagPrerelease/flagChannel/flagSourceRepo in runUpgrade; the bools flagCheck/
#           flagRollback/flagForce/flagYes/flagTargetVersion/flagInstallMethod are CONSUMED by P1.M4.T2's
#           dispatch — until then they may be flagged "used only via fs.Changed/address-of". To satisfy the
#           unused linter NOW, the placeholder notice or a `_ =` line can reference them; OR register them
#           so the &flagX address-of IS the use (the root.go convention: "&flagX is their use"). Confirm.)

# Scope guard: ONLY the 3 files.
git status --porcelain
# Expected: internal/cmd/upgrade.go, internal/cmd/upgrade_test.go, docs/cli.md. ZERO changes to root.go,
#           internal/upgrade/*, config.Load, exitcode.go, the commit path, go.mod.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The 5 new registration/validation tests.
go test ./internal/cmd/ -run 'TestUpgradeCommand' -v
# Expected: ALL PASS —
#   TestUpgradeCommand_Registered: rootCmd.Find("upgrade") succeeds, Name()=="upgrade".
#   TestUpgradeCommand_Flags: all 9 flags + -c/-y shorthands on upgradeCmd.Flags(); NONE on root PersistentFlags.
#   TestUpgradeCommand_NoBootstrapOutsideRepo: config.GlobalConfigPath() absent before AND after Execute(["upgrade"]).
#   TestUpgradeCommand_HelpDisambiguates: Long mentions "config upgrade"; --help lists --check; --help doesn't bootstrap.
#   TestUpgradeCommand_FlagValidation: the 3 mutex/enum rules + valid-combo negatives.

# Full cmd-package regression (the new command + flags don't disturb root's flag set / sibling commands).
go test ./internal/cmd/ -race
# Expected: green.

# Full race suite + lint + build.
make test && make lint && make build
# Expected: all green.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary (proves upgrade.go links into the real CLI).
make build

# Manual: the command is registered + disambiguates + doesn't bootstrap.
cd "$(mktemp -d)"  # NOT a git repo, NO global config
HOME="$(pwd)" XDG_CONFIG_HOME="" ../../bin/stagecoach upgrade --help | head
# Expected: the upgrade help, listing all 9 flags, disambiguating from `config upgrade`. NO config file written.

../../bin/stagecoach upgrade 2>&1 | head
# Expected: "stagecoach upgrade: not yet implemented (channel=stable source=dabstractor/stagecoach)" (the placeholder).
#   And `ls ~/.config/stagecoach/` shows NO config.toml (the no-op PersistentPreRunE prevented the bootstrap).

# Manual: --check is wired (the flag parses; the dispatch is the placeholder).
../../bin/stagecoach upgrade --check 2>&1 | head
# Expected: the placeholder notice (P1.M4.T2.S1 will make --check exit 6 when behind).

# Manual: flag validation.
../../bin/stagecoach upgrade --version 1.0 --prerelease; echo "exit=$?"
# Expected: a "mutually exclusive" error, exit 1.
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: the command is registered via init() (ZERO edits to root.go).
grep -n 'rootCmd.AddCommand(upgradeCmd)' internal/cmd/upgrade.go   # 1 hit
grep -c 'upgradeCmd' internal/cmd/root.go   # 0 (root.go untouched)

# Guard 2: the no-op PersistentPreRunE is present (the FR-U12/FR-B3 wall).
grep -n 'PersistentPreRunE: func(\*cobra.Command, \[\]string) error { return nil }' internal/cmd/upgrade.go  # 1 hit

# Guard 3: flags are LOCAL (upgradeCmd.Flags()), NOT persistent.
grep -n 'upgradeCmd.Flags()' internal/cmd/upgrade.go   # ≥1 hit (the init)
grep -nE 'PersistentFlags\(\).*(check|rollback|channel|source-repo|install-method)' internal/cmd/upgrade.go internal/cmd/root.go | grep upgrade   # 0 hits

# Guard 4: all 9 flags registered with the contract names + the 2 shorthands.
for f in check version prerelease force rollback install-method yes channel source-repo; do
  grep -q -- "\"$f\"" internal/cmd/upgrade.go || echo "MISSING flag --$f"
done
grep -q '"c"' internal/cmd/upgrade.go && grep -q '"y"' internal/cmd/upgrade.go   # shorthands

# Guard 5: runUpgrade NEVER os.Exit (returns exitcode.New/nil).
grep -n 'os.Exit' internal/cmd/upgrade.go   # 0 hits

# Guard 6: runUpgrade calls LoadUpgradeConfig (global-only; never config.Load).
grep -n 'config.LoadUpgradeConfig()' internal/cmd/upgrade.go   # 1 hit
grep -n 'config.Load(' internal/cmd/upgrade.go   # 0 hits (config.Load would bootstrap — forbidden)

# Guard 7: the Long/help disambiguates from config upgrade.
grep -n 'config upgrade' internal/cmd/upgrade.go   # ≥1 hit (in the Long string)

# Guard 8: the dispatch is a PLACEHOLDER (P1.M4.T2.S1 owns the real body).
grep -n 'TODO(P1.M4.T2.S1)' internal/cmd/upgrade.go   # 1 hit

# Guard 9: docs/cli.md gained the `### upgrade` entry + the `6` exit-code row.
grep -n '^### `upgrade`' docs/cli.md   # 1 hit
grep -n '^| `6` |' docs/cli.md          # 1 hit (Update Available row)

# Guard 10: scope — only the 3 files.
git status --porcelain
# Expected: internal/cmd/upgrade.go + internal/cmd/upgrade_test.go + docs/cli.md ONLY.
git diff --name-only | grep -vE '^(internal/cmd/upgrade\.go|internal/cmd/upgrade_test\.go|docs/cli\.md)$' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./internal/cmd/...` clean; `gofmt -l` empty on the 2 new files
- [ ] `make lint` zero errors (all 9 flag vars used; effChannel/effSourceRepo used by the placeholder)
- [ ] `go test ./internal/cmd/ -run TestUpgradeCommand -v` green (5 tests)
- [ ] `go test ./internal/cmd/ -race` green (no sibling test broke; root's flag set unchanged)
- [ ] `make test` + `make build` clean

### Feature Validation
- [ ] `stagecoach upgrade` registered (rootCmd.Find) (grep guard 1)
- [ ] No-op PersistentPreRunE present (the FR-B3/FR-U12 wall) (grep guard 2)
- [ ] 9 flags LOCAL on upgradeCmd.Flags() (NOT persistent — commit path unaffected) (grep guards 3,4)
- [ ] runUpgrade never os.Exit; calls LoadUpgradeConfig (not config.Load) (grep guards 5,6)
- [ ] Long disambiguates from `config upgrade` (grep guard 7)
- [ ] Dispatch is a PLACEHOLDER for P1.M4.T2.S1 (grep guard 8)
- [ ] No-bootstrap-outside-repo test passes (config.GlobalConfigPath absent after Execute(["upgrade"]))
- [ ] Flag-validation test passes (3 mutex/enum rules + valid combos)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY internal/cmd/upgrade.go + internal/cmd/upgrade_test.go + docs/cli.md (guard 10)
- [ ] NO edit to root.go (registration via init()) (guard 1), internal/upgrade/* (P1.M4.T2/T3), config.Load,
      exitcode.go, the commit path (FR-U12), go.mod, or any PRD/task file
- [ ] NO new exitcode const (6 is LANDED), NO new config field, NO new dep, NO internal/upgrade import
- [ ] NO edit to the existing `### config upgrade` docs entry (config-schema migration) — the new
      `### upgrade` entry disambiguates it

### Code Quality & Docs
- [ ] [Mode A] package doc on upgrade.go disambiguates + notes the FR-U12 wall + the §19 network exception
- [ ] [Mode A] docs/cli.md `### upgrade` subsection + `6` exit-code row
- [ ] validateUpgradeFlags is a pure helper (takes *pflag.FlagSet) — unit-testable in isolation
- [ ] runUpgrade's placeholder is clearly marked (TODO(P1.M4.T2.S1)) so the next subtask finds it

---

## Anti-Patterns to Avoid

- ❌ Don't add upgrade's flags to `rootCmd.PersistentFlags()`. They are LOCAL to `upgradeCmd.Flags()` — the
  contract is explicit ("Flags are LOCAL to upgrade (Flags(), NOT PersistentFlags) so they do NOT pollute
  the commit-path flag set"). Persistent registration would make `--check`/`--rollback` appear on
  `stagecoach --help` and on the commit path. Register in `init()` on `upgradeCmd.Flags()` (models.go precedent).
- ❌ Don't rely on `shouldSkipConfigLoad` for the no-bootstrap guarantee. Its `name=="upgrade"` case is
  `config upgrade` (the schema migration). The TOP-LEVEL `upgrade`'s guarantee is the no-op
  `PersistentPreRunE` (which overrides root's config.Load). Define it on upgradeCmd (mirror lock.go).
- ❌ Don't call `config.Load` from `runUpgrade`. It would bootstrap a config (FR-B3) and read the per-repo
  file (FR-U12). Call `config.LoadUpgradeConfig()` ONLY (the dedicated global-only reader that never writes).
- ❌ Don't implement the dispatch in this item. The `--check`/normal/`--rollback` dispatch (calling
  internal/upgrade's Check/Swap/Rollback/Delegate) is P1.M4.T2.S1. `runUpgrade` here is validation +
  resolution + a PLACEHOLDER. Implementing the dispatch would duplicate/conflict with P1.M4.T2.S1.
- ❌ Don't import `internal/upgrade` in upgrade.go. The dispatch is P1.M4.T2; this item is the CLI shell,
  walled off from the upgrade package at this layer (the placeholder doesn't call it). Importing it now
  would couple the shell to the dispatch prematurely.
- ❌ Don't `os.Exit` in `runUpgrade`. Return `exitcode.New(exitcode.Error, err)` / `nil`; main maps via
  `exitcode.For`. (Mirror every other RunE in internal/cmd.)
- ❌ Don't leave `effChannel`/`effSourceRepo` (or any flag var) "declared and not used." The Go compiler
  rejects unused locals; the linter rejects unused package vars. The placeholder stderr notice MUST
  reference effChannel/effSourceRepo. For flag vars consumed only by P1.M4.T2's dispatch (flagForce/flagYes/
  flagInstallMethod/flagTargetVersion/flagCheck/flagRollback), the `&flagX` address-of in `init()`'s
  flag-registration IS their use (the root.go convention) — confirm the linter is satisfied.
- ❌ Don't confuse `--version` (the upgrade target-pin flag) with cobra's auto-`--version`. cobra's auto-
  version is LOCAL to the command with `Version` set (rootCmd); upgradeCmd has no `Version` field ⇒ no
  collision. `upgradeCmd.Flags().String("version", ...)` is clean. (If you set `upgradeCmd.Version`, you'd
  get the collision — DON'T.)
- ❌ Don't touch the existing `### config upgrade` docs entry. It's the config-schema migration (FR-B5) — a
  DIFFERENT command. The new `### upgrade` entry cross-references it for disambiguation; do not merge/rewrite it.
- ❌ Don't edit root.go. Registration is via `init() { rootCmd.AddCommand(upgradeCmd) }` in upgrade.go (the
  providers/hook/integrate/lock pattern). root.go's flag registration + PersistentPreRunE are unchanged.
- ❌ Don't use the bool package vars for the mutex rules. `flagCheck==false` is ambiguous (not-set vs
  `--check=false`). Use `fs.Changed("<name>")` (true ONLY if the user passed it) for the mutex rules; use
  the `flagChannel` package var for the enum check (default "" ⇒ not-set ⇒ the enum check is safe on it).
- ❌ Don't forget the [Mode A] docs/cli.md addition. The contract DOCS clause requires it: the `### upgrade`
  subsection (flags + disambiguation + §19 network note + exit 0/1/6) AND the `6` row in the Exit codes table.