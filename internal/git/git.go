package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// FileChange is one entry in a diff-tree "what landed" listing.
// diff-tree --name-status -r emits "<status>\t<path>" or "<status>\t<src>\t<dst>" (rename/copy).
// The S6 (DiffTree) implementation parses these lines into FileChange values.
type FileChange struct {
	Status  string // "A","M","D","R","C","T","U"; R/C carry a similarity score e.g. "R100"
	SrcPath string // non-empty only for R/C (the rename/copy source); "" otherwise
	Path    string // the destination path — always set
}

// StagedDiffOptions configures staged-diff capture (commit-pi parity, PRD §9.1 / FINDING 7).
// The T3.S1 (StagedDiff) implementation consumes these.
type StagedDiffOptions struct {
	MaxDiffBytes int      // byte cap on the non-markdown section (commit-pi default 300000); 0 = unlimited
	MaxMDLines   int      // per-file line cap for markdown files (commit-pi default 100); 0 = unlimited
	Excludes     []string // pathspec magic-prefix excludes, e.g. []string{":!*.lock", ":!vendor/*"}
}

// Git is the shell-free boundary to the real git binary. Every method delegates to the private
// run() helper on *gitRunner, which execs git with args as []string (NEVER sh -c — PRD §19) and
// targets the repo via the -C flag (NEVER os.Chdir — goroutine-safe).
//
// Method ownership (each implemented in its own later subtask):
//
//	RevParseHEAD      — P1.M1.T2.S2   WriteTree        — P1.M1.T2.S3
//	CommitTree        — P1.M1.T2.S4   UpdateRefCAS     — P1.M1.T2.S5
//	DiffTree          — P1.M1.T2.S6
//	StagedDiff        — P1.M1.T3.S1   HasStagedChanges — P1.M1.T3.S2
//	RecentMessages    — P1.M1.T3.S3   CommitCount      — P1.M1.T3.S3
//	RecentSubjects    — P1.M1.T3.S4   AddAll           — P1.M1.T3.S5
type Git interface {
	// RevParseHEAD returns the SHA HEAD points at. On a repo with zero commits it returns
	// sha="" and isUnborn=true (detected via git exit 128, NOT stdout emptiness — FINDING 1).
	RevParseHEAD(ctx context.Context) (sha string, isUnborn bool, err error)

	// WriteTree materializes the index into a tree object and returns its SHA. Fails (non-nil err)
	// when the index has unresolved merge conflicts (git exit 128).
	WriteTree(ctx context.Context) (sha string, err error)

	// CommitTree creates a commit object for tree with the given parents and message (delivered
	// via stdin with -F -). parents==nil/empty ⇒ root commit (no -p). Returns the new commit SHA.
	CommitTree(ctx context.Context, tree string, parents []string, msg string) (sha string, err error)

	// UpdateRefCAS atomically moves ref to newSHA only if it currently equals expectedOld
	// (3-arg compare-and-swap; NEVER the 2-arg force form). For a root commit pass expectedOld =
	// the all-zeros hash. Returns a non-nil err on CAS mismatch (HEAD moved).
	UpdateRefCAS(ctx context.Context, ref, newSHA, expectedOld string) error

	// DiffTree returns the file-level change set of sha vs its first parent ("what landed").
	// isRoot must be true for a root commit so git diffs against the empty tree (--root flag).
	DiffTree(ctx context.Context, sha string, isRoot bool) ([]FileChange, error)

	// StagedDiff returns the staged diff payload (markdown per-file + non-markdown aggregate),
	// applying byte/line caps and pathspec excludes per opts (commit-pi parity, PRD §9.1).
	StagedDiff(ctx context.Context, opts StagedDiffOptions) (diff string, err error)

	// HasStagedChanges reports whether the index differs from HEAD (git diff --cached --quiet:
	// exit 1 ⇒ true, exit 0 ⇒ false). NOT an error when changes exist (FINDING 6).
	HasStagedChanges(ctx context.Context) (bool, error)

	// RecentMessages returns up to n most-recent full commit messages (NUL-delimited query,
	// FINDING 9). Callers must short-circuit when RevParseHEAD reports isUnborn.
	RecentMessages(ctx context.Context, n int) (messages []string, err error)

	// RecentSubjects returns up to n most-recent commit subjects (first line) for duplicate
	// detection. Callers must short-circuit when isUnborn.
	RecentSubjects(ctx context.Context, n int) (subjects []string, err error)

	// CommitCount returns the number of commits reachable from HEAD (decides mature vs new-repo
	// prompt). Callers must short-circuit when isUnborn.
	CommitCount(ctx context.Context) (count int, err error)

	// AddAll stages all changes (git add -A). Used by the auto-stage-all path (PRD §9.4 / FINDING 11).
	AddAll(ctx context.Context) error
}

// gitRunner is the production Git implementation. It wraps exec.CommandContext for the real git
// binary. Construct with New.
type gitRunner struct {
	workDir string // the repo path passed as -C <repo> by every bound method
}

// New returns a Git bound to workDir. The git binary is resolved lazily inside run() (New has no
// error return); a missing git surfaces as a runtime error from the first run() call.
func New(workDir string) Git {
	return &gitRunner{workDir: workDir}
}

// run is the low-level git exec helper. It is the ONLY place Stagehand shells out to git.
//   - resolves the git binary via exec.LookPath (PRD §19: real binary, never go-git per §22.3)
//   - targets repo via the -C flag (NOT os.Chdir / cmd.Dir — goroutine-safe)
//   - captures stdout and stderr to SEPARATE buffers
//   - returns the exit code extracted from *exec.ExitError
//
// INVARIANT: a NON-ZERO git exit is returned as (stdout, stderr, exitCode, nil) — err is nil.
// Git uses exit codes as semantic signals (1 = has-staged; 128 = unborn/not-a-SHA), and callers
// inspect exitCode. Only infrastructural failures (LookPath miss, context cancel, start/I/O)
// return err != nil, with exitCode = -1.
func (g *gitRunner) run(ctx context.Context, repo string, args ...string) (stdout string, stderr string, exitCode int, err error) {
	gitPath, lerr := exec.LookPath("git")
	if lerr != nil {
		return "", "", -1, fmt.Errorf("git binary not found in PATH: %w", lerr)
	}

	full := make([]string, 0, len(args)+2)
	full = append(full, "-C", repo) // repo via flag, not cmd.Dir (gotcha G1)
	full = append(full, args...)

	cmd := exec.CommandContext(ctx, gitPath, full...) // []string args, NO shell (PRD §19)
	var out, errb bytes.Buffer
	cmd.Stdout = &out  // separate buffer
	cmd.Stderr = &errb // separate buffer

	runErr := cmd.Run()
	stdout, stderr = out.String(), errb.String()

	if runErr == nil {
		return stdout, stderr, 0, nil
	}
	if cerr := ctx.Err(); cerr != nil { // context cancelled (timeout/signal) — not a git exit
		return stdout, stderr, -1, cerr
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) { // non-zero git exit → capture code, err stays nil (gotcha G2)
		return stdout, stderr, exitErr.ExitCode(), nil
	}
	return stdout, stderr, -1, runErr // start / I/O failure
}

// ---- Stubs: each method is implemented in its own later subtask. They panic to fail fast. ----

// RevParseHEAD returns the SHA HEAD currently points at. On a repository with zero commits it
// returns sha="" and isUnborn=true, detected via git's exit code 128 (NOT stdout emptiness —
// `git rev-parse HEAD` prints the literal string "HEAD\n" to stdout on an unborn repo, which is
// the latent bug in commit-pi; see critical_findings.md FINDING 1).
func (g *gitRunner) RevParseHEAD(ctx context.Context) (sha string, isUnborn bool, err error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "rev-parse", "HEAD")
	if err != nil {
		return "", false, err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code == 128 {
		return "", true, nil // unborn repo — exit-code signal, NOT string emptiness
	}
	if code != 0 {
		return "", false, fmt.Errorf("git rev-parse HEAD: unexpected exit %d: %s", code, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), false, nil
}

// WriteTree materializes the current index into a tree object and returns its SHA. It is a
// read-only-with-respect-to-refs operation: it writes a tree object to the object store but does
// NOT modify the index or HEAD (PRD §13.2). It is the immutable-snapshot primitive consumed by
// CommitTree (P1.M1.T2.S4) and the rescue protocol (P1.M3.T3).
//
// write-tree fails (non-zero exit, 128 on git 2.x) when the index has unresolved merge conflicts
// (unmerged stage 1/2/3 entries). That is surfaced here as run()'s exitCode != 0 (err stays nil per
// run()'s invariant); the error names "unresolved merge conflicts" and includes the trimmed stderr,
// whose text contains "unmerged"/"error building trees" on a real conflict (git_plumbing_reference
// §1: the stable signal is exit ≠ 0; do NOT match a single exact stderr phrase).
func (g *gitRunner) WriteTree(ctx context.Context) (sha string, err error) {
	stdout, stderr, code, err := g.run(ctx, g.workDir, "write-tree")
	if err != nil {
		return "", err // git binary missing / context cancelled / start failure (run sets code=-1)
	}
	if code != 0 {
		return "", fmt.Errorf("git write-tree: unresolved merge conflicts in index (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}

func (g *gitRunner) CommitTree(ctx context.Context, tree string, parents []string, msg string) (string, error) {
	panic("gitRunner.CommitTree: not yet implemented — see P1.M1.T2.S4")
}

func (g *gitRunner) UpdateRefCAS(ctx context.Context, ref, newSHA, expectedOld string) error {
	panic("gitRunner.UpdateRefCAS: not yet implemented — see P1.M1.T2.S5")
}

func (g *gitRunner) DiffTree(ctx context.Context, sha string, isRoot bool) ([]FileChange, error) {
	panic("gitRunner.DiffTree: not yet implemented — see P1.M1.T2.S6")
}

func (g *gitRunner) StagedDiff(ctx context.Context, opts StagedDiffOptions) (string, error) {
	panic("gitRunner.StagedDiff: not yet implemented — see P1.M1.T3.S1")
}

func (g *gitRunner) HasStagedChanges(ctx context.Context) (bool, error) {
	panic("gitRunner.HasStagedChanges: not yet implemented — see P1.M1.T3.S2")
}

func (g *gitRunner) RecentMessages(ctx context.Context, n int) ([]string, error) {
	panic("gitRunner.RecentMessages: not yet implemented — see P1.M1.T3.S3")
}

func (g *gitRunner) RecentSubjects(ctx context.Context, n int) ([]string, error) {
	panic("gitRunner.RecentSubjects: not yet implemented — see P1.M1.T3.S4")
}

func (g *gitRunner) CommitCount(ctx context.Context) (int, error) {
	panic("gitRunner.CommitCount: not yet implemented — see P1.M1.T3.S3")
}

func (g *gitRunner) AddAll(ctx context.Context) error {
	panic("gitRunner.AddAll: not yet implemented — see P1.M1.T3.S5")
}
