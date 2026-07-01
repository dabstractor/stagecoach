# P1.M4.T3.S1 — Design Decisions & Findings

Scope: add `stagehand config upgrade` (PRD §9.17 FR-B5) — a cobra subcommand that rewrites an existing
global config in place to `config_version = CurrentConfigVersion` via a minimal textual transform that
preserves every other line. Plus FR-B6 help de-duplication (remove the manual "Subcommands:" block from
the `config` and `providers` parent commands). Plus shouldSkipConfigLoad("upgrade").

## Decisions

**D1 — Textual transform, NOT TOML round-trip.** FR-B5 mandates "preserving user values … comments out
removed/renamed keys … leave all other content unchanged." go-toml/v2 marshal drops ALL comments and
reorders/reformats (see external-research.md §1). So the upgrade reads the file as TEXT, sets/adds ONLY
the top-level `config_version` line, and leaves every other byte identical. A single `toml.Unmarshal`
into `map[string]any` is used ONLY as a validity gate (refuse to mangle an unparseable file) — never
marshalled back.

**D2 — `upgradeConfigVersion(content string, version int) (string, bool)` is PURE.** No I/O, no error —
it returns the new text + a `changed` bool. Deterministic → fully unit-testable (no filesystem, no
GlobalConfigPath). `runConfigUpgrade` does the I/O (read → gate → transform → write → message) and
delegates the text mutation to this pure function. This mirrors the testability split the parallel
sibling (P1.M4.T2.S1) used for `buildBootstrapConfig` (F2 there).

**D3 — Scan only the top-level region; break at the first `[table]` header.** `config_version` is root
metadata (`toml:"config_version"` on `fileConfig`); TOML requires root keys before any `[table]` (external-
research.md §2). So the scan for an existing `config_version = N` walks lines until the first `[...]`
header, then stops. A `config_version` after a table is a different key — never matched (no false hit,
no duplicate root key).

**D4 — Three transform outcomes (idempotent by construction):**
  - existing top-level `config_version = N` with N != version → rewrite that ONE line's value; `changed=true`.
  - existing top-level `config_version = version` (already current) → return content byte-identical; `changed=false` (the "already up to date" path).
  - no top-level `config_version` → insert `config_version = <version>` after the leading comment/blank
    header block (so it sits naturally with other root keys, before the first table); `changed=true`.
Running twice: 2nd run hits the "already current" branch → no rewrite. Idempotent. (external-research.md §4.)

**D5 — v2.0 has NO removed/renamed keys → only the version line is touched.** FR-B5's "comments out
removed/renamed keys with a note" is FUTURE-EXTENSIBLE behavior (a v3 bump may add a migration step); for
v2.0 there is nothing to remove/rename ("There are no existing users to migrate … the first upgrade
simply adds config_version=N and is a no-op otherwise"). The textual approach inherently preserves all
user values, satisfying "preserves user values for keys that still exist" automatically. Do NOT invent
key migrations.

**D6 — Missing file → exit 1 pointing at `config init`.** `os.ReadFile(GlobalConfigPath())` IsNotExist →
`exitcode.New(exitcode.Error, fmt.Errorf("no config file at %s (run 'stagehand config init' first)", path))`.
Upgrade targets an EXISTING file (unlike `init`, which creates one). Consistent with the load.go advisory
wording (which suggests both `config upgrade` and `config init --force`).

**D7 — `--config`/`STAGEHAND_CONFIG` is intentionally NOT honored.** The work item INPUT names
`GlobalConfigPath()` (the GLOBAL file). Upgrade is in shouldSkipConfigLoad (config.Load does NOT run), so
the Layer-7 discovery override isn't resolved. Upgrade rewrites the global file; `--config` is a read-path
discovery override (semantically different). Reading the persistent flag manually would contradict the
contract. Note this in the command's Long help ("upgrades the GLOBAL config at `config path`").

**D8 — `configUpgradeCmd` is registered in `config.go`'s `init()`; "upgrade" added to shouldSkipConfigLoad.**
cobra auto-lists it in `Available Commands` (so FR-B6 dedup needs NO manual subcommand list). Add
`|| name == "upgrade"` to shouldSkipConfigLoad (root.go) so it works outside a repo (no git-config layer,
no config.Load). `Args: cobra.NoArgs` (extra args → exit 1), matching configInitCmd/configPathCmd.

**D9 — FR-B6 help de-dup: remove the manual "Subcommands:" block from `configCmd.Long` AND
`providersCmd.Long`.** cobra's auto-generated "Available Commands" is the single source (PRD §9.17 FR-B6;
the v1 `stagehand config` showed init/path both in prose AND in Available Commands). Removing the block
makes the new `upgrade` (and any future leaf) appear with zero extra edits. The contract's Mode-A "update
the config command group Long help to list the upgrade subcommand" is satisfied by REGISTRATION (cobra
auto-lists it) — do NOT re-add a manual list.

**D10 — Mode-A Long updates are minimal:** `configUpgradeCmd` gets a full Mode-A Long describing the
in-place rewrite, idempotency, preservation, and the missing-file remediation. `configCmd.Long`'s intro
prose is updated to drop the removed block (and reflect upgrade conceptually) but NOT re-list subcommands.

## Findings

**F1 — `CurrentConfigVersion = 2` (const, internal/config/config.go:18).** Read-only — the value the
upgrade writes. Do NOT read `Defaults().ConfigVersion` (it's the 0 "unset" sentinel). Use the const.

**F2 — `GlobalConfigPath()` (internal/config/file.go:83)** is the write/read target (the work item INPUT).
Already used by `runConfigPath`. In tests, `setupNoRepo` sets HOME=XDG=t.TempDir() so the path is
`<tmp>/stagehand/config.toml`.

**F3 — shouldSkipConfigLoad lives in root.go:97.** `func shouldSkipConfigLoad(cmd) bool { name :=
cmd.Name(); return name == "init" || name == "path" }`. Add `|| name == "upgrade"`. The PARALLEL sibling
P1.M4.T2.S1 does NOT edit root.go (its scope says "root.go … do NOT edit"), so this edit is conflict-free.

**F4 — The PARALLEL sibling P1.M4.T2.S1 rewrites `internal/cmd/config.go` (config init).** It KEEPS
`configCmd`'s manual "Subcommands:" block ("do NOT remove configCmd's 'Subcommands:' block; only update
the init line") and does NOT implement `config upgrade` ("config upgrade (P1.M4.T3)"). ⇒
- CONFLICT POINT: BOTH tasks edit `configCmd.Long` in config.go. The sibling updates the `init` line in
  the Subcommands block; THIS task REMOVES the whole block (FR-B6). **Sequencing: the sibling lands first
  (its PRP assumes it runs before T3); THIS task's edit removes whatever "Subcommands:" block then
  exists.** Describe the edit generically ("remove the manual 'Subcommands:' block from configCmd.Long").
- The sibling ADDS imports + helpers + a rewritten runConfigInit + buildBootstrapConfig to config.go. My
  ADDITIONS (configUpgradeCmd, runConfigUpgrade, upgradeConfigVersion, the AddCommand line) are
  independent of those — no overlap in the lines they touch. Both append a `configCmd.AddCommand(...)`
  line in `init()` (sibling keeps init/path; I add upgrade) — coordinate by adding to the SAME init().

**F5 — providers.go's "Subcommands:" block is NOT touched by the sibling** (sibling scope: providers.go
do-not-edit). So removing it (FR-B6) is conflict-free. providersCmd.Long currently lists `list`/`show`.

**F6 — The load.go advisory (P1.M4.T1.S1) already names `config upgrade`.** `configVersionNotice`
(load.go:263) emits, for a missing/older version: *"Run 'stagehand config upgrade' or 'stagehand config
init --force'."* So this command is the documented remediation — it MUST exist and behave as the advisory
implies (rewrite in place to CurrentConfigVersion; safe if already current).

**F7 — Test conventions (internal/cmd/config_test.go):** `setupNoRepo(t)` (isolates HOME/XDG, chdir to a
plain dir, returns globalDir), `saveRootState`/`restoreRootState` (rootCmd singleton hygiene — cobra's
rootCmd is a package global), drive via `rootCmd.SetArgs([...])` + `Execute(context.Background())`, assert
via `exitcode.For(err)` + `os.ReadFile(config.GlobalConfigPath())`. The file already imports `regexp`
(upgradeConfigVersion can use it). `upgradeConfigVersion` is tested DIRECTLY (same package, pure) for
determinism; runConfigUpgrade is tested via Execute for the I/O/error/missing-file paths.

**F8 — go.mod/go.sum UNCHANGED.** go-toml/v2 (the validity gate) is already a dep; cobra/exitcode already
imported in config.go. No new imports beyond `strconv` (maybe) — and `regexp`/`strings` (strings already
imported; regexp is in the test file already but the IMPL may need it — add it). `internal/config` +
`internal/exitcode` already imported in config.go.

**F9 — config.go package doc + comments mention only init/path.** After this task, update the package
doc comment and configCmd doc to mention the third leaf (`upgrade`). Minor Mode-A touch.

## Test plan (mirrors config_test.go)

`internal/cmd/config_test.go` (ADD; reuse setupNoRepo/saveRootState/restoreRootState):

PURE unit tests (call upgradeConfigVersion DIRECTLY — no Execute, no FS):
- `TestUpgradeConfigVersion_NoVersion_Inserts`: content with `[defaults]` but no config_version → returns
  content with `config_version = 2` inserted before the first table; `changed=true`; ALL other lines
  byte-identical (assert original lines are a subset / unchanged).
- `TestUpgradeConfigVersion_OlderVersion_Updates`: `config_version = 1\n...` → value becomes 2;
  `changed=true`; other lines unchanged.
- `TestUpgradeConfigVersion_CurrentVersion_NoChange`: `config_version = 2\n...` → returned byte-identical;
  `changed=false`.
- `TestUpgradeConfigVersion_CommentedVersionIgnored`: `# config_version = 2\n[defaults]...` → the
  commented line is NOT matched (regex anchored at col 0, no `#`) → inserts an uncommented
  `config_version = 2`; `changed=true`; the original comment line preserved.
- `TestUpgradeConfigVersion_VersionInTableNotMatched`: a `config_version` AFTER a `[defaults]` header is
  NOT the schema key → top-level scan breaks at the table → inserts a top-level one; no duplicate root key
  (parse the result: `toml.Unmarshal` succeeds, root `config_version == 2`).
- `TestUpgradeConfigVersion_Idempotent`: apply once to a no-version input, then apply again to the result
  → 2nd returns `changed=false`, byte-identical to the 1st result.

Execute-driven tests (runConfigUpgrade via rootCmd):
- `TestConfigUpgrade_NoFile_Errors`: setupNoRepo (no file) → SetArgs(["config","upgrade"]) → exit 1
  (exitcode.Error); err Contains "config init".
- `TestConfigUpgrade_AddsVersion`: pre-write a config WITHOUT config_version (e.g. `[defaults]\nprovider
  = "pi"\n`) → Execute → exit 0; file now CONTAINS `config_version = 2` AND still Contains `provider =
  "pi"` (preserved); stdout Contains "Upgraded".
- `TestConfigUpgrade_AlreadyCurrent`: pre-write `config_version = 2\n[defaults]\nprovider="pi"\n` →
  Execute → exit 0; file BYTE-IDENTICAL to the input; stdout Contains "no changes" (or "already").
- `TestConfigUpgrade_OlderUpdated`: pre-write `config_version = 1\n[generation]\nmax_md_lines = 7\n` →
  Execute → exit 0; file Contains `config_version = 2` and `max_md_lines = 7` (preserved).
- `TestConfigUpgrade_Idempotent`: run _AddsVersion, then run Execute again → 2nd exit 0, file unchanged
  from after the 1st run.
- `TestConfigUpgrade_MalformedTOML`: pre-write `bad {toml` → Execute → exit 1; err Contains "not valid
  TOML"; file UNCHANGED (not rewritten).
- `TestConfigUpgrade_WorksOutsideRepo`: (covered by setupNoRepo — no git repo; shouldSkipConfigLoad true).
- `TestConfigUpgrade_ExtraArgsExits1`: SetArgs(["config","upgrade","x"]) → exit 1 (cobra.NoArgs).
