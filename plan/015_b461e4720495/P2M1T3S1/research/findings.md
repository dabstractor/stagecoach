# Research Findings — P2.M1.T3.S1

**Item:** Improve `verifyFreezeSubset` errors: concept title + clearer phrasing + remedy
**(amended FR-M1c, §9.14)**

All line numbers verified against the working tree on 2026-07-10. This is a SMALL, surgical,
message-only refactor: one function signature gains a `string` param, two `fmt.Errorf` format
strings are rewritten verbatim, the single production call site passes the title, and the test
assertions that pin the OLD phrasing are updated. No control-flow, no git, no new files.

---

## 0. The authoritative contract = the ITEM DESCRIPTION (not delta_prd.md)

`plan/015_b461e4720495/delta_prd.md:120` (an EARLIER draft) suggested a different remedy: *"append
`; unstage the offending path(s) or re-run from a clean trigger (nothing staged)`"*. **The ITEM
DESCRIPTION SUPERSEDES that draft.** The item description is the binding contract: it gives the
COMPLETE verbatim rewrite of BOTH messages (with `(%q)` concept title + the *"This indicates
concurrent working-tree changes were picked up by the stager. Aborting to protect the freeze
boundary."* remedy). Implement the ITEM DESCRIPTION's literal messages exactly. Do NOT use the
delta_prd.md append text.

---

## 1. `verifyFreezeSubset` — current state (`internal/decompose/stager.go`)

**Signature (L159):**
```go
func verifyFreezeSubset(ctx context.Context, deps Deps, baseTree, tStart string, tStartPaths []string, i int, treeI string) error
```

**Doc-comment: L141–157.** Describes the two-part (A) PATH / (B) CONTENT check and says the error
"naming the offending path(s)". Mentions `ErrFreezeViolation`/`ErrDecomposeFailed` wrapping.

**Message (A) PATH — L173–176** (current):
```go
return fmt.Errorf("%w: concept %d staged paths not present in T_start: %s",
    ErrFreezeViolation, i, strings.Join(extra, ", "))
```
Renders → `decompose: freeze violation: concept 0 staged paths not present in T_start: sentinel.txt`

**Message (B) CONTENT — L193–196** (current):
```go
return fmt.Errorf("%w: concept %d staged content not traceable to T_start: %s",
    ErrFreezeViolation, i, strings.Join(mismatch, ", "))
```
Renders → `decompose: freeze violation: concept 0 staged content not traceable to T_start: foo.go`

**Two DiffTreeNames infra-error wraps (L165, L183)** use `i` and `ErrDecomposeFailed`:
`fmt.Errorf("%w: freeze check diff-tree-names[%d]: %w", ErrDecomposeFailed, i, err)`. These are
OUT OF SCOPE — the item rewrites only (A) and (B). Leave them (they are not user-facing freeze
violations; they wrap the orchestrator sentinel). Do NOT add conceptTitle to them.

---

## 2. `ErrFreezeViolation` sentinel — THE critical gotcha for the format string

`internal/decompose/stager.go:60`:
```go
var ErrFreezeViolation = errors.New("decompose: freeze violation")
```
**Its `.Error()` string is literally `"decompose: freeze violation"`.**

The item's verbatim message (A) starts: `decompose: freeze violation in concept %d (%q): ...`.
The `decompose: freeze violation ` prefix is EXACTLY the sentinel string (plus one trailing space).
So to render the item's literal message **while preserving `errors.Is(err, ErrFreezeViolation)`**
(which BOTH existing tests assert: `errors.Is(err, ErrFreezeViolation)` at stager_test.go:410 and
decompose_test.go:767), the `%w` verb must be followed by a SPACE, not the current `%w: ` colon:

```go
// CORRECT — %w + space (NOT %w: colon):
return fmt.Errorf("%w in concept %d (%q): staged paths not in the frozen working-tree snapshot: %s. "+
    "This indicates concurrent working-tree changes were picked up by the stager. "+
    "Aborting to protect the freeze boundary.",
    ErrFreezeViolation, i, conceptTitle, strings.Join(extra, ", "))
```
`%w` → `"decompose: freeze violation"`, so the full render is:
`decompose: freeze violation in concept 0 ("c1"): staged paths not in the frozen working-tree snapshot: sentinel.txt. This indicates concurrent working-tree changes were picked up by the stager. Aborting to protect the freeze boundary.`
— **byte-for-byte the item's literal message (A).** ✅

❌ If the implementer mechanically keeps `%w: ` (colon, the current style), the render becomes
`decompose: freeze violation: in concept 0 (...)` — a double "freeze violation:" / stray colon
that does NOT match the item's literal message. The new messages use `%w ` (space), uniquely.

---

## 3. The new signature + call site

**New signature (add `conceptTitle string` AFTER `i int`, BEFORE `treeI string`):**
```go
func verifyFreezeSubset(ctx context.Context, deps Deps, baseTree, tStart string, tStartPaths []string, i int, conceptTitle string, treeI string) error
```
(Parameter placement per the item description: "Add a `conceptTitle string` parameter … after the
`i int` parameter". `treeI string` is the terminal param today, so `conceptTitle` slots in between
`i` and `treeI`.)

**Single production call site — `internal/decompose/decompose.go:583`** (inside `runLoop`):
```go
if vErr := verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, i, treeI); vErr != nil {
```
→
```go
if vErr := verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, i, concepts[i].Title, treeI); vErr != nil {
```
- `runLoop` signature (L433) already receives `concepts []prompt.PlannerCommit`.
- The loop is `for i, concept := range concepts {` (L567), so BOTH `i` and `concept` are in scope at
  L583. The item description specifies `concepts[i].Title` (equivalent to `concept.Title`); use
  `concepts[i].Title` to match the contract verbatim.
- `prompt.PlannerCommit.Title` exists: `internal/prompt/planner.go:84`
  (`Title string \`json:"title"\`` — "<short concept>").

**Only ONE production caller.** Verified by `grep -rn verifyFreezeSubset internal/` → definition
(stager.go:159) + 1 prod call (decompose.go:583) + 4 test calls (stager_test.go).

---

## 4. New messages — VERBATIM from the item description

**(A) PATH** (replaces L173–176):
```
decompose: freeze violation in concept %d (%q): staged paths not in the frozen working-tree snapshot: %s. This indicates concurrent working-tree changes were picked up by the stager. Aborting to protect the freeze boundary.
```
Format args order: `ErrFreezeViolation` (%w), `i` (%d), `conceptTitle` (%q), `strings.Join(extra, ", ")` (%s).

**(B) CONTENT** (replaces L193–196):
```
decompose: freeze violation in concept %d (%q): staged content differs from the frozen working-tree snapshot for: %s. This indicates concurrent working-tree changes were picked up by the stager. Aborting to protect the freeze boundary.
```
Format args order: `ErrFreezeViolation` (%w), `i` (%d), `conceptTitle` (%q), `strings.Join(mismatch, ", ")` (%s).

**Note the phrasing difference between (A) and (B)** — get this exactly right:
- (A): `... snapshot: %s` (colon, then the path list)
- (B): `... snapshot for: %s` (the word "for", then colon, then the path list)

Both then share the identical suffix: `This indicates concurrent working-tree changes were picked
up by the stager. Aborting to protect the freeze boundary.`

`%q` on a string renders a double-quoted Go-quoted value, e.g. `"c1"` or `"add login"`. Matches the
item's `(%q)`.

---

## 5. Doc-comment update (`internal/decompose/stager.go:141–157`)

The doc-comment's final sentence currently says the error returns *"ErrFreezeViolation-wrapped error
naming the offending path(s)"*. Update it to also note it names the **concept by title** (the new
`conceptTitle` param) and uses the clearer phrasing. Minimal, factual rewrite of the last paragraph
— do not rewrite the algorithm description (the (A)/(B) logic is unchanged).

---

## 6. Test changes — COMPLETE inventory (the item says "two tests"; the truth is 4 call sites + 3 substring updates)

The item's "(d)" mentions "two tests" by approximate line number, but the COMPLETE set of required
test edits (derived from grep, all verified) is:

### 6a. ALL FOUR direct calls in `internal/decompose/stager_test.go` gain the new `conceptTitle` arg
The signature changed, so EVERY direct call MUST add a title argument or it will not compile:
- **L370** `TestVerifyFreezeSubset_Happy`: `verifyFreezeSubset(ctx, deps, baseTree, tStart, tStartPaths, 0, treeI)`
  → add `, "test-concept"` (or any literal) before `treeI`.
- **L406** `TestVerifyFreezeSubset_PathViolation`: same — add the title arg.
- **L452** `TestVerifyFreezeSubset_ContentViolation`: same — add the title arg.
- **L483** `TestVerifyFreezeSubset_EmptyStaging`: same — add the title arg.

(Position the title arg between `0` (the `i int`) and `treeI`, matching the new signature.)

### 6b. Substring-assertion UPDATES (the old phrasing is gone)
- **stager_test.go:416–417** (`TestVerifyFreezeSubset_PathViolation`): asserts
  `"not present in T_start"` → change to a substring of the NEW (A) message, e.g.
  `"staged paths not in the frozen working-tree snapshot"`.
- **stager_test.go:462–463** (`TestVerifyFreezeSubset_ContentViolation`): asserts
  `"not traceable to T_start"` → change to a substring of the NEW (B) message, e.g.
  `"staged content differs from the frozen working-tree snapshot"`.
- **decompose_test.go:773–774** (`TestDecompose_StagerFreezeViolation`): asserts
  `"not present in T_start"` → change to the same NEW (A) substring
  `"staged paths not in the frozen working-tree snapshot"`.

### 6c. Assertions that STAY VALID (do NOT touch — the new messages still name the paths)
- stager_test.go:413 (`"sentinel.txt"`), stager_test.go:460 (`"a.txt"`),
  decompose_test.go:770 (`"sentinel.txt"`) — the new messages keep the `%s` path join.
- All `errors.Is(err, ErrFreezeViolation)` assertions (stager_test.go:410, 456; decompose_test.go:767)
  stay valid because the `%w` wrap is PRESERVED (see §2).

### 6d. RECOMMENDED (optional, low-cost) new assertions
- In `TestVerifyFreezeSubset_PathViolation` / `ContentViolation`: assert the title literal you passed
  (e.g. `"test-concept"`) appears in `err.Error()` — proves the `(%q)` title wiring end-to-end.
- In `TestDecompose_StagerFreezeViolation`: the planner JSON is `{"title":"c1",...}` (decompose_test.go:750),
  so assert `"c1"` appears in `err.Error()` — proves the call-site `concepts[i].Title` plumbing.

These are cheap and directly validate the feature's headline (concept identified by title).

### 6e. Imports
`stager_test.go` already imports `strings` + `errors` (verified L3–L16). `decompose_test.go` already
imports `strings` + `errors`. **No import changes.**

---

## 7. Files touched (scope) — and what MUST NOT be touched

EDIT (3 files):
- `internal/decompose/stager.go` — signature (L159) + message (A) (L173–176) + message (B) (L193–196) + doc-comment (L141–157).
- `internal/decompose/decompose.go` — call site L583 (add `concepts[i].Title`).
- `internal/decompose/stager_test.go` — 4 call-site arg additions (§6a) + 2 substring updates (§6b) + optional (§6d).
- `internal/decompose/decompose_test.go` — 1 substring update (L773–774) + optional title assertion (§6d).

DO NOT TOUCH:
- `internal/prompt/planner.go` (PlannerCommit already has Title).
- The two `ErrDecomposeFailed` DiffTreeNames wraps in verifyFreezeSubset (L165, L183) — out of scope.
- `ErrFreezeViolation` sentinel string (L60) — changing it would break `errors.Is` semantics / the
  `%w`-renders-prefix trick in §2.
- PRD.md, any tasks.json, prd_snapshot.md, delta_prd.md, the architecture/ docs, the CLI.
- No docs surface change (item DOCS: "none — error message improvement"). The
  `docs/how-it-works.md` FR-M1c bullet already exists and needs no edit for phrasing.

---

## 8. Parallel-execution note (P2.M1.T2.S1 is implementing concurrently)

P2.M1.T2.S1 (the FR-M1e empty-index re-check) edits `Decompose()` in decompose.go (around L146) and
adds tests to decompose_test.go. It does NOT touch `verifyFreezeSubset`, its call site (decompose.go:583),
or stager.go. This item (T3.S1) edits the call site at decompose.go:583 + stager.go + the freeze-
violation tests. **The two edits in decompose.go are in disjoint regions** (T2.S1 ~L146 vs T3.S1 L583)
and the test additions are in different functions, so they compose cleanly when both land. No conflict
to design around beyond standard merge hygiene.

---

## 9. Validation commands (verified present in the repo)

```bash
go build ./...
go test ./internal/decompose/... -run 'FreezeSubset|StagerFreezeViolation' -race -v   # the touched tests
go test ./internal/decompose/... -race                                                # full package
go test -race ./...                                                                   # whole repo
gofmt -l internal/decompose/stager.go internal/decompose/stager_test.go internal/decompose/decompose.go internal/decompose/decompose_test.go   # must print nothing
go vet ./internal/decompose/...
git status --porcelain   # ONLY the 3-4 intended files
```
Coverage gate (`make coverage-gate`) gates internal/{git,provider,generate,config}, not decompose —
no gate impact, but internal/decompose tests must stay green.
