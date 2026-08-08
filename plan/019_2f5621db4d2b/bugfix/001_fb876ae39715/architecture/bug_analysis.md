# Bug Analysis — BUG-001 and BUG-002

## BUG-001 (Critical): Concurrent editors on shared STAGECOACH_EDITMSG

### Root Cause

`runLoopFastPath` (decompose.go:745-762) launches ALL N `generateMessage` goroutines concurrently.
Each goroutine calls `generateMessage`, which unconditionally calls `generate.EditMessage` at the
end (message.go:212-219) when `cfg.Edit == true`.

`EditMessage` (finalize.go:67-133) writes the candidate message to a SINGLE FIXED path:
```go
editMsgPath := filepath.Join(gitDir, "STAGECOACH_EDITMSG")  // finalize.go:78
```

With N>1 disjoint concepts and `--edit` enabled:
1. Goroutine A writes "feat: add a\n\n# ..." to STAGECOACH_EDITMSG
2. Goroutine B writes "feat: add b\n\n# ..." to STAGECOACH_EDITMSG (overwrites A's content)
3. Goroutine A's editor runs (e.g. `true`/`vi`) — file now has B's content
4. Goroutine A reads back — gets B's message
5. Both commits end up with the same message

The fast-path's concurrency-safety comment (decompose.go:745-749) only reasons about the `.git/index`
("never touches the live .git/index") and is SILENT on EditMessage's shared-file write + interactive
editor. The hazard was not analyzed.

### Why `runLoop` is NOT affected

`runLoop`'s 1-deep overlap means at most ONE message goroutine is in flight at a time:
`publish(inflight)` PRECEDES `launch(i, ...)`. So at most one `generateMessage` → one `EditMessage`
runs at a time. The shared STAGECOACH_EDITMSG file is implicitly serialized.

### Spec Violation

**FR-E4** (spec/01-product.md:462): "In decompose mode, `--edit` gates **each** commit's message
before its **(already serialized)** publication." The word "serialized" is load-bearing — it means
the editor gate does NOT parallelize publication. The bug makes editing non-serialized (N concurrent
editors), directly contradicting FR-E4.

### Fix Approach (PRD Recommendation (a))

**Defer EditMessage out of the concurrent goroutine and perform it in the serial publish loop.**

This preserves the concurrency win for the non-edit case (message generation stays concurrent) while
serializing the editor (one at a time, in CAS order, matching FR-E4's "serialized publication").

Implementation:
1. Extract `generateMessageCore` from `generateMessage` — does everything EXCEPT EditMessage.
2. `runLoopFastPath` goroutines call `generateMessageCore` (no editor).
3. The serial publish loop applies `EditMessage` after receiving each concept's message and before
   `publishCommit`. Since the loop is already strictly serial, the shared STAGECOACH_EDITMSG file is
   safe (only one EditMessage active at a time).

---

## BUG-002 (Major): Cross-concept duplicate-subject detection lost

### Root Cause

On the fast-path, all N `generateMessage` goroutines run BEFORE any commit is published
(decompose.go:745-762 launches all goroutines, then the serial publish loop at 770-828 publishes).

Each goroutine fetches recent subjects ONCE at message.go:120:
```go
recent, err := messageRecentSubjects(ctx, deps.Git, isUnborn)  // git log --format=%s -50
```

Since publication is serialized AFTER all generation, every concurrent goroutine sees the SAME
pre-run HEAD history. No concept can observe a sibling concept's just-produced subject. If two
concepts' message agents emit the same subject, both pass the `IsDuplicate` check (message.go:194)
against the stale snapshot, and both publish — yielding duplicate subjects in `git log`.

### Why `runLoop` is NOT affected

`runLoop` generates message[i] only AFTER publishing concept[i-1] (`publish(inflight)` precedes
`launch(i, ...)`). So message[i]'s fresh `messageRecentSubjects` fetch includes concept[i-1]'s
just-published subject. Cross-concept duplicates are caught.

### Spec Violation

**US7** (spec/01-product.md:212): "I want Stagecoach to guarantee no duplicate subjects, so that
`git log` doesn't contain the same line twice."

**FR30-FR33** (spec/01-product.md:299-302): the duplicate-rejection guarantee. FR-M14 (the
fast-path spec) is silent on dedupe, so the guarantee was dropped implicitly.

### Fix Approach (PRD Recommendation (a))

**After the concurrent generation sweep, run a serial dedupe pass over the N candidate subjects
before publishing.**

Since the serial publish loop already processes concepts one at a time in CAS order, the dedupe
check can be done INCREMENTALLY:

1. Initialize `seenSubjects` with `messageRecentSubjects(ctx, deps.Git, isUnborn)` (pre-run history).
2. For each concept in CAS order, after receiving its generated message:
   a. Extract subject: `generate.ExtractSubject(res.msg)`
   b. Check: `generate.IsDuplicate(subject, seenSubjects)`
   c. If NOT duplicate: add subject to `seenSubjects`, proceed.
   d. If duplicate: re-generate via `generateMessageCore(ctx, deps, sc.prevTree, sc.tree, seenSubjects)`
      with `seedRejections = seenSubjects`. The re-generation's internal dedupe loop checks against
      `recent ∪ seedRejections` and produces a different subject. If retries exhaust → rescue.
   e. Add accepted subject to `seenSubjects`.
3. Proceed to EditMessage + publish.

Key insight: because concept[i-1] is already published (in the serial loop's previous iteration)
before concept[i]'s collision is detected, the re-generation's fresh `messageRecentSubjects` fetch
also includes concept[i-1]'s subject. The `seedRejections` parameter is an additional safety measure
that tells the LLM upfront which subjects to avoid (reducing wasted retry attempts).

### Ordering in the serial loop

The order MUST be: dedupe check → EditMessage → publish. This preserves FR-E3 ("the edited message
bypasses the duplicate re-check") — the dedupe check judges the GENERATED subject, and the user's
hand-edited message is not re-checked.