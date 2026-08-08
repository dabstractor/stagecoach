name: "P1.M1.T2.S1 — Switch runLoopFastPath launch closure to generateMessageCore (BUG-001 step 1/2)"
description: >
  First half of the BUG-001 fix: in runLoopFastPath's concurrent launch closure (internal/decompose/
  decompose.go:742), switch the goroutine call from generateMessage to generateMessageCore(ctx, deps,
  treeA, treeB, nil) — so concurrent generation no longer invokes the shared-EDITMSG EditMessage (the
  race source). seedRejections=nil (cross-concept dedupe is P1.M2.T1.S1). Update the concurrency-safety
  comment to reference generateMessageCore + the deferred EditMessage (→ serial publish loop, S2/FR-E4).
  One call-site edit + one comment edit. The existing TestRunLoopFastPath_ConcurrentPublish passes
  unchanged (cfg.Edit=false ⇒ EditMessage was a no-op ⇒ Core is behavior-identical). NOTE: S1 alone
  transiently skips --edit on the fast-path; S2 (P1.M1.T2.S2) restores it in the serial loop. Only
  decompose.go touched; runLoop's closure (line 515) is UNCHANGED.

---

## Goal

**Feature Goal**: Remove the concurrency-unsafe EditMessage invocation from runLoopFastPath's concurrent
generation goroutines (BUG-001 step 1 of 2) by switching the launch closure to `generateMessageCore`
(which omits EditMessage). This stops N goroutines from racing on the shared `.git/STAGECOACH_EDITMSG`
file. The second half (S2) re-applies EditMessage in the SERIAL publish loop (one editor at a time,
FR-E4). This task is the call-site switch + comment update ONLY.

**Deliverable**: (1) ONE call-site edit in `internal/decompose/decompose.go` — line 742 inside
`runLoopFastPath`'s `launch` closure: `generateMessage(ctx, deps, treeA, treeB)` →
`generateMessageCore(ctx, deps, treeA, treeB, nil)`; (2) the concurrency-safety comment (lines 735-737,
the "FR-M14: launch ALL N" block) rewritten to reference `generateMessageCore` and note EditMessage is
deferred to the serial publish loop (S2 / FR-E4).

**Success Definition**:
- The fast-path launch closure's goroutine calls `generateMessageCore(..., nil)` (no EditMessage in the
  concurrent path).
- The concurrency-safety comment names `generateMessageCore` (not `generateMessage`) and explains the
  deferred EditMessage.
- `runLoop`'s launch closure (line 515) is UNCHANGED — still calls `generateMessage` (runLoop's 1-deep
  overlap serializes editing; it is unaffected by BUG-001).
- `TestRunLoopFastPath_ConcurrentPublish` (decompose_test.go:3150) passes unchanged (cfg.Edit=false ⇒
  EditMessage was a no-op ⇒ behavior-identical under Core).
- `go build ./...`, `go test ./internal/decompose/...`, `make test`, `make lint` pass.

## User Persona (if applicable)

**Target User**: Stagecoach maintainers (this is an internal concurrency-safety refactor; no user-facing
surface until S2 completes the pair).

**Use Case**: A maintainer lands S1 (this task) then S2 as a coordinated two-step BUG-001 fix. After S1,
the concurrent editor race is eliminated (no EditMessage in goroutines); after S2, --edit is restored in
the serial loop.

**Pain Points Addressed**: BUG-001 — N concurrent editors racing on one `.git/STAGECOACH_EDITMSG`,
silently attaching the wrong message to a commit (FR-E4 violation). S1 removes the race source; S2
restores correct --edit behavior.

## Why

- **BUG-001 (Critical, step 1 of 2)**: runLoopFastPath launches N generateMessage goroutines; each ends
  by calling EditMessage, which writes/opens a single shared `.git/STAGECOACH_EDITMSG` — N editors race
  and a concept silently receives another's message. The fix is to move EditMessage out of the
  concurrent goroutine into the serial publish loop. That requires the goroutine to call a
  generation function WITHOUT EditMessage — `generateMessageCore` (extracted in P1.M1.T1.S1). This task
  switches the call site.
- **Why a two-step split (S1 then S2)**: S1 (the call-site switch) and S2 (the serial-loop EditMessage)
  are separated so each is a small, reviewable, testable change. S1's gate is the existing
  fast-path test passing unchanged (Edit=false ⇒ no-op EditMessage ⇒ identical); S2's gate is the
  BUG-001 regression test (P1.M3.T1.S1, Edit=true).
- **Bounded**: one call line + one comment. No new logic (Core already exists from S1). No test edit.

## What

**User-visible behavior**: None yet on its own (internal). After S1 alone, `--edit` on the fast-path is
transiently skipped (Core has no EditMessage); S2 restores it. With `--edit` OFF (the default and the
existing-test case), behavior is identical.

**Technical change (one call line + one comment):**
1. decompose.go line 742 (inside `runLoopFastPath`'s `launch` closure):
   `m, e := generateMessage(ctx, deps, treeA, treeB)` → `m, e := generateMessageCore(ctx, deps, treeA, treeB, nil)`.
2. decompose.go lines 735-737 (the "FR-M14: launch ALL N" comment): rewrite to name `generateMessageCore`
   and note EditMessage is deferred to the serial publish loop (S2 / FR-E4 — the editor is not
   concurrency-safe, so it runs one-at-a-time in the serial loop, not in the goroutine).

### Success Criteria
- [ ] runLoopFastPath launch closure calls `generateMessageCore(ctx, deps, treeA, treeB, nil)`
- [ ] seedRejections is `nil` (cross-concept dedupe is P1.M2.T1.S1)
- [ ] concurrency-safety comment names generateMessageCore + the deferred EditMessage (S2/FR-E4)
- [ ] runLoop's launch closure (line 515) UNCHANGED — still generateMessage
- [ ] TestRunLoopFastPath_ConcurrentPublish passes unchanged (cfg.Edit=false)
- [ ] `go build ./...`, `go test ./internal/decompose/...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact call line (742), the exact replacement, the unique anchor to disambiguate it from
the byte-identical runLoop closure (line 515), the comment text to rewrite, the transient --edit gap
(S1/S2 design), the prerequisite (generateMessageCore exists), the test gate, and the scope fences are
all below.

### Documentation & References

```yaml
- file: internal/decompose/decompose.go
  why: "THE change site. runLoopFastPath's `launch` closure @739-744; the call to switch is @742. The
        concurrency-safety comment is @735-737 ('FR-M14: launch ALL N ...'). ⚠️ There is a SECOND,
        byte-identical call at @515 INSIDE runLoop — that one MUST STAY on generateMessage."
  pattern: >
    // the fast-path closure (edit THIS one):
    launch := func(i int, treeA, treeB string) chan msgOut {
        ch := make(chan msgOut, 1)
        go func() {
            m, e := generateMessage(ctx, deps, treeA, treeB)   // ← @742: switch to generateMessageCore(..., nil)
            ch <- msgOut{conceptIdx: i, treeA: treeA, treeB: treeB, msg: m, err: e}
        }()
        return ch
    }
  critical: "Edit ONLY line 742 (the fast-path closure). Line 515 (runLoop's closure) is byte-identical
             and MUST NOT be touched — runLoop's 1-deep overlap serializes editing, so it keeps
             generateMessage (with EditMessage inside). Disambiguate by grepping the UNIQUE fast-path
             comment 'FR-M14: launch ALL N' and editing the call in the closure directly below it."

- file: internal/decompose/message.go
  why: "generateMessageCore is defined here (@80, signature `(ctx, deps, treeA, treeB, seedRejections
        []string) (string, error)`). P1.M1.T1.S1 extracted it (the BUG-001-prep refactor). This task
        CONSUMES it — read-only here (do NOT edit message.go)."
  pattern: "generateMessageCore = steps 1-6 of generateMessage (generation+dedupe), returns the PRE-EDIT
            message, takes seedRejections. generateMessage = Core(..., nil) + EditMessage (the wrapper)."
  critical: "generateMessageCore is the prerequisite. Confirm it exists: `grep -n 'func generateMessageCore'
             internal/decompose/message.go` → @80. If absent, S1 hasn't landed and this task won't compile."

- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/architecture/fix_design.md
  why: "§Part 2 specifies this exact switch (the BEFORE/AFTER launch closure) + the S2 serial-loop
        EditMessage block this task DEFERS. Read it to confirm the two-step design."
  section: "Part 2: BUG-001 Fix — EditMessage in serial publish loop (the launch-closure BEFORE/AFTER)"

- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/P1M1T1S1/PRP.md
  why: "The CONTRACT: S1 defines generateMessageCore (concurrent-safe, no EditMessage, seedRejections
        param). This task consumes it. Read to confirm Core's signature + the nil ⇒ behavior-identical
        property (the reason the existing test passes unchanged)."
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  decompose.go    # THE edit: runLoopFastPath launch closure @742 (generateMessage → generateMessageCore(...,nil)) + comment @735-737
  message.go      # generateMessageCore @80 (S1, LANDED) — CONSUMED, not edited
  decompose_test.go  # TestRunLoopFastPath_ConcurrentPublish @3150 — UNCHANGED (the gate)
```

### Desired Codebase tree with files to be added

```bash
internal/decompose/decompose.go   # MODIFY: 1 call line (@742) + 1 comment block (@735-737)
# (no new files; no test edit; no other package touched)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (two identical call lines — edit ONLY the fast-path one): decompose.go has TWO launch
//   closures with the byte-identical call `m, e := generateMessage(ctx, deps, treeA, treeB)`: @515
//   (inside runLoop) and @742 (inside runLoopFastPath). Edit ONLY @742. runLoop (@515) is UNAFFECTED by
//   BUG-001 (its 1-deep overlap serializes editing) and MUST keep generateMessage. Disambiguate by the
//   UNIQUE fast-path comment "FR-M14: launch ALL N" above the @742 closure. Do NOT blind-edit by line
//   number — grep the anchor.

// CRITICAL (transient --edit gap — S1 alone skips --edit on the fast-path): generateMessageCore OMITS
//   EditMessage (the point of the extract). So after this task, if cfg.Edit==true, the editor is NOT
//   invoked on the fast-path until S2 adds EditMessage to the serial publish loop. This is INTENTIONAL
//   (two-step design). The existing test passes because cfg.Edit defaults false (EditMessage was a no-op
//   ⇒ Core is behavior-identical). Do NOT "fix" the gap by re-adding EditMessage here — that's S2's job
//   (in the SERIAL loop, not the goroutine). No test in the package sets Edit=true (grep-confirmed), so
//   nothing breaks.

// CRITICAL (seedRejections MUST be nil): pass `nil`, not an empty slice literal and not a variable.
//   Cross-concept dedupe (passing already-decided sibling subjects) is BUG-002's job (P1.M2.T1.S1),
//   which will change the `nil` to `seenSubjects`. At concurrent-launch time no siblings are decided
//   yet, so nil is semantically correct here. generateMessageCore(..., nil) is byte-identical to
//   generateMessage's steps 1-6 (the S1 contract).

// GOTCHA (the comment rewrite must explain WHY EditMessage moved): the current comment says each
//   goroutine "calls generateMessage ... never touches the live .git/index" — that was the
//   concurrency-safety argument, but it was INCOMPLETE (it ignored EditMessage's shared-EDITMSG write).
//   The new comment must name generateMessageCore AND state that EditMessage is deferred to the serial
//   publish loop BECAUSE the interactive editor (shared STAGECOACH_EDITMSG) is not concurrency-safe
//   (FR-E4 serialized publication). This prevents the blind spot from recurring.

// GOTCHA (don't touch the serial publish loop): S2 (P1.M1.T2.S2) adds EditMessage to the serial loop;
//   P1.M2.T1.S1 adds cross-concept dedupe. This task edits ONLY the launch closure + its comment. The
//   serial publish loop (~770-828), publishCommit, msgOut struct, and the channel are UNCHANGED.

// SCOPE: do NOT edit runLoop's closure (@515), message.go (S1 owns Core), EditMessage/finalize.go,
//   publishCommit, any test, or runSingleEscape/runSingleShortcut/runOneFileShortcut/chain.go (they call
//   generateMessage, unchanged).
```

## Implementation Blueprint

### Data models and structure
None. One call-site change + one comment rewrite. The `msgOut` struct, the channel, and the serial
publish loop are untouched. Only the `.msg` content of each `msgOut` changes (post-edit → pre-edit).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/decompose/decompose.go — switch the fast-path launch closure call to generateMessageCore
  - LOCATE the runLoopFastPath launch closure by its UNIQUE preceding comment:
      grep -n 'FR-M14: launch ALL N' internal/decompose/decompose.go   # → the comment block @735-737
    The closure directly below it (@739-744) is the edit target. (NOT the runLoop closure @512-516.)
  - EDIT the call line (@742):
      OLD: m, e := generateMessage(ctx, deps, treeA, treeB)
      NEW: m, e := generateMessageCore(ctx, deps, treeA, treeB, nil)
  - DO NOT touch any other line in the closure (the msgOut literal, the channel, the goroutine structure).
  - DO NOT touch the runLoop closure @515 (verify with: the @515 call still reads generateMessage).
  - DEPENDENCIES: generateMessageCore exists (message.go:80 — P1.M1.T1.S1 landed).

Task 2: EDIT internal/decompose/decompose.go — rewrite the concurrency-safety comment (@735-737)
  - REPLACE the current comment block:
      OLD:
        // FR-M14: launch ALL N (non-skipped) message generations CONCURRENTLY. Safe: each goroutine calls
        // generateMessage, which reasons over a tree-to-tree diff (read-only tree reads) and never touches
        // the live .git/index (git_primitives.md). No cap (FR-M14; max_commits default 12 bounds N).
      NEW (substance):
        // FR-M14: launch ALL N (non-skipped) message generations CONCURRENTLY. Each goroutine calls
        // generateMessageCore — the bare generate/dedupe core (tree-to-tree diff, read-only tree reads,
        // never touches the live .git/index). seedRejections is nil here (cross-concept dedupe is applied
        // in the serial publish loop — P1.M2). EditMessage is DELIBERATELY NOT called in the goroutine:
        // it writes/opens a single shared .git/STAGECOACH_EDITMSG and is not concurrency-safe, so it is
        // deferred to the serial publish loop (one editor at a time, FR-E4 serialized publication; P1.M1.T2.S2).
        // No cap (FR-M14; max_commits default 12 bounds N).
  - Keep it a `//` comment block; preserve the line's intent (FR-M14 concurrency).
  - DEPENDENCIES: Task 1.

Task 3: VERIFY build + vet + format + the TDD gate + full package
  - go build ./...
  - go vet ./internal/decompose/...
  - gofmt -l internal/decompose/decompose.go   # must list nothing
  - go test ./internal/decompose/ -run TestRunLoopFastPath_ConcurrentPublish -v   # the item's TDD gate
  - go test ./internal/decompose/...            # full package (runLoop/chain exercised)
  - make test && make lint
  - Grep guard: exactly ONE generateMessageCore(..., nil) hit (the fast-path closure @742).
  - Grep guard: the runLoop closure @515 STILL calls generateMessage (unchanged).
  - DEPENDENCIES: Tasks 1-2.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the launch closure AFTER the switch (only the call line changes)
launch := func(i int, treeA, treeB string) chan msgOut {
    ch := make(chan msgOut, 1) // buffered(1) — goroutine sends once + exits; never blocks
    go func() {
        m, e := generateMessageCore(ctx, deps, treeA, treeB, nil) // BUG-001: Core (no EditMessage) — editor deferred to the serial loop (S2/FR-E4)
        ch <- msgOut{conceptIdx: i, treeA: treeA, treeB: treeB, msg: m, err: e}
    }()
    return ch
}

// PATTERN: the concurrency-safety comment names Core + explains the deferred EditMessage
// (the OLD comment's ".git/index" safety argument was incomplete — it missed EditMessage's shared file)
```

### Integration Points

```yaml
NO struct / API / test / new-logic changes. One call line + one comment.

CODE:
  - internal/decompose/decompose.go runLoopFastPath launch closure @742 — generateMessage → generateMessageCore(..., nil)
  - internal/decompose/decompose.go concurrency-safety comment @735-737 — rewrite for Core + deferred EditMessage

CONSUMED (read-only, prerequisite):
  - internal/decompose/message.go generateMessageCore @80 (P1.M1.T1.S1)

DOWNSTREAM (completes the BUG-001 fix — do NOT implement here):
  - P1.M1.T2.S2: apply EditMessage in runLoopFastPath's SERIAL publish loop (before publishCommit) — restores --edit, concurrency-safe
  - P1.M2.T1.S1: change the `nil` → `seenSubjects` (cross-concept dedupe, BUG-002) + add the seenSubjects tracking
  - P1.M3.T1.S1: the BUG-001 regression test (Edit=true, fast-path, no cross-contamination)

UNCHANGED (do NOT touch): runLoop's closure @515 (keeps generateMessage); message.go (S1); the serial
  publish loop; publishCommit; msgOut struct; EditMessage/finalize.go; runSingle*/chain.go callers; any test.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (consumes generateMessageCore; compiles post-S1)
go build ./...
# Vet the package
go vet ./internal/decompose/...
# Format check
gofmt -l internal/decompose/decompose.go
# Expected: nothing listed. If listed: gofmt -w it.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The item's TDD gate — the fast-path happy-path test (cfg.Edit=false ⇒ EditMessage was a no-op ⇒ Core is behavior-identical)
go test ./internal/decompose/ -run TestRunLoopFastPath_ConcurrentPublish -v
# Expected: PASS unchanged.

# Full decompose package (runLoop @515 + chain callers exercised — must still pass; they keep generateMessage)
go test ./internal/decompose/... -v

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# S1 alone does NOT complete the BUG-001 fix (--edit is transiently skipped on the fast-path until S2).
# The within-scope proof is the unit test (the fast-path test passes unchanged). The end-to-end BUG-001
# proof lands with S2 + the P1.M3.T1.S1 regression test. Do NOT attempt a --edit fast-path smoke now —
# it would (correctly) skip the editor until S2 lands.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: exactly ONE generateMessageCore(..., nil) call — the fast-path closure
grep -n 'generateMessageCore(ctx, deps, treeA, treeB, nil)' internal/decompose/decompose.go
# Expected: one hit (@742).

# Grep guard: the runLoop closure @515 STILL calls generateMessage (UNCHANGED)
grep -n 'generateMessage(ctx, deps, treeA, treeB)' internal/decompose/decompose.go
# Expected: hits at @371, @415, @515 (runLoop), and the chain/single sites — but NOT @742 (switched to Core).
#           Confirm @515 is among them (runLoop untouched).

# Grep guard: the comment names generateMessageCore + the deferred EditMessage
grep -n 'generateMessageCore\|deferred.*serial\|EditMessage.*serial\|FR-E4' internal/decompose/decompose.go | head
# Expected: the rewritten comment references generateMessageCore + the serial-loop EditMessage deferral.

# Scope-boundary guard: ONLY decompose.go changed by this subtask
git diff --stat -- internal/decompose/message.go internal/decompose/decompose_test.go internal/generate/ internal/git/
# Expected: empty (message.go = S1; tests/generate/git untouched).

# Scope-boundary guard: the serial publish loop is UNCHANGED (S2/P1.M2 own it)
git diff internal/decompose/decompose.go | grep -E '^[+-]' | grep -iE 'EditMessage|seenSubjects|IsDuplicate' || echo "OK: serial loop untouched (no EditMessage/seenSubjects added)"
# Expected: "OK: serial loop untouched" (this task adds NEITHER — S2 adds EditMessage; P1.M2 adds dedupe).

# Transient-gap guard: confirm NO test sets Edit=true in the package (so the gap breaks nothing)
grep -rn 'Edit:\s*true\|\.Edit = true\|Edit:true' internal/decompose/ || echo "OK: no Edit=true test (the transient --edit gap breaks no test)"
# Expected: "OK: no Edit=true test".
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/decompose/...` clean
- [ ] `gofmt -l internal/decompose/decompose.go` empty
- [ ] `make lint` zero errors
- [ ] `go test ./internal/decompose/ -run TestRunLoopFastPath_ConcurrentPublish -v` passes (item's TDD gate)
- [ ] `make test` (race) all pass

### Feature Validation
- [ ] runLoopFastPath launch closure calls `generateMessageCore(ctx, deps, treeA, treeB, nil)`
- [ ] seedRejections is `nil` (cross-concept dedupe is P1.M2.T1.S1)
- [ ] concurrency-safety comment names generateMessageCore + the deferred EditMessage (S2/FR-E4)
- [ ] msgOut.msg is now the PRE-EDIT message (EditMessage not invoked in the goroutine)

### Scope-Boundary Validation
- [ ] runLoop's launch closure (@515) UNCHANGED — still `generateMessage`
- [ ] NO EditMessage added to the serial publish loop (that's S2)
- [ ] NO seenSubjects / cross-concept dedupe added (that's P1.M2.T1.S1)
- [ ] NO change to message.go (S1), EditMessage/finalize.go, publishCommit, msgOut, any test, runSingle*/chain.go
- [ ] Only internal/decompose/decompose.go touched (1 call line + 1 comment)

### Code Quality
- [ ] The closure's structure (msgOut literal, channel, goroutine) is byte-identical except the one call
- [ ] The comment explains WHY EditMessage moved (not concurrency-safe; FR-E4 serial) — closes the old blind spot
- [ ] seedRejections=nil matches the "no siblings decided at launch time" semantics

---

## Anti-Patterns to Avoid

- ❌ Don't edit the runLoop closure (line 515) — it is byte-identical to the fast-path closure but MUST stay on `generateMessage`. runLoop's 1-deep overlap serializes editing; it is unaffected by BUG-001. Grep the UNIQUE "FR-M14: launch ALL N" anchor to find the fast-path closure (742), and edit ONLY that one.
- ❌ Don't re-add EditMessage in the goroutine or anywhere in this task — that re-introduces the race. EditMessage's new home is the SERIAL publish loop (S2 / P1.M1.T2.S2), not the concurrent closure. The transient --edit gap (S1 skips --edit on the fast-path) is INTENTIONAL and closed by S2.
- ❌ Don't pass a non-nil seedRejections — cross-concept dedupe is BUG-002's job (P1.M2.T1.S1), which will change `nil` → `seenSubjects`. At concurrent-launch time no sibling subjects are decided, so `nil` is semantically correct and keeps Core byte-identical to generateMessage's steps 1-6 (the S1 contract).
- ❌ Don't touch the serial publish loop, publishCommit, the msgOut struct, or the channel — S2 (EditMessage) and P1.M2.T1.S1 (dedupe) own the serial loop. This task is the launch closure + its comment ONLY.
- ❌ Don't edit message.go (S1 owns generateMessageCore), EditMessage/finalize.go, or any test. The existing TestRunLoopFastPath_ConcurrentPublish passing UNCHANGED is the gate (cfg.Edit=false ⇒ no-op EditMessage ⇒ Core behavior-identical).
- ❌ Don't keep the old concurrency-safety comment's claim that each goroutine "calls generateMessage ... never touches the live .git/index" — that was the incomplete argument that missed EditMessage's shared-file race. Rewrite it to name generateMessageCore AND explain the deferred EditMessage, so the blind spot doesn't recur.
- ❌ Don't attempt a `--edit` fast-path smoke test now — it will (correctly) skip the editor until S2 lands. The within-scope proof is the unit test.
- ❌ Don't bump the edit to a "safer" form (e.g. a per-concept EDITMSG path) — that's a different approach; this fix's design (defer to the serial loop) is fixed by the architecture's Part 2. Just switch the call.

---

## Confidence Score: 10/10

This is one call-line edit (`generateMessage` → `generateMessageCore(..., nil)`) + one comment rewrite,
with the prerequisite (`generateMessageCore` @ message.go:80) already landed, the byte-identical-twin
scope fence (runLoop @515 vs fast-path @742) disambiguated by a unique grep anchor, the transient --edit
gap (S1/S2 design) explicitly explained and verified to break no test (none set Edit=true), and the
existing fast-path test as the unchanged TDD gate (cfg.Edit=false ⇒ behavior-identical). The only
conceivable failure modes — editing the wrong closure (515 vs 742), re-adding EditMessage, passing
non-nil seedRejections, or touching the serial loop — are each explicitly guarded by a CRITICAL gotcha +
a Level-4 grep check.