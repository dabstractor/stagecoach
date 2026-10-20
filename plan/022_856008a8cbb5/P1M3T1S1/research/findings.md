# Findings — P1.M3.T1.S1 (stubagent knobs: progressive-emit + chatty-heartbeat stub modes)

## 1. The authoritative recipe (architecture/call_sites_and_error_contract.md §9.6, verbatim constraints)

The report specifies the two missing knob families and their WIRING CONSTRAINTS:

**(b) Slow-but-progressively-emitting stub** (activity keeps resetting the idle clock; hard cap must
eventually kill; partial stdout must reach the caller): NOT expressible today — stdout is written
exactly once, immediately before exit. "Add a knob pair to cmd/stubagent, e.g. STAGECOACH_STUB_EMIT_MS
(write interval) + STAGECOACH_STUB_EMIT_BYTES/STAGECOACH_STUB_EMIT_LINE (chunk per tick), emitting ticks
to stdout AFTER the stdin drain/marker and BEFORE the final fmt.Fprint(out) + exit (optionally
STAGECOACH_STUB_EMIT_TOTAL_MS to bound the run and let a healthy path finish)." Wiring facts:
(1) provider.Execute captures stdout in a bytes.Buffer — ticks are invisible to ParseOutput until exit
but ARE the byte-activity an idle watcher observes, and on a deadline kill the PARTIAL buffer is
returned alongside DeadlineExceeded (executor.go:95-97), so a test can assert both the kill and the
partial-capture contract; (2) the stub must keep draining stdin first (deadlock guard) and keep writing
the marker before the first tick.

**(c) Chatty-forever heartbeat stub** (the false-alive pathology — codex-style periodic stderr timer;
idle must NOT fire, hard cap MUST): NOT expressible today (stderr is a single optional write). "Add
e.g. STAGECOACH_STUB_HEARTBEAT_MS — after the stdin drain + marker, loop fmt.Fprintln(os.Stderr,
'…tick…') every N ms forever (or until STAGECOACH_STUB_HEARTBEAT_TOTAL_MS), never writing stdout."
The regression test this enables: configure idle > heartbeat interval so the idle clock keeps
resetting, hard cap small, assert the kill is downstream-identical to a timeout (exit 124 / RescueError
/ no-retry / invariants hold).

**Both (b) and (c)** belong in cmd/stubagent (stdlib-only, single binary) with the knobs surfaced as
`stubtest.Options` fields + `optsEnvMap` entries, so every existing Manifest/NewScript/Env path picks
them up; the counter/marker/argsfile ordering (marker BEFORE first activity tick) must be preserved for
the deterministic-race tests.

## 2. The existing stub pipeline (cmd/stubagent/main.go, 207 lines; the insertion point)

Execution order today (each invocation = a fresh process):
1.  Drain stdin FIRST (deadlock guard) — STDINFILE capture / MATCHFILE tee / Discard.
1a. INTERVAL_FILE — capture the start instant (concurrency-overlap probe).
1b. MARKER — write "1" (readiness; BEFORE any sleep so tests can race deterministically).
1c. ARGSFILE — write argv NUL-joined.
2.  Sleep (SLEEP_MS uniform; MATCHFILE may add per-match sleep) — AFTER draining.
3.  STDERR — a single optional write.
4.  Stdout selection (MATCHFILE > Script > OUT) + ONE fmt.Fprint immediately before exit.
4b. INTERVAL_FILE append (end instant).
5.  EXIT (STAGECOACH_STUB_EXIT).

**Insertion point**: two new phases 1d (EMIT) + 1e (HEARTBEAT), between 1c (argsfile) and 2 (sleep).
This preserves: stdin-drain-first, marker-before-first-tick, argsfile ordering. With the knobs unset
both phases are no-ops ⇒ byte-identical default behavior (zero-config contract).

## 3. The knob design (5 new env vars; env-driven; stdlib-only)

- `STAGECOACH_STUB_EMIT_MS` (int, >0 enables) — stdout tick interval.
- `STAGECOACH_STUB_EMIT_LINE` (string; "" ⇒ ".") — the tick payload written verbatim each tick.
- `STAGECOACH_STUB_EMIT_TOTAL_MS` (int; 0 ⇒ FOREVER — a stdout-chatty variant that never answers;
  >0 ⇒ after the bound, fall through to the normal pipeline which emits the final answer: persona (b)).
- `STAGECOACH_STUB_HEARTBEAT_MS` (int, >0 enables) — stderr tick interval.
- `STAGECOACH_STUB_HEARTBEAT_TOTAL_MS` (int; 0 ⇒ forever; >0 ⇒ bounded loop then exit).
  Heartbeat mode NEVER writes stdout (persona (c) never answers) — step 4 is suppressed.

Phase order (deterministic; tests set ONE family at a time): drain → marker → argsfile →
[1d EMIT stdout ticks] → [1e HEARTBEAT stderr ticks] → sleep → stderr → stdout(answer; suppressed in
heartbeat mode) → interval append → exit. EMIT-with-bound then answer = persona (b); EMIT-forever =
stdout-chatty variant; HEARTBEAT-forever = persona (c).

Flushing: os.Stdout/os.Stderr in Go are UNBUFFERED (*os.File writes) — each fmt.Fprint/Fprintln is
immediately visible through the pipe to the parent's countingWriter. No flushing concern.

## 4. The harness seam (internal/stubtest/stubtest.go)

- `Options` struct (L23-36): Out/Exit/SleepMS/Stderr/Script/Counter/Output/StripCodeFence/ArgsFile/Marker.
- `optsEnvMap(o)` (L101+) is the SINGLE SOURCE OF TRUTH for the STAGECOACH_STUB_* knobs (both Env and
  Manifest use it). Note the subtlety: `STAGECOACH_STUB_OUT` is set only in the `Script == ""` branch.
  The NEW knobs are independent of the script/out selection — set them unconditionally when non-zero.
- ADD 5 fields: EmitMS/EmitLine/EmitTotalMS/HeartbeatMS/HeartbeatTotalMS + the optsEnvMap entries
  (only when set — the existing convention; keeps zero-config maps unchanged).
- `Build(t)` compiles ./cmd/stubagent ONCE per test process (sync.Once) — no change needed.
- Manifest(bin, o) → provider.Manifest{... Env: knobs} — the stub rides the REAL provider.Execute
  seam (Render → CmdSpec.Env → cmd.Env), so the new knobs flow automatically once in optsEnvMap.

## 5. The engine facts the cadences are chosen against

- The idle watcher (P1.M2.T1.S2, Implementing) polls on a **~500ms ticker** (S2 PRP: "watchIdle ...
  500ms ticker"; "read activity is the tick that OBSERVED the growth (granularity 500ms — fine for 60s
  deadlines)"; "the first groupReadBytes sample only records the baseline; no delta judgment until tick 2").
- `provider.Execute` NOW takes a `Deadlines` struct (S2 landed): the existing tests call
  `provider.Execute(ctx, *spec, provider.Deadlines{Hard: 200 * time.Millisecond}, nil)`.
- On a deadline kill, Execute returns the PARTIAL stdout/stderr buffers alongside the DeadlineExceeded
  error (executor.go:95-97 — "captured to separate bytes.Buffers and returned even on error").
- Cadence choice for the stub's OWN tests (which don't depend on the watcher): EMIT_MS/HEARTBEAT_MS of
  20-50ms and bounds of 100-300ms — measurable, fast, CI-safe. The S2/S3 consumers will pick cadences
  against the 500ms tick (interval ≪ 500ms so several ticks land between watcher polls).

## 6. The test idiom (internal/stubtest/stubtest_test.go — TestStub_*)

```go
bin := Build(t)
m := Manifest(bin, Options{Out: "...", SleepMS: ...})
spec, err := m.Render("", "", "payload", "off")
out, errb, err := provider.Execute(context.Background(), *spec, provider.Deadlines{Hard: X}, nil)
```
Existing: TestStub_EchoSuccess / _MultilineOut / _NonZeroExit / **_TimeoutKilled** (SleepMS 2000 +
Hard 200ms → DeadlineExceeded; asserts elapsed ≤ 5s) / _StderrCapture / _DrainStdinNoDeadlock /
_ScriptCallVarying / _ScriptBlankIsParseFailure / _MalformedEnvNoPanic. NOTE: cmd/stubagent/ has NO
test file of its own — stub behavior tests live HERE (package stubtest, riding the real Execute seam).

## 7. Consumers (the siblings this enables — NOT this task)

- P1.M3.T1.S2 (idle-kill e2e parity): uses persona (a) hung + (c) heartbeat to prove the idle kill is
  downstream-identical to a timeout (exit 124 / RescueError / no-retry / invariants).
- P1.M3.T1.S3 (false-kill regression): uses persona (b) slow-but-emitting to prove a productive-slow
  agent SURVIVES (idle clock resets on activity), and (c) heartbeat for the hard-cap pathology
  (activity is not progress; the cap must fire).
- This task delivers ONLY the stub knobs + harness fields + stub unit tests. No executor/engine change
  (S1/S2 own those), no docs (test infrastructure).

## 8. Validation

- `go build ./...`; `go vet ./cmd/stubagent/... ./internal/stubtest/...`; `gofmt -l` on both files.
- `go test ./internal/stubtest/ -v` (the new TestStub_* + ALL existing stay green — zero-config default).
- `make test` (the full suite — especially internal/generate + internal/decompose, which drive the stub
  through every existing path; byte-identical default behavior must keep them deterministic).
- `make lint`. grep guards (see PRP Level 4).
