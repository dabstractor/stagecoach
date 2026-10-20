name: "P1.M3.T1.S3 — false-kill regression (productive-slow survives) + hard-cap heartbeat pathology (FR-A1/FR-A2 proofs, tests-only)"
description: >
  TESTS-ONLY task (no production change, no docs — the item contract says "DOCS: none — tests only; Mock:
  stub agents only"). Prove the two discriminating behaviors of the v3.5 dual-deadline engine over the REAL
  generate.CommitStaged pipeline, using the S1-landed stub personas (commit ed94f2b) and the S2-landed parity
  harness (plan P1M3T1S2 — lands immediately before this task). RECIPE (b) FALSE-KILL REGRESSION (FR-A1: a
  productive-slow agent SURVIVES — the motivating v3.4 false-kill scenario): the progressive stub
  (stubtest.Options{EmitMS: 500, EmitLine: "\n", EmitTotalMS: 6000, Out: "feat: progressive answer"}) emits
  a stdout byte every ~0.5s for ~3× a tiny idle window (cfg.IdleTimeout=2s) then answers, under a LARGE hard
  cap (cfg.Timeout=30s) ⇒ generation SUCCEEDS and the commit IS created: err==nil, HEAD advanced,
  Result.Subject == "feat: progressive answer" (the "\n" tick makes the subject clean — ParseOutput strips
  leading newlines), verbose Execute-count == 1, NO "DEBUG: cause:" line, elapsed ≥ 6000ms (the run genuinely
  outlived the idle window ~3×). New test TestCommitStaged_ProductiveSlowEmitterSurvives APPENDED to
  internal/generate/generate_deadlines_test.go (package generate, internal — the established CommitStaged-
  deadline home whose header already documents the D4 platform split). D4 ARMING NOTE (the item's "stub's
  test manifest or Linux arming per the D4 rule"): the idle watcher arms iff Idle>0 AND (GOOS==linux OR
  spec.ProgressiveStdout) — and CmdSpec.ProgressiveStdout is ALWAYS false until P2.M1.T1.S1 (no manifest
  field can set it), so on Linux the watcher genuinely arms (rchar + pipe channels) and each 500ms tick
  resets the idle clock ~12× (the true FR-A1 discrimination); on macOS/Windows the watcher is DISARMED and
  the test degrades to a green-but-vacuous success-path smoke (D4's documented v3.4-degradation — correct,
  never red; the test comment MUST say so; never fabricate arming). RECIPE (c) HEARTBEAT PATHOLOGY (FR-A2:
  a false-alive heartbeat stream is BOUNDED — "activity is not progress"): the chatty-forever stub
  (Options{HeartbeatMS: 50} — stderr "tick" every 50ms forever, NEVER answers) under cfg.IdleTimeout=1s
  (deliberately SMALLER than the cap) + cfg.Timeout=1500ms (tiny hard cap) ⇒ the stderr ticks keep resetting
  the idle clock (20× margin) yet the HARD CAP kills at ~1.5s: cause line names "hard cap" — and because
  idle < hard, that cause PROVES activity was continuously observed while the run was still bounded. Two
  tests: a NEW "HardCapHeartbeat" row in internal/generate/invariants_test.go's TestInvariants (mirrors S2's
  "IdleKill" row: ErrTimeout + "DEBUG: cause: hard cap" + Execute-count==1 + the I1/I2/I3 no-mutation
  invariants via the harness) AND TestHardCapHeartbeat_ExitCodeParity APPENDED to internal/generate/
  idle_parity_test.go (S2's file, package generate_test — the EXTERNAL package required because
  internal/exitcode imports internal/generate): errors.As→*RescueError, errors.Is(generate.ErrTimeout),
  TreeSHA != "" + FormatRescue contains "Tree ID: "+TreeSHA, verbose "DEBUG: cause: hard cap", count==1,
  exitcode.For(err)==exitcode.Timeout (124), HEAD unchanged, elapsed ≤10s hang-guard — rescue parity exactly
  as in P1.M3.T1.S2 with the cause string swapped (NOTE: for a hard-cap kill the RescueError.Cause IS bare
  context.DeadlineExceeded — Error() "context deadline exceeded" does NOT name "hard cap"; the naming lives
  ONLY in the verbose cause line — assert it there). All three tests carry -short skip guards; ~9-10s added
  suite time. If ANY assertion fails, the defect is in the LANDED engine (P1.M2) or harness (S1/S2) — surface
  it, never patch the engine from here. Consumes: S1 personas (landed), S2 harness (lands first). Consumed
  by: nothing downstream (P1.M3.T1.S4 is config observability — independent).

---

## Goal

**Feature Goal**: Prove FR-A1 and FR-A2 — the two behaviors that justify the entire v3.5 idle+hard-cap redesign —
through the real CommitStaged pipeline with test-scaled deadlines and stub agents only:
- **FR-A1 (false-kill regression)**: an agent that emits a stdout byte every ~0.5s for well past a tiny idle
  window (idle 2s, emitting ~6s ≈ 3× idle) and then answers must **SURVIVE** — generation succeeds, the commit
  is created. This is the motivating v3.4 failure: a wall-clock-alone kill (the old 120s elapsed-time model)
  cannot distinguish this agent from a hung one; the activity-based idle deadline must.
- **FR-A2 (hard-cap heartbeat pathology)**: an agent that emits a steady stderr heartbeat forever and never
  answers must be **BOUNDED by the hard cap** despite continuous activity — activity is a productivity signal,
  not a progress guarantee. With idle (1s) deliberately SMALLER than the cap (1.5s), a kill whose verbose cause
  names "hard cap" simultaneously proves (on Linux, where the watcher arms) that the heartbeat's stderr bytes
  kept resetting the idle clock AND that the absolute cap still fired.

**Deliverable** (3 tests appended across 3 files; ZERO production change):
1. **internal/generate/generate_deadlines_test.go** — `TestCommitStaged_ProductiveSlowEmitterSurvives` (recipe b).
2. **internal/generate/invariants_test.go** — TestInvariants table row `"HardCapHeartbeat"` (recipe c, invariants).
3. **internal/generate/idle_parity_test.go** (S2's file) — `TestHardCapHeartbeat_ExitCodeParity` (recipe c, exit-124 parity).

**Success Definition**:
- Recipe (b): hung-window config (`IdleTimeout: 2s`, `Timeout: 30s`) + progressive stub (500ms ticks, 6s bound,
  then answer) ⇒ `err == nil`, HEAD advanced to `Result.CommitSHA`, `git log -1 --format=%s` ==
  "feat: progressive answer", verbose records exactly ONE `DEBUG: command: ` and NO `DEBUG: cause:` line,
  elapsed ≥ 6000ms.
- Recipe (c): heartbeat config (`IdleTimeout: 1s`, `Timeout: 1500ms`) + chatty-forever stub ⇒
  `*generate.RescueError` with `errors.Is(err, generate.ErrTimeout)`, non-empty `TreeSHA`, `FormatRescue`
  contains `"Tree ID: " + TreeSHA`, verbose contains `"DEBUG: cause: hard cap"`, Execute-count == 1,
  `exitcode.For(err) == exitcode.Timeout` (124), HEAD unchanged, I1/I2/I3 invariants hold, elapsed ≤ 10s.
- All existing tests stay green; `make test` + `make lint` green; `gofmt -l` empty; `git status` shows ONLY the
  3 test files.

## User Persona (if applicable)

**Target User**: The maintainer verifying the v3.5 engine's two headline guarantees end-to-end, and any future
contributor touching the idle watcher or the hard cap (these tests are the regression fence: a change that
re-introduces elapsed-time-only kills fails (b); a change that lets heartbeats starve the cap — or removes the
cap — fails (c)).

**Use Case**: A user runs stagecoach with a slow-but-healthy model (streaming partial output for minutes) — it
must NOT be killed while it is visibly working (FR-A1). A user's agent wedges into a heartbeat loop (periodic
stderr timer, never an answer — the codex-style pathology) — stagecoach must still terminate it at the hard cap
(FR-A2), never waiting forever because "it's still emitting".

**User Journey**: (b) agent emits → each byte stamps the countingWriter → idle clock resets → run outlives the
idle window 3× → answer arrives → commit created, exactly as a fast agent's run would. (c) agent heartbeats →
idle clock keeps resetting (never fires) → wall clock hits the cap → group SIGTERM → §18.3 rescue with tree SHA,
exit 124, verbose cause "hard cap (1.5s)".

**Pain Points Addressed**: §9.5's "NOT tested today" list (pre-v3.5): "nothing exercises progressive output or
heartbeat-style activity" through the pipeline. The engine landed (P1.M2) with seam-level tests; S2 proved idle-
kill PARITY (a kill is a kill); THIS task proves the two DISCRIMINATING behaviors — that activity prevents the
idle kill (b), and that activity cannot prevent the hard kill (c).

## Why

- **FR-A1/FR-A2 are the spec's justification for the whole redesign** (spec §9.30): "a wall clock alone cannot
  distinguish a hung agent from a productive-but-slow one" (FR-A1) and "the hard cap [is] the backstop for
  false-alive pathologies (heartbeat-only streams)" (FR-A2). Neither has a pipeline-level proof today.
- **Regression fence value**: (b) fails if anyone reintroduces elapsed-time-only killing or breaks the
  LastWrite reset; (c) fails if activity ever resets the HARD cap (it must not — the WithTimeout shadow is
  activity-independent) or if the cap is weakened. Both are cheap (~9-10s), hermetic (stub agents only), and
  -short-skippable.
- **Completes the milestone's proof triple with S2**: S2 = an idle kill behaves EXACTLY like a timeout
  downstream (parity); S3 = the idle deadline spares the productive (discrimination) and the hard cap bounds
  the chatty (backstop). Together they pin FR-A1/A2/A5 end-to-end.

## What

**User-visible behavior**: None (tests only).

**Technical change**: 3 tests appended to 3 existing/new-from-S2 files in internal/generate. See the Blueprint.

### Success Criteria
- [ ] `internal/generate/generate_deadlines_test.go` gains `TestCommitStaged_ProductiveSlowEmitterSurvives`:
      stub `Options{EmitMS: 500, EmitLine: "\n", EmitTotalMS: 6000, Out: "feat: progressive answer"}`;
      `cfg = config.Defaults(); cfg.IdleTimeout = 2 * time.Second; cfg.Timeout = 30 * time.Second`;
      `deps.Verbose = ui.NewVerbose(&vbuf, true)`; asserts `err == nil`, `res.Subject` AND `res.Message` ==
      "feat: progressive answer", HEAD advanced to `res.CommitSHA` (`git rev-parse HEAD`), last-commit subject
      matches, `strings.Count(vbuf.String(), "DEBUG: command: ") == 1`, `!strings.Contains(vbuf.String(),
      "DEBUG: cause:")`, `elapsed >= 6000*time.Millisecond`; `-short` skip; a header comment carrying the D4
      arming note (Linux: armed + genuinely discriminating; darwin/windows: disarmed, green-but-vacuous).
- [ ] `internal/generate/invariants_test.go` TestInvariants gains the `"HardCapHeartbeat"` row: stub
      `Options{HeartbeatMS: 50, Out: "feat: never"}`; `cfg.IdleTimeout = 1 * time.Second; cfg.Timeout =
      1500 * time.Millisecond`; inside the row assert `errors.Is(err, ErrTimeout)` + verbose contains
      `"DEBUG: cause: hard cap"` + `"DEBUG: command: "` count == 1; `return err, ""` (→ treeSHAFromErr +
      assertInvariants I1/I2/I3); `-short` skip.
- [ ] `internal/generate/idle_parity_test.go` (S2's file — APPEND; if absent because S2 renamed it, append to
      S2's actual external parity file) gains `TestHardCapHeartbeat_ExitCodeParity`: same stub/config as the
      invariants row + repo setup; asserts `errors.As → *generate.RescueError`, `errors.Is(generate.ErrTimeout)`,
      `re.TreeSHA != ""`, `generate.FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate)` contains
      `"Tree ID: " + re.TreeSHA`, verbose contains `"DEBUG: cause: hard cap"`, Execute-count == 1,
      `exitcode.For(err) == exitcode.Timeout`, HEAD unchanged, `elapsed <= 10*time.Second`; `-short` skip.
- [ ] ZERO production/engine change; `git status` shows ONLY the 3 test files; `make test` + `make lint` green;
      `gofmt -l` empty; all existing tests (incl. S2's, unchanged) stay green.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact landed engine semantics (Deadlines struct, the D4 arming rule with its ProgressiveStdout-
always-false-until-P2 consequence, the two VerboseCause strings `"idle deadline (%v without observable
activity)"` / `"hard cap (%v)"`, ErrIdleDeadline vs bare DeadlineExceeded, the 500ms watcher poll), the S1
knob semantics (EmitMS/EmitLine/EmitTotalMS/HeartbeatMS — tick cadence, the answer-as-suffix rule, the
heartbeat-never-writes-stdout rule, no-buffering), the S2 contract (its file/package layout, its
parity-assertion set this task mirrors, its helpers to reuse), the D4 platform analysis (which assertions are
meaningful on Linux vs vacuous-but-green on darwin/windows — and that this is by design, never fabricated),
the timing margins (500ms ticks vs 2s idle = 4×; 50ms heartbeat vs 1s idle = 20×; idle < hard as the (c)
discriminator), the ParseOutput newline-strip fact that makes EmitLine "\n" yield a clean subject, the import
cycle that forces the external package for the exitcode assertion, and the in-package helper names
(initRepo/commitRaw/writeFile/stageFile/headSHA) with their file homes.

### Documentation & References

```yaml
# MUST READ — this task's own findings (engine facts, S2 contract, platform analysis, timing design, parse detail)
- docfile: plan/022_856008a8cbb5/P1M3T1S3/research/findings.md
  why: "§1 the landed engine (Deadlines/arming rule/cause strings/kill mechanics); §2 the S1 persona table
        (exact Options fields + behavior); §3 the S2 contract (what lands before this task + its assertion
        set); §4 the existing generate deadline tests to mirror and NOT duplicate; §5 THE platform/D4
        analysis (Linux-armed vs darwin/windows-disarmed for both recipes — the item's 'D4 rule' note);
        §6 timing margins; §7 the EmitLine-\"\n\" parse detail (clean subject); §8 the file/package decision
        table; §9 Result fields for success assertions; §10 scope fences."
  critical: "§5 + §7 are the two facts an implementer will get wrong without them: (1) on non-Linux the idle
             watcher is DISARMED for stub tests (ProgressiveStdout is always false until P2.M1.T1.S1) — the
             tests stay green and correct but the discrimination is Linux-carried, and the test comments must
             say so; (2) ticks and the answer share one stdout buffer — EmitLine \"\\n\" makes ParseOutput
             strip the ticks so the subject is assertable by equality."

# MUST READ — the S2 contract (this task appends to its file and mirrors its assertions)
- docfile: plan/022_856008a8cbb5/P1M3T1S2/PRP.md
  why: "Defines internal/generate/idle_parity_test.go (package generate_test — external, REQUIRED for the
        exitcode import because internal/exitcode imports internal/generate), its own repo helpers
        (initTempRepo/writeFile/stageFile/headSHA mirroring hooks_freeze_test.go), the TestIdleKill_
        ExitCodeParity assertion set (this task's (c) mirrors it with 'idle deadline' → 'hard cap'), the
        'IdleKill' invariants row shape, the no-retry counting technique ('DEBUG: command: ' occurrences),
        and the LARGE/SMALL discriminator idiom."
  critical: "For a HARD-CAP kill re.Cause IS bare context.DeadlineExceeded (Error() 'context deadline exceeded'
             — names nothing); the 'hard cap' naming lives ONLY in the verbose cause line. Do not assert
             Cause.Error() contains 'hard'."

# MUST READ — the recipes' source + the error contract
- docfile: plan/022_856008a8cbb5/architecture/call_sites_and_error_contract.md
  section: "§9.6 recipes (b)/(c) (the stub personas this task consumes); §3 exit-124 mapping (exitcode.For
            order; FormatRescue's 'Tree ID:' line); §4 no-retry-on-timeout (generate.go:376 '§5: immediate
            rescue, NO retry'); §9.3 the invariants harness (assertInvariants I1/I2/I3, treeSHAFromErr)."
  critical: "A deadline kill returns from INSIDE the retry loop on the first hit ⇒ exactly ONE
             'DEBUG: command: ' line — that count is the no-retry proof in every test."

# MUST EDIT — recipe (b) home (the established CommitStaged-deadline test file)
- file: internal/generate/generate_deadlines_test.go   # package generate (internal); APPEND the new test
  why: "Home of TestCommitStaged_MessageRoleIdleTimeout — its header already documents the D4 platform split
        ('on Linux the idle watcher arms (D4) and kills; on darwin/windows print-mode the idle is DISARMED'),
        and it demonstrates the in-package helper set + the config.Defaults()+cfg.Timeout+cfg.IdleTimeout
        idiom. The new test inverts its expectation: the SAME kind of config but with ACTIVITY ⇒ success."
  pattern: "initRepo(t, repo); commitRaw(t, repo, \"initial\"); writeFile(t, repo, \"z.txt\", \"data\");
            stageFile(t, repo, \"z.txt\"); before := headSHA(t, repo); CommitStaged(context.Background(),
            Deps{Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&vbuf, true)}, cfg)."
  gotcha: "Add the internal/ui + bytes imports if absent (the existing test passes no Verbose). Keep the
           -short skip guard idiom."

# MUST EDIT — recipe (c) invariants row
- file: internal/generate/invariants_test.go   # the TestInvariants table; S2's 'IdleKill' row is the neighbor
  why: "Rows are {name, run func(t, ctx, repo, bin) (error, string)}; the harness funnels the returned error
        through treeSHAFromErr + assertInvariants (I1 idempotent index / I2 atomic HEAD / I3 snapshot
        immutability) — the no-mutation proof for the hard-cap kill comes FREE."
  pattern: "Mirror S2's 'IdleKill' row exactly, swapping the stub (HeartbeatMS for SleepMS), the knobs
            (IdleTimeout 1s / Timeout 1500ms), and the cause substring ('hard cap' for 'idle deadline')."
  gotcha: "Build Deps INSIDE the run closure; declare the verbose buffer inside run; `return err, \"\"`.
            The row lands AFTER S2's IdleKill row in the table."

# MUST EDIT — recipe (c) exit-124 parity test (S2's file — APPEND)
- file: internal/generate/idle_parity_test.go   # package generate_test (external); created by S2 — append
  why: "The ONLY place in internal/generate that can import internal/exitcode (external package breaks the
        cycle: exitcode imports generate). Reuse its helpers (initTempRepo/writeFile/stageFile/headSHA) and
        mirror TestIdleKill_ExitCodeParity's assertion ladder verbatim with the cause substring swapped."
  pattern: "generate.CommitStaged + generate.Deps + generate.RescueError + generate.FormatRescue +
            exitcode.For/exitcode.Timeout; everything generate.X-qualified."
  gotcha: "If S2's file name differs at implementation time (check `ls internal/generate/*parity*` /
            `grep -rl 'generate_test' internal/generate/`), append to S2's ACTUAL external parity file —
            never create a second external package fragment."

# CONTEXT — the landed engine under observation (READ-ONLY)
- file: internal/provider/executor.go   # Deadlines (:50), ErrIdleDeadline (:58-70), Execute (:104+),
        #   the D4 arming branch (~:144), the two VerboseCause calls in the Wait-error path
  why: "The engine being proven. Cause strings: idle → 'idle deadline (%v without observable activity)';
        hard cap → 'hard cap (%v)' (emitted when runCtx.Err() is DeadlineExceeded and the idleKilled flag is
        false). Hard cap = the WithTimeout shadow on dl.Hard — NEVER reset by activity. Idle watcher = 500ms
        ticker; countingWriter LastWrite stamps on EVERY stdout/stderr pipe byte."
  critical: "READ-ONLY. If any assertion fails, the defect is HERE (P1.M2's engine) or in S1/S2's harness —
             surface it in the report; do NOT modify the executor."

# CONTEXT — the stub personas (S1, LANDED commit ed94f2b — consume, don't rebuild)
- file: internal/stubtest/stubtest.go   # Options: EmitMS/EmitLine/EmitTotalMS/HeartbeatMS/HeartbeatTotalMS
- file: cmd/stubagent/main.go           # phases 1d (EMIT) / 1e (HEARTBEAT)
  why: "EmitMS>0 + EmitTotalMS>0 ⇒ stdout ticks every EmitMS for EmitTotalMS, THEN the final Out answer as
        the stdout SUFFIX. HeartbeatMS>0 ⇒ stderr 'tick' every interval forever (HeartbeatTotalMS 0), NEVER
        stdout. Ticks are unbuffered pipe writes (no bufio) — each is immediate observable activity."
  critical: "READ-ONLY (S1 scope, landed). If a persona misbehaves, that's an S1 regression — report, don't
             patch from here."

# CONTEXT — config knobs + exit code (READ-ONLY)
- file: internal/config/roles.go / config.go   # cfg.IdleTimeout (default 60s), cfg.Timeout (default 600s),
        #   ResolveRoleDeadlines — tests set BOTH knobs explicitly, never rely on defaults
- file: internal/exitcode/exitcode.go          # For() (:63) — ErrTimeout checked BEFORE ErrRescue; Timeout=124
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  generate_deadlines_test.go   # EDIT — +TestCommitStaged_ProductiveSlowEmitterSurvives (recipe b; package generate)
  invariants_test.go           # EDIT — +"HardCapHeartbeat" TestInvariants row (recipe c; package generate)
  idle_parity_test.go          # EDIT (S2's NEW file) — +TestHardCapHeartbeat_ExitCodeParity (recipe c; package generate_test)
  generate_test.go             # READ-ONLY — in-package helpers (initRepo/commitRaw/writeFile/stageFile/headSHA)
  hooks_freeze_test.go         # READ-ONLY — the external-package precedent (if S2's helpers need re-deriving)
internal/provider/
  executor.go                  # READ-ONLY — the LANDED engine under observation (D4 arming, cause lines)
  executor_idle_linux_test.go  # READ-ONLY — the rchar-reset seam proof (context; do not duplicate)
internal/stubtest/stubtest.go  # READ-ONLY — S1 personas (Emit*/Heartbeat* Options fields)
cmd/stubagent/main.go          # READ-ONLY — S1 phases 1d/1e
internal/exitcode/exitcode.go  # READ-ONLY — For() + Timeout=124
internal/config/               # READ-ONLY — IdleTimeout/Timeout knobs
# go.mod / Makefile — READ-ONLY (no new deps; stdlib + existing internal imports only)
```

### Desired Codebase tree with files to be modified

```bash
internal/generate/generate_deadlines_test.go   # MODIFIED — +1 test (recipe b, success path)
internal/generate/invariants_test.go           # MODIFIED — +1 table row ("HardCapHeartbeat")
internal/generate/idle_parity_test.go          # MODIFIED — +1 test (recipe c, exit-124 parity)
# (NO production file. NO engine change. NO stub change. NO docs. NO new files.)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the D4 arming rule — which platforms the tests actually discriminate on): the idle watcher arms
// iff dl.Idle > 0 AND (runtime.GOOS == "linux" || spec.ProgressiveStdout) — executor.go:144. CmdSpec.
// ProgressiveStdout is ALWAYS false until P2.M1.T1.S1 (render.go:43's comment says so; no manifest field
// sets it), and CommitStaged renders internally so a test cannot inject it. ⇒ On LINUX the watcher is armed
// (rchar + pipe channels) and both recipes genuinely discriminate (ticks reset the idle clock). On
// DARWIN/WINDOWS the watcher is DISARMED: recipe (b) degrades to a green-but-vacuous success smoke (idle
// cannot fire at all — D4's documented v3.4 degradation), and recipe (c) still kills via the platform-
// independent hard cap with the same cause line. Both tests stay GREEN and CORRECT everywhere; the FR-A1/A2
// discrimination is Linux-carried (CI runs Linux). Each test's comment MUST state this (the item's
// "note the stub's test manifest or Linux arming per the D4 rule"). NEVER try to force arming — "stagecoach
// never fabricates activity" (D4).

// CRITICAL (recipe (c): make idle SMALLER than the hard cap — that ordering IS the proof): cfg.IdleTimeout=1s
// < cfg.Timeout=1500ms. Without the heartbeat's resets the idle deadline would fire FIRST; a verbose cause
// naming "hard cap" therefore PROVES (on Linux) that the stderr bytes kept resetting the idle clock while
// the absolute cap still bounded the run — FR-A2's "activity is not progress" in one assertion. With idle >
// hard the test would pass trivially and prove nothing.

// CRITICAL (the hard-cap cause naming lives ONLY in the verbose line): for a hard-cap kill Execute returns
// BARE context.DeadlineExceeded — RescueError.Cause.Error() == "context deadline exceeded", which names
// nothing. The "hard cap (1.5s)" string is emitted by vb.VerboseCause → the verbose buffer line
// "DEBUG: cause: hard cap (1.5s)". Assert strings.Contains(vbuf.String(), "DEBUG: cause: hard cap") — never
// assert Cause.Error() contains "hard".

// CRITICAL (ticks and the answer share ONE stdout buffer — use EmitLine "\n"): the progressive stub writes
// ticks and then the final Out into the same stream, so captured stdout = tick bytes + answer. With
// EmitLine "\n" each tick is a bare newline and ParseOutput's TrimSpace strips them ⇒ Message and Subject
// are EXACTLY the Out value — assert equality. With the default "." tick the subject would be
// ".........feat: progressive answer" (only Contains assertions possible). Duplicate-reject compares the
// subject against repo history ("initial") — no collision; the empty-output retry never fires.

// CRITICAL (the import cycle — exit-124 assertions live ONLY in package generate_test): internal/exitcode
// imports internal/generate, so a package-generate test file CANNOT import exitcode. TestHardCapHeartbeat_
// ExitCodeParity MUST live in S2's idle_parity_test.go (package generate_test). The invariants row and the
// (b) test stay in package generate (they need no exitcode).

// CRITICAL (no-retry proof = count "DEBUG: command: "): provider.Execute emits exactly one
// "DEBUG: command: <argv>" line per invocation. A deadline kill returns from INSIDE the retry loop on the
// first hit ⇒ count == 1. A parse-fail retry would show 2+. Requires deps.Verbose = ui.NewVerbose(&buf, true)
// (nil Verbose makes every verbose assertion a silent no-op).

// GOTCHA (timing margins vs the 500ms watcher poll): the watcher polls every ~500ms; the FIRST tick only
// initializes the baseline. Recipe (b): 500ms ticks vs 2s idle = 4× margin (≥3 ticks land between polls) —
// safe on loaded CI. Recipe (c): 50ms heartbeat vs 1s idle = 20× margin. Elapsed floors prove survival
// ((b): elapsed ≥ 6000ms — the run outlived the idle window 3×); elapsed ceilings guard hangs ((c): ≤10s
// vs the ~1.5s expected kill + ≤3s WaitDelay SIGKILL grace).

// GOTCHA (every test needs the -short skip guard): mirror the Timeout row's
// `if testing.Short() { t.Skip("skipping slow … in -short mode") }` — the three tests add ~9-10s.

// GOTCHA (S2 lands first — append, don't collide): idle_parity_test.go and the "IdleKill" invariants row are
// S2's deliverables and will be in the tree before this task starts. APPEND to them; never edit S2's tests.
// If S2's external file has a different name, find it (`grep -rl 'package generate_test' internal/generate/`)
// and append there.

// GOTCHA (fresh repo per test — duplicate-reject hygiene): the (b) repo has exactly one "initial" commit;
// the generated subject "feat: progressive answer" cannot collide with the last-50 subjects, so the
// duplicate-retry loop (§9.7) never consumes attempts — keeping the Execute-count == 1 assertion honest.
```

## Implementation Blueprint

### Data models and structure

None (tests only). The tests consume the LANDED types: `stubtest.Options` (EmitMS/EmitLine/EmitTotalMS/
HeartbeatMS), `config.Defaults()` + `cfg.IdleTimeout`/`cfg.Timeout`, `generate.Deps{Git, Manifest, Verbose}`,
`generate.Result{CommitSHA, Subject, Message, ...}`, `*generate.RescueError{Kind, TreeSHA, ParentSHA,
Candidate, Cause}`, `generate.FormatRescue`, `exitcode.For`/`exitcode.Timeout`, `ui.NewVerbose(&buf, true)`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/generate/generate_deadlines_test.go — +TestCommitStaged_ProductiveSlowEmitterSurvives (recipe b)
  - APPEND after TestCommitStaged_MessageRoleIdleTimeout. Shape (adapt names/comments to file style):
        // TestCommitStaged_ProductiveSlowEmitterSurvives proves §9.30 FR-A1 (the false-kill regression — the
        // motivating v3.4 scenario): a productive-but-slow agent that emits a stdout byte every 500ms for
        // ~3× the idle window (idle 2s, emitting 6s) and THEN answers must SURVIVE — generation succeeds and
        // the commit is created. The v3.4 elapsed-time-only kill (120s wall clock) could not distinguish this
        // agent from a hung one; the activity-based idle deadline must.
        //
        // D4 arming note: the idle watcher arms iff Idle>0 AND (linux || spec.ProgressiveStdout); ProgressiveStdout
        // is always false until P2 (event-stream), so on LINUX the watcher is armed and each 500ms tick resets the
        // idle clock ~12× — this test genuinely discriminates there. On darwin/windows the watcher is DISARMED
        // (D4's v3.4 degradation): the idle deadline cannot fire at all and this test degrades to a green
        // success-path smoke — correct on every platform, never red, discrimination Linux-carried (CI).
        func TestCommitStaged_ProductiveSlowEmitterSurvives(t *testing.T) {
            if testing.Short() { t.Skip("skipping slow false-kill regression in -short mode") }
            bin := stubtest.Build(t); repo := t.TempDir()
            initRepo(t, repo); commitRaw(t, repo, "initial")
            writeFile(t, repo, "z.txt", "data"); stageFile(t, repo, "z.txt")
            before := headSHA(t, repo)
            var vbuf bytes.Buffer
            m := stubtest.Manifest(bin, stubtest.Options{
                EmitMS: 500, EmitLine: "\n", EmitTotalMS: 6000, // tick every 0.5s for ~3× the 2s idle…
                Out: "feat: progressive answer",                       // …then answer (stdout suffix)
            })
            cfg := config.Defaults()
            cfg.IdleTimeout = 2 * time.Second   // SMALL idle: a SILENT agent dies at ~2-2.5s
            cfg.Timeout = 30 * time.Second      // LARGE hard cap: mathematically cannot fire (~6.5s run)
            start := time.Now()
            res, err := CommitStaged(context.Background(),
                Deps{Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&vbuf, true)}, cfg)
            elapsed := time.Since(start)
            if err != nil { t.Fatalf("productive-slow emitter was killed / failed: %v (FR-A1 false-kill regression)", err) }
            // The commit IS created.
            if res.Subject != "feat: progressive answer" { t.Errorf("Subject = %q, want %q", res.Subject, "feat: progressive answer") }
            if res.Message != "feat: progressive answer" { t.Errorf("Message = %q, want %q (EmitLine \"\\n\" ticks strip in ParseOutput)", res.Message) }
            if got := headSHA(t, repo); got == before || got != res.CommitSHA {
                t.Errorf("HEAD = %s, want advanced to CommitSHA %s (before %s)", got, res.CommitSHA, before)
            }
            if subj := commitSubject(t, repo); subj != "feat: progressive answer" {   // git log -1 --format=%s helper (see note)
                t.Errorf("git log -1 subject = %q, want %q", subj, "feat: progressive answer")
            }
            // Single Execute, nothing killed.
            if n := strings.Count(vbuf.String(), "DEBUG: command: "); n != 1 { t.Errorf("Execute count = %d, want 1", n) }
            if strings.Contains(vbuf.String(), "DEBUG: cause:") { t.Errorf("a deadline fired on a productive run; buf:\n%s", vbuf.String()) }
            // The run genuinely outlived the idle window ~3× (the false-kill discriminator floor).
            if elapsed < 6000*time.Millisecond { t.Errorf("elapsed = %v, want ≥ 6s (emit phase must complete)", elapsed) }
        }
  - NOTE on commitSubject: if the package has no git-log-%s helper, inline `git.New(repo).Run("log", "-1",
    "--format=%s")`-equivalent via exec or the existing git wrapper — check generate_test.go for the idiom
    first (headSHA exists; a subject helper may too). Inlining `exec.Command("git", "-C", repo, "log", "-1",
    "--format=%s")` is acceptable if no wrapper fits.
  - IMPORTS: add "bytes" + internal/ui if absent (errors/strings/time/config/git/stubtest already in the block
    — verify).

Task 2: EDIT internal/generate/invariants_test.go — the "HardCapHeartbeat" TestInvariants row (recipe c)
  - ADD to the table immediately AFTER S2's "IdleKill" row (same shape; that row IS the doc):
        {
            name: "HardCapHeartbeat",
            run: func(t *testing.T, _ context.Context, repo, bin string) (error, string) {
                if testing.Short() { t.Skip("skipping slow HardCapHeartbeat scenario in -short mode") }
                var vbuf bytes.Buffer
                // Persona (c): stderr heartbeat every 50ms forever, NEVER answers — the false-alive pathology.
                m := stubtest.Manifest(bin, stubtest.Options{HeartbeatMS: 50, Out: "feat: never"})
                cfg := config.Defaults()
                cfg.IdleTimeout = 1 * time.Second        // SMALLER than the cap: without the heartbeat's resets the IDLE would fire first
                cfg.Timeout = 1500 * time.Millisecond    // tiny hard cap: the absolute backstop (activity never resets it)
                deps := Deps{Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&vbuf, true)}
                _, err := CommitStaged(context.Background(), deps, cfg)
                if err != nil {
                    if !errors.Is(err, ErrTimeout) { t.Errorf("HardCapHeartbeat: errors.Is(err, ErrTimeout) = false, got %v", err) }
                    // FR-A2: the cause names the HARD CAP — continuous activity kept the idle clock resetting
                    // (idle 1s < cap 1.5s) yet the absolute cap still bounded the run. Activity is not progress.
                    if !strings.Contains(vbuf.String(), "DEBUG: cause: hard cap") {
                        t.Errorf("HardCapHeartbeat: verbose missing hard-cap cause line; buf=\n%s", vbuf.String())
                    }
                    if n := strings.Count(vbuf.String(), "DEBUG: command: "); n != 1 {
                        t.Errorf("HardCapHeartbeat: Execute called %d times, want 1 (immediate rescue, NO retry)", n)
                    }
                }
                return err, "" // → treeSHAFromErr + assertInvariants (I1/I2/I3: no index/HEAD mutation, snapshot immutable)
            },
        },
  - Follows S2's IdleKill row verbatim (imports ui/bytes already added by S2; verify).
  - NOTE: on non-Linux the watcher is disarmed and only the cap can fire — same cause line; the row is
    correct everywhere (add one comment line noting it, mirroring the recipe-b test comment).

Task 3: EDIT internal/generate/idle_parity_test.go — +TestHardCapHeartbeat_ExitCodeParity (recipe c, exit-124)
  - APPEND after S2's TestIdleKill_ExitCodeParity; MIRROR its assertion ladder with the cause substring
    swapped ("idle deadline" → "hard cap") and the stub/config swapped:
        func TestHardCapHeartbeat_ExitCodeParity(t *testing.T) {
            if testing.Short() { t.Skip("skipping slow hard-cap heartbeat parity in -short mode") }
            repo := initTempRepo(t); writeFile+stageFile a change (reuse the file's helpers); before := headSHA(t, repo)
            var vbuf bytes.Buffer
            m := stubtest.Manifest(bin, stubtest.Options{HeartbeatMS: 50, Out: "feat: never"}) // chatty-forever
            cfg := config.Defaults()
            cfg.IdleTimeout = 1 * time.Second; cfg.Timeout = 1500 * time.Millisecond
            start := time.Now()
            _, err := generate.CommitStaged(context.Background(), generate.Deps{
                Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&vbuf, true)}, cfg)
            elapsed := time.Since(start)
            var re *generate.RescueError
            if !errors.As(err, &re) { t.Fatalf("err = %v, want *generate.RescueError", err) }
            if !errors.Is(err, generate.ErrTimeout) { t.Errorf("errors.Is(ErrTimeout)=false: %v", err) }
            if re.TreeSHA == "" { t.Error("TreeSHA empty — snapshot missing from the rescue") }
            if msg := generate.FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate); !strings.Contains(msg, "Tree ID: "+re.TreeSHA) {
                t.Errorf("FormatRescue missing 'Tree ID: %s'; msg=\n%s", re.TreeSHA, msg)
            }
            // The cause line is the ONLY place a hard-cap kill names itself (Cause IS bare DeadlineExceeded).
            if !strings.Contains(vbuf.String(), "DEBUG: cause: hard cap") { t.Errorf("missing hard-cap cause line; buf:\n%s", vbuf.String()) }
            if code := exitcode.For(err); code != exitcode.Timeout {
                t.Errorf("exitcode.For(err) = %d, want %d (124)", code, exitcode.Timeout)
            }
            if n := strings.Count(vbuf.String(), "DEBUG: command: "); n != 1 { t.Errorf("Execute count = %d, want 1", n) }
            if got := headSHA(t, repo); got != before { t.Errorf("HEAD moved: %s → %s", before, got) }
            if elapsed > 10*time.Second { t.Errorf("hard-cap kill took %v; want ≤ ~1.5s+grace (hang-guard)", elapsed) }
        }
  - Everything generate.X-qualified (external package); bin := stubtest.Build(t) first.
  - FILE-FIND NOTE: if idle_parity_test.go is absent, locate S2's actual external file
    (`grep -rl 'package generate_test' internal/generate/`) and append there; never create a competing one.

Task 4: VERIFY — focused runs, determinism, full gates
  - go test ./internal/generate/ -run 'TestCommitStaged_ProductiveSlowEmitterSurvives|TestInvariants|TestHardCapHeartbeat_ExitCodeParity|TestIdleKill' -v -count=1
  - go test ./internal/generate/ -count=1   # S2's tests + everything else untouched
  - make test ; make lint ; gofmt -l on the 3 files ; git status --short (only the 3 test files)
  - Optional local D4 confidence: run the suite on Linux (or note CI carries it): GOOS check via `go env GOOS`.
```

### Implementation Patterns & Key Details

```go
// PATTERN (recipe b — the survival proof): the config makes a SILENT agent die at ~2s; only ACTIVITY
// explains survival. The verbose buffer doubles as the kill-detector (no cause line ⇒ nothing fired).
cfg.IdleTimeout = 2 * time.Second; cfg.Timeout = 30 * time.Second
res, err := CommitStaged(ctx, Deps{Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&vbuf, true)}, cfg)
// err MUST be nil; HEAD MUST have advanced; exactly ONE Execute; ZERO cause lines; elapsed ≥ 6s.

// PATTERN (recipe c — the discriminator ordering): idle < hard + heartbeat ⇒ the cause line identifies
// WHICH deadline won, and "hard cap" certifies the idle clock was being reset the whole time.
cfg.IdleTimeout = 1 * time.Second; cfg.Timeout = 1500 * time.Millisecond  // idle SMALLER than the cap
// …kill at ~1.5s ⇒ errors.Is(err, ErrTimeout) && strings.Contains(vbuf, "DEBUG: cause: hard cap").

// PATTERN (assert the verbose cause, not Cause.Error(), for hard-cap kills):
// re.Cause == context.DeadlineExceeded (bare) → Error() "context deadline exceeded" (names nothing).
// vb.VerboseCause(fmt.Sprintf("hard cap (%v)", dl.Hard)) → verbose line "DEBUG: cause: hard cap (1.5s)".
if !strings.Contains(vbuf.String(), "DEBUG: cause: hard cap") { t.Errorf("…") }
```

### Integration Points

```yaml
GENERATE PACKAGE (internal/generate):
  - generate_deadlines_test.go: +1 test (recipe b — success path; package generate).
  - invariants_test.go: +1 table row "HardCapHeartbeat" (recipe c — invariants via the existing harness).
  - idle_parity_test.go (S2's): +1 test (recipe c — exit-124 parity; package generate_test).

NO engine/executor change (P1.M2 scope, landed); NO stub/stubtest change (S1 scope, landed);
NO config change (P1.M1 scope, landed — tests only SET the landed knobs); NO docs (item contract);
NO new deps; NO new files (idle_parity_test.go is S2's; if S2's file has a different name, append to it).

CONSUMERS: none downstream — P1.M3.T1.S4 (config observability) is independent of these tests.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...                 # tests compile alongside the tree (vet via make lint / go vet)
go vet ./internal/generate/...
gofmt -l internal/generate/generate_deadlines_test.go internal/generate/invariants_test.go internal/generate/idle_parity_test.go
# Expected: clean / empty. If gofmt lists a file: gofmt -w it.

# Scope guard: ONLY the 3 test files changed.
git status --short
# Expected: M internal/generate/generate_deadlines_test.go
#           M internal/generate/invariants_test.go
#           M internal/generate/idle_parity_test.go
# (plus whatever plan/ files the workflow itself tracks — but NO production file.)
```

### Level 2: Unit Tests (Component Validation) — THE PRIMARY GATE

```bash
# The three new tests, focused (plus S2's neighbors to confirm they still pass):
go test ./internal/generate/ -count=1 -v \
  -run 'TestCommitStaged_ProductiveSlowEmitterSurvives|TestInvariants$|TestHardCapHeartbeat_ExitCodeParity|TestIdleKill_ExitCodeParity'
# Expected: PASS. Recipe (b) ~6-7s; recipe (c) tests ~2s each; TestInvariants runs every row.

# Re-run for timing confidence (deadline tests are the flake-prone kind — two clean runs before trusting):
go test ./internal/generate/ -count=2 -run 'ProductiveSlow|HardCapHeartbeat'

# The full generate package (S2's tests + every existing deadline test untouched):
go test ./internal/generate/ -count=1
# Expected: green.

# Full suite + race + lint.
make test
make lint
# Expected: green / zero errors.
```

### Level 3: Integration Testing (System Validation)

```bash
# There is no separate integration surface — the tests ARE the integration (real CommitStaged → Render →
# provider.Execute → process group → stub binary → commit-tree/update-ref or rescue).

# Optional manual persona sanity (watch the byte patterns by hand):
go build -o /tmp/stubagent ./cmd/stubagent
STAGECOACH_STUB_EMIT_MS=500 STAGECOACH_STUB_EMIT_LINE="$'\n'" STAGECOACH_STUB_EMIT_TOTAL_MS=3000 \
  STAGECOACH_STUB_OUT="feat: progressive answer" /tmp/stubagent <<< "payload" | cat -A | tail -3
# expect: bare newline ticks then the answer line (~3s total).
STAGECOACH_STUB_HEARTBEAT_MS=100 timeout 1 /tmp/stubagent <<< "payload" > /tmp/out.txt 2> /tmp/err.txt
# expect: /tmp/out.txt EMPTY; /tmp/err.txt ~9 "tick" lines.

# Platform note: run the suite on Linux for the full discrimination (CI's Linux job carries it; on a
# darwin dev box the (b) test passes as the documented D4-degraded success smoke):
go env GOOS
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: the three tests exist.
grep -c 'func TestCommitStaged_ProductiveSlowEmitterSurvives' internal/generate/generate_deadlines_test.go   # 1
grep -c '"HardCapHeartbeat"' internal/generate/invariants_test.go                                            # 1
grep -c 'func TestHardCapHeartbeat_ExitCodeParity' internal/generate/idle_parity_test.go                     # 1

# Guard 2: the hard-cap cause string is asserted (never Cause.Error() "hard").
grep -c '"DEBUG: cause: hard cap"' internal/generate/invariants_test.go internal/generate/idle_parity_test.go # ≥1 each

# Guard 3: the (b) test asserts the NO-kill state (the false-kill proof's inverse half).
grep -c '"DEBUG: cause:"' internal/generate/generate_deadlines_test.go                                        # ≥1 (the absence assert)

# Guard 4: the (c) configs keep idle < hard (the discriminator ordering) — eyeball, then:
grep -n 'IdleTimeout = 1 \* time.Second' internal/generate/invariants_test.go internal/generate/idle_parity_test.go
grep -n 'Timeout = 1500 \* time.Millisecond' internal/generate/invariants_test.go internal/generate/idle_parity_test.go

# Guard 5: D4 note present in the (b) test (the item's documentation duty).
grep -c 'D4' internal/generate/generate_deadlines_test.go                                                     # ≥1

# Guard 6: -short guards on all three.
grep -c 'testing.Short()' internal/generate/generate_deadlines_test.go internal/generate/invariants_test.go internal/generate/idle_parity_test.go
# Expected: ≥1 per file beyond any existing occurrences (invariants already has some).

# Guard 7: scope — no production file touched.
git status --porcelain | grep -v '^?? plan/' | grep -v '_test.go'
# Expected: EMPTY.
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1-4 all pass; `go vet ./internal/generate/...` clean; `gofmt -l` empty on the 3 files
- [ ] `go test ./internal/generate/ -count=1` green (new + ALL existing, incl. S2's)
- [ ] `make test` (race) + `make lint` green

### Feature Validation
- [ ] **FR-A1**: progressive emitter (500ms ticks, 6s, then answer) under idle 2s / hard 30s ⇒ `err == nil`,
      commit created (HEAD == Result.CommitSHA), subject == "feat: progressive answer", one Execute, no cause
      line, elapsed ≥ 6s
- [ ] **FR-A2**: heartbeat-forever stub under idle 1s / hard 1.5s ⇒ `*RescueError{ErrTimeout}` with TreeSHA,
      FormatRescue "Tree ID:" line, verbose cause "hard cap", one Execute, `exitcode.For == 124`, HEAD
      unchanged, I1/I2/I3 invariants hold
- [ ] The D4 platform note is present in the test comments (Linux-armed discrimination vs non-Linux
      green-vacuous degradation — never fabricated)
- [ ] Both new timing tests re-ran clean (`-count=2`) — no flakes from CI scheduler variance

### Code Quality & Scope Validation
- [ ] `git status` == the 3 test files only; zero production/engine/stub/config changes
- [ ] Appended, never rewrote: S2's tests, S1's personas, the existing Timeout/IdleKill rows untouched
- [ ] Stdlib + existing internal imports only; no new deps; no new files
- [ ] Comments cite FR-A1/FR-A2/§9.30/D4 and the persona/knob names (the package's comment style)

---

## Anti-Patterns to Avoid

- ❌ Don't set idle ≥ hard in recipe (c). The proof IS the ordering: idle (1s) < hard (1.5s) means a "hard cap"
  cause line certifies the heartbeat kept resetting the idle clock while the cap still bounded the run.
  idle > hard passes trivially and proves nothing about activity.
- ❌ Don't assert `re.Cause.Error()` contains "hard" for a hard-cap kill. The Cause IS bare
  `context.DeadlineExceeded` ("context deadline exceeded" — names nothing); the naming exists ONLY in the
  verbose `DEBUG: cause: hard cap (…)` line. (Contrast: an IDLE kill's Cause is ErrIdleDeadline, which names
  "idle" — S2's territory.)
- ❌ Don't use the default "." EmitLine in recipe (b) if you want clean subject assertions. Ticks and the
  answer share one stdout buffer; `EmitLine: "\n"` lets ParseOutput's TrimSpace strip the ticks so
  Message/Subject equal the Out value exactly. With "." you may only assert Contains.
- ❌ Don't try to arm the idle watcher on non-Linux (no ProgressiveStdout injection — no manifest field sets
  it until P2, and CommitStaged renders internally). D4's degradation is by design ("stagecoach never
  fabricates activity"); the tests are green and correct everywhere, with the discrimination Linux-carried.
  State it in the comments, don't fight it.
- ❌ Don't rely on config defaults. Always set BOTH `cfg.IdleTimeout` and `cfg.Timeout` explicitly (defaults
  60s/600s would make (b) a 60s-wait and (c) a 600s hang on an engine regression). The LARGE/SMALL choices
  are the tests' discriminators.
- ❌ Don't skip the verbose buffer. Without `deps.Verbose = ui.NewVerbose(&buf, true)` every cause-line and
  Execute-count assertion silently no-ops (nil-safe methods) — the test would pass while proving nothing.
- ❌ Don't put the exit-124 assertion in package generate. internal/exitcode imports internal/generate —
  an in-package test importing exitcode is an import cycle. The parity test lives in S2's external
  `package generate_test` file, appended.
- ❌ Don't touch the engine, stub, or harness. If any assertion fails, the defect is in the LANDED engine
  (P1.M2) or harness (S1/S2) — surface it in the report; patching around it from a tests-only task is how
  parity silently rots.
- ❌ Don't write exact tick/elapsed upper bounds in recipe (b). Scheduler variance makes tight ceilings flaky:
  assert the elapsed FLOOR (≥ 6s — survival is the point) and leave the ceiling to the 30s hard cap + suite
  guard. (Recipe (c) keeps a generous ≤10s hang-guard — 5× the expected kill.)
- ❌ Don't skip the `-short` guards or let the tests exceed ~10s each — the suite's bounded-CI discipline
  (ci.yml keeps matrix jobs time-bounded) depends on every slow test opting out under -short.
- ❌ Don't restructure the TestInvariants table or S2's rows/columns. Append the row; the harness
  (treeSHAFromErr + assertInvariants) does the invariant work — a "smarter" row shape breaks every other row.

---

## Confidence Score: 9/10

Both recipes were pre-specified (call_sites_and_error_contract.md §9.6 (b)/(c) — knob names, cadence guidance,
the partial-capture contract), the personas are LANDED (S1, commit ed94f2b — field names verified against
stubtest.go), the engine is LANDED with its exact semantics verified at source (Deadlines struct, the D4 arming
branch at executor.go:144, both VerboseCause strings, the bare-DeadlineExceeded-on-hard-cap contract), the S2
parity assertion set this task mirrors is quoted in S2's PRP with file/package layout and the import-cycle
rationale, and the two non-obvious traps (D4 platform arming; ticks+answer sharing one buffer) are fenced with
concrete mitigations (comment duty; EmitLine "\n"). The -1: timing tests on shared CI runners can flake if the
host stalls >2s between watcher polls in recipe (b) (4× margin chosen, but a pathological runner can exceed
it — a re-run failure mode, not a wrong-test mode), and the append target idle_parity_test.go depends on S2's
in-flight landing (name verified from S2's PRP; a fallback locator is specified if it differs).
