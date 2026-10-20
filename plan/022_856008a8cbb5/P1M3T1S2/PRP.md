name: "P1.M3.T1.S2 — idle-kill end-to-end parity: exit-124 semantics, rescue, no-retry, cause line, no lock/index/HEAD mutation (FR-A5 proof, tests-only)"
description: >
  TESTS-ONLY task (no production change, no docs): prove FR-A5 — an idle-deadline kill is downstream-IDENTICAL to
  the old hard-timeout path — through the REAL pipeline, using the EXISTING hung-stub persona (stubtest.Options{
  SleepMS: N} — zero stdout/stderr bytes after the marker; persona (a) exists today; NO dependency on S1's un-landed
  emit/heartbeat knobs, so this task can land independently). FOUR required tests + one optional, mirroring
  established models verbatim: (A) a new "IdleKill" table row in internal/generate/invariants_test.go's TestInvariants —
  mirror of the "Timeout" row (L219-235): stub Out:"feat: slow"/SleepMS:2000, cfg.Timeout=30s (LARGE hard cap — the
  built-in discriminator: only the idle deadline can fire) + cfg.IdleTimeout=150ms (SMALL), Deps.Verbose=ui.NewVerbose
  (&buf,true); inside the row assert errors.Is(err, ErrTimeout) + the verbose buffer contains "DEBUG: cause: idle
  deadline" (the executor's VerboseCause line, executor.go:152 — FR-A5's ONLY observable difference) + the Execute-call
  count == 1 (strings.Count(buf,"DEBUG: command: ")==1 — the NO-retry proof); the row returns err → the harness funnels
  through assertInvariants (I1 idempotent index / I2 atomic HEAD / I3 snapshot immutability — the contract's "no
  lock/index/HEAD mutation" mirror of the Timeout row). (B) NEW FILE internal/generate/idle_parity_test.go as
  package generate_test (EXTERNAL — mirrors hooks_freeze_test.go's own-initTempRepo pattern; REQUIRED because
  internal/exitcode imports internal/generate, so package-generate tests CANNOT import exitcode — "import cycle not
  allowed in test" — while the external package can): TestIdleKill_ExitCodeParity drives the full generate.CommitStaged
  with the hung stub + IdleTimeout:150ms/Timeout:30s, asserting errors.As→*RescueError, errors.Is(generate.ErrTimeout),
  re.TreeSHA != "" + FormatRescue contains "Tree ID: "+re.TreeSHA (the rescue message with tree SHA), re.Cause.Error()
  contains "idle" (ErrIdleDeadline.Error() == "context deadline exceeded (idle: no observable agent activity)"),
  exitcode.For(err) == exitcode.Timeout (124 — THE exit-code semantics), HEAD unchanged, elapsed ≤10s hang-guard,
  + the verbose cause line; PLUS TestCommitStaged_WorkDescription_IdleKill (the workdesc asymmetry: cfg.WorkDescription
  + SessionMode:&"append" → same ErrTimeout/124 assertions, mirroring TestCommitStaged_WorkDescription_MessageRoleTimeout
  with idle swapped for hard). (C) internal/decompose/planner_test.go — TestCallPlanner_IdleDeadline: mirror
  TestCallPlanner_Timeout (L348) with Roles:{"planner":{IdleTimeout:100ms}} + cfg.Timeout=30s; override
  plannerDeps' Verbose:nil with a buffer; assert errors.Is(err, ErrPlannerFailed) + errors.Is(err,
  context.DeadlineExceeded) (the planner traversal UNCHANGED — planner.go:130-132 maps DeadlineExceeded→ErrPlannerFailed
  non-rescue, NO DecomposeRescueError) + the idle cause line + Execute-count==1. (D) internal/decompose/decompose_test.go —
  TestDecompose_MessageIdleKillParity: mirror TestDecompose_MessageRescuePartial (L1770-1850, reusing its dcmInitRepo/
  dcmWriteFile/dcmPlannerManifest/dcmOutBuffer/tooledStubManifest/dcmStagerSeam helpers verbatim) with messageM =
  stubtest.Manifest(bin, Options{Out:"feat: hang", SleepMS:5000}) (plain Manifest — concept 0's message call hangs;
  no Script) + cfg.Roles={"message":{IdleTimeout:100ms}}/Timeout:30s/MaxDuplicateRetries:0; set deps.Verbose; assert
  errors.As→*DecomposeRescueError, errors.Is(err, generate.ErrTimeout) (the DecomposeRescueError.Unwrap traversal),
  exitcode.For(err)==124 (decompose tests CAN import exitcode — no cycle), the idle cause line, HEAD unchanged +
  commit count unchanged (nothing landed). (Optional) hook/exec_test.go TestRun_IdleTimeoutNeverBlock mirroring
  TestRun_TimeoutNeverBlock with IdleTimeout swapped in. The chain being proven (every hop code-verified): hung stub →
  idle watcher (~500ms poll; kill ~0.5-1s in) → Execute returns ErrIdleDeadline (executor.go:51; Error() contains
  "idle"; Is() satisfies context.DeadlineExceeded — the D2/FR-A5 linchpin) + VerboseCause("idle deadline (…)") →
  generate.go:376-382 "§5: immediate rescue, NO retry" → &RescueError{Kind:ErrTimeout, TreeSHA, Cause} →
  exitcode.For → 124 → handleGenError prints FormatRescue ("Tree ID: <sha>") + "DEBUG: cause: …(idle:…)". Existing
  asymmetries PRESERVED (not "fixed"): one-shot/workdesc→124, multiturn turn-deadline→3 (covered by existing tests,
  out of scope), planner→ErrPlannerFailed non-rescue, message[i]→DecomposeRescueError→124. All timing tests carry
  -short skip guards (mirroring the Timeout row); ~5-8s added suite time. If ANY parity assertion fails, the defect is
  in the LANDED ENGINE (P1.M2.T1.S2/S3) — surface it, do not patch around it here. Consumed by P1.M3.T1.S4 (same harness;
  idle_parity_test.go is its natural extension home).

---

## Goal

**Feature Goal**: Prove FR-A5 through the real pipeline: when the idle deadline kills a hung agent, EVERY downstream
behavior is byte-identical to the old hard-timeout path — same `*RescueError{Kind: ErrTimeout}` with the tree SHA,
same exit-124 mapping, same immediate-rescue-NO-retry, same FormatRescue "Tree ID:" message — with the "DEBUG: cause:"
line naming "idle" as the ONLY observable difference (verbose.go:132-137's documented contract). Plus the decompose
sides: the planner's ErrPlannerFailed traversal and the message-role DecomposeRescueError→124 traversal unchanged
under an idle kill. And the §20.2 no-mutation invariants (I1/I2/I3) hold on the deadline path.

**Deliverable** (4 required tests + 1 optional across 3 files modified + 1 new file; ZERO production change):
1. **internal/generate/invariants_test.go** — new TestInvariants table row `"IdleKill"` (+ the `ui` import).
2. **internal/generate/idle_parity_test.go** — NEW FILE, `package generate_test` (external), with its own minimal
   repo helpers (mirroring hooks_freeze_test.go) + 2 tests (exit-124 parity + workdesc idle).
3. **internal/decompose/planner_test.go** — `TestCallPlanner_IdleDeadline` (+ `ui` import).
4. **internal/decompose/decompose_test.go** — `TestDecompose_MessageIdleKillParity` (+ `exitcode` + `ui` imports).
5. (optional) **internal/hook/exec_test.go** — `TestRun_IdleTimeoutNeverBlock`.

**Success Definition**:
- A hung stub (SleepMS, zero bytes) + `cfg.IdleTimeout` ≪ stub sleep ≪ `cfg.Timeout` ⇒ CommitStaged returns
  `*RescueError` with `errors.Is(err, generate.ErrTimeout)`, non-empty `TreeSHA`, a `Cause` whose Error() contains
  "idle", and `exitcode.For(err) == 124`.
- The rescue message proof: `generate.FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate)` contains
  `"Tree ID: " + re.TreeSHA`.
- NO-retry proof: the verbose buffer records exactly ONE Execute invocation (`strings.Count(buf, "DEBUG: command: ") == 1`).
- Cause line: the verbose buffer contains `"DEBUG: cause: idle deadline"` (the executor's VerboseCause) in every test.
- Invariants: the IdleKill row passes assertInvariants (I1 idempotent index, I2 atomic HEAD, I3 snapshot immutability).
- Decompose planner: idle kill ⇒ `errors.Is(err, ErrPlannerFailed)` AND `errors.Is(err, context.DeadlineExceeded)`
  (traversal unchanged), with the idle cause line.
- Decompose message: idle kill ⇒ `*DecomposeRescueError` wrapping ErrTimeout, `exitcode.For(err) == 124`, HEAD
  unchanged, nothing landed.
- All existing tests stay green (the engine is LANDED — this task only observes it); `make test` + `make lint` green;
  `gofmt -l` empty; `git status` shows ONLY test files.

## User Persona (if applicable)

**Target User**: The maintainer verifying the v3.5 dual-deadline engine (P1.M2, LANDED) actually preserved the v1-3.4
timeout contract end-to-end — and the P1.M3.T1.S4 author (config observability) who reuses this harness.

**Use Case**: A user sets `--idle-timeout 10s`; their agent hangs silently (network stall); stagecoach kills it at
~10s of silence. The user sees EXACTLY what a v3.4 timeout showed: the §18.3 rescue message with their tree SHA, exit
124, their staged files intact — plus (under --verbose) one new line explaining WHY ("idle: no observable agent
activity"). This test suite is the proof that migration to idle deadlines changed nothing user-visible except that line.

**User Journey**: hung agent → idle watcher kills the process group → Execute returns ErrIdleDeadline (+ VerboseCause
"DEBUG: cause: idle deadline (…)") → CommitStaged's `errors.Is(execErr, context.DeadlineExceeded)` branch (TRUE via
ErrIdleDeadline.Is()) → immediate rescue, NO retry → `*RescueError{Kind: ErrTimeout, TreeSHA, Cause}` → exitcode.For
→ 124 → handleGenError prints FormatRescue ("Tree ID: <sha>") + verbose "DEBUG: cause: …(idle:…)".

**Pain Points Addressed**: §9.5's "NOT tested today" list — the engine landed (P1.M2.T1.S2/S3) with unit tests at the
executor seam, but NOTHING drives an idle kill through the real CommitStaged/decompose pipelines to prove the FR-A5
parity chain (exit code, rescue shape, no-retry, invariants). A silent parity break (e.g. an idle sentinel that does
NOT satisfy DeadlineExceeded, or a rescue missing the tree SHA) would ship undetected.

## Why

- **FR-A5 (§9.30)**: "an idle-deadline kill is downstream-identical to a timeout today (exit 124, rescue, no-retry)" —
  the spec-level guarantee this suite proves. The linchpin is ErrIdleDeadline's `Is()` satisfying
  `context.DeadlineExceeded` (executor.go — D2), which routes the kill into generate.go:376's no-retry branch.
- **The chain is only as strong as its weakest hop**: exit-124 (exitcode.For), the rescue shape (RescueError fields →
  FormatRescue), no-retry (the loop-return), the invariants (I1/I2/I3), the decompose traversals (ErrPlannerFailed %w
  chain; DecomposeRescueError.Unwrap) — each hop is code-verified, but no test exercises them TOGETHER under an idle
  kill. This suite is that integration proof.
- **The cause line is the ONLY observable difference** (verbose.go:132-137 doc): asserting it in every test both
  proves WHICH deadline fired (the LARGE-hard/SMALL-idle config makes the test self-discriminating) and pins the
  FR-A5 contract.
- **Cheap and hermetic**: the stub agent IS the mock (SleepMS — zero new infrastructure); ~5-8s of suite time; all
  -short-skipped. No S1 dependency (persona (a) exists today) — this can land in parallel with S1.

## What

**User-visible behavior**: None (tests only).

**Technical change**: 4 required tests (+1 optional) mirroring existing models verbatim. See the Blueprint.

### Success Criteria
- [ ] invariants_test.go TestInvariants gains the `"IdleKill"` row: stub `Options{Out: "feat: slow", SleepMS: 2000}`;
      `cfg.Timeout = 30 * time.Second; cfg.IdleTimeout = 150 * time.Millisecond`; Deps.Verbose = ui.NewVerbose(&buf, true);
      row asserts `errors.Is(err, ErrTimeout)` + `strings.Contains(buf.String(), "DEBUG: cause: idle deadline")` +
      `strings.Count(buf.String(), "DEBUG: command: ") == 1`; returns `err, ""` (→ assertInvariants I1/I2/I3);
      `-short` skip guard.
- [ ] NEW internal/generate/idle_parity_test.go (`package generate_test`, own initTempRepo/git helpers mirroring
      hooks_freeze_test.go): `TestIdleKill_ExitCodeParity` asserts errors.As→*generate.RescueError, errors.Is(ErrTimeout),
      TreeSHA != "" AND FormatRescue contains "Tree ID: "+TreeSHA, re.Cause.Error() contains "idle",
      **exitcode.For(err) == exitcode.Timeout**, HEAD unchanged, elapsed ≤ 10s, verbose "DEBUG: cause: idle deadline",
      Execute-count == 1; `TestCommitStaged_WorkDescription_IdleKill` (cfg.WorkDescription + SessionMode:&"append") →
      same ErrTimeout + 124 assertions.
- [ ] planner_test.go `TestCallPlanner_IdleDeadline`: `cfg.Timeout = 30s` + `Roles: {"planner": {IdleTimeout: 100ms}}`;
      `deps.Verbose = ui.NewVerbose(&buf, true)` (plannerDeps sets nil); asserts errors.Is(ErrPlannerFailed) +
      errors.Is(context.DeadlineExceeded) + the idle cause line + Execute-count == 1.
- [ ] decompose_test.go `TestDecompose_MessageIdleKillParity`: mirrors TestDecompose_MessageRescuePartial's deps shape
      (dcm helpers + tooledStubManifest + dcmStagerSeam) with messageM = `stubtest.Manifest(bin, stubtest.Options{Out:
      "feat: hang", SleepMS: 5000})` + `cfg.Roles = {"message": {IdleTimeout: 100ms}}`, `cfg.Timeout = 30s`,
      `cfg.MaxDuplicateRetries = 0`; deps.Verbose buffer; asserts errors.As→*DecomposeRescueError +
      errors.Is(generate.ErrTimeout) + **exitcode.For(err) == 124** + the idle cause line + HEAD/commit-count unchanged.
- [ ] (optional) hook/exec_test.go `TestRun_IdleTimeoutNeverBlock`: mirror TestRun_TimeoutNeverBlock with
      `IdleTimeout: 50ms / Timeout: 30s` → error contains "timed out", msg-file byte-identical.
- [ ] ZERO production/engine change; `git status` shows ONLY the test files; `make test` + `make lint` green;
      `gofmt -l` empty; existing Timeout rows/tests stay green untouched.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the full verified exit-124 chain (stub → watcher → ErrIdleDeadline [Error() + Is() semantics] → VerboseCause
→ generate.go:376 no-retry branch → RescueError fields → exitcode.For → handleGenError's FormatRescue + DEBUG line),
the exact existing tests to mirror (with file:line anchors and their config idioms), the LARGE-hard/SMALL-idle
discriminator rationale, the import-cycle constraint that forces the external test package for the exitcode assertion
(+ the existing hooks_freeze_test.go pattern to follow), the planner's Verbose:nil override, the full-Decompose helpers
(dcm*/tooledStubManifest/dcmStagerSeam) and the plain-Manifest-hangs-on-concept-0 simplification, the ~500ms
watcher-poll timing facts (why 150ms idle + 2000ms sleep kills at ~1s), the no-retry counting technique
("DEBUG: command: " occurrences), the asymmetries to preserve (and that multiturn→3 is existing coverage, out of scope),
and the fence that a failing parity test indicates an ENGINE defect (P1.M2), not something to patch here.

### Documentation & References

```yaml
# MUST READ — the authoritative findings (the verified chain + the models + timing + the import-cycle constraint)
- docfile: plan/022_856008a8cbb5/P1M3T1S2/research/findings.md
  why: "§1 the exit-124 chain with every hop code-verified (incl. ErrIdleDeadline.Error() == 'context deadline exceeded
        (idle: no observable agent activity)' and its Is() satisfying DeadlineExceeded; executor.go:152's VerboseCause
        string 'idle deadline (%v without observable activity)'); §2 the existing models to mirror (invariants Timeout
        row L219-235; TestCallPlanner_Timeout L348; TestDecompose_MessageRescuePartial L1770-1850; the EXTERNAL
        package generate_test pattern from hooks_freeze_test.go — REQUIRED for the exitcode import); §3 the asymmetries
        (planner = ErrPlannerFailed non-rescue, NOT DecomposeRescueError; message[i] = DecomposeRescueError→124);
        §4 timing (500ms poll → kill at ~0.5-1s; why 150ms/2000ms/30s works; -short guards); §5 the test design;
        §6 scope fences (NO S1 dependency — SleepMS exists; engine is LANDED); §7 validation."
  critical: "§1/§2: the import cycle — internal/exitcode imports internal/generate, so package-generate test files
             CANNOT import exitcode; the exit-124 assertion MUST live in package generate_test (external) — the
             hooks_freeze_test.go precedent. §3: the planner does NOT produce DecomposeRescueError (ErrPlannerFailed,
             non-rescue) — the contract's 'DecomposeRescueError traversal' belongs to the MESSAGE role (test D).
             §4: LARGE-hard/SMALL-idle is the built-in discriminator (if the kill fires at all, it was idle).
             §6: if a parity assertion fails, the bug is in the LANDED engine — surface it, don't patch here."

# MUST READ — the call-sites/exit-124 map (§2-§4 + §9.3/§9.5/§9.6)
- docfile: plan/022_856008a8cbb5/architecture/call_sites_and_error_contract.md
  section: "§3 Exit-124 mapping (exitcode.For order, ErrTimeout, FormatRescue's 'Tree ID:' line, DecomposeRescueError
            traversal); §4 No-retry-on-timeout (the 4 sites + the fall-through contrast); §9.3 the invariants harness
            (assertInvariants I1/I2/I3, treeSHAFromErr); §9.5 timeout scenarios tested today; §9.6 stub persona (a)."
  why: "The authoritative error-contract map this suite proves. §3's exitcode.For order (ErrTimeout checked BEFORE
        rescue→3) is the 124 semantics; §4's generate.go:376 comment ('§5: immediate rescue, NO retry') is the no-retry
        invariant; §9.3 documents the TestInvariants table + assertInvariants the new row rides."
  critical: "§9.6 persona (a): SleepMS is the hung stub — 'a large SleepMS is the idiom'. This task needs NO new knobs."

# MUST EDIT — the invariants row (A)
- file: internal/generate/invariants_test.go   # the TestInvariants table (Timeout row at L219-235 is the neighbor)
  why: "The harness: rows are {name, run func(t, ctx, repo, bin) (error, string)}; the Timeout row is the verbatim
        template; assertInvariants (L97-130) + treeSHAFromErr do the I1/I2/I3 work after run returns."
  pattern: "Mirror the Timeout row's shape exactly (the -short skip guard, the errors.Is check inside run, `return err, \"\"`)."
  gotcha: "The row's Deps must add Verbose: ui.NewVerbose(&buf, true) (declare buf inside run) — the Timeout row has no
           Verbose; the ui import must be added. Count 'DEBUG: command: ' occurrences for the no-retry proof."

# MUST CREATE — the external parity file (B)
- file: internal/generate/idle_parity_test.go   # NEW; package generate_test (mirror hooks_freeze_test.go)
  why: "The ONLY place in internal/generate that can import internal/exitcode (external package breaks the cycle:
        exitcode imports generate; generate_test is a separate package). hooks_freeze_test.go:44 documents the same
        pattern for internal/hooks ('Own copy — the white-box generate_test.go initRepo is in package generate,
        unimportable here')."
  pattern: "Copy hooks_freeze_test.go's import block + initTempRepo (git init -q + user.name/email config + an initial
            commit) + write/stage helpers; then stubtest.Manifest + generate.CommitStaged(ctx, generate.Deps{Git:
            git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&buf, true)}, cfg)."
  gotcha: "package generate_test CANNOT see ErrTimeout/RescueError/FormatRescue unqualified — use generate.ErrTimeout,
           generate.RescueError, generate.FormatRescue. All exported ✓. exitcode.Timeout == 124 (exitcode.go:32)."

# MUST EDIT — the planner test (C)
- file: internal/decompose/planner_test.go   # TestCallPlanner_Timeout at L348 is the verbatim template
  why: "plannerDeps(t, repo, m) builds Deps{Git, Config, Roles, Verbose: nil} — the test must OVERRIDE Verbose with a
        buffer. freezeForPlanner(t, repo, false) gives (baseTree, tStart). The existing test's two errors.Is assertions
        ARE the traversal proof — keep them identical, add the idle-cause line + count."
  pattern: "cfg.Timeout = 30s (LARGE) + Roles{'planner': {IdleTimeout: 100ms}} (SMALL idle — the discriminator;
            SleepMS 2000 < 30s so the hard cap never fires; the planner's 60s global-idle default never fires either)."
  gotcha: "validMultiJSON is the existing planner-output fixture (defined in planner_test.go) — reuse it as Out."

# MUST EDIT — the full-Decompose test (D)
- file: internal/decompose/decompose_test.go   # TestDecompose_MessageRescuePartial (L1770-1850) is the template
  why: "The established full-Decompose harness: dcmInitRepo/dcmWriteFile/dcmPlannerManifest/dcmOutBuffer/
        tooledStubManifest/dcmStagerSeam + RoleManifests{...} + the (a) errors.As→*DecomposeRescueError assertion shape.
        TestDecomposeRescueError_ExitCode (L2063) is the Unwrap-chain idiom."
  pattern: "Copy MessageRescuePartial's deps block; replace messageM with the hung plain Manifest (Out:'feat: hang',
            SleepMS:5000 — concept 0's call hangs; NO Script needed since concept 0 dying is the scenario); set
            cfg.Roles={'message':{IdleTimeout:100ms}}, cfg.Timeout=30s, cfg.MaxDuplicateRetries=0; add
            deps.Verbose = ui.NewVerbose(&vbuf, true); add the exitcode import (NO cycle: exitcode does not import
            decompose)."
  gotcha: "MessageRescuePartial used a Script so concept 0 SUCCEEDS and concept 1 fails; the hung-Manifest variant
           kills concept 0 (dre.Index==0, NO commits published — assert HEAD/commit-count unchanged instead of
           partial-commit[0] assertions). Keep the stager seam (its plannerJSON shares a.txt across c1/c2 — non-disjoint
           ⇒ the tooled-stager path; do NOT switch to a disjoint JSON, that changes which path runs)."

# CONTEXT — the landed engine being observed (READ-ONLY; the engine under test)
- file: internal/provider/executor.go   # Deadlines (:41), ErrIdleDeadline (:46-56), Execute (:104), VerboseCause call (:152)
  why: "The kill semantics: ErrIdleDeadline.Error() contains 'idle'; its Is() satisfies context.DeadlineExceeded (the
        parity linchpin); executor.go:152 emits vb.VerboseCause('idle deadline (%v without observable activity)') BEFORE
        returning. The arming rule (D4): watcher arms iff Idle>0 AND an activity channel — always true here."
  critical: "READ-ONLY. If any parity assertion fails, the defect is HERE (P1.M2.T1.S2's engine) — surface it in the
             report; do NOT modify the executor in this task."

# CONTEXT — the config knobs (LANDED P1.M1.T1; consume)
- file: internal/config/roles.go   # ResolveRoleDeadlines (:78); config.go IdleTimeout (:87, default 60s) / Timeout (:86, 600s)
  why: "cfg.IdleTimeout + cfg.Roles[role].IdleTimeout are the knobs; ResolveRoleDeadlines('message'|'planner', cfg) is
        what generate/planner call. The global/large + role-or-global/small idiom."
  critical: "READ-ONLY. Defaults: Idle 60s / Hard 600s — tests always set BOTH explicitly (30s hard / 100-150ms idle)."

# CONTEXT — the consumers (S1 in-flight — NOT a dependency; S4 lands later)
- docfile: plan/022_856008a8cbb5/P1M3T1S1/PRP.md
  why: "S1 adds the emit/heartbeat knobs for personas (b)/(c) — consumed by P1.M3.T1.S3 (false-kill regression), NOT
        this task. Persona (a) (SleepMS) exists today. Confirms no overlap: S1 touches cmd/stubagent + internal/stubtest;
        S2 touches only test files in generate/decompose (+optional hook)."
  critical: "This task must NOT depend on S1's knobs (EmitMS/HeartbeatMS) — if you find yourself reaching for them,
             STOP: the false-kill/cap-pathology cases are S3's scope. A hung SleepMS stub is all S2 needs."
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  invariants_test.go            # EDIT — +the "IdleKill" TestInvariants row (+ui import)
  idle_parity_test.go           # CREATE — package generate_test: exit-124 parity + workdesc idle (own repo helpers)
  hooks_freeze_test.go          # READ-ONLY — the external-package precedent (initTempRepo pattern to copy)
  generate_test.go              # READ-ONLY — TestCommitStaged_Timeout (L449) + helpers (package generate, unimportable from _test pkg)
  generate_workdesc_test.go     # READ-ONLY — TestCommitStaged_WorkDescription_MessageRoleTimeout (the workdesc mirror's model)
  multiturn_test.go             # READ-ONLY — verboseCommandPrefix "DEBUG: command: " (L24)
internal/decompose/
  planner_test.go               # EDIT — +TestCallPlanner_IdleDeadline (mirror TestCallPlanner_Timeout L348)
  decompose_test.go             # EDIT — +TestDecompose_MessageIdleKillParity (mirror MessageRescuePartial L1770)
internal/hook/
  exec_test.go                  # EDIT (OPTIONAL) — +TestRun_IdleTimeoutNeverBlock (mirror TestRun_TimeoutNeverBlock L269)
internal/provider/executor.go   # READ-ONLY — the LANDED engine under observation (ErrIdleDeadline/Deadlines/VerboseCause)
internal/exitcode/exitcode.go   # READ-ONLY — For() (:63) + Timeout=124 (:32) + New (:19)
internal/config/roles.go        # READ-ONLY — ResolveRoleDeadlines (:78)
# go.mod / Makefile — READ-ONLY (no new deps; stdlib + existing internal imports only)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/generate/invariants_test.go        # MODIFIED — +1 table row ("IdleKill") + the ui import
internal/generate/idle_parity_test.go       # NEW — package generate_test; 2 tests + own minimal repo helpers
internal/decompose/planner_test.go          # MODIFIED — +TestCallPlanner_IdleDeadline + the ui import
internal/decompose/decompose_test.go        # MODIFIED — +TestDecompose_MessageIdleKillParity + exitcode/ui imports
internal/hook/exec_test.go                  # MODIFIED (OPTIONAL) — +TestRun_IdleTimeoutNeverBlock
# (NO production file. NO engine change. NO docs. NO cmd/stubagent or internal/stubtest change — S1's scope.)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (the import cycle that forces the external test package): internal/exitcode imports internal/generate
// (exitcode.go's For() checks errors.Is(err, generate.ErrTimeout)). A package-generate test file CANNOT import
// exitcode ("import cycle not allowed in test"). The exit-124 assertion MUST live in `package generate_test`
// (external — a separate package; hooks_freeze_test.go established the precedent and documents why: its header
// explains the same cycle for internal/hooks). The external file needs its OWN repo helpers (initTempRepo etc. —
// package generate's are unimportable); copy hooks_freeze_test.go's. decompose/hook tests have NO such constraint
// (exitcode does not import decompose/hook) — they import exitcode freely.

// CRITICAL (the LARGE-hard/SMALL-idle discriminator): ALWAYS set cfg.Timeout LARGE (30s) AND cfg.IdleTimeout small
// (100-150ms) with SleepMS 2000-5000. The stub sleeps SILENTLY, so the idle clock expires at the first detection
// tick (~0.5-1s) — well before both the 2000ms+ sleep ends AND the 30s hard cap. If the kill fires AT ALL under this
// config, it was the idle deadline (the hard cap mathematically cannot fire first). Never set only the idle knob with
// the 600s default hard — works, but the discriminator is illegible and the test would hang for minutes on an engine
// regression instead of failing fast.

// CRITICAL (the ~500ms watcher poll — why the kill lands at ~0.5-1s, not at 150ms): the idle watcher polls on ~500ms
// granularity; the FIRST tick initializes the activity baseline, a later tick detects no delta. An Idle of 100-150ms
// therefore kills at the first detection tick after expiry — roughly 0.5-1s after Execute starts. Budget test
// wall-time ~1-2s each; keep SleepMS ≥ 2× the expected kill point (2000ms standard, 5000ms for the decompose case);
// add -short skip guards (mirroring the Timeout row) so `-short` suites stay fast.

// CRITICAL (plannerDeps sets Verbose: nil — override it): planner_test.go's plannerDeps returns Deps{..., Verbose: nil}.
// The verbose cause line + the Execute-count are no-ops on a nil Verbose — the test MUST set
// deps.Verbose = ui.NewVerbose(&buf, true) after plannerDeps. Same for decompose's dcmOutBuffer (it wires deps.Out,
// the OUTPUT buffer — a different thing from Verbose; set deps.Verbose separately).

// CRITICAL (the no-retry counting technique — count "DEBUG: command: " occurrences): provider.Execute emits
// vb.VerboseCommand (→ "DEBUG: command: <argv>") exactly once per invocation (multiturn_test.go:24's
// verboseCommandPrefix idiom). A deadline kill returns from INSIDE the retry loop on the first hit ⇒ exactly 1
// occurrence. A parse-fail retry would show 2+. Assert strings.Count(buf.String(), "DEBUG: command: ") == 1.

// CRITICAL (the planner does NOT produce DecomposeRescueError — the contract's phrasing decoded): decompose/planner.go:130-132
// maps DeadlineExceeded → fmt.Errorf("%w: %w", ErrPlannerFailed, execErr) — NON-rescue, returned AS-IS by Decompose
// (decompose.go:219 "§G-PLANNER-NONRESCUE"). The "DecomposeRescueError traversal" the contract names belongs to the
// MESSAGE role (message.go:175-181 → *RescueError{ErrTimeout} → runLoop wraps DecomposeRescueError; Unwrap()→e.Rescue
// so errors.Is→generate.ErrTimeout→124 traverses — decompose_test.go:2063 pins the chain). Test C proves the planner
// traversal (ErrPlannerFailed+DeadlineExceeded); test D proves the DecomposeRescueError→124 traversal. BOTH are required.

// GOTCHA (preserve the asymmetries — do NOT "fix" them): one-shot/workdesc → ErrTimeout/124; multiturn turn-deadline
// → ErrRescue/3 (EXISTING tests cover the hard variant — TestRun_TurnError etc.; out of scope here); planner →
// non-rescue ErrPlannerFailed; hook → never-block "timed out" error. If an idle kill lands in a DIFFERENT bucket than
// its hard-cap sibling, THAT is an engine parity bug — report it, don't adjust the expectation.

// GOTCHA (the workdesc test needs SessionMode: append): RunWorkDescription's turn-1 RenderMultiTurn errors on a
// non-append manifest BEFORE Execute is reached (session_mode gate) — without m.SessionMode = &appendMode the test
// fails with a render-error cause, not an idle kill. Mirror TestCommitStaged_WorkDescription_MessageRoleTimeout's setup.

// GOTCHA (invariants row: build the Deps INSIDE run): the TestInvariants rows construct their own manifest/cfg/Deps
// per-scenario. Declare buf inside the run closure; the verbose assertions happen there; `return err, ""` hands the
// error to the harness for treeSHAFromErr + assertInvariants. Do NOT restructure the table.

// GOTCHA (external-package naming): in package generate_test everything is generate.X (generate.ErrTimeout,
// generate.RescueError, generate.FormatRescue, generate.CommitStaged, generate.Deps). Forgetting the qualifier is a
// compile error (the clearest possible failure — fine).

// GOTCHA (elapsed hang-guards): add `if elapsed := time.Since(start); elapsed > 10*time.Second { t.Errorf(...) }`
// style guards in the external parity test — an engine regression that makes the idle watcher never fire would
// otherwise hang the test until the 30s hard cap (or the suite timeout). 10s ≫ the expected ~1s kill.
```

## Implementation Blueprint

### Data models and structure

None (tests only). The tests consume the LANDED types: `provider.Deadlines` (via cfg), `*generate.RescueError`
(Kind/TreeSHA/ParentSHA/Candidate/Cause), `*decompose.DecomposeRescueError`, `exitcode.For`/`exitcode.Timeout`,
`ui.NewVerbose(w, on)`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/generate/invariants_test.go — the "IdleKill" TestInvariants row
  - ADD to the table immediately AFTER the existing "Timeout" row (same shape; the neighbor IS the doc):
        {
            name: "IdleKill",
            run: func(t *testing.T, _ context.Context, repo, bin string) (error, string) {
                if testing.Short() {
                    t.Skip("skipping slow IdleKill scenario in -short mode")
                }
                var vbuf bytes.Buffer
                m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: slow", SleepMS: 2000}) // persona (a): silent hang
                cfg := config.Defaults()
                cfg.Timeout = 30 * time.Second        // LARGE hard cap: mathematically cannot fire first
                cfg.IdleTimeout = 150 * time.Millisecond // SMALL idle: the only deadline that can kill
                deps := Deps{Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&vbuf, true)}
                _, err := CommitStaged(context.Background(), deps, cfg)
                if err != nil {
                    if !errors.Is(err, ErrTimeout) {
                        t.Errorf("IdleKill: errors.Is(err, ErrTimeout) = false, got %v", err)
                    }
                    // FR-A5: the ONLY observable difference — the cause line names the idle deadline.
                    if !strings.Contains(vbuf.String(), "DEBUG: cause: idle deadline") {
                        t.Errorf("IdleKill: verbose missing idle cause line; buf=\n%s", vbuf.String())
                    }
                    // NO-retry: exactly ONE Execute invocation (a retry would show 2+ "DEBUG: command: " lines).
                    if n := strings.Count(vbuf.String(), "DEBUG: command: "); n != 1 {
                        t.Errorf("IdleKill: Execute called %d times, want 1 (immediate rescue, NO retry)", n)
                    }
                }
                return err, "" // → treeSHAFromErr + assertInvariants (I1/I2/I3: no index/HEAD mutation, snapshot immutable)
            },
        },
  - IMPORTS: add "bytes" + internal/ui (errors/context/strings/testing/time/stubtest/config/git already present —
    verify against the file's block).
  - FOLLOW: the Timeout row verbatim (L219-235) — same skip guard, same error-return shape.
  - NOTE: the ErrIdleDeadline → ErrTimeout routing being asserted rides generate.go:376's
    errors.Is(execErr, context.DeadlineExceeded) — TRUE via ErrIdleDeadline.Is() (the D2 linchpin).

Task 2: CREATE internal/generate/idle_parity_test.go — package generate_test (the exit-124 proof + workdesc)
  - FILE COMMENT: cite FR-A5 + why the package is external (the exitcode import cycle; mirror hooks_freeze_test.go's
    header explanation).
  - HELPERS (own copies — package generate's are unimportable; mirror hooks_freeze_test.go):
    initTempRepo(t) (git init -q + user.name/user.email + an --allow-empty initial commit), writeFile, stageFile,
    headSHA (rev-parse HEAD).
  - TEST 1 — TestIdleKill_ExitCodeParity:
        repo := initTempRepo(t); writeFile+stageFile a change; before := headSHA(t, repo)
        var vbuf bytes.Buffer
        m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: never", SleepMS: 2000})
        cfg := config.Defaults(); cfg.Timeout = 30 * time.Second; cfg.IdleTimeout = 150 * time.Millisecond
        start := time.Now()
        _, err := generate.CommitStaged(context.Background(), generate.Deps{
            Git: git.New(repo), Manifest: m, Verbose: ui.NewVerbose(&vbuf, true)}, cfg)
        elapsed := time.Since(start)
        // (a) the rescue shape: *RescueError, Kind=ErrTimeout, tree SHA captured.
        var re *generate.RescueError
        if !errors.As(err, &re) { t.Fatalf("err = %v, want *generate.RescueError", err) }
        if !errors.Is(err, generate.ErrTimeout) { t.Errorf("errors.Is(ErrTimeout)=false: %v", err) }
        if re.TreeSHA == "" { t.Error("TreeSHA empty — snapshot missing from the rescue") }
        // (b) the rescue MESSAGE carries the tree SHA (the §18.3 "Tree ID:" line).
        if msg := generate.FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate); !strings.Contains(msg, "Tree ID: "+re.TreeSHA) {
            t.Errorf("FormatRescue missing 'Tree ID: %s'; msg=\n%s", re.TreeSHA, msg)
        }
        // (c) the cause names idle (ErrIdleDeadline.Error() contains "idle").
        if re.Cause == nil || !strings.Contains(re.Cause.Error(), "idle") {
            t.Errorf("Cause = %v, want an error naming 'idle'", re.Cause)
        }
        // (d) THE exit-124 semantics (the reason this file is package generate_test).
        if code := exitcode.For(err); code != exitcode.Timeout {
            t.Errorf("exitcode.For(err) = %d, want %d (124)", code, exitcode.Timeout)
        }
        // (e) no-retry + the FR-A5 cause line.
        if n := strings.Count(vbuf.String(), "DEBUG: command: "); n != 1 { t.Errorf("Execute count = %d, want 1", n) }
        if !strings.Contains(vbuf.String(), "DEBUG: cause: idle deadline") { t.Errorf("missing idle cause line") }
        // (f) nothing moved + a hang-guard.
        if got := headSHA(t, repo); got != before { t.Errorf("HEAD moved: %s → %s", before, got) }
        if elapsed > 10*time.Second { t.Errorf("idle kill took %v; want ≤ ~1s (hang-guard)", elapsed) }
  - TEST 2 — TestCommitStaged_WorkDescription_IdleKill: same shape + `cfg.WorkDescription = "add x"` and
    `appendMode := "append"; m.SessionMode = &appendMode` (mirror TestCommitStaged_WorkDescription_MessageRoleTimeout);
    assert errors.As→*RescueError + errors.Is(generate.ErrTimeout) + exitcode.For==124 + the idle cause line.
  - IMPORTS: bytes, context, errors, os/exec (repo helpers), strings, testing, time + internal/config,
    internal/exitcode, internal/generate, internal/git, internal/stubtest, internal/ui.
  - -short skip guards on both tests.

Task 3: EDIT internal/decompose/planner_test.go — TestCallPlanner_IdleDeadline
  - MIRROR TestCallPlanner_Timeout (L348-383) with the deadline knob swapped:
        bin := stubtest.Build(t); repo := t.TempDir(); initRepo(t, repo); commitRaw(t, repo, "initial")
        writeFile(t, repo, "a.txt", "new content") // UNSTAGED
        m := stubtest.Manifest(bin, stubtest.Options{Out: validMultiJSON, SleepMS: 2000})
        cfg := config.Defaults()
        cfg.Timeout = 30 * time.Second                                 // LARGE hard (discriminator)
        cfg.Roles = map[string]config.RoleConfig{"planner": {IdleTimeout: 100 * time.Millisecond}} // SMALL idle
        var vbuf bytes.Buffer
        deps := plannerDeps(t, repo, m); deps.Config = cfg
        deps.Verbose = ui.NewVerbose(&vbuf, true)   // plannerDeps sets nil — REQUIRED for the cause/count assertions
        baseTree, tStart := freezeForPlanner(t, repo, false)
        _, err := callPlanner(context.Background(), deps, 0, false, baseTree, tStart)
        // The traversal UNCHANGED (identical to TestCallPlanner_Timeout's assertions):
        if !errors.Is(err, ErrPlannerFailed) { t.Errorf("errors.Is(ErrPlannerFailed)=false: %v", err) }
        if !errors.Is(err, context.DeadlineExceeded) { t.Errorf("errors.Is(DeadlineExceeded)=false: %v", err) }
        // + the FR-A5 observables:
        if !strings.Contains(vbuf.String(), "DEBUG: cause: idle deadline") { t.Errorf("missing idle cause line") }
        if n := strings.Count(vbuf.String(), "DEBUG: command: "); n != 1 { t.Errorf("Execute count=%d, want 1 (no retry)", n) }
  - IMPORTS: add "bytes" + internal/ui (verify the existing block).
  - -short skip guard.

Task 4: EDIT internal/decompose/decompose_test.go — TestDecompose_MessageIdleKillParity
  - MIRROR TestDecompose_MessageRescuePartial (L1770-1850) with the message stub swapped to a HUNG plain Manifest:
        bin := stubtest.Build(t); repo := t.TempDir(); dcmInitRepo(t, repo)
        dcmWriteFile(t, repo, "a.txt", "aaa\n"); dcmWriteFile(t, repo, "b.txt", "bbb\n"); dcmWriteFile(t, repo, "c.txt", "ccc\n")
        plannerJSON := `{"count":3,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt"]},{"title":"c2","description":"b.txt","files":["a.txt"]},{"title":"c3","description":"c.txt","files":["c.txt"]}]}` // KEEP non-disjoint (tooled-stager path; do not switch to the fast-path)
        plannerM := dcmPlannerManifest(t, bin, plannerJSON)   // fast: returns the JSON immediately
        messageM := stubtest.Manifest(bin, stubtest.Options{Out: "feat: hang", SleepMS: 5000}) // HUNG: silent
        stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
        arbiterM := (mirror MessageRescuePartial's counter manifest, or a plain Manifest — the arbiter must NOT run)
        roles := RoleManifests{Planner: plannerM, Stager: stagerM, StagerAvailable: true, Message: messageM, Arbiter: arbiterM}
        deps, _ := dcmOutBuffer(t, repo, roles)
        cfg := config.Defaults()
        cfg.MaxDuplicateRetries = 0
        cfg.Timeout = 30 * time.Second
        cfg.Roles = map[string]config.RoleConfig{"message": {IdleTimeout: 100 * time.Millisecond}}
        deps.Config = cfg
        var vbuf bytes.Buffer
        deps.Verbose = ui.NewVerbose(&vbuf, true)
        deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}, "c2": {"b.txt"}, "c3": {"c.txt"}})
        before := headSHA-of-repo (the file's existing head helper; commitCount too)
        _, err := Decompose(context.Background(), deps)
        // (a) the DecomposeRescueError traversal: wrap → *RescueError → ErrTimeout → 124.
        var dre *DecomposeRescueError
        if !errors.As(err, &dre) { t.Fatalf("err = %T %v, want *DecomposeRescueError", err, err) }
        if !errors.Is(err, generate.ErrTimeout) { t.Errorf("errors.Is(generate.ErrTimeout)=false: %v", err) }
        if code := exitcode.For(err); code != exitcode.Timeout { t.Errorf("exitcode.For=%d, want 124", code) }
        // (b) the FR-A5 observables + (c) nothing landed (concept 0 died; no commit published).
        if !strings.Contains(vbuf.String(), "DEBUG: cause: idle deadline") { t.Errorf("missing idle cause line") }
        if got := head-helper; got != before { t.Errorf("HEAD moved on planner→message idle kill") }
        if count-after != count-before { t.Errorf("commits landed on a rescued run") }
  - IMPORTS: add internal/exitcode + internal/ui ("bytes" likely present — verify).
  - NOTE the DELTA from the model: MessageRescuePartial used a Script (concept 0 ok, concept 1 fails → partial
    commit[0]); here the hung Manifest kills concept 0 → NO commits published. Assert nothing-landed instead of
    partial-commit assertions.
  - -short skip guard.

Task 5 (OPTIONAL): EDIT internal/hook/exec_test.go — TestRun_IdleTimeoutNeverBlock
  - MIRROR TestRun_TimeoutNeverBlock (L269-294) with cfg{Timeout: 30s, IdleTimeout: 50ms} (config.Config literal +
    Roles map, matching the file's idiom): stub SleepMS 5000 → err contains "timed out"; msg-file byte-identical.

Task 6: VERIFY — build, vet, format, focused + full tests, lint, grep guards
  - go build ./... ; go vet ./internal/generate/... ./internal/decompose/... ./internal/hook/...
  - gofmt -l on the touched files   # empty
  - go test ./internal/generate/ -run 'IdleKill|Idle' -v ; go test ./internal/decompose/ -run 'Idle' -v
  - go test ./internal/generate/ ./internal/decompose/ ./internal/hook/ -count=1   # regression net
  - make test ; make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the LARGE-hard/SMALL-idle config (the self-discriminating fixture — every test uses it):
cfg := config.Defaults()
cfg.Timeout = 30 * time.Second           // LARGE: 30s ≫ SleepMS(2000-5000ms) — the hard cap CANNOT fire first
cfg.IdleTimeout = 150 * time.Millisecond // SMALL: the ONLY deadline that can kill the silent stub
// If the kill fires at all under this config, it was the idle deadline. The verbose cause line then CONFIRMS it.

// PATTERN: the three FR-A5 parity assertions (every test's core):
if !errors.Is(err, generate.ErrTimeout) { ... }                                  // same Kind as a hard timeout
if code := exitcode.For(err); code != exitcode.Timeout { ... }                    // 124 (external pkg / decompose only)
if !strings.Contains(vbuf.String(), "DEBUG: cause: idle deadline") { ... }       // the ONLY observable difference

// PATTERN: the no-retry proof (count Execute invocations in the verbose buffer):
if n := strings.Count(vbuf.String(), "DEBUG: command: "); n != 1 { ... } // 1 = killed on attempt 0; 2+ = a retry happened

// PATTERN: the rescue-message-with-tree-SHA proof (external file):
if msg := generate.FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate); !strings.Contains(msg, "Tree ID: "+re.TreeSHA) { ... }

// PATTERN: the external test package's own repo helpers (mirroring hooks_freeze_test.go — the cycle workaround):
// package generate_test — REQUIRED so this file can import internal/exitcode (exitcode imports generate;
// a package-generate test would be an import cycle). All generate identifiers qualified: generate.CommitStaged, …
```

### Integration Points

```yaml
TESTS (the ONLY deliverable):
  - internal/generate/invariants_test.go: +1 TestInvariants row ("IdleKill") → assertInvariants I1/I2/I3.
  - internal/generate/idle_parity_test.go: NEW external test pkg — exit-124 parity + workdesc idle.
  - internal/decompose/planner_test.go: +TestCallPlanner_IdleDeadline (ErrPlannerFailed traversal).
  - internal/decompose/decompose_test.go: +TestDecompose_MessageIdleKillParity (DecomposeRescueError→124 traversal).
  - (optional) internal/hook/exec_test.go: +TestRun_IdleTimeoutNeverBlock.

NO production/engine change (executor.go / generate.go / decompose/ / planner.go / hook/ / exitcode/ / cmd/ READ-ONLY —
  the engine LANDED in P1.M2.T1.S2/S3). NO stub change (persona (a) SleepMS exists; S1's knobs are S3's concern).
  NO docs ("tests only"). NO new deps (stdlib + existing internal imports).

DOWNSTREAM: P1.M3.T1.S4 (config observability — every precedence layer observably binds per role) reuses this harness;
  idle_parity_test.go (external, exitcode-capable) is its natural extension home. P1.M3.T1.S3 (false-kill regression)
  is the SIBLING that DOES need S1's personas (b)/(c) — no overlap with S2.

FENCE: if ANY parity assertion fails, the defect is in the LANDED ENGINE (e.g. ErrIdleDeadline's Is() not satisfying
  DeadlineExceeded, or a call site not threading Deadlines) — surface it in the implementation report; do NOT modify
  the engine or weaken the assertion in this task.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build + vet (test files compile alongside the landed engine; the external package resolves the exitcode import).
go build ./...
go vet ./internal/generate/... ./internal/decompose/... ./internal/hook/...
# Expected: clean. A failure usually means a missing import (ui/exitcode/bytes), a package-generate file importing
#           exitcode (the cycle — move it to the external file), or an unqualified generate.X in the external pkg.

# Format.
gofmt -l internal/generate/invariants_test.go internal/generate/idle_parity_test.go internal/decompose/planner_test.go internal/decompose/decompose_test.go
# (add internal/hook/exec_test.go if Task 5 done)   # Expected: empty.

# Lint.
make lint
# Expected: zero errors (all new symbols used; no unused imports).

# Scope guard: ONLY test files changed.
git status --short
# Expected: M invariants_test.go, ?? idle_parity_test.go, M planner_test.go, M decompose_test.go (+ M exec_test.go if
#           optional). NO internal/provider/, internal/generate/generate.go, internal/exitcode/, cmd/stubagent/, docs/.
```

### Level 2: Unit Tests (Component Validation) — THE PRIMARY GATE

```bash
# The new parity tests (focused).
go test ./internal/generate/ -run 'IdleKill|Idle' -v
# Expected: PASS — the invariants IdleKill row (I1/I2/I3 funnel) + TestIdleKill_ExitCodeParity (124 + Tree ID + cause
#           + count==1 + HEAD unchanged) + TestCommitStaged_WorkDescription_IdleKill.
go test ./internal/decompose/ -run 'Idle' -v
# Expected: PASS — TestCallPlanner_IdleDeadline (ErrPlannerFailed + DeadlineExceeded + cause + count) +
#           TestDecompose_MessageIdleKillParity (DecomposeRescueError → ErrTimeout → 124 + nothing landed).
go test ./internal/hook/ -run 'Idle' -v        # (only if Task 5)

# -short sanity (the guards skip; nothing hangs).
go test -short ./internal/generate/ ./internal/decompose/ -run 'Idle' -v
# Expected: all SKIP.

# The regression net: the existing timeout/parity suites stay green (the engine is untouched; the hard-cap rows still
# use cfg.Timeout).
go test ./internal/generate/ ./internal/decompose/ ./internal/hook/ -count=1
# Expected: green — esp. the invariants "Timeout" row, TestCallPlanner_Timeout, TestDecompose_MessageRescuePartial,
#           TestCommitStaged_WorkDescription_MessageRoleTimeout, TestRun_TimeoutNeverBlock (all untouched).

# Full race suite.
make test
# Expected: green (each test owns its repo/buffer; no shared state; -race clean).
```

### Level 3: Integration Testing (System Validation)

```bash
# There is no further integration surface for this task — the tests themselves ARE the pipeline integration (they
# drive the compiled stub through the REAL CommitStaged/Decompose pipelines, the real git plumbing on temp repos,
# and the real exitcode.For mapping). The e2e/subprocess harness (internal/e2e) is NOT extended (the contract's
# surface is the library level); a full CLI subprocess idle-kill e2e would be a future addition, not this task.

# Manual confidence check (optional): observe the parity by hand.
d=$(mktemp -d) && cd "$d" && git init -q && git config user.email t@t.com && git config user.name t
echo hi > f.txt && git add f.txt
SC=/home/dustin/projects/stagecoach/bin/stagecoach   # build first if absent
STAGECOACH_STUB_SLEEP_MS=5000 "$SC" --provider stub --model m --idle-timeout 1s --timeout 60s --verbose 2>&1 | tail -5
# Expected: the §18.3 rescue with "Tree ID: <sha>", exit code 124 (echo $?), and a "DEBUG: cause:" line mentioning idle.
cd - && rm -rf "$d"
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: the four required tests exist.
grep -c '"IdleKill"' internal/generate/invariants_test.go                                  # 1
grep -c 'func TestIdleKill_ExitCodeParity' internal/generate/idle_parity_test.go            # 1
grep -c 'func TestCommitStaged_WorkDescription_IdleKill' internal/generate/idle_parity_test.go # 1
grep -c 'func TestCallPlanner_IdleDeadline' internal/decompose/planner_test.go              # 1
grep -c 'func TestDecompose_MessageIdleKillParity' internal/decompose/decompose_test.go     # 1

# Guard 2: the external package (the import-cycle workaround) — idle_parity_test.go is package generate_test + imports exitcode.
head -20 internal/generate/idle_parity_test.go | grep -c 'package generate_test'            # 1
grep -c 'internal/exitcode' internal/generate/idle_parity_test.go                            # ≥1
# And NO package-generate test file imports exitcode (the cycle):
for f in internal/generate/*_test.go; do head -3 "$f" | grep -q '^package generate$' && grep -q 'internal/exitcode' "$f" && echo "CYCLE: $f"; done
# Expected: no CYCLE lines.

# Guard 3: the exit-124 assertions exist (the contract's headline).
grep -c 'exitcode.For(err)' internal/generate/idle_parity_test.go                            # ≥1
grep -c 'exitcode.For(err)' internal/decompose/decompose_test.go                             # ≥1

# Guard 4: the FR-A5 cause line + no-retry count asserted in each required test.
grep -c 'DEBUG: cause: idle deadline' internal/generate/invariants_test.go internal/generate/idle_parity_test.go internal/decompose/planner_test.go internal/decompose/decompose_test.go
# Expected: ≥1 each.
grep -c 'DEBUG: command: ' internal/generate/invariants_test.go internal/generate/idle_parity_test.go internal/decompose/planner_test.go
# Expected: ≥1 each (the count==1 no-retry proof).

# Guard 5: the LARGE-hard/SMALL-idle discriminator in every new test.
grep -n 'IdleTimeout = 1[05]0 \* time.Millisecond\|IdleTimeout: 1[05]0 \* time.Millisecond' internal/generate/invariants_test.go internal/generate/idle_parity_test.go internal/decompose/planner_test.go internal/decompose/decompose_test.go
# Expected: ≥1 per file; paired with a Timeout = 30s nearby.

# Guard 6: the -short guards.
grep -c 'testing.Short()' internal/generate/invariants_test.go   # ≥2 (Timeout row + IdleKill row)
grep -c 'testing.Short()' internal/generate/idle_parity_test.go internal/decompose/planner_test.go internal/decompose/decompose_test.go  # ≥1 each

# Guard 7: NO engine/production file touched.
git status --porcelain | grep -v '_test.go'
# Expected: EMPTY (only test files). Explicitly: no internal/provider/, no generate.go/planner.go/decompose.go/hook/,
#           no cmd/stubagent/, no internal/stubtest/ (S1's), no docs/.

# Guard 8: the existing models untouched (regression anchors).
git diff internal/generate/invariants_test.go | grep -E '^\-.*"Timeout"|^\-.*TestCallPlanner_Timeout'   # empty
git diff internal/decompose/planner_test.go internal/decompose/decompose_test.go | grep -E '^\-.*func Test' # empty
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `go vet ./internal/generate/... ./internal/decompose/... ./internal/hook/...` clean
- [ ] `gofmt -l` empty on all touched test files
- [ ] `make lint` zero errors
- [ ] `go test ./internal/generate/ -run 'IdleKill|Idle' -v` + `go test ./internal/decompose/ -run 'Idle' -v` green
- [ ] `go test -short ... -run 'Idle'` all SKIP (guards work; nothing hangs)
- [ ] `go test ./internal/generate/ ./internal/decompose/ ./internal/hook/ -count=1` green (regression net)
- [ ] `make test` (race) green

### Feature Validation (the FR-A5 parity contract)
- [ ] Idle kill → `*RescueError{Kind: ErrTimeout}` with non-empty TreeSHA (one-shot AND workdesc)
- [ ] `exitcode.For(err) == 124` (external parity test + decompose message test)
- [ ] FormatRescue contains "Tree ID: "+TreeSHA (the rescue message with tree SHA)
- [ ] NO retry: Execute-count == 1 in the invariants row, the parity test, and the planner test
- [ ] "--verbose contains the 'DEBUG: cause:' line naming 'idle'" — asserted in ALL FOUR required tests
- [ ] Invariants I1/I2/I3 hold on the deadline path (the IdleKill row's assertInvariants funnel)
- [ ] Planner: ErrPlannerFailed + DeadlineExceeded traversal UNCHANGED (non-rescue; no DecomposeRescueError)
- [ ] Decompose message: DecomposeRescueError → errors.Is(generate.ErrTimeout) → 124 traversal; HEAD unchanged; nothing landed
- [ ] The asymmetries preserved (not "fixed"): 124 vs multiturn-3 vs planner-non-rescue vs hook-never-block

### Scope-Boundary Validation
- [ ] `git status` == ONLY the test files (invariants_test.go, idle_parity_test.go NEW, planner_test.go, decompose_test.go, + optional exec_test.go)
- [ ] NO engine/production change (executor.go, generate.go, decompose/, planner.go, hook/, exitcode/, cmd/ — READ-ONLY)
- [ ] NO stub change (cmd/stubagent/, internal/stubtest/ — S1's scope; persona (a) SleepMS already exists)
- [ ] NO internal/e2e change; NO docs; NO new deps
- [ ] Grep guards 1–8 (Level 4) all pass

### Code Quality & Docs
- [ ] Each new test's comment cites FR-A5 + the mirror-source test (the file's documentation style)
- [ ] The external file's header documents the import-cycle rationale (mirroring hooks_freeze_test.go)
- [ ] Timing assertions are slack-tolerant (elapsed floors/hang-guards, never exact kill-time equality)

---

## Anti-Patterns to Avoid

- ❌ Don't put the exit-124 assertion in a `package generate` test file. internal/exitcode imports internal/generate
  (For() checks generate.ErrTimeout) — a package-generate test importing exitcode is "import cycle not allowed in test"
  and will NOT compile. The assertion MUST live in `package generate_test` (external — idle_parity_test.go, mirroring
  hooks_freeze_test.go's precedent) or in the decompose/hook tests (no cycle there). Guard 2 catches it.
- ❌ Don't rely on S1's knobs (EmitMS/HeartbeatMS). This task needs ONLY persona (a) — the hung SleepMS stub, which
  exists TODAY. The emit/heartbeat personas are P1.M3.T1.S3's (false-kill regression + cap pathology). A dependency on
  S1 here would serialize the two tasks for no benefit; if you find yourself configuring EmitMS/HeartbeatMS, you are
  writing S3's tests, not S2's.
- ❌ Don't set only the idle knob with the 600s default hard cap. Always set BOTH: `Timeout = 30s` (LARGE) +
  `IdleTimeout = 100-150ms` (SMALL) with SleepMS 2000-5000. The pairing is the built-in discriminator — under it, the
  hard cap mathematically cannot fire first (30s ≫ the 2-5s stub sleep), so a kill PROVES the idle deadline fired
  (and the cause line confirms it). With defaults, an engine regression makes the test hang for minutes instead of
  failing fast. Guard 5 checks the pairing.
- ❌ Don't assert the kill fires AT ~150ms. The idle watcher polls at ~500ms granularity; the first tick initializes
  the baseline — a 150ms idle kills at the first DETECTION tick (~0.5-1s). Assert the OUTCOME (the error chain, the
  cause line) and a generous hang-guard (elapsed ≤ 10s), never an exact kill-time window. Exact-time assertions flake
  on loaded CI runners.
- ❌ Don't expect DecomposeRescueError from the planner. decompose/planner.go:130-132 maps DeadlineExceeded →
  `ErrPlannerFailed` (NON-rescue, "no retry"), returned as-is by Decompose (§G-PLANNER-NONRESCUE). The contract's
  "DecomposeRescueError traversal" lives in the MESSAGE role (message.go:175 → *RescueError{ErrTimeout} → runLoop wraps
  DecomposeRescueError; Unwrap → 124 traverses). Test C asserts the planner chain (ErrPlannerFailed + DeadlineExceeded);
  test D asserts the DecomposeRescueError chain. Writing `errors.As(err, &dre)` in the planner test WILL fail — that's
  correct behavior, not a bug to work around.
- ❌ Don't forget `deps.Verbose` on the planner/decompose tests. plannerDeps hardcodes `Verbose: nil` (VerboseCause is
  a nil-safe no-op); dcmOutBuffer wires deps.Out (the OUTPUT buffer — a different thing). Without
  `deps.Verbose = ui.NewVerbose(&buf, true)` the cause-line and Execute-count assertions silently pass against an empty
  buffer... no — they FAIL (Contains on "") — but confusingly. Set the Verbose explicitly in every test that asserts it.
- ❌ Don't "fix" the asymmetries. one-shot/workdesc → 124; multiturn turn-deadline → 3; planner → non-rescue; hook →
  never-block "timed out". These are the SPECIFIED contract (§18.2, §13.6.6, FR-H5), not bugs. If an idle kill lands in
  a DIFFERENT bucket than its hard-cap sibling, that's an ENGINE parity defect — surface it; do not adjust the test's
  expectation to match a broken engine.
- ❌ Don't modify the engine to make a test pass. The executor (P1.M2.T1.S2), the call sites (S3), the exitcode mapping
  — all LANDED and READ-ONLY here. A failing parity assertion means the engine broke FR-A5; the correct action is to
  report it (the task FAILS loudly) — patching executor.go/generate.go from a tests-only task crosses the task boundary
  and hides the regression. Guard 7 enforces the fence.
- ❌ Don't restructure the TestInvariants table. The row shape ({name, run}) and the assertInvariants funnel are the
  §20.2 property suite's contract (cited by docs + other rows). Add the IdleKill row AFTER Timeout, mirroring its shape
  (the -short guard, the in-run errors.Is, `return err, ""`). A "cleaner" table refactor breaks the harness's
  treeSHAFromErr wiring.
- ❌ Don't switch the decompose test's plannerJSON to a disjoint partition. TestDecompose_MessageRescuePartial's JSON
  deliberately shares a.txt across c1/c2 (non-disjoint ⇒ the tooled-stager path ⇒ the dcmStagerSeam). A disjoint JSON
  would flip the run onto the FR-M13 fast-path (no stager agent) — a DIFFERENT code path than the model test. Keep the
  model's JSON + seam verbatim; only the message manifest and the cfg change.
- ❌ Don't drop the -short guards. The idle tests take ~1-2s each (the ~500ms poll forces real sleeps); `-short` runs
  (go test -short ./...) must stay fast. Every timing test gets `if testing.Short() { t.Skip(...) }` (the Timeout row's
  precedent). Guard 6 checks.
- ❌ Don't assert on the multiturn turn-deadline path here. The contract's LOGIC is one-shot CommitStaged (+ workdesc
  via the asymmetry line) + ONE decompose case each for planner and message. The multiturn → ErrRescue/3 asymmetry is
  EXISTING coverage (multiturn_test.go); extending it to idle variants is scope creep for S3/S4 to pick up if needed.

---

## Confidence Score: 9/10

Every hop of the proven chain is code-verified (the stub persona exists today; ResolveRoleDeadlines + the Deadlines
call shape at generate.go:375/planner.go:128/message.go:175 are LANDED; ErrIdleDeadline's Error() contains "idle" and
its Is() satisfies DeadlineExceeded — the D2 linchpin; executor.go:152's VerboseCause string; generate.go:376's
no-retry branch and its RescueError fields; exitcode.For's ErrTimeout-before-rescue order → 124; FormatRescue's
"Tree ID:" line; the planner's ErrPlannerFailed non-rescue mapping; DecomposeRescueError.Unwrap → 124). Every test
mirrors a read-verbatim existing model (the invariants Timeout row, TestRun_MessageRoleTimeoutBoundsTurn's
LARGE/SMALL idiom, TestCallPlanner_Timeout, TestDecompose_MessageRescuePartial + its dcm helpers,
TestCommitStaged_WorkDescription_MessageRoleTimeout, hooks_freeze_test.go's external-package precedent). The one
structural trap (the exitcode import cycle) is identified WITH its established workaround. The -1 from 10/10 reflects
inherent timing-test risk (six-ish real-sleep tests on loaded CI; the ~500ms poll granularity) — mitigated by the
slack-tolerant guidance (outcome assertions + 10s hang-guards, never exact kill-times; -short skips) — and the one
judgment call: the decompose test's concept-0-hangs simplification (vs the model's concept-1-fails Script) changes
which assertions apply (nothing-landed instead of partial-commit[0]) — spelled out in the Blueprint so the implementer
doesn't cargo-cult the model's partial-result assertions into a scenario where concept 0 died. No engine dependency
risk (LANDED), no S1 dependency (persona (a) exists), no docs, no production surface.
