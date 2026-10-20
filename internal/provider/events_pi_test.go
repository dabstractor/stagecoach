package provider

import (
	"strings"
	"testing"
)

// Fixtures below are the VERBATIM stdout lines captured by the P2.M2.T1.S1 harness run of
// 2026-10-20 (pi 1.0.2, `--mode json`, model override deepseek/deepseek-flash; full evidence:
// plan/022_856008a8cbb5/P2M2T1S2/research/pi_capture_2026-10-20.log). The capture is the
// authority (FR-S2 Gotcha 5); these tests pin the extractor to the wire as it actually speaks,
// not to docs/json.md's idealization. auto_retry_end's shape is docs-documented (json.md) and did
// NOT occur in the capture (the 400 invalid-model probe was non-retryable); its test below is
// labeled accordingly.

// piFxSession is the captured first line (session header).
const piFxSession = `{"type":"session","version":3,"id":"01a1596f-0d20-7102-aee7-8d881564be25","timestamp":"2026-10-20T13:10:12.512Z","cwd":"/tmp/TestEventStreamVerifypi3582551412/001"}`

// piFxAgentStart / piFxTurnStart / piFxAgentSettled: captured lifecycle markers (EventIgnored).
const (
	piFxAgentStart   = `{"type":"agent_start"}`
	piFxTurnStart    = `{"type":"turn_start"}`
	piFxAgentSettled = `{"type":"agent_settled"}`
)

// piFxMessageUpdate is a captured delta-only message_update (thinking_start here) — NEVER
// accumulated; the extractor must classify it EventIgnored (Gotcha 2).
const piFxMessageUpdate = `{"type":"message_update","usage":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0,"totalTokens":0,"cost":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0,"total":0}},"assistantMessageEvent":{"type":"thinking_start","contentIndex":0}}`

// piFxTerminal is the captured TERMINAL event: message_end{role:"assistant"} whose content
// carries a thinking block + the text block "stagecoach-event-verify" (the harness sentinel).
const piFxTerminal = `{"type":"message_end","message":{"role":"assistant","content":[{"type":"thinking","thinking":"The user asks to reply with exactly: stagecoach-event-verify. We should comply.","thinkingSignature":"reasoning_content"},{"type":"text","text":"stagecoach-event-verify"}],"api":"openai-completions","provider":"deepseek","model":"deepseek-flash","usage":{"input":572,"output":26,"cacheRead":0,"cacheWrite":0,"reasoning":19,"totalTokens":598,"cost":{"input":0.0001716,"output":0.0000312,"cacheRead":0,"cacheWrite":0,"total":0.0002028}},"stopReason":"stop","timestamp":1792501812583,"responseId":"229f132e-65ed-4d74-b635-ab38dd80ca19","rawStopReason":"stop","thinkingLevel":"high"}}`

// piFxErrorMessageEnd is the captured error event (forced invalid-model probe; pi exited 0 —
// FR-S3's premise): message_end{role:"assistant", content:[], stopReason:"error",
// errorMessage:"400: …"}.
const piFxErrorMessageEnd = `{"type":"message_end","message":{"role":"assistant","content":[],"api":"openai-completions","provider":"deepseek","model":"stagecoach-verify-invalid-model","usage":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0,"totalTokens":0,"cost":{"input":0,"output":0,"cacheRead":0,"cacheWrite":0,"total":0}},"stopReason":"error","timestamp":1792501814367,"errorMessage":"400: {\"message\":\"The supported API model names are deepseek-flash, deepseek-v4-pro, but you passed stagecoach-verify-invalid-model. (request_id: dfaf1131-ee0d-4b48-ae1b-8354a858f2f8)\",\"type\":\"invalid_request_error\",\"param\":null,\"code\":\"invalid_request_error\"}","thinkingLevel":"high"}}`

// piTestManifest is a minimal event-mode pi manifest (Resolve() defaults the rest).
func piTestManifest() Manifest {
	return Manifest{Name: "pi", EventStream: []string{"--mode", "json"}}
}

func TestPiExtractor_TerminalAssemblesTextBlocks(t *testing.T) {
	oc := piEventExtractor{}.Extract(piFxTerminal)
	if oc.Kind != EventTerminal {
		t.Fatalf("Kind = %v, want EventTerminal", oc.Kind)
	}
	// The captured reply is exactly the sentinel: thinking block skipped, text block kept.
	if oc.Message != "stagecoach-event-verify" {
		t.Errorf("Message = %q, want %q", oc.Message, "stagecoach-event-verify")
	}
	if oc.ErrText != "" {
		t.Errorf("ErrText = %q, want empty", oc.ErrText)
	}
}

func TestPiExtractor_ErrorInStream(t *testing.T) {
	oc := piEventExtractor{}.Extract(piFxErrorMessageEnd)
	if oc.Kind != EventError {
		t.Fatalf("Kind = %v, want EventError (captured stopReason:%q errorMessage present; exit was 0 — FR-S3)", oc.Kind, "error")
	}
	if !strings.Contains(oc.ErrText, "400") || !strings.Contains(oc.ErrText, "stagecoach-verify-invalid-model") {
		t.Errorf("ErrText = %q, want the captured errorMessage (400 … invalid model)", oc.ErrText)
	}
	if oc.Message != "" {
		t.Errorf("Message = %q, want empty", oc.Message)
	}
}

func TestPiExtractor_IgnoredShapes(t *testing.T) {
	ignored := map[string]string{
		"session header":                      piFxSession,
		"agent_start":                         piFxAgentStart,
		"turn_start":                          piFxTurnStart,
		"agent_settled":                       piFxAgentSettled,
		"message_update":                      piFxMessageUpdate, // delta-ONLY — never accumulated (Gotcha 2)
		"user message_end":                    `{"type":"message_end","message":{"role":"user","content":[{"type":"text","text":"hi"}]}}`,
		"system message_end (string content)": `{"type":"message_end","message":{"role":"system","content":""}}`,
	}
	ex := piEventExtractor{}
	for name, line := range ignored {
		if oc := ex.Extract(line); oc.Kind != EventIgnored {
			t.Errorf("%s: Kind = %v, want EventIgnored (oc=%+v)", name, oc.Kind, oc)
		}
	}
}

func TestPiExtractor_Lenient(t *testing.T) {
	lenient := map[string]string{
		"empty":                               "",
		"spaces":                              "   ",
		"garbage":                             "not json at all",
		"no type":                             `{"usage":{"input":1}}`,
		"array":                               `[1,2,3]`,
		"unknown":                             `{"type":"entry_appended"}`,
		"bare error event (uncaptured shape)": `{"type":"error"}`,
	}
	ex := piEventExtractor{}
	for name, line := range lenient {
		if oc := ex.Extract(line); oc.Kind != EventIgnored {
			t.Errorf("%s: Kind = %v, want EventIgnored (lenient contract)", name, oc.Kind)
		}
	}
}

// TestPiExtractor_AutoRetryEndFinalFailure pins the DOCS-documented final-failure branch
// (docs/json.md: auto_retry_end carries success=false + finalError when retries are exhausted).
// This shape did NOT occur in the 2026-10-20 capture; the test documents the conservative
// classification (success:true ⇒ the retry recovered ⇒ Ignored), not a captured wire shape.
func TestPiExtractor_AutoRetryEndFinalFailure(t *testing.T) {
	fail := piEventExtractor{}.Extract(`{"type":"auto_retry_end","success":false,"finalError":"529 overloaded (final)"}`)
	if fail.Kind != EventError || fail.ErrText != "529 overloaded (final)" {
		t.Errorf("auto_retry_end success=false: got %+v, want EventError/529 overloaded (final)", fail)
	}
	recovered := piEventExtractor{}.Extract(`{"type":"auto_retry_end","success":true}`)
	if recovered.Kind != EventIgnored {
		t.Errorf("auto_retry_end success=true: got %v, want EventIgnored", recovered.Kind)
	}
}

// TestPiEventStream_Composite pins ParseEventStream's win rules over the captured happy stream
// and the captured error stream (LAST terminal wins; terminal beats error; hint on error-only).
func TestPiEventStream_Composite(t *testing.T) {
	m := piTestManifest()

	happy := strings.Join([]string{piFxSession, piFxAgentStart, piFxTurnStart, piFxMessageUpdate, piFxTerminal, piFxAgentSettled}, "\n")
	msg, ok, errText := ParseEventStream(happy, m)
	if !ok || msg != "stagecoach-event-verify" || errText != "" {
		t.Errorf("happy stream: got (%q, %v, %q), want (stagecoach-event-verify, true, \"\")", msg, ok, errText)
	}

	errOnly := strings.Join([]string{piFxSession, piFxErrorMessageEnd, piFxAgentSettled}, "\n")
	msg2, ok2, errText2 := ParseEventStream(errOnly, m)
	if ok2 || msg2 != "" {
		t.Errorf("error stream: got (%q, %v), want (\"\", false)", msg2, ok2)
	}
	if !strings.Contains(errText2, "400") {
		t.Errorf("error stream errText = %q, want the captured 400 errorMessage", errText2)
	}

	// Terminal BEATS error: a produced (possibly late) message_end is a success (events.go rules).
	mixed := strings.Join([]string{piFxSession, piFxErrorMessageEnd, piFxTerminal}, "\n")
	if msg3, ok3, errText3 := ParseEventStream(mixed, m); !ok3 || msg3 != "stagecoach-event-verify" || errText3 != "" {
		t.Errorf("mixed stream: got (%q, %v, %q), want terminal win", msg3, ok3, errText3)
	}

	// FR-S3 surfacing: EventErrorHint fires ONLY on the error-only stream (single line, capped).
	if h := EventErrorHint(happy, m); h != "" {
		t.Errorf("EventErrorHint(happy) = %q, want empty", h)
	}
	if h := EventErrorHint(errOnly, m); h == "" || !strings.Contains(h, "400") {
		t.Errorf("EventErrorHint(errOnly) = %q, want the 400 cause", h)
	} else if strings.Contains(h, "\n") {
		t.Errorf("EventErrorHint must be single-line, got %q", h)
	}
}

func TestPiEventStream_Registered(t *testing.T) {
	if LookupEventExtractor("pi") == nil {
		t.Fatal("no extractor registered for pi — events_pi.go's init must register it (production, not test-scoped)")
	}
	m, ok := BuiltinManifests()["pi"]
	if !ok {
		t.Fatal("no built-in pi manifest")
	}
	if len(m.Resolve().EventStream) == 0 {
		t.Errorf("builtinPi ships no EventStream — the dated ledger entry (%q) requires the declaration", "2026-10-20")
	}
}
