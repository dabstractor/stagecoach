# Research notes — P1.M1.T2.S1: UpgradeConfig + global-only LoadUpgradeConfig

## Task (verbatim contract)
Add the optional, additive `[upgrade]` config table (channel, source_repo) read by the
upgrade command from the GLOBAL config file ONLY — without invoking full `config.Load`
(which needs a repo dir and triggers the FR-B3 bootstrap write). Per FR-B4 this additive
table does NOT bump `CurrentConfigVersion` (stays 3) and does NOT emit an upgrade advisory.

## Key codebase facts (verified)

- `internal/config/config.go`:
  - `const CurrentConfigVersion = 3` (line 20). MUST stay 3 (FR-B4 additive).
  - `func Defaults() Config` returns the Layer-1 `Config` (flat, resolved). There is NO
    `defaults.go` file — `Defaults()` lives in config.go. (The task text mentions
    `defaults.go`, but it does not exist; `Defaults()` is the single defaults source.)
  - `Config` is FLAT/RESOLVED and is NOT decoded directly from the §16.2 file; `fileConfig`
    (in file.go) is the decode intermediate, and `materialize` copies non-zero fields into `Config`.
- `internal/config/file.go`:
  - `fileConfig` struct is the §16.2 decode target: `ConfigVersion`, `Defaults`, `Generation`,
    `Role map[string]fileRoleConfig`, `Provider map[string]map[string]any`. Decode via
    `toml.Unmarshal(data, &fc)` (go-toml/v2). No `DisallowUnknownFields` anywhere.
  - `loadTOML(path) (*Config, error)` — read+decode+materialize; returns `(nil,nil)` for a
    MISSING file (layer-absent sentinel); wraps read/parse errors with the path.
  - `globalConfigPath()` / `GlobalConfigPath()` — XDG path: `$XDG_CONFIG_HOME/stagecoach/config.toml`
    when XDG set AND absolute; else `~/.config/stagecoach/config.toml` (home via os.UserHomeDir);
    last-resort `config.toml`. NOTE: `GlobalConfigPath()` does NOT honor `--config`/`STAGECOACH_CONFIG`
    (that is `ResolveConfigPath`). So LoadUpgradeConfig using `GlobalConfigPath()` reads ONLY the
    XDG discovery path — exactly "does NOT read flags/env".
  - `noticeOut` (swappable io.Writer, default os.Stderr) is where §19 notices + config-version
    advisories are written via `fmt.Fprint(noticeOut, ...)`.
- `internal/config/load.go`:
  - `Load(ctx, opts) (*Config, error)` — the 7-layer resolver (Defaults→global→repo→git→env→flag).
    Calls `bootstrapWriteConfig` (FR-B3) when no global file + no explicit override + not DisableBootstrap.
    Emits the config-version advisory (`configVersionNotice`/`migrationNotice`) to `noticeOut`.
  - The upgrade command MUST NOT call `Load` (it runs outside a git repo; a no-op
    `PersistentPreRunE` skips it — see `internal/cmd/lock.go`).

## Design decisions (load-bearing)

### D1: UpgradeConfig is a STANDALONE struct, NOT a Config field.
`[upgrade]` is GLOBAL-ONLY (FR-U10) and read by a DEDICATED reader, NOT the full 7-layer
resolver. If it were a `Config` field wired through `materialize`/`overlay`, the full `Load()`
would read `[upgrade]` from BOTH global AND repo `.stagecoach.toml`, violating global-only (a
repo `[upgrade]` would leak in). So `UpgradeConfig` is its own type; `LoadUpgradeConfig()` is
the only seam that reads it. Global-only is enforced three ways:
  1. `UpgradeConfig` is not a `Config` field → the full resolver has no Upgrade to populate.
  2. `LoadUpgradeConfig()` reads ONLY `GlobalConfigPath()` → never the repo file.
  3. `fileConfig.Upgrade` IS decoded by `loadTOML` (so a repo `[upgrade]` parses) but `materialize`
     DROPS it (no copy to `Config`) → a repo `[upgrade]` is a harmless no-op in the full path.

### D2: fileConfig gets an `Upgrade UpgradeConfig` field (decode target).
Add `Upgrade UpgradeConfig \`toml:"upgrade"\`` to `fileConfig`. This honors the contract's
"decode via fileConfig.Upgrade" literally and reuses the existing decode struct (every table is
"known" → zero unknown-field risk). The full `loadTOML` decodes it but `materialize` ignores it.

### D3: Defaults live in an unexported `defaultUpgradeConfig()` helper (config.go).
`Defaults()` returns `Config`, which has no `Upgrade` field (by D1). The [upgrade] defaults
(Channel="stable", SourceRepo="dabstractor/stagecoach", PRD §16.1) live in a package-private
`defaultUpgradeConfig() UpgradeConfig`, co-located with `Defaults()`. `LoadUpgradeConfig()` calls
it as the base, then overlays non-empty file values. This is the single source of truth for
[upgrade] defaults (mirrors how `Defaults()` is the single source for `Config`). `Defaults()` is
NOT modified.

### D4: LoadUpgradeConfig signature + behavior.
`func LoadUpgradeConfig() (UpgradeConfig, error)` in file.go:
  - `out := defaultUpgradeConfig()`.
  - `data, err := os.ReadFile(GlobalConfigPath())`; if `os.IsNotExist(err)` → `return out, nil`
    (mirrors `loadTOML`'s layer-absent sentinel — a missing global file ⇒ defaults, no error).
  - other read error → `fmt.Errorf("read upgrade config: %w", err)`.
  - `toml.Unmarshal(data, &fc)` into `fileConfig` (reuse the struct that now has `Upgrade`); on
    parse error → `fmt.Errorf("parse upgrade config: %w", err)`.
  - non-zero overlay (mirrors `materialize` discipline): `if fc.Upgrade.Channel != "" { out.Channel = ... }`;
    `if fc.Upgrade.SourceRepo != "" { out.SourceRepo = ... }`.
  - NEVER reads repo file / git-config / env / flags; NEVER calls `bootstrapWriteConfig`; NEVER
    writes to `noticeOut` (so NO advisory can fire — FR-B4).

### D5: go-toml/v2 unknown-field behavior is irrelevant (full fileConfig decode).
Decoding the full `fileConfig` means every `[defaults]/[generation]/[role]/[provider]/[upgrade]`
table has a matching struct field → no unknown-field errors regardless of go-toml's default
(which is "ignore unknown", confirmed by the absence of any DisallowUnknownFields call). A
malformed `[defaults].timeout` does NOT error at unmarshal (it's a string field; validated only
later by `loadTOML`'s `time.ParseDuration`). So `LoadUpgradeConfig` is decoupled from the
validity of non-upgrade sections. SAFE.

## PRD anchors (verified in PRD.md)
- §16.1 (line ~1697): "Built-in defaults ... upgrade channel \"stable\" + source_repo
  \"dabstractor/stagecoach\" (§9.29 FR-U10)." (NOTE: this line cites `internal/config/defaults.go`
  which does not exist; the real file is `config.go`. Do not "fix" the PRD text.)
- §16.2 (line ~1744-1746): example `[upgrade]` table:
    channel     = "stable"
    source_repo = "dabstractor/stagecoach"
  with comment: "self-update (§9.29); global config only — no per-repo meaning".
- §9.29 FR-U10 (line ~623): `[upgrade]` is global-only; per-repo `[upgrade]` ignored with a
  `--verbose` note (the note is the upgrade COMMAND's job — P1.M4.T1 — NOT this seam).
- §9.17 FR-B4 (line ~439): additive changes MUST NOT bump config_version and MUST NOT emit an
  advisory.

## Docs target (Mode A)
`docs/configuration.md`:
  - "Built-in defaults" table (line ~126-154): add two rows for `channel` ("stable") and
    `source_repo` ("dabstractor/stagecoach") with a "(global-only, §9.29 FR-U10)" note. These are
    NOT in `config.Defaults()` (they're in `defaultUpgradeConfig()`), so the Source column should
    say `config.defaultUpgradeConfig()` / "LoadUpgradeConfig" — or simply "(§9.29 FR-U10)" to avoid
    pointing at a private symbol in user docs. Prefer the §9.29 FR-U10 cite + a short note.
  - Optionally a short `[upgrade]` subsection under "File format" mirroring the PRD §16.2 block.
Inline comments: the `UpgradeConfig` struct + `defaultUpgradeConfig` + `fileConfig.Upgrade` +
`LoadUpgradeConfig` each get a doc/inline comment citing FR-U10 / FR-B4 / §16.1.

## Test plan (mirror file_test.go + load_test.go table style; white-box `package config`)
1. `TestLoadUpgradeConfig_NoFile` — global file missing (XDG temp dir empty) → defaults, nil.
2. `TestLoadUpgradeConfig_WithoutUpgradeTable` — global file with only `[defaults]` → defaults.
3. `TestLoadUpgradeConfig_WithUpgradeTable` — `[upgrade]` channel+source_repo set → merged.
4. `TestLoadUpgradeConfig_PartialUpgradeTable` — only `channel` set → channel overridden, source_repo default.
5. `TestLoadUpgradeConfig_OlderVersionFile` — `config_version=2` file WITH `[upgrade]` → merged, no error.
6. `TestLoadUpgradeConfig_BadTOML` — malformed file → wrapped error (contains "parse upgrade config").
7. `TestLoadUpgradeConfig_NoAdvisory` — capture `noticeOut` (strings.Builder); assert empty after LoadUpgradeConfig
   on a file WITH `[upgrade]` (FR-B4: no advisory).
8. `TestLoadUpgradeConfig_ReadError` (optional) — non-not-exist read error path (hard to trigger portably; the
   IsNotExist branch + BadTOML cover the error discipline).
9. `TestDefaultUpgradeConfig` — assert `defaultUpgradeConfig()` == {Channel:"stable", SourceRepo:"dabstractor/stagecoach"}.
10. `TestCurrentConfigVersion_Unchanged` — assert `CurrentConfigVersion == 3` (FR-B4 regression guard).
11. (Optional, full-resolver) `TestLoad_FullResolverDropsRepoUpgrade` — full `config.Load` with a repo
    `.stagecoach.toml` containing `[upgrade]` does NOT surface it (no Config.Upgrade field) and a global
    v3 file with `[upgrade]` emits NO advisory (capture noticeOut).

Test seam: `t.Setenv("XDG_CONFIG_HOME", tempDir)` then write `<tempDir>/stagecoach/config.toml`.
This is part of the global path resolution (not an excluded env/flag), matching TestGlobalConfigPath
and loadEnvSetup exactly.

## Validation (verified locally)
- `go vet ./internal/config/` → exit 0 ✓
- `go test ./internal/config/` → PASS (5.3s) ✓ (currently green; must stay green)
- `gofmt -l internal/config/` → must be empty
- golangci-lint is NOT on the local PATH (CI runs it via .golangci.yml); use `go vet` + `gofmt` for gates.

## Scope boundaries (do NOT touch)
- `bootstrap.go` (config init template) — writing `[upgrade]` into the on-disk template is a
  config-init concern, out of scope. [upgrade] is OPTIONAL/additive; a file without it → defaults.
- `load.go` `materialize`/`overlay` — do NOT add Upgrade plumbing (it would break global-only).
- `CurrentConfigVersion` — stays 3.
- The upgrade COMMAND (`internal/cmd/upgrade.go`) and the `--verbose` ignore-note are P1.M4.T1,
  not this seam. This task ONLY provides `LoadUpgradeConfig()`.
- No network, no commit-path code.
