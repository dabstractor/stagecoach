# P1.M2.T2.S1 Research Findings — Stale arbiter staging comment → FR-M1d frozen-tree (BUG-012)

Source: `architecture/bugfix_subsystems.md` §BUG-012, `spec/SPEC.md:14` (FR-M1d), and `internal/decompose/decompose.go`.

## 0. The stale comment (decompose.go L989-990, inside runArbiterPhase's doc block)

```
// FR-M1b: the leftover diff is FROZEN — TreeDiff(tipTree, tStart), not a live WorkingTreeDiff.
// The arbiter's STAGING (resolveArbiter via AddAll/Add) is UNCHANGED — it stages from the working tree
// (== T_start's source under the invariant); the freeze OUTPUT guarantee for staging is P3.M2.T1.S1.
```

The second sentence is **stale**: it claims staging is "UNCHANGED — it stages from the working tree". FR-M1d
(v2.2, spec/SPEC.md:14) **extended the freeze boundary into the arbiter**: gate, diff, AND staging all derive
from `T_start`. The comment still describes the pre-FR-M1d state (only the diff was frozen; staging read the
live tree). The `P3.M2.T1.S1` forward-reference is moot (that subtask landed the freeze).

## 1. The code is ALREADY correct (this is a comment-only fix)

Verified by grep — the arbiter paths stage from the frozen `tStart`, never the live tree:
- L278: `runArbiterPhase(ctx, deps, arbiterCommits, chainData, tStart, leftoverPaths)` — tStart threaded in.
- L416 (runSingleShortcut): `treePrime := tStart` then `OverlayTreePaths`/`DiffTreeNameStatus(..., treePrime)`.
- resolveArbiter/resolveNewCommit/amendTip resolve via the frozen tree (tStart/treePrime), not AddAll-of-live.
So NO code change — only the comment catches up to FR-M1d. (This is why BUG-012 is a docs/comment fix, not a code fix.)

## 2. FR-M1d (spec/SPEC.md:14, verbatim relevant clause)

"FR-M1d extends the freeze boundary into the arbiter: gate, diff, and staging all derive from T_start;
concurrent changes are left untouched in the working tree. Adds one git primitive (OverlayTreePaths, §13.6.5)."

## 3. The replacement (verbatim target text)

Replace lines 989-990 (the two stale sentences) with:
```
// FR-M1b/FR-M1d: the arbiter's gate, diff, AND staging all derive from the FROZEN T_start —
// TreeDiff(tipTree, tStart) for the leftover diff; resolveArbiter/resolveNewCommit/amendTip stage via
// OverlayTreePaths(treePrime := tStart), never the live working tree. A change written to the working
// tree after T_start capture is left untouched (not swept into an arbiter commit).
```
- References FR-M1b (diff frozen — still true) AND FR-M1d (staging frozen — the corrected claim).
- Names the OverlayTreePaths primitive + treePrime := tStart (matches the code).
- Drops the moot P3.M2.T1.S1 forward-reference and the false "UNCHANGED / stages from the working tree" claim.

## 4. Scope fence

- ONE comment edit in ONE file (internal/decompose/decompose.go), 2 source lines → 4 comment lines.
- NO code change (the arbiter staging is correct). NO test change (no behavioral delta). NO user-facing docs.
- NO PRD.md / tasks.json / prd_snapshot.md / spec.
- The sibling doc block above it (FR-M1b leftover-diff line) stays accurate and is preserved verbatim.

## 5. Validation

- `go build ./...` clean (comment-only). `go vet ./internal/decompose/...` clean.
- `gofmt -l internal/decompose/decompose.go` empty (the replacement uses standard `//` comment formatting).
- `make test` green (no behavioral change — the arbiter paths were already frozen; this is documentation).
- Grep guard: the stale phrase "stages from the working tree" is GONE; "FR-M1d" + "OverlayTreePaths" appear in the new comment.