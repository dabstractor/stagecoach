# P1.M4.T1.S1 — Orphaned-run lock reclamation e2e scenarios: research findings

Scope: a NEW e2e test file under `internal/e2e/` (build tag `//go:build e2e && !windows`) exercising
the LANDED FR-K1/K3/K4/K6 reclamation machinery (watchdog, SIGHUP, `lock status`, opt-out) against
REAL stagecoach subprocesses. Test-only; no production code changes.

## §0 — The harness primitives (internal/e2e/harness_test.go) — reuse, do NOT fork

All live in package `e2e`, build tag `//go:build e2e`:
- `buildStagecoach(t) string` — compiles `cmd/stagecoach` ONCE (sync.Once cache). The real binary.
- `buildStub(t) string` → `stubtest.Build(t)` — compiles `cmd/stubagent` ONCE.
- `newRepo(t) string` — `t.TempDir()` git repo with `user.name/email` set. UNIQUE path per test → unique lock hash.
- `seedCommit(t, repo, name, body)`, `writeFile`, `stageFile`, `runGit`, `headSHA`, `commitCount`, `statusPorcelain`.
- `writeStubConfig(t, stubBin, extras) string` — TOML with `[provider.stub]` pointing at the stub.
- `stubEnv(knobs map[string]string) []string` — `os.Environ()` + the `STAGECOACH_STUB_*` knobs. **Reads the
  current process env**, so `t.Setenv("XDG_RUNTIME_DIR", …)` BEFORE calling `stubEnv` flows to the subprocess.
- `runStagecoach(t, bin, repo, cfg, env, args...) e2eResult{Stdout,Stderr,ExitCode}` — runs to COMPLETION
  (60s ctx timeout), appends `--config cfg --no-color`. Returns exit code via `(*exec.ExitError).ExitCode()`.
- `waitForMarker(t, path, timeout)` — polls (20ms) for the stub's marker file. THE deterministic sync point.

**GAP**: `runStagecoach` runs to completion and returns no PID. Scenarios (a)/(b) need to (1) START
stagecoach without waiting, (2) send SIGHUP / detect parent-death, (3) THEN collect the exit. → add
file-local helpers `startStagecoach`/`waitForExit`/`waitProcessGone` in the new file (NOT in the shared
harness — keep blast radius to ONE new file).

## §1 — The stub agent (cmd/stubagent/main.go) — the blocking pattern

Sequence INSIDE the stub (locked ordering — this is what makes the races deterministic):
1. **Drain stdin FIRST** (deadlock guard — executor uses a ~64KiB pipe).
2. Write `STAGECOACH_STUB_MARKER` (if set) — **this is the "generation in-flight" signal the test waits on**.
3. Sleep `STAGECOACH_STUB_SLEEP_MS` (the block — holds the lock mid-generation).
4. Write `STAGECOACH_STUB_OUT` to stdout; exit `STAGECOACH_STUB_EXIT`.

So at `waitForMarker` return: stdin drained → marker written → stub is in its sleep → **generation is
mid-flight and the snapshot is armed** (see §2).

## §2 — CRITICAL: snapshot is armed BEFORE Execute runs the stub (determines exit codes)

`internal/generate/generate.go` `CommitStaged` ordering (the single-commit path; decompose mirrors it):
- L237 `treeSHA := deps.Git.WriteTree(ctx)` — freeze index into a tree object.
- L242 `signal.SetSnapshot(treeSHA, parentSHA, "")` — **ARMS the rescue path** (snapshot != "" → post-snapshot).
- L243 `lock.SetSnapshot(treeSHA)` — publishes the frozen tree for the no-op fast path.
- L335 `provider.Execute(ctx, *spec, …)` — runs the stub (which drains stdin → writes marker → sleeps).

**Consequence (deterministic exit codes at marker time)**:
- SIGHUP → `signal.handle` → `tree != ""` → rescue → **exit 3** + `OnRescueExit` (lock release). NOT 129.
- Parent-death watchdog fires → `signal.Trigger(SIGTERM)` → same `handle` → **exit 3** + lock release. NOT 143.
- Pre-snapshot 129/130/143 are NOT reachable once the marker exists. Tests that send SIGHUP AFTER
  `waitForMarker` → assert **exit 3**. (Do NOT assert 129/143 — they'd be wrong.)

The rescue path always calls `h.opts.OnRescueExit()` (signal.go handle, BOTH branches) → wired in
`cmd/stagecoach/main.go:61` to `lock.ReleaseCurrent` → `(*Locker).Release` → `os.Remove(path)`.
**So rescue ALWAYS removes the lock file.** The lock-removed assertion holds for both (a) and (b).

## §3 — The parent-death watchdog (internal/watchdog/) — the Arm() race (the hard part)

`watchdog.Arm(ctx, interval)` (default_action.go:108, gated `!cfg.NoParentWatchdog`, 1s cadence):
- `originalPpid := osGetppid()` captured AT ARM TIME, then a getppid poll fires when
  `osGetppid() != originalPpid` (a CHANGE — subreaper-safe; NOT `==1`, which is wrong under systemd-run/docker).
- Linux ALSO best-effort `prctl(PR_SET_PDEATHSIG, SIGTERM)` (pdeathsig_linux.go) — kernel delivers SIGTERM
  on parent death with no poll latency. Non-Linux (pdeathsig_nonlinux.go) is a no-op → poll is the only detector.

**THE RACE (must be handled in the test)**: if the parent dies BEFORE `Arm()` runs, `originalPpid` is
captured as init/the subreaper (the new parent), so there is NEVER a ppid change to detect → watchdog
never fires. (This is the documented prctl/getppid race; the man page warns to check `getppid()==1`
after PR_SET_PDEATHSIG.) Therefore the test's PARENT MUST STAY ALIVE PAST stagecoach's `Arm()` (~tens of
ms after start), THEN die. **Recipe**: wrap stagecoach so a parent shell backgrounds it, writes its PID,
`sleep`s ~1.5s (past Arm), THEN exits → stagecoach reparented → watchdog detects the change.

After reparenting, the test process is NOT stagecoach's parent (init/subreaper reaps it) → **the test
cannot `cmd.Wait()` it or read its exit code**. Detect exit by polling `syscall.Kill(pid, 0)` for ESRCH
(`waitProcessGone`). So scenario (a) asserts: process-gone + lock-removed + HEAD/index-unchanged — NOT
the exit code. (The SIGHUP scenario (b), where the test IS the parent, CAN read exit 3.)

Non-interactive `sh -c 'cmd &'` does NOT send SIGHUP to the backgrounded job on exit (SIGHUP-on-exit is
an interactive-shell/huponexit feature). So the backgrounded stagecoach is cleanly orphaned, not SIGHUP'd.
✓ (verified: bash/dash job control off in non-interactive mode).

## §4 — signal.Trigger + the rescue path (internal/signal/signal.go)

- `Trigger(sig)` → `active.Load().handle(sig)` (nil-safe no-op if no handler; stopped-guarded).
- `handle(sig)`: (1) `Kill(-childPID, sig)` → SIGTERM the child group (the stub dies); (2) `cancel()` the
  signal-aware ctx; (3) if snapshot armed → print rescue + `OnRescueExit` + **exit 3**; else `OnRescueExit`
  + `exitCodeForSignal(sig)` (129 SIGHUP / 130 SIGINT / 143 SIGTERM).
- `caughtSignals()` (signal_unix.go) includes SIGHUP on Unix → our handler catches it (not Go's default).
  So `syscall.Kill(stagecoachPid, syscall.SIGHUP)` → our handle → rescue exit 3.

## §5 — lock.Status + the `lock status` subcommand (internal/lock/lock.go, internal/cmd/lock.go)

`lock.Status(repoPath) (path, contents, alive, orphan, err)` — READ-ONLY (never acquires/breaks the flock):
- `path==""` (nil err) → no lock held.
- `alive = processAlive(pid, hostname)` (Kill(pid,0): nil/EPERM→true, ESRCH→false; foreign host→true).
- `orphan = appearsOrphaned(pid)` ONLY if alive: `ppid==1` → true, else false (CONSERVATIVE — false on
  any ambiguity; misses subreaper-reparented orphans, never false-positives).

`stagecoach lock status` output (internal/cmd/lock.go runLockStatus) — exact field labels to assert:
```
Lock: <path>
  pid:       <pid>
  hostname:  <host>
  repo:      <canonical repo>
  timestamp: <ts>
  snapshot:  <sha>            # ONLY if contents.Snapshot != ""
  alive:     <bool>
  orphaned:  true (holder reparented — launcher has exited)   # case orphan
  orphaned:  false                                            # case alive && !orphan
  orphaned:  unknown (holder is dead)                         # case !alive
```
No lock → `no run lock for <repoDir>` (exit 0). Uses `os.Getwd()` → subprocess MUST run with `cmd.Dir=repo`.
The `lock` group's no-op `PersistentPreRunE` OVERRIDES root's `config.Load` → works outside a git repo,
needs no `--config` (runStagecoach's `--config` is harmless/ignored). `runStagecoach(…, "lock", "status")`
works as-is (cmd.Dir=repo is set by runStagecoach).

## §6 — Lock path + isolation (so tests can assert "lock removed" / plant files)

`lock.lockDir()`: `XDG_RUNTIME_DIR`(abs) → `XDG_CACHE_HOME`(abs) → `~/.cache/stagecoach/locks` (NO CWD fallback).
`lock.lockPath(repo)`: `lockDir + sha256(EvalSymlinks(repo)) + ".lock"` (canonicalized via EvalSymlinks).

To KNOW the lock path in the e2e test (assert removal / plant a dead-pid file), replicate `lockPath`:
```go
func lockFilePath(repo string) string {
    canonical, err := filepath.EvalSymlinks(repo)
    if err != nil { canonical, _ = filepath.Abs(repo) }
    sum := sha256.Sum256([]byte(canonical))
    return filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "stagecoach", "locks", hex.EncodeToString(sum[:])+".lock")
}
```
AND isolate with `t.Setenv("XDG_RUNTIME_DIR", t.TempDir()); t.Setenv("XDG_CACHE_HOME", "")` BEFORE
`stubEnv` (so the value flows to the subprocess). This mirrors `lock_unix_test.go`'s `writeLockFile`
helper (format: `pid=…\nhostname=…\nrepo=…\ntimestamp=…\nsnapshot=…\n`) — `parseContents` reads these keys.

## §7 — Config: NoParentWatchdog opt-out (FR-K6) — the 3 knobs for scenario (d)

- ENV `STAGECOACH_NO_PARENT_WATCHDOG` (load.go:329 — presence-semantic, `strconv.ParseBool`, DIRECT set,
  can be false = escape hatch). → easiest for the e2e test (set in `stubEnv`).
- git config `stagecoach.noParentWatchdog` (git.go:188 — camelCase key).
- TOML `[generation].no_parent_watchdog` (file.go:69).
All only-true-propagates. Arming gate: `default_action.go:108  if !cfg.NoParentWatchdog { watchdog.Arm(ctx, …) }`.
With opt-out, `Arm` is NEVER called → no prctl, no poll goroutine → reparenting is undetected → stagecoach
runs the stub to completion and commits normally. Scenario (d) asserts HEAD ADVANCES (commit landed).

## §8 — Existing lock_scenarios_test.go patterns to mirror (the goroutine + drain idiom)

The 6 existing contention subtests (A–F) all use: spawn holder in a goroutine returning on a channel,
`waitForMarker`, poke the contender, then `<-resCh` to drain the holder. Reuse this EXACT structure for
the lock-status LIVE holder (hold a real lock, run `lock status`, drain). The NEW scenarios differ only
in: (a)/(d) a parent-shell wrapper that backgrounds + sleeps + exits; (b) start-without-wait + SIGHUP.

## §9 — The genuine-orphan (ppid==1) case is environment-dependent (be honest)

`appearsOrphaned` returns true ONLY for ppid==1. Under a subreaper (systemd, systemd-run, containers,
some shells) a reparented process's ppid is the subreaper, NOT 1 → `appearsOrphaned` returns false
(false negative — by design, conservative). So a genuine `orphaned: true` is reproducible ONLY when the
test environment reparents to init (ppid==1). The codebase already says so (orphan_unix.go comment;
P1.M3.T3.S1 drives the hint via a unit-test SEAM, not a real orphan). → the e2e orphan subtest must
**verify the produced holder's ppid==1 and `t.Skip` otherwise** (best-effort, no flake). The RELIABLE
core of scenario (c) is: no-lock / live / dead.

## §10 — Validation

- Build + run: `go test -tags=e2e ./internal/e2e/ -run 'TestE2EOrphan|TestE2ELockStatus' -v` (NO make
  target for e2e — it's the raw `go test -tags=e2e` invocation per the harness package doc).
- Race detector: `go test -tags=e2e -race ./internal/e2e/` (the harness uses subprocesses; -race is fine).
- Full e2e suite: `go test -tags=e2e ./internal/e2e/`.
- `go build ./...`, `go vet ./internal/e2e/`, `gofmt -l <new file>` must be clean.
- Scope guard: `git status --porcelain` shows ONLY the new `internal/e2e/orphan_reclaim_scenarios_test.go`.

## §11 — Sibling context (parallel-execution aware)

- P1.M3.T3.S1 (the Busy-message orphan hint + `lock.IsOrphaned`) is being implemented IN PARALLEL. It
  touches `internal/lock/lock.go` (+IsOrphaned), `internal/lock/lock_unix_test.go`, `internal/cmd/default_action.go`,
  `internal/cmd/lock_contention_test.go`. This item touches ONLY a NEW e2e file — **zero overlap**. The
  orphan hint itself is NOT asserted in e2e (P1.M3.T3.S1 drives it via its seam); this item asserts the
  underlying FR-K1/K3/K4/K6 machinery (watchdog/SIGHUP/lock-status/opt-out) end-to-end.
- `lock.Status`, `watchdog.Arm`, `signal.Trigger`, the `lock status` subcommand, the `NoParentWatchdog`
  config — all LANDED (P1.M1–P1.M3). This item CONSUMES them; it adds NO production code.
