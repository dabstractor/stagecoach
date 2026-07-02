// This file adds stagehand's single user-facing output abstraction: a
// TTY/color-aware Output writer that enforces the FR51 stream discipline.
// Progress and verbose diagnostics ALWAYS go to stderr, and only the commit
// result (subject + diff-tree) goes to stdout, so stdout stays byte-clean for
// piping (`stagehand --dry-run --no-color | tee /tmp/msg.txt`).
//
// Color is auto-enabled when stdout is a TTY and disabled by NO_COLOR (set to
// a non-empty value) or the --no-color flag (folded in by the CLI layer,
// M7.T2). TTY detection uses the stdlib os.ModeCharDevice char-device check —
// a pipe is not a char device, so color auto-disables under piping. No
// external color/term library is used; styling is raw SGR escape sequences.
//
// The package-doc comment lives in exitcode.go; this file is plain `package ui`
// to avoid a duplicate-package-comment lint error.
package ui

import (
	"fmt"
	"io"
	"os"
)

// Raw SGR escape sequences (ANSI CSI). Used directly by Color and its thin
// wrappers; never emitted unless Output.color is true. See
// https://en.wikipedia.org/wiki/ANSI_escape_code#SGR (reset=0, bold=1,
// red=31, green=32, yellow=33, cyan=36).
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiCyan   = "\x1b[36m"
)

// Output is stagehand's single user-facing output sink. It routes the three
// families of process output (progress, result, verbose) to the correct
// stream so the FR51 stdout-pipe-clean invariant is structural rather than
// convention-dependent: a caller physically cannot leak a progress line onto
// stdout because every method targets a fixed writer.
//
// All fields are unexported; callers construct via NewOutput (production,
// TTY/env aware) or, in tests, via a white-box struct literal.
type Output struct {
	stdout  io.Writer
	stderr  io.Writer
	color   bool
	verbose bool
}

// NewOutput constructs an Output wired to the given stdout/stderr writers,
// resolving color from the TTY-ness of stdout, the NO_COLOR environment
// variable (read here, not the STAGEHAND_* CLI-layer flag), and the
// already-resolved noColor flag. The CLI layer (cmd/stagehand, M7.T2) folds
// --no-color and STAGEHAND_NO_COLOR into the noColor bool, keeping cobra/pflag
// out of internal/ui. verbose gates Verbosef.
func NewOutput(stdout, stderr io.Writer, verbose, noColor bool) *Output {
	return &Output{
		stdout:  stdout,
		stderr:  stderr,
		color:   shouldColor(isTerminal(stdout), os.Getenv("NO_COLOR"), noColor),
		verbose: verbose,
	}
}

// shouldColor is the pure color-resolution predicate. Color is enabled iff
// stdout is a TTY, NO_COLOR is empty (per https://no-color.org an empty value
// still allows color; any non-empty value such as "1" disables it), and the
// --no-color flag is off. It is kept pure so tests can exhaustively cover all
// corners without faking a TTY.
func shouldColor(isTTY bool, noColorEnv string, noColorFlag bool) bool {
	return isTTY && noColorEnv == "" && !noColorFlag
}

// isTerminal reports whether w is a terminal by type-asserting it to *os.File
// and checking the char-device mode bit. A non-*os.File writer (bytes.Buffer,
// or any test-injected writer) returns false; a pipe (the receiving end of
// `| tee`) is not a char device and so also returns false, which is exactly
// why color auto-disables under piping. The stat error path returns false so
// a closed/invalid fd never accidentally enables color.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// Progressf writes a progress message to stderr, ALWAYS — it is never gated
// by verbose and never touches stdout. This is the sink for the FR18
// auto-stage notice ("Nothing staged — staging all changes...") and every
// other human-facing progress line. Routing it to stderr is what keeps
// stdout byte-clean for `stagehand --dry-run | tee` (FR51): if progress
// landed on stdout, the piped message file would be corrupted. The format
// string owns its trailing newline; no newline is forced here, so diff-tree
// style multi-line payloads compose cleanly. Returns the underlying fmt
// error so callers/tests can observe write failures.
func (o *Output) Progressf(format string, args ...any) error {
	_, err := fmt.Fprintf(o.stderr, format, args...)
	return err
}

// Resultf writes the success result payload to stdout: the FR42 commit
// summary line (`[sha] subject`) followed by the diff-tree lines. This is the
// ONLY method that writes to stdout, which is what makes stdout pipe-clean.
// The format string owns its trailing newline; no newline is forced here so
// callers compose multi-line diff-tree output deterministically. Returns the
// underlying fmt error so callers/tests can observe write failures.
func (o *Output) Resultf(format string, args ...any) error {
	_, err := fmt.Fprintf(o.stdout, format, args...)
	return err
}

// Verbosef writes a verbose diagnostic to stderr ONLY when verbose is
// enabled, the FR50 sink for the resolved command, raw agent stdout, and
// retry traces. When verbose is false it short-circuits and returns nil
// BEFORE formatting, so no work is done on the hot path. The format string
// owns its trailing newline; no newline is forced here. Returns the
// underlying fmt error when verbose, else nil.
func (o *Output) Verbosef(format string, args ...any) error {
	if !o.verbose {
		return nil
	}
	_, err := fmt.Fprintf(o.stderr, format, args...)
	return err
}

// Color wraps s with the given ANSI SGR sequence and a trailing reset when
// color is enabled, returning s unchanged otherwise. Stream routing and
// styling are intentionally orthogonal: the routing methods do NOT
// auto-colorize; callers wrap tokens via Color (or a thin wrapper) so a
// caller controls exactly what gets styled.
func (o *Output) Color(ansi, s string) string {
	if !o.color {
		return s
	}
	return ansi + s + ansiReset
}

// Green wraps s in the green SGR sequence when color is enabled.
func (o *Output) Green(s string) string { return o.Color(ansiGreen, s) }

// Yellow wraps s in the yellow SGR sequence when color is enabled.
func (o *Output) Yellow(s string) string { return o.Color(ansiYellow, s) }

// Red wraps s in the red SGR sequence when color is enabled.
func (o *Output) Red(s string) string { return o.Color(ansiRed, s) }

// Bold wraps s in the bold SGR sequence when color is enabled.
func (o *Output) Bold(s string) string { return o.Color(ansiBold, s) }

// Cyan wraps s in the cyan SGR sequence when color is enabled.
func (o *Output) Cyan(s string) string { return o.Color(ansiCyan, s) }
