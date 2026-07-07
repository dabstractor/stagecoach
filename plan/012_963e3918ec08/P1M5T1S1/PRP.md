---
name: "P1.M5.T1.S1 — plan/ historical rename: VERIFIED the in-scope surface (.md/.go/.toml/.txt excl. 012) is already clean (0 residue); residue only in FORBIDDEN tasks.json. Deliverable = verify + document the no-op + the corrected command as a safety net."
description: |

  ⚠️ HEADLINE RESEARCH FINDING (grep-verified in the live repo): the contract's premise is STALE. A
  case-insensitive grep across plan/ (excluding plan/012) for the contract's FOUR extensions
  (.md/.go/.toml/.txt) returns **ZERO** files with `stagehand` refs — the historical plan text files were
  ALREADY renamed by the earlier M1–M4 repo-wide passes. The corrected find enumerates 716 in-scope files;
  none contain stagehand. plan/001 alone has 147 files with `stagecoach` and 0 in-scope with stagehand.

  The ONLY remaining stagehand residue in plan/ (excluding 012) is **19 `tasks.json` files** — which are
  (a) `.json`, NOT in the contract's `.md/.go/.toml/.txt` list, and (b) **orchestrator-owned and FORBIDDEN**
  to modify (`**/tasks.json` — "owned by orchestrator"). This task MUST NOT touch them.

  CRITICAL CONSEQUENCE: because plan/001–011 text is clean, the ONLY `.md`/`.go`/`.toml`/`.txt` files in
  all of plan/ that still contain `stagehand` are in **plan/012** (the preserve target — 44 files
  documenting the rename). So the contract's UNFIXED command (no plan/012 prune) would find stagehand ONLY
  in plan/012 and CORRUPT it ("stagehand → stagecoach" ⇒ "stagecoach → stagecoach"). The corrected command's
  `-prune` is therefore the difference between a correct no-op and active corruption.

  DELIVERABLE = VERIFY + DOCUMENT (not a 622-file rename):
    1. RUN the corrected command (below) — it renames 0 in-scope files (the `xargs -r` makes the empty-grep
       case a clean no-op). This CONFIRMS the surface is clean.
    2. ASSERT the primary gate: zero stagehand residue in plan/ .md/.go/.toml/.txt excluding 012.
    3. ASSERT plan/012 preservation: it STILL contains stagehand refs (the prune worked).
    4. DOCUMENT the no-op + FLAG the 19 forbidden tasks.json files for the orchestrator (P1.M5.T2.S1's
       whole-repo audit will see them; renaming tasks.json is the orchestrator's call, NOT this task's).

  THE CORRECTED COMMAND (Linux/GNU — the contract's sketch + 3 fixes; it is the verification tool + safety
  net if the state differs at runtime):
    find plan -path plan/012_963e3918ec08 -prune -o -type f \
      \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
      | xargs grep -li 'stagehand' 2>/dev/null \
      | xargs -r sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g'

  CONTRACT (item_description §1–§5; PRD h2.30; critical_findings.md F8):
    1. RESEARCH NOTE: "~622 tracked files across plan/001_*–plan/011_* with stagehand references." ← STALE:
       verified 0 in-scope residue today (renamed by M1–M4). F8 describes the PRE-rename state.
    3. LOGIC: the bulk sed (the corrected form above). CAUTION: "do NOT modify plan/012_963e3918ec08/
       (documents the rename, references both names)."
    4. OUTPUT: "All plan/ files (except 012_*) use 'stagecoach' throughout." ← ACHIEVED for the in-scope
       (.md/.go/.toml/.txt) surface; the 19 tasks.json are out of scope (forbidden) — see Anti-Patterns.
    5. DOCS: "none — historical artifacts."

  SCOPE BOUNDARY (do NOT touch):
    - plan/012_963e3918ec08/ — the CURRENT changeset; documents the rename (both names); PRUNED.
    - **tasks.json anywhere** — orchestrator-owned, FORBIDDEN (the 19 residue files are these).
    - All non-plan/ surfaces (renamed M1–M4): Go source, Makefile, .goreleaser.yaml, .github/, README.md,
      docs/, providers/, FUTURE_SPEC.md, go.mod/go.sum.
    - PRD.md / prd_snapshot.md (orchestrator-owned).

  SUCCESS: primary gate (zero in-scope stagehand residue outside 012) GREEN; plan/012 STILL has stagehand
  refs (preserved); `go build ./... && go test ./...` green (plan/ not compiled); tasks.json UNTOUCHED; the
  no-op + the tasks.json flag are recorded in the implementation summary.

---

## Goal

**Feature Goal**: Verify (and, only if necessary, complete) the stagehand→stagecoach rename on the plan/
historical surface — the contract's `.md/.go/.toml/.txt` files under plan/001_*–plan/011_*. RESEARCH
VERIFIED the surface is ALREADY clean (0 in-scope files contain stagehand; the earlier M1–M4 passes renamed
them). So the task's actual work is: run the corrected command as VERIFICATION (it renames 0 files, via
`xargs -r`), assert the zero-residue + plan/012-preservation gates, and document the no-op — while NOT
touching the 19 forbidden `tasks.json` residue files.

**Deliverable**: a verified-clean in-scope surface (the corrected command run as verification; 0 files
renamed, confirming the clean state) + a documented record of the no-op + a flagged list of the 19
orchestrator-owned `tasks.json` files that still contain stagehand (out of scope; for the orchestrator).

**Success Definition**:
- Primary gate: `find plan -path plan/012_963e3918ec08 -prune -o -type f \( -name '*.md' -o -name '*.go' -o
  -name '*.toml' -o -name '*.txt' \) -print | xargs grep -li 'stagehand'` → **no output** (zero residue).
- Preservation: `grep -rli 'stagehand' plan/012_963e3918ec08/ | wc -l` → **> 0** (plan/012 intact).
- `go build ./... && go test ./...` green (historical plan/ not compiled; regression check).
- `tasks.json` files UNTOUCHED (git status shows no tasks.json changes).
- The implementation summary records: in-scope surface clean (0 residue); 19 tasks.json flagged as
  orchestrator-owned/out-of-scope.

## User Persona

**Target User**: the maintainer (and the orchestrator's final audit, P1.M5.T2.S1) confirming the rename is
complete across the whole repo. This task certifies the plan/ historical TEXT surface is clean and flags the
one surface (tasks.json) that remains — which is the orchestrator's to handle.

**Use Case**: P1.M5.T2.S1 runs the whole-repo `grep -ri stagehand`; it will report the 19 tasks.json files.
This task's documentation explains WHY they are not this task's responsibility (forbidden + wrong
extension) so the audit doesn't mis-attribute them.

## Why

- **It IS P1.M5.T1.S1.** The contract asks to sweep plan/ historical artifacts. The sweep is performed
  (via the corrected command); the verified result is that the in-scope surface is already clean.
- **Prevents plan/012 corruption.** Because plan/012 is now the ONLY in-scope `.md` surface with stagehand,
  running the contract's UNFIXED command would corrupt it. The corrected command's prune makes the
  verification safe. Documenting this protects the rename record.
- **Surfaces the tasks.json residue honestly.** A naive "0 files renamed, task done" would hide the 19
  tasks.json files that P1.M5.T2.S1 will catch. Flagging them now (as forbidden/orchestrator-owned) gives
  the orchestrator the information to decide.
- **Zero runtime risk.** Historical plan/ files are never compiled/executed/shipped (critical_findings F8).
  The verification and any safety-net rename cannot affect behavior or tests.

## What

1. **Verify** the in-scope surface is clean by running the corrected command (it will rename 0 files — the
   `xargs -r` makes the empty-grep case a clean no-op). This is the primary gate.
2. **Assert** plan/012 is preserved (still has stagehand refs — the prune worked).
3. **Document** the no-op and **flag** the 19 `tasks.json` residue files (FORBIDDEN — do not touch).
4. **Do NOT** modify tasks.json, plan/012, or any non-plan/ surface.

The corrected command differs from the contract's sketch in three ways (each fixing a real gap):

| gap | contract (unsafe) | corrected |
|-----|-------------------|-----------|
| plan/012 exclusion | absent — would corrupt plan/012 (the ONLY remaining in-scope stagehand .md surface) | `-path plan/012_963e3918ec08 -prune -o` |
| grep case | `grep -l 'stagehand'` (case-sensitive; misses Stagehand/STAGEHAND) | `grep -li 'stagehand'` |
| empty input | `xargs sed -i …` (errors on empty — and it IS empty today) | `xargs -r sed -i …` (GNU no-run-if-empty) |

### Success Criteria

- [ ] The corrected command (plan/012 prune + `grep -li` + `xargs -r`) is RUN; the contract's literal
      command was NOT run verbatim (no prune ⇒ plan/012 corruption).
- [ ] Primary gate: zero stagehand residue in plan/ .md/.go/.toml/.txt excluding 012 (`grep -li` → no output).
- [ ] plan/012 preserved: `grep -rli 'stagehand' plan/012_963e3918ec08/ | wc -l` → > 0.
- [ ] `tasks.json` UNTOUCHED (`git status` shows no tasks.json changes).
- [ ] `go build ./... && go test ./...` green.
- [ ] Implementation summary records: in-scope surface clean (0 residue, ~716 files verified); 19
      `tasks.json` files flagged as orchestrator-owned/out-of-scope (FORBIDDEN).

## All Needed Context

### Context Completeness Check

_Pass._ A developer with no prior repo knowledge can implement this from: the verified finding (in-scope
surface already clean; residue only in forbidden tasks.json), the corrected command (verbatim) + its 3
gap-fixes, the two assertion gates (zero residue + plan/012 preserved), the FORBIDDEN tasks.json rule, and
the scope boundary. No Go/provider/git-internals knowledge required.

### Documentation & References

```yaml
# MUST READ — THE decisive doc (the verified state + the 3 gaps + the corrected command)
- docfile: plan/012_963e3918ec08/P1M5T1S1/research/findings.md
  why: §1 the VERIFIED state (716 in-scope files enumerated; ZERO with stagehand — already renamed by
       M1–M4; the contract's ~622 is stale); §2 the ONLY residue is 19 tasks.json (FORBIDDEN + .json not in
       the 4 extensions); §3 plan/012 is now the ONLY in-scope .md stagehand surface ⇒ the prune is the
       difference between a correct no-op and corruption; §4 the 3 gaps + the corrected command; §5 safety
       analysis; §6 F8 + rename_surface_map verified present; §7 validation; §8 the real deliverable.
  critical: §1 (it's a verified no-op on the in-scope surface — don't expect 622 renames), §2 (tasks.json is
       FORBIDDEN — do not touch the 19 residue files), §3 (the prune is mandatory — plan/012 is the only
       remaining in-scope stagehand .md surface).

# MUST READ — the contract's RESEARCH NOTE source (verified present in the live repo)
- file: plan/012_963e3918ec08/architecture/critical_findings.md   (READ ONLY — §F8)
  section: "## F8: Historical plan/ files are tracked but non-functional" — "plan/001_* through plan/011_*
       contain previous task breakdowns, PRP files, architecture research … reference 'stagehand' extensively
       but are never compiled, never executed, never shipped … rename for completeness, lowest priority."
  why: F8 describes the PRE-rename state. This task VERIFIES the rename is now complete on the in-scope
       surface (.md/.go/.toml/.txt). The "extensively" is historical — grep confirms 0 in-scope residue today.

# MUST READ — the rename surface map (Layer 5 + the whole-repo zero-residue gate)
- file: plan/012_963e3918ec08/architecture/rename_surface_map.md   (READ ONLY)
  section: "## Layer 5: Documentation" (5.1 README, 5.2 docs/, 5.3 providers, 5.4 FUTURE_SPEC) +
       "Verification Gates" gate 5: `grep -ri 'stagehand' … | wc -l == 0`.
  why: plan/ historical is the implicit remainder of Layer 5; this task sweeps it. Gate 5 is the whole-repo
       audit (P1.M5.T2.S1) — it WILL report the 19 tasks.json; this task flags them so the audit isn't
       mis-attributed.

# MUST READ — the preserve target (do NOT edit; verify it survives)
- file: plan/012_963e3918ec08/   (READ ONLY — the CURRENT changeset; PRUNED by the corrected command)
  section: the PRP/research/architecture files. They DOCUMENT the rename and reference both names ("rename
       stagehand.* git-config keys → stagecoach.*", "part of the stagehand→stagecoach project rename", etc.).
  why: confirms why plan/012 is EXCLUDED. It is now the ONLY in-scope (.md/.go/.toml/.txt) surface with
       stagehand — so WITHOUT the prune, the command would corrupt ONLY it. The prune is mandatory.
  critical: after the run, plan/012 MUST still contain stagehand refs — that proves the prune worked.

# READ — the governing directive
- docfile: PRD.md (heading h2.30 — in context as selected_prd_content)
  section: "## Note: this project was originally named 'stagehand' and has been renamed. All references to
       'stagehand' must be replaced with 'stagecoach'."
  why: the project-wide rename directive. This task certifies the plan/ historical TEXT surface complies.

# READ — the sibling rename PRP (the pattern + the FUTURE_SPEC surface, already done)
- docfile: plan/012_963e3918ec08/P1M4T2S2/PRP.md   (FUTURE_SPEC.md rename — preceding surface)
  why: confirms the rename pattern (case-variant sed + zero-residue grep + go-test-as-regression-check) and
       that FUTURE_SPEC.md (Layer 5.4) is a separate, already-done surface. The historical plan/ surface
       uses the same three sed arms (STAGEHAND_* env docs appear in PRPs).
```

### Current Codebase tree (relevant slice)

```bash
plan/
  001_*/ … 011_*/        # *** VERIFIED CLEAN *** — historical changesets; 716 in-scope files (.md/.go/.toml/.txt),
                         #   ZERO with stagehand (already renamed by M1–M4). The corrected command confirms this.
  012_963e3918ec08/      # *** READ ONLY — PRUNED *** — the CURRENT changeset; 44 files with stagehand (the rename
                         #   docs). The ONLY in-scope .md surface with stagehand ⇒ prune is mandatory.
  **/tasks.json          # *** FORBIDDEN *** — 19 of these (under plan/001–011) still have stagehand. Orchestrator-
                         #   owned; .json (not in the 4 extensions). DO NOT TOUCH. Flag for the orchestrator.
# Non-plan/ surfaces (already renamed M1–M4; UNCHANGED):
cmd/stagecoach/ pkg/stagecoach/ internal/ docs/ providers/ README.md FUTURE_SPEC.md Makefile .goreleaser.yaml .github/
```

### Desired Codebase tree with files to be added/changed

```bash
# EXPECTED: NO file changes (verified no-op). The corrected command renames 0 in-scope files.
# IF a straggler is found at runtime (state drift), the corrected command renames just those files (safety net).
# tasks.json: UNCHANGED (forbidden). plan/012: UNCHANGED (pruned). Non-plan/ surfaces: UNCHANGED.
```

### Known Gotchas of our codebase & Library Quirks

```bash
# CRITICAL (the surface is ALREADY CLEAN — expect a NO-OP): grep-verified 0 in-scope (.md/.go/.toml/.txt)
#   stagehand refs in plan/001–011. The corrected command's grep leg returns EMPTY. The `xargs -r`
#   (--no-run-if-empty) is ESSENTIAL: without it, `sed -i` with no file args errors or reads stdin. With
#   `-r`, the empty case is a clean no-op. Do NOT interpret "0 files renamed" as a failure — it is the
#   VERIFIED correct outcome (the surface is clean).

# CRITICAL (plan/012 is now the ONLY in-scope .md stagehand surface): because plan/001–011 text is clean,
#   an UNPRUNED command would find stagehand ONLY in plan/012 and CORRUPT it ("stagehand → stagecoach" ⇒
#   "stagecoach → stagecoach"). The `-path plan/012_963e3918ec08 -prune -o` is MANDATORY. After the run,
#   ASSERT plan/012 still has stagehand refs (Success Criteria).

# CRITICAL (tasks.json is FORBIDDEN): the 19 remaining stagehand-ref files are all tasks.json —
#   orchestrator-owned ("NEVER MODIFY: **/tasks.json") AND .json (not in the .md/.go/.toml/.txt list). DO
#   NOT rename them. DO NOT add '*.json' to the find. Flag them for the orchestrator / P1.M5.T2.S1.

# CRITICAL (do NOT run the contract's literal command): it lacks the prune (⇒ corrupts plan/012), uses
#   case-sensitive grep (⇒ misses Stagehand/STAGEHAND-only stragglers), and has no empty-input guard
#   (⇒ errors on today's empty grep). Run the CORRECTED command.

# GOTCHA (THREE sed arms): use all three (stagehand/Stagehand/STAGEHAND). Case-disjoint + order-safe.
#   Historical PRPs document STAGEHAND_* env vars; a straggler could be any case.

# GOTCHA (every stagehand substring is a desired rename — no compound-token preservation): .stagehand.toml,
#   .stagehandignore, stagehand.* git keys, STAGEHAND_* env, github.com/dustin/stagehand import path, the
#   binary/prose name — all → stagecoach. `commit-pi` (originating tool) is untouched (no stagehand substring).

# GOTCHA (find -o grouping): once `-path … -prune -o` + explicit `-print` are added, the -name conditions
#   MUST be grouped `\( … \)` (also portable to BSD find). The ungrouped form happens to work on GNU find
#   but is not robust once prune is in play.

# GOTCHA (macOS BSD sed, local runs): `sed -i ''` (empty backup-ext arg). BSD xargs has no -r; on an empty
#   grep (today's reality) BSD xargs+sed would misbehave — run on Linux/CI, or pre-check the count is >0
#   before the sed pipe on a Mac. The CI is GNU/Linux.

# GOTCHA (historical plan/ is NOT compiled): `go build ./...` builds only the module's packages; plan/ .go
#   files are research/examples, not in the module. Build/test is a regression check, not a feature test.
```

## Implementation Blueprint

### Data models and structure

_None._ A verification (and at most a mechanical textual rename of any straggler). No data models, no code.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: VERIFY the in-scope surface state (READ/count — this frames the whole task)
  - RUN the primary gate (the corrected command's grep leg):
        find plan -path plan/012_963e3918ec08 -prune -o -type f \
          \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
          | xargs grep -li 'stagehand' 2>/dev/null | tee /tmp/stagehand_inscope.txt | wc -l
    EXPECTED: 0 (research-verified — the surface is already clean). Record the count.
  - IF count == 0 (expected): the in-scope surface is CLEAN. Proceed to Task 3 (document the no-op) — SKIP
      the bulk rename (Task 2 becomes the safety-net confirmation only). This is the verified-correct path.
  - IF count > 0 (state drift — a straggler re-appeared): proceed to Task 2 to rename exactly those files.
  - ALSO capture the forbidden-residue inventory (for the flag):
        grep -rli 'stagehand' plan/ 2>/dev/null | grep -vE '^plan/012_963e3918ec08/' | tee /tmp/stagehand_all.txt
    EXPECTED: ~19 paths, ALL `tasks.json`. (Confirms the residue is orchestrator-owned, not in-scope.)
  - WHY: the contract's "~622" is stale; Task 1 establishes the TRUE state before any mutation. Renaming
      nothing is the correct outcome when (not if) the count is 0.

Task 2: (SAFETY NET — only if Task 1 found in-scope stragglers) RUN the corrected bulk rename
  - PRECONDITION: Task 1's in-scope count > 0 (unexpected). If 0, SKIP this task (go to Task 3).
  - COMMAND (Linux/GNU):
        find plan -path plan/012_963e3918ec08 -prune -o -type f \
          \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
          | xargs grep -li 'stagehand' 2>/dev/null \
          | xargs -r sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g'
  - WHY: if a straggler exists (state drift), this renames exactly it, safely (prune protects plan/012;
      grep -li catches all case variants; xargs -r handles empty). This is the "completeness" the contract
      asks for, scoped to the actual stragglers.
  - GOTCHA: do NOT drop the prune (plan/012 is the only in-scope stagehand .md surface — it would corrupt).
      Do NOT add '*.json' (tasks.json is forbidden). Do NOT touch non-plan/ surfaces.

Task 3: VERIFY the gates + DOCUMENT the outcome + FLAG the forbidden residue (always run)
  - PRIMARY GATE (zero in-scope residue):
        find plan -path plan/012_963e3918ec08 -prune -o -type f \
          \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
          | xargs grep -li 'stagehand' 2>/dev/null | wc -l
    Expected: 0. (The deliverable.)
  - PRESERVATION (plan/012 intact — the prune worked):
        grep -rli 'stagehand' plan/012_963e3918ec08/ 2>/dev/null | wc -l
    Expected: > 0 (the rename docs survive).
  - FORBIDDEN-FILE AUDIT (the 19 tasks.json — DO NOT TOUCH, just flag):
        grep -rli 'stagehand' plan/ 2>/dev/null | grep -vE '^plan/012_963e3918ec08/' | grep -c 'tasks.json'
    Expected: ~19. RECORD this list in the implementation summary as "orchestrator-owned tasks.json residue,
      out of scope (FORBIDDEN); flagged for the orchestrator / P1.M5.T2.S1."
  - REGRESSION (plan/ not compiled):
        go build ./... && go test ./...
    Expected: green, unchanged.
  - SCOPE:
        git status --porcelain plan/      # expect: EMPTY (verified no-op) OR only stragglers Task 2 renamed
        git status --porcelain | grep -c 'tasks.json'   # expect: 0 (tasks.json untouched)
  - DOCUMENT in the implementation summary:
      (a) The in-scope plan/ surface (.md/.go/.toml/.txt, excluding 012) is CLEAN — 0 stagehand residue
          across ~716 files (verified; already renamed by M1–M4).
      (b) plan/012_963e3918ec08 PRESERVED (still references both names — the rename documentation).
      (c) 19 `tasks.json` files under plan/001–011 still contain stagehand; they are orchestrator-owned
          (FORBIDDEN) and .json (not in the contract's extension list) — out of scope for this task; flagged
          for the orchestrator. (P1.M5.T2.S1's whole-repo audit will see them.)
```

### Implementation Patterns & Key Details

```bash
# PATTERN: the corrected verification/rename command (the contract's sketch + 3 fixes). Run it; expect a
# no-op today (the surface is clean). The `xargs -r` makes the empty case safe.
find plan -path plan/012_963e3918ec08 -prune -o -type f \
  \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
  | xargs grep -li 'stagehand' 2>/dev/null \
  | xargs -r sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g'

# PATTERN: the zero-residue gate (primary) — same find/grep, expect no output.
# PATTERN: the plan/012 preservation gate — grep -rli 'stagehand' plan/012_963e3918ec08/ → expect > 0.

# CRITICAL: the surface is ALREADY CLEAN (verified 0 in-scope residue). "0 files renamed" is the CORRECT
#   outcome, not a failure. The contract's ~622 figure is stale (described the pre-rename state; F8).
# CRITICAL: plan/012 is the ONLY in-scope .md stagehand surface now — the prune is mandatory (an unpruned
#   command corrupts ONLY plan/012).
# CRITICAL: tasks.json (19 residue files) is FORBIDDEN — do not touch, do not add '*.json'.

# GOTCHA: three sed arms (case-disjoint, order-safe) — a straggler could be any case.
# GOTCHA: every stagehand substring is a desired rename; commit-pi untouched.
# GOTCHA: plan/ is not compiled — go build/test are regression checks.
```

### Integration Points

```yaml
PLAN.HISTORICAL (plan/001_*/…/plan/011_* — the verified-clean surface):
  - state: "716 in-scope files (.md/.go/.toml/.txt); ZERO with stagehand (already renamed by M1–M4)."
  - action: "verify (Task 1/3); rename only IF a straggler appears (Task 2 safety net)."
  - runtime: "NONE — never compiled/executed/shipped (F8). go build/test unaffected."

PLAN.012 (plan/012_963e3918ec08 — PRUNED, preserved):
  - state: "44 files reference stagehand (the rename docs, both names)."
  - action: "PRUNE — do not touch. Assert it survives (still has stagehand refs)."

TASKS.JSON (FORBIDDEN — the 19 residue files):
  - state: "19 tasks.json files under plan/001–011 still contain stagehand."
  - action: "DO NOT MODIFY. Orchestrator-owned. Flag for the orchestrator / P1.M5.T2.S1."

NON-PLAN.SURFACES (unchanged — already renamed M1–M4):
  - Go source / Makefile / .goreleaser.yaml / .github / README.md / docs/ / providers/ / FUTURE_SPEC.md /
    go.mod / go.sum: "UNCHANGED. Do NOT include them in the find."

GO.MODULE / BUILD / TEST: change NONE. Historical plan/ is not in the module build.

DOWNSTREAM (P1.M5.T2.S1 — whole-repo zero-residue audit): this task certifies the plan/ TEXT surface is
      clean and explains the 19 tasks.json hits the audit WILL see (forbidden, orchestrator-owned) so they
      are not mis-attributed to a failure of this task.
```

## Validation Loop

### Level 1: The gates (the core verification)

```bash
# PRIMARY GATE — zero in-scope residue (expect 0 / no output):
find plan -path plan/012_963e3918ec08 -prune -o -type f \
  \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
  | xargs grep -li 'stagehand' 2>/dev/null | wc -l
# Expected: 0.

# PRESERVATION — plan/012 still has stagehand refs (expect > 0):
grep -rli 'stagehand' plan/012_963e3918ec08/ 2>/dev/null | wc -l
# Expected: > 0. (If 0, the prune FAILED and plan/012 was corrupted — restore from git + fix the prune.)

# FORBIDDEN-FILE FLAG — the tasks.json residue (expect ~19; DO NOT TOUCH):
grep -rli 'stagehand' plan/ 2>/dev/null | grep -vE '^plan/012_963e3918ec08/' | grep -c 'tasks.json'
# Expected: ~19. Record the list in the summary; do NOT modify them.
```

### Level 2: Scope (verified no-op ⇒ no changes; tasks.json untouched)

```bash
git status --porcelain plan/                       # expect: EMPTY (no-op) OR only Task-2 stragglers
git status --porcelain | grep -c 'tasks.json'      # expect: 0 (tasks.json untouched)
git status --porcelain | grep -vE ' ^.plan/' | head # expect: EMPTY (no non-plan/ file touched)
# (if git-tracked) plan/012 byte-unchanged:
git diff --quiet plan/012_963e3918ec08/ 2>/dev/null && echo "plan/012 UNCHANGED (expected)"
```

### Level 3: Build/test regression check (plan/ not compiled)

```bash
go build ./...     # Expect clean.
go test ./...      # Expect all PASS — historical plan/ is not imported by tests.
```

### Level 4: Whole-repo residue context (informational — for the downstream audit)

```bash
# The whole-repo stagehand residue the P1.M5.T2.S1 audit will see (this task's documentation explains it):
grep -rli 'stagehand' . --include='*.md' --include='*.go' --include='*.toml' --include='*.txt' \
  --include='*.yaml' --include='*.yml' 2>/dev/null | grep -vE '/(plan/012_963e3918ec08|\.git/)/'
# Expected: empty (the in-scope TEXT surface is clean).
# NOTE: tasks.json is NOT in the --include list above; the audit's broader grep will catch it — that is the
# orchestrator's concern (flagged), not a failure of this task.
```

## Final Validation Checklist

### Technical Validation
- [ ] The corrected command (plan/012 prune + `grep -li` + `xargs -r`) was used for verification; the
      contract's literal command was NOT run verbatim.
- [ ] Primary gate: 0 in-scope stagehand residue (plan/ .md/.go/.toml/.txt excluding 012).
- [ ] plan/012 preserved: still contains stagehand refs (> 0 files).
- [ ] `go build ./... && go test ./...` green (plan/ not compiled; regression check).
- [ ] `tasks.json` UNTOUCHED; non-plan/ surfaces byte-unchanged.

### Feature Validation
- [ ] The in-scope plan/ historical surface (.md/.go/.toml/.txt, excluding 012) is verified clean (0 residue).
- [ ] plan/012_963e3918ec08 STILL references stagehand (rename documentation intact).
- [ ] The 19 `tasks.json` residue files are FLAGGED (not modified) as orchestrator-owned/out-of-scope.

### Code Quality Validation
- [ ] The contract's 3 gaps were addressed in the corrected command (prune / grep -li / xargs -r).
- [ ] No mutation beyond any verified straggler (a bulk `s///g` is mechanical; no rewording).
- [ ] Scope respected: tasks.json forbidden; plan/012 pruned; non-plan/ surfaces untouched.
- [ ] Anti-patterns avoided (see below).

### Documentation
- [ ] [Mode B] historical artifacts — no user-facing surface.
- [ ] Implementation summary records: in-scope surface clean (verified no-op); plan/012 preserved; 19
      tasks.json flagged as orchestrator-owned (FORBIDDEN) for P1.M5.T2.S1.

---

## Anti-Patterns to Avoid

- ❌ **Don't expect 622 renames.** Grep-verified: 0 in-scope (.md/.go/.toml/.txt) stagehand refs remain
  (already renamed by M1–M4). "0 files renamed" is the CORRECT outcome. The contract's ~622 is stale (F8
  describes the pre-rename state). (§1)
- ❌ **Don't run the contract's literal command.** No prune ⇒ corrupts plan/012 (now the ONLY in-scope .md
  stagehand surface). Case-sensitive grep ⇒ misses capitalized stragglers. No `xargs -r` ⇒ errors on the
  empty grep (today's reality). Run the CORRECTED command. (§4)
- ❌ **Don't modify tasks.json.** The 19 residue files are `tasks.json` — orchestrator-owned ("NEVER MODIFY:
  **/tasks.json") AND .json (not in the contract's extension list). Flag them; do not touch; do not add
  '*.json' to the find. (§2)
- ❌ **Don't rename plan/012.** It documents the rename with both names; the prune excludes it. Assert it
  survives. (§3)
- ❌ **Don't hide the no-op.** A bare "done, 0 changes" mis-leads P1.M5.T2.S1 (its whole-repo grep WILL find
  the 19 tasks.json). Document the no-op AND flag the tasks.json residue so the audit isn't mis-attributed. (§8)
- ❌ **Don't widen to non-plan/ surfaces.** docs/, providers/, README.md, FUTURE_SPEC.md, Go source, build
  files — all already renamed M1–M4. The find targets `plan/` only. (scope)
- ❌ **Don't use only two sed arms.** Use all three (a straggler could be any case; STAGEHAND_* appears in
  historical PRPs). Case-disjoint, order-safe. (gotcha)
- ❌ **Don't expect a test to validate this.** Historical plan/ is not compiled/imported. Validation is the
  grep gates. `go test ./...` is a regression check only. (gotcha)
