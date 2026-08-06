// Package cmd implements the upgrade command for Stagecoach (PRD §9.29 FR-U1–U12, §15.4).
// It provides the `upgrade` cobra leaf on root — the v3.0 binary self-update command:
// stagecoach detects the install method and delegates to that channel's updater
// (Homebrew/Scoop/winget/npm/mise/asdf/Nix/AUR/go-install), self-swapping only for the
// direct-binary channel.
//
// This is NOT `config upgrade`. `config upgrade` (a SEPARATE command, run as
// `stagecoach config upgrade`) is a config-schema migration (it rewrites an existing
// config file to the current schema version). `stagecoach upgrade` updates the binary.
// Two different commands; the Long below disambiguates them explicitly.
//
// FR-U12 wall: the command defines a no-op PersistentPreRunE that OVERRIDES root's
// config.Load (cobra runs only the nearest ancestor's PersistentPreRunE). So `upgrade`
// acquires NO run lock, reads NO repo, invokes NO provider, and — critically — never
// triggers config.Load's first-run bootstrap write (FR-B3). It runs anywhere CWD is,
// inside or outside a git repo, with or without a config file. Same rationale as
// lock.go / hook.go / integrate.go.
//
// Network note: `upgrade` is the one named exception to the no-network-calls commit
// path (v3_scope_boundary §19) — it fetches ONLY this project's own GitHub release
// artifacts + checksums (never an arbitrary URL, never the agent APIs).
//
// Scope: this file owns the command SHELL — the command definition, its 9 LOCAL flags,
// flag validation, and effective channel/source-repo resolution — PLUS the runUpgrade
// DISPATCH entry (the three-way branch + detect→direct-swap|delegate routing). The path
// helpers (runCheck/runDirectSwap/runDelegate) and the injectable test seams live in
// upgrade_run.go (P1.M4.T2.S1).
//
// Registered via init() — ZERO edits to root.go (the providers.go/hook.go/integrate.go/
// lock.go pattern). The 9 flags are LOCAL to upgradeCmd.Flags() (NOT persistent) so they
// do NOT pollute the commit-path flag set (FR-U9/U10).
package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/exitcode"
	"github.com/dabstractor/stagecoach/internal/upgrade"
)

// upgrade flags (FR-U9/U10). Zero defaults so presence is detectable. Bound to PACKAGE
// VARS in init() (the root.go flag-binding convention) but registered on upgradeCmd.Flags()
// (LOCAL — NOT persistent) so they never appear on the commit path.
//
// NOTE: --version is NOT a collision with cobra's auto --version. cobra adds the auto
// --version flag ONLY to the command whose Version field is set (rootCmd, via
// rootCmd.Version = Version). It is a LOCAL flag on rootCmd, NOT inherited by subcommands.
// upgradeCmd has NO Version field → no auto-version flag → upgradeCmd.Flags().String(
// "version", ...) is clean. (Two different commands, two different --version flags.)
var (
	flagCheck         bool   // --check/-c: check-only (FR-U6); exit 6 if behind
	flagTargetVersion string // --version <v>: pin a target release (FR-U5)
	flagPrerelease    bool   // --prerelease: = --channel prerelease (FR-U10)
	flagForce         bool   // --force: override a detected package manager (FR-U1)
	flagRollback      bool   // --rollback: restore the most recent backup (FR-U8)
	flagInstallMethod string // --install-method <m>: override detection (FR-U2; env STAGECOACH_INSTALL_METHOD)
	flagYes           bool   // --yes/-y: skip the confirmation prompt (FR-U9)
	flagChannel       string // --channel <stable|prerelease> (FR-U10)
	flagSourceRepo    string // --source-repo <owner/repo> (FR-U10; for forks)
)

// upgradeCmd is the PRD §9.29 binary self-update command (FR-U1). It is a LEAF (no
// subcommands), so RunE + Args are set directly on it. Its no-op PersistentPreRunE
// OVERRIDES root's config.Load (cobra runs only the nearest ancestor's): `upgrade`
// needs no config and must run outside a git repo — and must NOT trigger config.Load's
// first-run bootstrap write (FR-B3). Same rationale as lock.go.
var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Update the stagecoach binary to the latest release",
	Long: `Update the stagecoach binary to the latest release (PRD §9.29 FR-U1).

stagecoach detects the install method and delegates to that channel's updater
(Homebrew, Scoop, winget, npm, mise, asdf, Nix, AUR, go install), self-swapping only
for the direct-binary channel. This is the v3.0 delegate-first updater.

Distinct from 'config upgrade': 'config upgrade' (run as 'stagecoach config upgrade')
rewrites an existing config file to the current config-file schema version in place (a
one-time file-format update). 'stagecoach upgrade' updates the BINARY. Two different
commands; do not confuse them.

This command makes network calls to GitHub Releases only (this project's own release
artifacts + checksums). It is the one named exception to the no-network-calls commit
path. It acquires no run lock, reads no repo, invokes no provider, and runs outside a
git repo.`,
	SilenceErrors:     true,
	SilenceUsage:      true,
	PersistentPreRunE: func(*cobra.Command, []string) error { return nil }, // OVERRIDES root's config.Load (FR-B3/FR-U12)
	Args:              cobra.NoArgs,
	RunE:              runUpgrade,
}

func init() {
	fs := upgradeCmd.Flags()
	fs.BoolVarP(&flagCheck, "check", "c", false,
		"Check for an update without applying it (exit 6 if behind, 0 if up to date) (FR-U6)")
	fs.StringVar(&flagTargetVersion, "version", "",
		"Pin a target version to install (default: latest in the channel) (FR-U5)")
	fs.BoolVar(&flagPrerelease, "prerelease", false,
		"Admit pre-release tags (shorthand for --channel prerelease) (FR-U10)")
	fs.BoolVar(&flagForce, "force", false,
		"Override a detected package-manager install and self-swap (FR-U1)")
	fs.BoolVar(&flagRollback, "rollback", false,
		"Restore the most recent backup (one-step undo) (FR-U8)")
	fs.StringVar(&flagInstallMethod, "install-method", "",
		"Override install-method detection (env STAGECOACH_INSTALL_METHOD) (FR-U2)")
	fs.BoolVarP(&flagYes, "yes", "y", false,
		"Skip the confirmation prompt (for scripting) (FR-U9)")
	fs.StringVar(&flagChannel, "channel", "",
		"Release channel: stable (default) | prerelease (FR-U10)")
	fs.StringVar(&flagSourceRepo, "source-repo", "",
		"owner/repo to fetch releases from (default dabstractor/stagecoach; for forks) (FR-U10)")
	rootCmd.AddCommand(upgradeCmd) // register on root — NO edit to root.go (hook/integrate/providers/lock pattern)
}

// validateUpgradeFlags enforces the 3 flag-contract rules for `stagecoach upgrade`:
//  1. --version + --prerelease are mutually exclusive (both pin the target; redundant/confusing).
//  2. --rollback is exclusive with --check and --version (rollback restores a backup; it neither
//     checks nor targets a specific version).
//  3. --channel rejects unknown values (valid: "" (not-set), "stable", "prerelease").
//
// The bool mutex rules (1)/(2) use fs.Changed, NOT the package var: flagCheck==false is
// ambiguous (not-set vs --check=false); fs.Changed is true ONLY if the user passed it. The
// --channel enum rule (3) CAN use the flagChannel package var: default "" ⇒ not-set ⇒ OK, and a
// non-empty non-enum value is a real error. Pure helper (takes *pflag.FlagSet) — unit-testable.
func validateUpgradeFlags(fs *pflag.FlagSet) error {
	// (1) --version + --prerelease mutually exclusive.
	if fs.Changed("version") && fs.Changed("prerelease") {
		return fmt.Errorf("--version and --prerelease are mutually exclusive")
	}
	// (2) --rollback exclusive with --check and --version.
	if fs.Changed("rollback") && (fs.Changed("check") || fs.Changed("version")) {
		return fmt.Errorf("--rollback cannot be combined with --check or --version")
	}
	// (3) --channel rejects unknown values ("" ⇒ not-set ⇒ OK).
	if flagChannel != "" && flagChannel != "stable" && flagChannel != "prerelease" {
		return fmt.Errorf("--channel %q: must be stable or prerelease", flagChannel)
	}
	return nil
}

// runUpgrade implements `stagecoach upgrade` (PRD §9.29): validate flags, resolve the
// effective channel/source-repo, then DISPATCH to the three paths (--check / normal
// detect→delegate|swap / --rollback). The dispatch + the path helpers + the injectable
// test seams live in upgrade_run.go (P1.M4.T2.S1).
//
// Resolution (FR-U10): flag > [upgrade] config (global only) > Defaults. NO env for
// channel/source-repo (FR-U10 lists only config + flags). config.LoadUpgradeConfig reads
// the global file ONLY and never bootstraps (the no-op PersistentPreRunE already skipped
// config.Load, and LoadUpgradeConfig is deliberately walled off from it).
//
// Never exits the process directly: returns exitcode.New(exitcode.Error, err) on failure,
// nil on success. main maps via exitcode.For (mirrors every other RunE in this package).
func runUpgrade(cmd *cobra.Command, _ []string) error {
	if err := validateUpgradeFlags(cmd.Flags()); err != nil {
		return exitcode.New(exitcode.Error, err)
	}

	// Resolve effective channel/source-repo (FR-U10): flag > [upgrade] config (global) > Defaults.
	// NO env for channel/source-repo (FR-U10). LoadUpgradeConfig reads the global file ONLY —
	// never bootstraps a config (FR-B3 boundary), never reads the per-repo file (FR-U12).
	uc, err := config.LoadUpgradeConfig()
	if err != nil {
		return exitcode.New(exitcode.Error, err)
	}

	effChannel := uc.Channel // Defaults: "stable"
	if flagChannel != "" {
		effChannel = flagChannel
	} else if flagPrerelease {
		effChannel = "prerelease" // --prerelease = --channel prerelease (FR-U10)
	}

	effSourceRepo := uc.SourceRepo // Defaults: "dabstractor/stagecoach"
	if flagSourceRepo != "" {
		effSourceRepo = flagSourceRepo
	}

	// Dispatch (PRD §9.29 FR-U1–U12; §15.4 exit block 0/1/6 ONLY). The three paths consume the
	// LANDED internal/upgrade primitives via the package-cmd seams in upgrade_run.go:
	//   - --rollback (FR-U8) → upgradeRollback; ErrNoBackup ⇒ "no backup …" + exit 0.
	//   - --check (FR-U6)    → runCheck: ResolveTarget + CurrentSemver + Compare; behind ⇒ exit 6.
	//   - normal (FR-U1)     → upgradeDetect; ChannelDirect (or --force+warning) ⇒ runDirectSwap;
	//                          else ⇒ runDelegate.
	// NEVER os.Exit — returns exitcode.New(exitcode.Error, err) / exitcode.New(exitcode.UpdateAvailable,
	// nil) / nil; main maps via exitcode.For (mirrors every RunE).
	return dispatchUpgrade(cmd.Context(), cmd, effChannel, effSourceRepo)
}

// dispatchUpgrade is the thin dispatcher runUpgrade delegates to (kept in upgrade.go next to
// runUpgrade for readability; the path helpers + seams live in upgrade_run.go). It implements the
// three-way branch on flagRollback/flagCheck/normal and the detect→direct-swap|delegate routing for
// the normal path. It reads the S1 flagX package vars (flagRollback/flagCheck/flagForce/flagYes/
// flagInstallMethod/flagTargetVersion) directly. NEVER os.Exit; returns exitcode.New/nil.
func dispatchUpgrade(ctx context.Context, cmd *cobra.Command, effChannel, effSourceRepo string) error {
	log := verboseLog(cmd.ErrOrStderr())

	// --rollback path (FR-U8): one-step undo of a prior direct-binary upgrade.
	if flagRollback {
		ver, err := upgradeRollback(ctx)
		if errors.Is(err, upgrade.ErrNoBackup) {
			fmt.Fprintln(cmd.OutOrStdout(), "no backup — nothing to roll back")
			return nil // exit 0
		}
		if err != nil {
			return exitcode.New(exitcode.Error, fmt.Errorf("rollback: %w", err)) // exit 1
		}
		fmt.Fprintf(cmd.OutOrStdout(), "restored stagecoach %s\n", ver)
		return nil // exit 0
	}

	client := upgradeNewClient(effSourceRepo, upgradeToken())

	// --check path (FR-U6): check-only; exit 6 if behind, 0 if up-to-date/dev.
	if flagCheck {
		return runCheck(ctx, cmd, client, effChannel)
	}

	// Normal path (FR-U1): detect the install method, then route.
	ch, evidence, err := upgradeDetect(ctx, flagInstallMethod, log)
	if err != nil {
		// ErrUnknownChannel (a bad --install-method) or the os.Executable failure ⇒ exit 1.
		return exitcode.New(exitcode.Error, err)
	}
	log("detected install method: " + string(ch) + " (" + evidence + ")")

	// ChannelDirect is the ONLY self-swap-eligible channel (FR-U1/U5). --force overrides a detected
	// package-manager install and self-swaps too (FR-U1) — warn first so the user knows a PM-owned
	// binary is about to be overwritten (the PM's next upgrade may revert it).
	if ch == upgrade.ChannelDirect || flagForce {
		if flagForce && ch != upgrade.ChannelDirect {
			fmt.Fprintln(cmd.ErrOrStderr(), "stagecoach: warning: --force overriding a detected "+string(ch)+" install (FR-U1); self-swapping")
		}
		return runDirectSwap(ctx, cmd, client, effChannel)
	}
	return runDelegate(ctx, cmd, ch)
}
