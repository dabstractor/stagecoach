# Research: P1.M1.T2.S1 — Sweep README.md + docs/how-it-works.md for any FR-39a documentation surface

**Scope**: Mode B changeset-level documentation sweep for FR-39a (commit-identity transparency). Decide
whether README.md and/or docs/how-it-works.md need an edit, or whether "no change needed" is the correct
documented outcome. All findings verified against the working tree this session.

## 1. Conclusion: NO CHANGE NEEDED in either file (the documented outcome the item permits)

FR-39a is a **defensive/permanent internal invariant**: stagecoach never writes/overrides git
author/committer identity. It adds **no CLI flag, no config key, no output change**. Neither candidate
file makes any AI-branding / authorship / "commits as you" claim that would need retracting, and neither
has a natural section where stating the invariant would be accurate without over-documenting an internal
guarantee. Per the item's own logic (point 3c / point 4(ii)), **"no change needed" is the valid outcome**.

The canonical requirement text lives in **PRD.md §9.9 FR-39a** (the source), and the **in-code FR-39a
doc tags** land in `internal/git/git.go` (P1.M1.T1.S1 — the runWithInput comment @518-521 + the
CommitTree doc @672-679). That is the appropriate documentation home for an internal invariant — NOT the
marketing README or the architecture overview.

## 2. Sweep methodology + evidence

### 2a. Broad grep (the decisive signal)
A case-insensitive grep across BOTH files for the full commit-identity/authorship/branding vocabulary —
`commit.identity | fr-39a | author | committer | user.name | user.email | GIT_AUTHOR | GIT_COMMITTER |
co-authored | co-author | generated-by | machine-made | 🤖 | stagecoach agent | agent@stagecoach |
branding | impersonat | on behalf | your behalf | commits as you | commits are yours | as if you |
attribution` — returned **exactly ONE hit**, and it is a FALSE POSITIVE:

```
README.md:327: The **authoritative, always-available** reference lives in the binary itself:
```
(`authoritative`, not a commit-authorship claim.) **Zero real commit-identity/authorship/branding
surfaces exist in either file.**

### 2b. README.md structural read (headings)
Sections: 30-second demo · Why not opencommit/aicommits · Features · Install · Quick start · Configure
your agent · **The snapshot workflow** · Full CLI and config reference · Adding a new agent · **FAQ**
(Will it corrupt my repo? · Does it send my code anywhere new? · Does stagecoach make network calls? ·
Can it write multiple commits? · How does it match my project's style? · Which agents are supported? ·
How do I see what command it runs? · Does it run my pre-commit hooks? · What about PR generation...?) ·
Contributing.

The commit-related prose (lines 70, 190, 378, 380, 418) discusses: **commit hooks**, **decomposition /
the T_start freeze**, the **plumbing commands** (write-tree/commit-tree/update-ref), **atomicity /
safety**, and the **double-run lock**. **NONE of it discusses commit AUTHORSHIP or IDENTITY or AI
attribution.** There is no "commits as you" claim, no AI-branding claim, no `stagecoach agent` author
mention, nothing to retract. The "Will it corrupt my repo?" FAQ is about atomicity (byte-for-byte
unchanged on failure), not identity. → **No README edit warranted.**

### 2c. docs/how-it-works.md structural read (headings)
Sections: snapshot-based flow (Why not `git commit` · **The plumbing alternative** · Snapshot
invariants) · Stage-while-generating · Multi-commit decomposition (… **Safety** …) · **Safety and the
rescue protocol** (Per-repo run lock · **Safety invariant** · Failure modes · Rescue protocol) · Prompt
engineering · Multi-turn fallback · Work-description mode · **Commit hooks on the plumbing path** ·
Hook mode vs the snapshot flow.

The two candidate sections:
- **"### Safety invariant" (line 195)** — this is about **MUTATION safety** ("No provider mutates the
  repository … read-only constraint flags … chrome-less …"). Commit IDENTITY is a DIFFERENT axis (who
  the commit is attributed to), not mutation (whether the repo is touched). Adding a commit-identity
  sentence here would be a non-sequitur bolted onto a section about a different invariant. The section
  is **silent on authorship** (neither implies stagecoach sets identity nor claims otherwise).
- **"### The plumbing alternative" (lines 11-25)** — describes `write-tree`/`commit-tree`/`update-ref`
  mechanics and atomicity/freeze. It says `commit-tree` "creates a dangling commit object from the
  frozen tree" — it does NOT describe authorship. Per the item's rule (3b: "if it is silent or already
  correct, make NO edit"), this is silent → no edit.

→ **No docs/how-it-works.md edit warranted.**

### 2d. Architecture confirmation
`plan/018_29b859efcd56/architecture/fr39a_findings.md` documents the invariant's homes: **PRD.md §9.9
FR-39a** (the canonical text, source commit 347b23f) + the **in-code tags** (P1.M1.T1.S1, git.go
comments). It does NOT name any README/docs surface — confirming the overview docs are not the intended
home for this internal invariant.

## 3. Why adding a sentence would be OVER-DOCUMENTATION (the anti-pattern the item warns against)

The item explicitly warns: "No fabricated feature claims; do not over-document an internal invariant."
FR-39a is not a user-facing FEATURE — it is a permanent defensive guarantee that codifies EXISTING
behavior. Adding a "stagecoach never impersonates you" bullet to the README's FAQ, or a commit-identity
sentence to how-it-works.md's Safety invariant, would:
- introduce a mention of a concept (commit identity / AI branding) that NEITHER file currently raises,
  which reads as defensiveness about a non-problem (the files never claimed stagecoach brands commits);
- pollute the "Safety invariant" section (mutation safety) with a different-axis invariant (identity);
- duplicate the canonical text already in PRD.md + the in-code tags.

The RIGHT documentation for an internal invariant that adds no user surface is the code-level tag
(P1.M1.T1.S1) + the PRD — exactly what is already in place.

## 4. COORDINATION WITH THE SIBLING (no conflict)

- **P1.M1.T1.S1** (Implementing): adds the in-code FR-39a doc tags (git.go:518-521, 672-679) + the
  behavioral/structural tests. Its doc edits are **ONLY the two // comments in git.go** — it does NOT
  touch README.md or docs/how-it-works.md (its Anti-Patterns say so explicitly). No file conflict.
- **THIS task (P1.M1.T2.S1):** sweeps README.md + docs/how-it-works.md. Conclusion: **no edit** — so
  this task writes ZERO files. The decision is recorded in the PRP + carried into the commit message /
  PR description.

## 5. The deliverable (a documented decision, not a file edit)

The implementer's job:
1. **Re-run the sweep at implementation time** (the grep + the structural read) to confirm the
   no-surface conclusion still holds (the files could have drifted if another task edited them — though
   no sibling in this plan touches them).
2. If confirmed (expected): **make NO file edit**. Record the decision in the commit message / PR
   description: *"FR-39a: no changeset-level doc change required — internal invariant, no user-facing
   surface; canonical text in PRD §9.9 FR-39a + the in-code tags (internal/git/git.go, P1.M1.T1.S1)."*
3. If a surface HAS appeared (unexpected): make the MINIMAL accurate edit per the item's point 3a/3b
   rules (a one-line statement of the guarantee ONLY where a claim is now inaccurate or a natural
   surface exists), without fabricating a feature claim.

## 6. Validation

- Re-run the broad grep (§2a) → expect the same single false positive (`authoritative`), zero real hits.
- Confirm the two candidate sections' headings are unchanged (README FAQ + "The snapshot workflow";
  how-it-works "Safety invariant" + "The plumbing alternative").
- `git diff --stat README.md docs/how-it-works.md` → **empty** (the documented no-change outcome).
- `go build ./...` / `go test ./...` unaffected (no edit) — run only as a no-regression sanity check.