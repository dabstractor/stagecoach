package provider

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dabstractor/stagecoach/internal/signal"
	"github.com/dabstractor/stagecoach/internal/ui"
)

// StderrHint surfaces a provider's stderr when its stdout is empty/unusable — the common case when an
// agent errors BEFORE generating output (invalid/unentitled model, auth, quota, usage limit). Providers
// put these errors on stderr and leave stdout empty, which otherwise surfaces downstream as an opaque
// "not valid JSON" / empty-output parse error that hides the real cause.
//
// Returns "" when stdout carries usable (non-whitespace) content OR stderr is empty, so callers can
// branch on it unconditionally. Otherwise it returns a single-line hint carrying the trimmed, length-
// capped stderr — the text that explains why the agent produced nothing.
func StderrHint(stdout, stderr string) string {
	if strings.TrimSpace(stdout) != "" || strings.TrimSpace(stderr) == "" {
		return ""
	}
	hint := strings.TrimSpace(stderr)
	// collapse to a single line and cap so a noisy provider can't flood the error
	hint = strings.ReplaceAll(hint, "\n", " ")
	if len(hint) > 1000 {
		hint = hint[:1000] + "…"
	}
	return "agent produced no output; provider stderr: " + hint
}

// Deadlines carries the two §9.30 generation deadlines (FR-A1/FR-A2), resolved per role at the call
// site via config.ResolveRoleDeadlines (FR-R7 — provider stays config-free; D1 keeps resolution at
// the call sites). Both bind each invocation individually (FR-A4): a fresh idle clock per Execute call.
type Deadlines struct {
	Idle time.Duration // kill after this long with NO observable activity (FR-A1); ≤0 ⇒ not armed
	Hard time.Duration // absolute wall-clock backstop (FR-A2); ≤0 ⇒ unbounded (today's timeout==0)
}

// ErrIdleDeadline is returned when the idle watcher killed the agent's process group (§9.30 FR-A1).
// Its Is method satisfies errors.Is(err, context.DeadlineExceeded) (D2/FR-A5), so EVERY existing
// downstream branch — exit-124 mapping, ErrTimeout rescue, the no-retry-on-timeout rule, FR-T7
// multi-turn aborts, the arbiter's graceful-null — treats an idle kill exactly as a hard-cap timeout.
// Only the --verbose cause line (ui.VerboseCause) distinguishes them.
var ErrIdleDeadline = &idleDeadlineError{}

type idleDeadlineError struct{}

func (e *idleDeadlineError) Error() string {
	return "context deadline exceeded (idle: no observable agent activity)"
}

// Is makes an idle kill indistinguishable from a hard-cap timeout downstream (D2/FR-A5).
func (e *idleDeadlineError) Is(target error) bool { return target == context.DeadlineExceeded }

// Execute runs a provider CmdSpec as a subprocess and returns its captured stdout, captured stderr,
// and a result error. It is the third stage of the provider pipeline (PRD §9.5/FR24–FR25): manifests
// (T1–T3) describe the agent; Render (T4) composes the CmdSpec; Execute (T5.S1) runs it; the parser
// (T6) reads stdout. The returned stdout is the agent's raw output for the parser; err signals
// timeout/failure for the orchestrator's retry/rescue (§18.2).
//
// I/O: spec.Stdin (non-empty) is piped to the child's stdin; "" leaves stdin nil so os/exec gives the
// child /dev/null (the CmdSpec contract from render.go). stdout and stderr are captured through
// activity-counting writers (§9.30 FR-A3(a): every pipe write stamps the writer's LastWrite) and
// returned even on error (partial output is useful for the rescue path §18.3 + verbose mode). The
// returned strings are snapshot copies of those writers. cmd.Env = spec.Env when non-empty (Render
// builds os.Environ()+manifest env); nil Env ⇒ the child inherits the parent env. cmd.Dir is NOT
// set — the agent runs in the user's CWD.
//
// Deadlines (§9.30 FR-A1/FR-A2): dl.Idle is the ACTIVITY deadline — kill after this long with no
// observable agent activity: a pipe write to stdout/stderr (any platform), or — Linux only — growth
// of the process group's read counter (rchar), which covers agents whose stdout stays silent or
// buffered while they work (print-mode CLIs). dl.Hard is the absolute wall-clock backstop (v3.4's
// timeout). Per D4, the idle watcher arms only when an Idle deadline is set AND at least one
// activity channel exists (Linux's rchar counter, or spec.ProgressiveStdout on any platform):
// stagecoach never fabricates activity, so with no channel the run degrades to hard-cap-only —
// exactly v3.4 behavior. Both deadlines bind each invocation individually (FR-A4) and kill through
// the SAME group-kill path. dl.Idle ≤ 0 ⇒ idle disarmed; dl.Hard ≤ 0 ⇒ unbounded (the parent ctx
// still applies).
//
// Kill semantics (PRD §18.2/§18.4, FINDING 8): the child runs as its own process-group leader
// (Setpgid ⇒ PGID==PID). On runCtx cancellation (hard cap, idle expiry, OR parent/signal cancel)
// cmd.Cancel sends SIGTERM to the WHOLE group (-pid), killing grandchildren too; cmd.WaitDelay (3s)
// is the grace before Go escalates to SIGKILL. The platform specifics live in procgroup_unix.go
// (this file is platform-agnostic — setupProcessGroup is the single seam).
//
// Error contract (check the idleKilled flag FIRST, THEN runCtx.Err() — an idle kill cancels runCtx,
// which reads as Canceled at the ctx level; the flag set before cancel() is the discriminator):
//
//   - idle expiry   ⇒ err IS ErrIdleDeadline, whose Is() satisfies errors.Is(err,
//     context.DeadlineExceeded) (D2/FR-A5: an idle kill IS a timeout downstream — exit 124 +
//     rescue; only the --verbose cause line differs)
//   - hard cap      ⇒ err IS context.DeadlineExceeded  (orchestrator: exit 124 + rescue)
//   - signal/parent ⇒ err IS context.Canceled          (orchestrator: exit 3 + rescue)
//   - non-zero exit ⇒ wrapped *exec.ExitError           (orchestrator: retry, then rescue)
//   - start failure ⇒ wrapped LookPath/start error      (orchestrator: "command not found", exit 1)
//   - success       ⇒ err == nil
func Execute(ctx context.Context, spec CmdSpec, dl Deadlines, vb *ui.Verbose) (stdout string, stderr string, err error) {
	if dl.Hard > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, dl.Hard) // SHADOW — see doc; do not rename (now keyed on dl.Hard)
		defer cancel()
	}
	// runCtx is the idle watcher's kill path: cancelling it reaches the SAME cmd.Cancel group-kill
	// as the hard cap (both flow through CommandContext's ctx watching). The deferred cancelRun also
	// ends the watcher goroutine at Execute return (≤1 stray tick — groupReadBytes is read-only,
	// so a tail tick on a recycled pgid is harmless, the same discipline as signal.ClearChild).
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	cmd := exec.CommandContext(runCtx, spec.Command, spec.Args...) // NOT ctx — the idle watcher must reach the kill
	if spec.Stdin != "" {
		cmd.Stdin = strings.NewReader(spec.Stdin) // "" ⇒ nil ⇒ /dev/null (CmdSpec contract)
	}
	// Activity-counting capture (§9.30 FR-A3(a)): every pipe write stamps the writer's LastWrite —
	// the portable half of the idle-activity signal. Bytes() is a snapshot copy, safe against the
	// copy goroutines os/exec spawns for non-*os.File writers.
	var out, errb countingWriter
	cmd.Stdout = &out // separate capture + write stamps
	cmd.Stderr = &errb
	if len(spec.Env) > 0 {
		cmd.Env = spec.Env // Render populates; nil ⇒ inherit parent env
	}
	setupProcessGroup(cmd) // platform seam (procgroup_*.go): Setpgid + Cancel + WaitDelay

	vb.VerboseCommand(strings.Join(append([]string{spec.Command}, spec.Args...), " "))
	vb.VerbosePayload(spec.PayloadBytes) // size only (never contents) — exposes whether the token-limit gate ran
	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("provider %q: start: %w", spec.Command, err)
	}
	signal.RegisterChild(cmd.Process.Pid) // arm signal forwarding (Setpgid ⇒ PGID==PID)
	defer signal.ClearChild()             // clear before return so a later signal can't kill a recycled PID

	// D4 arming rule (locked): the watcher arms iff an idle deadline is set AND an activity channel
	// exists — Linux's group read counter covers every provider on Linux; elsewhere only progressive
	// (unbuffered-visible) stdout arms the pipe channel. No channel ⇒ disarmed ⇒ v3.4 behavior.
	var idleKilled atomic.Bool
	if dl.Idle > 0 && (runtime.GOOS == "linux" || spec.ProgressiveStdout) {
		go watchIdle(runCtx, cancelRun, &idleKilled, dl.Idle, cmd.Process.Pid, &out, &errb)
	}

	if werr := cmd.Wait(); werr != nil {
		vb.VerboseRawOutput(string(out.Bytes()))
		vb.VerboseStderr(string(errb.Bytes())) // surface the provider's real failure reason (upstream errors live on stderr)
		if idleKilled.Load() {                 // FIRST — an idle cancelRun reads as Canceled at the ctx level
			vb.VerboseCause(fmt.Sprintf("idle deadline (%v without observable activity)", dl.Idle))
			return string(out.Bytes()), string(errb.Bytes()), ErrIdleDeadline
		}
		if ctxErr := runCtx.Err(); ctxErr != nil {
			if errors.Is(ctxErr, context.DeadlineExceeded) {
				vb.VerboseCause(fmt.Sprintf("hard cap (%v)", dl.Hard))
			}
			return string(out.Bytes()), string(errb.Bytes()), ctxErr // hard cap → DeadlineExceeded; parent cancel → Canceled
		}
		return string(out.Bytes()), string(errb.Bytes()), fmt.Errorf("provider %q: %w", spec.Command, werr) // exit failure
	}
	vb.VerboseRawOutput(string(out.Bytes()))
	vb.VerboseStderr(string(errb.Bytes())) // surface provider warnings/diagnostics even on success
	return string(out.Bytes()), string(errb.Bytes()), nil
}

// watchIdle is the §9.30 FR-A1 activity watchdog: ONE clock, reset by whichever activity channel
// moved last (D3). Channels: pipe writes (out/errb LastWrite — portable) and, on Linux, growth of
// the process group's cumulative read counter (rchar — covers stdout-silent/buffered agents).
// Inactivity for `idle` kills the run by storing the killed flag (BEFORE cancel — the discriminator
// vs a parent cancel) and cancelling the run context, which triggers cmd.Cancel's group SIGTERM.
//
// Baseline rules: the clock starts at arm time — a never-written countingWriter reports the zero
// time.Time, which time.Since treats as "idle since forever", so max(LastWrite, armed) is REQUIRED.
// groupReadBytes is a CUMULATIVE sum over live group members (a member exiting REDUCES it; the
// counters are non-atomic), so the first sample only records the baseline and only a STRICTLY
// positive delta counts as activity (clamped — a negative delta is "no activity", never an error).
// ok==false (non-Linux or /proc unavailable) ⇒ the pipe channels alone (D4's degradation clause).
// Tail race: after Execute returns, its deferred cancelRun closes ctx.Done() and this goroutine
// exits within ≤1 tick; a stray read-only /proc sweep is harmless.
func watchIdle(ctx context.Context, cancel context.CancelFunc, killed *atomic.Bool, idle time.Duration, pgid int, out, errb *countingWriter) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	lastActivity := time.Now() // armed baseline (see doc)
	var lastRchar uint64
	haveRchar := false

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			// Channel 1 (FR-A3(a), portable): pipe writes to stdout/stderr.
			if lw := out.LastWrite(); lw.After(lastActivity) {
				lastActivity = lw
			}
			if lw := errb.LastWrite(); lw.After(lastActivity) {
				lastActivity = lw
			}
			// Channel 2 (FR-A3(b), Linux): group read-counter growth.
			if r, ok := groupReadBytes(pgid); ok {
				if haveRchar && r > lastRchar { // strictly positive delta only (clamped)
					lastActivity = now
				}
				lastRchar = r
				haveRchar = true
			}
			if now.Sub(lastActivity) >= idle {
				killed.Store(true) // set BEFORE cancel — distinguishes idle from a parent cancel
				cancel()
				return
			}
		}
	}
}
