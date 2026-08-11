# Research: P1.M2.T1.S2 — Stager-availability guard in runLoop

## The core finding (resolves the previous attempt's halt)

The previous implementation attempt HALTED on an apparent "spec conflict" between the runLoop guard
and FR-M12d. **It is not a conflict.** The spec governs two distinct conditions orthogonally:

| Condition | Spec | Meaning | Owner |
|-----------|------|---------|-------|
| **Capability gap** | `spec/02-providers.md:535` (§12.7): *"A provider with empty `tooled_flags` simply cannot serve as a stager"*; `spec/01-product.md:364` (FR-M13) | Stager manifest is bare (nil `tooled_flags`); can NEVER render tooled. Deterministic, permanent. S1 sets `StagerAvailable=false`. | **The guard (BUG-003 / S2).** Hard error before invocation. |
| **Runtime failure** | `spec/03-generation.md:150` (§13.6.6): *"Stager exits non-zero: retry once; on second failure treat as empty (FR-M8)"* | A TOOLED stager was resolved (`StagerAvailable=true`) and invoked, but crashed at exec. Transient. | **FR-M12d (`invokeStagerRetry`).** Retry → empty-skip. |

### Why they appeared to overlap (the implementation-level red herring)

`stageConcept` (`internal/decompose/stager.go:108`) wraps a render error as `ErrStagerFailed`:
```go
spec, rerr := deps.Roles.Stager.Render(mdl, "", task, rsn, provider.RenderTooled)
if rerr != nil { return fmt.Errorf("%w: render: %v", ErrStagerFailed, rerr) }
```
So a bare stager on a shared partition DOES manifest as `ErrStagerFailed` → caught by FR-M12d →
swallowed into empty-skip. That is the code-level overlap. BUT the spec frames them differently:
the bare provider "simply cannot serve as a stager" (§12.7) — a permanent capability gap, not a
transient exec failure. The guard (BUG-003) is the spec-intended mechanism for the capability gap:
detect it cheaply at role resolution and surface a clear error, rather than letting every concept
deterministically fail-and-skip into a silent zero-commit "success."

### The resolution

1. **Guard stays** — fires on `StagerAvailable=false` (capability gap) → hard error. Consistent with
   §12.7 + BUG-003. Fires BEFORE `invokeStagerRetry`/`stageConcept`, so the capability gap never
   reaches FR-M12d.
2. **FR-M12d preserved** for genuine runtime failures — a TOOLED stager (`StagerAvailable=true`)
   that exits non-zero still retries-then-empty. Guard does NOT fire. Proven by
   `TestDecompose_StagerRetryThenEmpty` (`decompose_test.go:2001`).
3. **One test conflated the two** — `shared_cannot_serve_as_stager` (`decompose_test.go:4522`) used
   a BARE stager (capability gap) to exercise FR-M12d. After BUG-003, re-scope it to assert the
   hard error. Its PURPOSE (prove a tooled-less provider can't stage a shared file) is preserved;
   only the asserted OUTCOME changes (silent-skip → clear error). See test_recipe.md.

**This is a test ALIGNMENT, not a spec change** (AGENTS.md rule 2 untouched; rule 3 applies — a bug
fix bringing silent-failure into line with the spec's clear-error intent).

---

## Exhaustive test audit (via subagent sweep of internal/decompose/decompose_test.go)

- `Stager:` is set in 29 locations.
- **27 `stagerM :=` assignments** ALL use `tooledStubManifest` (tooled) + paired with
  `StagerAvailable: true`. **None regress** under the guard.
- **The ONLY 2 BARE-stager usages** (`stubtest.Manifest` with nil TooledFlags) are both in
  `TestDecompose_FastPath_TooledFlagsLessProvider` (`decompose_test.go:4466`):
  - `disjoint_succeeds` (~4470, Stager@4493): DISJOINT partition (a/b/c.txt) → fast-path → guard
    never fires. **Unaffected.** (Proof of the FR-M13 bypass for a no-tooled provider.)
  - `shared_cannot_serve_as_stager` (~4522, Stager@4541): SHARED partition (store.py in both
    concepts) → runLoop → guard FIRES. **The ONLY regression.** Re-scope per test_recipe.md.
- The previous attempt's "31 broken tests" were TOOLED-stub literals missing `StagerAvailable:true`
  — that fix is correct (those stubs ARE tooled) and must stay. `dcmAllRoles` (`decompose_test.go:201`)
  sets `StagerAvailable: true`.
- `TestDecompose_StagerRetryThenEmpty` (2001) + `TestDecompose_StagerRetryThenSuccess` (2149): use
  tooled stub + runtime seam failure + `StagerAvailable: true` → guard does NOT fire → FR-M12d
  retry-then-empty runs unchanged. **The proof this fix preserves FR-M12d.**

---

## Exit-code flow (no CLI change needed)

The guard wraps `ErrDecomposeFailed` (infra sentinel). `handleDecomposeError`
(`internal/cmd/default_action.go:548-555`):
```go
var re *generate.RescueError; var ce *generate.CASError
if errors.As(err, &re) || errors.As(err, &ce) { return exitcode.New(exitcode.For(err), nil) } // silent
return exitcode.New(exitcode.Error, err) // planner/safety/infra → main prints "stagecoach: <msg>"
```
The guard's error is NOT a *RescueError/*CASError → takes the infra arm → `exitcode.Error` → main
prints `stagecoach: <message>`, exit 1. The actionable message surfaces automatically. Contract
point 4 ("surfaces to the user at the CLI level") is satisfied with ZERO CLI changes.

---

## Key file/line references

- Guard: `internal/decompose/decompose.go` runLoop, ~lines 518-530 (FIRST statement, before
  `tStartPaths, err := deps.Git.DiffTreeNames(...)`).
- Sentinel: `internal/decompose/roles.go` `RoleManifests.StagerAvailable` (set false only in the
  S1 no-fallback arm of ResolveRoles).
- Stager invocation: `internal/decompose/stager.go:94-122` (`stageConcept`; `ErrStagerFailed` @42).
- FR-M12d loop: `internal/decompose/decompose.go` `invokeStagerRetry`, ~596-640.
- Dispatch: `internal/decompose/decompose.go` Decompose ~248-258 (`isFileDisjoint` → runLoopFastPath
  / runLoop).
- New guard test: `internal/decompose/decompose_test.go:1068` `TestRunLoop_NoStagerAvailable_Errors`.
- Re-scope target: `internal/decompose/decompose_test.go:4522` `shared_cannot_serve_as_stager`.
- S1 sentinel test: `internal/decompose/roles_test.go:299` `TestResolveRoles_NoStagerCapable_Deferred`.