package provider

import (
	"strings"
	"testing"
)

// The FR-S2 enforcement ledger (PRD §9.30; §9.26 work-description mode): the mechanical record of
// WHICH providers have event_stream populated and WHEN each was empirically verified.
//
// THE RULE: populating event_stream without a passing dated record is FORBIDDEN. A built-in
// manifest may declare EventStream tokens ONLY alongside
//   1. a dated entry in eventStreamLedger below (added by P2.M2.T1.S2–S5 together with the
//      declaration + extractor registration), and
//   2. the VERIFIED record pasted into builtin.go's constructor doc comment (the §7.3
//      `// VERIFIED <date> via <probe>` precedent — see the SessionMode/ListModelsCommand lines).
//
// The VERIFICATION itself is the empirical protocol executed by the harness in
// internal/generate/events_verify_test.go (TestEventStreamVerify, build tag integration_real,
// STAGECOACH_RUN_REAL=1): a REAL generation in the candidate event mode proving (i) progressive
// emission, (ii) the terminal-event shape, (iii) in-stream error surfacing — see that file's
// header runbook. This test is the always-on teeth: declared ⇔ recorded, both directions, with
// the date mandatory. It ships GREEN with an EMPTY ledger and NO built-in declaring event_stream.

// eventStreamLedger maps provider Name → the VERIFIED date ("YYYY-MM-DD") of its passing
// empirical event-stream run. Ships EMPTY by design (FR-S2: nothing is populated yet).
// P2.M2.T1.S2–S5 add an entry ONLY together with a passing dated run recorded in builtin.go's
// constructor doc comment, alongside the EventStream declaration + RegisterEventExtractor call.
var eventStreamLedger = map[string]string{
	// "pi": "2026-07-21", // ← the shape S2–S5 will add (dated, probe-named, alongside the builtin.go record)
}

// TestEventStreamLedger is the FR-S2 population guard: every built-in declaring event_stream must
// have a dated ledger entry (no speculative declaration), every ledger entry must correspond to a
// declaration (no stale record), and no entry may lack its date. Failure text points at the
// protocol so the fix is always "run the harness, record the evidence" — never "delete the guard".
func TestEventStreamLedger(t *testing.T) {
	builtins := BuiltinManifests()

	for name, m := range builtins {
		declared := len(m.Resolve().EventStream) > 0
		date, recorded := eventStreamLedger[name]

		if declared && !recorded {
			t.Errorf("%s declares event_stream but the FR-S2 ledger has no dated verification record — "+
				"run the P2.M2.T1.S1 protocol (internal/generate/events_verify_test.go, TestEventStreamVerify: "+
				"STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/ -run TestEventStreamVerify/%s -v) "+
				"and record the dated evidence in builtin.go's constructor doc comment (§7.3 VERIFIED pattern) "+
				"BEFORE populating; then add the dated entry to eventStreamLedger", name, name)
		}
		if recorded && !declared {
			t.Errorf("%s has an FR-S2 ledger entry (verified %s) but the manifest declares no event_stream — "+
				"stale ledger entry; remove it (the record must mirror the shipped declaration)", name, date)
		}
		if recorded && strings.TrimSpace(date) == "" {
			t.Errorf("%s FR-S2 ledger entry has an empty date — the entry must carry the VERIFIED date "+
				"(YYYY-MM-DD) of its passing empirical run", name)
		}
	}

	// Ledger entries for names that are not built-ins are stale by definition (the ledger mirrors
	// BuiltinManifests; the main loop above cannot see orphan keys).
	for name := range eventStreamLedger {
		if _, ok := builtins[name]; !ok {
			t.Errorf("FR-S2 ledger entry %q has no matching built-in manifest — stale entry; remove it", name)
		}
	}
}
