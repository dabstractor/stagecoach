# P1.M3.T3.S1 — Research Findings (--rollback: restore backup via swap+sanity-run; no-op when none)

Source: direct codebase reads + the LANDED swap*.go (parallel P1.M3.T2.S1 is DONE on disk) + PRD
FR-U8. No external research (pure stdlib os.Rename + the existing upgrade primitives). ~9 tool calls.

## §0 — FR-U8 (verbatim, prd_snapshot.md line 621)

> **FR-U8. `--rollback`.** `stagecoach upgrade --rollback` restores the most recent backup (the
> `.stagecoach-backup` / `.exe.old` file) over the current binary, using the same atomic-swap +
> sanity-run discipline (a backup whose `--version` no longer runs is refused with an explanation).
> Without a backup it is a no-op reported as such. Rollback is a one-step undo: only the
> immediately-prior version is retained.

CONTRACT (item description): `Rollback(ctx) (restoredVersion string, err error)` in
internal/upgrade. backup absent → ErrNoBackup (command layer prints "no backup — nothing to roll
back", exit 0). backup unrunnable → error, current unchanged. backup OK → swap it into place, return
its version string. Mocking: temp-dir exe + fake backup binaries (compiled stubs).

## §1 — CRITICAL DESIGN REALIZATION: Rollback CANNOT reuse Swap

The LANDED `Swap` → `platformSwap` (swap_unix.go:40) backs-up-the-CURRENT FIRST:
```go
func platformSwap(currentExe, newBinPath string) error {
	backup := backupPath(currentExe)
	os.Rename(currentExe, backup)   // ← backs up CURRENT over the backup path
	os.Rename(newBinPath, currentExe)
}
```
If Rollback called `Swap(ctx, backupPath)`, step 1 would `os.Rename(currentExe, <exe>.stagecoach-backup)`
→ OVERWRITE the backup (the restore source!) with the current content. Then step 2 moves it back.
Net: current UNCHANGED + backup DESTROYED. Wrong on both counts. (Even a "copy backup to temp, then
Swap(temp)" trick leaves the backup holding the previous-current, making rollback a 2-state toggle —
violating FR-U8's one-step "previous-current is lost".)

DECISION: Rollback does its OWN platform-specific restore via NEW `platformRollback` build-tag twins
(in rollback_unix.go / rollback_windows.go) that move the backup DIRECTLY into place and CONSUME it.
This matches the item's "NO new backup-of-current … rollback overwrites current with backup, and the
previous-current is lost" decision exactly.

## §2 — The LANDED primitives Rollback CONSUMES (swap*.go + stage.go — treat as contracts)

All in package `upgrade` (same package ⇒ the unexported helpers are reachable from rollback*.go):
- `var resolveCurrentExe = func() (string, error)` (swap.go:60) — injectable exe-path seam. Override
  in tests via `restoreResolveCurrentExe` (swap_test.go:24: `var restoreResolveCurrentExe = resolveCurrentExe`
  captured at init; tests do `resolveCurrentExe = …; defer func(){ resolveCurrentExe = restoreResolveCurrentExe }()`).
- `func backupPath(exe string) string` (swap_unix.go:18 → `exe+".stagecoach-backup"`; swap_windows.go:18
  → `exe+".old"`) — the STABLE backup-name contract (the parallel PRP made it a shared helper FOR this item).
- `var execVersion = func(ctx, path) ([]byte, error)` (stage.go:65) — the sanity-run seam. Runs
  `path --version`, returns output + error. Override via `saveExecVersion()`/`restoreExecVersion(saved)`
  (stage_test.go:189/194). The DEFAULT leaves cmd.Env unset ⇒ child inherits os.Environ (incl. t.Setenv).
- (NOT consumed: `isWritable` / `privilegeCommand` / `NeedsPrivilegesError`. See §5 — Rollback adds no
  privilege gate; `privilegeCommand` is hardcoded `sudo "<exe>" upgrade`, wrong for `--rollback`.)

## §3 — Why Rollback uses execVersion directly (NOT stage.go's sanityCheck)

stage.go's `sanityCheck(ctx, path, wantTag)` requires the output to CONTAIN a specific `wantTag`
(the target release tag). Rollback has NO target tag — the backup's version is whatever it is; FR-U8
only requires "whose --version no longer runs is refused" = exit-0-on---version. So Rollback runs
`execVersion(ctx, backup)` itself: error (run-fail / non-zero-exit) ⇒ ErrBackupUnusable; else the
trimmed output IS the restoredVersion. Clean, no wantTag needed.

## §4 — The platform twins (NEW — rollback_unix.go / rollback_windows.go)

**Unix** (`//go:build !windows`) — a SINGLE atomic rename (the running process keeps its old inode):
```go
func platformRollback(currentExe, backup string) error {
	if err := os.Rename(backup, currentExe); err != nil {  // atomic replace; backup FILE consumed
		return fmt.Errorf("restore backup into place: %w", err)
	}
	return nil
}
```
os.Rename(backup, currentExe) on the SAME filesystem atomically replaces the directory entry; the
running process's text mapping holds the OLD inode (reclaimed at exit), so THIS process keeps running
the now-overwritten binary while the NEXT invocation runs the restored (prior) version. Never zero
runnable binaries. One-step (FR-U8): the backup file is CONSUMED (moved into place); previous-current
lost. (`backup` and `currentExe` are siblings in the install dir ⇒ same filesystem ⇒ rename works.)

**Windows** (`//go:build windows`) — the running .exe is LOCKED (can't overwrite in place, CAN rename).
A 2-file swap needs a 3rd name; route the prior-current through `.old` so the EXISTING CleanupOldBinary
reclaims it at next launch (no new litter):
```go
func platformRollback(currentExe, backup string) error {  // backup == currentExe + ".old"
	dir := filepath.Dir(currentExe)
	tf, err := os.CreateTemp(dir, ".stagecoach-rbk-*"); if err != nil { return ... }
	aside := tf.Name(); tf.Close(); os.Remove(aside)        // free a fresh scratch name
	if err := os.Rename(backup, aside); err != nil { return ... }      // 1. stash backup; free .old
	if err := os.Rename(currentExe, backup); err != nil {              // 2. running→.old (allowed)
		_ = os.Rename(aside, backup); return ...                        //    restore backup to .old
	}
	if err := os.Rename(aside, currentExe); err != nil {               // 3. backup content→place
		_ = os.Rename(backup, currentExe); _ = os.Rename(aside, backup); return ...  // FR-U11 restore
	}
	return nil  // currentExe = backup content; .old = prior-current (CleanupOldBinary reclaims next launch)
}
```
Final Windows state: `currentExe` = restored (backup) content; `.old` = prior-current (the now-unwanted
new binary) → CleanupOldBinary deletes it at next launch. The `aside` scratch name is consumed by
step 3 (no litter). FR-U11: each step's failure reverses the prior (best-effort) so the path always
resolves to a runnable image.

## §5 — SCOPE DECISION: no proactive privilege gate in Rollback

The contract pins 3 cases (absent / unrunnable / restored). Adding `isWritable` + `NeedsPrivilegesError`
would (a) be scope creep and (b) print the WRONG command — `privilegeCommand` is hardcoded
`sudo "<exe>" upgrade` (FR-U7's upgrade re-run form), not `--rollback`. So Rollback does NOT probe
writability; a non-writable install dir surfaces as a plain rename EACCES error (propagated). The
sanity-run-before-swap ordering preserves the "current unchanged" guarantee for the unrunnable case;
the rename-error case leaves the backup + current intact on both platforms (Unix: failed rename
changes nothing; Windows: the ordered dance restores). The command layer (P1.M4.T2) may layer sudo
guidance later if desired — out of scope here.

## §6 — File structure (ZERO overlap with the LANDED swap*.go)

- `internal/upgrade/rollback.go` (NEW, shared, no build tag) — `Rollback(ctx)` + `ErrNoBackup` +
  `ErrBackupUnusable`. Calls resolveCurrentExe (swap.go) + backupPath (swap twins) + execVersion
  (stage.go) + platformRollback (rollback twins).
- `internal/upgrade/rollback_unix.go` (NEW, `//go:build !windows`) — `platformRollback` (single rename).
- `internal/upgrade/rollback_windows.go` (NEW, `//go:build windows`) — `platformRollback` (3-step rotate).
- `internal/upgrade/rollback_test.go` (NEW, `//go:build !windows`, NOT parallel — mutates
  resolveCurrentExe) — the 3 contract cases via a REAL compiled cmd/stubcli backup.
- `internal/upgrade/rollback_windows_test.go` (NEW, `//go:build windows`) — the .old 3-step dance.

NO edit to swap.go/swap_unix.go/swap_windows.go/stage.go/version.go/main.go/exitcode.go. NO new
import beyond stdlib (context, errors, fmt, os, bytes, path/filepath). FR-U12 (stdlib-only) holds.

## §7 — Test design (real cmd/stubcli as the backup — matches stage_test.go + the contract)

Reuse `buildStubCLI(t)` (stage_test.go:63 — compiles cmd/stubcli once per process) + `restoreResolveCurrentExe`
(swap_test.go:24). stubcli ignores args, prints STAGECOACH_STUBCLI_OUT, exits STAGECOACH_STUBCLI_EXIT —
so `backup --version` is exactly "does the backup run + exit 0" (FR-U8). The DEFAULT execVersion is
used (no saveExecVersion needed) — the stub child inherits os.Environ incl. t.Setenv (proven by
stage_test.go's HappyPath).

- **TestRollback_RestoresBackup**: tempDir; exe=…/stagecoach (write "CURRENT"); backup=exe+".stagecoach-
  backup" = a COPY of buildStubCLI(t); t.Setenv("STAGECOACH_STUBCLI_OUT","v9.9.9\n"); resolveCurrentExe→exe
  (restore via restoreResolveCurrentExe). Rollback(ctx) → assert err==nil, version contains "v9.9.9";
  assert read(exe) == stubcli bytes (restored); assert backup file NO LONGER EXISTS (consumed — FR-U8 one-step).
- **TestRollback_NoBackup**: tempDir; exe; NO backup. resolveCurrentExe→exe. Rollback → ("",err);
  assert errors.Is(err, ErrNoBackup); assert read(exe)=="CURRENT" (unchanged).
- **TestRollback_BackupUnusable**: backup = stubcli copy; t.Setenv("STAGECOACH_STUBCLI_EXIT","1") (stub
  exits non-zero → execVersion errors). Rollback → ("",err); assert errors.Is(err, ErrBackupUnusable);
  assert read(exe)=="CURRENT" AND read(backup)==stubcli bytes (BOTH unchanged — sanity-run before swap).

Windows twin (rollback_windows_test.go): TestPlatformRollback_OldDance — exe=…\stagecoach.exe "NEW",
old=exe+".old" "OLD"; platformRollback(exe, old); assert read(exe)=="OLD", read(exe+".old")=="NEW",
and only those 2 files remain (no aside litter). + TestRollback_NoBackup mirror.

rollback_test.go MUST NOT call t.Parallel() (mutates the package-level resolveCurrentExe seam — same
constraint as swap_test.go:5 / stage_test.go:2).

## §8 — Validation commands (verified)

```bash
go build ./... && GOOS=linux go build ./... && GOOS=darwin go build ./... && GOOS=windows go build ./...
go vet ./internal/upgrade/...
gofmt -l internal/upgrade/rollback.go internal/upgrade/rollback_unix.go internal/upgrade/rollback_windows.go \
       internal/upgrade/rollback_test.go internal/upgrade/rollback_windows_test.go
go test ./internal/upgrade/ -run 'Rollback|PlatformRollback' -race -v
go test ./internal/upgrade/ -race          # full package regression (incl. LANDED swap/stage tests)
make test && make lint                     # matrix incl. windows-latest runs rollback_windows_test.go
git status --porcelain                     # == the 5 new rollback*.go files ONLY
```