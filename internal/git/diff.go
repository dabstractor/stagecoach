// This file adds the staged-diff capture primitive (P1.M3.T3.S1) that feeds
// the generate commit step (P1.M6.T1.S1) and prompt.AssemblePayload
// (P1.M4.T1.S3). It is a thin method over the shipped, unexported [Git.run]
// exec seam against the REAL git binary (PRD §22.3: shell out to real git, no
// go-git; §19: no sh -c — the pathspec tokens like ":!*.lock" are passed as
// literal args, never interpolated into a shell string). It reproduces the
// proven commit-pi md-vs-other capture (reference_impl.md §1): markdown files
// are captured PER-FILE via `git diff --cached -- <file>` capped at
// MaxMdLines lines (head -n semantics), and everything-else is captured in
// ONE `git diff --cached -- <contract pathspec exclusions>` invocation capped
// at MaxDiffBytes TOTAL bytes (head -c semantics), the two concatenated
// markdown-first. It uses a plain "package git" line because [git.go] OWNS
// the package doc comment, mirroring how internal/ui/exitcode.go defers to
// internal/ui/ui.go.
package git

import "strings"

// DiffSettings is the small configuration the staged-diff contract names
// (plan_overview.md §2.2: pre-config layers take scalar settings / a small
// prompt.Settings rather than importing the not-yet-existing M5 config). It
// lives in package git — NOT in an M5 config package — so M3 stays
// self-contained and testable without M5; field names are ALIGNED with the
// future M5 config.Config fields so the generate/CLI layer bridges
// config.Config → git.DiffSettings field-for-field with no rename
// (decisions.md §9 porting map).
//
// FR1-FR5 govern the staged-diff capture, and decisions.md §6 pins the
// defaults:
//   - MaxMdLines (default 100): the PER-FILE head -n line cap applied to each
//     markdown file's `git diff --cached -- <file>` output before
//     concatenation (one giant doc cannot drown the context).
//   - MaxDiffBytes (default 300000): the TOTAL head -c byte budget applied to
//     the single other-diff command across the whole non-markdown change set.
//
// [StagedDiff] clamps non-positive fields to these defaults on its BY-VALUE
// copy, so a zero-value DiffSettings{} is safe (a 0 cap would otherwise
// truncate everything).
type DiffSettings struct {
	// MaxMdLines is the PER-FILE head -n line cap for each markdown file's
	// staged diff (default 100, decisions.md §6). It is PER-FILE then
	// concatenated, NOT a total cap across all markdown files.
	MaxMdLines int

	// MaxDiffBytes is the TOTAL head -c byte cap on the single other-diff
	// `git diff --cached -- <exclusions>` command (default 300000, decisions.md
	// §6). It applies to the whole non-markdown change set, not per-file.
	MaxDiffBytes int
}

// capLines returns the prefix of s ending with the Nth newline (inclusive of
// that newline) — the head -n N equivalent. If n <= 0 it returns "" (nothing);
// if s has fewer than n newlines it returns the whole s, matching head -n
// (including a final partial line). It iterates BYTES counting '\n' and does
// NOT pre-trim s: head operates on the raw stream.
func capLines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	lines := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines++
			if lines == n {
				return s[:i+1] // include the Nth newline
			}
		}
	}
	return s // fewer than n newlines: whole string (matches head -n)
}

// capBytes returns the first n bytes of s — the head -c N equivalent. If
// n <= 0 it returns "" (nothing); if len(s) <= n it returns the whole s. It
// cuts at a byte boundary and may split a UTF-8 rune, which is acceptable and
// matches head -c. It does NOT pre-trim s: head operates on the raw stream.
func capBytes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) > n {
		return s[:n]
	}
	return s
}

// StagedDiff captures the staged (index vs HEAD) change set as a single diff
// payload string, faithfully porting the proven commit-pi md-vs-other
// capture (PRD §22.3: shell out to the real git binary, no go-git; §19: no
// sh -c; reference_impl.md §1: the exact capture pipeline; external_deps.md
// §D: the verified git 2.54 pathspec; decisions.md §9: the porting map pins
// this as the md + other diff capture with per-file md 100-line + total
// 300KB caps and identical exclusions).
//
// FR1-FR5 govern the behavior:
//   - FR1: markdown files are listed via
//     `git diff --cached --name-only -- '*.md' '*.markdown'` (the *.md /
//     *.markdown globs are literal args, no shell).
//   - FR2: each markdown file's diff is captured PER-FILE via
//     `git diff --cached -- <file>` and head -n capped to cfg.MaxMdLines
//     (PER-FILE, then concatenated — NOT a total line cap), so one giant doc
//     cannot drown the context.
//   - FR3: everything-else is captured in ONE
//     `git diff --cached -- <contract pathspec exclusions>` invocation and
//     head -c capped to cfg.MaxDiffBytes TOTAL bytes (the byte budget covers
//     the whole other-diff command, not per-file).
//   - FR4: the two parts are concatenated markdown-first
//     (markdown_diff + other_diff, NO separator), matching reference_impl.md
//     §1.
//   - FR5: when nothing is staged it returns ("", nil) — NOT an error. The
//     CLI/generate layer decides auto-stage/exit on an empty diff; StagedDiff
//     itself never errors on an empty index (git diff --cached exits 0 with
//     empty stdout when nothing is staged).
//
// cfg is passed BY VALUE and its non-positive fields are clamped to the
// contract defaults (MaxMdLines→100, MaxDiffBytes→300000) on this local copy,
// so a zero-value DiffSettings{} is safe.
//
// On any non-zero git exit (e.g. not a git repository) the typed *[ExitError]
// is surfaced as-is; markdown and other captures each run their own g.run and
// the first failure short-circuits.
func (g *Git) StagedDiff(cfg DiffSettings) (string, error) {
	// Clamp non-positive fields to the contract defaults on this BY-VALUE copy
	// (decisions.md §6). Prevents a zero-value footgun where a 0 cap would
	// truncate everything.
	if cfg.MaxMdLines <= 0 {
		cfg.MaxMdLines = 100
	}
	if cfg.MaxDiffBytes <= 0 {
		cfg.MaxDiffBytes = 300000
	}

	// FR1: list the staged markdown files. *.md / *.markdown are literal args
	// (no shell — PRD §19). mdOut is the RAW stdout (one path per line).
	mdOut, err := g.run("diff", "--cached", "--name-only", "--", "*.md", "*.markdown")
	if err != nil {
		return "", err
	}

	// FR2: capture each markdown file's diff PER-FILE and head -n cap it
	// (PER-FILE, then concatenated). Parse mdOut by splitting on "\n"; skip
	// empty lines (the trailing empty from the final newline); strip ONLY "\r"
	// defensively — do NOT TrimSpace a filename, it would corrupt names with
	// leading/trailing spaces.
	var md strings.Builder
	for _, line := range strings.Split(mdOut, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		d, err := g.run("diff", "--cached", "--", line)
		if err != nil {
			return "", err
		}
		md.WriteString(capLines(d, cfg.MaxMdLines))
	}

	// FR3: capture everything-else in ONE command, head -c capped TOTAL. Pass
	// the EXACT contract pathspec tokens verbatim (external_deps.md §D
	// verified working for .lock/.snap/.map/vendor). The literal no-star
	// :!.md / :!.markdown are FAITHFUL to reference_impl.md §1 — they do NOT
	// exclude *.md by design (a pathspec without a wildcard is a literal/
	// prefix match), so markdown intentionally also appears in other_diff;
	// do NOT change them to :!*.md.
	otherOut, err := g.run("diff", "--cached", "--",
		":!*.lock", ":!package-lock.json", ":!pnpm-lock.yaml", ":!yarn.lock",
		":!*.snap", ":!*.map", ":!vendor/*", ":!.md", ":!.markdown")
	if err != nil {
		return "", err
	}

	// FR4: concatenate markdown-first, NO separator (the per-file and other
	// diffs already end in "\n" from git, matching reference_impl.md §1).
	result := md.String() + capBytes(otherOut, cfg.MaxDiffBytes)

	// FR5: nothing staged is NOT an error — return ("", nil). The CLI/generate
	// layer decides auto-stage/exit on an empty diff.
	if result == "" {
		return "", nil
	}
	return result, nil
}
