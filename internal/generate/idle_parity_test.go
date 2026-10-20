// idle_parity_test.go is the FR-A5 (§9.30) exit-124 parity proof: a hung agent killed by the IDLE
// deadline must be downstream-IDENTICAL to a hard-cap timeout — same *generate.RescueError{Kind:
// ErrTimeout} with the tree SHA, same exit-124 mapping (exitcode.For), same immediate-rescue
// NO-retry, same FormatRescue "Tree ID:" message — with the ONLY observable difference being the
// --verbose "DEBUG: cause:" line naming the idle deadline (verbose.go's documented contract).
//
// This is an EXTERNAL test (package generate_test) so it can import internal/exitcode — a white-box
// package generate test CANNOT: exitcode imports generate (For() checks generate.ErrTimeout), so a
// package-generate test importing exitcode is "import cycle not allowed in test". It defines its own
// minimal initTempRepo/headSHA helpers (the white-box generate_test.go helpers are in package
// generate and so unimportable here) — the same pattern as hooks_freeze_test.go.
//
// Platform note (executor D4): the idle watcher arms only on Linux (the rchar group-read channel;
// spec.ProgressiveStdout is always false until P2). The hard cap is therefore BOUNDED at 3s (not the
// 600s default) so non-Linux matrix jobs take the hard-cap kill instead of watching the stub SUCCEED
// — the same platform-safe idiom as TestCallPlanner_IdleTimeout and
// TestCommitStaged_WorkDescription_MessageRoleIdleTimeout. Both deadline paths route through the
// SAME errors.Is(execErr, context.DeadlineExceeded) rescue branch (ErrIdleDeadline.Is() ⇒
// DeadlineExceeded — the D2 linchpin), so every assertion here is platform-safe EXCEPT the
// idle-specific cause assertions, which are asserted on Linux (where the idle actually fires) and
// mirrored as hard-cap cause assertions elsewhere.
package generate_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/exitcode"
	"github.com/dabstractor/stagecoach/internal/generate"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/stubtest"
	"github.com/dabstractor/stagecoach/internal/ui"
)

// stageChange modifies fileA.txt and stages it (something real for the snapshot to freeze).
// (Uses the seed file created by initTempRepo — hooks_freeze_test.go's copy, shared in this
// external package since the white-box generate_test.go helpers are unimportable here.)
func stageChange(t *testing.T, repo string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, "fileA.txt"), []byte("a-modified-for-idle-kill\n"), 0o644); err != nil {
		t.Fatalf("modify fileA: %v", err)
	}
	if out, err := exec.Command("git", "-C", repo, "add", "fileA.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add fileA: %v %s", err, out)
	}
}

// headSHA returns the repo's current HEAD SHA (trimmed).
func headSHA(t *testing.T, repo string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestIdleKill_ExitCodeParity is THE exit-124 proof: full CommitStaged with a hung stub + a SMALL
// idle deadline asserts the whole FR-A5 chain — *RescueError{Kind: ErrTimeout} with a non-empty
// TreeSHA, a rescue message carrying "Tree ID: <sha>", a DeadlineExceeded cause, exitcode.For == 124,
// exactly ONE Execute invocation (immediate rescue, NO retry), HEAD unchanged, and a fast kill
// (hang-guard). On Linux the cause names the idle deadline; elsewhere (D4-disarmed) the bounded hard
// cap — both through the identical downstream chain.
func TestIdleKill_ExitCodeParity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow idle-kill parity test in -short mode")
	}
	repo := initTempRepo(t)
	stageChange(t, repo)
	before := headSHA(t, repo)

	bin := stubtest.Build(t)
	var vbuf bytes.Buffer
	// Persona (a): the stub sleeps SILENTLY (no stdout/stderr bytes) past both deadlines.
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: never", SleepMS: 5000})
	cfg := config.Defaults()
	cfg.Timeout = 3 * time.Second            // BOUNDED hard cap (non-Linux D4 kill path)
	cfg.IdleTimeout = 150 * time.Millisecond // SMALL idle: the only deadline that can kill on Linux

	start := time.Now()
	_, err := generate.CommitStaged(context.Background(), generate.Deps{
		Git:      git.New(repo),
		Manifest: m,
		Verbose:  ui.NewVerbose(&vbuf, true),
	}, cfg)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected a deadline rescue, got nil (the stub completed — no deadline bounded Execute)")
	}

	// (a) The rescue shape: *RescueError, Kind=ErrTimeout, the frozen tree SHA captured.
	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Fatalf("err = %T %v, want *generate.RescueError", err, err)
	}
	if !errors.Is(err, generate.ErrTimeout) {
		t.Errorf("errors.Is(err, generate.ErrTimeout) = false: %v", err)
	}
	if re.TreeSHA == "" {
		t.Error("TreeSHA empty — the snapshot is missing from the rescue")
	}

	// (b) The rescue MESSAGE carries the tree SHA (the §18.3 "Tree ID:" line the CLI prints).
	if msg := generate.FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate); !strings.Contains(msg, "Tree ID: "+re.TreeSHA) {
		t.Errorf("FormatRescue missing 'Tree ID: %s'; msg=\n%s", re.TreeSHA, msg)
	}

	// (c) The cause is a DeadlineExceeded on BOTH platforms (the D2 linchpin — ErrIdleDeadline.Is()
	// satisfies it on Linux; a hard cap IS one elsewhere) + names "idle" on Linux specifically.
	if re.Cause == nil || !errors.Is(re.Cause, context.DeadlineExceeded) {
		t.Errorf("Cause = %v, want an error satisfying errors.Is(…, context.DeadlineExceeded)", re.Cause)
	}
	if runtime.GOOS == "linux" {
		if re.Cause == nil || !strings.Contains(re.Cause.Error(), "idle") {
			t.Errorf("Cause = %v, want an error naming 'idle' (ErrIdleDeadline)", re.Cause)
		}
		if !strings.Contains(vbuf.String(), "DEBUG: cause: idle deadline") {
			t.Errorf("verbose missing the idle cause line; buf=\n%s", vbuf.String())
		}
	} else if !strings.Contains(vbuf.String(), "DEBUG: cause: hard cap") {
		t.Errorf("verbose missing the hard-cap cause line (D4-disarmed platform); buf=\n%s", vbuf.String())
	}

	// (d) THE exit-124 semantics — the reason this file is package generate_test.
	if code := exitcode.For(err); code != exitcode.Timeout {
		t.Errorf("exitcode.For(err) = %d, want %d (124)", code, exitcode.Timeout)
	}

	// (e) NO-retry: exactly ONE Execute invocation — the DeadlineExceeded branch returns from inside
	// the retry loop on the first hit ("DEBUG: command: " is emitted once per Execute).
	if n := strings.Count(vbuf.String(), "DEBUG: command: "); n != 1 {
		t.Errorf("Execute called %d times, want 1 (immediate rescue, NO retry)", n)
	}

	// (f) Nothing moved + a hang-guard (an engine regression that never fires would otherwise ride
	// the 3s cap silently — the guard documents the expected ~1s kill window generously).
	if got := headSHA(t, repo); got != before {
		t.Errorf("HEAD moved on the rescued run: %s → %s", before, got)
	}
	if elapsed > 10*time.Second {
		t.Errorf("deadline kill took %v; want ≪10s (hang-guard)", elapsed)
	}
}

// TestCommitStaged_WorkDescription_IdleKill is the workdesc arm of the FR-A5 parity proof (mirroring
// TestCommitStaged_WorkDescription_MessageRoleTimeout / …MessageRoleIdleTimeout): with
// WorkDescription set + an append-mode manifest, an idle kill on turn-1's Execute routes through the
// UNCHANGED workdesc cause branch (generate.go's errors.Is(wdCause, context.DeadlineExceeded) →
// ErrTimeout) to the same *RescueError + exit-124 + NO-retry chain.
func TestCommitStaged_WorkDescription_IdleKill(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow workdesc idle-kill test in -short mode")
	}
	repo := initTempRepo(t)
	stageChange(t, repo)
	before := headSHA(t, repo)

	bin := stubtest.Build(t)
	var vbuf bytes.Buffer
	// turn-1 Execute sleeps SILENTLY past both deadlines — killed before any READ/message.
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: never reached", SleepMS: 5000})
	appendMode := "append"
	m.SessionMode = &appendMode // REQUIRED: RunWorkDescription's turn-1 RenderMultiTurn gate

	cfg := config.Defaults()
	cfg.WorkDescription = "add x"            // activates the workdesc branch (skips the one-shot loop)
	cfg.Timeout = 3 * time.Second            // BOUNDED hard cap (non-Linux D4 kill path)
	cfg.IdleTimeout = 150 * time.Millisecond // SMALL idle: the only deadline that can kill on Linux

	_, err := generate.CommitStaged(context.Background(), generate.Deps{
		Git:      git.New(repo),
		Manifest: m,
		Verbose:  ui.NewVerbose(&vbuf, true),
	}, cfg)
	if err == nil {
		t.Fatal("expected a workdesc deadline rescue, got nil")
	}

	// The same FR-A5 chain as the one-shot arm.
	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Fatalf("err = %T %v, want *generate.RescueError (workdesc turn-1 deadline → rescue)", err, err)
	}
	if !errors.Is(err, generate.ErrTimeout) {
		t.Errorf("errors.Is(err, generate.ErrTimeout) = false: %v", err)
	}
	if code := exitcode.For(err); code != exitcode.Timeout {
		t.Errorf("exitcode.For(err) = %d, want %d (124)", code, exitcode.Timeout)
	}
	if runtime.GOOS == "linux" {
		if !strings.Contains(vbuf.String(), "DEBUG: cause: idle deadline") {
			t.Errorf("verbose missing the idle cause line; buf=\n%s", vbuf.String())
		}
	} else if !strings.Contains(vbuf.String(), "DEBUG: cause: hard cap") {
		t.Errorf("verbose missing the hard-cap cause line (D4-disarmed platform); buf=\n%s", vbuf.String())
	}
	if n := strings.Count(vbuf.String(), "DEBUG: command: "); n != 1 {
		t.Errorf("Execute called %d times, want 1 (turn-1 killed, NO retry)", n)
	}
	if got := headSHA(t, repo); got != before {
		t.Errorf("HEAD moved on the rescued workdesc run: %s → %s", before, got)
	}
}
