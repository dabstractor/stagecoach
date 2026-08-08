# Research Findings — P1.M1.T1.S5 (Fast-path regression suite in decompose_test.go)

Verified by direct source read of `internal/decompose/*`, `internal/git/git.go`,
`internal/stubtest/*`, `cmd/stubagent/main.go`, `internal/provider/render.go`,
and the S1–S4 PRPs (2026-08-08). All citations `file:line`.

## 0. Starting state when S5 runs (CONTRACT — S3 + S4 landed)

S3 (`runLoopFastPath` concurrent phase) and S4 (dispatch gate) are BOTH complete when S5
begins. Concretely (current tree already reflects S3; S4 lands the dispatch):

- `internal/decompose/decompose.go`
  - `isFileDisjoint` (S1, :450) — LANDED, byte-identical.
  - `runLoopFastPath` (S2 sweep + S3 concurrent, :662) — FULLY implemented; **S3's sentinel
    is GONE** (grep `not yet implemented` → 0). Returns `(commits, chainData, err)` in the
    SAME shape as `runLoop`.
  - `Decompose()` dispatch (S4, :237) — `if isFileDisjoint(out.Commits) { runLoopFastPath }
    else { runLoop }`, one `VerboseWarn` per branch (nil-guarded).
  - `drainMsgs(chs []chan msgOut)` (:825) — S3's slice-drain sibling of `drainMsg`.
- `internal/decompose/decompose_test.go` (3234 lines, growing) — existing fast-path tests:
  - `TestIsFileDisjoint` (:2935, S1).
  - `TestRunLoopFastPath_ConcurrentPublish` (:2986, S3) — REPLACED S2's sentinel
    `TestRunLoopFastPath_Sweep`; direct-call, uniform 150ms sleep, CAS-order HARD gate,
    soft elapsed-timing gate, "launched 3 concurrent" log gate.
  - `TestRunLoopFastPath_RescueIsolation` (:3113, S3) — direct-call, **per-concept failure
    via `dcmMessageMatchManifest`** (input-derived, concurrency-safe), rescue + drain +
    authoritative-parent proof.
  - `TestDecompose_Dispatch_DisjointFastPath` + `TestDecompose_Dispatch_SharedFileFallback`
    (S4) — through `Decompose`, stager-seam routing oracle (t.Fatal on fast-path / flag on
    runLoop).
- S5 ADDS the comprehensive suite. It MUST NOT duplicate S3/S4 tests; it EXTENDS coverage
  end-to-end (through `Decompose`) + adds the cases S3/S4 don't cover (cases 3/4/6/7/8/9).

## 1. The function under test — `runLoopFastPath` (decompose.go:662)

Two phases:
1. **SERIAL staging sweep** (FR-M13): per concept → `deps.Git.Add(ctx, concept.Files)` →
   `freezeSnapshot` (= `WriteTree`) → `verifyFreezeSubset` (FR-M1c, EVERY concept, BEFORE
   the skip check) → FR-M8 skip (`treeI == prevTree` → `continue`, prevTree unchanged) →
   else append `stagedConcept{idx, tree, prevTree}`; `prevTree = treeI`.
   - On `Add`/freeze/verify error: `return commits, nil, err` (NON-rescue; nothing in flight).
   - `ctx.Err()` checked at loop top → cancelled aborts.
2. **CONCURRENT message + serial CAS publish** (FR-M14/FR-M7/FR-M12):
   - `inflight[i] = launch(sc.idx, sc.prevTree, sc.tree)` for all staged — ALL N at once
     (buffered(1) channels; goroutines never block on send).
   - Publish loop ranges `inflight` IN ORDER: `signal.SetSnapshot(sc.tree, prevSHA, "")` →
     `<-ch` → `signal.ClearSnapshot()` → on `*RescueError`: fix `re.ParentSHA = prevSHA`
     (authoritative), print `FormatRescueMulti`, `drainMsgs(inflight[i+1:])`, return
     `*DecomposeRescueError`; on other err: drainMsgs + propagate; `publishCommit(treeB,
     prevSHA, msg)` → on `*CASError`: print, drainMsgs, return `ce`; else
     `buildCommitResult` + append to commits/chainData; `prevSHA = newSHA`.

**KEY (no stager):** the fast-path NEVER calls `invokeStager`/`deps.stager` — it calls
`deps.Git.Add` directly. `deps.stager` is unreachable by construction (system_context §6).
This is the dispatch/routing oracle (S4 uses it; S5 case 1/2 reuse it).

## 2. The stub agent — `cmd/stubagent/main.go` (test-only binary)

Controlled entirely by `STAGECOACH_STUB_*` env. Relevant knobs:
- `STAGECOACH_STUB_OUT` (single response), `STAGECOACH_STUB_EXIT`, `STAGECOACH_STUB_STDERR`.
- `STAGECOACH_STUB_SLEEP_MS` (uniform sleep AFTER stdin drain, BEFORE output select).
- `STAGECOACH_STUB_SCRIPT` + `STAGECOACH_STUB_COUNTER` (call-varying; **RACES under
  concurrent launch** — N processes read/increment one counter file; NOT safe for the
  fast-path's concurrent messages).
- `STAGECOACH_STUB_MATCHFILE` (S3, input-derived, **concurrency-safe**): the stub drains
  stdin to a buffer, then `selectMatched(matchFile, stdin)` returns the FIRST rule whose
  `substr` is in stdin. Match-file line format: `substr|msg\n`. Consumed via the helper
  `dcmMessageMatchManifest(t, bin, []messageMatchRule{{substr, msg}})` (decompose_test.go:146).
  **This is THE deterministic-per-concept mechanism for the concurrent fast-path.**
- `STAGECOACH_STUB_MARKER`, `STAGECOACH_STUB_ARGSFILE`, `STAGECOACH_STUB_STDINFILE`.

**S3 already modified the stub** (added MATCHFILE) — so stub-agent extension is an accepted
pattern in this workstream. S5 adds TWO further ADDITIVE, backward-compatible hooks:
- `STAGECOACH_STUB_INTERVAL_FILE` (case 3): append `start_ns end_ns\n` per invocation
  (start = after stdin drain; end = after sleep/output-select). Default-off.
- per-match sleep (case 4/5b): extend the match-file line to `substr|msg|sleepMs` (3rd
  field optional, defaults 0 → S3's 2-field lines unchanged); `selectMatched` returns
  `(msg, sleepMs)`; the stub sleeps `sleepMs` (when MATCHFILE set). Backward-compatible.

## 3. Case-by-case design decisions (all verified against source)

| Case | Mechanism | Drive via | New infra |
|---|---|---|---|
| 1 disjoint flagship (concept isolation + T_start completeness both arbiter sub-cases) | disjoint files; stager seam t.Fatal; arbiter counter file | `Decompose` | none (reuse helpers) |
| 2 shared → fallback matches runLoop | shared file; stager seam stages; assert commits == runLoop-only run | `Decompose` | none |
| 3 concurrency = real interval overlap | `dcmMessageMatchManifest` (distinct msgs) + uniform sleep + INTERVAL_FILE | `Decompose` | INTERVAL_FILE stub hook |
| 4 out-of-order completion, ordered publish | per-match sleep (c0 longest) via match-sleep | `Decompose` | match-sleep stub+helper |
| 5a rescue through Decompose | `dcmMessageMatchManifest` (concept 1 empty → parse-fail → RescueError, `MaxDuplicateRetries=0`) | `Decompose` | none |
| 5b CAS on fast-path | per-match sleep (c0 fast, c1 slow) → window; poll log + move HEAD via `commit-tree`/`update-ref` | `Decompose` | reuses match-sleep |
| 6 FR-M8 empty-skip | concept with empty Files (or path not in T_start) | `Decompose` | none |
| 7 FR-M1c verifyFreezeSubset wired | counting git wrapper (embed `git.Git`, override `MergeTrees`) | `Decompose` | countingGit wrapper |
| 8 tooled_flags-less (G29 side effect) | `Roles.Stager = stubtest.Manifest` (nil TooledFlags); disjoint→succeed, shared→error | `Decompose` | none |
| 9 start-of-run freeze (FR-M1b) | write sentinel AFTER `FreezeWorkingTree`, BEFORE `runLoopFastPath` | direct `runLoopFastPath` (S3 idiom) | none |

### Case-specific verified facts

**Case 1 — concept isolation + T_start completeness:**
- `verifyFreezeSubset` passes trivially for whole-path `git add` of T_start content (part B:
  `MergeTrees(baseTree, treeI, tStart) == tStart`, no conflict). So disjoint adds are clean.
- Concept isolation: `git diff-tree --no-commit-id --name-only -r <sha> <parent>` == concept's
  Files (sorted). Parent[i] = i==0 ? preRunHEAD : commits[i-1].SHA.
- T_start completeness sub-case A (arbiter skipped): planner union == all T_start paths →
  after loop `DiffTreeNames(tipTree, T_start)` empty → arbiter NOT called → tipTree == T_start.
  Sub-case B (arbiter folds): one T_start path declared for NO concept → leftover non-empty →
  arbiter called (counter==1) → null-target → new commit with tree == T_start.
- Arbiter counter: `stubtest.Manifest(bin, Options{Script: dir+"/script.txt", Counter:
  counterFile})` (mirrors MessageRescuePartial / CASAbortPartial). Read counter file post-run.

**Case 7 — verifyFreezeSubset counting (the precise semantics):**
- `verifyFreezeSubset` (stager.go:168) calls `deps.Git.MergeTrees(baseTree, treeI, tStart)`
  exactly once per invocation (part B content check; NO short-circuit on empty changedTreeI).
- `MergeTrees` is called ONLY by `verifyFreezeSubset` (grep-confirmed; arbiter uses
  `OverlayTreePaths`/`AddAll`, not `MergeTrees`).
- `runLoopFastPath` calls `verifyFreezeSubset` for EVERY concept in the sweep (BEFORE the
  FR-M8 skip check). So with N concepts, `MergeTrees` is called N times.
- Item says "once per non-skipped concept" — with a 3-disjoint-file scenario (ALL
  non-skipped), count == 3 == len(concepts) == num non-skipped. **Assert count == 3.**
- `Deps.Git` is the `git.Git` INTERFACE (roles.go:56; `git.New` returns `Git`, git.go:489).
  A counting wrapper embeds `git.Git` and overrides only `MergeTrees`:
  ```go
  type countingGit struct { git.Git; n *atomic.Int64 }
  func (c *countingGit) MergeTrees(ctx, b, o, t string) (string, bool, error) {
      c.n.Add(1); return c.Git.MergeTrees(ctx, b, o, t)
  }
  ```
  Forwarding via embedding covers Add/WriteTree/DiffTreeNames/etc. Set `deps.Git =
  &countingGit{git.New(repo), &n}` BEFORE `Decompose`.

**Case 8 — tooled_flags-less (the exact error):**
- A BARE stub manifest has nil `TooledFlags`: `stubtest.Manifest(bin, opts)` returns a
  `provider.Manifest` with NO `TooledFlags` field set (stubtest.go:141). `tooledStubManifest`
  (stager_test.go:73) is the one that ADDS non-empty TooledFlags. So for case 8 set
  `Roles.Stager = stubtest.Manifest(bin, stubtest.Options{Out: ""})` (TooledFlags-less =
  opencode/qwen-code shape).
- Disjoint → fast-path → stager bypassed → run SUCCEEDS (RenderTooled never called).
- Shared → runLoop → `invokeStager` (deps.stager nil) → `stageConcept` → `Render(mdl, "",
  task, rsn, provider.RenderTooled)` → render.go:154-155: `if len(r.TooledFlags) == 0 {
  return nil, fmt.Errorf("provider %q: tooled mode requires non-empty tooled_flags",
  m.Name) }` → wrapped by `ErrStagerFailed` (stager.go: `return fmt.Errorf("%w: render:
  %v", ErrStagerFailed, rerr)`). Assert `errors.Is(err, ErrStagerFailed)` AND the message
  contains "tooled mode requires non-empty tooled_flags".
- DO NOT inject `deps.stager` for case 8's shared path — let it hit the REAL `stageConcept`
  (that's the faithful "cannot serve as a stager" path).

**Case 9 — start-of-run freeze (direct-call, the controlled-timing approach):**
- Mirror S3's direct-call idiom (`TestRunLoopFastPath_ConcurrentPublish` setup): seed base
  commit + N files; capture `baseTree := rev-parse HEAD^{tree}`; `tStart :=
  g.FreezeWorkingTree(ctx, baseTree)` (captures T_start AND resets index to baseTree).
- IMMEDIATELY after FreezeWorkingTree returns (T_start frozen) and BEFORE calling
  `runLoopFastPath`, write `dcmWriteFile(t, repo, "sentinel.txt", "concurrent")`. This is
  the post-freeze, in-run moment (no stager seam needed — direct control).
- Run `runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)`.
- Assert: (a) sentinel.txt in NO commit's file list (`commits[i].Files`), (b) sentinel.txt
  is still in the worktree post-run (`dcmStatusPorcelain` shows `?? sentinel.txt` or similar).

## 4. The 9 cases vs S3/S4 — distinctness (no wasteful overlap)

- S3 `_ConcurrentPublish` (uniform sleep, soft-timing gate) → S5 case 3 STRENGTHENS with
  HARD per-goroutine interval-overlap evidence + goes through `Decompose`.
- S3 `_RescueIsolation` (direct-call rescue) → S5 case 5a adds through-`Decompose` rescue;
  S5 case 5b (CAS on fast-path) is NEW (S3 only did rescue).
- S4 dispatch tests (routing oracle) → S5 cases 1/2 add concept-isolation + T_start-
  completeness + fallback-equivalence on top of the routing proof.
- Cases 4/6/7/8/9 are entirely NEW.

## 5. Validation commands (verified to exist)

```bash
go test ./internal/decompose/ -run TestDecompose_FastPath -race -count=1 -v   # the new suite
go test ./internal/decompose/... ./internal/git/... ./internal/config/... -race -count=1  # full (PRD §5 output)
go vet ./... && gofmt -l internal/ cmd/stubagent/
make test && make lint
```

## 6. Scope fences (what S5 does NOT touch)

- `runLoopFastPath`, `runLoop`, `isFileDisjoint`, `drainMsg`, `drainMsgs` — BYTE-IDENTICAL
  (S1/S2/S3 own them; S5 only EXERCISES them).
- The `Decompose` dispatch gate (S4) — UNCHANGED.
- The error block + arbiter phase + `rereadFinalCommits` — UNCHANGED.
- `arbiter.go`/`chain.go`/`roles.go`/`message.go`/`stager.go`/`decompose.go` — UNCHANGED.
- The ONLY non-test file S5 touches is `cmd/stubagent/main.go` (test-only binary): TWO
  additive, default-off, backward-compatible env hooks (INTERVAL_FILE + per-match sleep).
  S3 already established stub-agent edits as in-scope for this workstream.
- No config key / CLI flag / docs (S6 owns docs).