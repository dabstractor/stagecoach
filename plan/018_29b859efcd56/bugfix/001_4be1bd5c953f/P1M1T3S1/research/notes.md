# P1.M1.T3.S1 — Research notes (Mode B docs sweep for BUG-001 + BUG-002)

## 0. Task shape

Documentation-only (Mode B changeset-level sweep). Document the two code fixes:
- **BUG-001** (P1.M1.T1.S1+S2, Complete): `config init --force` now re-targets the regenerated template
  to the PRESERVED `[defaults] provider` instead of auto-detecting pi.
- **BUG-002** (P1.M1.T2.S1, code already landed in tree): the load-time version advisory NEVER suggests
  `config init --force` — newer-than-binary ⇒ "Upgrade stagecoach" only; older/missing ⇒ `config upgrade`.

Two doc files, four edit sites. No code, no tests.

## 1. The post-fix BEHAVIOR to document (verified against the code)

**BUG-001 — `config init --force` re-targeting** (internal/cmd/config.go:485-523,
`preservedDefaultProvider` @449):
- With `--force` and NO `--provider`: the regenerated template is re-targeted to the preserved
  `[defaults] provider` (read from the existing file). Generated `[role.*]` blocks use that provider's
  FR-D4 models, keeping them consistent with the preserved default (e.g. preserving
  `provider = "claude"` regenerates claude's role models, NOT pi's).
- An explicit `--provider <name>` ALWAYS overrides this (unchanged).
- A preserved CUSTOM/UNKNOWN provider falls back to "" (auto-detect) — never passed to
  GenerateBootstrapConfig, so custom providers are not broken.

**BUG-002 — version advisory** (internal/config/load.go configVersionNotice @629-645; ALREADY LANDED
in the working tree — advisory strings at :641/:644 say `config upgrade` with no `config init --force`;
only the doc comment @629-631 mentions `config init --force`, intentionally, as the rationale):
- **Newer-than-binary** config_version (the only LIVE configVersionNotice branch in Load): advisory =
  `"Upgrade stagecoach."` — upgrade-only, NEVER `config init --force` (it would regenerate at the older
  binary's schema and destroy the unreadable newer config; FR-B4).
- **Older/missing** config_version: the LIVE advisory is `migrationNotice` (internal/config/migrate.go:106),
  which says `Run 'stagecoach config upgrade' to persist this to the file.` — points at `config upgrade`
  only, NOT `config init --force`. (configVersionNotice's older/missing branches are dead in Load — the
  migration branch handles them first — and T2.S1 hardened them to also drop `config init --force`.)

⇒ The two existing doc advisory sentences (which say "config upgrade or config init --force") are now
STALE/INACCURATE for BOTH cases and must be rewritten.

## 2. The four edit sites (EXACT current text)

### docs/cli.md
- **config init section, `--force` flag row (line 197):**
  `| \`--force\` | Overwrite an existing config file |`
  → Add a re-targeting sentence AFTER the flag table (before the "--interactive runs a three-step wizard"
  paragraph @~199). [DO NOT touch line 413 `--force` — that is the `upgrade` command's flag, a different command.]
- **advisory sentence (line 214, end of `config upgrade` section):**
  `At load time, a missing or outdated \`config_version\` triggers an advisory pointing at \`config upgrade\` or \`config init --force\`.`
  → Rewrite: older/missing ⇒ `config upgrade`; newer-than-binary ⇒ "Upgrade stagecoach" only; NEVER
  `config init --force` (FR-B4 — would destroy a newer config).
- **line 182** `stagecoach config init --force` — a legitimate EXAMPLE command in the config-init code
  block. KEEP (it is a real usage, not an advisory claim).

### docs/configuration.md
- **Bootstrap section, `--force` flag row (line 49):**
  `| \`--force\` | Overwrite an existing config file. |`
  → Add a re-targeting sentence AFTER the "If a config file already exists, it is NOT overwritten unless
  `--force` is passed (exit code 1). Parent directories are created as needed." line (@~51).
- **Schema-versioning section, advisory sentence (line 68):**
  `At load time, if \`config_version\` is missing or older, stagecoach prints an advisory to stderr pointing at \`config upgrade\` (or \`config init --force\` to regenerate).`
  → Rewrite: drop the `config init --force` clause; add the newer-than-binary upgrade-only case.

## 3. Grep inventory — every `config init --force` occurrence in docs/ + README (reconcile completeness)

```
docs/configuration.md:68   advisory sentence  → EDIT (Edit 4)
docs/cli.md:182            example command    → KEEP (legitimate usage example)
docs/cli.md:214            advisory sentence  → EDIT (Edit 2)
```
(README.md: no `config init --force` references.) After the edits, the only `config init --force` hits
in docs will be: the KEEP'd example (cli.md:182) + the two new advisory sentences that mention it in a
"never suggests" context (intentional — the contract requires noting `config init --force` is NOT
suggested). NO other doc file references the old behavior.

## 4. Validation gates (docs-only; no Go code touched)

- **markdownlint baseline** (verified): `npx markdownlint-cli2 docs/cli.md docs/configuration.md` →
  **1 pre-existing error**: `docs/configuration.md:164 MD028/no-blanks-blockquote` (UNRELATED to the
  edit sites at lines 49/68; docs/cli.md is clean). The edits MUST NOT add new errors and MUST NOT
  disturb line 164. New sentences are regular paragraphs (not blockquotes) ⇒ no MD028 risk.
- `go test ./...` / `make test` — run as a no-regression sanity check (docs can't break Go tests; it
  confirms no source file was accidentally touched).
- Grep guards: the two advisory sentences rewritten; the two re-targeting sentences present; line 413
  (`upgrade` --force) untouched; line 182 example kept.

## 5. Scope fences

- ONLY docs/cli.md + docs/configuration.md edited. NO source code (the fixes are in
  internal/cmd/config.go + internal/config/load.go — already Complete/in-flight; this task documents them).
- NO PRD.md / tasks.json / prd_snapshot.md / .gitignore.
- The `config init --force` EXAMPLE (cli.md:182) and the `upgrade --force` row (cli.md:413) are NOT
  advisory claims — do NOT touch them. Only the two advisory SENTENCES + the two flag-section
  clarifications change.
- Mode B = changeset-level docs. No code-comment edits here (those were Mode A in T1.S2/T2.S1).