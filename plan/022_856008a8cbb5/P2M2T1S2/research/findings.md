# P2.M2.T1.S2 — Research findings (pi `--mode json` verification + disposition)

Captured 2026-07-2x at HEAD `45c57a3` ("add event_stream manifest + render + arming" — P2.M1.T1.S1
landed; P2.M1.T1.S2's events.go framework landed). S1's harness
(`events_verify_test.go` + `events_ledger_test.go`) is in flight — the CONTRACT.

## 0. The machine-local ground truth: pi IS installed

- `which pi` → `/home/dustin/.local/bin/pi`; `pi --version` → **1.0.2**.
- `pi --help` → `--mode <mode>   Output mode: text (default), json, or rpc` — the `--mode json`
  flag FR-S1 names exists on this build.
- pi's canonical JSON-mode doc: `/home/dustin/.local/lib/node_modules/@earendil-works/pi-coding-agent/docs/json.md`
  (226 lines) + `docs/message-types.md` (shared message shapes) + `docs/cli.md` (`--print`/`--mode`).

So the "absent CLI" disposition branch is MOOT on this machine — but the unauthenticated/failed-run
branch remains possible; the PRP keeps it.

## 1. pi `--mode json` documented event shapes (docs/json.md — the extractor's design input)

First record is a session header, then session events, exit after prompts finish:

```
{"type":"session","version":3,"id":"uuid","timestamp":"...","cwd":"/path"}
{"type":"agent_start"}
{"type":"turn_start"}
{"type":"message_start","message":{"role":"user","content":"...","timestamp":...}}
{"type":"message_end","message":{"role":"user","content":"...","timestamp":...}}
{"type":"message_start","message":{"role":"assistant","content":[],"stopReason":"pending",...}}
{"type":"message_update","usage":{...},"assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Hello"}}
{"type":"message_end","message":{"role":"assistant",...}}          ← "authoritative final message"
{"type":"turn_end","message":{"role":"assistant",...},"toolResults":[]}
{"type":"agent_end","messages":[...],"willRetry":false}
{"type":"agent_settled"}
```

Key rows (docs/json.md tables):
- **`message_end`** — "A message completed. **This is the authoritative final message.**" → the
  TERMINAL event. Assistant `content` is a block array (text/thinking/tool_call blocks per
  message-types.md); text assembled by concatenating text-block `text`s. `message_update` deltas are
  delta-ONLY (never accumulate them — `message_end.message` replaces all partials: "Replace the whole
  partial message with `message_end.message` when it arrives").
- **`turn_end`** also carries the final assistant `message` (+ `toolResults`) — a secondary terminal
  candidate; prefer `message_end{role:"assistant"}` as primary, both agree on content.
- Error surfacing (FR-S3): provider-level `{type:"error",reason,error}` events exist at the wire level
  but the normal agent loop translates provider `start/done/error` into `message_start`/`message_end`;
  retry/compaction failures appear as `auto_retry_start/end` (`errorMessage`; final failure =
  `success:false` + `finalError`), `compaction_start/end` (`errorMessage`), `agent_end.willRetry`.
  The EMPIRICAL invalid-model capture decides which shape pi actually emits on a hard failure —
  the doc is design input, the dated capture is the authority.
- **Exit code**: json.md/cli.md document NO nonzero-exit-on-failure guarantee ("then exits after the
  supplied prompts finish") — consistent with FR-S3's premise (pi documents that a failed response
  does NOT reliably produce nonzero exit). Exit code is recorded as EVIDENCE, never asserted.

## 2. The landed framework this task populates (internal/provider/events.go — verified live)

- `EventKind`: `EventIgnored | EventTerminal | EventError`; `EventOutcome{Kind, Message, ErrText}`;
  `Extractor` interface: `Extract(line string) EventOutcome` — STATELESS per line (registry entries
  are singletons shared by concurrent roles — no accumulator state).
- `var eventExtractors = map[string]Extractor{}` — EMPTY; header comment: "P2.M2 populates via
  RegisterEventExtractor … called from provider construction/registration code AFTER dated
  verification, never speculatively."
- `ParseEventStream(raw, m) (msg, ok, errText)`: CRLF-tolerant line scan via the registered
  extractor; **LAST terminal wins; FIRST error recorded; a terminal message BEATS an error event**
  (FR-S3 fires only when NO terminal exists). No extractor ⇒ ("",false,"").
- `EventErrorHint(raw, m)` — FR-S3 sibling of StderrHint; `UnwrapEventOutput(out, m)`.
- **parse.go:59 already wires it**: ParseOutput tries `ParseEventStream` first — no parse changes
  needed in this task.
- Render (S1, landed): EventStream tokens inserted into the render; `spec.ProgressiveStdout` arms
  iff manifest declares EventStream AND a registered extractor exists (the FR-S2 guard).

## 3. The pi manifest today (builtin.go:49 `builtinPi()`, verified live)

No `EventStream` field. Relevant neighbors: `PrintFlag: strPtr("-p")`, `PromptDelivery: stdin`,
`SystemPromptFlag: --system-prompt`, `ProviderFlag: --provider` (multi-backend — FR-R5b: model must
be `inference/model`, e.g. `anthropic/claude-haiku`), `ReasoningLevels --thinking`,
`Output: raw`, `StripCodeFence: true`. Doc comment at :29 already carries the §7.3 dated-VERIFIED
style (`// builtinPi returns the pi manifest per PRD §12.3 (FULLY VERIFIED vs pi --help,
external_deps.md §pi).`) — the event-stream record extends it (S1's recording template).

## 4. S1's harness contract (what this task RUNS; from P2M2T1S1/PRP.md — the authority)

- Manual command: `STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/
  -run TestEventStreamVerify/pi -v -timeout 10m`.
- Env knobs: `STAGECOACH_VERIFY_EVENT_FLAGS_PI` (space-separated token override; table seeds
  `--mode json`), `STAGECOACH_VERIFY_MIN_SPAN_MS` (default 500ms).
- Three proofs: (i) progressive — ≥2 non-blank lines at distinct offsets AND exit−first ≥ minSpan
  (exit-burst FAILS = the json-vs-stream-json trap; N/A-ish for pi — only ONE json mode — but the
  mechanical check still decides); (ii) terminal-event shape — verbatim dump → becomes the
  Extractor; (iii) error probe — model forced `stagecoach-verify-invalid-model`, exit code recorded
  as EVIDENCE (no exit assertion — FR-S3 premise), error-event shape recorded verbatim.
- Gap analysis (max inter-line gap) feeds the heartbeat/sparse JUDGMENT the human writes into the
  record. For pi the doc implies dense `message_update` deltas during generation (good) — the
  empirical gaps decide.
- Recording template emitted via t.Logf → paste into builtinPi()'s doc comment; ledger entry added
  to `eventStreamLedger` in `internal/provider/events_ledger_test.go` (S1's file — this task EDITS
  the ledger map, the one sanctioned touch).
- Phase B (red→green): after the real extractor is registered in production, re-run — assert
  ParseEventStream assembles the sentinel ("stagecoach-event-verify"), EventErrorHint non-empty on
  the error capture.
- Model plumbing: realagent_test.go:40 `realDefaults["pi"] = {"claude-haiku", "anthropic"}` (model +
  inference provider — the AGENT-vs-inference distinction, :28–33); harness reuses realConfig.

## 5. Where the production registration goes

events.go's contract: "called from provider construction/registration code AFTER dated verification."
Cleanest per-provider seam (mirrors what S3–S5 will each add): NEW
`internal/provider/events_pi.go`, package provider, `func init() { RegisterEventExtractor("pi",
piEventExtractor{}) }` + the stateless classifier + its dated doc comment. Never a package-level
mutable var beyond the registry itself.

## 6. Disposition branches (the contract's LOGIC)

- **PASS** (all three proofs): register extractor (events_pi.go) + `EventStream: []string{"--mode",
  "json"}` in builtinPi() + dated VERIFIED record in the ctor doc comment + ledger entry `"pi":
  "<date>"` + re-run Phase B green + fixture unit tests from the verbatim captures.
- **FAIL** (e.g. exit-burst → pi's json is buffered on this build/usage) or **ABSENT/UNAUTHED**:
  EventStream stays absent; the dated NEGATIVE record goes in the ctor doc comment (what was run,
  what failed, why); ledger stays without pi; NO extractor. Both are valid dispositions — FR-S2
  forbids speculation either way.
- pi being installed+authed makes PASS the likely branch, but the empirical run decides, not the doc.

## 7. Scope guards

- Files this task may touch: `internal/provider/events_pi.go` (new), `internal/provider/
  events_pi_test.go` (new), `internal/provider/builtin.go` (builtinPi doc comment + EventStream
  field ONLY), `internal/provider/events_ledger_test.go` (the ledger map entry ONLY — S1's file).
- NOT touched: events.go, parse.go, render.go, executor.go, realagent_test.go,
  events_verify_test.go, providers/*.toml (unless parity convention demands — check: built-ins are
  Go structs; providers/*.toml are reference docs; pi.toml parity comment may warrant the same
  record — check providers/pi.toml's existing VERIFIED convention and mirror the record there too,
  as the other manifests do), docs/ (Mode A: the dated record IS the doc), PRD, tasks.json.
- S3–S5 own claude/cursor/agy, codex, opencode — do not touch their manifests/extractors.
