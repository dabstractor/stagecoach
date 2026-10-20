name: "P2.M2.T1.S2 — pi (--mode json) event-stream verification + disposition (§9.30 FR-S1/S2/S3, → G30)"
description: |
  The EMPIRICAL, machine-local verification of pi's `--mode json` event mode and the landing of its
  disposition. Runs S1's harness (TestEventStreamVerify/pi) against the INSTALLED pi CLI (verified
  present: /home/dustin/.local/bin/pi, v1.0.2, `--mode json` exists in --help) to produce the three
  dated proofs — (i) progressive emission (≥2 lines, first ≥500ms pre-exit), (ii) the terminal
  message-event shape (verbatim → becomes the pi Extractor), (iii) in-stream error surfacing (forced
  invalid model; exit code recorded as EVIDENCE, never asserted — FR-S3's exit-0 premise). ON PASS:
  land the production population — NEW internal/provider/events_pi.go (init → RegisterEventExtractor
  ("pi", …), stateless classifier per the captured shapes + pi docs/json.md), builtinPi() gains
  `EventStream: []string{"--mode","json"}` + the dated VERIFIED record in its doc comment (Mode A —
  the record IS the doc), the ledger entry in events_ledger_test.go, fixture unit tests
  (events_pi_test.go) from the VERBATIM captures, and the Phase B re-run green (red→green). ON FAIL
  or ABSENT/UNAUTHENTICATED: EventStream stays absent with the dated NEGATIVE record in the ctor doc
  comment — an equally valid disposition (FR-S2: never speculative; shipping [] + the reason IS a
  disposition). No mocks. No parse/render changes (parse.go:59 already wires ParseEventStream).

---

## Goal

**Feature Goal**: pi's event-stream disposition is decided EMPIRICALLY and landed: a dated,
recorded run of `pi --mode json` through S1's harness proving (or disproving) the three FR-S2
proofs, followed by the matching production population (extractor + manifest tokens + dated record
+ ledger entry + tests) or the documented negative disposition.

**Deliverable** (PASS branch): (1) the filled dated VERIFIED record pasted into `builtinPi()`'s doc
comment (`internal/provider/builtin.go`) + `EventStream: []string{"--mode", "json"}`; (2) NEW
`internal/provider/events_pi.go` — the registered, stateless pi Extractor; (3) the ledger entry
`"pi": "<date>"` in `eventStreamLedger` (`internal/provider/events_ledger_test.go` — S1's file,
map-literal edit only); (4) NEW `internal/provider/events_pi_test.go` — fixture tests built from
the VERBATIM captured lines; (5) the Phase B harness re-run GREEN. (FAIL/ABSENT branch: only the
dated negative record; nothing populated; ledger untouched — documented in the implementation
report.)

**Success Definition**: The disposition is recorded, dated, and reproducible; every populated claim
is backed by the capture (FR-S2); `TestEventStreamLedger` GREEN (declaration ⇔ dated ledger entry);
the harness's Phase B mechanically proves assembly (sentinel message) + error hint on this
machine's real pi; all always-on suites stay green; no file outside the sanctioned set is touched.

## User Persona (if applicable)

**Target User**: The stagecoach maintainer/verifier executing the FR-S2 population plan (and the
orchestrator marking P2.M2.T1.S2 done).

**Use Case**: pi is the FIRST verified event-mode provider (P2.M2.T1.S2 of S2–S5; the others
follow this task's shape). The maintainer runs the dated probe once, lands extractor+tokens (or the
negative record), and pi gains portable idle detection (stdout becomes a continuous activity
channel, FR-S1/S4) + in-stream error surfacing (FR-S3) — or the codebase gains an honest record of
why pi cannot (yet).

**User Journey**: run the harness subtest → read the timestamped evidence + verbatim shapes →
decide PASS/FAIL per the mechanical criteria → author the extractor from the captured terminal
shape (pi docs/json.md as design input; the capture as authority) → land manifest+record+ledger+tests
→ re-run Phase B → green → commit.

**Pain Points Addressed**: today pi's failures surface as opaque empty-stdout parse errors (FR-S3);
idle detection relies on the Linux-only procfs read-counter (FR-A3(b)); and nothing about pi's
`--mode json` is recorded anywhere — the manifest cannot ship it (FR-S2 forbids speculation). This
task converts all three unknowns into dated facts.

## Why

- **FR-S2's verification bar is executed here, not assumed**: the PRD seeds `pi --mode json` as the
  candidate (FR-S1) but forbids shipping it unverified. S1 built the protocol; THIS task is its
  first execution and the template for S3–S5.
- **FR-S3 (error surfacing) has pi's name on it**: "a failed or aborted response … does NOT
  reliably produce a nonzero exit (pi documents this explicitly)". pi is the provider whose error
  detection MUST live in the stream parse — the error probe's captured shape becomes the
  extractor's EventError branch and EventErrorHint's input.
- **Portability payoff (FR-S4)**: on pass, pi's stdout becomes a continuous activity channel on
  every platform — removing the Linux-only procfs dependency for idle detection.
- **The registry is EMPTY by design until this task**: events.go's `eventExtractors` map and the
  ledger ship empty (S1/S2-of-P2.M1); pi's verified registration is the first real population.

## What

A two-phase task: **Phase 1 — the empirical run** (S1's harness, no code changes); **Phase 2 — the
disposition landing** (population or negative record). All pass criteria are mechanical.

### Phase 1 — the dated empirical run (S1's harness)

```bash
cd /home/dustin/projects/stagecoach
STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/ \
  -run 'TestEventStreamVerify/pi' -v -timeout 10m   2>&1 | tee /tmp/pi_events_verify.log
```

- The candidate tokens are seeded `--mode json` (S1's table); override with
  `STAGECOACH_VERIFY_EVENT_FLAGS_PI` only if the seeded form fails to render/run.
- The model comes from the harness's realagent plumbing (`realDefaults["pi"] =
  {"claude-haiku","anthropic"}` — model `anthropic/claude-haiku` after FR-R5b's split; pi is
  multi-backend). If the default model errors (transient 529/503 — pi's auto-retry handles it),
  re-run; record any override used.
- **Proof (i) — progressive**: ≥2 non-blank lines at distinct offsets AND `exit − firstLine ≥
  500ms` (tunable `STAGECOACH_VERIFY_MIN_SPAN_MS`). pi's documented stream (session header at
  start; `message_update` text_delta's during generation) should pass trivially — but the check
  decides, not the doc. EXIT-BURST = FAIL (buffered json masquerading as a stream).
- **Proof (ii) — terminal-event shape**: capture the verbatim terminal line(s). Expected per
  pi docs/json.md: `{"type":"message_end","message":{"role":"assistant","content":[…],…}}` —
  "the authoritative final message". Also record `turn_end`/`agent_end` verbatim (secondary
  shapes; `agent_end` carries `messages`+`willRetry`). **The capture is the authority; the doc is
  design input.** If they differ, follow the capture.
- **Proof (iii) — error surfacing**: the harness's forced `stagecoach-verify-invalid-model` probe;
  record the verbatim error-event line(s) + EXIT CODE + stderr. Candidates per docs/json.md:
  provider-level `{"type":"error",…}`, `auto_retry_end {success:false, finalError}`, or a
  `message_end{role:"assistant"}` carrying an error/empty content. **Record the exit code as
  EVIDENCE — never assert nonzero** (FR-S3's premise).
- **Gap analysis**: read the harness's max-gap/offset table; write the heartbeat/sparse judgment
  for the record (expected: dense deltas — but say what the capture shows).

Save the full `t.Logf` output (it contains the filled recording template S1 emits) into
`plan/022_856008a8cbb5/P2M2T1S2/research/pi_capture_<date>.log` — the durable evidence.

### Phase 2 — the disposition landing

**BRANCH PASS** (all three proofs green):

1. **NEW `internal/provider/events_pi.go`** — package provider:
   - `func init() { RegisterEventExtractor("pi", piEventExtractor{}) }`
   - `type piEventExtractor struct{}` — STATELESS (registry singletons are shared by concurrent
     roles; no accumulator state — events.go's interface contract).
   - `func (piEventExtractor) Extract(line string) EventOutcome`: unmarshal the line as JSON;
     classification (adjust to the CAPTURED shapes where they differ):
     - `type=="message_end"` AND `message.role=="assistant"` → `EventTerminal` with `Message` =
       concatenate the text blocks of `message.content` (message-types.md block shape; skip
       thinking/tool_call blocks; the captured shape governs the exact field names).
     - Error indications → `EventError` with `ErrText`: per the captured error shape — e.g.
       `type=="error"` (provider error), `type=="auto_retry_end"` with `success==false`
       (`finalError`), or the captured equivalent. Keep it minimal: classify ONLY what the capture
       proved; everything unknown stays EventIgnored (lenient by design).
     - EVERYTHING else → `EventIgnored`: `session` header, `agent_start`, `turn_start`,
       `message_start`, `message_update` (delta-only — NEVER accumulate), `turn_end`,
       `tool_execution_*`, `agent_end`, `agent_settled`, `compaction_*`, `entry_appended`,
       `session_info_changed`, non-JSON/blank lines.
     - Unmarshal failure → `EventIgnored` (no error return — the interface is lenient).
   - Dated doc comment citing the harness run (the §7.3 precedent).
2. **MODIFY `internal/provider/builtin.go` `builtinPi()`**: add `EventStream: []string{"--mode",
   "json"},` (with an inline comment naming the dated verification + extractor file) and paste
   S1's filled recording template into the constructor's doc comment (extend the existing
   "FULLY VERIFIED vs `pi --help`" header — do not replace it). THIS RECORD IS THE TASK'S
   DOCUMENTATION (Mode A).
3. **MODIFY `internal/provider/events_ledger_test.go`** (S1's file — map literal ONLY):
   `eventStreamLedger` gains `"pi": "<run-date>"`.
4. **NEW `internal/provider/events_pi_test.go`** (always-on, package provider): fixtures built by
   PASTING the verbatim captured lines (dated comment citing the run): the terminal `message_end`
   (asserts EventTerminal + assembled text), each captured error shape (asserts EventError +
   ErrText), representative ignored shapes (session header, message_update delta, agent_end,
   agent_settled — assert EventIgnored), a composite-stream `ParseEventStream` test (LAST terminal
   wins; terminal beats error; first error recorded), `LookupEventExtractor("pi") != nil`, and
   `EventErrorHint` non-empty over the error fixture.
5. **Re-run Phase B** — the harness now finds the real extractor: assert the sentinel
   ("stagecoach-event-verify") is assembled and EventErrorHint fires on the error capture.
   RED→GREEN complete.
6. **providers/pi.toml parity**: if `providers/pi.toml` carries per-field dated comments (check:
   `grep -n 'VERIFIED' providers/pi.toml`), mirror the same dated event-stream record + an
   `event_stream = ["--mode", "json"]` line there (reference-doc parity, matching the sibling
   manifests' convention). If it carries no such convention, leave it and note that in the report.

**BRANCH FAIL / ABSENT / UNAUTHENTICATED** (any proof fails; or pi cannot run — not installed,
unauthenticated, no usable model):

- `builtinPi()` keeps NO EventStream; paste the dated NEGATIVE record into its doc comment:
  what was run (exact command + date + pi version), which proof failed, the verbatim
  counter-evidence (e.g. the exit-burst offset table; or the auth error), and the retry conditions.
- No extractor, no ledger entry, no tests. `TestEventStreamLedger` stays green (nothing declared).
- This IS a complete deliverable — the implementation report states the disposition explicitly.

### Success Criteria

- [ ] The dated empirical run executed on this machine; full t.Logf evidence saved to
      `plan/022_856008a8cbb5/P2M2T1S2/research/pi_capture_<date>.log`.
- [ ] Disposition decided by the mechanical criteria (PASS iff all three proofs green).
- [ ] PASS branch: extractor registered (stateless, capture-shaped), manifest tokens + dated record
      landed, ledger entry added, fixture tests green, Phase B re-run green.
- [ ] FAIL branch: dated negative record landed; nothing populated; ledger green on the empty state.
- [ ] `TestEventStreamLedger` GREEN in either branch (declaration ⇔ dated entry).
- [ ] `go build ./...`, `go vet ./internal/provider/...`, `go vet -tags integration_real
      ./internal/generate/`, `go test ./internal/provider/... ./internal/generate/... -race`,
      gofmt — all clean.
- [ ] Only sanctioned files touched (see Scope-Boundary checklist).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed?_ **Yes** — the
harness command + env knobs (S1's contract), the installed-CLI ground truth (verified: pi 1.0.2,
`--mode json` in `--help`), pi's canonical event-shape documentation with the exact JSON records,
the landed framework API (verified signatures at events.go), the manifest's current fields, the
disposition branches with mechanical criteria, the §7.3 dated-record precedent with live examples,
and the per-branch file/scope boundaries. The one inherent unknown — pi's actual runtime stream —
is the thing the task exists to capture; the PRP equips the implementer to classify whatever
arrives (doc-informed, capture-authoritative).

### Documentation & References

```yaml
# MUST READ — this task's research (verified live at HEAD 45c57a3).
- docfile: plan/022_856008a8cbb5/P2M2T1S2/research/findings.md
  why: "§0 installed-CLI ground truth; §1 pi's documented event shapes (the extractor's design
        input + the delta-ONLY warning); §2 the landed events.go API (LAST-terminal-wins etc.);
        §3 the pi manifest today; §4 S1's harness contract (command, env knobs, proofs, template);
        §5 where production registration goes; §6 the disposition branches; §7 scope guards."

# MUST READ — the harness CONTRACT (this task executes it; its files are off-limits except the ledger).
- docfile: plan/022_856008a8cbb5/P2M2T1S1/PRP.md
  why: "The protocol: TestEventStreamVerify/pi manual command + STAGECOACH_RUN_REAL gate; the
        three proofs' mechanical criteria; the recording template + paste destination; the ledger
        mechanism; Phase B's sentinel assert; the probe-extractor workaround; agy exclusion
        (irrelevant here — pi only)."
  gotcha: "If S1 lands slightly differently than its PRP (names/env), follow the LANDED
           events_verify_test.go header runbook — the harness file IS the authority."

# MUST READ — the FR-S spec (what 'verified' means; the naming trap; the pitfalls).
- docfile: plan/022_856008a8cbb5/prd_snapshot.md
  section: "§9.30 (line ~673): FR-S1 (event-mode table; pi --mode json is named; the
            json-vs-stream-json trap), FR-S2 (the verification bar + heartbeat/sparse pitfalls),
            FR-S3 (error surfacing; the exit-0 premise names pi), FR-S4 (phasing/portability)."

# MUST READ — pi's canonical JSON-mode documentation (the extractor's design input).
- url: file:///home/dustin/.local/lib/node_modules/@earendil-works/pi-coding-agent/docs/json.md
  why: "The event taxonomy + verbatim shapes: session header; message_start/update/end (message_end
        = 'the authoritative final message'); turn_end; agent_end (messages, willRetry);
        agent_settled; auto_retry_*/compaction_* error carrying; the delta-only message_update
        warning ('Replace the whole partial message with message_end.message')."
  critical: "The doc is DESIGN INPUT. The captured stream on this machine/build is the AUTHORITY —
             where they differ, the extractor follows the capture (record the difference)."
- url: file:///home/dustin/.local/lib/node_modules/@earendil-works/pi-coding-agent/docs/message-types.md
  why: "The assistant message.content block array (text/thinking/tool_call blocks) — how to
        assemble Message text from the terminal event."

# MUST READ — the landed framework this task populates (verified signatures).
- file: internal/provider/events.go
  why: "Extractor/EventOutcome/EventKind; the EMPTY eventExtractors map ('P2.M2 populates via
        RegisterEventExtractor — called from provider construction/registration code AFTER dated
        verification'); ParseEventStream semantics (LAST terminal wins, FIRST error recorded,
        terminal BEATS error); EventErrorHint; the STATELESS-per-line contract."
- file: internal/provider/builtin.go
  why: "builtinPi() at :49 (the manifest to modify) + the §7.3 dated-VERIFIED comment precedent
        (:29 ctor header, :54/:61 field examples). Add EventStream + the record; change nothing else."
- file: internal/provider/parse.go
  why: "READ-ONLY: :59 already calls ParseEventStream in ParseOutput — NO parse changes needed.
        Confirms the extractor alone activates event-mode assembly."
- file: internal/generate/realagent_test.go
  why: "READ-ONLY: :40 realDefaults['pi'] = {'claude-haiku','anthropic'} (the harness's model
        plumbing; agent-vs-inference distinction :28-33). pi is multi-backend — FR-R5b."

# REFERENCE — the ledger file (S1's; map-literal edit ONLY).
- file: internal/provider/events_ledger_test.go   # (S1 lands it; if absent, S1 hasn't merged — see Gotcha 1)
  why: "eventStreamLedger map — add \"pi\": \"<date>\" on the PASS branch only."

# REFERENCE — the architecture doc (registry mechanics + the dated-record rule).
- docfile: plan/022_856008a8cbb5/architecture/provider_subsystem.md
  section: "§7.1 (BuiltinManifests fresh-map-per-call — copy before mutating, never write back),
            §7.3 (the VERIFIED-comment pattern: 'new per-provider capability claim must carry this
            same dated, probe-named verification')."
```

### Current Codebase tree (relevant slice)

```bash
# Run from repo root: cd /home/dustin/projects/stagecoach
internal/provider/events.go            # the landed framework; EMPTY registry (populate via events_pi.go)
internal/provider/parse.go             # :59 ParseOutput→ParseEventStream (READ-ONLY)
internal/provider/render.go            # EventStream tokens → spec.Args; ProgressiveStdout arming (READ-ONLY)
internal/provider/builtin.go           # builtinPi() :49 — EventStream + dated record land here
internal/provider/events_ledger_test.go# S1's ledger (map-literal edit only) [S1 in flight]
internal/generate/events_verify_test.go# S1's harness — the protocol this task RUNS [S1 in flight]
internal/generate/realagent_test.go    # the harness's helpers/model plumbing (READ-ONLY)
providers/pi.toml                      # reference doc — parity mirror if it carries dated comments
plan/022_856008a8cbb5/P2M2T1S1/PRP.md  # the harness CONTRACT
plan/022_856008a8cbb5/P2M2T1S2/research/findings.md  # this task's research
```

### Desired Codebase tree with files to be added

```bash
internal/provider/
  events_pi.go        # NEW (PASS branch) — init() registration + stateless piEventExtractor
  events_pi_test.go   # NEW (PASS branch) — fixture tests from the VERBATIM captures
# MODIFIED (PASS branch): builtin.go (builtinPi doc comment + EventStream field),
#                         events_ledger_test.go (ledger entry ONLY)
# NEW (evidence): plan/022_856008a8cbb5/P2M2T1S2/research/pi_capture_<date>.log
# FAIL branch: only builtin.go (negative record) + the evidence log. Nothing else, either branch.
```

### Known Gotchas of our codebase & Library Quirks

```text
# GOTCHA 0 (S1 dependency): this task RUNS S1's harness. If events_verify_test.go is not yet on the
#   branch, STOP and report — do not improvise a private probe (the protocol's value is its
#   uniformity across S2–S5). The ledger edit likewise requires S1's file.

# GOTCHA 1 (FR-S3's premise — never assert the exit code): pi documents that a failed response does
#   NOT reliably produce a nonzero exit. The error probe RECORDS the exit code as evidence; an
#   exit-code assertion anywhere in this task encodes the wrong premise and will flake.

# GOTCHA 2 (message_update is delta-ONLY — never accumulate): pi streams text_delta updates during
#   generation; the authoritative message is message_end.message ("Replace the whole partial message
#   with message_end.message when it arrives"). The extractor classifies message_update as
#   EventIgnored and assembles ONLY from the terminal event. (The deltas still count as idle-deadline
#   activity via the read counter — that's FR-S1's portability point — but they are not content.)

# GOTCHA 3 (LAST terminal wins / terminal BEATS error): ParseEventStream keeps the LAST
# EventTerminal and fires FR-S3 only when NO terminal exists. A pi run that produced a (possibly
# empty-but-present) assistant message_end after an error event is a SUCCESS by these rules; make
# the extractor's error branch conservative — classify as EventError only shapes the capture proved
# are hard failures (e.g. auto_retry_end success:false + finalError, or a bare provider error event).

# GOTCHA 4 (STATELESS extractor): registry entries are singletons shared across concurrent roles
# (decompose). No fields, no accumulators, no sync state in piEventExtractor — classify each line
# from its own bytes.

# GOTCHA 5 (the capture is the authority, the doc is design input): pi 1.0.2's live stream is what
#   ships against. If a captured shape contradicts docs/json.md (field names, event order, error
#   shape), the EXTRACTOR follows the capture and the discrepancy goes into the dated record.

# GOTCHA 6 (multi-backend model form): pi's manifest has ProviderFlag --provider; models must be
#   inference/model (FR-R5b) — the harness's realDefaults already encodes {"claude-haiku",
#   "anthropic"}. If you override the model, keep the prefix; a bare model fails Render's check.

# GOTCHA 7 (transient 529/503): pi's providers can return overloaded/capacity errors; pi auto-retries
#   (auto_retry_start/end events — classify Ignored; success:false+finalError is the error signal).
#   A transient failure during the HAPPY-path probe is a re-run, not a FAIL disposition; record it.

# GOTCHA 8 (BuiltinManifests fresh-map-per-call): the harness renders a TRIAL COPY with EventStream
#   set — never write trial results back through BuiltinManifests. Your production edit goes in
#   builtinPi() itself, behind the dated record.

# GOTCHA 9 (the naming trap does not apply to pi — but the check still decides): claude/cursor/agy
#   offer BOTH json and stream-json; pi has ONE json mode and FR-S1 calls it progressive. Expected
#   PASS — but the 500ms-span mechanical check is the proof; do not short-circuit it.

# GOTCHA 10 (ledger is both-directions): TestEventStreamLedger fails a declaration without a dated
#   entry AND an entry without a declaration. Landing EventStream without the ledger edit (or vice
#   versa) is red — land them in the same commit.
```

## Implementation Blueprint

### Data models and structure

No new production data models — the framework's `EventOutcome{Kind, Message, ErrText}` is the
shape. The extractor's internal parse type (test-visible):

```go
// internal/provider/events_pi.go
// pi event-stream extractor — VERIFIED <DATE> via S1's harness
// (`STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/ -run TestEventStreamVerify/pi -v`).
// Shapes per the <DATE> capture (pi 1.0.2, docs/json.md as design input).
type piEventExtractor struct{} // STATELESS (events.go contract)

// piEventLine is the minimal unmarshal target for classification — only the fields the
// extractor decisions need; everything else is ignored by omission.
type piEventLine struct {
	Type    string `json:"type"`
	Message *struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"` // "text" | "thinking" | "tool_call" | …
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
	// Error branches — keep ONLY what the capture proved:
	Success    *bool  `json:"success"`    // auto_retry_end
	FinalError string `json:"finalError"` // auto_retry_end (final failure)
	Error      string `json:"error"`      // provider-level error events, if captured
}
```

(Exact field set adjusts to the capture — Gotcha 5.)

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: PRECONDITION CHECK — S1's harness is on the branch; pi is runnable
  - RUN: ls internal/generate/events_verify_test.go internal/provider/events_ledger_test.go
  - EXPECT: both exist (S1 landed). If not: STOP and report (Gotcha 0).
  - RUN: which pi && pi --version && pi --help | grep -A1 '\-\-mode'
  - EXPECT: /home/dustin/.local/bin/pi, 1.0.2+, and `--mode <mode>` listing json.
    If pi is absent/broken: skip to the FAIL/ABSENT branch of Task 6 (a valid disposition).

Task 1: RUN the dated empirical verification (Phase 1 — no code changes)
  - RUN: STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/ \
         -run 'TestEventStreamVerify/pi' -v -timeout 10m 2>&1 | tee \
         plan/022_856008a8cbb5/P2M2T1S2/research/pi_capture_$(date +%Y-%m-%d).log
  - READ the output against the three proofs: (i) span check; (ii) the verbatim terminal
    message_end/turn_end/agent_end lines; (iii) the invalid-model capture (error line + exit code).
  - EXPECT (per docs + findings): session header first; message_update deltas during generation
    (progressive PASS); a terminal message_end{role:"assistant"}; some error shape on the invalid
    probe with ANY exit code.
  - Transient overload (529/503): re-run (Gotcha 7). Repeated hard failure of the HAPPY probe with
    a valid model: that is an UNAUTHENTICATED/failed-run disposition — record verbatim.

Task 2: DECIDE the disposition (mechanical)
  - PASS iff (i) AND a terminal assistant-message event was captured AND (iii) captured (error
    shape observed OR explicitly 'none observed' with the exit code — the record states which).
  - Anything else → FAIL branch. Record the decision + evidence line numbers from the capture log.

Task 3 [PASS only]: CREATE internal/provider/events_pi.go
  - IMPLEMENT: init() → RegisterEventExtractor("pi", piEventExtractor{}); the classifier per the
    blueprint data model + the CAPTURED shapes (Gotcha 5); dated doc comment (§7.3 pattern).
  - FOLLOW pattern: events.go's Extractor contract (stateless, lenient, no error return).
  - PLACEMENT: internal/provider/events_pi.go (per-provider file; S3–S5 mirror it).

Task 4 [PASS only]: MODIFY internal/provider/builtin.go — builtinPi()
  - ADD: EventStream: []string{"--mode", "json"}, // VERIFIED <DATE> via S1's harness (pi <ver>);
         extractor: events_pi.go. Progressive; terminal = message_end{assistant}; errors in-stream
         (FR-S3 — no reliable nonzero exit).
  - ADD to the ctor doc comment: S1's filled recording template (the three proofs + gap judgment).
  - PRESERVE: every existing field + comment (the :29 header, :54/:61 VERIFIED lines, etc.).

Task 5 [PASS only]: EDIT the ledger + CREATE events_pi_test.go
  - EDIT internal/provider/events_ledger_test.go: eventStreamLedger gains "pi": "<date>".
  - CREATE internal/provider/events_pi_test.go per the What section (verbatim-capture fixtures;
    terminal/error/ignored asserts; composite ParseEventStream test; Lookup non-nil; EventErrorHint).
  - NAMING: test funcs TestPiExtractor_<case>; fixtures consts named from the event types.

Task 6 [both branches]: LAND the record + validate the disposition
  - PASS: re-run Phase B (Task 1's command) — assert the sentinel assembles + EventErrorHint fires.
  - FAIL/ABSENT: paste the dated NEGATIVE record into builtinPi()'s doc comment (command run, pi
    version, the failed proof, verbatim counter-evidence, retry conditions); ledger untouched.
  - providers/pi.toml parity (PASS branch): grep VERIFIED providers/pi.toml — if it carries dated
    field comments, mirror the record + event_stream line; else note 'no convention' in the report.

Task 7: VERIFY — the full gate set (Validation Loop below)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the classifier — lenient, capture-shaped; adjust fields per Gotcha 5):
func (piEventExtractor) Extract(line string) EventOutcome {
	var ev piEventLine
	if err := json.Unmarshal([]byte(line), &ev); err != nil || ev.Type == "" {
		return EventOutcome{Kind: EventIgnored} // non-JSON/blank/garbage — lenient by contract
	}
	switch ev.Type {
	case "message_end":
		if ev.Message != nil && ev.Message.Role == "assistant" {
			var b strings.Builder
			for _, blk := range ev.Message.Content {
				if blk.Type == "text" { b.WriteString(blk.Text) } // thinking/tool_call skipped
			}
			return EventOutcome{Kind: EventTerminal, Message: b.String()}
		}
		return EventOutcome{Kind: EventIgnored} // user/system message_end
	case "auto_retry_end": // hard failure only (Gotcha 3 — conservative error branch)
		if ev.Success != nil && !*ev.Success && ev.FinalError != "" {
			return EventOutcome{Kind: EventError, ErrText: ev.FinalError}
		}
		return EventOutcome{Kind: EventIgnored}
	// "error" and any other capture-proven failure shape: add per the capture; everything
	// else (session/agent_start/turn_*/message_start/message_update/tool_execution_*/
	// agent_end/agent_settled/compaction_*/entry_*/session_info_changed) is EventIgnored.
	default:
		return EventOutcome{Kind: EventIgnored}
	}
}

// PATTERN (the composite-stream test — encodes ParseEventStream's win rules):
//   stream := sessionHeader + "\n" + msgUpdateDelta + "\n" + terminalMsgEnd + "\n" + agentEnd
//   msg, ok, errText := ParseEventStream(stream, piManifestWithEventStream)
//   assert ok && msg == wantText && errText == ""            // LAST terminal; no error fired
//   errStream := sessionHeader + "\n" + autoRetryFail + "\n" // no terminal anywhere
//   msg2, ok2, errText2 := ParseEventStream(errStream, m)
//   assert !ok2 && errText2 == "529 overloaded (final)"      // FR-S3 path
//   assert EventErrorHint(errStream, m) != ""                // the cause line
```

### Integration Points

```yaml
CONSUMES (READ-ONLY):
  - S1's harness: TestEventStreamVerify/pi + env knobs + recording template + ledger mechanism.
  - events.go: Extractor/EventOutcome/RegisterEventExtractor/ParseEventStream/EventErrorHint.
  - realagent plumbing: realDefaults["pi"] (model/inference) via the harness — never re-invented.
  - render.go/parse.go: arming + assembly already wired (NO changes in this task).
PRODUCES (consumed downstream):
  - The first populated eventExtractors entry + the first ledger row — the template S3–S5 copy.
  - pi's portable idle-activity channel (FR-S1/S4) + in-stream error surfacing (FR-S3).
  - The dated record in builtinPi() — Mode A documentation; P3's changeset docs sync reads it.
REGISTRY: exactly ONE new registration (init in events_pi.go). Never in a test file, never conditional.
MANIFEST: exactly ONE field added to builtinPi(). Never to other providers' ctors (S3–S5 own those).
```

## Validation Loop

### Level 1: Build & static (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach
go build ./...
go vet ./internal/provider/...
go vet -tags integration_real ./internal/generate/     # compiles against S1's harness either way
gofmt -l internal/provider/ | sed -n p                # must print nothing
# Expected: all clean, exit 0 (FAIL branch: identical, minus events_pi* files).
```

### Level 2: The empirical + ledger gates (the core)

```bash
cd /home/dustin/projects/stagecoach
# (a) The dated run's evidence exists and is complete:
ls plan/022_856008a8cbb5/P2M2T1S2/research/pi_capture_*.log && \
  grep -c 'message_end\|progressive\|exit' plan/022_856008a8cbb5/P2M2T1S2/research/pi_capture_*.log
# Expected: the log exists with the three proofs' evidence.

# (b) The ledger invariant holds in the LANDED branch:
go test ./internal/provider/ -run TestEventStreamLedger -v
# Expected: PASS — PASS branch: "pi" declared AND "pi": "<date>" recorded; FAIL branch: neither.

# (c) [PASS branch] The extractor fixtures + framework integration:
go test ./internal/provider/ -run 'TestPiExtractor|TestEventStreamLedger|ParseEventStream' -v
# Expected: all green (terminal/error/ignored classification + composite win rules + hint).

# (d) [PASS branch] Phase B — the real CLI through the REAL parse path (red→green closed):
STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/ \
  -run 'TestEventStreamVerify/pi' -v -timeout 10m
# Expected: PASS — sentinel "stagecoach-event-verify" assembled; EventErrorHint fires on the error
#           capture; progressiveness re-proven with the production extractor registered.
```

### Level 3: No regressions (existing suites)

```bash
cd /home/dustin/projects/stagecoach
go test ./internal/provider/... ./internal/generate/... -race
go test ./... -race
# Expected: GREEN — the extractor adds classification only; render/parse/registry suites untouched;
#           the arming guard tests still pass (pi now HAS an extractor — any no-extractor-guard
#           test fixtures must not be pi-specific; if one is, that's a finding to report, not patch).
```

### Level 4: FR-S2 discipline + scope guard

```bash
cd /home/dustin/projects/stagecoach
# (1) Exactly the sanctioned diff:
git status --porcelain
# Expected (PASS): ?? internal/provider/events_pi.go ?? internal/provider/events_pi_test.go
#                  M internal/provider/builtin.go M internal/provider/events_ledger_test.go
#                  ?? plan/.../research/pi_capture_<date>.log  (+ providers/pi.toml if parity)
# Expected (FAIL): M internal/provider/builtin.go + the evidence log. NOTHING else.
# (2) Only pi's ctor changed; only one registration site:
git diff internal/provider/builtin.go | grep -E '^\+' | grep -vE 'builtinPi|EventStream|VERIFIED|^\+\s*//|^\+\s*$' | grep -E '.' && echo "CHECK: unexpected builtin.go additions" || echo "OK"
rg -n 'RegisterEventExtractor\(' internal/ | grep -v _test
# Expected: events.go (definition) + events_pi.go (the ONE production call). Harness/test calls excluded.
# (3) The record is dated + probe-named (FR-S2):
rg -n 'VERIFIED.*20[0-9]{2}-[0-9]{2}-[0-9]{2}' internal/provider/builtin.go | grep -i 'event'
# Expected: the pi event-stream record present (positive or negative), naming the harness run.
# (4) No other provider touched:
git diff --name-only | grep -E 'builtin|events' | grep -vE 'builtin.go|events_pi|events_ledger|events.go$' && echo "FAIL: out of scope" || echo "OK"
# Note: events.go itself must be UNTOUCHED (git diff internal/provider/events.go → empty).
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1 clean (build/vet/gofmt; tagged vet)
- [ ] Level 2a: capture log exists with all three proofs' evidence
- [ ] Level 2b: `TestEventStreamLedger` GREEN in the landed branch
- [ ] Level 2c/2d [PASS branch]: extractor tests + Phase B re-run green (sentinel + hint)
- [ ] Level 3: `-race` suites green
- [ ] Level 4: scope guard clean; events.go untouched; one registration site

### Feature (Disposition) Validation
- [ ] Disposition decided by the mechanical criteria; recorded with date + pi version + command
- [ ] PASS branch: `EventStream: ["--mode","json"]` landed behind the dated record; extractor
      stateless + capture-shaped; ledger entry; verbatim-fixture tests; providers/pi.toml parity
      handled (mirrored or noted)
- [ ] FAIL branch: dated negative record with the counter-evidence; nothing populated
- [ ] FR-S3 honored: exit code recorded as evidence, never asserted; error branch conservative
      (terminal beats error; LAST terminal wins)

### Code Quality Validation
- [ ] Follows events.go's Extractor contract (lenient, stateless, no error return)
- [ ] Test fixtures are the VERBATIM captured lines (dated), not hand-synthesized idealizations
- [ ] Existing builtinPi comments preserved; the record extends the header
- [ ] S1's files otherwise untouched (ledger map-literal edit only); realagent untouched

### Documentation & Deployment
- [ ] Mode A satisfied: the dated record in builtinPi()'s doc comment IS the documentation
- [ ] The evidence log committed under the task's research/ directory
- [ ] Implementation report states the disposition + the one-line summary for the S3–S5 template

---

## Anti-Patterns to Avoid

- ❌ Don't ship `--mode json` (or an extractor) without the dated passing run — FR-S2 forbids
  speculation; the ledger test makes it red. Equally: don't leave a FAIL unrecorded — [] with the
  dated reason IS the deliverable.
- ❌ Don't assert pi's exit code (nonzero or zero) anywhere — FR-S3's premise; the code is evidence.
- ❌ Don't accumulate `message_update` deltas — delta-only wire shape; `message_end.message` is the
  authoritative terminal content (Gotcha 2).
- ❌ Don't make the extractor stateful or give it an error return — the registry shares singletons
  across concurrent roles; unknown lines are EventIgnored (Gotcha 4).
- ❌ Don't classify error shapes the capture didn't prove — a conservative error branch under
  terminal-beats-error semantics; speculation here misfires FR-S3 (Gotcha 3).
- ❌ Don't treat the pi docs as the authority over the capture — 1.0.2 on THIS machine is what
  ships; record discrepancies (Gotcha 5).
- ❌ Don't improvise a private probe if S1's harness is missing — report and stop (Gotcha 0).
- ❌ Don't touch events.go/parse.go/render.go — the framework is landed; this task only populates.
- ❌ Don't add a bare model override for pi — multi-backend; `inference/model` form only (Gotcha 6).
- ❌ Don't treat a transient 529/503 as a FAIL disposition — re-run; record it (Gotcha 7).
- ❌ Don't land EventStream and the ledger entry in separate commits — the both-directions guard
  goes red between them (Gotcha 10).

---

## Confidence Score

**One-pass success likelihood: 9/10.** The framework, harness contract, pi's documented event
shapes, the installed-CLI ground truth (pi 1.0.2 with `--mode json`), and the landed API signatures
are all verified against the live repo, and both disposition branches are fully specified with
mechanical criteria. The 1-point deduction is inherent to the task's nature: the terminal/error
shapes the extractor encodes are decided by a runtime capture that cannot be pre-read here — the
PRP mitigates with doc-informed classification guidance, capture-authority rules, and lenient
fallbacks, plus the S1-harness red→green loop that mechanically proves the result before landing.
