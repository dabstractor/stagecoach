//go:build integration_real

// Package generate test: the FR-S2 EVENT-STREAM VERIFICATION PROTOCOL (PRD §9.30) — the empirical,
// dated runbook P2.M2.T1.S2–S5 execute per provider BEFORE declaring event_stream + registering a
// real extractor. THE RUNBOOK (this header IS the protocol; no separate docs file):
//
//	WHAT — the three proofs a candidate event mode must deliver on a REAL generation:
//	  (i)   PROGRESSIVE EMISSION: ≥2 non-blank stdout lines at distinct arrival offsets AND the
//	        first line arrives ≥minSpan before process exit (default 500ms, env-tunable via
//	        STAGECOACH_VERIFY_MIN_SPAN_MS). The exit-burst signature (all lines at exit) FAILS —
//	        that is the §9.30 naming trap: a buffered `json` mode is NOT a progressive stream
//	        (claude/cursor/agy offer BOTH json and stream-json; only the progressive one qualifies).
//	  (ii)  TERMINAL-EVENT SHAPE: every stdout line is dumped VERBATIM with its arrival offset; the
//	        LAST line is the candidate terminal event whose exact shape becomes S2–S5's Extractor.
//	  (iii) IN-STREAM ERROR SURFACING: a second invocation with the model forced to
//	        "stagecoach-verify-invalid-model"; the stream, EXIT CODE (FR-S3's premise: failures do
//	        NOT reliably exit nonzero — pi documents this), stderr, and any error/abort-looking line
//	        are recorded VERBATIM. The exit code is EVIDENCE, never an assertion target.
//
//	MANUAL COMMAND (one provider at a time; mirrors TestRealAgents' gating):
//
//	  STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/ -run TestEventStreamVerify/<name> -v -timeout 10m
//
//	HEARTBEAT/SPARSE JUDGMENT (the human's duty, recorded in the template): the harness REPORTS
//	inter-line gaps (max gap; count >1s). Codex `--json` timer/heartbeat events continue while the
//	network is dead — liveness, NOT progress. Opencode `--format json` emits sparsely — fine for a
//	60s idle clock, fatal for a naive sub-second one. Read the gap analysis, write the judgment
//	into the "heartbeat/sparse note" of the record.
//
//	RECORDING TEMPLATE — the harness emits the FILLED template via t.Logf on a real run; S2–S5
//	paste it into internal/provider/builtin.go's constructor doc comment (the §7.3
//	`// VERIFIED <date> via <probe>` pattern — builtin.go's SessionMode/ListModelsCommand lines)
//	AND add the dated entry to eventStreamLedger (internal/provider/events_ledger_test.go). Only
//	then: declare event_stream + register the extractor (TestEventStreamLedger guards both).
//
//	RED→GREEN LOOP: Phase A evidence (this harness; probeExtractor satisfies S1's render guard —
//	capture only) → author the real Extractor (S2–S5) → register it in production code → re-run →
//	Phase B green (ParseEventStream/ParseOutput assemble the sentinel message; EventErrorHint
//	surfaces the error) → paste the record + ledger entry + declare. Phase B with no real extractor
//	registered is a SKIP with a pointer to Phase A — the red state must be navigable, not a wall.
//
// SCOPE: test infrastructure ONLY. This harness populates NOTHING: no built-in manifest is
// modified (the trial EventStream flags live on test-local copies; BuiltinManifests is never
// written back), the eventExtractors registry ships EMPTY, no docs file, no production change.
package generate

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dabstractor/stagecoach/internal/provider"
)

// verifySentinel is the fixed reply token the probe payload demands; Phase B's assembly assertion
// keys on it (mechanical, not model-dependent prose).
const verifySentinel = "stagecoach-event-verify"

// verifyPayload: a FIXED, hermetic probe payload — the instruction demands the sentinel reply, and
// a small fixed diff fixture stands in for a live repo (no git identity, no commits, t.TempDir cwd).
const verifyPayload = `You are being verified by stagecoach's event-stream verification harness (FR-S2).
Ignore the diff below entirely; reply with exactly: ` + verifySentinel + `

--- staged diff (fixture; ignore) ---
diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main
+// stagecoach event-stream verification fixture
 func main() { println("hello") }
--- end diff ---`

// eventCandidate is one provider's provisional event-mode flags — THE STARTING POINT ONLY.
type eventCandidate struct {
	flags []string // provisional event_stream tokens (PRD §9.30 FR-S1; S2–S5 verify/adjust + record)
}

// eventCandidates is seeded from §9.30 FR-S1's documented flags. The verification may FALSIFY a
// candidate (that is the point) — S2–S5 record the final disposition in the dated record.
var eventCandidates = map[string]eventCandidate{
	"pi":       {flags: []string{"--mode", "json"}},
	"claude":   {flags: []string{"--output-format", "stream-json"}},
	"cursor":   {flags: []string{"--output-format", "stream-json"}},
	"codex":    {flags: []string{"--json"}},
	"opencode": {flags: []string{"--format", "json"}},
}

// verifyEventFlags returns name's candidate flags, env-overridable per provider via
// STAGECOACH_VERIFY_EVENT_FLAGS_<NAME> (space-separated tokens; empty ⇒ keep the table entry).
func verifyEventFlags(name string) []string {
	if v := os.Getenv("STAGECOACH_VERIFY_EVENT_FLAGS_" + strings.ToUpper(name)); v != "" {
		if fields := strings.Fields(v); len(fields) > 0 {
			return fields
		}
	}
	return eventCandidates[name].flags
}

// verifyModel mirrors TestRealAgents' model plumbing exactly: realConfig (AGENT name into
// cfg.Provider, model from STAGECOACH_REAL_MODEL_<NAME> or the table) + the inference fold
// (STAGECOACH_REAL_INFERENCE_<NAME> → slash-prefix). The AGENT/inference terminology (realagent's
// header) is preserved: inference goes into the MODEL string, NEVER cfg.Provider.
func verifyModel(name string) string {
	cfg := realConfig(name)
	if inf := envOr("STAGECOACH_REAL_INFERENCE_"+strings.ToUpper(name), realDefaults[name].inference); inf != "" {
		switch {
		case cfg.Model == "":
			cfg.Model = inf + "/" + realDefaults[name].model
		case !strings.Contains(cfg.Model, "/"):
			cfg.Model = inf + "/" + cfg.Model
		}
	}
	return cfg.Model
}

// invalidModel forces the error probe's broken model, PRESERVING the inference slash-prefix so
// the agent starts and errors on the MODEL (in-stream / via stderr) rather than the harness
// failing at Render (FR-R5b: a bare model on a provider_flag agent is a render-time hard error).
func invalidModel(model string) string {
	if i := strings.Index(model, "/"); i >= 0 {
		return model[:i+1] + "stagecoach-verify-invalid-model"
	}
	return "stagecoach-verify-invalid-model"
}

// probeExtractor satisfies S1's render guard (Render refuses event_stream without a registered
// extractor) while classifying NOTHING — Phase A is capture, not assembly. There is no
// Unregister API; the manual run is a one-shot process, so a probe left in the seam map is
// harmless (the other generate tests never consult extractors; the map dies with the process).
type probeExtractor struct{}

func (probeExtractor) Extract(line string) provider.EventOutcome {
	return provider.EventOutcome{Kind: provider.EventIgnored}
}

// capturedLine is one stdout line stamped with its ARRIVAL offset — the evidence Execute's
// buffered stdout cannot provide (executor.go returns (stdout, stderr, err) only after Wait).
// Monotonic: one t0 at spawn, time.Since(t0) stamped in the reader goroutine ONLY.
type capturedLine struct {
	offset time.Duration
	text   string
}

// eventCapture is one probe invocation's evidence record. raw is the \n-joined stream — Phase B's
// parse input (the REAL ParseEventStream/ParseOutput path re-drives it).
type eventCapture struct {
	lines    []capturedLine
	raw      string
	exitAt   time.Duration
	exitCode int
	stderr   string
}

// verifyMinSpan is the progressiveness threshold: exitAt − firstLineOffset must reach it.
// Env-tunable (STAGECOACH_VERIFY_MIN_SPAN_MS) because fast/slow models vary; default 500ms.
func verifyMinSpan(t *testing.T) time.Duration {
	t.Helper()
	env := os.Getenv("STAGECOACH_VERIFY_MIN_SPAN_MS")
	ms, err := strconv.Atoi(envOr("STAGECOACH_VERIFY_MIN_SPAN_MS", "500"))
	if err != nil || ms < 0 {
		t.Logf("invalid STAGECOACH_VERIFY_MIN_SPAN_MS %q — using the 500ms default", env)
		return 500 * time.Millisecond
	}
	return time.Duration(ms) * time.Millisecond
}

// probeRun renders the TRIAL manifest and spawns the rendered CmdSpec DIRECTLY — provider.Execute
// returns BUFFERED stdout and cannot provide proof (i)'s per-line arrival offsets — capturing
// every line with a live timestamp, then the exit offset/code and stderr.
func probeRun(t *testing.T, m provider.Manifest, model string) eventCapture {
	t.Helper()

	// S1's render guard: event_stream renders ONLY with a registered extractor. Register the
	// probe (idempotent) BEFORE Render — see probeExtractor's doc for the no-Unregister note.
	provider.RegisterEventExtractor(m.Name, probeExtractor{})

	spec, err := m.Render(model, "", verifyPayload, "off")
	if err != nil {
		t.Fatalf("render %s trial (model %q): %v", m.Name, model, err)
	}
	if !spec.ProgressiveStdout {
		t.Fatalf("rendered %s spec has ProgressiveStdout=false — the event_stream arming guard did not fire (flags lost or guard bug)", m.Name)
	}
	for _, tok := range m.EventStream {
		if !slices.Contains(spec.Args, tok) {
			t.Fatalf("rendered argv is missing candidate token %q (args: %q)", tok, spec.Args)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	cmd.Env = spec.Env
	cmd.Dir = t.TempDir() // hermetic: no live repo, no git identity, no commits
	cmd.Stdin = strings.NewReader(spec.Stdin)

	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	errPipe, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	var cap eventCapture
	var wg sync.WaitGroup
	t0 := time.Now()
	wg.Add(2)
	go func() { // the ONLY goroutine that stamps offsets (monotonic, single t0)
		defer wg.Done()
		sc := bufio.NewScanner(outPipe)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		var raw strings.Builder
		for sc.Scan() {
			line := sc.Text()
			cap.lines = append(cap.lines, capturedLine{time.Since(t0), line})
			raw.WriteString(line)
			raw.WriteByte('\n')
		}
		cap.raw = raw.String()
	}()
	go func() {
		defer wg.Done()
		b, _ := io.ReadAll(errPipe)
		cap.stderr = string(b)
	}()

	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s %q: %v", spec.Command, spec.Args, err)
	}
	err = cmd.Wait()
	wg.Wait()
	cap.exitAt = time.Since(t0)

	if ctx.Err() != nil {
		t.Fatalf("%s did not exit within the harness timeout — killed at %v. For slower agents raise -timeout and this ctx, NOT minSpan", spec.Command, cap.exitAt)
	}
	var ee *exec.ExitError
	switch {
	case err == nil:
		cap.exitCode = 0
	case errors.As(err, &ee):
		cap.exitCode = ee.ExitCode() // EVIDENCE — recorded verbatim, never asserted (FR-S3)
	default:
		t.Fatalf("wait %s: %v", spec.Command, err)
	}
	return cap
}

// nonBlankLines returns the substantive lines (progressiveness + gap analysis ignore whitespace).
func nonBlankLines(cap eventCapture) []capturedLine {
	var nb []capturedLine
	for _, ln := range cap.lines {
		if strings.TrimSpace(ln.text) != "" {
			nb = append(nb, ln)
		}
	}
	return nb
}

// distinctOffsets counts the distinct arrival offsets among nb (proof i's "≥2 at DISTINCT offsets").
func distinctOffsets(nb []capturedLine) int {
	seen := make(map[time.Duration]struct{}, len(nb))
	for _, ln := range nb {
		seen[ln.offset] = struct{}{}
	}
	return len(seen)
}

// gapSummary reports the max inter-line gap and the count of gaps >1s over nb — the
// heartbeat/sparse judgment input (codex timer events; opencode sparse emission).
func gapSummary(nb []capturedLine) (maxGap time.Duration, over1s int) {
	for i := 1; i < len(nb); i++ {
		gap := nb[i].offset - nb[i-1].offset
		if gap > maxGap {
			maxGap = gap
		}
		if gap > time.Second {
			over1s++
		}
	}
	return maxGap, over1s
}

// offsetTable renders the verbatim evidence dump (every line with its arrival offset).
func offsetTable(cap eventCapture) string {
	var b strings.Builder
	for _, ln := range cap.lines {
		fmt.Fprintf(&b, "    +%-12v | %s\n", ln.offset.Round(time.Microsecond), ln.text)
	}
	return b.String()
}

// findCandidateError returns the first line that looks like an error/abort event (verbatim,
// capped) — Phase A's SHAPE record; the human judges, the real Extractor classifies (Phase B).
func findCandidateError(cap eventCapture) string {
	for _, ln := range cap.lines {
		low := strings.ToLower(ln.text)
		if strings.Contains(low, "error") || strings.Contains(low, "abort") {
			if len(ln.text) > 300 {
				return ln.text[:300] + "…"
			}
			return ln.text
		}
	}
	return ""
}

// summarizeStderr collapses stderr to one capped line for the report line.
func summarizeStderr(stderr string) string {
	s := strings.TrimSpace(strings.ReplaceAll(stderr, "\n", " ⏎ "))
	if s == "" {
		return "<empty>"
	}
	if len(s) > 400 {
		s = s[:400] + "…"
	}
	return s
}

// cliVersionRe scrapes a semver-looking token from the probe's stderr for the record's
// "(<cli + version>)" field; not found ⇒ the human fills it (some CLIs are silent on stderr).
var cliVersionRe = regexp.MustCompile(`\b\d+\.\d+\.\d+[^\s,;"']*`)

// verifyProvider executes the full protocol for ONE provider: Phase A (happy-path live capture +
// the mechanical progressiveness check + verbatim dump + gap analysis), the error probe (proof
// iii, record-only), Phase B (the real-extractor trial — SKIP while unregistered), and the
// filled recording-template emission.
func verifyProvider(t *testing.T, name string) {
	flags := verifyEventFlags(name)
	if len(flags) == 0 {
		t.Fatalf("no event-mode candidate flags for %s (table empty and no STAGECOACH_VERIFY_EVENT_FLAGS_%s override)", name, strings.ToUpper(name))
	}

	// Trial manifests — TEST-LOCAL copies. BuiltinManifests() returns a fresh map per call, so the
	// struct is ours; setting the (previously nil) EventStream slice cannot alias anything. The
	// trial is NEVER written back to BuiltinManifests/builtin.go — shipped manifests stay
	// event_stream-free until S2–S5's dated verification. That separation IS the FR-S2 discipline.
	m := provider.BuiltinManifests()[name]
	m.EventStream = flags
	mErr := provider.BuiltinManifests()[name]
	mErr.EventStream = flags

	// Install gate (mirrors TestRealAgents' per-subtest IsInstalled skip).
	r := m.Resolve()
	if _, err := exec.LookPath(*r.Command); err != nil {
		t.Skipf("%s (%s) not on $PATH", name, *r.Command)
	}

	// Save any PRODUCTION extractor BEFORE the probe overwrites the seam; restored just before
	// Phase B (ParseEventStream/EventErrorHint consult the live map). nil ⇒ the red→green loop's
	// red state (Phase B SKIPs; Phase A evidence is the deliverable).
	realEx := provider.LookupEventExtractor(name)
	_, realExIsProbe := realEx.(probeExtractor)

	model := verifyModel(name)
	t.Logf("[%s] candidate event flags: %v | model: %q | minSpan: %v", name, flags, model, verifyMinSpan(t))

	// ---- Phase A: happy-path live capture (proofs i + ii) ----
	cap := probeRun(t, m, model)
	nb := nonBlankLines(cap)

	t.Logf("[%s] captured %d lines (exit @%v, exit code %d)", name, len(cap.lines), cap.exitAt, cap.exitCode)
	t.Logf("[%s] ---- VERBATIM STREAM (arrival offset | line) ----", name)
	t.Logf("%s", strings.TrimRight(offsetTable(cap), "\n"))
	if cap.stderr != "" {
		t.Logf("[%s] ---- STDERR (verbatim) ----\n    %s", name, summarizeStderr(cap.stderr))
	}

	first := time.Duration(0)
	if len(nb) > 0 {
		first = nb[0].offset
	}
	maxGap, over1s := gapSummary(nb)
	t.Logf("[%s] gap analysis: %d non-blank lines, max inter-line gap %v, %d gaps >1s (heartbeat/sparse judgment input)", name, len(nb), maxGap, over1s)

	// Proof (i) — the mechanical progressiveness criterion; the exit-burst signature FAILS here.
	minSpan := verifyMinSpan(t)
	if len(nb) < 2 || distinctOffsets(nb) < 2 || cap.exitAt-first < minSpan {
		t.Fatalf("NOT progressive (FR-S2 proof i FAILED): %d non-blank lines (%d distinct offsets), first @%v, exit @%v (span %v < %v). "+
			"Exit-burst = the buffered `json` mode, not a stream (§9.30 naming trap: claude/cursor offer BOTH json and stream-json — only the progressive one qualifies). "+
			"Offset table:\n%s", len(nb), distinctOffsets(nb), first, cap.exitAt, cap.exitAt-first, minSpan, offsetTable(cap))
	}
	t.Logf("[%s] proof (i) PROGRESSIVE: PASS — %d lines, first @%v, exit @%v (span %v ≥ %v)", name, len(nb), first, cap.exitAt, cap.exitAt-first, minSpan)

	// Proof (ii) — the terminal-event SHAPE (verbatim; becomes S2–S5's Extractor input).
	t.Logf("[%s] proof (ii) TERMINAL-EVENT SHAPE — candidate terminal line (verbatim): %s", name, nb[len(nb)-1].text)

	// ---- Error probe (proof iii): forced invalid model. RECORD ONLY — the exit code is evidence,
	// never an assertion (FR-S3: failures do NOT reliably exit nonzero). ----
	errCap := probeRun(t, mErr, invalidModel(model))
	t.Logf("[%s] ---- ERROR PROBE (model %q) — verbatim stream ----", name, invalidModel(model))
	t.Logf("%s", strings.TrimRight(offsetTable(errCap), "\n"))
	t.Logf("[%s] error probe: exit=%d (EVIDENCE, not asserted); stderr: %s", name, errCap.exitCode, summarizeStderr(errCap.stderr))
	if cand := findCandidateError(errCap); cand != "" {
		t.Logf("[%s] proof (iii) candidate error/abort event (verbatim): %s", name, cand)
	} else {
		t.Logf("[%s] proof (iii) candidate error/abort event: none observed in-stream (check stderr/exit above)", name)
	}

	// ---- Phase B: the extractor trial (the red→green loop's green state) ----
	if realEx == nil || realExIsProbe {
		emitRecordingTemplate(t, name, flags, cap, errCap, false, false)
		t.Skipf("no real extractor registered for %s yet — Phase A evidence above is the deliverable; "+
			"author + register the extractor (P2.M2.T1.S2–S5), then re-run for Phase B", name)
	}
	provider.RegisterEventExtractor(name, realEx) // restore the production seam for the live lookups

	msg, ok, happyErrText := provider.ParseEventStream(cap.raw, m)
	if !ok || !strings.Contains(msg, verifySentinel) {
		t.Errorf("ParseEventStream did not assemble the sentinel (ok=%v, msg=%q) — adjust the Extractor's terminal-event branch against the Phase A verbatim dump", ok, msg)
	}
	pmsg, pok, _ := provider.ParseOutput(cap.raw, m)
	if !pok || !strings.Contains(pmsg, verifySentinel) {
		t.Errorf("ParseOutput event-mode assembly failed (ok=%v, msg=%q) — the step-0 terminal-event path", pok, pmsg)
	}
	if happyErrText != "" {
		t.Logf("[%s] note: the happy-path stream ALSO classified an error event: %q", name, happyErrText)
	}

	_, _, probeErrText := provider.ParseEventStream(errCap.raw, mErr)
	hint := provider.EventErrorHint(errCap.raw, mErr)
	hintSurfaced := false
	if probeErrText != "" { // an error event was classified — FR-S3's mechanical proof must hold
		if hint == "" {
			t.Errorf("EventErrorHint empty though the error probe classified an error event — FR-S3 mechanical proof failed")
		} else {
			hintSurfaced = true
			t.Logf("[%s] proof (iii) EventErrorHint: %s", name, hint)
		}
	} else {
		t.Logf("[%s] note: the error probe produced NO classifiable in-stream error event — record the observed shape (exit=%d, stderr) in the template's (iii) field", name, errCap.exitCode)
	}

	emitRecordingTemplate(t, name, flags, cap, errCap, true, hintSurfaced)
}

// emitRecordingTemplate t.Logfs the FILLED §7.3 recording template, ready to paste into
// builtin.go's constructor doc comment (plus the eventStreamLedger dated entry).
func emitRecordingTemplate(t *testing.T, name string, flags []string, cap, errCap eventCapture, extractorRegistered, hintSurfaced bool) {
	t.Helper()

	nb := nonBlankLines(cap)
	span, first := time.Duration(0), time.Duration(0)
	if len(nb) > 0 {
		first = nb[0].offset
		span = cap.exitAt - first
	}
	maxGap, over1s := gapSummary(nb)

	version := "<record cli + version>"
	if m := cliVersionRe.FindString(cap.stderr); m != "" {
		version = m
	}
	terminal := "<none observed>"
	if len(nb) > 0 {
		terminal = nb[len(nb)-1].text
	}
	errEvent := findCandidateError(errCap)
	if errEvent == "" {
		errEvent = "none observed"
	}
	hintCol := "pending-extractor"
	if extractorRegistered {
		if hintSurfaced {
			hintCol = "yes"
		} else {
			hintCol = "no"
		}
	}
	extractorCol := "NOT REGISTERED: author + register the extractor (P2.M2.T1.S2–S5), then re-run"
	if extractorRegistered {
		extractorCol = "registered in production (cite the S-ref when pasting)"
	}

	date := time.Now().Format("2006-01-02")
	t.Logf("RECORDING TEMPLATE (paste into builtin.go constructor doc comment §7.3 + add the dated eventStreamLedger entry):")
	for _, ln := range []string{
		fmt.Sprintf("// EVENT-STREAM (§9.30 FR-S1/FR-S2) — VERIFIED %s (%s %s), harness P2.M2.T1.S1", date, name, version),
		fmt.Sprintf("// (`STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/ -run TestEventStreamVerify/%s -v`):", name),
		fmt.Sprintf("//   (i)   PROGRESSIVE: %d lines over %v (first @%v, exit @%v; max gap %v, %d gaps >1s) — no exit-burst.", len(nb), span, first, cap.exitAt, maxGap, over1s),
		fmt.Sprintf("//   (ii)  TERMINAL EVENT (verbatim): %s", terminal),
		fmt.Sprintf("//   (iii) ERROR (forced invalid model): exit=%d; error event (verbatim): %s; EventErrorHint: %s", errCap.exitCode, errEvent, hintCol),
		fmt.Sprintf("//   event_stream = [%s] — extractor: %s;", strings.Join(flags, " "), extractorCol),
		"//   heartbeat/sparse note: <FILL from the gap analysis — judge liveness vs progress (codex timer events ≠ progress; opencode sparse gaps)>",
	} {
		t.Logf("%s", ln)
	}
}

// TestEventStreamVerify is the gated entry point: one subtest per candidate provider (registry
// preference order via providerNames; agy EXCLUDED, mirroring realagent's experimental-CLI
// exclusion), each runnable individually via -run TestEventStreamVerify/<name>.
func TestEventStreamVerify(t *testing.T) {
	if os.Getenv("STAGECOACH_RUN_REAL") != "1" {
		t.Skip("skipping event-stream verification; set STAGECOACH_RUN_REAL=1 and build with -tags integration_real (the runbook is this file's header comment)")
	}

	for _, name := range providerNames {
		if _, ok := eventCandidates[name]; !ok {
			continue // providerNames may drift ahead of the candidate table
		}
		t.Run(name, func(t *testing.T) {
			verifyProvider(t, name)
		})
	}
}
