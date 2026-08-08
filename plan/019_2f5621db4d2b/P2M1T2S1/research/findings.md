# Research Findings — P2.M1.T2.S1 (Mode B changeset-level docs sweep)

## §0. What this task IS (and is NOT)

This is a **Mode B changeset-level documentation sweep** (SOW §5: "must run and decide, not skip").
Its job is to sweep the TOP-LEVEL / OVERVIEW docs (README.md + docs/README.md) for any claim the
whole changeset would INVALIDATE, and update it ONLY if a top-level claim is now stale.

It is NOT a Mode A per-file doc edit (those are done: docs/how-it-works.md by P1.M1.T1.S6;
docs/providers.md by P2.M1.T1.S1). This task does NOT re-edit either of those.

**Expected verdict (per the contract + this research): NO edit needed.** Both changeset changes are
invisible/additive at the README/overview granularity. But the sweep MUST run and RECORD the verdict.

## §1. The two changeset changes (the inputs to sweep against)

**(Phase 1) File-disjoint staging fast-path (FR-M13/FR-M14, G29):**
- An **automatic, deterministic optimization** inside multi-commit decomposition: when the planner's
  partition is pairwise file-disjoint, stage each concept with `git add` (no stager agent), run all N
  messages concurrently, publish in CAS order.
- **No CLI flag, no config key, no user-facing surface** (confirmed: `grep -niE "fast.path|disjoint"
  docs/cli.md docs/configuration.md` → ZERO hits). It is INVISIBLE to the user.
- Documented in docs/how-it-works.md (P1.M1.T1.S6, Mode A) — a "File-disjoint fast-path" paragraph.
- P1.M1.T1.S6's PRP explicitly concluded: "NO README change — the fast-path is an invisible, automatic
  optimization with no CLI/config surface."

**(Phase 2) docs/providers.md stager sync (P2.M1.T1.S1):**
- Corrects docs/providers.md's stager-capability column + prose for agy/codex/opencode/cursor (now
  `✓ yes (unscoped)`) to match the committed `internal/provider/builtin.go` TooledFlags.
- A **per-provider CAPABILITY detail** at providers.md's granularity — NOT something the README
  enumerates (the PRD Phase 2 Mode B note: "The README does NOT enumerate per-provider stager status
  at this granularity").
- Documented in docs/providers.md (P2.M1.T1.S1, Mode A).

## §2. The candidate README.md claims to verify (sweep checklist)

README.md is the marketing surface (PRD §21.5). Below: every claim that TOUCHES the changeset's
subject matter (decompose / stager / providers), with the verdict. **All ACCURATE → no edit.**

| Loc | Claim (paraphrased) | Changeset axis | Verdict | Why |
|-----|---------------------|----------------|---------|-----|
| L6 (lede) | "v2.1 adds...; v3.0 adds..." version line | fast-path (v3.2) | ACCURATE — no edit | The fast-path is invisible; it needs no version bump or lede mention. The version line is a user-facing-feature summary; an internal optimization is correctly absent. |
| L32 (comparison table) | "Per-role model routing — Yes (planner/stager/message/arbiter)" | fast-path | ACCURATE — no edit | The 4-role model is unchanged; the fast-path bypasses the stager only for disjoint trees but the conceptual model is still 4-role. |
| L64 (Features table, Multi-commit decomposition) | "Auto-decompose... (planner → stager → message → arbiter)... planner partitions per file and leans toward a soft count target" | fast-path | ACCURATE — no edit | High-level conceptual description; the fast-path is an internal optimization WITHIN this. The stager is still part of the pipeline (used on shared files / fallback). Links to docs/how-it-works.md which now documents both modes. |
| L209 (Multi-commit decomposition section) | "four-role agent pipeline... The stager is constrained to staging operations: claude via a staging-only git allowlist; pi instructionally (its task prompt) plus a HEAD-movement guard..." | fast-path + providers.md stager sync | ACCURATE — no edit | (a) The stager-constraint sentence describes stager BEHAVIOR WHEN IT RUNS (claude scoped, pi unscoped) — correct examples, not an exhaustive list; it does NOT claim the stager always runs, so the fast-path (which bypasses the stager for disjoint trees) does not invalidate it. (b) claude/pi are the TWO correct examples; README intentionally does NOT enumerate all 6 stager-capable providers (PRD Phase 2 Mode B note). NOT a stale claim — a non-exhaustive example. |
| L425 (Which agents supported — verification status) | "pi, agy, codex, opencode, and claude... driven through a real commit-generation run. cursor is NOT yet verified..." | providers.md stager sync | ACCURATE — no edit | This is about END-TO-END VERIFICATION (a commit-generation run), a DIFFERENT axis than STAGER CAPABILITY (tooled_flags). The changeset does not change verification status. (NOTE: this verification-status claim is itself potentially stale vs reality — cursor may now be verified — but that drift is NOT caused by THIS changeset and is out of scope. The sweep concerns only claims THIS changeset would invalidate.) |

**Net: ZERO stale top-level claims in README.md.** Every claim touching the changeset's subject is
either (a) high-level and still correct, (b) a non-exhaustive example by design (README granularity),
or (c) about a different axis (verification vs capability). The fast-path has no user-facing surface
to invalidate; the providers.md sync is below README's granularity.

## §3. docs/README.md (the secondary overview doc) — sweep checklist

docs/README.md is the docs/ index. Its claims touching the changeset:
- Documentation index table: providers.md = "the 7 built-in providers (incl. agy and qwen-code)";
  how-it-works.md = "multi-commit decomposition pipeline". → ACCURATE (generic, no stager status, no
  fast-path mention needed — the index points INTO the detailed docs).
- Capability index: "Chrome-disable (v2.9) → providers.md#tools-disable-asymmetry". → ACCURATE (the
  § DEVIATION in P2.M1.T1.S1 extends the unscoped bullet but does not move/rename that anchor).

**Net: ZERO stale claims in docs/README.md. No edit.**

## §4. Other docs/ files (glance, expected no-op) — NOT overview docs

- `docs/cli.md`, `docs/configuration.md` — REFERENCE docs (flags/config), NOT "overview that summarizes
  capabilities across the whole changeset." `grep` confirms ZERO fast-path surface in either. The
  providers.md stager sync doesn't touch them. Out of the sweep's primary scope; a glance confirms
  nothing. No edit.
- `docs/packaging.md` — distribution doc, not capability overview. Not affected. No edit.
- `docs/ci-validation.md`, `docs/windows-test-support.md` — infra, not capability. No edit.
- `docs/how-it-works.md`, `docs/providers.md` — Mode A handled (Phase 1 / Phase 2). EXCLUDED — do NOT
  re-edit.

## §5. The decision criterion (what counts as "stale top-level claim")

A README/overview claim is **STALE (edit needed)** iff the changeset makes it FACTUALLY WRONG — e.g.:
- A claim that "only pi and claude can serve as the stager" (would be stale after the providers.md sync).
  README does NOT say this.
- A claim that "the stager always runs / decompose is always N sequential steps" (would be stale after
  the fast-path). README does NOT say this.
- A claim about a flag/config the fast-path would add. There is none (fast-path is config-free).

A claim is **NOT stale** if it is a **non-exhaustive example by design** (claude/pi stager examples are
correct, just not complete) or about a **different axis** (verification vs capability). README's
granularity is intentionally high-level (PRD §21.5 / Phase 2 Mode B note).

**CRITICAL ANTI-PATTERN: do NOT invent an edit to "feel productive."** If every candidate claim is
accurate, the correct deliverable is "verified accurate, no edit" — recorded, not forced. The contract
is explicit: "If README is already accurate, record that finding (no edit needed) rather than forcing
a change." This research confirms that is the expected outcome.

## §6. The recording mechanism (the no-edit deliverable)

When the verdict is "no edit," there is no source-file change to commit. The concrete, auditable
artifact is a **verification note** at
`plan/019_2f5621db4d2b/P2M1T2S1/research/sweep_findings.md` recording, per candidate claim, the
verdict + one-line rationale (mirroring §2/§3 above). This proves the sweep RAN and DECIDED (SOW §5),
which is the task's actual purpose. If an edit IS found necessary, the note additionally records the
rationale + the edit made.

## §7. Scope fence — do NOT re-edit Mode A files

- **docs/how-it-works.md** — Phase 1 Mode A (P1.M1.T1.S6). DO NOT re-edit.
- **docs/providers.md** — Phase 2 Mode A (P2.M1.T1.S1). DO NOT re-edit.
- **spec/** — human-owned (AGENTS.md hard rule 1). READ-ONLY.
- **internal/** code, **providers/*.toml**, **tasks.json**, **prd_snapshot.md** — NOT touched.
- The sweep edits ONLY README.md and/or docs/README.md IF (and only if) a stale top-level claim exists;
  otherwise it writes ONLY the sweep_findings.md note.