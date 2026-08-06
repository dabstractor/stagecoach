name: "P1.M4.T2.S1 — runUpgrade dispatcher (--check / normal detect→delegate|swap / --rollback) + injectable test seams (FR-U1/U4/U5/U6/U12)"
description: >
  The ORCHESTRATION of `stagecoach upgrade` (PRD §9.29). REPLACES the `TODO(P1.M4.T2.S1)` PLACEHOLDER in
  internal/cmd/upgrade.go's runUpgrade (S1 shipped the command shell + validation + channel/source-repo
  resolution; S2 shipped confirmUpgrade + printDelegatedUpdate/printPrivilegeCommand + confirmUpgradeIsTTY)
  with the real dispatch that consumes the LANDED internal/upgrade primitives (P1.M1–P1.M3 = Complete):
  CurrentSemver/Compare (version), Client (releases), ResolveTarget (resolve), Detector.Detect (detect),
  Delegate/DelegateResult (delegate), StageNewBinary (stage), Swap/NeedsPrivilegesError (swap),
  Rollback/ErrNoBackup (rollback). THREE branches: (1) --rollback → upgradeRollback; ErrNoBackup → print +
  exit 0. (2) --check (FR-U6) → ResolveTarget + CurrentSemver + Compare; up-to-date/dev → exit 0; behind →
  print + exit 6. (3) normal → upgradeDetect; if ChannelDirect (or --force with a warning, FR-U1) →
  direct-swap (ResolveTarget→StageNewBinary→confirmUpgrade→upgradeSwap; NeedsPrivileges→printPrivilegeCommand
  + exit 0); else → Delegate (RUN channels confirm first per FR-U9; PRINT channels AUR/Nix →
  printDelegatedUpdate + exit 0; RUN exit code mapped 0→0 / non-zero→1). Verbose (FR50): log detect steps,
  channel+evidence, target, planned action. Exit codes 0/1/6 ONLY (§15.4; FR-U12). NEVER os.Exit. PLUS the
  INJECTABLE SEAMS (the contract's test wall for P1.M4.T3): package-cmd vars — upgradeBaseURL (Client/BaseURL
  ⇒ httptest), upgradeExePath (os.Executable), upgradeExecRunner (Delegate ExecRunner), upgradeToken,
  upgradeNewClient, AND function seams upgradeDetect/upgradeSwap/upgradeRollback (required because
  Detector.Exec=nil SKIPS probes + osRunner is UNEXPORTED, and Swap/Rollback resolve the exe INTERNALLY —
  field-injection can't reach those; the function seams let P1.M4.T3 drive --check/direct-swap/rollback with
  NO real network, NO real subprocess, NO real binary rename). All seams live in package cmd — ZERO edits to
  internal/upgrade/* (read-only). TOUCHES: internal/cmd/upgrade.go (EDIT the placeholder) +
  internal/cmd/upgrade_run.go (NEW — dispatch helpers + seams + cmdRunner). Plus a smoke test (upgrade
  --help already covered by S1; add a runUpgrade-smoke if S1's doesn't exercise the dispatch). NO docs
  (S1 owns Mode-A docs/cli.md; P3 owns changeset docs).

---

## Goal

**Feature Goal**: Wire the `stagecoach upgrade` command to its LANDED internal/upgrade machinery so the
three user paths actually work: `--check` (FR-U6, exit 0/6), `--rollback` (FR-U8, exit 0), and the normal
detect→delegate|direct-swap path (FR-U1/U4/U5) — each producing ONLY §15.4 upgrade-block exit codes
(0/1/6, FR-U12), never touching the commit core, and fully testable by P1.M4.T3 against an httptest fake
with no real network/subprocess/binary-swap.

**Deliverable**:
1. `internal/cmd/upgrade.go` (EDIT, S1's file) — replace the `TODO(P1.M4.T2.S1)` + stderr-notice block in
   `runUpgrade` with the real dispatch (keep S1's validation + resolution prologue intact). runUpgrade stays
   `(cmd *cobra.Command, _ []string) error`, never os.Exit.
2. `internal/cmd/upgrade_run.go` (NEW) — the three path helpers (`runCheck`, `runDirectSwap`, `runDelegate`)
   + the small display/verbose helpers + the injectable SEAM package vars (upgradeBaseURL, upgradeExePath,
   upgradeExecRunner, upgradeToken, upgradeNewClient, upgradeDetect, upgradeSwap, upgradeRollback) + their
   production defaults + `cmdRunner` (the package-cmd `upgrade.Runner` for the production Detector, since
   upgrade.osRunner is unexported).
3. (Optional) a smoke test exercising one dispatch path through the seams — ONLY if S1's registration test
   doesn't already cover runUpgrade. (S1's TestUpgradeCommand_* cover the shell, not the dispatch; the
   dispatch's full coverage is P1.M4.T3. A single seam-driven smoke test here proves the wiring compiles +
   routes; it is NOT the regression net.)

**Success Definition**:
- `stagecoach upgrade --check` resolves current vs latest, prints the FR-U6 lines, and exits 0 (up-to-date
  or dev) or 6 (behind). `--rollback` prints "no backup — nothing to roll back" (exit 0) on ErrNoBackup.
  The normal path detects the channel; ChannelDirect (or --force + warning) self-swaps
  (ResolveTarget→Stage→confirm→Swap; NeedsPrivileges→print sudo + exit 0); other channels Delegate (RUN
  channels confirm first; PRINT channels print + exit 0; RUN non-zero → exit 1).
- Exit codes are 0/1/6 ONLY — never 2/3/5/124 (FR-U12). runUpgrade NEVER os.Exit.
- Every network/subprocess/filesystem touch is steered by a package-cmd seam, so P1.M4.T3 can run the full
  dispatch against an httptest server + fake runners + a temp exe (no real network, no real subprocess, no
  real binary rename) by overriding the seam vars.
- `go build ./...` clean; `go vet ./internal/cmd/...` clean; `gofmt -l` empty; `make test` + `make lint`
  clean (no U1000 — every seam var is read by runUpgrade; cmdRunner is used by the prodDetect default).
- `git status --porcelain` == `internal/cmd/upgrade.go` + `internal/cmd/upgrade_run.go` (+ optional smoke
  test). ZERO edits to internal/upgrade/*, upgrade_prompt.go (S2), root.go, config.Load, exitcode.go,
  docs/cli.md (S1), the commit path (FR-U12), go.mod, any PRD/task file.

## User Persona (if applicable)

**Target User**: A Stagecoach user keeping the binary current: `stagecoach upgrade` (normal delegate/swap),
`stagecoach upgrade --check` (CI/cron gate, exit 6 if behind), or `stagecoach upgrade --rollback` (undo).
**Use Case**: one memorable command across every install channel that never fights a package manager
(delegate-first) and never silently swaps a binary (FR-U9 confirm; FR-U4 print-don't-sudo).
**User Journey**: `upgrade --check` → "update available: 1.2.3 → 1.3.0" (exit 6) → `upgrade` → detect →
(brew: confirm + `brew upgrade stagecoach` streamed, exit 0) OR (direct: confirm + atomic swap, exit 0)
OR (/usr/local/bin: "re-run with privileges: sudo …", exit 0). `upgrade --rollback` → "restored 1.2.3".
**Pain Points Addressed**: FR-U1 (never overwrite a PM-owned binary); FR-U4 (never auto-sudo); FR-U6
(check-only for CI); FR-U8 (one-step undo); FR-U9 (explicit confirm); FR-U11 (abort-before-swap); FR-U12
(walled off from commits).

## Why

- **PRD §9.29 / §10.5**: v3.0's headline feature is self-update. P1.M1–P1.M3 shipped every primitive
  (version/Client/resolve/detect/delegate/stage/swap/rollback); S1/S2 shipped the CLI shell + prompt
  helpers. The ONLY missing piece is the ORCHESTRATION that ties them together behind the three user
  flags — this item.
- **FR-U12 (walled off)**: the dispatch must produce ONLY 0/1/6 and touch NO commit-core code. The seams
  exist precisely so the dispatch is provable in isolation (P1.M4.T3) without a real network/subprocess.
- **The seam design is the hard part**: Detector.Exec=nil SKIPS probes and osRunner is unexported;
  Swap/Rollback resolve the exe internally. Naïve field-injection can't make those testable from package
  cmd. This item's function-seam design (upgradeDetect/upgradeSwap/upgradeRollback) is what unblocks
  P1.M4.T3 without editing the read-only internal/upgrade package.

## What

Replace runUpgrade's placeholder with the real dispatch + add the seam file. The dispatch branches on the
three flags (--rollback, --check, normal); the normal branch detects then routes to direct-swap or delegate.

### Success Criteria
- [ ] runUpgrade (upgrade.go EDITED) calls `validateUpgradeFlags` + resolves effChannel/effSourceRepo (S1,
      UNCHANGED) then dispatches: `--rollback` → upgradeRollback path; `--check` → runCheck; else detect →
      runDirectSwap (ChannelDirect or --force+warning) / runDelegate. NEVER os.Exit; returns exitcode.New/nil.
- [ ] runCheck (upgrade_run.go): ResolveTarget + CurrentSemver + Compare → dev/up-to-date exit 0; behind
      prints "update available: X → Z; run \"stagecoach upgrade\"" + exit 6.
- [ ] runDirectSwap: MkdirTemp → ResolveTarget → StageNewBinary → confirmUpgrade (FR-U9) → upgradeSwap;
      NeedsPrivilegesError → printPrivilegeCommand + exit 0 (FR-U4/U7). LEAVES tempDir on failure (FR-U11);
      Swap cleans it on success (no defer-cleanup).
- [ ] runDelegate: RUN channels (≠ AUR/Nix) confirmUpgrade first (FR-U9); Delegate; Ran=false →
      printDelegatedUpdate + exit 0; Ran=true ExitCode!=0 → exit 1; ExitCode==0 → exit 0.
- [ ] --rollback: upgradeRollback; ErrNoBackup → "no backup — nothing to roll back" + exit 0; success →
      "restored stagecoach <ver>" + exit 0; ErrBackupUnusable/other → exit 1.
- [ ] ALL seams in package cmd: upgradeBaseURL, upgradeExePath, upgradeExecRunner, upgradeToken,
      upgradeNewClient, upgradeDetect (default prodDetect + cmdRunner), upgradeSwap (default upgrade.Swap),
      upgradeRollback (default upgrade.Rollback). runUpgrade reads every seam (no U1000).
- [ ] Exit codes 0/1/6 ONLY (grep guard: no os.Exit; no 2/3/5/124 produced).
- [ ] `go build ./...` + `go vet ./internal/cmd/...` clean; `gofmt -l` empty; `make test` + `make lint` clean.
- [ ] `git status --porcelain` == upgrade.go (EDIT) + upgrade_run.go (NEW) [+ optional smoke test].

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim LANDED internal/upgrade API (signatures + sentinels + the nil-semantics gotchas), the
S1 placeholder block to replace (verbatim), the S2 helpers to consume (signatures + the 4 confirm outcomes),
the complete dispatch pseudocode for all three paths with exact exit-code mapping, the seam design with the
osRunner-unexported / internal-resolveCurrentExe rationale, the §15.4 exit discipline, the verbose wiring,
and the scope fences.

### Documentation & References

```yaml
# MUST READ — the codebase-specific findings (the API inventory + the seam rationale + the dispatch pseudocode).
- docfile: plan/017_397abce9deb1/P1M4T2S1/research/findings.md
  why: "§0 the S1 placeholder block (verbatim) to replace; §1 the COMPLETE LANDED internal/upgrade API with
        signatures + sentinels + the two nil-semantics gotchas (Detector.Exec nil SKIPS; Delegate Exec nil
        ⇒ prod) + the unexported osRunner / internal resolveCurrentExe walls; §2 the S2 helpers; §3 THE SEAM
        DESIGN (why function seams are required + the cmdRunner dup rationale); §4 the verbatim dispatch
        pseudocode for all 3 paths; §5 exit discipline (incl. the exit-6 main.go print check); §6 verbose;
        §7 scope fence."

# MUST READ — the S1 file I EDIT (the runUpgrade placeholder + the prologue I keep).
- file: internal/cmd/upgrade.go
  why: "runUpgrade (the func I edit): validateUpgradeFlags → config.LoadUpgradeConfig → resolve effChannel/
        effSourceRepo → the TODO(P1.M4.T2.S1)+stderr-notice block I REPLACE. The prologue (validation +
        resolution) STAYS. runUpgrade signature + never-os.Exit contract STAYS. The 9 flag package vars
        (flagCheck/flagTargetVersion/flagPrerelease/flagForce/flagRollback/flagInstallMethod/flagYes/
        flagChannel/flagSourceRepo) are the dispatch inputs. flagVerbose (root.go) gates verbose."
  critical: "KEEP validateUpgradeFlags + the LoadUpgradeConfig + effChannel/effSourceRepo resolution VERBATIM
             (S1 tested them). Replace ONLY the TODO block. Do NOT touch upgradeCmd/init()/the flags."

# MUST READ — the S2 helpers I CONSUME (read-only; do NOT edit).
- file: internal/cmd/upgrade_prompt.go
  why: "confirmUpgrade(current,target,action,assumeYes,in,out)(bool,error) — the 4 outcomes (assumeYes→true;
        non-TTY+no-yes→(false,err) NO 'stagecoach:' prefix; TTY y/Y→true; TTY else→(false,nil)). The
        confirmUpgradeIsTTY seam (S2 owns it). printDelegatedUpdate(out,channel,command) + printPrivilegeCommand
        (out,command) echo the command verbatim. Wire cmd.InOrStdin()/OutOrStdout() as in/out."
  gotcha: "confirmUpgrade returns a PLAIN error (no 'stagecoach:' prefix) — wrap exitcode.New(exitcode.Error,
           err) WITHOUT re-adding 'stagecoach:'. main.go adds the prefix. (See S2's no-double-prefix rule.)"

# MUST READ — the LANDED upgrade API signatures (match these EXACTLY).
- file: internal/upgrade/version.go
  why: "CurrentSemver()(string,bool) — ('vX.Y.Z',true) or ('',false) for dev. Compare(a,b) -1/0/+1 (unparseable⇒0,
        so dev never falsely signals). SetCurrentVersion is the package-UPGRADE test seam (P1.M4.T3 pins
        current); runUpgrade does NOT call it."
- file: internal/upgrade/releases.go
  why: "Client{HTTP,BaseURL,Repo,Token}. HTTP nil⇒DefaultClient; BaseURL ''⇒api.github.com; Repo 'owner/repo';
        Token ''⇒unauth. This is the NETWORK seam — upgradeNewClient builds it with upgradeBaseURL."
- file: internal/upgrade/resolve.go
  why: "ResolveTarget(ctx,c,ResolveOptions{Version,Prerelease,GOOS,GOARCH})(Release,Asset,error). Version>
        Prerelease>LatestStable. Release{Tag,Assets}. Does NOT compare to current (runCheck/runDirectSwap do)."
- file: internal/upgrade/detect.go
  why: "Detector{Exec Runner; ExePath; GOOS; Override; Env; Log}. Detect(ctx)(Channel,string,error).
        CRITICAL: Exec nil⇒SKIP PM probes (NOT prod default) ⇒ runUpgrade's prodDetect MUST pass a real Runner.
        Runner.Run(ctx,name,args)(stdout,exitCode,err): non-zero ExitError⇒(stdout,code,nil); start-fail/ctx⇒err.
        Channel type; ChannelDirect='direct' is the ONLY self-swap channel. ErrUnknownChannel (bad --install-method)."
  critical: "osRunner is UNEXPORTED ⇒ package cmd CANNOT build &osRunner{}. prodDetect uses the package-cmd
             cmdRunner (same contract). This is WHY upgradeDetect is a function seam, not field-injection."
- file: internal/upgrade/delegate.go
  why: "Delegate(ctx,ch,DelegateOptions{Exec,Out,Env,Verbose,Confirmed})(DelegateResult,error). Exec nil⇒osExecRunner
        (prod default — UNLIKE Detector). DelegateResult{Ran,Command,ExitCode}. ChannelDirect⇒(zero,ErrDirectSwap);
        AUR/Nix⇒PRINT(Ran:false); else⇒RUN(Ran:true). ErrDirectSwap sentinel. Confirmed is advisory (Delegate
        never prompts; the command layer gates the call on confirmation)."
- file: internal/upgrade/stage.go
  why: "StageNewBinary(ctx,c,release,asset,tempDir)(newBinPath,err). LEAVES tempDir on failure (FR-U11 inspection)
        AND on success (Swap cleans it). Returns typed sentinels (ErrChecksumMismatch/ErrSanityRunFailed/etc.) —
        propagate via %w; the command layer maps them to exit 1."
- file: internal/upgrade/swap.go
  why: "Swap(ctx,newBinPath)error. Non-writable⇒*NeedsPrivilegesError{Command} (errors.Is ErrNeedsPrivileges;
        .Command='sudo \"<exe>\" upgrade'). On success Swap cleans filepath.Dir(newBinPath)=tempDir. resolveCurrentExe
        is PACKAGE-UPGRADE-level (Swap resolves the exe internally) ⇒ WHY upgradeSwap is a function seam."
- file: internal/upgrade/rollback.go
  why: "Rollback(ctx)(restoredVersion,err). ErrNoBackup⇒'no backup — nothing to roll back'+exit 0. ErrBackupUnusable⇒exit 1.
        resolveCurrentExe internal ⇒ WHY upgradeRollback is a function seam."

# MUST READ — exit codes (the §15.4 upgrade block).
- file: internal/exitcode/exitcode.go
  why: "Success=0, Error=1, UpdateAvailable=6 (+ErrUpdateAvailable sentinel). New(code,err); For(err) maps.
        runUpgrade returns exitcode.New(exitcode.Error,...) for failures, exitcode.New(exitcode.UpdateAvailable,nil)
        for --check-behind, nil for success. NEVER os.Exit. VERIFY ExitError.Error() with nil Err doesn't emit
        an ugly 'stagecoach: ' line on the exit-6 path (the message is already on stdout)."

# MUST READ — the FR-U1–U12 spec (the dispatch contract).
- docfile: plan/017_397abce9deb1/prd_snapshot.md
  section: "§9.29 FR-U1–U12 (lines ~594–625) + §15.4 exit codes"
  why: "FR-U1 (delegate-first; --force warns), FR-U4 (run-vs-print; never auto-sudo; PRINT exits 0),
        FR-U5 (direct swap), FR-U6 (--check exit 0/6), FR-U7 (non-writable→print sudo+exit 0), FR-U8 (rollback;
        no-backup no-op), FR-U9 (confirm y/N for swap + RUN-delegate; --check/PRINT never prompt), FR-U12 (walled
        off; 0/1/6 only)."

# MUST READ — the v3 scope boundary (the negative-space checklist).
- docfile: plan/017_397abce9deb1/architecture/v3_scope_boundary.md
  why: "Confirms: no lock, no repo, no provider, no config.Load bootstrap (S1's no-op PreRunE handles it),
        no auto-elevate, no overwrite of PM-owned without --force+warning. Exit 0/1/6 only. The §19 network
        exception is scoped to upgrade fetching ONLY release artifacts."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  upgrade.go            # EDIT (S1) — replace runUpgrade's TODO placeholder block with the dispatch
  upgrade_run.go        # NEW — runCheck/runDirectSwap/runDelegate + seams + cmdRunner + prod defaults
  upgrade_prompt.go     # READ-ONLY (S2) — confirmUpgrade/printDelegatedUpdate/printPrivilegeCommand/confirmUpgradeIsTTY (CONSUMED)
  upgrade_test.go       # READ-ONLY (S1) — registration/validation tests (the dispatch's coverage is P1.M4.T3)
  root.go               # READ-ONLY — flagVerbose (the verbose gate); ZERO edits
internal/upgrade/       # READ-ONLY (P1.M1–P1.M3 Complete) — version/releases/resolve/detect/delegate/stage/swap/rollback
internal/exitcode/exitcode.go  # READ-ONLY — Success/Error/UpdateAvailable/New/For (CONSUMED)
internal/ui/output.go   # READ-ONLY — IsTerminal (used by S2's seam, not directly by runUpgrade)
cmd/stagecoach/main.go  # READ-ONLY — line ~70 prepends "stagecoach: " (the no-double-prefix rule); maps via For()
go.mod                  # READ-ONLY — UNCHANGED (net/http/os/exec already stdlib; cobra/pflag present)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/cmd/upgrade.go       # EDIT — runUpgrade: replace TODO block with `return dispatchUpgrade(...)` (or inline); keep prologue
internal/cmd/upgrade_run.go   # NEW — dispatchUpgrade + runCheck/runDirectSwap/runDelegate + displayCurrent/verboseLog +
                              #       the 8 seam package vars + prod defaults (prodDetect/prodNewClient/prodExePath/cmdRunner)
# NOTHING ELSE. No edit to internal/upgrade/* (read-only — the seams make it testable WITHOUT editing them),
# upgrade_prompt.go (S2), root.go, config.Load, exitcode.go, docs/cli.md (S1 owns Mode-A), the commit path (FR-U12),
# go.mod, any PRD/task file.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (Detector.Exec nil SKIPS PM probes — NOT a prod default): detect.go's Detector{Exec Runner}
// treats nil as "skip tier (b)" (always falls through to path heuristics / direct). Production MUST pass a
// real Runner. But upgrade.osRunner is UNEXPORTED ⇒ package cmd can't build it. So prodDetect (the
// upgradeDetect default) builds the Detector with a PACKAGE-CMD cmdRunner implementing upgrade.Runner
// (exec.CommandContext; SAME contract as osRunner: non-zero ExitError⇒(stdout,code,nil), start-fail/ctx⇒err).
// This is WHY upgradeDetect is a function seam (P1.M4.T3 overrides it to return a canned channel — no subprocess).

// CRITICAL (Swap/Rollback resolve the exe INTERNALLY via package-upgrade resolveCurrentExe): package cmd
// cannot steer the swap/rollback exe by field-injection. So upgradeSwap/upgradeRollback are FUNCTION seams
// (default = upgrade.Swap/upgrade.Rollback). P1.M4.T3 overrides them (or overrides resolveCurrentExe in a
// package-upgrade test) to point at a temp-dir exe. upgradeExePath (the os.Executable seam) feeds ONLY
// Detector.ExePath (detection), NOT the swap.

// CRITICAL (Delegate Exec nil IS the prod default — opposite of Detector): DelegateOptions.Exec nil ⇒
// upgrade.Delegate uses its osExecRunner. So runDelegate passes upgradeExecRunner (default nil) directly.
// NO cmdRunner dup for Delegate — only for the Detector (the one place nil means skip).

// CRITICAL (runUpgrade NEVER os.Exit): return exitcode.New(exitcode.Error, err) / exitcode.New(
// exitcode.UpdateAvailable, nil) / nil. main maps via exitcode.For. (Mirrors every RunE.)

// CRITICAL (confirmUpgrade returns a PLAIN error — no "stagecoach:" prefix): wrap exitcode.New(exitcode.Error,
// err) WITHOUT re-adding "stagecoach:". main.go:70 adds the prefix; double-prefixing is the known bug pattern.
// (S2's rule — apply at the runUpgrade call site.)

// CRITICAL (ErrDirectSwap is UNREACHABLE in runDelegate): runUpgrade routes ChannelDirect (and --force) to
// runDirectSwap BEFORE runDelegate, so Delegate never sees ChannelDirect. Do NOT add a defensive
// errors.Is(err, ErrDirectSwap) branch in runDelegate (dead code). The ch==Direct||flagForce branch prevents it.

// CRITICAL (exit-6 print check): exitcode.New(exitcode.UpdateAvailable, nil) ⇒ For() returns 6. VERIFY main.go's
// Fprintf(os.Stderr, "stagecoach: %v\n", err) doesn't emit an ugly empty line (ExitError.Error() with nil Err).
// The "update available: …" message is already on STDOUT; if main double-prints, that's acceptable/minor, OR
// return a sentinel-free forced code. Check exitcode.ExitError.Error()'s nil-Err branch + main.go's print gate.

// CRITICAL (tempDir cleanup — do NOT defer): Swap cleans filepath.Dir(newBinPath)=tempDir on SUCCESS (swap.go
// step 4). On failure (Resolve/Stage/Swap-non-privilege/NeedsPrivileges) LEAVE tempDir (FR-U11 inspection).
// A `defer os.RemoveAll(tempDir)` would nuke the inspection artifact on failure — do NOT add it. The
// NeedsPrivileges case ALSO leaves tempDir (the proactive gate is step 2, before Swap's cleanup).

// GOTCHA (--force on a non-direct channel warns, FR-U1): if flagForce && ch != ChannelDirect, print a stderr
// WARNING ("--force overriding a detected <ch> install; self-swapping") THEN runDirectSwap. The branch
// `if ch == ChannelDirect || flagForce` handles it; the warning is the `flagForce && ch != direct` sub-case.

// GOTCHA (RUN-delegate confirm needs no pre-resolve): FR-U9 requires confirm for RUN-delegated channels, but
// the target version for a PM update is "latest" (the PM fetches it). Do NOT ResolveTarget for the delegate
// confirm (avoid an extra network call); use target="latest", action="Update via <channel>'s updater.". The
// exact updater COMMAND isn't known pre-Delegate (DelegateResult.Command is returned after), so the action is
// generic. AUR/Nix (PRINT) never confirm.

// GOTCHA (RUN-delegate exit code maps to 0/1 only): §15.4 upgrade block is 0/1/6. A RUN updater's non-zero
// exit (e.g. brew exit 1) maps to stagecoach exit 1 (Error) — NOT the raw code. Zero→0. (We do NOT propagate
// brew's exit 42 as stagecoach 42.)

// GOTCHA (flagVerbose is readable despite the no-op PersistentPreRunE): the --verbose/-v flag is set by pflag
// on the FlagSet regardless of whether config.Load ran. runUpgrade reads the package var flagVerbose (root.go)
// to gate verboseLog. Do NOT rely on cfg.Verbose (config.Load is skipped).

// GOTCHA (CurrentSemver returns ("",false) for dev; no raw getter): for the --check dev display + the
// confirm "current" string, use displayCurrent() = CurrentSemver's value when ok, else "dev" (the only !ok
// case). Do NOT add a raw-version getter to package upgrade (out of scope; "dev" suffices).
```

## Implementation Blueprint

### Data models and structure

None beyond the seam package vars (funcs/values) + `cmdRunner` (a struct implementing `upgrade.Runner`).
No new config fields, no new exit codes (6 is LANDED), no new types in package upgrade, no new deps
(net/http, os, os/exec, runtime, context, errors, fmt, io — all stdlib; cobra/pflag/upgrade/exitcode/config
already imported by the package).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/cmd/upgrade_run.go — the seam package vars + production defaults + cmdRunner
  - PACKAGE DOC: this file is the runUpgrade DISPATCH + its injectable test seams (FR-U1–U12; the FR-U12
    test wall for P1.M4.T3). Explain WHY function seams are required (Detector.Exec nil skips + osRunner
    unexported; Swap/Rollback internal resolveCurrentExe) and that ALL seams live in package cmd (ZERO edits
    to internal/upgrade/*). File comment only.
  - IMPORTS: context, errors, fmt, io, os, os/exec, runtime, github.com/dabstractor/stagecoach/internal/upgrade,
    github.com/dabstractor/stagecoach/internal/exitcode. (NO cobra/pflag here — the dispatch takes primitives;
    runUpgrade in upgrade.go passes cmd.Context()/OutOrStdout()/ErrOrStderr()/InOrStdin(). NO internal/ui —
    verboseLog is a plain stderr-gated closure; NO internal/config — S1's runUpgrade prologue already resolved
    effChannel/effSourceRepo and passes them in.)
  - THE SEAMS (package vars; every one read by the dispatch ⇒ no U1000):
      var (
          // upgradeBaseURL overrides the GitHub API root for the releases Client. "" ⇒ api.github.com (prod);
          // P1.M4.T3 sets it to an httptest.Server.URL (FR-U12 test wall — no real network).
          upgradeBaseURL string
          // upgradeExePath resolves the running binary for install-method DETECTION (Detector.ExePath). Default:
          // os.Executable + EvalSymlinks (detect.go:352-354 idiom). (The SWAP resolves its own exe internally.)
          upgradeExePath = prodExePath
          // upgradeExecRunner is the streaming subprocess seam for RUN-delegated updaters (DelegateOptions.Exec).
          // nil ⇒ upgrade.Delegate's osExecRunner prod default. P1.M4.T3 injects a fake (asserts argv + exit).
          upgradeExecRunner upgrade.ExecRunner
          // upgradeToken resolves the optional GitHub auth token. Default: STAGECOACH_GITHUB_TOKEN else GITHUB_TOKEN.
          upgradeToken = prodToken
          // upgradeNewClient builds the releases Client from repo/token + the upgradeBaseURL seam.
          upgradeNewClient = prodNewClient
          // FUNCTION seams (field-injection can't reach these — see file doc):
          upgradeDetect   = prodDetect   // (ctx, override, log) (Channel, string, error) — production Detector
          upgradeSwap     = upgrade.Swap // (ctx, newBinPath) error — Swap resolves exe internally
          upgradeRollback = upgrade.Rollback // (ctx) (string, error) — Rollback resolves exe internally
      )
  - prodExePath (clone detect.go:352-354 + swap.go's resolveCurrentExe idiom):
      func prodExePath() (string, error) {
          exe, err := os.Executable()
          if err != nil { return "", fmt.Errorf("os.Executable: %w", err) }
          if real, err := filepath.EvalSymlinks(exe); err == nil { return real, nil }
          return exe, nil // tolerate EvalSymlinks failure
      }
      (import path/filepath.)
  - prodToken:
      func prodToken() string {
          if t := os.Getenv("STAGECOACH_GITHUB_TOKEN"); t != "" { return t }
          return os.Getenv("GITHUB_TOKEN")
      }
  - prodNewClient:
      func prodNewClient(repo, token string) *upgrade.Client {
          return &upgrade.Client{BaseURL: upgradeBaseURL, Repo: repo, Token: token} // HTTP nil ⇒ DefaultClient
      }
  - cmdRunner (the package-cmd production upgrade.Runner — SAME contract as upgrade.osRunner):
      // cmdRunner implements upgrade.Runner for the production tier-(b) PM DB queries. upgrade.osRunner is
      // UNEXPORTED, so package cmd provides its own (identical contract: a NON-ZERO process exit — e.g. `brew
      // list stagecoach` exit 1 = "not installed" — ⇒ (stdout, code, nil) so the cascade treats "not installed"
      // as a SKIP, not an error; only a start failure / ctx deadline ⇒ err). Mirrors upgrade.osRunner exactly.
      type cmdRunner struct{}
      func (cmdRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
          var buf bytes.Buffer
          cmd := exec.CommandContext(ctx, name, args...)
          cmd.Stdout = &buf
          if err := cmd.Run(); err != nil {
              if cerr := ctx.Err(); cerr != nil { return "", 0, cerr } // ctx deadline ⇒ "PM hung" err
              var ee *exec.ExitError
              if errors.As(err, &ee) { return buf.String(), ee.ExitCode(), nil } // non-zero exit ⇒ skip, NOT err
              return "", 0, err // start failure (binary absent) ⇒ err
          }
          return buf.String(), 0, nil
      }
      (import bytes.)
  - prodDetect (builds the production Detector + Detect):
      func prodDetect(ctx context.Context, override string, log func(string)) (upgrade.Channel, string, error) {
          exe, err := upgradeExePath()
          if err != nil { return "", "", fmt.Errorf("stagecoach: %w", err) }
          d := &upgrade.Detector{
              Exec:     cmdRunner{},  // NON-NIL (nil skips PM probes — detect.go). osRunner is unexported.
              ExePath:  exe,
              GOOS:     runtime.GOOS,
              Override: override,
              Env:      os.Getenv,
              Log:      log,
          }
          return d.Detect(ctx)
      }
  - NAMING: upgradeBaseURL, upgradeExePath, upgradeExecRunner, upgradeToken, upgradeNewClient, upgradeDetect,
    upgradeSwap, upgradeRollback, prodExePath, prodToken, prodNewClient, prodDetect, cmdRunner (all unexported;
    same-package, consumed by runUpgrade in upgrade.go + overridden by P1.M4.T3 in package cmd tests).

Task 2: EDIT internal/cmd/upgrade.go — replace runUpgrade's TODO block with the dispatch
  - KEEP S1's prologue VERBATIM (validateUpgradeFlags → LoadUpgradeConfig → effChannel/effSourceRepo). REPLACE
    ONLY the `// TODO(P1.M4.T2.S1): ...` + the `fmt.Fprintf(cmd.ErrOrStderr(), "stagecoach upgrade: not yet
    implemented ...")` + `return nil` block with the dispatch. Add `ctx := cmd.Context()` after the prologue.
  - THE DISPATCH (inline in runUpgrade, or `return dispatchUpgrade(ctx, cmd, effChannel, effSourceRepo)` into
    upgrade_run.go — implementer's choice; a thin dispatchUpgrade helper keeps runUpgrade readable):
      ctx := cmd.Context()
      log := verboseLog(cmd.ErrOrStderr())
      // --rollback path (FR-U8)
      if flagRollback {
          ver, err := upgradeRollback(ctx)
          if errors.Is(err, upgrade.ErrNoBackup) {
              fmt.Fprintln(cmd.OutOrStdout(), "no backup — nothing to roll back")
              return nil // exit 0
          }
          if err != nil {
              return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: rollback: %w", err)) // exit 1
          }
          fmt.Fprintf(cmd.OutOrStdout(), "restored stagecoach %s\n", ver)
          return nil // exit 0
      }
      client := upgradeNewClient(effSourceRepo, upgradeToken())
      // --check path (FR-U6)
      if flagCheck {
          return runCheck(ctx, cmd, client, effChannel)
      }
      // normal path (FR-U1): detect → direct-swap | delegate
      ch, evidence, err := upgradeDetect(ctx, flagInstallMethod, log)
      if err != nil {
          return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err)) // ErrUnknownChannel → exit 1
      }
      log("detected install method: " + string(ch) + " (" + evidence + ")")
      if ch == upgrade.ChannelDirect || flagForce {
          if flagForce && ch != upgrade.ChannelDirect {
              fmt.Fprintln(cmd.ErrOrStderr(), "stagecoach: warning: --force overriding a detected "+string(ch)+" install (FR-U1); self-swapping")
          }
          return runDirectSwap(ctx, cmd, client, effChannel)
      }
      return runDelegate(ctx, cmd, ch)
  - IMPORTS added to upgrade.go: context, errors (if not present); upgrade (if dispatch helpers live in
    upgrade_run.go, upgrade.go may not need upgrade — but the inline `upgrade.ChannelDirect`/`upgrade.ErrNoBackup`
    refs do). Reconcile imports (gofmt/gosimple). Do NOT remove S1's imports (config/exitcode/cobra/pflag/fmt).

Task 3: CREATE the dispatch helpers in upgrade_run.go — runCheck / runDirectSwap / runDelegate + display/verbose
  - runCheck(ctx, cmd, client, effChannel) error  (FR-U6):
      cur, ok := upgrade.CurrentSemver()
      release, _, err := upgrade.ResolveTarget(ctx, client, upgrade.ResolveOptions{
          Version:    flagTargetVersion,
          Prerelease: effChannel == "prerelease",
      })
      if err != nil { return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: check: %w", err)) }
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
  - runDirectSwap(ctx, cmd, client, effChannel) error  (FR-U5/U7/U9/U11):
      tempDir, err := os.MkdirTemp("", "stagecoach-upgrade-*")
      if err != nil { return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err)) }
      // NO defer os.RemoveAll(tempDir) — failure LEAVES it (FR-U11 inspection); Swap cleans it on success.
      release, asset, err := upgrade.ResolveTarget(ctx, client, upgrade.ResolveOptions{
          Version: flagTargetVersion, Prerelease: effChannel == "prerelease",
      })
      if err != nil { return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err)) } // LEAVE tempDir
      cmd.ErrOrStderr()/*log*/ // (verboseLog already captured; optionally log("target: "+release.Tag))
      newBin, err := upgrade.StageNewBinary(ctx, client, release, asset, tempDir)
      if err != nil { return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err)) } // LEAVE tempDir
      out := cmd.OutOrStdout()
      ok, cerr := confirmUpgrade(displayCurrent(), release.Tag, "Self-swap the direct-binary install.", flagYes, cmd.InOrStdin(), out)
      if cerr != nil { return exitcode.New(exitcode.Error, cerr) } // non-TTY refusal → exit 1 (NO re-prefix)
      if !ok { return nil } // TTY decline → exit 0
      if serr := upgradeSwap(ctx, newBin); serr != nil {
          var npe *upgrade.NeedsPrivilegesError
          if errors.As(serr, &npe) {
              printPrivilegeCommand(out, npe.Command) // echo .Command verbatim
              return nil // exit 0 (FR-U4/U7 — never auto-elevate)
          }
          return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", serr)) // exit 1
      }
      fmt.Fprintf(out, "stagecoach upgraded to %s\n", release.Tag)
      return nil // exit 0 (Swap already cleaned tempDir)
  - runDelegate(ctx, cmd, ch) error  (FR-U3/U4/U9):
      out := cmd.OutOrStdout()
      isRun := ch != upgrade.ChannelAUR && ch != upgrade.ChannelNix // FR-U9: confirm only RUN channels
      if isRun {
          ok, cerr := confirmUpgrade(displayCurrent(), "latest", "Update via "+string(ch)+"'s updater.", flagYes, cmd.InOrStdin(), out)
          if cerr != nil { return exitcode.New(exitcode.Error, cerr) }
          if !ok { return nil }
      }
      res, err := upgrade.Delegate(ctx, ch, upgrade.DelegateOptions{
          Exec: upgradeExecRunner, // nil ⇒ osExecRunner prod default
          Out: out, Env: os.Getenv, Verbose: verboseLog(cmd.ErrOrStderr()), Confirmed: flagYes,
      })
      if err != nil { return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %w", err)) } // start failure → exit 1
      if !res.Ran { // PRINT channel (AUR/Nix) — Delegate already printed; frame it
          printDelegatedUpdate(out, string(ch), res.Command)
          return nil // exit 0 (FR-U4)
      }
      if res.ExitCode != 0 {
          return exitcode.New(exitcode.Error, fmt.Errorf("stagecoach: %s updater exited %d", ch, res.ExitCode)) // exit 1
      }
      fmt.Fprintf(out, "stagecoach updated via %s\n", ch)
      return nil // exit 0
  - displayCurrent() / verboseLog(w):
      func displayCurrent() string { cur, ok := upgrade.CurrentSemver(); if ok { return cur }; return "dev" }
      func verboseLog(w io.Writer) func(string) {
          return func(msg string) { if flagVerbose { fmt.Fprintln(w, "stagecoach: "+msg) } }
      }
  - NAMING: runCheck, runDirectSwap, runDelegate, displayCurrent, verboseLog (unexported). The dispatch helpers
    take primitive args (ctx, *cobra.Command, *upgrade.Client, string) — they read the flagX package vars directly.

Task 4: VERIFY — build, vet, format, tests, lint, grep guards
  - go build ./... ; go vet ./internal/cmd/... ; gofmt -l internal/cmd/upgrade.go internal/cmd/upgrade_run.go  # empty
  - go test ./internal/cmd/ -race   # S1's tests stay green (the prologue is UNCHANGED); no sibling broke
  - make test && make lint          # staticcheck/unused: every seam var is read by the dispatch; cmdRunner by prodDetect
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the seam var — package cmd, default = prod, overridable by P1.M4.T3):
var upgradeBaseURL string                       // Client/BaseURL; "" ⇒ api.github.com; tests ⇒ httptest URL
var upgradeSwap = upgrade.Swap                  // function seam (Swap resolves exe internally)
var upgradeDetect = prodDetect                  // function seam (osRunner unexported; prodDetect uses cmdRunner)

// PATTERN (prodDetect — the production Detector with a package-cmd Runner):
func prodDetect(ctx context.Context, override string, log func(string)) (upgrade.Channel, string, error) {
	exe, err := upgradeExePath()
	if err != nil { return "", "", fmt.Errorf("stagecoach: %w", err) }
	return (&upgrade.Detector{Exec: cmdRunner{}, ExePath: exe, GOOS: runtime.GOOS, Override: override, Env: os.Getenv, Log: log}).Detect(ctx)
}

// PATTERN (cmdRunner — package-cmd upgrade.Runner; SAME contract as upgrade.osRunner):
func (cmdRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
	var buf bytes.Buffer
	c := exec.CommandContext(ctx, name, args...); c.Stdout = &buf
	if err := c.Run(); err != nil {
		if cerr := ctx.Err(); cerr != nil { return "", 0, cerr }
		var ee *exec.ExitError
		if errors.As(err, &ee) { return buf.String(), ee.ExitCode(), nil } // non-zero ⇒ skip, not err
		return "", 0, err
	}
	return buf.String(), 0, nil
}

// PATTERN (exit discipline — NEVER os.Exit; 0/1/6 only):
return nil                                  // success / decline / printed / no-backup / up-to-date / dev
return exitcode.New(exitcode.Error, err)    // general failure (exit 1)
return exitcode.New(exitcode.UpdateAvailable, nil) // --check behind (exit 6)

// PATTERN (confirm at the runUpgrade call site — wrap the PLAIN error WITHOUT re-prefixing):
ok, err := confirmUpgrade(displayCurrent(), release.Tag, "Self-swap the direct-binary install.", flagYes, cmd.InOrStdin(), out)
if err != nil { return exitcode.New(exitcode.Error, err) } // non-TTY refusal → exit 1 (NO "stagecoach:" re-add)
if !ok { return nil }                                      // TTY decline → exit 0
```

### Integration Points

```yaml
CLI DISPATCH (the new runUpgrade body):
  - --rollback → upgradeRollback; ErrNoBackup ⇒ "no backup …" + exit 0.
  - --check → runCheck: ResolveTarget + CurrentSemver + Compare; dev/up-to-date exit 0; behind exit 6.
  - normal → upgradeDetect; ChannelDirect (or --force+warning) → runDirectSwap; else runDelegate.
CONSUMES (LANDED — read-only):
  - upgrade: CurrentSemver/Compare, Client, ResolveTarget/ResolveOptions, Detector/Channel/Runner,
    Delegate/DelegateOptions/DelegateResult/ErrDirectSwap, StageNewBinary, Swap/NeedsPrivilegesError,
    Rollback/ErrNoBackup/ErrBackupUnusable, ChannelDirect/ChannelAUR/ChannelNix.
  - exitcode: New/For/Success/Error/UpdateAvailable.
  - cmd (S1/S2): flagX vars, confirmUpgrade/printDelegatedUpdate/printPrivilegeCommand (S2), flagVerbose (root.go).
SEAMS (package cmd, for P1.M4.T3): upgradeBaseURL, upgradeExePath, upgradeExecRunner, upgradeToken,
  upgradeNewClient, upgradeDetect, upgradeSwap, upgradeRollback.
NO database / migration / routes / config-struct change / exitcode-const change (6 is LANDED) / go.mod change /
docs change (S1 owns Mode-A; P3 owns changeset). NO edit to internal/upgrade/*.
SCOPE FENCES:
  - Touches ONLY: internal/cmd/upgrade.go (EDIT the placeholder) + internal/cmd/upgrade_run.go (NEW) [+ optional smoke test].
  - Does NOT edit: internal/upgrade/* (read-only — the seams make it testable WITHOUT editing them),
    upgrade_prompt.go (S2), upgrade_test.go (S1), root.go, config.Load, exitcode.go, docs/cli.md (S1),
    the commit path (FR-U12), go.mod, any PRD/task file.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (the dispatch links runUpgrade to internal/upgrade; net/http/os/exec/runtime are stdlib).
go build ./...
# Expected: clean. Failures to watch: "upgradeBaseURL declared and not used" (it's read by prodNewClient —
#           confirm); "flagVerbose undefined" (it's in root.go, same package — confirm); an unused import
#           after reconciling upgrade.go's imports.

# Vet the changed package.
go vet ./internal/cmd/...
# Expected: clean.

# Format.
gofmt -l internal/cmd/upgrade.go internal/cmd/upgrade_run.go
# Expected: empty. If listed: gofmt -w them.

# Lint (staticcheck/unused/errcheck/gosimple/govet/ineffassign). Every seam var MUST be read by the dispatch
# (upgradeBaseURL by prodNewClient; upgradeExePath by prodDetect; upgradeExecRunner by runDelegate;
# upgradeToken/upgradeNewClient/upgradeDetect/upgradeSwap/upgradeRollback by runUpgrade) and cmdRunner by
# prodDetect — so no U1000. The flagX vars are S1's (already used).
make lint
# Expected: zero errors. If U1000 fires on a seam ⇒ the dispatch isn't reading it — wire it in.

# Scope guard: ONLY upgrade.go (EDIT) + upgrade_run.go (NEW) [+ optional smoke test].
git status --porcelain
# Expected: internal/cmd/upgrade.go + internal/cmd/upgrade_run.go ONLY (no internal/upgrade/*, no S2 file,
#           no root.go, no docs/cli.md, no go.mod).
```

### Level 2: Unit Tests (Component Validation)

```bash
# S1's registration/validation tests STAY GREEN (the prologue is UNCHANGED; only the placeholder block changed).
go test ./internal/cmd/ -run 'TestUpgradeCommand' -v
# Expected: ALL PASS (S1's 5 tests unaffected — they exercise the shell/flags/validation, not the dispatch).

# Full cmd-package regression (the new dispatch + seams don't disturb root's flag set / sibling commands).
go test ./internal/cmd/ -race
# Expected: green. (The seam vars are package-level; if a test overrides one it MUST restore via t.Cleanup so
#           it doesn't leak into a sibling — but THIS item adds no overriding test; P1.M4.T3 does, with restores.)

# Full race suite + lint + build.
make test && make lint && make build
# Expected: all green.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary (proves upgrade.go + upgrade_run.go link into the real CLI).
make build

# Manual smoke (the dispatch runs end-to-end against the REAL GitHub API — network required; informational):
TMP="$(mktemp -d)"; cd "$TMP"
../../bin/stagecoach upgrade --check; echo "exit=$?"
# Expected (current build is "dev"): "stagecoach dev (latest: v…; development build — cannot compare)" exit 0.
../../bin/stagecoach upgrade --rollback; echo "exit=$?"
# Expected: "no backup — nothing to roll back" exit 0.
# (The full dispatch — delegate/direct-swap/exit-6-behind — is exercised by P1.M4.T3 against httptest, NOT here.)

# Manual: flag validation still works (S1).
../../bin/stagecoach upgrade --version 1.0 --prerelease; echo "exit=$?"   # "mutually exclusive", exit 1
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: the placeholder is GONE (replaced by the dispatch).
grep -n 'TODO(P1.M4.T2.S1)' internal/cmd/upgrade.go   # 0 hits
grep -n 'not yet implemented' internal/cmd/upgrade.go  # 0 hits

# Guard 2: runUpgrade NEVER os.Exit (returns exitcode.New/nil).
grep -n 'os.Exit' internal/cmd/upgrade.go internal/cmd/upgrade_run.go | grep -v '_test.go'   # 0 hits

# Guard 3: the 3 dispatch paths exist (runCheck/runDirectSwap/runDelegate) + the --rollback inline branch.
grep -n 'func runCheck\|func runDirectSwap\|func runDelegate' internal/cmd/upgrade_run.go   # 3 hits
grep -n 'upgradeRollback(ctx)\|ErrNoBackup' internal/cmd/upgrade.go internal/cmd/upgrade_run.go   # ≥1 (the --rollback branch)

# Guard 4: exit 6 is produced ONLY by runCheck (--check behind).
grep -n 'exitcode.UpdateAvailable' internal/cmd/upgrade_run.go   # 1 hit (in runCheck)

# Guard 5: the seams exist as package vars + are read by the dispatch.
for s in upgradeBaseURL upgradeExePath upgradeExecRunner upgradeToken upgradeNewClient upgradeDetect upgradeSwap upgradeRollback; do
  grep -q "var $s\b\|^\s*$s \+=\|$s = " internal/cmd/upgrade_run.go || echo "MISSING seam $s"
done
grep -n 'upgradeDetect(\|upgradeSwap(\|upgradeRollback(\|upgradeNewClient(\|upgradeToken()\|upgradeExecRunner' internal/cmd/upgrade.go internal/cmd/upgrade_run.go | grep -v 'var '   # the reads

# Guard 6: cmdRunner implements upgrade.Runner (the production Detector Exec; osRunner is unexported).
grep -n 'type cmdRunner struct\|func (cmdRunner) Run' internal/cmd/upgrade_run.go   # 2 hits
grep -n 'Exec: *cmdRunner{}' internal/cmd/upgrade_run.go   # 1 hit (in prodDetect)

# Guard 7: confirmUpgrade is wired (NOT re-prefixed) + the print helpers are used.
grep -n 'confirmUpgrade(' internal/cmd/upgrade_run.go | grep -v 'stagecoach:'   # the calls (no re-prefix)
grep -n 'printPrivilegeCommand(\|printDelegatedUpdate(' internal/cmd/upgrade_run.go   # 2 hits

# Guard 8: ErrDirectSwap is NOT defensively handled in runDelegate (unreachable — the branch prevents it).
grep -n 'ErrDirectSwap' internal/cmd/upgrade_run.go | grep -i delegate && echo "WARN: dead ErrDirectSwap branch in runDelegate" || echo "OK: no dead ErrDirectSwap branch"

# Guard 9: NO defer os.RemoveAll(tempDir) (FR-U11 — failure LEAVES tempDir; Swap cleans on success).
grep -n 'defer os.RemoveAll' internal/cmd/upgrade_run.go | grep -i temp && echo "WARN: tempDir defer-cleanup nukes the FR-U11 inspection artifact" || echo "OK: no tempDir defer-cleanup"

# Guard 10: scope — ONLY upgrade.go (EDIT) + upgrade_run.go (NEW).
git status --porcelain
git diff --name-only | grep -vE '^internal/cmd/(upgrade|upgrade_run)\.go$' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
git diff --name-only | grep -qE '^internal/upgrade/' && echo "FAIL: edited read-only internal/upgrade/*" || echo "OK: internal/upgrade untouched"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean; `go vet ./internal/cmd/...` clean; `gofmt -l` empty on the 2 files
- [ ] `make lint` zero errors (every seam var read by the dispatch; cmdRunner read by prodDetect — no U1000)
- [ ] `go test ./internal/cmd/ -race` green (S1's tests unaffected; no sibling broke)
- [ ] `make test` + `make build` green

### Feature Validation (the 3 paths + seams)
- [ ] --check: dev/up-to-date exit 0; behind exit 6 (grep guard 4) (the message format matches FR-U6)
- [ ] --rollback: ErrNoBackup → "no backup …" exit 0; success → "restored …" exit 0; ErrBackupUnusable → exit 1
- [ ] normal direct-swap: ResolveTarget→Stage→confirm→Swap; NeedsPrivileges → printPrivilegeCommand + exit 0 (grep guard 7,9)
- [ ] normal delegate: RUN channels confirm first (FR-U9); PRINT → printDelegatedUpdate + exit 0; RUN non-zero → exit 1
- [ ] --force on a non-direct channel warns (FR-U1) then self-swaps
- [ ] runUpgrade never os.Exit (grep guard 2); exit codes 0/1/6 only

### Seam Validation (the P1.M4.T3 test wall)
- [ ] all 8 seam vars present (grep guard 5): upgradeBaseURL, upgradeExePath, upgradeExecRunner, upgradeToken,
      upgradeNewClient, upgradeDetect, upgradeSwap, upgradeRollback
- [ ] cmdRunner implements upgrade.Runner + is wired into prodDetect (grep guard 6)
- [ ] NO edit to internal/upgrade/* (the seams make it testable without editing it) (grep guard 10)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `internal/cmd/upgrade.go` (EDIT) + `internal/cmd/upgrade_run.go` (NEW) [+ optional smoke test]
- [ ] NO edit to internal/upgrade/* (read-only), upgrade_prompt.go (S2), upgrade_test.go (S1), root.go,
      config.Load, exitcode.go, docs/cli.md (S1 owns Mode-A), the commit path (FR-U12), go.mod, any PRD/task file

### Code Quality & Docs
- [ ] runUpgrade keeps S1's prologue (validation + resolution) verbatim; only the placeholder block changed
- [ ] The seam rationale (osRunner unexported; internal resolveCurrentExe) is in the upgrade_run.go file doc
- [ ] Exit-6 main.go print behavior verified (no ugly empty "stagecoach: " line on the --check-behind path)
- [ ] Verbose (FR50) logs detect steps, channel+evidence, target, planned action (gated on flagVerbose)

---

## Anti-Patterns to Avoid

- ❌ Don't pass `nil` for `Detector.Exec` in prodDetect. detect.go treats nil as SKIP (not prod default) —
  detection would always fall through to path/direct. Pass the package-cmd `cmdRunner{}` (osRunner is
  unexported; cmdRunner is its same-contract twin). This is WHY upgradeDetect is a function seam.
- ❌ Don't add `defer os.RemoveAll(tempDir)` in runDirectSwap. On failure (Resolve/Stage/Swap-non-privilege/
  NeedsPrivileges) the tempDir is the FR-U11 inspection artifact — leave it. Swap cleans it on success
  (swap.go step 4). A defer would nuke the inspection artifact.
- ❌ Don't defensively handle `ErrDirectSwap` in runDelegate. runUpgrade routes ChannelDirect (and --force) to
  runDirectSwap BEFORE runDelegate, so Delegate never sees ChannelDirect. A `errors.Is(err, ErrDirectSwap)`
  branch there is dead code.
- ❌ Don't re-prefix confirmUpgrade's error with "stagecoach:". It returns a PLAIN error (S2's rule); wrap
  `exitcode.New(exitcode.Error, err)` WITHOUT re-adding "stagecoach:". main.go:70 adds the prefix; double-
  prefixing is the known bug.
- ❌ Don't propagate a RUN-delegated updater's raw exit code as stagecoach's. §15.4 upgrade block is 0/1/6:
  a non-zero updater exit (e.g. brew exit 1) → stagecoach exit 1 (Error), NOT exit 1's raw value propagated.
- ❌ Don't ResolveTarget for the delegated-RUN confirm. The target for a PM update is "latest" (the PM fetches
  it); an extra network call is needless. Use target="latest", action="Update via <channel>'s updater.".
  (AUR/Nix PRINT channels never confirm.)
- ❌ Don't edit internal/upgrade/*. The seams (function + field) make the dispatch fully testable from package
  cmd WITHOUT touching the read-only P1.M1–P1.M3 package. If a seam seems to require an upgrade edit, the
  design is wrong — re-scope (e.g. add a cmd-level function seam instead).
- ❌ Don't `os.Exit` anywhere in runUpgrade/upgrade_run.go. Return `exitcode.New(...)`/`nil`; main maps via
  `exitcode.For`. (Mirrors every RunE.)
- ❌ Don't rely on `cfg.Verbose` for the verbose gate. config.Load is SKIPPED (the no-op PersistentPreRunE).
  Read the `flagVerbose` package var (root.go) — pflag sets it regardless of config.Load.
- ❌ Don't touch docs/cli.md. S1 owns the Mode-A `### upgrade` subsection + exit-6 row. The changeset-level
  docs are P3 (P3.M1.T1.S1). This item is code + seams only.
- ❌ Don't add a raw-version getter to package upgrade for the dev display. CurrentSemver returns ("",false)
  for dev; displayCurrent() returns "dev" for the !ok case (the only one in practice). Adding a getter is
  out of scope (edits read-only package upgrade).
- ❌ Don't forget to VERIFY the exit-6 print path. `exitcode.New(exitcode.UpdateAvailable, nil)` ⇒ For()→6,
  but confirm ExitError.Error() with nil Err + main.go's Fprintf don't emit an ugly empty "stagecoach: "
  line. The "update available: …" message is already on stdout. (Inspect exitcode.go's Error() else-branch +
  main.go's print gate; adjust only if it double-prints.)