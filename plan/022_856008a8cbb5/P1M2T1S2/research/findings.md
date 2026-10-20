# Research Findings — P1.M2.T1.S2 (Execute rework: Deadlines, idle watcher, ErrIdleDeadline, VerboseCause, ProgressiveStdout)

## 0. Task shape
Rework `provider.Execute` from a single wall-clock `timeout` to the two-deadline model (§9.30
FR-A1–A5): define `Deadlines{Idle, Hard}`, make stdout/stderr countingWriters (S1's), add the D4-armed
idle watcher goroutine, return the D2 `ErrIdleDeadline` sentinel (satisfies
`errors.Is(err, context.DeadlineExceeded)`), add `ui.VerboseCause` (D6) + `CmdSpec.ProgressiveStdout`
(D4), and mechanically update ALL 14 non-test call sites + the 23 test calls so the tree stays green.
Idle is DISARMED everywhere in this task (`Idle: 0` / zero-value) — exactly v3.4 behavior; S3
(P1.M2.T1.S3) threads the 13 role sites to `config.ResolveRoleDeadlines` (already exists — P1.M1.T1.S1
Complete: `config.ResolveRoleDeadlines(role string, cfg Config) (idle, hard time.Duration)` @
internal/config/roles.go:78; defaults 60s/600s seeded @ config.go:230/228).

## 1. ⭐ THE SCOPE RESOLUTION: S2 must update ALL call sites (or the tree won't compile)
The contract's LOGIC names ONLY models.go:150 among callers, but the signature change breaks ALL 14
non-test `provider.Execute(` sites. **Resolution (anchored in D1: "All 13 role call sites updated
mechanically")**: S2 updates all 14 sites MECHANICALLY — `provider.Deadlines{Hard: X}` (Idle
zero-value ⇒ disarmed ⇒ byte-identical behavior) — plus the test files. models.go additionally gets
the contract's comment (it is the ONE site S3 will never touch — not a role call; idle permanently
unarmed). S3 then replaces the mechanical `Deadlines{Hard: X}` at the 13 role sites with
`ResolveRoleDeadlines` threading (the semantic change). This keeps `go build ./...` + `make test`
green at every subtask boundary.

### The 14 non-test sites (verified by grep; X = the existing duration expression, unchanged)
| # | Site | Mechanical edit |
|---|------|-----------------|
| 1 | internal/cmd/models.go:149 | `provider.Deadlines{Idle: 0, Hard: timeout}` + comment (the contract's named site) |
| 2 | internal/decompose/arbiter.go:101 | `provider.Deadlines{Hard: arbiterTimeout}` |
| 3 | internal/decompose/message.go:173 | `provider.Deadlines{Hard: messageTimeout}` |
| 4 | internal/decompose/planner.go:128 | `provider.Deadlines{Hard: plannerTimeout}` |
| 5 | internal/decompose/stager.go:119 | `provider.Deadlines{Hard: stagerTimeout}` |
| 6 | internal/generate/generate.go:374 | `provider.Deadlines{Hard: msgTimeout}` |
| 7–9 | internal/generate/multiturn.go:172/183/194 | `provider.Deadlines{Hard: msgTimeout}` |
| 10–12 | internal/generate/workdesc.go:82/131/149 | `provider.Deadlines{Hard: msgTimeout}` |
| 13 | internal/hook/exec.go:187 | `provider.Deadlines{Hard: msgTimeout}` |
| 14 | pkg/stagecoach/stagecoach.go:569 | `provider.Deadlines{Hard: cfg.Timeout}` |

### Test sites (23 mechanical changes)
- internal/provider/executor_test.go — 11 `Execute(` calls (SAME package — no prefix): `…, 0, nil)` →
  `…, Deadlines{}, nil)`; `…, 200*time.Millisecond, nil)` → `…, Deadlines{Hard: 200 * time.Millisecond}, nil)`, etc.
- internal/stubtest/stubtest_test.go — 12 `provider.Execute(` calls (same mechanical form).

## 2. The current Execute (executor.go:65–103) — every load-bearing detail
- Hard-cap timeout via SHADOWING `ctx, cancel = context.WithTimeout(ctx, timeout)` (:66–70, "SHADOW —
  do not rename" — the later `ctx.Err()` reads the timeout ctx). `timeout ≤ 0` ⇒ unbounded.
- `var out, errb bytes.Buffer; cmd.Stdout = &out; cmd.Stderr = &errb` (:76–78) — fully buffered; no
  incremental visibility (the FR-A3 gap). → S2 swaps for countingWriters.
- Kill path: `exec.CommandContext` + `setupProcessGroup` (Setpgid ⇒ PGID==PID; cmd.Cancel = SIGTERM to
  `-pid`; WaitDelay 3s → SIGKILL) (:72/:82).
- `signal.RegisterChild(cmd.Process.Pid)` / `defer signal.ClearChild()` bracket Wait (:89–90).
- Error branch (:92–99): on Wait error → VerboseRawOutput+VerboseStderr dumps → `ctx.Err()` checked
  FIRST, returned BARE (DeadlineExceeded/Canceled) → else wrapped `*exec.ExitError`.
- Doc contract (:56–63): timeout ⇒ `err IS context.DeadlineExceeded`; signal/parent ⇒ Canceled;
  non-zero exit ⇒ wrapped ExitError; start failure ⇒ wrapped LookPath; success ⇒ nil.

## 3. The reworked Execute (verbatim design)
```go
// Deadlines — define in executor.go just above Execute (D1).
type Deadlines struct {
	Idle time.Duration // kill after this long with NO observable activity (FR-A1); ≤0 ⇒ not armed
	Hard time.Duration // absolute wall-clock backstop (FR-A2); ≤0 ⇒ unbounded (today's timeout==0)
}

// ErrIdleDeadline (D2/FR-A5) — sentinel whose Is() satisfies errors.Is(err, context.DeadlineExceeded).
var ErrIdleDeadline = &idleDeadlineError{}
type idleDeadlineError struct{}
func (e *idleDeadlineError) Error() string { return "context deadline exceeded (idle: no observable agent activity)" }
func (e *idleDeadlineError) Is(target error) bool { return target == context.DeadlineExceeded }
```
Execute body changes (in order):
1. `if dl.Hard > 0 { ctx, cancel = context.WithTimeout(ctx, dl.Hard); defer cancel() }` (KEEP the shadow).
2. NEW `runCtx, cancelRun := context.WithCancel(ctx); defer cancelRun()` — the idle watcher cancels
   runCtx, wrapping the SAME cmd.Cancel group-kill path (D3: "cancel via a dedicated WithCancel").
3. `cmd := exec.CommandContext(runCtx, …)` (was ctx).
4. `var out, errb countingWriter` + assign `&out`/`&errb` (S1's type; all return sites become
   `string(out.Bytes())` — the snapshot-copy contract).
5. After Start + RegisterChild: arm the watcher per D4 —
   `if dl.Idle > 0 && (runtime.GOOS == "linux" || spec.ProgressiveStdout) { go watchIdle(runCtx, cancelRun, &idleKilled, dl.Idle, cmd.Process.Pid, &out, &errb) }`
   with `var idleKilled atomic.Bool` declared before.
6. Wait-error branch: dumps (now `string(out.Bytes())`); then `if idleKilled.Load() { vb.VerboseCause(
   fmt.Sprintf("idle deadline (%v without observable activity)", dl.Idle)); return …, ErrIdleDeadline }`;
   then `if ctxErr := runCtx.Err(); ctxErr != nil { if errors.Is(ctxErr, context.DeadlineExceeded) {
   vb.VerboseCause(fmt.Sprintf("hard cap (%v)", dl.Hard)) }; return …, ctxErr }`; else the ExitError wrap.
   Success path: same dumps + nil.
7. Imports: DROP `bytes`; ADD `errors`, `runtime`, `sync/atomic` (context/fmt/os/exec/strings/time/signal/ui stay).

### watchIdle helper (in executor.go; the D3 single-clock design)
```go
func watchIdle(ctx context.Context, cancel context.CancelFunc, killed *atomic.Bool,
	idle time.Duration, pgid int, out, errb *countingWriter) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	armed := time.Now()          // baseline: never-written ⇒ idle-since-arm (S1's zero-time contract)
	var lastRchar uint64
	haveRchar := false
	var readMovedAt time.Time
	for {
		select {
		case <-ctx.Done():
			return // hard cap / parent cancel / interrupt / deferred cancelRun at Execute return
		case now := <-ticker.C:
			last := armed
			if t := out.LastWrite(); t.After(last) { last = t }   // pipe channel (portable)
			if t := errb.LastWrite(); t.After(last) { last = t }
			if sum, ok := groupReadBytes(pgid); ok {              // read-counter channel (Linux)
				if haveRchar && sum > lastRchar { readMovedAt = now } // positive delta ⇒ activity
				lastRchar, haveRchar = sum, true
			}
			if readMovedAt.After(last) { last = readMovedAt }
			if now.Sub(last) >= idle { killed.Store(true); cancel(); return }
		}
	}
}
```
Key semantics: ONE clock reset by whichever channel moved last; rchar timestamps = the tick that
observed growth (the counter is cumulative — a positive delta means activity within the window);
negative deltas (member exited / non-atomic counters, per external_procfs_notes §2) are implicitly
CLAMPED (only `sum > lastRchar` counts); `ok==false` (non-Linux stub / no /proc) ⇒ pipe channel only.
Benign tail race: after Execute returns, the deferred cancelRun() ends the goroutine within ≤1 tick;
groupReadBytes on a recycled pid is a harmless read-only sweep (same discipline as signal.ClearChild).

## 4. D4 arming rule + CmdSpec.ProgressiveStdout (render.go:32 struct — ADD the field)
All CmdSpec literals are KEYED (verified — no unkeyed literals anywhere), so adding a field breaks
nothing. Add after Env:
```go
	// ProgressiveStdout reports whether the rendered invocation emits stdout progressively
	// (event-stream mode, §9.30 FR-S1). Set by Render iff the manifest declares event_stream
	// tokens (P2.M1.T1.S1 — always false until then, so the idle watcher arms only on Linux,
	// where the rchar channel covers every provider). Arms the portable pipe-activity idle
	// channel (FR-A3(a)/D4). Stagecoach never fabricates activity (D4).
	ProgressiveStdout bool
```
Non-Linux + print-mode (ProgressiveStdout=false) ⇒ idle DISARMED ⇒ exactly v3.4 behavior (hard cap only).

## 5. ui.VerboseCause (D6) — internal/ui/verbose.go, mirroring VerboseWarn verbatim
The Verbose method idiom (verbose.go:51–58/123–130): `if v == nil || v.w == nil || !v.on { return }` +
`v.mu.Lock(); defer v.mu.Unlock()` + `fmt.Fprintln(v.w, "DEBUG: …")`. VerboseCause:
```go
// VerboseCause prints WHY a generation deadline fired (§9.30 FR-A1/FR-A2, D6): "idle deadline (…)"
// or "hard cap (…)". Emitted by provider.Execute on either deadline kill, after the raw-output/stderr
// dumps — the ONLY place idle vs hard cap is distinguished (downstream, an idle kill IS a timeout:
// FR-A5). Format: "DEBUG: cause: <reason>\n". No-op when v==nil, v.w==nil, or !v.on.
func (v *Verbose) VerboseCause(reason string) {
	if v == nil || v.w == nil || !v.on { return }
	v.mu.Lock(); defer v.mu.Unlock()
	fmt.Fprintln(v.w, "DEBUG: cause: "+reason)
}
```
Consistency note: default_action.go:381 already prints `"DEBUG: cause: "+re.Cause.Error()` for rescue
causes via fmt.Fprintln (verboseOn-gated) — the SAME prefix convention. Leave it as-is (it has no
*Verbose instance); VerboseCause is the new ui-canonical method invoked ONLY by Execute. NOT this task's
scope to refactor default_action.go.

## 6. The Execute doc-comment contract update
Timeout paragraph becomes: "Deadlines: dl.Hard > 0 derives a context.WithTimeout (shadowing ctx —
load-bearing…); dl.Hard ≤ 0 ⇒ unbounded. dl.Idle > 0 arms the idle watcher (FR-A1) when
runtime.GOOS==linux OR spec.ProgressiveStdout (D4) — kill after dl.Idle with no observable activity
(pipe writes + Linux group rchar growth, FR-A3), through the SAME group-kill path as the hard cap."
Error contract becomes:
```
//   - idle kill       ⇒ err IS ErrIdleDeadline — satisfies errors.Is(err, context.DeadlineExceeded)
//                        (D2/FR-A5): every downstream DeadlineExceeded branch (exit 124, ErrTimeout
//                        rescue, no-retry, FR-T7) works UNCHANGED; only the cause line differs
//   - hard cap        ⇒ err IS context.DeadlineExceeded  (orchestrator: exit 124 + rescue)
//   - signal/parent   ⇒ err IS context.Canceled          (orchestrator: exit 3 + rescue)
//   - non-zero exit  ⇒ wrapped *exec.ExitError …   (unchanged rows below)
```

## 7. The models.go:150 call (the contract's named site)
Current (models.go:149–152): `provider.Execute(cmd.Context(), CmdSpec{…}, timeout, vb)`.
New: `provider.Deadlines{Idle: 0, Hard: timeout}` + the comment: "// Not a role call — idle is
intentionally unarmed (a short models probe; the hard cap alone bounds it). P1.M2.T1.S3 threads the 13
role sites to ResolveRoleDeadlines." (models.go is the one site S3 will NOT touch.)

## 8. Tests (contract list; the executor_test.go harness idiom = mustBin + real PATH binaries —
`sleep`, `sh`; sh -c IS the stub-script mechanism; no mocks)
1. UPDATE TestExecute_TimeoutKillsProcess: `Deadlines{Hard: 200 * time.Millisecond}`; ADD assert
   `!errors.Is(err, ErrIdleDeadline)` (hard cap stays the BARE sentinel).
2. NEW TestExecute_IdleKillsSilentStub (portable): `sleep 30` spec with **ProgressiveStdout: true**
   (forces arming on every platform), `Deadlines{Idle: 300ms, Hard: 30s}` → err non-nil;
   `errors.Is(err, ErrIdleDeadline)` AND `errors.Is(err, context.DeadlineExceeded)` (the D2 keystone);
   elapsed < 5s (the group kill fired).
3. NEW TestExecute_IdleSparesBusyEmitter (portable pipe-activity proof): `sh -c "for i in 1 2 3 4 5 6 7 8; do echo x; sleep 0.4; done"` (~3.2s of writes every 400ms), **ProgressiveStdout: true**,
   `Deadlines{Idle: 900ms, Hard: 30s}` → err==nil; out contains 8 "x" lines. (Writes every 400ms <
   idle 900ms ⇒ the clock keeps resetting ⇒ FR-A2 productive-slow survives.)
4. NEW TestExecute_IdleReadCounterKeepsReaderAlive (**//go:build linux** — the rchar channel proof):
   `sh -c "while IFS= read -r line; do sleep 0.3; done"`, Stdin = 10 lines (~3s of slow reads),
   ProgressiveStdout NOT set (the POINT: stdout is silent — only the read counter shows life),
   `Deadlines{Idle: 900ms, Hard: 30s}` → err==nil. If the rchar channel were broken, idle would kill
   at ~900ms and the test fails. (sh+sleep children share the Setpgid group ⇒ their reads count.)
5. NEW internal/ui/verbose_test.go TestVerbose_Cause{WhenOn,WhenOff,NilSafe,NilWriterNoOp} — mirror
   TestVerbose_Warn* (@78–105) exactly, asserting `DEBUG: cause: <reason>`.
6. MECHANICAL: the 11 executor_test.go + 12 stubtest_test.go signature updates (§1).
NOTE on Linux CI: tests 2/3 ALSO exercise the rchar channel on Linux (sleep reads nothing after exec;
sh's echo/sleep children read little) — the assertions hold under both channel configurations, which is
exactly the portability the contract wants.

## 9. S1 dependency + what S2 does NOT do
S1 (P1.M2.T1.S1, in-flight parallel) ships activity.go/activity_linux.go/activity_other.go + tests.
S2 CONSUMES `countingWriter` + `groupReadBytes` — if S1 hasn't landed, executor.go won't compile
(the expected prerequisite signal). S2 does NOT edit activity*.go (S1's files), does NOT touch
config (P1.M1 Complete: ResolveRoleDeadlines @ roles.go:78, Config.IdleTimeout @ config.go:87,
Defaults 60s/600s @ config.go:230), does NOT thread role deadlines (S3), does NOT set
ProgressiveStdout in Render (P2.M1.T1.S1 — Render never sets it yet ⇒ false everywhere ⇒ idle armed
only on Linux), does NOT touch docs (S3's Mode-A docs), does NOT touch default_action.go:381.
D10: exit codes (124 stays 124), rescue, CAS, lock, index, freeze semantics untouched.

## 10. Validation (verified commands)
`go build ./...`; `GOOS=darwin go build ./internal/provider/` + `GOOS=windows …` (the watcher must
compile everywhere — groupReadBytes resolves to the stub off Linux); `go vet ./internal/provider/... ./internal/ui/...`;
`gofmt -l` on the touched files; `go test ./internal/provider/ -run 'Execute' -v`;
`go test ./internal/stubtest/ -v`; `go test ./internal/ui/ -run 'Cause' -v`;
`go test -race ./internal/provider/... ./internal/ui/...` (watcher/countingWriter concurrency);
`make test`; `make lint`. Grep guards: no call site passes a bare duration to Execute;
`ErrIdleDeadline` has the Is method; `ProgressiveStdout` in CmdSpec; `Deadlines{` at all 14 sites.
