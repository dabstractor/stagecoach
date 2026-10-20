# P1.M3.T1.S2 Research Findings — idle-kill end-to-end parity (FR-A5): exit-124, rescue, no-retry, cause line, invariants

> The parity proof: a hung-stub idle kill through the REAL pipeline must be downstream-IDENTICAL to the old
> hard-timeout path. Consumes persona (a) hung stub (SleepMS — EXISTS today) + the landed Deadlines engine
> (P1.M2.T1.S2/S3 COMPLETE). Tests-only; no production change; no docs.

---

## §1. THE EXIT-124 CHAIN (every hop verified in code)

1. **Stub (persona a — hung)**: `stubtest.Manifest(bin, stubtest.Options{Out: "feat: slow", SleepMS: 2000})` —
   drains stdin, writes marker, then sleeps IN SILENCE (no stdout/stderr bytes → flat activity).
2. **Config**: `config.Defaults()` seeds `IdleTimeout: 60s` + `Timeout: 600s` (config.go:230,86-87).
   `ResolveRoleDeadlines("message", cfg)` (roles.go:78) → (idle, hard); RoleConfig.IdleTimeout (config.go:43)
   overrides per-role; 0 ⇒ inherit global. The LARGE-hard/SMALL-idle idiom (from TestRun_MessageRoleTimeoutBoundsTurn):
   `cfg.IdleTimeout = 150ms` + `cfg.Timeout = 30s` ⇒ only the idle deadline can fire (30s ≫ 2000ms stub sleep) —
   the test's built-in DISCRIMINATOR: if the kill happens at all, it was the idle deadline, not the cap.
3. **Execute** (executor.go:104): `provider.Execute(ctx, *spec, provider.Deadlines{Idle: msgIdle, Hard: msgHard},
   deps.Verbose)` (generate.go:375). The idle watcher (D4 arming: idle set + activity channel — always armed when
   Idle > 0) polls at **~500ms granularity**; first tick initializes the baseline, a later tick with no byte delta
   and elapsed > Idle kills the process group. Returns **`ErrIdleDeadline`** (executor.go:51) —
   `Error() == "context deadline exceeded (idle: no observable agent activity)"` (**contains "idle"**) and its
   `Is()` **satisfies `errors.Is(err, context.DeadlineExceeded)`** (D2/FR-A5 — the parity linchpin).
   ALSO emits (executor.go:152, BEFORE returning): `vb.VerboseCause(fmt.Sprintf("idle deadline (%v without
   observable activity)", dl.Idle))` → ui prints **"DEBUG: cause: idle deadline (150ms without observable activity)"**
   (verbose.go:137-141; format "DEBUG: cause: <reason>\n"; nil/off no-op). verbose_test.go:110 pins the format.
4. **generate.go:376-382** (CommitStaged one-shot loop): `if errors.Is(execErr, context.DeadlineExceeded)` — TRUE for
   ErrIdleDeadline via its Is() → comment "§5: immediate rescue, NO retry — agent was killed." → returns
   `&RescueError{Kind: ErrTimeout, TreeSHA: treeSHA, ParentSHA: parentSHA, Candidate: candidate, Cause: execErr}`.
   **NO retry**: the DeadlineExceeded branch RETURNS from inside the `for attempt := 0; attempt <=
   cfg.MaxDuplicateRetries` loop on the first hit (attempts only consumed by parse-fail/duplicate).
5. **exitcode.For(err)** (exitcode.go:63-96): `errors.Is(err, generate.ErrTimeout)` → **124** (checked BEFORE the
   generic rescue→3; doc: "A *generate.RescueError whose Kind is ErrTimeout maps to 124").
6. **handleGenError** (default_action.go:354, `func handleGenError(stderr io.Writer, err error, verboseOn bool) error`):
   `errors.As(err, &re)` → prints `generate.FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate)` — line 4 of
   rescue.go is **"Tree ID: <treeSHA>"** — then under verbose `fmt.Fprintln(stderr, "DEBUG: cause: "+re.Cause.Error())`
   (default_action.go:381) → "DEBUG: cause: context deadline exceeded (idle: no observable agent activity)".
   Returns `exitcode.New(code, nil)` — `*ExitError{Code: 124}` (exitcode.go:19: `func New(code int, err error) *ExitError`).

**FR-A5 "the ONLY observable difference"**: per verbose.go:132-137's doc, the DEBUG: cause line is the ONLY
observable difference between an idle kill and a hard-cap kill — both surface as DeadlineExceeded downstream. The
tests assert BOTH: (a) the deadline-chain parity (ErrTimeout/124/rescue/no-retry/invariants) AND (b) the cause line
naming "idle" (proving WHICH deadline fired).

## §2. THE EXISTING MODELS TO MIRROR (all read verbatim)

- **invariants_test.go "Timeout" row (L219-235)** — THE template for the parity row: `stubtest.Manifest(bin,
  Options{Out: "feat: slow", SleepMS: 2000})` + `cfg.Timeout = 150ms` → `CommitStaged(...)` → asserts
  `errors.Is(err, ErrTimeout)`; returns `err, ""` → the TestInvariants harness funnels through
  **assertInvariants** (L97-130): **I1** idempotent index (`git diff --cached --name-only` AND full diff
  byte-identical), **I2** atomic HEAD (rev-parse unchanged), **I3** snapshot immutability (`cat-file -t` = tree;
  `cat-file -p <treeSHA>` stable after staging NEW content). `treeSHAFromErr` extracts TreeSHA. `-short` skip guard.
- **TestRun_MessageRoleTimeoutBoundsTurn (multiturn_test.go:246-274)** — the LARGE-global/SMALL-role config idiom.
- **TestCommitStaged_WorkDescription_MessageRoleTimeout (generate_workdesc_test.go:288-318)** — workdesc parity:
  `cfg.WorkDescription` + `SessionMode:&"append"` → `*RescueError` + ErrTimeout (generate.go:341-342 maps
  wdCause DeadlineExceeded → Kind: ErrTimeout).
- **TestCallPlanner_Timeout (planner_test.go:348-383)** — the planner model: `cfg.Timeout = 30s` (LARGE) +
  `Roles: {"planner": {Timeout: 100ms}}` (SMALL) + `Options{Out: validMultiJSON, SleepMS: 2000}` → `callPlanner(...)`
  → asserts `errors.Is(err, ErrPlannerFailed)` + `errors.Is(err, context.DeadlineExceeded)` (the traversal).
  Uses `plannerDeps(t, repo, m)` (sets **Verbose: nil** — tests must override `deps.Verbose = ui.NewVerbose(&buf, true)`)
  + `freezeForPlanner(t, repo, false)`.
- **TestDecompose_MessageRescuePartial (decompose_test.go:1770-1850)** — the full-Decompose model: helpers
  `dcmInitRepo/dcmWriteFile/dcmPlannerManifest/dcmOutBuffer/tooledStubManifest/dcmStagerSeam`;
  `RoleManifests{Planner:…, Stager:…, StagerAvailable:true, Message:…, Arbiter:…}`; asserts errors.As →
  `*DecomposeRescueError`.
- **TestDecomposeRescueError_ExitCode (decompose_test.go:2063)** — the Unwrap-chain idiom: DecomposeRescueError →
  *RescueError → Kind — `errors.Is(dre, generate.ErrTimeout)` traverses Unwrap.
- **External test package EXISTS in internal/generate**: hooks_freeze_test.go / repro_freeze_test.go are
  `package generate_test` (own initTempRepo — package generate's helpers are unimportable). This is how a generate
  test can import **internal/exitcode** WITHOUT a cycle (exitcode imports generate; package generate's INTERNAL
  tests CANNOT import exitcode — "import cycle not allowed in test"). decompose tests CAN import exitcode
  (exitcode does not import decompose).
- **Verbose capture idiom**: `ui.NewVerbose(&buf, true)` (generate_multiturn_test.go:87, generate_test.go:710).
  `verboseCommandPrefix = "DEBUG: command: "` (multiturn_test.go:24) — counting occurrences counts Execute calls
  → THE no-retry proof: `strings.Count(buf.String(), "DEBUG: command: ") == 1`.

## §3. THE ASYMMETRIES TO PRESERVE (contract point 1; context, mostly covered by existing tests)

- **one-shot (CommitStaged)** → `*RescueError{Kind: ErrTimeout}` → exit 124 (generate.go:376-382). THIS TASK proves
  the idle variant.
- **workdesc** → same ErrTimeout/124 (generate.go:335-342). THIS TASK adds the idle variant (cheap mirror of the
  existing workdesc timeout test).
- **multiturn turn-deadline** → ErrRescue exit 3 (cause aborts Run → the FR-T1 gate's fall-through; NOT this task's
  scope — existing multiturn tests cover the hard variant; noted so the implementer doesn't "fix" the asymmetry).
- **decompose planner** → `fmt.Errorf("%w: %w", ErrPlannerFailed, execErr)` — **NON-rescue, no DecomposeRescueError**
  (planner.go:130-132: "DeadlineExceeded || Canceled → non-rescue; no retry"; decompose.go:219 returns it AS-IS).
  The planner's "traversal" = the ErrPlannerFailed+DeadlineExceeded %w chain (TestCallPlanner_Timeout's assertions).
- **decompose message[i]** → `*RescueError{Kind: ErrTimeout}` (message.go:175-181, TreeSHA: treeB) → runLoop wraps
  **DecomposeRescueError** (decompose.go:77-100; `Unwrap()` → e.Rescue so errors.Is → generate.ErrTimeout → 124
  traverses). THIS TASK proves the idle variant through full Decompose.
- **hook/exec.go:190** → `errors.New("stagecoach: hook generation timed out")` — no rescue object, never-block
  (NOT required by the contract's LOGIC; optional mirror).

## §4. TIMING FACTS (why the numbers work)

- Idle watcher: ~500ms poll; first tick initializes baseline → an Idle of 100-150ms kills at the FIRST detection
  tick, i.e. ~500-1000ms after Execute starts. Stub SleepMS 2000ms ⇒ killed ~1s in, ~1s before it would finish.
  Test wall-time ≈ 1-2s each; `-short` skip guards (mirror the Timeout row) keep `-short` suites fast.
- S1's knob cadence guidance (intervals ≪ 500ms) applies to personas (b)/(c) — NOT this task (persona (a) needs
  only SleepMS, which exists today; NO dependency on S1's un-landed knobs!). **This task can land BEFORE S1.**

## §5. TEST DESIGN (4 required + 1 optional; ~5-8s added suite time, all -short-skipped)

- **A. invariants_test.go — new TestInvariants table row `"IdleKill"`** (package generate): mirrors the Timeout row;
  stub `Out:"feat: slow", SleepMS:2000`; `cfg := config.Defaults(); cfg.Timeout = 30s; cfg.IdleTimeout = 150ms`;
  Deps gains `Verbose: ui.NewVerbose(&buf, true)`; inside run: assert `errors.Is(err, ErrTimeout)`,
  `strings.Contains(buf, "DEBUG: cause: idle deadline")` (the cause line names idle),
  `strings.Count(buf.String(), "DEBUG: command: ") == 1` (NO retry); return `err, ""` → harness funnels through
  assertInvariants (I1/I2/I3 — the no lock/index/HEAD mutation mirror of the Timeout row).
- **B. NEW internal/generate/idle_parity_test.go (package generate_test — EXTERNAL, mirrors hooks_freeze_test.go's
  own initTempRepo)** — `TestIdleKill_ExitCodeParity`: full CommitStaged, hung stub, `IdleTimeout:150ms/Timeout:30s`;
  asserts `errors.As(&re)` + `errors.Is(err, generate.ErrTimeout)`; `re.TreeSHA != ""` and
  `strings.Contains(generate.FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate), "Tree ID: "+re.TreeSHA)`;
  `strings.Contains(re.Cause.Error(), "idle")`; **`exitcode.For(err) == exitcode.Timeout` (124)**; HEAD unchanged;
  elapsed ≤ 10s hang-guard. (+ `TestCommitStaged_WorkDescription_IdleKill` — the workdesc idle mirror:
  `cfg.WorkDescription="add x"` + `SessionMode:&"append"` → same ErrTimeout/124/cause assertions.)
- **C. planner_test.go — `TestCallPlanner_IdleDeadline`** (package decompose): mirror TestCallPlanner_Timeout with
  `Roles: {"planner": {IdleTimeout: 100ms}}` + `cfg.Timeout = 30s`; override `deps.Verbose = ui.NewVerbose(&buf, true)`;
  assert `errors.Is(err, ErrPlannerFailed)` + `errors.Is(err, context.DeadlineExceeded)` (traversal UNCHANGED) +
  `strings.Contains(buf, "DEBUG: cause: idle deadline")` + Execute-count == 1.
- **D. decompose_test.go — `TestDecompose_MessageIdleKillParity`** (package decompose): mirror
  TestDecompose_MessageRescuePartial with messageM = `stubtest.Manifest(bin, Options{Out: "feat: hang", SleepMS: 5000})`
  (plain Manifest — concept 0's message call hangs, no Script needed) + `cfg.Roles = {"message": {IdleTimeout: 100ms}}`,
  `cfg.Timeout = 30s`, `cfg.MaxDuplicateRetries = 0`; keep the seam/stager/arbiter deps shape verbatim; set
  `deps.Verbose = ui.NewVerbose(&vbuf, true)`; assert `errors.As(&dre)` (*DecomposeRescueError),
  `errors.Is(err, generate.ErrTimeout)` (the traversal), **`exitcode.For(err) == 124`** (decompose CAN import exitcode),
  `strings.Contains(vbuf, "DEBUG: cause: idle deadline")`, HEAD unchanged + no commits landed (commitCount unchanged).
- **(optional) hook/exec_test.go — `TestRun_IdleTimeoutNeverBlock`**: mirror TestRun_TimeoutNeverBlock with
  `IdleTimeout:50ms/Timeout:30s` → err contains "timed out", msg-file byte-identical. Cheap; include if time permits.

## §6. SCOPE FENCES

- NO production change: executor/generate/decompose/planner/hook/exitcode/cmd are READ-ONLY (the engine LANDED in
  P1.M2). If a parity test FAILS, the bug is in the ENGINE (P1.M2.T1.S2/S3) — surface it, don't patch around it here.
- NO S1 dependency: persona (a) (SleepMS) exists TODAY; S1's knobs are for S3 (false-kill regression), not S2.
- NO e2e/internal-e2e change (the contract's surface is the library: CommitStaged + decompose); no docs ("tests only").
- Consumed by P1.M3.T1.S4 (config observability — same harness; B's idle_parity_test.go is the natural extension home).

## §7. VALIDATION

- `go test ./internal/generate/ -run 'IdleKill|Idle' -v` + `go test ./internal/decompose/ -run 'Idle' -v`
- Full: `go test ./internal/generate/ ./internal/decompose/ -count=1` (regression: existing Timeout rows stay green —
  they still use cfg.Timeout/hard-cap, untouched); `make test`; `make lint`; `gofmt -l` empty.
- Grep guards: the 4-5 test names exist; `exitcode.For` appears in the external generate test + decompose test;
  "DEBUG: cause: idle" asserted in each; no engine file in `git status`.
