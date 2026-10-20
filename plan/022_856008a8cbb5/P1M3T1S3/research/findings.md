# P1.M3.T1.S3 Research Findings — false-kill regression (FR-A1) + hard-cap heartbeat pathology (FR-A2)

Repo state verified at HEAD `ed94f2b` ("stubagent: add progressive emit + heartbeat modes" — S1 LANDED).
P1.M2 engine LANDED (`8978be0` provider: rework Execute with two-deadline idle/hard model; `a919bfc` threading).
P1.M3.T1.S2 (idle-kill parity) is **Ready/in-flight** — its PRP is the contract; its tests are NOT yet in the tree
(`grep -c IdleKill internal/generate/invariants_test.go` → 0). S3 lands AFTER S2.

---

## 1. The landed engine (internal/provider/executor.go — READ-ONLY for S3)

```go
type Deadlines struct {
    Idle time.Duration // ≤0 ⇒ not armed
    Hard time.Duration // ≤0 ⇒ unbounded
}
var ErrIdleDeadline = &idleDeadlineError{} // Error(): "context deadline exceeded (idle: no observable agent activity)"
                                         // Is() satisfies errors.Is(err, context.DeadlineExceeded) — the D2/FR-A5 linchpin
```

- Execute signature: `Execute(ctx, spec CmdSpec, dl Deadlines, vb *ui.Verbose) (stdout, stderr string, err error)`.
- **Hard cap** = `context.WithTimeout` shadow keyed on `dl.Hard` (independent of activity — the WithTimeout
  shadow is NEVER reset by bytes). **Idle watcher** = `watchIdle` goroutine, 500ms ticker, killed via a separate
  `runCtx` cancel that reaches the same group-kill (SIGTERM → WaitDelay 3s → SIGKILL).
- **Activity channels** (FR-A3): (a) pipe writes — `cmd.Stdout`/`cmd.Stderr` are `countingWriter`s whose
  `LastWrite` stamps on EVERY stdout/stderr byte (portable); (b) Linux-only: process-group `rchar` growth via
  /proc. One clock, reset by whichever channel moved last (D3).
- **D4 ARMING RULE (executor.go:144)**: `if dl.Idle > 0 && (runtime.GOOS == "linux" || spec.ProgressiveStdout)`
  — the watcher arms ONLY on Linux or when the CmdSpec declares progressive stdout. `CmdSpec.ProgressiveStdout`
  (render.go:43) is **always false until P2.M1.T1.S1** (event-stream manifest field — PLANNED, not landed; the
  render.go comment says so explicitly). No manifest field can set it today; `stubtest.Manifest` cannot either.
  ⇒ **On Linux the watcher always arms (rchar channel); on macOS/Windows it is DISARMED for every stub-driven
  CommitStaged test.** Consequences for S3 (see §5).
- **Cause lines (ui.VerboseCause, verbose.go:137 — format `"DEBUG: cause: <reason>\n"`)**:
  - idle kill: `vb.VerboseCause(fmt.Sprintf("idle deadline (%v without observable activity)", dl.Idle))`
  - hard cap: `vb.VerboseCause(fmt.Sprintf("hard cap (%v)", dl.Hard))` — emitted when `runCtx.Err()` is
    DeadlineExceeded and the idleKilled flag is false.
  - Order: `idleKilled.Load()` checked FIRST, then `runCtx.Err()`.
- Error contract on deadline kill: idle ⇒ `ErrIdleDeadline`; hard cap ⇒ bare `context.DeadlineExceeded`. BOTH
  satisfy `errors.Is(err, context.DeadlineExceeded)` downstream (FR-A5).
- Partial stdout/stderr are returned even on kill (snapshot copies of the countingWriters).

## 2. The stub personas (S1 LANDED — commit ed94f2b)

`stubtest.Options` fields (internal/stubtest/stubtest.go) + env knobs read by cmd/stubagent/main.go:

| Persona | Options | Behavior |
|---|---|---|
| (a) hung | `SleepMS: N` | drains stdin, marker, then silent sleep — zero bytes |
| (b) slow-but-emitting | `EmitMS: 500, EmitLine: "\n", EmitTotalMS: 6000, Out: "feat: …"` | stdout tick every 500ms for 6000ms (each an unbuffered pipe write ⇒ countingWriter LastWrite stamp), THEN the final answer as the stdout SUFFIX; `EmitTotalMS` 0 ⇒ forever+never answer |
| (c) chatty-forever heartbeat | `HeartbeatMS: 50` (+ `HeartbeatTotalMS` 0 ⇒ forever) | stderr "tick" line every 50ms forever; NEVER writes stdout (step-4 write guarded by `!heartbeat`) |

Stub pipeline order (load-bearing): drain stdin FIRST → interval/marker/argsfile → **1d EMIT** → **1e HEARTBEAT**
→ sleep → stderr → stdout write (suppressed under heartbeat) → exit. No bufio — every tick is immediate pipe
activity. Knobs ride the REAL Execute seam via `stubtest.Manifest(bin, o)` (manifest `Env` = knobs).

## 3. The S2 contract (plan/022_856008a8cbb5/P1M3T1S2/PRP.md — treat as landed before S3)

S2 delivers (tests-only):
- `internal/generate/invariants_test.go`: TestInvariants table row `"IdleKill"` — hung stub
  (`Out:"feat: slow", SleepMS:2000`), `cfg.Timeout=30s` (LARGE) + `cfg.IdleTimeout=150ms` (SMALL); asserts
  `errors.Is(err, ErrTimeout)` + verbose contains `"DEBUG: cause: idle deadline"` + Execute count == 1
  (`strings.Count(buf, "DEBUG: command: ")`); `return err, ""` → `assertInvariants` (I1 idempotent index /
  I2 atomic HEAD / I3 snapshot immutability); `-short` skip.
- **NEW FILE** `internal/generate/idle_parity_test.go` — **`package generate_test`** (EXTERNAL — required
  because internal/exitcode imports internal/generate, so package-generate tests CANNOT import exitcode):
  `TestIdleKill_ExitCodeParity` (asserts `errors.As → *RescueError`, `errors.Is(ErrTimeout)`, `TreeSHA != ""`,
  `FormatRescue` contains `"Tree ID: "+TreeSHA`, cause contains "idle", **`exitcode.For(err) == exitcode.Timeout`
  (124)**, HEAD unchanged, elapsed ≤ 10s hang-guard, cause line, count == 1) + `TestCommitStaged_WorkDescription_IdleKill`.
  Own repo helpers (initTempRepo etc. copied from hooks_freeze_test.go pattern).
- `internal/decompose/planner_test.go` + `decompose_test.go` idle-kill parity tests (not S3's concern).

S2's parity-assertion set is EXACTLY what S3's recipe (c) mirrors with the cause line swapped to "hard cap".

## 4. Existing deadline tests in internal/generate (do not duplicate)

- `generate_deadlines_test.go` (**`package generate`**, internal) — `TestCommitStaged_MessageRoleIdleTimeout`:
  the canonical CommitStaged deadline test. Uses in-package helpers `initRepo/commitRaw/writeFile/stageFile/
  headSHA` (from generate_test.go). Header documents the platform split: on Linux the idle watcher arms (D4)
  and kills; on darwin/windows the idle is DISARMED and the bounded hard cap lands instead — asserting only
  `errors.Is(err, context.DeadlineExceeded)` is platform-safe. Pattern for cfg: `config.Defaults()` +
  `cfg.Timeout` + `cfg.Roles = map[string]config.RoleConfig{"message": {IdleTimeout: …}}`.
- `internal/provider/executor_idle_linux_test.go` — `TestExecute_IdleReadCounterKeepsReaderAlive` (FR-A3(b)
  read-counter reset, Execute seam, Linux-gated). The PIPE-channel reset (stdout/stderr bytes reset the idle
  clock) has NO executor-seam test — S3's recipe (b) at the CommitStaged level is its end-to-end proof on Linux.
- `internal/generate/invariants_test.go` TestInvariants table — row shape:
  `run: func(t *testing.T, _ context.Context, repo, bin string) (error, string)`; rows construct their own
  manifest/cfg/Deps inside the closure; `return err, ""` funnels to `treeSHAFromErr` + `assertInvariants`.
  Existing "Timeout" row (the template): `stubtest.Options{Out:"feat: slow", SleepMS:2000}` + `cfg.Timeout=150ms`.

## 5. Platform analysis — the D4 rule's effect on S3's two tests (THE key design note)

CI matrix includes Linux AND macOS (and windows for race) — `.github/workflows/ci.yml` matrix.jobs.

**Recipe (b) FALSE-KILL (FR-A1: productive-slow SURVIVES):**
- Config: `cfg.IdleTimeout = 2s` (SMALL — would kill a silent agent), `cfg.Timeout = 30s` (LARGE — never fires),
  stub `EmitMS: 500, EmitLine: "\n", EmitTotalMS: 6000` (emits every 0.5s for ~3× the idle window per the item
  contract) then answers `Out: "feat: progressive answer"`.
- Linux: watcher ARMED. Each tick (500ms ≪ 2s idle — 4× margin vs the 500ms poll granularity) stamps
  LastWrite ⇒ the idle clock resets ~12× during the run ⇒ NO kill; the run finishes at ~6s; generation SUCCEEDS
  and the commit IS created. **This is the FR-A1 discrimination — carried on Linux.**
- macOS/Windows: watcher DISARMED (ProgressiveStdout false until P2) ⇒ idle cannot fire ⇒ same SUCCEEDS
  outcome, but vacuously (D4's documented degradation = v3.4 behavior). The test stays GREEN and correct on
  every platform; the test comment MUST state this (the item's "note the stub's test manifest or Linux arming
  per the D4 rule"). Do NOT try to force arming — "stagecoach never fabricates activity" (D4).
- The elapsed floor (≥ 6s) holds on ALL platforms (the stub emits for 6s regardless) and on Linux doubles as
  proof the run genuinely outlived the 2s idle window ~3×.

**Recipe (c) HEARTBEAT PATHOLOGY (FR-A2: false-alive BOUNDED):**
- Config: `cfg.IdleTimeout = 1s` (SMALLER than the cap — the discriminator), `cfg.Timeout = 1500ms` (tiny hard
  cap), stub `HeartbeatMS: 50` forever (stderr "tick" every 50ms, never answers).
- Linux: watcher ARMED; stderr ticks stamp errb.LastWrite every 50ms (20× margin vs the 1s idle) ⇒ the idle
  clock keeps resetting ⇒ idle NEVER fires; the **hard cap** fires at 1.5s. Cause line says **"hard cap"** —
  and BECAUSE idle (1s) < hard (1.5s), a "hard cap" cause PROVES activity was continuously observed yet the
  run was still bounded: FR-A2 "activity is not progress". If the engine wrongly let activity starve the cap,
  or wrongly fired idle, the cause line would differ or the test would hang to the 60s test timeout.
- macOS/Windows: watcher DISARMED ⇒ only the hard cap can fire ⇒ same kill at 1.5s, same cause line. GREEN
  everywhere; Linux carries the activity-reset-under-pressure proof.

**Why idle < hard in (c) (not idle > hard):** with idle > hard the idle deadline could never fire anyway and
the test would not discriminate; with idle < hard, only the heartbeat's resets prevent an idle kill — the
"hard cap" cause line then certifies the resets happened (Linux) while the cap still bounded the run.

## 6. Timing budget (watcher 500ms poll)

- (b): kill would land at first poll ≥2s after last tick if the engine regressed; healthy run ~6-6.5s.
  Assert elapsed ≥ 6000ms (floor) — no upper bound needed (30s hard cap + suite guard); hang-guard optional.
- (c): kill at ~1.5s + SIGTERM grace (< 3s WaitDelay worst case). Assert elapsed ≤ 10s hang-guard.
- All three tests carry `if testing.Short() { t.Skip(...) }`. Suite time added ≈ 9-10s total.

## 7. Message-parse detail for (b) — use EmitLine "\n"

The stub's ticks and the final answer share one stdout buffer: captured stdout = ticks + answer. With
`EmitLine: "\n"` each tick is a bare newline; `ParseOutput` (TrimSpace → raw mode → normalizeNewlines →
TrimSpace) strips the leading newlines ⇒ `Result.Message == "feat: progressive answer"` and
`Result.Subject == "feat: progressive answer"` — CLEAN equality assertions. (With the default "." tick the
subject would be ".........feat: progressive answer" — only Contains assertions possible.) Duplicate-reject
(§9.7) compares the subject against the repo's history ("initial") — no collision. Empty-output retry never
triggers (message non-empty).

## 8. Where each test lives (and why)

| Test | File | Package | Why |
|---|---|---|---|
| `TestCommitStaged_ProductiveSlowEmitterSurvives` (b) | internal/generate/generate_deadlines_test.go (APPEND) | generate (internal) | Success path — no exitcode assertion needed; in-package helpers (initRepo/commitRaw/writeFile/stageFile/headSHA) + the file is the established CommitStaged-deadline home (its header already documents the D4 platform split) |
| TestInvariants row `"HardCapHeartbeat"` (c) | internal/generate/invariants_test.go (APPEND row) | generate (internal) | I1/I2/I3 no-mutation invariants for free via the harness; mirrors S2's "IdleKill" row |
| `TestHardCapHeartbeat_ExitCodeParity` (c) | internal/generate/idle_parity_test.go (APPEND — S2's file) | generate_test (EXTERNAL) | exitcode.For==124 assertion REQUIRES the external package (import cycle: exitcode imports generate); reuses S2's initTempRepo/writeFile/stageFile/headSHA helpers |

Assertion parity with S2 (recipe c): `errors.As → *RescueError`; `errors.Is(generate.ErrTimeout)`;
`TreeSHA != ""`; `FormatRescue(TreeSHA, ParentSHA, Candidate)` contains `"Tree ID: "+TreeSHA`; verbose contains
`"DEBUG: cause: hard cap"`; Execute count == 1; `exitcode.For(err) == exitcode.Timeout` (124); HEAD unchanged;
elapsed ≤ 10s. For the hard cap the Cause IS bare `context.DeadlineExceeded` (Error() = "context deadline
exceeded" — does NOT name "hard cap"); the "hard cap" naming lives ONLY in the verbose cause line — assert it
there, not on Cause.Error().

## 9. Facts for the success-path assertions (b)

`generate.Result{CommitSHA, Subject, Message, Provider, Model, Changes}`. Success checks: `err == nil`;
`Result.Subject == "feat: progressive answer"`; HEAD advanced (`git rev-parse HEAD` != before AND contains
Result.CommitSHA); `git log -1 --format=%s` == subject; the changed file is in the commit; verbose count of
"DEBUG: command: " == 1 (single Execute — no retry churn); verbose does NOT contain `"DEBUG: cause:"`
(nothing was killed); elapsed ≥ 6000ms.

## 10. Scope fences

- Tests ONLY (item: "DOCS: none — tests only"; "Mock: stub agents only"). ZERO production change; if any
  assertion fails, the defect is in the LANDED engine (P1.M2) or the S2/S1 harness — surface it, never patch
  the engine from S3.
- Do NOT touch cmd/stubagent or internal/stubtest (S1 scope, landed). Do NOT add executor-seam tests
  (P1.M2.T1.S2's executor_test.go owns the seam; executor_idle_linux_test.go owns rchar).
- Do NOT cover workdesc/multiturn/decompose variants (S2's domain; recipes (b)/(c) are CommitStaged-level).
- Keep S2's existing tests untouched (append only; S2 lands first — if S2's file names differ at
  implementation time, adapt the append target, not the assertions).
