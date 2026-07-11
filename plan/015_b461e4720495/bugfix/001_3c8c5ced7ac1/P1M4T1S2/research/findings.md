# P1.M4.T1.S2 Research Findings — Review docs/how-it-works.md + docs/cli.md

## §0 Headline

- **docs/cli.md**: ONE concrete stale reference — the `config upgrade` example output (line 210) omits the
  backup line that Issue 3 (P1.M2.T2.S1) added. **RECOMMENDED EDIT** (correct a stale transcript).
- **docs/how-it-works.md**: NO stale references. The token_limit description (line 150) stays accurate
  after Issue 4 (the floor fix preserves the "caps to a token budget" guarantee by *erroring* on sub-floor
  limits — vacuously true); the floor detail already lives in `configuration.md:167` (linked from line 152),
  landed in commit `fc51dad`. **DEFAULT: no edit**; review documented. (Optional alt: one brief floor clause
  on line 150 — see §5.)

## §1 Scope resolution (critical — read first)

The contract (`item_description`) INPUT/OUTPUT names **only** `docs/how-it-works.md` + `docs/cli.md`.
`architecture/test_patterns.md` "Documentation Files" assigns TWO *other* Mode-A doc notes:

| Doc | Line | Mode-A note | Status |
|-----|------|-------------|--------|
| `docs/configuration.md` | 167 | Issue 4 floor-rejection note | ✅ **ALREADY LANDED** — commit `fc51dad "Reject sub-floor token_limit with error (Issue 4)"`. Verified: "For extremely small limits (below the irreducible prompt floor …), Stagecoach rejects the limit with a clear error rather than silently violating the guarantee." |
| `docs/providers.md` | 125 | Issue 2 commented-pi-block blank note | ✅ **ALREADY LANDED** — commit `84b5296 "Blank commented-pi models on config init (FR-R5b)"`. Verified: "EXCEPT for **pi**, whose per-role models are written EMPTY in BOTH the active `[role.*]` block AND the commented-out pi block …". |

**Conclusion**: the configuration.md + providers.md Mode-A updates were done inline by their code-fix
commits (the "Mode A" pattern). They are NOT this task's scope (the contract names only the two files
above) AND they are already complete. **This task touches ONLY `docs/how-it-works.md` + `docs/cli.md`.**
`git status` confirms: only `tasks.json` is modified repo-wide; no doc is mid-edit.

## §2 Fixed behavior per issue (the source of truth the docs are measured against)

| Issue | Fix (landed commit) | Doc-relevant behavior |
|-------|---------------------|------------------------|
| 1 (bootstrap stager-fallback bare pi) | `457edc6`+blanking | `config init` writes pi per-role models EMPTY (stager-fallback blanked) |
| 2 (commented-out pi block bare models) | `84b5296` | commented-out pi `[role.*]` block also uses blank/prefixed models |
| 3 (config upgrade no backup, FR-B8) | `aa58840`+`d499742` | `config upgrade` now writes a timestamped backup FIRST + prints `Backed up previous config to <path>.bak.<ts>` (stderr) before `Upgraded config at <path> to version 3.` (stdout) |
| 4 (token gate sub-floor invariant, FR3j) | `fc51dad` | sub-floor `token_limit` is REJECTED with an error at StagedDiff/TreeDiff/WorkingTreeDiff BEFORE assembly (no silent over-budget payload) |
| 5 (doubled `stagecoach:` prefix) | `9a24822` | `--edit` empty abort now prints single `stagecoach: empty commit message — aborted` |
| 6 (auto-stage "(1 files)" grammar) | `e821046` | n==1 → "(1 file)"; n≥2 → "(N files)" |

Exact post-Issue-3 `config upgrade` output (verified `internal/cmd/config.go`):
- `config.go:183` stdout: `"Config at %s is already at version %d (no changes).\n"` (already-current no-op — NO backup)
- `config.go:193` stderr: `"Backed up previous config to %s\n"` (NEW — only when a real write happens)
- `config.go:198` stdout: `"Upgraded config at %s to version %d.\n"`

## §3 docs/how-it-works.md review (402 lines)

### Area (a) — token_limit / diff capture pipeline
- Line 136 `### Diff capture pipeline`; line 148 `Size budget (FR3d / FR3i)`; **line 150 "Holistic token
  budget"** — detailed water-fill description. Key claim: "Set `token_limit` … to cap the *whole* payload …
  to a token budget." Does NOT use the phrase "never exceeds the limit" / "closed-loop guarantee" (that
  language is in `configuration.md:167`, now with the floor note). Does NOT name FR3j.
- Line 152 links to `configuration.md#built-in-defaults` for the `token_limit` knob — where the floor note
  now lives (one click away).
- Line 312–316 (multi-turn path): "`token_limit` does not apply (FR-T12)" — accurate; unaffected by Issue 4
  (multi-turn re-captures with token_limit *disabled*, so the floor check in git.go one-shot path is moot).
- **FINDING**: line 150 is NOT stale. After Issue 4, a sub-floor limit ERRORS before assembly — it never
  produces an over-budget payload — so "caps the payload to a token budget" is *vacuously true* for the
  sub-floor case and literally true for all feasible limits. The floor detail is already in
  `configuration.md:167` (linked). → **NO EDIT (default).** Optional alt in §5.

### Area (b) — config init / config upgrade / backup
- grep (`config init|config upgrade|backup|bootstrap|per-role|left empty|bare model|FR-R5b|blank`) →
  **ZERO hits** in how-it-works.md. This doc covers the runtime pipeline, not config management.
  → **NO EDIT** (not covered; nothing to correct).

### Area (c) — auto-stage notice (Issue 6)
- grep (`staging all changes|Nothing staged|\(.*files\)|\(1 file`) → the literal notice
  "Nothing staged — staging all changes (N files)." does **NOT** appear. "nothing staged" appears only as
  descriptive prose about the decompose trigger (lines 49, 69, 179). → **NO EDIT** (Issue 6 has no surface).

### Bonus — --edit abort (Issue 5)
- grep (`empty commit|empty message|abort|stagecoach:`) → how-it-works.md never shows the literal
  `--edit` abort output. → **NO EDIT**.

**VERDICT (how-it-works.md)**: accurate across all areas; NO CHANGES. Review documented.

## §4 docs/cli.md review (506 lines)

### Area (a) — token_limit / floor
- grep (`token_limit|closed.loop|irreducible|floor|exceeds|water.fill|FR3j|diff capture`) → **ZERO hits**.
  cli.md documents flags/commands; token_limit is a config-file knob (covered in configuration.md).
  → **NO EDIT** (not covered).

### Area (b1) — config init (line 170–201)
- Line 172 ALREADY ACCURATE: "…EXCEPT for **pi** (the default), whose per-role models are left EMPTY so pi
  picks its own backend model (set the model with an inference-provider prefix (e.g. model = "zai/glm-5.2")
  to pin a backend (FR-R5b)). Other detected providers get their per-role models UNCOMMENTED. Other
  installed providers appear as commented-out `[role.*]` blocks."
  This EXACTLY matches the Issue 1/2 fixed behavior (pi models blanked in both active + commented blocks).
  No stale "bare pi model" claim. → **NO EDIT.**

### Area (b2) — config upgrade (line 203–214)  ⚠️ STALE
- Line 205 prose: "Upgrade an existing config's `config_version` … (3) **in place**. … Every other line is
  preserved. Idempotent … No flags." — does not mention the backup.
- Lines 208–211 example transcript:
  ```
  stagecoach config upgrade
  # Already at version 3 →  "Config at ~/.config/stagecoach/config.toml is already at version 3 (no changes)."
  # Upgraded from v1  →  "Upgraded config at ~/.config/stagecoach/config.toml to version 3."
  # No file          →  "no config file at <path> (run 'stagecoach config init' first)"  (exit 1)
  ```
  After Issue 3, the "Upgraded from v1" case ALSO prints (stderr) `Backed up previous config to
  <path>.bak.<ts>` BEFORE the "Upgraded config…" line. The example omits it → **STALE transcript**.
  → **RECOMMENDED EDIT** (§5): add the backup line to the "Upgraded from v1" scenario.

### Area (c) — auto-stage notice (Issue 6)
- grep → "Nothing staged" appears at lines 15–16 (synopsis routing), 406 (exit 2), 413 (busy) — all
  descriptive prose, NOT the literal notice "(N files)". → **NO EDIT** (Issue 6 has no surface).

### Bonus — --edit abort (Issue 5)
- Line 42 (`--edit`): "An empty result aborts (exit 1, not a rescue)." — descriptive, no literal
  doubled-prefix output string. → **NO EDIT.**

**VERDICT (cli.md)**: ONE EDIT — config upgrade example backup line (line 210). Optional companion: a
brief backup clause in the line-205 prose.

## §5 The edits (verbatim)

### EDIT 1 (RECOMMENDED) — cli.md line 210, config upgrade example
Corrects the stale "Upgraded from v1" transcript to include the backup line Issue 3 added.
- CURRENT:
  `# Upgraded from v1  →  "Upgraded config at ~/.config/stagecoach/config.toml to version 3."`
- PROPOSED:
  `# Upgraded from v1  →  "Backed up previous config to <path>.bak.<ts>" (stderr) then "Upgraded config at ~/.config/stagecoach/config.toml to version 3."`
- WHY: the example is a concrete transcript; after Issue 3 it prints a backup line first. Omitting it makes
  the example diverge from actual behavior. This is correcting a stale reference (Mode B), not a feature blurb.

### EDIT 2 (OPTIONAL companion) — cli.md line 205, config upgrade prose
- CURRENT: `…to the current schema version (3) in place.`
- PROPOSED: `…to the current schema version (3) in place (a timestamped backup of the prior file is written first).`
- WHY: makes the prose match the corrected behavior (FR-B8). Optional because EDIT 1 already surfaces the backup.

### EDIT 3 (OPTIONAL alt — how-it-works.md line 150, floor clause)
Only if the implementer decides the sub-floor rejection should be surfaced in the deep-dive doc (it is
already in configuration.md:167, linked from line 152). Insert after "to a token budget.":
`(A limit below the irreducible prompt floor is rejected with an error rather than silently broken; see [configuration](configuration.md#built-in-defaults).)`
- WHY DEFAULT = NO: line 150 is not stale (floor fix preserves the cap by erroring → vacuously true); the
  floor detail is already one click away in configuration.md:167; "no feature blurbs for bugfixes."

## §6 Validation approach (docs-review task)
- `git status --porcelain`: `docs/cli.md` ONLY (EDIT 1/2) OR `docs/cli.md` + `docs/how-it-works.md`
  (if EDIT 3 taken). NEVER a `.go` file, NEVER `README.md` (S1), NEVER `configuration.md`/`providers.md`
  (already landed), NEVER a PRD/task file.
- Scope guard: `git diff --name-only | grep -vE '^docs/(how-it-works|cli)\.md$' | grep -q . && echo FAIL || echo OK`
- Sanity: `make build && make test && make lint` (a markdown edit cannot break these; confirms no stray edit).
- Markdown eyeball: the cli.md example comment block still renders; the `→` arrow + quote style preserved.
