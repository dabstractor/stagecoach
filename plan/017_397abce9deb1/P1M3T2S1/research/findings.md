# Research Findings — P1.M3.T2.S1: Backup + atomic swap (Unix os.Rename / Windows .old dance) + non-writable→sudo

## 0. The authoritative spec — PRD §9.29 FR-U4/U5/U7/U8/U11/U12

The swap mechanics are pinned by the PRD (quoted verbatim during research):

- **FR-U5 step 7 (Backup)**: "move the current binary to `<exe>.stagecoach-backup` (one-deep; a prior
  backup is overwritten)."
- **FR-U7 (Atomic swap, per-platform)**:
  - **Unix (linux/darwin)**: "`os.Rename(newTemp, currentExe)`. Renaming over a running executable is
    safe — the running process keeps the old inode open; the path now resolves to the new file; the next
    invocation runs the new version. Atomic at the syscall level. If the target path is not writable
    (e.g. `/usr/local/bin` owned by root), detect it, leave everything untouched, and print the exact
    `sudo` command to re-run (never auto-elevate — FR-U4)."
  - **Windows**: "the running `stagecoach.exe` is LOCKED. Use the deferred-delete dance: rename the
    running `stagecoach.exe` → `stagecoach.exe.old` (Windows permits renaming a running image but not
    overwriting it in place), move the new file → `stagecoach.exe`, and clean up `*.old` at next launch
    (stagecoach deletes any `stagecoach.exe.old` sibling of its own exe at startup, best-effort). Because
    the new binary already passed the FR-U5 step-6 sanity run, the window during which both files coexist
    is a clean two-file state, not a corrupt one — there is never zero runnable binaries."
- **FR-U4 (never silently elevate)**: "stagecoach NEVER auto-`sudo`/auto-elevates … it prints the exact
  command for the user to run. A printed command exits 0 ('here is how to update')."
- **FR-U8 (--rollback)**: restores the most recent backup via the same atomic-swap + sanity-run discipline.
  (This item does NOT implement rollback — that's P1.M3.T3.S1 — but the backup path it creates IS the
  rollback source, so the `<exe>.stagecoach-backup` naming must be stable.)
- **FR-U11 (partial-state safety)**: "the swap is atomic by construction … a network/checksum/sanity
  failure aborts BEFORE the backup/swap and leaves the installed binary byte-for-byte unchanged. There is
  no 'half-upgraded' state." → my Swap must DETECT non-writable BEFORE attempting any rename, and on a
  rename FAILURE must RESTORE from the backup (never leave zero runnable binaries on Windows; on Unix the
  pre-rename backup-then-swap order means a swap failure leaves the backup in place → restore it).
- **FR-U12 (walled off)**: upgrade imports NO `internal/*` package — stdlib-only. Confirmed: `grep
  dabstractor/stagecoach/internal internal/upgrade/*.go` → ZERO hits. My new files must stay stdlib-only
  (os, os/path/filepath, errors, fmt, context, strings — all stdlib).

## 1. The parallel handoff — P1.M3.T1.S2 (Implementing) defines what Swap consumes

P1.M3.T1.S2's PRP (read in full) specifies the EXACT contract my Swap builds on:
- `StageNewBinary(ctx, c, release, asset, tempDir) (newBinPath string, err error)` returns
  `newBinPath = tempDir/new-stagecoach` (+`.exe` on Windows). The new binary is ALREADY downloaded,
  SHA256-verified, extracted, and sanity-run (`--version` matches release.Tag, exit 0) BEFORE Swap is
  called. **Swap trusts newBinPath is good** (it does NOT re-verify).
- "The tempDir is cleaned by **P1.M3.T2** on the success path; on failure it is left for the user/dev to
  inspect." → MY Swap owns success-path tempDir cleanup (derive `tempDir = filepath.Dir(newBinPath)`).
- StageNewBinary "NEVER touches the running binary (no rename/move/chmod of stagecoach — that is P1.M3.T2)."
  → MY Swap owns ALL on-disk mutation of the running binary.
- stage.go is LANDED-or-landing; it reserved `swap.go` for THIS task ("NOT swap.go — S1's PRP reserved
  swap.go for P1.M3.T2.S1"). → I create swap.go (shared) + swap_unix.go + swap_windows.go. No file-name
  collision with stage.go.

## 2. The design — shared orchestrator + build-tagged platform twins

Three new files + one main.go line. The split mirrors the existing build-tag twins in the repo
(`internal/signal/signal_{unix,windows}.go`, `internal/generate/editor_run{,_windows}.go`):

### internal/upgrade/swap.go (SHARED — no build tag)
- `var ErrNeedsPrivileges = errors.New("upgrade: install path not writable; re-run with privileges (FR-U4)")`
- `type NeedsPrivilegesError struct { Command string }` with `Error()` + `Unwrap() error { return
  ErrNeedsPrivileges }` — so the command layer (P1.M4.T2) detects via `errors.Is(err, ErrNeedsPrivileges)`
  AND type-asserts to read `.Command` to print. Mirrors `generate.CASError`'s typed-error-with-Unwrap
  precedent.
- `var resolveCurrentExe = func() (string, error) { … os.Executable() + filepath.EvalSymlinks … }` — the
  INJECTABLE seam (CLONE stage.go's `var execVersion` idiom: package-level var, tests override, NOT
  parallel). This is how tests point Swap at a temp-dir "exe" instead of the real running binary
  (you cannot reliably os.Rename over a running binary in a unit test, and you cannot test /usr/local/bin
  non-writability portably).
- `func Swap(ctx, newBinPath string) error` — the orchestrator:
  1. `currentExe, err := resolveCurrentExe()` (the seam).
  2. `dir := filepath.Dir(currentExe)`; `if !isWritable(dir) { return &NeedsPrivilegesError{Command:
     privilegeCommand(currentExe)} }` — the FR-U11 proactive gate (detect BEFORE any rename).
  3. `if err := platformSwap(currentExe, newBinPath); err != nil { return fmt.Errorf("upgrade: swap: %w",
     err) }` — the build-tagged impl.
  4. success: best-effort `os.RemoveAll(filepath.Dir(newBinPath))` GUARDED by
     `strings.HasPrefix(abs, filepath.Join(os.TempDir()))` (only clean if it's a real tempDir — never
     nuke an arbitrary dir; the parallel PRP's tempDir is os.MkdirTemp so this always matches in prod).
- shared helpers: `isWritable(dir)` (create+remove a `.stagecoach-writetest-*` temp file via
  `os.CreateTemp`; cross-platform; false on EACCES/EPERM), `absTempDir(path)` (the guard).
- `ctx` is in the signature (contract) but the ops are synchronous fs calls; reference it once at the top
  (`if err := ctx.Err(); err != nil { return err }`) so it's not an unused param + respects cancellation.

### internal/upgrade/swap_unix.go (`//go:build !windows`)
- `func platformSwap(currentExe, newBinPath string) error`:
  1. `backup := currentExe + ".stagecoach-backup"` (FR-U5 step 7; FR-U8's rollback source — stable name).
  2. `if err := os.Rename(currentExe, backup); err != nil { return … }` — os.Rename REPLACES an existing
     file on Unix (one-deep backup overwrite, FR-U8). The running process keeps the old inode open (safe).
  3. `if err := os.Rename(newBinPath, currentExe); err != nil {` → FR-U11 RESTORE: `_ =
     os.Rename(backup, currentExe)` (best-effort; if restore fails, wrap both errors); `return … }`.
  4. return nil.
- `func privilegeCommand(exe string) string` → `sudo "<exe>" upgrade` (FR-U7 "print the exact sudo command
  TO RE-RUN" — re-running elevated re-does detect→resolve→download→verify→sanity→swap AS ROOT; robust
  because it doesn't depend on the ephemeral tempDir surviving).
- `func CleanupOldBinary() {}` — NO-OP on Unix (the .old dance is Windows-only). Exported; main.go calls
  it unconditionally; the build-tag twin makes it a no-op here.

### internal/upgrade/swap_windows.go (`//go:build windows`)
- `func platformSwap(currentExe, newBinPath string) error`:
  1. `old := currentExe + ".old"`.
  2. `if err := os.Rename(currentExe, old); err != nil { return … }` — Windows PERMITS renaming a running
     image (not overwriting). os.Rename on Windows uses MoveFileEx(MOVEFILE_REPLACE_EXISTING) so a prior
     `.old` is replaced (one-deep).
  3. `if err := os.Rename(newBinPath, currentExe); err != nil {` → FR-U11 RESTORE: `_ = os.Rename(old,
     currentExe)` (the running image was moved aside; move it back); `return … }`.
  4. return nil. (The `.old` is NOT deleted here — it's locked while the process runs; CleanupOldBinary
     deletes it at the NEXT launch, FR-U7.)
- `func privilegeCommand(exe string) string` → a Windows elevation hint (the .old dance works for per-user
  installs; system-wide C:\Program Files installs are rarer — carry a clear message; the command layer
  may refine). Per the contract's Unix-sudo focus, keep this minimal but non-empty.
- `func CleanupOldBinary()` — best-effort: resolve own exe via os.Executable()+EvalSymlinks, `os.Remove(
  exe + ".old")` (ignore error — it's locked this launch if it just rotated; the NEXT launch cleans it).
  FR-U7's "delete any stagecoach.exe.old sibling of its own exe at startup."

### cmd/stagecoach/main.go — ONE new line at startup
After `upgrade.SetCurrentVersion(version)` (line 60), add `upgrade.CleanupOldBinary()` (Windows: deletes a
prior-launch `.old`; Unix: no-op via the build-tag twin). This keeps main.go cross-platform (no build tag
in cmd/stagecoach — the platform logic lives in the upgrade build-tag twins, FR-U12). Confirmed: there is
NO `_windows.go` in cmd/stagecoach today; adding the build tag THERE is unnecessary — the upgrade twin is
cleaner.

## 3. Conventions to follow (all verified in the codebase)

- **Build-tag twins**: `//go:build !windows` / `//go:build windows`, BLANK LINE, `package upgrade`. NO
  legacy `// +build` (Go 1.22; matches signal_unix.go:1 / editor_run_windows.go:1). Shared code (Swap,
  sentinels, seam) lives in the UNTAGGED swap.go; `platformSwap`/`privilegeCommand`/`CleanupOldBinary`
  are defined in BOTH twins (one per build) — the standard Go split (procgroup, signal).
- **Error sentinels**: `errors.New("upgrade: <description>")` — every upgrade sentinel follows this
  (ErrChecksumMismatch, ErrSanityRunFailed, …). Wrap with `%w` at use sites so `errors.Is` reaches them.
- **Typed error with Unwrap**: `generate.CASError{TreeSHA, Expected, Actual, …}` with `Unwrap()` returning
  a sentinel is the precedent for `NeedsPrivilegesError` (detect via errors.Is AND extract fields via
  type-assert).
- **Injectable seam**: `var execVersion = func(…) { … }` in stage.go (package-level var, tests override,
  the test file is NOT parallel). CLONE this for `var resolveCurrentExe = func() (string, error) { … }`.
  The swap test file must NOT be parallel either (it mutates the seam).
- **Exe-path resolution**: `filepath.EvalSymlinks` then fall back to the raw path on error — detect.go:353
  (`real, err := filepath.EvalSymlinks(d.ExePath); if err != nil { real = d.ExePath }`). Mirror this
  tolerance in resolveCurrentExe (a test "exe" that's a regular temp file EvalSymlinks fine; but tolerate).
- **FR-U12 stdlib-only**: swap.go/swap_unix.go/swap_windows.go import ONLY stdlib (os, path/filepath,
  errors, fmt, context, strings). NO `internal/*`. (main.go already imports upgrade — that's fine; the
  wall is one-directional: upgrade never imports internal/*.)

## 4. The "run as root" test gotcha (CRITICAL for the non-writable test)

The non-writable test (chmod the exe's dir 0500 → NeedsPrivilegesError) is MEANINGLESS when the test runs
as root: root bypasses Unix permission bits, so os.Rename into a 0500 dir SUCCEEDS as root, and the
isWritable probe (CreateTemp) also succeeds. CI ubuntu-latest/macos-latest run as a normal user
(`runner`), so the test works there; but some self-hosted/dev runs are root. The standard guard (used
across Go CLIs):
```go
if os.Geteuid() == 0 {
    t.Skip("permission test is meaningless when running as root")
}
```
Include this at the top of TestSwap_NotWritableNeedsPrivileges. (Windows has no euid; the _windows test
file doesn't need it.) Also: restore the dir perms in the test (t.Cleanup chmod back to 0700) so
t.TempDir's RemoveAll doesn't fail on a 0500 dir.

## 5. The sudo command form — FR-U7 "to re-run"

FR-U7 Unix says "print the exact `sudo` command **to re-run**". The robust reading: re-run the WHOLE
upgrade elevated — `sudo "<currentExe>" upgrade`. Why not `sudo install -m 0755 <newBinPath> <currentExe>`?
Because newBinPath is in an EPHEMERAL per-invocation tempDir (os.MkdirTemp) that may be gone by the time
the user runs the command; re-running `sudo stagecoach upgrade` re-does detect→resolve→download→verify→
sanity→swap AS ROOT (the dir is now writable), which is safe and self-contained. The command layer
(P1.M4.T2) prints `err.Command` verbatim and exits 0 (FR-U4). NeedsPrivilegesError.Command =
`sudo "<currentExe>" upgrade`.

## 6. FR-U11 restore-on-fail (never leave zero runnable binaries)

Both platform impls MUST restore on a mid-swap failure so the repo is never left with no runnable binary:
- Unix: backup FIRST (os.Rename current→backup), THEN swap (os.Rename new→current). If the swap fails,
  restore (os.Rename backup→current). Worst case after a failed swap: the original binary is back in
  place; newBinPath is untouched (still in tempDir for inspection). NEVER delete newBinPath or backup on
  failure.
- Windows: move running→.old FIRST, THEN move new→currentExe. If the second move fails, restore
  (os.Rename .old→currentExe) so the running binary's path resolves again. The .old is left for the next
  launch's CleanupOldBinary if the restore also fails (best-effort; both errors wrapped).

The proactive isWritable gate (step 2 of Swap) catches the COMMON non-writable case BEFORE any rename, so
FR-U11's "byte-for-byte unchanged" holds trivially (nothing was attempted). The restore-on-fail handles
the RARE case where the probe passed but the actual rename failed (e.g. a file became locked between the
probe and the rename).

## 7. Tests — Unix (swap_test.go `//go:build !windows`) + Windows (swap_windows_test.go `//go:build windows`)

CI matrix (`.github/workflows/ci.yml:47`): `os: [ubuntu-latest, macos-latest, windows-latest]` — so
windows-latest runs the _windows test file NATIVELY (no cross-compile-only gap). Coverage:

Unix (`internal/upgrade/swap_test.go`, `//go:build !windows`, NOT parallel — mutates resolveCurrentExe):
- **TestSwap_HappyPath**: resolveCurrentExe seam → a temp-dir "exe" (a regular file with "OLD" content);
  newBinPath = a temp file with "NEW" content; Swap(nil-ish ctx) → assert current exe now reads "NEW",
  `<exe>.stagecoach-backup` reads "OLD", tempDir (filepath.Dir(newBinPath)) removed.
- **TestSwap_NotWritableNeedsPrivileges**: chmod the exe's DIR 0500 (skip if `os.Geteuid()==0`); Swap →
  returns `*NeedsPrivilegesError`, `errors.Is(err, ErrNeedsPrivileges)` true, `.Command` contains "sudo"
  and the exe path, NOTHING on disk changed (exe + backup untouched). t.Cleanup restores 0700.
- **TestSwap_OverwritesPriorBackup**: pre-create `<exe>.stagecoach-backup` with "ANCIENT"; Swap → after,
  the backup reads "OLD" (the prior "ANCIENT" backup was replaced — one-deep, FR-U8).
- **TestSwap_RestoresOnSwapFailure**: make newBinPath's RENAME onto currentExe fail (e.g. newBinPath in a
  dir that becomes unwritable mid-test, OR mock platformSwap via a seam) → assert currentExe content
  UNCHANGED (restored from backup). (Hardest to make deterministic without a platformSwap seam; if flaky,
  document the restore logic with a code-path comment + rely on the grep guard that the restore call
  exists.)
- **TestNeedsPrivilegesError_ErrorsIs**: a `&NeedsPrivilegesError{Command:"sudo x"}` →
  `errors.Is(err, ErrNeedsPrivileges)` true (Unwrap chain); `.Command == "sudo x"`.
- **TestIsWritable**: a writable temp dir → true; a 0500 dir (skip if root) → false; a non-existent dir →
  false.

Windows (`internal/upgrade/swap_windows_test.go`, `//go:build windows`):
- **TestPlatformSwap_OldDance**: temp "exe" + newBinPath; platformSwap → currentExe now "NEW", `.old`
  holds "OLD". (The running-image lock is a kernel property; testing on regular files proves the rename
  sequence. On windows-latest CI this compiles + runs.)
- **TestCleanupOldBinary_DeletesOldSibling**: create `<exe>.old`; CleanupOldBinary (with resolveCurrentExe
  seam → that exe) → .old gone. (And the .old from the CURRENT process's own exe is locked this launch →
  CleanupOldBinary tolerates the Remove error; test with a SEPARATE temp exe via the seam.)
- AT MINIMUM the _windows file compiles on windows-latest and the rename-dance logic is exercised. If a
  real running-exe lock is impractical in-test, the regular-file rename-sequence test + the compile on
  windows-latest is the accepted bar (per the contract: "unit-test the rename sequence with mockable
  paths if a real running-exe lock is impractical in-test; at minimum the _windows file compiles and the
  logic is covered on windows-latest").

## 8. Scope boundaries (no overlap)

- **P1.M3.T1.S2** (parallel, Implementing) — stage.go: StageNewBinary (download+verify+extract+sanity).
  Produces newBinPath; NEVER touches the running binary. MY Swap consumes newBinPath + does all on-disk
  mutation. Different file (stage.go vs swap*.go); no overlap.
- **P1.M3.T3.S1** (planned) — --rollback: restores the backup my Swap creates. It REUSES the
  `<exe>.stagecoach-backup` / `.exe.old` names + the same platformSwap+sanity discipline. So my backup
  naming must be STABLE (a const/helper both this item and rollback use — note it for P1.M3.T3.S1).
- **P1.M4.T2** (planned) — runUpgrade orchestrator: calls ResolveTarget → StageNewBinary → Swap; catches
  NeedsPrivilegesError, prints `.Command`, exits 0 (FR-U4). MY Swap defines NeedsPrivilegesError +
  returns it; the command layer consumes it. Swap has ZERO production callers after this subtask
  (expected — consumer is P1.M4.T2).
- **main.go**: ONE new line (`upgrade.CleanupOldBinary()`). No other main.go change.
- This item touches ONLY: internal/upgrade/swap.go (new) + swap_unix.go (new) + swap_windows.go (new) +
  swap_test.go (new) + swap_windows_test.go (new) + cmd/stagecoach/main.go (1 line). NO edit to
  stage.go, detect.go, resolve.go, delegate.go, releases.go, download.go, exitcode.go, or any PRD/task file.

## 9. Validation commands (verified)

```bash
go build ./...                          # all 3 new files compile + link
GOOS=linux   go build ./...             # swap_unix.go's os.Rename + sudo compile
GOOS=darwin  go build ./...
GOOS=windows go build ./...             # swap_windows.go's .old dance + CleanupOldBinary compile
go vet ./internal/upgrade/... ./cmd/...
gofmt -l internal/upgrade/swap*.go internal/upgrade/swap_test.go internal/upgrade/swap_windows_test.go cmd/stagecoach/main.go   # empty
go test ./internal/upgrade/ -race -v    # Unix tests run on ubuntu/macos; the _windows file is skipped (build tag)
GOOS=windows go vet ./internal/upgrade/...   # the _windows file vets clean
make test                               # full matrix incl. windows-latest runs swap_windows_test.go natively
make lint
git status --porcelain                  # the 5 new files + main.go (1 line)
```

`internal/upgrade` is NOT in the coverage-gate list (Makefile gates `internal/{git,provider,generate,
config}` only) — no coverage-threshold pressure. (The grep for the gate confirmed this.)

## 10. The backup name is the rollback contract (forward-compat for P1.M3.T3.S1)

P1.M3.T3.S1 (--rollback) restores the backup my Swap creates. So the backup path derivation MUST be a
SHARED, STABLE helper (not an inline literal in platformSwap). Recommend a `backupPath(exe string) string`
in the shared swap.go returning `exe + ".stagecoach-backup"` (Unix) / `exe + ".old"` (Windows) — but since
the suffix differs by platform, put `backupPath` in the BUILD-TAGGED twins (Unix → `.stagecoach-backup`,
Windows → `.old`) so P1.M3.T3.S1 calls the SAME helper. Document this so rollback reuses it (don't
re-derive the suffix inline in P1.M3.T3.S1).