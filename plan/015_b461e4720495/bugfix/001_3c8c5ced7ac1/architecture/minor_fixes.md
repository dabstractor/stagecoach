# Minor Fixes: Issues 4, 5, 6

## Issue 4: Token Gate Sub-270 Invariant Violation (FR3j)

### Problem

`internal/git/tokengate.go`: the FR3j closed-loop gate cannot trim below an irreducible floor
(system prompt + numstat skeleton + payload framing + minBodyTokens slivers). For `token_limit`
below ~270, the assembled prompt exceeds the limit. The PRD states the invariant holds "always".

### Key Functions

- `applyWaterFillGate(mdDiffs, nmDiff, skeleton string, tokenLimit, promptReserve int) string`
  — `tokengate.go:135`. The FR3i first-cut water-fill gate.
- `closedLoopGate(mdDiffs, nmDiff, skeleton string, tokenLimit, promptReserve int, measure func(string) int) string`
  — `tokengate.go:195`. The FR3j closed-loop re-trim loop.

### Constants

- `tokenBudgetMargin = 1024` — tokengate.go:48
- `minBodyTokens = 8` — tokengate.go:75
- `maxClosedLoopPasses = 4` — tokengate.go:83
- `closedLoopSlack = 64` — tokengate.go:89

### Degenerate Case

- `tokengate.go:137-138`: `bodyBudget := tokenLimit - skeletonTokens - promptReserve - tokenBudgetMargin`.
  When `bodyBudget <= 0`, `budgetExhausted = true` and each file body is cut to `minBodyTokens` (8 tokens).
- But the irreducible system prompt + skeleton + framing still exceed `token_limit`.
- `closedLoopGate` (tokengate.go:195-216) returns the best attempt after `maxClosedLoopPasses=4`
  even if still over: `return bestDiff // best effort` — so below the floor the invariant is
  **silently violated**.

### Call Sites (git.go)

Three call sites in `internal/git/git.go`:
- StagedDiff: line 1058: `gatedBody := closedLoopGate(mdDiffs, nmDiff, skeleton, opts.TokenLimit, opts.PromptReserveTokens, opts.MeasureAssembled)`
- TreeDiff: line 1583
- WorkingTreeDiff: line 1758

All three are inside `if opts.TokenLimit > 0` branches. At these points, `skeleton` and
`opts.PromptReserveTokens` are available, so the irreducible floor is computable:
`floor = EstimateTokens(skeleton) + opts.PromptReserveTokens + tokenBudgetMargin`.

### MeasureAssembled Wiring

`pkg/stagecoach/stagecoach.go:443-465` — wires `MeasureAssembled` only when `token_limit != 0`.
The callback measures `EstimateTokens(sysPrompt + prompt.BuildUserPayload(gatedDiff, ...))`.

### Preferred Fix: Option (a) — Reject Sub-Floor Limits

Add a floor check at each `opts.TokenLimit > 0` call site in git.go, before calling `closedLoopGate`:

```go
floor := git.EstimateTokens(skeleton) + opts.PromptReserveTokens + git.TokenBudgetMargin
if opts.TokenLimit < floor {
    return "", fmt.Errorf("token_limit %d is below the irreducible prompt floor %d (system prompt + numstat skeleton + framing); raise it to at least %d", opts.TokenLimit, floor, floor)
}
```

To centralize: export a helper from tokengate.go:

```go
// IrreducibleFloor returns the minimum token_limit below which the assembled prompt cannot fit.
func IrreducibleFloor(skeleton string, promptReserve int) int {
    return EstimateTokens(skeleton) + promptReserve + tokenBudgetMargin
}
```

Then check at each call site: `if opts.TokenLimit < IrreducibleFloor(skeleton, opts.PromptReserveTokens)`.

This does NOT change `closedLoopGate`'s signature — the check is at the caller, and the error
propagates through the normal `(string, error)` return of StagedDiff/TreeDiff/WorkingTreeDiff.

**Alternative**: Change `closedLoopGate` to return `(string, error)`. More invasive (3 callers
must handle error). The caller-level check is simpler and equally correct.

### Documentation Impact

`docs/configuration.md:167` describes token_limit's closed-loop guarantee: "a closed-loop guarantee
(§9.1 FR3j) that the payload never exceeds `token_limit`". After this fix, a sub-floor limit errors
instead of silently violating the invariant. The docs section should note the floor rejection.

### Test Files

`internal/git/tokengate_test.go` — existing tests for the water-fill and closed-loop gate.

---

## Issue 5: Doubled "stagecoach:" Prefix in --edit Abort

### Problem

`internal/generate/finalize.go:45`:
```go
var ErrEmptyMessage = errors.New("stagecoach: empty commit message — aborted")
```

`cmd/stagecoach/main.go:67`:
```go
fmt.Fprintf(os.Stderr, "stagecoach: %v\n", err)
```

Result: `stagecoach: stagecoach: empty commit message — aborted` (double prefix).

### Propagation Path

- `finalize.go:118`: `return "", ErrEmptyMessage` from `EditMessage` (--edit path, §9.22 FR-E1)
- `generate.go:486`: `return Result{}, err` (propagates BARE)
- `decompose.go:409`: `return DecomposeResult{}, editErr` (per-concept abort)
- `message.go:254`: `return "", generate.ErrEmptyMessage` (hooks-empty-message guard)
- `exitcode.go:65`: `if errors.Is(err, generate.ErrEmptyMessage) { return Error }` → exit 1

### Fix

Remove the `"stagecoach: "` prefix from the `ErrEmptyMessage` literal at `finalize.go:45`:
```go
var ErrEmptyMessage = errors.New("empty commit message — aborted")
```

Since `main.go:67` always prepends `"stagecoach: "`, the final output becomes:
`stagecoach: empty commit message — aborted` (single prefix, correct).

The sentinel identity is preserved (`errors.Is` still works — it's the same `*errors.errorString`).
Both the --edit path and the hooks-empty-message path return this same sentinel, so both are fixed.

### Tests

`internal/generate/generate_test.go:858-883` and `internal/decompose/message_test.go` reference
`ErrEmptyMessage`. Any assertion on the literal string "stagecoach: empty commit message" must be
updated to drop the prefix.

---

## Issue 6: Auto-Stage Notice Grammar "(1 files)"

### Problem

`internal/cmd/default_action.go:150`:
```go
fmt.Fprintln(stderr, u.Yellow(fmt.Sprintf("Nothing staged — staging all changes (%d files).", n)))
```

Always pluralizes "files". When `n == 1`, output is "(1 files)" instead of "(1 file)".

### Fix

```go
noun := "files"
if n == 1 {
    noun = "file"
}
fmt.Fprintln(stderr, u.Yellow(fmt.Sprintf("Nothing staged — staging all changes (%d %s).", n, noun)))
```

### Tests

`internal/cmd/default_action_test.go:460`: asserts `'Nothing staged — staging all changes (2 files).'`
— this case stays green (n=2). Need to add a test for n=1 with singular "file".

The `n==0` clean-tree path returns early at `default_action.go:147-150` (skips the notice entirely),
so only `n>=1` reaches the format string.
