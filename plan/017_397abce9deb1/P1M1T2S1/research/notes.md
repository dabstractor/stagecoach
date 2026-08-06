# Research notes — P1.M1.T2.S1 (UpgradeConfig + global-only LoadUpgradeConfig)

Source: direct reads of `internal/config/{config,file,load,inert}.go`, their `*_test.go`,
`docs/configuration.md`, `PRD.md` (§9.29 FR-U10, §16.1, §16.2, FR-B4), and
`plan/017_397abce9deb1/architecture/{system_context,v3_scope_boundary}.md`.

## 1. Defaults() lives in config.go (NOT defaults.go)
The work-item INPUT names `defaults.go`, but it does NOT exist. `func Defaults() Config` is in
`internal/config/config.go` (line ~210). `CurrentConfigVersion = 3` is a const at the top of
config.go. PRD §16.1 prose says "Built-in defaults (`internal/config/defaults.go`)" — that prose is
aspirational; the code reality is config.go. **Seed the upgrade defaults inside config.go's Defaults().**

## 2. The two-struct split (file-decode vs resolved) is the established pattern
- Resolved value type in config.go (e.g. `RoleConfig`, the `Config` scalars).
- File-decode twin in file.go (e.g. `fileRoleConfig`, `fileDefaults`, `fileGeneration`) decoded via
  `fileConfig` (the §16.2 decode target). `materialize(fc, ...)` copies non-zero file fields into a
  fresh `*Config`; `overlay(dst, src)` merges across layers.
- `fileConfig` is unexported; only `loadTOML` (file.go) and tests decode into it.
- **go-toml/v2 SILENTLY DROPS unknown keys** (proven by the `remapAgentTerminology` comment: "otherwise
  go-toml silently drops [agent.*] — fileConfig has no Agent field"). So [upgrade] is harmless to old
  code; adding `fileConfig.Upgrade` makes it DECODABLE for LoadUpgradeConfig.

## 3. Global path resolution (the seam LoadUpgradeConfig must reuse)
`internal/config/file.go`:
- `globalConfigPath()` (UNEXPORTED): `$XDG_CONFIG_HOME/stagecoach/config.toml` when XDG set+absolute,
  else `~/.config/stagecoach/config.toml` via `os.UserHomeDir()`, else "config.toml" (last-resort CWD).
- `GlobalConfigPath()` is just the exported wrapper → `globalConfigPath()`.
- `ResolveConfigPath(flag)` = --config > STAGECOACH_CONFIG env > GlobalConfigPath(). **LoadUpgradeConfig
  must NOT use ResolveConfigPath** (it excludes env/flags by contract) → use `globalConfigPath()` directly.

## 4. Load() is the WRONG reader for the upgrade command (FR-B3 bootstrap hazard)
`internal/config/load.go` `Load()`:
- Layer 2: when the global file is MISSING and the path is NOT explicit AND `!opts.DisableBootstrap`,
  it calls `bootstrapWriteConfig(globalPath)` and writes a populated config (FR-B3 first-run). The
  upgrade command runs OUTSIDE a git repo with a NO-OP PersistentPreRunE (v3_scope_boundary.md) and
  MUST NOT trigger that write. → `[upgrade]` needs a DEDICATED reader (LoadUpgradeConfig) that reads
  the global file only and, on missing file, returns defaults with NO write.
- Load() also overlays repo `.stagecoach.toml` (layer 3) — which FR-U10 says must be IGNORED for
  [upgrade]. → another reason LoadUpgradeConfig is global-file-only.

## 5. Advisory / notice machinery (must stay silent for [upgrade])
- `noticeOut io.Writer` (file.go, default os.Stderr); tests swap via `noticeOut = &strings.Builder{}`
  or `SetNoticeOut(buf)`.
- `configVersionNotice(fileLoaded, version)` + `migrationNotice(orig)` (load.go/migrate.go) drive the
  §9.17 advisory. Fires only in `Load()`, gated on `fileLoaded && cfg.ConfigVersion < CurrentConfigVersion`.
- **LoadUpgradeConfig touches NONE of this** → it emits NO advisory by construction. Tests assert
  noticeOut stays empty to lock it.
- FR-B4: additive (new optional [section]/keys) must NOT bump CurrentConfigVersion and must NOT emit an
  advisory. Adding [upgrade] is purely additive → CurrentConfigVersion stays 3.

## 6. Test patterns to mirror (all `package config` = WHITE-BOX → unexported helpers callable)
- `loadEnvSetup(t)` (load_test.go:84): sets HOME+XDG_CONFIG_HOME to a temp `home`, returns
  `(home, repo, globalDir)` where `globalDir = $home/stagecoach`. Write the global file as
  `writeConfigFile(t, globalDir, "config.toml", body)`.
- `writeConfigFile(t, dir, relPath, body)` (load_test.go:18): MkdirAll + WriteFile.
- `chdir(t, dir)` (load_test.go): chdir + restore.
- Notice capture: `orig := noticeOut; noticeOut = &strings.Builder{}; defer func(){noticeOut=orig}()`.
- `TestGlobalConfigPath` (file_test.go:148) shows XDG set/unset/relative discipline.
- Direct loadTOML/materialize calls are used freely (white-box).

## 7. PRD §16.2 exact [upgrade] table shape (mirror verbatim)
```toml
[upgrade]                         # self-update (§9.29); global config only — no per-repo meaning
channel     = "stable"            # stable | prerelease (admits -rc/-beta tags; = --prerelease)
source_repo = "dabstractor/stagecoach"   # release source; override for a fork. Compile-time default.
```
Keys: `channel`, `source_repo` (snake_case). Defaults per FR-U10 + §16.1: "stable", "dabstractor/stagecoach".

## 8. DESIGN CALL — why Config.Upgrade is toml:"-" AND materialize() does NOT copy fc.Upgrade
- Add `UpgradeConfig` to config.go. Add `Upgrade UpgradeConfig` to `Config` tagged `toml:"-"`, seeded in
  Defaults() (satisfies the literal "Seed Defaults()" instruction; mirrors Roles/Providers being toml:"-").
- Add `fileUpgrade` + `Upgrade fileUpgrade` (`toml:"upgrade"`) to file.go's `fileConfig` (decode target).
- `materialize()` does NOT copy `fc.Upgrade` → `Config.Upgrade`. Consequence: the `Load()` resolver NEVER
  propagates [upgrade] from global OR repo files → `cfg.Upgrade` is always the built-in defaults after
  Load(). This is the cleanest guarantee of FR-U10 (per-repo [upgrade] ignored) and the global-only
  contract: the resolver simply does not expose [upgrade]; the SOLE reader is LoadUpgradeConfig()
  (global file only). Config.Upgrade documents the type/defaults and is the value LoadUpgradeConfig
  seeds from (`Defaults().Upgrade`). REJECTED ALTERNATIVE: copy fc.Upgrade in materialize — would let the
  repo file's [upgrade] leak into the resolved Config (FR-U10 violation) since materialize can't tell
  global vs repo apart. (The --verbose "per-repo [upgrade] ignored" NOTE is the upgrade command's job,
  P1.M4.T1.S1 — NOT this task; LoadUpgradeConfig just doesn't read the repo file at all.)

## 9. Scope boundary (do NOT do, per v3_scope_boundary.md)
- Do NOT add the `stagecoach upgrade` cobra command (P1.M4.T1.S1).
- Do NOT add --channel/--source-repo flags (P1.M4.T1.S1 local flags).
- Do NOT bump CurrentConfigVersion; do NOT add advisory logic for [upgrade].
- Do NOT read repo file / git-config / env / flags in LoadUpgradeConfig.
- Do NOT write/bootstrap anything; do NOT touch noticeOut.
- Do NOT add net/http or any new module dep (none needed — pure file read + go-toml already a dep).

## 10. Validation
- `go test ./internal/config/` (white-box; reuse loadEnvSetup/writeConfigFile helpers).
- `go test ./...` + `gofmt -l` + `go vet ./...` (repo has no ruff/mypy; it's Go).
- Coverage gate in CI is ≥85% on config/ — keep new file well-covered.