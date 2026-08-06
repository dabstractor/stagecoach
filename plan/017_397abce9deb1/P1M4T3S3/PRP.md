name: "P1.M4.T3.S3 — failure-safety + delegation-refuses-manager-owned + rollback (FR-U1/U8/U11)"
description: |
  The THIRD of P1.M4.T3's three test suites. A NEW package-cmd test file
  (internal/cmd/upgrade_safety_test.go, package `cmd`) that proves the FR-U1/U8/U11 SAFETY paths of
  `stagecoach upgrade` end-to-end through runUpgrade via Execute(ctx) — NO real network, NO real
  package-manager subprocess, NO rename of the real running test binary. It consumes the SAME package-cmd
  seams + S2's REUSABLE helpers (already LANDED on disk) and adds FIVE cases (each with
  exitcode.For(err) asserted):
    (a) tampered archive (wrong SHA256 in /checksums vs the served archive) → VerifySHA256 ⇒
        ErrChecksumMismatch → runDirectSwap errors; on-disk stub BYTE-FOR-BYTE UNCHANGED; NO backup
        created; a stagecoach-upgrade-* staging tempDir is LEFT (FR-U11 inspection); exit 1.
    (b) archive whose embedded stubversion prints the WRONG tag (v0.3.0 under a v0.2.0 release) →
        sanityCheck ⇒ ErrSanityVersionMismatch → same outcome as (a): unchanged, no backup, exit 1.
    (c) injected Detect→ChannelBrew (manager-owned), NO --force → runDelegate is taken; the injected
        upgradeExecRunner (a recordingExecRunner) RECORDS "brew upgrade stagecoach"; upgradeSwap is
        wired to a t.Fatal closure (PROVES the delegation path never swaps); the on-disk stub is
        UNTOUCHED; stdout has "stagecoach updated via brew"; exit 0.
    (d) --force WITH Detect→ChannelBrew → dispatchUpgrade prints the FR-U1 warning to stderr
        ("…--force overriding a detected brew install…; self-swapping") AND routes to runDirectSwap;
        the mini-swap runs (real swap happens); installed exe → v0.2.0, backup → v0.1.0; exit 0.
    (e) --rollback: with a backup present → upgradeRollback (mini-rollback closure) restores the
        backup over the temp installed exe → "restored stagecoach v0.1.0" + the installed exe now
        reports v0.1.0; with NO backup → upgradeRollback returns upgrade.ErrNoBackup →
        "no backup — nothing to roll back" + exit 0 (a no-op, never an error).
  REUSES S2's helpers (buildStubVersion/packSwapArchive/newSwapFake/runVersion/exeSuffix/
  backupSuffix/hostAssetName/hostEntryName/checksumsName/setupSwapSeams — all in upgrade_swap_test.go,
  same package) + cmd/stubversion (S2, LANDED — built a third time at v0.3.0 for case (b)). ADDS a
  recordingExecRunner (upgrade.ExecRunner fake), a parameterized runUpgradeArgs driver, and the
  miniSwap/failingSwap/miniRollback closures. go build/vet/gofmt/make test/make lint clean;
  `go test -race ./...` green on the ubuntu/macos/windows CI matrix. ZERO production edits.

---

## Goal

**Feature Goal**: Prove the FR-U1/U8/U11 `stagecoach upgrade` SAFETY contract end-to-end through
`runUpgrade` (LANDED in `internal/cmd/upgrade.go`/`upgrade_run.go`, P1.M4.T2.S1): a tampered-or-
broken payload ABORTS before the backup/swap leaving the on-disk binary byte-for-byte unchanged
with only a temp file behind (FR-U11); a detected manager-owned install is DELEGATED, never self-
swapped, unless `--force` (which warns) (FR-U1); and `--rollback` restores the prior backup (or is a
clean no-op when none exists) (FR-U8). All driven with the package-cmd seams + an httptest fake +
injected Detect/ExecRunner/Swap/Rollback — no real network, no real PM subprocess, no real-binary
rename.

**Deliverable**: ONE new file `internal/cmd/upgrade_safety_test.go` (package `cmd`) containing:
- a `recordingExecRunner` (implements `upgrade.ExecRunner`) that records argv + returns a canned
  `(code, err)` for the delegation case (c);
- a parameterized `runUpgradeArgs(t, args ...string)` Execute-driver (mirrors S2's `runUpgradeSwap`
  body, but takes args so it covers `["upgrade","--yes"]`, `["upgrade","--yes","--force"]`, and
  `["upgrade","--rollback"]`);
- focused closures `miniSwap(installedExe)`, `failingSwap(t)`, `miniRollback(installedExe)`;
- five test functions: `TestUpgradeFailure_TamperedArchive` (a),
  `TestUpgradeFailure_WrongVersionSanity` (b), `TestUpgradeDelegation_ManagerOwnedNoForce` (c),
  `TestUpgradeDelegation_ForceOverride` (d), `TestUpgradeRollback_*` (e1 no-backup + e2 backup-present,
  as table subtests or two funcs).

**Success Definition**:
- `go test ./internal/cmd/ -run "TestUpgradeFailure|TestUpgradeDelegation|TestUpgradeRollback" -race -v`
  PASSES all five cases.
- (a)/(b): `exitcode.For(err) == exitcode.Error (1)`; the installed exe's bytes are `bytes.Equal`
  before==after (unchanged); `os.Stat(installedExe+backupSuffix())` ⇒ `os.IsNotExist` (no backup);
  a `stagecoach-upgrade-*` tempDir appeared (FR-U11 inspection artifact left).
- (c): `exitcode.For(err) == exitcode.Success (0)`; `recorder.joinedCalls() == "brew upgrade stagecoach"`;
  stdout contains `"stagecoach updated via brew"`; the installed exe's bytes are unchanged; the
  `failingSwap` closure was NEVER invoked (no t.Fatal ⇒ the test reached its assertions).
- (d): `exitcode.For(err) == exitcode.Success (0)`; errBuf contains `"--force overriding a detected
  brew install"`; the installed exe's `--version` contains `v0.2.0` (swapped); the backup's
  `--version` contains `v0.1.0` (FR-U8 one-deep).
- (e1): `exitcode.For(err) == exitcode.Success (0)`; stdout contains `"no backup — nothing to roll back"`.
- (e2): `exitcode.For(err) == exitcode.Success (0)`; stdout contains `"restored stagecoach v0.1.0"`;
  the installed exe's `--version` contains `v0.1.0` (current became backup content).
- `go build ./...`, `go vet ./internal/cmd/...`, `gofmt -l`, `make test`, `make lint` all clean.
- `go test -race ./...` green on the CI matrix (ubuntu/macos/windows) — the suite is cross-platform
  (tar.gz/.stagecoach-backup vs zip/.old via `backupSuffix()`).
- `git status --porcelain` == exactly ONE new file (`internal/cmd/upgrade_safety_test.go`).

## User Persona (if applicable)

**Target User**: The Stagecoach maintainer / release engineer who must TRUST `stagecoach upgrade` to
(1) never leave a half-upgraded/bricked install — a bad download or a broken binary aborts before any
on-disk change; (2) never clobber a package-manager-owned binary without explicit consent; and (3)
offer a reliable one-step undo. This suite is the automated regression net for those three safety invariants.
**Use Case**: A user runs `stagecoach upgrade`; the suite proves a corrupt/sanity-failing payload is
rejected cleanly, a Homebrew install delegates to `brew` (not an overwrite), `--force` warns then
swaps, and `--rollback` restores (or no-ops cleanly).
**Pain Points Addressed**: FR-U1 (delegate-first; refuse to overwrite manager-owned unless `--force`
warns), FR-U8 (one-step rollback; clean no-op when no backup), FR-U11 (abort-before-swap; on-disk
byte-for-byte unchanged on failure; only a temp file left).

## Why

- **PRD §9.29 FR-U1/U8/U11 + §20.5 (line 2172)**: §20.5's self-update scenario list explicitly
  requires: "a corrupted/tampered asset (bad SHA256) and a new binary that exits non-zero on
  `--version` both abort BEFORE any swap with the on-disk binary unchanged (FR-U5 steps 4 & 6,
  FR-U11); `--rollback` restores the backup; install-method detection is asserted to delegate to
  (not overwrite) a simulated Homebrew/npm/Scoop install path, and to refuse a self-swap of a
  manager-owned binary unless `--force` (FR-U1)." These FIVE cases are that scenario's automated net.
- **P1.M4.T2.S1 (LANDED) shipped runUpgrade + dispatchUpgrade + the function seams specifically so
  this suite can run the real pipeline with no real network/PM/real-binary-rename.** The seam design
  (upgradeDetect + upgradeExecRunner + upgradeSwap + upgradeRollback) is the contract this suite
  consumes — it is the FR-U12 test wall's third customer (after S1's `--check` and S2's happy-path swap).
- **Risk being guarded**: a regression where (a) a checksum/sanity failure still swaps or leaves a
  backup (bricking the install), (b) a detected brew install gets self-overwritten silently (FR-U1
  violation — the v2.1 failure mode), or (c) `--rollback` exits non-zero / corrupts state when there
  is no backup. All caught here deterministically against the fake + injected runners.

## What

One new test file, no production edits, no new helper binary. The file reuses S2's helpers
(`buildStubVersion`/`packSwapArchive`/`newSwapFake`/`runVersion`/`exeSuffix`/`backupSuffix`/
`hostAssetName`/`hostEntryName`/`checksumsName`/`setupSwapSeams` — all in `upgrade_swap_test.go`, same
package `cmd`) and `cmd/stubversion` (S2, LANDED). It adds a `recordingExecRunner`, a parameterized
`runUpgradeArgs` driver, three closures (`miniSwap`/`failingSwap`/`miniRollback`), and five tests.

### Success Criteria
- [ ] NEW file `internal/cmd/upgrade_safety_test.go` (`package cmd`): a file doc comment names it the
      FR-U1/U8/U11 safety suite, lists the five cases, the seams overridden per case
      (upgradeBaseURL/upgradeDetect/upgradeExecRunner/upgradeSwap/upgradeRollback/upgrade.
      SetCurrentVersion), the no-network/no-PM/no-real-binary-rename guarantee, the S2-helper reuse,
      and the dedicated-file rationale (S1 owns upgrade_check_test.go; S2 owns upgrade_swap_test.go).
- [ ] `recordingExecRunner` struct implementing `upgrade.ExecRunner` (records calls, mutex-guarded,
      returns canned `(code, err)`); a `joinedCalls() string` accessor.
- [ ] `runUpgradeArgs(t, args ...string) (outBuf, errBuf *bytes.Buffer, err error)`: the Execute driver
      (mirrors S2's `runUpgradeSwap`: `saveRootState`/`restoreRootState` via t.Cleanup +
      `resetFlags(upgradeCmd.Flags())` + isolated HOME + outBuf/errBuf wired to rootCmd), but takes args.
- [ ] Five test functions covering (a)–(e) with the exact assertions in Success Definition.
- [ ] Each seam override restored via `t.Cleanup`; `upgrade.SetCurrentVersion("dev")` left as the final
      unwind (mirrors S1/S2 LIFO discipline).
- [ ] `go build ./...`, `go vet ./internal/cmd/...`, `gofmt -l`, `make test`, `make lint` clean.
- [ ] `git status --porcelain` == `internal/cmd/upgrade_safety_test.go` ONLY.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the LANDED dispatch logic (exact branch for `--rollback`/`--force`/delegate + the warning
Fprintf + the success/no-backup Fprintfs), the exact seam var names + types, the EXACT S2 helpers to
reuse (with signatures), the exact failure sentinels (VerifySHA256 ⇒ ErrChecksumMismatch;
sanityCheck ⇒ ErrSanityVersionMismatch) and WHERE they fire (inside StageNewBinary, before the swap),
the brew argv (`brew upgrade stagecoach`), the cross-platform backup suffix, the Execute-driver idiom
(with the mandatory `resetFlags(upgradeCmd.Flags())` gotcha), the wrong-version stub trick (build
stubversion at v0.3.0 under a v0.2.0 asset), and the precise per-case assertion matrix.

### Documentation & References

```yaml
# MUST READ — the dispatch + path helpers + the SEAMS (the code under test).
- file: internal/cmd/upgrade_run.go
  why: "runDirectSwap (LEAVE tempDir on ResolveTarget/StageNewBinary failure ⇒ FR-U11; NO defer cleanup),
        runDelegate (confirm RUN-only; Delegate with Exec=upgradeExecRunner; map Ran/ExitCode to exit 0/1),
        and the seam var declarations at the top (upgradeBaseURL/upgradeExePath/upgradeExecRunner/
        upgradeToken/upgradeNewClient/upgradeDetect/upgradeSwap/upgradeRollback). The file doc explains
        WHY upgradeSwap/upgradeRollback are FUNCTION seams (package cmd can't reach upgrade.Swap/Rollback's
        unexported resolveCurrentExe) and WHY upgradeExecRunner is a literal field seam (nil ⇒ osExecRunner
        prod — the OPPOSITE of Detector)."
  critical: "(a)/(b) fail INSIDE StageNewBinary (before confirmUpgrade/upgradeSwap) ⇒ runDirectSwap
             returns exitcode.New(exitcode.Error, …) ⇒ exit 1, tempDir LEFT. runDelegate builds
             DelegateOptions{Exec: upgradeExecRunner, Out: cmd.OutOrStdout(), Env: os.Getenv, …}."

- file: internal/cmd/upgrade.go
  why: "dispatchUpgrade is the dispatcher S3 exercises: flagRollback⇒upgradeRollback (errors.Is(err,
        ErrNoBackup)⇒'no backup …'+exit 0; err⇒exit 1; ok⇒'restored stagecoach %s'+exit 0);
        flagCheck⇒runCheck; else upgradeDetect ⇒ `if ch==ChannelDirect || flagForce { if flagForce &&
        ch!=ChannelDirect { Fprintln(ErrOrStderr(),'stagecoach: warning: --force overriding a detected
        '+string(ch)+' install (FR-U1); self-swapping') } ; runDirectSwap } else runDelegate`. Also
        validateUpgradeFlags (--rollback exclusive with --check/--version; NOT --yes ⇒ ['upgrade',
        '--rollback'] is valid)."
  critical: "the warning goes to cmd.ErrOrStderr() (errBuf); the success lines go to OutOrStdout()
             (outBuf). flagYes/flagForce/flagRollback are LOCAL to upgradeCmd.Flags() ⇒ resetFlags is
             mandatory. LoadUpgradeConfig needs isolated HOME (the driver handles it)."

# MUST READ — S2's REUSABLE helpers (the contract S3 builds on; ALREADY LANDED on disk).
- file: internal/cmd/upgrade_swap_test.go
  why: "THE source of every low-level helper S3 REUSES (same package cmd, directly callable):
        exeSuffix(), backupSuffix() ('.stagecoach-backup'/'.old'), hostAssetName(tag), hostEntryName(),
        checksumsName(tag), buildStubVersion(t, version) (cached), packSwapArchive(t, stubPath, tag)
        ([]byte, shaHex), newSwapFake(t, tag, archiveBytes, shaHex) *httptest.Server (3-route fake;
        ABSOLUTE asset URLs), runVersion(t, path) string, setupSwapSeams(t, tsURL, installedExe) (wires
        detect→direct + mini-swap + baseURL + SetCurrentVersion('0.1.0'); restores via t.Cleanup), and
        runUpgradeSwap(t) (the Execute driver S3's runUpgradeArgs mirrors)."
  pattern: "setupSwapSeams is REUSABLE AS-IS for cases (a)/(b) (the swap is never reached there, so its
            mini-swap is harmless). The packSwapArchive+newSwapFake pair is reused for (a) [wrong sha],
            (b) [wrong embedded version], and (d) [valid happy-path payload]."
  gotcha: "S3 does NOT redefine these (would be a redeclaration in the same package). S3 ADDS only
           recordingExecRunner/runUpgradeArgs/miniSwap/failingSwap/miniRollback + the 5 tests."

# MUST READ — S1's driver idiom (the Execute + state-restore pattern both S2 and S3 follow).
- file: internal/cmd/upgrade_check_test.go
  why: "setupCheckSeams/runUpgradeCheck establish the EXACT seam-overlay + Execute-driver idiom:
        upgradeBaseURL=httptest.URL; upgrade.SetCurrentVersion(v) + restore 'dev' via t.Cleanup (LIFO);
        saveRootState/restoreRootState + resetFlags(upgradeCmd.Flags()) + isolated HOME; drive
        Execute(ctx); map exit via exitcode.For. S3 mirrors runUpgradeCheck's body in runUpgradeArgs."
- file: internal/cmd/upgrade_test.go
  why: "TestUpgradeCommand_NoBootstrapOutsideRepo is the verbatim driver template (saveRootState/
        restoreRootState + resetFlags(upgradeCmd.Flags()) + rootCmd.SetOut/SetErr/SetArgs + Execute).
        resetFlags(upgradeCmd.Flags()) is SEPARATE from restoreRootState (which resets only rootCmd flags)."

# MUST READ — Delegate (the dispatcher case (c) exercises) + the brew argv + ExecRunner interface.
- file: internal/upgrade/delegate.go
  why: "Delegate(ctx, ch, DelegateOptions{Exec, Out, Env, Verbose, Confirmed}) (DelegateResult, error).
        ChannelBrew ⇒ runArgv ⇒ [['brew','upgrade','stagecoach']] ⇒ joinArgv ⇒ 'brew upgrade stagecoach'.
        ExecRunner interface: Run(ctx, stdout, stderr io.Writer, name string, args ...string) (int, error)
        — Delegate calls Run(ctx, out, out, 'brew', 'upgrade', 'stagecoach'). On (0,nil) ⇒
        DelegateResult{Ran:true, Command:'brew upgrade stagecoach', ExitCode:0} ⇒ runDelegate prints
        'stagecoach updated via brew' + exit 0. ChannelDirect ⇒ ErrDirectSwap (UNREACHABLE here —
        dispatchUpgrade routes Direct/--force to runDirectSwap BEFORE runDelegate)."
  critical: "a NON-ZERO process exit is (code, nil) — NOT a Delegate error (the updater ran). Only a
             start/LookPath failure ⇒ err != nil. S3's recorder returns (0,nil) for the happy delegate."

# MUST READ — Rollback (case (e)) — signature + sentinels.
- file: internal/upgrade/rollback.go
  why: "Rollback(ctx) (string, error) resolves the exe via the UNEXPORTED resolveCurrentExe (package cmd
        CANNOT steer it) ⇒ S3 overrides the upgradeRollback SEAM VAR. ErrNoBackup (command layer ⇒
        'no backup …' + exit 0); ErrBackupUnusable (⇒ exit 1); success ⇒ returns the trimmed --version
        (command layer ⇒ 'restored stagecoach %s'). dispatchUpgrade maps these."
  critical: "the REAL Rollback is exhaustively covered by rollback_test.go; S3 owns the runUpgrade
             --rollback DISPATCH path + the seam, NOT a re-test of Rollback's mechanics."

# MUST READ — StageNewBinary + the failure sentinels (where (a)/(b) fail).
- file: internal/upgrade/stage.go
  why: "StageNewBinary: FetchChecksums → sums[asset.Name] → DownloadFile → VerifySHA256 (bad sha ⇒
        ErrChecksumMismatch, propagated AS-IS) → extractBinary → sanityCheck(ctx, newBin, release.Tag)
        (execVersion runs '<newBin> --version' via the DEFAULT seam; output must CONTAIN release.Tag
        ⇒ wrong tag ⇒ ErrSanityVersionMismatch). runDirectSwap returns these as exitcode.New(Error, …)
        and LEAVES tempDir (FR-U11). stubversion bakes its version ⇒ the DEFAULT execVersion works
        (NO override needed)."
- file: internal/upgrade/download.go
  why: "VerifySHA256(path, want): wantNorm = strings.ToLower(strings.TrimSpace(want));
        subtle.ConstantTimeCompare(got, wantNorm). A valid-hex-but-wrong digest
        (strings.Repeat('a',64)) ⇒ ErrChecksumMismatch. (S3 serves the real archive + a wrong sha.)"

# MUST READ — the EXPORTED version seam + the Channel consts + exit codes.
- file: internal/upgrade/version.go
  why: "SetCurrentVersion(v) sets package var currentVersion ('dev' default). Pin per test; restore
        'dev' via t.Cleanup."
- file: internal/upgrade/detect.go
  why: "Channel consts: ChannelBrew='brew', ChannelDirect='direct', ChannelAUR='aur', ChannelNix='nix',
        ChannelScoop='scoop', etc. Case (c)/(d) inject ChannelBrew."
- file: internal/exitcode/exitcode.go
  why: "Success=0, Error=1, UpdateAvailable=6. For(err): nil→0; *ExitError→its Code (runDirectSwap's
        exitcode.New(Error, …) ⇒ 1)."
- file: internal/cmd/upgrade_prompt.go
  why: "confirmUpgrade(..., assumeYes=flagYes, ...): --yes⇒(true,nil) NO prompt. Called by runDirectSwap
        AND runDelegate (RUN channels). NOT called on the --rollback path ⇒ ['upgrade','--rollback']
        needs NO --yes. (Non-TTY + no --yes ⇒ refuse exit 1 — but every S3 case that reaches confirm
        passes --yes.)"

# MUST READ — the stubversion stub (S2, LANDED; S3 builds it a 3rd time at v0.3.0 for case (b)).
- file: cmd/stubversion/main.go
  why: "var version='dev'; main prints it. -ldflags '-X main.version=v0.3.0' bakes the WRONG tag for
        case (b): the archive is asset-named v0.2.0 but the embedded binary prints v0.3.0 ⇒ sanityCheck
        fails (output lacks release.Tag 'v0.2.0')."

# MUST READ — the FR-U1/U8/U11 spec + §20.5 scenario this suite automates.
- docfile: plan/017_397abce9deb1/prd_snapshot.md
  section: "§9.29 FR-U1 (delegate-first; refuse to overwrite manager-owned unless --force warns),
            FR-U8 (--rollback; no-op reported as such when no backup), FR-U11 (abort-before-swap; on-disk
            byte-for-byte unchanged; only a temp file left) + §20.5 line 2172 (the self-update scenario:
            tampered-asset/wrong-version abort before swap; delegate-to-not-overwrite a simulated PM;
            refuse self-swap unless --force; --rollback restores)"
  why: "the five cases map 1:1 to §20.5's enumerated self-update assertions."
- docfile: plan/017_397abce9deb1/P1M4T3S3/research/findings.md
  why: "§1 (S2 helpers reused + stubversion), §2 (exact seam types), §3 (dispatch logic per case),
        §4 (the failure sentinels + triggers), §5 (brew argv), §6 (case (c) needs no fake), §7 (the
        exit-code + on-disk matrix), §8 (verbatim spec)."
- file: plan/017_397abce9deb1/P1M4T3S2/PRP.md
  why: "S2 (the happy path) is the CONTRACT for the helpers S3 reuses + the seam-overlay discipline.
        Read §'Implementation Patterns' (the mini-swap closure, the fake server, the driver) — S3's
        miniSwap/runUpgradeArgs mirror them."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  upgrade.go              # LANDED — runUpgrade + dispatchUpgrade (the dispatch S3 exercises); READ-ONLY
  upgrade_run.go          # LANDED (P1.M4.T2.S1) — runDirectSwap/runDelegate + the seams; READ-ONLY
  upgrade_prompt.go       # LANDED — confirmUpgrade (flagYes⇒(true,nil)); printDelegatedUpdate; READ-ONLY
  upgrade_test.go         # LANDED — the Execute/saveRootState/resetFlags driver template; READ-ONLY
  upgrade_check_test.go   # S1 (Complete) — setupCheckSeams/runUpgradeCheck pattern; READ-ONLY (do NOT touch)
  upgrade_swap_test.go    # S2 (LANDED on disk) — REUSABLE helpers (buildStubVersion/packSwapArchive/
                          #   newSwapFake/runVersion/setupSwapSeams/runUpgradeSwap/exeSuffix/backupSuffix/
                          #   hostAssetName/hostEntryName/checksumsName); READ-ONLY (do NOT touch)
  root_test.go            # READ-ONLY — saveRootState/restoreRootState/resetFlags helpers (package cmd)
  root.go                 # READ-ONLY — Execute(ctx)
internal/upgrade/
  delegate.go             # READ-ONLY — Delegate/ExecRunner/runArgv(brew)/DelegateResult
  rollback.go             # READ-ONLY — Rollback/ErrNoBackup/ErrBackupUnusable (signature + sentinels)
  stage.go                # READ-ONLY — StageNewBinary/VerifySHA256-propagation/sanityCheck/sentinels
  download.go             # READ-ONLY — VerifySHA256 (wantNorm lowercased) / assetName / checksumsName
  version.go              # READ-ONLY — SetCurrentVersion (EXPORTED version seam)
  detect.go               # READ-ONLY — Channel consts (ChannelBrew/ChannelDirect/ChannelAUR/ChannelNix)
  swap.go / swap_unix.go / swap_windows.go  # READ-ONLY — backupPath (.stagecoach-backup / .old)
internal/exitcode/exitcode.go  # READ-ONLY — Success/Error/For
cmd/
  stubversion/main.go     # S2 (LANDED) — ldflags-baked version stub; S3 builds it at v0.3.0 for case (b)
  stagecoach/main.go      # READ-ONLY — reference for the ldflags pattern (S2 mirrored it for stubversion)
```

### Desired Codebase tree with files to be added

```bash
internal/cmd/upgrade_safety_test.go   # NEW — the FR-U1/U8/U11 safety suite (5 cases + helpers)
# NOTHING ELSE. cmd/stubversion/main.go ALREADY EXISTS (S2). ZERO edits to any production file,
# to S1's upgrade_check_test.go, or to S2's upgrade_swap_test.go.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (DO NOT redefine S2's helpers — same package cmd): buildStubVersion, packSwapArchive,
// newSwapFake, runVersion, exeSuffix, backupSuffix, hostAssetName, hostEntryName, checksumsName,
// setupSwapSeams, runUpgradeSwap, stubVerCache/stubVerMu ALL live in upgrade_swap_test.go. Redeclaring
// any in upgrade_safety_test.go is a compile error (redeclaration). REUSE them. S3 ADDS only
// recordingExecRunner, runUpgradeArgs, miniSwap, failingSwap, miniRollback, and the 5 tests.

// CRITICAL (cases (a)/(b) fail INSIDE StageNewBinary, BEFORE the swap): runDirectSwap does
// MkdirTemp → ResolveTarget → StageNewBinary (NO defer cleanup — LEAVE tempDir on failure, FR-U11) →
// confirmUpgrade → upgradeSwap. (a)/(b) return exitcode.New(exitcode.Error, …) from the StageNewBinary
// step ⇒ exit 1, tempDir LEFT, NO backup, on-disk unchanged. setupSwapSeams's mini-swap is NEVER
// invoked on these paths (harmless). REUSE setupSwapSeams for (a)/(b).

// CRITICAL (case (a) trigger — WRONG sha, not a wrong archive): VerifySHA256 lowercases+trims the want
// digest, then constant-time-compares. Serve the REAL archive (from packSwapArchive) but a WRONG sha in
// /checksums: newSwapFake(t, "v0.2.0", archive, strings.Repeat("a", 64)). (Using the real archive proves
// the failure is the SHA check, not a download/parse error.) A valid-hex wrong digest is required (64 hex
// chars) — a malformed digest could fail earlier in checksum parsing, not at VerifySHA256.

// CRITICAL (case (b) trigger — WRONG embedded version, asset still named v0.2.0): build stubversion at
// v0.3.0 (buildStubVersion(t,"v0.3.0")), pack it under the v0.2.0 ASSET name
// (packSwapArchive(t, v0.3.0stub, "v0.2.0") ⇒ real sha of the v0.3.0-bearing archive), serve with the
// MATCHING sha (newSwapFake(t,"v0.2.0",archive,sha)). VerifySHA256 PASSES (bytes match); sanityCheck
// runs '<newBin> --version' ⇒ "v0.3.0\n" ⇒ does NOT contain release.Tag "v0.2.0" ⇒ ErrSanityVersionMismatch.

// CRITICAL (case (c) needs NO httptest fake): runDelegate/Delegate NEVER touch the releases Client (only
// runCheck/runDirectSwap do). Leaving upgradeBaseURL at its "" default and running with NO fake server is
// the STRONGEST proof the delegate path doesn't network. Wire ONLY upgradeDetect→brew +
// upgradeExecRunner=recorder + upgradeSwap=failingSwap(t) + SetCurrentVersion.

// CRITICAL (case (c) PROVES no swap via failingSwap): wire upgradeSwap = failingSwap(t) — a closure that
// t.Fatal("upgradeSwap must not be called on the delegation path (FR-U1)"). If the dispatch (buggily)
// routed brew to runDirectSwap, the test fails loudly instead of renaming the test runner. The recorder
// (upgradeExecRunner) capturing "brew upgrade stagecoach" is the POSITIVE proof of delegation.

// CRITICAL (the ExecRunner interface signature — DO NOT get it wrong): upgrade.ExecRunner.Run is
// Run(ctx, stdout, stderr io.Writer, name string, args ...string) (int, error) — it STREAMS (takes
// writers), DISTINCT from detect.go's capturing Runner.Run(ctx, name, args...) (string, int, error).
// recordingExecRunner MUST implement the STREAMING signature or it won't satisfy upgrade.ExecRunner.

// CRITICAL (upgradeExecRunner is the Delegate seam; nil ⇒ osExecRunner prod): set
// upgradeExecRunner = &recordingExecRunner{code:0, err:nil} for case (c). runDelegate builds
// DelegateOptions{Exec: upgradeExecRunner, ...} and Delegate calls rec.Run(ctx, out, out, "brew",
// "upgrade", "stagecoach"). restore the original (nil) via t.Cleanup.

// CRITICAL (upgradeRollback is a FUNCTION seam — package cmd CANNOT reach upgrade.Rollback's exe):
// upgrade.Rollback resolves the exe via the package-upgrade UNEXPORTED resolveCurrentExe (returns the
// REAL test binary). Override the upgradeRollback SEAM VAR with miniRollback(installedExe) (backup-
// present) or a func returning upgrade.ErrNoBackup (no-backup). Do NOT call the real upgrade.Rollback.
// The real Rollback is covered by rollback_test.go.

// CRITICAL (--rollback needs NO --yes): dispatchUpgrade's flagRollback branch calls upgradeRollback
// DIRECTLY — no confirmUpgrade. So ["upgrade","--rollback"] is the correct args (validateUpgradeFlags
// allows --rollback + nothing else exclusive). Adding --yes is harmless but unnecessary.

// CRITICAL (resetFlags(upgradeCmd.Flags()) is SEPARATE from restoreRootState): restoreRootState resets
// rootCmd.Flags()+PersistentFlags() but NOT upgradeCmd's LOCAL flags. flagYes/flagForce/flagRollback/
// flagChannel leak across tests without `defer resetFlags(upgradeCmd.Flags())`. Mirror S2's two-defer
// idiom in runUpgradeArgs.

// CRITICAL (isolated HOME): runUpgrade calls config.LoadUpgradeConfig() which reads the GLOBAL config
// under $HOME/$XDG_CONFIG_HOME. Without isolation a real config's [upgrade].channel/source_repo could
// change effChannel/effSourceRepo. runUpgradeArgs isolates: t.Setenv("HOME", t.TempDir());
// t.Setenv("XDG_CONFIG_HOME", "") ⇒ Defaults (never bootstraps).

// GOTCHA (cross-platform backup SUFFIX): backupSuffix() = ".stagecoach-backup" (unix) / ".old" (windows).
// The "no backup created" assertion (a/b) AND the miniRollback/miniSwap MUST use backupSuffix() (NOT a
// hardcoded ".stagecoach-backup") or the windows-latest matrix job fails.

// GOTCHA (the "unchanged" assertion uses bytes.Equal, not --version): for (a)/(b)/(c) read the installed
// exe's bytes BEFORE runUpgradeArgs and bytes.Equal them AFTER. --version is insufficient (a tampered
// payload never reaches the installed exe, so its version is trivially unchanged — bytes.Equal is the
// byte-for-byte FR-U11 contract). For (d)/(e2) where the exe IS swapped/restored, use runVersion().

// GOTCHA (the stagecoach-upgrade-* tempDir is created by runDirectSwap, LEFT on failure): for (a)/(b)
// assert a NEW stagecoach-upgrade-* dir appeared after Execute (len(after) > len(before)) — the FR-U11
// inspection artifact. Glob os.TempDir()/"stagecoach-upgrade-*" before+after. (Cross-test accumulation
// is fine — compare deltas, not absolute counts.)

// GOTCHA (Go runs t.Cleanup LIFO): register seam restores BEFORE the per-call SetCurrentVersion where
// order matters; the "dev" reset registered in the helper runs last ⇒ final state "dev". Mirror S2's
// setupSwapSeams discipline.
```

## Implementation Blueprint

### Data models and structure

The file declares only test helpers (no production types):
- `recordingExecRunner` struct (implements `upgrade.ExecRunner`): `mu sync.Mutex; calls [][]string; code int; err error`.
  Methods: `Run(ctx, stdout, stderr, name, args...) (int, error)` (appends `append([]string{name}, args...)`,
  returns `r.code, r.err`); `joinedCalls() string` (space-joins each call, " && "-joins multi-step).
- `runUpgradeArgs(t, args ...string) (outBuf, errBuf *bytes.Buffer, err error)` — the parameterized Execute driver.
- `miniSwap(installedExe string) func(context.Context, string) error` — returns the backup→rename→cleanup
  closure (a faithful twin of S2's inline mini-swap; reused for case (d)).
- `failingSwap(t *testing.T) func(context.Context, string) error` — returns a closure that `t.Fatal`s
  (proves case (c) never swaps).
- `miniRollback(installedExe string) func(context.Context) (string, error)` — returns a closure that
  stats the backup (ErrNoBackup if absent), renames backup→installed, returns the trimmed `--version`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/cmd/upgrade_safety_test.go — file doc + imports + recordingExecRunner + runUpgradeArgs
  - PACKAGE DOC (file comment): "FR-U1/U8/U11 safety suite (P1.M4.T3.S3) — the THIRD of P1.M4.T3's test
    files. Drives runUpgrade end-to-end via Execute(ctx) against an httptest fake + injected Detect/
    ExecRunner/Swap/Rollback seams, proving: (a) a tampered archive (wrong SHA256) aborts before the
    swap with the on-disk binary byte-for-byte unchanged and no backup (FR-U11); (b) a binary whose
    --version reports the WRONG tag is refused by the sanity-run (FR-U5/U11); (c) a detected manager-
    owned install (brew) is DELEGATED — never self-swapped — recording 'brew upgrade stagecoach' via the
    injected ExecRunner (FR-U1); (d) --force overrides a detected manager-owned install, warns, and
    self-swaps (FR-U1); (e) --rollback restores the prior backup, or is a clean no-op when none exists
    (FR-U8). REUSES S2's helpers (buildStubVersion/packSwapArchive/newSwapFake/runVersion/setupSwapSeams/
    exeSuffix/backupSuffix/... in upgrade_swap_test.go) + cmd/stubversion (S2). NO real network, NO real
    PM subprocess, NO rename of the real running test binary (the swap/rollback closures target a temp
    stub). Dedicated file: S1 owns upgrade_check_test.go; S2 owns upgrade_swap_test.go; S3 owns this."
  - IMPORTS: bytes, context, io, os, os/exec, path/filepath, strings, sync, testing + internal/exitcode,
    internal/upgrade. (NO cobra/pflag/net/http/httptest UNLESS a given case needs the fake — only
    (a)/(b)/(d) call newSwapFake, which is S2's; the imports are whatever the bodies actually use. Drop
    unused imports or `make lint` (gosimple/unused) fails.)
  - TYPE recordingExecRunner: fields mu sync.Mutex; calls [][]string; code int; err error.
      Run(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) (int, error):
        r.mu.Lock(); r.calls = append(r.calls, append([]string{name}, args...)); r.mu.Unlock()
        return r.code, r.err   // ctx unused is fine (discard or _ = ctx)
      joinedCalls() string:
        r.mu.Lock(); defer r.mu.Unlock(); parts := []string{}; for _, c := range r.calls { parts = append(parts, strings.Join(c, " ")) };
        return strings.Join(parts, " && ")
  - FUNC runUpgradeArgs(t *testing.T, args ...string) (outBuf, errBuf *bytes.Buffer, err error):
      CLONE of S2's runUpgradeSwap body, but rootCmd.SetArgs(append([]string{"upgrade"}, args...)).
      saveRootState/restoreRootState via t.Cleanup; t.Cleanup(resetFlags(upgradeCmd.Flags()));
      t.Setenv("HOME", t.TempDir()); t.Setenv("XDG_CONFIG_HOME", ""); outBuf,errBuf=&bytes.Buffer{},&bytes.Buffer{};
      rootCmd.SetOut/SetErr; err = Execute(context.Background()); return.
  - NAMING: all unexported, package cmd.

Task 2: closures — miniSwap / failingSwap / miniRollback
  - FUNC miniSwap(installedExe string) func(context.Context, string) error:
      return func(ctx, newBinPath) error {
        if err := ctx.Err(); err != nil { return err }
        backup := installedExe + backupSuffix()
        if err := os.Rename(installedExe, backup); err != nil { return fmt.Errorf("backup: %w", err) }  // need "fmt"? use errors? — see GOTCHA
        if err := os.Rename(newBinPath, installedExe); err != nil { _ = os.Rename(backup, installedExe); return err }
        _ = os.RemoveAll(filepath.Dir(newBinPath))
        return nil }
      NOTE: S2's inline mini-swap uses fmt.Errorf("backup current binary: %w", err). To avoid importing
      fmt solely for that, either import fmt OR return the raw err (errors are not asserted on the swap
      in case (d) — the happy path). Prefer importing fmt for the diagnostic (matches S2).
  - FUNC failingSwap(t *testing.T) func(context.Context, string) error:
      return func(context.Context, string) error {
        t.Fatal("upgradeSwap must not be called on the delegation path (FR-U1: manager-owned binaries are delegated, not swapped)")
        return nil }
  - FUNC miniRollback(installedExe string) func(context.Context) (string, error):
      return func(ctx) (string, error) {
        if err := ctx.Err(); err != nil { return "", err }
        backup := installedExe + backupSuffix()
        if _, err := os.Stat(backup); err != nil { if os.IsNotExist(err) { return "", upgrade.ErrNoBackup }; return "", err }
        if err := os.Rename(backup, installedExe); err != nil { return "", err }
        out, err := exec.Command(installedExe, "--version").Output()   // the restored binary's version
        if err != nil { return "", err }
        return strings.TrimSpace(string(out)), nil }

Task 3: Case (a) TestUpgradeFailure_TamperedArchive
  - BUILD+PACK the VALID v0.2.0 payload: newStub := buildStubVersion(t, "v0.2.0");
    archive, realSha := packSwapArchive(t, newStub, "v0.2.0").
  - SERVE with a WRONG sha (the SHA-check failure, not a download failure):
    ts := newSwapFake(t, "v0.2.0", archive, strings.Repeat("a", 64)).   // 64 hex chars, != realSha
  - INSTALLED STUB (v0.1.0) in its OWN t.TempDir (SEPARATE from the staging dir):
    oldStub := buildStubVersion(t, "v0.1.0"); installDir := t.TempDir();
    installedExe := filepath.Join(installDir, "stagecoach"+exeSuffix());
    copy oldStub bytes → installedExe (0o644; chmod 0o755 on unix for the post-run --version, though it
    won't be reached).
  - SNAPSHOT before: beforeBytes := mustRead(installedExe); beforeTemp, _ := filepath.Glob(filepath.Join(os.TempDir(), "stagecoach-upgrade-*")).
  - SEAMS: setupSwapSeams(t, ts.URL, installedExe)  (detect→direct + mini-swap [never reached] + baseURL + SetCurrentVersion("0.1.0")).
  - DRIVE: outBuf, errBuf, err := runUpgradeArgs(t, "--yes").
  - ASSERT exit 1: if got := exitcode.For(err); got != exitcode.Error { t.Fatalf("exit=%d want 1; stdout=%q stderr=%q", got, outBuf, errBuf) }.
  - ASSERT on-disk UNCHANGED: afterBytes := mustRead(installedExe); if !bytes.Equal(beforeBytes, afterBytes) { t.Errorf("installed exe changed (FR-U11)") }.
  - ASSERT NO backup: if _, err := os.Stat(installedExe + backupSuffix()); !os.IsNotExist(err) { t.Errorf("backup created on a failing upgrade (FR-U11)") }.
  - ASSERT temp LEFT (FR-U11 inspection): afterTemp, _ := filepath.Glob(...); if len(afterTemp) <= len(beforeTemp) { t.Errorf("no staging tempDir left for inspection (FR-U11)") }.

Task 4: Case (b) TestUpgradeFailure_WrongVersionSanity
  - BUILD the WRONG-VERSION payload: badStub := buildStubVersion(t, "v0.3.0"); pack under the v0.2.0 ASSET
    name: archive, sha := packSwapArchive(t, badStub, "v0.2.0"); ts := newSwapFake(t, "v0.2.0", archive, sha).
    (VerifySHA256 PASSES — bytes match; sanityCheck fails — "v0.3.0" lacks "v0.2.0".)
  - INSTALLED STUB (v0.1.0) as in (a).
  - SNAPSHOT + SEAMS (setupSwapSeams) + DRIVE (runUpgradeArgs(t, "--yes")) as in (a).
  - ASSERT exit 1; on-disk UNCHANGED (bytes.Equal); NO backup; temp LEFT — IDENTICAL to (a)'s assertions.
    (Optional: assert stderr/stdout mentions the sanity/version failure for diagnostics — not required.)

Task 5: Case (c) TestUpgradeDelegation_ManagerOwnedNoForce
  - INSTALLED STUB (v0.2.0 — any version; it must remain untouched) in its OWN t.TempDir:
    installedExe := filepath.Join(t.TempDir(), "stagecoach"+exeSuffix()); copy a buildStubVersion(t,"v0.2.0") → it.
  - SNAPSHOT before: beforeBytes := mustRead(installedExe).
  - SEAMS (NO fake needed — delegate never networks):
      origDetect := upgradeDetect; upgradeDetect = func(context.Context, string, func(string)) (upgrade.Channel, string, error) {
          return upgrade.ChannelBrew, "brew", nil }; t.Cleanup(func() { upgradeDetect = origDetect })
      rec := &recordingExecRunner{code: 0, err: nil}
      origExec := upgradeExecRunner; upgradeExecRunner = rec; t.Cleanup(func() { upgradeExecRunner = origExec })
      origSwap := upgradeSwap; upgradeSwap = failingSwap(t); t.Cleanup(func() { upgradeSwap = origSwap })
      t.Cleanup(func() { upgrade.SetCurrentVersion("dev") }); upgrade.SetCurrentVersion("0.2.0")
      (Leave upgradeBaseURL at its "" default — PROVES no network. Leave upgradeRollback at default —
      unreachable on the normal path.)
  - DRIVE: outBuf, errBuf, err := runUpgradeArgs(t, "--yes").   (flagYes⇒confirmUpgrade(true,nil) for brew, a RUN channel.)
  - ASSERT exit 0: if got := exitcode.For(err); got != exitcode.Success { t.Fatalf("exit=%d want 0; stdout=%q stderr=%q", got, outBuf, errBuf) }.
  - ASSERT delegated to brew: if rec.joinedCalls() != "brew upgrade stagecoach" { t.Errorf("delegated argv=%q; want 'brew upgrade stagecoach'", rec.joinedCalls()) }.
  - ASSERT success line: if !strings.Contains(outBuf.String(), "stagecoach updated via brew") { t.Errorf("stdout=%q", outBuf) }.
  - ASSERT NO swap (failingSwap never fired ⇒ the test reached here) + on-disk UNCHANGED:
    afterBytes := mustRead(installedExe); if !bytes.Equal(beforeBytes, afterBytes) { t.Errorf("installed exe changed on delegation (FR-U1)") }.

Task 6: Case (d) TestUpgradeDelegation_ForceOverride
  - BUILD+PACK+SERVE the VALID v0.2.0 payload (as in S2's happy path): newStub := buildStubVersion(t, "v0.2.0");
    archive, sha := packSwapArchive(t, newStub, "v0.2.0"); ts := newSwapFake(t, "v0.2.0", archive, sha).
  - INSTALLED STUB (v0.1.0): installedExe := filepath.Join(t.TempDir(), "stagecoach"+exeSuffix()); copy buildStubVersion(t,"v0.1.0") → it; chmod 0o755 (unix).
  - SEAMS (detect→brew + baseURL + mini-swap [RUNS] + version):
      origBase := upgradeBaseURL; upgradeBaseURL = ts.URL; t.Cleanup(func() { upgradeBaseURL = origBase })
      origDetect := upgradeDetect; upgradeDetect = func(context.Context, string, func(string)) (upgrade.Channel, string, error) {
          return upgrade.ChannelBrew, "brew", nil }; t.Cleanup(func() { upgradeDetect = origDetect })
      origSwap := upgradeSwap; upgradeSwap = miniSwap(installedExe); t.Cleanup(func() { upgradeSwap = origSwap })
      t.Cleanup(func() { upgrade.SetCurrentVersion("dev") }); upgrade.SetCurrentVersion("0.1.0")
      (Leave upgradeExecRunner at default — runDirectSwap does NOT use it; only runDelegate does.)
  - DRIVE: outBuf, errBuf, err := runUpgradeArgs(t, "--yes", "--force").   (flagForce=true ⇒ dispatchUpgrade warns + routes to runDirectSwap.)
  - ASSERT exit 0: if got := exitcode.For(err); got != exitcode.Success { t.Fatalf("exit=%d want 0; stdout=%q stderr=%q", got, outBuf, errBuf) }.
  - ASSERT the FR-U1 warning on STDERR: if !strings.Contains(errBuf.String(), "--force overriding a detected brew install") { t.Errorf("stderr missing the --force warning; got:\n%s", errBuf) }.
  - ASSERT swapped to NEW: if !strings.Contains(runVersion(t, installedExe), "v0.2.0") { t.Errorf("installed not swapped to v0.2.0") }.
  - ASSERT backup is OLD: if !strings.Contains(runVersion(t, installedExe+backupSuffix()), "v0.1.0") { t.Errorf("backup not v0.1.0") }.

Task 7: Case (e) TestUpgradeRollback — no-backup (e1) + backup-present (e2)
  - (e1) no-backup:
      SEAMS: origRollback := upgradeRollback; upgradeRollback = func(context.Context) (string, error) {
        return "", upgrade.ErrNoBackup }; t.Cleanup(func() { upgradeRollback = origRollback })
        (Leave upgradeDetect/upgradeSwap/upgradeBaseURL at defaults — unreachable on the --rollback path.)
      DRIVE: outBuf, _, err := runUpgradeArgs(t, "--rollback").
      ASSERT exit 0: if got := exitcode.For(err); got != exitcode.Success { t.Fatalf("exit=%d want 0; stdout=%q", got, outBuf) }.
      ASSERT the no-op message: if !strings.Contains(outBuf.String(), "no backup — nothing to roll back") { t.Errorf("stdout=%q", outBuf) }.
  - (e2) backup-present:
      INSTALLED STUB (v0.2.0) + a pre-created BACKUP (v0.1.0):
        installedExe := filepath.Join(t.TempDir(), "stagecoach"+exeSuffix()); copy buildStubVersion(t,"v0.2.0") → it; chmod 0o755 (unix).
        backupPath := installedExe + backupSuffix(); copy buildStubVersion(t,"v0.1.0") → backupPath; chmod 0o755 (unix).
      SEAMS: origRollback := upgradeRollback; upgradeRollback = miniRollback(installedExe);
        t.Cleanup(func() { upgradeRollback = origRollback })
      DRIVE: outBuf, _, err := runUpgradeArgs(t, "--rollback").
      ASSERT exit 0: if got := exitcode.For(err); got != exitcode.Success { t.Fatalf("exit=%d want 0; stdout=%q", got, outBuf) }.
      ASSERT restored message: if !strings.Contains(outBuf.String(), "restored stagecoach v0.1.0") { t.Errorf("stdout=%q", outBuf) }.
      ASSERT current became backup content: if !strings.Contains(runVersion(t, installedExe), "v0.1.0") { t.Errorf("installed not restored to v0.1.0") }.
  - (Implement (e1)/(e2) as two functions OR t.Run subtests of TestUpgradeRollback.)

Task 8: small read helper + VERIFY (build/vet/fmt/test/lint/scope)
  - FUNC mustRead(t, path string) []byte { b, err := os.ReadFile(path); if err != nil { t.Fatal(err) }; return b }
    (used by (a)/(b)/(c) for the bytes.Equal unchanged assertion.)
  - go build ./... ; go vet ./internal/cmd/... ; gofmt -l (empty).
  - go test ./internal/cmd/ -run "TestUpgradeFailure|TestUpgradeDelegation|TestUpgradeRollback" -race -v  # all 5 PASS.
  - go test ./internal/cmd/ -race    # no sibling broke (seams/flags restored).
  - go test -race ./...              # full matrix-green suite.
  - make test && make lint           # staticcheck/unused/gosimple clean (no U1000 — every helper/import used).
  - git status --porcelain (grep guards — see Validation Loop Level 4).
```

### Implementation Patterns & Key Details

```go
// PATTERN (recordingExecRunner — the upgrade.ExecRunner fake for the delegation case (c)):
type recordingExecRunner struct {
	mu    sync.Mutex
	calls [][]string
	code  int
	err   error
}

func (r *recordingExecRunner) Run(_ context.Context, _, _ io.Writer, name string, args ...string) (int, error) {
	r.mu.Lock()
	r.calls = append(r.calls, append([]string{name}, args...))
	r.mu.Unlock()
	return r.code, r.err
}

func (r *recordingExecRunner) joinedCalls() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	parts := make([]string, 0, len(r.calls))
	for _, c := range r.calls {
		parts = append(parts, strings.Join(c, " "))
	}
	return strings.Join(parts, " && ")
}

// PATTERN (runUpgradeArgs — the parameterized Execute driver; mirrors S2's runUpgradeSwap body):
func runUpgradeArgs(t *testing.T, args ...string) (outBuf, errBuf *bytes.Buffer, err error) {
	t.Helper()
	_, origOut, origErr, origRunE := saveRootState(t)
	t.Cleanup(func() { restoreRootState(t, nil, origOut, origErr, origRunE) })
	t.Cleanup(func() { resetFlags(upgradeCmd.Flags()) }) // SEPARATE from restoreRootState
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "") // LoadUpgradeConfig ⇒ Defaults (no global config, no bootstrap)
	outBuf, errBuf = &bytes.Buffer{}, &bytes.Buffer{}
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs(append([]string{"upgrade"}, args...))
	err = Execute(context.Background())
	return outBuf, errBuf, err
}

// PATTERN (miniSwap — the swap closure for case (d); faithful twin of S2's inline mini-swap):
func miniSwap(installedExe string) func(context.Context, string) error {
	return func(ctx context.Context, newBinPath string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		backup := installedExe + backupSuffix()
		if err := os.Rename(installedExe, backup); err != nil { // one-deep backup (FR-U8)
			return fmt.Errorf("backup current binary: %w", err)
		}
		if err := os.Rename(newBinPath, installedExe); err != nil { // atomic rename into place
			_ = os.Rename(backup, installedExe) // FR-U11 restore (best-effort)
			return fmt.Errorf("rename new binary into place: %w (restored from backup)", err)
		}
		_ = os.RemoveAll(filepath.Dir(newBinPath)) // staging tempDir cleanup
		return nil
	}
}

// PATTERN (failingSwap — PROVES case (c) never swaps; a bug that routed brew to runDirectSwap fails loudly):
func failingSwap(t *testing.T) func(context.Context, string) error {
	t.Helper()
	return func(context.Context, string) error {
		t.Fatal("upgradeSwap must not be called on the delegation path (FR-U1: manager-owned binaries are delegated, not swapped)")
		return nil
	}
}

// PATTERN (miniRollback — case (e2); mirrors upgrade.Rollback's contract, operates on a temp exe):
func miniRollback(installedExe string) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		backup := installedExe + backupSuffix()
		if _, err := os.Stat(backup); err != nil {
			if os.IsNotExist(err) {
				return "", upgrade.ErrNoBackup
			}
			return "", err
		}
		if err := os.Rename(backup, installedExe); err != nil { // FR-U8 one-step: backup CONSUMED, prev-current LOST
			return "", err
		}
		out, err := exec.Command(installedExe, "--version").Output() // the restored binary's reported version
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}
}

// PATTERN (case (c) seam wiring — detect→brew + recorder + failingSwap; NO fake, baseURL left default):
origDetect := upgradeDetect
upgradeDetect = func(context.Context, string, func(string)) (upgrade.Channel, string, error) {
	return upgrade.ChannelBrew, "brew", nil
}
t.Cleanup(func() { upgradeDetect = origDetect })
rec := &recordingExecRunner{code: 0, err: nil}
origExec := upgradeExecRunner
upgradeExecRunner = rec
t.Cleanup(func() { upgradeExecRunner = origExec })
origSwap := upgradeSwap
upgradeSwap = failingSwap(t)
t.Cleanup(func() { upgradeSwap = origSwap })
t.Cleanup(func() { upgrade.SetCurrentVersion("dev") })
upgrade.SetCurrentVersion("0.2.0")
// outBuf, _, err := runUpgradeArgs(t, "--yes")
// if exitcode.For(err) != exitcode.Success { t.Fatalf(...) }
// if rec.joinedCalls() != "brew upgrade stagecoach" { t.Errorf(...) }      // POSITIVE proof of delegation
// if !strings.Contains(outBuf.String(), "stagecoach updated via brew") { t.Errorf(...) }
// if !bytes.Equal(beforeBytes, mustRead(t, installedExe)) { t.Errorf("FR-U1: installed exe changed") }

// PATTERN (case (a) trigger — serve the REAL archive but a WRONG sha in /checksums):
// archive, _ := packSwapArchive(t, buildStubVersion(t, "v0.2.0"), "v0.2.0")
// ts := newSwapFake(t, "v0.2.0", archive, strings.Repeat("a", 64))   // 64 hex chars != real sha
// setupSwapSeams(t, ts.URL, installedExe)                             // detect→direct + mini-swap (never reached)
// outBuf, errBuf, err := runUpgradeArgs(t, "--yes")
// if exitcode.For(err) != exitcode.Error { t.Fatalf("want exit 1; got %d; stderr=%q", exitcode.For(err), errBuf) }
// if !bytes.Equal(before, mustRead(t, installedExe)) { t.Errorf("FR-U11: installed exe changed") }
// if _, err := os.Stat(installedExe + backupSuffix()); !os.IsNotExist(err) { t.Errorf("FR-U11: backup created") }

// PATTERN (case (e1) no-backup — upgradeRollback returns ErrNoBackup):
// origRollback := upgradeRollback
// upgradeRollback = func(context.Context) (string, error) { return "", upgrade.ErrNoBackup }
// t.Cleanup(func() { upgradeRollback = origRollback })
// outBuf, _, err := runUpgradeArgs(t, "--rollback")   // NO --yes (rollback path has no confirm)
// if exitcode.For(err) != exitcode.Success { t.Fatalf(...) }
// if !strings.Contains(outBuf.String(), "no backup — nothing to roll back") { t.Errorf(...) }
```

### Integration Points

```yaml
CONSUMES (LANDED — READ-ONLY, zero edits):
  - internal/cmd (same package): the seam vars (upgradeBaseURL/upgradeDetect/upgradeExecRunner/
    upgradeSwap/upgradeRollback — upgrade_run.go); runUpgrade/dispatchUpgrade/runDirectSwap/runDelegate
    (upgrade.go/upgrade_run.go); flagYes/flagCheck/flagForce/flagRollback/flagChannel (upgrade.go, set by
    cobra on the args); saveRootState/restoreRootState/resetFlags/Execute/rootCmd/upgradeCmd (root.go/
    root_test.go).
  - internal/cmd/upgrade_swap_test.go (S2, same package — REUSED, NOT redefined): buildStubVersion,
    packSwapArchive, newSwapFake, runVersion, setupSwapSeams, runUpgradeSwap, exeSuffix, backupSuffix,
    hostAssetName, hostEntryName, checksumsName, stubVerCache/stubVerMu.
  - internal/upgrade: ChannelBrew/ChannelDirect (detect.go — consts); SetCurrentVersion (version.go —
    EXPORTED seam); ErrNoBackup (rollback.go — sentinel); ExecRunner (delegate.go — interface);
    ResolveTarget/Client/LatestStable/Release/Asset (resolve.go/releases.go — driven via the fake by
    cases (a)/(b)/(d)); StageNewBinary/VerifySHA256/sanityCheck (stage.go/download.go — REAL, against
    the fake, for (a)/(b)/(d)).
  - internal/exitcode: Success/Error/For (exitcode.go).
  - stdlib: bytes, context, io, os, os/exec, path/filepath, strings, sync, testing (+ fmt for miniSwap's
    Errorf diagnostics; + net/http/httptest ONLY if a case calls newSwapFake directly — (a)/(b)/(d) do;
    the imports must match the bodies actually used or gosimple/unused flags them).
  - cmd/stubversion (S2, LANDED): built by the test via buildStubVersion at v0.1.0/v0.2.0/v0.3.0.
NO database / migration / routes / config-struct change / exitcode-const change / go.mod change / docs change /
production-code change. The ONLY non-test artifact is the NEW test file (cmd/stubversion already exists from S2).
SCOPE FENCES:
  - Touches ONLY: internal/cmd/upgrade_safety_test.go (NEW).
  - Does NOT edit: internal/cmd/upgrade*.go (LANDED), internal/cmd/upgrade_check_test.go (S1),
    internal/cmd/upgrade_swap_test.go (S2), root.go, internal/upgrade/* (read-only — P1.M1–M3 Complete),
    internal/exitcode/*, config, the commit path (FR-U12), go.mod, any PRD/task file, cmd/stubversion/* (S2).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (the new test file compiles against the LANDED seams + S2's helpers).
go build ./...
# Expected: clean. Watch: "recordingExecRunner does not implement upgrade.ExecRunner" (Run signature!),
#           "upgradeDetect/upgradeSwap/upgradeRollback undefined" (same package — confirm), "imported and not used".

# Vet the changed package (test files included).
go vet ./internal/cmd/...
# Expected: clean.

# Format.
gofmt -l internal/cmd/upgrade_safety_test.go
# Expected: empty. If listed: gofmt -w it.

# Lint (staticcheck/unused/errcheck/gosimple). The test file must have no U1000 (every helper/import used;
# recordingExecRunner's Run ctx param may be unused — name it `_` or `_ = ctx`; the recorder fields are read;
# t.Cleanup restores satisfy errcheck).
make lint
# Expected: zero errors.

# Scope guard: ONLY the one new file.
git status --porcelain
# Expected: internal/cmd/upgrade_safety_test.go ONLY. FAIL if any production file (or S1/S2's test file) appears.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Run the five new safety-suite cases.
go test ./internal/cmd/ -run "TestUpgradeFailure|TestUpgradeDelegation|TestUpgradeRollback" -race -v
# Expected: all 5 PASS.
#   (a) TestUpgradeFailure_TamperedArchive            → exit 1; unchanged; no backup; temp left.
#   (b) TestUpgradeFailure_WrongVersionSanity         → exit 1; unchanged; no backup; temp left.
#   (c) TestUpgradeDelegation_ManagerOwnedNoForce     → exit 0; recorded 'brew upgrade stagecoach'; unchanged.
#   (d) TestUpgradeDelegation_ForceOverride           → exit 0; stderr warning; swapped→v0.2.0; backup v0.1.0.
#   (e1) TestUpgradeRollback (no-backup)              → exit 0; 'no backup — nothing to roll back'.
#   (e2) TestUpgradeRollback (backup-present)         → exit 0; 'restored stagecoach v0.1.0'; exe now v0.1.0.
# No network: (a)/(b)/(d) use the localhost fake (upgradeBaseURL=ts.URL); (c)/(e) use NO fake at all.

# Full cmd-package regression (the seams/flags are restored via t.Cleanup, so no leak into S1/S2's suites).
go test ./internal/cmd/ -race
# Expected: green (S1's TestUpgradeCheck_* + S2's TestUpgradeSwap_* + the registration/prompt tests unaffected).

# Full race suite (the CI matrix command).
go test -race ./...
# Expected: all green (incl. cmd/stubversion compiles; internal/upgrade/*_test.go unaffected).

make test && make lint && make build
# Expected: all green.
```

### Level 3: Integration Testing (System Validation)

```bash
# (This suite IS the integration test for the FR-U1/U8/U11 safety paths — it drives runUpgrade end-to-end via
#  Execute against the fake + injected runners, with the REAL StageNewBinary download/verify/extract/sanity for
#  (a)/(b)/(d), a REAL Delegate dispatch for (c), and a REAL dispatchUpgrade --rollback branch for (e). There is
#  no separate "service" to start.) Offline-capability smoke (prove no real network for (c)):
go test ./internal/cmd/ -run TestUpgradeDelegation_ManagerOwnedNoForce -race   # passes with NO fake (baseURL="")

# Optional: run the WHOLE self-update matrix together (S1 --check + S2 happy-path + S3 safety):
go test ./internal/cmd/ -run "TestUpgrade" -race -v   # every TestUpgrade* across the three files
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: ONLY the one new file; ZERO production/S1/S2 edits.
git status --porcelain
test "$(git status --porcelain | wc -l)" -eq 1 && echo "OK: one file" || echo "FAIL: expected one file"
git diff --name-only | grep -vE '^internal/cmd/upgrade_safety_test\.go$' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
git diff --name-only | grep -qE '^internal/(upgrade|exitcode)/|^internal/cmd/(upgrade|upgrade_run|upgrade_prompt|upgrade_check_test|upgrade_swap_test|root)(\.go|_test\.go)$' && echo "FAIL: edited LANDED file" || echo "OK: production + S1/S2 untouched"

# Guard 2: the five tests + the three closures + the recorder exist.
grep -q "func TestUpgradeFailure_TamperedArchive" internal/cmd/upgrade_safety_test.go        || echo "MISSING (a)"
grep -q "func TestUpgradeFailure_WrongVersionSanity" internal/cmd/upgrade_safety_test.go     || echo "MISSING (b)"
grep -q "func TestUpgradeDelegation_ManagerOwnedNoForce" internal/cmd/upgrade_safety_test.go || echo "MISSING (c)"
grep -q "func TestUpgradeDelegation_ForceOverride" internal/cmd/upgrade_safety_test.go       || echo "MISSING (d)"
grep -q "func TestUpgradeRollback" internal/cmd/upgrade_safety_test.go                       || echo "MISSING (e)"
grep -q "type recordingExecRunner struct" internal/cmd/upgrade_safety_test.go                || echo "MISSING recorder"
grep -qE "func miniSwap|func failingSwap|func miniRollback" internal/cmd/upgrade_safety_test.go || echo "MISSING closures"

# Guard 3: each case exercises its load-bearing seam.
grep -n 'upgrade.ChannelBrew' internal/cmd/upgrade_safety_test.go        # (c)/(d) detect→brew injection
grep -n 'upgradeExecRunner =' internal/cmd/upgrade_safety_test.go        # (c) the Delegate ExecRunner seam
grep -n 'failingSwap\|miniSwap' internal/cmd/upgrade_safety_test.go      # (c) no-swap proof / (d) swap
grep -n 'upgradeRollback =\|miniRollback\|upgrade.ErrNoBackup' internal/cmd/upgrade_safety_test.go  # (e)
grep -n 'upgrade.SetCurrentVersion' internal/cmd/upgrade_safety_test.go  # version seam (set + restore 'dev')

# Guard 4: exit codes asserted via exitcode.For on the Execute return (NOT os.Exit / NOT raw code) — per case.
grep -n 'exitcode.For(err)' internal/cmd/upgrade_safety_test.go
grep -n 'exitcode.Error\b' internal/cmd/upgrade_safety_test.go   # (a)/(b) want exit 1
grep -n 'exitcode.Success' internal/cmd/upgrade_safety_test.go   # (c)/(d)/(e) want exit 0

# Guard 5: FR-U11 invariants for the failure cases.
grep -n 'bytes.Equal\|backupSuffix()\|stagecoach-upgrade-\*' internal/cmd/upgrade_safety_test.go  # unchanged/no-backup/temp-left
grep -n 'strings.Repeat("a", 64)' internal/cmd/upgrade_safety_test.go   # (a) the wrong-sha trigger

# Guard 6: FR-U1 delegation + --force warning.
grep -n 'brew upgrade stagecoach' internal/cmd/upgrade_safety_test.go            # (c) recorded argv
grep -n 'updated via brew' internal/cmd/upgrade_safety_test.go                   # (c) success line
grep -n 'force overriding a detected brew install' internal/cmd/upgrade_safety_test.go  # (d) the warning

# Guard 7: FR-U8 rollback messages.
grep -n 'no backup — nothing to roll back' internal/cmd/upgrade_safety_test.go   # (e1)
grep -n 'restored stagecoach' internal/cmd/upgrade_safety_test.go                # (e2)

# Guard 8: the Execute driver + the mandatory resetFlags(upgradeCmd.Flags()) + isolated HOME.
grep -n 'Execute(context.Background())' internal/cmd/upgrade_safety_test.go
grep -n 'resetFlags(upgradeCmd.Flags())' internal/cmd/upgrade_safety_test.go
grep -n 'XDG_CONFIG_HOME' internal/cmd/upgrade_safety_test.go

# Guard 9: cross-platform backup suffix via backupSuffix() (NOT a hardcoded .stagecoach-backup).
grep -n 'runtime.GOOS\|backupSuffix()' internal/cmd/upgrade_safety_test.go   # the platform branch is in S2's helper; S3 CALLS backupSuffix()

# Guard 10: the suite is offline-capable for the delegation case (run it; NO fake is the no-network proof).
go test ./internal/cmd/ -run TestUpgradeDelegation_ManagerOwnedNoForce -race   # PASS
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./internal/cmd/...` clean; `gofmt -l` empty on the new file.
- [ ] `make lint` zero errors (no U1000 — every helper/import used; `_ = ctx`/`_` param for unused; recorder
      fields read; t.Cleanup restores satisfy errcheck).
- [ ] `go test ./internal/cmd/ -run "TestUpgradeFailure|TestUpgradeDelegation|TestUpgradeRollback" -race -v`
      PASSES all five cases.
- [ ] `go test ./internal/cmd/ -race` green (no sibling broke — S1's --check + S2's happy-path + registration
      + prompt tests unaffected; seams/flags restored between tests).
- [ ] `go test -race ./...` green (the CI matrix command).
- [ ] `make test` + `make build` green.

### Feature Validation (the FR-U1/U8/U11 safety paths)
- [ ] (a) tampered archive: `exitcode.For(err)==exitcode.Error`; installed exe `bytes.Equal` before==after;
      no backup (`os.IsNotExist`); a `stagecoach-upgrade-*` tempDir appeared (FR-U11).
- [ ] (b) wrong-version sanity: same assertions as (a); the v0.3.0 binary under a v0.2.0 release is refused.
- [ ] (c) delegation: `exitcode.For(err)==exitcode.Success`; `recorder.joinedCalls()=="brew upgrade
      stagecoach"`; stdout has "stagecoach updated via brew"; installed exe `bytes.Equal` before==after;
      `failingSwap` never fired (the test reached its assertions).
- [ ] (d) --force: `exitcode.For(err)==exitcode.Success`; errBuf has "--force overriding a detected brew
      install"; installed exe `--version` has `v0.2.0`; backup `--version` has `v0.1.0`.
- [ ] (e1) rollback no-backup: `exitcode.For(err)==exitcode.Success`; stdout has "no backup — nothing to roll
      back".
- [ ] (e2) rollback backup-present: `exitcode.For(err)==exitcode.Success`; stdout has "restored stagecoach
      v0.1.0"; installed exe `--version` has `v0.1.0` (current became backup content).

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `internal/cmd/upgrade_safety_test.go` (NEW) (grep guard 1).
- [ ] NO edit to internal/cmd/upgrade*.go (LANDED), internal/cmd/upgrade_check_test.go (S1),
      internal/cmd/upgrade_swap_test.go (S2), internal/upgrade/* (read-only), root.go, exitcode.go, config,
      the commit path (FR-U12), go.mod, any PRD/task file, cmd/stubversion/* (S2).

### Cross-Platform Validation
- [ ] linux/darwin: tar.gz archive + `stagecoach` entry + `.stagecoach-backup` backup (CI ubuntu/macos).
- [ ] windows: zip archive + `stagecoach.exe` entry + `.old` backup (CI windows-latest).
- [ ] `go test -race ./...` green on ubuntu-latest, macos-latest, windows-latest (the CI matrix) — every
      backup-path / backup-suffix reference goes through `backupSuffix()`.

### Code Quality & Docs
- [ ] upgrade_safety_test.go file doc names the suite (FR-U1/U8/U11), lists the five cases + the per-case
      seams overridden, the no-network/no-PM/no-real-binary-rename guarantee, the S2-helper reuse, and the
      dedicated-file rationale (S1=check, S2=swap, S3=safety).
- [ ] Each seam override is restored via t.Cleanup (upgradeDetect→prodDetect, upgradeExecRunner→original,
      upgradeSwap→upgrade.Swap, upgradeRollback→upgrade.Rollback, upgradeBaseURL→original/"",
      currentVersion→"dev", rootCmd state, upgradeCmd flags).
- [ ] recordingExecRunner implements the STREAMING upgrade.ExecRunner.Run (NOT detect.go's capturing Runner).
- [ ] miniSwap/miniRollback are FAITHFUL twins of upgrade.Swap/Rollback's success path and NEVER call
      os.Executable/resolveCurrentExe (the real test binary is safe); failingSwap t.Fatal's (case (c) proof).

---

## Anti-Patterns to Avoid

- ❌ Don't REDEFINE S2's helpers (buildStubVersion, packSwapArchive, newSwapFake, runVersion, exeSuffix,
  backupSuffix, hostAssetName, hostEntryName, checksumsName, setupSwapSeams, runUpgradeSwap). They are
  package-`cmd` package-level identifiers in upgrade_swap_test.go — redeclaring them is a COMPILE ERROR
  (redeclaration in the same package). REUSE them; S3 adds only its own NEW identifiers.
- ❌ Don't trigger the SHA failure (a) with a malformed/non-hex digest or by tampering the archive bytes.
  VerifySHA256 lowercases+trims the want digest and constant-time-compares; serve the REAL archive (from
  packSwapArchive) but a valid-hex WRONG digest in /checksums (`strings.Repeat("a", 64)`). This isolates
  the failure to the SHA check (not a download or checksum-parse error) and is the FR-U11 "checksum failure".
- ❌ Don't build the wrong-version payload (b) under a v0.3.0 ASSET name. The asset must be named
  `stagecoach_0.2.0_<GOOS>_<GOARCH>.<ext>` (release tag v0.2.0) so ResolveTarget/SelectAsset finds it; the
  trick is the EMBEDDED binary prints v0.3.0 (`buildStubVersion(t,"v0.3.0")` packed via
  `packSwapArchive(t, v0.3.0stub, "v0.2.0")`). Then VerifySHA256 passes (real sha) and sanityCheck fails.
- ❌ Don't implement recordingExecRunner.Run with detect.go's capturing signature
  `Run(ctx, name, args...) (string, int, error)`. upgrade.ExecRunner (delegate.go) is the STREAMING seam:
  `Run(ctx, stdout, stderr io.Writer, name string, args ...string) (int, error)`. A signature mismatch means
  the recorder does NOT satisfy the interface and Delegate would use the prod osExecRunner (real brew!).
- ❌ Don't leave `upgradeExecRunner` at its nil default for case (c). nil ⇒ osExecRunner (prod), which would
  shell out to the REAL `brew` (absent on CI ⇒ start failure ⇒ exit 1, a false negative). INJECT the recorder.
  (Case (d) is the opposite: runDirectSwap does NOT use upgradeExecRunner — leave it at default there.)
- ❌ Don't call the real `upgrade.Rollback`/`upgrade.Swap` for case (e)/(d)/(c). They resolve the exe via the
  package-upgrade UNEXPORTED resolveCurrentExe (returns the REAL test binary) and would rename/clobber it.
  Override the upgradeRollback/upgradeSwap SEAM VARS with miniRollback/miniSwap/failingSwap. The real
  Rollback/Swap are covered by rollback_test.go/swap_test.go.
- ❌ Don't wire `failingSwap` for case (d). (d) needs the swap to HAPPEN (miniSwap). failingSwap is for (c)
  only, to PROVE the delegation path never swaps. Mixing them up makes (c) pass trivially (no swap attempted)
  or (d) fail loudly — either way the contract is untested.
- ❌ Don't add `--yes` to the `--rollback` args and assume it's required. The --rollback branch in
  dispatchUpgrade calls upgradeRollback DIRECTLY (no confirmUpgrade) — `["upgrade","--rollback"]` is correct
  and `--yes` is unnecessary (harmless, but don't depend on it). validateUpgradeFlags allows --rollback alone.
- ❌ Don't assert on-disk "unchanged" via `--version` for (a)/(b)/(c). The installed exe is never reached by
  the new payload on those paths, so its version is trivially the same — that does NOT prove byte-for-byte
  invariance (FR-U11). Read the bytes before+after and `bytes.Equal` them.
- ❌ Don't hardcode `.stagecoach-backup`. The backup suffix is `.old` on windows (swap_windows.go backupPath).
  Use `backupSuffix()` for the "no backup" stat (a/b), the miniSwap backup, the miniRollback backup, AND the
  (d) backup assertion, or the windows-latest matrix job fails.
- ❌ Don't omit `resetFlags(upgradeCmd.Flags())` in runUpgradeArgs. restoreRootState resets only rootCmd's
  flags; the upgrade LOCAL flags (flagYes/flagForce/flagRollback/flagChannel) leak across tests. Mirror S2's
  two-defer idiom.
- ❌ Don't leave HOME unisolated. runUpgrade calls LoadUpgradeConfig (reads the global config under
  $HOME/$XDG). Isolate with t.TempDir() + XDG_CONFIG_HOME="" (runUpgradeArgs does this).
- ❌ Don't create an httptest fake for case (c) or the rollback cases. The delegate/rollback paths NEVER touch
  the releases Client. Running with NO fake (and upgradeBaseURL at its "" default) is the strongest proof
  those paths don't network. (A stray accidental network call would flake against api.github.com.)
- ❌ Don't add production code or a new stub binary. This item is TESTS-ONLY. runUpgrade/dispatchUpgrade +
  the seams are LANDED (P1.M4.T2.S1); StageNewBinary/Delegate/Rollback are LANDED (P1.M1–M3);
  cmd/stubversion is LANDED (S2). Any edit to upgrade*.go/internal/upgrade/* or a new cmd/* is out of scope.