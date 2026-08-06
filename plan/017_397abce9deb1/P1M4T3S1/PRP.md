name: "P1.M4.T3.S1 — upgrade --check httptest suite: exit 6 behind / exit 0 up-to-date / exit 0 dev / exit 1 no-releases / no on-disk change (FR-U6, §20.5)"
description: >
  The FIRST of P1.M4.T3's three test suites. A NEW package-cmd test file (internal/cmd/upgrade_check_test.go)
  that drives `stagecoach upgrade --check` through its LANDED package-var seams against an httptest fake
  GitHub Releases server — NO real network, NO real subprocess, NO real binary swap. It exercises the FR-U6
  --check dispatch (runCheck in internal/cmd/upgrade_run.go, LANDED by P1.M4.T2.S1) for the 5 contract cases:
  (a) current "0.1.0", latest tag "v0.2.0" ⇒ stdout "update available: v0.1.0 → v0.2.0; …" AND
  exitcode.For(err)==6 (UpdateAvailable); (b) current "0.2.0", latest "v0.2.0" ⇒ "up to date", exit 0;
  (c) current "dev" (unparseable) ⇒ informational dev line, exit 0, NO "update available"; (d) fake returns
  404 ⇒ exit 1 with errors.Is(err, upgrade.ErrNoReleases); (e) --check leaves NO on-disk artifact (no
  "stagecoach-upgrade-*" temp dir; os.Executable() byte-for-byte unchanged). DRIVING: Execute(ctx) with
  args ["upgrade","--check"] (the existing TestUpgradeCommand_NoBootstrapOutsideRepo pattern) returns
  runUpgrade's error; exitcode.For maps it. SEAMS overridden (package cmd, restored via t.Cleanup):
  upgradeBaseURL=httptest.URL (the single load-bearing NETWORK seam — prodNewClient reads it at call time);
  upgrade.SetCurrentVersion(...) (package-UPGRADE seam pins the running version CurrentSemver reads — the
  test process never runs main.go, so currentVersion starts at "dev"; restore via SetCurrentVersion("dev")).
  effChannel defaults to "stable" (isolated HOME ⇒ LoadUpgradeConfig ⇒ Defaults ⇒ no global config), so
  runCheck calls ResolveTarget→Client.LatestStable→GET /repos/{repo}/releases/latest — the fake serves that
  path. The other seams (upgradeExePath/upgradeExecRunner/upgradeDetect/upgradeSwap/upgradeRollback) belong
  to the normal/swap/rollback paths (S2/S3) and are NEVER reached by --check. TESTS-ONLY: zero production
  edits. go test ./internal/cmd/ -race green; no network; go vet/gofmt clean; lint clean (no U1000 — every
  seam var read by runCheck/runUpgrade in the LANDED code; the test reads them by override).

---

## Goal

**Feature Goal**: Prove the FR-U6 `stagecoach upgrade --check` dispatch (LANDED in
internal/cmd/upgrade_run.go::runCheck by P1.M4.T2.S1) behaves per contract across its 5 cases — exit 6
when behind, exit 0 when up-to-date, exit 0 informational for a dev build, exit 1 on no-releases, and
zero on-disk side effects — using ONLY an httptest fake + package-var/version seams, with no real network.

**Deliverable**: `internal/cmd/upgrade_check_test.go` (NEW, package `cmd`) — the --check test suite:
a small fake-server harness (canned GitHub `/releases/latest` payloads + a 404 variant + a request
counter), a seam-setup/restore helper, and 5 `TestUpgradeCheck_*` test functions (one per contract case).
**Dedicated file** (not additions to upgrade_test.go): S1 owns upgrade_test.go (registration/validation),
S2 owns upgrade_prompt_test.go; the per-feature-test-file precedent plus the 3 upcoming P1.M4.T3 suites
(S1/S2/S3) argue for one file each. (The item contract's "upgrade_test.go" is the logical suite name; the
physical file is an implementation detail — state this choice in a file-level comment.)

**Success Definition**:
- `go test ./internal/cmd/ -run TestUpgradeCheck -race -v` runs 5 cases, ALL PASS.
- Each case asserts BOTH the process exit code (via `exitcode.For(err)` on the Execute return value) AND
  the relevant stdout content / sentinel. Cases (a)/(b)/(c) assert the exact runCheck output strings.
- Case (a): `exitcode.For(err) == exitcode.UpdateAvailable` (6); stdout contains the v-prefixed
  `update available: v0.1.0 → v0.2.0` line; stderr is EMPTY (no double "stagecoach:" prefix — main.go's
  nil-Err gate, verified here because Execute surfaces the same `*ExitError`).
- Case (e): no `stagecoach-upgrade-*` temp dir is created (pre/post glob equal); `os.Executable()` stat
  (size+mtime) unchanged.
- No test makes a real network call: `upgradeBaseURL = httptest.Server.URL` (localhost); a request counter
  proves exactly 1 GET to `/releases/latest`. Run with `-args -httptest.serve=` unset; no `api.github.com`.
- `go build ./...` clean; `go vet ./internal/cmd/...` clean; `gofmt -l` empty; `make test` + `make lint`
  clean; `git status --porcelain` == the ONE new test file (zero production edits).

## User Persona (if applicable)

**Target User**: The Stagecoach maintainer / CI author who gates on `stagecoach upgrade --check` (exit 6 ⇒
an update is available). This suite is the regression net proving the exit-code contract (§15.4) and the
no-network/no-side-effect guarantees hold for the check-only surface scripts call.
**Use Case**: CI runs `stagecoach upgrade --check`; this test proves exit 6 fires when behind and exit 0
otherwise, with no accidental binary/temp changes.
**Pain Points Addressed**: FR-U6 ("--check is the side-effect-free surface scripts should call") + FR-U12
(walled off; no on-disk change) + the no-network guarantee (§19 exception scoped to upgrade fetching).

## Why

- **PRD §9.29 FR-U6 / §20.5**: `--check` is the check-only surface; §20.5's self-update scenario list
  explicitly requires "`upgrade --check` reports current vs. latest and exits `6` when behind". This suite
  is that scenario's automated regression net.
- **P1.M4.T2.S1 (LANDED) shipped the dispatch + seams specifically so this suite can run with no real
  network/subprocess/swap.** The seam design (`upgradeBaseURL` + `SetCurrentVersion`) is the contract this
  suite consumes — it is the FR-U12 test wall's first customer.
- **Risk being guarded**: `--check` silently regressing to exit 0 when behind (e.g. a Compare/sign flip,
  CurrentSemver treating dev as comparable), or leaking a temp dir / touching the binary. Both are caught
  here deterministically.

## What

A single new package-`cmd` test file. It does NOT edit any production code. It overrides two seams
(`upgradeBaseURL`, `upgrade.SetCurrentVersion`) per case, drives runUpgrade end-to-end via `Execute(ctx)`
with `["upgrade","--check"]`, and asserts exit code + stdout + (case e) on-disk invariants.

### Success Criteria
- [ ] NEW file `internal/cmd/upgrade_check_test.go` (package `cmd`); a file doc comment names it the FR-U6
      --check suite + explains the dedicated-file choice + the no-network seam (`upgradeBaseURL`).
- [ ] `TestUpgradeCheck_Behind_Exit6`: SetCurrentVersion("0.1.0"); fake serves 200 + tag_name "v0.2.0";
      `exitcode.For(err)==exitcode.UpdateAvailable`; stdout contains `update available` AND `v0.1.0 → v0.2.0`
      AND `→`; stderr (errBuf) is EMPTY (no double "stagecoach:" prefix); exactly 1 GET to the fake.
- [ ] `TestUpgradeCheck_UpToDate_Exit0`: SetCurrentVersion("0.2.0"); fake serves tag_name "v0.2.0"; err==nil
      (For==0); stdout contains `up to date` and both `v0.2.0`.
- [ ] `TestUpgradeCheck_Dev_Exit0`: SetCurrentVersion("dev"); fake serves tag_name "v0.2.0"; err==nil;
      stdout contains `development build — cannot compare`; stdout does NOT contain `update available`.
- [ ] `TestUpgradeCheck_NoReleases_Exit1`: fake serves 404; `exitcode.For(err)==exitcode.Error`;
      `errors.Is(err, upgrade.ErrNoReleases)` is true.
- [ ] `TestUpgradeCheck_NoOnDiskChange`: SetCurrentVersion("0.1.0"); fake serves tag_name "v0.2.0" (so the
      full --check path runs incl. the behind branch); BEFORE+AFTER Execute, `filepath.Glob` of
      `os.TempDir()/stagecoach-upgrade-*` is UNCHANGED (no new temp dir) AND `os.Stat(os.Executable())`
      Size+ModTime are UNCHANGED. (exit code is 6 — the change-free behind path.)
- [ ] A shared seam-setup helper restores `upgradeBaseURL` and calls `upgrade.SetCurrentVersion("dev")` via
      `t.Cleanup`; each test also does `saveRootState`/`restoreRootState` + `resetFlags(upgradeCmd.Flags())`
      (mirrors TestUpgradeCommand_NoBootstrapOutsideRepo) and isolates HOME so LoadUpgradeConfig ⇒ Defaults.
- [ ] `go build ./...`, `go vet ./internal/cmd/...`, `gofmt -l`, `make test`, `make lint` all clean.
- [ ] `git status --porcelain` == `internal/cmd/upgrade_check_test.go` ONLY.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the VERBATIM LANDED `runCheck` (exact Fprintf formats incl. the v-prefix), the exact seam var
names + their read sites, the exact `Execute`-driving pattern (with the `resetFlags(upgradeCmd.Flags())`
gotcha), the httptest fake shape (mirrored from releases_test.go), the exitcode.For mapping for the
behind/404 paths, the SetCurrentVersion restore discipline, and the precise per-case assertions.

### Documentation & References

```yaml
# MUST READ — the LANDED dispatch + seams this suite drives (the contract source of truth).
- file: internal/cmd/upgrade_run.go
  why: "runCheck (line ~197) is EXACTLY what --check does: CurrentSemver → ResolveTarget → Compare; the three
        output Fprintf formats + the exit-6 return. The seam vars (upgradeBaseURL/upgradeToken/
        upgradeNewClient) are declared here (line ~68); prodNewClient reads upgradeBaseURL at call time."
  critical: "cur = CurrentSemver() returns the V-PREFIXED canonical ('v0.1.0'), so the behind line is
             'update available: v0.1.0 → v0.2.0; run \"stagecoach upgrade\"' — NOT '0.1.0 → 0.2.0' (the item
             contract's shorthand). Match runCheck's real Fprintf. exit-6 returns
             exitcode.New(exitcode.UpdateAvailable, nil) → ExitError.Err=nil → Error()=='' → main.go skips
             the stderr print; the in-test errBuf is therefore EMPTY on the behind path (assert it)."

# MUST READ — the runUpgrade prologue + the --check branch (how Execute reaches runCheck).
- file: internal/cmd/upgrade.go
  why: "runUpgrade (line ~156): validateUpgradeFlags → config.LoadUpgradeConfig → resolve effChannel/
        effSourceRepo → `client := upgradeNewClient(effSourceRepo, upgradeToken())` → `if flagCheck { return
        runCheck(ctx, cmd, client, effChannel) }`. The 9 flag package vars (flagCheck/flagTargetVersion/
        flagPrerelease/...) are set by cobra when Execute parses ['upgrade','--check']. effChannel defaults to
        'stable' (LoadUpgradeConfig ⇒ Defaults when no global config)."
  gotcha: "LoadUpgradeConfig is called INSIDE runUpgrade (not via PersistentPreRunE) — it reads the global
           file. Isolate HOME (t.TempDir + XDG_CONFIG_HOME='') so it returns Defaults with no real config +
           no bootstrap. validateUpgradeFlags is pure; flagCheck/flagVersion leak across tests unless
           resetFlags(upgradeCmd.Flags()) runs in t.Cleanup."

# MUST READ — the existing command-test pattern (the Execute driver + state save/restore).
- file: internal/cmd/upgrade_test.go
  why: "TestUpgradeCommand_NoBootstrapOutsideRepo is the template: saveRootState/restoreRootState +
        resetFlags(upgradeCmd.Flags()) + rootCmd.SetOut/SetErr/SetArgs(['upgrade', ...]) + Execute(ctx).
        Execute returns runUpgrade's error (cobra SilenceErrors=true ⇒ returned, not printed); map via
        exitcode.For."
  pattern: "the 4-line restore idiom: `_, oO, oE, oR := saveRootState(t); defer restoreRootState(t,nil,oO,oE,oR);
            defer resetFlags(upgradeCmd.Flags())`. resetFlags(upgradeCmd.Flags()) is SEPARATE from
            restoreRootState (which resets only rootCmd's flags) — do NOT omit it or flagCheck leaks."

# MUST READ — the httptest fake shape (mirror these helpers in package cmd; they're unexported in package upgrade).
- file: internal/upgrade/releases_test.go
  why: "newFakeClient/cannedLatest/statusServer prove the exact GitHub /releases/latest JSON the Client
        decodes (ghRelease{tag_name,prerelease,draft,assets[]}) + that a 404 ⇒ ErrNoReleases. Re-implement
        cannedLatest-style helpers in the new test file (parameterize the tag). The Client path for stable is
        /repos/{Repo}/releases/latest."

# MUST READ — the version seam (pins current) + Compare's dev-defense.
- file: internal/upgrade/version.go
  why: "SetCurrentVersion(v) sets package var currentVersion (ignored if v==''; 'dev' accepted). CurrentSemver
        ⇒ ParseAndClean(currentVersion): 'dev' ⇒ ('',false) (the case-c branch); '0.1.0'/'v0.1.0' ⇒
        ('v0.1.0',true) (v-PREFIXED canonical — drives the exact behind-line format). Compare(a,b): -1/0/+1;
        an unparseable operand ⇒ 0 (dev never falsely signals update). The test process never runs main.go ⇒
        currentVersion starts at 'dev' (the package default); restore via SetCurrentVersion('dev')."

# MUST READ — exit codes + the nil-Err print gate.
- file: internal/exitcode/exitcode.go
  why: "UpdateAvailable=6, Error=1, Success=0. For(err): nil→0; *ExitError→.Code; … runCheck behind ⇒
        New(UpdateAvailable,nil) ⇒ For==6. 404 ⇒ New(Error, fmt.Errorf('stagecoach: check: %w', <ErrNoReleases-
        wrapped>)) ⇒ For==1 AND errors.Is(err, ErrNoReleases)==true (ExitError.Unwrap chains to it)."
- file: cmd/stagecoach/main.go
  why: "main's stderr gate `if err != nil && err.Error() != \"\"` ⇒ a nil-Err *ExitError (exit-6) prints
        NOTHING to stderr. The test asserts errBuf empty on the behind path to encode this no-double-prefix
        contract (Execute surfaces the same *ExitError main would see)."

# MUST READ — the release Client contract (the 404⇒ErrNoReleases mapping + the /releases/latest path).
- file: internal/upgrade/releases.go
  why: "Client.do: 404 ⇒ ErrNoReleases; 403/429 ⇒ ErrRateLimited; else non-2xx/transport ⇒ ErrHTTP.
        LatestStable GETs /repos/{Repo}/releases/latest. ResolveTarget (resolve.go) dispatches Version >
        Prerelease > LatestStable; for effChannel='stable'+no --version it is LatestStable."

# MUST READ — the FR-U6 spec + the §20.5 scenario this suite automates.
- docfile: plan/017_397abce9deb1/prd_snapshot.md
  section: "§9.29 FR-U6 (exit 0/6, the check-only surface) + §20.5 (self-update: 'upgrade --check reports
            current vs. latest and exits 6 when behind') + §15.4 (exit codes)"
  why: "FR-U6: '--check resolves current vs. latest (delegating nothing, downloading nothing, swapping
        nothing) and prints … Exit 0 if up to date, exit 6 if an update is available.' §20.5 lists the exact
        scenario this suite automates. §15.4: 6 is upgrade-path only."
- docfile: plan/017_397abce9deb1/P1M4T3S1/research/findings.md
  why: "The per-case assertion matrix (§6), the seam read-sites, the no-network proof (§7), the file-placement
        rationale (§8), and the scope fence (§9)."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  upgrade.go              # LANDED (S1 prologue + dispatch) — READ-ONLY consumer; runUpgrade reaches runCheck on --check
  upgrade_run.go          # LANDED (P1.M4.T2.S1) — READ-ONLY consumer; runCheck + the seam vars (upgradeBaseURL etc.)
  upgrade_prompt.go       # LANDED (S2) — NOT touched by --check (no confirm on --check per FR-U9)
  upgrade_test.go         # LANDED (S1) — READ-ONLY; the Execute/saveRootState/resetFlags pattern source
  upgrade_prompt_test.go  # LANDED (S2) — per-feature-test-file precedent
  root_test.go            # READ-ONLY — saveRootState/restoreRootState/resetFlags/Execute helpers (package cmd)
  root.go                 # READ-ONLY — Execute(ctx), flagVerbose
internal/upgrade/
  version.go              # READ-ONLY — SetCurrentVersion/CurrentSemver/Compare (the version seam)
  releases.go             # READ-ONLY — Client/LatestStable/ErrNoReleases/ErrRateLimited/ErrHTTP
  resolve.go              # READ-ONLY — ResolveTarget (stable⇒LatestStable)
internal/exitcode/exitcode.go  # READ-ONLY — UpdateAvailable/Error/Success/New/For
cmd/stagecoach/main.go    # READ-ONLY — the nil-Err stderr gate (no double "stagecoach:")
```

### Desired Codebase tree with files to be added/modified

```bash
internal/cmd/upgrade_check_test.go   # NEW — the FR-U6 --check suite (5 cases + fake-server harness + seam helper)
# NOTHING ELSE. Zero production edits. (S2/S3 will add their own upgrade_*_test.go files later.)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (cur is V-PREFIXIXED): upgrade.CurrentSemver() returns ParseAndClean(currentVersion), which
// ALWAYS yields the v-prefixed canonical ("v0.1.0", never "0.1.0"). runCheck's behind line is therefore
// `update available: v0.1.0 → v0.2.0; run "stagecoach upgrade"`. The item contract's shorthand '0.1.0 → 0.2.0'
// is missing the v-prefix — DO NOT assert the bare form; match runCheck's real Fprintf (read upgrade_run.go:~210).

// CRITICAL (the single load-bearing seam): prodNewClient builds &upgrade.Client{BaseURL: upgradeBaseURL, …}
// and runUpgrade calls upgradeNewClient(effSourceRepo, upgradeToken()) BEFORE runCheck. So setting
// upgradeBaseURL = ts.URL (the httptest.Server.URL) steers ALL --check network to localhost. There is NO other
// network call in the --check path (validateUpgradeFlags/LoadUpgradeConfig/effChannel-resolution are pure+file-IO).
// This is the COMPLETE no-network guarantee.

// CRITICAL (SetCurrentVersion restore — no getter): the test process never runs main.go, so package upgrade's
// currentVersion starts at "dev" (the default). Pin per-case via upgrade.SetCurrentVersion("0.1.0"); restore
// via t.Cleanup(func(){ upgrade.SetCurrentVersion("dev") }). SetCurrentVersion ignores "" but accepts "dev"
// (non-empty), so "dev" round-trips to the original state. (If a sibling test had set it non-dev, "dev" is
// still the canonical reset — there is no Get; document this in a comment.)

// CRITICAL (resetFlags(upgradeCmd.Flags()) is SEPARATE from restoreRootState): restoreRootState resets
// rootCmd.Flags()+rootCmd.PersistentFlags() but NOT upgradeCmd's LOCAL flags. flagCheck/flagVersion/flagChannel
// are bound to upgradeCmd.Flags(); without an explicit `defer resetFlags(upgradeCmd.Flags())` they leak across
// tests in the same binary (cobra/pflag don't reset between Parses). Mirror upgrade_test.go's two-defer idiom.

// CRITICAL (isolated HOME): runUpgrade's prologue calls config.LoadUpgradeConfig(), which reads the GLOBAL
// config file (GlobalConfigPath under $HOME/$XDG_CONFIG_HOME). Without isolation a real ~/.config/stagecoach/
// config.toml could set [upgrade].channel/source_repo and change effChannel/effSourceRepo. Isolate:
// t.Setenv("HOME", t.TempDir()); t.Setenv("XDG_CONFIG_HOME", "")  ⇒ GlobalConfigPath lands in temp (missing)
// ⇒ LoadUpgradeConfig returns Defaults{Channel:"stable", SourceRepo:"dabstractor/stagecoach"} (never bootstraps).

// GOTCHA (exit-6 stderr is silent): runCheck behind returns exitcode.New(exitcode.UpdateAvailable, nil).
// ExitError.Err==nil ⇒ ExitError.Error()=="" ⇒ main.go's `if err != nil && err.Error() != ""` gate SKIPS the
// stderr print. In the test, cobra (SilenceErrors=true) also doesn't print it ⇒ errBuf is EMPTY on the behind
// path. Assert errBuf=="" (or !strings.Contains(errBuf,"stagecoach:")) to encode the no-double-prefix contract.

// GOTCHA (Compare's dev-defense is what makes case-c exit 0): CurrentSemver("dev") ⇒ ("",false) ⇒ runCheck's
// `!ok` branch prints the dev line + returns nil (exit 0). Compare is never even called for dev. Assert the
// dev output does NOT contain "update available" — this is the FR-U6 "dev never falsely signals" guarantee.

// GOTCHA (httptest.Server.Close via t.Cleanup): httptest.NewServer allocates a real listener on 127.0.0.1; close
// it with t.Cleanup(ts.Close) to avoid leaking ports across the 5 cases. (releases_test.go's newFakeClient does
// exactly this.) The Client does NOT need a custom *http.Client — HTTP nil ⇒ http.DefaultClient, which dials
// the localhost server fine.

// GOTCHA (the fake's path): Client.LatestStable GETs /repos/{effSourceRepo}/releases/latest. The handler may
// assert strings.HasSuffix(r.URL.Path, "/releases/latest") and serve the canned body (proves the right endpoint);
// respond 404 for the no-releases case. Repo="dabstractor/stagecoach" (Defaults) — the handler need not parse it.

// GOTCHA (request counter for no-network proof): wrap the handler to count GETs under a mutex; assert exactly 1
// request after Execute. Combined with upgradeBaseURL=localhost, this proves the fake was the sole network target.
```

## Implementation Blueprint

### Data models and structure

None (test-only). The file declares only test helpers + test functions. No types beyond an optional
`fakeServer` helper struct or a `func(bool) http.HandlerFunc` — keep it minimal (mirror releases_test.go's
`statusServer`/`cannedLatest` style, parameterized).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/cmd/upgrade_check_test.go — file doc + imports + the fake-server harness + seam helper
  - PACKAGE DOC (file comment): "FR-U6 upgrade --check suite (P1.M4.T3.S1). Drives runUpgrade via Execute with
    args ['upgrade','--check'] against an httptest fake GitHub Releases server, overriding the package-cmd
    upgradeBaseURL seam (the NETWORK seam) + the package-upgrade SetCurrentVersion seam (pins the running
    version). NO real network, NO real subprocess, NO real binary swap (FR-U12). Dedicated file: S1 owns
    upgrade_test.go (registration/validation); this is the per-feature --check suite (S2/S3 get their own
    files). Zero production edits — this file only READS the LANDED runCheck/seams in upgrade_run.go."
  - IMPORTS: bytes, context, errors, net/http, net/http/httptest, os, path/filepath, strings, sync, testing,
    github.com/dabstractor/stagecoach/internal/exitcode, github.com/dabstractor/stagecoach/internal/upgrade.
    (NO cobra/pflag import needed — Execute + saveRootState/restoreRootState/resetFlags are same-package.)
  - HELPER: newCheckFake(t, tag string, status int) (*httptest.Server, *int32):
      Returns an httptest.Server that, for a request whose URL.Path has suffix "/releases/latest",
      responds with `status` and (for 200) a canned /releases/latest body whose tag_name == tag; increments a
      request counter (atomic int32) on every request. ts.Close via t.Cleanup. For status!=200 the body is a
      GitHub-shaped `{"message":"Not Found"}`. (Mirror releases_test.go::cannedLatest, parameterized by tag.)
      canned body shape (MUST decode via ghRelease — releases.go):
        `{"tag_name":"<tag>","name":"Release","prerelease":false,"draft":false,
          "assets":[{"name":"stagecoach_x_linux_amd64.tar.gz","browser_download_url":"https://example.com/a","size":1}]}`
  - HELPER: setupCheckSeams(t) (restore func):
      Captures + restores the seams the --check path reads. Returns nothing; wires t.Cleanup to:
        (a) upgradeBaseURL ← "" (the default) — but the CALLER sets upgradeBaseURL=ts.URL after calling this
            (or this helper takes the ts.URL). Prefer: this helper takes (t, tsURL) and sets upgradeBaseURL=
            tsURL immediately + t.Cleanup restores "". (single source of truth for the network seam.)
        (b) upgrade.SetCurrentVersion restored to "dev" via t.Cleanup (the test process never ran main.go, so
            "dev" is the original; no getter exists — documented).
        (c) the caller still owns saveRootState/restoreRootState + resetFlags(upgradeCmd.Flags()) (those reset
            cobra flag state; this helper owns the non-flag seam vars).
      Alternative: inline the seam setup in each test (5 cases). The helper dedupes; either is acceptable.
  - HELPER (driver): runUpgradeCheck(t) (stdout, stderr *bytes.Buffer, err error):
      Mirrors TestUpgradeCommand_NoBootstrapOutsideRepo: saveRootState/restoreRootState + resetFlags(
      upgradeCmd.Flags()) via t.Cleanup; isolates HOME (t.TempDir + XDG_CONFIG_HOME=""); wires outBuf/errBuf to
      rootCmd.SetOut/SetErr; rootCmd.SetArgs(["upgrade","--check"]); returns (Execute(ctx), &outBuf, &errBuf).
      (Execute returns runUpgrade's error — cobra SilenceErrors ⇒ returned not printed.)
  - NAMING: newCheckFake, setupCheckSeams (or inline), runUpgradeCheck (or inline). All unexported, package cmd.

Task 2: TestUpgradeCheck_Behind_Exit6  (contract case a)
  - ts, n := newCheckFake(t, "v0.2.0", 200); setupCheckSeams(t, ts.URL); upgrade.SetCurrentVersion("0.1.0")
    (the per-case version; restored to "dev" by the helper's t.Cleanup — order: helper registers restore FIRST,
    so the per-case SetCurrentVersion is unwound correctly; or set version AFTER setupCheckSeams and rely on the
    helper's t.Cleanup("dev") reset running LAST — confirm LIFO cleanup gives the right final state = "dev").
  - _, outBuf, errBuf, err := runUpgradeCheck(t)
  - ASSERT: exitcode.For(err) == exitcode.UpdateAvailable (6).
  - ASSERT: outBuf contains "update available" AND "v0.1.0 → v0.2.0" AND "→" AND `run "stagecoach upgrade"`.
    (Match runCheck's exact Fprintf — read internal/cmd/upgrade_run.go::runCheck. Use strings.Contains.)
  - ASSERT: errBuf is EMPTY (no "stagecoach:" double-prefix — the exit-6 nil-Err gate). `if errBuf.Len()!=0`.
  - ASSERT: exactly 1 request to the fake: atomic.LoadInt32(n) == 1.

Task 3: TestUpgradeCheck_UpToDate_Exit0  (contract case b)
  - ts,_ := newCheckFake(t, "v0.2.0", 200); setupCheckSeams(t, ts.URL); upgrade.SetCurrentVersion("0.2.0").
  - _, outBuf, _, err := runUpgradeCheck(t)
  - ASSERT: err == nil (so exitcode.For(err)==0). (Tolerate ONLY nil.)
  - ASSERT: outBuf contains "up to date" AND "v0.2.0" (appears twice: current + latest).

Task 4: TestUpgradeCheck_Dev_Exit0  (contract case c)
  - ts,_ := newCheckFake(t, "v0.2.0", 200); setupCheckSeams(t, ts.URL); upgrade.SetCurrentVersion("dev").
    (currentVersion default is already "dev"; setting it explicitly is robust against test ordering.)
  - _, outBuf, _, err := runUpgradeCheck(t)
  - ASSERT: err == nil (exit 0).
  - ASSERT: outBuf contains "development build — cannot compare" (match runCheck's dev Fprintf; note the em-dash "—").
  - ASSERT: outBuf does NOT contain "update available" (FR-U6: dev never falsely signals).

Task 5: TestUpgradeCheck_NoReleases_Exit1  (contract case d)
  - ts,_ := newCheckFake(t, "", 404)  (tag unused; status 404 ⇒ GitHub `{"message":"Not Found"}`)
  - setupCheckSeams(t, ts.URL); upgrade.SetCurrentVersion("0.1.0") (any parseable current; the 404 short-circuits).
  - _, _, _, err := runUpgradeCheck(t)
  - ASSERT: exitcode.For(err) == exitcode.Error (1).
  - ASSERT: errors.Is(err, upgrade.ErrNoReleases) == true (the *ExitError.Unwrap chain reaches ErrNoReleases;
    runCheck wraps `fmt.Errorf("stagecoach: check: %w", <ErrNoReleases-wrapped>)` then exitcode.New(Error, …)).

Task 6: TestUpgradeCheck_NoOnDiskChange  (contract case e)
  - ts,_ := newCheckFake(t, "v0.2.0", 200); setupCheckSeams(t, ts.URL); upgrade.SetCurrentVersion("0.1.0")
    (so the FULL --check path runs incl. the behind branch — proving even the "would update" path touches nothing).
  - BEFORE: glob0,_ := filepath.Glob(filepath.Join(os.TempDir(), "stagecoach-upgrade-*")); exeStat0,_ :=
    os.Stat(os.Executable()) (capture Size+ModTime; tolerate the rare os.Executable error by skipping the exe
    assertion with t.Skip if it fails — but on Linux/macOS/Windows test runners it succeeds).
  - _, _, _, err := runUpgradeCheck(t)
  - ASSERT: exitcode.For(err) == exitcode.UpdateAvailable (6) — confirms the full --check path executed.
  - AFTER: glob1,_ := filepath.Glob(...); exeStat1,_ := os.Stat(os.Executable()).
  - ASSERT: reflect.DeepEqual(glob0, glob1) (no new "stagecoach-upgrade-*" temp dir). (import reflect OR compare
    len + join — DeepEqual is cleanest.)
  - ASSERT: exeStat0.Size()==exeStat1.Size() && exeStat0.ModTime().Equal(exeStat1.ModTime()) (running exe untouched).
  - NOTE: --check never calls StageNewBinary/Swap (runCheck only does ResolveTarget+CurrentSemver+Compare), so
    these invariants hold by construction; the test MAKES the guarantee explicit (a future refactor that routes
    --check through a temp dir would fail here).

Task 7: VERIFY — build, vet, format, test, lint, scope guard
  - go build ./... ; go vet ./internal/cmd/... ; gofmt -l internal/cmd/upgrade_check_test.go   # empty
  - go test ./internal/cmd/ -run TestUpgradeCheck -race -v    # 5/5 PASS, no network (offline-capable)
  - go test ./internal/cmd/ -race                            # no sibling broke (seam vars restored via t.Cleanup)
  - make test && make lint                                   # staticcheck/unused clean (the test file has no U1000)
  - grep guard + git status (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the fake server — mirror releases_test.go, parameterized by tag + status):
func newCheckFake(t *testing.T, tag string, status int) (*httptest.Server, *int32) {
	t.Helper()
	var n int32
	body := fmt.Sprintf(`{"tag_name":%q,"name":"Release","prerelease":false,"draft":false,"assets":[{"name":"stagecoach_x_linux_amd64.tar.gz","browser_download_url":"https://example.com/a","size":1}]}`, tag)
	if status != 200 { body = `{"message":"Not Found"}` }
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		if status == 200 && !strings.HasSuffix(r.URL.Path, "/releases/latest") {
			http.NotFound(w, r); return
		}
		w.WriteHeader(status); _, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	return ts, &n
}

// PATTERN (drive runUpgrade end-to-end — the existing upgrade_test.go idiom):
func runUpgradeCheck(t *testing.T) (outBuf, errBuf *bytes.Buffer, err error) {
	t.Helper()
	_, origOut, origErr, origRunE := saveRootState(t)
	t.Cleanup(func() { restoreRootState(t, nil, origOut, origErr, origRunE) })
	t.Cleanup(func() { resetFlags(upgradeCmd.Flags()) })  // <-- SEPARATE from restoreRootState
	t.Setenv("HOME", t.TempDir()); t.Setenv("XDG_CONFIG_HOME", "")  // LoadUpgradeConfig ⇒ Defaults
	outBuf, errBuf = &bytes.Buffer{}, &bytes.Buffer{}
	rootCmd.SetOut(outBuf); rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"upgrade", "--check"})
	err = Execute(context.Background())
	return outBuf, errBuf, err
}

// PATTERN (the seam override + restore — the network seam + the version seam):
func setupCheckSeams(t *testing.T, tsURL string) {
	t.Helper()
	origBase := upgradeBaseURL
	upgradeBaseURL = tsURL
	t.Cleanup(func() { upgradeBaseURL = origBase })  // restores "" (the default)
	t.Cleanup(func() { upgrade.SetCurrentVersion("dev") })  // no getter; "dev" is the package default
}
// NOTE: a per-case upgrade.SetCurrentVersion("0.1.0") AFTER setupCheckSeams is unwound by the helper's
// t.Cleanup("dev") because Go runs Cleanup LIFO — the per-case set has no registered cleanup of its own,
// and the helper's "dev" reset is the LAST word. (If you prefer, register the per-case set's own restore.)

// PATTERN (the exit-6 + no-double-prefix assertion):
err := <runUpgradeCheck err>
if got := exitcode.For(err); got != exitcode.UpdateAvailable { t.Errorf("exit = %d, want 6", got) }
if errBuf.Len() != 0 { t.Errorf("exit-6 stderr must be empty (no double 'stagecoach:'); got %q", errBuf.String()) }

// PATTERN (the 404 ⇒ ErrNoReleases + exit 1 assertion):
if got := exitcode.For(err); got != exitcode.Error { t.Errorf("exit = %d, want 1", got) }
if !errors.Is(err, upgrade.ErrNoReleases) { t.Errorf("err = %v, want errors.Is ErrNoReleases", err) }
```

### Integration Points

```yaml
CONSUMES (LANDED — READ-ONLY, zero edits):
  - internal/cmd (same package): upgradeBaseURL, upgradeNewClient, upgradeToken (seam vars, upgrade_run.go);
    runUpgrade (upgrade.go); flagCheck/flagTargetVersion/flagChannel/flagPrerelease (upgrade.go, set by cobra);
    saveRootState/restoreRootState/resetFlags/Execute/rootCmd/upgradeCmd (root.go/root_test.go).
  - internal/upgrade: SetCurrentVersion, CurrentSemver (version.go); ResolveTarget/Client/LatestStable
    (resolve.go/releases.go); ErrNoReleases (releases.go).
  - internal/exitcode: UpdateAvailable/Error/Success/For (exitcode.go).
  - stdlib: net/http, net/http/httptest, sync/atomic (or sync), os, path/filepath, bytes, strings, errors,
    context, testing.
NO database / migration / routes / config-struct change / exitcode-const change / go.mod change / docs change /
production-code change. TESTS-ONLY.
SCOPE FENCES:
  - Touches ONLY: internal/cmd/upgrade_check_test.go (NEW).
  - Does NOT edit: internal/cmd/upgrade.go, upgrade_run.go, upgrade_prompt.go (LANDED), internal/upgrade/*
    (read-only), root.go, exitcode.go, config, the commit path (FR-U12), go.mod, any PRD/task file.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (the test file compiles against the LANDED seams + helpers; stdlib only).
go build ./...
# Expected: clean. Watch: "upgradeBaseURL undefined" (it's in upgrade_run.go, same package — confirm),
#           "imported and not used" (reconcile imports; e.g. drop sync if you used sync/atomic).

# Vet the changed package (test files included).
go vet ./internal/cmd/...
# Expected: clean.

# Format.
gofmt -l internal/cmd/upgrade_check_test.go
# Expected: empty. If listed: gofmt -w it.

# Lint (staticcheck/unused/errcheck/gosimple). The test file must have no U1000 (every helper is called;
# every import used). t.Cleanup restores satisfy errcheck (ignored returns are fine in test helpers).
make lint
# Expected: zero errors.

# Scope guard: ONLY the new test file.
git status --porcelain
# Expected: internal/cmd/upgrade_check_test.go ONLY. FAIL if any production file appears.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Run the new --check suite (the 5 cases).
go test ./internal/cmd/ -run TestUpgradeCheck -race -v
# Expected: TestUpgradeCheck_Behind_Exit6 / _UpToDate_Exit0 / _Dev_Exit0 / _NoReleases_Exit1 /
#           _NoOnDiskChange — ALL PASS. No network (the fake is localhost; run with the network unplugged to
#           prove it — optional: `unshare -n` or just trust upgradeBaseURL=localhost).

# Full cmd-package regression (the seams are restored via t.Cleanup, so no leak into siblings).
go test ./internal/cmd/ -race
# Expected: green (S1's TestUpgradeCommand_* + S2's upgrade_prompt tests unaffected; the seam vars + flags
#           are restored between tests).

# Full race suite + lint + build.
make test && make lint && make build
# Expected: all green.
```

### Level 3: Integration Testing (System Validation)

```bash
# (This suite IS the integration test for --check — it drives runUpgrade end-to-end via Execute against the
#  fake. There is no separate "service" to start.) Optional manual cross-check against the REAL GitHub API
#  (network required; informational ONLY — not part of the suite):
make build
./bin/stagecoach upgrade --check; echo "exit=$?"
# Expected (a dev test build): "stagecoach dev (latest: v…; development build — cannot compare)" exit 0.
# (The suite's case-c replicates this against the fake with a pinned tag.)

# Offline-capability smoke (prove no real network): run the suite with DNS to api.github.com blocked, e.g.
#   go test ./internal/cmd/ -run TestUpgradeCheck -race   # passes because upgradeBaseURL=localhost
# (No command needed — the localhost fake is the proof. This line documents the guarantee.)
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: ONLY one new test file; ZERO production edits.
git status --porcelain
test "$(git status --porcelain | wc -l)" -eq 1 && echo "OK: one file" || echo "FAIL: expected one file"
git diff --name-only | grep -vE '^internal/cmd/upgrade_check_test\.go$' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
git diff --name-only | grep -qE '^internal/(upgrade|exitcode|cmd/(upgrade|upgrade_run|upgrade_prompt|root)\.go)' && echo "FAIL: edited LANDED production file" || echo "OK: production untouched"

# Guard 2: the 5 contract cases exist as tests.
for tc in Behind_Exit6 UpToDate_Exit0 Dev_Exit0 NoReleases_Exit1 NoOnDiskChange; do
  grep -q "func TestUpgradeCheck_$tc" internal/cmd/upgrade_check_test.go || echo "MISSING TestUpgradeCheck_$tc"
done

# Guard 3: the two load-bearing seams are exercised.
grep -n 'upgradeBaseURL = ts.URL\|upgradeBaseURL = ' internal/cmd/upgrade_check_test.go   # the network seam set
grep -n 'upgrade.SetCurrentVersion' internal/cmd/upgrade_check_test.go                      # the version seam (≥4: 0.1.0/0.2.0/dev + restore)

# Guard 4: the Execute driver + the mandatory resetFlags(upgradeCmd.Flags()).
grep -n 'Execute(context.Background())' internal/cmd/upgrade_check_test.go                  # the driver
grep -n 'resetFlags(upgradeCmd.Flags())' internal/cmd/upgrade_check_test.go                 # the LOCAL-flag reset (must be present)

# Guard 5: exit codes asserted via exitcode.For on the Execute return (NOT os.Exit / NOT raw code).
grep -n 'exitcode.For(err)' internal/cmd/upgrade_check_test.go                              # ≥3 (cases a/d/e + the err==nil checks)

# Guard 6: no real network — the fake is httptest (localhost), and upgradeBaseURL is set to it.
grep -n 'httptest.NewServer\|httptest.Server' internal/cmd/upgrade_check_test.go            # the fake
grep -n 'api.github.com' internal/cmd/upgrade_check_test.go && echo "WARN: hardcoded api.github.com in a test (must use the fake)" || echo "OK: no hardcoded API host"

# Guard 7: case (e) on-disk invariants asserted.
grep -n 'stagecoach-upgrade-\*\|filepath.Glob' internal/cmd/upgrade_check_test.go           # temp-dir glob (no new temp)
grep -n 'os.Executable()' internal/cmd/upgrade_check_test.go                                # the exe-untouched assertion

# Guard 8: the dev case asserts NO false update signal.
grep -n '!strings.Contains.*update available\|not.*update available' internal/cmd/upgrade_check_test.go   # case (c) negative

# Guard 9: the suite is offline-capable (run it; the localhost fake IS the no-network proof).
go test ./internal/cmd/ -run TestUpgradeCheck -race   # 5/5 PASS
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./internal/cmd/...` clean; `gofmt -l` empty on the new file
- [ ] `make lint` zero errors (no U1000 — every helper/import used; t.Cleanup restores satisfy errcheck)
- [ ] `go test ./internal/cmd/ -run TestUpgradeCheck -race -v` 5/5 PASS
- [ ] `go test ./internal/cmd/ -race` green (no sibling broke — seams/flags restored between tests)
- [ ] `make test` + `make build` green

### Feature Validation (the 5 contract cases)
- [ ] (a) behind: SetCurrentVersion("0.1.0") + fake tag "v0.2.0" ⇒ exitcode.For==6; stdout has
      `update available` + `v0.1.0 → v0.2.0`; stderr EMPTY (grep guard 5 + the errBuf=="" assert)
- [ ] (b) up-to-date: SetCurrentVersion("0.2.0") + fake tag "v0.2.0" ⇒ err==nil; stdout has `up to date`
- [ ] (c) dev: SetCurrentVersion("dev") ⇒ err==nil; stdout has `development build — cannot compare`; stdout
      does NOT contain `update available` (grep guard 8)
- [ ] (d) no-releases: fake 404 ⇒ exitcode.For==1 AND errors.Is(err, upgrade.ErrNoReleases)
- [ ] (e) no on-disk change: temp glob UNCHANGED + os.Executable() Size/ModTime UNCHANGED (grep guard 7)
- [ ] no real network: upgradeBaseURL=httptest.URL (localhost); exactly 1 GET to the fake (request counter)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `internal/cmd/upgrade_check_test.go` (NEW) (grep guard 1)
- [ ] NO edit to internal/cmd/upgrade.go, upgrade_run.go, upgrade_prompt.go (LANDED), internal/upgrade/*
      (read-only), root.go, exitcode.go, config, the commit path (FR-U12), go.mod, any PRD/task file

### Code Quality & Docs
- [ ] File doc comment names the suite (FR-U6 --check), the no-network seam (upgradeBaseURL), the version seam
      (SetCurrentVersion), and the dedicated-file rationale (S1 owns upgrade_test.go)
- [ ] Each test restores every seam it overrides via t.Cleanup (upgradeBaseURL→"", currentVersion→"dev",
      rootCmd state via saveRootState/restoreRootState, upgradeCmd flags via resetFlags)
- [ ] Assertions match runCheck's REAL Fprintf formats (v-prefixed `v0.1.0 → v0.2.0`), not the contract's
      shorthand — the implementer reads internal/cmd/upgrade_run.go::runCheck to confirm exact strings

---

## Anti-Patterns to Avoid

- ❌ Don't assert the bare `0.1.0 → 0.2.0` (no v-prefix). CurrentSemver returns the v-PREFIXED canonical
  ("v0.1.0"); runCheck prints `update available: v0.1.0 → v0.2.0; …`. Match the real Fprintf (read
  upgrade_run.go). Asserting the bare form WILL fail. (Robust fallback: assert substrings "update
  available" + "→" + exit 6.)
- ❌ Don't omit `resetFlags(upgradeCmd.Flags())`. restoreRootState resets only rootCmd's flags; the upgrade
  LOCAL flags (flagCheck/flagVersion/flagChannel) are bound to upgradeCmd.Flags() and leak across tests in
  the same binary without an explicit reset. Mirror upgrade_test.go's two-defer idiom.
- ❌ Don't call runUpgrade directly with a hand-built `*cobra.Command`. Use `Execute(ctx)` with
  `["upgrade","--check"]` (the existing pattern) so the REAL prologue (validateUpgradeFlags +
  LoadUpgradeConfig + effChannel resolution + seam wiring) runs. A direct call bypasses the prologue and
  tests less. (Execute returns runUpgrade's error — cobra SilenceErrors=true.)
- ❌ Don't leave HOME unisolated. runUpgrade calls LoadUpgradeConfig (reads the global config under
  $HOME/$XDG). A real config's [upgrade].channel/source_repo would change effChannel/effSourceRepo and
  flake the test. Isolate with t.TempDir() + XDG_CONFIG_HOME="".
- ❌ Don't forget to restore the version seam. There is no Get; the test process starts at "dev". Register
  `t.Cleanup(func(){ upgrade.SetCurrentVersion("dev") })` so a non-dev set (case a/b) doesn't leak into a
  sibling. ("dev" round-trips because SetCurrentVersion accepts non-empty and ParseAndClean("dev")⇒dev.)
- ❌ Don't hardcode `api.github.com` anywhere in the test. The fake is `httptest.NewServer` (localhost) and
  `upgradeBaseURL = ts.URL` is the only network target. A hardcoded host would make a real network call
  (violating the no-network requirement) and flake offline.
- ❌ Don't assert the exit-6 path emits a stderr line. runCheck returns `exitcode.New(UpdateAvailable, nil)`
  ⇒ ExitError.Err==nil ⇒ Error()=="" ⇒ main.go's gate skips the print; cobra (SilenceErrors) doesn't print
  either. errBuf is EMPTY on the behind path — assert that (it encodes the no-double-prefix contract).
- ❌ Don't add production code. This item is TESTS-ONLY. runCheck + the seams are LANDED (P1.M4.T2.S1); this
  suite consumes them. Any edit to upgrade.go/upgrade_run.go/internal/upgrade/* is out of scope.
- ❌ Don't conflate this suite's seams with the swap/rollback seams. --check reads ONLY upgradeBaseURL +
  upgradeToken + upgradeNewClient (and, via runCheck, CurrentSemver/SetCurrentVersion). upgradeExePath /
  upgradeExecRunner / upgradeDetect / upgradeSwap / upgradeRollback belong to S2/S3 — overriding them here
  is dead code (linter U1000 if unused, and misleading).