# Research Notes — P1.M1.T1.S1 (isFileDisjoint pure predicate + unit test)

Verification against the CURRENT working tree. The task description + PRD §9.14 FR-M13 are accurate.
These notes record the exact verified anchors for a one-pass pure function.

## VERIFIED — the input type
`prompt.PlannerCommit` (internal/prompt/planner.go:83-87):
```go
type PlannerCommit struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Files       []string `json:"files"`  // FR-M3: every path this concept touches — THE disjointness-test input
}
```
The helper takes `[]prompt.PlannerCommit` (the planner output `Commits` field) — same slice type
`runLoop` already receives (decompose.go:456 `concepts []prompt.PlannerCommit`).

## VERIFIED — no fast-path code exists yet
grep `isFileDisjoint|disjoint|fastpath` across internal/decompose/ → ZERO hits. This subtask adds the
first piece (the gate predicate). S2-S4 add the staging sweep / concurrency / dispatch wiring; S5 the
regression suite; S6 the docs. S1 is the predicate + its unit test ONLY.

## VERIFIED — decompose.go already imports `prompt` (NO new import needed)
decompose.go imports (29-39): context, errors, fmt, strings + internal/{generate,git,lock,prompt,signal}.
The helper needs only `prompt.PlannerCommit` (already imported) + a `map[string]int` (builtin). ZERO
new imports.

## VERIFIED — co-location anchor
Place `isFileDisjoint` near `runLoop` (decompose.go:456) — both take `[]prompt.PlannerCommit`; the gate
(S4) will sit in Decompose() (line 144) just before the runLoop dispatch. Co-locating keeps the
fast-path helpers grouped.

## THE ALGORITHM (task-specified, literal)
```go
func isFileDisjoint(concepts []prompt.PlannerCommit) bool {
	seen := make(map[string]int)
	for _, c := range concepts {
		for _, p := range c.Files {
			seen[p]++
			if seen[p] > 1 {
				return false
			}
		}
	}
	return true
}
```
- Iterate every concept's Files; track a `map[string]int` occurrence count; return false as soon as any
  path's count exceeds 1 (early exit). Return true if none does.
- Deterministic, no I/O, no git/state mutation, no allocation beyond the map.

## EDGE CASES (the test matrix — all from the task contract + 2 defensive adds)
| Input | Expected | Why |
|-------|----------|-----|
| ≥3 concepts, pairwise-distinct Files | true | FR-M13 disjoint ⇒ fast-path eligible |
| one path in ≥2 concepts (shared) | false | hunk-split intent ⇒ disqualify WHOLE run |
| a concept with empty Files among disjoint others | true | shares no path; stages nothing; FR-M8 empty-skip downstream |
| all concepts empty Files | true | no path appears twice |
| single concept (any Files) | true | only one concept ⇒ nothing to conflict with |
| empty/nil concepts slice | true | vacuously disjoint (no path appears twice) — DEFENSIVE add |
| intra-concept duplicate (one concept lists "a.go" twice) | false | the occurrence-count algorithm counts it as 2 ⇒ false. DEFENSIVE add: documents the chosen behavior (degenerate planner output disqualifies; harmless for a real fast-path but malformed input). |

NOTE on the intra-concept-duplicate case: the task's algorithm is "occurrence count, any >1 ⇒ false"
(literal), which counts intra-concept dupes too. This is defensible (degenerate input) and keeps the
predicate dead-simple. The semantic FR-M13 intent is CROSS-concept sharing (a hunk split); an
intra-concept dupe is harmless for `git add` (idempotent) but is malformed planner output. The test
documents the literal-algorithm behavior (false). If a future revision wanted to dedupe within a
concept first, that's a one-line change — but S1 follows the task's literal algorithm.

## SCOPE BOUNDARIES (sibling subtasks — do NOT implement here)
- **P1.M1.T1.S2**: runLoopFastPath — serial deterministic staging sweep (`git add <files>` + write-tree
  + FR-M1c verifyFreezeSubset + FR-M8 empty-skip). DIFFERENT function.
- **P1.M1.T1.S3**: runLoopFastPath — concurrent message generation + CAS-ordered publish + FR-M12.
- **P1.M1.T1.S4**: Decompose() dispatch — `if isFileDisjoint(concepts) { runLoopFastPath(...) } else { runLoop(...) }`.
  This subtask provides the gate; S4 wires it.
- **P1.M1.T1.S5**: fast-path regression suite (the e2e/integration tests).
- **P1.M1.T1.S6**: docs/how-it-works.md paragraph.
- Do NOT: add any staging/generation/git logic; modify runLoop or Decompose; add config; or change the
  PlannerCommit type. S1 is ONE pure predicate + ONE table-driven unit test.

## TEST HOME + IDIOM
internal/decompose/decompose_test.go (package decompose — internal test ⇒ can call `isFileDisjoint`
directly). The existing tests are heavyweight (real git repos, Deps, stub providers) because they test
the loop. The predicate test is PURE: construct `[]prompt.PlannerCommit` literals, call the function,
assert bool. Table-driven. No t.TempDir, no git, no mocking.