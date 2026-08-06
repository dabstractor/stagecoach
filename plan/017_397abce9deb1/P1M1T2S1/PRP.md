# PRP for P1.M1.T2.S1

## Objective

Add a global-only `[upgrade]` config surface to `internal/config`: an `UpgradeConfig` struct (`Channel`, `SourceRepo`), the defaults seeded into `config.Defaults()`, a `[upgrade]` decode target on `fileConfig`, and a NEW exported seam `config.LoadUpgradeConfig() (UpgradeConfig, error)` that reads **only** the global config file (XDG path) with defaults applied — never the repo file / git-config / env / flags, never a bootstrap write, never an advisory. This is the seam the `stagecoach upgrade` command (P1.M4.T1.S1) calls. Additive per FR-B4: `CurrentConfigVersion` stays `3`, no load-time advisory. Update `docs/configuration.md` (Mode A). `go test ./internal/config/` must pass.

---

## Goal

**Feature Goal**: Give the self-update feature (PRD §9.29, FR-U10) its dedicated, **global-only** configuration reader that is walled off from the 7-layer `config.Load` resolver. Specifically: (1) a typed `UpgradeConfig{Channel, SourceRepo}` with built-in defaults (`channel="stable"`, `source_repo="dabstractor/stagecoach"`); (2) `[upgrade]` decodable from the global config file; (3) `config.LoadUpgradeConfig()` returning the merged global-only result. The upgrade command runs **outside a git repo** with a **no-op `PersistentPreRunE`** (it must NOT trigger `config.Load`'s first-run bootstrap write — FR-B3, v3_scope_boundary.md), so it needs this dedicated reader rather than the full resolver.

**Deliverable**: Modified `internal/config/config.go` (new `UpgradeConfig` struct + `Upgrade` field on `Config` + seeded `Defaults()`), modified `internal/config/file.go` (new `fileUpgrade` struct + `Upgrade` field on `fileConfig`), a NEW file `internal/config/upgrade.go` exposing `LoadUpgradeConfig() (UpgradeConfig, error)`, a NEW test file `internal/config/upgrade_test.go` (white-box, table-driven, mirroring `load_test.go`'s `loadEnvSetup`/`writeConfigFile` helpers), and Mode-A doc rows in `docs/configuration.md`.

**Success Definition**: `LoadUpgradeConfig()` returns the seeded defaults when the global file is missing OR lacks `[upgrade]`; returns the file's merged values when `[upgrade]` is present (non-empty wins); reads an older-version (e.g. v2) global file with `[upgrade]` without error and **without any advisory**; writes **nothing** on a missing global file (no bootstrap — the FR-B3 hazard); `config.Load()` on a global file with `[upgrade]` and a repo file with `[upgrade]` leaves `cfg.Upgrade` at the built-in defaults (resolver never propagates `[upgrade]` — global-only is structural). `CurrentConfigVersion == 3` unchanged; `go test ./internal/config/` green; `go build ./...` succeeds; gofmt-clean; no new module dependency.

---

## Why

- FR-U10 (PRD §9.29): `[upgrade]` is **global-only** ("an upgrade is global, not per-repo"); a per-repo `.stagecoach.toml` `[upgrade]` is "ignored with a `--verbose` note." The existing `config.Load` resolver **cannot** express "read this table from the global file but ignore it from the repo file" — `loadTOML` + `materialize` + `overlay` apply uniformly to every file layer. A dedicated, global-file-only reader is the only clean way to honor FR-U10 without special-casing the resolver.
- FR-B3 (bootstrap hazard, v3_scope_boundary.md): `config.Load` **writes** a populated bootstrap config when the global file is missing and the path is not explicit (`load.go` Layer 2, `bootstrapWriteConfig`). The upgrade command is repo-independent and runs with a no-op `PersistentPreRunE` (mirroring `internal/cmd/lock.go`), so it must never reach `Load`. `LoadUpgradeConfig` is the seam that reads the global file with **no write** on miss.
- FR-B4 (additive discipline): adding an optional `[upgrade]` table is forward-compatible — it must **not** bump `CurrentConfigVersion` (stays 3) and must **not** emit an upgrade advisory. This task encodes that discipline: `[upgrade]` is decoded into `fileConfig` but is **not** propagated by `materialize`/`overlay`, so it can never trigger the resolver's version logic, and `LoadUpgradeConfig` touches no advisory machinery.
- This is the **third** primitive of Milestone P1.M1 (Foundation), after exit-code 6 (P1.M1.T1.S1, done) and comparable semver (P1.M1.T1.S2, done). The GitHub Releases client (P1.M1.T3) and the upgrade command (P1.M4.T1) both consume `LoadUpgradeConfig()` to resolve `source_repo`/`channel` defaults before applying the `--source-repo`/`--channel`/`--prerelease` local flags.

---

## What

User-visible behavior: none yet (no caller consumes `LoadUpgradeConfig` until P1.M4.T1.S1). This task ships the typed config surface + its dedicated global-only reader + tests + Mode-A docs. It adds **no** CLI flag, **no** env var, **no** git-config key, **no** network, **no** filesystem write, and touches **no** commit-path code.

### Public API (package `config`)

```go
// UpgradeConfig is the global-only self-update configuration (PRD §9.29 FR-U10).
type UpgradeConfig struct {
    Channel    string `toml:"channel"`     // "stable" (default) | "prerelease" (= --prerelease)
    SourceRepo string `toml:"source_repo"` // "owner/repo"; default "dabstractor/stagecoach"
}

// LoadUpgradeConfig reads ONLY the global config file (XDG path) and returns the [upgrade]
// table with Defaults applied. It does NOT read the repo file / git-config / env / flags,
// does NOT bootstrap/write, and emits NO advisory. Missing file ⇒ defaults (no error).
func LoadUpgradeConfig() (UpgradeConfig, error)
```

### Success Criteria

- [ ] `internal/config/config.go`: `UpgradeConfig` struct exists with `Channel`/`SourceRepo` TOML tags `channel`/`source_repo`; `Config` gains an `Upgrade UpgradeConfig` field tagged `toml:"-"`; `Defaults()` seeds it `Channel:"stable", SourceRepo:"dabstractor/stagecoach"`. Inline field comments cite §9.29 FR-U10 and the global-only contract.
- [ ] `internal/config/file.go`: `fileUpgrade` struct (`Channel`/`SourceRepo`, same tags) + `Upgrade fileUpgrade` field (`toml:"upgrade"`) on `fileConfig`. `materialize()` does **NOT** copy `fc.Upgrade` (resolver never propagates `[upgrade]`).
- [ ] `internal/config/upgrade.go`: exported `LoadUpgradeConfig() (UpgradeConfig, error)` reading `globalConfigPath()` only; missing file ⇒ `Defaults().Upgrade` with `nil` error (NO write); present file decoded into `fileConfig`; non-empty `fc.Upgrade.Channel`/`SourceRepo` override the defaults; parse error wrapped with the path. Touches `noticeOut` **never**.
- [ ] `CurrentConfigVersion` is still `3`; `Load()` emits no new advisory for a file with/without `[upgrade]`.
- [ ] `internal/config/upgrade_test.go`: covers missing-file→defaults, no-`[upgrade]`-table→defaults, with-`[upgrade]`→merged, partial-`[upgrade]` (one field)→other field default, older-version (v2) file with `[upgrade]`→works + no advisory, malformed TOML→error, **missing-file writes nothing** (stat asserts not-exist), and a `Load()` regression guard (global+repo `[upgrade]` ⇒ `cfg.Upgrade` stays defaults; v3 file with `[upgrade]` ⇒ no advisory).
- [ ] `docs/configuration.md` (Mode A): `[upgrade]` rows in the "Built-in defaults" table + a commented `[upgrade]` block in the "File format" example + a global-only callout (per-repo ignored).
- [ ] `go test ./internal/config/` passes; `go build ./...` succeeds; `gofmt -l internal/config/` clean; no new `go.mod` require line.

---

## All Needed Context

### Context Completeness Check

"If someone knew nothing about this codebase, would they have everything needed to implement this successfully?" — **YES.** This PRP includes: the exact current `Defaults()` body and the exact insertion point; the exact `fileConfig`/`materialize`/`overlay` shapes and the explicit "do not copy `fc.Upgrade`" rule; the exact `globalConfigPath()` helper to reuse and the reason `ResolveConfigPath` must NOT be used; the exact test helpers to mirror (`loadEnvSetup`, `writeConfigFile`, notice-capture idiom) with file:line; the exact PRD §16.2 `[upgrade]` table to mirror verbatim; and the exact docs sections to edit. The implementing agent needs to read only the files named in References.

### Documentation & References

```yaml
# MUST READ — the files you are creating/editing
- file: internal/config/config.go
  why: Add UpgradeConfig struct + Config.Upgrade field (toml:"-") + seed Defaults(). Holds
        CurrentConfigVersion (STAYS 3 — do not touch the const) and Defaults() (the seed site).
  pattern: Defaults() returns Config BY VALUE with every field listed (see the Push/NoVerify/
           NoParentWatchdog/HookTimeout block for the exact append style). Add the Upgrade seed
           alongside the others (it is a plain struct value, not a pointer).
  gotcha: "defaults.go does NOT exist — Defaults() lives HERE in config.go. The work-item INPUT
          and PRD §16.1 prose say 'defaults.go'; that is aspirational. Seed in config.go."
- file: internal/config/file.go
  why: Add fileUpgrade struct + fileConfig.Upgrade field (toml:"upgrade"). Reuse globalConfigPath()
        from here in upgrade.go (same package). DO NOT copy fc.Upgrade in materialize().
  pattern: fileRoleConfig (line ~36) is the EXACT template for fileUpgrade (struct + toml tags),
           and fileConfig's `Role map[string]fileRoleConfig` shows how a new subtable field is
           declared. fileConfig is unexported; only loadTOML (here) and tests decode into it.
  gotcha: "go-toml/v2 SILENTLY DROPS unknown keys (see remapAgentTerminology comment line ~146:
          'otherwise go-toml silently drops [agent.*]'). So an old file with [upgrade] is harmless;
          adding fileConfig.Upgrade makes it DECODABLE for LoadUpgradeConfig. materialize() must
          NOT reference fc.Upgrade — see DESIGN CALL in this PRP (global-only contract)."
- file: internal/config/load.go
  why: The resolver you are deliberately NOT using. Read it to understand WHY LoadUpgradeConfig
        is separate: Layer 2 bootstrapWriteConfig (FR-B3 write hazard) + Layer 3 repo overlay
        (FR-U10 per-repo-ignore). DO NOT modify load.go.
  pattern: loadEnv/loadFlags set bools DIRECTLY (not via overlay) — relevant context only; you add
           NO env/flag wiring here (--channel/--source-repo are P1.M4.T1.S1 LOCAL flags).
  gotcha: "ResolveConfigPath(flag) honors --config AND STAGECOACH_CONFIG env. LoadUpgradeConfig
           must NOT use it (contract excludes env/flags) — use globalConfigPath() directly."

# CREATE — the new seam + its tests
- file: internal/config/upgrade.go        # NEW — LoadUpgradeConfig (package config, Mode A doc comment)
- file: internal/config/upgrade_test.go   # NEW — white-box (package config), table-driven

# TEST-STYLE TEMPLATES (read-only references — mirror, do not edit)
- file: internal/config/load_test.go
  why: The helpers + assertion style to reuse. WHITE-BOX `package config` (line 1) ⇒ unexported
        helpers (globalConfigPath, fileConfig, noticeOut, Defaults) are directly callable.
  pattern: "loadEnvSetup(t) (line 84) → sets HOME+XDG_CONFIG_HOME to a temp dir, returns
           (home, repo, globalDir=$home/stagecoach). writeConfigFile(t, dir, relPath, body) (line 18)
           → MkdirAll+WriteFile. Notice capture idiom (line ~707): `orig:=noticeOut;
           noticeOut=&strings.Builder{}; defer func(){noticeOut=orig}()`."
  gotcha: "loadEnvSetup also initRepo's a git repo (layer 4) — fine to ignore for upgrade tests;
           you only need home+globalDir. chdir(t, repo) is NOT needed (LoadUpgradeConfig ignores CWD)."
- file: internal/config/file_test.go
  why: TestGlobalConfigPath (line 148) shows the XDG set/unset/relative discipline; the notice-
        capture via SetNoticeOut (line ~428) is the alternative idiom.

# DECISION PROVENANCES (read the cited sections; do not edit these files)
- docfile: plan/017_397abce9deb1/architecture/v3_scope_boundary.md
  why: "THE walled-off contract. 'config.Load's first-run bootstrap write (FR-B3) — upgrade is
        repo-independent and must run outside a git repo. Its command PersistentPreRunE is a no-op.
        The [upgrade] config table is read from the GLOBAL config file only; a per-repo [upgrade]
        is ignored with a --verbose note (FR-U10).' Also: 'Additive config (no migration): the
        [upgrade] table is OPTIONAL and ADDITIVE ... MUST NOT bump CurrentConfigVersion (stays 3)
        and MUST NOT emit a load-time upgrade advisory.'"
- docfile: plan/017_397abce9deb1/architecture/system_context.md
  why: "Codebase state + conventions. 'Config: add a field to Config + seed in Defaults() + decode
        in file.go's fileConfig + overlay in load.go. [upgrade] is a nested struct (Channel,
        SourceRepo) — global-only (FR-U10).' NOTE the overlay-in-load.go part is DELIBERATELY NOT
        done here (see DESIGN CALL) — overlay would propagate repo [upgrade], violating FR-U10.
        'No new dep needed' (go-toml already a require)."
- docfile: plan/017_397abce9deb1/P1M1T2S1/research/notes.md
  why: The research notes behind every decision here (Defaults() is in config.go, go-toml drops
        unknown keys, the bootstrap hazard, the test helpers, the design-call rationale).

# PRD — requirement source (canonical; do not edit PRD.md)
- prd: §9.29 FR-U10   # [upgrade] table, global-only, channel/source_repo defaults, per-repo ignored
- prd: §9.17 FR-B4    # additive config MUST NOT bump version / emit advisory
- prd: §16.1          # built-in defaults incl. 'upgrade channel "stable" + source_repo "dabstractor/stagecoach"'
- prd: §16.2          # the exact [upgrade] TOML table shape to mirror (channel, source_repo keys)

# LIBRARY — already a dependency (no new require)
- url: https://github.com/pelletier/go-toml/v2
  why: toml.Unmarshal into fileConfig decodes the [upgrade] table. go.mod already requires v2.4.2.
  critical: "Unmarshal SILENTLY DROPS unknown keys and never errors on them (verified in-repo).
            A missing [upgrade] ⇒ zero-value fileUpgrade ⇒ defaults win. A present-but-empty value
            (channel = \"\") ⇒ non-empty-wins guard keeps the default (mirrors materialize semantics)."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  config.go          # CurrentConfigVersion=3 (DO NOT TOUCH); Config struct; Defaults() <- EDIT (+struct,+field,+seed)
  file.go            # fileConfig + fileDefaults/fileGeneration/fileRoleConfig; globalConfigPath();  <- EDIT (+fileUpgrade,+field)
                     #   loadTOML/materialize/overlay; noticeOut/SetNoticeOut                         (materialize must NOT copy fc.Upgrade)
  load.go            # Load() 7-layer resolver (bootstrapWriteConfig FR-B3 hazard) <- DO NOT EDIT
  inert.go           # IsInert (advisory suppression helper) — context only
  migrate.go         # migrationNotice — context only
  upgrade.go         # DOES NOT EXIST YET — this task creates it (LoadUpgradeConfig)
  upgrade_test.go    # DOES NOT EXIST YET — this task creates it
  load_test.go       # loadEnvSetup/writeConfigFile/chdir helpers + notice-capture idiom — MIRROR
  file_test.go       # TestGlobalConfigPath / TestSetNoticeOut — MIRROR
docs/
  configuration.md   # Mode A doc target <- EDIT (defaults table + File format [upgrade] block + global-only callout)
go.mod               # module github.com/dabstractor/stagecoach; go 1.22; go-toml/v2 v2.4.2 (NO new require)
```

### Desired Codebase tree (files added/changed)

```bash
internal/config/
  config.go          # +type UpgradeConfig; +Config.Upgrade (toml:"-"); +Defaults() seed
  file.go            # +type fileUpgrade; +fileConfig.Upgrade (toml:"upgrade")
  upgrade.go         # NEW — package doc (Mode A) + LoadUpgradeConfig()
  upgrade_test.go    # NEW — TestLoadUpgradeConfig_* + TestLoad_UpgradeTableIgnoredByResolver + const guard
docs/
  configuration.md   # +[upgrade] rows in "Built-in defaults"; +commented [upgrade] in "File format"; +global-only callout
go.mod               # UNCHANGED (go-toml already required; no new dep)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: Defaults() is in config.go, NOT defaults.go. The work-item INPUT and PRD §16.1
// prose both say "defaults.go" — that file does not exist. Seed the upgrade defaults in
// config.go's Defaults(). (Confirmed: `ls internal/config/defaults.go` → no such file.)

// CRITICAL (FR-B3): config.Load() WRITES a bootstrap config when the global file is missing
// and the path is not explicit (load.go Layer 2, bootstrapWriteConfig). LoadUpgradeConfig
// must NEVER reach Load(); on a missing global file it returns Defaults().Upgrade with nil
// error and writes NOTHING. Test this with an explicit "stat the path → os.IsNotExist".

// CRITICAL (FR-U10 global-only): materialize() must NOT copy fc.Upgrade → Config.Upgrade.
// materialize cannot tell the global file from the repo file apart (loadTOML calls it for
// both), so copying would let a repo [upgrade] leak into the resolved Config. By NOT copying,
// the resolver structurally ignores [upgrade] from EVERY layer; LoadUpgradeConfig is the SOLE
// reader (global file only). Config.Upgrade therefore always equals Defaults() after Load() —
// that is intentional and documented (it is the value LoadUpgradeConfig seeds from).

// LIBRARY QUIRK: go-toml/v2 toml.Unmarshal SILENTLY DROPS unknown keys (never errors). So an
// older binary reading a file WITH [upgrade], and a newer binary reading a file WITHOUT [upgrade],
// are both fine. fileConfig.Upgrade exists only so LoadUpgradeConfig can DECODE it.

// NON-EMPTY WINS: mirror materialize()'s non-zero overlay semantics. A file value of "" (e.g.
// `channel = ""`) must NOT clobber the default "stable" — guard with `if fc.Upgrade.Channel != ""`.

// PATH: use globalConfigPath() (the XDG discovery helper), NOT ResolveConfigPath(). The contract
// excludes env (--config / STAGECOACH_CONFIG) and flags; globalConfigPath is the single source of
// truth the global file loader falls back to.
```

---

## Implementation Blueprint

### Data models and structure

```go
// internal/config/config.go — ADD (near RoleConfig, before Config):

// UpgradeConfig is the global-only self-update configuration (PRD §9.29 FR-U10): which release
// channel to track and which owner/repo to fetch releases from. It is decoded from the [upgrade]
// TOML table by fileConfig.Upgrade (file.go) and surfaced to the upgrade command via the DEDICATED
// global-only reader LoadUpgradeConfig (upgrade.go) — NOT via the 7-layer config.Load resolver
// (which would (a) trigger the FR-B3 first-run bootstrap write and (b) read the per-repo
// .stagecoach.toml, both of which FR-U10/v3_scope_boundary forbid for [upgrade]). Config.Upgrade
// (below) carries the built-in DEFAULTS only; it is toml:"-" and is NEVER populated by the file
// loaders, so the resolver cannot leak a repo [upgrade] into the resolved config (global-only is
// structural). Defaults: Channel="stable", SourceRepo="dabstractor/stagecoach" (PRD §16.1).
type UpgradeConfig struct {
	Channel    string `toml:"channel"`     // "stable" (default) | "prerelease" (= --prerelease; admits -rc/-beta tags)
	SourceRepo string `toml:"source_repo"` // "owner/repo"; default "dabstractor/stagecoach" (set for a fork/self-host)
}

// ... inside `type Config struct { ... }`, near the other resolved fields (e.g. after Roles):
	// Upgrade holds the global-only self-update config DEFAULTS (PRD §9.29 FR-U10). toml:"-": Config is
	// never decoded from a §16.2 file (fileConfig is), and the [upgrade] table is intentionally NOT
	// propagated by materialize()/overlay() — the resolver must not read [upgrade] (global-only, and
	// reading would risk the FR-B3 bootstrap write + a per-repo leak). The upgrade command reads
	// [upgrade] via LoadUpgradeConfig() (global file only). This field always holds Defaults() after a
	// Load(); it exists as the typed home for the defaults and the seed LoadUpgradeConfig starts from.
	Upgrade UpgradeConfig `toml:"-"`

// ... inside Defaults(), add to the returned Config literal:
		Upgrade: UpgradeConfig{
			Channel:    "stable",                 // §9.29 FR-U10 default; "prerelease" admits -rc/-beta (= --prerelease)
			SourceRepo: "dabstractor/stagecoach", // §9.29 FR-U10 / §16.1 compile-time default (override for a fork)
		},
```

```go
// internal/config/file.go — ADD (near fileRoleConfig, and as a field on fileConfig):

// fileUpgrade is the FILE decode twin of config.UpgradeConfig (§9.29 FR-U10). A global [upgrade]
// table decodes into fc.Upgrade. materialize() does NOT copy it into Config (the resolver must not
// surface [upgrade] — global-only via LoadUpgradeConfig); it exists as the decode target for the
// dedicated reader. Channel/SourceRepo are plain strings (no duration parsing needed).
type fileUpgrade struct {
	Channel    string `toml:"channel"`
	SourceRepo string `toml:"source_repo"`
}

// ... inside `type fileConfig struct { ... }`, add (e.g. after the Role/Provider maps):
	Upgrade fileUpgrade `toml:"upgrade"` // §9.29 FR-U10 — [upgrade] table (global-only; NOT propagated by materialize)
```

```go
// internal/config/upgrade.go — NEW FILE (package config):

// Package config's upgrade reader is documented here. (The package doc lives in config.go; this is
// a Mode-A file comment explaining the global-only seam.)

// LoadUpgradeConfig reads ONLY the global config file (the XDG discovery path — the same path
// globalConfigPath() / GlobalConfigPath() resolve) and returns the [upgrade] table with the built-in
// Defaults applied (PRD §9.29 FR-U10). It is the DEDICATED reader the `stagecoach upgrade` command
// (P1.M4.T1) calls, deliberately walled off from config.Load because:
//   - config.Load's Layer 2 bootstraps (writes) a config on a missing global file (FR-B3); upgrade
//     runs outside a git repo with a no-op PersistentPreRunE and must NOT write.
//   - config.Load's Layer 3 reads the per-repo .stagecoach.toml; FR-U10 says a per-repo [upgrade] is
//     IGNORED. (The --verbose "ignored" note is the upgrade command's job, P1.M4.T1.S1.)
//
// Semantics: a MISSING global file ⇒ Defaults().Upgrade with a nil error (NO write, NO error). A
// present file is decoded into fileConfig; non-empty [upgrade] fields override the defaults
// (mirrors materialize()'s non-zero overlay). It never reads repo/git-config/env/flags, never writes,
// and emits NO advisory (it never touches noticeOut). A parse error is wrapped with the path.
func LoadUpgradeConfig() (UpgradeConfig, error) {
	uc := Defaults().Upgrade // seed: Channel="stable", SourceRepo="dabstractor/stagecoach"
	path := globalConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return uc, nil // missing global file ⇒ defaults; NO bootstrap write, NO error (FR-B3 boundary)
		}
		return uc, fmt.Errorf("read upgrade config %s: %w", path, err)
	}

	var fc fileConfig
	if err := toml.Unmarshal(data, &fc); err != nil {
		return uc, fmt.Errorf("parse upgrade config %s: %w", path, err)
	}

	// Non-empty wins (mirrors materialize's non-zero overlay; a "" value must not clobber the default).
	if fc.Upgrade.Channel != "" {
		uc.Channel = fc.Upgrade.Channel
	}
	if fc.Upgrade.SourceRepo != "" {
		uc.SourceRepo = fc.Upgrade.SourceRepo
	}
	return uc, nil
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/config/config.go — add the UpgradeConfig type + Config.Upgrade + Defaults() seed
  - ADD: `type UpgradeConfig struct { Channel string \`toml:"channel"\`; SourceRepo string \`toml:"source_repo"\` }`
          with a doc comment citing §9.29 FR-U10 and the global-only/resolver-never-propagates contract.
  - ADD: `Upgrade UpgradeConfig \`toml:"-"\`` field on `type Config struct`, with an inline comment
          explaining it holds DEFAULTS only (resolver does not populate it; LoadUpgradeConfig is the reader).
  - EDIT: `Defaults()` returned literal to seed `Upgrade: UpgradeConfig{Channel:"stable", SourceRepo:"dabstractor/stagecoach"}`.
  - FOLLOW pattern: the Push/NoVerify/NoParentWatchdog/HookTimeout block in Defaults() (append style;
          plain struct value, NOT a pointer).
  - NAMING: UpgradeConfig (CamelCase type); Channel/SourceRepo fields; toml tags channel/source_repo.
  - DO NOT TOUCH: the `CurrentConfigVersion = 3` const (FR-B4 — additive, no version bump).
  - GOTCHA: "defaults.go does NOT exist — Defaults() lives HERE. The work-item/PRD prose is aspirational."

Task 2: EDIT internal/config/file.go — add fileUpgrade + fileConfig.Upgrade (decode target)
  - ADD: `type fileUpgrade struct { Channel string \`toml:"channel"\`; SourceRepo string \`toml:"source_repo"\` }`
          (mirror fileRoleConfig's shape + doc comment).
  - ADD: `Upgrade fileUpgrade \`toml:"upgrade"\`` field on `type fileConfig struct`.
  - DO NOT EDIT: materialize() — it must NOT reference fc.Upgrade (the resolver never propagates [upgrade];
          this is the structural guarantee of FR-U10 global-only). overlay() is likewise untouched.
  - GOTCHA: "go-toml/v2 silently drops unknown keys, so this field is what makes [upgrade] DECODABLE;
            without it, LoadUpgradeConfig would always see zero-values. loadTOML already decodes fileConfig
            for both global+repo files — harmless (materialize ignores fc.Upgrade for both)."

Task 3: CREATE internal/config/upgrade.go — LoadUpgradeConfig (the seam)
  - IMPLEMENT: `func LoadUpgradeConfig() (UpgradeConfig, error)` per the blueprint above.
  - USE: `Defaults().Upgrade` for the seed; `globalConfigPath()` for the path (NOT ResolveConfigPath —
          contract excludes env/flags); `os.ReadFile` + `os.IsNotExist`; `toml.Unmarshal` into `fileConfig`;
          non-empty-wins overlay onto the seed.
  - DEPENDENCIES: reuses Defaults() (Task 1), globalConfigPath() + fileConfig (Task 2), go-toml/v2.
  - CONTRACT: missing file ⇒ (defaults, nil) with NO write; parse error ⇒ wrapped with path; NEVER writes;
          NEVER reads repo/git/env/flags; NEVER touches noticeOut (no advisory).
  - NAMING/PLACEMENT: package config; new file internal/config/upgrade.go (parallel to load.go/file.go).
  - IMPORTS: "fmt", "os", "github.com/pelletier/go-toml/v2".

Task 4: CREATE internal/config/upgrade_test.go — white-box, table-driven
  - PACKAGE: `package config` (white-box) so globalConfigPath/fileConfig/noticeOut/Defaults are callable.
  - REUSE: loadEnvSetup(t) (sets HOME+XDG_CONFIG_HOME → globalDir=$home/stagecoach) + writeConfigFile(t,
          globalDir, "config.toml", body) from load_test.go. Capture any notice via the
          `orig:=noticeOut; noticeOut=&strings.Builder{}; defer restore` idiom (defensive — LoadUpgradeConfig
          must write nothing, but assert it).
  - CASES (see Validation Loop for exact assertions):
      * missing global file        ⇒ Defaults() (Channel="stable", SourceRepo="dabstractor/stagecoach"); nil err; NO file written.
      * global file, no [upgrade]  ⇒ Defaults() (e.g. file has only `[defaults]\nprovider="pi"\n`).
      * global file, full [upgrade]⇒ merged (channel="prerelease", source_repo="foo/bar").
      * global file, partial       ⇒ one field set, other stays default (channel only; then source_repo only).
      * older-version file w/[upgrade] (config_version=2 + [upgrade]) ⇒ works, no advisory (noticeOut empty).
      * malformed TOML             ⇒ non-nil err whose message contains the path.
  - ADD: TestLoad_UpgradeTableIgnoredByResolver — `loadEnvSetup`; write a GLOBAL `[upgrade]` (channel=
          "prerelease") AND a REPO `.stagecoach.toml` `[upgrade]` (source_repo="evil/repo"); `chdir(repo)`;
          `Load(ctx, LoadOpts{RepoDir: repo})`; assert `cfg.Upgrade == Defaults().Upgrade` (resolver ignores
          BOTH), and (separately) a v3 file WITH [upgrade] emits NO advisory (noticeOut empty). This is the
          FR-U10 + FR-B4 regression guard.
  - ADD: TestCurrentConfigVersion_Unchanged — `if CurrentConfigVersion != 3 { t.Fatalf(...) }` (cheap lock).
  - FOLLOW pattern: load_test.go table-driven style (`tests := []struct{...}{...}; for _, tc := range ...`).

Task 5: EDIT docs/configuration.md (Mode A)
  - "Built-in defaults" table: add two rows — `channel` | `"stable"` | `config.Defaults()` (§9.29 FR-U10 —
          `[upgrade]`, global-only; `"prerelease"` admits -rc/-beta) and `source_repo` |
          `"dabstractor/stagecoach"` | `config.Defaults()` (§9.29 FR-U10 — release source; override for a fork).
  - "File format" populated block: add a commented `[upgrade]` section mirroring PRD §16.2 exactly:
          ```
          # [upgrade]                       # self-update (§9.29); GLOBAL config only — no per-repo meaning
          # channel     = "stable"          # stable | prerelease (admits -rc/-beta tags; = --prerelease)
          # source_repo = "dabstractor/stagecoach"  # release source; override for a fork. Compile-time default.
          ```
  - Add a one-paragraph global-only callout near the [upgrade] rows: "`[upgrade]` is read from the GLOBAL
          config file only. A per-repo `.stagecoach.toml` `[upgrade]` is ignored (a `--verbose` note is
          printed by `stagecoach upgrade`). `CurrentConfigVersion` is unchanged (3): `[upgrade]` is additive
          (FR-B4) and never emits a config advisory."
```

### Implementation Patterns & Key Details

```go
// PATTERN — seed a plain-struct default in Defaults() (config.go). UpgradeConfig is a value, not a
// pointer, so it is seeded inline in the returned Config literal exactly like the existing scalars:
func Defaults() Config {
	return Config{
		// ... existing fields ...
		Upgrade: UpgradeConfig{
			Channel:    "stable",
			SourceRepo: "dabstractor/stagecoach",
		},
	}
}

// PATTERN — LoadUpgradeConfig non-empty-wins overlay (upgrade.go). Identical discipline to
// materialize()'s non-zero overlay: a file cannot override a field to its zero value ("").
if fc.Upgrade.Channel != "" {
	uc.Channel = fc.Upgrade.Channel
}

// PATTERN — global-file-only test setup (upgrade_test.go). loadEnvSetup points XDG at a temp dir;
// writeConfigFile drops the global file at $XDG/stagecoach/config.toml (the path globalConfigPath()
// resolves). No chdir needed — LoadUpgradeConfig ignores CWD by contract.
_, _, globalDir := loadEnvSetup(t)            // sets HOME + XDG_CONFIG_HOME
writeConfigFile(t, globalDir, "config.toml", "[upgrade]\nchannel = \"prerelease\"\n")

// CRITICAL — the "no bootstrap write" assertion (the FR-B3 boundary that motivates this whole task):
t.Run("missing_file_writes_nothing", func(t *testing.T) {
	_, _, globalDir := loadEnvSetup(t) // XDG set; NO file written
	uc, err := LoadUpgradeConfig()
	if err != nil { t.Fatalf("err=%v", err) }
	if uc.Channel != "stable" || uc.SourceRepo != "dabstractor/stagecoach" {
		t.Fatalf("missing file ⇒ defaults; got %+v", uc)
	}
	// THE load-bearing assertion: config.Load would have bootstrapped a file HERE. LoadUpgradeConfig must NOT.
	if _, statErr := os.Stat(filepath.Join(globalDir, "config.toml")); !os.IsNotExist(statErr) {
		t.Fatalf("LoadUpgradeConfig must NOT write a bootstrap file (FR-B3); file exists")
	}
})
```

### Integration Points

```yaml
CONFIG (config.go):
  - add field: "Config.Upgrade UpgradeConfig (toml:\"-\") — DEFAULTS only; resolver does not populate it"
  - seed in:   "Defaults() → Upgrade:{Channel:\"stable\", SourceRepo:\"dabstractor/stagecoach\"}"
  - DO NOT:    "bump CurrentConfigVersion (stays 3); add advisory logic; touch the const"

FILE DECODE (file.go):
  - add type:  "fileUpgrade { Channel, SourceRepo } (toml channel/source_repo)"
  - add field: "fileConfig.Upgrade fileUpgrade (toml:\"upgrade\")"
  - DO NOT:    "copy fc.Upgrade in materialize(); reference it in overlay()"

NEW SEAM (upgrade.go):
  - func:      "LoadUpgradeConfig() (UpgradeConfig, error) — globalConfigPath() only; no write/bootstrap;
                no repo/git/env/flag; no advisory"

ROUTES/REGISTRATION:
  - none: "No cobra command, no flag registration, no env var, no git-config key in THIS task. The
           upgrade command (P1.M4.T1.S1) wires LoadUpgradeConfig + the LOCAL --channel/--source-repo flags."

DOCS (docs/configuration.md):
  - edit: "[upgrade] rows in 'Built-in defaults' table; commented [upgrade] block in 'File format';
           global-only callout (per-repo ignored; CurrentConfigVersion unchanged)"
```

---

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After each file — Go has no ruff/mypy; use gofmt + go vet.
gofmt -w internal/config/config.go internal/config/file.go internal/config/upgrade.go internal/config/upgrade_test.go
go vet ./internal/config/

# Whole module build + format check.
go build ./...
gofmt -l internal/config/   # Expected: empty output (all formatted).

# Expected: zero errors. If gofmt -l lists a file, re-run `gofmt -w` on it. If vet errors, read + fix.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new seam + the regression guard, verbose.
go test ./internal/config/ -run 'TestLoadUpgradeConfig|TestLoad_UpgradeTableIgnoredByResolver|TestCurrentConfigVersion_Unchanged' -v

# The full config package (must stay green — you must not regress load/file/materialize/overlay).
go test ./internal/config/ -v

# Expected: all PASS. Required assertion matrix for upgrade_test.go:
#   - missing global file            ⇒ {Channel:"stable", SourceRepo:"dabstractor/stagecoach"}, err=nil,
#                                      AND os.Stat(globalPath) ⇒ os.IsNotExist (NO bootstrap write — FR-B3).
#   - global file, no [upgrade]      ⇒ defaults (file has [defaults] only).
#   - global file, full [upgrade]    ⇒ {Channel:"prerelease", SourceRepo:"foo/bar"} (merged).
#   - global file, channel only      ⇒ Channel set, SourceRepo == default.
#   - global file, source_repo only  ⇒ SourceRepo set, Channel == "stable".
#   - older-version (config_version=2) file WITH [upgrade] ⇒ merged; noticeOut EMPTY (no advisory).
#   - malformed TOML                 ⇒ err != nil, err string contains the global path.
#   - Load() regression: global+repo [upgrade] ⇒ cfg.Upgrade == Defaults().Upgrade (both ignored);
#                                      v3 file WITH [upgrade] ⇒ noticeOut EMPTY (no advisory).
#   - CurrentConfigVersion == 3      (const guard).
```

### Level 3: Integration Testing (System Validation)

```bash
# Whole-module test suite (config changes can ripple — verify nothing else broke).
go test ./...

# Race detector (config is read concurrently in places; cheap insurance on the new file I/O).
go test -race ./internal/config/

# Coverage (CI gates config/ at ≥85%; keep the new file well-covered).
go test -cover ./internal/config/

# Expected: all packages PASS; config coverage does not regress below the gate.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Smoke: prove LoadUpgradeConfig resolves end-to-end against a real temp global file (no test funcs —
# a one-off main in a scratch _test.go or `go run` is overkill; the Level-2 cases already exercise this
# via loadEnvSetup). Instead, assert the doc/contract invariants directly:

# 1) Additive discipline (FR-B4): grep that CurrentConfigVersion is untouched and no advisory mentions [upgrade].
grep -n "CurrentConfigVersion = " internal/config/config.go   # → "CurrentConfigVersion = 3"
! grep -rn "upgrade" internal/config/migrate.go internal/config/load.go || echo "OK: no advisory/migration logic touches [upgrade]"

# 2) Global-only seam: LoadUpgradeConfig must use globalConfigPath() and nothing else.
grep -n "globalConfigPath\|ResolveConfigPath\|repoLocalConfigPath\|loadGitConfig\|loadEnv\|loadFlags\|bootstrapWrite\|noticeOut" internal/config/upgrade.go
# Expected: ONLY `globalConfigPath()` appears (no ResolveConfigPath/repo/git/env/flag/bootstrap/notice refs).

# 3) materialize/overlay do NOT propagate [upgrade].
! grep -n "Upgrade" internal/config/file.go | grep -E "materialize|overlay" && echo "OK: materialize/overlay ignore fc.Upgrade"

# Expected: all three invariants hold (the command prints the OK lines / the const line).
```

---

## Final Validation Checklist

### Technical Validation

- [ ] All 4 validation levels completed successfully.
- [ ] `go test ./internal/config/ -v` passes (incl. the new `TestLoadUpgradeConfig_*` + regression guard).
- [ ] `go test ./...` passes (no ripple into other packages).
- [ ] `go build ./...` succeeds.
- [ ] `gofmt -l internal/config/` is empty; `go vet ./internal/config/` is clean.
- [ ] No new line added to `go.mod`/`go.sum` (go-toml/v2 already required).

### Feature Validation

- [ ] Missing global file ⇒ defaults, **nil error**, **no file written** (FR-B3 boundary — the core reason this seam exists).
- [ ] Global file without `[upgrade]` ⇒ defaults.
- [ ] Global file with `[upgrade]` ⇒ merged (non-empty wins; `""` does not clobber the default).
- [ ] Older-version (v2) global file with `[upgrade]` ⇒ works, **no advisory**.
- [ ] `config.Load()` ignores `[upgrade]` from BOTH global and repo files (`cfg.Upgrade == Defaults().Upgrade`); v3 file with `[upgrade]` emits **no advisory**.
- [ ] `CurrentConfigVersion` is still `3`; no advisory/migration logic references `[upgrade]`.
- [ ] Malformed TOML ⇒ wrapped error naming the path.

### Code Quality Validation

- [ ] Follows existing patterns: `UpgradeConfig`/`fileUpgrade` mirror `RoleConfig`/`fileRoleConfig`; `Defaults()` seed mirrors the existing append style; test helpers (`loadEnvSetup`, `writeConfigFile`, notice-capture) reused, not reinvented.
- [ ] `Config.Upgrade` is `toml:"-"` and `materialize()` does not copy `fc.Upgrade` (global-only is structural).
- [ ] `LoadUpgradeConfig` uses `globalConfigPath()`, not `ResolveConfigPath()` (env/flags excluded by contract).
- [ ] Inline comments cite §9.29 FR-U10, FR-B4, and the global-only/resolver-never-propagates contract.

### Documentation & Deployment

- [ ] `docs/configuration.md` "Built-in defaults" table has `channel` + `source_repo` rows with the global-only note.
- [ ] `docs/configuration.md` "File format" example has a commented `[upgrade]` block mirroring PRD §16.2.
- [ ] A global-only callout is present (per-repo `[upgrade]` ignored; `CurrentConfigVersion` unchanged at 3).
- [ ] No new env var / git-config key / CLI flag documented (those belong to P1.M4.T1.S1).

---

## Anti-Patterns to Avoid

- ❌ **Don't copy `fc.Upgrade` in `materialize()`/`overlay()`.** That would let a per-repo `[upgrade]` leak into the resolved `Config` (FR-U10 violation) — `materialize` cannot distinguish the global file from the repo file. The resolver must structurally ignore `[upgrade]`; `LoadUpgradeConfig` is the sole reader.
- ❌ **Don't use `ResolveConfigPath()` in `LoadUpgradeConfig`.** It honors `--config` and `STAGECOACH_CONFIG` (env); the contract excludes env/flags. Use `globalConfigPath()` (the XDG discovery helper the global file loader falls back to).
- ❌ **Don't call `config.Load()` (or anything that can) from `LoadUpgradeConfig`.** `Load` writes a bootstrap config on a missing global file (FR-B3) and reads the repo file — both forbidden here.
- ❌ **Don't bump `CurrentConfigVersion` or add advisory logic for `[upgrade]`.** FR-B4: `[upgrade]` is additive — stays at version 3, never emits an advisory.
- ❌ **Don't add the cobra `upgrade` command or the `--channel`/`--source-repo`/`--prerelease` flags here.** Those are P1.M4.T1.S1 (LOCAL flags on the upgrade command). This task ships ONLY the config struct + the global-only reader + tests + docs.
- ❌ **Don't let a file's empty value (`channel = ""`) clobber the default.** Guard with `!= ""` (non-empty wins), mirroring `materialize`'s non-zero overlay.
- ❌ **Don't invent a `defaults.go` file.** `Defaults()` lives in `config.go` (the work-item/PRD "defaults.go" prose is aspirational). Seed in `config.go`.

---

## Confidence Score

**9/10** — one-pass success likelihood. The codebase has a crisp, well-documented config layer; the seam (`globalConfigPath` + `fileConfig` + `Defaults`) is fully specified with verbatim snippets; the test helpers (`loadEnvSetup`, `writeConfigFile`, notice-capture) are reused directly; and the one subtle correctness point (materialize must NOT propagate `[upgrade]`) is called out as a hard rule with a regression test. The −1 is for the inherent ambiguity in "Seed Defaults()" (resolved here as: add `Config.Upgrade` + seed in `config.go`'s `Defaults()`, with the toml:"-"/no-materialize-copy guard) — an implementer following this PRP verbatim lands the intended design.