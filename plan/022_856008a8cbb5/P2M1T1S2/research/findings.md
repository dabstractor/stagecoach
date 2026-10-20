# Research Findings — P2.M1.T1.S2: event-mode execution (NDJSON parse, Extractor, FR-S3)

## §0 What S1 (P2.M1.T1.S1) delivers — the contract S2 builds on

- `manifest.EventStream []string` (nil=off), regime-2 merge, render insertion (both Render/RenderMultiTurn,
  before print_flag) + **`CmdSpec.ProgressiveStdout = len(r.EventStream) > 0`** (the arming flag).
- **`internal/provider/events.go` (NEW file, from S1)**: `Extractor` interface + unexported `eventExtractors`
  map (**EMPTY** — FR-S2) + `LookupEventExtractor(name)` / `RegisterEventExtractor(name, e)`.
  S1's interface is an explicit PLACEHOLDER: `Extract(line string) (message string, terminal bool, err error)`;
  S1's PRP anticipates S2 refining the signature ("a signature refinement in S2 is cheap but nonzero churn") —
  only the registry + guard are the stable contract. S1's render_test.go has a `fakeExtractor` that S2 must
  update if the signature changes.
- Render HARD-errors when EventStream is set for a provider with no registered extractor. No built-in
  declares event_stream; `providers/*.toml` clean.

## §1 The verified FR-S spec (§9.30, prd_snapshot:659+)

- **FR-S1**: event modes are NDJSON-ish: pi `--mode json`; claude/cursor/agy `--output-format stream-json`
  (+ claude `--include-partial-messages`, cursor `--stream-partial-output`); codex `--json` (JSONL);
  opencode `--format json`. **Naming trap: claude/cursor/agy offer BOTH `json` (single buffered object) and
  `stream-json` (progressive) — only stream-json qualifies.** Final message assembled from the TERMINAL
  message event (not raw stdout); stdout becomes an activity channel on every platform.
- **FR-S2**: never speculative; requires a DATED empirical run proving (i) progressive emission DURING
  generation, (ii) the terminal-event shape, (iii) how in-stream errors surface. Pitfalls (verbatim for docs):
  heartbeat-only streams (codex: steady timer events while network dead — liveness not progress) and sparse
  emitters (opencode: one mid-run event + answer burst, ~3s stdout gaps — fine for 60s idle, fatal for a
  naive sub-second one).
- **FR-S3**: a failed/aborted response appears in the event stream and does NOT reliably produce a nonzero
  exit (pi documents this explicitly) — detection MUST key on the event stream, not the exit code. Surface
  the in-stream error text through the existing empty-output cause line (the StderrHint path).
- **FR-S4**: print-mode providers remain fully supported (migration is per-provider, gated on verification).

## §2 The parse/execute landscape (verified live)

- **parse.go `ParseOutput(raw, m) (msg, ok, fellback)`** — the 5-step §12.9 pipeline (trim → fence-strip →
  raw/json switch → normalize → trim+ok). Empty output ⇒ `ok==false` (NO error; caller retries per FR29).
  Called by **7 sites**: generate.go:397, decompose/message.go:198, generate/multiturn.go:201,
  generate/workdesc.go:120+136, hook/exec.go:199, pkg/stagecoach/stagecoach.go:594.
- **`StderrHint(stdout, stderr) string` lives in executor.go:23–38** (NOT parse.go). Returns "" when stdout
  has non-whitespace content — so in event mode (stdout = NDJSON events) it yields "" and the in-stream
  error is NEVER surfaced: exactly the opaque failure FR-S3 replaces. Consulted at exactly **3 sites**:
  generate.go:403, decompose/message.go:204, decompose/planner.go:143, always as
  `if h := provider.StderrHint(out, stderr); h != "" { lastCause = errors.New(h) }` inside the `!ok`/
  parse-failed block (lastCause → RescueError.Cause → verbose "DEBUG: cause:" line).
- **planner.go:139 parses via `prompt.ParsePlannerOutput(out)`** — NOT ParseOutput (the planner partition is
  its own JSON). planner.go:143's StderrHint block guards THAT parse. The arbiter likewise (only
  message.go in decompose calls ParseOutput). ⇒ non-ParseOutput parse sites need an event-unwrap step.
- **executor.go `Execute(ctx, spec, dl, vb) (stdout, stderr string, err error)`**: captures via countingWriters;
  D4 arming at :144 `if dl.Idle > 0 && (linux || spec.ProgressiveStdout)`; ErrIdleDeadline/deadline/cancel/
  ExitError contract. A failed response CAN return err==nil (exit 0) — FR-S3's premise.
- **countingWriter** (activity.go): pipe-write LastWrite stamps = the portable activity channel
  (ProgressiveStdout arms it on every platform).

## §3 Design decisions (locked)

1. **Extractor refinement** (S2's right, anticipated by S1): replace the 3-return placeholder with
   `Extract(line string) EventOutcome`; `EventKind ∈ {EventIgnored, EventTerminal, EventError}`;
   `EventOutcome{Kind, Message, ErrText}`. **No error return** — unknown/malformed lines classify as
   EventIgnored (lenient, mirroring ParseOutput's fallback philosophy; extractors json.Unmarshal their own
   shape and ignore what doesn't parse). Update S1's render_test.go fakeExtractor (the anticipated churn).
2. **Stateless per-line classification; the terminal event is SELF-CONTAINED** (carries the full assembled
   message) — matches FR-S2's "record the terminal-event shape the message is assembled from" (singular).
   A stateful accumulator would be a per-invocation object and the registry holds singletons shared by
   concurrent roles (decompose runs roles concurrently) — a data race. If P2.M2's verification finds a
   delta-accumulation shape, the interface grows THEN, at the registry seam.
3. **Framework scan `ParseEventStream(raw, m) (msg, ok, errText string)`** in events.go: split lines
   (CRLF-tolerant), skip blanks, classify each via LookupEventExtractor(m.Name); **LAST terminal wins;
   FIRST error recorded; terminal-wins-over-error** (a produced message is a success; FR-S3 fires only when
   NO terminal message event). Defensive: no registered extractor ⇒ (msg=="", ok==false) — never a panic
   (Render's guard makes it unreachable).
4. **ParseOutput step-0 branch** (events: `if len(r.EventStream) > 0 { s = assembled }` right after Resolve
   + trim, BEFORE the fence-strip): the assembled message then flows through the EXISTING steps 2/4/5
   (fence-strip correctly applies to a fenced message inside an event; normalize+trim apply). fellback stays
   false in event mode. Print mode (len==0) is BYTE-IDENTICAL — satisfies contract (c). All 7 ParseOutput
   sites get event parsing for free. Header comment documents the new step-0.
5. **`UnwrapEventOutput(out, m) string`** — one-line helper (event mode ⇒ assembled terminal message, else
   raw unchanged) for the two non-ParseOutput parse sites: planner.go (insert before
   `prompt.ParsePlannerOutput(out)`) and the arbiter's parse site (locate by grep at impl — likely
   internal/decompose/arbiter.go or the arbiter branch of message.go).
6. **`EventErrorHint(raw, m) string`** — the FR-S3 "precedence sibling" of StderrHint: returns "" unless
   (m declares event_stream) AND (no terminal message) AND (an error event exists); else a single-line,
   newline-collapsed, 1000-char-capped hint "agent event stream reported an error: <text>" (mirrors
   StderrHint's formatting). Inserted BEFORE StderrHint at the 3 sites — the in-stream error is more
   specific than stderr, and StderrHint returns "" in event mode anyway (stdout non-empty).

## §4 Test infrastructure (verified)

- **internal/stubtest**: `Build(t)` compiles ./cmd/stubagent once (CWD-independent); `Options{Out, Exit,
  SleepMS, Stderr, Script, EmitMS, EmitLine, EmitTotalMS, HeartbeatMS, ...}` → STAGECOACH_STUB_* env;
  `Manifest(...)`/`Env(...)` build the stub provider manifest. **stubtest imports internal/provider ⇒
  same-package provider tests CANNOT import stubtest (cycle)** — e2e tests live in internal/generate
  (external, has stubtest + the idle_parity precedent).
- **stubagent progressive-emit** (cmd/stubagent/main.go 1d): `STAGECOACH_STUB_EMIT_MS` writes
  `STAGECOACH_STUB_EMIT_LINE` every interval for `STAGECOACH_STUB_EMIT_TOTAL_MS` (0 ⇒ forever, never
  answer; >0 ⇒ bounded, then the answer = stdout SUFFIX via `STAGECOACH_STUB_OUT`). Exactly the JSONL
  persona needed: EmitLine = `{"type":"progress","n":1}` ticks, Out = the terminal event line.
  Default exit 0 (FR-S3's "no nonzero exit" case). Cadence guidance: tick ≪ the watcher's ~500ms poll
  (use 50ms).
- Watchdog tick is 500ms (executor.go watchIdle) — idle windows must exceed several ticks.

## §5 e2e test recipe (contract's three rides)

1. **Happy + portable idle payoff**: stub manifest (Command=stubagent, EventStream=["--mode","json"]-ish
   tokens — the fake extractor is keyed on the manifest NAME, tokens are opaque to the executor) +
   `RegisterEventExtractor(name, fake)` + t.Cleanup delete; EmitMS=50, EmitLine=`{"type":"progress","n":1}`,
   EmitTotalMS=2500, Out=`{"type":"result","message":"feat: add widget"}`; Render ⇒ assert
   spec.ProgressiveStdout && tokens precede print_flag; Execute(Deadlines{Idle:1s, Hard:30s}) ⇒ err==nil
   (2.5s of ticks > 1s idle — survives ONLY because each tick stamps the pipe channel; on macOS/Windows
   this works solely because ProgressiveStdout armed the pipe channel — FR-A3's portable payoff);
   ParseOutput ⇒ ("feat: add widget", true).
2. **Error event, exit 0**: Out=`{"type":"error","message":"usage limit exceeded"}` ⇒ ParseOutput ok==false
   (existing FR29 retry path); EventErrorHint carries "usage limit exceeded"; through the generate loop the
   RescueError.Cause is the event text (not the opaque empty-parse).
3. **Unregistered guard**: registry empty for the manifest name ⇒ ParseEventStream returns ok==false
   gracefully; S1's render guard tests stay green (no regression).

## §6 Mode A docs — docs/providers.md

S1 already landed the table row (verified: the `event_stream` row + "23 fields" are present). The
precedent section `### Multi-turn capability (session_mode)` sits right after the manifest table (~L40–49),
before `## Command rendering`. S2 adds `### Event-stream capability (event_stream)` after it, mirroring
its shape: what the field declares, the terminal-event assembly model, the FR-S2 verification bar (dated
empirical run; the three proof points), the **json-vs-stream-json naming trap**, and the heartbeat/sparse
pitfalls (codex heartbeat-only; opencode ~3s gaps) — the pitfalls VERBATIM from §9.30/FR-S2.
