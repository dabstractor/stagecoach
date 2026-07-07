# P1.M5.T1.S1 — Research Findings
## plan/ historical rename — VERIFIED NO-OP on the in-scope surface (already renamed); residue only in forbidden tasks.json

---

## 0. Task contract (verbatim summary)

Mode B mechanical rename. The plan/ directory's PRIOR changesets (plan/001_* – plan/011_*) allegedly
contain ~622 tracked files with `stagehand` references (RESEARCH NOTE citing critical_findings.md F8).
LOGIC: bulk sed `s/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g` across
.md/.go/.toml/.txt. CAUTION: do NOT modify plan/012_963e3918ec08 (documents the rename, references both
names). OUTPUT: all plan/ files except 012_* use 'stagecoach' throughout.

---

## 1. ⚠️ HEADLINE FINDING — the in-scope surface is ALREADY CLEAN (verified)

A case-insensitive recursive grep across plan/ (excluding plan/012) for the contract's FOUR extensions
(.md/.go/.toml/.txt) returns **ZERO** files with stagehand refs:

```
$ find plan -path plan/012_963e3918ec08 -prune -o -type f \
    \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
  | xargs grep -li 'stagehand' 2>/dev/null | wc -l
0
```
- The corrected `find` enumerates **716** in-scope files (≈ the contract's "~622") — the find WORKS; the
  716 files simply no longer contain `stagehand`.
- plan/001 alone has **147** files containing `stagecoach` (already renamed) and only **3** with stagehand
  — and those 3 are `tasks.json` (see §2).
- The historical plan/ text files (.md/.go/.toml/.txt) were renamed by EARLIER rename layers (M1–M4 ran
  repo-wide text passes that swept plan/ too). The contract's "~622 files with stagehand references" is
  STALE — it described the pre-rename state.

**Conclusion**: the contract's bulk-rename command, run on the actual current file set, renames **ZERO**
in-scope files. The task is a **verified no-op on the .md/.go/.toml/.txt surface**.

---

## 2. The ONLY remaining stagehand residue is in orchestrator-owned `tasks.json` (FORBIDDEN)

Across ALL of plan/ excluding 012, exactly **19** files still contain `stagehand` — and **all 19 are
`tasks.json`** (`.json`, not one of the contract's 4 extensions):

```
$ grep -rli 'stagehand' plan/ | grep -vE '^plan/012_963e3918ec08/' | sed 's|.*/||' | sort | uniq -c
     19 tasks.json
$ grep -rli 'stagehand' plan/ | grep -vE '^plan/012_963e3918ec08/' | grep -ciE '\.(md|go|toml|txt)$'
0
```

`tasks.json` is:
1. `.json` — NOT in the contract's `.md/.go/.toml/.txt` list (the contract's own command would skip it).
2. **Orchestrator-owned and FORBIDDEN** — the task's FORBIDDEN OPERATIONS: "NEVER MODIFY: `**/tasks.json`
   - Any tasks.json file anywhere (owned by orchestrator)."

→ This task MUST NOT touch the 19 tasks.json files. They are out of scope by BOTH the contract's extension
list AND the FORBIDDEN OPERATIONS. **Flag for the orchestrator** (P1.M5.T2.S1's whole-repo audit will see
them; renaming tasks.json is the orchestrator's call, not this task's).

---

## 3. plan/012 is the preserve target (VERIFIED) — and now the ONLY .md stagehand surface

plan/012_963e3918ec08 is the CURRENT changeset (this rename project). **44** files there contain stagehand
(deliberately — they document the rename: "rename stagehand.* git-config keys → stagecoach.*", "part of the
stagehand→stagecoach project rename", "github.com/dustin/stagehand … 404 occurrences"). These MUST be
preserved.

**Critical consequence of §1 + §3**: since plan/001–011 text files are clean, the ONLY `.md`/`.go`/`.toml`/
`.txt` files in all of plan/ that still contain `stagehand` are in **plan/012**. Therefore:
- The contract's UNFIXED command (no plan/012 prune) would find stagehand refs ONLY in plan/012 and RENAME
  THEM — **corrupting the rename documentation** ("stagehand → stagecoach" ⇒ "stagecoach → stagecoach").
- The CORRECTED command (with the prune) finds ZERO files and renames NOTHING — which is correct.

So the plan/012 prune is not just a nicety — it is the difference between a correct no-op and active
corruption, because plan/012 is now the sole remaining in-scope stagehand surface.

---

## 4. The 3 gaps in the contract's literal command (still documented — they are the verification tool)

Even though the command renames 0 files today, it is the task's VERIFICATION gate and a SAFETY NET if the
state differs at implementation time (a different branch, a partial revert, a re-added file). The corrected
form fixes three gaps in the contract's sketch:

| gap | contract (unsafe) | corrected |
|-----|-------------------|-----------|
| plan/012 exclusion | (absent — would corrupt plan/012, the ONLY remaining in-scope stagehand surface) | `-path plan/012_963e3918ec08 -prune -o` |
| grep case | `grep -l 'stagehand'` (case-sensitive; misses Stagehand/STAGEHAND) | `grep -li 'stagehand'` |
| empty input | `xargs sed -i …` (errors on empty — and it WILL be empty today) | `xargs -r sed -i …` |

The corrected command (Linux/GNU):
```
find plan -path plan/012_963e3918ec08 -prune -o -type f \
  \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
  | xargs grep -li 'stagehand' 2>/dev/null \
  | xargs -r sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g'
```
(The `xargs -r` is ESSENTIAL today: the grep returns nothing, so without `-r` the `sed -i` would error or
read stdin. With `-r` it is a clean no-op.)

---

## 5. Why the blanket sed is SAFE whenever it DOES find files (no compound-token risk)

Every `stagehand` substring in this codebase is a token that SHOULD be renamed: `.stagehand.toml`→
`.stagecoach.toml`; `.stagehandignore`→`.stagecoachignore`; `stagehand.no_verify`/`noVerify`→`stagecoach.*`;
`STAGEHAND_*` env→`STAGECOACH_*`; `github.com/dustin/stagehand`→`stagecoach` (contract: "the sed handles
those too"). `commit-pi` (the originating tool) has NO stagehand substring and is untouched. The three sed
arms are case-disjoint + order-safe (arm 1 lowercase `s`; arm 2 `Stagehand` needs lowercase `tagehand` so
it misses `STAGEHAND`; arm 3 all-caps). No arm re-creates another's pattern.

---

## 6. The plan/012 architecture docs (the contract's RESEARCH NOTE references — verified present)

The contract cites critical_findings.md F8. Verified in the live repo (plan/012_963e3918ec08/architecture/):
- **critical_findings.md §F8** ("Historical plan/ files are tracked but non-functional"): "The plan/001_*
  through plan/011_* directories contain previous task breakdowns, PRP files, and architecture research …
  They reference 'stagehand' extensively but are never compiled, never executed, and never shipped …
  should be renamed for completeness but are the lowest priority." → describes the PRE-rename state; this
  task verifies the rename is now complete on the in-scope surface.
- **rename_surface_map.md** Layer 5 (Documentation): lists 5.1 README, 5.2 docs/, 5.3 providers, 5.4
  FUTURE_SPEC; verification gate 5 = whole-repo `grep -ri stagehand … == 0`. plan/ historical is the
  implicit remainder; this task is its sweep (now verified clean on .md/.go/.toml/.txt).

---

## 7. Validation (the corrected command IS the verification; expect a clean no-op)

```bash
# PRIMARY GATE — zero in-scope residue (the corrected command's grep leg, expect 0):
find plan -path plan/012_963e3918ec08 -prune -o -type f \
  \( -name '*.md' -o -name '*.go' -o -name '*.toml' -o -name '*.txt' \) -print \
  | xargs grep -li 'stagehand' 2>/dev/null | wc -l      # expect: 0

# PRESERVATION — plan/012 still has stagehand refs (the prune worked; the rename docs are intact):
grep -rli 'stagehand' plan/012_963e3918ec08/ 2>/dev/null | wc -l   # expect: > 0

# SCOPE — no file touched (it's a verified no-op): git status should show NO plan/ changes (or only the
# expected ones if the safety-net found a straggler):
git -C /home/dustin/projects/stagehand status --porcelain plan/ | head

# FORBIDDEN-FILE AUDIT — the 19 tasks.json residue files (DO NOT TOUCH; flag for the orchestrator):
grep -rli 'stagehand' plan/ 2>/dev/null | grep -vE '^plan/012_963e3918ec08/'   # expect: 19 tasks.json paths

# Regression (historical plan/ is not compiled):
go build ./... && go test ./...    # expect: green, unaffected
```

---

## 8. Confidence & the task's real deliverable

**Confidence: 9.5/10.** The state is grep-verified, unambiguous, and reproducible.

**The real deliverable is a VERIFICATION + DOCUMENTATION, not a 622-file rename:**
1. RUN the corrected command (it renames 0 in-scope files — confirming the surface is clean). The `xargs -r`
   makes the empty case a clean no-op (without it, the empty grep would error).
2. ASSERT zero in-scope residue (the primary gate) + plan/012 preservation (the prune worked).
3. DOCUMENT: the in-scope plan/ surface (.md/.go/.toml/.txt, excluding 012) is already renamed (0 residue);
   the only remaining stagehand refs are 19 orchestrator-owned `tasks.json` files (FORBIDDEN — out of
   scope); flag them for the orchestrator / P1.M5.T2.S1.
4. DO NOT modify tasks.json. DO NOT modify plan/012.

**Risk to flag for the orchestrator**: P1.M5.T2.S1's whole-repo zero-residue audit (`grep -ri stagehand`)
will report the 19 tasks.json files as residue. That is NOT a failure of THIS task — tasks.json is
orchestrator-owned. The orchestrator must decide whether to rename tasks.json (outside this task's scope).
