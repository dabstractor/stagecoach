# P3.M4.T1.S1 Research Findings — Decompose() Orchestrator

## 1. Inputs (all SHIPPED — consume verbatim, do NOT edit)

| Symbol | File | Signature / Contract |
|---|---|---|
| `ResolveRoles` | decompose/roles.go | `(cfg, *Registry) (RoleManifests, RoleModels, error)`. Returns `Deps.Roles`. `Deps{Git, Registry, Config, Roles RoleManifests, Verbose}`. **Models NOT in Deps** — each role re-derives via `config.ResolveRoleModel(role, deps.Config)`. |
| `callPlanner` | decompose/planner.go | `(ctx, deps, forcedCount int, isUnborn bool) (prompt.PlannerOutput, error)`. **Already enforces safety cap** in auto mode (`forcedCount==0 && Count>MaxCommits → error`). Sentinel `ErrPlannerFailed`. All planner errors = NON-RESCUE (nothing snapshotted). forcedCount==0 → auto; ≥2 → forced; planner never sees `--single` (orchestrator bypasses). |
| `stageConcept` | decompose/stager.go | `(ctx, deps, prompt.PlannerCommit) error`. TOOLED, NO retry, NO parse. Mutates INDEX only. Sentinel `ErrStagerFailed`. Orchestrator owns FR-M8 retry-once-then-empty (S2). |
| `freezeSnapshot` | decompose/stager.go | `(ctx, deps) (string, error)` → thin WriteTree wrapper. §13.6.3 invariant-1. |
| `generateMessage` | decompose/message.go | `(ctx, deps, treeA, treeB string) (string, error)`. BARE message loop over TreeDiff(treeA,treeB). Derives parent+isUnborn INTERNALLY (RevParseHEAD). Returns `*generate.RescueError` on failure (propagate DIRECTLY). SIGNAL-FREE. |
| `publishCommit` | decompose/message.go | `(ctx, deps, tree, parentSHA, msg string) (string, error)`. CommitTree→UpdateRefCAS. `parentSHA` is EXPLICIT CAS expected-old. Root: `parentSHA==""` → no -p + all-zeros expected. Returns newSHA or `*generate.CASError` (DIRECTLY) / ErrPublicationFailed-wrapped. |
| `runArbiter` | decompose/arbiter.go | `(ctx, deps, commits []CommitInfo, leftoverDiff string) (prompt.ArbiterOutput, error)`. Returns `{Target *string}` (nil⇒new). OWNS "when in doubt null". CommitInfo{SHA,Subject,Files []git.FileChange}. Sentinel `ErrArbiterFailed`. |
| `resolveArbiter` | decompose/chain.go (**PARALLEL — P3.M3.T2.S1**) | `(ctx, deps, target *string, commits []CommitInfo, chainData []ChainEntry) error`. Reconciles leftovers → clean tree. Returns ONLY error (not new SHAs). ChainEntry{SHA,Tree,Message,Parent}. Sentinel `ErrArbiterResolutionFailed`. Propagates `*RescueError`/`*CASError` DIRECTLY. |
| `generate.CommitStaged` | generate/generate.go | `(ctx, generate.Deps{Git,Manifest,Verbose}, cfg) (Result, error)`. Single-commit primitive. DOES arm signal internally. Result{CommitSHA,Subject,Message,Provider,Model,Changes}. |

## 2. Key git.Git methods (all shipped, do NOT edit git.go — PARALLEL owns it this cycle)
RevParseHEAD→(sha,isUnborn,err) · RevParseTree(ref)→(tree,err) ["" on unborn] · WriteTree · CommitTree(tree,parents,msg) · UpdateRefCAS(ref,new,expected) · UpdateRefCAS · DiffTree(sha,isRoot)→[]FileChange · StatusPorcelain()→(string,err) · HasStagedChanges()→(bool,err) · AddAll. Consts: `git.EmptyTreeSHA`, `git.ErrCASFailed`. (Add is being added by parallel chain.go.)

## 3. Config fields used (config.Defaults)
`Single bool`, `Commits int` (0=auto,1=single,≥2=forced), `MaxCommits int` (12), `Timeout`, `MaxDiffBytes/MaxMdLines/BinaryExtensions`, `MaxDuplicateRetries`, `Verbose`. All flow through to callPlanner/generateMessage.

## 4. The 1-deep overlap pipeline (DESIGN — §13.6.3)
Overlap = `stager[i] ∥ message[i-1]` (1-deep, NOT unbounded). Pipeline per iteration i:
1. `stageConcept(concepts[i])` — message[i-1] goroutine runs concurrently ✓
2. `freezeSnapshot` → tree[i]
3. FR-M8 empty-skip: `tree[i]==prevTree` → skip (no message, no publish); prevTree unchanged
4. drain+publish message[i-1] (CAS, serialized in order)
5. launch message[i] goroutine (treeA=prevTree, treeB=tree[i])
Final: drain+publish message[N-1].

**Invariant:** publish order i-1 before i (CAS chain). Safe because message[i] uses frozen tree-to-tree diff (immune to live index). Channel-buffered(1) goroutines; MUST drain on any error (avoid leak).

## 5. FR-M11 single-shortcut (NOT generate.CommitStaged)
Planner `single==true` + `Message` → AddAll → WriteTree(treePrime) → dup-check planner.Message vs RecentSubjects → if dup: `generateMessage(baseTree,treePrime)` fallback (RescueError on fail) → `publishCommit(treePrime, parentSHA, msg)`. Distinct from the --single ESCAPE-HATCH (which delegates to generate.CommitStaged, planner bypassed entirely).

## 6. S1 vs S2 boundary (CRITICAL)
**S1 (this):** full happy-path pipeline + overlap + FR-M8 empty-skip + safety cap + planner failure(non-rescue) + single paths + arbiter wiring + DecomposeResult. Loop errors PROPAGATE structurally (stager err→return; msg→*RescueError; CAS→*CASError).
**S2 (next):** FR-M12 per-concept isolation (stager retry-once-then-empty; msg rescue-for-concept-i; CAS abort-with-recovery) + signal arming in loop + multi-commit rescue variant. S2 wraps S1's seams; S1 must structure them cleanly.

## 7. Testing challenge + solution
Stub agent (stubtest) CANNOT run git → can't stage → loop sees tree[i]==tree[i-1] → all skipped. **Solution: injectable stager seam** — add unexported `stager func(ctx,deps,concept)error` to Deps (nil→stageConcept). Orchestrator calls `deps.invokeStager(...)`. Tests inject a stager that runs `git add` for real files → full happy-path N-commit loop testable. SAFE to edit roles.go (parallel task owns chain.go+git.go, NOT roles.go). dcm*-prefixed fixtures (distinct from arb*/chn*/msg*/stg*/planner).

## 8. DecomposeResult gap (honest)
resolveArbiter returns ONLY error (can't change — parallel-owned). So after arbiter: null-path (N+1)-th commit + amended SHAs not knowable without git re-read. **S1 decision:** `Commits` = loop's []CommitResult (accurate for happy path / no-arbiter); `Amended` = count from arbiter target (0/1/N-i). Document staleness for amended/new-commit cases; P4 (public API) re-reads git for final display. NO git.go edit allowed this cycle (can't add SHA-list method).
