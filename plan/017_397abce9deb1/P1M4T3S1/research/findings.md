# Findings — P1.M4.T3.S1 (--check httptest suite: exit 6 behind / exit 0 up-to-date)

The dependency P1.M4.T2.S1 is **LANDED** (internal/cmd/upgrade_run.go exists in-tree). So the seams +
`runCheck` are CONCRETE, not speculative. This file records the exact contract the test must drive.

## 0. The seams I consume (internal/cmd/upgrade_run.go — LANDED, package cmd, unexported vars)

For the **--check** path ONLY two seams matter (+ one package-UPGRADE seam). The other seams
(upgradeExePath / upgradeExecRunner / upgradeDetect / upgradeSwap / upgradeRollback) belong to the
normal/swap/rollback paths and are exercised by S2/S3 — --check never reaches them.

```go
var (
    upgradeBaseURL   string                                  // NETWORK seam. "" ⇒ api.github.com. TEST ⇒ httptest URL.
    upgradeToken     = prodToken                             // func() string; reads STAGECOACH_GITHUB_TOKEN||GITHUB_TOKEN. Optional to override.
    upgradeNewClient = prodNewClient                         // func(repo, token) *upgrade.Client; builds {BaseURL: upgradeBaseURL, Repo, Token}.
)
```
- `prodNewClient` reads `upgradeBaseURL` at CALL TIME (runUpgrade calls `upgradeNewClient(effSourceRepo, upgradeToken())`
  → builds `&upgrade.Client{BaseURL: upgradeBaseURL, Repo, Token}`). So setting `upgradeBaseURL = ts.URL`
  BEFORE Execute is sufficient to steer ALL --check network to the fake. **This is the single load-bearing seam.**
- Package-UPGRADE seam: `upgrade.SetCurrentVersion(v string)` (internal/upgrade/version.go) pins the running
  binary's version that `CurrentSemver()` reads. The test process never runs main.go, so `currentVersion`
  starts at its package default `"dev"`. Restore via `SetCurrentVersion("dev")` in t.Cleanup (there is NO
  getter — "dev" is the documented default; it round-trips because SetCurrentVersion ignores "" but accepts "dev").

## 1. runCheck (LANDED — internal/cmd/upgrade_run.go:197, VERBATIM)

```go
func runCheck(ctx context.Context, cmd *cobra.Command, client *upgrade.Client, effChannel string) error {
	cur, ok := upgrade.CurrentSemver()
	release, _, err := upgrade.ResolveTarget(ctx, client, upgrade.ResolveOptions{
		Version:    flagTargetVersion,
		Prerelease: effChannel == "prerelease",
	})
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: check: %w", err))
	}
	latest := release.Tag
	out := cmd.OutOrStdout()
	if !ok { // dev build — informational; do NOT claim an update
		fmt.Fprintf(out, "stagecoach dev (latest: %s; development build — cannot compare)\n", latest)
		return nil // exit 0
	}
	if upgrade.Compare(cur, latest) >= 0 {
		fmt.Fprintf(out, "stagecoach %s (latest: %s; up to date)\n", cur, latest)
		return nil // exit 0
	}
	fmt.Fprintf(out, "update available: %s → %s; run \"stagecoach upgrade\"\n", cur, latest)
	return exitcode.New(exitcode.UpdateAvailable, nil) // exit 6
}
```

KEY: `cur` = `CurrentSemver()` which returns the **v-prefixed** canonical ("v0.1.0", NOT "0.1.0"), because
`ParseAndClean` always returns `"v"+core`. So the EXACT behind-line is:
`update available: v0.1.0 → v0.2.0; run "stagecoach upgrade"` (when current="0.1.0", latest tag="v0.2.0").
The item contract wrote `'0.1.0 → 0.2.0'` (no v-prefix) — that is a simplification; the test MUST assert the
v-prefixed form produced by runCheck (read the source), or assert the robust substrings
("update available" + "→" + the exit code). Reconcile by matching runCheck's real Fprintf.

## 2. The dispatch to --check (internal/cmd/upgrade.go runUpgrade, LANDED prologue + dispatch)

```go
// after S1's prologue (validateUpgradeFlags → LoadUpgradeConfig → effChannel/effSourceRepo):
client := upgradeNewClient(effSourceRepo, upgradeToken())
if flagCheck {
    return runCheck(ctx, cmd, client, effChannel)
}
```
- `effChannel`: LoadUpgradeConfig → Defaults (Channel="stable") when no global config (isolated HOME).
  flagChannel="" + flagPrerelease=false ⇒ effChannel="stable" ⇒ runCheck ResolveOptions{Version:"", Prerelease:false}
  ⇒ ResolveTarget ⇒ `Client.LatestStable` ⇒ GET `/repos/{Repo}/releases/latest`.
- `effSourceRepo`: Defaults ("dabstractor/stagecoach") ⇒ Client.Repo. The httptest handler need NOT validate
  the repo segment (respond to any path with suffix "/releases/latest"); asserting the suffix proves the
  right endpoint was hit.

## 3. exitcode mapping (internal/exitcode/exitcode.go — LANDED)

- `exitcode.UpdateAvailable = 6`; `exitcode.Error = 1`; `exitcode.Success = 0`.
- `exitcode.For(err)`: nil→0; `*ExitError`→its `.Code`; then sentinel/domain mapping; else 1.
- runCheck behind ⇒ `exitcode.New(exitcode.UpdateAvailable, nil)` ⇒ `ExitError{Code:6, Err:nil}`.
  `For()` ⇒ 6. `ExitError.Error()` returns "" when Err==nil ⇒ **main.go does NOT print "stagecoach: "** (main's
  gate is `if err != nil && err.Error() != ""`). So the exit-6 path is silent on stderr — the "update
  available" line is on STDOUT only. The test can assert errBuf is empty (no double "stagecoach:" prefix).
- runCheck 404 ⇒ `exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: check: %w", <ErrNoReleases-wrapped>))`.
  `For()` ⇒ 1. `errors.Is(err, upgrade.ErrNoReleases)` ⇒ true (the *ExitError.Unwrap chain reaches it).

## 4. The httptest fake pattern (mirror internal/upgrade/releases_test.go — LANDED, package upgrade)

releases_test.go already proves the exact shape. Reuse its idioms (re-implemented in package cmd since the
helpers there are unexported):
```go
// canned /releases/latest payload with a chosen tag (stable channel hits this endpoint).
func cannedLatestTag(tag string) string {
	return `{"tag_name": "` + tag + `", "name": "Release", "prerelease": false, "draft": false,
	         "assets": [{"name":"stagecoach_x_linux_amd64.tar.gz","browser_download_url":"https://ex/a","size":1}]}`
}
// handler: serve 200+body when path suffix is /releases/latest; count requests.
// 404 variant: w.WriteHeader(404); w.Write([]byte(`{"message":"Not Found"}`)).
```
The Client (`releases.go`): 404 ⇒ ErrNoReleases; 403/429 ⇒ ErrRateLimited; else non-2xx ⇒ ErrHTTP. So a 404
fake ⇒ ResolveTarget returns an ErrNoReleases-wrapped error ⇒ runCheck's `err != nil` branch ⇒ exit 1.

## 5. How to DRIVE runUpgrade from a package-cmd test (the existing, verified pattern)

internal/cmd/upgrade_test.go::TestUpgradeCommand_NoBootstrapOutsideRepo is the template. Drive via
`Execute(ctx)` (returns the RunE error; cobra SilenceErrors=true ⇒ error is returned, not printed):
```go
_, origOut, origErr, origRunE := saveRootState(t)
defer restoreRootState(t, nil, origOut, origErr, origRunE)
defer resetFlags(upgradeCmd.Flags())   // <-- MUST also reset upgradeCmd's LOCAL flags (restoreRootState resets only root's)

var outBuf, errBuf bytes.Buffer
rootCmd.SetOut(&outBuf); rootCmd.SetErr(&errBuf)
rootCmd.SetArgs([]string{"upgrade", "--check"})   // cobra sets flagCheck=true
err := Execute(context.Background())              // == runUpgrade's return value
code := exitcode.For(err)
```
GOTCHA: `restoreRootState` resets `rootCmd.Flags()` + `rootCmd.PersistentFlags()` but NOT `upgradeCmd.Flags()`.
The existing test adds an explicit `defer resetFlags(upgradeCmd.Flags())`. Mirror that or flagCheck/flagVersion
leak across tests (same test binary, shared package vars).

GOTCHA: isolate HOME so LoadUpgradeConfig (called inside runUpgrade's prologue) reads no real global config
and returns Defaults (stable / dabstractor/stagecoach): `t.Setenv("HOME", t.TempDir()); t.Setenv("XDG_CONFIG_HOME","")`.
(LoadUpgradeConfig: missing file ⇒ Defaults + nil error — never bootstraps.)

## 6. The 5 contract cases → exact assertions

| case | SetCurrentVersion | fake serves | runCheck branch | stdout contains | exitcode.For | extra |
|------|-------------------|-------------|-----------------|-----------------|--------------|-------|
| (a) behind | `"0.1.0"` | 200, tag `"v0.2.0"` | behind | `"update available"` + `"v0.1.0 → v0.2.0"` + `→` | **6** (UpdateAvailable) | errBuf empty (no double "stagecoach:") |
| (b) up-to-date | `"0.2.0"` | 200, tag `"v0.2.0"` | Compare>=0 | `"up to date"` + both `v0.2.0` | **0** | err=nil |
| (c) dev | `"dev"` (or unset) | 200, tag `"v0.2.0"` | !ok (dev) | `"development build — cannot compare"` | **0** | stdout does NOT contain `"update available"` |
| (d) 404 no releases | `"0.1.0"` (any) | **404** | err!=nil | — (message on stderr via main only; in-test errBuf may have nothing) | **1** (Error) | `errors.Is(err, upgrade.ErrNoReleases)` true |
| (e) no on-disk change | `"0.1.0"` | 200, tag `"v0.2.0"` | behind (full --check) | — | 6 | `stagecoach-upgrade-*` temp glob UNCHANGED pre/post; os.Executable stat (size/mtime) UNCHANGED |

Case (e) detail: --check never calls StageNewBinary/Swap (runCheck only does ResolveTarget+CurrentSemver+
Compare), so by construction no temp dir is created and the running exe is untouched. ASSERT it: snapshot
`filepath.Glob(filepath.Join(os.TempDir(), "stagecoach-upgrade-*"))` before+after Execute; assert equal.
Belt-and-suspenders: snapshot `os.Stat(os.Executable())` size+mtime before+after; assert equal.

## 7. No-network proof (the contract's hard requirement)

Setting `upgradeBaseURL = ts.URL` (a localhost httptest.Server) makes the Client target localhost — there is
no path to api.github.com. The --check code path's ONLY network call is `Client.LatestStable` (via
ResolveTarget) inside runCheck; the prologue (validateUpgradeFlags/LoadUpgradeConfig/effChannel resolution)
is pure + file-IO. So `upgradeBaseURL=ts.URL` is a COMPLETE no-network guarantee. Strengthen with a
request-counting handler (assert exactly 1 GET, to the /releases/latest path) to prove the fake was the sole
target.

## 8. File placement — dedicated test file recommended

`internal/cmd/upgrade_test.go` is S1's (registration/validation). `upgrade_prompt_test.go` is S2's. The
precedent is ONE TEST FILE PER FEATURE. P1.M4.T3 has THREE subtasks (S1 --check, S2 direct-swap, S3 failure/
rollback) — dropping all three suites into upgrade_test.go would bloat it to ~1500 lines and risk merge
conflicts. RECOMMEND a dedicated `internal/cmd/upgrade_check_test.go` for this suite (the contract's
"upgrade_test.go" is the logical suite name; the physical file is an implementation detail). S2/S3 get their
own files. State this choice explicitly in the PRP.

## 9. Scope fence

TOUCH: `internal/cmd/upgrade_check_test.go` (NEW — the only file). DO NOT TOUCH: internal/cmd/upgrade.go,
upgrade_run.go, upgrade_prompt.go (all LANDED — read-only consumers), internal/upgrade/* (read-only), root.go,
exitcode.go, config, the commit path (FR-U12), go.mod, any PRD/task file. This item ADDS TESTS ONLY — zero
production-code edits.