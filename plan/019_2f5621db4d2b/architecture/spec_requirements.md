# Authoritative Spec Text — File-Disjoint Fast-Path (FR-M13/M14)

Verified verbatim from `spec/`. **Location correction:** FR-M13/M14 are DEFINED in
`spec/01-product.md §9.14` (lines 363-364), NOT `spec/03-generation.md`. The latter only
REFERENCES them in the §13.6.3 addendum. §20.2/§20.5 live in `spec/06-reliability.md`.

## FR-M13 (spec/01-product.md:363) — verbatim summary

When the planner's partition is **pairwise file-disjoint** (no path in >1 concept's `files`),
stagecoach stages each concept **deterministically with `git add`, invoking NO stager agent**,
under the unchanged accumulate-never-reset index model: from the baseTree-reset index, for each
concept _i_ run `git add <concept[i].files>` (stages adds, mods, AND deletions for those whole
paths) and freeze `tree[i] = write-tree`. FR-M1c (`verifyFreezeSubset`) runs on every `tree[i]`
unchanged. Paths declared for no concept flow to the arbiter (FR-M3b/FR-M9).

**A path declared for ≥2 concepts disqualifies the fast-path FOR THE WHOLE RUN** — fall back to
the FR-M5 tooled stager for EVERY concept (no per-concept mixing). The gate is automatic +
deterministic (set-membership test over planner `files`); adds no configuration. Because the
fast-path invokes no tooled agent, a `tooled_flags`-less provider (opencode, qwen-code) can
decompose a disjoint tree it otherwise could not.

## FR-M14 (spec/01-product.md:364) — verbatim summary

On the FR-M13 fast-path the deterministic staging sweep freezes every `tree[i]` BEFORE any message
agent starts, so all per-concept diffs `diff(tree[i-1], tree[i])` are available at once. Launch the N
message generations **concurrently** and publish in strict CAS order (FR-M7) as each completes — the
1-deep overlap of FR-M6 is lifted (no serial stager to gate on). Invariants preserved: every message
reasons over tree-to-tree diff (never live index/HEAD); `update-ref`s serialize in order with the
same CAS guard; every `tree[i]` frozen before its message uses it. Critical path collapses from
"planner + ~N sequential steps" to "planner + one message latency + N cheap git add/write-tree ops."
The fast-path is the SOLE route to full message concurrency; the tooled-stager fallback keeps 1-deep.

## §13.6.3 addendum (spec/03-generation.md:120) — "File-disjoint fast-path (FR-M13/M14)"

The stager is the pipeline's only tooled/serial role. When the partition is pairwise file-disjoint,
stage each concept deterministically with `git add` (no stager agent), freeze all `tree[i]` up front,
then run N message generations concurrently + publish in CAS order. `verifyFreezeSubset` (FR-M1c)
still guards every fast-path tree; unclaimed paths still flow to the arbiter (§13.6.5). Any shared
file falls back transparently to the tooled stager for the WHOLE run. The three unchanged invariants
(spec/03-generation.md:114-116):
1. `tree[i]` is frozen before stager[i+1] starts (write-tree is ref/index-read-only).
2. The concept diff is tree-to-tree, never index-vs-HEAD (`message[i]` reasons over `diff tree[i-1] tree[i]`).
3. `update-ref`s serialize — commit[i] parents `newSHA[i-1]`, CAS-moves HEAD only if `HEAD==newSHA[i-1]`.

Index model (spec/03-generation.md:118): **accumulate, never reset** — after stager[i] the index
holds concepts[0..i]; `tree[i]` is that full accumulation. (On the fast-path, the per-concept `git add`
IS the accumulation step; the index is never reset between concepts.)

## Goal G29 (spec/01-product.md:165) — verbatim

Add a file-disjoint staging fast-path: when the planner's partition is pairwise file-disjoint, stage
each concept deterministically with `git add` (bypassing the stager entirely) and run all N message
generations concurrently, publishing in CAS order. FR-M1c guards every tree; any shared-file partition
falls back to the tooled stager. Cuts a disjoint N-concept run's critical path from ~N sequential LLM
steps to one message latency, and lets `tooled_flags`-less providers (opencode, qwen-code) decompose
disjoint trees they otherwise could not.

## §13.6.6 Failure handling within the loop (spec/03-generation.md:145-153)

On the fast-path the first two bullets (stager stages nothing / stager exits non-zero) do not apply,
BUT the deterministic `git add` sweep can still yield `tree[i]==tree[i-1]` → FR-M8 empty-skip must run
per concept. These apply UNCHANGED:
- **message[i] generation fails** → rescue path for concept i only; commits 0..i-1 stand; print frozen
  tree[i] + recovery; (on the fast-path, the other N-1 in-flight messages must be DRAINED to avoid
  goroutine leaks — generalize `drainMsg` from 1 channel to a slice).
- **CAS failure on commit[i]** (HEAD moved externally) → abort with "HEAD moved" message; prior commits
  stand; print in-flight tree[i] recovery command.

## §20.2 / §20.5 testable invariants (spec/06-reliability.md:114-147)

Must preserve: concept isolation (each commit's diff-tree vs parent == exactly that concept's files,
no sibling leakage); T_start completeness; start-of-run freeze (FR-M1b — a file written to the worktree
after T_start freeze appears in NO commit and remains in the worktree); atomic HEAD (after CAS failure
HEAD unchanged); snapshot immutability. §20.5 e2e must-cover: "N unrelated files → N commits" and the
"concurrent file mid-run excluded from every commit, left in worktree" scenario (drive both through the
fast-path now).

## FRs referenced (spec/01-product.md §9.14, lines 344-365)
FR-M1b (start-of-run freeze), FR-M1c (verifyFreezeSubset, hard error on violation), FR-M4 (max_commits
default 12), FR-M5 (tooled stager = fallback), FR-M6 (1-deep overlap — lifted on fast-path), FR-M7
(serialized CAS publication), FR-M8 (empty-skip, no empty commits), FR-M12 (per-concept failure
isolation, partial commits stand), FR-M13 (fast-path gate + deterministic staging), FR-M14 (parallel
message gen). No new config key / CLI flag / FR.