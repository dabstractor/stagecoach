name: "P2.M1.T2.S1 — Mode B changeset-level docs sweep: README.md + overview docs; update only stale top-level claims"
description: >
  A Mode B (SOW §5) changeset-level documentation SWEEP — NOT a guaranteed edit. Sweep README.md and the
  overview docs (docs/README.md) for any top-level capability claim the WHOLE changeset would INVALIDATE,
  and update it ONLY if a top-level claim is now stale. The changeset is Phase 1 (file-disjoint staging
  fast-path, FR-M13/FR-M14, G29 — invisible/automatic, NO CLI/config surface, documented in
  docs/how-it-works.md by P1.M1.T1.S6) + Phase 2 (docs/providers.md stager sync for agy/codex/opencode/cursor,
  by P2.M1.T1.S1). RESEARCH VERDICT (see findings.md §2/§3): ZERO stale top-level claims — the fast-path
  has no user-facing surface to invalidate, and the providers.md stager sync is BELOW README's granularity
  (the PRD's own Phase 2 Mode B note: "The README does NOT enumerate per-provider stager status at this
  granularity"). So the EXPECTED deliverable is a RECORDED VERIFICATION (no edit), NOT a forced change —
  the contract is explicit: "If README is already accurate, record that finding (no edit needed) rather than
  forcing a change." But the sweep MUST RUN and DECIDE, not skip. Concretely: (1) sweep README.md against
  the candidate-claim checklist (the 5 claims touching decompose/stager/providers); (2) sweep docs/README.md;
  (3) glance at docs/cli.md + docs/configuration.md (reference docs, expected no-op — the fast-path has no
  surface there); (4) make a surgical edit to README.md/docs/README.md ONLY if a claim is factually WRONG
  (not merely non-exhaustive); (5) RECORD the per-claim verdict in
  plan/019_2f5621db4d2b/P2M1T2S1/research/sweep_findings.md (the no-edit artifact). DO NOT re-edit the Mode
  A files (docs/how-it-works.md, docs/providers.md). DO NOT touch spec/, code, providers/*.toml, tasks.json,
  prd_snapshot.md. NO `go build`/`go test` — docs-only.

---

## Goal

**Feature Goal**: Confirm (or surgically correct) that the TOP-LEVEL / OVERVIEW docs — README.md and
docs/README.md — are in sync with the whole changeset (the file-disjoint fast-path + the providers.md
stager sync). The sweep must RUN and reach a recorded DECISION per SOW §5 ("must run and decide, not
skip"), even though the expected decision is "already accurate, no edit."

**Deliverable**: ONE of two outcomes —
  (a) **No edit (expected)**: a verification note at
      `plan/019_2f5621db4d2b/P2M1T2S1/research/sweep_findings.md` recording, per candidate claim, the
      verdict + rationale, and a CLEAN `git status` (no doc changed). This note IS the deliverable — it
      proves the sweep ran.
  (b) **Surgical edit (only if a stale top-level claim exists)**: a minimal edit to README.md and/or
      docs/README.md correcting ONLY the factually-wrong claim, PLUS the sweep_findings.md note recording
      what changed and why.

**Success Definition**:
- The sweep RAN: every candidate claim in the checklist (§"What" / findings §2-§3) was checked against
  the changeset and given a verdict in sweep_findings.md.
- The decision is DEFENSIBLE: each "no edit" verdict cites why the changeset does not invalidate the
  claim (invisible surface / non-exhaustive example / different axis); each edit (if any) cites the
  specific factual wrongness.
- NO forced change: if all claims are accurate, README.md/docs/README.md are byte-for-byte unchanged
  (`git status` clean for those files).
- Mode A files untouched: docs/how-it-works.md and docs/providers.md are NOT re-edited.
- Scope clean: no spec/, code, providers/*.toml, tasks.json, prd_snapshot.md touched.

## User Persona (if applicable)

**Target User**: A reader evaluating Stagecoach from the README (the marketing surface, PRD §21.5) who
needs the top-level capability claims to remain TRUE after the fast-path + providers.md changes land.
**Use Case**: A new user reads README's "Multi-commit decomposition" + "Which agents are supported?"
sections to decide whether to install; the claims there must not contradict the shipped binary.
**Pain Points Addressed**: A stale top-level claim (e.g., "only pi/claude can stage", or "decompose is
always N sequential steps") would mislead a reader. The sweep guarantees none exists after the changeset.

## Why

- **SOW §5 compliance (the real purpose)**: a changeset that adds a feature + syncs docs MUST close with a
  top-level sweep that confirms the overview docs still tell the truth — even when (as here) the features
  are invisible/additive. Skipping the sweep because "it's probably fine" is the exact failure SOW §5
  forbids. The sweep's value is the NEGATIVE result, recorded.
- **Defend the README's high-level contract**: README is the marketing surface; its claims are the first
  thing a user reads. The changeset added an internal optimization (fast-path) and a per-provider detail
  (stager sync) — the sweep confirms neither broke a top-level promise.
- **Avoid the two failure modes**: (1) shipping a stale top-level claim (missed), and (2) "feeling
  productive" by inventing an edit where none is needed (scope creep / churn). The sweep's decision
  criterion (§"What") rules out both.

## What

A documentation SWEEP + DECISION. Two phases:

**Phase A — sweep the candidate claims** (the exhaustive list of README/overview claims that TOUCH the
changeset's subject matter — decompose / stager / providers). For each, decide STALE (edit) vs ACCURATE
(no edit) using the criterion below. Record every verdict.

**Phase B — act on the verdict**: if any claim is STALE, make the minimal surgical edit; if all ACCURATE,
make no doc change. Always write the sweep_findings.md note.

### The candidate-claim checklist (README.md)

These are the ONLY README.md claims touching the changeset (found via
`grep -niE "stager|tooled|disjoint|fast-path|overlap|decompose|cannot.*stage|v2\.|v3\." README.md`):

| Loc | Claim | Changeset axis | Expected verdict |
|-----|-------|----------------|------------------|
| L6 (lede) | "v2.1 adds...; v3.0 adds..." version line | fast-path (v3.2) | ACCURATE — fast-path is invisible; needs no version/lede mention |
| L32 (comparison table) | "Per-role model routing — Yes (planner/stager/message/arbiter)" | fast-path | ACCURATE — 4-role model unchanged |
| L64 (Features table) | "Auto-decompose... (planner → stager → message → arbiter)... partitions per file, soft target" | fast-path | ACCURATE — high-level; fast-path is internal; links to docs/how-it-works.md (now documents both modes) |
| L209 (decompose section) | "...four-role agent pipeline... The stager is constrained to staging operations: claude via a staging-only git allowlist...; pi instructionally (its task prompt) plus a HEAD-movement guard..." | fast-path + providers.md stager sync | ACCURATE — (a) describes stager BEHAVIOR WHEN IT RUNS (claude/pi are correct examples, non-exhaustive by design); (b) README intentionally does NOT enumerate all 6 stager-capable providers (Phase 2 Mode B note); (c) does NOT claim the stager always runs, so the fast-path bypass doesn't invalidate it |
| L425 (Which agents supported) | "pi, agy, codex, opencode, and claude... driven through a real commit-generation run. cursor is NOT yet verified..." | providers.md stager sync | ACCURATE — about END-TO-END VERIFICATION, a DIFFERENT axis than STAGER CAPABILITY (tooled_flags). The changeset does not change verification status. (Any real-world drift in this claim is NOT from THIS changeset → out of scope.) |

### The candidate-claim checklist (docs/README.md)

| Loc | Claim | Expected verdict |
|-----|-------|------------------|
| Documentation index table | providers.md = "the 7 built-in providers (incl. agy and qwen-code)"; how-it-works.md = "multi-commit decomposition pipeline" | ACCURATE — generic index; no stager status; no fast-path mention needed |
| Capability index | "Chrome-disable (v2.9) → providers.md#tools-disable-asymmetry" | ACCURATE — the § DEVIATION in P2.M1.T1.S1 extends the unscoped bullet but does not move/rename that anchor |

### Glance (reference docs, expected no-op — NOT overview docs)

- `docs/cli.md`, `docs/configuration.md` — reference docs, not "overview that summarizes capabilities."
  `grep -niE "fast.path|disjoint" docs/cli.md docs/configuration.md` → ZERO hits (confirms the fast-path
  has no surface there). The providers.md stager sync doesn't touch them. Expected: no edit. (Checked
  only to be thorough; if a stale claim IS found, edit surgically.)

### Decision criterion (STALE vs ACCURATE)

A README/overview claim is **STALE → edit** iff the changeset makes it **FACTUALLY WRONG**:
- e.g. "only pi and claude can serve as the stager" (would be stale after the providers.md sync) — README does NOT say this.
- e.g. "the stager always runs / decompose is always ~N sequential steps" (would be stale after the fast-path) — README does NOT say this.
- e.g. a claim about a fast-path flag/config — there is none (fast-path is config-free).

A claim is **ACCURATE → no edit** if it is a **non-exhaustive example by design** (claude/pi stager
examples are correct, just incomplete — README granularity is intentionally high-level, PRD §21.5 /
Phase 2 Mode B note) or about a **different axis** (verification status ≠ stager capability).

### Success Criteria
- [ ] sweep_findings.md exists at `plan/019_2f5621db4d2b/P2M1T2S1/research/sweep_findings.md` and records
      a verdict + one-line rationale for EVERY candidate claim in the checklists above (README L6/L32/L64/
      L209/L425 + docs/README.md index rows + the cli.md/configuration.md glance).
- [ ] Each verdict cites the changeset axis it was checked against (fast-path / providers.md sync / both).
- [ ] IF (and only if) a claim is factually wrong: a minimal surgical edit to README.md and/or
      docs/README.md is made, and sweep_findings.md records what changed + why. If all accurate: those
      files are byte-for-byte unchanged (`git diff --stat` shows nothing for them).
- [ ] docs/how-it-works.md and docs/providers.md are NOT re-edited (Mode A files — excluded).
- [ ] No spec/, code, providers/*.toml, tasks.json, prd_snapshot.md touched.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the complete candidate-claim checklist (with line numbers + expected verdicts), the exact
decision criterion (stale-vs-accurate with worked examples), the changeset's two changes and WHY each is
invisible/below-granularity at the README level, the recording mechanism (the sweep_findings.md note path
+ what it must contain), the grep commands to re-derive the candidate claims, and the scope fences. The
implementer needs no judgment beyond applying the stated criterion to the stated checklist.

### Documentation & References

```yaml
# MUST READ — the sweep analysis (the candidate-claim checklist + verdicts + decision criterion).
- docfile: plan/019_2f5621db4d2b/P2M1T2S1/research/findings.md
  why: "§0 what this task IS (Mode B sweep-and-decide, NOT a guaranteed edit); §1 the two changeset changes
        and WHY each is invisible/below-README-granularity; §2 the README candidate-claim checklist with
        per-claim verdicts; §3 the docs/README.md checklist; §4 the reference-doc glance; §5 the STALE-vs-
        ACCURATE decision criterion with worked examples; §6 the recording mechanism; §7 the scope fence."

# MUST READ — the files under sweep.
- file: README.md
  why: "THE primary sweep target (the marketing surface, PRD §21.5). The 5 candidate claims are at L6
        (lede/version), L32 (comparison table — per-role routing), L64 (Features table — Multi-commit
        decomposition), L209 (Multi-commit decomposition section — the stager-constraint sentence is the
        one most likely to look stale but is actually a correct non-exhaustive example), L425 (Which agents
        supported — verification status). Re-derive the list with the grep in §What."
  pattern: "README is high-level by design (PRD §21.5); it intentionally does NOT enumerate per-provider
            stager status. A non-exhaustive example (claude/pi stager) is NOT a stale claim."
  gotcha: "L209's 'The stager is constrained to: claude scoped; pi unscoped + HEAD-guard' looks like it
           might need updating after the providers.md sync — it does NOT. It describes stager BEHAVIOR WHEN
           IT RUNS (correct examples), not an exhaustive capability list, and the fast-path (which bypasses
           the stager for disjoint trees) doesn't contradict it (it doesn't claim the stager always runs).
           Do NOT add agy/codex/opencode/cursor here — that's providers.md's granularity (Phase 2 Mode B note)."

- file: docs/README.md
  why: "THE secondary sweep target (the docs/ index). Its index table + capability index are generic
        pointers; none enumerate stager status or the fast-path. Expected: no edit."
  gotcha: "The capability-index link 'providers.md#tools-disable-asymmetry' stays valid — P2.M1.T1.S1's
           § DEVIATION extends the unscoped bullet under that heading but does not rename/move the anchor."

# CONTEXT — the changeset's Mode A doc edits (READ-ONLY — do NOT re-edit; confirms what's already covered).
- file: docs/how-it-works.md
  why: "Phase 1 Mode A output (P1.M1.T1.S6) — added the 'File-disjoint fast-path (FR-M13/FR-M14).' paragraph.
        This is the detailed decompose doc; README links into it. DO NOT re-edit (Mode A done)."
- file: docs/providers.md
  why: "Phase 2 Mode A output (P2.M1.T1.S1) — fixed the agy/codex/opencode/cursor stager rows + prose +
        stager-scope bullet + footer. This is the per-provider capability doc. DO NOT re-edit (Mode A done)."

# CONTEXT — the changeset's PRPs (the CONTRACT for what each Mode A task produced).
- docfile: plan/019_2f5621db4d2b/P1M1T1S6/PRP.md
  why: "Phase 1 docs PRP. Its scope-fence explicitly says 'NO README change — the fast-path is an
        invisible, automatic optimization with no CLI/config surface.' This sweep CONFIRMS that decision
        still holds at the changeset level."
- docfile: plan/019_2f5621db4d2b/P2M1T1S1/PRP.md
  why: "Phase 2 docs PRP (docs/providers.md stager sync). Establishes the TRUE stager set (6 capable:
        pi/claude/agy/opencode/codex/cursor; 1 not: qwen-code). Confirms README does NOT need to enumerate
        this — it's providers.md's granularity."

# CONTEXT — the architecture findings (the divergences + the PRD's own granularity note).
- docfile: plan/019_2f5621db4d2b/architecture/findings_and_divergences.md
  why: "D1 (opencode IS stager-capable — resolved in Phase 2), D2 (role_defaults.go drift — flagged, not
        fixed), D4 (spec §12.7 codex stale — flagged, not fixed). Confirms the changeset is docs-only at
        the provider level; no README-level claim is affected."

# CONTEXT — the spec (READ-ONLY; the changeset's FRs + README structure).
- url: spec/SPEC.md (§21.5 README structure; §9.14 FR-M13/FR-M14 fast-path; §9.15 per-role)
  why: "§21.5 defines README as the high-level marketing surface (hero → demo → install → quick start →
        snapshot workflow → full reference link → FAQ). §9.14 FR-M13/FR-M14 define the fast-path. Confirms
        the fast-path belongs in the detailed docs, NOT the README lede."
  gotcha: "spec/ is human-owned (AGENTS.md hard rule 1) — READ-ONLY; never edit in this task."

# CONTEXT — markdownlint config (NOT a CI gate).
- file: .markdownlint.json
  why: "{ default:true, MD013:false, MD033:false, MD060:false }. If an edit IS made, match the existing
        long-line style (MD013 OFF). markdownlint is NOT run in CI."
```

### Current Codebase tree (the sweep surface)

```bash
# SWEEP TARGETS (overview docs):
README.md                 # PRIMARY sweep target (5 candidate claims)
docs/README.md            # SECONDARY sweep target (index rows + capability index)
# GLANCE (reference docs, expected no-op):
docs/cli.md               # flag reference — fast-path has no surface (grep confirms zero hits)
docs/configuration.md     # config reference — fast-path adds no key (grep confirms zero hits)
# EXCLUDED (Mode A files — do NOT re-edit):
docs/how-it-works.md      # Phase 1 Mode A (P1.M1.T1.S6) — fast-path paragraph already added
docs/providers.md         # Phase 2 Mode A (P2.M1.T1.S1) — stager sync already done
# OUT OF SCOPE (not capability overview / not docs):
docs/packaging.md, docs/ci-validation.md, docs/windows-test-support.md   # not capability overview
spec/                     # human-owned, READ-ONLY (AGENTS.md rule 1)
internal/, providers/*.toml, plan/**/tasks.json, prd_snapshot.md         # NOT touched
```

### Desired Codebase tree with files to be added/edited

```bash
plan/019_2f5621db4d2b/P2M1T2S1/research/sweep_findings.md   # NEW (always) — the per-claim verdict note
README.md          # EDIT ONLY IF a stale top-level claim is found (expected: NO edit)
docs/README.md     # EDIT ONLY IF a stale top-level claim is found (expected: NO edit)
# NOTHING ELSE. No Mode A files, no spec/, no code, no tasks.json.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (this is a SWEEP-AND-DECIDE task, NOT a guaranteed edit). The contract's own RESEARCH NOTE
     says the sweep is "EXPECTED to conclude README needs no (or minimal) change" but "must run and decide,
     not skip." Forcing an edit when the claims are accurate is the #1 failure mode. If every candidate
     claim is accurate, the deliverable is the sweep_findings.md NOTE + a clean git status — that is a
     SUCCESS, not a no-op to apologize for. -->

<!-- CRITICAL (the L209 stager sentence is NOT stale — do not "fix" it). "The stager is constrained to
     staging operations: claude via a staging-only git allowlist...; pi instructionally..." reads like it
     might need agy/codex/opencode/cursor added after the providers.md sync. It does NOT: (a) it describes
     stager BEHAVIOR WHEN IT RUNS (claude/pi are correct examples), (b) README intentionally does not
     enumerate all 6 stager-capable providers (PRD Phase 2 Mode B note: "README does NOT enumerate
     per-provider stager status at this granularity"), (c) the fast-path's stager bypass doesn't contradict
     it (it never claims the stager always runs). Adding the 4 providers here would be scope creep into
     providers.md's granularity. LEAVE IT. -->

<!-- CRITICAL (the L425 verification-status claim is a DIFFERENT axis). "cursor is NOT yet verified
     end-to-end" is about a real commit-generation run, NOT stager capability (tooled_flags). The changeset
     does not change verification status. (If that claim is stale vs reality, that's a PRE-EXISTING drift
     unrelated to THIS changeset — out of scope. The sweep concerns only claims THIS changeset would
     invalidate.) Do NOT "correct" it here. -->

<!-- GOTCHA (the fast-path is INVISIBLE — it cannot invalidate any README claim). It has no flag, no config
     key, no user-facing surface (grep -niE 'fast.path|disjoint' docs/cli.md docs/configuration.md → 0).
     So no README "features" / "install" / "capability overview" claim can be made wrong by it. The only
     doc that needed to mention it was docs/how-it-works.md (Phase 1 Mode A, done). -->

<!-- GOTCHA (a non-exhaustive example is NOT a stale claim). README's high-level examples (claude/pi for
     the stager) are correct-by-design incompleteness, not factual errors. The decision criterion (§What)
     requires the changeset to make the claim FACTUALLY WRONG to warrant an edit. -->

<!-- GOTCHA (Mode A files are EXCLUDED). docs/how-it-works.md (Phase 1) and docs/providers.md (Phase 2)
     were ALREADY edited by their own Mode A subtasks. Re-editing them here would duplicate/conflict with
     that work (and risk undoing the § DEVIATION in P2.M1.T1.S1). Sweep README + docs/README.md only. -->

<!-- GOTCHA (MD013 OFF). If an edit IS made, match the existing long-line style; do NOT hard-wrap to 80. -->
```

## Implementation Blueprint

### Data models and structure
None (documentation sweep only). No code, config, schema, or test change.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: SWEEP README.md against the candidate-claim checklist (the decision)
  - RE-DERIVE the candidate claims (do not trust line numbers blindly — they drift): run
        grep -niE "stager|tooled|disjoint|fast-path|overlap|decompose|cannot.*(stage|stager)|v2\.|v3\." README.md
    and confirm the 5 claims in §"What" (L6 lede, L32 comparison table, L64 Features table, L209 decompose
    section, L425 Which-agents-supported) are the complete set touching the changeset's subject matter.
  - FOR EACH claim, apply the decision criterion (§"What"): is it FACTUALLY WRONG after the changeset
    (fast-path + providers.md stager sync)? Expected verdicts (from findings §2): all ACCURATE.
  - RECORD each verdict (claim loc + verdict + one-line rationale + changeset axis) for Task 4.
  - SPECIAL-CHECK L209 (the stager-constraint sentence): confirm it describes stager BEHAVIOR WHEN IT
    RUNS (claude/pi correct examples), NOT an exhaustive capability list, and does NOT claim the stager
    always runs. → expected ACCURATE (see Known Gotchas). Do NOT add agy/codex/opencode/cursor.
  - SPECIAL-CHECK L425 (verification status): confirm it's a DIFFERENT axis (verification ≠ capability).
    → expected ACCURATE. Do NOT "correct" pre-existing verification drift here (out of scope).

Task 2: SWEEP docs/README.md (the secondary overview doc)
  - Read docs/README.md. Check the Documentation index table (providers.md / how-it-works.md rows) and
    the Capability index (Chrome-disable link) for any claim the changeset invalidates. Expected: none.
  - CONFIRM the 'providers.md#tools-disable-asymmetry' anchor still resolves (P2.M1.T1.S1's § DEVIATION
    extended the unscoped bullet under that heading but did not rename/move the anchor). → expected valid.
  - RECORD the verdict(s) for Task 4.

Task 3: GLANCE docs/cli.md + docs/configuration.md (reference docs, expected no-op)
  - These are REFERENCE docs, not overview docs — out of the sweep's primary scope. Glance only to be
    thorough. Run grep -niE "fast.path|disjoint" docs/cli.md docs/configuration.md → expected ZERO hits
    (confirms the fast-path has no surface). The providers.md stager sync doesn't touch them.
  - IF (unlikely) a stale claim is found, edit surgically; else no edit. RECORD the glance verdict.

Task 4: WRITE the sweep_findings.md note (the always-delivered artifact)
  - FILE: plan/019_2f5621db4d2b/P2M1T2S1/research/sweep_findings.md
  - STRUCTURE: a short header ("Changeset-level docs sweep — verdict") + a table or list with one row per
    candidate claim: { doc, loc, claim (short), changeset axis, verdict (STALE/ACCURATE), rationale
    (one line) }. Mirror findings.md §2/§3 as the EXPECTED baseline; update if the live sweep diverged.
  - CLOSE with a one-line SUMMARY verdict: "No stale top-level claims; no edit needed" (expected) OR
    "N stale claim(s) corrected: <list>" (if edits made). This summary is the task's decision of record.
  - This note is what makes the sweep AUDITABLE (proves it ran + decided, per SOW §5).

Task 5: ACT on the verdict (edit ONLY if a claim is STALE)
  - IF Task 1-3 found a factually-wrong claim (unexpected per findings §2): make the MINIMAL surgical
    edit to README.md and/or docs/README.md correcting ONLY that claim. Match the existing voice + long-
    line style (MD013 OFF). Record the edit in sweep_findings.md (Task 4).
  - IF all claims are ACCURATE (expected): make NO doc change. `git status` stays clean for README.md /
    docs/README.md. The sweep_findings.md note is the deliverable.
  - DO NOT invent an edit to "feel productive." A clean no-edit verdict is the correct, contract-mandated
    outcome when the claims are accurate.

Task 6: VERIFY (see Validation Loop) — scope guard + the no-edit gate.
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN (the sweep verdict record — the note's per-claim row). One row per candidate claim:
     | README.md | L209 | "stager constrained: claude scoped; pi unscoped+guard" | fast-path + providers.md sync | ACCURATE | correct non-exhaustive example; README granularity (Phase 2 Mode B note); fast-path bypass doesn't contradict (no "always runs" claim) | -->

<!-- PATTERN (applying the criterion). Ask, per claim: "Did the fast-path or the providers.md stager sync
     make this sentence FACTUALLY WRONG?" If yes → STALE (edit). If the sentence is merely incomplete
     (non-exhaustive example) or about a different axis (verification vs capability) → ACCURATE (no edit). -->

<!-- PATTERN (the no-edit success). git status shows ONLY plan/.../research/sweep_findings.md (untracked
     or the note); README.md + docs/README.md are UNMODIFIED. That clean state, plus the note, IS the
     completed deliverable — not a sign the task was skipped. -->
```

### Integration Points

```yaml
NO code / config / schema / CLI / test integration. Docs sweep only.

DOCS (conditional edit — ONLY if a stale claim exists):
  - README.md            — edit ONLY a factually-wrong top-level claim (expected: none).
  - docs/README.md       — edit ONLY a factually-wrong index/capability claim (expected: none).

ARTIFACT (always written):
  - plan/019_2f5621db4d2b/P2M1T2S1/research/sweep_findings.md — the per-claim verdict note.

EXCLUDED (Mode A — do NOT re-edit):
  - docs/how-it-works.md (Phase 1, P1.M1.T1.S6) + docs/providers.md (Phase 2, P2.M1.T1.S1).

READ-ONLY:
  - spec/ (AGENTS.md hard rule 1) + internal/ code + providers/*.toml + tasks.json + prd_snapshot.md.
```

## Validation Loop

> **Docs-only sweep.** No `go build`/`go test`/`make` — no Go code is touched. The gates are: the sweep
> ran (the note exists + is complete), the no-edit gate (README/docs-README unchanged IF all accurate),
> and the scope guard (no Mode A / spec / code touched).

### Level 1: The sweep ran (the decision of record exists)

```bash
# The sweep_findings.md note exists and records a verdict for every candidate claim.
test -f plan/019_2f5621db4d2b/P2M1T2S1/research/sweep_findings.md && echo "note exists" || echo "MISSING note"
# Expected: "note exists".

# It records the expected 5 README claims + docs/README.md rows + the cli/configuration glance.
grep -ciE "README.md|docs/README.md|cli.md|configuration.md" plan/019_2f5621db4d2b/P2M1T2S1/research/sweep_findings.md
# Expected: ≥5 (each sweep target mentioned). The note's SUMMARY line states the verdict.

# It states a clear decision (one of):
grep -iE "no stale top-level claims|no edit needed|stale claim.*corrected" plan/019_2f5621db4d2b/P2M1T2S1/research/sweep_findings.md
# Expected: ≥1 match (the SUMMARY verdict line).
```

### Level 2: The no-edit gate (IF the verdict is "all accurate" — the expected case)

```bash
# README.md and docs/README.md are byte-for-byte UNCHANGED when all claims are accurate.
git diff --stat README.md docs/README.md
# Expected (no-edit verdict): EMPTY output (nothing changed). If this shows changes, EITHER an edit was
# made (fine, IF a claim was stale — check sweep_findings.md) OR an edit was forced when none was needed
# (a failure — revert it).

# The ONLY working-tree change (expected no-edit case) is the sweep_findings.md note:
git status --porcelain
# Expected (no-edit): ?? plan/019_.../P2M1T2S1/research/sweep_findings.md  (and nothing else).
```

### Level 3: Scope guard (regardless of verdict)

```bash
# NO Mode A files re-edited; NO spec/; NO code; NO tasks/snapshot.
git status --porcelain | grep -E 'how-it-works\.md|providers\.md|^spec/|internal/|providers/.*\.toml|tasks\.json|prd_snapshot' \
  && echo "FAIL: forbidden/out-of-scope file touched" || echo "OK: scope clean"
# Expected: "OK: scope clean". docs/how-it-works.md + docs/providers.md MUST be untouched (Mode A done);
# spec/ is READ-ONLY; no code/tasks/snapshot.

# IF an edit WAS made (stale-claim case), confirm it touched ONLY README.md and/or docs/README.md:
git diff --name-only | grep -vE '^(README\.md|docs/README\.md)$' | grep -vE 'research/sweep_findings\.md' \
  && echo "FAIL: unexpected edited file" || echo "OK: only overview docs (if any) + the note"
# Expected: "OK: ..." (an edit to README.md/docs/README.md is valid ONLY IF sweep_findings.md records the stale claim).
```

### Level 4: Re-derivation sanity (the candidate-claim list is complete)

```bash
# Confirm the sweep checked the COMPLETE set of claims touching the changeset's subject matter.
grep -niE "stager|tooled|disjoint|fast-path|overlap|decompose" README.md | wc -l
# Expected: a small number (the 5 candidate claims + maybe the comparison-table 'stager' hit). Every hit
# should appear in sweep_findings.md with a verdict. If a hit is NOT in the note, the sweep was incomplete.

# Confirm the fast-path has no CLI/config surface (so no reference-doc claim can be stale).
grep -niE "fast.path|disjoint" docs/cli.md docs/configuration.md
# Expected: EMPTY (zero hits) — confirms the fast-path is invisible and docs/cli.md + docs/configuration.md
# need no edit.
```

## Final Validation Checklist

### Technical Validation
- [ ] Level 1: `plan/019_2f5621db4d2b/P2M1T2S1/research/sweep_findings.md` exists and records a verdict +
      rationale for every candidate claim (README L6/L32/L64/L209/L425 + docs/README.md rows + cli/
      configuration glance)
- [ ] Level 1: the note has a clear SUMMARY verdict line ("no stale top-level claims; no edit needed" OR
      "N stale claim(s) corrected")
- [ ] Level 2: IF all accurate — `git diff --stat README.md docs/README.md` is empty (no forced edit)
- [ ] Level 3: scope clean — no Mode A file (how-it-works.md, providers.md), no spec/, no code, no
      tasks.json, no prd_snapshot touched
- [ ] Level 4: every `grep` hit for stager/tooled/disjoint/fast-path/overlap/decompose in README.md
      appears in the note with a verdict (the sweep is complete, not sampled)

### Feature Validation (documentation correctness)
- [ ] Every "no edit" verdict cites why the changeset does NOT invalidate the claim (invisible surface /
      non-exhaustive example / different axis) — defensible, not hand-waved
- [ ] IF an edit was made: it corrects ONLY a factually-wrong claim, matches the existing voice/long-line
      style, and sweep_findings.md records what changed + why
- [ ] The L209 stager sentence was NOT wrongly expanded with agy/codex/opencode/cursor (that's providers.md
      granularity — README stays high-level)
- [ ] The L425 verification-status claim was NOT "corrected" (it's a different axis; any real-world drift
      there is out of scope for THIS changeset)

### Scope-Boundary Validation
- [ ] `git status --porcelain` shows ONLY `plan/.../research/sweep_findings.md` (no-edit case) OR that
      note + a surgical README.md/docs/README.md edit (stale-claim case)
- [ ] `docs/how-it-works.md` UNCHANGED (Phase 1 Mode A — P1.M1.T1.S6)
- [ ] `docs/providers.md` UNCHANGED (Phase 2 Mode A — P2.M1.T1.S1)
- [ ] `spec/` UNCHANGED (human-owned, AGENTS.md hard rule 1)
- [ ] No code, providers/*.toml, tasks.json, or prd_snapshot.md touched

---

## Anti-Patterns to Avoid

- ❌ Don't skip the sweep because "the changeset is invisible." SOW §5 mandates it RUN and DECIDE. The
  deliverable is the recorded verdict (sweep_findings.md), even when the verdict is "no edit." A missing
  note = the task was skipped = failure.
- ❌ Don't force an edit to "feel productive." If every candidate claim is accurate (the expected case),
  the correct outcome is a CLEAN README/docs-README + the sweep_findings.md note. Inventing an edit is
  scope creep and contradicts the contract ("If README is already accurate, record that finding rather
  than forcing a change").
- ❌ Don't expand the L209 stager sentence with agy/codex/opencode/cursor. It's a correct NON-EXHAUSTIVE
  example (claude/pi); README intentionally does not enumerate all 6 stager-capable providers (PRD Phase 2
  Mode B note). That granularity lives in docs/providers.md (Phase 2, done). Adding them here = scope creep.
- ❌ Don't "correct" the L425 verification-status claim ("cursor is NOT yet verified"). That's about a real
  commit-generation run — a DIFFERENT axis than stager capability (tooled_flags). The changeset doesn't
  change verification status. Any real-world drift there is pre-existing and out of scope.
- ❌ Don't re-edit docs/how-it-works.md or docs/providers.md. They are Mode A files (Phase 1 / Phase 2),
  already edited by their own subtasks. Re-editing duplicates/conflicts with that work.
- ❌ Don't treat a non-exhaustive example as a stale claim. The decision criterion requires the changeset to
  make the claim FACTUALLY WRONG. Incomplete-but-correct (claude/pi examples) is NOT wrong.
- ❌ Don't edit spec/, code, providers/*.toml, tasks.json, or prd_snapshot.md. This is a docs sweep; those
  are out of scope (spec/ human-owned; code needs its own PRP; tasks/snapshot orchestrator-owned).
- ❌ Don't run `go build`/`go test`/`make` as a gate. No Go code is touched; the gates are the note's
  completeness + the no-edit/scope guards (Levels 1-4). (Running them is harmless but wastes a cycle.)
- ❌ Don't hard-wrap an edit to 80 columns (MD013 is OFF; match the existing long-line style).

---

## Confidence Score: 9/10

The candidate-claim checklist is exhaustive (re-derived by grep, verified against the live README +
docs/README.md), every claim's expected verdict is evidenced (findings §2/§3), the decision criterion is
concrete (stale = factually wrong; non-exhaustive/different-axis = accurate), and the recording mechanism
is concrete (sweep_findings.md). The expected outcome (no edit) is strongly supported by: (a) the fast-path
has ZERO CLI/config surface (grep-confirmed), so it cannot invalidate any user-facing README claim; (b) the
PRD's own Phase 2 Mode B note states README does not enumerate per-provider stager status. The 1-point
deduction is for the residual judgment call on L209 (the stager sentence looks borderline but is correct)
and L425 (verification drift unrelated to this changeset) — both could tempt an over-eager editor to "fix"
them; the Known Gotchas + Anti-Patterns section guard against that explicitly.