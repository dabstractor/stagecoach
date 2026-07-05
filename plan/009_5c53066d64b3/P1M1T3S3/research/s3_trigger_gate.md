# P1.M1.T3.S3 Research — FR-T1 trigger gate wiring into CommitStaged

## §1. The edit site (verified against the live `internal/generate/generate.go`)

Exact line anchors (verified by grep):
- **L8** — import block (`context, errors, fmt, strings` + internal pkgs; NO `os`, NO `time`).
- **L211** — `resolved := deps.Manifest.Resolve()` — function-scoped; REUSABLE in the gate (no re-decl).
- **L212** — `retryInstr := *resolved.RetryInstruction`.
- **L226** — `for attempt := 0; attempt <= cfg.MaxDuplicateRetries; attempt++ {` (the one-shot loop).
- **L228** — `payload := prompt.BuildUserPayload(diff, cfg.Context, rejected)` — **LOOP-SCOPED** (`:=`).
- **L246, L252** — the IN-LOOP rescue returns (timeout/cancel). **UNCHANGED** (not the gate site).
- **L287–292** — `if !success { return Result{}, &RescueError{Kind: ErrRescue, TreeSHA: treeSHA, ParentSHA: parentSHA, Candidate: candidate, Cause: lastCause} }` — the gate insertion point.

### In-scope variables at L287 (verified)
`ctx, deps, cfg, sysPrompt, diff, treeSHA, parentSHA, isUnborn, recent, resolved, retryInstr, msgModel,
msgReasoning, rejected, candidate, parseFail, lastCause, msg, success`. **`payload` is NOT in scope** (loop-
local at L228). **`spec` is NOT in scope** (loop-local at L237).

## §2. The seam contracts (verified)

- **`multiturn.Run`** (same package `generate`; multiturn.go):
  `func Run(ctx, deps Deps, cfg config.Config, manifest provider.Manifest, sysPrompt, payload, msgModel, msgReasoning string) (msg string, ok bool, cause error)`.
  Called UNQUALIFIED (`Run(...)`) from generate.go — same package. Returns raw cause (NOT *RescueError).
- **`chunkPayload`** (same package; multiturn.go, unexported but accessible): `func chunkPayload(payload string, chunkTokens int) []chunk`. `N = len(chunkPayload(payload, cfg.MultiTurnChunkTokens))` — the chunk count for the progress line. Pure string math; safe to call twice (Run calls it again internally — deterministic).
- **`git.EstimateTokens(s string) int`** (internal/git/tokens.go:25) — `ceil(runes/4)`. First DIRECT call on `payload` in this file (it's already passed as a fn-value to MessageReserveTokens at L174).
- **`resolved.SessionMode`** is `*string` (manifest.go:66). `Resolve()` defaults nil → strPtr(""). `Validate()` enforces "", "append". Gate must NIL-CHECK: `resolved.SessionMode != nil && *resolved.SessionMode == "append"`.
- **Config** (config.go): `MultiTurnFallback bool` (default true), `MultiTurnChunkTokens int` (default 32000), `Timeout time.Duration` (default 120s).
- **`FinalizeMessage(msg, cfg)` / `ExtractSubject(m)` / `IsDuplicate(subj, recent)`** — all SAME-package (`finalize.go:37` / `dedupe.go:19` / `dedupe.go:46`). No import needed.
- **`signal.SetCandidate(m)`** — same-package usage as one-shot L273 (keeps §18.3 rescue candidate current).
- **`deps.Verbose.VerboseWarn(msg)`** (ui/verbose.go:103) — nil-safe generic one-liner: `fmt.Fprintln(v.w, "DEBUG: "+msg)`. No dedicated "trigger" method exists; VerboseWarn is the closest fit (no ui-package change needed).

## §3. The trigger gate (FR-T1 a–d) — all conditions

FR-T1: multi-turn activates ONLY when ALL hold:
- **(a)** one-shot exhausted its retry loop on empty/unparseable output — **already true at `!success`** (the gate is INSIDE `if !success`).
- **(b)** `payload_tokens > multi_turn_chunk_tokens` → `git.EstimateTokens(payload) > cfg.MultiTurnChunkTokens`.
- **(c)** `multi_turn_fallback` enabled → `cfg.MultiTurnFallback` (default true).
- **(d)** resolved manifest declares `session_mode = "append"` → `resolved.SessionMode != nil && *resolved.SessionMode == "append"`.

If ANY fails → fall through to the existing rescue return (byte-identical, FR-T7).

## §4. Key decisions

- **D1 (payload scope — hoist, do not recompute):** hoist `var payload string` BEFORE the loop (after the
  `var rejected`/`candidate`/etc. block, ~L219); change L228 `payload :=` → `payload =`. The last-built
  payload survives to L287. This honors the contract's "do not recompute" (the multi-turn path reads the
  existing payload variable, NOT a fresh BuildUserPayload call). The hoist is a PURE refactor: the loop's
  payload construction is unchanged except `:=`→`=`. [Research §1b option 1, confirmed in the contract.]

- **D2 (FR-T12 — the captured-payload variable, NOT a fresh un-water-filled capture):** the contract is
  EXPLICIT: multi-turn uses the existing `payload` variable. FR-T2/FR-T12 say "no token_limit water-fill /
  use the UNTRUNCATED payload," but re-capturing the diff with TokenLimit=0 is a "recompute" the contract
  forbids. The contract resolves the tension by treating the captured `payload` variable as the payload
  multi-turn delivers, and NOT re-applying token_limit in the multi-turn path. Rationale: (1) the primary
  reliability mechanism is chunkPayload's per-request chunking, which works on any payload; (2) if
  TokenLimit truncated the diff, multi-turn still helps by chunking what remains; (3) re-capturing would
  deviate from the contract. The contract is the work-item authority. **Follow it: pass `payload` as-is.**

- **D3 (finalize BEFORE dedupe — one-shot parity):** the contract's pseudocode says
  "IsDuplicate(ExtractSubject(msg2), recent)" (un-finalized), but the one-shot path finalizes BEFORE dedupe
  (generate.go comment: "template BEFORE dedupe (§9.7 judges the final subject)"). To avoid the template-
  duplicate-slip bug (msg2="feat: x" templates to "feat: x (#1)" matching history, but un-finalized dedupe
  misses it), FINALIZE msg2 FIRST, then dedupe on the finalized subject. "Run the EXISTING duplicate check"
  = do what the one-shot path does (finalize → dedupe). On duplicate, Candidate = the FINALIZED message
  (one-shot sets `candidate = m` post-finalize). On parse-fail/cause, Candidate = msg2 RAW (one-shot sets
  `candidate = m` = raw parse output on `!ok`).

- **D4 (control flow — nested gate + re-check):** structure is `if !success { if <gate> { <multi-turn> }
  if !success { return &RescueError{...} } }`. The SECOND `if !success` lets multi-turn success SKIP the
  rescue return while preserving the rescue return literal BYTE-IDENTICAL on fall-through (FR-T7). No
  goto/labeled-break needed.

- **D5 (progress line — direct stderr write):** FR-T5 wants a USER-VISIBLE progress line (not --verbose-
  gated). `Deps.Progress` is a `func()` callback (no message param), so it can't carry the dynamic
  turn-count/budget. The contract allows a direct stderr write: `fmt.Fprintf(os.Stderr, "↳ falling back to
  multi-turn: %d turns, ~%dm total\n", turns, totalMin)`. `turns = N+1` where `N = len(chunkPayload(...))`;
  `totalMin = int((cfg.Timeout * time.Duration(turns)).Minutes())`, floored at 1. Adds `os` + `time` imports.

- **D6 (verbose trigger — VerboseWarn):** `deps.Verbose.VerboseWarn("one-shot exhausted → multi-turn
  fallback")` — nil-safe, no ui-package change. FR-T11's per-turn payload/stdout/stderr is already emitted
  by provider.Execute (research-tests-ui.md §1 item 10) — Run calls Execute per turn, so per-turn verbose
  is FREE (no extra wiring here).

- **D7 (candidate/lastCause mapping — one-shot parity):**
  - cause != nil (turn error/timeout): `lastCause = cause`; `candidate = msg2` if msg2 != "".
  - ok2 == false (final parse empty, cause == nil): `lastCause` UNCHANGED (one-shot's last); `candidate = msg2` if msg2 != "".
  - duplicate: `candidate = finalMsg` (finalized); `lastCause` UNCHANGED.
  - success: `msg = finalMsg`; `success = true`.
  The rescue return struct literal is UNCHANGED (same fields, same order) → FR-T7 byte-identical.

## §5. Test design (focused — exhaustive truth table is S4)

The stub (NewScript) is call-indexed and CLAMPS to the last line after exhaustion. The one-shot path
consumes call 1; multi-turn consumes calls 2..(N+2). Recipe for the happy wiring test:
- `cfg.MaxDuplicateRetries = 0` (one-shot: 1 attempt), `cfg.MultiTurnChunkTokens` small (e.g. 2–4 → N≥2).
- Stage a small file; manifest with `SessionMode = &"append"`.
- NewScript: `["", "ok", "ok", "feat: multi-turn success"]` — call 1 (one-shot) gets "" (parse-fail →
  exhaust); multi-turn turns get "ok" then "feat: multi-turn success" (clamped → final turn gets it).
- Assert: commit lands, subject == "feat: multi-turn success".

Focused tests (3–4): multi-turn success commits / non-append skip → rescue / small-payload skip → rescue
(default chunkTokens=32000, small diff). Exhaustive 4-condition truth table + token_limit non-interaction
= S4 (P1.M1.T3.S4). T4 (P1.M1.T4) owns the integration matrix.

## §6. Non-overlap with the parallel subtask (P1.M1.T3.S2)

S2 (Implementing, in parallel) MODIFIES `internal/generate/multiturn.go` + `multiturn_test.go` (adds Run +
newSessionID + constants). S3 MODIFIES `internal/generate/generate.go` + `generate_test.go` (the trigger
gate + payload hoist + multi-turn branch + focused tests). **Distinct files** — no conflict. S3 CONSUMES
S2's `Run` (same-package call). If S2 has not landed when S3 runs, `Run` is undefined → `go build` fails —
S3 MUST wait for S2 (the dependency is hard). The PRP's edit anchors are all in generate.go (independent of
multiturn.go's state except for the `Run` symbol).

## §7. Imports to add to generate.go

```go
"os"
"time"
```
gofmt-sorted into the stdlib block: `context, errors, fmt, os, strings, time`. NO new internal-package
imports (Run/chunkPayload/FinalizeMessage/etc. are same-package; git/config/provider/prompt already imported).
