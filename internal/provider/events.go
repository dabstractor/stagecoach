package provider

import "strings"

// Event-extractor registry + the event-stream execution semantics for the event-stream output
// mode (§9.30 FR-S1–FR-S3).
//
// OWNERSHIP (P2.M1.T1):
//   - S1: the registry seam (map + Lookup/Register) + the render arming/guard.
//   - S2 (this file's semantics): the Extractor refinement (EventKind/EventOutcome — S1's PRP
//     explicitly anticipated the placeholder refinement), ParseEventStream (the NDJSON-ish
//     framework scan), UnwrapEventOutput (the non-ParseOutput parse-site integration), and
//     EventErrorHint (the FR-S3 in-stream-error cause surfacing).
//   - P2.M2.T1.S2–S5: POPULATION — one verified extractor per provider, registered only after
//     dated end-to-end verification (FR-S2: never speculative; the map ships EMPTY here).
//
// WHY A SIBLING FILE, NOT A REGISTRY MEMBER: provider Registry (registry.go) is deliberately a
// pure data structure (name → Manifest); the extractor registry is process-wide capability
// plumbing, keyed by the same provider Name but orthogonal to manifest storage. A sibling keeps
// Registry pure and gives tests a same-package seam (they mutate eventExtractors directly and
// restore via t.Cleanup — the seam-var idiom; no mutex needed, tests run sequentially).

// EventKind classifies one event-stream line (§9.30 FR-S1).
type EventKind int

const (
	// EventIgnored marks a progress/heartbeat/tool/unknown/unparseable line — not a message or
	// error event. Classification is LENIENT: an event stream carries many event types stagecoach
	// doesn't care about, and a malformed line must never kill the parse.
	EventIgnored EventKind = iota
	// EventTerminal marks the FINAL assistant message event — carries the assembled commit
	// message (self-contained; FR-S2's recorded terminal-event shape).
	EventTerminal
	// EventError marks an in-stream error/abort event (FR-S3). A failed response does NOT
	// reliably produce a nonzero exit (pi documents this), so detection keys on the stream.
	EventError
)

// EventOutcome is one line's classification. Message is the assembled final message
// (EventTerminal); ErrText is the error event's text (EventError).
//
// Extractors are STATELESS per line: registry entries are singletons shared by concurrent roles
// (decompose runs roles concurrently), so a stateful accumulator would be a data race. The
// terminal event is self-contained by FR-S2's recorded-shape model; if a provider's verified
// shape ever needs delta accumulation, the interface grows at this seam THEN.
type EventOutcome struct {
	Kind    EventKind
	Message string
	ErrText string
}

// Extractor is the per-provider event-stream classifier (§9.30 FR-S1). S2 refines S1's 3-return
// placeholder into this single-outcome shape: classify one raw NDJSON-ish stdout line. There is
// NO error return — unknown/unparseable lines classify as EventIgnored (lenient, mirroring
// ParseOutput's fallback philosophy; each extractor unmarshals its own provider shape and
// ignores what doesn't fit).
type Extractor interface {
	Extract(line string) EventOutcome
}

// eventExtractors maps provider Name → its verified event extractor. EMPTY by design (FR-S2:
// never speculative — Render refuses event_stream for any provider without a registered
// extractor, so an unverified event mode cannot ship). P2.M2 populates via RegisterEventExtractor.
var eventExtractors = map[string]Extractor{}

// LookupEventExtractor returns the provider's registered Extractor, or nil if none is registered
// (the FR-S2 "unverified provider" condition Render's event_stream guard enforces).
func LookupEventExtractor(name string) Extractor {
	return eventExtractors[name]
}

// RegisterEventExtractor installs e as name's event extractor. P2.M2's population seam — called
// from provider construction/registration code AFTER dated verification, never speculatively.
func RegisterEventExtractor(name string, e Extractor) {
	eventExtractors[name] = e
}

// ParseEventStream scans event-mode stdout: split into lines (CRLF-tolerant), skip blank /
// whitespace-only lines, classify each via the provider's registered Extractor. Win rules:
// the LAST terminal event wins; the FIRST error event is recorded; a terminal message BEATS an
// error event (a produced message is a success — FR-S3 fires only when NO terminal message
// event exists). Unknown/ignored lines are skipped silently.
//
// No registered extractor ⇒ ("", false, "") — defensive, never a panic (Render's FR-S2 guard
// makes it unreachable in production). errText feeds EventErrorHint (FR-S3).
func ParseEventStream(raw string, m Manifest) (msg string, ok bool, errText string) {
	ex := LookupEventExtractor(m.Name)
	if ex == nil {
		return "", false, "" // unverified provider — nothing to scan (defensive)
	}
	lastTerminal := ""
	firstErr := ""
	for _, ln := range strings.Split(raw, "\n") {
		ln = strings.TrimRight(ln, "\r") // CRLF-tolerant
		if strings.TrimSpace(ln) == "" {
			continue // blank / whitespace-only line
		}
		switch oc := ex.Extract(ln); oc.Kind {
		case EventTerminal:
			lastTerminal = oc.Message // LAST terminal wins
		case EventError:
			if firstErr == "" {
				firstErr = oc.ErrText // FIRST error recorded
			}
		default: // EventIgnored: progress/heartbeat/tool/unknown — skip
		}
	}
	if lastTerminal != "" {
		return lastTerminal, true, "" // terminal beats error
	}
	return "", false, firstErr
}

// UnwrapEventOutput returns the assembled terminal message in event mode ("" when no terminal
// event was produced), or out unchanged in print mode — the one-line integration for parse
// sites that do NOT route through ParseOutput (the planner's prompt.ParsePlannerOutput and the
// arbiter's prompt.ParseArbiterOutput). Resolve() first, mirroring ParseOutput's idiom.
func UnwrapEventOutput(out string, m Manifest) string {
	if len(m.Resolve().EventStream) == 0 {
		return out // print mode: passthrough
	}
	msg, _, _ := ParseEventStream(out, m)
	return msg
}

// EventErrorHint is the FR-S3 precedence sibling of StderrHint (executor.go). It returns ""
// unless the manifest is in event mode AND no terminal message was produced AND an error event
// exists; otherwise it returns a single-line, newline-collapsed, 1000-char-capped
// "agent event stream reported an error: <text>" (mirroring StderrHint's formatting idiom).
//
// WHY IT MUST PRECEDE StderrHint AT THE CAUSE SITES: StderrHint returns "" whenever stdout has
// non-whitespace content — in event mode stdout ALWAYS carries events, so it can never surface
// an in-stream error. The cause sites check this hint FIRST; StderrHint keeps serving print mode.
func EventErrorHint(raw string, m Manifest) string {
	if len(m.Resolve().EventStream) == 0 {
		return "" // print mode: not an event-mode failure
	}
	_, ok, errText := ParseEventStream(raw, m)
	if ok || errText == "" {
		return "" // a terminal message exists (success), or no error event to surface
	}
	hint := strings.TrimSpace(errText)
	// collapse to a single line and cap so a noisy provider can't flood the error (StderrHint's idiom)
	hint = strings.ReplaceAll(hint, "\n", " ")
	if len(hint) > 1000 {
		hint = hint[:1000] + "…"
	}
	return "agent event stream reported an error: " + hint
}
