package stubtest

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/dabstractor/stagecoach/internal/provider"
)

func TestStub_EchoSuccess(t *testing.T) {
	bin := Build(t)
	m := Manifest(bin, Options{Out: "feat: add x"})
	spec, err := m.Render("", "", "fake prompt payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out, _, err := provider.Execute(context.Background(), *spec, provider.Deadlines{Hard: 5 * time.Second}, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "feat: add x" {
		t.Errorf("stdout = %q, want %q", out, "feat: add x")
	}
}

func TestStub_MultilineOut(t *testing.T) {
	bin := Build(t)
	want := "subject\n\nbody line"
	m := Manifest(bin, Options{Out: want})
	spec, err := m.Render("", "", "payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out, _, err := provider.Execute(context.Background(), *spec, provider.Deadlines{Hard: 5 * time.Second}, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestStub_NonZeroExit(t *testing.T) {
	bin := Build(t)
	m := Manifest(bin, Options{Out: "", Exit: 1})
	spec, err := m.Render("", "", "payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	_, _, err = provider.Execute(context.Background(), *spec, provider.Deadlines{Hard: 5 * time.Second}, nil)
	if err == nil {
		t.Fatal("err = nil, want non-nil (exit 1)")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("err = %T, want *exec.ExitError", err)
	}
}

func TestStub_TimeoutKilled(t *testing.T) {
	bin := Build(t)
	m := Manifest(bin, Options{Out: "", SleepMS: 2000})
	spec, err := m.Render("", "", "payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	start := time.Now()
	var execErr error
	_, _, execErr = provider.Execute(context.Background(), *spec, provider.Deadlines{Hard: 200 * time.Millisecond}, nil)
	elapsed := time.Since(start)
	err = execErr
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Execute took %v; process-group kill should have fired within ~3s", elapsed)
	}
}

func TestStub_StderrCapture(t *testing.T) {
	bin := Build(t)
	m := Manifest(bin, Options{Out: "", Stderr: "boom"})
	spec, err := m.Render("", "", "payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out, errb, err := provider.Execute(context.Background(), *spec, provider.Deadlines{Hard: 5 * time.Second}, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if errb != "boom" {
		t.Errorf("stderr = %q, want %q", errb, "boom")
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
}

func TestStub_DrainStdinNoDeadlock(t *testing.T) {
	bin := Build(t)
	m := Manifest(bin, Options{Out: "ok", SleepMS: 500})
	spec, err := m.Render("", "", "payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	// Override Stdin with a 1 MiB payload to pin drain-before-sleep (§4).
	spec.Stdin = strings.Repeat("x", 1<<20)
	out, _, err := provider.Execute(context.Background(), *spec, provider.Deadlines{Hard: 10 * time.Second}, nil)
	if err != nil {
		t.Fatalf("Execute: %v (possible deadlock — stub must drain stdin before sleeping)", err)
	}
	if out != "ok" {
		t.Errorf("stdout = %q, want %q", out, "ok")
	}
}

func TestStub_ScriptCallVarying(t *testing.T) {
	bin := Build(t)
	m := NewScript(t, bin, []string{"feat: dup", "feat: fresh"})

	// Call 1: first response.
	spec, err := m.Render("", "", "payload1", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out, _, err := provider.Execute(context.Background(), *spec, provider.Deadlines{Hard: 5 * time.Second}, nil)
	if err != nil {
		t.Fatalf("Execute call 1: %v", err)
	}
	if out != "feat: dup" {
		t.Errorf("call 1 stdout = %q, want %q", out, "feat: dup")
	}

	// Call 2: second response.
	spec2, err2 := m.Render("", "", "payload2", "off")
	if err2 != nil {
		t.Fatalf("Render: %v", err2)
	}
	out, _, err = provider.Execute(context.Background(), *spec2, provider.Deadlines{Hard: 5 * time.Second}, nil)
	if err != nil {
		t.Fatalf("Execute call 2: %v", err)
	}
	if out != "feat: fresh" {
		t.Errorf("call 2 stdout = %q, want %q", out, "feat: fresh")
	}

	// Call 3: clamps to last.
	spec3, err3 := m.Render("", "", "payload3", "off")
	if err3 != nil {
		t.Fatalf("Render: %v", err3)
	}
	out, _, err = provider.Execute(context.Background(), *spec3, provider.Deadlines{Hard: 5 * time.Second}, nil)
	if err != nil {
		t.Fatalf("Execute call 3: %v", err)
	}
	if out != "feat: fresh" {
		t.Errorf("call 3 stdout = %q, want %q (clamp to last)", out, "feat: fresh")
	}
}

func TestStub_ScriptBlankIsParseFailure(t *testing.T) {
	bin := Build(t)
	m := NewScript(t, bin, []string{"", "feat: good"})

	// Call 1: blank → empty stdout → ParseOutput ok=false.
	spec, err := m.Render("", "", "payload1", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out, _, err := provider.Execute(context.Background(), *spec, provider.Deadlines{Hard: 5 * time.Second}, nil)
	if err != nil {
		t.Fatalf("Execute call 1: %v", err)
	}
	if out != "" {
		t.Errorf("call 1 stdout = %q, want empty", out)
	}

	// Call 2: the good response.
	spec, err2 := m.Render("", "", "payload2", "off")
	if err2 != nil {
		t.Fatalf("Render: %v", err2)
	}
	out, _, err = provider.Execute(context.Background(), *spec, provider.Deadlines{Hard: 5 * time.Second}, nil)
	if err != nil {
		t.Fatalf("Execute call 2: %v", err)
	}
	if out != "feat: good" {
		t.Errorf("call 2 stdout = %q, want %q", out, "feat: good")
	}
}

func TestStub_MalformedEnvNoPanic(t *testing.T) {
	bin := Build(t)
	// Bypass Manifest/Render — build a raw CmdSpec with a malformed EXIT value.
	spec := provider.CmdSpec{
		Command: bin,
		Stdin:   "payload",
		Env:     append(os.Environ(), "STAGECOACH_STUB_EXIT=not-a-number", "STAGECOACH_STUB_OUT=x"),
	}
	out, _, err := provider.Execute(context.Background(), spec, provider.Deadlines{Hard: 5 * time.Second}, nil)
	if err != nil {
		t.Fatalf("Execute: %v (malformed EXIT should not cause non-zero exit)", err)
	}
	if out != "x" {
		t.Errorf("stdout = %q, want %q", out, "x")
	}
}

// TestStub_EmitThenAnswer (P1.M3.T1.S1, persona b — bounded) proves the emit phase ticks stdout for
// EMIT_TOTAL_MS and THEN the final answer arrives as the stdout SUFFIX (ticks prefix + answer).
func TestStub_EmitThenAnswer(t *testing.T) {
	bin := Build(t)
	m := Manifest(bin, Options{Out: "feat: answer", EmitMS: 50, EmitTotalMS: 300})
	spec, err := m.Render("", "", "payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	start := time.Now()
	out, _, err := provider.Execute(context.Background(), *spec, provider.Deadlines{Hard: 5 * time.Second}, nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.HasSuffix(out, "feat: answer") {
		t.Errorf("stdout = %q, want suffix %q (the answer must be emitted AFTER the ticks)", out, "feat: answer")
	}
	if strings.Count(out, ".") < 1 {
		t.Errorf("stdout = %q, want >=1 emit tick (default payload \".\")", out)
	}
	if elapsed < 300*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 300ms (the emit bound must elapse before the answer)", elapsed)
	}
}

// TestStub_EmitCadence (P1.M3.T1.S1) proves the emit interval is honored: 280ms / 40ms ≈ 7 ticks,
// asserted over a slack-tolerant window (never an exact count — scheduler/CI variance).
func TestStub_EmitCadence(t *testing.T) {
	bin := Build(t)
	m := Manifest(bin, Options{Out: "feat: x", EmitMS: 40, EmitTotalMS: 280})
	spec, err := m.Render("", "", "payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out, _, err := provider.Execute(context.Background(), *spec, provider.Deadlines{Hard: 5 * time.Second}, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if ticks := strings.Count(out, "."); ticks < 6 || ticks > 9 {
		t.Errorf("tick count = %d, want in [6, 9] (280ms / 40ms ≈ 7; slack-tolerant window)", ticks)
	}
	if !strings.HasSuffix(out, "feat: x") {
		t.Errorf("stdout = %q, want suffix %q", out, "feat: x")
	}
}

// TestStub_EmitForeverHardCapKilled (P1.M3.T1.S1, persona b — unbounded) proves EMIT_TOTAL_MS==0 ⇒
// ticks forever and NEVER answers: the hard cap kills it, the PARTIAL ticks are still captured in
// the returned stdout buffer (§9.6 partial-capture contract), and the answer is absent.
func TestStub_EmitForeverHardCapKilled(t *testing.T) {
	bin := Build(t)
	m := Manifest(bin, Options{Out: "feat: never", EmitMS: 50}) // no total ⇒ emit forever, never answer
	spec, err := m.Render("", "", "payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	start := time.Now()
	out, _, err := provider.Execute(context.Background(), *spec, provider.Deadlines{Hard: 250 * time.Millisecond}, nil)
	elapsed := time.Since(start)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded (the hard cap must kill the unbounded emitter)", err)
	}
	if !strings.Contains(out, ".") {
		t.Errorf("partial stdout = %q, want >=1 tick (§9.6: the buffer returns even on kill)", out)
	}
	if strings.Contains(out, "feat: never") {
		t.Errorf("unbounded emit must NEVER answer; stdout = %q", out)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Execute took %v; process-group kill should have fired promptly", elapsed)
	}
}

// TestStub_HeartbeatHardCapKilled (P1.M3.T1.S1, persona c — the false-alive pathology) proves an
// unbounded stderr heartbeat is killed ONLY by the hard cap: DeadlineExceeded + stderr ticks +
// stdout EMPTY (heartbeat mode never answers).
func TestStub_HeartbeatHardCapKilled(t *testing.T) {
	bin := Build(t)
	m := Manifest(bin, Options{Out: "feat: never", HeartbeatMS: 50}) // forever
	spec, err := m.Render("", "", "payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	start := time.Now()
	out, errb, err := provider.Execute(context.Background(), *spec, provider.Deadlines{Hard: 250 * time.Millisecond}, nil)
	elapsed := time.Since(start)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded (only the hard cap can kill a heartbeat)", err)
	}
	if !strings.Contains(errb, "tick") {
		t.Errorf("stderr = %q, want >=1 heartbeat tick", errb)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty (heartbeat mode NEVER answers)", out)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Execute took %v; process-group kill should have fired promptly", elapsed)
	}
}

// TestStub_HeartbeatBoundedExit (P1.M3.T1.S1, persona c — bounded) proves the heartbeat loop ends
// normally at HEARTBEAT_TOTAL_MS: nil error, stdout still empty (never answers), stderr ticks seen.
func TestStub_HeartbeatBoundedExit(t *testing.T) {
	bin := Build(t)
	m := Manifest(bin, Options{Out: "feat: never", HeartbeatMS: 20, HeartbeatTotalMS: 100})
	spec, err := m.Render("", "", "payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out, errb, err := provider.Execute(context.Background(), *spec, provider.Deadlines{Hard: 5 * time.Second}, nil)
	if err != nil {
		t.Fatalf("Execute: %v (the bounded heartbeat should exit normally)", err)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty (heartbeat mode NEVER answers even when bounded)", out)
	}
	if !strings.Contains(errb, "tick") {
		t.Errorf("stderr = %q, want >=1 heartbeat tick", errb)
	}
}

// TestStub_EmitZeroConfigDefault (P1.M3.T1.S1) is the byte-identical-default guard: with none of the
// emit/heartbeat knobs set, the stub behaves exactly as before (no ticks, no extra bytes).
func TestStub_EmitZeroConfigDefault(t *testing.T) {
	bin := Build(t)
	m := Manifest(bin, Options{Out: "feat: plain"})
	spec, err := m.Render("", "", "payload", "off")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	out, _, err := provider.Execute(context.Background(), *spec, provider.Deadlines{Hard: 5 * time.Second}, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "feat: plain" {
		t.Errorf("stdout = %q, want EXACTLY %q (zero-config default must stay byte-identical — no ticks leaked)", out, "feat: plain")
	}
}
