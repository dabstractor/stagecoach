// Package git provides shell-free access to the git binary.
//
// FR3g compact numstat skeleton render + capture (PRD §9.1 FR3g):
//
// renderNumstatSkeleton renders the parsed numstat rows from numstat.go (S1) as a compact
// one-line-per-file skeleton block; numstatSkeleton captures the rows for a diff domain and renders
// them. The skeleton is PREPENDED to every diff payload (StagedDiff/TreeDiff/WorkingTreeDiff) so the
// model sees the full shape of the change (every file, add/delete magnitude, binary-ness) even when
// bodies are truncated — the FR3g completeness floor. It is the dual-use consumer of S1's
// numstatRows (the model's completeness view); P1.M4.T2 is the other consumer (water-fill sizing).
// (PRD §9.1 FR3g.)
package git

import (
	"context"
	"fmt"
	"strings"
)

// numstatSkeletonHeader is the one-line label prepended to the skeleton rows so the model knows what
// the block is. The columns use REAL tabs to mirror the row format exactly (the header is a literal
// template for the rows). (PRD §9.1 FR3g.)
const numstatSkeletonHeader = "Change summary (numstat: added\tdeleted\tpath):"

// renderNumstatSkeleton renders the compact per-file skeleton block: the header, one line per row,
// then a trailing blank line that separates the skeleton from the placeholders/diff bodies. Binary
// rows (numstatRow.IsBinary) render as `-\t-\t<path>` — mirroring `git diff --numstat`'s literal
// output for binary files (numstatRow stores Added=Deleted=0 for binary; the skeleton shows the
// `-`/`-` form, NOT 0/0, so it faithfully represents the file as binary). Non-binary rows render
// `<added>\t<deleted>\t<path>`. Rows are already sorted by Path (numstatRows, S1) ⇒ deterministic
// output (do NOT re-sort here).
//
// Returns "" for an empty row set: an empty change set has nothing to summarize, and a header-only
// skeleton would defeat the caller's `diff == ""` nothing-staged check (PRD §9.4 FR5). Pure
// function; no I/O. (PRD §9.1 FR3g.)
func renderNumstatSkeleton(rows []numstatRow) string {
	if len(rows) == 0 {
		return "" // nothing changed → no skeleton (preserve the FR5 empty-payload check)
	}
	var b strings.Builder
	b.WriteString(numstatSkeletonHeader)
	b.WriteByte('\n')
	for _, r := range rows {
		if r.IsBinary {
			b.WriteString("-\t-\t")
			b.WriteString(r.Path)
			b.WriteByte('\n')
		} else {
			fmt.Fprintf(&b, "%d\t%d\t%s\n", r.Added, r.Deleted, r.Path)
		}
	}
	b.WriteByte('\n') // blank-line separator before the placeholders / diff bodies
	return b.String()
}

// numstatSkeleton captures the numstat rows for the given diff domain (via S1's numstatRows) and
// renders them as the compact skeleton block. diffArgs selects the domain and is forwarded verbatim
// — append "-M" (the caller does) so a rename is ONE row (matching the diff bodies' always-on -M,
// FR3e) rather than a delete+add pair; resolveNumstatPath (S1) resolves the `=>`/`{…}` rename
// notation. The caller ALSO appends the exclude pathspecs (`-- <defaultExcludes> <opts.Excludes>`)
// so the skeleton mirrors the SAME change set the model sees in the diff bodies — default-denylified
// noise (lockfiles, snapshots, sourcemaps, vendor/) is filtered upstream and stays out of the model's
// view (PRD §9.1 FR3i: "already excluded upstream by FR3's default denylist"). Binary files are NOT
// excluded (they have placeholders). Read-only w.r.t. refs/index. Returns "" when there are no
// changed files (renderNumstatSkeleton's empty rule). Each of StagedDiff/TreeDiff/WorkingTreeDiff
// calls this once and prepends the result. (PRD §9.1 FR3g.)
//
// NOT routed through buildDiffArgs (which emits a leading "diff" AND a -U<n> that is irrelevant to
// numstat); the caller passes the post-"diff" tokens (domain + "-M" + "--" + excludes). numstatRows
// builds its own "diff" argv and places --numstat before the caller's `--`.
func (g *gitRunner) numstatSkeleton(ctx context.Context, diffArgs ...string) (string, error) {
	rows, err := g.numstatRows(ctx, diffArgs...)
	if err != nil {
		return "", err
	}
	return renderNumstatSkeleton(rows), nil
}
