// Package cmd: upgrade_prompt.go holds the user-facing I/O half of `stagecoach upgrade`
// (PRD §9.29). It provides three shared helpers the dispatch (P1.M4.T2 runUpgrade) calls before any
// on-disk change:
//   - confirmUpgrade: the FR-U9 current→target + action y/N confirmation. --yes (assumeYes) skips it.
//   - printDelegatedUpdate: the FR-U4 PRINT framing for a delegated updater channel (AUR/Nix) —
//     stagecoach prints the exact updater command for the user to run and never runs it itself.
//   - printPrivilegeCommand: the FR-U7 needs-privileges framing — the install path is not writable, so
//     stagecoach prints the exact sudo re-run command and NEVER auto-elevates (FR-U4).
//
// confirmUpgrade is STRICTER than integrate.DefaultConfirm: a non-TTY stdin without --yes is REFUSED
// with an error (caller⇒exit 1), not silently auto-declined — a declined file edit is reversible, but a
// botched binary swap is not (the contract's non-interactive-safety posture). A TTY decline ('n'/empty/
// anything-but-y) is an intentional abort ⇒ (false, nil) ⇒ exit 0.
//
// confirmUpgrade never prompts for --check or a pure PRINT — the caller simply does not call it on those
// paths (FR-U9). Every seam is injectable: stdin via io.Reader, stdout via io.Writer, and TTY-ness via
// the confirmUpgradeIsTTY package-var seam (mirrors interactiveStdinIsTTY). It returns a PLAIN error (no
// "stagecoach:" prefix — main.go adds it; no internal/exitcode import — the caller wraps). walled off
// (FR-U12: the commit path is untouched by this file).
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dabstractor/stagecoach/internal/ui"
)

// confirmUpgradeIsTTY is the TTY gate for confirmUpgrade. Overridable in tests so the non-TTY refusal
// and the TTY prompt paths are exercisable without a real terminal (mirrors interactiveStdinIsTTY in
// config_init_interactive.go). Production defaults to ui.IsTerminal(os.Stdin).
var confirmUpgradeIsTTY = func() bool { return ui.IsTerminal(os.Stdin) }

// confirmUpgrade implements the FR-U9 confirmation for `stagecoach upgrade`. It prints the current→target
// transition + the human-readable action, then a y/N prompt, accepting ONLY an explicit 'y'/'Y' as the
// first byte of the response. The four contract outcomes:
//   - assumeYes (--yes): (true, nil) with no prompt printed — the explicit script/CI bypass.
//   - non-TTY stdin + no --yes: (false, error) — refuse. The error has NO "stagecoach:" prefix (main.go
//     adds it) and points the user at --yes. The caller wraps exitcode.New(exitcode.Error, err) ⇒ exit 1.
//     No "Proceed?" prompt is printed (the refusal happens before prompting).
//   - TTY 'y'/'Y' (first byte): (true, nil) — proceed.
//   - TTY 'n'/empty/anything-else: (false, nil) — intentional abort (exit 0). Prints "stagecoach: upgrade
//     aborted".
//
// in/out are the stdin/stdout seams (testable with bytes.Buffer + strings.NewReader); the TTY-ness is
// read from the confirmUpgradeIsTTY package var. confirmUpgrade does NOT os.Exit and does NOT import
// internal/exitcode (it stays a pure, dependency-light prompt helper).
func confirmUpgrade(current, target, action string, assumeYes bool, in io.Reader, out io.Writer) (bool, error) {
	if assumeYes {
		return true, nil
	}
	if !confirmUpgradeIsTTY() {
		// Non-interactive + no --yes ⇒ refuse (never silently swap a binary in a piped/scripted run).
		// --yes is the explicit script bypass. Stricter than integrate.DefaultConfirm's auto-decline:
		// a declined file edit is reversible; a botched binary swap is not. NO "stagecoach:" prefix
		// (main.go adds it); the caller wraps via exitcode.New(exitcode.Error, err) ⇒ exit 1.
		return false, fmt.Errorf("non-interactive stdin — refusing to upgrade without confirmation; re-run with --yes to confirm non-interactively")
	}
	fmt.Fprintf(out, "stagecoach %s → %s\n%s\nProceed? [y/N] ", current, target, action)
	br := bufio.NewReader(in)
	line, _ := br.ReadString('\n') // EOF/empty ⇒ decline (best-effort; mirrors DefaultConfirm)
	line = strings.TrimSpace(line)
	if len(line) > 0 && (line[0] == 'y' || line[0] == 'Y') {
		return true, nil
	}
	fmt.Fprintln(out, "stagecoach: upgrade aborted")
	return false, nil
}

// printDelegatedUpdate frames the FR-U4 PRINT-channel output: stagecoach detected the install channel
// but will NOT run the updater itself (AUR needs root; Nix is declarative/immutable), so it prints the
// exact command for the user to run and exits 0 (the caller returns nil). command is the verbatim command
// internal/upgrade.Delegate produced (DelegateResult.Command); channel is the friendly channel label
// (e.g. "aur", "nix"). The command is echoed verbatim — it is never mutated/quoted/re-shelled (the user
// pastes it).
func printDelegatedUpdate(out io.Writer, channel, command string) {
	fmt.Fprintf(out, "stagecoach: installed via %s — run its updater yourself (stagecoach does not run it for you):\n  %s\n", channel, command)
}

// printPrivilegeCommand frames the FR-U7 needs-privileges output: the install path is not writable by the
// current user, so stagecoach left everything untouched and prints the exact elevated re-run command (it
// NEVER auto-sudo/auto-elevates — FR-U4). command is the verbatim command from
// internal/upgrade.NeedsPrivilegesError.Command. The caller returns nil (exit 0). The command is echoed
// verbatim — it is never mutated/quoted/re-shelled (the user pastes it).
func printPrivilegeCommand(out io.Writer, command string) {
	fmt.Fprintf(out, "stagecoach: install path not writable by the current user; re-run with privileges (stagecoach never auto-elevates):\n  %s\n", command)
}
