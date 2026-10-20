package provider

// Event-extractor registry for the event-stream output mode (§9.30 FR-S1).
//
// OWNERSHIP (P2.M1.T1):
//   - S1 (this file): the Extractor SHAPE + the registry seam (map + Lookup/Register).
//   - P2.M1.T1.S2: the NDJSON parsing + terminal-event + FR-S3 error-surfacing SEMANTICS.
//   - P2.M2.T1.S2–S5: POPULATION — one verified extractor per provider, registered only after
//     dated end-to-end verification (the builtin.go VERIFIED-comment discipline).
//
// WHY A SIBLING FILE, NOT A REGISTRY MEMBER: provider Registry (registry.go) is deliberately a
// pure data structure (name → Manifest); the extractor registry is process-wide capability
// plumbing, keyed by the same provider Name but orthogonal to manifest storage. A sibling keeps
// Registry pure and gives tests a same-package seam (they mutate eventExtractors directly and
// restore via t.Cleanup — the seam-var idiom; no mutex needed, tests run sequentially).

// Extractor is the per-provider terminal-event scanner for event-stream output (§9.30 FR-S1).
// S1 defines the shape; P2.M1.T1.S2 implements the real semantics behind it.
type Extractor interface {
	// Extract scans one raw stdout line of an event-stream invocation.
	//
	// terminal=true marks the FINAL assistant message; message is its text (the commit
	// message). err wraps a malformed-event condition (S2 defines the error semantics).
	// Non-terminal lines return terminal=false with an empty message (intermediate
	// progress/system events are consumed silently by S2's executor loop).
	Extract(line string) (message string, terminal bool, err error)
}

// eventExtractors maps provider Name → its verified event extractor. EMPTY in S1 by design
// (FR-S2: never speculative — Render refuses event_stream for any provider without a registered
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
