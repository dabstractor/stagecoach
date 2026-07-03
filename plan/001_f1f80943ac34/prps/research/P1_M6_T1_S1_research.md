# Research — P1.M6.T1.S1: internal/generate/generate.go — CommitStaged(ctx, deps) (Result, error)

The two-nested-loop snapshot-based atomic-commit orchestrator. Integrates M2 (provider
Run/parse), M3 (git plumbing/diff/log), M4 (prompt builders), M5 (config), and this
milestone's S2 (firstLine/isDuplicate) + S3 (Rescue) + T2.S1 (signal handler). This note
records the **five ambiguities** found during codebase analysis and the binding resolution
for each, so the implementer does not have to re-derive them.

## 0. Verified dependency signatures (the exact call shapes)

- `git.StagedDiff(cfg git.DiffSettings) (string, error)` — `DiffSettings{MaxMdLines, MaxDiffBytes int}`.
- `git.RevParseHEAD() (sha string, hasParent bool, err error)` — hasParent=false on unborn (root) repo.
- `git.WriteTree() (string, error)` — abort error mentions "conflict" on unresolved merge.
- `git.CommitTree(parent, msg, tree string) (string, error)` — OMITS `-p` when parent=="".
- `git.UpdateRefCAS(ref, newSHA, expected string) error` — 3-arg CAS; expected=="" ⇒ root 1-arg form.
- `git.RecentSubjects(n int) ([]string, error)` — trimmed, newest-first, nil on unborn.
- `git.CommitCount() (int, error)` / `git.RecentMessages(n int) (string, error)` — both route unborn → (0,nil)/("",nil).
- `provider.Executor.Run(ctx, m Manifest, model, provider, sys, payload string) (string, error)`.
- `provider.Manifest` value (Name, Command, RetryInstruction, Env, …); `provider.NewExecutor(dir)`.
- `prompt.FetchExamples(g HistoryReader, n int) (examples []string, hasMultiline bool, err error)` —
  `*git.Git` satisfies `prompt.HistoryReader` structurally (CommitCount + RecentMessages).
  `prompt.DefaultExampleCount = 20`. Returns (nil,false,nil) when CommitCount<=1.
- `prompt.BuildSystemPrompt(examples []string, hasMultiline, newRepo bool, target int) string`.
- `prompt.AssemblePayload(diff, instruction string, rejected []string) string` — **diff FIRST** (D5).
- `firstLine(msg string) string`, `isDuplicate(subject string, subjects []string) bool` (S2, shipped).
- `Rescue(out *ui.Output, tree, parent, candidate string)` (S3, shipped) — pure render to stderr.
- `installSignalHandler(ctxCancel, rescueFn func())`, `restoreSignalHandler()` (T2.S1, shipped).
- `ui.NewOutput(stdout, stderr io.Writer, verbose, noColor bool) *Output`; `out.Resultf`→stdout,
  `out.Progressf`→stderr, `out.Verbosef`→stderr(gated). Exit constants `ui.ExitSuccess/0`,
  `ExitError/1`, `ExitNothingToCommit/2`, `ExitRescue/3`, `ExitTimeout/124`.
- `config.Config{Provider, Model string; Timeout time.Duration; AutoStageAll, Verbose, NoColor bool;
  MaxDiffBytes, MaxMdLines, MaxDuplicateRetries, SubjectTargetChars int; Output string;
  StripCodeFence bool; …}`; `config.Default()` ⇒ Timeout=120s, MaxDuplicateRetries=3,
  MaxDiffBytes=300000, MaxMdLines=100, SubjectTargetChars=50.

## 1. Ambiguity #1 — `parseOutput` is UNEXPORTED in `provider` (RESOLVED → add `(*Executor).Parse`)

`parseOutput(raw, m)` (P1.M2.T2.S1) is deliberately package-private (`provider/parse.go`).
`generate` is a DIFFERENT package, so it CANNOT call `provider.parseOutput`. The parse research
(P1_M2_T2_S1 §4) explicitly deferred this: *"The cross-package consumer (generate, P1.M6.T1.S1)
will, when it arrives, either export it (`ParseOutput`) or call through an exported provider
entry point (e.g. an Executor wrapper). That export decision belongs to M6."*

**Resolution:** add a one-line exported method to the Executor (the natural seam — it already owns
`Run`, and the orchestrator already injects an `*provider.Executor`):

```go
// in internal/provider/executor.go (ADDITIVE — do not touch parse.go/parse_test.go)
func (e *Executor) Parse(raw string, m Manifest) (string, bool) { return parseOutput(raw, m) }
```

This keeps `parseOutput` pure/unexported (parse_test.go stays white-box `package provider`),
exposes the two-step Run-then-Parse shape the work-item contract names
(`stdout,err=Run(...); msg,ok=parseOutput(...)`), and lets the `runner` interface (below) carry
both methods so a stub runner in unit tests fakes Parse while the REAL Executor satisfies it for
integration tests.

## 2. Ambiguity #2 — the corrective-retry PAYLOAD (RESOLVED → keep the diff; swap the instruction slot)

The contract pseudocode says `else payload=manifest.RetryInstruction`, and decisions.md §3 says
`payload = correctiveInstruction`. Taken literally (payload = ONLY the instruction string) this
DROPS the diff — but each `Run` is a FRESH process with NO conversation memory, so the model would
have nothing to commit. The proven reference (`reference_impl.md §2`) is unambiguous:
`run_agent(stdin = diff + user_prompt)` EVERY attempt; on parse-failure only `user_prompt` is
swapped to the corrective instruction (`user_prompt = "Output STRICT valid JSON only…"`).

**Resolution (reference-faithful):** the corrective retry REBUILDS the payload via
`prompt.AssemblePayload(diff, correctiveInstruction, rejected)` — the diff (and the growing
`rejected` list) STAYS; only the `instruction` slot changes. `correctiveInstruction =
manifest.RetryInstruction` when non-empty, else the original instruction
(`"Generate a commit message for these changes:"`). Note only `pi` ships a non-empty
`RetryInstruction`; for raw-output providers it is empty, so the corrective retry re-sends the
same payload (acceptable — parse rarely fails for raw output, and a non-deterministic model may
succeed). The `instruction` slot is RESET to the original at the top of each OUTER iteration (the
corrective state does not leak across dup-retries).

## 3. Ambiguity #3 — the counting semantics (RESOLVED → PRD wording: 1 initial + N retries)

`reference_impl.md §2` flags an off-by-one: the reference ran the body for
`duplicate_retry ∈ {0,1,2}` = **3 total** generations, while PRD FR32 phrases it as "up to
`max_duplicate_retries` (default 3) retries" = 1 initial + 3 retries = **4 total**. The reference
doc's OWN recommendation: *"implement the loop as the PRD describes … because the PRD is the
product spec."* The work-item contract is explicit: `for dupAttempt:=0; dupAttempt<=cfg.MaxDuplicateRetries;
dupAttempt++ (== 1 initial + N retries per PRD FR32)`.

**Resolution:** OUTER loop `for dupAttempt := 0; dupAttempt <= cfg.MaxDuplicateRetries; dupAttempt++`
(inclusive `<=`) ⇒ `MaxDuplicateRetries+1` total generations (default 4). INNER loop
`for parseAttempt := 1; parseAttempt <= 2; parseAttempt++` ⇒ a FRESH 2-try budget EVERY outer
iteration (the inner budget does NOT accumulate across dup-retries). Comment the counting in code.

## 4. Ambiguity #4 — does CommitStaged PRINT the success result, or just return Result? (RESOLVED → it prints, per §3 + work-item contract)

- Work-item contract: *"print [short] subject + `git diff-tree --no-commit-id --name-status -r NEW`
  (FR42); return Result{CommitSHA,Subject,Message}"*.
- `decisions.md §3` (★ THE core contract ★): *"print [short] subject; git diff-tree --name-status;
  return NEW_SHA"*.
- Appendix D porting map: *"`git diff-tree --name-status` success print | `main.go`"* — this
  CONFLICTS with §3 (an internal inconsistency). Per the task's own RESEARCH NOTE, §3 governs.

**Resolution (§3 + work-item contract govern):** CommitStaged prints the FR42 success block via
the injected `*ui.Output` (Resultf → stdout: `[<short-sha>] <subject>\n` then the diff-tree lines)
AND returns `Result`. This requires `*ui.Output` in `Deps` (needed anyway for Rescue) and a new
minimal git method `git.DiffTreeNameStatus(sha)` (runs `git diff-tree --no-commit-id --name-status
-r <sha>`, returns the raw stdout). The CLI task (P1.M7.T2) must NOT re-print the diff-tree
(CommitStaged owns the success print); pkg/stagehand.GenerateCommit delegates to CommitStaged and
returns Result. The `<short-sha>` is the first 7 hex chars of NEW (matches `git rev-parse --short`
minimum); guard `len>7`. NOTE: this SUPERSEDES Appendix D's "main.go" assignment — flag in code.

(Defensible alternative, NOT chosen: return-only + CLI prints per Appendix D. Rejected because the
work-item contract is the primary authority and explicitly assigns the print to CommitStaged, and
`*ui.Output` is already injected for the failure paths.)

## 5. Ambiguity #5 — error model & the signal/timeout double-rescue race (RESOLVED)

CommitStaged RETURNS sentinel errors (it does NOT `os.Exit` — that keeps it testable; the CLI
P1.M7.T2 maps them via `errors.Is` to exit codes). Define three package-level sentinels in
generate.go:

- `ErrNothingToCommit` (diff=="") → CLI maps to `ui.ExitNothingToCommit` (2). NO Rescue (no snapshot).
- `ErrRescue` (timeout / agent-error / parse-fail-after-inner / dup-exhaustion / post-snapshot git
  error / CommitTree error) → CLI maps to `ui.ExitRescue` (3). Rescue(out,tree,parent,candidate) is
  rendered BEFORE returning.
- `ErrHeadMoved` (UpdateRefCAS failed = HEAD moved) → CLI maps to `ui.ExitError` (1). Prints the
  §13.5 head-moved message + manual recovery (via Progressf/stderr); NEVER force, NEVER Rescue.

**Candidate message for Rescue:** pass `candidate=msg` ONLY on dup-exhaustion (a valid message that
duplicated a recent subject). Pass `candidate=""` on timeout/agent-error/parse-fail (no valid msg)
and on post-snapshot git errors. (Matches contract: "If outer exhausted→Rescue(candidate=msg)".)

**WriteTree error (FR8):** returned directly (NOT ErrRescue — WriteTree failing means NO tree was
created, so there is nothing to rescue). The git layer's error already mentions "conflict". CLI exit 1.

**The double-rescue race (timeout vs signal):** the executor returns `*provider.TimeoutError`
(wraps `context.DeadlineExceeded`) on a deadline, `*provider.AgentError` on a non-zero exit, and the
BARE `context.Canceled` when the context is cancelled (the signal path). CommitStaged's Run-error
branch therefore:
- `errors.Is(err, context.Canceled)` ⇒ the signal handler already renders Rescue + os.Exit(3); return
  `ErrRescue` WITHOUT rendering Rescue (avoids a double rescue block).
- otherwise (timeout/agent-error) ⇒ `Rescue(out, tree, parent, "")` then return `ErrRescue`.

(Both paths end at exit 3; the distinction is only whether THIS goroutine renders the block.) The
context CommitStaged hands to Run is `context.WithTimeout(ctx, cfg.Timeout)`; its `cancel` func is
the SAME cancel threaded into `installSignalHandler`, so signal-cancel and timeout-deadline collapse
onto one `ctx.Done()` observed by the executor's group-kill. Guard `cfg.Timeout>0` before WithTimeout
(the config default 120s keeps it positive; a zero/negative would otherwise fire immediately).

## 6. The Deps struct + consumer-side interfaces (the "all stubbable" contract)

`CommitStaged(ctx, deps)` takes a single `Deps` value so every collaborator is injectable. Define
TWO consumer-side interfaces in `internal/generate` (Go duck-typing — `*git.Git` and
`*provider.Executor` satisfy them structurally, so NO existing type is modified for the interfaces):

```go
type gitClient interface {
    StagedDiff(git.DiffSettings) (string, error)
    RevParseHEAD() (string, bool, error)
    WriteTree() (string, error)
    CommitTree(parent, msg, tree string) (string, error)
    UpdateRefCAS(ref, newSHA, expected string) error
    CommitCount() (int, error)
    RecentMessages(n int) (string, error)   // makes gitClient satisfy prompt.HistoryReader too
    RecentSubjects(n int) ([]string, error)
    DiffTreeNameStatus(sha string) (string, error)   // NEW — added to *git.Git
}
type runner interface {
    Run(ctx, provider.Manifest, model, providerName, sys, payload string) (string, error)
    Parse(string, provider.Manifest) (string, bool)   // NEW seam on *provider.Executor
}
type Deps struct {
    Git      gitClient
    Runner   runner
    Manifest provider.Manifest
    Config   config.Config
    Output   *ui.Output          // for Rescue + the FR42 success print
}
type Result struct{ CommitSHA, Subject, Message string }
```

Because `gitClient` embeds `CommitCount`+`RecentMessages`, a `gitClient` value can be passed straight
to `prompt.FetchExamples(g, prompt.DefaultExampleCount)` (FetchExamples takes `prompt.HistoryReader`).

## 7. The ordered control flow (the binding spec)

```
CommitStaged(ctx, deps):
  cfg, out := deps.Config, deps.Output
  diff, err := deps.Git.StagedDiff(git.DiffSettings{MaxMdLines:cfg.MaxMdLines, MaxDiffBytes:cfg.MaxDiffBytes})
  if err != nil → return Result{}, err                     // pre-snapshot git failure (exit 1)
  if diff == "" → return Result{}, ErrNothingToCommit       // exit 2; NO rescue
  parentSHA, hasParent, err := deps.Git.RevParseHEAD()
  if err != nil → return Result{}, err
  treeSHA, err := deps.Git.WriteTree()
  if err != nil → return Result{}, err                      // FR8 conflict; NO rescue (no tree)
  // post-snapshot: from here a dangling tree exists → failures Rescue.
  runCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)   // (guard Timeout>0)
  defer cancel()
  installSignalHandler(cancel, func(){ if treeSHA!="" { Rescue(out, treeSHA, parentSHA, "") } })
  count, err := deps.Git.CommitCount()
  if err != nil → Rescue(out,treeSHA,parentSHA,""); return Result{}, ErrRescue
  examples, hasMultiline, err := prompt.FetchExamples(deps.Git, prompt.DefaultExampleCount)
  if err != nil → Rescue(...,""); return Result{}, ErrRescue
  sys := prompt.BuildSystemPrompt(examples, hasMultiline, count<=1, cfg.SubjectTargetChars)
  subjects, err := deps.Git.RecentSubjects(50)
  if err != nil → Rescue(...,""); return Result{}, ErrRescue
  const instruction = "Generate a commit message for these changes:"
  rejected := []string{}
  var msg string; var ok, committed bool
outer:
  for dupAttempt := 0; dupAttempt <= cfg.MaxDuplicateRetries; dupAttempt++ {
      instr := instruction                              // fresh each outer iter
      ok = false
      for parseAttempt := 1; parseAttempt <= 2; parseAttempt++ {
          payload := prompt.AssemblePayload(diff, instr, rejected)
          stdout, runErr := deps.Runner.Run(runCtx, deps.Manifest, cfg.Model, cfg.Provider, sys, payload)
          if runErr != nil {
              if errors.Is(runErr, context.Canceled) { return Result{}, ErrRescue } // signal path
              Rescue(out, treeSHA, parentSHA, ""); return Result{}, ErrRescue
          }
          msg, ok = deps.Runner.Parse(stdout, deps.Manifest)
          if ok { break }
          if deps.Manifest.RetryInstruction != "" { instr = deps.Manifest.RetryInstruction }  // §2
      }
      if !ok { Rescue(out, treeSHA, parentSHA, ""); return Result{}, ErrRescue }
      subject := firstLine(msg)
      if !isDuplicate(subject, subjects) { committed = true; break outer }   // → COMMIT
      rejected = append(rejected, subject)
  }
  if !committed { Rescue(out, treeSHA, parentSHA, msg); return Result{}, ErrRescue } // dup-exhaust
  // COMMIT
  newSHA, err := deps.Git.CommitTree(parentSHA, msg, treeSHA)   // omits -p when parentSHA==""
  if err != nil { Rescue(out, treeSHA, parentSHA, ""); return Result{}, ErrRescue }
  restoreSignalHandler()                                        // BEFORE UpdateRefCAS (ref `trap - INT TERM`)
  if err := deps.Git.UpdateRefCAS("HEAD", newSHA, parentSHA); err != nil {   // CAS; root: no expected
      // §13.5 head-moved message + manual recovery (Progressf/stderr); NEVER force, NEVER Rescue
      out.Progressf("HEAD moved while generating; aborting to avoid a non-fast-forward.\n")
      out.Progressf("Your generated message was: %s\n", msg)
      out.Progressf("To commit the snapshot manually:\n  git commit-tree %s -m %q %s | xargs git update-ref HEAD\n",
          parentFlag(parentSHA), msg, treeSHA)        // omit -p when root
      return Result{}, ErrHeadMoved
  }
  short := newSHA; if len(short) > 7 { short = short[:7] }
  out.Resultf("[%s] %s\n", short, firstLine(msg))              // FR42 (stdout)
  if dt, derr := deps.Git.DiffTreeNameStatus(newSHA); derr == nil { out.Resultf("%s", dt) }
  return Result{CommitSHA: newSHA, Subject: firstLine(msg), Message: msg}, nil
```

## 8. Test strategy for THIS task (generate_test.go, white-box `package generate`)

The interfaces make CommitStaged fully stubbable WITHOUT real git or a real agent. Define a
`stubGit` (implements gitClient with canned returns + call recording) and reuse the shipped
`stubRunner` shape (Run returns canned stdout; Parse returns canned (msg,ok)). Cover:

- happy path (unique subject first try) → Result, no rescue, CommitTree+UpdateRefCAS called once.
- dup-retry-then-success (1st subject dup, 2nd unique) → rejected grows, 2nd commits.
- parse-retry-then-success (1st Parse ok=false, corrective payload rebuilt, 2nd ok=true).
- parse-fail-after-inner (both Parse ok=false) → Rescue(candidate="") + ErrRescue.
- dup-exhaustion (always dup, MaxDuplicateRetries+1 generations) → Rescue(candidate=last msg) + ErrRescue.
- nothing-to-commit (diff=="") → ErrNothingToCommit, NO Rescue, NO WriteTree.
- WriteTree error → returned directly (no rescue, no ErrRescue).
- timeout (Run returns *provider.TimeoutError) → Rescue + ErrRescue.
- signal-cancel (Run returns context.Canceled) → ErrRescue, Rescue NOT called by CommitStaged.
- head-moved (UpdateRefCAS errors) → §13.5 message + ErrHeadMoved; NEVER force; CommitTree called.
- root commit (RevParseHEAD hasParent=false ⇒ parentSHA="") → CommitTree/UpdateRefCAS called with "".

Integration against a REAL temp repo + the stub agent binary (stubprovider_test.go) is the
SEPARATE task P1.M6.T3.S2 (it has the temp-repo harness); this task ships the stubbable unit tests.

## 9. Scope boundaries (DO NOT)
- Do NOT call `git add`/`AddAll`/`HasStagedChanges` (staging is CLI-only — the v2 seam).
- Do NOT `os.Exit` from CommitStaged (return sentinels; the CLI maps exit codes).
- Do NOT modify parse.go/parse_test.go/manifest.go (add ONLY `(*Executor).Parse` to executor.go).
- Do NOT force update-ref; never use the 1-arg form except root; never call `git commit`.
- Do NOT create generate.go's package doc on a sibling — generate.go OWNS `// Package generate`
  (dedupe.go/rescue.go/signal.go already defer to it via a plain `package generate` line).

## 10. Confidence
9/10. Every dependency signature is verified in the shipped code; the five ambiguities are
resolved with explicit citations; the interfaces make the unit tests hermetic. The only residual
risk is the §3-vs-Appendix-D print-responsibility tension (resolved in §4 by honoring §3).
