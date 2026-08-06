# P1.M4.T3.S2 — Research Findings: Direct-swap happy path (download→verify→sanity→backup→rename)

> The SECOND of P1.M4.T3's three test suites. S1 (P1.M4.T3.S1) owns the `--check` suite
> (`internal/cmd/upgrade_check_test.go`); THIS item (S2) owns the direct-binary **swap happy-path**
> suite. S3 will own failure-safety + delegation-refuses-manager-owned + rollback. This file is the
> research backing the PRP — the PRP is the implementer's source of truth.

## 1. What S1 produces (treat as a CONTRACT — it is being implemented in parallel)

S1 adds `internal/cmd/upgrade_check_test.go` (package `cmd`) and proves the seam-overlay pattern for
package-cmd upgrade tests:
- `upgradeBaseURL` (the NETWORK seam in `upgrade_run.go`) → set to an `httptest.Server.URL`; `prodNewClient`
  reads it at call time so the releases Client dials localhost.
- `upgrade.SetCurrentVersion(v)` (the EXPORTED test seam in `internal/upgrade/version.go`) pins the running
  version; restored to `"dev"` via `t.Cleanup`.
- Driver: `Execute(ctx)` with `["upgrade","--check"]`, wrapped in `saveRootState/restoreRootState` +
  `resetFlags(upgradeCmd.Flags())` + isolated HOME.
- Exit mapping via `exitcode.For(err)` on the Execute return.

**S2 consumes the SAME seam-overlay pattern** but drives `["upgrade","--yes"]` (normal path) and needs the
swap seams too. S2 MUST NOT touch S1's file; S2 gets its OWN file: `internal/cmd/upgrade_swap_test.go`.

## 2. The LANDED seams S2 drives (READ-ONLY consumers — zero edits to these)

`internal/cmd/upgrade_run.go` declares the package-cmd seams (all READ by `runUpgrade`/`dispatchUpgrade`/
`runDirectSwap`, so no U1000):

| Seam (package `cmd`) | Type | Default | S2 override |
|---|---|---|---|
| `upgradeBaseURL` | `string` | `""` (⇒ api.github.com) | `ts.URL` (NETWORK seam) |
| `upgradeNewClient` | `func(string,string)*upgrade.Client` | `prodNewClient` | **NOT overridden** — `prodNewClient` reads `upgradeBaseURL`, so setting the var suffices |
| `upgradeToken` | `func()string` | `prodToken` | **NOT overridden** — empty is fine (unauth) |
| `upgradeExePath` | `func()(string,error)` | `prodExePath` | **NOT overridden** — `prodDetect` (which reads it) is replaced by `upgradeDetect` |
| `upgradeDetect` | `func(ctx,override,log)(Channel,string,error)` | `prodDetect` | **OVERRIDE → `(upgrade.ChannelDirect,"direct",nil)`** ("injected Detect→direct") |
| `upgradeSwap` | `func(ctx,newBin)error` | `upgrade.Swap` | **OVERRIDE → mini-swap closure** (see §3) |
| `upgradeRollback` | `func(ctx)(string,error)` | `upgrade.Rollback` | NOT reached (normal path, not --rollback) |

The flow S2 exercises (`dispatchUpgrade`, `internal/cmd/upgrade.go:230`): `upgradeDetect`→ChannelDirect ⇒
`runDirectSwap(ctx,cmd,client,effChannel)`:
1. `tempDir := os.MkdirTemp("","stagecoach-upgrade-*")` ← the staging tempDir (cleaned on swap success)
2. `release,asset := upgrade.ResolveTarget(ctx,client,opts)` ⇒ `client.LatestStable` GETs
   `/repos/{Repo}/releases/latest` (Repo="dabstractor/stagecoach" from Defaults), then `SelectAsset` picks
   the host asset.
3. `newBin := upgrade.StageNewBinary(ctx,client,release,asset,tempDir)` ⇒ **REAL** download+SHA256-verify+
   extract+sanity-run against the fake server (this is the part S2 proves end-to-end).
4. `confirmUpgrade(displayCurrent(), release.Tag, "Self-swap…", flagYes, …)` ⇒ `flagYes==true` ⇒
   `(true,nil)` — NO prompt, NO stdin read (verified in `upgrade_prompt.go:confirmUpgrade`).
5. `upgradeSwap(ctx,newBin)` ⇒ the mini-swap closure.
6. `fmt.Fprintf(out,"stagecoach upgraded to %s\n", release.Tag)` ⇒ exit 0.

**`execVersion` (package `upgrade`, `stage.go`) is NOT overridden** — the staged stub's `--version`
prints its BAKED version (see §4), so the default sanity-run works without a seam override.

## 3. THE swap decision: override `upgradeSwap` with a faithful mini-swap (test-only)

**The core testability problem**: `upgrade.Swap` resolves the running exe via the **package-upgrade,
UNEXPORTED** `resolveCurrentExe` seam (`swap.go`). Package `cmd` CANNOT touch it, so it cannot make the
real `upgrade.Swap` target a temp-dir "installed" stub. The `upgrade_run.go` file doc states this verbatim
and EXPLICITLY sanctions the workaround:

> "So upgradeSwap (default upgrade.Swap) … are function seams **P1.M4.T3 overrides** … to point at a
> temp-dir exe."

So S2 **overrides `upgradeSwap`** (the package-cmd function seam) with a closure that performs a REAL,
faithful swap against the temp "installed" stub: `backup = os.Rename(installed, installed+suffix)` →
`rename = os.Rename(newBin, installed)` → (restore-on-failure) → `os.RemoveAll(filepath.Dir(newBin))`
(staging tempDir cleanup). This produces the contract's REAL end-effects (backup created with old bytes,
installed now new bytes, temp cleaned) and is **test-only** (zero edits to `internal/upgrade/*`).

**Why NOT add an exported `resolveCurrentExe` seam** (the alternative): that would (a) edit a
**Complete** milestone file (`swap.go`, P1.M3.T2), (b) break S1's strict test-only precedent, and (c)
be redundant — the doc already designed `upgradeSwap` as the seam for exactly this. The REAL
`upgrade.Swap` logic (backup+rename+restore+isTempDir guard+isWritable gate) is **exhaustively covered**
by `swap_test.go` (unix) + `swap_windows_test.go` (windows). S2's job is the **orchestration** through
`runDirectSwap` (that it calls StageNewBinary then upgradeSwap with the right newBin, after --yes confirm)
PLUS the **real** download/verify/extract/sanity — NOT a re-test of upgrade.Swap.

> NOTE on the `swap_windows_test.go` / `swap_test.go` file comments that say "the running-image lock is
> verified by the P1.M4.T3.S2 e2e": those are aspirational. NO in-process test can rename the ACTUAL
> running test binary (on Windows it is kernel-locked; on Unix the test keeps running the old inode).
> Every swap test — S2 included — operates on **regular temp files**, proving the rename SEQUENCE. The
> running-image lock is an OS property of the real stagecoach binary swapping ITSELF (production only).
> This is fine and matches every sibling test.

## 4. cmd/stubversion is REQUIRED (the existing cmd/stubcli CANNOT express two versions)

The contract: "a compiled stub 'stagecoach' whose --version prints the target tag" (new) and "the
'installed binary' is a temp-dir stub whose --version prints the older tag" (installed). The new payload
and the installed stub must report **DIFFERENT** versions to each other.

**`cmd/stubcli` cannot do this**: it prints `STAGECOACH_STUBCLI_OUT` (env-controlled), so two identical
stubcli binaries print whatever env is set — there is no way to make the backup print v0.1.0 while the
installed prints v0.2.0 from the same global env. Byte-comparison also fails (identical bytes).

**Solution (contract-sanctioned: "or a new cmd/stubversion")**: add `cmd/stubversion/main.go` — a tiny
`package main` with `var version = "dev"` printed verbatim, baked at BUILD time via
`-ldflags "-X main.version=v0.2.0"`. Build it TWICE:
- `buildStubVersion(t,"v0.1.0")` → the "installed" stub (copied to `<tempDir>/stagecoach(.exe)`).
- `buildStubVersion(t,"v0.2.0")` → packed into the archive as the "new" payload.

This mirrors `cmd/stagecoach/main.go`'s own `var version = "dev"` ldflags pattern (verified: line 24) and
the `buildStubCLI` once-per-process build helper in `internal/upgrade/stage_test.go` (resolve module root
via `go env GOMOD`, `-buildvcs=false`, stdlib-only ⇒ builds on linux/darwin/windows × amd64/arm64).

## 5. Cross-platform facts (the suite runs on the ubuntu/macos/windows matrix via `go test -race ./...`)

From `internal/upgrade/swap_unix.go` + `swap_windows.go` (build-tagged) + `download.go::assetName`:

| Concern | unix (`!windows`) | windows |
|---|---|---|
| backup suffix (`backupPath`) | `exe + ".stagecoach-backup"` | `exe + ".old"` |
| archive format (asset suffix) | `.tar.gz` | `.zip` |
| archive binary entry base | `stagecoach` | `stagecoach.exe` |
| staged dest name | `new-stagecoach` | `new-stagecoach.exe` |
| exe suffix for paths | `""` | `.exe` |

The test computes these via `runtime.GOOS` (the contract's ".stagecoach-backup" is the unix shorthand; the
test asserts the **platform-correct** suffix). The asset name formula (mirrors unexported
`download.go::assetName`): `"stagecoach_" + TrimPrefix(tag,"v") + "_" + GOOS + "_" + GOARCH` + (`.zip` if
windows else `.tar.gz`). Checksums file name: `"stagecoach_" + TrimPrefix(tag,"v") + "_checksums.txt"`.

The mini-swap renames **regular temp files** (the installed stub is NOT running), so `os.Rename` works on
ALL platforms (no Windows image-lock — that only applies to the ACTUAL running .exe).

## 6. The fake GitHub Releases server shape (single httptest.Server, 3 routes)

`Client.LatestStable` GETs `/repos/{Repo}/releases/latest` and decodes `ghRelease{tag_name,prerelease,
draft,assets[]}`; each `ghAsset{name,browser_download_url,size}`. `StageNewBinary` then:
`FetchChecksums` (finds `*_checksums.txt` asset, downloads its URL, parses "<64hex>  <file>" lines) →
`DownloadFile` (ABSOLUTE DownloadURL) → `VerifySHA256` → `extractBinary` → `sanityCheck`.

So the fake serves:
- `GET /repos/dabstractor/stagecoach/releases/latest` → the release JSON. The asset list MUST include the
  host asset (exact goreleaser name) + the checksums asset (exact name); BOTH `browser_download_url`s are
  ABSOLUTE `ts.URL+"/archive"` and `ts.URL+"/checksums"`.
- `GET /archive` → the archive bytes (the v0.2.0 stubversion packed host-native).
- `GET /checksums` → `"<sha256hex>  <hostAssetName>\n"` (sha computed over the packed archive).

`packArchive` is ported from `internal/upgrade/stage_test.go` (package cmd needs its own copy — the
original is package `upgrade`). It packs the stub bytes under the host entry base + a throwaway README
entry (proves extractBinary pulls ONLY the stagecoach entry). Format keyed off the asset suffix.

## 7. Exact assertions (the contract, mapped to runDirectSwap's real output)

Drive `Execute(ctx)` with `["upgrade","--yes"]` after seam setup. Then:
- `exitcode.For(err) == exitcode.Success (0)` (`runDirectSwap` returns nil on success).
- `outBuf` contains `stagecoach upgraded to v0.2.0` (runDirectSwap's success Fprintf; tag is v-prefixed).
- **installed swapped**: run `<installedExe> --version` (real exec) → output contains `v0.2.0`
  (the installed stub now holds the NEW stubversion bytes).
- **backup created**: run `<installedExe+backupSuffix> --version` → output contains `v0.1.0`
  (the backup holds the OLD stubversion bytes).
- **temp cleaned**: `filepath.Glob(filepath.Join(os.TempDir(),"stagecoach-upgrade-*"))` BEFORE Execute
  equals AFTER Execute (runDirectSwap created one staging dir; the mini-swap `os.RemoveAll`'d it).
- **the real test binary is untouched**: the mini-swap operates ONLY on the captured temp stub path; it
  never calls the real `os.Executable()`/`resolveCurrentExe`, so the running test binary is safe.

## 8. Scope fences & dependency hygiene

- S2 TOUCHES ONLY: `cmd/stubversion/main.go` (NEW test-helper binary) + `internal/cmd/upgrade_swap_test.go`
  (NEW test file). Nothing else.
- S2 does NOT edit: `internal/cmd/upgrade*.go` (LANDED), `internal/upgrade/*` (read-only — P1.M1–M3
  Complete), `root.go`, `exitcode.go`, `config/*`, the commit path (FR-U12), `go.mod`, any PRD/task file.
- S2 does NOT duplicate S1's `upgrade_check_test.go` or S3's forthcoming failure/rollback suite.
- The mini-swap is the ONLY swap logic S2 owns; the real `upgrade.Swap` stays the property of `swap*.go` +
  `swap*_test.go`. If a future refactor changes `backupPath`'s suffix, `swap*_test.go` catches it; S2's
  cross-platform suffix is a test-local twin (documented).