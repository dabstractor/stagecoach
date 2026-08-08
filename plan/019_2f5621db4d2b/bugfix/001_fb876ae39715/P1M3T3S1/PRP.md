name: "P1.M3.T3.S1 — Tighten runLoopFastPath concurrency-safety comment (EditMessage + dedupe, not just .git/index)"
description: >
  A COMMENT-ONLY documentation fix (PRD h2.5 Recommendation, last bullet; the contract). The
  runLoopFastPath concurrency-safety comment in internal/decompose/decompose.go is fragmented and its
  high-level framing is incomplete after the LANDED BUG-001 (EditMessage → serial loop, P1.M1.T2.S2) and
  BUG-002 (seenSubjects dedupe, P1.M2.T1.S1) fixes: the launch comment frames dedupe as a side-note and
  does not name the two bugs, and the publish-loop overview comment is silent on the two serial-only steps
  (dedupe check + EditMessage). This task TIGHTENS two OVERVIEW comment blocks in decompose.go — it does
  NOT touch code, tests, or the detailed mechanics comments. (1) REWRITE the launch comment (decompose.go
  ~736-742) into a single "FR-M14 CONCURRENCY-SAFETY CONTRACT" block enumerating the invariants the
  contract pins: (a) goroutines call generateMessageCore ONLY (concurrent-safe: read-only tree reads +
  message-agent + per-concept dedupe vs pre-run history; NO EditMessage, NO interactive I/O, NO live
  .git/index — the staging sweep is serial); (b) EditMessage is deferred to the serial publish loop (one
  editor at a time, FR-E4 — BUG-001); (c) cross-concept dedupe is incremental in the serial loop
  (seenSubjects, US7/FR30-33 — BUG-002); (d) the .git/index safety argument holds (staging sweep serial);
  + name both bug IDs so the blind spot recurs less easily; (2) UPDATE the publish-loop comment
  (decompose.go ~769-773) to state the loop is the serialization point for the CAS chain AND the two
  serial-only steps (the seenSubjects dedupe check before publish, and EditMessage one-editor-at-a-time
  before publish, FR-E4). TDD: existing tests pass UNCHANGED (comment-only — no behavior change).
  [Mode A]: the comment IS the documentation update (no README/docs sync — that is P1.M3.T4.S1).
  ZERO overlap with the parallel P1.M3.T2.S1 (TEST-ONLY — adds a test to decompose_test.go; explicitly
  excludes "the concurrency-comment tightening (P1.M3.T3.S1)") and P1.M3.T1.S1 (Complete, test-only).
  Scope: `git status --porcelain` == internal/decompose/decompose.go ONLY.

---

## Goal

**Feature Goal**: Make runLoopFastPath's concurrency-safety documentation ACCURATE and COMPREHENSIVE so
the BUG-001/BUG-002 blind spot (a concurrent goroutine doing something only safe serially) cannot recur
unseen. The current comments are correct but fragmented and the two OVERVIEW blocks under-state the
safety argument: the launch comment treats dedupe as a side-note and never names the bugs, and the
publish-loop overview omits the two serial-only steps entirely.

**Deliverable**: TWO comment-block edits in `internal/decompose/decompose.go` (comment-only — zero code,
zero test, zero behavior change):
1. REWRITE the launch comment (~736-742, the `FR-M14: launch ALL N …` block immediately above the
   `launch := func(...)` closure) into a comprehensive "FR-M14 CONCURRENCY-SAFETY CONTRACT" block.
2. UPDATE the publish-loop comment (~769-773, the `FR-M7: PUBLISH STRICTLY IN CAS ORDER …` block
   immediately above `prevSHA := preRunHEAD`) to name the dedupe check + EditMessage as serial-only steps.

**Success Definition**:
- The launch comment states, in one block: goroutines call `generateMessageCore` ONLY (concurrent-safe:
  read-only tree reads + message-agent + per-concept dedupe vs pre-run history; NO EditMessage / NO
  interactive I/O / NO live .git/index — staging sweep serial); EditMessage deferred to the serial loop
  (BUG-001, FR-E4); cross-concept dedupe incremental in the serial loop (BUG-002, US7/FR30-33); names
  both bug IDs; keeps the fan-out note (no cap; max_commits default 12 bounds N).
- The publish-loop comment states the loop is the serialization point for the CAS chain AND the two
  serial-only steps: the `seenSubjects` dedupe check (before publish) and EditMessage (one editor at a
  time, FR-E4, before publish) — pointing back to the launch contract.
- ZERO code change: `go build ./...` is byte-for-byte unaffected; the existing decompose test suite passes
  UNCHANGED (comment-only).
- `gofmt -l internal/decompose/decompose.go` empty; `go vet ./internal/decompose/...` clean;
  `go test ./internal/decompose/...` green; `make test` + `make lint` clean.
- `git status --porcelain` == `internal/decompose/decompose.go` ONLY.

## User Persona (if applicable)

**Target User**: Future maintainers (human or agent) editing `runLoopFastPath` — especially anyone
tempted to move a NOT-concurrency-safe operation (an editor, a shared-file write, a cross-concept dedupe)
back into the concurrent launch goroutine. The comment is the guardrail that makes them stop and think.

**Use Case**: A maintainer reads runLoopFastPath to add a feature; the launch comment immediately tells
them the concurrency-safety contract (what goroutines may/may not do) and the two historical violations
(BUG-001, BUG-002) so they don't reintroduce one.

**User Journey**: maintainer opens decompose.go → reads the launch block → sees "generateMessageCore
ONLY; EditMessage + dedupe are serial-only (BUG-001/BUG-002)" → designs their change to respect the
contract → no regression.

**Pain Points Addressed**: The PRD h2.5 recommendation — "Tighten the runLoopFastPath concurrency-safety
comment so it accounts for EditMessage and dedupe, not only the .git/index, to prevent the same blind
spot recurring." The original comment reasoned only about the .git/index; the two bugs lived in the
un-analyzed gaps (a shared edit file; cross-concept dedupe).

## Why

- **Prevents recurrence**: BUG-001 and BUG-002 both stemmed from the launch comment's incomplete safety
  argument — it claimed "safe … never touches the live .git/index" and stopped there, leaving EditMessage
  (shared `.git/STAGECOACH_EDITMSG` + interactive `$EDITOR`) and cross-concept dedupe (sibling-blind) as
  un-analyzed hazards. A comment that enumerates ALL the invariants + names the bugs makes the next editor
  of this code check their change against the full contract.
- **Documents the LANDED fixes where they live**: the fixes (P1.M1.T2.S2, P1.M2.T1.S1) added detailed
  inline mechanics comments, but the two OVERVIEW blocks (the entry points a reader hits first) still
  under-stated the safety argument. This task fixes the overview, not the mechanics (which are already
  well-documented and retained).
- **Cheap, zero-risk**: comment-only. Cannot change behavior, cannot break tests. The only validation is
  that the suite still passes + gofmt/vet clean + the comment says the right things (grep guards).
- **[Mode A]**: this comment IS the changeset documentation for the concurrency-safety aspect — no
  separate README/docs sync is owed for it (the broader changeset docs are P1.M3.T4.S1).

## What

**User-visible behavior**: none (comment-only).

**Technical change**: rewrite two `//`-comment blocks in `internal/decompose/decompose.go`. No code, no
tests, no other files. The detailed mechanics comments (the `seenSubjects` BUG-002 accumulator block at
~759-767, the inline dedupe-fix block at ~809+, the EditMessage-fix block at ~858+) are RETAINED — this
task tightens only the two OVERVIEW comments.

### Success Criteria
- [ ] The launch comment block (decompose.go ~736-742, immediately above `launch := func(...)`) is
      rewritten to a single comprehensive block that states: goroutines call `generateMessageCore` ONLY;
      its concurrent-safe ops (read-only tree reads, message-agent, per-concept dedupe vs pre-run
      history); the things it does NOT do (no EditMessage, no interactive I/O, no live .git/index —
      staging sweep serial); and names the two deferred-to-serial operations with their bug IDs
      (EditMessage=BUG-001/FR-E4; cross-concept dedupe=BUG-002/US7-FR30-33); plus the fan-out note.
- [ ] The publish-loop comment block (decompose.go ~769-773, immediately above `prevSHA := preRunHEAD`)
      states the loop serializes the CAS chain AND the two serial-only steps (the `seenSubjects` dedupe
      check before publish; EditMessage one-editor-at-a-time before publish, FR-E4), pointing back to the
      launch contract.
- [ ] The OLD sole-argument framing is GONE: the launch block no longer presents "never touches the live
      .git/index" as the WHOLE safety argument (it is now ONE invariant among several). The goroutine is
      documented as calling `generateMessageCore`, NOT `generateMessage` (grep guard: the launch comment
      block contains no `generateMessage,` — only `generateMessageCore`).
- [ ] ZERO code change: no function signature, statement, or import is altered. `go build ./...` unaffected.
- [ ] The detailed mechanics comments (seenSubjects block ~759-767, inline dedupe ~809+, EditMessage
      ~858+) are RETAINED (this task does not delete them).
- [ ] `gofmt -l internal/decompose/decompose.go` empty; `go vet ./internal/decompose/...` clean;
      `go test ./internal/decompose/...` green (unchanged); `make test` + `make lint` clean.
- [ ] `git status --porcelain` == `internal/decompose/decompose.go` ONLY (scope guard).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim current text of BOTH target comment blocks (with exact line numbers), the verbatim
replacement text for both (drop-in), the mapping of the contract's (a)-(d) to the new launch block, the
confirmation that the fixes are LANDED (so the comment describes real code), the parallel-task no-conflict
confirmation (P1.M3.T2.S1 is test-only on a different file), the validation commands, and the grep guards.

### Documentation & References

```yaml
# MUST READ — codebase-specific findings for THIS item (verbatim current comment blocks + the exact
#              rewrites + the parallel no-conflict confirmation + validation + grep guards).
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/P1M3T3S1/research/findings.md
  why: "§0 the contract; §1 current state (fixes LANDED, comments good-but-fragmented, the 3 existing
        mechanics blocks to RETAIN); §2 the VERBATIM current launch comment (736-742) — the PRIMARY target;
        §3 the VERBATIM current publish-loop comment (769-773) — the SECONDARY target; §4 the parallel
        P1.M3.T2.S1 no-conflict confirmation (test-only on decompose_test.go); §5 the desired rewrites;
        §6 validation; §7 grep guards."
  critical: "This is COMMENT-ONLY. The two target blocks are the launch comment (above `launch := func`)
             and the publish-loop comment (above `prevSHA := preRunHEAD`). Do NOT touch the closure body,
             the seenSubjects call, the for-loop, or any mechanics comment. The fixes are LANDED — the
             comment describes real code; do not 'fix' anything but the comment."

# MUST READ — the PRD recommendation this implements (h2.5, last bullet).
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/prd_snapshot.md
  section: "h2.5 Recommendations (last bullet): 'Tighten the runLoopFastPath concurrency-safety comment
            (decompose.go:737-738) so it accounts for EditMessage and dedupe, not only the .git/index, to
            prevent the same blind spot recurring.' + h2.1 BUG-001 (EditMessage shared-file race) +
            h2.2 BUG-002 (cross-concept dedupe loss) for the bug context the comment must name."
  why: "The comment must NAME BUG-001 (EditMessage) and BUG-002 (dedupe) and cite FR-E4 (serialized
        editing) + US7/FR30-33 (no duplicate subjects) — the exact guarantees the fixes restored."

# MUST READ — the file under edit (the two target comment blocks + the surrounding code, to locate them).
- file: internal/decompose/decompose.go
  why: "Lines ~736-742: the launch comment (FR-M14: launch ALL N …) — REWRITE. Lines ~769-773: the
        publish-loop comment (FR-M7: PUBLISH STRICTLY IN CAS ORDER …) — UPDATE. The closure body
        (launch := func), the seenSubjects block (~759-767), the for-loop, and the inline mechanics
        comments (~809+ dedupe, ~858+ EditMessage) are RETAINED (read-only context)."
  pattern: "The codebase's `//` line-comment style: each line `// ` + tab-indent to match the enclosing
            func body. Multi-line comments are a sequence of `//` lines (NOT `/* */` block comments) —
            preserves gofmt + avoids any `*/`-termination risk. Comments cite PRD FRs + bug IDs freely
            (e.g. 'FR-M14', 'BUG-001', 'FR-E4', 'US7') — match that convention."
  gotcha: "The launch comment block must reference `generateMessageCore` (the LANDED refactor), NOT
           `generateMessage` — the goroutine calls the Core variant. The publish loop applies EditMessage
           + runs the dedupe check (both LANDED). The comment must describe the code as it IS."

# CONTEXT — the parallel PRP (P1.M3.T2.S1) — confirm ZERO conflict (it is test-only on a different file).
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/P1M3T2S1/PRP.md
  why: "Confirms the sibling is TEST-ONLY: 'adds ONE test function to internal/decompose/decompose_test.go
        (NO new file, NO production change)' + 'NOT in scope: the concurrency-comment tightening
        (P1.M3.T3.S1)' + 'git status == decompose_test.go ONLY.' → this item's single-file edit
        (decompose.go) does NOT overlap. No merge conflict."
  critical: "Do NOT edit decompose_test.go (the sibling owns it during its implementation). This item is
             decompose.go ONLY."
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  decompose.go        # EDIT — rewrite 2 OVERVIEW comment blocks (launch ~736-742; publish-loop ~769-773). COMMENT-ONLY.
  decompose_test.go   # READ-ONLY — owned by the parallel P1.M3.T2.S1 (test-only); DO NOT TOUCH
  message.go          # READ-ONLY — generateMessage/generateMessageCore/EditMessage call sites (context for the comment)
  git_primitives.md   # READ-ONLY — referenced by the launch comment (read-only-tree-reads invariant)
go.mod / Makefile / .golangci.yml   # READ-ONLY — validation (gofmt/vet/test/lint)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/decompose/decompose.go   # EDIT — 2 comment blocks rewritten (launch contract + publish-loop overview). NOTHING ELSE.
# No new files. No test changes. No other production files. No go.mod/Makefile/PRD/task changes.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (COMMENT-ONLY — do not touch code): the two edits are // comment blocks ONLY. Do NOT alter
// the launch closure body, the seenSubjects call, the for-loop, any statement, any signature, or any
// import. A comment-only change must leave `go build` byte-for-byte unaffected. If you find yourself
// editing a non-comment line, STOP — you are out of scope.

// CRITICAL (reference generateMessageCore, NOT generateMessage, in the launch comment): the LANDED
// BUG-001 fix (P1.M1.T2.S1/T2) split generateMessage into generateMessageCore (concurrent-safe, no
// editor) + the EditMessage application (moved to the serial loop). The goroutine calls generateMessageCore.
// Naming generateMessage in the launch contract would re-introduce the very inaccuracy this task fixes.

// CRITICAL (retain the detailed mechanics comments): the seenSubjects BUG-002 block (~759-767), the
// inline dedupe-fix block (~809+), and the EditMessage-fix block (~858+) document HOW the fixes work.
// This task tightens the two OVERVIEW comments; it does NOT delete or merge the mechanics blocks.

// GOTCHA (gofmt enforces // comment indentation): each comment line is `\t// text` (tab + // + space +
// text), indented to the enclosing function body. A mis-indented or bare-text line fails gofmt. Run
// `gofmt -w` after the edit if `gofmt -l` lists the file.

// GOTCHA (// line-comments only — no /* */ block comments): the codebase uses // line-comments
// throughout decompose.go. A /* */ block comment risks an accidental */ termination and diverges from
// the file's style. Use // lines (one per line).

// GOTCHA (the line numbers ~736-742 / ~769-773 are APPROXIMATE — they shift as comments are edited):
// LOCATE the target blocks by their stable ANCHOR text, not line number: the launch block is the
// `// FR-M14: launch ALL N` comment immediately above `launch := func(i int, treeA, treeB string)`;
// the publish-loop block is the `// FR-M7: PUBLISH STRICTLY IN CAS ORDER` comment immediately above
// `prevSHA := preRunHEAD`. Match on these anchors.
```

## Implementation Blueprint

### Data models and structure

None. Comment-only — no types, no code.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: REWRITE the launch comment block (decompose.go ~736-742) — the comprehensive concurrency-safety contract
  - LOCATE: the `// FR-M14: launch ALL N (non-skipped) message generations CONCURRENTLY.` block (the lines
    BETWEEN the `// --- P1.M1.T1.S3: ...` separator above and the `launch := func(i int, treeA, treeB
    string) chan msgOut {` line below). Anchor on the FIRST line `// FR-M14: launch ALL N` and replace
    through the `// No cap (FR-M14; max_commits default 12 bounds N).` line.
  - CURRENT (verbatim, to replace):
        // FR-M14: launch ALL N (non-skipped) message generations CONCURRENTLY. Each goroutine calls
        // generateMessageCore — the bare generate/dedupe core (tree-to-tree diff, read-only tree reads, never
        // touches the live .git/index; git_primitives.md). seedRejections is nil here (cross-concept dedupe is
        // applied in the serial publish loop — P1.M2). EditMessage is DELIBERATELY NOT called in the goroutine:
        // it writes/opens a single shared .git/STAGECOACH_EDITMSG and is not concurrency-safe, so it is deferred
        // to the serial publish loop (one editor at a time, FR-E4 serialized publication; P1.M1.T2.S2).
        // No cap (FR-M14; max_commits default 12 bounds N).
  - REPLACE WITH (drop-in — tab-indented // line-comments; covers contract a/b/c/d + names both bugs):
        // FR-M14 CONCURRENCY-SAFETY CONTRACT — why launching all N (non-skipped) message generations
        // concurrently is safe, and what is deliberately held back to the serial publish loop. Both bugs that
        // prompted this block (BUG-001, BUG-002) were a goroutine doing something that is only safe serially;
        // this contract enumerates the invariants so the blind spot recurs less easily.
        //
        // Each goroutine calls generateMessageCore ONLY — the bare generate + per-concept dedupe core. It is
        // concurrent-safe because it does THREE things and no more: (1) read-only tree reads
        // (diff(sc.prevTree, sc.tree) — never Add/WriteTree/UpdateRef; git_primitives.md); (2) the message-agent
        // call; (3) the per-concept duplicate-rejection loop against a PRE-RUN history snapshot
        // (seedRejections is nil here). It does NO interactive I/O and touches NO live .git/index — the staging
        // sweep above is strictly serial (FR-M13), so the index is never mutated concurrently.
        //
        // The two NOT-concurrency-safe operations are DEFERRED to the serial publish loop below:
        //   - EditMessage (BUG-001): it writes/opens a single shared .git/STAGECOACH_EDITMSG + an interactive
        //     $EDITOR; N concurrent editors on one file silently cross-contaminate commit messages. Applied one
        //     editor at a time in CAS order in the serial loop (FR-E4 "serialized publication"; P1.M1.T2.S2).
        //   - cross-concept dedupe (BUG-002): each goroutine sees only the pre-run history snapshot, so two
        //     disjoint concepts emitting the same subject both pass per-concept dedupe. Checked incrementally in
        //     the serial loop — each concept's subject is judged against seenSubjects (pre-run history + the
        //     already-decided siblings) BEFORE publish, restoring US7/FR30-33 (P1.M2.T1.S1).
        //
        // No cap on the fan-out (FR-M14; max_commits default 12 bounds N).
  - FOLLOW pattern: the file's `//` line-comment style (tab + `// ` + text); multi-paragraph via blank `//`
    separator lines (already used elsewhere in the file). Cite FRs + bug IDs freely.
  - GOTCHA: anchor on the `// FR-M14: launch ALL N` first line + the `// No cap (FR-M14; ...)` last line —
    replace THAT range only. Do NOT touch the `// --- P1.M1.T1.S3: ---` separator above or the closure below.
  - VERIFY: `grep -n 'generateMessageCore ONLY' internal/decompose/decompose.go` (1 hit, in the new block);
    `grep -n 'BUG-001' + 'BUG-002'` (both present in the new block).

Task 2: UPDATE the publish-loop comment block (decompose.go ~769-773) — name the two serial-only steps
  - LOCATE: the `// FR-M7: PUBLISH STRICTLY IN CAS ORDER (concept order).` block immediately above
    `prevSHA := preRunHEAD`. Anchor on the first line `// FR-M7: PUBLISH STRICTLY` and replace through the
    `// time, so arm signal + fix the rescue parent HERE, in this serial loop).` line.
  - CURRENT (verbatim, to replace):
        // FR-M7: PUBLISH STRICTLY IN CAS ORDER (concept order). The publish loop is the serialization
        // point: commit[i] parent = prevSHA (preRunHEAD/root for i=0, newSHA[i-1] otherwise); each CAS
        // requires HEAD == prevSHA. prevSHA is AUTHORITATIVE for the rescue parent (see findings §5 —
        // runLoop's 1-deep overlap guarantees it at launch; the concurrent path knows it only at publish
        // time, so arm signal + fix the rescue parent HERE, in this serial loop).
  - REPLACE WITH (drop-in — adds the dedupe check + EditMessage as serial-only steps; preserves the
    CAS/rescue-parent content + the findings §5 cross-ref):
        // FR-M7: PUBLISH STRICTLY IN CAS ORDER (concept order). The publish loop is the serialization
        // point for EVERYTHING that cannot run concurrently in the launch phase above: (1) the CAS chain —
        // commit[i] parent = prevSHA (preRunHEAD/root for i=0, newSHA[i-1] otherwise), each CAS requires
        // HEAD == prevSHA; (2) the cross-concept dedupe check against the growing seenSubjects set
        // (BUG-002 — see the launch contract above), run BEFORE publish; and (3) EditMessage applied one
        // editor at a time (BUG-001, FR-E4), also before publish. prevSHA is AUTHORITATIVE for the rescue
        // parent (see findings §5 — runLoop's 1-deep overlap guarantees it at launch; the concurrent path
        // knows it only at publish time, so arm signal + fix the rescue parent HERE, in this serial loop).
  - FOLLOW pattern: same // line-comment style; keep the existing `findings §5` cross-ref.
  - GOTCHA: this is an UPDATE (preserve the CAS/rescue-parent sentences verbatim) — ADD the dedupe +
    EditMessage enumeration. Do not drop the prevSHA/rescue-parent content.

Task 3: VERIFY — comment-only: build unaffected, gofmt/vet clean, suite unchanged, lint clean, grep guards
  - go build ./...                                  # unaffected (comment-only)
  - gofmt -l internal/decompose/decompose.go        # empty; if listed → gofmt -w
  - go vet ./internal/decompose/...                 # clean
  - go test ./internal/decompose/...                # green + UNCHANGED (comment-only — same pass/fail as before)
  - make test && make lint                          # green
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the comprehensive launch contract — enumerates invariants + names the bugs; the goroutine
// calls generateMessageCore, NOT generateMessage):
//   // FR-M14 CONCURRENCY-SAFETY CONTRACT — why launching all N … concurrently is safe, and what is
//   // deliberately held back to the serial publish loop. Both bugs that prompted this block (BUG-001,
//   // BUG-002) were a goroutine doing something only safe serially …
//   //
//   // Each goroutine calls generateMessageCore ONLY — … (1) read-only tree reads; (2) message-agent;
//   // (3) per-concept dedupe vs pre-run history. NO interactive I/O, NO live .git/index (staging sweep
//   // is serial — FR-M13).
//   //   - EditMessage (BUG-001) → serial loop, one editor at a time (FR-E4).
//   //   - cross-concept dedupe (BUG-002) → serial loop, seenSubjects incremental (US7/FR30-33).

// PATTERN (the publish-loop overview — names the CAS chain + the two serial-only steps):
//   // FR-M7: PUBLISH STRICTLY IN CAS ORDER. The publish loop is the serialization point for EVERYTHING
//   // that cannot run concurrently: (1) the CAS chain; (2) the seenSubjects dedupe check (BUG-002), before
//   // publish; (3) EditMessage one editor at a time (BUG-001, FR-E4), before publish. prevSHA is …
```

### Integration Points

```yaml
CODE: NONE (comment-only). No signature, statement, import, or test change.
DOCUMENTATION ([Mode A]):
  - The launch comment + publish-loop comment ARE the changeset documentation for runLoopFastPath's
    concurrency safety. No README/docs sync is owed for THIS subtask (the broader changeset docs are
    P1.M3.T4.S1).
SCOPE FENCES:
  - Touches ONLY internal/decompose/decompose.go (2 comment blocks).
  - Does NOT edit decompose_test.go (parallel P1.M3.T2.S1), message.go, generate/finalize.go, any other
    production file, go.mod, Makefile, or any PRD/task file.
  - Adds NO code, NO test, NO type, NO import, NO flag, NO dependency.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Comment-only: build must be byte-for-byte unaffected.
go build ./...
# Expected: clean (identical to before — no code changed).

# Format (the // comment indentation is the only thing gofmt could object to).
gofmt -l internal/decompose/decompose.go
# Expected: empty. If listed: gofmt -w internal/decompose/decompose.go (then re-check).

# Vet.
go vet ./internal/decompose/...
# Expected: clean.

# Lint.
make lint
# Expected: zero errors (comment-only — no new symbols, no unused anything).

# Scope guard: ONLY decompose.go changed.
git status --porcelain
# Expected: internal/decompose/decompose.go ONLY. ZERO changes to decompose_test.go / message.go / any
#           other file.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Comment-only: the suite must pass UNCHANGED (same pass/fail as the pre-edit HEAD).
go test ./internal/decompose/... -race
# Expected: green (identical results to before — comments do not affect execution).

# Full race suite + lint.
make test
# Expected: green.
```

### Level 3: Integration Testing (System Validation)

```bash
# N/A — comment-only change has no runtime behavior to integration-test. The unit suite (Level 2) is the
# proof the change is behavior-neutral. (The concurrency invariants the comment DOCUMENTS are exercised by
# the BUG-001 regression test P1.M3.T1.S1 and the BUG-002 regression test P1.M3.T2.S1 — both test-only,
# both independent of this comment edit.)
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: the launch comment names BOTH bugs + the deferred operations (the recurrence-prevention core).
grep -n 'BUG-001' internal/decompose/decompose.go   # ≥1 hit in the launch block
grep -n 'BUG-002' internal/decompose/decompose.go   # ≥1 hit in the launch block
grep -n 'FR-E4' internal/decompose/decompose.go     # ≥1 hit in the launch block (EditMessage serialized)
grep -n 'US7\|FR30' internal/decompose/decompose.go # ≥1 hit in the launch block (dedupe guarantee)

# Guard 2: the launch comment references generateMessageCore (NOT generateMessage) as the goroutine's call.
grep -n 'generateMessageCore ONLY' internal/decompose/decompose.go   # 1 hit in the launch block
# And the OLD inaccurate framing is gone — the launch block must NOT say the goroutine calls generateMessage:
sed -n '/FR-M14 CONCURRENCY-SAFETY CONTRACT/,/No cap on the fan-out/p' internal/decompose/decompose.go | grep -n 'generateMessage[^C]' || echo "OK: no bare generateMessage in the launch contract"

# Guard 3: the launch comment enumerates the concurrent-safe ops + the .git/index invariant.
grep -n 'read-only tree reads' internal/decompose/decompose.go       # in the launch block
grep -n 'staging sweep' internal/decompose/decompose.go              # in the launch block (.git/index safety, FR-M13)

# Guard 4: the publish-loop comment names BOTH serial-only steps (dedupe + EditMessage).
sed -n '/FR-M7: PUBLISH STRICTLY IN CAS ORDER/,/in this serial loop)/p' internal/decompose/decompose.go | grep -n 'seenSubjects'
sed -n '/FR-M7: PUBLISH STRICTLY IN CAS ORDER/,/in this serial loop)/p' internal/decompose/decompose.go | grep -n 'EditMessage'
# Expect: 1 hit each (both serial-only steps named in the publish-loop overview).

# Guard 5: COMMENT-ONLY — no non-comment line changed (the diff is all `^[-+]\s*//`).
git diff internal/decompose/decompose.go | grep -E '^[-+]' | grep -vE '^[-+]\s*//|^[-+]{3}' && echo "FAIL: a non-comment line changed" || echo "OK: comment-only diff"
# Expect: OK (every -/+ line is a // comment or the diff header).

# Guard 6: scope — only decompose.go.
git status --porcelain
# Expect: internal/decompose/decompose.go ONLY.
git diff --name-only | grep -vE '^internal/decompose/decompose\.go$' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean (byte-for-byte unaffected — comment-only)
- [ ] `gofmt -l internal/decompose/decompose.go` empty
- [ ] `go vet ./internal/decompose/...` clean
- [ ] `make lint` zero errors
- [ ] `go test ./internal/decompose/... -race` green + UNCHANGED (same as pre-edit)
- [ ] `make test` green

### Feature Validation
- [ ] Launch comment states: goroutines call `generateMessageCore` ONLY; concurrent-safe ops (read-only
      tree reads, message-agent, per-concept dedupe vs pre-run); NO EditMessage/interactive I/O/live
      .git/index (staging sweep serial) (contract a + d) (grep guards 2,3)
- [ ] Launch comment names EditMessage (BUG-001, FR-E4) + cross-concept dedupe (BUG-002, US7/FR30-33) as
      deferred-to-serial (contract b + c) (grep guard 1)
- [ ] Publish-loop comment names the seenSubjects dedupe check + EditMessage as serial-only steps
      (grep guard 4)
- [ ] The OLD sole-.git/index framing is gone; the goroutine is documented as generateMessageCore (grep guard 2)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY internal/decompose/decompose.go (grep guard 6)
- [ ] The diff is ALL `//` comment lines (no non-comment line changed) (grep guard 5)
- [ ] NO edit to decompose_test.go (parallel P1.M3.T2.S1), message.go, generate/finalize.go, go.mod, any
      PRD/task file
- [ ] The detailed mechanics comments (seenSubjects block, inline dedupe, EditMessage block) are RETAINED

### Code Quality & Docs
- [ ] [Mode A] The two comment blocks ARE the changeset documentation for runLoopFastPath concurrency safety
- [ ] Comments follow the file's // line-comment style (tab + `// ` + text); gofmt-clean
- [ ] Comments cite the PRD FRs (FR-M13/M14/E4) + bug IDs (BUG-001/002) + guarantees (US7/FR30-33) the fixes restored

---

## Anti-Patterns to Avoid

- ❌ Don't touch any code. This is COMMENT-ONLY. Do not alter the launch closure body, the seenSubjects
  call, the for-loop, any statement, signature, or import. The diff must be all `//` lines (grep guard 5).
  If you find yourself editing a non-comment line, you are out of scope.
- ❌ Don't reference `generateMessage` (the wrapper) as the goroutine's call in the launch contract. The
  LANDED BUG-001 fix split it: the goroutine calls `generateMessageCore` (no editor); EditMessage is in
  the serial loop. Naming `generateMessage` re-introduces the exact inaccuracy this task removes.
- ❌ Don't delete or merge the detailed mechanics comments. The seenSubjects BUG-002 block (~759-767), the
  inline dedupe-fix block (~809+), and the EditMessage-fix block (~858+) document HOW the fixes work. This
  task tightens the two OVERVIEW comments only — the mechanics stay.
- ❌ Don't present "never touches the live .git/index" as the WHOLE safety argument. That was the original
  comment's blind spot (it left EditMessage + dedupe un-analyzed → BUG-001/BUG-002). The new launch
  contract enumerates ALL the invariants and names both bugs.
- ❌ Don't use `/* */` block comments. The file uses `//` line-comments throughout; a block comment risks
  an accidental `*/` termination and diverges from the style. Use `//` lines.
- ❌ Don't locate the blocks by line number alone. Comments shift as you edit. Anchor on the stable text:
  the launch block is `// FR-M14: launch ALL N` (above `launch := func`); the publish-loop block is
  `// FR-M7: PUBLISH STRICTLY IN CAS ORDER` (above `prevSHA := preRunHEAD`).
- ❌ Don't drop the CAS/rescue-parent content from the publish-loop comment. Task 2 is an UPDATE (add the
  dedupe + EditMessage steps), not a replacement — preserve the prevSHA/rescue-parent/`findings §5` sentences.
- ❌ Don't edit `decompose_test.go`. It is owned by the parallel P1.M3.T2.S1 (test-only, in flight). This
  item is `decompose.go` ONLY — zero merge conflict by construction.
- ❌ Don't add regression tests here. The BUG-001 regression is P1.M3.T1.S1 (Complete); the BUG-002
  regression is P1.M3.T2.S1 (parallel). This subtask is the comment only — TDD = "existing tests pass
  unchanged" (the contract's explicit wording).
- ❌ Don't sync README/docs here. The broader changeset documentation is P1.M3.T4.S1. [Mode A]: this
  comment IS the documentation update for the concurrency-safety aspect.