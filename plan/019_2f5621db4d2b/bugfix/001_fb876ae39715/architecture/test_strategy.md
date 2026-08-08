# Test Strategy — Regression Tests for BUG-001 and BUG-002

## Test Infrastructure

The `internal/decompose` package uses real temp git repos (`t.TempDir()` + `dcmInitRepo`) and
compiled stub agent binaries (`cmd/stubagent`, `cmd/stubeditor`) driven by `STAGECOACH_STUB_*` and
`STAGECOACH_EDITOR_*` env vars. Tests are in `package decompose` (internal test helpers accessible).

### Key test helpers (from decompose_test.go)

- `dcmInitRepo(t, repo)` — git init + repo-local user.name/email
- `dcmWriteFile` / `dcmStageFile` / `dcmCommitRaw` / `dcmRunGit` / `dcmGitOut`
- `dcmHeadSHA` / `dcmLogOneline` / `dcmLogCount` / `dcmStatusPorcelain`
- `dcmMessageMatchManifest(t, bin, []messageMatchRule)` — concurrency-safe, input-derived message stub
- `dcmDeps(t, repo, roles)` / `dcmDepsWithConfig` / `dcmOutBuffer`
- `stubtest.Build(t)` — compiles cmd/stubagent once per process (cached)
- `stubtest.BuildEditor(t)` — compiles cmd/stubeditor
- `stubtest.SetEditorEnv(t, stubtest.EditorOptions{Msg: "..."})` — sets STAGECOACH_EDITOR_MSG

### Skeleton for runLoopFastPath tests

Mirror `TestRunLoopFastPath_ConcurrentPublish` (decompose_test.go:3142):
1. Create temp repo, seed a base commit (BORN repo)
2. Create disjoint dirty files
3. `g.FreezeWorkingTree(ctx, baseTree)` → tStart
4. `preRunHEAD = dcmHeadSHA(t, repo)`
5. Build message manifest + Deps
6. Call `runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)` directly
7. Assert on returned `commits` and `git log`

### The `cfg.Edit` recipe (from generate_test.go:836)

```go
t.Setenv("GIT_EDITOR", stubtest.BuildEditor(t))
stubtest.SetEditorEnv(t, stubtest.EditorOptions{Msg: "edited subject"})
cfg := config.Defaults()
cfg.Edit = true
```

**stubeditor behavior**: writes `STAGECOACH_EDITOR_MSG` to the EDITMSG file, overwriting its
content. If `Msg == ""`, truncates → `generate.ErrEmptyMessage`. The stub is single-response (same
Msg for every invocation).

For a NO-OP editor (preserve input): `t.Setenv("GIT_EDITOR", "true")` — the `true` command exits 0
without modifying the file, so EditMessage reads back what it wrote (the original message).

## BUG-001 Regression Test

**Goal**: Verify that with `--edit` on the fast-path, each concept's published message matches its
own generated message (no cross-contamination from the shared STAGECOACH_EDITMSG race).

**Setup**:
- 2+ disjoint concepts (e.g., a.go, b.go) with distinct dirty changes
- Message stub returns distinct messages per concept: `dcmMessageMatchManifest` with
  `{substr: "a.go", msg: "feat: add a"}` and `{substr: "b.go", msg: "feat: add b"}`
- `GIT_EDITOR=true` (no-op editor) — preserves each concept's generated message unchanged
- `cfg.Edit = true`

**Assertions**:
- `len(commits) >= 2`
- `commits[0].Subject == "feat: add a"` (concept 0's own message, NOT concept 1's)
- `commits[1].Subject == "feat: add b"` (concept 1's own message, NOT concept 0's)
- `commits[0].Subject != commits[1].Subject` (no cross-contamination)

**Why this catches the bug**: On the OLD code, both goroutines write to the shared STAGECOACH_EDITMSG
concurrently. With `GIT_EDITOR=true`, the file isn't modified by the editor, but the last writer's
content is what both goroutines read back. So one concept gets the other's message. On the FIXED
code, EditMessage runs serially (one at a time in the publish loop), so each concept gets its own
message.

**Note on determinism**: The race is non-deterministic on the old code. On the fixed code, there IS
no race, so the test is naturally deterministic. The test verifies FIXED behavior (correctness),
not bug reproduction.

## BUG-002 Regression Test

**Goal**: Verify that when two disjoint concepts' message agent emits the same subject, the result
does NOT contain duplicate subjects.

**Setup**:
- 2 disjoint concepts (e.g., a.txt, b.txt) with distinct dirty changes
- Message stub returns the SAME subject for both: `dcmMessageMatchManifest` with
  `{substr: "a.txt", msg: "chore: update thing"}` and `{substr: "b.txt", msg: "chore: update thing"}`
- `cfg.Edit = false` (isolate BUG-002 from BUG-001)

**Assertions** (rescue case — most likely with single-response stub):
- Either a `*DecomposeRescueError` is returned with concept 0 published and concept 1 rescued
- Or both concepts are published with DIFFERENT subjects
- In either case: NO two published commits share a subject

**Rescue case flow**:
1. Both goroutines generate "chore: update thing" (same subject)
2. Serial loop processes concept 0: subject not in seenSubjects → accepted, published
3. Serial loop processes concept 1: subject IS in seenSubjects (concept 0's) → collision
4. Re-generation: `generateMessageCore(ctx, deps, treeA, treeB, seenSubjects)`
   - Internal dedupe loop rejects "chore: update thing" (it's in seedRejections)
   - Message stub still returns "chore: update thing" (same match rule)
   - After MaxDuplicateRetries+1 attempts → RescueError
5. Concept 1 enters rescue; concept 0 stands

**Alternative success case** (requires stateful stub):
If the message stub returns a DIFFERENT subject on retry, concept 1 is re-generated successfully
with a distinct subject, and both are published. This requires a richer matcher than
`dcmMessageMatchManifest` (which is single-response per rule). The implementer may enhance the stub
or use `MaxDuplicateRetries=0` to force immediate rescue (simpler, still valid).

## Test Infrastructure Limitations

1. **stubeditor is single-response**: writes the same `STAGECOACH_EDITOR_MSG` for every invocation.
   For per-concept edited messages, use `GIT_EDITOR=true` (no-op) instead. This is sufficient for
   BUG-001's regression test (verifying no cross-contamination with a no-op editor).

2. **dcmMessageMatchManifest is single-response per rule**: cannot return different subjects on
   retry. For BUG-002's success-case test (re-generation produces a different subject), either:
   - Use `MaxDuplicateRetries=0` to force immediate rescue (simpler)
   - Enhance the stub to be stateful (more work, but tests the success path)
   The rescue case is sufficient to verify "no duplicate subjects."

3. **No existing --edit test in internal/decompose**: this is a genuine coverage gap. The reference
   recipe exists only in `internal/generate/generate_test.go:836` (`TestCommitStaged_EditGate`).