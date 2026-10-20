package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// jsonEventExtractor classifies the stub-persona event shapes: {"type":"result","message":...} is
// the terminal event, {"type":"error","message":...} an in-stream error (FR-S3), everything else
// (progress ticks, heartbeats, non-JSON junk) is EventIgnored — the lenient contract.
type jsonEventExtractor struct{}

func (jsonEventExtractor) Extract(line string) EventOutcome {
	var ev struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		return EventOutcome{Kind: EventIgnored} // unparseable line — lenient, never an error
	}
	switch ev.Type {
	case "result":
		return EventOutcome{Kind: EventTerminal, Message: ev.Message}
	case "error":
		return EventOutcome{Kind: EventError, ErrText: ev.Message}
	default:
		return EventOutcome{Kind: EventIgnored}
	}
}

// registerEventExtractorForTest registers e under name and removes it on cleanup (the seam-var
// idiom — tests run sequentially, no mutex).
func registerEventExtractorForTest(t *testing.T, name string, e Extractor) {
	t.Helper()
	RegisterEventExtractor(name, e)
	t.Cleanup(func() { delete(eventExtractors, name) })
}

// eventTestManifest is a minimal event-mode manifest: EventStream non-empty (the mode switch) and
// Name keyed to the registered extractor.
func eventTestManifest(name string) Manifest {
	return Manifest{Name: name, EventStream: []string{"--mode", "json"}}
}

// printTestManifest is a minimal print-mode manifest (EventStream nil — the v3.4 default).
func printTestManifest() Manifest {
	return Manifest{Name: "t", Command: strPtr("t"), Output: strPtr("raw"), StripCodeFence: boolPtr(true)}
}

// ---------------------------------------------------------------------------
// ParseEventStream — scan semantics (win rules, leniency, defensiveness)
// ---------------------------------------------------------------------------

func TestParseEventStream_ProgressLinesIgnored_TerminalAssembled(t *testing.T) {
	registerEventExtractorForTest(t, "evp", jsonEventExtractor{})
	m := eventTestManifest("evp")
	raw := "{\"type\":\"progress\",\"n\":1}\n" +
		"{\"type\":\"progress\",\"n\":2}\n" +
		"{\"type\":\"result\",\"message\":\"feat: add widget\"}\n"
	msg, ok, errText := ParseEventStream(raw, m)
	if !ok || msg != "feat: add widget" || errText != "" {
		t.Errorf("ParseEventStream = (%q, %v, %q), want (\"feat: add widget\", true, \"\")", msg, ok, errText)
	}
}

func TestParseEventStream_LastTerminalWins(t *testing.T) {
	registerEventExtractorForTest(t, "evp", jsonEventExtractor{})
	m := eventTestManifest("evp")
	raw := "{\"type\":\"result\",\"message\":\"first\"}\n" +
		"{\"type\":\"result\",\"message\":\"second\"}\n" +
		"{\"type\":\"result\",\"message\":\"feat: final\"}\n"
	msg, ok, _ := ParseEventStream(raw, m)
	if !ok || msg != "feat: final" {
		t.Errorf("ParseEventStream = (%q, %v), want last terminal (\"feat: final\", true)", msg, ok)
	}
}

func TestParseEventStream_TerminalBeatsError(t *testing.T) {
	registerEventExtractorForTest(t, "evp", jsonEventExtractor{})
	m := eventTestManifest("evp")
	raw := "{\"type\":\"error\",\"message\":\"usage limit exceeded\"}\n" +
		"{\"type\":\"result\",\"message\":\"feat: recovered\"}\n"
	msg, ok, errText := ParseEventStream(raw, m)
	// A produced message is a SUCCESS — FR-S3 fires only with NO terminal message event.
	if !ok || msg != "feat: recovered" || errText != "" {
		t.Errorf("ParseEventStream = (%q, %v, %q), want terminal to beat error", msg, ok, errText)
	}
}

func TestParseEventStream_ErrorOnlyStream(t *testing.T) {
	registerEventExtractorForTest(t, "evp", jsonEventExtractor{})
	m := eventTestManifest("evp")
	raw := "{\"type\":\"progress\",\"n\":1}\n" +
		"{\"type\":\"error\",\"message\":\"usage limit exceeded\"}\n"
	msg, ok, errText := ParseEventStream(raw, m)
	if ok || msg != "" || errText != "usage limit exceeded" {
		t.Errorf("ParseEventStream = (%q, %v, %q), want (\"\", false, \"usage limit exceeded\")", msg, ok, errText)
	}
}

func TestParseEventStream_FirstErrorRecorded(t *testing.T) {
	registerEventExtractorForTest(t, "evp", jsonEventExtractor{})
	m := eventTestManifest("evp")
	raw := "{\"type\":\"error\",\"message\":\"first failure\"}\n" +
		"{\"type\":\"error\",\"message\":\"second failure\"}\n"
	_, _, errText := ParseEventStream(raw, m)
	if errText != "first failure" {
		t.Errorf("errText = %q, want the FIRST error recorded", errText)
	}
}

func TestParseEventStream_CRLF_BlankAndJunkLinesIgnored(t *testing.T) {
	registerEventExtractorForTest(t, "evp", jsonEventExtractor{})
	m := eventTestManifest("evp")
	// CRLF line endings, blank/whitespace-only lines, and non-JSON junk must all be tolerated.
	raw := "{\"type\":\"progress\",\"n\":1}\r\n" +
		"\r\n" +
		"   \n" +
		"not json at all {{{\n" +
		"\t\n" +
		"{\"type\":\"result\",\"message\":\"feat: crlf ok\"}\r\n"
	msg, ok, errText := ParseEventStream(raw, m)
	if !ok || msg != "feat: crlf ok" || errText != "" {
		t.Errorf("ParseEventStream = (%q, %v, %q), want CRLF/junk tolerance", msg, ok, errText)
	}
}

func TestParseEventStream_NoExtractorDefensive(t *testing.T) {
	// NO registration for this name — must return the zero result, never panic (defensive;
	// Render's FR-S2 guard makes it unreachable in production).
	m := eventTestManifest("never-registered-provider")
	msg, ok, errText := ParseEventStream("{\"type\":\"result\",\"message\":\"x\"}\n", m)
	if ok || msg != "" || errText != "" {
		t.Errorf("ParseEventStream without extractor = (%q, %v, %q), want (\"\", false, \"\")", msg, ok, errText)
	}
}

func TestParseEventStream_EmptyStream(t *testing.T) {
	registerEventExtractorForTest(t, "evp", jsonEventExtractor{})
	m := eventTestManifest("evp")
	msg, ok, errText := ParseEventStream("", m)
	if ok || msg != "" || errText != "" {
		t.Errorf("ParseEventStream(\"\") = (%q, %v, %q), want zero result", msg, ok, errText)
	}
}

// ---------------------------------------------------------------------------
// UnwrapEventOutput — the non-ParseOutput parse-site integration
// ---------------------------------------------------------------------------

func TestUnwrapEventOutput_PrintModePassthrough(t *testing.T) {
	registerEventExtractorForTest(t, "evp", jsonEventExtractor{})
	m := printTestManifest() // EventStream nil
	raw := "fix: plain print-mode message"
	if got := UnwrapEventOutput(raw, m); got != raw {
		t.Errorf("UnwrapEventOutput print mode = %q, want passthrough %q", got, raw)
	}
}

func TestUnwrapEventOutput_EventModeAssembles(t *testing.T) {
	registerEventExtractorForTest(t, "evp", jsonEventExtractor{})
	m := eventTestManifest("evp")
	raw := "{\"type\":\"progress\",\"n\":1}\n{\"type\":\"result\",\"message\":\"feat: assembled\"}\n"
	if got := UnwrapEventOutput(raw, m); got != "feat: assembled" {
		t.Errorf("UnwrapEventOutput event mode = %q, want \"feat: assembled\"", got)
	}
}

func TestUnwrapEventOutput_EventModeNoTerminalEmpty(t *testing.T) {
	registerEventExtractorForTest(t, "evp", jsonEventExtractor{})
	m := eventTestManifest("evp")
	raw := "{\"type\":\"error\",\"message\":\"boom\"}\n"
	if got := UnwrapEventOutput(raw, m); got != "" {
		t.Errorf("UnwrapEventOutput (error event only) = %q, want \"\"", got)
	}
}

// ---------------------------------------------------------------------------
// EventErrorHint — the FR-S3 precedence sibling of StderrHint
// ---------------------------------------------------------------------------

func TestEventErrorHint_ErrorEventSurfaced(t *testing.T) {
	registerEventExtractorForTest(t, "evp", jsonEventExtractor{})
	m := eventTestManifest("evp")
	raw := "{\"type\":\"progress\",\"n\":1}\n{\"type\":\"error\",\"message\":\"usage limit exceeded\"}\n"
	want := "agent event stream reported an error: usage limit exceeded"
	if got := EventErrorHint(raw, m); got != want {
		t.Errorf("EventErrorHint = %q, want %q", got, want)
	}
}

func TestEventErrorHint_PrintModeEmpty(t *testing.T) {
	registerEventExtractorForTest(t, "evp", jsonEventExtractor{})
	m := printTestManifest() // EventStream nil ⇒ the fast path
	raw := "{\"type\":\"error\",\"message\":\"x\"}\n"
	if got := EventErrorHint(raw, m); got != "" {
		t.Errorf("EventErrorHint print mode = %q, want \"\"", got)
	}
}

func TestEventErrorHint_TerminalPresentEmpty(t *testing.T) {
	registerEventExtractorForTest(t, "evp", jsonEventExtractor{})
	m := eventTestManifest("evp")
	// A terminal message is a success — no hint even with an earlier error event (FR-S3 fires
	// only when NO terminal message exists).
	raw := "{\"type\":\"error\",\"message\":\"x\"}\n{\"type\":\"result\",\"message\":\"feat: ok\"}\n"
	if got := EventErrorHint(raw, m); got != "" {
		t.Errorf("EventErrorHint with terminal = %q, want \"\"", got)
	}
}

func TestEventErrorHint_MultilineCollapsedAndCapped(t *testing.T) {
	registerEventExtractorForTest(t, "evp", jsonEventExtractor{})
	m := eventTestManifest("evp")
	// A multi-line error (>1000 chars) collapses to ONE line and caps at 1000 + the ellipsis. The
	// JSON carries an ESCAPED \n so the parsed message value contains a real newline.
	long := strings.Repeat("boom ", 300) // 1500 chars
	raw := "{\"type\":\"error\",\"message\":\"" + long + "line2\\nline3\"}\n"
	got := EventErrorHint(raw, m)
	if !strings.HasPrefix(got, "agent event stream reported an error: ") {
		t.Fatalf("EventErrorHint missing the prefix: %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("EventErrorHint must be single-line: %q", got)
	}
	body := strings.TrimPrefix(got, "agent event stream reported an error: ")
	// StderrHint's idiom: truncate to 1000 BYTES then append the 3-byte "…" rune ⇒ ≤1003 bytes.
	if len(body) > 1003 {
		t.Errorf("EventErrorHint body = %d bytes, want ≤1000+ellipsis(3)", len(body))
	}
	if !strings.HasSuffix(body, "…") {
		t.Errorf("EventErrorHint body should end with the cap ellipsis, got %q", body)
	}
}

// ---------------------------------------------------------------------------
// ParseOutput step-0 — event mode through the full pipeline
// ---------------------------------------------------------------------------

func TestParseOutput_EventModeAssemblesThroughCleanupSteps(t *testing.T) {
	registerEventExtractorForTest(t, "evp", jsonEventExtractor{})
	m := eventTestManifest("evp")
	// The terminal event carries a FENCED message — step 2 (fence-strip) must apply to the
	// assembled text exactly as in print mode.
	raw := "{\"type\":\"progress\",\"n\":1}\n" +
		"{\"type\":\"result\",\"message\":\"```\\nfeat: fenced inside event\\n```\"}\n"
	msg, ok, fellback := ParseOutput(raw, m)
	if !ok || msg != "feat: fenced inside event" {
		t.Errorf("ParseOutput event mode = (%q, %v), want the fenced terminal message stripped", msg, ok)
	}
	if fellback {
		t.Errorf("fellback = true, want ALWAYS false in event mode")
	}
}

func TestParseOutput_EventModeNoTerminalOKFalse(t *testing.T) {
	registerEventExtractorForTest(t, "evp", jsonEventExtractor{})
	m := eventTestManifest("evp")
	// Error-only stream ⇒ no terminal message ⇒ ok==false ⇒ the existing FR29 retry path.
	raw := "{\"type\":\"error\",\"message\":\"usage limit exceeded\"}\n"
	msg, ok, fellback := ParseOutput(raw, m)
	if ok || msg != "" || fellback {
		t.Errorf("ParseOutput event error-only = (%q, %v, %v), want (\"\", false, false)", msg, ok, fellback)
	}
}

func TestParseOutput_PrintModeRegression(t *testing.T) {
	// Print mode (EventStream nil) must be byte-identical to the pre-event pipeline — a raw
	// message with no registered extractor in sight parses unchanged.
	m := printTestManifest()
	msg, ok, fellback := ParseOutput("fix: plain message\n", m)
	if !ok || msg != "fix: plain message" || fellback {
		t.Errorf("ParseOutput print mode = (%q, %v, %v), want unchanged raw behavior", msg, ok, fellback)
	}
}
