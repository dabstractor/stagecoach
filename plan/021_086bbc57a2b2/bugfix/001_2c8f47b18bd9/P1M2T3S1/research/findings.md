# P1.M2.T3.S1 — Research Findings

**Task**: Review README.md + docs/how-it-works.md for any statement that decompose *unconditionally*
requires a stager-capable (tooled) provider; if found, update to note the FR-M13 file-disjoint fast-path
allows no-`tooled_flags` providers for disjoint partitions. If docs already match the spec → no changes.

---

## §0 — The FR-M13 spec (the source of truth the docs must reflect)

- **spec/SPEC.md:24 (v3.2 revision row)**: "Adds the **file-disjoint staging fast-path** to multi-commit
  decomposition (§9.14 new FR-M13/FR-M14…). … When the partition is **pairwise file-disjoint** … stagecoach
  stages each concept **deterministically** with `git add` … **invoking no stager agent at all** … Side
  effect: a provider whose manifest declares no `tooled_flags` (FR-D4 — opencode, qwen-code) can now
  decompose a disjoint tree, where it otherwise could not serve the stager role."
- **spec/03-generation.md:120 ("File-disjoint fast-path (FR-M13/M14)")**: verbatim match of the above —
  "it lets a provider with no `tooled_flags` … decompose a disjoint tree it otherwise could not serve."

**The claim the docs must NOT contradict**: decompose does NOT *unconditionally* require a tooled
(stager-capable) provider — a no-`tooled_flags` provider can decompose a **pairwise file-disjoint** partition
via the fast-path (no stager agent invoked). A shared-file (non-disjoint) partition DOES still need a tooled
stager (falls back to the FR-M5 loop).

---

## §1 — AUDIT RESULT: README.md and docs/how-it-works.md ALREADY accurately reflect FR-M13

### 1a. docs/how-it-works.md:140 — the fast-path IS documented (and accurate)

`docs/how-it-works.md` "**File-disjoint fast-path (/)**" key-design-point (the bold lead-in paragraph under
`### Key design points`, line 136) states verbatim:

> "…When that partition is **pairwise file-disjoint** — no path appears in two concepts' `files` — stagecoach
> stages each concept **deterministically** with `git add` … **invoking no stager agent at all** … **it lets a
> provider with no `tooled_flags` (e.g. a user-defined provider) decompose a disjoint tree it otherwise could
> not serve.**"

This is a faithful, complete restatement of FR-M13 (§0). **No accuracy defect.**

### 1b. docs/how-it-works.md roles table (line 62) — describes the stager *role*, not an unconditional requirement

```
| **stager** | tooled | Stage one concept's subset of files (`git add`, hunk-level staging) | Mutates the index; exits 0 |
```

This row describes the stager *role's contract* (mode=tooled, job=hunk-level staging). It is **accurate** —
when the stager *does* run (non-disjoint partition), it is tooled and does hunk-level staging. It is NOT an
unconditional "decompose requires a tooled provider" claim, and it is clarified ~80 lines later by the
fast-path note (§1a). **Not a defect — but a reader who stops at the table could misinfer** that a tooled
provider is always required. This is a *discoverability* gap, not an *accuracy* gap (the only candidate for
an optional polish — see §3).

### 1c. README.md — NO unconditional claim (all hits accurate)

README mentions the four-role pipeline (lines 34, 35, 59, 211, 422) and the stager's *constraints*
(line 211: "claude via a staging-only git allowlist…"), but makes **no claim** that decompose requires a
tooled/stager-capable provider. It defers architecture detail to how-it-works.md. **No accuracy defect.**

### 1d. Cross-doc grep — the only "cannot/require tooled" hits live in providers.md (OUT OF SCOPE)

`grep -rniE 'cannot (decompose|serve|stage)|tooled_flags|requires a (tooled|stager)'`:
- `docs/providers.md:31` — "`nil`/empty ⇒ not stager-capable."
- `docs/providers.md:70` — "tooled mode with empty `tooled_flags` errors — that provider cannot serve as a stager."
- `docs/providers.md:107` — "A provider with nil/empty `tooled_flags` **cannot** serve as a stager (render errors
  at invocation time); falls back to the next stager-capable provider."

These describe the **stager role's per-role contract** (still accurate as a role contract — a no-tooled
provider genuinely cannot *serve the stager role*; it just doesn't NEED to on a disjoint partition). The
"render errors at invocation time" phrasing reflects the **pre-BUG-003** eager-error behavior, which
P1.M2.T1.S1/S2 now **defers** to the runLoop (errors only when a stager is genuinely needed for a non-disjoint
partition). **BUT providers.md is explicitly OUT OF SCOPE for this task** (scope = README.md +
docs/how-it-works.md only). Flagged here so the implementer does NOT scope-creep into providers.md; if desired,
a providers.md fix is a SEPARATE task.

---

## §2 — Heading / anchor structure (for any cross-reference)

`docs/how-it-works.md` headings (MkDocs Material anchors):
- `## Multi-commit decomposition` → `#multi-commit-decomposition` (line 47)
- `### The four roles` → `#the-four-roles` (line 57) ← the roles table
- `### Pipeline flow` → `#pipeline-flow` (line 66)
- `### Key design points` → `#key-design-points` (line 136) ← contains the bold **File-disjoint fast-path (/)** paragraph (NOT its own heading → no `#file-disjoint-fast-path` anchor exists)

**Implication**: a forward-reference link must target `#key-design-points` (the section), not a
`#file-disjoint-fast-path` anchor (which does not exist — would be a broken link). Prefer a plain-text
mention ("the *File-disjoint fast-path* note below") or a link to `#key-design-points`.

README already links decompose detail as `[how it works](docs/how-it-works.md#multi-commit-decomposition)`
(line 59) — that anchor is correct and unchanged.

---

## §3 — The ONE candidate edit (OPTIONAL discoverability polish)

**Problem**: a reader who reads only the roles table (line 62, stager=tooled) and the pipeline diagram
(line 112-113, `stager[i] (tooled)`) could conclude "my no-`tooled_flags` provider (opencode / a
user-defined provider) cannot decompose" — because the correction lives ~80 lines later (§1a). The docs are
*accurate* but the correction is not *discoverable* from the table.

**Optional polish** (apply ONLY if it reads cleanly and does not duplicate §1a — a no-op audit-confirmed
outcome is an equally acceptable terminal state per the task contract "If docs already match the spec, no
changes needed"):

Insert ONE short note immediately after the roles table (after line 64, before `### Pipeline flow` at line
66). Exact wording (see PRP §Implementation Tasks):

> **The stager is conditional, not mandatory.** It is the pipeline's only *tooled* role, but it runs only
> when a concept shares a file with another. On a pairwise **file-disjoint** partition — the common case for
> cleanly separated changes — the stager is skipped entirely (each concept is staged deterministically with
> `git add`), so a provider with no `tooled_flags` (e.g. a user-defined provider) can still decompose a
> disjoint tree. See the *File-disjoint fast-path* note under [Key design points](#key-design-points) below.

This is a 4-sentence *pointer/summary*; the full mechanism stays at line 140 (§1a) — not a duplication.
Anchor `#key-design-points` resolves (§2).

**Why optional / bounded**: the task contract explicitly permits no-op when docs already match. The note is
a discoverability aid, not an accuracy fix. The implementer may judge the existing structure (table →
pipeline → key-design-points with the fast-path) sufficient and close the task with a no-op + audit summary.
Either terminal state is defensible.

---

## §4 — Validation (how the implementer confirms the outcome)

1. **Audit greps** (prove no unconditional claim was missed / left behind):
   `grep -rniE 'cannot (decompose|serve|stage)|tooled_flags|requires? a (tooled|stager)|(must|need) be tooled' README.md docs/how-it-works.md`
   → Expected: no hit that asserts decompose *requires* a tooled provider unconditionally. (The roles-table
   "tooled" mode label is a role-contract descriptor, not a requirement claim.) providers.md hits are
   out-of-scope (§1d).
2. **If the optional note is applied** — confirm: (a) Markdown still well-formed (table intact, note renders);
   (b) anchor `#key-design-points` resolves (it's an existing heading, line 136); (c) the note does not
   *contradict* line 140 (it must say "disjoint ⇒ stager skipped, no-tooled provider OK"; shared file ⇒ tooled
   loop — same as line 140); (d) `git diff --name-only` ⊆ {README.md, docs/how-it-works.md}.
3. **No code / build / test impact** — pure Markdown. `make` / `go build` / `go test` are unaffected (a docs
   change cannot break Go). CI docs-build (if any) is the only relevant check; run it if present.

---

## §5 — Scope fences & coordination

- **IN SCOPE**: README.md, docs/how-it-works.md (read/audit; optional ≤1 note in how-it-works.md).
- **OUT OF SCOPE**: `docs/providers.md` (§1d — separate task if the "render errors at invocation time"
  deferral needs documenting); `docs/cli.md`, `docs/configuration.md`; any `.go` code; spec/ (READ-ONLY);
  PRD.md / tasks.json / prd_snapshot.md.
- **No overlap** with the parallel P1.M2.T2.S1 (a code-comment fix in `internal/decompose/decompose.go` —
  different file, different concern). No overlap with P1.M2.T1.S1/S2/S3 (Go code in roles.go/decompose.go).
- This task is **dependent on** the BUG-003 fix (P1.M2.T1) having landed conceptually (the docs describe the
  *behavior* the code now implements); it does NOT edit the code and does not require the code change to be
  merged to edit the docs (the fast-path behavior is already shipped since v3.2 — the docs already match).

---

## §6 — Bottom line

**Docs already match the spec.** The FR-M13 fast-path + its no-tooled-provider consequence are accurately
documented at `docs/how-it-works.md:140`. README is clean. The task's "no changes needed" branch is the
substantively-correct outcome. The only value-add is an *optional* one-paragraph discoverability note after
the roles table (§3). Confidence the docs are accurate: very high (verified against spec §0 + cross-doc grep).