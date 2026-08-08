# P1.M3.T3.S1 — Research Findings (Tighten runLoopFastPath concurrency-safety comment)

Source: direct read of internal/decompose/decompose.go (the LANDED BUG-001 + BUG-002 fixes carry the
current comments) + the parallel P1.M3.T2.S1 PRP (test-only, no conflict). No external research
(comment-only Go change). ~4 tool calls.

## §0 — The contract (item description)

Rewrite the runLoopFastPath concurrency-safety comment so it accounts for EditMessage and dedupe, not
only the .git/index (the PRD Recommendation, h2.5 last bullet). State: (a) goroutines call
generateMessageCore (concurrent-safe: read-only tree reads, no EditMessage, no interactive I/O);
(b) EditMessage applied in the serial publish loop (one editor at a time, FR-E4); (c) cross-concept
dedupe incremental in the serial loop (seenSubjects, US7/FR32); (d) .git/index safe (staging sweep
serial). ALSO update the serial publish loop comment to mention the dedupe check + EditMessage steps.
Comment-only — existing tests pass unchanged. [Mode A]: the comment IS the doc.

## §1 — Current state: the fixes are LANDED; the comments are GOOD but FRAGMENTED

P1.M1.T2.S2 (EditMessage → serial loop) and P1.M2.T1.S1 (seenSubjects dedupe) are COMPLETE on disk.
decompose.go already carries detailed comments, but the HIGH-LEVEL concurrency-safety framing is split
across blocks and the PUBLISH-LOOP overview comment (769-773) is silent on the two serial-only steps.
This task TIGHTENS/CONSOLIDATES that framing (the gap the contract names).

The three existing comment blocks (DO NOT delete — they document mechanics; this task tightens the two
OVERVIEW comments):
- Launch comment (736-742) — the FR-M14 launch block (PRIMARY target).
- seenSubjects block (759-767) — the BUG-002 accumulator mechanics (keep).
- Publish-loop comment (769-773) — the FR-M7 overview (SECONDARY target — add dedupe + EditMessage).

## §2 — VERBATIM current launch comment (decompose.go:736-742) — the PRIMARY rewrite target

```
	// FR-M14: launch ALL N (non-skipped) message generations CONCURRENTLY. Each goroutine calls
	// generateMessageCore — the bare generate/dedupe core (tree-to-tree diff, read-only tree reads, never
	// touches the live .git/index; git_primitives.md). seedRejections is nil here (cross-concept dedupe is
	// applied in the serial publish loop — P1.M2). EditMessage is DELIBERATELY NOT called in the goroutine:
	// it writes/opens a single shared .git/STAGECOACH_EDITMSG and is not concurrency-safe, so it is deferred
	// to the serial publish loop (one editor at a time, FR-E4 serialized publication; P1.M1.T2.S2).
	// No cap (FR-M14; max_commits default 12 bounds N).
```
This already covers (a)/(b)/(d) but frames dedupe (c) as a side-note, not as a core concurrency-safety
invariant, and does NOT cite BUG-001/BUG-002 as the historical reason. The contract wants a SINGLE
comprehensive block that enumerates ALL the invariants + names the two bugs (so the blind spot recurs
less easily).

## §3 — VERBATIM current publish-loop comment (decompose.go:769-773) — the SECONDARY target

```
	// FR-M7: PUBLISH STRICTLY IN CAS ORDER (concept order). The publish loop is the serialization
	// point: commit[i] parent = prevSHA (preRunHEAD/root for i=0, newSHA[i-1] otherwise); each CAS
	// requires HEAD == prevSHA. prevSHA is AUTHORITATIVE for the rescue parent (see findings §5 —
	// runLoop's 1-deep overlap guarantees it at launch; the concurrent path knows it only at publish
	// time, so arm signal + fix the rescue parent HERE, in this serial loop).
```
This mentions ONLY the CAS/rescue-parent serialization. It does NOT mention that the loop is ALSO the
serialization point for the dedupe check (BUG-002) and EditMessage (BUG-001) — the contract's explicit ask.

## §4 — Parallel-task no-conflict confirmation

P1.M3.T2.S1 (the BUG-002 regression test) is TEST-ONLY: it adds `TestRunLoopFastPath_DuplicateSubjectDedupe`
to `internal/decompose/decompose_test.go` and states "NOT in scope: the concurrency-comment tightening
(P1.M3.T3.S1)" + "git status == decompose_test.go ONLY. NO production change." → ZERO overlap with this
item's single-file edit (decompose.go). P1.M3.T1.S1 (BUG-001 test) is Complete and also test-only. So this
item is the ONLY decompose.go production edit in flight → no merge conflict.

## §5 — The desired rewrites (drop-in replacements — see PRP Implementation Blueprint for the exact text)

- **Launch comment (replace 736-742)**: reframe as a "FR-M14 CONCURRENCY-SAFETY CONTRACT" block that (i)
  states each goroutine calls generateMessageCore ONLY + lists its 3 concurrent-safe ops (read-only tree
  reads, message-agent call, per-concept dedupe vs pre-run history) + the 2 things it does NOT do (no
  EditMessage/interactive I/O, no live .git/index — staging sweep is serial); (ii) names the two
  DEFERRED-to-serial operations with their bug IDs (EditMessage=BUG-001, FR-E4; cross-concept dedupe=
  BUG-002, US7/FR30-33); (iii) keeps the fan-out note (no cap; max_commits default 12 bounds N). Preserves
  the existing `git_primitives.md` + `findings §5` cross-refs where relevant.
- **Publish-loop comment (replace 769-773)**: add that the loop is the serialization point for the CAS
  chain AND the two serial-only steps — the seenSubjects dedupe check (BUG-002, before publish) and
  EditMessage (BUG-001, one editor at a time, FR-E4, before publish) — pointing back to the launch block.

## §6 — Validation (comment-only — no behavior change)

```bash
gofmt -l internal/decompose/decompose.go   # empty (// comments; tab-indented to match the block)
go vet ./internal/decompose/...            # clean
go test ./internal/decompose/...           # unchanged — existing suite passes (comment-only)
make test && make lint                     # green
git status --porcelain                     # == internal/decompose/decompose.go ONLY
```
GOTCHA: // line-comments only (no block comments → no `*/`-termination risk); each comment line must be
tab-indented to match the enclosing function body (gofmt enforces). A comment-only change cannot break
compilation unless a line loses its `//` prefix (then it's code) — the grep guards catch that.

## §7 — Grep guards (the "did it actually say the right things" checks)
- Launch comment contains: generateMessageCore, EditMessage, dedupe/seenSubjects, BUG-001, BUG-002,
  FR-E4, US7 (or FR30-33), and the .git/index invariant.
- Publish-loop comment contains: seenSubjects (or dedupe) AND EditMessage.
- The OLD sole-argument framing ("Safe: each goroutine calls generateMessage, which reasons over a tree-
  to-tree diff … never touches the live .git/index" as the ONLY safety claim) is GONE — replaced by the
  multi-invariant contract. (Guard: `grep -n 'generateMessage,' decompose.go` in the launch comment block
  returns ZERO hits — the goroutine calls generateMessageCore, NOT generateMessage.)
- Scope: ONLY decompose.go changed (no test file, no other production file).