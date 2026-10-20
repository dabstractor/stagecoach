# Provider.Execute call sites, the timeout error contract, verbose surfaces, and the stub test harness

**Grounding for spec v3.5 (idle + hard-cap generation deadlines).** Research snapshot at HEAD `df98a9a` ("spec: add idle + hard-cap generation deadlines (v3.5)"). Spec anchor — `spec/SPEC.md` revision row (L28) summarizes v3.5: kill for *silence* (idle deadline, default 60s; stdout/stderr writes + Linux `/proc/<pid>/io` read counter of the process group) with a raised hard cap (120s→600s) as the backstop for false-alive pathologies (heartbeat-only streams); both resolve **per role exactly like provider/model/reasoning (FR-R7)**; bind **each invocation individually** ("a fresh idle clock per multi-turn turn / work-description round"); and an idle kill must be **downstream-identical to a timeout today (exit 124, rescue, no-retry — only the `--verbose` cause line differs)**. Everything below maps where that threading must land.

---

## 1. The seam: `provider.Execute` (internal/provider/executor.go)

`func Execute(ctx, spec CmdSpec, timeout time.Duration, vb *ui.Verbose) (stdout, stderr string, err error)` — **executor.go:65**.

Timeout mechanics (executor.go:66–70):

```go
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout) // SHADOW — see doc; do not rename
		defer cancel()
	}
```

Kill semantics: the child is its own process-group leader (`setupProcessGroup`, procgroup_unix.go / procgroup_windows.go — the windows file's L39 comment notes the same sentinel reaches "orchestrator exit 3 / 124 + rescue (§18.2)"); `cmd.Cancel` SIGTERMs the whole group, `WaitDelay` 3s then SIGKILL.

Error contract — verbatim from the Execute doc (executor.go:56–63):

```
// Error contract (check ctx.Err() FIRST):
//
//   - timeout        ⇒ err IS context.DeadlineExceeded  (orchestrator: exit 124 + rescue)
//   - signal/parent  ⇒ err IS context.Canceled          (orchestrator: exit 3 + rescue)
//   - non-zero exit  ⇒ wrapped *exec.ExitError           (orchestrator: retry, then rescue)
//   - start failure  ⇒ wrapped LookPath/start error      (orchestrator: "command not found", exit 1)
//   - success        ⇒ err == nil
```

The discriminating branch (executor.go:92–99):

```go
	if werr := cmd.Wait(); werr != nil {
		vb.VerboseRawOutput(out.String())
		vb.VerboseStderr(errb.String()) // surface the provider's real failure reason (upstream errors live on stderr)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return out.String(), errb.String(), ctxErr // timeout → DeadlineExceeded; cancel → Canceled
		}
		return out.String(), errb.String(), fmt.Errorf("provider %q: %w", spec.Command, werr) // exit failure
	}
```

**Load-bearing detail for v3.5:** stdout/stderr are captured to separate `bytes.Buffer`s and returned **even on error** (partial output reaches callers), but the buffers only fill when the child *writes* — `Execute` itself has no per-write visibility today. An idle-deadline watcher needs a byte-activity signal (buffer-write hook, `io/proc` polling, or event-stream parsing) at or below this seam; every call site below then keeps its existing `DeadlineExceeded` branch untouched.

Verbose calls inside Execute (executor.go:88–101): `vb.VerboseCommand(argv)`, `vb.VerbosePayload(spec.PayloadBytes)`, then on Wait error `VerboseRawOutput` + `VerboseStderr`, and the same two on success. `signal.RegisterChild(cmd.Process.Pid)` / `defer signal.ClearChild()` bracket the wait (executor.go:92–94).

---

## 2. Call-site inventory (every `provider.Execute` in non-test code)

| # | File:line | Role | Timeout resolution | Timeout error branch |
|---|-----------|------|--------------------|----------------------|
| 1 | internal/generate/generate.go:374 | message (one-shot, CommitStaged) | `config.ResolveRoleTimeout("message", cfg)` @ L303 | `RescueError{Kind: ErrTimeout}` — immediate return, no retry |
| 2 | internal/generate/generate.go:335–341 | message (workdesc mode, cause mapping) | resolved inside RunWorkDescription (workdesc.go:71) | `errors.Is(wdCause, DeadlineExceeded)` → `RescueError{Kind: ErrTimeout}` |
| 3–5 | internal/generate/multiturn.go:172 / 183 / 194 | message (multi-turn turns 1 / 2..N / N+1) | `config.ResolveRoleTimeout("message", cfg)` inside `Run` (~L153) | `execErr != nil` → return cause (FR-T7 abort, ALL error classes) |
| 6–8 | internal/generate/workdesc.go:82 / 131 / 149 | message (workdesc turn 1 / forced-conclusion / read-answer) | `config.ResolveRoleTimeout("message", cfg)` @ L71 | `execErr != nil` → return cause (FR-T7 parity) |
| 9 | internal/decompose/planner.go:128 | planner | `config.ResolveRoleTimeout("planner", deps.Config)` @ L69 | `ErrPlannerFailed` wrap, immediate return, no retry |
| 10 | internal/decompose/stager.go:119 | stager | `config.ResolveRoleTimeout("stager", deps.Config)` @ L98 | any execErr → `ErrStagerFailed` wrap, no retry, no parse |
| 11 | internal/decompose/message.go:173 | message (per-concept, decompose) | `config.ResolveRoleTimeout("message", deps.Config)` @ L145 | `generate.RescueError{Kind: generate.ErrTimeout, TreeSHA: treeB}` — immediate |
| 12 | internal/decompose/arbiter.go:101 | arbiter | `config.ResolveRoleTimeout("arbiter", deps.Config)` @ L83 | DeadlineExceeded‖Canceled → **graceful null**, not an error |
| 13 | internal/hook/exec.go:187 | message (prepare-commit-msg hook one-shot) | `config.ResolveRoleTimeout("message", cfg)` @ L167 | `errors.New("stagecoach: hook generation timed out")` — no retry, never-block |
| 14 | internal/cmd/models.go:150 | n/a (ListModelsCommand probe) | `cfg.Timeout` (120s default when cfg nil) — **not role-resolved** | any err → curated fallback table + stderr notice, exit 0 |

`ResolveRoleTimeout` (internal/config/roles.go:96–104): per-role `cfg.Roles[role].Timeout` → built-in `defaultRoleTimeouts` (roles.go:11 — planner 480s is the ONLY built-in) → `cfg.Timeout`. v3.5 retires the planner special-case as moot and adds idle/hard-cap resolution "exactly like provider/model/reasoning" — this function (and its flag/env/file loaders) is the resolution seam to extend.

### 2.1 generate.go:374 — CommitStaged one-shot message loop

Role: **message**. `msgTimeout := config.ResolveRoleTimeout("message", cfg)` @ generate.go:303 (comment L299–302: "FR-R7/FR25: … [role.message].timeout / --message-timeout bound the message agent's one-shot generation (and the multi-turn total budget, FR-T5) instead of the flat cfg.Timeout").

```go
// generate.go:369–394
		spec, rerr := deps.Manifest.Render(msgModel, sysPrompt, payload, msgReasoning)
		if rerr != nil {
			return Result{}, fmt.Errorf("commit staged: render: %w", rerr)
		}

		out, stderr, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
		if execErr != nil {
			if errors.Is(execErr, context.DeadlineExceeded) {
				// §5: immediate rescue, NO retry — agent was killed.
				return Result{}, &RescueError{
					Kind: ErrTimeout, TreeSHA: treeSHA, ParentSHA: parentSHA,
					Candidate: candidate, Cause: execErr,
				}
			}
			if errors.Is(execErr, context.Canceled) {
				return Result{}, &RescueError{
					Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA,
					Candidate: candidate, Cause: execErr,
				}
			}
			// Non-zero exit (*exec.ExitError): fall through to ParseOutput.
			// stdout may be partial-valid. Record the cause for eventual rescue.
			lastCause = execErr
		} else {
			lastCause = nil
		}
```

Snapshot context: `treeSHA, err := deps.Git.WriteTree(ctx)` @ L271; `signal.SetSnapshot(treeSHA, parentSHA, "")` @ L276 (arms the interrupt rescue); `lock.SetSnapshot(treeSHA)` @ L277.

### 2.2 generate.go:335–341 — work-description cause mapping

```go
// generate.go:335–342
		wdMsg, wdOK, wdCause := RunWorkDescription(ctx, deps, cfg, deps.Manifest,
			wdSysPrompt, wdPayload, skeleton, msgModel, msgReasoning)
		if wdCause != nil {
			// FR-T7 parity: a turn error/timeout/cancel/non-append-provider aborts → rescue.
			lastCause = wdCause
			if errors.Is(wdCause, context.DeadlineExceeded) {
				return Result{}, &RescueError{Kind: ErrTimeout, TreeSHA: treeSHA, ParentSHA: parentSHA, Candidate: wdMsg, Cause: wdCause}
			}
		} else if wdOK {
```

(Non-deadline cause falls through to the generic `&RescueError{Kind: ErrRescue, …, Cause: lastCause}` at L505–508 after the one-shot loop / multi-turn gate.)

### 2.3 multiturn.go:172 / 183 / 194 — `Run`, the N+1-turn protocol

Role: **message**, per-turn. `msgTimeout := config.ResolveRoleTimeout("message", cfg)` inside `Run` (~L153; doc at multiturn.go:132–135 states "Per-turn timeout = msgTimeout = ResolveRoleTimeout("message", cfg) (FR-R7/FR-T5; Execute shadows ctx with WithTimeout)"). Failure-handling doc (multiturn.go:134–140) — verbatim, the FR-T7 contract:

```
// stdout ("ok") is discarded. Failure handling (FR-T7): ANY turn's Execute error OR any RenderMultiTurn
// error ⇒ Run aborts and returns the raw error as `cause` (the caller maps it to &RescueError{Cause:
// cause}). The decision is `execErr != nil` (NOT errors.Is(...)) because FR-T7 treats a turn timeout,
// cancel, AND non-zero-exit ALL as abort — and intermediate turns discard stdout anyway, so there is no
// value in parsing partial output (unlike the one-shot path's fall-through at generate.go:242).
```

```go
// multiturn.go:165–174  (turn 1)
	spec, rerr := manifest.RenderMultiTurn(msgModel, sysPrompt, preamble+"\n\n"+chunks[0].text,
		msgReasoning, sessionID, 1)
	if rerr != nil {
		return "", false, rerr // non-append provider ⇒ RenderMultiTurn's session_mode gate; surface as cause
	}
	if _, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose); execErr != nil {
		return "", false, execErr // FR-T7: any turn error/timeout/cancel/non-zero-exit aborts
	}

// multiturn.go:176–186  (turns 2..N)
	// (6) Turns 2..N: each chunk's body; sysPrompt="" ⇒ RenderMultiTurn drops the system_prompt_flag.
	for i := 2; i <= N; i++ {
		spec, rerr := manifest.RenderMultiTurn(msgModel, "", chunks[i-1].text,
			msgReasoning, sessionID, i)
		if rerr != nil {
			return "", false, rerr
		}
		if _, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose); execErr != nil {
			return "", false, execErr // FR-T7
		}
	}

// multiturn.go:188–197  (final turn N+1)
	// (7) Turn N+1 (final): the commit-message request. The candidate message is THIS turn's stdout.
	spec, rerr = manifest.RenderMultiTurn(msgModel, "", finalInstruction,
		msgReasoning, sessionID, N+1)
	if rerr != nil {
		return "", false, rerr
	}
	out, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
	if execErr != nil {
		return "", false, execErr // (8) FR-T7: final-turn error aborts
	}
```

Caller side (generate.go:480–505): `msg2, ok2, cause := Run(...)`; on `cause != nil || !ok2` → `lastCause = cause; candidate = msg2` → generic ErrRescue. Protocol constants: `preambleFmt` (multiturn.go:117) and `finalInstruction` (multiturn.go:121) are VERBATIM spec strings. **v3.5 note:** "a fresh idle clock per multi-turn turn" is already the shape here — each turn calls Execute with its own timeout; only the *kind* of deadline changes.

### 2.4 workdesc.go:82 / 131 / 149 — `RunWorkDescription` read/answer loop

Role: **message**. `msgTimeout := config.ResolveRoleTimeout("message", cfg)` @ workdesc.go:71.

```go
// workdesc.go:78–85  (turn 1)
	spec, rerr := manifest.RenderMultiTurn(msgModel, sysPrompt, payload, msgReasoning, sessionID, 1)
	if rerr != nil {
		return "", false, rerr // non-append provider ⇒ RenderMultiTurn's session_mode gate; surface as cause
	}
	out, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
	if execErr != nil {
		return "", false, execErr // FR-T7 parity: any turn error/timeout/cancel/non-zero-exit aborts
	}

// workdesc.go:125–134  (forced-conclusion turn, FR-W6 round cap)
			refuse := fmt.Sprintf(refuseReadsFmt, st.rounds, st.N)
			spec, rerr := manifest.RenderMultiTurn(msgModel, "", refuse, msgReasoning, sessionID, turn)
			if rerr != nil {
				return "", false, rerr
			}
			out2, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
			if execErr != nil {
				return "", false, execErr
			}

// workdesc.go:145–152  (each read-answer turn)
		spec, rerr = manifest.RenderMultiTurn(msgModel, "", answer, msgReasoning, sessionID, turn)
		if rerr != nil {
			return "", false, rerr
		}
		out, _, execErr = provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
		if execErr != nil {
			return "", false, execErr // FR-T7 parity
		}
```

**v3.5 note:** the read loop is unbounded in turns (only the READ round cap bounds it) — "a fresh idle clock per work-description round" means each of these three Execute calls gets its own idle clock, exactly as they each get `msgTimeout` today.

### 2.5 decompose/planner.go:128 — `callPlanner` (retry ≤2 on PARSE only)

Role: **planner**. `plannerTimeout := config.ResolveRoleTimeout("planner", deps.Config)` @ planner.go:69 (doc: "the built-in 480s default (longer than the 120s global) … A non-zero cfg.Roles["planner"].Timeout always wins").

```go
// planner.go:128–135
		out, stderr, execErr := provider.Execute(ctx, *spec, plannerTimeout, deps.Verbose)
		if execErr != nil {
			if errors.Is(execErr, context.DeadlineExceeded) || errors.Is(execErr, context.Canceled) {
				return prompt.PlannerOutput{}, fmt.Errorf("%w: %w", ErrPlannerFailed, execErr) // non-rescue; no retry
			}
			// Non-zero exit — stdout may be partial; fall through to parse.
		}

		parsed, perr := prompt.ParsePlannerOutput(out)
```

Timeout/cancel → **non-rescue** `ErrPlannerFailed` immediately; only parse/contract failures consume the second attempt.

### 2.6 decompose/stager.go:119 — `stageConcept` (tooled, single shot)

Role: **stager**. `stagerTimeout := config.ResolveRoleTimeout("stager", deps.Config)` @ stager.go:98.

```go
// stager.go:118–121
	// 4. Execute once. NO retry (the orchestrator owns FR-M8); NO parse (no JSON contract).
	if _, _, execErr := provider.Execute(ctx, *spec, stagerTimeout, deps.Verbose); execErr != nil {
		return fmt.Errorf("%w: %w", ErrStagerFailed, execErr)
	}
```

Timeout, cancel, and non-zero exit are indistinguishable here — all fold into `ErrStagerFailed` (the orchestrator retries once then treats the concept as empty, FR-M8/M12).

### 2.7 decompose/message.go:173 — per-concept message loop (mirror of CommitStaged)

Role: **message**. `messageTimeout := config.ResolveRoleTimeout("message", deps.Config)` @ message.go:145.

```go
// message.go:173–193
		out, stderr, execErr := provider.Execute(ctx, *spec, messageTimeout, deps.Verbose)
		if execErr != nil {
			if errors.Is(execErr, context.DeadlineExceeded) {
				// §5: immediate rescue, NO retry — agent was killed.
				return "", &generate.RescueError{
					Kind: generate.ErrTimeout, TreeSHA: treeB,
					ParentSHA: parentSHA, Candidate: candidate, Cause: execErr,
				}
			}
			if errors.Is(execErr, context.Canceled) {
				return "", &generate.RescueError{
					Kind: generate.ErrRescue, TreeSHA: treeB,
					ParentSHA: parentSHA, Candidate: candidate, Cause: execErr,
				}
			}
			// Non-zero exit (*exec.ExitError): fall through to ParseOutput.
			// stdout may be partial-valid. Record the cause for eventual rescue.
			lastCause = execErr
		} else {
			lastCause = nil
		}
```

The concept rescue is re-wrapped by the orchestrator into `*DecomposeRescueError` (decompose.go:79–103) whose `Unwrap()` returns the inner `*generate.RescueError` so `errors.Is(err, generate.ErrTimeout)` traverses → exitcode 124.

### 2.8 decompose/arbiter.go:101 — `runArbiter` (single shot, owns the null)

Role: **arbiter**. `arbiterTimeout := config.ResolveRoleTimeout("arbiter", deps.Config)` @ arbiter.go:83.

```go
// arbiter.go:100–108
	// 5. Execute ONCE (NO retry — §17.7 defines no retry instruction; the arbiter is "when in doubt, null").
	out, _, execErr := provider.Execute(ctx, *spec, arbiterTimeout, deps.Verbose)
	if execErr != nil {
		if errors.Is(execErr, context.DeadlineExceeded) || errors.Is(execErr, context.Canceled) {
			// Timeout / cancel → graceful null (the arbiter OWNS the null decision, §13.6.5).
			return prompt.ArbiterOutput{Target: nil}, nil
		}
		// Non-zero exit — stdout may be partial/valid; fall through to parse (mirrors callPlanner/generate).
	}
```

**v3.5 note:** an *idle* kill must land in this same branch (graceful null), NOT become a hard decompose failure — the arbiter's "when in doubt, null" contract swallows deadline kills today.

### 2.9 hook/exec.go:187 — prepare-commit-msg hook one-shot

Role: **message** (FR-H6). `msgTimeout := config.ResolveRoleTimeout("message", cfg)` @ hook/exec.go:167 (comment verbatim in §8 below).

```go
// hook/exec.go:187–196
		out, _, execErr := provider.Execute(ctx, *spec, msgTimeout, deps.Verbose)
		if execErr != nil {
			if errors.Is(execErr, context.DeadlineExceeded) {
				return errors.New("stagecoach: hook generation timed out") // no retry; never-block
			}
			if errors.Is(execErr, context.Canceled) {
				return errors.New("stagecoach: hook generation cancelled")
			}
			// Non-zero exit: fall through to ParseOutput (partial stdout may be valid).
		}
```

The hook never rescues (git owns this commit): any error propagates to the cmd layer's never-block mapping (exit 0, msg-file untouched — FR-H5). The hook's own multi-turn gate (~L243–266) calls the exported `generate.Run` and falls through to `"stagecoach: hook generation failed after %d retries"` on ANY turn failure.

### 2.10 cmd/models.go:150 — `renderModelBlock` (models --all live list)

```go
// cmd/models.go:143–165 (abridged; comment at L134: "if ListModelsCommand is set, run it via
// provider.Execute; on success print the live list")
		timeout := 120 * time.Second
		if cfg != nil {
			timeout = cfg.Timeout // bound (FR25 knob; default 120s)
		}
		var vb *ui.Verbose
		if cfg != nil && cfg.Verbose {
			vb = ui.NewVerbose(cmd.ErrOrStderr(), true)
		}
		out, _, err := provider.Execute(cmd.Context(),
			provider.CmdSpec{Command: argv[0], Args: argv[1:], Stdin: ""}, // Env nil ⇒ inherit
			timeout, vb)
		if err == nil {
			printLiveList(cmd.OutOrStdout(), t.name, out)
			return
		}
		// FR-L1 (b): command failed → curated fallback
		fmt.Fprintf(cmd.ErrOrStderr(), "stagecoach: %s list command failed (%v); using curated table\n", t.name, err)
```

No role, no rescue, no retry — a timeout is just "command failed". An idle-deadline change that alters what `Execute` accepts for `timeout` should keep this site compiling (it passes a plain `time.Duration`).

---

## 3. Exit-124 mapping, ErrTimeout, rescue entry, tree-SHA print

**Exit code constant** — internal/exitcode/exitcode.go:32:

```go
	Timeout         = 124 // generation exceeded --timeout (mirrors GNU `timeout`)
```

**`exitcode.For(err)`** (exitcode.go:63–96) — the timeout checks, in order:

```go
	// *RescueError.Unwrap()==Kind; check timeout BEFORE rescue (a timeout IS a rescue with Kind=ErrTimeout).
	if errors.Is(err, generate.ErrTimeout) {
		return Timeout
	}
	if errors.Is(err, generate.ErrRescue) {
		return Rescue
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Timeout
	}
```

Doc (exitcode.go:61–62): "generate-domain mapping (NothingToCommit→2, Rescue→3, Timeout/Deadline→124, CAS→1); else 1. A *generate.RescueError whose Kind is ErrTimeout maps to 124 (checked BEFORE the generic rescue→3)."

**ErrTimeout definition** — internal/generate/generate.go:85–88:

```go
// ErrTimeout is returned when generation exceeded cfg.Timeout (the agent was
// killed). CLI → exit 124 + FormatRescue (PRD §15.4). Returned wrapped in
// *RescueError{Kind: ErrTimeout}. Reached AFTER the snapshot — TREE_SHA is set.
var ErrTimeout = errors.New("stagecoach: generation timed out")
```

**RescueError** — generate.go:104–118: `Kind` (ErrTimeout|ErrRescue, `Unwrap()` returns it), `TreeSHA`, `ParentSHA` ("" on root), `Candidate`, `Cause` ("underlying: context.DeadlineExceeded / *exec.ExitError / nil — for verbose/diag"). CLI recipe in the doc (generate.go:108–111):

```
//	var re *generate.RescueError
//	if errors.As(err, &re) {
//	    print(FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate))
//	    exit = errors.Is(err, generate.ErrTimeout) ? 124 : 3
//	}
```

**CLI entry** — internal/cmd/default_action.go `handleGenError` (~L374–388): `errors.As(err, &re)` → `fmt.Fprintln(stderr, generate.FormatRescue(re.TreeSHA, re.ParentSHA, re.Candidate))`; under verbose, `fmt.Fprintln(stderr, "DEBUG: cause: "+re.Cause.Error())` (**default_action.go:381** — the only `DEBUG:` writer outside internal/ui); then

```go
		code := exitcode.Rescue
		if errors.Is(err, generate.ErrTimeout) { // timeout → 124; rescue → 3 (timeout checked first)
			code = exitcode.Timeout
		}
```

Dry-run variant (default_action.go:357–368): a `*RescueError` under `--dry-run` prints a short line ("generation timed out; run without --dry-run to see the recovery recipe" when `errors.Is(err, generate.ErrTimeout)`) and maps to exit 1 — no recipe.

**Rescue message / tree-SHA print** — internal/generate/rescue.go `FormatRescue` (L30–79). The tree-SHA print is line 4 of the message (~rescue.go:44–48):

```go
	// Line 3 — reassurance.
	b.WriteString("Your staged files were safely snapshotted before generation.\n")
	// Line 4 — Tree ID (substitute treeSHA).
	b.WriteString("Tree ID: ")
	b.WriteString(treeSHA)
	b.WriteByte('\n')
```

followed by the manual-recovery recipe `  git commit-tree [-p <parentSHA>] -m "Your message" <treeSHA> | xargs git update-ref HEAD`. `FormatRescueMulti` (rescue.go:81+) is the per-concept variant ("Concepts already published are final and untouched. …"). Signature FROZEN per the work-item contract.

**Interrupt rescue entry** — internal/signal/signal.go: `SetSnapshot(treeSHA, parentSHA, candidate)` (L202–207, called at generate.go:276 immediately after WriteTree) arms it; `SetCandidate` (L212) keeps the candidate note current. On signal with a snapshot armed, the handler prints and exits 3 (signal.go:168–179):

```go
	tree, parent, cand := h.snapTree, h.snapParent, h.snapCandidate
	…
	if tree != "" {
		fmt.Fprintln(h.opts.Out, h.opts.RescueFormat(tree, parent, cand)) // Fprintln adds the trailing \n
		…
		h.opts.Exit(3)                                                    // §18.2: post-snapshot → exit 3
```

The library default RescueFormat override (signal.go:90–91): `return "❌ Commit generation failed.\nTree ID: " + tree`. `signal.RestoreDefault()` runs before UpdateRefCAS (generate.go:581); `signal.ClearSnapshot()` on success (generate.go:626). `signal.RegisterChild(pid)`/`ClearChild` are the nil-safe wrappers Execute calls (signal.go:183–194).

**Decompose traversal** — decompose.go:79–103: `DecomposeRescueError{Rescue *generate.RescueError, ConceptTitle, Index, Count, Commits}`; `Unwrap()` returns `e.Rescue` so `errors.Is(..., generate.ErrTimeout)` → 124 traverses; the loop prints `FormatRescueMulti` BEFORE returning it.

---

## 4. No-retry-on-timeout (where a timeout bypasses the parse-retry loop)

The FR29/FR32 bounded retry loops (`for attempt := 0; attempt <= cfg.MaxDuplicateRetries; …`) consume attempts ONLY on parse-fail and duplicate-subject. A `DeadlineExceeded` **returns from inside the loop body on the first hit**:

- **generate.go:376–382** — `if errors.Is(execErr, context.DeadlineExceeded) { // §5: immediate rescue, NO retry — agent was killed. return Result{}, &RescueError{Kind: ErrTimeout, …} }` (the comment "§5: immediate rescue, NO retry — agent was killed." is the invariant).
- **decompose/message.go:175–181** — identical shape (`Kind: generate.ErrTimeout, TreeSHA: treeB`).
- **hook/exec.go:189–191** — `return errors.New("stagecoach: hook generation timed out") // no retry; never-block` (no rescue object at all).
- **decompose/planner.go:130–132** — `DeadlineExceeded || Canceled → return … ErrPlannerFailed … // non-rescue; no retry`.
- **decompose/arbiter.go:103–105** — timeout → graceful null (also exits the single-shot call immediately).
- **multiturn.go / workdesc.go** — `execErr != nil` aborts the turn sequence outright (no retry concept exists there).

By contrast, a **non-zero exit** deliberately falls through to `ParseOutput` ("stdout may be partial-valid") and only exhausts into rescue. v3.5's "idle kill is downstream-identical to a timeout today (exit 124, rescue, no-retry)" means: whatever sentinel/wrapper an idle kill produces must satisfy `errors.Is(err, context.DeadlineExceeded)` at every branch above (or each branch must be extended), or the no-retry guarantee silently breaks.

---

## 5. The FR-T5 multi-turn budget/progress line (generate.go ~L470–474)

Verbatim, generate.go:465–477 (gate → budget → progress → verbose trigger → Run):

```go
			// Condition (b): the (now-untruncated) payload must exceed one chunk for multi-turn to help.
			if git.EstimateTokens(mtPayload) > cfg.MultiTurnChunkTokens {
				// FR-T5: surface the turn count + total wall-clock budget (timeout × turns) on the progress
				// line. Deps.Progress is a no-arg callback (can't carry the message) → direct stderr write.
				turns := len(chunkPayload(mtPayload, cfg.MultiTurnChunkTokens)) + 1 // N chunks + 1 final turn
				totalMin := int((msgTimeout * time.Duration(turns)).Minutes())
				if totalMin < 1 {
					totalMin = 1
				}
				fmt.Fprintf(os.Stderr, "↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n", turns, cfg.MultiTurnChunkTokens, totalMin)

				// FR-T11: verbose trigger line (per-turn verbose is emitted by provider.Execute inside Run).
				deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn fallback")
```

Variable names: `turns` = `len(chunkPayload(mtPayload, cfg.MultiTurnChunkTokens)) + 1` (N chunks + 1 final turn); `totalMin` = `(msgTimeout * turns).Minutes()` floored at 1; format `"↳ falling back to multi-turn: %d turns (chunks of ~%d tokens), ~%dm total\n"`. **This is the one place today that multiplies a role timeout by a turn count** — v3.5's "bind each invocation individually" makes this `timeout × N` display wrong (N independent idle clocks, not one shared budget); the wording must be reworked when deadline semantics change. The hook twin is hook/exec.go:~255–262 (uses the exported `generate.ChunkCount` + `1`, same Fprintf string split across two lines, nil-guarded Verbose).

---

## 6. internal/ui — files, exported verbose/logging functions, every `DEBUG:` emitter

Files: `output.go` (+`output_test.go`), `verbose.go` (+`verbose_test.go`), `isatty_{darwin,linux,other,windows}.go`, `termwidth_{other,unix,windows}.go`.

**verbose.go — the verbose sink** (`Verbose` struct: `mu sync.Mutex`, `w io.Writer`, `on bool`; every method no-ops on nil receiver / nil writer / off; goroutine-safe for the concurrent fast-path):

| Function | Line | Format |
|---|---|---|
| `NewVerbose(w io.Writer, on bool) *Verbose` | 44 | — |
| `VerboseCommand(cmd string)` | 51 | `"DEBUG: command: <cmd>\n"` — argv ONLY, never Env (PRD §19) |
| `VerboseRawOutput(output string)` | 68 | `"DEBUG: raw output:\n"` + stdout verbatim (trailing \n ensured) |
| `VerboseStderr(stderr string)` | 90 | `"DEBUG: stderr:\n"` + stderr verbatim; skipped when stderr empty |
| `VerbosePayload(bytes int)` | 112 | `"DEBUG: payload: %d bytes (~%d tokens est)\n"` (size only, `(bytes+3)/4`) |
| `VerboseWarn(msg string)` | 123 | `"DEBUG: <msg>\n"` |
| `VerboseRetry(attempt int, reason string)` | 135 | `"DEBUG: attempt %d: %s\n"` (1-based) |
| `VerboseRoles(roles []RoleLine)` | 164 | `"DEBUG: %-8s %s%s\n"` per role (name, model/provider, reasoning suffix) |
| `RoleLine` struct | ~146 | Name/Model/Provider/Reasoning |

**output.go — the progress/success layer (not DEBUG-prefixed):** `IsTerminal` (32), `TerminalWidth` (41), `ResolveColor` (58), `New(stdout, stderr, color)` (84), `Color` (95), `Green/Red/Yellow` (98/101/104), `Progress(msg)` (118 — the "↳"-style progress writer), `Success(msg)` (125), `invocation` (133), `ProgressLabel(verb, model, provider)` (147 — builds "Generating with <model>…"), `Error(msg)` (156). The FR-T5 line deliberately bypasses `Progress` (no-arg callback) with a direct `os.Stderr` write.

**Every `DEBUG:` emitter** (rg over non-test code): all of verbose.go above (L57, 74, 96, 118, 129, 141, 171) plus exactly one outside internal/ui — **default_action.go:381** `fmt.Fprintln(stderr, "DEBUG: cause: "+re.Cause.Error())` (rescue cause under --verbose; deliberately mirrors VerboseWarn style because the §18.3 rescue message is spec-frozen byte-for-byte). Notable `VerboseWarn` callers an idle-cause line would join: exclude.go:85 ("ignoring unsupported negation pattern in .stagecoachignore: …"), default_action.go:207/210 ("note: --model/--provider shadowed by [role.message].…"), generate.go:477 and hook/exec.go:~264 ("one-shot exhausted → multi-turn fallback"), generate.go:~604 ("post-mutation index reconcile failed (commit stands): …"). Tests already assert `DEBUG:` substrings (default_action_test.go:926–930, 1441, 1686; executor_test.go:156–162, 194; exclude_test.go:100; generate_multiturn_test.go:24 splits the verbose buffer on `"DEBUG: command: "`). VERBOSE=2 is documented but rejected by config.loadEnv (verbose.go doc, D9) — a verbose-intensity axis exists conceptually if the v3.5 cause line needs gating.

---

## 7. Multi-turn failure handling (FR-T7) — the deadline abort branches

`rg -n 'DeadlineExceeded|deadline' internal/generate/` (non-test) resolves to exactly:

- **generate.go:117** — `RescueError.Cause` doc: "underlying: context.DeadlineExceeded / *exec.ExitError / nil".
- **generate.go:340** — workdesc cause branch (§2.2 above): `if errors.Is(wdCause, context.DeadlineExceeded) { return … &RescueError{Kind: ErrTimeout, TreeSHA: treeSHA, …} }`; non-deadline causes keep `lastCause = wdCause` and fall to the generic ErrRescue return.
- **generate.go:376** — one-shot branch (§2.1).
- multiturn.go/workdesc.go carry no `DeadlineExceeded` identifier at all — their abort decision is `execErr != nil` (§2.3/§2.4), and the deadline-vs-cancel-vs-exit distinction is made by the CALLER only where it matters (generate.go:340). The caller-side mapping of a Run cause (generate.go:482–505): `if cause != nil { lastCause = cause; if msg2 != "" { candidate = msg2 } }` → `&RescueError{Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA, Candidate: candidate, Cause: lastCause}` (L505–508). **Note the asymmetry:** a multi-turn turn TIMEOUT surfaces as `Kind: ErrRescue` (exit 3), NOT `ErrTimeout` (exit 124) — only the workdesc branch (L340) and the one-shot branch (L376) mint ErrTimeout today. v3.5 says an idle kill is "exit 124, rescue, no-retry" — if that must hold on the multi-turn path too, this caller branch needs the same `errors.Is(cause, context.DeadlineExceeded)` check the workdesc branch has.

---

## 8. internal/hook/exec.go — the role-deadline comment (verbatim)

hook/exec.go:161–169 (Step F of `hook.Run`):

```go
	// Step F: resolve the message role (FR-H6 — exactly like the single-commit path).
	_, msgModel, msgReasoning := config.ResolveRoleModel("message", cfg)
	// FR-R7/FR-H6: resolve the message role's timeout so [role.message].timeout / --message-timeout bound
	// the hook one-shot generation (and the multi-turn total budget, FR-T5) instead of the flat cfg.Timeout.
	// With no per-role override ResolveRoleTimeout returns cfg.Timeout (no message built-in) — behavior-
	// preserving by default.
	msgTimeout := config.ResolveRoleTimeout("message", cfg)
```

The "(and the multi-turn total budget, FR-T5)" clause is the same stale shared-budget assumption as §5. FR-V7 itself lives one layer up — hook ABORTS (not timeouts) are mapped in generate.go:531–534: "A hook abort returns BEFORE CommitTree … as a *RescueError (FR-V7 → exit 3, byte-identical to a generation failure)"; the hook's own timeout branch is the never-block `errors.New("stagecoach: hook generation timed out") // no retry; never-block` (exec.go:190) — no rescue object, msg-file untouched, exit 0 at the cmd layer (FR-H5).

---

## 9. Stub / integration test harness

### 9.1 internal/stubtest (the reusable fake-agent kit)

`stubtest.go`:
- `Build(t)` — compiles **cmd/stubagent** ONCE per test process (`sync.Once` cache), `go build` run at the module root discovered from the file's own path (cwd-independent); skips if no go toolchain. Returns the binary path.
- `Options` (L23–36) — the env-knob struct: `Out` (`STAGECOACH_STUB_OUT`, single-response), `Exit` (`…_EXIT`), `SleepMS` (`…_SLEEP_MS`, ">0 ⇒ slow/timing-out agent"), `Stderr` (`…_STDERR`), `Script`+`Counter` (`…_SCRIPT`/`…_COUNTER`, call-varying), `Output`/`StripCodeFence` (manifest parse knobs), `ArgsFile` (`…_ARGSFILE`, dumps received argv NUL-joined), `Marker` (`…_MARKER`, writes "1" once stdin is drained = generation-in-flight signal).
- `Env(o)` — os.Environ() + knobs (for raw CmdSpecs). `Manifest(bin, o)` — a full `provider.Manifest{Name:"stub", Command:bin, PromptDelivery:"stdin", Output:"raw", StripCodeFence:true, Env:knobs}` — **the stub rides the REAL `provider.Execute` seam** (Render → CmdSpec.Env → cmd.Env). `NewScript(t, bin, responses)` — writes script.txt + counter to TempDir and returns the Manifest (blank entries significant; last response repeats).

`editor.go`: `BuildEditor(t)` (compiles cmd/stubeditor), `EditorOptions{Msg, Exit}` → `STAGECOACH_EDITOR_*` env, `SetEditorEnv` (t.Setenv). `BuildTee(t)` (compiles cmd/stubtee — stdin-tee wrapper for capture). There is also `STAGECOACH_STUBCLI_SLEEP_MS` (editor.go:192) for a slow/hanging *editor*.

### 9.2 cmd/stubagent/main.go — the fake agent binary (stdlib only)

Execution order (each invocation = fresh process):
1. **Drain stdin FIRST** (deadlock guard: the executor pipes the payload through a ~64KiB OS pipe; sleeping before draining deadlocks parent+child on big payloads). Optional `STAGECOACH_STUB_STDINFILE` capture or `STAGECOACH_STUB_MATCHFILE` tee.
2. `STAGECOACH_STUB_INTERVAL_FILE` — appends `start_ns end_ns` per invocation (concurrency-overlap probe).
3. `STAGECOACH_STUB_MARKER` — writes "1" (readiness; BEFORE the sleep, so tests can race deterministically).
4. `STAGECOACH_STUB_ARGSFILE` — writes argv NUL-joined.
5. **Sleep AFTER draining**: `STAGECOACH_STUB_SLEEP_MS` uniform; MATCHFILE rules may add a per-match `|sleepMs`.
6. `STAGECOACH_STUB_STDERR` → stderr (single write).
7. **Stdout selection, precedence MATCHFILE > Script > OUT** — `selectScripted` (L124: file-backed counter `STAGECOACH_STUB_COUNTER`, read-increment-write; clamps to last line after exhaustion), `selectMatched` (L181: first `<substring>|<output>[|<sleepMs>]` rule whose substring appears in the stdin payload wins — concurrency-safe, no counter). Output is written with ONE `fmt.Fprint(os.Stdout, out)` immediately before exit — **no timed/progressive emission exists today**.
8. `STAGECOACH_STUB_EXIT` → os.Exit.

### 9.3 internal/generate tests — how CommitStaged is driven with stubs

- **invariants_test.go** (§20.2 property suite): `waitForMarker(t, path, timeout)` (L35–46, 10ms poll); `repoSnapshot{head, indexNames, indexFull}`; `treeSHAFromErr` (extracts TreeSHA from `*RescueError`/`*CASError`, fails on pre-snapshot); `assertInvariants(t, repo, before, treeSHA, wantHead)` (L97–130) — the no-mutation property: **I1** idempotent index (`git diff --cached --name-only` AND `git diff --cached` byte-identical), **I2** atomic HEAD (`git rev-parse HEAD` unchanged), **I3** snapshot immutability (`cat-file -t` = tree; `cat-file -p <treeSHA>` stable after staging NEW content into the index). `TestInvariants` table (L163+): ParseFail, DuplicateExhaustion, AgentNonzeroExit, **Timeout** (`stubtest.Manifest(bin, Options{Out: "feat: slow", SleepMS: 2000})` + `cfg.Timeout = 150ms` → asserts `errors.Is(err, ErrTimeout)`, then all three invariants), SigintContextCancel (marker + `cancel()` from another goroutine → ErrRescue), CASFailure (marker-raced `moveHEADWithoutStaging`). Every scenario funnels through `assertInvariants`.
- **repro_freeze_test.go**: `waitForFile` (L23–36) + `TestCommitStaged_GenerationFreeze_HoldsForLiveStagedSentinel` — stub `SleepMS: 800, Marker:`; stages a sentinel into the LIVE index mid-generation via a goroutine + marker poll; asserts the commit contains only the frozen snapshot (index/HEAD discipline on the generation window).
- **hooks_freeze_test.go**: same freeze discipline across blocking pre-commit hooks (`deadline := time.Now().Add(5s)` guard loop at L126).
- **multiturn_test.go**: `TestRun_TurnError` (~L231, global `Exit:1` ⇒ turn-1 abort, cause != nil), **`TestRun_MessageRoleTimeoutBoundsTurn` (L246–274)** — the canonical role-timeout proof: stub `Out:"ok", SleepMS:2000`; `cfg.Timeout = 30s` (LARGE global) + `cfg.Roles = {"message": {Timeout: 150ms}}` (SMALL role); asserts `errors.Is(cause, context.DeadlineExceeded)`. `TestRun_FinalParseEmpty`, `TestRun_NonAppendManifest`.
- **generate_workdesc_test.go**: **`TestCommitStaged_WorkDescription_MessageRoleTimeout` (L288–318)** — same LARGE-global/SMALL-role pattern through `cfg.WorkDescription` + `SessionMode:&"append"`; asserts `*RescueError` + `errors.Is(err, ErrTimeout)`.
- **generate_multiturn_failure_test.go**: header doc explains the mid-turn-failure mechanism without stub changes (global `STUBCOACH_STUB_EXIT`-style single exit knob; the STUB_COUNTER file is the discriminator: "1" ⇒ one-shot only / gate skipped; "2" ⇒ gate fired + Run aborted at turn 1). `assertMultiTurnRescue` asserts TreeSHA == pre-run `git write-tree`, ParentSHA == pre-run HEAD, HEAD unchanged, index unchanged (names + full diff).
- **generate_multiturn_test.go**: `verboseCommandPrefix = "DEBUG: command: "` (L24) — splits the verbose buffer to count per-turn Execute calls.
- **realagent_test.go**: build tag `integration_real`, gated on `STAGECOACH_RUN_REAL=1` — real 6-provider matrix; not in CI.

### 9.4 internal/e2e (build tag `e2e` — subprocess CLI harness)

harness_test.go: `buildStagecoach` (compiles cmd/stagecoach once), `buildStub` (stubtest.Build), `newRepo`/`seedCommit`/`writeFile`/`stageFile`/`runGit`/`headSHA`/`commitCount`/`diffTreeNames`/`statusPorcelain`, `writeStubConfig` (TOML with `[provider.stub]` command=stubBin, `prompt_delivery="stdin"`, `output="raw"` + extras), `stubEnv(knobs)` (process env carries the STAGECOACH_STUB_* knobs — G-ENV-FLOW), `runStagecoach` (**60s context timeout so a hung agent fails the test, not the suite**; `--config cfg --no-color` + args; captures stdout/stderr/exit code), `waitForMarker`, `skipIfNotReal`/`realAgent` (STAGECOACH_RUN_REAL=1 + STAGECOACH_E2E_PROVIDER). Scenarios: scenarios_test.go (S-series incl. concurrent CAS races), hook_scenarios_test.go, lock_scenarios_test.go, orphan_reclaim_scenarios_test.go, config_precedence_test.go.

### 9.5 Timeout scenarios tested TODAY (complete list)

1. generate/invariants_test.go `Timeout` scenario (SleepMS 2000 / cfg.Timeout 150ms → ErrTimeout + I1/I2/I3).
2. generate/multiturn_test.go `TestRun_MessageRoleTimeoutBoundsTurn` (role 150ms vs global 30s → DeadlineExceeded cause).
3. generate/generate_workdesc_test.go `TestCommitStaged_WorkDescription_MessageRoleTimeout` (same pattern via CommitStaged → RescueError/ErrTimeout).
4. hook/exec_test.go `TestRun_TimeoutNeverBlock` (L269–294: SleepMS 5000 / Timeout 50ms → error contains "timed out", msg-file byte-identical) and `TestRun_MessageRoleTimeoutNeverBlock` (L298+: global 30s / role 50ms; notes the budget display is grep-guarded, not exercised).
5. decompose per-role timeout proofs: planner_test.go:348, stager_test.go:167–179, message_test.go:219–241, arbiter_test.go:237–251 — each `Roles: {"<role>": {Timeout: 100ms}}` + sleeping stub, proving ResolveRoleTimeout bounded Execute.
6. stubtest_test.go:67,106 — the stub's own drain-before-sleep deadlock guard.

**NOT tested today:** e2e has NO timeout scenario (no SleepMS/124/"timed out" in internal/e2e — only the 60s harness cap); no test distinguishes timeout from cancel on the multi-turn caller path (§7 asymmetry); nothing exercises progressive output or heartbeat-style activity.

### 9.6 How to build the three v3.5 stubs

**(a) Hung agent stub** (zero activity → idle deadline must fire; hard cap alone would also catch it):
exists today. `stubtest.Manifest(bin, stubtest.Options{Out: "feat: never", SleepMS: 600000, Marker: markerPath})` + a small role/global timeout. The stub drains stdin, writes the marker, then `time.Sleep`s in silence — no stdout/stderr bytes, flat /proc io. For a *deterministic* post-snapshot kill, poll `waitForMarker` before arming whatever the test races. Config recipe mirrors TestRun_MessageRoleTimeoutBoundsTurn: `cfg.Timeout` LARGE, `cfg.Roles["message"].Timeout` SMALL — swap the SMALL knob for the v3.5 idle deadline once it resolves through the same `RoleConfig` fields. There is no "infinite" hang knob; a large SleepMS is the idiom (a `SleepMS: 0 && hang-forever` knob would need adding only if a test must outlive every configured deadline).

**(b) Slow-but-progressively-emitting stub** (activity keeps resetting the idle clock; hard cap must eventually kill; partial stdout must reach the caller): **NOT expressible with today's stubagent** — stdout is written exactly once, immediately before exit (step 7 of §9.2), and Script/MATCHFILE vary content, not timing. Add a knob pair to cmd/stubagent, e.g. `STAGECOACH_STUB_EMIT_MS` (write interval) + `STAGECOACH_STUB_EMIT_BYTES`/`STAGECOACH_STUB_EMIT_LINE` (chunk per tick), emitting ticks to stdout AFTER the stdin drain/marker and BEFORE the final `fmt.Fprint(out)` + exit (optionally `STAGECOACH_STUB_EMIT_TOTAL_MS` to bound the run and let a healthy path finish). Wiring facts that make it work: (1) `provider.Execute` captures stdout in a `bytes.Buffer` — the ticks are invisible to `ParseOutput` until exit but ARE the byte-activity an idle watcher observes, and on a deadline kill the PARTIAL buffer is returned alongside `DeadlineExceeded` (executor.go:95–97), so a test can assert both the kill and the partial-capture contract; (2) the stub must keep draining stdin first (deadlock guard) and keep writing the marker before the first tick.

**(c) Chatty-forever heartbeat stub** (the false-alive pathology — codex-style periodic stderr timer; idle must NOT fire, hard cap MUST): **NOT expressible today** (stderr is a single optional write, step 6). Add e.g. `STAGECOACH_STUB_HEARTBEAT_MS` — after the stdin drain + marker, loop `fmt.Fprintln(os.Stderr, "…tick…")` every N ms forever (or until `STAGECOACH_STUB_HEARTBEAT_TOTAL_MS`), never writing stdout. This stub is the regression test for "activity is a productivity signal, not a progress guarantee, and the cap MUST NOT be removed" (spec v3.5 row): configure idle > heartbeat interval so the idle clock keeps resetting, hard cap small, and assert the kill is downstream-identical to a timeout (exit 124 / RescueError / no-retry / invariants hold). Both (b) and (c) belong in cmd/stubagent (stdlib-only, single binary) with the knobs surfaced as `stubtest.Options` fields + `optsEnvMap` entries, so every existing Manifest/NewScript/Env path picks them up; the counter/marker/argsfile ordering (marker BEFORE first activity tick) must be preserved for the deterministic-race tests.

---

## 10. Implementer checklist (what v3.5 threading touches, from this map)

1. **Seam:** `provider.Execute` (executor.go:65) — inject/observe byte activity per invocation; return something `errors.Is(…, context.DeadlineExceeded)`-compatible for an idle kill (or extend all 14 branches).
2. **Resolution:** `config.ResolveRoleTimeout` (roles.go:96) + the flag/env/file/TOML loaders + `defaultRoleTimeouts` (roles.go:11, planner 480s retired as moot) — add idle/hard-cap fields to `RoleConfig` with the same precedence.
3. **Per-invocation binding:** already structurally true — every call site passes its own duration to each Execute (multi-turn turns and workdesc rounds each call Execute individually); only the *budget display* at generate.go:467–474 and hook/exec.go:~255–262 assumes one shared wall clock.
4. **Branches to keep byte-compatible:** §2.1, §2.2, §2.5, §2.6, §2.7, §2.8, §2.9 (arbiter must keep swallowing deadline kills as null; hook keeps never-block); §2.3/§2.4 keep `execErr != nil` aborts.
5. **Asymmetry to decide:** multi-turn turn-timeout currently exits 3 (ErrRescue), not 124 — only one-shot (generate.go:376) and workdesc (generate.go:340) mint ErrTimeout. v3.5 says idle kill ⇒ 124; align deliberately.
6. **Verbose:** the cause line is `default_action.go:381` `DEBUG: cause: …` — the "only the --verbose cause line differs" requirement lands there (and/or a VerboseWarn at the kill site).
7. **Tests:** reuse stubtest + the LARGE-global/SMALL-role idiom; add the two new stubagent knobs (§9.6 b/c); add an e2e timeout scenario (none exists); extend invariants' Timeout row to the idle/hard-cap kills; grep-guarded budget line (hook/exec_test.go:~315 note) must be updated with the §5 rewording.
