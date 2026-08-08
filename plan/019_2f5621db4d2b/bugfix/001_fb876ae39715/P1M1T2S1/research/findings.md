# Research: P1.M1.T2.S1 — Switch fast-path launch closure to generateMessageCore (BUG-001 step 1 of 2)

**Scope**: In runLoopFastPath's launch closure (internal/decompose/decompose.go:742), switch the call
from `generateMessage` to `generateMessageCore(..., nil)` + update the concurrency-safety comment. This is
the FIRST half of the BUG-001 fix (S2 applies EditMessage in the serial publish loop). All verified
against the working tree this session.

## 1. The deliverable (ONE call-site edit + one comment edit)

**File:** internal/decompose/decompose.go, inside `runLoopFastPath`'s `launch` closure.

**The call-site edit (line 742):**
- OLD: `m, e := generateMessage(ctx, deps, treeA, treeB)`
- NEW: `m, e := generateMessageCore(ctx, deps, treeA, treeB, nil)`

Pass `nil` for `seedRejections` — cross-concept dedupe is BUG-002's job (P1.M2.T1.S1), NOT this task.
At concurrent-launch time no sibling subjects are decided yet, so the empty seed is correct.

**The comment edit (lines 735-737, the "FR-M14: launch ALL N" block):** rewrite to say
`generateMessageCore` (not `generateMessage`), and note EditMessage is DEFERRED to the serial publish
loop (P1.M1.T2.S2 / FR-E4). The current comment says "each goroutine calls generateMessage ... never
touches the live .git/index" — that's now inaccurate (the goroutine calls Core, which has no EditMessage;
EditMessage moves to the serial loop precisely BECAUSE it is not concurrency-safe).

## 2. ⚠️ CRITICAL SCOPE FENCE: there are TWO identical call lines — edit ONLY line 742

There are TWO `launch` closures with byte-identical call lines:
- **line 515** — inside `runLoop` (the tooled-stager path, 1-deep overlap). MUST STAY on `generateMessage`.
  runLoop is UNAFFECTED by BUG-001 (its 1-deep overlap generates ≤1 message at a time ⇒ editing is
  implicitly serialized). Editing line 515 would break runLoop.
- **line 742** — inside `runLoopFastPath` (the file-disjoint concurrent path). THIS is the edit target.

**Disambiguation:** line 742 is the call INSIDE `runLoopFastPath`, immediately preceded by the comment
`// FR-M14: launch ALL N (non-skipped) message generations CONCURRENTLY.` (unique to the fast-path).
Locate it by `grep -n 'FR-M14: launch ALL N' internal/decompose/decompose.go` → the closure directly
below that comment. Line 515's closure is inside `runLoop` and has a DIFFERENT preceding comment.
Do NOT blind-edit by line number — grep for the unique fast-path anchor and edit the call in THAT closure.

## 3. The transient --edit gap (intentional, S2 closes it) — the key correctness point

`generateMessageCore` OMITS EditMessage (that's the whole point of S1's extract). So after THIS task
(S1) lands, the fast-path's `msgOut.msg` is the generated message WITHOUT EditMessage applied. If
`cfg.Edit == true`, the editor is **silently NOT invoked** on the fast-path — until S2 adds the
EditMessage call to the serial publish loop.

This is the INTENTIONAL two-step design: S1 switches to Core (breaking --edit transiently), S2
re-applies EditMessage in the serial loop (restoring --edit, now concurrency-safe — one editor at a
time, FR-E4). The plan_status shows S2 ("Apply EditMessage in the serial publish loop before
publishCommit") is Planned immediately after S1.

**Why the existing test still passes (no break):** `TestRunLoopFastPath_ConcurrentPublish`
(decompose_test.go:3150) runs with `cfg.Edit == false` (the default — no test in the package sets
`Edit: true`, grep-confirmed). When Edit=false, EditMessage is a NO-OP, so `generateMessage` (Core +
no-op EditMessage) and `generateMessageCore` (no EditMessage) are behavior-IDENTICAL. The test is green
before and after S1. S2 + P1.M3.T1.S1 (the BUG-001 regression test with Edit=true) close the gap and
prove --edit works end-to-end.

## 4. Prerequisite MET: generateMessageCore exists

`generateMessageCore` is defined at `internal/decompose/message.go:80` (signature
`(ctx context.Context, deps Deps, treeA, treeB string, seedRejections []string) (string, error)`).
P1.M1.T1.S1 (the extract) has landed ⇒ this task's edit compiles. (If somehow not present, grep
`func generateMessageCore` returns nothing and the edit won't compile — S1 is the hard prerequisite.)

## 5. The data flow (unchanged structure, changed .msg content)

The launch closure sends `msgOut{conceptIdx: i, treeA: treeA, treeB: treeB, msg: m, err: e}`. The serial
publish loop reads `res.msg` (the `.msg` field). With the switch to Core, `res.msg` is the PRE-EDIT
message. S2 will apply EditMessage to `res.msg` in the serial loop (before publishCommit); P1.M2.T1.S1
will add cross-concept dedupe. **S1 changes ONLY the .msg content (post-edit → pre-edit); the msgOut
struct, the channel, and the serial-loop structure are untouched.**

## 6. What this task does NOT do (scope fences)

- Does NOT touch runLoop's launch closure (line 515) — stays on generateMessage.
- Does NOT add EditMessage to the serial publish loop (that's S2 / P1.M1.T2.S2).
- Does NOT add cross-concept dedupe / seenSubjects (that's P1.M2.T1.S1).
- Does NOT touch message.go (S1 owns generateMessageCore; this task consumes it).
- Does NOT touch EditMessage/finalize.go, publishCommit, msgOut struct, or any test.
- Does NOT change runSingleEscape/runSingleShortcut/runOneFileShortcut/chain.go (they call generateMessage, unchanged).

## 7. COORDINATION WITH SIBLINGS (no conflict)

- **P1.M1.T1.S1** (the extract): LANDED — generates generateMessageCore at message.go:80. This task
  consumes it. Different file (message.go vs decompose.go) — no merge conflict.
- **P1.M1.T2.S2** (next): adds EditMessage to the serial publish loop in decompose.go. SAME file as
  this task but a DIFFERENT region (the serial publish loop ~770-828, not the launch closure 739-744).
  Land S1 first; S2's edit is additive in a non-overlapping region.
- **P1.M2.T1.S1** (BUG-002): adds seenSubjects + cross-concept dedupe in the serial loop + passes
  non-nil seedRejections to the Core call. SAME line 742 region (it will change `nil` → `seenSubjects`),
  but it's a LATER task — land S1 with `nil` now; P1.M2.T1.S1 updates it.

## 8. Validation

- `go build ./...` (consumes generateMessageCore; compiles post-S1).
- `go vet ./internal/decompose/...`.
- `gofmt -l internal/decompose/decompose.go` → empty.
- `go test ./internal/decompose/ -run TestRunLoopFastPath_ConcurrentPublish -v` (the item's TDD gate; cfg.Edit=false ⇒ behavior-identical).
- `go test ./internal/decompose/...` (full package — runLoop/chain callers exercised; must still pass).
- `make test && make lint`.
- Grep guard: `grep -n 'generateMessageCore(ctx, deps, treeA, treeB, nil)' internal/decompose/decompose.go` → exactly ONE hit (line 742, the fast-path closure).
- Grep guard: `grep -n 'generateMessage(ctx, deps, treeA, treeB)' internal/decompose/decompose.go` → still present at line 515 (runLoop) and the 3 single/chain sites (371/415); line 742 GONE (switched to Core).