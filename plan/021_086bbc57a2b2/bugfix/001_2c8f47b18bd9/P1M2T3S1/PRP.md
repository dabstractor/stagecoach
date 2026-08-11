name: "P1.M2.T3.S1 — Review and update README.md and docs/how-it-works.md for decompose fast-path accuracy (BUG-003 docs sync, FR-M13)"
description: >
  Documentation-sync/audit task for the BUG-003 (FR-M13) decompose fix. SCOPE: README.md and
  docs/how-it-works.md ONLY. Audit both files for any statement that decompose UNCONDITIONALLY requires a
  stager-capable (tooled) provider; if found, update to note the FR-M13 file-disjoint fast-path lets a
  no-`tooled_flags` provider decompose a disjoint partition (no stager invoked). PRE-COMPUTED FINDING
  (research verified): the docs ALREADY accurately reflect FR-M13 — docs/how-it-works.md:140 ("File-disjoint
  fast-path") explicitly states a no-`tooled_flags` provider can decompose a disjoint tree, matching
  spec/SPEC.md:24 (v3.2 row) and spec/03-generation.md:120; README makes no unconditional claim. So the
  task's "If docs already match the spec, no changes needed" branch applies. PRIMARY deliverable = the audit
  (confirm accuracy). OPTIONAL value-add = ONE short discoverability note after the roles table
  (docs/how-it-works.md:64) so a reader who stops at the stager=tooled table row doesn't misinfer a tooled
  provider is mandatory — exact wording provided, applied only if it reads cleanly and does not duplicate
  line 140. BOTH terminal states (no-op audit summary OR the optional note) are acceptable per the contract.
  Pure Markdown; no code/build/test impact. docs/providers.md, docs/cli.md, and any .go file are OUT OF SCOPE.

---

## Goal

**Feature Goal**: Ensure README.md and docs/how-it-works.md do not claim — explicitly or by strong
implication — that multi-commit decompose *unconditionally* requires a stager-capable (tooled) provider, so
user docs reflect the FR-M13 file-disjoint fast-path: a no-`tooled_flags` provider (opencode, a user-defined
provider) can decompose a **pairwise file-disjoint** partition via deterministic `git add` with no stager
invoked (shared-file partitions still fall back to the tooled-stager loop).

**Deliverable**: An AUDIT of README.md + docs/how-it-works.md confirming they already reflect FR-M13 (they
do — see Context), with EITHER (A) a clean no-op + audit summary, OR (B) one optional, surgical,
discoverability note inserted after the roles table in `docs/how-it-works.md` (~line 64). NO edits to code,
spec, or other docs.

**Success Definition**:
- An audit grep of README.md + docs/how-it-works.md finds NO statement asserting decompose *requires* a
  tooled/stager-capable provider unconditionally (the roles-table `tooled` mode label is a role-contract
  descriptor, not a requirement claim).
- `docs/how-it-works.md:140` (the "File-disjoint fast-path" note) remains intact and accurate — it already
  carries the FR-M13 no-tooled-provider consequence.
- IF the optional note (B) is applied: Markdown is well-formed, the `#key-design-points` link resolves, the
  note does not contradict line 140, and `git diff --name-only` ⊆ {README.md, docs/how-it-works.md}.
- `git diff` touches no `.go` file, no `spec/**`, no `docs/providers.md`, no PRD/tasks.json/prd_snapshot.md.

## User Persona (if applicable)

**Target User**: A developer whose default agent is a no-`tooled_flags` provider (opencode, or a custom
user-defined provider) reading the docs to learn whether they can use `stagecoach` (decompose) at all.

**Use Case**: User reads README's "Multi-commit decomposition" section + the how-it-works roles table, sees
`stager | tooled`, and asks "do I need a stager-capable provider to decompose?"

**User Journey**: README (high-level) → how-it-works roles table → (optional note: stager is conditional,
disjoint ⇒ skipped, no-tooled OK) → "Key design points / File-disjoint fast-path" (full mechanism). The user
learns their no-tooled provider works for the common disjoint case.

**Pain Points Addressed**: A reader who stops at the roles table (`stager | tooled`) could misinfer decompose
is unavailable to them. The optional note closes that discoverability gap; the audit confirms no *accuracy*
defect exists.

## Why

- **BUG-003 / FR-M13 (v3.2)**: the file-disjoint fast-path exists precisely so a no-tooled provider can
  decompose a disjoint tree without a stager. P1.M2.T1 (the code fix) made that reachable by deferring the
  eager stager-resolution error; this task ensures the *user-facing docs* don't re-assert the old
  unconditional requirement the code just stopped enforcing.
- **Docs-as-contract hygiene**: user docs are the first thing a provider-restricted user reads. A stale
  unconditional "needs a tooled provider" would drive them away from a feature that now works for them.
- **Bounded, low-risk**: pure Markdown audit + (optionally) one paragraph. No code, no tests, no schema.

## What

**User-visible behavior**: None at runtime (documentation). Reader-facing: if the optional note is applied,
the how-it-works roles table is immediately followed by a one-paragraph clarification that the stager is
conditional and disjoint partitions need no tooled provider.

**Technical change**: ZERO code. At most ONE Markdown paragraph in `docs/how-it-works.md` (after the roles
table, ~line 64). README.md requires no change (it makes no unconditional claim).

### Success Criteria
- [ ] Audit grep confirms README.md + docs/how-it-works.md contain no unconditional "decompose requires a
      tooled/stager-capable provider" claim
- [ ] `docs/how-it-works.md:140` (File-disjoint fast-path note) is intact and accurate (not weakened)
- [ ] (IF optional note applied) the note is well-formed Markdown, links to `#key-design-points`, and does
      not contradict line 140 or duplicate it beyond a short pointer
- [ ] `git diff --name-only` ⊆ {README.md, docs/how-it-works.md}; NO code/spec/providers.md/PRD change

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim FR-M13 spec clauses to mirror (§Documentation), the pre-computed audit result (the docs
are already accurate; line 140 is the proof), the exact optional-edit wording + placement + anchor, the audit
grep commands, and the explicit scope fences (providers.md / code / spec are OUT).

### Documentation & References

```yaml
# MUST READ — the authoritative research (audit result + optional edit + validation)
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M2T3S1/research/findings.md
  why: "§0 the FR-M13 spec clauses (source of truth); §1 the AUDIT RESULT (docs already accurate — line 140
        is the proof; README clean; providers.md out-of-scope); §2 heading/anchor structure (no
        #file-disjoint-fast-path anchor exists → link to #key-design-points); §3 the optional edit's exact
        wording + placement; §4 validation greps; §5 scope fences."
  critical: "§1: the docs ALREADY match the spec. The task's 'no changes needed' branch is the
             substantively-correct outcome. The optional note (§3) is a discoverability aid, NOT an accuracy
             fix — apply only if it reads cleanly. §1d: do NOT scope-creep into docs/providers.md (out of scope)."

# MUST READ — the FR-M13 spec (the wording the docs must match)
- file: spec/SPEC.md
  why: "Line 24 (v3.2 revision row): '…it lets a provider whose manifest declares no tooled_flags (FR-D4 —
        opencode, qwen-code) can now decompose a disjoint tree, where it otherwise could not serve the
        stager role…' — the canonical statement the docs must not contradict."
  gotcha: "READ-ONLY (spec is human-owned; never edit). The docs must MATCH it, not weaken it."

# MUST READ — the FR-M13 mechanism detail (the prose line 140 mirrors)
- file: spec/03-generation.md
  why: "Line 120 ('File-disjoint fast-path (FR-M13/M14)'): the full mechanism — disjoint partition ⇒
        deterministic git add, no stager invoked; shared file ⇒ tooled-stager fallback. This is what
        docs/how-it-works.md:140 restates."

# MUST READ (the only candidate edit site) — the roles table + the already-correct fast-path note
- file: docs/how-it-works.md
  why: "Line 57 '### The four roles' → the table at line 62 (stager row: mode=tooled). Line 64 = end of
        table (the OPTIONAL note inserts here, before '### Pipeline flow' at line 66). Line 136
        '### Key design points' → the bold **File-disjoint fast-path (/)** paragraph at line 140 ALREADY
        documents the no-tooled-provider consequence accurately — DO NOT weaken, remove, or duplicate it."
  pattern: "The doc structure is: roles table (role contracts) → pipeline diagram → key design points
            (incl. the fast-path note). The table's 'tooled' label is a role-contract descriptor, not a
            requirement claim."
  gotcha: "There is NO '#file-disjoint-fast-path' anchor — the fast-path is a bold paragraph inside
           '### Key design points', not its own heading. Any link must target '#key-design-points' (resolves
           to line 136) or be plain text. README's existing link 'docs/how-it-works.md#multi-commit-decomposition'
           (README:59) is correct and unchanged."

# CONFIRMING (no edit needed) — README makes no unconditional claim
- file: README.md
  why: "Lines 34/35/59/211/422 mention the four-role pipeline (planner → stager → message → arbiter) and the
        stager's *constraints* (line 211: the claude allowlist / pi instructional guard), but assert NO
        unconditional 'decompose requires a tooled provider'. README defers architecture to how-it-works.md."
  critical: "READ-ONLY for this task (no edit required/expected). If a future unconditional claim appears,
             fix it here too — but as of this audit, README is clean."

# CONTEXT (out of scope — do NOT edit) — providers.md carries pre-BUG-003 phrasing
- file: docs/providers.md
  why: "Lines 31/70/107 describe the stager *role contract* ('tooled_flags nil/empty ⇒ cannot serve as a
        stager; render errors at invocation time'). The 'render errors at invocation time' reflects the
        PRE-BUG-003 eager error, which P1.M2.T1/S2 now DEFERS to runLoop (errors only when a stager is
        genuinely needed for a non-disjoint partition)."
  critical: "OUT OF SCOPE for P1.M2.T3.S1 (scope = README + how-it-works ONLY). If the providers.md
             invocation-time-error phrasing should be updated to reflect the deferral, that is a SEPARATE
             task. Do NOT edit providers.md in this PRP."

# CONTEXT — the code fix this docs task tracks (already shipped behavior; no code dependency for the docs edit)
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M2T1S1/PRP.md
  why: "BUG-003: ResolveRoles no longer hard-errors for a no-tooled stager with no tooled fallback (defers
        to runLoop via the StagerAvailable sentinel). The docs describe the *behavior* the code implements;
        the fast-path itself has shipped since v3.2, so the docs edit does not depend on the code PR being merged."
```

### Current Codebase tree (relevant slice)

```bash
# DOCS (scope of this task):
README.md                  # AUDIT ONLY — no unconditional claim found; expect NO edit
docs/how-it-works.md       # AUDIT + (optional) ONE note after the roles table (~line 64); line 140 already accurate
# OUT OF SCOPE (do not edit):
docs/providers.md          # carries pre-BUG-003 'render errors at invocation time' phrasing — separate task
docs/cli.md, docs/configuration.md   # not in scope
spec/SPEC.md, spec/03-generation.md  # READ-ONLY (human-owned spec)
**/*.go                    # NO code change (this is a docs task)
```

### Desired Codebase tree with files to be added/modified

```bash
# OPTIONALLY MODIFIED (no new files):
docs/how-it-works.md       # +1 short paragraph after the roles table (OPTIONAL; no-op is equally acceptable)
# (README.md — no change expected)
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (the docs are ALREADY accurate — do not "fix" a non-defect): docs/how-it-works.md:140 already
     states the FR-M13 no-tooled-provider consequence verbatim. The PRIMARY outcome is an audit that confirms
     this. Adding a second full fast-path explanation would DUPLICATE line 140 — the optional note must be a
     short POINTER (≤4 sentences), not a restatement. If in doubt, NO-OP. -->

<!-- CRITICAL (anchor gotcha): there is NO '#file-disjoint-fast-path' anchor. The fast-path is a bold
     paragraph (**File-disjoint fast-path (/).**) INSIDE '### Key design points' (line 136), not a heading.
     A link to '#file-disjoint-fast-path' would be BROKEN. Link to '#key-design-points' (resolves to the
     section) or use plain text. README's existing link docs/how-it-works.md#multi-commit-decomposition
     is correct and must NOT be changed. -->

<!-- GOTCHA (MkDocs Material table syntax): the roles table is a GFM table (line 62). A note inserted
     immediately AFTER it must be separated by a blank line from the table's last row, and the next heading
     ('### Pipeline flow') must also be separated by a blank line. Verify the table still renders (no stray
     pipe broke a row). -->

<!-- GOTCHA (do not scope-creep into providers.md): docs/providers.md:107 says a no-tooled provider 'cannot
     serve as a stager (render errors at invocation time)'. That describes the stager *role contract* and
     reflects pre-BUG-003 behavior. It is OUT OF SCOPE (separate task). Editing it here would exceed this
     PRP's scope fence. -->

<!-- GOTCHA (the roles-table 'tooled' label is NOT a defect): the table row '| stager | tooled | ...' is a
     role-contract descriptor (the stager role IS tooled when it runs). It is accurate. Do NOT change it to
     'conditional' or remove 'tooled' — that would mis-describe the role. The optional NOTE (not a table
     edit) is the correct surface for the clarification. -->
```

## Implementation Blueprint

### Data models and structure
None. Pure Markdown. No types, no code.

### Implementation Tasks (ordered — audit first, then decide)

```yaml
Task 1: AUDIT README.md + docs/how-it-works.md for an unconditional "decompose requires a tooled provider" claim
  - RUN: grep -rniE 'cannot (decompose|serve|stage)|tooled_flags|requires? a (tooled|stager)|(must|need) be tooled|no tooled' README.md docs/how-it-works.md
  - EVALUATE each hit against the FR-M13 spec (spec/SPEC.md:24, spec/03-generation.md:120): does it assert
    decompose REQUIRES a tooled/stager-capable provider UNCONDITIONALLY (ignoring the disjoint fast-path)?
  - EXPECTED RESULT (pre-computed by research): NO unconditional claim. The only candidate surface is the
    roles-table 'tooled' mode label (docs/how-it-works.md:62) — a role-contract descriptor, NOT a requirement
    claim — and it is already clarified 80 lines later by the fast-path note (line 140). README is clean.
  - DECISION POINT:
      (A) If the audit finds an unconditional claim the research missed → make the surgical edit (update the
          offending sentence to note the disjoint fast-path for no-tooled providers; mirror spec/SPEC.md:24).
      (B) If the audit confirms docs already match the spec (expected) → proceed to Task 2 (optional note),
          OR close with a no-op audit summary (both acceptable per the contract "If docs already match the
          spec, no changes needed").

Task 2 (OPTIONAL discoverability note — apply only if it reads cleanly; a no-op is equally acceptable):
  EDIT docs/how-it-works.md — insert ONE short paragraph immediately AFTER the roles table (after line 64,
  before '### Pipeline flow' at line 66). Separate from the table and the heading by blank lines.
  - INSERT (verbatim; adjust only if a prior edit shifted line numbers — anchor by the table, not the number):
      **The stager is conditional, not mandatory.** It is the pipeline's only *tooled* role, but it runs only
      when a concept shares a file with another. On a pairwise **file-disjoint** partition — the common case
      for cleanly separated changes — the stager is skipped entirely (each concept is staged deterministically
      with `git add`), so a provider with no `tooled_flags` (e.g. a user-defined provider) can still decompose
      a disjoint tree. See the *File-disjoint fast-path* note under [Key design points](#key-design-points)
      below.
  - PRESERVE: the roles table (line 62) UNCHANGED (do not edit the 'tooled' label — it is a correct
    role-contract descriptor); the pipeline diagram (lines 96-134) UNCHANGED; the fast-path note (line 140)
    UNCHANGED (this note is a POINTER to it, not a duplication).
  - GOTCHA: the link target is '#key-design-points' (the heading at line 136), NOT '#file-disjoint-fast-path'
    (that anchor does not exist). Verify the anchor resolves.

Task 3: VERIFY — audit greps, markdown well-formedness, scope fence
  - Re-run the Task 1 grep → still no unconditional requirement claim.
  - Confirm docs/how-it-works.md:140 (fast-path note) is intact + accurate.
  - (If Task 2 applied) confirm: table still renders; note separated by blank lines; '#key-design-points'
    resolves; note does not contradict line 140 (disjoint ⇒ stager skipped, no-tooled OK; shared ⇒ tooled loop).
  - git diff --name-only → ⊆ {README.md, docs/how-it-works.md}. NO .go / spec / providers.md / PRD change.
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: the optional note is a forward-reference POINTER, not a restatement.
     It summarizes in ≤4 sentences and links to the full mechanism (line 140). It exists so a reader who
     stops at the roles table learns the stager is conditional — it does NOT re-explain the fast-path. -->

<!-- PATTERN: accuracy phrasing — "disjoint partition ⇒ stager skipped (deterministic git add); shared file
     ⇒ tooled-stager loop (the role contract above still holds)." This mirrors spec/03-generation.md:120 and
     spec/SPEC.md:24 exactly. Never say "decompose never needs a tooled provider" — that's FALSE for
     shared-file partitions. -->

<!-- PATTERN: the roles-table 'tooled' label stays. It describes the stager ROLE (tooled when it runs). The
     clarification lives in a NOTE after the table, not by editing the table cell — editing the cell would
     mis-describe the role contract. -->
```

### Integration Points

```yaml
NO code / types / config / routes / deps / migration / CLI flags. At most ONE Markdown paragraph.

DOCS (docs/how-it-works.md):
  - (optional) one note after the roles table (~line 64) pointing to the file-disjoint fast-path (line 140).

SCOPE FENCES: NO README.md edit expected (clean per audit); NO docs/providers.md (separate task — its
  'render errors at invocation time' phrasing reflects pre-BUG-003 behavior, out of scope here); NO docs/cli.md
  or docs/configuration.md; NO .go code; NO spec/ (READ-ONLY); NO PRD.md / tasks.json / prd_snapshot.md.
NO overlap with the parallel P1.M2.T2.S1 (a code-comment fix in internal/decompose/decompose.go — different
  file, different concern) or P1.M2.T1.S1/S2/S3 (Go code in roles.go/decompose.go).
```

## Validation Loop

> A Markdown-only change cannot break Go semantics. Validation = the audit grep confirms no unconditional
> claim, the existing accurate fast-path note (line 140) is intact, and (if the optional note is applied) the
> Markdown is well-formed with a resolving anchor.

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Audit grep — confirm no unconditional "decompose requires a tooled provider" claim remains.
grep -rniE 'cannot (decompose|serve|stage)|tooled_flags|requires? a (tooled|stager)|(must|need) be tooled|no tooled' README.md docs/how-it-works.md
# Expected: NO hit that asserts decompose REQUIRES a tooled provider unconditionally. (The roles-table
#           'tooled' mode label at how-it-works.md:62 is a role-contract descriptor — not a match for these
#           patterns; it is accurate and stays. providers.md hits are out of scope — not grepped here.)

# Confirm the existing accurate fast-path note is intact (the FR-M13 no-tooled-provider consequence).
grep -n 'decompose a disjoint tree it otherwise could not serve' docs/how-it-works.md
# Expected: 1 hit (line ~140, the File-disjoint fast-path note — unchanged).

# Scope guard: only docs changed (no code/spec/PRD).
git diff --name-only
# Expected: ⊆ {README.md, docs/how-it-works.md}. (README.md expected UNCHANGED → typically just
#           docs/how-it-works.md, or empty if the no-op branch was taken.)
git diff --name-only | grep -E '\.go$|^spec/|providers\.md|PRD|tasks\.json|prd_snapshot'
# Expected: empty (no out-of-scope file touched).
```

### Level 2: Unit Tests (Component Validation)

```bash
# N/A — pure documentation. No Go test is affected. (A docs change cannot alter behavior.)
# If the repo has a markdown-lint / docs-build step, run it on the changed file:
#   (project-specific) e.g. markdownlint docs/how-it-works.md || npx mkdocs build --strict
# Confirm the table still renders (no stray pipe) and headings/anchors are intact.
```

### Level 3: Integration Testing (System Validation)

```bash
# N/A for a docs change. The Go binary/tests are unaffected:
go build ./...   # sanity — must still build (it will; no code changed)
# Expected: succeeds. (Run only if you suspect an accidental code touch; otherwise skip.)

# If a docs-build / site-preview exists, render it and confirm the roles table + the optional note display
# correctly and the '#key-design-points' link navigates to the Key design points section.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard 1 (THE audit gate): no unconditional requirement claim.
grep -niE 'decompose.*(requires|needs|must).*(tooled|stager)' README.md docs/how-it-works.md
# Expected: empty (no statement that decompose unconditionally requires a tooled/stager provider).

# Grep guard 2 (optional note applied — confirm presence + correct anchor):
grep -n 'The stager is conditional, not mandatory' docs/how-it-works.md
grep -n 'key-design-points' docs/how-it-works.md
# Expected (if Task 2 applied): 1 hit each. (If the no-op branch was taken: 0 hits — acceptable.)

# Grep guard 3 (anchor integrity — '#key-design-points' is a real heading):
grep -nE '^### Key design points' docs/how-it-works.md
# Expected: 1 hit (line ~136). The note's link target resolves.

# Grep guard 4 (no duplication / contradiction): the fast-path note (line 140) is the SINGLE full mechanism;
#   the optional note is only a pointer.
grep -c 'invoking no stager agent at all' docs/how-it-works.md
# Expected: 1 (only line 140 — the note does not restate "invoking no stager agent").

# Grep guard 5 (scope — README expected clean / unchanged):
git diff --stat -- README.md
# Expected: empty (README makes no unconditional claim → no edit). If non-empty, re-audit README.
```

## Final Validation Checklist

### Technical Validation
- [ ] Audit grep (Level 1) finds NO unconditional "decompose requires a tooled provider" claim
- [ ] `docs/how-it-works.md:140` (File-disjoint fast-path note) intact and accurate
- [ ] `go build ./...` still succeeds (sanity — no accidental code touch)
- [ ] `git diff --name-only` ⊆ {README.md, docs/how-it-works.md}

### Feature (Docs-Accuracy) Validation
- [ ] README.md + docs/how-it-works.md reflect FR-M13: a no-`tooled_flags` provider can decompose a disjoint partition
- [ ] No doc contradicts the spec (spec/SPEC.md:24, spec/03-generation.md:120)
- [ ] (IF optional note applied) the note is well-formed Markdown, links to `#key-design-points`, and does not duplicate/contradict line 140

### Scope-Boundary Validation
- [ ] NO `.go` file changed
- [ ] NO `spec/**` edit (READ-ONLY)
- [ ] NO `docs/providers.md` edit (out of scope — separate task)
- [ ] NO `docs/cli.md` / `docs/configuration.md` edit
- [ ] NO `PRD.md` / `tasks.json` / `prd_snapshot.md` edit
- [ ] No overlap with the parallel P1.M2.T2.S1 (decompose.go comment) or P1.M2.T1.S1/S2/S3 (Go code)

### Code Quality & Docs
- [ ] The roles-table `tooled` label is preserved (it is a correct role-contract descriptor, not edited)
- [ ] Any added note is a short pointer (≤4 sentences), not a restatement of the fast-path mechanism
- [ ] Anchor links resolve (no broken `#file-disjoint-fast-path` link)

---

## Anti-Patterns to Avoid

- ❌ Don't "fix" a non-defect. The docs ALREADY accurately reflect FR-M13 (`docs/how-it-works.md:140`). The
  PRIMARY deliverable is the audit that confirms this. A no-op + audit summary is a fully acceptable outcome
  per the task contract ("If docs already match the spec, no changes needed"). Don't manufacture an edit to
  justify the task.
- ❌ Don't duplicate the fast-path mechanism. If you apply the optional note, make it a ≤4-sentence POINTER
  to line 140 ("See the File-disjoint fast-path note under Key design points below"), not a second full
  explanation. Two full explanations of the same mechanism invite drift.
- ❌ Don't edit the roles-table `tooled` cell. `| stager | tooled | …` is a correct role-contract descriptor
  (the stager role IS tooled when it runs). Changing it to "conditional" or stripping "tooled" would
  mis-describe the role. The clarification belongs in a NOTE after the table, not in the table cell.
- ❌ Don't use a `#file-disjoint-fast-path` anchor. That anchor does NOT exist — the fast-path is a bold
  paragraph (`**File-disjoint fast-path (/).**`) inside `### Key design points` (line 136), not a heading.
  Link to `#key-design-points` (resolves) or use plain text. A broken anchor is worse than no link.
- ❌ Don't scope-creep into `docs/providers.md`. Its line 107 ("cannot serve as a stager (render errors at
  invocation time)") reflects pre-BUG-003 behavior and describes the stager *role contract*. Updating it to
  reflect the BUG-003 deferral is a SEPARATE task — out of scope for P1.M2.T3.S1.
- ❌ Don't weaken the spec's accuracy. The docs must MATCH spec/SPEC.md:24 + spec/03-generation.md:120
  ("disjoint ⇒ stager skipped, no-tooled provider OK; shared file ⇒ tooled-stager loop"). Never write
  "decompose never needs a tooled provider" — that is FALSE for shared-file partitions and would introduce a
  new inaccuracy.
- ❌ Don't touch README.md unless the audit finds an actual unconditional claim there. Pre-computed: README is
  clean (lines 34/35/59/211/422 describe the pipeline + stager constraints, not a requirement). An unneeded
  README edit adds review noise.
- ❌ Don't anchor the optional note to a line number. Lines drift. Anchor by the roles table (the
  `### The four roles` → table → insert before `### Pipeline flow` structure), located via
  `grep -n '### The four roles' docs/how-it-works.md`.

---

## Confidence Score: 9/10

This is a bounded docs-audit task with a pre-computed result (the docs already match FR-M13 — verified against
spec/SPEC.md:24 + spec/03-generation.md:120, with `docs/how-it-works.md:140` as the proof and a cross-doc grep
confirming no unconditional claim in README/how-it-works). The only edit surface (the optional discoverability
note) is given as verbatim wording + exact placement + a verified anchor (`#key-design-points`). The risk
surface is small and fully fenced: no code, no spec, no providers.md, no PRD. The one residual (not a full 10):
because the optional note is genuinely optional, two terminal states (no-op vs. the note) are both acceptable,
which is slightly less deterministic than a single prescribed diff — but the contract explicitly permits both,
and the audit greps + the "no duplication / no broken anchor" guards make either outcome verifiably correct.
One-pass success is highly likely.