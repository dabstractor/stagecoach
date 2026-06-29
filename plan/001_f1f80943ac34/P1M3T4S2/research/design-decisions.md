# Design Decisions — P1.M3.T4.S2 (CommitStaged orchestrator)

> Verified against the working tree (2026-06-29). Every signature/behavior cross-checked against the
> real source. This file captures the NON-OBVIOUS load-bearing decisions; the PRP carries the full
> blueprint. Read alongside the PRP's "Known Gotchas" + "Implementation Blueprint".

## §0 — Scope (the boundary that protects v2)

This subtask delivers **ONLY** `internal/generate/generate.go` (the `CommitStaged` orchestrator + its
types) and `internal/generate/generate_test.go` (integration tests via the stub provider). It CONSUMES
(read-only) every upstream layer: `internal/git` (P1.M1.T2/T3), `internal/provider` (P1.M2.T1–T6),
`internal/prompt` (P1.M3.T1), `internal/generate/{dedupe,rescue}.go` (P1.M3.T2/T3), `internal/config`
(P1.M1.T4), and `internal/stubtest` (P1.M3.T4.S1 — the stub provider + helper). It does NOT touch the
CLI (P1.M4), signals (P1.M4.T2), the public API wrapper (P1.M3.T5), or property tests (P1.M5.T1). It
adds NO dependency (imports only already-present internal packages + stdlib). `go mod tidy` is a no-op.

The orchestrator **NEVER calls `git add`** (PRD §11.3 — "the core is `commitStaged(ctx, cfg)` that
assumes the index is already in the desired state"). Auto-stage-all is the CLI layer's job (P1.M4.T1.S2).
This is the boundary that makes v2's `for each partition { reset+stage; commitStaged() }` trivial.

## §1 — Why `Deps` + `Config` as separate params (dependency injection)

Signature: `CommitStaged(ctx context.Context, deps Deps, cfg Config) (Result, error)`.

`Deps` carries the RUNTIME collaborators that vary by environment/test:
```go
type Deps struct {
    Git      git.Git            // the git boundary (real *gitRunner in prod; test can substitute)
    Manifest provider.Manifest  // the RESOLVED provider manifest to render+execute
}
```
`Config` carries the resolved generation TUNING (Timeout, MaxDuplicateRetries, SubjectTargetChars,
MaxDiffBytes, MaxMdLines, Model, Provider) — pure data, already resolved by P1.M1.T4.

**Why `Manifest` is in `Deps`, not resolved from `cfg.Provider` inside the orchestrator:** the
integration tests drive `CommitStaged` with a **test-only** manifest (`stubtest.Manifest(...)`) whose
`Command` points at the built stub binary and whose `Env` knobs control its behavior (P1.M3.T4.S1
design-decisions §2). Injecting the manifest lets the test bypass the registry entirely — no `$PATH`
probe, no real agent. In production, the CLI (P1.M4.T1) resolves the manifest via the registry and
hands it to `Deps`. The orchestrator stays registry-agnostic and therefore unit-testable.

`Manifest` arrives UNRESOLVED (as the registry stores it). `CommitStaged` calls `deps.Manifest.Validate()`
+ `deps.Manifest.Resolve()` before the first `Render` (mirrors `provider.Render`'s own guard, and
`ParseOutput`'s). Resolve on a COPY (caller's manifest untouched).

## §2 — `Result` fields (incl. `Changes`, the home for step 9)

```go
type Result struct {
    CommitSHA string             // NEW_SHA from commit-tree (the published commit)
    Subject   string             // ExtractSubject(Message) — for the "[<sha>] <subject>" report (FR42)
    Message   string             // the full commit message (subject [+ body])
    Provider  string             // deps.Manifest.Name (the concrete provider used)
    Model     string             // resolved model (cfg.Model or manifest default_model)
    Changes   []git.FileChange   // DiffTree(newSHA, isUnborn) — step 9 output, for FR42's diff-tree listing
}
```
The contract lists `{CommitSHA, Subject, Message, Provider, Model}`. **`Changes` is added** as the
natural home for step 9's `DiffTree` output: FR42 requires the CLI to print the "what landed" file
listing, and carrying it in `Result` means the CLI (and the public API wrapper P1.M3.T5) need NOT
re-query git. This is a strictly additive, non-conflicting extension (P1.M3.T5 wraps this Result).

## §3 — Typed errors: sentinels + context-carrying wrappers

The CLI layer (P1.M4.T1/T3) maps errors to exit codes (PRD §15.4):
`ErrNothingToCommit → 2`, `ErrRescue → 3`, `ErrTimeout → 124`, `ErrCASFailed → 1`.
The orchestrator returns errors that are BOTH `errors.Is`-able (so the CLI picks the exit code) AND
carry the context the CLI needs to render the recovery message (PRD §18.3 / §13.5).

```go
var (
    ErrNothingToCommit = errors.New("stagehand: nothing staged to commit")
    ErrTimeout         = errors.New("stagehand: generation timed out")
    ErrRescue          = errors.New("stagehand: commit generation failed after retries")
)
// ErrCASFailed is git.ErrCASFailed (already a typed sentinel wrapping the update-ref exit code).
// Re-exported from internal/git so the CLI imports a single package; detected via errors.Is.
var ErrCASFailed = git.ErrCASFailed
```

Two context-carrying error types wrap the sentinels (the CLI calls `errors.As` to read the fields):

```go
// RescueError carries the post-snapshot context for FormatRescue (§18.3 / FR43–FR44).
// Returned for ErrTimeout AND ErrRescue (both lead to the rescue message; exit code differs).
type RescueError struct {
    Kind      error  // ErrTimeout or ErrRescue — enables errors.Is(err, ErrTimeout|ErrRescue)
    TreeSHA   string // the frozen snapshot (always set — the rescue only fires AFTER WriteTree)
    ParentSHA string // "" on a root commit (FormatRescue omits -p)
    Candidate string // the last generated message ("" if none) — FormatRescue appends the candidate note
    Cause     error  // underlying: context.DeadlineExceeded / *exec.ExitError / nil — for verbose/diagnostics
}
func (e *RescueError) Error() string  // human-readable; names Kind + reason
func (e *RescueError) Unwrap() error  // returns e.Kind (so errors.Is(err, ErrTimeout|ErrRescue) works)

// CASError carries the §13.5 "HEAD moved" context. The orchestrator RE-READS HEAD via RevParseHEAD
// on CAS failure (per internal/git/git.go's ErrCASFailed docstring, decision D5) to get Actual.
type CASError struct {
    TreeSHA  string // the snapshot tree (for manual recovery)
    Expected string // the parentSHA captured at step 1
    Actual   string // HEAD re-read after the CAS failed (the concurrent commit's SHA)
    Message  string // the generated message (for the manual commit-tree command)
}
func (e *CASError) Error() string  // the §13.5 message body
func (e *CASError) Unwrap() error  // returns git.ErrCASFailed (so errors.Is(err, ErrCASFailed) works)
```

The CLI does: `errors.Is(err, ErrNothingToCommit)` → exit 2; `errors.As(err, &rescueErr)` →
`FormatRescue(...)` + exit (124 if `errors.Is(err, ErrTimeout)` else 3); `errors.As(err, &casErr)` →
print `casErr.Error()` (the §13.5 message) + exit 1. The public API (P1.M3.T5) re-exports these.

## §4 — Unified generation loop (FR29 parse-retry + FR32 dup-retry share one bounded counter)

The contract describes ONE loop: "for attempt in 0..maxRetries: BuildUserPayload(payload, rejectedList);
Render; Execute; ParseOutput; if !ok → retry with retry_instruction (FR29); ExtractSubject; IsDuplicate?
→ append to rejectedList, continue; else break". So FR29 and FR32 are **unified into one loop** bounded
by `cfg.MaxDuplicateRetries` (default 3). Interpretation: **attempt 0..maxRetries INCLUSIVE =
maxRetries+1 total attempts** (1 initial + up to `maxRetries` retries). With default 3 → 4 attempts.

Two distinct CORRECTIVE SIGNALS, selected by the previous attempt's failure mode:
- **Parse failure** (`!ok`, FR29): the NEXT attempt's user payload is prepended with the manifest's
  `RetryInstruction` (resolved default: "Output ONLY the commit message. No preamble, no markdown, no
  quotes."). Tracked via a `parseFail bool` flag.
- **Duplicate** (`IsDuplicate==true`, FR32): the matched subject is appended to `rejected` and the
  NEXT attempt's payload grows the §17.3 rejection block (via `prompt.BuildUserPayload(diff, rejected)`).

Both consume an attempt. After the loop exhausts with no success → `&RescueError{Kind: ErrRescue, ...}`.

The SYSTEM PROMPT is built ONCE before the loop (it does not change per attempt). Only the USER PAYLOAD
is rebuilt each attempt (it carries the corrective signal). `recent` subjects for `IsDuplicate` are
fetched ONCE before the loop (they cannot change — the repo isn't committed until step 8).

## §5 — Execute error handling: timeout → immediate rescue; non-zero exit → fall through to Parse

`provider.Execute` returns (stdout, stderr, err) with this contract (executor.go): timeout →
`context.DeadlineExceeded`; parent/signal cancel → `context.Canceled`; non-zero exit → wrapped
`*exec.ExitError` (stdout still captured); start miss → wrapped LookPath error.

The orchestrator branches:
- `errors.Is(execErr, context.DeadlineExceeded)` → **return `&RescueError{Kind: ErrTimeout}` IMMEDIATELY**.
  Rationale (FR25 / §13.5): the agent was KILLED mid-generation; retrying would just time out again
  (waste 120s × retries). No retry. Exit 124.
- `errors.Is(execErr, context.Canceled)` → return `&RescueError{Kind: ErrRescue}` (interrupted; full
  signal handling is P1.M4.T2, but the orchestrator must not spin on a cancelled context).
- **Non-zero exit (`*exec.ExitError`)** → do NOT short-circuit. Fall through to `ParseOutput`: the
  stdout captured before the crash may still be a valid (partial) message. If `ok==false` → it becomes
  a parse-failure retry (FR29); if `ok==true` → proceed to dedupe. Record `execErr` as the candidate
  `Cause` for the eventual rescue if all retries fail. This gracefully handles "agent exited non-zero
  but emitted a usable message" AND "agent crashed, empty stdout → retry".

## §6 — Nothing-to-commit gate: StagedDiff emptiness (contract step 2), NOT HasStagedChanges

The contract pipeline step 2 is literally "StagedDiff → payload (if empty, return nothing-to-commit)".
The orchestrator captures `diff := git.StagedDiff(...)`; if `diff == ""` → return `ErrNothingToCommit`.
This is MORE correct than gating on `HasStagedChanges`: it reflects what the MODEL will actually see
(after markdown caps + lock/snap/map/vendor excludes). Edge case it correctly handles: HasStagedChanges
could be `true` (staged files exist) while `StagedDiff==""` (every staged file is excluded noise) →
nothing meaningful to commit → `ErrNothingToCommit`. `git.StagedDiff` is documented to return `""` with
NO error on a nothing-staged index, so the gate is safe. (`HasStagedChanges` is NOT called — the CLI
layer's auto-stage logic owns that; `CommitStaged` assumes the index is already staged per §11.3.)

## §7 — The exact step ordering (atomicity invariant, PRD §18.1)

```
 1. parentSHA, isUnborn = RevParseHEAD(ctx)
 2. diff = StagedDiff(ctx, opts)            ── if diff=="" → return ErrNothingToCommit
 3. treeSHA = WriteTree(ctx)                ── if err → return err (merge conflicts; abort exit 1)
    [snapshot taken — HEAD & index frozen w.r.t. this commit from here]
 4. sysPrompt = mature OR fallback (based on CommitCount; RecentMessages/DetectMultiline if mature)
    recent = isUnborn ? nil : RecentSubjects(ctx, 50)        [for IsDuplicate; fetched once]
 5. LOOP attempt 0..cfg.MaxDuplicateRetries:
       payload = BuildUserPayload(diff, rejected)
       if previous was parse-fail: payload = RetryInstruction + "\n\n" + payload   (FR29)
       spec, _ = deps.Manifest.Render(cfg.Model, cfg.Provider, sysPrompt, payload)
       out, _, execErr = provider.Execute(ctx, *spec, cfg.Timeout)
       [execErr branch — §5: timeout→ErrTimeout rescue; cancel→ErrRescue rescue; exit→fall through]
       msg, ok, _ = provider.ParseOutput(out, deps.Manifest)
       if !ok: parseFail=true; candidate=msg; continue      (FR29 retry; consumes an attempt)
       subject = ExtractSubject(msg)
       if IsDuplicate(subject, recent): rejected=append(rejected, subject); parseFail=false; candidate=msg; continue  (FR32)
       else: BREAK (success — msg is the message)
    [loop exhausted → return &RescueError{Kind: ErrRescue, TreeSHA, ParentSHA, Candidate, Cause}]
 7. newSHA, err = CommitTree(ctx, treeSHA, parents, msg)     ── parents = isUnborn?nil:[]string{parentSHA}
 8. err = UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld)     ── expectedOld = isUnborn?zeros:parentSHA
       if err (CAS fail): actual,_=RevParseHEAD(ctx); return &CASError{TreeSHA, Expected:parentSHA, Actual, Message:msg}
 9. changes = DiffTree(ctx, newSHA, isUnborn)                ── "what landed" (FR42)
10. return Result{CommitSHA:newSHA, Subject, Message:msg, Provider:deps.Manifest.Name, Model, Changes}, nil
```
The invariant holds: steps 1–3 mutate no ref; 4–6 mutate nothing external; 7 creates a DANGLING commit
object (no ref move); 8 is the ONLY ref mutation (CAS — fails cleanly if HEAD moved). Any failure
before/including 8 leaves the repo byte-for-byte unchanged (modulo a harmless dangling tree/commit).

## §8 — CAS failure: re-read HEAD, never force

`git.UpdateRefCAS` returns a wrapped `git.ErrCASFailed` (carrying the exit code + stderr) when the
compare-and-swap did not match (HEAD moved since the snapshot, §13.5). The orchestrator MUST:
1. NOT retry / NOT force-update (FR41 — "Do not force-update").
2. Return `&CASError{...}` so the CLI prints the §13.5 message + exits 1.
3. RE-READ HEAD via `RevParseHEAD(ctx)` to obtain `Actual` (the concurrent commit's SHA) — per the
   `ErrCASFailed` docstring (decision D5: the actual SHA is deliberately NOT captured inside
   UpdateRefCAS; the orchestrator reads it when it observes the error). The CAS error from git carries
   the exit code/stderr but NOT the new HEAD value.

Edge: if the re-read `RevParseHEAD` itself errors (rare), fall back to `Actual=""` and still return the
CASError (the message degrades gracefully; correctness of HEAD-unchanged is already guaranteed by git's
CAS semantics — the update-ref did not move HEAD).

## §9 — Root commit (unborn repo) handling

When `isUnborn==true` (RevParseHEAD step 1): `parentSHA=""`. Then:
- `recent = nil` (no subjects to dedupe against — `IsDuplicate` returns false for nil recent, vacuously
  correct). `RecentSubjects` is NOT called (the interface docstring says "callers must short-circuit
  when isUnborn" — `RecentSubjects` returns `(nil,nil)` on unborn anyway, but we avoid the call).
- `sysPrompt = BuildFallbackPrompt(cfg.SubjectTargetChars)` (§17.2 — a repo with 0 commits is ≤1 →
  conventional-commit fallback). `CommitCount` is NOT strictly needed for the unborn case (we know it's
  0), but for the 1-commit case we need it to pick fallback vs mature. So: if `isUnborn` → fallback;
  else fetch `CommitCount`; if `<=1` → fallback; else mature.
- `parents = nil` for `CommitTree` (no `-p` → root commit; mirrors git.CommitTree's root semantics).
- `expectedOld = all-zeros` (40 × '0') for `UpdateRefCAS` (the CAS succeeds only if HEAD is truly
  unborn — matches `TestUpdateRefCAS_RootCommit`).
- `DiffTree(newSHA, isRoot=true)` (the `--root` flag; without it a root commit yields empty output —
  verified, see git.DiffTree docstring).

## §10 — Test strategy: stub provider + temp git repos (the generate package needs its OWN fixtures)

The integration tests (`internal/generate/generate_test.go`, `package generate`) drive `CommitStaged`
end-to-end with the **real** `provider.Execute` seam but a **stub** agent (`internal/stubtest`,
P1.M3.T4.S1 — FROZEN API), against **real temp git repos** (`git.New(t.TempDir())`). No real LLM, no
network, no API key — deterministic, runs in CI on all OSes (§20.4).

The git package's fixture helpers (`initRepo`, `makeEmptyCommit`, `writeFile`, `stageFile`, `headSHA`)
live in `internal/git/*_test.go` — they are **package-private AND in `_test.go` files**, so they are
NOT importable by `internal/generate`'s tests. Standard Go practice: the generate tests define their
OWN minimal fixtures (copy the ~10-line `initRepo`/`writeFile`/`stageFile`/`headSHA`/`commitRaw`
helpers — they're trivial `exec.Command("git", "-C", dir, ...)` wrappers with `t.Helper()`).

**The five contract scenarios** (all via the stub against a temp repo):
1. **Success** — `stubtest.Manifest(bin, Options{Out:"feat: add login"})`; repo with 1+ commits + a
   staged file; assert `Result.CommitSHA` is a real SHA, `Subject=="feat: add login"`, HEAD moved to
   `CommitSHA`, `git log --format=%B -n1 CommitSHA == "feat: add login"`, `len(Changes)>0`.
2. **Dedupe-retry-then-success** — `stubtest.NewScript(t, bin, []string{"feat: existing","feat: fresh"})`;
   repo whose HEAD subject IS "feat: existing"; assert call 1 is rejected (dup), call 2 succeeds with
   "feat: fresh"; HEAD == the fresh commit. (Exercises FR30/FR32 + the rejection-list growth.)
3. **Parse-fail-rescue** — `stubtest.NewScript(t, bin, []string{"","feat: good"})` with
   `cfg.MaxDuplicateRetries=0` (so the blank → ok=false → loop exhausts → rescue); assert
   `errors.As(err, &RescueError)` + `errors.Is(err, ErrRescue)` + `TreeSHA` set + HEAD UNCHANGED +
   index UNCHANGED (the invariant). (Or a 1-response blank script with retries=3 → same rescue.)
4. **CAS-failure** — stub with `SleepMS:400`; start `CommitStaged` in a goroutine; in the main goroutine
   `time.Sleep(150ms)` (let it snapshot + enter generation), then move HEAD via a raw
   `git commit --allow-empty -m concurrent` (or `update-ref`); wait for `CommitStaged`; assert
   `errors.As(err, &CASError)` + `errors.Is(err, git.ErrCASFailed)` + `Actual == concurrentSHA` +
   HEAD == concurrentSHA (the orchestrator's commit did NOT land). (Exercises §13.5 + D5 re-read.)
5. **Root commit** — unborn repo (`git init` only), stage a file, stub `Options{Out:"chore: init"}`;
   assert success, `Result.CommitSHA` has no parent (`git cat-file -p <sha>` lacks a `parent` line),
   HEAD == CommitSHA, `DiffTree` ran with `isRoot=true` (Changes non-empty).

**Property/invariant tests** (§20.2, light version here — the full set is P1.M5.T1):
- **Idempotent index on failure**: snapshot `git diff --cached --name-only` before + after a rescue
  path; assert byte-identical (no index mutation — CommitStaged never calls `git add`).
- **Atomic HEAD on CAS failure**: `git rev-parse HEAD` unchanged after a CASError.

The stub is the ONLY mock; everything else (git, Render, Execute, ParseOutput, ExtractSubject,
IsDuplicate, FormatRescue types) is the REAL production code path — so these tests also regression-test
the entire upstream pipeline for free.
