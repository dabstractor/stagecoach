name: "P1.M4.T3.S2 — direct-swap happy path (download→verify→sanity→backup→rename) in a temp dir"
description: >
  The SECOND of P1.M4.T3's three test suites. A NEW package-cmd test file
  (internal/cmd/upgrade_swap_test.go) PLUS a NEW tiny test-helper binary (cmd/stubversion/main.go) that
  together prove the FR-U5/U7/U9/U11 direct-binary self-swap HAPPY PATH end-to-end through runDirectSwap
  against an httptest fake GitHub Releases server — NO real network, NO real binary rename of the test
  runner, NO real api.github.com. It drives `stagecoach upgrade --yes` (the NORMAL path) via Execute(ctx)
  with args ["upgrade","--yes"], with the package-cmd seams overridden so detect→ChannelDirect and the
  swap targets a temp-dir "installed" stub: (1) upgradeBaseURL=httptest.URL (the NETWORK seam — prodNewClient
  reads it at call time); (2) upgradeDetect→(upgrade.ChannelDirect,"direct",nil) ("injected Detect→direct");
  (3) upgradeSwap→a faithful mini-swap closure that backs up + renames + cleans the staging tempDir against
  the temp stub (the SANCTIONED override — package cmd CANNOT reach upgrade.Swap's unexported
  resolveCurrentExe seam, so upgrade_run.go explicitly designed upgradeSwap as the function seam to
  override); (4) upgrade.SetCurrentVersion("0.1.0") (the EXPORTED version seam, restored to "dev").
  cmd/stubversion is a NEW ldflags-baked version stub (the existing cmd/stubcli is ENV-controlled and
  CANNOT express two distinct versions — the installed stub must print v0.1.0 while the new payload prints
  v0.2.0). It is built TWICE (v0.1.0 → the installed stub; v0.2.0 → packed host-native into the archive the
  fake serves). The REAL pipeline under test: runDirectSwap → os.MkdirTemp(staging) → ResolveTarget (GET
  fake /releases/latest → SelectAsset host asset) → StageNewBinary (DownloadFile+VerifySHA256+extractBinary+
  sanityCheck via the DEFAULT execVersion — stubversion bakes its --version, so NO execVersion override) →
  confirmUpgrade(--yes⇒true,nil, no prompt) → upgradeSwap(mini-swap) → "stagecoach upgraded to v0.2.0".
  ASSERTS: exit 0; stdout has "stagecoach upgraded to v0.2.0"; the installed exe's --version==v0.2.0
  (swapped); the backup's (installed+".stagecoach-backup" on unix / ".old" on windows) --version==v0.1.0
  (backed up); the "stagecoach-upgrade-*" staging tempDir glob is UNCHANGED before==after (cleaned); the
  real test runner binary is never touched. CROSS-PLATFORM: .tar.gz+stagecoach+.stagecoach-backup on
  linux/darwin; .zip+stagecoach.exe+.old on windows. `go test -race ./...` green on the
  ubuntu/macos/windows CI matrix. ZERO edits to internal/upgrade/* (the real upgrade.Swap is exhaustively
  covered by swap_test.go/swap_windows_test.go — S2 owns runDirectSwap ORCHESTRATION + the swap SEAM, not a
  re-test of upgrade.Swap). go build/vet/gofmt/make test/make lint clean.

---

## Goal

**Feature Goal**: Prove the FR-U5/U7/U9/U11 `stagecoach upgrade` direct-binary self-swap happy path
(`runDirectSwap` in `internal/cmd/upgrade_run.go`, LANDED by P1.M4.T2.S1) performs the full
download→verify→sanity→backup→rename→cleanup sequence correctly and safely in a temp dir, using only an
httptest fake + a compiled version stub + the package-cmd/upgrade seams — no real network, no real
subprocess beyond the stub's own --version, and no rename of the real running test binary.

**Deliverable**:
1. `cmd/stubversion/main.go` (NEW, `package main`, stdlib-only) — a tiny binary that prints a build-time
   ldflags-baked `version` var (default "dev") on any invocation. Built twice by the test to stand in for
   the "installed" v0.1.0 binary and the "new" v0.2.0 payload.
2. `internal/cmd/upgrade_swap_test.go` (NEW, package `cmd`) — the direct-swap happy-path suite: a
   `buildStubVersion` helper, host-asset/archive/pack helpers (ported from `internal/upgrade/stage_test.go`),
   a 3-route httptest fake GitHub Releases server, a seam-setup/restore helper, and the
   `TestUpgradeSwap_DirectHappyPath` test.

**Success Definition**:
- `go test ./internal/cmd/ -run TestUpgradeSwap -race -v` PASSES (the happy-path case).
- Exit code: `exitcode.For(err) == exitcode.Success (0)`; stdout contains `stagecoach upgraded to v0.2.0`.
- The temp "installed" stub's `--version` is `v0.2.0` after the swap (the NEW binary was renamed into place).
- The backup file's (installed stub path + platform suffix) `--version` is `v0.1.0` (the OLD binary was
  backed up — FR-U8 one-deep).
- The `stagecoach-upgrade-*` staging tempDir glob is UNCHANGED before vs. after Execute (created by
  runDirectSwap, removed by the mini-swap on success).
- No real network: `upgradeBaseURL = httptest.URL` (localhost); no `api.github.com` anywhere in the file.
- The real running test binary is untouched (the mini-swap uses only the captured temp-stub path).
- `go build ./...`, `go vet ./internal/cmd/...`, `gofmt -l`, `make test`, `make lint` all clean.
- `go test -race ./...` green on the CI matrix (ubuntu-latest linux/amd64, macos-latest darwin/arm64,
  windows-latest windows/amd64) — the suite is cross-platform (tar.gz vs zip; .stagecoach-backup vs .old).
- `git status --porcelain` == exactly TWO new files (`cmd/stubversion/main.go` + `internal/cmd/upgrade_swap_test.go`).

## User Persona (if applicable)

**Target User**: The Stagecoach maintainer / release engineer who must trust `stagecoach upgrade` to swap
the direct-binary install atomically and safely (download a verified archive, back up the current binary,
rename the new one into place, clean up) without ever leaving a half-upgraded install (FR-U11). This suite
is the automated regression net for that happy path.
**Use Case**: A user on the direct-binary channel runs `stagecoach upgrade`; the suite proves it ends with
the new version installed, the old version backed up, and no staging litter.
**Pain Points Addressed**: FR-U5 (the full download→verify→extract→sanity→swap pipeline works),
FR-U7 (atomic per-platform swap + one-deep backup), FR-U8 (backup retained), FR-U9 (--yes non-interactive
proceeds), FR-U11 (never half-upgraded — abort-before-swap; staging cleaned on success), FR-U12 (walled
off — no commit-path/network-beyond-the-fake touch).

## Why

- **PRD §9.29 FR-U5/U7/U8/U9/U11 + §20.5**: §20.5's self-update scenario list requires the direct-binary
  self-swap happy path as a regression scenario. FR-U5 spells out the download→verify→extract→sanity→swap
  sequence; FR-U7 the atomic backup+rename; FR-U11 the "never half-upgraded" + staging-tempDir-cleanup-on-
  success contract. This suite is that scenario's automated net.
- **P1.M4.T2.S1 (LANDED) shipped runDirectSwap + the function seams specifically so this suite can run the
  real pipeline with no real network/subprocess/real-binary-rename.** The seam design (upgradeBaseURL +
  upgradeDetect + upgradeSwap + SetCurrentVersion) is the contract this suite consumes — it is the FR-U12
  test wall's second customer (after S1's --check suite).
- **Risk being guarded**: a regression where runDirectSwap mis-orders the steps (e.g. swap before sanity,
  or no backup), leaks the staging tempDir on success, swaps the wrong exe, or breaks the --yes non-
  interactive path. All caught here deterministically against the fake.

## What

Two new files, no production edits. A test-helper binary + a package-cmd test. The test sets up a temp
"installed" stub (v0.1.0), serves a v0.2.0 archive+checksums from an httptest fake, overrides the four
seams, drives `Execute(ctx)` with `["upgrade","--yes"]`, and asserts the swap's end-effects (installed now
v0.2.0, backup now v0.1.0, temp cleaned, exit 0).

### Success Criteria
- [ ] NEW file `cmd/stubversion/main.go` (`package main`): declares `var version = "dev"` and `main()`
      prints it verbatim (ignores argv). `-ldflags "-X main.version=v0.2.0"` bakes a release value. stdlib
      only; builds on linux/darwin/windows × amd64/arm64.
- [ ] NEW file `internal/cmd/upgrade_swap_test.go` (package `cmd`): a file doc comment names it the
      FR-U5/U7/U9/U11 direct-swap happy-path suite, the no-network seam (`upgradeBaseURL`), the
      detect→direct injection (`upgradeDetect`), the swap injection (`upgradeSwap` mini-swap + WHY it is a
      function seam), the version seam (`SetCurrentVersion`), and the dedicated-file rationale
      (S1 owns upgrade_check_test.go; S3 owns the failure/rollback suite).
- [ ] `TestUpgradeSwap_DirectHappyPath`: sets up the v0.1.0 installed stub; packs+serves the v0.2.0 archive
      + checksums; overrides the 4 seams; runs `["upgrade","--yes"]`; asserts ALL of:
      exit 0; stdout has `stagecoach upgraded to v0.2.0`; installed `--version` has `v0.2.0`; backup
      `--version` has `v0.1.0`; staging tempDir glob unchanged (cleaned).
- [ ] A shared seam-setup/restore helper (mirrors S1's `setupCheckSeams` discipline) restores
      `upgradeBaseURL`, `upgradeDetect`, `upgradeSwap` via `t.Cleanup` and calls
      `upgrade.SetCurrentVersion("dev")`; the test ALSO does `saveRootState/restoreRootState` +
      `resetFlags(upgradeCmd.Flags())` + isolated HOME (mirrors S1's driver).
- [ ] `go build ./...`, `go vet ./internal/cmd/...`, `gofmt -l`, `make test`, `make lint` clean.
- [ ] `git status --porcelain` == `cmd/stubversion/main.go` + `internal/cmd/upgrade_swap_test.go` ONLY.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the LANDED `runDirectSwap` flow (exact steps + the success Fprintf), the exact seam var names +
which are overridden vs. left at default, the EXACT reason `upgradeSwap` is a function seam (package cmd
can't reach the unexported `resolveCurrentExe`), the `cmd/stubversion` ldflags pattern (mirrors
main.go's own `var version`), the host-asset/archive/checksums formulas (mirrors the unexported
`assetName`/`checksumsName`), the cross-platform backup-suffix/format table, the httptest fake's 3 routes,
the `buildStubCLI`-style once-per-process build helper, the Execute-driver idiom (with the mandatory
`resetFlags(upgradeCmd.Flags())` gotcha), and the precise per-assertion checks.

### Documentation & References

```yaml
# MUST READ — the LANDED runDirectSwap (the pipeline under test) + the package-cmd seams (the override targets).
- file: internal/cmd/upgrade_run.go
  why: "runDirectSwap (line ~250) is the EXACT pipeline: os.MkdirTemp('stagecoach-upgrade-*') → ResolveTarget →
        StageNewBinary → confirmUpgrade(flagYes) → upgradeSwap → 'stagecoach upgraded to %s'. The seam vars
        (upgradeBaseURL/upgradeDetect/upgradeSwap/upgradeNewClient/upgradeExePath/upgradeToken/upgradeRollback)
        are declared at the top of the file (line ~50). prodNewClient reads upgradeBaseURL at call time."
  critical: "the file doc (lines ~30-46) explains WHY upgradeSwap is a FUNCTION seam: upgrade.Swap resolves the
             exe via the package-upgrade UNEXPORTED resolveCurrentExe seam, which package cmd CANNOT reach, so
             upgrade_run.go designed upgradeSwap as the seam 'P1.M4.T3 overrides … to point at a temp-dir exe'.
             Override upgradeSwap (do NOT add an exported seam). upgradeExePath feeds ONLY prodDetect (NOT
             reached when upgradeDetect is overridden), so leave it at default."

# MUST READ — the runUpgrade prologue + dispatch (how Execute reaches runDirectSwap on the normal path).
- file: internal/cmd/upgrade.go
  why: "runUpgrade (line ~156): validateUpgradeFlags → config.LoadUpgradeConfig (isolated HOME needed) →
        resolve effChannel (default 'stable' from Defaults)/effSourceRepo (default 'dabstractor/stagecoach')
        → dispatchUpgrade. dispatchUpgrade (line ~230): client := upgradeNewClient(effSourceRepo,
        upgradeToken()); if flagCheck → runCheck; else ch,evidence,err := upgradeDetect(ctx, flagInstallMethod,
        log); if ch==upgrade.ChannelDirect || flagForce → runDirectSwap. args ['upgrade','--yes'] ⇒ flagYes=true,
        flagCheck=false, flagRollback=false ⇒ reaches runDirectSwap."
  gotcha: "LoadUpgradeConfig reads the GLOBAL config under $HOME/$XDG — isolate HOME (t.TempDir +
           XDG_CONFIG_HOME='') so it returns Defaults (no real config, no bootstrap). flagYes is bound to
           upgradeCmd.Flags() (LOCAL) — MUST resetFlags(upgradeCmd.Flags()) in t.Cleanup or it leaks."

# MUST READ — the existing command-test pattern (the Execute driver + state save/restore). S1's contract.
- file: plan/017_397abce9deb1/P1M4T3S1/PRP.md
  why: "S1 (the --check suite, being implemented in parallel) establishes the EXACT seam-overlay + Execute-driver
        pattern S2 consumes: upgradeBaseURL=httptest.URL; upgrade.SetCurrentVersion(v) + restore to 'dev';
        saveRootState/restoreRootState + resetFlags(upgradeCmd.Flags()) + isolated HOME; drive Execute(ctx) with
        ['upgrade',...]; map exit via exitcode.For(err). S2 mirrors this and adds the swap seams
        (upgradeDetect, upgradeSwap). Treat S1 as the pattern source of truth."
- file: internal/cmd/upgrade_test.go
  why: "TestUpgradeCommand_NoBootstrapOutsideRepo is the verbatim driver template: saveRootState/restoreRootState
        + resetFlags(upgradeCmd.Flags()) + rootCmd.SetOut/SetErr/SetArgs + Execute(ctx). resetFlags(upgradeCmd.
        Flags()) is SEPARATE from restoreRootState (which resets only rootCmd flags) — flagYes/flagCheck leak
        across tests without it."

# MUST READ — StageNewBinary + extractBinary + sanityCheck (the REAL download→verify→extract→sanity under test).
- file: internal/upgrade/stage.go
  why: "StageNewBinary: FetchChecksums (finds *_checksums.txt asset, downloads its URL, parses '<64hex>  <file>')
        → DownloadFile (ABSOLUTE url) → VerifySHA256 → extractBinary (format from asset suffix: .zip⇒archive/zip,
        else .tar.gz⇒archive/tar+gzip; pulls ONLY the 'stagecoach'(.exe) entry to tempDir/'new-stagecoach'(.exe))
        → sanityCheck (execVersion seam runs '<newBin> --version', asserts output CONTAINS release.Tag)."
  critical: "sanityCheck uses the DEFAULT execVersion (no override needed): stubversion bakes its version, so
             '<newBin> --version' prints 'v0.2.0' which CONTAINS release.Tag 'v0.2.0' ⇒ passes. The asset NAME
             must EXACTLY match SelectAsset's expected goreleaser name (download.go::assetName) or
             ResolveTarget⇒SelectAsset returns ErrNoMatchingAsset."

# MUST READ — the httptest fake + archive-packing + stub-build patterns (PORT these to package cmd).
- file: internal/upgrade/stage_test.go
  why: "THE TEMPLATE for: buildStubCLI (once-per-process `go build` with Dir=GOMOD's dir, -buildvcs=false);
        packArchive (pack stub bytes under host entry base + a throwaway README entry, .zip or .tar.gz by asset
        suffix, returns archiveBytes+sha256hex); archiveServer (httptest mux: /archive→bytes, /checksums→body);
        exeSuffix()/hostAssetName()/hostEntryName() (host-native name/entry helpers). PORT copies into
        upgrade_swap_test.go (they are package-upgrade/unexported — package cmd cannot import them)."
  pattern: "packArchive's tar header Mode=0o755 + the README-skip entry (proves extract pulls ONLY stagecoach).
            The checksums body is '<sha>  <assetName>\n'. buildStubCLI skips t if `go` is not on PATH."

# MUST READ — the swap mechanics + cross-platform backup suffix (the mini-swap's contract).
- file: internal/upgrade/swap.go
  why: "Swap: resolveCurrentExe (UNEXPORTED seam — package cmd CANNOT touch) → isWritable gate → platformSwap
        (backup+rename+restore-on-failure) → isTempDir-guarded os.RemoveAll(filepath.Dir(newBinPath)). The mini-
        swap closure replicates platformSwap's success path (backup→rename→tempDir-cleanup) against the captured
        temp stub. On failure it restores (best-effort) like platformSwap."
- file: internal/upgrade/swap_unix.go
  why: "backupPath(exe) = exe + '.stagecoach-backup' (unix). platformSwap = os.Rename(exe,backup) then
        os.Rename(new,exe). The contract's '.stagecoach-backup' is THIS unix suffix."
- file: internal/upgrade/swap_windows.go
  why: "backupPath(exe) = exe + '.old' (windows). The mini-swap + the test's backup assertion MUST use '.old'
        on windows (not '.stagecoach-backup'). os.Rename on regular temp files works (no running-image lock —
        that only applies to the ACTUAL running .exe)."

# MUST READ — the GitHub Releases JSON shape + the asset/checksums name formulas (the fake's payload).
- file: internal/upgrade/releases.go
  why: "Client.LatestStable GETs /repos/{Repo}/releases/latest and decodes ghRelease{tag_name,prerelease,draft,
        assets[]}⇒Release{Tag,Assets}; ghAsset{name,browser_download_url,size}⇒Asset{Name,DownloadURL,Size}.
        Repo='dabstractor/stagecoach' (Defaults). The fake serves that path with the host asset + checksums
        asset, both with ABSOLUTE browser_download_urls pointing back at the fake."
- file: internal/upgrade/download.go
  why: "assetName(tag,goos,goarch) = 'stagecoach_'+TrimPrefix(tag,'v')+'_'+goos+'_'+goarch + ('.zip' if windows
        else '.tar.gz') — UNEXPORTED, so the test computes its OWN twin. checksumsName(tag) =
        'stagecoach_'+TrimPrefix(tag,'v')+'_checksums.txt'. FetchChecksums matches the exact name OR any
        *_checksums.txt suffix; DownloadFile uses ABSOLUTE urls. VerifySHA256 lowercases+trims the want digest."

# MUST READ — the EXPORTED version seam (pins current; mirrors S1) + Compare.
- file: internal/upgrade/version.go
  why: "SetCurrentVersion(v) sets package var currentVersion (ignored if v==''; 'dev' accepted). The test process
        never runs main.go ⇒ currentVersion starts at 'dev'. Pin '0.1.0' per test (for displayCurrent()'s confirm
        prompt); restore via t.Cleanup(SetCurrentVersion('dev')). NOT load-bearing for --yes (confirmUpgrade
        short-circuits on flagYes), but set it for fidelity."

# MUST READ — exit codes + the confirm --yes short-circuit.
- file: internal/exitcode/exitcode.go
  why: "Success=0, Error=1, UpdateAvailable=6. For(err): nil→0. runDirectSwap success ⇒ nil ⇒ For==0."
- file: internal/cmd/upgrade_prompt.go
  why: "confirmUpgrade(..., assumeYes=true,...) ⇒ (true,nil) — NO prompt printed, NO stdin read. So
        ['upgrade','--yes'] proceeds without a TTY. (Non-TTY + no --yes would REFUSE with exit 1 — the stricter-
        than-integrate rule — but --yes bypasses it.)"

# MUST READ — the FR-U5/U7/U11 spec + §20.5 scenario this suite automates + the ldflags version pattern.
- docfile: plan/017_397abce9deb1/prd_snapshot.md
  section: "§9.29 FR-U5 (download→verify→extract→sanity→swap) + FR-U7 (atomic per-platform swap + one-deep
            backup) + FR-U8 (backup) + FR-U9 (--yes) + FR-U11 (never half-upgraded; staging cleaned on success)
            + §20.5 (self-update scenarios) + §15.4 (exit codes)"
  why: "FR-U5 spells out the exact pipeline sequence; FR-U11 the staging-tempDir-cleanup-on-success contract;
        §20.5 names the direct-swap happy path as a required regression scenario."
- docfile: plan/017_397abce9deb1/P1M4T3S2/research/findings.md
  why: "§3 (the swap decision: override upgradeSwap, WHY not an exported seam), §4 (why cmd/stubversion is
        required, stubcli can't), §5 (cross-platform table), §6 (fake server shape), §7 (assertion matrix),
        §8 (scope fences)."
- file: cmd/stagecoach/main.go
  why: "line 24: `var version = 'dev'` injected via -ldflags '-X main.version=…'. cmd/stubversion mirrors THIS
        exact pattern (the test bakes v0.1.0 / v0.2.0 via the same -X main.version= flag)."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  upgrade.go              # LANDED — runUpgrade + dispatchUpgrade (REACHED on ['upgrade','--yes']); READ-ONLY
  upgrade_run.go          # LANDED (P1.M4.T2.S1) — runDirectSwap + the package-cmd seams; READ-ONLY
  upgrade_prompt.go       # LANDED — confirmUpgrade (flagYes⇒(true,nil)); READ-ONLY
  upgrade_test.go         # LANDED (S1 reg/flags) — the Execute/saveRootState/resetFlags pattern; READ-ONLY
  upgrade_prompt_test.go  # LANDED (P1.M4.T1.S2) — READ-ONLY
  upgrade_check_test.go   # S1 (being implemented) — the --check suite; DO NOT TOUCH (S2 gets its own file)
  root_test.go            # READ-ONLY — saveRootState/restoreRootState/resetFlags helpers (package cmd)
  root.go                 # READ-ONLY — Execute(ctx)
internal/upgrade/
  releases.go             # READ-ONLY — Client/LatestStable/ghRelease/Asset/Release
  download.go             # READ-ONLY — assetName/checksumsName/SelectAsset/FetchChecksums/DownloadFile/VerifySHA256
  resolve.go              # READ-ONLY — ResolveTarget (stable⇒LatestStable⇒SelectAsset)
  stage.go                # READ-ONLY — StageNewBinary/extractBinary/sanityCheck/execVersion seam
  swap.go                 # READ-ONLY — Swap/resolveCurrentExe (UNEXPORTED)/isWritable/isTempDir
  swap_unix.go            # READ-ONLY — backupPath=".stagecoach-backup"/platformSwap (unix)
  swap_windows.go         # READ-ONLY — backupPath=".old"/platformSwap (windows)
  stage_test.go           # READ-ONLY — buildStubCLI/packArchive/archiveServer/exeSuffix/hostAssetName TEMPLATE
  version.go              # READ-ONLY — SetCurrentVersion/CurrentSemver/Compare (EXPORTED version seam)
internal/exitcode/exitcode.go  # READ-ONLY — Success/Error/For
cmd/
  stagecoach/main.go      # READ-ONLY — `var version = "dev"` ldflags pattern (stubversion mirrors it)
  stubcli/                # READ-ONLY — the ENV-controlled stub (CANNOT express two versions — see findings §4)
```

### Desired Codebase tree with files to be added

```bash
cmd/stubversion/main.go                # NEW — ldflags-baked version stub (test-helper binary; prints `version`)
internal/cmd/upgrade_swap_test.go      # NEW — the FR-U5/U7/U9/U11 direct-swap happy-path suite
# NOTHING ELSE. Zero edits to internal/upgrade/* or internal/cmd/upgrade*.go (S1/S3 own their own files).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (upgradeSwap is a FUNCTION seam — package cmd CANNOT reach upgrade.Swap's exe): upgrade.Swap resolves
// the running exe via the package-upgrade, UNEXPORTED `resolveCurrentExe` seam (swap.go). Package cmd cannot
// override it, so upgrade_run.go explicitly designed `upgradeSwap` (default upgrade.Swap) as the function seam
// to override "to point at a temp-dir exe" (upgrade_run.go file doc, lines ~30-46). OVERRIDE upgradeSwap with a
// closure that backs up + renames + cleans the staging tempDir against the captured temp-stub path. Do NOT add
// an exported resolveCurrentExe seam (would edit a Complete milestone file + break S1's test-only precedent).
// The REAL upgrade.Swap is exhaustively covered by swap_test.go/swap_windows_test.go.

// CRITICAL (cmd/stubcli CANNOT express two versions): stubcli prints STAGECOACH_STUBCLI_OUT (env-controlled), so
// two identical stubcli binaries print whatever ONE global env is set — you cannot make the backup print v0.1.0
// while the installed prints v0.2.0. Byte-comparison also fails (identical bytes). ADD cmd/stubversion with a
// BAKED `var version` (ldflags -X main.version=…), built TWICE (v0.1.0 installed; v0.2.0 packed as the new
// payload). The sanity-run then works with the DEFAULT execVersion (stubversion prints its baked version).

// CRITICAL (the asset NAME must EXACTLY match SelectAsset): ResolveTarget→SelectAsset computes the expected
// goreleaser name via download.go::assetName (UNEXPORTED): "stagecoach_" + TrimPrefix(tag,"v") + "_" + GOOS +
// "_" + GOARCH + (".zip" if windows else ".tar.gz"). The fake's release JSON MUST list an asset with this EXACT
// name (e.g. "stagecoach_0.2.0_linux_amd64.tar.gz") or SelectAsset returns ErrNoMatchingAsset. The test computes
// its OWN twin of assetName (it cannot call the unexported one). checksumsName = "stagecoach_" + TrimPrefix(tag,
// "v") + "_checksums.txt".

// CRITICAL (DownloadURLs are ABSOLUTE): Client.DownloadFile/FetchChecksums use the asset's browser_download_url
// verbatim (NO BaseURL prefix — BaseURL is metadata-only for asset downloads). So the fake's release assets MUST
// carry ABSOLUTE urls: "<ts.URL>/archive" and "<ts.URL>/checksums". A relative "/archive" would fail to dial.

// CRITICAL (cross-platform backup SUFFIX): the backup path is installed+".stagecoach-backup" on unix and
// installed+".old" on windows (swap_unix.go/swap_windows.go backupPath). The contract's ".stagecoach-backup" is
// the unix shorthand. The mini-swap AND the test's backup assertion MUST use the platform-correct suffix via
// runtime.GOOS. Asserting ".stagecoach-backup" on windows WILL fail (the file is ".old" there).

// CRITICAL (resetFlags(upgradeCmd.Flags()) is SEPARATE from restoreRootState): restoreRootState resets
// rootCmd.Flags()+PersistentFlags() but NOT upgradeCmd's LOCAL flags. flagYes/flagCheck/flagChannel are bound to
// upgradeCmd.Flags(); without `defer resetFlags(upgradeCmd.Flags())` they leak across tests. Mirror
// upgrade_test.go's two-defer idiom (S1's PRP documents this too).

// CRITICAL (isolated HOME): runUpgrade calls config.LoadUpgradeConfig() which reads the GLOBAL config under
// $HOME/$XDG_CONFIG_HOME. Without isolation a real ~/.config/stagecoach/config.toml could set [upgrade].channel/
// source_repo and change effChannel/effSourceRepo (and the fake path). Isolate: t.Setenv("HOME", t.TempDir());
// t.Setenv("XDG_CONFIG_HOME", "") ⇒ LoadUpgradeConfig returns Defaults{Channel:"stable",
// SourceRepo:"dabstractor/stagecoach"} (never bootstraps).

// GOTCHA (NO execVersion override needed): stubversion bakes its version, so StageNewBinary's sanityCheck runs
// '<newBin> --version' via the DEFAULT execVersion and the output ("v0.2.0\n") CONTAINS release.Tag ("v0.2.0").
// Overriding execVersion would be dead code (it is package-upgrade; package cmd cannot reach it anyway).

// GOTCHA (the staging tempDir is created INSIDE runDirectSwap, not by the test): runDirectSwap does
// os.MkdirTemp("","stagecoach-upgrade-*") (the staging dir). StageNewBinary writes the archive +
// new-stagecoach(.exe) into it. The mini-swap's os.RemoveAll(filepath.Dir(newBin)) removes it (newBin =
// <stagingDir>/new-stagecoach(.exe)). To ASSERT cleanup, glob os.TempDir()/"stagecoach-upgrade-*" BEFORE+AFTER
// Execute and assert unchanged (the created dir was removed). Do NOT use t.TempDir() for the staging dir — it is
// internal to runDirectSwap.

// GOTCHA (the installed stub dir is SEPARATE from the staging dir): the test's "installed" stub lives in its OWN
// t.TempDir() (e.g. <tmp>/stagecoach(.exe)); the staging dir is the os.MkdirTemp runDirectSwap created. The
// mini-swap removes filepath.Dir(newBin) (the staging dir), NOT the installed-stub dir. Keep them separate or
// the cleanup nukes the backup you want to assert on.

// GOTCHA (the real test binary is never touched): the mini-swap operates ONLY on the captured temp-stub path.
// It never calls os.Executable()/resolveCurrentExe, so the running `go test` binary is safe. Do NOT leave
// upgradeSwap at its default (upgrade.Swap) — that would resolve + rename the REAL test binary (catastrophic).

// GOTCHA (buildStubVersion must be CWD-independent + cross-platform): mirror stage_test.go::buildStubCLI:
// resolve the module root via `go env GOMOD` and set build.Dir = filepath.Dir(gomod) (tests may chdir); use
// -buildvcs=false; pick the .exe suffix on windows for the -o path; build "github.com/dabstractor/stagecoach/
// cmd/stubversion". Skip t if `go` is not on PATH (CI always has it). Cache per-version with sync.Once to avoid
// rebuilding across multiple S2 tests.

// GOTCHA (Go runs t.Cleanup LIFO): if setupSwapSeams registers the SetCurrentVersion("dev") restore AND the test
// then calls SetCurrentVersion("0.1.0"), the "dev" reset registered FIRST runs LAST ⇒ final state "dev" (correct).
// Prefer: register ALL seam restores in the helper, then do the per-test SetCurrentVersion after. Or register a
// per-call restore. Either is fine — just ensure the FINAL unwind leaves "dev".
```

## Implementation Blueprint

### Data models and structure

None beyond the test helpers. The test file declares only: a `versionSuffix()`/`backupSuffix()`/`exeSuffix()`
helper set, `hostAssetName(tag)`/`hostEntryName()`/`checksumsName(tag)` host-native name helpers, a cached
`buildStubVersion(t, version)`, a `packSwapArchive(t, stubPath, tag)` (returns archiveBytes+sha), a
`newSwapFake(t, tag, archiveBytes, sha)` httptest builder, a `setupSwapSeams(t, tsURL, installedExe)`
restore helper, and the `runUpgradeSwap(t)` Execute-driver. cmd/stubversion/main.go is a 6-line `package main`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE cmd/stubversion/main.go (NEW test-helper binary)
  - PURPOSE: a cross-platform compiled stub whose --version prints a BUILD-TIME ldflags-baked `version`. Built
    TWICE by the test (v0.1.0 = installed; v0.2.0 = packed as the new payload). stubcli (env-controlled) CANNOT
    express two versions — stubversion can (findings §4).
  - PACKAGE DOC: "Command stubversion is a tiny version-printing stub for Stagecoach's `upgrade` direct-swap
    e2e test (P1.M4.T3.S2). It prints a build-time ldflags-baked `version` var verbatim on any invocation,
    ignoring argv (stagecoach's sanityCheck runs '<bin> --version'). STDLIB ONLY. A compiled binary runs on
    every OS (linux/darwin/windows); the env-controlled cmd/stubcli cannot distinguish two versions from one
    global env, so this stub bakes its version via -ldflags '-X main.version=…' (mirrors cmd/stagecoach/main.go)."
  - IMPLEMENT: `var version = "dev"` + `func main() { fmt.Println(version) }` (fmt only). Ignore os.Args.
  - NAMING: package main; file main.go; the `-X main.version=<v>` target is the unexported `version` var.
  - PLACEMENT: cmd/stubversion/main.go (mirrors cmd/stubcli/, cmd/stubagent/ convention).

Task 2: CREATE internal/cmd/upgrade_swap_test.go — file doc + imports + host-native helpers + buildStubVersion
  - PACKAGE DOC (file comment): "FR-U5/U7/U9/U11 direct-swap happy-path suite (P1.M4.T3.S2). Drives runUpgrade
    via Execute with args ['upgrade','--yes'] against an httptest fake GitHub Releases server, overriding the
    package-cmd seams: upgradeBaseURL (the NETWORK seam — prodNewClient reads it), upgradeDetect (→ChannelDirect,
    'injected Detect→direct'), upgradeSwap (a faithful mini-swap closure — package cmd CANNOT reach
    upgrade.Swap's unexported resolveCurrentExe seam, so upgrade_run.go designed upgradeSwap as the function seam
    to override), and upgrade.SetCurrentVersion (the EXPORTED version seam). NO real network, NO real binary
    rename of the test runner. The REAL download→verify→extract→sanity runs via StageNewBinary against the fake;
    the backup→rename→cleanup runs via the mini-swap (the real upgrade.Swap is covered by swap_test.go). Dedicated
    file: S1 owns upgrade_check_test.go; S3 owns the failure/rollback suite."
  - IMPORTS: archive/tar, archive/zip, bytes, compress/gzip, context, crypto/sha256, encoding/hex, fmt, io,
    net/http, net/http/httptest, os, os/exec, path/filepath, runtime, strings, sync, testing, + internal/exitcode,
    internal/upgrade. (NO cobra/pflag — Execute + saveRootState/restoreRootState/resetFlags are same-package.)
  - HELPERS (host-native twins of the unexported package-upgrade helpers):
      exeSuffix() string            // ".exe" on windows else ""
      backupSuffix() string         // ".stagecoach-backup" on unix, ".old" on windows (MIRRORS swap_*.go backupPath)
      hostAssetName(tag) string     // "stagecoach_"+TrimPrefix(tag,"v")+"_"+GOOS+"_"+GOARCH + zip/tar.gz (twin of assetName)
      hostEntryName() string        // "stagecoach"+exeSuffix() (the archive's binary entry base)
      checksumsName(tag) string     // "stagecoach_"+TrimPrefix(tag,"v")+"_checksums.txt" (twin of download.go::checksumsName)
  - HELPER buildStubVersion(t, version string) string:
      sync.Once-PER-VERSION cache (map[string]string guarded by a mutex, OR a sync.Once per pinned version — the
      happy-path test uses exactly v0.1.0 + v0.2.0). CLONE of stage_test.go::buildStubCLI: exec.LookPath("go")
      (t.Skipf if absent); resolve module root via `go env GOMOD` (TrimSpace; skip the "/dev/null" sentinel) and
      set build.Dir=filepath.Dir(gomod) (CWD-independence); os.MkdirTemp("","stagecoach-stubversion-*") for the
      -o path; name="stubversion-"+version+exeSuffix(); `go build -buildvcs=false -ldflags "-X main.version=<v>"
      -o <out> github.com/dabstractor/stagecoach/cmd/stubversion`; CombinedOutput on error (t.Fatalf). Return out.
  - NAMING: all unexported, package cmd.

Task 3: CREATE the archive-packing + fake-server helpers in upgrade_swap_test.go
  - HELPER packSwapArchive(t, stubPath, tag string) (archiveBytes []byte, shaHex string):
      PORT of stage_test.go::packArchive (package cmd copy). Read stubPath bytes. var buf bytes.Buffer.
      if strings.HasSuffix(hostAssetName(tag),".zip"): zip.NewWriter; add a "README.md" throwaway entry (proves
        extract pulls ONLY stagecoach); add hostEntryName() via CreateHeader with SetMode(0o755); write stub bytes;
        Close. else: gzip.NewWriter+tar.NewWriter; README entry (0o644); hostEntryName() tar header (Mode 0o755,
        Size len(stubBytes)); write stub bytes; Close both. archive=buf.Bytes(); sum=sha256.Sum256(archive);
        return archive, hex.EncodeToString(sum[:]).
  - HELPER newSwapFake(t, tag string, archiveBytes []byte, shaHex string) *httptest.Server:
      Build the release JSON: `{"tag_name":"<tag>","name":"Release <v>","prerelease":false,"draft":false,
       "assets":[{"name":"<hostAssetName>","browser_download_url":"<ts.URL>/archive","size":<lenArchive>},
                 {"name":"<checksumsName>","browser_download_url":"<ts.URL>/checksums","size":<lenBody>}]}`
      (NOTE: the URLs need ts.URL, so build the handler as a CLOSURE capturing ts after httptest.NewServer, OR
       build the body inside the handler using a pre-computed ts.URL. Cleanest: create the server with a handler
       that rebuilds the JSON from r.Host — or capture ts.URL via a two-step: declare `var ts *httptest.Server`,
       assign, then set ts.Config.Handler. Simplest robust pattern: a helper that takes ts.URL as a param and
       builds a fixed handler — but ts.URL is only known after NewServer. Use: `ts := httptest.NewServer(nil);
       ts.Config.Handler = handler(ts.URL)` — Config is settable.)
      mux := http.NewServeMux():
        "/repos/dabstractor/stagecoach/releases/latest" (match suffix "/releases/latest") → 200 + release JSON
          (Content-Type application/json). Optional: assert method GET.
        "/archive" → 200 + archiveBytes (application/octet-stream; io.Copy from bytes.Reader).
        "/checksums" → 200 + "<shaHex>  <hostAssetName>\n" (text/plain).
      t.Cleanup(ts.Close). Return ts.
      (Mirror stage_test.go::archiveServer + releases_test.go::newFakeClient; both are package-upgrade but the
       SHAPE is what to copy. The host-asset-name + checksums-name must be computed for THIS tag.)

Task 4: CREATE the seam-setup helper + the Execute-driver in upgrade_swap_test.go
  - HELPER setupSwapSeams(t, tsURL, installedExe string):
      Captures + restores the 4 seams via t.Cleanup (LIFO):
        (a) upgradeBaseURL ← tsURL; restore "" (the default).
        (b) upgradeDetect ← func(ctx,override,log)(upgrade.Channel,string,error){
              return upgrade.ChannelDirect, "direct", nil }; restore prodDetect (the original default).
        (c) upgradeSwap ← the mini-swap closure (see Implementation Patterns); restore upgrade.Swap (original).
        (d) upgrade.SetCurrentVersion("0.1.0"); t.Cleanup(upgrade.SetCurrentVersion("dev")).
      NOTE: register the restores BEFORE the per-call sets where order matters; Go LIFO ⇒ the last-registered
      restore runs first. The per-call SetCurrentVersion has no own restore — rely on the helper's "dev" reset
      running last (it was registered first). (Matches S1's setupCheckSeams discipline.)
  - HELPER (mini-swap closure): build it INSIDE setupSwapSeams (captures installedExe):
        return func(ctx context.Context, newBinPath string) error {
            if err := ctx.Err(); err != nil { return err }
            backup := installedExe + backupSuffix()
            if err := os.Rename(installedExe, backup); err != nil {  // one-deep backup (FR-U8)
                return fmt.Errorf("backup current binary: %w", err)
            }
            if err := os.Rename(newBinPath, installedExe); err != nil {  // atomic rename into place
                _ = os.Rename(backup, installedExe)  // FR-U11 restore (best-effort, like platformSwap)
                return fmt.Errorf("rename new binary into place: %w (restored from backup)", err)
            }
            _ = os.RemoveAll(filepath.Dir(newBinPath))  // staging tempDir cleanup (isTempDir-by-construction)
            return nil
        }
      (This is a FAITHFUL twin of upgrade.Swap's success path; it never calls resolveCurrentExe ⇒ the real test
       binary is safe. backupSuffix() gives the platform-correct name.)
  - HELPER (driver) runUpgradeSwap(t) (outBuf, errBuf *bytes.Buffer, err error):
      Mirrors upgrade_test.go::TestUpgradeCommand_NoBootstrapOutsideRepo: saveRootState/restoreRootState via
      t.Cleanup; t.Cleanup(resetFlags(upgradeCmd.Flags())); t.Setenv("HOME",t.TempDir())+t.Setenv("XDG_CONFIG_HOME","");
      outBuf,errBuf=&bytes.Buffer{},&bytes.Buffer{}; rootCmd.SetOut/SetErr; rootCmd.SetArgs(["upgrade","--yes"]);
      err=Execute(context.Background()); return.

Task 5: TestUpgradeSwap_DirectHappyPath  (the contract case)
  - BUILD + PACK + SERVE:
      oldStub := buildStubVersion(t, "v0.1.0"); newStub := buildStubVersion(t, "v0.2.0")
      archive, sha := packSwapArchive(t, newStub, "v0.2.0")
      ts := newSwapFake(t, "v0.2.0", archive, sha)
  - INSTALLED STUB (the "current" binary, in its OWN temp dir, SEPARATE from the staging dir):
      installDir := t.TempDir(); installedExe := filepath.Join(installDir, "stagecoach"+exeSuffix())
      copy oldStub bytes → installedExe (os.WriteFile; os.Chmod 0o755 on unix so it is runnable for the
      post-swap --version check; on windows 0o644 is fine for a regular file).
  - PRE-STATE (for the temp-cleaned assertion):
      before,_ := filepath.Glob(filepath.Join(os.TempDir(), "stagecoach-upgrade-*"))
  - SEAMS + DRIVE:
      setupSwapSeams(t, ts.URL, installedExe)
      outBuf, _, err := runUpgradeSwap(t)
  - ASSERT exit 0: `if got := exitcode.For(err); got != exitcode.Success { t.Fatalf(...) }` (print outBuf/errBuf
      on failure for diagnostics — a SelectAsset/StageNewBinary/swap failure surfaces here).
  - ASSERT success line: strings.Contains(outBuf.String(), "stagecoach upgraded to v0.2.0").
  - ASSERT installed swapped to NEW: run `<installedExe> --version` (os/exec; inherit env — stubversion ignores
      it); assert output contains "v0.2.0". (helper: runVersion(t, path) string.)
  - ASSERT backup created with OLD: backup := installedExe + backupSuffix(); run `<backup> --version`; assert
      output contains "v0.1.0".
  - ASSERT temp cleaned: after,_ := filepath.Glob(filepath.Join(os.TempDir(), "stagecoach-upgrade-*"));
      assert reflect.DeepEqual(before, after) (the staging dir runDirectSwap created was removed by the mini-swap).
  - ASSERT (optional, defense): the staging dir's archive/new-stagecoach do NOT linger — covered by the glob.

Task 6: VERIFY — build, vet, fmt, test (race, cross-platform), lint, scope
  - go build ./... ; go vet ./internal/cmd/... ./cmd/stubversion/... ; gofmt -l (empty)
  - go test ./internal/cmd/ -run TestUpgradeSwap -race -v    # the happy-path PASSES
  - go test ./internal/cmd/ -race                            # no sibling broke (seams/flags restored)
  - go test ./... -race                                      # full matrix-green suite (incl. cmd/stubversion compiles)
  - make test && make lint                                   # staticcheck/unused clean
  - git status --porcelain (grep guards — see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (cmd/stubversion/main.go — the ldflags-baked version stub; mirrors cmd/stagecoach/main.go:24):
package main

import "fmt"

// version is baked at build time via -ldflags "-X main.version=v0.2.0" (default "dev"). The upgrade
// direct-swap e2e builds this twice — once as the "installed" stub and once as the packed "new" payload —
// so each binary's --version reports a DISTINCT, build-time-fixed version (the env-controlled cmd/stubcli
// cannot express two versions from one global env). STDLIB ONLY ⇒ builds on linux/darwin/windows.
var version = "dev"

func main() {
	fmt.Println(version) // ignore argv; stagecoach's sanityCheck runs "<bin> --version"
}

// PATTERN (buildStubVersion — clone of stage_test.go::buildStubCLI, CWD-independent + cross-platform):
var (
	stubVerOnce sync.Once
	stubVerMu   sync.Mutex
	stubVerCache = map[string]string{} // version → compiled path
)
func buildStubVersion(t *testing.T, version string) string {
	t.Helper()
	stubVerMu.Lock()
	if p, ok := stubVerCache[version]; ok { stubVerMu.Unlock(); return p }
	stubVerMu.Unlock()
	goPath, err := exec.LookPath("go")
	if err != nil { t.Skipf("go toolchain not on PATH; cannot build stubversion: %v", err) }
	dir, _ := os.MkdirTemp("", "stagecoach-stubversion-*")
	name := "stubversion-" + version + exeSuffix()
	out := filepath.Join(dir, name)
	build := exec.Command(goPath, "build", "-buildvcs=false",
		"-ldflags", "-X main.version="+version, "-o", out,
		"github.com/dabstractor/stagecoach/cmd/stubversion")
	if b, err := exec.Command(goPath, "env", "GOMOD").Output(); err == nil {
		if gomod := strings.TrimSpace(string(b)); gomod != "" && gomod != "/dev/null" {
			build.Dir = filepath.Dir(gomod)
		}
	}
	if b, err := build.CombinedOutput(); err != nil { t.Fatalf("go build stubversion %s: %v\n%s", version, err, b) }
	stubVerMu.Lock(); stubVerCache[version] = out; stubVerMu.Unlock()
	return out
}

// PATTERN (the fake server — 3 routes; the release JSON carries ABSOLUTE asset URLs back at the fake):
func newSwapFake(t *testing.T, tag string, archiveBytes []byte, shaHex string) *httptest.Server {
	t.Helper()
	asset := hostAssetName(tag)
	// Two-step: create the server first so ts.URL is known, then install the handler that closes over it.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	releaseJSON := fmt.Sprintf(`{"tag_name":%q,"name":%q,"prerelease":false,"draft":false,"assets":[`+
		`{"name":%q,"browser_download_url":%q,"size":%d},`+
		`{"name":%q,"browser_download_url":%q,"size":%d}]}`,
		tag, "Release "+strings.TrimPrefix(tag,"v"), asset, ts.URL+"/archive", len(archiveBytes),
		checksumsName(tag), ts.URL+"/checksums", len(shaHex)+2+len(asset)+1)
	checkBody := fmt.Sprintf("%s  %s\n", shaHex, asset)
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, releaseJSON)
		case strings.HasSuffix(r.URL.Path, "/archive"):
			_, _ = io.Copy(w, bytes.NewReader(archiveBytes))
		case strings.HasSuffix(r.URL.Path, "/checksums"):
			_, _ = io.WriteString(w, checkBody)
		default:
			http.NotFound(w, r)
		}
	})
	t.Cleanup(ts.Close)
	return ts
}
// GOTCHA: the release JSON is built BEFORE the handler is installed but AFTER ts is created (ts.URL known).
// ts.Config.Handler is the documented way to (re)assign a server's handler post-construction.

// PATTERN (the mini-swap closure — a faithful twin of upgrade.Swap's success path; NEVER touches os.Executable):
miniSwap := func(ctx context.Context, newBinPath string) error {
	if err := ctx.Err(); err != nil { return err }
	backup := installedExe + backupSuffix()
	if err := os.Rename(installedExe, backup); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}
	if err := os.Rename(newBinPath, installedExe); err != nil {
		_ = os.Rename(backup, installedExe) // FR-U11 restore (best-effort)
		return fmt.Errorf("rename new binary into place: %w (restored from backup)", err)
	}
	_ = os.RemoveAll(filepath.Dir(newBinPath)) // staging tempDir cleanup (newBin = <staging>/new-stagecoach)
	return nil
}

// PATTERN (drive runUpgrade end-to-end + seam override — the S1 idiom, extended with swap seams):
func setupSwapSeams(t *testing.T, tsURL, installedExe string) {
	t.Helper()
	origBase := upgradeBaseURL; upgradeBaseURL = tsURL
	t.Cleanup(func() { upgradeBaseURL = origBase })
	origDetect := upgradeDetect; upgradeDetect = func(context.Context, string, func(string)) (upgrade.Channel, string, error) {
		return upgrade.ChannelDirect, "direct", nil
	}
	t.Cleanup(func() { upgradeDetect = origDetect })
	origSwap := upgradeSwap; upgradeSwap = func(ctx context.Context, newBinPath string) error {
		if err := ctx.Err(); err != nil { return err }
		backup := installedExe + backupSuffix()
		if err := os.Rename(installedExe, backup); err != nil { return fmt.Errorf("backup current binary: %w", err) }
		if err := os.Rename(newBinPath, installedExe); err != nil {
			_ = os.Rename(backup, installedExe)
			return fmt.Errorf("rename new binary into place: %w (restored from backup)", err)
		}
		_ = os.RemoveAll(filepath.Dir(newBinPath))
		return nil
	}
	t.Cleanup(func() { upgradeSwap = origSwap })
	upgrade.SetCurrentVersion("0.1.0")
	t.Cleanup(func() { upgrade.SetCurrentVersion("dev") })
}
func runUpgradeSwap(t *testing.T) (*bytes.Buffer, *bytes.Buffer, error) {
	t.Helper()
	_, oO, oE, oR := saveRootState(t)
	t.Cleanup(func() { restoreRootState(t, nil, oO, oE, oR) })
	t.Cleanup(func() { resetFlags(upgradeCmd.Flags()) }) // <-- SEPARATE from restoreRootState
	t.Setenv("HOME", t.TempDir()); t.Setenv("XDG_CONFIG_HOME", "")
	outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	rootCmd.SetOut(outBuf); rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"upgrade", "--yes"})
	return outBuf, errBuf, Execute(context.Background())
}

// PATTERN (run a stub's --version for the post-swap assertions — stubversion ignores env, just prints baked v):
func runVersion(t *testing.T, path string) string {
	t.Helper()
	out, err := exec.Command(path, "--version").Output()
	if err != nil { t.Fatalf("run %s --version: %v", path, err) }
	return string(out)
}
// ... outBuf, _, err := runUpgradeSwap(t)
// if got := exitcode.For(err); got != exitcode.Success { t.Fatalf("exit=%d stdout=%q stderr=%q", got, outBuf, errBuf) }
// if !strings.Contains(outBuf.String(), "stagecoach upgraded to v0.2.0") { t.Errorf(...) }
// if !strings.Contains(runVersion(t, installedExe), "v0.2.0") { t.Errorf("installed not swapped to new") }
// if !strings.Contains(runVersion(t, installedExe+backupSuffix()), "v0.1.0") { t.Errorf("backup not old") }
```

### Integration Points

```yaml
CONSUMES (LANDED — READ-ONLY, zero edits):
  - internal/cmd (same package): upgradeBaseURL, upgradeDetect, upgradeSwap, upgradeNewClient, upgradeToken,
    upgradeExePath (seam vars, upgrade_run.go); runUpgrade/dispatchUpgrade/runDirectSwap (upgrade.go/
    upgrade_run.go); flagYes/flagCheck/flagRollback/flagTargetVersion/flagChannel/flagForce (upgrade.go, set by
    cobra on ['upgrade','--yes']); saveRootState/restoreRootState/resetFlags/Execute/rootCmd/upgradeCmd (root.go/
    root_test.go).
  - internal/upgrade: ChannelDirect (the channel const); SetCurrentVersion (version.go — EXPORTED seam);
    ResolveTarget/Client/LatestStable/Release/Asset (resolve.go/releases.go — driven via the fake);
    StageNewBinary/extractBinary/sanityCheck (stage.go — REAL, against the fake).
  - internal/exitcode: Success/Error/For (exitcode.go).
  - stdlib: archive/tar, archive/zip, compress/gzip, crypto/sha256, encoding/hex, net/http, net/http/httptest,
    os, os/exec, path/filepath, runtime, sync, io, bytes, fmt, strings, context, testing.
  - cmd/stubversion (NEW, Task 1): built by the test via `go build -ldflags "-X main.version=<v>"`.
NO database / migration / routes / config-struct change / exitcode-const change / go.mod change / docs change /
production-code change. The ONLY non-test file is cmd/stubversion/main.go (a test-helper binary, stdlib main).
SCOPE FENCES:
  - Touches ONLY: cmd/stubversion/main.go (NEW) + internal/cmd/upgrade_swap_test.go (NEW).
  - Does NOT edit: internal/cmd/upgrade*.go (LANDED), internal/upgrade/* (read-only — P1.M1–M3 Complete),
    internal/cmd/upgrade_check_test.go (S1's file), root.go, exitcode.go, config, the commit path (FR-U12),
    go.mod, any PRD/task file, cmd/stubcli/* (stubversion is a SEPARATE stub, not an edit to stubcli).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (the new test helper compiles on every target + the test file compiles against the LANDED seams).
go build ./...
# Expected: clean. Watch: "upgradeSwap undefined" (it's in upgrade_run.go, same package — confirm),
#           "upgrade.ChannelDirect undefined" (exported const — confirm), "imported and not used".

# Vet the changed packages (test files included).
go vet ./internal/cmd/... ./cmd/stubversion/...
# Expected: clean.

# Format.
gofmt -l cmd/stubversion/main.go internal/cmd/upgrade_swap_test.go
# Expected: empty. If listed: gofmt -w them.

# Lint (staticcheck/unused/errcheck/gosimple). The test file must have no U1000 (every helper/import used;
# the cached map is read; t.Cleanup restores satisfy errcheck). cmd/stubversion has no unused (`version` printed).
make lint
# Expected: zero errors.

# Scope guard: ONLY the two new files.
git status --porcelain
# Expected: cmd/stubversion/main.go + internal/cmd/upgrade_swap_test.go ONLY. FAIL if any production file appears.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Run the new direct-swap happy-path suite.
go test ./internal/cmd/ -run TestUpgradeSwap -race -v
# Expected: TestUpgradeSwap_DirectHappyPath PASSES. No network (the fake is localhost; upgradeBaseURL=localhost).
#           On Linux: tar.gz archive, .stagecoach-backup backup. On Windows: zip archive, .old backup.

# Full cmd-package regression (the seams are restored via t.Cleanup, so no leak into S1's --check suite or
# the registration tests).
go test ./internal/cmd/ -race
# Expected: green (S1's TestUpgradeCheck_* + TestUpgradeCommand_* + upgrade_prompt tests unaffected).

# Full race suite + lint + build (the CI matrix command).
go test -race ./...
# Expected: all green (incl. cmd/stubversion compiles; internal/upgrade/*_test.go unaffected).

make test && make lint && make build
# Expected: all green.
```

### Level 3: Integration Testing (System Validation)

```bash
# (This suite IS the integration test for the direct-swap happy path — it drives runUpgrade end-to-end via
#  Execute against the fake, with the REAL StageNewBinary download/verify/extract/sanity + a faithful mini-swap.
#  There is no separate "service" to start.) Optional manual cross-check against the REAL GitHub API
#  (network required; informational ONLY — not part of the suite):
make build
./bin/stagecoach upgrade --check; echo "exit=$?"   # dev build ⇒ exit 0 informational (proves the client works)
# (The suite's happy path replicates the staging+swap against the fake with pinned v0.1.0→v0.2.0.)

# Offline-capability smoke (prove no real network): run the suite with DNS to api.github.com blocked.
go test ./internal/cmd/ -run TestUpgradeSwap -race   # passes because upgradeBaseURL=localhost (the httptest fake)
# (No command needed — the localhost fake is the proof. This line documents the no-network guarantee.)
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: ONLY the two new files; ZERO production edits.
git status --porcelain
test "$(git status --porcelain | wc -l)" -eq 2 && echo "OK: two files" || echo "FAIL: expected two files"
git diff --name-only | grep -vE '^(cmd/stubversion/main\.go|internal/cmd/upgrade_swap_test\.go)$' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
git diff --name-only | grep -qE '^internal/(upgrade|exitcode|cmd/(upgrade|upgrade_run|upgrade_prompt|upgrade_check_test|root)\.go)' && echo "FAIL: edited LANDED file" || echo "OK: production untouched"

# Guard 2: the happy-path test exists + the stub exists.
grep -q "func TestUpgradeSwap_DirectHappyPath" internal/cmd/upgrade_swap_test.go || echo "MISSING test"
grep -q "func main()" cmd/stubversion/main.go && grep -q "var version = \"dev\"" cmd/stubversion/main.go || echo "MISSING stubversion version var"

# Guard 3: the 4 load-bearing seams are exercised.
grep -n 'upgradeBaseURL = ts.URL\|upgradeBaseURL =' internal/cmd/upgrade_swap_test.go   # network seam
grep -n 'upgrade.ChannelDirect' internal/cmd/upgrade_swap_test.go                        # detect→direct injection
grep -n 'upgradeSwap =' internal/cmd/upgrade_swap_test.go                                # the swap function seam
grep -n 'upgrade.SetCurrentVersion' internal/cmd/upgrade_swap_test.go                    # version seam (set + restore)

# Guard 4: the Execute driver + the mandatory resetFlags(upgradeCmd.Flags()).
grep -n 'Execute(context.Background())' internal/cmd/upgrade_swap_test.go                # the driver
grep -n 'resetFlags(upgradeCmd.Flags())' internal/cmd/upgrade_swap_test.go               # LOCAL-flag reset (must be present)

# Guard 5: exit code asserted via exitcode.For on the Execute return (NOT os.Exit / NOT raw code).
grep -n 'exitcode.For(err)' internal/cmd/upgrade_swap_test.go                            # the success-exit assertion

# Guard 6: no real network — the fake is httptest (localhost), upgradeBaseURL set to it, no api.github.com.
grep -n 'httptest.NewServer\|httptest.Server' internal/cmd/upgrade_swap_test.go          # the fake
grep -n 'api.github.com' internal/cmd/upgrade_swap_test.go && echo "WARN: hardcoded api.github.com (must use the fake)" || echo "OK: no hardcoded API host"

# Guard 7: the cross-platform backup suffix + the temp-cleaned assertions.
grep -n 'backupSuffix\|\.stagecoach-backup\|\.old' internal/cmd/upgrade_swap_test.go     # platform-correct backup name
grep -n 'stagecoach-upgrade-\*\|filepath.Glob' internal/cmd/upgrade_swap_test.go         # temp-dir glob (cleaned)

# Guard 8: the success line + the version assertions.
grep -n 'stagecoach upgraded to v0.2.0' internal/cmd/upgrade_swap_test.go                # runDirectSwap success line
grep -n 'runVersion\|--version' internal/cmd/upgrade_swap_test.go                        # installed==v0.2.0 + backup==v0.1.0

# Guard 9: the suite is offline-capable (run it; the localhost fake IS the no-network proof).
go test ./internal/cmd/ -run TestUpgradeSwap -race   # PASS

# Guard 10 (cross-platform): the suite must NOT hardcode a unix-only suffix or format.
grep -n 'runtime.GOOS' internal/cmd/upgrade_swap_test.go   # the branch that picks .tar.gz/.zip + backup suffix
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./internal/cmd/... ./cmd/stubversion/...` clean; `gofmt -l` empty on both files
- [ ] `make lint` zero errors (no U1000 — every helper/import used; the cached map is read; t.Cleanup restores
      satisfy errcheck; cmd/stubversion has no unused)
- [ ] `go test ./internal/cmd/ -run TestUpgradeSwap -race -v` PASSES
- [ ] `go test ./internal/cmd/ -race` green (no sibling broke — S1's --check suite + registration tests
      unaffected; seams/flags restored between tests)
- [ ] `go test -race ./...` green (the CI matrix command; cmd/stubversion compiles)
- [ ] `make test` + `make build` green

### Feature Validation (the direct-swap happy path)
- [ ] exit 0: `exitcode.For(err) == exitcode.Success` (runDirectSwap success)
- [ ] stdout contains `stagecoach upgraded to v0.2.0` (runDirectSwap success Fprintf; v-prefixed tag)
- [ ] installed swapped to NEW: `<installedExe> --version` contains `v0.2.0` (the new stubversion renamed in)
- [ ] backup created with OLD: `<installedExe+backupSuffix()> --version` contains `v0.1.0` (FR-U8 one-deep)
- [ ] temp cleaned: `stagecoach-upgrade-*` glob UNCHANGED before==after Execute (FR-U11 success cleanup)
- [ ] no real network: upgradeBaseURL=httptest.URL (localhost); no api.github.com (grep guard 6)
- [ ] real test binary untouched: the mini-swap uses only the captured temp-stub path (never os.Executable)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `cmd/stubversion/main.go` (NEW) + `internal/cmd/upgrade_swap_test.go` (NEW)
      (grep guard 1)
- [ ] NO edit to internal/cmd/upgrade*.go (LANDED), internal/upgrade/* (read-only — P1.M1–M3 Complete),
      internal/cmd/upgrade_check_test.go (S1's file), root.go, exitcode.go, config, the commit path (FR-U12),
      go.mod, any PRD/task file, cmd/stubcli/* (stubversion is a SEPARATE stub)

### Cross-Platform Validation
- [ ] linux/darwin: tar.gz archive + `stagecoach` entry + `.stagecoach-backup` backup (CI ubuntu/macos)
- [ ] windows: zip archive + `stagecoach.exe` entry + `.old` backup (CI windows-latest)
- [ ] `go test -race ./...` green on ubuntu-latest, macos-latest, windows-latest (the CI matrix)

### Code Quality & Docs
- [ ] cmd/stubversion file doc names it the version-printing stub + the ldflags pattern + why stubcli can't
- [ ] upgrade_swap_test.go file doc names the suite (FR-U5/U7/U9/U11 direct-swap happy path), the 4 seams + WHY
      upgradeSwap is a function seam (package cmd can't reach resolveCurrentExe), the no-network guarantee, and
      the dedicated-file rationale (S1 owns upgrade_check_test.go; S3 owns failure/rollback)
- [ ] Each seam override is restored via t.Cleanup (upgradeBaseURL→"", upgradeDetect→prodDetect,
      upgradeSwap→upgrade.Swap, currentVersion→"dev", rootCmd state, upgradeCmd flags)
- [ ] The mini-swap is a FAITHFUL twin of upgrade.Swap's success path (backup→rename→restore-on-failure→tempDir
      cleanup) and NEVER calls os.Executable/resolveCurrentExe
- [ ] Assertions match runDirectSwap's REAL success Fprintf (`stagecoach upgraded to v0.2.0`) + the REAL swap
      end-effects (installed==v0.2.0, backup==v0.1.0, temp cleaned)

---

## Anti-Patterns to Avoid

- ❌ Don't leave `upgradeSwap` at its default (`upgrade.Swap`). It resolves the exe via the UNEXPORTED
  `resolveCurrentExe` seam (which returns the REAL test binary) and would rename the running `go test` binary —
  catastrophic. OVERRIDE `upgradeSwap` with the mini-swap closure that targets the temp stub. This is exactly
  what upgrade_run.go's file doc sanctions ("P1.M4.T3 overrides [upgradeSwap] … to point at a temp-dir exe").
- ❌ Don't try to call the real `upgrade.Swap` from package cmd to "test more". Package cmd CANNOT override
  `resolveCurrentExe` (it is unexported, package-upgrade), so the real Swap would target the test runner.
  Adding an exported seam would edit a Complete milestone file (swap.go, P1.M3.T2) and break S1's test-only
  precedent. The real upgrade.Swap is exhaustively covered by swap_test.go/swap_windows_test.go; S2 owns the
  runDirectSwap ORCHESTRATION + the swap SEAM, not a re-test of upgrade.Swap.
- ❌ Don't use `cmd/stubcli` for the two versions. It is ENV-controlled (prints STAGECOACH_STUBCLI_OUT), so two
  identical stubcli binaries print whatever ONE global env is set — you cannot make the backup print v0.1.0
  while the installed prints v0.2.0, and byte-comparison fails (identical bytes). ADD `cmd/stubversion` with a
  BAKED `var version` (ldflags -X main.version=…), built twice. (findings §4.)
- ❌ Don't override `execVersion`. stubversion bakes its version, so StageNewBinary's sanityCheck runs
  `<newBin> --version` via the DEFAULT execVersion and the output (`v0.2.0`) CONTAINS release.Tag (`v0.2.0`) ⇒
  passes. Overriding execVersion is dead code (and package cmd can't reach it anyway). Leave it at default.
- ❌ Don't assert the bare `.stagecoach-backup` backup on Windows. The backup path is platform-specific:
  `installed+".stagecoach-backup"` on unix, `installed+".old"` on windows (swap_*.go backupPath). The contract's
  `.stagecoach-backup` is the unix shorthand. Use `backupSuffix()` (branch on runtime.GOOS) for BOTH the
  mini-swap AND the backup assertion, or the windows-latest matrix job fails.
- ❌ Don't put the "installed" stub in the SAME temp dir as the staging dir. The mini-swap does
  `os.RemoveAll(filepath.Dir(newBin))` (the staging dir runDirectSwap created). If the installed stub lived
  there, the cleanup would nuke the backup you want to assert on. Put the installed stub in its OWN
  `t.TempDir()`.
- ❌ Don't omit `resetFlags(upgradeCmd.Flags())`. restoreRootState resets only rootCmd's flags; the upgrade
  LOCAL flags (flagYes/flagCheck/flagChannel) are bound to upgradeCmd.Flags() and leak across tests in the same
  binary without an explicit reset. Mirror upgrade_test.go's two-defer idiom (S1 documents this too).
- ❌ Don't leave HOME unisolated. runUpgrade calls LoadUpgradeConfig (reads the global config under
  $HOME/$XDG). A real config's [upgrade].channel/source_repo would change effChannel/effSourceRepo (and the
  fake path). Isolate with t.TempDir() + XDG_CONFIG_HOME="".
- ❌ Don't use a RELATIVE `browser_download_url` in the fake's release JSON. Client.DownloadFile/FetchChecksums
  use the url VERBATIM (BaseURL is metadata-only for asset downloads). The assets MUST carry ABSOLUTE urls
  (`<ts.URL>/archive`, `<ts.URL>/checksums`) or the download fails to dial. Build the JSON AFTER creating the
  server (so ts.URL is known) and install it via `ts.Config.Handler = …`.
- ❌ Don't mismatch the asset NAME. ResolveTarget→SelectAsset computes the expected goreleaser name via the
  unexported `assetName` (`stagecoach_<v>_<GOOS>_<GOARCH>.zip|.tar.gz`). The fake's release JSON MUST list an
  asset with this EXACT name or SelectAsset returns ErrNoMatchingAsset. Compute `hostAssetName(tag)` with the
  SAME formula in the test.
- ❌ Don't hardcode `api.github.com` anywhere in the test. The fake is `httptest.NewServer` (localhost) and
  `upgradeBaseURL = ts.URL` is the only network target. A hardcoded host would make a real network call
  (violating the no-network requirement) and flake offline.
- ❌ Don't add production code beyond cmd/stubversion/main.go. This item is TESTS-ONLY (+ one test-helper
  binary). runDirectSwap + the seams are LANDED (P1.M4.T2.S1); StageNewBinary/Swap/ResolveTarget are LANDED
  (P1.M1–M3). Any edit to upgrade.go/upgrade_run.go/internal/upgrade/* is out of scope. S1's upgrade_check_test.go
  is S1's file — do not touch it; S2 gets its own (upgrade_swap_test.go).