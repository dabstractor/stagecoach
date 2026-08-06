# System Context: v3.0 Self-Update Subsystem

## Scope
This bugfix targets three logic gaps in the `internal/upgrade/` package — the walled-off,
stdlib-only self-update subsystem added in v3.0 (`stagecoach upgrade`, PRD §9.29 FR-U1–U12).
No code outside `internal/upgrade/` is changed by these fixes (the command layer in
`internal/cmd/` is only touched by regression tests).

## Package Layout (`internal/upgrade/`)
| File | Responsibility |
|------|---------------|
| `version.go` | Semver parse/compare (`ParseAndClean`, `Compare`, `CurrentSemver`). `Compare` returns **0 for unparseable** operands (dev-build defense). |
| `releases.go` | GitHub Releases metadata client (sole network surface). `LatestStable`, `ReleaseByTag`, `LatestAdmittingPrereleases`. |
| `download.go` | Download + SHA256 verify + asset-name/checksums-name helpers. `SelectAsset`, `FetchChecksums`, `DownloadFile`, `VerifySHA256`. |
| `resolve.go` | Thin composition: channel/version dispatch (`ResolveTarget`) → Client method → `SelectAsset`. |
| `stage.go` | **BUG-001**: download→verify→extract→**sanity-run** pipeline (`StageNewBinary`, `sanityCheck`, `extractBinary`). |
| `detect.go` | **BUG-002**: FR-U2 install-method detection cascade. `Detect`, `detectPath`, `detectPackageManager`, `detectOverride`. |
| `swap.go` / `rollback.go` | Atomic swap + one-step rollback (NOT touched by these fixes). |

## Key Architectural Patterns (MUST follow)
1. **Sentinel-error convention**: every typed error is a package-level `var Err… = errors.New(…)`,
   wrapped with `%w` at its use site so `errors.Is` reaches the sentinel AND the message survives.
   BUG-001/003 fixes must preserve this (no new error sentinels needed — the existing
   `ErrSanityVersionMismatch` / `ErrNoReleases` are reused).
2. **Injectable seams**: `execVersion` (package-level var, stage.go), `Detector.Env`/`Detector.Exec`
   (detect.go), `Client.HTTP`/`Client.BaseURL` (releases.go). Tests override these; production wires
   the real os/exec / os.Getenv / http.DefaultClient. Fixes must NOT add hard env reads or
   os/exec calls — use the existing seams.
3. **stdlib-only wall** (FR-U12): `internal/upgrade/` imports NO `internal/*` and no third-party
   deps. Fixes must add zero `go.mod` requires.
4. **Substring sanity ≠ semver compare**: `sanityCheck` deliberately does a raw substring check
   (NOT `Compare`) — the command layer's job is the semver compare. The BUG-001 fix must keep it a
   substring check, just v-normalized.
5. **Test stubs**: `cmd/stubcli` (env-driven `STAGECOACH_STUBCLI_OUT/EXIT`), `cmd/stubversion`
   (ldflags-baked `-X main.version=…`). Regression tests for BUG-001 should use `stubcli` with the
   no-v env value, or `stubversion` baked WITHOUT the `v` prefix.

## Call Chain (upgrade self-swap path)
```
cmd/stagecoach/main.go  →  internal/cmd/upgrade_run.go (runUpgrade)
    → upgrade.Detect()                    [detect.go — BUG-002]
    → upgrade.ResolveTarget()             [resolve.go]
        → LatestStable / ReleaseByTag / LatestAdmittingPrereleases  [releases.go — BUG-003]
        → SelectAsset()                   [download.go]
    → upgrade.StageNewBinary()            [stage.go — BUG-001]
        → FetchChecksums + DownloadFile + VerifySHA256
        → extractBinary()
        → sanityCheck(newBinPath, release.Tag)   ← BUG-001 failure point
    → upgrade.SwapBinary()                [swap.go — not affected]
```

## Version Injection Reality (root cause of BUG-001)
- `.goreleaser.yaml` ldflags: `-X main.version={{.Version}}`
- goreleaser `{{.Version}}` = the git tag **WITHOUT** the leading `v` (e.g. tag `v1.2.0` → `1.2.0`)
- `cmd/stagecoach/main.go`: `var version` is injected; `resolveVersion(version)` returns it as-is
  for a tagged release; `cmd.Version` is set to this string; cobra's `--version` prints
  `stagecoach version 1.2.0` (no `v`).
- BUT `release.Tag` (from GitHub `tag_name`) IS `v1.2.0` (WITH `v`).
- So `bytes.Contains(stdout_containing_"1.2.0", []byte("v1.2.0"))` → **FALSE** → sanity aborts.