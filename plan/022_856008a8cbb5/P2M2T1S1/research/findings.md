# P2.M2.T1.S1 — Research Findings (event-stream verification harness + recording protocol)

## 0. What S2 (P2.M1.T1.S2, in flight — the CONTRACT) lands; this task consumes it

S2's landed surface (treat as exact; follow S2's exported names if merge churn occurs):
- `internal/provider/events.go`: `EventKind` (`EventIgnored`/`EventTerminal`/`EventError`),
  `EventOutcome{Kind, Message, ErrText}`, `Extractor interface{ Extract(line string) EventOutcome }`,
  `ParseEventStream(raw string, m Manifest) (msg string, ok bool, errText string)`,
  `UnwrapEventOutput(out string, m Manifest) string`,
  `EventErrorHint(raw string, m Manifest) string`.
  Registry seams UNCHANGED from S1: `LookupEventExtractor(name) Extractor`,
  `RegisterEventExtractor(name string, e Extractor)` — the `eventExtractors` map ships EMPTY (FR-S2).
- `parse.go` step-0 event branch (`len(r.EventStream) > 0` → assemble from terminal event);
  print mode byte-identical.
- 3 cause sites get `EventErrorHint` BEFORE `StderrHint`; 2 unwrap sites (planner + arbiter).
- S2's OWN files: `internal/provider/events_test.go`, `internal/generate/generate_eventstream_test.go`
  (stubtest e2e), `docs/providers.md` capability section. **THIS TASK MUST NOT TOUCH THOSE FILES** —
  my deliverables are two NEW files (see §5).

S2's Integration Points name THIS task explicitly: "P2.M2.T1.S1 (verification harness): drives
Render→Execute→ParseOutput per provider and records the dated evidence; uses RegisterEventExtractor to
trial candidate extractors."

## 1. The dated-verification comment precedent (provider_subsystem.md §7.3 + builtin.go)

The pattern is an inline `// VERIFIED <YYYY-MM-DD> via <exact probe> ; <FR ref>` on the field/block,
plus a dated paragraph in the constructor doc comment. Live examples:
- `SessionMode: strPtr("append"), // VERIFIED 2026-07-05 via \`pi --session-id X ... -p "remember BANANA"\` then recall returns BANANA; FR-T9.` (builtin.go:61)
- `ListModelsCommand ... // VERIFIED 2026-07-03 via \`pi --list-models\` (exit 0); FR-L2/FR-D5.` (builtin.go:54)
- Tooled flags: "VERIFIED 2026-07-09 (agy v1.1.11) end-to-end: stdin task + this combo stages exactly..."

§7.3 states verbatim: "A new per-provider capability claim (e.g. 'emits JSONL events on stdout with
flag X') must carry this same dated, probe-named verification." THIS TASK defines that protocol for
the event-stream claim; S2–S5 execute it per provider and paste the record.

## 2. The real-agent CI-gating precedent (internal/generate/realagent_test.go)

- Build tag: `//go:build integration_real` (file line 1) — NOT built by `make test`/`make coverage`
  (no -tags) ⇒ never required in CI. Runtime gate: `STAGECOACH_RUN_REAL=1`, else t.Skip.
- Manual command (documented in its header):
  `STAGECOACH_RUN_REAL=1 go test -tags integration_real ./internal/generate/ -run TestRealAgents -v -timeout 30m`
- Same-package helpers MY harness reuses (tagged files in package generate share symbols):
  `envOr(key, def)` (:59), `realDefaults map[string]realDefault{model,inference}` (:42 — pi needs
  `{"claude-haiku","anthropic"}`, opencode `{"anthropic/claude-sonnet-4",""}`, claude/codex/cursor
  default), env overrides `STAGECOACH_REAL_MODEL_<NAME>` / `STAGECOACH_REAL_INFERENCE_<NAME>`,
  `realConfig(name) config.Config` (:63 — sets cfg.Provider=AGENT, model, inference into the
  manifest's DefaultProvider path). NOTE the AGENT-vs-inference terminology comment (:28–33).
- agy is EXCLUDED from realagent's list (experimental) — mirror that exclusion here.

## 3. The API the harness drives (verified live)

- `type CmdSpec struct { Command string; Args []string; Stdin string; PayloadBytes int; Env []string;
  ProgressiveStdout bool }` (render.go:32–45). ProgressiveStdout is armed by Render iff the manifest
  declares event_stream tokens + a registered extractor exists (S1's render guard).
- `func (m Manifest) Render(model, sysPrompt, userPayload, reasoning string, mode ...RenderMode)
  (*CmdSpec, error)` (render.go:106) — Validate→Resolve→token assembly; exported; safe on a COPY.
- `func Execute(ctx context.Context, spec CmdSpec, dl Deadlines, vb *ui.Verbose) (stdout, stderr,
  err string...)` (executor.go:104); `Deadlines{Idle, Hard time.Duration}` (:41).
- **Execute BUFFERS stdout — it cannot provide per-line arrival timestamps.** Proof (i) (progressive
  emission DURING generation) requires LIVE line-stamped capture ⇒ the harness spawns the rendered
  process DIRECTLY (`exec.CommandContext(spec.Command, spec.Args...)`, `cmd.Env = spec.Env`,
  `cmd.Stdin = strings.NewReader(spec.Stdin)`, `cmd.Dir = repo`, StdoutPipe → reader goroutine
  stamping `time.Since(t0)` per line). This deliberately bypasses Execute for Phase A capture; Phase
  B re-drives assembly through ParseEventStream/ParseOutput on the captured raw stream (and S2's
  stubtest e2e already proves the Execute path mechanically).

## 4. The FR-S2 render guard forces probe registration BEFORE Render

S1 landed: Render REFUSES event_stream for a provider with no registered extractor. So Phase A
(trial flags on a manifest copy) must first `RegisterEventExtractor(name, probeExtractor{})` where
probeExtractor returns `{Kind: EventIgnored}` for every line (satisfies the guard; classifies
nothing — Phase A is capture, not assembly). There is NO Unregister API — acceptable: the manual
run is a one-shot process (the map dies with it), and realagent tests don't consult extractors.
Phase B consults `LookupEventExtractor(name)`; when S2–S5 register the REAL extractor (production
code), Phase B turns green — the red→green loop.

## 5. Deliverables (two NEW files; zero overlap with S2's file set)

1. `internal/generate/events_verify_test.go` — `//go:build integration_real`, package generate.
   The file-header doc comment IS the runbook (protocol + 3 proofs + manual command + recording
   template). Harness: `TestEventStreamVerify` (STAGECOACH_RUN_REAL=1-gated) with per-provider
   subtests pi/claude/cursor/codex/opencode (agy excluded, mirroring realagent); candidate
   event-mode flag table seeded from PRD §9.30 (pi `--mode json`; claude/cursor/agy
   `--output-format stream-json`; codex `--json`; opencode `--format json`), env-overridable
   `STAGECOACH_VERIFY_EVENT_FLAGS_<NAME>` (space-separated). Phases A (empirical capture) and B
   (extractor trial); error probe via forced invalid model; emits the filled recording template
   via t.Logf.
2. `internal/provider/events_ledger_test.go` — ALWAYS-ON (no tag), package provider. The FR-S2
   enforcement ledger: `eventStreamLedger map[string]string{}` (provider → VERIFIED date, EMPTY
   now); `TestEventStreamLedger` asserts every built-in declaring EventStream has a dated ledger
   entry and vice versa; failure text points at this task's protocol. S2–S5 add ledger entries
   ONLY with a passing dated run — the mechanical form of "populating event_stream without a
   passing dated record is FORBIDDEN."

## 6. The three proofs — mechanical criteria (what the harness reports/asserts)

- **(i) Progressive emission (the FR-S2 core test):** per-line arrival offsets from spawn; exit
  offset; PASS iff ≥2 non-blank lines at distinct offsets AND (exitOffset − firstLineOffset) ≥ 500ms
  (env-tunable `STAGECOACH_VERIFY_MIN_SPAN_MS`). The clear FAIL signature: every line arrives in one
  burst at exit (the `json`-not-`stream-json` trap, PRD §9.30 naming trap).
- **(ii) Terminal-event shape:** verbatim dump of every captured line; the human identifies the
  terminal event (its exact JSON becomes S2–S5's Extractor). Phase B mechanically asserts
  ParseEventStream+ParseOutput assembles the expected message once the real extractor is registered.
- **(iii) Error/abort surfacing:** a second invocation with the model forced invalid
  (`stagecoach-verify-invalid-model`); record the stream verbatim, the EXIT CODE (FR-S3's premise:
  failures do NOT reliably exit nonzero — pi documents this), stderr, and whether EventErrorHint
  surfaces text (Phase B; Phase A records the raw shape for the human).

## 7. Heartbeat/sparse judgment (recorded in the template, decided by the HUMAN)

FR-S2 pitfalls the record must address: codex `--json` heartbeat-only streams (steady timer events
continue while the network is dead — liveness, not progress — idle-survival works but is NOT
progress evidence) and opencode `--format json` sparse emission (one mid-run event + answer burst;
fine for a 60s idle clock, fatal for a naive sub-second one). The template has a
`heartbeat/sparse note:` field; the harness REPORTS inter-line gaps (max gap, count by bucket) so
the human can fill it.

## 8. Scope boundaries

- NO built-in declares event_stream; NO real extractor registered; registry map stays EMPTY
  (S2–S5 own population). The ledger ships EMPTY and green.
- NO docs file (item DOCS: "none beyond the protocol itself" — providers.md's capability section is
  S2's, already landing).
- NO changes to events.go/parse.go/executor.go/render.go/builtin.go/providers/*.toml.
- Mock: none (the protocol is empirical by definition; stub-based framework tests are S2's).
