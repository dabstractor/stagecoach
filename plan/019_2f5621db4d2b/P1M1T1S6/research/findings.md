# Research Findings — P1.M1.T1.S6 (docs/how-it-works.md fast-path paragraph)

> A Mode-A documentation edit (rides with the work). Goal: add ONE "File-disjoint
> fast-path" paragraph to the decompose section so the doc covers BOTH decompose modes.
> Source of truth for the paragraph's content = the SPEC (FR-M13/FR-M14 + §13.6.3 addendum),
> NOT the S5 test code (S5 only verifies the spec'd behavior).

## 1. The file under edit — docs/how-it-works.md

- Path: `docs/how-it-works.md` (41967 bytes, separate from `spec/`). It is the
  "cross-cutting architecture overview" (its own intro line).
- The decompose section is `## Multi-commit decomposition`. Its design paragraphs live under
  `### Key design points`, each a **bold-lead-in** paragraph (no headings, no TOC at file top).
- **Insertion point — the "Overlapped staging and generation" paragraph is line 103:**
  ```
  **Overlapped staging and generation.** `stager[i+1]` runs in parallel with `message[i]` —
  the stager prepares the next concept's index while the message agent generates the current
  commit message. This 1-deep overlap keeps latency low.
  ```
  Line 104 is blank; line 105 begins `**Stage-while-editing (FR-E2).** …`.
  → The new paragraph goes BETWEEN line 103 and the "Stage-while-editing" paragraph (i.e. it
  directly contrasts the 1-deep overlap it lifts). This is the item's preferred placement
  ("adjacent to the 'Overlapped staging and generation' paragraph (~line 103)").
- **Line 123 — the "Output quality is bounded by stager-model discipline" paragraph** ends:
  "…For clean, reviewable multi-commit history, prefer a strong stager model, or use `--single` /
  pre-stage each concept manually. The disjoint-files common case (the planner emits exact
  per-concept `files` lists) is the most robust against this."
  → This is the item's "line 123 already notes … most robust." It COMPLEMENTS the new paragraph
  (fast-path explains WHY disjoint is robust+fast). Do NOT reflow it (item: "Surgical edit").
- Doc style: **bold lead-in** + prose; `code spans` for git cmds / tree vars; em-dashes; FR-Mxx
  tags used throughout (FR-M1c, FR-M2b, FR-M8, FR-M12, FR-E2 …); `verifyFreezeSubset` name AND
  FR-M1c tag BOTH appear (e.g. "Freeze enforcement" para + the Safety bullet line 132).

## 2. Canonical spec text (verified verbatim against spec/)

### FR-M13 — spec/01-product.md:363 (the fast-path gate + deterministic staging)
When the planner's partition is **pairwise file-disjoint** (no path in >1 concept's `files`),
stagecoach stages each concept **deterministically with `git add`, invoking NO stager agent**,
under the unchanged accumulate-never-reset index model: `git add <concept[i].files>` stages
adds/mods/**and deletions** for those whole paths, then `tree[i] = write-tree`. Whole-file
staging realizes the partition exactly (no hunk split needed → no agent judgment). FR-M1c
(`verifyFreezeSubset`) runs on every `tree[i]` unchanged (whole-path adds of `T_start` content
pass trivially). Unclaimed paths flow to the arbiter (FR-M3b/FR-M9). **A path in ≥2 concepts
disqualifies the fast-path FOR THE WHOLE RUN** → fall back to the FR-M5 tooled stager for EVERY
concept (no per-concept mixing). Gate is automatic + deterministic (set-membership over `files`);
adds NO config. A `tooled_flags`-less provider (FR-D4 — opencode, qwen-code) can decompose a
disjoint tree it otherwise could not serve as stager.

### FR-M14 — spec/01-product.md:364 (parallel message gen + CAS publish)
The deterministic sweep freezes every `tree[i]` **before any message agent starts** → all
per-concept diffs available at once. Launch the N message generations **concurrently**, publish
in strict CAS order (FR-M7) as each completes. The FR-M6 1-deep overlap ceiling is LIFTED (no
serial stager to gate on). §13.6.3 invariants preserved: every message reasons over tree-to-tree
diff (never live index/HEAD); `update-ref`s serialize in order with the same CAS guard; every
`tree[i]` frozen before its message uses it. Critical path collapses from
"planner + ~N sequential (stager ∥ message) steps" to "planner + one message latency + N cheap
`git add`/`write-tree` ops." The fast-path is the SOLE route to full message concurrency; the
tooled-stager fallback (FR-M5/FR-M6) retains 1-deep overlap (its staging is inherently serial).

### §13.6.3 addendum — spec/03-generation.md:120 (the model prose the doc should mirror)
"The stager is the pipeline's only tooled role and its only inherently serial step … But the
stager exists solely to hunk-split a file shared across concepts; the planner already declares a
file-level partition (`files` per concept). When that partition is pairwise file-disjoint …
stagecoach stages each concept deterministically with `git add` … invoking no stager agent at
all. With every `tree[i]` frozen before any message starts, the N message generations then run
concurrently and publish in CAS order … `verifyFreezeSubset` (FR-M1c) still guards every
fast-path tree; unclaimed paths still flow to the arbiter (§13.6.5). Any shared file falls back
transparently to the tooled stager for the whole run."

### G29 — spec/01-product.md:165 (the goal sentence; the "what + why")
Add a file-disjoint staging fast-path: pairwise file-disjoint → `git add` each concept
(bypassing stager) → N message gens concurrent → CAS order. FR-M1c guards every tree; shared-file
→ tooled stager fallback. Cuts critical path ~N sequential LLM steps → one message latency; lets
`tooled_flags`-less providers (opencode, qwen-code) decompose disjoint trees.

## 3. Required content points (from item description + spec) — the paragraph MUST state
1. When the planner's partition is **pairwise file-disjoint** → no stager, no 1-deep overlap.
2. Stagecoach stages each concept **deterministically with `git add`** (adds/mods/deletions for
   whole paths), under the unchanged accumulate-never-reset index model.
3. N message generations run **concurrently** and publish **in CAS order**.
4. Critical path collapses to **"planner + one message latency"**.
5. Any **shared file falls back transparently to the tooled-stager loop above**.
6. Accuracy to FR-M13/FR-M14: **`verifyFreezeSubset` (FR-M1c) still guards every tree**;
   **unclaimed paths still flow to the arbiter**; tree-to-tree-diff + serialized-CAS invariants
   unchanged.
7. (Optional, ties to line 123 + G29 side effect) the disjoint case is the most robust AND
   fastest; `tooled_flags`-less providers (opencode/qwen-code) can decompose disjoint trees.

## 4. Scope fences (confirmed)
- **EDIT ONLY**: `docs/how-it-works.md`. Surgical — one new paragraph; do NOT reflow the section.
- **NO README change** — confirmed: README.md mentions "decompose"/"stager" (planner → stager →
  message → arbiter) but NEVER overlap / 1-deep / fast-path / disjoint. The fast-path is
  invisible/additive at the user surface (no CLI flag, no config key), so README is accurate as-is.
- **NO cli.md / configuration.md change** — the fast-path is config-free/automatic (FR-M13:
  "adds no configuration"). No flag, no key.
- **NO providers.md change** — that is Phase 2 (P2.M1.T1.S1, the agy/codex/opencode stager sync).
  OUT of scope here. (Note: G29's "tooled_flags-less providers" side effect is worth ONE clause in
  the new paragraph, but providers.md's stager table is a separate, deferred task.)
- **NO spec/ change** — spec/SPEC.md (and spec/*.md) is human-owned, read-only for agents.
- **`spec_requirements.md`** (the plan's research extraction) is already verbatim-accurate; use it
  as a cross-check, but the SPEC (spec/01-product.md §9.14) is the authority.

## 5. Validation surface
- `.markdownlint.json` = `{ default:true, MD013:false, MD033:false, MD060:false }` → MD013
  (line-length) is OFF (matches the existing doc's long lines); other rules ON. **markdownlint is
  NOT run in `.github/workflows/ci.yml`** (grep: only the docs/ci-validation.md comment matches).
  So markdown conformance is a LOCAL nicety, not a CI gate. If `markdownlint-cli2`/`npx
  markdownlint-cli` is available, run it as an optional check; the real gates are the surgical
  `git diff` and the accuracy cross-check.
- The doc has no TOC to update (file starts at `# How Stagecoach works` + intro paragraph; the
  decompose subsection is plain prose). Adding a bold-lead-in paragraph needs no TOC/index edit.
- This is a docs-only change → `make test`/`make lint` (golangci-lint, Go tests) are UNAFFECTED
  and need not be gate-relevant (note this so the implementer doesn't waste a cycle re-running
  the whole Go suite for a prose edit — though running `make` gates is harmless).

## 6. Relationship to sibling PRPs (parallel context)
- S1 (isFileDisjoint gate), S2 (deterministic sweep + FR-M1c + FR-M8 skip), S3 (concurrent
  messages + CAS publish + FR-M12 isolation), S4 (Decompose dispatch), S5 (regression suite) all
  implement/verify the BEHAVIOR this paragraph DOCUMENTS. S6's source of truth is the SPEC
  (FR-M13/FR-M14 + §13.6.3), which S1–S5 conform to. No dependency on S5's test code; S6 can be
  written in parallel and ships with the changeset (Mode A: rides with the work).