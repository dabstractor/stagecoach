name: "P1.M1.T2.S1 — FR-39a changeset-level docs sweep: README.md + docs/how-it-works.md (CONCLUSION: no change needed)"
description: >
  Mode B documentation sweep for FR-39a (commit-identity transparency). The sweep CONCLUDES THAT NO
  EDIT IS WARRANTED in README.md or docs/how-it-works.md: FR-39a is a defensive/permanent internal
  invariant (no CLI flag, no config key, no output change); neither file makes any AI-branding /
  authorship / "commits as you" claim to retract; and neither has a natural section where stating the
  invariant would be accurate without over-documenting an internal guarantee (the how-it-works "Safety
  invariant" section is about MUTATION safety — a different axis from commit IDENTITY). The canonical
  text lives in PRD §9.9 FR-39a + the in-code tags (internal/git/git.go, P1.M1.T1.S1). This PRP's
  deliverable is the DOCUMENTED DECISION (zero file edits), plus the re-verification steps the
  implementer runs to confirm the conclusion holds at implementation time.

---

## Goal

**Feature Goal**: Formally gate the FR-39a changeset-level documentation question — does README.md or
docs/how-it-works.md need an edit to state the commit-identity-transparency guarantee? — and produce the
correct, documented answer. The sweep (run this session) concludes **NO CHANGE IS NEEDED**: the
invariant is internal, neither file has a relevant surface, and the canonical text already lives in
PRD §9.9 FR-39a + the in-code doc tags (P1.M1.T1.S1). The deliverable is the **documented decision**
(zero file edits), with re-verification steps so the implementing agent confirms the conclusion rather
than assuming it.

**Deliverable**: (i) A re-run of the sweep (broad grep + structural read of both files) at
implementation time; (ii) if confirmed (expected), **ZERO file edits** to README.md and
docs/how-it-works.md, with the decision recorded in the commit message / PR description; (iii) if a
surface has unexpectedly appeared (a sibling edit introduced an authorship/branding claim), the MINIMAL
accurate one-line edit per the item's rules. This is the item's explicitly-permitted outcome (point 3c
+ point 4(ii)): "a documented 'no change needed' decision."

**Success Definition**:
- The implementing agent re-runs the broad grep (§2a of the research notes) and the structural read of
  both files and CONFIRMS there is no commit-identity/authorship/branding surface.
- NO edit is made to README.md or docs/how-it-works.md (the documented no-change outcome).
- The decision is recorded in the commit message / PR description with the verbatim rationale + the
  pointer to where the invariant IS documented (PRD §9.9 FR-39a + git.go tags).
- `git diff --stat README.md docs/how-it-works.md` is empty for this subtask.
- If (unexpectedly) a surface now exists, a minimal accurate edit is made per the item's rules — never
  a fabricated feature claim, never over-documenting an internal invariant.

## User Persona (if applicable)

**Target User**: Stagecoach maintainers / reviewers (this is a documentation-governance gate; no
end-user surface).

**Use Case**: A reviewer asks "did FR-39a get reflected in the overview docs?" The maintainer points at
this subtask's recorded decision: the invariant is internal; the overview docs make no branding claim;
the canonical text is in the PRD + the code tags; no overview-doc edit was warranted.

**Pain Points Addressed**: Prevents two failure modes: (a) shipping a stale/inaccurate branding claim
that FR-39a would contradict (none exists — confirmed); (b) over-documenting an internal invariant by
bolting a "stagecoach never impersonates you" line onto docs that never raised the topic (the item's
explicit anti-pattern).

## Why

- **FR-39a (P0, defensive/permanent)**: stagecoach never sets/overrides/injects a git author/committer
  identity. It is a defensive guarantee codifying EXISTING behavior — it adds no user-facing surface.
  P1.M1.T1.S1 makes it executable (behavioral + structural tests) and tags the code. P1.M1.T2.S1 is the
  cross-cutting-docs gate that confirms or overrides whether the overview docs (README + how-it-works)
  need a corresponding statement.
- **Mode B (changeset-level docs) "does not apply" here**: the PRD §3 mode-B trigger is "a feature
  whose overview docs would otherwise be stale." FR-39a introduces no feature, no flag, no output change,
  and retracts no prior claim (the README makes no AI-branding claim). So the correct Mode B outcome is
  a documented "no change."
- **Avoid over-documentation**: the item explicitly warns against fabricating a feature claim or
  over-documenting an internal invariant. The invariant's documentation home is the PRD (canonical) +
  the code tags (discoverable) — NOT the marketing README or the architecture overview.

## What

**User-visible behavior**: None (documentation governance; zero file edits expected).

**The decision (and the reasoning the implementer must verify):**
1. **README.md** — makes NO claim about commit authorship, AI attribution, or branding. Its commit
   prose is about hooks, decomposition/freeze, plumbing mechanics, atomicity, and the double-run lock.
   No claim to retract; no natural FAQ entry to add without inventing the topic. → no edit.
2. **docs/how-it-works.md** — its "### Safety invariant" section is about MUTATION safety (read-only
   constraint, chrome-less), a DIFFERENT axis from commit IDENTITY. Its "plumbing alternative" section
   describes write-tree/commit-tree/update-ref mechanics and is SILENT on authorship. Per the item's
   rule (3b: "if it is silent or already correct, make NO edit"), neither warrants an edit. → no edit.
3. **The invariant IS documented** — in PRD §9.9 FR-39a (canonical) + the in-code tags at
   internal/git/git.go:518-521 and :672-679 (P1.M1.T1.S1). That is the appropriate home.

### Success Criteria
- [ ] The implementing agent re-runs the broad grep (research §2a) → confirms zero real commit-identity/authorship/branding hits (the single `authoritative` false positive aside)
- [ ] The implementing agent confirms the two candidate sections' headings/content are unchanged and raise no authorship claim
- [ ] NO edit to README.md or docs/how-it-works.md (the documented no-change outcome)
- [ ] The decision is recorded in the commit message / PR description with the rationale + the pointer to PRD §9.9 FR-39a + the git.go tags
- [ ] `git diff --stat README.md docs/how-it-works.md` is empty for this subtask
- [ ] (If a surface unexpectedly exists) a minimal accurate edit is made per the item's rules — no fabricated claim

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact grep to re-run (with the full vocabulary), the two files' structures, the two
candidate sections and why neither warrants an edit, the verbatim decision rationale, the pointer to
where the invariant IS documented, and the explicit anti-pattern (don't over-document) are all below.

### Documentation & References

```yaml
- file: README.md
  why: "Candidate #1 (the marketing/feature surface). RE-RUN the broad grep (research §2a) and read the
        commit-related prose (lines ~70, 190, 378, 380, 418) + the FAQ section. Confirm there is NO
        authorship/branding/commits-as-you claim. The one grep hit (line 327 'authoritative') is a FALSE
        POSITIVE on the 'author' substring — not a commit-identity claim."
  pattern: "README commit prose is about hooks/decomposition/plumbing/atomicity/double-run lock — NOT identity."
  critical: "Do NOT add an FAQ entry or a 'stagecoach never impersonates you' bullet — the README never
             raised the topic, so adding one is OVER-DOCUMENTATION (the item's anti-pattern). The
             invariant's home is the PRD + code tags, not the marketing surface."

- file: docs/how-it-works.md
  why: "Candidate #2 (the plumbing narrative). RE-READ '### Safety invariant' (line ~195) and
        '### The plumbing alternative' (lines ~11-25). The Safety invariant is about MUTATION safety
        (read-only constraint, chrome-less) — a DIFFERENT axis from commit IDENTITY. The plumbing
        section is SILENT on authorship. Per the item's rule (3b: silent ⇒ no edit), neither warrants an edit."
  pattern: "Safety invariant: 'No provider mutates the repository … read-only constraint flags …'.
            Plumbing: 'commit-tree creates a dangling commit object'. Neither mentions author/committer."
  critical: "Do NOT bolt a commit-identity sentence onto the '### Safety invariant' section — it is about
             MUTATION safety; identity is a different axis, and the section is silent on it (not wrong).
             Adding it would be a non-sequitur + over-documentation."

- docfile: plan/018_29b859efcd56/architecture/fr39a_findings.md
  why: "Confirms the invariant's documentation homes: PRD.md §9.9 FR-39a (canonical text, source commit
        347b23f) + the in-code tags (git.go, P1.M1.T1.S1). It does NOT name any README/docs surface —
        confirming the overview docs are not the intended home."

- docfile: plan/018_29b859efcd56/P1M1T1S1/PRP.md
  why: "The sibling contract. S1 adds the in-code FR-39a tags (git.go:518-521, 672-679) + the tests. Its
        doc edits are ONLY the two // comments in git.go — it does NOT touch README.md or docs/how-it-
        works.md (its Anti-Patterns: 'Don't edit README.md/docs/*.md here — the changeset-level docs
        sweep is P1.M1.T2.S1'). No file conflict; S1 provides the code-side documentation home that
        makes this task's 'no overview-doc edit' decision safe."
```

### Current Codebase tree (relevant slice)

```bash
README.md               # candidate #1 — SWEEP (expected: no edit warranted)
docs/how-it-works.md    # candidate #2 — SWEEP (expected: no edit warranted)
internal/git/git.go     # the in-code FR-39a tags (P1.M1.T1.S1) — NOT touched by this task
PRD.md                  # §9.9 FR-39a canonical text — READ-ONLY (never modified by any subtask)
```

### Desired Codebase tree with files to be added

```bash
# (NONE — this subtask writes ZERO files. The deliverable is the documented decision.)
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (the valid outcome is "no change"): the item EXPLICITLY permits a documented "no change
     needed" decision (point 3c + point 4(ii)). Do NOT manufacture an edit just to "do work." The
     invariant is internal; the overview docs make no branding claim; the canonical text is in the PRD
     + code tags. Adding a sentence to README or how-it-works would be the item's named anti-pattern:
     "over-documenting an internal invariant." -->

<!-- CRITICAL (re-verify, don't assume): this PRP's conclusion is based on the tree as read this session.
     The implementing agent MUST re-run the broad grep (research §2a) + re-read the two candidate
     sections at implementation time, in case a sibling edit introduced an authorship/branding claim.
     (No sibling in THIS plan touches these files, but the gate is cheap and the discipline matters.) -->

<!-- GOTCHA (the grep's one false positive): README.md:327 "authoritative" matches the "author"
     substring. It is NOT a commit-authorship claim. Do not "fix" it. The grep must be read for real
     commit-identity/branding surfaces, not bare substring hits. -->

<!-- GOTCHA (mutation-safety ≠ commit-identity): docs/how-it-works.md "### Safety invariant" is about
     whether the repo is MUTATED (read-only constraint, chrome-less). Commit IDENTITY is a different
     axis (WHO the commit is attributed to). Do not conflate them; do not bolt an identity sentence
     onto the mutation-safety section. -->

<!-- GOTCHA (the plumbing section is silent, not wrong): "### The plumbing alternative" describes
     write-tree/commit-tree/update-ref mechanics without mentioning authorship. The item's rule (3b):
     "if it is silent or already correct, make NO edit." Silent ⇒ no edit. -->

<!-- SCOPE: do NOT touch internal/git/git.go (S1 owns the in-code tags). do NOT touch PRD.md (read-only,
     human-owned). do NOT touch any other docs/*.md (per-file docs were Mode A in their implementing
     subtasks; this task sweeps ONLY README.md + docs/how-it-works.md). -->
```

## Implementation Blueprint

### Data models and structure
None. This is a documentation-governance decision. No code, no types, no file edit (expected).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: RE-VERIFY the sweep at implementation time (the gate — do not assume)
  - RE-RUN the broad grep across BOTH files (research §2a vocabulary):
      grep -niE "commit.identity|fr-39a|author|committer|user\.name|user\.email|GIT_AUTHOR|GIT_COMMITTER|co-authored|co-author|generated-by|machine-made|🤖|stagecoach agent|agent@stagecoach|branding|impersonat|on behalf|your behalf|commits as you|commits are yours|as if you|attribution" README.md docs/how-it-works.md
  - EXPECTED: exactly one hit, README.md:327 "authoritative" (the false positive). Zero real hits.
  - If the output matches (expected): the no-surface conclusion is RE-CONFIRMED → proceed to Task 2.
  - If a REAL hit now exists (an authorship/branding claim, unexpected): proceed to Task 3.
  - DEPENDENCIES: none.

Task 2 (EXPECTED PATH): RECORD THE NO-CHANGE DECISION — zero file edits
  - MAKE NO EDIT to README.md or docs/how-it-works.md.
  - VERIFY: git diff --stat README.md docs/how-it-works.md  →  empty.
  - RECORD the decision in the commit message / PR description, verbatim or paraphrased:
      "FR-39a (P1.M1.T2.S1): no changeset-level doc change required — internal invariant, no user-facing
       surface. Sweep of README.md + docs/how-it-works.md found zero commit-identity/authorship/branding
       claims (the one grep hit, README.md:327 'authoritative', is a false positive). The how-it-works
       '### Safety invariant' covers MUTATION safety (a different axis); the plumbing section is silent
       on authorship. Canonical text: PRD §9.9 FR-39a + the in-code tags at internal/git/git.go
       (518-521, 672-679, P1.M1.T1.S1). Adding an overview-doc sentence would over-document an internal
       invariant (the item's named anti-pattern)."
  - DEPENDENCIES: Task 1 (re-verified).

Task 3 (FALLBACK — ONLY if Task 1 found a real surface): make the MINIMAL accurate edit
  - ONLY if a sibling edit introduced an actual authorship/branding claim that FR-39a would contradict:
    (a) In README.md: if an FAQ entry or feature bullet now claims or implies stagecoach sets the commit
        author/branding, add ONE line stating the guarantee (e.g. under the relevant FAQ): "Stagecoach
        never sets or overrides the commit author/committer — every commit is attributed to your own
        configured git identity (FR-39a)." Edit ONLY the inaccurate claim's vicinity.
    (b) In docs/how-it-works.md: if the plumbing/commit section now implies stagecoach sets authorship,
        add ONE line: "author/committer are git's resolved identity (FR-39a); stagecoach sets no identity
        and adds no branding/trailer." Edit ONLY where the implication exists.
  - DO NOT fabricate a feature claim. DO NOT add the sentence to a section that is silent/correct.
  - This path is UNEXPECTED (no sibling in this plan touches these files); if reached, prefer the
    smallest edit that corrects the inaccuracy, then still record the rationale.
  - DEPENDENCIES: Task 1 (found a real surface).
```

### Implementation Patterns & Key Details

```markdown
<!-- This PRP's "implementation" is a DECISION, not a code/doc change. The pattern is:
     1. Re-run the gate (the grep) — don't trust the PRP's stale read.
     2. Confirm the no-surface conclusion.
     3. Record the decision in the commit message / PR description (the deliverable).
     4. Leave README.md and docs/how-it-works.md UNTOUCHED.

     The verbatim decision text for the commit message is in Task 2. The fallback (Task 3) is a
     safety net for an unexpected drift; it is not the expected path. -->
```

### Integration Points

```yaml
NO code / test / config / build changes. ZERO file edits expected (the documented no-change outcome).

DOCUMENTATION HOMES FOR FR-39a (already in place — this task adds nothing):
  - PRD.md §9.9 FR-39a — the canonical requirement text (read-only, human-owned)
  - internal/git/git.go:518-521 + :672-679 — the in-code FR-39a doc tags (P1.M1.T1.S1)

DELIVERABLE:
  - A commit message / PR-description line recording the no-change decision + rationale (Task 2)

UNCHANGED (do NOT touch): README.md; docs/how-it-works.md; internal/git/git.go (S1); PRD.md (read-only);
  any other docs/*.md.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# No code change. The "syntax" gate is the re-verification grep + the empty diff.
grep -niE "commit.identity|fr-39a|author|committer|user\.name|user\.email|GIT_AUTHOR|GIT_COMMITTER|co-authored|co-author|generated-by|machine-made|🤖|stagecoach agent|agent@stagecoach|branding|impersonat|on behalf|your behalf|commits as you|commits are yours|as if you|attribution" README.md docs/how-it-works.md
# Expected: one hit — README.md:327 "authoritative" (false positive). Zero real commit-identity hits.

# The no-change outcome:
git diff --stat README.md docs/how-it-works.md
# Expected: empty (this subtask writes zero files).
```

### Level 2: Unit Tests (Component Validation)

```bash
# No code change — run the suite as a no-regression sanity check (confirms no accidental file touch).
go build ./... && go test ./...
# Expected: clean / all pass (a docs-governance subtask with zero edits cannot affect the build).
```

### Level 3: Integration Testing (System Validation)

```bash
# Not applicable — no runtime behavior change. The within-scope proof is the re-verification grep (Level 1)
# + the structural read confirming the two candidate sections raise no authorship claim.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: confirm the candidate sections are unchanged and raise no authorship claim
grep -nE "^### Safety invariant" docs/how-it-works.md          # the mutation-safety section (NOT identity)
sed -n '/^### Safety invariant$/,/^### /p' docs/how-it-works.md | grep -iE "author|committer|identity|branding" || echo "OK: Safety invariant section has no identity/authorship claim"
# Expected: "OK: …" (the section is about mutation safety, silent on identity).

grep -nE "^### The plumbing alternative" docs/how-it-works.md  # the mechanics section (silent on authorship)
sed -n '/^### The plumbing alternative$/,/^### /p' docs/how-it-works.md | grep -iE "author|committer|identity|branding" || echo "OK: plumbing section silent on authorship"
# Expected: "OK: …" (silent ⇒ no edit per the item's rule 3b).

# Scope-boundary guard: ZERO files changed by this subtask
git diff --stat
# Expected: empty for this subtask (README.md and docs/how-it-works.md untouched; no other file touched).

# Decision-recorded guard: the commit message / PR description carries the no-change rationale
# (manual: confirm the commit message or PR description includes the Task 2 verbatim line.)
```

## Final Validation Checklist

### Technical Validation
- [ ] Broad grep re-run on README.md + docs/how-it-works.md → only the `authoritative` false positive; zero real commit-identity hits
- [ ] `git diff --stat README.md docs/how-it-works.md` empty (zero edits)
- [ ] `go build ./...` / `go test ./...` clean (no-regression sanity; no edit can affect them)

### Feature Validation (the decision)
- [ ] README.md confirmed to make NO authorship/branding/commits-as-you claim (commit prose is hooks/decomposition/plumbing/atomicity)
- [ ] docs/how-it-works.md "### Safety invariant" confirmed to be about MUTATION safety (different axis from identity)
- [ ] docs/how-it-works.md "### The plumbing alternative" confirmed SILENT on authorship (⇒ no edit per item rule 3b)
- [ ] Decision recorded in commit message / PR description with the rationale + the pointer to PRD §9.9 FR-39a + git.go tags

### Scope-Boundary Validation
- [ ] NO edit to README.md
- [ ] NO edit to docs/how-it-works.md
- [ ] NO edit to internal/git/git.go (S1 owns the in-code tags)
- [ ] NO edit to PRD.md (read-only, human-owned)
- [ ] NO edit to any other docs/*.md (this task sweeps ONLY the two named files)

### Documentation Quality
- [ ] The no-change decision is documented (not silently skipped)
- [ ] No fabricated feature claim; no over-documentation of an internal invariant
- [ ] The rationale names WHERE the invariant IS documented (PRD §9.9 FR-39a + git.go tags)

---

## Anti-Patterns to Avoid

- ❌ Don't manufacture an edit to "do work" — the item EXPLICITLY permits a documented "no change needed" outcome (point 3c + 4(ii)). FR-39a is an internal invariant with no user-facing surface; the overview docs make no branding claim. Adding a sentence would be the item's named anti-pattern: "over-documenting an internal invariant."
- ❌ Don't assume the conclusion — RE-RUN the sweep (the broad grep + the structural read) at implementation time. The PRP's read is a snapshot; a sibling edit could (unexpectedly) have introduced a surface. The gate is cheap; run it.
- ❌ Don't treat the `README.md:327 "authoritative"` grep hit as a real commit-identity claim — it is a false positive on the "author" substring. Read the grep output for real branding/authorship surfaces, not bare substring matches.
- ❌ Don't conflate MUTATION safety with commit IDENTITY — docs/how-it-works.md's "### Safety invariant" is about whether the repo is mutated (read-only constraint, chrome-less). Commit identity (who the commit is attributed to) is a different axis. Do not bolt an identity sentence onto the mutation-safety section.
- ❌ Don't edit the "### The plumbing alternative" section either — it is SILENT on authorship (describes write-tree/commit-tree/update-ref mechanics), and the item's rule (3b) says silent ⇒ no edit.
- ❌ Don't touch internal/git/git.go (S1 owns the in-code FR-39a tags), PRD.md (read-only, human-owned), or any other docs/*.md (per-file docs were Mode A in their implementing subtasks; this task sweeps ONLY README.md + docs/how-it-works.md).
- ❌ Don't leave the decision unrecorded — "no change needed" is valid ONLY if it is documented (commit message / PR description) with the rationale + the pointer to where the invariant IS documented. A silent skip is not a gate.
- ❌ Don't add a future-tense "stagecoach will never brand commits" FAQ — that raises a topic the docs never raised and reads as defensiveness about a non-problem. The invariant's home is the PRD + code tags.

---

## Confidence Score: 10/10

The conclusion is overdetermined: a broad grep across both files (the full commit-identity/authorship/
branding vocabulary) returns zero real hits; a structural read of both files' sections confirms neither
raises commit authorship (README's commit prose is hooks/decomposition/plumbing/atomicity; how-it-works's
"### Safety invariant" is mutation-safety, a different axis; the plumbing section is silent on authorship);
and the architecture note confirms the invariant's documentation home is PRD §9.9 FR-39a + the in-code
tags (P1.M1.T1.S1) — not the overview docs. The item explicitly permits the "no change needed" outcome
and explicitly warns against over-documenting an internal invariant. The only implementing-agent job is
to RE-RUN the gate (not assume), confirm, and record the decision — which is deterministic given the
grep. The fallback (Task 3, a real edit) is a safety net for an unexpected drift that no sibling in this
plan can introduce (none touch these two files).