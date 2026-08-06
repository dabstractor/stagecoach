# Findings — P1.M4.T2.S1 (runUpgrade dispatcher: --check / normal / --rollback)

The CLI shell (S1: upgradeCmd + 9 flags + validateUpgradeFlags + runUpgrade prologue) and the prompt
helpers (S2: confirmUpgrade + printDelegatedUpdate + printPrivilegeCommand + confirmUpgradeIsTTY) are
DONE. MY task replaces runUpgrade's `TODO(P1.M4.T2.S1)` PLACEHOLDER with the real dispatch that consumes
the LANDED internal/upgrade primitives (P1.M1–P1.M3 = Complete), + the injectable test seams.

## 0. The runUpgrade PLACEHOLDER I replace (internal/cmd/upgrade.go, S1, LANDED)
`runUpgrade(cmd, _)`: validateUpgradeFlags → config.LoadUpgradeConfig → resolve effChannel/effSourceRepo
(flag > [upgrade] config > Defaults; NO env for channel/source-repo) → THEN the block:
```go
// TODO(P1.M4.T2.S1): dispatch ...
fmt.Fprintf(cmd.ErrOrStderr(), "stagecoach upgrade: not yet implemented (channel=%s source=%s)\n", effChannel, effSourceRepo)
return nil
```
MY task KEEPS the validation + resolution prologue; REPLACES the TODO+notice with the dispatch. runUpgrade
signature stays `(cmd *cobra.Command, _ []string) error`; NEVER os.Exit (returns exitcode.New/nil).

## 1. The LANDED internal/upgrade API I consume (verified signatures + sentinels)

### version.go — the comparable version (FR-U6/--check + the direct-swap confirm display)
- `func CurrentSemver() (string, bool)` — (canonical "vX.Y.Z", ok). Dev/unparseable → ("", false).
- `func Compare(a, b string) int` — -1/0/+1; **unparseable operand ⇒ 0** (dev never falsely signals update).
- `func SetCurrentVersion(v string)` — package-upgrade test seam (P1.M4.T3 pins current via this).
- NOTE: CurrentSemver returns ("",false) for dev; there is NO exported raw getter. For the --check "dev"
  display line, print "dev" (the only !ok case in practice).

### releases.go — the Client (MY runUpgrade constructs it; Token from env)
- `type Client struct { HTTP *http.Client; BaseURL string; Repo string; Token string }`.
  HTTP nil ⇒ http.DefaultClient; BaseURL "" ⇒ "https://api.github.com"; Repo "owner/repo"; Token "" ⇒ unauth.
- Methods: `LatestStable(ctx)`, `LatestAdmittingPrereleases(ctx)`, `ReleaseByTag(ctx, tag)`,
  `FetchChecksums(ctx, release)`, `DownloadFile(ctx, url, dest)`. (ResolveTarget composes these.)
- The Client is the NETWORK seam: tests inject `BaseURL = httptest.Server.URL`.

### resolve.go — target resolution (FR-U5 step 2-3)
- `func ResolveTarget(ctx, c *Client, opts ResolveOptions) (Release, Asset, error)`.
- `ResolveOptions{Version string; Prerelease bool; GOOS, GOARCH string}`. Version > Prerelease > LatestStable.
- `Release{Tag string; Assets []Asset}`. ResolveTarget does NOT compare to current (MY runUpgrade does).

### detect.go — install-method detection (FR-U2)
- `type Channel string`; consts ChannelBrew/Scoop/Winget/AUR/Npm/Mise/Asdf/Nix/GoInstall/Direct.
  **ChannelDirect = "direct" is the ONLY self-swap-eligible channel (FR-U1/U5).**
- `type Detector struct { Exec Runner; ExePath string; GOOS string; Override string; Env func(string)string; Log func(string) }`.
  **CRITICAL: `Exec nil ⇒ SKIP PM probes` (NOT a prod default). Production MUST pass a real Runner.**
- `type Runner interface { Run(ctx, name, args...) (stdout string, exitCode int, err error) }`.
- `func (d *Detector) Detect(ctx) (Channel, string, error)` — returns (channel, evidence, err).
- `var ErrUnknownChannel` — invalid --install-method override (→ exit 1). flagChannel enum is validated by
  S1; --install-method is NOT pre-validated, so Detect surfaces ErrUnknownChannel.
- **osRunner is UNEXPORTED** — package cmd CANNOT construct `&osRunner{}`. (See §3 seam decision.)

### delegate.go — delegation dispatcher (FR-U3/U4)
- `func Delegate(ctx, ch Channel, opts DelegateOptions) (DelegateResult, error)`.
- `DelegateOptions{Exec ExecRunner; Out io.Writer; Env func(string)string; Verbose func(string); Confirmed bool}`.
  **Exec nil ⇒ osExecRunner (prod default) — UNLIKE Detector, nil IS the prod default here.** Out required.
- `type ExecRunner interface { Run(ctx, stdout, stderr io.Writer, name, args...) (exitCode int, err error) }`.
- `DelegateResult{Ran bool; Command string; ExitCode int}`.
  - ChannelDirect → `(DelegateResult{}, ErrDirectSwap)` sentinel → MY runUpgrade routes to self-swap.
  - ChannelAUR/ChannelNix → PRINT → `(DelegateResult{Ran:false, Command:primary, ExitCode:0}, nil)`.
  - RUN channels → `(DelegateResult{Ran:true, ExitCode}, nil)` (map the code) OR `(..., err)` start-failure.
- `var ErrDirectSwap` — the route-to-self-swap sentinel (MY branch on `ch==Direct||flagForce` prevents it).

### stage.go + swap.go — the direct swap (FR-U5/U7/U11)
- `func StageNewBinary(ctx, c, release, asset, tempDir string) (newBinPath string, err error)` — download+
  verify+extract+sanity-run; LEAVES tempDir on failure (inspection) AND on success (Swap cleans it).
- `func Swap(ctx, newBinPath string) error` — backup+atomic rename. Non-writable → `*NeedsPrivilegesError`.
  On success Swap best-effort cleans `filepath.Dir(newBinPath)` (= tempDir). NEVER os.Exit.
- `type NeedsPrivilegesError struct { Command string }` — errors.Is(ErrNeedsPrivileges); `.Command` = the sudo
  re-run (`sudo "<exe>" upgrade` on Unix). MY runUpgrade prints it + exit 0 (FR-U4/FR-U7).
- `var resolveCurrentExe = func() (string, error){...}` — **PACKAGE-UPGRADE-level** exe seam (Swap/Rollback
  resolve the exe INTERNALLY; package cmd cannot redirect it). (See §3 seam decision.)

### rollback.go — --rollback (FR-U8)
- `func Rollback(ctx) (restoredVersion string, err error)`.
- `var ErrNoBackup` — no prior backup → MY runUpgrade prints "no backup — nothing to roll back" + exit 0.
- `var ErrBackupUnusable` — backup's --version fails → exit non-zero (general error → exit 1).

## 2. The S2 helpers I consume (internal/cmd/upgrade_prompt.go, LANDED)
- `confirmUpgrade(current, target, action string, assumeYes bool, in io.Reader, out io.Writer) (bool, error)`
  — 4 outcomes: assumeYes→(true,nil); non-TTY+no-yes→(false,err, msg has "re-run with --yes", NO
  "stagecoach:" prefix); TTY 'y'/'Y'→(true,nil); TTY else→(false,nil)+"aborted".
- `var confirmUpgradeIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }` — the TTY seam (S2 owns it).
- `printDelegatedUpdate(out, channel, command)` — FR-U4 PRINT framing (echoes command verbatim).
- `printPrivilegeCommand(out, command)` — FR-U7 needs-privileges framing (echoes command verbatim).
- I wire these from runUpgrade; the isTTY/stdin seams are S2's (cmd.InOrStdin()/OutOrStdout() feed in/out).

## 3. THE SEAM DESIGN (the crux — read carefully)

The contract: "Expose seams (Client/BaseURL, exec-runner, os.Executable, isTTY, stdin) as package vars
... so P1.M4.T3 can drive everything against an httptest server with no real network and no real binary
swap." Two walls force FUNCTION seams (not just field injection):

- **Detector.Exec nil SKIPS probes** (detect.go) and **osRunner is UNEXPORTED** → package cmd cannot
  build a production Detector by field-injection. ⇒ expose `upgradeDetect func(ctx, override, log)
  (Channel, string, error)` whose PRODUCTION default builds a Detector with a package-cmd Runner
  (`cmdRunner`, ~15 lines implementing upgrade.Runner via exec.CommandContext, SAME contract as
  osRunner: non-zero ExitError ⇒ (stdout, code, nil); start-failure/ctx ⇒ err). P1.M4.T3 overrides
  upgradeDetect to return a canned channel — NO real subprocess.
- **Swap/Rollback resolve the exe INTERNALLY** via the package-upgrade-level `resolveCurrentExe` →
  package cmd cannot steer the swap exe by field-injection. ⇒ expose `upgradeSwap func(ctx, newBinPath)
  error` (default upgrade.Swap) + `upgradeRollback func(ctx) (string, error)` (default upgrade.Rollback).
  P1.M4.T3 overrides them (or overrides resolveCurrentExe in a package-upgrade test).

PLUS the contract's literal field seams:
- `upgradeBaseURL string` (default "") — flows into the Client (NETWORK seam; P1.M4.T3 ⇒ httptest URL).
- `upgradeExePath func() (string, error)` (default os.Executable+EvalSymlinks) — feeds Detector.ExePath.
- `upgradeExecRunner upgrade.ExecRunner` (default nil ⇒ Delegate's osExecRunner) — feeds DelegateOptions.Exec.
- `upgradeToken func() string` (default STAGECOACH_GITHUB_TOKEN||GITHUB_TOKEN) — Client.Token.
- `upgradeNewClient func(repo, token string) *Client` (default builds Client{BaseURL: upgradeBaseURL,...}).

runUpgrade calls upgrade.ResolveTarget / upgrade.StageNewBinary / upgrade.CurrentSemver / upgrade.Delegate
DIRECTLY (steered by the Client + upgradeExecRunner seams — those don't need function seams). The
function seams (upgradeDetect/upgradeSwap/upgradeRollback) exist ONLY where field-injection can't reach
(unexported osRunner; internal resolveCurrentExe). This is the MINIMAL surface that satisfies "no real
network / no real binary swap / no real subprocess" for P1.M4.T3.

ALL seams live in package cmd (a NEW file internal/cmd/upgrade_run.go — separate from S1's upgrade.go to
avoid churning S1's tested prologue). NO edits to internal/upgrade/* (P1.M2/M3 Complete, read-only).

## 4. The dispatch logic (the three paths)

```
runUpgrade (after S1 prologue):  ctx=cmd.Context(); stdout=OutOrStdout(); stderr=ErrOrStderr()
  log := verboseLog(stderr)  # func(string), gated on flagVerbose (FR50)

  if flagRollback:                                   # --rollback path (FR-U8)
      ver, err := upgradeRollback(ctx)
      if errors.Is(err, upgrade.ErrNoBackup): fmt.Fprintln(stdout, "no backup — nothing to roll back"); return nil  # exit 0
      if err != nil: return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: rollback: %w", err))
      fmt.Fprintf(stdout, "restored stagecoach %s\n", ver); return nil  # exit 0

  client := upgradeNewClient(effSourceRepo, upgradeToken())

  if flagCheck: return runCheck(ctx, cmd, client, effChannel)   # --check path (FR-U6) → exit 0 or 6

  ch, evidence, err := upgradeDetect(ctx, flagInstallMethod, log)   # normal path (FR-U1)
  if err != nil: return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))  # ErrUnknownChannel
  log("detected install method: " + string(ch) + " (" + evidence + ")")
  if ch == upgrade.ChannelDirect || flagForce:
      if flagForce && ch != upgrade.ChannelDirect:
          fmt.Fprintln(stderr, "stagecoach: warning: --force overriding a detected "+string(ch)+" install (FR-U1); self-swapping")
      return runDirectSwap(ctx, cmd, client, effChannel)
  return runDelegate(ctx, cmd, ch)

runCheck:  cur,ok := upgrade.CurrentSemver()
  release,_,err := upgrade.ResolveTarget(ctx, client, ResolveOptions{Version: flagTargetVersion, Prerelease: effChannel=="prerelease"})
  if err != nil: return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: check: %w", err))
  latest := release.Tag
  if !ok: fmt.Fprintf(stdout, "stagecoach dev (latest: %s; development build — cannot compare)\n", latest); return nil  # exit 0
  if upgrade.Compare(cur, latest) >= 0: fmt.Fprintf(stdout, "stagecoach %s (latest: %s; up to date)\n", cur, latest); return nil  # exit 0
  fmt.Fprintf(stdout, "update available: %s → %s; run \"stagecoach upgrade\"\n", cur, latest)
  return exitcode.New(exitcode.UpdateAvailable, nil)   # exit 6 (VERIFY main.go prints cleanly — see §5)

runDirectSwap:  tempDir,err := os.MkdirTemp("", "stagecoach-upgrade-*"); if err: return Error
  release,asset,err := upgrade.ResolveTarget(ctx, client, ResolveOptions{...}); if err: return Error (LEAVE tempDir)
  newBin,err := upgrade.StageNewBinary(ctx, client, release, asset, tempDir); if err: return Error (LEAVE tempDir, FR-U11)
  ok,err := confirmUpgrade(displayCurrent(), release.Tag, "Self-swap the direct-binary install.", flagYes, cmd.InOrStdin(), stdout)
    if err: return exitcode.New(exitcode.Error, err)  # non-TTY refusal → exit 1
    if !ok: return nil  # TTY decline → exit 0
  if err := upgradeSwap(ctx, newBin); err != nil:
      var npe *upgrade.NeedsPrivilegesError
      if errors.As(err, &npe): printPrivilegeCommand(stdout, npe.Command); return nil  # exit 0 (FR-U4/U7)
      return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))
  fmt.Fprintf(stdout, "stagecoach upgraded to %s\n", release.Tag); return nil  # exit 0
  # NO defer cleanup: Swap cleans tempDir on success (swap.go step 4); failure paths LEAVE it (FR-U11 inspection).

runDelegate:  isRun := ch != ChannelAUR && ch != ChannelNix   # FR-U9: confirm only RUN channels
  if isRun:
      ok,err := confirmUpgrade(displayCurrent(), "latest", "Update via "+string(ch)+"'s updater.", flagYes, cmd.InOrStdin(), stdout)
        if err: return exitcode.New(exitcode.Error, err); if !ok: return nil
  res,err := upgrade.Delegate(ctx, ch, DelegateOptions{Exec: upgradeExecRunner, Out: stdout, Env: os.Getenv, Verbose: log, Confirmed: flagYes})
    if err: return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err))  # start failure → exit 1
    if !res.Ran: printDelegatedUpdate(stdout, string(ch), res.Command); return nil  # PRINT (AUR/Nix) → exit 0
    if res.ExitCode != 0: return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %s updater exited %d", ch, res.ExitCode))  # exit 1
    fmt.Fprintf(stdout, "stagecoach updated via %s\n", ch); return nil  # exit 0
  # NOTE: ErrDirectSwap is UNREACHABLE here (the ch==Direct||flagForce branch routed direct away first).

displayCurrent():  cur,ok := upgrade.CurrentSemver(); if ok: return cur; return "dev"
verboseLog(stderr):  return func(msg string) { if flagVerbose { fmt.Fprintln(stderr, "stagecoach: "+msg) } }
```

## 5. Exit-code discipline (§15.4 upgrade block = 0/1/6 ONLY)
- exit 0: up-to-date/--check dev/upgraded/printed/declined/needs-privileges/no-backup.
- exit 1: general failure (Resolve/Stage/Swap-non-privilege/Rollback-unusable/Delegate-start-failure/
  RUN-updater-non-zero/ErrUnknownChannel/non-TTY-confirm-refusal).
- exit 6: --check behind. **VERIFY** main.go's print of `exitcode.New(exitcode.UpdateAvailable, nil)`:
  ExitError.Error() with nil Err — confirm it doesn't emit an ugly "stagecoach: " line (if it does, the
  message is already on stdout; acceptable, or use a sentinel-free return). For() maps *ExitError→Code=6.
- NEVER 2/3/5/124 from upgrade (FR-U12). NEVER os.Exit (return exitcode.New/nil; main maps via For()).

## 6. Verbose (FR50)
flagVerbose (root.go package var, set by --verbose/-v REGARDLESS of the no-op PersistentPreRunE) gates a
`func(string)` logger wired into Detector.Log + DelegateOptions.Verbose. Log: detect steps, resolved
channel+evidence, target version, planned action (direct-swap vs delegate). ui.NewVerbose exists but the
func(string) seams prefer a simple stderr-gated closure (no ui import needed in upgrade_run.go).

## 7. Scope fence (touch ONLY these)
TOUCH: `internal/cmd/upgrade.go` (EDIT — replace the TODO placeholder block in runUpgrade with
`return dispatchUpgrade(...)` or inline the dispatch) + `internal/cmd/upgrade_run.go` (NEW — the dispatch
helpers runCheck/runDirectSwap/runDelegate + the seam package vars + cmdRunner + production defaults).
DO NOT TOUCH: PRD.md, plan/**, tasks.json, prd_snapshot.md, internal/upgrade/* (read-only; the seams make
it testable WITHOUT editing them), internal/cmd/upgrade_prompt.go (S2, read-only — I CONSUME it),
root.go, config.Load, exitcode.go, the commit path (FR-U12), go.mod, docs/cli.md (S1 owns Mode-A), any
PRD/task file. P1.M4.T3 (parallel/next) writes the tests that override these seams.