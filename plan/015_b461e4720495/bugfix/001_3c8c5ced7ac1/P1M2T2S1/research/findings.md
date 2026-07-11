# P1.M2.T2.S1 Research Findings — Insert WriteTimestampedBackup in runConfigUpgrade (Issue 3, FR-B8)

Source: `architecture/config_upgrade_backup.md`, `internal/cmd/config.go` (runConfigUpgrade + writeBootstrapFile),
`internal/config/backup.go`, `internal/cmd/config_test.go` (the 3 upgrade tests), and the parallel PRP P1.M2.T1.S2.

## 0. The bug (one line, no backup)

`runConfigUpgrade` (`internal/cmd/config.go:157`) overwrites the config in-place at `:186`
(`os.WriteFile(path, []byte(newContent), 0o644)`) with NO preceding backup. This violates FR-B8 ("every config-writing
command must also leave a timestamped backup of the prior file"). `config init --force` (writeBootstrapFile :521-528)
DOES back up; `config upgrade` does not — an asymmetry.

## 1. The exact insertion point + the exact code

Between the `if !changed { … return nil }` block (closes at the `}` before :186) and the `os.WriteFile` call (:186),
insert (verbatim from the contract / the writeBootstrapFile :521-528 inner block):

```go
// FR-B8 reversible-write guarantee (mirrors writeBootstrapFile): back up the prior config BEFORE the overwrite so
// every upgrade is undoable. runConfigUpgrade already proved the file exists (os.ReadFile at the top succeeded), so
// no os.Stat guard is needed; WriteTimestampedBackup is nil-safe for a missing file regardless.
if backup, berr := config.WriteTimestampedBackup(path); berr != nil {
    return exitcode.New(exitcode.Error, fmt.Errorf("backup existing config %s: %w", path, berr))
} else if backup != "" {
    fmt.Fprintf(cmd.OutOrStderr(), "Backed up previous config to %s\n", backup)
}
```

## 2. Why no os.Stat guard is needed (divergence from writeBootstrapFile)

writeBootstrapFile wraps the backup in `if force { if _, err := os.Stat(path); err == nil { …backup… } }` because
`config init` can be a FIRST-TIME write (no existing file → nothing to back up). `runConfigUpgrade` is DIFFERENT:
it has already done `os.ReadFile(path)` at :159 and returned "no config file at …" on IsNotExist — so by the time we
reach the write, the file DEFINITELY exists. No Stat guard needed. AND `WriteTimestampedBackup` is itself nil-safe
(returns "", nil for a missing file), so even defensively it's correct. Calling it directly is the faithful, simpler mirror.

## 3. WriteTimestampedBackup contract (backup.go:18 — verified)

```go
func WriteTimestampedBackup(path string) (string, error)
```
- `("", nil)` if path does not exist (nothing to back up).
- `(backupPath, nil)` on success — backupPath = `path + ".bak.<RFC3339-compact-UTC>"` (e.g. `config.toml.bak.2026-07-10T120000Z`).
- `("", error)` on read/write failure (HARD error — the caller must NOT proceed to clobber).

## 4. Imports already in scope — NO new import

config.go's import block has `fmt`, `os`, `path/filepath`, etc. The existing `runConfigUpgrade` body already calls
`config.ResolveConfigPath`, `config.CurrentConfigVersion`, `config.IsInert`, `exitcode.New`, `fmt.Errorf`, `fmt.Fprintf`,
`os.WriteFile`. So `internal/config` (as `config`), `exitcode`, and `fmt` are ALL imported and used. The insertion adds
`config.WriteTimestampedBackup` (already-imported package) + `exitcode.New`/`fmt.Errorf`/`fmt.Fprintf` (all in scope).
**Zero new imports. Zero new functions.**

## 5. The backup runs ONLY on a real write (the early-return paths skip it)

The insertion is AFTER these gates, all of which return early WITHOUT reaching the backup:
- `:164` no-file (`os.IsNotExist` → "no config file")
- `:171` malformed TOML (`toml.Unmarshal` error)
- `:177` inert file (`config.IsInert` → "nothing to upgrade")
- `:182` already-current (`!changed` → "no changes")

So the backup fires ONLY when `changed == true` (a genuine overwrite). This matches FR-B8: back up the prior file only
when you actually overwrite it. No spurious backups on no-op runs.

## 6. CRITICAL: existing upgrade tests stay GREEN (verified — stderr is discarded)

This is the key validation insight. The 3 existing upgrade tests in `internal/cmd/config_test.go`:
- **TestConfigUpgrade_OlderUpdated** (:1137): `rootCmd.SetErr(io.Discard)` → my "Backed up previous config to …" notice
  (which goes to `cmd.OutOrStderr()`) is DISCARDED. Asserts stdout contains "Upgraded" (unchanged) + content (unchanged).
  Does NOT glob the dir / assert file count → the new `.bak.*` file is harmless. ✓
- **TestConfigUpgrade_V2ToV3Rewrite** (:1423): `SetErr(io.Discard)` → notice discarded. First run: changed=true → backup
  created (of the v2 content — correct, FR-B8 = prior file). Asserts upgraded content (unchanged). SECOND run (idempotency):
  file is now v3 → `changed==false` → early return → NO backup created → `string(data2) != upgraded` still holds. ✓
- **TestConfigUpgrade_Idempotent** (:1186): `SetErr(io.Discard)`. First run changed=true → backup created (harmless, not
  asserted). Second run changed=false → "no changes" + content-equal assertions hold. ✓

So `make test` stays GREEN after the fix WITHOUT needing S2. S2 ADDS positive backup assertions (glob `config.toml.bak.*`
→ ≥1 match, mirroring config_test.go:631-634); the existing tests neither require nor forbid the backup. The fix is
behaviorally invisible to them.

## 7. Idempotency is preserved (no double-backup, no content change on 2nd run)

The 2nd/no-op run hits `!changed` at :182 and returns BEFORE the backup code. So: no second backup, no content change.
The existing `secondContent == firstContent` idempotency assertions hold. (WriteTimestampedBackup's second-granular
timestamp collision only matters if two REAL upgrades happen in the same UTC second — impossible in the idempotency tests
since the 2nd is a no-op.)

## 8. Scope fence — production code ONLY; tests are S2; no docs

- THIS task (S1): edit `internal/cmd/config.go` (runConfigUpgrade) — the single insertion. NOTHING else.
- S2 (P1.M2.T2.S2, separate): add backup assertions to TestConfigUpgrade_OlderUpdated + TestConfigUpgrade_V2ToV3Rewrite
  (clone the config_test.go:631-634 glob idiom). I do NOT write these.
- DOCS: none (the backup is transparent user-facing behavior; FR-B8 is already documented).
- NO change to `upgradeConfigVersion` (the pure transform), the inert/already-current gates, writeBootstrapFile, or backup.go.

## 9. Parallel-sibling non-overlap

P1.M2.T1.S2 (parallel, currently implementing) is TEST-ONLY in `internal/config/bootstrap_test.go` — a DIFFERENT package
and file from my edit (`internal/cmd/config.go`). ZERO file overlap → no merge conflict. (It tests the commented-pi-block
blanking from Issue 2; unrelated to the upgrade backup.)

## 10. Validation (verified Makefile commands)

- `go build ./...` — the insertion compiles (no new import; all symbols in scope).
- `make test` (= `go test -race ./...`) — GREEN: the 3 existing upgrade tests pass (§6). S2 later adds positive assertions.
- `make lint` + `make coverage-gate` — green (internal/cmd is NOT in the coverage-gate set; only git/provider/generate/config
  are; but the insertion is coverage-neutral anyway).
- `gofmt -l internal/cmd/config.go` — empty (the block is gofmt-conventional).
- Grep guard: exactly ONE `config.WriteTimestampedBackup(path)` call site added in runConfigUpgrade (2 total in config.go:
  the new one + writeBootstrapFile's existing one).
