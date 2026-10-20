// generate_eventstream_test.go is the event-mode e2e proof (§9.30 FR-S1 runtime half + FR-S3):
// through the REAL Render → Execute → ParseOutput seam with the stubagent progressive-emit persona,
// it proves
//
//  1. (happy + FR-A3's portable payoff) progressive JSONL ticks + a terminal event ⇒ the run
//     SURVIVES a 1s idle deadline across a ~2.5s generation on EVERY platform (each 50ms tick
//     stamps the pipe-activity channel — armed by Render because the manifest declares
//     event_stream; on macOS/Windows that arming is the ONLY activity channel), and ParseOutput
//     assembles the commit message from the terminal event;
//  2. (FR-S3) an in-stream error event with EXIT 0 (pi documents that failures do NOT reliably
//     exit nonzero) ⇒ ParseOutput ok==false (the existing FR29 retry), EventErrorHint carries the
//     event text while StderrHint structurally cannot (stdout is non-empty), and the full
//     CommitStaged loop surfaces the event text as the RescueError.Cause;
//  3. (guard regression) S1's FR-S2 render guard — event_stream without a registered extractor
//     errors — is un-weakened from this package's perspective.
//
// This is an EXTERNAL test (package generate_test) because internal/stubtest imports
// internal/provider — a same-package provider test CANNOT import stubtest (import cycle), so the
// e2e rides here (the idle_parity_test.go precedent; initTempRepo/stageChange are shared from that
// package's files).
package generate_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/generate"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/provider"
	"github.com/dabstractor/stagecoach/internal/stubtest"
	"github.com/dabstractor/stagecoach/internal/ui"
)

// streamFakeExtractor classifies the stub's JSONL persona: {"type":"result","message":...} is the
// terminal event, {"type":"error","message":...} an in-stream error (FR-S3), everything else
// (progress ticks) is ignored — the lenient per-line contract (stateless; registry singletons are
// shared by concurrent roles).
type streamFakeExtractor struct{}

func (streamFakeExtractor) Extract(line string) provider.EventOutcome {
	var ev struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		return provider.EventOutcome{Kind: provider.EventIgnored}
	}
	switch ev.Type {
	case "result":
		return provider.EventOutcome{Kind: provider.EventTerminal, Message: ev.Message}
	case "error":
		return provider.EventOutcome{Kind: provider.EventError, ErrText: ev.Message}
	default:
		return provider.EventOutcome{Kind: provider.EventIgnored}
	}
}

// registerStreamExtractor installs the fake under name; cleanup nils the entry (Lookup then
// returns nil — the same "unregistered" condition — since the map itself is unexported).
func registerStreamExtractor(t *testing.T, name string) {
	t.Helper()
	provider.RegisterEventExtractor(name, streamFakeExtractor{})
	t.Cleanup(func() { provider.RegisterEventExtractor(name, nil) })
}

// eventStubManifest builds the stub manifest in EVENT MODE: Name keyed to the registered
// extractor, EventStream non-empty (opaque tokens to the executor — what matters is that Render
// arms spec.ProgressiveStdout and ParseOutput takes the event branch), print_flag set so the
// token-order assertion (event tokens STRICTLY before print_flag, §12.2) is observable.
func eventStubManifest(t *testing.T, bin, name string, opts stubtest.Options) provider.Manifest {
	t.Helper()
	registerStreamExtractor(t, name)
	m := stubtest.Manifest(bin, opts)
	m.Name = name
	m.EventStream = []string{"--mode", "json"}
	m.PrintFlag = strPtr("--print")
	return m
}

// strPtr — the stubtest pointer helpers are unexported; this external package needs its own.
func strPtr(s string) *string { return &s }

// idx returns the first index of token in args, or -1.
func idx(args []string, token string) int {
	for i, a := range args {
		if a == token {
			return i
		}
	}
	return -1
}

// TestEventStream_HappyPath_PortableIdleSurvival is FR-A3's portable payoff: a ~2.5s generation
// emitting a 50ms JSONL tick cadence SURVIVES a 1s idle deadline — on every platform, because
// Render armed spec.ProgressiveStdout (the executor's pipe-write channel) when the manifest
// declared event_stream. ParseOutput then assembles the message from the terminal event.
func TestEventStream_HappyPath_PortableIdleSurvival(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow event-stream idle-survival test in -short mode")
	}
	bin := stubtest.Build(t)
	const name = "eventstream-e2e-happy"
	m := eventStubManifest(t, bin, name, stubtest.Options{
		Out:         `{"type":"result","message":"feat: add widget"}`,
		EmitMS:      50,
		EmitLine:    "{\"type\":\"progress\",\"n\":1}\n", // trailing \n ⇒ each tick is its own NDJSON line
		EmitTotalMS: 2500,                                // bounded ⇒ the answer arrives as the stdout suffix
	})

	spec, rerr := m.Render("", "", "generate a commit message", "", provider.RenderBare)
	if rerr != nil {
		t.Fatalf("Render: %v", rerr)
	}
	if !spec.ProgressiveStdout {
		t.Error("spec.ProgressiveStdout = false, want true (event_stream must arm the progressive channel)")
	}
	// §12.2 order: event tokens STRICTLY BEFORE print_flag (print_flag is ALWAYS LAST).
	if idx(spec.Args, "--mode") == -1 || idx(spec.Args, "json") == -1 {
		t.Errorf("event_stream tokens missing from Args: %v", spec.Args)
	}
	if !(idx(spec.Args, "--mode") < idx(spec.Args, "--print")) {
		t.Errorf("event tokens must precede print_flag: %v", spec.Args)
	}

	// The ~2.5s run vs a 1s idle clock: survives ONLY because each 50ms tick stamps the pipe
	// channel (on macOS/Windows solely via the ProgressiveStdout arming — FR-A3's payoff).
	var vbuf strings.Builder
	start := time.Now()
	out, _, execErr := provider.Execute(context.Background(), *spec,
		provider.Deadlines{Idle: time.Second, Hard: 30 * time.Second}, ui.NewVerbose(&vbuf, false))
	elapsed := time.Since(start)
	if execErr != nil {
		t.Fatalf("Execute = %v, want nil (the tick cadence must keep the idle clock alive for the full ~2.5s)", execErr)
	}
	if elapsed < 2*time.Second {
		t.Errorf("run took %v; want ≈2.5s (the stub must actually tick for the survival claim to mean anything)", elapsed)
	}

	msg, ok, _ := provider.ParseOutput(out, m)
	if !ok || msg != "feat: add widget" {
		t.Errorf("ParseOutput = (%q, %v), want the terminal event assembled (\"feat: add widget\", true)", msg, ok)
	}
}

// TestEventStream_ErrorEventExit0_FRS3 is THE FR-S3 proof: the stub emits an error event and exits
// 0 (the premise — pi documents that failed responses do NOT reliably exit nonzero). ParseOutput
// yields ok==false (the existing FR29 retry), the EventErrorHint carries the event text while
// StderrHint structurally cannot (stdout is non-empty in event mode), and the full CommitStaged
// loop surfaces the event text as the RescueError.Cause — not the opaque empty-parse failure.
func TestEventStream_ErrorEventExit0_FRS3(t *testing.T) {
	bin := stubtest.Build(t)
	const name = "eventstream-e2e-error"
	m := eventStubManifest(t, bin, name, stubtest.Options{
		Out: `{"type":"error","message":"usage limit exceeded"}`,
		// Exit unset ⇒ 0 (the stub default) — the FR-S3 premise. Setting Exit=1 would test the
		// ExitError path instead.
	})

	spec, rerr := m.Render("", "", "generate a commit message", "", provider.RenderBare)
	if rerr != nil {
		t.Fatalf("Render: %v", rerr)
	}
	out, stderr, execErr := provider.Execute(context.Background(), *spec,
		provider.Deadlines{Idle: time.Second, Hard: 30 * time.Second}, ui.NewVerbose(&strings.Builder{}, false))
	if execErr != nil {
		t.Fatalf("Execute = %v, want nil (the stub exits 0 — the FR-S3 premise)", execErr)
	}

	// (a) The event-mode parse: an error event is NOT a message ⇒ ok==false ⇒ FR29 retry.
	msg, ok, _ := provider.ParseOutput(out, m)
	if ok || msg != "" {
		t.Errorf("ParseOutput = (%q, %v), want (\"\", false) — an error event is not a commit message", msg, ok)
	}

	// (b) The precedence sibling surfaces the in-stream error; StderrHint cannot.
	hint := provider.EventErrorHint(out, m)
	if !strings.Contains(hint, "usage limit exceeded") {
		t.Errorf("EventErrorHint = %q, want it to carry the event text", hint)
	}
	if sh := provider.StderrHint(out, stderr); sh != "" {
		t.Errorf("StderrHint = %q, want \"\" (stdout is non-empty in event mode — the structural hole FR-S3 closes)", sh)
	}

	// (c) Through the REAL CommitStaged loop: the rescue's Cause carries the event text.
	repo := initTempRepo(t)
	stageChange(t, repo)
	_, err := generate.CommitStaged(context.Background(), generate.Deps{
		Git:      git.New(repo),
		Manifest: m,
		Verbose:  ui.NewVerbose(&strings.Builder{}, false),
	}, config.Defaults())
	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Fatalf("CommitStaged err = %T %v, want *generate.RescueError", err, err)
	}
	if re.Cause == nil || !strings.Contains(re.Cause.Error(), "usage limit exceeded") {
		t.Errorf("RescueError.Cause = %v, want the in-stream error event text (FR-S3)", re.Cause)
	}
}

// TestEventStream_RenderGuardUnweakened re-asserts S1's FR-S2 guard from THIS package's
// perspective: an event_stream manifest with NO registered extractor must fail at Render —
// an unverified event mode cannot ship (and cannot reach the parse below).
func TestEventStream_RenderGuardUnweakened(t *testing.T) {
	bin := stubtest.Build(t)
	m := stubtest.Manifest(bin, stubtest.Options{Out: "x"})
	m.Name = "eventstream-e2e-unregistered" // deliberately NOT registered
	m.EventStream = []string{"--mode", "json"}
	if _, rerr := m.Render("", "", "p", "", provider.RenderBare); rerr == nil {
		t.Fatal("Render err = nil, want the FR-S2 no-extractor guard error")
	}
}
