# Issue 3: Config Upgrade Missing Backup (FR-B8)

## Root Cause

`config upgrade` overwrites the config file in-place via `os.WriteFile` with NO preceding backup.
This violates FR-B8: "Every command that writes the config file — `config init`, `config init --force`
and `--template`, `config upgrade`, and the install/first-run bootstrap — must also leave a timestamped
backup of the prior file alongside it."

## Bug Location

`internal/cmd/config.go:157-191`, function `runConfigUpgrade`:

```go
func runConfigUpgrade(cmd *cobra.Command, args []string) error {
    path := config.ResolveConfigPath(flagConfig)
    data, err := os.ReadFile(path)                          // line 159
    // ... validity gates (not-exist, TOML parse, inert no-op) ...
    newContent, changed := upgradeConfigVersion(string(data), config.CurrentConfigVersion)  // line 181
    if !changed { return nil }                              // line 182 (already-current no-op)
    if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {   // line 186 ← NO BACKUP
        return exitcode.New(exitcode.Error, fmt.Errorf("write config %s: %w", path, err))
    }
    fmt.Fprintf(cmd.OutOrStdout(), "Upgraded config at %s to version %d.\n", path, config.CurrentConfigVersion)
    return nil
}
```

There is NO `config.WriteTimestampedBackup(path)` call between the `changed` check (line 182)
and the `os.WriteFile` (line 186).

## Correct Reference Pattern

`internal/cmd/config.go:504-533`, function `writeBootstrapFile` (used by `config init --force`):

```go
func writeBootstrapFile(cmd *cobra.Command, path, content string, force bool) error {
    // ... MkdirAll ...
    if force {
        if _, err := os.Stat(path); err == nil {            // existing file present
            if backup, berr := config.WriteTimestampedBackup(path); berr != nil {
                return exitcode.New(exitcode.Error, fmt.Errorf("backup existing config %s: %w", path, berr))
            } else if backup != "" {
                fmt.Fprintf(cmd.OutOrStderr(), "Backed up previous config to %s\n", backup)
            }
        }
    }
    if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
        return exitcode.New(exitcode.Error, fmt.Errorf("write config %s: %w", path, err))
    }
    return nil
}
```

## WriteTimestampedBackup Signature

`internal/config/backup.go:18`:
```go
func WriteTimestampedBackup(path string) (string, error)
```
- Copies `path` → `path + ".bak.<RFC3339-compact-UTC>"` (e.g. `config.toml.bak.2026-07-10T120000Z`).
- Returns `("", nil)` if path does not exist (nothing to back up).
- Returns `(backupPath, nil)` on success.
- Returns `("", error)` on read/write failure (hard error — never clobber without recovery).

## Fix Location

In `runConfigUpgrade` (`internal/cmd/config.go`), insert a backup block between line 182
(the `if !changed` return) and line 186 (the `os.WriteFile`):

```go
// After the `if !changed { … return nil }` block, BEFORE the os.WriteFile:
if backup, berr := config.WriteTimestampedBackup(path); berr != nil {
    return exitcode.New(exitcode.Error, fmt.Errorf("backup existing config %s: %w", path, berr))
} else if backup != "" {
    fmt.Fprintf(cmd.OutOrStderr(), "Backed up previous config to %s\n", backup)
}
```

**No new import needed** (`internal/config` already imported; `fmt`/`exitcode` in scope).
**No new function needed** — reuse `WriteTimestampedBackup`.

The backup runs ONLY when `changed == true` (a real write happens). The inert (line 174),
already-current (line 182), malformed-TOML, and no-file paths all return early without writing,
so they correctly skip the backup.

## Existing Tests

- `config init --force` backup assertions exist at `internal/cmd/config_test.go:631-634, 675-678`
  (pattern: `filepath.Glob("config.toml.bak.*")` → assert ≥1 match).
- `config upgrade` tests (`TestConfigUpgrade_*` at lines 1058-1500) assert ONLY content — NONE
  assert a backup exists. `TestConfigUpgrade_OlderUpdated` (line 1137) and
  `TestConfigUpgrade_V2ToV3Rewrite` (line 1423) are the tests to extend with backup assertions.

## Risks

- **Timestamp collision**: `WriteTimestampedBackup` is second-granular; two upgrades within the
  same UTC second collide on the backup filename (the second write fails as a hard error). Same
  behavior as `config init --force` today — acceptable and symmetric.
- **Scope**: The fix is a single localized insertion. Does NOT touch the pure transform
  (`upgradeConfigVersion`) or the inert/already-current gates. No risk of altering upgrade semantics.
