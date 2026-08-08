name: "P1.M1.T2.S2 — Apply EditMessage in runLoopFastPath's serial publish loop before publishCommit (BUG-001 step 2/2)"
description: >
  Second half of the BUG-001 fix: in runLoopFastPath's SERIAL publish loop (internal/decompose/
  decompose.go:760-840), insert a `generate.EditMessage` block BETWEEN the `if res.err != nil`
  error-handling close (~795) and `publishCommit` (~799), gated on `deps.Config.Edit`. This restores
  --edit on the file-disjoint fast-path in a CONCURRENCY-SAFE way: the serial loop runs one editor at
  a time (FR-E4), so the shared `.git/STAGECOACH_EDITMSG` file is no longer raced by N goroutines.
  Mirrors generateMessage's existing EditMessage site (message.go:255-262) 1:1 (treeA=sc.prevTree,
  treeB=sc.tree, best-effort nameStatus, propagate eErr as a NON-RESCUE hard error via
  `drainMsgs(inflight[i+1:]); return commits, nil, eErr`). Plus the BUG-001 regression test (cfg.Edit=true
  + GIT_EDITOR=true no-op on 2+ disjoint concepts; each commit's subject == its own concept's message).
  CONSUMES generateMessageCore from P1.M1.T1.S1 + the launch-closure switch from P1.M1.T2.S1 (S1 must
  land FIRST — else EditMessage runs twice). Does NOT touch the launch closure (S1), message.go,
  EditMessage/finalize.go (FROZEN), runLoop's serial loop (~501), publishCommit, cross-concept dedupe
  (P1.M2.T1.S1), or docs.

---

## Goal

**Feature Goal**: Restore `--edit` on the file-disjoint fast-path CONCURRENCY-SAFE (BUG-001 step 2 of 2)
by applying `generate.EditMessage` in runLoopFastPath's SERIAL publish loop — one editor at a time — so
N disjoint concepts no longer race on the single shared `.git/STAGECOACH_EDITMSG` file. After S1
(P1.M1.T2.S1) removed EditMessage from the concurrent goroutine (switching to generateMessageCore),
this task re-applies it at the serialization point (FR-E4: "--edit gates each commit's message before
its already-serialized publication").

**Deliverable**:
1. **internal/decompose/decompose.go** — a new `if deps.Config.Edit { ... }` block in runLoopFastPath's
   serial publish loop, inserted after the `if res.err != nil` error-handling block close (~line 795)
   and BEFORE `// Publish in CAS order` / `publishCommit` (~line 797/799). The block calls
   `generate.EditMessage(ctx, res.msg, deps.Config, generate.EditContext{Git: deps.Git, TreeSHA: sc.tree,
   NameStatus: nameStatus})`, assigns the result to `res.msg`, and on error does
   `drainMsgs(inflight[i+1:]); return commits, nil, eErr`.
2. **internal/decompose/decompose_test.go** — the BUG-001 regression test: 2+ disjoint concepts,
   `cfg.Edit=true`, `GIT_EDITOR=true` (no-op), `dcmMessageMatchManifest` distinct messages; assert each
   commit's subject matches its OWN concept's generated message (no cross-contamination).

**Success Definition**:
- With `--edit` on the fast-path (2+ disjoint concepts), exactly ONE editor runs at a time (the serial
  loop serializes them); each published commit carries its OWN concept's edited message — never another
  concept's (BUG-001 fixed; FR-E4 honored).
- The EditMessage block is byte-for-byte the same logic as generateMessage's existing site
  (message.go:255-262): best-effort `DiffTreeNameStatus(sc.prevTree, sc.tree)`, `EditContext{Git,
  TreeSHA: sc.tree, NameStatus}`, assign `res.msg`.
- An `ErrEmptyMessage` (editor emptied the file) or editor-abort error from EditMessage propagates as a
  NON-RESCUE hard error (`return commits, nil, eErr`), NOT wrapped in RescueError — matching
  generateMessage's current behavior and the CLI's exit-1 mapping.
- `drainMsgs(inflight[i+1:])` runs on the error path (no goroutine leak), matching the existing
  error paths in the loop.
- `go build ./...`, `go test ./internal/decompose/...`, `make test`, `make lint` pass.

## User Persona (if applicable)

**Target User**: A developer running `stagecoach --edit` on a dirty, un-staged tree whose planner
partition is pairwise file-disjoint (the common case for cleanly separated changes, ≥2 concepts).

**Use Case**: Multi-commit decomposition with `--edit`: the user wants to review/edit each concept's
commit message before it lands. Pre-fix (BUG-001), N editors opened at once on one shared EDITMSG file
and a concept silently received another's message. Post-fix, the editor opens once per concept,
serially, in publish order — each edit gates its own commit.

**User Journey**: `stagecoach --edit` → fast-path launches N concurrent generations (no editor in the
goroutine, per S1) → serial publish loop, for each concept: receive `res` → (if `cfg.Edit`) open the
editor on this concept's message → `res.msg = edited` → publishCommit. One editor at a time.

**Pain Points Addressed**: BUG-001 (Critical) — silent cross-contamination of commit messages via the
shared-EDITMSG race; N editors fighting in one terminal; a published commit whose message describes a
DIFFERENT diff. This task makes `--edit` correct and concurrency-safe on the fast-path.

## Why

- **BUG-001 (Critical, step 2 of 2)**: the fix's design (architecture/fix_design.md Part 2) moves
  EditMessage out of the concurrent goroutine (S1) and into the serial publish loop (this task). The
  serial loop is the natural serialization point — it already publishes one commit at a time in CAS
  order (FR-M7), so running the editor there gives one editor at a time for free (FR-E4 "serialized").
- **Why mirror generateMessage's site exactly**: message.go:255-262 is the proven EditMessage call
  (best-effort nameStatus, EditContext shape, error propagation). The serial-loop block differs ONLY in
  (a) the treeA/treeB source (`sc.prevTree`/`sc.tree` from the stagedConcept, vs the closure's params)
  and (b) the error return shape (`drainMsgs(inflight[i+1:]); return commits, nil, eErr` vs
  `return "", err`). Reusing the proven logic minimizes risk.
- **Bounded**: one insertion block (≈12 lines) + one regression test. EditMessage's signature is FROZEN;
  generateMessageCore (the goroutine's new call) is from S1. No struct/API/new-logic changes.
- **Complementary, non-overlapping**: S1 owns the launch closure; THIS task owns the serial loop's
  EditMessage. runLoop's serial loop (~501-548) is UNAFFECTED (its 1-deep overlap already serializes
  editing). Cross-concept dedupe (P1.M2.T1.S1) adds a separate block BEFORE this one.

## What

**User-visible behavior**: `--edit` on the file-disjoint fast-path now works correctly — one editor per
concept, in publish order, each concept's commit carrying its own edited message. (After S1 alone,
`--edit` was transiently skipped on the fast-path; this task restores it.)

**Technical change (one block + one test):**
1. The serial-loop EditMessage block — `if deps.Config.Edit { nameStatus, _ := DiffTreeNameStatus(...);
   edited, eErr := EditMessage(...); if eErr != nil { drainMsgs(...); return commits, nil, eErr };
   res.msg = edited }`.
2. The regression test — `cfg.Edit=true`, `GIT_EDITOR=true` (no-op), 2+ disjoint concepts, distinct
   messages; assert no cross-contamination.

### Success Criteria
- [ ] runLoopFastPath's serial publish loop has an EditMessage block between the res.err close and publishCommit
- [ ] The block is gated on `deps.Config.Edit` (no-op when false — the byte-identity regression invariant)
- [ ] `DiffTreeNameStatus(ctx, sc.prevTree, sc.tree)` feeds `EditContext{NameStatus}` (best-effort, error ignored)
- [ ] `EditContext{Git: deps.Git, TreeSHA: sc.tree, NameStatus: nameStatus}` (treeB = sc.tree)
- [ ] On success: `res.msg = edited` (so publishCommit uses the edited message)
- [ ] On error: `drainMsgs(inflight[i+1:]); return commits, nil, eErr` (NON-RESCUE hard error, no leak)
- [ ] runLoop's serial loop (~501-548) is UNCHANGED (its 1-deep overlap already serializes editing)
- [ ] BUG-001 regression test passes: each commit's subject == its own concept's message (no cross-contamination)
- [ ] `go build ./...`, `go test ./internal/decompose/...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact insertion point (with the unique anchor text), the verbatim block to insert (from
the architecture design + the message.go mirror), the stagedConcept field semantics (tree/prevTree),
the EditContext shape, the error-handling discipline (non-rescue hard error + drainMsgs), the test
harness to clone (TestRunLoopFastPath_ConcurrentPublish) with the exact cfg.Edit + GIT_EDITOR recipe,
the prerequisite chain (S1 + P1.M1.T1.S1), and the scope fences are all enumerated below.

### Documentation & References

```yaml
- file: internal/decompose/decompose.go
  why: "THE change site. runLoopFastPath's serial publish loop @760-840. The `if res.err != nil` block
        closes @~795; `// Publish in CAS order` comment @~797; `newSHA, err := publishCommit(...)` @~799.
        INSERT the EditMessage block in the blank line between the `}` (@795) and the `// Publish in CAS
        order` comment. stagedConcept struct @659-663 (sc.tree=treeB, sc.prevTree=treeA). drainMsgs @843.
        msgOut @109 (value type — res.msg=edited mutates the local copy). The loop header @762
        `for i, ch := range inflight` — inflight[i+1:] is the drain slice."
  pattern: >
    // the insertion anchor (INSERT the block in the blank line between the `}` and the comment):
    		drainMsgs(inflight[i+1:])
    		return commits, nil, res.err // HARD (ErrMessageFailed-wrapped infra) — propagate
    	}

    	// Publish in CAS order: parent = prevSHA (CAS expected-old). publishCommit runs hooks +
    	newSHA, err := publishCommit(ctx, deps, res.treeB, prevSHA, res.msg)
  critical: "The PRIMARY unique anchor is the comment `// Publish in CAS order: parent = prevSHA (CAS
    expected-old). publishCommit runs hooks +` (@795) — grep returns EXACTLY ONE hit (verified).
    INSERT the block in the blank line immediately BEFORE that comment (after the `}` closing the
    `if res.err != nil` block). ⚠️ NOTE: the line `return commits, nil, res.err // HARD (ErrMessageFailed-
    wrapped infra) — propagate` (@792) is NOT unique by itself — `HARD (ErrMessageFailed-wrapped infra)
    — propagate` ALSO appears in runLoop's loop (@546, as bare `return res.err`). Disambiguate by (a) the
    `commits, nil,` prefix (fast-path only) AND (b) the immediately-following `// Publish in CAS order`
    comment (fast-path only — runLoop @548 goes straight to publishCommit with an inline `// parentSHA =
    prevSHA` comment, no `// Publish in CAS order` block). Do NOT blind-edit by line number (S1's parallel
    edit to the launch closure ~735-744 may shift lines). Do NOT touch runLoop's serial loop (~501-548) —
    it is byte-similar but unaffected (1-deep overlap serializes editing there)."

- file: internal/decompose/message.go
  why: "THE mirror. generateMessage's EditMessage site @255-262 is the proven call to clone 1:1:
        `nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, treeA, treeB)` then
        `generate.EditMessage(ctx, msg, deps.Config, generate.EditContext{Git: deps.Git, TreeSHA: treeB,
        NameStatus: nameStatus})`. In the serial loop treeA=sc.prevTree, treeB=sc.tree. The `nameStatus, _ :=`
        (ignored error) is INTENTIONAL best-effort — mirror it."
  pattern: >
    // message.go @255-262 (the mirror):
    	nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, treeA, treeB) // best-effort; "" on err
    	msg, err = generate.EditMessage(ctx, msg, deps.Config, generate.EditContext{Git: deps.Git, TreeSHA: treeB, NameStatus: nameStatus})
    	if err != nil {
    		return "", err // ErrEmptyMessage → propagates to runLoop's FR-M12 handling
    	}
  critical: "generateMessageCore (the goroutine's call after S1) is @80 in this file — CONSUMED (read-only).
    Do NOT edit message.go (P1.M1.T1.S1 owns it). The serial-loop block differs from this mirror ONLY in
    treeA/treeB source (sc.prevTree/sc.tree) and the error return (drainMsgs + return commits, nil, eErr)."

- file: internal/generate/finalize.go
  why: "EditMessage @67 (FROZEN signature: `func EditMessage(ctx, msg string, cfg config.Config, editCtx
        EditContext) (string, error)`). EditContext struct @50-56 (`Git git.Git; TreeSHA string; NameStatus
        string`). ErrEmptyMessage @44 (sentinel — editor returned empty). The no-op guard @70
        (`if !cfg.Edit { return msg, nil }`) is why the block is safe to gate on deps.Config.Edit."
  pattern: "EditMessage returns: msg,nil (no-op if !cfg.Edit); ErrEmptyMessage (empty after strip);
    fmt.Errorf('--edit: ...') wrapped errors (git dir, write, editor abort via :cq, read)."
  critical: "EditMessage is FROZEN — do NOT change its signature or behavior. NONE of its return values
    is a *RescueError, so the serial-loop error path propagates eErr DIRECTLY (not errors.As &re, not
    wrapped in RescueError). ErrEmptyMessage → exit 1 (CLI mapping), editor abort → exit 1. This task
    only ADDS A CALL SITE."

- file: internal/decompose/decompose_test.go
  why: "The test harness to clone. TestRunLoopFastPath_ConcurrentPublish @3150 — the full fast-path test:
        temp repo (dcmInitRepo), base commit (dcmCommitRaw), disjoint dirty files (dcmWriteFile),
        g.FreezeWorkingTree → tStart, preRunHEAD=dcmHeadSHA, dcmMessageMatchManifest (concurrency-safe
        per-concept messages), Deps{Git, Config: config.Defaults(), Roles, Verbose}, runLoopFastPath(...)
        direct call, assert on commits[i].Subject. dcmMessageMatchManifest @147. Imports: config @18,
        stubtest @23 (already present)."
  pattern: >
    // the BUG-001 test skeleton (clone of TestRunLoopFastPath_ConcurrentPublish + cfg.Edit recipe):
    	t.Setenv("GIT_EDITOR", "true")   // NO-OP editor (preserves each concept's written message)
    	... deps := Deps{Git: g, Config: config.Defaults(), Roles: roles, Verbose: ...}
    	deps.Config.Edit = true
    	commits, _, err := runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)
    	// assert commits[0].Subject=="feat: add a" AND commits[1].Subject=="feat: add b"
  critical: "Use GIT_EDITOR=true (NOT stubtest.BuildEditor) for the no-op case — the stubeditor is
    SINGLE-response (writes the same STAGECOACH_EDITOR_MSG every time); with 2 concepts both would get
    the SAME edited message, FAILING the per-concept assertion. `true` exits 0 without modifying the
    file, so EditMessage reads back what it wrote (each concept's OWN message). deps.Config.Edit must be
    set TRUE (config.Defaults() has Edit=false)."

- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/architecture/fix_design.md
  why: "Part 2 specifies this exact insertion (the serial-loop EditMessage block, verbatim) + the error
        handling (drainMsgs + return commits, nil, eErr). Read it to confirm the design + the comment text."
  section: "Part 2: BUG-001 Fix — EditMessage in serial publish loop (Modified serial publish loop)"

- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/architecture/test_strategy.md
  why: "The BUG-001 Regression Test section specifies this exact test: 2+ disjoint concepts, distinct
        dcmMessageMatchManifest messages, GIT_EDITOR=true (no-op), cfg.Edit=true; assert each commit's
        subject == its own message. Also the cfg.Edit recipe (from generate_test.go:836) + why the no-op
        editor is required (single-response stub limitation)."
  section: "BUG-001 Regression Test + The cfg.Edit recipe"

- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/P1M1T2S1/PRP.md
  why: "S1 is the CONTRACT: it switches the launch closure to generateMessageCore(ctx, deps, treeA,
        treeB, nil) (no EditMessage in the goroutine). THIS task depends on S1 landing FIRST — without
        S1, EditMessage would run BOTH in the goroutine (via generateMessage) AND in the serial loop
        (double-edit). S1 also rewrites the concurrency-safety comment @735-737 to reference the deferred
        EditMessage (→ this task). Read it to confirm S1's scope (launch closure + comment ONLY — it does
        NOT touch the serial loop, so no conflict)."
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  decompose.go         # THE edit: runLoopFastPath serial publish loop @760-840 — INSERT EditMessage block @~795-797;
                       # stagedConcept @659; drainMsgs @843; msgOut @109. (launch closure @739-744 = S1; NOT touched)
  message.go           # generateMessageCore @80 (P1.M1.T1.S1) + generateMessage's EditMessage mirror @255-262 — CONSUMED, not edited
  decompose_test.go    # TestRunLoopFastPath_ConcurrentPublish @3150 (the harness to clone) ← ADD the BUG-001 regression test
internal/generate/
  finalize.go          # EditMessage @67 (FROZEN) + EditContext @50 + ErrEmptyMessage @44 — CONSUMED, not edited
internal/git/
  git.go               # DiffTreeNameStatus @2177 — CONSUMED, not edited
```

### Desired Codebase tree with files to be added/modified

```bash
internal/decompose/decompose.go        # MODIFY: +1 EditMessage block in runLoopFastPath's serial loop
internal/decompose/decompose_test.go   # MODIFY: +1 BUG-001 regression test (TestRunLoopFastPath_EditSerial)
# (no new files; no struct/API changes; no other package touched; no docs)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (insert BETWEEN res.err close and publishCommit — NOT inside the res.err block, NOT after
//   publishCommit): the EditMessage block goes after the `}` closing `if res.err != nil` (@~793) and
//   BEFORE the `// Publish in CAS order` comment (@795) / publishCommit (@797). If placed INSIDE the
//   res.err block it would never run (res.err != nil means generation failed). If placed AFTER publishCommit
//   the commit would carry the UNEDITED message. The block must run on the SUCCESS path, after `res` is
//   received and error-checked, before publishCommit consumes res.msg. PRIMARY unique anchor: the comment
//   `// Publish in CAS order: parent = prevSHA (CAS expected-old). publishCommit runs hooks +` (@795) —
//   grep it (EXACTLY ONE hit); INSERT before it. ⚠️ Do NOT anchor on `HARD (ErrMessageFailed-wrapped infra)
//   — propagate` alone — it ALSO appears in runLoop (@546); the fast-path hit (@792) is the one with the
//   `commits, nil,` prefix AND the following `// Publish in CAS order` comment.

// CRITICAL (NON-RESCUE hard error — propagate eErr DIRECTLY): EditMessage returns ErrEmptyMessage
//   (sentinel) or fmt.Errorf("--edit: ...") wrapped errors — NONE is a *RescueError. So the error path is
//   `drainMsgs(inflight[i+1:]); return commits, nil, eErr` — do NOT errors.As(&re), do NOT wrap in
//   RescueError, do NOT print a FormatRescueMulti recipe. This matches the existing HARD path at line ~794
//   (`return commits, nil, res.err`) and the contract. ErrEmptyMessage → exit 1 (CLI), editor abort → exit 1.

// CRITICAL (drainMsgs prevents goroutine leaks): on the error path call `drainMsgs(inflight[i+1:])` BEFORE
//   return — exactly as every other error path in the loop does (res.err block @793-794, CAS block, etc.).
//   The N-i-1 still-in-flight goroutines each send once to a buffered(1) channel; drainMsgs receives+discards
//   each so they exit (no leak). Omitting it leaks goroutines that block forever on the channel send.

// CRITICAL (depends on S1 — must land AFTER P1.M1.T2.S1): S1 switches the launch closure to
//   generateMessageCore (no EditMessage in the goroutine). If THIS task landed WITHOUT S1, EditMessage
//   would run BOTH in the goroutine (via generateMessage) AND in the serial loop → double-edit (editor
//   opens twice per concept). So S1 must be merged first. Confirm: `grep -n 'generateMessageCore' internal/
//   decompose/decompose.go` shows the launch-closure call (~742). If absent, S1 hasn't landed — STOP.

// CRITICAL (mirror message.go's best-effort nameStatus — IGNORE the error): use
//   `nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, sc.prevTree, sc.tree)` (error discarded). This is
//   INTENTIONAL (matches message.go:258 + the contract): DiffTreeNameStatus feeds only the EDITMSG summary
//   comment block; "" is a valid best-effort fallback (finalize.go renders an empty "Changes:" section).
//   Do NOT handle the error or fail the commit on it.

// CRITICAL (treeA/treeB mapping — sc.prevTree and sc.tree): stagedConcept (decompose.go:659) has
//   `tree` (= treeB, the frozen write-tree) and `prevTree` (= treeA, the tree it was staged on top of).
//   DiffTreeNameStatus(sc.prevTree, sc.tree) = diff(treeA, treeB) = the concept's tree-to-tree diff.
//   EditContext.TreeSHA = sc.tree (treeB, the snapshot). Do NOT use res.treeB for the diff (it IS treeB,
//   but sc.prevTree is the correct treeA — res has no prevTree field).

// CRITICAL (res.msg is a value copy — assign then use): `res := <-ch` makes res a local VALUE copy of
//   msgOut. `res.msg = edited` mutates that copy; the subsequent `publishCommit(ctx, deps, res.treeB,
//   prevSHA, res.msg)` reads the updated copy. This is correct (msgOut is not a pointer). Do NOT try to
//   re-send on the channel or restructure the loop.

// CRITICAL (NO new imports): decompose.go already imports `generate` (@35) and `git` (@36). We use
//   deps.Config (type config.Config, already typed) and deps.Git (type git.Git) — no `config` package
//   reference. errors/fmt already imported. Zero new imports.

// CRITICAL (runLoop's serial loop is UNAFFECTED — do NOT touch it): runLoop (~490-548) has a
//   byte-similar serial loop with its own `if res.err != nil` block (~530) and publishCommit (~548). It
//   is UNAFFECTED by BUG-001 (its 1-deep overlap generates at most one message → one editor at a time →
//   editing is already serialized). Edit ONLY runLoopFastPath's serial loop. Disambiguate: runLoopFastPath
//   is the one with `for i, ch := range inflight` (a SLICE of channels); runLoop uses a single `ch`.

// CRITICAL (test uses GIT_EDITOR=true, NOT stubtest.BuildEditor): for the no-op "preserve each concept's
//   own message" case, `t.Setenv("GIT_EDITOR", "true")`. The stubeditor (stubtest.BuildEditor) is
//   SINGLE-response — it writes the same STAGECOACH_EDITOR_MSG every invocation, so 2 concepts would get
//   the SAME edited message, FAILING the per-concept assertion. The `true` command exits 0 without
//   modifying the file, so EditMessage reads back what it wrote (each concept's OWN message). This is the
//   only way to assert "no cross-contamination" deterministically.

// SCOPE: do NOT touch the launch closure (@739-744, S1), message.go (P1.M1.T1.S1), EditMessage/
//   finalize.go (FROZEN), runLoop's serial loop (~501-548), publishCommit, msgOut, drainMsgs, git.go,
//   cross-concept dedupe (P1.M2.T1.S1 adds a block BEFORE this one), or any docs.
```

## Implementation Blueprint

### Data models and structure
None. No new types, no struct changes. One new conditional block in the serial publish loop (reuses
`generate.EditMessage` + `generate.EditContext` + `git.DiffTreeNameStatus` + `drainMsgs`, all existing)
and one regression test. `res.msg` is updated in place (a value copy of msgOut) before publishCommit.

### Implementation Tasks (ordered by dependencies)

> **Prerequisite**: P1.M1.T2.S1 (S1) merged — the launch closure must call `generateMessageCore` (not
> `generateMessage`), so EditMessage is NOT in the goroutine. CONFIRM: `grep -n 'generateMessageCore'
> internal/decompose/decompose.go` shows the call at ~742. Also P1.M1.T1.S1 (generateMessageCore exists):
> `grep -n 'func generateMessageCore' internal/decompose/message.go` → @80. If either is absent, STOP.

```yaml
Task 1: EDIT internal/decompose/decompose.go — insert the EditMessage block in runLoopFastPath's serial loop
  - LOCATE the insertion point by the UNIQUE anchor `// Publish in CAS order` (do NOT blind-edit by line
    number — S1's parallel launch-closure edit may shift lines; this comment is in a DIFFERENT region):
      grep -n 'Publish in CAS order' internal/decompose/decompose.go   # → EXACTLY ONE hit (@~795)
    INSERT in the blank line immediately BEFORE that comment. The lines above it are: `drainMsgs(inflight
    [i+1:])` (@~791) → `return commits, nil, res.err // HARD ... propagate` (@~792) → `}` (close of res.err
    block, @~793) → BLANK (@~794) → `// Publish in CAS order ...` (@~795) → publishCommit (@~797).
    ⚠️ NOTE: `HARD (ErrMessageFailed-wrapped infra) — propagate` is NOT unique (also in runLoop @546 as
    `return res.err`); use `// Publish in CAS order` (unique) OR the `commits, nil,` prefix to disambiguate.
  - INSERT the following block in the blank line between the `}` and the `// Publish in CAS order` comment:
    	// BUG-001 fix: apply EditMessage in the SERIAL loop (one editor at a time). After S1 moved
    	// generation to generateMessageCore (no editor in the goroutine), the serial loop is the only
    	// place the shared .git/STAGECOACH_EDITMSG file is touched — so it is concurrency-safe (FR-E4:
    	// --edit gates each commit's message before its already-serialized publication). Mirrors
    	// generateMessage's EditMessage site (message.go) exactly; treeA=sc.prevTree, treeB=sc.tree.
    	if deps.Config.Edit {
    		nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, sc.prevTree, sc.tree) // best-effort; "" on err
    		edited, eErr := generate.EditMessage(ctx, res.msg, deps.Config, generate.EditContext{
    			Git: deps.Git, TreeSHA: sc.tree, NameStatus: nameStatus,
    		})
    		if eErr != nil {
    			// ErrEmptyMessage (editor emptied) or editor abort (e.g. vim :cq) — NON-RESCUE hard error:
    			// propagate directly (exit 1, NOT a rescue), mirroring generateMessage's behavior. Commits
    			// 0..i-1 stand; drain the N-i-1 in-flight channels to avoid a goroutine leak.
    			drainMsgs(inflight[i+1:])
    			return commits, nil, eErr
    		}
    		res.msg = edited
    	}
  - VERIFY: the block is OUTSIDE the `if res.err != nil` block (on the success path) and BEFORE publishCommit.
  - VERIFY: `nameStatus, _ :=` (error ignored — best-effort, matches message.go + the contract).
  - VERIFY: the error path calls drainMsgs(inflight[i+1:]) BEFORE return (no goroutine leak).
  - NO new imports (generate, git already imported; deps.Config is already config.Config).
  - DEPENDENCIES: S1 (generateMessageCore in the goroutine — else double-edit) + P1.M1.T1.S1 (Core exists).

Task 2: EDIT internal/decompose/decompose_test.go — add the BUG-001 regression test
  - CLONE TestRunLoopFastPath_ConcurrentPublish (@3150) as TestRunLoopFastPath_EditSerial. Key differences
    from the clone:
      (a) 2 disjoint concepts (a.go, b.go) — sufficient for the no-cross-contamination assertion.
      (b) dcmMessageMatchManifest with distinct messages: {substr:"a.go", msg:"feat: add a"},
          {substr:"b.go", msg:"feat: add b"}.
      (c) t.Setenv("GIT_EDITOR", "true")  // NO-OP editor — preserves each concept's written message.
      (d) deps.Config.Edit = true         // after constructing deps with Config: config.Defaults().
      (e) (optional) drop the concurrency-timing soft gate (SLEEP_MS) — not needed for the edit test.
  - ASSERTIONS (the BUG-001 proof):
      - err == nil (runLoopFastPath succeeded with --edit)
      - len(commits) == 2
      - commits[0].Subject == "feat: add a"  (concept 0's OWN message)
      - commits[1].Subject == "feat: add b"  (concept 1's OWN message — NOT concept 0's)
      - commits[0].Subject != commits[1].Subject  (no cross-contamination)
  - NAME: TestRunLoopFastPath_EditSerial. PLACE next to TestRunLoopFastPath_ConcurrentPublish.
  - USE t.Setenv (auto-cleanup). deps.Config.Edit = true (config.Defaults() has Edit=false).
  - WHY GIT_EDITOR=true (not stubtest.BuildEditor): the stubeditor is single-response (same msg every
    invocation) — 2 concepts would get the SAME edited message, failing the per-concept assertion. `true`
    exits 0 without modifying the file, so EditMessage reads back what it wrote (each concept's OWN msg).
  - DEPENDENCIES: Task 1 (the EditMessage block must exist for --edit to work on the fast-path).

Task 3: VERIFY build + vet + format + the regression test + full package
  - go build ./...
  - go vet ./internal/decompose/...
  - gofmt -l internal/decompose/decompose.go internal/decompose/decompose_test.go
  - go test ./internal/decompose/ -run TestRunLoopFastPath_EditSerial -v   # the BUG-001 regression gate
  - go test ./internal/decompose/ -run TestRunLoopFastPath_ConcurrentPublish -v   # S1's gate still green
  - go test ./internal/decompose/...            # full package (runLoop, chain, arbiter exercised)
  - make test && make lint
  - Grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the serial-loop EditMessage block (1:1 mirror of message.go:255-262, adapted for the loop)
// INSERT between the `if res.err != nil { ... }` close and `// Publish in CAS order` / publishCommit.
if deps.Config.Edit {
	nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, sc.prevTree, sc.tree) // best-effort; "" on err (mirrors message.go:258)
	edited, eErr := generate.EditMessage(ctx, res.msg, deps.Config, generate.EditContext{
		Git: deps.Git, TreeSHA: sc.tree, NameStatus: nameStatus, // treeB = sc.tree (the snapshot)
	})
	if eErr != nil {
		// ErrEmptyMessage or editor abort — NON-RESCUE hard error (propagate directly, NOT a rescue).
		// Commits 0..i-1 stand; drain in-flight channels (no goroutine leak), matching the loop's other error paths.
		drainMsgs(inflight[i+1:])
		return commits, nil, eErr
	}
	res.msg = edited // value copy of msgOut — publishCommit below reads the updated res.msg
}

// CONTRAST: the res.err block ABOVE this (do NOT confuse) handles *RescueError (errors.As &re) +
// DecomposeRescueError. EditMessage NEVER returns a *RescueError, so this block does NOT do that —
// it propagates eErr directly as a HARD error (like line ~794 `return commits, nil, res.err`).

// PATTERN: the regression test (clone of TestRunLoopFastPath_ConcurrentPublish + cfg.Edit recipe)
func TestRunLoopFastPath_EditSerial(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.go", "package a\n\nvar A = 1\n")
	dcmWriteFile(t, repo, "b.go", "package b\n\nvar B = 1\n")
	dcmRunGit(t, repo, "add", "a.go", "b.go")
	dcmCommitRaw(t, repo, "initial")
	dcmWriteFile(t, repo, "a.go", "package a\n\nvar A = 2\n") // disjoint dirty change
	dcmWriteFile(t, repo, "b.go", "package b\n\nvar B = 2\n")
	g := git.New(repo)
	ctx := context.Background()
	baseTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}")
	tStart, err := g.FreezeWorkingTree(ctx, baseTree)
	if err != nil { t.Fatalf("FreezeWorkingTree: %v", err) }
	preRunHEAD := dcmHeadSHA(t, repo)
	t.Setenv("GIT_EDITOR", "true") // NO-OP editor — preserves each concept's written message
	messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
		{substr: "a.go", msg: "feat: add a"},
		{substr: "b.go", msg: "feat: add b"},
	})
	var logBuf bytes.Buffer
	deps := Deps{Git: g, Config: config.Defaults(), Roles: RoleManifests{Message: messageM}, Verbose: ui.NewVerbose(&logBuf, true)}
	deps.Config.Edit = true // BUG-001: --edit on the fast-path
	concepts := []prompt.PlannerCommit{{Title: "c1", Files: []string{"a.go"}}, {Title: "c2", Files: []string{"b.go"}}}
	commits, _, err := runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)
	if err != nil { t.Fatalf("runLoopFastPath: %v", err) }
	if len(commits) != 2 { t.Fatalf("commits len = %d, want 2", len(commits)) }
	want := []string{"feat: add a", "feat: add b"}
	for i, w := range want {
		if commits[i].Subject != w { t.Errorf("commits[%d].Subject = %q, want %q (BUG-001: own message, no cross-contamination)", i, commits[i].Subject, w) }
	}
	if commits[0].Subject == commits[1].Subject { t.Errorf("cross-contamination: both subjects = %q", commits[0].Subject) }
}
```

### Integration Points

```yaml
NO struct / API / new-logic / docs changes. One conditional block + one regression test.

CODE (internal/decompose/decompose.go):
  - +1 block in runLoopFastPath's serial publish loop: if deps.Config.Edit { DiffTreeNameStatus;
    EditMessage; on eErr drainMsgs+return; res.msg = edited }

CONSUMED (read-only, prerequisites):
  - internal/decompose/message.go generateMessageCore @80 (P1.M1.T1.S1) — the goroutine's call (after S1)
  - internal/decompose/message.go generateMessage's EditMessage site @255-262 — the mirror
  - internal/generate/finalize.go EditMessage @67 + EditContext @50 + ErrEmptyMessage @44 (FROZEN)
  - internal/git/git.go DiffTreeNameStatus @2177

TEST (internal/decompose/decompose_test.go):
  - +1 test: TestRunLoopFastPath_EditSerial (cfg.Edit=true, GIT_EDITOR=true, 2 disjoint concepts)

DOWNSTREAM / COORDINATION:
  - P1.M1.T2.S1 (S1) MUST land first (switches goroutine to generateMessageCore — removes EditMessage
    from the concurrent path; without it this task causes a double-edit).
  - P1.M2.T1.S1 (cross-concept dedupe) adds a seenSubjects block BEFORE this EditMessage block in the
    serial loop — non-overlapping (different concern, earlier in the loop body).

UNCHANGED (do NOT touch): launch closure @739-744 (S1); message.go (P1.M1.T1.S1); EditMessage/
  finalize.go (FROZEN); runLoop's serial loop ~501-548 (1-deep overlap serializes editing — unaffected);
  publishCommit; msgOut; drainMsgs; git.go; the channel; any docs.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (consumes generateMessageCore from S1 + EditMessage FROZEN; the new block must compile)
go build ./...
# Vet the package
go vet ./internal/decompose/...
# Format check
gofmt -l internal/decompose/decompose.go internal/decompose/decompose_test.go
# Expected: nothing listed. If listed: gofmt -w the file(s).
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# THE BUG-001 regression gate — the new test (cfg.Edit=true, no-op editor, no cross-contamination)
go test ./internal/decompose/ -run TestRunLoopFastPath_EditSerial -v
# Expected: PASS — 2 commits, each with its OWN concept's message, no cross-contamination.

# S1's gate still green (cfg.Edit=false ⇒ the new block is a no-op ⇒ behavior-identical)
go test ./internal/decompose/ -run TestRunLoopFastPath_ConcurrentPublish -v
# Expected: PASS unchanged.

# Full decompose package (runLoop, chain, arbiter exercised — must still pass)
go test ./internal/decompose/... -v

# Whole suite (race) — the serial loop is on the fast-path; the race detector confirms no shared-file race
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary
make build

# End-to-end BUG-001 smoke: --edit on a disjoint fast-path opens ONE editor at a time (serial).
# (The unit test TestRunLoopFastPath_EditSerial is the deterministic proof; this is a manual sanity check.)
BIN=/home/dustin/projects/stagecoach/bin/stagecoach
mkdir -p /tmp/sc_bug001 && cd /tmp/sc_bug001 && git init -q
git config user.email t@t; git config user.name t
printf 'package a\n\nvar A = 1\n' > a.go; printf 'package b\n\nvar B = 1\n' > b.go
git add a.go b.go && git commit -qm initial
printf 'package a\n\nvar A = 2\n' > a.go; printf 'package b\n\nvar B = 2\n' > b.go
# GIT_EDITOR=true makes the editor a no-op; with --edit the serial loop runs one (no-op) editor per concept.
GIT_EDITOR=true $BIN --edit --no-color --provider stub --single 2>&1 | head -20 || true
# Expected (post S1+S2): loads without error; the --edit flag is honored (no race). A real editor
# (GIT_EDITOR=...) would open once per concept in publish order. (The unit test is the authoritative gate.)
cd / && rm -rf /tmp/sc_bug001
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the EditMessage block exists exactly once in runLoopFastPath's serial loop
#           (BEFORE S1, there were ZERO generate.EditMessage hits in decompose.go — it lived in message.go)
grep -n 'generate.EditMessage' internal/decompose/decompose.go
# Expected: exactly ONE hit inside runLoopFastPath (the new block). (generateMessage's call is in
#           message.go, a different file.) Confirm it is NOT in runLoop (~501-548).

# Grep guard: the block is gated on deps.Config.Edit
grep -n 'if deps.Config.Edit' internal/decompose/decompose.go
# Expected: one hit (the new block).

# Grep guard: the error path propagates eErr DIRECTLY (non-rescue) + drains
grep -nA1 'eErr != nil' internal/decompose/decompose.go
# Expected: the block does `drainMsgs(inflight[i+1:]); return commits, nil, eErr` — NOT errors.As(&re),
#           NOT DecomposeRescueError, NOT FormatRescueMulti.

# Grep guard: the block uses sc.prevTree + sc.tree (NOT res.treeB for the diff)
grep -n 'DiffTreeNameStatus(ctx, sc.prevTree, sc.tree)' internal/decompose/decompose.go
# Expected: one hit (the new block's nameStatus line).

# Grep guard: the launch closure STILL calls generateMessageCore (S1 landed — no double-edit)
grep -n 'generateMessageCore(ctx, deps, treeA, treeB, nil)' internal/decompose/decompose.go
# Expected: one hit (~742). If absent, S1 hasn't landed — this task would cause a double-edit. STOP.

# Grep guard: runLoop's serial loop (~548) is UNCHANGED — still publishCommit, no EditMessage added there
grep -n 'generate.EditMessage' internal/decompose/decompose.go
# Expected: only the runLoopFastPath hit — runLoop (~501-548) must NOT have one (it serializes via 1-deep overlap).

# Scope-boundary guard: ONLY decompose.go + decompose_test.go changed
git diff --stat -- internal/decompose/message.go internal/generate/ internal/git/
# Expected: empty (message.go = P1.M1.T1.S1; generate/git consumed, not edited).

# Scope-boundary guard: no cross-concept dedupe (seenSubjects) added (that's P1.M2.T1.S1)
git diff internal/decompose/decompose.go | grep -E '^[+-]' | grep -iE 'seenSubjects|IsDuplicate|ExtractSubject' || echo "OK: no dedupe added (P1.M2.T1.S1 owns it)"
# Expected: "OK: no dedupe added".
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/decompose/...` clean
- [ ] `gofmt -l` on the 2 changed .go files empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass, incl. the new TestRunLoopFastPath_EditSerial

### Feature Validation
- [ ] runLoopFastPath's serial loop has an EditMessage block between the res.err close and publishCommit
- [ ] The block is gated on `deps.Config.Edit` (no-op when false)
- [ ] `DiffTreeNameStatus(ctx, sc.prevTree, sc.tree)` feeds EditContext.NameStatus (best-effort, error ignored)
- [ ] `EditContext{Git: deps.Git, TreeSHA: sc.tree, NameStatus: nameStatus}`
- [ ] On success: `res.msg = edited` (publishCommit uses the edited message)
- [ ] On error: `drainMsgs(inflight[i+1:]); return commits, nil, eErr` (non-rescue hard error, no leak)
- [ ] BUG-001 regression test passes: each commit's subject == its own concept's message (no cross-contamination)

### Scope-Boundary Validation
- [ ] launch closure (@739-744) UNCHANGED — still generateMessageCore (S1's edit; this task does NOT touch it)
- [ ] runLoop's serial loop (~501-548) UNCHANGED (1-deep overlap serializes editing; unaffected by BUG-001)
- [ ] NO cross-concept dedupe / seenSubjects added (P1.M2.T1.S1)
- [ ] NO message.go / EditMessage-finalize.go / git.go / publishCommit / msgOut / drainMsgs changes
- [ ] Only internal/decompose/decompose.go + internal/decompose/decompose_test.go changed

### Code Quality
- [ ] The block mirrors generateMessage's EditMessage site (message.go:255-262) — same logic, adapted for the loop
- [ ] The comment cites BUG-001 / FR-E4 + the S1 dependency (generateMessageCore removed the editor from the goroutine)
- [ ] The error path matches the loop's existing HARD-error discipline (drainMsgs + direct return)
- [ ] The test uses GIT_EDITOR=true (no-op) + deps.Config.Edit=true + ≥2 disjoint concepts

---

## Anti-Patterns to Avoid

- ❌ Don't place the EditMessage block INSIDE the `if res.err != nil` block — it would never run (res.err != nil means generation failed). Place it on the SUCCESS path, AFTER the res.err block closes and BEFORE publishCommit.
- ❌ Don't wrap the EditMessage error in a `*RescueError` / `*DecomposeRescueError` / `FormatRescueMulti` recipe — EditMessage returns `ErrEmptyMessage` (sentinel) or a `fmt.Errorf("--edit: ...")` wrapped error, NEVER a `*RescueError`. Propagate `eErr` DIRECTLY (`return commits, nil, eErr`), matching the loop's existing HARD path (line ~794). ErrEmptyMessage → exit 1 (CLI), editor abort → exit 1.
- ❌ Don't omit `drainMsgs(inflight[i+1:])` on the error path — the N-i-1 in-flight goroutines each block on a buffered(1) channel send; without the drain they leak. Every other error path in the loop drains; this one must too.
- ❌ Don't land this task WITHOUT S1 (P1.M1.T2.S1) first — S1 switches the goroutine to `generateMessageCore` (no EditMessage). Without S1, EditMessage runs BOTH in the goroutine (via generateMessage) AND in the serial loop → double-edit (editor opens twice per concept). Confirm S1 landed: `grep -n 'generateMessageCore' internal/decompose/decompose.go` shows the launch-closure call.
- ❌ Don't handle the `DiffTreeNameStatus` error — use `nameStatus, _ :=` (best-effort, error discarded). This is INTENTIONAL (matches message.go:258 + the contract): nameStatus only feeds the EDITMSG summary comment; "" is a valid fallback. Handling/failing on it would break the best-effort contract.
- ❌ Don't use `res.treeB` for the diff's treeA — `res` has no prevTree field. Use `sc.prevTree` (treeA) and `sc.tree` (treeB) from the stagedConcept for this iteration. EditContext.TreeSHA = `sc.tree` (the snapshot treeB).
- ❌ Don't touch runLoop's serial loop (~501-548) — it is byte-similar but UNAFFECTED by BUG-001 (its 1-deep overlap generates at most one message → one editor at a time → editing is already serialized). Edit ONLY runLoopFastPath's serial loop (`for i, ch := range inflight` — a slice of channels). Disambiguate by the `inflight` slice.
- ❌ Don't use `stubtest.BuildEditor` for the BUG-001 regression test — the stubeditor is SINGLE-response (writes the same STAGECOACH_EDITOR_MSG every invocation); with 2 concepts both get the SAME edited message, FAILING the per-concept assertion. Use `t.Setenv("GIT_EDITOR", "true")` (no-op — exits 0 without modifying the file, so EditMessage reads back each concept's OWN written message).
- ❌ Don't add new imports — decompose.go already imports `generate` and `git`; `deps.Config` is already typed `config.Config` (no `config` package reference needed). `errors`/`fmt` already imported.
- ❌ Don't add cross-concept dedupe (`seenSubjects`/`IsDuplicate`/`ExtractSubject`) — that's P1.M2.T1.S1, which adds a block BEFORE this EditMessage block. This task is the EditMessage block ONLY.
- ❌ Don't change EditMessage's signature or behavior (it's FROZEN) — this task only ADDS A CALL SITE in the serial loop, reusing the existing function verbatim.

---

## Confidence Score: 10/10

This is one conditional block (≈12 lines) inserted at a uniquely-anchored spot (between the res.err
close and publishCommit), byte-for-byte mirroring generateMessage's proven EditMessage site
(message.go:255-262) with only the treeA/treeB source and the error-return shape adapted for the loop.
The prerequisites (generateMessageCore from P1.M1.T1.S1; the launch-closure switch from S1) are
specified as hard prerequisites with grep-confirmed confirmations. The error discipline (non-rescue
hard error + drainMsgs) matches the loop's existing HARD path exactly. The regression test clones the
existing TestRunLoopFastPath_ConcurrentPublish harness with the documented cfg.Edit + GIT_EDITOR=true
recipe (the no-op editor is the only deterministic way to assert no cross-contamination, per
test_strategy.md). The scope fences (runLoop's loop, launch closure, message.go, finalize.go, dedupe)
are each guarded by a CRITICAL gotcha + a Level-4 grep check. The only conceivable failure modes —
wrong insertion spot, wrapping eErr in RescueError, omitting drainMsgs, landing without S1, handling
the nameStatus error, using the wrong tree fields, touching runLoop, or using the single-response
stubeditor — are each explicitly enumerated.