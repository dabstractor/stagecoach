name: "P2.M2.T1.S1 — Event-stream verification harness + recording protocol (dated empirical runs, manifest header records) (§9.30 FR-S2, → G30)"
description: |
  A TEST-INFRASTRUCTURE-ONLY subtask that defines the empirical verification PROTOCOL for the
  event-stream output mode (FR-S2) and ships the harness that executes it — WITHOUT verifying any
  provider (S2–S5 do that) and WITHOUT populating anything (registry stays empty; no built-in
  declares event_stream). Two NEW files: (1) `internal/generate/events_verify_test.go`
  (`//go:build integration_real`, package generate — reuses realagent_test.go's envOr/realDefaults/
  realConfig helpers): the file-header doc comment IS the runbook; `TestEventStreamVerify` (gated on
  STAGECOACH_RUN_REAL=1, per-provider subtests) drives, per candidate CLI, a REAL generation in its
  event mode capturing stdout WITH PER-LINE TIMESTAMPS (spawn the rendered CmdSpec directly — Execute
  buffers stdout and cannot timestamp; Phase B re-drives assembly via ParseEventStream/ParseOutput),
  proving (i) progressive emission DURING generation (≥2 lines, first arrival ≥500ms before exit —
  the exit-burst signature FAILS, the json-vs-stream-json trap), (ii) the terminal-event shape
  (verbatim dump; it becomes S2–S5's Extractor; Phase B mechanically asserts assembly once the real
  extractor is registered — the red→green loop), (iii) in-stream error surfacing (forced invalid
  model; record the verbatim error event + EXIT CODE — FR-S3's exit-0 premise — + EventErrorHint).
  The harness emits the FILLED recording template via t.Logf for copy-paste into builtin.go's
  constructor doc comment (the §7.3 `VERIFIED <date> via <probe>` precedent). (2)
  `internal/provider/events_ledger_test.go` (ALWAYS-ON, package provider): the FR-S2 enforcement
  ledger — `eventStreamLedger` (EMPTY now; S2–S5 add entries ONLY with a passing dated run);
  `TestEventStreamLedger` asserts built-ins declaring EventStream ⇔ ledger entries, failing with a
  pointer to this protocol — the mechanical form of "populating event_stream without a passing dated
  record is FORBIDDEN." Consumes S2's landed exports (ParseEventStream/EventErrorHint/EventOutcome/
  EventIgnored + S1's registry seams). NO production-code change, NO docs file, NO mock (the item's
  Mock: none). Validates via go build, go vet -tags, the ledger guard green, the harness SKIPPING
  cleanly without the env gate, existing suites green, gofmt, and a scope guard.

---

## Goal

**Feature Goal**: The empirical verification protocol for event-stream mode (FR-S2) exists as an
executable, dated, self-documenting runbook — so P2.M2.T1.S2–S5 can, per provider, run a REAL
generation in its event mode, capture timestamped evidence for the three required proofs, paste a
filled `VERIFIED <date>` record into the manifest header, and only then declare `event_stream` +
register the extractor — with an always-on ledger test mechanically forbidding undated population.

**Deliverable**:
- `internal/generate/events_verify_test.go` — NEW, `//go:build integration_real`, package generate:
  the runbook (file-header doc comment), `TestEventStreamVerify` + per-provider subtests, the
  candidate event-flag table (env-overridable), `probeExtractor`, Phase A live timestamped capture,
  Phase B extractor trial, the invalid-model error probe, the gap analysis for the heartbeat/sparse
  judgment, and the t.Logf recording-template emitter.
- `internal/provider/events_ledger_test.go` — NEW, always-on, package provider: `eventStreamLedger`
  (ships EMPTY) + `TestEventStreamLedger` (the FR-S2 both-directions guard).

**Success Definition**: `go build ./...` clean; `go vet -tags integration_real ./internal/generate/`
clean; `go test ./internal/provider/ -run TestEventStreamLedger -v` GREEN with the empty ledger;
`go test -tags integration_real ./internal/generate/ -run TestEventStreamVerify -v` reports SKIP
(not FAIL) without `STAGECOACH_RUN_REAL=1`; `go test ./internal/generate/... ./internal/provider/...
-race` GREEN (no regressions — the new files add no always-on behavior beyond the ledger assert);
gofmt clean; `git status --porcelain` shows EXACTLY the two new files; no built-in declares
event_stream; the registry map untouched (S2's `rg` gates still pass).

## User Persona (if applicable)

**Target User**: The stagecoach maintainer/verifier executing P2.M2.T1.S2–S5 (per-provider
verification + population). Today, no end user — nothing is populated.
**Use Case**: For a candidate provider (e.g. pi `--mode json`), the maintainer runs
`STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/ -run TestEventStreamVerify/pi -v -timeout 10m`,
reads the timestamped evidence + verbatim terminal event, authors the extractor (S2–S5's task),
re-runs until Phase B is green, pastes the emitted `VERIFIED <date>` record into builtin.go's
constructor comment, adds the ledger entry, and declares `event_stream` — the red→green loop.
**Pain Points Addressed**: (1) FR-S2's "MUST NOT be set speculatively" had no executable protocol —
now it does; (2) proving *progressive* emission needs per-line timestamps, which `Execute`'s
buffered stdout cannot provide — the harness captures live; (3) undated population is currently
only conventionally forbidden — the ledger test makes it mechanically red.

## Why

- **FR-S2 discipline needs teeth**: the PRD's verification bar (dated empirical run proving (i)
  progressive emission, (ii) terminal-event shape, (iii) error surfacing) is what keeps a buffered
  `json` mode or a heartbeat-only stream from being mistaken for a progressive event stream. The
  protocol + ledger make the bar executable and enforceable rather than aspirational.
- **The §7.3 precedent generalizes**: provider_subsystem.md §7.3 already requires every per-provider
  capability claim to carry `// VERIFIED <date> via <probe>` (builtin.go:54/:61 live examples). The
  event-stream claim is the next such claim; this task defines its probe protocol + record format.
- **S2's contract names this task**: S2's Integration Points say P2.M2.T1.S1 "drives
  Render→Execute→ParseOutput per provider and records the dated evidence; uses
  RegisterEventExtractor to trial candidate extractors" — S2 ships the framework EMPTY precisely so
  this protocol governs population.
- **Scope discipline**: this task ships ZERO population, ZERO production-code change, ZERO docs
  (providers.md's capability section is S2's). The two new files are the entire surface.

## What

A build-tagged verification harness (the runbook + the three-proof capture) plus an always-on
FR-S2 ledger guard. No built-in changes, no extractor registration, no docs.

### Success Criteria
- [ ] `internal/generate/events_verify_test.go` exists with `//go:build integration_real` (line 1),
      package `generate`, and a file-header doc comment carrying THE RUNBOOK: the manual command, the
      three proofs and their pass criteria, the heartbeat/sparse judgment duty, the recording
      template, and the paste destination (builtin.go constructor doc comment, §7.3 precedent).
- [ ] `TestEventStreamVerify` skips unless `STAGECOACH_RUN_REAL=1` (realagent_test.go's exact gate)
      and has per-provider subtests for pi, claude, cursor, codex, opencode (agy excluded —
      experimental, mirroring realagent's exclusion), runnable individually via `-run
      TestEventStreamVerify/<name>`.
- [ ] The candidate event-flag table is seeded from §9.30 (pi `--mode json`; claude/cursor
      `--output-format stream-json`; codex `--json`; opencode `--format json`) and is env-overridable
      per provider via `STAGECOACH_VERIFY_EVENT_FLAGS_<NAME>` (space-separated tokens).
- [ ] Phase A: registers `probeExtractor` (all-`EventIgnored`) via `RegisterEventExtractor` to pass
      S1's render guard; renders a COPY of the built-in manifest with `EventStream = <candidate>`
      via `Render`; asserts `spec.ProgressiveStdout == true`; spawns `spec.Command`+`spec.Args`
      DIRECTLY (Env, Stdin, repo cwd) with a StdoutPipe reader goroutine stamping
      `time.Since(t0)` per line; captures exit offset, exit code, stderr; asserts the mechanical
      progressiveness criterion (≥2 non-blank lines at distinct offsets AND
      `exitOffset − firstLineOffset ≥ 500ms`, tunable via `STAGECOACH_VERIFY_MIN_SPAN_MS`) — the
      exit-burst signature FAILS with the json-vs-stream-json explanation; dumps every line
      VERBATIM via t.Logf.
- [ ] Phase B: if `LookupEventExtractor(name) != nil`, run `ParseEventStream` on the captured raw
      stream + `ParseOutput` and assert the assembled message equals the probe prompt's expected
      message shape; run `EventErrorHint` on the error capture. Else t.Skipf with a pointer to the
      Phase A evidence (the red→green loop's red state is a SKIP, not a FAIL).
- [ ] Error probe (proof iii): a second invocation with the model forced to
      `stagecoach-verify-invalid-model`; capture the stream verbatim + the EXIT CODE + stderr;
      record whether an error/abort event appears in-stream (Phase A: human reads the shape;
      Phase B: `EventErrorHint` mechanically surfaces it).
- [ ] Gap analysis reported: max inter-line gap + a per-line offset table (input for the
      heartbeat/sparse judgment field in the record — codex heartbeat / opencode sparse pitfalls).
- [ ] The harness emits the FILLED recording template via `t.Logf` (date auto-filled) ready to paste.
- [ ] `internal/provider/events_ledger_test.go` exists (always-on): `eventStreamLedger
      map[string]string{}` ships EMPTY; `TestEventStreamLedger` fails if any built-in resolves a
      non-empty `EventStream` without a dated ledger entry, or a ledger entry exists without a
      declaration (stale), or an entry's date is empty — with failure text pointing at the protocol.
- [ ] `go build ./...`, `go vet -tags integration_real ./internal/generate/`,
      `go test ./internal/provider/ -run TestEventStreamLedger`, the no-env SKIP check,
      `go test ./internal/generate/... ./internal/provider/... -race`, gofmt — all clean.
- [ ] `git status --porcelain` shows EXACTLY the two new files; no registry change; no built-in
      change; `rg -n 'event_stream' providers/*.toml` zero hits (S2's gate still green).

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes** — the S2 exported API surface this harness calls, the exact CmdSpec/Render/
Execute signatures (verified live), why Execute cannot timestamp (buffered) and the direct-spawn
alternative, S1's render guard + the probe-extractor workaround (no Unregister — process-lifetime
is fine), the realagent gating pattern + the exact same-package helpers available for reuse
(envOr/realDefaults/realConfig with the AGENT-vs-inference terminology), the §7.3 dated-record
precedent with live line examples, the mechanical pass criteria, the §9.30 pitfalls the record must
address, and the file-level no-conflict boundary against S2's in-flight file set.

### Documentation & References

```yaml
# MUST READ — this task's verified research (APIs, precedents, criteria, scope).
- docfile: plan/022_856008a8cbb5/P2M2T1S1/research/findings.md
  why: "§0 S2's landed exports (the API the harness calls); §1 the §7.3 dated-record precedent +
        live builtin.go examples; §2 the realagent gating pattern + the reusable same-package helpers;
        §3 CmdSpec/Render/Execute signatures + WHY Phase A spawns directly (Execute buffers stdout);
        §4 the render-guard/probe-extractor mechanism (no Unregister — acceptable); §5 the two-file
        deliverable set (zero overlap with S2's files); §6 the three proofs' mechanical criteria;
        §7 the heartbeat/sparse judgment; §8 scope boundaries."

# MUST READ — S2 is the CONTRACT (its exports are the harness's API; its files are off-limits).
- docfile: plan/022_856008a8cbb5/P2M1T1S2/PRP.md
  why: "Defines EventKind/EventOutcome/Extractor/ParseEventStream/UnwrapEventOutput/EventErrorHint
        (the Phase B API), the registry seams (S1's, unchanged), the FR-S3 exit-0 premise, the 500ms
        watcher tick, the stubtest import-cycle note (NOT applicable here — no stub), and its OWN
        file set (events_test.go, generate_eventstream_test.go, docs/providers.md) which THIS TASK
        MUST NOT TOUCH."
  gotcha: "S2 is in flight. If an exported name shifts at merge (e.g. ParseEventStream's returns),
           follow the LANDED events.go — the harness compiles against reality, not the PRP."

# MUST READ — the FR-S spec (§9.30) + the verification bar this protocol executes.
- docfile: plan/022_856008a8cbb5/prd_snapshot.md
  section: "§9.30 (line ~659): FR-S1 (the event-mode flag table + the naming trap: claude/cursor/agy
            offer BOTH json and stream-json — only the progressive one qualifies), FR-S2 (the
            verification bar THIS TASK makes executable + heartbeat/sparse pitfalls), FR-S3 (error
            surfacing + the exit-0 premise), FR-A3 (portable pipe activity)."

# MUST READ — the dated-record precedent + the registry-placement rationale.
- docfile: plan/022_856008a8cbb5/architecture/provider_subsystem.md
  section: "§7.3 (the VERIFIED-comment pattern to copy, with builtin.go:54/:61 examples and the
            verbatim 'new per-provider capability claim must carry this same dated, probe-named
            verification' rule); §7.1 (BuiltinManifests fresh-map-per-call — copy before mutating);
            §7.4 (the events.go sibling-registry rationale)."

# MUST READ — the gating + helper precedent (same package, same tag ⇒ symbol reuse).
- file: internal/generate/realagent_test.go
  why: "The exact pattern to mirror: //go:build integration_real (line 1), STAGECOACH_RUN_REAL=1
        runtime gate, the manual-command header comment, and the same-package helpers my harness
        REUSES: envOr (:59), realDefaults + realDefault{model,inference} (:36–48, incl. pi
        {claude-haiku,anthropic} and opencode {anthropic/claude-sonnet-4,\"\"}), realConfig (:63).
        Note the AGENT-vs-inference-provider terminology comment (:28–33) — never conflate."
  pattern: "func TestX(t *testing.T) { if os.Getenv(\"STAGECOACH_RUN_REAL\") != \"1\" {
            t.Skip(\"...\") } for _, name := range providers { t.Run(name, func(t *testing.T){...}) } }"

# MUST READ — the API the harness drives (verified signatures).
- file: internal/provider/render.go
  why: "CmdSpec{Command, Args, Stdin, PayloadBytes, Env, ProgressiveStdout} (:32–45) — Phase A
        spawns spec.Command + spec.Args with spec.Env/spec.Stdin. Render(model, sysPrompt,
        userPayload, reasoning string, mode ...RenderMode) (*CmdSpec, error) (:106) — Validate→
        Resolve→tokens; safe on a manifest COPY (Resolve copies). ProgressiveStdout arms iff the
        manifest declares event_stream + a registered extractor exists (S1's guard — §4 findings)."
- file: internal/provider/executor.go
  why: "READ-ONLY: Execute (:104) returns BUFFERED (stdout, stderr, err) — it cannot provide
        per-line timestamps (why Phase A spawns directly). Deadlines{Idle,Hard} (:41). NO edits."
- file: internal/provider/events.go
  why: "S1's landed registry (LookupEventExtractor/RegisterEventExtractor; the EMPTY eventExtractors
        map — STAYS EMPTY) + S2's in-flight additions (the Phase B API). NO edits."

# MUST READ — where the records LAND (the paste destination S2–S5 write to).
- file: internal/provider/builtin.go
  why: "The VERIFIED-comment precedent lines (:54, :61) — the format the recording template emits and
        where S2–S5 paste it (constructor doc comment + inline field comment). NO edits in THIS task."

# CONTEXT — the consumers.
- docfile: plan/022_856008a8cbb5/tasks.json
  why: "P2.M2.T1.S2–S5 are the protocol's consumers (pi, claude/cursor/agy, codex, opencode) — the
        recording template + ledger are their interface. (Read-only reference; never edit.)"
```

### Current Codebase tree (relevant slice — READ-ONLY references)

```bash
internal/generate/realagent_test.go        # the gating + helpers precedent (REUSE its symbols)
internal/provider/events.go                # S1's registry (EMPTY) + S2's in-flight Phase-B API
internal/provider/render.go                # CmdSpec + Render (the harness drives these)
internal/provider/executor.go              # Execute/Deadlines (READ-ONLY; why Phase A bypasses it)
internal/provider/builtin.go               # the VERIFIED-record paste destination (:54/:61 precedent)
plan/022_856008a8cbb5/architecture/provider_subsystem.md   # §7.3 the dated-record pattern
plan/022_856008a8cbb5/P2M1T1S2/PRP.md      # the S2 CONTRACT (API + off-limits files)
```

### Desired Codebase tree with files to be added

```bash
internal/generate/
  events_verify_test.go    # NEW — //go:build integration_real; runbook header + TestEventStreamVerify
                           #   (Phase A live timestamped capture / Phase B extractor trial / error probe
                           #   / gap analysis / recording-template emitter)
internal/provider/
  events_ledger_test.go    # NEW — always-on; eventStreamLedger (EMPTY) + TestEventStreamLedger (FR-S2)
# NOT touched: events.go, parse.go, executor.go, render.go, builtin.go, providers/*.toml,
#              docs/providers.md, S2's events_test.go + generate_eventstream_test.go, stubtest, PRD/plan
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (Execute cannot timestamp — Phase A must spawn directly): provider.Execute returns BUFFERED
//   stdout; proof (i) needs per-line ARRIVAL offsets. Spawn the rendered CmdSpec yourself
//   (exec.CommandContext(ctx, spec.Command, spec.Args...), cmd.Env = spec.Env, cmd.Stdin =
//   strings.NewReader(spec.Stdin), cmd.Dir = <temp repo>, cmd.Stderr captured) with a StdoutPipe
//   reader goroutine stamping time.Since(t0) per line. Phase B then runs the REAL parse path
//   (ParseEventStream/ParseOutput/EventErrorHint) on the captured raw stream — and S2's stubtest e2e
//   already covers the Execute path mechanically.

// CRITICAL (S1's render guard forces probe registration BEFORE Render): Render REFUSES event_stream
//   for a provider with no registered extractor. Phase A must RegisterEventExtractor(name,
//   probeExtractor{}) FIRST (probeExtractor returns {Kind: EventIgnored} for every line — it satisfies
//   the guard and classifies nothing). There is NO Unregister API — fine: the manual run is a one-shot
//   process (the global map dies with it) and no other test in that process consults extractors.

// CRITICAL (copy the manifest before mutating — BuiltinManifests returns a FRESH map, but a returned
//   Manifest's SLICE fields may alias internals): build the trial as m := provider.BuiltinManifests()
//   [name]; m.EventStream = candidateFlags — a fresh map per call means the struct is yours; setting a
//   (previously nil) slice field cannot alias anything. NEVER write back to BuiltinManifests or
//   builtin.go — the trial is test-local (that IS the FR-S2 point: shipped manifests stay clean).

// CRITICAL (the AGENT vs inference-provider terminology — realagent_test.go:28–33): the provider NAME
//   (pi, claude, …) is the AGENT; realDefault.inference is the upstream MODEL-PROVIDER (anthropic,
//   zai, …) routed via the agent's --provider flag. Reuse realConfig(name) exactly as realagent does;
//   never conflate the two, never put inference into cfg.Provider.

// CRITICAL (agy excluded): realagent's provider list excludes agy (experimental — not real-tested).
//   Mirror the exclusion (claude/cursor/agy verification is ONE sibling task P2.M2.T1.S3, but the
//   agy SUBTEST in this harness would violate the established exclusion — leave it out; S3 can extend
//   the table if it chooses to verify agy against the live binary).

// CRITICAL (exit code is EVIDENCE, not failure — FR-S3's premise): a failed/aborted generation does
//   NOT reliably produce a nonzero exit (pi documents this). The error probe records the exit code
//   VERBATIM into the evidence; the harness must not assert exit!=0 (that would encode the wrong
//   premise) — it asserts only that the capture happened and reports what surfaced.

// GOTCHA (build-tag discipline): BOTH new behaviors must not run in CI. The harness file carries
//   //go:build integration_real (make test/coverage pass no -tags ⇒ never compiled); inside, the
//   STAGECOACH_RUN_REAL=1 runtime gate skips. The LEDGER test is the ONLY always-on piece (it must be
//   — it guards population) and must live in the UNTAGGED events_ledger_test.go, not the harness file.

// GOTCHA (no conflict with S2's in-flight files): S2 creates internal/provider/events_test.go and
//   internal/generate/generate_eventstream_test.go. This task's files are events_verify_test.go +
//   events_ledger_test.go — distinct names, both packages. Do NOT touch S2's files.

// GOTCHA (S2's exported names are the API — verify against the landed events.go at impl): the PRP
//   names ParseEventStream(raw, m) (msg, ok, errText), EventErrorHint(raw, m) string, EventOutcome/
//   EventIgnored per S2's contract. If S2's landed signatures differ, code against the LANDED ones.

// GOTCHA (the probe prompt must be deterministic and identifiable): the happy-path probe payload
//   should instruct a FIXED short message (e.g. "Reply with exactly: stagecoach-event-verify") so
//   Phase B's assembly assertion is mechanical (assembled message contains the sentinel), not
//   model-dependent prose.

// GOTCHA (timestamps must be monotonic): use time.Since(t0) with one t0 captured at spawn; do NOT
//   compare wall-clock time.Now() strings across goroutines. Stamp in the reader goroutine only.

// GOTCHA (CI/OS): the harness runs manually on the maintainer's machine; keep it free of
//   procfs/Linux-only assumptions (plain exec + pipe reading is portable). The temp repo needs git
//   identity for nothing (no commits are made — Render+spawn only), but create the payload from a
//   fixed diff string, not a live repo, to keep the probe hermetic; a t.TempDir() working dir suffices.
```

## Implementation Blueprint

### Data models and structure

No production data models — this is test infrastructure. The "data" is:

```go
// internal/generate/events_verify_test.go (package generate, //go:build integration_real):

// eventCandidate: one provider's provisional event-mode flags (S2–S5 verify/adjust + record).
type eventCandidate struct {
	flags []string // provisional EventStream tokens (PRD §9.30; env-overridable)
}

// eventCandidates: seeded from §9.30 FR-S1's documented flags. THE STARTING POINT ONLY — the
// verification may falsify a candidate (that is the point); S2–S5 record the final disposition.
var eventCandidates = map[string]eventCandidate{
	"pi":       {flags: []string{"--mode", "json"}},
	"claude":   {flags: []string{"--output-format", "stream-json"}},
	"cursor":   {flags: []string{"--output-format", "stream-json"}},
	"codex":    {flags: []string{"--json"}},
	"opencode": {flags: []string{"--format", "json"}},
}

// probeExtractor satisfies S1's render guard; classifies NOTHING (Phase A is capture).
type probeExtractor struct{}
func (probeExtractor) Extract(line string) provider.EventOutcome { return provider.EventOutcome{Kind: provider.EventIgnored} }

// capturedLine / eventCapture: the evidence record.
type capturedLine struct {
	offset time.Duration // time.Since(t0) when the line arrived (stamped in the reader goroutine)
	text   string        // the raw line, verbatim
}
type eventCapture struct {
	lines    []capturedLine
	exitAt   time.Duration
	exitCode int
	stderr   string
}

// internal/provider/events_ledger_test.go (package provider, ALWAYS-ON):

// eventStreamLedger: provider → the VERIFIED date from its passing empirical run. Ships EMPTY
// (FR-S2: nothing is populated yet). P2.M2.T1.S2–S5 add an entry ONLY with a passing dated run
// recorded in builtin.go's constructor doc comment, alongside their declaration + extractor.
var eventStreamLedger = map[string]string{
	// "pi": "2026-07-21", // ← the shape S2–S5 will add (dated, probe-named, alongside the builtin.go record)
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/provider/events_ledger_test.go — the always-on FR-S2 guard
  - Package provider, NO build tag (always compiled, always run).
  - Header comment: the FR-S2 rule ("populating event_stream without a passing dated record is
    FORBIDDEN"), the ledger's role (the mechanical record of WHAT is populated + WHEN verified), and
    a pointer to the protocol (internal/generate/events_verify_test.go's runbook; the §7.3
    builtin.go VERIFIED-comment precedent for the per-provider record).
  - eventStreamLedger (EMPTY map, per the blueprint) + TestEventStreamLedger:
    for name, m := range BuiltinManifests() {
        r := m.Resolve()
        declared := len(r.EventStream) > 0
        date, recorded := eventStreamLedger[name]
        if declared && !recorded → t.Errorf("%s declares event_stream but the FR-S2 ledger has no dated verification record — run the P2.M2.T1.S1 protocol (internal/generate/events_verify_test.go) and record the dated evidence in builtin.go before populating", name)
        if recorded && !declared → t.Errorf("%s has a ledger entry (%s) but the manifest declares no event_stream — stale ledger", name, date)
        if recorded && strings.TrimSpace(date) == "" → t.Errorf("%s ledger entry has an empty date", name)
    }
  - VALIDATE: go test ./internal/provider/ -run TestEventStreamLedger -v   # PASS (empty ledger, nothing declared)

Task 2: CREATE internal/generate/events_verify_test.go — the runbook header + gating + candidates
  - Line 1: //go:build integration_real. Package generate. Header doc comment = THE RUNBOOK:
    (a) WHAT: the FR-S2 empirical verification protocol for event-stream mode — the three proofs;
    (b) MANUAL COMMAND: STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/
        -run TestEventStreamVerify/<name> -v -timeout 10m  (mirrors realagent's header);
    (c) THE THREE PROOFS + pass criteria (i: ≥2 lines, first ≥500ms pre-exit, exit-burst = FAIL
        [the json-vs-stream-json trap]; ii: the verbatim terminal event → S2–S5's Extractor;
        iii: forced invalid model → verbatim error event + EXIT CODE + EventErrorHint);
    (d) THE RECORDING TEMPLATE (verbatim, with <placeholders>) + the paste destination
        (builtin.go constructor doc comment, §7.3 VERIFIED pattern) + the ledger entry step;
    (e) THE HEARTBEAT/SPARSE JUDGMENT duty (codex timer-events ≠ progress; opencode sparse gaps —
        read the gap analysis, write the judgment into the record);
    (f) the red→green loop (Phase A evidence → author extractor [S2–S5] → register in production
        code → re-run → Phase B green → paste record + ledger + declare).
  - TestEventStreamVerify: the STAGECOACH_RUN_REAL gate (copy realagent's exact idiom) +
    t.Run subtests over eventCandidates' keys in registry preference order (pi, opencode, cursor,
    codex, claude — NO agy).
  - Candidate override: STAGECOACH_VERIFY_EVENT_FLAGS_<NAME> (uppercase; space-split; empty ⇒ keep
    the table entry). Reuse envOr from realagent_test.go (same package + tag).
  - VALIDATE: go vet -tags integration_real ./internal/generate/ && go test -tags integration_real
    ./internal/generate/ -run TestEventStreamVerify -v   # SKIP cleanly (no env set)

Task 3: IMPLEMENT Phase A — the live timestamped capture (the harness core)
  - probeRun(t, name string, flags []string, model string, payload string) eventCapture:
    1. m := provider.BuiltinManifests()[name]; m.EventStream = flags (the TRIAL — test-local copy).
    2. provider.RegisterEventExtractor(name, probeExtractor{}) — S1's render guard (findings §4;
       no Unregister; the process is one-shot — note in a comment).
    3. spec, err := m.Render(model, "", payload, "off") — assert err==nil; assert
       spec.ProgressiveStdout == true (the arming proof) AND the candidate tokens appear in
       spec.Args (spot-check the render inserted them).
    4. Spawn DIRECTLY (findings §3): ctx (test timeout), exec.CommandContext(ctx, spec.Command,
       spec.Args...), cmd.Env = spec.Env, cmd.Stdin = strings.NewReader(spec.Stdin), cmd.Dir =
       t.TempDir(), StderrPipe captured; StdoutPipe → reader goroutine: bufio.Scanner, per line
       stamp time.Since(t0) + append {offset, text}. Wait; record exitAt/exitCode/stderr.
  - The happy-path probe: payload = a fixed instruction ("You are being verified. Reply with
    exactly: stagecoach-event-verify") + a small fixed diff snippet (hermetic — no live repo
    needed); model from realagent's realDefaults/realConfig plumbing (envOr overrides).
  - MECHANICAL CHECK (proof i): ≥2 non-blank lines at distinct offsets AND
    exitAt − lines[0].offset ≥ minSpan (500ms; env STAGECOACH_VERIFY_MIN_SPAN_MS). On FAIL include
    the json-vs-stream-json explanation + the offset table in the message.
  - REPORT (t.Logf): every line VERBATIM with its offset; exit code; stderr; the gap analysis
    (max inter-line gap; count of gaps >1s) — the heartbeat/sparse input.
  - VALIDATE: compile (Task 2's vet covers it); the real run is S2–S5's to execute.

Task 4: IMPLEMENT the error probe (proof iii)
  - The same probeRun with model forced to "stagecoach-verify-invalid-model" (the rendered model
    flag carries a bogus value ⇒ the agent errors in-stream or via stderr/exit).
  - RECORD ONLY (assert nothing about exit — findings' FR-S3 gotcha): dump the captured lines
    verbatim, the exit code, stderr; note whether any line looks like an error/abort event.
    Phase A's job is the SHAPE; the human (S2–S5) reads it into the Extractor's error branch.

Task 5: IMPLEMENT Phase B — the extractor trial (the red→green loop's green)
  - If provider.LookupEventExtractor(name) returns non-nil AND it is NOT probeExtractor (i.e.
    S2–S5 registered a REAL one in production code): msg, ok, errText := provider.ParseEventStream(
    capturedRaw, m); assert ok && strings.Contains(msg, "stagecoach-event-verify"); then
    provider.ParseOutput(capturedRaw, m) → the assembled message; EventErrorHint on the ERROR
    capture → assert non-empty (the FR-S3 mechanical proof) when an error event was recorded.
  - Else: t.Skipf("no real extractor registered for %s yet — Phase A evidence above is the
    deliverable; author + register the extractor (P2.M2.T1.S2–S5), then re-run for Phase B", name).

Task 6: IMPLEMENT the recording-template emitter
  - t.Logf the FILLED template (date = time.Now().Format("2006-01-02"); CLI version from the
    capture's stderr if present, else "<record version>"):
      // EVENT-STREAM (§9.30 FR-S1/FR-S2) — VERIFIED <DATE> (<cli + version>), harness P2.M2.T1.S1
      // (`STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/ -run TestEventStreamVerify/<name> -v`):
      //   (i)   PROGRESSIVE: <N> lines over <span>ms (first @<t1>ms, exit @<tE>ms; max gap <g>ms) — no exit-burst.
      //   (ii)  TERMINAL EVENT (verbatim): <the terminal line>
      //   (iii) ERROR (forced invalid model): exit=<code>; error event (verbatim): <line or "none observed">; EventErrorHint: <yes/no/pending-extractor>
      //   event_stream = [<flags>] — extractor: <registered in <S-ref> | NOT REGISTERED: <reason>>;
      //   heartbeat/sparse note: <the human judgment from the gap analysis>
  - VALIDATE: visual (the t.Logf fires only on a real run).

Task 7: VERIFY — the full gate set (see Validation Loop)
```

### Implementation Patterns & Key Details

```go
// PATTERN (Phase A's live capture — the harness core; Execute cannot timestamp):
func probeRun(t *testing.T, name string, flags []string, model, payload string) eventCapture {
	t.Helper()
	m := provider.BuiltinManifests()[name] // fresh map per call — the struct is ours
	m.EventStream = flags                  // the TRIAL: test-local, NEVER written back (FR-S2)
	provider.RegisterEventExtractor(name, probeExtractor{}) // S1's render guard; one-shot process
	spec, err := m.Render(model, "", payload, "off")
	if err != nil { t.Fatalf("render: %v", err) }
	if !spec.ProgressiveStdout { t.Fatalf("ProgressiveStdout not armed — render guard or flags wrong") }

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	cmd.Env, cmd.Dir = spec.Env, t.TempDir()
	cmd.Stdin = strings.NewReader(spec.Stdin)
	out, err1 := cmd.StdoutPipe()
	eb, err2 := cmd.StderrPipe()
	if err1 != nil || err2 != nil { t.Fatalf("pipes: %v %v", err1, err2) }
	var cap eventCapture
	t0 := time.Now()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); sc := bufio.NewScanner(out); sc.Buffer(make([]byte, 1024*1024), 1024*1024)
		for sc.Scan() { cap.lines = append(cap.lines, capturedLine{time.Since(t0), sc.Text()}) } }()
	wg.Add(1)
	go func() { defer wg.Done(); b, _ := io.ReadAll(eb); cap.stderr = string(b) }()
	if err := cmd.Start(); err != nil { t.Fatalf("start %s: %v", spec.Command, err) }
	err = cmd.Wait(); wg.Wait()
	cap.exitAt = time.Since(t0)
	if ee, ok := err.(*exec.ExitError); ok { cap.exitCode = ee.ExitCode() } else if err != nil { t.Fatalf("wait: %v", err) }
	return cap
}

// PATTERN (the progressiveness check — the exit-burst signature is the FAIL):
first, exit := cap.lines[0].offset, cap.exitAt
n := countNonBlank(cap.lines)
if n < 2 || exit-first < minSpan {
	t.Fatalf("NOT progressive (FR-S2 proof i): %d lines, first @%v, exit @%v (span %v < %v). "+
		"Exit-burst = the buffered `json` mode, not a stream (§9.30 naming trap). "+
		"Offset table:\n%s", n, first, exit, exit-first, minSpan, offsetTable(cap))
}

// PATTERN (the ledger guard — both directions + empty-date):
if declared && !recorded { t.Errorf("%s declares event_stream but the FR-S2 ledger has no dated "+ ...) }
if recorded && !declared { t.Errorf("%s ledger entry (%s) without a declaration — stale", name, date) }
```

### Integration Points

```yaml
CONSUMES (READ-ONLY):
  - S2's exports: ParseEventStream / ParseOutput / EventErrorHint / EventOutcome / EventIgnored
    (Phase B) — compile against the LANDED events.go at impl.
  - S1's registry seams: RegisterEventExtractor (probe + trial) / LookupEventExtractor (Phase B).
  - render.go: CmdSpec + Render (the trial manifest → spawn spec; ProgressiveStdout arming assert).
  - realagent_test.go's same-package tagged helpers: envOr, realDefaults, realConfig (model plumbing).
PRODUCES (consumed by P2.M2.T1.S2–S5):
  - The RUNBOOK (the harness file header) — the protocol each provider task executes.
  - The RECORDING TEMPLATE + t.Logf emitter — the record pasted into builtin.go (§7.3 pattern).
  - The LEDGER (events_ledger_test.go) — the population seam S2–S5 extend with dated entries.
  - The candidate flag table — the provisional §9.30 starting point each task verifies/adjusts.
NO production-code change. NO registry change (stays EMPTY). NO builtin/providers/docs changes.
```

## Validation Loop

> **This is a test-infrastructure item.** The template's ruff/mypy/pytest gates do not apply. The
> real-CLI run is S2–S5's job — THIS task validates that the harness compiles, gates correctly
> (skips without the env), the ledger guard is green on the empty state, and nothing regressed.

### Level 1: Build & static (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach
go build ./...                                        # the always-on ledger test compiles
go vet ./internal/provider/...
go vet -tags integration_real ./internal/generate/    # the tagged harness compiles (its only green light w/o real CLIs)
gofmt -l internal/generate/events_verify_test.go internal/provider/events_ledger_test.go   # must print nothing
# Expected: all clean, exit 0.
```

### Level 2: The gates behave (gating + ledger)

```bash
cd /home/dustin/projects/stagecoach
# (a) The ledger guard is green on the empty state (the FR-S2 invariant TODAY):
go test ./internal/provider/ -run TestEventStreamLedger -v
# Expected: PASS (no built-in declares event_stream; ledger empty).

# (b) The harness SKIPS (not FAILs) without the real-agent env, even when the tag is supplied:
go test -tags integration_real ./internal/generate/ -run TestEventStreamVerify -v
# Expected: SKIP per subtest (or the parent SKIP), exit 0 — NEVER a compile error or a FAIL.

# (c) The FR-S2 ledger guard actually FIRES on a violation (self-test: temporarily add
#     EventStream to a built-in in a scratch edit — or trust the (b)-style logic review):
go test ./internal/provider/ -run TestEventStreamLedger   # after REVERTING any scratch edit: PASS
# Expected: logic verified (declared⇒recorded, recorded⇒declared, empty-date) — review or scratch-test, then revert.
```

### Level 3: No regressions (existing suites)

```bash
cd /home/dustin/projects/stagecoach
go test ./internal/provider/... ./internal/generate/... -race
# Expected: GREEN — the two new files add no always-on behavior beyond the ledger assert; S1's
#           render/registry tests and S2's framework tests (if landed) untouched.
go test ./... -race    # whole-repo belt-and-braces
# Expected: GREEN.
```

### Level 4: FR-S2 discipline + scope guard

```bash
cd /home/dustin/projects/stagecoach
rg -n 'event_stream' providers/ && echo "FAIL: a built-in declares event_stream (FR-S2)" || echo "OK: none"
# Expected: "OK: none" (this task populates nothing).
rg -n 'RegisterEventExtractor\(' internal/provider/builtin.go internal/provider/render.go internal/provider/events.go && echo "check: only the map literal + seam defs (no real registration)" || echo "OK: no registration sites"
# Expected: no NEW registration call sites; the eventExtractors map literal remains the only declaration.
rg -n 'STAGECOACH_RUN_REAL' internal/generate/events_verify_test.go   # the gate present
rg -n 'integration_real' internal/generate/events_verify_test.go      # the tag present (line 1)
test ! -f internal/provider/events_ledger_test.go.tag 2>/dev/null; head -1 internal/provider/events_ledger_test.go | grep -q 'package provider' && echo "OK: ledger untagged (always-on)"
git status --porcelain
# Expected EXACTLY: ?? internal/generate/events_verify_test.go  ?? internal/provider/events_ledger_test.go (nothing else).
git status --porcelain | grep -vE '^\?\? internal/generate/events_verify_test\.go$|^\?\? internal/provider/events_ledger_test\.go$' | grep -E '.' && echo "FAIL: out-of-scope change" || echo "OK: scope clean"
# Expected: "OK: scope clean".
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1: build/vet/gofmt clean (incl. `go vet -tags integration_real ./internal/generate/`)
- [ ] Level 2a: `TestEventStreamLedger` GREEN on the empty state
- [ ] Level 2b: the harness SKIPs cleanly without `STAGECOACH_RUN_REAL=1` under the tag
- [ ] Level 3: `go test ./internal/provider/... ./internal/generate/... -race` (+ `./...`) GREEN
- [ ] Level 4: no population, scope guard clean

### Feature Validation
- [ ] The runbook header: manual command, the three proofs + criteria, the recording template, the
      paste destination, the heartbeat/sparse judgment, the red→green loop
- [ ] Per-provider subtests (pi/claude/cursor/codex/opencode — NO agy), individually runnable via `-run`
- [ ] The candidate table seeded from §9.30 + `STAGECOACH_VERIFY_EVENT_FLAGS_<NAME>` override
- [ ] Phase A: probe registration (S1's guard), trial manifest copy, `ProgressiveStdout` assert,
      direct spawn with per-line monotonic timestamps, verbatim dump, gap analysis, exit-burst FAIL
- [ ] Phase B: real-extractor trial (ParseEventStream + ParseOutput sentinel assert + EventErrorHint);
      SKIP with pointer when unregistered
- [ ] Error probe: forced invalid model, exit code recorded as EVIDENCE (no exit-code assert)
- [ ] The t.Logf recording-template emitter (date auto-filled)
- [ ] The ledger guard: declared⇒recorded, recorded⇒declared, empty-date — failure text names the protocol

### Scope-Boundary Validation
- [ ] `git status --porcelain` shows ONLY the two new files
- [ ] events.go / parse.go / executor.go / render.go / builtin.go / providers/*.toml UNTOUCHED
- [ ] S2's files (events_test.go, generate_eventstream_test.go, docs/providers.md) UNTOUCHED
- [ ] The eventExtractors registry ships EMPTY; no real extractor registered anywhere
- [ ] realagent_test.go UNTOUCHED (reused via same-package symbols, never edited)
- [ ] spec/ / PRD / plan / tasks.json untouched

### Documentation & Deployment
- [ ] The protocol is self-documenting (the harness header IS the runbook — no separate docs file,
      per the item's DOCS: "none beyond the protocol itself")
- [ ] The ledger file's header documents the FR-S2 rule + the pointer to the runbook

---

## Anti-Patterns to Avoid

- ❌ Don't verify ANY provider or populate ANYTHING in this task — S2–S5 own per-provider
  verification + population; the registry ships EMPTY and the ledger ships EMPTY (FR-S2).
- ❌ Don't use provider.Execute for the timestamped capture — it returns BUFFERED stdout; proof (i)
  needs per-line arrival offsets. Spawn the rendered CmdSpec directly; Phase B exercises the real
  parse path on the captured stream.
- ❌ Don't skip the probe-extractor registration before Render — S1's render guard REFUSES
  event_stream for an unregistered provider; Phase A cannot even render without it.
- ❌ Don't write the trial flags back into builtin.go/BuiltinManifests (or anywhere shipped) — the
  trial manifest is a TEST-LOCAL copy; shipped manifests stay event_stream-free until S2–S5's
  dated verification. That separation IS the FR-S2 discipline.
- ❌ Don't assert a nonzero exit in the error probe — FR-S3's premise is that failures do NOT
  reliably exit nonzero; the exit code is recorded evidence, not an assertion target.
- ❌ Don't make Phase B a FAIL when no extractor is registered — the red state is a SKIP with a
  pointer to Phase A (the red→green loop must be navigable, not a wall).
- ❌ Don't include agy in the subtest list — realagent's established exclusion (experimental, not
  real-tested); S3 extends the table deliberately if it verifies agy.
- ❌ Don't conflate AGENT and inference provider — reuse realConfig exactly (pi needs
  {"claude-haiku","anthropic"}, opencode needs a model); never put inference into cfg.Provider.
- ❌ Don't tag the ledger file — it must be ALWAYS-ON (it guards population); only the harness
  carries //go:build integration_real.
- ❌ Don't touch realagent_test.go to "share" helpers — the tagged same-package files already share
  symbols; editing the precedent file is out of scope.
- ❌ Don't use wall-clock time.Now() strings across goroutines for offsets — one t0, time.Since(t0),
  stamped in the reader goroutine only (monotonic).
- ❌ Don't put the runbook in a separate docs file — the item says DOCS: none beyond the protocol
  itself; providers.md's capability section is S2's and already landing.
- ❌ Don't hard-code the min-span at a value that can't be tuned — fast/slow models vary; make it
  env-overridable (STAGECOACH_VERIFY_MIN_SPAN_MS) with a 500ms default.

---

## Confidence Score

**One-pass success likelihood: 9/10.** The harness is test-only (no production risk), every API it
calls is verified against the live code (CmdSpec/Render signatures at render.go:32/:106, Execute's
buffered contract at executor.go:104, the registry seams in events.go, realagent's same-package
helpers at realagent_test.go:36–90), the two precedents it mirrors are read in full (the §7.3
VERIFIED-comment pattern; the integration_real + STAGECOACH_RUN_REAL gating), and the file set has
zero overlap with S2's in-flight deliverables. The 1-point deduction: the harness compiles against
S2's exported names (ParseEventStream/EventErrorHint/EventOutcome), which are contractual but still
landing — if S2's signatures shift at merge, the implementer must follow the landed events.go (an
explicit, one-read adaptation, called out in the gotchas). The real-CLI execution itself is
S2–S5's task, so this deliverable is fully validatable without any real agent installed.
