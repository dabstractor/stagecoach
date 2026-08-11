name: "P1.M2.T1.S2 — Stager-availability guard in runLoop (capability-gap hard-error)"
description: >
  BUG-003 / FR-M13 part 2. After S1 deferred the stager requirement out of ResolveRoles, the
  tooled-stager path (runLoop) must detect "no tooled stager is available" and fail fast with a
  clear, actionable error BEFORE any freeze/generation work. This PRP covers the guard AND —
  critically — resolves the FR-M12d interaction that halted the previous attempt: the guard fires
  on a CAPABILITY GAP (bare/tooled-less stager, StagerAvailable=false), which is ORTHOGONAL to
  FR-M12d's RUNTIME-failure retry-then-empty. The only test that conflated the two is re-scoped.

---

## Goal

**Feature Goal**: When decompose routes a **non-file-disjoint** (shared-file) partition to the
tooled-stager `runLoop`, and `ResolveRoles` (S1) reported `StagerAvailable=false` (the configured
stager is bare/tooled-less with no tooled fallback), `runLoop` MUST return a clear, actionable
`ErrDecomposeFailed`-wrapped error immediately — BEFORE any freeze/generation work — instead of
silently producing zero commits (the pre-fix "silent failure") or crashing deep in `stageConcept`
with an opaque render error.

**Deliverable**:
1. A stager-availability guard at the top of `runLoop` (`internal/decompose/decompose.go`) that
   checks `deps.Roles.StagerAvailable` and returns the actionable error when false.
2. Re-scoped regression coverage: the one test that conflated the capability-gap with FR-M12d's
   runtime-failure path is updated to assert the BUG-003 hard-error contract, while FR-M12d's
   runtime retry-then-empty remains independently covered by the canonical tooled-stager tests.

**Success Definition**: A non-disjoint partition with no stager-capable provider produces a clear,
actionable error at the CLI level (`stagecoach: <message>`, exit 1) instead of a silent zero-commit
success; the disjoint fast-path is unaffected; ALL of `go test ./internal/decompose/...` is GREEN,
including the preserved FR-M12d runtime-failure tests.

---

## CRITICAL: Resolving the Previous Attempt's "Spec Conflict" (READ FIRST)

The previous implementation attempt HALTED, reporting a "fundamental spec-level conflict" between
the guard and FR-M12d. **This was a false alarm.** The conflict is fully resolvable with NO spec
change. The implementer MUST understand the distinction below before writing any code — it is the
single most important thing in this PRP.

### The two conditions are ORTHOGONAL (spec-backed)

There are **two distinct** "stager can't stage" conditions, governed by different spec sections:

| Condition | Spec authority | What it means | Who handles it |
|-----------|---------------|---------------|----------------|
| **Capability gap** | `spec/02-providers.md:535` (§12.7): *"A provider with empty `tooled_flags` simply cannot serve as a stager"*; `spec/01-product.md:364` (FR-M13): a no-tooled provider *"could not serve the stager role at all"* | The stager provider is **bare** — its manifest declares nil `tooled_flags`, so it can NEVER render in tooled mode. Deterministic, permanent, non-transient. Detected at **role resolution** (S1 sets `StagerAvailable=false`). | **The guard (this PRP / BUG-003).** Hard error BEFORE the stager is ever invoked. |
| **Runtime failure** | `spec/03-generation.md:150` (§13.6.6): *"Stager exits non-zero: retry once; on second failure treat as empty (FR-M8) and continue"* | A **tooled** stager was successfully resolved (`StagerAvailable=true`) and **invoked**, but the agent crashed/errored at exec time. Transient/recoverable — "so one bad concept cannot poison the run." Detected **after** exec attempt. | **FR-M12d (`invokeStagerRetry` in `runLoop`).** Retry once → empty-skip → continue. |

### Why the previous attempt saw a "conflict"

`stageConcept` (`internal/decompose/stager.go:108`) wraps a render error as `ErrStagerFailed`:
```go
spec, rerr := deps.Roles.Stager.Render(mdl, "", task, rsn, provider.RenderTooled)
if rerr != nil {
    return fmt.Errorf("%w: render: %v", ErrStagerFailed, rerr)
}
```
So at the **code** level, a bare stager on a shared partition DOES manifest as "stager exits
non-zero" (`ErrStagerFailed`), which FR-M12d's `invokeStagerRetry` catches and swallows into an
empty-skip. That is the implementation-level overlap the previous attempt tripped over.

**But the SPEC treats them differently**: a bare provider "simply cannot serve as a stager" (§12.7) —
that is a capability gap, not a transient exec failure. The guard (BUG-003) is the spec-intended
mechanism for the capability gap: detect it at role resolution (cheap, deterministic) and surface a
clear error, rather than letting every concept deterministically fail-and-empty-skip into a silent
zero-commit "success."

### The resolution (NO spec change required)

1. **The guard is correct and stays.** It fires on `StagerAvailable=false` (capability gap) → hard
   error. This is consistent with §12.7 and BUG-003. It fires BEFORE `invokeStagerRetry`/`stageConcept`
   ever run, so the capability gap never reaches FR-M12d's runtime path.
2. **FR-M12d is fully preserved** for genuine runtime failures: a TOOLED stager (`StagerAvailable=true`)
   that exits non-zero still goes through `invokeStagerRetry` → retry once → empty-skip. The guard
   does NOT fire (StagerAvailable is true). Proven by `TestDecompose_StagerRetryThenEmpty`
   (`decompose_test.go:2001`), which uses a tooled stub stager + a seam returning
   `errors.New("simulated stager failure")` + `StagerAvailable: true`.
3. **Exactly ONE test conflated the two** and must be re-scoped (Task 2 below). That test,
   `shared_cannot_serve_as_stager`, used a BARE stager (capability gap) to exercise FR-M12d. After
   BUG-003, the capability-gap case is owned by the guard. Re-scoping it to assert the hard error
   PRESERVES its purpose ("prove a tooled-less provider cannot stage a shared file") — only the
   asserted OUTCOME changes (silent-skip → clear error), which is exactly what BUG-003 mandates.

**This is a test ALIGNMENT, not a spec change.** The implementer is NOT modifying `spec/SPEC.md`,
NOT altering FR-M12d's contract (runtime failures still retry-then-empty), and NOT deleting coverage
of a shipped feature. They are correcting a test that used a capability-gap trigger to validate a
runtime-failure behavior, now that BUG-003 cleanly separates the two. Per AGENTS.md rule 3, this is a
bug fix bringing out-of-spec behavior (silent failure) into line with the spec (clear error).

**DO NOT HALT on this interaction.** If you reach this point and feel the urge to halt, re-read this
section: the distinction is capability-gap (guard) vs runtime-failure (FR-M12d), and they do not
overlap once `StagerAvailable` is the gate. Proceed with the tasks.

---

## Why

- **Business value**: A no-tooled provider (opencode, or any user-defined provider without
  `tooled_flags`) on a shared-file partition currently decomposes to ZERO commits with NO error —
  the user gets no signal that their changeset couldn't be split. BUG-003 turns that silent failure
  into a clear, actionable error naming the remedy.
- **Integration with existing features**: This is the **second half** of BUG-003. S1
  (`P1.M2.T1.S1`, complete) deferred the stager requirement out of `ResolveRoles` so a no-tooled
  provider can reach the FR-M13 disjoint fast-path. S2 (this task) closes the loop: when the
  partition is NOT disjoint (the tooled loop will actually run), the deferred error must fire. S3
  (`P1.M2.T1.S3`, ready) separately extends `FirstTooledProvider` to consider user-defined tooled
  providers — orthogonal to this guard.
- **Problems solved and for whom**: For a user running `stagecoach decompose` with a no-tooled
  provider over a shared-file changeset. Today: silent no-op. After: clear error with a remedy.

---

## What

**User-visible behavior**: `stagecoach decompose` over a shared-file partition with no
stager-capable provider prints, to stderr, a single actionable message and exits 1. Example:
```
stagecoach: decompose: orchestrator failed: this partition shares files across concepts, so a
tooled stager is required, but the configured provider has no tooled_flags and no stager-capable
provider is installed; use a disjoint partition (or a disjoint changeset) or install a tooled
provider (pi or claude)
```
The disjoint fast-path (`runLoopFastPath`) is unaffected — it never enters `runLoop` and never
checks the stager (FR-M13: deterministic `git add`, no stager agent).

### Success Criteria

- [ ] `runLoop` returns an `ErrDecomposeFailed`-wrapped error when `deps.Roles.StagerAvailable` is
      false, BEFORE any freeze/generation work (before `DiffTreeNames` for `tStartPaths`).
- [ ] The error message names the cause and the remedy: "tooled stager", "tooled_flags", a disjoint
      remedy, and "pi or claude".
- [ ] The guard fires ONLY for non-disjoint partitions routed to `runLoop` (the disjoint
      `runLoopFastPath` never checks it).
- [ ] `TestRunLoop_NoStagerAvailable_Errors` (`decompose_test.go:1068`) passes (direct `runLoop`
      call, `StagerAvailable=false`, shared partition → hard error).
- [ ] `TestDecompose_FastPath_TooledFlagsLessProvider/disjoint_succeeds` still passes (disjoint
      partition, bare stager → fast-path bypass, NO error).
- [ ] `TestDecompose_FastPath_TooledFlagsLessProvider/shared_cannot_serve_as_stager` is RE-SCOPED
      (Task 2) to assert the BUG-003 hard-error contract and passes.
- [ ] `TestDecompose_StagerRetryThenEmpty` (`decompose_test.go:2001`) and
      `TestDecompose_StagerRetryThenSuccess` (`decompose_test.go:2149`) still pass (FR-M12d runtime
      retry-then-empty preserved — tooled stager + seam failure + `StagerAvailable: true`).
- [ ] `go test ./internal/decompose/...` is GREEN (all sub-tests).

---

## All Needed Context

### Context Completeness Check

_Before writing this PRP, validated: "If someone knew nothing about this codebase, would they have
everything needed to implement this successfully?"_ — YES. The guard already exists from the
previous attempt; this PRP's job is to (a) validate it against the spec, (b) resolve the one test
regression by re-scoping it, and (c) prevent another halt. Exact file/line references, the spec
distinction, and the test-update recipe are all below.

### Documentation & References

```yaml
# SPEC — the authoritative source of truth for the capability-gap vs runtime-failure distinction
- url: spec/03-generation.md (§13.6.6 "Failure handling within the loop", lines 148-153)
  why: Defines FR-M12d's contract as a RUNTIME failure: "Stager exits non-zero: retry once; on
       second failure treat as empty (FR-M8) and continue." This is the framing that proves FR-M12d
       governs exec failures, NOT capability gaps — so the guard (capability gap) does not conflict.
  critical: The word "exits non-zero" — the stager was INVOKED and returned non-zero. A bare/tooled-
            less stager "simply cannot serve as a stager" (§12.7) — it never gets to "exit."

- url: spec/02-providers.md:535 (§12.7 / §11.5, "The stager role inverts this")
  why: "A provider with empty tooled_flags simply cannot serve as a stager; it can still serve the
       bare roles." This is the CAPABILITY-GAP authority — the bare provider lacks the capability,
       deterministically and permanently. The guard operationalizes this sentence.
  critical: "simply cannot serve" = permanent capability gap, not a transient exec failure.

- url: spec/01-product.md:364 (FR-M13, "File-disjoint staging fast-path")
  why: "Because the fast-path invokes no tooled agent, a provider whose manifest declares no
       tooled_flags ... can decompose a disjoint tree, where it otherwise could not serve the stager
       role at all." Establishes that a no-tooled provider's ONLY valid decompose path is the
       disjoint fast-path; a shared partition is out of reach → the guard's error is the correct
       response (not a silent skip).

# CODE — the guard, the sentinel, the stager invocation, and the exit-code path
- file: internal/decompose/decompose.go (runLoop, lines 518-530)
  why: THE GUARD. Already implemented by the previous attempt. Verify it reads
       `if !deps.Roles.StagerAvailable { ... return ErrDecomposeFailed-wrapped error }` as the
       FIRST statement in runLoop, before `tStartPaths, err := deps.Git.DiffTreeNames(...)`.
  pattern: Mirrors the existing infra-error idiom — `return nil, nil, fmt.Errorf("%w: <msg>",
           ErrDecomposeFailed)`. The guard must fire BEFORE any git work so a zero-Git Deps returns
           the error without dereferencing Git (see TestRunLoop_NoStagerAvailable_Errors).
  gotcha: The guard's message must contain the actionable substrings the test checks:
          "tooled stager", "tooled_flags", "disjoint", "pi or claude". The current message has all
          four — do NOT regress them if you touch the message.

- file: internal/decompose/roles.go (RoleManifests.StagerAvailable, the S1 sentinel)
  why: The sentinel the guard reads. S1 sets `StagerAvailable=false` ONLY when the configured stager
       has nil TooledFlags AND `FirstTooledProvider(installed)==""` (no tooled fallback). In every
       other case (native TooledFlags, or a successful FR-D4 fallback) it is true. This is exactly
       the capability-gap signal the guard needs.
  pattern: See the `stagerAvailable` local + the BUG-003/FR-M13 comment block in ResolveRoles.
  gotcha: `StagerAvailable` is the ZERO-VALUE-false bool. Tests that build RoleManifests DIRECTLY
          (bypassing ResolveRoles) with a TOOLED stub stager MUST set `StagerAvailable: true`
          (dcmAllRoles at decompose_test.go:201 already does). A test with a GENUINELY BARE stager
          correctly leaves it false — that is the capability-gap case the guard fires on.

- file: internal/decompose/stager.go (stageConcept, lines 94-122; ErrStagerFailed line 42)
  why: Shows WHY the capability gap manifests as a runtime error today: Render(mdl,...,RenderTooled)
       fails on a bare manifest (nil TooledFlags) → wrapped as ErrStagerFailed → caught by
       invokeStagerRetry (FR-M12d). The guard pre-empts this path entirely.
  pattern: stageConcept is the tooled, no-retry, no-parse single invocation. It returns
           `fmt.Errorf("%w: render: %v", ErrStagerFailed, rerr)` on a render error and
           `fmt.Errorf("%w: %w", ErrStagerFailed, execErr)` on exec failure.
  critical: The guard fires BEFORE stageConcept is ever called, so the render error for a bare
            manifest is never produced for the capability-gap case. FR-M12d still handles genuine
            exec failures of a TOOLED stager (ErrStagerFailed from line 120, not line 108).

- file: internal/decompose/decompose.go (invokeStagerRetry, ~lines 596-640; the FR-M12d loop)
  why: FR-M12d's retry-once-then-empty. `runOnce()` calls invokeStager→stageConcept; on error (not
       ErrStagerMovedHEAD) it retries once; on second failure it returns nil → tree[i]==prevTree →
       empty-skip. UNCHANGED by this PRP — the guard just ensures a capability gap never reaches it.
  gotcha: ErrStagerMovedHEAD (stager moved HEAD — safety violation) is HARD and bypasses retry. The
          guard's ErrDecomposeFailed is also HARD but fires earlier (before any staging).

- file: internal/cmd/default_action.go (handleDecomposeError, lines 548-555)
  why: The exit-code mapping. The guard's ErrDecomposeFailed-wrapped error is NOT a *RescueError /
       *CASError, so it takes the `exitcode.New(exitcode.Error, err)` arm → main prints
       `stagecoach: <message>`, exit 1. NO CLI CHANGE NEEDED — the actionable message surfaces
       automatically via the existing infra-error path.
  pattern: rescue/CAS → silent exit (loop already printed); planner/safety/infra → printed by main.

- file: internal/decompose/decompose.go (Decompose dispatch, lines 248-258)
  why: Shows WHERE the guard sits in the flow: `if isFileDisjoint(out.Commits) { runLoopFastPath }
       else { runLoop }`. The guard is inside runLoop only — the fast-path never checks the stager.

- file: internal/decompose/decompose_test.go (the tests to touch — see Tasks)
  why: TestUpdate recipe + the FR-M12d-preservation proof.
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
├── decompose.go        # runLoop (guard @518), runLoopFastPath, Decompose dispatch, invokeStagerRetry
├── roles.go            # RoleManifests.StagerAvailable (S1 sentinel), ResolveRoles
├── stager.go           # stageConcept (ErrStagerFailed wrapping), ErrStagerMovedHEAD, ErrFreezeViolation
└── decompose_test.go   # TestDecompose_FastPath_TooledFlagsLessProvider (@4466), TestRunLoop_NoStagerAvailable_Errors (@1068),
                        #   TestDecompose_StagerRetryThenEmpty (@2001), dcmAllRoles (@201)
```

### Desired Codebase tree with files to be MODIFIED (not added — no new files)

```bash
internal/decompose/
├── decompose.go        # VERIFY ONLY — guard already present from previous attempt (lines 518-530)
└── decompose_test.go   # MODIFY — re-scope shared_cannot_serve_as_stager (@4522); KEEP TestRunLoop_NoStagerAvailable_Errors (@1068)
```

> **No new files are created.** The guard code and `TestRunLoop_NoStagerAvailable_Errors` already
> exist from the previous attempt. This PRP's net code change is: (1) verify the guard, (2) re-scope
> exactly one test. If the previous attempt's changes are NOT present in your checkout, Task 0
> (below) restores them from the research/ notes.

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL: StagerAvailable is a bool with zero-value false. Tests that construct RoleManifests
// DIRECTLY (not via ResolveRoles) must set it explicitly. dcmAllRoles sets it true (its stub is
// tooled). A test with a GENUINELY bare stager (nil TooledFlags) correctly leaves it false.
// The previous attempt's "31 broken tests" were all TOOLED-stub literals missing StagerAvailable:true
// — that fix is correct and must stay. Do NOT "fix" shared_cannot_serve_as_stager by setting
// StagerAvailable:true; its stager is genuinely bare (the whole point).

// CRITICAL: The guard MUST be the FIRST statement in runLoop, before `tStartPaths, err := deps.Git.
// DiffTreeNames(...)`. TestRunLoop_NoStagerAvailable_Errors passes a zero-value Deps (nil Git) and
// relies on the guard firing before any Git dereference. If you move the guard after the DiffTreeNames
// call, that test will panic (nil pointer) and the "before any freeze/generation work" contract breaks.

// GOTCHA: The guard's error message is asserted by substring in TWO tests
// (TestRunLoop_NoStagerAvailable_Errors + the re-scoped shared_cannot_serve_as_stager). It must
// contain: "tooled stager", "tooled_flags", "disjoint", "pi or claude". Keep all four.

// GOTCHA: Decompose's flow to reach runLoop requires a real repo + real freeze (FreezeWorkingTree)
// + a planner partition whose files share ≥1 path (isFileDisjoint==false). The re-scoped test keeps
// its existing realistic store.py setup for that reason — the guard fires at runLoop entry, AFTER
// Decompose has already frozen the tree and run the planner, but BEFORE any per-concept staging.
```

---

## Implementation Blueprint

### Data models and structure

No new data models. The sentinel (`RoleManifests.StagerAvailable`, bool) was added by S1
(`internal/decompose/roles.go`). The guard consumes it. The error is a `fmt.Errorf("%w: ...",
ErrDecomposeFailed)` (the existing orchestrator-infra sentinel, `decompose.go:46`).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0 (ONLY if the previous attempt's changes are absent): RESTORE the guard + its test
  - CONDITION: run `rg -n "StagerAvailable" internal/decompose/decompose.go`. If there is NO match
    at the top of runLoop (~line 518), the previous attempt's guard is absent — restore it.
  - IMPLEMENT (internal/decompose/decompose.go, as the FIRST statement inside runLoop, before the
    `tStartPaths, err := deps.Git.DiffTreeNames(...)` line):
        if !deps.Roles.StagerAvailable {
            return nil, nil, fmt.Errorf("%w: this partition shares files across concepts, so a "+
                "tooled stager is required, but the configured provider has no tooled_flags and "+
                "no stager-capable provider is installed; use a disjoint partition (or a disjoint "+
                "changeset) or install a tooled provider (pi or claude)", ErrDecomposeFailed)
        }
  - ALSO RESTORE TestRunLoop_NoStagerAvailable_Errors (internal/decompose/decompose_test.go) per the
    research/ notes if absent — see research/test_recipe.md.
  - NOTE: In the likely case the previous attempt's changes ARE present (guard at decompose.go:518,
    test at decompose_test.go:1068, dcmAllRoles StagerAvailable:true at decompose_test.go:201), SKIP
    this task and go to Task 1.
  - VERIFY: `go build ./internal/decompose/...` succeeds.

Task 1: VERIFY the guard against the contract
  - READ internal/decompose/decompose.go runLoop (~lines 518-530). Confirm:
      (a) The `if !deps.Roles.StagerAvailable` check is the FIRST statement (before DiffTreeNames).
      (b) It returns `nil, nil, fmt.Errorf("%w: ...", ErrDecomposeFailed)`.
      (c) The message contains all four substrings: "tooled stager", "tooled_flags", "disjoint",
          "pi or claude".
      (d) The comment cites BUG-003/FR-M13 + the capability-gap rationale (keep it accurate).
  - If any of (a)-(d) is off, FIX it (small edit). Do NOT change the message's actionable terms.
  - VERIFY: `go vet ./internal/decompose/...` and `gofmt -l internal/decompose/decompose.go` (empty).

Task 2 (THE CONFLICT RESOLUTION): RE-SCOPE shared_cannot_serve_as_stager to the BUG-003 contract
  - FILE: internal/decompose/decompose_test.go, the sub-test at ~line 4522 inside
    TestDecompose_FastPath_TooledFlagsLessProvider (the `shared_cannot_serve_as_stager` t.Run block).
  - WHY: This is the ONLY test where a BARE stager meets a SHARED partition (confirmed by exhaustive
    audit — all other shared-partition tests use tooledStubManifest + StagerAvailable:true). It
    currently asserts FR-M12d's silent empty-skip (err==nil, 0 commits, retry log). After BUG-003,
    the capability-gap case is owned by the guard and MUST assert the hard error instead.
  - KEEP UNCHANGED: the realistic setup (store.py base→tStart, 2-concept shared partition via
    plannerJSON, bare stager `stubtest.Manifest(bin, stubtest.Options{Out: ""})`, Commits=2). This
    setup faithfully reaches runLoop (freeze + planner + isFileDisjoint==false), where the guard
    fires. The Stager manifest STAYS bare (do NOT add StagerAvailable:true — that would defeat the
    test's purpose and misrepresent the sentinel).
  - REPLACE the assertions block. Old assertions (DELETE):
        if err != nil { t.Fatalf("FR-M12d swallows the stager error ...") }
        if len(result.Commits) != 0 { ... }
        logStr := logBuf.String(); if !strings.Contains(logStr, "stager failed twice") ...
    New assertions (the BUG-003 contract):
        if err == nil {
            t.Fatal("BUG-003: a bare/tooled-less stager on a shared partition must hard-error; got nil")
        }
        if !errors.Is(err, ErrDecomposeFailed) {
            t.Errorf("error not ErrDecomposeFailed-wrapped: %v", err)
        }
        for _, want := range []string{"tooled stager", "tooled_flags", "disjoint", "pi or claude"} {
            if !strings.Contains(err.Error(), want) {
                t.Errorf("error %q missing %q", err.Error(), want)
            }
        }
        if len(result.Commits) != 0 {
            t.Errorf("Commits len = %d, want 0 (guard fires before any staging)", len(result.Commits))
        }
  - The `logBuf` / Verbose capture is no longer needed for assertions (the guard fires before the
    retry log). You MAY drop the `var logBuf bytes.Buffer` + `ui.NewVerbose(&logBuf, true)` and use a
    plain Verbose (or nil), OR keep them harmlessly. Prefer dropping logBuf and using a plain
    `Verbose: ui.NewVerbose(io.Discard, true)` (or the dcmDeps default) to keep the test honest about
    what it asserts. If you keep logBuf, do NOT assert on its contents.
  - UPDATE the sub-test's doc comment to reflect the new contract. Replace the FR-M12d-swallow
    framing with (paraphrase, keep accurate):
        // --- Sub-case: shared partition + BARE stager → BUG-003 hard error (capability gap). ---
        // A TooledFlags-less provider "simply cannot serve as a stager" (spec §12.7). BUG-003 turns
        // the old silent empty-skip into a clear, actionable error: runLoop's StagerAvailable guard
        // fires BEFORE any staging (it checks the S1 sentinel, not exec output), so Decompose returns
        // ErrDecomposeFailed naming the remedy. The guard is ORTHOGONAL to FR-M12d: FR-M12d's
        // retry-then-empty governs RUNTIME failures of a TOOLED stager (StagerAvailable=true), which
        // is covered separately by TestDecompose_StagerRetryThenEmpty. The disjoint sub-case above
        // proves the fast-path bypass; this proves the capability-gap hard error.
  - ALSO update the Case-8 HEADER comment block (~line 4454) where it claims the shared case is
    "swallowed into an empty-skip for BOTH concepts" — reword to "produces a BUG-003 hard error
    (capability gap: a tooled-less provider cannot serve as a stager, spec §12.7)". Keep the
    disjoint_succeeds description intact.
  - FOLLOW pattern: the assertion style + substring check mirror TestRunLoop_NoStagerAvailable_Errors
    (decompose_test.go:1068) — errors.Is(err, ErrDecomposeFailed) + the four substrings.
  - NAMING: keep the sub-test name `shared_cannot_serve_as_stager` (it still describes the scenario
    accurately — a bare provider cannot serve as a stager — only the asserted outcome changed).
  - DEPENDENCIES: needs `errors` + `strings` imported (both already imported in decompose_test.go).
  - PLACEMENT: in-place edit of the existing t.Run block; do not move it or rename the parent test.

Task 3: VERIFY disjoint_succeeds is unaffected
  - READ the `disjoint_succeeds` sub-test (~line 4470). It uses a bare stager on a DISJOINT partition
    (a.txt/b.txt/c.txt) → routes to runLoopFastPath → guard never fires → 3 commits, no error.
    CONFIRM it has NO `StagerAvailable` set (correct — the fast-path doesn't check it) and that it
    still asserts err==nil + 3 commits. DO NOT MODIFY it. (It is the positive proof of the FR-M13
    fast-path bypass for a no-tooled provider.)
  - VERIFY: `go test ./internal/decompose/ -run TestDecompose_FastPath_TooledFlagsLessProvider/disjoint_succeeds -v` PASSES.

Task 4: VERIFY FR-M12d runtime-failure tests are preserved
  - READ TestDecompose_StagerRetryThenEmpty (decompose_test.go:2001) and
    TestDecompose_StagerRetryThenSuccess (decompose_test.go:2149). Confirm BOTH use
    `tooledStubManifest` for the stager + `StagerAvailable: true` + a RUNTIME seam failure
    (deps.stager returning errors.New(...) or exiting non-zero). The guard does NOT fire for these
    (StagerAvailable is true), so FR-M12d's retry-then-empty runs unchanged. DO NOT MODIFY them.
  - VERIFY: `go test ./internal/decompose/ -run 'TestDecompose_StagerRetryThen(Empty|Success)' -v` PASSES.
  - These tests are the proof that this PRP did NOT regress FR-M12d. If either fails, the most likely
    cause is a missing `StagerAvailable: true` on its RoleManifests literal — add it (the stub IS
    tooled). Report in the issue-feedback if they fail for any other reason.

Task 5: RUN THE FULL DECOMPOSE SUITE — the mandatory green gate
  - RUN: `go test ./internal/decompose/...` — MUST be GREEN (all packages, all sub-tests).
  - If any OTHER test fails with a "StagerAvailable" or runLoop-guard symptom, check whether that
    test builds RoleManifests directly with a TOOLED stub stager but omits StagerAvailable:true — if
    so, add `StagerAvailable: true` (faithful fix, mirrors dcmAllRoles at decompose_test.go:201). Do
    NOT add it to any test whose stager is genuinely BARE (that is the capability-gap case).
  - This is the gate the previous attempt violated. It MUST be green before declaring success.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the runLoop guard (capability-gap hard error). It is the FIRST statement in runLoop.
func runLoop(ctx context.Context, deps Deps, concepts []prompt.PlannerCommit, baseTree, tStart, preRunHEAD string, isUnborn bool) ([]CommitResult, []ChainEntry, error) {
    // BUG-003/FR-M13: ResolveRoles (S1) defers the stager requirement so a no-tooled provider can
    // reach the file-disjoint fast-path. This runLoop runs ONLY for a shared-file partition, which
    // genuinely requires a tooled stager (FR-M5). If StagerAvailable=false (capability gap: the
    // configured stager is bare/tooled-less with no tooled fallback — spec §12.7 "simply cannot
    // serve as a stager"), fail fast BEFORE any freeze/generation work. ORTHOGONAL to FR-M12d,
    // which governs RUNTIME failures ("stager exits non-zero", spec §13.6.6) of a TOOLED stager
    // (StagerAvailable=true) and is handled by invokeStagerRetry below.
    if !deps.Roles.StagerAvailable {
        return nil, nil, fmt.Errorf("%w: this partition shares files across concepts, so a tooled "+
            "stager is required, but the configured provider has no tooled_flags and no "+
            "stager-capable provider is installed; use a disjoint partition (or a disjoint changeset) "+
            "or install a tooled provider (pi or claude)", ErrDecomposeFailed)
    }
    // ... rest of runLoop (tStartPaths, launch/publish/invokeStagerRetry) unchanged ...

// PATTERN: the re-scoped assertion block (mirrors TestRunLoop_NoStagerAvailable_Errors @1068).
// The guard fires at runLoop entry, after Decompose froze the tree + ran the planner, but before
// any per-concept staging — so result.Commits is empty and err is the actionable hard error.
if err == nil {
    t.Fatal("BUG-003: a bare/tooled-less stager on a shared partition must hard-error; got nil")
}
if !errors.Is(err, ErrDecomposeFailed) {
    t.Errorf("error not ErrDecomposeFailed-wrapped: %v", err)
}
for _, want := range []string{"tooled stager", "tooled_flags", "disjoint", "pi or claude"} {
    if !strings.Contains(err.Error(), want) {
        t.Errorf("error %q missing %q", err.Error(), want)
    }
}
```

### Integration Points

```yaml
CODE (internal/decompose/decompose.go):
  - location: "runLoop — the `if !deps.Roles.StagerAvailable` guard (FIRST statement, ~line 518)"
  - pattern: "return nil, nil, fmt.Errorf(\"%w: <msg>\", ErrDecomposeFailed) — the infra-error idiom"
  - preserve: "the disjoint dispatch (isFileDisjoint→runLoopFastPath) is unchanged; the fast-path
               never checks the stager (FR-M13)."

TESTS (internal/decompose/decompose_test.go):
  - modify: "shared_cannot_serve_as_stager (~line 4522) — re-scope to assert ErrDecomposeFailed + 4 substrings"
  - keep-green: "TestRunLoop_NoStagerAvailable_Errors (1068), disjoint_succeeds (4470),
                 TestDecompose_StagerRetryThenEmpty (2001), TestDecompose_StagerRetryThenSuccess (2149),
                 dcmAllRoles StagerAvailable:true (201)"

CLI / EXIT CODES:
  - none: "handleDecomposeError (internal/cmd/default_action.go:548) already maps non-rescue/non-CAS
           decompose errors to exitcode.Error → main prints `stagecoach: <msg>`, exit 1. The guard's
           ErrDecomposeFailed-wrapped error takes that arm automatically. NO CLI CHANGE."

CONFIG / DATABASE / ROUTES: none.
```

---

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# After any edit — fix before proceeding.
go build ./internal/decompose/...
go vet ./internal/decompose/...
gofmt -w internal/decompose/decompose.go internal/decompose/decompose_test.go
gofmt -l internal/decompose/   # must print NOTHING

# Expected: zero output from all four. gofmt -l must be empty (no diffs).
```

### Level 2: Unit Tests (Component Validation)

```bash
# The guard test (direct runLoop call) — MUST PASS.
go test ./internal/decompose/ -run TestRunLoop_NoStagerAvailable_Errors -v

# The re-scoped + disjoint sub-cases — BOTH MUST PASS.
go test ./internal/decompose/ -run TestDecompose_FastPath_TooledFlagsLessProvider -v

# FR-M12d runtime-failure preservation — BOTH MUST PASS (proof this PRP didn't regress FR-M12d).
go test ./internal/decompose/ -run 'TestDecompose_StagerRetryThen(Empty|Success)' -v

# S1 sentinel tests (roles_test.go) — MUST PASS (the sentinel the guard reads).
go test ./internal/decompose/ -run TestResolveRoles -v

# Expected: all PASS. If TestDecompose_FastPath_TooledFlagsLessProvider/shared_cannot_serve_as_stager
# still asserts the OLD contract, you missed Task 2 — re-read it.
```

### Level 3: Integration Testing (Full Package)

```bash
# THE MANDATORY GREEN GATE the previous attempt violated.
go test ./internal/decompose/... -v

# Also run the provider package (the FR-D4 fallback S1 relies on).
go test ./internal/provider/...

# Expected: ALL GREEN. If a test fails with a runLoop-guard symptom, see Task 5's
# StagerAvailable:true guidance. Report any failure NOT explained by that guidance.
```

### Level 4: CLI Surface Verification (the user-facing contract)

```bash
# Verify the actionable error surfaces at the CLI level via the existing exit-code path.
# This is a read-only CONFIRMATION (no CLI code changed) — check handleDecomposeError maps the guard:
rg -n "ErrDecomposeFailed" internal/cmd/default_action.go   # should show the infra arm prints the msg
# The guard's error is non-rescue/non-CAS → exitcode.Error → main prints "stagecoach: <msg>" → exit 1.
# (A full CLI integration test is out of scope for this unit-level PRP; the exit-code path is
#  unchanged infra plumbing verified by the existing decompose CLI tests in pkg/stagecoach.)
```

---

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./internal/decompose/...` clean
- [ ] `go vet ./internal/decompose/...` clean
- [ ] `gofmt -l internal/decompose/` empty
- [ ] `go test ./internal/decompose/...` GREEN (the mandatory gate)

### Feature Validation
- [ ] The runLoop guard is the FIRST statement in runLoop (before DiffTreeNames) and reads
      `deps.Roles.StagerAvailable`
- [ ] The guard returns `ErrDecomposeFailed`-wrapped error with all four actionable substrings
- [ ] The guard fires ONLY on the non-disjoint path (disjoint → runLoopFastPath, never checked)
- [ ] `TestRunLoop_NoStagerAvailable_Errors` PASSES
- [ ] `TestDecompose_FastPath_TooledFlagsLessProvider/disjoint_succeeds` PASSES (fast-path bypass)
- [ ] `TestDecompose_FastPath_TooledFlagsLessProvider/shared_cannot_serve_as_stager` is RE-SCOPED
      (asserts ErrDecomposeFailed + 4 substrings + 0 commits) and PASSES
- [ ] `TestDecompose_StagerRetryThenEmpty` + `TestDecompose_StagerRetryThenSuccess` PASS (FR-M12d
      runtime retry-then-empty preserved)

### Spec-Alignment Validation (the anti-halt checklist)
- [ ] The capability-gap (guard) vs runtime-failure (FR-M12d) distinction is reflected in the code
      comment on the guard
- [ ] NO change to `spec/SPEC.md` or any spec/*.md (read-only — AGENTS.md rule 2)
- [ ] NO change to FR-M12d's runtime retry-then-empty behavior (only the conflated test's assertions
      changed, to match BUG-003's corrected capability-gap outcome)
- [ ] The re-scoped test's doc comment cites §12.7 (capability) + §13.6.6 (runtime) to prevent a
      future implementer re-deriving the false "conflict"

### Code Quality
- [ ] The guard's comment is accurate (BUG-003/FR-M13 + the capability-vs-runtime distinction)
- [ ] The re-scoped test is self-documenting (comments explain WHY the outcome changed)
- [ ] No `StagerAvailable:true` added to any genuinely-bare stager (would misrepresent the sentinel)

---

## Anti-Patterns to Avoid

- ❌ **Do NOT halt on the FR-M12d interaction.** It is resolved (see the CRITICAL section). The guard
  handles capability gaps (StagerAvailable=false); FR-M12d handles runtime failures (tooled stager
  exits non-zero). They do not overlap once `StagerAvailable` is the gate.
- ❌ **Do NOT "fix" `shared_cannot_serve_as_stager` by setting `StagerAvailable: true`.** Its stager
  is GENUINELY bare (nil TooledFlags) — that is the whole point of the test. Setting the sentinel
  true would both defeat the test's purpose AND misrepresent S1's definition.
- ❌ **Do NOT delete `shared_cannot_serve_as_stager`.** Its purpose (prove a tooled-less provider
  cannot stage a shared file) is preserved — only the asserted OUTCOME changes (silent-skip → clear
  error), per BUG-003.
- ❌ **Do NOT move the guard after `tStartPaths, err := deps.Git.DiffTreeNames(...)`.** It must be the
  first statement so a zero-Git Deps returns the error without dereferencing Git.
- ❌ **Do NOT change the four actionable substrings** in the guard message ("tooled stager",
  "tooled_flags", "disjoint", "pi or claude") — two tests assert them by substring.
- ❌ **Do NOT modify `spec/SPEC.md` or any spec file** — this is a bug fix + test alignment, not a
  spec change (AGENTS.md rule 2).
- ❌ **Do NOT introduce a new error sentinel** — reuse `ErrDecomposeFailed` (the orchestrator-infra
  sentinel) so the existing `handleDecomposeError` exit-code path works unchanged.

---

## Confidence Score

**9/10** for one-pass implementation success. The guard already exists from the previous attempt and
compiles/passes its own test; the ONLY remaining work is re-scoping one test (Task 2) whose exact
old→new assertion blocks are specified verbatim. The 1-point reservation is for the possibility that
the previous attempt's changes are partially absent in the checkout (Task 0 handles that). The
previous halt was a reasoning failure (not analyzing the capability-vs-runtime distinction against
the spec), not an implementation difficulty — this PRP front-loads the spec distinction to prevent
a repeat.