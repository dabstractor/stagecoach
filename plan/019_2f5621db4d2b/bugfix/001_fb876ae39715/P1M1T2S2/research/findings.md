# Research Findings — P1.M1.T2.S2 (Apply EditMessage in the serial publish loop before publishCommit)

## 1. The bug (BUG-001) and the two-step fix

runLoopFastPath launches N `generateMessage` goroutines concurrently (decompose.go:739-744). Each
goroutine ends by calling `generate.EditMessage`, which writes/opens a SINGLE shared
`.git/STAGECOACH_EDITMSG` (finalize.go:77,91,97) — N editors race and a concept silently receives
another concept's message (FR-E4 violation). The fix is to move EditMessage OUT of the concurrent
goroutine INTO the serial publish loop (one editor at a time).

- **S1 (P1.M1.T2.S1)** — switch the launch closure to `generateMessageCore` (no EditMessage in goroutine).
  TOUCHES decompose.go launch closure (~742) + comment (~735-737).
- **S2 (THIS task)** — add EditMessage to the serial publish loop (before publishCommit).
  TOUCHES decompose.go serial loop (~795).
- Both edit decompose.go in NON-OVERLAPPING regions → clean merge. S2 REQUIRES S1 landed (else
  EditMessage runs twice: once in the goroutine via generateMessage, once in the serial loop).

## 2. The exact serial publish loop (decompose.go:760-840)

```go
prevSHA := preRunHEAD
for i, ch := range inflight {
    sc := staged[i]                              // stagedConcept: idx, tree, prevTree
    signal.SetSnapshot(sc.tree, prevSHA, "")
    res := <-ch                                  // @767
    signal.ClearSnapshot()                       // @768
    if res.err != nil {                          // @770 — error-handling block
        ... rescue (errors.As &re) ...
        drainMsgs(inflight[i+1:]); return commits, nil, &DecomposeRescueError{...}
        drainMsgs(inflight[i+1:]); return commits, nil, res.err   // HARD — propagate
    }                                            // @795 (close of res.err block)

    // Publish in CAS order: parent = prevSHA ...                 // @797 (comment)
    newSHA, err := publishCommit(ctx, deps, res.treeB, prevSHA, res.msg)  // @799
    ...
}
```

**INSERTION POINT**: between line ~793 (the `}` closing `if res.err != nil`) and the
`// Publish in CAS order` comment (~795), before `publishCommit` (~797). The PRIMARY unique anchor is
the comment `// Publish in CAS order: parent = prevSHA (CAS expected-old). publishCommit runs hooks +`
(grep returns EXACTLY ONE hit). ⚠️ Do NOT anchor on `HARD (ErrMessageFailed-wrapped infra) — propagate`
alone — it appears TWICE: runLoop's loop (@546, as bare `return res.err`) AND the fast-path (@792, as
`return commits, nil, res.err`). The fast-path hit is disambiguated by (a) the `commits, nil,` prefix
and (b) the immediately-following `// Publish in CAS order` comment (runLoop @548 goes straight to
publishCommit with an inline `// parentSHA = prevSHA` comment, no `// Publish in CAS order` block):
```
		return commits, nil, res.err // HARD (ErrMessageFailed-wrapped infra) — propagate   ← @792 (fast-path; runLoop @546 is bare `return res.err`)
	}                                                                                       ← @793 (close of res.err block)

	// Publish in CAS order: parent = prevSHA (CAS expected-old). publishCommit runs hooks +    ← @795 (UNIQUE anchor; INSERT in the blank line above)
	newSHA, err := publishCommit(ctx, deps, res.treeB, prevSHA, res.msg)                      ← @797
```
Insert the EditMessage block in the blank line between `}` (@793) and `// Publish in CAS order` (@795).

## 3. The exact code to insert (from fix_design.md Part 2 + contract)

```go
// BUG-001 fix: apply EditMessage in the SERIAL loop (one editor at a time). The serial loop
// guarantees only one EditMessage is active at a time, so the shared STAGECOACH_EDITMSG file is
// safe. (FR-E4: --edit gates each commit's message before its already-serialized publication.)
// Mirrors generateMessage's EditMessage site (message.go:258-262) exactly; treeA=sc.prevTree, treeB=sc.tree.
if deps.Config.Edit {
    nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, sc.prevTree, sc.tree) // best-effort; "" on err
    edited, eErr := generate.EditMessage(ctx, res.msg, deps.Config, generate.EditContext{
        Git: deps.Git, TreeSHA: sc.tree, NameStatus: nameStatus,
    })
    if eErr != nil {
        drainMsgs(inflight[i+1:])
        return commits, nil, eErr
    }
    res.msg = edited
}
```

## 4. The EditMessage call to mirror (message.go:255-262)

```go
nameStatus, _ := deps.Git.DiffTreeNameStatus(ctx, treeA, treeB) // best-effort; "" on err
msg, err = generate.EditMessage(ctx, msg, deps.Config, generate.EditContext{Git: deps.Git, TreeSHA: treeB, NameStatus: nameStatus})
if err != nil {
    return "", err // ErrEmptyMessage → propagates to runLoop's FR-M12 handling
}
```
In the serial loop: treeA = `sc.prevTree`, treeB = `sc.tree`. The `nameStatus, _ :=` (ignored error)
is INTENTIONAL best-effort (matches message.go + the contract). EditContext fields: `Git`, `TreeSHA`,
`NameStatus` (finalize.go:50-56).

## 5. Error handling: NON-RESCUE hard error (propagate directly)

EditMessage (finalize.go) returns:
- `ErrEmptyMessage` (sentinel, finalize.go:44) — editor returned empty after strip
- `fmt.Errorf("--edit: ...")` wrapped errors (git dir resolve, write, editor abort, read)

NONE is a `*generate.RescueError`. So the serial-loop handling is: `drainMsgs(inflight[i+1:]); return
commits, nil, eErr` — propagate DIRECTLY (NOT wrapped in RescueError, NOT `errors.As(&re)`). This
matches the existing `res.err` block's HARD path (line 794: `return commits, nil, res.err`) and the
contract. The CLI exit-code mapping: ErrEmptyMessage → exit 1 (NOT 3/124 rescue), editor abort →
wrapped error → exit 1. `drainMsgs(inflight[i+1:])` prevents goroutine leaks (decompose.go:843).

## 6. Key types confirmed

- `type stagedConcept struct { idx int; tree string; prevTree string }` (decompose.go:659-663).
  `sc.tree` = treeB (frozen write-tree); `sc.prevTree` = treeA (the tree staged on top of).
- `type Deps struct { Git git.Git; ...; Config config.Config; ... }` (roles.go:55-58). `deps.Config`
  is `config.Config`; `deps.Git` is `git.Git`.
- `type EditContext struct { Git git.Git; TreeSHA string; NameStatus string }` (finalize.go:50-56).
- `func EditMessage(ctx, msg string, cfg config.Config, editCtx EditContext) (string, error)`
  (finalize.go:67) — FROZEN signature.
- `func (g *gitRunner) DiffTreeNameStatus(ctx, treeA, treeB string) (string, error)` (git.go:2177).
- `func drainMsgs(chs []chan msgOut)` (decompose.go:843) — nil-safe, drains a slice of buffered(1) chans.
- `msgOut` is a VALUE type; `res := <-ch` is a local copy → `res.msg = edited` mutates the copy,
  then `publishCommit(..., res.msg)` uses it. Correct.

## 7. Imports — NO new imports needed

decompose.go already imports `generate` and `git` (decompose.go:35-36). We use `deps.Config` (not the
`config` package directly), so `config` is NOT needed. `errors`/`fmt` already imported (used by the
res.err block). Zero new imports.

## 8. The TDD test (BUG-001 regression — also P1.M3.T1.S1)

Mirror `TestRunLoopFastPath_ConcurrentPublish` (decompose_test.go:3150):
- 2+ disjoint concepts (a.go, b.go) with `dcmMessageMatchManifest` distinct messages per concept
- `t.Setenv("GIT_EDITOR", "true")` (NO-OP editor — exits 0, preserves the written message)
- `deps.Config.Edit = true`
- Call `runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)` directly
- Assert `commits[0].Subject == "feat: add a"` AND `commits[1].Subject == "feat: add b"` (each concept
  gets its OWN message — NO cross-contamination)

**Why GIT_EDITOR=true (not stubtest.BuildEditor)**: the stubeditor is SINGLE-response (writes the same
STAGECOACH_EDITOR_MSG every time) — with 2 concepts both would get the SAME edited message, FAILING
the per-concept assertion. The no-op `true` preserves each concept's OWN written message, so the
assertion proves the serial loop gives each concept its own message (the BUG-001 fix).

**Why deterministic on the fixed code**: on the OLD code (concurrent EditMessage in goroutines), the
shared file race made one concept get another's message (non-deterministic). On the FIXED code (S1+S2),
EditMessage runs serially → each concept gets its own → deterministic. The test verifies FIXED behavior.

`config` + `stubtest` already imported in decompose_test.go (lines 18, 23).

## 9. Coordination / scope fences

- S2 REQUIRES S1 (generateMessageCore switch) — else double-edit (goroutine + serial loop).
- S2 REQUIRES P1.M1.T1.S1 (generateMessageCore exists).
- Do NOT touch: launch closure (S1), message.go (P1.M1.T1.S1), EditMessage/finalize.go (FROZEN),
  runLoop's serial loop (~501-548 — unaffected; its 1-deep overlap already serializes editing),
  publishCommit, msgOut, drainMsgs, cross-concept dedupe (P1.M2.T1.S1).

## 10. Validation (verified)

- `go build ./...` — consumes generateMessageCore (S1) + EditMessage (FROZEN)
- `go vet ./internal/decompose/...`
- `gofmt -l internal/decompose/decompose.go`
- `make lint` = `golangci-lint run`
- `make test` = `go test -race ./...`
- Targeted: `go test ./internal/decompose/ -run TestRunLoopFastPath_EditSerial -v`