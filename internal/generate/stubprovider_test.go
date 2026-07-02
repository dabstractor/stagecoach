package generate

// White-box test harness + self-test suite for the stub-agent provider
// (P1.M6.T3.S1). It is `package generate` (NOT `generate_test`) so it shares
// the package with its future consumer — generate.CommitStaged (P1.M6.T1.S1)
// — exactly like dedupe_test.go / rescue_test.go / signal_test.go. It exposes
// the reusable builders a downstream test composes with a single call:
//
//   - BuildStubBinary(t) — compiles internal/generate/testdata/stubagent once
//     (sync.Once) and returns the cached binary path.
//   - NewStubManifest(t, cfg) — returns a fully-wired provider.Manifest whose
//     Env carries the stub config (STAGEHAND_STUB_SCRIPT/_STATE/_STDIN).
//   - StubConfig / StubResponse — the config + per-call descriptor types.
//
// It stands as its OWN unit (plan_overview §8; decisions.md §8 — "the
// genuinely-separate test harness/infra"), so the e2e suite (M6.T3.S2) and the
// §18.1 invariant tests (M6.T3.S3) compose these helpers instead of
// reinventing a fake agent each. The self-tests below drive EVERY behavior mode
// THROUGH the REAL provider.Executor (the P1.M2.T4.S1 contract) — not by
// invoking the binary directly — mirroring internal/provider/executor_test.go's
// house convention (inline Manifest, provider.NewExecutor(""), errors.As into
// *TimeoutError/*AgentError, REAL process + timing assertions).
//
// Dependencies are stdlib + internal/provider ONLY (NO testify, NO real LLM).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/provider"
)

// StubResponse is the per-invocation behavior descriptor the test harness
// marshals into STAGEHAND_STUB_SCRIPT. Its JSON tags ({emit,hang,fail}) mirror
// stubagent.stubResponse EXACTLY — the binary json.Unmarshal's the bytes this
// type marshals, so a tag drift is a silent wrong-behavior bug (coupling
// gotcha). Per-entry precedence is Hang > Fail>0 > Emit (see
// testdata/stubagent/main.go).
type StubResponse struct {
	// Emit is written to stdout (exit 0) when neither Hang nor Fail is active.
	Emit string `json:"emit"`
	// Hang blocks forever (select{}); the executor's ctx-driven group kill
	// fells it (self-test asserts *provider.TimeoutError). Highest precedence.
	Hang bool `json:"hang"`
	// Fail makes the process exit non-zero with a stderr line (self-test
	// asserts *provider.AgentError with Code==Fail). A zero value means "do not
	// fail". Precedence over Emit.
	Fail int `json:"fail"`
}

// StubConfig is the input to NewStubManifest: the behavior script (one entry
// per sequential agent invocation), the cross-process counter file path (the
// stateful seam; required for multi-entry scripts), and an optional stdin
// capture path. Script MUST be non-empty (NewStubManifest fatals otherwise).
type StubConfig struct {
	// Script is the per-call behavior script. Entry N drives the Nth
	// invocation; once exhausted the LAST entry is replayed (clamp).
	Script []StubResponse
	// StateFile is the counter file path threaded through STAGEHAND_STUB_STATE.
	// It MUST be set whenever Script has more than one entry (the binary
	// advances the counter on every call; without it every call selects entry
	// 0). A per-test t.TempDir() path is the usual choice.
	StateFile string
	// StdinLog is an optional capture path threaded through STAGEHAND_STUB_STDIN
	// so the delivered payload can be asserted (payload-delivery proof).
	StdinLog string
}

// Package-level build cache for BuildStubBinary. Compiling the stub binary is
// ~100ms+ and is needed by every self-test, so a single sync.Once builds it for
// the whole test binary run and caches the path (or the build error, so a
// failure is reported once and every dependent test fails fast on the same
// root cause rather than re-running the build).
var (
	stubBinOnce sync.Once
	stubBinPath string
	stubBinErr  error
)

// BuildStubBinary compiles internal/generate/testdata/stubagent into a temp
// binary once per test binary run and returns the cached path. The build uses
// the relative package path "./testdata/stubagent" (valid because a Go test's
// working directory is its package directory, i.e. internal/generate/). On
// build failure it t.Fatalf's with the combined go-build output and caches the
// error so subsequent callers fail identically. It is safe for concurrent use
// (sync.Once) and exported so the e2e/invariant suites can reuse it.
func BuildStubBinary(t testing.TB) string {
	t.Helper()
	stubBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "stagehand-stub-*")
		if err != nil {
			stubBinErr = err
			return
		}
		stubBinPath = filepath.Join(dir, "stubagent")
		cmd := exec.Command("go", "build", "-o", stubBinPath, "./testdata/stubagent")
		if out, err := cmd.CombinedOutput(); err != nil {
			stubBinErr = fmt.Errorf("go build stubagent: %w\n%s", err, out)
		}
	})
	if stubBinErr != nil {
		t.Fatalf("build stub binary: %v", stubBinErr)
	}
	return stubBinPath
}

// NewStubManifest wires a StubConfig into a provider.Manifest ready to drive
// the stub THROUGH provider.Executor. The config travels via the manifest's Env
// map (STAGEHAND_STUB_SCRIPT/_STATE/_STDIN), which the executor appends LAST to
// os.Environ() — so it is per-invocation and isolated, NEVER process-global
// (os.Setenv would leak across sibling subtests; Gotcha #2). PromptDelivery is
// provider.DeliveryStdin so the payload is piped to the stub's stdin (the
// payload-delivery + stdin-capture path). Script MUST be non-empty.
func NewStubManifest(t testing.TB, cfg StubConfig) provider.Manifest {
	t.Helper()
	if len(cfg.Script) == 0 {
		t.Fatal("NewStubManifest: cfg.Script must be non-empty")
	}
	b, err := json.Marshal(cfg.Script)
	if err != nil {
		t.Fatalf("marshal stub script: %v", err)
	}
	env := map[string]string{"STAGEHAND_STUB_SCRIPT": string(b)}
	if cfg.StateFile != "" {
		env["STAGEHAND_STUB_STATE"] = cfg.StateFile
	}
	if cfg.StdinLog != "" {
		env["STAGEHAND_STUB_STDIN"] = cfg.StdinLog
	}
	return provider.Manifest{
		Name:           "stub",
		Command:        BuildStubBinary(t),
		PromptDelivery: provider.DeliveryStdin,
		Env:            env,
	}
}

// TestStubProvider_EmitsCannedMessage proves the happy path through the REAL
// provider.Executor: a single Emit entry's message is returned byte-exactly on
// stdout. This is the foundational wiring proof — NewStubManifest's manifest
// execs the stub, the stub reads the env-driven script, and the canned message
// flows back through Run's stdout capture (FR24).
func TestStubProvider_EmitsCannedMessage(t *testing.T) {
	d := t.TempDir()
	m := NewStubManifest(t, StubConfig{
		Script:    []StubResponse{{Emit: "feat: x\n\nBody."}},
		StateFile: filepath.Join(d, "c"),
	})

	out, err := provider.NewExecutor("").Run(context.Background(), m, "", "", "", "PAYLOAD")
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if out != "feat: x\n\nBody." {
		t.Errorf("stdout = %q, want %q (canned Emit must be returned byte-exactly)", out, "feat: x\n\nBody.")
	}
}

// TestStubProvider_EmptyThenValidIsStateful proves the inner parse-correction
// retry wiring: a 2-entry script [{Emit:""},{Emit:"feat: ok"}] must emit empty
// on call 1 and the message on call 2 — i.e. the cross-process counter file
// advanced across two SEPARATE Run calls (each Run is a fresh process). This is
// the contract the generate inner loop (parse-correction retry) relies on: the
// first call produces an unparseable (empty) message, the retry call produces a
// valid one.
func TestStubProvider_EmptyThenValidIsStateful(t *testing.T) {
	state := filepath.Join(t.TempDir(), "c")
	m := NewStubManifest(t, StubConfig{
		Script:    []StubResponse{{Emit: ""}, {Emit: "feat: ok"}},
		StateFile: state,
	})
	ex := provider.NewExecutor("")

	// Call 1: counter reads 0 (file absent) → entry 0 → Emit "".
	out1, err := ex.Run(context.Background(), m, "", "", "", "PAYLOAD")
	if err != nil {
		t.Fatalf("call 1 Run error: %v", err)
	}
	if out1 != "" {
		t.Errorf("call 1 stdout = %q, want %q (empty Emit on the first entry)", out1, "")
	}

	// Call 2: counter advanced to 1 → entry 1 → Emit "feat: ok".
	out2, err := ex.Run(context.Background(), m, "", "", "", "PAYLOAD")
	if err != nil {
		t.Fatalf("call 2 Run error: %v", err)
	}
	if out2 != "feat: ok" {
		t.Errorf("call 2 stdout = %q, want %q (valid Emit on the second entry, proving the counter advanced)", out2, "feat: ok")
	}
}

// TestStubProvider_DupSubjectThenUnique proves the outer duplicate-rejection
// loop wiring: a script that emits a (dup) subject then a unique one must emit
// them in order across two Run calls. generate.CommitStaged's outer loop calls
// the agent once, sees the subject duplicates a recent commit, rejects, and
// calls again — this test asserts the stub serves the two entries sequentially
// (the harness the outer loop drives).
func TestStubProvider_DupSubjectThenUnique(t *testing.T) {
	state := filepath.Join(t.TempDir(), "c")
	m := NewStubManifest(t, StubConfig{
		Script:    []StubResponse{{Emit: "feat: dup"}, {Emit: "feat: unique"}},
		StateFile: state,
	})
	ex := provider.NewExecutor("")

	out1, err := ex.Run(context.Background(), m, "", "", "", "PAYLOAD")
	if err != nil {
		t.Fatalf("call 1 Run error: %v", err)
	}
	if out1 != "feat: dup" {
		t.Errorf("call 1 stdout = %q, want %q (the dup subject first)", out1, "feat: dup")
	}

	out2, err := ex.Run(context.Background(), m, "", "", "", "PAYLOAD")
	if err != nil {
		t.Fatalf("call 2 Run error: %v", err)
	}
	if out2 != "feat: unique" {
		t.Errorf("call 2 stdout = %q, want %q (the unique subject second, proving in-order emission)", out2, "feat: unique")
	}
}

// TestStubProvider_HangExercisesTimeout proves the real process-group timeout
// path: a [{Hang:true}] script blocks forever (select{}), so a short ctx
// deadline triggers the executor's SIGTERM+SIGKILL group kill and yields a
// *provider.TimeoutError that wraps context.DeadlineExceeded. This mirrors
// executor_test.go's TestRun_TimeoutKillsGroup — the stub is the realistic
// analog of /bin/sleep that an agent timeout would actually hit.
func TestStubProvider_HangExercisesTimeout(t *testing.T) {
	m := NewStubManifest(t, StubConfig{
		Script:    []StubResponse{{Hang: true}},
		StateFile: filepath.Join(t.TempDir(), "c"),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	out, err := provider.NewExecutor("").Run(ctx, m, "", "", "", "")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("Run returned nil error; want *provider.TimeoutError (out=%q)", out)
	}
	var te *provider.TimeoutError
	if !errors.As(err, &te) {
		t.Fatalf("Run error is %T; want *provider.TimeoutError", err)
	}
	// TimeoutError MUST wrap context.DeadlineExceeded so callers can treat
	// timeouts uniformly via errors.Is (executor.go's documented contract).
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("errors.Is(err, context.DeadlineExceeded) = false; want true (TimeoutError must wrap it)")
	}
	// select{} falls to SIGTERM fast (well under the 2s grace); elapsed should
	// be ~100ms. A regression to "group kill broken, waited the full 2s grace"
	// would blow past 1s.
	if elapsed >= time.Second {
		t.Errorf("elapsed = %v; want < 1s (proves the group kill felled the blocking stub fast)", elapsed)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty on timeout", out)
	}
}

// TestStubProvider_FailsWithAgentError proves the non-zero-exit path: a
// [{Fail:7}] script exits 7 with a stderr line, yielding a *provider.AgentError
// carrying Code==7. This is the contract generate's rescue routing keys off
// (decisions.md §3): a typed AgentError selects the agent-failure branch.
func TestStubProvider_FailsWithAgentError(t *testing.T) {
	m := NewStubManifest(t, StubConfig{
		Script:    []StubResponse{{Fail: 7}},
		StateFile: filepath.Join(t.TempDir(), "c"),
	})

	out, err := provider.NewExecutor("").Run(context.Background(), m, "", "", "", "")
	if err == nil {
		t.Fatalf("Run returned nil error; want *provider.AgentError (out=%q)", out)
	}
	var ae *provider.AgentError
	if !errors.As(err, &ae) {
		t.Fatalf("Run error is %T; want *provider.AgentError", err)
	}
	if ae.Code != 7 {
		t.Errorf("AgentError.Code = %d, want 7", ae.Code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty on non-zero exit", out)
	}
}

// TestStubProvider_RecordsStdin proves payload delivery: with StdinLog set, the
// bytes the executor pipes to the stub's stdin (the rendered prompt payload)
// are captured to the log file. This is the assertion the payload-ordering
// invariant tests (M6.T3.S3) build on — feeding a diff-shaped payload and
// asserting it reached the agent.
func TestStubProvider_RecordsStdin(t *testing.T) {
	d := t.TempDir()
	log := filepath.Join(d, "stdin")
	m := NewStubManifest(t, StubConfig{
		Script:    []StubResponse{{Emit: "x"}},
		StateFile: filepath.Join(d, "c"),
		StdinLog:  log,
	})

	const payload = "DIFF-BODY\n\nGenerate a commit message"
	if _, err := provider.NewExecutor("").Run(context.Background(), m, "", "", "", payload); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	got, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("read stdin log %s: %v", log, err)
	}
	if !strings.Contains(string(got), "DIFF-BODY") {
		t.Errorf("stdin log = %q; want it to contain the delivered payload %q", string(got), "DIFF-BODY")
	}
	if !strings.Contains(string(got), payload) {
		t.Errorf("stdin log = %q; want the full payload %q (stdin must be captured byte-exactly)", string(got), payload)
	}
}

// TestStubProvider_ClampsOverflowToLastEntry proves the clamp contract
// (Gotcha #3): once the script is exhausted the LAST entry is replayed on every
// subsequent call (NOT wrapped to entry 0, which would silently change meaning
// and break determinism). This guards a 3-call sequence against a 2-entry
// script: calls 1 and 2 hit entries 0 and 1, call 3 clamps to entry 1 again.
func TestStubProvider_ClampsOverflowToLastEntry(t *testing.T) {
	state := filepath.Join(t.TempDir(), "c")
	m := NewStubManifest(t, StubConfig{
		Script:    []StubResponse{{Emit: "first"}, {Emit: "second"}},
		StateFile: state,
	})
	ex := provider.NewExecutor("")

	out1, err := ex.Run(context.Background(), m, "", "", "", "")
	if err != nil {
		t.Fatalf("call 1 Run error: %v", err)
	}
	if out1 != "first" {
		t.Errorf("call 1 stdout = %q, want %q", out1, "first")
	}

	out2, err := ex.Run(context.Background(), m, "", "", "", "")
	if err != nil {
		t.Fatalf("call 2 Run error: %v", err)
	}
	if out2 != "second" {
		t.Errorf("call 2 stdout = %q, want %q", out2, "second")
	}

	// Call 3 overflows: index 2 >= len 2 → clamp to entry 1 ("second"). A wrap
	// would yield "first" — caught here.
	out3, err := ex.Run(context.Background(), m, "", "", "", "")
	if err != nil {
		t.Fatalf("call 3 Run error: %v", err)
	}
	if out3 != "second" {
		t.Errorf("call 3 stdout = %q, want %q (clamp on overflow must replay the LAST entry, not wrap)", out3, "second")
	}
}
