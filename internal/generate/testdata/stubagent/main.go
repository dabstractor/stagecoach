// Command stubagent is the fake coding-agent CLI used by the generate
// integration test harness (P1.M6.T3.S1). It is a stdlib-only `package main`
// binary that lives under testdata/ — and so is EXCLUDED from the shipped
// `go build ./...` output and the package's normal build/vet (test data is
// only compiled by the explicit gate `go build ./internal/generate/testdata/
// stubagent`). It replaces the real LLM agent entirely (PRD §20.1 layer 3;
// decisions.md §8) so the e2e suite (M6.T3.S2) and the §18.1 invariant tests
// (M6.T3.S3) can drive generate.CommitStaged with deterministic, scriptable
// behavior.
//
// The binary is stateless in memory but STATEFUL across invocations via a
// cross-process counter FILE (each agent call is a fresh process, so in-memory
// state cannot persist — reference_impl.md §2 / decisions.md §3's two-nested-
// loop needs the stub to advance across both the inner parse-retry calls and
// the outer dup-rejection calls). Configuration reaches the binary through
// THREE environment variables (NOT process-global; the test harness sets them
// via provider.Manifest.Env so each invocation is isolated — Gotcha #2):
//
//   - STAGEHAND_STUB_SCRIPT — a JSON array of stubResponse entries describing
//     the per-call behavior "script". The Nth invocation selects entry N.
//     When unset/empty a single valid canned message is assumed.
//   - STAGEHAND_STUB_STATE — a path to the counter file. The binary reads the
//     current index N, writes N+1 back BEFORE acting (so a follow-on process
//     observes the advance even if this one is killed), then selects
//     script[min(N, len-1)] — CLAMP on overflow, never wrap (determinism,
//     Gotcha #3).
//   - STAGEHAND_STUB_STDIN — an optional path to capture the delivered stdin
//     payload to, for the payload-delivery self-test.
//
// Per-entry precedence is Hang (block forever via select{} so the executor's
// process-group SIGTERM/SIGKILL fells it) > Fail>0 (stderr line + os.Exit)
// > Emit (write the message to stdout, exit 0).
//
// stubResponse's JSON tags ({emit,hang,fail}) MUST mirror
// generate.StubResponse EXACTLY: the test harness json.Marshal's a
// []generate.StubResponse into STAGEHAND_STUB_SCRIPT and this binary
// json.Unmarshal's the same bytes into []stubResponse, so a tag drift is a
// silent wrong-behavior bug (coupling gotcha).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// stubResponse is the per-invocation behavior descriptor. Its JSON tags
// mirror generate.StubResponse (emit/hang/fail) so the bytes the harness
// marshals are the bytes this binary unmarshals.
type stubResponse struct {
	// Emit is the canned agent output written to stdout when neither Hang nor
	// Fail is active. It models a generated commit message (subject + body).
	Emit string `json:"emit"`
	// Hang blocks the process forever (select{}) so the executor's context
	// deadline triggers the real process-group kill path (self-test asserts
	// *provider.TimeoutError). It takes precedence over Fail and Emit.
	Hang bool `json:"hang"`
	// Fail makes the process exit non-zero with a stderr line, modeling a
	// crashed/misconfigured agent. It takes precedence over Emit. A zero value
	// means "do not fail" (so the default zero entry emits "" — empty, the
	// parse-correction retry trigger).
	Fail int `json:"fail"`
}

func main() {
	// (1) Drain stdin FIRST — the executor pipes the payload via stdin and the
	// pipe MUST be drained before the process acts/exits, or os/exec's write
	// goroutine blocks (Gotcha). Capture it to STAGEHAND_STUB_STDIN when set
	// (payload-delivery proof for TestStubProvider_RecordsStdin).
	in, _ := io.ReadAll(os.Stdin)
	if p := os.Getenv("STAGEHAND_STUB_STDIN"); p != "" {
		_ = os.WriteFile(p, in, 0o644)
	}

	// (2) Parse the behavior script. Default to a single valid canned message
	// when STAGEHAND_STUB_SCRIPT is unset/empty (so a misconfigured manifest
	// still emits a sane message, not a crash). A malformed script is a hard
	// error: print to stderr and exit non-zero (the harness never feeds a
	// malformed script, but failing loudly beats silent wrong behavior).
	script := []stubResponse{{Emit: "feat: stubbed commit message"}}
	if s := os.Getenv("STAGEHAND_STUB_SCRIPT"); s != "" {
		if err := json.Unmarshal([]byte(s), &script); err != nil {
			fmt.Fprintf(os.Stderr, "stubagent: invalid STAGEHAND_STUB_SCRIPT: %v\n", err)
			os.Exit(1)
		}
	}

	// (3) Read the cross-process index N from STAGEHAND_STUB_STATE and write
	// N+1 back BEFORE acting. Each agent call is a fresh process, so in-memory
	// state cannot carry across calls — the counter FILE is the stateful seam.
	// Writing the advance first means a subsequent process observes it even if
	// this one is killed (e.g. by a Hang timeout).
	idx := 0
	if sp := os.Getenv("STAGEHAND_STUB_STATE"); sp != "" {
		if b, err := os.ReadFile(sp); err == nil {
			if n, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
				idx = n
			}
		}
		_ = os.WriteFile(sp, []byte(strconv.Itoa(idx+1)), 0o644)
	}

	// CLAMP on overflow (never wrap): once the script is exhausted, keep
	// replaying the LAST entry. This keeps behavior deterministic — a wrap
	// would silently change the script's tail meaning (Gotcha #3).
	if idx >= len(script) {
		idx = len(script) - 1
	}
	r := script[idx]

	// (4) Act with precedence Hang > Fail>0 > Emit.
	if r.Hang {
		// Block forever so the executor's ctx-driven SIGTERM+SIGKILL on the
		// process group is the only exit — mirroring /bin/sleep in the
		// executor tests. A bare select{} would trip Go's all-goroutines-
		// asleep deadlock detector and panic-exit (code 2) BEFORE the timeout
		// could fire, surfacing as an *AgentError instead of *TimeoutError.
		// A read syscall is immune to the deadlock detector (a syscall-blocked
		// goroutine is in _Gsyscall, skipped by checkdead), so we block on a
		// pipe whose write end is never closed: the read never sees EOF.
		hangForever()
	}
	if r.Fail > 0 {
		fmt.Fprintf(os.Stderr, "stubagent: simulated agent failure (exit %d)\n", r.Fail)
		os.Exit(r.Fail)
	}
	fmt.Fprint(os.Stdout, r.Emit)
}

// hangForever blocks the process indefinitely on a pipe read whose write end
// is intentionally never closed, so the read never receives EOF. The block is
// in a read syscall (via io.Copy), which the Go runtime's deadlock detector
// ignores (syscall-blocked goroutines are not counted as "asleep"), unlike a
// bare select{} or <-chan that would fatal-panic with
// "all goroutines are asleep - deadlock!". The only exit is a signal
// (SIGTERM/SIGKILL from the executor's process-group kill on ctx timeout),
// whose default Go disposition terminates the process.
func hangForever() {
	r, w, err := os.Pipe()
	if err != nil {
		os.Exit(1)
	}
	_ = w                         // keep the write end reachable so it is never closed/GC'd mid-scope
	_, _ = io.Copy(io.Discard, r) // blocks forever on the read syscall
}
