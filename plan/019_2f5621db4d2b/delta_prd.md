# Delta PRD — File-disjoint staging fast-path + provider-docs sync

**Scope of this delta:** bring the code and docs into conformance with the v3.2 spec (the gap
between the v3.0 PRD the prior session worked against and the current v3.2 spec). Most of the
v3.0→v3.2 spec delta is **already implemented and committed** — see "Already shipped (no work
needed)" below. This PRD covers only the remaining gaps.

---

## What changed in the spec (v3.0 → v3.2) — and what's already done

| Spec delta | Status in code | Action this delta |
| --- | --- | --- |
| **v3.1 — `.deb`/`.rpm` via goreleaser `nfpms` + hosted apt/dnf repos on GitHub Pages** (G28; FR-U2/FR-U3 `deb`/`rpm` channels; §21.2/§21.3) | ✅ Shipped (commits `5504afe`, `cc20d75`, `9340dbe`, …) | None |
| **v3.1 — npm package renamed `@dabstractor/stagecoach` → `stagecoach-ai`** (§21.2, §21.3, G27, FR-U3) | ✅ Shipped (README/docs already say `stagecoach-ai`) | None |
| **agy is now stager-capable** (§12.5.1 / §12.5.1.1 item 4 RESOLVED; tooled_flags = `--mode accept-edits --dangerously-skip-permissions`, `tooled_repo_dir_flag = --add-dir`; FR-D4 note) | ✅ Shipped in code (`c34f480`); ✅ spec updated (`50299b2`) | **Doc sync** — `docs/providers.md` is stale (Phase 2) |
| **codex is now stager-capable** (§12.7; `f40ac87`) | ✅ Shipped in code + spec | **Doc sync** — `docs/providers.md` is stale (Phase 2) |
| **Spec split into `spec/01-product.md … 07-reference.md`** + `FUTURE_SPEC.md` → `spec/` | ✅ Shipped (`99700e6`) | None |
| **v3.2 — file-disjoint staging fast-path** (G29; new FR-M13/FR-M14; §13.6.3 addendum) | ❌ **Spec-only** (`c621673` = 8 insertions across 3 spec files, zero code) | **Implement** (Phase 1) |

**Conclusion:** the only code gap is the **file-disjoint staging fast-path (FR-M13/M14)**; the only
doc gap is the **stale `docs/providers.md` stager table** for the already-shipped agy/codex stagers.

---

## Phase 1 — File-disjoint staging fast-path (G29, FR-M13/FR-M14)

### Why (one paragraph)

The stager is the decompose pipeline's only **tooled** role and its only inherently-serial step —
every stager mutates the same live index, so message generation is capped at the 1-deep overlap of
FR-M6. But the stager exists solely to **hunk-split** a file shared across concepts, and the planner
already declares a **file-level** partition (`Files` per concept). When that partition is pairwise
file-disjoint (the common case for cleanly separated changes), stagecoach can stage each concept
**deterministically** with `git add` — invoking no stager agent at all — and, with all `tree[i]`
frozen up front, run the N message generations **concurrently**. This collapses a disjoint run's
critical path from "planner + ~N sequential steps" to "planner + one message latency," and lets a
provider with empty `tooled_flags` (opencode, qwen-code) decompose a disjoint tree it otherwise
could not. The fast-path is automatic, deterministic, and adds no configuration. **Spec is already
final** (`spec/01-product.md` G29, `spec/03-generation.md` FR-M13/M14, §13.6.3 addendum).

### What "done" looks like

- A disjoint planner partition stages via deterministic `git add` (no stager agent invoked) and
  publishes N commits with all messages generated concurrently, in strict CAS order.
- A partition with any shared file falls back transparently to the existing tooled-stager loop for
  the whole run — identical observable commits, identical safety guarantees.
- Every §13.6.3 / §20.2 invariant is preserved on both paths (freeze, FR-M1c subset check, FR-M8
  empty-skip, FR-M12 per-concept failure isolation, serialized CAS).
- A regression suite covers the fast-path's new behaviors and its fallback.

### Milestone P1.M1 — Implement + test the fast-path

**Mode A docs (ride with the work):**
- `docs/how-it-works.md` — its decompose section (lines ~62, ~80–113) currently presents the
  tooled-stager + 1-deep-overlap as *the* decompose model ("`stager[i+1]` runs in parallel with
  `message[i]` … This 1-deep overlap keeps latency low"). The fast-path adds a second mode the
  narrative must mention: when the partition is file-disjoint, there is no stager and messages run
  fully concurrent. Add a short "File-disjoint fast-path" paragraph. **No README change** — the
  fast-path is an invisible, automatic optimization with no CLI/config surface.

#### Task P1.M1.T1 — Implement the file-disjoint staging fast-path + tests

**Subtask P1.M1.T1.S1 — Fast-path code (disjointness gate → deterministic `git add` staging → concurrent message gen → CAS-ordered publish) + regression suite**

- **Story points:** 5
- **Dependencies:** none
- **prd_selectors:** `§9.14` (FR-M13/FR-M14), `§13.6.3` (File-disjoint fast-path), `§6.1` (G29), `§20.2`, `§20.5`

**CONTRACT DEFINITION:**

1. **RESEARCH NOTE — the spec is final, the code does not exist.** Verified this session:
   `grep -niE "disjoint|fastpath|fast-path|pairwise" internal/decompose/` returns ZERO code hits;
   `grep -rniE "FR-M13|FR-M14" internal/ cmd/` returns ZERO hits. The only v3.2 artifact is the spec
   commit `c621673`. Authoritative requirements: `spec/03-generation.md` FR-M13/FR-M14 and the
   §13.6.3 "File-disjoint fast-path (FR-M13/M14)" paragraph. Read both in full before coding. No
   web search needed — the algorithm is fully specified and reuses only existing primitives.

2. **INPUT — where the fast-path gate inserts.** The entry point is `Decompose()` at
   `internal/decompose/decompose.go:144`. By the time control reaches the post-planner dispatch
   (~line 226, after `callPlanner` returns `out prompt.PlannerOutput` with `out.Commits
   []prompt.PlannerCommit` and `out.Single == false`), these are already in scope and MUST be reused
   unchanged: `preRunHEAD string`, `isUnborn bool`, `baseTree string`, `tStart string`,
   `deps.Git` (git wrapper), `deps.Config`, `deps.Out`, `deps.Verbose`, `signal` (the snapshot
   toggle), `lock`. The planner output shape: `prompt.PlannerCommit{ Title, Description string; Files
   []string }` (`internal/prompt/planner.go:83`) — `Files` is the disjointness-test input and the
   `git add` pathspec source.

   The existing per-concept loop to fall back to is `runLoop()` at
   `internal/decompose/decompose.go:468` (always invokes the tooled stager via `invokeStagerRetry`).
   **Do not modify `runLoop`'s algorithm** — the fast-path is a SIBLING function; `runLoop` stays as
   the shared-file fallback, byte-identical in behavior.

3. **LOGIC.** Add the fast-path as a new code path dispatched from `Decompose()` immediately after
   the planner returns a non-single multi-concept partition (before the existing `runLoop` call site).

   **(a) Disjointness gate (FR-M13).** A pure function over `concepts []prompt.PlannerCommit`:
   `isFileDisjoint(concepts) bool` returns true iff **no path appears in more than one concept's
   `Files`**. Implementation: iterate every concept's `Files`, track a `map[string]int` occurrence
   count; disjoint iff every path's count ≤ 1. A concept with empty `Files` does NOT disqualify
   (it shares no path) — it will stage nothing and hit FR-M8 empty-skip downstream. The gate is a
   set-membership test, deterministic, no I/O. **A single path in ≥2 concepts → return false →
   `Decompose` calls the existing `runLoop` for the WHOLE run** (no per-concept mixing of the two
   paths — FR-M13 is explicit: "disqualifies the fast-path for the whole run"). Log the gate's
   decision at `deps.Verbose` (which path was taken).

   **(b) Deterministic staging sweep (FR-M13).** New function, e.g.
   `runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD string, isUnborn bool)` returning
   `([]CommitResult, []ChainEntry, error)` — the SAME signature as `runLoop`. Algorithm:
   - The index is already reset to `baseTree` by `Decompose` step (3) (the existing
     `FreezeWorkingTree`-adjacent reset that `runLoop` also relies on) — confirm and reuse; do NOT
     re-reset. `runLoop`'s `prevTree := baseTree` baseline is identical here.
   - Compute `tStartPaths` via `deps.Git.DiffTreeNames(ctx, baseTree, tStart)` exactly as `runLoop`
     does (line ~484) — reuse the same call; it is the FR-M1c subset baseline.
   - **Staging sweep (serial, cheap, no agent):** for each concept `i` in order, run
     `git add <concept[i].Files...>` (a per-path add; see MOCKING below for the primitive). This
     stages adds, modifications, AND deletions for those whole paths. Then `tree[i] =
     freezeSnapshot(ctx, deps)` (`write-tree`). Then run **`verifyFreezeSubset`** (FR-M1c) on
     `tree[i]` against `tStart`/`tStartPaths` — identical call to `runLoop`'s (the whole-path adds
     of `T_start` content pass it trivially, but it MUST still run on every tree — defense-in-depth
     is unchanged). Apply **FR-M8 empty-skip**: if `tree[i] == prevTree` (concept staged nothing —
     e.g. empty `Files` or planner hallucination), skip concept `i` (no message, no commit), log,
     keep `prevTree` unchanged, continue. On `verifyFreezeSubset` violation: drain any work and
     return `(commits, nil, vErr)` NON-RESCUE — mirror `runLoop`'s exact handling (it drains
     `inflight` then returns partial; here there is no in-flight message yet at staging-sweep time,
     so just return the partial commits). Collect the non-skipped `tree[i]` + their `prevTree`
     (the tree-to-tree diff base) + concept index into a slice for the message phase.
   - **Concurrent message generation (FR-M14):** because every `tree[i]` is frozen before any
     message starts, launch ALL N (non-skipped) message generations concurrently, reusing the
     existing `generateMessage(ctx, deps, treeA, treeB)` and the buffered-channel `launch()` pattern
     (`runLoop` line ~495). Each goroutine reasons over `diff(prevTree, tree[i])` — tree-to-tree,
     never the live index/HEAD (§13.6.3 invariant 2, preserved). Arm `signal.SetSnapshot(tree[i],
     prevSHA, "")` per in-flight message as `runLoop` does (rescue coverage). **Publish strictly in
     CAS order** reusing the existing `publishCommit(ctx, deps, tree, parentSHA, msg)` + the CAS
     `update-ref`: commit[0] first (parent = `preRunHEAD` or root if `isUnborn`), commit[i] parent =
     `newSHA[i-1]`, serialized. Reuse `buildCommitResult` + `ChainEntry` accumulation exactly as
     `runLoop`'s `publish` closure does.
   - **FR-M12 per-concept failure isolation (preserve on the concurrent path):** on a message
     `*generate.RescueError` at position `i`: publish commits 0..i-1 (already landed), print
     `generate.FormatRescueMulti(...)` naming concept `i`, **drain the remaining in-flight message
     channels (i+1..N-1)** to avoid goroutine leaks, and return `*DecomposeRescueError` carrying
     the partial commits — the same partial-commits-stand semantics as `runLoop`. On a
     `*generate.CASError` during publish: print `ce.Error()`, drain remaining, return `ce` (prior
     commits stand). The ONLY difference from `runLoop` is that "drain remaining" covers N-i-1
     channels instead of 1 — generalize `drainMsg` to a slice (or loop it). `RestoreDefault`/signal
     disarm semantics around the CAS window are identical.
   - Return `(commits, chainData, nil)` — the arbiter (`runArbiterPhase`) runs UNCHANGED after
     either path (it consumes `tipTree`/`tStart`/`chainData`, all of which the fast-path produces in
     the same shapes). Do NOT touch `arbiter.go`/`chain.go`.

   **(c) Dispatch in `Decompose`.** Replace the single `runLoop(...)` call with:
   ```
   if isFileDisjoint(out.Commits) {
       commits, chainData, err = runLoopFastPath(ctx, deps, out.Commits, baseTree, tStart, preRunHEAD, isUnborn)
   } else {
       commits, chainData, err = runLoop(ctx, deps, out.Commits, baseTree, tStart, preRunHEAD, isUnborn)
   }
   ```
   Everything after (arbiter phase, `rereadFinalCommits`, result assembly) is shared and unchanged.

   **(d) Concurrency bound — judgment call, spec-permitted.** FR-M14 says "concurrently" with no
   cap; the planner's `max_commits` (default 12) already bounds N. Launch all N unless profiling or
   a provider-rate-limit concern argues for a semaphore — if you add one, default it high (≥12) and
   document it; do NOT introduce a new config key unless you find a real provider that errors on
   ≥12 concurrent one-shot calls. Prefer no cap (matches the spec) unless evidence demands one.

4. **MOCKING / primitives.** No external services, and **no new git primitive is needed.** A per-path
   `Add(ctx context.Context, paths []string) error` ALREADY EXISTS — interface at `internal/git/git.go:158`,
   impl at `internal/git/git.go:1332` (doc: "the path-specific companion to AddAll … stages … modifications,
   additions/untracked, AND deletions"). It is currently consumed only by the arbiter's resolution; the
   fast-path is its second call site — call `deps.Git.Add(ctx, concept.Files)` directly, do NOT add a
   wrapper. (`AddAll(ctx)` at `git.go:148`/`:1321` is the whole-tree form, used by the escape-hatch at
   `decompose.go:314` — not what the fast-path wants.) The freeze/verify/publish/message primitives
   (`freezeSnapshot`, `verifyFreezeSubset`, `generateMessage`, `publishCommit`, `buildCommitResult`,
   `drainMsg`, `signal.SetSnapshot`/`ClearSnapshot`) all already exist in `decompose.go` — call them, do
   not reimplement. The stub provider (`internal/stubtest`) is the mock for every agent role; the
   existing `decompose_test.go` temp-repo + stub-planner/stub-stager/stub-arbiter scaffolding is the
   test substrate.

5. **TESTS (required, in `internal/decompose/decompose_test.go`, extending the existing stub suite).**
   Add a `runLoopFastPath` (or fast-path-gated `Decompose`) coverage set:
   - **Disjoint → N commits, no stager invoked:** a stub planner returning ≥3 concepts with
     pairwise-disjoint `Files`; inject a stager stub that FAILS the test if ever called (or assert
     the stager seam was never invoked). Assert exactly N commits land, each commit's `diff-tree`
     (vs its parent) == exactly that concept's `Files` (concept isolation, §20.2), the chain parents
     correctly, and all of `T_start` is committed (frozen leftover empty → arbiter skipped, OR
     arbiter folds unclaimed paths — drive both sub-cases).
   - **Shared file → fallback to tooled stager for the WHOLE run:** a stub planner where one path
     appears in two concepts' `Files`. Assert the tooled stager stub IS invoked for every concept
     (the existing `runLoop` path) and the resulting commits match the pre-fast-path behavior
     byte-for-byte (golden-compare against the current behavior if practical).
   - **Concurrency is real:** assert the N message goroutines overlap in time (e.g. each stub
     message call records a start/end timestamp into a shared slice under a mutex; assert the
     intervals pairwise overlap, not strictly serial). Guard against a future refactor silently
     re-serializing.
   - **CAS-ordered publication under concurrency:** publish in order even though messages complete
     out of order (stub message `i` sleeps proportional to `N-i` so later concepts finish first);
     assert the final chain is strictly ordered `preRunHEAD → c0 → c1 → …` and each parent is
     correct.
   - **FR-M12 failure isolation on the fast-path:** (a) stub message at position 1 returns a
     `*generate.RescueError`-inducing output → assert commits 0 landed, the rescue message names
     concept 1, and the remaining in-flight messages were drained (no goroutine leak — assert with
     `goleak` if the suite already uses it, else a buffered-channel-completion check). (b) Stub
     publish at position 1 trips a CAS failure (move HEAD mid-publish) → assert prior commits stand
     and the run aborts.
   - **FR-M8 empty-skip on the fast-path:** a concept whose `Files` stage nothing (empty `Files`, or
     paths not in `T_start`) → `tree[i]==prevTree` → skipped, no empty commit, run continues.
   - **FR-M1c still guards fast-path trees:** (defense test) — hard to trip via deterministic
     `git add` of `T_start` paths, but assert `verifyFreezeSubset` is CALLED once per non-skipped
     concept (count via a counting git wrapper or a seam) so the guard is provably wired.
   - **tooled_flags-less provider succeeds on a disjoint tree (G29 side effect):** a stub stager
     manifest with empty `tooled_flags` (opencode/qwen-code shape) on a DISJOINT partition → the
     run SUCCEEDS via the fast-path (no stager needed); on a SHARED-file partition → the run falls
     back and surfaces the existing "cannot serve as a stager" error (unchanged). This is the
     enabling side effect called out in G29's last sentence — assert it explicitly.
   - **Start-of-run freeze unchanged (FR-M1b):** write a sentinel file to the working tree after
     `T_start` is frozen and before the run completes; assert it lands in NO fast-path commit and
     remains in the working tree post-run (the existing §20.5 concurrent-exclusion scenario, now
     also driven through the fast-path).
   Run the full `go test ./internal/decompose/... ./internal/git/... ./internal/config/...` green.

6. **OUTPUT.** A new `runLoopFastPath` (+ `isFileDisjoint`) in `internal/decompose/decompose.go`, a
   one-line dispatch in `Decompose`, possibly a thin `git.Add(paths...)` if absent, the
   `docs/how-it-works.md` fast-path paragraph (Mode A), and the new test cases in
   `decompose_test.go`. `runLoop`, `arbiter.go`, `chain.go`, the planner, and the message role are
   UNCHANGED. The arbiter consumes the fast-path's output without modification (prove this in tests).

---

## Phase 2 — Provider-docs sync (close the stale-doc gap from the shipped agy/codex stagers)

### Why

Commits `c34f480` (agy stager) and `f40ac87` (codex stager) made agy and codex **stager-capable** in
code and updated the **spec** (`spec/02-providers.md` §12.5.1 / §12.7, and the FR-D4 note: "agy
became stager-capable in this revision … via the unscoped `--dangerously-skip-permissions` path, the
same model pi uses"). But the user-facing `docs/providers.md` was NOT updated — it still says both
"cannot serve as a stager." A reader of `docs/providers.md` is now misinformed about two of seven
built-in providers. This is the docs analogue of the code work that already shipped; the only
remaining gap is the doc.

### Milestone P2.M1 — Sync `docs/providers.md` to the stager-capable agy/codex

**Mode B note (cross-cutting):** no README change is required — the README does not enumerate
per-provider stager status at this granularity. The fix is a single per-file doc.

#### Task P2.M1.T1 — Update `docs/providers.md` stager table + prose for agy and codex

**Subtask P2.M1.T1.S1 — Fix the agy/codex stager rows and the asymmetry prose**

- **Story points:** 1
- **Dependencies:** none (independent of Phase 1 — the agy/codex stager code is already shipped)
- **prd_selectors:** `§12.5.1` (agy), `§12.5.1.1` (item 4 RESOLVED), `§12.7` (codex), `§9.16` (FR-D4 note), `§12.7.1`

**CONTRACT DEFINITION:**

1. **RESEARCH NOTE — code+spec are the source of truth; only `docs/providers.md` is stale.**
   Verified this session: `internal/provider/builtin.go` — agy `TooledFlags = ["--mode",
   "accept-edits", "--dangerously-skip-permissions"]` + `TooledRepoDirFlag = "--add-dir"` (lines
   ~244, verified 2026-07-09 agy v1.1.11); codex `TooledFlags` non-nil (verified 2026-07-09 codex
   0.143.0, line ~400). Both are UNSCOPED (§12.7.1 — same model as pi: the §17.6 stager prompt + the
   HEAD-movement guard + `verifyFreezeSubset` are the safety net, not flag-scoping). The spec
   `spec/02-providers.md` already reflects this. ONLY `docs/providers.md` contradicts it.

2. **INPUT.** `docs/providers.md` lines ~78–88 (the provider table with the `Stager?` column) and
   ~88 (the prose note) and ~93–100 (the "Tools-disable asymmetry" section). Current stale claims:
   - agy row (`Stager?` cell): "`— no`" → must become stager-capable.
   - codex row (`Stager?` cell): "`— no`" → must become stager-capable.
   - Prose note (line ~88): "…`agy` … cannot serve as a stager (empty `tooled_flags`)" → false; rewrite.
   - "Tools-disable asymmetry" section: presents codex/cursor as the read-only-constraint examples
     and is silent that agy+codex now ALSO have a tooled/stager profile (unscoped).

3. **LOGIC.** Bring `docs/providers.md` into agreement with `spec/02-providers.md` §12.5.1/§12.7 and
   the code:
   - **Table `Stager?` column:** agy → "`✓ yes (unscoped)`"; codex → "`✓ yes (unscoped)`". Leave
     opencode and qwen-code as "`— no`" (their `TooledFlags` are still nil — confirmed
     `builtin.go:303`). Use the same "(unscoped)" annotation the spec uses for pi/agy/codex so a
     reader knows the safety model (instructional + HEAD-guard + freeze-subset, not flag-scoping).
   - **Prose note (line ~88):** rewrite the agy sentence to: agy is **experimental** (§12.5.1) — the
     non-TTY stdout drop (#76) no longer reproduces (2026-07-08) and §12.5.1.1 item 4 (stager flags)
     is RESOLVED (2026-07-09, agy v1.1.11); agy IS stager-capable via the unscoped
     `--mode accept-edits --dangerously-skip-permissions` + `--add-dir <repo>` combo (same unscoped
     model as pi). Drop the "cannot serve as a stager" clause entirely. Leave the qwen-code sentence
     as-is (it is still non-stager).
   - **"Tools-disable asymmetry" section:** add a sentence noting agy and codex join pi as
     stager-capable providers, all via the UNSCOPED tooled profile (no git-scoped allowlist); claude
     remains the only STRUCTURALLY-scoped stager (`--allowed-tools Bash(git …)`). Cross-reference
     §12.7.1 and the §17.6 stager prompt + HEAD-movement guard + `verifyFreezeSubset` as the safety
     net for the unscoped providers.
   - Keep the edit surgical: only the agy/codex stager status and the directly-dependent prose. Do
     not reflow the rest of the table or section.

4. **MOCKING.** None — documentation only. No code, no tests. Verify by re-reading the file and
   confirming every `Stager?` cell matches `internal/provider/builtin.go`'s `TooledFlags` (non-nil ⇔
   stager-capable) for all seven providers.

5. **OUTPUT.** An updated `docs/providers.md` whose provider table and prose match the code and
   `spec/02-providers.md`. Consumed by anyone reading the provider reference; no downstream code
   dependency.

---

## Out of scope (explicitly)

- The `.deb`/`.rpm`/apt/dnf distribution work, the npm rename, the agy/codex stager **code**, and
  the spec-file split are all **already shipped** — no tasks here.
- No new CLI flag, config key, provider manifest, or public-API change. The fast-path is automatic
  and configuration-free (FR-M13: "adds no configuration").
- No change to the arbiter, the planner, the message role, the snapshot/CAS/rescue core, or the run
  lock. The fast-path composes the existing primitives; `runLoop` is preserved verbatim as the
  shared-file fallback.
- A scoped (git/Read/Edit allowlist) tooled profile for agy/codex remains a desirable future
  hardening (§12.5.1.1 item 4 note) — out of scope; the unscoped path is the shipped, spec-sanctioned
  model.