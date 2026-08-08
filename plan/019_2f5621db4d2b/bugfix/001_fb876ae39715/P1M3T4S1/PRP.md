name: "P1.M3.T4.S1 — Review and update changeset-level documentation (BUG-001/BUG-002 fast-path fixes)"
description: >
  A DOC-ONLY changeset-level documentation sync ([Mode B]; the contract). The BUG-001/BUG-002 fixes
  (P1.M1.T2.S2 serial EditMessage; P1.M2.T1.S1 serial seenSubjects dedupe) bring the fast-path CODE
  INTO conformance with the spec — and the spec always stated the correct behavior (FR-E4 serialized
  editing; US7/FR32 no duplicate subjects). Per the item's RESEARCH NOTE, docs that describe the SPEC's
  intended behavior should already be correct; this task SWEEPS cross-cutting docs for STALE claims about
  the fast-path and corrects any that describe the buggy behavior. FINDINGS (verified in
  research/findings.md): (1) the SHIPPED docs — README.md + docs/*.md — contain ZERO stale claims (they
  describe FR-E4/US7 as the spec intends, which the fixes restore) ⇒ this portion is a CONFIRMED NO-OP
  (record the review; no edits); (2) the ONE stale claim lives in a TRACKED, CODE-REFERENCED architecture
  overview: plan/019_2f5621db4d2b/architecture/git_primitives.md (the fast-path "Git Primitives &
  Concurrency Model" doc that internal/decompose/decompose.go:743 points readers to). Its concurrency-
  safety analysis (L49-52 + the "Net" L59-63) names `generateMessage` as the goroutine's function and
  concludes "concurrency is safe … does NOT touch the live index. ✅" reasoning ONLY about `.git/index`.
  Post-fix the goroutine calls `generateMessageCore` (no editor); `generateMessage` (message.go:249→259)
  now ALSO applies `EditMessage` (writes shared `.git/STAGECOACH_EDITMSG` + opens `$EDITOR` — NOT
  concurrency-safe, BUG-001) which is deferred to the serial loop (FR-E4). The index-only test is
  necessary-but-NOT-sufficient — the EXACT blind spot that let BUG-001 (EDITMSG shared file) + BUG-002
  (cross-concept dedupe) ship. This task SURGICALLY corrects that one claim (rename generateMessage →
  generateMessageCore; add EditMessage-is-serial + BUG-001/FR-E4 + the index-only blind-spot note +
  BUG-002; cross-ref the now-tightened in-code comment decompose.go~741-757, P1.M3.T3.S1). DOC-ONLY:
  zero code, zero tests, zero spec changes. ZERO overlap with the parallel P1.M3.T3.S1 (edits the
  IN-CODE comment in internal/decompose/decompose.go — a DIFFERENT file; the two are COMPLEMENTARY:
  P1.M3.T3.S1 tightens the in-code comment, this item fixes the .md that comment points to).
  Scope: `git status --porcelain` == plan/019_2f5621db4d2b/architecture/git_primitives.md ONLY (shipped
  docs get zero edits — confirmed accurate).

---

## Goal

**Feature Goal**: Make the changeset-level documentation CONSISTENT with the corrected fast-path code, so
no shipped or code-referenced doc still describes the buggy `generateMessage`-is-concurrency-safe /
index-only reasoning that caused BUG-001 and BUG-002. This is the PRD h2.5 "Recommendations" + the
bugfix's own documentation-sync obligation ([Mode B] cross-cutting sweep).

**Deliverable** (doc-only):
1. **CONFIRM + RECORD the no-op on shipped docs**: README.md + docs/*.md were reviewed (research/findings.md
   §1) and are ACCURATE — they describe FR-E4 (serialized editing) and US7/FR32 (no duplicate subjects) as
   the spec intends, which the fixes restore. No edits. The review is recorded in
   `plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/P1M3T4S1/research/findings.md` (already written).
2. **FIX the ONE stale claim** in `plan/019_2f5621db4d2b/architecture/git_primitives.md`: the concurrency-
   safety analysis that names `generateMessage` and reasons only about `.git/index`. Two small block edits
   (the "message-generation phase" sub-bullet + the "Net:" line) — see Implementation Tasks for verbatim
   current/replacement text.

**Success Definition**:
- Shipped docs (README.md + docs/*.md): ZERO edits, re-confirmed accurate by re-running the §1 grep sweep
  (no `fast-path`/`EDITMSG`/`generateMessage`-is-concurrent stale claim appears).
- `git_primitives.md` concurrency claim names `generateMessageCore` (NOT `generateMessage`) as the
  goroutine's function; states `EditMessage` is serial-only (BUG-001, FR-E4); notes the index-only
  reasoning was the blind spot (BUG-001 EDITMSG shared file + BUG-002 cross-concept dedupe); cross-refs
  the tightened in-code comment (decompose.go ~741-757, P1.M3.T3.S1).
- DOC-ONLY: `make test` + `make lint` green and byte-for-byte unaffected for all CODE (no .go touched).
- `git status --porcelain` == `plan/019_2f5621db4d2b/architecture/git_primitives.md` ONLY (scope guard).

## User Persona (if applicable)

**Target User**: Future maintainers (human or agent) who read `git_primitives.md` — the architecture
overview the production code comment (decompose.go:743) sends them to — to understand WHY the fast-path
can run message generations concurrently. The corrected doc prevents the BUG-001/BUG-002 blind spot from
recurring in the reader's mental model.

**Use Case**: A maintainer reads the launch comment in runLoopFastPath, follows the `git_primitives.md`
pointer to learn the concurrency model, and sees an ACCURATE model (generateMessageCore is the
concurrent-safe core; EditMessage + cross-concept dedupe are serial-only) — so they don't reintroduce a
non-concurrent-safe op into the goroutine.

**User Journey**: maintainer hits decompose.go:743 → opens git_primitives.md → reads "goroutines call
generateMessageCore ONLY; EditMessage is serial (BUG-001); the OLD index-only analysis missed the EDITMSG
shared file + cross-concept dedupe" → designs their change to respect the full contract.

**Pain Points Addressed**: A code-referenced architecture doc that still claims "`generateMessage` …
concurrency is safe … does NOT touch the live index. ✅" actively misleads — it names the wrong function
and reasons about only ONE of the concurrency hazards. This is the doc-level twin of the in-code comment
blind spot P1.M3.T3.S1 closes.

## Why

- **The doc points to by production code must not lie.** `decompose.go:743` cites `git_primitives.md` as
  the authority for the fast-path's read-only-tree-reads invariant. A reader following that pointer today
  reads a concurrency model that (a) names `generateMessage` (the wrapper, which now includes EditMessage)
  where the code calls `generateMessageCore`, and (b) concludes safety from index-only reasoning — the
  exact gap BUG-001/BUG-002 fell through. Fixing the .md completes the documentation half of the bugfix.
- **Cheap, zero-risk.** Doc-only — no code, no tests, no spec. Cannot change behavior. Validation = lint
  clean + grep guards + scope guard.
- **[Mode B] completeness.** This IS the changeset's cross-cutting doc-sync task. The shipped docs are a
  confirmed no-op (they already match the spec the fixes restore); the actionable work is the one
  code-referenced architecture overview.

## What

**User-visible behavior**: none (doc-only).

**Technical change**: edit TWO small blocks in `plan/019_2f5621db4d2b/architecture/git_primitives.md`
(the fast-path concurrency-safety analysis). No code, no tests, no other docs, no spec.

### Success Criteria
- [ ] Shipped docs (README.md + docs/*.md) get ZERO edits; the re-run of the §1 grep sweep shows no stale
      fast-path claim (sanity re-confirmation).
- [ ] `git_primitives.md` "message-generation phase" sub-bullet: names `generateMessageCore` (not
      `generateMessage`); states `EditMessage` is serial-only (BUG-001/FR-E4); notes the index-only
      reasoning missed the `.git/STAGECOACH_EDITMSG` shared-file hazard (BUG-001) + the cross-concept
      dedupe gap (BUG-002).
- [ ] `git_primitives.md` "Net:" line: PROVIDED clause names `generateMessageCore` + "EditMessage is
      serial-only"; conclusion ("SOUND … needs NO new mutex") retained.
- [ ] DOC-ONLY: no `.go`/`go.mod`/`Makefile`/spec/task change; `make test` + `make lint` green.
- [ ] `git status --porcelain` == `plan/019_2f5621db4d2b/architecture/git_primitives.md` ONLY.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim current text of BOTH target blocks in git_primitives.md, the verbatim drop-in
replacements, the confirmed code facts (generateMessage vs generateMessageCore vs EditMessage call sites
with line numbers), the per-file verdict for every shipped doc (all accurate → no-op), the parallel-task
no-conflict confirmation (P1.M3.T3.S1 is a different file), the validation commands, and the grep guards.

### Documentation & References

```yaml
# MUST READ — the per-file review + the exact stale text + the surgical fix (the PRIMARY research artifact).
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/P1M3T4S1/research/findings.md
  why: "§0 the corrected-behavior contract (BUG-001/BUG-002 fixes that LANDED) + the verified code facts
        (generateMessageCore @ message.go:80; generateMessage wrapper @ message.go:249→259 incl. EditMessage;
        fast-path goroutine calls generateMessageCore @ decompose.go:761; EditMessage serial @
        decompose.go:886; runLoop still calls generateMessage @ decompose.go:371/415/515). §1 the SHIPPED-
        docs table — ALL ACCURATE (no-op). §2 the architecture-overview table — git_primitives.md is the
        ONE stale claim; system_context.md accurate (describes runLoop). §3 the EXACT stale text + the
        fix. §4 scope fences. §5 validation."
  critical: "Shipped docs (README.md + docs/*.md) need ZERO edits. The ONLY file to change is
             git_primitives.md. Do NOT edit spec/ (human-owned, already correct), system_context.md
             (correctly describes runLoop), or any code."

# MUST READ — the file under edit (the two target blocks + surrounding context).
- file: plan/019_2f5621db4d2b/architecture/git_primitives.md
  why: "The fast-path 'Git Primitives & Concurrency Model' architecture overview. The STALE block is the
        'message-generation phase' sub-bullet under '⚠️ CRITICAL CONCURRENCY FINDING — there is NO
        in-process index lock' (the 'Implication for the fast-path' list) + the 'Net:' line. EDIT those
        two. The 'Safe-to-concurrentize set' line (git primitives only) is RETAINED (correct)."
  pattern: "The file is GitHub-flavored markdown with `- **bold lead-in**` list items and inline `code`.
            Preserve that style. It cites `file:line` (e.g. `git.go:158`) and FRs/§-refs freely — match
            that convention. The repo's markdownlint config is `.markdownlint.json`."
  gotcha: "The concurrency claim must reference `generateMessageCore` (the LANDED BUG-001 refactor), NOT
           `generateMessage` — the goroutine calls the Core variant; `generateMessage` (the wrapper) now
           includes EditMessage. The 'Safe-to-concurrentize set' line is about GIT PRIMITIVES (index-
           read-only), not message-gen — leave it (it is correct)."

# MUST READ — the production code that POINTS TO git_primitives.md (confirms it is a living, code-referenced doc).
- file: internal/decompose/decompose.go
  section: "Line ~743: the launch-contract comment '(diff(sc.prevTree, sc.tree) — never
            Add/WriteTree/UpdateRef; git_primitives.md)' — the in-code pointer to the doc being fixed.
            Lines ~741-757: the tightened launch contract (P1.M3.T3.S1, parallel) — the corrected .md
            should AGREE with this comment (generateMessageCore ONLY; EditMessage serial; cross-concept
            dedupe serial). READ-ONLY — do NOT edit decompose.go (owned by P1.M3.T3.S1)."
  why: "Confirms (a) git_primitives.md is referenced by production code (so it must be accurate), and
        (b) the corrected .md must not contradict the in-code comment the sibling is tightening."

# MUST READ — the PRD recommendation this implements + the bug context.
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/prd_snapshot.md
  section: "h2.0 Overview + h2.1 BUG-001 (EditMessage shared-file race) + h2.2 BUG-002 (cross-concept
            dedupe loss) + h2.5 Recommendations. The doc fix must NAME BUG-001/BUG-002 + cite FR-E4
            (serialized editing) + US7/FR32 (no duplicate subjects) — the guarantees the fixes restored."

# CONTEXT — the parallel PRP (P1.M3.T3.S1) — confirm ZERO conflict (it edits a DIFFERENT file).
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/P1M3T3S1/PRP.md
  why: "Confirms the sibling edits ONLY internal/decompose/decompose.go (two in-code comment blocks) and
        explicitly scopes 'git status == decompose.go ONLY'. This item edits a PLAN .md — DIFFERENT file.
        The two are COMPLEMENTARY (P1.M3.T3.S1 tightens the in-code comment; this item fixes the .md the
        comment points to). No merge conflict."
  critical: "Do NOT edit internal/decompose/decompose.go (the sibling owns it). This item is
             git_primitives.md ONLY."
```

### Current Codebase tree (relevant slice)

```bash
plan/019_2f5621db4d2b/architecture/
  git_primitives.md            # EDIT — the fast-path concurrency-safety analysis (2 small blocks: the
                               #         "message-generation phase" sub-bullet + the "Net:" line). DOC-ONLY.
  system_context.md            # READ-ONLY — describes runLoop (accurate; fast-path divergence is in-code).
  spec_requirements.md         # READ-ONLY — verbatim spec quotes (correct).
  findings_and_divergences.md  # READ-ONLY — scope/selector notes (no concurrency claim).
  provider_docs_state.md       # READ-ONLY — no keyword hits.
  role_defaults_drift.md       # READ-ONLY — no keyword hits.
README.md + docs/*.md          # READ-ONLY — ALL ACCURATE (research/findings.md §1); ZERO edits.
spec/                          # READ-ONLY — human-owned; already states correct behavior.
internal/decompose/decompose.go # READ-ONLY — owned by parallel P1.M3.T3.S1 (in-code comment).
internal/decompose/message.go  # READ-ONLY — generateMessage/generateMessageCore/EditMessage (verified facts).
.gitignore / Makefile / .markdownlint.json   # READ-ONLY — validation (lint/test; markdownlint config).
```

### Desired Codebase tree with files to be added/modified

```bash
plan/019_2f5621db4d2b/architecture/git_primitives.md   # EDIT — 2 small blocks (concurrency claim). NOTHING ELSE.
# No new files. No shipped-doc edits. No code/test/spec/go.mod/Makefile/PRD/task changes.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (DOC-ONLY — do not touch code): the edit is markdown text in git_primitives.md ONLY. Do NOT
     edit any .go file, any shipped doc (README.md/docs/*.md — confirmed accurate), spec/, system_context.md,
     tasks.json, or prd_snapshot. If you find yourself editing anything but git_primitives.md, STOP. -->

<!-- CRITICAL (reference generateMessageCore, NOT generateMessage, in the concurrency claim): the LANDED
     BUG-001 fix (P1.M1.T1.S1/T2) split generateMessage into generateMessageCore (concurrent-safe, no
     editor) + the EditMessage application (moved to the serial loop). The fast-path goroutine calls
     generateMessageCore (decompose.go:761); generateMessage (message.go:249→259) is the WRAPPER that
     additionally applies EditMessage (NOT concurrency-safe). Naming generateMessage in the concurrency
     claim re-introduces the exact inaccuracy this task removes. -->

<!-- CRITICAL (the index-only test is necessary-but-NOT-sufficient): the STALE claim concluded "concurrency
     is safe … does NOT touch the live index. ✅" — true re: .git/index, but it IGNORED the
     .git/STAGECOACH_EDITMSG shared-file hazard (BUG-001) and the cross-concept dedupe gap (BUG-002).
     The corrected claim must state generateMessageCore is safe BECAUSE it does ONLY read-only tree reads
     + message-agent + per-concept dedupe, and that EditMessage + cross-concept dedupe are serial-only. -->

<!-- GOTCHA (the "Safe-to-concurrentize set" line is about GIT PRIMITIVES, not message-gen): that line
     lists DiffTreeNames/TreeDiff/RevParseTree/DiffTree/CommitTree/UpdateRefCAS as index-read-only git
     primitives. It is CORRECT and is NOT the stale claim — RETAIN it. The stale claim is the
     "message-generation phase" sub-bullet that names generateMessage + the "Net:" line. -->

<!-- GOTCHA (.markdownlint.json governs markdown style): the repo has a markdownlint config. After editing,
     run markdownlint on the file (or `make lint` if it covers plan/) — fix any long-line/heading/style
     violation the edit introduces. The existing file uses long wrapped lines in list items, so match
     the surrounding style (do not reflow the whole file). -->

<!-- GOTCHA (git_primitives.md is a PLANNING artifact but git-TRACKED + code-referenced): 1170 files under
     plan/ are tracked. decompose.go:743 points readers here. So it IS in scope for a changeset doc sync
     ("any architecture overviews") — it is not frozen history. Editing it is the doc-level twin of
     P1.M3.T3.S1's in-code comment tightening. -->
```

## Implementation Blueprint

### Data models and structure

None. Doc-only — no types, no code.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: RE-CONFIRM the shipped-docs no-op (sanity) — README.md + docs/*.md get ZERO edits
  - RUN: the §1 grep sweep from research/findings.md:
      grep -rniE 'fast-path|fast path|disjoint|STAGECOACH_EDITMSG|stagecoach_EDITMSG|EDITMSG|EditMessage|generateMessage|FR-E4|FR-M13|FR-M14' README.md docs/
      grep -rniE 'concurrent.*edit|edit.*concurrent|--edit.*decompose|dedupe.*concurrent' README.md docs/
  - VERIFY: no hit describes the BUGGY behavior (N concurrent editors on one EDITMSG; per-concept-only
    dedupe on the fast-path). The hits are (a) the general FR-E4/US7 guarantees (accurate — the fixes
    restore them), (b) the external-during-run freeze invariant (unrelated), (c) run-lock docs
    (unrelated), (d) `--edit`'s "each commit is gated" (matches FR-E4).
  - ACTION: NONE. Do not edit any shipped doc. The no-op review is already recorded in
    research/findings.md §1. (If a NEW stale claim is somehow found, STOP and surface it — but the
    research found none.)
  - OUTPUT: a one-line mental confirmation that shipped docs are accurate (the deliverable for them is
    the recorded review, not an edit).

Task 1: EDIT the "message-generation phase" sub-bullet in git_primitives.md (the stale concurrency claim)
  - LOCATE: in plan/019_2f5621db4d2b/architecture/git_primitives.md, under the heading "⚠️ CRITICAL
    CONCURRENCY FINDING — there is NO in-process index lock", the "Implication for the fast-path (stage
    serially → generate messages concurrently):" list. Anchor on the SECOND sub-bullet beginning
    "- The **message-generation phase** MUST NOT touch the live `.git/index`."
  - CURRENT (verbatim, to replace — the whole second sub-bullet):
        - The **message-generation phase** MUST NOT touch the live `.git/index`. As long as each goroutine
          reasons only over **tree-to-tree diffs** (`generateMessage` does `diff(tree[i-1], tree[i])`,
          which is read-only tree reads via `DiffTreeNames`/`TreeDiff`) — no `Add`/`ReadTree`/`WriteTree`(live)
          during message gen — concurrency is safe. `generateMessage`'s existing contract already uses
          tree-to-tree diffs (§13.6.3 invariant 2), so it does NOT touch the live index. ✅
  - REPLACE WITH (drop-in — preserves the `DiffTreeNames`/`TreeDiff` read-only framing; renames
    generateMessage → generateMessageCore; adds EditMessage-is-serial + BUG-001/FR-E4 + the index-only
    blind-spot note + BUG-002):
        - The **message-generation phase** MUST NOT touch the live `.git/index` — and must NOT do any
          interactive I/O or write any shared file. Each goroutine calls **`generateMessageCore`** ONLY
          (the BUG-001 refactor, P1.M1.T1.S1/T2.S1): the bare tree-to-tree diff via read-only tree reads
          (`diff(tree[i-1], tree[i])` over `DiffTreeNames`/`TreeDiff`) + the message-agent call + the
          per-concept dedupe loop against a pre-run history snapshot — no `Add`/`ReadTree`/`WriteTree`(live),
          so the `.git/index` is never touched concurrently. `generateMessage` (message.go:249) is the
          WRAPPER that ADDITIONALLY applies `EditMessage`; it is therefore NOT concurrency-safe and the
          fast-path deliberately calls the Core variant. Two operations are held back to the SERIAL
          publish loop (decompose.go ~769-890): **`EditMessage`** — it writes/opens a single shared
          `.git/STAGECOACH_EDITMSG` + an interactive `$EDITOR`, so N concurrent editors on one file
          silently cross-contaminate commit messages (BUG-001); applied one editor at a time in CAS order
          (FR-E4 "serialized publication"). **cross-concept dedupe** — each goroutine sees only the pre-run
          history snapshot, so two disjoint concepts emitting the same subject both pass per-concept dedupe;
          checked incrementally against the growing `seenSubjects` set before publish (US7/FR32, BUG-002).
          ✅ (The ORIGINAL version of this bullet reasoned ONLY about the `.git/index` — "does NOT touch
          the live index" — and named `generateMessage`; that index-only test was necessary-but-NOT-
          sufficient and was the exact blind spot that let BUG-001 and BUG-002 ship. See the tightened
          in-code launch contract at decompose.go ~741-757, P1.M3.T3.S1.)
  - FOLLOW pattern: the file's `- **bold lead-in**` markdown list style; inline `code` for symbols;
    `file:line` + FR/§-refs (match existing citations).
  - GOTCHA: replace the WHOLE second sub-bullet (from "- The **message-generation phase**" through the
    trailing "✅"). Do NOT touch the FIRST sub-bullet (staging sweep) or the THIRD (publishCommit) — they
    are correct. Do NOT touch the "Safe-to-concurrentize set" line below (it is about git primitives).

Task 2: EDIT the "Net:" line in git_primitives.md (name generateMessageCore + EditMessage serial)
  - LOCATE: the "- **Net:** the fast-path's design (…)" line a few lines below the sub-bullets.
  - CURRENT (verbatim, to replace):
        - **Net:** the fast-path's design (serial staging sweep freezes all trees up front → concurrent
          message gen over frozen trees → serial CAS publish) is SOUND and needs NO new mutex, PROVIDED the
          implementer keeps the staging sweep strictly serial and confines message goroutines to tree reads.
  - REPLACE WITH (drop-in — retains the SOUND/NO-mutex conclusion; tightens the PROVIDED clause):
        - **Net:** the fast-path's design (serial staging sweep freezes all trees up front → concurrent
          message gen over frozen trees → serial CAS publish) is SOUND and needs NO new mutex, PROVIDED the
          implementer keeps the staging sweep strictly serial, confines each message goroutine to
          `generateMessageCore` (tree reads + message-agent + per-concept dedupe ONLY), and keeps
          `EditMessage` + the cross-concept `seenSubjects` dedupe in the SERIAL publish loop (BUG-001/FR-E4,
          BUG-002/US7). The original analysis named `generateMessage` and confined its reasoning to the
          `.git/index`; that understated the contract (see above).
  - GOTCHA: retain "SOUND … needs NO new mutex" — the conclusion is still correct (generateMessageCore is
    safe; EditMessage/dedupe are serial). This edit only tightens the PROVIDED clause.

Task 3: VERIFY — doc-only: code/tests unaffected, markdown lint clean, grep guards, scope guard
  - grep -c 'generateMessage' plan/019_2f5621db4d2b/architecture/git_primitives.md   # see Task 3 guards
  - npx markdownlint-cli2 plan/019_2f5621db4d2b/architecture/git_primitives.md 2>/dev/null || make lint
  - make test && make lint                       # green (doc-only — no code touched)
  - git status --porcelain                       # plan/019_2f5621db4d2b/architecture/git_primitives.md ONLY
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN (the corrected concurrency claim — names generateMessageCore; EditMessage + dedupe are serial;
     names both bugs; notes the index-only blind spot; cross-refs the in-code comment): -->
<!--   - The message-generation phase MUST NOT touch the live .git/index — and must NOT do any interactive
       I/O or write any shared file. Each goroutine calls generateMessageCore ONLY (the BUG-001 refactor) …
       generateMessage (message.go:249) is the WRAPPER that ADDITIONALLY applies EditMessage; it is NOT
       concurrency-safe and the fast-path calls the Core variant. Two ops are serial-only: EditMessage
       (BUG-001/FR-E4) + cross-concept dedupe (BUG-002/US7). ✅ (Original reasoned ONLY about .git/index …) -->

<!-- PATTERN (the corrected "Net:" PROVIDED clause — names generateMessageCore + EditMessage/dedupe serial): -->
<!--   … PROVIDED the implementer keeps the staging sweep strictly serial, confines each message goroutine to
       generateMessageCore (… ONLY), and keeps EditMessage + the cross-concept seenSubjects dedupe in the
       SERIAL publish loop (BUG-001/FR-E4, BUG-002/US7). … -->
```

### Integration Points

```yaml
CODE: NONE (doc-only). No .go/go.mod/Makefile/test change.
DOCUMENTATION ([Mode B]):
  - git_primitives.md IS the changeset documentation for the fast-path concurrency model (the .md the
    production code points to). The shipped docs (README.md/docs/*.md) are a confirmed no-op (accurate).
  - COMPLEMENTARY to P1.M3.T3.S1: that tightens the IN-CODE comment (decompose.go); this fixes the .md
    the comment cites. Together they close the doc-level blind spot end-to-end.
SCOPE FENCES:
  - Touches ONLY plan/019_2f5621db4d2b/architecture/git_primitives.md (2 small blocks).
  - Does NOT edit README.md / docs/*.md (confirmed accurate — no-op), spec/ (human-owned), internal/
    decompose/decompose.go (parallel P1.M3.T3.S1), message.go, system_context.md, any PRD/task file.
  - Adds NO code, NO test, NO type, NO import, NO flag, NO dependency.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Doc-only: no code is touched, so the build is byte-for-byte unaffected.
go build ./...
# Expected: clean (identical to before — no code changed).

# Markdown lint (the repo has .markdownlint.json). Run on the edited file.
npx markdownlint-cli2 plan/019_2f5621db4d2b/architecture/git_primitives.md 2>/dev/null \
  || npx -y markdownlint-cli plan/019_2f5621db4d2b/architecture/git_primitives.md \
  || echo "NOTE: markdownlint not installed; rely on make lint below"
# Expected: clean. If a style violation appears (long line, etc.), fix ONLY the lines you added.

# Project lint (covers markdown + go via golangci-lint per .golangci.yml).
make lint
# Expected: zero errors (doc-only — no new code).

# Scope guard: ONLY git_primitives.md changed.
git status --porcelain
# Expected: plan/019_2f5621db4d2b/architecture/git_primitives.md ONLY.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Doc-only: the suite must pass UNCHANGED (no .go touched).
make test
# Expected: green (identical to before).
```

### Level 3: Integration Testing (System Validation)

```bash
# N/A — doc-only change has no runtime behavior. The concurrency invariants the doc DESCRIBES are
# exercised by the BUG-001 regression (P1.M3.T1.S1) + BUG-002 regression (P1.M3.T2.S1) — both
# test-only, both independent of this doc edit. The in-code comment (P1.M3.T3.S1) is the runtime-
# adjacent twin; this .md is the architecture-overview twin.
```

### Level 4: Creative & Domain-Specific Validation (grep guards + no-op re-confirm)

```bash
# Guard 1: the concurrency claim now names generateMessageCore (NOT generateMessage) as the goroutine's call.
grep -n 'generateMessageCore ONLY' plan/019_2f5621db4d2b/architecture/git_primitives.md   # ≥1 hit
# And the OLD inaccurate naming is gone from the concurrency-claim sub-bullet + "Net:" line:
sed -n '/message-generation phase/,/cross-concept/p' plan/019_2f5621db4d2b/architecture/git_primitives.md \
  | grep -n 'generateMessage[^C]' && echo "FAIL: bare generateMessage still in the concurrency claim" \
  || echo "OK: concurrency claim uses generateMessageCore"

# Guard 2: the corrected block names BOTH bugs + the serial-only operations (the recurrence-prevention core).
grep -n 'BUG-001' plan/019_2f5621db4d2b/architecture/git_primitives.md   # ≥1 hit (EditMessage)
grep -n 'BUG-002' plan/019_2f5621db4d2b/architecture/git_primitives.md   # ≥1 hit (cross-concept dedupe)
grep -n 'FR-E4'   plan/019_2f5621db4d2b/architecture/git_primitives.md   # ≥1 hit (serialized editing)

# Guard 3: the blind-spot note is present (the OLD index-only framing is acknowledged as insufficient).
grep -n 'necessary-but-NOT-sufficient\|index-only' plan/019_2f5621db4d2b/architecture/git_primitives.md  # ≥1 hit

# Guard 4: the "Safe-to-concurrentize set" line (git primitives) is RETAINED (it is correct — not the stale claim).
grep -n 'Safe-to-concurrentize set' plan/019_2f5621db4d2b/architecture/git_primitives.md   # 1 hit, unchanged

# Guard 5: SHIPPED-DOCS NO-OP — re-confirm README.md + docs/*.md have no stale fast-path claim (sanity).
grep -rniE 'fast-path|disjoint|STAGECOACH_EDITMSG|stagecoach_EDITMSG|EditMessage|generateMessage' README.md docs/ \
  | grep -iE 'concurrent.*editor|editor.*concurrent|per-concept only|dedupe.*per-concept' \
  && echo "FAIL: stale fast-path claim found in shipped doc" || echo "OK: shipped docs accurate (no-op)"

# Guard 6: DOC-ONLY — no non-markdown file changed.
git status --porcelain
# Expect: plan/019_2f5621db4d2b/architecture/git_primitives.md ONLY.
git diff --name-only | grep -vE '^plan/019_2f5621db4d2b/architecture/git_primitives\.md$' \
  && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean (byte-for-byte unaffected — doc-only)
- [ ] `make lint` zero errors (markdown + go)
- [ ] `make test` green + UNCHANGED (same as pre-edit — no code touched)

### Feature Validation
- [ ] git_primitives.md "message-generation phase" sub-bullet names `generateMessageCore` ONLY as the
      goroutine's call (grep guard 1); states `EditMessage` is serial-only (BUG-001/FR-E4) + cross-concept
      dedupe serial (BUG-002/US7) (grep guard 2); acknowledges the index-only blind spot (grep guard 3)
- [ ] git_primitives.md "Net:" line PROVIDED clause names `generateMessageCore` + serial EditMessage/dedupe
- [ ] The "Safe-to-concurrentize set" line is RETAINED (grep guard 4)
- [ ] Shipped docs (README.md + docs/*.md) re-confirmed accurate — ZERO edits (grep guard 5)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY plan/019_2f5621db4d2b/architecture/git_primitives.md (grep guard 6)
- [ ] NO edit to README.md / docs/*.md (confirmed no-op), spec/ (human-owned), internal/decompose/
      decompose.go (parallel P1.M3.T3.S1), message.go, system_context.md, any PRD/task file

### Code Quality & Docs
- [ ] [Mode B] git_primitives.md + the recorded shipped-docs review ARE the changeset documentation sync
- [ ] The corrected .md AGREES with the tightened in-code comment (decompose.go ~741-757, P1.M3.T3.S1):
      generateMessageCore ONLY; EditMessage serial; cross-concept dedupe serial
- [ ] Markdown style preserved (`.markdownlint.json`); markdownlint/make lint clean
- [ ] The fix cites the PRD guarantees (FR-E4, US7/FR32) + bug IDs (BUG-001/BUG-002) the fixes restored

---

## Anti-Patterns to Avoid

- ❌ Don't edit any shipped doc (README.md / docs/*.md). They were reviewed (research/findings.md §1) and
  are ACCURATE — they describe FR-E4/US7 as the spec intends, which the fixes restore. The deliverable for
  them is the RECORDED REVIEW, not an edit. If you "improve" them, you risk drifting from the spec.
- ❌ Don't reference `generateMessage` (the wrapper) as the goroutine's call in the concurrency claim. The
  LANDED BUG-001 fix split it: the goroutine calls `generateMessageCore` (no editor); `generateMessage`
  (message.go:249→259) additionally applies `EditMessage` (serial-only). Naming `generateMessage` re-
  introduces the exact inaccuracy this task removes.
- ❌ Don't conclude "concurrency is safe" from index-only reasoning. That was the original blind spot
  (BUG-001 EDITMSG shared file; BUG-002 cross-concept dedupe). `generateMessageCore` is safe because it
  does ONLY read-only tree reads + message-agent + per-concept dedupe; EditMessage + cross-concept dedupe
  are serial-only. State the FULL contract.
- ❌ Don't touch the "Safe-to-concurrentize set" line, the "staging sweep" sub-bullet, or the "publishCommit"
  sub-bullet. They are CORRECT (git-primitive-level / staging-serial / CAS-serial). The stale claim is the
  "message-generation phase" sub-bullet + the "Net:" line ONLY.
- ❌ Don't edit `system_context.md`. It correctly describes runLoop (the reference impl, unchanged); the
  fast-path divergence (generateMessageCore) is captured in the in-code comment (P1.M3.T3.S1). Editing it
  would be redundant + risk drift.
- ❌ Don't edit `spec/` (human-owned; AGENTS.md hard rule #1) or `internal/decompose/decompose.go` (owned
  by the parallel P1.M3.T3.S1 — different file, no conflict).
- ❌ Don't reflow or "tidy" the whole git_primitives.md file. Make the TWO targeted block edits only; match
  the surrounding markdown style (long wrapped lines in list items are the file's norm).
- ❌ Don't drop the SOUND/NO-mutex conclusion from the "Net:" line. The design is still sound; this edit
  only tightens the PROVIDED clause (name generateMessageCore + serial EditMessage/dedupe).
- ❌ Don't add regression tests here. The BUG-001 regression is P1.M3.T1.S1 (Complete); the BUG-002
  regression is P1.M3.T2.S1 (Complete). This subtask is the DOC-ONLY sync — TDD = "make test/lint green,
  unchanged" (doc-only).