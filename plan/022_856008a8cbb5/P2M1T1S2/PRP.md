name: "P2.M1.T1.S2 — event-mode execution: NDJSON parse, per-provider terminal-event extractor, FR-S3 error surfacing"
description: |
  The execution half of the event-stream output mode (§9.30 FR-S1's runtime side + FR-S3; P2.M1.T1's S2).
  Builds on S1's landed contract (manifest.EventStream field, render insertion + CmdSpec.ProgressiveStdout
  arming, the Extractor registry in internal/provider/events.go — EMPTY per FR-S2). S2 delivers: (1) the
  Extractor interface REFINED from S1's placeholder to carry the error-event signal (EventKind/EventOutcome —
  S1's PRP explicitly anticipates this refinement; S1's render_test fakeExtractor is updated); (2)
  ParseEventStream — the NDJSON-ish framework scan (line-split, CRLF-tolerant, blank-skip, per-line
  classification; LAST terminal wins, FIRST error recorded, terminal-wins-over-error); (3) a step-0 event
  branch inside ParseOutput (assembled message flows through the existing fence/normalize/trim steps; print
  mode byte-identical — contract (c)); (4) UnwrapEventOutput — a one-line helper for the two parse sites that
  DON'T use ParseOutput (planner.go's prompt.ParsePlannerOutput + the arbiter's parse); (5) EventErrorHint —
  the FR-S3 "precedence sibling" of StderrHint (executor.go:23–38), consulted BEFORE StderrHint at the 3
  existing cause sites (generate.go:403, decompose/message.go:204, decompose/planner.go:143) so an in-stream
  error/abort event (which does NOT reliably produce a nonzero exit — pi documents this) replaces the opaque
  empty-output failure; (6) e2e tests via the stubtest progressive-emit stub (JSONL ticks + terminal event →
  assembled message AND survives a 1s idle on every platform — FR-A3's portable payoff; error event exit-0 →
  cause surfaced); (7) Mode A docs: the `### Event-stream capability (event_stream)` section in
  docs/providers.md after the session_mode precedent — FR-S2 verification bar, json-vs-stream-json trap,
  heartbeat/sparse pitfalls. NO real CLIs (empirical verification is P2.M2's duty); NO built-in declares
  event_stream; registry stays empty. Consumed by P2.M2.T1.S1 (verification harness) and P2.M2.T1.S2–S5
  (extractor registration + population).

---

## Goal

**Feature Goal**: When a manifest declares `event_stream` (spec.ProgressiveStdout armed by S1's render),
stagecoach executes the agent in event mode and (a) assembles the final message from the TERMINAL message
event via a per-provider Extractor, (b) surfaces an in-stream error/abort event's text through the existing
empty-output cause path (FR-S3) — keying on the event stream, NOT the exit code — and (c) leaves the
print-mode parse path completely untouched.

**Deliverable**: Edits to `internal/provider/events.go` (refined Extractor + ParseEventStream +
UnwrapEventOutput + EventErrorHint), `internal/provider/parse.go` (step-0 branch + header), 3 call-site
precedence insertions (generate.go, decompose/message.go, decompose/planner.go) + 2 unwrap insertions
(planner.go, the arbiter site), NEW `internal/provider/events_test.go` + parse_test additions, NEW
`internal/generate/generate_eventstream_test.go` (e2e via stubtest), S1's render_test.go fakeExtractor
update, and the docs/providers.md capability section.

**Success Definition**: `go build ./...` + `go vet ./...` + gofmt clean; `go test ./internal/provider/...
./internal/generate/... ./internal/decompose/... -race` GREEN (every existing print-mode test byte-identical
in behavior); the e2e JSONL-stub test proves (i) progressive events + terminal event → assembled message,
(ii) survival under a 1s idle with 2.5s of 50ms ticks on EVERY platform (the portable pipe-activity payoff),
(iii) an exit-0 error event → RescueError.Cause carries the event text; `rg -n 'event_stream' providers/*.toml`
zero hits (registry + built-ins stay empty — FR-S2; P2.M2 populates).

## User Persona (if applicable)

**Target User**: Today, no end user (no built-in declares event_stream; the registry is empty — FR-S2).
The immediate consumers are the P2.M2 verification/population tasks and, once populated, any user of a
verified provider — who gets portable idle detection (macOS/Windows, no procfs) and real error messages
("usage limit exceeded") instead of opaque empty-output failures.
**Use Case**: P2.M2 registers a verified extractor (e.g. claude's stream-json `result` event) + declares
`event_stream = ["--output-format", "stream-json", "--include-partial-messages"]`; from that moment every
claude generation assembles its message from the terminal event, keeps the idle clock alive via progressive
events on every platform, and surfaces in-stream errors through the rescue cause line.
**Pain Points Addressed**: (1) off-Linux idle detection for buffered print CLIs (FR-S4's portability gap);
(2) the opaque "no valid commit message" failure when a provider errors in-stream with exit 0 (FR-S3).

## Why

- **FR-S1's runtime half**: S1 landed the declarative surface (field + render + arming + registry type);
  S2 is the execution path that makes the capability real. Without S2, a populated manifest would emit
  NDJSON that ParseOutput cannot read.
- **FR-S3 correctness**: a failed response does NOT reliably produce a nonzero exit (pi documents this
  explicitly) — the executor's ExitError contract can't be the signal. Detection must key on the event
  stream. StderrHint (executor.go:23–38) returns "" whenever stdout is non-empty — in event mode stdout
  always has events, so today's path structurally CANNOT surface in-stream errors. The precedence sibling
  closes exactly this hole through the SAME cause line.
- **FR-A3's portable payoff**: ProgressiveStdout (armed by S1) + the countingWriter pipe channel = idle
  detection on every platform, but only if the provider actually emits progressively — event mode is what
  makes that true. S2's e2e test is the first cross-platform proof.
- **FR-S2 discipline preserved**: S2 ships ZERO real extractors and ZERO built-in declarations — the
  framework only. P2.M2 populates per-provider after dated empirical verification.

## What

The event-mode execution path: NDJSON-ish scan → per-provider Extractor classification → terminal-event
message assembly (ParseOutput step-0 + UnwrapEventOutput for non-ParseOutput sites) → FR-S3 in-stream error
surfacing (EventErrorHint before StderrHint at the 3 cause sites). Print mode byte-identical.

### Success Criteria
- [ ] events.go: `EventKind` (EventIgnored/EventTerminal/EventError) + `EventOutcome{Kind, Message, ErrText}`
      + refined `Extractor interface { Extract(line string) EventOutcome }` (replaces S1's 3-return
      placeholder; S1's render_test.go fakeExtractor updated to the new signature).
- [ ] events.go: `ParseEventStream(raw string, m Manifest) (msg string, ok bool, errText string)` —
      line-split (CRLF-tolerant), skip blanks, classify via `LookupEventExtractor(m.Name)`; LAST terminal
      wins; FIRST error recorded; terminal-wins-over-error; no-extractor ⇒ ("", false, "") (defensive,
      never panic — Render's guard makes it unreachable).
- [ ] parse.go: step-0 event branch — after Resolve+trim, `if len(r.EventStream) > 0 { s = <assembled> }`;
      the assembled message then flows through the EXISTING steps 2/4/5; fellback stays false; header
      comment documents the branch; print mode (len==0) executes the byte-identical existing code.
- [ ] events.go: `UnwrapEventOutput(out string, m Manifest) string` — event mode ⇒ assembled terminal
      message; else out unchanged. Inserted at the two non-ParseOutput parse sites: planner.go (before
      `prompt.ParsePlannerOutput(out)`) and the arbiter's parse site (locate via grep at impl).
- [ ] events.go: `EventErrorHint(raw string, m Manifest) string` — "" unless (event-mode AND no terminal
      message AND an error event exists); else single-line, newline-collapsed, 1000-char-capped
      "agent event stream reported an error: <text>" (mirrors StderrHint's formatting).
- [ ] The 3 cause sites insert the precedence sibling BEFORE StderrHint:
      `if h := provider.EventErrorHint(out, <manifest>); h != "" { lastCause = errors.New(h) } else if
      h := provider.StderrHint(out, stderr); h != "" { lastCause = errors.New(h) }` —
      generate.go (~:403, deps.Manifest), decompose/message.go (~:204, deps.Roles.Message),
      decompose/planner.go (~:143, deps.Roles.Planner).
- [ ] e2e (internal/generate, via stubtest — provider tests can't import stubtest: cycle): progressive
      JSONL stub (EmitMS=50, EmitLine=`{"type":"progress","n":1}`, EmitTotalMS=2500) + terminal event
      Out=`{"type":"result","message":"feat: add widget"}` + a registered fake extractor + Deadlines{Idle:1s,
      Hard:30s} ⇒ Execute err==nil (portable idle survival), ParseOutput assembles "feat: add widget".
- [ ] e2e: error event Out=`{"type":"error","message":"usage limit exceeded"}` with stub EXIT 0 ⇒ ParseOutput
      ok==false (existing FR29 retry), and the run's RescueError.Cause carries the event text.
- [ ] Unit tests: multi-event streams, CRLF, blank + non-JSON junk lines (ignored), last-terminal-wins,
      terminal-beats-error, error-cap formatting, no-extractor defensiveness, print-mode regression
      (every existing parse_test untouched and green).
- [ ] docs/providers.md: `### Event-stream capability (event_stream)` section after the session_mode
      precedent section (~L40–49, before `## Command rendering`) — FR-S2 verification bar, json-vs-stream-json
      trap, heartbeat/sparse pitfalls verbatim from §9.30.
- [ ] `rg -n 'event_stream' providers/*.toml` ⇒ zero hits; the eventExtractors map ships EMPTY (FR-S2).

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the S1 contract (what exists: field/arming/registry shape), the exact Extractor refinement + its
anticipated-churn note, the full scan semantics (win rules, leniency, defensiveness), the exact insertion
points with verified live line numbers, the StderrHint mechanism + why it structurally misses event-mode
errors, the pi exit-0 premise, the stubtest/stubagent e2e recipe (env knobs, cadence vs the 500ms watcher
tick, the import-cycle constraint on test placement), and the docs precedent.

### Documentation & References

```yaml
# MUST READ — S2's own findings (verified live; the design rationale).
- docfile: plan/022_856008a8cbb5/P2M1T1S2/research/findings.md
  why: "§0 the S1 contract (incl. the explicit placeholder-refinement clause); §1 FR-S1–S4 verbatim essentials
        (naming trap, verification bar, pitfalls); §2 the parse/execute landscape (7 ParseOutput sites, 3
        StderrHint sites, planner's ParsePlannerOutput, StderrHint's empty-on-nonempty-stdout hole); §3 the
        six locked design decisions + why; §4 test infra (stubtest cycle, stub knobs, 500ms tick); §5 the
        e2e recipe; §6 the docs section."

# MUST READ — the S1 contract (treat as landed: field, render arming, registry).
- docfile: plan/022_856008a8cbb5/P2M1T1S1/PRP.md
  why: "S1's deliverables are S2's INPUT: EventStream field + regime-2 merge, render insertion + spec.
        ProgressiveStdout arming (both render paths), the Extractor registry (Lookup/Register, empty map),
        the render no-extractor hard guard, the render_test fakeExtractor S2 must update. S1's PRP states
        the interface is a placeholder for S2's semantics."

# MUST READ — the FR-S spec (§9.30).
- docfile: plan/022_856008a8cbb5/prd_snapshot.md
  section: "§9.30 (line ~659): FR-S1 (event modes + naming trap), FR-S2 (verification duty + heartbeat/sparse
            pitfalls — the docs section quotes these), FR-S3 (error surfacing + the exit-0 premise), FR-S4
            (phasing), FR-A3 (the activity channels S2's e2e proves portable)."

# MUST READ — the files under edit.
- file: internal/provider/events.go   # S1's NEW file — S2 EXTENDS it (refine Extractor; add ParseEventStream,
  #                                     UnwrapEventOutput, EventErrorHint, EventKind/EventOutcome).
  why: "Owns the registry S2 consumes. Keep LookupEventExtractor/RegisterEventExtractor signatures UNCHANGED
        (the stable contract); only the Extractor interface method shape changes."
- file: internal/provider/parse.go
  why: "ParseOutput (5-step §12.9 pipeline). Insert the step-0 branch after `r := m.Resolve()` + the trim,
        BEFORE the fence-strip; update the header comment. Print mode must stay byte-identical (len==0 skips
        the branch entirely). Do NOT touch parseJSON/extractJSONObject/stripCodeFence/normalizeNewlines."
- file: internal/provider/executor.go
  why: "READ-ONLY: Execute (:104) returns (stdout, stderr, err); StderrHint (:23–38) — the cause mechanism S2
        feeds and whose empty-on-nonempty-stdout behavior is WHY EventErrorHint must precede it; the D4 arming
        (:144). NO edits (P1.M2's landed surface)."
- file: internal/generate/generate.go        # the cause site ~:403 (deps.Manifest) + the loop it sits in
- file: internal/decompose/message.go        # the cause site ~:204 (deps.Roles.Message)
- file: internal/decompose/planner.go        # the cause site ~:143 (deps.Roles.Planner) AND the unwrap
  #                                           insertion before prompt.ParsePlannerOutput(out) (~:139)
  why: "The 3 EXISTING StderrHint cause sites (the contract's 'existing empty-output cause path') + the one
        verified non-ParseOutput parse site. Insert the precedence sibling + the unwrap exactly as specified."
- file: internal/provider/render_test.go     # S1's fakeExtractor — update to the refined interface

# MUST READ — the test infrastructure.
- file: internal/stubtest/stubtest.go
  why: "Build(t) (CWD-independent stubagent compile), Options{Out, Exit, EmitMS, EmitLine, EmitTotalMS, ...}
        → STAGECOACH_STUB_* env, Manifest/Env helpers. CRITICAL: stubtest imports internal/provider ⇒
        same-package provider tests CANNOT import it (cycle) — e2e tests live in internal/generate."
- file: cmd/stubagent/main.go
  why: "The 1d progressive-emit phase: EMIT_MS writes EMIT_LINE every interval for EMIT_TOTAL_MS (0 ⇒ forever,
        never answer; >0 ⇒ bounded, then the answer = stdout SUFFIX via STAGECOACH_STUB_OUT). Default exit 0 —
        exactly FR-S3's exit-0 premise. Tick ≪ the watcher's 500ms poll (use 50ms)."
- file: internal/generate/idle_parity_test.go  # the e2e precedent (stubtest + deadline wiring + cause asserts)

# CONTEXT — the docs precedent + Sibling tasks.
- file: docs/providers.md
  why: "The manifest table's event_stream row is LANDED (S1). The `### Multi-turn capability (session_mode)`
        section (~L40–49, after the table, before ## Command rendering) is the shape to mirror for the NEW
        `### Event-stream capability (event_stream)` section."
- docfile: plan/022_856008a8cbb5/architecture/provider_subsystem.md
  section: "§6 (parse.go pipeline + the §6.3 REALITY FLAG: StderrHint lives in executor.go, not parse.go;
            the DEBUG: cause line is emitted in internal/cmd/default_action.go:381)."
```

### Current Codebase tree (relevant slice)

```bash
# EDIT:
internal/provider/events.go                      # EXTEND (S1's file): EventKind/EventOutcome + refined Extractor
                                                 #   + ParseEventStream + UnwrapEventOutput + EventErrorHint
internal/provider/parse.go                       # step-0 event branch + header comment
internal/provider/events_test.go                 # NEW — unit tests (same package; pure functions, no stub)
internal/provider/parse_test.go                  # + event-mode ParseOutput tests (print-mode tests untouched)
internal/provider/render_test.go                 # update S1's fakeExtractor to the refined interface
internal/generate/generate_eventstream_test.go   # NEW — e2e (stubtest; happy+idle-survival, error-cause, guard)
internal/generate/generate.go                    # precedence sibling at the ~:403 cause site
internal/decompose/message.go                    # precedence sibling at the ~:204 cause site
internal/decompose/planner.go                    # precedence sibling at ~:143 + UnwrapEventOutput before ParsePlannerOutput (~:139)
internal/decompose/arbiter*.go                   # UnwrapEventOutput at the arbiter's parse site (locate by grep)
docs/providers.md                                # + the Event-stream capability section
# READ-ONLY:
internal/provider/executor.go                    # Execute + StderrHint + arming (P1.M2 landed; FR-S3 feeds it)
internal/provider/manifest.go  render.go  merge.go  builtin.go   # S1's landed field/render/merge (no built-in declares)
internal/stubtest/  cmd/stubagent/               # test infra (no changes needed)
providers/*.toml                                 # stays event_stream-free (FR-S2)
```

### Desired Codebase tree with files to be added/edited

```bash
internal/provider/events.go        # EventKind/EventOutcome + Extractor{Extract(line) EventOutcome}
                                   # + ParseEventStream(raw, m) (msg, ok, errText)
                                   # + UnwrapEventOutput(out, m) string
                                   # + EventErrorHint(raw, m) string     (registry map stays EMPTY)
internal/provider/events_test.go   # NEW: scan semantics, leniency, win rules, hint formatting, defensiveness
internal/generate/generate_eventstream_test.go  # NEW: e2e happy+idle-survival / error-cause / no-extractor
# + the 5 call-site insertions (3 precedence siblings, 2 unwraps) + parse.go step-0 + docs section
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (print mode must be BYTE-IDENTICAL — contract (c)): the step-0 branch is `if len(r.EventStream) > 0`.
// Every existing manifest ships nil ⇒ the branch never fires ⇒ the 5-step pipeline runs the exact existing
// code. Do NOT restructure the existing steps; insert, don't rewrite. Every existing parse_test must pass
// UNCHANGED.

// CRITICAL (StderrHint structurally misses event-mode errors): StderrHint(stdout, stderr) returns "" when
// stdout has non-whitespace content — in event mode stdout ALWAYS has events, so it can never surface the
// in-stream error. EventErrorHint(raw, m) is a PRECEDENCE SIBLING (checked FIRST at the cause sites), not a
// StderrHint edit. Do NOT touch StderrHint itself (print mode depends on it).

// CRITICAL (the exit-0 premise): a failed/aborted response does NOT reliably produce a nonzero exit (pi
// documents this). The e2e error test MUST use the stub's default exit 0 — if you set STAGECOACH_STUB_EXIT=1
// you're testing the ExitError path, not FR-S3.

// CRITICAL (import cycle): internal/stubtest imports internal/provider. A same-package provider test
// (package provider) CANNOT import stubtest. e2e tests go in internal/generate (package generate, external).
// Unit tests for ParseEventStream/EventErrorHint are pure functions — same package, no stub needed.

// CRITICAL (win rules): LAST terminal event wins; FIRST error event recorded; a terminal message BEATS an
// error event (a produced message is a success — FR-S3 fires only when NO terminal message event). Unknown,
// unrecognized, or unparseable lines classify as EventIgnored (lenient — the stream carries many event types
// stagecoach doesn't care about; a malformed line must not kill the parse).

// GOTCHA (the registry holds SINGLETONS shared by concurrent roles — decompose runs roles concurrently):
// the Extractor must be STATELESS per line (the terminal event is self-contained; FR-S2 records "the
// terminal-event shape the message is assembled from"). A stateful accumulator would be a data race. If
// P2.M2's verification later finds a delta-accumulation shape, grow the interface THEN, at the registry seam.

// GOTCHA (watcher cadence): the idle watcher polls every 500ms (executor.go watchIdle). Test tick intervals
// must be well below it (50ms) so several ticks land between polls; idle windows must exceed several ticks.

// GOTCHA (the assembled message still flows through cleanup): steps 2/4/5 (fence-strip, newline-normalize,
// trim) apply to the ASSEMBLED message — a provider whose terminal event carries a fenced message gets the
// same strip behavior as print mode. fellback is ALWAYS false in event mode.

// GOTCHA (planner + arbiter don't use ParseOutput): planner.go:139 parses via prompt.ParsePlannerOutput(out);
// the arbiter similarly (only message.go in decompose calls ParseOutput — grep at impl to locate the arbiter
// site). These sites need the explicit `out = provider.UnwrapEventOutput(out, <manifest>)` line BEFORE their
// parse; the 7 ParseOutput sites get event mode for free via step-0.

// GOTCHA (S1's fakeExtractor churn): render_test.go's fake implements S1's 3-return placeholder. Update it to
// the refined `Extract(line) EventOutcome` signature — S1's PRP explicitly anticipated this refinement.
```

## Implementation Blueprint

### Data models and structure

```go
// internal/provider/events.go — S2's additions (S1's registry Lookup/Register stay UNCHANGED):

// EventKind classifies one event-stream line.
type EventKind int

const (
	EventIgnored  EventKind = iota // progress/heartbeat/tool/unknown — not a message or error event
	EventTerminal                  // the terminal message event — carries the assembled final message
	EventError                     // an in-stream error/abort event (FR-S3)
)

// EventOutcome is one line's classification. Message is the assembled final message (EventTerminal);
// ErrText is the error event's text (EventError). Extractors are STATELESS (registry singletons are shared
// by concurrent roles); the terminal event is self-contained (FR-S2's recorded shape).
type EventOutcome struct {
	Kind    EventKind
	Message string
	ErrText string
}

// Extractor is refined from S1's placeholder (S2 owns the semantics; S1 anticipated the refinement):
// classify one raw NDJSON-ish stdout line. Unknown/unparseable lines are EventIgnored — classification
// never errors (lenient, mirroring ParseOutput's fallback philosophy; each extractor unmarshals its own
// provider shape and ignores what doesn't fit).
type Extractor interface {
	Extract(line string) EventOutcome
}

// ParseEventStream scans event-mode stdout: split lines (CRLF-tolerant), skip blanks, classify each via
// LookupEventExtractor(m.Name). LAST terminal wins; FIRST error recorded; a terminal message beats an error
// (FR-S3 fires only with NO terminal message event). No registered extractor ⇒ ("", false, "") — defensive,
// never a panic (Render's guard makes it unreachable). errText feeds EventErrorHint (FR-S3).
func ParseEventStream(raw string, m Manifest) (msg string, ok bool, errText string)

// UnwrapEventOutput returns the assembled terminal message in event mode, or out unchanged otherwise —
// the one-line integration for parse sites that DON'T use ParseOutput (planner, arbiter).
func UnwrapEventOutput(out string, m Manifest) string

// EventErrorHint is the FR-S3 precedence sibling of StderrHint (executor.go): "" unless the manifest is in
// event mode AND no terminal message was produced AND an error event exists; else a single-line,
// newline-collapsed, 1000-char-capped "agent event stream reported an error: <text>" (mirrors StderrHint's
// formatting). Consulted BEFORE StderrHint at the cause sites — the in-stream error is the specific cause.
func EventErrorHint(raw string, m Manifest) string
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/provider/events.go — the refined Extractor + the three framework functions
  - Replace S1's placeholder interface with EventKind/EventOutcome/Extractor per the blueprint.
    LookupEventExtractor/RegisterEventExtractor signatures UNCHANGED; the map stays EMPTY.
  - ParseEventStream: strings.Split on "\n" (trim each line's "\r"; skip blanks/whitespace-only);
    classify; track lastTerminal + firstErr; terminal-beats-error. Guard the no-extractor case first.
  - UnwrapEventOutput: len(EventStream)>0 ⇒ ParseEventStream's msg (empty if none); else raw.
    Use m.Resolve() for nil-safe EventStream access (mirror ParseOutput's Resolve-first idiom).
  - EventErrorHint: "" fast-path when not event mode; else ParseEventStream; if ok || errText=="" ⇒ "";
    else collapse newlines + cap at 1000 chars (copy StderrHint's exact formatting idiom) + the prefix.
  - VALIDATE: go build ./internal/provider/ && go vet ./internal/provider/

Task 2: EDIT internal/provider/render_test.go — update S1's fakeExtractor
  - The fake now returns EventOutcome (e.g. {Kind: EventTerminal, Message: "fake"}); delete the old
    3-return body. All S1 render tests must still pass (registry + guard behavior unchanged).
  - VALIDATE: go test ./internal/provider/ -run 'EventStream|Render' -v

Task 3: EDIT internal/provider/parse.go — the step-0 event branch
  - After `r := m.Resolve()` and the step-1 trim, before step 2 (fence-strip):
    `if len(r.EventStream) > 0 { if a, okA, _ := ParseEventStream(s, m); okA { s = a } }` — when no
    terminal event, s becomes "" and step 5's ok==false drives the existing FR29 retry (contract: "empty
    message handled by the existing parse-retry path"). fellback stays false (event mode never "falls back").
    Update the header comment: document step-0 (event mode assembles from the terminal event; steps 2/4/5
    then apply to the assembled message; §9.30 FR-S1).
  - Do NOT touch parseJSON/extractJSONObject/stripCodeFence/normalizeNewlines.
  - VALIDATE: go test ./internal/provider/ -run ParseOutput -v   (all existing tests green — print mode untouched)

Task 4: EDIT the 3 cause sites — the FR-S3 precedence sibling
  - generate.go (~:403, in the !ok block; manifest = deps.Manifest):
      if h := provider.EventErrorHint(out, deps.Manifest); h != "" {
          lastCause = errors.New(h)
      } else if h := provider.StderrHint(out, stderr); h != "" {
          lastCause = errors.New(h)
      }
  - decompose/message.go (~:204, manifest = deps.Roles.Message): identical shape.
  - decompose/planner.go (~:143, in the ParsePlannerOutput-failure block; manifest = deps.Roles.Planner):
    the same if/else-if over lastErr (that site assigns lastErr, not lastCause — read it and mirror).
  - VALIDATE: go build ./... && go vet ./internal/generate/... ./internal/decompose/...

Task 5: EDIT the 2 unwrap sites — planner + arbiter
  - planner.go (~:139): `out = provider.UnwrapEventOutput(out, deps.Roles.Planner)` immediately BEFORE
    `parsed, perr := prompt.ParsePlannerOutput(out)`.
  - Arbiter: rg -n 'ParseArbiterOutput|target' internal/decompose/*.go to locate its parse of raw stdout;
    insert the same unwrap with the arbiter's manifest (deps.Roles.Arbiter). If the arbiter shares
    message.go's path, it may already be covered by ParseOutput — verify before editing (don't double-unwrap).
  - VALIDATE: go build ./... && go vet ./internal/decompose/...

Task 6: CREATE internal/provider/events_test.go — unit tests (same package, pure functions)
  - A table-driven fakeExtractor keyed on a test-only name; RegisterEventExtractor + t.Cleanup delete
    (S1's seam idiom). Cover: multi-event stream (progress lines ignored) → terminal assembled; LAST
    terminal wins across multiple; terminal-beats-error; error-only stream → ("", false, text); CRLF +
    blank + non-JSON junk lines ignored; no-extractor defensiveness ("", false, ""); EventErrorHint
    formatting (prefix, single-line, 1000 cap) + "" fast-paths (not event mode; terminal present);
    UnwrapEventOutput passthrough in print mode. ParseOutput event-mode: fence inside a terminal message
    is stripped; empty stream → ok==false; fellback always false.
  - VALIDATE: go test ./internal/provider/ -run 'Event' -v -race

Task 7: CREATE internal/generate/generate_eventstream_test.go — the e2e rides (package generate, stubtest)
  - Shared helper: build the stub manifest (stubtest.Manifest + set EventStream=["--mode","json"] — the
    tokens are opaque to the executor; what matters is the field is non-empty so Render arms
    ProgressiveStdout and ParseOutput takes the event branch) + RegisterEventExtractor(t, name, fake)
    + t.Cleanup delete. The fake: {"type":"result","message":...} → EventTerminal; {"type":"error",
    "message":...} → EventError; else EventIgnored.
  - TEST 1 (happy + portable idle survival — FR-A3's payoff): EmitMS=50, EmitLine=`{"type":"progress","n":1}`,
    EmitTotalMS=2500, Out=`{"type":"result","message":"feat: add widget"}`; Render → assert
    spec.ProgressiveStdout==true AND the event tokens precede print_flag in spec.Args; Execute(ctx, spec,
    Deadlines{Idle: time.Second, Hard: 30*time.Second}, verbose) → err==nil (the run spans ~2.5s > the 1s
    idle — it survives ONLY because each 50ms tick stamps the pipe channel; on macOS/Windows this works
    solely because ProgressiveStdout armed it); ParseOutput(out, m) → ("feat: add widget", true, false).
  - TEST 2 (FR-S3, exit 0): Out=`{"type":"error","message":"usage limit exceeded"}`, NO Emit, Exit=0
    (stub default) → ParseOutput ok==false; EventErrorHint(out, m) contains "usage limit exceeded";
    StderrHint(out, stderr)=="" (proving the sibling is the only path that could surface it). Then run the
    CommitStaged-level loop (or the minimal harness the idle_parity tests use) with the same stub and
    assert the resulting RescueError.Cause carries the event text.
  - TEST 3 (guard regression): registry empty for the name → Render errors (S1's guard) — re-assert S1's
    behavior from THIS package's perspective (one line), proving S2 didn't weaken it.
  - VALIDATE: go test ./internal/generate/ -run EventStream -v -race   (runs on EVERY platform)

Task 8: EDIT docs/providers.md — the Mode A capability section
  - Insert `### Event-stream capability (\`event_stream\`)` AFTER the Multi-turn capability (session_mode)
    section (~L40–49) and BEFORE `## Command rendering`, mirroring its shape. Content:
    (1) what the field declares (verified JSON-event flags; the executor arms the progressive-stdout idle
    channel; the message is assembled from the terminal event, not raw stdout);
    (2) the FR-S2 verification bar: a dated, recorded empirical run proving (i) progressive emission DURING
    generation, (ii) the terminal-event shape, (iii) how in-stream errors surface — "MUST NOT be set
    speculatively";
    (3) the naming trap: claude/cursor/agy offer BOTH `json` (single buffered object) and `stream-json`
    (progressive) — only `stream-json` qualifies; pi `--mode json`, codex `--json`, opencode `--format json`
    are the progressive ones;
    (4) the pitfalls VERBATIM from §9.30/FR-S2: heartbeat-only streams (codex: steady timer events continue
    while the network is dead — liveness, not progress) and sparse emitters (opencode `--format json`: one
    mid-run event + an answer burst, ~3s stdout gaps on a trivial prompt — fine for a 60s idle clock, fatal
    for a naive sub-second one);
    (5) status line: no built-in ships it today; requires a registered extractor (render errors otherwise).
  - VALIDATE: markdown renders (eyeball); `rg -n 'Event-stream capability' docs/providers.md` (1 hit)

Task 9: VERIFY — the full gate set (see Validation Loop)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the step-0 branch — insert, don't rewrite):
func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool) {
	r := m.Resolve()
	s := strings.TrimSpace(raw)
	// Step 0 (§9.30 FR-S1, event mode): assemble the message from the TERMINAL event. Steps 2/4/5 then
	// apply to the assembled text (fence-strip, normalize, trim). No terminal event ⇒ s="" ⇒ step 5's
	// ok==false drives the existing FR29 retry. Print mode (len==0) is byte-identical to v3.4.
	if len(r.EventStream) > 0 {
		if a, okA, _ := ParseEventStream(s, m); okA {
			s = a
		} else {
			s = ""
		}
	}
	// ... existing steps 2–5 UNCHANGED ...

// PATTERN (the precedence sibling — the site's existing block becomes if/else-if):
if h := provider.EventErrorHint(out, deps.Manifest); h != "" { // FR-S3: in-stream error, exit code unreliable
	lastCause = errors.New(h)
} else if h := provider.StderrHint(out, stderr); h != "" { // print-mode: stderr is the cause
	lastCause = errors.New(h)
}

// PATTERN (the scan's win rules):
lastTerminal := ""; firstErr := ""
for _, ln := range strings.Split(raw, "\n") {
	ln = strings.TrimRight(ln, "\r")
	if strings.TrimSpace(ln) == "" { continue }
	switch oc := ex.Extract(ln); oc.Kind {
	case EventTerminal: lastTerminal = oc.Message
	case EventError:    if firstErr == "" { firstErr = oc.ErrText }
	} // EventIgnored: progress/heartbeat/tool — skip
}
// terminal beats error: if lastTerminal != "" ⇒ (lastTerminal, true, ""); else ("" , false, firstErr)
```

### Integration Points

```yaml
CONSUMES (READ-ONLY — S1's landed surface):
  - manifest.EventStream + CmdSpec.ProgressiveStdout (render arms it; executor.go:144 consumes it — P1.M2).
  - events.go's registry (LookupEventExtractor/RegisterEventExtractor — signatures UNCHANGED in S2).
  - executor.go Execute + StderrHint + countingWriter (P1.M2 — NO edits).
PRODUCES (consumed by siblings):
  - P2.M2.T1.S1 (verification harness): drives Render→Execute→ParseOutput per provider and records the
    dated evidence; uses RegisterEventExtractor to trial candidate extractors.
  - P2.M2.T1.S2–S5 (population): register the verified per-provider Extractors + declare event_stream in
    builtin.go — the framework they plug into is THIS task's deliverable.
NO executor changes. NO built-in declarations. NO providers/*.toml changes. NO config-schema changes.
```

## Validation Loop

### Level 1: Build & static (Immediate Feedback)

```bash
go build ./...
go vet ./internal/provider/... ./internal/generate/... ./internal/decompose/...
gofmt -l internal/provider/ internal/generate/ internal/decompose/ | grep -v '^$' && echo "FAIL: unformatted" || echo "OK: formatted"
# Expected: all clean, exit 0.
```

### Level 2: Unit tests (the scan semantics + print-mode regression)

```bash
go test ./internal/provider/ -race -run 'Event|ParseOutput' -v   # events_test + parse_test (new + existing)
go test ./internal/provider/... -race                            # the WHOLE package — S1's render/manifest/
                                                                 # merge/builtin tests green (incl. the updated fake)
go test ./internal/generate/... ./internal/decompose/... -race   # the call-site packages
# Expected: all green. Every PRE-EXISTING parse/render test passes UNCHANGED (print mode byte-identical).
```

### Level 3: The e2e rides (the contract's three tests — every platform)

```bash
go test ./internal/generate/ -race -run 'EventStream' -v
# Expected: PASS — (1) happy: progressive JSONL ticks + terminal event → err==nil under Idle:1s with a
# ~2.5s run (portable idle survival) + ParseOutput assembles "feat: add widget";
# (2) FR-S3: exit-0 error event → ok==false + EventErrorHint carries the text + RescueError.Cause set;
# (3) S1's no-extractor render guard un-weakened.
```

### Level 4: FR-S2 discipline + scope guard

```bash
rg -n 'event_stream' providers/ && echo "FAIL: a built-in declares event_stream (FR-S2)" || echo "OK: none"
rg -n 'RegisterEventExtractor\(' internal/provider/builtin.go internal/provider/render.go && echo "FAIL: real extractor registered" || echo "OK: registry code-free"
# (the eventExtractors map literal in events.go is the ONLY declaration site and ships empty)
rg -n 'Event-stream capability' docs/providers.md   # Expected: 1 hit (the Mode A section)
git status --porcelain
# Expected EXACTLY: internal/provider/{events.go,events_test.go,parse.go,parse_test.go,render_test.go},
# internal/generate/{generate_eventstream_test.go,generate.go}, internal/decompose/{message.go,planner.go,
# arbiter*.go — only if the arbiter site needed the unwrap}, docs/providers.md.
git status --porcelain | grep -E 'executor\.go|manifest\.go|merge\.go|render\.go$|builtin\.go|providers/.*toml|stubtest|stubagent|spec/|PRD|plan/' && echo "FAIL: out of scope" || echo "OK: scope clean"
# Expected: "OK: scope clean".
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1: build/vet/gofmt clean
- [ ] Level 2: `go test ./internal/provider/... ./internal/generate/... ./internal/decompose/... -race` GREEN
- [ ] Level 3: the three e2e rides PASS on every platform
- [ ] Level 4: no providers/*.toml event_stream; registry ships empty; docs section present; scope guard clean

### Feature Validation
- [ ] Extractor refined to EventOutcome (Ignored/Terminal/Error); S1's fakeExtractor updated; registry seams unchanged
- [ ] ParseEventStream semantics: last-terminal-wins, first-error-recorded, terminal-beats-error, lenient on junk
- [ ] ParseOutput step-0: event mode assembles from the terminal event; cleanup steps apply; print mode byte-identical
- [ ] UnwrapEventOutput at planner (+ arbiter if it parses raw stdout outside ParseOutput)
- [ ] EventErrorHint precedes StderrHint at all 3 cause sites; exit-0 error events surface through RescueError.Cause
- [ ] e2e: portable idle survival proven (1s idle, 2.5s of 50ms ticks, every platform)
- [ ] docs/providers.md: the Event-stream capability section (verification bar, naming trap, pitfalls verbatim)

### Scope-Boundary Validation
- [ ] executor.go / manifest.go / merge.go / render.go / builtin.go / providers/*.toml UNTOUCHED
- [ ] stubtest + stubagent UNCHANGED (the existing knobs suffice — EMIT_LINE + OUT carry the JSONL)
- [ ] No real extractor registered; no built-in declares event_stream (FR-S2 — P2.M2 owns population)
- [ ] spec/ / PRD / plan/ untouched

### Documentation & Deployment
- [ ] Mode A satisfied: the docs/providers.md capability section rides with this task
- [ ] parse.go + events.go headers document the event path (step-0 comment; FR refs)

---

## Anti-Patterns to Avoid

- ❌ Don't rewrite/reorder the existing 5-step pipeline — INSERT the step-0 branch; print mode must stay
  byte-identical (every existing parse_test passes unchanged).
- ❌ Don't edit StderrHint (or Execute) — P1.M2's landed surface. EventErrorHint is a PRECEDENCE SIBLING
  consulted first at the cause sites; StderrHint keeps serving print mode.
- ❌ Don't give the test stub a nonzero exit for the FR-S3 case — the premise is exit 0 (pi documents that
  failures don't reliably exit nonzero). Exit≠0 tests the ExitError path instead.
- ❌ Don't put e2e stubtest tests in internal/provider's same package — stubtest imports provider (cycle).
  Unit tests there are pure-function; e2e lives in internal/generate.
- ❌ Don't make the Extractor stateful or accumulate deltas — registry singletons are shared by concurrent
  roles (data race); the terminal event is self-contained by FR-S2's recorded-shape model.
- ❌ Don't error on unknown/malformed event lines — classify EventIgnored (lenient; streams carry many
  event types stagecoach doesn't care about).
- ❌ Don't let an error event beat a terminal message — a produced message is a success; FR-S3 fires only
  when NO terminal message event exists.
- ❌ Don't register any real extractor or declare event_stream on any built-in — FR-S2: never speculative;
  P2.M2.T1.S2–S5 own population after dated verification.
- ❌ Don't forget the planner/arbiter unwrap — they parse via ParsePlannerOutput/arbiter-parse, NOT
  ParseOutput; without `UnwrapEventOutput` their event-mode stdout is unparseable NDJSON.
- ❌ Don't double-unwrap the arbiter if it routes through message.go's ParseOutput — grep first; wrap only
  the sites that parse raw Execute stdout directly.
- ❌ Don't skip updating S1's fakeExtractor — the interface refinement breaks its compile (S1's PRP
  anticipated exactly this churn).
- ❌ Don't use tick intervals near/above the 500ms watcher poll — several ticks must land between polls
  (50ms) or the idle-survival test flakes.

---

## Confidence Score

**One-pass success likelihood: 9/10.** Every integration point is verified against the live code (the 7
ParseOutput sites, the 3 StderrHint cause sites with their exact in-scope manifest variables, the planner's
ParsePlannerOutput divergence, StderrHint's structural hole, the D4 arming, the stubtest/stubagent knobs incl.
the exit-0 default and the 500ms watcher tick); the S1 contract is explicit about the placeholder refinement
S2 performs; the scan semantics (win rules, leniency, defensiveness) are fully specified; and the print-mode
byte-identity requirement is structurally guaranteed by the `len==0` branch guard. The 1-point deduction:
the arbiter's parse site is located by grep at impl time (verified NOT to use ParseOutput, but its exact
file/shape was not read), and the RescueError.Cause e2e assertion depends on following the idle_parity test
harness's exact wiring — both are one-read tasks at impl, called out as explicit task steps.
