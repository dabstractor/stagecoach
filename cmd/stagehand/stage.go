// This file owns the SOLE staging POLICY entry-point the CLI default action
// calls: maybeAutoStage (FR16–FR20). P1.M7.T2.S1 shipped the default action
// with a close-but-divergent int-returning maybeAutoStagePolicy inlined into
// run.go (both staging outcomes collapsed to one exit code and the FR18 notice
// lacked the file count); this file is S2's contract-faithful reconciliation:
//   - maybeAutoStage RETURNS error (nil=proceed), so the two staging outcomes
//     can carry DISTINCT sentinels — ErrNothingStaged (FR19: the user declined
//     auto-staging) vs ErrNothingToCommit (FR17: clean tree even after add) —
//     even though both map to exit 2.
//   - the FR18 notice now carries the "(N files)" count via the new
//     (*Git).StagedFileCount primitive.
//
// run.go's call site is now `if err := maybeAutoStage(...); err != nil {
// return mapErrorToExitCode(err) }`, and mapErrorToExitCode gained one branch
// (ErrNothingStaged → ui.ExitNothingToCommit). The old maybeAutoStagePolicy and
// its stager interface were DELETED from run.go (moved here, extended).
//
// Staging POLICY lives HERE and ONLY here (decisions.md §1: the v2 seam —
// CommitStaged/generate assume the index is already staged; staging decisions
// are a CLI concern). v2 will swap this whole function for a partitioned-
// staging loop (plan_overview key decision 6), which is exactly why it must be
// ONE named, contract-faithful function. The minimal `stager` interface lets
// *git.Git satisfy it in production AND an in-process stub satisfy it in the
// stage_test.go MOCKING tests, keeping the FR16–FR20 logic hermetic.
//
// This file is a plain `package main` sibling of main.go (which OWNS the
// // Package doc comment), mirroring how run.go/providers.go/config.go defer
// the package doc to main.go.
package main

import (
	"errors"
	"fmt"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/ui"
	"github.com/dustin/stagehand/pkg/stagehand"
)

// ErrNothingStaged is the CLI-layer sentinel for FR19: nothing is staged AND
// the user declined auto-staging — either via --no-auto-stage OR because
// config auto_stage_all=false. In that case the index is NEVER touched (AddAll
// is not called) and maybeAutoStage returns this error so runDefault maps it to
// exit 2.
//
// It is DISTINCT from stagehand.ErrNothingToCommit (FR17: the index was
// auto-staged but the worktree is STILL clean — there is genuinely nothing to
// commit). Both sentinels map to ui.ExitNothingToCommit (2), so the PRD §15.4
// exit-code table is UNCHANGED; the distinction is the SENTINEL, letting an
// integrator/test branch via errors.Is even though the exit code is the same.
// ErrNothingToCommit lives in pkg/stagehand (a generate-layer concept: the
// diff=="" gate); ErrNothingStaged is a CLI-layer concept (the user's
// auto-stage DECLINE), which is why it is defined HERE rather than in
// pkg/stagehand or internal/generate.
var ErrNothingStaged = errors.New("nothing staged (auto-stage declined)")

// stager is the minimal staging-primitive surface maybeAutoStage needs
// (HasStagedChanges + AddAll + StagedFileCount, all shipped on *git.Git in
// internal/git/stage.go). Defining it as an interface — rather than taking
// *git.Git directly — lets the staging-POLICY tests run hermetically against an
// in-process stub instead of a real git binary, while *git.Git still satisfies
// it in production. This is the same testability seam as the generate
// package's function-typed deps. (Moved here from run.go in P1.M7.T2.S2 and
// extended with StagedFileCount for the FR18 "(N files)" notice.)
type stager interface {
	// HasStagedChanges reports whether the index holds staged changes (git
	// diff --cached --quiet → exit0=false/exit1=true).
	HasStagedChanges() (bool, error)
	// AddAll runs `git add -A` (stages new + modified + deleted across the
	// worktree). It is a PRIMITIVE — the WHEN/WHY is decided by maybeAutoStage.
	AddAll() error
	// StagedFileCount returns the number of files currently staged (git diff
	// --cached --name-only), the value maybeAutoStage prints in the FR18
	// "(N files)" notice.
	StagedFileCount() (int, error)
}

// maybeAutoStage implements the FR16–FR20 staging policy: the decision of WHEN
// to run `git add -A` and what to do on a still-clean index. It is THE sole
// staging entry-point the v1 default action (runDefault) calls; v2 swaps it
// whole for a partitioned-staging loop (decisions.md §1, plan_overview key
// decision 6). It runs BEFORE GenerateCommit so the snapshot captures the
// staged set, and it is owned HERE — never inside CommitStaged/generate, which
// assume the index is already staged (the v2 seam).
//
// It is pure except for the staging side effects routed through the injected
// stager g and the human-facing notices routed to out.Progressf (stderr, FR51,
// so stdout stays byte-clean for piping).
//
// Policy (binding — see research §3):
//   - Something already staged: proceed (nil), unless --all forces an
//     additional `git add -A` first (FR20).
//   - Nothing staged + (--no-auto-stage OR auto_stage_all disabled): print
//     "Nothing staged; nothing to commit." and return ErrNothingStaged (FR19)
//     — never stage.
//   - Nothing staged + auto-staging allowed: run `git add -A` (FR16), count the
//     staged files, print "Nothing staged — staging all changes (N files)."
//     (FR18), re-check, and if STILL clean (an empty worktree) print
//     "Nothing to commit." and return stagehand.ErrNothingToCommit (FR17);
//     otherwise proceed (nil).
//
// Returns nil to proceed to generation, or a sentinel/wrapped error for
// runDefault to route via mapErrorToExitCode: ErrNothingStaged (FR19) and
// ErrNothingToCommit (FR17) both → exit 2; a wrapped git failure → exit 1.
// Git failures are wrapped with fmt.Errorf("stage: %w", err) so they fall
// through to mapErrorToExitCode's generic ExitError(1) branch (they are NOT
// the staging sentinels).
func maybeAutoStage(g stager, out *ui.Output, cfg config.Config, allFlag, noAutoStage bool) error {
	staged, err := g.HasStagedChanges()
	if err != nil {
		return fmt.Errorf("stage: %w", err) // → mapErrorToExitCode ExitError(1)
	}

	if staged {
		if allFlag {
			// FR20: force `git add -A` even though something is already staged.
			if err := g.AddAll(); err != nil {
				return fmt.Errorf("stage: %w", err)
			}
		}
		return nil
	}

	// Nothing staged. FR19: --no-auto-stage OR config auto_stage_all=false →
	// decline auto-staging WITHOUT touching the index.
	if noAutoStage || !cfg.AutoStageAll {
		out.Progressf("Nothing staged; nothing to commit.\n")
		return ErrNothingStaged
	}

	// FR16/FR18: auto-stage all, count, print the notice, then re-check.
	if err := g.AddAll(); err != nil {
		return fmt.Errorf("stage: %w", err)
	}
	n, err := g.StagedFileCount()
	if err != nil {
		return fmt.Errorf("stage: %w", err)
	}
	noun := "files"
	if n == 1 {
		noun = "file"
	}
	out.Progressf("Nothing staged — staging all changes (%d %s).\n", n, noun) // FR18

	staged, err = g.HasStagedChanges()
	if err != nil {
		return fmt.Errorf("stage: %w", err)
	}
	if !staged {
		// FR17: clean worktree even after `git add -A`.
		out.Progressf("Nothing to commit.\n")
		return stagehand.ErrNothingToCommit
	}
	return nil
}
