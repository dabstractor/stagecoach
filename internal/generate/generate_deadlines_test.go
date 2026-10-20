package generate

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/stubtest"
	"github.com/dabstractor/stagecoach/internal/ui"
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

// TestCommitStaged_ProductiveSlowEmitterSurvives proves §9.30 FR-A1 (the false-kill regression —
// the motivating v3.4 scenario): a productive-but-slow agent that emits a stdout byte every 500ms
// for ~3× the idle window (idle 2s, emitting 6s) and THEN answers must SURVIVE — generation
// succeeds and the commit is created. The v3.4 elapsed-time-only kill (a 120s wall clock) could
// not distinguish this agent from a hung one; the activity-based idle deadline must.
//
// D4 arming note: the idle watcher arms iff Idle>0 AND (linux || spec.ProgressiveStdout);
// ProgressiveStdout is always false until P2 (event-stream), so on LINUX the watcher is armed and
// each 500ms tick resets the idle clock — this test genuinely discriminates there. On
// darwin/windows the watcher is DISARMED (D4's documented v3.4 degradation): the idle deadline
// cannot fire at all and this test degrades to a green success-path smoke — correct on every
// platform, never red, discrimination Linux-carried (CI).
func TestCommitStaged_ProductiveSlowEmitterSurvives(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow false-kill regression in -short mode")
	}
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "z.txt", "data")
	stageFile(t, repo, "z.txt")
	before := headSHA(t, repo)

	var vbuf bytes.Buffer
	// Persona (b): stdout tick every 500ms for ~6s, THEN the final answer (the stdout suffix).
	// Ticks and the answer share ONE stdout buffer — EmitLine "\n" makes each tick a bare newline
	// so ParseOutput's TrimSpace strips them: Message/Subject equal the Out value exactly.
	m := stubtest.Manifest(bin, stubtest.Options{
		EmitMS:      500,
		EmitLine:    "\n",
		EmitTotalMS: 6000,
		Out:         "feat: progressive answer",
	})
	cfg := config.Defaults()
	cfg.IdleTimeout = 2 * time.Second // SMALL idle: a SILENT agent dies at ~2-2.5s
	cfg.Timeout = 30 * time.Second    // LARGE hard cap: mathematically cannot fire (~6s run)

	start := time.Now()
	res, err := CommitStaged(context.Background(),
		Deps{Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&vbuf, true)}, cfg)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("productive-slow emitter was killed / failed: %v (FR-A1 false-kill regression)", err)
	}

	// The commit IS created — subject/message exactly the answer (ticks stripped by ParseOutput).
	if res.Subject != "feat: progressive answer" {
		t.Errorf("Subject = %q, want %q", res.Subject, "feat: progressive answer")
	}
	if res.Message != "feat: progressive answer" {
		t.Errorf("Message = %q, want %q (EmitLine \"\\n\" ticks strip in ParseOutput)", res.Message, "feat: progressive answer")
	}
	if got := headSHA(t, repo); got == before || got != res.CommitSHA {
		t.Errorf("HEAD = %s, want advanced to CommitSHA %s (before %s)", got, res.CommitSHA, before)
	}
	if subj := gitOut(t, repo, "log", "-1", "--format=%s"); subj != "feat: progressive answer" {
		t.Errorf("git log -1 subject = %q, want %q", subj, "feat: progressive answer")
	}

	// Single Execute, nothing killed: no parse/duplicate retry, no deadline cause line.
	if n := strings.Count(vbuf.String(), "DEBUG: command: "); n != 1 {
		t.Errorf("Execute count = %d, want 1", n)
	}
	if strings.Contains(vbuf.String(), "DEBUG: cause:") {
		t.Errorf("a deadline fired on a productive run; buf:\n%s", vbuf.String())
	}

	// The run genuinely outlived the idle window ~3× (the false-kill discriminator floor; no tight
	// ceiling — scheduler variance is absorbed by the 30s hard cap, not by a flaky upper bound).
	if elapsed < 6000*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 6s (the emit phase must complete)", elapsed)
	}
}
