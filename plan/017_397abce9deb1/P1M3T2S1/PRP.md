name: "P1.M3.T2.S1 — Backup + atomic swap (Unix os.Rename / Windows .old dance) + non-writable→print sudo (FR-U5 step 7, FR-U7, FR-U4)"
description: >
  The ON-DISK change step of the direct-binary self-update (FR-U5 step 7 + FR-U7 + FR-U11 + FR-U4). THREE
  new files in package upgrade (stdlib-only, FR-U12) + ONE main.go line: (1) internal/upgrade/swap.go
  (SHARED, no build tag) — `func Swap(ctx, newBinPath string) error` orchestrator + the
  `ErrNeedsPrivileges` sentinel + a typed `NeedsPrivilegesError{Command string}` (Unwrap→sentinel, so
  errors.Is works AND the command layer reads .Command) + an injectable `var resolveCurrentExe` seam
  (clone of stage.go's `execVersion` idiom — lets tests point Swap at a temp-dir "exe") + shared
  `isWritable` (CreateTemp probe) + the os.TempDir()-guarded tempDir cleanup on success (P1.M3.T1.S2's
  contract: my item cleans tempDir on success, leaves it on failure); (2) internal/upgrade/swap_unix.go
  (`//go:build !windows`) — `platformSwap` (backup `os.Rename(current→<exe>.stagecoach-backup)` then
  `os.Rename(new→current)`, with FR-U11 restore-from-backup on swap failure) + `privilegeCommand` (FR-U7
  "sudo … upgrade" — the re-run-elevated form, robust vs the ephemeral tempDir) + `CleanupOldBinary` (Unix
  no-op); (3) internal/upgrade/swap_windows.go (`//go:build windows`) — `platformSwap` (the .old deferred-
  delete dance: rename running→.old, move new→current, FR-U11 restore on failure; never zero runnable
  binaries) + `privilegeCommand` (elevation hint) + `CleanupOldBinary` (best-effort os.Remove of the .old
  sibling at startup, FR-U7). cmd/stagecoach/main.go gains ONE line `upgrade.CleanupOldBinary()` after
  SetCurrentVersion (Windows: deletes a prior-launch .old; Unix: no-op via the twin). CONSUMES the parallel
  P1.M3.T1.S2 output `newBinPath` (= tempDir/new-stagecoach, already SHA256-verified + sanity-run); Swap
  trusts it and does ALL on-disk mutation of the running binary. The backup path derivation is a stable
  `backupPath` helper (in the twins — suffix differs: .stagecoach-backup vs .old) so P1.M3.T3.S1 (--rollback)
  reuses it. Tests: swap_test.go (`//go:build !windows`, NOT parallel — mutates resolveCurrentExe) —
  happy-path (temp "exe", assert new content + backup == old content + tempDir cleaned), not-writable→
  NeedsPrivilegesError (chmod dir 0500, SKIP if os.Geteuid()==0), overwrites-prior-backup (one-deep FR-U8),
  restore-on-swap-failure (FR-U11), errors.Is(ErrNeedsPrivileges), isWritable branches; swap_windows_test.go
  (`//go:build windows`) — the .old rename-dance + CleanupOldBinary deletes the .old sibling (CI's
  windows-latest runs it natively). Exit-code contract: Swap RETURNS NeedsPrivilegesError (never os.Exit);
  the command layer P1.M4.T2 catches it, prints .Command, exits 0 (FR-U4). NOT in scope: stage.go (S2),
  --rollback (P1.M3.T3.S1 reuses my backupPath), runUpgrade orchestration (P1.M4.T2), the confirmation
  prompt (P1.M4.T1.S2), exitcode mapping (FR-U4 printed-command exits 0 = Success, handled by the command
  layer), docs (P3.M1). go.mod unchanged (all imports stdlib).

---

## Goal

**Feature Goal**: Implement the backup + atomic-swap step of the direct-binary self-update (PRD §9.29
FR-U5 step 7 + FR-U7 + FR-U11): given P1.M3.T1.S2's verified+sanity-run `newBinPath`, move the current
binary to a one-deep backup (`<exe>.stagecoach-backup` on Unix / `<exe>.old` on Windows) and atomically
rename the new binary into its place — per-platform (Unix single `os.Rename` over the running inode;
Windows `.old` deferred-delete dance that never leaves zero runnable binaries). If the install dir is not
writable (e.g. `/usr/local/bin` root-owned), detect it BEFORE any rename, leave everything untouched, and
return a typed `NeedsPrivilegesError` carrying the exact `sudo` command for the command layer to print
(FR-U4 — never auto-elevate). A mid-swap rename failure restores from the backup (FR-U11 — never a
half-upgraded state). On success, clean the staging tempDir.

**Deliverable** (3 new upgrade files + 1 main.go line + 2 test files):
1. `internal/upgrade/swap.go` (NEW, shared) — `Swap(ctx, newBinPath) error` + `ErrNeedsPrivileges` +
   `NeedsPrivilegesError{Command}` + `var resolveCurrentExe` seam + `isWritable` + tempDir cleanup.
2. `internal/upgrade/swap_unix.go` (NEW, `//go:build !windows`) — `platformSwap` (backup+rename+restore) +
   `privilegeCommand` (sudo) + `backupPath` + `CleanupOldBinary` (no-op).
3. `internal/upgrade/swap_windows.go` (NEW, `//go:build windows`) — `platformSwap` (.old dance+restore) +
   `privilegeCommand` (elevation hint) + `backupPath` + `CleanupOldBinary` (delete .old sibling).
4. `cmd/stagecoach/main.go` — +1 line `upgrade.CleanupOldBinary()` at startup.
5. `internal/upgrade/swap_test.go` (NEW, `//go:build !windows`) — Unix unit tests.
6. `internal/upgrade/swap_windows_test.go` (NEW, `//go:build windows`) — Windows unit tests.

**Success Definition**:
- `Swap(ctx, newBinPath)` resolves the current exe (via the injectable `resolveCurrentExe` seam),
  proactively checks the install dir is writable, and on success backs up the old binary + atomically
  renames `newBinPath` into place + cleans the staging tempDir.
- Unix: `os.Rename(current, current+".stagecoach-backup")` then `os.Rename(newBinPath, current)`; a swap
  failure restores from the backup (FR-U11); a prior backup is overwritten (one-deep, FR-U8).
- Windows: `os.Rename(current, current+".old")` then `os.Rename(newBinPath, current)`; a swap failure
  restores from `.old` (FR-U11); `CleanupOldBinary()` best-effort deletes the `.old` sibling at the next
  launch (FR-U7).
- A non-writable install dir → Swap returns `*NeedsPrivilegesError` with `.Command` set to the sudo/re-run
  command, `errors.Is(err, ErrNeedsPrivileges)` true, and NOTHING on disk changed (FR-U11 — detected before
  any rename). Swap NEVER `os.Exit`s (the command layer P1.M4.T2 prints `.Command` + exits 0, FR-U4).
- `upgrade.CleanupOldBinary()` is called once at startup (main.go); Windows deletes a prior `.old`, Unix
  is a no-op (build-tag twin).
- `go build ./...` + `GOOS={linux,darwin,windows}` all clean; `go vet` clean; `gofmt -l` empty;
  `go test ./internal/upgrade/ -race` green; `make test` (incl. windows-latest) + `make lint` clean.
- Stdlib-only (FR-U12): no `internal/*` import in any new upgrade file. go.mod unchanged.
- Scope: `git status --porcelain` == the 5 new files + main.go (1 line). ZERO production callers of Swap
  after this subtask (consumer is P1.M4.T2 runUpgrade) — expected.

## User Persona (if applicable)

**Target User**: The `stagecoach upgrade` command layer (P1.M4.T2 `runUpgrade`), which calls
ResolveTarget (P1.M3.T1.S1) → StageNewBinary (P1.M3.T1.S2) → Swap (this task) to atomically replace the
running binary. End users never call Swap directly.

**Use Case**: `stagecoach upgrade` (direct-binary channel) → StageNewBinary prepares a verified
`tempDir/new-stagecoach` → Swap backs up the old `stagecoach` and atomically renames `new-stagecoach` →
`stagecoach`. Next invocation runs the new version (Unix: the old inode is gone from the path; Windows:
the path now resolves to the new file). If the install is `/usr/local/bin` (root-owned), Swap returns
NeedsPrivilegesError and the command layer prints `sudo "/usr/local/bin/stagecoach" upgrade` + exits 0.

**User Journey**: user runs `stagecoach upgrade` → runUpgrade (P1.M4.T2) → StageNewBinary (downloads +
verifies + sanity-runs to tempDir/new-stagecoach) → Swap: [writable → backup + atomic rename + tempDir
  cleanup → done] OR [not writable → NeedsPrivilegesError{Command: sudo …} → command layer prints it,
  exit 0, running binary byte-for-byte unchanged (FR-U4/U11)].

**Pain Points Addressed**: FR-U5 step 7 + FR-U7 + FR-U11 — the self-swap must be ATOMIC (one rename on
Unix; a two-step rename on Windows that never leaves zero binaries), must NEVER corrupt the running
binary on a mid-swap failure (restore from backup), and must NEVER auto-elevate (detect non-writable →
print sudo, exit 0). This task is the on-disk gate that makes self-update safe.

## Why

- **FR-U5 step 7 + FR-U7**: the direct-binary swap is platform-specific (Unix single atomic rename;
  Windows `.old` deferred-delete dance). This task implements both behind a shared orchestrator.
- **FR-U11 (never half-upgraded)**: the proactive writability gate + restore-on-failure mean a
  non-writable dir or a mid-swap rename failure leaves the installed binary byte-for-byte unchanged. There
  is no corrupt intermediate state.
- **FR-U4 (never auto-elevate)**: a tool that both writes binaries AND auto-`sudo`s is a footgun. Swap
  detects non-writable, leaves everything untouched, and returns the exact sudo command for the user to
  run (the command layer prints it, exit 0). Auto-privilege-escalation is explicitly forbidden.
- **Consumes S2, enables rollback**: Swap consumes P1.M3.T1.S2's verified `newBinPath` (it does not
  re-verify — trust the sanity gate). The `backupPath` helper it defines is the STABLE contract
  P1.M3.T3.S1 (--rollback) restores from — so the backup naming must be a shared helper, not an inline
  literal.
- **Bounded, no-conflict scope**: 3 new upgrade files + 1 main.go line + 2 test files. stage.go is S2's
  (parallel); swap*.go is this task's. No edit to stage.go/detect.go/resolve.go/delegate.go/exitcode.go.

## What

**User-visible behavior**: a successful `stagecoach upgrade` (direct channel) atomically replaces the
running binary; the next `stagecoach` invocation runs the new version (and on Windows the prior `.exe.old`
is cleaned at the following launch). A non-writable install prints `sudo "<exe>" upgrade` and exits 0
(the user re-runs elevated). No partial state is ever left on disk.

**Technical change**: a shared `Swap` orchestrator + build-tagged `platformSwap` twins + a startup
`CleanupOldBinary` twin + a typed `NeedsPrivilegesError`. Stdlib-only.

### Success Criteria
- [ ] `internal/upgrade/swap.go` exports `func Swap(ctx context.Context, newBinPath string) error`, the
      `ErrNeedsPrivileges` sentinel, and `type NeedsPrivilegesError struct { Command string }` with
      `Error()` + `Unwrap() error { return ErrNeedsPrivileges }`.
- [ ] `swap.go` defines `var resolveCurrentExe = func() (string, error) { … }` (os.Executable +
      EvalSymlinks; injectable — clone of stage.go's `execVersion` seam).
- [ ] `Swap` checks `isWritable(filepath.Dir(currentExe))` BEFORE any rename; on false returns
      `&NeedsPrivilegesError{Command: privilegeCommand(currentExe)}` with nothing changed.
- [ ] `Swap` calls `platformSwap(currentExe, newBinPath)` (build-tagged twin); on success best-effort
      `os.RemoveAll(filepath.Dir(newBinPath))` GUARDED by an `os.TempDir()` prefix check.
- [ ] `swap_unix.go` (`//go:build !windows`): `platformSwap` does backup `os.Rename(current,
      backupPath(current))` then `os.Rename(newBinPath, current)`; on swap-failure restores
      `os.Rename(backupPath(current), current)` (FR-U11). `privilegeCommand` returns `sudo "<exe>" upgrade`.
      `CleanupOldBinary()` is a no-op. `backupPath(exe)` returns `exe+".stagecoach-backup"`.
- [ ] `swap_windows.go` (`//go:build windows`): `platformSwap` does `os.Rename(current, current+".old")`
      then `os.Rename(newBinPath, current)`; on swap-failure restores `os.Rename(current+".old", current)`
      (FR-U11). `CleanupOldBinary()` best-effort `os.Remove` of the `.old` sibling of the resolved exe.
      `backupPath(exe)` returns `exe+".old"`. `privilegeCommand` returns an elevation hint.
- [ ] `cmd/stagecoach/main.go` adds `upgrade.CleanupOldBinary()` after `upgrade.SetCurrentVersion(version)`.
- [ ] `swap_test.go` (`//go:build !windows`, NOT parallel) covers: happy-path, not-writable→
      NeedsPrivilegesError (skip if `os.Geteuid()==0`), overwrites-prior-backup, restore-on-failure,
      errors.Is(ErrNeedsPrivileges) + .Command, isWritable branches.
- [ ] `swap_windows_test.go` (`//go:build windows`) covers the `.old` rename-dance + CleanupOldBinary
      (runs natively on windows-latest CI).
- [ ] `go build ./...` + `GOOS={linux,darwin,windows}` clean; `go vet` clean; `gofmt -l` empty on all new files.
- [ ] `go test ./internal/upgrade/ -race` green; `make test` (matrix incl. windows-latest) + `make lint` clean.
- [ ] NO `internal/*` import in swap.go/swap_unix.go/swap_windows.go (FR-U12; grep guard). go.mod unchanged.
- [ ] `git status --porcelain` == swap.go + swap_unix.go + swap_windows.go + swap_test.go +
      swap_windows_test.go + cmd/stagecoach/main.go.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the authoritative FR-U4/U5/U7/U8/U11/U12 spec (quoted), the parallel S2 handoff contract
(`newBinPath = tempDir/new-stagecoach`; my item cleans tempDir on success / leaves on failure), the exact
3-file design (shared swap.go + build-tagged twins) with the platformSwap/privilegeCommand/CleanupOldBinary
split, the verbatim conventions to clone (build-tag form from signal_unix.go/editor_run_windows.go,
`errors.New("upgrade: …")` sentinels, the `execVersion` package-level-seam idiom from stage.go, the
CASError typed-error-with-Unwrap precedent, detect.go's EvalSymlinks-then-fallback exe resolution), the
sudo-command form (FR-U7 "re-run": `sudo "<exe>" upgrade`), the run-as-root test gotcha (`os.Geteuid()==0`
skip), the FR-U11 restore-on-failure logic for both platforms, the backupPath stability contract for
P1.M3.T3.S1 rollback, the CI matrix (windows-latest runs the _windows tests natively), and 9 grep guards.

### Documentation & References

```yaml
# MUST READ — the authoritative swap spec (FR-U4/U5 step 7/U7/U8/U11/U12) — quoted verbatim
- docfile: plan/017_397abce9deb1/prd_snapshot.md
  section: "§9.29 FR-U4 (never auto-elevate; printed command exits 0), FR-U5 step 7 (backup <exe>.stagecoach-backup, one-deep), FR-U7 (Unix os.Rename atomic; Windows .old deferred-delete dance; non-writable→print sudo), FR-U8 (--rollback reuses the backup), FR-U11 (atomic by construction; abort-before-swap; never half-upgraded), FR-U12 (walled off, stdlib-only)"
  why: "FR-U7 is the per-platform swap contract: Unix os.Rename(newTemp, currentExe) is atomic (running process keeps old inode); Windows renames running→.old (allowed) then moves new→exe, cleans .old at next launch. FR-U4: never auto-sudo — print the command, exit 0. FR-U11: detect non-writable BEFORE any rename; restore on failure."
  critical: "FR-U7 Unix: 'If the target path is not writable, detect it, leave everything untouched, and print the exact sudo command to re-run (never auto-elevate — FR-U4).' FR-U7 Windows: 'there is never zero runnable binaries.' FR-U12: upgrade imports NO internal/*."

# MUST READ — codebase-specific findings for THIS item (the design + all conventions + gotchas)
- docfile: plan/017_397abce9deb1/P1M3T2S1/research/findings.md
  why: "§0 the FR-U spec (quoted); §1 the S2 handoff (newBinPath=tempDir/new-stagecoach; MY item cleans tempDir on success/leaves on failure; stage.go reserved swap.go for me); §2 the 3-file design (shared swap.go + swap_unix.go + swap_windows.go + main.go line); §3 conventions (build-tag twins, errors.New('upgrade: …'), execVersion seam→resolveCurrentExe, CASError→NeedsPrivilegesError, EvalSymlinks-then-fallback); §4 the run-as-root test skip (os.Geteuid()==0); §5 the sudo form (FR-U7 're-run': sudo '<exe>' upgrade); §6 FR-U11 restore-on-fail per platform; §7 the test matrix (Unix swap_test.go + Windows swap_windows_test.go on windows-latest); §8 scope fences; §9 validation cmds; §10 backupPath is the rollback contract (P1.M3.T3.S1 reuses it)."
  critical: "Swap must DETECT non-writable BEFORE any rename (FR-U11) and RESTORE from backup on a mid-swap rename failure (never zero runnable binaries). NeedsPrivilegesError.Command = sudo \"<exe>\" upgrade (FR-U7 're-run'). The resolveCurrentExe seam is MANDATORY for testability (you can't os.Rename over a running binary in a unit test). The swap_test.go file must NOT be t.Parallel() (mutates the seam)."

# MUST READ — the parallel S2 PRP (the contract Swap consumes — newBinPath + tempDir-cleanup ownership)
- docfile: plan/017_397abce9deb1/P1M3T1S2/PRP.md
  why: "Defines StageNewBinary(ctx, c, release, asset, tempDir) → newBinPath = tempDir/new-stagecoach(.exe),
        ALREADY SHA256-verified + sanity-run. 'The tempDir is cleaned by P1.M3.T2 on the success path; on
        failure it is left for the user/dev to inspect.' → MY Swap owns success-path tempDir cleanup.
        stage.go NEVER touches the running binary — MY Swap owns ALL on-disk mutation. stage.go is NOT
        swap.go (reserved for me) — no file-name collision."
  critical: "Swap TRUSTS newBinPath (it does not re-verify or re-sanity-run — S2 already did). Swap's job is
             backup + atomic rename + tempDir cleanup. The staging sentinels (ErrChecksumMismatch etc.) are
             S2's — Swap does not produce them."

# MUST READ — the seam idiom to CLONE (stage.go's execVersion → resolveCurrentExe)
- file: internal/upgrade/stage.go
  why: "Lines 44-55: `var execVersion = func(ctx, path) ([]byte, error) { … }` — the package-level injectable
        seam. Tests override it (stage_test.go is NOT parallel). CLONE this for `var resolveCurrentExe =
        func() (string, error) { exe, err := os.Executable(); …; return filepath.EvalSymlinks(exe) }`. The
        swap_test.go that overrides it must NOT be t.Parallel() either."
  pattern: "Package-level `var <name> = func(…) { … }`; production default uses the real stdlib call; tests
            assign a stub; the test file skips t.Parallel(). This is how Swap becomes testable against a
            temp-dir 'exe' instead of the real running binary."
  gotcha: "Do NOT make Swap a struct-method with fields (the contract pins `func Swap(ctx, newBinPath)`). Use
           the package-level var seam to keep the function signature AND gain injectability."

# MUST READ — the build-tag twin convention to CLONE
- file: internal/signal/signal_unix.go
  why: "Line 1: `//go:build !windows`, BLANK LINE, `package signal`. Defines a function (KillProcessGroup)
        whose Windows twin is in signal_windows.go. This is the EXACT split for platformSwap/
        privilegeCommand/CleanupOldBinary/backupPath: define each in BOTH swap_unix.go and swap_windows.go."
- file: internal/generate/editor_run_windows.go
  why: "Line 1: `//go:build windows`. The Windows twin of editor_run.go. Confirms the modern build-tag form
        (no legacy `// +build`) + the blank-line-before-package rule. parseEditorArgv lives ONLY in the
        windows file (unused on the !windows build) — Go allows unused package-level funcs, just not unused
        imports. Mirror: CleanupOldBinary's Windows-only logic lives in swap_windows.go."
  pattern: "Shared orchestrator in an UNTAGGED file (swap.go) calls platform-specific funcs defined in BOTH
            build-tagged twins. NO legacy `// +build` line (Go 1.22 codebase)."

# CONTEXT — the typed-error-with-Unwrap precedent (CASError) for NeedsPrivilegesError
- file: internal/generate/case_error.go
  why: "*generate.CASError{TreeSHA, Expected, Actual, …} has Error() + Unwrap() returning a sentinel, so
        errors.Is works AND callers type-assert to read fields. CLONE this shape for NeedsPrivilegesError
        {Command string} + Unwrap() → ErrNeedsPrivileges. The command layer (P1.M4.T2) does
        `var npe *upgrade.NeedsPrivilegesError; errors.As(err, &npe); print(npe.Command)`."
  pattern: "type XError struct { … }; func (e *XError) Error() string; func (e *XError) Unwrap() error { return ErrX }"

# CONTEXT — the exe-path resolution pattern (detect.go's EvalSymlinks-then-fallback)
- file: internal/upgrade/detect.go
  why: "Lines 352-354: `real, err := filepath.EvalSymlinks(d.ExePath); if err != nil { real = d.ExePath }` —
        tolerate EvalSymlinks failure by falling back to the raw path. resolveCurrentExe should do the same
        (os.Executable → EvalSymlinks → on error fall back to the os.Executable result). Also shows the
        Detector injectable-struct idiom — but Swap uses the function + package-level seam, NOT a struct
        (the contract pins the signature)."
  gotcha: "os.Executable() returns the running binary's path; on macOS it may be under /private/var (a
           symlink). EvalSymlinks canonicalizes it. The fallback-on-error keeps a test 'exe' (a regular temp
           file) working even if EvalSymlinks has opinions."

# CONTEXT — main.go (the ONE startup line for CleanupOldBinary)
- file: cmd/stagecoach/main.go
  why: "main() (line 58): calls upgrade.SetCurrentVersion(version) at line 60. ADD `upgrade.CleanupOldBinary()`
        immediately after (line 61). main.go is cross-platform (no build tag) — the platform logic lives in
        the upgrade build-tag twins (FR-U12: upgrade stays stdlib-only; main.go already imports upgrade)."
  critical: "Add the call BEFORE cmd.Execute(ctx) (cleanup is best-effort + early; it must not run inside the
             commit path). ONE line only — do not restructure main()."

# CONTEXT — PRD §10.5 v3.0 (self-update scope) + §15.4 exit codes (printed-command = 0)
- docfile: plan/017_397abce9deb1/prd_snapshot.md
  section: "§10.5 v3.0 (self-update is the named §19 network exception) + §15.4 (exit 0 = success/up-to-date/printed-the-command)"
  why: "FR-U4: a printed command exits 0. Swap does NOT os.Exit — it RETURNS NeedsPrivilegesError; the command
        layer (P1.M4.T2) catches it, prints .Command, exits 0 (exitcode.Success). So Swap needs NO exitcode.go
        mapping (FR-U4's printed-command = Success = 0, handled by the command layer)."
```

### Current Codebase tree (relevant slice)

```bash
internal/upgrade/
  releases.go / download.go / resolve.go / detect.go / delegate.go / stage.go  # READ-ONLY — LANDED/landing siblings
  version.go                   # READ-ONLY — SetCurrentVersion (main.go calls it)
  swap.go                      # NEW (shared) — Swap + ErrNeedsPrivileges + NeedsPrivilegesError + resolveCurrentExe + isWritable
  swap_unix.go                 # NEW (//go:build !windows) — platformSwap + privilegeCommand + backupPath + CleanupOldBinary(no-op)
  swap_windows.go              # NEW (//go:build windows)  — platformSwap (.old dance) + privilegeCommand + backupPath + CleanupOldBinary
  swap_test.go                 # NEW (//go:build !windows) — Unix unit tests (NOT parallel — mutates resolveCurrentExe)
  swap_windows_test.go         # NEW (//go:build windows)  — Windows unit tests (runs on windows-latest CI)
cmd/stagecoach/
  main.go                      # EDIT — +1 line: upgrade.CleanupOldBinary() after SetCurrentVersion
internal/exitcode/
  exitcode.go                  # READ-ONLY — printed-command exits 0 (Success); handled by command layer, NOT Swap
internal/signal/signal_{unix,windows}.go / internal/generate/editor_run{,_windows}.go  # READ-ONLY — build-tag twin convention to clone
go.mod                         # READ-ONLY — module github.com/dabstractor/stagecoach; go.mod UNCHANGED (all new imports stdlib)
.github/workflows/ci.yml       # matrix os: [ubuntu-latest, macos-latest, windows-latest] — windows-latest runs swap_windows_test.go natively
```

### Desired Codebase tree with files to be added/modified

```bash
internal/upgrade/
  swap.go                      # NEW (shared) — Swap orchestrator + sentinels + seam + isWritable + tempDir cleanup
  swap_unix.go                 # NEW — Unix platformSwap (backup+rename+restore) + sudo command + no-op cleanup
  swap_windows.go              # NEW — Windows platformSwap (.old dance+restore) + elevation hint + .old cleanup
  swap_test.go                 # NEW — Unix tests (happy / not-writable / overwrites-backup / restore / errors.Is / isWritable)
  swap_windows_test.go         # NEW — Windows tests (.old rename-dance + CleanupOldBinary)
cmd/stagecoach/
  main.go                      # MODIFIED — +upgrade.CleanupOldBinary() (1 line)
# NOTHING ELSE. No edit to stage.go, detect.go, resolve.go, delegate.go, releases.go, download.go,
# exitcode.go, go.mod, or any PRD/task file.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (FR-U11 — detect non-writable BEFORE any rename): the isWritable probe (CreateTemp in the exe's
// dir) MUST run before platformSwap. If it's skipped and Swap dives into backup+rename against a root-owned
// dir, the FIRST os.Rename(current→backup) fails with EACCES AFTER already... no — os.Rename(current→backup)
// also needs dir write, so it'd fail too, but you'd have a confusing mid-flow error. The proactive probe
// gives the clean NeedsPrivilegesError path (exit 0, nothing changed). Always probe first.

// CRITICAL (FR-U11 — restore on mid-swap failure, never zero runnable binaries): Unix platformSwap does
// backup THEN swap; if swap fails, os.Rename(backup→current) restores. Windows platformSwap does
// move-running→.old THEN move-new→current; if the second move fails, os.Rename(.old→current) restores
// (the running image was aside; move it back). Both wrap the restore error if it also fails. NEVER delete
// newBinPath or the backup on failure (leave for inspection).

// CRITICAL (run-as-root test gotcha): the not-writable test (chmod dir 0500) is MEANINGLESS as root — root
// bypasses Unix permission bits, so os.Rename + CreateTemp both succeed. Guard with
// `if os.Geteuid() == 0 { t.Skip("permission test meaningless as root") }`. (CI ubuntu/macos run as a normal
// user, so it works there; the skip covers self-hosted/dev root runs.) Restore dir perms in t.Cleanup so
// t.TempDir's RemoveAll doesn't fail on a 0500 dir.

// CRITICAL (the resolveCurrentExe seam is MANDATORY for testability): you CANNOT os.Rename over the real
// running binary in a unit test (it's executing; and you can't test /usr/local/bin portably). The package-
// level `var resolveCurrentExe = func() (string, error) { … }` seam (clone of stage.go's execVersion) lets
// tests point Swap at a temp-dir "exe" (a regular file). swap_test.go MUST NOT call t.Parallel() (it
// mutates the package-level seam).

// CRITICAL (Swap NEVER os.Exit; FR-U4 printed-command exits 0 = the command layer's job): Swap RETURNS
// *NeedsPrivilegesError; it does not print or exit. The command layer (P1.M4.T2) catches it (errors.As),
// prints .Command, exits 0 (exitcode.Success). So Swap needs NO exitcode.go mapping — do not add one.

// GOTCHA (the sudo command is the FR-U7 "re-run" form, not a one-shot install): NeedsPrivilegesError.Command
// = `sudo "<currentExe>" upgrade` — re-run the WHOLE upgrade elevated. Do NOT use `sudo install <newBinPath>
// <currentExe>`: newBinPath is in an EPHEMERAL per-invocation tempDir (os.MkdirTemp) that may be gone by the
// time the user runs the command. Re-running elevated is robust (re-does detect→…→swap as root).

// GOTCHA (tempDir cleanup is GUARDED by os.TempDir() prefix): on success Swap does os.RemoveAll(filepath.Dir
// (newBinPath)) ONLY IF that dir is under os.TempDir(). The parallel PRP's tempDir IS os.MkdirTemp (always
// under os.TempDir), so this always matches in prod. The guard prevents nuking an arbitrary dir if a caller
// ever passes a newBinPath outside temp. On failure, NEVER clean (leave for inspection).

// GOTCHA (backupPath must be a SHARED, STABLE helper — the rollback contract): P1.M3.T3.S1 (--rollback)
// restores the backup Swap creates. The suffix differs by platform (.stagecoach-backup vs .old), so put
// `func backupPath(exe string) string` in the BUILD-TAGGED twins (not inline in platformSwap). Document it
// so P1.M3.T3.S1 calls backupPath() rather than re-deriving the suffix.

// GOTCHA (Windows os.Rename replaces; the running-image lock is the constraint, not overwrite): Go's
// os.Rename on Windows uses MoveFileEx(MOVEFILE_REPLACE_EXISTING), so it replaces an existing dest. The .old
// dance exists because the RUNNING exe is LOCKED (can't be overwritten IN PLACE) but CAN be RENAMED. So:
// rename running→.old (allowed), then rename new→exe (the exe path is now free). A prior .old from a
// previous run is replaced by the first rename (one-deep, FR-U8).

// GOTCHA (CleanupOldBinary deletes the .old at the NEXT launch, not this one): the .old created by this
// run's swap is STILL LOCKED while THIS process runs (it IS this process's image, renamed). CleanupOldBinary
// (called at startup) deletes a .old left by the PREVIOUS launch — this process's own .old is cleaned next
// time. Ignore the Remove error (best-effort). Resolve own exe via os.Executable+EvalSymlinks (use the
// resolveCurrentExe seam for testability).

// GOTCHA (stdlib-only, FR-U12): swap.go/swap_unix.go/swap_windows.go import ONLY os, path/filepath, errors,
// fmt, context, strings. NO internal/* (grep guard). main.go already imports upgrade — that's fine; the wall
// is one-directional (upgrade never imports internal/*).
```

## Implementation Blueprint

### Data models and structure

```go
// NeedsPrivilegesError — typed error carrying the sudo/re-run command (clone of generate.CASError's shape).
// errors.Is(err, ErrNeedsPrivileges) is true (Unwrap chain); the command layer type-asserts to read .Command.
type NeedsPrivilegesError struct {
	Command string // the exact privilege-elevation command, ready to paste (e.g. `sudo "/usr/local/bin/stagecoach" upgrade`)
}
func (e *NeedsPrivilegesError) Error() string { return fmt.Sprintf("%s: %s", ErrNeedsPrivileges, e.Command) }
func (e *NeedsPrivilegesError) Unwrap() error { return ErrNeedsPrivileges }

// No new structs/types beyond this. Swap/platformSwap/isWritable/backupPath/privilegeCommand/CleanupOldBinary
// are plain functions. resolveCurrentExe is a package-level var (the seam).
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/upgrade/swap.go (SHARED — no build tag) — orchestrator + sentinels + seam
  - FILE COMMENT: a doc comment explaining this is FR-U5 step 7 + FR-U7 (backup + atomic swap), per-platform
    via the swap_unix.go/swap_windows.go twins, FR-U11 (proactive writability gate + restore-on-fail), FR-U4
    (non-writable→NeedsPrivilegesError, never auto-elevate). File comment only — releases.go owns the pkg doc.
  - IMPORTS (all stdlib): context, errors, fmt, os, path/filepath, strings.
  - SENTINEL: `var ErrNeedsPrivileges = errors.New("upgrade: install path not writable; re-run with privileges (FR-U4)")`
  - TYPED ERROR: `type NeedsPrivilegesError struct { Command string }` + Error() + Unwrap()→ErrNeedsPrivileges.
  - SEAM: `var resolveCurrentExe = func() (string, error) { exe, err := os.Executable(); if err != nil { return "", fmt.Errorf("os.Executable: %w", err) }; real, err := filepath.EvalSymlinks(exe); if err != nil { return exe, nil }; return real, nil }`
    (Clone stage.go's execVersion idiom. EvalSymlinks-then-fallback per detect.go:352-354.)
  - `func Swap(ctx context.Context, newBinPath string) error`:
      if err := ctx.Err(); err != nil { return err }   // reference ctx (sync fs ops don't take it)
      currentExe, err := resolveCurrentExe()
      if err != nil { return fmt.Errorf("upgrade: resolve current exe: %w", err) }
      if !isWritable(filepath.Dir(currentExe)) {
          return &NeedsPrivilegesError{Command: privilegeCommand(currentExe)}   // FR-U4/U11 — nothing changed
      }
      if err := platformSwap(currentExe, newBinPath); err != nil {
          return fmt.Errorf("upgrade: swap: %w", err)
      }
      // success: best-effort tempDir cleanup (guarded — only if under os.TempDir())
      if dir := filepath.Dir(newBinPath); isTempDir(dir) { _ = os.RemoveAll(dir) }
      return nil
  - `func isWritable(dir string) bool { f, err := os.CreateTemp(dir, ".stagecoach-writetest-*"); if err != nil { return false }; name := f.Name(); f.Close(); os.Remove(name); return true }`
  - `func isTempDir(dir string) bool { abs, err := filepath.Abs(dir); if err != nil { return false }; tmp, err := filepath.Abs(os.TempDir()); if err != nil { return false }; return strings.HasPrefix(abs, tmp+string(filepath.Separator)) }`
  - NAMING: Swap, ErrNeedsPrivileges, NeedsPrivilegesError, resolveCurrentExe, isWritable, isTempDir (all
    matching the contract + the package's errors.New("upgrade: …") convention).
  - GOTCHA: platformSwap + privilegeCommand are NOT defined here — they live in the build-tag twins. Swap
    CALLS them (the Go compiler resolves them per-build via the twin).

Task 2: CREATE internal/upgrade/swap_unix.go (`//go:build !windows`) — Unix backup+rename+restore
  - HEADER: `//go:build !windows`, BLANK LINE, `package upgrade`. (Clone signal_unix.go:1 — no legacy +build.)
  - IMPORTS: fmt, os, path/filepath (all stdlib).
  - `func backupPath(exe string) string { return exe + ".stagecoach-backup" }`  // FR-U5 step 7; FR-U8 rollback source
  - `func platformSwap(currentExe, newBinPath string) error`:
      backup := backupPath(currentExe)
      if err := os.Rename(currentExe, backup); err != nil {   // one-deep: replaces a prior backup (FR-U8)
          return fmt.Errorf("backup current binary: %w", err)
      }
      if err := os.Rename(newBinPath, currentExe); err != nil {
          _ = os.Rename(backup, currentExe)                    // FR-U11 restore (best-effort; running inode safe)
          return fmt.Errorf("rename new binary into place: %w (restored from backup)", err)
      }
      return nil
  - `func privilegeCommand(exe string) string { return fmt.Sprintf("sudo %q upgrade", exe) }`  // FR-U7 "re-run"
  - `func CleanupOldBinary() {}`  // no-op on Unix (.old dance is Windows-only)
  - GODOC [Mode A] on platformSwap: explain Unix atomicity (os.Rename over a running inode is safe — the
    kernel keeps the old inode open for the running process; the path now resolves to the new file; next
    invocation runs the new version), the one-deep backup (FR-U8), and the FR-U11 restore-on-swap-failure.

Task 3: CREATE internal/upgrade/swap_windows.go (`//go:build windows`) — Windows .old deferred-delete dance
  - HEADER: `//go:build windows`, BLANK LINE, `package upgrade`. (Clone editor_run_windows.go:1.)
  - IMPORTS: fmt, os, path/filepath (all stdlib).
  - `func backupPath(exe string) string { return exe + ".old" }`
  - `func platformSwap(currentExe, newBinPath string) error`:
      old := backupPath(currentExe)
      if err := os.Rename(currentExe, old); err != nil {   // Windows: renaming a running image is ALLOWED
          return fmt.Errorf("rotate current binary to .old: %w", err)
      }
      if err := os.Rename(newBinPath, currentExe); err != nil {
          _ = os.Rename(old, currentExe)                    // FR-U11 restore (move the running image back)
          return fmt.Errorf("move new binary into place: %w (restored from .old)", err)
      }
      return nil   // the .old is NOT deleted here — it's locked while this process runs; CleanupOldBinary
                   // (next launch) deletes it. FR-U7.
  - `func privilegeCommand(exe string) string` → a Windows elevation hint (per-user installs work via the
    .old dance; system-wide C:\Program Files installs need elevation — carry a clear message; the command
    layer may refine). Keep minimal but non-empty.
  - `func CleanupOldBinary()`:
      exe, err := resolveCurrentExe(); if err != nil { return }
      _ = os.Remove(backupPath(exe))   // best-effort: deletes a .old left by the PREVIOUS launch.
        // (This process's own .old is still locked; it'll be cleaned next launch. Ignore the error.)
  - GODOC [Mode A] on platformSwap: explain the Windows constraint (the running .exe is LOCKED — can't be
    overwritten in place, but CAN be renamed), the .old dance (rotate running→.old, move new→exe; never zero
    runnable binaries because the new binary already passed S2's sanity-run), the FR-U11 restore, and that
    the .old is cleaned at the NEXT launch (CleanupOldBinary, FR-U7).

Task 4: EDIT cmd/stagecoach/main.go — add the startup cleanup call (1 line)
  - After line 60 (`upgrade.SetCurrentVersion(version)`), ADD:
      upgrade.CleanupOldBinary()   // FR-U7 Windows: delete a prior-launch .exe.old (no-op on Unix)
  - NO other main.go change. NO build tag in main.go (the platform logic lives in the upgrade twin).
  - PRESERVE: the existing signal.Install / cmd.Execute / exitcode.For / os.Exit flow.

Task 5: CREATE internal/upgrade/swap_test.go (`//go:build !windows`) — Unix tests (NOT parallel)
  - HEADER: `//go:build !windows`, BLANK LINE, `package upgrade`. IMPORTS: bytes, context, errors, os,
    path/filepath, testing. (NOT t.Parallel() — mutates resolveCurrentExe.)
  - helper: `writeExe(t, path, content string)` — os.WriteFile + os.Chmod 0o755 (a fake runnable "exe").
  - TestSwap_HappyPath:
      tempDir := t.TempDir(); exe := filepath.Join(tempDir, "stagecoach"); writeExe(t, exe, "OLD")
      newDir := t.TempDir(); newBin := filepath.Join(newDir, "new-stagecoach"); writeExe(t, newBin, "NEW")
      resolveCurrentExe = func() (string, error) { return exe, nil }; t.Cleanup(restore)   // the seam
      if err := Swap(context.Background(), newBin); err != nil { t.Fatal(err) }
      assert read(exe) == "NEW"; assert read(exe+".stagecoach-backup") == "OLD"
      // newDir cleaned? (it's under t.TempDir, NOT os.TempDir, so the guard SKIPS removal — to test the
      //  cleanup, place newDir under os.TempDir via os.MkdirTemp, OR assert the guard skips). For a clean
      //  happy-path, use os.MkdirTemp("",…) for newDir so the cleanup guard matches.
  - TestSwap_NotWritableNeedsPrivileges:
      if os.Geteuid() == 0 { t.Skip("permission test meaningless as root") }
      tempDir := t.TempDir(); exe := filepath.Join(tempDir, "stagecoach"); writeExe(t, exe, "OLD")
      newBin := …; require.NoError(os.Chmod(tempDir, 0o500)); t.Cleanup(func(){ os.Chmod(tempDir,0o700) })
      resolveCurrentExe = func() (string,error){ return exe,nil }
      err := Swap(ctx, newBin); var npe *NeedsPrivilegesError
      require errors.As(err, &npe); require errors.Is(err, ErrNeedsPrivileges)
      assert strings.Contains(npe.Command, "sudo") && strings.Contains(npe.Command, exe)
      assert read(exe) == "OLD" (unchanged — FR-U11) && no backup file exists
  - TestSwap_OverwritesPriorBackup:
      pre-create exe+".stagecoach-backup" with "ANCIENT"; run Swap; assert backup now reads "OLD" (FR-U8 one-deep).
  - TestSwap_RestoresOnSwapFailure:
      make the new→current rename fail: e.g. newBin in a dir whose target rename crosses filesystems AND the
      rename fails, OR (cleaner) make currentExe's DIR read-only AFTER the backup succeeds so the swap
      rename fails — then assert read(exe) == "OLD" (restored from backup). If hard to make deterministic,
      document the restore path with a code comment + rely on grep guard 7 (the restore os.Rename exists).
      (A simple approach: after backup, chmod tempDir 0o500 so os.Rename(new→exe) fails EACCES; the restore
       os.Rename(backup→exe) also needs dir write… so restore also fails. To test restore SUCCESS, mock
       platformSwap via a seam OR use a newBin on a different filesystem where rename-fails-but-backup-
       restore-succeeds. Prefer: skip if impractical + grep-guard the restore call.)
  - TestNeedsPrivilegesError_ErrorsIs:
      err := &NeedsPrivilegesError{Command: `sudo "/x" upgrade`}; assert errors.Is(err, ErrNeedsPrivileges);
      assert err.Command == `sudo "/x" upgrade`.
  - TestIsWritable:
      writable temp dir → true; chmod 0o500 (skip if root) → false; non-existent dir → false.

Task 6: CREATE internal/upgrade/swap_windows_test.go (`//go:build windows`) — Windows tests
  - HEADER: `//go:build windows`, BLANK LINE, `package upgrade`. IMPORTS: context, os, path/filepath, testing.
  - TestPlatformSwap_OldDance:
      tempDir; exe := …\stagecoach.exe (write "OLD"); newBin := …\new.exe (write "NEW")
      if err := platformSwap(exe, newBin); err != nil { t.Fatal(err) }
      assert read(exe) == "NEW"; assert read(exe+".old") == "OLD"
      (Regular files prove the rename sequence. The running-image lock is a kernel property verified by the
       windows-latest CI run of the full e2e in P1.M4.T3.)
  - TestCleanupOldBinary_DeletesOldSibling:
      tempDir; exe := …\stagecoach.exe (write "X"); old := exe+".old" (write "STALE")
      resolveCurrentExe = func() (string,error){ return exe,nil }; t.Cleanup(restore)
      CleanupOldBinary(); assert old does NOT exist (deleted).
  - NOTE: these run natively on windows-latest CI (.github/workflows/ci.yml:47 matrix). At minimum the
    _windows file compiles + the rename-dance logic is covered.

Task 7: VERIFY — build (native+cross), vet, format, tests (Unix + Windows via CI), lint, grep guards
  - go build ./... ; GOOS=linux go build ./... ; GOOS=darwin go build ./... ; GOOS=windows go build ./...
  - go vet ./internal/upgrade/... ./cmd/...
  - gofmt -l internal/upgrade/swap.go internal/upgrade/swap_unix.go internal/upgrade/swap_windows.go internal/upgrade/swap_test.go internal/upgrade/swap_windows_test.go cmd/stagecoach/main.go   # empty
  - go test ./internal/upgrade/ -race -v     # Unix tests on ubuntu/macos
  - GOOS=windows go vet ./internal/upgrade/...   # the _windows file vets clean
  - make test                                # matrix incl. windows-latest runs swap_windows_test.go
  - make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the shared orchestrator calls build-tagged twins — clone of signal's split):
//   swap.go (no tag):        Swap → resolveCurrentExe (seam) → isWritable → platformSwap → tempDir cleanup
//   swap_unix.go (!windows): platformSwap (backup+rename+restore) + privilegeCommand + backupPath + CleanupOldBinary(no-op)
//   swap_windows.go(Windows): platformSwap (.old dance+restore) + privilegeCommand + backupPath + CleanupOldBinary
func Swap(ctx context.Context, newBinPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	currentExe, err := resolveCurrentExe()
	if err != nil {
		return fmt.Errorf("upgrade: resolve current exe: %w", err)
	}
	if !isWritable(filepath.Dir(currentExe)) {
		return &NeedsPrivilegesError{Command: privilegeCommand(currentExe)} // FR-U4/U11: nothing changed
	}
	if err := platformSwap(currentExe, newBinPath); err != nil {
		return fmt.Errorf("upgrade: swap: %w", err)
	}
	if dir := filepath.Dir(newBinPath); isTempDir(dir) {
		_ = os.RemoveAll(dir) // best-effort tempDir cleanup (S2's contract); guarded by os.TempDir() prefix
	}
	return nil
}

// PATTERN (Unix platformSwap — backup THEN swap, restore on failure):
func platformSwap(currentExe, newBinPath string) error {
	backup := backupPath(currentExe)
	if err := os.Rename(currentExe, backup); err != nil { // one-deep: replaces prior backup (FR-U8)
		return fmt.Errorf("backup current binary: %w", err)
	}
	if err := os.Rename(newBinPath, currentExe); err != nil {
		_ = os.Rename(backup, currentExe) // FR-U11 restore (running process keeps the old inode safe)
		return fmt.Errorf("rename new binary into place: %w (restored from backup)", err)
	}
	return nil
}

// PATTERN (Windows platformSwap — the .old deferred-delete dance):
func platformSwap(currentExe, newBinPath string) error {
	old := backupPath(currentExe) // currentExe + ".old"
	if err := os.Rename(currentExe, old); err != nil { // Windows: renaming a running image is allowed
		return fmt.Errorf("rotate current binary to .old: %w", err)
	}
	if err := os.Rename(newBinPath, currentExe); err != nil {
		_ = os.Rename(old, currentExe) // FR-U11 restore (move the running image back to its path)
		return fmt.Errorf("move new binary into place: %w (restored from .old)", err)
	}
	return nil // the .old is cleaned at the NEXT launch (CleanupOldBinary); it's locked while this runs
}

// PATTERN (the injectable seam — clone of stage.go's execVersion):
var resolveCurrentExe = func() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("os.Executable: %w", err)
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		return real, nil
	}
	return exe, nil // tolerate EvalSymlinks failure (detect.go:352-354 idiom)
}
```

### Integration Points

```yaml
PRODUCTION (internal/upgrade/ — 3 new files):
  - swap.go:         func Swap(ctx, newBinPath string) error + ErrNeedsPrivileges + NeedsPrivilegesError + resolveCurrentExe + isWritable + isTempDir
  - swap_unix.go:    func platformSwap(currentExe, newBinPath) + func privilegeCommand(exe) + func backupPath(exe) + func CleanupOldBinary()
  - swap_windows.go: func platformSwap(currentExe, newBinPath) + func privilegeCommand(exe) + func backupPath(exe) + func CleanupOldBinary()
  - cmd/stagecoach/main.go: +upgrade.CleanupOldBinary() (1 line, after SetCurrentVersion)

IMPORTS (all stdlib — FR-U12):
  - swap.go: context, errors, fmt, os, path/filepath, strings
  - swap_unix.go: fmt, os, path/filepath
  - swap_windows.go: fmt, os, path/filepath
  - NO new go.mod requires. NO internal/* import (grep guard).

CONSUMERS (treat as contracts — they land AFTER this item; do NOT implement them here):
  - P1.M3.T1.S2 (parallel, Implementing): StageNewBinary → newBinPath (= tempDir/new-stagecoach, verified +
    sanity-run). Swap CONSUMES newBinPath; trusts it.
  - P1.M4.T2 runUpgrade: ResolveTarget → StageNewBinary → Swap; catches NeedsPrivilegesError (errors.As),
    prints .Command, exits 0 (FR-U4). Calls Swap with the command's ctx.
  - P1.M3.T3.S1 --rollback: reuses backupPath() + the same platformSwap+sanity discipline to restore the
    backup. (backupPath is the stable contract — define it once, in the twins.)

NO database / migration / routes / new types beyond NeedsPrivilegesError / new flag / config change /
exitcode mapping (FR-U4 printed-command = exit 0 = Success, command-layer's job) / docs.

SCOPE FENCES:
  - Touches ONLY internal/upgrade/{swap.go,swap_unix.go,swap_windows.go,swap_test.go,swap_windows_test.go} (new)
    + cmd/stagecoach/main.go (1 line).
  - Does NOT edit stage.go, detect.go, resolve.go, delegate.go, releases.go, download.go, version.go,
    exitcode.go, go.mod, or any PRD/task file.
  - Adds NO flag, NO config field, NO third-party dependency, NO exitcode constant (FR-U4's exit 0 reuses
    exitcode.Success).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native + cross-build (swap_unix.go's os.Rename + sudo compile on linux/darwin; swap_windows.go's .old
# dance + CleanupOldBinary compile on windows; the shared swap.go compiles everywhere).
go build ./...
GOOS=linux   go build ./...
GOOS=darwin  go build ./...
GOOS=windows go build ./...
# Expected: all clean. If GOOS=windows fails, swap_windows.go references a symbol not defined there (or
#           defines platformSwap/privilegeCommand/backupPath/CleanupOldBinary inconsistently with swap_unix.go).

# Vet (the build-tag twins vet on their respective OSes; run all-platform vet locally + a windows vet).
go vet ./internal/upgrade/... ./cmd/...
GOOS=windows go vet ./internal/upgrade/...
# Expected: clean.

# Format.
gofmt -l internal/upgrade/swap.go internal/upgrade/swap_unix.go internal/upgrade/swap_windows.go \
       internal/upgrade/swap_test.go internal/upgrade/swap_windows_test.go cmd/stagecoach/main.go
# Expected: empty. If listed: gofmt -w <those files>

# Lint.
make lint   # golangci-lint
# Expected: zero errors. (Swap/platformSwap/resolveCurrentExe all used; the unused-func linter won't fire on
#           the build-tag twins because each is compiled on its OS.)

# Scope guard: ONLY the 6 files changed.
git status --porcelain
# Expected: the 5 new internal/upgrade/ files + cmd/stagecoach/main.go. ZERO changes to stage.go/detect.go/
#           resolve.go/delegate.go/releases.go/download.go/exitcode.go/go.mod.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Unix upgrade tests (run on ubuntu-latest/macos-latest; the //go:build !windows file).
go test ./internal/upgrade/ -run 'Swap|NeedsPrivileges|IsWritable|PlatformSwap' -race -v
# Expected: ALL PASS.
#   - TestSwap_HappyPath: exe now "NEW", backup "OLD", tempDir cleaned.
#   - TestSwap_NotWritableNeedsPrivileges: *NeedsPrivilegesError, .Command has sudo+exe, exe unchanged (SKIP if root).
#   - TestSwap_OverwritesPriorBackup: backup now "OLD" (prior "ANCIENT" replaced — FR-U8).
#   - TestSwap_RestoresOnSwapFailure: exe restored to "OLD" (FR-U11) — or documented + grep-guarded.
#   - TestNeedsPrivilegesError_ErrorsIs: errors.Is true, .Command correct.
#   - TestIsWritable: writable→true, 0500→false (skip root), non-existent→false.

# Full upgrade-package regression (the new tests + all LANDED siblings: releases/download/detect/delegate/
# resolve/stage).
go test ./internal/upgrade/ -race
# Expected: green. (Swap has ZERO production callers — that's expected; the consumer is P1.M4.T2.)

# Full race suite (matrix incl. windows-latest runs swap_windows_test.go natively).
make test
# Expected: green on ubuntu-latest, macos-latest, AND windows-latest (the .old dance + CleanupOldBinary
#           tests run on windows-latest).
```

### Level 3: Integration Testing (System Validation)

```bash
# This item is the on-disk swap primitive; its end-to-end exercise (download→verify→sanity→backup→rename
# against a REAL running binary, asserting os.Executable() now reports the new version + the backup exists)
# is P1.M4.T3.S2 (the e2e self-update harness). The unit tests (Task 5/6) are the within-scope proof.
# Smoke (optional): build + a manual temp-dir swap on Unix.
make build
# (In a throwaway: copy bin/stagecoach to a temp "exe", create a "new" file, call Swap via a tiny main, or
#  defer to P1.M4.T3.S2's harness. The unit tests are the real proof; this is a sanity check.)

# Windows .old cleanup smoke (windows-latest): after a swap, relaunch stagecoach; assert the .old is gone.
# (Covered by P1.M4.T3.S2 e2e; the unit TestCleanupOldBinary_DeletesOldSibling is the within-scope proof.)
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: Swap exists with the contract signature + the proactive writability gate + the tempDir cleanup.
grep -n 'func Swap(ctx context.Context, newBinPath string) error' internal/upgrade/swap.go   # 1 hit
grep -n 'isWritable(filepath.Dir(currentExe))' internal/upgrade/swap.go                       # 1 hit (BEFORE platformSwap)
grep -n 'platformSwap(currentExe, newBinPath)' internal/upgrade/swap.go                       # 1 hit
grep -n 'os.RemoveAll' internal/upgrade/swap.go                                               # 1 hit (guarded by isTempDir)

# Guard 2: FR-U11 — both platform impls RESTORE on a mid-swap rename failure.
grep -n 'os.Rename(backup, currentExe)\|os.Rename(old, currentExe)' internal/upgrade/swap_unix.go internal/upgrade/swap_windows.go
# Expect: 1 hit each (the restore call inside the swap-failure branch).

# Guard 3: build tags are the MODERN form, both twins define the SAME 4 funcs.
head -1 internal/upgrade/swap_unix.go      # //go:build !windows
head -1 internal/upgrade/swap_windows.go   # //go:build windows
grep -c '+build' internal/upgrade/swap_unix.go internal/upgrade/swap_windows.go   # 0 (no legacy +build)
for fn in platformSwap privilegeCommand backupPath CleanupOldBinary; do
  echo "== $fn =="; grep -l "func $fn" internal/upgrade/swap_unix.go internal/upgrade/swap_windows.go
done   # each fn appears in BOTH files.

# Guard 4: NeedsPrivilegesError is typed + Unwraps to the sentinel.
grep -n 'type NeedsPrivilegesError struct' internal/upgrade/swap.go
grep -n 'func (e \*NeedsPrivilegesError) Unwrap() error { return ErrNeedsPrivileges }' internal/upgrade/swap.go

# Guard 5: the resolveCurrentExe seam exists (injectable, like stage.go's execVersion).
grep -n 'var resolveCurrentExe = func() (string, error)' internal/upgrade/swap.go

# Guard 6: FR-U12 — NO internal/* import in the new upgrade files.
grep -E 'dabstractor/stagecoach/internal' internal/upgrade/swap.go internal/upgrade/swap_unix.go internal/upgrade/swap_windows.go
# Expect: ZERO hits.

# Guard 7: main.go calls CleanupOldBinary exactly once, after SetCurrentVersion.
grep -n 'upgrade.CleanupOldBinary()' cmd/stagecoach/main.go   # 1 hit, on/after the SetCurrentVersion line.

# Guard 8: backupPath is a shared helper (not an inline literal) — the rollback contract.
grep -n 'func backupPath' internal/upgrade/swap_unix.go internal/upgrade/swap_windows.go   # 1 hit each.

# Guard 9: scope — only the 6 files.
git status --porcelain
# Expect: internal/upgrade/swap.go, swap_unix.go, swap_windows.go, swap_test.go, swap_windows_test.go + cmd/stagecoach/main.go.
git diff --name-only | grep -E 'stage\.go|detect\.go|resolve\.go|delegate\.go|releases\.go|download\.go|exitcode\.go|go\.mod' && echo "FAIL: out-of-scope file edited" || echo "OK: scope clean"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `GOOS=linux` + `GOOS=darwin` + `GOOS=windows` all clean
- [ ] `go vet ./internal/upgrade/... ./cmd/...` clean (incl. `GOOS=windows go vet ./internal/upgrade/...`)
- [ ] `gofmt -l` empty on the 6 files
- [ ] `make lint` zero errors
- [ ] `go test ./internal/upgrade/ -race` green (Unix tests + all LANDED siblings)
- [ ] `make test` green on the FULL matrix (ubuntu-latest, macos-latest, **windows-latest** runs swap_windows_test.go)

### Feature Validation
- [ ] `Swap(ctx, newBinPath)` resolves the exe (seam), proactively checks writability, and on success backs
      up + atomically renames + cleans tempDir (grep guards 1)
- [ ] Unix platformSwap: backup `os.Rename` then swap `os.Rename`, restore on failure (grep guard 2)
- [ ] Windows platformSwap: `.old` rotate then move-new, restore on failure (grep guard 2); `.old` cleaned
      at next launch via CleanupOldBinary
- [ ] Non-writable → `*NeedsPrivilegesError` with `.Command`, `errors.Is(ErrNeedsPrivileges)` true, nothing
      changed (FR-U4/U11); Swap NEVER os.Exit
- [ ] `NeedsPrivilegesError` typed + Unwrap→sentinel (grep guard 4); `resolveCurrentExe` seam (grep guard 5)
- [ ] `backupPath` is a shared helper in both twins (grep guard 8 — the P1.M3.T3.S1 rollback contract)
- [ ] main.go calls `CleanupOldBinary()` once at startup (grep guard 7)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY the 5 new internal/upgrade/ files + cmd/stagecoach/main.go
- [ ] NO edit to stage.go, detect.go, resolve.go, delegate.go, releases.go, download.go, version.go,
      exitcode.go, go.mod, or any PRD/task file (grep guard 9)
- [ ] NO `internal/*` import in swap.go/swap_unix.go/swap_windows.go (FR-U12; grep guard 6)
- [ ] NO new flag, NO config field, NO exitcode constant, NO third-party dependency (go.mod unchanged)
- [ ] ZERO production callers of Swap (consumer is P1.M4.T2 runUpgrade) — expected

### Code Quality & Docs
- [ ] Build tags modern form (`//go:build !windows` / `//go:build windows`), blank-line-before-package, no legacy `// +build`
- [ ] [Mode A] Godoc on `platformSwap` (both twins): Unix atomicity / Windows `.old` lock constraint + the
      FR-U11 restore + the FR-U7 `.old`-at-next-launch cleanup
- [ ] `swap_test.go` is NOT `t.Parallel()` (mutates the `resolveCurrentExe` seam); the not-writable test
      skips if `os.Geteuid()==0`
- [ ] Contract honored: Swap returns NeedsPrivilegesError (never os.Exit); the command layer prints + exits 0

---

## Anti-Patterns to Avoid

- ❌ Don't skip the proactive writability check. FR-U11 demands the installed binary be byte-for-byte
  unchanged when the dir isn't writable — detect it with `isWritable` BEFORE any rename and return
  NeedsPrivilegesError. Diving into backup+rename against a root-owned dir gives a confusing mid-flow
  EACCES instead of the clean sudo-print path.
- ❌ Don't omit the restore-on-failure. A mid-swap rename failure (rare, but possible — a file locks between
  the probe and the rename) MUST restore from the backup so there's never zero runnable binaries (FR-U11).
  Unix: `os.Rename(backup, current)`; Windows: `os.Rename(old, current)`. Wrap both errors if restore fails.
- ❌ Don't auto-elevate. FR-U4 forbids auto-`sudo`/UAC by a tool that writes binaries. Swap RETURNS
  NeedsPrivilegesError (never os.Exit, never runs sudo); the command layer prints `.Command` + exits 0.
- ❌ Don't use a one-shot `sudo install <newBinPath> <exe>` command. newBinPath is in an EPHEMERAL
  per-invocation tempDir (os.MkdirTemp) that may be gone when the user runs the command. Use FR-U7's "re-run"
  form: `sudo "<exe>" upgrade` (re-runs the whole pipeline elevated — robust, self-contained).
- ❌ Don't make Swap a struct-method or add a tempDir parameter. The contract pins `Swap(ctx, newBinPath)`.
  Derive tempDir = `filepath.Dir(newBinPath)`; use the package-level `resolveCurrentExe` seam (clone of
  stage.go's `execVersion`) for exe-path injectability — NOT a struct with fields.
- ❌ Don't forget the run-as-root test skip. The not-writable test (chmod 0500) is meaningless as root (root
  bypasses permission bits). Guard with `if os.Geteuid()==0 { t.Skip(…) }`. Restore dir perms in t.Cleanup.
- ❌ Don't call `t.Parallel()` in swap_test.go. It mutates the package-level `resolveCurrentExe` seam; two
  concurrent tests would race on it (exactly stage.go's documented constraint for `execVersion`).
- ❌ Don't delete the `.old` in Windows platformSwap. It's LOCKED while this process runs (it IS this
  process's image, renamed). CleanupOldBinary (called at the NEXT launch) deletes a prior `.old`. Deleting
  it mid-swap would fail (locked) and serves no purpose.
- ❌ Don't nuke an arbitrary dir on tempDir cleanup. Guard `os.RemoveAll(filepath.Dir(newBinPath))` with an
  `os.TempDir()` prefix check (`isTempDir`). S2's tempDir is always os.MkdirTemp (matches); the guard
  prevents a footgun if a caller ever passes a newBinPath outside temp. On failure, NEVER clean (leave for
  inspection per S2's contract).
- ❌ Don't inline the backup suffix as a literal in platformSwap. `backupPath` must be a SHARED helper (in
  the twins — suffix differs: `.stagecoach-backup` vs `.old`) because P1.M3.T3.S1 (--rollback) restores the
  backup and must use the SAME name. Re-deriving the suffix inline in rollback would desync.
- ❌ Don't import `internal/*` in the new upgrade files. FR-U12: upgrade is stdlib-only (os, path/filepath,
  errors, fmt, context, strings). main.go may import upgrade (one-directional wall); upgrade never imports
  internal/*. (grep guard 6.)
- ❌ Don't add an exitcode constant for NeedsPrivileges. FR-U4's printed-command exits 0 = `exitcode.Success`
  (already exists). Swap RETURNS the error; the command layer catches it, prints, exits 0. No exitcode.go
  edit. (Scope fence.)
- ❌ Don't restructure main.go beyond the one line. Add `upgrade.CleanupOldBinary()` after SetCurrentVersion,
  before cmd.Execute. No build tag in main.go (the platform logic lives in the upgrade twin). Preserve the
  existing signal.Install / Execute / exitcode.For / os.Exit flow.
- ❌ Don't duplicate S2's work. Swap TRUSTS newBinPath (already SHA256-verified + sanity-run by
  StageNewBinary). Swap does NOT re-download, re-verify, re-extract, or re-sanity-run. Its job is backup +
  atomic rename + tempDir cleanup — nothing more. Re-verifying would conflict with S2's "leave tempDir for
  inspection on failure" contract.