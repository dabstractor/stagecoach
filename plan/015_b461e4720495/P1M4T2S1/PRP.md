name: "P1.M4.T2.S1 — Update README + docs/how-it-works + docs/configuration + docs/cli for per-role timeouts (FR-R7, §9.15)"
description: >
  THE changeset-level (Mode B) documentation-sync task for the v2.8 per-role-generation-timeout feature
  (PRD §9.15 FR-R7) that LANDED in P1.M1–P1.M3. The Mode A "riding docs" shipped UNEVENLY: docs/cli.md got
  the four `--<role>-timeout` flags + the 120s global default (P1.M1.T2.S2 ✓); docs/configuration.md got the
  `stagecoach.role.<role>.timeout` git-config key (P1.M1.T2.S3 ✓) but is MISSING the per-role ENV-VAR rows,
  the `[defaults]` FR-R7 comment, the `[role.planner]` timeout example, and the per-role prose; README +
  docs/how-it-works have ZERO timeout docs. This task closes those gaps and fixes ONE stale artifact
  (docs/cli.md's flag map shows the per-role-timeout git-config column as "—" but the layer EXISTS). It
  edits ONLY docs: README.md, docs/how-it-works.md, docs/configuration.md, docs/cli.md. Concretely:
  (1) docs/configuration.md — `[defaults]` timeout comment (FR-R7; planner 480s), a `[role.planner]`
  timeout example, four STAGECOACH_<ROLE>_TIMEOUT env-var rows, and a per-role-timeout prose sentence;
  (2) docs/how-it-works.md — per-role-timeout note in the cross-ref + fix the multi-turn budget line to the
  message-role timeout (FR-T5); (3) README.md — `--<role>-timeout` mention in the multi-commit section;
  (4) docs/cli.md — VERIFY the `--timeout` default is 120s (it is) + FIX the stale git-config column on the
  four per-role-timeout flag-map rows. Wording must match PRD §9.15/§16.1/§16.2/§16.4 + the exact shipped
  names. NO source code (the bootstrap.go template gap is a production file — flagged, NOT edited). Validates
  via markdownlint (MD013/MD033/MD060 off) + grep guards.

---

## Goal

**Feature Goal**: Make the shipped human-readable documentation (README.md + docs/) consistent with the
FR-R7 per-role-generation-timeout feature shipped in P1.M1–P1.M3, so a user reading the docs learns that
(a) each role (planner/stager/message/arbiter) resolves its OWN timeout, (b) the planner defaults to 480s
(the heavy role) while the others inherit the global 120s, (c) the four override surfaces
(`--<role>-timeout` / `STAGECOACH_<ROLE>_TIMEOUT` / `[role.<role>].timeout` / `stagecoach.role.<role>.timeout`)
exist, and (d) the multi-turn fallback now uses the message role's resolved timeout (FR-T5). The docs must
match the PRD §9.15/§16.1–§16.4 wording AND the exact binary/config names.

**Deliverable**: Edits to FOUR docs files only — `docs/configuration.md` (primary), `docs/how-it-works.md`,
`README.md`, `docs/cli.md`. No new files. Deltas: a `[defaults]` FR-R7 timeout comment + a `[role.planner]`
timeout example + four env-var rows + a per-role prose sentence in configuration.md; a per-role-timeout note
+ an FR-T5 multi-turn budget fix in how-it-works.md; a `--<role>-timeout` mention in README's multi-commit
section; a git-config-column fix on the four per-role-timeout rows of cli.md's flag map (plus a verify that
the global `--timeout` default reads 120s).

**Success Definition**:
- `grep -rniE "per-role|role-timeout|STAGECOACH_.*_TIMEOUT|role\..*\.timeout|FR-R7|480s" README.md docs/`
  returns hits in all FOUR docs files (per-role timeouts are now documented across the surface).
- `grep -niE "default 480s|default.*480s" docs/cli.md README.md` returns ZERO global-flag default claims
  of 480s (the global `--timeout` default is 120s everywhere; the 480s appears ONLY as the planner role's
  built-in). docs/cli.md L27 already says 120s (verified).
- `grep -nE '\| `--planner-timeout`.*\| `stagecoach.role.planner.timeout`' docs/cli.md` returns 1 hit (the
  stale "—" git-config column is fixed for all four per-role-timeout rows).
- `npx markdownlint-cli2 'README.md' 'docs/**/*.md'` is CLEAN (config disables MD013/MD033/MD060; match the
  existing long-line, `> [!NOTE]`/table style).
- `git status --porcelain` shows ONLY the four docs files (scope guard). NO source, NO PRD, NO plan/tasks.

## User Persona (if applicable)

**Target User**: A Stagecoach user running multi-commit decomposition (or a maintainer supporting them) whose
planner times out on a large diff, or who wants the fast bare roles snappier than the planner.
**Use Case**: The user hits `decompose: planner failed: context deadline exceeded` and wants to know how to
tune the planner's timeout independently, or wants to shorten the message-role timeout on the single-commit
path. They read README → docs/how-it-works → docs/configuration → docs/cli to find `--planner-timeout` /
`[role.planner].timeout` / `STAGECOACH_PLANNER_TIMEOUT` / `stagecoach.role.planner.timeout`.
**User Journey**: README multi-commit section (discovers per-role timeout exists) → docs/configuration.md
(sets `[role.planner].timeout` or env) → docs/cli.md (flag + git-config map) → docs/how-it-works.md (the
FR-T5 multi-turn nuance).
**Pain Points Addressed**: "why did the planner time out at 120s?" (it didn't — it's 480s; the docs must say
so); "how do I give the planner more time without slowing the other roles?" (per-role override); the stale
cli.md flag map that hid the `stagecoach.role.<role>.timeout` git-config surface.

## Why

- **FR-R7 traceability to docs**: the per-role-timeout config + resolution + consumption LANDED
  (P1.M1–P1.M3 = "Complete") but the human-readable docs are incomplete (env rows, [defaults] comment,
  [role.planner] example, README/how-it-works all missing) and one artifact is stale (cli.md flag map).
  This item closes the doc gaps so the feature is discoverable and the planner-480s asymmetry is explained.
- **The planner-480s asymmetry is the #1 confusion point**: `--timeout` (global) does NOT change the planner.
  The docs must state this explicitly, or users will set `--timeout 600s` expecting the planner to get it and
  be surprised it stays 480s (it has a built-in that beats the global).
- **Mode B coherence**: the delta_prd Phase 3 tasked this final doc pass; the per-surface Mode A docs landed
  unevenly (cli.md done, configuration.md partial, README/how-it-works zero). This task reconciles them.

## What

Doc-only edits across four files. Every addition must match (a) the PRD §9.15/§16.1–§16.4 wording and
(b) the exact shipped names captured in research/findings.md §0.

### Success Criteria
- [ ] **docs/configuration.md**: `[defaults]` timeout line (L86) has the FR-R7 comment ("global fallback for
      every role (FR-R7); planner defaults to 480s"); the `[role.*]` file-format block has a `[role.planner]`
      `# timeout = "600s"` example; the env-vars table has four `STAGECOACH_<ROLE>_TIMEOUT` rows; the per-role
      prose (L251) names `--<role>-timeout` + the planner-480s / others-120s asymmetry.
- [ ] **docs/how-it-works.md**: the L132 cross-ref mentions per-role timeouts (planner 480s); the multi-turn
      L303 line reads `message-timeout × (N+1)` (FR-T5), not flat `timeout × (N+1)`.
- [ ] **README.md**: the multi-commit decomposition section mentions `--<role>-timeout` (FR-R7) + the planner
      480s default (either a `[role.planner] timeout` example line or a `> [!NOTE]`).
- [ ] **docs/cli.md**: L27 `--timeout` default verified as `"120s"` (no edit — it's correct); the four
      per-role-timeout rows of the flag↔env↔git-config map (L453-456) have the git-config column filled
      (`stagecoach.role.<role>.timeout`) instead of the stale `—`.
- [ ] NO docs file claims the GLOBAL `--timeout` default is 480s (480s appears ONLY as the planner role's
      built-in).
- [ ] markdownlint clean on the four files; scope guard shows only the four docs files.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact shipped names (4 surfaces + the 120s/480s asymmetry) with code line numbers, the verbatim
PRD wording to mirror (§9.15/§16.1–§16.4 + the delta_prd FR-R7/FR-T5 text), the exact docs insertion points
(file + heading + line number), the stale cli.md flag-map artifact, the bootstrap.go out-of-scope flag, the
markdownlint config, and the validation commands.

### Documentation & References

```yaml
# MUST READ — the codebase-specific findings for THIS item (shipped surface + docs gaps + insertion points).
- docfile: plan/015_b461e4720495/P1M4T2S1/research/findings.md
  why: "§0 the EXACT shipped surface (table of names + code line numbers) + the planner-480s asymmetry +
        the git-config-WAS-implemented note + single-commit=message=120s; §1 PRD anchors; §2 docs insertion
        points (file+line); §3 the bootstrap.go out-of-scope flag; §4 markdownlint; §5 scope fence."

# MUST READ — the verbatim PRD wording to mirror (the docs are DERIVED from the PRD).
- docfile: plan/015_b461e4720495/prd_snapshot.md
  section: "§9.15 (FR-R7 per-role timeout — see the delta_prd:35-54 verbatim); §16.1 (timeout 120s global,
            planner 480s built-in); §16.2 [defaults] (timeout comment wording); §16.4 (per-role config)."
  why: "Copy the FR-R7 phrasing: 'Each role resolves its OWN generation timeout … Layers (highest wins):
        --<role>-timeout > STAGECOACH_<ROLE>_TIMEOUT > [role.<role>].timeout > built-in > [defaults].timeout
        (120s). planner = 480s; stager/message/arbiter inherit the global.' + the back-compat sentence."
- docfile: plan/015_b461e4720495/delta_prd.md
  section: "FR-R7 (line 35-54); FR-T5 (line 51 — multi-turn per-turn = message-role timeout); Phase 3 (line
            127-138 — the Mode B doc spec for this task)."
  why: "The delta_prd's Phase 3 is the authoritative doc-task breakdown. NOTE its #1 wanted bootstrap.go
        edited — but bootstrap.go is a SOURCE file and is NOT in this task's contract (a/b/c/d are docs);
        do NOT edit it (see findings §3). The human-readable docs ARE in scope."

# MUST READ — the shipped resolution + defaults (match these EXACTLY).
- file: internal/config/roles.go
  why: "ResolveRoleTimeout (L96): per-role override (cfg.Roles[role].Timeout != 0) > built-in
        (defaultRoleTimeouts[role]) > cfg.Timeout. defaultRoleTimeouts (L11) = {'planner': 480s} — the ONLY
        built-in. So planner ALWAYS gets ≥480s (the global --timeout does NOT change it); stager/message/
        arbiter fall through to the global 120s."
  gotcha: "Do NOT document a per-role built-in for stager/message/arbiter — only the planner has one."

# MUST READ — the four override surfaces (names + parse semantics).
- file: internal/cmd/root.go
  why: "L258-265: --planner-timeout/--stager-timeout/--message-timeout/--arbiter-timeout (string, zero
        default, read via fs.Changed). L169: --timeout global help 'default 120s'. These are the canonical
        flag names + the 120s default docs/cli.md L27 already shows."
- file: internal/config/load.go
  why: "L319: env `prefix + \"_TIMEOUT\"` ⇒ STAGECOACH_PLANNER_TIMEOUT/_STAGER_/_MESSAGE_/_ARBITER_
        (parseTimeout accepts '600s' AND bare '600'). L69 setRoleTimeout. These are the env-var names."
- file: internal/config/git.go
  why: "L167-175: reads `stagecoach.role.<role>.timeout` (timeout-ONLY — provider/model/reasoning are NOT
        read here). THIS IS WHY docs/cli.md's flag-map '—' for the four timeout rows is STALE and must be
        filled in. parseTimeout accepts both forms."
- file: internal/config/file.go
  why: "fileRoleConfig.Timeout string (parsed by materialize via time.ParseDuration — accepts '600s', NOT
        bare '600'). The config-file key is [role.<role>].timeout. (Note: the file layer is stricter than
        env/flag/git on bare-int — a pre-existing inconsistency; the docs example should use '600s' form.)"

# CONTEXT — the existing docs structure to mirror / extend (format + anchors).
- file: docs/configuration.md
  why: "L82-101 the file-format [defaults]+[role.*] example; L124-152 the defaults table (L133 timeout=120s
        is the GLOBAL, correct); L169-202 the env-vars table (4 cols: Variable|Mirrors flag|Description|
        Example); L203-241 the git-config keys (L228 already has stagecoach.role.<role>.timeout ✓, L241 NOTE
        confirms the timeout-only asymmetry ✓); L244-254 the per-role prose (L251 is the line to extend)."
- file: docs/how-it-works.md
  why: "L132 the cross-ref sentence (the per-role-config touch point); L295-304 the multi-turn section
        (L303 'timeout × (N+1)' is the FR-T5 line to fix). No dedicated per-role overview — L132 + multi-turn
        are the spots."
- file: docs/cli.md
  why: "L27 --timeout default '120s' (VERIFIED correct — contract d passes); L60-63 the four --<role>-timeout
        flags (P1.M1.T2.S2 ✓); L388-456 the flag↔env↔git-config map (L453-456 the four timeout rows show
        git-config as '—' — STALE, fix to stagecoach.role.<role>.timeout). NOTE the provider/model/reasoning
        per-role rows correctly stay '—' (no git-config layer for those)."
- file: README.md
  why: "L146-178 the Multi-commit decomposition section — L160-170 has the per-repo config example
        ([role.planner] provider/model). README has ZERO timeout docs. Add the --<role>-timeout mention +
        planner-480s here (or a > [!NOTE]). Match the existing `> [!NOTE]` style (L171-173)."

# CONTEXT — markdownlint config (rules off) + validation.
- file: .markdownlint.json
  why: "{ default:true, MD013:false, MD033:false, MD060:false } — line length / inline HTML / heading-
        punctuation are DISABLED. The docs use long single-line paragraphs + tables + > [!NOTE]; match that
        (do NOT hard-wrap at 80 cols). No `make docs` target — invoke `npx markdownlint-cli2` directly."

# OUT OF SCOPE — the bootstrap template (PRODUCTION file; flag the gap, do NOT edit).
- file: internal/config/bootstrap.go
  why: "L161 [defaults] line says '# timeout = \"120s\"' (correct value) but LACKS the planner-480s comment;
        the env block (L248-260) lists STAGECOACH_TIMEOUT + per-role PROVIDER/MODEL but NOT the per-role
        STAGECOACH_<ROLE>_TIMEOUT variants. The delta_prd Phase 3 #1 wanted both fixed — but bootstrap.go is
        a SOURCE file (FORBIDDEN) and is NOT in this task's contract. DO NOT EDIT. The human-readable
        docs/configuration.md env table + [defaults] comment ARE in scope and are the authoritative public docs."
```

### Current Codebase tree (relevant slice)

```bash
docs/configuration.md   # EDIT — [defaults] comment + [role.planner] timeout example + 4 env rows + per-role prose
docs/how-it-works.md    # EDIT — L132 cross-ref timeout note + L303 multi-turn FR-T5 fix
README.md               # EDIT — multi-commit section --<role>-timeout mention + planner 480s
docs/cli.md             # EDIT — fix the 4 per-role-timeout flag-map git-config columns (— → stagecoach.role.*.timeout)
# READ-ONLY references (do NOT edit):
PRD.md / prd_snapshot.md / delta_prd.md   # READ-ONLY — §9.15/§16.1-16.4/FR-R7/FR-T5 (the wording source)
internal/config/roles.go                  # READ-ONLY — ResolveRoleTimeout + defaultRoleTimeouts{planner:480s}
internal/config/config.go                 # READ-ONLY — Defaults().Timeout=120s, RoleConfig.Timeout
internal/config/load.go                   # READ-ONLY — STAGECOACH_<ROLE>_TIMEOUT env, parseTimeout
internal/config/git.go                    # READ-ONLY — stagecoach.role.<role>.timeout (timeout-only)
internal/config/file.go                   # READ-ONLY — fileRoleConfig.Timeout (the [role.<role>].timeout key)
internal/cmd/root.go                      # READ-ONLY — --<role>-timeout flags + --timeout help (120s)
internal/config/bootstrap.go              # READ-ONLY — PRODUCTION; the [defaults] template (gap flagged §3, NOT edited)
.markdownlint.json                        # READ-ONLY — MD013/MD033/MD060 off
```

### Desired Codebase tree with files to be added/edited

```bash
# FOUR docs files edited (NO new files). See "Implementation Tasks" for exact insertions.
docs/configuration.md   # +[defaults] FR-R7 comment +[role.planner] timeout example +4 env rows +per-role prose
docs/how-it-works.md    # +L132 cross-ref note +L303 FR-T5 multi-turn budget fix
README.md               # +--<role>-timeout mention (planner 480s) in the multi-commit section
docs/cli.md             # fix 4 per-role-timeout flag-map rows: git-config column — → stagecoach.role.<role>.timeout
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (the planner 480s is a BUILT-IN, not overridable by --timeout): ResolveRoleTimeout returns
     defaultRoleTimeouts["planner"]=480s BEFORE falling back to cfg.Timeout. So `--timeout 600s` does NOT
     change the planner — it changes stager/message/arbiter. State this explicitly in the docs (the #1 user
     confusion). To change the planner: --planner-timeout / [role.planner].timeout / STAGECOACH_PLANNER_TIMEOUT
     / stagecoach.role.planner.timeout. -->

<!-- CRITICAL (the git-config layer WAS implemented despite the delta_prd "Out of scope"): git.go:167 reads
     stagecoach.role.<role>.timeout. docs/configuration.md already documents it (L228 ✓). BUT docs/cli.md's
     flag map (L453-456) shows the four per-role-timeout rows' git-config column as "—" — that is STALE and
     MUST be filled in. (Provider/model/reasoning per-role rows correctly stay "—" — those have NO git-config
     layer; ONLY timeout does.) -->

<!-- CRITICAL (single-commit path = message role = 120s): the message role has NO built-in ⇒ inherits the
     global 120s (was 480s pre-v2.8). Document the deliberate default change + the back-compat escape
     (--timeout 480s / [role.message].timeout) so users who relied on the old 480s aren't surprised. -->

<!-- CRITICAL (do NOT edit internal/config/bootstrap.go): the `config init` template's [defaults] timeout
     comment lacks the planner note + the env block lacks the per-role STAGECOACH_<ROLE>_TIMEOUT variants.
     The delta_prd Phase 3 #1 wanted these fixed, but bootstrap.go is a SOURCE file (FORBIDDEN) and is NOT in
     this task's contract. The human-readable docs/configuration.md env table + [defaults] comment ARE in
     scope. Flag the residual; do not edit the .go file. -->

<!-- GOTCHA (markdownlint rules): MD013 (line length) is OFF — the existing docs are single-long-line
     paragraphs (no hard wrapping). Match that: write prose as long lines, do NOT hard-wrap at 80 cols.
     MD033 (inline HTML) and MD060 are also off, so > [!NOTE] and tables are fine — mirror the existing style. -->

<!-- GOTCHA (the config-FILE layer is stricter than env/flag/git on bare ints): file.go parses [role.<role>].timeout
     via time.ParseDuration (accepts "600s", NOT bare "600"); env/flag/git use parseTimeout (accepts both). Use the
     "600s" form in config-file examples to be safe and consistent. -->

<!-- GOTCHA (docs/cli.md L27 is ALREADY correct): the global --timeout default reads "120s" — contract point
     (d) PASSES with no edit. Do NOT "fix" a non-bug. The ONLY cli.md edit is the flag-map git-config column
     on the four per-role-timeout rows. -->
```

## Implementation Blueprint

### Data models and structure
None — this is a documentation task. No code, no schemas. The "data" is the exact surface names
(`--<role>-timeout`, `STAGECOACH_<ROLE>_TIMEOUT`, `[role.<role>].timeout`, `stagecoach.role.<role>.timeout`)
and the 120s/480s asymmetry captured in research/findings.md §0.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT docs/configuration.md — the PRIMARY target (contract a)
  - SUBTASK 1a — [defaults] timeout comment (contract a3). LOCATE L86 in the file-format [defaults] example:
        `# timeout        = "120s"`
    REPLACE with:
        `# timeout        = "120s"   # global fallback for every role (FR-R7); the planner role defaults to 480s`
    (PRD §16.2 wording — match it; do NOT change the value 120s, which is correct.)
  - SUBTASK 1b — [role.planner] timeout example (contract a2). LOCATE the [role.*] file-format block (~L92):
        [role.planner]
        model = "opus"
    ADD a commented timeout line under it:
        [role.planner]
        model = "opus"
        # timeout = "600s"   # per-role generation timeout (FR-R7); overrides the planner's 480s built-in
    (Only the planner needs the example — it's the role with the built-in. Leave stager/message/arbiter as-is.)
  - SUBTASK 1c — four per-role ENV-VAR rows (contract a1). LOCATE the env-vars table (L169-202). After the
    per-role REASONING rows (~L202), ADD four rows mirroring the existing 4-col format
    (Variable | Mirrors flag | Description | Example):
        | `STAGECOACH_PLANNER_TIMEOUT` | `--planner-timeout` | Per-role: planner generation timeout (FR-R7; planner built-in 480s) | `STAGECOACH_PLANNER_TIMEOUT=600s stagecoach` |
        | `STAGECOACH_STAGER_TIMEOUT`  | `--stager-timeout`  | Per-role: stager generation timeout (FR-R7; inherits 120s) | `STAGECOACH_STAGER_TIMEOUT=300s stagecoach` |
        | `STAGECOACH_MESSAGE_TIMEOUT` | `--message-timeout` | Per-role: message generation timeout (FR-R7; the single-commit path's only role; inherits 120s) | `STAGECOACH_MESSAGE_TIMEOUT=120s stagecoach` |
        | `STAGECOACH_ARBITER_TIMEOUT` | `--arbiter-timeout` | Per-role: arbiter generation timeout (FR-R7; inherits 120s) | `STAGECOACH_ARBITER_TIMEOUT=120s stagecoach` |
    ALSO update the existing STAGECOACH_TIMEOUT row (L184) Description to: "Global generation timeout — the
    fallback for every role (FR-R7); the planner role defaults to 480s and is NOT changed by this" so a
    reader who lands on the global row learns the asymmetry.
  - SUBTASK 1d — per-role prose (contract a1). LOCATE L251:
        "Every role (including message) exposes `--<role>-provider`/`--<role>-model`/`--<role>-reasoning` (FR-R3)."
    EXTEND the flag list to include timeout AND add a sentence:
        "Every role (including message) exposes `--<role>-provider`/`--<role>-model`/`--<role>-reasoning`/
        `--<role>-timeout` (FR-R3/FR-R7). Each role also resolves its **own generation timeout** (FR-R7): the
        **planner defaults to 480s** (the heavy role that reasons over the full frozen diff), while
        **stager/message/arbiter inherit the global `120s`**. Override a role with `--<role>-timeout` /
        `STAGECOACH_<ROLE>_TIMEOUT` / `[role.<role>].timeout` / `stagecoach.role.<role>.timeout` (precedence:
        flag > env > `[role.<role>]` > built-in > `[defaults]`). Note the global `--timeout` does **not**
        change the planner — it has a 480s built-in that wins; set `--planner-timeout` to override it."

Task 2: EDIT docs/how-it-works.md (contract b)
  - SUBTASK 2a — cross-ref note (L132). LOCATE:
        "See [configuration.md](configuration.md) for per-role model configuration and [cli.md](cli.md) for the decompose and per-role flags."
    EXTEND to mention timeout + the asymmetry:
        "See [configuration.md](configuration.md) for per-role provider/model/reasoning/**timeout**
        configuration and [cli.md](cli.md) for the decompose and per-role flags. Each role resolves its own
        generation timeout (FR-R7): the **planner defaults to 480s** (the heavy role), while stager/message/
        arbiter inherit the global `120s`; override with `--<role>-timeout`."
  - SUBTASK 2b — multi-turn budget fix (L303, FR-T5). LOCATE in the multi-turn section:
        "Each turn is a separate provider invocation with its own timeout; total wall-clock ≈ `timeout × (N+1)`, surfaced on the progress line at fallback time."
    REPLACE `timeout` with the message-role timeout (FR-T5):
        "Each turn is a separate provider invocation bounded by the **message role's** resolved timeout
        (FR-R7/FR-T5); total wall-clock ≈ `message-timeout × (N+1)`, surfaced on the progress line at fallback time."
    (multiturn.go uses ResolveRoleTimeout("message", cfg); the old flat `cfg.Timeout` wording is stale.)

Task 3: EDIT README.md — multi-commit section (contract c)
  - LOCATE the per-repo config example in the Multi-commit decomposition section (~L167-170):
        # Route planning to a bigger model (per-repo .stagecoach.toml):
        # [role.planner]
        # provider = "claude"
        # model = "opus"
    ADD a commented timeout line to the [role.planner] example AND a one-line note:
        # Route planning to a bigger model (per-repo .stagecoach.toml):
        # [role.planner]
        # provider = "claude"
        # model = "opus"
        # timeout = "600s"   # per-role generation timeout (FR-R7); the planner defaults to 480s
  - OPTIONALLY add a `> [!NOTE]` after the existing `--reasoning` NOTE (~L171-173) — only if the example line
    alone feels undiscoverable:
        > [!NOTE]
        > Each role resolves its **own generation timeout** (`--<role>-timeout`, FR-R7): the planner defaults
        > to **480s** (the heavy role), the others inherit the global `120s`. See the [CLI reference](docs/cli.md).
    (Pick ONE of the two — the example line is the lighter touch and matches the section's existing comment
    style; the NOTE is the more discoverable option. Do NOT bloat — README is the marketing surface, §21.5.)

Task 4: EDIT docs/cli.md — VERIFY default + FIX the stale flag map (contract d)
  - VERIFY (no edit): L27 `--timeout <dur>` Default column reads `"120s"`. (It does — contract point d PASSES.)
    Do NOT change it.
  - FIX the flag↔env↔git-config map (L453-456). LOCATE the four per-role-timeout rows:
        | `--planner-timeout` | `STAGECOACH_PLANNER_TIMEOUT` | — |
        | `--stager-timeout`  | `STAGECOACH_STAGER_TIMEOUT`  | — |
        | `--message-timeout` | `STAGECOACH_MESSAGE_TIMEOUT` | — |
        | `--arbiter-timeout` | `STAGECOACH_ARBITER_TIMEOUT` | — |
    REPLACE the git-config column `—` with the real key (git.go:167 reads it):
        | `--planner-timeout` | `STAGECOACH_PLANNER_TIMEOUT` | `stagecoach.role.planner.timeout` |
        | `--stager-timeout`  | `STAGECOACH_STAGER_TIMEOUT`  | `stagecoach.role.stager.timeout`  |
        | `--message-timeout` | `STAGECOACH_MESSAGE_TIMEOUT` | `stagecoach.role.message.timeout` |
        | `--arbiter-timeout` | `STAGECOACH_ARBITER_TIMEOUT` | `stagecoach.role.arbiter.timeout` |
    DO NOT touch the per-role provider/model/reasoning rows (L437-452) — those correctly stay `—` (NO
    git-config layer exists for provider/model/reasoning; ONLY timeout has one).

Task 5: VERIFY — markdownlint, grep guards, scope guard
  - npx markdownlint-cli2 'README.md' 'docs/**/*.md'              # clean (MD013/MD033/MD060 off)
  - grep guards (see Validation Loop Level 2/4)
  - git status --porcelain                                         # ONLY the four docs files
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN (env-var row — mirror docs/configuration.md's 4-col table exactly):
     | `STAGECOACH_<ROLE>_TIMEOUT` | `--<role>-timeout` | Per-role: <role> generation timeout (FR-R7; …) | `STAGECOACH_<ROLE>_TIMEOUT=… stagecoach` |
     Keep the Description column concise but name the asymmetry (planner built-in 480s; others inherit 120s). -->

<!-- PATTERN (config-file example — match the existing [role.*] block's commented style):
     [role.planner]
     model = "opus"
     # timeout = "600s"   # per-role generation timeout (FR-R7); overrides the planner's 480s built-in
     Use the "600s" duration-string form (the file layer's time.ParseDuration does NOT accept bare "600"). -->

<!-- CRITICAL (the asymmetry sentence — repeat the key fact everywhere a reader might land):
     "the planner defaults to 480s (a built-in the global --timeout does NOT change); stager/message/arbiter
     inherit the global 120s." This belongs in: the [defaults] comment (1a), the env STAGECOACH_TIMEOUT row
     (1c), the per-role prose (1d), the how-it-works cross-ref (2a), and the README note (3). Repetition is
     intentional — readers enter at any anchor. -->

<!-- CRITICAL (cli.md flag map — ONLY the four timeout rows get a git-config value). Do NOT add
     stagecoach.role.* keys to the provider/model/reasoning rows — those have NO git-config layer (git.go is
     timeout-only). Filling those in would be a FALSE doc. -->
```

### Integration Points

```yaml
DOC CROSS-LINKS (anchors must resolve):
  - docs/configuration.md#environment-variables, #git-config-keys, #file-format — the edited sections.
  - docs/cli.md#global-flags (the flag table) + the flag↔env↔git-config map.
  - docs/how-it-works.md#multi-commit-decomposition + the multi-turn section.
  - README's multi-commit section links INTO docs/cli.md and docs/how-it-works.md (existing links — preserve).
CONSISTENCY (the same names across all four files):
  - `--<role>-timeout`, `STAGECOACH_<ROLE>_TIMEOUT`, `[role.<role>].timeout`, `stagecoach.role.<role>.timeout`,
    "planner defaults to 480s", "global 120s", "FR-R7".
NO build/config/runtime integration — docs-only.
```

## Validation Loop

### Level 1: Markdown lint (Immediate Feedback)

```bash
# markdownlint on the edited files (config disables MD013 line-length, MD033 inline-HTML, MD060).
npx markdownlint-cli2 'README.md' 'docs/**/*.md'
# Expected: clean. If a finding appears, it is a real style issue (stray blank line / table pipe) — fix it.
# Long lines, > [!NOTE], and tables are fine (those rules are off). Do NOT hard-wrap prose.

# Fallback if markdownlint-cli2 is unavailable:
npx -y markdownlint-cli README.md docs/cli.md docs/configuration.md docs/how-it-works.md
# Expected: clean.
```

### Level 2: Wording + consistency (grep-based)

```bash
# Per-role timeouts are now documented across all FOUR files.
for f in README.md docs/how-it-works.md docs/configuration.md docs/cli.md; do
  echo "== $f =="; grep -ciE "per-role|role-timeout|STAGECOACH_.*_TIMEOUT|role\..*\.timeout|FR-R7|480s" "$f"
done
# Expected: every file ≥1.

# The planner-480s asymmetry is stated (the #1 confusion point).
grep -rniE "planner.*(defaults to|built-in).*480|480s.*planner" README.md docs/
# Expected: ≥3 hits (configuration [defaults] comment + prose + how-it-works cross-ref; README note).

# The four override surfaces are named consistently.
grep -rn "STAGECOACH_PLANNER_TIMEOUT" docs/configuration.md docs/cli.md   # env
grep -rn "stagecoach.role.planner.timeout" docs/configuration.md docs/cli.md   # git-config
grep -rn "\[role.planner\].*timeout\|role.planner.*timeout\|--planner-timeout" docs/configuration.md docs/cli.md README.md
# Expected: each ≥1 hit.

# NO docs file claims the GLOBAL --timeout default is 480s (480s is planner-only).
grep -rniE "default 480s|default.*480s" docs/cli.md README.md docs/configuration.md
# Expected: ZERO hits that tie 480s to the global --timeout default. (480s may appear as the planner built-in.)
grep -nE '\| `--timeout`' docs/cli.md   # confirm the global --timeout row shows 120s, not 480s
# Expected: the --timeout row's Default column says 120s.
```

### Level 3: cli.md flag-map coherence (the stale-artifact fix)

```bash
# The four per-role-timeout rows now show the git-config key (was the stale "—").
grep -nE '\| `--planner-timeout`.*stagecoach.role.planner.timeout' docs/cli.md
grep -nE '\| `--stager-timeout`.*stagecoach.role.stager.timeout' docs/cli.md
grep -nE '\| `--message-timeout`.*stagecoach.role.message.timeout' docs/cli.md
grep -nE '\| `--arbiter-timeout`.*stagecoach.role.arbiter.timeout' docs/cli.md
# Expected: 1 hit each (4 total).

# The provider/model/reasoning per-role rows STILL show "—" (NO git-config layer for those — correct).
grep -nE '\| `--planner-(provider|model|reasoning)`.*\| —' docs/cli.md
# Expected: hits (provider/model/reasoning correctly have no git-config key). Do NOT "fix" these to a key.

# The multi-turn how-it-works line reflects FR-T5 (message-role timeout, not flat timeout).
grep -n "message-timeout\|message role" docs/how-it-works.md | grep -i "timeout\|N+1"
# Expected: ≥1 hit (the L303 fix).
```

### Level 4: Stale-reference + scope guards

```bash
# Guard 1: scope — ONLY the four docs files changed.
git status --porcelain
# Expected: README.md, docs/cli.md, docs/configuration.md, docs/how-it-works.md ONLY.
git diff --name-only | grep -vE '^(README\.md|docs/(cli|configuration|how-it-works)\.md)$' && echo "FAIL: out-of-scope file edited" || echo "OK: scope clean"

# Guard 2: NO source/PRD/plan/tasks files touched — especially NOT bootstrap.go.
git diff --name-only | grep -E '^(PRD\.md|plan/|tasks\.json|prd_snapshot\.md|internal/|cmd/|providers/|FUTURE_SPEC\.md|\.markdownlint\.json|Makefile|go\.mod)' && echo "FAIL: forbidden file edited" || echo "OK: no forbidden files"
git diff --name-only | grep -q 'internal/config/bootstrap.go' && echo "FAIL: edited bootstrap.go (PRODUCTION, out of scope — see findings §3)" || echo "OK: bootstrap.go untouched"

# Guard 3: internal links still resolve (markdownlint does NOT check anchors).
grep -nE "^## Environment variables|^### File format|^## Global flags|^## Multi-commit decomposition" docs/configuration.md docs/cli.md docs/how-it-works.md README.md
# Expected: the headings the new content links to all exist.

# Guard 4: the global --timeout default is 120s in cli.md (contract point d — verify, do not "fix").
grep -nE '\| `--timeout <dur>`' docs/cli.md
# Expected: the row shows Default "120s". (If it shows 480s, that IS a bug — but it currently shows 120s.)
```

## Final Validation Checklist

### Technical Validation
- [ ] `npx markdownlint-cli2 'README.md' 'docs/**/*.md'` clean (MD013/MD033/MD060 off — long lines OK)
- [ ] grep guards (Level 2/3/4) all pass: planner-480s asymmetry stated; four override surfaces named; no
      global-480s default claim; cli.md flag-map git-config column filled for the four timeout rows only

### Feature Validation
- [ ] docs/configuration.md: `[defaults]` timeout comment (FR-R7; planner 480s) + `[role.planner]` timeout
      example + four `STAGECOACH_<ROLE>_TIMEOUT` env rows + per-role prose naming the asymmetry
- [ ] docs/how-it-works.md: L132 cross-ref notes per-role timeouts (planner 480s); L303 multi-turn line reads
      `message-timeout × (N+1)` (FR-T5)
- [ ] README.md: multi-commit section mentions `--<role>-timeout` + planner 480s
- [ ] docs/cli.md: L27 `--timeout` default verified 120s (no edit); four per-role-timeout flag-map rows have
      the git-config column filled (`stagecoach.role.<role>.timeout`)
- [ ] A user reading the docs discovers per-role timeouts and the planner-480s / others-120s asymmetry (the
      contract's OUTPUT criterion: "Documentation consistently describes per-role timeouts as a feature")

### Scope-Boundary Validation
- [ ] `git status` shows ONLY the four docs files (Level 4 Guard 1)
- [ ] NO edit to PRD.md, plan/**, tasks.json, prd_snapshot.md, any internal/* or cmd/* source, providers/*.toml,
      FUTURE_SPEC.md, .markdownlint.json, Makefile, go.mod (Guard 2)
- [ ] internal/config/bootstrap.go UNCHANGED (PRODUCTION; the [defaults]-comment + env-block gap is flagged in
      findings §3 as a known residual owned elsewhere — NOT edited here) (Guard 2)

---

## Anti-Patterns to Avoid

- ❌ Don't claim `--timeout` (global) changes the planner — it does NOT (the planner has a 480s built-in that
  wins). State the asymmetry explicitly; it is the #1 confusion point.
- ❌ Don't invent per-role built-ins for stager/message/arbiter — only the planner has one (480s). The other
  three inherit the global 120s.
- ❌ Don't leave the cli.md flag-map git-config column as "—" for the per-role-timeout rows — the layer EXISTS
  (git.go:167); fill it in. BUT do NOT fill in the provider/model/reasoning rows — those have NO git-config
  layer (git.go is timeout-only); leaving them "—" is CORRECT.
- ❌ Don't hard-wrap prose at 80 cols — MD013 is OFF and the existing docs are long-line; wrapping is
  inconsistent and a reviewer red flag.
- ❌ Don't edit internal/config/bootstrap.go. It is a PRODUCTION file (FORBIDDEN) and NOT in this task's
  contract. Its [defaults]-comment + env-block gaps are flagged (findings §3); the human-readable
  docs/configuration.md is the authoritative public surface and IS in scope.
- ❌ Don't use bare-int ("600") in the config-FILE example (`[role.planner].timeout`) — the file layer
  (time.ParseDuration) rejects bare ints; use "600s". (Env/flag/git accept bare ints via parseTimeout, but
  the config-file example should use the stricter "600s" form to be safe.)
- ❌ Don't "fix" docs/cli.md L27 — the global `--timeout` default already reads "120s" (contract point d
  PASSES). The ONLY cli.md edit is the flag-map git-config column on the four per-role-timeout rows.
- ❌ Don't bloat the README — it is the marketing surface (§21.5). One example line OR one `> [!NOTE]` for
  per-role timeouts; do not re-document the full surface (point at docs/cli.md).
- ❌ Don't contradict the planner-480s fact across anchors. State it in the `[defaults]` comment, the
  STAGECOACH_TIMEOUT env row, the per-role prose, the how-it-works cross-ref, and the README note — readers
  enter at any anchor.
