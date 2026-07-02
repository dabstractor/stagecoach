package ui

import (
	"bytes"
	"strings"
	"testing"
)

// TestShouldColor exhaustively covers all 8 corners of the pure color
// predicate (isTTY × NO_COLOR ∈ {"","1"} × noColorFlag). Color is enabled
// ONLY for (TTY && NO_COLOR empty && !flag); every other combination is off.
// It is table-driven so the color-ON path is covered without faking a TTY.
func TestShouldColor(t *testing.T) {
	tests := []struct {
		name        string
		isTTY       bool
		noColorEnv  string
		noColorFlag bool
		want        bool
	}{
		{"TTY, empty env, no flag", true, "", false, true},
		{"TTY, empty env, flag on", true, "", true, false},
		{"TTY, NO_COLOR=1, no flag", true, "1", false, false},
		{"TTY, NO_COLOR=1, flag on", true, "1", true, false},
		{"non-TTY, empty env, no flag", false, "", false, false},
		{"non-TTY, empty env, flag on", false, "", true, false},
		{"non-TTY, NO_COLOR=1, no flag", false, "1", false, false},
		{"non-TTY, NO_COLOR=1, flag on", false, "1", true, false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldColor(tc.isTTY, tc.noColorEnv, tc.noColorFlag); got != tc.want {
				t.Errorf("shouldColor(%v, %q, %v) = %v, want %v",
					tc.isTTY, tc.noColorEnv, tc.noColorFlag, got, tc.want)
			}
		})
	}
}

// TestRouting verifies the FR51 stream discipline: Progressf and Verbosef
// land ONLY on stderr, Resultf lands ONLY on stdout. Each method writes a
// distinct marker so a misroute is caught by absence assertions, not just
// presence. Newlines are passed in the format string (the methods do not
// force them).
func TestRouting(t *testing.T) {
	out := &bytes.Buffer{}
	errb := &bytes.Buffer{}
	o := &Output{stdout: out, stderr: errb, color: false, verbose: true}

	if e := o.Progressf("P%s\n", "1"); e != nil {
		t.Fatalf("Progressf returned error: %v", e)
	}
	if e := o.Resultf("R%s\n", "2"); e != nil {
		t.Fatalf("Resultf returned error: %v", e)
	}
	if e := o.Verbosef("V%s\n", "3"); e != nil {
		t.Fatalf("Verbosef returned error: %v", e)
	}

	gotStderr := errb.String()
	gotStdout := out.String()

	// stderr holds progress + verbose, never the result.
	for _, want := range []string{"P1", "V3"} {
		if !strings.Contains(gotStderr, want) {
			t.Errorf("stderr = %q, missing %q", gotStderr, want)
		}
	}
	if strings.Contains(gotStderr, "R2") {
		t.Errorf("stderr = %q, must not contain result R2", gotStderr)
	}

	// stdout holds the result only, never progress or verbose.
	if !strings.Contains(gotStdout, "R2") {
		t.Errorf("stdout = %q, missing result R2", gotStdout)
	}
	for _, unwanted := range []string{"P1", "V3"} {
		if strings.Contains(gotStdout, unwanted) {
			t.Errorf("stdout = %q, must not contain %q", gotStdout, unwanted)
		}
	}
}

// TestVerboseSuppressed verifies that Verbosef is a complete no-op (writes
// nothing, returns nil) when verbose is false — the FR50 gate must
// short-circuit before any formatting happens.
func TestVerboseSuppressed(t *testing.T) {
	errb := &bytes.Buffer{}
	o := &Output{stdout: &bytes.Buffer{}, stderr: errb, color: false, verbose: false}

	if e := o.Verbosef("V%s\n", "x"); e != nil {
		t.Errorf("Verbosef with verbose=false returned error %v, want nil", e)
	}
	if errb.Len() != 0 {
		t.Errorf("stderr buffer = %q, want empty (verbose suppressed)", errb.String())
	}
}

// TestColorAbsentWhenDisabled verifies that with color off (the state under
// NO_COLOR=1, a non-TTY, or --no-color), Color returns the input string with
// NO ANSI escape bytes — this is what keeps piped output byte-clean.
func TestColorAbsentWhenDisabled(t *testing.T) {
	o := &Output{stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}, color: false, verbose: false}

	got := o.Color("\x1b[31m", "x")
	if got != "x" {
		t.Errorf("Color() = %q, want %q", got, "x")
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("Color() = %q, must not contain ANSI escape", got)
	}
}

// TestColorPresentWhenEnabled verifies the color-ON path: with color forced
// on (the white-box construction tests use, mirroring a real TTY run), Green
// wraps the input in the green SGR sequence plus a reset.
func TestColorPresentWhenEnabled(t *testing.T) {
	o := &Output{stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}, color: true, verbose: false}

	want := "\x1b[32mx\x1b[0m"
	if got := o.Green("x"); got != want {
		t.Errorf("Green(%q) = %q, want %q", "x", got, want)
	}
}

// TestNewOutputNoColorEnv verifies the production constructor resolves color
// to off when NO_COLOR is set (and stdout is a non-TTY buffer, as it is in
// every test). The env-vs-TTY decomposition itself is covered by
// TestShouldColor; this confirms NewOutput threads both signals through.
// t.Setenv is auto-restored after the test (Go 1.17+).
func TestNewOutputNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	o := NewOutput(&bytes.Buffer{}, &bytes.Buffer{}, false, false)
	if o.color {
		t.Errorf("NewOutput().color = true, want false (non-TTY buffer + NO_COLOR=1)")
	}
}
