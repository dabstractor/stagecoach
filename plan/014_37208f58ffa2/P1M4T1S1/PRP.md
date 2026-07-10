name: "P1.M4.T1.S1 — Orphaned-run lock reclamation e2e scenarios (FR-K1/K3/K4/K6)"
description: >
  A NEW e2e test file `internal/e2e/orphan_reclaim_scenarios_test.go` (build tag `//go:build e2e && !windows`)
  that exercises the LANDED FR-K1/K3/K4/K6 reclamation machinery end-to-end against REAL stagecoach
  subprocesses on REAL temp git repos (the §20.5 layer unit tests cannot reach: real flock across real
  processes, real signal delivery, real process reparenting). It CONSUMES — adds NO production code.
  Reuses the existing harness primitives (buildStagecoach/buildStub/newRepo/runStagecoach/waitForMarker/
  writeStubConfig/stubEnv/headSHA/commitCount/statusPorcelain) + the stub's STAGECOACH_STUB_MARKER +
  STAGECOACH_STUB_SLEEP_MS blocking pattern (NO new binary). FOUR scenarios: (a) parent-death watchdog
  self-exit (FR-K1) — a parent shell backgrounds stagecoach, sleeps past watchdog.Arm, exits → stagecoach
  reparented → watchdog fires → rescue exit 3 → lock released + HEAD/index unchanged; (b) SIGHUP rescue
  (FR-K3) — start stagecoach directly, waitForMarker (snapshot armed), syscall.Kill(stagecoachPid, SIGHUP)
  → rescue exit 3 → lock released + HEAD/index unchanged; (c) `stagecoach lock status` diagnostics (FR-K4)
  — no-lock / live / dead / (best-effort) genuine orphan; (d) STAGECOACH_NO_PARENT_WATCHDOG=1 opt-out
  (FR-K6) — repeat (a)'s wrapper, assert the watchdog does NOT fire and the commit LANDS. Determinism
  keys: §2 the snapshot is armed (generate.go:242) BEFORE Execute runs the stub (line 335) → SIGHUP +
  parent-death both reach the rescue **exit 3** path by marker time (NOT 129/143); §3 the parent MUST
  survive past watchdog.Arm() (the documented prctl/getppid race) so the wrapper sleeps ~1.5s before
  exiting; a reparented process can't be Wait'd by the test → poll syscall.Kill(pid,0)==ESRCH for "gone"
  and assert OUTCOMES (lock removed / HEAD unchanged), not the exit code, for (a). The genuine
  ppid==1 orphan (§9) is environment-dependent (subreaper) → guarded with a t.Skip when ppid≠1; the
  reliable core of (c) is no-lock/live/dead. Test-only; no docs (P1.M4.T2.S1 owns the README sync).

---

## Goal

**Feature Goal**: End-to-end regression coverage (PRD §20.5) for the §9.27 orphaned-run lock reclamation
features shipped in P1.M1–P1.M3: the parent-death watchdog (FR-K1), SIGHUP-on-terminal-close rescue
(FR-K3), the read-only `stagecoach lock status` diagnostic (FR-K4), and the `no_parent_watchdog` opt-out
(FR-K6). Every "bug found in the wild" (lazygit-TUI-closed-without-killing, IDE-quit, detaching-terminal,
orphaned-but-alive holder) becomes a scenario here. These tests spawn REAL stagecoach subprocesses and
assert real cross-process flock + signal + reparenting behavior that in-process unit tests cannot reach.

**Deliverable**: ONE new file — `internal/e2e/orphan_reclaim_scenarios_test.go` — with build tag
`//go:build e2e && !windows`, `package e2e`, containing:
1. A small set of file-local process-management helpers (`startStagecoach`, `waitForExit`,
   `waitProcessGone`) and lock-path helpers (`lockFilePath`, `plantLockFile`, `ppidOf`) — ADDITIVE, no
   edit to the shared `harness_test.go` (keeps blast radius to one file).
2. Four top-level test functions (or one `TestE2EOrphanReclaim` with `t.Run` subtests) covering scenarios
   (a)–(d). The lock-status diagnostics (c) may be a separate `TestE2ELockStatus` with subtests.

**Success Definition**:
- `go test -tags=e2e ./internal/e2e/ -run 'TestE2EOrphan|TestE2ELockStatus' -v` is GREEN, deterministically,
  with no per-run flakiness (generous timeouts; outcome-based assertions).
- Scenario (a): the reparented stagecoach self-exits within ~10s; the lock FILE is removed; HEAD is
  unchanged (== seed SHA); the index still shows the staged file (no commit landed).
- Scenario (b): SIGHUP → stagecoach exits **3** (rescue); lock FILE removed; HEAD unchanged; index unchanged.
- Scenario (c): `lock status` prints `no run lock for <repo>` with no lock; `alive: true` for a live
  holder; `alive: false` for a dead-pid holder; and (best-effort, skip-guarded) `orphaned: true (holder
  reparented…)` for a genuine ppid==1 holder.
- Scenario (d): with `STAGECOACH_NO_PARENT_WATCHDOG=1`, the reparented stagecoach is NOT killed by the
  watchdog — it runs the stub to completion and COMMITS (commitCount advances; HEAD subject == the stub's
  `feat: …`).
- `go build ./...`, `GOOS=linux/darwin go build ./...`, `go vet ./internal/e2e/`, `gofmt -l` clean.
- `git status --porcelain` shows ONLY `internal/e2e/orphan_reclaim_scenarios_test.go` (scope guard). NO
  production code, NO edit to harness_test.go, NO edit to any `internal/{lock,cmd,signal,watchdog,config}`
  file (those are LANDED and/or owned by the parallel P1.M3.T3.S1).

## User Persona (if applicable)

**Target User**: The Stagecoach maintainer (developer). These are regression tests, not user-facing.
**Use Case**: Catch any future regression in the reclamation machinery (watchdog arming, signal routing,
lock-status read path, opt-out precedence) before it ships — "every bug found in the wild becomes a scenario here."
**User Journey**: A maintainer runs `go test -tags=e2e ./internal/e2e/` (locally or in CI) and gets
deterministic green/red signal on the FR-K1/K3/K4/K6 features.
**Pain Points Addressed**: The unit tests (§20.1 layers 1–3) can't reach real flock across real processes,
real SIGHUP delivery, or real prctl/getppid reparenting — only the §20.5 subprocess harness can. Without
these e2e scenarios, the reclamation features are unit-tested in isolation but never proven against the
real OS process model they rely on.

## Why

- **PRD §20.5** mandates an end-to-end scenario harness that "catches CLI-routing + config-load + real-repo
  bugs that the in-process library tests cannot reach." The §9.27 reclamation features (FR-K1/K3/K4/K6) are
  EXACTLY the kind of OS-process-model behavior that needs subprocess e2e proof: flock is inode-bound
  across real processes, SIGHUP is kernel-delivered, and the parent-death watchdog depends on real
  reparenting. This item is that proof.
- **FR-K1/K3/K4/K6 traceability**: each scenario maps 1:1 to a functional requirement (a→K1, b→K3, c→K4,
  d→K6). The existing `lock_scenarios_test.go` covers §18.5 CONTENTION (FR52) but NOT reclamation; this
  item fills that gap.
- **Regression net for "the lock stays forever"**: the originating hazard (§9.27, §18.5) is an orphaned
  run whose launcher exited without killing it. Scenarios (a)+(d) prove the watchdog reclaims it (and that
  opt-out is respected); (b) proves the SIGHUP terminal-close path reclaims it; (c) gives the user a
  diagnostic to confirm the state. "Every bug found in the wild becomes a scenario here."

## What

A new `//go:build e2e && !windows` test file under `internal/e2e/` with four scenarios driven by the
existing harness + the stub's blocking pattern. The file is Unix-only because ALL four scenarios are
Unix-specific: SIGHUP does not exist on Windows, process reparenting/init does not exist on Windows, and
the watchdog is a Windows no-op (FR-K7) — all already unit-tested per-OS. The reliable lock-status cases
(no-lock/live/dead) are platform-agnostic in behavior but co-located for simplicity.

### Success Criteria
- [ ] **(a) Parent-death watchdog self-exit**: a parent shell (`sh -c`) backgrounds stagecoach (stub
      sleeping 3s), writes its PID to a pidfile, `sleep`s ~1.5s (past `watchdog.Arm`), then exits. After
      the shell exits the stagecoach is reparented; within ~10s the watchdog fires → stagecoach self-exits
      → the lock FILE is removed, HEAD == seed SHA (unchanged), and `status --porcelain` still shows the
      staged file (no commit landed).
- [ ] **(b) SIGHUP rescue**: stagecoach started DIRECTLY (test is the parent), `waitForMarker` (snapshot
      armed), `syscall.Kill(stagecoachPid, syscall.SIGHUP)` → stagecoach exits **3**; lock FILE removed;
      HEAD unchanged; index unchanged.
- [ ] **(c) `lock status` diagnostics**: with NO lock → `no run lock for <repo>` (exit 0); with a LIVE
      real holder (sleeping stub) → `alive: true` + `pid:`/`hostname:`/`Lock:` present; with a planted
      DEAD-pid lock → `alive: false`; (best-effort) with a genuine ppid==1 holder → `orphaned: true
      (holder reparented…)`, `t.Skip`'d when the environment reparents to a subreaper (ppid≠1).
- [ ] **(d) Opt-out (FR-K6)**: scenario (a)'s wrapper re-run with `STAGECOACH_NO_PARENT_WATCHDOG=1` → the
      watchdog does NOT fire; stagecoach runs the stub to completion and COMMITS (commitCount advances by
      one; HEAD subject == stub's `STAGECOACH_STUB_OUT`).
- [ ] All four are deterministic (no flakes) — generous timeouts (marker 10s; process-gone 10s), outcome-
      based assertions, and the snapshot-armed timing guarantee from §2.
- [ ] `go build ./...` + `GOOS=linux` + `GOOS=darwin` clean; `go vet ./internal/e2e/` clean; `gofmt -l`
      empty; `go test -tags=e2e ./internal/e2e/` green.
- [ ] `git status --porcelain` shows ONLY the new file.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim harness API (signatures + the goroutine/marker/drain idiom), the stub's locked
ordering (stdin→marker→sleep→out), the CRITICAL generate.go ordering (SetSnapshot at L242 BEFORE Execute
at L335 → rescue exit 3, not 129/143), the watchdog Arm() race + the exact wrapper recipe (background +
pidfile + sleep 1.5 + exit), the `lock status` exact field labels, the lock-path replication + isolation,
the NoParentWatchdog env knob, the genuine-orphan subreaper caveat (skip-guarded), the sibling scope fence
(P1.M3.T3.S1 is parallel; this item adds NO production code), and the exact validation commands.

### Documentation & References

```yaml
# MUST READ — the architecture spec for FR-K4/§20.5 (the exact 4 scenarios this item implements + the
#              test-implementation considerations, incl. the getppid 1s interval and the SIGHUP/pid approach).
- docfile: plan/014_37208f58ffa2/architecture/cli_test_extension.md
  why: "'E2E test scenarios (§20.5 v2.7)' enumerates the 4 required scenarios (parent-death self-exit,
        SIGHUP rescue, lock-status diagnostics, no_parent_watchdog opt-out) and the implementation notes
        (1s getppid cadence → ~2-3s detection; SIGHUP via syscall.Kill(stagecoachPid, SIGHUP); lock-status
        by planting a lock file). This PRP operationalizes exactly those notes."
  critical: "The parent-death test is 'the hardest to E2E' (the arch doc says so). The wrapper must keep
             the parent alive past watchdog.Arm() then exit (see findings §3 / the wrapper recipe below).
             Assert OUTCOMES (lock removed, HEAD unchanged), NOT the exit code — a reparented process
             can't be Wait'd by the test."

# MUST READ — codebase-specific findings for THIS item (verbatim harness API, the stub ordering, the
#              generate.go snapshot-arm-before-Execute ordering, the watchdog Arm race + wrapper recipe,
#              the lock status field labels, the lock-path replication, the subreaper caveat, validation).
- docfile: plan/014_37208f58ffa2/P1M4T1S1/research/findings.md
  why: "§0 harness API; §1 stub ordering (marker = snapshot armed); §2 THE exit-code determination
        (rescue exit 3 at marker time — NOT 129/143); §3 the Arm() race + the wrapper recipe; §4 the
        rescue path always releases the lock (OnRescueExit=lock.ReleaseCurrent); §5 lock status field
        labels + os.Getwd()⇒cmd.Dir=repo; §6 lock-path replication + XDG isolation; §7 the 3 opt-out
        knobs; §8 the goroutine/marker/drain idiom to mirror; §9 the subreaper caveat (skip-guard)."

# MUST READ — the harness (the primitives to REUSE; the gap that needs the new file-local helpers).
- file: internal/e2e/harness_test.go
  why: "buildStagecoach/buildStub/newRepo/runStagecoach/waitForMarker/writeStubConfig/stubEnv/headSHA/
        commitCount/statusPorcelain/seedCommit/writeFile/stageFile/runGit — call these as-is. runStagecoach
        runs to COMPLETION and returns no PID → scenarios (a)/(b) need startStagecoach (Start, no wait) +
        waitForExit / waitProcessGone, added as file-local helpers in the NEW file (do NOT edit this file)."
  pattern: "runStagecoach builds exec.CommandContext(60s), sets cmd.Dir=repo, cmd.Env=env, captures
            stdout+stderr, maps (*exec.ExitError).ExitCode() → e2eResult. startStagecoach is the SAME setup
            minus the Run (return the *exec.Cmd). stubEnv = os.Environ()+knobs → t.Setenv BEFORE stubEnv flows."

# MUST READ — the stub agent (the blocking pattern that makes the races deterministic).
- file: cmd/stubagent/main.go
  why: "The locked ordering: drain stdin → write STAGECOACH_STUB_MARKER → sleep STAGECOACH_STUB_SLEEP_MS
        → write STAGECOACH_STUB_OUT → exit STAGECOACH_STUB_EXIT. waitForMarker returns AFTER stdin drain +
        marker write, i.e. AFTER generate.go:242 SetSnapshot armed the rescue → exit 3 is deterministic."
  gotcha: "The stub drains stdin FIRST (deadlock guard). The marker is written AFTER drain, BEFORE sleep —
           so at waitForMarker return the snapshot is armed (see generate.go ordering)."

# MUST READ — the generate ordering that determines exit codes (rescue 3 vs 129/143).
- file: internal/generate/generate.go
  why: "CommitStaged: L237 WriteTree → L242 signal.SetSnapshot (ARMS rescue) → L243 lock.SetSnapshot →
        L335 provider.Execute (runs the stub). So once the stub's marker exists, snapshot != '' → SIGHUP
        and parent-death BOTH route to the post-snapshot rescue → exit 3 + OnRescueExit (lock release)."
  critical: "NEVER assert 129/143 for SIGHUP or parent-death sent AFTER waitForMarker — those are the
             PRE-snapshot codes and are unreachable at marker time. Assert exit 3 (scenario b)."

# MUST READ — the watchdog (the Arm() race is the crux of scenario (a)).
- file: internal/watchdog/watchdog.go
  why: "Arm captures originalPpid=osGetppid() AT ARM TIME, then polls for osGetppid()!=originalPpid (a
        CHANGE — subreaper-safe, not ==1). If the parent dies BEFORE Arm runs, originalPpid is captured as
        init → no change ever → watchdog never fires (the race). So the test's parent MUST survive past
        Arm() (~tens of ms). default_action.go:108 gates Arm on !cfg.NoParentWatchdog (scenario d)."
- file: internal/watchdog/arm_unix.go
  why: "On ppid change → signal.Trigger(SIGTERM) → the single rescue path (exit 3 + lock release). Linux
        ALSO best-effort prctl(PR_SET_PDEATHSIG) (pdeathsig_linux.go) — kernel SIGTERM on parent death; the
        poll ALWAYS runs (covers prctl loss). On darwin the poll is the only detector (~1s latency)."

# MUST READ — the rescue path ALWAYS releases the lock (why 'lock removed' is a valid assertion).
- file: internal/signal/signal.go
  why: "handle() calls h.opts.OnRescueExit() on BOTH branches (post-snapshot exit 3 AND pre-snapshot
        129/130/143). main.go:61 wires OnRescueExit = lock.ReleaseCurrent → Release → os.Remove(path). So
        rescue ALWAYS removes the lock file → 'lock removed' holds for scenarios (a) and (b). caughtSignals()
        (signal_unix.go) includes SIGHUP → our handler catches syscall.Kill(pid, SIGHUP) (not Go's default)."

# MUST READ — lock.Status + the lock status subcommand (exact field labels for scenario (c) assertions).
- file: internal/lock/lock.go
  why: "Status(repoPath) (path, contents, alive, orphan, err): path==''⇒no lock; alive=processAlive(pid,
        hostname); orphan=appearsOrphaned(pid) if alive. lockPath = lockDir+sha256(EvalSymlinks(repo))+'.lock'.
        lockDir = XDG_RUNTIME_DIR(abs)→XDG_CACHE_HOME(abs)→~/.cache/stagecoach/locks (NO CWD fallback)."
- file: internal/cmd/lock.go
  why: "runLockStatus output labels (assert these EXACT strings): 'Lock: <path>', '  pid:       <pid>',
        '  hostname:  <host>', '  repo:      <repo>', '  timestamp: <ts>', '  snapshot:  <sha>' (only if
        set), '  alive:     <bool>', then the 3-way '  orphaned:  true (holder reparented — launcher has
        exited)' / '  orphaned:  false' / '  orphaned:  unknown (holder is dead)'. No lock → 'no run lock
        for <repoDir>' (exit 0). Uses os.Getwd() → run with cmd.Dir=repo (runStagecoach sets it)."

# CONTEXT — the existing contention e2e tests (mirror the goroutine/marker/drain idiom for the live holder).
- file: internal/e2e/lock_scenarios_test.go
  why: "TestE2ELockContention subtests A–F: spawn holder in a goroutine on a channel, waitForMarker, poke
        the contender, <-resCh to drain. The scenario-(c) LIVE holder reuses this EXACT structure (hold a
        real lock via a sleeping stub, run 'lock status', drain the holder). NO overlap (contention ≠ reclaim)."

# CONTEXT — the NoParentWatchdog config (scenario d's env knob).
- file: internal/config/load.go
  why: "L329 STAGECOACH_NO_PARENT_WATCHDOG (presence-semantic, strconv.ParseBool, DIRECT set — can be
        false = escape hatch). Set it via stubEnv in scenario (d). The arming gate is default_action.go:108."

# CONTEXT — the orphan heuristic's subreaper limitation (scenario c's genuine-orphan subtest).
- file: internal/lock/orphan_unix.go
  why: "appearsOrphaned: ppid==1⇒true, else false (CONSERVATIVE — false on any ambiguity; MISSES
        subreaper-reparented orphans whose ppid is the subreaper, not 1). So a genuine orphaned:true is
        reproducible ONLY when the env reparents to init (ppid==1) → the e2e orphan subtest must verify
        ppid==1 and t.Skip otherwise (no flake). ppidOf(pid): Linux /proc/<pid>/status; else 'ps -o ppid= -p'."

# CONTEXT — the existing lock unit-test helpers (the planted-lock format to replicate).
- file: internal/lock/lock_unix_test.go
  why: "writeLockFile(t, path, pid, hostname) writes 'pid=…\\nhostname=…\\nrepo=fake\\ntimestamp=fake\\nsnapshot=\\n'
        — the EXACT format parseContents reads. The e2e plantLockFile mirrors it (with a real repo path +
        timestamp). Also shows the XDG isolation idiom (t.Setenv XDG_RUNTIME_DIR=t.TempDir(); XDG_CACHE_HOME='')."
```

### Current Codebase tree (relevant slice)

```bash
internal/e2e/                           # the §20.5 harness (all //go:build e2e)
  harness_test.go                       # READ-ONLY — reuse primitives; NO edit (keep blast radius to 1 file)
  lock_scenarios_test.go                # READ-ONLY — contention (FR52); mirror the goroutine/marker/drain idiom
  scenarios_test.go                     # READ-ONLY — decompose/single-commit
  hook_scenarios_test.go                # READ-ONLY — hook mode
  config_precedence_test.go             # READ-ONLY
cmd/stubagent/main.go                   # READ-ONLY — the stub (STAGECOACH_STUB_MARKER + _SLEEP_MS)
internal/lock/lock.go                   # READ-ONLY — Status, lockPath, LockContents (consumed)
internal/cmd/lock.go                    # READ-ONLY — the `lock status` subcommand (consumed)
internal/watchdog/{watchdog,arm_unix,pdeathsig_*}.go  # READ-ONLY — Arm + the race (consumed)
internal/signal/signal.go               # READ-ONLY — Trigger/handle/OnRescueExit (consumed)
internal/generate/generate.go           # READ-ONLY — SetSnapshot-before-Execute ordering (consumed)
internal/config/load.go                 # READ-ONLY — STAGECOACH_NO_PARENT_WATCHDOG (consumed)
cmd/stagecoach/main.go                  # READ-ONLY — OnRescueExit=lock.ReleaseCurrent (consumed)
Makefile                                # READ-ONLY — no e2e target; invoke via `go test -tags=e2e`
```

### Desired Codebase tree with files to be added

```bash
internal/e2e/orphan_reclaim_scenarios_test.go   # NEW — //go:build e2e && !windows; package e2e.
  # File-local helpers (additive; NOT promoted to harness_test.go):
  #   startStagecoach(t, bin, repo, cfg, env, args...) *exec.Cmd   # Start, NO wait (returns cmd; PID ready)
  #   waitForExit(t, cmd, timeout) e2eResult                       # Wait w/ timeout; maps exit code
  #   waitProcessGone(t, pid, timeout)                             # poll syscall.Kill(pid,0)==ESRCH
  #   lockFilePath(repo) string                                    # replicate lock.lockPath (XDG-isolated)
  #   plantLockFile(t, path, pid, hostname, repo)                  # write a *.lock with known contents
  #   ppidOf(pid) (int, error)                                     # /proc or `ps -o ppid= -p` (subreaper guard)
  # Scenarios: TestE2EOrphanReclaim (t.Run: ParentDeath/SIGHUP/OptOut) + TestE2ELockStatus (t.Run:
  #   NoLock/Live/Dead/Orphan-besteffort).  OR four top-level funcs — implementer's choice.
# NOTHING ELSE. No production code. No edit to harness_test.go, lock_scenarios_test.go, any internal/*
# file, root.go, main.go, go.mod, or any PRD/task file. No docs (P1.M4.T2.S1 owns the README sync).
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (exit codes are rescue 3, NOT 129/143): generate.go arms the snapshot (signal.SetSnapshot,
// L242) BEFORE provider.Execute (L335) runs the stub. The stub writes its marker AFTER stdin drain
// (post-Execute-start). So once waitForMarker returns, snapshot != "" → SIGHUP and parent-death BOTH
// route through handle()'s POST-snapshot branch → exit 3 + OnRescueExit (lock release). NEVER assert
// 129/143 for a signal sent after waitForMarker. Scenario (b) asserts exit == 3.

// CRITICAL (the watchdog Arm() race — scenario a's crux): watchdog.Arm captures originalPpid at ARM
// time and polls for a CHANGE. If the parent dies BEFORE Arm runs (~tens of ms after start), originalPpid
// is captured as init/the subreaper → NO change ever → watchdog never fires. So the test's parent shell
// MUST survive past Arm(): background stagecoach, write its PID, `sleep 1.5`, THEN exit. (This is the
// documented prctl/getppid race — the man page warns to check getppid()==1 after PR_SET_PDEATHSIG.)

// CRITICAL (a reparented process can't be Wait'd): after the parent shell exits, stagecoach is reparented
// to init/a subreaper — the test process is NOT its parent, so cmd.Wait() can't reap it and the exit code
// is UNREADABLE. Detect exit by polling syscall.Kill(pid, 0) == ESRCH (waitProcessGone). Scenario (a)
// asserts OUTCOMES (process gone + lock removed + HEAD/index unchanged), NOT the exit code. Scenario (b)
// (test IS the parent) CAN read exit 3.

// CRITICAL (rescue ALWAYS removes the lock): handle() calls OnRescueExit (=lock.ReleaseCurrent → Release
// → os.Remove) on BOTH branches. So after a rescue exit (scenarios a, b), the lock FILE is GONE →
// os.Stat(lockPath) → os.IsNotExist. (Scenario d's clean commit ALSO removes the lock via the deferred
// Release — so the lock-removed assertion does NOT distinguish a from d; HEAD advancement does.)

// CRITICAL (lock isolation flows through stubEnv→os.Environ()): stubEnv() = os.Environ() + knobs, and
// os.Environ() reads the test process's CURRENT env. So t.Setenv("XDG_RUNTIME_DIR", tmpDir) MUST be
// called BEFORE stubEnv() for the value to reach the stagecoach subprocess (and let lockFilePath compute
// the same path the subprocess uses). t.Setenv AFTER stubEnv silently misses. Mirror lock_unix_test.go.

// GOTCHA (the wrapper must NOT SIGHUP the backgrounded job): non-interactive `sh -c 'cmd &'` does NOT
// send SIGHUP to the backgrounded job on exit (SIGHUP-on-exit is an interactive-shell/huponexit feature,
// off by default). So the backgrounded stagecoach is cleanly orphaned (reparented), not SIGHUP'd. Do NOT
// add `disown`/`nohup` (they're bash-isms; `sh -c` is POSIX and already correct).

// GOTCHA (the dead-pid for lock-status): capture a GUARANTEED-dead pid by Start()+Wait() a `true`/`sleep 0`
// and use its pid — reaped, dead. Do NOT use a magic number (999999) which could (vanishingly rarely) be
// recycled. Micro-race: a reaped pid can be recycled before the assertion — negligible for a test; if
// paranoid, assert alive==false and tolerate a retry. plantLockFile must set hostname=THIS host (else
// processAlive short-circuits to true for a foreign host).

// GOTCHA (the genuine orphan is subreaper-dependent): appearsOrphaned returns true ONLY for ppid==1.
// Under a subreaper (systemd, systemd-run, containers, some shells) a reparented process's ppid is the
// subreaper, not 1 → false. So the e2e Orphan subtest must VERIFY the produced holder's ppid==1 (ppidOf)
// and t.Skip otherwise — never flake. The RELIABLE core of scenario (c) is NoLock/Live/Dead.

// GOTCHA (Unix-only file): ALL four scenarios are Unix-specific (SIGHUP, init-reparenting, watchdog
// semantics). Use build tag //go:build e2e && !windows. Windows behavior (watchdog no-op, no SIGHUP) is
// already covered by per-OS unit tests (FR-K7). The file-local helpers use syscall.Kill/syscall.SIGHUP
// (Unix-only symbols) → the !windows gate is mandatory for compilation.

// GOTCHA (do NOT edit harness_test.go): add startStagecoach/waitForExit/waitProcessGone/lockFilePath/
// plantLockFile/ppidOf as FILE-LOCAL helpers in the NEW file. Editing the shared harness churns the 4
// existing scenario files' invariants. Keep the scope to ONE new file (scope guard enforces this).
```

## Implementation Blueprint

### Data models and structure

None NEW beyond reusing the harness's `e2eResult{Stdout, Stderr, ExitCode string/int}` (extend nothing —
`waitForExit` returns the same `e2eResult` type). The lock-path/plant helpers operate on plain strings +
`os.WriteFile`. No production types, no config, no imports beyond stdlib (`os`, `os/exec`, `syscall`,
`path/filepath`, `crypto/sha256`, `encoding/hex`, `fmt`, `time`, `strings`, `strconv`, `testing`) +
`internal/stubtest` (already used by the harness for `buildStub`). NO `internal/lock`/`internal/watchdog`
imports — these are SUBPROCESS tests; the stagecoach binary IS the SUT. (Cross-package import of internal/*
from a `_test.go` in package e2e is unnecessary and would couple the e2e suite to internal layout.)

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE internal/e2e/orphan_reclaim_scenarios_test.go — header + build tag + imports + helpers
  - BUILD TAG: `//go:build e2e && !windows` (the newline after the tag, then `//go:build`-style blank line,
    then `package e2e`). Unix-only — ALL scenarios are Unix-specific (syscall.SIGHUP/Kill, reparenting).
  - PACKAGE: `e2e` (white-box — reuses the unexported harness primitives buildStagecoach/runStagecoach/etc.).
  - IMPORTS: bytes, context, crypto/sha256, encoding/hex, errors, fmt, os, os/exec, path/filepath, runtime,
    strconv, strings, syscall, testing, time — PLUS `github.com/dustin/stagecoach/internal/stubtest` ONLY
    if you call buildStub directly (the harness exposes buildStub(t) already → prefer that; no new import).
    DO NOT import internal/lock, internal/watchdog, internal/signal, internal/generate (subprocess tests).
  - FILE-LOCAL HELPERS (additive; do NOT promote to harness_test.go):
      // startStagecoach is runStagecoach MINUS the Run: it builds the exec.CommandContext (60s), sets
      // cmd.Dir=repo, cmd.Env=env, wires stdout/stderr capture, calls Start(), and returns the *exec.Cmd
      // (cmd.Process.Pid ready). Caller does waitForExit(cmd, timeout) or waitProcessGone(pid, timeout).
      func startStagecoach(t *testing.T, bin, repo, cfg string, env []string, args ...string) *exec.Cmd
      // waitForExit waits for cmd (with timeout) and returns e2eResult{Stdout,Stderr,ExitCode}. Maps
      // (*exec.ExitError).ExitCode(); a context-deadline/other error → t.Fatalf (a hang is a test failure).
      func waitForExit(t *testing.T, cmd *exec.Cmd, timeout time.Duration) e2eResult
      // waitProcessGone polls syscall.Kill(pid, 0) for ESRCH (process exited) up to timeout. For a
      // reparented process the test can't Wait() it — this is the exit detector. Fatalf on timeout.
      func waitProcessGone(t *testing.T, pid int, timeout time.Duration)
      // lockFilePath replicates lock.lockPath under an isolated XDG_RUNTIME_DIR: EvalSymlinks(repo)
      // (→Abs on error), sha256, hex, join(XDG_RUNTIME_DIR, "stagecoach", "locks", hash+".lock"). Caller
      // MUST t.Setenv XDG_RUNTIME_DIR + XDG_CACHE_HOME='' BEFORE stubEnv so the subprocess agrees.
      func lockFilePath(repo string) string
      // plantLockFile writes a *.lock with known pid/hostname/repo (format: pid=…\nhostname=…\nrepo=…
      // \ntimestamp=…\nsnapshot=\n — mirrors lock_unix_test.go writeLockFile + parseContents). MkdirAll
      // the dir (0o700); WriteFile 0o600. Used by scenario (c) Dead + Orphan.
      func plantLockFile(t *testing.T, path, pid, hostname, repo string)
      // ppidOf returns the parent pid of pid (Linux /proc/<pid>/status PPid:; else `ps -o ppid= -p <pid>`).
      // Used by scenario (c) Orphan to verify ppid==1 (skip if subreaper-reparented, ppid≠1). Mirror
      // internal/lock/orphan_unix.go ppidOf exactly (the same platform dispatch).
      func ppidOf(pid int) (int, error)
  - FOLLOW pattern: harness_test.go's runStagecoach (startStagecoach is its Start-only twin) and
    lock_unix_test.go's writeLockFile + ppidOf (plantLockFile/ppidOf are e2e-local copies).
  - GOTCHA: the 60s context timeout in startStagecoach must be CANCELLED in waitForExit's happy path
    (defer cancel()) so the test process doesn't leak a live ctx. Mirror runStagecoach's defer cancel().

Task 2: SCENARIO (b) SIGHUP rescue (FR-K3) — implement FIRST (cleanest, fully deterministic, validates
         the rescue-exit-3 + lock-removed invariants the other scenarios lean on)
  - STRUCTURE: newRepo + seedCommit("readme.md","init") + writeFile("a.txt") + stageFile("a.txt").
    Isolate locks: t.Setenv("XDG_RUNTIME_DIR", t.TempDir()); t.Setenv("XDG_CACHE_HOME", ""). Compute
    lockPath := lockFilePath(repo). marker := t.TempDir()+"/ready.marker".
  - ENV: stubEnv(map{"STAGECOACH_STUB_OUT":"feat: a","STAGECOACH_STUB_MARKER":marker,"STAGECOACH_STUB_SLEEP_MS":"3000"}).
  - START: cmd := startStagecoach(t, bin, repo, cfg, env, "--provider","stub").
    seedHead := headSHA(t, repo)  // capture BEFORE generation
  - SYNC: waitForMarker(t, marker, 10*time.Second)  // snapshot armed (generate.go:242), stub blocked.
  - SIGNAL: if err := syscall.Kill(cmd.Process.Pid, syscall.SIGHUP); err != nil { t.Fatalf(...) }.
  - COLLECT: res := waitForExit(t, cmd, 10*time.Second).
  - ASSERT:
      res.ExitCode == 3                                    // rescue (snapshot armed) — NOT 129
      os.Stat(lockPath) → os.IsNotExist (lock FILE removed by OnRescueExit=lock.ReleaseCurrent)
      headSHA(t, repo) == seedHead                         // HEAD unchanged (no commit landed)
      strings.Contains(statusPorcelain(t, repo), "a.txt")  // index unchanged (a.txt still staged)
  - NAMING: TestE2EOrphanReclaim_SIGHUP (or t.Run("B_SIGHUPRescue") under TestE2EOrphanReclaim).
  - GOTCHA: assert exit == 3 (NOT 129). The marker guarantees snapshot armed (generate.go L242<L335).

Task 3: SCENARIO (a) Parent-death watchdog self-exit (FR-K1) — the hard one
  - STRUCTURE: same repo/seed/stage + XDG isolation + lockPath + marker as (b).
  - PIDFILE: pidfile := t.TempDir()+"/child.pid" (the wrapper writes stagecoach's PID here).
  - THE WRAPPER (the crux — keeps the parent alive past watchdog.Arm then exits to reparent stagecoach):
      script := fmt.Sprintf(
        "%s --config %s --provider stub &\n"+        // background stagecoach (child of sh)
          "echo $! > %s\n"+                            // write its PID to the pidfile
          "sleep 1.5\n",                               // STAY ALIVE ~1.5s (past watchdog.Arm ~tens of ms)
        // (no trailing 'wait' — sh exits after sleep → stagecoach orphaned → reparented to init/subreaper)
        shQuote(bin), shQuote(cfg), shQuote(pidfile))
      wrapper := exec.Command("sh","-c",script)
      wrapper.Env = env                                 // STAGECOACH_STUB_* inherited by the bg'd stagecoach
      wrapper.Dir = repo
      if err := wrapper.Start(); err != nil { t.Fatalf(...) }
      if err := wrapper.Wait(); err != nil { /* sh exits 0 normally; tolerate non-zero if you didn't add 'exit 0' */ }
    NOTE: shQuote wraps paths in single quotes (t.TempDir paths are usually space-free, but be safe). The
    'sleep 1.5' is the race-defeating delay: stagecoach's watchdog.Arm (default_action.go:108) captures
    originalPpid = sh's pid; when sh exits at t≈1.5s, stagecoach's getppid changes → poll (≤1s on darwin)
    or prctl SIGTERM (Linux) fires → signal.Trigger(SIGTERM) → rescue exit 3 + lock release.
  - READ PID: data, _ := os.ReadFile(pidfile); pid, err := strconv.Atoi(strings.TrimSpace(string(data))).
  - SYNC (optional but recommended): waitForMarker(t, marker, 10s) confirms the stub reached generation
    (snapshot armed) — if the marker is missing after wrapper.Wait(), the wrapper raced stagecoach's start;
    bump the sleep. (Normally the marker exists ~0.1s after start, long before sh's 1.5s exit.)
  - DETECT EXIT: waitProcessGone(t, pid, 10*time.Second)  // can't Wait() a reparented process → poll ESRCH
  - ASSERT:
      os.Stat(lockPath) → os.IsNotExist (lock removed by rescue OnRescueExit)
      headSHA(t, repo) == seedHead (capture seedHead BEFORE the wrapper)  // no commit landed
      strings.Contains(statusPorcelain(t, repo), "a.txt")                 // index unchanged
      // NOTE: do NOT assert exit code — a reparented process's code is unreadable from the test.
  - NAMING: TestE2EOrphanReclaim_ParentDeath (or t.Run("A_ParentDeathWatchdog")).
  - GOTCHA: the 'sleep 1.5' is MANDATORY (without it sh exits before Arm → originalPpid=init → no detection).
    Use a generous waitProcessGone timeout (10s) for the 1s poll + exit + reap. If the stub's 3s sleep is
    shorter than the detection window, bump STAGECOACH_STUB_SLEEP_MS to e.g. 5000 so the watchdog fires
    DURING the sleep (the stub is then SIGTERM'd via KillProcessGroup and stagecoach exits 3 before commit).

Task 4: SCENARIO (c) `stagecoach lock status` diagnostics (FR-K4)
  - STRUCTURE: a top-level TestE2ELockStatus with t.Run subtests (or four funcs). Isolate XDG per subtest.
  - SUBTEST NoLock: repo := newRepo(t); t.Setenv XDG_RUNTIME_DIR/XDG_CACHE_HOME (BEFORE any stubEnv).
    res := runStagecoach(t, bin, repo, cfg, stubEnv(nil), "lock","status").
    assert res.ExitCode == 0; strings.Contains(res.Stdout, "no run lock for"); strings.Contains(res.Stdout, repo).
    NOTE: lock status writes to STDOUT (cmd.OutOrStdout) — assert res.Stdout, not res.Stderr.
  - SUBTEST Live: spawn a holder (sleeping stub) in a goroutine (mirror lock_scenarios_test.go A), waitForMarker,
    then res := runStagecoach(t, bin, repo, cfg, stubEnv(nil), "lock","status"). Assert ExitCode==0;
    strings.Contains(res.Stdout, "Lock:"); strings.Contains(res.Stdout, "alive:     true");
    strings.Contains(res.Stdout, "pid:"); strings.Contains(res.Stdout, "hostname:");
    (assert "orphaned:" field is PRESENT — do not pin its value; CI-under-init could differ).
    Drain the holder (<-resCh) so the lock releases.
  - SUBTEST Dead: capture a guaranteed-dead pid: dead := exec.Command("true"); dead.Start();
    deadPid := dead.Process.Pid; dead.Wait().  // reaped → dead
    lp := lockFilePath(repo); thisHost,_ := os.Hostname(); plantLockFile(t, lp, strconv.Itoa(deadPid), thisHost, repo).
    res := runStagecoach(t, bin, repo, cfg, stubEnv(nil), "lock","status"). Assert ExitCode==0;
    strings.Contains(res.Stdout, "alive:     false"); strings.Contains(res.Stdout, "orphaned:  unknown (holder is dead)").
  - SUBTEST Orphan (BEST-EFFORT, skip-guarded): produce a genuine ppid==1 holder:
      orph := exec.Command("sh","-c","sleep 30 &\necho $!"); out,_ := orph.Output();
      sleepPid := atoi(strings.TrimSpace(out))         // the sleep is reparented to init when sh exits
      ppid, err := ppidOf(sleepPid); if err != nil || ppid != 1 { kill(sleepPid); t.Skipf("subreaper: ppid=%d≠1", ppid) }
      lp := lockFilePath(repo); thisHost,_ := os.Hostname(); plantLockFile(t, lp, atoi(sleepPid), thisHost, repo).
      res := runStagecoach(..., "lock","status"). Assert strings.Contains(res.Stdout, "alive:     true");
      strings.Contains(res.Stdout, "orphaned:  true (holder reparented"). DEFER syscall.Kill(sleepPid, SIGKILL) cleanup.
  - NAMING: TestE2ELockStatus_NoLock / _Live / _Dead / _Orphan (or t.Run under TestE2ELockStatus).
  - GOTCHA: lock status STDOUT (not stderr). XDG t.Setenv BEFORE stubEnv. The Dead pid uses hostname=THIS host
    (else processAlive short-circuits true for foreign host). The Orphan subtest t.Skip's on subreaper.

Task 5: SCENARIO (d) Opt-out (FR-K6) — proves the watchdog does NOT fire when disabled
  - STRUCTURE: identical wrapper to scenario (a), but env ADDS "STAGECOACH_NO_PARENT_WATCHDOG":"1".
  - ASSERT (the watchdog is OFF → stagecoach runs the stub to completion and COMMITS):
      waitProcessGone(t, pid, 15*time.Second)          // survives PAST the watchdog window; runs the 3s stub
      commitCount(t, repo) == 2                        // seed + the stub's commit LANDED
      runGit(t, repo, "log","-1","--format=%s") == "feat: a"   // HEAD advanced to the stub's message
    (The lock FILE is also removed — by the clean deferred Release — but HEAD advancement is the discriminator
     that proves opt-out: with the watchdog ON (scenario a) HEAD is unchanged; with it OFF HEAD advances.)
  - NAMING: TestE2EOrphanReclaim_OptOut (or t.Run("D_NoParentWatchdogOptOut")).
  - GOTCHA: give waitProcessGone a GENEROUS timeout (15s) — the stub sleeps 3s then commits then exits; the
    process must NOT be gone at the ~2-3s watchdog window (if it were, opt-out failed). The OUTCOME assertion
    (commitCount==2) is the proof; do not rely on timing.

Task 6: VERIFY — build (native + cross-compile), vet, format, run, scope guard
  - go build ./... ; GOOS=linux go build ./... ; GOOS=darwin go build ./...   # windows excluded by tag; still build windows to prove no breakage: GOOS=windows go build ./...
  - go vet ./internal/e2e/ ; gofmt -l internal/e2e/orphan_reclaim_scenarios_test.go   # empty
  - go test -tags=e2e ./internal/e2e/ -run 'TestE2EOrphan|TestE2ELockStatus' -v        # the new scenarios
  - go test -tags=e2e -race ./internal/e2e/                                           # full e2e suite incl. contention
  - make test ; make lint ; make build                                                # NO regression in the rest
  - git status --porcelain   # ONLY the new file. grep guards in Validation Loop Level 4.
```

### Implementation Patterns & Key Details

```go
// PATTERN (startStagecoach = runStagecoach minus the Run — mirror harness_test.go:runStagecoach):
func startStagecoach(t *testing.T, bin, repo, cfg string, env []string, args ...string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	allArgs := append([]string{"--config", cfg, "--no-color"}, args...)
	cmd := exec.CommandContext(ctx, bin, allArgs...)
	cmd.Dir = repo
	cmd.Env = env
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start stagecoach: %v", err)
	}
	// Stash cancel + buffers on the cmd via a wrapper OR return a struct; simplest: the caller passes
	// the SAME buffers by closing over them in waitForExit. (If returning just *exec.Cmd, capture out/errb
	// in a small e2eCmd struct { *exec.Cmd; out, errb bytes.Buffer; cancel } and have waitForExit take it.)
	t.Cleanup(func() { cancel(); _ = cmd.Wait() }) // never leak a process if the test aborts
	return cmd
}
// (Prefer a small `type e2eCmd struct{ *exec.Cmd; stdout, stderr bytes.Buffer; cancel }` so waitForExit
//  can read the captured buffers + map the exit code — mirrors e2eResult. Keep it file-local.)

// PATTERN (the scenario-a wrapper — the race-defeating recipe):
script := fmt.Sprintf("%s --config %s --provider stub &\necho $! > %s\nsleep 1.5\n",
	shQuote(bin), shQuote(cfg), shQuote(pidfile))
wrapper := exec.Command("sh", "-c", script)
wrapper.Env = env // STAGECOACH_STUB_* inherited by the backgrounded stagecoach
wrapper.Dir = repo
_ = wrapper.Start()
_ = wrapper.Wait()                  // sh exits ~1.5s → stagecoach reparented → watchdog detects
pid := readPid(pidfile)
waitProcessGone(t, pid, 10*time.Second)   // poll syscall.Kill(pid,0)==ESRCH (can't Wait a reparented proc)

// PATTERN (scenario b — test IS the parent, exit code readable):
cmd := startStagecoach(t, bin, repo, cfg, env, "--provider", "stub")
waitForMarker(t, marker, 10*time.Second)                  // snapshot armed → exit 3 deterministic
syscall.Kill(cmd.Process.Pid, syscall.SIGHUP)
res := waitForExit(t, cmd, 10*time.Second)                // exit 3 (NOT 129)
// assert res.ExitCode == 3 + lock gone + HEAD unchanged

// PATTERN (lockFilePath — replicate lock.lockPath under isolated XDG):
func lockFilePath(repo string) string {
	canonical, err := filepath.EvalSymlinks(repo)
	if err != nil {
		canonical, _ = filepath.Abs(repo)
	}
	sum := sha256.Sum256([]byte(canonical))
	return filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "stagecoach", "locks",
		hex.EncodeToString(sum[:])+".lock")
}

// PATTERN (plantLockFile — mirror lock_unix_test.go writeLockFile + parseContents keys):
func plantLockFile(t *testing.T, path, pid, hostname, repo string) {
	t.Helper()
	content := fmt.Sprintf("pid=%s\nhostname=%s\nrepo=%s\ntimestamp=2026-07-10T00:00:00Z\nsnapshot=\n",
		pid, hostname, repo)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { t.Fatalf(...) }
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil { t.Fatalf(...) }
}
```

### Integration Points

```yaml
E2E HARNESS (internal/e2e/):
  - ADD one new file orphan_reclaim_scenarios_test.go (//go:build e2e && !windows, package e2e).
  - CONSUMES the existing primitives verbatim (buildStagecoach/buildStub/newRepo/runStagecoach/waitForMarker/
    writeStubConfig/stubEnv/headSHA/commitCount/statusPorcelain/seedCommit/writeFile/stageFile/runGit).
  - ADDS file-local helpers (startStagecoach/waitForExit/waitProcessGone/lockFilePath/plantLockFile/ppidOf) —
    NOT promoted to harness_test.go (scope guard).
NO production code. NO edit to internal/{lock,cmd,signal,watchdog,config,generate}, main.go, root.go, go.mod,
  any PRD/task file, or harness_test.go / the other scenario files.
NO docs (P1.M4.T2.S1 owns the README + docs/ sync for the reclamation features).
SCOPE FENCES:
  - Touches ONLY: internal/e2e/orphan_reclaim_scenarios_test.go (NEW).
  - Does NOT touch: harness_test.go, lock_scenarios_test.go, scenarios_test.go, hook_scenarios_test.go,
    config_precedence_test.go (existing e2e files), any internal/* production file, the parallel
    P1.M3.T3.S1 files (internal/lock/lock.go +IsOrphaned, internal/cmd/default_action.go, etc.).
  - The orphan-hint E2E is NOT here (P1.M3.T3.S1 drives the hint via its unit-test seam; a genuine
    ppid==1 holder is flaky — this item's Orphan subtest is skip-guarded best-effort, not hint-assertion).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Native + cross-compile (the !windows tag excludes the new file on windows; verify the rest still builds).
go build ./...
GOOS=linux   go build ./...
GOOS=darwin  go build ./...
GOOS=windows go build ./...   # the new file is excluded; proves no collateral breakage
# Expected: all clean.

# Vet the e2e package.
go vet ./internal/e2e/
# Expected: clean.

# Format the new file.
gofmt -l internal/e2e/orphan_reclaim_scenarios_test.go
# Expected: empty. If listed: gofmt -w internal/e2e/orphan_reclaim_scenarios_test.go.

# Lint (the file-local helpers must all be USED; no unused-symbol findings).
make lint
# Expected: zero errors.

# Scope guard: ONLY the new file changed.
git status --porcelain
# Expected: internal/e2e/orphan_reclaim_scenarios_test.go ONLY.
git diff --name-only | grep -vE '^internal/e2e/orphan_reclaim_scenarios_test\.go$' && echo "FAIL: out-of-scope file edited" || echo "OK: scope clean"
```

### Level 2: Unit/Component — the new e2e scenarios (run the SUT as a subprocess)

```bash
# The four scenarios in isolation (fast feedback on each).
go test -tags=e2e ./internal/e2e/ -run 'TestE2EOrphanReclaim' -v
go test -tags=e2e ./internal/e2e/ -run 'TestE2ELockStatus' -v
# Expected: ALL GREEN —
#   TestE2EOrphanReclaim/A_ParentDeathWatchdog: process gone + lock removed + HEAD/index unchanged.
#   TestE2EOrphanReclaim/B_SIGHUPRescue: exit 3 + lock removed + HEAD/index unchanged.
#   TestE2ELockStatus/NoLock: "no run lock for <repo>" (exit 0).
#   TestE2ELockStatus/Live: alive:true + pid/hostname/Lock present.
#   TestE2ELockStatus/Dead: alive:false + "orphaned:  unknown (holder is dead)".
#   TestE2ELockStatus/Orphan: alive:true + orphaned:true (or t.Skip under a subreaper — NOT a failure).
#   TestE2EOrphanReclaim/D_NoParentWatchdogOptOut: commitCount==2 + HEAD subject "feat: a" (commit landed).

# Race detector (subprocess tests; -race is fine and catches test-side data races in the helpers).
go test -tags=e2e -race ./internal/e2e/ -run 'TestE2EOrphan|TestE2ELockStatus' -v
# Expected: green.

# Run the scenario a few times to flush timing flakes (the parent-death/opt-out are timing-sensitive).
go test -tags=e2e ./internal/e2e/ -run 'TestE2EOrphanReclaim' -count=3
# Expected: 3/3 green. If flaky, RAISE the stub sleep (STAGECOACH_STUB_SLEEP_MS) and/or the waitProcessGone
#           timeout — do NOT tighten assertions.
```

### Level 3: Integration — full e2e suite + the rest of the repo

```bash
# Full e2e suite (contention + decompose + hook + the new reclaim scenarios).
go test -tags=e2e ./internal/e2e/
# Expected: green (the new scenarios don't disturb the existing ones — different repos/locks).

# Build the binary (proves the test file compiles into the package; also needed by buildStagecoach).
make build

# The rest of the repo (the new file is e2e-tagged → excluded from `make test`; prove no collateral).
make test    # -race ./...
make lint
# Expected: green (the new file is not compiled without -tags=e2e, so make test is unaffected).
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: build tag is e2e + !windows (Unix-only — SIGHUP/reparenting/watchdog are Unix concepts).
head -1 internal/e2e/orphan_reclaim_scenarios_test.go
# Expect: "//go:build e2e && !windows"

# Guard 2: the file is package e2e (white-box — reuses unexported harness primitives).
grep -n '^package e2e' internal/e2e/orphan_reclaim_scenarios_test.go
# Expect: 1 hit.

# Guard 3: NO production-package imports (these are subprocess tests; stagecoach IS the SUT).
grep -nE 'internal/(lock|watchdog|signal|generate|cmd|config)' internal/e2e/orphan_reclaim_scenarios_test.go && echo "FAIL: internal/* import in subprocess test" || echo "OK: no internal/* prod imports"
# Expect: OK (the helpers replicate lockPath/ppidOf locally rather than importing internal/lock).

# Guard 4: scenario (b) asserts exit 3 (rescue), NOT 129/143 (pre-snapshot codes unreachable at marker time).
grep -n 'ExitCode == 3\|ExitCode, want 3\|exit = %d, want 3' internal/e2e/orphan_reclaim_scenarios_test.go
grep -nE 'ExitCode == 129|ExitCode == 143|want 129|want 143' internal/e2e/orphan_reclaim_scenarios_test.go && echo "WARN: 129/143 asserted — unreachable after waitForMarker (snapshot armed)" || echo "OK: no 129/143 assertion"
# Expect: exit 3 asserted; NO 129/143 assertion.

# Guard 5: scenario (a)'s wrapper sleeps past watchdog.Arm (the race-defeating delay).
grep -n 'sleep 1.5\|sleep 2' internal/e2e/orphan_reclaim_scenarios_test.go
# Expect: ≥1 hit (the wrapper's parent-stays-alive delay).

# Guard 6: scenario (a)/(d) detect exit via waitProcessGone (reparented process can't be Wait'd).
grep -n 'waitProcessGone\|syscall.Kill(pid, 0)\|ESRCH' internal/e2e/orphan_reclaim_scenarios_test.go
# Expect: ≥1 hit.

# Guard 7: scenario (c) asserts lock-status STDOUT (runLockStatus writes to OutOrStdout, not stderr).
grep -n 'res.Stdout' internal/e2e/orphan_reclaim_scenarios_test.go | grep -iE 'no run lock|alive:|Lock:'
# Expect: lock-status assertions read res.Stdout.

# Guard 8: scenario (d) sets the opt-out env (FR-K6).
grep -n 'STAGECOACH_NO_PARENT_WATCHDOG' internal/e2e/orphan_reclaim_scenarios_test.go
# Expect: ≥1 hit (scenario d's stubEnv knob).

# Guard 9: XDG isolation flows to the subprocess (t.Setenv BEFORE stubEnv; lockFilePath uses XDG_RUNTIME_DIR).
grep -n 'XDG_RUNTIME_DIR' internal/e2e/orphan_reclaim_scenarios_test.go
# Expect: ≥1 hit (the isolation that makes lockFilePath match the subprocess's lock dir).

# Guard 10: scope — ONLY the new file.
git status --porcelain
# Expect: internal/e2e/orphan_reclaim_scenarios_test.go ONLY.
git diff --name-only | grep -vE '^internal/e2e/orphan_reclaim_scenarios_test\.go$' && echo "FAIL: out-of-scope file edited" || echo "OK: scope clean"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `GOOS=linux` + `GOOS=darwin` + `GOOS=windows` all clean (windows excludes the new file)
- [ ] `go vet ./internal/e2e/` clean; `gofmt -l` empty on the new file
- [ ] `make lint` zero errors (all file-local helpers are USED)
- [ ] `go test -tags=e2e ./internal/e2e/ -run 'TestE2EOrphan|TestE2ELockStatus' -v` green
- [ ] `go test -tags=e2e -race ./internal/e2e/` green (full e2e suite incl. contention)
- [ ] `go test -tags=e2e ./internal/e2e/ -run 'TestE2EOrphanReclaim' -count=3` green 3/3 (no timing flake)
- [ ] `make test` + `make build` green (no collateral — the new file is e2e-tagged)

### Feature Validation
- [ ] (a) Parent-death: reparented stagecoach self-exits (~10s); lock FILE removed; HEAD unchanged; index
      unchanged (a.txt still staged)
- [ ] (b) SIGHUP: stagecoach exits **3** (NOT 129); lock FILE removed; HEAD unchanged; index unchanged
      (grep guard 4)
- [ ] (c) lock status: NoLock→"no run lock for <repo>" (exit 0); Live→alive:true + pid/hostname/Lock present;
      Dead→alive:false + "orphaned:  unknown (holder is dead)"; Orphan→alive:true + orphaned:true (or skip)
- [ ] (d) Opt-out (STAGECOACH_NO_PARENT_WATCHDOG=1): commitCount==2 + HEAD subject "feat: a" (commit landed —
      the watchdog did NOT fire)
- [ ] All four deterministic — generous timeouts (marker 10s; process-gone 10–15s), outcome-based assertions,
      the snapshot-armed timing guarantee (generate.go L242<L335)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `internal/e2e/orphan_reclaim_scenarios_test.go` (grep guard 10)
- [ ] NO edit to harness_test.go, lock_scenarios_test.go, scenarios_test.go, hook_scenarios_test.go,
      config_precedence_test.go (existing e2e files)
- [ ] NO edit to any internal/* production file, main.go, root.go, go.mod, or any PRD/task file
- [ ] NO production-package import in the new file (grep guard 3 — subprocess tests; helpers replicate locally)
- [ ] NO docs (README/docs sync is P1.M4.T2.S1); NO orphan-hint assertion (P1.M3.T3.S1 drives it via its seam)

### Code Quality & Docs
- [ ] File-local helpers documented (startStagecoach = runStagecoach-minus-Run; waitProcessGone = ESRCH poll
      for reparented procs; lockFilePath replicates lock.lockPath; ppidOf mirrors orphan_unix.go)
- [ ] Each scenario has a header comment naming its FR (a→K1, b→K3, c→K4, d→K6) + the §20.5 rationale
- [ ] The genuine-orphan subtest is honestly documented as best-effort + subreaper-skip-guarded (no flake)
- [ ] Generous timeouts + outcome-based assertions (no brittle timing); `-count=3` proven non-flaky

---

## Anti-Patterns to Avoid

- ❌ Don't assert exit 129/143 for SIGHUP or parent-death sent AFTER `waitForMarker`. The snapshot is armed
  (generate.go:242) before Execute (line 335) runs the stub, so by marker time BOTH routes hit the POST-snapshot
  rescue → exit 3. Assert exit 3 for SIGHUP; assert OUTCOMES (not exit code) for the reparented parent-death case.
- ❌ Don't let the parent shell exit immediately in scenario (a). If sh exits before `watchdog.Arm` runs,
  `originalPpid` is captured as init → the ppid never changes → the watchdog never fires (the documented
  prctl/getppid race). The wrapper MUST `sleep ~1.5s` after backgrounding stagecoach to stay alive past Arm().
- ❌ Don't try to `cmd.Wait()` the reparented stagecoach in scenario (a)/(d). After the parent shell exits,
  stagecoach is reparented to init/a subreaper — the test is no longer its parent, so Wait can't reap it and
  the exit code is unreadable. Poll `syscall.Kill(pid, 0) == ESRCH` (`waitProcessGone`) and assert OUTCOMES
  (lock removed / HEAD unchanged-or-advanced), not the exit code.
- ❌ Don't import `internal/lock`/`internal/watchdog`/etc. in the new file. These are SUBPROCESS tests — the
  stagecoach binary is the SUT. Replicate `lockPath` (sha256 of EvalSymlinks) and `ppidOf` (`/proc` or `ps`)
  as file-local helpers. Importing internal/* would couple the e2e suite to internal package layout.
- ❌ Don't edit `harness_test.go`. Add `startStagecoach`/`waitForExit`/`waitProcessGone`/`lockFilePath`/
  `plantLockFile`/`ppidOf` as FILE-LOCAL helpers in the new file. Editing the shared harness churns the 4
  existing scenario files' invariants and widens the blast radius (scope guard enforces one new file).
- ❌ Don't `t.Setenv("XDG_RUNTIME_DIR", …)` AFTER calling `stubEnv`. `stubEnv` = `os.Environ()` + knobs reads
  the env at call time; a t.Setenv after it silently misses → the subprocess uses a DIFFERENT lock dir than
  `lockFilePath` computes → "lock removed" assertions fail spuriously. Set XDG FIRST, then stubEnv.
- ❌ Don't use a magic dead pid (e.g. "999999") in scenario (c) Dead. Capture a guaranteed-dead pid via
  `exec.Command("true").Start()` → grab `Process.Pid` → `Wait()` (reaped, dead). A magic number can (rarely)
  be recycled to a live process → `processAlive` returns true → false failure.
- ❌ Don't plant the Dead-pid lock with a foreign hostname. `processAlive(pid, hostname)` short-circuits to
  TRUE for a foreign/empty hostname (conservative — don't reap). Set hostname = THIS host (`os.Hostname()`)
  so `Kill(pid,0)` actually runs and returns ESRCH → alive=false.
- ❌ Don't make the genuine-orphan subtest (c Orphan) flake under a subreaper. `appearsOrphaned` returns true
  ONLY for ppid==1; under systemd/containers a reparented process's ppid is the subreaper. Verify `ppidOf`
  of the produced holder and `t.Skip` when ppid≠1. The RELIABLE core of (c) is NoLock/Live/Dead.
- ❌ Don't write lock-status assertions against `res.Stderr`. `runLockStatus` writes to `cmd.OutOrStdout()`.
  Assert `res.Stdout`.
- ❌ Don't skip `-count=3` (or similar repeat) for the timing-sensitive parent-death/opt-out scenarios. A
  single green run doesn't prove determinism. Run them ≥3×; if flaky, RAISE the stub sleep / poll timeout —
  never weaken the outcome assertions to mask a race.
- ❌ Don't add a `make e2e` target or otherwise touch the Makefile. The e2e suite is invoked via the raw
  `go test -tags=e2e ./internal/e2e/` (per the harness package doc). The Makefile is out of scope.
