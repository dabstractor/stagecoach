package main

// White-box MOCKING tests for the sole staging entry-point maybeAutoStage in
// stage.go (FR16–FR20). They are package main (NOT main_test) so they can
// reference the unexported stager interface, the ErrNothingStaged sentinel,
// maybeAutoStage, and mapErrorToExitCode directly. They mirror the repo's
// testing conventions: stdlib + internal/* + pkg/stagehand only, NO testify.
//
// The hermetic target is maybeAutoStage driven against a STATEFUL fakeStager
// (it flips `staged` to `stagedAfterAdd` when AddAll is called) so the
// "proceed after auto-stage" happy path — MISSING in S1's int-returning tests —
// is now covered, alongside the FR17 still-clean, FR19 declined, and FR20
// force-add paths. *git.Git satisfies the real stager interface; this stub
// lets the policy tests run without a real git binary or a temp repo.

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/ui"
	"github.com/dustin/stagehand/pkg/stagehand"
)

// fakeStager is a STATEFUL in-process stager stub. HasStagedChanges returns
// (staged, stageErr); AddAll records that it was called AND flips `staged` to
// `stagedAfterAdd` (so a test can model "auto-stage made something staged" vs
// "auto-stage still left the worktree clean"); StagedFileCount returns
// (fileCount, countErr) — the value maybeAutoStage prints in the FR18 "(N
// files)" notice. *git.Git satisfies the real stager interface; this stub lets
// the FR16–FR20 policy tests run without a real git binary.
type fakeStager struct {
	staged         bool
	stagedAfterAdd bool // staged is flipped to this when AddAll is called
	fileCount      int
	stageErr       error
	addErr         error
	countErr       error
	addCalled      bool
}

func (f *fakeStager) HasStagedChanges() (bool, error) { return f.staged, f.stageErr }

func (f *fakeStager) AddAll() error {
	f.addCalled = true
	f.staged = f.stagedAfterAdd // stateful flip: model the post-add index state
	return f.addErr
}

func (f *fakeStager) StagedFileCount() (int, error) { return f.fileCount, f.countErr }

// TestMaybeAutoStage covers the FR16–FR20 MOCKING contract for maybeAutoStage
// via t.Run subtests (research §4). The assertions moved from S1's int codes to
// errors.Is against the DISTINCT sentinels (ErrNothingStaged for FR19 vs
// stagehand.ErrNothingToCommit for FR17) and check the FR18 "(N files)" notice
// and the AddAll call/no-call discipline.
func TestMaybeAutoStage(t *testing.T) {
	// Case 1 (NEW happy path, MISSING in S1): nothing staged + auto-staging on
	// → AddAll runs, the FR18 notice carries the file count, and the now-staged
	// index lets maybeAutoStage return nil (proceed to generation).
	t.Run("AutoStagesThenProceeds", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		out := ui.NewOutput(&stdout, &stderr, false, false)
		// stagedAfterAdd=true models "git add -A staged 3 files".
		g := &fakeStager{staged: false, stagedAfterAdd: true, fileCount: 3}
		cfg := config.Config{AutoStageAll: true}

		err := maybeAutoStage(g, out, cfg, false /*all*/, false /*noAutoStage*/)

		if err != nil {
			t.Fatalf("maybeAutoStage = %v, want nil (proceed after auto-stage)", err)
		}
		if !g.addCalled {
			t.Error("AddAll was not called, want it called under auto_stage_all")
		}
		if !strings.Contains(stderr.String(), "(3 files)") {
			t.Errorf("stderr = %q, want the FR18 \"(3 files)\" notice", stderr.String())
		}
		// FR51: staging notices go to stderr only; stdout must stay byte-clean.
		if stdout.Len() != 0 {
			t.Errorf("stdout = %q, want empty (FR51 byte-clean)", stdout.String())
		}
	})

	// Case 2 (FR17): nothing staged + auto-staging on, but the worktree is
	// STILL clean after `git add -A` (an empty worktree) → errors.Is
	// ErrNothingToCommit + the "Nothing to commit." notice.
	t.Run("AutoStagesThenClean", func(t *testing.T) {
		var stderr bytes.Buffer
		out := ui.NewOutput(&bytes.Buffer{}, &stderr, false, false)
		// stagedAfterAdd=false models "git add -A left the index clean".
		g := &fakeStager{staged: false, stagedAfterAdd: false, fileCount: 0}
		cfg := config.Config{AutoStageAll: true}

		err := maybeAutoStage(g, out, cfg, false, false)

		if !errors.Is(err, stagehand.ErrNothingToCommit) {
			t.Errorf("err = %v, want errors.Is ErrNothingToCommit (FR17)", err)
		}
		if !g.addCalled {
			t.Error("AddAll was not called, want it called before the clean re-check")
		}
		if !strings.Contains(stderr.String(), "Nothing to commit") {
			t.Errorf("stderr = %q, want the FR17 \"Nothing to commit.\" notice", stderr.String())
		}
	})

	// Case 3 (FR19, trigger 1): nothing staged + --no-auto-stage → errors.Is
	// ErrNothingStaged, WITHOUT touching the index (AddAll must NOT be called).
	t.Run("NoAutoStageFlagReturnsNothingStaged", func(t *testing.T) {
		var stderr bytes.Buffer
		out := ui.NewOutput(&bytes.Buffer{}, &stderr, false, false)
		g := &fakeStager{staged: false}
		cfg := config.Config{AutoStageAll: true} // auto-stage IS on, but the flag overrides

		err := maybeAutoStage(g, out, cfg, false /*all*/, true /*noAutoStage*/)

		if !errors.Is(err, ErrNothingStaged) {
			t.Errorf("err = %v, want errors.Is ErrNothingStaged (FR19)", err)
		}
		if g.addCalled {
			t.Error("AddAll was called, want it NOT called under --no-auto-stage")
		}
		if !strings.Contains(stderr.String(), "nothing to commit") {
			t.Errorf("stderr = %q, want a nothing-to-commit notice", stderr.String())
		}
	})

	// Case 4 (FR19, trigger 2): nothing staged + cfg.AutoStageAll=false (no
	// --no-auto-stage) → errors.Is ErrNothingStaged, WITHOUT calling AddAll.
	t.Run("AutoStageAllFalseReturnsNothingStaged", func(t *testing.T) {
		var stderr bytes.Buffer
		out := ui.NewOutput(&bytes.Buffer{}, &stderr, false, false)
		g := &fakeStager{staged: false}
		cfg := config.Config{AutoStageAll: false} // config disables auto-staging

		err := maybeAutoStage(g, out, cfg, false /*all*/, false /*noAutoStage*/)

		if !errors.Is(err, ErrNothingStaged) {
			t.Errorf("err = %v, want errors.Is ErrNothingStaged (FR19 via AutoStageAll=false)", err)
		}
		if g.addCalled {
			t.Error("AddAll was called, want it NOT called when AutoStageAll=false")
		}
		if !strings.Contains(stderr.String(), "nothing to commit") {
			t.Errorf("stderr = %q, want a nothing-to-commit notice", stderr.String())
		}
	})

	// Case 5 (FR20): already staged + --all → AddAll is forced on top of the
	// existing staged set, then maybeAutoStage returns nil (proceed).
	t.Run("AllFlagForcesAddOnStaged", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		out := ui.NewOutput(&stdout, &stderr, false, false)
		g := &fakeStager{staged: true}
		cfg := config.Config{AutoStageAll: true}

		err := maybeAutoStage(g, out, cfg, true /*all*/, false /*noAutoStage*/)

		if err != nil {
			t.Fatalf("maybeAutoStage = %v, want nil (proceed after --all)", err)
		}
		if !g.addCalled {
			t.Error("AddAll was not called, want it forced under --all with staged changes")
		}
		// FR20 force-add is silent: no FR18/FR17 notice lands on stderr.
		if stderr.Len() != 0 {
			t.Errorf("stderr = %q, want empty (--all on staged adds silently)", stderr.String())
		}
	})

	// Case 6 (common path): already staged, no --all → proceed (nil) WITHOUT
	// calling AddAll.
	t.Run("StagedNoAllProceeds", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		out := ui.NewOutput(&stdout, &stderr, false, false)
		g := &fakeStager{staged: true}
		cfg := config.Config{AutoStageAll: true}

		err := maybeAutoStage(g, out, cfg, false /*all*/, false /*noAutoStage*/)

		if err != nil {
			t.Fatalf("maybeAutoStage = %v, want nil (proceed, already staged)", err)
		}
		if g.addCalled {
			t.Error("AddAll was called, want it NOT called when staged and no --all")
		}
		if stderr.Len() != 0 {
			t.Errorf("stderr = %q, want empty (already-staged is silent)", stderr.String())
		}
	})
}

// TestMapErrorToExitCode_ErrNothingStaged asserts the distinct ErrNothingStaged
// sentinel maps to exit 2 (ui.ExitNothingToCommit) — the same code as the FR17
// clean-after-add path, so the PRD §15.4 table is unchanged; the distinction is
// the sentinel for programmatic errors.Is branching.
func TestMapErrorToExitCode_ErrNothingStaged(t *testing.T) {
	if got := mapErrorToExitCode(ErrNothingStaged); got != ui.ExitNothingToCommit {
		t.Errorf("mapErrorToExitCode(ErrNothingStaged) = %d, want %d (ExitNothingToCommit)", got, ui.ExitNothingToCommit)
	}
}
