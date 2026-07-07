package signal

import (
	"bytes"
	"context"
	"os"
	"syscall"
	"testing"
)

// installTestHandler creates a Handler with the given options, stores it in the package-level
// active singleton, and resets active to nil when the test finishes. CRITICAL: prevents test
// poisoning (singleton state leaking between tests, especially with -race).
func installTestHandler(t *testing.T, opts Options) *Handler {
	t.Helper()
	ctx, h := Install(context.Background(), opts) // must use Background; nil panics (Go 1.22)
	t.Cleanup(func() {
		active.Store(nil) // reset singleton so other tests start clean
	})
	_ = ctx
	return h
}

// TestHandler_ForwardsToChildGroup verifies that a signal forwarded to a registered child PID
// calls the injectable Kill with the correct pid and signal.
func TestHandler_ForwardsToChildGroup(t *testing.T) {
	var killedPid int
	var killedSig os.Signal
	var exitCode int

	h := installTestHandler(t, Options{
		Kill: func(pid int, sig os.Signal) error {
			killedPid = pid
			killedSig = sig
			return nil
		},
		Exit: func(code int) { exitCode = code },
		Out:  new(bytes.Buffer),
	})

	RegisterChild(1234)
	h.handle(syscall.SIGINT)

	if killedPid != 1234 {
		t.Errorf("Kill pid = %d, want 1234", killedPid)
	}
	if killedSig != syscall.SIGINT {
		t.Errorf("Kill sig = %v, want SIGINT", killedSig)
	}
	if exitCode != 130 {
		t.Errorf("exitCode = %d, want 130 (no snapshot → exit 130)", exitCode)
	}
}

// TestHandler_RescueOnSignalWithSnapshot verifies that a signal with an armed snapshot prints
// the rescue message and exits 3.
func TestHandler_RescueOnSignalWithSnapshot(t *testing.T) {
	var exitCode int
	buf := &bytes.Buffer{}

	h := installTestHandler(t, Options{
		RescueFormat: func(tree, parent, cand string) string {
			return "RESCUE: Tree=" + tree + " Parent=" + parent + " Cand=" + cand
		},
		Exit: func(code int) { exitCode = code },
		Out:  buf,
	})

	SetSnapshot("abc123", "def456", "feat: my change")
	h.handle(syscall.SIGINT)

	if exitCode != 3 {
		t.Errorf("exitCode = %d, want 3", exitCode)
	}
	got := buf.String()
	if !contains(got, "Tree=abc123") {
		t.Errorf("rescue output missing Tree=abc123: %q", got)
	}
	if !contains(got, "Parent=def456") {
		t.Errorf("rescue output missing Parent=def456: %q", got)
	}
	if !contains(got, "Cand=feat: my change") {
		t.Errorf("rescue output missing Cand=feat: my change: %q", got)
	}
}

// TestHandler_Exit130PreSnapshot verifies that a signal WITHOUT an armed snapshot exits 130
// and prints no rescue message.
func TestHandler_Exit130PreSnapshot(t *testing.T) {
	var exitCode int
	buf := &bytes.Buffer{}

	h := installTestHandler(t, Options{
		Exit: func(code int) { exitCode = code },
		Out:  buf,
	})

	// No SetSnapshot call — snapshot is empty.
	h.handle(syscall.SIGINT)

	if exitCode != 130 {
		t.Errorf("exitCode = %d, want 130", exitCode)
	}
	if buf.Len() != 0 {
		t.Errorf("unexpected output (want empty): %q", buf.String())
	}
}

// TestHandler_Exit143SIGTERM verifies SIGTERM produces exit code 143.
func TestHandler_Exit143SIGTERM(t *testing.T) {
	var exitCode int

	h := installTestHandler(t, Options{
		Exit: func(code int) { exitCode = code },
		Out:  new(bytes.Buffer),
	})

	h.handle(syscall.SIGTERM)

	if exitCode != 143 {
		t.Errorf("exitCode = %d, want 143", exitCode)
	}
}

// TestHandler_RestoreDefaultStopsForward verifies that after RestoreDefault, handle is a no-op.
func TestHandler_RestoreDefaultStopsForward(t *testing.T) {
	var killCalled bool
	var exitCode int

	h := installTestHandler(t, Options{
		Kill: func(pid int, sig os.Signal) error { killCalled = true; return nil },
		Exit: func(code int) { exitCode = code },
		Out:  new(bytes.Buffer),
	})

	RegisterChild(9999)
	SetSnapshot("tree", "parent", "cand")
	RestoreDefault() // stop signal delivery

	h.handle(syscall.SIGINT)

	if killCalled {
		t.Error("Kill was called after RestoreDefault, want no-op")
	}
	if exitCode != 0 {
		t.Errorf("exitCode = %d, want 0 (handle should be no-op)", exitCode)
	}
}

// TestHandler_RestoreDefaultIdempotent verifies calling RestoreDefault twice doesn't panic.
func TestHandler_RestoreDefaultIdempotent(t *testing.T) {
	_ = installTestHandler(t, Options{
		Exit: func(code int) {},
		Out:  new(bytes.Buffer),
	})

	RestoreDefault()
	RestoreDefault() // should not panic
}

// TestHandler_NilWrappersNoOp verifies that all package wrappers are safe when no handler is
// installed (active == nil).
func TestHandler_NilWrappersNoOp(t *testing.T) {
	// Ensure no handler is installed.
	active.Store(nil)

	// These must not panic.
	RegisterChild(1234)
	ClearChild()
	SetSnapshot("tree", "parent", "cand")
	SetCandidate("msg")
	ClearSnapshot()
	RestoreDefault()

	if Active() != nil {
		t.Error("Active() should be nil")
	}
}

// TestHandler_SetCandidateUpdates verifies that SetCandidate updates the snapshot candidate
// without touching tree/parent, by checking the rescue format receives it.
func TestHandler_SetCandidateUpdates(t *testing.T) {
	var gotTree, gotCand string
	var exitCode int

	h := installTestHandler(t, Options{
		RescueFormat: func(tree, parent, cand string) string {
			gotTree = tree
			gotCand = cand
			return "rescue"
		},
		Exit: func(code int) { exitCode = code },
		Out:  new(bytes.Buffer),
	})

	SetSnapshot("t1", "p1", "old")
	SetCandidate("new candidate")

	h.handle(syscall.SIGINT)

	if exitCode != 3 {
		t.Fatalf("exitCode = %d, want 3", exitCode)
	}
	if gotTree != "t1" {
		t.Errorf("tree = %q, want t1", gotTree)
	}
	if gotCand != "new candidate" {
		t.Errorf("candidate = %q, want 'new candidate'", gotCand)
	}
}

// TestHandler_CancelContext verifies that Install returns a context cancelled when handle fires.
func TestHandler_CancelContext(t *testing.T) {
	buf := &bytes.Buffer{}

	ctx, _ := Install(context.Background(), Options{
		Kill: func(pid int, sig os.Signal) error { return nil },
		Exit: func(code int) {}, // don't actually exit
		Out:  buf,
	})
	t.Cleanup(func() { active.Store(nil) })

	select {
	case <-ctx.Done():
		t.Error("context should not be cancelled yet")
	default:
	}

	active.Load().handle(syscall.SIGINT)

	select {
	case <-ctx.Done():
		// expected — context should be cancelled
	default:
		t.Error("context should be cancelled after handle")
	}
}

// TestHandler_NoChildKill verifies that without a registered child, Kill is NOT called.
func TestHandler_NoChildKill(t *testing.T) {
	var killCalled bool
	var exitCode int

	h := installTestHandler(t, Options{
		Kill: func(pid int, sig os.Signal) error { killCalled = true; return nil },
		Exit: func(code int) { exitCode = code },
		Out:  new(bytes.Buffer),
	})

	// No RegisterChild call.
	h.handle(syscall.SIGINT)

	if killCalled {
		t.Error("Kill was called without a registered child, want no-op")
	}
	if exitCode != 130 {
		t.Errorf("exitCode = %d, want 130", exitCode)
	}
}

// TestHandler_OnRescueExit_PostSnapshot verifies the FR52 §18.5 exit-path-release seam (P1.M2.T2.S1): on the
// POST-SNAPSHOT signal branch (snapshot armed → Exit 3), OnRescueExit fires EXACTLY ONCE and BEFORE Exit. The
// ordering is proven by OnRescueExit setting a flag (rescueFired) that Exit reads — since handle() is synchronous
// in one goroutine, if Exit ran first the flag would still be false. (Contract a.)
func TestHandler_OnRescueExit_PostSnapshot(t *testing.T) {
	var rescueCalls int
	var rescueFired bool
	var exitCode int
	var exitSawRescueFired bool

	h := installTestHandler(t, Options{
		OnRescueExit: func() {
			rescueCalls++
			rescueFired = true
		},
		Exit: func(code int) {
			exitCode = code
			exitSawRescueFired = rescueFired // Exit "checks" the flag OnRescueExit set
		},
		Out: new(bytes.Buffer), // keep the rescue print off real stderr
	})

	SetSnapshot("tree", "parent", "cand") // arm the rescue path (snapTree != "" → post-snapshot branch)
	h.handle(os.Interrupt)                // direct call — no goroutine timing (handle() is extracted for testing)

	if rescueCalls != 1 {
		t.Errorf("OnRescueExit calls = %d, want 1 (post-snapshot exit path)", rescueCalls)
	}
	if exitCode != 3 {
		t.Errorf("Exit code = %d, want 3 (post-snapshot rescue)", exitCode)
	}
	if !exitSawRescueFired {
		t.Error("Exit observed rescueFired=false, want true — OnRescueExit must fire BEFORE Exit (FR52 exit-path release)")
	}
}

// TestHandler_OnRescueExit_PreSnapshot verifies the seam on the PRE-SNAPSHOT branch (no snapshot → Exit 130):
// OnRescueExit fires EXACTLY ONCE and BEFORE Exit. The lock is acquired at default_action.go:59 BEFORE the
// snapshot is armed, so a pre-snapshot Ctrl-C finds the lock HELD → the release must fire here too (both branches
// need it). Same ordering flag technique as the post-snapshot test. (Contract b.)
func TestHandler_OnRescueExit_PreSnapshot(t *testing.T) {
	var rescueCalls int
	var rescueFired bool
	var exitCode int
	var exitSawRescueFired bool

	h := installTestHandler(t, Options{
		OnRescueExit: func() {
			rescueCalls++
			rescueFired = true
		},
		Exit: func(code int) {
			exitCode = code
			exitSawRescueFired = rescueFired
		},
		Out: new(bytes.Buffer),
	})

	// NO SetSnapshot — snapTree == "" → pre-snapshot branch → Exit(exitCodeForSignal(os.Interrupt)) == 130.
	h.handle(os.Interrupt)

	if rescueCalls != 1 {
		t.Errorf("OnRescueExit calls = %d, want 1 (pre-snapshot exit path — lock held since default_action.go:59)", rescueCalls)
	}
	if exitCode != 130 {
		t.Errorf("Exit code = %d, want 130 (SIGINT, pre-snapshot)", exitCode)
	}
	if !exitSawRescueFired {
		t.Error("Exit observed rescueFired=false, want true — OnRescueExit must fire BEFORE Exit (FR52 exit-path release)")
	}
}

// TestHandler_OnRescueExit_SkippedAfterRestoreDefault verifies that after RestoreDefault the handler is STOPPED:
// handle() returns at its first line (if h.stopped.Load()) and never reaches OnRescueExit or Exit. This is correct
// — RestoreDefault is the SUCCESS path (before update-ref; no os.Exit) → defer locker.Release() runs normally, so
// the exit-path seam must NOT fire (it would be a redundant double-release). Asserts OnRescueExit NOT called AND
// Exit NOT called. (Contract c. "No real lock needed — the seam is a recorder.")
func TestHandler_OnRescueExit_SkippedAfterRestoreDefault(t *testing.T) {
	var rescueCalls int
	var exitCalled bool

	h := installTestHandler(t, Options{
		OnRescueExit: func() { rescueCalls++ },
		Exit:         func(int) { exitCalled = true },
		Out:          new(bytes.Buffer),
	})

	RestoreDefault() // stop signal delivery — handler is now inert (h.stopped == true)
	h.handle(os.Interrupt)

	if rescueCalls != 0 {
		t.Errorf("OnRescueExit calls = %d, want 0 (handler stopped after RestoreDefault; success path uses defer Release)", rescueCalls)
	}
	if exitCalled {
		t.Error("Exit was called after RestoreDefault, want no-op (handler stopped; default disposition applies)")
	}
}

// contains reports whether s contains substr.
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
