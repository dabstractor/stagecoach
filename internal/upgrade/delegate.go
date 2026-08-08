// delegate.go implements the FR-U3 delegation table + FR-U4 run-vs-print policy — the dispatcher
// the `stagecoach upgrade` command (P1.M4.T2 runUpgrade) calls after Detect (detect.go) has resolved
// how the running binary was installed. Given a detected Channel, Delegate builds that channel's
// NATIVE updater command and either RUNs it (brew/scoop/npm/mise/asdf/go-install — streaming
// its output live to opts.Out) or PRINTs it (aur needs root; nix is declarative/immutable;
// chocolatey needs admin and owns the binary — FR-U1/FR-U4). The
// `direct` channel is NOT delegated: Delegate returns ErrDirectSwap and the command layer routes it
// to the P1.M3 self-swap (FR-U5). This is the "delegate-first" core of `stagecoach upgrade`: it
// never overwrites a package-manager-owned file (FR-U1) because it never writes a file at all — it
// asks the manager to perform the update so the manager's bookkeeping stays consistent and its next
// upgrade does not revert stagecoach.
//
// Every environment-touching seam is an injectable DelegateOptions field (the exec runner, the
// output stream, the env getter for npm-variant detection, the verbose logger) so the dispatcher is
// fully unit-testable against a canned fakeExecRunner without any real package manager on PATH. The
// streaming ExecRunner is DISTINCT from detect.go's capturing Runner — same package, different name
// and semantics (Run takes stdout/stderr io.Writers and STREAMS, rather than capturing stdout to a
// string), so detect.go's Runner/osRunner names are not reused here. The exit-vs-start-failure
// distinction (a *exec.ExitError is NOT a Delegate error — the updater ran and reported failure;
// only a Start/LookPath failure is) mirrors detect.go's osRunner and the house git subprocess
// pattern (internal/git/git.go run). Delegate never self-swaps (FR-U1), never auto-sudo (FR-U4),
// and never prompts — the command layer (P1.M4.T1.S2) owns the y/N confirmation + --yes and sets
// the pre-confirmed bool. Walled off (FR-U12: stdlib-only, no internal/* imports); the verbose
// logger is injected as a func(string) field, not imported from internal/ui. File comment only —
// releases.go owns the package doc.
package upgrade

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// npmPackage is the npm registry package spec for stagecoach. It is shared by the npm, pnpm, and bun
// variant argvs (they differ only in the command verb: npm uses "install", pnpm/bun use "add").
const npmPackage = "stagecoach-ai@latest"

// ErrDirectSwap is returned by Delegate when the detected channel is ChannelDirect. The direct
// channel (a standalone binary, not managed by any package manager) is NOT delegated — its update
// path is the P1.M3 verified-atomic self-swap (FR-U5), which is the command layer's job
// (P1.M4.T2 runUpgrade routes ErrDirectSwap to the swap). Delegate never performs a self-swap
// (FR-U1) and never overwrites a binary; it hands the direct case off via this sentinel. errors.Is
// is the intended check (mirrors ErrHTTP/ErrRateLimited in releases.go).
var ErrDirectSwap = errors.New("upgrade: direct channel requires self-swap, not delegation")

// ExecRunner is the injectable streaming subprocess seam for the RUN channels. Production code uses
// an osExecRunner (exec.CommandContext + stream); tests inject a canned fakeExecRunner that records
// calls and returns canned (code, err).
//
// The (exitCode, err) contract is load-bearing and mirrors the distinction in detect.go's osRunner
// (and internal/git/git.go run): a NON-ZERO process exit — e.g. `brew upgrade` exits 1 on a network
// blip — is returned as (exitCode, nil) so Delegate treats it as "the updater ran and reported
// failure" (Ran:true, ExitCode:code, err==nil; the command layer P1.M4 maps the code to §15.4 exit
// codes). Only infrastructural failures (the PM binary absent on PATH / a start failure / a context
// deadline) return err != nil — that IS a Delegate error (Ran:true, ExitCode, err). Conflating a
// non-zero exit with an error would make every failed `brew upgrade` look like "brew not installed".
//
// Distinct from detect.go's capturing Runner: Runner CAPTURES stdout to a string (for parsing PM DB
// query output), while ExecRunner STREAMS stdout/stderr to the injected writers (for surfacing the
// updater's live output to the terminal). Runner/osRunner are already in package upgrade; ExecRunner
// reuses neither name.
type ExecRunner interface {
	// Run executes name args... and STREAMS its stdout/stderr to the supplied writers (FR-U4:
	// "streaming its output"). Returns (exitCode, nil) for a process exit (zero or non-zero) and
	// (0, err) for a start/LookPath failure or context deadline (the PM is unavailable/hung).
	Run(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) (exitCode int, err error)
}

// osExecRunner is the production ExecRunner. Each Run builds an exec.CommandContext and assigns the
// injected stdout/stderr writers so the updater's output streams live to opts.Out (and, by the
// Delegate convention, opts.Out is used for both streams so the user sees a single interleaved
// stream). No Setpgid/process-group setup is applied — a delegated brew/scoop/npm/mise/asdf is the USER's
// package manager running in the FOREGROUND group; Ctrl-C reaches it naturally (the Setpgid dance in
// internal/provider/executor.go is for stagecoach's OWN child agents in the commit path — a
// different concern). The exit-vs-start-failure distinction is the same as detect.go's osRunner.
type osExecRunner struct{}

// Run executes name args... and maps the outcome to the ExecRunner contract:
//   - exit 0 ⇒ (0, nil) — the updater succeeded.
//   - non-zero exit (*exec.ExitError) ⇒ (code, nil) — the updater ran and reported failure (NOT an
//     error for Delegate; the command layer maps the code to §15.4).
//   - start/LookPath failure (binary absent) or context deadline ⇒ (0, err) — the PM is unavailable
//     or hung; that IS a Delegate error.
//
// A context cancellation/timeout kills the child with a signal that surfaces as a *exec.ExitError
// (signal: killed), so ctx.Err() is checked FIRST — mirroring detect.go's osRunner — so a timeout is
// treated as a start-failure-style error (PM hung) rather than a non-zero exit.
func (osExecRunner) Run(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		// Per-query timeout / cancelled context kills the child with a signal that looks like an
		// ExitError — check ctx.Err() first so the timeout is treated as "PM hung" (start-failure-
		// style), not a non-zero "updater failed" exit. Mirrors detect.go's osRunner ordering.
		if cerr := ctx.Err(); cerr != nil {
			return 0, cerr
		}
		// A non-zero process exit is *exec.ExitError — extract the code, return err==nil so Delegate
		// treats it as "the updater ran and reported failure" (the load-bearing distinction).
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), nil
		}
		// Start failure (binary absent on PATH) or I/O failure — the PM is unavailable right now.
		return 0, err
	}
	return 0, nil
}

// DelegateOptions holds every injectable seam for Delegate. All fields are optional/nil-safe; a
// minimal opts with just Out set (and Exec left nil) is the production shape (Exec nil ⇒ osExecRunner
// prod default, exactly like Client.HTTP nil ⇒ http.DefaultClient in releases.go). The command layer
// (P1.M4.T1.S1/S2) constructs this: Out ← the terminal stdout, Env ← os.Getenv, Verbose ←
// ui.Verbose, Confirmed ← (the y/N prompt result OR --yes). Delegate itself never prompts and never
// reads the environment directly.
type DelegateOptions struct {
	// Exec is the streaming subprocess seam for the RUN channels. nil ⇒ osExecRunner (production).
	// Tests inject a fakeExecRunner to assert the exact argv + exit-code mapping without any real PM.
	Exec ExecRunner

	// Out is the stream target for RUN channels (the updater's live stdout/stderr) AND the print
	// destination for the AUR/Nix PRINT channels (the exact command for the user to run). It is the
	// one required field — production wires the terminal stdout; tests wire a bytes.Buffer. A nil Out
	// is tolerated as io.Discard so a library caller that forgets it gets silent rather than panicking
	// (the update still runs).
	Out io.Writer

	// Env reads the environment for the npm-variant detection (PNPM_HOME ⇒ pnpm, BUN_INSTALL ⇒ bun).
	// nil ⇒ npmVariant returns "npm" (the default; Detect already resolved ChannelNpm so the variant
	// only picks command SYNTAX — it never shells out to `npm config`/`which pnpm`). yarn is deferred
	// (berry removed `global`); the variant→argv map is trivially extensible.
	Env func(string) string

	// Verbose is the --verbose logger; nil ⇒ no-op. It logs the resolved command (e.g.
	// "running: brew upgrade stagecoach"). Injected as a func(string) so delegate.go stays walled off
	// (no internal/ui import, FR-U12); the command layer wires ui.Verbose in.
	Verbose func(string)

	// Confirmed is the pre-confirmation flag set by the command layer (P1.M4.T1.S2) AFTER its y/N
	// prompt (or when --yes is passed). Delegate NEVER prompts — it trusts this flag. It is an
	// exported field so staticcheck U1000 never fires on it even though Delegate does not branch on it
	// for control flow (the command layer gates the whole Delegate call on confirmation rather than
	// Delegate re-checking; keeping the field here documents the contract and leaves room for a future
	// per-command confirmation without changing the API).
	Confirmed bool
}

// DelegateResult is the dispatcher's return. The command layer (P1.M4.T2 runUpgrade) maps it to the
// §15.4 exit codes: a print channel (Ran=false) ⇒ exit 0 ("here is how to update"); a run channel
// with ExitCode 0 ⇒ exit 0 (success); a run channel with non-zero ExitCode ⇒ exit non-zero (run
// failure); ErrDirectSwap ⇒ route to the P1.M3 self-swap. Delegate returns the RAW updater exit code
// — it does NOT map to 0/1/6 (that is the command layer's job; coupling it here would conflate the
// dispatcher with the CLI surface).
type DelegateResult struct {
	// Ran is true when a real updater was executed (a RUN channel) and false when the command was
	// printed for the user to run (AUR/Nix) or when Delegate returned an error before exec (the
	// direct/ErrDirectSwap case returns a zero-value result with Ran=false).
	Ran bool

	// Command is the executed/printed command as a single string. For RUN channels it is the argv
	// space-joined (and " && "-joined for asdf's install+global 2-step). For PRINT channels it is the
	// PRIMARY command (the first line of the printed text — e.g. "sudo pacman -Syu stagecoach-bin"),
	// so the command layer can surface "to update, run: <Command>" without re-parsing the print body.
	Command string

	// ExitCode is the updater's exit code for RUN channels (0 = success); 0 for PRINT channels. For
	// a start-failure (err != nil) it carries the runner's code (0 from osExecRunner) — the command
	// layer keys off err, not ExitCode, in that case.
	ExitCode int
}

// out returns opts.Out or io.Discard when Out is nil, so a library caller that forgets Out gets a
// silent run rather than a nil-pointer write panic. Production (the command layer) always wires the
// terminal stdout; tests wire a bytes.Buffer.
func (o DelegateOptions) out() io.Writer {
	if o.Out != nil {
		return o.Out
	}
	return io.Discard
}

// Delegate maps a detected install Channel to its native updater and either RUNs it (streaming its
// output to opts.Out) or PRINTs it (writing the exact command for the user to run). The dispatch
// follows the FR-U3 table + the FR-U4 run-vs-print policy:
//
//   - ChannelDirect → (zero, ErrDirectSwap). The direct channel's update is the P1.M3 self-swap
//     (FR-U5); Delegate never self-swaps (FR-U1) and hands it off via the sentinel.
//   - ChannelAUR, ChannelNix, ChannelDeb, ChannelRpm → PRINT. AUR needs root (FR-U4); Nix is
//     immutable/declarative; .deb/.rpm ship in Releases with NO apt/dnf repo (the PM updaters
//     cannot fetch a new version) and are PM-owned (FR-U1 — never self-swap). The exact command is
//     written to opts.Out and Ran=false, ExitCode=0 (FR-U4: "a printed command exits 0").
//     ChannelChocolatey also PRINTs: choco owns the binary under ProgramData\chocolatey (FR-U1 —
//     never self-swap) and `choco upgrade` needs admin (FR-U4 — never auto-elevate).
//   - everything else (brew/scoop/npm/mise/asdf/go-install) → RUN. The channel's argv is
//     built by runArgv and executed via opts.Exec (nil ⇒ osExecRunner), streaming to opts.Out.
//     asdf is a 2-step (install then global), executed in sequence; the loop stops on the first
//     non-zero exit or error.
//
// A run-channel non-zero exit ⇒ (DelegateResult{Ran:true, ExitCode:code}, nil) — an exit is NOT a
// Delegate error (the updater ran and reported failure; the command layer maps the code). A
// start-failure (the PM binary absent) ⇒ (DelegateResult{Ran:true}, err) — that IS a Delegate error.
// FR-U4: Delegate NEVER auto-sudo/auto-elevate (no RUN argv begins with "sudo"; sudo appears only in
// the AUR print string the USER runs).
func Delegate(ctx context.Context, ch Channel, opts DelegateOptions) (DelegateResult, error) {
	switch ch {
	case ChannelDirect:
		// The direct channel is not delegated — its update is the P1.M3 self-swap (FR-U5). Return the
		// sentinel; the command layer routes it. Exec is never called (no recorded calls).
		return DelegateResult{}, ErrDirectSwap

	case ChannelAUR, ChannelNix, ChannelDeb, ChannelRpm, ChannelChocolatey:
		// PRINT channels: AUR needs root, Nix is declarative/immutable, Chocolatey needs admin and
		// owns the binary (FR-U1/FR-U4). Write the exact command for the user to run; a printed
		// command exits 0 (FR-U4).
		primary, full := printCommand(ch)
		fmt.Fprintln(opts.out(), full)
		verbose(opts, "printed update command for "+string(ch)+": "+primary)
		return DelegateResult{Ran: false, Command: primary, ExitCode: 0}, nil

	default:
		// RUN channels: brew/scoop/npm/mise/asdf/go-install. Build the channel's argv (asdf is
		// a 2-step), stream the updater's output to opts.Out, and return Ran:true + the exit code.
		argvs := runArgv(ch, opts)
		cmd := joinArgv(argvs)
		verbose(opts, "running: "+cmd)

		runner := opts.Exec
		if runner == nil {
			runner = osExecRunner{} // prod default (mirror Client.HTTP nil ⇒ http.DefaultClient).
		}

		out := opts.out()
		code := 0
		for _, a := range argvs {
			c, err := runner.Run(ctx, out, out, a[0], a[1:]...)
			if err != nil {
				// Start/LookPath/timeout failure — the PM binary is unavailable/hung. That IS a
				// Delegate error (Ran:true because we attempted the run).
				return DelegateResult{Ran: true, Command: cmd, ExitCode: c}, err
			}
			if c != 0 {
				// The updater ran and reported failure — NOT a Delegate error; the command layer maps
				// the code to §15.4. Stop the multi-step loop (e.g. asdf) on the first failure.
				return DelegateResult{Ran: true, Command: cmd, ExitCode: c}, nil
			}
			code = c // 0 (the loop only continues on success; code stays 0 across a clean multi-step run).
		}
		return DelegateResult{Ran: true, Command: cmd, ExitCode: code}, nil
	}
}

// verbose logs msg to the injected Verbose logger when one is configured; it is a no-op when Verbose
// is nil so a zero-value DelegateOptions never calls a nil func (mirrors Detector.log in detect.go).
func verbose(opts DelegateOptions, msg string) {
	if opts.Verbose != nil {
		opts.Verbose(msg)
	}
}

// runArgv returns the channel's native updater argv(s) for the RUN channels (FR-U3 table). The
// return is a slice-of-slices so asdf's 2-step (install + global) is a clean 2-element slice; every
// other RUN channel is a single-element slice. asdf's "if it was global" condition (FR-U3) is
// simplified to "always set global": asdf global is idempotent, and an upgrade implies promoting to
// the active latest, so always running both steps is safe and matches the user's intent (documented
// simplification). npm delegates to npmVariant to pick pnpm/bun/npm command syntax from the injected
// Env (Detect already resolved ChannelNpm — the variant only picks SYNTAX, it never re-detects).
func runArgv(ch Channel, opts DelegateOptions) [][]string {
	switch ch {
	case ChannelBrew:
		return [][]string{{"brew", "upgrade", "stagecoach"}}
	case ChannelScoop:
		return [][]string{{"scoop", "update", "stagecoach"}}
	case ChannelMise:
		return [][]string{{"mise", "upgrade", "stagecoach"}}
	case ChannelGoInstall:
		// The go-install argv uses the REAL module path (go.mod: github.com/dabstractor/stagecoach).
		return [][]string{{"go", "install", "github.com/dabstractor/stagecoach/cmd/stagecoach@latest"}}
	case ChannelNpm:
		// npm-variant: pnpm/bun use "add"; npm uses "install". The package spec is shared.
		switch v := npmVariant(opts.Env); v {
		case "pnpm":
			return [][]string{{"pnpm", "add", "-g", npmPackage}}
		case "bun":
			return [][]string{{"bun", "add", "-g", npmPackage}}
		default: // "npm" (and any future default).
			return [][]string{{"npm", "install", "-g", npmPackage}}
		}
	case ChannelAsdf:
		// 2-step: install then global. "global" is idempotent; an upgrade implies promoting to the
		// active latest (FR-U3 simplification — noted).
		return [][]string{
			{"asdf", "install", "stagecoach", "latest"},
			{"asdf", "global", "stagecoach", "latest"},
		}
	default:
		// Unreachable for the documented RUN channels (Delegate switches only the known channels into
		// this default branch). Return a zero-value slice so an unknown channel no-ops rather than
		// panicking; the command layer never feeds an unknown channel here (Detect validates).
		return nil
	}
}

// npmVariant picks the npm-family command syntax from the injected Env: PNPM_HOME set ⇒ pnpm,
// BUN_INSTALL set ⇒ bun, else npm (nil Env ⇒ npm). This only picks COMMAND SYNTAX — Detect already
// resolved ChannelNpm, so it never shells out to `npm config`/`which pnpm` (that would be detection,
// not delegation). yarn is intentionally deferred: berry removed the `global` subcommand so
// yarn-global is genuinely unreliable; defaulting to npm is safe because the package is the same
// (stagecoach-ai). The variant→argv map in runArgv is trivially extensible.
func npmVariant(env func(string) string) string {
	if env == nil {
		return "npm"
	}
	if env("PNPM_HOME") != "" {
		return "pnpm"
	}
	if env("BUN_INSTALL") != "" {
		return "bun"
	}
	return "npm"
}

// joinArgv joins a slice-of-argv (as returned by runArgv) into a single human-readable command
// string: each argv is space-joined, and the per-step strings are joined with " && " so asdf's
// 2-step renders as "asdf install stagecoach latest && asdf global stagecoach latest". A single-step
// channel (the common case) yields just its space-joined argv with no separator.
func joinArgv(argvs [][]string) string {
	parts := make([]string, 0, len(argvs))
	for _, a := range argvs {
		parts = append(parts, strings.Join(a, " "))
	}
	return strings.Join(parts, " && ")
}

// printCommand returns the (primary, full) command text for the PRINT channels (AUR/Nix). primary is
// the single canonical command (the first line) — it becomes DelegateResult.Command so the command
// layer can surface "to update, run: <Command>". full is the multi-line text written to opts.Out
// (primary + a comment line with an alternative for the user's setup). FR-U4: AUR needs root (so it
// is printed, not auto-sudo'd); Nix is immutable/declarative (no in-place swap — a flake user runs
// `nix flake update` in their config, not a profile upgrade).
func printCommand(ch Channel) (primary, full string) {
	switch ch {
	case ChannelAUR:
		// AUR/pacman needs root — print the exact command for the user (FR-U4: never auto-sudo). The
		// alternative (yay) covers AUR-helper users. sudo appears ONLY here (never in a RUN argv).
		primary = "sudo pacman -Syu stagecoach-bin"
		full = "sudo pacman -Syu stagecoach-bin\n# (or, with an AUR helper: yay -Syu stagecoach-bin)"
		return primary, full
	case ChannelNix:
		// Nix is immutable/declarative — no in-place swap. Print the imperative-profile command as the
		// primary and note the declarative/flake alternative (the user runs it in their config repo).
		primary = "nix profile upgrade stagecoach"
		full = "nix profile upgrade stagecoach\n# (declarative/flake users: run `nix flake update` in your config)"
		return primary, full
	case ChannelDeb:
		// .deb (Debian/Ubuntu/Mint) ships in GitHub Releases with NO apt repo, so apt's own updater
		// cannot fetch a new version. Per FR-U1 /usr/bin/stagecoach is dpkg-owned (never self-swap);
		// print the canonical apt reinstall + a no-repo fallback (download the new .deb). Needs root
		// ⇒ print, never auto-sudo (FR-U4).
		primary = "sudo apt install --only-upgrade stagecoach"
		full = "sudo apt install --only-upgrade stagecoach\n" +
			"# (stagecoach's .deb is in GitHub Releases, NOT an apt repo; if apt finds no update,\n" +
			"#  download the new stagecoach_<version>_linux_amd64.deb from\n" +
			"#  https://github.com/dabstractor/stagecoach/releases/latest and run:\n" +
			"#  sudo apt install ./stagecoach_<version>_linux_amd64.deb)"
		return primary, full
	case ChannelRpm:
		// .rpm (Fedora/RHEL/Rocky/Alma/SUSE) ships in GitHub Releases with NO dnf repo, so dnf's own
		// updater cannot fetch a new version. FR-U1: /usr/bin/stagecoach is rpm-owned (never self-swap);
		// print the canonical dnf upgrade + a no-repo fallback. Needs root ⇒ print (FR-U4).
		primary = "sudo dnf upgrade stagecoach"
		full = "sudo dnf upgrade stagecoach\n" +
			"# (stagecoach's .rpm is in GitHub Releases, NOT a dnf repo; if dnf finds no update,\n" +
			"#  download the new .rpm from https://github.com/dabstractor/stagecoach/releases/latest\n" +
			"#  and run: sudo dnf install ./stagecoach-<version>.<arch>.rpm)"
		return primary, full
	case ChannelChocolatey:
		// Chocolatey owns the binary under ProgramData\chocolatey (FR-U1: never self-swap) and
		// `choco upgrade` needs admin (FR-U4: print, never auto-elevate). The user runs this.
		primary = "choco upgrade stagecoach -y"
		full = "choco upgrade stagecoach -y\n# (choco owns the binary under ProgramData\\chocolatey; run as admin — FR-U1/FR-U4)"
		return primary, full
	default:
		// Unreachable: Delegate routes only ChannelAUR/ChannelNix/ChannelDeb/ChannelRpm/ChannelChocolatey
		// into printCommand. Return empty strings so an unknown channel no-ops rather than panicking.
		return "", ""
	}
}
