name: "P1.M2.T1.S2 — Execute rework: Deadlines struct, idle watcher + D4 arming rule, ErrIdleDeadline sentinel, ui.VerboseCause, CmdSpec.ProgressiveStdout (FR-A1–A5, §9.30)"
description: >
  Rework internal/provider/executor.go's Execute from a single wall-clock timeout to the two-deadline
  model: (1) `provider.Deadlines{Idle, Hard}` (D1) replaces the `timeout time.Duration` param — Hard
  keeps the existing WithTimeout SHADOW (≤0 ⇒ unbounded, as today); (2) stdout/stderr become S1's
  countingWriters (FR-A3(a)); (3) a watchIdle goroutine (D3) — ONE clock reset by whichever channel
  moved last (pipe LastWrite + Linux groupReadBytes rchar growth, deltas implicitly clamped), ~0.5s
  tick, exits on ctx.Done() — armed iff `dl.Idle > 0 AND (runtime.GOOS=="linux" OR
  spec.ProgressiveStdout)` (D4); on expiry it cancels a dedicated WithCancel wrapping the SAME
  cmd.Cancel group-kill path and Execute returns the `ErrIdleDeadline` sentinel whose Is() satisfies
  errors.Is(err, context.DeadlineExceeded) (D2/FR-A5 — every downstream branch unchanged); (4) new
  `ui.VerboseCause` (D6: "DEBUG: cause: idle deadline (…)" / "DEBUG: cause: hard cap (…)") invoked by
  Execute on either deadline kill; (5) new `CmdSpec.ProgressiveStdout` field (Render sets it only in
  P2.M1.T1.S1 ⇒ false everywhere now ⇒ non-Linux = idle disarmed = exactly v3.4 behavior); (6) ALL 14
  non-test call sites + 23 test calls updated MECHANICALLY (`Deadlines{Hard: X}`, idle disarmed — the
  tree stays green; models.go gets the contract's not-a-role-call comment); S3 threads the 13 role
  sites to ResolveRoleDeadlines. Consumes S1's countingWriter/groupReadBytes. NO config (P1.M1 done),
  NO S1 files, NO Render change (P2), NO docs (S3), NO exit-code/rescue/CAS/lock changes (D10).

---

## Goal

**Feature Goal**: Give `provider.Execute` the §9.30 two-deadline engine — an activity-based idle kill
(FR-A1) alongside the raised hard cap (FR-A2) — with the D2 error-contract preservation (an idle kill
satisfies `errors.Is(err, context.DeadlineExceeded)`, so exit-124/ErrTimeout-rescue/no-retry/FR-T7 all
work unchanged) and the D6 verbose cause line as the ONLY observable difference. This is the engine;
arming (S3's ResolveRoleDeadlines threading) and portability (P2's event mode) layer on top.

**Deliverable**:
1. **internal/provider/executor.go** — `Deadlines` struct + `ErrIdleDeadline` sentinel (with Is) +
   the reworked `Execute(ctx, spec, dl Deadlines, vb)` + the `watchIdle` helper + updated doc comment.
2. **internal/provider/render.go** — `CmdSpec.ProgressiveStdout bool` (keyed-literal-safe; always
   false until P2's Render sets it).
3. **internal/ui/verbose.go** (+ **verbose_test.go**) — `VerboseCause(reason)` mirroring VerboseWarn.
4. **14 non-test call sites** mechanically updated (`provider.Deadlines{Hard: X}`; models.go with the
   Idle:0 + comment form) + **executor_test.go (11)** / **stubtest_test.go (12)** signature updates.
5. **NEW tests**: idle-kills-silent-stub (portable, ProgressiveStdout forced), idle-spares-busy-emitter
   (portable), linux-build-tagged read-counter-keeps-reader-alive, + ui VerboseCause tests.

**Success Definition**:
- `Execute(ctx, spec{ProgressiveStdout:true}, Deadlines{Idle:300ms, Hard:30s}, nil)` over a silent
  `sleep 30` returns an error with `errors.Is(err, ErrIdleDeadline)` AND
  `errors.Is(err, context.DeadlineExceeded)` both TRUE, within ~5s (the group kill fired).
- A stub emitting a byte every 400ms under `Idle: 900ms` completes with err==nil (FR-A2: productive-slow survives).
- Hard cap (Idle disarmed) still returns BARE `context.DeadlineExceeded` (NOT ErrIdleDeadline).
- `--verbose` shows `DEBUG: cause: idle deadline (…)` / `DEBUG: cause: hard cap (…)` on the respective kills.
- All existing behavior is byte-identical at every call site (idle disarmed ⇒ v3.4 semantics).
- `go build ./...` + GOOS=darwin/windows provider builds + `go test -race ./internal/provider/... ./internal/ui/...`
  + `make test` + `make lint` green; `gofmt -l` empty.

## User Persona (if applicable)

**Target User**: Every stagecoach user once S3 arms the deadlines (this task is the engine); the
immediate consumer is the S3 call-site threading + the P1.M3 integration-parity tests.

**Use Case**: An agent hangs silently (dead socket — zero output, zero reads): today it burns the full
120s wall clock; with the idle engine (once armed, 60s default) it is killed at 60s of true silence
with the SAME exit-124/rescue UX — while a productive-but-slow agent (16KB/0.5s of reads, buffered
stdout) runs to completion (FR-A2).

**User Journey**: (post-S3) agent goes silent → watchIdle's clock expires → cancelRun() → cmd.Cancel
SIGTERMs the process group (WaitDelay 3s → SIGKILL) → Execute returns ErrIdleDeadline → downstream
`errors.Is(err, context.DeadlineExceeded)` fires exactly as a timeout → exit 124 + rescue;
`--verbose` adds `DEBUG: cause: idle deadline (1m0s without observable activity)`.

**Pain Points Addressed**: FR-A1/§9.30's measured basis — a wall clock cannot distinguish hung from
productive-slow (print-mode CLIs emit zero stdout for 18–19s healthy runs); byte activity can.

## Why

- **FR-A1–A3 (§9.30)**: the idle deadline is the primary kill criterion; activity = the more recent of
  pipe writes (portable) and Linux group rchar growth (covers silent print modes); ONE clock, whichever
  channel moved last. This task is the engine that implements exactly that.
- **D1–D6 (locked design decisions, system_context.md — binding)**: D1 struct-not-duration; D2 the
  Is()-sentinel preserving the error contract EXACTLY; D3 the channel design; D4 the arming rule
  ("stagecoach never fabricates activity" — non-Linux + print mode degrades to the hard cap alone,
  i.e. exactly v3.4); D6 the cause line. This task implements D1–D4 + D6 (D5 config landed in P1.M1).
- **FR-A5/D10**: exit codes, rescue, CAS, lock, index, freeze semantics UNTOUCHED — an idle kill is a
  timeout downstream; only the verbose cause line differs.

## What

**User-visible behavior**: none in this task (idle disarmed at every call site ⇒ byte-identical
v3.4 behavior). The engine + the mechanical signature migration land here; S3 arms it.

**Technical change** (verbatim in the Blueprint):
1. `Deadlines{Idle, Hard time.Duration}` + `ErrIdleDeadline` sentinel in executor.go.
2. Execute: Hard keeps the WithTimeout SHADOW; NEW `runCtx, cancelRun := context.WithCancel(ctx)` and
   `exec.CommandContext(runCtx, …)`; countingWriters for stdout/stderr; the D4-armed `go watchIdle(…)`;
   the Wait-error branch gains the `idleKilled` → ErrIdleDeadline path + the hard-cap cause line.
3. `watchIdle(ctx, cancel, killed, idle, pgid, out, errb)` — 500ms ticker; last = max(armed,
   out.LastWrite, errb.LastWrite, readMovedAt); rchar positive-delta ⇒ readMovedAt=now; expiry ⇒
   killed.Store(true) + cancel().
4. `CmdSpec.ProgressiveStdout bool` (render.go).
5. `ui.VerboseCause(reason)` (mirrors VerboseWarn).
6. models.go:149 → `provider.Deadlines{Idle: 0, Hard: timeout}` + the not-a-role-call comment; the 13
   role sites → `provider.Deadlines{Hard: X}`; the 23 test calls mechanically.

### Success Criteria
- [ ] `Execute` signature is `(ctx, spec CmdSpec, dl Deadlines, vb *ui.Verbose) (string, string, error)`
- [ ] `Deadlines` + `ErrIdleDeadline` defined in executor.go; Is() returns true for context.DeadlineExceeded
- [ ] countingWriters on cmd.Stdout/Stderr; every return uses `string(w.Bytes())`
- [ ] watcher armed iff `dl.Idle > 0 && (runtime.GOOS == "linux" || spec.ProgressiveStdout)`
- [ ] idle kill ⇒ ErrIdleDeadline + VerboseCause("idle deadline (…)"); hard cap ⇒ bare
      context.DeadlineExceeded + VerboseCause("hard cap (…)"); parent cancel ⇒ Canceled (no cause line)
- [ ] `CmdSpec.ProgressiveStdout bool` added (keyed literals only — verified safe)
- [ ] `ui.VerboseCause` + tests; mirrors the VerboseWarn idiom
- [ ] all 14 non-test sites pass Deadlines (models.go with the comment); 23 test calls updated
- [ ] the 4 new tests pass; existing provider/stubtest/ui/cmd suites green; -race green
- [ ] `go build ./...` + GOOS=darwin/windows provider builds + make test + make lint green

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim reworked Execute + watchIdle + sentinel + struct + VerboseCause code, the exact
14-site + 23-test-call mechanical-edit table, the S1 consumption contract (countingWriter.LastWrite
zero-time semantics; groupReadBytes ok==false ⇒ pipe-only; the delta-clamp duty), the D1–D6/D10
decision text, the test designs (including WHY ProgressiveStdout is forced in the portable tests and
why the linux test leaves it unset), the doc-comment rewrite, and the explicit scope fences (S1/S3/
P2/config/docs/default_action.go all excluded).

### Documentation & References

```yaml
# MUST READ — the authoritative research (verbatim code + the 14-site table + test designs)
- docfile: plan/022_856008a8cbb5/P1M2T1S2/research/findings.md
  why: "§1 = THE scope resolution (all 14 sites must be updated mechanically or the tree won't compile;
        anchored in D1 'All 13 role call sites updated mechanically') + the exact site/test table. §2 =
        every load-bearing detail of current Execute (shadow, buffers, kill path, error branch). §3 =
        the verbatim reworked Execute + watchIdle + sentinel. §4 = ProgressiveStdout placement. §5 =
        VerboseCause. §6 = the doc-comment rewrite. §8 = the test designs with their channel logic."
  critical: "§1: the contract names ONLY models.go among callers, but the signature change breaks all
             14 — S2 updates them ALL mechanically (Idle disarmed = byte-identical behavior), keeping
             go build/make test green; S3 later replaces the role sites' Deadlines{Hard:X} with
             ResolveRoleDeadlines. §3: idleKilled is checked BEFORE runCtx.Err() (an idle cancelRun
             makes runCtx.Err()==Canceled — the flag, not the ctx, distinguishes idle)."

# MUST READ — the locked design decisions (D1–D10, binding)
- docfile: plan/022_856008a8cbb5/architecture/system_context.md
  why: "'Locked design decisions' §(lines 30–55): D1 (struct + mechanical call-site update), D2 (the
        Is()-sentinel preserving the error contract EXACTLY), D3 (channels + one watcher + ctx.Done
        exit), D4 (arming rule + 'never fabricates activity'), D6 (VerboseCause wording). Also the
        maintainer-measured facts (print-mode zero-stdout 18–19s healthy runs; rchar ~16KB/0.5s; 13–14s
        worst legitimate silence) and the decomposition shape (S2 vs S3 split)."
  critical: "D4's degradation clause is the behavior contract: non-Linux + print-mode ⇒ idle disarmed ⇒
             EXACTLY v3.4 behavior — never worse. D2: only the verbose cause line differs downstream."

# MUST READ — the S1 PRP (the countingWriter/groupReadBytes contract this task consumes)
- docfile: plan/022_856008a8cbb5/P1M2T1S1/PRP.md
  why: "S1 ships activity.go (countingWriter: Write/LastWrite/Bytes — LastWrite is zero time.Time when
        never written; Bytes() is a snapshot COPY), activity_linux.go (groupReadBytes(pgid) (uint64,
        bool) — rchar sum; ok==false ONLY when /proc is unavailable), activity_other.go ((0,false)
        stub), + tests. S2 is the FIRST production consumer. S1's Integration Points name S2's duties:
        poll groupReadBytes(cmd.Process.Pid) every ~500ms; ok==false ⇒ countingWriter only; CLAMP
        negative rchar deltas (non-atomic counters — the DELTA consumer's job); baseline
        max(LastWrite, armTime)."
  critical: "If S1 hasn't landed, executor.go won't compile (countingWriter undefined) — the expected
             prerequisite signal, not a bug. Do NOT edit activity*.go (S1's files)."

# MUST READ — the file being reworked
- file: internal/provider/executor.go
  why: "Execute @65; the WithTimeout SHADOW @66–70 ('SHADOW — do not rename' — KEEP it, now keyed on
        dl.Hard); the bytes.Buffers @76–78 (→ countingWriters); setupProcessGroup/RegisterChild/
        ClearChild @82–90 (unchanged — the watcher reuses the SAME kill path); the Wait-error branch
        @92–99 (gains the idleKilled path + hard-cap cause line). StderrHint @23–38 is UNTOUCHED."
  pattern: "returns are BARE sentinels on deadline/cancel; wrapped *exec.ExitError otherwise; stdout/
            stderr returned even on error (now string(w.Bytes()) — the snapshot copy)."
  gotcha: "cmd := exec.CommandContext(runCtx, …) — NOT ctx (the watcher must be able to cancel the
           command through the SAME path). The deferred cancelRun() both prevents leaks and ends the
           watcher goroutine at Execute return (≤1 stray tick, harmless — groupReadBytes is read-only)."

# MUST READ — the call-site + error-contract inventory
- docfile: plan/022_856008a8cbb5/architecture/call_sites_and_error_contract.md
  why: "§1 the seam + the verbatim error contract; §2 the 14-site table with each site's timeout
        expression and error branch (all downstream branches are errors.Is(err,
        context.DeadlineExceeded) — the D2 sentinel satisfies them unchanged); §2.1–2.5 the verbatim
        call-site code (generate/multiturn/workdesc/planner/stager/message/arbiter/hook) this task
        edits MECHANICALLY; the test-harness section (mustBin idiom)."
  critical: "The arbiter site (arbiter.go:101) treats DeadlineExceeded‖Canceled as graceful-null — the
             D2 sentinel flows through that branch identically. No downstream file changes AT ALL in
             this task beyond the mechanical signature arg."

# MUST READ — where ProgressiveStdout goes
- file: internal/provider/render.go
  why: "CmdSpec @32 (Command/Args/Stdin/PayloadBytes/Env — ALL literals everywhere are keyed, verified:
        adding a field breaks nothing). Add ProgressiveStdout last with the D4 comment. Render itself
        is NOT changed (P2.M1.T1.S1 sets it from the event_stream manifest field — false everywhere
        until then)."

# MUST READ — the Verbose method idiom to mirror
- file: internal/ui/verbose.go
  why: "VerboseCommand @51 / VerboseWarn @123: the exact nil-guard + mutex + fmt.Fprintln pattern
        VerboseCause copies. NewVerbose(w, on). The DEBUG: prefix convention."
  gotcha: "default_action.go:381 already prints 'DEBUG: cause: '+re.Cause.Error() directly (no Verbose
           instance) — LEAVE IT (out of scope); VerboseCause is a NEW method invoked only by Execute."

# MUST READ — the test harness idiom
- file: internal/provider/executor_test.go
  why: "mustBin(t, names…) skip-guard; real PATH binaries (cat/sleep/printenv); TestExecute_Timeout
        KillsProcess @~44 (the hard-cap keystone — UPDATE its signature + add the !ErrIdleDeadline
        assert); TestExecute_Verbose (the verbose-assert pattern for the ui tests). 11 Execute calls
        to update (same package — no provider. prefix)."

# MUST READ — the FR-A spec
- docfile: plan/022_856008a8cbb5/prd_snapshot.md
  why: "§9.30 FR-A1–A5 verbatim: the activity definition (more-recent-of pipe writes + rchar growth),
        the hard-cap rationale (codex heartbeat ~27B/2s + 1.1MB re-reads — activity ≠ progress), the
        arming degradation ('where no channel exists … only the hard cap applies — degrades to exactly
        v3.4's'), FR-A5 (idle == timeout downstream; only the cause line differs)."
  section: "§9.30 (FR-A1–A5)"

# CONTEXT — the second test caller (mechanical update)
- file: internal/stubtest/stubtest_test.go
  why: "12 provider.Execute( calls (prefixed — different package). Same mechanical Deadlines{Hard: X}
        form. Run its package's tests after the edit."

# CONTEXT — what S3 will build on (NOT this task)
- file: internal/config/roles.go
  why: "ResolveRoleDeadlines(role, cfg) (idle, hard) @78 EXISTS (P1.M1.T1.S1 Complete; defaults 60s/600s
        via config.go:230). S3 threads the 13 role sites to it; THIS task only leaves the mechanical
        Deadlines{Hard: X} placeholders. Do NOT import config into provider for this (provider must
        stay config-free — the struct keeps resolution at call sites, per D1)."
```

### Current Codebase tree (relevant slice)

```bash
internal/provider/
  executor.go          # EDIT — Deadlines + ErrIdleDeadline + reworked Execute + watchIdle + doc
  render.go            # EDIT — CmdSpec +ProgressiveStdout (Render untouched)
  executor_test.go     # EDIT — 11 signature updates + hard-cap assert + 2 new portable tests (+1 linux-tagged)
  activity*.go         # READ-ONLY / S1's (in flight) — countingWriter + groupReadBytes twins
  procgroup_*.go       # READ-ONLY — setupProcessGroup (the kill path the watcher reuses)
internal/ui/
  verbose.go           # EDIT — +VerboseCause
  verbose_test.go      # EDIT — +TestVerbose_Cause* (mirror TestVerbose_Warn*)
internal/stubtest/
  stubtest_test.go     # EDIT — 12 mechanical signature updates
internal/cmd/models.go # EDIT — the Execute call (Deadlines{Idle:0, Hard: timeout} + comment)
internal/decompose/    # EDIT (mechanical) — arbiter.go:101 message.go:173 planner.go:128 stager.go:119
internal/generate/     # EDIT (mechanical) — generate.go:374 multiturn.go:172/183/194 workdesc.go:82/131/149
internal/hook/exec.go  # EDIT (mechanical) — :187
pkg/stagecoach/stagecoach.go  # EDIT (mechanical) — :569
internal/config/       # READ-ONLY — ResolveRoleDeadlines exists (S3 threads it; NOT this task)
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (no new files):
internal/provider/executor.go         # +Deadlines +ErrIdleDeadline +watchIdle; Execute rework
internal/provider/render.go           # CmdSpec +ProgressiveStdout bool
internal/provider/executor_test.go    # signature updates + TestExecute_Timeout assert + 3 new tests
internal/ui/verbose.go                # +VerboseCause
internal/ui/verbose_test.go           # +TestVerbose_Cause* (4 variants)
internal/stubtest/stubtest_test.go    # 12 mechanical updates
internal/cmd/models.go                # the call: Deadlines{Idle: 0, Hard: timeout} + comment
internal/decompose/{arbiter,message,planner,stager}.go   # mechanical Deadlines{Hard: X}
internal/generate/{generate,multiturn,workdesc}.go       # mechanical Deadlines{Hard: X}
internal/hook/exec.go                 # mechanical
pkg/stagecoach/stagecoach.go          # mechanical
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (ALL call sites must move in the SAME change or nothing compiles): the signature change
//   breaks 14 non-test sites + 23 test calls. Update them ALL mechanically (findings §1's table):
//   `provider.Execute(ctx, *spec, X, deps.Verbose)` → `provider.Execute(ctx, *spec, provider.Deadlines{Hard: X}, deps.Verbose)`.
//   Idle zero-value = disarmed = byte-identical v3.4 behavior. S3 replaces the 13 role sites with
//   ResolveRoleDeadlines threading; models.go (not a role call) keeps Idle: 0 permanently + the comment.

// CRITICAL (check idleKilled BEFORE runCtx.Err()): the watcher kills via cancelRun(), which makes
//   runCtx.Err() == context.Canceled — indistinguishable from a parent cancel at the ctx level. The
//   atomic.Bool flag (set BEFORE cancel()) is what distinguishes idle. Order: idleKilled.Load() →
//   ErrIdleDeadline; else runCtx.Err() (DeadlineExceeded → hard-cap cause line; Canceled → bare);
//   else the wrapped ExitError.

// CRITICAL (keep the WithTimeout SHADOW, keyed on dl.Hard): the existing `ctx, cancel = context.
//   WithTimeout(ctx, …)` shadow is load-bearing (the doc says "do not rename") — the later ctx.Err()
//   reads the timeout ctx. Keep it exactly, then wrap with runCtx := context.WithCancel(ctx) and use
//   exec.CommandContext(runCtx, …). Two layers: hard cap on ctx, idle+everything on runCtx — both
//   reach the SAME cmd.Cancel group-kill (SIGTERM -pid, WaitDelay 3s → SIGKILL).

// CRITICAL (the D2 sentinel direction): errors.Is(err, target) calls err.Is(target). Define
//   `func (e *idleDeadlineError) Is(target error) bool { return target == context.DeadlineExceeded }`
//   so errors.Is(ErrIdleDeadline, context.DeadlineExceeded) == true. Pointer-singleton var
//   (`var ErrIdleDeadline = &idleDeadlineError{}`) — errors.Is(err, ErrIdleDeadline) also works
//   (identity first). Downstream branches stay UNTOUCHED (arbiter's graceful-null, exit-124, FR-T7).

// CRITICAL (rchar deltas: CLAMP, don't trust): groupReadBytes returns a CUMULATIVE sum over live
//   members — a member exiting REDUCES it; the counters are non-atomic. Only `sum > lastRchar`
//   (strictly positive delta) counts as activity; negatives/zero mean nothing. The timestamp for
//   read activity is the tick that OBSERVED the growth (granularity 500ms — fine for 60s deadlines).

// CRITICAL (the first tick initializes, doesn't judge): haveRchar starts false — the first
//   groupReadBytes sample only records the baseline; no delta judgment until tick 2. The idle
//   baseline is armed := time.Now() (start); never-written countingWriters have zero LastWrite —
//   time.Since(zero) is huge, so max() with armed is REQUIRED (S1's zero-time contract).

// GOTCHA (os/exec io.Writer piping is concurrency-safe here by design): assigning countingWriters to
//   cmd.Stdout/Stderr makes os/exec spawn copy goroutines writing WHILE watchIdle reads LastWrite/
//   Bytes — exactly the mutex+atomic discipline S1 built. Verify under -race.

// GOTCHA (provider must NOT import internal/config): D1 keeps resolution at call sites (the struct).
//   The 13 role sites keep resolving via their EXISTING ResolveRoleTimeout expressions inside
//   Deadlines{Hard: X}; S3 swaps them to ResolveRoleDeadlines. No new import edges in provider.

// GOTCHA (the watcher's tail race is benign): after Execute returns, deferred cancelRun() closes
//   runCtx.Done() and the goroutine exits within ≤1 tick; a stray groupReadBytes on a recycled pgid
//   is a read-only /proc sweep (harmless — the same discipline as signal.ClearChild's defer).

// GOTCHA (portable idle tests MUST force ProgressiveStdout: true): off Linux with
//   ProgressiveStdout=false the watcher is DISARMED (D4) and a silent stub would run to the hard cap
//   (30s of test hang). Forcing the field arms the pipe channel on every platform — that is the
//   contract's explicit test design ("any platform with ProgressiveStdout forced true in the test spec").
//   The linux-tagged test deliberately leaves it UNSET — it proves the rchar channel alone keeps a
//   stdout-silent reader alive.

// GOTCHA (Duration formatting in the cause line): D6's examples ("60s"/"600s") are illustrative.
//   fmt.Sprintf("idle deadline (%v without observable activity)", dl.Idle) renders "1m0s" for 60s —
//   acceptable (the spec wording is "e.g."). Keep the two message SHAPES: "idle deadline (<d> without
//   observable activity)" and "hard cap (<d>)".

// GOTCHA (build tags for the linux test): the read-counter test file section carries
//   `//go:build linux` (the procgroup_windows_test.go / S1's activity_linux_test.go precedent).
//   The two portable tests live untagged in executor_test.go. GOOS=darwin/windows provider builds
//   must stay clean (groupReadBytes resolves to the (0,false) stub off Linux).
```

## Implementation Blueprint

### Data models and structure

```go
// executor.go — above Execute
// Deadlines carries the two §9.30 generation deadlines (FR-A1/FR-A2), resolved per role at the call
// site via config.ResolveRoleDeadlines (FR-R7 — provider stays config-free; D1). Both bind each
// invocation individually (FR-A4): a fresh idle clock per Execute call.
type Deadlines struct {
	Idle time.Duration // kill after this long with NO observable activity (FR-A1); ≤0 ⇒ not armed
	Hard time.Duration // absolute wall-clock backstop (FR-A2); ≤0 ⇒ unbounded (today's timeout==0)
}

// ErrIdleDeadline is returned when the idle watcher killed the agent's process group (§9.30 FR-A1).
// Its Is method satisfies errors.Is(err, context.DeadlineExceeded) (D2/FR-A5), so EVERY existing
// downstream branch — exit-124 mapping, ErrTimeout rescue, the no-retry-on-timeout rule, FR-T7
// multi-turn aborts, the arbiter's graceful-null — treats an idle kill exactly as a hard-cap timeout.
// Only the --verbose cause line (ui.VerboseCause) distinguishes them.
var ErrIdleDeadline = &idleDeadlineError{}

type idleDeadlineError struct{}

func (e *idleDeadlineError) Error() string {
	return "context deadline exceeded (idle: no observable agent activity)"
}
func (e *idleDeadlineError) Is(target error) bool { return target == context.DeadlineExceeded }

// render.go — CmdSpec gains (last field)
	// ProgressiveStdout reports whether the rendered invocation emits stdout progressively
	// (event-stream mode, §9.30 FR-S1). Set by Render iff the manifest declares event_stream tokens
	// (P2.M1.T1.S1 — always false until then, so the idle watcher arms only on Linux, where the
	// rchar channel covers every provider). Arms the portable pipe-activity idle channel (FR-A3(a)/D4).
	// Stagecoach never fabricates activity.
	ProgressiveStdout bool
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/provider/render.go — add CmdSpec.ProgressiveStdout (last field, D4 comment).
  - All CmdSpec literals repo-wide are KEYED (verified) — additive, breaks nothing.

Task 2: EDIT internal/provider/executor.go — Deadlines + ErrIdleDeadline + the reworked Execute + watchIdle
  - ADD the Deadlines struct + ErrIdleDeadline sentinel (Blueprint §Data models) above Execute.
  - REWORK Execute (findings §3 verbatim): signature (ctx, spec, dl Deadlines, vb); dl.Hard>0 keeps the
    WithTimeout SHADOW; runCtx,cancelRun := context.WithCancel(ctx) (+defer); CommandContext(runCtx,…);
    var out, errb countingWriter (+&out/&errb assignments); after RegisterChild:
      var idleKilled atomic.Bool
      if dl.Idle > 0 && (runtime.GOOS == "linux" || spec.ProgressiveStdout) {
          go watchIdle(runCtx, cancelRun, &idleKilled, dl.Idle, cmd.Process.Pid, &out, &errb)
      }
    Wait-error branch: dumps (string(out.Bytes())/string(errb.Bytes())); idleKilled.Load() →
    VerboseCause("idle deadline (%v without observable activity)") + return …, ErrIdleDeadline;
    ctxErr := runCtx.Err() → DeadlineExceeded: VerboseCause("hard cap (%v)") + return bare; Canceled:
    return bare; else the ExitError wrap. Success: dumps + nil.
  - ADD watchIdle (findings §3 verbatim): 500ms ticker; armed baseline; pipe LastWrites; groupReadBytes
    positive-delta → readMovedAt; expiry → killed.Store(true) + cancel().
  - REWRITE the doc comment (findings §6): deadlines paragraph + the new error-contract rows.
  - IMPORTS: drop bytes; add errors, runtime, sync/atomic.

Task 3: EDIT internal/ui/verbose.go + verbose_test.go — VerboseCause
  - The method (findings §5 verbatim — the VerboseWarn idiom). Tests: TestVerbose_CauseWhenOn /
    WhenOff / NilSafe / NilWriterNoOp mirroring TestVerbose_Warn* (@78–105).

Task 4: EDIT the 14 non-test call sites (mechanical; findings §1 table)
  - The 13 role sites: `provider.Deadlines{Hard: X}` where X is the site's EXISTING duration
    expression (arbiterTimeout/messageTimeout/plannerTimeout/stagerTimeout/msgTimeout/cfg.Timeout) —
    behavior byte-identical (Idle zero ⇒ disarmed). NO other change at these sites.
  - models.go:149: `provider.Deadlines{Idle: 0, Hard: timeout}` + comment:
      // Not a role call — idle is intentionally unarmed (a short models probe; the hard cap alone
      // bounds it). P1.M2.T1.S3 threads the 13 role sites to ResolveRoleDeadlines.

Task 5: EDIT the test files (mechanical + new)
  - executor_test.go: 11 calls → Deadlines{} (for 0) / Deadlines{Hard: X}. TestExecute_TimeoutKills
    Process gains: `if errors.Is(err, ErrIdleDeadline) { t.Errorf("hard cap must stay the bare sentinel") }`.
  - stubtest_test.go: 12 provider.Execute calls → provider.Deadlines{Hard: X}.
  - NEW (executor_test.go, portable): TestExecute_IdleKillsSilentStub — sleep 30, ProgressiveStdout:
    true, Deadlines{Idle: 300ms, Hard: 30s} → errors.Is(err, ErrIdleDeadline) AND errors.Is(err,
    context.DeadlineExceeded) AND elapsed < 5s (findings §8.2).
  - NEW (executor_test.go, portable): TestExecute_IdleSparesBusyEmitter — sh -c "for i in 1 2 3 4 5 6
    7 8; do echo x; sleep 0.4; done", ProgressiveStdout: true, Deadlines{Idle: 900ms, Hard: 30s} →
    err==nil; 8 "x" lines in out (findings §8.3).
  - NEW (a `//go:build linux` section/file): TestExecute_IdleReadCounterKeepsReaderAlive — sh -c
    "while IFS= read -r line; do sleep 0.3; done", Stdin 10 lines, ProgressiveStdout UNSET,
    Deadlines{Idle: 900ms, Hard: 30s} → err==nil (findings §8.4; on Linux the rchar channel alone
    must keep the stdout-silent reader alive).

Task 6: VERIFY — builds (native + GOOS), vet, format, focused + race tests, lint, grep guards
  - go build ./... ; GOOS=darwin go build ./internal/provider/ ; GOOS=windows go build ./internal/provider/
  - go vet ./internal/provider/... ./internal/ui/... ; gofmt -l <touched files>
  - go test ./internal/provider/ -run 'Execute' -v ; go test ./internal/stubtest/ -v ;
    go test ./internal/ui/ -run 'Cause' -v ; go test -race ./internal/provider/... ./internal/ui/...
  - make test ; make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the two-layer ctx + the watcher arm (the D3/D4 core)
	if dl.Hard > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, dl.Hard) // SHADOW — keep (doc comment explains)
		defer cancel()
	}
	runCtx, cancelRun := context.WithCancel(ctx) // the idle watcher's kill path — SAME cmd.Cancel
	defer cancelRun()
	cmd := exec.CommandContext(runCtx, spec.Command, spec.Args...)
	…
	var idleKilled atomic.Bool
	if dl.Idle > 0 && (runtime.GOOS == "linux" || spec.ProgressiveStdout) { // D4 — never fabricates activity
		go watchIdle(runCtx, cancelRun, &idleKilled, dl.Idle, cmd.Process.Pid, &out, &errb)
	}

// PATTERN: the Wait-error discrimination (idleKilled FIRST — a cancelRun makes runCtx.Err() Canceled)
	if werr := cmd.Wait(); werr != nil {
		vb.VerboseRawOutput(string(out.Bytes()))
		vb.VerboseStderr(string(errb.Bytes()))
		if idleKilled.Load() {
			vb.VerboseCause(fmt.Sprintf("idle deadline (%v without observable activity)", dl.Idle))
			return string(out.Bytes()), string(errb.Bytes()), ErrIdleDeadline
		}
		if ctxErr := runCtx.Err(); ctxErr != nil {
			if errors.Is(ctxErr, context.DeadlineExceeded) {
				vb.VerboseCause(fmt.Sprintf("hard cap (%v)", dl.Hard))
			}
			return string(out.Bytes()), string(errb.Bytes()), ctxErr
		}
		return string(out.Bytes()), string(errb.Bytes()), fmt.Errorf("provider %q: %w", spec.Command, werr)
	}

// PATTERN: watchIdle's single clock (findings §3 for the full body)
//   last := armed; pipe LastWrites max in; rchar positive-delta stamps readMovedAt=now;
//   readMovedAt maxed in; now.Sub(last) >= idle ⇒ killed.Store(true); cancel(); return

// PATTERN: the mechanical call-site edit (13 role sites)
//   BEFORE: provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
//   AFTER:  provider.Execute(ctx, *spec, provider.Deadlines{Hard: msgTimeout}, deps.Verbose)
//   (models.go uses the explicit Idle: 0 form + the comment — the contract's named site)
```

### Integration Points

```yaml
NO config/schema/flag/docs/exit-code/rescue/CAS/lock changes (D10). provider gains NO new import
edges (still NO internal/config import — D1 keeps resolution at call sites).

PROVIDER (internal/provider):
  - executor.go: Deadlines + ErrIdleDeadline + reworked Execute + watchIdle (the engine).
  - render.go: CmdSpec.ProgressiveStdout (set only by P2.M1.T1.S1's Render; false everywhere now).

UI (internal/ui):
  - VerboseCause(reason) — the D6 cause line; invoked ONLY by Execute (default_action.go:381's
    existing fmt.Fprintln rescue-cause line stays as-is).

CALL SITES (mechanical, behavior-preserving):
  - 13 role sites → Deadlines{Hard: X} (S3 replaces with ResolveRoleDeadlines threading).
  - models.go → Deadlines{Idle: 0, Hard: timeout} + the not-a-role-call comment (S3 never touches it).

DOWNSTREAM (UNCHANGED — the D2/FR-A5 guarantee): exit-124 mapping, ErrTimeout rescue, no-retry,
FR-T7 aborts, the arbiter's graceful-null — all keep their existing errors.Is(err,
context.DeadlineExceeded) branches; the sentinel satisfies them.

SCOPE FENCES: NO activity*.go (S1's); NO config/* (P1.M1 done); NO S3 threading; NO Render/event_stream
(P2.M1.T1.S1); NO docs (S3's Mode-A docs ride with the call sites); NO default_action.go:381 change.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native build — the 14-site mechanical migration MUST compile.
go build ./...
# Expected: clean. (A failure names a call site you missed — grep: `grep -rn 'provider.Execute(' --include='*.go' internal/ pkg/ cmd/ | grep -v _test.go` = 14 hits, all passing Deadlines.)

# Cross-platform provider builds — the watcher + stub must compile everywhere.
GOOS=darwin  go build ./internal/provider/
GOOS=windows go build ./internal/provider/
# Expected: clean (groupReadBytes resolves to the (0,false) stub off Linux; runtime.GOOS arming is
#           platform-agnostic code).

# Vet + format.
go vet ./internal/provider/... ./internal/ui/...
gofmt -l internal/provider/executor.go internal/provider/render.go internal/provider/executor_test.go internal/ui/verbose.go internal/ui/verbose_test.go
# Expected: empty.

# Lint.
make lint
# Expected: zero errors (`unused` clean — watchIdle/ErrIdleDeadline/Deadlines all referenced).

# Scope guard: the expected file set only.
git diff --name-only
# Expected: executor.go, render.go, executor_test.go (provider); verbose.go, verbose_test.go (ui);
#           stubtest_test.go; models.go; arbiter/message/planner/stager (decompose); generate/
#           multiturn/workdesc (generate); hook/exec.go; stagecoach.go (pkg) — nothing else.
```

### Level 2: Unit Tests (Component Validation)

```bash
# THE D2 keystone + the new engine tests.
go test ./internal/provider/ -run 'Execute' -v
# Expected: PASS — TestExecute_IdleKillsSilentStub (errors.Is BOTH ErrIdleDeadline AND
#           DeadlineExceeded; <5s), TestExecute_IdleSparesBusyEmitter (err==nil, 8 lines),
#           TestExecute_TimeoutKillsProcess (updated: bare DeadlineExceeded, NOT ErrIdleDeadline),
#           all existing tests green on the new signature.

# The linux rchar-channel proof (runs on Linux CI; absent elsewhere).
go test ./internal/provider/ -run 'IdleReadCounter' -v
# Expected (Linux): PASS — the stdout-silent stdin-reader survives past idle via the read counter.

# The ui helper.
go test ./internal/ui/ -run 'Cause' -v
# Expected: PASS — On/Off/NilSafe/NilWriterNoOp.

# The mechanical test migration.
go test ./internal/stubtest/ -v
# Expected: green.

# THE concurrency gate (watcher goroutine × countingWriter copy goroutines).
go test -race ./internal/provider/... ./internal/ui/...
# Expected: green.

# Full repo.
make test
# Expected: green (all 14 call-site packages compile + pass on the mechanical Deadlines{Hard:X}).
```

### Level 3: Integration Testing (System Validation)

```bash
# A verbose end-to-end sanity of the cause line (portable, forced-progressive):
cat > /tmp/idle_e2e_test.go <<'EOF'
package provider
import ("bytes"; "context"; "errors"; "strings"; "testing"; "time"; "os/exec"
        "github.com/dabstractor/stagecoach/internal/ui")
func TestScratchIdleCauseLine(t *testing.T) {
    mustBin(t, "sleep")
    var buf bytes.Buffer
    vb := ui.NewVerbose(&buf, true)
    spec := CmdSpec{Command: "sleep", Args: []string{"30"}, ProgressiveStdout: true}
    _, _, err := Execute(context.Background(), spec, Deadlines{Idle: 300*time.Millisecond, Hard: 30*time.Second}, vb)
    _ = exec.Command // (import anchor)
    if !errors.Is(err, ErrIdleDeadline) { t.Fatalf("err=%v want ErrIdleDeadline", err) }
    if !strings.Contains(buf.String(), "DEBUG: cause: idle deadline") { t.Errorf("cause line missing:\n%s", buf) }
    t.Log(buf.String())
}
EOF
cp /tmp/idle_e2e_test.go internal/provider/zz_scratch_test.go
go test ./internal/provider/ -run TestScratchIdleCauseLine -v && rm internal/provider/zz_scratch_test.go
# Expected: PASS — the DEBUG: cause line present; scratch file removed. (The permanent coverage is
#           Level 2; this is the visible-cause-line confidence check.)

# Hard-cap cause line (same shape, Deadlines{Idle:0, Hard:200ms} over sleep 30 → "DEBUG: cause: hard cap").
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1: the signature is migrated everywhere — no bare-duration Execute call remains.
grep -rn 'provider.Execute(' --include='*.go' internal/ pkg/ cmd/ | grep -v 'Deadlines'
grep -rn '= Execute(\|:= Execute(\| Execute(context\| Execute(ctx' --include='*_test.go' internal/provider/ internal/stubtest/ | grep -v 'Deadlines'
# Expected: both empty (every call passes Deadlines — prefixed or same-package form).

# Grep guard 2: the sentinel + its Is method.
grep -n 'ErrIdleDeadline = &idleDeadlineError{}\|func (e \*idleDeadlineError) Is(target error) bool { return target == context.DeadlineExceeded }' internal/provider/executor.go
# Expected: both present.

# Grep guard 3: the D4 arming predicate is exactly the locked rule.
grep -n 'dl.Idle > 0 && (runtime.GOOS == "linux" || spec.ProgressiveStdout)' internal/provider/executor.go
# Expected: 1 hit.

# Grep guard 4: runCtx wraps CommandContext (the shared kill path).
grep -n 'runCtx, cancelRun := context.WithCancel(ctx)\|exec.CommandContext(runCtx' internal/provider/executor.go
# Expected: 2 hits.

# Grep guard 5: countingWriters + Bytes() returns.
grep -c 'var out, errb countingWriter\|string(out.Bytes())\|string(errb.Bytes())' internal/provider/executor.go
# Expected: ≥6 (declaration + every return path).

# Grep guard 6: ProgressiveStdout field + no Render writer of it yet.
grep -n 'ProgressiveStdout bool' internal/provider/render.go
grep -n 'ProgressiveStdout' internal/provider/render.go | grep -v '//'
# Expected: the field decl; and NO assignment in Render (only tests set it — P2.M1.T1.S1 wires Render).

# Grep guard 7: VerboseCause exists and is invoked by Execute only.
grep -rn 'VerboseCause' --include='*.go' internal/ | grep -v _test.go
# Expected: verbose.go (the method) + executor.go (2 invocation sites: idle + hard cap). NOT
#           default_action.go (its :381 fmt.Fprintln rescue line is untouched).

# Grep guard 8: provider gained no config import (D1).
grep -n 'internal/config' internal/provider/executor.go
# Expected: no hits.

# Grep guard 9: scope — the S1 files + docs + default_action untouched.
git diff --name-only | grep -E 'activity|\.md$|default_action'
# Expected: empty.

# Regression: the four downstream-branch packages still green on the mechanical migration.
go test ./internal/generate/... ./internal/decompose/... ./internal/hook/... ./internal/cmd/... ./pkg/stagecoach/... 2>&1 | tail -8
# Expected: all ok (idle disarmed ⇒ byte-identical behavior).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean (all 14 sites migrated)
- [ ] `GOOS=darwin` + `GOOS=windows` provider builds clean
- [ ] `go vet` + `gofmt -l` clean on all touched files
- [ ] `go test -race ./internal/provider/... ./internal/ui/...` green
- [ ] `make test` + `make lint` green

### Feature Validation
- [ ] `Execute(ctx, spec, dl Deadlines, vb)` signature live; Deadlines{Idle,Hard} + ErrIdleDeadline defined
- [ ] silent-stub idle kill: `errors.Is(err, ErrIdleDeadline)` AND `errors.Is(err, context.DeadlineExceeded)` (D2 keystone), <5s
- [ ] busy-emitter survives idle (FR-A2); linux read-counter keeps a stdout-silent reader alive (FR-A3(b))
- [ ] hard cap returns BARE DeadlineExceeded (not the idle sentinel); parent cancel returns Canceled
- [ ] `DEBUG: cause: idle deadline (…)` / `DEBUG: cause: hard cap (…)` on the respective kills (D6)
- [ ] watcher armed iff `Idle > 0 && (linux || ProgressiveStdout)`; disarmed ⇒ exactly v3.4 behavior
- [ ] `CmdSpec.ProgressiveStdout` added; Render never sets it (false everywhere until P2)
- [ ] models.go passes `Deadlines{Idle: 0, Hard: timeout}` with the not-a-role-call comment

### Scope-Boundary Validation
- [ ] The 13 role sites are MECHANICAL only (`Deadlines{Hard: X}` — no ResolveRoleDeadlines import/threading; S3's)
- [ ] activity*.go (S1), config/* (P1.M1), render's Render func (P2), docs (S3), default_action.go:381 untouched
- [ ] No exit-code/rescue/CAS/lock/index/freeze change (D10); provider imports no internal/config
- [ ] No downstream error-branch file changed (the sentinel satisfies them unchanged — D2)

### Code Quality & Docs
- [ ] Execute doc comment rewritten: "idle OR hard cap ⇒ errors.Is(err, context.DeadlineExceeded)"
- [ ] watchIdle documented (single clock; delta clamping; the ok==false fallback; the benign tail race)
- [ ] Cause-line message shapes match D6 ("idle deadline (<d> without observable activity)" / "hard cap (<d>)")
- [ ] Tests use the mustBin/real-binary harness idiom; the linux test carries `//go:build linux`

---

## Anti-Patterns to Avoid

- ❌ Don't change ONLY models.go and leave the 13 role sites on the old signature. The tree won't
  compile — `go build ./...` and every test gate fail. D1: "All 13 role call sites updated
  MECHANICALLY" — this task moves them all to `Deadlines{Hard: X}` (byte-identical behavior, idle
  disarmed); S3 later threads ResolveRoleDeadlines. (Findings §1 — the contract names models.go
  because it's the one site S3 will NEVER touch.)
- ❌ Don't check `runCtx.Err()` before `idleKilled`. The watcher kills via cancelRun(), which makes
  runCtx.Err() == context.Canceled — identical to a parent cancel at the ctx level. The atomic flag
  (set before cancel) is the discriminator. Order: flag → ErrIdleDeadline; then ctx.Err().
- ❌ Don't let the watcher judge rchar on the first sample. groupReadBytes is CUMULATIVE — the first
  tick only records the baseline (haveRchar=false → no delta). Also clamp: only STRICTLY positive
  deltas count (a member exiting reduces the sum; the counters are non-atomic — negative deltas are
  "no activity", never a clock reset, never an error).
- ❌ Don't baseline the idle clock at countingWriter's zero LastWrite. Never-written ⇒ zero time.Time
  ⇒ time.Since(zero) is huge ⇒ instant false kill. The baseline is `armed := time.Now()` at watcher
  start; max() it against every channel timestamp (S1's zero-time contract).
- ❌ Don't give the sentinel a plain string identity only. The ENTIRE point (D2/FR-A5) is the
  `Is(target error) bool { return target == context.DeadlineExceeded }` method — without it, every
  downstream errors.Is(err, context.DeadlineExceeded) branch misses and the arbiter/exit-124/rescue
  semantics break. Test it directly (the keystone test asserts BOTH errors.Is forms).
- ❌ Don't arm the watcher off the arming rule. D4 is mechanical: `Idle > 0 && (linux ||
  ProgressiveStdout)`. Non-Linux + print-mode stays DISARMED (v3.4 behavior — "stagecoach never
  fabricates activity"). Don't "helpfully" arm it on stdout-only off Linux (a buffered print-mode agent
  would be false-killed — the exact bug FR-A1 exists to fix).
- ❌ Don't pass ctx (not runCtx) to exec.CommandContext. The watcher must reach the SAME cmd.Cancel
  group-kill as the hard cap — that's what the WithCancel layer is for. CommandContext(ctx) would make
  the idle cancel a no-op for the process.
- ❌ Don't drop the WithTimeout SHADOW. The existing `ctx, cancel = context.WithTimeout(ctx, …)`
  pattern (now keyed on dl.Hard) is load-bearing — the doc comment says so; the later ctx.Err()
  semantics depend on it. Keep the shadow, then layer runCtx on top.
- ❌ Don't emit the hard-cap cause line on a parent Canceled. Only DeadlineExceeded through runCtx is a
  "deadline" — a signal/parent cancel is not (no cause line; the existing behavior). The Canceled row
  of the error contract is unchanged.
- ❌ Don't import internal/config from provider. D1's struct-not-duration design keeps resolution at
  the call sites precisely so provider stays config-free. The 13 role sites keep their EXISTING
  ResolveRoleTimeout expressions inside Deadlines{Hard: X} — S3 swaps them.
- ❌ Don't write the portable idle tests without forcing ProgressiveStdout: true. Off Linux with it
  false, the watcher is disarmed and the silent-stub test would hang to the 30s hard cap. Conversely,
  DON'T set it in the linux read-counter test — leaving it unset is what proves the rchar channel alone.
- ❌ Don't touch activity*.go (S1's, in flight), Render's body (P2 sets ProgressiveStdout),
  config (P1.M1 done), the downstream error branches (D2 leaves them unchanged), docs (S3), or
  default_action.go:381 (its fmt.Fprintln rescue-cause line predates and stays beside VerboseCause).
- ❌ Don't anchor to line numbers (65/76/92/149/569…). They drift on the parallel S1/M1 landings.
  Locate by content: `grep -n 'func Execute'`, `grep -rn 'provider.Execute('`, `grep -n 'type CmdSpec'`.
- ❌ Don't use a mock/fake executor in the tests. The harness idiom is real PATH binaries behind
  mustBin (sleep, sh) — sh -c IS the stub-script mechanism the contract names ("stub shell scripts per
  the existing executor_test.go harness").

---

## Confidence Score: 9/10

Every load-bearing fact is grounded: the verbatim reworked Execute/watchIdle/sentinel designs (written
against the actual executor.go), the D1–D6/D10 locked decisions quoted, the S1 consumption contract
(zero-time baseline, ok==false fallback, delta-clamp duty — S1's Integration Points), the exact 14-site
+ 23-test-call mechanical-edit table (grep-verified, including pkg/stagecoach.go:569 and the
same-package unprefixed executor_test calls), the keyed-literal safety for the CmdSpec field
(verified), the VerboseWarn idiom to mirror, and the four new test designs with their channel logic
(why ProgressiveStdout is forced in two and unset in the linux one). The residuals (not a full 10): (a)
S1 is a parallel in-flight prerequisite — if it hasn't landed, executor.go won't compile (the expected
signal, and S1's PRP freezes the signatures); (b) the busy-emitter/read-counter tests carry inherent
timing sensitivity (400ms/300ms cadences vs 900ms idle — 2×+ margin, but a pathologically loaded CI
box could flake; the hard gates — the D2 keystone and the kill-fires gates — are race-free). One-pass
success is highly likely.
