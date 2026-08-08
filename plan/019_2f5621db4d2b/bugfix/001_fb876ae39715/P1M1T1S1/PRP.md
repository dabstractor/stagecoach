name: "P1.M1.T1.S1 — Refactor generateMessage into generateMessageCore + wrapper (BUG-001 prep)"
description: >
  Behavior-preserving refactor of internal/decompose/message.go: extract steps 1-6 (HEAD parse,
  system prompt, TreeDiff, recent subjects, model resolution, generate+dedupe loop) of generateMessage
  into a new generateMessageCore(ctx, deps, treeA, treeB, seedRejections) that adds a seedRejections
  parameter (pre-seeds the dedupe rejection list + the dedupe check set) and OMITS EditMessage.
  generateMessage becomes a thin wrapper: Core(..., nil) + the existing EditMessage block verbatim.
  The BUG-001 fix (S2) switches the concurrent fast-path to Core; S1 only DEFINES Core. All callers
  and tests are unchanged (signature preserved; seedRejections=nil ⇒ byte-identical behavior).

---

## Goal

**Feature Goal**: Extract a concurrency-safe, EditMessage-free generation core (`generateMessageCore`)
from `generateMessage`, parametrized by `seedRejections`, so the BUG-001 fix (S2) can run generation
concurrently without the shared-EDITMSG editor race, and the BUG-002 fix (P1.M2) can pre-seed
cross-concept dedupe — without changing any caller or existing test.

**Deliverable**: (1) `generateMessageCore(ctx, deps, treeA, treeB, seedRejections []string) (string, error)`
in internal/decompose/message.go (steps 1-6 + the two seedRejections modifications, NO EditMessage);
(2) `generateMessage` rewritten as a wrapper (Core(..., nil) + the verbatim EditMessage block);
(3) updated doc comments on both.

**Success Definition**:
- `generateMessage` signature `(ctx, deps, treeA, treeB) (string, error)` is UNCHANGED ⇒ all 5 callers
  (chain.go:106, decompose.go:371/415/515/742) and the 6 TestGenerateMessage_* tests pass unmodified.
- `generateMessageCore(..., nil)` behaves byte-identically to today's generateMessage steps 1-6; the
  wrapper applies the same EditMessage ⇒ end-to-end behavior preserved.
- All error types/wrapping preserved EXACTLY (*generate.RescueError direct; ErrMessageFailed wrapped;
  ErrEmptyMessage bare from EditMessage).
- `seedRejections` non-empty pre-seeds both the `rejected` BuildUserPayload list and the `dedupeRecent`
  IsDuplicate set (used by S2/P1.M2; not wired in S1).
- `go build ./...`, `go test ./internal/decompose/...`, `make test`, `make lint` pass.

## Why

- **BUG-001 (Critical, prep)**: runLoopFastPath launches N generateMessage goroutines concurrently;
  each ends by calling EditMessage, which writes/opens a single shared `.git/STAGECOACH_EDITMSG` —
  N editors race and a concept silently receives another's message (FR-E4 violation). The fix (S2)
  moves EditMessage out of the concurrent goroutine into the serial publish loop. That requires a
  generation function that does NOT call EditMessage — `generateMessageCore`. S1 extracts it.
- **BUG-002 (Major, prep)**: concurrent generation loses cross-concept dedupe. The fix (P1.M2) passes
  already-decided sibling subjects as `seedRejections` to Core. S1 adds the parameter + the dedupe
  integration; P1.M2 wires the call site.
- **Why a pure refactor first**: landing the extract with seedRejections=nil proves behavior
  preservation via the existing test suite (zero test edits), giving S2/P1.M2 a validated Core to call.

## What

**User-visible behavior**: None (internal refactor; all callers unchanged).

**Technical change (extract + wrapper, one file):**
- `generateMessageCore(ctx, deps, treeA, treeB, seedRejections []string) (string, error)` = current
  steps 1-6 (message.go:75-210), with `rejected` pre-seeded from seedRejections and the dedupe check
  using `dedupeRecent = recent ∪ seedRejections`. Returns `msg, nil` (NO EditMessage, NO nameStatus).
- `generateMessage(ctx, deps, treeA, treeB) (string, error)` = `msg, err := Core(..., nil)` + the
  existing EditMessage block (lines 215-219 verbatim) + `return msg, nil`.

### Success Criteria
- [ ] `generateMessage` signature unchanged; all callers + tests pass unmodified
- [ ] `generateMessageCore` contains steps 1-6, returns pre-edit msg, takes seedRejections
- [ ] seedRejections pre-seeds `rejected` (BuildUserPayload) and `dedupeRecent` (IsDuplicate)
- [ ] `recent` stays pristine (fetched fresh; NOT merged into the prompt)
- [ ] Error types/wrapping preserved exactly (RescueError direct, ErrMessageFailed wrapped, ErrEmptyMessage bare)
- [ ] `go build ./...`, `go test ./internal/decompose/...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact current line numbers, the verbatim split, the seedRejections integration (with the aliasing-safe dedupeRecent build), the behavior-preservation proof (nil ⇒ identical), the error-type contract, and the scope boundaries are all enumerated below (verified by reading source).

### Documentation & References

```yaml
- file: internal/decompose/message.go
  why: "THE file. generateMessage (70-221); step anchors: RevParseHEAD~75, system prompt~86, TreeDiff~104, recent@120, var rejected@135, IsDuplicate(subject,recent)@194, EditMessage block@215-219, return msg,nil@221."
  pattern: "The refactor is a CUT-AND-PASTE of statements: steps 1-6 + the success return move into generateMessageCore; the EditMessage block stays in generateMessage (now prepended by the Core call)."
  gotcha: "recent (120) is fetched AFTER the system prompt and used ONLY at the dedupe check (194) — it is NOT fed to the prompt. Keep recent pristine; build a SEPARATE dedupeRecent = recent ∪ seedRejections for the check only."

- file: internal/decompose/message.go (callers — DO NOT edit, just confirm)
  why: "chain.go:106, decompose.go:371/415/515/742 all call generateMessage(ctx,deps,treeA,treeB). The signature is UNCHANGED ⇒ zero caller edits. Only runLoopFastPath (decompose.go:742) switches to Core in S2."
  gotcha: "Do NOT touch any caller in S1. The 6 message_test.go call sites also stay — they are the TDD gate (must pass unmodified)."

- file: internal/decompose/message_test.go
  why: "The behavior-preservation gate. TestGenerateMessage_Success/DedupeRetryThenSuccess/ParseFailRescue/Timeout/PerRoleTimeout/EmptyDiff/ResolvesSubProvider all call generateMessage(ctx,deps,treeA,treeB) and MUST pass without modification after the refactor."
  pattern: "No new test required in S1 (the existing suite IS the regression net for a behavior-preserving extract). Optionally add ONE TestGenerateMessageCore_SeedRejections unit test asserting seedRejections pre-seeds dedupe — but the hard gate is the existing suite passing unchanged."

- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/architecture/fix_design.md
  why: "The full split design (Part 1) + the seedRejections integration code + the caller-impact analysis."
  section: "Part 1: generateMessageCore Extraction"
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/P1M1T1S1/research/verification_deltas.md
  why: "The exact verified line numbers, the behavior-preservation proof (nil ⇒ identical), the error-type contract, the recent-vs-dedupeRecent clarification, and the scope boundaries. READ THIS before editing."
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  message.go           # THE file: generateMessage(70) → split into generateMessageCore + wrapper
  message_test.go      # TestGenerateMessage_* — UNCHANGED (the behavior-preservation gate)
  decompose.go         # callers at 371/415/515/742 — UNCHANGED in S1 (742 switches to Core in S2)
  chain.go             # caller at 106 — UNCHANGED
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (behavior-preserving extract): generateMessageCore(..., nil) MUST be byte-identical to
//   today's generateMessage steps 1-6. Verify: append([]string{}, nil...) == empty (≡ var rejected []string),
//   and len(nil)>0 is false ⇒ dedupeRecent == recent ⇒ IsDuplicate unchanged. The existing test suite
//   passing UNMODIFIED is the proof.

// CRITICAL (error types — move verbatim, do NOT re-wrap): *generate.RescueError is returned DIRECTLY
//   (not wrapped) from the loop so errors.As(err,&re) works — keep that. ErrMessageFailed is
//   fmt.Errorf("%w: ...: %w")-wrapped — keep that. ErrEmptyMessage comes from EditMessage, returned
//   bare — it stays in the WRAPPER. The refactor changes NO error-handling logic.

// GOTCHA (recent stays pristine): recent (line 120) is fetched fresh and used ONLY at the dedupe check.
//   Do NOT merge seedRejections into recent. Build dedupeRecent = recent ∪ seedRejections as a SEPARATE
//   value and use it ONLY in IsDuplicate. recent itself is unchanged.

// GOTCHA (aliasing-safe dedupeRecent): build dedupeRecent with a fresh head copy —
//   dedupeRecent = append(append([]string{}, recent...), seedRejections...) — so append cannot mutate
//   recent's backing array. (recent from RecentSubjects is likely len==cap so append would allocate
//   anyway, but the fresh-copy is the defensive, obviously-correct form.)

// GOTCHA (EditMessage block stays verbatim in the wrapper): the nameStatus fetch (215) + EditMessage (216)
//   + the if-err return (217-219) move as ONE unit into generateMessage, prepended by the Core call.
//   Do NOT move nameStatus into Core (Core has no EditMessage; it doesn't need nameStatus).

// SCOPE: S1 is message.go ONLY (the extract + wrapper + doc comments). Do NOT edit any caller
//   (decompose.go:371/415/515/742, chain.go:106), EditMessage/finalize.go, publishCommit, or any test.
```

## Implementation Blueprint

### Data models and structure

No struct/type changes. One new unexported function; one existing function rewritten as a wrapper.
No new imports (all deps already imported in message.go: context, errors, fmt, strings, config,
generate, git, prompt, provider).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE generateMessageCore in internal/decompose/message.go
  - PLACE: immediately ABOVE generateMessage (so the wrapper reads top-down: Core then the thin wrapper),
    or immediately below — co-locate the pair. (Either is fine; keep them adjacent.)
  - BODY: the CURRENT generateMessage body from step 1 (RevParseHEAD, ~line 75) through the success
    `return msg, nil` (the line after the `if !success { ... }` block), with these TWO changes:
    (a) Signature: func generateMessageCore(ctx context.Context, deps Deps, treeA, treeB string, seedRejections []string) (string, error)
    (b) Line 135 (`var rejected []string`) → rejected := append([]string{}, seedRejections...)
    (c) After line 120 (`recent, err := messageRecentSubjects(...)` and its err check), insert:
            dedupeRecent := recent
            if len(seedRejections) > 0 {
                dedupeRecent = append(append([]string{}, recent...), seedRejections...)
            }
    (d) Line 194 (`if generate.IsDuplicate(subject, recent)`) → if generate.IsDuplicate(subject, dedupeRecent)
    - STOP at the success return (msg, nil). Do NOT include the EditMessage block (215-219) or the
      nameStatus fetch (215). Those stay in the wrapper.
  - DOC COMMENT: "// generateMessageCore is the BARE message-role generate/dedupe core (steps 1-6 of
    generateMessage) WITHOUT EditMessage — concurrent-safe (no interactive I/O, no shared EDITMSG file).
    Used by runLoopFastPath's goroutines (S2). seedRejections pre-seeds the dedupe rejection list
    (BuildUserPayload's rejection block) and the IsDuplicate check set (dedupeRecent = recent ∪
    seedRejections) for cross-concept duplicate avoidance (P1.M2). With seedRejections=nil it is
    byte-identical to generateMessage's steps 1-6. Returns the pre-edit message, or
    *generate.RescueError / ErrMessageFailed-wrapped errors exactly as generateMessage does."
  - NO new import.
  - DEPENDENCIES: none.

Task 2: REWRITE generateMessage as the wrapper in internal/decompose/message.go
  - REPLACE the entire current generateMessage body (lines 72-221) with:
        msg, err := generateMessageCore(ctx, deps, treeA, treeB, nil)
        if err != nil {
            return "", err
        }
        // §9.22 FR-E1: post-dedupe editor gate (verbatim from the former step 7). AFTER Core accepts a
        // message and BEFORE the caller publishes. The user's hand-edited message bypasses re-check
        // (FR-E3 git parity). This site ALSO covers the arbiter N+1 (chain.go resolveNewCommit).
        nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, treeA, treeB) // best-effort; "" on err
        msg, err = generate.EditMessage(ctx, msg, deps.Config, generate.EditContext{Git: deps.Git, TreeSHA: treeB, NameStatus: nameStatus})
        if err != nil {
            return "", err // ErrEmptyMessage → propagates to runLoop's FR-M12 handling
        }
        return msg, nil
  - KEEP the existing generateMessage doc comment, and APPEND one sentence: "Delegates generation+dedupe
    to generateMessageCore (concurrent-safe; seedRejections=nil here) and applies EditMessage — the
    interactive, non-concurrent-safe tail that runLoopFastPath must NOT run in a goroutine (S2)."
  - DEPENDENCIES: Task 1.

Task 3: VERIFY behavior preservation (the TDD gate — no test edits)
  - RUN: go test ./internal/decompose/ -run TestGenerateMessage -v
  - EXPECTED: all 7 TestGenerateMessage_* + TestPublishCommit_* PASS UNCHANGED.
  - RUN: go test ./internal/decompose/... (full package — chain.go/runLoop callers exercised too).
  - OPTIONAL: add ONE TestGenerateMessageCore_SeedRejections unit test proving seedRejections pre-seeds
    dedupe (a stub that would normally succeed instead hits a duplicate because seedRejections contains
    its subject). This is optional in S1 (the existing suite is the hard gate) but recommended to pin
    the seedRejections contract before P1.M2 wires it.
  - DEPENDENCIES: Tasks 1-2.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the seedRejections integration in generateMessageCore (two touches, aliasing-safe)
rejected := append([]string{}, seedRejections...)        // pre-seed the BuildUserPayload rejection block
recent, err := messageRecentSubjects(ctx, deps.Git, isUnborn)  // pristine git-history subjects
// ... err check ...
dedupeRecent := recent
if len(seedRejections) > 0 {
	dedupeRecent = append(append([]string{}, recent...), seedRejections...)  // fresh head copy (no aliasing)
}
// ... in the loop:
if generate.IsDuplicate(subject, dedupeRecent) {          // checks recent ∪ seedRejections
	rejected = append(rejected, subject)                   // grows the rejection block for the next attempt
	...
}

// PATTERN: the wrapper (EditMessage tail preserved verbatim)
msg, err := generateMessageCore(ctx, deps, treeA, treeB, nil)
if err != nil { return "", err }
nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, treeA, treeB)
msg, err = generate.EditMessage(ctx, msg, deps.Config, generate.EditContext{Git: deps.Git, TreeSHA: treeB, NameStatus: nameStatus})
if err != nil { return "", err }
return msg, nil
```

### Integration Points

```yaml
NO struct / API / caller / test changes. Pure extract-and-wrap inside message.go.

CODE:
  - internal/decompose/message.go — +generateMessageCore; generateMessage rewritten as wrapper; doc comments updated

UNCHANGED: all generateMessage callers (chain.go:106, decompose.go:371/415/515/742); message_test.go;
  EditMessage/finalize.go; publishCommit; error sentinels (ErrMessageFailed/ErrPublicationFailed).

DOWNSTREAM (consumes generateMessageCore — do NOT implement in S1):
  - P1.M1.T2.S1: runLoopFastPath launch closure (decompose.go:742) generateMessage → generateMessageCore
  - P1.M1.T2.S2: EditMessage applied in runLoopFastPath's serial publish loop (BUG-001 fix)
  - P1.M2.T1.S1: runLoopFastPath passes non-nil seedRejections (cross-concept dedupe, BUG-002 fix)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...
go vet ./...
gofmt -l internal/decompose/
# Expected: empty.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation — THE gate for a behavior-preserving refactor)

```bash
# The existing generateMessage tests MUST pass UNCHANGED (behavior-preservation proof)
go test ./internal/decompose/ -run TestGenerateMessage -v
# Expected: all TestGenerateMessage_* PASS (Success/DedupeRetryThenSuccess/ParseFailRescue/Timeout/
#           PerRoleTimeout/EmptyDiff/ResolvesSubProvider).

# Full decompose package (callers in runLoop/chain exercised — must still pass)
go test ./internal/decompose/... -v

# Whole suite (race)
make test
# Expected: ALL pass — ZERO test files modified.
```

### Level 3: Integration Testing (System Validation)

```bash
# (S1 is a pure refactor with no caller change — the unit suite IS the integration proof.
#  The BUG-001/BUG-002 end-to-end fixes land in S2/P1.M2 and are gated by P1.M3's regression tests.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: both functions exist; Core takes seedRejections; wrapper delegates
grep -n "func generateMessageCore\|func generateMessage(" internal/decompose/message.go
grep -n "generateMessageCore(ctx, deps, treeA, treeB, nil)" internal/decompose/message.go
# Expected: both funcs present; the wrapper calls Core(..., nil).

# Grep guard: Core uses dedupeRecent (the seedRejections integration), wrapper uses EditMessage
grep -n "dedupeRecent\|generate.EditMessage" internal/decompose/message.go
# Expected: dedupeRecent only in generateMessageCore; EditMessage only in generateMessage (the wrapper).

# Grep guard: NO caller edited in S1 (signature unchanged)
git diff --stat internal/decompose/decompose.go internal/decompose/chain.go
# Expected: empty (callers untouched). Only internal/decompose/message.go changed.

# Mutation guard (manual, then revert): prove the split is real — temporarily make Core return a
#   constant string (bypass the loop) and confirm TestGenerateMessage_* FAILS, then revert; or
#   temporarily drop the EditMessage call in the wrapper and confirm the --edit test fails, then revert.

# Behavior-preservation guard: the diff is a pure move
git diff internal/decompose/message.go | grep -E '^-' | grep -vE '^---|^\s*//|^\s*$' | head
# Expected: the only REMOVED non-comment lines are the ones MOVED into Core/wrapper (the body
#           statements), not deleted logic. Cross-check the added lines are the same statements.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/decompose/` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass — ZERO test files modified

### Feature Validation
- [ ] `generateMessage` signature `(ctx, deps, treeA, treeB)` UNCHANGED
- [ ] `generateMessageCore(ctx, deps, treeA, treeB, seedRejections)` exists, returns pre-edit msg
- [ ] seedRejections pre-seeds `rejected` (BuildUserPayload) and `dedupeRecent` (IsDuplicate)
- [ ] `recent` stays pristine; EditMessage block verbatim in the wrapper
- [ ] All error types/wrapping preserved exactly

### Scope-Boundary Validation
- [ ] NO caller edited (decompose.go:371/415/515/742, chain.go:106)
- [ ] NO test edited (message_test.go unchanged and passing)
- [ ] NO EditMessage/finalize.go/publishCommit change
- [ ] Only internal/decompose/message.go changed
- [ ] seedRejections parameter DEFINED but not yet WIRED (S2/P1.M2 wire the call sites)

### Code Quality
- [ ] generateMessageCore + generateMessage co-located (adjacent in message.go)
- [ ] Doc comments: Core = concurrent-safe core + seedRejections rationale; wrapper = delegates + EditMessage tail
- [ ] dedupeRecent built aliasing-safe (fresh head copy)

---

## Anti-Patterns to Avoid

- ❌ Don't change generateMessage's signature or any caller — the refactor is behavior-preserving precisely because the wrapper keeps `(ctx, deps, treeA, treeB)`. Any caller edit is out of scope (S2 edits decompose.go:742).
- ❌ Don't re-wrap or alter error handling — *generate.RescueError must stay direct (errors.As), ErrMessageFailed stays %w-wrapped, ErrEmptyMessage stays bare. Move statements verbatim.
- ❌ Don't merge seedRejections into `recent` — recent is the pristine git-history list; build a separate dedupeRecent for the IsDuplicate check only. (recent is not even fed to the prompt.)
- ❌ Don't build dedupeRecent with `append(recent, seedRejections...)` directly — that can mutate recent's backing array. Use the fresh-head-copy form `append(append([]string{}, recent...), seedRejections...)`.
- ❌ Don't move the nameStatus fetch into Core — Core has no EditMessage and doesn't need it. It stays in the wrapper, right before EditMessage.
- ❌ Don't add a new test as a SUBSTITURE for the existing suite — the existing TestGenerateMessage_* passing UNCHANGED is the behavior-preservation proof. An optional Core seedRejections test is fine, but the hard gate is the unchanged suite.
- ❌ Don't wire seedRejections into any caller in S1 — S2 (decompose.go:742) and P1.M2 own the call-site changes. S1 only DEFINES the parameter.
- ❌ Don't edit EditMessage/finalize.go to "fix" the shared EDITMSG path — that's a different approach; this fix (S2) moves EditMessage to the serial loop. S1 just extracts the Core.

---

## Confidence Score: 9/10

One-pass success is very high: a pure extract-and-wrap with the verbatim split spelled out, the
behavior-preservation proof (nil ⇒ identical) explicit, the error-type contract fixed, and the existing
test suite as the regression gate (zero test edits required). The -1 is for the seedRejections
integration being the one piece of NEW logic (the dedupeRecent build + the rejected pre-seed) — a
subtle aliasing or recent/dedupeRecent mix-up could silently change dedupe behavior. Mitigated by the
aliasing-safe dedupeRecent form, the "recent stays pristine" gotcha, and the existing dedupe test
(TestGenerateMessage_DedupeRetryThenSuccess) which exercises the IsDuplicate path and must still pass.