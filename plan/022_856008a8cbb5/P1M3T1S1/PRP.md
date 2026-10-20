name: "P1.M3.T1.S1 — stubagent knobs: progressive-emit + chatty-heartbeat stub modes (test infrastructure for FR-A1–A5/G30)"
description: >
  TEST-INFRASTRUCTURE task (no engine change, no docs). Extend cmd/stubagent (the stdlib-only fake-agent
  binary the whole test suite drives through the REAL provider.Execute seam) with the two stub personas
  the v3.5 activity-deadline testing needs, exactly per architecture/call_sites_and_error_contract.md §9.6:
  (b) SLOW-BUT-EMITTING — STAGECOACH_STUB_EMIT_MS (stdout tick interval) + STAGECOACH_STUB_EMIT_LINE (the
  tick payload; "" ⇒ ".") + STAGECOACH_STUB_EMIT_TOTAL_MS (0 ⇒ emit FOREVER and never answer — a
  stdout-chatty variant; >0 ⇒ after the bound fall through to the normal pipeline and emit the final
  answer); (c) CHATTY-FOREVER HEARTBEAT — STAGECOACH_STUB_HEARTBEAT_MS (stderr tick interval; the codex-style
  false-alive pathology) + STAGECOACH_STUB_HEARTBEAT_TOTAL_MS (0 ⇒ forever; >0 ⇒ bounded then exit);
  heartbeat mode NEVER writes stdout. The two new phases insert into main.go's pipeline between step 1c
  (argsfile) and step 2 (sleep) — preserving the three load-bearing ordering constraints from §9.6:
  stdin-drain-FIRST (deadlock guard), MARKER-BEFORE-FIRST-TICK (deterministic-race tests), argsfile
  ordering. Zero-config default is byte-identical (knobs unset ⇒ both phases are no-ops; the entire
  existing suite stays deterministic). Knobs surface as 5 new stubtest.Options fields (EmitMS/EmitLine/
  EmitTotalMS/HeartbeatMS/HeartbeatTotalMS) + optsEnvMap entries (set only when non-zero, UNCONDITIONAL of
  the Script/Out branch — the OUT-only-in-else-branch subtlety) so every existing Manifest/NewScript/Env
  path picks them up automatically. Tests ride the existing TestStub_* idiom in internal/stubtest/stubtest_test.go
  (Build → Manifest → Render → provider.Execute(ctx, spec, provider.Deadlines{Hard: X}, nil)): emit-then-answer
  (ticks PREFIX + answer SUFFIX, elapsed ≥ total), emit cadence (tolerant tick-count range), emit-forever +
  hard-cap kill (DeadlineExceeded + partial-tick stdout captured + answer ABSENT — the §9.6 partial-capture
  contract), heartbeat + hard-cap kill (DeadlineExceeded + stderr ticks + stdout EMPTY — never answers),
  heartbeat-bounded exit (err==nil, stdout empty), and a zero-config default guard. Cadences are chosen
  against the ~500ms watcher tick from the in-flight P1.M2.T1.S2 (stub-internal tests use 20-50ms intervals,
  100-300ms bounds — measurable and CI-fast; the S2/S3 consumers pick cadences vs the 500ms granularity).
  Deliverable = 3 stub personas — (a) hung (EXISTS: SleepMS), (b) slow-but-emitting, (c) chatty-forever
  heartbeat — consumable by P1.M3.T1.S2 (idle-kill parity) and P1.M3.T1.S3 (false-kill regression).

---

## Goal

**Feature Goal**: Make the three v3.5 deadline-test stub personas expressible with the stubagent harness:
(a) a HUNG agent (exists today — SleepMS), (b) a SLOW-BUT-EMITTING agent (stdout ticks keep the idle clock
resetting, then the answer arrives — or never, when unbounded), and (c) a CHATTY-FOREVER HEARTBEAT agent
(periodic stderr ticks forever, never answers — the false-alive pathology only the hard cap can kill).
Today only (a) is expressible: stdout is written exactly once immediately before exit, and stderr is a
single optional write — neither progressive output nor a heartbeat loop exists (§9.6: "NOT expressible with
today's stubagent"). Without these knobs the idle-deadline engine (P1.M2.T1.S2) and its parity/regression
tests (P1.M3.T1.S2/S3) have no way to produce the byte-activity patterns they must observe.

**Deliverable** (2 files modified; no new files except tests appended to an existing file):
1. **cmd/stubagent/main.go** — two new phases (1d EMIT, 1e HEARTBEAT) inserted between step 1c (argsfile)
   and step 2 (sleep), driven by 5 new env vars; stdlib-only; zero-config default unchanged.
2. **internal/stubtest/stubtest.go** — 5 new `Options` fields + their `optsEnvMap` entries.
3. **internal/stubtest/stubtest_test.go** — 6 new TestStub_* tests (emit-then-answer, emit cadence,
   emit-forever hard-cap kill + partial capture, heartbeat hard-cap kill, heartbeat bounded exit,
   zero-config default guard).

**Success Definition**:
- `Options{EmitMS: 50, EmitTotalMS: 300, Out: "feat: answer"}` ⇒ Execute returns stdout = ticks* +
  "feat: answer" (the answer is the SUFFIX — emitted only after the emit phase completes), elapsed ≥ 300ms.
- `Options{EmitMS: 50}` (no total ⇒ forever) + `Deadlines{Hard: 250ms}` ⇒ DeadlineExceeded AND stdout
  contains ≥1 tick AND does NOT contain the answer (the §9.6 partial-capture contract: the buffer returns
  even on kill).
- `Options{HeartbeatMS: 50}` (forever) + `Deadlines{Hard: 250ms}` ⇒ DeadlineExceeded AND stderr contains
  ticks AND stdout == "" (never answers).
- `Options{HeartbeatMS: 20, HeartbeatTotalMS: 100}` + a large Hard ⇒ err == nil, stdout == "" (bounded
  loop ends; still never answers), stderr has ticks.
- Zero-config: no new knob set ⇒ byte-identical behavior (the entire existing suite green and deterministic).
- The stdin-drain-first / marker-before-first-tick / argsfile ordering constraints hold (§9.6).
- `go build ./...`; `go vet`; `gofmt -l` empty; `go test ./internal/stubtest/ -v` green (new + existing);
  `make test` + `make lint` green.

## User Persona (if applicable)

**Target User**: The P1.M3.T1.S2 (idle-kill e2e parity) and P1.M3.T1.S3 (false-kill regression) test
authors — this task IS the mock infrastructure they consume. (End users are unaffected: test-only.)

**Use Case**: S2 needs a stub whose silence trips the idle deadline (persona a — exists) and one whose
stderr heartbeat keeps the idle clock resetting while never producing output, so ONLY the hard cap kills
it (persona c — the false-alive pathology the spec's "activity is a productivity signal, not a progress
guarantee" row demands be killed by the cap). S3 needs a stub whose stdout activity keeps the idle clock
resetting while it works slowly and then produces a valid answer (persona b — the false-kill regression:
a productive-slow agent must SURVIVE).

**User Journey**: S2/S3 test author → `stubtest.Manifest(bin, stubtest.Options{HeartbeatMS: 50, ...})` →
the stub drains stdin, writes the marker, then ticks stderr every 50ms forever → the executor's idle
watcher sees byte activity (clock resets) while the hard cap (small in the test) eventually kills the
process group → Execute returns (partial-stdout="", stderr=ticks, DeadlineExceeded) → the test asserts the
kill is downstream-identical to a timeout. WITHOUT this task those tests cannot be written.

**Pain Points Addressed**: §9.5 "NOT tested today" list — "nothing exercises progressive output or
heartbeat-style activity". The v3.5 engine's two discriminating behaviors (idle-reset-on-activity;
cap-fires-on-heartbeat) are untestable end-to-end without these knobs.

## Why

- **§9.6 (the authoritative recipe)**: the report explicitly specifies both knob families, their wiring
  constraints (stdin-drain guard, marker-before-tick, partial-buffer return), and their home (cmd/stubagent,
  stdlib-only, surfaced through stubtest.Options/optsEnvMap). This task implements that recipe verbatim.
- **Unblocks the milestone**: P1.M3.T1.S2/S3 are the parity + regression proofs of the entire v3.5
  deadlines feature (FR-A1–A5); both are blocked on personas (b) and (c) existing. This is the cheapest
  task in the chain and the one everything downstream composes with.
- **Zero-risk additive**: env-driven no-op defaults keep every existing test byte-identical; the new phases
  slot between two existing numbered steps without touching any existing line's semantics; stdlib-only.

## What

**User-visible behavior**: None (test infrastructure).

**Technical change**: two new phases in cmd/stubagent/main.go + 5 Options fields + optsEnvMap entries +
6 TestStub_* tests. See the Implementation Blueprint for the verbatim phase code.

### Success Criteria
- [ ] `cmd/stubagent/main.go` gains phase 1d (EMIT: stdout ticks every STAGECOACH_STUB_EMIT_MS, payload
      STAGECOACH_STUB_EMIT_LINE default ".") + phase 1e (HEARTBEAT: stderr ticks every
      STAGECOACH_STUB_HEARTBEAT_MS, payload a fixed short "tick" line), inserted after argsfile (1c),
      before the sleep (2).
- [ ] EMIT: EMIT_TOTAL_MS > 0 ⇒ tick for that long then fall through to the normal pipeline (the final
      answer is emitted AFTER the phase — persona b); EMIT_TOTAL_MS == 0 ⇒ tick forever, never answer.
- [ ] HEARTBEAT: HEARTBEAT_TOTAL_MS == 0 ⇒ tick forever; > 0 ⇒ bounded then continue; heartbeat mode
      NEVER writes stdout (step 4 suppressed — persona c never answers).
- [ ] `internal/stubtest/stubtest.go`: Options gains EmitMS/EmitLine/EmitTotalMS/HeartbeatMS/
      HeartbeatTotalMS; optsEnvMap emits them when set (EmitLine when != "", ints when > 0),
      unconditionally of the Script/Out branch.
- [ ] 6 new tests in stubtest_test.go (the Build→Manifest→Render→Execute(Deadlines) idiom) covering:
      emit-then-answer (ticks prefix + answer suffix + elapsed bound), emit cadence (tolerant count),
      emit-forever hard-cap kill (DeadlineExceeded + partial ticks + answer absent), heartbeat hard-cap
      kill (DeadlineExceeded + stderr ticks + stdout empty), heartbeat bounded exit (nil err, stdout
      empty), zero-config default guard.
- [ ] Zero-config default: with none of the 5 knobs set, stubagent behavior is byte-identical (existing
      suite green, deterministic).
- [ ] `go build ./...`; `go vet ./cmd/stubagent/... ./internal/stubtest/...`; `gofmt -l` empty on the
      touched files; `go test ./internal/stubtest/ -v` green; `make test` + `make lint` green.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim §9.6 recipe (the knob names, payloads, bounds semantics, and the three wiring
constraints), the exact insertion point in main.go's numbered pipeline (between 1c and 2) with the existing
step inventory, the verbatim phase code (loop structure, payload defaults, the never-answer rule), the
optsEnvMap subtlety (OUT is only set in the Script=="" branch — the new knobs must be unconditional), the
test idiom verbatim (Build → Manifest → Render("", "", "payload", "off") → provider.Execute(ctx, spec,
provider.Deadlines{Hard: X}, nil)) including how the already-landed S2 Deadlines struct is called, the
~500ms watcher-tick context the cadences are chosen against, the partial-buffer-on-kill contract
(executor.go:95-97), the Go unbuffered-os.Stdout fact (no flush concern), and the zero-config default
requirement with its regression net (the full make test).

### Documentation & References

```yaml
# MUST READ — the authoritative findings (the §9.6 recipe + the pipeline inventory + the seam subtleties)
- docfile: plan/022_856008a8cbb5/P1M3T1S1/research/findings.md
  why: "§1 the verbatim §9.6 recipe (knob names, payloads, bounds, the 3 wiring constraints, the
        partial-capture wiring fact); §2 the existing stub pipeline steps + the insertion point (between
        1c argsfile and 2 sleep); §3 the knob design table (all 5 env vars + persona mapping + the
        unbuffered-stdout fact); §4 the optsEnvMap subtlety (OUT only in the else branch; new knobs
        unconditional); §5 the engine facts (500ms tick; Execute now takes Deadlines{Hard}; partial
        buffers return on kill); §6 the test idiom verbatim; §7 the S2/S3 consumers; §8 validation."
  critical: "§1/§2: the three load-bearing constraints — stdin-drain FIRST, MARKER before the first tick,
             argsfile ordering preserved. §3: EMIT_TOTAL_MS==0 ⇒ forever-and-never-answer (a deliberate
             stdout-chatty variant); HEARTBEAT never writes stdout. §4: set the new optsEnvMap entries
             UNCONDITIONALLY of the Script branch."

# MUST READ — the recipe's source of truth (§9.6 verbatim)
- docfile: plan/022_856008a8cbb5/architecture/call_sites_and_error_contract.md
  section: "§9.6 How to build the three v3.5 stubs" (+ §9.2 the stub pipeline inventory, §9.5 the NOT-tested-today list)
  why: "The authoritative knob spec: (b) EMIT_MS + EMIT_LINE/BYTES + optional EMIT_TOTAL_MS, ticks to stdout
        AFTER stdin-drain/marker and BEFORE the final Fprint+exit; (c) HEARTBEAT_MS loop Fprintln(stderr)
        every N ms forever (or TOTAL_MS), never writing stdout; both in cmd/stubagent, stdlib-only, surfaced
        as Options fields + optsEnvMap entries; marker BEFORE first tick preserved. Plus the wiring facts:
        Execute's bytes.Buffer stdout means ticks are invisible to ParseOutput until exit but ARE the idle
        watcher's byte-activity; a deadline kill returns the PARTIAL buffer with DeadlineExceeded."
  critical: "Do NOT deviate from the recipe's ordering constraints — the deterministic-race tests depend on
             marker-before-first-tick, and the deadlock guard depends on drain-first."

# MUST EDIT — the stub binary (the two new phases)
- file: cmd/stubagent/main.go   # insert phases 1d/1e between step 1c (argsfile, ~L71-74) and step 2 (sleep, ~L80)
  why: "The fake-agent binary. Stdlib-only (package doc: 'STDLIB ONLY; no internal/*, no third-party'). The
        numbered-step pipeline is documented in §9.2 and inline; the new phases slot in as 1d and 1e."
  pattern: "Mirror the existing numbered-step comment style (each step cites its knob + why). envInt(key, def)
            is the existing accessor for int knobs; os.Getenv for string knobs."
  gotcha: "os.Stdout/os.Stderr are UNBUFFERED in Go (*os.File) — each Fprint/Fprintln is immediately visible
           through the pipe; do NOT introduce bufio (it would delay ticks past the kill and break the
           partial-capture assertions)."

# MUST EDIT — the harness seam (Options + optsEnvMap)
- file: internal/stubtest/stubtest.go   # Options struct L23-36; optsEnvMap L101-125
  why: "optsEnvMap is the SINGLE SOURCE OF TRUTH for the STAGECOACH_STUB_* knobs (both Env and Manifest use
        it) — adding the 5 entries there makes every existing Manifest/NewScript/Env path pick them up
        automatically. Manifest() rides the REAL provider.Execute seam (Render → CmdSpec.Env → cmd.Env)."
  pattern: "Follow the existing convention: ints emitted only when > 0 (SleepMS), strings only when != ''
            (Stderr/ArgsFile/Marker). Document each field with its env-var name + persona."
  gotcha: "STAGECOACH_STUB_OUT is set only in the Script==\"\" branch of optsEnvMap — that is intentional
           (script mode replaces single-response). The NEW knobs are independent of that selection: emit
           them whenever set, in BOTH branches."

# MUST EDIT — the stub tests (append; the TestStub_* idiom)
- file: internal/stubtest/stubtest_test.go   # TestStub_TimeoutKilled L65 is the kill-test template
  why: "Where stub behavior tests live (cmd/stubagent has NO test file of its own). The idiom: Build(t) →
        Manifest(bin, Options{...}) → m.Render(\"\", \"\", \"payload\", \"off\") → provider.Execute(ctx,
        *spec, provider.Deadlines{Hard: X}, nil) → assert on (out, errb, err) + elapsed. Imports already
        include context/errors/os/os-exec/strings/testing/time + internal/provider."
  pattern: "TestStub_TimeoutKilled: SleepMS 2000 + Deadlines{Hard: 200ms} → errors.Is(err, DeadlineExceeded)
            + elapsed ≤ 5s. TestStub_StderrCapture: asserts errb == the configured stderr exactly."
  gotcha: "Cadence assertions must tolerate scheduler/CI slack — assert elapsed ≥ total (a hard floor) and a
           GENEROUS tick-count window (e.g. count in [total/interval − 1, total/interval + 2]); never an
           exact count. Keep every test ≤ ~1s so the suite stays fast."

# CONTEXT — the engine the knobs are chosen against (P1.M2.T1.S2, Implementing; consume, don't rebuild)
- docfile: plan/022_856008a8cbb5/P1M2T1S2/PRP.md
  why: "Defines provider.Execute(ctx, spec, dl Deadlines, vb) + watchIdle (a ~500ms ticker; the first tick
        only initializes the baseline) + ErrIdleDeadline. The stub's OWN tests only need Deadlines{Hard: X}
        (already the landed call shape the existing tests use); the 500ms granularity is why S2/S3 will pick
        tick intervals ≪ 500ms — noted so the knob docs can recommend the cadence range."
  critical: "Do NOT touch the executor/engine in this task (S2 owns it). The stub tests use Hard-cap kills,
             which land regardless of the idle watcher's state."

# CONTEXT — the activity infrastructure (P1.M2.T1.S1, Complete; read-only)
- file: internal/provider/activity.go   # countingWriter (output-side activity channel)
  why: "Confirms the byte-activity observation seam exists: the executor assigns countingWriters to
        cmd.Stdout/cmd.Stderr, so every stub tick is observed as activity. No change needed here — the stub
        just has to WRITE bytes on a cadence (unbuffered writes; see the main.go gotcha)."

# CONTEXT — the consumers (LAND LATER; not this task)
- docfile: (P1.M3.T1.S2 + P1.M3.T1.S3 PRPs, when written)
  why: "S2 (idle-kill parity) consumes personas (a)+(c): idle > heartbeat interval so the clock keeps
        resetting, small hard cap, assert the kill is downstream-identical to a timeout (exit 124 /
        RescueError / no-retry / invariants). S3 (false-kill regression) consumes persona (b): a
        productive-slow agent survives; + (c) for the cap pathology. This task delivers the knobs only."
  critical: "After this task, the new knobs have ZERO production/engine consumers outside stubagent +
             stubtest tests — expected. The engine consumers are S2/S3's tests, not engine code."
```

### Current Codebase tree (relevant slice)

```bash
cmd/stubagent/
  main.go                 # EDIT — +phase 1d (EMIT stdout ticks) + phase 1e (HEARTBEAT stderr ticks); stdlib-only
internal/stubtest/
  stubtest.go             # EDIT — Options +5 fields; optsEnvMap +5 entries (unconditional of the Script branch)
  stubtest_test.go        # EDIT — +6 TestStub_* tests (the Build→Manifest→Render→Execute(Deadlines) idiom)
  editor.go               # READ-ONLY — stubeditor/stubtee (unrelated knobs)
internal/provider/
  activity.go             # READ-ONLY (P1.M2.T1.S1 Complete) — countingWriter; the observation seam
  executor.go             # READ-ONLY (P1.M2.T1.S2 in-flight) — Execute(Deadlines); partial-buffer-on-kill
# go.mod / Makefile — READ-ONLY (no new dep; stdlib only)
```

### Desired Codebase tree with files to be modified

```bash
cmd/stubagent/main.go            # MODIFIED — phases 1d/1e + the 5 env vars (envInt/os.Getenv; stdlib only)
internal/stubtest/stubtest.go    # MODIFIED — Options +5 fields + optsEnvMap +5 entries
internal/stubtest/stubtest_test.go  # MODIFIED — +6 TestStub_* tests appended
# (NO new files. NO engine/executor change. NO docs — test infrastructure.)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the three §9.6 ordering constraints — do not break them): (1) stdin drains FIRST (the executor
// pipes the payload through a ~64KiB OS pipe; sleeping/ticking before draining deadlocks parent+child on
// big payloads); (2) the MARKER is written BEFORE the first activity tick (deterministic-race tests poll
// it to know generation is in-flight — a tick that precedes the marker breaks their race window);
// (3) argsfile ordering stays (after marker). The new phases insert AFTER 1c and BEFORE 2 — never earlier.

// CRITICAL (heartbeat mode NEVER writes stdout — persona (c)'s defining property): when
// STAGECOACH_STUB_HEARTBEAT_MS > 0, suppress step 4's fmt.Fprint(stdout). The false-alive pathology the
// S2 test asserts is "stderr ticks keep the idle clock resetting while NO answer ever arrives" — an
// answer leaking through would turn it into a success path. (The final EXIT code still applies.)

// CRITICAL (EMIT_TOTAL_MS == 0 means FOREVER-and-never-answer, not "no emit"): the knob only ENABLES the
// emit phase when EMIT_MS > 0; TOTAL_MS==0 is a deliberate stdout-chatty variant (ticks forever, answer
// never — the stdout-side twin of the heartbeat pathology). TOTAL_MS>0 is persona (b): tick for TOTAL,
// then fall through and emit the final answer AFTER the phase. Do not treat 0 as "disable".

// CRITICAL (NO bufio — Go's os.Stdout/os.Stderr are already unbuffered): each fmt.Fprint(os.Stdout, ...)
// is a pipe write immediately visible to the parent's countingWriter. Wrapping stdout in bufio.Writer
// would batch ticks past the kill point and break the partial-capture assertions (the parent would see
// fewer bytes than ticks emitted, or none). Write directly; never flush-manage.

// CRITICAL (optsEnvMap: the new entries are UNCONDITIONAL of the Script/Out branch): the existing map sets
// STAGECOACH_STUB_OUT only in the Script=="" branch (script mode replaces single-response output). The
// emit/heartbeat knobs are TIMING behaviors orthogonal to output selection — emit them in BOTH branches
// whenever set (EmitLine when != "", the four ints when > 0), mirroring how SleepMS/Stderr/Marker are
// handled. Conditional-on-Script would silently drop ticks in script-mode tests.

// GOTCHA (cadence assertions must be slack-tolerant): scheduler granularity + CI variance make exact tick
// counts flaky. Assert elapsed >= EMIT_TOTAL_MS (a hard floor) and a GENEROUS count window
// (count in [total/interval - 1, total/interval + 2]); assert Contains for payloads, never equality on a
// full tick stream. Keep each new test ≤ ~1s (the suite must stay fast).

// GOTCHA (the ~500ms watcher tick is NOT this task's concern — but the knob DOCS should note it): the
// engine's idle watcher polls on ~500ms granularity (P1.M2.T1.S2). The stub's OWN tests don't depend on
// it (they use Hard-cap kills). The S2/S3 consumers will choose tick intervals ≪ 500ms (e.g. 50ms) so
// several ticks land between watcher polls — mention the recommended range in the Options field docs.

// GOTCHA (per-invocation = fresh process): every stub invocation is a separate process (no shared state),
// so the tick loops are plain per-process time.Ticker loops — no cleanup/coordination needed beyond the
// process dying (the hard-cap kill SIGTERMs the process group; the loop ends with the process).

// GOTCHA (zero-config default is byte-identical — the regression net): with none of the 5 knobs set, both
// new phases are no-ops and main.go's behavior is unchanged. The proof is `make test` green — especially
// internal/generate + internal/decompose, which drive the stub through every existing path (timeout
// scenarios, concurrency probes, freeze tests). If ANY existing test breaks, the phases leaked into the
// default path.
```

## Implementation Blueprint

### Data models and structure

None beyond the Options struct fields (5 ints/strings). No new types. The phases are inline loops in
main() using stdlib `time`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT cmd/stubagent/main.go — phase 1d (EMIT: stdout ticks) + phase 1e (HEARTBEAT: stderr ticks)
  - PLACE: immediately AFTER step 1c (the argsfile block) and BEFORE step 2 (the sleep block). Renumber
    nothing (existing comments stay; the new block cites "1d"/"1e").
  - ADD (verbatim shape; adapt comments to the file's style):
        // 1d. (P1.M3.T1.S1) Progressive-emit phase — the "slow-but-emitting" stub persona (§9.6 (b)) and its
        //     unbounded stdout-chatty variant. If STAGECOACH_STUB_EMIT_MS > 0, write the emit payload to
        //     stdout every interval, for STAGECOACH_STUB_EMIT_TOTAL_MS (0 ⇒ forever — never answer). When
        //     bounded, the phase ends and the normal pipeline continues (the final answer is emitted AFTER
        //     the ticks — the answer is the stdout SUFFIX). Ticks ARE the byte-activity an idle watcher
        //     observes (Execute buffers stdout; a deadline kill returns the PARTIAL ticks alongside the
        //     error). Ordering: AFTER the stdin drain (deadlock guard) and AFTER the marker (deterministic
        //     races); BEFORE the sleep/stdout/exit. os.Stdout is unbuffered — each write is immediately
        //     observable. Zero-config: EMIT_MS unset/0 ⇒ this phase is a no-op.
        if emitMS := envInt("STAGECOACH_STUB_EMIT_MS", 0); emitMS > 0 {
            line := os.Getenv("STAGECOACH_STUB_EMIT_LINE")
            if line == "" {
                line = "."
            }
            total := time.Duration(envInt("STAGECOACH_STUB_EMIT_TOTAL_MS", 0)) * time.Millisecond
            deadline := time.Time{} // zero ⇒ forever
            if total > 0 {
                deadline = time.Now().Add(total)
            }
            ticker := time.NewTicker(time.Duration(emitMS) * time.Millisecond)
            for range ticker.C {
                fmt.Fprint(os.Stdout, line)
                if !deadline.IsZero() && time.Now().After(deadline) {
                    ticker.Stop()
                    break
                }
            }
        }

        // 1e. (P1.M3.T1.S1) Chatty-heartbeat phase — the false-alive persona (§9.6 (c)). If
        //     STAGECOACH_STUB_HEARTBEAT_MS > 0, write a short tick line to STDERR every interval, forever
        //     (STAGECOACH_STUB_HEARTBEAT_TOTAL_MS == 0) or until the bound. Heartbeat mode NEVER writes
        //     stdout (the persona's defining property — no answer ever arrives; step 4 is suppressed).
        //     Same ordering constraints as 1d (after drain + marker). Zero-config ⇒ no-op.
        heartbeat := envInt("STAGECOACH_STUB_HEARTBEAT_MS", 0) > 0
        if heartbeat {
            total := time.Duration(envInt("STAGECOACH_STUB_HEARTBEAT_TOTAL_MS", 0)) * time.Millisecond
            deadline := time.Time{}
            if total > 0 {
                deadline = time.Now().Add(total)
            }
            ticker := time.NewTicker(time.Duration(envInt("STAGECOACH_STUB_HEARTBEAT_MS", 0)) * time.Millisecond)
            for range ticker.C {
                fmt.Fprintln(os.Stderr, "tick")
                if !deadline.IsZero() && time.Now().After(deadline) {
                    ticker.Stop()
                    break
                }
            }
        }
  - STEP 4 SUPPRESSION: guard the existing stdout write with `if !heartbeat { ... fmt.Fprint(os.Stdout, out) }`
    (keep the out-selection logic; only the WRITE is suppressed — a comment on the guard cites §9.6 (c)
    "never writing stdout"). The interval append (4b) + EXIT (5) still run.
  - GOTCHA: do NOT renumber the existing steps' comments (they're cited across the codebase/docs); the new
    blocks carry their own 1d/1e labels. Keep envInt/os.Getenv (the existing accessors).

Task 2: EDIT internal/stubtest/stubtest.go — Options +5 fields + optsEnvMap +5 entries
  - Options additions (documented with env-var names + the persona + the recommended cadence note):
        EmitMS          int    // STAGECOACH_STUB_EMIT_MS (stdout tick interval; >0 enables the emit phase)
        EmitLine        string // STAGECOACH_STUB_EMIT_LINE (tick payload; "" ⇒ ".")
        EmitTotalMS     int    // STAGECOACH_STUB_EMIT_TOTAL_MS (0 ⇒ forever/never answer; >0 ⇒ bounded, then the answer)
        HeartbeatMS     int    // STAGECOACH_STUB_HEARTBEAT_MS (stderr tick interval; >0 enables; never answers)
        HeartbeatTotalMS int   // STAGECOACH_STUB_HEARTBEAT_TOTAL_MS (0 ⇒ forever; >0 ⇒ bounded then exit)
  - optsEnvMap additions — UNCONDITIONAL of the Script/Out branch (mirror the SleepMS/Stderr handling):
        if o.EmitMS > 0 { m["STAGECOACH_STUB_EMIT_MS"] = strconv.Itoa(o.EmitMS) }
        if o.EmitLine != "" { m["STAGECOACH_STUB_EMIT_LINE"] = o.EmitLine }
        if o.EmitTotalMS > 0 { m["STAGECOACH_STUB_EMIT_TOTAL_MS"] = strconv.Itoa(o.EmitTotalMS) }
        if o.HeartbeatMS > 0 { m["STAGECOACH_STUB_HEARTBEAT_MS"] = strconv.Itoa(o.HeartbeatMS) }
        if o.HeartbeatTotalMS > 0 { m["STAGECOACH_STUB_HEARTBEAT_TOTAL_MS"] = strconv.Itoa(o.HeartbeatTotalMS) }
  - NOTE: emit them OUTSIDE the if o.Script != "" / else branches (the OUT-only-in-else subtlety — the new
    knobs are timing behaviors, orthogonal to output selection).

Task 3: EDIT internal/stubtest/stubtest_test.go — append 6 TestStub_* tests (the existing idiom)
  - (a) TestStub_EmitThenAnswer: Options{EmitMS: 50, EmitTotalMS: 300, Out: "feat: answer"} →
        Execute(Deadlines{Hard: 5s}) → err==nil; out ENDS WITH "feat: answer" (HasSuffix) AND contains ≥1
        tick (strings.Count(out, ".") ≥ 1 — EmitLine default "."); elapsed ≥ 300ms.
  - (b) TestStub_EmitCadence: Options{EmitMS: 40, EmitTotalMS: 280, Out: "feat: x"} → Hard 5s → ticks =
        strings.Count(out, ".") in [280/40 − 1, 280/40 + 2] = [6, 9] (slack-tolerant); out ends with "feat: x".
  - (c) TestStub_EmitForeverHardCapKilled: Options{EmitMS: 50, Out: "feat: never"} (NO total ⇒ forever) →
        Execute(Deadlines{Hard: 250ms}) → errors.Is(err, context.DeadlineExceeded); out contains ≥1 tick
        (the §9.6 partial-capture contract — the buffer returns even on kill); out does NOT contain
        "feat: never" (never answered); elapsed ≤ 5s (process-group kill fires promptly).
  - (d) TestStub_HeartbeatHardCapKilled: Options{HeartbeatMS: 50, Out: "feat: never"} (forever) →
        Execute(Deadlines{Hard: 250ms}) → DeadlineExceeded; errb contains "tick" (≥1); out == "" (never
        answers); elapsed ≤ 5s.
  - (e) TestStub_HeartbeatBoundedExit: Options{HeartbeatMS: 20, HeartbeatTotalMS: 100, Out: "feat: never"} →
        Execute(Deadlines{Hard: 5s}) → err == nil (the bounded loop ends); out == "" (still never answers);
        errb contains "tick".
  - (f) TestStub_EmitZeroConfigDefault: Options{Out: "feat: plain"} (no emit/heartbeat knobs) → Hard 5s →
        out == "feat: plain" EXACTLY (no ticks, no newline — the byte-identical default guard).
  - FOLLOW pattern: TestStub_TimeoutKilled (the kill-test shape) + TestStub_StderrCapture (the exact-output
    assertion shape). Imports are already present (context/errors/strings/testing/time + provider).
  - GOTCHA: every test ≤ ~1s (5s Hard caps only guard hangs; the fast paths return in ms).

Task 4: VERIFY — build, vet, format, focused + full tests, lint, grep guards
  - go build ./... ; go vet ./cmd/stubagent/... ./internal/stubtest/...
  - gofmt -l cmd/stubagent/main.go internal/stubtest/stubtest.go internal/stubtest/stubtest_test.go  # empty
  - go test ./internal/stubtest/ -v          # the 6 new + ALL existing TestStub_* green
  - go test ./internal/generate/ ./internal/decompose/ -count=1   # the stub's heaviest consumers (determinism net)
  - make test ; make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the bounded-or-forever tick loop (both phases share the shape; zero deadline ⇒ forever).
if emitMS := envInt("STAGECOACH_STUB_EMIT_MS", 0); emitMS > 0 {
    deadline := time.Time{}                       // zero ⇒ forever
    if total := envInt("STAGECOACH_STUB_EMIT_TOTAL_MS", 0); total > 0 {
        deadline = time.Now().Add(time.Duration(total) * time.Millisecond)
    }
    ticker := time.NewTicker(time.Duration(emitMS) * time.Millisecond)
    for range ticker.C {
        fmt.Fprint(os.Stdout, line)               // unbuffered — immediately observable activity
        if !deadline.IsZero() && time.Now().After(deadline) {
            ticker.Stop()
            break                                  // bounded ⇒ fall through to the answer
        }
    }
}

// PATTERN: the never-answer guard (heartbeat suppresses step 4's stdout write).
if !heartbeat {
    fmt.Fprint(os.Stdout, out)                     // §9.6 (c): heartbeat mode never writes stdout
}

// PATTERN: the partial-capture assertion (the §9.6 wiring fact — the S2/S3 contract this enables).
out, errb, err := provider.Execute(ctx, *spec, provider.Deadlines{Hard: 250 * time.Millisecond}, nil)
if !errors.Is(err, context.DeadlineExceeded) { t.Fatalf(...) }
if !strings.Contains(out, ".") { t.Errorf("partial stdout missing ticks: %q", out) }        // buffer returns on kill
if strings.Contains(out, "feat: never") { t.Errorf("unbounded emit must never answer") }   // persona integrity
```

### Integration Points

```yaml
STUB BINARY (cmd/stubagent/main.go):
  - +phase 1d (EMIT stdout ticks; 3 env vars) + phase 1e (HEARTBEAT stderr ticks; 2 env vars).
  - Step 4's stdout write guarded by !heartbeat. Everything else byte-identical.

HARNESS (internal/stubtest/stubtest.go):
  - Options +5 fields; optsEnvMap +5 entries (unconditional of the Script branch).

TESTS (internal/stubtest/stubtest_test.go):
  - +6 TestStub_* (emit-then-answer, cadence, emit-forever-kill, heartbeat-kill, heartbeat-bounded,
    zero-config guard).

NO engine/executor change (P1.M2.T1.S2 owns Execute/watchIdle/ErrIdleDeadline); NO docs (test infra);
NO new deps (stdlib only); NO new files.

CONSUMERS (LAND LATER — this task unblocks them):
  - P1.M3.T1.S2 (idle-kill parity): personas (a)+(c) — idle > heartbeat interval, small hard cap, assert
    the kill is downstream-identical to a timeout (exit 124 / RescueError / no-retry / invariants).
  - P1.M3.T1.S3 (false-kill regression): persona (b) survives (idle clock resets on activity); persona (c)
    for the hard-cap pathology.
  - ZERO production/engine consumers of the knobs — the consumers are those test files.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build + vet (stdlib-only; both packages must compile alongside the in-flight engine work).
go build ./...
go vet ./cmd/stubagent/... ./internal/stubtest/...
# Expected: clean.

# Format.
gofmt -l cmd/stubagent/main.go internal/stubtest/stubtest.go internal/stubtest/stubtest_test.go
# Expected: empty. If listed: gofmt -w.

# Lint.
make lint
# Expected: zero errors (the new Options fields are read by optsEnvMap + tests ⇒ not `unused`).

# Scope guard: only the 3 expected files changed.
git status --short
# Expected: M cmd/stubagent/main.go  M internal/stubtest/stubtest.go  M internal/stubtest/stubtest_test.go (only).
```

### Level 2: Unit Tests (Component Validation) — THE PRIMARY GATE

```bash
# The stub's own tests (new + existing — the zero-config default must keep every existing one green).
go test ./internal/stubtest/ -v
# Expected: PASS — the 6 new (emit-then-answer / cadence / emit-forever-kill / heartbeat-kill /
#           heartbeat-bounded / zero-config) + EchoSuccess, MultilineOut, NonZeroExit, TimeoutKilled,
#           StderrCapture, DrainStdinNoDeadlock, ScriptCallVarying, ScriptBlankIsParseFailure,
#           MalformedEnvNoPanic.

# The determinism net: the stub's heaviest consumers (timeout scenarios, concurrency probes, freeze tests).
go test ./internal/generate/ ./internal/decompose/ -count=1
# Expected: green (knobs unset ⇒ byte-identical stub behavior).

# Full race suite.
make test
# Expected: green. (The tick loops are per-process; no shared state; -race clean.)
```

### Level 3: Integration Testing (System Validation)

```bash
# There is no separate integration surface — the TestStub_* tests ARE the integration (they drive the
# compiled stub through the REAL provider.Execute seam: Render → CmdSpec.Env → cmd.Env → process group).
# The e2e consumers (idle-kill parity, false-kill regression) are P1.M3.T1.S2/S3 — NOT this task.

# Manual confidence check (optional): run one persona by hand to see the byte pattern.
STAGECOACH_STUB_EMIT_MS=100 STAGECOACH_STUB_EMIT_TOTAL_MS=500 STAGECOACH_STUB_OUT="feat: answer" \
  cmd/stubagent 2>/dev/null <<< "payload"   # expect ~5 ticks then "feat: answer" on one line
STAGECOACH_STUB_HEARTBEAT_MS=100 timeout 1 cmd/stubagent <<< "payload"   # expect ~9 stderr "tick" lines, no stdout
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: the 5 env vars exist in the stub.
grep -c 'STAGECOACH_STUB_EMIT_MS\|STAGECOACH_STUB_EMIT_LINE\|STAGECOACH_STUB_EMIT_TOTAL_MS\|STAGECOACH_STUB_HEARTBEAT_MS\|STAGECOACH_STUB_HEARTBEAT_TOTAL_MS' cmd/stubagent/main.go
# Expected: ≥5 hits (each knob read at least once).

# Guard 2: the knobs are surfaced in the harness (the single source of truth).
grep -c 'STAGECOACH_STUB_EMIT_MS\|STAGECOACH_STUB_HEARTBEAT_MS' internal/stubtest/stubtest.go
# Expected: ≥2 (the optsEnvMap entries; the Options field docs also name them).

# Guard 3: heartbeat NEVER writes stdout — the step-4 guard exists.
grep -n 'if !heartbeat' cmd/stubagent/main.go
# Expected: 1 hit guarding the fmt.Fprint(os.Stdout, out) write.

# Guard 4: NO bufio (ticks must be unbuffered).
grep -n 'bufio' cmd/stubagent/main.go
# Expected: ZERO hits (introducing bufio would break the partial-capture timing).

# Guard 5: the ordering constraints — the new phases come AFTER the marker and BEFORE the sleep.
grep -n 'STAGECOACH_STUB_MARKER\|STAGECOACH_STUB_EMIT_MS\|STAGECOACH_STUB_HEARTBEAT_MS\|STAGECOACH_STUB_SLEEP_MS' cmd/stubagent/main.go
# Expected: the MARKER read precedes the EMIT/HEARTBEAT reads, which precede the SLEEP read (line order).

# Guard 6: stdlib-only (no internal/*, no third-party) in the stub.
grep -n 'stagecoach/internal\|"[^"]*"' cmd/stubagent/main.go | grep -v 'os\|fmt\|io\|time\|bytes\|strconv\|strings'
# Expected: EMPTY beyond the stdlib imports.

# Guard 7: the 6 new tests exist.
grep -c 'func TestStub_Emit\|func TestStub_Heartbeat' internal/stubtest/stubtest_test.go
# Expected: 6 (EmitThenAnswer, EmitCadence, EmitForeverHardCapKilled, HeartbeatHardCapKilled,
#           HeartbeatBoundedExit, EmitZeroConfigDefault — the zero-config one greps as TestStub_Emit...).

# Guard 8: scope — only 3 files.
git status --porcelain
# Expected: exactly the 3 files (main.go, stubtest.go, stubtest_test.go). No engine/docs files.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `go vet ./cmd/stubagent/... ./internal/stubtest/...` clean
- [ ] `gofmt -l` empty on the 3 touched files
- [ ] `make lint` zero errors
- [ ] `go test ./internal/stubtest/ -v` green (6 new + all existing TestStub_*)
- [ ] `go test ./internal/generate/ ./internal/decompose/ -count=1` green (the determinism net)
- [ ] `make test` (race) green

### Feature Validation
- [ ] Persona (b): `EmitMS + EmitTotalMS` ⇒ ticks-then-answer (answer is the stdout SUFFIX); `EmitMS` alone
      ⇒ forever, never answer; a hard-cap kill returns the PARTIAL ticks with DeadlineExceeded
- [ ] Persona (c): `HeartbeatMS` ⇒ stderr ticks forever (or to `HeartbeatTotalMS`), stdout always empty,
      never answers; a hard-cap kill returns stderr ticks + empty stdout + DeadlineExceeded
- [ ] Persona (a) unchanged (SleepMS still works — existing TimeoutKilled test green)
- [ ] The three §9.6 ordering constraints hold (drain first; marker before first tick; argsfile order)
- [ ] Zero-config default byte-identical (the zero-config guard test + the full suite)

### Scope-Boundary Validation
- [ ] `git status` == {cmd/stubagent/main.go, internal/stubtest/stubtest.go, internal/stubtest/stubtest_test.go}
- [ ] NO engine/executor change (internal/provider/* untouched — P1.M2.T1.S2 owns it)
- [ ] NO new files; NO new deps (stdlib only); NO docs (test infrastructure)
- [ ] NO renumbering of the existing pipeline step comments (cited across the codebase)
- [ ] Grep guards 1–8 (Level 4) all pass

### Code Quality & Docs
- [ ] The new phase comments cite §9.6 + the persona + the ordering constraints (the file's comment style)
- [ ] The Options field docs name their env vars + the recommended cadence range (≪ the ~500ms watcher tick)
- [ ] Cadence assertions are slack-tolerant (elapsed floors + generous count windows; never exact counts)

---

## Anti-Patterns to Avoid

- ❌ Don't tick before draining stdin. The executor pipes the payload through a ~64KiB OS pipe; writing
  ticks (or sleeping) before the drain deadlocks parent+child on large payloads — the deadlock guard is
  step 1 for a reason. The new phases go AFTER the marker/argsfile (1b/1c), never before the drain.
- ❌ Don't let the marker lag the first tick. The deterministic-race tests (CAS-race, freeze-race) poll the
  marker to know generation is in-flight; a tick that lands before the marker breaks their race window
  semantics. Marker first, THEN ticks — the exact constraint §9.6 calls out.
- ❌ Don't let heartbeat mode write stdout. Persona (c)'s defining property is "never answers" — the
  false-alive pathology only exists if NO answer ever arrives. Guard step 4's write with `if !heartbeat`.
  An answer leaking through silently converts the S2 hard-cap test into a success-path test.
- ❌ Don't treat `EMIT_TOTAL_MS == 0` as "disable the emit phase". The phase is enabled by `EMIT_MS > 0`;
  `TOTAL_MS == 0` is the deliberate forever-never-answer variant (the stdout-chatty twin of the heartbeat).
  Mapping 0 to "no emit" would make the unbounded persona inexpressible.
- ❌ Don't wrap stdout/stderr in bufio. Go's *os.File writes are unbuffered — each Fprint is immediately
  observable pipe activity, which is the entire point (the idle watcher counts bytes). bufio would batch
  ticks past the kill and break the partial-capture assertions. Grep guard 4 enforces this.
- ❌ Don't gate the new optsEnvMap entries on the Script branch. `STAGECOACH_STUB_OUT` is intentionally
  script-conditional (script mode replaces single-response output); the emit/heartbeat knobs are timing
  behaviors orthogonal to output selection. Emit them unconditionally when set, in both branches.
- ❌ Don't add an engine/executor change. The idle watcher, ErrIdleDeadline, Deadlines resolution, and the
  13 call sites are P1.M2.T1.S2/S3's scope (in flight). This task is ONLY the stub + harness + stub tests.
  If a test needs the idle watcher to fire, that test belongs to S2 — not here.
- ❌ Don't write exact tick-count assertions. Scheduler granularity + CI variance make `count == N` flaky.
  Assert elapsed floors (≥ EMIT_TOTAL_MS) and generous windows (±1/+2 ticks); assert Contains/HasSuffix
  for payloads. Keep every test ≤ ~1s so the suite stays fast.
- ❌ Don't renumber the existing pipeline step comments. The numbered steps (1, 1a, 1b, 1c, 2, 3, 4, 4b, 5)
  are cited in §9.2 of the architecture doc and across test comments. The new phases are 1d/1e — additive
  labels, existing comments untouched.
- ❌ Don't add a third knob family or configurability beyond the recipe. The 5 env vars implement §9.6's
  personas exactly. A configurable heartbeat PAYLOAD, a jitter knob, or byte-count-vs-line modes are
  scope creep — the payload content is irrelevant to byte-activity observation; only cadence matters.
- ❌ Don't skip the zero-config guard test. The default path staying byte-identical is a contract
  (existing tests stay deterministic); without an explicit guard (TestStub_EmitZeroConfigDefault asserting
  out == "feat: plain" EXACTLY), a future edit could leak ticks into the default and only fail downstream
  suites mysteriously.

---

## Confidence Score: 9/10

The task implements a verbatim, pre-researched recipe (§9.6 names the knobs, payloads, bounds semantics,
and all three wiring constraints), in a 207-line stdlib-only stub whose pipeline is fully inventoried, at a
precise insertion point (between 1c and 2), surfaced through a single-source-of-truth seam (optsEnvMap)
with a known subtlety (the OUT-in-else-branch) explicitly fenced, tested with an established idiom
(TestStub_TimeoutKilled's Build→Manifest→Render→Execute(Deadlines{Hard}) shape — S2's Deadlines call
signature verified landed), and the engine facts the cadences are chosen against (~500ms watcher tick;
partial-buffer-on-kill at executor.go:95-97) documented. The -1 from 10/10 reflects the inherent timing-test
fragility: six new time-dependent tests on shared CI runners can flake if the slack windows are too tight —
the PRP mandates generous windows (elapsed floors + ±1/+2 count ranges, ≤1s per test) but the exact bounds
are the implementer's judgment call at test-authoring time, and one over-tight assertion would produce an
intermittent failure rather than a one-pass failure (caught by re-runs, not the first gate). No engine
dependency risk (the stub tests use Hard-cap kills only, which work regardless of the in-flight watcher's
state), no new deps, no docs, no production surface.
