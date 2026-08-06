name: "P1.M3.T3.S1 — --rollback: restore prior backup via sanity-run + swap; no-op when none (FR-U8)"
description: >
  The FR-U8 rollback primitive (PRD §9.29, prd_snapshot line 621): `Rollback(ctx) (restoredVersion
  string, err error)` in internal/upgrade that restores the most-recent backup (the `.stagecoach-backup`
  / `.exe.old` sibling created by the LANDED Swap, P1.M3.T2.S1) over the current binary, using the
  SAME sanity-run discipline (a backup whose `--version` no longer runs is refused with an
  explanation) — WITHOUT a backup it is a no-op (ErrNoBackup; the command layer prints "no backup —
  nothing to roll back" and exits 0). ONE-STEP undo (FR-U8): the backup is CONSUMED by the restore
  (moved into place); the previous-current is LOST (not re-backed-up). FIVE new stdlib-only files
  (FR-U12), ZERO edit to the LANDED swap*.go / stage.go: (1) rollback.go (shared) — Rollback +
  ErrNoBackup + ErrBackupUnusable; (2) rollback_unix.go (`//go:build !windows`) — platformRollback
  (a SINGLE atomic os.Rename(backup, currentExe) — the running process keeps its old inode); (3)
  rollback_windows.go (`//go:build windows`) — platformRollback (a 3-step rotate routing the prior-
  current through `.old` so the EXISTING CleanupOldBinary reclaims it next launch — no litter); (4)
  rollback_test.go + (5) rollback_windows_test.go. CRITICAL DESIGN: Rollback does NOT reuse Swap —
  Swap's platformSwap backs-up-current FIRST (os.Rename(currentExe→backupPath)), so calling Swap with
  the backup as the "new" binary would OVERWRITE the backup with the current content (destroying the
  restore source) then move it back — a net no-op that destroys the backup. Rollback instead moves the
  backup DIRECTLY into place via its own platformRollback twins (matches the contract's "NO new
  backup-of-current … overwrites current with backup, previous-current is lost" decision). CONSUMES
  (same package, LANDED): resolveCurrentExe + backupPath (swap*.go) + execVersion (stage.go). Rollback
  runs execVersion(ctx, backup) DIRECTLY (NOT stage.go's sanityCheck — that needs a wantTag; rollback
  only needs "the backup runs --version + exits 0", FR-U8) and returns the trimmed output as
  restoredVersion. SCOPE: NO privilege gate (the contract pins 3 cases; privilegeCommand is hardcoded
  `sudo "<exe>" upgrade`, wrong for `--rollback`; a non-writable dir surfaces as a plain rename EACCES).
  NEVER os.Exit. Tests use the REAL compiled cmd/stubcli as the backup binary (matches stage_test.go's
  idiom + the contract's "compiled stubs" hint), driving the unusable case via STAGECOACH_STUBCLI_EXIT=1;
  rollback_test.go is NOT t.Parallel() (mutates resolveCurrentExe). ZERO overlap with the parallel
  swap*.go (different files). The command-layer wiring (`--rollback` flag, runUpgrade dispatch, the
  "no backup" print+exit-0) is P1.M4 — NOT this item.

---

## Goal

**Feature Goal**: Implement the FR-U8 `--rollback` primitive in `internal/upgrade`: restore the most-
recent backup (the `.stagecoach-backup` / `.exe.old` sibling created by Swap) over the current binary,
after a sanity-run (`--version` exits 0) — refusing an unrunnable backup with current unchanged, and
no-op'ing cleanly (ErrNoBackup) when no backup exists. One-step (FR-U8): the backup is consumed, the
previous-current is lost.

**Deliverable** (5 new stdlib-only files in package upgrade):
1. `internal/upgrade/rollback.go` (NEW, shared) — `func Rollback(ctx) (restoredVersion string, err error)`
   + `var ErrNoBackup` + `var ErrBackupUnusable`.
2. `internal/upgrade/rollback_unix.go` (NEW, `//go:build !windows`) — `func platformRollback(currentExe, backup string) error`.
3. `internal/upgrade/rollback_windows.go` (NEW, `//go:build windows`) — `func platformRollback(currentExe, backup string) error`.
4. `internal/upgrade/rollback_test.go` (NEW, `//go:build !windows`, NOT parallel) — the 3 contract cases.
5. `internal/upgrade/rollback_windows_test.go` (NEW, `//go:build windows`) — the `.old` 3-step dance.

**Success Definition**:
- `Rollback(ctx)` with a backup present: sanity-runs the backup (`--version` exits 0), then atomically
  restores it over the current binary, and returns the backup's reported version (trimmed `--version`
  output). The backup FILE is consumed (Unix: moved into place; Windows: routed through `.old`). The
  previous-current is lost (one-step, FR-U8).
- With NO backup: returns `ErrNoBackup` (so the command layer prints "no backup — nothing to roll
  back" and exits 0) and changes nothing on disk.
- With an UNRUNNABLE backup (`--version` fails / exits non-zero): returns an error wrapping
  `ErrBackupUnusable` and leaves BOTH the current binary AND the backup byte-for-byte unchanged
  (sanity-run BEFORE the swap — FR-U8 "refused with an explanation").
- Unix `platformRollback`: a single `os.Rename(backup, currentExe)` atomically replaces the running
  binary (the running process keeps its old inode; next invocation runs the restored version); the
  backup file is consumed; never zero runnable binaries.
- Windows `platformRollback`: the 3-step rotate (backup→aside, running→`.old`, aside→current) leaves
  `currentExe` = backup content and `.old` = prior-current (reclaimed by the EXISTING CleanupOldBinary
  at next launch — no new litter); FR-U11 restore-on-failure reverses prior steps (best-effort).
- Rollback NEVER `os.Exit`s (the command layer maps ErrNoBackup → exit 0). Stdlib-only (FR-U12):
  no `internal/*` import in any new rollback file. go.mod unchanged.
- `go build ./...` + `GOOS={linux,darwin,windows}` all clean; `go vet` clean; `gofmt -l` empty;
  `go test ./internal/upgrade/ -race` green; `make test` (matrix incl. windows-latest) + `make lint` clean.
- `git status --porcelain` == the 5 new rollback*.go files. ZERO production callers after this subtask
  (the consumer is P1.M4.T2 runUpgrade) — expected.

## User Persona (if applicable)

**Target User**: The `stagecoach upgrade` command layer (P1.M4.T2 `runUpgrade`'s `--rollback` branch),
which calls `Rollback(ctx)` and maps the result: success → print "rolled back to <version>"; ErrNoBackup
→ print "no backup — nothing to roll back" + exit 0; other error → print it + exit non-zero. End users
never call Rollback directly.

**Use Case**: `stagecoach upgrade --rollback` after a bad upgrade (the new version misbehaves) →
Rollback restores the immediately-prior binary from the `.stagecoach-backup` / `.exe.old` sibling →
the next `stagecoach` invocation runs the prior version.

**User Journey**: user upgrades → new version is broken → `stagecoach upgrade --rollback` → runUpgrade
calls Rollback → sanity-run of the backup passes → backup atomically swapped into place → user re-runs
stagecoach → prior version runs. (If there's no backup — e.g. first install, or Windows where the
`.old` was already cleaned by a prior launch — Rollback returns ErrNoBackup and the command prints a
friendly no-op + exits 0.)

**Pain Points Addressed**: FR-U8 — the missing one-step "undo a bad upgrade" surface. Without it, a
broken upgrade requires the user to manually locate + reinstall the prior binary. Rollback automates
the restore with the same safety discipline as the upgrade (sanity-run before swap; never zero runnable
binaries).

## Why

- **FR-U8 / §9.29**: rollback is the explicit safety valve for the direct-binary self-swap. A bad
  upgrade (a binary that passes the sanity-run in CI but misbehaves on the user's box, or a regression)
  must be reversible in one command. This item is exactly that primitive.
- **Reuses the upgrade's discipline**: the SAME sanity-run-before-swap + atomic-rename discipline as
  the upgrade (FR-U5 step 6 / FR-U11) — a backup that no longer runs is refused, and the restore is
  atomic (single Unix rename; ordered Windows dance with restore-on-failure). Never zero runnable binaries.
- **One-step semantics (FR-U8)**: only the immediately-prior version is retained. Rollback CONSUMES
  the backup (it becomes the current binary) and does NOT re-back-up the previous-current. This keeps
  the on-disk footprint to one backup file and matches the PRD's "one-step undo" wording. (A two-state
  toggle would be surprising and would accumulate versions.)
- **Why NOT reuse Swap**: Swap's `platformSwap` backs-up-current FIRST (`os.Rename(currentExe,
  backupPath)`). Calling Swap with the backup as the "new" binary would overwrite the backup with the
  current content (destroying the restore source) then move it back — a net no-op that destroys the
  backup. Rollback needs the OPPOSITE ordering (backup→current, no backup-of-current), so it has its own
  `platformRollback` twins. (Detailed in the research findings §1.)
- **Bounded, no-conflict scope**: 5 new files in `internal/upgrade`. The LANDED swap*.go / stage.go are
  CONSUMED (same-package helpers/seams), NOT edited. The command-layer wiring (`--rollback` flag,
  runUpgrade dispatch) is P1.M4 — not here.

## What

**User-visible behavior** (via the P1.M4 command layer; Rollback itself prints nothing):
```
$ stagecoach upgrade --rollback      # backup present + runnable
(rolled back to <prior version>; next invocation runs it)

$ stagecoach upgrade --rollback      # no backup (first install / already-cleaned .old)
no backup — nothing to roll back
(exit 0)

$ stagecoach upgrade --rollback      # backup present but --version fails (corrupt/incompatible)
rollback: backup <path> unusable: <reason>
(exit non-zero; current binary byte-for-byte unchanged)
```

**Technical change**: a shared `Rollback` orchestrator + build-tagged `platformRollback` twins + 2
sentinels, all stdlib-only, consuming the LANDED resolveCurrentExe/backupPath/execVersion primitives.

### Success Criteria
- [ ] `internal/upgrade/rollback.go` exports `func Rollback(ctx context.Context) (restoredVersion string, err error)`,
      `var ErrNoBackup`, and `var ErrBackupUnusable` (all `errors.New("upgrade: …")`, matching the package convention).
- [ ] `Rollback`: checks `ctx.Err()`; resolves the exe via `resolveCurrentExe()` (the swap.go seam);
      `os.Stat(backupPath(currentExe))` — `os.IsNotExist` → `ErrNoBackup`, other stat error → wrapped.
- [ ] `Rollback` sanity-runs the backup via `execVersion(ctx, backup)` (the stage.go seam); a non-nil
      error → `fmt.Errorf("…: %w", ErrBackupUnusable)` (current + backup unchanged); else the trimmed
      output is captured as `restoredVersion`.
- [ ] `Rollback` calls `platformRollback(currentExe, backup)` (the build-tagged twin); success → returns
      `(restoredVersion, nil)`; failure → wrapped error (FR-U11 — current restored by the twin).
- [ ] `rollback_unix.go` (`//go:build !windows`): `platformRollback` = `os.Rename(backup, currentExe)`
      (atomic replace; backup consumed; running inode safe). Error wrapped `fmt.Errorf("restore backup
      into place: %w", err)`.
- [ ] `rollback_windows.go` (`//go:build windows`): `platformRollback` = the 3-step rotate (backup→aside
      via `os.CreateTemp(dir, ".stagecoach-rbk-*")`+remove, running→`.old`, aside→current) with FR-U11
      restore-on-failure (reverse prior steps, best-effort). Final: `currentExe`=backup content, `.old`
      =prior-current (reclaimed by CleanupOldBinary next launch).
- [ ] `rollback_test.go` (`//go:build !windows`, NOT `t.Parallel()`) covers: RestoresBackup (backup =
      compiled cmd/stubcli copy; `STAGECOACH_STUBCLI_OUT="v9.9.9"`; assert exe==stub bytes + backup
      gone + version contains "v9.9.9"), NoBackup (`errors.Is(err, ErrNoBackup)` + exe unchanged),
      BackupUnusable (`STAGECOACH_STUBCLI_EXIT=1` → `errors.Is(err, ErrBackupUnusable)` + exe AND
      backup unchanged). Uses `buildStubCLI` (stage_test.go) + `restoreResolveCurrentExe` (swap_test.go).
- [ ] `rollback_windows_test.go` (`//go:build windows`) covers: PlatformRollback_OldDance (exe="NEW",
      `.old`="OLD" → after: exe=="OLD", `.old`=="NEW", only those 2 files remain) + Rollback_NoBackup mirror.
- [ ] NO `internal/*` import in any rollback*.go (FR-U12; grep guard). imports stdlib only (rollback.go:
      bytes, context, errors, fmt, os; rollback_unix.go: fmt, os; rollback_windows.go: fmt, os, path/filepath).
- [ ] `go build ./...` + `GOOS={linux,darwin,windows}` clean; `go vet ./internal/upgrade/...` clean;
      `gofmt -l` empty on the 5 files.
- [ ] `go test ./internal/upgrade/ -race` green; `make test` (matrix incl. windows-latest) + `make lint` clean.
- [ ] `git status --porcelain` == the 5 new rollback*.go files (scope guard).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim FR-U8 spec, the CRITICAL "Rollback cannot reuse Swap" realization (with the
trace proving why), the verbatim bodies of every consumed LANDED primitive (resolveCurrentExe,
backupPath-per-platform, execVersion, restoreResolveCurrentExe, buildStubCLI, the cmd/stubcli env
contract), the verbatim platformRollback bodies for both platforms (Unix single-rename; Windows 3-step
`.old`-routing dance with FR-U11 restore), the exact test design (real stubcli backup driving all 3
branches via env, NOT t.Parallel), the scope fences (5 new files; zero edit to swap*.go/stage.go), and
9 grep guards.

### Documentation & References

```yaml
# MUST READ — the verbatim FR-U8 spec (the rollback contract).
- docfile: plan/017_397abce9deb1/prd_snapshot.md
  section: "§9.29 FR-U8 (line 621) + the v3.0 self-update risk note (line 2270) + the e2e note (line 2172)"
  why: "FR-U8 verbatim: 'restores the most recent backup (.stagecoach-backup / .exe.old) over the current
        binary, using the same atomic-swap + sanity-run discipline (a backup whose --version no longer
        runs is refused). Without a backup it is a no-op. One-step undo: only the immediately-prior
        version is retained.' This pins the 3 cases (absent→no-op, unrunnable→refused, OK→restore) and
        the one-step (backup consumed) semantics."
  critical: "'a backup whose --version no longer runs is refused' = exit-non-zero-on---version → refuse
             (ErrBackupUnusable), current unchanged. 'no-op reported as such' = ErrNoBackup → command
             layer prints + exits 0. Rollback itself NEVER os.Exit."

# MUST READ — codebase-specific findings for THIS item (the design + the CITICAL Swap-reuse analysis +
#              the verbatim platformRollback bodies + the test design + scope fences).
- docfile: plan/017_397abce9deb1/P1M3T3S1/research/findings.md
  why: "§0 FR-U8 verbatim; §1 WHY Rollback cannot reuse Swap (the trace — Swap backs-up-current first,
        destroying the restore source) + the DECISION (own platformRollback twins); §2 the LANDED
        consumed primitives (resolveCurrentExe, backupPath per-platform, execVersion) with their exact
        signatures + the test seam helpers (restoreResolveCurrentExe, saveExecVersion); §3 why execVersion
        direct (not sanityCheck — no wantTag); §4 the verbatim platformRollback bodies for BOTH platforms
        (Unix single rename; Windows 3-step .old-routing); §5 the no-privilege-gate scope decision; §6
        the 5-file structure (zero overlap with swap*.go); §7 the test design (real cmd/stubcli backup);
        §8 validation commands."

# MUST READ — the LANDED swap*.go (the contracts Rollback CONSUMES). Read to confirm the exact signatures.
- file: internal/upgrade/swap.go
  why: "`var resolveCurrentExe = func() (string, error)` (line 60) — the exe-path seam Rollback calls +
        the test override mechanism. `Swap` (line 91) — DO NOT call it from Rollback (see findings §1:
        its platformSwap backs-up-current first). Confirms Rollback reuses resolveCurrentExe but NOT Swap."
  pattern: "The shared-orchestrator-calls-build-tagged-twins structure. Rollback mirrors it: rollback.go
            (shared) calls platformRollback (defined in the rollback_unix.go/rollback_windows.go twins)."
  gotcha: "Rollback is in the SAME package (upgrade) ⇒ it can call the UNEXPORTED resolveCurrentExe + the
           twins' backupPath directly. Do NOT re-declare resolveCurrentExe or backupPath — reuse them."

# MUST READ — the LANDED backupPath twins (the stable backup-name contract Rollback MUST use verbatim).
- file: internal/upgrade/swap_unix.go
  why: "`func backupPath(exe string) string { return exe + \".stagecoach-backup\" }` (line 18). The comment
        explicitly says it is a SHARED helper 'because P1.M3.T3.S1 (--rollback) restores the backup Swap
        creates and MUST use the SAME name.' Rollback_unix.go's platformRollback receives `backup` from
        Rollback (which derived it via backupPath) — do NOT re-derive the suffix."
- file: internal/upgrade/swap_windows.go
  why: "`func backupPath(exe string) string { return exe + \".old\" }` (line 18) + CleanupOldBinary (the
        .old reclaimer at next launch). The Windows platformRollback ROUTES the prior-current through .old
        so THIS existing CleanupOldBinary reclaims it — read this to confirm .old is the right channel."

# MUST READ — the LANDED execVersion seam + sanityCheck (Rollback uses execVersion directly, NOT sanityCheck).
- file: internal/upgrade/stage.go
  why: "`var execVersion = func(ctx, path) ([]byte, error)` (line 65) runs `path --version`, cmd.Env unset
        ⇒ child inherits os.Environ (incl. t.Setenv — proven by stage_test.go's HappyPath). `sanityCheck`
        (line 73) REQUIRES a wantTag (the target release tag) — Rollback has NO target tag, so it calls
        execVersion DIRECTLY: error ⇒ ErrBackupUnusable, else trimmed output = restoredVersion."
  gotcha: "Do NOT call sanityCheck from Rollback (it would need a wantTag that doesn't exist for a backup).
           Call execVersion(ctx, backup) directly + interpret err yourself."

# MUST READ — the test helpers to REUSE (buildStubCLI + restoreResolveCurrentExe + saveExecVersion).
- file: internal/upgrade/stage_test.go
  why: "`buildStubCLI(t)` (line 63) compiles cmd/stubcli ONCE per process (cached) — use it as the REAL
        backup binary. cmd/stubcli ignores args, prints STAGECOACH_STUBCLI_OUT, exits STAGECOACH_STUBCLI_EXIT
        — so `backup --version` is exactly 'does the backup run + exit 0' (FR-U8). `saveExecVersion()` /
        `restoreExecVersion(saved)` (line 189/194) — ONLY needed if you override execVersion (the real-stub
        test design does NOT need to). The file is NOT t.Parallel() (mutates execVersion) — rollback_test.go
        must follow suit (it mutates resolveCurrentExe)."
- file: internal/upgrade/swap_test.go
  why: "`var restoreResolveCurrentExe = resolveCurrentExe` (line 24) captured at init — tests override
        resolveCurrentExe and restore via `defer func(){ resolveCurrentExe = restoreResolveCurrentExe }()`.
        REUSE this in rollback_test.go (do NOT invent a new save/restore). Confirms the NOT-t.Parallel rule
        for any test that mutates resolveCurrentExe."

# CONTEXT — cmd/stubcli (the fake backup binary) — env-driven, cross-platform compiled.
- file: cmd/stubcli/main.go
  why: "Confirms stubcli ignores argv, prints STAGECOACH_STUBCLI_OUT (+ trailing newline), exits
        STAGECOACH_STUBCLI_EXIT (default 0). So: RestoreBackup test ⇒ STAGECOACH_STUBCLI_OUT='v9.9.9',
        EXIT unset (0); BackupUnusable test ⇒ STAGECOACH_STUBCLI_EXIT='1'. The child inherits these via
        os.Environ (execVersion leaves cmd.Env unset). A compiled binary runs on every OS (the /bin/sh
        stubs cannot be execed on Windows)."

# CONTEXT — the build-tag twin convention to CLONE for rollback_unix.go / rollback_windows.go.
- file: internal/upgrade/swap_unix.go
  why: "Line 1: `//go:build !windows`, BLANK LINE, `package upgrade`. rollback_unix.go uses the SAME header.
        (swap_windows.go is the `//go:build windows` twin.) No legacy `// +build` line (Go 1.22 codebase)."
- file: internal/upgrade/swap_windows.go
  why: "The `//go:build windows` twin. rollback_windows.go uses the SAME header. Confirms a function
        (platformSwap) defined in BOTH twins with the SAME signature — platformRollback follows suit."

# CONTEXT — FR-U12 (walled off, stdlib-only) + the package doc owner.
- docfile: plan/017_397abce9deb1/prd_snapshot.md
  section: "§9.29 FR-U12 (upgrade imports NO internal/*; the wall is one-directional)"
  why: "rollback*.go import ONLY stdlib (context, errors, fmt, os, bytes, path/filepath). NO internal/*
        (grep guard). The package doc lives in releases.go — rollback.go is a file-comment-only file."
```

### Current Codebase tree (relevant slice)

```bash
internal/upgrade/
  releases.go                 # READ-ONLY — owns the package doc
  version.go / download.go / resolve.go / detect.go / delegate.go / stage.go  # READ-ONLY — LANDED siblings
  stage.go                    # READ-ONLY — execVersion seam (CONSUMED) + sanityCheck (NOT used by rollback)
  swap.go                     # READ-ONLY (LANDED) — resolveCurrentExe seam + Swap (NOT called) + isWritable
  swap_unix.go                # READ-ONLY (LANDED) — backupPath (.stagecoach-backup) + platformSwap + CleanupOldBinary(no-op)
  swap_windows.go             # READ-ONLY (LANDED) — backupPath (.old) + platformSwap + CleanupOldBinary
  swap_test.go                # READ-ONLY — restoreResolveCurrentExe helper (REUSED)
  stage_test.go               # READ-ONLY — buildStubCLI + saveExecVersion/restoreExecVersion helpers (REUSED)
  rollback.go                 # NEW (shared) — Rollback + ErrNoBackup + ErrBackupUnusable
  rollback_unix.go            # NEW (//go:build !windows) — platformRollback (single rename)
  rollback_windows.go         # NEW (//go:build windows)  — platformRollback (3-step .old-routing dance)
  rollback_test.go            # NEW (//go:build !windows, NOT parallel) — the 3 contract cases (real stubcli backup)
  rollback_windows_test.go    # NEW (//go:build windows)  — the .old dance + no-backup
cmd/stubcli/main.go           # READ-ONLY — the env-driven fake binary (the backup in tests)
cmd/stagecoach/main.go        # READ-ONLY — the --rollback command wiring is P1.M4 (NOT this item)
go.mod                        # READ-ONLY — module github.com/dabstractor/stagecoach; UNCHANGED (all new imports stdlib)
.github/workflows/ci.yml      # matrix os: [ubuntu-latest, macos-latest, windows-latest] — windows-latest runs rollback_windows_test.go natively
```

### Desired Codebase tree with files to be added

```bash
internal/upgrade/
  rollback.go                 # NEW (shared) — Rollback(ctx) + ErrNoBackup + ErrBackupUnusable
  rollback_unix.go            # NEW (//go:build !windows) — platformRollback (single atomic os.Rename)
  rollback_windows.go         # NEW (//go:build windows)  — platformRollback (3-step rotate via .old)
  rollback_test.go            # NEW (//go:build !windows) — RestoresBackup / NoBackup / BackupUnusable
  rollback_windows_test.go    # NEW (//go:build windows)  — PlatformRollback_OldDance / Rollback_NoBackup
# NOTHING ELSE. No edit to swap*.go, stage.go, version.go, main.go, exitcode.go, go.mod, or any PRD/task file.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (Rollback MUST NOT call Swap): Swap→platformSwap backs-up-current FIRST
// (os.Rename(currentExe, backupPath(currentExe))). Calling Swap(ctx, backupPath) would OVERWRITE the
// backup (the restore source) with the current content, then move it back — a net no-op that DESTROYS
// the backup. Rollback has its OWN platformRollback twins that move the backup DIRECTLY into place
// (consuming it). This is the single most important design constraint — reusing Swap silently breaks
// rollback. (See research findings §1 for the full trace.)

// CRITICAL (one-step semantics — FR-U8): the backup is CONSUMED by the restore. Unix platformRollback
// does os.Rename(backup, currentExe) — the backup FILE ceases to exist (its content is now at currentExe).
// Do NOT re-back-up the previous-current (no tertiary backup). After rollback there is NO backup to
// roll back to again. (A "copy backup to temp then Swap" trick would preserve the previous-current AS
// the new backup, making rollback a 2-state toggle — explicitly rejected by FR-U8's one-step wording.)

// CRITICAL (sanity-run BEFORE swap — FR-U8 "refused with an explanation"): execVersion(ctx, backup)
// MUST run BEFORE platformRollback. A backup whose --version fails/non-zero-exits ⇒ ErrBackupUnusable
// with BOTH current and backup unchanged. Use execVersion DIRECTLY (not stage.go's sanityCheck — that
// needs a wantTag Rollback doesn't have). Capture the trimmed output as restoredVersion on success.

// CRITICAL (the no-op case is ErrNoBackup, NOT an error the command exits non-zero on): os.Stat(backup)
// with os.IsNotExist ⇒ return ErrNoBackup. The COMMAND layer (P1.M4.T2) maps ErrNoBackup → "no backup —
// nothing to roll back" + exit 0. Rollback itself NEVER os.Exit and does not print.

// CRITICAL (Rollback NEVER os.Exit): like Swap, Rollback RETURNS errors. The command layer maps them
// (ErrNoBackup→0, ErrBackupUnusable/swap-err→non-zero). Do not add an exitcode mapping for rollback.

// GOTCHA (reuse the LANDED primitives — do NOT re-declare): resolveCurrentExe (swap.go:60), backupPath
// (swap_unix.go:18 / swap_windows.go:18), execVersion (stage.go:65) are all UNEXPORTED package-level
// vars/funcs. Rollback is in package upgrade ⇒ it can call them directly. Re-declaring any would be a
// compile error (duplicate declaration) or a silent desync (a second backupPath with a different suffix).
// backupPath's comment EXPLICITLY says it is shared "because P1.M3.T3.S1 (--rollback) ... MUST use the
// SAME name" — honor that contract.

// GOTCHA (Unix os.Rename over a running binary is safe): os.Rename(backup, currentExe) atomically
// replaces the directory entry; the running process's text mapping holds the OLD inode (the kernel
// reclaims it at last-fd-close / process exit). So THIS process keeps running the now-overwritten
// binary while the NEXT invocation runs the restored version. backup and currentExe are siblings in
// the install dir ⇒ same filesystem ⇒ rename succeeds (rename(2) does not cross filesystems).

// GOTCHA (Windows: the running .exe is LOCKED — can rename, not overwrite): you CANNOT os.Rename(backup,
// currentExe) directly on Windows (currentExe is the locked running image). The 3-step rotate is
// mandatory: backup→aside, running→.old (rename of a running image IS allowed), aside→current. Route
// the prior-current through .old so the EXISTING CleanupOldBinary (swap_windows.go) reclaims it next
// launch — do NOT invent a new cleanup suffix (it would leak). The `aside` scratch name (os.CreateTemp
// in the install dir) is consumed by step 3 (no litter).

// GOTCHA (rollback_test.go MUST NOT t.Parallel()): it mutates the package-level resolveCurrentExe seam
// (swap_test.go:5 and stage_test.go:2 establish this rule for seam-mutating tests). Two concurrent
// rollback tests would race on resolveCurrentExe and trip the race detector. (Other upgrade tests that
// don't touch the seam may stay parallel.)

// GOTCHA (no privilege gate in Rollback — deliberate scope): the contract pins 3 cases (absent /
// unrunnable / restored). privilegeCommand (swap twins) is hardcoded `sudo "<exe>" upgrade` (the upgrade
// re-run form) — wrong for `--rollback`. So Rollback does NOT probe isWritable; a non-writable install
// dir surfaces as a plain os.Rename EACCES error (propagated). The command layer (P1.M4) may add sudo
// guidance later. (Adding a privilege gate here would be scope creep + print the wrong command.)

// GOTCHA (stdlib-only, FR-U12): rollback.go imports bytes, context, errors, fmt, os. rollback_unix.go
// imports fmt, os. rollback_windows.go imports fmt, os, path/filepath. NO internal/* (grep guard).
```

## Implementation Blueprint

### Data models and structure

```go
// Two sentinels (errors.New("upgrade: …"), matching the package convention). No new types/structs.
var (
	ErrNoBackup       = errors.New("upgrade: no prior backup to roll back to (FR-U8)")
	ErrBackupUnusable = errors.New("upgrade: backup binary no longer runs (FR-U8)")
)
// No structs. Rollback/platformRollback are plain functions. resolveCurrentExe/backupPath/execVersion
// are the CONSUMED LANDED package-level primitives (do NOT re-declare).
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/upgrade/rollback.go (SHARED — no build tag) — orchestrator + 2 sentinels
  - FILE COMMENT (Mode A): explain this is FR-U8 `--rollback` — restores the most-recent backup
    (.stagecoach-backup / .exe.old created by Swap) over the current binary after a sanity-run, no-op
    (ErrNoBackup) when none, one-step (backup consumed, previous-current lost). State WHY it does NOT
    reuse Swap (platformSwap backs-up-current first ⇒ would destroy the restore source). File comment
    only — releases.go owns the package doc.
  - IMPORTS (all stdlib): bytes, context, errors, fmt, os.
  - SENTINELS:
      var (
          ErrNoBackup       = errors.New("upgrade: no prior backup to roll back to (FR-U8)")
          ErrBackupUnusable = errors.New("upgrade: backup binary no longer runs (FR-U8)")
      )
  - `func Rollback(ctx context.Context) (restoredVersion string, err error)`:
      if err := ctx.Err(); err != nil { return "", err }
      currentExe, err := resolveCurrentExe()                      // swap.go seam (CONSUMED — do NOT redeclare)
      if err != nil { return "", fmt.Errorf("upgrade: resolve current exe: %w", err) }
      backup := backupPath(currentExe)                            // swap-twin helper (CONSUMED) — .stagecoach-backup / .old
      if _, err := os.Stat(backup); err != nil {
          if os.IsNotExist(err) { return "", ErrNoBackup }        // FR-U8 no-op → command layer prints + exits 0
          return "", fmt.Errorf("upgrade: stat backup %s: %w", backup, err)
      }
      out, runErr := execVersion(ctx, backup)                     // stage.go seam (CONSUMED) — runs `backup --version`
      if runErr != nil {
          return "", fmt.Errorf("rollback: backup %s unusable: %w", backup, ErrBackupUnusable)  // FR-U8 refuse; current+backup unchanged
      }
      if err := platformRollback(currentExe, backup); err != nil {  // build-tag twin (Task 2/3)
          return "", fmt.Errorf("upgrade: rollback: %w", err)       // FR-U11 — twin restored current on failure
      }
      return string(bytes.TrimSpace(out)), nil                    // the restored binary's reported version
  - NAMING: Rollback, ErrNoBackup, ErrBackupUnusable (all matching the package's errors.New("upgrade: …") convention).
  - GOTCHA: platformRollback is NOT defined here — it lives in the build-tag twins (Task 2/3). Rollback
    CALLS it (the Go compiler resolves it per-build via the twin).
  - GOTCHA: do NOT call Swap, sanityCheck, isWritable, or privilegeCommand (see Known Gotchas).

Task 2: CREATE internal/upgrade/rollback_unix.go (`//go:build !windows`) — single atomic rename
  - HEADER: `//go:build !windows`, BLANK LINE, `package upgrade`. (Clone swap_unix.go:1.)
  - IMPORTS: fmt, os (all stdlib).
  - GODOC (Mode A) on platformRollback: explain Unix atomicity (os.Rename over a running inode is safe —
    the running process keeps the old inode open; the path now resolves to the backup; next invocation
    runs the restored version), one-step (FR-U8 — the backup FILE is consumed by the rename; previous-
    current lost; no rollback-of-rollback), and that this is DELIBERATELY NOT platformSwap (which backs-
    up-current first and would destroy this backup).
  - BODY:
      func platformRollback(currentExe, backup string) error {
          if err := os.Rename(backup, currentExe); err != nil { // atomic replace; backup consumed; running inode safe
              return fmt.Errorf("restore backup into place: %w", err)
          }
          return nil
      }
  - GOTCHA: backup + currentExe are siblings in the install dir ⇒ same filesystem ⇒ rename(2) succeeds.
    A non-writable dir surfaces as a rename EACCES (propagated) — Rollback does NOT probe isWritable (scope).

Task 3: CREATE internal/upgrade/rollback_windows.go (`//go:build windows`) — 3-step .old-routing dance
  - HEADER: `//go:build windows`, BLANK LINE, `package upgrade`. (Clone swap_windows.go:1.)
  - IMPORTS: fmt, os, path/filepath (all stdlib).
  - GODOC (Mode A) on platformRollback: explain the Windows constraint (running .exe LOCKED — can rename,
    not overwrite), the 3-step rotate (backup→aside, running→.old, aside→current), WHY prior-current is
    routed through .old (so the EXISTING CleanupOldBinary reclaims it next launch — no new litter), FR-U11
    restore-on-failure (reverse prior steps, best-effort), and that this is DELIBERATELY NOT platformSwap.
  - BODY (verbatim):
      func platformRollback(currentExe, backup string) error { // backup == currentExe + ".old" (backupPath)
          dir := filepath.Dir(currentExe)
          tf, err := os.CreateTemp(dir, ".stagecoach-rbk-*") // a fresh scratch name in the install dir
          if err != nil { return fmt.Errorf("create rollback aside: %w", err) }
          aside := tf.Name()
          _ = tf.Close()
          _ = os.Remove(aside) // free the name so os.Rename can target it
          // 1. Stash the backup content at `aside`; free the .old name.
          if err := os.Rename(backup, aside); err != nil {
              return fmt.Errorf("move backup aside: %w", err)
          }
          // 2. Move the locked running binary to .old (rename of a running image IS allowed; frees current).
          if err := os.Rename(currentExe, backup); err != nil { // backup == .old, freed in step 1
              _ = os.Rename(aside, backup) // restore the backup to .old (FR-U11)
              return fmt.Errorf("move current aside to .old: %w (backup restored)", err)
          }
          // 3. Move the backup content into place.
          if err := os.Rename(aside, currentExe); err != nil {
              _ = os.Rename(backup, currentExe) // restore the running binary (FR-U11)
              _ = os.Rename(aside, backup)      // restore the backup to .old
              return fmt.Errorf("restore backup into place: %w (current restored)", err)
          }
          return nil // currentExe = backup content; .old = prior-current (CleanupOldBinary reclaims next launch)
      }
  - GOTCHA: `backup` and `backupPath(currentExe)` are the SAME string (Rollback derived backup via
    backupPath). Step 2's `os.Rename(currentExe, backup)` targets `.old` (freed in step 1) — correct.

Task 4: CREATE internal/upgrade/rollback_test.go (`//go:build !windows`, NOT parallel) — 3 contract cases
  - HEADER: `//go:build !windows`, BLANK LINE, `package upgrade`. IMPORTS: bytes, context, errors, os,
    path/filepath, testing. (NOT t.Parallel() — mutates resolveCurrentExe.)
  - FILE COMMENT: state this is NOT parallel because it mutates the package-level resolveCurrentExe seam
    (clone of swap_test.go:5 / stage_test.go:2's rule), and that it uses the REAL compiled cmd/stubcli
    (buildStubCLI) as the backup binary.
  - helper (if not already present in the package): `writeFile(t, path, content)` — os.WriteFile(path,
    []byte(content), 0o644). (swap_test.go may already have a writeExe; reuse if so.)
  - TestRollback_RestoresBackup:
      stub := buildStubCLI(t)                       // stage_test.go helper — compiled cmd/stubcli
      t.Setenv("STAGECOACH_STUBCLI_OUT", "v9.9.9\n") // the backup's --version output
      tempDir := t.TempDir()
      exe := filepath.Join(tempDir, "stagecoach")
      require writeFile(t, exe, "CURRENT")          // the (unwanted) current binary
      backup := exe + ".stagecoach-backup"          // == backupPath(exe)
      stubBytes := readFile(t, stub); require writeFile(t, backup, stubBytes) // backup = a runnable stub copy
      resolveCurrentExe = func() (string, error) { return exe, nil }; defer func() { resolveCurrentExe = restoreResolveCurrentExe }()
      version, err := Rollback(context.Background())
      require err == nil
      assert strings.Contains(version, "v9.9.9")    // the restored version (trimmed --version output)
      assert readFile(t, exe) == stubBytes          // current now == backup content (restored)
      _, statErr := os.Stat(backup); assert os.IsNotExist(statErr) // backup CONSUMED (FR-U8 one-step)
  - TestRollback_NoBackup:
      tempDir := t.TempDir(); exe := filepath.Join(tempDir, "stagecoach"); writeFile(t, exe, "CURRENT")
      resolveCurrentExe = func() (string, error) { return exe, nil }; defer restore
      version, err := Rollback(context.Background())
      assert errors.Is(err, ErrNoBackup); assert version == ""
      assert readFile(t, exe) == "CURRENT"          // unchanged
  - TestRollback_BackupUnusable:
      stub := buildStubCLI(t)
      t.Setenv("STAGECOACH_STUBCLI_EXIT", "1")      // stub exits non-zero → execVersion errors
      tempDir := t.TempDir(); exe := filepath.Join(tempDir, "stagecoach"); writeFile(t, exe, "CURRENT")
      backup := exe + ".stagecoach-backup"; stubBytes := readFile(t, stub); writeFile(t, backup, stubBytes)
      resolveCurrentExe = func() (string, error) { return exe, nil }; defer restore
      version, err := Rollback(context.Background())
      assert errors.Is(err, ErrBackupUnusable); assert version == ""
      assert readFile(t, exe) == "CURRENT"          // unchanged (sanity-run BEFORE swap)
      assert readFile(t, backup) == stubBytes       // backup ALSO unchanged
  - FOLLOW pattern: swap_test.go's resolveCurrentExe override + restoreResolveCurrentExe restore; stage_test.go's
    buildStubCLI + the STAGECOACH_STUBCLI_* env contract.
  - GOTCHA: the DEFAULT execVersion is used (no saveExecVersion) — the stub child inherits os.Environ
    incl. the t.Setenv values (proven by stage_test.go's HappyPath). Only resolveCurrentExe is overridden.

Task 5: CREATE internal/upgrade/rollback_windows_test.go (`//go:build windows`) — .old dance + no-backup
  - HEADER: `//go:build windows`, BLANK LINE, `package upgrade`. IMPORTS: context, errors, os, path/filepath, testing.
  - TestPlatformRollback_OldDance:
      tempDir := t.TempDir()
      exe := filepath.Join(tempDir, "stagecoach.exe"); writeFile(t, exe, "NEW")
      old := exe + ".old"; writeFile(t, old, "OLD") // the backup (backupPath on Windows)
      require platformRollback(exe, old) == nil
      assert readFile(t, exe) == "OLD"              // restored
      assert readFile(t, exe+".old") == "NEW"       // prior-current routed to .old (cleaned next launch)
      entries, _ := os.ReadDir(tempDir); assert len(entries) == 2 // only exe + .old — no aside litter
  - TestRollback_NoBackup (Windows mirror of the Unix no-backup test):
      tempDir; exe := …\stagecoach.exe; writeFile(exe,"CURRENT"); resolveCurrentExe→exe (defer restore)
      _, err := Rollback(ctx); assert errors.Is(err, ErrNoBackup); assert read(exe)=="CURRENT"
  - NOTE: runs natively on windows-latest CI (.github/workflows/ci.yml matrix). The running-image lock is
    a kernel property verified end-to-end by P1.M4.T3's e2e; here regular files prove the rename sequence.

Task 6: VERIFY — build (native+cross), vet, format, tests (Unix + Windows via CI), lint, grep guards
  - go build ./... ; GOOS=linux go build ./... ; GOOS=darwin go build ./... ; GOOS=windows go build ./...
  - go vet ./internal/upgrade/...
  - gofmt -l internal/upgrade/rollback.go internal/upgrade/rollback_unix.go internal/upgrade/rollback_windows.go \
         internal/upgrade/rollback_test.go internal/upgrade/rollback_windows_test.go   # empty
  - go test ./internal/upgrade/ -run 'Rollback|PlatformRollback' -race -v
  - GOOS=windows go vet ./internal/upgrade/...   # the _windows file vets clean
  - go test ./internal/upgrade/ -race            # full package regression (incl. LANDED swap/stage tests)
  - make test && make lint                        # matrix incl. windows-latest runs rollback_windows_test.go
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the shared orchestrator calls build-tagged twins — clone of swap.go's split):
//   rollback.go (no tag):       Rollback → resolveCurrentExe → os.Stat(backup) → execVersion(sanity) → platformRollback
//   rollback_unix.go (!windows): platformRollback (single atomic os.Rename)
//   rollback_windows.go(Windows): platformRollback (3-step .old-routing dance)
func Rollback(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	currentExe, err := resolveCurrentExe()
	if err != nil {
		return "", fmt.Errorf("upgrade: resolve current exe: %w", err)
	}
	backup := backupPath(currentExe)
	if _, err := os.Stat(backup); err != nil {
		if os.IsNotExist(err) {
			return "", ErrNoBackup // no-op → command layer prints + exits 0
		}
		return "", fmt.Errorf("upgrade: stat backup %s: %w", backup, err)
	}
	out, runErr := execVersion(ctx, backup)
	if runErr != nil {
		return "", fmt.Errorf("rollback: backup %s unusable: %w", backup, ErrBackupUnusable)
	}
	if err := platformRollback(currentExe, backup); err != nil {
		return "", fmt.Errorf("upgrade: rollback: %w", err)
	}
	return string(bytes.TrimSpace(out)), nil
}

// PATTERN (Unix platformRollback — single atomic rename, backup consumed):
func platformRollback(currentExe, backup string) error {
	if err := os.Rename(backup, currentExe); err != nil {
		return fmt.Errorf("restore backup into place: %w", err)
	}
	return nil
}

// PATTERN (Windows platformRollback — 3-step rotate routing prior-current through .old):
func platformRollback(currentExe, backup string) error {
	dir := filepath.Dir(currentExe)
	tf, err := os.CreateTemp(dir, ".stagecoach-rbk-*")
	if err != nil {
		return fmt.Errorf("create rollback aside: %w", err)
	}
	aside := tf.Name()
	_ = tf.Close()
	_ = os.Remove(aside)
	if err := os.Rename(backup, aside); err != nil {
		return fmt.Errorf("move backup aside: %w", err)
	}
	if err := os.Rename(currentExe, backup); err != nil {
		_ = os.Rename(aside, backup)
		return fmt.Errorf("move current aside to .old: %w (backup restored)", err)
	}
	if err := os.Rename(aside, currentExe); err != nil {
		_ = os.Rename(backup, currentExe)
		_ = os.Rename(aside, backup)
		return fmt.Errorf("restore backup into place: %w (current restored)", err)
	}
	return nil
}
```

### Integration Points

```yaml
PRODUCTION (internal/upgrade/ — 3 new files):
  - rollback.go:         func Rollback(ctx) (restoredVersion string, err error) + ErrNoBackup + ErrBackupUnusable
  - rollback_unix.go:    func platformRollback(currentExe, backup string) error
  - rollback_windows.go: func platformRollback(currentExe, backup string) error

IMPORTS (all stdlib — FR-U12):
  - rollback.go: bytes, context, errors, fmt, os
  - rollback_unix.go: fmt, os
  - rollback_windows.go: fmt, os, path/filepath
  - NO new go.mod requires. NO internal/* import (grep guard).

CONSUMES (LANDED — same package; treat as contracts, do NOT edit/re-declare):
  - resolveCurrentExe (swap.go:60) — the exe-path seam.
  - backupPath (swap_unix.go:18 / swap_windows.go:18) — the stable backup-name helper (built FOR this item).
  - execVersion (stage.go:65) — the sanity-run seam (called DIRECTLY, not via sanityCheck).

CONSUMERS (land AFTER this item; do NOT implement here):
  - P1.M4.T2 runUpgrade: the `--rollback` branch calls Rollback(ctx); success → print "rolled back to
    <restoredVersion>"; errors.Is(err, ErrNoBackup) → print "no backup — nothing to roll back" + exit 0;
    other error → print + exit non-zero. Rollback NEVER os.Exit / never prints — the command layer does.

NO database / migration / routes / new types beyond the 2 sentinels / new flag (the --rollback flag is
P1.M4.T1) / config change / exitcode mapping (ErrNoBackup→0 is the command layer's job) / docs (P3.M1).

SCOPE FENCES:
  - Touches ONLY internal/upgrade/{rollback.go,rollback_unix.go,rollback_windows.go,rollback_test.go,
    rollback_windows_test.go} (new). 5 files.
  - Does NOT edit swap.go, swap_unix.go, swap_windows.go, swap_test.go, stage.go, stage_test.go, version.go,
    detect.go, resolve.go, delegate.go, releases.go, download.go, cmd/stagecoach/main.go, exitcode.go,
    go.mod, or any PRD/task file.
  - Adds NO flag, NO config field, NO third-party dependency, NO exitcode constant, NO new exported TYPE
    (the 2 sentinels are vars; platformRollback is unexported; Rollback is the lone new export).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native + cross-build (rollback_unix.go's os.Rename compiles on linux/darwin; rollback_windows.go's
# 3-step dance + os.CreateTemp compile on windows; the shared rollback.go compiles everywhere).
go build ./...
GOOS=linux   go build ./...
GOOS=darwin  go build ./...
GOOS=windows go build ./...
# Expected: all clean. GOOS=windows failure ⇒ rollback_windows.go references a symbol undefined there, or
#           defines platformRollback inconsistently with rollback_unix.go's signature.

# Vet (the build-tag twins vet on their respective OSes; run all-platform vet + a windows vet).
go vet ./internal/upgrade/...
GOOS=windows go vet ./internal/upgrade/...
# Expected: clean.

# Format.
gofmt -l internal/upgrade/rollback.go internal/upgrade/rollback_unix.go internal/upgrade/rollback_windows.go \
       internal/upgrade/rollback_test.go internal/upgrade/rollback_windows_test.go
# Expected: empty. If listed: gofmt -w <those files>.

# Lint.
make lint   # errcheck/gosimple/govet/ineffassign/staticcheck/unused
# Expected: zero errors. (Rollback/platformRollback/ErrNoBackup/ErrBackupUnusable all used; the unused-func
#           linter won't fire on the build-tag twins — each compiles on its OS.)

# Scope guard: ONLY the 5 new files changed.
git status --porcelain
# Expected: the 5 new internal/upgrade/rollback*.go files. ZERO changes to swap*.go/stage.go/main.go/etc.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Unix rollback tests (run on ubuntu-latest/macos-latest; the //go:build !windows file).
go test ./internal/upgrade/ -run 'Rollback|PlatformRollback' -race -v
# Expected: ALL PASS —
#   TestRollback_RestoresBackup: err nil, version contains "v9.9.9", exe==stub bytes, backup GONE (consumed).
#   TestRollback_NoBackup: errors.Is(err, ErrNoBackup), exe unchanged ("CURRENT").
#   TestRollback_BackupUnusable: errors.Is(err, ErrBackupUnusable), exe AND backup unchanged.

# Full upgrade-package regression (the new tests + ALL LANDED siblings: releases/download/detect/delegate/
# resolve/stage/swap/version).
go test ./internal/upgrade/ -race
# Expected: green. (Rollback has ZERO production callers — expected; the consumer is P1.M4.T2.)

# Full race suite (matrix incl. windows-latest runs rollback_windows_test.go natively).
make test
# Expected: green on ubuntu-latest, macos-latest, AND windows-latest (the .old 3-step dance + no-backup).
```

### Level 3: Integration Testing (System Validation)

```bash
# This item is the rollback PRIMITIVE; its end-to-end exercise (stagecoach upgrade --rollback against a
# REAL install with a REAL prior binary, asserting os.Executable() reports the prior version + the backup
# is gone) is P1.M4.T3 (the e2e self-update harness). The unit tests (Task 4/5) are the within-scope proof.
# Smoke (optional): build + a manual temp-dir rollback on Unix.
make build
# (In a throwaway: copy bin/stagecoach to a temp "exe", plant a temp "backup" at exe.stagecoach-backup,
#  call Rollback via a tiny main, assert the exe now equals the backup content. The unit tests are the
#  real proof; this is a sanity check.)

# Windows .old routing smoke (windows-latest): after rollback, relaunch stagecoach; assert the .old
# (prior-current) is reclaimed by CleanupOldBinary. (Covered by P1.M4.T3 e2e; the unit
# TestPlatformRollback_OldDance is the within-scope proof.)
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: Rollback exists with the contract signature + the 3-case flow.
grep -n 'func Rollback(ctx context.Context) (restoredVersion string, err error)' internal/upgrade/rollback.go  # 1 hit
grep -n 'resolveCurrentExe()' internal/upgrade/rollback.go     # 1 hit (CONSUMED — not redeclared)
grep -n 'backupPath(currentExe)' internal/upgrade/rollback.go  # 1 hit (CONSUMED — not redeclared)
grep -n 'execVersion(ctx, backup)' internal/upgrade/rollback.go # 1 hit (CONSUMED — not redeclared)
grep -n 'platformRollback(currentExe, backup)' internal/upgrade/rollback.go # 1 hit

# Guard 2: the 2 sentinels exist with the package convention.
grep -n 'ErrNoBackup' internal/upgrade/rollback.go        # ≥2 hits (decl + return)
grep -n 'ErrBackupUnusable' internal/upgrade/rollback.go  # ≥2 hits (decl + return)

# Guard 3: Rollback does NOT call Swap (the critical design constraint).
grep -n 'Swap(' internal/upgrade/rollback.go   # ZERO hits (Rollback uses platformRollback, NOT Swap)

# Guard 4: FR-U8 no-op — os.IsNotExist → ErrNoBackup (and Rollback returns it, does not os.Exit).
grep -n 'os.IsNotExist(err)' internal/upgrade/rollback.go   # 1 hit
grep -n 'return "", ErrNoBackup' internal/upgrade/rollback.go # 1 hit
grep -n 'os.Exit' internal/upgrade/rollback.go internal/upgrade/rollback_unix.go internal/upgrade/rollback_windows.go # ZERO hits

# Guard 5: FR-U8 refuse — unrunnable backup → ErrBackupUnusable BEFORE the swap (current+backup unchanged).
grep -n 'ErrBackupUnusable' internal/upgrade/rollback.go   # the sanity-run error wraps it BEFORE platformRollback

# Guard 6: platformRollback defined in BOTH build-tag twins with the SAME signature.
grep -n 'func platformRollback(currentExe, backup string) error' internal/upgrade/rollback_unix.go internal/upgrade/rollback_windows.go
# Expect: 1 hit in EACH file (the build-tag twin).

# Guard 7: Unix platformRollback = a single os.Rename (atomic; backup consumed).
grep -n 'os.Rename(backup, currentExe)' internal/upgrade/rollback_unix.go  # 1 hit

# Guard 8: Windows platformRollback routes prior-current through .old (CleanupOldBinary reclaims it).
grep -n 'os.Rename(currentExe, backup)' internal/upgrade/rollback_windows.go # 1 hit (running→.old, step 2)
grep -n 'os.CreateTemp(dir, ".stagecoach-rbk-")' internal/upgrade/rollback_windows.go  # 1 hit (the aside)

# Guard 9: stdlib-only (FR-U12) — no internal/* import in any rollback file.
grep -nE 'github.com/dabstractor/stagecoach/internal/' internal/upgrade/rollback.go internal/upgrade/rollback_unix.go internal/upgrade/rollback_windows.go
# Expect: ZERO hits.

# Guard 10: scope — only the 5 new rollback files.
git status --porcelain
# Expect: the 5 internal/upgrade/rollback*.go files ONLY.
git diff --name-only | grep -E 'swap\.go|swap_unix\.go|swap_windows\.go|stage\.go|main\.go|exitcode\.go|go\.mod' && echo "FAIL: out-of-scope file edited" || echo "OK: scope clean"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `GOOS=linux` + `GOOS=darwin` + `GOOS=windows` all clean
- [ ] `go vet ./internal/upgrade/...` clean (incl. `GOOS=windows go vet`)
- [ ] `gofmt -l` empty on the 5 new files
- [ ] `make lint` zero errors
- [ ] `go test ./internal/upgrade/ -run 'Rollback|PlatformRollback' -race -v` green (3 Unix + 2 Windows)
- [ ] `go test ./internal/upgrade/ -race` green (full package regression incl. LANDED swap/stage)
- [ ] `make test` (matrix incl. windows-latest) green

### Feature Validation
- [ ] Backup present + runnable → restored; exe == backup content; backup FILE consumed (gone) (Task 4 RestoreBackup)
- [ ] No backup → ErrNoBackup; exe unchanged (Task 4 NoBackup)
- [ ] Backup unrunnable → ErrBackupUnusable; exe AND backup unchanged (sanity-run before swap) (Task 4 BackupUnusable)
- [ ] restoredVersion = the backup's trimmed `--version` output
- [ ] Unix: single atomic os.Rename (grep guard 7); Windows: 3-step .old-routing (grep guard 8)
- [ ] Rollback never os.Exit; returns ErrNoBackup/ErrBackupUnusable/wrapped-swap-err (grep guard 4)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY the 5 new internal/upgrade/rollback*.go files (grep guard 10)
- [ ] NO edit to swap*.go, stage.go, stage_test.go, swap_test.go, version.go, main.go, exitcode.go, go.mod,
      or any PRD/task file
- [ ] NO new exported TYPE (2 sentinels are vars; platformRollback unexported; Rollback lone new export)
- [ ] NO new flag (--rollback flag is P1.M4.T1), NO config field, NO exitcode constant, NO third-party dep
- [ ] NO `internal/*` import (FR-U12; grep guard 9)

### Code Quality & Docs
- [ ] [Mode A] Godoc on Rollback: FR-U8 one-step semantics, the no-op/refuse cases, WHY it does NOT reuse Swap
- [ ] [Mode A] Godoc on each platformRollback twin: the platform constraint + the atomicity/3-step mechanism
- [ ] rollback_test.go reuses buildStubCLI + restoreResolveCurrentExe (NOT a new seam helper); NOT t.Parallel()
- [ ] The Windows twin's prior-current routes through .old (reuses the EXISTING CleanupOldBinary — no new litter)

---

## Anti-Patterns to Avoid

- ❌ Don't call `Swap` from `Rollback`. Swap's `platformSwap` backs-up-current FIRST (`os.Rename(currentExe,
  backupPath)`), so `Swap(ctx, backup)` would overwrite the backup with the current content then move it
  back — a net no-op that DESTROYS the restore source. Rollback has its OWN `platformRollback` twins that
  move the backup directly into place. (This is the single most important constraint — see findings §1.)
- ❌ Don't re-back-up the previous-current. FR-U8 is ONE-STEP: the backup is consumed, the previous-current
  is LOST. A "copy backup → temp, then Swap(temp)" trick would preserve the previous-current as the new
  backup, making rollback a 2-state toggle — explicitly rejected by FR-U8. Move the backup into place; done.
- ❌ Don't re-declare `resolveCurrentExe`, `backupPath`, or `execVersion`. They are LANDED package-level
  primitives (swap.go / swap twins / stage.go). Rollback is in package `upgrade` ⇒ call them directly.
  Re-declaring any is a compile error (duplicate) or a silent desync (a second backupPath with a wrong suffix).
  backupPath's comment EXPLICITLY says it is shared FOR this item — honor it.
- ❌ Don't use `sanityCheck` for the backup's sanity-run. It requires a `wantTag` (the target release tag);
  a backup has no target — its version is whatever it is. Call `execVersion(ctx, backup)` DIRECTLY: error ⇒
  ErrBackupUnusable; else trimmed output = restoredVersion. (FR-U8: "whose --version no longer runs is refused.")
- ❌ Don't put the sanity-run AFTER the swap. FR-U8 refuses an unrunnable backup with current UNCHANGED —
  that requires sanity-run BEFORE platformRollback. (The BackupUnusable test asserts BOTH exe and backup
  are byte-identical after the refusal.)
- ❌ Don't make the no-backup case an error the command exits non-zero on. `os.IsNotExist` ⇒ return
  `ErrNoBackup`; the COMMAND layer (P1.M4.T2) maps it to "no backup — nothing to roll back" + exit 0.
  Rollback itself never os.Exit / never prints.
- ❌ Don't add a privilege gate (`isWritable` / `NeedsPrivilegesError`). The contract pins 3 cases; and
  `privilegeCommand` is hardcoded `sudo "<exe>" upgrade` (wrong for `--rollback`). A non-writable dir
  surfaces as a plain rename EACCES (propagated). Sudo guidance is the command layer's (P1.M4) concern.
- ❌ Don't `os.Exit` anywhere. Like Swap/stage, Rollback RETURNS errors; the command layer maps them.
- ❌ Don't make `rollback_test.go` call `t.Parallel()`. It mutates the package-level `resolveCurrentExe`
  seam (swap_test.go:5 / stage_test.go:2 establish the rule for seam-mutating tests). Two concurrent
  rollback tests would race on the seam and trip the race detector.
- ❌ Don't invent a new Windows cleanup suffix. The Windows `platformRollback` routes the prior-current
  through `.old` PRECISELY so the EXISTING `CleanupOldBinary` (swap_windows.go) reclaims it next launch.
  Using a `.rollback-old` or similar would leak (no cleanup path). The `aside` scratch name (os.CreateTemp)
  is consumed by step 3 of the dance — verify with the "only 2 files remain" assertion.
- ❌ Don't edit the LANDED swap*.go / stage.go / *_test.go. They are CONSUMED (same-package helpers/seams),
  not modified. If a needed helper is missing, ADD it in a rollback*.go file (do not patch swap_test.go to
  export `restoreResolveCurrentExe` — it's already a package-level var reachable from rollback_test.go).
- ❌ Don't wire the `--rollback` flag or runUpgrade dispatch here. That is P1.M4.T1 (flag) + P1.M4.T2
  (runUpgrade `--rollback` branch). This item is the `Rollback` PRIMITIVE + its tests only. ZERO production
  callers after this subtask is expected.
- ❌ Don't sync docs here. The README/§19/§21 changeset sync is P3.M1. This item is the code primitive.