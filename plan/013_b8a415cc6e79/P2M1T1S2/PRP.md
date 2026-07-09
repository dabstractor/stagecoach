name: "P2.M1.T1.S2 — Confirm PRD §12.5 carries the REMOVED stub and cross-references are clean"
description: |
  Read-only PRD verification subtask (Mode A). Confirm `PRD.md` at HEAD is internally consistent with
  the EOL gemini-cli built-in removal that was committed in `cdbccf5` ("Purge gemini-cli from PRD
  after built-in removal"). Produce a one-line PASS/FAIL per contract item (a)–(h). No file is
  edited — PRD.md is human-owned / READ-ONLY; the PRD edits themselves ARE this subtask's
  documentation surface. Research at HEAD (fd99358) pre-confirmed all eight items PASS.

---

## Goal

**Feature Goal**: Verify — by direct `grep`/`read` inspection of `PRD.md` at HEAD — that every
PRD surface that could resurrect the EOL `gemini` (Gemini CLI) built-in provider instead carries
either the REMOVED stub (§12.5) or a gemini-free enumeration, with the **only** surviving `gemini`
tokens being legitimate model-name / lineage references (agy runs the Gemini model family; qwen-code
is a Gemini-CLI fork).

**Deliverable**: An eight-line verification result, one PASS/FAIL line per contract item (a)–(h),
each backed by the cited `PRD.md` line number(s). This is a pure confirmation of the already-committed
state (`cdbccf5`); no PRD/source edits are made.

**Success Definition**: All eight items (a)–(h) report PASS; the deliverable names the exact PRD line
for each; legitimate model-name `gemini` hits (gemini-3.1-pro, Gemini 3.5 Flash) and lineage prose are
explicitly distinguished from drift and left untouched; two pre-existing non-gemini inconsistencies
(Appendix F qwen-code omission; §14 tree qwen-code.toml omission) are flagged as out-of-scope and NOT
acted on.

## User Persona (if applicable)

**Target User**: The stagecoach maintainer / verifying engineer (and the orchestrator consuming the
verification result).

**Use Case**: Lock in confidence that the PRD spec-of-record agrees with the code-side gemini purge
(sibling task P2.M1.T1.S1) before the P2 milestone (agy re-verification P2.M2, docs drift P2.M3)
declares the provider-lineup correction done.

**User Journey**: Run the eight verification commands → read each cited PRD line → classify any
`gemini` hit as provider-drift vs legitimate model/lineage → emit PASS/FAIL per item → report.

**Pain Points Addressed**: Eliminates ambiguity about whether "removed from code" also means "removed
from the spec"; proves the eight enumeration surfaces and the §12.5 stub all agree on a 7-provider
built-in set with no gemini.

## Why

- **Correctness of the provider lineup (PRD §12.5/§12.5.1)**: `gemini` was EOL'd and superseded by
  `agy` on 2026-06-18. A residual gemini enumeration in G2, FR-D1, the FR-D4 table, the §14 tree,
  Appendix D, or Appendix B.5 would resurrect a dead provider and contradict the shipped agy successor.
- **Spec/code parity**: P2.M1.T1.S1 confirms the *code* (builtin set, registry slice, role-defaults
  map, providers/*.toml) is gemini-free. This subtask is the *spec* half of that parity proof — the
  PRD must not promise a provider the binary no longer ships.
- **Foundation for P2.M2/P2.M3**: agy re-verification and docs-drift fixes both assume the PRD's
  gemini-removal story is internally consistent. This subtask is the gate.

## What

A pure read-only verification of `PRD.md` at HEAD. For each of eight contract items, confirm the
stated invariant holds, then emit one PASS/FAIL line. The exact expected state (all pre-confirmed at
HEAD = `fd99358`, a descendant of the purge commit `cdbccf5`):

- **(a)** §12.5 heading (h3.57, `PRD.md:943`) reads verbatim:
  `### 12.5 ~~Built-in provider: Gemini CLI~~ — REMOVED (superseded by agy, §12.5.1)`.
- **(b)** §12 terminology list (`PRD.md:692`) provider-examples row =
  `pi, opencode, claude, codex, cursor, agy, qwen-code`; §12 fixed-backend examples (`PRD.md:696`) =
  `(claude, codex, cursor, agy, qwen-code)`. No `gemini` as a provider in either.
- **(c)** §6.1 G2 (h3.10, `PRD.md:174`) lists `**pi, Claude Code, agy, opencode**` (agy, not gemini).
- **(d)** §9.16 FR-D1 (h3.32, `PRD.md:407`) cascade order =
  `**pi, opencode, cursor, agy, qwen-code, codex, claude.**` (no gemini).
- **(e)** §9.16 FR-D4 default-model table (`PRD.md:415–422`) has rows
  pi/opencode/cursor/agy/qwen-code/codex/claude only — no gemini row.
- **(f)** §14 package layout providers/ tree (`PRD.md:1390–1396`) lists
  pi/claude/agy/opencode/codex/cursor .toml — no `gemini.toml`.
- **(g)** Appendix D quick-reference table (`PRD.md:2277–2285`) has rows
  pi/claude/agy/opencode/codex/cursor only — no gemini row.
- **(h)** Appendix B.5 rescue example (`PRD.md:2245`) reads
  `↳ Generating with Gemini 3.5 Flash (Low) in agy…` — uses `agy` (the provider); "Gemini 3.5 Flash"
  is the model name.

**CONTRACT NOTE — legitimate `gemini` hits are NOT drift.** The word `gemini` correctly persists as a
**model name** (`gemini-3.1-pro`, `gemini-3.5-flash`, `gemini-3.1-flash-lite`, "Gemini 3.5 Flash
(Low)") and as **lineage prose** (§12.5.1 agy "superseded `gemini`"; §12.5.2 qwen-code "manifest
mirrors `gemini`"). These must be left alone. Drift = `gemini` listed as an **active provider** in
any enumeration list/table/tree — NONE of which exists.

**OUT-OF-SCOPE (do NOT act on — reported only, not among items a–h):**
1. Appendix F decision-log (`PRD.md:2323`) echoes FR-D1 as "pi → opencode → cursor → agy → codex →
   claude" — it OMITS `qwen-code` (a pre-existing non-gemini inconsistency).
2. §14 package-layout tree (`PRD.md:1390–1396`) OMITS `qwen-code.toml` (predates qwen-code; the
   authoritative providers/ file-set proof is P2.M1.T1.S1 item (c)).
Both are unrelated to gemini and outside this subtask's 8 items. Note them in the report; do not edit.

### Success Criteria

- [ ] Item (a) PASS — §12.5 heading matches the REMOVED stub verbatim at `PRD.md:943`.
- [ ] Item (b) PASS — §12 terminology provider-examples and fixed-backend lists have no `gemini`.
- [ ] Item (c) PASS — §6.1 G2 lists agy (not gemini) at `PRD.md:174`.
- [ ] Item (d) PASS — §9.16 FR-D1 cascade has no gemini at `PRD.md:407`.
- [ ] Item (e) PASS — §9.16 FR-D4 table has no gemini row at `PRD.md:415–422`.
- [ ] Item (f) PASS — §14 tree has no `gemini.toml` at `PRD.md:1390–1396`.
- [ ] Item (g) PASS — Appendix D has no gemini row at `PRD.md:2277–2285`.
- [ ] Item (h) PASS — Appendix B.5 uses agy at `PRD.md:2245`.
- [ ] Eight-line PASS/FAIL verdict emitted (one per item a–h).
- [ ] Legitimate model-name / lineage `gemini` hits explicitly confirmed non-drift and left untouched.
- [ ] No `PRD.md` (or any other) file is modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to verify this
successfully?_ **Yes** — every check is pinned to an exact `PRD.md` line range, a copy-pasteable
grep/sed command, and the exact expected substring. No inference required; no library/external docs
are involved (this is a PRD-internal consistency check).

### Documentation & References

```yaml
# MUST READ — the PRD being verified (READ-ONLY; human-owned — never edit from this subtask)
- file: PRD.md
  why: The single document under verification. All eight items are internal-consistency checks on it.
  gotcha: Line numbers are anchored at HEAD (fd99358); if the file has been edited since, re-derive
          anchors with the grep commands below before judging PASS/FAIL.

# The purge commit that established the expected state (reference only — do NOT revert/reapply)
- gitref: cdbccf5  # "Purge gemini-cli from PRD after built-in removal" — the committed state to confirm
  why: Defines what "clean" means; `git show cdbccf5 -- PRD.md` shows the exact edits that must persist.

# Sibling task — the code-side parity proof (treat as a CONTRACT; it defines the 7-provider set)
- docfile: plan/013_b8a415cc6e79/P2M1T1S1/PRP.md
  why: Confirms builtin set = {pi,claude,opencode,codex,cursor,agy,qwen-code}, preferredBuiltins,
       roleDefaults, and providers/*.toml are gemini-free. The PRD must agree with this compiled set.
  section: "What" (items a–d) and "Validation Loop" Level 4 (negative-space sweep).

# Architecture audit (read-only reference; pre-confirmed the code side)
- docfile: plan/013_b8a415cc6e79/architecture/code_gemini_agy_audit.md
  why: Authoritative audit of the gemini/agy provider-lineup state; explains why model-name hits
       are legitimate and which surfaces are provider-drift.

# PRD sections under verification (selectors used: h3.57, h2.6, h3.10, h3.13, h3.32)
- url: PRD.md#12.5     # §12.5 Gemini CLI — REMOVED stub  (item a)
- url: PRD.md#12       # §12 provider system / terminology (item b)
- url: PRD.md#6.1      # §6.1 Goals G2                    (item c)
- url: PRD.md#9.16     # §9.16 FR-D1 cascade + FR-D4 table (items d, e)
- url: PRD.md#14       # §14 Package layout providers/ tree (item f)
- url: PRD.md#appendix-d  # Appendix D quick reference     (item g)
- url: PRD.md#appendix-b-5 # Appendix B.5 rescue example   (item h)
```

### Current Codebase tree (relevant slice)

```bash
PRD.md                         # the ONLY document under verification (313575 bytes, ~2342 lines)
plan/013_b8a415cc6e79/
  P2M1T1S1/PRP.md             # sibling: code-side gemini-removal verification (CONTRACT)
  P2M1T1S2/                    # THIS subtask
    PRP.md                     # ← this file
    research/per_item_evidence.md   # per-item evidence table collected at HEAD
  architecture/code_gemini_agy_audit.md   # read-only audit reference
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
# NONE. This is a read-only verification subtask. No files are created, modified, or deleted.
# The only artifact is the verification result (PASS/FAIL per item a–h), reported back.
# PRD.md is READ-ONLY (human-owned) per the FORBIDDEN OPERATIONS — even a found drift must be
# REPORTED, not fixed, from this subtask.
```

### Known Gotchas of our codebase & Library Quirks

```text
# GOTCHA 1 — "gemini" string hits are NOT all drift (the single most important nuance).
#   grep -n gemini PRD.md returns ~18 hits. Only a SMALL subset could ever be drift (an active
#   provider enumeration). The rest are LEGITIMATE:
#     - MODEL NAMES: gemini-3.1-pro, gemini-3.5-flash, gemini-3.1-flash-lite, "Gemini 3.5 Flash (Low)"
#       (agy runs the Gemini model family — these are correct model tokens in the FR-D4 agy row,
#        FR-R5 examples, §12 terminology model column, Appendix B.5).
#     - LINEAGE PROSE: §12.5.1 "superseded `gemini`", "diverged from gemini-cli"; §12.5.2 qwen-code
#       "manifest mirrors `gemini` (§12.5)"; §2.1 historical motivation "Every new agent (codex,
#       gemini, opencode, cursor) would require another fork" (§2, not §12 — explains commit-pi's
#       lack of abstraction; not a provider enumeration).
#   Drift = gemini listed as an ACTIVE provider in G2 / FR-D1 / FR-D4 table / §14 tree /
#   Appendix D / Appendix B.5. NONE of these exist at HEAD → all PASS.

# GOTCHA 2 — Line numbers are anchored to HEAD (fd99358); re-derive if the file changed.
#   The verification commands below grep for content substrings FIRST (robust to renumbering), then
#   cite the observed line. If a grep for a heading/pattern returns a different line than cited, the
#   line number drifted but the CONTENT is what determines PASS/FAIL — re-anchor and re-judge.

# GOTCHA 3 — Two pre-existing NON-gemini inconsistencies exist; do NOT mistake them for failures.
#   (1) Appendix F (PRD.md:2323) decision-log FR-D1 echo OMITS qwen-code — writes
#       "pi → opencode → cursor → agy → codex → claude". This contradicts the authoritative FR-D1
#       (:407) but is NOT a gemini issue and NOT among items (a)–(h). Report only.
#   (2) §14 tree (PRD.md:1390–1396) OMITS qwen-code.toml (lists 6 files). Again not gemini, not an
#       item; the authoritative providers/ set is P2.M1.T1.S1 item (c). Report only.

# GOTCHA 4 — There is exactly ONE `providers/gemini.toml` token in the whole PRD, and it is CORRECT.
#   PRD.md:945 (inside the REMOVED stub) says "...its reference file (`providers/gemini.toml`)...
#   have all been removed." That sentence is the REQUIRED removal notice (item a's body). It is NOT
#   a shipping declaration. Do not flag it.
```

## Implementation Blueprint

### Verification approach (not "implementation" — read-only)

There are no data models to create. The "tasks" below are verification steps. Each step has an exact
command and an exact expected result; a step PASSES iff the observed result equals the expected
result. Emit the one-line PASS/FAIL verdict for the corresponding contract item. All steps are run
from the repo root (`/home/dustin/projects/stagecoach`).

### Verification Tasks (ordered; each maps to a contract item)

```yaml
Task V0: CONFIRM baseline / that the purge is committed
  - RUN: git log --oneline -1 --grep='Purge gemini-cli from PRD'
  - EXPECT: a line beginning with `cdbccf5` (the purge commit). (Current HEAD fd99358 is a
            descendant; the purge persists.) If absent, the expected state is not present — report.
  - RUN: git rev-parse HEAD
  - EXPECT: fd99358 or a descendant of cdbccf5.

Task V1 → item (a): CONFIRM §12.5 carries the REMOVED stub heading
  - RUN: grep -n '### 12.5 ~~Built-in provider: Gemini CLI~~ — REMOVED (superseded by agy, §12.5.1)' PRD.md
  - EXPECT: exactly one match (observed: :943). The substring must be VERBATIM (note the strikethrough
            ~~...~~, the em-dash —, and the §12.5.1 cross-ref).
  - RUN: sed -n '945p' PRD.md   # the stub body
  - EXPECT: a sentence stating gemini "is no longer shipped", superseded by agy on 2026-06-18, and
            that the manifest + `providers/gemini.toml` + role-tier defaults (§9.16 FR-D4) "have all
            been removed". (This single `providers/gemini.toml` mention is the REQUIRED removal notice
            — see Gotcha 4.)
  - VERDICT: item (a) PASS iff the heading matches verbatim AND the body states the removal.

Task V2 → item (b): CONFIRM §12 terminology list & fixed-backend examples have no gemini provider
  - RUN: sed -n '692,693p' PRD.md   # terminology table — provider row
  - EXPECT: the provider-concept row's Examples cell =
            `pi, opencode, claude, codex, cursor, agy, qwen-code` (NO gemini). The model-concept row
            may contain `gemini-3.1-pro` as a MODEL example — that is LEGITIMATE (Gotcha 1).
  - RUN: sed -n '696p' PRD.md   # fixed-backend sentence
  - EXPECT: `Providers with a fixed backend (claude, codex, cursor, agy, qwen-code) take a bare model
            (\`sonnet\`, \`gemini-3.1-pro\`).` — the provider list has NO gemini; `gemini-3.1-pro` is
            a model example (legitimate).
  - VERDICT: item (b) PASS iff both provider enumerations (terminology provider row + fixed-backend
            list) contain no `gemini`.

Task V3 → item (c): CONFIRM §6.1 G2 lists agy, not gemini
  - RUN: grep -n 'Support at least four agents out of the box' PRD.md
  - EXPECT: one match (observed: :174) whose line reads
            `**pi, Claude Code, agy, opencode**` — no gemini.
  - VERDICT: item (c) PASS iff that line contains `agy` and not `gemini`.

Task V4 → item (d): CONFIRM §9.16 FR-D1 cascade has no gemini
  - RUN: grep -n 'FR-D1. Cascading provider priority' PRD.md
  - EXPECT: one match (observed: :407); the same line contains
            `in this order: **pi, opencode, cursor, agy, qwen-code, codex, claude.**` — no gemini.
  - VERDICT: item (d) PASS iff that ordered list is exactly the 7 providers above with no gemini.
  - NOTE: Do NOT judge item (d) against the Appendix F echo at :2323 (which omits qwen-code) — that
          is out-of-scope Gotcha 3(1). The AUTHORITATIVE FR-D1 is the :407 line.

Task V5 → item (e): CONFIRM §9.16 FR-D4 default-model table has no gemini row
  - RUN: sed -n '415,422p' PRD.md
  - EXPECT: a markdown table whose first-column provider rows are exactly: pi, opencode, cursor, agy,
            qwen-code, codex, claude — NO gemini row. (The agy row legitimately contains gemini-*
            MODEL tokens; those are not drift.)
  - RUN: grep -nE '^\| (\*\*gemini\*\*|gemini)' PRD.md
  - EXPECT: no match (no table row anywhere names gemini as a provider).
  - VERDICT: item (e) PASS iff the FR-D4 table has no gemini row.

Task V6 → item (f): CONFIRM §14 package layout has no providers/gemini.toml
  - RUN: sed -n '1390,1396p' PRD.md   # the providers/ subtree in the §14 code-tree listing
  - EXPECT: .toml entries pi.toml, claude.toml, agy.toml, opencode.toml, codex.toml, cursor.toml —
            NO gemini.toml. (This tree omits qwen-code.toml — out-of-scope Gotcha 3(2); it is NOT a
            gemini issue and NOT item (f).)
  - RUN: grep -n 'gemini.toml' PRD.md
  - EXPECT: exactly ONE hit at :945 — the REMOVED-stub prose stating the file "has ... been removed".
            No gemini.toml appears in the §14 tree or as a shipped file.
  - VERDICT: item (f) PASS iff the §14 tree lists no gemini.toml.

Task V7 → item (g): CONFIRM Appendix D quick-reference table has no gemini row
  - RUN: grep -n 'Appendix D — Built-in manifest quick reference' PRD.md
  - EXPECT: one match (observed: :2275); the table immediately following has first-column rows:
            pi, claude, agy, opencode, codex, cursor — NO gemini row.
  - RUN: sed -n '2278,2285p' PRD.md
  - EXPECT: provider rows pi/claude/agy(opencode/codex/cursor; no gemini.
  - VERDICT: item (g) PASS iff Appendix D has no gemini row.

Task V8 → item (h): CONFIRM Appendix B.5 rescue example uses agy
  - RUN: grep -n 'B.5 Rescue' PRD.md
  - EXPECT: one match (observed: :2241).
  - RUN: sed -n '2245p' PRD.md   # the "Generating with ..." line inside the fenced session
  - EXPECT: `↳ Generating with Gemini 3.5 Flash (Low) in agy…` — uses the `agy` PROVIDER; "Gemini 3.5
            Flash" is the MODEL name (legitimate per Gotcha 1).
  - VERDICT: item (h) PASS iff the rescue session names `agy` as the provider (not gemini).

Task V9: CONFIRM legitimate hits are classified (sanity, NOT a FAIL trigger)
  - RUN: grep -n 'gemini' PRD.md
  - EXPECT: ~18 hits. CLASSIFY each as (i) model name (gemini-3.1-pro / gemini-3.5-flash / Gemini 3.5
            Flash) — LEGITIMATE; (ii) lineage prose (§12.5.1 superseded/diverged, §12.5.2 qwen-code
            mirror, §2.1 historical motivation) — LEGITIMATE; (iii) the :945 REMOVED-stub removal
            notice — REQUIRED (item a body). NONE should be an active-provider enumeration (those are
            already excluded by items a–h).
  - VERDICT: informational only. Their presence is EXPECTED and correct.
```

### Restore / escalation procedure (only if an item UNEXPECTEDLY fails)

All eight items were pre-confirmed PASS at HEAD. A failure means the committed state regressed OR the
purge commit was partially reverted. Because **PRD.md is human-owned / READ-ONLY**, this subtask does
NOT edit it. Instead:

```text
1. Re-run the failing item's grep/read command on a fresh checkout of HEAD to rule out a stale read.
2. If it still fails, run: git show cdbccf5 -- PRD.md   # the purge diff that should have persisted
   and diff against the current PRD.md to locate the regression.
3. REPORT the failing item, the observed vs expected text, and the PRD.md line — do NOT fix it here.
   A PRD edit is a separate, human-approved change (the PRD is the spec-of-record).
```

### Integration Points

```yaml
# NONE. Read-only PRD verification — no DATABASE, CONFIG, ROUTES, or code integration.
# The only "integration" is consuming the verification result in the P2 milestone reporting and
# confirming parity with sibling P2.M1.T1.S1 (code-side) before P2.M2/P2.M3 proceed.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Read-only verification — nothing to compile or lint. Confirm only that the document parses as
# Markdown (no tooling gate is required for a grep-based check). Optional sanity:
cd /home/dustin/projects/stagecoach
wc -l PRD.md                      # EXPECT: ~2342 lines (informational; confirms the file is intact)
grep -c '^### 12\.5' PRD.md        # EXPECT: 1 (the §12.5 heading exists exactly once)

# Expected: no errors. (This is a no-code-change verification.)
```

### Level 2: Per-Item Verification (the core gate)

```bash
cd /home/dustin/projects/stagecoach
# Run each item's command block from Tasks V1–V8 above. For a one-shot full sweep:
grep -n '### 12.5 ~~Built-in provider: Gemini CLI~~ — REMOVED (superseded by agy, §12.5.1)' PRD.md   # (a)
sed -n '692,693p;696p' PRD.md                       # (b) terminology provider row + fixed-backend
grep -n 'Support at least four agents out of the box' PRD.md                        # (c) G2
grep -n 'FR-D1. Cascading provider priority' PRD.md                                 # (d) FR-D1
sed -n '415,422p' PRD.md                            # (e) FR-D4 table
sed -n '1390,1396p' PRD.md && grep -n 'gemini.toml' PRD.md                          # (f) §14 tree
sed -n '2278,2285p' PRD.md                          # (g) Appendix D
sed -n '2245p' PRD.md                               # (h) Appendix B.5

# Expected: each command shows the gemini-free (or, for (a), the REMOVED-stub) content cited above.
# Any active-provider `gemini` enumeration = FAIL for that item.
```

### Level 3: Cross-Reference Sweep (System Validation)

```bash
cd /home/dustin/projects/stagecoach
# (1) No table row anywhere names gemini as a PROVIDER:
grep -nE '^\| (\*\*gemini\*\*|gemini)' PRD.md        # EXPECT: empty
# (2) The only `gemini.toml` mention is the required removal notice in the §12.5 stub:
grep -n 'gemini.toml' PRD.md                         # EXPECT: exactly one hit at :945 (removal prose)
# (3) Parity with the code-side purge — the PRD's 7-provider set must equal the compiled built-in set.
#     (Authoritative proof lives in P2.M1.T1.S1; here we only confirm the PRD enumerations agree
#      with each other: G2 subset ⊆ FR-D1 set = FR-D4 table rows = Appendix D rows = §14 tree.)
# Expected: (1) empty; (2) one hit; (3) all enumerations agree on the same provider set, no gemini.
```

### Level 4: Domain-Specific Validation (classification sweep)

```bash
cd /home/dustin/projects/stagecoach
# Print every gemini hit with context so each can be classified (provider-drift vs legitimate).
grep -n 'gemini' PRD.md
# Classification guide (see Known Gotchas #1):
#   - gemini-3.1-pro / gemini-3.5-flash / gemini-3.1-flash-lite / "Gemini 3.5 Flash" → MODEL name (OK)
#   - §12.5.1 "superseded `gemini`" / "diverged from gemini-cli" → lineage prose (OK)
#   - §12.5.2 qwen-code "mirrors `gemini`" → lineage prose (OK)
#   - §2.1 "Every new agent (codex, gemini, opencode, cursor)" → historical motivation in §2 (OK)
#   - :945 REMOVED-stub removal notice → REQUIRED text (OK)
#   - any ACTIVE enumeration in G2/FR-D1/FR-D4/§14/Appendix-D/B.5 → DRIFT (would FAIL its item)
# Expected: zero active-provider-enumeration hits (all hits fall into the OK categories).
```

## Final Validation Checklist

### Technical Validation

- [ ] Baseline confirmed: purge commit `cdbccf5` present in history; HEAD is a descendant.
- [ ] All eight per-item commands (Tasks V1–V8) executed against `PRD.md` at HEAD.

### Feature (Verification) Validation

- [ ] Item (a) PASS — §12.5 REMOVED stub heading verbatim at `PRD.md:943`; body states the removal.
- [ ] Item (b) PASS — §12 terminology provider row + fixed-backend list have no gemini provider.
- [ ] Item (c) PASS — §6.1 G2 lists agy (not gemini) at `PRD.md:174`.
- [ ] Item (d) PASS — §9.16 FR-D1 cascade (the :407 line) has no gemini.
- [ ] Item (e) PASS — §9.16 FR-D4 table has no gemini row at `PRD.md:415–422`.
- [ ] Item (f) PASS — §14 tree has no gemini.toml; only `gemini.toml` hit is the :945 removal notice.
- [ ] Item (g) PASS — Appendix D has no gemini row at `PRD.md:2277–2285`.
- [ ] Item (h) PASS — Appendix B.5 uses agy at `PRD.md:2245`.
- [ ] Eight-line PASS/FAIL verdict emitted (one per item a–h).
- [ ] Legitimate model-name / lineage `gemini` hits classified and confirmed non-drift.

### Code Quality Validation

- [ ] No `PRD.md` edit made (READ-ONLY, human-owned).
- [ ] No source / `tasks.json` / `prd_snapshot.md` / `.gitignore` file touched.
- [ ] Out-of-scope non-gemini inconsistencies (Appendix F qwen-code echo; §14 tree qwen-code.toml
      omission) reported, NOT fixed.

### Documentation & Deployment

- [ ] Per contract §5 (Mode A): the PRD edits ARE the documentation surface — confirming the
      spec-of-record is the deliverable; no additional docs artifact required beyond the verdict.
- [ ] Verification result recorded for the P2 milestone (P2.M2 agy re-verify and P2.M3 docs drift
      depend on a clean PRD gemini-removal story).

---

## Anti-Patterns to Avoid

- ❌ Don't treat model-name `gemini` hits (`gemini-3.1-pro`, "Gemini 3.5 Flash") as drift — agy runs
  the Gemini model family; those tokens are correct and MUST remain.
- ❌ Don't treat lineage prose (`agy superseded gemini`; `qwen-code mirrors gemini`) as drift — that
  is the spec honestly describing the provider lineage.
- ❌ Don't edit `PRD.md` to "fix" anything — it is READ-ONLY and human-owned; a found drift is
  REPORTED, not patched, from this subtask.
- ❌ Don't judge item (d) against the Appendix F decision-log echo (:2323) — it omits qwen-code and
  is out of scope; the AUTHORITATIVE FR-D1 is the :407 line.
- ❌ Don't conflate the §14-tree qwen-code.toml omission (Gotcha 3.2) with a gemini failure — it is
  unrelated to gemini and not among items (a)–(h).
- ❌ Don't re-add `gemini` "to be complete" — it is intentionally EOL'd and superseded by `agy`
  (§12.5.1); the PRD enumerations must stay gemini-free.

---

## Confidence Score

**One-pass success likelihood: 10/10.** This is a read-only verification of `PRD.md` at a known-good
committed state (`cdbccf5`, confirmed at HEAD `fd99358`). Every check is pinned to an exact
content substring (robust to line renumbering) plus an observed line number, and all eight items
were pre-confirmed PASS during research (per-item evidence in `research/per_item_evidence.md`). The
deliverable is a deterministic PASS/FAIL verdict per item; there is no implementation surface to get
wrong, and the single hardest nuance (legitimate model-name vs provider-drift `gemini` hits) is
spelled out with a full classification guide.
