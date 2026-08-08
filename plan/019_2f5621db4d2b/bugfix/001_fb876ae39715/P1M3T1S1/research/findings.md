# Codebase Findings — P1.M3.T1.S1 (BUG-001 regression test: --edit + disjoint fast-path, no cross-contamination)

## 1. The fix this test guards (P1.M1 — COMPLETE; verify, don't rebuild)

BUG-001 (Critical): `runLoopFastPath` (internal/decompose/decompose.go:680) launched ALL N message
generations CONCURRENTLY (goroutines), and each goroutine's `generateMessage` ended by calling
`generate.EditMessage` — which writes the candidate msg to a SINGLE shared `<gitDir>/STAGECOACH_EDITMSG`
file, runs the editor, and reads it back. With N>1 disjoint concepts the file-write race silently attached
one concept's message to another's commit (both commits got concept 1's subject). Contradicts FR-E4.

The FIX (P1.M1, COMPLETE) split `generateMessage` into `generateMessageCore` (bare generate/dedupe, NO
editor) + a wrapper; the fast-path launch closure now calls `generateMessageCore` in the goroutine
(decompose.go:746), and `EditMessage` is applied in the SERIAL publish loop, one concept at a time
(decompose.go:806), BEFORE `publishCommit`. So editing is serialized (FR-E4's "serialized" honored) without
giving up the concurrency win for the non-edit case. **This test verifies the FIXED behavior.**

## 2. The skeleton to mirror (TestRunLoopFastPath_ConcurrentPublish, decompose_test.go:3150)

The architect's test_strategy.md says "Mirror TestRunLoopFastPath_ConcurrentPublish". Verified verbatim:
```go
func TestRunLoopFastPath_ConcurrentPublish(t *testing.T) {
    bin := stubtest.Build(t)
    repo := t.TempDir()
    dcmInitRepo(t, repo)
    // Seed base commit with N files, then modify all N disjointly.
    dcmWriteFile(t, repo, "a.go", "...v1..."); ... ; dcmRunGit(t, repo, "add", ...); dcmCommitRaw(t, repo, "initial")
    dcmWriteFile(t, repo, "a.go", "...v2..."); ...  // disjoint dirty change set
    g := git.New(repo); ctx := context.Background()
    baseTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}")
    tStart, err := g.FreezeWorkingTree(ctx, baseTree)   // captures T_start, resets index to baseTree
    preRunHEAD := dcmHeadSHA(t, repo)
    messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{{substr:"a.go",msg:"feat: add a"}, ...})
    roles := RoleManifests{Message: messageM}
    var logBuf bytes.Buffer
    deps := Deps{Git: g, Config: config.Defaults(), Roles: roles, Verbose: ui.NewVerbose(&logBuf, true)}
    concepts := []prompt.PlannerCommit{{Title:"c1", Files:[]string{"a.go"}}, ...}
    commits, chainData, err := runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)
    // ... assertions on commits[i].Subject, CAS order, HEAD, log count ...
}
```
My test is this skeleton + 3 deltas: `cfg.Edit = true`, `t.Setenv("GIT_EDITOR", "true")`, 2 concepts.

## 3. The dcm* helpers (all in decompose_test.go, package decompose — REUSE, don't redeclare)

- `dcmInitRepo(t, dir)` (:30) — git init + repo-local user.name/email (no env pollution).
- `dcmWriteFile(t, dir, name, body)` (:38); `dcmRunGit(t, dir, args...)` (:59); `dcmGitOut` (:70, alias);
  `dcmCommitRaw(t, dir, msg)` (:51, `git commit --allow-empty -m msg` — commits staged content too).
- `dcmHeadSHA(t, dir)` (:76) — `git rev-parse HEAD`.
- `dcmMessageMatchManifest(t, bin, []messageMatchRule)` (:147) — INPUT-DERIVED, concurrency-safe stub:
  each stub process inspects its OWN stdin (the concept's tree-to-tree diff) and emits the FIRST matching
  rule's msg. Writes the rules to `STAGECOACH_STUB_MATCHFILE`. This is the concurrency-safe message source.
- `messageMatchRule{substr, msg string; sleepMs int}` (:172) — unexported fields, but the test is
  `package decompose` so they're accessible. The contract uses the 2-field form `{substr:"a.go", msg:"..."}`.
- `dcmDepsWithConfig(t, repo, roles, cfg)` (:190) — builds Deps with a custom config (Verbose: nil).
- `stubtest.Build(t)` — compiles cmd/stubagent once per process (cached).

## 4. The cfg.Edit + GIT_EDITOR=true recipe (test_strategy.md confirmed)

- `config.Edit bool` (config.go:137, `toml:"-"`, FLAG-ONLY). Set via `cfg := config.Defaults(); cfg.Edit = true`.
- `generate.EditMessage` (finalize.go:67): writes candidate to `<gitDir>/STAGECOACH_EDITMSG`, resolves the
  editor via `git var GIT_EDITOR` (→ VISUAL → EDITOR → vi), runs it, reads the file back.
- **`t.Setenv("GIT_EDITOR", "true")`** ⇒ `git var GIT_EDITOR` outputs `true` ⇒ EditMessage runs
  `sh -c "true <editmsg>"` ⇒ `/bin/true` exits 0 WITHOUT modifying the file ⇒ EditMessage reads back
  EXACTLY what it wrote (the concept's own generated message). NO stubeditor needed (test_strategy.md: "For
  a NO-OP editor (preserve input): `t.Setenv(\"GIT_EDITOR\", \"true\")`").
- WHY this catches the bug (test_strategy.md): on the OLD code, both goroutines write the shared
  STAGECOACH_EDITMSG concurrently; with GIT_EDITOR=true the editor doesn't touch the file, but the LAST
  writer's content is what BOTH read back ⇒ one concept gets the other's msg. On the FIXED code,
  EditMessage runs SERIALLY (one at a time in the publish loop) ⇒ each concept gets its own msg.
- DETERMINISM note (test_strategy.md): the race is non-deterministic on old code; on fixed code there IS
  no race, so the test is naturally deterministic. **The test verifies FIXED correctness, not bug reproduction.**

## 5. The runLoopFastPath signature (FROZEN; BUG-002 in-flight does NOT change it)

```go
func runLoopFastPath(ctx, deps Deps, concepts []prompt.PlannerCommit, baseTree, tStart, preRunHEAD string, isUnborn bool) ([]CommitResult, []ChainEntry, error)
```
- `CommitResult{SHA, Subject, Message string; Files []git.FileChange}` (decompose.go:53) — assert on
  `.Subject` and `.SHA`.
- `isUnborn=false` for a BORN repo (the skeleton seeds a base commit). Confirmed across 4 existing tests.
- The in-flight BUG-002 fix (P1.M2.T1S1) adds `seenSubjects` dedupe INSIDE the serial publish loop (after
  the `res.err` block, before EditMessage) — NO signature change ("FROZEN signatures"). My test uses
  DISTINCT subjects (a.go→"feat: add a", b.go→"feat: add b") so the dedupe path never triggers ⇒ my test
  is compatible with OR without the BUG-002 fix. NO collision with that sibling.

## 6. The architect's exact recipe (test_strategy.md "BUG-001 Regression Test")

- **Setup**: 2+ disjoint concepts (a.go, b.go) with distinct dirty changes; dcmMessageMatchManifest with
  `{substr:"a.go", msg:"feat: add a"}` + `{substr:"b.go", msg:"feat: add b"}`; `GIT_EDITOR=true`; `cfg.Edit=true`.
- **Assertions**: `len(commits) >= 2`; `commits[0].Subject == "feat: add a"`; `commits[1].Subject == "feat: add b"`;
  `commits[0].Subject != commits[1].Subject` (no cross-contamination).
- **Optional cross-check** (contract step j): `dcmGitOut(t, repo, "log", "--format=%s", "-n1", commits[i].SHA)`.

## 7. Name + placement + scope

- Name: `TestRunLoopFastPath_EditGate_NoCrossContamination` (contract: "or similar"). No existing test
  with EditGate/NoCrossContamination names (verified — no collision).
- Placement: `internal/decompose/decompose_test.go` (package decompose, where the dcm* helpers + skeleton
  live) OR a new `internal/decompose/fastpath_edit_test.go`. The contract says "in internal/decompose/".
  Prefer appending to decompose_test.go (the skeleton + helpers are there; no new file needed) — matches
  where TestRunLoopFastPath_ConcurrentPublish lives. (A new file is fine too if decompose_test.go is huge.)
- Scope: TEST-ONLY. NO production-code change (the fix P1.M1 is COMPLETE). NO docs (contract: "none —
  test-only file"). Does NOT touch the BUG-002 dedupe logic (distinct subjects ⇒ no collision path).

## 8. Validation (verified)

- `go test ./internal/decompose/ -run 'TestRunLoopFastPath_EditGate_NoCrossContamination' -v` (the new test).
- `go test -race ./internal/decompose/...` (race detector — the test exercises concurrent generation).
- `make test` (full suite; the new test is additive). `make lint`; `gofmt -l`.
- grep guards (see PRP Validation Level 4).