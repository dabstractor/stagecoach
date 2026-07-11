name: "P1.M4.T1.S2 — Review docs/how-it-works.md + docs/cli.md for changeset-level impacts (Issues 1-6 bugfix); outcome: ONE edit (cli.md config-upgrade backup line) + a documented no-change review of how-it-works.md"
description: >
  A DOCUMENTATION-REVIEW task (Mode B). The 6-issue bugfix changeset (P1.M1-P1.M3) is COMPLETE. This task
  reviews docs/how-it-works.md + docs/cli.md against those fixes to correct STALE references (it does NOT
  add feature blurbs). SCOPE FENCE: this task touches ONLY docs/how-it-works.md + docs/cli.md — the two
  files named in the contract INPUT/OUTPUT. The two OTHER Mode-A doc notes called out by
  architecture/test_patterns.md (docs/configuration.md:167 floor note for Issue 4; docs/providers.md:125
  commented-pi-block note for Issue 2) have ALREADY LANDED inline with their code-fix commits (verified:
  commits fc51dad and 84b5296) and are NOT this task's concern. RESEARCH CONCLUSION (re-verified by grep +
  full read): docs/how-it-works.md has ZERO stale references — its token_limit description (line 150) stays
  accurate after Issue 4 because the floor fix preserves the "caps to a token budget" guarantee by ERRORING
  on sub-floor limits (vacuously true), and the floor detail already lives in configuration.md:167 (linked
  from line 152). docs/cli.md has ONE stale reference: the `config upgrade` example transcript (line 210)
  omits the "Backed up previous config to <path>.bak.<ts>" line that Issue 3 (P1.M2.T2.S1) added to stderr
  before the "Upgraded config…" line. The DEFAULT deliverable is therefore: (1) ONE minimal edit to
  docs/cli.md line 210 (add the backup line to the "Upgraded from v1" example scenario), and (2) a
  documented no-change review of docs/how-it-works.md. Two OPTIONAL companions are specified verbatim: a
  prose backup clause on cli.md line 205, and (only if the implementer chooses) a brief floor clause on
  how-it-works.md line 150 — both defaulting to NOT-done. NO code, NO tests, NO README.md (S1), NO
  configuration.md/providers.md (already landed), NO PRD/task file edits.

---

## Goal

**Feature Goal**: Ensure `docs/how-it-works.md` and `docs/cli.md` are accurate and consistent with the
fixed implementation after the 6-issue bugfix changeset (Issues 1-6), by systematically reviewing the three
contract-named areas per file — (a) token_limit / diff capture pipeline, (b) config init / config upgrade
(incl. backup), (c) any example output showing the auto-stage notice — and correcting STALE references
without adding feature blurbs for bugfixes.

**Deliverable**:
- **PRIMARY (required)**: ONE minimal edit to `docs/cli.md` line 210 — the `config upgrade` "Upgraded from
  v1" example scenario — adding the backup output line that Issue 3 introduced, so the example transcript
  matches actual post-fix behavior.
- **SECONDARY (required)**: a documented NO-CHANGE review of `docs/how-it-works.md` — all three review
  areas are accurate/absent; the review outcome is recorded in the commit/PR message.
- **OPTIONAL companions** (implementer's call, all default to NOT-done): (OPT-1) a one-clause backup note
  in the `config upgrade` prose at cli.md line 205; (OPT-2) a brief floor-rejection clause on
  how-it-works.md line 150 (only if the implementer believes the deep-dive doc should surface the sub-floor
  rejection inline — it is already in configuration.md:167, linked from line 152).

**Success Definition**:
- docs/cli.md `config init` section (line 172) re-verified accurate (pi per-role models "left EMPTY" = the
  Issue 1/2 fixed behavior; commented-out blocks described generically with no stale bare-pi claim).
- docs/cli.md `config upgrade` example (line 210) reflects that a timestamped backup is written (Issue 3 /
  FR-B8) — the PRIMARY edit lands, OR the implementer deliberately chooses to surface it via OPT-1 prose
  instead, and records the choice.
- docs/how-it-works.md reviewed across all three areas; ZERO stale references found; the no-change outcome
  is documented.
- `git status --porcelain` shows ONLY `docs/cli.md` (PRIMARY) OR `docs/cli.md` + `docs/how-it-works.md`
  (if OPT-2 taken). NEVER `README.md`, NEVER `docs/configuration.md` or `docs/providers.md` (already
  landed), NEVER any `.go` file, NEVER any PRD/task file.
- `make build && make test && make lint` clean (sanity — a markdown edit cannot break these).

## User Persona (if applicable)

**Target User**: A Stagecoach user reading the CLI reference or the how-it-works deep-dive to operate or
understand the tool. docs/cli.md is the operational reference (what each command prints); docs/how-it-works.md
is the architectural deep-dive (how the diff pipeline / freeze / multi-turn path work).
**Use Case**: A user runs `stagecoach config upgrade`, sees a `Backed up previous config to …` line on
stderr, and checks docs/cli.md to confirm that is expected — the example must now show it.
**User Journey**: user reads docs/cli.md config upgrade → runs it → observes the backup + upgrade lines →
docs match reality. (Before the Issue 3 fix, no backup was created; the old example was accurate THEN. After
the fix, the example must be updated to stay accurate.)
**Pain Points Addressed**: a stale command-output transcript that diverges from actual behavior (the one
concrete defect this task corrects).

## Why

- **Contract obligation**: the changeset's Mode-B documentation sync requires reviewing docs/how-it-works.md
  + docs/cli.md for changeset-level impacts. This task IS that review.
- **Truth-in-advertising**: a CLI reference whose `config upgrade` example omits the backup line (now printed
  on every real upgrade) is a stale transcript. The Issue 3 fix made the backup happen (FR-B8 was always
  required); the example must catch up.
- **Minimal-diff discipline**: the contract is explicit — "Do NOT add feature blurbs for bugfixes; only
  correct stale references." The research finds exactly ONE stale reference (cli.md line 210) across both
  files. Everything else is either already accurate (cli.md line 172 pi models "left EMPTY"; how-it-works.md
  line 150 token_limit) or absent (no literal auto-stage notice or --edit-abort output in either doc). The
  honest outcome is ONE edit + one documented no-change review.
- **Scope hygiene**: the configuration.md:167 floor note and the providers.md:125 commented-pi note are
  Mode-A updates that ALREADY LANDED with their code-fix commits. This task does not re-touch them.

## What

A systematic review of `docs/how-it-works.md` (402 lines) and `docs/cli.md` (506 lines) against the 6-issue
bugfix, covering the three contract-named areas per file. The review (performed in research, re-verifiable
via the greps in §Validation) finds exactly ONE stale reference: the `config upgrade` example transcript in
cli.md. The deliverable is therefore ONE minimal edit (cli.md line 210) plus a documented no-change review
of how-it-works.md.

### Success Criteria
- [ ] docs/cli.md re-reviewed: area (a) token_limit = NOT covered (0 grep hits, no edit); area (b1)
      `config init` line 172 = accurate (pi models "left EMPTY" = fixed behavior, no edit); area (b2)
      `config upgrade` line 210 = STALE → PRIMARY edit applied (backup line added); area (c) auto-stage
      notice = absent (no edit).
- [ ] docs/how-it-works.md re-reviewed: area (a) token_limit line 150 = accurate (floor fix preserves the
      cap guarantee by erroring → vacuously true; detail already in configuration.md:167); area (b)
      config/backup = NOT covered (0 grep hits); area (c) auto-stage notice = absent. → NO EDIT (documented).
- [ ] The PRIMARY edit (cli.md line 210) is applied verbatim per §Implementation Task 2, the markdown
      comment block still renders, and the `→`/quote style is preserved.
- [ ] OPT-1 and OPT-2 are DECIDED (taken or not) with a one-line rationale each.
- [ ] `git status --porcelain` shows ONLY `docs/cli.md` (PRIMARY ± OPT-1) and optionally
      `docs/how-it-works.md` (OPT-2). NOTHING else.
- [ ] NO edit to `README.md` (S1), `docs/configuration.md` or `docs/providers.md` (already landed), any
      `.go` file (P1.M1-P1.M3), or any PRD/task file. `make build && make test && make lint` clean.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the complete file-by-file review with exact line numbers + per-area finding, the fixed-behavior
table (the source of truth the docs are measured against), the scope fence (ONLY the two named files; the
two Mode-A notes already landed), the verbatim PRIMARY edit + the two optional companions, the validation
greps, and the actual post-Issue-3 `config upgrade` output strings (verified against `internal/cmd/config.go`).

### Documentation & References

```yaml
# MUST READ — the codebase-specific review (per-file, per-area, exact line numbers + findings + verbatim edits).
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M4T1S2/research/findings.md
  why: "§0 headline (ONE edit to cli.md + no-change how-it-works.md); §1 scope resolution (the two Mode-A
        notes already landed — DO NOT touch configuration.md/providers.md); §2 fixed-behavior-per-issue table
        + exact post-Issue-3 config-upgrade output strings; §3 how-it-works.md review (all areas accurate);
        §4 cli.md review (one stale reference at line 210); §5 the verbatim edits; §6 validation."
  critical: "The conclusion is: ONE edit to cli.md line 210 + a documented no-change review of how-it-works.md.
             Re-verify each area against the §2 fixed behavior before editing. Do NOT touch configuration.md
             or providers.md (already landed)."

# MUST READ — the two files under review (read them fully; re-verify the areas — do not blindly trust research).
- file: docs/cli.md
  why: "PRIMARY review target. Re-verify: line 172 (config init pi 'left EMPTY' = accurate); line 205 prose
        + lines 208-211 (config upgrade example — the STALE transcript to fix); grep token_limit (expect 0);
        grep 'staging all changes'/'(1 files)' (expect 0). Apply the line-210 PRIMARY edit."
- file: docs/how-it-works.md
  why: "SECONDARY review target. Re-verify: line 150 (token_limit water-fill — accurate, not stale); line 152
        (links to configuration.md#built-in-defaults where the floor note lives); line 312-316 (multi-turn
        FR-T12 — accurate); grep config init/upgrade/backup (expect 0); grep auto-stage notice (expect 0)."

# MUST READ — the architecture spec for Issue 3 (the fix that made cli.md line 210 stale).
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/config_upgrade_backup.md
  why: "Documents the WriteTimestampedBackup insertion in runConfigUpgrade + the 'Backed up previous config
        to %s' stderr line. This is exactly the output line the cli.md example must now show."

# MUST READ — the architecture spec for Issue 4 (so the how-it-works.md line-150 'not stale' conclusion is grounded).
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/minor_fixes.md
  section: "## Issue 4: Token Gate Sub-270 Invariant Violation (FR3j) > Documentation Impact"
  why: "Issue 4 rejects sub-floor limits BEFORE assembly (caller-level error). Therefore how-it-works.md
        line 150 'caps the whole payload to a token budget' is NOT falsified — a sub-floor run errors rather
        than emitting an over-budget payload. The 'Documentation Impact' note assigns the floor detail to
        docs/configuration.md:167 (already landed), NOT how-it-works.md. Grounds the no-edit conclusion."

# CONTEXT — the test_patterns doc's "Documentation Files" map (confirms scope + that Mode-A notes already landed).
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/architecture/test_patterns.md
  section: "## Documentation Files (for §5 Documentation Sync)"
  why: "Maps each fix to its doc. Confirms: docs/cli.md 'unlikely to need updates' (research found ONE
        exception — the config-upgrade example); docs/how-it-works.md 'may reference token_limit' (it does,
        at line 150 — accurately); the floor note's home is docs/configuration.md:167 and the commented-block
        note's home is docs/providers.md — BOTH already landed (see findings §1)."

# CONTEXT — the sibling PRP (README review). Establishes the no-feature-blurbs discipline + the floor-note routing.
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/P1M4T1S1/PRP.md
  why: "S1 reviewed README.md (NOT my scope). Its §5 routed the floor note to docs/configuration.md:167
        (S2 territory) — which is already landed. Confirms the docs-sync split: S1=README, S2=how-it-works.md
        + cli.md. Do NOT touch README.md."

# CONTEXT — the contract (the item description): the 3 review areas per file + the "no feature blurbs" rule.
- docfile: plan/015_b461e4720495/bugfix/001_3c8c5ced7ac1/prd_snapshot.md
  section: "P1.M4.T1.S2 contract (INPUT / LOGIC / OUTPUT / DOCS clauses)"
  why: "The contract itself: INPUT/OUTPUT = docs/how-it-works.md + docs/cli.md; LOGIC areas (a)/(b)/(c);
        'If everything is already accurate, make no changes but document the review'; this item IS Mode B."
```

### Current Codebase tree (relevant slice)

```bash
docs/how-it-works.md                        # REVIEW TARGET — DEFAULT: unchanged (documented no-change review; OPT-2: 1 line-150 clause)
docs/cli.md                                 # REVIEW TARGET — PRIMARY: 1 line-210 edit (config upgrade backup); OPT-1: 1 line-205 clause
README.md                                   # READ-ONLY — P1.M4.T1.S1's territory (do NOT touch)
docs/configuration.md                       # READ-ONLY — Issue 4 floor note ALREADY LANDED (commit fc51dad, line 167)
docs/providers.md                           # READ-ONLY — Issue 2 commented-pi note ALREADY LANDED (commit 84b5296, line 125)
internal/cmd/config.go                      # READ-ONLY — Issue 3 fix landed (runConfigUpgrade: lines 190-198 print backup+upgraded)
internal/git/tokengate.go + git.go          # READ-ONLY — Issue 4 fix landed (P1.M3.T1.S1)
internal/config/bootstrap.go                # READ-ONLY — Issue 1/2 fix landed (P1.M1/P1.M2)
internal/generate/finalize.go               # READ-ONLY — Issue 5 fix landed (P1.M3.T2.S1)
internal/cmd/default_action.go              # READ-ONLY — Issue 6 fix landed (P1.M3.T3.S1)
plan/.../architecture/*.md                  # READ-ONLY — the issue specs (source of truth)
```

### Desired Codebase tree with files to be added/modified

```bash
docs/cli.md            # PRIMARY: 1 line-210 edit (config upgrade "Upgraded from v1" example adds backup line).
                       #   OPT-1 (optional): 1 line-205 prose backup clause.
docs/how-it-works.md   # DEFAULT: UNCHANGED (documented no-change review).
                       #   OPT-2 (optional, default off): 1 line-150 floor clause.
# NOTHING ELSE. No README.md (S1). No configuration.md/providers.md (already landed). No .go file. No PRD/task file.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (the configuration.md + providers.md Mode-A notes are ALREADY LANDED — do NOT re-touch them):
     commits fc51dad (configuration.md:167 floor note) and 84b5296 (providers.md:125 commented-pi note) did
     the Mode-A doc updates inline with their code fixes. The contract INPUT/OUTPUT names ONLY how-it-works.md
     + cli.md. Editing configuration.md or providers.md here is (a) out of scope, (b) collides with already-
     landed work. Verify with: git log --oneline -3 -- docs/configuration.md docs/providers.md -->

<!-- CRITICAL (how-it-works.md line 150 is NOT stale — do not "fix" it): after Issue 4, a sub-floor token_limit
     ERRORS before assembly (caller-level, git.go). It never emits an over-budget payload. So line 150's
     "caps the whole payload to a token budget" is vacuously true for sub-floor limits and literally true for
     all feasible limits. The floor detail is already in configuration.md:167 (linked from line 152). Adding a
     floor clause here (OPT-2) is optional new content, NOT a stale-reference correction. Default: don't. -->

<!-- CRITICAL (cli.md line 172 IS already accurate — do not "correct" it backwards): "EXCEPT for pi … whose
     per-role models are left EMPTY" is EXACTLY the Issue 1/2 fixed behavior. Before the fix it was inaccurate
     (bootstrap wrote bare models); after, it's accurate. The bugs were in the IMPLEMENTATION, not the docs. -->

<!-- GOTCHA (the PRIMARY edit is a comment-line transcript, not prose): cli.md lines 208-211 are bash-code-block
     comments of the form `# <scenario>  →  "<stdout>"`. The "Upgraded from v1" line must add the backup line
     WHILE staying one comment line (no newline that breaks the block) and preserving the `→` arrow + quote
     style. The backup prints to STDERR, the upgrade line to STDOUT — the example conflates them (as the other
     scenarios do); keep that convention (do not add stdout/stderr labels unless OPT-1 prose covers it). -->

<!-- GOTCHA (the auto-stage notice and the --edit abort are NOWHERE in either doc): grep confirms zero literal
     matches for "staging all changes", "(1 files)", "empty commit message — aborted" in both files. Issues 5
     and 6 are internal UI-text fixes with NO doc surface here. Do not add example output for them (that would
     be a feature blurb). -->

<!-- GOTCHA (do NOT touch README.md — that's S1; and do NOT touch any .go file): the docs-sync split is
     S1=README.md, S2=how-it-works.md + cli.md. P1.M1-P1.M3 own the .go fixes and have landed them. -->

<!-- GOTCHA (config init --force backup is NOT shown in cli.md either): cli.md line 182 lists the
     `config init --force` command with no output transcript, so there is no stale claim to correct there.
     The backup detail for --force is symmetric with config upgrade; if you take OPT-1, you may optionally
     note --force also backs up, but it is NOT required (the contract flags only "backup created on upgrade"). -->
```

## Implementation Blueprint

### Data models and structure

None. This is a documentation review/edit task. No code, no types, no config.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: RE-VERIFY the 3 review areas in EACH file against the fixed implementation (do not blindly trust research)
  - READ docs/cli.md fully (506 lines) and docs/how-it-works.md fully (402 lines). Re-verify each area against
    the findings.md §2 fixed-behavior table.
  - docs/cli.md:
      (a) token_limit/floor — `grep -niE "token_limit|closed.loop|irreducible|floor|water.fill|FR3j" docs/cli.md`
          → EXPECT 0 hits (not covered; no edit).
      (b1) config init — READ line 172. EXPECT: "EXCEPT for **pi** … whose per-role models are left EMPTY …
          (FR-R5b)" = accurate (Issue 1/2 fixed behavior). No stale bare-pi claim. → no edit.
      (b2) config upgrade — READ lines 203-214. EXPECT: the "Upgraded from v1" example (line 210) shows ONLY
          "Upgraded config at … to version 3." and OMITS the backup line Issue 3 added → STALE. → proceed to Task 2.
      (c) auto-stage notice — `grep -niE "staging all changes|Nothing staged" docs/cli.md` → "Nothing staged"
          appears only as routing/exit-code prose (lines 15-16, 406, 413), NOT the literal "(N files)" notice.
          → no edit (Issue 6 has no surface).
      bonus: --edit abort (Issue 5) — READ line 42. EXPECT descriptive "An empty result aborts (exit 1 …)";
          no literal doubled-prefix output. → no edit.
  - docs/how-it-works.md:
      (a) token_limit — READ line 150 (water-fill) + line 152 (link to configuration.md) + lines 312-316
          (multi-turn FR-T12). EXPECT: line 150 accurate (floor fix preserves cap by erroring → vacuously
          true); floor detail already in configuration.md:167. → no edit (default); OPT-2 optional.
      (b) config/backup — `grep -niE "config init|config upgrade|backup|bootstrap" docs/how-it-works.md`
          → EXPECT 0 hits (not covered). → no edit.
      (c) auto-stage notice — `grep -niE "staging all changes|Nothing staged" docs/how-it-works.md` →
          "nothing staged" only as decompose-trigger prose (lines 49, 69, 179); no literal notice. → no edit.
  - OUTPUT of Task 1: a confirmed per-area finding table. If ANY area diverges from findings.md, STOP and
    re-read the relevant architecture/*.md spec before deciding.

Task 2 (PRIMARY — REQUIRED): EDIT docs/cli.md line 210 — add the backup line to the config-upgrade example
  - LOCATE: docs/cli.md, inside the `### config upgrade` bash code block, the comment line beginning
    `# Upgraded from v1`.
  - CURRENT (exact line):
      # Upgraded from v1  →  "Upgraded config at ~/.config/stagecoach/config.toml to version 3."
  - REPLACE WITH (keep it ONE comment line; preserve the `→` arrow and the quote style; the backup prints to
      stderr before the upgrade line prints to stdout — the example conflates streams like the other scenarios):
      # Upgraded from v1  →  "Backed up previous config to <path>.bak.<ts>" then "Upgraded config at ~/.config/stagecoach/config.toml to version 3."
  - WHY: after Issue 3 (internal/cmd/config.go:190-198), a real upgrade writes a timestamped backup and prints
    "Backed up previous config to <path>.bak.<ts>" (stderr) before "Upgraded config at <path> to version 3."
    (stdout). The example transcript must show both lines to match actual behavior. This corrects a stale
    reference (Mode B), not a feature blurb.
  - PRESERVE: the surrounding comment lines (the "Already at version 3" and "No file" scenarios are UNCHANGED
    — the already-current no-op does NOT back up; the no-file error does NOT back up). Keep the code-block
    fences (```bash … ```) intact.
  - VERIFY after edit: the bash code block still renders; the "Upgraded from v1" line is still one line.

Task 3 (OPT-1 — OPTIONAL, implementer's call): ADD a backup clause to the config-upgrade PROSE (cli.md line 205)
  - DECIDE: is the line-210 example edit (Task 2) enough, or does the prose also need the backup note?
    DEFAULT: skip (Task 2 already surfaces the backup). Take OPT-1 only if you want the prose to state it too.
  - IF TAKEN — LOCATE: docs/cli.md line 205, the sentence ending "…to the current schema version (3) in place."
  - CURRENT clause: `…to the current schema version (3) in place.`
  - REPLACE WITH: `…to the current schema version (3) in place (a timestamped backup of the prior file is written first).`
  - RECORD the decision + one-line rationale (commit message).

Task 4 (OPT-2 — OPTIONAL, default OFF): ADD a floor clause to how-it-works.md line 150
  - DECIDE: should the deep-dive doc surface the sub-floor rejection inline? DEFAULT: NO — line 150 is not
    stale (the floor fix preserves the cap by erroring), and the floor detail is already in
    configuration.md:167 (linked from line 152). Take OPT-2 only if you believe a reader setting
    token_limit=100 needs the hint in the deep-dive (defensible but optional; edges toward new content).
  - IF TAKEN — LOCATE: docs/how-it-works.md line 150, the sentence ending "…to cap the *whole* payload … to a
    token budget." Insert IMMEDIATELY AFTER "to a token budget." and BEFORE " Stagecoach reserves room …":
      (A limit below the irreducible prompt floor is rejected with an error rather than silently broken; see [configuration](configuration.md#built-in-defaults).)
  - RECORD the decision + one-line rationale (commit message).

Task 5: DOCUMENT the review + VALIDATE
  - WRITE the review summary into the changeset's commit message / PR description. Suggested text:
      "Reviewed docs/how-it-works.md + docs/cli.md against the Issues 1-6 bugfix changeset (P1.M4.T1.S2).
       docs/how-it-works.md: no stale references — the token_limit water-fill description (line 150) stays
       accurate after Issue 4 (the floor fix preserves the cap guarantee by erroring on sub-floor limits; the
       floor detail is in configuration.md:167, linked from line 152); config/backup and the auto-stage notice
       are not covered in this file. docs/cli.md: ONE edit — the config upgrade example (line 210) now shows
       the 'Backed up previous config to <path>.bak.<ts>' line Issue 3 added; config init (line 172) was
       already accurate (pi models 'left EMPTY'). The configuration.md:167 floor note and providers.md:125
       commented-pi note already landed with their code-fix commits (Mode A). [OPT-1/OPT-2 decisions noted.]"
  - VALIDATE per §Validation Loop (scope guard + sanity build/test/lint + grep guards).
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN (the review — the core of this task): for each of the 3 areas in each of the 2 files, (1) read the
     exact line(s), (2) compare against the findings.md §2 fixed behavior, (3) classify: ACCURATE (no edit) /
     STALE (correct it) / ABSENT (no edit — not covered). Research pre-classifies: how-it-works.md = all
     ACCURATE/ABSENT; cli.md = one STALE (line 210), rest ACCURATE/ABSENT. The implementer RE-VERIFIES. -->

<!-- PATTERN (the PRIMARY edit): a single comment-line transcript correction inside a ```bash code block. Keep
     the line one physical line; preserve the `→` arrow + quote style; do not disturb the other two scenarios. -->

<!-- PATTERN (scope fence): ONLY docs/how-it-works.md + docs/cli.md. Everything else is owned by another task
     or already landed. -->
```

### Integration Points

```yaml
DOCUMENTATION (the ONLY files this task may touch):
  - docs/cli.md: PRIMARY line-210 edit (required); OPT-1 line-205 prose clause (optional).
  - docs/how-it-works.md: DEFAULT unchanged; OPT-2 line-150 floor clause (optional, default off).
NO README.md edit (P1.M4.T1.S1 owns it).
NO docs/configuration.md or docs/providers.md edit (Mode-A notes ALREADY LANDED — commits fc51dad, 84b5296).
NO source/test edit (P1.M1-P1.M3 own the .go fixes; this task CONSUMES them, reviews against them).
NO PRD/task file edit.
SCOPE FENCES:
  - Touches ONLY: docs/cli.md (required) + optionally docs/how-it-works.md.
  - Does NOT touch: README.md, docs/configuration.md, docs/providers.md, any .go file, go.mod, any PRD/task file.
  - Adds NO code, NO test, NO new doc section (the edits are 1-clause inline corrections, not new sections).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# A markdown edit has no compiler. Validate structure manually + via grep.
# The cli.md edit is inside a ```bash code block — confirm the block is intact (fences + the 3 comment lines).
sed -n '203,214p' docs/cli.md   # eyeball the config upgrade section after the edit
# Expected: the ```bash fence, the 3 `# <scenario> → "<output>"` comment lines (the "Upgraded from v1" line now
# shows the backup line first), and the closing ``` fence. The "Upgraded from v1" line is ONE physical line.

# Markdown lint (the repo has .markdownlint.json; run it if available, else eyeball).
npx markdownlint-cli2 docs/cli.md docs/how-it-works.md 2>/dev/null || echo "(no markdownlint configured — eyeball)"

# Scope guard: ONLY docs/cli.md (+ optionally docs/how-it-works.md).
git status --porcelain
# Expected: ` M docs/cli.md` (PRIMARY ± OPT-1), and optionally ` M docs/how-it-works.md` (OPT-2). NOTHING else.
git diff --name-only | grep -vE '^docs/(how-it-works|cli)\.md$' | grep -q . && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
# Expected: OK.
```

### Level 2: Unit Tests (Component Validation)

```bash
# N/A — this is a documentation task. There are no unit tests for doc content. A markdown edit cannot affect
# `go test`. The "test" is the Task 1 review (re-verify each area).
```

### Level 3: Integration Testing (System Validation)

```bash
# Sanity: a markdown edit cannot break the build/tests, but confirm no collateral (e.g. an accidental stray
# edit to a .go file via a fat-fingered sed).
make build && make test && make lint
# Expected: all green (identical to pre-edit). If RED, you accidentally edited a .go file — revert it.

# Optional cross-check: the cli.md example now matches actual config-upgrade output.
# (Informational only — do NOT run a real upgrade in the dev tree unless you intend to.)
grep -n 'Backed up previous config' internal/cmd/config.go   # confirms the source string the example mirrors
# Expected: 2 hits (config.go:193 for upgrade, config.go:535 for init --force). The example mirrors line 193.
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: the PRIMARY edit landed — cli.md config-upgrade example now shows the backup line.
grep -n 'Backed up previous config' docs/cli.md
# Expected: 1 hit (line 210, the "Upgraded from v1" scenario). (Before edit: 0 hits in config-upgrade context.)

# Guard 2: the other two config-upgrade scenarios are UNCHANGED (already-current + no-file do NOT back up).
grep -n 'already at version 3 (no changes)' docs/cli.md   # present, unchanged
grep -n "no config file at" docs/cli.md                   # present, unchanged
# Expected: both present; neither gained a backup clause.

# Guard 3: cli.md config init (line 172) still accurate + UNCHANGED (pi models "left EMPTY" = fixed behavior).
grep -n 'per-role models are left EMPTY' docs/cli.md
# Expected: 1 hit (line 172). Do NOT edit this line — it is already correct.

# Guard 4: how-it-works.md is UNCHANGED (DEFAULT) OR has only the OPT-2 line-150 clause.
git diff --stat docs/how-it-works.md
# Expected (default): nothing. (OPT-2): "1 file changed, 1 insertion(+), 1 deletion(-)".

# Guard 5: scope — NO README.md, NO configuration.md/providers.md, NO .go file touched.
git diff --name-only | grep -E 'README\.md|docs/configuration\.md|docs/providers\.md|\.go$' && echo "FAIL: out-of-scope" || echo "OK: no README/config/providers/.go edit"
# Expected: OK.

# Guard 6: no literal auto-stage notice or doubled-prefix abort leaked into either doc (Issues 5/6 have no surface).
grep -cE 'staging all changes|\(1 files\)|stagecoach: stagecoach:' docs/how-it-works.md docs/cli.md
# Expected: 0 0.

# Guard 7 (OPT-2 only): the floor clause is present + inline on how-it-works.md line 150.
grep -n 'irreducible prompt floor' docs/how-it-works.md
# Expected (OPT-2): 1 hit (line 150). (default): 0 hits (routed to configuration.md:167, already landed).
```

## Final Validation Checklist

### Technical Validation
- [ ] `make build` + `make test` + `make lint` clean (sanity — a markdown edit cannot break these)
- [ ] `git status --porcelain`: `docs/cli.md` (PRIMARY ± OPT-1) and optionally `docs/how-it-works.md` (OPT-2)
- [ ] NO `README.md`, `docs/configuration.md`, `docs/providers.md`, or any `.go` file touched (grep guard 5)

### Feature Validation (the review)
- [ ] docs/cli.md area (a) token_limit: confirmed NOT covered (0 grep hits) — no edit
- [ ] docs/cli.md area (b1) config init line 172: confirmed accurate ("pi per-role models left EMPTY" = fixed behavior) — no edit
- [ ] docs/cli.md area (b2) config upgrade line 210: STALE corrected — backup line added (PRIMARY edit); scenarios "already-current" + "no-file" unchanged (guard 2)
- [ ] docs/cli.md area (c) auto-stage notice: confirmed ABSENT (grep) — no edit
- [ ] docs/how-it-works.md area (a) line 150: confirmed accurate (floor fix preserves cap by erroring; detail in configuration.md:167) — no edit (OPT-2 decided)
- [ ] docs/how-it-works.md area (b) config/backup: confirmed NOT covered (0 grep hits) — no edit
- [ ] docs/how-it-works.md area (c) auto-stage notice: confirmed ABSENT (grep) — no edit
- [ ] ZERO stale references remain in either file (every claim consistent with the findings.md §2 fixed behavior)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `docs/cli.md` and (optionally) `docs/how-it-works.md`
- [ ] NO edit to `README.md` (P1.M4.T1.S1)
- [ ] NO edit to `docs/configuration.md` or `docs/providers.md` (Mode-A notes ALREADY LANDED — fc51dad, 84b5296)
- [ ] NO edit to any `.go` file (P1.M1-P1.M3 own the fixes)
- [ ] NO new doc section (the edits are 1-clause inline corrections, not feature blurbs)
- [ ] NO PRD/task file edit

### Code Quality & Docs
- [ ] The cli.md `config upgrade` code block still renders (fences intact; "Upgraded from v1" is one line)
- [ ] The review summary is written into the commit/PR message (per the contract: "If no changes needed, note that")
- [ ] OPT-1 and OPT-2 are each DECIDED (taken or not) with a one-line rationale

---

## Anti-Patterns to Avoid

- ❌ Don't touch `docs/configuration.md` or `docs/providers.md`. Their Mode-A notes (floor at line 167;
  commented-pi at line 125) ALREADY LANDED with their code-fix commits (`fc51dad`, `84b5296`). Re-editing
  them is out of scope and collides with landed work. The contract INPUT/OUTPUT names ONLY how-it-works.md +
  cli.md. Verify: `git log --oneline -3 -- docs/configuration.md docs/providers.md`.
- ❌ Don't "fix" how-it-works.md line 150. The token_limit water-fill description is NOT stale: after Issue 4,
  a sub-floor limit ERRORS before assembly, so "caps the whole payload to a token budget" is vacuously true
  (no over-budget payload is ever emitted). The floor detail is already in configuration.md:167 (linked from
  line 152). Adding a floor clause (OPT-2) is optional NEW content, not a stale-reference fix — default off.
- ❌ Don't "correct" cli.md line 172 backwards. "EXCEPT for pi … whose per-role models are left EMPTY" is
  EXACTLY the Issue 1/2 fixed behavior. The bugs were in the IMPLEMENTATION (bootstrap wrote bare models);
  the docs were right. Do not rewrite it.
- ❌ Don't disturb the other two config-upgrade example scenarios. The "Already at version 3" no-op and the
  "No file" error do NOT create a backup (the backup runs only on a real write — `config.go:190` is after the
  `if !changed` return). Only the "Upgraded from v1" line gets the backup clause.
- ❌ Don't add feature blurbs for Issues 5/6. The auto-stage notice ("(N files)") and the --edit empty-message
  abort ("stagecoach: empty commit message — aborted") appear NOWHERE in either doc (grep confirms). They are
  internal UI-text fixes with no doc surface. Adding example output for them is out of scope.
- ❌ Don't touch README.md. That's P1.M4.T1.S1's territory. The docs-sync split is S1=README, S2=how-it-works
  + cli.md.
- ❌ Don't edit any .go file. P1.M1-P1.M3 own and have LANDED the fixes. This task CONSUMES them (reviews the
  docs against the fixed behavior); it does not modify source or tests.
- ❌ Don't invent stale references to "find" work. If the review confirms a file is accurate (how-it-works.md
  is), the correct outcome is a DOCUMENTED no-change review — not a forced edit. A docs-review task with a
  no-change outcome for one of its two files is a SUCCESS, not a failure.
- ❌ Don't break the cli.md bash code block. The config-upgrade example is a fenced ```bash block of `#`-
  comment lines. The "Upgraded from v1" edit must stay ONE physical line (no newline), or the block breaks.
  Preserve the `→` arrow and the quote style. Eyeball the block renders after the edit (Validation Level 1).
- ❌ Don't forget to document the review. The contract says "If no changes needed, note that." An undocumented
  no-change review of how-it-works.md looks like a skipped half. Write the Task 5 summary into the commit/PR
  message.
