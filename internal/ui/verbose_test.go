package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestVerbose_CommandWhenOn(t *testing.T) {
	var buf bytes.Buffer
	v := NewVerbose(&buf, true)
	v.VerboseCommand("pi --model x")
	want := "DEBUG: command: pi --model x\n"
	if buf.String() != want {
		t.Errorf("VerboseCommand: got %q, want %q", buf.String(), want)
	}
}

func TestVerbose_RawOutputWhenOn(t *testing.T) {
	var buf bytes.Buffer
	v := NewVerbose(&buf, true)

	// With trailing newline in output.
	v.VerboseRawOutput("feat: x\n")
	want := "DEBUG: raw output:\nfeat: x\n"
	if buf.String() != want {
		t.Errorf("VerboseRawOutput (trailing NL): got %q, want %q", buf.String(), want)
	}
	buf.Reset()

	// Without trailing newline — should add one.
	v.VerboseRawOutput("feat: x")
	want = "DEBUG: raw output:\nfeat: x\n"
	if buf.String() != want {
		t.Errorf("VerboseRawOutput (no trailing NL): got %q, want %q", buf.String(), want)
	}
}

func TestVerbose_RetryWhenOn(t *testing.T) {
	var buf bytes.Buffer
	v := NewVerbose(&buf, true)
	v.VerboseRetry(1, `subject "x" matches an existing commit`)
	want := `DEBUG: attempt 1: subject "x" matches an existing commit` + "\n"
	if buf.String() != want {
		t.Errorf("VerboseRetry: got %q, want %q", buf.String(), want)
	}
}

func TestVerbose_NoOpWhenOff(t *testing.T) {
	var buf bytes.Buffer
	v := NewVerbose(&buf, false)
	v.VerboseCommand("pi --model x")
	v.VerboseRawOutput("feat: x\n")
	v.VerboseRetry(1, "reason")
	if buf.Len() != 0 {
		t.Errorf("off: wrote %q, want zero bytes", buf.String())
	}
}

func TestVerbose_NilSafeReceiver(t *testing.T) {
	var v *Verbose = nil
	v.VerboseCommand("x")   // must not panic
	v.VerboseRawOutput("y") // must not panic
	v.VerboseRetry(1, "z")  // must not panic
}

func TestVerbose_NilWriterNoOp(t *testing.T) {
	v := NewVerbose(nil, true)
	v.VerboseCommand("x")   // must not panic
	v.VerboseRawOutput("y") // must not panic
	v.VerboseRetry(1, "z")  // must not panic
}

func TestVerbose_MultipleLinesAccumulate(t *testing.T) {
	var buf bytes.Buffer
	v := NewVerbose(&buf, true)
	v.VerboseCommand("pi --model x")
	v.VerboseRawOutput("feat: hello\n")
	v.VerboseRetry(1, `subject "feat: existing" matches an existing commit`)

	s := buf.String()

	// All three substrings present in order.
	if !strings.Contains(s, "DEBUG: command: pi --model x\n") {
		t.Errorf("missing command line; got %q", s)
	}
	if !strings.Contains(s, "DEBUG: raw output:\nfeat: hello\n") {
		t.Errorf("missing raw output line; got %q", s)
	}
	if !strings.Contains(s, `DEBUG: attempt 1: subject "feat: existing" matches an existing commit`+"\n") {
		t.Errorf("missing retry line; got %q", s)
	}

	// Command comes before raw output.
	cmdIdx := strings.Index(s, "DEBUG: command:")
	rawIdx := strings.Index(s, "DEBUG: raw output:")
	if cmdIdx >= rawIdx {
		t.Errorf("command (%d) should come before raw output (%d)", cmdIdx, rawIdx)
	}

	// Raw output comes before retry.
	retryIdx := strings.Index(s, "DEBUG: attempt")
	if rawIdx >= retryIdx {
		t.Errorf("raw output (%d) should come before retry (%d)", rawIdx, retryIdx)
	}
}
