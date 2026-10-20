package provider

import (
	"encoding/json"
	"strings"
)

// pi event-stream extractor (§9.30 FR-S1–FR-S3) — VERIFIED 2026-10-20 via the P2.M2.T1.S1 harness
// (`STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/ -run TestEventStreamVerify/pi -v`;
// full evidence: plan/022_856008a8cbb5/P2M2T1S2/research/pi_capture_2026-10-20.log; pi 1.0.2;
// model override deepseek/deepseek-flash — the seeded anthropic default had no API key on this
// machine, an auth condition, not a mode failure). docs/json.md was design input; the capture below
// is the authority (Gotcha 5).
//
// CAPTURED SHAPES (pi 1.0.2, 2026-10-20):
//   - TERMINAL: {"type":"message_end","message":{"role":"assistant","content":[
//     {"type":"thinking","thinking":…,"thinkingSignature":…},
//     {"type":"text","text":"stagecoach-event-verify"}],"stopReason":"stop",…}}
//     — "the authoritative final message" (docs/json.md). Message = the CONCATENATION of the
//     content blocks' "text" fields (thinking/tool_call blocks skipped). message_update deltas are
//     DELTA-ONLY and never accumulated (docs: "Replace the whole partial message with
//     message_end.message when it arrives").
//   - ERROR: the same message_end shape with "stopReason":"error" and
//     "errorMessage":"400: {…}" (content empty) — captured via the forced invalid-model probe.
//     pi exited 0 (recorded as EVIDENCE, never asserted): FR-S3's premise proven — detection MUST
//     key on the stream, never the exit code. The twin message_start and the trailing
//     turn_end/agent_end{willRetry:false} carry the same payload; only message_end classifies
//     (it is the authoritative completion event, and ParseEventStream records the FIRST error).
//   - IGNORED: session, agent_start, turn_start, message_start, message_end{user,system},
//     message_update, turn_end, tool_execution_*, agent_end, agent_settled, compaction_*,
//     entry_appended, session_info_changed — everything the capture did not prove is a
//     terminal/error classifies leniently as EventIgnored.
//   - auto_retry_end with success==false + finalError (docs/json.md's documented final-failure
//     shape; NOT fired in this capture — the 400 invalid-model probe was non-retryable, so no
//     retry cycle ran) classifies as EventError: a run that exhausted its retries is a hard
//     failure by definition, and ParseEventStream's terminal-beats-error semantics keep it inert
//     whenever a real message was produced. success:true (retry recovered) stays EventIgnored.
//
// STATELESS per the Extractor contract (events.go): registry entries are singletons shared by
// concurrent roles (decompose), so classification reads ONLY the line's own bytes — no fields, no
// accumulators. Unmarshal failures and unknown types are EventIgnored; there is no error return.
func init() { RegisterEventExtractor("pi", piEventExtractor{}) }

// piEventExtractor classifies one `pi --mode json` stdout line per the 2026-10-20 capture.
type piEventExtractor struct{}

// piEventLine is the minimal unmarshal target for classification — only the fields the extractor's
// decisions need; everything else is ignored by omission.
type piEventLine struct {
	Type    string          `json:"type"`
	Message *piEventMessage `json:"message"`
	// auto_retry_end carries (docs shape): success=false + finalError on final failure.
	Success    *bool  `json:"success"`
	FinalError string `json:"finalError"`
}

// piEventMessage is the message_* payload. Content is json.RawMessage because pi's content shape
// varies by role: block arrays on assistant/user messages, a plain string on the system preamble
// message (captured 2026-10-20) — a typed []piContentBlock field would fail the WHOLE line's
// unmarshal on the string shape, and that line must be Ignored, not garbaged.
type piEventMessage struct {
	Role         string          `json:"role"`
	StopReason   string          `json:"stopReason"`
	ErrorMessage string          `json:"errorMessage"`
	Content      json.RawMessage `json:"content"`
}

// piContentBlock is one assistant-message content block (docs/message-types.md). Only "text"
// blocks carry commit-message prose; thinking/tool_call blocks are skipped by assemblePiText.
type piContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Extract classifies ONE verbatim pi --mode json line (stateless; lenient; no error return).
func (piEventExtractor) Extract(line string) EventOutcome {
	var ev piEventLine
	if err := json.Unmarshal([]byte(line), &ev); err != nil || ev.Type == "" {
		return EventOutcome{Kind: EventIgnored} // blank / non-JSON / garbage — lenient by contract
	}
	switch ev.Type {
	case "message_end":
		if ev.Message == nil || ev.Message.Role != "assistant" {
			return EventOutcome{Kind: EventIgnored} // user/system message_end
		}
		if ev.Message.StopReason == "error" || ev.Message.ErrorMessage != "" {
			// 2026-10-20 capture: a failed response is message_end{stopReason:"error",
			// errorMessage:"400: …"} with EMPTY content and a zero exit — FR-S3's in-stream signal.
			msg := ev.Message.ErrorMessage
			if msg == "" {
				msg = `assistant message failed (stopReason "error")`
			}
			return EventOutcome{Kind: EventError, ErrText: msg}
		}
		return EventOutcome{Kind: EventTerminal, Message: assemblePiText(ev.Message.Content)}
	case "auto_retry_end": // docs-documented final-failure shape; conservative per Gotcha 3
		if ev.Success != nil && !*ev.Success && ev.FinalError != "" {
			return EventOutcome{Kind: EventError, ErrText: ev.FinalError}
		}
		return EventOutcome{Kind: EventIgnored} // retry recovered (success:true) — the run continues
	default:
		// session, agent_start, turn_start, message_start, message_update (delta-ONLY — never
		// accumulated), turn_end, tool_execution_*, agent_end, agent_settled, compaction_*,
		// entry_appended, session_info_changed, bare {"type":"error"} (uncaptured — structure
		// unknown; docs say the agent loop translates provider errors into message_* events) …
		return EventOutcome{Kind: EventIgnored}
	}
}

// assemblePiText concatenates the text blocks of an assistant message's content array (skipping
// thinking/tool_call blocks); a non-array content (the system message's plain string) yields "".
func assemblePiText(raw json.RawMessage) string {
	var blocks []piContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return "" // string content — no text blocks to assemble
	}
	var b strings.Builder
	for _, blk := range blocks {
		if blk.Type == "text" {
			b.WriteString(blk.Text)
		}
	}
	return b.String()
}
