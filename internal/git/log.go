// This file adds the three history-query primitives that feed the prompt
// example block (P1.M4.T1.S1), the multi-line detection input
// (reference_impl.md §6), and the duplicate-subject set (P1.M6.T1.S2):
// CommitCount, RecentMessages, and RecentSubjects. They are thin methods over
// the shipped, unexported [Git.run] exec seam against the REAL git binary
// (PRD §22.3: no go-git; §19: no sh -c — the --format=---%n%B / --format=%s
// tokens and the -<n> limit are passed as literal args, never interpolated
// into a shell string). Each method builds a []string of args, calls g.run,
// and on a non-zero exit uses errors.As into the typed *[ExitError] to route
// the unborn/rootless-repo case into a non-error empty result (the normal
// root-commit case, NOT a failure). It uses a plain "package git" line because
// [git.go] OWNS the package doc comment, mirroring how plumbing.go/diff.go
// defer to git.go.
package git

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// CommitCount returns the number of commits reachable from HEAD — the
// history signal that gates whether the generate step builds example-backed
// prompts (PRD §22.3: shell out to the real git binary, no go-git; §19: no
// sh -c; reference_impl.md §1: `commit_count = git rev-list --count HEAD || 0`;
// external_deps.md §D: the verified git 2.54 command table).
//
// It runs `git rev-list --count HEAD` (Args are built as a []string and passed
// directly to exec.Command, NEVER via sh — PRD §19). An UNBORN / rootless
// repo (no commits yet) makes git exit 128 with stderr "ambiguous argument
// 'HEAD': unknown revision ..." — the IDENTICAL stderr rev-parse HEAD emits —
// so the unborn case is detected exactly like [Git.RevParseHEAD] and swallowed
// into (0, nil). That is the normal root-commit case (FR39), NOT a failure,
// faithfully porting the reference's `|| 0`. Any OTHER non-zero exit (e.g.
// "not a git repository") is a genuine failure surfaced as the typed
// *[ExitError] so the caller can route it.
//
// On success the raw stdout (e.g. "2\n") is strconv.Atoi'd after a TrimSpace.
func (g *Git) CommitCount() (int, error) {
	out, runErr := g.run("rev-list", "--count", "HEAD")
	if runErr != nil {
		// An UNBORN repo makes rev-list --count HEAD exit 128 with the same
		// "unknown revision" stderr as rev-parse HEAD (verified git 2.54.0).
		// That is the root-commit case (FR39), returned as (0, nil) — NOT a
		// failure — mirroring RevParseHEAD's detection verbatim. Any other
		// non-zero exit (e.g. not-a-repo) is a real error surfaced as-is.
		var ee *ExitError
		if errors.As(runErr, &ee) && ee.Code == 128 && strings.Contains(ee.Stderr, "unknown revision") {
			return 0, nil
		}
		return 0, runErr
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("git rev-list --count HEAD: parsing %q: %w", out, err)
	}
	return n, nil
}

// RecentMessages returns the raw `---`-separated %B bodies of the last n
// commits, newest-first — the example block for prompt assembly AND the input
// to the multi-line detection heuristic (PRD §22.3: shell out to the real git
// binary, no go-git; §19: no sh -c; reference_impl.md §1:
// `git log --format='---%n%B' -20`; reference_impl.md §6: the multi-line awk
// scan splits this stream on the `---` separators and flags any group with
// >1 non-empty line as multiline; external_deps.md §D: the verified git 2.54
// command table).
//
// It runs `git log --format=---%n%B -<n>`. The `--format=---%n%B` value is
// passed as ONE literal arg — `%n` is a newline, `%B` is the raw body (subject
// + body); `%` is not special in a Go string, and this arg form is verified
// byte-identical to the shell-quoted `'---%n%B'` form. The limit is the
// dash-number form `-<n>` (matching the reference `-20`), built as
// fmt.Sprintf("-%d", n) and passed as a literal arg. n <= 0 yields empty
// stdout (exit 0) naturally; no special guard is needed.
//
// The output is returned RAW and VERBATIM — the trailing newline and the
// embedded blank lines within multi-line bodies are INTACT. Do NOT trim, cap,
// or strip the `---` separators HERE: the prompt layer runs
// `sed '/^$/d' | head -100` and the multi-line awk scan, and trimming here
// would corrupt that heuristic (reference_impl.md §6).
//
// An UNBORN / rootless repo (no commits yet) makes git log exit 128 with
// stderr "... does not have any commits yet" — a DIFFERENT message than
// rev-list's "unknown revision". That is the normal root-commit case (FR39),
// returned as ("", nil) — NOT a failure (matched on the stable "does not have
// any commits yet" substring, NOT the branch name which varies main/master).
// Any OTHER non-zero exit is surfaced as the typed *[ExitError].
func (g *Git) RecentMessages(n int) (string, error) {
	out, runErr := g.run("log", "--format=---%n%B", fmt.Sprintf("-%d", n))
	if runErr != nil {
		// An UNBORN repo makes git log exit 128 with "... does not have any
		// commits yet" (a DIFFERENT message than rev-list's "unknown
		// revision"). That is the root-commit case (FR39), returned as ("",
		// nil) — NOT an error — matching the stable substring, NOT the branch
		// name which varies main/master/etc. Any other non-zero exit is a real
		// error surfaced as-is.
		var ee *ExitError
		if errors.As(runErr, &ee) && ee.Code == 128 && strings.Contains(ee.Stderr, "does not have any commits yet") {
			return "", nil
		}
		return "", runErr
	}
	return out, nil // RAW — no trim, no line cap, no '---' stripping (prompt layer trims+caps, reference_impl.md §6)
}

// RecentSubjects returns the trimmed subjects of the last n commits,
// newest-first — the duplicate-subject set the generate step checks against to
// never reuse an existing subject (PRD §22.3: shell out to the real git
// binary, no go-git; §19: no sh -c; reference_impl.md §1:
// `dedupe subject in git log --format=%s -50`; external_deps.md §D: the
// verified git 2.54 command table).
//
// It runs `git log --format=%s -<n>` (the `%s` subject token and the `-<n>`
// limit are each passed as a literal arg — PRD §19). The raw stdout (one
// subject per line, newest-first, with a trailing newline) is split on "\n",
// each element TrimSpace'd, and the empty elements dropped (the trailing
// newline yields a final empty element the skip-empties step removes). n
// larger than the commit count returns ALL subjects (git log exits 0); n <= 0
// yields empty.
//
// An UNBORN / rootless repo (no commits yet) makes git log exit 128 with
// stderr "... does not have any commits yet" — the normal root-commit case
// (FR39), returned as (nil, nil) — NOT a failure. Any OTHER non-zero exit is
// surfaced as the typed *[ExitError].
func (g *Git) RecentSubjects(n int) ([]string, error) {
	out, runErr := g.run("log", "--format=%s", fmt.Sprintf("-%d", n))
	if runErr != nil {
		// An UNBORN repo makes git log exit 128 with "... does not have any
		// commits yet" — the root-commit case (FR39), returned as (nil, nil) —
		// NOT an error. Any other non-zero exit is a real error surfaced as-is.
		var ee *ExitError
		if errors.As(runErr, &ee) && ee.Code == 128 && strings.Contains(ee.Stderr, "does not have any commits yet") {
			return nil, nil
		}
		return nil, runErr
	}
	var subjects []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			subjects = append(subjects, line)
		}
	}
	return subjects, nil // newest-first in order (git log emits newest-first)
}
