# Changeset-level docs sweep — verdict (P2.M1.T2.S1)

> **Mode B (SOW §5) changeset-level sweep.** Swept `README.md` and the overview docs
> (`docs/README.md`) for any TOP-LEVEL capability claim the whole changeset would INVALIDATE,
> and updated it ONLY if a claim is now factually wrong. This note records the per-claim verdict
> — it is the auditable proof the sweep RAN and DECIDED (it is NOT a skip).
>
> **Changeset under sweep:** Phase 1 — file-disjoint staging fast-path (FR-M13/FR-M14, G29;
> documented in `docs/how-it-works.md` by P1.M1.T1.S6) + Phase 2 — `docs/providers.md` stager
> sync for agy/codex/opencode/cursor (by P2.M1.T1.S1).

## Decision criterion (STALE vs ACCURATE)

A README/overview claim is **STALE → edit** iff the changeset makes it **FACTUALLY WRONG**. A claim
is **ACCURATE → no edit** if it is a *non-exhaustive example by design* (README granularity is
intentionally high-level, PRD §21.5 / Phase 2 Mode B note) or about a *different axis*
(verification status ≠ stager capability). A claim is also ACCURATE if the changeset is INVISIBLE
to it (the fast-path has no CLI/config surface).

## §1. README.md — candidate-claim checklist

Candidate claims re-derived live (do not trust line numbers blindly — they drift) with:
`grep -niE "stager|tooled|disjoint|fast-path|overlap|decompose|cannot.*(stage|stager)|v2\.|v3\." README.md`
→ 14 hits, all of which fall within the 5 subject-matter claims below or are non-subject hits
(e.g. the comparison-table "Multi-commit decomposition" row at L31, the FAQ "Can it write multiple
commits?" / "Safe to run twice" rows at L399/L415, the v3.0 self-update FAQ at L441). Every
subject-matter hit is accounted for in this table.

| Doc | Loc | Claim (short) | Changeset axis | Verdict | Rationale |
|-----|-----|---------------|----------------|---------|-----------|
| README.md | L6 (lede) | "v2.1 adds…; v3.0 adds `stagecoach upgrade`…" version/feature line | fast-path (v3.2) | **ACCURATE — no edit** | The fast-path is an invisible, automatic, deterministic optimization with NO CLI flag, NO config key, NO user-facing surface. The version line is a user-facing-FEATURE summary; an internal optimization is correctly absent. Needs no version bump or lede mention. |
| README.md | L32 (comparison table) | "Per-role model routing — Yes (planner/stager/message/arbiter)" | fast-path | **ACCURATE — no edit** | The 4-role conceptual model is unchanged. The fast-path bypasses the stager agent ONLY for pairwise file-disjoint partitions; the conceptual model is still four-role (the stager still runs on shared files / fallback). |
| README.md | L64 (Features table) | "Auto-decompose… (planner → stager → message → arbiter)… planner partitions per file and leans toward a soft count target" | fast-path | **ACCURATE — no edit** | High-level conceptual description; the fast-path is an internal optimization WITHIN this pipeline. The row links to `docs/how-it-works.md#multi-commit-decomposition`, which (post-P1.M1.T1.S6) documents both modes. No top-level promise is broken. |
| README.md | L209 (Multi-commit decomposition section) | "four-role agent pipeline… The stager is constrained to staging operations: claude via a staging-only git allowlist (`git add`/`apply`/`status`/`diff`); pi instructionally (its task prompt) plus a HEAD-movement guard…" | fast-path + providers.md stager sync | **ACCURATE — no edit** | (a) The stager-constraint sentence describes stager BEHAVIOR WHEN IT RUNS (claude scoped, pi unscoped) — correct examples, NOT an exhaustive capability list. It does NOT claim the stager always runs, so the fast-path (which bypasses the stager agent for disjoint trees) does not contradict it. (b) claude/pi are TWO correct examples; README intentionally does NOT enumerate all 6 stager-capable providers (PRD Phase 2 Mode B note: "The README does NOT enumerate per-provider stager status at this granularity"). That granularity lives in `docs/providers.md` (Phase 2, done). NOT a stale claim — a non-exhaustive example by design. **Did NOT add agy/codex/opencode/cursor here** (would be scope creep into providers.md's granularity). |
| README.md | L425 (FAQ — Which agents are supported?) | "**pi**, **agy**, **codex**, **opencode**, and **claude** have each been driven through a real commit-generation run. **cursor is NOT yet verified end-to-end**…" | providers.md stager sync | **ACCURATE — no edit** | This claim is about END-TO-END VERIFICATION (a real commit-generation run) — a DIFFERENT axis than STAGER CAPABILITY (`tooled_flags`). The changeset (providers.md stager sync) corrects the *capability* column, not the *verification* status; the changeset does not change which providers are end-to-end verified. Any real-world drift in this verification-status claim is PRE-EXISTING and NOT caused by THIS changeset → out of scope. Did NOT "correct" it. |

**Net for README.md: ZERO stale top-level claims. No edit.**

## §2. docs/README.md — candidate-claim checklist

Read `docs/README.md` in full (66 lines). Subject-matter rows:

| Doc | Loc | Claim (short) | Changeset axis | Verdict | Rationale |
|-----|-----|---------------|----------------|---------|-----------|
| docs/README.md | Documentation index table | "Provider manifests → …the 7 built-in providers (incl. agy and qwen-code)…"; "How Stagecoach works → …multi-commit decomposition pipeline…" | fast-path + providers.md sync | **ACCURATE — no edit** | Generic index pointers INTO the detailed docs. Does not enumerate stager status; does not mention the fast-path. Both the providers.md count ("7 built-in providers") and the how-it-works.md description ("multi-commit decomposition pipeline") remain TRUE after the changeset (the sync corrected per-provider capability rows, not the count; the fast-path is an internal detail of the pipeline). |
| docs/README.md | Capability index | "Chrome-disable (v2.9) → `providers.md#tools-disable-asymmetry`" | providers.md stager sync | **ACCURATE — no edit** | The `#tools-disable-asymmetry` anchor still resolves — confirmed: `docs/providers.md` L90 has the heading `## Tools-disable asymmetry` (GitHub slug `tools-disable-asymmetry`). P2.M1.T1.S1's § DEVIATION extended the *unscoped* bullet under that heading but did NOT rename or move the anchor. Link stays valid. |

**Net for docs/README.md: ZERO stale claims. No edit.**

## §3. Reference-doc glance (expected no-op — NOT overview docs)

`docs/cli.md` and `docs/configuration.md` are REFERENCE docs (flag/config keys), not "overview
that summarizes capabilities across the whole changeset." They are out of the sweep's primary
scope; glanced only to be thorough.

| Doc | Check | Verdict | Rationale |
|-----|-------|---------|-----------|
| docs/cli.md | `grep -niE "fast.path\|disjoint" docs/cli.md` | **ACCURATE — no edit** | ZERO hits. The fast-path has no CLI flag (FR-M13/FR-M14 add no user-facing surface). The providers.md stager sync doesn't touch the CLI reference. Nothing to update. |
| docs/configuration.md | `grep -niE "fast.path\|disjoint" docs/configuration.md` | **ACCURATE — no edit** | ZERO hits. The fast-path adds no config key (config-free optimization). The providers.md stager sync doesn't touch the config reference. Nothing to update. |

(Confirmed live: `grep -niE "fast.path|disjoint" docs/cli.md docs/configuration.md` → exit 1 / no
output.)

## §4. Scope fence — files NOT touched (confirmed)

- `docs/how-it-works.md` — Phase 1 Mode A (P1.M1.T1.S6). **EXCLUDED — not re-edited.**
- `docs/providers.md` — Phase 2 Mode A (P2.M1.T1.S1). **EXCLUDED — not re-edited.**
- `spec/` — human-owned (AGENTS.md hard rule 1). **READ-ONLY.**
- `internal/` code, `providers/*.toml`, `**/tasks.json`, `prd_snapshot.md` — **NOT touched.**

This sweep made NO edit to README.md or docs/README.md (all claims accurate), so the only
working-tree artifact is THIS note.

## SUMMARY — decision of record

**No stale top-level claims; no edit needed.** All 5 README.md candidate claims (L6, L32, L64,
L209, L425) and both docs/README.md rows, plus the docs/cli.md + docs/configuration.md glance,
are ACCURATE against the changeset (fast-path + providers.md stager sync). README.md and
docs/README.md are byte-for-byte unchanged (`git status` clean for both). The Mode A files
(how-it-works.md, providers.md), spec/, code, providers/*.toml, tasks.json, and prd_snapshot.md
are untouched. This recorded verdict IS the deliverable — the sweep ran and decided (SOW §5);
the negative result is the expected, contract-mandated outcome for an invisible/additive changeset.