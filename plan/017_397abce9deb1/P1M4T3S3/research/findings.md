# P1.M4.T3.S3 — Research Findings

**Item:** Failure-safety + delegation-refuses-manager-owned + rollback (FR-U1/U8/U11).
**Suite:** the FAILURE / DELEGATION / ROLLBACK test file (S1 owns `--check`; S2 owns the direct-swap
happy path; **S3 owns failure-safety + delegation-refuses + rollback**).

## 1. What is ALREADY LANDED (READ-ONLY contracts S3 consumes)

S1 (`upgrade_check_test.go`) and S2 (`upgrade_swap_test.go` + `cmd/stubversion/main.go`) are BOTH
already on disk (verified). S3 REUSES S2's package-`cmd` helpers — it does NOT redefine them:

**S2 helpers (in `internal/cmd/upgrade_swap_test.go`, package `cmd` — directly callable by S3):**
- `exeSuffix() string` — `.exe` on windows else `""`.
- `backupSuffix() string` — `.stagecoach-backup` (unix) / `.old` (windows). **MIRRORS `swap_*.go` `backupPath`.**
- `hostAssetName(tag string) string` — twin of unexported `download.go::assetName`.
- `hostEntryName() string` — `stagecoach` + `exeSuffix()`.
- `checksumsName(tag string) string` — twin of unexported `download.go::checksumsName`.
- `buildStubVersion(t, version string) string` — compiles `cmd/stubversion` with `-ldflags -X main.version=<v>`, cached per version.
- `packSwapArchive(t, stubPath, tag string) ([]byte, string)` — packs stub bytes under `hostEntryName()` + a README throwaway entry; `.zip` (windows) / `.tar.gz` (unix); returns `(archiveBytes, sha256hex)`.
- `newSwapFake(t, tag, archiveBytes, shaHex string) *httptest.Server` — 3-route httptest fake (`/releases/latest`, `/archive`, `/checksums`); release JSON carries **ABSOLUTE** `browser_download_url`s back at the fake.
- `runVersion(t, path string) string` — runs `<path> --version`, returns stdout (fatals on error).
- `setupSwapSeams(t, tsURL, installedExe string)` — wires `upgradeBaseURL=tsURL`, `upgradeDetect→ChannelDirect`, `upgradeSwap=mini-swap`, `SetCurrentVersion("0.1.0")`; restores via `t.Cleanup` (LIFO). **REUSABLE AS-IS for cases (a)/(b)** (the swap is never reached there).
- `runUpgradeSwap(t) (outBuf, errBuf *bytes.Buffer, err error)` — drives `Execute(ctx)` with hardcoded `["upgrade","--yes"]`.

**S1 helpers (in `internal/cmd/upgrade_check_test.go`)** — `setupCheckSeams`/`runUpgradeCheck`; S3 does
NOT need them (S3's paths are swap/delegate/rollback, not `--check`), but S1 establishes the
driver idiom (`saveRootState`/`restoreRootState` + `resetFlags(upgradeCmd.Flags())` + isolated HOME).

**`cmd/stubversion/main.go`** (S2, LANDED) — `var version = "dev"` + `fmt.Println(version)`. S3 builds
it a THIRD time at `v0.3.0` for the wrong-version-sanity case (b): the archive is named/asset-tagged
`v0.2.0` but its embedded binary prints `v0.3.0`, so `sanityCheck` (which asserts output CONTAINS
`release.Tag` = `v0.2.0`) fails.

**Collision check (verified):** none of `recordingExecRunner`, `runUpgradeArgs`, `miniSwap`,
`failingSwap`, `miniRollback` exist anywhere in `internal/cmd/*.go` — all free for S3.

## 2. The seams S3 overrides (exact types — from `internal/cmd/upgrade_run.go`)

```go
var (
    upgradeBaseURL     string                                                         // NETWORK seam (fake URL)
    upgradeDetect      = prodDetect  // func(ctx, override string, log func(string)) (upgrade.Channel, string, error)
    upgradeExecRunner  upgrade.ExecRunner                                             // nil ⇒ osExecRunner (Delegate seam)
    upgradeSwap        = upgrade.Swap // func(ctx, newBinPath string) error            // FUNCTION seam
    upgradeRollback    = upgrade.Rollback // func(ctx) (string, error)                 // FUNCTION seam
)
```
- `(c)/(d)` override **`upgradeDetect`** → `(upgrade.ChannelBrew, "brew", nil)`.
- `(c)` injects **`upgradeExecRunner`** = a recording fake (records argv, returns canned `(code,err)`).
- `(d)` overrides **`upgradeSwap`** = mini-swap (swap happens) — brew + `--force` routes to `runDirectSwap`.
- `(c)` overrides **`upgradeSwap`** = a `t.Fatal` closure (PROVES the delegation path never swaps).
- `(e)` overrides **`upgradeRollback`** = either `("...", upgrade.ErrNoBackup)` (no-backup) or a
  mini-rollback closure (backup-present). Package cmd CANNOT reach `upgrade.Rollback`'s unexported
  `resolveCurrentExe` (returns the REAL test binary) — MUST override the seam var.

## 3. The dispatch logic S3 exercises (from `internal/cmd/upgrade.go::dispatchUpgrade` + `upgrade_run.go`)

- `--rollback`: `ver, err := upgradeRollback(ctx)`; `errors.Is(err, upgrade.ErrNoBackup)` ⇒
  `"no backup — nothing to roll back"` + exit 0; other err ⇒ exit 1; success ⇒ `"restored stagecoach %s\n"` + exit 0.
  **No `confirmUpgrade` on this path ⇒ `["upgrade","--rollback"]` needs NO `--yes`.**
- normal: `upgradeDetect(...)`; `if ch == upgrade.ChannelDirect || flagForce { … warning if
  flagForce && ch != Direct …; runDirectSwap }` else `runDelegate`.
- `runDirectSwap`: `MkdirTemp("stagecoach-upgrade-*")` → `ResolveTarget` → `StageNewBinary` (LEAVE
  tempDir on failure) → `confirmUpgrade(flagYes)` → `upgradeSwap`. **(a)/(b) fail inside
  `StageNewBinary` BEFORE `confirmUpgrade`/`upgradeSwap` ⇒ exit 1, no backup, on-disk unchanged.**
- `runDelegate`: `isRun := ch != ChannelAUR && ch != ChannelNix`; confirm (RUN only, `--yes`⇒true);
  `upgrade.Delegate(ctx, ch, DelegateOptions{Exec: upgradeExecRunner, Out: out, Env: os.Getenv,
  Verbose: …, Confirmed: flagYes})`; `!res.Ran` ⇒ print + exit 0; `res.ExitCode != 0` ⇒ exit 1; else
  `"stagecoach updated via %s\n"` + exit 0.

## 4. The exact failure sentinels (from `internal/upgrade/stage.go` + `download.go`)

- **(a) tampered archive:** `VerifySHA256(path, want)` = `subtle.ConstantTimeCompare(got, wantNorm)`
  where `wantNorm = strings.ToLower(strings.TrimSpace(want))`. A valid-hex-but-wrong digest
  (e.g. `strings.Repeat("a", 64)`) ⇒ `ErrChecksumMismatch`, propagated AS-IS by `StageNewBinary` ⇒
  `runDirectSwap` returns `exitcode.New(exitcode.Error, …)` ⇒ exit 1.
  **Trigger:** `newSwapFake(t, "v0.2.0", archive, strings.Repeat("a", 64))` (serve the real archive
  but a wrong sha in `/checksums`).
- **(b) wrong-version sanity:** `sanityCheck` runs `<newBin> --version` (DEFAULT `execVersion` —
  stubversion bakes its version) and asserts `bytes.Contains(out, []byte(release.Tag))`. A
  `v0.3.0` binary under a `v0.2.0` release ⇒ output `"v0.3.0\n"` lacks `"v0.2.0"` ⇒
  `ErrSanityVersionMismatch` ⇒ exit 1. **Trigger:** `buildStubVersion(t, "v0.3.0")` then
  `packSwapArchive(t, v0.3.0stub, "v0.2.0")` (real sha, real archive, WRONG embedded version).

## 5. The delegation argv (from `internal/upgrade/delegate.go::runArgv`)

`ChannelBrew ⇒ [][]string{{"brew", "upgrade", "stagecoach"}}`. `joinArgv` ⇒ `"brew upgrade stagecoach"`.
The injected `ExecRunner.Run` signature: `Run(ctx, stdout, stderr io.Writer, name string, args ...string) (int, error)`.
`Delegate` calls `runner.Run(ctx, out, out, "brew", "upgrade", "stagecoach")`. On `(0, nil)` ⇒
`DelegateResult{Ran:true, Command:"brew upgrade stagecoach", ExitCode:0}` ⇒ `runDelegate` prints
`"stagecoach updated via brew\n"` + exit 0.

## 6. (c) needs NO fake server

`runDelegate`/`Delegate` NEVER touch the releases `Client` (only `runCheck`/`runDirectSwap` do).
Leaving `upgradeBaseURL` at its `""` default and passing with NO httptest server is the STRONGEST
proof the delegate path doesn't network. (S3 wires ONLY `upgradeDetect→brew` +
`upgradeExecRunner=recorder` + `upgradeSwap=failingSwap(t)`.)

## 7. The 5 cases → exit-code + on-disk matrix

| case | args | detect | swap | execRunner | exit | stdout / on-disk |
|------|------|--------|------|------------|------|------------------|
| (a) tampered sha | `--yes` | direct | mini (never reached) | — | **1** | unchanged; **no backup**; temp left (FR-U11) |
| (b) wrong ver | `--yes` | direct | mini (never reached) | — | **1** | unchanged; **no backup**; temp left |
| (c) brew, no force | `--yes` | **brew** | **failingSwap** | **recorder(0,nil)** | **0** | recorded `brew upgrade stagecoach`; `"updated via brew"`; untouched |
| (d) brew, `--force` | `--yes --force` | **brew** | **mini (runs)** | — | **0** | stderr warning; swapped→v0.2.0; backup v0.1.0 |
| (e1) rollback no backup | `--rollback` | — | — | — | **0** | `"no backup — nothing to roll back"` |
| (e2) rollback has backup | `--rollback` | — | — | — | **0** | `"restored stagecoach v0.1.0"`; exe now v0.1.0 |

## 8. Spec anchors (PRD, verbatim)

- **FR-U1** (PRD line 594): "…MUST NOT overwrite a binary the detection logic believes a package
  manager owns, unless the user passes `--force` (which warns)."
- **FR-U8** (line 621): "`--rollback` restores the most recent backup… Without a backup it is a
  no-op reported as such."
- **FR-U11** (line 624): "…a checksum failure, or a sanity-run failure aborts BEFORE the
  backup/swap and leaves the installed binary byte-for-byte unchanged. … The only artifact of a
  failed upgrade is a temp file."
- **§20.5** (line 2172): "a corrupted/tampered asset (bad SHA256) and a new binary that exits
  non-zero on `--version` both abort BEFORE any swap with the on-disk binary unchanged (FR-U5 steps
  4 & 6, FR-U11); `--rollback` restores the backup; install-method detection is asserted to
  delegate to (not overwrite) a simulated Homebrew/npm/Scoop install path, and to refuse a
  self-swap of a manager-owned binary unless `--force` (FR-U1)."

## 9. Scope fences

S3 touches **ONE file**: `internal/cmd/upgrade_safety_test.go` (NEW, package `cmd`). It REUSES S2's
helpers + `cmd/stubversion` (no edit). ZERO production edits (no `upgrade*.go`, no `internal/upgrade/*`).
Does NOT touch S1's `upgrade_check_test.go` or S2's `upgrade_swap_test.go`.