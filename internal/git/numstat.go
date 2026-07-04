// Package git provides shell-free access to the git binary.
//
// numstat capture + parse — the shared per-file size/shape primitive (PRD §9.1 FR3g/FR3i):
//
// numstatRows runs `git diff <diffArgs> --numstat` and parses each line into a numstatRow
// {Added, Deleted, IsBinary, Path}, keyed on the DESTINATION path (rename-resolved) and sorted by
// path for deterministic output. It is the dual-use input for the FR3g compact skeleton
// (P1.M3.T1.S2 renders it as the per-file skeleton line) and the FR3i water-fill sizing
// (P1.M4.T2 uses each row's add/delete magnitude as the file body-size estimate) — ONE git call
// serves both (the skeleton is both the model's completeness view AND the sizing input).
//
// This file is the GENERAL counterpart to binary.go's detectBinaryFiles (which keeps ONLY binary
// rows). numstatRows keeps ALL rows; it is NOT on the Git interface (internal *gitRunner helper,
// like detectBinaryFiles/fileStatuses) and is NOT routed through buildDiffArgs (which does not
// emit --numstat).
package git

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// numstatRow is one parsed line of `git diff --numstat`: the added/deleted line counts (0 for a
// binary file, whose numstat counts are "-"), whether git content-sniffed it as binary, and the
// destination (b/) path. For a rename the path is the NEW path (numstat's `old => new` notation
// resolved by resolveNumstatPath). Consumed by the FR3g compact skeleton (P1.M3.T1.S2) and the
// FR3i water-fill sizing (P1.M4.T2) — one git call, dual-use (the skeleton is both the model's
// completeness view and the sizing input). (PRD §9.1 FR3g/FR3i.)
type numstatRow struct {
	Added    int
	Deleted  int
	IsBinary bool
	Path     string // destination (b/) path, rename-resolved
}

// resolveNumstatPath resolves a `git diff --numstat` path field to the DESTINATION (b/) path. For
// a rename, numstat emits git's `=>` notation — and git ≥2.9 emits it EVEN WITHOUT -M
// (diff.renames defaulted on in the 2.8–2.9 era; verified on git 2.54.0). Two forms:
//   - simple:   `old => new`               → `new`
//   - brace:    `prefix{old => new}suffix` → `prefix` + `new` + `suffix`
//
// (git collapses the common prefix/suffix into the braces; e.g. `dir/{a.go => b.go}` →
// `dir/b.go`, and the suffix-collapsed `dir/{a => b}.go` → `dir/b.go`).
//
// A field with no `=>` is returned verbatim (a normal path, or a path that legitimately contains
// `{`/`}` but no rename). Pure function; no I/O.
func resolveNumstatPath(p string) string {
	if !strings.Contains(p, "=>") {
		return p
	}
	// Brace-collapsed form: prefix{old => new}suffix.
	if bi := strings.Index(p, "{"); bi >= 0 {
		if bj := strings.Index(p, "}"); bj > bi {
			return p[:bi] + rightOfArrow(p[bi+1:bj]) + p[bj+1:]
		}
	}
	// Simple form: old => new.
	return rightOfArrow(p)
}

// rightOfArrow returns the trimmed text after the first `=>` in s (the rename destination). Pure
// helper for resolveNumstatPath.
func rightOfArrow(s string) string {
	if i := strings.Index(s, "=>"); i >= 0 {
		return strings.TrimSpace(s[i+len("=>"):])
	}
	return s
}

// numstatRows runs `git diff <diffArgs> --numstat` and returns one numstatRow per changed file,
// keyed on the DESTINATION path (rename-resolved), sorted by path for deterministic output.
// diffArgs selects the diff domain and is forwarded verbatim — ["--cached"] (staged), [] (working
// tree), or [treeA, treeB] (tree-to-tree) — and may include caller-composed flags (-M, -U<n> via
// buildDiffArgs). `--numstat` is placed BEFORE any `--` pathspec separator in diffArgs so a caller
// that appends `-- <excludes>` (the FR3i sizing path, P1.M4.T2) composes correctly (a trailing
// --numstat would be swallowed as a pathspec).
//
// Binary rows (`added == "-"`, git emits "-\t-\t<path>") set IsBinary with Added=Deleted=0.
// Read-only w.r.t. refs/index. (PRD §9.1 FR3g/FR3i.)
//
// NOT routed through buildDiffArgs (which doesn't emit --numstat) and NOT on the Git interface —
// it is an internal *gitRunner helper, the general counterpart to binary.go's detectBinaryFiles
// (which keeps only binary rows; numstatRows keeps ALL rows for the skeleton/sizing).
func (g *gitRunner) numstatRows(ctx context.Context, diffArgs ...string) ([]numstatRow, error) {
	args := []string{"diff"}
	// Place --numstat before any `--` pathspec separator so excludes compose correctly.
	splitAt := len(diffArgs)
	for i, a := range diffArgs {
		if a == "--" {
			splitAt = i
			break
		}
	}
	args = append(args, diffArgs[:splitAt]...)
	args = append(args, "--numstat")
	args = append(args, diffArgs[splitAt:]...)

	stdout, stderr, code, err := g.run(ctx, g.workDir, args...)
	if err != nil {
		return nil, err // infrastructural (git missing / ctx cancel / start failure) — propagate unwrapped
	}
	if code != 0 {
		return nil, fmt.Errorf("git diff --numstat: failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	var rows []numstatRow
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 3 {
			continue // trailing "\n" → empty line; or malformed — skip (do NOT TrimSpace the line; preserve paths)
		}
		added, deleted, path := fields[0], fields[1], fields[2]
		row := numstatRow{Path: resolveNumstatPath(path)}
		if added == "-" { // git emits "-\t-\t<path>" for binary (both counts "-")
			row.IsBinary = true
		} else {
			row.Added, _ = strconv.Atoi(added)
			row.Deleted, _ = strconv.Atoi(deleted)
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Path < rows[j].Path })
	return rows, nil
}
