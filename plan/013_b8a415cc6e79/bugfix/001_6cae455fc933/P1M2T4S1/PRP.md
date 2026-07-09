name: "P1.M2.T7.S1 (written to P1M2T4S1 path per dispatch) — Sweep README.md and overview docs for stale config-precedence references"
description: >
  Mode-B, changeset-level documentation sync that runs LAST, after every implementing subtask
  (P1.M1.T1.S1 → P1.M2.T6.S1) has landed its per-feature Mode A doc updates. Its job is to catch
  REMAINING DRIFT in the overview/README docs and make the whole doc set internally consistent with
  the resolved behavior of the P1 changeset:
    (a) `auto_stage_all` and `multi_turn_fallback` are now `*bool` (precedence-aware) — a file or
        git-config `false` is honored end-to-end instead of being silently dropped (old "only-true-
        propagates" limitation gone).
    (b) `STAGECOACH_AUTO_STAGE_ALL` and `STAGECOACH_MULTI_TURN_FALLBACK` env vars now exist (DIRECT set).
    (c) The git-config key is the camelCase `stagecoach.autoStageAll` (snake_case `auto_stage_all` is
        invalid — git rejects underscores in the name segment).
    (d) `STAGECOACH_VERBOSE=2` is now handled gracefully (clear "not yet supported" message) rather
        than an opaque parse error.
  Pure documentation — NO Go code, NO test changes. Primary deliverable: targeted edits to
  `docs/cli.md` (the file with real drift), `docs/configuration.md` (one cleanup + a verify pass),
  and verify-only passes over `README.md` and `docs/how-it-works.md`. After this, a reader can
  successfully disable `auto_stage_all` via TOML, env, or git-config by following the docs.

  DISPATCH NOTE (read once, then ignore): the orchestrator routed this PRP to the `P1M2T4S1/`
  directory and labelled it "P1.M2.T4.S1", but the item_title/item_description (the actual work
  contract) describe the "Sweep README.md and overview docs for stale config-precedence references"
  task, which tasks.json maps to P1.M2.T7.S1. P1.M2.T4.S1 (VERBOSE=2) is a *different* task and is a
  *dependency* of this sweep, not its subject. This PRP builds the doc sweep per the contract.

---

## Goal

**Feature Goal**: Make every overview/README/configuration doc in the repo internally consistent with the
P1 changeset's resolved config-precedence behavior, so that NO doc still advertises the old
limitations ("cannot disable `auto_stage_all` via file", "no env var", invalid snake_case git key,
opaque `VERBOSE=2` error) and a reader following the docs can successfully disable `auto_stage_all`
through any of TOML / env / git-config.

**Deliverable**: Documentation-only edits to at most 4 files (no Go source, no tests):
1. `docs/cli.md` — fix 3 drifted lines that still say `--no-auto-stage` has "no env-var or git-config
   analogs" (it now has both), and cross-check `--verbose`/`--model` rows against the T4/T6 deliverables.
2. `docs/configuration.md` — clean up one misleading phrase in the `multi_turn_fallback` note (line 166)
   and run a final consistency sweep (most rows already correct from per-feature Mode A updates).
3. `README.md` — verify the precedence line is accurate (it is) and that no stale limitation language
   remains; optionally add a one-line configurability note. Verify-only unless drift is found.
4. `docs/how-it-works.md` — verify the `auto_stage_all` reference at line 327 is NOT stale (it is
   valid work-description behavior). Verify-only.

**Success Definition** (all must hold; every command run from repo root):
- `grep -rn -iE "cannot disable|only-true-propagates|only true propagates" docs/ README.md` returns
  **only** the `no_verify`/`push` notes in `docs/configuration.md` (which are correct AND explicitly
  state `auto_stage_all`/`multi_turn_fallback` are exempt as `*bool`).
- `grep -rn "no env-var or git-config analogs" docs/cli.md` returns a sentence that lists ONLY
  `--all` and `--dry-run` (i.e. `--no-auto-stage` has been removed from that list).
- `grep -rn "stagecoach.auto_stage_all" docs/ README.md` returns **nothing** (snake_case git key fully
  purged from all docs).
- `docs/cli.md` line 31 (flag table) and line 396 (env→flag→gitconfig table) both show
  `--no-auto-stage` mapped to `STAGECOACH_AUTO_STAGE_ALL` (env) and `stagecoach.autoStageAll` (git).
- No doc claims `multi_turn_fallback` is settable via a flag or a git-config key (it is not — see
  Gotchas). `auto_stage_all` is the only field configurable across all four layers.
- `go build ./...` succeeds and `go test ./...` is unchanged/green (proves docs-only; no behavioral
  change).

## User Persona (if applicable)

**Target User**: A developer reading the README or overview docs who wants to **persistently** disable
`auto_stage_all` (or `multi_turn_fallback`) and must trust the docs to pick a working method.

**Use Case**: "I don't want stagecoach to auto-stage — how do I turn that off permanently?" The reader
should find a correct, working method (TOML `auto_stage_all = false`, env `STAGECOACH_AUTO_STAGE_ALL=false`,
or `git config stagecoach.autoStageAll false`) in whatever doc they land on, with no stale "you can't"
language.

**User Journey**: README → (link) docs/configuration.md or docs/cli.md → copy the exact TOML/env/git-config
snippet → it works first try.

**Pain Points Addressed**: Docs that contradict the binary (invalid keys, missing env vars, "cannot
disable" claims that are no longer true) cause silent misconfiguration and erosion of trust in the docs.

## Why

- The P1 changeset (Issues 1–6) fixed real config-precedence bugs; the per-feature doc updates rode with
  each code subtask (Mode A). This task (Mode B) is the **final cross-cutting sweep** that catches drift
  the per-feature edits didn't reach — chiefly the high-level `docs/cli.md` flag/env tables and the README.
- Without it, a reader could still conclude `--no-auto-stage` is flag-only (it isn't anymore) or that
  `auto_stage_all` can't be disabled via file (it can now), defeating the user-facing point of the fix.
- It is explicitly the **last** task in the milestone (depends on T1–T6) precisely so it can reconcile
  every doc against the final resolved behavior in one pass.

## What

Documentation consistency across the four overview/reference docs. Concretely:

### Success Criteria

- [ ] `docs/cli.md` lines 31, 63, 396 no longer claim `--no-auto-stage` lacks env/git-config analogs.
- [ ] `docs/cli.md` `--verbose` (L28/L393) and `--model` (L25) rows are consistent with the T4 (VERBOSE=2)
      and T6 (--model shadow hint) deliverables.
- [ ] `docs/configuration.md` L166 `multi_turn_fallback` note no longer references the wrong field
      (`autoStageAll`) and accurately states multi_turn_fallback's real config surface.
- [ ] No overview doc contains stale "cannot disable" / "only-true-propagates" language for
      `auto_stage_all` or `multi_turn_fallback`.
- [ ] `README.md` precedence line (L264) is verified accurate; no stale limitation language anywhere.
- [ ] `docs/how-it-works.md` L327 `auto_stage_all` reference verified as valid (not stale).
- [ ] No Go source or test file is modified; `go build ./...` and `go test ./...` stay green.

## All Needed Context

### Context Completeness Check

_Yes._ The exact drifted lines are quoted below with their current text, the code-level ground truth
(the precise set of git-config keys `internal/config/git.go` reads) is stated, and the validation
commands are repo-specific and copy-pasteable. An implementer who has never seen this repo can apply
the edits and verify them with the provided greps.

### Documentation & References

```yaml
# MUST READ — the contract this sweep enforces
- file: PRD.md
  why: "§9.8 FR34 (5-layer precedence), FR35 (stagecoach_ env prefix), FR36 (git-config keys); §19 (VERBOSE=2)"
  section: "Issues 1–3 (Major) and Issue 4 (Minor) — the resolved behavior the docs must match"
  critical: "FR36 lists the SNAKE_CASE key stagecoach.auto_stage_all, but git rejects underscores in the
    name segment. The CODE reads camelCase stagecoach.autoStageAll. Docs must use camelCase. This is
    exactly the discrepancy this sweep exists to prevent from recurring in overview docs."

- file: plan/013_b8a415cc6e79/bugfix/001_6cae455fc933/prd_snapshot.md
  why: "Snapshot of the PRD sections relevant to this changeset (Overview, Major/Minor issues, testing)."
  section: "h2.2 Issue 1–3, h2.3 Issue 4"

# Code ground truth — the single source of what config keys ACTUALLY exist
- file: internal/config/git.go
  why: "Defines EXACTLY which stagecoach.* git-config keys are read. The docs must match this set 1:1."
  pattern: "gitConfigBool(repoDir, \"stagecoach.autoStageAll\") at L159 (camelCase). Comment at L104
    explicitly says keys follow §16.3 camelCase, NOT FR36 snake_case."
  critical: "There is NO stagecoach.multiTurnFallback key in this file, and no CLI flag for multi_turn.
    So multi_turn_fallback is settable via TOML + STAGECOACH_MULTI_TURN_FALLBACK env ONLY — do NOT let
    any overview doc claim a flag or git-config path for it."

- file: internal/config/load.go
  why: "loadEnv() defines the STAGECOACH_* env vars. STAGECOACH_AUTO_STAGE_ALL and
    STAGECOACH_MULTI_TURN_FALLBACK were added in P1.M1.T2.S1 (DIRECT set so false works)."

# The doc files being swept (current state captured 2026-07-09)
- file: docs/cli.md
  why: "Contains the real drift: lines 31, 63, 396 still treat --no-auto-stage as flag-only."
  pattern: "Two tables: the main flag table (~L22–L60) and the env→flag→gitconfig table (~L387–L403)."
  gotcha: "multi_turn_fallback has NO flag, so it legitimately does NOT appear in either cli.md table —
    do not add a row for it. The authoritative env table lives in docs/configuration.md."

- file: docs/configuration.md
  why: "Already consistent from per-feature Mode A updates, EXCEPT L166 which mis-attributes multi_turn's
    git behavior to autoStageAll."
  pattern: "Env table ~L179–L201; git-config INI + table ~L204–L220; *bool notes ~L157, L166."

- file: README.md
  why: "Config-precedence line L264 is the overview statement this sweep must keep accurate."
  pattern: "High-level Features table (L56–67) + config section (~L255–L265). No stale limitation language
    found in README during research — verify-only."

- file: docs/how-it-works.md
  why: "L327 references auto_stage_all in the context of --work-description mode."
  gotcha: "L327 says stagecoach auto-stages 'even with auto_stage_all disabled' — this is VALID behavior,
    NOT a stale limitation. Leave it. (Only the multi_turn_fallback note at L276 references a default;
    also accurate.)"
```

### Current Codebase tree (overview, relevant slices)

```bash
docs/
  cli.md            # <- DRIFT: L31, L63, L396 (--no-auto-stage analogs)
  configuration.md  # <- L166 cleanup + verify pass (else already consistent)
  how-it-works.md   # <- verify-only (L327 is valid, not stale)
  providers.md
  README.md         # docs/README — not a target
README.md           # <- verify L264 precedence; optional one-line note
internal/config/
  git.go            # GROUND TRUTH: exact set of stagecoach.* keys (camelCase)
  load.go           # GROUND TRUTH: STAGECOACH_* env vars (incl. the 2 new bools)
```

### Desired Codebase tree with files to be edited and their responsibility

```bash
docs/cli.md            # EDIT L31, L63, L396 (auto_stage_all env+git analogs); cross-check L25/L28 vs T4/T6
docs/configuration.md  # EDIT L166 (multi_turn_fallback wording); VERIFY the rest
README.md              # VERIFY L264 + grep; OPTIONAL one-line configurability note
docs/how-it-works.md   # VERIFY L327 is not stale (expected: no change needed)
# (no new files; no code; no tests)
```

### Known Gotchas of our codebase & Library Quirks

```text
# CRITICAL ACCURACY: the item contract says "auto_stage_all and multi_turn_fallback are fully
# configurable through all layers". That is literally true ONLY for auto_stage_all.
#   auto_stage_all    -> flag (--no-auto-stage, INVERSE) + env (STAGECOACH_AUTO_STAGE_ALL) +
#                        git-config (stagecoach.autoStageAll) + TOML.  [ALL 4 LAYERS]
#   multi_turn_fallback -> env (STAGECOACH_MULTI_TURN_FALLBACK) + TOML ONLY.
#                        No flag, no git-config key (verified: not in internal/config/git.go).
# Do NOT let any overview doc claim multi_turn_fallback is flag- or git-config-settable.
# Disable multi-turn per-provider alternatively via session_mode = "" (see providers.md).

# Git forbids underscores in the final config-key name segment:
#   `git config stagecoach.auto_stage_all false`  -> error: invalid key (exit 1)
#   `git config stagecoach.autoStageAll false`    -> OK (exit 0), disables auto-stage
# All multi-word git keys in this codebase are camelCase (stripCodeFence, tokenLimit,
# diffContext, noVerify, autoStageAll, maxDiffBytes, ...). Docs must match.

# --no-auto-stage is the INVERSE of auto_stage_all: the flag DISABLES, but the env/git/TOML
# values are "true = enable, false = disable" (positive sense). When you map --no-auto-stage to
# STAGECOACH_AUTO_STAGE_ALL in a table, label it "(inverse)" to avoid confusion.

# VERBOSE=2 (PRD §19): not implemented as a feature. After T4 it is rejected with a clear
# "not yet supported" message instead of an opaque strconv.ParseBool error. Do NOT document
# VERBOSE=2 as a working feature in overview docs; if a doc mentions it, mark it unsupported.

# This is a Mode B sweep that runs LAST. Some referenced lines (e.g. configuration.md L179
# STAGECOACH_VERBOSE row, cli.md --model help) are owned by T4/T6. If those tasks haven't
# landed yet, make this sweep's edits that DON'T depend on them now, and re-verify the T4/T6-
# dependent lines against those tasks' final output before declaring done.
```

## Implementation Blueprint

### Data models and structure

_None._ Pure documentation; no structs, schemas, or migrations.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: FIX docs/cli.md — --no-auto-stage env + git-config analogs (3 lines)
  WHY: These are the only lines in the whole doc set that still assert the PRE-fix reality.
  EDIT L31 (main flag table row):
    OLD: | `--no-auto-stage` | bool | false | — | — | If nothing is staged, exit instead of auto-staging |
    NEW: | `--no-auto-stage` | bool | false | `STAGECOACH_AUTO_STAGE_ALL` (inverse) | `stagecoach.autoStageAll` | If nothing is staged, exit instead of auto-staging (env/git-config use the POSITIVE sense: true=enable, false=disable) |
  EDIT L63 (prose):
    OLD: "The behavioral flags (`--all`, `--no-auto-stage`, `--dry-run`) have no env-var or git-config analogs."
    NEW: "The behavioral flags `--all` and `--dry-run` have no env-var or git-config analogs.
          (`--no-auto-stage` does: it mirrors `STAGECOACH_AUTO_STAGE_ALL` and `stagecoach.autoStageAll`,
          in the positive sense — true=enable, false=disable.)"
  EDIT L396 (env→flag→gitconfig table row):
    OLD: | `--no-auto-stage` | — | — |
    NEW: | `--no-auto-stage` | `STAGECOACH_AUTO_STAGE_ALL` (inverse) | `stagecoach.autoStageAll` |
  FOLLOW pattern: the other rows in the same tables (e.g. L390 `--model` → STAGECOACH_MODEL → stagecoach.model).
  NAMING: camelCase git key `stagecoach.autoStageAll`; env var `STAGECOACH_AUTO_STAGE_ALL` (uppercase snake).
  VERIFY after edit: grep -n "no-auto-stage" docs/cli.md  # rows now show the env var + git key.

Task 2: CROSS-CHECK docs/cli.md --verbose / --model rows against T4 / T6 deliverables
  WHY: T4 (VERBOSE=2 graceful reject) and T6 (--model shadow hint) own these rows; this sweep
        must ensure the overview tables agree with their final wording.
  CHECK L28 (`--verbose`, STAGECOACH_VERBOSE): ensure the Description does NOT promise VERBOSE=2
        works. If T4 added a "not yet supported" note anywhere, mirror a short "(VERBOSE=2 not yet
        supported)" hint here ONLY if T4's docs did so; otherwise leave the row as-is.
  CHECK L25 (`--model`): if T6 added a --model-shadow hint to the binary's --help, ensure the cli.md
        --model description is consistent (it may stay brief; the detail lives in T6's hint).
  NOTE: This is a CONSISTENCY check, not an independent feature. Do not invent behavior; match T4/T6.
  DEPENDENCIES: ideally run after T4/T6 land; if not yet landed, defer only these two sub-checks.

Task 3: CLEAN UP docs/configuration.md L166 — multi_turn_fallback note mis-attributes the git key
  WHY: L166 currently says multi_turn_fallback can be disabled "... or via `git config
        stagecoach.autoStageAll`-style `*bool` behavior". That references autoStageAll, the WRONG
        field — there is no stagecoach.multiTurnFallback git key (see git.go). This is exactly the
        kind of subtle drift this sweep exists to catch.
  EDIT L166 (multi_turn_fallback bullet):
    - Keep the TRUE part: it is a *bool field; a file `multi_turn_fallback = false` IS honored
      end-to-end; you can also disable multi-turn per-provider via session_mode = "".
    - REMOVE/REPLACE the misleading "git config stagecoach.autoStageAll-style" clause. Replace with an
      accurate statement of multi_turn_fallback's real surface, e.g.:
        "Settable via a config file (`multi_turn_fallback = false`) or the
        `STAGECOACH_MULTI_TURN_FALLBACK=false` env var; there is no CLI flag or git-config key for it,
        so to disable multi-turn persistently use the config file or env var (or set
        `session_mode = \"\"` on the provider — see providers.md)."
  FOLLOW pattern: the adjacent auto_stage_all/no_verify notes (L157) which precisely name each field's
        available layers.
  GOTCHA: do NOT claim a git-config key or flag for multi_turn_fallback — neither exists.

Task 4: VERIFY docs/configuration.md — final consistency sweep (no edits expected beyond Task 3)
  VERIFY each referenced line is correct and contains no stale limitation language:
    - L87   template `# auto_stage_all = true` (fine)
    - L133  defaults table `auto_stage_all | true` (fine)
    - L140  `multi_turn_fallback | true` (fine)
    - L157  no_verify note (correctly states auto_stage_all/multi_turn are *bool and EXEMPT from
            only-true-propagates — leave)
    - L166  (fixed in Task 3)
    - L179  STAGECOACH_VERBOSE row (final wording owned by T4 — verify consistency with T4)
    - L200  STAGECOACH_AUTO_STAGE_ALL row (present, correct)
    - L201  STAGECOACH_MULTI_TURN_FALLBACK row (present, correct)
    - L212  INI `autoStageAll = true` (camelCase, correct)
    - L220  git-config table `stagecoach.autoStageAll` (camelCase, correct)
  VERIFY: grep -n -iE "cannot disable|only-true-propagates" docs/configuration.md
          -> ONLY the no_verify/push lines, which are correct.

Task 5: VERIFY README.md — precedence line + no stale limitation language (verify-only; optional note)
  VERIFY L264: "**Config precedence** (highest → lowest): CLI flags > `STAGECOACH_*` env vars > repo
        `git config` (`stagecoach.*`) > repo `.stagecoach.toml` > global config file > provider
        defaults > built-in defaults." -> ACCURATE (lists all layers). Leave.
  VERIFY: grep -n -iE "cannot disable|limitation|auto_stage_all|only-true" README.md
          -> research found NO stale limitation language for these fields. Confirm none.
  OPTIONAL (only if it reads naturally): if README has a config/features blurb near L264, a one-line
        note that `auto_stage_all`/`multi_turn_fallback` are tunable through all layers may help
        discoverability. Keep it to ONE line; do NOT enumerate every layer in the high-level README
        (that detail lives in docs/configuration.md). If unsure, SKIP — correctness > completeness.

Task 6: VERIFY docs/how-it-works.md — L327 is valid behavior, not stale (verify-only; no edit expected)
  VERIFY L320–L330: the `auto_stage_all` reference at L327 ("stagecoach auto-stages all (even with
        `auto_stage_all` disabled)") describes --work-description mode's deliberate behavior — VALID,
        NOT a stale limitation. Leave it.
  VERIFY L275–L276: multi-turn trigger condition "multi_turn_fallback is enabled (default true)" —
        accurate. Leave.
  EDIT ONLY IF you find actual stale limitation language (research found none here).

Task 7: WHOLE-DOC GREP SWEEP + build sanity
  RUN (expect the indicated outcomes):
    grep -rn -iE "cannot disable|only-true-propagates|only true propagates" docs/ README.md
      # -> only the no_verify/push notes in docs/configuration.md (correct + exempt auto_stage_all/multi_turn)
    grep -rn "no env-var or git-config analogs" docs/cli.md
      # -> the rewritten sentence lists ONLY --all and --dry-run
    grep -rn "stagecoach.auto_stage_all" docs/ README.md
      # -> nothing
    grep -rn "STAGECOACH_AUTO_STAGE_ALL\|STAGECOACH_MULTI_TURN_FALLBACK" docs/cli.md docs/configuration.md
      # -> present and correctly mapped
  RUN (docs-only sanity; must stay green/unchanged):
    go build ./...      # proves nothing in code changed/broke
    go test ./...       # full suite green (no behavioral change)
```

### Implementation Patterns & Key Details

```text
# Doc-edit discipline for this repo:
# - Match the surrounding table column alignment/formatting exactly (markdown tables in cli.md).
# - Preserve existing cross-doc links and PRD section anchors (e.g. "§9.8 FR34").
# - CamelCase for ALL multi-word git-config keys; UPPERCASE_SNAKE for STAGECOACH_* env vars;
#   snake_case for TOML/config-file keys. These three casings are load-bearing — do not mix them.
# - When mapping the INVERSE flag --no-auto-stage to the POSITIVE env/git values, always annotate
#   "(inverse)" so a reader isn't misled into thinking --no-auto-stage=true enables staging.
# - Do not expand scope: this is the cross-cutting sweep. Per-field depth already lives in
#   docs/configuration.md; keep README/cli.md rows tight.
```

### Integration Points

```yaml
NO BUILD/BINARY/CONFIG CHANGES:
  - This task edits Markdown only. There is no config-file schema, no flag, no env var, no migration.
  - The resolved behavior it documents was IMPLEMENTED by the prerequisite subtasks
    (P1.M1.T1.S1 *bool, P1.M1.T2.S1 env vars, P1.M1.T3.S1 git-key casing, P1.M2.T4.S1 VERBOSE=2,
    P1.M2.T6.S1 --model hint). This task reconciles DOCS to that already-shipped behavior.

DOC CROSS-REFERENCES TO KEEP CONSISTENT:
  - README.md L264  <-> docs/configuration.md precedence section <-> docs/cli.md L385 mapping note
  - docs/cli.md L31/L396 <-> docs/configuration.md L200 (STAGECOACH_AUTO_STAGE_ALL) + L220 (autoStageAll)
  - docs/configuration.md L166 <-> docs/providers.md (session_mode schema) for the multi_turn alternative
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Markdown has no compiler; validate structure/links instead.
# 1. No broken/changed code paths (docs-only):
go build ./... && echo "build OK (unchanged)"
go vet ./...       # optional; should be clean and unchanged

# 2. Confirm ONLY markdown files were touched (no .go files):
git status --short                # expect only docs/*.md and/or README.md
git diff --name-only | grep -v -E '\.(md)$' && echo "FAIL: non-markdown file changed" || echo "OK: docs-only"
```

### Level 2: Content Grep Gates (the real "unit tests" for a doc sweep)

```bash
# Gate A: no stale limitation language for the FIXED fields
grep -rn -iE "cannot disable|only-true-propagates|only true propagates" docs/ README.md
# EXPECT: only docs/configuration.md lines about no_verify/push, which EXEMPT auto_stage_all/multi_turn.

# Gate B: the --no-auto-stage "no analogs" sentence is fixed
grep -rn "no env-var or git-config analogs" docs/cli.md
# EXPECT: a sentence listing ONLY --all and --dry-run (NOT --no-auto-stage).

# Gate C: snake_case git key fully purged
grep -rn "stagecoach.auto_stage_all" docs/ README.md
# EXPECT: no output.

# Gate D: the new env vars are mapped in the reference tables
grep -rn "STAGECOACH_AUTO_STAGE_ALL\|STAGECOACH_MULTI_TURN_FALLBACK" docs/cli.md docs/configuration.md
# EXPECT: STAGECOACH_AUTO_STAGE_ALL present in cli.md (L31/L396) AND configuration.md (L200);
#         STAGECOACH_MULTI_TURN_FALLBACK present in configuration.md (L201).
#         (multi_turn has no flag, so it correctly does NOT appear in cli.md's flag tables.)

# Gate E: auto_stage_all git key is camelCase everywhere
grep -rn "autoStageAll" docs/ README.md
# EXPECT: camelCase occurrences only (configuration.md INI L212 + table L220; cli.md L31/L396).
```

### Level 3: Integration Testing (System Validation)

```bash
# Prove the documented methods actually work end-to-end (docs match the binary).
# (Requires the P1 code changes from the prerequisite subtasks to be in the tree.)

# 3a. Build the binary
go build -o /tmp/stagecoach ./cmd/stagecoach

# 3b. git-config path the docs now advertise (camelCase) must be SETTABLE:
tmp=$(mktemp -d) && git init -q "$tmp" && cd "$tmp"
git config stagecoach.autoStageAll false && echo "OK: camelCase key settable (exit 0)"
git config stagecoach.auto_stage_all false 2>/dev/null && echo "FAIL: snake_case should be rejected" \
  || echo "OK: snake_case rejected by git (matches docs)"
cd - >/dev/null && rm -rf "$tmp"

# 3c. env-var path the docs advertise must work (DIRECT set, false honored):
#     (smoke check that STAGECOACH_AUTO_STAGE_ALL=false is accepted at load — no parse error)
STAGECOACH_AUTO_STAGE_ALL=false /tmp/stagecoach --help >/dev/null && echo "OK: env var accepted"

# 3d. Full test suite unaffected (docs-only change must not alter behavior):
go test ./...
# EXPECT: all packages pass, same as before this task.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Render-check the edited Markdown tables (column counts must match headers) — quick awk sanity:
awk -F'|' '/^\| .+ \|$/ {n=NF} NR==1{h=NF} END{}' docs/cli.md   # eyeball: row field counts == header
# Alternatively, paste docs/cli.md into a Markdown previewer and confirm the two tables render.

# Cross-doc link lints (ensure no anchors were broken by edits):
grep -rn "configuration.md#\|how-it-works.md#\|cli.md#" docs/ README.md | grep -iE "auto|stage|multi|verbose"
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1: `go build ./...` succeeds; `git status` shows ONLY `*.md` changes.
- [ ] Level 2 Gate A: no stale "cannot disable"/"only-true-propagates" for auto_stage_all/multi_turn_fallback.
- [ ] Level 2 Gate B: `docs/cli.md` "no analogs" sentence lists only `--all` and `--dry-run`.
- [ ] Level 2 Gate C: `grep "stagecoach.auto_stage_all"` returns nothing across docs/README.
- [ ] Level 2 Gate D: `STAGECOACH_AUTO_STAGE_ALL` mapped in cli.md + configuration.md.
- [ ] Level 2 Gate E: `autoStageAll` is camelCase everywhere.
- [ ] Level 3: `git config stagecoach.autoStageAll false` succeeds; snake_case rejected; `go test ./...` green.

### Feature Validation

- [ ] `docs/cli.md` L31, L63, L396 no longer treat `--no-auto-stage` as flag-only.
- [ ] `docs/cli.md` `--verbose`/`--model` rows consistent with T4/T6 final wording.
- [ ] `docs/configuration.md` L166 multi_turn_fallback note no longer references `autoStageAll`; states the
      real (env + TOML only) surface; no false flag/git-config claim.
- [ ] `README.md` L264 precedence verified accurate; no stale limitation language.
- [ ] `docs/how-it-works.md` L327 verified valid (no edit needed unless real drift found).
- [ ] A reader can disable `auto_stage_all` via TOML, env, OR git-config by following the docs.

### Code Quality Validation

- [ ] Edits follow existing table column alignment and markdown style.
- [ ] Casings respected: camelCase git keys, UPPERCASE_SNAKE env vars, snake_case TOML keys.
- [ ] `--no-auto-stage` → env/git mapping annotated "(inverse)".
- [ ] Scope held to the cross-cutting sweep (no per-field depth duplicated into README/cli.md).
- [ ] No PRD.md, tasks.json, prd_snapshot.md, or .gitignore modified.

### Documentation & Deployment

- [ ] Cross-doc links and PRD section anchors preserved.
- [ ] No overview doc overclaims multi_turn_fallback's config surface (no flag, no git-config key).
- [ ] VERBOSE=2 not advertised as a working feature anywhere.

---

## Anti-Patterns to Avoid

- ❌ Don't claim `multi_turn_fallback` is settable via a flag or git-config — it isn't (verified in
  `internal/config/git.go`). Only `auto_stage_all` spans all four layers.
- ❌ Don't reintroduce the snake_case `stagecoach.auto_stage_all` key anywhere — git rejects it.
- ❌ Don't document `VERBOSE=2` as implemented — it is gracefully rejected ("not yet supported"), not a feature.
- ❌ Don't map `--no-auto-stage` to its env/git values without the "(inverse)" note — the flag is the
  inverse sense of the positive env/git/TOML values.
- ❌ Don't expand depth into README/cli.md — those are overviews; full detail stays in configuration.md.
- ❌ Don't edit any `.go` file, PRD.md, tasks.json, or prd_snapshot.md — this is docs-only.
- ❌ Don't skip the `go test ./...` gate "because it's just docs" — it proves no accidental code change.
