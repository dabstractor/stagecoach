// Package cmd: upgrade_run.go holds the runUpgrade DISPATCH (PRD §9.29 FR-U1–U12, §15.4) and its
// INJECTABLE TEST SEAMS — the FR-U12 test wall for P1.M4.T3. It wires the LANDED internal/upgrade
// primitives (P1.M1–P1.M3 = Complete: CurrentSemver/Compare, Client, ResolveTarget, Detector.Detect,
// Delegate/DelegateResult, StageNewBinary, Swap/NeedsPrivilegesError, Rollback/ErrNoBackup) behind the
// three user paths exposed by S1's command shell:
//   - --rollback (FR-U8)  → upgradeRollback; ErrNoBackup ⇒ "no backup — nothing to roll back" + exit 0.
//   - --check (FR-U6)     → runCheck: ResolveTarget + CurrentSemver + Compare; dev/up-to-date exit 0;
//     behind ⇒ "update available: X → Z; run \"stagecoach upgrade\"" + exit 6.
//   - normal (FR-U1/U4/U5)→ upgradeDetect; ChannelDirect (or --force + warning, FR-U1) ⇒ runDirectSwap
//     (ResolveTarget→StageNewBinary→confirmUpgrade→upgradeSwap;
//     NeedsPrivileges ⇒ printPrivilegeCommand + exit 0, FR-U4/U7); else ⇒
//     runDelegate (RUN channels confirm first per FR-U9; PRINT channels AUR/Nix
//     ⇒ printDelegatedUpdate + exit 0; RUN exit code mapped 0→0 / non-zero→1).
//
// Exit codes are 0/1/6 ONLY (§15.4; FR-U12). runUpgrade NEVER os.Exit — it returns
// exitcode.New(exitcode.Error, err) / exitcode.New(exitcode.UpdateAvailable, nil) / nil; main maps via
// exitcode.For (mirrors every RunE in this package). confirmUpgrade's PLAIN error (no "stagecoach:"
// prefix) is wrapped via exitcode.New(exitcode.Error, err) WITHOUT re-adding the prefix (S2's rule;
// main.go adds it).
//
// # WHY THE SEAMS ARE FUNCTION SEAMS (not field injection)
//
// Two walls in package upgrade make naive field-injection unable to make the dispatch testable from
// package cmd, so three FUNCTION seams are required:
//
//  1. detect.go's Detector.Exec nil SKIPS the tier-(b) PM probes (nil is NOT the prod default), AND
//     upgrade.osRunner is UNEXPORTED ⇒ package cmd cannot build a production Detector by setting a
//     field. So upgradeDetect (default prodDetect) builds a Detector with a PACKAGE-CMD cmdRunner
//     (same contract as osRunner) and P1.M4.T3 overrides upgradeDetect wholesale to return a canned
//     channel — no real subprocess.
//
//  2. swap.go's Swap and rollback.go's Rollback resolve the running exe INTERNALLY via the package-
//     upgrade-level resolveCurrentExe seam ⇒ package cmd cannot steer the swap/rollback exe by field
//     injection. So upgradeSwap (default upgrade.Swap) and upgradeRollback (default upgrade.Rollback)
//     are function seams P1.M4.T3 overrides (or it overrides resolveCurrentExe in a package-upgrade
//     test) to point at a temp-dir exe.
//
// The literal field seams (the contract's Client/BaseURL + os.Executable + ExecRunner + token + new-
// client) cover everywhere field-injection DOES reach: upgradeBaseURL feeds the Client (the NETWORK
// seam — P1.M4.T3 sets it to an httptest.Server.URL); upgradeExePath feeds Detector.ExePath (detection
// only — NOT the swap); upgradeExecRunner feeds DelegateOptions.Exec (nil ⇒ Delegate's osExecRunner
// prod default — the OPPOSITE of Detector); upgradeToken + upgradeNewClient build the Client.
//
// ALL seams live in package cmd (ZERO edits to internal/upgrade/* — P1.M1–M3 are read-only; the seams
// make the dispatch fully testable WITHOUT editing them). runUpgrade reads EVERY seam var (no U1000);
// prodDetect reads cmdRunner. Walled off (FR-U12: the commit path is untouched by this file).
package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/dabstractor/stagecoach/internal/exitcode"
	"github.com/dabstractor/stagecoach/internal/upgrade"
)

// Seams — every var here is READ by runUpgrade (no U1000). P1.M4.T3 overrides them in package-cmd
// tests (restoring via t.Cleanup) to drive the full dispatch against an httptest server + fake runners
// + a temp exe with NO real network, NO real subprocess, NO real binary rename.
var (
	// upgradeBaseURL overrides the GitHub API root for the releases Client. "" ⇒ api.github.com (prod);
	// P1.M4.T3 sets it to an httptest.Server.URL (FR-U12 test wall — no real network).
	upgradeBaseURL string

	// upgradeExePath resolves the running binary for install-method DETECTION (Detector.ExePath). It
	// feeds DETECTION ONLY — the SWAP resolves its own exe internally via the package-upgrade
	// resolveCurrentExe seam (which is why upgradeSwap is a function seam, not this). Default:
	// os.Executable + EvalSymlinks (detect.go:352-354 idiom).
	upgradeExePath = prodExePath

	// upgradeExecRunner is the streaming subprocess seam for RUN-delegated updaters
	// (DelegateOptions.Exec). nil ⇒ upgrade.Delegate's osExecRunner prod default (the OPPOSITE of
	// Detector — for Delegate, nil IS prod). P1.M4.T3 injects a fake to assert argv + exit.
	upgradeExecRunner upgrade.ExecRunner

	// upgradeToken resolves the optional GitHub auth token. Default: STAGECOACH_GITHUB_TOKEN else
	// GITHUB_TOKEN. Feeds the Client.Token (rate-limit relief).
	upgradeToken = prodToken

	// upgradeNewClient builds the releases Client from repo/token + the upgradeBaseURL seam. Default
	// prodNewClient builds Client{BaseURL: upgradeBaseURL, Repo, Token} (HTTP nil ⇒ DefaultClient).
	upgradeNewClient = prodNewClient

	// FUNCTION seams (field-injection can't reach these — see the file doc):
	//
	// upgradeDetect builds the production Detector + Detect. osRunner is unexported, so prodDetect
	// uses the package-cmd cmdRunner (same contract). P1.M4.T3 overrides it to return a canned channel.
	upgradeDetect = prodDetect
	// upgradeSwap defaults to upgrade.Swap (Swap resolves the exe internally — package cmd cannot
	// steer it by field). P1.M4.T3 overrides it (or overrides package-upgrade resolveCurrentExe).
	upgradeSwap = upgrade.Swap
	// upgradeRollback defaults to upgrade.Rollback (Rollback resolves the exe internally). P1.M4.T3
	// overrides it (or overrides package-upgrade resolveCurrentExe).
	upgradeRollback = upgrade.Rollback
)

// prodExePath is the production upgradeExePath: os.Executable canonicalized via EvalSymlinks (macOS
// /private/var symlinks; Homebrew installs are symlinks into the Cellar), tolerating an EvalSymlinks
// failure by falling back to the raw os.Executable result (detect.go:352-354 / swap.go idiom).
func prodExePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("os.Executable: %w", err)
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		return real, nil
	}
	return exe, nil // tolerate EvalSymlinks failure
}

// prodToken is the production upgradeToken: STAGECOACH_GITHUB_TOKEN beats GITHUB_TOKEN (the
// stagecoach-scoped name takes precedence over the generic one). Empty ⇒ unauthenticated.
func prodToken() string {
	if t := os.Getenv("STAGECOACH_GITHUB_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("GITHUB_TOKEN")
}

// prodNewClient is the production upgradeNewClient: it builds the releases Client from the repo + token
// plus the upgradeBaseURL seam (the NETWORK seam — P1.M4.T3 sets upgradeBaseURL to an httptest URL).
// HTTP is left nil so releases.go's httpClient() falls back to http.DefaultClient.
func prodNewClient(repo, token string) *upgrade.Client {
	return &upgrade.Client{BaseURL: upgradeBaseURL, Repo: repo, Token: token}
}

// cmdRunner implements upgrade.Runner for the production tier-(b) PM DB queries. upgrade.osRunner is
// UNEXPORTED, so package cmd provides its own identical-contract twin. The contract is load-bearing
// (mirrors upgrade.osRunner + internal/git/git.go run): a NON-ZERO process exit — e.g. `brew list
// stagecoach` exits 1 when the package is not installed — ⇒ (stdout, code, nil) so the cascade treats
// "not installed" as a SKIP, not an error; only a start failure (binary absent) or a context deadline
// ⇒ err. Each Run wraps the command in a per-query 3s deadline (upgrade.DefaultQueryTimeout) +
// cmd.WaitDelay, mirroring upgrade.osRunner (the root ctx from main.go's signal.Install is
// signal-cancelable with NO deadline, so the per-query bound is load-bearing — FR-U2(b)/external_deps
// §7; BUG-004). The shared constant keeps the two runners from drifting.
type cmdRunner struct{}

// Run executes name args... and maps the outcome to the upgrade.Runner contract:
//   - exit 0 ⇒ (stdout, 0, nil);
//   - non-zero exit (*exec.ExitError) ⇒ (stdout, code, nil) — "not installed", the probe skips;
//   - start/LookPath failure (binary absent) or context deadline ⇒ ("", 0, err) — the probe skips.
//
// A context cancellation/timeout kills the child with a signal that surfaces as a *exec.ExitError, so
// ctx.Err() is checked FIRST (mirrors upgrade.osRunner's ordering).
func (cmdRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
	// BUG-004: bound each PM query to the shared 3s deadline so a hung PM (brew refreshing its DB, NFS
	// home, broken PM) cannot stall the cascade. The root ctx (main.go signal.Install) is signal-cancelable
	// with NO deadline, so this per-query bound is load-bearing (FR-U2(b)/external_deps §7). Mirrors
	// upgrade.osRunner.Run; the shared upgrade.DefaultQueryTimeout keeps the two runners from drifting.
	ctx, cancel := context.WithTimeout(ctx, upgrade.DefaultQueryTimeout)
	defer cancel()

	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &buf
	// WaitDelay bounds how long Run waits for the process (and any forked grandchild still holding the
	// stdout pipe) to release its I/O AFTER the deadline fires. Without it a hung PM that forked a helper
	// keeps the pipe open past the cancel. Same window as the timeout (mirrors osRunner).
	cmd.WaitDelay = upgrade.DefaultQueryTimeout

	if err := cmd.Run(); err != nil {
		// ctx deadline / cancellation kills the child with a signal that looks like an ExitError —
		// check ctx.Err() first so a timeout is treated as "PM hung" (err), not "not installed".
		if cerr := ctx.Err(); cerr != nil {
			return buf.String(), 0, cerr
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return buf.String(), ee.ExitCode(), nil // non-zero exit ⇒ skip, NOT err
		}
		return "", 0, err // start failure (binary absent) ⇒ err
	}
	return buf.String(), 0, nil
}

// prodDetect is the production upgradeDetect: it builds a Detector with the package-cmd cmdRunner
// (NON-NIL — detect.go treats nil Exec as SKIP, not prod), the upgradeExePath result for path
// heuristics, runtime.GOOS, the --install-method override, os.Getenv, and the injected verbose logger;
// then runs Detect. The override is the flagInstallMethod package var (S1). Errors are surfaced as
// PLAIN errors (no "stagecoach:" prefix — main.go adds it), consistent with every other RunE
// (e.g. ErrUnknownChannel from a bad --install-method).
func prodDetect(ctx context.Context, override string, log func(string)) (upgrade.Channel, string, error) {
	exe, err := upgradeExePath()
	if err != nil {
		return "", "", fmt.Errorf("%w", err)
	}
	d := &upgrade.Detector{
		Exec:     cmdRunner{}, // NON-NIL (nil skips PM probes — detect.go). osRunner is unexported.
		ExePath:  exe,
		GOOS:     runtime.GOOS,
		Override: override,
		Env:      os.Getenv,
		Log:      log,
	}
	return d.Detect(ctx)
}

// runCheck implements the FR-U6 --check path: resolve the target release, compare it to the running
// binary's CurrentSemver, and report. A dev build (CurrentSemver ok=false) is informational — it
// prints the latest tag but does NOT claim an update (dev never falsely signals; Compare returns 0 for
// an unparseable operand). Up-to-date (Compare >= 0) ⇒ exit 0. Behind ⇒ "update available: X → Z" +
// exit 6 (exitcode.UpdateAvailable; main maps via For). Never os.Exit; never prompts (FR-U9).
func runCheck(ctx context.Context, cmd *cobra.Command, client *upgrade.Client, effChannel string) error {
	cur, ok := upgrade.CurrentSemver()
	release, _, err := upgrade.ResolveTarget(ctx, client, upgrade.ResolveOptions{
		Version:    flagTargetVersion,
		Prerelease: effChannel == "prerelease",
	})
	if err != nil {
		return exitcode.New(exitcode.Error, fmt.Errorf("check: %w", err))
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

// runDirectSwap implements the FR-U5/U7/U9/U11 direct-binary self-swap path (ChannelDirect, or --force
// overriding a detected PM). Flow: MkdirTemp → ResolveTarget → StageNewBinary → confirmUpgrade (FR-U9)
// → upgradeSwap. NeedsPrivilegesError ⇒ printPrivilegeCommand + exit 0 (FR-U4/U7 — never auto-elevate).
// On Resolve/Stage/Swap failure the tempDir is LEFT for inspection (FR-U11) — there is NO defer cleanup
// (Swap cleans tempDir on SUCCESS; a defer would nuke the failure-inspection artifact). A non-TTY
// confirm refusal (no --yes) ⇒ exit 1; a TTY decline ⇒ exit 0. Never os.Exit.
func runDirectSwap(ctx context.Context, cmd *cobra.Command, client *upgrade.Client, effChannel string) error {
	log := verboseLog(cmd.ErrOrStderr())
	tempDir, err := os.MkdirTemp("", "stagecoach-upgrade-*")
	if err != nil {
		return exitcode.New(exitcode.Error, err)
	}
	// NO defer os.RemoveAll(tempDir): failure paths LEAVE it (FR-U11 inspection); Swap cleans it on success.
	release, asset, err := upgrade.ResolveTarget(ctx, client, upgrade.ResolveOptions{
		Version:    flagTargetVersion,
		Prerelease: effChannel == "prerelease",
	})
	if err != nil {
		return exitcode.New(exitcode.Error, err) // LEAVE tempDir
	}
	log("target: " + release.Tag)
	newBin, err := upgrade.StageNewBinary(ctx, client, release, asset, tempDir)
	if err != nil {
		return exitcode.New(exitcode.Error, err) // LEAVE tempDir (FR-U11)
	}
	out := cmd.OutOrStdout()
	ok, cerr := confirmUpgrade(displayCurrent(), release.Tag, "Self-swap the direct-binary install.", flagYes, cmd.InOrStdin(), out)
	if cerr != nil {
		return exitcode.New(exitcode.Error, cerr) // non-TTY refusal → exit 1 (NO re-prefix — S2's rule)
	}
	if !ok {
		return nil // TTY decline → exit 0
	}
	if serr := upgradeSwap(ctx, newBin); serr != nil {
		var npe *upgrade.NeedsPrivilegesError
		if errors.As(serr, &npe) {
			printPrivilegeCommand(out, npe.Command) // echo .Command verbatim
			return nil                              // exit 0 (FR-U4/U7 — never auto-elevate)
		}
		return exitcode.New(exitcode.Error, fmt.Errorf("%w", serr)) // exit 1
	}
	fmt.Fprintf(out, "stagecoach upgraded to %s\n", release.Tag)
	return nil // exit 0 (Swap already cleaned tempDir)
}

// runDelegate implements the FR-U3/U4/U9 delegation path (every non-direct channel: brew/scoop/
// npm/mise/asdf/go-install RUN; aur/nix/deb/rpm/chocolatey PRINT). RUN channels confirm first (FR-U9) — target "latest"
// (the PM fetches it; no extra ResolveTarget network call), action "Update via <channel>'s updater.".
// AUR/Nix/deb/rpm/chocolatey (PRINT) NEVER confirm. Delegate is called with upgradeExecRunner (nil ⇒ osExecRunner prod
// default). Ran=false (PRINT) ⇒ printDelegatedUpdate + exit 0 (FR-U4). Ran=true non-zero ⇒ exit 1
// (the raw updater code is NOT propagated — §15.4 is 0/1/6). Ran=true zero ⇒ exit 0.
//
// ErrDirectSwap is UNREACHABLE here: runUpgrade routes ChannelDirect (and --force) to runDirectSwap
// BEFORE runDelegate, so Delegate never sees ChannelDirect. There is NO defensive errors.Is(err,
// ErrDirectSwap) branch (dead code).
func runDelegate(ctx context.Context, cmd *cobra.Command, ch upgrade.Channel) error {
	out := cmd.OutOrStdout()
	// FR-U9: confirm ONLY RUN channels (AUR/Nix/deb/rpm/chocolatey are PRINT — they never prompt).
	isRun := ch != upgrade.ChannelAUR && ch != upgrade.ChannelNix && ch != upgrade.ChannelDeb && ch != upgrade.ChannelRpm && ch != upgrade.ChannelChocolatey
	if isRun {
		ok, cerr := confirmUpgrade(displayCurrent(), "latest", "Update via "+string(ch)+"'s updater.", flagYes, cmd.InOrStdin(), out)
		if cerr != nil {
			return exitcode.New(exitcode.Error, cerr) // non-TTY refusal → exit 1 (NO re-prefix)
		}
		if !ok {
			return nil // TTY decline → exit 0
		}
	}
	res, err := upgrade.Delegate(ctx, ch, upgrade.DelegateOptions{
		Exec:      upgradeExecRunner,             // nil ⇒ osExecRunner prod default
		Out:       out,                           // stream the updater's live output
		Env:       os.Getenv,                     // npm-variant detection (pnpm/bun)
		Verbose:   verboseLog(cmd.ErrOrStderr()), // --verbose logger (FR50)
		Confirmed: flagYes,                       // advisory (Delegate never prompts; the gate is above)
	})
	if err != nil {
		// Start/LookPath/timeout failure — the PM binary is unavailable/hung ⇒ exit 1.
		return exitcode.New(exitcode.Error, err)
	}
	if !res.Ran {
		// PRINT channel (AUR/Nix) — Delegate already printed the command; frame it. exit 0 (FR-U4).
		printDelegatedUpdate(out, string(ch), res.Command)
		return nil
	}
	if res.ExitCode != 0 {
		// The updater ran and reported failure. §15.4 is 0/1/6: map the raw code to exit 1 (NOT propagate it).
		return exitcode.New(exitcode.Error, fmt.Errorf("%s updater exited %d", ch, res.ExitCode))
	}
	fmt.Fprintf(out, "stagecoach updated via %s\n", ch)
	return nil // exit 0
}

// displayCurrent returns the running binary's version for the confirmUpgrade display: the canonical
// semver when CurrentSemver succeeds, else "dev" (the only !ok case in practice — there is NO exported
// raw getter in package upgrade, and adding one is out of scope; "dev" suffices for the prompt).
func displayCurrent() string {
	cur, ok := upgrade.CurrentSemver()
	if ok {
		return cur
	}
	return "dev"
}

// verboseLog returns a func(string) logger gated on flagVerbose (root.go's package var — pflag sets it
// regardless of the no-op PersistentPreRunE, so it is readable here even though config.Load is skipped).
// It writes "stagecoach: <msg>\n" to w. nil-safe consumers (Detector.Log / DelegateOptions.Verbose)
// receive it directly. FR50: logs detect steps, channel+evidence, target, planned action.
func verboseLog(w io.Writer) func(string) {
	return func(msg string) {
		if flagVerbose {
			fmt.Fprintln(w, "stagecoach: "+msg)
		}
	}
}
