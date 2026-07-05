# P2.M1.T1.S2 Research Findings — FR-M3b deterministic non-fatal planner coverage check

Verified by reading decompose.go (the integration point), planner.go (callPlanner + output type),
roles.go (Deps), git.go (DiffTreeNames), verbose.go (the sink), the S1 PRP (the Files field), and the
existing decompose_test.go verbose-capture pattern. Load-bearing for the PRP.

## §1. The integration point — between callPlanner success and `if out.Single`

`Decompose` (internal/decompose/decompose.go) flow around the insertion point:

```go
out, err := callPlanner(ctx, deps, deps.Config.Commits, isUnborn, baseTree, tStart)
if err != nil {
    return DecomposeResult{}, err // non-rescue
}

// <<<< FR-M3b coverage check goes HERE (after success, before the single short-circuit) >>>>

// (4) FR-M11 single-SHORTCUT: planner judged N=1 + supplied a message.
if out.Single {
    return runSingleShortcut(ctx, deps, out.Message, preRunHEAD, isUnborn, baseTree, tStart)
}
```

`baseTree` (HEAD^{tree}, or EmptyTreeSHA for unborn) and `tStart` (the frozen working-tree snapshot) are
BOTH in scope at this point. `out` is `prompt.PlannerOutput`; `out.Commits` is `[]prompt.PlannerCommit`.
The check runs only when `!out.Single && len(out.Commits) > 0` (single ⇒ short-circuit past it; zero
concepts ⇒ nothing to union). The check is PURELY DIAGNOSTIC: it MUST NOT return an error or alter control
flow. After it, `if out.Single` and `runLoop` proceed exactly as today.

## §2. The data — `concept.Files` (from S1) + `DiffTreeNames`

- **`out.Commits[i].Files []string`** — added by S1 (P2.M1.T1.S1) to `prompt.PlannerCommit`. Populated for
  free by `ParsePlannerOutput`'s generic `json.Unmarshal`. This task CONSUMES it (S1 is the contract).
- **`deps.Git.DiffTreeNames(ctx, baseTree, tStart) ([]string, error)`** (git.go:288) — returns the SORTED,
  DEDUPED list of paths differing between two trees via `git diff-tree -r --name-only --no-commit-id`.
  Clean paths (NO status prefix) ⇒ directly comparable to `concept.Files` paths. Identical trees ⇒
  `(nil, nil)`. Read-only w.r.t. refs/index.

The coverage set is the UNION of all `concept.Files`. A path in `DiffTreeNames(baseTree, tStart)` NOT in
that union is "unclaimed" → log it.

## §3. DiffTreeNames does NOT apply excludes (consistent with the arbiter gate) — ACCEPTED

`DiffTreeNames` returns ALL changed paths (no pathspec excludes). The planner, by contrast, saw
`TreeDiff(baseTree, tStart, opts)` WITH excludes (lock/snap/map/vendor). So an excluded-but-changed file
(e.g. a `*.lock`) is in `DiffTreeNames` but the planner never saw it ⇒ it would show as "unclaimed".
This is ACCEPTABLE and CONSISTENT: the arbiter gate (decompose.go, ~line 199) ALSO uses
`DiffTreeNames(tipTree, tStart)` without excludes, so excluded-but-changed files already flow to the
arbiter as frozen leftovers. The coverage check flagging them as unclaimed is correct (they ARE unclaimed
by any concept; the arbiter reconciles them). The contract explicitly says use `DiffTreeNames(baseTree,
tStart)` — do NOT apply excludes. Diagnostic-only ⇒ false-positive noise on excluded files is tolerable.

## §4. The verbose sink — `VerboseRawOutput` (nil-receiver-safe; contract-mandated)

`deps.Verbose` is `*ui.Verbose`. `VerboseRawOutput(output string)` (verbose.go:54) is nil-receiver-safe:
`if v == nil || v.w == nil || !v.on { return }`. So the helper calls it unconditionally with no nil guard.

The contract MANDATES `VerboseRawOutput` for both the unclaimed-path line AND the skip line:
- unclaimed: `deps.Verbose.VerboseRawOutput(fmt.Sprintf("decompose: path %q not claimed by any concept (likely leftover for the arbiter)", p))`
- skip: `deps.Verbose.VerboseRawOutput(fmt.Sprintf("coverage check skipped: %v", err))`

SEMANTIC NOTE: `VerboseRawOutput` prepends `"DEBUG: raw output:\n"` (it is designed for agent stdout).
Using it for a diagnostic is slightly odd (VerboseWarn prints `"DEBUG: <msg>"` and fits better), but the
CONTRACT is explicit — follow it verbatim. The test asserts the path STRING appears in the captured
buffer regardless of prefix, so either method satisfies the test; use VerboseRawOutput per the contract.

## §5. The helper — `checkPlannerCoverage(ctx, deps, baseTree, tStart, concepts)` (void, best-effort)

The contract suggests extracting a helper for testability. Signature (ctx is required — DiffTreeNames
takes it; the contract's `checkPlannerCoverage(deps, baseTree, tStart, concepts)` omits ctx for brevity):

```go
func checkPlannerCoverage(ctx context.Context, deps Deps, baseTree, tStart string, concepts []prompt.PlannerCommit)
```

Body:
1. Build `claimed := map[string]bool{}`; for each concept, for each `f := range c.Files`, `claimed[f]=true`.
2. `changed, err := deps.Git.DiffTreeNames(ctx, baseTree, tStart)`. On err: `deps.Verbose.VerboseRawOutput
   (fmt.Sprintf("coverage check skipped: %v", err))`; `return` (NEVER propagate — best-effort).
3. For each `p := range changed`: if `!claimed[p]`: log the unclaimed line (§4).

VOID return — it NEVER returns an error and NEVER alters the run. The caller in `Decompose` ignores it
entirely (just calls it for the side-effect). `prompt` is already imported in decompose.go's package
(planner.go imports it), so `[]prompt.PlannerCommit` adds no import. The helper lives in decompose.go
(contract: "belongs in internal/decompose/decompose.go").

## §6. The Decompose call site edit — 4 lines, guarded

In `Decompose`, immediately after the `callPlanner` err-check and BEFORE `if out.Single`:

```go
// FR-M3b: deterministic, NON-FATAL planner coverage check. Unions concept.Files and logs (verbose) any
// frozen changed-path the planner left unclaimed — a likely arbiter leftover. Diagnostic ONLY: never
// aborts, never hard-constrains the stager (FR-M1c/verifyFreezeSubset remains the sole content guarantee).
if !out.Single && len(out.Commits) > 0 {
    checkPlannerCoverage(ctx, deps, baseTree, tStart, out.Commits)
}
```

The `!out.Single` guard avoids running it on the single-shortcut path (which short-circuits to one commit
anyway). The `len(out.Commits) > 0` guard avoids a vacuous check when there are no concepts.

## §7. The test — stub-planner-driven, real git, verbose-capture (decompose_test.go)

The contract: "drives a stub planner JSON whose concepts' files deliberately omit one changed path; assert
the run SUCCEEDS (no error) AND a capturing Verbose writer received a line naming the unclaimed path."

Follow the EXISTING pattern at decompose_test.go:~2395-2428 (the verbose-capture decompose test):
- `stubtest.Manifest(bin, stubtest.Options{Out: plannerJSON})` injects canned planner JSON.
- `dcmDeps(t, repo, roles)` builds real-git Deps; `dcmStagerSeam(t, repo, map[string][]string{...})` is
  the stager test seam (the stubtest agent can't run git).
- `var lb lockedBuffer; deps.Verbose = ui.NewVerbose(&lb, true)` captures verbose output.
- `Decompose(ctx, deps)` runs the full pipeline (real git for DiffTreeNames + the freeze).
- Assert `err == nil` AND `strings.Contains(lb.String(), `decompose: path "c.txt" not claimed`)`.

Test scenario (3 changed files, planner claims 2, omits 1):
1. Repo: commit a base, then modify 3 files (a.txt, b.txt, c.txt) so `DiffTreeNames(baseTree, tStart)`
   = [a.txt, b.txt, c.txt].
2. plannerJSON: `{"count":2,"single":false,"commits":[{"title":"A","description":"d1","files":["a.txt"]},
   {"title":"B","description":"d2","files":["b.txt"]}]}` — c.txt deliberately omitted.
3. Stager seam: c1→["a.txt"], c2→["b.txt"]. Message script: 3 entries (2 loop + 1 arbiter). Arbiter:
   `{"target": null}` → c.txt becomes an arbiter new-commit (the realistic "planner missed a file" path).
4. Run succeeds (3 commits: 2 loop + 1 arbiter); `lb.String()` contains the c.txt unclaimed-coverage line.

This is a full-Decompose integration test (the contract's "Mock: stubtest.Manifest stub planner returning
canned JSON; real git for DiffTreeNames"). `lockedBuffer` is already defined in decompose_test.go (reuse
it — do NOT redefine). The helper's direct unit-testability (§5) is a bonus; the contract wants THIS
Decompose-level test, so it is the primary.

## §8. Scope fence — what NOT to touch

- `validatePlannerOutput` (planner.go) — UNCHANGED. FR-M3b is diagnostic-only; Files is guidance, never
  validated (FR-M1c is the sole content guarantee). The coverage check is NOT validation.
- `callPlanner`, `ParsePlannerOutput`, `PlannerCommit` — UNCHANGED (S1 owns the Files field; this task
  only READS it).
- `runLoop`, `verifyFreezeSubset`, the arbiter — UNCHANGED. The coverage check never feeds them; it logs
  and is forgotten. FR-M1c (freeze-subset) stays the SOLE content guarantee.
- `internal/prompt/*` — UNCHANGED (S1's domain; this task reads `prompt.PlannerCommit.Files`).
- The planner/stager/arbiter system prompts, docs/how-it-works.md — UNCHANGED (T2/T3/P3 own them; item
  §5: "DOCS: none — diagnostic-only, no user-facing surface").
- go.mod/go.sum — UNCHANGED (no new import; `fmt` + `prompt` already imported in decompose.go's package).

## §9. FR-M3b vs FR-M1c (the key distinction — load-bearing)

FR-M1c (freeze-subset verification in runLoop) is the SOLE content guarantee: it HARD-aborts if a stager
stages content not traceable to T_start. FR-M3b (this check) is DIAGNOSTIC ONLY: it logs unclaimed paths
and never aborts, never constrains the stager. The two are complementary but DISTINCT: M3b is about
planner PRECISION (did the planner account for every path?); M1c is about freeze SAFETY (did the stager
stay within T_start?). The PRP must make clear the coverage check MUST NOT evolve into a hard constraint
— that would duplicate/violate M1c's role and would change the run's failure semantics. The helper is
void + best-effort by design.
