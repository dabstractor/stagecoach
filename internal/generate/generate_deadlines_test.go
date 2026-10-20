package generate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/stubtest"
)

// TestCommitStaged_MessageRoleIdleTimeout verifies §9.30 FR-A4/FR-R7 on the single-commit path:
// the message role's resolved IDLE deadline (config.ResolveRoleDeadlines("message", cfg)) — not
// just the hard cap — bounds the one-shot Execute. Sets the role's IdleTimeout SMALL (150ms) with
// the GLOBAL hard cap BOUNDED (3s) and the stub sleeping past both (4s): on Linux the idle watcher
// arms (D4) and kills at 150ms of silence; on darwin/windows print-mode the idle is DISARMED and
// the 3s hard cap lands instead. Asserting ONLY errors.Is(err, context.DeadlineExceeded) is
// platform-safe because BOTH kill kinds satisfy it (FR-A5: an idle kill is downstream-identical to
// a hard-cap timeout; ErrIdleDeadline.Is(DeadlineExceeded) — the provider-level distinction is
// S2's executor test's job, not a call-site concern).
func TestCommitStaged_MessageRoleIdleTimeout(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "z.txt", "data")
	stageFile(t, repo, "z.txt")

	// Stub prints only AFTER its sleep ⇒ silent while running ⇒ the idle clock (150ms) expires.
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: slow", SleepMS: 4000})
	cfg := config.Defaults()
	cfg.Timeout = 3 * time.Second                                                              // BOUNDED global hard cap (non-Linux print-mode kill path)
	cfg.Roles = map[string]config.RoleConfig{"message": {IdleTimeout: 150 * time.Millisecond}} // SMALL role idle → times out (Linux kill path)

	beforeHEAD := headSHA(t, repo)

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err == nil {
		t.Fatal("expected error on message-role idle deadline, got nil")
	}

	var re *RescueError
	if !errors.As(err, &re) {
		t.Fatalf("error type = %T, want *RescueError", err)
	}
	// ErrTimeout is minted ONLY by the DeadlineExceeded branch (§5: immediate rescue, NO retry), so
	// errors.Is(err, ErrTimeout) proves a deadline (idle on Linux / hard cap elsewhere) reached
	// Execute. RescueError.Unwrap returns Kind (not Cause), so the raw sentinel is asserted on Cause.
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("errors.Is(err, ErrTimeout) = false, want true (FR-A5: the idle kill maps through the unchanged timeout branch)")
	}
	if !errors.Is(re.Cause, context.DeadlineExceeded) {
		t.Errorf("errors.Is(re.Cause, context.DeadlineExceeded) = false, want true (cause = %v; the 150ms role idle — or the 3s cap on non-Linux — should be the kill)", re.Cause)
	}
	if re.TreeSHA == "" {
		t.Error("RescueError.TreeSHA is empty, want non-empty (snapshot was taken)")
	}

	// HEAD unchanged (rescue, no commit).
	if got := headSHA(t, repo); got != beforeHEAD {
		t.Errorf("HEAD changed from %q to %q on idle deadline, want unchanged", beforeHEAD, got)
	}
}
